package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"google.golang.org/appengine"
)

type Manifest struct {
	Overrides map[string]string
}

var dataStore DataStore

const bucketName = "multiverse-312721.appspot.com"

var webSuffix = ".web.localhost:8080"

const cidDir = 0x88

func main() {
	webSuffixEnv := os.Getenv("WEB_SUFFIX")
	if webSuffixEnv != "" {
		webSuffix = webSuffixEnv
	}
	log.Printf("web suffix: %#v", webSuffix)

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
	// router.GET("/", indexHandler)
	// router.GET("/render_sw.js", swHandler)
	// router.GET("/snapshot_sw.js", snapshotSwHandler)
	// TODO: individual upload vs in-tree
	router.POST("/*path", uploadHandler)
	// router.GET("/snapshot", snapshotHandler)
	// router.GET("/h/:hash", hashHandler)

	router.GET("/*path", renderHandler)

	// router.GET("/proxy", proxyHandler)

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
	log.Printf("filename: %s", file.Filename)
	mpFile, err := file.Open()
	if err != nil {
		log.Fatal(err)
	}
	b, err := ioutil.ReadAll(mpFile)
	if err != nil {
		log.Fatal(err)
	}

	host := c.Request.Host
	pathString := c.Param("path")
	pathString = strings.TrimPrefix(pathString, "/")
	segments := strings.Split(pathString, "/")
	log.Printf("host: %v", host)
	log.Printf("path: %v", pathString)
	log.Printf("segments: %#v", segments)

	hash := ""

	if strings.HasSuffix(host, webSuffix) {
		hash = strings.TrimSuffix(host, webSuffix)
		log.Printf("hash: %v", hash)
	}

	segments = append(segments, file.Filename)

	h, err := traverseAdd(c, hash, segments, b)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	targetURL := fmt.Sprintf("http://%s%s%s", h, webSuffix, c.Param("path"))
	// c.String(http.StatusOK, "%s", targetURL)
	c.Redirect(http.StatusFound, targetURL)
}

func add(c context.Context, blob []byte) string {
	h := hash(blob)
	err := dataStore.Set(c, h, blob)
	if err != nil {
		log.Fatal(err)
	}
	return h
}

func get(c context.Context, hash string) ([]byte, error) {
	return dataStore.Get(c, hash)
}

func traverse(c context.Context, base string, segments []string) (string, error) {
	if len(segments) == 0 || segments[0] == "" {
		return base, nil
	} else {
		manifest := Manifest{}
		bytes, err := get(c, base)
		if err != nil {
			return "", fmt.Errorf("could not get blob %s", base)
		}
		err = json.Unmarshal(bytes, &manifest)
		if err != nil {
			return "", fmt.Errorf("could not parse blob %s as manifest", base)
		}
		head := segments[0]
		next := manifest.Overrides[head]
		if next == "" {
			return "", fmt.Errorf("could not traverse %s.%s", base, head)
		}
		return traverse(c, next, segments[1:])
	}
}

func traverseAdd(c context.Context, base string, segments []string, blob []byte) (string, error) {
	log.Printf("base: %v", base)
	log.Printf("segments: %#v", segments)

	manifest := Manifest{}
	bytes, err := get(c, base)
	if err != nil {
		return "", fmt.Errorf("could not get blob %s", base)
	}
	err = json.Unmarshal(bytes, &manifest)
	if err != nil {
		return "", fmt.Errorf("could not parse blob %s as manifest: %v", base, err)
	}

	if manifest.Overrides == nil {
		manifest.Overrides = make(map[string]string)
	}

	head := segments[0]

	if head == "" {
		return traverseAdd(c, base, segments[1:], blob)
	}

	if len(segments) == 1 {
		newHash := add(c, blob)
		manifest.Overrides[head] = newHash
	} else {
		next := manifest.Overrides[head]
		log.Printf("next: %v", next)

		newHash, err := traverseAdd(c, next, segments[1:], blob)
		if err != nil {
			return "", fmt.Errorf("could not call recursively: %v", err)
		}

		manifest.Overrides[head] = newHash
	}

	newManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal new manifest: %v", err)
	}

	return add(c, newManifest), nil
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
	host := c.Request.Host
	pathString := c.Param("path")
	pathString = strings.TrimPrefix(pathString, "/")
	segments := strings.Split(pathString, "/")
	log.Printf("host: %v", host)
	log.Printf("path: %v", pathString)
	log.Printf("segments: %#v", segments)

	hash := ""

	if host == "localhost:8080" {
		hash = segments[0]
		log.Printf("hash: %v", hash)
		c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", hash, webSuffix))
		return
	}

	if strings.HasSuffix(host, webSuffix) {
		hash = strings.TrimSuffix(host, webSuffix)
		log.Printf("hash: %v", hash)
	}

	if hash == "empty" {
		hash = add(c, []byte("{}"))
		c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", hash, webSuffix))
		return
	}

	target, err := traverse(c, hash, segments)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	log.Printf("target: %s", target)
	blob, err := get(c, target)
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	manifest := Manifest{}
	err = json.Unmarshal(blob, &manifest)
	if err != nil {
		log.Printf("could not parse manifest: %v", err)
		c.Data(http.StatusOK, "", blob)
		return
	}
	base := ""
	if segments[0] != "" {
		base = fmt.Sprintf("/%s/", path.Join(segments...))
	}
	c.HTML(http.StatusOK, "render.tmpl", gin.H{
		"hash":     target,
		"path":     segments,
		"blob":     template.HTML(blob),
		"manifest": manifest,
		"base":     base,
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
