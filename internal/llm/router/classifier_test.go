// Package router_test는 router 패키지의 외부 테스트를 포함한다.
package router_test

import (
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/llm/router"
)

// newDefaultClassifier는 기본 설정의 SimpleClassifier를 생성한다.
func newDefaultClassifier() *router.SimpleClassifier {
	return router.NewSimpleClassifier(router.DefaultClassifierConfig())
}

// TestClassifier_SimpleGreeting_ClassifiesSimple은 단순 인사 메시지가
// simple_turn으로 분류되는지 검증한다. AC-ROUTER-001 classifier 부분.
func TestClassifier_SimpleGreeting_ClassifiesSimple(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()
	tests := []struct {
		name       string
		msg        string
		wantSimple bool
	}{
		{"단순 영어 인사", "Hello, how are you?", true},
		{"단순 한국어 인사", "안녕하세요, 오늘 날씨 어때요?", true},
		{"빈 메시지", "", true},
		{"짧은 단어", "hi", true},
		{"단일 단어", "hello", true},
		{"공백만", "   ", true},
		{"숫자만", "42", true},
		{"이모지 포함 짧은 메시지", "좋아요! 👍", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cls.Classify(tc.msg)
			if result.IsSimple != tc.wantSimple {
				t.Errorf("Classify(%q): IsSimple=%v, want=%v; Reasons=%v",
					tc.msg, result.IsSimple, tc.wantSimple, result.Reasons)
			}
		})
	}
}

// TestClassifier_ComplexKeyword_ClassifiesComplex는 복잡 키워드를 포함한 메시지가
// complex_task로 분류되는지 검증한다. AC-ROUTER-002.
func TestClassifier_ComplexKeyword_ClassifiesComplex(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()
	tests := []struct {
		name        string
		msg         string
		wantSimple  bool
		wantReasons []string
	}{
		{
			name:        "debug 키워드",
			msg:         "debug this function please",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "implement 키워드",
			msg:         "implement a new feature",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "refactor 키워드",
			msg:         "refactor this code",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "test 키워드",
			msg:         "test the function",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "analyze 키워드",
			msg:         "analyze this data",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "design 키워드",
			msg:         "design a new API",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "architecture 키워드",
			msg:         "show me the architecture",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "terminal 키워드",
			msg:         "open a terminal",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "docker 키워드",
			msg:         "run docker compose",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "대문자 DEBUG (case-insensitive)",
			msg:         "DEBUG the issue",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "혼합 대소문자 Implement",
			msg:         "Implement the handler",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "deploy 키워드",
			msg:         "deploy to production",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "install 키워드",
			msg:         "install dependencies",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "fix 키워드",
			msg:         "fix the bug",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "build 키워드",
			msg:         "build the project",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "compile 키워드",
			msg:         "compile the code",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "migrate 키워드",
			msg:         "migrate the database",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "schema 키워드",
			msg:         "show database schema",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
		{
			name:        "query 키워드",
			msg:         "query the database",
			wantSimple:  false,
			wantReasons: []string{"has_complex_keyword"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cls.Classify(tc.msg)
			if result.IsSimple != tc.wantSimple {
				t.Errorf("Classify(%q): IsSimple=%v, want=%v; Reasons=%v",
					tc.msg, result.IsSimple, tc.wantSimple, result.Reasons)
			}
			// Reasons에 기대하는 항목이 포함되어 있는지 확인
			for _, want := range tc.wantReasons {
				if !containsReason(result.Reasons, want) {
					t.Errorf("Classify(%q): Reasons=%v, want to contain %q",
						tc.msg, result.Reasons, want)
				}
			}
		})
	}
}

// TestClassifier_WordBoundary_KeywordMatch는 단어 경계 매칭이 올바르게
// 동작하는지 검증한다 (부분 매칭 배제). REQ-ROUTER-008.
func TestClassifier_WordBoundary_KeywordMatch(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()
	tests := []struct {
		name       string
		msg        string
		wantSimple bool
	}{
		// "debug"는 키워드지만 "debugger"는 아님
		{"debugger는 키워드 아님", "the debugger runs fine", true},
		// "test"는 키워드지만 "testament"는 아님
		{"testament는 키워드 아님", "this is a testament", true},
		// "fix"는 키워드지만 "prefix"는 아님
		{"prefix는 키워드 아님", "check the prefix value", true},
		// "build"는 키워드지만 "rebuild"의 경우
		{"rebuild — fix 포함 안 함", "rebuild from scratch", true},
		// 실제 키워드가 독립 단어로 있는 경우
		{"debug 독립 단어", "I need to debug this", false},
		{"test 독립 단어", "please test this function", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cls.Classify(tc.msg)
			if result.IsSimple != tc.wantSimple {
				t.Errorf("Classify(%q): IsSimple=%v, want=%v; Reasons=%v",
					tc.msg, result.IsSimple, tc.wantSimple, result.Reasons)
			}
		})
	}
}

