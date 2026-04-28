// Package commands provides unit tests for the tool command.
package commands

import (
	"bytes"
	"strings"
	"testing"
)

// TestNewToolCommand verifies the tool command is properly initialized.
func TestNewToolCommand(t *testing.T) {
	registry := NewStaticToolRegistry()
	cmd := NewToolCommand(registry)

	if cmd == nil {
		t.Fatal("NewToolCommand returned nil")
	}

	if cmd.Use != "tool" {
		t.Errorf("expected Use 'tool', got '%s'", cmd.Use)
	}

	// Verify list subcommand exists
	hasList := false
	for _, subcmd := range cmd.Commands() {
		if subcmd.Use == "list" {
			hasList = true
			break
		}
	}

	if !hasList {
		t.Error("tool command should have 'list' subcommand")
	}
}

// TestToolListCommand verifies the list subcommand behavior.
func TestToolListCommand(t *testing.T) {
	tests := []struct {
		name        string
		registry    ToolRegistry
		args        []string
		exitCode    int
		verifyOut   func(*testing.T, string)
	}{
		{
			name: "list default tools",
			registry: NewStaticToolRegistry(),
			args: []string{"list"},
			exitCode: 0,
			verifyOut: func(t *testing.T, out string) {
				// Verify some default tools are listed
				expectedTools := []string{"read", "write", "edit", "bash"}
				for _, tool := range expectedTools {
					if !strings.Contains(out, tool) {
						t.Errorf("expected output to contain '%s', got: %s", tool, out)
					}
				}
			},
		},
		{
			name: "list with custom registry",
			registry: &mockToolRegistry{
				tools: []ToolInfo{
					{Name: "custom_tool", Description: "A custom tool"},
				},
			},
			args: []string{"list"},
			exitCode: 0,
			verifyOut: func(t *testing.T, out string) {
				if !strings.Contains(out, "custom_tool") {
					t.Errorf("expected output to contain 'custom_tool', got: %s", out)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewToolCommand(tt.registry)
			cmd.SetArgs(tt.args)

			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(bytes.NewBuffer(nil))

			exitCode := ExecuteWithCommand(cmd)

			if exitCode != tt.exitCode {
				t.Errorf("expected exit code %d, got %d", tt.exitCode, exitCode)
			}

			if tt.verifyOut != nil {
				tt.verifyOut(t, out.String())
			}
		})
	}
}

// TestStaticToolRegistry verifies the default tool registry.
func TestStaticToolRegistry(t *testing.T) {
	registry := NewStaticToolRegistry()

	tools, err := registry.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) == 0 {
		t.Error("expected at least one tool")
	}

	// Verify each tool has required fields
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool name should not be empty")
		}
		if tool.Description == "" {
			t.Error("tool description should not be empty")
		}
	}
}

// mockToolRegistry is a mock implementation of ToolRegistry for testing.
type mockToolRegistry struct {
	tools []ToolInfo
	err   error
}

func (m *mockToolRegistry) ListTools() ([]ToolInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tools, nil
}
