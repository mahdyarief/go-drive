package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gdrive "google.golang.org/api/drive/v3"

	"go-drive/server/internal/config"
	"go-drive/server/internal/drive"
	"go-drive/server/internal/model"
)

const (
	gdriveCallbackPath = "/api/admin/settings/gdrive/callback"
	oauthStateTTL      = 10 * time.Minute
)

// pendingOAuth holds a PKCE verifier until the OAuth callback arrives.
type pendingOAuth struct {
	verifier  string
	clientID  string
	expiresAt time.Time
}

var (
	oauthMu    sync.Mutex
	pendingMap = map[string]pendingOAuth{}
)

// gdriveSettingKeys are the app_settings keys backing the Google Drive config.
var gdriveSettingKeys = []string{
	"gdrive_client_id",
	"gdrive_client_secret",
	"gdrive_refresh_token",
	"gdrive_folder_id",
}

// loadGDriveSettings reads the Google Drive app_settings from the DB.
func loadGDriveSettings(ctx context.Context, db *bun.DB) (map[string]string, error) {
	var rows []model.AppSetting
	if err := db.NewSelect().Model(&rows).Where("key IN (?)", bun.In(gdriveSettingKeys)).Scan(ctx); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

// setAppSetting upserts a single key-value app setting.
func setAppSetting(ctx context.Context, db *bun.DB, key, value string) error {
	_, err := db.NewInsert().
		Model(&model.AppSetting{Key: key, Value: value}).
		On("CONFLICT (key) DO UPDATE SET value = EXCLUDED.value").
		Exec(ctx)
	return err
}

// maskClientID hides all but the first 6 and last 4 characters of an ID.
func maskClientID(id string) string {
	if id == "" {
		return ""
	}
	if len(id) <= 10 {
		return id[:2] + "…" + id[len(id)-2:]
	}
	return id[:6] + "…" + id[len(id)-4:]
}

// randomState returns a cryptographically random hex string for OAuth state.
func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand never fails on supported platforms; fall back to time+pid.
		return fmt.Sprintf("%d%d", time.Now().UnixNano(), os.Getpid())
	}
	return hex.EncodeToString(b)
}

// gdriveConfig builds an oauth2.Config for the given client credentials.
func gdriveConfig(clientID, clientSecret, refreshToken string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  config.BaseURL() + gdriveCallbackPath,
		Scopes:       []string{gdrive.DriveFileScope, gdrive.DriveReadonlyScope},
	}
}

// LoadGDriveConfigFromDB seeds the drive package from UI-managed app_settings
// when a client_id has been stored. Falls back to env config otherwise.
// Called once at startup after public migrations.
func LoadGDriveConfigFromDB(ctx context.Context, db *bun.DB) {
	s, err := loadGDriveSettings(ctx, db)
	if err != nil {
		log.Printf("[gdrive] failed to load settings at startup: %v", err)
		return
	}
	if s["gdrive_client_id"] == "" {
		return
	}
	cfg := drive.GetConfig()
	drive.SetConfig(drive.Config{
		ClientID:     s["gdrive_client_id"],
		ClientSecret: firstNonEmpty(s["gdrive_client_secret"], cfg.ClientSecret),
		RefreshToken: firstNonEmpty(s["gdrive_refresh_token"], cfg.RefreshToken),
		FolderID:     firstNonEmpty(s["gdrive_folder_id"], cfg.FolderID),
	})
}

// AdminGetGDriveSettings returns the current Google Drive configuration status.
// Never exposes the secret or refresh token.
func AdminGetGDriveSettings(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		s, err := loadGDriveSettings(ctx, db)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to load settings")
			return
		}

		// Prefer DB values, fall back to the in-memory config (seeded from env).
		cfg := drive.GetConfig()
		clientID := firstNonEmpty(s["gdrive_client_id"], cfg.ClientID)
		folderID := firstNonEmpty(s["gdrive_folder_id"], cfg.FolderID)
		refreshToken := firstNonEmpty(s["gdrive_refresh_token"], cfg.RefreshToken)

		Success(c, gin.H{
			"configured":       clientID != "" && folderID != "",
			"connected":        refreshToken != "",
			"folder_id":        folderID,
			"client_id_masked": maskClientID(clientID),
			"redirect_uri":     config.BaseURL() + gdriveCallbackPath,
		})
	}
}

// AdminGDriveStorage returns the account's Google Drive storage usage.
func AdminGDriveStorage(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		quota, err := drive.StorageQuota(c.Request.Context())
		if err != nil {
			Err(c, http.StatusBadGateway, "failed to fetch storage quota")
			return
		}
		Success(c, gin.H{
			"limit":                quota.Limit,
			"usage":                quota.Usage,
			"usage_in_drive":       quota.UsageInDrive,
			"usage_in_drive_trash": quota.UsageInDriveTrash,
		})
	}
}

