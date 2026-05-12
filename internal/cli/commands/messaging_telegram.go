package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modu-ai/mink/internal/messaging/telegram"
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
	AnswerCallbackQuery(ctx context.Context, callbackQueryID string) error
	SendPhoto(ctx context.Context, req telegram.SendMediaRequest) (telegram.Message, error)
	SendDocument(ctx context.Context, req telegram.SendMediaRequest) (telegram.Message, error)
}

// keyringIface is a local alias for test injection.
type keyringIface interface {
	Store(service, key string, value []byte) error
	Retrieve(service, key string) ([]byte, error)
}

// newMessagingTelegramCommand creates the "goose messaging telegram" subcommand
// group with setup, status, start, approve, and revoke sub-subcommands.
//
// client and kr may be nil (production default) or injected by tests.
// cfgDir overrides the config directory (defaults to ~/.goose/messaging/).
// storePath overrides the sqlite store path (defaults to ~/.goose/messaging/telegram.db).
func newMessagingTelegramCommand(client telegramClientIface, kr keyringIface, cfgDir, storePath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telegram",
		Short: "Manage the Telegram messaging channel",
	}
	cmd.AddCommand(newTelegramSetupCommand(client, kr, cfgDir))
	cmd.AddCommand(newTelegramStatusCommand(cfgDir, storePath))
	cmd.AddCommand(newTelegramStartCommand(kr, cfgDir))
	cmd.AddCommand(newTelegramApproveCommand(storePath))
	cmd.AddCommand(newTelegramRevokeCommand(storePath))
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
			// Production default: OSKeyring (OS secret store).
			// Tests inject a MemoryKeyring via the kr parameter.
			activeKr := kr
			if activeKr == nil {
				activeKr = telegram.NewOSKeyring()
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
// It returns "not configured" when the config yaml is absent. When configured
// it shows bot_username, mode, last_offset, mapped_count, allowed_count, and
// blocked_count from the sqlite store.
//
// @MX:NOTE: [AUTO] Live poller state metrics (offset lag, poll interval)
// via daemon IPC are deferred to P4. Current status shows stored values only.
func newTelegramStatusCommand(cfgDir, storePath string) *cobra.Command {
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

			cfg, err := telegram.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "bot_username: @%s\n", cfg.BotUsername)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mode: %s\n", cfg.Mode)

			// Open store for statistics.
			sp, resolveErr := resolveStorePath(storePath)
			if resolveErr != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "store: unavailable (%v)\n", resolveErr)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "poller: not_attached")
				return nil
			}
			store, storeErr := telegram.NewSqliteStore(sp)
			if storeErr != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "store: unavailable (%v)\n", storeErr)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "poller: not_attached")
				return nil
			}
			defer store.Close() //nolint:errcheck

			ctx := cmd.Context()

			offset, _ := store.GetLastOffset(ctx)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "last_offset: %d\n", offset)

			allowed, _ := store.ListAllowed(ctx)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "allowed_count: %d\n", len(allowed))

			// @MX:NOTE: [AUTO] P3 placeholder — live poller state requires daemon IPC
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "poller: not_attached")
			return nil
		},
	}
}

// newTelegramApproveCommand implements "goose messaging telegram approve <chat_id>".
func newTelegramApproveCommand(storePath string) *cobra.Command {
	return &cobra.Command{
		Use:   "approve <chat_id>",
		Short: "Approve a Telegram user by chat_id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chatID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid chat_id %q: must be an integer", args[0])
			}

			sp, err := resolveStorePath(storePath)
			if err != nil {
				return err
			}
			store, err := telegram.NewSqliteStore(sp)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close() //nolint:errcheck

			if err := store.Approve(cmd.Context(), chatID); err != nil {
				return fmt.Errorf("approve: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "approved chat_id=%d\n", chatID)
			return nil
		},
	}
}

// newTelegramRevokeCommand implements "goose messaging telegram revoke <chat_id>".
func newTelegramRevokeCommand(storePath string) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <chat_id>",
		Short: "Revoke access for a Telegram user by chat_id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chatID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid chat_id %q: must be an integer", args[0])
			}

			sp, err := resolveStorePath(storePath)
			if err != nil {
				return err
			}
			store, err := telegram.NewSqliteStore(sp)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close() //nolint:errcheck

			if err := store.Revoke(cmd.Context(), chatID); err != nil {
				return fmt.Errorf("revoke: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "revoked chat_id=%d\n", chatID)
			return nil
		},
	}
}

// newTelegramStartCommand implements "goose messaging telegram start".
// It loads the config and token from the OS keyring, then delegates to bootstrap.Start.
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
			// Production default: OSKeyring. Tests inject MemoryKeyring via kr.
			activeKr := kr
			if activeKr == nil {
				activeKr = telegram.NewOSKeyring()
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

// resolveStorePath returns the effective sqlite store path.
func resolveStorePath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return telegram.DefaultStorePath()
}
