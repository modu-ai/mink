---
id: SPEC-GOOSE-LORA-001
version: 0.2.0
status: deprecated
created_at: 2026-04-21
updated_at: 2026-05-16
deprecated_at: 2026-05-16
author: manager-spec
priority: P2
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-anchored
labels: [deprecated]
supersedes: []
superseded_by: [SPEC-MINK-MEMORY-QMD-001]
---

# SPEC-GOOSE-LORA-001 — User-specific QLoRA Trainer (Go 인터페이스 + Rust 위임 경계) [DEPRECATED]

> **DEPRECATED 2026-05-16**: 사용자 결정으로 on-device QLoRA 학습 노선을 폐기한다. 대체 경로는 외부 GOAT LLM (Anthropic Claude / DeepSeek / OpenAI GPT / Codex / z.ai GLM-5-Turbo) + QMD 기반 메모리 (SPEC-MINK-MEMORY-QMD-001). 근거: ADR-001 (`.moai/decisions/ADR-001-qlora-rl-training-deprecation.md`). 본 SPEC 본문은 역사 보존을 위해 그대로 유지하되 신규 구현 대상에서 제외된다.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (learning-engine.md §5-6 + tech.md §2.1 goose-ml 기반) | manager-spec |
| 0.2.0 | 2026-05-16 | **Deprecated**. 사용자 결정으로 on-device QLoRA/강화학습 폐기, 외부 GOAT LLM + QMD 메모리로 대체. ADR-001 참조. | MoAI orchestrator |

---

## 1. 개요 (Overview)

GOOSE 사용자별 **QLoRA adapter** (10~200 MB) 를 생성·관리·핫스왑·롤백하는 **Go 인터페이스 레이어** 를 정의한다. 실제 tensor 수학·GPU 커널·ONNX 실행 같은 **성능 크리티컬 레이어는 Rust `goose-ml` crate 로 위임**하며, Go 는 훈련 오케스트레이션·데이터셋 빌드·안전장치 통합·버전관리·서빙에 집중한다.

본 SPEC 의 네 가지 책임:

1. **Go ↔ Rust 경계 정의** — gRPC(기본) 또는 CGO(hot path) 두 방식의 `LoRATrainer` 인터페이스. 본 SPEC 은 **프로토콜 contract** 만 고정.
2. **LoRA Version Registry** — adapter 의 SemVer 관리, 버전 히스토리, 롤백, 30-day cooldown.
3. **Continual Learning 오케스트레이션** — EWC / LwF / Replay Buffer 3 기법의 호출 시점과 데이터 공급. 실제 gradient 수식은 Rust 책임.
4. **Hot-swap & Evaluation** — adapter 파일 디스크립터 교체, A/B 비교, 회귀 감지.

본 SPEC 은 `internal/learning/lora/` 패키지의 인터페이스·불변식·에러를 규정한다. Rust crate `crates/goose-ml/` 은 **별도 로드맵(ROADMAP-RUST.md)** 에서 관리되며 본 SPEC 은 그 crate 의 존재를 가정하고 인터페이스만 정의한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- `learning-engine.md` §5 (User-specific LoRA Adapters) 가 "10-200MB per-user adapter" 를 명시. §5.2 는 1000+ 대화 후 주간 훈련을, §5.3 은 QLoRA/DoRA, base model 선택(Qwen3-0.6B / Gemma-1B), ONNX Runtime GenAI 를 요구.
- `learning-engine.md` §6 (Continual Learning Pipeline) 은 EWC / LwF / Replay Buffer 로 catastrophic forgetting 방지. §6.3 은 회귀 감지 자동 롤백 트리거.
- `tech.md` §2.1 은 `goose-ml` Rust crate 가 candle/burn/ort/safetensors 로 실제 훈련을 수행한다고 명시. Go 는 **오케스트레이션 만**.
- Phase 6 최종 퍼즐. IDENTITY-001 / VECTOR-001 / SAFETY-001 / REFLECT-001 이 모두 있어야 훈련 데이터 구성·승인·롤백 체인이 완결됨.

