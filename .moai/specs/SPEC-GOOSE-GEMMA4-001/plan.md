# SPEC-GOOSE-GEMMA4-001 — Implementation Plan

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-29 | 초안 작성 | manager-spec |

---

## 1. 구현 전략

### 1.1 접근 방식

기존 `LLMProvider` 인터페이스를 변경하지 않고, 설정 확장과 신규 패키지 추가로 local mode 기능을 구현한다. Ollama 어댑터는 그대로 재사용하며, auto-pull과 RAM 감지 로직을 별도 패키지로 분리한다.

### 1.2 핵심 원칙

- **인터페이스 불변**: `LLMProvider` 인터페이스 수정 없음.
- **설정 확장**: CONFIG-001의 구조에 `llm.local.*` 필드만 추가.
- **점진적 구현**: E2B fallback -> auto-pull -> mode routing -> MLX 순서.

---

## 2. 마일스톤

### Milestone 1: 모델 프로파일 및 RAM 감지 (Priority: High)

**목표**: 시스템 RAM을 감지하고 적절한 모델 프로파일을 추천.

**작업 항목**:
- `internal/llm/local/profiles.go` — 모델 프로파일 매트릭스 정의
- `internal/llm/local/local.go` — RAM 감지 로직 (Linux/macOS/Windows)
- `RecommendModelProfile()` 함수 및 단위 테스트
- CONFIG-001 설정 스키마에 `llm.local.*` 필드 추가

**산출물**:
- `internal/llm/local/profiles.go`
- `internal/llm/local/local.go`
- `internal/llm/local/profiles_test.go`
- `internal/llm/local/local_test.go`
- `internal/config/defaults.go` 업데이트

### Milestone 2: Auto-Pull (Priority: High)

**목표**: Ollama 모델 미존재 시 자동 다운로드.

**작업 항목**:
- `internal/llm/local/autopull.go` — `/api/tags` 확인 + `/api/pull` 실행
- 진행률 콜백 인터페이스 정의
- 중단/resume 처리 (Ollama layer-based)
- fallback 모델 전환 로직

**산출물**:
- `internal/llm/local/autopull.go`
- `internal/llm/local/autopull_test.go`

### Milestone 3: Mode Routing (Priority: High)

**목표**: local/cloud/hybrid 모드별 라우팅.

**작업 항목**:
- `internal/llm/router/` 확장 — `ModeRouter` 추가
- `llm.mode` 설정값 읽기 및 분기
- hybrid 모드 fallback 로직
- 기존 라우터와 통합

**산출물**:
- `internal/llm/router/mode_router.go`
- `internal/llm/router/mode_router_test.go`

### Milestone 4: MLX-LM Provider (Priority: Medium)

**목표**: Mac Apple Silicon에서 MLX-LM 선택적 provider 지원.

**작업 항목**:
- `internal/llm/mlx/mlx.go` — MLXProvider 구현
- `internal/llm/mlx/client.go` — subprocess 호출 및 파싱
- Mac 전용 빌드 태그 적용
- Ollama로 자동 fallback 로직

**산출물**:
- `internal/llm/mlx/mlx.go`
- `internal/llm/mlx/client.go`
- `internal/llm/mlx/mlx_test.go`
- `internal/llm/mlx/client_test.go`

### Milestone 5: 통합 및 설정 마이그레이션 (Priority: Medium)

**목표**: 기존 사용자 마이그레이션 및 end-to-end 통합.

**작업 항목**:
- 기본 모델 `qwen2.5:3b` -> `ai-goose/gemma4-e4b-rl-v1` 전환
- 기존 설정 호환성 유지
- 통합 테스트 작성
- CLI에 `--mode` 플래그 추가 (선택)

**산출물**:
- `internal/config/defaults.go` 변경
- 통합 테스트
- 마이그레이션 가이드 문서

---

## 3. 기술 접근

### 3.1 RAM 감지 구현

```
DetectTotalRAM() (uint64, error):
  - runtime.GOOS == "linux": /proc/meminfo 파싱
  - runtime.GOOS == "darwin": syscall.Sysctl("hw.memsize")
  - runtime.GOOS == "windows": golang.org/x/sys/windows GlobalMemoryStatusEx
  - fallback: 8 GB 가정 + 경고 로그
```

### 3.2 Auto-Pull 흐름

```
EnsureModelAvailable(ctx, model):
  1. GET /api/tags -> 모델 존재 확인
  2. 존재 -> nil 반환
  3. 미존재 + auto_pull=false -> ErrModelNotFound
  4. auto_pull=true -> POST /api/pull {name: model, stream: true}
  5. NDJSON 응답 파싱 -> 진행률 콜백
  6. 완료 -> GET /api/tags 재확인 -> nil 반환
  7. 실패 -> fallback_model로 재시도
```

### 3.3 Mode Routing 흐름

```
ModeRouter.Route(ctx, req):
  - mode == "local": return localProvider.Complete(ctx, req)
  - mode == "cloud": return cloudProvider.Complete(ctx, req)
  - mode == "hybrid":
    result, err := localProvider.Complete(ctx, req)
    if err != nil:
      log.Warn("local failed, falling back to cloud", err)
      return cloudProvider.Complete(ctx, req)
    return result, nil
```

### 3.4 설정 기본값

```go
var DefaultLLMLocalConfig = LocalConfig{
    Provider:      "ollama",
    Model:         "",  // empty = auto-detect from RAM
    FallbackModel: "ai-goose/gemma4-e2b-rl-v1",
    AutoPull:      true,
    MLXEnabled:    false,
}
```

---

## 4. 리스크 완화 전략

| 리스크 | 완화 |
|-------|------|
| RAM 감지 실패 | E4B 기본값 + 경고 로그 |
| MLX-LM 미설치 | 초기화 단계 감지, Ollama fallback |
| 기존 사용자 설정 충돌 | 명시적 model 설정 시 우선, 마이그레이션 가이드 |
| Ollama 서버 미실행 | 명확한 에러 메시지 |

---

## 5. 의존성 그래프

```
M1 (Profiles/RAM) ──┐
                     ├── M3 (Mode Routing) ── M5 (Integration)
M2 (Auto-Pull) ─────┘
                     M4 (MLX) ───────────────── M5 (Integration)
```

- M1, M2, M4는 독립적 병렬 개발 가능.
- M3는 M1, M2에 의존.
- M5는 모든 마일스톤에 의존.

---

## 6. 테스트 전략

### 6.1 단위 테스트

- `TestRecommendModelProfile_*`: 각 RAM bracket별 프로파일 추천.
- `TestEnsureModelAvailable_AlreadyExists`: 모델 존재 시 no-op.
- `TestEnsureModelAvailable_AutoPull`: pull 흐름 + 진행률.
- `TestEnsureModelAvailable_NoAutoPull`: 에러 반환.
- `TestModeRouter_Local`: local 모드 라우팅.
- `TestModeRouter_Cloud`: cloud 모드 라우팅.
- `TestModeRouter_Hybrid_LocalFails`: hybrid fallback.
- `TestMLXProvider_NonMac`: 비 Mac 환경에서 MLX 비활성화.

### 6.2 통합 테스트

- `TestLocalMode_E2E`: RAM 감지 -> 프로파일 선택 -> auto-pull -> Complete.
- `TestFallback_E2E`: 기본 모델 실패 -> fallback 모델 전환.

### 6.3 플랫폼별 테스트

- Linux: `/proc/meminfo` mock.
- macOS: `syscall.Sysctl` mock.
- Windows: `GlobalMemoryStatusEx` mock.

---

Version: 0.1.0
Last Updated: 2026-04-29
