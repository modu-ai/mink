package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// AskClient defines the interface for chat interactions with the daemon.
// @MX:ANCHOR This interface allows mocking in tests and different implementations.
type AskClient interface {
	ChatStream(ctx context.Context, messages []Message) (<-chan StreamEvent, error)
}

// Message represents a chat message (defined in transport package, redefined here to avoid circular dependency).
// @MX:NOTE This is a simplified version for the command layer.
type Message struct {
	Role    string
	Content string
}

// StreamEvent represents a streaming response event (from transport package).
type StreamEvent struct {
	Type    string
	Content string
}

// NewAskCommand creates the ask subcommand.
// @MX:NOTE Default timeout is 30 seconds (REQ-CLI-009).
func NewAskCommand(client AskClient, defaultAddr string) *cobra.Command {
	var timeout time.Duration
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "ask <message>",
		Short: "Send a message to the LLM and stream the response",
		Long: `Send a message to the LLM and stream the response in real-time.

Examples:
  goose ask "Hello, how are you?"
  echo "Hello" | goose ask --stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Get address from persistent flag or use default
			addr, err := cmd.PersistentFlags().GetString("daemon-addr")
			if err != nil {
				return err
			}
			if addr == "" {
				addr = defaultAddr
			}

			// Validate input: either args or --stdin, not neither
			message := ""
			if len(args) > 0 {
				message = args[0]
			} else if useStdin {
				// Read from stdin
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				message = string(data)
			} else {
				// No args and no stdin → usage error
				// REQ-CLI-016: exit code 2
				return fmt.Errorf("requires a message argument or --stdin flag")
			}

			if message == "" {
				return fmt.Errorf("message cannot be empty")
			}

			// Set default timeout if not specified
			if timeout == 0 {
				timeout = 30 * time.Second // REQ-CLI-009
			}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Call ChatStream
			messages := []Message{{Role: "user", Content: message}}
			eventCh, err := client.ChatStream(ctx, messages)
			if err != nil {
				// Check if error is "daemon unreachable"
				if isUnreachableError(err) {
					// REQ-CLI-008: stderr message + exit 69
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "goose: daemon unreachable at %s\n", addr)
					return fmt.Errorf("daemon unreachable")
				}
				return fmt.Errorf("failed to start chat stream: %w", err)
			}

			// Stream events to stdout
			for event := range eventCh {
				switch event.Type {
				case "text":
					// Stream text to stdout
					_, _ = fmt.Fprint(cmd.OutOrStdout(), event.Content)
				case "error":
					// Print error to stderr and exit with error
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "goose: %s\n", event.Content)
					return fmt.Errorf("LLM error: %s", event.Content)
				case "done":
					// Stream ended successfully
					_, _ = fmt.Fprintln(cmd.OutOrStdout()) // Final newline
					return nil
				default:
					fmt.Fprintf(cmd.ErrOrStderr(), "goose: unknown event type: %s\n", event.Type)
					return fmt.Errorf("unknown event type: %s", event.Type)
				}
			}

			return nil
		},
	}

	// Command flags
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Timeout for the request (default 30s)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read message from stdin")

	return cmd
}

// isUnreachableError checks if error indicates daemon is unreachable.
// This is a placeholder - actual implementation depends on error type.
// @MX:TODO Make this interface-based once transport layer error types are defined.
func isUnreachableError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common "connection refused" patterns in error message
	errMsg := err.Error()
	return contains(errMsg, "connection refused") ||
		contains(errMsg, "daemon unreachable") ||
		contains(errMsg, "no such host") ||
		contains(errMsg, "connect: connection refused")
}

// contains checks if string contains substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
