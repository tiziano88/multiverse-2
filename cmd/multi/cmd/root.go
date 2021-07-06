package cmd

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/spf13/cobra"
	"github.com/tiziano88/multiverse/datastore"
	"github.com/tiziano88/multiverse/nodeservice"
	"github.com/tiziano88/multiverse/tagstore"
	"github.com/tiziano88/multiverse/utils"
)

var (
	blobStore nodeservice.NodeService
	tagStore  tagstore.TagStore
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
			tagStore = tagstore.File{
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
		// log.Printf("parsed config: %#v", config)

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
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(tagsCmd)
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
