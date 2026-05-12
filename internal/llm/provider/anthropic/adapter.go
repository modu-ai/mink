package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modu-ai/mink/internal/llm/cache"
	"github.com/modu-ai/mink/internal/llm/credential"
	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"github.com/modu-ai/mink/internal/message"
	"go.uber.org/zap"
)

const (
	// defaultAPIEndpoint는 Anthropic Messages API 엔드포인트이다.
	defaultAPIEndpoint = "https://api.anthropic.com"
	// anthropicVersion은 Anthropic API 버전이다.
	anthropicVersion = "2023-06-01"
	// requestTimeout은 non-streaming 요청 타임아웃이다.
	requestTimeout = provider.DefaultNonStreamDataTimeout
)

// AnthropicOptions는 AnthropicAdapter 생성 옵션이다.
type AnthropicOptions struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// CachePlanner는 캐시 계획자이다.
	CachePlanner *cache.BreakpointPlanner
	// CacheStrategy는 캐시 전략이다.
	CacheStrategy cache.CacheStrategy
	// CacheTTL은 캐시 TTL이다.
	CacheTTL cache.TTL
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// Refresher는 OAuth refresher이다 (optional).
	Refresher *AnthropicRefresher
	// APIEndpoint는 Anthropic API base URL이다.
	APIEndpoint string
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다.
	HTTPClient *http.Client
	// HeartbeatTimeout은 streaming heartbeat 타임아웃이다 (REQ-ADAPTER-013).
	// zero value이면 provider.DefaultStreamHeartbeatTimeout(60s)를 사용한다.
	HeartbeatTimeout time.Duration
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// AnthropicAdapter는 Anthropic Claude API 어댑터이다.
// provider.Provider 인터페이스를 구현한다.
type AnthropicAdapter struct {
	pool             *credential.CredentialPool
	tracker          *ratelimit.Tracker
	cachePlanner     *cache.BreakpointPlanner
	cacheStrategy    cache.CacheStrategy
	cacheTTL         cache.TTL
	secretStore      provider.SecretStore
	refresher        *AnthropicRefresher
	httpClient       *http.Client
	apiEndpoint      string
	heartbeatTimeout time.Duration
	logger           *zap.Logger
}

// New는 AnthropicAdapter를 생성한다.
func New(opts AnthropicOptions) (*AnthropicAdapter, error) {
	if opts.Pool == nil {
		return nil, fmt.Errorf("anthropic: Pool is required")
	}

	endpoint := opts.APIEndpoint
	if endpoint == "" {
		endpoint = defaultAPIEndpoint
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}

	hbTimeout := opts.HeartbeatTimeout
	if hbTimeout <= 0 {
		hbTimeout = provider.DefaultStreamHeartbeatTimeout
	}

	return &AnthropicAdapter{
		pool:             opts.Pool,
		tracker:          opts.Tracker,
		cachePlanner:     opts.CachePlanner,
		cacheStrategy:    opts.CacheStrategy,
		cacheTTL:         opts.CacheTTL,
		secretStore:      opts.SecretStore,
		refresher:        opts.Refresher,
		httpClient:       httpClient,
		apiEndpoint:      strings.TrimRight(endpoint, "/"),
		heartbeatTimeout: hbTimeout,
		logger:           opts.Logger,
	}, nil
}

// Name은 provider 이름을 반환한다.
func (a *AnthropicAdapter) Name() string { return "anthropic" }

// Capabilities는 Anthropic 어댑터의 기능 목록을 반환한다.
func (a *AnthropicAdapter) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Streaming:        true,
		Tools:            true,
		Vision:           true,
		Embed:            false,
		AdaptiveThinking: true,
		MaxContextTokens: 200000,
		MaxOutputTokens:  16000,
		// JSONMode is unsupported — Anthropic Messages API has no response_format field.
		// Capability gate in NewLLMCall will block json requests with ErrCapabilityUnsupported.
		// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-002
		JSONMode: false,
		// UserID is supported via the nested metadata.user_id field.
		// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-002
		UserID: true,
	}
}

