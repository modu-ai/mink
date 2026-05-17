// Package oauth — auto-refresh logic with 60-second safety margin.
//
// Refresh checks whether the stored Codex OAuthToken is about to expire and,
// if so, silently obtains a new access_token using the stored refresh_token.
// The caller never sees a prompt or warning (SD-4, AC-CR-018).
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (ED-4, ED-5, SD-4, T-011, T-012)
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// safetyMargin is the duration before token expiry at which a proactive
// refresh is triggered.  A 60-second margin ensures that the access_token
// remains valid for at least this long after Refresh returns (AC-CR-016).
const safetyMargin = 60 * time.Second

// Refresh checks whether current is within the safety margin of expiry and,
// if so, exchanges the refresh_token for a new access_token using store.
//
// Behaviour:
//   - If time.Until(current.ExpiresAt) >= safetyMargin: returns current unchanged
//     (no network call is made).
//   - If expired or within safetyMargin: POSTs to the Codex token endpoint.
//   - On HTTP 200: updates current with new tokens, calls store.Store("codex", …),
//     and returns the updated token.
//   - On HTTP 400 {"error":"invalid_grant"} or HTTP 401: returns
//     credential.ErrReAuthRequired wrapped with provider context (ED-5).
//   - On 5xx or network error: returns a transient error (the caller may retry;
//     it is NOT wrapped as ErrReAuthRequired so retry logic can distinguish).
//
// @MX:ANCHOR: [AUTO] Refresh is called by the LLM-ROUTING-V2 layer and CLI
// on every Codex credential Load (fan_in >= 3).
// @MX:REASON: Token refresh is security-critical: a wrong return value (stale
// or nil token) would silently fail LLM API calls or expose expired credentials.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (ED-4, SD-4, T-011)
func Refresh(ctx context.Context, store credential.Service, current *credential.OAuthToken) (*credential.OAuthToken, error) {
	return refreshWithClient(ctx, store, current, &http.Client{Timeout: 30 * time.Second}, codexTokenURL)
}

// refreshWithClient is the testable core of Refresh that accepts an injectable
// HTTP client and token URL so that tests can target an httptest server.
func refreshWithClient(
	ctx context.Context,
	store credential.Service,
	current *credential.OAuthToken,
	client *http.Client,
	tokenURL string,
) (*credential.OAuthToken, error) {
	// If the token is still comfortably valid, return it unchanged.
	if time.Until(current.ExpiresAt) >= safetyMargin {
		return current, nil
	}

	// Perform the refresh grant.
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {codexClientID},
		"refresh_token": {current.RefreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("codex refresh: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		// Network error — transient, do not wrap as ErrReAuthRequired.
		return nil, fmt.Errorf("codex refresh: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("codex refresh: read response: %w", err)
	}

	// Detect invalid_grant (8-day idle expiry or revoked token) — ED-5.
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		// Parse body to distinguish invalid_grant from other 400/401 errors.
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &errBody)
		if errBody.Error == "invalid_grant" || resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("codex: %w", credential.ErrReAuthRequired)
		}
		// Other 400 errors are treated as transient.
		return nil, fmt.Errorf("codex refresh: token endpoint HTTP %d: %s", resp.StatusCode, string(body))
	}

	// 5xx — transient server error; do NOT wrap as ErrReAuthRequired.
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("codex refresh: token endpoint HTTP %d: %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codex refresh: unexpected HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse the successful response.
	var tok refreshResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("codex refresh: parse response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("codex refresh: response missing access_token")
	}

	newToken := &credential.OAuthToken{
		Provider:    current.Provider,
		AccessToken: tok.AccessToken,
		ExpiresAt:   time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second),
		Scope:       current.Scope,
	}

	// Honour refresh_token rotation: use new token if provided, else keep old.
	if tok.RefreshToken != "" {
		newToken.RefreshToken = tok.RefreshToken
	} else {
		newToken.RefreshToken = current.RefreshToken
	}

	// Persist the updated token so the next call uses the refreshed credentials.
	if err := store.Store(current.Provider, *newToken); err != nil {
		return nil, fmt.Errorf("codex refresh: persist updated token: %w", err)
	}

	return newToken, nil
}

// refreshResponse is the JSON shape of a refresh_token grant response.
type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"` // may be absent if rotation is disabled
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}
