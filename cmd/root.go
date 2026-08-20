package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/cmd/download"
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
	var sess session.Global

	rootCmd := &cobra.Command{
		Use:           "ec2",
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	rootCmd.PersistentFlags().StringSliceVarP(&sess.Regions, "regions", "r", nil, "List of regions, comma-separated")
	rootCmd.PersistentFlags().StringSliceVarP(
		&sess.ExcludeRegions,
		"exclude-regions",
		"R",
		nil,
		"Comma-separated list of regions to exclude, automatically enables '-r all'",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&sess.IgnoreAccessErrors,
		"ignore-access-errors",
		"X",
		false,
		"Silently ignore AWS API errors originating from insufficient permissions",
	)
	rootCmd.PersistentFlags().IntVar(&sess.HTTPTimeoutSeconds, "timeout", 0, "HTTP request timeout in seconds")
	rootCmd.PersistentFlags().BoolVarP(
		&sess.SkipConnErrRetry,
		"no-retry-conn-err",
		"E",
		false,
		"Don't retry network connection errors",
	)

	addCommands(rootCmd, &sess)

	return rootCmd.Execute()
}

func addCommands(rootCmd *cobra.Command, sess *session.Global) {
	cmds := []*cobra.Command{
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
		download.Command(sess),
		ssh.Command(),
		s3.Command(sess),
	}

	rootCmd.AddCommand(cmds...)

	for _, c := range cmds {
		// Catch flag shorthand conflict early for all commands
		c.InheritedFlags()
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "bash_completion",
		Short: "Generate Bash-completion script",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Root().GenBashCompletion(os.Stdout)
		},
	})
}
