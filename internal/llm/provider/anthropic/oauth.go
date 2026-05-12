package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
	"github.com/modu-ai/mink/internal/llm/provider"
	"go.uber.org/zap"
)

const (
	// defaultTokenEndpoint는 Anthropic OAuth 토큰 갱신 엔드포인트이다.
	defaultTokenEndpoint = "https://console.anthropic.com/v1/oauth/token"
)

// RefresherOptions는 AnthropicRefresher 생성 옵션이다.
type RefresherOptions struct {
	// SecretStore는 credential secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다.
	HTTPClient *http.Client
	// TokenEndpoint는 OAuth 토큰 갱신 엔드포인트이다.
	// 빈 문자열이면 defaultTokenEndpoint를 사용한다.
	TokenEndpoint string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// AnthropicRefresher는 Anthropic OAuth PKCE refresh를 구현한다.
// credential.Refresher 인터페이스를 구현한다.
type AnthropicRefresher struct {
	secretStore   provider.SecretStore
	httpClient    *http.Client
	tokenEndpoint string
	logger        *zap.Logger
}

// NewAnthropicRefresher는 AnthropicRefresher를 생성한다.
func NewAnthropicRefresher(opts RefresherOptions) *AnthropicRefresher {
	endpoint := opts.TokenEndpoint
	if endpoint == "" {
		endpoint = defaultTokenEndpoint
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &AnthropicRefresher{
		secretStore:   opts.SecretStore,
		httpClient:    httpClient,
		tokenEndpoint: endpoint,
		logger:        opts.Logger,
	}
}

// oauthTokenRequest는 토큰 갱신 요청 바디이다.
type oauthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
}

// oauthTokenResponse는 토큰 갱신 응답이다.
type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// rawCredFile은 credential JSON 파일의 raw 내용이다.
type rawCredFile struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
}

// Refresh는 만료된 Anthropic OAuth 토큰을 갱신한다.
// AC-ADAPTER-003 구현.
func (r *AnthropicRefresher) Refresh(ctx context.Context, cred *credential.PooledCredential) error {
	// credential 파일에서 refresh_token과 client_id 읽기
	credData, err := r.readRawCred(cred.KeyringID)
	if err != nil {
		return fmt.Errorf("oauth: credential 읽기 실패: %w", err)
	}

	if credData.RefreshToken == "" {
		return fmt.Errorf("oauth: refresh_token이 없음 (keyringID=%s)", cred.KeyringID)
	}

	// 토큰 갱신 요청
	reqBody := oauthTokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: credData.RefreshToken,
		ClientID:     credData.ClientID,
	}

	tokenResp, err := r.doTokenRequest(ctx, reqBody)
	if err != nil {
		return fmt.Errorf("oauth: 토큰 갱신 실패: %w", err)
	}

	// credential 업데이트 (in-place mutation — Refresher 계약)
	cred.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// secret store에 새 access_token 기록
	if r.secretStore != nil {
		if err := r.secretStore.WriteBack(ctx, cred.KeyringID, tokenResp.AccessToken); err != nil {
			return fmt.Errorf("oauth: access_token WriteBack 실패: %w", err)
		}
	}

	// rotated refresh_token 저장
	if tokenResp.RefreshToken != "" && tokenResp.RefreshToken != credData.RefreshToken {
		if err := r.storeRotatedRefreshToken(cred.KeyringID, tokenResp.RefreshToken, credData.ClientID); err != nil {
			if r.logger != nil {
				r.logger.Warn("rotated refresh_token 저장 실패",
					zap.String("keyring_id", cred.KeyringID),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

// readRawCred는 keyringID에 해당하는 credential 파일의 raw 내용을 읽는다.
func (r *AnthropicRefresher) readRawCred(keyringID string) (*rawCredFile, error) {
	// FileSecretStore 타입인 경우 파일에서 직접 읽는다.
	if fss, ok := r.secretStore.(*provider.FileSecretStore); ok {
		// CredentialFile을 통해 경로 순회(path traversal) 방어 로직을 재사용한다.
		path, err := fss.CredentialFile(keyringID)
		if err != nil {
			return nil, fmt.Errorf("oauth: 잘못된 keyringID: %w", err)
		}
		data, err := readFile(path)
		if err != nil {
			return &rawCredFile{}, nil
		}
		var raw rawCredFile
		_ = json.Unmarshal(data, &raw)
		return &raw, nil
	}
	return &rawCredFile{}, nil
}

// storeRotatedRefreshToken은 rotated refresh_token을 credential 파일에 기록한다.
func (r *AnthropicRefresher) storeRotatedRefreshToken(keyringID, refreshToken, clientID string) error {
	if fss, ok := r.secretStore.(*provider.FileSecretStore); ok {
		// CredentialFile을 통해 경로 순회(path traversal) 방어 로직을 재사용한다.
		path, err := fss.CredentialFile(keyringID)
		if err != nil {
			return fmt.Errorf("oauth: 잘못된 keyringID: %w", err)
		}
		existing := map[string]any{}
		if data, err := readFile(path); err == nil {
			_ = json.Unmarshal(data, &existing)
		}
		existing["refresh_token"] = refreshToken
		if clientID != "" {
			existing["client_id"] = clientID
		}
		raw, err := json.Marshal(existing)
		if err != nil {
			return err
		}
		return writeFileAtomic(path, raw)
	}
	return nil
}

// doTokenRequest는 OAuth 토큰 갱신 HTTP 요청을 수행한다.
func (r *AnthropicRefresher) doTokenRequest(ctx context.Context, reqBody oauthTokenRequest) (*oauthTokenResponse, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("요청 바디 직렬화 실패: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.tokenEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("응답 바디 읽기 실패: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("토큰 엔드포인트 응답 %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp oauthTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	return &tokenResp, nil
}

// Ensure AnthropicRefresher implements credential.Refresher at compile time.
var _ credential.Refresher = (*AnthropicRefresher)(nil)
