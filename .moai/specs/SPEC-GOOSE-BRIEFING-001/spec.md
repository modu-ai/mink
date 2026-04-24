---
id: SPEC-GOOSE-BRIEFING-001
version: 0.1.0
status: planned
created_at: 2026-04-22
updated_at: 2026-04-22
author: manager-spec
priority: P0
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-BRIEFING-001 — Morning Briefing Orchestrator (Fortune + Weather + Calendar, LLM Narrative, TTS Optional)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 #35, FORTUNE/WEATHER/CALENDAR/SCHEDULER 통합) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE v6.0 Daily Companion의 **아침 루틴 통합 SPEC**. SCHEDULER-001이 `MorningBriefingTime` 이벤트를 emit하면, 본 SPEC이 활성화되어 **3개 데이터 소스를 병렬 수집 → LLM으로 자연스러운 내러티브 생성 → 사용자에게 전달**한다.

3개 소스:
1. **FORTUNE-001** — 오늘의 운세 (opt-in, 엔터테인먼트)
2. **WEATHER-001** — 현재 날씨 + 오늘 예보 + 공기질
3. **CALENDAR-001** — 오늘의 일정 요약 + 다가오는 주요 이벤트

**핵심 가치**: 3개 소스를 별도로 나열하면 건조한 정보 덤프가 된다. 본 SPEC은 LLM을 이용해 **"안녕하세요 goos님, 서울은 오늘 맑고 따뜻해요. 출근 전 회의 준비하시고, 오후에는 감성 바이오리듬이 좋아서 창의적인 일에 잘 맞을 거예요..."** 같은 **연결된 내러티브**를 생성한다.

본 SPEC이 통과한 시점에서 `internal/ritual/briefing/` 패키지는:

- `BriefingOrchestrator`가 3개 소스를 `errgroup`으로 병렬 fetch,
- `NarrativeGenerator`가 LLM(ADAPTER-001)으로 통합 서사 생성,
- 사용자 선호 스타일 적용 (길이·톤·이모지·언어),
- TTS 옵션 (eSpeak / Piper / cloud TTS, 오프라인 우선),
- 브리핑 스킵·재생 control (사용자 interactive),
- SCHEDULER-001의 HOOK 이벤트 consumer로 등록.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): "매일 사용자에게 **아침마다** 오늘의 운세와 날씨 정보, 하루 일정을 **브리핑**" — 핵심 요구.
- 3개 소스 SPEC (FORTUNE/WEATHER/CALENDAR) 은 개별 데이터만 제공. "브리핑" = orchestration layer 필요.
- SCHEDULER-001이 이벤트를 emit 하지만 소비자가 없으면 무의미.
- adaptation.md §3 Style Adaptation (formality/verbosity) + §6.1 아침 모드 (6-10시 일정 브리핑 모드) 구체화.

### 2.2 상속 자산

- **SCHEDULER-001**: `MorningBriefingTime` HOOK 이벤트.
- **FORTUNE-001**: `FortuneGenerator.Generate(ctx, userID, date)`.
- **WEATHER-001**: `WeatherTool.GetCurrent/GetForecast/GetAirQuality`.
- **CALENDAR-001**: `Aggregator.GetTodaySchedule(userID)`.
- **ADAPTER-001**: LLM 호출.
- **MEMORY-001**: 사용자 style 선호 + 과거 브리핑 히스토리.
- **INSIGHTS-001**: 사용자 mood, verbosity 선호.

### 2.3 범위 경계

- **IN**: `BriefingOrchestrator`, 3개 소스 병렬 fetch, NarrativeGenerator, style 적용, TTS (optional), SCHEDULER HOOK consumer, 저장된 브리핑 history, skip/replay control.
- **OUT**: 각 소스 본체 구현 (FORTUNE/WEATHER/CALENDAR), STT (음성 입력), email/SMS 발송, Smart Display 연동 (별도 Gateway), 그래픽 차트 생성, 다국어 자동 번역 (언어는 config 고정).

