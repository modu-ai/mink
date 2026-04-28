// Package commands provides unit tests for the plugin command.
package commands

import (
	"bytes"
	"strings"
	"testing"
)

// TestNewPluginCommand verifies the plugin command is properly initialized.
func TestNewPluginCommand(t *testing.T) {
	cmd := NewPluginCommand()

	if cmd == nil {
		t.Fatal("NewPluginCommand returned nil")
	}

	if cmd.Use != "plugin" {
		t.Errorf("expected Use 'plugin', got '%s'", cmd.Use)
	}

	// Verify subcommands exist (extract command name from Use field)
	subcommands := cmd.Commands()
	if len(subcommands) != 3 {
		t.Errorf("expected 3 subcommands, got %d", len(subcommands))
	}

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

	if !subcmdNames["list"] {
		t.Error("plugin command should have 'list' subcommand")
	}
	if !subcmdNames["install"] {
		t.Error("plugin command should have 'install' subcommand")
	}
	if !subcmdNames["remove"] {
		t.Error("plugin command should have 'remove' subcommand")
	}
}

// TestPluginListCommand verifies the list subcommand behavior.
func TestPluginListCommand(t *testing.T) {
	cmd := NewPluginCommand()
	cmd.SetArgs([]string{"list"})

	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(bytes.NewBuffer(nil))

	exitCode := ExecuteWithCommand(cmd)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	output := out.String()
	if !strings.Contains(output, "No plugins installed") {
		t.Errorf("expected 'No plugins installed' in output, got: %s", output)
	}
}

// TestPluginInstallCommand verifies the install subcommand behavior.
func TestPluginInstallCommand(t *testing.T) {
	cmd := NewPluginCommand()
	cmd.SetArgs([]string{"install", "test-plugin"})

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	exitCode := ExecuteWithCommand(cmd)

	// Install should fail (not implemented)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(errOut.String(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' in error output, got: %s", errOut.String())
	}
}

// TestPluginRemoveCommand verifies the remove subcommand behavior.
func TestPluginRemoveCommand(t *testing.T) {
	cmd := NewPluginCommand()
	cmd.SetArgs([]string{"remove", "test-plugin"})

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	exitCode := ExecuteWithCommand(cmd)

	// Remove should fail (not implemented)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(errOut.String(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' in error output, got: %s", errOut.String())
	}
}

// TestPluginInstallWithoutArgs verifies install requires plugin name.
func TestPluginInstallWithoutArgs(t *testing.T) {
	cmd := NewPluginCommand()
	cmd.SetArgs([]string{"install"})

	exitCode := ExecuteWithCommand(cmd)

	// Should fail with error (cobra returns ExitError for argument mismatches)
	if exitCode == 0 {
		t.Error("expected non-zero exit code")
	}
}

// TestPluginRemoveWithoutArgs verifies remove requires plugin name.
func TestPluginRemoveWithoutArgs(t *testing.T) {
	cmd := NewPluginCommand()
	cmd.SetArgs([]string{"remove"})

	exitCode := ExecuteWithCommand(cmd)

	// Should fail with error
	if exitCode == 0 {
		t.Error("expected non-zero exit code")
	}
}
