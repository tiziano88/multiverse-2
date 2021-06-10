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
		router.RedirectTrailingSlash = true
		router.RedirectFixedPath = true
		router.LoadHTMLGlob("templates/*")
		router.POST("/api/update", apiUpdateHandler)
		router.POST("/api/rename", apiRenameHandler)
		router.POST("/api/remove", apiRemoveHandler)
		router.GET("/blobs/:root/*path", browseBlobHandler)
		router.StaticFile("/static/tailwind.min.css", "./templates/tailwind.min.css")
		// router.GET("/static/tailwind.min.css", http.Fil)
		// router.POST("/*path", uploadHandler)
		// router.GET("/*path", renderHandler)
		handlerBrowse = router
	}
	{
		router := gin.Default()
		router.GET("/*path", renderHandler)
		handlerWWW = router
	}

	// router := gin.Default()
	// router.RedirectTrailingSlash = true
	// router.RedirectFixedPath = true
	// router.LoadHTMLGlob("templates/*")
	// // router.GET("/tailwind.min.css", gin.Stat)
	// router.POST("/*path", uploadHandler)
	// router.GET("/*path", renderHandler)
	// router.Run()

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
	current := c.Param("path")
	if !strings.HasSuffix(current, "/") {
		current += "/"
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
		c.HTML(http.StatusOK, "render.tmpl", gin.H{
			"type":         "directory",
			"wwwHost":      wwwSegment + "." + domainName,
			"root":         root,
			"path":         current,
			"parentPath":   path.Dir(path.Dir(current)),
			"pathSegments": templateSegments,
			"node":         node,
		})
	case cid.Raw:
		c.HTML(http.StatusOK, "render.tmpl", gin.H{
			"type":         "file",
			"wwwHost":      wwwSegment + "." + domainName,
			"root":         root,
			"path":         current,
			"parentPath":   path.Dir(path.Dir(current)),
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
	Type    string // file | dir
	Path    string
	Content []byte
}

type UploadResponse struct {
	Root string
}

// root, pathSegments
func parseFullPath(p string) (string, []string) {
	segments := strings.Split(p, "/")
	return segments[0], segments[1:]
}

func apiUpdateHandler(c *gin.Context) {
	var u UploadRequest
	json.NewDecoder(c.Request.Body).Decode(&u)
	log.Printf("upload: %#v", u)
	root, err := cid.Decode(u.Root)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	for _, b := range u.Blobs {
		pathSegments := parsePath(b.Path)
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
		root, err = traverseAdd(c, root, pathSegments, newHash)
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}
	c.JSON(http.StatusOK, UploadResponse{
		Root: root.String(),
	})
}

func apiRenameHandler(c *gin.Context) {
	var r RenameRequest
	json.NewDecoder(c.Request.Body).Decode(&r)
	log.Printf("rename: %#v", r)
	// TODO
}

func apiRemoveHandler(c *gin.Context) {
	var r RemoveRequest
	json.NewDecoder(c.Request.Body).Decode(&r)
	log.Printf("remove: %#v", r)
	root, err := cid.Decode(r.Root)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	pathSegments := parsePath(r.Path)
	hash, err := traverseRemove(c, root, pathSegments)
	if err != nil {
		log.Print(err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, UploadResponse{
		Root: hash.String(),
	})
}

func xxxuploadHandler(c *gin.Context) {
	hostSegments := hostSegments(c.Request.Host)
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
				root, err := cid.Decode(u.Root)
				if err != nil {
					log.Print(err)
					c.AbortWithStatus(http.StatusNotFound)
					return
				}
				for _, b := range u.Blobs {
					pathSegments := parsePath(b.Path)
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
					root, err = traverseAdd(c, root, pathSegments, newHash)
					if err != nil {
						log.Print(err)
						c.AbortWithStatus(http.StatusNotFound)
						return
					}
				}
				c.JSON(http.StatusOK, UploadResponse{
					Root: root.String(),
				})
				return
			case "rename":
				var r RenameRequest
				json.NewDecoder(c.Request.Body).Decode(&r)
				log.Printf("rename: %#v", r)
				// TODO
				return
			case "remove":
				var r RemoveRequest
				json.NewDecoder(c.Request.Body).Decode(&r)
				log.Printf("remove: %#v", r)
				root, err := cid.Decode(r.Root)
				if err != nil {
					log.Print(err)
					c.AbortWithStatus(http.StatusNotFound)
					return
				}
				pathSegments := parsePath(r.Path)
				hash, err = traverseRemove(c, root, pathSegments)
				if err != nil {
					log.Print(err)
					c.AbortWithStatus(http.StatusNotFound)
					return
				}
				c.JSON(http.StatusOK, UploadResponse{
					Root: hash.String(),
				})
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

func browseBlobHandler(c *gin.Context) {
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
	root, err = cid.Decode(c.Param("root"))
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

	if pathString == "/tailwind.min.css" {
		c.File("./templates/tailwind.min.css")
		return
	}

	root := cid.Undef
	var err error

	/*
		if host == "localhost:8080" {
			root, err = cid.Decode(segments[0])
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			log.Printf("root: %v", root)
			c.Redirect(http.StatusFound, fmt.Sprintf("//%s%s", root, webSuffix))
			return
		}
	*/

	if len(hostSegments) == 0 {
		root, err = cid.Decode(segments[0])
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		target, err := traverse(c, root, segments[1:])
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
		serveUI(c, root, segments[1:], target, blob)
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

	if len(segments) >= 2 && segments[0] == "blobs" {
		log.Printf("API get blob")
		root, err = cid.Decode(segments[1])
		if err != nil {
			log.Print(err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		segments = segments[2:]
	}

	target, err := traverse(c, root, segments)
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

	serveWWW(c, root, segments)
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
