// Package ollama는 Ollama 로컬 LLM 어댑터를 구현한다.
// Credential 없이 /api/chat endpoint에 JSON-L 스트리밍 방식으로 통신한다.
// SPEC-GOOSE-ADAPTER-001 M4 T-050
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/zap"
)

const (
	// defaultEndpoint는 Ollama API 기본 엔드포인트이다.
	defaultEndpoint = "http://localhost:11434"
	// requestTimeout은 non-streaming 요청 타임아웃이다.
	requestTimeout = 30 * time.Second
)

// OllamaOptions는 OllamaAdapter 생성 옵션이다.
type OllamaOptions struct {
	// Endpoint는 Ollama API 엔드포인트이다. 빈 값이면 defaultEndpoint 사용.
	Endpoint string
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다.
	HTTPClient *http.Client
	// Tracker는 rate limit tracker이다 (optional).
	Tracker *ratelimit.Tracker
	// HeartbeatTimeout은 streaming heartbeat 타임아웃이다 (REQ-ADAPTER-013).
	// zero value이면 provider.DefaultStreamHeartbeatTimeout(60s)를 사용한다.
	HeartbeatTimeout time.Duration
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// OllamaAdapter는 Ollama 로컬 LLM 어댑터이다.
// Credential이 없으며 로컬 endpoint와 직접 통신한다.
type OllamaAdapter struct {
	endpoint         string
	httpClient       *http.Client
	tracker          *ratelimit.Tracker
	heartbeatTimeout time.Duration
	logger           *zap.Logger
}

// New는 OllamaAdapter를 생성한다.
func New(opts OllamaOptions) (*OllamaAdapter, error) {
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}

	hbTimeout := opts.HeartbeatTimeout
	if hbTimeout <= 0 {
		hbTimeout = provider.DefaultStreamHeartbeatTimeout
	}

	return &OllamaAdapter{
		endpoint:         strings.TrimRight(endpoint, "/"),
		httpClient:       httpClient,
		tracker:          opts.Tracker,
		heartbeatTimeout: hbTimeout,
		logger:           opts.Logger,
	}, nil
}

// Name은 provider 이름을 반환한다.
func (a *OllamaAdapter) Name() string { return "ollama" }

// Capabilities는 OllamaAdapter의 기능 목록을 반환한다.
func (a *OllamaAdapter) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Streaming:        true,
		Tools:            true,
		Vision:           true,
		Embed:            false,
		AdaptiveThinking: false,
	}
}

// ollamaRequest는 Ollama /api/chat 요청 바디이다.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMsg     `json:"messages"`
	Tools    []ollamaToolDef `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

// ollamaMsg는 Ollama 메시지 형식이다.
type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaToolDef는 Ollama tool 정의이다.
type ollamaToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters,omitempty"`
	} `json:"function"`
}

// ollamaStreamLine은 Ollama JSON-L 스트림 라인 구조이다.
type ollamaStreamLine struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls"`
	} `json:"message"`
	Done bool `json:"done"`
}

// ollamaToolCall는 Ollama tool_call 구조이다.
type ollamaToolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// Stream은 스트리밍 방식으로 LLM 응답을 반환한다.
// AC-ADAPTER-007: Ollama localhost 스트리밍 검증.
func (a *OllamaAdapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	// 메시지 변환
	msgs := convertMessages(req.Messages)

	// tools 변환
	var tools []ollamaToolDef
	for _, t := range req.Tools {
		td := ollamaToolDef{Type: "function"}
		td.Function.Name = t.Name
		td.Function.Description = t.Description
		td.Function.Parameters = t.Parameters
		tools = append(tools, td)
	}

	apiReq := ollamaRequest{
		Model:    req.Route.Model,
		Messages: msgs,
		Tools:    tools,
		Stream:   true,
	}

	if a.logger != nil {
		a.logger.Debug("ollama stream request",
			zap.String("provider", "ollama"),
			zap.String("model", req.Route.Model),
			zap.Int("message_count", len(req.Messages)),
		)
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: 요청 직렬화 실패: %w", err)
	}

	url := a.endpoint + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: HTTP 요청 생성 실패: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: HTTP 요청 실패: %w", err)
	}

	// rate limit 헤더 파싱 (REQ-ADAPTER-004). Ollama는 표준 rate-limit 헤더를 제공하지
	// 않으나 tracker.Parse는 noop이므로 conformance 목적으로 호출한다.
	if a.tracker != nil {
		a.tracker.Parse("ollama", resp.Header, time.Now())
	}

	if resp.StatusCode != http.StatusOK {
		rbody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama: API 응답 %d: %s", resp.StatusCode, string(rbody))
	}

	out := make(chan message.StreamEvent, 8)
	go func() {
		defer close(out)
		parseJSONL(ctx, resp.Body, out, a.heartbeatTimeout, a.logger)
	}()

	return out, nil
}

