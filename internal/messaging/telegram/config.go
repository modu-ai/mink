package telegram

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ErrPlainTokenRejected is returned when a YAML config file contains a bot_token
// field, which violates REQ-MTGM-N01 (tokens must not be stored in plain text).
var ErrPlainTokenRejected = errors.New("telegram: bot_token field in yaml is not allowed; use keyring or env var")

// Config holds the Telegram channel configuration loaded from
// ~/.goose/messaging/telegram.yaml (or a custom path).
type Config struct {
	// BotUsername is the bot's @username (without @), populated during setup.
	BotUsername string `yaml:"bot_username"`

	// AllowedUsers lists the Telegram user IDs permitted to interact with the bot.
	// An empty list means no users are allowed unless AutoAdmitFirstUser is true.
	AllowedUsers []int64 `yaml:"allowed_users"`

	// Mode selects the update ingestion strategy. Valid values: "polling", "webhook".
	// Defaults to "polling" when omitted.
	Mode string `yaml:"mode"`

	// AuditEnabled controls whether inbound and outbound events are appended to
	// the AUDIT-001 log (REQ-MTGM-U01).
	AuditEnabled bool `yaml:"audit_enabled"`

	// AutoAdmitFirstUser automatically grants admin access to the first user who
	// messages the bot when AllowedUsers is empty (REQ-MTGM-S04).
	AutoAdmitFirstUser bool `yaml:"auto_admit_first_user"`

	// DefaultStreaming controls whether responses are streamed by default.
	DefaultStreaming bool `yaml:"default_streaming"`
}

// rawConfig is the internal struct used to detect forbidden fields during parsing.
// Separating it from Config avoids exposing bot_token in the public API.
type rawConfig struct {
	BotToken           *string `yaml:"bot_token"` // must be nil after load
	BotUsername        string  `yaml:"bot_username"`
	AllowedUsers       []int64 `yaml:"allowed_users"`
	Mode               string  `yaml:"mode"`
	AuditEnabled       bool    `yaml:"audit_enabled"`
	AutoAdmitFirstUser bool    `yaml:"auto_admit_first_user"`
	DefaultStreaming   bool    `yaml:"default_streaming"`
}

// LoadConfig reads and validates the Telegram channel YAML configuration at path.
//
// It returns ErrPlainTokenRejected if the file contains a bot_token field
// (REQ-MTGM-N01). Missing files are wrapped as fs.ErrNotExist.
//
// @MX:ANCHOR: [AUTO] LoadConfig is the single config loading entry point for the telegram package.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 N01; fan_in via bootstrap, start subcommand, tests, and future P2 wiring.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("telegram: read config %s: %w", path, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("telegram: parse config %s: %w", path, err)
	}

	// Reject any yaml that includes a bot_token field, even if empty.
	// Storing the token in plain text violates REQ-MTGM-N01 / REQ-MTGM-U02.
	if raw.BotToken != nil {
		return nil, ErrPlainTokenRejected
	}

	cfg := &Config{
		BotUsername:        raw.BotUsername,
		AllowedUsers:       raw.AllowedUsers,
		Mode:               raw.Mode,
		AuditEnabled:       raw.AuditEnabled,
		AutoAdmitFirstUser: raw.AutoAdmitFirstUser,
		DefaultStreaming:   raw.DefaultStreaming,
	}

	// Apply defaults.
	if cfg.Mode == "" {
		cfg.Mode = "polling"
	}

	return cfg, nil
}

// DefaultConfigPath returns the default path for the Telegram config file.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("telegram: determine home dir: %w", err)
	}
	return home + "/.goose/messaging/telegram.yaml", nil
}
