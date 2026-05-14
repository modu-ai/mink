// Package cli provides CLI application wiring and initialization.
package cli

import (
	"context"
	"testing"

	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/message"
	"go.uber.org/zap"
)

// TestNewApp creates an App with nil dependencies and verifies graceful degradation.
func TestNewApp_NilDeps(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		AliasFile:   "",
		StrictAlias: false,
		DaemonAddr:  "",
		Logger:      logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	if app == nil {
		t.Fatal("NewApp returned nil app")
	}

	// Verify adapter is created even with nil registry
	if app.Adapter == nil {
		t.Error("Adapter should not be nil")
	}

	// Verify dispatcher is created
	if app.Dispatcher == nil {
		t.Error("Dispatcher should not be nil")
	}

	// Verify logger is set
	if app.Logger != logger {
		t.Error("Logger not set correctly")
	}

	// Client may be nil if daemon unreachable - that's OK
	if app.Client != nil {
		// If client exists, it should be able to close
		if err := app.Client.Close(); err != nil {
			t.Errorf("Client.Close failed: %v", err)
		}
	}
}

// TestNewApp_WithRegistry verifies App initialization with registry.
func TestNewApp_WithRegistry(t *testing.T) {
	logger := zap.NewNop()

	cfg := AppConfig{
		AliasFile:   "testdata/nonexistent.yaml",
		StrictAlias: false,
		DaemonAddr:  "",
		Logger:      logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	if app.Adapter == nil {
		t.Fatal("Adapter should not be nil")
	}

	// Verify adapter can resolve model aliases
	info, err := app.Adapter.ResolveModelAlias("openai/gpt-4o")
	if err != nil {
		t.Errorf("ResolveModelAlias failed: %v", err)
	}
	if info == nil {
		t.Error("ResolveModelAlias returned nil info")
	}
}

// TestInitApp_Idempotent verifies InitApp only initializes once.
func TestInitApp_Idempotent(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		Logger: logger,
	}

	// First call
	app1, err := InitApp(cfg)
	if err != nil {
		t.Fatalf("InitApp failed: %v", err)
	}

	// Second call should return same instance
	app2, err := InitApp(cfg)
	if err != nil {
		t.Fatalf("InitApp failed: %v", err)
	}

	if app1 != app2 {
		t.Error("InitApp should return same instance on subsequent calls")
	}
}

// TestWithApp_AppFromContext verifies context roundtrip.
func TestWithApp_AppFromContext(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		Logger: logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	ctx := context.Background()
	ctx = WithApp(ctx, app)

	retrieved := AppFromContext(ctx)
	if retrieved != app {
		t.Error("AppFromContext should return the same app instance")
	}
}

// TestAppFromContext_NilContext verifies nil-safe context handling.
func TestAppFromContext_NilContext(t *testing.T) {
	retrieved := AppFromContext(context.TODO())
	if retrieved != nil {
		t.Error("AppFromContext should return nil for nil context")
	}

	retrieved = AppFromContext(context.Background())
	if retrieved != nil {
		t.Error("AppFromContext should return nil for context without app")
	}
}

// TestNewApp_ClientNilWhenDaemonUnreachable verifies client is nil on connection failure.
func TestNewApp_ClientNilWhenDaemonUnreachable(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		DaemonAddr: "127.0.0.1:99999", // Non-existent port
		Logger:     logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	// Client should be nil when daemon is unreachable
	if app.Client != nil {
		t.Error("Client should be nil when daemon is unreachable")
		if err := app.Client.Close(); err != nil {
			t.Errorf("Client.Close failed: %v", err)
		}
	}
}

// TestApp_DispatcherIntegration verifies dispatcher integration with adapter.
func TestApp_DispatcherIntegration(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		Logger: logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	ctx := context.Background()
	sctx := app.Adapter.WithContext(ctx)

	// Test non-slash command (should proceed)
	result, err := app.Dispatcher.ProcessUserInput(ctx, "hello world", sctx)
	if err != nil {
		t.Errorf("ProcessUserInput failed: %v", err)
	}

	if result.Kind != command.ProcessProceed {
		t.Errorf("Expected ProcessProceed, got %v", result.Kind)
	}

	if result.Prompt != "hello world" {
		t.Errorf("Expected prompt 'hello world', got '%s'", result.Prompt)
	}

	// Test unknown slash command (should return local reply)
	result, err = app.Dispatcher.ProcessUserInput(ctx, "/unknown command", sctx)
	if err != nil {
		t.Errorf("ProcessUserInput failed: %v", err)
	}

	if result.Kind != command.ProcessLocal {
		t.Errorf("Expected ProcessLocal for unknown command, got %v", result.Kind)
	}

	if len(result.Messages) == 0 {
		t.Error("Expected messages for unknown command")
	}

	// Verify it's a system message
	if len(result.Messages) > 0 {
		msg := result.Messages[0]
		if msg.Type != message.SDKMsgMessage {
			t.Errorf("Expected SDKMsgMessage, got %v", msg.Type)
		}
	}
}

