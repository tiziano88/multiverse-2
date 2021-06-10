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
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/tiziano88/multiverse/utils"
	"google.golang.org/appengine"
)

var (
	blobStore DataStore
	tagStore  DataStore

	handlerBrowse http.Handler
	handlerWWW    http.Handler
)

const blobBucketName = "multiverse-312721.appspot.com"
const tagBucketName = "multiverse-312721-key"

const wwwSegment = "www"
const tagSegment = "tag"

var domainName = "localhost:8080"

func hostSegments(host string) []string {
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

	{
		router := gin.Default()
		router.RedirectTrailingSlash = false
		router.RedirectFixedPath = false
		router.LoadHTMLGlob("templates/*")

		router.POST("/api/get", apiGetHandler)
		router.POST("/api/update", apiUpdateHandler)
		router.POST("/api/rename", apiRenameHandler)
		router.POST("/api/remove", apiRemoveHandler)

		router.GET("/blobs/:root", browseBlobHandler)
		router.GET("/blobs/:root/*path", browseBlobHandler)

		router.StaticFile("/static/tailwind.min.css", "./templates/tailwind.min.css")

		handlerBrowse = router
	}
	{
		router := gin.Default()
		router.GET("/*path", renderHandler)
		handlerWWW = router
	}

	s := &http.Server{
		Addr:           ":8080",
		Handler:        http.HandlerFunc(handlerRoot),
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())

	appengine.Main()
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	hostSegments := hostSegments(r.Host)
	log.Printf("host segments: %#v", hostSegments)
	if len(hostSegments) == 0 {
		handlerBrowse.ServeHTTP(w, r)
	} else {
		handlerWWW.ServeHTTP(w, r)
	}
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

func serveUI(c *gin.Context, root cid.Cid, segments []string, target cid.Cid, blob []byte) {
	templateSegments := []TemplateSegment{}
	for i, s := range segments {
		templateSegments = append(templateSegments, TemplateSegment{
			Name: s,
			Path: path.Join(segments[0 : i+1]...),
		})
	}
	pathStr := c.Param("path")
	// if !strings.HasSuffix(current, "/") {
	// 	current += "/"
	// }
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
		c.HTML(http.StatusOK, "browse.tmpl", gin.H{
			"type":         "directory",
			"wwwHost":      wwwSegment + "." + domainName,
			"root":         root,
			"path":         pathStr,
			"parentPath":   path.Dir(path.Dir(pathStr)),
			"pathSegments": templateSegments,
			"node":         node,
		})
	case cid.Raw:
		c.HTML(http.StatusOK, "browse.tmpl", gin.H{
			"type":         "file",
			"wwwHost":      wwwSegment + "." + domainName,
			"root":         root,
			"path":         pathStr,
			"parentPath":   path.Dir(path.Dir(pathStr)),
			"pathSegments": templateSegments,
			"blob":         blob,
			"blob_str":     string(blob),
		})
	}
}

type TemplateSegment struct {
	Name string
	Path string
}

func serveWWW(c *gin.Context, root cid.Cid, segments []string) {
	target, err := traverse(c, root, segments)
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
		serveUI(c, root, segments, target, blob)
	} else {
		log.Printf("unknown codec: %v", target.Prefix().Codec)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
}

type RenameRequest struct {
	Root     string
	FromPath string
	ToPath   string
}

type RemoveRequest struct {
	Root string
	Path string
}

type UploadRequest struct {
	Root  string
	Blobs []UploadBlob
}

type UploadBlob struct {
	Type    string // file | directory
	Path    string
	Content []byte
}

type UploadResponse struct {
	Root string
}

type GetRequest struct {
	Root string
	Path string
}

type GetResponse struct {
	Content []byte
}

