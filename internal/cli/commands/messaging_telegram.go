package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ErrAlreadyConfigured is returned when "goose messaging telegram setup" is run
// on a channel that already has a config file (E3 from SPEC-GOOSE-MSG-TELEGRAM-001).
var ErrAlreadyConfigured = errors.New("telegram: already configured; delete the config file to reconfigure")

// telegramClientIface is a local alias for test injection.
// It mirrors telegram.Client and avoids import cycles in the commands package.
type telegramClientIface interface {
	GetMe(ctx context.Context) (telegram.User, error)
	SendMessage(ctx context.Context, req telegram.SendMessageRequest) (telegram.Message, error)
	GetUpdates(ctx context.Context, offset int, timeoutSec int) ([]telegram.Update, error)
}

// keyringIface is a local alias for test injection.
type keyringIface interface {
	Store(service, key string, value []byte) error
	Retrieve(service, key string) ([]byte, error)
}

// newMessagingTelegramCommand creates the "goose messaging telegram" subcommand
// group with setup, status, and start sub-subcommands.
//
// client and kr may be nil (production default) or injected by tests.
// cfgDir overrides the config directory (defaults to ~/.goose/messaging/).
func newMessagingTelegramCommand(client telegramClientIface, kr keyringIface, cfgDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telegram",
		Short: "Manage the Telegram messaging channel",
	}
	cmd.AddCommand(newTelegramSetupCommand(client, kr, cfgDir))
	cmd.AddCommand(newTelegramStatusCommand(cfgDir))
	cmd.AddCommand(newTelegramStartCommand(kr, cfgDir))
	return cmd
}

// newTelegramSetupCommand implements "goose messaging telegram setup".
// It validates the token via GetMe, stores it in the keyring, and writes
// ~/.goose/messaging/telegram.yaml.
func newTelegramSetupCommand(client telegramClientIface, kr keyringIface, cfgDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure the Telegram bot token and create the config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _ := cmd.Flags().GetString("token")
			token = strings.TrimSpace(token)
			if token == "" {
				return fmt.Errorf("--token is required (or set GOOSE_TELEGRAM_BOT_TOKEN env var)")
			}

			dir, err := resolveConfigDir(cfgDir)
			if err != nil {
				return err
			}

			cfgPath := filepath.Join(dir, "telegram.yaml")

			// Check if already configured (E3).
			if _, statErr := os.Stat(cfgPath); statErr == nil {
				return ErrAlreadyConfigured
			}

			// Validate token by calling GetMe.
			activeClient := client
			if activeClient == nil {
				activeClient, err = telegram.NewClient(token)
				if err != nil {
					return fmt.Errorf("create telegram client: %w", err)
				}
			}

			user, err := activeClient.GetMe(cmd.Context())
			if err != nil {
				return fmt.Errorf("invalid token (GetMe failed): %w", err)
			}

			// Store token in keyring.
			activeKr := kr
			if activeKr == nil {
				activeKr = telegram.NewMemoryKeyring()
			}
			if err := activeKr.Store(telegram.KeyringService, telegram.KeyringKey, []byte(token)); err != nil {
				return fmt.Errorf("store token in keyring: %w", err)
			}

			// Create config directory.
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return fmt.Errorf("create config directory %s: %w", dir, err)
			}

			// Write config yaml (no bot_token field — REQ-MTGM-N01).
			cfg := map[string]interface{}{
				"bot_username":          user.Username,
				"allowed_users":         []int64{},
				"mode":                  "polling",
				"audit_enabled":         true,
				"auto_admit_first_user": false,
				"default_streaming":     false,
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
				return fmt.Errorf("write config %s: %w", cfgPath, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "bot username: @%s\n", user.Username)
			// User-visible guidance (documentation output, not code comment):
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"Telegram 에서 @%s 에게 /start 보내세요\n", user.Username)
			return nil
		},
	}

	cmd.Flags().String("token", "", "Telegram bot token (or set GOOSE_TELEGRAM_BOT_TOKEN)")
	return cmd
}

// newTelegramStatusCommand implements "goose messaging telegram status".
// In P1 it returns "not configured" when the config yaml is absent.
//
// @MX:TODO P2 — expand status to show live polling state, offset, and allowed user count.
func newTelegramStatusCommand(cfgDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Telegram channel status",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveConfigDir(cfgDir)
			if err != nil {
				return err
			}
			cfgPath := filepath.Join(dir, "telegram.yaml")
			if _, statErr := os.Stat(cfgPath); os.IsNotExist(statErr) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "not configured")
				return nil
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "configured")
			return nil
		},
	}
}

// newTelegramStartCommand implements "goose messaging telegram start".
// In P1 it loads the config and token, then delegates to bootstrap.Start.
//
// @MX:TODO P2 — wire to real credproxy keyring for token retrieval.
func newTelegramStartCommand(kr keyringIface, cfgDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Telegram polling loop (foreground)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveConfigDir(cfgDir)
			if err != nil {
				return err
			}
			cfgPath := filepath.Join(dir, "telegram.yaml")

			cfg, err := telegram.LoadConfig(cfgPath)
			if err != nil {
				if os.IsNotExist(err) {
					// REQ-MTGM-S02: when token is absent, skip gracefully.
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
						"warn: Run goose messaging telegram setup first")
					return nil
				}
				return fmt.Errorf("load config: %w", err)
			}

			// Retrieve token from keyring.
			activeKr := kr
			if activeKr == nil {
				activeKr = telegram.NewMemoryKeyring()
			}
			tokenBytes, err := activeKr.Retrieve(telegram.KeyringService, telegram.KeyringKey)
			if err != nil {
				// Missing token → graceful skip per REQ-MTGM-S02.
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
					"warn: Run goose messaging telegram setup first")
				return nil
			}

			client, err := telegram.NewClient(string(tokenBytes))
			if err != nil {
				return fmt.Errorf("create telegram client: %w", err)
			}

			deps := telegram.Deps{
				Config: cfg,
				Client: client,
			}

			return telegram.Start(cmd.Context(), deps)
		},
	}
}

// resolveConfigDir returns the effective config directory path.
func resolveConfigDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home dir: %w", err)
	}
	return filepath.Join(home, ".goose", "messaging"), nil
}