### 2.2 상속 자산 (패턴만 계승)

- **QLoRA (Dettmers et al. 2023)**: 4-bit 양자화 베이스 + LoRA adapter. 2GB VRAM 으로 훈련 가능.
- **DoRA (Liu et al. 2024)**: Weight-Decomposed LoRA. rank=16 기본, ~0.5-1% 정확도 향상.
- **ONNX Runtime GenAI**: Windows/Linux 온디바이스 추론. CGO or gRPC 호출.
- **Apple MLX / CoreML**: Mac on-device 훈련 (빠름).
- **EWC** (Kirkpatrick 2017), **LwF** (Li & Hoiem 2016), **Replay Buffer** (Rolnick 2019).

### 2.3 범위 경계 — Go vs Rust

| 항목 | Go (본 SPEC) | Rust (goose-ml, ROADMAP-RUST.md) |
|-----|-------------|--------------------------------|
| 훈련 오케스트레이션 | ✅ | — |
| 데이터셋 빌드 (VECTOR-001 필터) | ✅ | — |
| 안전장치 호출 (SAFETY-001) | ✅ | — |
| 버전 레지스트리·롤백 | ✅ | — |
| gRPC 서버 런너 | ✅ (process spawn·health check) | ✅ (gRPC 엔드포인트 내부) |
| Tensor 수학 / gradient | — | ✅ |
| ONNX Runtime GenAI 호출 | — | ✅ |
| LoRA 파일 포맷 (safetensors) | 읽기만 (metadata) | 쓰기·읽기 |
| 4-bit 양자화 | — | ✅ |
| EWC Fisher info 계산 | — | ✅ |
| Hypernetwork (Sakana AI) | — | ✅ (차기 별도 SPEC) |

