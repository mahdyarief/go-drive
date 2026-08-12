package handler

import (
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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
)

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
