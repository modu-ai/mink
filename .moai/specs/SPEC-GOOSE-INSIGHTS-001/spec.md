---
id: SPEC-GOOSE-INSIGHTS-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P1
issue_number: null
phase: 4
size: 중(M)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-INSIGHTS-001 — 다차원 Insights 추출 (Pattern/Preference/Error/Opportunity)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-learning.md §4 + Hermes `insights.py` 34KB 기반) | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE **자기진화 파이프라인의 Layer 3**를 정의한다. TRAJECTORY-001이 기록한 `.jsonl` 궤적 파일 + MEMORY-001의 `facts` 테이블을 입력으로, **4개 다차원 집계(overview / models / tools / activity)**와 **4개 질적 분류(Pattern / Preference / Error / Opportunity)**를 생성한다. 각 분류 항목은 **신뢰도 점수**(관찰 횟수 + 분산 기반)를 달며, REFLECT-001(Phase 5)의 5단계 승격 파이프라인 입력이자 사용자 주간/월간 리포트의 데이터 소스로 사용된다.

본 SPEC이 통과한 시점에서:

- `InsightsEngine.Extract(ctx, period InsightsPeriod) (*Report, error)`가 지정 기간의 Trajectory + Memory를 스캔하여 완성된 `Report` 반환,
- 기본 `period`는 "지난 7일"이지만 임의 `[from, to]` 범위 지원,
- 4개 양적 지표(Overview, Models, Tools, Activity)는 deterministic 집계 (LLM 호출 없음),
- 4개 질적 분류(Pattern/Preference/Error/Opportunity)는 **heuristic 기반 (LLM 선택)** — 기본은 heuristic, `ClassifierOptions.UseLLMSummary=true` 시 Summarizer 호출로 요약,
- 터미널 UI 표 출력용 `Report.RenderTable()` 헬퍼 + JSON export 지원.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 4의 "사용자에게 보이는 산출물". Trajectory + Compressor + Memory는 데이터 처리 인프라, Insights는 **사람이 읽는 결과물**이다.
- `.moai/project/research/hermes-learning.md` §4의 `InsightsEngine` 스키마(overview/models/tools/activity 4-dim)가 이식 대상.
- REFLECT-001(Phase 5) 5단계 승격은 본 SPEC의 질적 분류를 **관찰(Observation) 입력**으로 사용한다. Insights 없이 REFLECT는 데이터 없이 동작 불가.
- `/goose insights` CLI 명령(CLI-001)의 백엔드. 주간 리포트 자동 생성 + 사용자 확인 워크플로우.
- 로드맵 v2.0 §4 Phase 4 #21.

### 2.2 상속 자산

- **Hermes Agent Python** (`./hermes-agent-main/agent/insights.py` 34KB): `InsightsEngine` 스키마 + overview/models/tools/activity 4차원 집계 + terminal UI 테이블 출력. 80% 재사용.
- **Claude Code TypeScript**: Insights 기능 없음.
- **Hermes 질적 분류**는 (§4 기준) 명시적 별도 코드 없음 → 본 SPEC이 **신규 설계**(4 category + confidence).

### 2.3 범위 경계

