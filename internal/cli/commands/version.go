// Package commands provides CLI subcommands for the goose tool.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCommand creates the version subcommand.
// @MX:NOTE Version information is injected via ldflags at build time.
func NewVersionCommand(version, commit, builtAt string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "goose version %s (commit %s, built %s)\n",
				version, commit, builtAt)
			return nil
		},
	}
}