// TestClassifier_CodeBlock_ClassifiesComplex는 코드 블록 포함 메시지가
// complex_task로 분류되는지 검증한다. AC-ROUTER-003.
func TestClassifier_CodeBlock_ClassifiesComplex(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()
	tests := []struct {
		name       string
		msg        string
		wantSimple bool
		wantReason string
	}{
		{
			name:       "backtick fence 3개",
			msg:        "fix this\n```go\nfunc main(){}\n```",
			wantSimple: false,
			wantReason: "has_code_block",
		},
		{
			name:       "tilde fence 3개",
			msg:        "look at this\n~~~python\nx = 1\n~~~",
			wantSimple: false,
			wantReason: "has_code_block",
		},
		{
			name:       "언어 없는 backtick fence",
			msg:        "```\nsome code\n```",
			wantSimple: false,
			wantReason: "has_code_block",
		},
		{
			name:       "멀티라인 코드 블록",
			msg:        "```go\nfunc main(){\n  fmt.Println(\"hello\")\n}\n```",
			wantSimple: false,
			wantReason: "has_code_block",
		},
		{
			name:       "인라인 backtick은 코드 블록 아님",
			msg:        "check the `x` variable",
			wantSimple: true,
			wantReason: "",
		},
		{
			name:       "backtick 2개는 fence 아님",
			msg:        "`` is nothing",
			wantSimple: true,
			wantReason: "",
		},
		{
			name:       "4+ space 연속 2줄 이상 — 코드 블록 (REQ-ROUTER-013)",
			msg:        "look:\n    line one code\n    line two code",
			wantSimple: false,
			wantReason: "has_code_block",
		},
		{
			name:       "탭 들여쓰기 연속 2줄",
			msg:        "example:\n\tfunc foo() {}\n\treturn nil",
			wantSimple: false,
			wantReason: "has_code_block",
		},
		{
			name:       "4 space 1줄만은 코드 블록 아님",
			msg:        "    just one indented line",
			wantSimple: true, // 1줄만이므로 code block 아님
			wantReason: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cls.Classify(tc.msg)
			if result.IsSimple != tc.wantSimple {
				t.Errorf("Classify(%q): IsSimple=%v, want=%v; Reasons=%v",
					tc.msg, result.IsSimple, tc.wantSimple, result.Reasons)
			}
			if tc.wantReason != "" && !containsReason(result.Reasons, tc.wantReason) {
				t.Errorf("Classify(%q): Reasons=%v, want to contain %q",
					tc.msg, result.Reasons, tc.wantReason)
			}
		})
	}
}

// TestClassifier_URL_ClassifiesComplex는 URL 포함 메시지가
// complex_task로 분류되는지 검증한다. AC-ROUTER-004.
func TestClassifier_URL_ClassifiesComplex(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()
	tests := []struct {
		name       string
		msg        string
		wantSimple bool
		wantReason string
	}{
		{
			name:       "https URL",
			msg:        "check https://example.com for details",
			wantSimple: false,
			wantReason: "has_url",
		},
		{
			name:       "http URL",
			msg:        "visit http://example.com",
			wantSimple: false,
			wantReason: "has_url",
		},
		{
			name:       "URL with path",
			msg:        "see https://github.com/user/repo for code",
			wantSimple: false,
			wantReason: "has_url",
		},
		{
			name:       "IP 주소 URL",
			msg:        "connect to http://192.168.1.1/api",
			wantSimple: false,
			wantReason: "has_url",
		},
		{
			name:       "URL with query string",
			msg:        "go to https://example.com/search?q=hello&lang=ko",
			wantSimple: false,
			wantReason: "has_url",
		},
		{
			name:       "도메인만 (http 없음) — URL 아님",
			msg:        "check example.com for info",
			wantSimple: true,
			wantReason: "",
		},
		{
			name:       "ftp URL은 해당 없음",
			msg:        "ftp://files.example.com/file.zip",
			wantSimple: true,
			wantReason: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cls.Classify(tc.msg)
			if result.IsSimple != tc.wantSimple {
				t.Errorf("Classify(%q): IsSimple=%v, want=%v; Reasons=%v",
					tc.msg, result.IsSimple, tc.wantSimple, result.Reasons)
			}
			if tc.wantReason != "" && !containsReason(result.Reasons, tc.wantReason) {
				t.Errorf("Classify(%q): Reasons=%v, want to contain %q",
					tc.msg, result.Reasons, tc.wantReason)
			}
		})
	}
}

