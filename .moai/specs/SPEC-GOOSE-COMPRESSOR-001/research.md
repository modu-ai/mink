# SPEC-GOOSE-COMPRESSOR-001 — Research & Porting Analysis

> **목적**: Hermes `trajectory_compressor.py` 1517 LoC의 알고리즘을 Go로 이식할 때의 결정점, 알고리즘 재사용 범위, Go 이디엄 재작성 영역을 정리한다. `.moai/project/research/hermes-learning.md` §3 원문을 본 SPEC 요구사항과 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/learning/compressor/` 패키지 9 파일.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/
hermes-agent-main/trajectory_compressor.py   # 1517 LoC 원본
hermes-agent-main/agent/trajectory.py        # Trajectory 구조 정의 (TRAJECTORY-001 소비)
```

- `internal/learning/compressor/` → **전부 부재**. Phase 4에서 신규 작성.
- 선행 SPEC-GOOSE-TRAJECTORY-001이 생성할 `Trajectory` / `TrajectoryEntry` 타입을 import 전제.
- SPEC-GOOSE-ROUTER-001이 제공할 저렴한 요약 모델(Gemini 3 Flash 등) 선택 로직은 `Summarizer` 구현체 내부.

---

## 2. hermes-learning.md §3 원문 → SPEC 요구사항 매핑

hermes-learning.md §3 알고리즘 원문:

```python
def compress_trajectory(trajectory) -> (trajectory, metrics):
    """첫 N턴 + 마지막 M턴 보호, 중간 압축"""
    # 1. 토큰 계산
    turn_tokens = [count_tokens(t) for t in trajectory]
    total = sum(turn_tokens)
    
    # 2. 목표 미만이면 스킵
    if total <= TARGET_MAX_TOKENS (15_250):
        metrics.skipped_under_target = True
        return trajectory, metrics
    
    # 3. 보호된 인덱스 (첫 system/human/gpt/tool, 마지막 4턴)
    protected = find_protected_indices(trajectory)
    compress_start, compress_end = find_compressible_region(protected)
    
    # 4. 압축량 계산
    tokens_to_save = total - TARGET_MAX_TOKENS
    target_compress = tokens_to_save + SUMMARY_TARGET_TOKENS (750)
    
    # 5. 누적해서 충분한 토큰 모일 때까지 턴 수집
    accumulated = 0
    compress_until = compress_start
    for i in range(compress_start, compress_end):
        accumulated += turn_tokens[i]
        compress_until = i + 1
        if accumulated >= target_compress:
            break
    
    # 6. 요약 생성 (Gemini 3 Flash, async)
    content = extract_turn_content(trajectory, compress_start, compress_until)
    summary = generate_summary_async(content, model="google/gemini-3-flash-preview")
    
    # 7. 재구성
    compressed = []
    compressed.extend(trajectory[:compress_start])  # head
    compressed.append({"from": "human", "value": summary})
    compressed.extend(trajectory[compress_until:])  # tail
    return compressed, metrics
```

매핑 표:

| Hermes 단계 | 본 SPEC REQ/AC | Go 매핑 |
|---|---|---|
| 1. 토큰 계산 | REQ-004, §6.3 L3-4 | `Tokenizer` 인터페이스 |
| 2. 목표 미만 스킵 | REQ-005, AC-001 | `SkippedUnderTarget=true` |
| 3. 보호 인덱스 | REQ-003, AC-003/004 | `findProtectedIndices(t, tail)` |
| 4. 압축량 계산 | REQ-006 (a)(b) | `tokensToSave`, `targetCompress` |
| 5. 누적 수집 | REQ-006 (c) | for-loop with break |
| 6. 요약 생성 | REQ-006 (d), REQ-007 | `Summarizer.Summarize` + retry |
| 7. 재구성 | REQ-013, AC-002 | 3-way concat (head + summary + tail) |

