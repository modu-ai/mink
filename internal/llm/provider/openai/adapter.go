// Package openai는 OpenAI API 호환 어댑터를 구현한다.
// xAI (Grok), DeepSeek 등 OpenAI API 호환 provider도 이 어댑터를 재사용한다.
// SPEC-GOOSE-ADAPTER-001 M2 T-030
package openai

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

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/zap"
)

const (
	// defaultBaseURL은 OpenAI API의 기본 엔드포인트이다.
	defaultBaseURL = "https://api.openai.com/v1"
	// requestTimeout은 non-streaming 요청 타임아웃이다.
	requestTimeout = 30 * time.Second
)

// OpenAIOptions는 OpenAIAdapter 생성 옵션이다.
type OpenAIOptions struct {
	// Name은 provider 이름이다 (예: "openai", "xai", "deepseek").
	Name string
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 defaultBaseURL 사용.
	BaseURL string
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다.
	HTTPClient *http.Client
	// Capabilities는 이 어댑터의 기능 목록이다.
	Capabilities provider.Capabilities
	// HeartbeatTimeout은 streaming heartbeat 타임아웃이다 (REQ-ADAPTER-013).
	// zero value이면 provider.DefaultStreamHeartbeatTimeout(60s)를 사용한다.
	HeartbeatTimeout time.Duration
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// OpenAIAdapter는 OpenAI API 호환 어댑터이다.
// provider.Provider 인터페이스를 구현한다.
type OpenAIAdapter struct {
	pool             *credential.CredentialPool
	tracker          *ratelimit.Tracker
	secretStore      provider.SecretStore
	httpClient       *http.Client
	baseURL          string
	providerName     string
	caps             provider.Capabilities
	heartbeatTimeout time.Duration
	logger           *zap.Logger
}

// New는 OpenAIAdapter를 생성한다.
func New(opts OpenAIOptions) (*OpenAIAdapter, error) {
	if opts.Pool == nil {
		return nil, fmt.Errorf("openai: Pool is required")
	}
	if opts.SecretStore == nil {
		return nil, fmt.Errorf("openai: SecretStore is required")
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	name := opts.Name
	if name == "" {
		name = "openai"
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}

	caps := opts.Capabilities
	// 기본 Capabilities 설정
	if !caps.Streaming && !caps.Tools && !caps.Vision {
		caps = provider.Capabilities{
			Streaming: true,
			Tools:     true,
			Vision:    true,
		}
	}

	hbTimeout := opts.HeartbeatTimeout
	if hbTimeout <= 0 {
		hbTimeout = provider.DefaultStreamHeartbeatTimeout
	}

	return &OpenAIAdapter{
		pool:             opts.Pool,
		tracker:          opts.Tracker,
		secretStore:      opts.SecretStore,
		httpClient:       httpClient,
		baseURL:          strings.TrimRight(baseURL, "/"),
		providerName:     name,
		caps:             caps,
		heartbeatTimeout: hbTimeout,
		logger:           opts.Logger,
	}, nil
}

// Name은 provider 이름을 반환한다.
func (a *OpenAIAdapter) Name() string { return a.providerName }

// Capabilities는 어댑터의 기능 목록을 반환한다.
func (a *OpenAIAdapter) Capabilities() provider.Capabilities { return a.caps }

// openAIRequest는 OpenAI chat completions API 요청 바디이다.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMsg     `json:"messages"`
	Tools       []OpenAIToolDef `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
}

// openAIMsg는 OpenAI chat completions API 메시지이다.
type openAIMsg struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"` // string 또는 []openAIMsgPart
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	Name       string           `json:"name,omitempty"`
}

// openAIMsgPart는 멀티파트 메시지 조각이다.
type openAIMsgPart struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *openAIImage `json:"image_url,omitempty"`
}

// openAIImage는 OpenAI 이미지 URL/data이다.
type openAIImage struct {
	URL string `json:"url"`
}

// openAIToolCall은 OpenAI tool call이다.
type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Stream은 스트리밍 방식으로 LLM 응답을 반환한다.
// AC-ADAPTER-004, REQ-003 (ctx propagation), REQ-005 (credential rotation), REQ-013 (timeout)
func (a *OpenAIAdapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	return a.stream(ctx, req, 0)
}

