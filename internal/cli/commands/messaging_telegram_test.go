package commands_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/cli/commands"
	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient is a test double for telegram.Client.
type fakeClient struct {
	getMeUser telegram.User
	getMeErr  error
	sendCalls []telegram.SendMessageRequest
	sendErr   error
}

func (f *fakeClient) GetMe(_ context.Context) (telegram.User, error) {
	return f.getMeUser, f.getMeErr
}

func (f *fakeClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	f.sendCalls = append(f.sendCalls, req)
	return telegram.Message{ID: 1, ChatID: req.ChatID, Text: req.Text}, f.sendErr
}

func (f *fakeClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	return nil, nil
}
func (f *fakeClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (f *fakeClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 10, ChatID: req.ChatID}, nil
}
func (f *fakeClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 11, ChatID: req.ChatID}, nil
}
func (f *fakeClient) EditMessageText(_ context.Context, req telegram.EditMessageTextRequest) (telegram.Message, error) {
	return telegram.Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (f *fakeClient) SetWebhook(_ context.Context, _ telegram.SetWebhookRequest) error { return nil }
func (f *fakeClient) DeleteWebhook(_ context.Context, _ bool) error                    { return nil }
func (f *fakeClient) SendChatAction(_ context.Context, _ int64, _ string) error        { return nil }

// runSetup executes the "messaging telegram setup" command with the given
// options and returns stdout content and any error.
func runSetup(t *testing.T, client telegram.Client, kr telegram.Keyring, cfgDir string, extraArgs ...string) (string, error) {
	t.Helper()
	cmd := commands.NewMessagingCommandWithDeps(client, kr, cfgDir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	args := append([]string{"telegram", "setup"}, extraArgs...)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// TestSetup_ValidToken verifies that a valid token stores the token in keyring
// and writes the config yaml.
func TestSetup_ValidToken(t *testing.T) {
	cfgDir := t.TempDir()
	kr := telegram.NewMemoryKeyring()
	client := &fakeClient{
		getMeUser: telegram.User{ID: 42, Username: "testbot", IsBot: true, FirstName: "Test"},
	}

	_, err := runSetup(t, client, kr, cfgDir, "--token", "valid-token")
	require.NoError(t, err)

	// Keyring should contain the token.
	stored, krErr := kr.Retrieve(telegram.KeyringService, telegram.KeyringKey)
	require.NoError(t, krErr)
	assert.Equal(t, "valid-token", string(stored))

	// Config yaml should exist with bot_username.
	cfgPath := filepath.Join(cfgDir, "telegram.yaml")
	data, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "testbot")
}

// TestSetup_InvalidToken verifies that a token rejected by the bot (GetMe error)
// does NOT write to keyring or yaml.
func TestSetup_InvalidToken(t *testing.T) {
	cfgDir := t.TempDir()
	kr := telegram.NewMemoryKeyring()
	client := &fakeClient{
		getMeErr: fmt.Errorf("telegram API error: Unauthorized (HTTP 401)"),
	}

	_, err := runSetup(t, client, kr, cfgDir, "--token", "bad-token")
	require.Error(t, err)

	// Keyring should remain empty.
	_, krErr := kr.Retrieve(telegram.KeyringService, telegram.KeyringKey)
	assert.Error(t, krErr, "keyring should not contain a token after failed setup")

	// Config yaml should not exist.
	cfgPath := filepath.Join(cfgDir, "telegram.yaml")
	_, statErr := os.Stat(cfgPath)
	assert.True(t, os.IsNotExist(statErr), "config yaml should not exist after failed setup")
}

// TestSetup_AlreadyConfigured verifies that running setup a second time returns E3.
func TestSetup_AlreadyConfigured(t *testing.T) {
	cfgDir := t.TempDir()
	kr := telegram.NewMemoryKeyring()
	client := &fakeClient{
		getMeUser: telegram.User{ID: 42, Username: "testbot", IsBot: true},
	}

	// First setup should succeed.
	_, err := runSetup(t, client, kr, cfgDir, "--token", "valid-token")
	require.NoError(t, err)

	// Second setup should fail with ErrAlreadyConfigured.
	_, err = runSetup(t, client, kr, cfgDir, "--token", "valid-token")
	require.Error(t, err)
	assert.ErrorIs(t, err, commands.ErrAlreadyConfigured)
}

// TestSetup_EmptyToken verifies that an empty token is rejected immediately.
func TestSetup_EmptyToken(t *testing.T) {
	cfgDir := t.TempDir()
	kr := telegram.NewMemoryKeyring()
	client := &fakeClient{}

	_, err := runSetup(t, client, kr, cfgDir, "--token", "")
	require.Error(t, err)

	// Keyring should remain empty.
	_, krErr := kr.Retrieve(telegram.KeyringService, telegram.KeyringKey)
	assert.Error(t, krErr)
}

// --- P2 approve / revoke / status tests ---

// runTelegramCmd runs a "messaging telegram <args>" command with a shared store path.
func runTelegramCmd(t *testing.T, cfgDir, storePath string, args ...string) (string, error) {
	t.Helper()
	cmd := commands.NewMessagingCommandWithDepsFull(nil, nil, cfgDir, storePath)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	fullArgs := append([]string{"telegram"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.ExecuteContext(context.Background())
	return buf.String(), err
}

// TestApprove_SetsAllowedTrue verifies that "approve <chat_id>" sets the user
// to allowed=true in the store.
func TestApprove_SetsAllowedTrue(t *testing.T) {
	cfgDir := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "telegram.db")

	// Pre-insert a blocked user.
	s, err := telegram.NewSqliteStore(storePath)
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, s.PutUserMapping(context.Background(), telegram.UserMapping{
		ChatID: 111, UserProfileID: "tg-111", Allowed: false, FirstSeenAt: now, LastSeenAt: now,
	}))
	s.Close() //nolint:errcheck

	out, err := runTelegramCmd(t, cfgDir, storePath, "approve", "111")
	require.NoError(t, err)
	assert.Contains(t, out, "approved")

	// Verify store state.
	s2, err := telegram.NewSqliteStore(storePath)
	require.NoError(t, err)
	defer s2.Close() //nolint:errcheck
	m, found, _ := s2.GetUserMapping(context.Background(), 111)
	require.True(t, found)
	assert.True(t, m.Allowed)
}

// TestRevoke_SetsAllowedFalse verifies that "revoke <chat_id>" sets
// the user to allowed=false in the store.
func TestRevoke_SetsAllowedFalse(t *testing.T) {
	cfgDir := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "telegram.db")

	// Pre-insert an allowed user.
	s, err := telegram.NewSqliteStore(storePath)
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, s.PutUserMapping(context.Background(), telegram.UserMapping{
		ChatID: 222, UserProfileID: "tg-222", Allowed: true, FirstSeenAt: now, LastSeenAt: now,
	}))
	s.Close() //nolint:errcheck

	out, err := runTelegramCmd(t, cfgDir, storePath, "revoke", "222")
	require.NoError(t, err)
	assert.Contains(t, out, "revoked")

	// Verify store state.
	s2, err := telegram.NewSqliteStore(storePath)
	require.NoError(t, err)
	defer s2.Close() //nolint:errcheck
	m, found, _ := s2.GetUserMapping(context.Background(), 222)
	require.True(t, found)
	assert.False(t, m.Allowed)
}

