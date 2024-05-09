package infra

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go"

	"kevwargo/ec2-playground/internal/config"
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
	errCodeValidation = "ValidationError"
)

type Resources struct {
	SecurityGroup   string
	Subnet          string
	InstancePolicy  string
	InstanceProfile string
	Bucket          string
}

type Fetcher struct {
	cfn        *cloudformation.Client
	stackName  string
	skipDeploy bool
	log        *log.Logger
}

func NewFetcher(awsCfg aws.Config, runCfg config.RunConfig) Fetcher {
	return Fetcher{
		cfn:        cloudformation.NewFromConfig(awsCfg),
		stackName:  runCfg.InfraStackName,
		skipDeploy: runCfg.SkipInfraDeploy,
		log:        log.New(os.Stderr, fmt.Sprintf("%s: ", awsCfg.Region), log.LstdFlags),
	}
}

func (f Fetcher) Fetch(ctx context.Context) (Resources, error) {
	if !f.skipDeploy {
		if err := f.deploy(ctx); err != nil {
			return Resources{}, err
		}
	}

	resp, err := f.cfn.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
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
	stackID, err := f.tryStartUpdate(ctx)
	if err != nil {
		if errors.Is(err, noUpdates) {
			return nil
		}

		return err
	}

	if stackID == "" {
		stackID, err = f.startCreate(ctx)
		if err != nil {
			return err
		}
	}

	return f.wait(ctx, stackID, types.StackStatusCreateComplete, types.StackStatusUpdateComplete)
}

func (f Fetcher) tryStartUpdate(ctx context.Context) (string, error) {
	resp, err := f.cfn.UpdateStack(ctx, &cloudformation.UpdateStackInput{
		StackName:    &f.stackName,
		Capabilities: []types.Capability{types.CapabilityCapabilityIam},
		TemplateBody: &templateBody,
	})

	if err == nil {
		f.log.Printf("Updating stack %q", *resp.StackId)
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
		f.log.Printf("Stack %q is up-to-date", f.stackName)
		return "", noUpdates
	}

	if errMsgStackNonExistent.MatchString(msg) {
		f.log.Printf("Stack %q does not exist, creating", f.stackName)
		return "", nil
	}

	if m := errMsgInvalidState.FindStringSubmatch(msg); m != nil {
		stackID, stackStatus := m[1], m[2]

		if inProgress(stackStatus) {
			f.log.Printf("Stack %q is %s, waiting for completion", f.stackName, stackStatus)
			return stackID, nil
		}

		if stackStatus == string(types.StackStatusRollbackComplete) {
			return "", f.deleteRolledBack(ctx, stackID)
		}
	}

	return "", err
}

func (f Fetcher) deleteRolledBack(ctx context.Context, stackID string) error {
	f.log.Printf("Deleting rolled back %q", stackID)

	_, err := f.cfn.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: &stackID,
	})
	if err != nil {
		return err
	}

	return f.wait(ctx, stackID, types.StackStatusDeleteComplete)
}

func (f Fetcher) startCreate(ctx context.Context) (string, error) {
	resp, err := f.cfn.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &f.stackName,
		Capabilities: []types.Capability{types.CapabilityCapabilityIam},
		TemplateBody: &templateBody,
	})
	if err != nil {
		return "", err
	}

	f.log.Printf("Creating stack %q", *resp.StackId)
	return *resp.StackId, nil
}
