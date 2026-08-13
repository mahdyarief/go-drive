package handler

import (
	"net/http"
	"time"

	authula "github.com/Authula/authula"
	authulamodels "github.com/Authula/authula/models"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

// AdminListUsers returns all users with their admin flag and organization
// membership count for the admin user management page.
func AdminListUsers(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		type userView struct {
			ID             string    `json:"id"`
			Name           string    `json:"name"`
			Email          string    `json:"email"`
			CreatedAt      time.Time `json:"created_at"`
			IsAdmin        bool      `json:"is_admin"`
			OrgCount       int       `json:"org_count"`
			QuotaLimit     int64     `json:"quota_limit"`
			QuotaAllocated int64     `json:"quota_allocated"`
		}
		var users []userView
		err := db.NewRaw(`
			SELECT u.id, u.name, u.email, u.created_at,
				EXISTS(SELECT 1 FROM admins a WHERE a.user_id = u.id) AS is_admin,
				(SELECT COUNT(*) FROM organization_members m WHERE m.user_id = CAST(u.id AS TEXT)) AS org_count,
				COALESCE((SELECT q.quota_limit FROM user_quotas q WHERE q.user_id = CAST(u.id AS TEXT)), 0) AS quota_limit,
				(SELECT COALESCE(SUM(oq.quota_limit), 0) FROM org_quotas oq WHERE oq.owner_user_id = CAST(u.id AS TEXT)) AS quota_allocated
			FROM users u
			ORDER BY u.created_at DESC
		`).Scan(ctx, &users)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to list users")
			return
		}

		Success(c, gin.H{"users": users})
	}
}

// AdminCreateUser creates a user + email-password account so the user can sign in.
func AdminCreateUser(auth *authula.Auth, db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
			IsAdmin  bool   `json:"is_admin"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request")
			return
		}
		if req.Name == "" || req.Email == "" || req.Password == "" {
			Err(c, http.StatusBadRequest, "name, email, and password are required")
			return
		}
		if len(req.Password) < 8 {
			Err(c, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		cs := auth.CoreServices()
		// Reject duplicate emails before inserting. GetByEmail returns
		// (nil, nil) when the email is free, so only a non-nil user means taken.
		if existing, err := cs.UserService.GetByEmail(ctx, req.Email); err == nil && existing != nil {
			Err(c, http.StatusConflict, "email already in use")
			return
		}
		// Email verification is disabled in this app, so mark it verified
		// (same behavior as the sign-up plugin's RequireEmailVerification=false).
		user, err := cs.UserService.Create(ctx, req.Name, req.Email, true, nil, nil)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to create user")
			return
		}
		hash, err := cs.PasswordService.Hash(req.Password)
		if err != nil {
			cs.UserService.Delete(ctx, user.ID) // rollback best-effort
			Err(c, http.StatusInternalServerError, "failed to hash password")
			return
		}
		if _, err := cs.AccountService.Create(ctx, user.ID, user.Email, authulamodels.AuthProviderEmail.String(), &hash); err != nil {
			cs.UserService.Delete(ctx, user.ID) // rollback best-effort
			Err(c, http.StatusInternalServerError, "failed to create account")
			return
		}
		if req.IsAdmin {
			if _, err := db.NewRaw("INSERT INTO admins (user_id) VALUES (?) ON CONFLICT DO NOTHING", user.ID).Exec(ctx); err != nil {
				Err(c, http.StatusInternalServerError, "failed to set admin flag")
				return
			}
		}
		Created(c, gin.H{"user": user})
	}
}

// AdminUpdateUser updates name/email/password and/or toggles the admin flag.
// Superset of the old AdminSetUserAdmin — backward compatible: the existing
// frontend sends {is_admin: bool} and still works.
func AdminUpdateUser(auth *authula.Auth, db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("id")
		var req struct {
			Name     *string `json:"name"`
			Email    *string `json:"email"`
			Password *string `json:"password"`
			IsAdmin  *bool   `json:"is_admin"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request")
			return
		}
		ctx := c.Request.Context()
		if _, err := auth.CoreServices().UserService.GetByID(ctx, userID); err != nil {
			Err(c, http.StatusNotFound, "user not found")
			return
		}
		cs := auth.CoreServices()

		if req.Name != nil && *req.Name != "" {
			if err := cs.UserService.UpdateFields(ctx, userID, map[string]any{"name": *req.Name}); err != nil {
				Err(c, http.StatusInternalServerError, "failed to update user")
				return
			}
		}
		if req.Email != nil && *req.Email != "" {
			if existing, err := cs.UserService.GetByEmail(ctx, *req.Email); err == nil && existing != nil && existing.ID != userID {
				Err(c, http.StatusConflict, "email already in use")
				return
			}
			if err := cs.UserService.UpdateFields(ctx, userID, map[string]any{"email": *req.Email}); err != nil {
				Err(c, http.StatusInternalServerError, "failed to update email")
				return
			}
		}
		if req.Password != nil {
			if len(*req.Password) < 8 {
				Err(c, http.StatusBadRequest, "password must be at least 8 characters")
				return
			}
			hash, err := cs.PasswordService.Hash(*req.Password)
			if err != nil {
				Err(c, http.StatusInternalServerError, "failed to hash password")
				return
			}
			if err := cs.AccountService.UpdateFields(ctx, userID, map[string]any{"password": hash}); err != nil {
				Err(c, http.StatusInternalServerError, "failed to update password")
				return
			}
		}
		if req.IsAdmin != nil {
			if *req.IsAdmin {
				if _, err := db.NewRaw("INSERT INTO admins (user_id) VALUES (?) ON CONFLICT DO NOTHING", userID).Exec(ctx); err != nil {
					Err(c, http.StatusInternalServerError, "failed to set admin flag")
					return
				}
			} else {
				// Prevent revoking the last admin to avoid locking everyone out.
				var isAdmin bool
				_ = db.NewRaw("SELECT EXISTS(SELECT 1 FROM admins WHERE user_id = ?)", userID).Scan(ctx, &isAdmin)
				if isAdmin {
					var adminCount int
					_ = db.NewRaw("SELECT COUNT(*) FROM admins").Scan(ctx, &adminCount)
					if adminCount <= 1 {
						Err(c, http.StatusBadRequest, "cannot revoke the last admin")
						return
					}
				}
				if _, err := db.NewRaw("DELETE FROM admins WHERE user_id = ?", userID).Exec(ctx); err != nil {
					Err(c, http.StatusInternalServerError, "failed to revoke admin")
					return
				}
			}
		}
		Success(c, gin.H{"ok": true})
	}
}

