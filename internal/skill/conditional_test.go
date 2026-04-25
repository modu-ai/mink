package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestFileChangedConsumer_GitignoreMatching는 FileChangedConsumer가 gitignore 문법으로
// 파일 변경을 매칭하는지 검증한다.
// RED #5 — AC-SK-004, REQ-SK-007, REQ-SK-011
func TestFileChangedConsumer_GitignoreMatching(t *testing.T) {
	logger := zap.NewNop()
	reg := NewSkillRegistry(logger)

	// conditional skill: src/**/*.ts 매칭, !**/test/** 부정 제외
	reg.replaceInternal(map[string]*SkillDefinition{
		"conditional": {
			ID:      "conditional",
			Trigger: TriggerConditional,
			Frontmatter: SkillFrontmatter{
				Name:        "conditional",
				Description: "conditional skill",
				Paths:       []string{"src/**/*.ts", "!**/test/**"},
			},
		},
		"unconditional": {
			ID:      "unconditional",
			Trigger: TriggerInline,
			Frontmatter: SkillFrontmatter{
				Name:        "unconditional",
				Description: "unconditional skill",
				// paths 없음
			},
		},
	})

	changedPaths := []string{
		"src/foo/bar.ts",      // positive 매칭
		"src/test/baz.ts",    // 부정 패턴으로 제외
		"README.md",           // 미매칭
	}

	result := reg.FileChangedConsumer(changedPaths)

	// conditional skill은 결과에 포함
	assert.Contains(t, result, "conditional",
		"paths 매칭 skill은 결과에 포함되어야 한다")

	// unconditional skill은 결과에 포함되지 않음 (REQ-SK-011)
	assert.NotContains(t, result, "unconditional",
		"paths 미지정 skill은 FileChangedConsumer 결과에 포함되지 않아야 한다")

	// 중복 없음
	count := 0
	for _, id := range result {
		if id == "conditional" {
			count++
		}
	}
	assert.Equal(t, 1, count, "skill ID는 정확히 1회만 포함되어야 한다")
}

// TestIsForked_ContextFork는 context: fork 설정 시 IsForked가 true를 반환하는지 검증한다.
// RED #6 — AC-SK-005, REQ-SK-008
func TestIsForked_ContextFork(t *testing.T) {
	tests := []struct {
		name          string
		context       string
		expectForked  bool
		expectInline  bool
	}{
		{"fork context", "fork", true, false},
		{"inline context", "inline", false, true},
		{"empty context", "", false, true},
		{"other context", "other", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := SkillFrontmatter{Context: tt.context}
			assert.Equal(t, tt.expectForked, IsForked(fm))
			assert.Equal(t, tt.expectInline, IsInline(fm))
		})
	}
}

// TestIsConditional는 paths 설정 시 IsConditional이 true를 반환하는지 검증한다.
// REQ-SK-011
func TestIsConditional(t *testing.T) {
	t.Run("with paths", func(t *testing.T) {
		fm := SkillFrontmatter{Paths: []string{"src/**/*.ts"}}
		assert.True(t, IsConditional(fm))
	})

	t.Run("without paths", func(t *testing.T) {
		fm := SkillFrontmatter{}
		assert.False(t, IsConditional(fm))
	})
}
