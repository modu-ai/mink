// Package tool는 LLM tool/function 정의 타입을 제공한다.
// SPEC-GOOSE-ADAPTER-001 M0 T-002
package tool

// Definition은 LLM에 전달되는 tool/function의 스키마 정의이다.
// OpenAI function calling 형식을 기반으로 한다.
type Definition struct {
	// Name은 tool의 고유 이름이다.
	Name string
	// Description은 tool의 기능을 설명한다.
	Description string
	// Parameters는 JSON Schema 형식의 파라미터 정의이다.
	Parameters map[string]any
}
