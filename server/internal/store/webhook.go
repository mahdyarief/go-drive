package store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/crypto"
	"go-drive/server/internal/model"
	"go-drive/server/internal/tenant"
)

// CreateWebhook inserts a new webhook subscription.
// The secret is encrypted before storage.
func CreateWebhook(ctx context.Context, tx bun.IDB, w *model.Webhook) error {
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()

	// Encrypt the secret before storing
	if w.Secret != "" {
		enc, err := crypto.Encrypt(w.Secret)
		if err != nil {
			return fmt.Errorf("encrypting webhook secret: %w", err)
		}
		w.Secret = enc
	}

	_, err := tx.NewInsert().Model(w).Exec(ctx)
	return err
}

// ListWebhooks returns all webhooks for the tenant.
func ListWebhooks(ctx context.Context, tx bun.IDB) ([]model.Webhook, error) {
	var webhooks []model.Webhook
	err := tx.NewSelect().Model(&webhooks).Order("created_at DESC").Scan(ctx)
	return webhooks, err
}

// GetWebhook loads a webhook by ID.
func GetWebhook(ctx context.Context, tx bun.IDB, id uuid.UUID) (*model.Webhook, error) {
	var w model.Webhook
	err := tx.NewSelect().Model(&w).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// DeleteWebhook removes a webhook and its deliveries.
func DeleteWebhook(ctx context.Context, tx bun.IDB, id uuid.UUID) error {
	_, err := tx.NewDelete().Model((*model.Webhook)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

// ListWebhookDeliveries returns delivery attempts for a webhook.
func ListWebhookDeliveries(ctx context.Context, tx bun.IDB, webhookID uuid.UUID, limit, offset int) ([]model.WebhookDelivery, error) {
	var deliveries []model.WebhookDelivery
	err := tx.NewSelect().Model(&deliveries).
		Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	return deliveries, err
}

// DispatchEvent sends an event to all active webhooks that subscribe to it.
// This is async and best-effort — failures are logged but never fail the main operation.
// db is needed so the background goroutine can persist delivery records
// after the request transaction has committed. orgSlug identifies the tenant schema.
func DispatchEvent(ctx context.Context, tx bun.Tx, db *bun.DB, orgSlug string, eventType string, payload map[string]any) {
	var webhooks []model.Webhook
	err := tx.NewSelect().Model(&webhooks).
		Where("is_active = true").
		Scan(ctx)
	if err != nil {
		log.Printf("webhook: listing webhooks: %v", err)
		return
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: marshaling payload: %v", err)
		return
	}

	for _, w := range webhooks {
		if !subscribesTo(w.EventTypes, eventType) {
			continue
		}
		// Decrypt the secret for HMAC signing in the goroutine
		secret := w.Secret
		if secret != "" {
			if dec, err := crypto.Decrypt(secret); err == nil {
				secret = dec
			} else {
				log.Printf("webhook: decrypting secret for %s: %v", w.ID, err)
				secret = ""
			}
		}
		go deliverWebhook(w, secret, eventType, string(payloadJSON), db, orgSlug)
	}
}

// subscribesTo checks if the webhook's event_types includes eventType.
func subscribesTo(eventTypes []string, eventType string) bool {
	for _, et := range eventTypes {
		if et == eventType || et == "*" {
			return true
		}
	}
	return false
}

// deliverWebhook sends the event to the webhook's target URL with HMAC signature.
// secret is the already-decrypted plaintext secret for HMAC signing.
// db and orgSlug are used to persist the delivery record after the request tx has committed.
func deliverWebhook(w model.Webhook, secret, eventType, payload string, db *bun.DB, orgSlug string) {
	delivery := &model.WebhookDelivery{
		ID:        uuid.New(),
		WebhookID: w.ID,
		EventType: eventType,
		Payload:   payload,
	}

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		delivery.Attempts = attempt
		delivery.LastAttemptAt = time.Now()

		req, err := http.NewRequest("POST", w.TargetURL, bytes.NewBufferString(payload))
		if err != nil {
			delivery.ResponseBody = fmt.Sprintf("creating request: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Event", eventType)
		req.Header.Set("X-Webhook-Delivery", delivery.ID.String())

		// HMAC signature for payload verification.
		if secret != "" {
			sig := computeHMAC(payload, secret)
			req.Header.Set("X-Webhook-Signature", sig)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			delivery.ResponseBody = fmt.Sprintf("sending request: %v", err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			continue
		}

		delivery.ResponseCode = resp.StatusCode
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		delivery.ResponseBody = string(body)
		resp.Body.Close() // Close immediately, not deferred

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			delivery.Success = true
			break
		}

		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	// Persist delivery record to the tenant's schema using a fresh connection.
	persistDelivery(db, orgSlug, delivery)

	log.Printf("webhook: delivery %s to %s: success=%v, code=%d",
		delivery.ID, w.TargetURL, delivery.Success, delivery.ResponseCode)
}

// persistDelivery saves the delivery record to the tenant's schema.
func persistDelivery(db *bun.DB, orgSlug string, d *model.WebhookDelivery) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := tenant.OpenTx(ctx, db, orgSlug)
	if err != nil {
		log.Printf("webhook: persist delivery: opening tenant tx for %s: %v", orgSlug, err)
		return
	}
	defer tx.Rollback()

	if _, err := tx.NewInsert().Model(d).Exec(ctx); err != nil {
		log.Printf("webhook: persist delivery: inserting delivery %s: %v", d.ID, err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("webhook: persist delivery: committing delivery %s: %v", d.ID, err)
	}
}

// computeHMAC returns the HMAC-SHA256 signature of payload using secret.
func computeHMAC(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
