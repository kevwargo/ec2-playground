package images

import (
	"context"

	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/images"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	var input images.ResolveInput

	cmd := &cobra.Command{
		Use:   "images NAME",
		Short: "List matching EC2 images",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sess.Run(cmd.Context(), func(ctx context.Context, r *session.Regional) error {
				input.Patterns = args
				resolver := images.NewResolver(r.SSM(), r.EC2())
				resolvedImages, err := resolver.ResolveWithDetails(ctx, input)
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

	cmd.Flags().BoolVar(&input.IncludeDeprecated, "deprecated", false, "Include deprecated images when searching by name")
	cmd.Flags().BoolVar(&input.IncludeDisabled, "disabled", false, "Include disabled images when searching by name")

	return cmd
}
