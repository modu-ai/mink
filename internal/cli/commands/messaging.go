package commands

import (
	"github.com/spf13/cobra"
)

// NewMessagingCommand creates the top-level "goose messaging" command.
// Messaging channel management (Telegram, etc.) is grouped here.
//
// @MX:ANCHOR: [AUTO] NewMessagingCommand is the parent entry point for all messaging subcommands.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1/P2; fan_in via rootcmd.go, tests, and future channel siblings.
func NewMessagingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messaging",
		Short: "Messaging channel management",
		Long:  "Manage messaging channels (Telegram, etc.) for the goose daemon.",
	}
	cmd.AddCommand(newMessagingTelegramCommand(nil, nil, "", ""))
	return cmd
}

// NewMessagingCommandWithDeps creates the "goose messaging" command with explicit
// dependencies injected for testing. client and kr may be nil for production
// use (they fall back to defaults during command execution).
func NewMessagingCommandWithDeps(client telegramClientIface, kr keyringIface, cfgDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messaging",
		Short: "Messaging channel management",
		Long:  "Manage messaging channels (Telegram, etc.) for the goose daemon.",
	}
	cmd.AddCommand(newMessagingTelegramCommand(client, kr, cfgDir, ""))
	return cmd
}

// NewMessagingCommandWithDepsFull creates the "goose messaging" command with
// all dependencies injected, including a custom sqlite store path.
// This variant is used in P2+ tests to inject the store path.
func NewMessagingCommandWithDepsFull(client telegramClientIface, kr keyringIface, cfgDir, storePath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messaging",
		Short: "Messaging channel management",
		Long:  "Manage messaging channels (Telegram, etc.) for the goose daemon.",
	}
	cmd.AddCommand(newMessagingTelegramCommand(client, kr, cfgDir, storePath))
	return cmd
}
