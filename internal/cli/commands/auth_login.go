// Package commands — mink login / mink logout subcommands.
//
// "mink login {provider}" prompts for credentials interactively (with echo
// disabled for secret fields) and persists them via the supplied
// credential.Service.
//
// "mink logout {provider}" calls Delete on both keyring and file backends to
// ensure a complete sweep regardless of which backend was used at registration
// time (ED-3, idempotent).
//
// Provider → credential mapping (8 providers, research.md §4.2):
//
//	anthropic, deepseek, openai_gpt, zai_glm → APIKey  (single masked input)
//	codex                                     → OAuthToken (PKCE browser flow)
//	telegram_bot                              → BotToken (single masked input)
//	slack                                     → SlackCombo (2 masked inputs)
//	discord                                   → DiscordCombo (2 masked inputs)
//
// Masked input uses golang.org/x/term.ReadPassword so that the secret never
// appears on screen (UN-1).
//
// Korean-language user messages per language.yaml (conversation_language: ko).
// English comments per language.yaml (code_comments: en).
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UB-4, ED-3, OP-3, AC-CR-012, AC-CR-015, AC-CR-032, T-016)
package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"

	"github.com/modu-ai/mink/internal/auth/credential"
	filebe "github.com/modu-ai/mink/internal/auth/file"
	"github.com/modu-ai/mink/internal/auth/keyring"
	"github.com/modu-ai/mink/internal/auth/oauth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// fileNewBackend constructs the file backend using its default path.
func fileNewBackend() (*filebe.Backend, error) {
	return filebe.NewBackend()
}

// loginServices bundles the credential.Service implementations used by the
// login / logout commands.  Both fields are required for logout (sweep both
// backends idempotently per ED-3).
type loginServices struct {
	// primary is the Service used by login to persist credentials.
	// Typically the Dispatcher, but a stub in tests.
	primary credential.Service

	// keyringBE is the keyring-only backend swept during logout.
	keyringBE credential.Service

	// fileBE is the file-only backend swept during logout.
	fileBE credential.Service
}

// inputReader abstracts stdin so tests can inject a pipe instead of real TTY
// input.  The zero value uses os.Stdin.
type inputReader struct {
	r io.Reader
}

// newInputReader creates an inputReader backed by the provided reader.
// If r is nil, os.Stdin is used.
func newInputReader(r io.Reader) inputReader {
	if r == nil {
		r = os.Stdin
	}
	return inputReader{r: r}
}