hermes-learning.md §3 특성:
- **병렬 처리**: `max_concurrent_requests: 50` (asyncio semaphore) → Go `chan struct{}` semaphore 패턴 (AC-008)
- **Tokenizer**: `moonshotai/Kimi-K2-Thinking` (trust_remote_code=True) → Go는 `tiktoken-go` 또는 `SimpleTokenizer` 근사
- **요약 모델**: Google Gemini 3 Flash (temp 0.3) → `Summarizer` 구현체 내부 결정
- **재시도**: max_retries=3, retry_delay=2s (jittered) → REQ-007/AC-005 매핑
- **타임아웃**: 300s/trajectory → REQ-010/AC-007 매핑

---

## 3. Python → Go 이식 결정

### 3.1 asyncio → goroutine 매핑

| Python 이디엄 | Go 이디엄 | 근거 |
|---|---|---|
| `async def summarize()` + `await` | `func Summarize(ctx context.Context, ...)` 동기 | Go 이디엄: context로 cancellation, 블로킹 OK |
| `asyncio.Semaphore(50)` | `chan struct{}` capacity 50 | Go 표준 패턴 |
| `await asyncio.gather(*tasks)` | `sync.WaitGroup` + result channel | `BatchResult` slice로 인덱스 보존 |
| `asyncio.wait_for(coro, timeout=300)` | `context.WithTimeout(ctx, 300s)` | cancellation propagation 자동 |
| `try/except` | `if err != nil` + `errors.Is/As` | sentinel errors (`ErrTransient` / `ErrPermanent`) |
| `random.uniform(0.5, 1.5)` | `rand.Float64() + 0.5` | math/rand 충분 (보안 무관) |

### 3.2 Tokenizer 선택

| 후보 | 장점 | 단점 | 결정 |
|---|---|---|---|
| `tiktoken-go` | OpenAI 호환, BPE 정확 | 외부 의존성, Go 1.22+ 필요 | **주입 가능**(프로덕션 권장) |
| Kimi-K2 Python binding | Hermes와 동일 | Python 의존성, Go 네이티브 아님 | 거부 |
| `SimpleTokenizer` (단어 × 1.3 + 특수문자) | 외부 의존성 0 | 정확도 ±20% | **기본 제공** (개발/테스트) |
| HuggingFace `tokenizers` Rust binding | 정확, 빠름 | CGO 부담 | Phase 6 LoRA에서 재검토 |

결정: **`Tokenizer` 인터페이스로 분리**, 기본 `SimpleTokenizer`, 프로덕션 tiktoken-go 주입 권고.

### 3.3 Summarizer 인터페이스 분리

Hermes는 Summarizer 로직이 `trajectory_compressor.py` 내부에 하드코딩됐다. 본 SPEC은 **인터페이스로 분리**:

근거:
- **ADAPTER-001**이 실제 LLM 호출 담당(Anthropic/OpenAI/Gemini).
- **ROUTER-001**이 모델 선택 로직 담당(비용/지연 기반).
- Compressor는 "요약을 어떻게 얻는가"와 무관하게 알고리즘만 책임.
- 테스트에서 stub Summarizer로 격리(AC-005~011 모두 stub).

### 3.4 재시도 백오프 라이브러리

| 후보 | 결정 |
|---|---|
| `github.com/cenkalti/backoff/v4` | 기능 풍부하지만 외부 의존성 |
| `github.com/avast/retry-go` | 간단하지만 jitter 직접 구현 필요 |
| **직접 구현** (math/rand + time.Sleep) | **채택** — 20 LoC로 충분, 외부 의존성 0 |

---

## 4. Go 라이브러리 결정

| 용도 | 채택 | 대안 | 근거 |
|---|---|---|---|
| Semaphore | `chan struct{}` 직접 | `golang.org/x/sync/semaphore` | 표준 이디엄, acquire/release 2줄 |
| Context cancellation | 표준 `context` | — | Go 이디엄 |
| 테스트 시계 | `github.com/jonboulle/clockwork` | `benbjohnson/clock` | TRAJECTORY-001과 동일 선택 |
| 토큰 카운팅(기본) | 직접 구현 | — | 근사값 충분 |
| 토큰 카운팅(프로덕션) | `github.com/tiktoken-go/tokenizer` | Python subprocess | CGO 불필요, 순 Go |
| JSON 템플릿 | 표준 `text/template` | `jinja-go` | prompt 렌더링 단순 |

