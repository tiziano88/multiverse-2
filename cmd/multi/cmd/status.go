package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:  "status",
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}
		hash := traverse(target, "", status)
		fmt.Printf("%s %s\n", hash, target)
	},
}
