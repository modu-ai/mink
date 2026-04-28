---
id: SPEC-GOOSE-GEMMA4-001
version: 0.1.0
status: planned
created_at: 2026-04-29
updated_at: 2026-04-29
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 중(M)
lifecycle: spec-anchored
labels: [gemma4, local-llm, ollama, cross-platform, ml-lm, area/llm, type/feature, priority/p0-critical]
---

# SPEC-GOOSE-GEMMA4-001 — Gemma 4 Local Mode Integration

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-29 | 초안 작성 — Gemma 4 로컬 모델 기본값 전환, RAM 프로파일 자동 감지, auto-pull, local/cloud/hybrid 모드 지원 | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE의 **기본 로컬 LLM을 `qwen2.5:3b`에서 `ai-goose/gemma4-e4b-rl-v1`로 전환**하고, 사용자 시스템 RAM에 따른 최적 모델 프로파일 자동 추천, Ollama 모델 자동 다운로드, 그리고 local/cloud/hybrid 3모드 LLM 운영 체계를 확립한다.

수락 조건 통과 시점에서:

- 기본 로컬 모델이 `ai-goose/gemma4-e4b-rl-v1`로 설정된다.
- 사용자 시스템 RAM을 감지하여 적절한 모델 프로파일(E2B/E4B/E4B-Q5/E4B-Q8/26B)을 추천한다.
- 설정된 모델이 Ollama에 없으면 자동으로 pull하며 진행 상황을 표시한다.
- `llm.mode: local` 설정 시 모든 LLM 호출이 Ollama로 라우팅되며 API key가 필요 없다.
- Mac 사용자는 MLX-LM을 선택적 provider로 사용할 수 있다.
- 기본 모델 로드 실패 시 fallback 모델로 자동 전환된다.

---

## 2. 배경 (Background)

### 2.1 왜 Gemma 4인가

- Google Gemma 4는 2026년 기준 오픈모델 중 **성능/효율 최상위권**이다. RL(Reinforcement Learning) 파인튜닝을 통해 AI.GOOSE의 컴패니언 AI 시나리오(대화, 코드 생성, 도구 호출)에 최적화된 `gemma4-e4b-rl-v1` 모델을 개발했다.
- E4B(4-billion effective parameters) 크기는 **8-16GB RAM 시스템**에서 실행 가능하며, M4 Max 128GB에서 훈련 후 Ollama registry를 통해 배포한다.
- Ollama registry 배포를 통해 Windows/Mac/Linux 모든 사용자가 `ollama pull ai-goose/gemma4-e4b-rl-v1` 한 줄로 설치 가능하다.

### 2.2 기존 아키텍처와의 관계

- **SPEC-GOOSE-LLM-001**이 정의한 `LLMProvider` 인터페이스와 Ollama 어댑터를 **그대로 재사용**한다. 본 SPEC은 모델 선택, 설정 구조, auto-pull, 모드 전환 로직을 추가한다.
- **SPEC-GOOSE-CONFIG-001**의 계층형 설정 로더를 확장하여 `llm` 섹션에 `mode`, `local`, `fallback_model`, `auto_pull`, `mlx_enabled` 필드를 추가한다.
- **SPEC-GOOSE-ROUTER-001**의 스마트 라우팅이 `llm.mode` 값을 읽어 local-only / cloud-only / hybrid 분기를 수행한다.

### 2.3 MLX-LM 옵션

- Mac(Apple Silicon) 사용자는 MLX-LM 프레임워크를 통해 Metal GPU 가속을 받을 수 있다.
- MLX-LM은 `LLMProvider` 인터페이스의 **또 다른 구현체**로, Ollama와 동일한 인터페이스를 만족한다.
- `mlx_enabled: true` 시 MLX-LM을 우선 provider로 시도하고, 실패 시 Ollama로 fallback한다.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. 기본 로컬 모델을 `ai-goose/gemma4-e4b-rl-v1`로 변경 (config default).
2. 모델 프로파일 매트릭스 정의 및 RAM 기반 자동 추천 로직.
3. `llm.mode` 설정 (local-only | cloud-only | hybrid | delegation) 및 모드별 라우팅 로직.
4. Ollama 모델 존재 여부 감지 및 auto-pull 기능 (진행 바 표시).
5. 다운로드 중단 시 resume 지원.
6. 기본 모델 실패 시 fallback 모델 자동 전환.
7. `internal/llm/local/` 신규 패키지 — local mode 관련 로직.
8. `internal/llm/mlx/` 신규 패키지 — MLX-LM provider (Mac-only).
9. 설정 스키마 확장 — `llm.local.*` 필드.

