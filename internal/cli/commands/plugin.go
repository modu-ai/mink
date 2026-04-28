// Package commands provides plugin management commands for goose CLI.
// @MX:TODO This is a stub implementation. Real plugin system will be implemented in future phases.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewPluginCommand creates the plugin command with subcommands.
// @MX:ANCHOR This function creates the complete plugin command tree (stub implementation).
func NewPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage goose plugins",
		Long:  `List, install, and remove goose plugins.`,
	}

	// Add subcommands (all stubs for now)
	cmd.AddCommand(newPluginListCommand())
	cmd.AddCommand(newPluginInstallCommand())
	cmd.AddCommand(newPluginRemoveCommand())

	return cmd
}

// newPluginListCommand creates the plugin list subcommand.
func newPluginListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "No plugins installed.")
		},
	}
}

// newPluginInstallCommand creates the plugin install subcommand.
// @MX:TODO Implement actual plugin installation in future phases.
func newPluginInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install <name>",
		Short: "Install a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginName := args[0]
			return fmt.Errorf("goose: plugin system not yet implemented (cannot install '%s')", pluginName)
		},
	}
}

// newPluginRemoveCommand creates the plugin remove subcommand.
// @MX:TODO Implement actual plugin removal in future phases.
func newPluginRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginName := args[0]
			return fmt.Errorf("goose: plugin system not yet implemented (cannot remove '%s')", pluginName)
		},
	}
}
