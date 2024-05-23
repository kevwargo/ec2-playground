package session

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type (
	scanner func(context.Context, aws.Config) ([]types.Instance, error)
	actor   func(context.Context, aws.Config, []types.Instance) error
)

type result struct {
	index    int
	instance types.Instance
}

func (s *Session) RunEC2(ctx context.Context, scan scanner, act actor) error {
	configs := make(map[string]aws.Config, len(s.configs))
	for _, cfg := range s.configs {
		configs[cfg.Region] = cfg
	}

	results, err := s.scanEC2(ctx, scan)
	if err != nil {
		return err
	}

	log.Printf("Scanned all %d", len(results))

	selector, err := selectInstances(results)
	if err != nil {
		return err
	}

	errsC := make(chan error)
	var jobsCount int
	for region, batch := range results {
		var instances []types.Instance
		for _, result := range batch {
			if selector.all || selector.indices[result.index] {
				instances = append(instances, result.instance)
			}
		}

		if len(instances) > 0 {
			go func(config aws.Config) {
				errsC <- act(ctx, config, instances)
			}(configs[region])

			jobsCount++
		}
	}

	errs := make([]error, 0, jobsCount)
	for i := 0; i < jobsCount; i++ {
		errs = append(errs, <-errsC)
	}

	return errors.Join(errs...)
}

type scanResp struct {
	region    string
	instances []types.Instance
	err       error
}

func (s *Session) scanEC2(ctx context.Context, scan scanner) (map[string][]result, error) {
	respC := make(chan scanResp)
	for _, cfg := range s.configs {
		go func(cfg aws.Config) {
			instances, err := scan(ctx, cfg)
			respC <- scanResp{
				region:    cfg.Region,
				instances: instances,
				err:       err,
			}
		}(cfg)
	}

	index := 1
	results := make(map[string][]result)
	errs := make([]error, 0, len(s.configs))
	for range s.configs {
		resp := <-respC

		errs = append(errs, resp.err)
		for _, instance := range resp.instances {
			results[resp.region] = append(results[resp.region], result{
				index:    index,
				instance: instance,
			})
			index++
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return results, nil
}

type selector struct {
	all     bool
	indices map[int]bool
}

func selectInstances(results map[string][]result) (selector, error) {
	for region, batch := range results {
		for _, result := range batch {
			fmt.Printf("[%d] %s %s\n", result.index, region, formatInstance(result.instance))
		}
	}

	fmt.Print("Select instances: ")
	prompt := bufio.NewScanner(os.Stdin)
	if ok := prompt.Scan(); !ok {
		if err := prompt.Err(); err != nil {
			return selector{}, err
		}

		return selector{}, errors.New("empty input")
	}

	resp := prompt.Text()

	if resp == "$ALL" {
		return selector{all: true}, nil
	}

	s := selector{indices: make(map[int]bool)}
	for _, group := range strings.Split(resp, ",") {
		include := true
		if strings.HasPrefix(group, "!") {
			include = false
			group = group[1:]
		}

		beg, end, err := parseRange(group)
		if err != nil {
			return selector{}, err
		}

		for i := beg; i <= end; i++ {
			s.indices[i] = include
		}
	}

	return s, nil
}

func parseRange(raw string) (beg int, end int, err error) {
	parts := strings.Split(raw, "-")
	switch len(parts) {
	case 1:
		beg, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, err
		}
		end = beg
	case 2:
		beg, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, err
		}
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
	default:
		return 0, 0, fmt.Errorf("invalid number range: %s", raw)
	}

	if beg > end {
		return 0, 0, fmt.Errorf("%d > %d", beg, end)
	}

	return beg, end, nil
}

func formatInstance(instance types.Instance) string {
	var name string
	for _, tag := range instance.Tags {
		if *tag.Key == "Name" {
			name = *tag.Value
			break
		}
	}

	return fmt.Sprintf("%s %s %s", *instance.InstanceId, name, instance.State.Name)
}
