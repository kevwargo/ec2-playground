package download

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/ssmcmd"
	"kevwargo/ec2-playground/internal/vmstate"
)

func Command(sess *session.Global) *cobra.Command {
	var cfg ssmcmd.TransferConfig

	cmd := vmstate.BuildCommand(
		sess,
		func(ctx context.Context, sess *session.Regional, instanceIds []string) ([]types.InstanceStateChange, error) {
			switch len(instanceIds) {
			case 0:
				return nil, nil
			case 1:
			default:
				return nil, errors.New("Currently only one instance can be specified for downloading artifacts")
			}

			return nil, ssmcmd.ExecuteTransfer(ctx, ssmcmd.TransferInput{
				Cfg:         cfg,
				Sess:        sess,
				InstanceIds: instanceIds,
			})
		},
	)

	cmd.Use = "download"
	cmd.Short = "Download files/directories from a Windows using 'Powershell -> ZIP -> S3' flow"

	cmd.Flags().StringArrayVarP(&cfg.Paths, "paths", "p", nil, "List of paths on the remote end (can use PS syntax)")
	cmd.Flags().StringVarP(&cfg.TargetDir, flagTargetDir, "d", "", "Unpack the uploaded ZIP into that directory")
	cmd.Flags().StringVarP(&cfg.TargetZip, flagTargetZip, "z", "", "Save the ZIP here. If empty, a temp file will be used and deleted after unpacking")
	cmd.Flags().StringVar(&cfg.RemoteZipPath, "remote-zip-path", "", "The path of the ZIP on the remote, if empty a temp file is created")
	cmd.Flags().StringVarP(&cfg.FileExcludeRegexp, "exclude-regexp", "x", "", "Exclude files matching this regexp from the ZIP")

	cmd.MarkFlagsOneRequired(flagTargetDir, flagTargetZip)

	return cmd
}

const (
	flagTargetZip = "target-dir"
	flagTargetDir = "target-zip"
)
