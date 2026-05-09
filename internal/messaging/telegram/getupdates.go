package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-telegram/bot/models"
)

// getUpdatesParams mirrors the Telegram getUpdates request body.
type getUpdatesParams struct {
	Offset  int `json:"offset,omitempty"`
	Limit   int `json:"limit,omitempty"`
	Timeout int `json:"timeout,omitempty"`
}

// telegramResponseRaw is the envelope returned by the Telegram Bot API.
type telegramResponseRaw struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"description,omitempty"`
}

// httpPostJSON is the shared HTTP POST helper for all Telegram API methods.
// It encodes params as JSON, posts to baseURL/bot<token>/<method>, and
// decodes the result field into dest.
//
// @MX:NOTE: [AUTO] Central HTTP helper; every Client method routes through here
// so the test httptest.Server intercepts all API calls via a single mux.
func httpPostJSON(ctx context.Context, baseURL, token, method string, params interface{}, dest interface{}) error {
	var body []byte
	var err error
	if params != nil {
		body, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal %s params: %w", method, err)
		}
	}

	url := fmt.Sprintf("%s/bot%s/%s", baseURL, token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s HTTP: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var env telegramResponseRaw
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if !env.OK {
		return fmt.Errorf("telegram API error (%s): %s (HTTP %d)", method, env.Error, resp.StatusCode)
	}

	if dest != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, dest); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
}

// httpClientGetUpdates performs the getUpdates call and returns raw model updates.
func httpClientGetUpdates(ctx context.Context, baseURL, token string, offset, timeoutSec int) ([]*models.Update, error) {
	params := getUpdatesParams{
		Offset:  offset,
		Timeout: timeoutSec,
	}

	var updates []*models.Update
	if err := httpPostJSON(ctx, baseURL, token, "getUpdates", params, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}
