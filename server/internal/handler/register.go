package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

const registerDisabledKey = "register_disabled"

// loadAppSetting reads a single app_settings value; empty string if unset.
func loadAppSetting(ctx context.Context, db *bun.DB, key string) (string, error) {
	var row model.AppSetting
	err := db.NewSelect().Model(&row).Where("key = ?", key).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return row.Value, nil
}

// loadRegisterDisabled reports whether user registration is disabled.
func loadRegisterDisabled(ctx context.Context, db *bun.DB) (bool, error) {
	v, err := loadAppSetting(ctx, db, registerDisabledKey)
	if err != nil {
		return false, err
	}
	return v == "true", nil
}

// PublicGetRegisterSetting returns whether registration is disabled. Public —
// the signup page reads it before rendering the form.
func PublicGetRegisterSetting(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		disabled, err := loadRegisterDisabled(c.Request.Context(), db)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to load settings")
			return
		}
		Success(c, gin.H{"register_disabled": disabled})
	}
}

// AdminGetRegisterSetting returns the current register-disabled flag.
func AdminGetRegisterSetting(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		disabled, err := loadRegisterDisabled(c.Request.Context(), db)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to load settings")
			return
		}
		Success(c, gin.H{"register_disabled": disabled})
	}
}

// AdminSaveRegisterSetting persists the register-disabled flag.
func AdminSaveRegisterSetting(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			RegisterDisabled bool `json:"register_disabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		value := "false"
		if body.RegisterDisabled {
			value = "true"
		}
		if err := setAppSetting(c.Request.Context(), db, registerDisabledKey, value); err != nil {
			Err(c, http.StatusInternalServerError, "failed to save settings")
			return
		}
		Success(c, gin.H{"register_disabled": body.RegisterDisabled})
	}
}

// RegisterDisabled blocks sign-up while registration is disabled. Mounted on
// the /auth/*path wildcard; it only blocks POST /auth/sign-up and lets all
// other auth requests (sign-in, sign-out, me) pass through.
func RegisterDisabled(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost || c.Request.URL.Path != "/auth/sign-up" {
			c.Next()
			return
		}
		disabled, err := loadRegisterDisabled(c.Request.Context(), db)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to load settings")
			c.Abort()
			return
		}
		if disabled {
			Err(c, http.StatusForbidden, "registration is disabled")
			c.Abort()
			return
		}
		c.Next()
	}
}
