package main

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	proto "github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	merkledag_pb "github.com/ipfs/go-merkledag/pb"
	"github.com/tiziano88/multiverse/utils"
	"google.golang.org/appengine"
)

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
	/*
		log.Printf("form: %v", form)
		dir := form.File["directory"]
		if dir != nil {
			for f := range dir {
				log.Printf("dir: %d %v", f, dir[f].Filename)
			}
		}
	*/

	host := c.Request.Host
	pathString := c.Param("path")
	log.Printf("path: %v", pathString)
	pathString = strings.TrimPrefix(pathString, "/")
	segments := strings.Split(pathString, "/")
	log.Printf("host: %v", host)
	log.Printf("segments: %#v", segments)

	hash := cid.Undef
	var err error
	if strings.HasSuffix(host, webSuffix) {
		hash, err = cid.Decode(strings.TrimSuffix(host, webSuffix))
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		log.Printf("hash: %v", hash)
	} else {
		log.Print("straight upload")
		codec := c.Query("codec")
		log.Printf("codec: %v", codec)

		codecParsed := cid.Raw

		if codec != "" {
			codecParsed, err = strconv.Atoi(codec)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}
		// Straight upload.
		bytes, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		hash := addCodec(c, uint64(codecParsed), bytes)
		// c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", hash, webSuffix))
		log.Printf("uploaded: %s", hash)
		c.String(http.StatusOK, hash.String())
		return
	}

	// See https://github.com/gin-gonic/gin#upload-files .
	form, err := c.MultipartForm()
	if err != nil {
		log.Print(err)
	}

	if dirName, _ := c.GetPostForm("dir"); dirName != "" {
		log.Printf("creating empty dir: %s", dirName)
		segments := strings.Split(pathString, "/")
		segments = append(segments, dirName)

		// Empty node.
		hash, err = traverseAdd(c, hash, segments, cid.DagProtobuf, []byte{})
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s%s", hash, webSuffix, c.Param("path")))
		return
	}

	for _, file := range form.File["file"] {
		log.Printf("processing file %s (%dB)", file.Filename, file.Size)
		mpFile, err := file.Open()
		if err != nil {
			log.Fatal(err)
		}
		b, err := ioutil.ReadAll(mpFile)
		if err != nil {
			log.Fatal(err)
		}

		segments := strings.Split(pathString, "/")
		segments = append(segments, file.Filename)

		hash, err = traverseAdd(c, hash, segments, cid.Raw, b)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	targetURL := fmt.Sprintf("http://%s%s%s", hash, webSuffix, c.Param("path"))
	// c.String(http.StatusOK, "%s", targetURL)
	c.Redirect(http.StatusFound, targetURL)
}

func addCodec(c context.Context, codec uint64, blob []byte) cid.Cid {
	h := utils.HashCodec(codec, blob)
	err := dataStore.Set(c, h.String(), blob)
	if err != nil {
		log.Fatal(err)
	}
	return h
}

func addRaw(c context.Context, blob []byte) cid.Cid {
	return addCodec(c, cid.Raw, blob)
}

func addNode(c context.Context, node *merkledag_pb.PBNode) cid.Cid {
	b, err := node.Marshal()
	if err != nil {
		log.Fatal(err)
	}
	return addCodec(c, cid.DagProtobuf, b)
}

func get(c context.Context, hash string) ([]byte, error) {
	return dataStore.Get(c, hash)
}

func traverse(c context.Context, base cid.Cid, segments []string) (cid.Cid, error) {
	if len(segments) == 0 || segments[0] == "" {
		return base, nil
	} else {
		bytes, err := get(c, base.String())
		if err != nil {
			return cid.Undef, fmt.Errorf("could not get blob %s", base)
		}
		node := merkledag_pb.PBNode{}
		err = node.Unmarshal(bytes)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not parse blob %s as node", base)
		}
		head := segments[0]
		next, err := utils.GetLink(&node, head)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not traverse %s/%s: %v", base, head, err)
		}
		return traverse(c, next, segments[1:])
	}
}

func traverseAdd(c context.Context, base cid.Cid, segments []string, codec uint64, blob []byte) (cid.Cid, error) {
	log.Printf("base: %v", base)
	log.Printf("segments: %#v", segments)

	node := merkledag_pb.PBNode{}
	bytes, err := get(c, base.String())
	if err != nil {
		return cid.Undef, fmt.Errorf("could not get blob %s", base)
	}
	err = node.Unmarshal(bytes)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not parse blob %s as manifest: %v", base, err)
	}

	head := segments[0]

	if head == "" {
		return traverseAdd(c, base, segments[1:], codec, blob)
	}

	if len(segments) == 1 {
		newHash := addCodec(c, codec, blob)
		node.Links = append(node.Links, &merkledag_pb.PBLink{
			Name: proto.String(head),
			Hash: newHash.Bytes(),
		})
	} else {
		next, err := utils.GetLink(&node, head)
		log.Printf("next: %v", next)

		newHash, err := traverseAdd(c, next, segments[1:], codec, blob)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not call recursively: %v", err)
		}

		utils.SetLink(&node, head, newHash)
	}

	return addNode(c, &node), nil
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
	log.Printf("path: %v", pathString)
	pathString = strings.TrimPrefix(pathString, "/")
	segments := strings.Split(pathString, "/")
	log.Printf("host: %v", host)
	log.Printf("segments: %#v", segments)

	hash := cid.Undef
	var err error

	if host == "localhost:8080" {
		hash, err = cid.Decode(segments[0])
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		log.Printf("hash: %v", hash)
		c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", hash, webSuffix))
		return
	}

	if strings.HasSuffix(host, webSuffix) {
		subdomain := strings.TrimSuffix(host, webSuffix)
		if subdomain == "empty" {
			hash = addNode(c, &merkledag_pb.PBNode{})
			c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", hash, webSuffix))
			return
		}

		hash, err = cid.Decode(subdomain)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		log.Printf("hash: %v", hash)
	}

	target, err := traverse(c, hash, segments)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	log.Printf("target: %s", target)
	log.Printf("target CID: %#v", target.Prefix())
	blob, err := get(c, target.String())
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	if target.Prefix().Codec == cid.Raw {
		c.Header("multiverse-hash", target.String())
		ext := filepath.Ext(pathString)
		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = http.DetectContentType(blob)
		}
		c.Header("Content-Type", contentType)
		c.Data(http.StatusOK, "", blob)
		return
	} else if target.Prefix().Codec == cid.DagProtobuf {
		node := merkledag_pb.PBNode{}
		err = node.Unmarshal(blob)
		if err != nil {
			log.Printf("could not parse manifest: %v", err)
			c.Header("multiverse-hash", target.String())
			ext := filepath.Ext(pathString)
			contentType := mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = http.DetectContentType(blob)
			}
			c.Header("Content-Type", contentType)
			c.Data(http.StatusOK, "", blob)
			return
		}
		base := c.Param("path")
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		c.HTML(http.StatusOK, "render.tmpl", gin.H{
			"hash":   target,
			"path":   segments,
			"blob":   template.HTML(blob),
			"node":   node,
			"base":   base,
			"parent": path.Dir(path.Dir(base)),
		})
	} else {
		log.Print("unknown codec: %v", hash.Prefix().Codec)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
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
	h := addRaw(c, blob)
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