본 SPEC 의 Go 코드는 **tensor 값 자체를 절대 조작하지 않는다**. 모든 수치 연산은 gRPC/CGO 경계를 통해 Rust 로 위임.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/learning/lora/` 패키지 생성.
2. `LoRATrainer` Go 인터페이스 — `PrepareDataset`, `Train`, `Evaluate`, `Merge`, `Apply`, `Rollback`, `ListVersions`.
3. `LoRAAdapter` 메타데이터 구조체 (`version`, `base_model`, `rank`, `alpha`, `target_modules`, `size_bytes`, `checksum`, `created_at`, `training_config`).
4. gRPC 프로토콜 `crates/goose-ml` 과 공유 (proto 파일 `proto/lora/v1/trainer.proto`).
5. CGO 바인딩 경로 `internal/learning/lora/cgo/` — 선택적 핫패스, 빌드 태그 `cgo_lora`.
6. `DatasetBuilder` — VECTOR-001 `Search(cosine>=0.7)` + IDENTITY-001 활성 fact 필터 → 200~500 sample JSON-L.
7. `Trainer Runner` — Rust 서비스 프로세스 lifecycle (spawn, health check, graceful shutdown).
8. `VersionRegistry` — `$GOOSE_HOME/lora/versions.json` (SemVer + SHA-256 checksum + timestamp).
9. `QLoRAConfig` / `DoRAConfig` — rank=16(기본), alpha=32, target_modules=["q_proj","v_proj","output"], 4-bit 양자화 on.
10. `BaseModel` 선택지: `qwen3-0.6b` / `gemma-1b` / 사용자 경로 (config `lora.base_model`).
11. Continual learning hooks: `WithEWC(fisher []float32)`, `WithLwF(prevAdapterPath string)`, `WithReplayBuffer(buf ReplayBuffer)`.
12. Hot-swap: `Apply(version string) error` — inference 세션을 draining 한 뒤 adapter 파일 디스크립터 교체, zero-downtime 목표.
13. Rollback: 30-day cooldown 적용 후 `Rollback(toVersion string, reason string) error`.
14. Evaluation: `Evaluate(adapterPath, testSet) (Score, error)` — Rust 가 계산한 score 래퍼. threshold < 0.75 시 자동 rollback 트리거 hook 제공.
15. Training Condition Gate — REQ-LORA-014 참조 (min conversations, data quality, SAFETY-001 통과, 사용자 승인).

### 3.2 OUT OF SCOPE

- **Tensor 연산 / gradient 계산**: Rust `goose-ml`.
- **ONNX Runtime GenAI 호출**: Rust 내부.
- **Hypernetwork (Doc-to-LoRA, Text-to-LoRA)**: 차기 별도 SPEC (learning-engine §5.4).
- **Federated LoRA aggregation**: Phase 7+ privacy.
- **Rust crate 버전 관리 / 빌드**: ROADMAP-RUST.md.
- **Base model 자체 배포·다운로드**: 사용자가 직접 제공 (`--lora.base-model-path`), 자동 다운로드 미지원.
- **A/B 실험 트래픽 분할**: 본 SPEC 은 단일 current adapter + 단일 candidate 만.
- **Multi-tenant 동시 훈련**: 사용자당 1 훈련 job, 동시 실행 금지.

---

## 4. 목표 (Goals)

- Go↔Rust 경계 오버헤드: gRPC 모드에서 per-call p95 ≤ 2 ms (localhost), CGO 모드에서 ≤ 50 µs.
- 200 example dataset 훈련 시작부터 완료까지 Rust 에서 응답한 estimated time 을 Go 가 90%+ 정확도로 사용자에게 보고.
- 1 versionRegistry transaction 의 내구성: 쓰기 실패 시 `versions.json` 은 변경 전 상태로 유지 (원자 rename).
- Hot-swap 실행 중 in-flight inference 요청 drop 수 0 (drain grace = 5s).
- Rollback 요청은 SAFETY-001 approval_manager 를 경유한 경우에만 실행.
- 훈련 실패 시(Rust OOM, ONNX 오류) Go 레이어가 SAFETY-001 에 이벤트 publish 하고 사용자에게 원문 오류를 익명화 없이 반환.

## 5. 비목표 (Non-Goals)

- 1 사용자 당 여러 base model 동시 운용.
- LoRA 파일 자체의 상세 binary serde (Rust 책임).
- 훈련 중 모델 quantization 방식 교체(4-bit ↔ 8-bit 동적 변경).
- 사용자별 LoRA 의 클라우드 상호 공유 (learning-engine §5.3.b cloud-assisted 는 본 SPEC 범위 외 — 별도 SPEC 필요).

---

## 6. 요구사항 (EARS Requirements)

### REQ-LORA-001 [Ubiquitous]
The LoRA service shall expose the `LoRATrainer` Go interface with method set: `PrepareDataset`, `Train`, `Evaluate`, `Merge`, `Apply`, `Rollback`, `ListVersions`, `Delete`, `Close`.

### REQ-LORA-002 [Ubiquitous]
All tensor-level computation (gradient, quantization, ONNX inference) shall be delegated to the external `goose-ml` Rust service via gRPC or CGO. The Go layer shall not implement tensor arithmetic.

### REQ-LORA-003 [State-Driven]
While the configuration value `lora.ipc` is `grpc` (default), the Go layer shall communicate with `goose-ml` over a gRPC connection at `lora.grpc_endpoint` (default `unix:///tmp/goose-ml.sock`).

### REQ-LORA-004 [State-Driven]
While the configuration value `lora.ipc` is `cgo`, the Go layer shall call into the Rust static library via a CGO bridge defined in `internal/learning/lora/cgo/`. This build path shall require the `cgo_lora` build tag.

### REQ-LORA-005 [Event-Driven]
When `Train(ctx, cfg)` is invoked, the service shall first call the Training Condition Gate; if any precondition fails (see REQ-LORA-014), it shall return `ErrTrainingConditionFailed` without contacting the Rust backend.

### REQ-LORA-006 [Event-Driven]
When `PrepareDataset(userID, opts)` is invoked, the service shall build a JSON-L dataset of size `[opts.MinSamples, opts.MaxSamples]` (default 200..500) by:
  (a) querying VECTOR-001 for interactions with cosine similarity >= `opts.MinSimilarity` (default 0.7),
  (b) filtering by IDENTITY-001 currently-valid facts,
  (c) stripping PII via the redaction pipeline registered in TRAJECTORY-001.

