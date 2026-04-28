# SPEC-GOOSE-LLM-001 — Amendment v0.2

## Amendment: CLI Subprocess Providers (Layer 3)

| 필드 | 값 |
|-----|-----|
| Amendment Version | 0.2.0 |
| Base SPEC Version | 0.1.0 |
| Status | planned |
| Created | 2026-04-29 |
| Author | manager-spec |
| Priority | P1 |
| Phase | 1 |

---

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.2.0 | 2026-04-29 | CLI subprocess provider (Layer 3) 추가 — Claude CLI, Gemini CLI, Codex CLI를 LLMProvider 인터페이스로 래핑 | manager-spec |

---

## 1. 개요 (Overview)

본 수정안(amendment)은 SPEC-GOOSE-LLM-001 v0.1.0에 **CLI subprocess provider 레이어(Layer 3)**를 추가한다. 기존 SPEC은 `LLMProvider` 인터페이스와 Ollama HTTP 어댑터(Layer 1)를 정의했으며, 후속 SPEC-GOOSE-ADAPTER-001/002가 SDK 기반 cloud provider(Layer 2)를 추가했다. 본 수정안은 CLI 도구를 subprocess로 실행하여 동일한 `LLMProvider` 인터페이스를 구현하는 **Layer 3**를 도입한다.

Layer 3가 필요한 이유:

- **Claude Code CLI**, **Gemini CLI**, **Codex CLI** 등의 AI 도구는 HTTP API가 아닌 CLI 명령으로만 접근 가능한 경우가 있다.
- API key가 CLI 도구에 이미 설정되어 있어 별도 인증 관리가 불필요하다.
- CLI 도구의 응답 형식이 HTTP API와 다를 수 있어 전용 파서가 필요하다.

---

## 2. 변경 사항 요약

### 2.1 추가되는 요구사항

기존 REQ-LLM-001 ~ REQ-LLM-015 이후에 다음 요구사항을 추가한다:

### 2.2 EARS 요구사항 (Requirements)

#### Ubiquitous

**REQ-LLM-020 [Ubiquitous]** — The system **shall** support CLI subprocess providers that implement the `LLMProvider` interface defined in REQ-LLM-001, treating CLI tool invocations as first-class LLM providers. When a CLI tool does not support a native streaming mode, `Stream()` **shall** execute `Complete()` internally and return the entire response as a single-chunk stream (one `Chunk` with `Done: true`), setting `Capabilities().Streaming = false`.

**REQ-LLM-024 [Ubiquitous]** — All CLI providers **shall** implement the same `LLMProvider` interface methods: `Name()`, `Complete()`, `Stream()`, `Capabilities()`, `Close()`. `Capabilities()` **shall** return a `Capabilities` struct where `Streaming` is `true` if the CLI tool supports `--output-format stream-json` (or equivalent) and `false` otherwise; `MaxTokens` **shall** be set to 0 (unknown) unless the CLI tool reports a limit.

#### Event-Driven

**REQ-LLM-021 [Event-Driven]** — **When** `ClaudeCLIProvider.Complete()` is invoked, the system **shall** execute `claude -p "{prompt}" --output-format json` as a subprocess, parse the JSON response, and return a `CompletionResponse`.

**REQ-LLM-022 [Event-Driven]** — **When** `GeminiCLIProvider.Complete()` is invoked, the system **shall** execute `gemini -p "{prompt}" --output-format json` as a subprocess, parse the JSON response, and return a `CompletionResponse`.

**REQ-LLM-023 [Event-Driven]** — **When** `CodexCLIProvider.Complete()` is invoked, the system **shall** execute `codex exec "{prompt}"` as a subprocess, parse the response, and return a `CompletionResponse`.

**REQ-LLM-026 [Event-Driven]** — **When** the system starts and `llm.delegation.enabled` is `true`, the system **shall** auto-detect CLI tool availability by checking if each configured CLI path is executable via `os.Stat` + executable bit check.

#### State-Driven

**REQ-LLM-025 [State-Driven]** — **While** a CLI subprocess is running, the provider **shall** respect context cancellation by sending `SIGTERM` to the subprocess within 100ms of context cancellation; if the subprocess does not exit within an additional 5s, it **shall** be killed with `SIGKILL`.

