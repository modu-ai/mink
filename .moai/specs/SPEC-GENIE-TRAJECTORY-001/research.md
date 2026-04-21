# SPEC-GENIE-TRAJECTORY-001 — Research & Porting Analysis

> **목적**: Hermes Agent의 `trajectory.py` / `trajectory_compressor.py` 상단 데이터 구조를 Go로 이식할 때의 결정점, 재사용 가능 패턴, Go 이디엄 재작성 영역을 정리한다. `.moai/project/research/hermes-learning.md` §1-2의 분석 결과를 본 SPEC 요구사항과 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/learning/trajectory/` 단일 패키지 + `redact/` 서브패키지.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/
.claude  .moai  CLAUDE.md  README.md
claude-code-source-map/   # TypeScript (Trajectory 관련 없음)
hermes-agent-main/        # Python 원천
```

- `internal/learning/` → **전부 부재**. Phase 4에서 신규 작성.
- 상위 SPEC-GENIE-CORE-001이 수립할 `GENIE_HOME` 해석, zap 로거, `~/.genie/` 디렉토리 보장을 전제.
- SPEC-GENIE-QUERY-001이 제공하는 `PostSamplingHooks`, `StopFailureHooks`, Terminal 이벤트의 consumer로만 동작.

**결론**: 본 SPEC의 GREEN 단계는 `internal/learning/trajectory/` 패키지를 **zero-to-one 신규 작성**이며, Hermes Python의 I/O 계약과 스키마만 계승한다.

---

## 2. hermes-learning.md §2 원문 → SPEC 요구사항 매핑

hermes-learning.md §2 Trajectory 스키마 원문:

```python
entry = {
    "conversations": [
        {"from": "system"|"human"|"gpt"|"tool", "value": str},
        ...
    ],
    "timestamp": datetime.isoformat(),
    "model": str,                          # e.g., "anthropic/claude-opus"
    "completed": bool,
}

# 저장
trajectory_samples.jsonl      # 성공
failed_trajectories.jsonl     # 실패
```

스키마 요소 → 본 SPEC REQ 매핑:

| Hermes 원문 필드 | 본 SPEC 필드 | REQ | Go 매핑 |
|---|---|---|---|
| `conversations[{from, value}]` | `Trajectory.Conversations []TrajectoryEntry` | REQ-TRAJECTORY-002 | JSON tag 그대로 |
| `timestamp` ISO | `Trajectory.Timestamp time.Time` | REQ-TRAJECTORY-005 | UTC 기준, `time.Time` 기본 RFC3339 |
| `model` | `Trajectory.Model string` | (스키마 preservation) | QueryEngine 주입 값 그대로 |
| `completed bool` | `Trajectory.Completed bool` | REQ-TRAJECTORY-005/006 | success/failed 분기 근거 |
| `trajectory_samples.jsonl` | `success/YYYY-MM-DD.jsonl` | REQ-TRAJECTORY-005 | 날짜 분리 + 회전 |
| `failed_trajectories.jsonl` | `failed/YYYY-MM-DD.jsonl` | REQ-TRAJECTORY-006 | 같음 |

**신규 추가 필드**(Hermes에는 없음):
- `SessionID` — MEMORY-001 / INSIGHTS-001과의 조인 키로 필수.
- `Metadata.FailureReason` — QueryEngine Terminal의 `error` 필드 보존. ERROR-CLASS-001이 소비.
- `Metadata.Partial` — REQ-TRAJECTORY-012 buffer spill 식별자.

---

## 3. Python → Go 이식 결정

### 3.1 비동기 I/O 전략

Hermes는 Python `asyncio` + `aiofiles`로 비동기 append를 수행한다. Go 이식 시:

| Python 이디엄 | Go 이디엄 | 근거 |
|---|---|---|
| `async def write_trajectory()` + `aiofiles` | 전용 worker goroutine + `chan Trajectory` | REQ-TRAJECTORY-013 (1ms 차단 금지) |
| `asyncio.Queue` | `chan hookEvent` (buffered, capacity 256) | backpressure는 drop-oldest 대신 block at call site OK (1ms 내 send 완료 보장) |
| `logging.Logger.warning()` | `zap.Logger.Warn()` | 구조화 필드 `session_id`, `path` |
| file handle 관리 | `sync.Mutex` 보호된 `map[bucket]*openFile` | 날짜별 자동 rollover |

