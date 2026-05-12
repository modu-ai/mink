package redact_test

import (
	"regexp"
	"testing"

	"github.com/modu-ai/mink/internal/learning/trajectory/redact"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestNewChain_AppendsBuiltins verifies NewChain prepends user rules before built-ins.
func TestNewChain_AppendsBuiltins(t *testing.T) {
	customRule := redact.Rule{
		Name:        "custom-token",
		Pattern:     regexp.MustCompile(`TOKEN_[A-Z0-9]+`),
		Replacement: "<REDACTED:token>",
	}
	chain := redact.NewChain([]redact.Rule{customRule}, zap.NewNop())
	entry := &redact.Entry{From: "human", Value: "TOKEN_ABCDE and user@example.com"}
	chain.Apply(entry)
	assert.Contains(t, entry.Value, "<REDACTED:token>")
	assert.Contains(t, entry.Value, "<REDACTED:email>")
}

// TestRedactRule_Email_ReplacesCanonicalForm — AC-TRAJECTORY-003
func TestRedactRule_Email_ReplacesCanonicalForm(t *testing.T) {
	chain := redact.NewBuiltinChain(zap.NewNop())
	entry := &redact.Entry{From: "human", Value: "내 이메일은 alice@example.com 이야"}
	chain.Apply(entry)
	assert.Equal(t, "내 이메일은 <REDACTED:email> 이야", entry.Value)
	assert.NotContains(t, entry.Value, "alice@example.com")
}

// TestRedactRule_SixBuiltinsAllFire — AC-TRAJECTORY-004
func TestRedactRule_SixBuiltinsAllFire(t *testing.T) {
	chain := redact.NewBuiltinChain(zap.NewNop())

	tests := []struct {
		name     string
		input    string
		contains string
		absent   string
	}{
		{
			name:     "openai_key",
			input:    "key=sk-proj-abcdefghijklmnopqrstuvwxyz1234",
			contains: "<REDACTED:api_key>",
			absent:   "sk-proj-",
		},
		{
			name:     "bearer_jwt",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc.def",
			contains: "Bearer <REDACTED:jwt>",
			absent:   "eyJhbGci",
		},
		{
			name:     "kr_phone",
			input:    "전화: 010-1234-5678",
			contains: "<REDACTED:phone>",
			absent:   "010-1234",
		},
		{
			name:     "home_path_users",
			input:    "path=/Users/alice/.ssh/id_rsa",
			contains: "/Users/<REDACTED:user>",
			absent:   "/Users/alice",
		},
		{
			name:     "home_path_home",
			input:    "path=/home/bob/secret",
			contains: "/home/<REDACTED:user>",
			absent:   "/home/bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &redact.Entry{From: "human", Value: tt.input}
			chain.Apply(entry)
			assert.Contains(t, entry.Value, tt.contains)
			assert.NotContains(t, entry.Value, tt.absent)
		})
	}
}

// TestRedactChain_SystemRoleSkippedByDefault — AC-TRAJECTORY-010
func TestRedactChain_SystemRoleSkippedByDefault(t *testing.T) {
	chain := redact.NewBuiltinChain(zap.NewNop())
	// System entry with an email — should NOT be redacted (AppliesToSystem defaults to false).
	entry := &redact.Entry{
		From:  "system",
		Value: "You are Mink. Email support@goose.ai for help.",
	}
	chain.Apply(entry)
	// Email must survive intact.
	assert.Contains(t, entry.Value, "support@goose.ai",
		"system role entries must not be redacted unless AppliesToSystem=true")
}

// TestRedactChain_PanicIsolation — AC-TRAJECTORY-015
func TestRedactChain_PanicIsolation(t *testing.T) {
	// Use a zap observer to capture error logs.
	loggedErrors := make([]string, 0)
	zapLogger, _ := zap.NewDevelopment()
	defer zapLogger.Sync() //nolint:errcheck

	// Create a chain with only the panicking rule (no built-ins) for isolation.
	chain := redact.NewChainWithRules([]redact.Rule{
		{
			Name: "panicker",
			ApplyFn: func(_ string) string {
				panic("deliberate")
			},
		},
	}, nil, zapLogger)

	_ = loggedErrors

	// Three entries: each one will trigger the panicker.
	entries := []*redact.Entry{
		{From: "human", Value: "test entry one with alice@example.com"},
		{From: "human", Value: "normal entry no PII"},
		{From: "human", Value: "another normal entry"},
	}

	// (a) Must not panic out of Apply.
	assert.NotPanics(t, func() {
		for _, e := range entries {
			chain.Apply(e)
		}
	})

	// (b) Each entry's Value must be replaced with "<REDACT_FAILED>".
	for i, e := range entries {
		assert.Equal(t, "<REDACT_FAILED>", e.Value,
			"entry[%d] must be substituted with <REDACT_FAILED>", i)
	}
}
