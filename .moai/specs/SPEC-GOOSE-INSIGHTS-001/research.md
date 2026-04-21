# SPEC-GOOSE-INSIGHTS-001 — Research & Porting Analysis

> **목적**: Hermes `insights.py` 34KB의 다차원 집계 + 4-category 질적 분류 설계를 Go로 이식할 때의 결정점, 신뢰도 공식 검증, 테이블 render 구현 전략을 정리한다.
> **작성일**: 2026-04-21
> **범위**: `internal/learning/insights/` 패키지 12 파일.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/hermes-agent-main/agent/
insights.py    # 34KB, InsightsEngine 본체
```

- `internal/learning/insights/` → **전부 부재**. Phase 4 마지막 SPEC으로 신규 작성.
- 선행 TRAJECTORY-001의 `.jsonl` 파일 포맷, ERROR-CLASS-001의 FailoverReason 문자열, MEMORY-001의 `MemoryManager` 인터페이스 전제.
- COMPRESSOR-001의 `Summarizer` 인터페이스는 선택적 의존성(UseLLMSummary=true인 경우만).

---

## 2. hermes-learning.md §4 원문 → SPEC 매핑

hermes-learning.md §4 `InsightsEngine` 스키마:

```python
overview = {
    "total_sessions": int,
    "total_tokens": int,
    "estimated_cost": float,
    "total_hours": float,
    "avg_session_duration": float,
}

models = [{
    "model": str,
    "sessions": int,
    "input_tokens": int,
    "output_tokens": int,
    "cache_read_tokens": int,
    "cache_write_tokens": int,
    "total_tokens": int,
    "tool_calls": int,
    "cost": float,
    "has_pricing": bool,
}]

tools = [{"tool": str, "count": int, "percentage": float}]

