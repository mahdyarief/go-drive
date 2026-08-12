package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/crypto"
	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
)

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
