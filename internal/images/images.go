package images

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Resolver struct {
	ssm   *ssm.Client
	cache map[string]string
}

func NewResolver(ssmClient *ssm.Client) Resolver {
	return Resolver{
		ssm:   ssmClient,
		cache: make(map[string]string),
	}
}

func (r Resolver) Resolve(ctx context.Context, name string) (string, error) {
	if strings.HasPrefix(name, "ami-") {
		return name, nil
	}

	if resolved, exists := r.cache[name]; exists {
		return resolved, nil
	}

	resolved, err := r.resolve(ctx, name)
	if err != nil {
		return "", err
	}

	r.cache[name] = resolved
	return resolved, nil
}

func (r Resolver) resolve(ctx context.Context, name string) (string, error) {
	if param, exists := builtinParams[name]; exists {
		return r.getParam(ctx, param)
	}

	return "", fmt.Errorf("invalid image description: %q", name)
}

func (r Resolver) getParam(ctx context.Context, name string) (string, error) {
	resp, err := r.ssm.GetParameter(ctx, &ssm.GetParameterInput{Name: &name})
	if err != nil {
		return "", err
	}

	return *resp.Parameter.Value, nil
}

var builtinParams = map[string]string{
	"amazon": "/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2",
	"win22":  "/aws/service/ami-windows-latest/Windows_Server-2022-English-Full-Base",
	"win19":  "/aws/service/ami-windows-latest/Windows_Server-2019-English-Full-Base",
	"win16":  "/aws/service/ami-windows-latest/Windows_Server-2016-English-Full-Base",
	"win12":  "/aws/service/ami-windows-latest/Windows_Server-2012-R2_RTM-English-64Bit-Base",
}
