package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/ent/datastore"
	"github.com/google/ent/nodeservice"
	"github.com/google/ent/objectstore"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag/dagutils"
	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:  "diff",
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			fmt.Println("usage")
			fmt.Println("ent diff <from> <to>")
			fmt.Println("<from> and <to> are either local file paths, or ent hashes")
			os.Exit(1)
		}

		from := args[0]
		if !strings.HasPrefix(from, "baf") {
			hash, inMemory := buildInMemory(from)
			blobStore = nodeservice.Multiplex{
				Inner: []nodeservice.NodeService{
					inMemory,
					blobStore,
				},
			}
			from = hash.String()
		}

		to := args[1]
		if !strings.HasPrefix(to, "baf") {
			hash, inMemory := buildInMemory(to)
			blobStore = nodeservice.Multiplex{
				Inner: []nodeservice.NodeService{
					inMemory,
					blobStore,
				},
			}
			to = hash.String()
		}

		diff(from, to)
	},
}

func buildInMemory(path string) (cid.Cid, nodeservice.DataStore) {
	s := nodeservice.DataStore{
		Inner: objectstore.Store{
			Inner: datastore.InMemory{
				Inner: make(map[string][]byte),
			},
		},
	}
	f := func(filename string, node format.Node) error {
		// TODO
		return nil
	}
	i, err := ignore.CompileIgnoreFile(filepath.Join(path, ".gitignore"))
	if err != nil {
		log.Panic(err)
	}
	hash := traverse(path, "", i, f)
	return hash, s
}

func diff(from string, to string) error {
	fromCid, err := cid.Decode(from)
	if err != nil {
		return fmt.Errorf("could not decode from: %v", err)
	}
	toCid, err := cid.Decode(to)
	if err != nil {
		return fmt.Errorf("could not decode to: %v", err)
	}
	diffs, err := diffCid(fromCid, toCid)
	if err != nil {
		return fmt.Errorf("could not compute diff: %v", err)
	}
	for _, d := range diffs {
		switch d.Type {
		case dagutils.Add:
			fmt.Printf("+ %v\n", d.Path)
		case dagutils.Remove:
			fmt.Printf("- %v\n", d.Path)
		case dagutils.Mod:
			fmt.Printf("* %v\n", d.Path)
		}
	}
	return nil
}

func diffCid(from cid.Cid, to cid.Cid) ([]*dagutils.Change, error) {
	fromNode, err := blobStore.Get(context.Background(), from)
	if err != nil {
		return nil, fmt.Errorf("could not get from: %v", err)
	}
	toNode, err := blobStore.Get(context.Background(), to)
	if err != nil {
		return nil, fmt.Errorf("could not get to: %v", err)
	}
	return dagutils.Diff(context.TODO(), blobStore, fromNode, toNode)
}
