package handler

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gdrive "google.golang.org/api/drive/v3"

	"go-drive/server/internal/config"
	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
)

// storeGDriveCallbackPath is the PUBLIC OAuth redirect target for the
// per-tenant Attach Store flow (Google redirects the browser here after the
// user grants consent). It must match the RedirectURL sent to Google.
const storeGDriveCallbackPath = "/api/gdrive/store-callback"

// pendingStoreOAuth holds the PKCE verifier + target store until the OAuth
// callback completes. Scoped to orgSlug so only that tenant can claim it via
// GDriveStoreComplete (the callback itself is public and must not trust the
// browser-provided state alone).
type pendingStoreOAuth struct {
	verifier     string
	orgSlug      string
	storeID      uuid.UUID
	returnBase   string
	clientID     string
	clientSecret string
	refreshToken string
	expiresAt    time.Time
}

var (
	storeOAuthMu    sync.Mutex
	pendingStoreMap = map[string]pendingStoreOAuth{}
)

// storeOAuthConfig builds an oauth2.Config for one tenant's GDrive store
// credentials. Distinct from gdriveConfig (admin flow) because the callback
// path differs.
func storeOAuthConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  config.BaseURL() + storeGDriveCallbackPath,
		Scopes:       []string{gdrive.DriveFileScope, gdrive.DriveReadonlyScope},
	}
}

// GDriveStoreAuthURL starts the OAuth consent flow for an existing GDrive
// store. The client id/secret come from the store's saved credentials (set at
// Attach Store time); the refresh token is obtained from Google and stored
// back by GDriveStoreComplete.
func GDriveStoreAuthURL(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgSlug := c.GetString("org_slug")
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
		if s.Provider != "gdrive" {
			Err(c, http.StatusBadRequest, "store is not a google drive store")
			return
		}
		creds, err := loadStoreCredentials(ctx, tx, id)
		if err != nil {
			Err(c, http.StatusBadRequest, "store credentials not found")
			return
		}
		// Comma-ok assertion: a missing/wrong-typed key becomes "" so the
		// required check below actually fires.
		clientID, _ := creds["clientId"].(string)
		clientSecret, _ := creds["clientSecret"].(string)
		clientID = strings.TrimSpace(clientID)
		clientSecret = strings.TrimSpace(clientSecret)
		if clientID == "" || clientSecret == "" {
			Err(c, http.StatusBadRequest, "google drive client id and secret are required; update the store first")
			return
		}

		// Redirect the user back to the origin they started from. In dev the
		// frontend is served by Vite (:5173) while the API + OAuth callback
		// live on :8081, so config.BaseURL() alone would land on the Go
		// server's empty embedded static dir.
		returnBase := c.GetHeader("Origin")
		if returnBase == "" || returnBase == "null" {
			returnBase = config.BaseURL()
		}

		conf := storeOAuthConfig(clientID, clientSecret)
		verifier := oauth2.GenerateVerifier()
		state := randomState()
		// access_type=offline requests a refresh token; prompt=consent forces
		// Google to show the consent screen again so a fresh refresh token is
		// issued even for accounts that already authorized this app
		// (otherwise the exchange can succeed but return no refresh_token).
		authURL := conf.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))

		storeOAuthMu.Lock()
		pendingStoreMap[state] = pendingStoreOAuth{
			verifier:     verifier,
			orgSlug:      orgSlug,
			storeID:      id,
			returnBase:   returnBase,
			clientID:     clientID,
			clientSecret: clientSecret,
			expiresAt:    time.Now().Add(oauthStateTTL),
		}
		storeOAuthMu.Unlock()

		Success(c, gin.H{"auth_url": authURL})
	}
}

