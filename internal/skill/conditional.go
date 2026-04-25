package skill

import (
	"strings"

	gitignore "github.com/denormal/go-gitignore"
)

// matchesPaths는 변경된 파일 경로 목록이 skill의 paths 패턴에 매칭되는지 확인한다.
// gitignore 문법(! 부정 패턴 포함)을 사용한다.
// REQ-SK-007, §6.5
func matchesPaths(patterns []string, changedPaths []string) bool {
	if len(patterns) == 0 {
		return false
	}

	// patterns를 gitignore 형식 문자열로 결합
	patternContent := strings.Join(patterns, "\n") + "\n"
	reader := strings.NewReader(patternContent)

	ig := gitignore.New(reader, "", func(e gitignore.Error) bool {
		return true // 에러 무시하고 계속
	})

	for _, changed := range changedPaths {
		if isPathMatched(ig, changed) {
			return true
		}
	}
	return false
}

// isPathMatched는 단일 파일 경로가 gitignore 패턴에 매칭되는지 확인한다.
// Relative 메서드는 base 기준 상대 경로로 매칭한다.
func isPathMatched(ig gitignore.GitIgnore, path string) bool {
	// Relative(path, isDir) — 파일이므로 isDir=false
	match := ig.Relative(path, false)
	if match == nil {
		return false
	}
	// gitignore에서 "Ignore()"는 해당 패턴이 무시(제외) 의미
	// 우리는 "match하여 활성화"를 원하므로 Ignore()=true가 매칭됨
	return match.Ignore()
}

// IsForked는 skill이 fork trigger인지 반환한다.
// REQ-SK-008: context: fork 설정 시 true.
func IsForked(fm SkillFrontmatter) bool {
	return fm.Context == "fork"
}

// IsInline은 skill이 inline trigger인지 반환한다.
func IsInline(fm SkillFrontmatter) bool {
	return fm.Context != "fork" && len(fm.Paths) == 0
}

// IsConditional은 skill이 conditional trigger인지 반환한다.
// REQ-SK-011: paths 미지정이면 false.
func IsConditional(fm SkillFrontmatter) bool {
	return len(fm.Paths) > 0
}