### REQ-LORA-007 [Event-Driven]
When `Train` succeeds, the resulting adapter file shall be written to `$GOOSE_HOME/lora/<userID>/<semver>.safetensors` and registered in `versions.json` atomically (temp file + `os.Rename`).

### REQ-LORA-008 [Event-Driven]
When `Apply(version)` is called, the service shall (a) drain in-flight inference requests for up to 5 seconds, (b) swap the active adapter file descriptor, (c) emit a `lora.applied` event via SAFETY-001 approval manager. Drop count during drain shall be 0 unless drain times out.

### REQ-LORA-009 [Event-Driven]
When `Evaluate(adapterPath, testSet)` is called, the service shall delegate to the Rust backend and receive `EvaluationScore{mean: float64, perCriterion: map[string]float64, confidence: float64}`; if `mean < cfg.RollbackThreshold` (default 0.75), the service shall emit a `lora.regression` event.

### REQ-LORA-010 [Event-Driven]
When `Rollback(toVersion, reason)` is invoked, the service shall refuse unless SAFETY-001 `approval_manager.IsApproved("lora.rollback", version, reason)` returns true; approved rollback shall update the active pointer and record an entry in the rollback log.

### REQ-LORA-011 [Unwanted]
The service shall not initiate training if the rate limiter (SAFETY-001) reports that the weekly evolution budget is exhausted.

### REQ-LORA-012 [Unwanted]
The service shall not expose training jobs as concurrent per user; a second `Train` call while one is in progress shall return `ErrTrainingInProgress`.

### REQ-LORA-013 [State-Driven]
While a LoRA version is within its 30-day cooldown window (`created_at + 30d > now`), it shall not be permanently deleted; `Delete(version)` shall soft-mark the version but keep the file until cooldown expires.

### REQ-LORA-014 [Complex]
While the Training Condition Gate evaluates preconditions, when any of (`min_conversations >= 1000`, `data_quality >= 0.75`, `frozen_guard_passed == true`, `rate_limit_passed == true`, `user_approval_granted == true`) returns false, the service shall return `ErrTrainingConditionFailed` with the failing condition name in the error payload.

### REQ-LORA-015 [Optional]
Where the caller supplies `WithEWC(fisher)`, the service shall forward the Fisher Information vector to the Rust backend as part of the training request; absent value means EWC is disabled.

### REQ-LORA-016 [Optional]
Where the caller supplies `WithLwF(prevAdapterPath)`, the service shall forward the previous adapter's path to the Rust backend for knowledge distillation; absent value means LwF is disabled.

### REQ-LORA-017 [Optional]
Where the caller supplies `WithReplayBuffer(buf)`, the service shall mix `buf.Sample(batchSize)` into each training batch according to the ratio defined in `cfg.ReplayRatio` (default 0.2).

### REQ-LORA-018 [Event-Driven]
When the Rust backend reports a fatal error (OOM, CUDA/Metal failure, ONNX parse error), the Go layer shall propagate the error verbatim to the caller wrapped in `ErrRustBackend{Cause}` and shall not swallow or summarize the root cause.

### REQ-LORA-019 [Ubiquitous]
Every write to `versions.json` shall include a SHA-256 checksum of the adapter file; subsequent `Apply(version)` shall verify the checksum and refuse to apply on mismatch (`ErrChecksumMismatch`).

### REQ-LORA-020 [State-Driven]
While the health-check to `goose-ml` Rust service fails for 10 consecutive seconds, the service shall mark itself `degraded` and reject `Train`/`Apply` calls with `ErrBackendDegraded`; `Evaluate` and read-only `ListVersions` shall remain available.

---

## 7. 설계 결정 (Design Decisions)

### DD-LORA-01 — gRPC 기본, CGO 선택

