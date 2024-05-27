package vmformat

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type instanceData struct {
	Id     string
	Type   string
	State  string
	Region string
	I      types.Instance

	ctx context.Context

	tags map[string]string

	ssm     *ssm.Client
	ssmInfo *ssmtypes.InstanceInformation
}

func (f Formatter) prepareInstanceData(ctx context.Context, instance types.Instance) *instanceData {
	return &instanceData{
		Id:     *instance.InstanceId,
		Type:   string(instance.InstanceType),
		State:  string(instance.State.Name),
		Region: f.session.Region,
		I:      instance,

		ctx: ctx,
		ssm: f.session.SSM(),
	}
}

func (i *instanceData) Name() string {
	return i.Tags()["Name"]
}

type tags map[string]string

func (t tags) String() string {
	pairs := make([]string, 0, len(t))
	for key, value := range t {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}

	slices.Sort(pairs)
	return strings.Join(pairs, " ")
}

func (i *instanceData) Tags() tags {
	if i.tags == nil {
		i.tags = make(tags, len(i.I.Tags))
		for _, t := range i.I.Tags {
			i.tags[*t.Key] = *t.Value
		}
	}

	return i.tags
}

func (i *instanceData) SSM() (*ssmtypes.InstanceInformation, error) {
	if i.ssmInfo != nil {
		return i.ssmInfo, nil
	}

	resp, err := i.ssm.DescribeInstanceInformation(i.ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{
				Key:    aws.String(string(ssmtypes.InstanceInformationFilterKeyInstanceIds)),
				Values: []string{i.Id},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.InstanceInformationList) == 0 {
		return nil, nil
	}

	i.ssmInfo = &resp.InstanceInformationList[0]

	return i.ssmInfo, nil
}

func (i *instanceData) Ping() (string, error) {
	ssmInfo, err := i.SSM()
	if err != nil {
		return "", err
	}

	if ssmInfo == nil {
		return "", nil
	}

	return string(ssmInfo.PingStatus), nil
}
