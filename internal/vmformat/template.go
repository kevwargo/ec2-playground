package vmformat

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type instanceData struct {
	Id     string
	Type   string
	Name   string
	State  string
	Region string
	I      types.Instance

	ctx     context.Context
	ssm     *ssm.Client
	ssmInfo *ssmtypes.InstanceInformation
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