---

## 3. 스코프

### 3.1 IN SCOPE

1. `internal/ritual/briefing/` 패키지.
2. `BriefingOrchestrator` struct:
   - `HandleMorningBriefingTime(ctx, event ScheduledEvent)` — SCHEDULER HOOK consumer
   - `Generate(ctx, userID, opts BriefingOptions) (*MorningBriefing, error)` — 직접 호출
   - `Deliver(ctx, briefing *MorningBriefing)` — 사용자에게 표시/재생
3. 3-way 병렬 fetch:
   - `errgroup.WithContext` 로 fortune/weather/calendar 동시
   - 개별 실패는 skip (해당 섹션 생략), 전체 실패 없이 부분 브리핑
4. `NarrativeGenerator`:
   - 시스템 프롬프트에 스타일 힌트(tone, length, emoji_pref, language)
   - Structured output: `{greeting, summary, transitions, closing}`
   - Fallback: LLM 실패 시 template 기반 flat rendering
5. `BriefingStyle` 설정:
   - `briefing.style.length: "short"|"medium"|"long"` (short=100자, medium=200자, long=400자)
   - `briefing.style.tone: "warm"|"professional"|"playful"`
   - `briefing.style.language: "ko"|"en"|"ja"|"zh"` (conversation_language 기본)
   - `briefing.style.emoji_level: 0|1|2|3` (0=없음, 3=많이)
6. `MorningBriefing` DTO:
   ```
   {
     userID, date, triggered_by: "scheduler" | "manual",
     sections: {fortune?, weather, calendar},
     narrative: "전체 연결된 텍스트",
     narrative_structure: {greeting, main, closing},
     audio_url?: "path/to/tts.wav",
     created_at, llm_model, fallback_used: bool
   }
   ```
7. TTS (optional):
   - `VoiceOutput` 인터페이스 + `PiperProvider` (offline, 로컬 모델) + `EspeakProvider` (가벼운 fallback)
   - Cloud TTS (Google, Amazon Polly) — 사용자 opt-in only, 프라이버시 경고
   - 음성 파일 캐시 `~/.goose/briefing/audio/{date}.wav`
8. Delivery:
   - Terminal text (default)
   - Audio playback (opt-in)
   - 나중에 Gateway로 push 가능 (현재는 local)
9. Skip/Replay:
   - 사용자가 이미 본 브리핑 → `is_read=true` 표시
   - "다시 들려줘" 요청 시 audio replay 또는 narrative 재출력
10. Briefing history: MEMORY-001에 `briefing:{userID}:{YYYYMMDD}` 로 영속.

### 3.2 OUT OF SCOPE

- 각 소스 본체 로직 (FORTUNE/WEATHER/CALENDAR에 위임)
- STT (voice input) — 사용자가 음성으로 "들려줘" 요청하는 기능은 별도 Voice SPEC
- Push notification 발송 — Gateway SPEC
- 이메일/SMS 브리핑 전송 — 외부 channel SPEC
- Smart Display (Google Nest, Amazon Show) 연동 — 별도 SPEC
- 그래픽 차트 (날씨 아이콘, 일정 타임라인 SVG) — CLI-001 TUI
- 실시간 업데이트 (일정 추가 시 자동 재생성) — batch-only
- 저녁 브리핑 / 밤 요약 — JOURNAL-001 의 역할, 본 SPEC은 아침만
- Multi-user family briefing — adaptation §9.2 후속
- A/B testing framework — 별도

---

## 4. EARS 요구사항

### 4.1 Ubiquitous

**REQ-BRIEF-001 [Ubiquitous]** — The `BriefingOrchestrator` **shall** register as a HOOK-001 consumer for `MorningBriefingTime` event at `goosed` startup; registration failure **shall** cause bootstrap error.

**REQ-BRIEF-002 [Ubiquitous]** — Every generated `MorningBriefing` **shall** include `userID`, `date` (local, midnight), `triggered_by`, `sections` map (may be partial), and `created_at` UTC timestamp.

