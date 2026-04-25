package skill

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

// ParseSkillFile은 SKILL.md 파일 내용을 파싱하여 SkillDefinition을 반환한다.
// frontmatter를 allowlist-default-deny로 검증하며, 알 수 없는 키 발견 시 ErrUnsafeFrontmatterProperty를 반환한다.
// REQ-SK-001, REQ-SK-005, §6.7 2단계 파싱 전략
//
// @MX:ANCHOR: [AUTO] ParseSkillFile — 외부 입력 진입 + Allowlist-Default-Deny 보안 게이트
// @MX:REASON: 모든 SKILL.md의 frontmatter 파싱이 이 함수를 통과한다. fan_in >= 3 (loader, remote, test)
// @MX:SPEC: REQ-SK-013, REQ-SK-022
func ParseSkillFile(path string, content []byte) (*SkillDefinition, error) {
	// frontmatter와 body 분리
	fmBytes, body, err := splitFrontmatterAndBody(content)
	if err != nil {
		return nil, fmt.Errorf("ParseSkillFile %s: frontmatter 분리 실패: %w", path, err)
	}

	// 2단계 frontmatter 파싱 (allowlist 검증 포함)
	fm, err := parseFrontmatter(fmBytes)
	if err != nil {
		return nil, err
	}

	// ID는 name 필드에서 파생 (없으면 파일 디렉토리명)
	id := fm.Name
	if id == "" {
		id = filepath.Base(filepath.Dir(path))
	}

	// effort 레벨 결정
	effort := ResolveEffort(fm)

	// trigger 결정
	trigger := DetermineTrigger(fm, id)

	// frontmatter 토큰 추정
	tokens := EstimateSkillFrontmatterTokens(fm)

	def := &SkillDefinition{
		ID:                id,
		AbsolutePath:      path,
		Frontmatter:       fm,
		Body:              string(body),
		Trigger:           trigger,
		Effort:            effort,
		PreferredModel:    fm.Model,
		FrontmatterTokens: tokens,
		IsRemote:          strings.HasPrefix(id, "_canonical_"),
		ArgumentHint:      fm.ArgumentHint,
	}

	return def, nil
}

// splitFrontmatterAndBody는 SKILL.md 내용을 frontmatter(--- ... ---) 부분과 body로 분리한다.
// frontmatter가 없는 경우 빈 []byte와 전체 content를 반환한다.
func splitFrontmatterAndBody(content []byte) (frontmatter []byte, body []byte, err error) {
	const delimiter = "---"

	lines := bytes.Split(content, []byte("\n"))

	// 첫 번째 줄이 ---로 시작해야 frontmatter 블록
	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != delimiter {
		return []byte{}, content, nil
	}

	// 두 번째 --- 찾기
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == delimiter {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		// 닫는 --- 없음 → frontmatter 없는 것으로 처리
		return []byte{}, content, nil
	}

	// frontmatter: 1번째 줄(인덱스 1)부터 endIdx-1번째 줄까지
	fmLines := lines[1:endIdx]
	frontmatter = bytes.Join(fmLines, []byte("\n"))

	// body: endIdx+1번째 줄부터
	if endIdx+1 < len(lines) {
		body = bytes.Join(lines[endIdx+1:], []byte("\n"))
		// 앞쪽 빈 줄 trim
		body = bytes.TrimLeft(body, "\n")
	}

	return frontmatter, body, nil
}