// anthropicMetadata holds the optional user identifier for abuse tracking.
// Serialized as {"user_id": "..."} and omitted entirely when nil.
// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-006
type anthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// anthropicAPIRequest는 Anthropic Messages API 요청 바디이다.
type anthropicAPIRequest struct {
	Model       string                  `json:"model"`
	System      string                  `json:"system,omitempty"`
	Messages    []AnthropicMessage      `json:"messages"`
	Tools       []AnthropicTool         `json:"tools,omitempty"`
	MaxTokens   int                     `json:"max_tokens"`
	Temperature float64                 `json:"temperature,omitempty"`
	Thinking    *AnthropicThinkingParam `json:"thinking,omitempty"`
	Stream      bool                    `json:"stream"`
	// Metadata carries optional user_id for abuse tracking (REQ-AMEND-006).
	// Pointer + omitempty ensures the field is absent when UserID is empty.
	Metadata *anthropicMetadata `json:"metadata,omitempty"`
}

// Stream은 스트리밍 방식으로 LLM 응답을 반환한다.
// AC-ADAPTER-001, AC-ADAPTER-008, AC-ADAPTER-010 구현.
func (a *AnthropicAdapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	return a.stream(ctx, req, 0)
}

// stream은 내부 streaming 구현이다. retryCount로 재시도 횟수를 추적한다.
func (a *AnthropicAdapter) stream(ctx context.Context, req provider.CompletionRequest, retryCount int) (<-chan message.StreamEvent, error) {
	// 1. credential 획득
	cred, err := a.pool.Select(ctx)
	if err != nil {
		return nil, fmt.Errorf("anthropic: credential 선택 실패: %w", err)
	}
	lease := a.pool.AcquireLease(cred.ID)

	// 2. token 해결 (optional refresh)
	token, err := a.resolveToken(ctx, cred)
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		return nil, fmt.Errorf("anthropic: token 해결 실패: %w", err)
	}

	// 3. prompt cache 계획
	var plan *cache.CachePlan
	if a.cachePlanner != nil {
		plan, _ = a.cachePlanner.Plan(req.Messages, a.cacheStrategy, a.cacheTTL)
	}

	// 4. 메시지 변환
	system, msgs, err := ConvertMessages(req.Messages)
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		return nil, fmt.Errorf("anthropic: 메시지 변환 실패: %w", err)
	}
	if plan != nil {
		msgs = ApplyCacheMarkers(msgs, *plan)
	}

	// 5. tools 변환
	tools := ConvertTools(req.Tools)

	// 6. thinking 파라미터
	model := NormalizeModel(req.Route.Model)
	thinking := BuildThinkingParam(req.Thinking, model)

	// 7. max_tokens 결정
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = MaxOutputTokensFor(model)
	}

	// 8. API 요청 바디 구성
	apiReq := anthropicAPIRequest{
		Model:       model,
		System:      system,
		Messages:    msgs,
		Tools:       tools,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Thinking:    thinking,
		Stream:      true,
	}

	// UserID forwarding: inject as nested metadata.user_id when provided (REQ-AMEND-006).
	if req.Metadata.UserID != "" {
		apiReq.Metadata = &anthropicMetadata{UserID: req.Metadata.UserID}
	}

	// 로깅: PII 미포함 (REQ-ADAPTER-014)
	if a.logger != nil {
		a.logger.Debug("anthropic stream request",
			zap.String("provider", "anthropic"),
			zap.String("model", model),
			zap.Int("message_count", len(req.Messages)),
		)
	}

	// 9. HTTP 요청
	resp, err := a.doHTTPRequest(ctx, token, apiReq)
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		return nil, err
	}

	// 10. rate limit 헤더 전달 (REQ-ADAPTER-004)
	if a.tracker != nil {
		a.tracker.ParseHTTPHeader("anthropic", resp.Header, time.Now())
	}

	// 11. 429/402 처리 — 1회 재시도 (AC-ADAPTER-008)
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 402 {
		resp.Body.Close()
		if retryCount >= 1 {
			// 이미 재시도했음
			if lease != nil {
				lease.Release()
			}
			return errorStream("anthropic: rate limit exceeded after retry")
		}

		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		next, rotateErr := a.pool.MarkExhaustedAndRotate(ctx, cred.ID, resp.StatusCode, retryAfter)
		if rotateErr != nil {
			return errorStream(fmt.Sprintf("anthropic: credential rotation 실패: %v", rotateErr))
		}
		// MarkExhaustedAndRotate가 next를 leased=true로 반환하므로
		// 재귀 stream()에서 Select가 next를 다시 선택할 수 있도록 lease를 반환한다.
		if next != nil {
			_ = a.pool.Release(next)
		}
		// 새 credential로 재시도
		return a.stream(ctx, req, retryCount+1)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if lease != nil {
			lease.Release()
		}
		return errorStream(fmt.Sprintf("anthropic: API 응답 %d: %s", resp.StatusCode, string(body)))
	}

	// 12. SSE 스트림 변환 goroutine
	out := make(chan message.StreamEvent, 8)
	go func() {
		defer func() {
			if lease != nil {
				lease.Release()
			}
		}()
		ParseAndConvert(ctx, resp.Body, out, a.heartbeatTimeout, a.logger)
	}()

	return out, nil
}

