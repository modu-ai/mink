# SPEC-GOOSE-GEMMA4-001 — Acceptance Criteria

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-29 | 초안 작성 | manager-spec |

---

## 1. 수용 기준 (Given-When-Then)

### AC-G4-001: 기본 로컬 모델 설정

- **Given** `config.yaml`에 `llm.mode: local` 설정, `llm.local.model` 미명시
- **When** 시스템이 초기화되고 `LocalModeManager.Initialize()` 호출
- **Then** 기본 모델이 `ai-goose/gemma4-e4b-rl-v1`로 설정됨
- **Then** `provider.Name() == "ollama"`
- **Satisfies**: REQ-G4-001

### AC-G4-002: RAM 기반 모델 추천 — E4B

- **Given** 시스템 RAM이 12 GB (8-16 GB bracket)
- **When** `RecommendModelProfile(12 * 1024 * 1024 * 1024)` 호출
- **Then** 반환값의 `ModelTag == "ai-goose/gemma4-e4b-rl-v1"`
- **Then** 반환값의 `Name == "E4B"`
- **Satisfies**: REQ-G4-002, REQ-G4-003

### AC-G4-003: RAM 기반 모델 추천 — E2B

- **Given** 시스템 RAM이 4 GB (< 8 GB bracket)
- **When** `RecommendModelProfile(4 * 1024 * 1024 * 1024)` 호출
- **Then** 반환값의 `ModelTag == "ai-goose/gemma4-e2b-rl-v1"`
- **Then** 반환값의 `Name == "E2B"`
- **Satisfies**: REQ-G4-002, REQ-G4-012

### AC-G4-004: RAM 기반 모델 추천 — E4B-Q8

- **Given** 시스템 RAM이 64 GB (32+ GB bracket, GPU 없음)
- **When** `RecommendModelProfile(64 * 1024 * 1024 * 1024)` 호출
- **Then** 반환값의 `ModelTag == "ai-goose/gemma4-e4b-q8-rl-v1"`
- **Then** 반환값의 `Name == "E4B-Q8"`
- **Satisfies**: REQ-G4-002

### AC-G4-005: 모델 미존재 시 auto-pull

- **Given** Ollama `/api/tags` 응답에 `ai-goose/gemma4-e4b-rl-v1` 포함되지 않음
- **Given** `auto_pull: true`
- **When** `LocalModeManager.EnsureModelAvailable(ctx, "ai-goose/gemma4-e4b-rl-v1")` 호출
- **Then** `POST /api/pull` 요청 발생 (body: `{"name": "ai-goose/gemma4-e4b-rl-v1", "stream": true}`)
- **Then** 진행률 콜백이 최소 1회 호출됨 (percentage > 0)
- **Then** 완료 후 `GET /api/tags`에 모델이 포함됨
- **Then** 에러 없이 반환됨
- **Satisfies**: REQ-G4-005, REQ-G4-006

### AC-G4-006: auto-pull 비활성화 시 에러

- **Given** Ollama `/api/tags`에 모델 미포함
- **Given** `auto_pull: false`, `fallback_model` 미설정
- **When** `LocalModeManager.EnsureModelAvailable(ctx, "ai-goose/gemma4-e4b-rl-v1")` 호출
- **Then** `ErrModelNotFound` 반환
- **Then** 에러 메시지에 `ollama pull ai-goose/gemma4-e4b-rl-v1` 포함
- **Then** `POST /api/pull` 요청 발생하지 않음
- **Satisfies**: REQ-G4-014

### AC-G4-007: Local 모드 라우팅

- **Given** `llm.mode: "local"`, Ollama provider 정상 동작
- **Given** `ModeRouter`가 local provider와 cloud provider로 초기화됨
- **When** `ModeRouter.Route(ctx, req)` 호출
- **Then** local provider의 `Complete()`가 호출됨
- **Then** cloud provider의 `Complete()`가 호출되지 않음 (mock으로 검증)
- **Satisfies**: REQ-G4-008, REQ-G4-011

### AC-G4-008: Cloud 모드 라우팅

- **Given** `llm.mode: "cloud"`
- **When** `ModeRouter.Route(ctx, req)` 호출
- **Then** cloud provider의 `Complete()`가 호출됨
- **Then** local provider의 `Complete()`가 호출되지 않음
- **Satisfies**: REQ-G4-008

### AC-G4-009: Hybrid 모드 — 로컬 성공

- **Given** `llm.mode: "hybrid"`, local provider 정상 동작
- **When** `ModeRouter.Route(ctx, req)` 호출
- **Then** local provider의 `Complete()`가 호출됨
- **Then** cloud provider의 `Complete()`가 호출되지 않음
- **Then** local provider의 응답이 반환됨
- **Satisfies**: REQ-G4-008

