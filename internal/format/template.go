package format

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type instanceData struct {
	Id     string
	Name   string
	Region string
	I      types.Instance

	ctx         context.Context
	ssm         *ssm.Client
	ssmInstance *ssmtypes.InstanceInformation
}

func (i *instanceData) SSM() (*ssmtypes.InstanceInformation, error) {
	if i.ssmInstance != nil {
		return i.ssmInstance, nil
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
		return nil, fmt.Errorf("SSM instance %s not found", i.Id)
	}

	i.ssmInstance = &resp.InstanceInformationList[0]

	return i.ssmInstance, nil
}
