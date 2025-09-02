package lister

import (
	"context"
	"slices"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmformat"
)

type VMLister struct {
	session   *session.Regional
	formatter vmformat.Formatter
}

func New(sess *session.Regional, formatTemplate *template.Template) VMLister {
	formatter := vmformat.New(sess, formatTemplate)

	return VMLister{
		session:   sess,
		formatter: formatter,
	}
}

func (l VMLister) ListVMs(ctx context.Context, matchNames []string) ([]vmformat.VM, error) {
	var vms []vmformat.VM
	paginator := ec2.NewDescribeInstancesPaginator(l.session.EC2(), &ec2.DescribeInstancesInput{})

	filterFn := func(tags []types.Tag) bool { return true }
	if len(matchNames) > 0 {
		filterFn = func(tags []types.Tag) bool {
			return slices.ContainsFunc(tags, func(tag types.Tag) bool {
				return *tag.Key == "Name" && slices.ContainsFunc(matchNames, func(name string) bool {
					return strings.Contains(strings.ToLower(*tag.Value), strings.ToLower(name))
				})
			})
		}
	}

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if instance.State.Name == types.InstanceStateNameTerminated {
					continue
				}

				if !filterFn(instance.Tags) {
					continue
				}

				vm, err := l.formatter.Format(ctx, instance)
				if err != nil {
					return nil, err
				}

				vms = append(vms, vm)
			}
		}
	}

	return vms, nil
}
