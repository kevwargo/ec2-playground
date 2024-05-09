package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/format"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

type InstanceRunner struct {
	cfg          config.RunConfig
	ec2          *ec2.Client
	sess         *session.Session
	infraFetcher infra.Fetcher
	formatter    format.Formatter
	printer      *log.Logger
}

func New(awsCfg aws.Config, runCfg config.RunConfig, sess *session.Session) InstanceRunner {
	ec2Client := ec2.NewFromConfig(awsCfg)
	ssmClient := ssm.NewFromConfig(awsCfg)

	return InstanceRunner{
		cfg:          runCfg,
		ec2:          ec2Client,
		sess:         sess,
		infraFetcher: infra.NewFetcher(awsCfg, runCfg),
		formatter:    format.New(awsCfg.Region, ec2Client, ssmClient, runCfg.DumpFormat.Template()),
		printer:      log.New(os.Stdout, "", 0),
	}
}

func (r InstanceRunner) RunInstances(ctx context.Context) error {
	resources, err := r.infraFetcher.Fetch(ctx)
	if err != nil {
		return err
	}

	inputs, err := r.buildInputs(ctx, resources)
	if err != nil {
		return err
	}

	for idx := range inputs {
		resp, err := r.ec2.RunInstances(ctx, &inputs[idx])
		if err != nil {
			return err
		}

		for _, instance := range resp.Instances {
			formatted, err := r.formatter.Format(ctx, instance)
			if err != nil {
				return err
			}

			r.printer.Println(formatted)
		}
	}

	return nil
}

func (r InstanceRunner) buildInputs(ctx context.Context, resources infra.Resources) ([]ec2.RunInstancesInput, error) {
	in := r.createBasicInput(resources)

	if err := r.setTags(&in); err != nil {
		return nil, err
	}

	if err := r.setProfile(ctx, &in, resources); err != nil {
		return nil, err
	}

	var inputs []ec2.RunInstancesInput
	for idx := range r.cfg.Images {
		in.ImageId = &r.cfg.Images[idx]
		inputs = append(inputs, in)
	}

	return inputs, nil
}

func (r InstanceRunner) createBasicInput(resources infra.Resources) ec2.RunInstancesInput {
	in := ec2.RunInstancesInput{
		InstanceType: types.InstanceType(r.cfg.Type),
		DryRun:       aws.Bool(r.cfg.DryRun),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		MetadataOptions: &types.InstanceMetadataOptionsRequest{
			InstanceMetadataTags: types.InstanceMetadataTagsStateEnabled,
		},
	}

	if resources.Subnet != "" {
		in.NetworkInterfaces = []types.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int32(0),
				Groups:                   []string{resources.SecurityGroup},
				AssociatePublicIpAddress: aws.Bool(!r.cfg.SkipPublicIPv4),
				SubnetId:                 &resources.Subnet,
			},
		}
	} else {
		in.SecurityGroupIds = []string{resources.SecurityGroup}
	}

	return in
}

func (r InstanceRunner) setTags(in *ec2.RunInstancesInput) error {
	tagsMap := make(map[string]string)

	for _, expr := range r.cfg.Tags {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid tag specification: %q", expr)
		}

		tagsMap[parts[0]] = parts[1]
	}

	if r.cfg.Name != "" {
		tagsMap["Name"] = r.cfg.Name
	}

	if len(tagsMap) == 0 {
		return nil
	}

	tags := make([]types.Tag, 0, len(tagsMap))
	for k, v := range tagsMap {
		tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}

	in.TagSpecifications = []types.TagSpecification{
		{
			ResourceType: types.ResourceTypeInstance,
			Tags:         tags,
		},
	}

	return nil
}

func (r InstanceRunner) setProfile(ctx context.Context, in *ec2.RunInstancesInput, resources infra.Resources) error {
	var profile string

	if r.cfg.Profile != "" {
		profile = r.cfg.Profile
	} else if r.cfg.Policy != "" {
		err := r.sess.RunIAM(ctx, func(ctx context.Context, iamClient *iam.Client) error {
			builder := profileBuilder{
				iam:           iamClient,
				userPolicy:    r.cfg.Policy,
				defaultPolicy: resources.InstancePolicy,
				infraName:     r.cfg.InfraStackName,
			}

			profileName, err := builder.buildProfile(ctx)
			if err == nil {
				profile = profileName
			}

			return err
		})
		if err != nil {
			return err
		}
	} else {
		profile = resources.InstanceProfile
	}

	in.IamInstanceProfile = &types.IamInstanceProfileSpecification{
		Name: &profile,
	}

	return nil
}
