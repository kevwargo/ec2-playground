package cmd

import (
	"log"

	"github.com/spf13/cobra"

	dumpinfra "kevwargo/ec2-playground/cmd/dump-infra"
	"kevwargo/ec2-playground/cmd/exec"
	"kevwargo/ec2-playground/cmd/ls"
	"kevwargo/ec2-playground/cmd/rm"
	"kevwargo/ec2-playground/cmd/run"
	"kevwargo/ec2-playground/cmd/ssh"
	"kevwargo/ec2-playground/cmd/start"
	"kevwargo/ec2-playground/cmd/stop"
	"kevwargo/ec2-playground/internal/session"
)

func Execute() {
	var sess session.Global

	rootCmd := &cobra.Command{
		Use:           "ec2",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().StringSliceVarP(&sess.Regions, "regions", "r", nil, "List of regions, comma-separated")

	rootCmd.AddCommand(run.Command(&sess))
	rootCmd.AddCommand(ls.Command(&sess))
	rootCmd.AddCommand(exec.Command())
	rootCmd.AddCommand(rm.Command(&sess))
	rootCmd.AddCommand(start.Command(&sess))
	rootCmd.AddCommand(stop.Command(&sess))
	rootCmd.AddCommand(ssh.Command())
	rootCmd.AddCommand(dumpinfra.Command(&sess))

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
