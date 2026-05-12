package anthropic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/provider/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnthropic_OAuthRefresh_Success는 AC-ADAPTER-003을 커버한다.
// 풀 엔트리 expires_at=now+2분, refreshMargin=5분
// → Refresh가 호출되어 새 access_token과 rotated refresh_token을 받음
func TestAnthropic_OAuthRefresh_Success(t *testing.T) {
	t.Parallel()

	// 토큰 엔드포인트 stub
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/oauth/token", r.URL.Path)

		var body struct {
			GrantType    string `json:"grant_type"`
			RefreshToken string `json:"refresh_token"`
			ClientID     string `json:"client_id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "refresh_token", body.GrantType)
		assert.NotEmpty(t, body.RefreshToken)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-token-abc",
			"refresh_token": "new-refresh-token-xyz",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	// 임시 credentials 파일 디렉터리
	dir := t.TempDir()
	credFile := filepath.Join(dir, "anthropic-oauth.json")
	require.NoError(t, os.WriteFile(credFile, []byte(`{
		"access_token": "old-token",
		"refresh_token": "old-refresh-token",
		"client_id": "test-client-id"
	}`), 0600))

	secretStore := provider.NewFileSecretStore(dir)

	refresher := anthropic.NewAnthropicRefresher(anthropic.RefresherOptions{
		SecretStore:   secretStore,
		HTTPClient:    tokenServer.Client(),
		TokenEndpoint: tokenServer.URL + "/v1/oauth/token",
	})

	cred := &credential.PooledCredential{
		ID:        "cred-1",
		Provider:  "anthropic",
		KeyringID: "anthropic-oauth",
		// expires_at = now + 2분 (refreshMargin=5분이면 갱신 필요)
		ExpiresAt: time.Now().Add(2 * time.Minute),
	}

	err := refresher.Refresh(context.Background(), cred)
	require.NoError(t, err)

	// 풀 엔트리의 ExpiresAt이 갱신되었는지 확인
	assert.True(t, cred.ExpiresAt.After(time.Now().Add(30*time.Minute)),
		"ExpiresAt은 현재 시각보다 최소 30분 이후여야 함")

	// credentials 파일에 새 토큰이 기록되었는지 확인
	token, err := secretStore.Resolve(context.Background(), "anthropic-oauth")
	require.NoError(t, err)
	assert.Equal(t, "new-access-token-abc", token)
}

// TestAnthropic_OAuthRefresh_ServerError는 토큰 엔드포인트 에러 시 에러를 반환하는지 검증한다.
func TestAnthropic_OAuthRefresh_ServerError(t *testing.T) {
	t.Parallel()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer tokenServer.Close()

	dir := t.TempDir()
	secretStore := provider.NewFileSecretStore(dir)

	refresher := anthropic.NewAnthropicRefresher(anthropic.RefresherOptions{
		SecretStore:   secretStore,
		HTTPClient:    tokenServer.Client(),
		TokenEndpoint: tokenServer.URL + "/v1/oauth/token",
	})

	cred := &credential.PooledCredential{
		ID:        "cred-1",
		Provider:  "anthropic",
		KeyringID: "anthropic-oauth",
	}

	err := refresher.Refresh(context.Background(), cred)
	assert.Error(t, err)
}
