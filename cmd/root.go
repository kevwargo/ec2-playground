package cmd

import (
	"errors"
	"os"

	"github.com/aws/smithy-go"
	"github.com/spf13/cobra"

	dumpinfra "kevwargo/ec2-playground/cmd/dump-infra"
	"kevwargo/ec2-playground/cmd/exec"
	"kevwargo/ec2-playground/cmd/images"
	"kevwargo/ec2-playground/cmd/ls"
	"kevwargo/ec2-playground/cmd/rdp"
	"kevwargo/ec2-playground/cmd/rm"
	"kevwargo/ec2-playground/cmd/run"
	"kevwargo/ec2-playground/cmd/s3"
	"kevwargo/ec2-playground/cmd/ssh"
	"kevwargo/ec2-playground/cmd/start"
	"kevwargo/ec2-playground/cmd/stop"
	"kevwargo/ec2-playground/cmd/tag"
	"kevwargo/ec2-playground/internal/session"
)

func Execute() error {
	var (
		sess               session.Global
		ignoreAccessErrors bool
	)

	rootCmd := &cobra.Command{
		Use:           "ec2",
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	rootCmd.PersistentFlags().StringSliceVarP(&sess.Regions, "regions", "r", nil, "List of regions, comma-separated")
	rootCmd.PersistentFlags().BoolVarP(&ignoreAccessErrors, "ignore-access-errors", "X", false, "Silently ignore AWS API errors originating from insufficient permissions")

	addCommands(rootCmd, &sess)

	err := rootCmd.Execute()
	if ignoreAccessErrors && err != nil {
		err = ignoreError(err)
	}

	return err
}

func addCommands(rootCmd *cobra.Command, sess *session.Global) {
	rootCmd.AddCommand(
		run.Command(sess),
		ls.Command(sess),
		rm.Command(sess),
		start.Command(sess),
		stop.Command(sess),
		tag.Command(sess),
		rdp.Command(sess),
		images.Command(sess),
		dumpinfra.Command(sess),
		exec.Command(sess),
		ssh.Command(),
		s3.Command(sess),
	)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "bash_completion",
		Short: "Generate Bash-completion script",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Root().GenBashCompletion(os.Stdout)
		},
	})
}

func ignoreError(err error) error {
	var (
		opErr  *smithy.OperationError
		apiErr smithy.APIError
	)

	if !(errors.As(err, &opErr) && errors.As(err, &apiErr)) {
		return err
	}

	for _, ignored := range ignoredAccessErrors {
		if opErr.ServiceID == ignored.service && apiErr.ErrorCode() == ignored.errorCode {
			return nil
		}
	}

	return err
}

type awsErrorPattern struct {
	service   string
	errorCode string
}

var ignoredAccessErrors = []awsErrorPattern{
	{"EC2", "UnauthorizedOperation"},
	{"EC2", "AuthFailure"},
	{"CloudFormation", "AccessDenied"},
	{"SSM", "AccessDeniedException"},
}
