# internal/permission

**Permission System 패키지** — Tool 및 리소스 접근 권한 관리

## 개요

본 패키지는 AI.GOOSE의 **권한 관리 시스템**을 구현합니다. Tool 실행, 파일 접근, 네트워크 요청에 대한 권한을 관리하며, allowlist/denylist 기반의 정책과 사용자 승인 흐름을 제공합니다.

## 핵심 기능

### Permission Store

권한 정책 저장 및 조회:

```go
type Store interface {
    // Check verifies permission for a tool/action on a resource
    Check(ctx context.Context, tool string, resource string) (Decision, error)

    // Grant adds an allow rule
    Grant(ctx context.Context, tool string, resource string, duration time.Duration) error

    // Revoke removes a permission
    Revoke(ctx context.Context, tool string, resource string) error
}
```

### Decision

```go
type Decision int

const (
    Deny    Decision = iota // 명시적 거부
    Allow                    // 명시적 허용
    Ask                      // 사용자 확인 필요
)
```

### Permission Scope

```go
type Scope struct {
    Tool     string   // tool name (e.g., "Bash", "Write")
    Paths    []string // file path patterns
    Network  []string // allowed hosts
    Duration time.Duration // permission TTL
}
```

## 서브패키지

| 패키지 | 설명 |
|--------|------|
| `store/` | 권한 저장소 구현 |

## 파일 구조

```
internal/permission/
├── permission.go     # Permission, Decision 타입
├── scope.go          # Scope 정의
├── checker.go        # 권한 검증
├── store/            # 권한 저장소
└── *_test.go         # 테스트
```

## 관련 SPEC

- **SPEC-GOOSE-PERMISSION-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-TOOLS-001**: Tool 실행 권한 검증
- **SPEC-GOOSE-SUBAGENT-001**: Sub-agent 권한 격리

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-PERMISSION-001
