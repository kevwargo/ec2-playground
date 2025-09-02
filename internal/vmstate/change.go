package vmstate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/lister"
	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmformat"
)

func BuildCommand(
	sess *session.Global,
	op func(context.Context, *ec2.Client, []string) ([]types.InstanceStateChange, error),
) *cobra.Command {
	var (
		dumpFormat config.VMFormat
		cmd        cobra.Command
		matchNames []string
	)

	cmd.RunE = func(c *cobra.Command, _ []string) error {
		return changeState(c.Context(), changeStateInput{
			session:    sess,
			op:         op,
			tmpl:       dumpFormat.Template(),
			matchNames: matchNames,
		})
	}

	cmd.Flags().VarP(&dumpFormat, "format", "f", "Instance format")
	cmd.Flags().StringArrayVarP(&matchNames, "name-filter", "N", nil, "Simple case-insensitive matching by instance name")

	return &cmd
}

type changeStateInput struct {
	session    *session.Global
	op         func(context.Context, *ec2.Client, []string) ([]types.InstanceStateChange, error)
	tmpl       *template.Template
	matchNames []string
}

func changeState(ctx context.Context, in changeStateInput) error {
	all, err := listVMs(ctx, in)
	if err != nil {
		return err
	}

	selected, err := selectVMs(all)
	if err != nil {
		return err
	}

	return in.session.Run(ctx, func(ctx context.Context, s *session.Regional) error {
		vms := selected[s.Region]
		if len(vms) == 0 {
			return nil
		}

		ids := make([]string, 0, len(vms))
		for _, vm := range vms {
			ids = append(ids, *vm.InstanceId)
		}

		changes, err := in.op(ctx, s.EC2(), ids)
		if err != nil {
			return err
		}

		for _, change := range changes {
			s.Log("%s: %s -> %s", *change.InstanceId, change.PreviousState.Name, change.CurrentState.Name)
		}

		return nil
	})
}

func listVMs(ctx context.Context, in changeStateInput) (map[string][]vmformat.VM, error) {
	var vmLock sync.Mutex
	all := make(map[string][]vmformat.VM)

	if err := in.session.Run(ctx, func(ctx context.Context, s *session.Regional) error {
		vms, err := lister.New(s, in.tmpl).ListVMs(ctx, in.matchNames)
		if err != nil {
			return err
		}

		vmLock.Lock()
		defer vmLock.Unlock()

		all[s.Region] = vms

		return nil
	}); err != nil {
		return nil, err
	}

	return all, nil
}

func selectVMs(all map[string][]vmformat.VM) (map[string][]vmformat.VM, error) {
	indexedVMs := make(map[string][]indexed)
	index := 1
	for region, vms := range all {
		for _, vm := range vms {
			indexedVMs[region] = append(indexedVMs[region], indexed{
				vm:    vm,
				index: index,
			})

			fmt.Printf("[%d] %s\n", index, vm)

			index++
		}
	}

	s, err := promptSelector()
	if err != nil {
		return nil, err
	}

	selected := make(map[string][]vmformat.VM)
	for region, vms := range indexedVMs {
		for _, vm := range vms {
			if s.match(vm.index) {
				selected[region] = append(selected[region], vm.vm)
			}
		}
	}

	return selected, nil
}

type indexed struct {
	vm    vmformat.VM
	index int
}

func promptSelector() (selector, error) {
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

type selector struct {
	all     bool
	include map[int]bool
}

func (s selector) match(idx int) bool {
	return s.all || s.include[idx]
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
