---
id: SPEC-GOOSE-TRAJECTORY-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 4
size: 소(S)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-TRAJECTORY-001 — Trajectory 수집 + 익명화 (ShareGPT JSON-L)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-learning.md §1-2 + ROADMAP v2.0 Phase 4 기반) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT **자기진화 파이프라인의 Layer 1**을 정의한다. QueryEngine의 한 턴(turn)이 끝날 때마다 발생하는 대화 기록을 **ShareGPT 호환 JSON-L 포맷**으로 로컬 디스크에 유실 없이 적재하고, 저장 직전 **Redact 파이프라인**으로 이메일·API 키·신용카드·전화번호·경로상 PII를 제거하며, 성공(`completed=true`) vs 실패(`completed=false`) 궤적을 물리적으로 분리 저장하여 Layer 2(COMPRESSOR) · Layer 3(INSIGHTS)의 입력을 준비한다.

본 SPEC이 통과한 시점에서:

- QueryEngine의 **PostToolUse / SessionEnd / Terminal 훅**이 수신되면 `TrajectoryCollector`가 메모리 버퍼의 턴들을 조립하여 `Trajectory` 구조체로 변환하고,
- `Redactor` 체인이 설정 가능한 규칙(기본 6종)을 적용하여 민감 토큰을 `<REDACTED:kind>` 플레이스홀더로 치환하며,
- `Writer`가 `~/.goose/trajectories/{success|failed}/YYYY-MM-DD.jsonl`에 append-only 기록하고, 일일 파일 크기가 10MB를 넘거나 날짜가 바뀌면 회전(rotation)하며,
- 기록 실패는 에이전트의 사용자 흐름을 **차단하지 않는다**(best-effort 로깅).

본 SPEC은 Layer 2 이후 파이프라인 전체가 소비할 **정규화된 궤적 스키마와 그 저장 계약**의 단일 진실 공급원이다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 4 나머지 4개 SPEC(COMPRESSOR / INSIGHTS / ERROR-CLASS / MEMORY)의 **유일한 입력 소스**다. Trajectory 스키마가 고정되지 않으면 후속 SPEC이 기록 포맷에 커플링되어 반복 수정을 유발한다.
- `.moai/project/research/hermes-learning.md` §2가 Hermes의 `trajectory_samples.jsonl` / `failed_trajectories.jsonl` 포맷을 이식 대상으로 명시한다. ShareGPT 호환은 외부 도구(LoRA 훈련, HuggingFace Trainer, MLflow)와의 상호 운용성을 확보한다.
- 로드맵 v2.0 §4 Phase 4 첫 번째 SPEC. SPEC-GOOSE-COMPRESSOR-001 · INSIGHTS-001 · MEMORY-001이 모두 본 SPEC의 `Trajectory` 타입을 import한다.
- 개인정보 보호(GDPR Art.25 privacy-by-design, CCPA)는 Layer 1에서 선제 차단해야 한다. Layer 2(LLM 요약) 이후에는 PII가 이미 외부 프로바이더로 유출됐을 수 있다.

### 2.2 상속 자산 (패턴만 계승)

- **Hermes Agent Python** (`./hermes-agent-main/agent/trajectory.py` 100 LoC + `trajectory_compressor.py` 상단 데이터 구조): ShareGPT 스키마(`conversations[{from, value}]`) + success/failed 이분법 + JSON-L append. 본 SPEC은 Go로 재작성.
- **Claude Code TypeScript** (`./claude-code-source-map/`): Trajectory 수집 기능 없음. 계승 대상 아님.
- **MoAI-ADK-Go**: 본 레포 미러 없음. 패턴 계승 전무.

### 2.3 범위 경계

