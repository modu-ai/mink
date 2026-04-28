// Package cli provides exit code constants for the goose CLI.
package cli

const (
	// ExitOK indicates successful execution (0).
	ExitOK = 0

	// ExitError indicates a general error (1).
	ExitError = 1

	// ExitUsage indicates incorrect command usage (2).
	ExitUsage = 2

	// ExitUnavailable indicates service unavailable (69, EX_UNAVAILABLE).
	ExitUnavailable = 69

	// ExitConfig indicates configuration error (78, EX_CONFIG).
	ExitConfig = 78
)
