package vmformat

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"kevwargo/ec2-playground/internal/session"
)

type ssmCache struct {
	session        *session.Regional
	client         *ssm.Client
	instances      map[string]types.InstanceInformation
	lastToken      *string
	paginationDone bool
}

func initSSMCache(session *session.Regional) *ssmCache {
	if os.Getenv("DISABLE_SSM_CACHE") == "1" {
		return nil
	}

	return &ssmCache{
		session:   session,
		client:    session.SSM(),
		instances: make(map[string]types.InstanceInformation),
	}
}

func (c *ssmCache) resolve(ctx context.Context, instanceID string) (*types.InstanceInformation, error) {
	if instance, found := c.instances[instanceID]; found {
		return &instance, nil
	}

	if c.paginationDone {
		return nil, nil
	}

	input := ssm.DescribeInstanceInformationInput{
		Filters: []types.InstanceInformationStringFilter{
			{
				Key:    aws.String(string(types.InstanceInformationFilterKeyResourceType)),
				Values: []string{string(types.ResourceTypeEc2Instance)},
			},
		},
		NextToken: c.lastToken,
	}

	for !c.paginationDone {
		resp, err := c.client.DescribeInstanceInformation(ctx, &input)
		if err != nil {
			return nil, err
		}

		input.NextToken = resp.NextToken
		c.lastToken = resp.NextToken
		c.paginationDone = resp.NextToken == nil

		for _, instance := range resp.InstanceInformationList {
			c.instances[*instance.InstanceId] = instance
		}

		if instance, found := c.instances[instanceID]; found {
			return &instance, nil
		}
	}

	return nil, nil
}