// TestApprove_InvalidChatID verifies that a non-numeric chat_id argument returns
// an error and exits 1.
func TestApprove_InvalidChatID(t *testing.T) {
	cfgDir := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "telegram.db")
	_, err := runTelegramCmd(t, cfgDir, storePath, "approve", "not-a-number")
	require.Error(t, err)
}

// TestStatus_NotConfigured verifies the fast path when config yaml is absent.
func TestStatus_NotConfigured(t *testing.T) {
	cfgDir := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "telegram.db")
	out, err := runTelegramCmd(t, cfgDir, storePath, "status")
	require.NoError(t, err)
	assert.Contains(t, out, "not configured")
}

// TestStatus_Configured verifies the configured path shows basic statistics.
func TestStatus_Configured(t *testing.T) {
	cfgDir := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "telegram.db")

	// Write a minimal config yaml.
	cfgContent := "bot_username: mybot\nmode: polling\naudit_enabled: true\nauto_admit_first_user: false\ndefault_streaming: false\n"
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "telegram.yaml"), []byte(cfgContent), 0o600))

	// Insert some mappings.
	s, err := telegram.NewSqliteStore(storePath)
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, s.PutUserMapping(context.Background(), telegram.UserMapping{ChatID: 1, UserProfileID: "tg-1", Allowed: true, FirstSeenAt: now, LastSeenAt: now}))
	require.NoError(t, s.PutUserMapping(context.Background(), telegram.UserMapping{ChatID: 2, UserProfileID: "tg-2", Allowed: false, FirstSeenAt: now, LastSeenAt: now}))
	require.NoError(t, s.PutLastOffset(context.Background(), 42))
	s.Close() //nolint:errcheck

	out, err := runTelegramCmd(t, cfgDir, storePath, "status")
	require.NoError(t, err)
	assert.Contains(t, out, "mybot")
	assert.Contains(t, out, "polling")
}
