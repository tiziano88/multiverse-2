package datastore

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

type Multi struct {
	Inner []NodeService
}

func (s Multi) Has(ctx context.Context, hash cid.Cid) (bool, error) {
	for _, i := range s.Inner {
		ok, _ := i.Has(ctx, hash)
		if ok {
			return ok, nil
		}
	}
	return false, nil
}

func (s Multi) Get(ctx context.Context, hash cid.Cid) (format.Node, error) {
	for _, i := range s.Inner {
		node, err := i.Get(ctx, hash)
		if err != nil {
			continue
		}
		return node, nil
	}
	return nil, fmt.Errorf("not found")
}

func (s Multi) GetMany(ctx context.Context, hashes []cid.Cid) <-chan *format.NodeOption {
	return nil
}

func (s Multi) Add(ctx context.Context, node format.Node) error {
	return s.Inner[0].Add(ctx, node)
}

func (s Multi) AddMany(ctx context.Context, nodes []format.Node) error {
	return fmt.Errorf("not implemented")
}

func (s Multi) Remove(ctx context.Context, hash cid.Cid) error {
	return fmt.Errorf("not implemented")
}

func (s Multi) RemoveMany(ctx context.Context, hashes []cid.Cid) error {
	return fmt.Errorf("not implemented")
}
