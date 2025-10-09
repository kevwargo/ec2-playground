package execute

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

type ExecuteInput struct {
	Cfg         Config
	Sess        *session.Regional
	InstanceIds []string
}

func Execute(ctx context.Context, in ExecuteInput) error {
	resources, err := infra.NewFetcher(in.Sess, config.InfraConfig{
		StackName:  infra.DefaultStackName,
		SkipDeploy: false,
	}).Fetch(ctx)
	if err != nil {
		return err
	}

	resp, err := in.Sess.SSM().SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName:       &in.Cfg.Document,
		InstanceIds:        in.InstanceIds,
		OutputS3BucketName: &resources.Bucket,
		OutputS3KeyPrefix:  aws.String("ssm-command-logs"),
		Parameters:         in.Cfg.Params,
	})
	if err != nil {
		return err
	}

	in.Sess.Log("Command %s started", *resp.Command.CommandId)

	return watchCommand(ctx, watchInput{
		sess:       in.Sess,
		cmd:        resp.Command,
		outputsDir: in.Cfg.OutputsDir,
	})
}

type watchInput struct {
	sess       *session.Regional
	cmd        *types.Command
	outputsDir string
}

func watchCommand(ctx context.Context, in watchInput) error {
	var state instanceStatuses

	for !state.allFinished() {
		time.Sleep(commandCheckInterval)

		resp, err := in.sess.SSM().ListCommandInvocations(ctx, &ssm.ListCommandInvocationsInput{
			CommandId: in.cmd.CommandId,
		})
		if err != nil {
			return fmt.Errorf("listing SSM command %q invocations: %w", *in.cmd.CommandId, err)
		}

		for _, change := range state.getChanges(resp.CommandInvocations) {
			if change.prev != nil {
				in.sess.Log("%s: %s -> %s", change.instanceID, change.prev, change.current)
			} else {
				in.sess.Log("%s: %s", change.instanceID, change.current)
			}

			if !change.current.isPending() {
				if err = downloadAllOutputs(ctx, downloadAllInput{
					sess:       in.sess,
					cmd:        in.cmd,
					instanceID: change.instanceID,
					outputsDir: in.outputsDir,
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

type instanceStatuses map[string]instanceStatus

type instanceStatus struct {
	status  types.CommandInvocationStatus
	details *string
}

func (s instanceStatus) String() string {
	status := string(s.status)
	if s.details != nil && *s.details != status {
		status += fmt.Sprintf(" (%s)", *s.details)
	}

	return status
}

func (s instanceStatus) isPending() bool {
	return slices.Contains(pendingStatuses, s.status)
}

type statusChange struct {
	instanceID string
	current    instanceStatus
	prev       *instanceStatus
}

func (is instanceStatuses) allFinished() bool {
	if len(is) == 0 {
		return false
	}

	for _, s := range is {
		if s.isPending() {
			return false
		}
	}

	return true
}

func (is *instanceStatuses) getChanges(invocations []types.CommandInvocation) (changes []statusChange) {
	for _, inv := range invocations {
		status := instanceStatus{
			status:  inv.Status,
			details: inv.StatusDetails,
		}

		prev, ok := (*is)[*inv.InstanceId]
		if !ok {
			if *is == nil {
				*is = make(instanceStatuses)
			}

			(*is)[*inv.InstanceId] = status
			changes = append(changes, statusChange{
				instanceID: *inv.InstanceId,
				current:    status,
			})
		} else if prev.status != inv.Status {
			(*is)[*inv.InstanceId] = status
			changes = append(changes, statusChange{
				instanceID: *inv.InstanceId,
				current:    status,
				prev:       &prev,
			})
		}
	}

	return changes
}

type downloadAllInput struct {
	sess       *session.Regional
	cmd        *types.Command
	instanceID string
	outputsDir string
}

func downloadAllOutputs(ctx context.Context, in downloadAllInput) error {
	outdir := in.outputsDir
	if outdir == "" {
		outdir = *in.cmd.CommandId
	}

	prefix := fmt.Sprintf("%s/%s/%s", *in.cmd.OutputS3KeyPrefix, *in.cmd.CommandId, in.instanceID)
	listIn := s3.ListObjectsV2Input{
		Bucket: in.cmd.OutputS3BucketName,
		Prefix: &prefix,
	}
	paginator := s3.NewListObjectsV2Paginator(in.sess.S3(), &listIn)
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing S3 objects (%+v): %w", listIn, err)
		}

		for _, obj := range resp.Contents {
			if err = downloadSingleOutput(ctx, downloadSingleInput{
				sess:       in.sess,
				bucket:     *in.cmd.OutputS3BucketName,
				prefix:     prefix,
				key:        *obj.Key,
				instanceID: in.instanceID,
				outdir:     outdir,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

type downloadSingleInput struct {
	sess       *session.Regional
	bucket     string
	prefix     string
	key        string
	instanceID string
	outdir     string
}

func downloadSingleOutput(ctx context.Context, in downloadSingleInput) error {
	url := fmt.Sprintf("s3://%s/%s", in.bucket, in.key)

	if !strings.HasPrefix(in.key, in.prefix) {
		in.sess.Log("Ignoring %s (has no %q prefix)", url, in.prefix)
		return nil
	}

	resp, err := in.sess.S3().GetObject(ctx, &s3.GetObjectInput{
		Bucket: &in.bucket,
		Key:    &in.key,
	})
	if err != nil {
		return fmt.Errorf("getting S3 object %s", url)
	}
	defer resp.Body.Close()

	path := fmt.Sprintf("%s/%s/%s", in.outdir, in.instanceID, strings.TrimPrefix(in.key, in.prefix))
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating dir %s: %w", filepath.Dir(path), err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	if _, err = io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("reading %s: %w", url, err)
	}

	in.sess.Log("%s -> %s", url, path)

	return nil
}

const commandCheckInterval = 10 * time.Second

var pendingStatuses = []types.CommandInvocationStatus{
	types.CommandInvocationStatusPending,
	types.CommandInvocationStatusInProgress,
	types.CommandInvocationStatusDelayed,
	types.CommandInvocationStatusCancelling,
}
