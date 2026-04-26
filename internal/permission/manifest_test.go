package permission

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequiresParser_FourCategories_PartialSuccess는 AC-PE-001을 검증한다.
// fixture (a): 정상 4-카테고리 + 미지 카테고리 1종
// Covers: REQ-PE-002, REQ-PE-010, REQ-PE-018
func TestRequiresParser_FourCategories_PartialSuccess(t *testing.T) {
	t.Parallel()

	p := &RequiresParser{}

	t.Run("fixture_a_4categories_plus_unknown", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"net":     []any{"api.openai.com"},
			"fs_read": []any{"./.goose/**"},
			"exec":    []any{"git"},
			"misc":    []any{"something"},
		}
		m, errs := p.Parse(raw)

		assert.Equal(t, []string{"api.openai.com"}, m.NetHosts)
		assert.Equal(t, []string{"./.goose/**"}, m.FSReadPaths)
		assert.Equal(t, []string{"git"}, m.ExecBinaries)
		assert.Nil(t, m.FSWritePaths, "fs_write not declared, must be nil")

		require.Len(t, errs, 1)
		var unknownErr ErrUnknownCapability
		require.True(t, errors.As(errs[0], &unknownErr))
		assert.Equal(t, "misc", unknownErr.Key)
	})

	t.Run("fixture_b_scalar_value_rejected", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"net": "api.openai.com", // 스칼라 — 거부
		}
		m, errs := p.Parse(raw)

		assert.Nil(t, m.NetHosts, "scalar must not be coerced to []string")
		require.Len(t, errs, 1)
		var shapeErr ErrInvalidScopeShape
		require.True(t, errors.As(errs[0], &shapeErr))
		assert.Equal(t, "net", shapeErr.Category)
	})

	t.Run("fixture_c_nested_requires_rejected", func(t *testing.T) {
		t.Parallel()
		// 중첩 requires: 구조
		raw := map[string]any{
			"requires": map[string]any{
				"net": []any{"api.openai.com"},
			},
		}
		m, errs := p.Parse(raw)

		assert.Nil(t, m.NetHosts, "nested: all fields must be nil")
		assert.Nil(t, m.FSReadPaths)
		assert.Nil(t, m.FSWritePaths)
		assert.Nil(t, m.ExecBinaries)
		require.Len(t, errs, 1)
		var shapeErr ErrInvalidScopeShape
		require.True(t, errors.As(errs[0], &shapeErr))
		assert.True(t, shapeErr.Nested, "must have nested:true annotation")
	})
}

// TestRequiresParser_AllFourCategories는 4가지 카테고리 모두 올바르게 파싱됨을 검증한다.
func TestRequiresParser_AllFourCategories(t *testing.T) {
	t.Parallel()

	p := &RequiresParser{}
	raw := map[string]any{
		"net":      []any{"api.openai.com", "*.anthropic.com"},
		"fs_read":  []any{"./.goose/**"},
		"fs_write": []any{"./.goose/memory/**"},
		"exec":     []any{"git", "go"},
	}
	m, errs := p.Parse(raw)

	assert.Empty(t, errs)
	assert.Equal(t, []string{"api.openai.com", "*.anthropic.com"}, m.NetHosts)
	assert.Equal(t, []string{"./.goose/**"}, m.FSReadPaths)
	assert.Equal(t, []string{"./.goose/memory/**"}, m.FSWritePaths)
	assert.Equal(t, []string{"git", "go"}, m.ExecBinaries)
}

// TestRequiresParser_EmptyRaw는 빈 map을 파싱하면 오류 없이 빈 Manifest를 반환함을 검증한다.
func TestRequiresParser_EmptyRaw(t *testing.T) {
	t.Parallel()

	p := &RequiresParser{}
	m, errs := p.Parse(map[string]any{})

	assert.Empty(t, errs)
	assert.Nil(t, m.NetHosts)
	assert.Nil(t, m.FSReadPaths)
	assert.Nil(t, m.FSWritePaths)
	assert.Nil(t, m.ExecBinaries)
}

// TestRequiresParser_MultipleErrors는 여러 오류가 동시에 누적됨을 검증한다.
func TestRequiresParser_MultipleErrors(t *testing.T) {
	t.Parallel()

	p := &RequiresParser{}
	raw := map[string]any{
		"net":     "scalar",   // ErrInvalidScopeShape
		"exec":    "scalar",   // ErrInvalidScopeShape
		"unknown": []any{"x"}, // ErrUnknownCapability
	}
	_, errs := p.Parse(raw)
	assert.Len(t, errs, 3)
}

// TestManifest_Declares는 Manifest.Declares가 정확히 선언된 scope만 true를 반환함을 검증한다.
func TestManifest_Declares(t *testing.T) {
	t.Parallel()

	m := &Manifest{
		NetHosts:    []string{"api.openai.com"},
		FSReadPaths: []string{"./.goose/**"},
	}

	assert.True(t, m.Declares(CapNet, "api.openai.com"))
	assert.False(t, m.Declares(CapNet, "evil.com"))
	assert.True(t, m.Declares(CapFSRead, "./.goose/**"))
	assert.False(t, m.Declares(CapFSWrite, "./.goose/**"))
	assert.False(t, m.Declares(CapExec, "git"))
}

// TestCapability_String은 Capability.String()의 출력을 검증한다.
func TestCapability_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cap  Capability
		want string
	}{
		{CapNet, "net"},
		{CapFSRead, "fs_read"},
		{CapFSWrite, "fs_write"},
		{CapExec, "exec"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.cap.String())
	}
}

// TestCapabilityFromString은 문자열 → Capability 변환을 검증한다.
func TestCapabilityFromString(t *testing.T) {
	t.Parallel()

	got, ok := CapabilityFromString("net")
	assert.True(t, ok)
	assert.Equal(t, CapNet, got)

	_, ok = CapabilityFromString("invalid")
	assert.False(t, ok)
}
