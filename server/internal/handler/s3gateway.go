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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/config"
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
// segment is the workspace slug and — since the gateway is single-tenant per
// key — the only valid bucket name. X-Org-Slug is NOT required for object
// operations (enforcing it would break existing SigV4 clients); it is honored
// only by ListBuckets on the empty path to resolve the caller's tenant.
// Requests are authenticated with AWS SigV4 against a tenant-scoped
// s3_api_keys row, either via the Authorization header or a presigned query
// string (GET only).
func S3Gateway(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		p := strings.Trim(c.Param("path"), "/")

		// GET /api/s3 with no slug is ListBuckets (aws s3 ls).
		if p == "" {
			s3ListBuckets(c, db)
			return
		}

		parts := strings.SplitN(p, "/", 2)
		workspaceSlug := parts[0]
		key := ""
		if len(parts) > 1 {
			key = parts[1]
		}

		// Presigned URLs carry no Authorization header; they authenticate via
		// X-Amz-Credential/X-Amz-Signature query params instead.
		presigned := c.Request.Header.Get("Authorization") == ""
		var accessKeyID string
		var err error
		if presigned {
			accessKeyID, err = s3.PresignedAccessKeyID(c.Request)
		} else {
			accessKeyID, err = s3.AccessKeyID(c.Request)
		}
		if err != nil {
			s3Err(c, http.StatusForbidden, "InvalidAccessKeyId", "invalid access key id")
			return
		}

		// Strict bucket validation: the slug must be a real tenant. Checking
		// before OpenTx also prevents SQLite mode from auto-creating a phantom
		// tenant file for a mistyped bucket.
		if !bucketExists(ctx, db, workspaceSlug) {
			s3Err(c, http.StatusNotFound, "NoSuchBucket", "the specified bucket does not exist")
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
			s3Err(c, http.StatusForbidden, "InvalidAccessKeyId", "invalid access key id")
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
			s3Err(c, http.StatusForbidden, "InvalidAccessKeyId", "invalid access key id")
			return
		}
		if presigned {
			if c.Request.Method != http.MethodGet {
				s3Err(c, http.StatusForbidden, "AccessDenied", "presigned URLs are only supported for GET")
				return
			}
			err = s3.VerifyPresignedQuery(c.Request, secret)
		} else {
			err = s3.VerifySigV4(c.Request, secret)
		}
		if err != nil {
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
		// AWS SDKs send ?uploads with an empty value; q.Get would return ""
		// and the multipart branches would never match.
		_, hasUploads := q["uploads"]

		switch {
		case method == http.MethodHead && key == "":
			c.Status(http.StatusOK) // bucket validated above
		case method == http.MethodGet && hasUploads:
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
			s3PutObject(c, db, key, apiKey.UserID)
		case method == http.MethodPost && hasUploads:
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
	s, _, err := store.ResolveReadStore(ctx, tx, f.BlobID, f.StoragePath)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	st, err := store.BuildStorage(ctx, tx, s)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	// Download the file's own storage path (the object key), not the
	// blob_locations path which can diverge from the upload key and 404.
	body, size, err := st.Download(ctx, f.StoragePath)
	if err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchKey", "object not found")
		return
	}
	defer body.Close()

	c.Header("Content-Type", mimeTypeFor(f))
	c.Header("ETag", etagFor(f))
	c.Header("Last-Modified", f.UpdatedAt.UTC().Format(http.TimeFormat))
	if size >= 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	c.DataFromReader(http.StatusOK, size, mimeTypeFor(f), body, nil)
}

func s3HeadObject(c *gin.Context, tx bun.IDB, key string) {
	ctx := c.Request.Context()
	f, err := findFileByKey(ctx, tx, key)
	if err != nil {
		s3Err(c, http.StatusNotFound, "NoSuchKey", "object not found")
		return
	}
	c.Header("Content-Type", mimeTypeFor(f))
	c.Header("ETag", etagFor(f))
	c.Header("Last-Modified", f.UpdatedAt.UTC().Format(http.TimeFormat))
	c.Header("Content-Length", strconv.FormatInt(f.Size, 10))
	c.Status(http.StatusOK)
}

// ------------------------------------------------------------------ put/delete

func s3PutObject(c *gin.Context, db *bun.DB, key, userID string) {
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

	// Phase 1: Prepare and create pending record (short transaction)
	var blob *model.FileBlob
	var f *model.File
	var st storage.Storage
	var s *model.Store
	var contentType string
	err := func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		folderID, err := resolveOrCreateFolderChain(ctx, tx, userID, key)
		if err != nil {
			return err
		}
		contentType = c.GetHeader("Content-Type")
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(name))
		}
		size := c.Request.ContentLength
		if size <= 0 {
			size = 0
		}
		// S3 PUT overwrites: drop any existing row at this key so the UNIQUE
		// object_key constraint doesn't reject the re-upload.
		if existing, err := findFileByKey(ctx, tx, key); err == nil {
			if err := store.DeleteFileEverywhere(ctx, tx, existing.ID); err != nil {
				return err
			}
		}
		blob, f, err = store.CreatePendingFileUpload(ctx, tx, userID, folderID, name, key, contentType, size)
		if err != nil {
			return err
		}
		st, s, err = buildGatewayStorage(ctx, tx)
		if err != nil {
			return err
		}
		return tx.Commit()
	}()
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	// Phase 2: Upload file to storage (no database lock held)
	h := md5.New()
	body := io.TeeReader(c.Request.Body, h)
	if err := st.Upload(ctx, key, body, contentType); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	etag := `"` + hex.EncodeToString(h.Sum(nil)) + `"`

	// Phase 3: Mark file as ready (short transaction)
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	defer tx2.Rollback()

	if err := store.MarkFileUploadReady(ctx, tx2, f.ID, blob.ID, s.ID, key); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if mode, err := store.GetStorageMode(ctx, tx2); err == nil && mode == "replicate" {
		_ = store.SyncFileToStores(ctx, tx2, f.ID, nil, nil, userID)
	}
	if err := tx2.Commit(); err != nil {
		s3Err(c, http.StatusInternalServerError, "InternalError", err.Error())
		return
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

// s3Err writes an S3-compatible XML error response (with a per-request
// RequestId and a static HostId for AWS SDK compatibility) and records on the
// context that the request failed, so S3Gateway rolls back the transaction
// instead of committing partially-written records (e.g. orphaned pending file
// rows).
func s3Err(c *gin.Context, status int, code, message string) {
	c.Set("s3_errored", true)
	requestID := tokenHex(8)
	c.Header("x-amz-request-id", requestID)
	c.Header("Content-Type", "application/xml")
	c.Status(status)
	_ = xml.NewEncoder(c.Writer).Encode(s3ErrorResponse{
		XMLName:   xml.Name{Local: "Error"},
		Code:      code,
		Message:   message,
		RequestID: requestID,
		HostID:    s3HostID,
	})
}

type s3ErrorResponse struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	RequestID string   `xml:"RequestId"`
	HostID    string   `xml:"HostId"`
}