### 3.2 OUT OF SCOPE

- 모델 RL 훈련 프로세스 (별도 SPEC-GOOSE-TRAIN-001).
- Ollama 서버 자체의 설치/실행 (`ollama serve`는 사용자 책임).
- 클라우드 프로바이더 API 연동 (SPEC-GOOSE-ADAPTER-001, ADAPTER-002).
- 모델 성능 벤치마크 및 벤치마크 자동화.
- MLX-LM CLI 설치 및 설정 (사용자가 `pip install mlx-lm` 수행 가정).
- 모델 버전 관리 및 자동 업데이트 (후속 SPEC 후보).
- GPU/CPU 사용량 모니터링 및 리소스 제한.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-G4-001 [Ubiquitous]** — The system **shall** use `ai-goose/gemma4-e4b-rl-v1` as the default local model when `llm.mode` is `local` and no model override is specified in configuration.

**REQ-G4-002 [Ubiquitous]** — The system **shall** support the following model profiles and map each profile to the system RAM bracket:

| Profile | Model Tag | RAM Bracket | Target |
|---------|----------|-------------|--------|
| E2B | `ai-goose/gemma4-e2b-rl-v1` | < 8 GB | Entry devices |
| E4B | `ai-goose/gemma4-e4b-rl-v1` | 8-16 GB | Standard laptops |
| E4B-Q5 | `ai-goose/gemma4-e4b-q5-rl-v1` | 16-32 GB | Power users |
| E4B-Q8 | `ai-goose/gemma4-e4b-q8-rl-v1` | 32 GB+ | Workstations |
| 26B | `ai-goose/gemma4-26b-rl-v1` | 32 GB+ (with GPU) | High-end |

**REQ-G4-010 [Ubiquitous]** — The system **shall** reuse the existing `LLMProvider` interface defined in SPEC-GOOSE-LLM-001 for all local mode operations; no new interface methods are introduced.

### 4.2 Event-Driven

**REQ-G4-003 [Event-Driven]** — **When** the system starts and no model is configured, the system **shall** detect total system RAM and recommend the appropriate model profile from REQ-G4-002, logging the recommendation.

**REQ-G4-005 [Event-Driven]** — **When** the system initializes the local LLM provider, it **shall** detect if the configured model is available in the local Ollama instance by querying `GET /api/tags`.

**REQ-G4-006 [Event-Driven]** — **When** the configured model is not present in Ollama (`GET /api/tags` returns no match), and `auto_pull` is `true`, the system **shall** execute `POST /api/pull` with the model name and display download progress (percentage and bytes downloaded) to the user.

**REQ-G4-007 [Event-Driven]** — **When** a model download via auto-pull is interrupted (network error, user cancellation), the system **shall** support resume by re-issuing the `POST /api/pull` request; Ollama's layer-based download handles partial resume natively.

**REQ-G4-011 [Event-Driven]** — **When** `llm.mode` is `local-only`, the system **shall** route ALL LLM calls to the local Ollama provider; no cloud API calls shall be attempted.

**REQ-G4-013 [Event-Driven]** — **When** the primary local model fails to load (Ollama returns 404 or connection error after retries), the system **shall** attempt to use the `fallback_model` if configured.

### 4.3 State-Driven

**REQ-G4-008 [State-Driven]** — **While** `llm.mode` is set to `local-only`, `cloud-only`, `hybrid`, or `delegation`, the system **shall** apply the corresponding routing behavior as defined in SPEC-GOOSE-ROUTER-001 v1.1.0 REQ-RT-017/REQ-RT-018:

| Mode | Local Calls | Cloud Calls | Delegation | API Key Required |
|------|-------------|-------------|------------|-----------------|
| local-only | ALL | NONE | NONE | No |
| cloud-only | NONE | ALL | NONE | Yes |
| hybrid | Prefers local | Fallback to cloud | NONE | Yes (for cloud) |
| delegation | Per rules | Per rules | Per rules | Per target |

