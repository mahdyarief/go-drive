package handler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/crypto"
	"go-drive/server/internal/model"
	"go-drive/server/internal/s3"
	"go-drive/server/internal/storage"
	"go-drive/server/internal/store"
	"go-drive/server/internal/tenant"
)

// ---------------------------------------------------------------- S3 gateway

// S3Gateway is the top-level handler for the S3-compatible gateway at
// /api/s3/*path. Path layout: /api/s3/{workspaceSlug}/{key...}. The first path
// segment is the workspace slug; the remainder is the object key (display
// path). Requests are authenticated with AWS SigV4 against a tenant-scoped
// s3_api_keys row.
func S3Gateway(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		p := strings.Trim(c.Param("path"), "/")
		parts := strings.SplitN(p, "/", 2)
		if parts[0] == "" {
			s3Err(c, http.StatusBadRequest, "InvalidRequest", "workspace slug is required")
			return
		}
		workspaceSlug := parts[0]
		key := ""
		if len(parts) > 1 {
			key = parts[1]
		}

		// Look up the API key by access key id from the Authorization header.
		accessKeyID, err := s3.AccessKeyID(c.Request)
		if err != nil {
			s3Err(c, http.StatusForbidden, "AccessDenied", err.Error())
			return
		}

		tx, err := tenant.OpenTx(ctx, db, workspaceSlug)
		if err != nil {
			s3Err(c, http.StatusNotFound, "NoSuchBucket", "workspace not found")
			return
		}
		ok := false
		defer func() {
			if !ok {
				_ = tx.Rollback()
			}
		}()

		var apiKey model.S3APIKey
		if err := tx.NewSelect().Model(&apiKey).Where("access_key_id = ?", accessKeyID).Scan(ctx); err != nil {
			s3Err(c, http.StatusForbidden, "AccessDenied", "invalid access key")
			return
		}
		if !apiKey.IsActive {
			s3Err(c, http.StatusForbidden, "AccessDenied", "access key is inactive")
			return
		}
		if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
			s3Err(c, http.StatusForbidden, "AccessDenied", "access key has expired")
			return
		}

		secret, err := crypto.Decrypt(apiKey.EncryptedSecret)
		if err != nil {
			s3Err(c, http.StatusForbidden, "AccessDenied", "invalid access key")
			return
		}
		if err := s3.VerifySigV4(c.Request, secret); err != nil {
			s3Err(c, http.StatusForbidden, "SignatureDoesNotMatch", err.Error())
			return
		}

		// readonly keys cannot write.
		if apiKey.Permissions == "readonly" &&
			c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			s3Err(c, http.StatusForbidden, "AccessDenied", "readonly key")
			return
		}

		method := c.Request.Method
		q := c.Request.URL.Query()

		switch {
		case method == http.MethodGet && q.Get("uploads") != "":
			s3ListMultipartUploads(c, tx, key, apiKey.UserID)
		case method == http.MethodGet && q.Get("uploadId") != "" && q.Get("partNumber") == "":
			s3ListParts(c, tx, key, q.Get("uploadId"))
		case method == http.MethodGet && (q.Get("list-type") == "2" || key == ""):
			s3ListObjectsV2(c, tx, key)
		case method == http.MethodGet:
			s3GetObject(c, tx, key)
		case method == http.MethodHead:
			s3HeadObject(c, tx, key)
		case method == http.MethodPut && q.Get("uploadId") != "" && q.Get("partNumber") != "":
			s3UploadPart(c, tx, key, q.Get("uploadId"), q.Get("partNumber"))
		case method == http.MethodPut:
			s3PutObject(c, tx, key, apiKey.UserID)
		case method == http.MethodPost && q.Get("uploads") != "":
			s3CreateMultipartUpload(c, tx, key, apiKey.UserID)
		case method == http.MethodPost && q.Get("uploadId") != "":
			s3CompleteMultipartUpload(c, tx, key, q.Get("uploadId"))
		case method == http.MethodDelete && q.Get("uploadId") != "":
			s3AbortMultipartUpload(c, tx, key, q.Get("uploadId"))
		case method == http.MethodDelete:
			s3DeleteObject(c, tx, key)
		default:
			s3Err(c, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported S3 operation")
		}
		if !c.GetBool("s3_errored") {
			ok = true
			_ = tx.Commit()
		}
	}
}

