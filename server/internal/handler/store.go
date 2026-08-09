package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/crypto"
	"go-drive/server/internal/model"
	"go-drive/server/internal/storage"
	"go-drive/server/internal/store"
)

// ListStores returns attached stores + the primary store id.
func ListStores(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var stores []model.Store
		if err := tx.NewSelect().Model(&stores).Order("created_at ASC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing stores: "+err.Error())
			return
		}
		// Backfill quota for GDrive stores that have never been measured (e.g.
		// connected before the quota columns existed) so the quota bar always
		// shows data without requiring a manual Test Connection.
		for i := range stores {
			s := &stores[i]
			if s.Provider != "gdrive" || s.Status != "active" || s.QuotaLimit != 0 {
				continue
			}
			st, err := store.BuildStorage(ctx, tx, s)
			if err != nil {
				continue
			}
			used, limit, err := st.Quota(ctx)
			if err != nil || limit == 0 {
				continue
			}
			now := time.Now()
			if _, err := tx.NewUpdate().Model((*model.Store)(nil)).
				Where("id = ?", s.ID).
				Set("quota_used = ?", used, "quota_limit = ?", limit).
				Set("last_tested_at = ?", now, "updated_at = ?", now).
				Exec(ctx); err == nil {
				s.QuotaUsed = used
				s.QuotaLimit = limit
			}
		}
		var primaryID *uuid.UUID
		var setting model.WorkspaceStorageSetting
		if err := tx.NewSelect().Model(&setting).Limit(1).Scan(ctx); err == nil {
			primaryID = &setting.PrimaryStoreID
		}
		Success(c, gin.H{"stores": stores, "primaryStoreId": primaryID})
	}
}

// storeRequest is the shared create/update body for stores. Credentials are
// stored encrypted in store_secrets; config holds provider-specific settings.
type storeRequest struct {
	Name         string         `json:"name"`
	Provider     string         `json:"provider"`
	WriteMode    string         `json:"writeMode"`
	IngestMode   string         `json:"ingestMode"`
	ReadPriority int            `json:"readPriority"`
	Config       map[string]any `json:"config"`
	Credentials  map[string]any `json:"credentials"`
}

// CreateStore attaches a new store. It tests the connection first; the first
// store becomes the workspace primary via workspace_storage_settings.
func CreateStore(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var req storeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		req.Name = trimSpace(req.Name)
		if req.Name == "" || req.Provider == "" {
			Err(c, http.StatusBadRequest, "name and provider are required")
			return
		}
		if req.Config == nil {
			req.Config = map[string]any{}
		}

		// A gdrive store can be attached with just client id/secret; the
		// refresh token is obtained later via the browser OAuth consent flow
		// (Connect Google Drive on the store card). Such stores are created
		// as "pending" and skip the connection test.
		refreshToken, _ := req.Credentials["refreshToken"].(string)
		gdrivePending := req.Provider == "gdrive" && strings.TrimSpace(refreshToken) == ""
		if gdrivePending {
			s := &model.Store{
				ID:           uuid.New(),
				Name:         req.Name,
				Provider:     req.Provider,
				Status:       "pending",
				WriteMode:    orStr(req.WriteMode, "write"),
				IngestMode:   orStr(req.IngestMode, "none"),
				ReadPriority: req.ReadPriority,
				Config:       req.Config,
			}
			if s.ReadPriority == 0 {
				s.ReadPriority = 100
			}
			if _, err := tx.NewInsert().Model(s).Exec(ctx); err != nil {
				Err(c, http.StatusInternalServerError, "creating store: "+err.Error())
				return
			}
			if err := saveStoreCredentials(ctx, tx, s.ID, req.Credentials); err != nil {
				Err(c, http.StatusInternalServerError, err.Error())
				return
			}
			count, err := tx.NewSelect().Model((*model.Store)(nil)).Count(ctx)
			if err == nil && count == 1 {
				if err := setPrimaryStore(ctx, tx, s.ID); err != nil {
					Err(c, http.StatusInternalServerError, err.Error())
					return
				}
			}
			Created(c, gin.H{"store": s})
			return
		}

		// Test the connection before persisting.
		tmp := &model.Store{
			ID:         uuid.New(),
			Name:       req.Name,
			Provider:   req.Provider,
			IngestMode: orStr(req.IngestMode, "none"),
			Config:     req.Config,
		}
		st, err := buildTestStorage(ctx, tmp, req.Credentials)
		if err != nil {
			Err(c, http.StatusBadRequest, "connection test failed: "+err.Error())
			return
		}
		if _, _, err := st.Quota(ctx); err != nil {
			Err(c, http.StatusBadRequest, "connection test failed: "+err.Error())
			return
		}

		now := time.Now()
		s := &model.Store{
			ID:           uuid.New(),
			Name:         req.Name,
			Provider:     req.Provider,
			Status:       "active",
			WriteMode:    orStr(req.WriteMode, "write"),
			IngestMode:   orStr(req.IngestMode, "none"),
			ReadPriority: req.ReadPriority,
			Config:       req.Config,
			LastTestedAt: &now,
		}
		if s.ReadPriority == 0 {
			s.ReadPriority = 100
		}
		if _, err := tx.NewInsert().Model(s).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "creating store: "+err.Error())
			return
		}
		if err := saveStoreCredentials(ctx, tx, s.ID, req.Credentials); err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}

		// First store becomes primary.
		count, err := tx.NewSelect().Model((*model.Store)(nil)).Count(ctx)
		if err == nil && count == 1 {
			if err := setPrimaryStore(ctx, tx, s.ID); err != nil {
				Err(c, http.StatusInternalServerError, err.Error())
				return
			}
		}
		Created(c, gin.H{"store": s})
	}
}

