package format

import (
	"bytes"
	"context"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

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

func (f Formatter) Format(ctx context.Context, instance types.Instance) (string, error) {
	tags := make(map[string]string, len(instance.Tags))
	for _, tag := range instance.Tags {
		tags[*tag.Key] = *tag.Value
	}

	var buf bytes.Buffer
	err := f.tmpl.Execute(&buf, instanceData{
		Id:     *instance.InstanceId,
		Name:   tags["Name"],
		Region: f.region,
		I:      instance,

		ctx: ctx,
		ssm: f.ssm,
	})
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
