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

func (g *Global) init(ctx context.Context) error {
	if g.defaultRegional != nil && g.regional != nil {
		return nil
	}

	if len(g.Regions) == 1 && g.Regions[0] == "all" {
		return g.resolveAll(ctx)
	}

	if len(g.Regions) == 0 {
		return g.resolveDefault(ctx)
	}

	return g.resolveLiteral(ctx, g.Regions)
}

func (g *Global) resolveAll(ctx context.Context) error {
	if err := g.resolveDefault(ctx); err != nil {
		return err
	}

	ec2Client := ec2.NewFromConfig(g.defaultRegional.cfg)
	g.defaultRegional.ec2 = ec2Client

	resp, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{AllRegions: aws.Bool(false)})
	if err != nil {
		return err
	}

	regions := make([]string, 0, len(resp.Regions)-1)
	for _, r := range resp.Regions {
		if *r.RegionName != g.defaultRegional.cfg.Region {
			regions = append(regions, *r.RegionName)
		}
	}

	return g.resolveLiteral(ctx, regions)
}

func (g *Global) resolveDefault(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	g.defaultRegional = g.newRegional(cfg)
	g.regional = map[string]*Regional{cfg.Region: g.defaultRegional}

	return nil
}

type resolveResp struct {
	cfg aws.Config
	err error
}

func (g *Global) resolveLiteral(ctx context.Context, regions []string) error {
	respC := make(chan resolveResp)

	for _, region := range regions {
		go func() {
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			respC <- resolveResp{
				cfg: cfg,
				err: err,
			}
		}()
	}

	if g.regional == nil {
		g.regional = make(map[string]*Regional)
	}

	var errs []error
	for range len(regions) {
		resp := <-respC
		if err := resp.err; err != nil {
			errs = append(errs, err)
		} else {
			g.regional[resp.cfg.Region] = g.newRegional(resp.cfg)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	if g.defaultRegional == nil {
		for _, r := range regions {
			if s, exists := g.regional[r]; exists {
				g.defaultRegional = s
				break
			}
		}
	}

	return nil
}
