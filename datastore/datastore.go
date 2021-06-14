package datastore

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"cloud.google.com/go/storage"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/tiziano88/multiverse/utils"
)

type DataStore interface {
	Set(ctx context.Context, name string, value []byte) error
	Get(ctx context.Context, name string) ([]byte, error)
	Has(ctx context.Context, name string) (bool, error)
}

type FileDataStore struct {
	DirName string
}

func (s FileDataStore) Set(ctx context.Context, name string, value []byte) error {
	return ioutil.WriteFile(path.Join(s.DirName, name), value, 0644)
}

func (s FileDataStore) Get(ctx context.Context, name string) ([]byte, error) {
	return ioutil.ReadFile(path.Join(s.DirName, name))
}

func (s FileDataStore) Has(ctx context.Context, name string) (bool, error) {
	_, err := os.Stat(path.Join(s.DirName, name))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

type CloudDataStore struct {
	Client     *storage.Client
	BucketName string
}

func (s CloudDataStore) Set(ctx context.Context, name string, value []byte) error {
	wc := s.Client.Bucket(s.BucketName).Object(name).NewWriter(ctx)
	defer wc.Close()
	_, err := wc.Write(value)
	return err
}

func (s CloudDataStore) Get(ctx context.Context, name string) ([]byte, error) {
	rc, err := s.Client.Bucket(s.BucketName).Object(name).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	body, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s CloudDataStore) Has(ctx context.Context, name string) (bool, error) {
	// TODO: return size
	_, err := s.Client.Bucket(s.BucketName).Object(name).Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

// https://github.com/ipfs/go-ipld-format/blob/579737706ba5da3e550111621e2ab1bf122ed53f/merkledag.go
type NodeService interface {
	Has(context.Context, cid.Cid) (bool, error)
	Get(context.Context, cid.Cid) (format.Node, error)
	Add(context.Context, format.Node) (cid.Cid, error)
}

type Adaptor struct {
	Inner DataStore
}

func (s Adaptor) Has(ctx context.Context, hash cid.Cid) (bool, error) {
	return s.Inner.Has(ctx, hash.String())
}

func (s Adaptor) Get(ctx context.Context, hash cid.Cid) (format.Node, error) {
	bytes, err := s.Inner.Get(ctx, hash.String())
	if err != nil {
		return nil, err
	}
	switch hash.Prefix().Codec {
	case cid.DagProtobuf:
		return utils.ParseProtoNode(bytes)
	case cid.Raw:
		return utils.ParseRawNode(bytes)
	default:
		return nil, fmt.Errorf("invalid codec")
	}
}

func (s Adaptor) Add(ctx context.Context, node format.Node) (cid.Cid, error) {
	err := s.Inner.Set(ctx, node.Cid().String(), node.RawData())
	return node.Cid(), err
}