#### Unwanted Behavior

**REQ-LLM-025a [Unwanted]** — **If** the CLI tool is not found at the configured path, **then** the provider **shall** return `ErrServerUnavailable` with a message indicating the missing tool; it **shall not** attempt to install the tool.

**REQ-LLM-025b [Unwanted]** — **If** the CLI subprocess exits with a non-zero status code, **then** the provider **shall** map the exit code to the appropriate `LLMError` subclass:
- Exit code 1 (general error): `ErrInvalidRequest`
- Exit code 2 (auth error): `ErrUnauthorized`
- Signal killed: `ErrServerUnavailable`
- Timeout: `ErrServerUnavailable`

**REQ-LLM-025c [Unwanted]** — **If** the CLI output cannot be parsed as valid JSON (for `--output-format json` mode), **then** the provider **shall** return `ErrInvalidRequest` with the raw output truncated to 500 characters for debugging.

---

## 3. 수용 기준 (Acceptance Criteria)

### AC-LLM-020: ClaudeCLIProvider Complete

- **Given** `claude` CLI가 `/usr/local/bin/claude`에 존재하고 정상 응답
- **Given** `ClaudeCLIProvider`가 `cli_path: "/usr/local/bin/claude"`로 초기화됨
- **When** `provider.Complete(ctx, CompletionRequest{Model: "claude-sonnet-4", Messages: [...]})` 호출
- **Then** subprocess가 `claude -p "{prompt}" --output-format json` 명령으로 실행됨
- **Then** `resp.Text`가 JSON 파싱된 응답의 content 필드와 일치함
- **Then** `resp.Usage`가 JSON 파싱된 usage 필드에서 추출됨 (불가 시 `Usage{Unknown: true}`)
- **Satisfies**: REQ-LLM-021, REQ-LLM-024

### AC-LLM-021: GeminiCLIProvider Complete

- **Given** `gemini` CLI가 존재하고 정상 응답
- **When** `GeminiCLIProvider.Complete()` 호출
- **Then** subprocess가 `gemini -p "{prompt}" --output-format json` 명령으로 실행됨
- **Then** JSON 파싱된 응답 반환
- **Satisfies**: REQ-LLM-022

### AC-LLM-022: CodexCLIProvider Complete

- **Given** `codex` CLI가 존재하고 정상 응답
- **When** `CodexCLIProvider.Complete()` 호출
- **Then** subprocess가 `codex exec "{prompt}"` 명령으로 실행됨
- **Then** 응답 파싱 후 `CompletionResponse` 반환
- **Satisfies**: REQ-LLM-023

### AC-LLM-023: CLI not found

- **Given** `claude` CLI가 `/usr/local/bin/claude`에 존재하지 않음
- **When** `ClaudeCLIProvider.Complete()` 호출
- **Then** `errors.Is(err, ErrServerUnavailable) == true`
- **Then** 에러 메시지에 "claude CLI not found at /usr/local/bin/claude" 포함
- **Satisfies**: REQ-LLM-025a

### AC-LLM-024: CLI subprocess timeout

- **Given** `claude` CLI가 응답하지 않음 (10초 이상 대기)
- **Given** context에 5초 timeout 설정
- **When** `provider.Complete(ctx, req)` 호출
- **Then** 5초 후 context cancel 전파
- **Then** subprocess에 SIGTERM 전송
- **Then** `errors.Is(err, ErrServerUnavailable) == true`
- **Satisfies**: REQ-LLM-025

### AC-LLM-024a: SIGTERM followed by SIGKILL

- **Given** `claude` CLI subprocess가 SIGTERM에도 종료되지 않음
- **Given** context에 5초 timeout 설정
- **When** `provider.Complete(ctx, req)` 호출 후 context cancel
- **Then** SIGTERM 전송 후 5초 이내 프로세스 미종료 시 SIGKILL 전송
- **Then** `cmd.Wait()` 호출로 좀비 프로세스 방지
- **Then** `errors.Is(err, ErrServerUnavailable) == true`
- **Satisfies**: REQ-LLM-025

### AC-LLM-025: CLI malformed output