gRPC 의 ~1-2ms overhead 는 "훈련 30분-2h" 스케일에서는 무시 가능. 또 Rust crate 를 독립 프로세스로 두면 OOM/crash 가 Go 데몬을 죽이지 않음. CGO 는 latency-critical inference path 전용(차후 PR).

### DD-LORA-02 — Go 는 tensor 값을 조작하지 않는다

metadata(version, checksum, size_bytes) 만 읽고, 파일 바이트는 opaque. safetensors 파싱 같은 구조 해석도 Rust. 이것은 **hard invariant** — Go 코드 리뷰 시 `import "gonum.org/v1/gonum/mat"` 등 발견되면 PR reject.

### DD-LORA-03 — Training Condition Gate

learning-engine.md §5.2 의 5 조건을 명시적으로 게이트화 → 사용자 승인 없는 훈련 절대 시작 안 됨. 이 게이트는 SAFETY-001 과 **함께** 호출되어 Layer 1-5 보호 (FrozenGuard, RateLimiter, Approval) 모두 거친다.

### DD-LORA-04 — versions.json 원자성

`versions.json.tmp` 쓰기 → fsync → rename. 중단 시점 어디든 복구 가능. JSON 스키마는 버전 필드 `schema_version: 1` 로 미래 migration 대비.

### DD-LORA-05 — 30-day cooldown

Rollback 가능성을 보장하기 위해 삭제 미루기. 디스크 용량 초과 시 `goose lora gc --force` 로 수동 삭제 (경고 메시지 + 사용자 확인).

### DD-LORA-06 — Base model 자동 다운로드 거부

HuggingFace 미러에서 자동 fetch 는 licensing/bandwidth/공급망 리스크. 사용자가 직접 경로 지정.

### DD-LORA-07 — Continual Learning 은 Hook 주입

Go 가 EWC/LwF/Replay 각각의 "데이터 공급 책임" 을 가지지만 수치 연산은 Rust. `Trainer` 인터페이스가 option functional 패턴(`WithEWC` 등) 으로 표현 → 조합이 설정파일이 아닌 코드로 명확.

### DD-LORA-08 — Evaluate 실패 시 자동 rollback 아님

자동 rollback 은 위험. Go 는 `lora.regression` 이벤트만 publish → SAFETY-001 approval_manager 가 사용자에게 AskUserQuestion → 승인 시에만 rollback. ROLLBACK-001 이 이 이벤트 consumer.

---

## 8. 데이터 모델 (Data Model)