- **IN**: `Trajectory` / `TrajectoryEntry` 타입, `TrajectoryCollector`(QueryEngine 이벤트 수신), `Redactor` 체인 + 기본 6개 규칙, `Writer`(append-only JSON-L + rotation), success/failed 2경로 분리, 디스크 공간 관리(90일 retention 기본), 수집 비활성화 설정(`telemetry.trajectory.enabled: false`).
- **OUT**: Trajectory 압축 알고리즘(→ COMPRESSOR-001), Insights 분석(→ INSIGHTS-001), Memory 저장(→ MEMORY-001), Error 분류(→ ERROR-CLASS-001), LoRA 훈련 데이터셋 변환(→ LORA-001), 원격 전송/업로드(본 SPEC은 로컬 전용), UI 시각화(→ CLI-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/learning/trajectory/` 패키지: `Trajectory`, `TrajectoryEntry`, `Role` enum, `TrajectoryMetadata`.
2. `internal/learning/trajectory/collector.go`: QueryEngine의 `PostSamplingHooks` / `StopFailureHooks` / Terminal 이벤트 구독, 메모리 버퍼에서 턴 조립.
3. `internal/learning/trajectory/writer.go`: append-only JSON-L writer + rotation(크기 10MB 또는 날짜 변경).
4. `internal/learning/trajectory/redact/` 서브패키지: `Redactor` 인터페이스 + 6개 기본 규칙(이메일 / API 키 패턴 / Bearer 토큰 / 신용카드 번호 / 전화번호 / 홈 디렉토리 경로 사용자명).
5. `internal/learning/trajectory/rotation.go`: 날짜/크기 기반 파일 회전.
6. `internal/learning/trajectory/retention.go`: 90일 초과 궤적 자동 삭제 (기본값, 설정 가능).
7. Disk layout: `${GOOSE_HOME}/trajectories/success/YYYY-MM-DD.jsonl`, `${GOOSE_HOME}/trajectories/failed/YYYY-MM-DD.jsonl`.
8. 설정 통합: `config.yaml`의 `telemetry.trajectory.{enabled, retention_days, redact_rules, max_file_bytes}`.
9. 비동기 기록(QueryEngine critical path 차단 금지): 내부 channel + goroutine worker.
10. 기록 실패는 에이전트 흐름 차단 금지 — zap warning 로그만.
11. Session ID 격리: `Trajectory.SessionID`는 QueryEngine 인스턴스 단위로 할당 (MEMORY-001의 session_id와 공유).

### 3.2 OUT OF SCOPE (명시적 제외)

- Trajectory **압축**(COMPRESSOR-001이 담당). 본 SPEC은 원본 그대로 기록.
- Trajectory **요약 / Insights 추출**(INSIGHTS-001).
- **LLM 호출**(PII 재검출 등 고급 redact): 본 SPEC의 Redactor는 정규식/패턴 기반만. LLM 기반 NER은 향후 별도 SPEC.
- **Trajectory 원격 전송**(Federated Learning용): PRIVACY-001 또는 Federated SPEC(현재 OUT OF SCOPE).
- **다중 사용자 격리**: Phase 4 MVP는 단일 사용자 로컬 홈(`~/.goose/`). 멀티테넌시는 Phase 7+.
- **암호화 저장**: 본 SPEC은 파일 시스템 권한(0600)만. 전체 디스크 암호화는 OS/디바이스 레이어.
- **Retention 정책의 법적 준수**(GDPR 우편함 요청 등): 인터페이스만 제공, 법적 workflow는 별도 SPEC.
- **Trajectory 재생(replay)**: 디버깅 용도의 재생은 별도 개발자 도구. 본 SPEC은 기록 only.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-TRAJECTORY-001 [Ubiquitous]** — The `TrajectoryCollector` **shall** maintain a 1:1 correspondence between a QueryEngine session (identified by `session_id`) and a single in-memory trajectory buffer until the session terminates.

**REQ-TRAJECTORY-002 [Ubiquitous]** — Every `TrajectoryEntry` written to disk **shall** conform to the ShareGPT schema: exactly one of `{"from": "system" | "human" | "gpt" | "tool"}` and a non-empty `value` string.

**REQ-TRAJECTORY-003 [Ubiquitous]** — The `Writer` **shall** open trajectory files with POSIX mode `0600` (owner read/write only) and create the parent directory with mode `0700` if absent.

**REQ-TRAJECTORY-004 [Ubiquitous]** — The `Redactor` chain **shall** be applied to every `TrajectoryEntry.value` exactly once, before the entry is serialized for disk write (not after — to prevent PII ever hitting the filesystem).

### 4.2 Event-Driven (이벤트 기반)

**REQ-TRAJECTORY-005 [Event-Driven]** — **When** the QueryEngine's Terminal event fires with `success: true`, the `TrajectoryCollector` **shall** flush the session buffer to `${GOOSE_HOME}/trajectories/success/YYYY-MM-DD.jsonl` (date computed in UTC).

**REQ-TRAJECTORY-006 [Event-Driven]** — **When** the QueryEngine's Terminal event fires with `success: false`, the `TrajectoryCollector` **shall** flush the session buffer to `${GOOSE_HOME}/trajectories/failed/YYYY-MM-DD.jsonl` with `TrajectoryMetadata.failure_reason` populated from the Terminal `error` field.

**REQ-TRAJECTORY-007 [Event-Driven]** — **When** a file's cumulative byte count exceeds `max_file_bytes` (default 10,485,760 = 10MB) during append, the `Writer` **shall** rotate by closing the current file and opening `YYYY-MM-DD-{N}.jsonl` where N starts at 1 and monotonically increases.

**REQ-TRAJECTORY-008 [Event-Driven]** — **When** the UTC date rolls over (detected at write time), the `Writer` **shall** close the previous day's file handle and open the new day's file (natural rotation).

**REQ-TRAJECTORY-009 [Event-Driven]** — **When** the `Retention.Sweep()` scheduler fires (daily at 03:00 local time by default), files older than `retention_days` (default 90) **shall** be deleted.

**REQ-TRAJECTORY-010 [Event-Driven]** — **When** the disk write fails (ENOSPC, EACCES, EIO), the `Writer` **shall** log a structured zap warning with `{session_id, path, error}` and **shall not** propagate the error to the QueryEngine goroutine.

### 4.3 State-Driven (상태 기반)

**REQ-TRAJECTORY-011 [State-Driven]** — **While** `config.telemetry.trajectory.enabled == false`, the `TrajectoryCollector` **shall** be a no-op (receive events but neither buffer nor write).

**REQ-TRAJECTORY-012 [State-Driven]** — **While** a session buffer exceeds `in_memory_turn_cap` (default 1000 turns) without terminating, the `TrajectoryCollector` **shall** spill the oldest half to disk as a partial `.jsonl` fragment tagged `{partial: true}` to bound memory usage.

### 4.4 Unwanted Behavior (방지)

**REQ-TRAJECTORY-013 [Unwanted]** — The `TrajectoryCollector` **shall not** block the QueryEngine goroutine for more than 1ms per event dispatch; all I/O **shall** occur on the collector's dedicated worker goroutine.

**REQ-TRAJECTORY-014 [Unwanted]** — **If** a Redact rule throws during application (malformed input, panic in regex), the `Redactor` chain **shall** catch the panic, replace the entry's value with the literal string `"<REDACT_FAILED>"`, and log a zap error — **shall not** terminate the collector.

**REQ-TRAJECTORY-015 [Unwanted]** — The `Writer` **shall not** interleave bytes from two different sessions within the same `.jsonl` record; each trajectory's serialized bytes **shall** be written with a single `write()` syscall (or retry on partial write).

**REQ-TRAJECTORY-016 [Unwanted]** — The `Redactor` **shall not** mutate `TrajectoryEntry` values that originated from `{"from": "system"}` messages (preserving system prompts unchanged for reproducibility), **unless** the rule is explicitly tagged `applies_to_system: true`.

### 4.5 Optional (선택적)

**REQ-TRAJECTORY-017 [Optional]** — **Where** `config.telemetry.trajectory.redact_rules` is provided, the `Redactor` chain **shall** load user-supplied regex rules in addition to the 6 built-in rules, and rules **shall** be applied in config-declaration order followed by built-ins.

**REQ-TRAJECTORY-018 [Optional]** — **Where** `TrajectoryMetadata.tags` is non-empty, the tag set **shall** be persisted as a JSON array field on every trajectory record for downstream filtering (e.g. `["skill:code-review", "model:anthropic"]`).

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `*_test.go`로 변환 가능한 수준.

**AC-TRAJECTORY-001 — ShareGPT 스키마 준수**
- **Given** `TrajectoryCollector`가 `{role: human, value:"hi"}` + `{role: gpt, value:"hello"}` 두 턴 수신
- **When** Terminal(success:true) 발화 후 `~/.goose/trajectories/success/2026-04-21.jsonl` 읽기
- **Then** 한 줄 JSON이 `{"conversations":[{"from":"human","value":"hi"},{"from":"gpt","value":"hello"}],"timestamp":"...","model":"...","completed":true}` 스키마 매칭, `from` 필드가 허용된 4종 외 값 없음

**AC-TRAJECTORY-002 — 실패 궤적 분리 저장**
- **Given** QueryEngine이 `Terminal{success:false, error:"context_overflow"}`로 종료
- **When** Writer flush
- **Then** `~/.goose/trajectories/failed/2026-04-21.jsonl`에 기록, `TrajectoryMetadata.failure_reason == "context_overflow"` 필드 존재, `success/` 디렉토리에는 동일 session_id 기록 없음

**AC-TRAJECTORY-003 — 이메일 redact**
- **Given** 사용자 메시지 `"내 이메일은 alice@example.com 이야"`
- **When** 궤적 파일 inspect
- **Then** 디스크의 value가 `"내 이메일은 <REDACTED:email> 이야"` (치환됨), 원본 문자열 `alice@example.com`이 디스크에 존재하지 않음(grep 검증)

**AC-TRAJECTORY-004 — API 키 redact (6종 기본 규칙)**
- **Given** `value: "sk-proj-abc123... AWS_SECRET=XYZ Bearer eyJhbGci... 1234-5678-9012-3456 010-1234-5678 /home/alice/.ssh"`
- **When** redact 적용
- **Then** 모든 6종(OpenAI key, AWS 패턴, Bearer JWT, credit card Luhn, KR phone, unix home username)이 `<REDACTED:*>`로 치환됨

**AC-TRAJECTORY-005 — 파일 회전(크기)**
- **Given** `max_file_bytes=1024` (테스트용 작게), 단일 날짜에 2KB 분량의 궤적 append
- **When** 두 번째 1KB 기록 시점
- **Then** `2026-04-21.jsonl`(첫 1KB) + `2026-04-21-1.jsonl`(두 번째 1KB) 두 파일 존재

**AC-TRAJECTORY-006 — 날짜 rollover**
- **Given** 자정 전 1분, 세션 지속 중. Mock clock을 23:59:58로 설정
- **When** 자정(00:00:02) 이후 새 turn 기록
- **Then** `2026-04-21.jsonl`(자정 이전) + `2026-04-22.jsonl`(자정 이후) 두 파일에 분할 기록, 각 파일 첫/마지막 timestamp가 날짜 경계를 넘지 않음

**AC-TRAJECTORY-007 — 설정 비활성화**
- **Given** `config.telemetry.trajectory.enabled = false`
- **When** QueryEngine 1턴 실행
- **Then** `~/.goose/trajectories/` 디렉토리가 생성되지 않거나 빈 상태, Writer goroutine 미생성(runtime.NumGoroutine delta == 0)

**AC-TRAJECTORY-008 — Retention 90일 sweep**
- **Given** `retention_days=30`, 31일 전 타임스탬프 파일 + 29일 전 파일 각 1개 사전 배치
- **When** `Retention.Sweep()` 호출
- **Then** 31일 전 파일은 삭제, 29일 전 파일은 보존, Writer가 아직 열어둔 오늘 파일은 영향 없음

**AC-TRAJECTORY-009 — 쓰기 실패 격리**
- **Given** writer 대상 디렉토리에 쓰기 권한 제거 (`chmod 0500`)
- **When** QueryEngine Terminal 이벤트 발화
- **Then** QueryEngine goroutine은 blocked되지 않고 정상 종료, zap warning 로그 1건 `"trajectory write failed"` 기록, 프로세스는 계속 실행

**AC-TRAJECTORY-010 — 시스템 프롬프트 redact 스킵**
- **Given** `{from:"system", value:"You are Goose. Email support@goose.ai for help."}`
- **When** redact 적용 (기본 규칙 `applies_to_system: false`)
- **Then** `support@goose.ai`는 치환되지 않음(시스템 프롬프트 재현성 보존)

**AC-TRAJECTORY-011 — 버퍼 spill (1000턴 초과)**
- **Given** `in_memory_turn_cap=100` (테스트용), 단일 세션에서 150턴 누적
- **When** 101번째 턴 도착
- **Then** 50턴이 `${GOOSE_HOME}/trajectories/success/2026-04-21.jsonl`에 `{partial:true}` 플래그로 append, 메모리 버퍼는 50턴으로 축소

**AC-TRAJECTORY-012 — 동시성 무결성**
- **Given** 10개 QueryEngine 인스턴스가 병렬로 각 10턴씩 종료
- **When** 모든 flush 완료 후 `2026-04-21.jsonl` 파싱
- **Then** 10개 JSON-L 라인 각각이 valid JSON, 라인 간 바이트 혼재 없음(각 라인이 독립적으로 `json.Unmarshal` 성공)

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/
└── learning/
    └── trajectory/
        ├── collector.go            # TrajectoryCollector + hook 등록
        ├── collector_test.go
        ├── types.go                # Trajectory, TrajectoryEntry, Role, Metadata
        ├── writer.go               # Append-only JSON-L writer + rotation
        ├── writer_test.go
        ├── rotation.go             # 크기/날짜 기반 회전
        ├── retention.go            # 90일 sweep
        ├── config.go               # TelemetryConfig 매핑
        └── redact/
            ├── redactor.go         # Redactor interface + chain
            ├── rules.go            # 6 built-in rules
            ├── rules_test.go       # 각 rule별 positive/negative case
            └── config.go           # 사용자 규칙 로드
```

### 6.2 핵심 타입 (Go 시그니처 제안)

```go
// internal/learning/trajectory/types.go

// Role은 ShareGPT 호환 4종.
type Role string
const (
    RoleSystem Role = "system"
    RoleHuman  Role = "human"
    RoleGPT    Role = "gpt"
    RoleTool   Role = "tool"
)

// TrajectoryEntry는 ShareGPT의 conversations 배열 원소.
type TrajectoryEntry struct {
    From  Role   `json:"from"`
    Value string `json:"value"`
}

// Trajectory는 한 세션의 전체 대화 + 메타.
type Trajectory struct {
    Conversations []TrajectoryEntry   `json:"conversations"`
    Timestamp     time.Time           `json:"timestamp"`
    Model         string              `json:"model"`                // e.g. "anthropic/claude-opus-4-7"
    Completed     bool                `json:"completed"`
    SessionID     string              `json:"session_id"`
    Metadata      TrajectoryMetadata  `json:"metadata,omitempty"`
}

type TrajectoryMetadata struct {
    Tags           []string      `json:"tags,omitempty"`
    FailureReason  string        `json:"failure_reason,omitempty"` // Terminal.error
    Partial        bool          `json:"partial,omitempty"`        // buffer spill
    TurnCount      int           `json:"turn_count"`
    DurationMs     int64         `json:"duration_ms"`
    TokensInput    int           `json:"tokens_input,omitempty"`
    TokensOutput   int           `json:"tokens_output,omitempty"`
}


// internal/learning/trajectory/collector.go

type Collector struct {
    cfg       TelemetryConfig
    buffers   map[string]*sessionBuffer   // session_id -> buffer
    mu        sync.RWMutex
    writer    *Writer
    redactor  redact.Chain
    events    chan hookEvent               // QueryEngine hook 수신
    done      chan struct{}
    logger    *zap.Logger
}

type sessionBuffer struct {
    sessionID string
    entries   []TrajectoryEntry
    startedAt time.Time
    model     string
    mu        sync.Mutex
}

// New는 Collector + worker goroutine 기동.
// QueryEngine config의 PostSamplingHooks / StopFailureHooks에 등록할 어댑터 반환.
func New(cfg TelemetryConfig, logger *zap.Logger) (*Collector, QueryHookAdapter, error)

// OnTurn은 PostSamplingHooks에서 턴당 1회 호출.
func (c *Collector) OnTurn(sessionID string, entries []TrajectoryEntry)

// OnTerminal은 Terminal 이벤트 수신 시 호출. buffer flush.
func (c *Collector) OnTerminal(sessionID string, success bool, meta TrajectoryMetadata)

// Close는 graceful shutdown. drain + file close.
func (c *Collector) Close(ctx context.Context) error


// internal/learning/trajectory/writer.go

type Writer struct {
    baseDir       string
    maxFileBytes  int64
    currentFiles  map[string]*openFile  // "success"|"failed" -> file
    mu            sync.Mutex
    logger        *zap.Logger
}

type openFile struct {
    path         string
    file         *os.File
    bytesWritten int64
    rotationIdx  int
    dateStr      string
}

func (w *Writer) WriteTrajectory(t *Trajectory) error  // best-effort, logs on fail


// internal/learning/trajectory/redact/redactor.go

type Rule struct {
    Name            string
    Pattern         *regexp.Regexp
    Replacement     string              // "<REDACTED:email>" 등
    AppliesToSystem bool                // 기본 false
}

type Chain struct {
    rules []Rule
}

func (c *Chain) Apply(entry *TrajectoryEntry) {
    // panics caught, AppliesToSystem 플래그 체크
}

// BuiltinRules는 6종 기본 규칙.
func BuiltinRules() []Rule {
    return []Rule{
        {Name: "email",       Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
         Replacement: "<REDACTED:email>"},
        {Name: "openai_key",  Pattern: regexp.MustCompile(`sk-[A-Za-z0-9\-_]{20,}`),
         Replacement: "<REDACTED:api_key>"},
        {Name: "bearer_jwt",  Pattern: regexp.MustCompile(`Bearer\s+ey[A-Za-z0-9\-_\.]+`),
         Replacement: "Bearer <REDACTED:jwt>"},
        {Name: "credit_card", Pattern: regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`),
         Replacement: "<REDACTED:cc>"},   // 후처리에서 Luhn 체크
        {Name: "kr_phone",    Pattern: regexp.MustCompile(`\b01[016789]-\d{3,4}-\d{4}\b`),
         Replacement: "<REDACTED:phone>"},
        {Name: "home_path",   Pattern: regexp.MustCompile(`(/Users|/home)/[a-zA-Z][a-zA-Z0-9_-]{1,30}`),
         Replacement: "$1/<REDACTED:user>"},
    }
}
```

### 6.3 QueryEngine 통합 계약

본 SPEC은 SPEC-GOOSE-QUERY-001의 `PostSamplingHooks`, `StopFailureHooks`, Terminal 이벤트를 consumer로 사용한다. 주입 책임은 GOOSE 부트스트랩(CORE-001 + 본 SPEC 등록자)에 있다.

| QueryEngine 훅 | 본 SPEC 핸들러 | 용도 |
|---|---|---|
| `PostSamplingHooks` | `Collector.OnSampledMessage` | assistant/tool 메시지 수신 → buffer append |
| StopFailureHooks | `Collector.OnFailure` | 오류 시 failed 버킷 예약 |
| Terminal SDKMessage | `Collector.OnTerminal` | success/failed 분기 + flush |

**훅 등록 예시** (bootstrap 쪽 코드, 본 SPEC의 scope 아님):

```go
collector, adapter, _ := trajectory.New(cfg, logger)
queryCfg.PostSamplingHooks = append(queryCfg.PostSamplingHooks, adapter.PostSampling)
queryCfg.StopFailureHooks  = append(queryCfg.StopFailureHooks,  adapter.StopFailure)
// Terminal은 engine 내부 이벤트이므로 engine.Subscribe(collector.OnTerminal)
```

### 6.4 Role 매핑 정책

QueryEngine의 `message.Role`을 Trajectory의 `Role`로 매핑:

| QueryEngine Role | Trajectory Role | 비고 |
|---|---|---|
| `"system"` | `RoleSystem` | 시스템 프롬프트(redact 스킵 대상) |
| `"user"` | `RoleHuman` | ShareGPT 용어로 변환 |
| `"assistant"` | `RoleGPT` | ShareGPT 용어 유지 (historical naming) |
| `tool_use` / `tool_result` content block | `RoleTool` | 실행된 도구의 input/output을 하나의 entry로 concat |

### 6.5 파일 레이아웃

```
~/.goose/                                    # GOOSE_HOME
├── trajectories/
│   ├── success/
│   │   ├── 2026-04-21.jsonl                 # 기본 날짜별
│   │   ├── 2026-04-21-1.jsonl               # 10MB 초과 시 rotation
│   │   └── 2026-04-22.jsonl                 # 자정 rollover
│   └── failed/
│       └── 2026-04-21.jsonl
└── config.yaml
```

### 6.6 Redact 규칙 우선순위

1. 사용자 정의 규칙(`config.telemetry.trajectory.redact_rules`, 선언 순)
2. 6 built-in 규칙(위 §6.2 순서: email → openai_key → bearer_jwt → credit_card → kr_phone → home_path)

우선순위 결정 근거: 사용자가 도메인별 PII(예: 사내 사번)를 **먼저** 삭제해야 하위 규칙이 혼동되지 않음.

### 6.7 TDD 진입 순서

1. **RED #1**: `TestRedactRule_Email_ReplacesCanonicalForm` — AC-TRAJECTORY-003.
2. **RED #2**: `TestRedactRule_SixBuiltinsAllFire` — AC-TRAJECTORY-004.
3. **RED #3**: `TestRedactChain_SystemRoleSkippedByDefault` — AC-TRAJECTORY-010.
4. **RED #4**: `TestCollector_OnTerminalSuccess_WritesToSuccessDir` — AC-TRAJECTORY-001.
5. **RED #5**: `TestCollector_OnTerminalFailure_WritesToFailedDir` — AC-TRAJECTORY-002.
6. **RED #6**: `TestWriter_RotatesOnMaxBytes` — AC-TRAJECTORY-005.
7. **RED #7**: `TestWriter_RolloverOnDateChange` — AC-TRAJECTORY-006 (`clockwork` mock clock).
8. **RED #8**: `TestCollector_DisabledIsNoop` — AC-TRAJECTORY-007.
9. **RED #9**: `TestRetention_SweepOldFiles` — AC-TRAJECTORY-008.
10. **RED #10**: `TestWriter_WritePermissionDeniedDoesNotBlock` — AC-TRAJECTORY-009.
11. **RED #11**: `TestCollector_SpillOnBufferCap` — AC-TRAJECTORY-011.
12. **RED #12**: `TestConcurrentSessions_NoInterleaving` (10 goroutines, `-race`) — AC-TRAJECTORY-012.
13. **GREEN**: 최소 구현 + `golangci-lint run`.
14. **REFACTOR**: `redact/rules.go`의 규칙 표 기반 정규화, `writer.go`의 rotation을 전략 패턴화.

### 6.8 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 12개 AC 전부 integration test, `-race` 통과, redact 규칙별 positive/negative 케이스 |
| **R**eadable | 패키지 분리(collector vs writer vs redact), Role enum으로 ShareGPT 어휘 명시 |
| **U**nified | `go fmt` + `golangci-lint` (errcheck, govet, staticcheck), 모든 I/O는 `Writer` 단일 경로 |
| **S**ecured | 파일 모드 0600, PII redact 기본 6종, 쓰기 실패 best-effort(DoS 방지), home-dir username redact |
| **T**rackable | 모든 Trajectory에 `session_id` + `timestamp`, zap 구조화 로그 (warn/error 레벨) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | `GOOSE_HOME` 해석, zap 로거, context 루트 |
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `PostSamplingHooks`, `StopFailureHooks`, Terminal 이벤트, `SessionID` |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `config.telemetry.trajectory.*` 로드 |
| 후속 SPEC | SPEC-GOOSE-COMPRESSOR-001 | 본 SPEC의 `Trajectory` 타입을 입력으로 |
| 후속 SPEC | SPEC-GOOSE-INSIGHTS-001 | `.jsonl` 파일을 입력으로 스캔 |
| 후속 SPEC | SPEC-GOOSE-MEMORY-001 | `SessionID` 공유 |
| 외부 | Go 1.22+ | regexp, encoding/json, os, sync |
| 외부 | `go.uber.org/zap` v1.27+ | CORE-001 계승 |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |
| 외부 (테스트) | `github.com/jonboulle/clockwork` v0.4+ | 가상 시계 (AC-006 날짜 rollover) |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Redact 정규식의 false negative (새로운 API 키 포맷, 국가별 전화번호) | 고 | 고 | 사용자 정의 규칙(REQ-017)으로 확장 창구, 6 built-in은 보수적 기본값. 분기별 rule 리뷰를 `.moai/project/security.md`에 선언 |
| R2 | Redact 정규식의 false positive (신용카드 Luhn 오탐) | 중 | 중 | credit_card 규칙은 Luhn 체크 후처리로 보강. 다른 규칙도 test suite에 negative case 포함 |
| R3 | QueryEngine critical path에 1ms 이상 블로킹 | 중 | 고 | REQ-TRAJECTORY-013 강제. `OnTurn`은 buffered channel에 즉시 send만. Worker goroutine이 모든 I/O 처리 |
| R4 | 디스크 가득 참(ENOSPC) 시 이후 모든 세션 silent drop | 중 | 중 | REQ-TRAJECTORY-010으로 warn 로그, Retention 90일 자동 삭제. 선택적으로 `max_total_bytes` 한도(향후 SPEC 확장) |
| R5 | 날짜 rollover 시 race(22:59:59.999에 2개 goroutine 동시 기록) | 낮 | 중 | `Writer.mu`로 직렬화, date 계산을 write 시점 1회만 |
| R6 | 멀티 사용자 홈(공유 서버)에서 다른 사용자 Trajectory 노출 | 낮 | 고 | 파일 mode 0600 + 디렉토리 0700 강제(REQ-TRAJECTORY-003). 단일 사용자 전제(Phase 4 scope) |
| R7 | Redact 후 trajectory가 downstream(LoRA)에서 학습 가치 저하 | 중 | 중 | redact 태그(`<REDACTED:kind>`)가 카테고리 정보 보존. 훈련 시 학습 대상 외(target masking) |
| R8 | `failed/` 디렉토리에 누적되는 실패 궤적이 compressor/insights 부담 | 중 | 낮 | INSIGHTS-001이 failed/success 비율 집계. Retention은 동일 정책 |
| R9 | ShareGPT 스키마가 tool_use/tool_result의 구조화 데이터(JSON)를 단일 string value로 평탄화해서 정보 손실 | 중 | 중 | `TrajectoryMetadata.tags`에 `tool:*` 태그로 보완. 구조화 보존은 Phase 7+에서 확장 스키마 도입 고려 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-learning.md` §1 학습 파이프라인 E2E, §2 Trajectory 스키마
- `.moai/project/learning-engine.md` §1.1 Short-term Learning (세션 수준), §9.1 `internal/learning/` 구조
- `.moai/project/adaptation.md` §10.1-10.2 Identity Graph / LoRA 데이터 연동
- `.moai/specs/ROADMAP.md` §4 Phase 4 #19, §13 핵심 설계 원칙 5
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` §4.5 REQ-QUERY-018 (PostSamplingHooks)

### 9.2 외부 참조

- **ShareGPT format**: https://huggingface.co/datasets/anon8231489123/ShareGPT_Vicuna_unfiltered (표준 reference)
- **Hermes source**: `./hermes-agent-main/agent/trajectory.py`, `./hermes-agent-main/trajectory_compressor.py` (상단 스키마 정의)
- **ISO 8601 UTC**: https://www.iso.org/iso-8601-date-and-time-format.html
- **Go regexp RE2 syntax**: https://pkg.go.dev/regexp/syntax (catastrophic backtracking 없음 → redact 안전)

### 9.3 부속 문서

- `./research.md` — hermes-learning.md §2 원문 인용 + Go 이식 결정 + TDD 전략
- `../SPEC-GOOSE-COMPRESSOR-001/spec.md` — 본 SPEC의 `Trajectory` 소비자
- `../SPEC-GOOSE-INSIGHTS-001/spec.md` — `.jsonl` 파일 스캐너

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **Trajectory 압축/요약을 구현하지 않는다**. COMPRESSOR-001.
- 본 SPEC은 **Insights 분석 / 통계 / 시각화를 구현하지 않는다**. INSIGHTS-001.
- 본 SPEC은 **Memory Provider 저장을 구현하지 않는다**. MEMORY-001.
- 본 SPEC은 **Error 분류를 구현하지 않는다**(저장만). ERROR-CLASS-001.
- 본 SPEC은 **LLM 기반 NER 고급 redact를 포함하지 않는다**(regex/패턴 only).
- 본 SPEC은 **Trajectory 원격 전송(Federated / Cloud upload)을 구현하지 않는다**.
- 본 SPEC은 **Trajectory 재생(replay) UI/CLI 도구를 포함하지 않는다**.
- 본 SPEC은 **암호화 저장을 구현하지 않는다**(OS file permission 0600 only).
- 본 SPEC은 **다중 사용자 / 멀티테넌시 격리를 포함하지 않는다**(단일 사용자 `~/.goose/`).
- 본 SPEC은 **LoRA 훈련 데이터셋 변환을 포함하지 않는다**. LORA-001.

---

**End of SPEC-GOOSE-TRAJECTORY-001**
