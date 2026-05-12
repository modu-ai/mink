// Package credential — config-driven pool factory.
//
// SPEC-GOOSE-CREDPOOL-001 OI-06.
//
// NewPoolsFromConfig is the wiring point between SPEC-GOOSE-CONFIG-001
// (LLMConfig.Providers[*].Credentials) and the credential pool runtime.
// It builds one CredentialPool per provider that declares at least one
// credential source. Providers without credentials are silently skipped.
package credential

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/modu-ai/mink/internal/config"
	"go.uber.org/zap"
)

// NewPoolsFromConfig builds a CredentialPool per provider declared in
// cfg.LLM.Providers. Each pool's Source composes the provider's
// ProviderConfig.Credentials entries:
//
//   - "anthropic_claude_file" → AnthropicClaudeSource
//   - "openai_codex_file"     → OpenAICodexSource
//   - "nous_hermes_file"      → NousSource
//
// When a provider declares N > 1 credentials, the resulting Source is a
// multiSource that concatenates results from each individual source. When
// N == 1, the single source is wired directly to avoid the wrapper overhead.
//
// The default selection strategy is RoundRobinStrategy. Callers that need a
// different strategy must construct pools manually.
//
// Returns map[providerName]*CredentialPool. Providers with zero credentials
// produce no map entry. Missing vendor files inside a source are tolerated
// (per OI-05 rule 3); only schema validation errors and unexpected IO/parse
// failures abort. A pre-cancelled context propagates as context.Canceled.
//
// @MX:ANCHOR: [AUTO] Config-to-pool wiring entry point — invoked by the
// goosed bootstrap, future SPEC-GOOSE-ROUTER-001, and any CLI command that
// needs to inspect the live credential pools.
// @MX:REASON: All non-test code that materializes a CredentialPool from
// configured providers will route through this function (fan_in >= 3).
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001 OI-06
func NewPoolsFromConfig(ctx context.Context, cfg *config.Config, logger *zap.Logger) (map[string]*CredentialPool, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg == nil {
		return nil, fmt.Errorf("credential factory: nil config")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pools := make(map[string]*CredentialPool)

	for providerName, provider := range cfg.LLM.Providers {
		if len(provider.Credentials) == 0 {
			logger.Debug("credential factory: provider has no credentials, skipping",
				zap.String("provider", providerName))
			continue
		}

		source, err := buildProviderSource(providerName, provider.Credentials)
		if err != nil {
			return nil, err
		}

		pool, err := New(source, NewRoundRobinStrategy())
		if err != nil {
			return nil, fmt.Errorf("credential factory: build pool for %q: %w", providerName, err)
		}
		pools[providerName] = pool
		logger.Debug("credential factory: built pool",
			zap.String("provider", providerName),
			zap.Int("credentials", len(provider.Credentials)))
	}

	return pools, nil
}

// buildProviderSource composes a CredentialSource for a single provider.
// When entries has length 1, the source is returned directly; otherwise a
// multiSource is wrapped around the individual sources to preserve the
// declared order.
func buildProviderSource(providerName string, entries []config.CredentialConfig) (CredentialSource, error) {
	sources := make([]CredentialSource, 0, len(entries))
	for idx, entry := range entries {
		src, err := buildCredentialSource(entry)
		if err != nil {
			return nil, fmt.Errorf("credential factory: provider %q credentials[%d]: %w",
				providerName, idx, err)
		}
		sources = append(sources, src)
	}

	if len(sources) == 1 {
		return sources[0], nil
	}
	return &multiSource{sources: sources}, nil
}

// buildCredentialSource dispatches a single CredentialConfig entry to the
// matching vendor source constructor. Unknown Type values are rejected
// here as a defense-in-depth check on top of config.Validate().
func buildCredentialSource(entry config.CredentialConfig) (CredentialSource, error) {
	switch entry.Type {
	case "anthropic_claude_file":
		path := entry.Path
		if path == "" {
			path = DefaultAnthropicClaudeCredentialsPath()
		}
		path = expandUserPath(path)
		src := NewAnthropicClaudeSource(path)
		if entry.KeyringRef != "" {
			src = src.WithAnthropicKeyringRef(entry.KeyringRef)
		}
		return src, nil

	case "openai_codex_file":
		path := entry.Path
		if path == "" {
			path = DefaultOpenAICodexCredentialsPath()
		}
		path = expandUserPath(path)
		src := NewOpenAICodexSource(path)
		if entry.KeyringRef != "" {
			src = src.WithOpenAIKeyringRef(entry.KeyringRef)
		}
		return src, nil

	case "nous_hermes_file":
		path := entry.Path
		if path == "" {
			path = DefaultNousCredentialsPath()
		}
		path = expandUserPath(path)
		src := NewNousSource(path)
		if entry.KeyringRef != "" {
			src = src.WithNousKeyringRef(entry.KeyringRef)
		}
		return src, nil

	default:
		return nil, fmt.Errorf("unknown credential source kind %q", entry.Type)
	}
}

// expandUserPath returns path unchanged when it is already absolute; this is
// a placeholder for future ~/ expansion if we decide to support it. The
// CONFIG-001 layer is expected to deliver fully-resolved paths today.
func expandUserPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return path
}

// multiSource concatenates results from multiple CredentialSource
// implementations in declared order. It is package-private; callers obtain
// it indirectly via NewPoolsFromConfig.
type multiSource struct {
	sources []CredentialSource
}

// Load invokes each inner source in order and concatenates the results.
// Any inner error aborts the load (so a malformed file is loud, not silent).
// Context cancellation is checked between sources to honor REQ-CREDPOOL-017
// for the aggregate as well.
func (m *multiSource) Load(ctx context.Context) ([]*PooledCredential, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]*PooledCredential, 0, len(m.sources))
	for _, s := range m.sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		creds, err := s.Load(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, creds...)
	}
	return out, nil
}
