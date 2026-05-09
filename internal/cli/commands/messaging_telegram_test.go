package commands_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
