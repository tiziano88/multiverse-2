package utils

import (
	"log"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	mh "github.com/multiformats/go-multihash"
)

func HashRaw(data []byte) cid.Cid {
	// Create a cid manually by specifying the 'prefix' parameters
	// https://github.com/ipfs/go-cid
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.SHA2_256,
		MhLength: -1, // default length
	}
	c, err := pref.Sum(data)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func HashCodec(codec uint64, data []byte) cid.Cid {
	// Create a cid manually by specifying the 'prefix' parameters
	// https://github.com/ipfs/go-cid
	pref := cid.Prefix{
		Version:  1,
		Codec:    codec,
		MhType:   mh.SHA2_256,
		MhLength: -1, // default length
	}
	c, err := pref.Sum(data)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func HashNode(node *merkledag.ProtoNode) cid.Cid {
	return node.Cid()
}

func GetLink(node *merkledag.ProtoNode, name string) (cid.Cid, error) {
	link, err := node.GetNodeLink(name)
	if err != nil {
		return cid.Undef, err
	}
	return link.Cid, nil
}

func SetLink(node *merkledag.ProtoNode, name string, hash cid.Cid) error {
	for _, l := range node.GetPBNode().Links {
		if l.GetName() == name {
			l.Hash = hash.Bytes()
			return nil
		}
	}
	return node.AddRawLink(name, &format.Link{
		Cid: hash,
	})
}
