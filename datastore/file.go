package datastore

import (
	"context"
	"io/ioutil"
	"os"
	"path"
)

// File is an implementation of DataStore using the local file system, rooted at the
// specified directory.
type File struct {
	DirName string
}

func (s File) Set(ctx context.Context, name string, value []byte) error {
	return ioutil.WriteFile(path.Join(s.DirName, name), value, 0644)
}

func (s File) Get(ctx context.Context, name string) ([]byte, error) {
	return ioutil.ReadFile(path.Join(s.DirName, name))
}

func (s File) Has(ctx context.Context, name string) (bool, error) {
	_, err := os.Stat(path.Join(s.DirName, name))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}
