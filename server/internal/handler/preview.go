package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"mime"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
	"go-drive/server/internal/tenant"
)

const (
	// previewTokenBytes is the number of random bytes in a preview token
	// (hex-encoded, so 32 bytes become 64 characters).
	previewTokenBytes = 32
	// previewTokenTTL is how long a preview token stays valid.
	previewTokenTTL = 10 * time.Minute
)

// CreateFilePreviewToken issues a short-lived preview token for a ready file.
// The raw token is returned once; only its SHA-256 hash is persisted. The raw
// token is also registered in the public link_tokens registry so the public
// GET /api/preview/:token endpoint can resolve the owning tenant.
func CreateFilePreviewToken(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		orgSlug := c.GetString("org_slug")
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid file id")
			return
		}

		var f model.File
		if err := tx.NewSelect().Model(&f).Where("id = ? AND status = 'ready'", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "file not found")
			return
		}

		raw := make([]byte, previewTokenBytes)
		if _, err := rand.Read(raw); err != nil {
			Err(c, http.StatusInternalServerError, "generating token: "+err.Error())
			return
		}
		token := hex.EncodeToString(raw)
		sum := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(sum[:])
		expiresAt := time.Now().Add(previewTokenTTL)

		if err := store.CreatePreviewToken(ctx, tx, f.ID, userID, tokenHash, expiresAt); err != nil {
			Err(c, http.StatusInternalServerError, "creating preview token: "+err.Error())
			return
		}
		if err := registerLinkToken(ctx, db, token, orgSlug, "preview", f.ID.String()); err != nil {
			Err(c, http.StatusInternalServerError, "registering preview token: "+err.Error())
			return
		}

		Success(c, gin.H{"token": token, "expiresAt": expiresAt, "url": "/api/preview/" + token})
	}
}

// PublicPreviewByToken streams a ready file inline using a short-lived preview
// token. Public endpoint — no session or password required.
func PublicPreviewByToken(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		rawToken := c.Param("token")

		row, err := resolveLinkToken(ctx, db, rawToken)
		if err != nil {
			Err(c, http.StatusInternalServerError, "resolving token: "+err.Error())
			return
		}
		if row == nil || row.LinkType != "preview" {
			Err(c, http.StatusNotFound, "preview not found")
			return
		}
		tx, err := tenant.OpenTx(ctx, db, row.OrgSlug)
		if err != nil {
			Err(c, http.StatusInternalServerError, "opening tenant: "+err.Error())
			return
		}
		ok := false
		defer func() {
			if !ok {
				_ = tx.Rollback()
			}
		}()

		sum := sha256.Sum256([]byte(rawToken))
		tokenHash := hex.EncodeToString(sum[:])
		tok, err := store.GetPreviewTokenByHash(ctx, tx, tokenHash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				Err(c, http.StatusNotFound, "preview not found")
				return
			}
			Err(c, http.StatusInternalServerError, "loading preview token: "+err.Error())
			return
		}
		if time.Now().After(tok.ExpiresAt) {
			Err(c, http.StatusNotFound, "preview not found")
			return
		}

		var f model.File
		if err := tx.NewSelect().Model(&f).Where("id = ? AND status = 'ready'", tok.FileID).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "file not found")
			return
		}

		s, path, err := store.ResolveReadStore(ctx, tx, f.BlobID, f.StoragePath)
		if err != nil {
			Err(c, http.StatusInternalServerError, "no active storage configured")
			return
		}
		st, err := store.BuildStorage(ctx, tx, s)
		if err != nil {
			Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
			return
		}
		r, size, err := st.Download(ctx, path)
		if err != nil {
			Err(c, http.StatusInternalServerError, "downloading file: "+err.Error())
			return
		}
		defer r.Close()

		mimeType := f.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": f.Name}))
		c.DataFromReader(http.StatusOK, size, mimeType, r, nil)
		ok = true
	}
}
