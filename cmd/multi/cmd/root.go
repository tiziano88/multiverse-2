package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/spf13/cobra"
	"github.com/tiziano88/multiverse/datastore"
	"github.com/tiziano88/multiverse/nodeservice"
	"github.com/tiziano88/multiverse/utils"
)

var (
	blobStore nodeservice.NodeService
	tagStore  datastore.DataStore
)

type Config struct {
	DefaultRemote string `toml:"default_remote"`
	Remotes       map[string]Remote
}

type Remote struct {
	Path string
	URL  string
}

func InitRemote(remote Remote) {
	if remote.URL != "" {
		blobStore = nodeservice.Remote{
			APIURL: remote.URL,
		}
	} else if remote.Path != "" {
		baseDir := remote.Path

		{
			multiverseBlobCacheDir := filepath.Join(baseDir, "blobs")
			err := os.MkdirAll(multiverseBlobCacheDir, 0755)
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
			multiverseTagsCacheDir := filepath.Join(baseDir, "tags")
			err := os.MkdirAll(multiverseTagsCacheDir, 0755)
			if err != nil {
				log.Fatalf("could not create tag cache dir: %v", err)
			}
			tagStore = datastore.File{
				DirName: multiverseTagsCacheDir,
			}
		}
	}
}

var rootCmd = &cobra.Command{
	Use: "multi",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		s, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("could not load config dir: %v", err)
		}
		s = filepath.Join(s, "multiverse.toml")
		f, err := ioutil.ReadFile(s)
		if err != nil {
			log.Printf("could not read config: %v", err)
			// Continue anyways.
		}
		config := Config{}
		err = toml.Unmarshal(f, &config)
		if err != nil {
			log.Fatalf("could not parse config: %v", err)
		}
		log.Printf("parsed config: %#v", config)

		if remoteName == "" && config.DefaultRemote != "" {
			remoteName = config.DefaultRemote
		}

		remote, ok := config.Remotes[remoteName]
		if !ok {
			log.Fatalf("Invalid remote name: %q", remoteName)
		}
		InitRemote(remote)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var (
	remoteName string
	tagName    string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&remoteName, "remote", "", "")

	pushCmd.Flags().StringVar(&tagName, "tag", "", "")

	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(statusCmd)
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

func push(filename string, node format.Node) error {
	localHash := node.Cid()
	if !exists(localHash) {
		return blobStore.Add(context.Background(), node)
	}
	return nil
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

func exists(hash cid.Cid) bool {
	ok, err := blobStore.Has(context.Background(), hash)
	if err != nil {
		log.Fatal(err)
	}
	return ok
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