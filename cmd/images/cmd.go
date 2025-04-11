package images

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	return &cobra.Command{
		Use:   "images NAME",
		Short: "List matching public images",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sess.Run(cmd.Context(), func(ctx context.Context, r *session.Regional) error {
				return run(ctx, r, args[0])
			})
		},
	}
}

func run(ctx context.Context, sess *session.Regional, name string) error {
	paginator := ec2.NewDescribeImagesPaginator(sess.EC2(), &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
			{
				Name:   aws.String("virtualization-type"),
				Values: []string{"hvm"},
			},
			{
				Name:   aws.String("name"),
				Values: []string{name},
			},
		},
		ExecutableUsers: []string{"all"},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, image := range page.Images {
			sess.Print("%s %s %s", sess.Region, *image.ImageId, *image.Name)
		}
	}

	return nil
}
