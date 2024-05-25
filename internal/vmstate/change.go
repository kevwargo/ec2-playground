package vmstate

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/config"
	"kevwargo/ec2-playground/internal/lister"
	"kevwargo/ec2-playground/internal/session"
	"kevwargo/ec2-playground/internal/vmformat"
)

func BuildChangeCommand(
	name string,
	sess *session.Session,
	change func(context.Context, *ec2.Client, []string) ([]types.InstanceStateChange, error),
) *cobra.Command {
	var dumpFormat config.VMFormat

	cmd := &cobra.Command{
		Use: name,
		RunE: func(c *cobra.Command, _ []string) error {
			tmpl := dumpFormat.Template()

			return sess.RunEC2(
				c.Context(),
				func(ctx context.Context, cfg aws.Config) ([]vmformat.VM, error) {
					return lister.New(cfg, tmpl).ListVMs(ctx)
				},
				func(ctx context.Context, cfg aws.Config, vms []vmformat.VM) error {
					logger := log.New(os.Stderr, fmt.Sprintf("%s: ", cfg.Region), log.LstdFlags)
					client := ec2.NewFromConfig(cfg)

					ids := make([]string, 0, len(vms))
					for _, vm := range vms {
						ids = append(ids, *vm.InstanceId)
					}

					changes, err := change(ctx, client, ids)
					if err != nil {
						return err
					}

					for _, c := range changes {
						logger.Printf("%s: %s -> %s", *c.InstanceId, c.PreviousState.Name, c.CurrentState.Name)
					}

					return nil
				},
			)
		},
	}

	cmd.Flags().VarP(&dumpFormat, "format", "f", "Instance format")

	return cmd
}
