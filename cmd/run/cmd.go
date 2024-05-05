package run

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Session) *cobra.Command {
	var stackName string
	var skipDeploy bool

	cmd := &cobra.Command{
		Use:           "run",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, _ []string) error {
			return sess.Run(c.Context(), func(ctx context.Context, cfg aws.Config) error {
				return runInstances(ctx, infra.NewFetcher(cfg, stackName, skipDeploy))
			})
		},
	}

	cmd.Flags().StringVar(&stackName, "infra-stack", infra.DefaultStackName, "Infra stack name")
	cmd.Flags().BoolVar(&skipDeploy, "skip-infra-deploy", false, "Don't attempt to deploy the infra stack")

	return cmd
}

func runInstances(ctx context.Context, infraFetcher infra.Fetcher) error {
	resources, err := infraFetcher.Fetch(ctx)
	if err != nil {
		return err
	}

	log.Printf("infra: %+v", resources)
	return nil
}