// UpdateStore updates a store's name/config/writeMode/ingestMode/readPriority.
func UpdateStore(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid store id")
			return
		}
		var req storeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		var s model.Store
		if err := tx.NewSelect().Model(&s).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "store not found")
			return
		}
		u := tx.NewUpdate().Model((*model.Store)(nil))
		if n := trimSpace(req.Name); n != "" && n != s.Name {
			u.Set("name = ?", n)
		}
		if req.WriteMode != "" && req.WriteMode != s.WriteMode {
			u.Set("write_mode = ?", req.WriteMode)
		}
		if req.IngestMode != "" && req.IngestMode != s.IngestMode {
			u.Set("ingest_mode = ?", req.IngestMode)
		}
		if req.ReadPriority != 0 {
			u.Set("read_priority = ?", req.ReadPriority)
		}
		if req.Config != nil {
			u.Set("config = ?", req.Config)
		}
		u.Set("updated_at = ?", time.Now())
		if _, err := u.Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating store: "+err.Error())
			return
		}
		if req.Credentials != nil {
			if err := saveStoreCredentials(ctx, tx, id, req.Credentials); err != nil {
				Err(c, http.StatusInternalServerError, err.Error())
				return
			}
		}
		var updated model.Store
		if err := tx.NewSelect().Model(&updated).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "reloading store: "+err.Error())
			return
		}
		Success(c, gin.H{"store": updated})
	}
}

// DeleteStore removes a store and its encrypted secret.
func DeleteStore(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid store id")
			return
		}
		if _, err := tx.NewDelete().Model((*model.StoreSecret)(nil)).Where("store_id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "deleting store secret: "+err.Error())
			return
		}
		res, err := tx.NewDelete().Model((*model.Store)(nil)).Where("id = ?", id).Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "deleting store: "+err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			Err(c, http.StatusNotFound, "store not found")
			return
		}
		Msg(c, "store deleted")
	}
}

// TestStore verifies connectivity and returns quota.
func TestStore(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid store id")
			return
		}
		var s model.Store
		if err := tx.NewSelect().Model(&s).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "store not found")
			return
		}
		st, err := store.BuildStorage(ctx, tx, &s)
		if err != nil {
			Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
			return
		}
		used, limit, err := st.Quota(ctx)
		if err != nil {
			Err(c, http.StatusBadRequest, "connection test failed: "+err.Error())
			return
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model((*model.Store)(nil)).
			Set("last_tested_at = ?", now, "updated_at = ?", now).
			Set("quota_used = ?", used, "quota_limit = ?", limit).
			Where("id = ?", id).
			Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating store: "+err.Error())
			return
		}
		Success(c, gin.H{"ok": true, "used": used, "limit": limit})
	}
}