// ollamaLine은 reader goroutine이 반환하는 라인 또는 에러이다.
type ollamaLine struct {
	line string
	err  error
}

// parseJSONL는 Ollama JSON-L 스트림을 파싱하여 StreamEvent로 변환한다.
// ctx 취소 또는 hbTimeout 초과 시 즉시 종료한다.
//
// @MX:WARN: [AUTO] reader goroutine + reslide-timer watchdog — goroutine 누수 위험
// @MX:REASON: readerLoop goroutine은 body.Close() 호출로 정리된다.
//
//	hbTimeout 타임아웃 경로에서도 호출 측 defer body.Close()가 반드시 실행되어야 한다.
func parseJSONL(ctx context.Context, body io.ReadCloser, out chan<- message.StreamEvent, hbTimeout time.Duration, logger *zap.Logger) {
	defer body.Close()

	// reader goroutine
	lineCh := make(chan ollamaLine, 4)
	go func() {
		defer close(lineCh)
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			lineCh <- ollamaLine{line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			lineCh <- ollamaLine{err: err}
		}
	}()

	hb := time.NewTimer(hbTimeout)
	defer hb.Stop()

	send := func(evt message.StreamEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- evt:
			return true
		}
	}

	resetHB := func() {
		if !hb.Stop() {
			select {
			case <-hb.C:
			default:
			}
		}
		hb.Reset(hbTimeout)
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-hb.C:
			send(message.StreamEvent{
				Type:  message.TypeError,
				Error: fmt.Sprintf("ollama: heartbeat timeout: no data for %s", hbTimeout),
			})
			return

		case r, ok := <-lineCh:
			if !ok {
				return
			}
			resetHB()

			if r.err != nil {
				send(message.StreamEvent{Type: message.TypeError, Error: r.err.Error()})
				return
			}

			line := r.line
			if line == "" {
				continue
			}

			var chunk ollamaStreamLine
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				if logger != nil {
					logger.Debug("ollama JSON-L 파싱 실패", zap.Error(err))
				}
				continue
			}

			// tool_calls 처리
			if len(chunk.Message.ToolCalls) > 0 {
				for _, tc := range chunk.Message.ToolCalls {
					toolID := "tool-" + tc.Function.Name
					if !send(message.StreamEvent{
						Type:      message.TypeContentBlockStart,
						BlockType: "tool_use",
						ToolUseID: toolID,
					}) {
						return
					}
					if len(tc.Function.Arguments) > 0 {
						if !send(message.StreamEvent{
							Type:  message.TypeInputJSONDelta,
							Delta: string(tc.Function.Arguments),
						}) {
							return
						}
					}
					if !send(message.StreamEvent{Type: message.TypeContentBlockStop}) {
						return
					}
				}
			} else if chunk.Message.Content != "" {
				// 텍스트 delta
				if !send(message.StreamEvent{
					Type:  message.TypeTextDelta,
					Delta: chunk.Message.Content,
				}) {
					return
				}
			}

			// done=true 시 message_stop
			if chunk.Done {
				send(message.StreamEvent{Type: message.TypeMessageStop})
				return
			}
		}
	}
}

// Complete는 blocking 방식으로 LLM 응답을 반환한다.
func (a *OllamaAdapter) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
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
				return nil, fmt.Errorf("ollama: stream error: %s", evt.Error)
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

// convertMessages는 message.Message 목록을 Ollama 형식으로 변환한다.
func convertMessages(msgs []message.Message) []ollamaMsg {
	result := make([]ollamaMsg, 0, len(msgs))
	for _, m := range msgs {
		var content strings.Builder
		for _, block := range m.Content {
			if block.Type == "text" {
				content.WriteString(block.Text)
			} else if block.Type == "tool_result" {
				content.WriteString(block.ToolResultJSON)
			}
		}
		role := m.Role
		if m.ToolUseID != "" {
			role = "tool"
		}
		result = append(result, ollamaMsg{
			Role:    role,
			Content: content.String(),
		})
	}
	return result
}

// Ensure OllamaAdapter implements provider.Provider at compile time.
var _ provider.Provider = (*OllamaAdapter)(nil)
