package exec

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use: "exec",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("ssm:SendCommand")
		},
	}
}
