package ssmcmd

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type Config struct {
	Document   Document
	OutputsDir string
}

type Document struct {
	Name         string
	Params       Parameters
	InlineScript string
	ScriptFile   string

	upload *upload
}

func (d *Document) Resolve(ctx context.Context, bucket string, s3Client *s3.Client) error {
	switch d.Name {
	case "sh", "shell", docShellScript:
		d.Name = docShellScript
	case "ps", "powershell", docPowerShellScript:
		d.Name = docPowerShellScript
	case "ps-upload":
		d.Name = docPowerShellScript
		if err := d.resolveUploadScript(ctx, bucket, s3Client); err != nil {
			return err
		}
		fallthrough
	default:
		if d.InlineScript != "" || d.ScriptFile != "" {
			return fmt.Errorf("script can only be specified for %s and %s documents", docPowerShellScript, docShellScript)
		}
	}

	if d.upload == nil {
		switch d.Name {
		case docPowerShellScript, docShellScript:
			if err := d.resolveScriptContent(); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveScriptContent resolves d.InlineScript or d.ScriptFile in-place
// and adjusts d.Params accordingly.
func (d *Document) resolveScriptContent() error {
	if d.InlineScript == "" && d.ScriptFile == "" {
		return fmt.Errorf("no script content specified for document %s", d.Name)
	}

	if d.Params == nil {
		d.Params = make(Parameters)
	}

	if d.InlineScript != "" {
		d.Params["commands"] = []string{d.InlineScript}
	} else {
		body, err := os.ReadFile(d.ScriptFile)
		if err != nil {
			return err
		}

		body = bytes.ReplaceAll(body, []byte{'\r'}, []byte{})

		for _, line := range bytes.Split(body, []byte{'\n'}) {
			d.Params["commands"] = append(d.Params["commands"], string(line))
		}
	}

	return nil
}

//go:embed upload-template.ps1
var uploadTemplateBody string
var uploadTemplate = template.Must(template.New("upload.ps1").Parse(uploadTemplateBody))

// resolveUploadScript resolves PowerShell upload script in-place
// and modifies p content.
func (d *Document) resolveUploadScript(ctx context.Context, bucket string, s3Client *s3.Client) error {
	var dir string
	if dirVal := d.Params["dir"]; len(dirVal) != 1 {
		return fmt.Errorf("parameter dir has invalid value %v", dirVal)
	} else {
		dir = dirVal[0]
	}

	var zipPath string
	if zipVal := d.Params["zip"]; len(zipVal) == 1 {
		zipPath = zipVal[0]
	} else {
		parts := strings.Split(dir, `\`)
		if len(parts) > 1 {
			zipPath = fmt.Sprintf(`$env:TEMP\%s.zip`, parts[len(parts)-1])
		} else {
			return fmt.Errorf("cannot deduce ZIP path from %q, provide ZIP path explicitly", dir)
		}
	}

	var outfile string
	if outVal := d.Params["out"]; len(outVal) == 1 {
		outfile = outVal[0]
	} else {
		return fmt.Errorf("invalid value for out: %v", outVal)
	}

	delete(d.Params, "dir")
	delete(d.Params, "zip")
	delete(d.Params, "out")

	parts := strings.Split(zipPath, `\`)
	filename := parts[len(parts)-1]
	key := fmt.Sprintf("ps-uploads/%s/%s", uuid.NewString(), filename)

	presignClient := s3.NewPresignClient(s3Client, s3.WithPresignExpires(time.Hour))
	req, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return err
	}

	// TODO: making skipping ".exe"s optional
	var scriptBuf bytes.Buffer
	if err := uploadTemplate.Execute(&scriptBuf, struct {
		Dir     string
		ZipPath string
		URL     string
	}{
		Dir:     dir,
		ZipPath: zipPath,
		URL:     req.URL,
	}); err != nil {
		return err
	}

	sc := bufio.NewScanner(&scriptBuf)
	for sc.Scan() {
		d.Params["commands"] = append(d.Params["commands"], sc.Text())
	}
	if sc.Err() != nil {
		return sc.Err()
	}

	d.upload = &upload{
		bucket:  bucket,
		key:     key,
		outfile: outfile,
	}

	return nil
}

type Parameters map[string][]string

func (p Parameters) Type() string {
	return "SSM document parameters"
}

func (p Parameters) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *Parameters) Set(raw string) error {
	if *p == nil {
		*p = make(Parameters)
	}

	parts := strings.SplitN(raw, "=", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid document parameter specifier: %q", raw)
	}

	(*p)[parts[0]] = append((*p)[parts[0]], parts[1])

	return nil
}

type upload struct {
	bucket  string
	key     string
	outfile string
}

func (u upload) save(ctx context.Context, s3Client *s3.Client) error {
	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &u.bucket,
		Key:    &u.key,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(u.outfile, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

const (
	docShellScript      = "AWS-RunShellScript"
	docPowerShellScript = "AWS-RunPowerShellScript"
)
