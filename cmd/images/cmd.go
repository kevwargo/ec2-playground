package images

import (
	"context"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/images"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	var (
		includeDeprecated bool
		includeDisabled   bool
	)

	cmd := &cobra.Command{
		Use:   "images NAME",
		Short: "List matching EC2 images",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sess.Run(cmd.Context(), func(ctx context.Context, r *session.Regional) error {
				resolver := images.NewResolver(r.SSM(), r.EC2())
				resolvedImages, err := resolver.ResolveWithDetails(ctx, images.ResolveInput{
					Patterns:          args,
					IncludeDeprecated: includeDeprecated,
					IncludeDisabled:   includeDisabled,
				})
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

	cmd.Flags().BoolVar(&includeDeprecated, "deprecated", false, "Include deprecated images when searching by name")
	cmd.Flags().BoolVar(&includeDisabled, "disabled", false, "Include disabled images when searching by name")

	return cmd
}
