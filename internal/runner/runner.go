package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/images"
	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/runner/userdata"
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
		infraFetcher:  infra.NewFetcher(sess, cfg.Infra),
		formatter:     vmformat.New(sess, cfg.DumpFormat.Template()),
		imageResolver: images.NewResolver(sess.SSM(), sess.EC2()),
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

	if waitForProfile {
		for profileNotReady(err, *input.IamInstanceProfile.Name) {
			r.sess.Log("%s waiting for profile %s", *input.ImageId, *input.IamInstanceProfile.Name)
			time.Sleep(time.Second)

			resp, err = r.sess.EC2().RunInstances(ctx, &input)
		}
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

func profileNotReady(err error, profileName string) bool {
	var ae smithy.APIError
	if !errors.As(err, &ae) {
		return false
	}

	if ae.ErrorCode() != errProfileInvalidCode {
		return false
	}

	return ae.ErrorMessage() == fmt.Sprintf(errProfileInvalidMsgTmpl, profileName)
}

func (r InstanceRunner) buildParams(ctx context.Context, resources infra.Resources) (runParams, error) {
	in := r.createBasicInput(resources)

	waitForProfile, err := r.setProfile(ctx, &in, resources)
	if err != nil {
		return runParams{}, err
	}

	if err := r.setKeyPair(ctx, &in); err != nil {
		return runParams{}, err
	}

	userData, err := userdata.Build(r.cfg.UserData)
	if err != nil {
		return runParams{}, err
	}
	in.UserData = userData

	resolvedImages, err := r.imageResolver.Resolve(ctx, r.cfg.Images)
	if err != nil {
		return runParams{}, err
	}

	var inputs []ec2.RunInstancesInput
	for _, image := range resolvedImages {
		in.ImageId = &image.ID

		if err := r.setTags(ctx, &in, image); err != nil {
			return runParams{}, err
		}

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

const (
	errProfileInvalidMsgTmpl = "Value (%s) for parameter iamInstanceProfile.name is invalid. Invalid IAM Instance Profile name"
	errProfileInvalidCode    = "InvalidParameterValue"
)
