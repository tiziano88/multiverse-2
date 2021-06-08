package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strconv"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/tiziano88/multiverse/utils"
)

// const apiURL = "01.plus"

const apiURL = "localhost:8080"

const webURL = "www." + apiURL

func main() {
	flag.Parse()
	target := flag.Arg(0)
	hash := traverse(target)
	fmt.Printf("%s %s\n", hash, target)
	log.Printf("http://%s/%s", apiURL, hash)
}

func upload(node format.Node) (cid.Cid, error) {
	localHash := node.Cid()
	if !exists(localHash) {
		b := node.RawData()
		res, err := http.Post("http://"+apiURL+"/upload?codec="+strconv.Itoa(int(localHash.Prefix().Codec)), "", bytes.NewReader(b))
		if err != nil {
			return cid.Undef, fmt.Errorf("could not POST request: %v", err)
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not read response body: %v", err)
		}
		log.Printf("uploaded: %s", string(body))
		remoteHash := string(body)
		if localHash.String() != remoteHash {
			return cid.Undef, fmt.Errorf("hash mismatch; local: %s, remote: %s", localHash, remoteHash)
		}
		log.Printf("http://%s/%s", apiURL, localHash)
	}
	return localHash, nil
}

func exists(hash cid.Cid) bool {
	res, err := http.Get("http://" + apiURL + "/blobs/" + hash.String() + "?stat=1")
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

func traverse(p string) cid.Cid {
	fmt.Printf(": %s\n", p)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		log.Fatal(err)
	}

	node := utils.NewProtoNode()

	for _, file := range files {
		if file.IsDir() {
			hash := traverse(path.Join(p, file.Name()))
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

			hash, err := upload(newNode)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%s %s\n", hash, file.Name())
			utils.SetLink(node, file.Name(), newNode.Cid())
		}
	}

	hash, err := upload(node)
	if err != nil {
		log.Fatal(err)
	}

	return hash
}
