package infra

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/session"
)

//go:embed template.yml
var templateBody string

var (
	DefaultStackName = "EC2PlaygroundToolkit"

	errMsgStackNonExistent = regexp.MustCompile("Stack .* does not exist")
	errMsgInvalidState     = regexp.MustCompile("Stack:(.*) is in ([A-Z_]+) state and can not be updated.")

	noUpdates = errors.New("No updates are to be performed.")
)

const (
	errCodeValidation   = "ValidationError"
	defaultVPCParamName = "DefaultVPC"
)

type Resources struct {
	SecurityGroup   string `json:"SecurityGroup"`
	Subnet          string `json:"Subnet"`
	InstancePolicy  string `json:"InstancePolicy"`
	InstanceProfile string `json:"InstanceProfile"`
	Bucket          string `json:"Bucket"`
}

type Fetcher struct {
	session    *session.Regional
	stackName  string
	skipDeploy bool
}

func NewFetcher(sess *session.Regional, cfg config.InfraConfig) Fetcher {
	return Fetcher{
		session:    sess,
		stackName:  cfg.StackName,
		skipDeploy: cfg.SkipDeploy,
	}
}

func (f Fetcher) Fetch(ctx context.Context) (Resources, error) {
	if !f.skipDeploy {
		if err := f.deploy(ctx); err != nil {
			return Resources{}, err
		}
	}

	resp, err := f.session.CFN().DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &f.stackName,
	})
	if err != nil {
		return Resources{}, err
	}

	if len(resp.Stacks) == 0 {
		return Resources{}, fmt.Errorf("DescribeStacks response for %q is empty", f.stackName)
	}

	outputs := resp.Stacks[0].Outputs
	outputsMap := make(map[string]string, len(outputs))
	for _, o := range outputs {
		outputsMap[*o.OutputKey] = *o.OutputValue
	}

	outputsJson, err := json.Marshal(outputsMap)
	if err != nil {
		return Resources{}, err
	}

	var infra Resources
	if err := json.Unmarshal(outputsJson, &infra); err != nil {
		return Resources{}, err
	}

	return infra, nil
}

func (f Fetcher) deploy(ctx context.Context) error {
	params, err := f.prepareStackParams(ctx)
	if err != nil {
		return err
	}

	stackID, err := f.tryStartUpdate(ctx, params)
	if err != nil {
		if errors.Is(err, noUpdates) {
			return nil
		}

		return err
	}

	if stackID == "" {
		stackID, err = f.startCreate(ctx, params)
		if err != nil {
			return err
		}
	}

	return f.wait(ctx, stackID, types.StackStatusCreateComplete, types.StackStatusUpdateComplete)
}

func (f Fetcher) tryStartUpdate(ctx context.Context, params []types.Parameter) (string, error) {
	resp, err := f.session.CFN().UpdateStack(ctx, &cloudformation.UpdateStackInput{
		StackName:    &f.stackName,
		Capabilities: []types.Capability{types.CapabilityCapabilityIam},
		TemplateBody: &templateBody,
		Parameters:   params,
	})

	if err == nil {
		f.session.Log("Updating stack %q", *resp.StackId)
		return *resp.StackId, nil
	}

	return f.handleValidationError(ctx, err)
}

func (f Fetcher) handleValidationError(ctx context.Context, err error) (string, error) {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return "", err
	}

	code, msg := apiErr.ErrorCode(), apiErr.ErrorMessage()

	if code != errCodeValidation {
		return "", err
	}

	if msg == noUpdates.Error() {
		f.session.Log("Stack %q is up-to-date", f.stackName)
		return "", noUpdates
	}

	if errMsgStackNonExistent.MatchString(msg) {
		f.session.Log("Stack %q does not exist, creating", f.stackName)
		return "", nil
	}

	if m := errMsgInvalidState.FindStringSubmatch(msg); m != nil {
		stackID, stackStatus := m[1], m[2]

		if inProgress(stackStatus) {
			f.session.Log("Stack %q is %s, waiting for completion", f.stackName, stackStatus)
			return stackID, nil
		}

		if stackStatus == string(types.StackStatusRollbackComplete) {
			return "", f.deleteRolledBack(ctx, stackID)
		}
	}

	return "", err
}

func (f Fetcher) deleteRolledBack(ctx context.Context, stackID string) error {
	f.session.Log("Deleting rolled back %q", stackID)

	_, err := f.session.CFN().DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: &stackID,
	})
	if err != nil {
		return err
	}

	return f.wait(ctx, stackID, types.StackStatusDeleteComplete)
}

func (f Fetcher) startCreate(ctx context.Context, params []types.Parameter) (string, error) {
	resp, err := f.session.CFN().CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &f.stackName,
		Capabilities: []types.Capability{types.CapabilityCapabilityIam},
		TemplateBody: &templateBody,
		Parameters:   params,
	})
	if err != nil {
		return "", err
	}

	f.session.Log("Creating stack %q", *resp.StackId)
	return *resp.StackId, nil
}

func (f Fetcher) prepareStackParams(ctx context.Context) ([]types.Parameter, error) {
	defaultVPC, err := f.getDefaultVPC(ctx)
	if defaultVPC == "" {
		return nil, err
	}

	return []types.Parameter{
		{
			ParameterKey:   aws.String(defaultVPCParamName),
			ParameterValue: &defaultVPC,
		},
	}, nil
}

func (f Fetcher) getDefaultVPC(ctx context.Context) (string, error) {
	paginator := ec2.NewDescribeVpcsPaginator(f.session.EC2(), &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("is-default"),
				Values: []string{"true"},
			},
		},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return "", err
		}

		for _, vpc := range page.Vpcs {
			if vpc.VpcId != nil {
				return *vpc.VpcId, nil
			}
		}
	}

	return "", nil
}
