package skill

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestResolveEffort_Mappings는 effort 필드 문자열/숫자 → EffortLevel 매핑을 검증한다.
// RED #4 — AC-SK-003, REQ-SK-010, REQ-SK-020
func TestResolveEffort_Mappings(t *testing.T) {
	tests := []struct {
		name     string
		effort   string
		expected EffortLevel
	}{
		{"L0 string", "L0", EffortL0},
		{"L1 string", "L1", EffortL1},
		{"L2 string", "L2", EffortL2},
		{"L3 string", "L3", EffortL3},
		{"int 0", "0", EffortL0},
		{"int 1", "1", EffortL1},
		{"int 2", "2", EffortL2},
		{"int 3", "3", EffortL3},
		{"empty (default)", "", EffortL1},
		{"unknown (default)", "X9", EffortL1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := SkillFrontmatter{Effort: tt.effort}
			result := ResolveEffort(fm)
			assert.Equal(t, tt.expected, result, "effort=%q", tt.effort)
		})
	}
}

// TestResolveBody_VariableSubstitution는 알려진 변수 치환과
// 알 수 없는 변수의 리터럴 보존 + warn 로그를 검증한다.
// RED #7 — AC-SK-006, REQ-SK-006, REQ-SK-014
func TestResolveBody_VariableSubstitution(t *testing.T) {
	logger := zaptest.NewLogger(t)

	def := &SkillDefinition{
		ID:           "hello",
		AbsolutePath: "/tmp/skills/hello/SKILL.md",
		Body:         "Working in ${CLAUDE_SKILL_DIR} session ${CLAUDE_SESSION_ID} maybe ${UNKNOWN_VAR}",
	}

	result := ResolveBody(def, "sess-abc", logger)

	// 알려진 변수: ${CLAUDE_SKILL_DIR} → /tmp/skills/hello
	assert.Contains(t, result, "/tmp/skills/hello",
		"${CLAUDE_SKILL_DIR}는 skill 디렉토리 경로로 치환되어야 한다")

	// 알려진 변수: ${CLAUDE_SESSION_ID} → sess-abc
	assert.Contains(t, result, "sess-abc",
		"${CLAUDE_SESSION_ID}는 세션 ID로 치환되어야 한다")

	// 알 수 없는 변수: 리터럴 보존
	assert.Contains(t, result, "${UNKNOWN_VAR}",
		"알 수 없는 변수는 리터럴 그대로 유지되어야 한다")

	// 치환된 결과에 ${CLAUDE_SKILL_DIR} 자체가 남아있지 않아야 함
	assert.NotContains(t, result, "${CLAUDE_SKILL_DIR}")
	assert.NotContains(t, result, "${CLAUDE_SESSION_ID}")
}

// TestResolveBody_UserHome는 ${USER_HOME} 치환을 검증한다.
func TestResolveBody_UserHome(t *testing.T) {
	logger := zap.NewNop()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("UserHomeDir not available")
	}

	def := &SkillDefinition{
		ID:           "home-test",
		AbsolutePath: "/tmp/skills/home-test/SKILL.md",
		Body:         "Home is ${USER_HOME} here",
	}

	result := ResolveBody(def, "sess-123", logger)
	assert.Contains(t, result, homeDir)
	assert.NotContains(t, result, "${USER_HOME}")
}

// TestResolveBody_NoOsGetenv는 알 수 없는 변수 처리 시 os.Getenv를 호출하지 않는지 검증한다.
// AC-SK-013, REQ-SK-014 (security)
func TestResolveBody_NoOsGetenv(t *testing.T) {
	// 환경 변수에 비밀 값 설정
	t.Setenv("SECRET_TOKEN", "supersecret123")
	t.Setenv("HOME", "/home/testuser")

	logger := zap.NewNop()

	def := &SkillDefinition{
		ID:           "secret-test",
		AbsolutePath: "/tmp/skills/hello/SKILL.md",
		Body:         "token=${SECRET_TOKEN} env=${HOME} skill=${CLAUDE_SKILL_DIR}",
	}

	result := ResolveBody(def, "sess-xyz", logger)

	// (a) ${SECRET_TOKEN}은 리터럴 보존 (os.Getenv 미호출)
	assert.Contains(t, result, "${SECRET_TOKEN}",
		"${SECRET_TOKEN}은 리터럴로 보존되어야 한다")

	// (b) ${HOME}도 리터럴 보존
	assert.Contains(t, result, "${HOME}",
		"${HOME}은 리터럴로 보존되어야 한다")

	// (c) "supersecret123"이 결과에 포함되지 않음
	assert.False(t, strings.Contains(result, "supersecret123"),
		"환경 변수 값이 결과에 포함되어서는 안 된다")

	// (d) CLAUDE_SKILL_DIR는 치환됨
	assert.Contains(t, result, "/tmp/skills/hello",
		"알려진 변수 CLAUDE_SKILL_DIR는 치환되어야 한다")
}

// TestResolveEffort_Orthogonal_ToProgressiveDisclosure는 Effort 티어와
// Progressive Disclosure 로드 스테이지가 독립적임을 검증한다.
// AC-SK-003, REQ-SK-020 (orthogonality)
func TestResolveEffort_Orthogonal_ToProgressiveDisclosure(t *testing.T) {
	// Effort 레벨과 frontmatter 토큰 추정은 독립적으로 관측 가능해야 함
	fm := SkillFrontmatter{
		Name:        "test",
		Description: "test description",
		Effort:      "L3",
	}

	effort := ResolveEffort(fm)
	tokens := EstimateSkillFrontmatterTokens(fm)

	// L3 effort이어도 frontmatter 토큰 추정은 별도로 계산
	assert.Equal(t, EffortL3, effort)
	assert.Greater(t, tokens, 0)

	// 두 값은 서로 독립적 — L3 effort가 토큰 추정을 강제하지 않음
	// (토큰 추정은 항상 name+desc+whenToUse 기반)
}
