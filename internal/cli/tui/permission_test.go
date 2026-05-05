// Package tui provides permission modal integration tests.
package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/goose/internal/cli/tui/permission"
	"github.com/modu-ai/goose/internal/cli/tui/snapshots"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPermClient is a mockDaemonClient that also implements ResolvePermission.
type mockPermClient struct {
	chatStreamFunc      func(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error)
	resolvePermCalled   bool
	resolvePermToolName string
	resolvePermDecision string
}

func (m *mockPermClient) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error) {
	if m.chatStreamFunc != nil {
		return m.chatStreamFunc(ctx, messages)
	}
	ch := make(chan StreamEvent, 1)
	close(ch)
	return ch, nil
}

func (m *mockPermClient) Close() error { return nil }

func (m *mockPermClient) ResolvePermission(_ context.Context, _, toolName, decision string) (bool, error) {
	m.resolvePermCalled = true
	m.resolvePermToolName = toolName
	m.resolvePermDecision = decision
	return true, nil
}

// TestPermission_Modal_OpensOnRequest verifies AC-CLITUI-003:
// mock client injects permission_request payload → permissionState.active=true + ToolName="Bash".
func TestPermission_Modal_OpensOnRequest(t *testing.T) {
	snapshots.SetupAsciiTermenv()

	model := NewModel(nil, "test-session", true /* noColor */)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Inject permission_request event.
	msg := StreamEventMsg{
		Event: StreamEvent{
			Type:    "permission_request",
			Content: `{"tool_name":"Bash","tool_use_id":"tu-001"}`,
		},
	}

	newModel, _ := model.Update(msg)
	m := newModel.(*Model)

	assert.True(t, m.permissionState.Active, "permissionState.active must be true")
	assert.Equal(t, "Bash", m.permissionState.ToolName)
	assert.Equal(t, "tu-001", m.permissionState.ToolUseID)

	// Snapshot: modal overlay in view.
	view := m.View()
	assert.NotEmpty(t, view)
	snapshots.WriteGolden(t, "permission_modal_open", []byte(view))
	snapshots.RequireSnapshot(t, "permission_modal_open", []byte(view))
}

// TestPermission_AllowAlways_PersistsToDisk verifies AC-CLITUI-004:
// modal open Bash + permissions.json absent, "Allow always" + Enter →
// permissions.json created with {"version":1,"tools":{"Bash":"allow"}} +
// ResolvePermission RPC called.
func TestPermission_AllowAlways_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	permPath := filepath.Join(dir, "permissions.json")

	// Override HOME so permStore uses our temp dir.
	t.Setenv("HOME", dir)

	client := &mockPermClient{}
	model := NewModel(client, "test-session", true)
	model.width = 80
	model.height = 24
	// Wire the store to our tmpdir path.
	model.permStore = permission.NewStore(permPath)

	// Step 1: inject permission_request.
	permMsg := StreamEventMsg{
		Event: StreamEvent{
			Type:    "permission_request",
			Content: `{"tool_name":"Bash","tool_use_id":"tu-allow"}`,
		},
	}
	m1, _ := model.Update(permMsg)
	m := m1.(*Model)
	require.True(t, m.permissionState.Active)

	// Step 2: move cursor to "Allow always" (index 1).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(*Model)

	// Step 3: press Enter to confirm.
	m3, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(*Model)
	assert.False(t, m.permissionState.Active, "modal should close after Enter")

	// Execute the returned command chain:
	// Enter produces ResolveMsg cmd → Update processes ResolveMsg → returns resolveCmd.
	// We need to fully drain the command chain.
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		var nextCmd tea.Cmd
		m4, nextCmd := m.Update(msg)
		m = m4.(*Model)
		cmd = nextCmd
	}

	// Assert: permissions.json was created.
	data, err := os.ReadFile(permPath)
	require.NoError(t, err, "permissions.json must be created")

	assert.Contains(t, string(data), `"version":1`)
	assert.Contains(t, string(data), `"Bash"`)
	assert.Contains(t, string(data), `"allow"`)

	// Assert: ResolvePermission RPC was called.
	assert.True(t, client.resolvePermCalled, "ResolvePermission RPC must be called")
	assert.Equal(t, "Bash", client.resolvePermToolName)
}

