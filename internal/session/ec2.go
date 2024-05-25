package session

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"

	"kevwargo/ec2-playground/internal/vmformat"
)

func (s *Session) RunEC2(
	ctx context.Context,
	scan func(context.Context, aws.Config) ([]vmformat.VM, error),
	transform func(context.Context, aws.Config, []vmformat.VM) error,
) error {
	configs := make(map[string]aws.Config, len(s.configs))
	for _, cfg := range s.configs {
		configs[cfg.Region] = cfg
	}

	scanned, err := s.scanEC2(ctx, scan)
	if err != nil {
		return err
	}

	selector, err := selectVMs(scanned)
	if err != nil {
		return err
	}

	errsC := make(chan error)
	var jobsCount int
	for region, vmGroup := range scanned {
		var vms []vmformat.VM
		for _, vm := range vmGroup {
			if selector.all || selector.include[vm.index] {
				vms = append(vms, vm.vm)
			}
		}

		if len(vms) > 0 {
			go func(config aws.Config) {
				errsC <- transform(ctx, config, vms)
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

type indexedVM struct {
	index int
	vm    vmformat.VM
}

type scanResp struct {
	region string
	vms    []vmformat.VM
	err    error
}

func (s *Session) scanEC2(
	ctx context.Context,
	scan func(context.Context, aws.Config) ([]vmformat.VM, error),
) (map[string][]indexedVM, error) {
	respC := make(chan scanResp)
	for _, cfg := range s.configs {
		go func(cfg aws.Config) {
			vms, err := scan(ctx, cfg)
			respC <- scanResp{
				region: cfg.Region,
				vms:    vms,
				err:    err,
			}
		}(cfg)
	}

	index := 1
	scanned := make(map[string][]indexedVM)
	errs := make([]error, 0, len(s.configs))
	for range s.configs {
		resp := <-respC

		errs = append(errs, resp.err)
		for _, vm := range resp.vms {
			scanned[resp.region] = append(scanned[resp.region], indexedVM{
				index: index,
				vm:    vm,
			})
			index++
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return scanned, nil
}

type selector struct {
	all     bool
	include map[int]bool
}

func selectVMs(vmGroups map[string][]indexedVM) (selector, error) {
	for _, vmGroup := range vmGroups {
		for _, vm := range vmGroup {
			fmt.Printf("[%d] %s\n", vm.index, vm.vm)
		}
	}

	fmt.Print("Select VMs: ")
	prompt := bufio.NewScanner(os.Stdin)
	if ok := prompt.Scan(); !ok {
		if err := prompt.Err(); err != nil {
			return selector{}, err
		}

		return selector{}, errors.New("empty input")
	}

	resp := prompt.Text()

	if resp == "all" {
		return selector{all: true}, nil
	}

	s := selector{include: make(map[int]bool)}
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
			s.include[i] = include
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
		return 0, 0, fmt.Errorf("invalid number range: %s: %d > %d", raw, beg, end)
	}

	return beg, end, nil
}