// SetPrimaryStore designates a store as the workspace primary.
func SetPrimaryStore(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid store id")
			return
		}
		if err := tx.NewSelect().Model((*model.Store)(nil)).Where("id = ?", id).Scan(ctx, &model.Store{}); err != nil {
			Err(c, http.StatusNotFound, "store not found")
			return
		}
		if err := setPrimaryStore(ctx, tx, id); err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		Success(c, gin.H{"primaryStoreId": id})
	}
}

// SyncStatus returns the replication run history.
func SyncStatus(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var runs []model.ReplicationRun
		if err := tx.NewSelect().Model(&runs).Order("created_at DESC").Limit(50).Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing runs: "+err.Error())
			return
		}
		Success(c, gin.H{"runs": runs})
	}
}

// TriggerSync replicates every ready file to all writable stores.
func TriggerSync(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		run, err := store.SyncWorkspace(ctx, tx, nil, userID)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		Success(c, gin.H{"run": run})
	}
}

// TriggerIngest pulls objects from a read-only store into the workspace.
func TriggerIngest(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid store id")
			return
		}
		n, err := store.IngestFromStore(ctx, tx, id, userID)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		Success(c, gin.H{"ingested": n})
	}
}

// saveStoreCredentials encrypts the credentials JSON and upserts store_secrets.
func saveStoreCredentials(ctx context.Context, tx bun.IDB, storeID uuid.UUID, creds map[string]any) error {
	if creds == nil {
		return nil
	}
	raw, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	enc, err := crypto.Encrypt(string(raw))
	if err != nil {
		return err
	}
	secret := &model.StoreSecret{
		StoreID:              storeID,
		EncryptionVersion:    1,
		EncryptedCredentials: enc,
	}
	_, err = tx.NewInsert().Model(secret).
		On("CONFLICT (store_id) DO UPDATE SET encrypted_credentials = EXCLUDED.encrypted_credentials, updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	return err
}

// loadStoreCredentials reads + decrypts a store's stored credentials.
func loadStoreCredentials(ctx context.Context, tx bun.IDB, storeID uuid.UUID) (map[string]any, error) {
	var secret model.StoreSecret
	if err := tx.NewSelect().Model(&secret).Where("store_id = ?", storeID).Scan(ctx); err != nil {
		return nil, err
	}
	raw, err := crypto.Decrypt(secret.EncryptedCredentials)
	if err != nil {
		return nil, err
	}
	creds := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, err
	}
	return creds, nil
}

// setPrimaryStore upserts the workspace_storage_settings row.
func setPrimaryStore(ctx context.Context, tx bun.IDB, storeID uuid.UUID) error {
	_, err := tx.NewInsert().Model(&model.WorkspaceStorageSetting{
		WorkspaceID:    uuid.Nil, // schema-per-tenant: single row per schema
		PrimaryStoreID: storeID,
	}).On("CONFLICT (workspace_id) DO UPDATE SET primary_store_id = EXCLUDED.primary_store_id, updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	return err
}

// buildTestStorage builds a storage instance from a temp store + credentials
// without persisting anything (used by CreateStore's connection test). It
// duplicates BuildStorage's config assembly because the temp store has no
// persisted store_secrets row to decrypt.
func buildTestStorage(ctx context.Context, s *model.Store, creds map[string]any) (storage.Storage, error) {
	cfg := storage.Config{
		Provider:  s.Provider,
		BaseDir:   fmt.Sprint(s.Config["baseDir"]),
		PublicURL: fmt.Sprint(s.Config["publicUrl"]),
		Bucket:    fmt.Sprint(s.Config["bucket"]),
		Region:    fmt.Sprint(s.Config["region"]),
		Endpoint:  fmt.Sprint(s.Config["endpoint"]),
		FolderID:  fmt.Sprint(s.Config["folderId"]),
	}
	if creds != nil {
		cfg.AccessKey = fmt.Sprint(creds["accessKeyId"])
		cfg.SecretKey = fmt.Sprint(creds["secretAccessKey"])
		cfg.ClientID = fmt.Sprint(creds["clientId"])
		cfg.ClientSecret = fmt.Sprint(creds["clientSecret"])
		cfg.RefreshToken = fmt.Sprint(creds["refreshToken"])
	}
	return storage.New(ctx, cfg)
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
