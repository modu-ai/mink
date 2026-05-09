package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"
)

// DefaultWebhookSecretBytes is the byte length of the random secret used in
// the webhook path (32 bytes → 64 hex chars).
const DefaultWebhookSecretBytes = 32

// GenerateWebhookSecret produces a hex-encoded random secret suitable for
// use as both the URL path component and the X-Telegram-Bot-Api-Secret-Token
// header (REQ-MTGM-E07). It uses crypto/rand to ensure cryptographic strength.
func GenerateWebhookSecret() (string, error) {
	buf := make([]byte, DefaultWebhookSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("telegram webhook: random secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// WebhookPath returns the URL path component for a given secret token.
// The secret is embedded in the path so that an attacker cannot guess the
// endpoint URL even without the secret-token header.
func WebhookPath(secret string) string {
	return "/webhook/telegram/" + secret
}

// WebhookHandler returns an http.Handler that decodes Telegram update bodies,
// verifies the X-Telegram-Bot-Api-Secret-Token header, and dispatches each
// update to the supplied Handler.
//
// When secretToken is empty, the header check is skipped (useful for local dev).
//
// @MX:ANCHOR: [AUTO] WebhookHandler is the entry point for HTTP-driven update ingestion in webhook mode.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-E07; fan_in via bootstrap webhook branch, RegisterWebhook, and unit tests.
func WebhookHandler(handler Handler, secretToken string, logger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if secretToken != "" {
			if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != secretToken {
				logger.Warn("telegram webhook: secret token mismatch")
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		var raw models.Update
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			logger.Warn("telegram webhook: decode body failed", zap.Error(err))
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		updates := convertUpdates([]*models.Update{&raw})
		if len(updates) == 0 {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Use the request context but enforce a budget so a slow agent
		// does not block Telegram's retry loop.
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		if err := handler.Handle(ctx, updates[0]); err != nil {
			logger.Warn("telegram webhook: handler failed",
				zap.Error(err),
				zap.Int("update_id", updates[0].UpdateID))
		}
		w.WriteHeader(http.StatusOK)
	})
}

// RegisterWebhook attaches the webhook endpoint to the supplied http.ServeMux
// and registers the URL with Telegram via setWebhook. On failure it returns an
// error so the caller can decide whether to fall back to polling
// (REQ-MTGM-E07).
//
// publicBaseURL must be the externally reachable HTTPS origin
// (e.g. "https://goose.example.com"); the path is appended internally.
func RegisterWebhook(
	ctx context.Context,
	client Client,
	mux *http.ServeMux,
	handler Handler,
	publicBaseURL string,
	secret string,
	logger *zap.Logger,
) error {
	if publicBaseURL == "" {
		return fmt.Errorf("telegram webhook: publicBaseURL is required")
	}
	if secret == "" {
		return fmt.Errorf("telegram webhook: secret is required")
	}
	if mux == nil {
		return fmt.Errorf("telegram webhook: mux is required")
	}

	path := WebhookPath(secret)
	mux.Handle(path, WebhookHandler(handler, secret, logger))

	fullURL := publicBaseURL + path
	if err := client.SetWebhook(ctx, SetWebhookRequest{
		URL:            fullURL,
		SecretToken:    secret,
		AllowedUpdates: []string{"message", "callback_query"},
	}); err != nil {
		return fmt.Errorf("telegram webhook: setWebhook: %w", err)
	}
	logger.Info("telegram webhook registered",
		zap.String("path", path),
		zap.String("public_url", fullURL))
	return nil
}
