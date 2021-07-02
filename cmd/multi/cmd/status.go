package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use: "status",
	Run: func(cmd *cobra.Command, args []string) {
		target := args[0]
		if target == "" {
			target = "."
		}
		hash := traverse(target, "", status)
		fmt.Printf("%s %s\n", hash, target)
	},
}
