package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewBriefingCommand(t *testing.T) {
	// Create command with mock factory
	cmd := NewBriefingCommand(MockBriefingCollectorFactory)

	// Test command registration
	if cmd.Use != "briefing" {
		t.Errorf("expected command Use='briefing', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected command Short to be set")
	}

	if cmd.Long == "" {
		t.Error("expected command Long to be set")
	}
}

func TestBriefingCmd_Flags(t *testing.T) {
	cmd := NewBriefingCommand(MockBriefingCollectorFactory)

	// Test flags are registered
	flags := cmd.Flags()

	// Check --plain flag
	plainFlag := flags.Lookup("plain")
	if plainFlag == nil {
		t.Error("--plain flag not registered")
	} else if plainFlag.DefValue != "false" {
		t.Errorf("expected --plain default='false', got '%s'", plainFlag.DefValue)
	}

	// Check --channels flag
	channelsFlag := flags.Lookup("channels")
	if channelsFlag == nil {
		t.Error("--channels flag not registered")
	}

	// Check --dry-run flag
	dryRunFlag := flags.Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("--dry-run flag not registered")
	} else if dryRunFlag.DefValue != "false" {
		t.Errorf("expected --dry-run default='false', got '%s'", dryRunFlag.DefValue)
	}
}

func TestBriefingCmd_Execute(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		flags           map[string]string
		wantInOutput    []string
		notWantInOutput []string
	}{
		{
			name: "default output (TTY with ANSI)",
			args: []string{},
			wantInOutput: []string{
				"MORNING BRIEFING",
				"Weather",
				"Temperature:",
				"18.5°C",
				"Journal Recall",
				"Date & Calendar",
				"Daily Mantra",
				"Every day is a new beginning",
			},
		},
		{
			name: "plain text output",
			args: []string{"--plain"},
			wantInOutput: []string{
				"MORNING BRIEFING",
				"Weather",
				"Temperature:",
			},
			notWantInOutput: []string{
				"\033[", // No ANSI codes
			},
		},
		{
			name: "with channels flag (no effect in M1)",
			args: []string{"--channels=weather,journal"},
			wantInOutput: []string{
				"MORNING BRIEFING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewBriefingCommand(MockBriefingCollectorFactory)

			// Set flags
			for k, v := range tt.flags {
				cmd.Flags().Set(k, v)
			}

			// Capture output
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			// Execute command
			err := cmd.RunE(cmd, tt.args)
			if err != nil {
				t.Fatalf("command execution failed: %v", err)
			}

			output := out.String()

			// Check expected content
			for _, mustContain := range tt.wantInOutput {
				if !strings.Contains(output, mustContain) {
					t.Errorf("output should contain '%s', got:\n%s", mustContain, output)
				}
			}

			// Check content that should NOT be present
			for _, mustNotContain := range tt.notWantInOutput {
				if strings.Contains(output, mustNotContain) {
					t.Errorf("output should NOT contain '%s', got:\n%s", mustNotContain, output)
				}
			}
		})
	}
}

func TestBriefingCmd_HelpOutput(t *testing.T) {
	cmd := NewBriefingCommand(MockBriefingCollectorFactory)

	// Set output to capture help
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Execute help
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	output := out.String()

	// Verify help content
	requiredInHelp := []string{
		"briefing",
		"weather",
		"recall",
		"mantra",
		"--plain",
		"--channels",
		"--dry-run",
	}

	for _, required := range requiredInHelp {
		if !strings.Contains(output, required) {
			t.Errorf("help should contain '%s'", required)
		}
	}
}

func TestMockBriefingCollectorFactory(t *testing.T) {
	weather, journal, date, mantra := MockBriefingCollectorFactory()

	if weather == nil {
		t.Error("weather collector should not be nil")
	}

	if journal == nil {
		t.Error("journal collector should not be nil")
	}

	if date == nil {
		t.Error("date collector should not be nil")
	}

	if mantra == nil {
		t.Error("mantra collector should not be nil")
	}

	// Test collectors return valid data
	ctx := context.Background()
	today := time.Now()

	weatherModule, weatherStatus := weather.Collect(ctx, "test-user", today)
	if weatherStatus != "ok" {
		t.Errorf("expected weather status='ok', got '%s'", weatherStatus)
	}
	if weatherModule == nil {
		t.Error("weather module should not be nil")
	}

	journalModule, journalStatus := journal.Collect(ctx, "test-user", today)
	if journalStatus != "ok" {
		t.Errorf("expected journal status='ok', got '%s'", journalStatus)
	}
	if journalModule == nil {
		t.Error("journal module should not be nil")
	}

	dateModule, dateStatus := date.Collect(ctx, "test-user", today)
	if dateStatus != "ok" {
		t.Errorf("expected date status='ok', got '%s'", dateStatus)
	}
	if dateModule == nil {
		t.Error("date module should not be nil")
	}

	mantraModule, mantraStatus := mantra.Collect(ctx, "test-user", today)
	if mantraStatus != "ok" {
		t.Errorf("expected mantra status='ok', got '%s'", mantraStatus)
	}
	if mantraModule == nil {
		t.Error("mantra module should not be nil")
	}
}
