//go:build integration

// Package telegram — integration tests for the full P2 pipeline.
//
// These tests bootstrap the telegram channel with real sqlite store, real
// audit recording, and mock Telegram Bot API / AgentQuery. They verify
// AC-MTGM-002 (audit hash), AC-MTGM-003 (offset survives restart),
// AC-MTGM-004 (partial: first-message gate), AC-MTGM-006 (graceful skip).
package telegram_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- fake Telegram Bot API server ---

type fakeTgServer struct {
	mu       sync.Mutex
	updates  []map[string]interface{}
	sent     []string
	nextID   int
	consumed int32 // atomic: updates consumed count
}

func newFakeTgServer(t *testing.T) (*fakeTgServer, *httptest.Server) {
	f := &fakeTgServer{}
	mux := http.NewServeMux()

	mux.HandleFunc("/bot", func(w http.ResponseWriter, r *http.Request) {
		// Route based on path suffix.
		path := r.URL.Path
		switch {
		case len(path) > 4 && path[len(path)-5:] == "getMe":
			f.handleGetMe(w, r)
		case len(path) > 10 && path[len(path)-10:] == "getUpdates":
			f.handleGetUpdates(w, r)
		case len(path) > 11 && path[len(path)-11:] == "sendMessage":
			f.handleSendMessage(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Register with path prefix to match the bot token in URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Extract method from /bot<token>/method
		for _, method := range []string{"getMe", "getUpdates", "sendMessage"} {
			if len(path) > len(method) && path[len(path)-len(method):] == method {
				switch method {
				case "getMe":
					f.handleGetMe(w, r)
				case "getUpdates":
					f.handleGetUpdates(w, r)
				case "sendMessage":
					f.handleSendMessage(w, r)
				}
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeTgServer) handleGetMe(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]interface{}{
		"ok": true,
		"result": map[string]interface{}{
			"id":         12345,
			"username":   "testbot",
			"is_bot":     true,
			"first_name": "Test",
		},
	}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

func (f *fakeTgServer) handleGetUpdates(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.updates) == 0 {
		// No updates — return empty (non-blocking).
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": []interface{}{}}) //nolint:errcheck
		return
	}

	// Return one update and remove it.
	upd := f.updates[0]
	f.updates = f.updates[1:]
	atomic.AddInt32(&f.consumed, 1)
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": []interface{}{upd}}) //nolint:errcheck
}

func (f *fakeTgServer) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

	f.mu.Lock()
	defer f.mu.Unlock()
	if text, ok := body["text"].(string); ok {
		f.sent = append(f.sent, text)
	}

	f.nextID++
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"ok": true,
		"result": map[string]interface{}{
			"message_id": f.nextID,
			"chat": map[string]interface{}{
				"id": body["chat_id"],
			},
			"text": body["text"],
			"date": time.Now().Unix(),
		},
	})
}

func (f *fakeTgServer) addUpdate(chatID int64, updateID int, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, map[string]interface{}{
		"update_id": updateID,
		"message": map[string]interface{}{
			"message_id": updateID * 10,
			"chat":       map[string]interface{}{"id": chatID},
			"from":       map[string]interface{}{"id": chatID, "first_name": "User"},
			"text":       text,
			"date":       time.Now().Unix(),
		},
	})
}

func (f *fakeTgServer) sentTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.sent))
	copy(cp, f.sent)
	return cp
}

// --- integration tests ---