// readPassword reads a masked secret from the reader.
//
// When the reader is the real os.Stdin file descriptor, golang.org/x/term is
// used to disable terminal echo.  Otherwise (e.g. in tests where a pipe is
// passed) the raw reader is read directly so that tests can pipe canned input
// without requiring a real TTY.
func (ir inputReader) readPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Use term.ReadPassword only when stdin is the real OS file descriptor.
	// pipes and bytes.Buffer readers (used in tests) do not have a valid
	// file descriptor and would cause term.ReadPassword to fail.
	if f, ok := ir.r.(*os.File); ok && f == os.Stdin {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Println() // newline after hidden input
		if err != nil {
			return "", fmt.Errorf("masked input read: %w", err)
		}
		return string(b), nil
	}

	// Fallback: plain-text read from any io.Reader (test path).
	var buf []byte
	single := make([]byte, 1)
	for {
		n, err := ir.r.Read(single)
		if n > 0 {
			if single[0] == '\n' {
				break
			}
			buf = append(buf, single[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("input read: %w", err)
		}
	}
	fmt.Println()
	return string(buf), nil
}

// supportedProviders returns a human-readable comma-separated list of all 8
// supported provider IDs in deterministic order.
func supportedProvidersList() []string {
	// deterministic ordering for error messages
	return []string{
		"anthropic", "deepseek", "openai_gpt", "zai_glm",
		"codex",
		"telegram_bot",
		"slack",
		"discord",
	}
}

// NewLoginCommand returns the "mink login {provider}" cobra.Command.
//
// The command resolves a default loginServices (keyring primary + both
// backends for logout sweep) at execution time.  Tests use
// newLoginCommandWithServices to inject stubs.
//
// @MX:ANCHOR: [AUTO] NewLoginCommand is the public entry point wired into
// rootcmd.go and referenced by integration tests (fan_in >= 3).
// @MX:REASON: Wiring change here cascades to rootcmd.go, integration tests,
// and the user-facing CLI contract.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (T-016, AC-CR-012, AC-CR-015)
func NewLoginCommand() *cobra.Command {
	return newLoginCommandWithServices(nil, nil)
}

// newLoginCommandWithServices creates the login command with optional service
// overrides.  When svcs is nil, default backends are constructed at RunE time.
// When in is nil, os.Stdin is used.
func newLoginCommandWithServices(svcs *loginServices, in io.Reader) *cobra.Command {
	ir := newInputReader(in)

	cmd := &cobra.Command{
		Use:   "login {provider}",
		Short: "저장소에 credential을 등록합니다 (mink login {provider})",
		Long: `credential을 대화형으로 입력하여 OS keyring (또는 평문 fallback) 에 저장합니다.

지원 provider: anthropic, deepseek, openai_gpt, zai_glm, codex, telegram_bot, slack, discord

  mink login anthropic     — Anthropic API key 등록
  mink login codex         — OpenAI Codex OAuth 2.1 + PKCE 브라우저 인증
  mink login telegram_bot  — Telegram Bot Token 등록
  mink login slack         — Slack Signing Secret + Bot Token 등록
  mink login discord       — Discord Public Key + Bot Token 등록`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]
			svc, err := resolveLoginServices(svcs)
			if err != nil {
				return err
			}
			return runLogin(cmd.Context(), provider, svc.primary, ir, cmd.OutOrStdout())
		},
	}

	logoutCmd := &cobra.Command{
		Use:   "logout {provider}",
		Short: "저장된 credential을 삭제합니다 (mink logout {provider})",
		Long: `keyring 및 파일 backend 양쪽에서 credential을 삭제합니다 (idempotent).

지원 provider: anthropic, deepseek, openai_gpt, zai_glm, codex, telegram_bot, slack, discord`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]
			svc, err := resolveLoginServices(svcs)
			if err != nil {
				return err
			}
			return runLogout(provider, svc.keyringBE, svc.fileBE, cmd.OutOrStdout())
		},
	}

	cmd.AddCommand(logoutCmd)
	return cmd
}

// resolveLoginServices constructs the default loginServices when svcs is nil.
// This is intentionally deferred to RunE time so that tests can stub backends
// without triggering real keyring probes during command construction.
func resolveLoginServices(svcs *loginServices) (*loginServices, error) {
	if svcs != nil {
		return svcs, nil
	}
	kb := keyring.NewBackend()

	// File backend construction failure is non-fatal; we proceed with
	// keyring-only and skip the file sweep during logout.
	fb, _ := fileNewBackend()

	var fileSvc credential.Service
	if fb != nil {
		fileSvc = fb
	}

	return &loginServices{
		// Primary for login defaults to keyring. In production the App context
		// would supply the Dispatcher; M4 CLI wiring uses keyring as the default
		// so that the command works stand-alone without daemon access.
		primary:   kb,
		keyringBE: kb,
		fileBE:    fileSvc,
	}, nil
}

// runLogin dispatches to the provider-specific login handler.
func runLogin(ctx context.Context, provider string, svc credential.Service, ir inputReader, out io.Writer) error {
	all := supportedProvidersList()
	if !slices.Contains(all, provider) {
		return fmt.Errorf(
			"알 수 없는 provider: %q\n지원 목록: %v",
			provider, all,
		)
	}

	switch provider {
	case "codex":
		return loginCodex(ctx, svc, out)
	case "telegram_bot":
		return loginBotToken(provider, svc, ir, out)
	case "slack":
		return loginSlack(svc, ir, out)
	case "discord":
		return loginDiscord(svc, ir, out)
	default:
		// anthropic, deepseek, openai_gpt, zai_glm → APIKey
		return loginAPIKey(provider, svc, ir, out)
	}
}

