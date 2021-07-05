package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tiziano88/multiverse/utils"
)

var pushCmd = &cobra.Command{
	Use:  "push",
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}
		hash := traverse(target, "", push)
		fmt.Printf("%s %s\n", hash, target)
		if tagName != "" {
			tagStore.Set(context.Background(), tagName, []byte(utils.Hash(hash)))
		}
	},
}
