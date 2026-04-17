package ssmcmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

type TransferConfig struct {
	Paths             []string
	RemoteZipPath     string
	TargetZip         string
	TargetDir         string
	FileExcludeRegexp string
}

type TransferInput struct {
	Cfg         TransferConfig
	Sess        *session.Regional
	InstanceIds []string
}

//go:embed upload-template.ps1
var uploadTemplateBody string
var uploadTemplate = template.Must(template.New("upload.ps1").Parse(uploadTemplateBody))

func ExecuteTransfer(ctx context.Context, in TransferInput) error {
	resources, err := infra.NewFetcher(in.Sess, config.InfraConfig{
		StackName:  infra.DefaultStackName,
		SkipDeploy: true,
	}).Fetch(ctx)
	if err != nil {
		return err
	}

	upload, err := buildUpload(ctx, uploadInput{
		paths:             in.Cfg.Paths,
		remoteZipPath:     in.Cfg.RemoteZipPath,
		fileExcludeRegexp: in.Cfg.FileExcludeRegexp,
		s3Client:          in.Sess.S3(),
		bucket:            resources.Bucket,
	})
	if err != nil {
		return err
	}

	in.Sess.Log("sending %s command:\n%s\n", docPowerShellScript, strings.Join(upload.params["commands"], "\n"))

	err = Execute(ctx, ExecuteInput{
		Cfg: Config{
			Document: Document{Name: docPowerShellScript, Params: upload.params},
			Outcfg:   OutputConfig{Dump: true},
		},
		Sess:        in.Sess,
		InstanceIds: in.InstanceIds,
	})
	if err != nil {
		return err
	}

	return downloadZip(ctx, downloadInput{
		sess:      in.Sess,
		bucket:    resources.Bucket,
		key:       upload.key,
		targetZip: in.Cfg.TargetZip,
		targetDir: in.Cfg.TargetDir,
	})
}

type uploadInput struct {
	paths             []string
	remoteZipPath     string
	fileExcludeRegexp string
	s3Client          *s3.Client
	bucket            string
}

type uploadOutput struct {
	params Parameters
	key    string
}

func buildUpload(ctx context.Context, in uploadInput) (uploadOutput, error) {
	var zipPath string
	if in.remoteZipPath != "" {
		zipPath = in.remoteZipPath
	} else {
		if len(in.paths) == 1 {
			if name, isQualified := splitPath(in.paths[0]); isQualified {
				zipPath = fmt.Sprintf(`$env:TEMP\%s.zip`, name)
			}
		}
	}
	if zipPath == "" {
		zipPath = fmt.Sprintf(`$env:TEMP\%s.zip`, uuid.NewString())
	}

	zipName, _ := splitPath(zipPath)
	key := fmt.Sprintf("ps-uploads/%s/%s", uuid.NewString(), zipName)

	presignClient := s3.NewPresignClient(in.s3Client, s3.WithPresignExpires(time.Hour))
	req, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: &in.bucket,
		Key:    &key,
	})
	if err != nil {
		return uploadOutput{}, err
	}

	var scriptBuf bytes.Buffer
	if err := uploadTemplate.Execute(&scriptBuf, struct {
		Paths   []string
		ZipPath string
		URL     string
		Exclude string
	}{
		Paths:   in.paths,
		ZipPath: zipPath,
		URL:     req.URL,
		Exclude: in.fileExcludeRegexp,
	}); err != nil {
		return uploadOutput{}, err
	}

	params := make(Parameters)
	sc := bufio.NewScanner(&scriptBuf)
	for sc.Scan() {
		params["commands"] = append(params["commands"], sc.Text())
	}
	if sc.Err() != nil {
		return uploadOutput{}, sc.Err()
	}

	return uploadOutput{
		params: params,
		key:    key,
	}, nil
}

type downloadInput struct {
	sess      *session.Regional
	bucket    string
	key       string
	targetZip string
	targetDir string
}

func downloadZip(ctx context.Context, in downloadInput) (err error) {
	var targetZip *os.File
	if in.targetZip == "" {
		targetZip, err = os.CreateTemp("", filepath.Base(in.key))
		if err != nil {
			return err
		}
		defer func() {
			targetZip.Close()
			if err := os.Remove(targetZip.Name()); err != nil {
				in.sess.Log("cannot delete temp zip %q: %s", targetZip.Name(), err)
			}
		}()

		in.sess.Log("saving artifact to temp %s", targetZip.Name())
	} else {
		targetZip, err = os.OpenFile(in.targetZip, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		if err != nil {
			return err
		}
		defer targetZip.Close()
	}

	resp, err := in.sess.S3().GetObject(ctx, &s3.GetObjectInput{
		Bucket: &in.bucket,
		Key:    &in.key,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	n, err := io.Copy(targetZip, resp.Body)
	if err != nil {
		return err
	}

	in.sess.Log("downloaded %s (%d bytes)", targetZip.Name(), n)

	if in.targetDir != "" {
		if err := unzip(in.sess, targetZip, in.targetDir); err != nil {
			return err
		}
	}

	_, err = in.sess.S3().DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &in.bucket,
		Key:    &in.key,
	})
	if err != nil {
		return err
	}

	in.sess.Log("deleted s3://%s/%s", in.bucket, in.key)

	return nil
}

func unzip(sess *session.Regional, zipFile *os.File, dir string) (err error) {
	targetDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	sess.Log("unzipping %s to %s", zipFile.Name(), targetDir)

	info, err := zipFile.Stat()
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(zipFile, info.Size())
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, string(os.PathSeparator))

		destPath := filepath.Join(targetDir, name)
		if !strings.HasPrefix(destPath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			sess.Log("skipping unallowed %s", destPath)
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := unzipEntry(f, destPath); err != nil {
			return err
		}
	}

	return nil
}

func unzipEntry(entry *zip.File, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return err
	}

	srcFile, err := entry.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, entry.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func splitPath(path string) (base string, qualified bool) {
	parts := strings.Split(path, `\`)
	return parts[len(parts)-1], len(parts) > 1
}
