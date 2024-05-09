package lister

import (
	"context"
	"errors"
	"log"
	"os"
	"slices"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go"

	"kevwargo/ec2-playground/internal/format"
)

type InstanceLister struct {
	ec2Client *ec2.Client
	formatter format.Formatter
	printer   *log.Logger
}

func New(cfg aws.Config, formatTemplate *template.Template) InstanceLister {
	ec2Client := ec2.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)
	formatter := format.New(cfg.Region, ec2Client, ssmClient, formatTemplate)

	return InstanceLister{
		ec2Client: ec2Client,
		formatter: formatter,
		printer:   log.New(os.Stdout, "", 0),
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
				formatted, err := l.formatter.Format(ctx, instance)
				if err != nil {
					return err
				}

				l.printer.Println(formatted)
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
