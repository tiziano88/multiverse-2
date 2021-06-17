package nodeservice

import (
	"context"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

// https://github.com/ipfs/go-ipld-format/blob/579737706ba5da3e550111621e2ab1bf122ed53f/merkledag.go
type NodeService interface {
	Has(context.Context, cid.Cid) (bool, error)
	// Get(context.Context, cid.Cid) (format.Node, error)
	// Add(context.Context, format.Node) error
	format.DAGService
}
