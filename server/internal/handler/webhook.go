package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
)

// validWebhookEvents is the set of events a webhook can subscribe to.
var validWebhookEvents = map[string]bool{
	"file.upload":   true,
	"file.download": true,
	"file.delete":   true,
	"file.move":     true,
	"file.copy":     true,
	"share.create":  true,
	"share.delete":  true,
	"*":             true,
}

// ListWebhooks returns all webhooks for the tenant.
func ListWebhooks(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		webhooks, err := store.ListWebhooks(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "listing webhooks: "+err.Error())
			return
		}

		Success(c, gin.H{"webhooks": webhooks})
	}
}

// CreateWebhook creates a new webhook subscription.
func CreateWebhook(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		var req struct {
			Name       string   `json:"name"`
			TargetURL  string   `json:"targetUrl"`
			Secret     string   `json:"secret"`
			EventTypes []string `json:"eventTypes"`
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
		req.TargetURL = strings.TrimSpace(req.TargetURL)
		if req.TargetURL == "" {
			Err(c, http.StatusBadRequest, "targetUrl is required")
			return
		}
		if _, err := url.ParseRequestURI(req.TargetURL); err != nil {
			Err(c, http.StatusBadRequest, "targetUrl is not a valid URL")
			return
		}
		if len(req.EventTypes) == 0 {
			Err(c, http.StatusBadRequest, "at least one eventType is required")
			return
		}
		for _, et := range req.EventTypes {
			if !validWebhookEvents[et] {
				Err(c, http.StatusBadRequest, "invalid eventType: "+et)
				return
			}
		}

		w := &model.Webhook{
			ID:         uuid.New(),
			UserID:     userID,
			Name:       req.Name,
			TargetURL:  req.TargetURL,
			Secret:     req.Secret,
			EventTypes: req.EventTypes,
			IsActive:   true,
		}

		// Auto-generate a secret if none provided
		if w.Secret == "" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err == nil {
				w.Secret = hex.EncodeToString(b)
			}
		}

		if err := store.CreateWebhook(ctx, tx, w); err != nil {
			Err(c, http.StatusInternalServerError, "creating webhook: "+err.Error())
			return
		}

		auditLog(ctx, tx, userID, "webhook_create", "webhook", w.ID.String(), map[string]any{"name": w.Name, "target_url": w.TargetURL})
		Created(c, gin.H{"webhook": w})
	}
}

// DeleteWebhook removes a webhook.
func DeleteWebhook(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid webhook id")
			return
		}

		if err := store.DeleteWebhook(ctx, tx, id); err != nil {
			Err(c, http.StatusInternalServerError, "deleting webhook: "+err.Error())
			return
		}

		auditLog(ctx, tx, userID, "webhook_delete", "webhook", id.String(), nil)
		Msg(c, "webhook deleted")
	}
}

// ListWebhookDeliveries returns delivery attempts for a webhook.
func ListWebhookDeliveries(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid webhook id")
			return
		}

		p := ParsePagination(c)

		var total int
		q := tx.NewSelect().Model((*model.WebhookDelivery)(nil)).Where("webhook_id = ?", id)
		total, err = q.Count(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "counting deliveries: "+err.Error())
			return
		}

		deliveries, err := store.ListWebhookDeliveries(ctx, tx, id, p.PageSize, p.Offset)
		if err != nil {
			Err(c, http.StatusInternalServerError, "listing deliveries: "+err.Error())
			return
		}

		PaginatedResponse(c, "deliveries", deliveries, total, p)
	}
}
