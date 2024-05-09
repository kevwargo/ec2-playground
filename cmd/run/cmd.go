package run

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/runner"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Session) *cobra.Command {
	var runCfg config.RunConfig

	cmd := &cobra.Command{
		Use:           "run image1 [image2...]",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, images []string) error {
			return sess.Run(c.Context(), func(ctx context.Context, awsCfg aws.Config) error {
				runCfg.Images = images
				return runner.New(awsCfg, runCfg, sess).RunInstances(ctx)
			})
		},
	}

	f := cmd.Flags()

	f.StringVarP(&runCfg.Type, "type", "t", "t3.micro", "Instance type")

	f.StringVarP(&runCfg.Name, "name", "n", "", "Instance name (the value for the 'Name' tag)")
	f.StringArrayVarP(&runCfg.Tags, "tags", "T", nil, "A 'key=value' pairs of tags")

	f.StringVar(&runCfg.Profile, "profile", "", "The name or ARN of an IAM instance profile")
	f.StringVar(&runCfg.Policy, "policy", "", "The ARN of an IAM ManagedPolicy which will be attached to the instance's profile")

	f.StringVarP(&runCfg.KeyPair, flagKeyPair, "k", "", "Existing EC2 key pair")
	f.StringVarP(&runCfg.SSHPublicKeyFile, flagSSHPublicKeyFile, "s", "", "A path to the SSH public key file which will be imported and attached to the instance")
	f.BoolVar(&runCfg.SkipPublicIPv4, "skip-public-ipv4", false, "Don't assign an IPv4 address to the instance")
	cmd.MarkFlagsMutuallyExclusive(flagKeyPair, flagSSHPublicKeyFile)

	f.StringVarP(&runCfg.UserData, "user-data", "U", "", "A file which will be used as user data")

	f.StringVar(&runCfg.InfraStackName, "infra-stack", infra.DefaultStackName, "Infra stack name")
	f.BoolVar(&runCfg.SkipInfraDeploy, "skip-infra-deploy", false, "Don't attempt to deploy the infra stack")

	f.VarP(&runCfg.DumpFormat, "dump-format", "f", "Format for printing new instances")
	f.BoolVarP(&runCfg.DryRun, "dry-run", "d", false, "Dry run operation")

	return cmd
}

const (
	flagKeyPair          = "key-pair"
	flagSSHPublicKeyFile = "ssh-public-key-file"
)