// buildGatewayStorage resolves the primary store and returns a storage client.
func buildGatewayStorage(ctx context.Context, tx bun.IDB) (storage.Storage, *model.Store, error) {
	s, err := store.ResolvePrimaryStore(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	st, err := store.BuildStorage(ctx, tx, s)
	if err != nil {
		return nil, nil, err
	}
	return st, s, nil
}

// resolveOrCreateFolderChain ensures the folder chain implied by key exists and
// returns the deepest folder id (nil for a root-level file). The final segment
// of key is the file name and is NOT created as a folder.
func resolveOrCreateFolderChain(ctx context.Context, tx bun.IDB, userID, key string) (*uuid.UUID, error) {
	segs := strings.Split(key, "/")
	if len(segs) <= 1 {
		return nil, nil
	}
	var parentID *uuid.UUID
	for _, seg := range segs[:len(segs)-1] {
		if seg == "" {
			continue
		}
		var f model.Folder
		var err error
		if parentID == nil {
			err = tx.NewSelect().Model(&f).Where("parent_id IS NULL AND name = ?", seg).Scan(ctx)
		} else {
			err = tx.NewSelect().Model(&f).Where("parent_id = ? AND name = ?", *parentID, seg).Scan(ctx)
		}
		if err != nil {
			f = model.Folder{ID: uuid.New(), UserID: userID, ParentID: parentID, Name: seg}
			if _, err := tx.NewInsert().Model(&f).Exec(ctx); err != nil {
				return nil, err
			}
		}
		parentID = &f.ID
	}
	return parentID, nil
}

// findFileByKey locates a ready file whose storage path equals the object key.
func findFileByKey(ctx context.Context, tx bun.IDB, key string) (*model.File, error) {
	var f model.File
	if err := tx.NewSelect().Model(&f).Where("storage_path = ? AND status = 'ready'", key).Scan(ctx); err != nil {
		return nil, err
	}
	return &f, nil
}

// etagFor produces a quoted ETag for a file.
func etagFor(f *model.File) string {
	if f.Checksum != "" {
		return `"` + f.Checksum + `"`
	}
	return fmt.Sprintf(`"%x"`, f.ID[:8])
}

// ------------------------------------------------------------------ get/head

func s3GetObject(c *gin.Context, tx bun.IDB, key string) {
	ctx := c.Request.Context()
	f, err := findFileByKey(ctx, tx, key)
	if err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchKey", "object not found")
		return
	}
	s, path, err := store.ResolveReadStore(ctx, tx, f.BlobID, f.StoragePath)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	st, err := store.BuildStorage(ctx, tx, s)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	body, size, err := st.Download(ctx, path)
	if err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchKey", "object not found")
		return
	}
	defer body.Close()

	c.Header("Content-Type", f.MimeType)
	c.Header("ETag", etagFor(f))
	c.Header("Last-Modified", f.UpdatedAt.UTC().Format(http.TimeFormat))
	if size >= 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	c.DataFromReader(http.StatusOK, size, f.MimeType, body, nil)
}

func s3HeadObject(c *gin.Context, tx bun.IDB, key string) {
	ctx := c.Request.Context()
	f, err := findFileByKey(ctx, tx, key)
	if err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchKey", "object not found")
		return
	}
	c.Header("Content-Type", f.MimeType)
	c.Header("ETag", etagFor(f))
	c.Header("Last-Modified", f.UpdatedAt.UTC().Format(http.TimeFormat))
	c.Header("Content-Length", strconv.FormatInt(f.Size, 10))
	c.Status(http.StatusOK)
}

// ------------------------------------------------------------------ put/delete

func s3PutObject(c *gin.Context, tx bun.IDB, key, userID string) {
	ctx := c.Request.Context()
	if key == "" {
		s3Err(c, http.StatusBadRequest, "InvalidRequest", "object key is required")
		return
	}
	name := filepath.Base(strings.TrimSuffix(key, "/"))
	if name == "" {
		s3Err(c, http.StatusBadRequest, "InvalidRequest", "invalid object key")
		return
	}
	folderID, err := resolveOrCreateFolderChain(ctx, tx, userID, key)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(name))
	}
	size := c.Request.ContentLength
	if size <= 0 {
		size = 0
	}
	blob, f, err := store.CreatePendingFileUpload(ctx, tx, userID, folderID, name, key, contentType, size)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	st, s, err := buildGatewayStorage(ctx, tx)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	// Stream the body to the primary store while computing the MD5 ETag.
	h := md5.New()
	body := io.TeeReader(c.Request.Body, h)
	if err := st.Upload(ctx, key, body, contentType); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	etag := `"` + hex.EncodeToString(h.Sum(nil)) + `"`

	if err := store.MarkFileUploadReady(ctx, tx, f.ID, blob.ID, s.ID, key); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if mode, err := store.GetStorageMode(ctx, tx); err == nil && mode == "replicate" {
		_ = store.SyncFileToStores(ctx, tx, f.ID, nil, nil, userID)
	}

	c.Header("ETag", etag)
	c.Status(http.StatusOK)
}

