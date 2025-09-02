package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	var p parameters

	cmd := &cobra.Command{
		Use: "s3",
	}

	cmd.Flags().StringVarP(&p.bucket, "bucket", "b", "", "A custom bucket to use, defaults to the one created by EC2Playground infra")

	cmd.AddCommand(
		&cobra.Command{
			Use:  "get-upload-url",
			Args: cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return sess.Run(c.Context(), func(ctx context.Context, r *session.Regional) error {
					return upload(ctx, p.withSession(r).withFilename(args[0]))
				})
			},
		},
	)

	return cmd
}

func upload(ctx context.Context, p parameters) error {
	presigner := s3.NewPresignClient(p.session.S3())

	bucket := p.bucket
	if bucket == "" {
		infraFetcher := infra.NewFetcher(p.session, config.InfraConfig{
			StackName:  infra.DefaultStackName,
			SkipDeploy: true,
		})
		resources, err := infraFetcher.Fetch(ctx)
		if err != nil {
			return err
		}

		bucket = resources.Bucket
	}

	req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &p.filename,
	}, func(po *s3.PresignOptions) {
		po.Expires = time.Hour
	})
	if err != nil {
		return err
	}

	fmt.Printf("Invoke-WebRequest -Method %s -Uri '%s' -InFile\n", req.Method, req.URL)
	fmt.Printf("aws s3 cp s3://%s/%s\n", bucket, p.filename)

	return nil
}
