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

	"kevwargo/ec2-playground/internal/format"
)

type InstanceLister struct {
	ec2Client *ec2.Client
	formatter format.Formatter
}

func New(cfg aws.Config, formatTemplate *template.Template) InstanceLister {
	ec2Client := ec2.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)
	formatter := format.New(cfg.Region, ec2Client, ssmClient, formatTemplate)

	return InstanceLister{
		ec2Client: ec2Client,
		formatter: formatter,
	}
}

func (l InstanceLister) ListInstances(ctx context.Context) error {
	paginator := ec2.NewDescribeInstancesPaginator(l.ec2Client, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return ignoreErrorCodes(err, "UnauthorizedOperation", "AuthFailure")
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if instance.State.Name == types.InstanceStateNameTerminated {
					continue
				}

				if err := l.formatter.Print(ctx, instance); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func ignoreErrorCodes(err error, codes ...string) error {
	if ae := smithy.APIError(nil); errors.As(err, &ae) {
		if c := ae.ErrorCode(); slices.Contains(codes, c) {
			return nil
		}
	}

	return err
}
