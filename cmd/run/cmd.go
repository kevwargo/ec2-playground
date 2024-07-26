package run

import (
	"context"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/runner"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	var cfg config.RunConfig

	cmd := &cobra.Command{
		Use:           "run image1 [image2...]",
		Short:         "Run new EC2 instances",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, images []string) error {
			return sess.Run(c.Context(), func(ctx context.Context, sess *session.Regional) error {
				cfg.Images = images
				return runner.New(cfg, sess).RunInstances(ctx)
			})
		},
	}

	f := cmd.Flags()

	f.StringVarP(&cfg.Type, "type", "t", "t3.micro", "Instance type")

	f.StringVarP(&cfg.Name, "name", "n", "", "Instance name (the value for the 'Name' tag)")
	f.StringArrayVarP(&cfg.Tags, "tags", "T", nil, "A 'key=value' pairs of tags")

	f.StringVar(&cfg.Profile, "profile", "", "The name or ARN of an IAM instance profile")
	f.StringArrayVar(&cfg.Policies, "policy", nil, "The ARN of an IAM ManagedPolicy which will be attached to the instance's profile. Can be supplied multiple times to attach multiple policies.")

	f.StringVarP(&cfg.KeyPair, flagKeyPair, "k", "", "Existing EC2 key pair")
	f.StringVarP(&cfg.SSHPublicKeyFile, flagSSHPublicKeyFile, "s", "", "A path to the SSH public key file which will be imported and attached to the instance")
	f.BoolVar(&cfg.SkipPublicIPv4, "skip-public-ipv4", false, "Don't assign an IPv4 address to the instance")
	cmd.MarkFlagsMutuallyExclusive(flagKeyPair, flagSSHPublicKeyFile)

	f.StringVar(&cfg.InfraStackName, "infra-stack", infra.DefaultStackName, "Infra stack name")
	f.BoolVar(&cfg.SkipInfraDeploy, "skip-infra-deploy", false, "Don't attempt to deploy the infra stack")

	f.VarP(&cfg.DumpFormat, "dump-format", "f", "Format for printing new instances")
	f.BoolVarP(&cfg.DryRun, "dry-run", "d", false, "Dry run operation")

	return cmd
}

const (
	flagKeyPair          = "key-pair"
	flagSSHPublicKeyFile = "ssh-public-key-file"
)
