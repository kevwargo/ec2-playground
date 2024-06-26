package session

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

type Global struct {
	Regions []string

	regional        map[string]*Regional
	defaultRegional *Regional
	iam             *iam.Client
	iamMutex        sync.Mutex
	logMutex        sync.Mutex
}

func (c *Global) init(ctx context.Context) error {
	if c.defaultRegional != nil && c.regional != nil {
		return nil
	}

	if len(c.Regions) == 1 && c.Regions[0] == "all" {
		return c.resolveAll(ctx)
	}

	if len(c.Regions) == 0 {
		return c.resolveDefault(ctx)
	}

	return c.resolveLiteral(ctx, c.Regions)
}

func (c *Global) resolveAll(ctx context.Context) error {
	if err := c.resolveDefault(ctx); err != nil {
		return err
	}

	ec2Client := ec2.NewFromConfig(c.defaultRegional.cfg)
	c.defaultRegional.ec2 = ec2Client

	resp, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{AllRegions: aws.Bool(false)})
	if err != nil {
		return err
	}

	regions := make([]string, 0, len(resp.Regions)-1)
	for _, r := range resp.Regions {
		if *r.RegionName != c.defaultRegional.cfg.Region {
			regions = append(regions, *r.RegionName)
		}
	}

	return c.resolveLiteral(ctx, regions)
}

func (c *Global) resolveDefault(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	c.defaultRegional = c.newRegional(cfg)
	c.regional = map[string]*Regional{cfg.Region: c.defaultRegional}

	return nil
}

type resolveResp struct {
	cfg aws.Config
	err error
}

func (c *Global) resolveLiteral(ctx context.Context, regions []string) error {
	respC := make(chan resolveResp)

	for _, r := range regions {
		go func(region string) {
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			respC <- resolveResp{
				cfg: cfg,
				err: err,
			}
		}(r)
	}

	if c.regional == nil {
		c.regional = make(map[string]*Regional)
	}

	errs := make([]error, len(regions))
	for i := range errs {
		resp := <-respC
		if err := resp.err; err != nil {
			errs[i] = err
		} else {
			c.regional[resp.cfg.Region] = c.newRegional(resp.cfg)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	if c.defaultRegional == nil {
		for _, r := range regions {
			if s, exists := c.regional[r]; exists {
				c.defaultRegional = s
				break
			}
		}
	}

	return nil
}