// loginAPIKey prompts for a single API key and stores an APIKey credential.
func loginAPIKey(provider string, svc credential.Service, ir inputReader, out io.Writer) error {
	fmt.Fprintf(out, "[%s] API Key를 입력하세요 (입력값은 화면에 표시되지 않습니다):\n", provider)
	val, err := ir.readPassword("API Key: ")
	if err != nil {
		return fmt.Errorf("입력 오류: %w", err)
	}
	if val == "" {
		return fmt.Errorf("API key는 비워둘 수 없습니다")
	}
	cred := credential.APIKey{Value: val}
	if err := svc.Store(provider, cred); err != nil {
		return fmt.Errorf("%s credential 저장 실패: %w", provider, err)
	}
	fmt.Fprintf(out, "%s credential이 저장되었습니다 (%s).\n", provider, cred.MaskedString())
	return nil
}

// loginBotToken prompts for a Telegram bot token and stores a BotToken credential.
func loginBotToken(provider string, svc credential.Service, ir inputReader, out io.Writer) error {
	fmt.Fprintf(out, "[%s] Bot Token을 입력하세요 (입력값은 화면에 표시되지 않습니다):\n", provider)
	tok, err := ir.readPassword("Bot Token: ")
	if err != nil {
		return fmt.Errorf("입력 오류: %w", err)
	}
	if tok == "" {
		return fmt.Errorf("bot token은 비워둘 수 없습니다")
	}
	cred := credential.BotToken{Provider: provider, Token: tok}
	if err := svc.Store(provider, cred); err != nil {
		return fmt.Errorf("%s credential 저장 실패: %w", provider, err)
	}
	fmt.Fprintf(out, "%s credential이 저장되었습니다 (%s).\n", provider, cred.MaskedString())
	return nil
}

// loginSlack prompts for a Slack signing secret and bot token, then stores a
// SlackCombo credential.
func loginSlack(svc credential.Service, ir inputReader, out io.Writer) error {
	fmt.Fprintln(out, "[slack] Slack credential을 입력하세요 (입력값은 화면에 표시되지 않습니다):")
	secret, err := ir.readPassword("Signing Secret: ")
	if err != nil {
		return fmt.Errorf("입력 오류: %w", err)
	}
	botToken, err := ir.readPassword("Bot Token: ")
	if err != nil {
		return fmt.Errorf("입력 오류: %w", err)
	}
	if secret == "" || botToken == "" {
		return fmt.Errorf("signing_secret 과 bot_token은 비워둘 수 없습니다")
	}
	cred := credential.SlackCombo{SigningSecret: secret, BotToken: botToken}
	if err := svc.Store("slack", cred); err != nil {
		return fmt.Errorf("slack credential 저장 실패: %w", err)
	}
	fmt.Fprintf(out, "slack credential이 저장되었습니다 (%s).\n", cred.MaskedString())
	return nil
}

// loginDiscord prompts for a Discord Ed25519 public key and bot token, then
// stores a DiscordCombo credential.
func loginDiscord(svc credential.Service, ir inputReader, out io.Writer) error {
	fmt.Fprintln(out, "[discord] Discord credential을 입력하세요 (입력값은 화면에 표시되지 않습니다):")
	pubKey, err := ir.readPassword("Public Key (64자 hex): ")
	if err != nil {
		return fmt.Errorf("입력 오류: %w", err)
	}
	botToken, err := ir.readPassword("Bot Token: ")
	if err != nil {
		return fmt.Errorf("입력 오류: %w", err)
	}
	if pubKey == "" || botToken == "" {
		return fmt.Errorf("public_key 와 bot_token은 비워둘 수 없습니다")
	}
	cred := credential.DiscordCombo{PublicKey: pubKey, BotToken: botToken}
	if err := svc.Store("discord", cred); err != nil {
		return fmt.Errorf("discord credential 저장 실패: %w", err)
	}
	fmt.Fprintf(out, "discord credential이 저장되었습니다 (%s).\n", cred.MaskedString())
	return nil
}

