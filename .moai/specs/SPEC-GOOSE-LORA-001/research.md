# SPEC-GOOSE-LORA-001 research.md — LoRA Trainer 구현 자료 (Go 레이어)

> 본 문서는 Go 인터페이스 측의 실구현 참고 자료이다. Rust `goose-ml` crate 의 내부 구현은 본 문서에 포함하지 않는다 (→ ROADMAP-RUST.md).

---

## 1. Go↔Rust 경계 아키텍처

### 1.1 프로세스 토폴로지

```
┌─────────────────────────────────────────────┐
│ goosed (Go daemon)                          │
│  internal/learning/lora/                    │
│    ├─ trainer.go        (공개 API)          │
│    ├─ grpc_client.go    (기본)              │
│    ├─ cgo_bridge.go     (build tag)         │
│    └─ registry.go       (versions.json)     │
└──────────────┬──────────────────────────────┘
               │ gRPC over unix:///tmp/goose-ml.sock
               ▼
┌─────────────────────────────────────────────┐
│ goose-ml (Rust process, spawned by goosed)  │
│  crates/goose-ml/                           │
│    ├─ src/server.rs       (tonic)           │
│    ├─ src/trainer.rs      (candle/burn)     │
│    ├─ src/onnx.rs         (ort)             │
│    └─ src/quantize.rs     (QLoRA 4-bit)     │
└─────────────────────────────────────────────┘
```

프로세스 spawn 은 `trainer_runner.go` 가 담당:
- `exec.Command("goose-ml", "--socket", sockPath, "--home", goosehome)`
- 표준 오류는 pipe 로 수신해서 Go 로그로 re-emit.
- Graceful shutdown: SIGTERM → 5s wait → SIGKILL fallback.

### 1.2 Health-check 프로토콜

- Go 측 5초 주기로 `HealthCheck` gRPC 호출.
- 연속 2회(10초) 실패 → `degraded` 상태, `Train/Apply` 거부 (REQ-LORA-020).
- 연속 1회(5초) 실패 + 프로세스 종료 감지 → restart (max 3 retries per 5min).

---

## 2. 외부 의존 (Go 측)

| 의존 | 버전 고정 | 용도 |
|-----|---------|----|
| `google.golang.org/grpc` | v1.66.x | gRPC 클라이언트 |
| `google.golang.org/protobuf` | v1.34.x | .proto 생성물 |
| `github.com/bufbuild/buf` | v1.35 (dev tool) | .proto 린팅 |
| `github.com/stretchr/testify` | v1.9.x | 테스트 |
| `golang.org/x/sync/semaphore` | latest | 단일 훈련 job 제약 |
| `github.com/cenkalti/backoff/v4` | v4.3.x | gRPC 재시도 |

**Rust 의존은 본 SPEC 범위 외** — ROADMAP-RUST.md 에서 별도 고정.

---

## 3. 패키지 디렉터리 계획

```
internal/learning/lora/
├── doc.go
├── types.go            // LoRAAdapter, TrainingConfig, EvaluationScore
├── errors.go
├── interface.go        // LoRATrainer
├── trainer.go          // NewLoRATrainer, orchestration
├── grpc_client.go      // gRPC 경로 (기본)
├── cgo/                // CGO 경로 (build tag)
│   ├── bridge.go
│   ├── bridge.h
│   └── bridge.c
├── dataset/
│   ├── builder.go      // PrepareDataset 구현
│   └── builder_test.go
├── registry/
│   ├── versions.go     // versions.json 원자 rename
│   └── versions_test.go
├── condition/
│   ├── gate.go         // TrainingConditionGate
│   └── gate_test.go
├── hotswap/
│   ├── swap.go         // Apply drain
│   └── swap_test.go
└── runner/
    ├── process.go      // goose-ml 프로세스 관리
    └── healthcheck.go
```

---

## 4. `versions.json` 스키마

```json
{
  "schema_version": 1,
  "user_id": "user-abc123",
  "active_version": "v1.2.0",
  "versions": [
    {
      "version": "v1.0.0",
      "base_model": "qwen3-0.6b",
      "rank": 16,
      "alpha": 32,
      "target_modules": ["q_proj", "v_proj", "output"],
      "use_dora": true,
      "use_qlora": true,
      "file_path": "$GOOSE_HOME/lora/user-abc123/v1.0.0.safetensors",
      "size_bytes": 104857600,
      "checksum": "sha256:abc...",
      "created_at": "2026-04-21T10:00:00Z",
      "soft_deleted": false,
      "training_config": { ... }
    }
  ],
  "rollback_log": [
    {
      "from": "v1.2.0",
      "to": "v1.1.0",
      "reason": "regression detected: mean=0.70",
      "approved_by": "user",
      "approved_at": "2026-04-21T11:00:00Z"
    }
  ]
}
```

원자 쓰기:

```go
func (r *Registry) Save() error {
    tmp := r.path + ".tmp"
    f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil { return err }
    if err := json.NewEncoder(f).Encode(r.data); err != nil { f.Close(); return err }
    if err := f.Sync(); err != nil { f.Close(); return err }
    if err := f.Close(); err != nil { return err }
    return os.Rename(tmp, r.path)
}
```

