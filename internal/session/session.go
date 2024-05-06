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

type Config struct {
	Regions []string
}

type Session struct {
	configs  []aws.Config
	iam      *iam.Client
	iamMutex sync.Mutex
}

func New(ctx context.Context, cfg *Config) (Session, error) {
	awsConfigs, err := resolveConfigs(ctx, cfg.Regions)
	if err != nil {
		return Session{}, err
	}
	if len(awsConfigs) == 0 {
		return Session{}, errors.New("couldn't find a usable AWS region")
	}

	return Session{configs: awsConfigs}, nil
}

func (s *Session) Run(ctx context.Context, run func(context.Context, aws.Config) error) error {
	errsC := make(chan error)
	for _, cfg := range s.configs {
		go func(cfg aws.Config) {
			errsC <- run(ctx, cfg)
		}(cfg)
	}

	errs := make([]error, 0, len(s.configs))
	for len(errs) < len(s.configs) {
		err := <-errsC
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (s *Session) RunIAM(ctx context.Context, run func(context.Context, *iam.Client) error) error {
	s.iamMutex.Lock()
	defer s.iamMutex.Unlock()

	if s.iam == nil {
		s.iam = iam.NewFromConfig(s.configs[0])
	}

	return run(ctx, s.iam)
}

func resolveConfigs(ctx context.Context, regions []string) ([]aws.Config, error) {
	if len(regions) == 1 && regions[0] == "all" {
		return resolveAll(ctx)
	}

	if len(regions) == 0 {
		return resolveDefault(ctx)
	}

	return resolveRegions(ctx, regions)
}

func resolveAll(ctx context.Context) ([]aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	ec2Client := ec2.NewFromConfig(cfg)
	resp, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{AllRegions: aws.Bool(false)})
	if err != nil {
		return nil, err
	}

	regions := make([]string, 0, len(resp.Regions)-1)
	for _, r := range resp.Regions {
		if *r.RegionName != cfg.Region {
			regions = append(regions, *r.RegionName)
		}
	}

	configs, err := resolveRegions(ctx, regions)
	if err != nil {
		return nil, err
	}

	return append(configs, cfg), nil
}

func resolveDefault(ctx context.Context) ([]aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return []aws.Config{cfg}, nil
}

func resolveRegions(ctx context.Context, regions []string) ([]aws.Config, error) {
	configsC := make(chan aws.Config)
	errsC := make(chan error)

	for _, r := range regions {
		go func(region string) {
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err == nil {
				configsC <- cfg
			}
			errsC <- err
		}(r)
	}

	errs := make([]error, 0, len(regions))
	configs := make([]aws.Config, 0)
	for len(errs) < len(regions) {
		select {
		case err := <-errsC:
			errs = append(errs, err)
		case cfg := <-configsC:
			configs = append(configs, cfg)
		}
	}

	return configs, errors.Join(errs...)
}