// TestIntegration_HappyPath_AuditHashAndOffset covers AC-MTGM-002 and AC-MTGM-003
// (partial — offset persistence across restart is tested separately).
func TestIntegration_HappyPath_AuditHashAndOffset(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "telegram.db")

	fakeSrv, httpSrv := newFakeTgServer(t)

	// Add an update for an allowed user (will be auto-admitted).
	fakeSrv.addUpdate(9001, 1, "hello agent")

	client, err := telegram.NewClient("test-token", telegram.WithServerURL(httpSrv.URL))
	require.NoError(t, err)

	store, err := telegram.NewSqliteStore(dbPath)
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck

	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	agent := &fakeAgentQuery{response: "hello back"}

	cfg := &telegram.Config{
		BotUsername:        "testbot",
		Mode:               "polling",
		AutoAdmitFirstUser: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Run until at least one update is consumed + response sent.
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Store:  store,
		Audit:  aw,
		Agent:  agent,
		Logger: zap.NewNop(),
	}

	var runWg sync.WaitGroup
	runWg.Add(1)
	go func() {
		defer runWg.Done()
		_ = telegram.Start(ctx, deps)
	}()

	// Wait for the response to be sent (max 2s).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		texts := fakeSrv.sentTexts()
		if len(texts) > 0 {
			assert.Equal(t, "hello back", texts[0])
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	runWg.Wait()

	// AC-MTGM-002: audit must have >= 2 events with content_hash.
	require.GreaterOrEqual(t, mw.EventCount(), 2, "expected at least inbound + outbound audit events")
	for _, ev := range mw.Events {
		hash, ok := ev.Metadata["content_hash"]
		require.True(t, ok, "every audit event must have content_hash")
		assert.Len(t, hash, 64, "content_hash must be SHA-256 hex")
		assert.NotContains(t, ev.Message, "hello agent", "raw body must not appear in audit")
		assert.NotContains(t, ev.Message, "hello back", "raw body must not appear in audit")
	}

	// AC-MTGM-003 (partial): offset must be > 0 after processing.
	offset, err := store.GetLastOffset(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), offset, "offset must advance to updateID+1=2")
}

// TestIntegration_OffsetSurvivesRestart covers AC-MTGM-003 (persistence across
// Stop → Start restart cycle).
func TestIntegration_OffsetSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "restart.db")

	fakeSrv, httpSrv := newFakeTgServer(t)
	fakeSrv.addUpdate(9002, 5, "first session msg")

	client, err := telegram.NewClient("test-token", telegram.WithServerURL(httpSrv.URL))
	require.NoError(t, err)

	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{response: "ok"}
	cfg := &telegram.Config{BotUsername: "testbot", Mode: "polling", AutoAdmitFirstUser: true}

	// Session 1.
	{
		store, err := telegram.NewSqliteStore(dbPath)
		require.NoError(t, err)

		aw := telegram.NewAuditWrapper(mw, zap.NewNop())
		deps := telegram.Deps{Config: cfg, Client: client, Store: store, Audit: aw, Agent: agent, Logger: zap.NewNop()}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = telegram.Start(ctx, deps)
		}()

		// Wait for the update to be processed.
		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if len(fakeSrv.sentTexts()) > 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		cancel()
		wg.Wait()
		store.Close() //nolint:errcheck
	}

	// Verify offset was persisted.
	{
		store, err := telegram.NewSqliteStore(dbPath)
		require.NoError(t, err)
		defer store.Close() //nolint:errcheck

		offset, err := store.GetLastOffset(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(6), offset, "offset after first session must be updateID+1=6")
	}

	// Session 2: add a new update with higher updateID.
	fakeSrv.addUpdate(9002, 10, "second session msg")

	{
		store, err := telegram.NewSqliteStore(dbPath)
		require.NoError(t, err)

		aw := telegram.NewAuditWrapper(mw, zap.NewNop())
		deps := telegram.Deps{Config: cfg, Client: client, Store: store, Audit: aw, Agent: agent, Logger: zap.NewNop()}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = telegram.Start(ctx, deps)
		}()

		// Wait for second update.
		before := len(fakeSrv.sentTexts())
		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if len(fakeSrv.sentTexts()) > before {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		cancel()
		wg.Wait()
		store.Close() //nolint:errcheck
	}
}