### 3.2 Redact 전략

Hermes는 redact 파이프라인이 없거나 (확인된 원본 소스 범위 내) 최소한이다. 본 SPEC이 추가하는 신규 기능이며, 다음 근거로 정당화:

- **GDPR Art.25 privacy-by-design**: 처리 시점에 데이터 최소화.
- **CCPA §1798.100**: 개인정보 수집 사실 통지 + 삭제권. Redact된 궤적은 "식별 불가능"으로 간주되어 삭제 의무 외.
- **ICLR 2026 Lifelong Agents Workshop**: LoRA 훈련 데이터에 PII가 포함되면 모델이 PII를 생성해버리는 취약점 보고됨(재현 가능).

Go 구현: `regexp.Regexp` RE2 엔진은 catastrophic backtracking이 없어 악의적 입력에 안전하다.

### 3.3 Rotation 전략 비교

| 전략 | 장점 | 단점 | 결정 |
|---|---|---|---|
| **날짜 + 크기 복합**(본 SPEC) | 일별 파티션 + 대형 세션도 파일 크기 bounded | 회전 인덱스 관리 | **채택** |
| 크기만 | 단순 | 날짜 기반 retention 난이도 상승 | 채택 안 함 |
| 날짜만 | 단순 | 단일 세션이 GB급일 때 파일 팽창 | 채택 안 함 |
| lumberjack 라이브러리 | 검증된 rotation | 날짜 기반 retention 기본 미지원 | 채택 안 함 (직접 구현이 retention 통합 용이) |

---

## 4. Go 라이브러리 결정

| 용도 | 채택 | 대안 | 근거 |
|---|---|---|---|
| JSON encode | 표준 `encoding/json` | `goccy/go-json`, `json-iterator/go` | Trajectory 쓰기는 I/O bound (JSON 속도가 병목 아님). 표준 라이브러리로 외부 의존성 축소 |
| 정규식 | 표준 `regexp` (RE2) | `dlclark/regexp2` (PCRE) | PCRE backtracking 보안 리스크. 모든 6 built-in 규칙은 RE2로 표현 가능 |
| 시계(테스트) | `github.com/jonboulle/clockwork` | `benbjohnson/clock` | `clockwork`가 Go 1.22 generics 호환, 최근 유지보수 활발 |
| 파일 rotation | 직접 구현 | `natefinch/lumberjack` | retention + date rollover 통합 난이도. 본 SPEC의 파일 수가 적어(일별 2개) 복잡성 낮음 |
| 토큰 카운팅(향후) | N/A | `tiktoken-go` | 본 SPEC 범위 외. COMPRESSOR-001에서 결정 |

---

## 5. 테스트 전략

### 5.1 격리 테스트 환경

- 각 테스트마다 `t.TempDir()`로 격리된 `GENIE_HOME` 생성 → 병렬 테스트 안전(`t.Parallel()` OK).
- `clockwork.FakeClock`으로 시간 주입 → 날짜 rollover, retention 검증.
- QueryEngine 훅은 실제 engine 없이 collector의 `OnTurn` / `OnTerminal`을 직접 호출하여 단위 테스트.

### 5.2 동시성 테스트

AC-TRAJECTORY-012는 `-race` 플래그 필수:

```
go test -race -count=10 ./internal/learning/trajectory/...
```

`count=10`으로 반복 실행하여 flakiness 조기 발견.

### 5.3 Redact 규칙 테스트 표

각 built-in 규칙에 대해 positive 3건 + negative 2건:

| 규칙 | Positive | Negative |
|---|---|---|
| email | `a@b.com`, `first.last@sub.example.co.kr`, `test+tag@x.io` | `@ no-local-part`, `invalid@` |
| openai_key | `sk-proj-abc123def456ghi789...`, `sk-abc123...` | `sk-short` (길이 미만) |
| bearer_jwt | `Bearer eyJhbGci.payload.sig` | `Bearer plain_token` (JWT 아님) |
| credit_card | 4111-1111-1111-1111 (Visa test), 5555 5555 5555 4444 | phone number 010-1234-5678 (Luhn fail → negative) |
| kr_phone | `010-1234-5678`, `011-123-4567` | `02-1234-5678` (지역번호는 범위 외) |
| home_path | `/Users/alice/code`, `/home/bob/.ssh` | `/Users/`, `/tmp/alice` (home 아님) |

