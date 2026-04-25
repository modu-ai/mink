package skill

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// ResolveEffort는 frontmatter의 effort 필드를 EffortLevel enum으로 변환한다.
// REQ-SK-010: L0/L1/L2/L3 문자열 또는 정수 0/1/2/3을 처리한다. 기본값은 EffortL1이다.
func ResolveEffort(fm SkillFrontmatter) EffortLevel {
	switch strings.TrimSpace(fm.Effort) {
	case "L0", "0":
		return EffortL0
	case "L1", "1", "":
		return EffortL1
	case "L2", "2":
		return EffortL2
	case "L3", "3":
		return EffortL3
	default:
		// 알 수 없는 값 → 기본값 L1
		return EffortL1
	}
}

// EstimateSkillFrontmatterTokens는 frontmatter만으로 토큰 수를 추정한다.
// REQ-SK-003: name + description + when-to-use 필드 길이 기반 휴리스틱.
// skill body를 읽지 않는다. 정확한 tokenizer가 아닌 상한 근사값이다.
func EstimateSkillFrontmatterTokens(fm SkillFrontmatter) int {
	total := len(fm.Name) + len(fm.Description) + len(fm.WhenToUse)
	// 평균적으로 4자당 1토큰으로 추정 (UTF-8 영문 기준 상한 근사)
	tokens := total / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// DetermineTrigger는 frontmatter와 ID를 기반으로 TriggerMode를 결정한다.
// §6.3 우선순위: remote > fork > conditional > inline
// REQ-SK-021: 정확히 4종 trigger만 지원한다.
func DetermineTrigger(fm SkillFrontmatter, id string) TriggerMode {
	// 1. ID prefix _canonical_ → remote
	if strings.HasPrefix(id, "_canonical_") {
		return TriggerRemote
	}
	// 2. context: fork → fork
	if fm.Context == "fork" {
		return TriggerFork
	}
	// 3. paths 있음 → conditional
	if len(fm.Paths) > 0 {
		return TriggerConditional
	}
	// 4. 기본 → inline
	return TriggerInline
}

// IsRemoteID는 ID가 remote skill 접두사를 가지는지 반환한다.
func IsRemoteID(id string) bool {
	return strings.HasPrefix(id, "_canonical_")
}

// varPattern은 ${VAR_NAME} 형식의 변수를 매칭한다.
var varPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ResolveBody는 skill body의 변수를 치환한다.
// REQ-SK-006: 알려진 변수만 치환하고, 알 수 없는 변수는 리터럴 보존 + warn 로그.
// REQ-SK-014: os.Getenv 호출 절대 금지. 환경 변수 노출 위험 방지.
//
// 안전한 변수 목록 (고정):
//   - ${CLAUDE_SKILL_DIR}: skill 파일이 위치한 디렉토리의 절대 경로
//   - ${CLAUDE_SESSION_ID}: sessionID 인자로 주입된 값
//   - ${USER_HOME}: os.UserHomeDir() 결과 (에러 시 빈 문자열)
//
// agentskills.io 표준과의 차이: 임의 env var 치환을 허용하지 않는다 (REQ-SK-014, §2.4.1).
func ResolveBody(def *SkillDefinition, sessionID string, logger *zap.Logger) string {
	skillDir := filepath.Dir(def.AbsolutePath)
	userHome, _ := os.UserHomeDir() // 에러 시 빈 문자열 사용

	warnedVars := make(map[string]bool)

	result := varPattern.ReplaceAllStringFunc(def.Body, func(match string) string {
		// ${VAR_NAME}에서 VAR_NAME 추출
		inner := match[2 : len(match)-1] // ${ } 제거

		switch inner {
		case "CLAUDE_SKILL_DIR":
			return skillDir
		case "CLAUDE_SESSION_ID":
			return sessionID
		case "USER_HOME":
			return userHome
		default:
			// 알 수 없는 변수: 리터럴 보존 + warn (skill당 1회)
			// REQ-SK-014: os.Getenv 호출 금지
			if !warnedVars[inner] {
				logger.Warn("알 수 없는 skill body 변수 — 리터럴로 보존",
					zap.String("skill_id", def.ID),
					zap.String("variable", inner),
				)
				warnedVars[inner] = true
			}
			return match // 원래 ${VAR_NAME} 그대로 반환
		}
	})

	return result
}
