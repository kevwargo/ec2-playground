package exec

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/ssmcmd"
	"kevwargo/ec2-playground/internal/vmstate"
)

func Command(sess *session.Global) *cobra.Command {
	var cfg ssmcmd.Config

	cmd := vmstate.BuildCommand(
		sess,
		func(ctx context.Context, sess *session.Regional, instanceIds []string) ([]types.InstanceStateChange, error) {
			return nil, ssmcmd.Execute(ctx, ssmcmd.ExecuteInput{
				Cfg:         cfg,
				Sess:        sess,
				InstanceIds: instanceIds,
			})
		},
	)

	cmd.Use = "exec"
	cmd.Short = "Execute SSM command on EC2 instances"

	cmd.Flags().StringVarP(&cfg.Document.Name, flagDocument, "d", "", "SSM document name/ARN")
	cmd.Flags().VarP(&cfg.Document.Params, "parameters", "p", "A list of parameters to pass to the SSM document")
	cmd.Flags().StringVarP(&cfg.Document.InlineScript, flagInlineScript, "c", "", "Script content passed as literal string")
	cmd.Flags().StringVarP(&cfg.Document.ScriptFile, flagScriptFile, "s", "", "Filename with script content")
	cmd.Flags().StringVarP(
		&cfg.Outcfg.Dir,
		"outputs-dir",
		"o",
		"",
		"A directory to store execution outputs in. Defaults to the SSM command ID.",
	)
	cmd.Flags().BoolVarP(&cfg.Outcfg.Ignore, "quiet", "q", false, "Ignore SSM command output")
	cmd.Flags().BoolVarP(&cfg.Outcfg.Dump, "dump-output", "O", false, "Dump all SSM command output to stdout directly")

	cmd.MarkFlagRequired(flagDocument)
	cmd.MarkFlagsMutuallyExclusive(flagInlineScript, flagScriptFile)

	return cmd
}

const (
	flagDocument     = "document"
	flagInlineScript = "inline-script"
	flagScriptFile   = "script-file"
)
