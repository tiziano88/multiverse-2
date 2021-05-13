package utils

import (
	"fmt"
	"log"

	"github.com/ipfs/go-cid"
	merkledag_pb "github.com/ipfs/go-merkledag/pb"
	mh "github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/proto"
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

func HashNode(node *merkledag_pb.PBNode) cid.Cid {
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.DagProtobuf,
		MhType:   mh.SHA2_256,
		MhLength: -1, // default length
	}
	data, err := node.Marshal()
	if err != nil {
		log.Fatal(err)
	}
	c, err := pref.Sum(data)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func GetLink(node *merkledag_pb.PBNode, name string) (cid.Cid, error) {
	for _, l := range node.Links {
		if l.GetName() == name {
			_, c, err := cid.CidFromBytes(l.Hash)
			return c, err
		}
	}
	return cid.Undef, fmt.Errorf("not found")
}

func SetLink(node *merkledag_pb.PBNode, name string, hash cid.Cid) {
	for _, l := range node.Links {
		if l.GetName() == name {
			l.Hash = hash.Bytes()
			return
		}
	}
	node.Links = append(node.Links, &merkledag_pb.PBLink{
		Name: proto.String(name),
		Hash: hash.Bytes(),
	})
}
