package stop

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmstate"
)

func Command(sess *session.Global) *cobra.Command {
	cmd := vmstate.BuildCommand(sess, execute)

	cmd.Use = "stop"
	cmd.Short = "Stop EC2 instances"

	return cmd
}

func execute(ctx context.Context, sess *session.Regional, ids []string) ([]types.InstanceStateChange, error) {
	resp, err := sess.EC2().StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: ids})
	if err != nil {
		return nil, err
	}

	return resp.StoppingInstances, nil
}
