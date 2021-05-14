package utils

import (
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
)

func NewProtoNode() *merkledag.ProtoNode {
	node := merkledag.ProtoNode{}
	node.SetCidBuilder(merkledag.V1CidPrefix())
	return &node
}

func ParseProtoNode(b []byte) (*merkledag.ProtoNode, error) {
	node, err := merkledag.DecodeProtobuf(b)
	if err != nil {
		return nil, err
	}
	node.SetCidBuilder(merkledag.V1CidPrefix())
	return node, nil
}

func ParseRawNode(b []byte) (*merkledag.RawNode, error) {
	node, err := merkledag.NewRawNodeWPrefix(b, merkledag.V1CidPrefix())
	if err != nil {
		return nil, err
	}
	return node, nil
}

func GetLink(node *merkledag.ProtoNode, name string) (cid.Cid, error) {
	link, err := node.GetNodeLink(name)
	if err != nil {
		return cid.Undef, err
	}
	return link.Cid, nil
}

func SetLink(node *merkledag.ProtoNode, name string, hash cid.Cid) error {
	node.RemoveNodeLink(name) // Ignore errors
	return node.AddRawLink(name, &format.Link{
		Cid: hash,
	})
}
