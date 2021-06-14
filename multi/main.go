package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/tiziano88/multiverse/datastore"
	"github.com/tiziano88/multiverse/utils"
)

var (
	blobStore datastore.DataStore
)

// const apiURL = "01.plus"

const apiURL = "localhost:8080"

const webURL = "www." + apiURL

type UploadRequest struct {
	Root  string
	Blobs []UploadBlob
}

type UploadBlob struct {
	Type    string // file | directory
	Path    string
	Content []byte
}

type UploadResponse struct {
	Root string
}

type GetRequest struct {
	Root string
	Path string
}

type GetResponse struct {
	Content []byte
}

func main() {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		fmt.Printf("could not get cache dir: %v", err)
		os.Exit(1)
	}

	multiverseBlobCacheDir := filepath.Join(userCacheDir, "multiverse", "blobs")
	err = os.MkdirAll(multiverseBlobCacheDir, 0755)
	if err != nil {
		fmt.Printf("could not create cache dir: %v", err)
		os.Exit(1)
	}

	blobStore = datastore.FileDataStore{
		DirName: multiverseBlobCacheDir,
	}

	if len(os.Args) < 2 {
		fmt.Println("expected command")
		os.Exit(1)
	}

	// https://gobyexample.com/command-line-subcommands
	switch os.Args[1] {
	case "upload":
		set := flag.NewFlagSet("upload", flag.ExitOnError)
		set.Parse(os.Args[2:])
		target := set.Arg(0)
		hash := traverse(target, upload)
		fmt.Printf("%s %s\n", hash, target)
		log.Printf("http://%s/blobs/%s", apiURL, hash)

	case "local":
		set := flag.NewFlagSet("local", flag.ExitOnError)
		set.Parse(os.Args[2:])
		target := set.Arg(0)
		hash := traverse(target, local)
		fmt.Printf("%s %s\n", hash, target)
		log.Printf("http://%s/blobs/%s", apiURL, hash)

	case "status":
		set := flag.NewFlagSet("status", flag.ExitOnError)
		set.Parse(os.Args[2:])
		target := set.Arg(0)
		hash := traverse(target, status)
		fmt.Printf("%s %s\n", hash, target)
		log.Printf("http://%s/blobs/%s", apiURL, hash)

	default:
		fmt.Println("invalid command: ", os.Args[1])
		os.Exit(1)
	}

	// flag.Parse()
}

func status(node format.Node) error {
	return nil
}

func local(node format.Node) error {
	return blobStore.Set(context.Background(), node.Cid().String(), node.RawData())
}

func upload(node format.Node) error {
	localHash := node.Cid()
	if !exists(localHash) {
		blobType := ""
		switch localHash.Prefix().Codec {
		case cid.Raw:
			blobType = "file"
		case cid.DagProtobuf:
			blobType = "directory"
		}
		r := UploadRequest{
			Root: "",
			Blobs: []UploadBlob{
				{
					Type:    blobType,
					Path:    "",
					Content: node.RawData(),
				},
			},
		}
		buf := bytes.Buffer{}
		json.NewEncoder(&buf).Encode(r)
		res, err := http.Post("http://"+apiURL+"/api/update", "", &buf)
		if err != nil {
			return fmt.Errorf("could not POST request: %v", err)
		}
		resJson := UploadResponse{}
		err = json.NewDecoder(res.Body).Decode(&resJson)
		if err != nil {
			return fmt.Errorf("could not read response body: %v", err)
		}
		log.Printf("uploaded: %#v", resJson)
		remoteHash := resJson.Root
		if localHash.String() != remoteHash {
			return fmt.Errorf("hash mismatch; local: %s, remote: %s", localHash, remoteHash)
		}
		log.Printf("http://%s/blobs/%s", apiURL, localHash)
	}
	return nil
}

func exists(hash cid.Cid) bool {
	r := GetRequest{
		Root: hash.String(),
		Path: "",
	}
	buf := bytes.Buffer{}
	json.NewEncoder(&buf).Encode(r)
	res, err := http.Post("http://"+apiURL+"/api/get", "", &buf)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode == http.StatusOK {
		return true
	}
	if res.StatusCode == http.StatusNotFound {
		return false
	}
	log.Fatalf("invalid status: %s %d", res.Status, res.StatusCode)
	return false
}

func traverse(p string, f func(format.Node) error) cid.Cid {
	fmt.Printf(": %s\n", p)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		log.Fatal(err)
	}

	node := utils.NewProtoNode()

	for _, file := range files {
		if file.IsDir() {
			hash := traverse(path.Join(p, file.Name()), f)
			fmt.Printf("%s %s\n", hash, file.Name())
			utils.SetLink(node, file.Name(), hash)
		} else {
			filePath := path.Join(p, file.Name())
			bytes, err := ioutil.ReadFile(filePath)
			if err != nil {
				log.Fatal(err)
			}
			newNode, err := utils.ParseRawNode(bytes)
			if err != nil {
				log.Fatal(err)
			}

			hash := node.Cid()
			fmt.Printf("%s %s\n", hash, file.Name())

			err = f(newNode)
			if err != nil {
				log.Fatal(err)
			}

			utils.SetLink(node, file.Name(), newNode.Cid())
		}
	}

	hash := node.Cid()

	err = f(node)
	if err != nil {
		log.Fatal(err)
	}

	return hash
}
