package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/spf13/cobra"
)

// NewAuditCommand creates the audit command with subcommands.
// REQ-AUDIT-004: WHEN goose audit query [--since=...] [--type=...] 가 실행되면
//
//	the system SHALL 구조화된 검색 결과를 반환한다
//
// @MX:ANCHOR: [AUTO] Main audit command entry point
// @MX:REASON: Primary CLI entry point for audit log operations, fan_in >= 3 (users, scripts, admin tools)
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-004
func NewAuditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Query and manage audit logs",
		Long:  `Query security audit logs from the filesystem.`,
	}

	// Add subcommands
	cmd.AddCommand(newAuditQueryCommand())

	return cmd
}

// newAuditQueryCommand creates the audit query subcommand.
// The query command reads audit logs and outputs filtered results as JSON.
func newAuditQueryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query audit logs",
		Long: `Query audit logs with optional filtering.

Outputs results as a JSON array to stdout. Supports filtering by time range
(RFC3339 timestamps) and event types (comma-separated).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flag values
			sinceStr, _ := cmd.Flags().GetString("since")
			untilStr, _ := cmd.Flags().GetString("until")
			typeStr, _ := cmd.Flags().GetString("type")
			logDir, _ := cmd.Flags().GetString("log-dir")

			// Parse --since flag
			var since *time.Time
			if sinceStr != "" {
				t, err := time.Parse(time.RFC3339, sinceStr)
				if err != nil {
					return fmt.Errorf("invalid --since timestamp: %w (expected RFC3339 format, e.g., 2026-04-29T12:00:00Z)", err)
				}
				since = &t
			}

			// Parse --until flag
			var until *time.Time
			if untilStr != "" {
				t, err := time.Parse(time.RFC3339, untilStr)
				if err != nil {
					return fmt.Errorf("invalid --until timestamp: %w (expected RFC3339 format, e.g., 2026-04-29T12:00:00Z)", err)
				}
				until = &t
			}

			// Parse --type flag (comma-separated)
			var types []audit.EventType
			if typeStr != "" {
				typeParts := strings.Split(typeStr, ",")
				types = make([]audit.EventType, 0, len(typeParts))
				for _, part := range typeParts {
					eventType := audit.EventType(strings.TrimSpace(part))
					types = append(types, eventType)
				}
			}

			// Set default log directory if not specified
			if logDir == "" {
				// Default to global audit log directory
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to determine home directory: %w", err)
				}
				logDir = fmt.Sprintf("%s/.goose/logs", homeDir)
			}

			// Build query options
			opts := audit.QueryOptions{
				Since: since,
				Until: until,
				Types: types,
			}

			// Execute query
			events, err := audit.Query(logDir, opts)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}

			// Output as JSON array
			output := cmd.OutOrStdout()
			encoder := json.NewEncoder(output)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(events); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().String("since", "", "Filter events after this timestamp (RFC3339 format, e.g., 2026-04-29T12:00:00Z)")
	cmd.Flags().String("until", "", "Filter events before this timestamp (RFC3339 format, e.g., 2026-04-29T12:00:00Z)")
	cmd.Flags().String("type", "", "Filter by event types (comma-separated, e.g., fs.write,permission.grant)")
	cmd.Flags().String("log-dir", "", "Path to audit log directory (default: ~/.goose/logs)")

	return cmd
}
