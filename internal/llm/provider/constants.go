// Package provider는 LLM provider 공용 상수를 정의한다.
package provider

import "time"

// streaming / non-streaming 타임아웃 상수 (REQ-ADAPTER-013).
//
// @MX:ANCHOR: [AUTO] 패키지 수준 타임아웃 상수 — adapter Options.HeartbeatTimeout 기본값
// @MX:REASON: 4개 provider(anthropic, openai, ollama, google)가 모두 참조하며
//
//	테스트에서는 이 값을 override하여 빠른 검증을 수행한다.
const (
	// DefaultStreamHeartbeatTimeout는 streaming heartbeat 타임아웃이다.
	// streaming 중 이 시간 동안 데이터가 없으면 연결을 중단하고 error StreamEvent를 방출한다.
	DefaultStreamHeartbeatTimeout = 60 * time.Second

	// DefaultNonStreamDataTimeout는 non-streaming 요청 타임아웃이다.
	// HTTP 응답 헤더 이후 이 시간 동안 응답 바디가 없으면 중단한다.
	DefaultNonStreamDataTimeout = 30 * time.Second
)