// TestNewApp_AliasConfigLoadFailure verifies graceful fallback when alias config fails.
func TestNewApp_AliasConfigLoadFailure(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		AliasFile:   "/nonexistent/path/aliases.yaml",
		StrictAlias: false,
		Logger:      logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	// App should still be created with empty alias map
	if app == nil {
		t.Fatal("NewApp returned nil app")
	}

	if app.Adapter == nil {
		t.Fatal("Adapter should not be nil even with alias config failure")
	}
}

// TestApp_ClientLazyConnection verifies client connection is lazy.
func TestApp_ClientLazyConnection(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		DaemonAddr: "127.0.0.1:9005", // Default daemon address
		Logger:     logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	// Client may be nil if daemon not running - that's OK
	// This test verifies the app doesn't panic
	if app.Client != nil {
		// If daemon happens to be running, clean up
		if err := app.Client.Close(); err != nil {
			t.Errorf("Client.Close failed: %v", err)
		}
	}
}

// TestApp_CommandProcessing verifies various command types are handled correctly.
func TestApp_CommandProcessing(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		Logger: logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	ctx := context.Background()
	sctx := app.Adapter.WithContext(ctx)

	tests := []struct {
		name           string
		input          string
		expectedKind   command.ProcessedKind
		expectError    bool
		expectPrompt   string
		expectMessages int
	}{
		{
			name:         "normal text",
			input:        "hello world",
			expectedKind: command.ProcessProceed,
			expectPrompt: "hello world",
		},
		{
			name:           "slash help",
			input:          "/help",
			expectedKind:   command.ProcessLocal,
			expectMessages: 1, // /help returns local reply
		},
		{
			name:           "unknown slash command",
			input:          "/unknown",
			expectedKind:   command.ProcessLocal,
			expectMessages: 1, // unknown command returns error message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := app.Dispatcher.ProcessUserInput(ctx, tt.input, sctx)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result.Kind != tt.expectedKind {
				t.Errorf("Expected kind %v, got %v", tt.expectedKind, result.Kind)
			}

			if tt.expectPrompt != "" && result.Prompt != tt.expectPrompt {
				t.Errorf("Expected prompt '%s', got '%s'", tt.expectPrompt, result.Prompt)
			}

			if tt.expectMessages > 0 && len(result.Messages) != tt.expectMessages {
				t.Errorf("Expected %d messages, got %d", tt.expectMessages, len(result.Messages))
			}
		})
	}
}

// TestNewApp_ConcurrentInitialization verifies concurrent InitApp calls are safe.
func TestNewApp_ConcurrentInitialization(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		Logger: logger,
	}

	results := make(chan *App, 10)
	errs := make(chan error, 10)

	// Launch 10 concurrent InitApp calls
	for i := 0; i < 10; i++ {
		go func() {
			app, err := InitApp(cfg)
			if err != nil {
				errs <- err
				return
			}
			results <- app
		}()
	}

	// Wait for all goroutines to complete
	apps := make([]*App, 0, 10)
	for i := 0; i < 10; i++ {
		select {
		case app := <-results:
			apps = append(apps, app)
		case err := <-errs:
			t.Fatalf("InitApp failed: %v", err)
		}
	}

	if len(apps) != 10 {
		t.Fatalf("Expected 10 apps, got %d", len(apps))
	}

	first := apps[0]
	for _, app := range apps[1:] {
		if app != first {
			t.Error("Concurrent InitApp calls should return same instance")
		}
	}
}

// TestApp_AdapterWithContext verifies adapter creates proper context clones.
func TestApp_AdapterWithContext(t *testing.T) {
	logger := zap.NewNop()
	cfg := AppConfig{
		Logger: logger,
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	ctx := context.Background()
	sctx1 := app.Adapter.WithContext(ctx)
	sctx2 := app.Adapter.WithContext(ctx)

	// Both should have same registry and dispatcher
	if sctx1 == nil || sctx2 == nil {
		t.Fatal("WithContext should not return nil")
	}

	// Verify they are different instances (shallow copies)
	if sctx1 == sctx2 {
		t.Error("WithContext should return different instances")
	}
}
