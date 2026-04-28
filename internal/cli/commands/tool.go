// Package commands provides tool management commands for goose CLI.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ToolInfo represents information about an available tool.
// @MX:ANCHOR This type defines the tool metadata structure.
type ToolInfo struct {
	Name        string
	Description string
}

// ToolRegistry defines the interface for tool discovery.
// @MX:ANCHOR This abstraction allows different tool sources (static, daemon, plugin).
type ToolRegistry interface {
	// ListTools returns all available tools.
	ListTools() ([]ToolInfo, error)
}

// StaticToolRegistry provides a hardcoded list of default tools.
// @MX:NOTE This is a placeholder. Future phases will fetch tools from the daemon.
type StaticToolRegistry struct {
	tools []ToolInfo
}

// NewStaticToolRegistry creates a new static tool registry with default tools.
// @MX:ANCHOR This is the primary constructor for StaticToolRegistry.
func NewStaticToolRegistry() *StaticToolRegistry {
	return &StaticToolRegistry{
		tools: []ToolInfo{
			{Name: "read", Description: "Read file contents"},
			{Name: "write", Description: "Write or create files"},
			{Name: "edit", Description: "Edit existing files with string replacement"},
			{Name: "bash", Description: "Execute shell commands"},
			{Name: "browse", Description: "Browse web pages and fetch content"},
			{Name: "grep", Description: "Search for patterns in files"},
		},
	}
}

// ListTools returns the hardcoded list of tools.
func (s *StaticToolRegistry) ListTools() ([]ToolInfo, error) {
	return s.tools, nil
}

// NewToolCommand creates the tool command with subcommands.
// @MX:ANCHOR This function creates the complete tool command tree.
func NewToolCommand(registry ToolRegistry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage goose tools",
		Long:  `List and manage available tools for goose.`,
	}

	// Add subcommands
	cmd.AddCommand(newToolListCommand(registry))

	return cmd
}

// newToolListCommand creates the tool list subcommand.
func newToolListCommand(registry ToolRegistry) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available tools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := registry.ListTools()
			if err != nil {
				return fmt.Errorf("goose: failed to list tools: %w", err)
			}

			if len(tools) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tools available.")
				return nil
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Available tools:")
			fmt.Fprintln(out)

			for _, tool := range tools {
				fmt.Fprintf(out, "  %s\t%s\n", tool.Name, tool.Description)
			}

			return nil
		},
	}
}
