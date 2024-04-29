package stop

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use: "stop",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("ec2:StopInstances")
		},
	}
}