**REQ-G4-012 [State-Driven]** — **While** system RAM is below 8 GB, the system **shall** use the E2B model profile as the default; if user explicitly selects a larger profile, a warning shall be logged.

### 4.4 Unwanted Behavior

**REQ-G4-014 [Unwanted]** — **If** `auto_pull` is `false` and the model is not found locally, **then** the system **shall** return `ErrModelNotFound` with a user-facing message suggesting `ollama pull <model>`; it **shall not** silently fall back to a different model unless `fallback_model` is configured.

**REQ-G4-015 [Unwanted]** — **If** the MLX-LM provider is enabled but the runtime OS is not macOS, **then** the system **shall** log a warning and fall back to the Ollama provider; it **shall not** attempt to invoke MLX-LM.

### 4.5 Optional

**REQ-G4-004 [Optional]** — **Where** `.goose/config.yaml` exists with `llm.local.model` set, the system **shall** use the user-specified model instead of the auto-detected recommendation.

**REQ-G4-009 [Optional]** — **Where** the runtime is macOS with Apple Silicon and `llm.local.mlx_enabled` is `true`, the system **shall** use the MLX-LM provider as an alternative to Ollama for potential performance gains.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-G4-001 — 기본 로컬 모델 설정**
- **Given** `config.yaml`에 `llm.mode: local-only` 설정, 모델 명시 없음
- **When** 시스템이 초기화됨
- **Then** 기본 모델이 `ai-goose/gemma4-e4b-rl-v1`로 설정됨, `provider.Name() == "ollama"`

**AC-G4-002 — RAM 기반 모델 추천**
- **Given** 시스템 RAM이 12 GB
- **When** `RecommendModelProfile(totalRAM)` 호출
- **Then** 반환값이 E4B 프로파일(`ai-goose/gemma4-e4b-rl-v1`)임

**AC-G4-003 — 모델 미존재 시 auto-pull**
- **Given** Ollama `/api/tags`에 `ai-goose/gemma4-e4b-rl-v1`이 없음, `auto_pull: true`
- **When** 시스템이 로컬 provider 초기화
- **Then** `POST /api/pull` 호출됨, 진행률 콜백이 호출됨 (최소 1회 status callback), 다운로드 완료 후 `/api/tags`에 모델이 존재함

**AC-G4-004 — auto-pull 비활성화 시 에러**
- **Given** Ollama `/api/tags`에 모델이 없음, `auto_pull: false`, `fallback_model` 미설정
- **When** 로컬 provider 초기화
- **Then** `ErrModelNotFound` 반환, 에러 메시지에 `ollama pull` 명령어 제안 포함

**AC-G4-005 — Local 모드 라우팅**
- **Given** `llm.mode: local-only`
- **When** `router.Route(ctx, req)` 호출
- **Then** 모든 LLM 호출이 Ollama provider로 라우팅됨, cloud provider는 호출되지 않음

**AC-G4-006 — Fallback 모델 전환**
- **Given** 기본 모델이 로드 실패 (Ollama 404 + retry 소진), `fallback_model: ai-goose/gemma4-e2b-rl-v1` 설정
- **When** `provider.Complete(ctx, req)` 호출
- **Then** fallback 모델로 자동 전환 시도, 성공 시 응답 반환, 로그에 fallback 이벤트 기록

**AC-G4-007 — 다운로드 재개 (Resume)** (verifies REQ-G4-007)
- **Given** 모델 다운로드 중 네트워크 중단으로 `POST /api/pull` 실패
- **When** `EnsureModelAvailable` 재호출
- **Then** `POST /api/pull` 재실행, Ollama의 layer-based resume으로 이전에 완료된 layer는 재다운로드하지 않음

**AC-G4-008 — Hybrid 모드 동작**
- **Given** `llm.mode: hybrid`, 로컬 Ollama 정상 동작
- **When** LLM 호출 발생
- **Then** 우선 로컬 provider로 시도, 로컬 실패 시 cloud provider로 fallback

**AC-G4-009 — 사용자 모델 오버라이드**
- **Given** `config.yaml`에 `llm.local.model: ai-goose/gemma4-e4b-q5-rl-v1` 명시
- **When** 시스템 초기화
- **Then** RAM 감지 결과와 무관하게 사용자 지정 모델 사용

