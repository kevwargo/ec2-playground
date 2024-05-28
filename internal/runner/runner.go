package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/images"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmformat"
)

type InstanceRunner struct {
	cfg           config.RunConfig
	sess          *session.Regional
	infraFetcher  infra.Fetcher
	formatter     vmformat.Formatter
	imageResolver images.Resolver
}

func New(cfg config.RunConfig, sess *session.Regional) InstanceRunner {
	return InstanceRunner{
		cfg:           cfg,
		sess:          sess,
		infraFetcher:  infra.NewFetcher(sess, cfg),
		formatter:     vmformat.New(sess, cfg.DumpFormat.Template()),
		imageResolver: images.NewResolver(sess.SSM()),
	}
}

func (r InstanceRunner) RunInstances(ctx context.Context) error {
	resources, err := r.infraFetcher.Fetch(ctx)
	if err != nil {
		return err
	}

	params, err := r.buildParams(ctx, resources)
	if err != nil {
		return err
	}

	errC := make(chan error)
	for _, input := range params.inputs {
		go func(in ec2.RunInstancesInput) {
			errC <- r.runInstances(ctx, in, params.waitForProfile)
		}(input)
	}

	errs := make([]error, len(params.inputs))
	for idx := range errs {
		errs[idx] = <-errC
	}

	return errors.Join(errs...)
}

type runParams struct {
	inputs         []ec2.RunInstancesInput
	waitForProfile bool
}

func (r InstanceRunner) runInstances(ctx context.Context, input ec2.RunInstancesInput, waitForProfile bool) error {
	resp, err := r.sess.EC2().RunInstances(ctx, &input)

	for waitForProfile && err != nil {
		var ae smithy.APIError
		if !errors.As(err, &ae) {
			return err
		}

		if ae.ErrorCode() != errProfileInvalidCode ||
			ae.ErrorMessage() != fmt.Sprintf(errProfileInvalidMsgTmpl, *input.IamInstanceProfile.Name) {
			return err
		}

		r.sess.Log("%s waiting for profile %s", *input.ImageId, *input.IamInstanceProfile.Name)
		time.Sleep(time.Second)

		resp, err = r.sess.EC2().RunInstances(ctx, &input)
	}

	if err != nil {
		return err
	}

	for _, instance := range resp.Instances {
		vm, err := r.formatter.Format(ctx, instance)
		if err != nil {
			return err
		}

		r.sess.Print(vm.String())
	}

	return nil
}

func (r InstanceRunner) buildParams(ctx context.Context, resources infra.Resources) (runParams, error) {
	in := r.createBasicInput(resources)

	if err := r.setTags(&in); err != nil {
		return runParams{}, err
	}

	waitForProfile, err := r.setProfile(ctx, &in, resources)
	if err != nil {
		return runParams{}, err
	}

	if err := r.setKeyPair(ctx, &in); err != nil {
		return runParams{}, err
	}

	var inputs []ec2.RunInstancesInput
	for _, image := range r.cfg.Images {
		imageId, err := r.imageResolver.Resolve(ctx, image)
		if err != nil {
			return runParams{}, err
		}

		in.ImageId = &imageId
		inputs = append(inputs, in)
	}

	return runParams{
		inputs:         inputs,
		waitForProfile: waitForProfile,
	}, nil
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

const (
	errProfileInvalidMsgTmpl = "Value (%s) for parameter iamInstanceProfile.name is invalid. Invalid IAM Instance Profile name"
	errProfileInvalidCode    = "InvalidParameterValue"
)