const s3HostID = "go-drive"

// mimeTypeFor returns the file's stored MIME type, defaulting to
// application/octet-stream when the row has none.
func mimeTypeFor(f *model.File) string {
	if f.MimeType != "" {
		return f.MimeType
	}
	return "application/octet-stream"
}

// ------------------------------------------------------------- bucket admin

type listAllMyBucketsResult struct {
	XMLName xml.Name   `xml:"ListAllMyBucketsResult"`
	Xmlns   string     `xml:"xmlns,attr"`
	Owner   s3Owner    `xml:"Owner"`
	Buckets []s3Bucket `xml:"Buckets>Bucket"`
}

type s3Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type s3Bucket struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

// bucketExists reports whether the tenant schema/DB for slug exists. In SQLite
// mode a file check is used because tenant.DB auto-creates a fresh file for
// any slug; in Postgres mode the public organizations table is authoritative.
func bucketExists(ctx context.Context, db *bun.DB, slug string) bool {
	if config.IsSQLite() {
		_, err := os.Stat(config.SQLiteTenantPath(slug))
		return err == nil
	}
	exists, err := db.NewSelect().Model((*model.Organization)(nil)).Where("slug = ?", slug).Exists(ctx)
	return err == nil && exists
}

// findTenantForKey scans the known tenants for the one that owns accessKeyID.
// Used by ListBuckets when no X-Org-Slug header is present.
func findTenantForKey(ctx context.Context, db *bun.DB, accessKeyID string) string {
	if config.IsSQLite() {
		entries, err := os.ReadDir(config.SQLiteTenantDir())
		if err != nil {
			return ""
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "tenant_") || !strings.HasSuffix(name, ".db") {
				continue
			}
			slug := strings.TrimSuffix(strings.TrimPrefix(name, "tenant_"), ".db")
			if tenantHasKey(ctx, db, slug, accessKeyID) {
				return slug
			}
		}
		return ""
	}
	var orgs []model.Organization
	if err := db.NewSelect().Model(&orgs).Scan(ctx); err != nil {
		return ""
	}
	for _, o := range orgs {
		if tenantHasKey(ctx, db, o.Slug, accessKeyID) {
			return o.Slug
		}
	}
	return ""
}

