package vmformat

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type instanceData struct {
	I      types.Instance
	Id     string
	Type   string
	State  string
	Region string
	Tags   tags

	ctx context.Context

	ssm      *ssm.Client
	ec2      *ec2.Client
	ssmCache *ssmCache
	ssmInfo  *ssmtypes.InstanceInformation
	amiCache map[string]types.Image
}

func (f Formatter) prepareInstanceData(ctx context.Context, instance types.Instance) *instanceData {
	data := &instanceData{
		I:      instance,
		Id:     *instance.InstanceId,
		Type:   string(instance.InstanceType),
		State:  string(instance.State.Name),
		Region: f.session.Region,
		Tags:   make(tags, len(instance.Tags)),

		ctx:      ctx,
		ssm:      f.session.SSM(),
		ec2:      f.session.EC2(),
		ssmCache: f.ssmCache,
		amiCache: f.amiCache,
	}
	for _, t := range instance.Tags {
		data.Tags[*t.Key] = *t.Value
	}

	return data
}

func (i *instanceData) Name() string {
	return i.Tags["Name"]
}

func (i *instanceData) Image() (types.Image, error) {
	imageID := *i.I.ImageId

	if cached, ok := i.amiCache[imageID]; ok {
		return cached, nil
	}

	resp, err := i.ec2.DescribeImages(i.ctx, &ec2.DescribeImagesInput{ImageIds: []string{imageID}})
	if err != nil {
		return types.Image{}, err
	}
	if len(resp.Images) == 0 {
		return types.Image{}, fmt.Errorf("AMI %s not found in %s", imageID, i.Region)
	}

	i.amiCache[imageID] = resp.Images[0]

	return resp.Images[0], nil
}

func (i *instanceData) ImageName() (string, error) {
	img, err := i.Image()
	if err != nil {
		return "", err
	}

	return *img.Name, nil
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

func (i *instanceData) SSM() (*ssmtypes.InstanceInformation, error) {
	if i.ssmCache != nil {
		return i.ssmCache.resolve(i.ctx, i.Id)
	}

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
