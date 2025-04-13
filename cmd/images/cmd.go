package images

import (
	"context"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/images"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	return &cobra.Command{
		Use:   "images NAME",
		Short: "List matching public images",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sess.Run(cmd.Context(), func(ctx context.Context, r *session.Regional) error {
				resolver := images.NewResolver(r.SSM(), r.EC2())
				resolvedImages, err := resolver.ResolveWithDetails(ctx, args)
				if err != nil {
					return err
				}

				for _, img := range resolvedImages {
					r.Print("%s %s %s", r.Region, img.ID, *img.Details.Name)
				}

				return nil
			})
		},
	}
}