// TestPermission_AllowOnce_DoesNotPersist verifies AC-CLITUI-005:
// modal open FileWrite + permissions.json absent, default Enter (Allow once) →
// permissions.json NOT created + ResolvePermission called +
// in-memory NOT recorded (next call shows modal again).
func TestPermission_AllowOnce_DoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	permPath := filepath.Join(dir, "permissions.json")

	t.Setenv("HOME", dir)

	client := &mockPermClient{}
	model := NewModel(client, "test-session", true)
	model.width = 80
	model.height = 24
	model.permStore = permission.NewStore(permPath)

	// Step 1: inject permission_request for FileWrite.
	permMsg := StreamEventMsg{
		Event: StreamEvent{
			Type:    "permission_request",
			Content: `{"tool_name":"FileWrite","tool_use_id":"tu-once"}`,
		},
	}
	m1, _ := model.Update(permMsg)
	m := m1.(*Model)
	require.True(t, m.permissionState.Active)

	// Step 2: press Enter with default cursor (0 = Allow once).
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(*Model)
	assert.False(t, m.permissionState.Active, "modal must close")

	// Drain command chain.
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		m3, nextCmd := m.Update(msg)
		m = m3.(*Model)
		cmd = nextCmd
	}

	// Assert: permissions.json NOT created.
	_, err := os.Stat(permPath)
	assert.True(t, os.IsNotExist(err), "permissions.json must NOT be created for allow_once")

	// Assert: ResolvePermission RPC was called.
	assert.True(t, client.resolvePermCalled, "ResolvePermission RPC must be called even for allow_once")

	// Assert: in-memory has NO persisted allow for FileWrite.
	_, hasAllow := m.permStore.Has("FileWrite")
	assert.False(t, hasAllow, "FileWrite must not be in persistent store for allow_once")

	// Assert: next permission_request still shows modal.
	permMsg2 := StreamEventMsg{
		Event: StreamEvent{
			Type:    "permission_request",
			Content: `{"tool_name":"FileWrite","tool_use_id":"tu-once-2"}`,
		},
	}
	m4, _ := m.Update(permMsg2)
	m = m4.(*Model)
	assert.True(t, m.permissionState.Active, "modal must reopen for allow_once tool")
}

// TestPermission_StreamPaused_WhileModalOpen verifies AC-CLITUI-006:
// 5 chunk stream + permission_request before 3rd chunk, modal + Allow once →
// during modal viewport unchanged + after close chunks 4/5 arrive in order + 0 missing chunks.
func TestPermission_StreamPaused_WhileModalOpen(t *testing.T) {
	model := NewModel(nil, "test-session", true)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Send chunks 1 and 2.
	chunk1 := StreamEventMsg{Event: StreamEvent{Type: "text", Content: "chunk1"}}
	chunk2 := StreamEventMsg{Event: StreamEvent{Type: "text", Content: "chunk2"}}

	m1, _ := model.Update(chunk1)
	m := m1.(*Model)
	m2, _ := m.Update(chunk2)
	m = m2.(*Model)

	// Inject permission_request (before chunk 3).
	permMsg := StreamEventMsg{
		Event: StreamEvent{
			Type:    "permission_request",
			Content: `{"tool_name":"Bash","tool_use_id":"tu-stream"}`,
		},
	}
	m3, _ := m.Update(permMsg)
	m = m3.(*Model)
	require.True(t, m.permissionState.Active, "modal must be active")

	// Capture viewport content while modal is active — must not change.
	viewportBeforeModal := m.viewport.View()

	// Send chunks 3 and 4 WHILE modal is open — should be buffered.
	chunk3 := StreamEventMsg{Event: StreamEvent{Type: "text", Content: "chunk3"}}
	chunk4 := StreamEventMsg{Event: StreamEvent{Type: "text", Content: "chunk4"}}
	chunk5 := StreamEventMsg{Event: StreamEvent{Type: "text", Content: "chunk5"}}

	m4, _ := m.Update(chunk3)
	m = m4.(*Model)
	m5, _ := m.Update(chunk4)
	m = m5.(*Model)
	m6, _ := m.Update(chunk5)
	m = m6.(*Model)

	// Viewport must not have changed while modal is open.
	assert.Equal(t, viewportBeforeModal, m.viewport.View(), "viewport must be paused while modal is open")

	// Buffered chunks count.
	assert.Equal(t, 3, len(m.pendingChunks), "3 chunks must be buffered while modal is open")

	// Resolve modal: Allow once.
	m7, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m7.(*Model)

	// Drain command chain to flush pending chunks.
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		m8, nextCmd := m.Update(msg)
		m = m8.(*Model)
		cmd = nextCmd
	}

	// After modal closes, pending chunks should be flushed.
	assert.Equal(t, 0, len(m.pendingChunks), "pending chunks must be flushed after modal closes")

	// Viewport must now include all 5 chunks.
	viewportAfter := m.viewport.View()
	assert.Contains(t, viewportAfter, "chunk3")
	assert.Contains(t, viewportAfter, "chunk4")
	assert.Contains(t, viewportAfter, "chunk5")
}
