// Package builtin은 내장 tool 6종의 자동 등록을 담당한다.
// SPEC-GOOSE-TOOLS-001 §3.1 #3
package builtin

import "github.com/modu-ai/goose/internal/tools"

// Register는 tool을 전역 built-in 목록에 추가한다.
// 각 tool 파일의 init()에서 호출된다.
func Register(t tools.Tool) {
	tools.RegisterBuiltin(t)
}
