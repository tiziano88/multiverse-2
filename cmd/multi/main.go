package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/tiziano88/multiverse/datastore"
	"github.com/tiziano88/multiverse/nodeservice"
	"github.com/tiziano88/multiverse/utils"
)

var (
	blobStore nodeservice.NodeService
	tagStore  datastore.DataStore
)

// const webURL = "www." + apiURL

func init_local() {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatalf("could not get user cache dir: %v", err)
	}

	{
		multiverseBlobCacheDir := filepath.Join(userCacheDir, "multiverse", "blobs")
		err = os.MkdirAll(multiverseBlobCacheDir, 0755)
		if err != nil {
			log.Fatalf("could not create blob cache dir: %v", err)
		}
		blobStore = nodeservice.DataStore{
			Inner: datastore.File{
				DirName: multiverseBlobCacheDir,
			},
		}
	}

	{
		multiverseTagsCacheDir := filepath.Join(userCacheDir, "multiverse", "tags")
		err = os.MkdirAll(multiverseTagsCacheDir, 0755)
		if err != nil {
			log.Fatalf("could not create tag cache dir: %v", err)
		}
		tagStore = datastore.File{
			DirName: multiverseTagsCacheDir,
		}
	}
}

func init_remote(apiURL string) {
	blobStore = nodeservice.Remote{
		APIURL: apiURL,
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("expected command")
		os.Exit(1)
	}

	// https://gobyexample.com/command-line-subcommands
	switch os.Args[1] {
	case "push":
		set := flag.NewFlagSet("push", flag.ExitOnError)
		remoteURL := set.String("url", "", "URL of the remote to push to [01.plus, localhost:8080]")

		set.Parse(os.Args[2:])

		if *remoteURL == "" {
			init_local()
		} else {
			init_remote(*remoteURL)
		}

		target := set.Arg(0)
		hash := traverse(target, "", push)
		fmt.Printf("%s %s\n", hash, target)
		// log.Printf("http://%s/blobs/%s", apiURL, hash)

	case "status":
		set := flag.NewFlagSet("status", flag.ExitOnError)
		remoteURL := set.String("url", "", "URL of the remote to push to [01.plus, localhost:8080]")

		set.Parse(os.Args[2:])

		if *remoteURL == "" {
			init_local()
		} else {
			init_remote(*remoteURL)
		}

		target := set.Arg(0)
		if target == "" {
			target = "."
		}
		hash := traverse(target, "", status)
		fmt.Printf("%s %s\n", hash, target)

	case "diff":
		set := flag.NewFlagSet("diff", flag.ExitOnError)
		set.Parse(os.Args[2:])

		if set.NArg() != 2 {
			fmt.Println("usage")
			fmt.Println("multi diff <from> <to>")
			fmt.Println("<from> and <to> are either local file paths, or multiverse hashes")
			os.Exit(1)
		}

		from := set.Arg(0)
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

		to := set.Arg(1)
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

	default:
		log.Fatalf("invalid command: %s", os.Args[1])
	}
}

func buildInMemory(path string) (cid.Cid, nodeservice.InMemory) {
	m := make(map[cid.Cid]format.Node)
	f := func(filename string, node format.Node) error {
		m[node.Cid()] = node
		return nil
	}
	hash := traverse(path, "", f)
	return hash, nodeservice.InMemory{
		Inner: m,
	}
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

func push(filename string, node format.Node) error {
	localHash := node.Cid()
	if !exists(localHash) {
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

func traverse(base string, relativeFilename string, f func(string, format.Node) error) cid.Cid {
	file, err := os.Open(path.Join(base, relativeFilename))
	if err != nil {
		log.Fatal(err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}

	if fileInfo.IsDir() {
		files, err := file.Readdir(-1)
		if err != nil {
			log.Fatal(err)
		}

		node := utils.NewProtoNode()
		for _, ff := range files {
			newRelativeFilename := path.Join(relativeFilename, ff.Name())
			if ignore(newRelativeFilename) {
				// Nothing
			} else {
				hash := traverse(base, newRelativeFilename, f)
				utils.SetLink(node, ff.Name(), hash)
			}
		}

		err = f(relativeFilename, node)
		if err != nil {
			log.Fatal(err)
		}

		return node.Cid()
		// } else if fileInfo.Mode() == os.ModeSymlink {
		// skip
	} else {
		bytes, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}
		node, err := utils.ParseRawNode(bytes)
		if err != nil {
			log.Fatal(err)
		}

		err = f(relativeFilename, node)
		if err != nil {
			log.Fatal(err)
		}

		return node.Cid()
	}
}

func ignore(p string) bool {
	return filepath.Base(p) == ".git"
}
