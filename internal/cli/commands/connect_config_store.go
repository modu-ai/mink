// SPEC-GOOSE-CLI-001 Phase B3 — ConfigStore backed by ConnectClient.ConfigService.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/cli/transport"
)

// connectConfigClient captures the subset of *transport.ConnectClient that
// ConnectConfigStore depends on. Tests inject a fake by implementing this
// interface so the store can be exercised without a live daemon.
type connectConfigClient interface {
	GetConfig(ctx context.Context, key string) (string, bool, error)
	SetConfig(ctx context.Context, key, value string) error
	ListConfig(ctx context.Context, prefix string) (map[string]string, error)
}

// connectConfigDefaultTimeout bounds the per-call deadline applied when
// ConfigStore is invoked from ConfigStore.Get/Set/List, since the
// commands.ConfigStore interface is context-less.
const connectConfigDefaultTimeout = 5 * time.Second

// ConnectConfigStore satisfies ConfigStore by delegating to
// ConnectClient.ConfigService over the daemon. The legacy MemoryConfigStore
// remains the in-process fallback used by tests; this store talks to the
// real backend.
//
// @MX:ANCHOR ConnectConfigStore is the production ConfigStore, replacing
// the in-memory placeholder for the rootcmd-wired binary.
// @MX:REASON SPEC-GOOSE-CLI-001 Phase B3; fan_in == 1 today via rootcmd.
type ConnectConfigStore struct {
	daemonAddr string
	newClient  func(daemonURL string, opts ...transport.ConnectOption) (*transport.ConnectClient, error)
	timeout    time.Duration

	// clientOverride lets tests bypass ConnectClient construction and inject
	// a stub implementing connectConfigClient. When non-nil, newClient is
	// not called.
	clientOverride connectConfigClient
}

// NewConnectConfigStore returns a ConfigStore that issues Connect-protocol
// RPCs against the daemon at daemonAddr. The address may be supplied in
// either "host:port" or "http://host:port" form.
func NewConnectConfigStore(daemonAddr string) *ConnectConfigStore {
	return &ConnectConfigStore{
		daemonAddr: daemonAddr,
		newClient:  transport.NewConnectClient,
		timeout:    connectConfigDefaultTimeout,
	}
}

// Get retrieves a config value, mapping a missing key to ErrConfigKeyNotFound.
//
// @MX:NOTE ConnectClient.GetConfig returns (value, exists, err); the
// commands.ConfigStore interface uses an error sentinel instead, so the
// adapter translates exists=false → ErrConfigKeyNotFound.
func (s *ConnectConfigStore) Get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.effectiveTimeout())
	defer cancel()

	client, err := s.client()
	if err != nil {
		return "", err
	}

	value, exists, err := client.GetConfig(ctx, key)
	if err != nil {
		return "", fmt.Errorf("get config: %w", err)
	}
	if !exists {
		return "", ErrConfigKeyNotFound
	}
	return value, nil
}

// Set stores a config value.
func (s *ConnectConfigStore) Set(key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.effectiveTimeout())
	defer cancel()

	client, err := s.client()
	if err != nil {
		return err
	}

	if err := client.SetConfig(ctx, key, value); err != nil {
		return fmt.Errorf("set config: %w", err)
	}
	return nil
}

// List returns every config entry. The empty prefix retrieves all keys.
func (s *ConnectConfigStore) List() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.effectiveTimeout())
	defer cancel()

	client, err := s.client()
	if err != nil {
		return nil, err
	}

	entries, err := client.ListConfig(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list config: %w", err)
	}
	return entries, nil
}

// client returns the underlying connectConfigClient, preferring the test
// override when one has been installed.
func (s *ConnectConfigStore) client() (connectConfigClient, error) {
	if s.clientOverride != nil {
		return s.clientOverride, nil
	}
	c, err := s.newClient(transport.NormalizeDaemonURL(s.daemonAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	return c, nil
}

func (s *ConnectConfigStore) effectiveTimeout() time.Duration {
	if s.timeout > 0 {
		return s.timeout
	}
	return connectConfigDefaultTimeout
}
