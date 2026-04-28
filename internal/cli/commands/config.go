// Package commands provides config management commands for goose CLI.
package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// ErrConfigKeyNotFound is returned when a config key does not exist.
// @MX:ANCHOR This error identifies missing config keys.
var ErrConfigKeyNotFound = fmt.Errorf("config key not found")

// ConfigStore defines the interface for config storage.
// @MX:ANCHOR This abstraction allows different storage backends (memory, file, database).
type ConfigStore interface {
	// Get retrieves a config value by key.
	// Returns ErrConfigKeyNotFound if the key does not exist.
	Get(key string) (string, error)

	// Set stores a config value.
	Set(key, value string) error

	// List returns all config key-value pairs.
	List() (map[string]string, error)
}

// MemoryConfigStore provides an in-memory config storage implementation.
// @MX:NOTE This is a placeholder implementation. Will be replaced with file-based config in future phases.
type MemoryConfigStore struct {
	config map[string]string
}

// NewMemoryConfigStore creates a new in-memory config store.
// @MX:ANCHOR This is the primary constructor for MemoryConfigStore.
func NewMemoryConfigStore() *MemoryConfigStore {
	return &MemoryConfigStore{
		config: make(map[string]string),
	}
}

// Get retrieves a config value by key.
func (m *MemoryConfigStore) Get(key string) (string, error) {
	val, ok := m.config[key]
	if !ok {
		return "", ErrConfigKeyNotFound
	}
	return val, nil
}

// Set stores a config value.
func (m *MemoryConfigStore) Set(key, value string) error {
	m.config[key] = value
	return nil
}

// List returns all config key-value pairs.
func (m *MemoryConfigStore) List() (map[string]string, error) {
	return m.config, nil
}

// NewConfigCommand creates the config command with subcommands.
// @MX:ANCHOR This function creates the complete config command tree.
func NewConfigCommand(store ConfigStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage goose configuration",
		Long:  `Get, set, and list configuration values for goose.`,
	}

	// Add subcommands
	cmd.AddCommand(newConfigGetCommand(store))
	cmd.AddCommand(newConfigSetCommand(store))
	cmd.AddCommand(newConfigListCommand(store))

	return cmd
}

// newConfigGetCommand creates the config get subcommand.
func newConfigGetCommand(store ConfigStore) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			value, err := store.Get(key)
			if err != nil {
				return fmt.Errorf("goose: %w: %s", err, key)
			}

			fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}
}

// newConfigSetCommand creates the config set subcommand.
func newConfigSetCommand(store ConfigStore) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			if err := store.Set(key, value); err != nil {
				return fmt.Errorf("goose: failed to set config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Config updated: %s = %s\n", key, value)
			return nil
		},
	}
}

// newConfigListCommand creates the config list subcommand.
func newConfigListCommand(store ConfigStore) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := store.List()
			if err != nil {
				return fmt.Errorf("goose: failed to list config: %w", err)
			}

			if len(config) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No config values set.")
				return nil
			}

			// Sort keys for consistent output
			keys := make([]string, 0, len(config))
			for k := range config {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			out := cmd.OutOrStdout()
			for _, key := range keys {
				fmt.Fprintf(out, "%s = %s\n", key, config[key])
			}

			return nil
		},
	}
}
