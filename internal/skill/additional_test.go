package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseSkillFile_PreferredModel은 model 필드가 리터럴로 보존되는지 검증한다.
// AC-SK-015, REQ-SK-017
func TestParseSkillFile_PreferredModel(t *testing.T) {
	t.Run("model set", func(t *testing.T) {
		content := []byte(`---
name: with-model
description: "skill with model"
model: "opus[1m]"
---

Body
`)
		def, err := ParseSkillFile("/tmp/skills/with-model/SKILL.md", content)
		require.NoError(t, err)
		assert.Equal(t, "opus[1m]", def.PreferredModel,
			"model 필드는 리터럴 그대로 보존되어야 한다")
	})

	t.Run("model absent", func(t *testing.T) {
		content := []byte(`---
name: no-model
description: "skill without model"
---

Body
`)
		def, err := ParseSkillFile("/tmp/skills/no-model/SKILL.md", content)
		require.NoError(t, err)
		assert.Equal(t, "", def.PreferredModel,
			"model 미지정 시 PreferredModel은 빈 문자열")
	})
}

// TestParseSkillFile_ArgumentHint는 argument-hint 필드가 노출되는지 검증한다.
// AC-SK-016, REQ-SK-018
func TestParseSkillFile_ArgumentHint(t *testing.T) {
	t.Run("argument-hint set", func(t *testing.T) {
		content := []byte(`---
name: with-hint
description: "skill with argument hint"
argument-hint: "<path> [--recursive]"
---

Body
`)
		def, err := ParseSkillFile("/tmp/skills/with-hint/SKILL.md", content)
		require.NoError(t, err)
		assert.Equal(t, "<path> [--recursive]", def.Frontmatter.ArgumentHint,
			"argument-hint 필드는 리터럴 그대로 노출되어야 한다")
	})

	t.Run("argument-hint absent", func(t *testing.T) {
		content := []byte(`---
name: no-hint
description: "skill without argument hint"
---

Body
`)
		def, err := ParseSkillFile("/tmp/skills/no-hint/SKILL.md", content)
		require.NoError(t, err)
		assert.Equal(t, "", def.Frontmatter.ArgumentHint,
			"argument-hint 미지정 시 ArgumentHint는 빈 문자열")
	})
}

// TestDetermineTrigger는 4-trigger 결정 로직의 우선순위를 검증한다.
// §6.3, REQ-SK-021
func TestDetermineTrigger(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		fm       SkillFrontmatter
		expected TriggerMode
	}{
		{
			name:     "remote prefix → TriggerRemote",
			id:       "_canonical_some-skill",
			fm:       SkillFrontmatter{},
			expected: TriggerRemote,
		},
		{
			name:     "fork context → TriggerFork",
			id:       "forked-skill",
			fm:       SkillFrontmatter{Context: "fork"},
			expected: TriggerFork,
		},
		{
			name:     "paths → TriggerConditional",
			id:       "conditional-skill",
			fm:       SkillFrontmatter{Paths: []string{"src/**/*.ts"}},
			expected: TriggerConditional,
		},
		{
			name:     "default → TriggerInline",
			id:       "inline-skill",
			fm:       SkillFrontmatter{},
			expected: TriggerInline,
		},
		{
			name:     "remote prefix beats fork",
			id:       "_canonical_forked",
			fm:       SkillFrontmatter{Context: "fork"},
			expected: TriggerRemote,
		},
		{
			name:     "fork beats conditional",
			id:       "forked-conditional",
			fm:       SkillFrontmatter{Context: "fork", Paths: []string{"src/**/*.ts"}},
			expected: TriggerFork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineTrigger(tt.fm, tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}
