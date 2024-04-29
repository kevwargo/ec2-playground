package ls

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use: "ls",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("ec2:DescribeInstances")
		},
	}
}
