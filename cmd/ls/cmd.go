package ls

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
)

var plain = log.New(os.Stdout, "", 0)

func Command(sess *session.Session) *cobra.Command {
	return &cobra.Command{
		Use: "ls",
		RunE: func(c *cobra.Command, _ []string) error {
			return sess.Run(c.Context(), listInstances)
		},
	}
}

func listInstances(ctx context.Context, cfg aws.Config) error {
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return ignoreErrorCodes(cfg, err, "UnauthorizedOperation", "AuthFailure")
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				printInstance(instance)
			}
		}
	}

	return nil
}

func ignoreErrorCodes(cfg aws.Config, err error, codes ...string) error {
	if ae := smithy.APIError(nil); errors.As(err, &ae) {
		if c := ae.ErrorCode(); slices.Contains(codes, c) {
			return nil
		}
	}

	return fmt.Errorf("%s: %w", cfg.Region, err)
}

func printInstance(instance types.Instance) {
	tags := make(map[string]string, len(instance.Tags))
	for _, tag := range instance.Tags {
		tags[*tag.Key] = *tag.Value
	}

	id := *instance.InstanceId
	az := *instance.Placement.AvailabilityZone
	name := tags["Name"]
	state := instance.State.Name

	plain.Printf("%s %s %s %s", az, id, name, state)
}
