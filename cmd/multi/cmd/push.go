package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use: "push",
	Run: func(cmd *cobra.Command, args []string) {
		target := args[0]
		hash := traverse(target, "", push)
		fmt.Printf("%s %s\n", hash, target)
	},
}