### AC-G4-010: Hybrid 모드 — 로컬 실패 시 cloud fallback

- **Given** `llm.mode: "hybrid"`, local provider가 `ErrServerUnavailable` 반환
- **When** `ModeRouter.Route(ctx, req)` 호출
- **Then** local provider 시도 후 cloud provider로 fallback
- **Then** cloud provider의 응답이 반환됨
- **Then** 로그에 fallback 이벤트 기록됨
- **Satisfies**: REQ-G4-008

### AC-G4-011: Fallback 모델 전환

- **Given** 기본 모델 `ai-goose/gemma4-e4b-rl-v1`이 Ollama에서 404 반환 (retry 소진)
- **Given** `fallback_model: "ai-goose/gemma4-e2b-rl-v1"` 설정
- **When** `LocalModeManager.EnsureModelAvailable()` 호출
- **Then** fallback 모델로 전환 시도
- **Then** 로그에 fallback 전환 이벤트 기록
- **Then** fallback 모델이 사용 가능하면 에러 없이 반환
- **Satisfies**: REQ-G4-013

### AC-G4-012: 사용자 모델 오버라이드

- **Given** `config.yaml`에 `llm.local.model: "ai-goose/gemma4-e4b-q5-rl-v1"` 명시
- **Given** 시스템 RAM이 12 GB (E4B 추천 bracket)
- **When** 시스템 초기화
- **Then** RAM 감지 결과와 무관하게 `ai-goose/gemma4-e4b-q5-rl-v1` 사용
- **Satisfies**: REQ-G4-004

### AC-G4-013: MLX-LM 비활성화 (비 macOS)

- **Given** `runtime.GOOS == "linux"`, `mlx_enabled: true`
- **When** MLX provider 초기화 시도
- **Then** 경고 로그 출력 ("MLX-LM is only supported on macOS with Apple Silicon")
- **Then** Ollama provider 사용, MLX-LM subprocess 미실행
- **Satisfies**: REQ-G4-015

### AC-G4-014: RAM 감지 실패 시 기본값

- **Given** RAM 감지 함수가 에러 반환 (예: `/proc/meminfo` 없음)
- **When** `RecommendModelProfile()` 호출
- **Then** E4B 프로파일 반환 (기본값)
- **Then** 경고 로그 출력 ("Failed to detect system RAM, using default E4B profile")
- **Satisfies**: REQ-G4-003

---

## 2. 엣지 케이스

| # | 시나리오 | 기대 동작 |
|---|---------|----------|
| E1 | Ollama 서버 미실행, local mode | `ErrServerUnavailable` + "Run `ollama serve` to start" 메시지 |
| E2 | 모델 다운로드 중 context cancel | 즉시 종료, partial download는 Ollama가 관리 |
| E3 | RAM이 정확히 8 GB (경계값) | E4B 프로파일 선택 (>= 8 GB) |
| E4 | fallback_model도 사용 불가 | `ErrModelNotFound` 반환 (이중 실패) |
| E5 | hybrid 모드에서 cloud provider도 실패 | 마지막 에러 반환, 두 에러 모두 로그 |
| E6 | 빈 문자열 모델명 설정 | 기본값(`ai-goose/gemma4-e4b-rl-v1`) 사용 |
| E7 | 동시에 여러 요청이 들어오고 auto-pull 진행 중 | pull은 한 번만 실행, 나머지는 대기 |

---

## 3. 품질 게이트 (Quality Gate Criteria)

### 3.1 Definition of Done

- [ ] AC-G4-001 ~ AC-G4-014 모두 통과
- [ ] 테스트 커버리지 85% 이상 (`internal/llm/local/`, `internal/llm/mlx/`)
- [ ] `go vet`, `golangci-lint` 통과
- [ ] race detector clean (`go test -race ./...`)
- [ ] 기존 LLM-001, CONFIG-001, ROUTER-001 테스트 회귀 없음
- [ ] Exclusions에 나열된 항목 중 구현된 것이 없음

### 3.2 TRUST 5 검증

| 차원 | 검증 항목 |
|-----|----------|
| Tested | 단위 테스트 + 통합 테스트 + 플랫폼별 mock 테스트 |
| Readable | godoc 주석, 함수명 의도 명확 |
| Unified | 기존 에러 타입 재사용, 설정 구조 일관성 |
| Secured | 다운로드 검증, 민감 정보 없음 |
| Trackable | 구조화 로그, 이벤트 추적 |

### 3.3 비기능 요구사항

- RAM 감지: 100ms 이내 완료.
- 모델 존재 확인: 500ms 이내 완료 (Ollama localhost).
- Mode routing 오버헤드: 1ms 미만.
- MLX provider 초기화: 2초 이내 (모델 로드 제외).

---

Version: 0.1.0
Last Updated: 2026-04-29