// TestClassifier_LongMessage_ClassifiesComplex는 길이 초과 메시지가
// complex_task로 분류되는지 검증한다. AC-ROUTER-005.
func TestClassifier_LongMessage_ClassifiesComplex(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()
	tests := []struct {
		name       string
		msg        string
		wantSimple bool
		wantReason string
	}{
		// 문자 수 경계 테스트
		{
			name:       "정확히 160자 — simple",
			msg:        strings.Repeat("a", 160),
			wantSimple: true,
			wantReason: "",
		},
		{
			name:       "161자 — complex",
			msg:        strings.Repeat("a", 161),
			wantSimple: false,
			wantReason: "exceeds_char_limit",
		},
		{
			name:       "200자 — complex",
			msg:        strings.Repeat("a", 200),
			wantSimple: false,
			wantReason: "exceeds_char_limit",
		},
		// 단어 수 경계 테스트
		{
			name:       "정확히 28 단어 — simple",
			msg:        strings.TrimSpace(strings.Repeat("word ", 28)),
			wantSimple: true,
			wantReason: "",
		},
		{
			name:       "29 단어 — complex",
			msg:        strings.TrimSpace(strings.Repeat("word ", 29)),
			wantSimple: false,
			wantReason: "exceeds_word_limit",
		},
		// 개행 수 경계 테스트
		{
			name:       "개행 2개 — simple (3줄)",
			msg:        "line1\nline2\nline3",
			wantSimple: true,
			wantReason: "",
		},
		{
			name:       "개행 3개 — complex (4줄)",
			msg:        "a\nb\nc\nd",
			wantSimple: false,
			wantReason: "exceeds_newline_limit",
		},
		{
			name:       "개행 0개 — simple",
			msg:        "no newlines here",
			wantSimple: true,
			wantReason: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cls.Classify(tc.msg)
			if result.IsSimple != tc.wantSimple {
				t.Errorf("Classify(%q...): IsSimple=%v, want=%v; Reasons=%v",
					tc.msg[:min(len(tc.msg), 30)], result.IsSimple, tc.wantSimple, result.Reasons)
			}
			if tc.wantReason != "" && !containsReason(result.Reasons, tc.wantReason) {
				t.Errorf("Reasons=%v, want to contain %q", result.Reasons, tc.wantReason)
			}
		})
	}
}

// TestClassifier_CJK_Messages는 CJK(한국어/중국어/일본어) 메시지를 올바르게
// 처리하는지 검증한다. SPEC §8 Risk R6.
func TestClassifier_CJK_Messages(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()
	tests := []struct {
		name       string
		msg        string
		wantSimple bool
	}{
		{"단순 한국어", "오늘 날씨가 좋네요", true},
		{"단순 중국어", "今天天气很好", true},
		{"단순 일본어", "今日の天気はいいですね", true},
		{"한국어 복잡 키워드 구현", "이 함수 구현해줘", false},
		{"한국어 복잡 키워드 디버그", "이 코드 디버그 해줘", false},
		{"한국어 복잡 키워드 테스트", "유닛 테스트 작성해줘", false},
		{"한국어 복잡 키워드 분석", "데이터 분석해줘", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cls.Classify(tc.msg)
			if result.IsSimple != tc.wantSimple {
				t.Errorf("Classify(%q): IsSimple=%v, want=%v; Reasons=%v",
					tc.msg, result.IsSimple, tc.wantSimple, result.Reasons)
			}
		})
	}
}

// TestClassifier_MultipleReasons는 여러 복잡 기준이 동시에 충족될 때
// 모든 이유가 Reasons에 포함되는지 검증한다.
func TestClassifier_MultipleReasons(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()

	// URL + 키워드 동시 포함
	result := cls.Classify("debug https://x.com")
	if result.IsSimple {
		t.Error("URL + keyword: IsSimple=true, want false")
	}
	if !containsReason(result.Reasons, "has_url") {
		t.Errorf("Reasons=%v, want to contain 'has_url'", result.Reasons)
	}
	if !containsReason(result.Reasons, "has_complex_keyword") {
		t.Errorf("Reasons=%v, want to contain 'has_complex_keyword'", result.Reasons)
	}
}

// TestClassifier_ConservativeDesign은 6개 기준 중 하나라도 실패하면
// complex_task로 분류되는지 검증한다 (conservative by design).
func TestClassifier_ConservativeDesign(t *testing.T) {
	t.Parallel()

	cls := newDefaultClassifier()

	// 160자 이하지만 키워드 포함 → complex
	shortButKeyword := "debug" // 5자, 1단어, 0개행
	result := cls.Classify(shortButKeyword)
	if result.IsSimple {
		t.Error("키워드 포함 짧은 메시지가 simple로 분류됨")
	}

	// 키워드 없지만 URL 포함 → complex
	noKeywordButURL := "see https://example.com for info"
	result = cls.Classify(noKeywordButURL)
	if result.IsSimple {
		t.Error("URL 포함 메시지가 simple로 분류됨")
	}
}

// containsReason은 reasons 슬라이스에 target이 포함되어 있는지 확인한다.
func containsReason(reasons []string, target string) bool {
	for _, r := range reasons {
		if r == target {
			return true
		}
	}
	return false
}

// min은 두 정수 중 작은 값을 반환한다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
