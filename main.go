package main

import (
	"context"
	"fmt"
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
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/tiziano88/multiverse/utils"
	"google.golang.org/appengine"
)

var dataStore DataStore

const bucketName = "multiverse-312721.appspot.com"

var webSuffix = ".www.localhost:8080"

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
	router.POST("/*path", uploadHandler)
	router.GET("/*path", renderHandler)

	router.Run()

	appengine.Main()
}

func indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.tmpl", gin.H{})
}

func parsePath(p string) []string {
	if p == "/" || p == "" {
		return []string{}
	} else {
		return strings.Split(strings.TrimPrefix(p, "/"), "/")
	}
}

func uploadHandler(c *gin.Context) {
	host := c.Request.Host
	log.Printf("host: %v", host)
	pathString := c.Param("path")
	log.Printf("path: %v", pathString)
	segments := parsePath(pathString)
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

		var node format.Node
		if codecParsed == cid.Raw {
			node, err = utils.ParseRawNode(bytes)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		} else if codecParsed == cid.DagProtobuf {
			node, err = utils.ParseProtoNode(bytes)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}
		if node == nil {
			log.Print("invalid cid")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		hash := add(c, node)
		// c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", hash, webSuffix))
		log.Printf("uploaded: %s", hash)
		c.String(http.StatusOK, hash.String())
		return
	}

	if dirName, _ := c.GetPostForm("dir"); dirName != "" {
		segmentsLocal := parsePath(pathString)
		segmentsLocal = append(segmentsLocal, dirName)
		log.Printf("creating empty dir: %s -> %v", dirName, segmentsLocal)

		// Empty node.
		newHash := add(c, utils.NewProtoNode())
		hash, err = traverseAdd(c, hash, segmentsLocal, newHash)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s%s", hash, webSuffix, c.Param("path")))
		return
	}

	if linkName, _ := c.GetPostForm("link_name"); linkName != "" {
		linkHashString, _ := c.GetPostForm("link_hash")
		linkHash, err := cid.Decode(linkHashString)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		segmentsLocal := parsePath(pathString)
		segmentsLocal = append(segmentsLocal, linkName)
		log.Printf("creating link: %s/%v -> %s", linkName, segmentsLocal, linkHash)

		// Target node.
		hash, err = traverseAdd(c, hash, segmentsLocal, linkHash)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s%s", hash, webSuffix, c.Param("path")))
		return
	}

	// See https://github.com/gin-gonic/gin#upload-files .
	form, err := c.MultipartForm()
	if err != nil {
		log.Print(err)
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

		segmentsLocal := parsePath(pathString)
		segmentsLocal = append(segmentsLocal, file.Filename)

		node, err := utils.ParseRawNode(b)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		newHash := add(c, node)
		hash, err = traverseAdd(c, hash, segmentsLocal, newHash)
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

func add(c context.Context, node format.Node) cid.Cid {
	h := node.Cid()
	err := dataStore.Set(c, h.String(), node.RawData())
	if err != nil {
		log.Fatal(err)
	}
	return h
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
		node, err := utils.ParseProtoNode(bytes)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not parse blob %s as node", base)
		}
		head := segments[0]
		next, err := utils.GetLink(node, head)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not traverse %s/%s: %v", base, head, err)
		}
		return traverse(c, next, segments[1:])
	}
}

func traverseAdd(c context.Context, base cid.Cid, segments []string, nodeToAdd cid.Cid) (cid.Cid, error) {
	log.Printf("traverseAdd %v/%v", base, segments)

	bytes, err := get(c, base.String())
	if err != nil {
		return cid.Undef, fmt.Errorf("could not get blob %s", base)
	}
	node, err := utils.ParseProtoNode(bytes)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not parse blob %s as manifest: %v", base, err)
	}

	head := segments[0]

	if len(segments) == 1 {
		log.Printf("adding raw link %s", head)
		log.Printf("pre: %v", node.Cid())
		err = utils.SetLink(node, head, nodeToAdd)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not add link: %v", err)
		}
		log.Printf("links: %#v", node.Tree("", 1))
		log.Printf("post: %v", node.Cid())
	} else {
		next, err := utils.GetLink(node, head)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not get link: %v", err)
		}
		log.Printf("next: %v", next)

		newHash, err := traverseAdd(c, next, segments[1:], nodeToAdd)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not call recursively: %v", err)
		}

		err = utils.SetLink(node, head, newHash)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not add link: %v", err)
		}
	}

	return add(c, node), nil
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
	log.Printf("host: %v", host)
	pathString := c.Param("path")
	log.Printf("path: %v", pathString)
	segments := parsePath(pathString)
	log.Printf("segments: %#v", segments)

	base := cid.Undef
	var err error

	/*
		if host == "localhost:8080" {
			base, err = cid.Decode(segments[0])
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			log.Printf("base: %v", base)
			c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", base, webSuffix))
			return
		}
	*/

	if strings.HasSuffix(host, webSuffix) {
		baseDomain := strings.TrimSuffix(host, webSuffix)
		log.Printf("base domain: %s", baseDomain)
		if baseDomain == "empty" {
			target := add(c, utils.NewProtoNode())
			log.Printf("target: %s", target.String())
			c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", target.String(), webSuffix))
			return
		}

		base, err = cid.Decode(baseDomain)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		log.Printf("base: %v", base)
	}

	if len(segments) >= 2 && segments[0] == "blobs" {
		log.Printf("API get blob")
		base, err = cid.Decode(segments[1])
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		segments = segments[2:]
	}

	target, err := traverse(c, base, segments)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	log.Printf("target: %s", target)
	log.Printf("target CID: %#v", target.Prefix())

	if c.Query("stat") != "" {
		ok, err := dataStore.Has(c, target.String())
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if ok {
			c.AbortWithStatus(http.StatusOK)
			return
		} else {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

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
		node, err := utils.ParseProtoNode(blob)
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
		current := c.Param("path")
		if !strings.HasSuffix(current, "/") {
			current += "/"
		}
		c.HTML(http.StatusOK, "render.tmpl", gin.H{
			"hash":    target,
			"path":    segments,
			"node":    node,
			"parent":  path.Dir(path.Dir(current)),
			"current": current,
		})
	} else {
		log.Printf("unknown codec: %v", target.Prefix().Codec)
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
	node, err := utils.ParseRawNode(blob)
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	h := add(c, node)
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
	Has(ctx context.Context, name string) (bool, error)
}

type FileDataStore struct{}

func (s FileDataStore) Set(ctx context.Context, name string, value []byte) error {
	return ioutil.WriteFile("./data/"+name, value, 0644)
}

func (s FileDataStore) Get(ctx context.Context, name string) ([]byte, error) {
	return ioutil.ReadFile("./data/" + name)
}

func (s FileDataStore) Has(ctx context.Context, name string) (bool, error) {
	_, err := os.Stat("./data/" + name)
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

func (s CloudDataStore) Has(ctx context.Context, name string) (bool, error) {
	// TODO: return size
	_, err := s.client.Bucket(bucketName).Object(name).Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}
