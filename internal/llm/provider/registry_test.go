package provider_test

import (
	"context"
	"sync"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistry_Register_Get는 Register 후 Get이 올바르게 반환하는지 검증한다.
func TestRegistry_Register_Get(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()
	p := &stubProvider{}

	err := reg.Register(p)
	require.NoError(t, err)

	got, ok := reg.Get("stub")
	assert.True(t, ok)
	assert.Equal(t, p, got)
}

// TestRegistry_Get_NotFound는 미등록 provider 조회 시 false를 반환하는지 검증한다.
func TestRegistry_Get_NotFound(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()
	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

// TestRegistry_Register_DuplicateReturnsError는 같은 이름 중복 등록 시 에러를 검증한다.
func TestRegistry_Register_DuplicateReturnsError(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()
	p := &stubProvider{}

	require.NoError(t, reg.Register(p))
	err := reg.Register(p)
	assert.Error(t, err)
}

// TestRegistry_Names_AlphabeticallySorted는 Names()가 알파벳 순으로 정렬되는지 검증한다.
func TestRegistry_Names_AlphabeticallySorted(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	// 순서 없이 등록
	providers := []*namedProvider{
		{name: "zebra"},
		{name: "apple"},
		{name: "mango"},
	}
	for _, p := range providers {
		require.NoError(t, reg.Register(p))
	}

	names := reg.Names()
	assert.Equal(t, []string{"apple", "mango", "zebra"}, names)
}

// TestRegistry_Concurrent_RegisterLookup는 동시 등록/조회가 안전한지 검증한다.
func TestRegistry_Concurrent_RegisterLookup(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	var wg sync.WaitGroup
	// 10개 등록
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := &namedProvider{name: "provider-" + string(rune('a'+i))}
			_ = reg.Register(p)
		}(i)
	}
	// 10개 조회
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.Get("provider-a")
		}()
	}
	wg.Wait()
}

// TestRegistry_Len는 등록된 provider 수가 올바른지 검증한다.
func TestRegistry_Len(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()
	assert.Equal(t, 0, reg.Len())

	require.NoError(t, reg.Register(&namedProvider{name: "a"}))
	require.NoError(t, reg.Register(&namedProvider{name: "b"}))

	assert.Equal(t, 2, reg.Len())
}

// namedProvider는 이름을 지정할 수 있는 테스트용 Provider이다.
type namedProvider struct {
	name string
}

func (n *namedProvider) Name() string { return n.name }

func (n *namedProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

func (n *namedProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{}, nil
}

func (n *namedProvider) Stream(_ context.Context, _ provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	ch := make(chan message.StreamEvent)
	close(ch)
	return ch, nil
}
