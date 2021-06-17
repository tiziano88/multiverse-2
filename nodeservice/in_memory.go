package nodeservice

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

type InMemory struct {
	Inner map[cid.Cid]format.Node
}

func (s InMemory) Has(ctx context.Context, hash cid.Cid) (bool, error) {
	_, ok := s.Inner[hash]
	return ok, nil
}

func (s InMemory) Get(ctx context.Context, hash cid.Cid) (format.Node, error) {
	node, ok := s.Inner[hash]
	if ok {
		return node, nil
	} else {
		return nil, fmt.Errorf("not found")
	}
}

func (s InMemory) GetMany(ctx context.Context, hashes []cid.Cid) <-chan *format.NodeOption {
	return nil
}

func (s InMemory) Add(ctx context.Context, node format.Node) error {
	s.Inner[node.Cid()] = node
	return nil
}

func (s InMemory) AddMany(ctx context.Context, nodes []format.Node) error {
	return fmt.Errorf("not implemented")
}

func (s InMemory) Remove(ctx context.Context, hash cid.Cid) error {
	return fmt.Errorf("not implemented")
}

func (s InMemory) RemoveMany(ctx context.Context, hashes []cid.Cid) error {
	return fmt.Errorf("not implemented")
}
