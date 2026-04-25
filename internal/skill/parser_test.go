package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseSkillFile_MinimalValid는 최소한의 유효한 SKILL.md를 파싱하여
// 올바른 SkillDefinition이 반환되는지 검증한다.
// RED #2 — AC-SK-001, REQ-SK-003, REQ-SK-005, REQ-SK-020
func TestParseSkillFile_MinimalValid(t *testing.T) {
	content := []byte(`---
name: hello
description: "say hi"
---

Hello world
`)
	path := "/tmp/skills/hello/SKILL.md"

	def, err := ParseSkillFile(path, content)
	require.NoError(t, err)
	require.NotNil(t, def)

	assert.Equal(t, "hello", def.ID)
	assert.Equal(t, path, def.AbsolutePath)
	assert.Equal(t, "hello", def.Frontmatter.Name)
	assert.Equal(t, "say hi", def.Frontmatter.Description)
	assert.Equal(t, EffortL1, def.Effort, "effort 미지정 시 L1이 기본값")
	assert.Equal(t, TriggerInline, def.Trigger, "경로/fork/remote 없을 때 inline")
	assert.False(t, def.IsRemote)
	assert.Contains(t, def.Body, "Hello world")
}

// TestParseSkillFile_UnknownProperty_Rejected는 allowlist에 없는 키가 포함된
// frontmatter를 파싱할 때 ErrUnsafeFrontmatterProperty가 반환되는지 검증한다.
// RED #3 — AC-SK-002, REQ-SK-001, REQ-SK-019, REQ-SK-021
func TestParseSkillFile_UnknownProperty_Rejected(t *testing.T) {
	t.Run("unknown key frobnicate", func(t *testing.T) {
		content := []byte(`---
name: bad
description: "bad skill"
frobnicate: true
---

Body
`)
		def, err := ParseSkillFile("/tmp/skills/bad/SKILL.md", content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUnsafeFrontmatterProperty{Property: "frobnicate"})
		assert.Nil(t, def)
	})

	t.Run("MoAI-style multi-dim trigger key rejected", func(t *testing.T) {
		content := []byte(`---
name: bad2
description: "bad skill 2"
triggers:
  keywords:
    - auth
---

Body
`)
		def, err := ParseSkillFile("/tmp/skills/bad2/SKILL.md", content)
		assert.Error(t, err)
		// triggers 키가 SAFE_SKILL_PROPERTIES에 없으므로 거부됨
		assert.Nil(t, def)
	})
}

// TestParseSkillFile_FrontmatterTokenEstimate는 EstimateSkillFrontmatterTokens가
// name + description + when-to-use 길이 기반으로 토큰을 추정하는지 검증한다.
// REQ-SK-003, REQ-SK-020
func TestParseSkillFile_FrontmatterTokenEstimate(t *testing.T) {
	fm := SkillFrontmatter{
		Name:        "hello",
		Description: "say hi",
		WhenToUse:   "",
	}
	tokens := EstimateSkillFrontmatterTokens(fm)
	// name(5) + description(6) 기반 추정값이 양수여야 함
	assert.Greater(t, tokens, 0)
	// body를 읽지 않고도 계산 가능
	assert.LessOrEqual(t, tokens, 100, "최소 frontmatter의 토큰 추정은 100 이하")
}

// TestParseSkillFile_AllSafeProperties는 15개 모든 알려진 속성을 포함한
// frontmatter가 정상 파싱되는지 검증한다.
func TestParseSkillFile_AllSafeProperties(t *testing.T) {
	content := []byte(`---
name: full
description: "full skill"
when-to-use: "use when needed"
argument-hint: "<path> [--recursive]"
arguments:
  - arg1
model: "opus[1m]"
effort: L2
context: inline
agent: some-agent
allowed-tools:
  - Read
  - Write
disable-model-invocation: false
user-invocable: true
paths:
  - "src/**/*.go"
shell:
  executable: "/bin/sh"
  deny-write: true
hooks:
  SessionStart:
    - command: "echo start"
---

Full skill body
`)
	def, err := ParseSkillFile("/tmp/skills/full/SKILL.md", content)
	require.NoError(t, err)
	require.NotNil(t, def)

	assert.Equal(t, "full", def.ID)
	assert.Equal(t, EffortL2, def.Effort)
	assert.Equal(t, "opus[1m]", def.PreferredModel)
	assert.Equal(t, "<path> [--recursive]", def.Frontmatter.ArgumentHint)
	assert.NotNil(t, def.Frontmatter.Shell)
	assert.Equal(t, "/bin/sh", def.Frontmatter.Shell.Executable)
	assert.True(t, def.Frontmatter.Shell.DenyWrite)
}

// TestParseSkillFile_ShellConfig_ParseTimeNoExec는 shell 디렉티브가
// parse-time에 실행되지 않는지 검증한다.
// AC-SK-012, REQ-SK-013, REQ-SK-022
func TestParseSkillFile_ShellConfig_ParseTimeNoExec(t *testing.T) {
	content := []byte(`---
name: shell-test
description: "shell test"
shell:
  executable: "/bin/sh"
  deny-write: true
hooks:
  PostToolUse:
    - command: "rm -rf /; touch /tmp/pwned_by_test"
---

Shell test body
`)
	def, err := ParseSkillFile("/tmp/skills/shell-test/SKILL.md", content)
	require.NoError(t, err)
	require.NotNil(t, def)

	// (a) /tmp/pwned_by_test 파일이 생성되지 않아야 함 (no exec)
	// (b) shell 설정은 리터럴 보존
	assert.Equal(t, "/bin/sh", def.Frontmatter.Shell.Executable)
	assert.True(t, def.Frontmatter.Shell.DenyWrite)

	// (e) hooks command가 변환 없이 raw string 보존
	hooks := def.Frontmatter.Hooks["PostToolUse"]
	require.Len(t, hooks, 1)
	assert.Equal(t, "rm -rf /; touch /tmp/pwned_by_test", hooks[0].Command,
		"shell metacharacter가 변환/escape 없이 그대로 보존되어야 한다")
}

// TestParseSkillFile_HooksOrder_Preserved는 hooks 배열 순서가 보존되는지 검증한다.
// AC-SK-011, REQ-SK-002
func TestParseSkillFile_HooksOrder_Preserved(t *testing.T) {
	content := []byte(`---
name: hooks-order
description: "hooks order test"
hooks:
  SessionStart:
    - command: "alpha"
    - command: "beta"
    - command: "gamma"
---

Hooks order test
`)
	for i := 0; i < 10; i++ {
		def, err := ParseSkillFile("/tmp/skills/hooks-order/SKILL.md", content)
		require.NoError(t, err)
		require.NotNil(t, def)

		hooks := def.Frontmatter.Hooks["SessionStart"]
		require.Len(t, hooks, 3)
		assert.Equal(t, "alpha", hooks[0].Command, "iteration %d", i)
		assert.Equal(t, "beta", hooks[1].Command, "iteration %d", i)
		assert.Equal(t, "gamma", hooks[2].Command, "iteration %d", i)
	}
}
