//go:build tools

// Package tools는 개발 도구 의존성을 Go 모듈에 고정한다.
// SPEC-GOOSE-TRANSPORT-001 §6.4 생성 파이프라인
package tools

import (
	_ "github.com/bufbuild/buf/cmd/buf"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
