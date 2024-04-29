package ssh

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use: "ssh",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("ssh")
		},
	}
}
