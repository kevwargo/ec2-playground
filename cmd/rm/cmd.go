package rm

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use: "rm",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("ec2:TerminateInstances")
		},
	}
}