- **IN**: `InsightsEngine` + 4-dim 양적 집계 + 4-cat 질적 분류, `Report` / `Insight` / `InsightCategory` 구조체, 기간 지정, 신뢰도 계산, 터미널 테이블 렌더러, JSON export, 모델별/도구별/요일별/시간대별 히스토그램, busiest_day / busiest_hour / active_days / max_streak 메트릭.
- **OUT**: 실제 5단계 승격 의사결정(REFLECT-001), 사용자 승인 UI(CLI-001), 실시간 streaming insights(본 SPEC은 batch-only), 차트 이미지 생성(터미널 텍스트 only, 그래프는 별도 SPEC), 다른 사용자와의 비교(federated insights는 별도 SPEC), Identity Graph 연동 query(IDENTITY-001은 Phase 6).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/learning/insights/` 패키지: `InsightsEngine`, `Report`, `Insight`, `InsightCategory` enum, `InsightsPeriod` 구조체.
2. `internal/learning/insights/overview.go`: `Overview` 집계 (total_sessions / total_tokens / estimated_cost / total_hours / avg_session_duration).
3. `internal/learning/insights/models.go`: `ModelStat` 집계 (per-model sessions / input_tokens / output_tokens / cache_tokens / tool_calls / cost / has_pricing).
4. `internal/learning/insights/tools.go`: `ToolStat` 집계 (per-tool count / percentage).
5. `internal/learning/insights/activity.go`: `ActivityPattern` 집계 (by_day / by_hour / busiest_day / busiest_hour / active_days / max_streak).
6. `internal/learning/insights/analyzer.go`: 4-cat 질적 분류(Pattern / Preference / Error / Opportunity) heuristic 엔진.
7. `internal/learning/insights/confidence.go`: 신뢰도 계산 (관찰 횟수 N + 분산 σ² 기반 bayesian adjustment).
8. `internal/learning/insights/scanner.go`: `${GOOSE_HOME}/trajectories/**/*.jsonl` 스캔 (`TrajectoryReader`).
9. `internal/learning/insights/render.go`: terminal UI 테이블(ASCII) + JSON export.
10. Cost 계산: 모델별 가격표 (lookup 기반, `config.insights.pricing.yaml` 주입).
11. Period 지정: `Last(days int)`, `Between(from, to time.Time)`, `AllTime()`.

### 3.2 OUT OF SCOPE (명시적 제외)

- **5단계 승격 의사결정**: REFLECT-001.
- **사용자 승인 워크플로우**: REFLECT-001 + CLI-001.
- **실시간 streaming**: 본 SPEC은 `Extract()` 1회 호출 batch만. 이벤트 기반 incremental update는 향후 확장.
- **차트 이미지(PNG/SVG)**: 터미널 텍스트 only. 시각화 SPEC은 별도.
- **Federated / 다른 사용자 비교**: 로컬 single-user만.
- **Anomaly Detection**: Phase 4 범위 외 (learning-engine.md §1.2 (d) Isolation Forest는 Phase 5+).
- **Markov chain 행동 예측**: learning-engine.md §1.2 (b)는 별도 SPEC.
- **LoRA 훈련 데이터 준비**: 별도 SPEC.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-INSIGHTS-001 [Ubiquitous]** — The `InsightsEngine.Extract` method **shall** return a `*Report` whose `Period.From`, `Period.To` match the input period exactly (bounds preserved for reproducibility).

**REQ-INSIGHTS-002 [Ubiquitous]** — All quantitative dimensions (Overview, Models, Tools, Activity) **shall** be computed deterministically — given identical input trajectories, repeated calls **shall** produce identical output values (within float64 precision).

**REQ-INSIGHTS-003 [Ubiquitous]** — Every `Insight` in the qualitative categories **shall** carry a `Confidence` score in `[0, 1]` and an `Evidence` list referencing concrete trajectory `session_id`s.

**REQ-INSIGHTS-004 [Ubiquitous]** — The `TrajectoryReader` scanner **shall** never load a single trajectory file that exceeds 100MB into memory; such files **shall** be streamed line-by-line.

### 4.2 Event-Driven (이벤트 기반)

**REQ-INSIGHTS-005 [Event-Driven]** — **When** `Extract(period=Last(7))` is invoked, the engine **shall** scan `${GOOSE_HOME}/trajectories/{success,failed}/YYYY-MM-DD.jsonl` for dates in the range `[now-7d, now]` and ignore files outside range.

**REQ-INSIGHTS-006 [Event-Driven]** — **When** a trajectory file is malformed JSON on any line, the scanner **shall** log a zap warning with `{path, line_number}`, skip that line, and continue; one malformed line **shall not** abort the full extraction.

**REQ-INSIGHTS-007 [Event-Driven]** — **When** computing `Activity.MaxStreak`, the engine **shall** find the longest consecutive run of days with at least one session; days with zero sessions **shall** break the streak.

**REQ-INSIGHTS-008 [Event-Driven]** — **When** a session's `model` field references a model absent from `config.insights.pricing`, the corresponding `ModelStat.HasPricing` **shall** be `false` and `ModelStat.Cost` **shall** be 0 (not nil, not error).

**REQ-INSIGHTS-009 [Event-Driven]** — **When** the same tool is invoked N times across M sessions, `ToolStat.Count = N` and `ToolStat.Percentage = N / TotalToolCalls * 100` (rounded to 2 decimals).

**REQ-INSIGHTS-010 [Event-Driven]** — **When** a qualitative `Insight` is promoted (observed ≥ 3 times within period), `Confidence` **shall** increase monotonically with observation count using the formula `confidence = min(1.0, observations / (observations + σ² * penalty))` (§6.5).

### 4.3 State-Driven (상태 기반)

**REQ-INSIGHTS-011 [State-Driven]** — **While** `config.telemetry.trajectory.enabled == false` (TRAJECTORY-001 disabled), `Extract` **shall** return an empty `*Report{Period: input_period, Empty: true}` with `Overview.TotalSessions=0` and log a zap info message — **shall not** error.

**REQ-INSIGHTS-012 [State-Driven]** — **While** `period.From > period.To` (invalid range), `Extract` **shall** return `nil, ErrInvalidPeriod`.

### 4.4 Unwanted Behavior (방지)

**REQ-INSIGHTS-013 [Unwanted]** — The `Extract` method **shall not** invoke external LLM calls unless `ExtractOptions.UseLLMSummary == true`; by default all insights **shall** be derived from deterministic heuristics.

**REQ-INSIGHTS-014 [Unwanted]** — `ActivityPattern.ByDay` **shall not** contain more than 7 entries (Mon-Sun), and `ByHour` **shall not** contain more than 24 entries (0-23); boundary bugs that produce 8th day or 25th hour **shall** be caught by assertion.

**REQ-INSIGHTS-015 [Unwanted]** — The qualitative `Analyzer` **shall not** generate insights whose `Evidence` list is empty; insights without concrete evidence **shall** be suppressed.

**REQ-INSIGHTS-016 [Unwanted]** — `Cost` values **shall not** be rendered to the user in USD if `HasPricing == false`; instead render as `"N/A"` to avoid misleading zero-cost displays.

### 4.5 Optional (선택적)

**REQ-INSIGHTS-017 [Optional]** — **Where** `ExtractOptions.UseLLMSummary == true`, the engine **shall** invoke `Summarizer.Summarize` (COMPRESSOR-001의 인터페이스) once per category to produce human-readable narratives; narratives **shall** be stored in `Insight.Narrative` field.

**REQ-INSIGHTS-018 [Optional]** — **Where** `Report.RenderTable()` is invoked, the output **shall** include sections: `Overview`, `Top 10 Models by Tokens`, `Top 10 Tools by Frequency`, `Activity Heatmap (Day × Hour)`, `Top 5 Insights per Category` in that order, using Unicode box-drawing characters for tables.

**REQ-INSIGHTS-019 [Optional]** — **Where** `ExtractOptions.MemoryManager != nil`, the engine **shall** query `MemoryManager.Prefetch` with category keywords (e.g. "preference", "error_pattern") and include matched facts as additional evidence.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then.

**AC-INSIGHTS-001 — Overview 집계 결정론성**
- **Given** `t.TempDir()`에 5개 trajectory(총 sessions=5, 총 input_tokens=10,000, 총 output_tokens=3,000, 총 duration=300s)를 배치
- **When** `Extract(Last(30))` 두 번 호출
- **Then** 두 호출의 `Overview` 필드 전부 동일. `TotalSessions==5`, `TotalTokens==13_000`, `TotalHours==300/3600.0`

**AC-INSIGHTS-002 — Models 집계 + Top 10**
- **Given** 세션 12개가 모델 15종에 분포
- **When** `Extract`
- **Then** `Report.Models` 길이 15, 토큰 수 내림차순 정렬, 테이블 render 시 Top 10만 표시

**AC-INSIGHTS-003 — Tool count + percentage**
- **Given** tool A가 40회, tool B가 30회, tool C가 30회 호출 (총 100회)
- **When** `Extract`
- **Then** `Report.Tools[0]` = `{Tool:"A", Count:40, Percentage:40.00}`, 정렬 내림차순

**AC-INSIGHTS-004 — Activity by_day**
- **Given** trajectory들이 Mon:10, Tue:5, Wed:0, ..., Sun:8 분포
- **When** `Extract`
- **Then** `Report.Activity.ByDay` 길이 7, `{"Mon":10, "Tue":5, "Wed":0, ...}` 순서(월요일 시작), `BusiestDay.Day=="Mon"`

**AC-INSIGHTS-005 — Activity by_hour**
- **Given** 세션 시작 시각이 0시~23시 분포
- **When** `Extract`
- **Then** `Report.Activity.ByHour` 길이 24, `BusiestHour.Hour` 값 ∈ [0,23]

**AC-INSIGHTS-006 — MaxStreak**
- **Given** 활동 일자: 2026-04-15, 16, 17, 20, 21, 22, 23, 24 (첫 3일 연속, 후 5일 연속)
- **When** `Extract(Between("2026-04-15", "2026-04-25"))`
- **Then** `Activity.MaxStreak == 5`, `ActiveDays == 8`

**AC-INSIGHTS-007 — 모델 가격 미정**
- **Given** `config.insights.pricing`에 "claude-opus-4-7"만 있음, trajectory에 "claude-opus-4-7"과 "unknown-model"이 혼재
- **When** `Extract`
- **Then** `ModelStat["claude-opus-4-7"].HasPricing==true, Cost>0`, `ModelStat["unknown-model"].HasPricing==false, Cost==0.0`

**AC-INSIGHTS-008 — Period 범위 준수**
- **Given** trajectory 파일들: 2026-04-10, 15, 20, 25 (4개)
- **When** `Extract(Between("2026-04-14", "2026-04-22"))`
- **Then** 04-15와 04-20 파일만 스캔, 04-10과 04-25는 무시. `Overview.TotalSessions`가 04-15+04-20 파일의 합계와 일치

**AC-INSIGHTS-009 — 잘못된 period**
- **Given** `period.From = 2026-04-25`, `period.To = 2026-04-20` (역순)
- **When** `Extract(period)`
- **Then** 반환 `(nil, ErrInvalidPeriod)`

**AC-INSIGHTS-010 — 빈 trajectory**
- **Given** `config.telemetry.trajectory.enabled=false` 또는 trajectory 디렉토리 비어있음
- **When** `Extract(Last(7))`
- **Then** 반환 `*Report`, `err==nil`, `Overview.TotalSessions==0`, `Empty=true`

**AC-INSIGHTS-011 — 큰 파일 스트리밍**
- **Given** trajectory 파일 1개가 150MB, 총 50,000 라인
- **When** `Extract`
- **Then** 프로세스 RSS 증가량 ≤ 100MB(파일 전체 메모리 로드 안 함), 모든 50,000 세션 집계됨

**AC-INSIGHTS-012 — 잘못된 JSON 라인 skip**
- **Given** 파일의 중간 라인에 `{"conversations":[broken` (깨진 JSON)
- **When** `Extract`
- **Then** 전체 extraction 성공, 깨진 라인은 skip, zap warning 로그 1건 (`path`, `line_number`)

**AC-INSIGHTS-013 — 질적 분류 Pattern**
- **Given** 같은 사용자가 4번의 세션에서 "오후 2시경에 항상 코드 리뷰 요청" 패턴 (명확)
- **When** `Extract`
- **Then** `Report.Insights[Pattern]`에 적어도 1개 항목, `Evidence` 길이 4, `Confidence >= 0.7`

**AC-INSIGHTS-014 — Preference 감지 (응답 길이)**
- **Given** 5개 세션에서 사용자가 "더 짧게 답해줘"를 4번 요청
- **When** `Extract`
- **Then** `Report.Insights[Preference]`에 "prefers_shorter_responses" 관련 insight, `Confidence >= 0.7`

**AC-INSIGHTS-015 — Error 집계 (FailoverReason 빈도)**
- **Given** failed/*.jsonl에 `FailureReason: "rate_limit"` 10개, `"context_overflow"` 3개
- **When** `Extract`
- **Then** `Report.Insights[Error]`의 top 2가 `rate_limit`, `context_overflow`, 각각 observation count와 confidence 기록

**AC-INSIGHTS-016 — Opportunity 감지 (도구 미사용)**
- **Given** `MemoryManager`가 `memory_recall` tool 노출, 30일간 해당 tool 호출 0회
- **When** `Extract(Last(30), opts{MemoryManager: mm})`
- **Then** `Report.Insights[Opportunity]`에 "memory_recall tool never used" insight

**AC-INSIGHTS-017 — LLM summary off (기본)**
- **Given** `ExtractOptions.UseLLMSummary == false` (기본값)
- **When** `Extract`
- **Then** Summarizer mock 호출 0회, 모든 Insight의 `Narrative` 필드는 heuristic-generated (deterministic)

**AC-INSIGHTS-018 — Report.RenderTable 섹션**
- **Given** 비어있지 않은 Report
- **When** `report.RenderTable()`
- **Then** 반환 문자열에 `Overview`, `Top 10 Models`, `Top 10 Tools`, `Activity Heatmap`, `Top 5 Insights` 5 섹션 헤더 모두 존재 (ASCII box-drawing `┌─┬─┐` 포함)

**AC-INSIGHTS-019 — JSON export 스키마**
- **Given** Report
- **When** `json.Marshal(report)`
- **Then** 결과 JSON이 `{period, overview, models, tools, activity, insights}` 5 top-level 필드 포함, 각 필드가 스펙 문서화된 스키마와 매칭

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/
└── learning/
    └── insights/
        ├── engine.go               # InsightsEngine + Extract
        ├── engine_test.go
        ├── types.go                # Report, Overview, ModelStat, ToolStat, Activity, Insight
        ├── period.go               # InsightsPeriod (Last, Between, AllTime)
        ├── scanner.go              # TrajectoryReader (streaming .jsonl)
        ├── scanner_test.go
        ├── overview.go             # Overview 집계
        ├── models.go               # ModelStat 집계 + pricing lookup
        ├── tools.go                # ToolStat 집계
        ├── activity.go             # ActivityPattern 집계 + streak
        ├── analyzer.go             # 4-cat 질적 분류 (Pattern/Preference/Error/Opportunity)
        ├── analyzer_test.go
        ├── confidence.go           # 신뢰도 공식
        ├── render.go               # Terminal table + JSON
        └── pricing.go              # ModelPricing 구조체 + config 로드
```

### 6.2 핵심 타입

```go
// internal/learning/insights/types.go

type Report struct {
    Period    InsightsPeriod
    Empty     bool

    Overview  Overview
    Models    []ModelStat       // 토큰 내림차순
    Tools     []ToolStat        // count 내림차순
    Activity  ActivityPattern
    Insights  map[InsightCategory][]Insight   // 4-cat
    GeneratedAt time.Time
}

type Overview struct {
    TotalSessions       int
    TotalSuccessful     int
    TotalFailed         int
    TotalTokens         int
    EstimatedCost       float64    // USD, 미지정 모델 제외
    HasFullPricing      bool
    TotalHours          float64
    AvgSessionDuration  float64    // seconds
}

type ModelStat struct {
    Model             string
    Sessions          int
    InputTokens       int
    OutputTokens      int
    CacheReadTokens   int
    CacheWriteTokens  int
    TotalTokens       int
    ToolCalls         int
    Cost              float64
    HasPricing        bool
}

type ToolStat struct {
    Tool       string
    Count      int
    Percentage float64    // 0-100, 소수 2자리
    UsedIn     int         // distinct session 수
}

type ActivityPattern struct {
    ByDay       []DayBucket     // 길이 7 Mon-Sun
    ByHour      []HourBucket    // 길이 24 0-23
    BusiestDay  DayBucket
    BusiestHour HourBucket
    ActiveDays  int
    MaxStreak   int
}

type DayBucket struct {
    Day   string   // "Mon", "Tue", ...
    Count int
}

type HourBucket struct {
    Hour  int      // 0-23
    Count int
}

type InsightCategory int
const (
    CategoryPattern     InsightCategory = iota    // 반복 행동 패턴
    CategoryPreference                             // 명시적/암묵적 선호도
    CategoryError                                  // 실패 유형
    CategoryOpportunity                            // 미사용 기능 / 개선 여지
)

type Insight struct {
    Category     InsightCategory
    Title        string       // "Prefers shorter responses"
    Description  string
    Observations int          // 관찰 횟수
    Confidence   float64      // [0,1]
    Evidence     []EvidenceRef
    Narrative    string       // heuristic or LLM-generated
    CreatedAt    time.Time
}

type EvidenceRef struct {
    SessionID string
    Timestamp time.Time
    Snippet   string         // 50자 이내 (PII 우려로 50자 cap)
}


// internal/learning/insights/engine.go

type InsightsEngine struct {
    cfg       InsightsConfig
    scanner   *TrajectoryReader
    pricing   PricingTable
    memMgr    *memory.MemoryManager   // optional
    summarizer compressor.Summarizer  // optional, for UseLLMSummary
    logger    *zap.Logger
}

func New(cfg InsightsConfig, opts ...Option) *InsightsEngine

type ExtractOptions struct {
    UseLLMSummary  bool
    MemoryManager  *memory.MemoryManager
    ConfidenceMin  float64   // 기본 0.5
}

func (e *InsightsEngine) Extract(
    ctx context.Context,
    period InsightsPeriod,
    opts ...ExtractOption,
) (*Report, error)


// internal/learning/insights/period.go

type InsightsPeriod struct {
    From time.Time
    To   time.Time
}

func Last(days int) InsightsPeriod {
    now := time.Now().UTC()
    return InsightsPeriod{From: now.AddDate(0, 0, -days), To: now}
}

func Between(from, to time.Time) InsightsPeriod
func AllTime() InsightsPeriod


// internal/learning/insights/scanner.go

type TrajectoryReader struct {
    baseDir string
    logger  *zap.Logger
}

// ScanPeriod는 iterator 패턴. 개별 trajectory를 streaming으로 반환.
// 내부적으로 100MB 초과 파일도 안전하게 line-by-line scan.
func (r *TrajectoryReader) ScanPeriod(
    period InsightsPeriod,
    bucket string,        // "success" | "failed" | "" (both)
) <-chan *trajectory.Trajectory


// internal/learning/insights/confidence.go

// Confidence는 관찰 수 N과 분산 penalty를 결합.
// N=1 → low, N=10+ → ~1.0, 분산이 크면 penalty.
func CalculateConfidence(observations int, variance float64, penalty float64) float64 {
    if observations == 0 { return 0.0 }
    adjusted := float64(observations) / (float64(observations) + variance*penalty)
    if adjusted > 1.0 { return 1.0 }
    return adjusted
}
```

### 6.3 Analyzer Heuristic

4개 카테고리별 heuristic:

**Pattern (반복 행동)**:
- 같은 prompt prefix(첫 50자)가 ≥ 3회 → 잠재 패턴
- 같은 요일+시간대 조합에서 ≥ 3회 활동 → "매주 월 9시에 활동"

**Preference (선호도)**:
- 사용자가 "short" / "brief" / "concise" / "더 짧게" 메시지를 ≥ 3회 → shorter_responses
- 응답 후 "고마워/완벽/좋아" 같은 긍정 시그널이 특정 응답 길이에 집중 → optimal length
- 특정 도구 호출 후 negation("다시", "틀렸어") ≥ 3회 → tool 불만족

**Error (실패 유형)**:
- failed/*.jsonl의 `FailureReason` 빈도 집계
- Top 5 reason × 대응 session_id evidence

**Opportunity (개선 여지)**:
- Memory tool 노출됐으나 미사용
- 특정 skill이 도움이 될 수 있는 prompt 패턴 감지 (예: "코드 리뷰" 요청 많으나 `code_review` skill 활성 안 됨)

### 6.4 Activity 알고리즘

```go
func computeActivity(sessions []*trajectory.Trajectory) ActivityPattern {
    byDay := make([]DayBucket, 7)    // [Mon, Tue, ..., Sun]
    byHour := make([]HourBucket, 24) // [0, 1, ..., 23]
    daySet := map[string]struct{}{}  // for ActiveDays, MaxStreak

    for _, s := range sessions {
        weekday := s.Timestamp.UTC().Weekday()     // time.Weekday: Sunday=0, Monday=1, ..., Saturday=6
        dayIdx := (int(weekday) + 6) % 7           // remap to Mon=0, Sun=6
        byDay[dayIdx].Count++
        byHour[s.Timestamp.UTC().Hour()].Count++
        daySet[s.Timestamp.UTC().Format("2006-01-02")] = struct{}{}
    }

    // Fill day labels
    dayLabels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
    for i := range byDay { byDay[i].Day = dayLabels[i] }
    for i := range byHour { byHour[i].Hour = i }

    activeDays := len(daySet)
    maxStreak := computeMaxStreak(daySet)   // 정렬 후 연속 일자 카운트

    busyDay := maxDayBucket(byDay)
    busyHour := maxHourBucket(byHour)

    return ActivityPattern{
        ByDay: byDay, ByHour: byHour,
        BusiestDay: busyDay, BusiestHour: busyHour,
        ActiveDays: activeDays, MaxStreak: maxStreak,
    }
}

func computeMaxStreak(daySet map[string]struct{}) int {
    days := make([]time.Time, 0, len(daySet))
    for s := range daySet {
        d, _ := time.Parse("2006-01-02", s)
        days = append(days, d)
    }
    sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
    streak, max := 1, 1
    for i := 1; i < len(days); i++ {
        if days[i].Sub(days[i-1]) == 24*time.Hour {
            streak++
            if streak > max { max = streak }
        } else {
            streak = 1
        }
    }
    return max
}
```

### 6.5 신뢰도 공식

Bayesian-inspired:

```
confidence(N, σ², p) = N / (N + σ² * p)

where:
  N = 관찰 횟수
  σ² = 관찰값의 분산 (낮을수록 일관된 패턴)
  p = penalty factor (기본 1.0, tunable)

예:
  N=3, σ²=0.1, p=1   → 3 / (3 + 0.1) = 0.968
  N=3, σ²=1.0, p=1   → 3 / (3 + 1.0) = 0.750
  N=1, σ²=0.0, p=1   → 1 / (1 + 0)   = 1.0   (단 관찰값이 명확하면)
```

REFLECT-001의 5단계 승격 기준(3회 → heuristic, 5회 → rule, 10회 → high_conf)과 정렬.

### 6.6 Pricing 테이블

`config.insights.pricing.yaml`:

```yaml
pricing:
  "anthropic/claude-opus-4-7":
    input_per_1k:  0.015
    output_per_1k: 0.075
    cache_read_per_1k: 0.0015
    cache_write_per_1k: 0.01875
  "anthropic/claude-sonnet-4-5":
    input_per_1k:  0.003
    output_per_1k: 0.015
  "openai/gpt-4o":
    input_per_1k:  0.0025
    output_per_1k: 0.010
  "google/gemini-3-flash-preview":
    input_per_1k:  0.0001   # Hermes compressor 비용
    output_per_1k: 0.0004
  # ... 미지정 모델은 HasPricing=false
```

### 6.7 Terminal 테이블 Render

```
┌─ Overview ─────────────────────────────────┐
│ Period:          2026-04-14 ~ 2026-04-21   │
│ Total Sessions:  128 (124 success, 4 fail) │
│ Total Tokens:    2,341,589                  │
│ Estimated Cost:  $12.47 (partial pricing)   │
│ Total Hours:     18.3                       │
│ Avg Session:     8m 34s                     │
└─────────────────────────────────────────────┘

┌─ Top 10 Models by Tokens ──────────────────────────────┐
│ Rank │ Model                      │ Sessions │ Tokens  │
│  1   │ anthropic/claude-opus-4-7  │ 68       │ 1.2M    │
│ ...  │                            │          │         │
└────────────────────────────────────────────────────────┘

┌─ Top 10 Tools by Frequency ──────────────┐
│ Tool           │ Count │ % of all calls  │
│ Read           │ 412   │ 23.4%           │
│ Edit           │ 287   │ 16.3%           │
│ ...            │       │                 │
└──────────────────────────────────────────┘

┌─ Activity Heatmap (7×24) ───────────────────────┐
│       00 01 02 ... 09 10 11 12 13 14 15 ... 23  │
│ Mon    0  0  0 ...  8 12 15  3  2  9  7 ...  0  │
│ Tue    0  0  0 ... 10 14  8 ...                  │
│ ...                                              │
└──────────────────────────────────────────────────┘

┌─ Top 5 Insights per Category ──────────────────────────┐
│ [Pattern]                                              │
│  - "Weekly code review Mon 14:00" (n=4, conf=0.85)     │
│ [Preference]                                           │
│  - "Prefers shorter responses" (n=6, conf=0.78)        │
│ [Error]                                                │
│  - "rate_limit hit 3× on anthropic" (conf=0.92)        │
│ [Opportunity]                                          │
│  - "memory_recall tool never used (0 calls)" (conf=1.0)│
└────────────────────────────────────────────────────────┘
```

### 6.8 TDD 진입 순서

1. **RED #1**: `TestOverview_DeterministicAggregate` — AC-INSIGHTS-001.
2. **RED #2**: `TestModels_TokenDescSort` — AC-INSIGHTS-002.
3. **RED #3**: `TestTools_CountPercentageRounding` — AC-INSIGHTS-003.
4. **RED #4**: `TestActivity_ByDay_MonFirst` — AC-INSIGHTS-004.
5. **RED #5**: `TestActivity_ByHour_0to23` — AC-INSIGHTS-005.
6. **RED #6**: `TestActivity_MaxStreak_ConsecutiveDays` — AC-INSIGHTS-006.
7. **RED #7**: `TestModels_PricingMissing_NA` — AC-INSIGHTS-007.
8. **RED #8**: `TestPeriod_BoundsHonored` — AC-INSIGHTS-008.
9. **RED #9**: `TestPeriod_Invalid_ErrInvalidPeriod` — AC-INSIGHTS-009.
10. **RED #10**: `TestExtract_EmptyTrajectories_NoError` — AC-INSIGHTS-010.
11. **RED #11**: `TestScanner_150MBFile_StreamedNotLoaded` — AC-INSIGHTS-011.
12. **RED #12**: `TestScanner_MalformedLineSkipped` — AC-INSIGHTS-012.
13. **RED #13**: `TestAnalyzer_Pattern_Detected` — AC-INSIGHTS-013.
14. **RED #14**: `TestAnalyzer_Preference_ShorterResponses` — AC-INSIGHTS-014.
15. **RED #15**: `TestAnalyzer_Error_FailoverReasonFreq` — AC-INSIGHTS-015.
16. **RED #16**: `TestAnalyzer_Opportunity_UnusedTool` — AC-INSIGHTS-016.
17. **RED #17**: `TestExtract_LLMSummaryOffByDefault` — AC-INSIGHTS-017.
18. **RED #18**: `TestRenderTable_FiveSections` — AC-INSIGHTS-018.
19. **RED #19**: `TestJSONExport_TopLevelFields` — AC-INSIGHTS-019.
20. **GREEN**: scanner / 4 aggregators / analyzer / confidence / renderer.
21. **REFACTOR**: analyzer heuristic rule을 데이터 구조로 분리, streaming scanner를 generic iterator로 일반화.

### 6.9 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 19 AC 전부 테스트, 결정론성(AC-001) 재현 테스트 |
| **R**eadable | 4-dim + 4-cat 분리(engine.go vs analyzer.go), Insight 스키마 명시 |
| **U**nified | `golangci-lint`, JSON export 스키마와 내부 struct 일치 |
| **S**ecured | Evidence snippet 50자 cap (PII 우려), LLM 선택적(기본 off), 큰 파일 streaming(OOM 방지) |
| **T**rackable | 모든 Insight에 `Evidence[].SessionID` + `Timestamp`, zap 로그에 period + session count |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-TRAJECTORY-001 | `.jsonl` 파일 스키마, `Trajectory` 타입, `TrajectoryMetadata.FailureReason` |
| 선행 SPEC | SPEC-GOOSE-COMPRESSOR-001 | (선택) `Summarizer` 인터페이스 재사용 — `UseLLMSummary=true` 시 |
| 선행 SPEC | SPEC-GOOSE-MEMORY-001 | (선택) `MemoryManager` 주입 — Opportunity category 보강 |
| 선행 SPEC | SPEC-GOOSE-ERROR-CLASS-001 | `FailureReason` 문자열 ↔ `FailoverReason` enum 매핑 |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | `GOOSE_HOME`, zap 로거 |
| 후속 SPEC | SPEC-GOOSE-REFLECT-001 (Phase 5) | `Insight` 목록을 5단계 승격 입력으로 소비 |
| 후속 SPEC | SPEC-GOOSE-CLI-001 | `goose insights [--last 7]` 명령이 본 SPEC의 `Report.RenderTable()` 호출 |
| 외부 | Go 1.22+ | generics, time package |
| 외부 | `go.uber.org/zap` v1.27+ | CORE-001 계승 |
| 외부 | `github.com/tidwall/gjson` v1.17+ | trajectory JSON path 접근 (선택) |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 질적 분류 heuristic의 false positive/negative가 사용자 신뢰 저하 | 고 | 고 | 기본 `ConfidenceMin=0.5`로 cutoff, Evidence 강제(REQ-015), 사용자가 리포트에서 직접 승인/거부(REFLECT-001) |
| R2 | 큰 파일 streaming 중 scanner buffer overflow | 중 | 중 | `bufio.Scanner` 기본 `MaxScanTokenSize=64KB` 초과 라인 처리 위해 `bufio.Reader.ReadBytes('\n')` 사용 (64KB 제한 없음). 단일 라인이 파일 전체인 병리 케이스는 10MB cap으로 skip + warn |
| R3 | Pricing table에 없는 모델이 너무 많으면 `Cost` 값 무의미 | 중 | 중 | `HasFullPricing` bool로 명시, UI에서 "부분 pricing"이라고 표시. YAML 업데이트를 분기별 cron으로 권장 |
| R4 | MaxStreak 계산이 시간대(timezone) 전환 이슈 | 낮 | 중 | 모든 날짜 비교를 UTC로 통일(REQ-INSIGHTS-005의 `now.UTC()`), 사용자 로컬 표시만 최종 render에서 변환 |
| R5 | LLM narrative이 사용자 데이터를 "요약"하면서 재노출 | 중 | 중 | Evidence snippet은 TRAJECTORY-001의 redact된 값. 그래도 context 조합 우려 있으므로 UseLLMSummary=false 기본 |
| R6 | Weekday remap 오류(Go time.Weekday Sunday=0) | 중 | 낮 | `(int(weekday)+6)%7`로 Mon=0 변환, AC-004로 boundary 검증 |
| R7 | `ActivityPattern.ByDay`가 길이 7 아닌 경우 | 낮 | 중 | REQ-INSIGHTS-014 assertion, 초기화 시 fixed-length slice |
| R8 | Top 10/Top 5 cutoff가 장기 사용 후 중요 항목 누락 | 중 | 낮 | Report는 전체 저장, Render만 cutoff. JSON export는 전체 포함 |
| R9 | JSON export 포맷 변경이 하위 호환 깨뜨림 | 중 | 중 | `Report.SchemaVersion int` 필드 추가, minor는 하위 호환 유지 |
| R10 | Analyzer가 30일 범위에서도 느림(수천 세션) | 중 | 중 | 단일 pass 스캐너 + 인메모리 map. 10,000 sessions × 1KB = 10MB — 수용 가능. 더 큰 경우 streaming aggregate 고려(향후) |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-learning.md` §4 Insights 분류 체계(overview/models/tools/activity)
- `.moai/project/learning-engine.md` §1.2 Medium-term Learning (a)(b)(c)(d), §10 평가 메트릭
- `.moai/project/adaptation.md` §2 Persona Detection, §6 Time-based Adaptation
- `.moai/specs/ROADMAP.md` §4 Phase 4 #21, §7 MVP Milestone 3
- `.moai/specs/SPEC-GOOSE-TRAJECTORY-001/spec.md` — `.jsonl` 공급자
- `.moai/specs/SPEC-GOOSE-ERROR-CLASS-001/spec.md` — FailureReason 매핑

### 9.2 외부 참조

- **Hermes `insights.py`** (34KB): 원본 InsightsEngine
- **Bayesian confidence**: https://en.wikipedia.org/wiki/Beta_distribution (신뢰도 prior)
- **Go time.Weekday**: https://pkg.go.dev/time#Weekday (Sunday=0 주의)
- **Unicode box-drawing**: https://en.wikipedia.org/wiki/Box-drawing_character

### 9.3 부속 문서

- `./research.md` — Hermes 34KB → Go 600 LoC 이식 매핑, 신뢰도 공식 검증, 테이블 render 구현 전략
- `../SPEC-GOOSE-TRAJECTORY-001/spec.md` — 선행
- `../SPEC-GOOSE-MEMORY-001/spec.md` — Opportunity 보강
- `../SPEC-GOOSE-ERROR-CLASS-001/spec.md` — Error 카테고리 데이터
- `../SPEC-GOOSE-COMPRESSOR-001/spec.md` — (선택) Summarizer 재사용

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **5단계 승격 의사결정을 구현하지 않는다**. REFLECT-001.
- 본 SPEC은 **사용자 승인 UI를 포함하지 않는다**. CLI-001.
- 본 SPEC은 **실시간 streaming insights를 구현하지 않는다**(batch-only).
- 본 SPEC은 **차트 이미지 생성을 포함하지 않는다**(ASCII 테이블만).
- 본 SPEC은 **Federated / 다른 사용자 비교를 포함하지 않는다**.
- 본 SPEC은 **Anomaly Detection(Isolation Forest)을 구현하지 않는다**. Phase 5+.
- 본 SPEC은 **Markov chain 행동 예측을 포함하지 않는다**. 별도 SPEC.
- 본 SPEC은 **LoRA 훈련 데이터셋 준비를 포함하지 않는다**. LORA-001.
- 본 SPEC은 **Identity Graph 쿼리 통합을 포함하지 않는다**. IDENTITY-001 (Phase 6).
- 본 SPEC은 **Incremental update를 구현하지 않는다**(매 Extract는 full scan).
- 본 SPEC은 **LLM narrative 생성을 기본 활성화하지 않는다**(UseLLMSummary opt-in).

---

**End of SPEC-GOOSE-INSIGHTS-001**