// resolveToken은 credential에서 access token을 조회한다.
// 만료 임박 시 refresh를 수행한다.
func (a *AnthropicAdapter) resolveToken(ctx context.Context, cred *credential.PooledCredential) (string, error) {
	// 만료 임박 검사 (refreshMargin = 5분)
	const refreshMargin = 5 * time.Minute
	if a.refresher != nil && !cred.ExpiresAt.IsZero() && time.Until(cred.ExpiresAt) < refreshMargin {
		if err := a.refresher.Refresh(ctx, cred); err != nil {
			// refresh 실패는 경고로 처리하고 기존 token 사용
			if a.logger != nil {
				a.logger.Warn("token refresh 실패, 기존 token 사용", zap.Error(err))
			}
		}
	}

	if a.secretStore == nil {
		return "", fmt.Errorf("anthropic: SecretStore가 없음")
	}

	token, err := a.secretStore.Resolve(ctx, cred.KeyringID)
	if err != nil {
		return "", fmt.Errorf("anthropic: token 조회 실패: %w", err)
	}
	return token, nil
}

// doHTTPRequest는 Anthropic API에 HTTP 요청을 수행한다.
func (a *AnthropicAdapter) doHTTPRequest(ctx context.Context, token string, apiReq anthropicAPIRequest) (*http.Response, error) {
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: 요청 직렬화 실패: %w", err)
	}

	url := a.apiEndpoint + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: HTTP 요청 생성 실패: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("X-Api-Key", token) // some endpoints use this

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: HTTP 요청 실패: %w", err)
	}
	return resp, nil
}

// Complete는 blocking 방식으로 LLM 응답을 반환한다.
// 내부적으로 Stream을 호출하여 결과를 버퍼링한다.
func (a *AnthropicAdapter) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	ch, err := a.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	resp := &provider.CompletionResponse{
		Message: message.Message{Role: "assistant"},
	}

	var textBuilder strings.Builder
	for evt := range ch {
		switch evt.Type {
		case message.TypeTextDelta:
			textBuilder.WriteString(evt.Delta)
		case message.TypeMessageDelta:
			resp.StopReason = evt.StopReason
		case message.TypeError:
			if evt.Error != "" {
				return nil, fmt.Errorf("anthropic: stream error: %s", evt.Error)
			}
		}
	}

	text := textBuilder.String()
	if text != "" {
		resp.Message.Content = []message.ContentBlock{{Type: "text", Text: text}}
	}
	if resp.StopReason == "" {
		resp.StopReason = "end_turn"
	}

	return resp, nil
}

// errorStream은 에러 이벤트를 방출하고 닫힌 채널을 반환한다.
func errorStream(msg string) (<-chan message.StreamEvent, error) {
	ch := make(chan message.StreamEvent, 1)
	ch <- message.StreamEvent{Type: message.TypeError, Error: msg}
	close(ch)
	return ch, nil
}

// parseRetryAfter는 Retry-After 헤더 값을 time.Duration으로 변환한다.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 60 * time.Second
	}
	secs, err := strconv.Atoi(header)
	if err != nil {
		return 60 * time.Second
	}
	return time.Duration(secs) * time.Second
}

// Ensure AnthropicAdapter implements provider.Provider at compile time.
var _ provider.Provider = (*AnthropicAdapter)(nil)