// stream은 내부 streaming 구현이다. retryCount로 재시도 횟수를 추적한다.
func (a *OpenAIAdapter) stream(ctx context.Context, req provider.CompletionRequest, retryCount int) (<-chan message.StreamEvent, error) {
	// 1. credential 획득
	cred, err := a.pool.Select(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: credential 선택 실패: %w", a.providerName, err)
	}

	// 2. secret 해결
	token, err := a.secretStore.Resolve(ctx, cred.KeyringID)
	if err != nil {
		return nil, fmt.Errorf("%s: token 조회 실패: %w", a.providerName, err)
	}

	// 3. 메시지 변환
	msgs := convertMessages(req.Messages)

	// 4. tools 변환
	var tools []OpenAIToolDef
	if len(req.Tools) > 0 {
		tools = ConvertTools(req.Tools)
	}

	// 5. API 요청 바디 구성
	model := req.Route.Model
	apiReq := openAIRequest{
		Model:       model,
		Messages:    msgs,
		Tools:       tools,
		MaxTokens:   req.MaxOutputTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}

	if a.logger != nil {
		a.logger.Debug("openai stream request",
			zap.String("provider", a.providerName),
			zap.String("model", model),
			zap.Int("message_count", len(req.Messages)),
		)
	}

	// 6. HTTP 요청
	resp, err := a.doRequest(ctx, token, apiReq)
	if err != nil {
		return nil, err
	}

	// 7. rate limit 헤더 전달
	if a.tracker != nil {
		a.tracker.Parse(a.providerName, resp.Header, time.Now())
	}

	// 8. 429/402 처리 — 1회 재시도
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 402 {
		resp.Body.Close()
		if retryCount >= 1 {
			return errorStream(fmt.Sprintf("%s: rate limit exceeded after retry", a.providerName))
		}
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		next, rotateErr := a.pool.MarkExhaustedAndRotate(ctx, cred.ID, resp.StatusCode, retryAfter)
		if rotateErr != nil {
			return errorStream(fmt.Sprintf("%s: credential rotation 실패: %v", a.providerName, rotateErr))
		}
		// MarkExhaustedAndRotate가 next를 leased 상태로 반환하므로
		// 재귀 stream()에서 다시 Select하면 next가 이미 leased=true라 ErrExhausted가 된다.
		// next.leased를 해제 후 stream() 재귀 호출하면 Select가 next를 재선택한다.
		// 단, pool 내부 leased 상태를 직접 해제하려면 Release가 필요하다.
		if next != nil {
			_ = a.pool.Release(next) // leased=false 복원, 재귀 stream이 다시 Select할 수 있게
		}
		return a.stream(ctx, req, retryCount+1)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return errorStream(fmt.Sprintf("%s: API 응답 %d: %s", a.providerName, resp.StatusCode, string(body)))
	}

	// 9. SSE 스트림 변환 goroutine
	out := make(chan message.StreamEvent, 8)
	go func() {
		ParseAndConvert(ctx, resp.Body, out, a.heartbeatTimeout)
	}()

	return out, nil
}

// Complete는 blocking 방식으로 LLM 응답을 반환한다.
func (a *OpenAIAdapter) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
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
		case message.TypeError:
			if evt.Error != "" {
				return nil, fmt.Errorf("%s: stream error: %s", a.providerName, evt.Error)
			}
		}
	}

	text := textBuilder.String()
	if text != "" {
		resp.Message.Content = []message.ContentBlock{{Type: "text", Text: text}}
	}
	if resp.StopReason == "" {
		resp.StopReason = "stop"
	}
	return resp, nil
}

// doRequest는 OpenAI API에 HTTP 요청을 수행한다.
func (a *OpenAIAdapter) doRequest(ctx context.Context, token string, apiReq openAIRequest) (*http.Response, error) {
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("%s: 요청 직렬화 실패: %w", a.providerName, err)
	}

	url := a.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: HTTP 요청 생성 실패: %w", a.providerName, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: HTTP 요청 실패: %w", a.providerName, err)
	}
	return resp, nil
}

// convertMessages는 message.Message 목록을 OpenAI API 형식으로 변환한다.
func convertMessages(msgs []message.Message) []openAIMsg {
	result := make([]openAIMsg, 0, len(msgs))
	for _, m := range msgs {
		msg := convertSingleMessage(m)
		result = append(result, msg)
	}
	return result
}

// convertSingleMessage는 단일 message.Message를 OpenAI 메시지로 변환한다.
func convertSingleMessage(m message.Message) openAIMsg {
	msg := openAIMsg{Role: m.Role}

	// tool_result 메시지: role="tool"
	if m.ToolUseID != "" {
		msg.Role = "tool"
		msg.ToolCallID = m.ToolUseID
	}

	// 콘텐츠 블록 변환
	var textParts []openAIMsgPart
	var hasImage bool

	for _, block := range m.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, openAIMsgPart{Type: "text", Text: block.Text})
		case "image":
			hasImage = true
			// base64 data URL 형식
			imgURL := fmt.Sprintf("data:%s;base64,%s", block.ImageMediaType, block.Image)
			textParts = append(textParts, openAIMsgPart{
				Type:     "image_url",
				ImageURL: &openAIImage{URL: imgURL},
			})
		case "tool_result":
			// tool_result는 role=tool + content=JSON으로 처리
			msg.Content = block.ToolResultJSON
			return msg
		}
	}

	if hasImage || len(textParts) > 1 {
		msg.Content = textParts
	} else if len(textParts) == 1 && textParts[0].Type == "text" {
		msg.Content = textParts[0].Text
	} else if len(textParts) > 0 {
		msg.Content = textParts
	} else {
		msg.Content = ""
	}

	return msg
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

// Ensure OpenAIAdapter implements provider.Provider at compile time.
var _ provider.Provider = (*OpenAIAdapter)(nil)