---

## 5. Training Condition Gate 흐름

```
Check(userID):
  report := ConditionReport{}

  1. MEMORY-001.CountConversations(userID)  >= 1000 ?
  2. TRAJECTORY-001.DataQualityScore(userID) >= 0.75 ?
  3. SAFETY-001.frozen_guard.Passed()              ?
  4. SAFETY-001.rate_limiter.HasWeeklyBudget()     ?
  5. SAFETY-001.approval_manager.HasApproval("lora.train", userID) ?

  report.Pass = all of the above
  if not pass: report.FailingCondition = first_failing_name
  return report
```

---

## 6. Hot-swap Drain 알고리즘

```
Apply(version):
  1. registry.Get(version) → adapter 메타 + checksum 검증 (REQ-LORA-019)
  2. inference_gate.SignalDraining()            // 신규 요청을 새 버전으로 라우팅 대기
  3. wait up to 5s for inflight counter == 0
     if timeout: 강제 swap 하되 drop 카운터 증가, warning 로그
  4. atomic pointer swap (active_version := version)
  5. inference_gate.SignalReady()
  6. emit lora.applied event → SAFETY-001
  7. registry.Save()
```

inflight counter 는 `atomic.Int64`. drain 실패 측정은 `metrics.lora_apply_drops_total`.

---

## 7. gRPC 서비스 호출 예시 (grpc_client.go)

```go
func (g *GRPCClient) Train(ctx context.Context, cfg TrainingConfig, opts trainOpts) (LoRAAdapter, error) {
    req := &pb.TrainRequest{
        UserId:          cfg.UserID,
        DatasetPath:     cfg.DatasetPath,
        BaseModel:       string(cfg.BaseModel),
        Rank:            int32(cfg.Rank),
        Alpha:           int32(cfg.Alpha),
        TargetModules:   cfg.TargetModules,
        QuantBits:       int32(cfg.QuantBits),
        EwcFisher:       float32SliceToBytes(opts.ewcFisher),
        LwfPrevAdapter:  opts.lwfPrevAdapter,
        ReplayRatio:     float32(cfg.ReplayRatio),
        BatchSize:       int32(cfg.BatchSize),
        LearningRate:    float32(cfg.LearningRate),
        Epochs:          int32(cfg.Epochs),
        MaxWallclockSeconds: int64(cfg.MaxWallClock.Seconds()),
    }

    stream, err := g.client.Train(ctx, req)
    if err != nil { return LoRAAdapter{}, wrap(err) }

    for {
        prog, err := stream.Recv()
        if err == io.EOF { break }
        if err != nil { return LoRAAdapter{}, wrap(err) }
        g.onProgress(prog)
        if prog.Stage == pb.TrainProgress_FAILED {
            return LoRAAdapter{}, &ErrRustBackend{Cause: prog.Message}
        }
        if prog.Stage == pb.TrainProgress_DONE {
            return LoRAAdapter{
                UserID:   cfg.UserID,
                FilePath: prog.OutputPath,
                Checksum: prog.Checksum,
                // 나머지는 cfg 에서 복사
            }, nil
        }
    }
    return LoRAAdapter{}, errors.New("stream ended without DONE")
}
```

---

## 8. TDD 테스트 전략

### 8.1 Import Lint (AC-LORA-002 핵심)

`tools/check_imports.go` 는 ci 에서 다음을 검사:
```go
var forbidden = []string{
    "gonum.org/v1/gonum/mat",
    "gorgonia.org/gorgonia",
    "github.com/sugarme/gotch",
    "github.com/apache/arrow/go",  // tensor 의심
}
```
`internal/learning/lora/**/*.go` 파일에서 위 import 가 발견되면 PR 실패.

### 8.2 Mock gRPC 서버

`test/mockml/server.go`:
- 설정 가능한 response: DONE / FAILED / 중간 중단
- Chaos 시나리오: 스트림 끊김, deadline 초과, 연결 리셋.

### 8.3 RED 단계

1. `registry_test.go`: `versions.json` 의 원자 rename 시나리오.
2. `condition_gate_test.go`: 5 조건 조합 중 실패 하나 존재하면 Pass=false.
3. `hotswap_test.go`: drain timeout 시 drop 카운터 1 증가.
4. `grpc_client_test.go`: FAILED stage → `ErrRustBackend` wrap.

### 8.4 GREEN 단계

- 각 테스트별 최소 구현 후 contract 만족.
- 실제 Rust 바이너리는 mock 으로 대체.

### 8.5 REFACTOR 단계

- `trainer.go` 의 option pattern 정리 (With* 함수 통합).
- `runner/process.go` 의 lifecycle 을 `context.Context` 기반으로 명확화.

### 8.6 통합 테스트 (-tags=integration)

- 실제 `goose-ml` 바이너리 spawn.
- 작은 합성 데이터셋 (10 samples) 으로 end-to-end 1분 이내 완료 확인.

### 8.7 커버리지 목표

- Go 전체 85%+.
- `registry/`, `condition/`, `hotswap/` 패키지 90%+.

---

