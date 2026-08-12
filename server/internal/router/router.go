package router

import (
	"io/fs"
	"time"

	authula "github.com/Authula/authula"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"go-drive/server/internal/handler"
	"go-drive/server/internal/middleware"
)

// New creates a configured Gin engine with all routes and middleware.
// If staticFiles is provided, the frontend is served from the embedded filesystem.
func New(auth *authula.Auth, db *bun.DB, staticFiles fs.FS) *gin.Engine {
	r := gin.Default()

	r.Use(middleware.CORS(), middleware.SecurityHeaders())

	// Auth routes (Authula handles sign-up, sign-in, sign-out, me).
	// Rate-limited to slow brute-force attempts on credentials.
	// RegisterDisabled blocks POST /auth/sign-up server-side when
	// registration is disabled, while letting all other auth paths through.
	r.Any("/auth/*path", middleware.RateLimit(30, time.Minute), handler.RegisterDisabled(db), gin.WrapH(auth.Handler()))

	// Public API routes
	r.GET("/api/health", handler.Health())
	r.GET("/api/message", handler.Message())
	r.GET("/api/settings/register", handler.PublicGetRegisterSetting(db))

	// Public link endpoints (M6) — token-authenticated, no session required.
	r.GET("/api/shared/:token", handler.PublicShareLink(db))
	r.GET("/api/shared/:token/download", handler.PublicShareDownload(db))
	r.GET("/api/shared/:token/raw", handler.PublicShareRaw(db))
	r.POST("/api/upload/public", handler.PublicUpload(db))
	r.GET("/api/tracked/:token", handler.PublicTrackedLink(db))
	r.GET("/api/tracked/:token/download", handler.PublicTrackedDownload(db))
	r.GET("/api/tracked/:token/raw", handler.PublicTrackedRaw(db))

	// External upload API (M9) — API-key authenticated, no session required.
	// RequireAPIKey resolves the owning org from the key; APIKeyTenantTx opens
	// the tenant transaction for that org.
	r.POST("/api/v1/uploads", middleware.RequireAPIKey(db, "files:upload"), middleware.APIKeyTenantTx(db), handler.PublicUploadByAPIKey(db))

	// Per-tenant Google Drive OAuth callback (PUBLIC — Google redirects the browser here)
	r.GET("/api/gdrive/store-callback", handler.GDriveStoreCallback())

	// Authenticated API routes (Bearer session token)
	authed := r.Group("/api", middleware.Auth(db))
	{
		authed.GET("/me", handler.Me(db))
		authed.POST("/sign-out", handler.SignOut(db))

		// Organization management
		authed.POST("/orgs", handler.CreateOrg(db))
		authed.GET("/orgs", handler.ListOrgs(db))
		authed.GET("/orgs/:slug", handler.GetOrg(db))
		authed.PATCH("/orgs/:slug", handler.UpdateOrg(db))
		authed.DELETE("/orgs/:slug", handler.DeleteOrg(db))
		authed.POST("/orgs/:slug/members", handler.AddMember(db))
		authed.DELETE("/orgs/:slug/members/:userId", handler.RemoveMember(db))
		authed.PATCH("/orgs/:slug/members/:userId", handler.UpdateMemberRole(db))
	}

	// Tenant-scoped API routes (Bearer + tenant middleware)
	tenant := r.Group("/api/t", middleware.Auth(db), middleware.Tenant(db))
	{
		tenant.GET("/status", handler.TenantStatus())
		tenant.POST("/upload", handler.UploadFile(db))
		tenant.GET("/storage/usage", handler.StorageUsage(db))
		tenant.GET("/storage/breakdown", handler.StorageBreakdown(db))

		// Storage backends (M7)
		tenant.GET("/stores", handler.ListStores(db))
		tenant.POST("/stores", handler.CreateStore(db))
		tenant.PATCH("/stores/:id", handler.UpdateStore(db))
		tenant.DELETE("/stores/:id", handler.DeleteStore(db))
		tenant.POST("/stores/:id/test", handler.TestStore(db))
		tenant.POST("/stores/:id/primary", handler.SetPrimaryStore(db))
		tenant.POST("/stores/:id/ingest", handler.TriggerIngest(db))
		tenant.GET("/stores/sync", handler.SyncStatus(db))
		tenant.POST("/stores/sync", handler.TriggerSync(db))
		tenant.PATCH("/storage-mode", handler.SetStorageMode(db))
		tenant.GET("/storage/routing-policy", handler.GetRoutingPolicy(db))
		tenant.PATCH("/storage/routing-policy", handler.UpdateRoutingPolicy(db))
		// Per-tenant Google Drive OAuth consent flow (Attach Store → connect from store card)
		tenant.POST("/stores/:id/gdrive/auth-url", handler.GDriveStoreAuthURL(db))
		tenant.GET("/stores/gdrive/complete", handler.GDriveStoreComplete(db))

		// File explorer (M5)
		tenant.GET("/files", handler.ListFiles(db))
		tenant.GET("/files/search", handler.SearchFiles(db))
		tenant.PATCH("/files/batch", handler.BatchMoveFiles(db))
		tenant.DELETE("/files/batch", handler.BatchDeleteFiles(db))
		tenant.GET("/files/:id", handler.GetFile(db))
		tenant.PATCH("/files/:id", handler.UpdateFile(db))
		tenant.DELETE("/files/:id", handler.DeleteFile(db))
		tenant.GET("/files/:id/download-url", handler.FileDownloadURL(db))

		tenant.GET("/folders", handler.ListFolders(db))
		tenant.GET("/folders/recent", handler.RecentFolders(db))
		tenant.GET("/folders/breadcrumbs", handler.FolderBreadcrumbs(db))
		tenant.POST("/folders", handler.CreateFolder(db))
		tenant.PATCH("/folders/:id", handler.UpdateFolder(db))
		tenant.DELETE("/folders/:id", handler.DeleteFolder(db))

		tenant.GET("/tags", handler.ListTags(db))
		tenant.POST("/tags", handler.CreateTag(db))
		tenant.PATCH("/tags/:id", handler.UpdateTag(db))
		tenant.DELETE("/tags/:id", handler.DeleteTag(db))
		tenant.POST("/tags/set-file-tags", handler.SetFileTags(db))
		tenant.POST("/tags/for-files", handler.TagsForFiles(db))

		// Links (M6)
		tenant.GET("/share-links", handler.ListShareLinks(db))
		tenant.POST("/share-links", handler.CreateShareLink(db))
		tenant.PATCH("/share-links/:id", handler.UpdateShareLink(db))
		tenant.DELETE("/share-links/:id", handler.DeleteShareLink(db))

		tenant.GET("/upload-links", handler.ListUploadLinks(db))
		tenant.POST("/upload-links", handler.CreateUploadLink(db))
		tenant.PATCH("/upload-links/:id", handler.UpdateUploadLink(db))
		tenant.DELETE("/upload-links/:id", handler.DeleteUploadLink(db))

		tenant.GET("/tracked-links", handler.ListTrackedLinks(db))
		tenant.POST("/tracked-links", handler.CreateTrackedLink(db))
		tenant.PATCH("/tracked-links/:id", handler.UpdateTrackedLink(db))
		tenant.DELETE("/tracked-links/:id", handler.DeleteTrackedLink(db))
		tenant.GET("/tracked-links/:id/events", handler.ListTrackedLinkEvents(db))

		// S3 API keys (M8)
		tenant.GET("/s3-keys", handler.ListS3Keys(db))
		tenant.POST("/s3-keys", handler.CreateS3Key(db))
		tenant.DELETE("/s3-keys/:id", handler.DeleteS3Key(db))

		// External API keys (M9) — manage upload API credentials
		tenant.GET("/api-keys", handler.ListAPIKeys(db))
		tenant.POST("/api-keys", handler.CreateAPIKey(db))
		tenant.DELETE("/api-keys/:id", handler.DeleteAPIKey(db))

		// Audit log
		tenant.GET("/audit-logs", handler.ListAuditLogs(db))
	}

	// Admin API routes (Bearer + admin role required)
	admin := r.Group("/api/admin", middleware.Auth(db), middleware.AdminAuth(db))
	{
		admin.GET("/orgs", handler.AdminListOrgs(db))
		admin.GET("/orgs/:slug", handler.AdminGetOrg(db))
		admin.PATCH("/orgs/:slug", handler.AdminUpdateOrg(db))
		admin.DELETE("/orgs/:slug", handler.AdminDeleteOrg(db))

		admin.GET("/users", handler.AdminListUsers(db))
		admin.POST("/users", handler.AdminCreateUser(auth, db))
		admin.PATCH("/users/:id", handler.AdminUpdateUser(auth, db))
		admin.DELETE("/users/:id", handler.AdminDeleteUser(db))

		// Google Drive settings (OAuth user flow)
		admin.GET("/settings/gdrive", handler.AdminGetGDriveSettings(db))
		admin.GET("/settings/gdrive/storage", handler.AdminGDriveStorage(db))
		admin.PUT("/settings/gdrive", handler.AdminSaveGDriveSettings(db))
		admin.POST("/settings/gdrive/auth-url", handler.AdminGDriveAuthURL(db))
		admin.POST("/settings/gdrive/disconnect", handler.AdminGDriveDisconnect(db))

		// Registration settings
		admin.GET("/settings/register", handler.AdminGetRegisterSetting(db))
		admin.PUT("/settings/register", handler.AdminSaveRegisterSetting(db))
	}

	// Google Drive OAuth callback (PUBLIC — Google redirects here after consent)
	r.GET("/api/admin/settings/gdrive/callback", handler.AdminGDriveCallback(db))

	// HMAC-signed local file serving (public, signature-verified)
	r.GET("/api/files/serve/*path", handler.ServeFile())

	// S3-compatible gateway (M8) — SigV4-authenticated, no session required.
	// Path: /api/s3/{workspaceSlug}/{key...}
	r.Any("/api/s3/*path", handler.S3Gateway(db))

	// Serve embedded frontend in production
	if staticFiles != nil {
		r.NoRoute(handler.Static(staticFiles))
	}

	return r
}
