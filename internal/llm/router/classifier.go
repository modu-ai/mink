package router

import (
	"regexp"
	"strings"
	"unicode"
)

// 기본 판정 기준값 (Hermes Agent 원형 인용).
const (
	// DefaultMaxChars는 문자 수 기본 상한이다.
	DefaultMaxChars = 160
	// DefaultMaxWords는 단어 수 기본 상한이다.
	DefaultMaxWords = 28
	// DefaultMaxNewlines는 개행 수 기본 상한이다.
	DefaultMaxNewlines = 2
)

// DefaultComplexKeywords는 기본 복잡 키워드 목록이다 (Hermes 원형 확장).
// 사용자가 RoutingConfig.ComplexKeywords로 override할 수 있다.
var DefaultComplexKeywords = []string{
	// 코드 작업
	"debug", "implement", "refactor", "test", "analyze",
	"design", "architecture", "fix", "build", "compile",
	"review", "optimize", "profile",
	// 인프라
	"terminal", "docker", "kubernetes", "deploy", "install",
	"configure", "setup",
	// 파일/검색
	"grep", "search", "find", "read", "write", "edit",
	"delete",
	// 데이터
	"query", "migrate", "schema", "index",
	// 한국어 기본 (v0.1)
	"구현", "디버그", "리팩토링", "테스트", "분석",
	"설계", "배포",
}

// pre-compiled 정규식 (패키지 초기화 시 한 번만 컴파일).
var (
	// urlPattern은 http/https URL을 감지한다.
	urlPattern = regexp.MustCompile(`https?://\S+`)
	// codeFencePattern은 ``` 또는 ~~~ 코드 펜스를 감지한다.
	codeFencePattern = regexp.MustCompile("(?m)^(`{3,}|~{3,})")
	// indentedCodePattern은 4+ 공백 또는 탭으로 시작하는 연속 2줄 이상을 감지한다.
	// REQ-ROUTER-013: 멀티라인 코드 블록 탐지.
	indentedCodePattern = regexp.MustCompile(`(?m)^(    |\t).+\n(    |\t).+`)
)

// Classifier는 메시지를 단순/복잡으로 분류하는 인터페이스이다.
type Classifier interface {
	// Classify는 메시지를 분류하고 결과를 반환한다.
	Classify(msg string) ClassifierResult
}

// ClassifierResult는 분류 결과이다.
type ClassifierResult struct {
	// IsSimple은 메시지가 단순(cheap route 대상)이면 true이다.
	IsSimple bool
	// Reasons는 complex_task 판정 근거 목록이다 (observability).
	// IsSimple=true이면 비어 있다.
	Reasons []string
}

// SimpleClassifier는 Hermes의 choose_cheap_model_route 로직을 구현한 기본 classifier이다.
// 6개 기준을 모두 충족해야 단순(IsSimple=true)으로 판정한다 (conservative by design).
type SimpleClassifier struct {
	// MaxChars는 char_count 기준값이다.
	MaxChars int
	// MaxWords는 word_count 기준값이다.
	MaxWords int
	// MaxNewlines는 newline_count 기준값이다.
	MaxNewlines int
	// ComplexKeywords는 복잡 키워드 맵이다 (lowercase, whole word match).
	ComplexKeywords map[string]struct{}
	// koreanKeywords는 한국어 키워드 목록이다 (substring match).
	koreanKeywords []string
	// keywordPatterns는 각 영어 키워드에 대해 미리 컴파일된 정규식이다.
	keywordPatterns []*regexp.Regexp
}

// ClassifierConfig는 SimpleClassifier 생성 설정이다.
type ClassifierConfig struct {
	MaxChars        int
	MaxWords        int
	MaxNewlines     int
	ComplexKeywords []string
}

// DefaultClassifierConfig는 기본 ClassifierConfig를 반환한다.
func DefaultClassifierConfig() ClassifierConfig {
	return ClassifierConfig{
		MaxChars:        DefaultMaxChars,
		MaxWords:        DefaultMaxWords,
		MaxNewlines:     DefaultMaxNewlines,
		ComplexKeywords: DefaultComplexKeywords,
	}
}