// loginCodex runs the OAuth 2.1 + PKCE flow for the Codex provider.
//
// Steps:
//  1. Call oauth.BeginAuthorization to get an auth URL and a local callback listener.
//  2. Print the URL and attempt to open it in the system browser.
//  3. Call oauth.CompleteAuthorization to wait for the redirect callback.
//  4. Store the resulting OAuthToken credential.
func loginCodex(ctx context.Context, svc credential.Service, out io.Writer) error {
	fmt.Fprintln(out, "[codex] OpenAI Codex OAuth 2.1 인증을 시작합니다...")

	pending, err := oauth.BeginAuthorization(ctx)
	if err != nil {
		return fmt.Errorf("codex: OAuth 인증 시작 실패: %w", err)
	}

	fmt.Fprintln(out, "\n아래 URL을 브라우저에서 열어 인증을 완료하세요:")
	fmt.Fprintln(out, pending.AuthURL)
	fmt.Fprintln(out, "")

	// Best-effort: attempt to open the browser automatically.
	openBrowser(pending.AuthURL)

	fmt.Fprintln(out, "인증 완료를 기다리는 중...")

	tok, err := oauth.CompleteAuthorization(ctx, pending)
	if err != nil {
		return fmt.Errorf("codex: OAuth 인증 완료 실패: %w", err)
	}

	if err := svc.Store("codex", tok); err != nil {
		return fmt.Errorf("codex credential 저장 실패: %w", err)
	}
	fmt.Fprintf(out, "codex credential이 저장되었습니다 (%s).\n", tok.MaskedString())
	return nil
}

// openBrowser attempts to open url in the system default browser.
// Failure is silently ignored — the user can always open the URL manually.
func openBrowser(url string) {
	var cmdName string
	switch runtime.GOOS {
	case "darwin":
		cmdName = "open"
	case "windows":
		cmdName = "cmd"
	default:
		cmdName = "xdg-open"
	}
	openBrowserWith(cmdName, url)
}

// openBrowserWith calls the named binary to open url.
// On Windows the cmd binary is invoked with /c start <url>.
// On other platforms the binary is invoked directly with url as the sole arg.
// Any error is silently discarded.
func openBrowserWith(cmdName, url string) {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command(cmdName, "/c", "start", url)
	} else {
		c = exec.Command(cmdName, url)
	}
	_ = c.Start()
}

// runLogout removes the credential for provider from both the keyring and file
// backends (idempotent sweep, ED-3).  Errors from individual backends are
// collected and returned as a combined error; a "not found" result is treated
// as success per the idempotent delete contract.
func runLogout(provider string, keyringBE, fileBE credential.Service, out io.Writer) error {
	all := supportedProvidersList()
	if !slices.Contains(all, provider) {
		return fmt.Errorf(
			"알 수 없는 provider: %q\n지원 목록: %v",
			provider, all,
		)
	}

	return LogoutAllBackends(provider, keyringBE, fileBE, out)
}

// LogoutAllBackends deletes the credential for provider from both keyringBE
// and fileBE.  Either backend may be nil (e.g. when file backend construction
// failed); nil backends are skipped.
//
// The function is exported so that integration tests can call it directly
// without constructing a cobra.Command.
//
// @MX:ANCHOR: [AUTO] LogoutAllBackends is called by runLogout and integration
// tests (fan_in >= 3 expected with CLI and test suite growth).
// @MX:REASON: All logout paths funnel through this function; a behaviour
// change here must be reflected in tests and the CLI RunE.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (ED-3, AC-CR-015, T-016)
func LogoutAllBackends(provider string, keyringBE, fileBE credential.Service, out io.Writer) error {
	var errs []error

	if keyringBE != nil {
		if err := keyringBE.Delete(provider); err != nil && !errors.Is(err, credential.ErrNotFound) {
			errs = append(errs, fmt.Errorf("keyring backend 삭제 실패: %w", err))
		}
	}

	if fileBE != nil {
		if err := fileBE.Delete(provider); err != nil && !errors.Is(err, credential.ErrNotFound) {
			errs = append(errs, fmt.Errorf("file backend 삭제 실패: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s logout 실패: %v", provider, errs)
	}

	fmt.Fprintf(out, "%s credential이 삭제되었습니다.\n", provider)
	return nil
}