// AdminDeleteUser deletes a user, their sessions, admin flag, and memberships.
// Memberships must be deleted explicitly — organization_members has no FK
// cascade to users in the live schema.
func AdminDeleteUser(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("id")
		ctx := c.Request.Context()

		var count int
		if err := db.NewRaw("SELECT COUNT(*) FROM users WHERE id = ?", userID).Scan(ctx, &count); err != nil || count == 0 {
			Err(c, http.StatusNotFound, "user not found")
			return
		}

		// Prevent deleting the last admin.
		var isAdmin bool
		_ = db.NewRaw("SELECT EXISTS(SELECT 1 FROM admins WHERE user_id = ?)", userID).Scan(ctx, &isAdmin)
		if isAdmin {
			var adminCount int
			_ = db.NewRaw("SELECT COUNT(*) FROM admins").Scan(ctx, &adminCount)
			if adminCount <= 1 {
				Err(c, http.StatusBadRequest, "cannot delete the last admin")
				return
			}
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			Err(c, http.StatusInternalServerError, "internal error")
			return
		}
		defer tx.Rollback()

		// Delete memberships explicitly — no FK cascade exists in the live schema.
		if _, err := tx.ExecContext(ctx, "DELETE FROM organization_members WHERE user_id = ?", userID); err != nil {
			Err(c, http.StatusInternalServerError, "failed to delete user memberships")
			return
		}
		// Delete sessions first (revokes active logins).
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
			Err(c, http.StatusInternalServerError, "failed to delete user sessions")
			return
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM admins WHERE user_id = ?", userID); err != nil {
			Err(c, http.StatusInternalServerError, "failed to delete user admin flags")
			return
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID); err != nil {
			Err(c, http.StatusInternalServerError, "failed to delete user")
			return
		}

		if err := tx.Commit(); err != nil {
			Err(c, http.StatusInternalServerError, "internal error")
			return
		}

		Msg(c, "user deleted")
	}
}
