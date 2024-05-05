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
	"kevwargo/ec2-playground/internal/session"
)

func Execute() {
	var cfg session.Config
	var sess session.Session

	rootCmd := &cobra.Command{
		Use:           "ec2",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session.New(cmd.Context(), &cfg)
			if err != nil {
				return err
			}

			sess = s
			return nil
		},
	}

	rootCmd.PersistentFlags().StringSliceVarP(&cfg.Regions, "regions", "r", nil, "List of regions, comma-separated")

	rootCmd.AddCommand(run.Command(&sess))
	rootCmd.AddCommand(ls.Command(&sess))
	rootCmd.AddCommand(exec.Command())
	rootCmd.AddCommand(rm.Command())
	rootCmd.AddCommand(start.Command())
	rootCmd.AddCommand(stop.Command())
	rootCmd.AddCommand(ssh.Command())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