// AdminSaveGDriveSettings persists the Google Drive client credentials.
func AdminSaveGDriveSettings(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			FolderID     string `json:"folder_id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		body.ClientID = strings.TrimSpace(body.ClientID)
		body.FolderID = strings.TrimSpace(body.FolderID)
		if body.ClientID == "" || body.FolderID == "" {
			Err(c, http.StatusBadRequest, "client_id and folder_id are required")
			return
		}

		ctx := c.Request.Context()
		s, err := loadGDriveSettings(ctx, db)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to load settings")
			return
		}
		refreshToken := firstNonEmpty(s["gdrive_refresh_token"], drive.GetConfig().RefreshToken)

		updates := map[string]string{
			"gdrive_client_id":     body.ClientID,
			"gdrive_client_secret": body.ClientSecret,
			"gdrive_folder_id":     body.FolderID,
		}
		for key, value := range updates {
			if err := setAppSetting(ctx, db, key, value); err != nil {
				log.Printf("[gdrive] saving %s: %v", key, err)
				Err(c, http.StatusInternalServerError, "failed to save settings")
				return
			}
		}

		drive.SetConfig(drive.Config{
			ClientID:     body.ClientID,
			ClientSecret: body.ClientSecret,
			RefreshToken: refreshToken,
			FolderID:     body.FolderID,
		})

		Msg(c, "settings saved")
	}
}

// AdminGDriveAuthURL starts the OAuth consent flow and returns the auth URL.
func AdminGDriveAuthURL(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		s, err := loadGDriveSettings(ctx, db)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to load settings")
			return
		}
		clientID := s["gdrive_client_id"]
		clientSecret := s["gdrive_client_secret"]
		if clientID == "" || clientSecret == "" {
			Err(c, http.StatusBadRequest, "save google drive client id and secret first")
			return
		}

		conf := gdriveConfig(clientID, clientSecret, "")
		verifier := oauth2.GenerateVerifier()
		state := randomState()
		// access_type=offline requests a refresh token; prompt=consent forces
		// Google to show the consent screen again so a fresh refresh token is
		// issued even for accounts that already authorized this app
		// (otherwise the exchange can succeed but return no refresh_token).
		authURL := conf.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))

		oauthMu.Lock()
		pendingMap[state] = pendingOAuth{verifier: verifier, clientID: clientID, expiresAt: time.Now().Add(oauthStateTTL)}
		oauthMu.Unlock()

		Success(c, gin.H{"auth_url": authURL})
	}
}

// gdriveAuthResultPage renders a minimal HTML result page after the OAuth
// callback. Works in both dev (Vite on :5173) and prod (same origin).
func gdriveAuthResultPage(ok bool, message string) string {
	emoji, title := "❌", "Connection failed"
	if ok {
		emoji, title = "✅", "Connected!"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><title>%s</title></head>
<body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center"><div style="font-size:48px">%s</div><h1>%s</h1><p>%s</p>
<p>Kembali ke tab Admin Settings.</p></div></body></html>`, title, emoji, title, message)
}

// AdminGDriveCallback is the OAuth redirect target (PUBLIC — Google redirects
// the browser here). Exchanges the code, persists the refresh token, and
// renders a success/error page.
func AdminGDriveCallback(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := c.Query("state")
		code := c.Query("code")

		oauthMu.Lock()
		p, ok := pendingMap[state]
		if ok {
			delete(pendingMap, state)
		}
		oauthMu.Unlock()

		if !ok || time.Now().After(p.expiresAt) || code == "" {
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte(gdriveAuthResultPage(false, "State invalid or expired. Ulangi dari halaman Settings.")))
			return
		}

		ctx := c.Request.Context()
		s, err := loadGDriveSettings(ctx, db)
		if err != nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte(gdriveAuthResultPage(false, "Gagal membaca settings.")))
			return
		}

		conf := gdriveConfig(p.clientID, s["gdrive_client_secret"], "")
		tok, err := conf.Exchange(ctx, code, oauth2.VerifierOption(p.verifier))
		if err != nil || tok.RefreshToken == "" {
			log.Printf("[gdrive] token exchange failed: %v", err)
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte(gdriveAuthResultPage(false, "Token exchange gagal. Coba lagi.")))
			return
		}

		if err := setAppSetting(ctx, db, "gdrive_refresh_token", tok.RefreshToken); err != nil {
			log.Printf("[gdrive] saving refresh token: %v", err)
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte(gdriveAuthResultPage(false, "Gagal menyimpan refresh token.")))
			return
		}

		drive.SetConfig(drive.Config{
			ClientID:     p.clientID,
			ClientSecret: s["gdrive_client_secret"],
			RefreshToken: tok.RefreshToken,
			FolderID:     firstNonEmpty(s["gdrive_folder_id"], drive.GetConfig().FolderID),
		})

		c.Data(http.StatusOK, "text/html; charset=utf-8",
			[]byte(gdriveAuthResultPage(true, "Google Drive berhasil dihubungkan.")))
	}
}

// AdminGDriveDisconnect clears the stored refresh token.
func AdminGDriveDisconnect(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if err := setAppSetting(ctx, db, "gdrive_refresh_token", ""); err != nil {
			Err(c, http.StatusInternalServerError, "failed to save settings")
			return
		}
		cfg := drive.GetConfig()
		drive.SetConfig(drive.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RefreshToken: "",
			FolderID:     cfg.FolderID,
		})
		Msg(c, "disconnected")
	}
}

// firstNonEmpty returns the first non-empty string argument.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