// tenantHasKey reports whether a tenant has an s3_api_keys row for the id.
func tenantHasKey(ctx context.Context, db *bun.DB, slug, accessKeyID string) bool {
	tx, err := tenant.OpenTx(ctx, db, slug)
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var k model.S3APIKey
	return tx.NewSelect().Model(&k).Where("access_key_id = ?", accessKeyID).Scan(ctx) == nil
}

// s3ListBuckets implements ListBuckets (aws s3 ls). The gateway is
// single-tenant per key, so the response contains exactly one bucket named
// after the caller's workspace slug. The tenant is resolved from the
// X-Org-Slug header when present, otherwise by scanning for the tenant that
// owns the access key.
func s3ListBuckets(c *gin.Context, db *bun.DB) {
	ctx := c.Request.Context()
	presigned := c.Request.Header.Get("Authorization") == ""
	var accessKeyID string
	var err error
	if presigned {
		accessKeyID, err = s3.PresignedAccessKeyID(c.Request)
	} else {
		accessKeyID, err = s3.AccessKeyID(c.Request)
	}
	if err != nil {
		s3Err(c, http.StatusForbidden, "InvalidAccessKeyId", "invalid access key id")
		return
	}

	slug := c.GetHeader("X-Org-Slug")
	if slug != "" {
		if !bucketExists(ctx, db, slug) {
			s3Err(c, http.StatusNotFound, "NoSuchBucket", "the specified bucket does not exist")
			return
		}
	} else {
		slug = findTenantForKey(ctx, db, accessKeyID)
		if slug == "" {
			s3Err(c, http.StatusForbidden, "InvalidAccessKeyId", "invalid access key id")
			return
		}
	}

	tx, err := tenant.OpenTx(ctx, db, slug)
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
		s3Err(c, http.StatusForbidden, "InvalidAccessKeyId", "invalid access key id")
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
		s3Err(c, http.StatusForbidden, "InvalidAccessKeyId", "invalid access key id")
		return
	}
	if presigned {
		err = s3.VerifyPresignedQuery(c.Request, secret)
	} else {
		err = s3.VerifySigV4(c.Request, secret)
	}
	if err != nil {
		s3Err(c, http.StatusForbidden, "SignatureDoesNotMatch", err.Error())
		return
	}

	ok = true
	_ = tx.Commit()

	c.XML(http.StatusOK, listAllMyBucketsResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Owner: s3Owner{ID: apiKey.UserID, DisplayName: apiKey.UserID},
		Buckets: []s3Bucket{{
			Name:         slug,
			CreationDate: apiKey.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		}},
	})
}
