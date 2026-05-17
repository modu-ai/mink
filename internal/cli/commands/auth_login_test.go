// Package commands — unit tests for mink login / mink logout.
//
// These tests use an in-process stubService to avoid touching real OS keyring
// or filesystem.  The inputReader is driven by a bytes.Buffer pipe so that
// masked password prompts work without a real TTY.
//
// AC-CR-012: CLI stores the correct Credential type for each provider.
// AC-CR-015: mink logout calls Delete on both backends (idempotent).
// AC-CR-032: mink logout provider ID validation.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (T-016)
package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// ---------------------------------------------------------------------------
// stubService — minimal credential.Service for unit tests
// ---------------------------------------------------------------------------

// stubCall records a single Store or Delete invocation.
type stubCall struct {
	method   string // "Store" or "Delete"
	provider string
	cred     credential.Credential // nil for Delete calls
}

// stubService is a test-only credential.Service that records method calls.
type stubService struct {
	calls     []stubCall
	storeErr  error // returned by every Store call when non-nil
	deleteErr error // returned by every Delete call when non-nil
}

func (s *stubService) Store(provider string, cred credential.Credential) error {
	s.calls = append(s.calls, stubCall{method: "Store", provider: provider, cred: cred})
	return s.storeErr
}

func (s *stubService) Load(provider string) (credential.Credential, error) {
	return nil, credential.ErrNotFound
}

func (s *stubService) Delete(provider string) error {
	s.calls = append(s.calls, stubCall{method: "Delete", provider: provider})
	return s.deleteErr
}

func (s *stubService) List() ([]string, error) { return nil, nil }