---

## 5. 알고리즘 깊이 분석

### 5.1 보호 인덱스의 Edge Case

| 상황 | `findProtectedIndices` 동작 |
|---|---|
| 궤적이 5턴 미만(tail protected overlap) | Head ∩ Tail 허용, 반환 set에 중복 제거 |
| Role이 3개 미만(예: tool 없음) | 있는 role만 보호, 없는 role은 skip |
| Tail 4턴이 head 4 role 포함 | 동일 index 한 번만 등재 |
| 전부 같은 role (10개 system) | head 1개 + tail 4개 = 5개만 보호 |

### 5.2 `findCompressibleRegion` 결정 규칙

```
protected = {0, 1, 2, 4, 17, 18, 19}  # 예시
totalTurns = 20

# compressStart = 최소 unprotected index
# compressEnd   = 최대 unprotected index + 1

compressStart = 3   # 2 다음 unprotected
compressEnd   = 17  # 17 직전 unprotected
region = [3, 17) = 14 turns
```

Edge case: protected가 연속적(예: {0, 1, 2, 3}) + tail 4 → compressible region == 전체 중간.

### 5.3 Compression ratio 계산

```go
metrics.CompressionRatio = float64(metrics.CompressedTokens) / float64(metrics.OriginalTokens)
```

- 1.0 = 압축 없음(스킵)
- 0.5 = 절반 압축 = 이상적
- 0.3 = aggressive 압축 = 정보 손실 주의
- 1.0+ = 압축 실패 (StillOverLimit)

INSIGHTS-001에서 ratio 분포 히스토그램으로 품질 추적.

---

## 6. 테스트 전략

### 6.1 Stub 설계

```go
// 테스트용 Summarizer stub
type stubSummarizer struct {
    responses []string      // FIFO 응답
    errors    []error       // 에러 주입
    callLog   []callRecord  // 호출 기록
}

func (s *stubSummarizer) Summarize(ctx context.Context, turns []TrajectoryEntry, maxTokens int) (string, error) {
    s.callLog = append(s.callLog, callRecord{turns, time.Now()})
    if len(s.errors) > 0 {
        err := s.errors[0]; s.errors = s.errors[1:]
        if err != nil { return "", err }
    }
    resp := s.responses[0]; s.responses = s.responses[1:]
    return resp, nil
}

// 테스트용 Tokenizer stub
type stubTokenizer struct{ counts map[string]int }  // value → token count
```

### 6.2 Timing 검증

AC-005(jittered backoff)는 실제 `time.Sleep`을 사용하면 테스트가 느려진다. `clockwork.FakeClock`으로 가속:

```go
clock := clockwork.NewFakeClock()
compressor := New(cfg, sum, tok, log, withClock(clock))

go compressor.Compress(ctx, t)
clock.Advance(3 * time.Second)   // 1차 재시도
clock.Advance(6 * time.Second)   // 2차 재시도
```

### 6.3 병렬 테스트

AC-008(50 병렬) 검증:

```go
trajs := make([]*Trajectory, 200)
for i := range trajs { trajs[i] = fixture() }

start := time.Now()
results := compressor.CompressBatch(ctx, trajs)
elapsed := time.Since(start)

assert.Len(t, results, 200)
assert.InDelta(t, 400*time.Millisecond, elapsed, 100*time.Millisecond)
// 순차라면 200 * 100ms = 20s, 50 병렬이면 ~400ms
```

### 6.4 CONTEXT-001 어댑터 테스트

AC-013은 실제 CONTEXT-001 구현이 없어도 검증 가능 — 인터페이스 signature matching만 컴파일 타임 체크:

```go
var _ context.Compactor = (*CompactorAdapter)(nil)
```

동작 검증은 CONTEXT-001 GREEN 단계 이후 통합 테스트.

---

## 7. Hermes 재사용 평가