**AC-G4-010 — MLX-LM 비활성화 (비 macOS)**
- **Given** 런타임 OS가 Linux, `mlx_enabled: true`
- **When** 시스템 초기화
- **Then** 경고 로그 출력 후 Ollama provider 사용, MLX-LM 호출 시도 없음

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/llm/
├── provider.go                  # 기존 — 변경 없음
├── registry.go                  # 기존 — 변경 없음
├── ollama/                      # 기존 — 변경 없음
├── local/                       # 신규 — local mode 관리
│   ├── local.go                 # LocalMode manager (RAM 감지, 모델 추천)
│   ├── profiles.go              # 모델 프로파일 매트릭스
│   ├── autopull.go              # auto-pull 로직 + 진행 바
│   └── *_test.go
├── mlx/                         # 신규 — MLX-LM provider (Mac-only)
│   ├── mlx.go                   # MLXProvider struct + New()
│   ├── client.go                # subprocess 호출
│   └── *_test.go
└── router/                      # 기존 — mode 분기 로직 추가
    └── router.go                # llm.mode 기반 라우팅 확장
```

### 6.2 설정 스키마 확장

```yaml
llm:
  mode: local-only               # local-only | cloud-only | hybrid | delegation
  default_provider: ollama       # 기존 유지
  local:
    provider: ollama             # ollama | mlx
    model: ""                    # 빈 값 = 자동 감지
    fallback_model: "ai-goose/gemma4-e2b-rl-v1"
    auto_pull: true
    mlx_enabled: false           # Mac-only 최적화
  providers:                     # 기존 유지
    ollama:
      kind: ollama
      host: "http://localhost:11434"
```

### 6.3 핵심 타입

```go
// ModelProfile defines a RAM-bracket model mapping.
type ModelProfile struct {
    Name      string // "E4B"
    ModelTag  string // "ai-goose/gemma4-e4b-rl-v1"
    MinRAM    uint64 // 8 GB
    RAMBracket string // "8-16GB"
}

// LocalModeManager manages local LLM initialization.
type LocalModeManager struct {
    config    LocalConfig
    registry  *Registry
    logger    *zap.Logger
}

// RecommendModelProfile returns the best profile for the given RAM.
func RecommendModelProfile(totalRAM uint64) ModelProfile

// EnsureModelAvailable checks availability and pulls if needed.
func (m *LocalModeManager) EnsureModelAvailable(ctx context.Context, model string) error

// ModeRouter routes based on llm.mode setting.
type ModeRouter struct {
    local  LLMProvider
    cloud  LLMProvider
    mode   string // local-only | cloud-only | hybrid | delegation
}
```

### 6.4 RAM 감지

- Linux: `/proc/meminfo`의 `MemTotal` 필드.
- macOS: `syscall.Sysctl("hw.memsize")`.
- Windows: `globalMemoryStatusEx` Windows API (`golang.org/x/sys/windows`).
- 감지 실패 시 E4B 프로파일을 기본값으로 사용 (경고 로그).

### 6.5 Auto-Pull 진행 표시

- `POST /api/pull`의 NDJSON 응답에서 `status` 필드 파싱.
- `pulling <layer>` 상태에서 `completed` / `total` 바이트로 진행률 계산.
- 콜백 함수 `OnProgress(status, completed, total int64)`로 진행 상황 전달.
- CLI에서는 `fmt.Fprintf(os.Stderr, "\rPulling %s: %.1f%%", model, pct)`로 표시.

### 6.6 MLX-LM Provider

- MLX-LM은 CLI subprocess로 실행: `python -m mlx_lm.generate --model <path> --prompt <text>`.
- MLXProvider는 `LLMProvider` 인터페이스를 구현.
- `Complete()`: subprocess 실행, stdout 파싱.
- `Stream()`: subprocess stdout 라인 단위 읽기, `<-chan Chunk` 변환.
- Mac 전용 빌드 태그: `//go:build darwin && arm64`.
- MLX-LM 미설치 시 초기화 단계에서 감지, Ollama로 자동 fallback.

### 6.7 TRUST 5 매핑