func (s *stubService) Health(provider string) (credential.HealthStatus, error) {
	return credential.HealthStatus{}, credential.ErrNotFound
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// makeSvcs returns a loginServices with the provided stubs for all three
// backend slots.
func makeSvcs(primary, kring, file *stubService) *loginServices {
	var fileSvc credential.Service
	if file != nil {
		fileSvc = file
	}
	return &loginServices{
		primary:   primary,
		keyringBE: kring,
		fileBE:    fileSvc,
	}
}

// pipe creates a *bytes.Buffer pre-loaded with newline-delimited lines.
// Each readPassword call consumes one line.
func pipe(lines ...string) *bytes.Buffer {
	return bytes.NewBufferString(strings.Join(lines, "\n") + "\n")
}

// fakeAPIKey is a test-only placeholder value; not a real credential.
const fakeAPIKey = "test-fake-apikey-1234"

// ---------------------------------------------------------------------------
// API key providers
// ---------------------------------------------------------------------------

func TestLoginAPIKey_Anthropic(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe(fakeAPIKey)

	var out bytes.Buffer
	if err := runLogin(t.Context(), "anthropic", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin anthropic: %v", err)
	}
	if len(primary.calls) != 1 || primary.calls[0].method != "Store" {
		t.Fatalf("expected 1 Store call, got %d calls", len(primary.calls))
	}
	call := primary.calls[0]
	if call.provider != "anthropic" {
		t.Errorf("provider: got %q, want %q", call.provider, "anthropic")
	}
	ak, ok := call.cred.(credential.APIKey)
	if !ok {
		t.Fatalf("expected APIKey credential, got %T", call.cred)
	}
	if ak.Value != fakeAPIKey {
		t.Errorf("APIKey.Value: got %q, want %q", ak.Value, fakeAPIKey)
	}
}

func TestLoginAPIKey_DeepSeek(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe("deepseek-fake-key-9999")

	var out bytes.Buffer
	if err := runLogin(t.Context(), "deepseek", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin deepseek: %v", err)
	}
	if len(primary.calls) != 1 {
		t.Fatalf("expected 1 Store call, got %d", len(primary.calls))
	}
	if _, ok := primary.calls[0].cred.(credential.APIKey); !ok {
		t.Fatalf("expected APIKey for deepseek, got %T", primary.calls[0].cred)
	}
}

func TestLoginAPIKey_OpenAIGPT(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe("gpt-fake-key-abc")

	var out bytes.Buffer
	if err := runLogin(t.Context(), "openai_gpt", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin openai_gpt: %v", err)
	}
	if _, ok := primary.calls[0].cred.(credential.APIKey); !ok {
		t.Fatalf("expected APIKey for openai_gpt")
	}
}

func TestLoginAPIKey_ZaiGLM(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe("glm-fake-key-xyz")

	var out bytes.Buffer
	if err := runLogin(t.Context(), "zai_glm", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin zai_glm: %v", err)
	}
	if _, ok := primary.calls[0].cred.(credential.APIKey); !ok {
		t.Fatalf("expected APIKey for zai_glm")
	}
}

func TestLoginAPIKey_EmptyValue(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe("") // empty line → empty API key

	var out bytes.Buffer
	err := runLogin(t.Context(), "anthropic", svcs.primary, newInputReader(in), &out)
	if err == nil {
		t.Error("expected error for empty API key, got nil")
	}
	// No Store call should have been made.
	if len(primary.calls) != 0 {
		t.Errorf("expected 0 Store calls for empty input, got %d", len(primary.calls))
	}
}

// ---------------------------------------------------------------------------
// Telegram bot token
// ---------------------------------------------------------------------------

func TestLoginBotToken_TelegramBot(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	// Format: <numeric-id>:<alphanumeric-string>
	in := pipe("123456789:FAKE-TelegramBotToken-Test")

	var out bytes.Buffer
	if err := runLogin(t.Context(), "telegram_bot", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin telegram_bot: %v", err)
	}
	if len(primary.calls) != 1 {
		t.Fatalf("expected 1 Store call, got %d", len(primary.calls))
	}
	bt, ok := primary.calls[0].cred.(credential.BotToken)
	if !ok {
		t.Fatalf("expected BotToken credential, got %T", primary.calls[0].cred)
	}
	if bt.Provider != "telegram_bot" {
		t.Errorf("BotToken.Provider: got %q, want %q", bt.Provider, "telegram_bot")
	}
}

// ---------------------------------------------------------------------------
// Slack combo
// ---------------------------------------------------------------------------

func TestLoginSlack(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	// Two lines: signing_secret then bot_token
	in := pipe("slack-sign-secret-value", "bot-fake-slack-token")

	var out bytes.Buffer
	if err := runLogin(t.Context(), "slack", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin slack: %v", err)
	}
	if len(primary.calls) != 1 {
		t.Fatalf("expected 1 Store call, got %d", len(primary.calls))
	}
	sc, ok := primary.calls[0].cred.(credential.SlackCombo)
	if !ok {
		t.Fatalf("expected SlackCombo credential, got %T", primary.calls[0].cred)
	}
	if sc.SigningSecret != "slack-sign-secret-value" {
		t.Errorf("SigningSecret: got %q", sc.SigningSecret)
	}
	if sc.BotToken != "bot-fake-slack-token" {
		t.Errorf("BotToken: got %q", sc.BotToken)
	}
}

func TestLoginSlack_EmptySecret(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe("", "bot-fake-slack-token") // empty signing secret

	var out bytes.Buffer
	err := runLogin(t.Context(), "slack", svcs.primary, newInputReader(in), &out)
	if err == nil {
		t.Error("expected error for empty signing secret")
	}
	if len(primary.calls) != 0 {
		t.Errorf("expected 0 Store calls for empty input, got %d", len(primary.calls))
	}
}

// ---------------------------------------------------------------------------
// Discord combo
// ---------------------------------------------------------------------------

func TestLoginDiscord(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	// public_key must be exactly 64 lowercase hex chars
	pubKey := strings.Repeat("a1", 32) // 64 chars
	in := pipe(pubKey, "discord-bot-token-value")

	var out bytes.Buffer
	if err := runLogin(t.Context(), "discord", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin discord: %v", err)
	}
	if len(primary.calls) != 1 {
		t.Fatalf("expected 1 Store call, got %d", len(primary.calls))
	}
	dc, ok := primary.calls[0].cred.(credential.DiscordCombo)
	if !ok {
		t.Fatalf("expected DiscordCombo credential, got %T", primary.calls[0].cred)
	}
	if dc.PublicKey != pubKey {
		t.Errorf("PublicKey: got %q, want %q", dc.PublicKey, pubKey)
	}
}

func TestLoginDiscord_EmptyBotToken(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	pubKey := strings.Repeat("b2", 32)
	in := pipe(pubKey, "") // empty bot token

	var out bytes.Buffer
	err := runLogin(t.Context(), "discord", svcs.primary, newInputReader(in), &out)
	if err == nil {
		t.Error("expected error for empty bot token")
	}
}

// ---------------------------------------------------------------------------
// Unknown provider
// ---------------------------------------------------------------------------

func TestLoginUnknownProvider(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)

	var out bytes.Buffer
	err := runLogin(t.Context(), "unknown_provider_xyz", svcs.primary, newInputReader(nil), &out)
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "알 수 없는 provider") {
		t.Errorf("error should mention unknown provider, got: %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// mink logout — both backends swept
// ---------------------------------------------------------------------------

func TestLogoutAllBackends_SweepsBoth(t *testing.T) {
	kring := &stubService{}
	file := &stubService{}

	var out bytes.Buffer
	if err := LogoutAllBackends("anthropic", kring, file, &out); err != nil {
		t.Fatalf("LogoutAllBackends: %v", err)
	}

	// Both backends should have received exactly one Delete call.
	if len(kring.calls) != 1 || kring.calls[0].method != "Delete" {
		t.Errorf("keyring: expected 1 Delete, got %v", kring.calls)
	}
	if len(file.calls) != 1 || file.calls[0].method != "Delete" {
		t.Errorf("file: expected 1 Delete, got %v", file.calls)
	}
}

func TestLogoutAllBackends_Idempotent_NotFound(t *testing.T) {
	// Both backends return ErrNotFound — this is not an error.
	kring := &stubService{deleteErr: credential.ErrNotFound}
	file := &stubService{deleteErr: credential.ErrNotFound}

	var out bytes.Buffer
	if err := LogoutAllBackends("anthropic", kring, file, &out); err != nil {
		t.Errorf("LogoutAllBackends: expected nil for ErrNotFound, got: %v", err)
	}
}

func TestLogoutAllBackends_NilFileBE(t *testing.T) {
	// file backend is nil (e.g. construction failed) — only keyring is swept.
	kring := &stubService{}

	var out bytes.Buffer
	if err := LogoutAllBackends("anthropic", kring, nil, &out); err != nil {
		t.Fatalf("LogoutAllBackends with nil file: %v", err)
	}
	if len(kring.calls) != 1 {
		t.Errorf("keyring: expected 1 Delete, got %d", len(kring.calls))
	}
}

func TestLogoutAllBackends_UnknownProvider(t *testing.T) {
	var out bytes.Buffer
	err := runLogout("not_a_provider", &stubService{}, &stubService{}, &out)
	if err == nil {
		t.Error("expected error for unknown provider in logout")
	}
	if !strings.Contains(err.Error(), "알 수 없는 provider") {
		t.Errorf("unexpected error: %q", err.Error())
	}
}

func TestLogoutAllBackends_BackendError(t *testing.T) {
	// Backend returns a non-NotFound error — should be surfaced.
	kring := &stubService{deleteErr: errors.New("keyring io error")}
	file := &stubService{}

	var out bytes.Buffer
	err := LogoutAllBackends("anthropic", kring, file, &out)
	if err == nil {
		t.Error("expected error when keyring Delete fails with non-NotFound error")
	}
}

// ---------------------------------------------------------------------------
// mink login cobra command integration
// ---------------------------------------------------------------------------

func TestLoginCommandUnknownProvider(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe("dummy-key-value")

	loginCmd := newLoginCommandWithServices(svcs, in)
	loginCmd.SetArgs([]string{"invalid_provider_id"})
	var out bytes.Buffer
	loginCmd.SetOut(&out)

	err := loginCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

func TestLogoutCommandSweepsBoth(t *testing.T) {
	kring := &stubService{}
	file := &stubService{}
	svcs := &loginServices{
		primary:   kring,
		keyringBE: kring,
		fileBE:    file,
	}

	loginCmd := newLoginCommandWithServices(svcs, nil)
	loginCmd.SetArgs([]string{"logout", "anthropic"})
	var out bytes.Buffer
	loginCmd.SetOut(&out)

	if err := loginCmd.Execute(); err != nil {
		t.Fatalf("logout anthropic: %v", err)
	}

	if len(kring.calls) != 1 || kring.calls[0].method != "Delete" {
		t.Errorf("keyring: expected 1 Delete, got %v", kring.calls)
	}
	if len(file.calls) != 1 || file.calls[0].method != "Delete" {
		t.Errorf("file: expected 1 Delete, got %v", file.calls)
	}
}

// ---------------------------------------------------------------------------
// Store error propagation
// ---------------------------------------------------------------------------

func TestLoginAPIKey_StoreError(t *testing.T) {
	storeErr := errors.New("keyring locked")
	primary := &stubService{storeErr: storeErr}
	svcs := makeSvcs(primary, &stubService{}, nil)
	in := pipe("test-fake-key-value")

	var out bytes.Buffer
	err := runLogin(t.Context(), "anthropic", svcs.primary, newInputReader(in), &out)
	if err == nil {
		t.Error("expected Store error to be propagated")
	}
	if !strings.Contains(err.Error(), "저장 실패") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
	if !errors.Is(err, storeErr) {
		t.Errorf("Store error not wrapped in returned error")
	}
}

// ---------------------------------------------------------------------------
// Output content checks
// ---------------------------------------------------------------------------

func TestLoginAPIKey_OutputContainsMasked(t *testing.T) {
	primary := &stubService{}
	svcs := makeSvcs(primary, &stubService{}, nil)
	fakeKey := "test-fake-verylongkey9999"
	in := pipe(fakeKey)

	var out bytes.Buffer
	if err := runLogin(t.Context(), "anthropic", svcs.primary, newInputReader(in), &out); err != nil {
		t.Fatalf("runLogin: %v", err)
	}
	// Output should include masked representation (***LAST4 format).
	output := out.String()
	if strings.Contains(output, fakeKey) {
		t.Errorf("output must not contain plaintext credential; got: %q", output)
	}
	if !strings.Contains(output, "***") {
		t.Errorf("output should contain masked representation (***); got: %q", output)
	}
}

// ---------------------------------------------------------------------------
// supportedProvidersList
// ---------------------------------------------------------------------------

func TestSupportedProvidersList_Count(t *testing.T) {
	list := supportedProvidersList()
	if len(list) != 8 {
		t.Errorf("expected 8 providers, got %d: %v", len(list), list)
	}
	// Verify all known provider IDs appear.
	for _, p := range []string{"anthropic", "deepseek", "openai_gpt", "zai_glm",
		"codex", "telegram_bot", "slack", "discord"} {
		found := false
		for _, s := range list {
			if s == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("provider %q not in supportedProvidersList", p)
		}
	}
}

// ---------------------------------------------------------------------------
// inputReader fallback path
// ---------------------------------------------------------------------------

func TestInputReaderFallback(t *testing.T) {
	// Verify that a non-*os.File reader (bytes.Buffer) is read correctly.
	buf := bytes.NewBufferString("my-secret-value\n")
	ir := newInputReader(buf)
	val, err := ir.readPassword("Prompt: ")
	if err != nil {
		t.Fatalf("readPassword: %v", err)
	}
	if val != "my-secret-value" {
		t.Errorf("got %q, want %q", val, "my-secret-value")
	}
}

func TestInputReaderFallback_EmptyLine(t *testing.T) {
	buf := bytes.NewBufferString("\n")
	ir := newInputReader(buf)
	val, err := ir.readPassword("Prompt: ")
	if err != nil {
		t.Fatalf("readPassword: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

// ---------------------------------------------------------------------------
// openBrowserWith — smoke test (no real browser spawned)
// ---------------------------------------------------------------------------

func TestOpenBrowserWith_NonBlocking(t *testing.T) {
	// Invoke with a well-known no-op binary so the function does not panic
	// or block.  On POSIX "true" exits immediately with code 0.
	// The test only checks that the call returns without hanging.
	openBrowserWith("true", "about:blank")
}
