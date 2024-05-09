package ls

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/lister"
	"kevwargo/ec2-playground/internal/session"
)

var plain = log.New(os.Stdout, "", 0)

func Command(sess *session.Session) *cobra.Command {
	var dumpFormat config.TemplateFlag

	cmd := &cobra.Command{
		Use: "ls",
		RunE: func(c *cobra.Command, _ []string) error {
			return sess.Run(c.Context(), func(ctx context.Context, cfg aws.Config) error {
				return lister.New(cfg, dumpFormat.Template()).ListInstances(ctx)
			})
		},
	}

	cmd.Flags().VarP(&dumpFormat, "format", "f", "Instance format")

	return cmd
}
