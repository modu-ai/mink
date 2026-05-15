# ritual/briefing 패키지 — Morning Briefing 통합 (SPEC-MINK-BRIEFING-001)

**위치**: internal/ritual/briefing/
**파일**: 22 production + 22 test = 44개
**최근 변경**: SPEC-MINK-BRIEFING-001 v0.3.1 (M4 full wiring, PR #186)
**상태**: ✅ Implemented (v0.3.1, AC 21/21 GREEN, coverage 88.1%)

---

## 목적

매일 아침 한 번 4개 데이터 모듈(Weather + Journal Recall + Date/Calendar + Mantra)을 수집해 3개 채널(CLI · Telegram · TUI)로 출력. WEATHER-001 / JOURNAL-001 / SCHEDULER-001 / MSG-TELEGRAM-001 4 SPEC을 통합하는 Sprint 2 두 번째 SPEC.

외부 dependency 0 — 24절기 + 한국 공휴일은 internal table.

---

## 공개 API

### Payload (types.go)

```go
type BriefingPayload struct {
    GeneratedAt   time.Time
    Date          string
    DateCalendar  *DateModule
    Weather       *WeatherModule
    JournalRecall *RecallModule
    Mantra        *MantraModule
    LLMSummary    *LLMSummaryModule  // M3 (v0.3.0+)
    Status        map[string]string  // module → "ok"|"timeout"|"error"|"offline"
}

type WeatherModule struct {
    Current    *WeatherCurrent
    Forecast   *WeatherForecast
    AirQuality *AirQuality
    Offline    bool
}
// + WeatherCurrent, WeatherForecast (Days []WeatherForecastDay), AirQuality, RecallModule,
//   AnniversaryEntry, Anniversary, MoodTrend, DateModule, SolarTerm, KoreanHoliday, MantraModule
```

### Orchestrator (orchestrator.go)

```go
type Collector interface {
    Collect(ctx context.Context, userID string, today time.Time) (any, string)
}

type Orchestrator struct { /* 4 collectors + Options */ }

func NewOrchestrator(weather, journal, date, mantra Collector, opts ...Option) *Orchestrator
func (o *Orchestrator) Run(ctx context.Context, userID string, today time.Time) (*BriefingPayload, error)

// Options
func WithLLMProvider(p llm.LLMProvider) Option
func WithLLMModel(model string) Option
func WithConfig(cfg *Config) Option
```

### Collectors

| 파일 | Collector | 역할 |
|------|-----------|------|
| collect_weather.go | `WeatherCollector` | `WeatherFetcher` interface(Current/Forecast/AirQuality) 호출, MockForecastDay → WeatherForecastDay 변환 |
| collect_journal.go | `JournalCollector` | `JournalRecaller` interface로 기념일·MoodTrend 회수 |
| collect_date.go | `DateCollector` | 절기 + 한국 공휴일 lookup (외부 dep 0) |
| collect_mantra.go | `MantraCollector` | mantra 라이브러리 무작위 선택 + clinical-vocab silent reject |
| collect_adapters.go | 4 adapter | concrete collector → `Collector` interface 매핑 |

### Renderers

| 파일 | 출력 | 비고 |
|------|------|------|
| render_cli.go | colorized CLI text | M4 amendment: `Status` map 순회를 `ModuleStatusOrder` fixed slice로 결정성 보장 |
| render_telegram.go | Markdown V2 | crisis prepend 적용 |
| render_tui.go | bubbletea-friendly string | BriefingPanel 본문 |

3 renderer 모두 `PrependCrisisResponseIfDetected` 통과 (REQ-BR-061).

### LLM Summary (M3 — llm_summary.go)

```go
// Categorical-only payload — entry text / mantra body / chat_id 금지
type LLMSummaryRequest struct {
    Date              string
    DayOfWeek         string
    WeatherStatus     string
    WeatherCondition  string
    WeatherTempInt    int
    WeatherLocation   string
    AQILevel          string
    JournalStatus     string
    AnniversaryCount  int
    AnniversaryYears  []int
    MoodTrendSlope    int
    SolarTermPresent  bool
    HolidayPresent    bool
    MantraPresent     bool
}

func BuildLLMSummaryRequest(payload *BriefingPayload) *LLMSummaryRequest
func FormatLLMPrompt(req *LLMSummaryRequest) string  // 한국어 2~3줄 요청 프롬프트
func GenerateLLMSummary(ctx, provider, payload, cfg, model) (string, error)
```

Privacy invariant: 일기 본문/mantra 본문/좌표/chat_id는 prompt에 절대 포함되지 않음 (`privacy_test.go` 보증).

M4 wiring: `Orchestrator.Run()` 내부에서 `cfg.LLMSummary == true && provider != nil` 일 때만 호출 — error 시 graceful degradation (Status="error_llm", 본 흐름 영향 0).

### Crisis Response (M3 — crisis_response.go)

```go
func CheckCrisis(rendered string) bool
func PrependCrisisResponseIfDetected(rendered string) string  // 3 renderer 모두 적용
func PayloadHasCrisis(payload *BriefingPayload) bool
```

JOURNAL-001의 `CrisisDetector` + `CrisisResponse`를 재사용 — 위기 키워드(자해/극단선택 어휘) 검출 시 한국 자살예방상담전화 1393 + 정신건강위기상담 1577-0199를 응답 prefix로 부착.

### Cron / Hook (cron.go)

```go
type BriefingHookHandler struct { /* orchestrator + userID + onDone */ }

func NewBriefingHookHandler(orch, userID, onDone, logger) *BriefingHookHandler
func (h *BriefingHookHandler) Handle(ctx, hook.HookInput) (hook.HookJSONOutput, error)
func (h *BriefingHookHandler) Matches(hook.HookInput) bool  // EvMorningBriefingTime

func RegisterMorningBriefing(reg *hook.HookRegistry, h *BriefingHookHandler) error
```

SCHEDULER-001과 정확히 1 hook event(`EvMorningBriefingTime`)으로 연결. SCHEDULER 측 수정 0.

### Archive (archive.go)

```go
func ArchiveDir() (string, error)                              // ~/.mink/briefings (0700)
func WriteArchive(payload *BriefingPayload) (string, error)    // ./<date>.json (0600)
func WriteArchiveToDir(dir string, payload *BriefingPayload) (string, error)
```

Privacy Invariant 2: archive permissions ≤ 0700/0600. 실서비스 경로 + test path 동일 강제.

### Audit (audit.go)

```go
type AuditLogger struct { /* zap structured logger */ }

func NewAuditLogger() *AuditLogger
func (a *AuditLogger) LogCollection(module, status, duration, err)
func (a *AuditLogger) LogOrchestration(payload, totalDuration)
func (a *AuditLogger) Sync() error

// Redaction helpers
func RedactEntryText(text string) string   // → "<entry>"
func RedactMantraText(text string) string  // → "<mantra>"
func RedactChatID(chatID string) string    // → "<chat>"
func RedactAPIKey(key string) string       // → "***"
```

### Calendar (외부 dep 0)

```go
func SolarTermOnDate(year, month, day int) (*SolarTerm, error)          // 24절기 internal table
func LookupKoreanHoliday(year, month, day int) (*KoreanHoliday, error)  // 한국 명절 internal table
```

### Config (config.go)

```go
type Config struct {
    Enabled         bool
    LLMSummary      bool   // default false (M1/M2 deterministic mode)
    TelegramEnabled bool
    Location        string
    LLMModel        string
    LLMTimeout      time.Duration
}

func DefaultConfig() *Config
func (c *Config) Validate() error
```

---

## 외부 의존

| 의존 | 사용 위치 | 비고 |
|------|----------|------|
| `internal/llm` (`LLMProvider`) | llm_summary.go | optional — cfg.LLMSummary off 시 미사용 |
| `internal/ritual/journal` (`CrisisDetector`/`CrisisResponse`) | crisis_response.go | 위기 어휘 재사용 |
| `internal/hook` | cron.go | EvMorningBriefingTime 매칭 |
| `internal/cli/messenger/telegram` | render_telegram.go (지표만) | sender 의존성은 호출자 측 주입 |
| `go.uber.org/zap` | audit.go | structured logging |

내부 ritual/journal 와 강결합 — crisis vocabulary 단일 source of truth.

---

## 의존 호출자 (fan-in)

| 호출자 | 진입점 |
|--------|--------|
| `internal/cli/commands/briefing.go` | `mink briefing` cobra 명령 (production wiring, M4) |
| `internal/cli/tui/briefing_dispatch.go` | TUI `/briefing` slash command (BriefingRunner) |
| SCHEDULER-001 hook registry | EvMorningBriefingTime → `BriefingHookHandler.Handle` |

자세한 TUI wiring은 [cli-tui-briefing.md](./cli-tui-briefing.md) 참고.

---

## 핵심 invariants (REQ-BR-001~064, AC-001~017)

1. **Privacy Invariant 1**: LLM prompt에 entry text/mantra body/chat_id 부재 (`privacy_test.go`)
2. **Privacy Invariant 2**: archive 파일 0600 / 디렉토리 0700
3. **Crisis prepend**: 3 renderer 모두 위기 어휘 검출 시 hotline prefix 필수 (REQ-BR-055/061)
4. **Module Status order**: Map iteration 비결정성 차단 — `ModuleStatusOrder` fixed slice (M4)
5. **Graceful degradation**: 4 module 어느 하나라도 실패 시 Status="error_module"로 표기, 나머지 모듈 진행
6. **Determinism by default**: cfg.LLMSummary 기본 false — LLM 미사용 시 동일 입력 동일 출력

---

## 테스트 커버리지 (v0.3.1)

| 영역 | 파일 | 비고 |
|------|------|------|
| Privacy | privacy_test.go | 6 invariants (5 GREEN, 1 partial = M3 폐기 미사용 path) |
| Orchestration | orchestrator_test.go + _integration + _llm | 4-module fanout + LLM toggle |
| Renderers | render_cli/telegram/tui + render_crisis + render_cli_order | 3 채널 + crisis prepend + order determinism |
| Crisis | crisis_response_test.go | JOURNAL-001 vocabulary 재사용 검증 |
| Archive | archive_test.go | perm 0600/0700 enforcement |
| Cron | cron_test.go | EvMorningBriefingTime matcher |
| Audit | audit_test.go | redaction 4종 + zap structured |
| Solarterm/Holiday | solarterm_test.go, holiday_test.go | 24절기 + 한국 명절 table |
| Fanout | fanout_integration_test.go | 4 collector 병렬 + status map |

총 coverage 88.1% (M4 종결 시점). DoD 85% 충족.

---

## SPEC 진화 요약

| 버전 | M | 주요 변경 |
|------|---|----------|
| 0.1.0 | — | 초안. Sprint 2 두 번째 SPEC, 4 SPEC 통합 청사진 |
| 0.2.0 | M1+M2 | 4 collectors + orchestrator + CLI render + cobra + telegram + archive + SCHEDULER hook |
| 0.3.0 | M3 | LLM summary (categorical-only) + crisis hotline (3 renderer) + BriefingPanel snapshot |
| 0.3.1 | M4 | 5 wiring gap 종결 — Mock→Real factory / Orchestrator Options / LLM error path / 3 channel crisis / slash dispatch / Module Status order |

---

## 참고

- SPEC: `.moai/specs/SPEC-MINK-BRIEFING-001/`
- TUI 측 wiring: [cli-tui-briefing.md](./cli-tui-briefing.md)
- 자매 모듈: ritual/journal (crisis vocabulary), ritual/scheduler (cron event), llm-provider (summary backend)
