package commands_test

import (
	"bytes"
	"strings"
	"testing"

	zKeyring "github.com/zalando/go-keyring"

	"github.com/modu-ai/mink/internal/auth/credential"
	"github.com/modu-ai/mink/internal/auth/keyring"
	"github.com/modu-ai/mink/internal/cli/commands"
)

// TestAuthDoctorOutput verifies the basic output contract of
// "mink doctor auth-keyring":
//   - Header includes "Backend: keyring"
//   - Table has exactly 8 provider rows
//   - No plaintext tokens appear in the output
//
// AC-CR-031: mink doctor auth-keyring output validation.
func TestAuthDoctorOutput(t *testing.T) {
	// Activate in-memory mock so the test runs without a real OS keyring.
	zKeyring.MockInit()

	// Store a credential for one provider so the output includes a "present"
	// row.
	b := keyring.NewBackend()
	_ = b.Store("anthropic", credential.APIKey{Value: "sk-ant-1234567890"})

	// Build the doctor command and capture its output.
	docCmd := commands.NewDoctorCommand()
	var buf bytes.Buffer
	docCmd.SetOut(&buf)
	// Execute the auth-keyring subcommand.
	docCmd.SetArgs([]string{"auth-keyring"})
	if err := docCmd.Execute(); err != nil {
		t.Fatalf("doctor auth-keyring: %v", err)
	}

	output := buf.String()

	// (1) Active backend must appear in the header.
	if !strings.Contains(output, "Backend: keyring") {
		t.Errorf("output missing 'Backend: keyring'; got:\n%s", output)
	}

	// (2) All 8 provider rows must be present.
	expected := []string{
		"anthropic", "deepseek", "openai_gpt", "codex",
		"zai_glm", "telegram_bot", "slack", "discord",
	}
	for _, p := range expected {
		if !strings.Contains(output, p) {
			t.Errorf("output missing provider row %q; got:\n%s", p, output)
		}
	}

	// (3) Plaintext must not appear in the output.
	// We stored "sk-ant-1234567890" — the full string must not be present.
	const plaintext = "sk-ant-1234567890"
	if strings.Contains(output, plaintext) {
		t.Errorf("output contains plaintext credential %q; want masked only", plaintext)
	}
}
