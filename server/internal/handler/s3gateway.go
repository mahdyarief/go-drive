package handler

import (
	"context"
	"crypto/md5"
	"crypto/rand"
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

// ---------------------------------------------------------------- multipart

// multipartTempPath returns the storage key used to hold one uploaded part.
func multipartTempPath(uploadID, partNumber string) string {
	return ".s3parts/" + uploadID + "/" + partNumber
}

func s3CreateMultipartUpload(c *gin.Context, tx bun.IDB, key, userID string) {
	ctx := c.Request.Context()
	if key == "" {
		s3Err(c, http.StatusBadRequest, "InvalidRequest", "object key is required")
		return
	}
	name := filepath.Base(strings.TrimSuffix(key, "/"))
	folderID, err := resolveOrCreateFolderChain(ctx, tx, userID, key)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(name))
	}
	// Reserve the file record now; content lands on CompleteMultipartUpload.
	name, err = dedupFileName(ctx, tx, name, folderID)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	blob, f, err := store.CreatePendingFileUpload(ctx, tx, userID, folderID, name, key, contentType, 0)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	uploadID := uuid.NewString()
	up := &model.S3MultipartUpload{
		ID:          uuid.New(),
		UploadID:    uploadID,
		S3Key:       key,
		StoragePath: key,
		ContentType: contentType,
		UserID:      userID,
		Status:      "in_progress",
	}
	if _, err := tx.NewInsert().Model(up).Exec(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	_ = blob // reserved; not yet uploaded
	_ = f
	c.XML(http.StatusOK, struct {
		XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
		Xmlns    string   `xml:"xmlns,attr"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		UploadID string   `xml:"UploadId"`
	}{Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/", Bucket: "", Key: key, UploadID: uploadID})
}

func s3UploadPart(c *gin.Context, tx bun.IDB, key, uploadID, partNumber string) {
	ctx := c.Request.Context()
	partNum, err := strconv.Atoi(partNumber)
	if err != nil || partNum < 1 {
		s3Err(c, http.StatusBadRequest, "InvalidArgument", "invalid partNumber")
		return
	}
	var up model.S3MultipartUpload
	if err := tx.NewSelect().Model(&up).Where("upload_id = ?", uploadID).Scan(ctx); err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchUpload", "upload not found")
		return
	}
	st, _, err := buildGatewayStorage(ctx, tx)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	tempPath := multipartTempPath(uploadID, partNumber)
	h := md5.New()
	body := io.TeeReader(c.Request.Body, h)
	if err := st.Upload(ctx, tempPath, body, "application/octet-stream"); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	etag := `"` + hex.EncodeToString(h.Sum(nil)) + `"`
	size := c.Request.ContentLength
	if size <= 0 {
		size = 0
	}
	part := &model.S3MultipartPart{
		ID:          uuid.New(),
		UploadID:    uploadID,
		PartNumber:  partNum,
		StoragePath: tempPath,
		Size:        size,
		ETag:        etag,
	}
	if _, err := tx.NewInsert().Model(part).
		On("CONFLICT (upload_id, part_number) DO UPDATE SET storage_path = EXCLUDED.storage_path, size = EXCLUDED.size, etag = EXCLUDED.etag").
		Exec(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	c.Header("ETag", etag)
	c.Status(http.StatusOK)
}

func s3CompleteMultipartUpload(c *gin.Context, tx bun.IDB, key, uploadID string) {
	ctx := c.Request.Context()
	var up model.S3MultipartUpload
	if err := tx.NewSelect().Model(&up).Where("upload_id = ?", uploadID).Scan(ctx); err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchUpload", "upload not found")
		return
	}
	var parts []model.S3MultipartPart
	if err := tx.NewSelect().Model(&parts).Where("upload_id = ?", uploadID).Order("part_number ASC").Scan(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if len(parts) == 0 {
		s3Err(c, http.StatusBadRequest, "InvalidPart", "no parts uploaded")
		return
	}
	st, s, err := buildGatewayStorage(ctx, tx)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	// Concatenate all parts in order into a single io.Reader and stream to the
	// final key. Each part is opened from its temp path on the primary store.
	var readers []io.ReadCloser
	var streams []io.Reader
	// Register the close before downloading so a mid-loop failure still closes
	// the readers for parts already opened (avoiding an FD leak on error).
	defer func() {
		for _, r := range readers {
			_ = r.Close()
		}
	}()
	for i := range parts {
		pr, _, err := st.Download(ctx, parts[i].StoragePath)
		if err != nil {
			s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		readers = append(readers, pr)
		streams = append(streams, pr)
	}
	multi := io.MultiReader(streams...)
	if err := st.Upload(ctx, key, multi, up.ContentType); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	// Delete temp parts.
	for i := range parts {
		_ = st.Delete(ctx, parts[i].StoragePath)
	}

	// Find the reserved file record and mark it ready.
	var f model.File
	if err := tx.NewSelect().Model(&f).Where("blob_id = (SELECT id FROM file_blobs WHERE object_key = ? AND created_by_id = ?)", key, up.UserID).Order("created_at DESC").Limit(1).Scan(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	var blob model.FileBlob
	if err := tx.NewSelect().Model(&blob).Where("id = ?", f.BlobID).Scan(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if err := store.MarkFileUploadReady(ctx, tx, f.ID, blob.ID, s.ID, key); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if mode, err := store.GetStorageMode(ctx, tx); err == nil && mode == "replicate" {
		_ = store.SyncFileToStores(ctx, tx, f.ID, nil, nil, up.UserID)
	}

	// Clean up the upload + part rows.
	if _, err := tx.NewDelete().Model((*model.S3MultipartPart)(nil)).Where("upload_id = ?", uploadID).Exec(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if _, err := tx.NewDelete().Model((*model.S3MultipartUpload)(nil)).Where("upload_id = ?", uploadID).Exec(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	c.XML(http.StatusOK, struct {
		XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
		Xmlns    string   `xml:"xmlns,attr"`
		Location string   `xml:"Location"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		ETag     string   `xml:"ETag"`
	}{Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/", Location: "", Bucket: "", Key: key, ETag: fmt.Sprintf(`"%x"`, f.ID[:8])})
}

func s3AbortMultipartUpload(c *gin.Context, tx bun.IDB, key, uploadID string) {
	ctx := c.Request.Context()
	var up model.S3MultipartUpload
	if err := tx.NewSelect().Model(&up).Where("upload_id = ?", uploadID).Scan(ctx); err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	st, _, err := buildGatewayStorage(ctx, tx)
	if err == nil {
		var parts []model.S3MultipartPart
		if err := tx.NewSelect().Model(&parts).Where("upload_id = ?", uploadID).Scan(ctx); err == nil {
			for i := range parts {
				_ = st.Delete(ctx, parts[i].StoragePath)
			}
		}
	}
	// Delete the reserved file record + blob.
	if err == nil {
		var f model.File
		if err := tx.NewSelect().Model(&f).Where("blob_id = (SELECT id FROM file_blobs WHERE object_key = ? AND created_by_id = ?)", key, up.UserID).Order("created_at DESC").Limit(1).Scan(ctx); err == nil {
			_ = store.DeleteFileEverywhere(ctx, tx, f.ID)
		}
	}
	_, _ = tx.NewDelete().Model((*model.S3MultipartPart)(nil)).Where("upload_id = ?", uploadID).Exec(ctx)
	_, _ = tx.NewDelete().Model((*model.S3MultipartUpload)(nil)).Where("upload_id = ?", uploadID).Exec(ctx)
	c.Status(http.StatusNoContent)
}

func s3ListParts(c *gin.Context, tx bun.IDB, key, uploadID string) {
	ctx := c.Request.Context()
	var up model.S3MultipartUpload
	if err := tx.NewSelect().Model(&up).Where("upload_id = ?", uploadID).Scan(ctx); err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchUpload", "upload not found")
		return
	}
	var parts []model.S3MultipartPart
	if err := tx.NewSelect().Model(&parts).Where("upload_id = ?", uploadID).Order("part_number ASC").Scan(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	type partEntry struct {
		PartNumber   int    `xml:"PartNumber"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
	}
	entries := make([]partEntry, 0, len(parts))
	for _, p := range parts {
		entries = append(entries, partEntry{
			PartNumber:   p.PartNumber,
			LastModified: p.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			ETag:         p.ETag,
			Size:         p.Size,
		})
	}
	c.XML(http.StatusOK, struct {
		XMLName  xml.Name    `xml:"ListPartsResult"`
		Xmlns    string      `xml:"xmlns,attr"`
		Bucket   string      `xml:"Bucket"`
		Key      string      `xml:"Key"`
		UploadID string      `xml:"UploadId"`
		Parts    []partEntry `xml:"Part"`
	}{Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/", Bucket: "", Key: key, UploadID: uploadID, Parts: entries})
}

func s3ListMultipartUploads(c *gin.Context, tx bun.IDB, prefix, userID string) {
	ctx := c.Request.Context()
	var ups []model.S3MultipartUpload
	if err := tx.NewSelect().Model(&ups).Order("created_at DESC").Limit(100).Scan(ctx); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	type uploadEntry struct {
		Key      string `xml:"Key"`
		UploadID string `xml:"UploadId"`
	}
	entries := make([]uploadEntry, 0, len(ups))
	for _, u := range ups {
		entries = append(entries, uploadEntry{Key: u.S3Key, UploadID: u.UploadID})
	}
	c.XML(http.StatusOK, struct {
		XMLName xml.Name      `xml:"ListMultipartUploadsResult"`
		Xmlns   string        `xml:"xmlns,attr"`
		Bucket  string        `xml:"Bucket"`
		Uploads []uploadEntry `xml:"Upload"`
	}{Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/", Bucket: "", Uploads: entries})
}

// ---------------------------------------------------------------- API key CRUD

func ListS3Keys(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()
		var keys []model.S3APIKey
		if err := tx.NewSelect().Model(&keys).Order("created_at DESC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing keys: "+err.Error())
			return
		}
		for i := range keys {
			keys[i].EncryptedSecret = ""
		}
		Success(c, gin.H{"keys": keys})
	}
}

func CreateS3Key(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		var req struct {
			Name        string `json:"name"`
			Permissions string `json:"permissions"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		req.Name = trimSpace(req.Name)
		if req.Name == "" {
			Err(c, http.StatusBadRequest, "name is required")
			return
		}
		if req.Permissions == "" {
			req.Permissions = "readwrite"
		}
		if req.Permissions != "readwrite" && req.Permissions != "readonly" {
			Err(c, http.StatusBadRequest, "permissions must be readwrite or readonly")
			return
		}

		ak := "GO" + tokenHex(10)
		sk := tokenHex(20)
		enc, err := crypto.Encrypt(sk)
		if err != nil {
			Err(c, http.StatusInternalServerError, "encrypting secret: "+err.Error())
			return
		}
		k := &model.S3APIKey{
			ID:              uuid.New(),
			UserID:          userID,
			AccessKeyID:     ak,
			EncryptedSecret: enc,
			Name:            req.Name,
			Permissions:     req.Permissions,
			IsActive:        true,
		}
		if _, err := tx.NewInsert().Model(k).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "creating key: "+err.Error())
			return
		}
		Created(c, gin.H{"key": k, "accessKeyId": ak, "secretAccessKey": sk})
	}
}

func DeleteS3Key(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid key id")
			return
		}
		res, err := tx.NewDelete().Model((*model.S3APIKey)(nil)).Where("id = ?", id).Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "deleting key: "+err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			Err(c, http.StatusNotFound, "key not found")
			return
		}
		Msg(c, "key deleted")
	}
}

// ---------------------------------------------------------------- helpers

// dedupFileName returns name, or a "name (N)" variant when a file with the
// same name already exists in the folder. Needed for multipart, where two
// uploads of the same key would otherwise collide on name.
func dedupFileName(ctx context.Context, tx bun.IDB, name string, folderID *uuid.UUID) (string, error) {
	existing := map[string]bool{}
	q := tx.NewSelect().Model((*model.File)(nil)).Where("name = ?", name)
	if folderID != nil {
		q.Where("folder_id = ?", *folderID)
	} else {
		q.Where("folder_id IS NULL")
	}
	var files []model.File
	if err := q.Scan(ctx, &files); err != nil {
		return "", err
	}
	for _, f := range files {
		existing[f.Name] = true
	}
	return store.DedupName(name, existing), nil
}

// tokenHex returns n random bytes hex-encoded.
func tokenHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", uuid.New().String())
	}
	return hex.EncodeToString(b)
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