- **Given** `claude` CLI가 유효하지 않은 JSON 반환
- **When** `ClaudeCLIProvider.Complete()` 호출
- **Then** `errors.Is(err, ErrInvalidRequest) == true`
- **Then** 에러 메시지에 raw output (500자 제한) 포함
- **Satisfies**: REQ-LLM-025c

### AC-LLM-025a: CLI exit code to LLMError mapping

- **Given** `claude` CLI가 exit code 1로 종료
- **When** `ClaudeCLIProvider.Complete()` 호출
- **Then** `errors.Is(err, ErrInvalidRequest) == true`
- **Given** `claude` CLI가 exit code 2로 종료
- **When** `ClaudeCLIProvider.Complete()` 호출
- **Then** `errors.Is(err, ErrUnauthorized) == true`
- **Given** `claude` CLI가 signal로 종료
- **When** `ClaudeCLIProvider.Complete()` 호출
- **Then** `errors.Is(err, ErrServerUnavailable) == true`
- **Satisfies**: REQ-LLM-025b

### AC-LLM-026: Auto-detect CLI availability

- **Given** `llm.delegation.enabled: true`
- **Given** `cli_paths: {claude: "/usr/local/bin/claude", gemini: "/usr/local/bin/gemini"}`
- **Given** `claude`는 존재, `gemini`는 미존재
- **When** 시스템 시작 시 auto-detect 실행
- **Then** `claude` provider는 available로 등록
- **Then** `gemini` provider는 경고 로그와 함께 unavailable로 표시
- **Satisfies**: REQ-LLM-026

### AC-LLM-027: CLI provider in registry

- **Given** `ClaudeCLIProvider`가 초기화됨
- **When** `registry.Get("claude-cli")` 호출
- **Then** `ClaudeCLIProvider` 인스턴스 반환
- **Then** `provider.Name() == "claude-cli"`
- **Satisfies**: REQ-LLM-020

---

## 4. 기술적 접근 (Technical Approach)

### 4.1 패키지 레이아웃

```
internal/llm/
├── provider.go              # 기존 — 변경 없음
├── registry.go              # 기존 — factory에 "claude-cli", "gemini-cli", "codex-cli" 추가
├── cli/                     # 신규 — CLI subprocess providers
│   ├── cli.go               # 공통 BaseCLIProvider (subprocess 실행, 파싱)
│   ├── claude.go            # ClaudeCLIProvider
│   ├── gemini.go            # GeminiCLIProvider
│   ├── codex.go             # CodexCLIProvider
│   ├── parser.go            # JSON 출력 파서
│   ├── detect.go            # CLI 가용성 자동 감지
│   └── *_test.go
└── ... (기존 패키지 유지)
```

### 4.2 설정 스키마 추가

```yaml
llm:
  delegation:
    enabled: true
    cli_paths:
      claude: /usr/local/bin/claude
      gemini: /usr/local/bin/gemini
      codex: /usr/local/bin/codex
    timeout: 60s
```

### 4.3 핵심 타입

```go
// BaseCLIProvider provides shared CLI subprocess execution logic.
type BaseCLIProvider struct {
    name     string
    cliPath  string
    timeout  time.Duration
    logger   *zap.Logger
}

// Exec runs the CLI command and returns raw output.
func (p *BaseCLIProvider) Exec(ctx context.Context, args []string, stdin string) ([]byte, error)

// ClaudeCLIProvider implements LLMProvider via Claude CLI.
type ClaudeCLIProvider struct {
    BaseCLIProvider
}

func (p *ClaudeCLIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
func (p *ClaudeCLIProvider) Stream(ctx context.Context, req CompletionRequest) (StreamReader, error)
func (p *ClaudeCLIProvider) Capabilities(ctx context.Context, model string) (Capabilities, error)
func (p *ClaudeCLIProvider) Close() error
```

### 4.4 CLI 명령 매핑

| Provider | Complete Command | Stream Command |
|----------|-----------------|----------------|
| ClaudeCLI | `claude -p "{prompt}" --output-format json` | `claude -p "{prompt}" --output-format stream-json` |
| GeminiCLI | `gemini -p "{prompt}" --output-format json` | `gemini -p "{prompt}" --output-format stream-json` |
| CodexCLI | `codex exec "{prompt}"` | `codex exec --stream "{prompt}"` |

