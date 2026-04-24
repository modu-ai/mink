// Package google는 Google Gemini API 어댑터를 구현한다.
// google.golang.org/genai SDK를 사용하며, 테스트용 ClientFactory 주입을 지원한다.
// SPEC-GOOSE-ADAPTER-001 M3 T-040
package google

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/zap"
)

// ErrStreamDone은 스트림이 완료되었음을 나타내는 센티넬 에러이다.
var ErrStreamDone = errors.New("stream done")

// GeminiRequest는 Gemini 스트림 요청 파라미터이다.
type GeminiRequest struct {
	Model    string
	Messages []message.Message
}

// GeminiChunk는 Gemini 스트림 청크이다.
type GeminiChunk struct {
	Text     string
	IsDone   bool
	HasTool  bool
	ToolName string
	ToolArgs string
	ToolID   string
}

// GeminiStream은 Gemini 스트림 인터페이스이다.
type GeminiStream interface {
	Next() (*GeminiChunk, error)
	Close()
}

// GeminiClientIface는 Gemini 클라이언트 추상화 인터페이스이다.
// 테스트에서 fake를 주입할 수 있도록 추상화한다. (plan.md Risk R2)
type GeminiClientIface interface {
	GenerateStream(ctx context.Context, req GeminiRequest) (GeminiStream, error)
}

// FakeChunk는 테스트용 fake 청크 데이터이다.
type FakeChunk struct {
	Text     string
	IsDone   bool
	HasTool  bool
	ToolName string
	ToolArgs string
}

// GoogleOptions는 GoogleAdapter 생성 옵션이다.
type GoogleOptions struct {
	// APIKey는 Gemini API 키이다. ClientFactory가 제공되면 무시된다.
	APIKey string
	// Pool은 credential pool이다 (optional, future use).
	// Tracker는 rate limit tracker이다 (optional).
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다 (optional).
	SecretStore provider.SecretStore
	// ClientFactory는 테스트용 fake client 주입 함수이다.
	// nil이면 실제 genai SDK 클라이언트를 생성한다.
	ClientFactory func(apiKey string) GeminiClientIface
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// GoogleAdapter는 Google Gemini API 어댑터이다.
type GoogleAdapter struct {
	client  GeminiClientIface
	tracker *ratelimit.Tracker
	logger  *zap.Logger
}

// New는 GoogleAdapter를 생성한다.
func New(opts GoogleOptions) (*GoogleAdapter, error) {
	var client GeminiClientIface
	if opts.ClientFactory != nil {
		client = opts.ClientFactory(opts.APIKey)
	} else {
		// 실제 genai SDK 클라이언트 — 라이브 API 테스트에서만 사용
		client = newRealGeminiClient(opts.APIKey)
	}

	return &GoogleAdapter{
		client:  client,
		tracker: opts.Tracker,
		logger:  opts.Logger,
	}, nil
}

// Name은 provider 이름을 반환한다.
func (a *GoogleAdapter) Name() string { return "google" }

// Capabilities는 GoogleAdapter의 기능 목록을 반환한다.
func (a *GoogleAdapter) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Streaming:        true,
		Tools:            true,
		Vision:           true,
		Embed:            false,
		AdaptiveThinking: false,
		MaxContextTokens: 1000000,
		MaxOutputTokens:  8192,
	}
}

// Stream은 스트리밍 방식으로 LLM 응답을 반환한다.
// AC-ADAPTER-006: Google Gemini 스트리밍 검증.
func (a *GoogleAdapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	gemReq := GeminiRequest{
		Model:    req.Route.Model,
		Messages: req.Messages,
	}

	if a.logger != nil {
		a.logger.Debug("google stream request",
			zap.String("provider", "google"),
			zap.String("model", req.Route.Model),
			zap.Int("message_count", len(req.Messages)),
		)
	}

	stream, err := a.client.GenerateStream(ctx, gemReq)
	if err != nil {
		return nil, fmt.Errorf("google: generate stream 실패: %w", err)
	}

	out := make(chan message.StreamEvent, 8)
	go func() {
		defer close(out)
		defer stream.Close()
		consumeStream(ctx, stream, out)
	}()

	return out, nil
}

// consumeStream은 GeminiStream을 소비하여 StreamEvent로 변환한다.
func consumeStream(ctx context.Context, stream GeminiStream, out chan<- message.StreamEvent) {
	send := func(evt message.StreamEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- evt:
			return true
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		chunk, err := stream.Next()
		if err != nil {
			if errors.Is(err, ErrStreamDone) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				send(message.StreamEvent{Type: message.TypeMessageStop})
				return
			}
			send(message.StreamEvent{Type: message.TypeError, Error: err.Error()})
			return
		}

		if chunk == nil {
			continue
		}

		if chunk.IsDone {
			send(message.StreamEvent{Type: message.TypeMessageStop})
			return
		}

		// tool call 처리
		if chunk.HasTool {
			toolID := chunk.ToolID
			if toolID == "" {
				toolID = "tool-" + chunk.ToolName
			}
			if !send(message.StreamEvent{
				Type:      message.TypeContentBlockStart,
				BlockType: "tool_use",
				ToolUseID: toolID,
			}) {
				return
			}
			if chunk.ToolArgs != "" {
				if !send(message.StreamEvent{
					Type:  message.TypeInputJSONDelta,
					Delta: chunk.ToolArgs,
				}) {
					return
				}
			}
			if !send(message.StreamEvent{Type: message.TypeContentBlockStop}) {
				return
			}
			continue
		}

		// 텍스트 delta
		if chunk.Text != "" {
			if !send(message.StreamEvent{
				Type:  message.TypeTextDelta,
				Delta: chunk.Text,
			}) {
				return
			}
		}
	}
}

// Complete는 blocking 방식으로 LLM 응답을 반환한다.
func (a *GoogleAdapter) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
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
				return nil, fmt.Errorf("google: stream error: %s", evt.Error)
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

// Ensure GoogleAdapter implements provider.Provider at compile time.
var _ provider.Provider = (*GoogleAdapter)(nil)
