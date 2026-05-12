// Package commands provides session subcommands for the goose CLI.
package commands

import (
	"fmt"
	"github.com/modu-ai/mink/internal/cli/session"
	"github.com/spf13/cobra"
)

// NewSessionCommand creates the session subcommand with list/load/save/rm.
// @MX:ANCHOR NewSessionCommand creates the session command tree.
// @MX:REASON fan_in >= 3 (called by rootcmd, tests, and future extensions).
func NewSessionCommand(daemonAddr string) *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage chat sessions",
		Long:  `Manage chat sessions (save, load, list, remove).`,
	}

	// list subcommand
	sessionCmd.AddCommand(newSessionListCommand())

	// load subcommand
	sessionCmd.AddCommand(newSessionLoadCommand(daemonAddr))

	// save subcommand (errors - should use /save in TUI)
	sessionCmd.AddCommand(newSessionSaveCommand())

	// rm subcommand
	sessionCmd.AddCommand(newSessionRmCommand())

	return sessionCmd
}

// newSessionListCommand creates the session list command.
func newSessionListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all saved sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := session.List()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			if len(names) == 0 {
				cmd.OutOrStdout().Write([]byte("No sessions found\n"))
				return nil
			}

			// Print one session per line
			for _, name := range names {
				cmd.OutOrStdout().Write([]byte(name + "\n"))
			}

			return nil
		},
	}
}

// newSessionLoadCommand creates the session load command.
// @MX:NOTE load reads session file and enters TUI with viewport populated (REQ-CLI-011).
func newSessionLoadCommand(daemonAddr string) *cobra.Command {
	return &cobra.Command{
		Use:   "load <name>",
		Short: "Load a session and enter chat mode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			messages, err := session.Load(name)
			if err != nil {
				return fmt.Errorf("failed to load session: %w", err)
			}

			// TODO: Phase D - Enter TUI with loaded messages
			// For now, just print message count
			cmd.OutOrStdout().Write([]byte(fmt.Sprintf("Loaded %d messages from session '%s'\n", len(messages), name)))
			cmd.OutOrStdout().Write([]byte("TUI mode not yet implemented (Phase D)\n"))

			return nil
		},
	}
}

// newSessionSaveCommand creates the session save command.
// REQ-CLI-012: /save should only work in TUI, not as CLI command.
// @MX:NOTE This command errors to redirect users to /save in chat mode.
func newSessionSaveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "save <name>",
		Short: "Save current session (only available in chat mode)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("save command only works in chat mode\nUse '/save %s' while in chat to save the session", args[0])
		},
	}
}

// newSessionRmCommand creates the session rm command.
func newSessionRmCommand() *cobra.Command {
	var yesFlag bool

	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a saved session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Require confirmation unless --yes flag is set
			if !yesFlag {
				cmd.Printf("Remove session '%s'? [y/N] ", name)
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "Y" {
					cmd.OutOrStdout().Write([]byte("Cancelled\n"))
					return nil
				}
			}

			if err := session.Delete(name); err != nil {
				return fmt.Errorf("failed to remove session: %w", err)
			}

			cmd.OutOrStdout().Write([]byte(fmt.Sprintf("Removed session '%s'\n", name)))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Skip confirmation prompt")

	return cmd
}
