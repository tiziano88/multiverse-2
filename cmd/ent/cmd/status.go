package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

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
		plan, err := parsePlan(filepath.Join(target, planFilename))
		if err != nil {
			log.Panic(err)
		}
		fmt.Printf("%#v\n", plan)
		i := parseIgnore(target)
		traverse(target, "", i, status)
	},
}

func status(filename string, node format.Node) error {
	if filename == "" {
		filename = "."
	}
	marker := color.RedString("*")
	ok, _ := blobStore.Has(context.Background(), node.Cid())
	if ok {
		marker = color.GreenString("✓")
	}
	// hash = hash[len(hash)-16:]
	c := node.Cid().String()
	fmt.Printf("%s %s %s\n", color.YellowString(c), marker, filename)
	// h := node.Cid().Hash()
	// fmt.Printf("%s %s %s\n", color.YellowString(h.HexString()), marker, filename)
	// fmt.Printf("%s %s %s\n", color.YellowString(h.B58String()), marker, filename)
	return nil
}
