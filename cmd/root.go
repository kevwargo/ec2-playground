package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/cmd/exec"
	"kevwargo/ec2-playground/cmd/ls"
	"kevwargo/ec2-playground/cmd/rm"
	"kevwargo/ec2-playground/cmd/run"
	"kevwargo/ec2-playground/cmd/ssh"
	"kevwargo/ec2-playground/cmd/start"
	"kevwargo/ec2-playground/cmd/stop"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:           "ec2",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.AddCommand(run.Command())
	rootCmd.AddCommand(ls.Command())
	rootCmd.AddCommand(exec.Command())
	rootCmd.AddCommand(rm.Command())
	rootCmd.AddCommand(start.Command())
	rootCmd.AddCommand(stop.Command())
	rootCmd.AddCommand(ssh.Command())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
