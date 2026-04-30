// Package commands provides unit tests for the config command.
package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Exit codes for testing
const (
	ExitOK          = 0
	ExitError       = 1
	ExitUsage       = 2
	ExitConfig      = 78
	ExitUnavailable = 69
)

// ExecuteWithCommand is a test helper that executes a command and returns the exit code.
func ExecuteWithCommand(cmd *cobra.Command) int {
	if err := cmd.Execute(); err != nil {
		return ExitError
	}
	return ExitOK
}

// mockConfigStore is a mock implementation of ConfigStore for testing.
// @MX:ANCHOR This mock enables testing without real config persistence.
type mockConfigStore struct {
	config  map[string]string
	getErr  error
	setErr  error
	listErr error
}

func newMockConfigStore() *mockConfigStore {
	return &mockConfigStore{
		config: make(map[string]string),
	}
}

func (m *mockConfigStore) Get(key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	val, ok := m.config[key]
	if !ok {
		return "", ErrConfigKeyNotFound
	}
	return val, nil
}

func (m *mockConfigStore) Set(key, value string) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.config[key] = value
	return nil
}

func (m *mockConfigStore) List() (map[string]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.config, nil
}

// TestNewConfigCommand verifies the config command is properly initialized.
// @MX:ANCHOR This test ensures the command structure is correct.
func TestNewConfigCommand(t *testing.T) {
	store := newMockConfigStore()
	cmd := NewConfigCommand(store)

	if cmd == nil {
		t.Fatal("NewConfigCommand returned nil")
	}

	if cmd.Use != "config" {
		t.Errorf("expected Use 'config', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Verify subcommands exist (Cobra Use field includes args, extract command name)
	subcommands := cmd.Commands()
	if len(subcommands) != 3 {
		t.Errorf("expected 3 subcommands, got %d", len(subcommands))
	}

	// Check subcommand names (extract first word from Use field)
	subcmdNames := make(map[string]bool)
	for _, subcmd := range subcommands {
		// Extract command name (first word before space)
		name := subcmd.Use
		for i, r := range name {
			if r == ' ' {
				name = name[:i]
				break
			}
		}
		subcmdNames[name] = true
	}

	if !subcmdNames["get"] {
		t.Error("config command should have 'get' subcommand")
	}
	if !subcmdNames["set"] {
		t.Error("config command should have 'set' subcommand")
	}
	if !subcmdNames["list"] {
		t.Error("config command should have 'list' subcommand")
	}
}

// TestConfigGetCommand verifies the get subcommand behavior.
func TestConfigGetCommand(t *testing.T) {
	tests := []struct {
		name        string
		store       *mockConfigStore
		args        []string
		expectedOut string
		expectedErr string
		exitCode    int
	}{
		{
			name:        "get existing key",
			store:       func() *mockConfigStore { m := newMockConfigStore(); m.config["test.key"] = "test-value"; return m }(),
			args:        []string{"get", "test.key"},
			expectedOut: "test-value\n",
			exitCode:    0,
		},
		{
			name:        "get non-existent key",
			store:       newMockConfigStore(),
			args:        []string{"get", "nonexistent"},
			expectedErr: "goose: config key not found: nonexistent",
			exitCode:    ExitError,
		},
		{
			name:        "get without args",
			store:       newMockConfigStore(),
			args:        []string{"get"},
			expectedErr: "goose: requires config key",
			exitCode:    ExitError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewConfigCommand(tt.store)
			cmd.SetArgs(tt.args)

			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(bytes.NewBuffer(nil))

			exitCode := ExecuteWithCommand(cmd)

			if exitCode != tt.exitCode {
				t.Errorf("expected exit code %d, got %d", tt.exitCode, exitCode)
			}

			if tt.expectedOut != "" && out.String() != tt.expectedOut {
				t.Errorf("expected output '%s', got '%s'", tt.expectedOut, out.String())
			}
		})
	}
}

// TestConfigSetCommand verifies the set subcommand behavior.
func TestConfigSetCommand(t *testing.T) {
	tests := []struct {
		name        string
		store       *mockConfigStore
		args        []string
		expectedOut string
		exitCode    int
		verifyStore func(*testing.T, *mockConfigStore)
	}{
		{
			name:        "set key value",
			store:       newMockConfigStore(),
			args:        []string{"set", "test.key", "test-value"},
			expectedOut: "Config updated: test.key = test-value\n",
			exitCode:    0,
			verifyStore: func(t *testing.T, store *mockConfigStore) {
				if val, ok := store.config["test.key"]; !ok || val != "test-value" {
					t.Errorf("key not set correctly: got '%s'", val)
				}
			},
		},
		{
			name:     "set without args",
			store:    newMockConfigStore(),
			args:     []string{"set"},
			exitCode: ExitError,
		},
		{
			name:     "set with missing value",
			store:    newMockConfigStore(),
			args:     []string{"set", "key"},
			exitCode: ExitError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewConfigCommand(tt.store)
			cmd.SetArgs(tt.args)

			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(bytes.NewBuffer(nil))

			exitCode := ExecuteWithCommand(cmd)

			if exitCode != tt.exitCode {
				t.Errorf("expected exit code %d, got %d", tt.exitCode, exitCode)
			}

			if tt.expectedOut != "" && out.String() != tt.expectedOut {
				t.Errorf("expected output '%s', got '%s'", tt.expectedOut, out.String())
			}

			if tt.verifyStore != nil {
				tt.verifyStore(t, tt.store)
			}
		})
	}
}

// TestConfigListCommand verifies the list subcommand behavior.
func TestConfigListCommand(t *testing.T) {
	t.Run("list with values", func(t *testing.T) {
		store := func() *mockConfigStore {
			m := newMockConfigStore()
			m.config["key1"] = "value1"
			m.config["key2"] = "value2"
			return m
		}()

		cmd := NewConfigCommand(store)
		cmd.SetArgs([]string{"list"})

		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(bytes.NewBuffer(nil))

		exitCode := ExecuteWithCommand(cmd)

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d", exitCode)
		}

		output := out.String()
		if !strings.Contains(output, "key1 = value1") {
			t.Errorf("expected output to contain 'key1 = value1', got '%s'", output)
		}
		if !strings.Contains(output, "key2 = value2") {
			t.Errorf("expected output to contain 'key2 = value2', got '%s'", output)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		store := newMockConfigStore()

		cmd := NewConfigCommand(store)
		cmd.SetArgs([]string{"list"})

		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(bytes.NewBuffer(nil))

		exitCode := ExecuteWithCommand(cmd)

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d", exitCode)
		}

		output := out.String()
		if !strings.Contains(output, "No config values set") {
			t.Errorf("expected 'No config values set' in output, got '%s'", output)
		}
	})
}

// TestMemoryConfigStore verifies the in-memory config store implementation.
func TestMemoryConfigStore(t *testing.T) {
	store := NewMemoryConfigStore()

	// Test Set and Get
	err := store.Set("test.key", "test-value")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := store.Get("test.key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "test-value" {
		t.Errorf("expected 'test-value', got '%s'", val)
	}

	// Test Get non-existent
	_, err = store.Get("nonexistent")
	if err != ErrConfigKeyNotFound {
		t.Errorf("expected ErrConfigKeyNotFound, got %v", err)
	}

	// Test List
	store.Set("key2", "value2")
	config, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(config) != 2 {
		t.Errorf("expected 2 items, got %d", len(config))
	}
}
