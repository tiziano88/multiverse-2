package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"google.golang.org/appengine"
)

type Manifest struct {
	Target    string
	Overrides map[string]string
}

var dataStore DataStore

const bucketName = "multiverse-312721.appspot.com"

const cidDir = 0x88

func main() {
	ctx := context.Background()
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Print(err)
		dataStore = FileDataStore{}
	} else {
		dataStore = CloudDataStore{
			client: storageClient,
		}
	}

	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.GET("/", indexHandler)
	router.GET("/render_sw.js", swHandler)
	router.GET("/snapshot_sw.js", snapshotSwHandler)
	router.POST("/upload", uploadHandler)
	router.GET("/snapshot", snapshotHandler)
	router.GET("/h/:hash", hashHandler)

	router.GET("/web/:hash/*path", renderHandler)
	router.GET("/web/:hash", renderHandler)

	router.GET("/proxy", proxyHandler)

	router.Run()

	appengine.Main()
}

func indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.tmpl", gin.H{})
}

func swHandler(c *gin.Context) {
	c.File("./templates/render_sw.js")
}

func snapshotSwHandler(c *gin.Context) {
	c.File("./templates/snapshot_sw.js")
}

func uploadHandler(c *gin.Context) {
	// See https://github.com/gin-gonic/gin#upload-files .
	form, err := c.MultipartForm()
	log.Printf("form: %v", form)
	dir := form.File["directory"]
	if dir != nil {
		for f := range dir {
			log.Printf("dir: %d %v", f, dir[f].Filename)
		}
	}
	file, err := c.FormFile("file")
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	log.Println(file.Filename)
	mpFile, err := file.Open()
	if err != nil {
		log.Fatal(err)
	}
	b, err := ioutil.ReadAll(mpFile)
	if err != nil {
		log.Fatal(err)
	}
	h := add(c, b)
	c.String(http.StatusOK, "%s", h)
}

func add(c context.Context, blob []byte) string {
	h := hash(blob)
	err := dataStore.Set(c, h, blob)
	if err != nil {
		log.Fatal(err)
	}
	return h
}

func hashHandler(c *gin.Context) {
	hash := c.Param("hash")
	b, err := dataStore.Get(c, hash)
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	c.Data(http.StatusOK, "", b)
}

func renderHandler(c *gin.Context) {
	hash := c.Param("hash")
	path := strings.Split(c.Param("path"), "/")
	log.Printf("path: %s", path)
	c.HTML(http.StatusOK, "render.tmpl", gin.H{
		"hash": hash,
		"path": path,
	})
}

func proxyHandler(c *gin.Context) {
	target := c.Query("target")
	log.Printf("proxy: %s", target)
	res, err := http.Get(target)
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	blob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	h := add(c, blob)
	log.Printf("%s -> %s", target, h)
	c.String(http.StatusOK, "%s", h)
}

func snapshotHandler(c *gin.Context) {
	target := c.Query("target")
	log.Printf("snapshot target: %s", target)
	/*
		res, err := http.Get(target)
		if err != nil {
			log.Printf("could not fetch %s: %s", target, err)
			c.Abort()
			return
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("could not fetch %s: %s", target, err)
			c.Abort()
			return
		}
	*/
	c.HTML(http.StatusOK, "snapshot.tmpl", gin.H{
		"target": target,
	})
}

func hash(data []byte) string {
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
	return c.String()
}

type DataStore interface {
	Set(ctx context.Context, name string, value []byte) error
	Get(ctx context.Context, name string) ([]byte, error)
}

type FileDataStore struct{}

func (s FileDataStore) Set(ctx context.Context, name string, value []byte) error {
	return ioutil.WriteFile("./data/"+name, value, 0644)
}

func (s FileDataStore) Get(ctx context.Context, name string) ([]byte, error) {
	return ioutil.ReadFile("./data/" + name)
}

type CloudDataStore struct {
	client *storage.Client
}

func (s CloudDataStore) Set(ctx context.Context, name string, value []byte) error {
	wc := s.client.Bucket(bucketName).Object(name).NewWriter(ctx)
	defer wc.Close()
	_, err := wc.Write(value)
	return err
}

func (s CloudDataStore) Get(ctx context.Context, name string) ([]byte, error) {
	rc, err := s.client.Bucket(bucketName).Object(name).NewReader(ctx)
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
