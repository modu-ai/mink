// SPEC-GOOSE-CLI-001 Phase B4 — ConnectToolRegistry tests.
package commands

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/cli/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeToolClient struct {
	listFn func(ctx context.Context) ([]transport.ToolDescriptor, error)
}

func (f *fakeToolClient) ListTools(ctx context.Context) ([]transport.ToolDescriptor, error) {
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return nil, nil
}

func newRegistryWithFake(client *fakeToolClient) *ConnectToolRegistry {
	r := NewConnectToolRegistry("127.0.0.1:9005")
	r.clientOverride = client
	return r
}

// RED #9 (success): ListTools projects ToolDescriptor → ToolInfo and drops
// Source/ServerID.
func TestConnectToolRegistry_ListTools_Success(t *testing.T) {
	t.Parallel()

	r := newRegistryWithFake(&fakeToolClient{
		listFn: func(_ context.Context) ([]transport.ToolDescriptor, error) {
			return []transport.ToolDescriptor{
				{Name: "fs.read", Description: "Read file", Source: "builtin", ServerID: ""},
				{Name: "shell.exec", Description: "Run shell", Source: "mcp", ServerID: "srv-1"},
			}, nil
		},
	})

	got, err := r.ListTools()
	require.NoError(t, err)
	assert.Equal(t, []ToolInfo{
		{Name: "fs.read", Description: "Read file"},
		{Name: "shell.exec", Description: "Run shell"},
	}, got)
}

// RED #9 (empty): ListTools returns an empty slice, never nil for missing.
func TestConnectToolRegistry_ListTools_Empty(t *testing.T) {
	t.Parallel()

	r := newRegistryWithFake(&fakeToolClient{
		listFn: func(_ context.Context) ([]transport.ToolDescriptor, error) {
			return []transport.ToolDescriptor{}, nil
		},
	})

	got, err := r.ListTools()
	require.NoError(t, err)
	assert.NotNil(t, got, "must return [] not nil")
	assert.Len(t, got, 0)
}

// ListTools wraps RPC errors with a "list tools" prefix.
func TestConnectToolRegistry_ListTools_RPCError(t *testing.T) {
	t.Parallel()

	r := newRegistryWithFake(&fakeToolClient{
		listFn: func(_ context.Context) ([]transport.ToolDescriptor, error) {
			return nil, errors.New("server down")
		},
	})

	_, err := r.ListTools()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "list tools"), "got: %v", err)
}

// RED #10: StaticToolRegistry characterization — interface compatibility
// must continue to hold so existing tool-list tests remain green.
func TestStaticToolRegistry_StillWorks(t *testing.T) {
	t.Parallel()

	registry := NewStaticToolRegistry()

	got, err := registry.ListTools()
	require.NoError(t, err)
	assert.NotEmpty(t, got, "static registry must surface its hard-coded entries")
	for _, info := range got {
		assert.NotEmpty(t, info.Name, "every static tool must carry a name")
	}
}

func TestNewConnectToolRegistry_Defaults(t *testing.T) {
	t.Parallel()

	r := NewConnectToolRegistry("daemon:9005")
	assert.Equal(t, "daemon:9005", r.daemonAddr)
	assert.NotNil(t, r.newClient)
	assert.Equal(t, connectToolDefaultTimeout, r.timeout)
}
