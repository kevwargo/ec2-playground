package infracmd

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
	cmd := &cobra.Command{
		Use:           "infra",
		Short:         "A set of commands to view/manage CloudFormation infrastructure",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(createBasicCmd(
		sess,
		config.InfraConfig{SkipDeploy: true},
		"dump",
		"Show currently deployed state of helper resources",
	))
	cmd.AddCommand(createBasicCmd(
		sess,
		config.InfraConfig{SkipDeploy: false},
		"deploy",
		"Deploy the CloudFormation stack with helper resources",
	))

	return cmd
}

func createBasicCmd(sess *session.Global, cfg config.InfraConfig, name, description string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           name,
		Short:         description,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return sess.Run(cmd.Context(), func(ctx context.Context, r *session.Regional) error {
				f := infra.NewFetcher(r, cfg)
				res, err := f.Fetch(ctx)
				if err != nil {
					return err
				}

				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			})
		},
	}

	cmd.Flags().StringVarP(&cfg.StackName, "stack-name", "n", infra.DefaultStackName, "Infra stack name")

	return cmd
}
