package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSchema_SafeSkillProperties_ContainsExpected는 SAFE_SKILL_PROPERTIES 맵이
// REQ-SK-019에서 정의한 정확히 15개의 키를 포함하는지 검증한다.
// RED #1 — AC-SK-002 (allowlist-default-deny), REQ-SK-019
func TestSchema_SafeSkillProperties_ContainsExpected(t *testing.T) {
	expectedKeys := []string{
		"name",
		"description",
		"when-to-use",
		"argument-hint",
		"arguments",
		"model",
		"effort",
		"context",
		"agent",
		"allowed-tools",
		"disable-model-invocation",
		"user-invocable",
		"paths",
		"shell",
		"hooks",
	}

	// 정확히 15개의 키여야 한다.
	assert.Equal(t, 15, len(SAFE_SKILL_PROPERTIES),
		"SAFE_SKILL_PROPERTIES must contain exactly 15 keys per REQ-SK-019")

	for _, key := range expectedKeys {
		_, ok := SAFE_SKILL_PROPERTIES[key]
		assert.True(t, ok, "SAFE_SKILL_PROPERTIES must contain key %q", key)
	}
}
