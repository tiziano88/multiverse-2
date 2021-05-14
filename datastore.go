package main

import (
	"context"
	"io/ioutil"
	"os"
	"path"

	"cloud.google.com/go/storage"
)

type DataStore interface {
	Set(ctx context.Context, name string, value []byte) error
	Get(ctx context.Context, name string) ([]byte, error)
	Has(ctx context.Context, name string) (bool, error)
}

type FileDataStore struct {
	dirName string
}

func (s FileDataStore) Set(ctx context.Context, name string, value []byte) error {
	return ioutil.WriteFile(path.Join(s.dirName, name), value, 0644)
}

func (s FileDataStore) Get(ctx context.Context, name string) ([]byte, error) {
	return ioutil.ReadFile(path.Join(s.dirName, name))
}

func (s FileDataStore) Has(ctx context.Context, name string) (bool, error) {
	_, err := os.Stat(path.Join(s.dirName, name))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

type CloudDataStore struct {
	client     *storage.Client
	bucketName string
}

func (s CloudDataStore) Set(ctx context.Context, name string, value []byte) error {
	wc := s.client.Bucket(s.bucketName).Object(name).NewWriter(ctx)
	defer wc.Close()
	_, err := wc.Write(value)
	return err
}

func (s CloudDataStore) Get(ctx context.Context, name string) ([]byte, error) {
	rc, err := s.client.Bucket(s.bucketName).Object(name).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	body, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s CloudDataStore) Has(ctx context.Context, name string) (bool, error) {
	// TODO: return size
	_, err := s.client.Bucket(s.bucketName).Object(name).Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}
