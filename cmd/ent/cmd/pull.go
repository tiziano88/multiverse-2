package cmd

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:  "pull [hash] [target directory]",
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		hash := args[0]
		targetDir := "."
		if len(args) >= 2 {
			targetDir = args[1]
		}
		targetDir, err := filepath.Abs(targetDir)
		if err != nil {
			log.Fatalf("could not normalize target directory %q, %v", targetDir, err)
		}

		files, err := ioutil.ReadDir(targetDir)
		if os.IsNotExist(err) {
			// Continue.
		} else if err != nil {
			log.Fatalf("could not read target directory %q: %v", targetDir, err)
		}
		if len(files) > 0 {
			log.Fatalf("cannot pull to non-empty directory %q", targetDir)
		}

		base, err := cid.Decode(hash)
		if err != nil {
			log.Fatalf("could not decode cid: %v", err)
		}

		traverseRemote(base, "", func(p string, node format.Node) error {
			fullPath := filepath.Join(targetDir, p)
			log.Printf("%s\n", fullPath)
			switch node := node.(type) {
			case *merkledag.ProtoNode:
				err := os.MkdirAll(fullPath, 0755)
				if err != nil {
					log.Fatalf("could not create directory %q: %v", fullPath, err)
				}
			case *merkledag.RawNode:
				err := ioutil.WriteFile(fullPath, node.RawData(), 0644)
				if err != nil {
					log.Fatalf("could not create file %q: %v", fullPath, err)
				}
			}
			return nil
		})

		log.Printf("pull %s %s", hash, targetDir)
	},
}

func traverseRemote(base cid.Cid, relativeFilename string, f func(string, format.Node) error) {
	node, err := blobStore.Get(context.Background(), base)
	if err != nil {
		log.Fatal(err)
	}

	switch node := node.(type) {
	case *merkledag.ProtoNode:
		err = f(relativeFilename, node)
		if err != nil {
			log.Fatal(err)
		}

		for _, l := range node.Links() {
			if l.Name == "" {
				continue
			}
			newRelativeFilename := path.Join(relativeFilename, l.Name)
			traverseRemote(l.Cid, newRelativeFilename, f)
		}
	case *merkledag.RawNode:
		err = f(relativeFilename, node)
		if err != nil {
			log.Fatal(err)
		}
	}
}