func s3DeleteObject(c *gin.Context, tx bun.IDB, key string) {
	ctx := c.Request.Context()
	f, err := findFileByKey(ctx, tx, key)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	if err := store.DeleteFileEverywhere(ctx, tx, f.ID); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ------------------------------------------------------------------ list V2

type listBucketResult struct {
	XMLName         xml.Name     `xml:"ListBucketResult"`
	Xmlns           string       `xml:"xmlns,attr"`
	Name            string       `xml:"Name"`
	Prefix          string       `xml:"Prefix"`
	KeyCount        int          `xml:"KeyCount"`
	MaxKeys         int          `xml:"MaxKeys"`
	Delimiter       string       `xml:"Delimiter,omitempty"`
	IsTruncated     bool         `xml:"IsTruncated"`
	ContinuationTok string       `xml:"ContinuationToken,omitempty"`
	NextToken       string       `xml:"NextContinuationToken,omitempty"`
	Contents        []objContent `xml:"Contents"`
	CommonPrefixes  []commonPref `xml:"CommonPrefixes"`
}

type objContent struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type commonPref struct {
	Prefix string `xml:"Prefix"`
}

func s3ListObjectsV2(c *gin.Context, tx bun.IDB, key string) {
	ctx := c.Request.Context()
	prefix := strings.Trim(c.Query("prefix"), "/")
	delimiter := c.Query("delimiter")
	maxKeys := 1000
	if v := c.Query("max-keys"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxKeys = n
		}
	}
	startAfter := ""
	if v := c.Query("start-after"); v != "" {
		startAfter = v
	}
	if v := c.Query("continuation-token"); v != "" {
		if raw, err := hex.DecodeString(v); err == nil {
			startAfter = string(raw)
		}
	}

	var files []model.File
	q := tx.NewSelect().Model(&files).
		Where("status = 'ready' AND storage_path LIKE ?", prefix+"%").
		Order("storage_path ASC")
	if startAfter != "" {
		q.Where("storage_path > ?", startAfter)
	}
	if err := q.Scan(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	result := listBucketResult{
		Xmlns:     "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:      key,
		Prefix:    prefix,
		MaxKeys:   maxKeys,
		Delimiter: delimiter,
	}
	seenPrefix := map[string]bool{}
	truncated := false
	lastProcessed := ""
	for _, f := range files {
		if result.KeyCount >= maxKeys {
			truncated = true
			break
		}
		objKey := f.StoragePath
		if delimiter != "" {
			rel := strings.TrimPrefix(objKey, prefix)
			if idx := strings.Index(rel, delimiter); idx >= 0 {
				cp := prefix + rel[:idx+1]
				if !seenPrefix[cp] {
					seenPrefix[cp] = true
					result.CommonPrefixes = append(result.CommonPrefixes, commonPref{Prefix: cp})
					result.KeyCount++
				}
				lastProcessed = objKey
				continue
			}
		}
		result.Contents = append(result.Contents, objContent{
			Key:          objKey,
			LastModified: f.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			ETag:         etagFor(&f),
			Size:         f.Size,
			StorageClass: "STANDARD",
		})
		result.KeyCount++
		lastProcessed = objKey
	}
	result.IsTruncated = truncated
	if truncated && lastProcessed != "" {
		result.NextToken = hex.EncodeToString([]byte(lastProcessed))
	}
	c.XML(http.StatusOK, result)
}

// s3Err writes an S3-compatible XML error response and records on the context
// that the request failed, so S3Gateway rolls back the transaction instead of
// committing partially-written records (e.g. orphaned pending file rows).
func s3Err(c *gin.Context, status int, code, message string) {
	c.Set("s3_errored", true)
	c.Header("Content-Type", "application/xml")
	c.Status(status)
	_ = xml.NewEncoder(c.Writer).Encode(struct {
		XMLName xml.Name `xml:"Error"`
		Code    string   `xml:"Code"`
		Message string   `xml:"Message"`
	}{Code: code, Message: message})
}