| 차원 | 달성 |
|-----|------|
| Tested | `httptest.Server`로 Ollama mock, RAM 감지 mock, MLX subprocess mock |
| Readable | `LocalModeManager` doc-comment에 전제/사후 조건 명시 |
| Unified | `golangci-lint` + 기존 에러 체계 재사용 |
| Secured | 로컬 모델 다운로드 시 checksum 검증 (Ollama 내장), 민감 정보 없음 |
| Trackable | auto-pull 이벤트, fallback 전환, 모드 변경 로그 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-LLM-001** | `LLMProvider` 인터페이스, Ollama 어댑터, Registry |
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | 계층형 설정 로더, `llm` 섹션 |
| 선행 SPEC | **SPEC-GOOSE-ROUTER-001** | 스마트 라우팅, `mode` 분기 확장 |
| 후속 SPEC | SPEC-GOOSE-TRAIN-001 | Gemma 4 RL 훈련 프로세스 |
| 후속 SPEC | SPEC-GOOSE-CROSSPLAT-001 | 크로스 플랫폼 RAM 감지 검증 |
| 외부 | Ollama 서버 | `GET /api/tags`, `POST /api/pull` |
| 외부 | MLX-LM (선택) | `python -m mlx_lm.generate` |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Ollama `/api/pull` API 변경 (진행률 필드 변경) | 중 | 중 | Ollama 버전 체크, unknown status는 "downloading..."으로 fallback 표시 |
| R2 | MLX-LM subprocess 응답 파싱 실패 (버전별 출력 형식 변경) | 중 | 낮 | MLX-LM 버전 고정 권장, 파싱 실패 시 Ollama로 자동 fallback |
| R3 | RAM 감지 실패 (권한, 컨테이너 환경) | 낮 | 중 | 감지 실패 시 E4B 기본값 + 경고 로그 |
| R4 | 대용량 모델 다운로드 중 네트워크 불안정 | 중 | 낮 | Ollama의 layer-based resume 활용, 사용자에게 재시도 안내 |
| R5 | 기존 사용자의 `qwen2.5:3b` 설정과 충돌 | 낮 | 중 | 설정에 `model`이 명시되어 있으면 기존 값 유지, 마이그레이션 가이드 제공 |
| R6 | Ollama 서버 미실행 (local 모드) | 높 | 높 | 명확한 에러 메시지 + `ollama serve` 실행 안내 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/tech.md` §3.2 (AI/LLM 스택), §9 (프로바이더 지원 매트릭스)
- `.moai/project/structure.md` §1 `internal/llm/` 패키지 스케치
- `.moai/project/product.md` §3.1 "로컬 우선" 원칙

### 9.2 외부 참조

- Ollama API 레퍼런스: https://github.com/ollama/ollama/blob/main/docs/api.md
- Ollama Registry: https://ollama.com/registry
- MLX-LM: https://github.com/ml-explore/mlx-lm
- Gemma 모델: https://ai.google.dev/gemma

### 9.3 부속 문서

- `.moai/specs/SPEC-GOOSE-LLM-001/spec.md` (인터페이스 정의)
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` (설정 로더)
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` (라우팅)

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **Gemma 4 RL 훈련 파이프라인을 구현하지 않는다** — SPEC-GOOSE-TRAIN-001의 범위.
- 본 SPEC은 **Ollama 서버 자체의 설치, 실행, 관리를 자동화하지 않는다** — 사용자가 `ollama serve`를 실행했다고 가정.
- 본 SPEC은 **클라우드 프로바이더 어댑터를 구현하지 않는다** — SPEC-GOOSE-ADAPTER-001, ADAPTER-002의 범위.
- 본 SPEC은 **모델 성능 벤치마크 자동화를 제공하지 않는다** — 후속 SPEC 후보.
- 본 SPEC은 **MLX-LM의 설치(`pip install mlx-lm`)를 자동화하지 않는다** — 사용자 책임.
- 본 SPEC은 **모델 버전 관리, 자동 업데이트를 구현하지 않는다** — 후속 SPEC 후보.
- 본 SPEC은 **GPU/CPU 리소스 모니터링 및 제한을 구현하지 않는다** — 후속 SPEC 후보.
- 본 SPEC은 **기존 `LLMProvider` 인터페이스를 변경하지 않는다** — 기존 인터페이스 재사용만.

---

**End of SPEC-GOOSE-GEMMA4-001**
