package exec

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/execute"
	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmstate"
)

func Command(sess *session.Global) *cobra.Command {
	var cfg execute.Config

	cmd := vmstate.BuildCommand(
		sess,
		func(ctx context.Context, sess *session.Regional, instanceIds []string) ([]types.InstanceStateChange, error) {
			return nil, execute.Execute(ctx, execute.ExecuteInput{
				Cfg:         cfg,
				Sess:        sess,
				InstanceIds: instanceIds,
			})
		},
	)

	cmd.Use = "exec"
	cmd.Short = "Execute SSM command on EC2 instances"

	cmd.Flags().StringVarP(&cfg.Document, flagDocument, "d", "", "SSM document name/ARN")
	cmd.Flags().VarP(&cfg.Params, "parameters", "p", "A list of parameters to pass to the SSM document")
	cmd.Flags().StringVarP(
		&cfg.OutputsDir,
		"outputs-dir",
		"o",
		"",
		"A directory to store execution outputs in. Defaults to the SSM command ID.",
	)

	cmd.MarkFlagRequired(flagDocument)

	return cmd
}

const (
	flagDocument = "document"
)