## 9. 성능·리소스 목표

| 항목 | 목표 |
|----|----|
| gRPC per-call overhead (localhost) | p95 ≤ 2 ms |
| CGO call overhead (옵션) | p95 ≤ 50 µs |
| `versions.json` write (after training) | ≤ 50 ms |
| Hot-swap drain (100 inflight, 각 50 ms) | ≤ 5 s |
| `Apply` 후 첫 inference 지연 | ≤ 200 ms |
| Go 패키지 메모리 (idle) | ≤ 50 MB |
| Rust 프로세스 메모리 (training idle) | ≤ 200 MB |
| Rust 프로세스 메모리 (training active, Qwen3-0.6B 4-bit) | ≤ 3 GB |

---

## 10. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|------|----|----|
| Go 개발자가 실수로 tensor lib 추가 | High | Import lint (AC-LORA-002) + PR template 체크박스 |
| Rust 프로세스 crash loop | High | max 3 retries / 5min, 초과 시 `degraded` 유지 + 사용자 알림 |
| versions.json 동시 쓰기 | High | 파일 단일 writer (서비스 내부 mutex) + 원자 rename |
| gRPC proto breaking change | Medium | `buf breaking` CI gate, 양측 동시 PR 필수 |
| Base model 라이선스 위반 | High | 자동 다운로드 금지, 사용자 명시 동의 필요 (Gemma license) |
| 30-day cooldown 디스크 부족 | Medium | `goose lora gc --force` CLI + 사용자 확인 |
| EWC Fisher 벡터 bandwidth | Low | 압축(gzip) 옵션, default off |
| CGO 플랫폼 편차 | High | Phase 6 첫 릴리즈는 gRPC only, CGO 는 후속 |
| Go process 종료 시 goose-ml orphan | Medium | parent-death signal (Linux prctl, macOS kqueue), 아니면 heartbeat timeout self-exit |

---

## 11. 관측·로깅

- Prometheus metrics:
  - `goose_lora_train_total{result=success|failed}`
  - `goose_lora_train_duration_seconds`
  - `goose_lora_apply_drops_total`
  - `goose_lora_backend_degraded_seconds`
  - `goose_lora_rollback_total{reason=...}`
- Structured logs (zap):
  - `lora.train.start` / `lora.train.done` / `lora.train.failed`
  - `lora.apply.drain_timeout` (warn)
  - `lora.regression.detected` (warn)
  - `lora.backend.down` (error)

---

## 12. CLI 스케치 (CLI-001 후속)

```bash
$ goose lora list
Active: v1.2.0 (created 2026-04-14, size 104 MB, sha256:abc...)
History:
  v1.1.0 (2026-03-28, soft_deleted=false)
  v1.0.0 (2026-03-14, soft_deleted=true, cooldown until 2026-04-13 → safe to gc)

$ goose lora train --replay-ratio 0.3 --use-ewc
[gate] conversations=1240 ✅ quality=0.81 ✅ frozen=OK ✅ rate=OK ✅ approval=granted ✅
[spawn] goose-ml PID 42311, socket /tmp/goose-ml.sock
[progress] LOADING epoch=0 …
[progress] TRAINING epoch=3 loss=0.42
[progress] EVALUATING mean=0.88
[progress] DONE → v1.3.0 saved (108 MB, sha256:def…)

$ goose lora rollback v1.2.0 --reason "new version misses formal tone"
[approval] requested → user approved via AskUserQuestion
[rollback] active_version := v1.2.0, logged
```

---

## 13. 추적성

- spec.md REQ-LORA-001..020 → 본 문서 §1 (경계), §4 (versions.json), §5 (gate), §6 (hotswap), §8 (TDD).
- AC-LORA-001..012 → 본 문서 §8 (mock 서버 + chaos).
- learning-engine.md §5.2 (5 조건) → spec.md REQ-LORA-014 (TrainingConditionGate).
- learning-engine.md §6.2 (EWC/LwF/Replay) → spec.md REQ-LORA-015..017 (TrainOption).
- learning-engine.md §6.3 (회귀 감지) → spec.md REQ-LORA-009, AC-LORA-007, DD-LORA-08.
- tech.md §2.1 (goose-ml Rust crate) → 본 SPEC 의 Rust 위임 경계.

---

## 14. Rust 위임 경계 재확인

| 시그널 | 관찰 방법 |
|----|----|
| Go 는 tensor 값을 조작하지 않는다 | Import lint (§8.1) |
| proto 는 Go·Rust 공유 | `proto/lora/v1/trainer.proto` 단일 출처, `buf generate` |
| safetensors 는 opaque | Go 는 SHA-256 만 계산, 파싱 없음 |
| Rust 프로세스 독립 배포 | `goose-ml` 바이너리 별도 릴리즈, goosed 가 exec 로만 호출 |
| gRPC breaking change 감지 | CI 의 `buf breaking --against` 차단 |

본 SPEC 의 구현이 완료된 시점에도 Rust crate 의 진전이 없으면 **mock 서버로 end-to-end 통과** 가능하도록 테스트 스위트가 분리되어 있다 → Rust 스케줄과 디커플.
