package datastore

import (
	"context"
)

// DataStore is an interface defining low-level operations for handling unstructured key/value
// pairs. At this level, there is no concept of hashes or any structure of the values.
type DataStore interface {
	Set(ctx context.Context, name string, value []byte) error
	Get(ctx context.Context, name string) ([]byte, error)
	// TODO: return size
	Has(ctx context.Context, name string) (bool, error)
}
