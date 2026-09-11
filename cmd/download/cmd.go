package download

import (
	"context"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/ssmcmd"
)

func Command(sess *session.Global) *cobra.Command {
	var cfg ssmcmd.TransferConfig

	cmd := &cobra.Command{
		Use:   "download INSTANCE_ID",
		Short: "Download files/directories from a Windows using 'Powershell -> ZIP -> S3' flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return sess.RunSingle(c.Context(), func(ctx context.Context, r *session.Regional) error {
				return ssmcmd.ExecuteTransfer(ctx, ssmcmd.TransferInput{
					Cfg:         cfg,
					Sess:        r,
					InstanceIds: args,
				})
			})
		},
	}

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
