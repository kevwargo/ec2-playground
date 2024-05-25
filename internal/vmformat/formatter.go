package vmformat

import (
	"bytes"
	"context"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type VM struct {
	types.Instance

	format string
}

func (v VM) String() string {
	return v.format
}

type Formatter struct {
	region string
	ec2    *ec2.Client
	ssm    *ssm.Client
	tmpl   *template.Template
}

func New(region string, ec2Client *ec2.Client, ssmClient *ssm.Client, tmpl *template.Template) Formatter {
	return Formatter{
		region: region,
		ec2:    ec2Client,
		ssm:    ssmClient,
		tmpl:   tmpl,
	}
}

func (f Formatter) Format(ctx context.Context, instance types.Instance) (VM, error) {
	var buf bytes.Buffer
	err := f.tmpl.Execute(&buf, f.prepareInstanceData(ctx, instance))
	if err != nil {
		return VM{}, err
	}

	return VM{
		Instance: instance,
		format:   buf.String(),
	}, nil
}
