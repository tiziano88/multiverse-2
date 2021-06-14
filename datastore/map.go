package datastore

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

type MapAdaptor struct {
	Inner map[cid.Cid]format.Node
}

func (s MapAdaptor) Has(ctx context.Context, hash cid.Cid) (bool, error) {
	_, ok := s.Inner[hash]
	return ok, nil
}

func (s MapAdaptor) Get(ctx context.Context, hash cid.Cid) (format.Node, error) {
	node, ok := s.Inner[hash]
	if ok {
		return node, nil
	} else {
		return nil, fmt.Errorf("not found")
	}
}

func (s MapAdaptor) GetMany(ctx context.Context, hashes []cid.Cid) <-chan *format.NodeOption {
	return nil
}

func (s MapAdaptor) Add(ctx context.Context, node format.Node) error {
	s.Inner[node.Cid()] = node
	return nil
}

func (s MapAdaptor) AddMany(ctx context.Context, nodes []format.Node) error {
	return fmt.Errorf("not implemented")
}

func (s MapAdaptor) Remove(ctx context.Context, hash cid.Cid) error {
	return fmt.Errorf("not implemented")
}

func (s MapAdaptor) RemoveMany(ctx context.Context, hashes []cid.Cid) error {
	return fmt.Errorf("not implemented")
}
