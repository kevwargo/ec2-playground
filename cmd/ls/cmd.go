package ls

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use: "ls",
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()

			cfg, err := config.LoadDefaultConfig(ctx)
			if err != nil {
				return err
			}

			client := ec2.NewFromConfig(cfg)
			paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
			for paginator.HasMorePages() {
				page, err := paginator.NextPage(ctx)
				if err != nil {
					return err
				}

				for _, r := range page.Reservations {
					for _, i := range r.Instances {
						printInstance(i)
					}
				}
			}

			return nil
		},
	}
}

func printInstance(i types.Instance) {
	tags := make(map[string]string, len(i.Tags))
	for _, t := range i.Tags {
		tags[*t.Key] = *t.Value
	}

	name := tags["Name"]

	fmt.Printf("%s %s %s\n", *i.InstanceId, name, i.State.Name)
}
