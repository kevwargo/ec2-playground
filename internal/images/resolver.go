package images

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Resolver struct {
	ssm       *ssm.Client
	ec2Client *ec2.Client
}

func NewResolver(ssmClient *ssm.Client, ec2Client *ec2.Client) Resolver {
	return Resolver{
		ssm:       ssmClient,
		ec2Client: ec2Client,
	}
}

func (r Resolver) Resolve(ctx context.Context, patterns []string) ([]Image, error) {
	var (
		resolvedImages []Image
		amiNames       []string
	)

	for _, p := range patterns {
		if amiRegex.MatchString(p) {
			resolvedImages = append(resolvedImages, Image{
				Spec: p,
				ID:   p,
			})
			continue
		}

		imageID, err := r.resolveSSMParam(ctx, p)
		if err != nil {
			return nil, err
		}
		if imageID != nil {
			resolvedImages = append(resolvedImages, Image{
				Spec: p,
				ID:   *imageID,
			})
			continue
		}

		amiNames = append(amiNames, p)
	}

	foundImages, err := r.searchImages(ctx, amiNames)
	if err != nil {
		return nil, err
	}

	return append(resolvedImages, foundImages...), nil
}

func (r Resolver) ResolveWithDetails(ctx context.Context, patterns []string) ([]Image, error) {
	resolved, err := r.Resolve(ctx, patterns)
	if err != nil {
		return nil, err
	}

	incomplete := make(map[string]int)
	for idx, img := range resolved {
		if img.Details == nil {
			incomplete[img.ID] = idx
		}
	}
	if len(incomplete) == 0 {
		return resolved, nil
	}

	paginator := ec2.NewDescribeImagesPaginator(r.ec2Client, &ec2.DescribeImagesInput{
		ImageIds: slices.Collect(maps.Keys(incomplete)),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, img := range page.Images {
			if idx, ok := incomplete[*img.ImageId]; ok {
				resolved[idx].Details = &img
				delete(incomplete, *img.ImageId)
			}
		}
	}

	if len(incomplete) == 0 {
		return resolved, nil
	}

	var errs []error
	for imageID := range incomplete {
		errs = append(errs, fmt.Errorf("AMI %s not found", imageID))
	}

	return nil, errors.Join(errs...)
}

func (r Resolver) resolveSSMParam(ctx context.Context, name string) (*string, error) {
	if param, exists := builtinParams[name]; exists {
		resp, err := r.ssm.GetParameter(ctx, &ssm.GetParameterInput{Name: &param})
		if err != nil {
			return nil, err
		}

		return resp.Parameter.Value, nil
	}

	return nil, nil
}

func (r Resolver) searchImages(ctx context.Context, names []string) ([]Image, error) {
	var images []Image

	paginator := ec2.NewDescribeImagesPaginator(r.ec2Client, &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: names,
			},
		},
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, img := range page.Images {
			images = append(images, Image{
				Spec:    *img.Name,
				ID:      *img.ImageId,
				Details: &img,
			})
		}
	}

	return images, nil
}

var builtinParams = map[string]string{
	"amzn2":      "/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2",
	"al2023":     "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64",
	"al2023-min": "/aws/service/ami-amazon-linux-latest/al2023-ami-minimal-kernel-default-x86_64",
	"win22":      "/aws/service/ami-windows-latest/Windows_Server-2022-English-Full-Base",
	"win19":      "/aws/service/ami-windows-latest/Windows_Server-2019-English-Full-Base",
	"win16":      "/aws/service/ami-windows-latest/Windows_Server-2016-English-Full-Base",
	"win12":      "/aws/service/ami-windows-latest/Windows_Server-2012-R2_RTM-English-64Bit-Base",
}

var amiRegex = regexp.MustCompile("^ami-([0-9a-f]{9})?([0-9a-f]{8})$")
