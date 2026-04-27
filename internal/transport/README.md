# internal/transport

**Transport Layer 패키지** — gRPC 및 HTTP 통신 추상화

## 개요

본 패키지는 AI.GOOSE의 **통신 계층**을 구현합니다. gRPC 기반의 클라이언트-서버 통신과 HTTP transport를 추상화하여, `internal/query`와 `internal/mcp`가 프로토콜에 종속되지 않도록 합니다.

## 핵심 구성 요소

### Transport Interface

통신 추상화 인터페이스:

```go
type Transport interface {
    // Send delivers a message to the remote endpoint
    Send(ctx context.Context, msg *Message) error

    // Receive blocks until a message arrives or context cancels
    Receive(ctx context.Context) (*Message, error)

    // Close gracefully shuts down the transport
    Close() error
}
```

### gRPC Transport

`internal/transport/grpc/`에서 gRPC 기반 통신 구현:

```go
type GRPCTransport struct {
    conn   *grpc.ClientConn
    stream proto.Service_StreamClient
}

func NewGRPCClient(addr string, opts ...grpc.DialOption) (*GRPCTransport, error)
```

### Message Types

```go
type Message struct {
    ID        string            // message identifier
    Type      MessageType       // request/response/event
    Payload   []byte            // message body (protobuf or JSON)
    Headers   map[string]string // metadata headers
    Timestamp time.Time         // send/receive time
}
```

## 서브패키지

| 패키지 | 설명 |
|--------|------|
| `grpc/` | gRPC 클라이언트/서버 구현 |

## 파일 구조

```
internal/transport/
├── transport.go          # Transport interface 정의
├── message.go            # Message 타입 정의
├── errors.go             # Transport 에러
└── grpc/
    ├── client.go         # gRPC 클라이언트
    ├── server.go         # gRPC 서버
    └── options.go        # gRPC 옵션
```

## 관련 SPEC

- **SPEC-GOOSE-TRANSPORT-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-DAEMON-WIRE-001**: Daemon ↔ CLI 통신 wiring

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-TRANSPORT-001
