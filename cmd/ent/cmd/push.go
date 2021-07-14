package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/fatih/color"
	"github.com/google/ent/utils"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:  "push",
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}
		i := parseIgnore(target)
		hash := traverse(target, "", i, push)
		if tagName != "" {
			tagStore.Set(context.Background(), tagName, []byte(utils.Hash(hash)))
		}
	},
}

func push(filename string, node format.Node) error {
	if filename == "" {
		filename = "."
	}
	marker := color.BlueString("↑")
	localHash := node.Cid()
	if !exists(localHash) {
		fmt.Printf("%s %s %s\n", color.YellowString(localHash.String()), marker, filename)
		return blobStore.Add(context.Background(), node)
	}
	return nil
}

func exists(hash cid.Cid) bool {
	ok, err := blobStore.Has(context.Background(), hash)
	if err != nil {
		log.Fatal(err)
	}
	return ok
}