```go
// internal/learning/lora/types.go

// LoRAAdapter — metadata only (actual weights는 Rust side)
type LoRAAdapter struct {
    UserID         string
    Version        string           // SemVer, e.g., "v1.2.0"
    BaseModel      BaseModel        // qwen3-0.6b | gemma-1b | custom
    Rank           int              // 기본 16
    Alpha          int              // 기본 32
    TargetModules  []string         // ["q_proj", "v_proj", "output"]
    UseDoRA        bool             // 기본 true
    UseQLoRA       bool             // 기본 true (4-bit)
    FilePath       string           // $GOOSE_HOME/lora/<userID>/<version>.safetensors
    SizeBytes      int64
    Checksum       string           // SHA-256 hex
    CreatedAt      time.Time
    TrainingConfig TrainingConfig
}

type BaseModel string

const (
    BaseQwen3_0_6B BaseModel = "qwen3-0.6b"
    BaseGemma_1B   BaseModel = "gemma-1b"
    BaseCustom     BaseModel = "custom"
)

type TrainingConfig struct {
    BatchSize      int          // 기본 8
    LearningRate   float64      // 기본 1e-4
    Epochs         int          // 기본 3
    QuantBits      int          // 기본 4 (QLoRA)
    ReplayRatio    float64      // 기본 0.2
    MaxWallClock   time.Duration // 기본 2시간
    DatasetPath    string        // JSON-L 파일
    NumSamples     int
    DataQuality    float64       // [0,1]
    PreviousVersion string       // LwF 용
}

type EvaluationScore struct {
    Mean         float64
    PerCriterion map[string]float64
    Confidence   float64
    Evaluator    string   // "goose-ml:<version>"
}

// LoRATrainer — 외부 소비자가 쓰는 유일한 인터페이스
type LoRATrainer interface {
    PrepareDataset(ctx context.Context, userID string, opts DatasetOptions) (TrainingConfig, error)
    Train(ctx context.Context, cfg TrainingConfig, options ...TrainOption) (LoRAAdapter, error)
    Evaluate(ctx context.Context, adapterPath string, testSetPath string) (EvaluationScore, error)
    Merge(ctx context.Context, base, delta string) (string, error)   // 고급 (optional)
    Apply(ctx context.Context, version string) error
    Rollback(ctx context.Context, toVersion, reason string) error
    ListVersions(ctx context.Context, userID string) ([]LoRAAdapter, error)
    Delete(ctx context.Context, version string) error
    Close(ctx context.Context) error
}

type DatasetOptions struct {
    MinSamples     int     // 기본 200
    MaxSamples     int     // 기본 500
    MinSimilarity  float64 // 기본 0.7 (VECTOR-001 cosine)
    OnlyValidFacts bool    // 기본 true (IDENTITY-001 filter)
    RedactPII      bool    // 기본 true
}

type TrainOption func(*trainOpts)

type trainOpts struct {
    ewcFisher        []float32
    lwfPrevAdapter   string
    replayBuffer     ReplayBuffer
    rollbackOnRegression bool    // 기본 false (DD-LORA-08)
}

func WithEWC(fisher []float32) TrainOption { ... }
func WithLwF(prevAdapter string) TrainOption { ... }
func WithReplayBuffer(buf ReplayBuffer) TrainOption { ... }

type ReplayBuffer interface {
    Add(sample LearningExample)
    Sample(batchSize int) []LearningExample
    Size() int
    Capacity() int
}

// Training condition gate
type TrainingConditionGate interface {
    Check(ctx context.Context, userID string) (ConditionReport, error)
}

type ConditionReport struct {
    MinConversationsMet bool
    DataQualityMet      bool
    FrozenGuardPassed   bool
    RateLimitPassed     bool
    UserApprovalGranted bool
    Pass                bool
    FailingCondition    string // 첫 번째 실패 이름
}
```

---

## 9. gRPC 프로토콜 (요약, 상세는 research.md)

```
proto/lora/v1/trainer.proto

service LoraTrainer {
    rpc Train(TrainRequest) returns (stream TrainProgress);
    rpc Evaluate(EvaluateRequest) returns (EvaluateResponse);
    rpc Merge(MergeRequest) returns (MergeResponse);
    rpc HealthCheck(google.protobuf.Empty) returns (HealthStatus);
}

message TrainRequest {
    string user_id = 1;
    string dataset_path = 2;
    string base_model = 3;
    int32 rank = 4;
    int32 alpha = 5;
    repeated string target_modules = 6;
    int32 quant_bits = 7;
    bytes ewc_fisher = 8;           // optional
    string lwf_prev_adapter = 9;    // optional
    float replay_ratio = 10;
    int32 batch_size = 11;
    float learning_rate = 12;
    int32 epochs = 13;
    int64 max_wallclock_seconds = 14;
}

message TrainProgress {
    enum Stage { INIT = 0; LOADING = 1; TRAINING = 2; EVALUATING = 3; SAVING = 4; DONE = 5; FAILED = 6; }
    Stage stage = 1;
    int32 epoch = 2;
    float loss = 3;
    string message = 4;
    string output_path = 5;        // DONE 시점
    string checksum = 6;           // DONE 시점
}
```

실제 .proto 파일은 Rust/Go 공용 저장소 `proto/lora/v1/` 에 commit. 생성 규약: `buf generate` (proto-gen-go + tonic).

---

## 10. Exclusions (What NOT to Build)