activity = {
    "by_day": [{"day": str, "count": int}, ...],
    "by_hour": [{"hour": int, "count": int}, ...],
    "busiest_day": {"day": str, "count": int},
    "busiest_hour": {"hour": int, "count": int},
    "active_days": int,
    "max_streak": int,
}
```

매핑:

| Hermes 필드 | 본 SPEC 구조체 | REQ/AC |
|---|---|---|
| `overview.total_sessions` | `Overview.TotalSessions` | AC-INSIGHTS-001 |
| `overview.total_tokens` | `Overview.TotalTokens` | AC-INSIGHTS-001 |
| `overview.estimated_cost` | `Overview.EstimatedCost` | AC-INSIGHTS-007 |
| `overview.total_hours` | `Overview.TotalHours` | AC-INSIGHTS-001 |
| `overview.avg_session_duration` | `Overview.AvgSessionDuration` | AC-INSIGHTS-001 |
| `models[]` | `[]ModelStat` | AC-INSIGHTS-002 |
| `tools[]` | `[]ToolStat` | AC-INSIGHTS-003 |
| `activity.by_day` | `ActivityPattern.ByDay []DayBucket` | AC-INSIGHTS-004 |
| `activity.by_hour` | `ActivityPattern.ByHour []HourBucket` | AC-INSIGHTS-005 |
| `activity.busiest_day` | `ActivityPattern.BusiestDay` | AC-INSIGHTS-004 |
| `activity.busiest_hour` | `ActivityPattern.BusiestHour` | AC-INSIGHTS-005 |
| `activity.active_days` | `ActivityPattern.ActiveDays` | AC-INSIGHTS-006 |
| `activity.max_streak` | `ActivityPattern.MaxStreak` | AC-INSIGHTS-006 |

**신규 추가** (Hermes에 없음):
- 4 `InsightCategory` enum (Pattern / Preference / Error / Opportunity) + `Insight` 구조체
- `Confidence` 계산 공식 (§6.5)
- `Period` 타입 (시간 범위 명시)
- `Empty` 플래그 (빈 trajectory 디렉토리 안전 처리)
- `SchemaVersion` (JSON export 버전 관리)
- `Evidence[].Snippet` 50자 cap (PII 안전)

---

## 3. Python → Go 이식 결정

### 3.1 dict → struct

Hermes Python의 dict-of-dict → Go의 struct 계층. 이유:
- 컴파일 타임 field 검증
- JSON export 스키마 자동 (json tags)
- IDE 자동완성 + godoc

### 3.2 asyncio → 동기 (batch)

Hermes는 asyncio 기반. 본 SPEC은 batch `Extract(ctx, period)` 단일 호출:
- Insights는 **사용자 요청 시점** 계산 → 실시간성 불필요
- Go 동기 + goroutine semaphore로 스캐너 병렬화 가능하지만, 초기는 단일 pass.

### 3.3 Streaming Scanner

Hermes는 `json.load()`로 전체 파일 로드(약 1GB OOM 위험). Go 이식 시:

```go
// bufio.Scanner 기본 64KB 제한 → Reader.ReadBytes('\n') 사용
r := bufio.NewReader(f)
for {
    line, err := r.ReadBytes('\n')
    if err == io.EOF { break }
    if err != nil { return err }
    if len(line) > 10*1024*1024 {      // 10MB 라인 skip
        logger.Warn("line too large, skipped")
        continue
    }
    var t Trajectory
    if err := json.Unmarshal(line, &t); err != nil {
        logger.Warn("malformed JSON line")
        continue
    }
    out <- &t
}
```

### 3.4 Weekday 0=Sun → 0=Mon 변환

Go `time.Weekday`:
```
Sunday    = 0
Monday    = 1
...
Saturday  = 6
```

Hermes는 "Mon-Sun" 순서 (Mon=0). 변환:
```go
dayIdx := (int(weekday) + 6) % 7
// Sunday(0) → 6 (Sun)
// Monday(1) → 0 (Mon)
// Saturday(6) → 5 (Sat)
```

AC-INSIGHTS-004/006 검증.

### 3.5 MaxStreak 알고리즘

Hermes는 (의사코드 기반 추정) `sorted(days)` 후 연속 쌍 비교. Go 이식:

```go
func computeMaxStreak(daySet map[string]struct{}) int {
    if len(daySet) == 0 { return 0 }
    days := make([]time.Time, 0, len(daySet))
    for s := range daySet {
        d, _ := time.Parse("2006-01-02", s)
        days = append(days, d)
    }
    sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
    maxS, cur := 1, 1
    for i := 1; i < len(days); i++ {
        gap := days[i].Sub(days[i-1])
        if gap == 24*time.Hour {
            cur++
            if cur > maxS { maxS = cur }
        } else {
            cur = 1
        }
    }
    return maxS
}
```

**Edge case**:
- DST(Daylight Saving Time) 전환 일자는 23h 또는 25h → gap 계산 오류 가능. UTC 고정으로 회피(§6.4).
- 윤초(leap second) → 초 단위 정밀 비교 안 하므로 영향 없음.

---

## 4. 4-Category 분석기 설계

### 4.1 Pattern Detector

**Heuristic 규칙**:
```
1. Prompt prefix (첫 50자) 빈도 집계
2. 빈도 >= 3인 prefix를 후보로
3. (Prefix, Weekday, HourBucket) 3-tuple로 그룹화
4. 같은 3-tuple이 >= 3회 → Pattern {title: "<prefix> on <day> <hour>"}
```

**예시**:
- Prefix "코드 리뷰해줘" × 4회, 요일=Mon, hour=14 → "Code review Monday 14:00" (conf=0.85)

### 4.2 Preference Detector

**명시적 시그널**(사용자 메시지):
- `/(짧게|간결|concise|brief)/i` → shorter_responses
- `/(자세히|길게|detailed|verbose)/i` → longer_responses
- `/(이모지|emoji)/i` → emoji preference
- `/(반말|casual)/i` vs `/(존댓말|formal)/i`

**암묵적 시그널**(응답 후 행동):
- Positive after (short response): 긍정 표현 → short 선호도 강화
- Negative after (long response): 부정 표현 → long 기피

**임계치**: 3회 이상 관찰 시 Preference로 승격.

### 4.3 Error Detector

**입력**: `~/.goose/trajectories/failed/*.jsonl`의 `TrajectoryMetadata.FailureReason`

**집계**:
```go
reasonFreq := map[string]int{}
reasonSessions := map[string][]string{}   // reason → session_ids
for trajectory in failedFiles:
    reasonFreq[t.Metadata.FailureReason]++
    reasonSessions[...] = append(..., t.SessionID)
```

**Top 5**로 Insight 생성. Confidence는 `N/(N+1)` (단순).

### 4.4 Opportunity Detector

**규칙**:
- 노출된 tool 중 호출 0회인 것(미사용) → "Unused feature" insight
- Prompt 패턴이 특정 skill과 매칭되지만 skill 비활성 → "Consider enabling {skill}"
- Memory fact이 stale (> 30일 update 없음) → "Stale memory fact"

**MemoryManager 통합** (REQ-INSIGHTS-019): 주입된 경우 `Prefetch("opportunity")` 호출하여 메모리 기반 힌트 추가.

---

## 5. 신뢰도 공식 검증

### 5.1 Bayesian Intuition

Beta 분포의 posterior:
```
confidence = α / (α + β)
where α = observations (positive), β = noise (variance proxy)
```

본 SPEC 공식:
```
confidence(N, σ², p) = N / (N + σ² * p)
```

$N$이 크면 → 1에 수렴. $\sigma^2$가 크면 → 낮아짐.

### 5.2 검증 테이블

| N | σ² | penalty p | confidence | REFLECT-001 tier |
|--:|---:|---:|---:|---|
| 1 | 0.0 | 1 | 1.00 | Observation |
| 1 | 1.0 | 1 | 0.50 | Observation |
| 3 | 0.1 | 1 | 0.97 | Heuristic |
| 3 | 1.0 | 1 | 0.75 | Heuristic |
| 5 | 0.5 | 1 | 0.91 | Rule (≥ 0.80 요구) |
| 5 | 2.0 | 1 | 0.71 | Observation (0.80 미달) |
| 10 | 0.5 | 1 | 0.95 | High-Confidence (0.95 요구) |
| 10 | 2.0 | 1 | 0.83 | Rule (미달) |

REFLECT-001의 승격 기준(3회 / 5회+0.80 / 10회+0.95)과 정렬됨을 확인.

### 5.3 분산 추정

Preference 예시:
- 사용자가 "짧게" 4회 + "길게" 1회 → 분산 = (4 × (1 - 0.8)² + 1 × (0 - 0.8)²) / 5 = 0.16
- confidence(5, 0.16, 1) = 5 / 5.16 = 0.97 → Rule 승격

사용자가 "짧게" 3회 + "길게" 3회 → 분산 = 0.25
- confidence(6, 0.25, 1) = 6 / 6.25 = 0.96 → High-Confidence지만 **모호**

∴ 단순 observation count만 쓰는 Hermes 대비 **분산 penalty가 필수**.

---

## 6. Go 라이브러리 결정

| 용도 | 채택 | 대안 | 근거 |
|---|---|---|---|
| JSON 파싱 | 표준 `encoding/json` | `tidwall/gjson` | 본 SPEC은 전체 구조 필요, path 접근 아님 |
| Buffered I/O | 표준 `bufio` | — | `ReadBytes('\n')` 표준 충분 |
| 정렬 | 표준 `sort.Slice` | `slices.SortFunc` (1.21+) | 기존 SPEC과 일관성 |
| 시간 | 표준 `time` | — | UTC 고정 |
| Terminal colors (향후) | `fatih/color` | — | render.go에서 선택적 |
| Box-drawing | Unicode 문자 직접 | `olekukonko/tablewriter` | 경량 — 직접 구현 100 LoC |
| 테스트 | `testify` + `t.TempDir()` | — | 전 레포 일관 |

---

## 7. 테스트 전략

### 7.1 Fixture 생성기

```go
func newTrajectory(sessionID, model string, tokens int, ts time.Time) *trajectory.Trajectory {
    return &trajectory.Trajectory{
        SessionID: sessionID,
        Model: model,
        Timestamp: ts,
        Completed: true,
        Conversations: []trajectory.TrajectoryEntry{
            {From: "system", Value: "You are Goose."},
            {From: "human", Value: fmt.Sprintf("prompt_%s", sessionID)},
            {From: "gpt", Value: fmt.Sprintf("response_%s", sessionID)},
        },
        Metadata: trajectory.TrajectoryMetadata{
            TurnCount: 2, TokensInput: tokens/2, TokensOutput: tokens/2,
        },
    }
}

func writeTrajectoryFile(t *testing.T, dir string, bucket string, date time.Time, trajs []*trajectory.Trajectory) string {
    // 테스트용 .jsonl 생성
}
```

### 7.2 테이블 테스트

대부분의 AC는 data-driven:

```go
func TestComputeActivity(t *testing.T) {
    cases := []struct {
        name    string
        timestamps []time.Time
        wantMax int
        wantActive int
    }{
        {"3 days consecutive", [d1, d2, d3], 3, 3},
        {"gap breaks streak",  [d1, d2, d4, d5, d6], 3, 5},
        {"single day",         [d1, d1, d1], 1, 1},
        {"empty",              []time.Time{}, 0, 0},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := computeActivityFromTimestamps(tc.timestamps)
            assert.Equal(t, tc.wantMax, got.MaxStreak)
            assert.Equal(t, tc.wantActive, got.ActiveDays)
        })
    }
}
```

### 7.3 Property-based test (선택)

Activity.ByDay 합 = 총 세션 수 invariant:

```go
func TestActivity_SumInvariant(t *testing.T) {
    for i := 0; i < 100; i++ {
        sessions := randomSessions(rand.Intn(100))
        activity := computeActivity(sessions)
        sum := 0
        for _, d := range activity.ByDay { sum += d.Count }
        assert.Equal(t, len(sessions), sum)
    }
}
```

### 7.4 큰 파일 테스트

AC-INSIGHTS-011 (150MB streaming):
- t.Skip if short mode
- 임시 파일 150MB 생성 (repeat 50,000 lines of 3KB each)
- runtime.ReadMemStats 전후 비교, 증가량 ≤ 100MB 확인

---

## 8. Hermes 재사용 평가

| Hermes 구성요소 | 재사용 가능성 | 재작성 필요 이유 |
|---|---|---|
| InsightsEngine 스키마 | **95% 재사용** | snake_case → PascalCase |
| overview/models/tools/activity 집계 로직 | **85% 재사용** | Python dict → Go struct |
| Pricing table 구조 | **100% 재사용** | YAML 포맷 동일 |
| MaxStreak 알고리즘 | **90% 재사용** | UTC 고정 추가 |
| Weekday 인덱싱 | **70% 재사용** | Go time.Weekday 0=Sun 보정 |
| Terminal UI 테이블 | **50% 재사용** | Python `rich` → Go 직접 구현 |
| asyncio file scanning | **0% 재사용** | Go bufio.Reader 재작성 |
| Insight 분류 (4-cat) | **신규 설계** | Hermes는 4-cat 명시 없음 |
| Confidence 공식 | **신규 설계** | Hermes는 단순 count |
| JSON export | **신규 추가** | Hermes CLI UI only |

**추정 Go LoC** (hermes-learning.md §10: `learning/insights: 600 LoC`):
- engine.go: 150
- types.go: 100
- period.go: 40
- scanner.go: 120
- overview.go: 70
- models.go: 100 (pricing 포함)
- tools.go: 60
- activity.go: 110 (streak 포함)
- analyzer.go: 220 (4-cat)
- confidence.go: 50
- render.go: 250 (terminal table)
- pricing.go: 70
- 테스트: 800+
- **합계**: ~1,340 production + 800 test ≈ 2,140 LoC (Hermes 34KB ~= 1,000 LoC 추정 대비 34% 증가, 주된 이유: 4-cat 분석기 신규 + streaming scanner + render 재작성)

---

## 9. 통합 시나리오

### 9.1 CLI 통합 (CLI-001 향후)

```bash
$ goose insights --last 7
┌─ Overview ────────────────────────────┐
│ Period: 2026-04-14 ~ 2026-04-21       │
│ Total Sessions: 128 (124 ✓, 4 ✗)      │
│ ...                                   │
└───────────────────────────────────────┘
...

$ goose insights --last 30 --json > report.json
```

### 9.2 REFLECT-001 (Phase 5) 통합

```go
// REFLECT-001 (참고)
report, _ := insightsEngine.Extract(ctx, insights.Last(7))
for _, insight := range report.Insights[insights.CategoryPreference] {
    if insight.Confidence >= 0.80 {
        reflect.ProposePromotion(insight)   // → Observation → Heuristic → Rule
    }
}
```

### 9.3 주간 cron 자동 생성

```go
// 향후 scheduler (본 SPEC scope 외)
cron.Every("Monday 09:00", func() {
    report, _ := engine.Extract(ctx, insights.Last(7))
    notify.Send(user, report.RenderTable())
})
```

---

## 10. hermes-learning.md §12 Layer 매핑

```
Layer 1: Trajectory 수집      ← TRAJECTORY-001
Layer 2: 압축                 ← COMPRESSOR-001
Layer 3: Insights 추출        ← 본 SPEC
Layer 4: Memory 저장          ← MEMORY-001
Layer 5: Skill/Prompt 자동 진화 ← REFLECT-001 (Phase 5)
```

본 SPEC은 **Layer 3의 단일 책임**이며, 관찰 데이터(Layer 1+2) → 메모리(Layer 4) 및 진화(Layer 5)로 이어지는 파이프라인의 **분석 허브**다. Report는 사용자에게 직접 보이는 유일한 Phase 4 산출물이기도 하다.

---

## 11. 오픈 이슈

| # | 이슈 | 결정 시점 |
|---|---|---|
| O1 | Pricing YAML 업데이트 주기 | Phase 5 REFLECT-001과 함께 cron 도입 검토 |
| O2 | 다국어 narrative (한/영) | CLI-001에서 i18n 결정 |
| O3 | Weekly vs Monthly 기본 period | 사용자 feedback 수집 후 결정 |
| O4 | Anomaly Detection(Isolation Forest) 통합 시점 | Phase 5에서 재평가 |
| O5 | Incremental update (session 단위 증분) | v2.0 이후 별도 SPEC |

---

**End of Research**