// NewSimpleClassifier는 ClassifierConfig로 SimpleClassifier를 생성한다.
func NewSimpleClassifier(cfg ClassifierConfig) *SimpleClassifier {
	maxChars := cfg.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}
	maxWords := cfg.MaxWords
	if maxWords <= 0 {
		maxWords = DefaultMaxWords
	}
	maxNewlines := cfg.MaxNewlines
	if maxNewlines <= 0 {
		maxNewlines = DefaultMaxNewlines
	}
	keywords := cfg.ComplexKeywords
	if len(keywords) == 0 {
		keywords = DefaultComplexKeywords
	}

	kwMap := make(map[string]struct{}, len(keywords))
	var koreanKWs []string
	var patterns []*regexp.Regexp

	for _, kw := range keywords {
		lower := strings.ToLower(kw)
		kwMap[lower] = struct{}{}

		// CJK 문자가 포함된 키워드는 substring 매칭으로 처리
		if containsCJK(kw) {
			koreanKWs = append(koreanKWs, lower)
		} else {
			// 영어 키워드는 word-boundary 정규식으로 처리
			pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
			patterns = append(patterns, pattern)
		}
	}

	return &SimpleClassifier{
		MaxChars:        maxChars,
		MaxWords:        maxWords,
		MaxNewlines:     maxNewlines,
		ComplexKeywords: kwMap,
		koreanKeywords:  koreanKWs,
		keywordPatterns: patterns,
	}
}

// Classify는 메시지를 6개 기준으로 분류한다.
// 6개 기준 중 하나라도 실패하면 IsSimple=false (conservative by design).
func (c *SimpleClassifier) Classify(msg string) ClassifierResult {
	var reasons []string

	// 기준 1: 문자 수
	if len([]rune(msg)) > c.MaxChars {
		reasons = append(reasons, "exceeds_char_limit")
	}

	// 기준 2: 단어 수
	if countWords(msg) > c.MaxWords {
		reasons = append(reasons, "exceeds_word_limit")
	}

	// 기준 3: 개행 수
	if strings.Count(msg, "\n") > c.MaxNewlines {
		reasons = append(reasons, "exceeds_newline_limit")
	}

	// 기준 4: 코드 블록 존재
	if hasCodeBlock(msg) {
		reasons = append(reasons, "has_code_block")
	}

	// 기준 5: URL 포함
	if urlPattern.MatchString(msg) {
		reasons = append(reasons, "has_url")
	}

	// 기준 6: 복잡 키워드 포함
	if c.hasComplexKeyword(msg) {
		reasons = append(reasons, "has_complex_keyword")
	}

	return ClassifierResult{
		IsSimple: len(reasons) == 0,
		Reasons:  reasons,
	}
}

// hasCodeBlock은 메시지에 코드 블록이 포함되어 있는지 확인한다.
// ``` 또는 ~~~ fence, 또는 연속 2줄 이상의 들여쓰기를 코드 블록으로 간주한다.
func hasCodeBlock(msg string) bool {
	return codeFencePattern.MatchString(msg) || indentedCodePattern.MatchString(msg)
}

// hasComplexKeyword는 메시지에 복잡 키워드가 포함되어 있는지 확인한다.
// 영어 키워드는 word-boundary (whole word, case-insensitive) 매칭을 사용한다.
// 한국어/CJK 키워드는 substring 매칭을 사용한다.
func (c *SimpleClassifier) hasComplexKeyword(msg string) bool {
	lower := strings.ToLower(msg)

	// 영어 키워드: word-boundary 정규식 매칭
	for _, pattern := range c.keywordPatterns {
		if pattern.MatchString(msg) {
			return true
		}
	}

	// 한국어/CJK 키워드: substring 매칭
	for _, kw := range c.koreanKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	return false
}

// countWords는 메시지의 단어 수를 반환한다.
// 유니코드 공백으로 분리하며, CJK 문자는 각각 1단어로 계산한다.
func countWords(msg string) int {
	if strings.TrimSpace(msg) == "" {
		return 0
	}

	// 유니코드 공백 기준 분리
	fields := strings.FieldsFunc(msg, unicode.IsSpace)
	return len(fields)
}

// containsCJK는 문자열에 CJK 문자가 포함되어 있는지 확인한다.
func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) ||
			unicode.Is(unicode.Hangul, r) ||
			unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}