- Tensor arithmetic / gradient computation (→ Rust `goose-ml`).
- ONNX Runtime GenAI 직접 호출 (→ Rust 내부).
- Hypernetwork 즉시 개인화 (Doc-to-LoRA / Text-to-LoRA; → 차기 SPEC).
- Federated LoRA aggregation (→ Phase 7+ privacy).
- Cloud-assisted fine-tuning (Claude Haiku, GPT-4o-mini; learning-engine §5.3.b) — 별도 SPEC 필요.
- Base model 자동 다운로드 / HuggingFace hub 연동.
- A/B traffic split 서빙 (단일 current adapter 만).
- Multi-tenant 동시 훈련 job.
- Tensor 압축 이후의 format migration (safetensors v1 → v2 등).

---

## 11. Acceptance Criteria

### AC-LORA-001 — 인터페이스 계약
Given `NewLoRATrainer(cfg)` 로 서비스를 초기화한 뒤, When `ListVersions(userID)` → `PrepareDataset` → `Train` → `Evaluate` → `Apply` 를 순차 호출할 때, Then 모든 호출이 정의된 에러 외 실패 없이 완료되고 최종 `Apply` 이후 `ListVersions` 결과에서 해당 버전의 `active=true` 를 관찰 가능하다.

### AC-LORA-002 — Tensor 미조작 불변식 (Code Invariant)
Go 패키지 `internal/learning/lora/` 하위의 어떤 `.go` 파일도 `gonum.org/v1/gonum/mat`, `gorgonia.org/gorgonia`, `github.com/sugarme/gotch` 등 tensor 라이브러리를 import 하지 않는다. (CI 의 import-lint 규칙으로 강제.)

### AC-LORA-003 — Training Condition Gate 차단
Given `rate_limiter.ConsumeWeeklyBudget()` 가 false 를 반환하도록 모킹된 상태에서, When `Train(ctx, cfg)` 를 호출하면, Then `ErrTrainingConditionFailed{FailingCondition:"rate_limit_passed"}` 가 반환되고 Rust gRPC 서비스로 어떤 요청도 전송되지 않는다 (spy 로 검증).

### AC-LORA-004 — 데이터셋 빌드
Given VECTOR-001 이 10 개의 cosine>=0.7 interaction 을 반환하고 IDENTITY-001 이 모두 valid 로 마킹할 때, When `PrepareDataset(userID, {MinSamples:200})` 를 호출하면, Then `ErrInsufficientSamples` 가 반환된다 (200 미만).

### AC-LORA-005 — versions.json 원자성
Given 훈련이 SAVING 단계에서 `SIGKILL` 로 중단되는 상황을 시뮬레이션할 때, When 프로세스 재시작 후 `ListVersions(userID)` 를 호출하면, Then `versions.json` 은 중단 이전 상태와 동일하고 유령 버전이 없다.

### AC-LORA-006 — Hot-swap Drain
Given 100 concurrent inference 요청이 진행 중일 때 `Apply(newVersion)` 를 호출하면, Then 5 초 drain window 내에 모든 요청이 완료되고 drop 카운트는 0 이며, swap 이후 첫 요청은 `newVersion` 을 사용한다.

### AC-LORA-007 — 회귀 이벤트 발행
Given `Evaluate(adapter, testSet)` 이 `mean=0.70` 을 반환할 때 (threshold=0.75), When 호출이 완료되면, Then `lora.regression` 이벤트가 SAFETY-001 approval_manager 에 정확히 1번 발행되고 자동 rollback 은 수행되지 않는다.

### AC-LORA-008 — Rollback 승인 필수
When `Rollback("v1.1.0", "user_dislike")` 를 호출하되 `approval_manager.IsApproved(...)` 가 false 를 반환하도록 모킹하면, Then `ErrRollbackNotApproved` 가 반환되고 active pointer 는 변경되지 않는다.

### AC-LORA-009 — 30-day Cooldown
Given `v1.0.0` 이 `created_at = time.Now() - 20*24h` 일 때, When `Delete("v1.0.0")` 를 호출하면, Then 파일은 디스크에 남아있고 `versions.json` 에서는 `soft_deleted=true` 로만 마킹된다.

