package provider_test

import (
	"context"
	"testing"

	"github.com/modu-ai/goose/internal/llm/cache"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// makeRegistryWithProvider registers a single provider and returns registry + LLMCallFunc.
func makeRegistryWithProvider(t *testing.T, p provider.Provider, logger *zap.Logger) (query.LLMCallFunc, *provider.ProviderRegistry) {
	t.Helper()
	reg := provider.NewRegistry()
	require.NoError(t, reg.Register(p))

	pool := makeTestPool(t, "cred-1")
	fn := provider.NewLLMCall(
		reg,
		pool,
		ratelimit.NewTracker(),
		&cache.BreakpointPlanner{},
		cache.StrategyNone,
		cache.TTLEphemeral,
		nil,
		logger,
	)
	return fn, reg
}

// noJSONModeProvider is a stub provider with JSONMode=false.
type noJSONModeProvider struct{}

func (p *noJSONModeProvider) Name() string { return "no-json-mode" }
func (p *noJSONModeProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Streaming: true, JSONMode: false, UserID: false}
}
func (p *noJSONModeProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return nil, nil
}
func (p *noJSONModeProvider) Stream(_ context.Context, _ provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	ch := make(chan message.StreamEvent, 1)
	close(ch)
	return ch, nil
}

// noUserIDProvider is a stub provider with UserID=false but JSONMode=true.
type noUserIDProvider struct{}

func (p *noUserIDProvider) Name() string { return "no-user-id" }
func (p *noUserIDProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Streaming: true, JSONMode: true, UserID: false}
}
func (p *noUserIDProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return nil, nil
}
func (p *noUserIDProvider) Stream(_ context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	// echo back user_id via a synthetic event so the test can observe it was dropped
	ch := make(chan message.StreamEvent, 2)
	// The request arriving here must have UserID stripped by the gate
	if req.Metadata.UserID != "" {
		ch <- message.StreamEvent{Type: message.TypeError, Error: "user_id not dropped: " + req.Metadata.UserID}
	} else {
		ch <- message.StreamEvent{Type: message.TypeTextDelta, Delta: "ok"}
	}
	ch <- message.StreamEvent{Type: message.TypeMessageStop}
	close(ch)
	return ch, nil
}

// TestNewLLMCall_JSONModeUnsupportedFails verifies that ResponseFormat=="json" with a
// JSONMode=false provider returns ErrCapabilityUnsupported before any HTTP call.
// AC-AMEND-002.
func TestNewLLMCall_JSONModeUnsupportedFails(t *testing.T) {
	t.Parallel()

	logger, _ := zap.NewDevelopment()
	fn, _ := makeRegistryWithProvider(t, &noJSONModeProvider{}, logger)

	req := query.LLMCallReq{
		Route:          router.Route{Model: "test", Provider: "no-json-mode"},
		Messages:       []message.Message{{Role: "user"}},
		ResponseFormat: "json",
	}

	_, err := fn(context.Background(), req)
	require.Error(t, err)

	var capErr provider.ErrCapabilityUnsupported
	require.ErrorAs(t, err, &capErr)
	assert.Equal(t, "json_mode", capErr.Feature)
	assert.Equal(t, "no-json-mode", capErr.ProviderName)
}

// TestNewLLMCall_UserIDSilentDrop verifies that UserID is silently dropped and a DEBUG log
// is emitted when Capabilities.UserID==false, with no error returned.
// AC-AMEND-008 (gate path).
func TestNewLLMCall_UserIDSilentDrop(t *testing.T) {
	t.Parallel()

	// Build observed logger to capture log entries
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	fn, _ := makeRegistryWithProvider(t, &noUserIDProvider{}, logger)

	req := query.LLMCallReq{
		Route:    router.Route{Model: "test", Provider: "no-user-id"},
		Messages: []message.Message{{Role: "user"}},
		Metadata: query.RequestMetadata{UserID: "u-test-drop"},
	}

	ch, err := fn(context.Background(), req)
	require.NoError(t, err, "UserID silent drop must not return an error")

	var events []message.StreamEvent
	for evt := range ch {
		events = append(events, evt)
	}

	// stream should succeed — the provider echoes "ok" when UserID was properly dropped
	textDeltas := llmCallFilterByType(events, message.TypeTextDelta)
	require.Len(t, textDeltas, 1)
	assert.Equal(t, "ok", textDeltas[0].Delta)

	// verify DEBUG log was emitted
	userDropLogs := logs.FilterMessage("user_id_dropped")
	assert.NotEmpty(t, userDropLogs.All(), "DEBUG log 'user_id_dropped' must be emitted")
}

// TestNewLLMCall_RequestImmutability verifies that the caller-owned req is not mutated
// by the capability gate (UserID silent drop path).
// AC-AMEND-011.
func TestNewLLMCall_RequestImmutability(t *testing.T) {
	t.Parallel()

	logger, _ := zap.NewDevelopment()
	fn, _ := makeRegistryWithProvider(t, &noUserIDProvider{}, logger)

	original := query.LLMCallReq{
		Route:          router.Route{Model: "test", Provider: "no-user-id"},
		Messages:       []message.Message{{Role: "user"}},
		ResponseFormat: "json",
		Metadata:       query.RequestMetadata{UserID: "u-immutable-check"},
	}

	// snapshot before call
	beforeUserID := original.Metadata.UserID
	beforeFormat := original.ResponseFormat

	// call — JSONMode=true so no error, UserID will be dropped by gate
	ch, err := fn(context.Background(), original)
	require.NoError(t, err)
	for range ch {
	}

	// caller's req must be unchanged
	assert.Equal(t, beforeUserID, original.Metadata.UserID,
		"Metadata.UserID must not be mutated by capability gate")
	assert.Equal(t, beforeFormat, original.ResponseFormat,
		"ResponseFormat must not be mutated by capability gate")
}

// TestNewLLMCall_Amend_ZapTestLogger is a helper test that verifies zaptest observer works.
func TestNewLLMCall_Amend_ZapObserverBaseline(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	logger.Debug("baseline_check", zap.String("key", "val"))
	assert.Equal(t, 1, logs.Len())
}

// zaptest is used only to import the package; observer is what we actually need.
var _ = zaptest.NewLogger