### 5.4 Integration 테스트

`TestE2E_QueryEngineToDisk` — Mock QueryEngine을 구동하여 2턴 대화 후 디스크 파일 검증. CORE-001 / QUERY-001 stub이 필요하므로 본 SPEC GREEN 단계 마지막에 배치.

---

## 6. 설정 스키마 (참고)

`.moai/config/sections/telemetry.yaml` (CONFIG-001이 정의, 본 SPEC은 consumer):

```yaml
telemetry:
  trajectory:
    enabled: true
    retention_days: 90
    max_file_bytes: 10485760   # 10MB
    in_memory_turn_cap: 1000
    redact_rules:              # 사용자 정의, built-in에 append
      - name: "employee_id"
        pattern: '\b[A-Z]{2}\d{6}\b'
        replacement: "<REDACTED:emp_id>"
        applies_to_system: false
```

---

## 7. Hermes 재사용 평가

| Hermes 구성요소 | 재사용 가능성 | 재작성 필요 이유 |
|---|---|---|
| `trajectory.py` 스키마 (`conversations[]`) | **100% 재사용**(필드 이름 그대로) | — |
| ISO timestamp | **100% 재사용** | — |
| success/failed 이분법 | **100% 재사용** | — |
| JSON-L append 로직 | **0% 재사용** | Python `json.dumps` vs Go `encoding/json`, 파일 핸들 관리 상이 |
| asyncio 이벤트 연결 | **0% 재사용** | Go goroutine + channel로 완전 재작성 |
| Redact 규칙 | **신규 추가** | Hermes 원본에 없음 |
| Retention sweep | **신규 추가** | Hermes 원본에 없음 |

**추정 Go LoC**:
- types.go: 60
- collector.go: 180
- writer.go: 220 (rotation 포함)
- rotation.go: 80
- retention.go: 90
- redact/{redactor, rules, config}.go: 190
- 테스트 코드: 600+
- **합계**: ~820 production + 600 test ≈ 1,400 LoC

hermes-learning.md §10 매핑 표(`learning/trajectory: 100 LoC`)와 비교하면 본 SPEC이 추가한 redact + retention + rotation 때문에 8배 증가 — 설계 결정으로 반영.

---

## 8. 향후 SPEC 연계

본 SPEC 완료 후 다음 후속 작업이 가능:

1. **SPEC-GENIE-COMPRESSOR-001**: `Trajectory` 구조체를 입력으로 받아 Protected head/tail + middle LLM summary.
2. **SPEC-GENIE-INSIGHTS-001**: `~/.genie/trajectories/**/*.jsonl`을 스캔하여 overview/models/tools/activity 집계.
3. **SPEC-GENIE-MEMORY-001**: `SessionID` 공유 키로 Trajectory를 SQLite에 인덱싱.
4. **SPEC-GENIE-ERROR-CLASS-001**: `Metadata.FailureReason`에서 역산하여 14 FailoverReason으로 분류.
5. **SPEC-GENIE-LORA-001**(Phase 6): ShareGPT JSON-L은 HuggingFace SFTTrainer 표준 포맷 → 변환 없이 훈련 입력 사용 가능.

---

## 9. hermes-learning.md §12 Hermes SPEC-REFLECT 연계

```
Layer 1: Trajectory 수집 → internal/learning/trajectory         ← 본 SPEC
Layer 2: 압축              → internal/learning/compressor        ← COMPRESSOR-001
Layer 3: Insights 추출     → internal/evolve/{reflect, safety}   ← INSIGHTS-001 + REFLECT-001
Layer 4: Memory 저장       → internal/memory/{provider, manager} ← MEMORY-001
Layer 5: Skill/Prompt 자동 진화 → internal/skill/*              ← Phase 5 REFLECT-001
```

본 SPEC은 **Layer 1의 단일 책임**을 수행한다. 후속 Layer는 본 SPEC이 생성한 `.jsonl` 파일 또는 in-process `Trajectory` 구조체를 소비한다.

---

**End of Research**
