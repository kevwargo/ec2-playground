package lister

import (
	"context"
	"errors"
	"slices"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go"

	"kevwargo/ec2-playground/internal/vmformat"
)

type VMLister struct {
	ec2Client *ec2.Client
	formatter vmformat.Formatter
}

func New(cfg aws.Config, formatTemplate *template.Template) VMLister {
	ec2Client := ec2.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)
	formatter := vmformat.New(cfg.Region, ec2Client, ssmClient, formatTemplate)

	return VMLister{
		ec2Client: ec2Client,
		formatter: formatter,
	}
}

func (l VMLister) ListVMs(ctx context.Context) ([]vmformat.VM, error) {
	var vms []vmformat.VM
	paginator := ec2.NewDescribeInstancesPaginator(l.ec2Client, &ec2.DescribeInstancesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ignoreErrorCodes(err, "UnauthorizedOperation", "AuthFailure")
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

func ignoreErrorCodes(err error, codes ...string) error {
	if ae := smithy.APIError(nil); errors.As(err, &ae) {
		if c := ae.ErrorCode(); slices.Contains(codes, c) {
			return nil
		}
	}

	return err
}
