package nodeservice

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/tiziano88/multiverse/datastore"
	"github.com/tiziano88/multiverse/utils"
)

type DataStore struct {
	Inner datastore.DataStore
}

func (s DataStore) Has(ctx context.Context, hash cid.Cid) (bool, error) {
	return s.Inner.Has(ctx, hash.String())
}

func (s DataStore) Get(ctx context.Context, hash cid.Cid) (format.Node, error) {
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

func (s DataStore) GetMany(ctx context.Context, hashes []cid.Cid) <-chan *format.NodeOption {
	return nil
}

func (s DataStore) Add(ctx context.Context, node format.Node) error {
	err := s.Inner.Set(ctx, node.Cid().String(), node.RawData())
	return err
}

func (s DataStore) AddMany(ctx context.Context, nodes []format.Node) error {
	return fmt.Errorf("not implemented")
}

func (s DataStore) Remove(ctx context.Context, hash cid.Cid) error {
	return fmt.Errorf("not implemented")
}

func (s DataStore) RemoveMany(ctx context.Context, hashes []cid.Cid) error {
	return fmt.Errorf("not implemented")
}
