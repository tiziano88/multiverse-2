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
	merkledag_pb "github.com/ipfs/go-merkledag/pb"
	"github.com/tiziano88/multiverse/utils"
)

// const apiURL = "01.plus"

const apiURL = "localhost:8080"

const webURL = "web." + apiURL

func main() {
	flag.Parse()
	target := flag.Arg(0)
	hash := traverse(target)
	fmt.Printf("%s %s\n", hash, target)
	log.Printf("http://%s.%s", hash, webURL)
}

func uploadRaw(b []byte) (cid.Cid, error) {
	res, err := http.Post("http://"+apiURL+"/upload", "", bytes.NewReader(b))
	if err != nil {
		return cid.Undef, fmt.Errorf("could not POST request: %v", err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not read response body: %v", err)
	}
	log.Printf("uploaded: %s", string(body))
	localHash := utils.HashRaw(b)
	remoteHash := string(body)
	if localHash.String() != remoteHash {
		return cid.Undef, fmt.Errorf("hash mismatch; local: %s, remote: %s", localHash, remoteHash)
	}
	log.Printf("http://%s.%s", localHash, webURL)
	return localHash, nil
}

func uploadNode(node *merkledag_pb.PBNode) (cid.Cid, error) {
	b, err := node.Marshal()
	if err != nil {
		return cid.Undef, fmt.Errorf("could not marshal node: %v", err)
	}
	res, err := http.Post("http://"+apiURL+"/upload?codec="+strconv.Itoa(cid.DagProtobuf), "", bytes.NewReader(b))
	if err != nil {
		return cid.Undef, fmt.Errorf("could not POST request: %v", err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not read response body: %v", err)
	}
	log.Printf("uploaded: %s", string(body))
	localHash := utils.HashNode(node)
	remoteHash := string(body)
	if localHash.String() != remoteHash {
		return cid.Undef, fmt.Errorf("hash mismatch; local: %s, remote: %s", localHash, remoteHash)
	}
	log.Printf("http://%s.%s", localHash, webURL)
	return localHash, nil
}

func traverse(p string) cid.Cid {
	fmt.Printf(": %s\n", p)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		log.Fatal(err)
	}

	node := merkledag_pb.PBNode{}

	for _, file := range files {
		if file.IsDir() {
			hash := traverse(path.Join(p, file.Name()))
			fmt.Printf("%s %s\n", hash, file.Name())
			utils.SetLink(&node, file.Name(), hash)
		} else {
			hash := hashFile(path.Join(p, file.Name()))
			fmt.Printf("%s %s\n", hash, file.Name())
			utils.SetLink(&node, file.Name(), hash)
		}
	}

	hash, err := uploadNode(&node)
	if err != nil {
		log.Fatal(err)
	}

	return hash
}

func hashFile(p string) cid.Cid {
	bytes, err := ioutil.ReadFile(p)
	if err != nil {
		log.Fatal(err)
	}
	hash, err := uploadRaw(bytes)
	if err != nil {
		log.Fatal(err)
	}
	return hash
}