// TestIntegration_FirstMessageGate_AutoAdmitOff covers AC-MTGM-004 (partial).
func TestIntegration_FirstMessageGate_AutoAdmitOff(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "gate.db")

	fakeSrv, httpSrv := newFakeTgServer(t)
	fakeSrv.addUpdate(8001, 1, "first msg")

	client, err := telegram.NewClient("test-token", telegram.WithServerURL(httpSrv.URL))
	require.NoError(t, err)

	store, err := telegram.NewSqliteStore(dbPath)
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck

	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	agent := &fakeAgentQuery{response: "should not be called"}
	cfg := &telegram.Config{BotUsername: "testbot", Mode: "polling", AutoAdmitFirstUser: false}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	deps := telegram.Deps{Config: cfg, Client: client, Store: store, Audit: aw, Agent: agent, Logger: zap.NewNop()}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = telegram.Start(ctx, deps)
	}()

	// Wait for gate notice.
	deadline := time.Now().Add(2 * time.Second)
	gateNoticeSent := false
	for time.Now().Before(deadline) {
		for _, text := range fakeSrv.sentTexts() {
			if len(text) > 0 {
				gateNoticeSent = true
			}
		}
		if gateNoticeSent {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	wg.Wait()

	assert.True(t, gateNoticeSent, "gate notice must be sent to unknown user")

	// User must be stored with Allowed=false.
	m, found, _ := store.GetUserMapping(context.Background(), 8001)
	require.True(t, found)
	assert.False(t, m.Allowed)
}

// TestIntegration_BlockedUser_SilentDrop verifies that a revoked user is
// silently dropped (no SendMessage).
func TestIntegration_BlockedUser_SilentDrop(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "blocked.db")

	fakeSrv, httpSrv := newFakeTgServer(t)

	// Pre-register a blocked user.
	store, err := telegram.NewSqliteStore(dbPath)
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, store.PutUserMapping(context.Background(), telegram.UserMapping{
		ChatID: 7777, UserProfileID: fmt.Sprintf("tg-%d", 7777), Allowed: false, FirstSeenAt: now, LastSeenAt: now,
	}))

	fakeSrv.addUpdate(7777, 1, "blocked message")

	client, err := telegram.NewClient("test-token", telegram.WithServerURL(httpSrv.URL))
	require.NoError(t, err)

	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	agent := &fakeAgentQuery{response: "should not be called"}
	cfg := &telegram.Config{BotUsername: "testbot", Mode: "polling", AutoAdmitFirstUser: false}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	deps := telegram.Deps{Config: cfg, Client: client, Store: store, Audit: aw, Agent: agent, Logger: zap.NewNop()}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = telegram.Start(ctx, deps)
	}()

	// Wait enough for the update to be processed.
	time.Sleep(400 * time.Millisecond)
	cancel()
	wg.Wait()
	store.Close() //nolint:errcheck

	// No messages should have been sent.
	assert.Empty(t, fakeSrv.sentTexts(), "blocked user should not receive any message")

	// Audit event should have dropped_blocked flag.
	found := false
	for _, ev := range mw.Events {
		if v, ok := ev.Metadata["dropped_blocked"]; ok && v == "true" {
			found = true
		}
	}
	assert.True(t, found, "audit should record dropped_blocked=true")
}

// TestIntegration_GracefulSkip_NilDeps covers AC-MTGM-006: Start with partial
// deps falls back to EchoHandler without panicking.
func TestIntegration_GracefulSkip_NilDeps(t *testing.T) {
	_, httpSrv := newFakeTgServer(t)

	client, err := telegram.NewClient("test-token", telegram.WithServerURL(httpSrv.URL))
	require.NoError(t, err)

	cfg := &telegram.Config{BotUsername: "testbot", Mode: "polling"}
	// Store/Audit/Agent intentionally nil → EchoHandler fallback.
	deps := telegram.Deps{Config: cfg, Client: client, Logger: zap.NewNop()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately — just verify no panic

	err = telegram.Start(ctx, deps)
	assert.ErrorIs(t, err, context.Canceled)
}