### 4.5 응답 파싱

- `--output-format json` 응답에서 `content`, `usage`, `model` 필드 추출.
- 필드 누락 시 기본값 사용 (`Usage{Unknown: true}`).
- 파싱 실패 시 `ErrInvalidRequest` + raw output (500자 제한).

### 4.6 TRUST 5 매핑

| 차원 | 달성 |
|-----|------|
| Tested | `exec.Command` mock, CLI 출력 fixture, timeout 테스트 |
| Readable | 각 CLI provider에 doc-comment로 명령 매핑 명시 |
| Unified | 기존 `LLMError` 계층 재사용, 설정 구조 일관성 |
| Secured | prompt escaping, path 검증, subprocess 격리 |
| Trackable | CLI 실행 로그 (명령, 종료 코드, 소요 시간) |

---

## 5. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-LLM-001 v0.1.0** | `LLMProvider` 인터페이스, Registry, 에러 타입 |
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | `llm.delegation.*` 설정 필드 |
| 외부 | Claude CLI | `claude` 명령어 (선택적) |
| 외부 | Gemini CLI | `gemini` 명령어 (선택적) |
| 외부 | Codex CLI | `codex` 명령어 (선택적) |
| 외부 | `os/exec` | Go 표준 라이브러리 subprocess 실행 |

---

## 6. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | CLI 출력 형식이 버전에 따라 변경 | 높 | 중 | 버전 감지 + 파서 fallback, raw output 보존 |
| R2 | CLI subprocess가 좀비 프로세스로 남음 | 중 | 높 | context cancel + SIGTERM + SIGKILL 보장, `cmd.Wait()` 필수 호출 |
| R3 | Prompt에 특수문자/따옴표 포함 시 명령어 깨짐 | 중 | 중 | stdin으로 prompt 전달 (명령행 인수 escaping 대신) |
| R4 | CLI 도구 설치 경로가 사용자마다 다름 | 높 | 낮 | `cli_paths` 설정으로 경로 지정, `PATH`에서 자동 탐색 fallback |
| R5 | CLI 도구가 스트리밍을 지원하지 않음 | 중 | 낮 | `Stream()` 미지원 시 `ErrCapabilityUnsupported` 반환, `Capabilities()`에 명시 |

---

## 7. Exclusions (What NOT to Build)

- 본 수정안은 **CLI 도구 자체의 설치를 자동화하지 않는다** — 사용자 책임.
- 본 수정안은 **CLI 도구의 인증/로그인을 자동화하지 않는다** — CLI 도구에 이미 인증되어 있다고 가정.
- 본 수정안은 **CLI 도구의 기능(툴 호출, 멀티모달 등)을 네이티브로 지원하지 않는다** — 텍스트 입출력만 래핑.
- 본 수정안은 **기존 LLMProvider 인터페이스를 변경하지 않는다** — 구현체만 추가.
- 본 수정안은 **CLI provider 간의 로드 밸런싱을 구현하지 않는다** — Router의 기존 로직에 위임.

---

## 8. SPEC v0.1.0과의 관계

본 수정안은 SPEC-GOOSE-LLM-001 v0.1.0에 **추가 레이어**를 더한다:

```
Layer 1: LLMProvider 인터페이스 + Ollama HTTP 어댑터     (v0.1.0)
Layer 2: SDK 기반 cloud provider 어댑터                  (ADAPTER-001/002)
Layer 3: CLI subprocess provider 어댑터                   (본 수정안 v0.2.0)
```

- 기존 REQ-LLM-001 ~ REQ-LLM-015는 **변경 없이 유지**.
- 본 수정안은 REQ-LLM-020 ~ REQ-LLM-027을 **추가**.
- Registry의 factory 맵에 `"claude-cli"`, `"gemini-cli"`, `"codex-cli"`를 추가.
- 기존 인터페이스, 에러 타입, retry 로직은 그대로 재사용.

---

Version: 0.2.0 (Amendment to SPEC-GOOSE-LLM-001 v0.1.0)
Last Updated: 2026-04-29
Author: manager-spec