### AC-LORA-010 — Checksum 불일치 거부
Given `v2.0.0` 파일이 외부 도구로 1 byte 변조되었을 때, When `Apply("v2.0.0")` 를 호출하면, Then `ErrChecksumMismatch` 가 반환되고 swap 이 수행되지 않는다.

### AC-LORA-011 — Backend 에러 전파
Given Rust 서비스가 `"CUDA out of memory"` 를 gRPC status `RESOURCE_EXHAUSTED` 로 반환할 때, When `Train` 을 호출하면, Then Go caller 는 `ErrRustBackend{Cause: "CUDA out of memory"}` 를 받으며 원문 메시지가 그대로 포함된다.

### AC-LORA-012 — Degraded 모드
Given `goose-ml` 서비스가 10 초간 health-check 실패할 때, When `Train` 을 호출하면, Then `ErrBackendDegraded` 가 반환되고 `ListVersions` 는 여전히 정상 응답한다.

---

## 12. 테스트 전략 (요약, 상세는 research.md)

- 단위: `version_registry_test.go`(원자 rename), `condition_gate_test.go`(5 조건 조합), `option_test.go`(WithEWC 등).
- Mock gRPC server (Go): `goose-ml` 프로토콜 시뮬레이터. 각 `TrainProgress` 스트림 단계 검증.
- 통합: 실제 Rust `goose-ml` 바이너리 (빌드 artifact) + `-tags=integration`.
- Import-lint: `tools/check_imports.go` 가 tensor 라이브러리 금지 규칙 적용 (AC-LORA-002).
- 커버리지: 85%+.
- Chaos: SIGKILL 중단, 네트워크 끊김, 파일 시스템 full.

---

## 13. 의존성 & 영향 (Dependencies & Impact)

- **상위 의존**: VECTOR-001 (데이터셋 빌드), IDENTITY-001 (fact 필터), SAFETY-001 (승인·롤백), REFLECT-001 (5-tier 승격 후 훈련 트리거), TRAJECTORY-001 (raw episode + redaction).
- **하위 소비자**: QueryEngine (SPEC-GOOSE-QUERY-001)의 inference path 가 `Apply(version)` 결과의 adapter 를 사용 (이는 ADAPTER-001 의 책임).
- **Rust crate**: `crates/goose-ml/` — 본 SPEC 의 범위 외. ROADMAP-RUST.md 에서 관리.
- **CLI 영향**: `goose lora list`, `goose lora train`, `goose lora rollback`, `goose lora gc`, `goose lora apply <version>`.

---

## 14. 오픈 이슈 (Open Issues)

1. **Rust 서비스 배포 전략**: goose-ml 을 `goosed` 내부에 embedded(port 공유)로 둘지, 별도 `goose-ml` 바이너리로 둘지. 현재 계획은 별도 프로세스 + unix socket.
2. **Base model 라이선스**: Qwen3-0.6B (Apache-2.0, OK) vs Gemma-1B (custom Gemma License, 재배포 제한). 두 모델 모두 사용자 다운로드 책임 명시.
3. **Fisher Information 직렬화 포맷**: []float32 를 gRPC bytes 로 보낼 때 compression 여부. 모델 파라미터 수가 수백만이면 수십 MB.
4. **Apple Silicon 전용 최적화**: MLX backend vs ONNX Runtime 차이를 Go 쪽에서 어떻게 config 로 노출할지. 현재는 `lora.runtime = onnx | mlx | coreml` 세 enum.
5. **Replay buffer 영속화**: ReplayBuffer 구현체의 저장소 (in-memory vs SQLite vs MEMORY-001 plugin). 기본은 in-memory, persistent 는 사용자 설정.
6. **CGO 빌드 복잡도**: `cgo_lora` 태그 시 CI/CD matrix 가 급격히 복잡해짐. Phase 6 첫 릴리스에서는 gRPC only 권장.
7. **proto 파일 위치**: 단일 `proto/` 디렉토리 vs Go·Rust 각자 복제. 단일 저장소 + buf breaking-change check 권장.
