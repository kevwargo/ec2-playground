package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (r InstanceRunner) setTags(ctx context.Context, in *ec2.RunInstancesInput, imageName string) error {
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
		expanded, err := expandTagValue(v, &tagTemplateData{
			ImageName: imageName,
			ImageID:   *in.ImageId,

			ctx: ctx,
			ec2: r.sess.EC2(),
		})
		if err != nil {
			return fmt.Errorf("expanding tag %q=%q for image %q: %w", k, v, *in.ImageId, err)
		}

		tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(expanded)})
	}

	in.TagSpecifications = []types.TagSpecification{
		{
			ResourceType: types.ResourceTypeInstance,
			Tags:         tags,
		},
	}

	return nil
}

type tagTemplateData struct {
	ImageName string
	ImageID   string

	ctx context.Context
	ec2 *ec2.Client

	image *types.Image
}

func (d *tagTemplateData) Image() (*types.Image, error) {
	if d.image != nil {
		return d.image, nil
	}

	resp, err := d.ec2.DescribeImages(d.ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{d.ImageID},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Images) == 0 {
		return nil, fmt.Errorf("image %s not found", d.ImageID)
	}

	d.image = &resp.Images[0]

	return d.image, nil
}

func (d *tagTemplateData) CreationDate() (string, error) {
	image, err := d.Image()
	if err != nil {
		return "", err
	}

	return (*image.CreationDate)[:10], nil
}

func (d *tagTemplateData) OSType() (string, error) {
	image, err := d.Image()
	if err != nil {
		return "", err
	}

	if image.Platform != "" {
		return string(image.Platform), nil
	}

	if image.PlatformDetails == nil {
		return "", nil
	}

	platform := strings.ToLower(*image.PlatformDetails)
	for _, ostype := range []string{"windows", "linux"} {
		if strings.Contains(platform, ostype) {
			return ostype, nil
		}
	}

	return "", nil
}

func expandTagValue(tmplText string, data *tagTemplateData) (string, error) {
	tmpl, err := template.New("tag").Parse(tmplText)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
