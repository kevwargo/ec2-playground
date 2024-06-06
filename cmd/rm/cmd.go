package rm

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmstate"
)

func Command(sess *session.Global) *cobra.Command {
	cmd := vmstate.BuildCommand(
		sess,
		func(ctx context.Context, client *ec2.Client, ids []string) ([]types.InstanceStateChange, error) {
			resp, err := client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: ids})
			if err != nil {
				return nil, err
			}

			return resp.TerminatingInstances, nil
		},
	)

	cmd.Use = "rm"
	cmd.Short = "Terminate EC2 instances"

	return cmd
}
