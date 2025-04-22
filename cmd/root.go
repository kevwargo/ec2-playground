package cmd

import (
	"errors"

	"github.com/aws/smithy-go"
	"github.com/spf13/cobra"

	dumpinfra "kevwargo/ec2-playground/cmd/dump-infra"
	"kevwargo/ec2-playground/cmd/exec"
	"kevwargo/ec2-playground/cmd/images"
	"kevwargo/ec2-playground/cmd/ls"
	"kevwargo/ec2-playground/cmd/rdp"
	"kevwargo/ec2-playground/cmd/rm"
	"kevwargo/ec2-playground/cmd/run"
	"kevwargo/ec2-playground/cmd/ssh"
	"kevwargo/ec2-playground/cmd/start"
	"kevwargo/ec2-playground/cmd/stop"
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
	rootCmd.AddCommand(run.Command(sess))
	rootCmd.AddCommand(ls.Command(sess))
	rootCmd.AddCommand(rm.Command(sess))
	rootCmd.AddCommand(start.Command(sess))
	rootCmd.AddCommand(stop.Command(sess))
	rootCmd.AddCommand(rdp.Command(sess))
	rootCmd.AddCommand(images.Command(sess))
	rootCmd.AddCommand(dumpinfra.Command(sess))
	rootCmd.AddCommand(exec.Command())
	rootCmd.AddCommand(ssh.Command())
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