**REQ-BRIEF-003 [Ubiquitous]** — The orchestrator **shall** emit structured zap logs `{user_id, sections_fetched, sections_failed, llm_latency_ms, tts_used, total_latency_ms}` at INFO level for every briefing.

### 4.2 Event-Driven

**REQ-BRIEF-004 [Event-Driven]** — **When** `HandleMorningBriefingTime(ctx, event)` is invoked by HOOK-001, the orchestrator **shall** (a) extract userID from session, (b) call `Generate(ctx, userID, defaultOpts)`, (c) call `Deliver(ctx, briefing)`, (d) persist to MEMORY-001.

**REQ-BRIEF-005 [Event-Driven]** — **When** `Generate` is called, the orchestrator **shall** fetch 3 sources in parallel via `errgroup.WithContext` with 10-second total timeout; sections that timeout or fail **shall** be omitted from `sections` with a WARN log, and the briefing **shall** proceed with remaining sections.

**REQ-BRIEF-006 [Event-Driven]** — **When** all 3 sources fail, `Generate` **shall** return `*MorningBriefing{Narrative: "죄송해요, 오늘 아침 정보를 가져오지 못했어요. 잠시 후 다시 시도해주세요.", FallbackUsed: true}` instead of returning an error; this prevents cascading failure.

**REQ-BRIEF-007 [Event-Driven]** — **When** `NarrativeGenerator.Build(sections, style)` is invoked, the LLM **shall** be given: (a) structured JSON of fetched sections, (b) style hints (length, tone, emoji level, language), (c) user context (recent mood from INSIGHTS-001, occupation from IDENTITY-001); response **shall** be structured JSON with `{greeting, main, closing, full_text}` fields.

**REQ-BRIEF-008 [Event-Driven]** — **When** LLM call fails or response fails schema validation, the orchestrator **shall** fall back to a template-based flat narrative: `"안녕하세요, [name]님. 오늘 [weather_summary]. 일정은 [calendar_summary]. 좋은 하루 되세요."` — `FallbackUsed=true`.

**REQ-BRIEF-009 [Event-Driven]** — **When** `config.briefing.tts.enabled == true` and `Deliver` is called, the orchestrator **shall** synthesize audio via the configured provider and store in `~/.goose/briefing/audio/{userID}-{date}.wav`; playback is launched asynchronously and **shall not** block the function return.

### 4.3 State-Driven

**REQ-BRIEF-010 [State-Driven]** — **While** `config.briefing.enabled == false`, the HOOK consumer registration **shall** be skipped; `MorningBriefingTime` events **shall** be ignored; no narrative is generated.

**REQ-BRIEF-011 [State-Driven]** — **While** a briefing for the same `(userID, local-date)` already exists in MEMORY-001 with `is_read==false`, new `MorningBriefingTime` events **shall** skip generation and log `briefing already pending` at INFO; user sees the prior unread briefing.

**REQ-BRIEF-012 [State-Driven]** — **While** `userID` has no `birth_date` in IDENTITY-001 AND `config.briefing.style.fortune == true`, the fortune section **shall** be silently omitted with `ErrBirthDateMissing` logged once per day (to avoid log spam).

### 4.4 Unwanted

**REQ-BRIEF-013 [Unwanted]** — The orchestrator **shall not** generate a briefing longer than 1000 characters total regardless of style setting; this prevents runaway LLM output.

**REQ-BRIEF-014 [Unwanted]** — The orchestrator **shall not** use personal information (예: 사용자 본명, 주소, 전화번호) in the narrative unless `userID` has the corresponding attribute in IDENTITY-001 AND `attribute.visibility == "briefing_ok"`.

