package tag

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmstate"
)

var (
	tagsSet    []string
	tagsDelete []string
)

func Command(sess *session.Global) *cobra.Command {
	cmd := vmstate.BuildCommand(sess, apply)

	cmd.Use = "tag"
	cmd.Short = "Set or delete tags of EC2 instances"

	cmd.Flags().StringArrayVarP(&tagsSet, "set", "s", nil, "Tags to set")
	cmd.Flags().StringArrayVarP(&tagsDelete, "delete", "d", nil, "Tags to delete")

	return cmd
}

func apply(ctx context.Context, sess *session.Regional, ids []string) ([]types.InstanceStateChange, error) {
	var (
		ec2Client = sess.EC2()
		errs      []error
	)

	if len(tagsSet) > 0 {
		_, err := ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
			Resources: ids,
			Tags:      convertTagsSet(tagsSet),
		})
		errs = append(errs, err)
	}

	if len(tagsDelete) > 0 {
		_, err := ec2Client.DeleteTags(ctx, &ec2.DeleteTagsInput{
			Resources: ids,
			Tags:      convertTagsDelete(tagsDelete),
		})
		errs = append(errs, err)
	}

	return nil, errors.Join(errs...)
}

func convertTagsSet(tagsSet []string) (tags []types.Tag) {
	for _, expr := range tagsSet {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 {
			continue
		}

		tags = append(tags, types.Tag{Key: &parts[0], Value: &parts[1]})
	}

	return tags
}

func convertTagsDelete(tagsDelete []string) (tags []types.Tag) {
	for _, expr := range tagsDelete {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) == 1 {
			tags = append(tags, types.Tag{Key: &expr})
		} else {
			tags = append(tags, types.Tag{Key: &parts[0], Value: &parts[1]})
		}
	}

	return tags
}
