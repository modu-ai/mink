//go:build tools

// Package core는 goosed 데몬의 핵심 런타임 컴포넌트를 포함한다.
//
// tools.go는 go.mod에 미래 Phase에서 사용할 의존성을 선언적으로 유지하기 위한
// build-tag 격리 파일이다. 실제 사용은 각 SPEC 구현 단계에서 진행한다.
//
// SPEC-GOOSE-CORE-001 §6 기술 스택 확정 참조.
package core

import (
	// Phase 2+: SQLite 임베디드 DB (CGO-free, 순수 Go)
	_ "modernc.org/sqlite"

	// Phase 2+: tiktoken 호환 토크나이저 (LLM 토큰 계산)
	_ "github.com/pkoukk/tiktoken-go"

	// Phase 8+: 임베디드 그래프 DB
	_ "github.com/kuzudb/go-kuzu"

	// TRANSPORT-001+: gRPC 스트리밍 (LLM 스트림)
	_ "google.golang.org/grpc"
)
