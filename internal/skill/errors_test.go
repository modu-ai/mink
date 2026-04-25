package skill

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrUnsafeFrontmatterProperty_Error는 에러 메시지 형식을 검증한다.
func TestErrUnsafeFrontmatterProperty_Error(t *testing.T) {
	err := ErrUnsafeFrontmatterProperty{Property: "frobnicate"}
	msg := err.Error()
	assert.Contains(t, msg, "frobnicate")
	assert.Contains(t, msg, "SAFE_SKILL_PROPERTIES")
}

// TestErrUnsafeFrontmatterProperty_Is는 errors.Is 매칭을 검증한다.
func TestErrUnsafeFrontmatterProperty_Is(t *testing.T) {
	err1 := ErrUnsafeFrontmatterProperty{Property: "bad-key"}
	err2 := ErrUnsafeFrontmatterProperty{Property: "bad-key"}
	err3 := ErrUnsafeFrontmatterProperty{Property: "other-key"}

	assert.ErrorIs(t, err1, err2, "같은 Property는 Is 매칭")
	assert.False(t, errors.Is(err1, err3), "다른 Property는 Is 미매칭")
}

// TestErrSymlinkEscape_Error는 에러 메시지 형식을 검증한다.
func TestErrSymlinkEscape_Error(t *testing.T) {
	err := ErrSymlinkEscape{Path: "/tmp/evil"}
	msg := err.Error()
	assert.Contains(t, msg, "/tmp/evil")
	assert.True(t, strings.Contains(msg, "symlink") || strings.Contains(msg, "escape"))
}

// TestErrDuplicateSkillID_Error는 에러 메시지 형식을 검증한다.
func TestErrDuplicateSkillID_Error(t *testing.T) {
	err := ErrDuplicateSkillID{ID: "my-skill", Path: "/tmp/dup/SKILL.md"}
	msg := err.Error()
	assert.Contains(t, msg, "my-skill")
	assert.Contains(t, msg, "/tmp/dup/SKILL.md")
}

// TestIsRemoteID는 _canonical_ 접두사 감지를 검증한다.
func TestIsRemoteID(t *testing.T) {
	assert.True(t, IsRemoteID("_canonical_some-skill"))
	assert.True(t, IsRemoteID("_canonical_"))
	assert.False(t, IsRemoteID("regular-skill"))
	assert.False(t, IsRemoteID(""))
}

// TestParseFrontmatter_MissingClosingDelimiter는 닫는 --- 없는 경우를 검증한다.
func TestParseFrontmatter_MissingClosingDelimiter(t *testing.T) {
	// frontmatter 닫는 --- 없음 → body 전체 반환
	content := []byte(`---
name: no-close
description: "test"
This is body with no closing delimiter
`)
	// ParseSkillFile은 이를 body 없는 skill로 처리
	def, err := ParseSkillFile("/tmp/no-close/SKILL.md", content)
	// 닫는 --- 없으면 frontmatter 없이 처리되므로 에러 없이 파싱됨
	// (name은 디렉토리명 "no-close"로 파생)
	assert.NoError(t, err)
	assert.NotNil(t, def)
}

// TestEstimateSkillFrontmatterTokens_WithWhenToUse는 when-to-use 필드 포함 토큰 추정을 검증한다.
func TestEstimateSkillFrontmatterTokens_WithWhenToUse(t *testing.T) {
	fm := SkillFrontmatter{
		Name:        "test-skill",
		Description: "a test skill description",
		WhenToUse:   "use when testing is needed and you want comprehensive coverage",
	}
	tokens := EstimateSkillFrontmatterTokens(fm)
	// when-to-use가 있으면 더 높은 추정값
	fm2 := SkillFrontmatter{
		Name:        "test-skill",
		Description: "a test skill description",
	}
	tokens2 := EstimateSkillFrontmatterTokens(fm2)
	assert.Greater(t, tokens, tokens2, "when-to-use 포함 시 토큰 추정이 더 높아야 한다")
}
