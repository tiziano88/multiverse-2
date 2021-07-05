package cmd

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	format "github.com/ipfs/go-ipld-format"
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

func status(filename string, node format.Node) error {
	marker := color.RedString("*")
	ok, _ := blobStore.Has(context.Background(), node.Cid())
	if ok {
		marker = color.GreenString("✓")
	}
	hash := node.Cid().String()
	// hash = hash[len(hash)-16:]
	fmt.Printf("%s %s %s\n", color.YellowString(hash), marker, filename)
	return nil
}
