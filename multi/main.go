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

	"github.com/fatih/color"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/tiziano88/multiverse/datastore"
	"github.com/tiziano88/multiverse/utils"
)

var (
	blobStore datastore.DataStore
	tagStore  datastore.DataStore
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
		log.Fatalf("could not get user cache dir: %v", err)
	}

	{
		multiverseBlobCacheDir := filepath.Join(userCacheDir, "multiverse", "blobs")
		err = os.MkdirAll(multiverseBlobCacheDir, 0755)
		if err != nil {
			log.Fatalf("could not create blob cache dir: %v", err)
		}
		blobStore = datastore.FileDataStore{
			DirName: multiverseBlobCacheDir,
		}
	}

	{
		multiverseTagsCacheDir := filepath.Join(userCacheDir, "multiverse", "tags")
		err = os.MkdirAll(multiverseTagsCacheDir, 0755)
		if err != nil {
			log.Fatalf("could not create tag cache dir: %v", err)
		}
		tagStore = datastore.FileDataStore{
			DirName: multiverseTagsCacheDir,
		}
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
		log.Fatalf("invalid command: %s", os.Args[1])
	}

	// flag.Parse()
}

func status(filename string, node format.Node) error {
	marker := color.RedString("*")
	ok, _ := blobStore.Has(context.Background(), node.Cid().String())
	if ok {
		marker = color.GreenString("✓")
	}
	fmt.Printf("%s %s %s\n", color.YellowString(node.Cid().String()), marker, filename)
	return nil
}

func local(filename string, node format.Node) error {
	fmt.Printf("%s %s\n", node.Cid().String(), filename)
	return blobStore.Set(context.Background(), node.Cid().String(), node.RawData())
}

func upload(filename string, node format.Node) error {
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

func traverse(p string, f func(string, format.Node) error) cid.Cid {
	file, err := os.Open(p)
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
			filePath := path.Join(p, ff.Name())
			hash := traverse(filePath, f)
			utils.SetLink(node, ff.Name(), hash)
		}

		err = f(file.Name(), node)
		if err != nil {
			log.Fatal(err)
		}

		return node.Cid()
	} else {
		bytes, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}
		node, err := utils.ParseRawNode(bytes)
		if err != nil {
			log.Fatal(err)
		}

		err = f(file.Name(), node)
		if err != nil {
			log.Fatal(err)
		}

		return node.Cid()
	}
}
