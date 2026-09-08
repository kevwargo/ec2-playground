package ssmcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

type Config struct {
	Document Document
	Outcfg   OutputConfig
}

type ExecuteInput struct {
	Cfg         Config
	Sess        *session.Regional
	InstanceIds []string
}

type OutputConfig struct {
	Bucket string
	Ignore bool
	Dump   bool
	Dir    string
}

func Execute(ctx context.Context, in ExecuteInput) error {
	resources, err := infra.NewFetcher(in.Sess, config.InfraConfig{
		StackName:  infra.DefaultStackName,
		SkipDeploy: true,
	}).Fetch(ctx)
	if err != nil {
		return err
	}

	bucket := in.Cfg.Outcfg.Bucket
	if bucket == "" {
		bucket = resources.Bucket
	}

	if err := in.Cfg.Document.resolve(); err != nil {
		return err
	}

	resp, err := in.Sess.SSM().SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName:       &in.Cfg.Document.Name,
		InstanceIds:        in.InstanceIds,
		OutputS3BucketName: &bucket,
		OutputS3KeyPrefix:  aws.String("ssm-command-logs"),
		Parameters:         in.Cfg.Document.Params,
		NotificationConfig: &ssmtypes.NotificationConfig{
			NotificationArn:    &resources.CmdNotification,
			NotificationType:   ssmtypes.NotificationTypeInvocation,
			NotificationEvents: []ssmtypes.NotificationEvent{ssmtypes.NotificationEventAll},
		},
		ServiceRoleArn: &resources.CmdNotificationRole,
	})
	if err != nil {
		return err
	}

	in.Sess.Log("Command %s started", *resp.Command.CommandId)

	err = watchCommand(ctx, watchInput{
		sess:   in.Sess,
		cmd:    resp.Command,
		queue:  resources.CmdNotificationQueue,
		outcfg: in.Cfg.Outcfg,
	})
	if err != nil {
		return err
	}

	return err
}

type watchInput struct {
	sess   *session.Regional
	cmd    *ssmtypes.Command
	queue  string
	outcfg OutputConfig
}

func watchCommand(ctx context.Context, in watchInput) error {
	state := make(commandState)

	for !state.allFinished() {
		notifications, err := pollNotifications(ctx, in)
		if err != nil {
			return err
		}

		if len(notifications) == 0 {
			notifications, err = checkNotifications(ctx, in)
			if err != nil {
				return err
			}
		}

		for _, change := range state.applyChanges(notifications) {
			if change.prev != nil {
				in.sess.Log("%s: %s -> %s (%s)", change.instanceID, change.prev, change.current, change.source)
			} else {
				in.sess.Log("%s: %s (%s)", change.instanceID, change.current, change.source)
			}

			if !in.outcfg.Ignore && !change.current.isPending() {
				if err = processInstanceOutputs(ctx, instanceOutputs{
					sess:       in.sess,
					cmd:        in.cmd,
					instanceID: change.instanceID,
					outcfg:     in.outcfg,
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

type commandNotification struct {
	CommandID      string
	InstanceID     string
	Status         ssmtypes.CommandInvocationStatus
	DetailedStatus *string

	source string
}

// even if error occurs, may return non-empty list of notifications
func pollNotifications(ctx context.Context, in watchInput) ([]commandNotification, error) {
	resp, err := in.sess.SQS().ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:        &in.queue,
		WaitTimeSeconds: 20,
	})
	if err != nil {
		return nil, fmt.Errorf("sqs:ReceiveMessage: %w", err)
	}

	var notifications []commandNotification

	for _, msg := range resp.Messages {
		var notification commandNotification
		if json.Unmarshal([]byte(*msg.Body), &notification) != nil {
			continue
		}

		if notification.CommandID != *in.cmd.CommandId {
			continue
		}

		notification.source = "sqs"
		notifications = append(notifications, notification)

		if _, err := in.sess.SQS().DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      &in.queue,
			ReceiptHandle: msg.ReceiptHandle,
		}); err != nil {
			return notifications, fmt.Errorf("sqs:DeleteMessage(%q, %q): %w", *msg.MessageId, *msg.ReceiptHandle, err)
		}
	}

	return notifications, nil
}

func checkNotifications(ctx context.Context, in watchInput) ([]commandNotification, error) {
	resp, err := in.sess.SSM().ListCommandInvocations(ctx, &ssm.ListCommandInvocationsInput{
		CommandId: in.cmd.CommandId,
	})
	if err != nil {
		return nil, fmt.Errorf("listing SSM command %q invocations: %w", *in.cmd.CommandId, err)
	}

	var notifications []commandNotification

	for _, inv := range resp.CommandInvocations {
		notifications = append(notifications, commandNotification{
			CommandID:      *inv.CommandId,
			InstanceID:     *inv.InstanceId,
			Status:         inv.Status,
			DetailedStatus: inv.StatusDetails,

			source: "ssm",
		})
	}

	return notifications, nil
}

type commandState map[string]instanceStatus

type instanceStatus struct {
	status  ssmtypes.CommandInvocationStatus
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
	source     string
	instanceID string
	current    instanceStatus
	prev       *instanceStatus
}

func (is commandState) allFinished() bool {
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

func (s commandState) applyChanges(notifications []commandNotification) (changes []statusChange) {
	for _, notification := range notifications {
		status := instanceStatus{
			status:  notification.Status,
			details: notification.DetailedStatus,
		}

		prev, ok := s[notification.InstanceID]
		if !ok {
			s[notification.InstanceID] = status
			changes = append(changes, statusChange{
				instanceID: notification.InstanceID,
				current:    status,
				source:     notification.source,
			})
		} else if prev.status != notification.Status {
			s[notification.InstanceID] = status
			changes = append(changes, statusChange{
				instanceID: notification.InstanceID,
				current:    status,
				prev:       &prev,
				source:     notification.source,
			})
		}
	}

	return changes
}

type instanceOutputs struct {
	sess       *session.Regional
	cmd        *ssmtypes.Command
	instanceID string
	outcfg     OutputConfig
}

func processInstanceOutputs(ctx context.Context, in instanceOutputs) error {
	if !in.outcfg.Dump && in.outcfg.Dir == "" {
		in.outcfg.Dir = *in.cmd.CommandId
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
			if err = processSingleOutput(ctx, singleOutput{
				sess:       in.sess,
				bucket:     *in.cmd.OutputS3BucketName,
				prefix:     prefix,
				key:        *obj.Key,
				instanceID: in.instanceID,
				outcfg:     in.outcfg,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

type singleOutput struct {
	sess       *session.Regional
	bucket     string
	prefix     string
	key        string
	instanceID string
	outcfg     OutputConfig
}

func processSingleOutput(ctx context.Context, in singleOutput) error {
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

	var out io.Writer

	if in.outcfg.Dump {
		out = os.Stdout
	} else {
		path := fmt.Sprintf("%s/%s/%s", in.outcfg.Dir, in.instanceID, strings.TrimLeft(strings.TrimPrefix(in.key, in.prefix), "/"))
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating dir %s: %w", filepath.Dir(path), err)
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		defer f.Close()

		out = f
		in.sess.Log("Downloading %s -> %s...", url, path)
	}

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("reading %s: %w", url, err)
	}

	return nil
}

var pendingStatuses = []ssmtypes.CommandInvocationStatus{
	ssmtypes.CommandInvocationStatusPending,
	ssmtypes.CommandInvocationStatusInProgress,
	ssmtypes.CommandInvocationStatusDelayed,
	ssmtypes.CommandInvocationStatusCancelling,
}