// gdriveStoreResultPage renders a minimal HTML result page after the OAuth
// callback. Works in both dev (Vite on :5173) and prod (same origin) because
// the callback renders the page itself instead of redirecting to the SPA —
// a 302 to the Go server's static root would land on the empty embedded dir.
func gdriveStoreResultPage(ok bool, title, message, backURL string) string {
	emoji := "❌"
	if ok {
		emoji = "✅"
	}
	// backURL is derived from a browser-supplied Origin header, so escape it
	// before embedding in the href to avoid HTML/attribute injection.
	backURL = html.EscapeString(backURL)
	// On success, auto-forward the browser back to the app so the SPA can
	// finish the connection via GDriveStoreComplete (the "Kembali" link stays
	// as a fallback for users on browsers that disable meta refresh).
	meta := ""
	if ok {
		meta = fmt.Sprintf(`<meta http-equiv="refresh" content="2;url=%s">`, backURL)
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="id"><head><meta charset="utf-8">%s<title>%s</title></head>
<body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center"><div style="font-size:48px">%s</div><h1>%s</h1><p>%s</p>
<p><a href="%s" style="color:#2563eb">Kembali ke halaman Stores</a></p></div></body></html>`, meta, title, emoji, title, message, backURL)
}

// GDriveStoreCallback is the OAuth redirect target (PUBLIC — Google redirects
// the browser here after consent). It exchanges the code for tokens via PKCE,
// stashes the refresh token back into the pending entry, and renders an HTML
// result page (like manajemen-tiket). The "Kembali ke halaman Stores" link
// carries the state back to the SPA, which finishes the connection with
// GDriveStoreComplete.
func GDriveStoreCallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		state := c.Query("state")
		code := c.Query("code")

		storeOAuthMu.Lock()
		p, ok := pendingStoreMap[state]
		storeOAuthMu.Unlock()

		if !ok || time.Now().After(p.expiresAt) || code == "" {
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte(gdriveStoreResultPage(false, "Connection failed", "State invalid or expired. Ulangi dari halaman Stores.", config.BaseURL()+"/app/settings/stores")))
			return
		}

		conf := storeOAuthConfig(p.clientID, p.clientSecret)
		tok, err := conf.Exchange(c.Request.Context(), code, oauth2.VerifierOption(p.verifier))
		if err != nil || tok.RefreshToken == "" {
			log.Printf("[gdrive store] token exchange failed: %v", err)
			storeOAuthMu.Lock()
			delete(pendingStoreMap, state)
			storeOAuthMu.Unlock()
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte(gdriveStoreResultPage(false, "Connection failed", "Token exchange gagal. Coba lagi.", p.returnBase+"/app/settings/stores")))
			return
		}

		storeOAuthMu.Lock()
		p.refreshToken = tok.RefreshToken
		pendingStoreMap[state] = p
		storeOAuthMu.Unlock()

		// The callback is public and has no tenant DB context, so the refresh
		// token is finished by GDriveStoreComplete after the user returns to
		// the app.
		c.Data(http.StatusOK, "text/html; charset=utf-8",
			[]byte(gdriveStoreResultPage(true, "Connected!", "Google Drive berhasil dihubungkan.", p.returnBase+"/app/settings/stores?gdrive=connected&state="+state)))
	}
}

// GDriveStoreComplete saves the refresh token into the store's credentials,
// tests the connection, and flips the store to "active". The pending entry is
// intentionally NOT consumed here: the complete call can legitimately fire
// more than once (React StrictMode double-mounts effects in dev, or the user
// refreshes the store page after the OAuth redirect), and the entry is
// org-scoped + short-TTL so replays are harmless. It expires via oauthStateTTL.
func GDriveStoreComplete(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgSlug := c.GetString("org_slug")
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()
		state := c.Query("state")

		storeOAuthMu.Lock()
		p, ok := pendingStoreMap[state]
		storeOAuthMu.Unlock()

		if !ok || p.orgSlug != orgSlug || p.refreshToken == "" {
			Err(c, http.StatusBadRequest, "gdrive connection expired or invalid")
			return
		}

		var s model.Store
		if err := tx.NewSelect().Model(&s).Where("id = ?", p.storeID).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "store not found")
			return
		}

		creds, err := loadStoreCredentials(ctx, tx, p.storeID)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading store credentials: "+err.Error())
			return
		}
		creds["refreshToken"] = p.refreshToken
		if err := saveStoreCredentials(ctx, tx, p.storeID, creds); err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}

		markFailed := func() {
			_, _ = tx.NewUpdate().Model((*model.Store)(nil)).
				Where("id = ?", p.storeID).
				Set("status = ?", "error").
				Set("updated_at = ?", time.Now()).
				Exec(ctx)
		}

		st, err := store.BuildStorage(ctx, tx, &s)
		if err != nil {
			markFailed()
			Err(c, http.StatusBadRequest, "building storage: "+err.Error())
			return
		}
		used, limit, err := st.Quota(ctx)
		if err != nil {
			markFailed()
			Err(c, http.StatusBadRequest, "connection test failed: "+err.Error())
			return
		}

		now := time.Now()
		if _, err := tx.NewUpdate().Model((*model.Store)(nil)).
			Where("id = ?", p.storeID).
			Set("status = ?", "active").
			Set("last_tested_at = ?", now).
			Set("quota_used = ?", used, "quota_limit = ?", limit).
			Set("updated_at = ?", now).
			Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating store: "+err.Error())
			return
		}

		Success(c, gin.H{"ok": true, "used": used, "limit": limit, "storeId": p.storeID.String()})
	}
}
