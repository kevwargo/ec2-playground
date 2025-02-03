package dumpinfra

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	var cfg config.InfraConfig

	cmd := &cobra.Command{
		Use:           "dump-infra",
		Short:         "Dump CFN infra used for running new instances in JSON format",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return sess.Run(cmd.Context(), func(ctx context.Context, r *session.Regional) error {
				return execute(ctx, r, cfg)
			})
		},
	}

	cmd.Flags().StringVarP(&cfg.StackName, "stack-name", "n", infra.DefaultStackName, "Infra stack name")
	cmd.Flags().BoolVarP(&cfg.SkipDeploy, "skip-deploy", "s", true, "Don't attempt to deploy the infra stack")

	return cmd
}

func execute(ctx context.Context, sess *session.Regional, cfg config.InfraConfig) error {
	f := infra.NewFetcher(sess, cfg)
	res, err := f.Fetch(ctx)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}