**REQ-BRIEF-015 [Unwanted]** — The orchestrator **shall not** send telemetry (LLM prompts or narrative content) to any external service; all data stays local except the LLM call itself (which goes through ADAPTER-001's configured provider).

**REQ-BRIEF-016 [Unwanted]** — TTS synthesis **shall not** use cloud providers (Google Cloud TTS, Amazon Polly) unless `config.briefing.tts.cloud_opt_in == true`; default is local `piper` or `espeak`.

### 4.5 Optional

**REQ-BRIEF-017 [Optional]** — **Where** `config.briefing.adaptive_timing == true`, the orchestrator **shall** use INSIGHTS-001's last 7-day wake time to dynamically adjust the SCHEDULER's morning time; proposed time **shall** be sent to SCHEDULER via its update API, not modified directly by this SPEC.

**REQ-BRIEF-018 [Optional]** — **Where** recent 3-day mood trend shows negative valence, the LLM prompt **shall** include "사용자가 힘든 시기를 보내고 있으니, 따뜻하고 지지적인 톤으로" and the briefing closing **shall** end with an explicit supportive sentence.

**REQ-BRIEF-019 [Optional]** — **Where** an "emergency" weather alert is active (heavy rain, heat wave, 미세먼지 "매우 나쁨"), the briefing **shall** prepend a warning section before the narrative, rendered with distinctive formatting (prefix: "⚠️ 오늘 주의사항:").

**REQ-BRIEF-020 [Optional]** — **Where** Calendar section has an event starting within 2 hours of briefing time, the narrative **shall** highlight it ("곧 [시간]에 [제목] 일정이 있어요") — urgency emphasis.

---

## 5. 수용 기준

**AC-BRIEF-001 — HOOK 등록**
- **Given** `goosed` bootstrap
- **When** `briefing.NewOrchestrator(deps).Register(hookRegistry)`
- **Then** `hookRegistry.Handlers(EvMorningBriefingTime, _)` 반환에 본 orchestrator 포함.

**AC-BRIEF-002 — 3-way 병렬 fetch + 부분 실패**
- **Given** weather mock 성공, calendar mock 성공, fortune mock이 `ErrBirthDateMissing` 반환
- **When** `Generate(ctx, "u1", defaultOpts)`
- **Then** `err==nil`, `briefing.Sections.Weather != nil`, `briefing.Sections.Calendar != nil`, `briefing.Sections.Fortune == nil`, 로그에 "fortune section skipped" WARN.

**AC-BRIEF-003 — 전체 실패 → fallback narrative**
- **Given** 3개 소스 mock 모두 error
- **When** `Generate`
- **Then** `err==nil`, `briefing.FallbackUsed==true`, `briefing.Narrative` 에 "정보를 가져오지 못했어요" 포함.

**AC-BRIEF-004 — LLM narrative 생성 (scheme 일치)**
- **Given** 3개 소스 정상 응답 + LLM mock이 valid JSON 반환
- **When** `Generate`
- **Then** `briefing.Narrative` 비어있지 않음, `narrative_structure` 에 greeting/main/closing 3 필드 모두 채워짐.

**AC-BRIEF-005 — LLM 실패 시 template fallback**
- **Given** LLM mock이 400 에러, 3개 소스 정상
- **When** `Generate`
- **Then** `briefing.FallbackUsed==true`, `briefing.Narrative` 에 `weather_summary` / `calendar_summary` 치환 결과 포함.

**AC-BRIEF-006 — TTS 비활성 기본값**
- **Given** `config.briefing.tts.enabled=false` (default)
- **When** `Deliver(briefing)`
- **Then** `briefing.AudioURL==""`, TTS provider mock 호출 0회.

**AC-BRIEF-007 — 로컬 TTS만 default**
- **Given** `config.briefing.tts.enabled=true, tts.cloud_opt_in=false`
- **When** `Deliver`
- **Then** `PiperProvider` 또는 `EspeakProvider` 호출, Google/Polly mock 호출 0회.

**AC-BRIEF-008 — 중복 브리핑 억제**
- **Given** 오늘 07:30 브리핑 이미 생성됨 (`is_read=false`), 07:45 SCHEDULER가 재트리거 (restart 등)
- **When** `HandleMorningBriefingTime(ctx, event)`
- **Then** LLM mock 호출 0회, 로그에 "briefing already pending" INFO.

**AC-BRIEF-009 — 최대 길이 cap**
- **Given** LLM mock이 3000자 narrative 반환 (악의적/버그)
- **When** `Generate(style.length="long")`
- **Then** `len(briefing.Narrative) <= 1000`, 초과분 truncate + "..." 부착.

**AC-BRIEF-010 — 공기질 경보 prepend**
- **Given** WEATHER mock이 `AirQuality.Level="very_unhealthy"`, PM2.5=90
- **When** `Generate`
- **Then** `briefing.Narrative` 가 "⚠️ 오늘 주의사항:" 으로 시작, "미세먼지" 키워드 포함.

**AC-BRIEF-011 — 임박 일정 강조**
- **Given** 07:30 브리핑, 08:45 회의 있음 (75분 후)
- **When** `Generate`
- **Then** narrative 본문에 "곧 8시 45분에" 또는 "75분 뒤" 같은 임박 표현 포함.

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── briefing/
        ├── orchestrator.go        # BriefingOrchestrator
        ├── narrative.go           # NarrativeGenerator (LLM)
        ├── template.go            # fallback template
        ├── style.go               # BriefingStyle 적용
        ├── deliver.go             # text + TTS output
        ├── tts_piper.go           # 로컬 Piper
        ├── tts_espeak.go          # 로컬 espeak
        ├── tts_cloud.go           # Google/Polly (opt-in)
        ├── dedup.go               # 중복 방지
        ├── types.go               # MorningBriefing DTO
        ├── config.go
        └── *_test.go
```

### 6.2 핵심 타입 (의사코드)

```
BriefingOrchestrator {
  fortune   fortune.Generator
  weather   weather.Provider
  calendar  calendar.Aggregator
  narrative *NarrativeGenerator
  tts       VoiceOutput (optional)
  memory    memory.MemoryManager
  hooks     hook.Registry
  insights  insights.Reader
  identity  identity.IdentityGraph
  cfg       Config
  logger    *zap.Logger
}

HandleMorningBriefingTime(ctx, event hook.ScheduledEvent) error
Generate(ctx, userID, opts BriefingOptions) (*MorningBriefing, error)
Deliver(ctx, briefing *MorningBriefing) error

MorningBriefing {
  UserID, Date time.Time, TriggeredBy string
  Sections {
    Fortune  *fortune.FortuneReport
    Weather  *weather.WeatherReport
    Calendar *calendar.DailySchedule
  }
  Narrative string
  NarrativeStructure {Greeting, Main, Closing string}
  AudioURL string
  CreatedAt time.Time
  LLMModel string
  FallbackUsed bool
  IsRead bool
}

BriefingStyle {
  Length string ("short"|"medium"|"long")
  Tone string ("warm"|"professional"|"playful")
  Language string (BCP-47)
  EmojiLevel int (0-3)
}
```

### 6.3 Fetch 병렬성

```
g, gctx := errgroup.WithContext(ctx)
var fr *FortuneReport
var wr *WeatherReport
var ds *DailySchedule

g.Go(func() error {
  r, err := o.fortune.Generate(gctx, userID, today)
  if err != nil {
    o.logger.Warn("fortune failed", zap.Error(err))
    return nil  // skip, not fatal
  }
  fr = r
  return nil
})

g.Go(func() error { /* weather */ })
g.Go(func() error { /* calendar */ })

// 10s timeout via ctx
g.Wait()
```

### 6.4 LLM Prompt 구조

```
System:
당신은 [user_name]님의 따뜻한 아침 비서입니다. 제공된 정보를 한 편의 자연스러운 인사로 전달하세요.

스타일 가이드:
- 길이: {length} (short=100자, medium=200자, long=400자)
- 톤: {tone}
- 이모지: {emoji_level}
- 언어: {language}
- [optional] 사용자 최근 기분: {mood_hint}
- [optional] 사용자 직업: {occupation}

정보:
- 날씨: {weather_json}
- 일정: {calendar_json}
- 운세: {fortune_json}

출력은 반드시 JSON 형식:
{
  "greeting": "인사말 (20~30자)",
  "main": "본문 (스타일 length에 따라)",
  "closing": "마무리 격려 (10~30자)",
  "full_text": "greeting + main + closing 자연스럽게 연결"
}
```

### 6.5 Fallback Template

```
{greeting_hour}, {user_name}님. 오늘 {city}은(는) {weather_condition}이고 기온은 {temp_c}도입니다.
오늘 일정은 {calendar_summary}. {fortune_hint_or_empty}
좋은 하루 보내세요!
```

### 6.6 TTS

- **Piper** (`rhasspy/piper`): 오픈소스 neural TTS. 한국어 모델 존재 (ko_KR-lee). 로컬 실행, 프라이버시 안전.
- **espeak-ng**: 가볍지만 로봇 음성. fallback only.
- **cloud TTS**: Google Cloud TTS, Amazon Polly — 품질 우수하나 음성 데이터 전송. opt-in 필수.

Piper 호출:

```
exec.Command("piper", "--model", "ko_KR-lee.onnx", "--output_file", outPath)
stdin: narrative text
```

### 6.7 중복 억제

MEMORY-001 `facts` 테이블:
- key: `briefing:{userID}:{YYYYMMDD}`
- content: `MorningBriefing` JSON 요약
- source: "briefing"

`HandleMorningBriefingTime` 진입 시 key 존재 체크.

### 6.8 라이브러리 결정

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| 병렬 fetch | `golang.org/x/sync/errgroup` | 표준 |
| TTS (local) | `piper` 바이너리 외부 호출 | 품질·프라이버시 |
| TTS (legacy) | `espeak-ng` 바이너리 | fallback 가벼움 |
| Audio playback | `faiface/beep` 또는 OS 명령 (`afplay`, `aplay`) | 간단 |
| LLM | ADAPTER-001 경유 | 내부 |

### 6.9 TDD 진입

1. RED: `TestOrchestrator_RegistersHook` — AC-BRIEF-001
2. RED: `TestParallelFetch_PartialFailure` — AC-BRIEF-002
3. RED: `TestAllSourcesFail_FallbackNarrative` — AC-BRIEF-003
4. RED: `TestLLMNarrative_StructuredOutput` — AC-BRIEF-004
5. RED: `TestLLMFailure_TemplateFallback` — AC-BRIEF-005
6. RED: `TestTTS_DisabledByDefault` — AC-BRIEF-006
7. RED: `TestTTS_LocalProviderDefault` — AC-BRIEF-007
8. RED: `TestDuplicate_SkipsGeneration` — AC-BRIEF-008
9. RED: `TestMaxLength_1000Cap` — AC-BRIEF-009
10. RED: `TestAirQualityAlert_PrependWarning` — AC-BRIEF-010
11. RED: `TestUpcomingEvent_UrgencyEmphasis` — AC-BRIEF-011
12. GREEN → REFACTOR

### 6.10 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| **T**ested | 85%+, mock fortune/weather/calendar, fallback cascade 검증 |
| **R**eadable | orchestrator/narrative/template/tts 파일 분리 |
| **U**nified | MorningBriefing DTO 스키마 엄격 |
| **S**ecured | TTS 로컬 default, cloud opt-in, 개인정보 visibility 필터, LLM prompt에 PII 최소화 |
| **T**rackable | 각 섹션 fetch latency + LLM latency + total 기록 |

---

## 7. 의존성

| 타입 | 대상 | 설명 |
|-----|------|-----|
| 선행 SPEC | SCHEDULER-001 | `MorningBriefingTime` HOOK 이벤트 |
| 선행 SPEC | FORTUNE-001 | 오늘의 운세 |
| 선행 SPEC | WEATHER-001 | 날씨 |
| 선행 SPEC | CALENDAR-001 | 일정 |
| 선행 SPEC | ADAPTER-001 | LLM |
| 선행 SPEC | MEMORY-001 | 브리핑 history 영속 |
| 선행 SPEC | INSIGHTS-001 | mood 힌트 |
| 선행 SPEC | IDENTITY-001 | 사용자 이름, 직업 |
| 선행 SPEC | HOOK-001 | 이벤트 subscribe |
| 후속 SPEC | RITUAL-001 | 리추얼 완수 추적 |
| 외부 | Piper TTS (optional) | 로컬 음성 |
| 외부 | espeak-ng (optional) | fallback 음성 |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | LLM latency 10초 초과로 briefing 지연 | 중 | 중 | 10s timeout, fallback template, 사용자에게 "조금 늦어요" 안내 |
| R2 | 3개 소스 병렬 호출로 rate limit 동시 hit | 낮 | 중 | 각 provider 자체 rate limit, aggregator는 관찰만 |
| R3 | TTS 생성이 매일 500MB 누적 | 중 | 낮 | 7일 rolling retention, 자동 정리 cron |
| R4 | 개인정보 LLM 프롬프트 노출 | 중 | 고 | visibility 필터 (attribute.visibility=="briefing_ok"만 허용), PII 최소화 |
| R5 | 중복 억제 key 충돌 (TZ 변경) | 낮 | 낮 | key에 TZ 포함 |
| R6 | 우울 상태 사용자에게 부적절한 playful 톤 | 중 | 중 | mood 감지 → tone auto-adjust (REQ-BRIEF-018) |
| R7 | TTS 한국어 모델 다운로드 실패 | 중 | 낮 | 첫 실행 시 모델 fetch, 실패 시 text-only |

---

## 9. 참고

### 9.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-SCHEDULER-001/spec.md` — trigger source
- `.moai/specs/SPEC-GOOSE-FORTUNE-001/spec.md` — source 1
- `.moai/specs/SPEC-GOOSE-WEATHER-001/spec.md` — source 2
- `.moai/specs/SPEC-GOOSE-CALENDAR-001/spec.md` — source 3
- `.moai/specs/SPEC-GOOSE-RITUAL-001/spec.md` — completion tracking
- `.moai/project/adaptation.md` §3 Style, §6.1 Time-based, §7 Mood

### 9.2 외부 참조

- Piper TTS: https://github.com/rhasspy/piper
- espeak-ng: https://github.com/espeak-ng/espeak-ng
- Google Cloud TTS: https://cloud.google.com/text-to-speech
- errgroup: https://pkg.go.dev/golang.org/x/sync/errgroup

### 9.3 부속 문서

- `./research.md` — LLM prompt 실험, TTS 한국어 모델 비교, 스타일 가이드 샘플

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **각 데이터 소스 본체를 구현하지 않는다** (FORTUNE/WEATHER/CALENDAR에 위임).
- 본 SPEC은 **STT (음성 입력) 를 포함하지 않는다** (별도 Voice SPEC).
- 본 SPEC은 **Push notification 발송을 포함하지 않는다** (Gateway SPEC).
- 본 SPEC은 **Email/SMS 브리핑 전송을 포함하지 않는다**.
- 본 SPEC은 **Smart Display 연동을 포함하지 않는다**.
- 본 SPEC은 **그래픽 차트 (SVG/PNG) 를 생성하지 않는다** (text + audio only).
- 본 SPEC은 **실시간 브리핑 업데이트를 지원하지 않는다** (일회성 생성).
- 본 SPEC은 **저녁 브리핑 / 밤 요약을 포함하지 않는다** (JOURNAL-001).
- 본 SPEC은 **가족 모드 multi-user briefing을 포함하지 않는다**.
- 본 SPEC은 **A/B 테스트 인프라를 포함하지 않는다**.
- 본 SPEC은 **사용자 voice cloning 을 포함하지 않는다** (TTS는 사전 훈련된 목소리만).

---

**End of SPEC-GOOSE-BRIEFING-001**
