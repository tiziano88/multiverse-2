//
// Copyright 2021 The Multiverse Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
