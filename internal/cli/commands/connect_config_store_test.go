// SPEC-GOOSE-CLI-001 Phase B3 — ConnectConfigStore tests.
package commands

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConfigClient implements connectConfigClient with scriptable behavior.
type fakeConfigClient struct {
	getFn  func(ctx context.Context, key string) (string, bool, error)
	setFn  func(ctx context.Context, key, value string) error
	listFn func(ctx context.Context, prefix string) (map[string]string, error)
}

func (f *fakeConfigClient) GetConfig(ctx context.Context, key string) (string, bool, error) {
	if f.getFn != nil {
		return f.getFn(ctx, key)
	}
	return "", false, nil
}

func (f *fakeConfigClient) SetConfig(ctx context.Context, key, value string) error {
	if f.setFn != nil {
		return f.setFn(ctx, key, value)
	}
	return nil
}

func (f *fakeConfigClient) ListConfig(ctx context.Context, prefix string) (map[string]string, error) {
	if f.listFn != nil {
		return f.listFn(ctx, prefix)
	}
	return map[string]string{}, nil
}

func newStoreWithFake(client *fakeConfigClient) *ConnectConfigStore {
	store := NewConnectConfigStore("127.0.0.1:9005")
	store.clientOverride = client
	return store
}

// RED #6 (found): Get returns the stored value for an existing key.
func TestConnectConfigStore_Get_Found(t *testing.T) {
	t.Parallel()

	store := newStoreWithFake(&fakeConfigClient{
		getFn: func(_ context.Context, key string) (string, bool, error) {
			if key != "alias.openai" {
				t.Errorf("unexpected key: %q", key)
			}
			return "sk-test", true, nil
		},
	})

	got, err := store.Get("alias.openai")
	require.NoError(t, err)
	assert.Equal(t, "sk-test", got)
}

// RED #6 (not found): Get maps exists=false to ErrConfigKeyNotFound.
func TestConnectConfigStore_Get_NotFound(t *testing.T) {
	t.Parallel()

	store := newStoreWithFake(&fakeConfigClient{
		getFn: func(_ context.Context, _ string) (string, bool, error) {
			return "", false, nil
		},
	})

	_, err := store.Get("missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConfigKeyNotFound)
}

// Get wraps RPC errors with a "get config" prefix.
func TestConnectConfigStore_Get_RPCError(t *testing.T) {
	t.Parallel()

	store := newStoreWithFake(&fakeConfigClient{
		getFn: func(_ context.Context, _ string) (string, bool, error) {
			return "", false, errors.New("boom")
		},
	})

	_, err := store.Get("any")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "get config"), "got: %v", err)
}

// RED #7 (set): Set forwards the key/value to the underlying client.
func TestConnectConfigStore_Set(t *testing.T) {
	t.Parallel()

	var capturedKey, capturedValue string
	store := newStoreWithFake(&fakeConfigClient{
		setFn: func(_ context.Context, key, value string) error {
			capturedKey, capturedValue = key, value
			return nil
		},
	})

	require.NoError(t, store.Set("env", "prod"))
	assert.Equal(t, "env", capturedKey)
	assert.Equal(t, "prod", capturedValue)
}

func TestConnectConfigStore_Set_RPCError(t *testing.T) {
	t.Parallel()

	store := newStoreWithFake(&fakeConfigClient{
		setFn: func(_ context.Context, _, _ string) error {
			return errors.New("denied")
		},
	})

	err := store.Set("k", "v")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "set config"), "got: %v", err)
}

// RED #7 (list): List passes an empty prefix and returns the map verbatim.
func TestConnectConfigStore_ListConfig_Prefix(t *testing.T) {
	t.Parallel()

	var capturedPrefix string
	store := newStoreWithFake(&fakeConfigClient{
		listFn: func(_ context.Context, prefix string) (map[string]string, error) {
			capturedPrefix = prefix
			return map[string]string{"alias.openai": "sk", "env": "prod"}, nil
		},
	})

	got, err := store.List()
	require.NoError(t, err)
	assert.Equal(t, "", capturedPrefix, "ConfigStore.List must request the full set (empty prefix)")
	assert.Equal(t, map[string]string{"alias.openai": "sk", "env": "prod"}, got)
}

func TestConnectConfigStore_List_RPCError(t *testing.T) {
	t.Parallel()

	store := newStoreWithFake(&fakeConfigClient{
		listFn: func(_ context.Context, _ string) (map[string]string, error) {
			return nil, errors.New("server down")
		},
	})

	_, err := store.List()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "list config"), "got: %v", err)
}

// RED #8: MemoryConfigStore characterization — interface compatibility
// must continue to hold so existing tests remain green.
func TestMemoryConfigStore_StillWorks(t *testing.T) {
	t.Parallel()

	store := NewMemoryConfigStore()

	require.NoError(t, store.Set("env", "prod"))

	got, err := store.Get("env")
	require.NoError(t, err)
	assert.Equal(t, "prod", got)

	_, err = store.Get("missing")
	assert.ErrorIs(t, err, ErrConfigKeyNotFound)

	all, err := store.List()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"env": "prod"}, all)
}

// NewConnectConfigStore wires the default timeout and factory.
func TestNewConnectConfigStore_Defaults(t *testing.T) {
	t.Parallel()

	store := NewConnectConfigStore("localhost:9005")
	assert.Equal(t, "localhost:9005", store.daemonAddr)
	assert.NotNil(t, store.newClient)
	assert.Equal(t, connectConfigDefaultTimeout, store.timeout)
}

// effectiveTimeout falls back to the constant when zero is configured.
func TestConnectConfigStore_EffectiveTimeoutFallback(t *testing.T) {
	t.Parallel()

	store := &ConnectConfigStore{}
	assert.Equal(t, connectConfigDefaultTimeout, store.effectiveTimeout())
}