| Hermes 구성요소 | 재사용 가능성 | 재작성 필요 이유 |
|---|---|---|
| 알고리즘 본체(의사코드) | **90% 재사용** | 로직 그대로, 언어 차이만 |
| 보호 인덱스 규칙 | **100% 재사용** | 정책 그대로 |
| 상수(`TARGET_MAX_TOKENS=15250`, `SUMMARY_TARGET=750`) | **100% 재사용** | Hermes 프로덕션 검증값 |
| Semaphore 패턴(max 50) | **90% 재사용** | Go `chan` 치환 |
| 재시도/백오프 | **80% 재사용** | jitter formula 동일, time.Sleep → clock.Advance(테스트) |
| `TrajectoryMetrics` 구조 | **100% 재사용** | 필드명 camelCase ↔ snake_case 변환만 |
| asyncio 이벤트 | **0%** | goroutine + channel 재작성 |
| Kimi tokenizer | **0%** | tiktoken-go 또는 근사 |
| Summarizer prompt | **70% 재사용** | 본 SPEC이 템플릿화(Hermes는 하드코딩) |
| Gemini 3 Flash 하드코딩 | **0%** | ROUTER-001로 위임 |

**추정 Go LoC** (hermes-learning.md §10 기준 compressor 800 LoC):
- compactor.go: 180
- config.go: 60
- protected.go: 90
- summarizer.go: 120 (retry wrapper 포함)
- tokenizer.go: 80 (SimpleTokenizer 포함)
- metrics.go: 60
- batch.go: 100
- adapter.go: 70
- 테스트: 700+
- **합계**: ~760 production + 700 test ≈ 1,460 LoC

---

## 8. Prompt 엔지니어링 결정

### 8.1 기본 템플릿 (§6.4 재확인)

원본 Hermes는 프롬프트를 소스에 하드코딩했다. 본 SPEC은 **text/template** 기반으로 변수화:

```go
defaultTmpl := `You are summarizing a middle section of an AI agent's ... (§6.4)`

tmpl, _ := template.New("summary").Parse(defaultTmpl)
var buf bytes.Buffer
tmpl.Execute(&buf, map[string]any{
    "Turns":        middleSlice,
    "TargetTokens": cfg.SummaryTargetTokens,
    "ModelName":    t.Model,
})
```

### 8.2 Prompt Override 시나리오

| 시나리오 | 커스터마이징 |
|---|---|
| 한국어 요약 강제 | `.agency/context/brand-voice.ko.md` 포함 |
| 코드 블록 보존 | "preserve all ```code``` blocks verbatim" 추가 |
| 민감 도메인(의료/금융) | "never summarize diagnostic codes" 추가 |

### 8.3 Temperature 결정

Hermes는 0.3(결정적 요약). 본 SPEC은 `Summarizer` 구현체 책임으로 위임 — Compressor는 그냥 문자열 응답 수용.

---

## 9. 성능 예상

| 시나리오 | 예상 처리량 |
|---|---|
| 단일 궤적 20K tokens | ~1-3s (Gemini 3 Flash, p50) |
| 배치 200 궤적, 50 parallelism | ~4-12s |
| 배치 1000 궤적 | ~20-60s |

병목 분석:
- LLM 호출 (I/O bound) → 병렬화 효과 극대
- Tokenizer(SimpleTokenizer) → negligible
- JSON marshaling → negligible

---

## 10. hermes-learning.md §12 Layer 매핑

```
Layer 1: Trajectory 수집      ← TRAJECTORY-001
Layer 2: 압축                 ← 본 SPEC
Layer 3: Insights 추출        ← INSIGHTS-001 (metrics 소비자)
Layer 4: Memory 저장          ← MEMORY-001
Layer 5: Skill/Prompt 자동 진화 ← Phase 5 REFLECT-001
```

본 SPEC의 핵심 출력 `TrajectoryMetrics`는 Layer 3의 주요 집계 대상이다. 필드 이름과 snake_case/PascalCase 매핑 일치성이 INSIGHTS-001과의 통합 테스트 핵심.

---

**End of Research**
