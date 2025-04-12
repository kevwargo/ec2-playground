package lister

import (
	"context"
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

func (l VMLister) ListVMs(ctx context.Context) ([]vmformat.VM, error) {
	var vms []vmformat.VM
	paginator := ec2.NewDescribeInstancesPaginator(l.session.EC2(), &ec2.DescribeInstancesInput{})

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
