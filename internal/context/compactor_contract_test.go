// Package context_test — SPEC-GOOSE-CONTEXT-001 인터페이스 계약 테스트.
// DefaultCompactor가 query.Compactor 인터페이스를 구현함을 빌드타임에 검증한다.
// SPEC-GOOSE-CONTEXT-001 §6.3
package context_test

import (
	goosecontext "github.com/modu-ai/goose/internal/context"
	"github.com/modu-ai/goose/internal/query"
)

// 빌드타임 인터페이스 assertion.
// 컴파일 에러가 발생하면 DefaultCompactor가 query.Compactor 계약을 위반한다.
var _ query.Compactor = (*goosecontext.DefaultCompactor)(nil)
