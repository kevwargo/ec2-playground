package ls

import (
	"context"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/lister"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	var dumpFormat config.VMFormat
	var matchNames []string

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List existing EC2 instances",
		RunE: func(c *cobra.Command, _ []string) error {
			tmpl := dumpFormat.Template()

			return sess.Run(c.Context(), func(ctx context.Context, sess *session.Regional) error {
				l := lister.New(sess, tmpl)

				vms, err := l.ListVMs(ctx, matchNames)
				if err != nil {
					return err
				}

				for _, vm := range vms {
					sess.Print(vm.String())
				}

				return nil
			})
		},
	}

	cmd.Flags().VarP(&dumpFormat, "format", "f", "Instance format")
	cmd.Flags().StringArrayVarP(&matchNames, "name-filter", "N", nil, "Simple case-insensitive matching by instance name")

	return cmd
}
