package main

import (
	"context"
	"encoding/json"
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

var (
	blobStore DataStore
	tagStore  DataStore
)

const blobBucketName = "multiverse-312721.appspot.com"
const tagBucketName = "multiverse-312721-key"

const wwwSegment = "www"
const tagSegment = "tag"

var domainName = "localhost:8080"

func hostSegments(c *gin.Context) []string {
	host := c.Request.Host
	host = strings.TrimSuffix(host, domainName)
	host = strings.TrimSuffix(host, ".")
	hostSegments := strings.Split(host, ".")
	if len(hostSegments) > 0 && hostSegments[0] == "" {
		return hostSegments[1:]
	} else {
		return hostSegments
	}
}

func redirectToCid(c *gin.Context, target cid.Cid, path string) {
	c.Redirect(http.StatusFound, fmt.Sprintf("//%s.%s.%s%s", target.String(), wwwSegment, domainName, path))
}

func main() {
	domainNameEnv := os.Getenv("DOMAIN_NAME")
	if domainNameEnv != "" {
		domainName = domainNameEnv
	}
	log.Printf("domain name: %s", domainName)

	ctx := context.Background()
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Print(err)
		blobStore = FileDataStore{
			dirName: "data",
		}
		tagStore = FileDataStore{
			dirName: "tags",
		}
	} else {
		blobStore = CloudDataStore{
			client:     storageClient,
			bucketName: blobBucketName,
		}
		tagStore = CloudDataStore{
			client:     storageClient,
			bucketName: tagBucketName,
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

func parseHost(p string) []string {
	if p == "/" || p == "" {
		return []string{}
	} else {
		return strings.Split(strings.TrimPrefix(p, "/"), "/")
	}
}

func postTagHandler(c *gin.Context) {
	segments := parsePath(c.Param("path"))
	tagName := segments[1]
	tagValueString, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tagValue, err := cid.Decode(string(tagValueString))
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	err = tagStore.Set(c, tagName, []byte(tagValue.String()))
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func serveUI(c *gin.Context, base cid.Cid, segments []string, target cid.Cid, blob []byte) {
	templateSegments := []TemplateSegment{}
	for i, s := range segments {
		templateSegments = append(templateSegments, TemplateSegment{
			Name: s,
			Path: path.Join(segments[0 : i+1]...),
		})
	}
	switch target.Prefix().Codec {
	case cid.DagProtobuf:
		node, err := utils.ParseProtoNode(blob)
		if err != nil {
			log.Printf("could not parse manifest: %v", err)
			c.Header("multiverse-hash", target.String())
			ext := filepath.Ext(segments[len(segments)-1])
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
			"type":     "directory",
			"base":     base,
			"path":     path.Join(segments...),
			"segments": templateSegments,
			"hash":     target,
			"node":     node,
			"parent":   path.Dir(path.Dir(current)),
			"current":  current,
		})
	case cid.Raw:
		current := c.Param("path")
		if !strings.HasSuffix(current, "/") {
			current += "/"
		}
		c.HTML(http.StatusOK, "render.tmpl", gin.H{
			"type":     "file",
			"base":     base,
			"path":     path.Join(segments...),
			"segments": templateSegments,
			"hash":     target,
			"parent":   path.Dir(path.Dir(current)),
			"current":  current,
			"blob":     blob,
			"blob_str": string(blob),
		})
	}
}

type TemplateSegment struct {
	Name string
	Path string
}

func serveWWW(c *gin.Context, base cid.Cid, segments []string) {
	target, err := traverse(c, base, segments)
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
		ext := filepath.Ext(segments[len(segments)-1])
		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = http.DetectContentType(blob)
		}
		c.Header("Content-Type", contentType)
		c.Data(http.StatusOK, "", blob)
		return
	} else if target.Prefix().Codec == cid.DagProtobuf {
		serveUI(c, base, segments, target, blob)
	} else {
		log.Printf("unknown codec: %v", target.Prefix().Codec)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
}

type RenameRequest struct {
	Base     string
	FromPath string
	ToPath   string
}

type UploadRequest struct {
	// Base  string
	// Blobs []UploadBlob
	Type    string // file | dir
	Base    string
	Path    string
	Content []byte
}

type UploadBlob struct {
	Type    string // file | dir
	Path    string
	Content []byte
}

type UploadResponse struct {
	RedirectURL string
}

// base, pathSegments
func parseFullPath(p string) (string, []string) {
	segments := strings.Split(p, "/")
	return segments[0], segments[1:]
}

func uploadHandler(c *gin.Context) {
	hostSegments := hostSegments(c)
	log.Printf("host segments: %#v", hostSegments)
	pathString := c.Param("path")
	log.Printf("path: %v", pathString)
	segments := parsePath(pathString)
	log.Printf("segments: %#v", segments)

	hash := cid.Undef
	var err error
	if len(hostSegments) == 0 {
		switch segments[0] {
		case "api":
			switch segments[1] {
			case "update":
				var u UploadRequest
				json.NewDecoder(c.Request.Body).Decode(&u)
				log.Printf("upload: %#v", u)
				pathSegments := strings.Split(u.Path, "/")
				base, err := cid.Decode(u.Base)
				if err != nil {
					log.Print(err)
					c.AbortWithStatus(http.StatusNotFound)
					return
				}
				var newNode format.Node
				switch u.Type {
				case "file":
					newNode, err = utils.ParseRawNode(u.Content)
					if err != nil {
						log.Print(err)
						c.AbortWithStatus(http.StatusNotFound)
						return
					}
				case "directory":
					newNode = utils.NewProtoNode()
				default:
					log.Printf("invalid type: %s", u.Type)
					c.AbortWithStatus(http.StatusNotFound)
					return
				}
				newHash := add(c, newNode)
				hash, err = traverseAdd(c, base, pathSegments, newHash)
				if err != nil {
					log.Print(err)
					c.AbortWithStatus(http.StatusNotFound)
					return
				}
				c.JSON(http.StatusOK, UploadResponse{
					RedirectURL: "/" + hash.String() + "/" + path.Join(pathSegments...),
				})
				return
			case "rename":
				var r RenameRequest
				json.NewDecoder(c.Request.Body).Decode(&r)
				log.Printf("rename: %#v", r)
				// TODO
				return
			}
		}
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

	if len(hostSegments) == 2 && hostSegments[1] == wwwSegment {
		hash, err = cid.Decode(hostSegments[0])
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		log.Printf("hash: %v", hash)
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
		redirectToCid(c, hash, c.Param("path"))
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
		redirectToCid(c, hash, c.Param("path"))
		return
	}

	if tagName, _ := c.GetPostForm("tag_name"); tagName != "" {
		tagValueString, _ := c.GetPostForm("tag_value")
		log.Printf("setting tag %s -> %s", tagName, tagValueString)
		tagValue, err := cid.Decode(tagValueString)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		err = tagStore.Set(c, tagName, []byte(tagValue.String()))
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("//%s.%s.%s", tagName, tagSegment, domainName))
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

	redirectToCid(c, hash, c.Param("path"))
}

func add(c context.Context, node format.Node) cid.Cid {
	h := node.Cid()
	err := blobStore.Set(c, h.String(), node.RawData())
	if err != nil {
		log.Fatal(err)
	}
	return h
}

func get(c context.Context, hash string) ([]byte, error) {
	return blobStore.Get(c, hash)
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
	b, err := blobStore.Get(c, hash)
	if err != nil {
		log.Print(err)
		c.Abort()
		return
	}
	c.Data(http.StatusOK, "", b)
}

func renderHandler(c *gin.Context) {
	hostSegments := hostSegments(c)
	log.Printf("host segments: %v", hostSegments)
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

	if len(hostSegments) == 0 {
		base, err = cid.Decode(segments[0])
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		target, err := traverse(c, base, segments[1:])
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		blob, err := get(c, target.String())
		if err != nil {
			log.Print(err)
			c.Abort()
			return
		}
		serveUI(c, base, segments[1:], target, blob)
	}

	if len(hostSegments) == 2 {
		switch hostSegments[1] {
		case wwwSegment:
			baseDomain := hostSegments[0]
			log.Printf("base domain: %s", baseDomain)
			if baseDomain == "empty" {
				target := add(c, utils.NewProtoNode())
				log.Printf("target: %s", target.String())
				redirectToCid(c, target, "")
				return
			}

			base, err = cid.Decode(baseDomain)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			log.Printf("base: %v", base)
		case tagSegment:
			tagValueBytes, err := tagStore.Get(c, hostSegments[0])
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			tagValue, err := cid.Decode(string(tagValueBytes))
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			serveWWW(c, tagValue, segments)
			return
		default:
			log.Printf("invalid segment")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
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
		ok, err := blobStore.Has(c, target.String())
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

	serveWWW(c, base, segments)
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