func apiUpdateHandler(c *gin.Context) {
	var req UploadRequest
	json.NewDecoder(c.Request.Body).Decode(&req)
	// log.Printf("req: %#v", req)

	if req.Root == "" && len(req.Blobs) == 1 {
		log.Printf("individual blob")
		// Individual blob upload.
		b := req.Blobs[0]
		var node format.Node
		var err error
		switch b.Type {
		case "file":
			node, err = utils.ParseRawNode(b.Content)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		case "directory":
			node, err = utils.ParseProtoNode(b.Content)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		default:
			log.Printf("invalid type: %s", b.Type)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if node == nil {
			log.Print("invalid cid")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		hash := add(c, node)
		// c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", hash, webSuffix))
		log.Printf("uploaded: %s", hash)
		c.JSON(http.StatusOK, UploadResponse{
			Root: hash.String(),
		})
		return
	}

	root, err := cid.Decode(req.Root)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	for _, b := range req.Blobs {
		log.Printf("type: %s", b.Type)
		log.Printf("path: %s", b.Path)
		pathSegments := parsePath(b.Path)
		log.Printf("path segments: %#v", pathSegments)
		var newNode format.Node
		switch b.Type {
		case "file":
			newNode, err = utils.ParseRawNode(b.Content)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		case "directory":
			newNode = utils.NewProtoNode()
		default:
			log.Printf("invalid type: %s", b.Type)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		newHash := add(c, newNode)
		log.Printf("new hash: %s", newHash.String())
		root, err = traverseAdd(c, root, pathSegments, newHash)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}
	res := UploadResponse{
		Root: root.String(),
	}
	log.Printf("res: %#v", res)
	c.JSON(http.StatusOK, res)
}

func apiRenameHandler(c *gin.Context) {
	var r RenameRequest
	json.NewDecoder(c.Request.Body).Decode(&r)
	log.Printf("rename: %#v", r)
	// TODO
}

func apiRemoveHandler(c *gin.Context) {
	var req RemoveRequest
	json.NewDecoder(c.Request.Body).Decode(&req)
	log.Printf("req: %#v", req)
	root, err := cid.Decode(req.Root)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	pathSegments := parsePath(req.Path)
	hash, err := traverseRemove(c, root, pathSegments)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	res := UploadResponse{
		Root: hash.String(),
	}
	log.Printf("res: %#v", res)
	c.JSON(http.StatusOK, res)
}

func apiGetHandler(c *gin.Context) {
	var req GetRequest
	json.NewDecoder(c.Request.Body).Decode(&req)
	log.Printf("req: %#v", req)

	root, err := cid.Decode(req.Root)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	segments := parsePath(req.Path)
	target, err := traverse(c, root, segments)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	blob, err := get(c, target.String())
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	res := GetResponse{
		Content: blob,
	}
	log.Printf("res: %#v", res)
	c.JSON(http.StatusOK, res)
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

func traverse(c context.Context, root cid.Cid, segments []string) (cid.Cid, error) {
	if len(segments) == 0 {
		return root, nil
	} else {
		bytes, err := get(c, root.String())
		if err != nil {
			return cid.Undef, fmt.Errorf("could not get blob %s", root)
		}
		node, err := utils.ParseProtoNode(bytes)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not parse blob %s as node", root)
		}
		head := segments[0]
		next, err := utils.GetLink(node, head)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not traverse %s/%s: %v", root, head, err)
		}
		log.Printf("next: %v", next)
		return traverse(c, next, segments[1:])
	}
}

func traverseAdd(c context.Context, root cid.Cid, segments []string, nodeToAdd cid.Cid) (cid.Cid, error) {
	log.Printf("traverseAdd %v/%#v", root, segments)
	if len(segments) == 0 {
		return nodeToAdd, nil
	} else {
		bytes, err := get(c, root.String())
		if err != nil {
			return cid.Undef, fmt.Errorf("could not get blob %s", root)
		}
		node, err := utils.ParseProtoNode(bytes)
		if err != nil {
			return cid.Undef, fmt.Errorf("could not parse blob %s as manifest: %v", root, err)
		}
		head := segments[0]
		var next cid.Cid
		next, err = utils.GetLink(node, head)
		if err == merkledag.ErrLinkNotFound {
			// Ok
			next = add(c, utils.NewProtoNode())
		} else if err != nil {
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
		return add(c, node), nil
	}
}

func traverseRemove(c context.Context, root cid.Cid, segments []string) (cid.Cid, error) {
	log.Printf("traverseRemove %v/%#v", root, segments)
	bytes, err := get(c, root.String())
	if err != nil {
		return cid.Undef, fmt.Errorf("could not get blob %s", root)
	}
	node, err := utils.ParseProtoNode(bytes)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not parse blob %s as manifest: %v", root, err)
	}

	if len(segments) == 1 {
		utils.RemoveLink(node, segments[0])
	} else {
		head := segments[0]
		var next cid.Cid
		next, err = utils.GetLink(node, head)
		if err == merkledag.ErrLinkNotFound {
			// Ok
			next = add(c, utils.NewProtoNode())
		} else if err != nil {
			return cid.Undef, fmt.Errorf("could not get link: %v", err)
		}
		log.Printf("next: %v", next)

		newHash, err := traverseRemove(c, next, segments[1:])
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

func browseBlobHandler(c *gin.Context) {
	pathString := c.Param("path")
	log.Printf("path: %q", pathString)
	segments := parsePath(pathString)
	log.Printf("segments: %#v", segments)

	// if pathString != "/" && strings.HasSuffix(pathString, "/") {
	// 	c.Redirect(http.StatusMovedPermanently, strings.TrimSuffix(pathString, "/"))
	// 	return
	// }
	if strings.HasSuffix(c.Request.URL.Path, "/") {
		to := strings.TrimSuffix(c.Request.URL.Path, "/")
		log.Printf("redirecting to: %q", to)
		c.Redirect(http.StatusMovedPermanently, to)
		return
	}

	root, err := cid.Decode(c.Param("root"))
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	target, err := traverse(c, root, segments)
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
	serveUI(c, root, segments, target, blob)
}

func renderHandler(c *gin.Context) {
	hostSegments := hostSegments(c.Request.Host)
	log.Printf("host segments: %v", hostSegments)
	pathString := c.Param("path")
	log.Printf("path: %v", pathString)
	segments := parsePath(pathString)
	log.Printf("segments: %#v", segments)
	if pathString != "/" && strings.HasSuffix(pathString, "/") {
		c.Redirect(http.StatusMovedPermanently, strings.TrimSuffix(pathString, "/"))
		return
	}

	root := cid.Undef
	var err error

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

			root, err = cid.Decode(baseDomain)
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			log.Printf("root: %v", root)
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

	serveWWW(c, root, segments)
}
