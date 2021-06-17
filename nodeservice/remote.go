package nodeservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

type Remote struct {
	APIURL string
}

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

func (s Remote) Has(ctx context.Context, hash cid.Cid) (bool, error) {
	r := GetRequest{
		Root: hash.String(),
		Path: "",
	}
	buf := bytes.Buffer{}
	json.NewEncoder(&buf).Encode(r)
	res, err := http.Post("http://"+s.APIURL+"/api/get", "", &buf)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode == http.StatusOK {
		return true, nil
	}
	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("invalid status code: %d", res.StatusCode)
}

func (s Remote) Get(ctx context.Context, hash cid.Cid) (format.Node, error) {
	return nil, fmt.Errorf("not found")
}

func (s Remote) GetMany(ctx context.Context, hashes []cid.Cid) <-chan *format.NodeOption {
	return nil
}

func (s Remote) Add(ctx context.Context, node format.Node) error {
	blobType := ""
	switch node.Cid().Prefix().Codec {
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
	res, err := http.Post("http://"+s.APIURL+"/api/update", "", &buf)
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
	if node.Cid().String() != remoteHash {
		return fmt.Errorf("hash mismatch; local: %s, remote: %s", node.Cid().String(), remoteHash)
	}
	return nil
}

func (s Remote) AddMany(ctx context.Context, nodes []format.Node) error {
	return fmt.Errorf("not implemented")
}

func (s Remote) Remove(ctx context.Context, hash cid.Cid) error {
	return fmt.Errorf("not implemented")
}

func (s Remote) RemoveMany(ctx context.Context, hashes []cid.Cid) error {
	return fmt.Errorf("not implemented")
}
