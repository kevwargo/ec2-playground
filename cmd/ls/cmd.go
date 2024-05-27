package ls

import (
	"context"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/lister"
	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmformat"
)

func Command(sess *session.Global) *cobra.Command {
	var dumpFormat config.VMFormat

	cmd := &cobra.Command{
		Use: "ls",
		RunE: func(c *cobra.Command, _ []string) error {
			tmpl := dumpFormat.Template()

			return sess.Run(c.Context(), func(ctx context.Context, sess *session.Regional) error {
				l := lister.New(sess, tmpl)

				vms, err := l.ListVMs(ctx)
				if err != nil {
					return err
				}

				for _, vm := range vms {
					vmformat.Print(vm)
				}

				return nil
			})
		},
	}

	cmd.Flags().VarP(&dumpFormat, "format", "f", "Instance format")

	return cmd
}
