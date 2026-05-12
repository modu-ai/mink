// Package google는 Google Gemini API 어댑터를 구현한다.
// google.golang.org/genai SDK를 사용하며, 테스트용 ClientFactory 주입을 지원한다.
// SPEC-GOOSE-ADAPTER-001 M3 T-040
package google

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"github.com/modu-ai/mink/internal/message"
	"go.uber.org/zap"
)

// ErrStreamDone은 스트림이 완료되었음을 나타내는 센티넬 에러이다.
var ErrStreamDone = errors.New("stream done")

// GeminiRequest는 Gemini 스트림 요청 파라미터이다.
type GeminiRequest struct {
	Model    string
	Messages []message.Message
	// ResponseFormat carries the requested output format ("json" or "").
	// When set to "json", the real client injects ResponseMIMEType="application/json".
	// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-007
	ResponseFormat string
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
	// Pool은 credential pool이다 (REQ-ADAPTER-005). ClientFactory가 nil일 때 필수.
	Pool *credential.CredentialPool
	// SecretStore는 secret 저장소이다 (REQ-ADAPTER-005). ClientFactory가 nil일 때 필수.
	SecretStore provider.SecretStore
	// Tracker는 rate limit tracker이다 (optional, REQ-ADAPTER-004).
	Tracker *ratelimit.Tracker
	// ClientFactory는 테스트용 fake client 주입 함수이다.
	// nil이면 실제 genai SDK 클라이언트를 Pool+SecretStore로 API key를 해결하여 생성한다.
	ClientFactory func(apiKey string) GeminiClientIface
	// HeartbeatTimeout은 streaming heartbeat 타임아웃이다 (REQ-ADAPTER-013).
	// zero value이면 provider.DefaultStreamHeartbeatTimeout(60s)를 사용한다.
	HeartbeatTimeout time.Duration
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// GoogleAdapter는 Google Gemini API 어댑터이다.
//
// @MX:NOTE: [AUTO] Gemini는 genai SDK를 통해 스트리밍하므로 HTTP 헤더에 직접 접근 불가.
// tracker.Parse는 빈 헤더로 호출되며 RATELIMIT-001 구현 시 실질적 파싱이 추가된다.
type GoogleAdapter struct {
	pool             *credential.CredentialPool
	secretStore      provider.SecretStore
	clientFactory    func(apiKey string) GeminiClientIface
	tracker          *ratelimit.Tracker
	heartbeatTimeout time.Duration
	logger           *zap.Logger
}

// New는 GoogleAdapter를 생성한다.
// ClientFactory가 nil이면 Pool과 SecretStore가 필수이다 (REQ-ADAPTER-005).
func New(opts GoogleOptions) (*GoogleAdapter, error) {
	if opts.ClientFactory == nil && opts.Pool == nil {
		return nil, fmt.Errorf("google: Pool is required when ClientFactory is nil")
	}
	if opts.ClientFactory == nil && opts.SecretStore == nil {
		return nil, fmt.Errorf("google: SecretStore is required when ClientFactory is nil")
	}

	hbTimeout := opts.HeartbeatTimeout
	if hbTimeout <= 0 {
		hbTimeout = provider.DefaultStreamHeartbeatTimeout
	}

	return &GoogleAdapter{
		pool:             opts.Pool,
		secretStore:      opts.SecretStore,
		clientFactory:    opts.ClientFactory,
		tracker:          opts.Tracker,
		heartbeatTimeout: hbTimeout,
		logger:           opts.Logger,
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
		// JSONMode is supported via generationConfig.responseMimeType (REQ-AMEND-002).
		// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-002
		JSONMode: true,
		// UserID is unsupported — GenerateContentRequest schema has no user identifier field.
		// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-002
		UserID: false,
	}
}

// resolveClient는 요청에 사용할 GeminiClientIface를 반환한다.
// ClientFactory가 주입된 경우(테스트) 그대로 반환한다.
// 그렇지 않으면 Pool+SecretStore로 API key를 해결하여 실제 클라이언트를 생성한다 (REQ-ADAPTER-005).
func (a *GoogleAdapter) resolveClient(ctx context.Context) (GeminiClientIface, *credential.PooledCredential, error) {
	if a.clientFactory != nil {
		// 테스트 경로: fake client를 직접 반환
		return a.clientFactory(""), nil, nil
	}

	// 프로덕션 경로: CredentialPool에서 credential 선택 (REQ-ADAPTER-005)
	cred, err := a.pool.Select(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("google: credential 선택 실패: %w", err)
	}

	// SecretStore에서 실제 API key 조회
	apiKey, err := a.secretStore.Resolve(ctx, cred.KeyringID)
	if err != nil {
		return nil, cred, fmt.Errorf("google: API key 해결 실패: %w", err)
	}

	return newRealGeminiClient(apiKey), cred, nil
}

// Stream은 스트리밍 방식으로 LLM 응답을 반환한다.
// AC-ADAPTER-006: Google Gemini 스트리밍 검증.
func (a *GoogleAdapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	// credential 해결 (REQ-ADAPTER-005)
	client, cred, err := a.resolveClient(ctx)
	if err != nil {
		return nil, err
	}

	// rate limit 헤더 파싱 (REQ-ADAPTER-004).
	// genai SDK는 raw HTTP 헤더를 노출하지 않으므로 빈 헤더로 호출한다.
	// RATELIMIT-001 구현 시 SDK 메타데이터에서 limit 정보를 추출할 예정이다.
	if a.tracker != nil {
		a.tracker.ParseHTTPHeader("google", http.Header{}, time.Now())
	}

	// 429 응답 시 credential rotation (REQ-ADAPTER-005).
	// genai SDK는 에러 타입으로 HTTP 상태를 노출하지 않으므로 현재는 단순 에러 반환.
	// 향후 SDK 에러 타입 검사로 개선 예정.
	_ = cred // 프로덕션 경로에서만 non-nil; SDK 기반이므로 직접 rotation은 별도 구현 필요

	gemReq := GeminiRequest{
		Model:          req.Route.Model,
		Messages:       req.Messages,
		ResponseFormat: req.ResponseFormat,
	}

	if a.logger != nil {
		a.logger.Debug("google stream request",
			zap.String("provider", "google"),
			zap.String("model", req.Route.Model),
			zap.Int("message_count", len(req.Messages)),
		)
	}

	stream, err := client.GenerateStream(ctx, gemReq)
	if err != nil {
		return nil, fmt.Errorf("google: generate stream 실패: %w", err)
	}

	out := make(chan message.StreamEvent, 8)
	go func() {
		defer close(out)
		defer stream.Close()
		consumeStream(ctx, stream, out, a.heartbeatTimeout)
	}()

	return out, nil
}

// geminiResult는 reader goroutine이 반환하는 청크 또는 에러이다.
type geminiResult struct {
	chunk *GeminiChunk
	err   error
}

// consumeStream은 GeminiStream을 소비하여 StreamEvent로 변환한다.
// ctx 취소 또는 hbTimeout 초과 시 즉시 종료한다.
//
// @MX:WARN: [AUTO] reader goroutine + reslide-timer watchdog — goroutine 누수 위험
// @MX:REASON: readerLoop goroutine은 stream.Close() 호출(ctx 취소 또는 hbTimeout)로 정리된다.
//
//	stream.Close()는 호출 측의 defer stream.Close()에서 반드시 실행된다.
func consumeStream(ctx context.Context, stream GeminiStream, out chan<- message.StreamEvent, hbTimeout time.Duration) {
	// reader goroutine: stream.Next()를 호출하여 chunkCh에 전달한다.
	// stream.Close() 호출 시 Next()가 에러를 반환하여 goroutine이 종료된다.
	chunkCh := make(chan geminiResult, 4)
	go func() {
		defer close(chunkCh)
		for {
			chunk, err := stream.Next()
			chunkCh <- geminiResult{chunk: chunk, err: err}
			if err != nil {
				return
			}
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
				Error: fmt.Sprintf("google: heartbeat timeout: no data for %s", hbTimeout),
			})
			return

		case r, ok := <-chunkCh:
			if !ok {
				return
			}
			resetHB()

			if r.err != nil {
				if errors.Is(r.err, ErrStreamDone) || errors.Is(r.err, context.Canceled) || errors.Is(r.err, context.DeadlineExceeded) {
					send(message.StreamEvent{Type: message.TypeMessageStop})
					return
				}
				send(message.StreamEvent{Type: message.TypeError, Error: r.err.Error()})
				return
			}

			chunk := r.chunk
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
