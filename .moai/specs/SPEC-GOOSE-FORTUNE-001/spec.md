---
id: SPEC-GOOSE-FORTUNE-001
version: 0.1.0
status: planned
created_at: 2026-04-22
updated_at: 2026-05-17
author: manager-spec
priority: P1
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels: [fortune, personalization, entertainment, opt-in, phase-7]
target_milestone: v0.2.0
mvp_status: deferred
deferred_reason: "0.1.0 MVP 범위 외 — v0.2.0 이월 (2026-05-17 사용자 확정). 엔터테인먼트 기능, MVP 후순위."
---

# SPEC-GOOSE-FORTUNE-001 — Personalized Fortune Generator (사주·바이오리듬·오늘의 운세, Opt-in)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 #33, IDENTITY-001 생년월일 소비) | manager-spec |

---

## 1. 개요 (Overview)

MINK v6.0 Daily Companion의 **아침 브리핑 3대 축** 중 하나인 **개인화 운세 생성기**를 정의한다. 사용자의 생년월일(IDENTITY-001 Person entity 에서 소비) + 오늘 날짜 + 최근 감정 트렌드 (INSIGHTS-001)를 입력으로, **한국 사주 + 서양 점성술 + 바이오리듬**을 융합한 "오늘의 운세"를 LLM이 생성한다.

본 SPEC은 **엔터테인먼트 목적**임을 명확히 선언한다:
- 의학적 진단 불가
- 금융 투자 조언 불가
- 중요한 인생 결정의 근거로 사용 불가

모든 생성물에는 `disclaimer` 필드가 자동 부착되며, 사용자는 언제든 opt-out 가능하다.

본 SPEC이 통과한 시점에서 `internal/ritual/fortune/` 패키지는:

- `FortuneGenerator` 가 LLM (ADAPTER-001) + 사주/바이오리듬 결정론 계산기를 조합,
- `SajuCalculator`가 사용자 생년월일시 → 사주팔자(천간·지지) 산출,
- `BiorhythmCalculator`가 신체/감성/지성 3대 사이클 계산 (23/28/33일),
- 생성된 `FortuneReport` 가 opt-in 사용자에게만 BRIEFING-001 경유 표시,
- 의학·금전 관련 키워드는 LLM 프롬프트 레벨에서 **차단**.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): "아침마다 **오늘의 운세**와 날씨 정보, 하루 일정을 브리핑" — 한국 사용자 친숙한 아침 루틴 요소.
- 한국 사용자 대상: 사주·운세는 문화적으로 친숙. 다만 미신적 요소로 **엔터테인먼트임을 명시** 필수.
- 생년월일은 이미 IDENTITY-001 Person entity 에 attribute로 저장 가능. 본 SPEC은 그 데이터를 소비.
- adaptation.md §6.3 Season Mode 에 "연말연초 회고, 결심" 언급. 운세는 감정적 공감 포인트.

### 2.2 상속 자산

- **IDENTITY-001 Entity**: Person entity의 `Attributes` map에서 `birth_date` (YYYY-MM-DD) + optional `birth_time_hhmm` + `birth_place_tz` 획득.
- **ADAPTER-001 (LLM)**: LLM 호출로 운세 narrative 생성.
- **INSIGHTS-001**: 최근 감정 트렌드 (Mood Pattern) 참조로 개인화.
- **한국 만세력 DB**: 음력/양력 변환, 년주·월주·일주·시주 계산.
- **바이오리듬 공식**: 출생일 경과일 × sin(2π / cycle_days).

### 2.3 범위 경계

- **IN**: `FortuneGenerator`, `SajuCalculator`, `BiorhythmCalculator`, Tarot-like "오늘의 카드" (optional), Opt-in config, LLM prompt guard (의료/금융 키워드 차단), `FortuneReport` DTO, disclaimer 자동 부착.
- **OUT**: 연간 운세 / 평생 사주 풀이 (심도 높은 상담은 사용자에게 전문가 권장), 타로 카드 전체 덱 (78장) 고도화, 관상/수상(손금) 등 이미지 기반, 음력 달력 전체 기능(만세력 lib에 위임).

---

## 3. 스코프

### 3.1 IN SCOPE

1. `internal/ritual/fortune/` 패키지.
2. `FortuneGenerator` struct:
   - `Generate(ctx, userID, date) (*FortuneReport, error)`
   - 입력: userID → IDENTITY-001 조회, date
   - 출력: FortuneReport (narrative + 구조화 필드)
3. `SajuCalculator`:
   - 생년월일시 → 사주팔자 (4 기둥 × 2 글자 = 8 글자)
   - 오행 (木火土金水) 균형 판정
   - 일주 (日柱) = 당일 주도 기운
4. `BiorhythmCalculator`:
   - 신체 (physical, 23일), 감성 (emotional, 28일), 지성 (intellectual, 33일)
   - 출생일 경과일 `D` → `sin(2π × D / cycle) × 100` (%)
   - "오늘의 고점/저점" 플래그
5. `WesternAstrology` (optional):
   - 양력 생일 → 12 별자리 매핑
   - 해당 별자리 일반 특성 + 오늘의 간단 해석
6. `LLMFortunePrompt`:
   - 사주·바이오리듬·별자리·최근 감정 정보를 조합한 시스템 프롬프트
   - Output 제약: JSON schema (narrative + categories)
   - **Guard**: 의료("병", "질병", "치료"), 금융("투자", "주식", "부자"), 복권 숫자 등 키워드 → 프롬프트에서 명시적으로 "이런 표현을 피하고, 일반적 응원 톤을 유지하라" 지시
7. `FortuneReport` DTO:
   ```
   {
     date, user_id,
     saju: { day_pillar_hanja, day_pillar_kor, five_elements, dominant },
     biorhythm: { physical_pct, emotional_pct, intellectual_pct, highlights },
     astrology?: { zodiac_sign_ko, zodiac_sign_en, today_hint },
     narrative: "오늘은 감성 바이오리듬이 상승세네요...",
     mood_suggestion: "차분한 음악과 함께 시작해보세요",
     lucky_color?: "연한 파란색",
     lucky_number?: 7,
     disclaimer: "본 운세는 엔터테인먼트 목적이며, ...",
     generated_at, llm_model_used, opt_out_url
   }
   ```
8. Opt-in config:
   - `fortune.enabled: false` (기본값 false, 명시적 opt-in 필요)
   - `fortune.style: "saju_western_biorhythm" | "saju_only" | "western_only" | "biorhythm_only"`
   - `fortune.tone: "gentle" | "playful" | "mystical"`
   - `fortune.include_lucky_items: true`
9. 캐시: per (user_id, date) 1회만 생성 (같은 날 재호출 시 동일 결과 반환).

### 3.2 OUT OF SCOPE

- **타로 카드 78장 풀 덱**: 본 SPEC은 "오늘의 한 장 (Major Arcana 22장)" simplified 만 optional.
- **연간 운세 / 신년 사주**: 사용자가 명시 요청 시 별도 LLM 호출, 본 SPEC 스코프 외.
- **관상 / 수상 / 풍수**: 이미지 기반 점술. 스코프 외.
- **궁합 / 커플 운세**: 두 번째 사용자 데이터 필요. 별도 SPEC.
- **매 시간 운세** (시간대별): 오전/오후 2회만 가능하되, 본 SPEC은 1일 1회.
- **음력 달력 API**: 공공 OpenAPI 만세력(만세력 2.0) 또는 `korean-lunar-calendar` Go 포팅. 본 SPEC은 소비자.
- **투자·복권 번호 생성**: 명시적으로 제외, LLM guard가 차단.

---

## 4. EARS 요구사항

### 4.1 Ubiquitous

**REQ-FORTUNE-001 [Ubiquitous]** — Every `FortuneReport` **shall** include a non-empty `Disclaimer` field with text: "본 운세는 엔터테인먼트 목적으로 생성되었으며, 의학·금융·법률적 조언을 대체할 수 없습니다."; Korean (ko) and English (en) variants selected by `conversation_language`.

**REQ-FORTUNE-002 [Ubiquitous]** — The `FortuneGenerator` **shall** require explicit opt-in via `config.fortune.enabled == true`; default value is `false`; attempts to call `Generate` when disabled **shall** return `ErrFortuneDisabled`.

**REQ-FORTUNE-003 [Ubiquitous]** — The `SajuCalculator.Calculate(birth)` **shall** be deterministic — identical birth inputs **shall** produce identical pillars regardless of call time or locale.

**REQ-FORTUNE-004 [Ubiquitous]** — The `FortuneGenerator` **shall** cache results per `(user_id, YYYYMMDD)` for 24 hours; calling `Generate` twice the same day **shall** return the cached report.

### 4.2 Event-Driven

**REQ-FORTUNE-005 [Event-Driven]** — **When** `Generate(ctx, userID, date)` is called, the generator **shall** (a) load user birth_date from IDENTITY-001, (b) compute Saju + Biorhythm deterministically, (c) build LLM prompt with guard instructions, (d) call ADAPTER-001 LLM, (e) parse structured output, (f) attach disclaimer, (g) persist cache.

**REQ-FORTUNE-006 [Event-Driven]** — **When** the user's birth_date is absent or invalid in IDENTITY-001, the generator **shall** return `ErrBirthDateMissing` with Korean message "생년월일 정보가 필요해요. 설정에서 입력해주세요."; **shall not** guess or use default values.

**REQ-FORTUNE-007 [Event-Driven]** — **When** the LLM output contains any of the blocked keywords (`투자`, `주식`, `복권`, `질병`, `병`, `치료`, `확실히 낫는다`, `반드시`, `100%`), the generator **shall** log a WARN, strip the offending sentence, and regenerate up to 2 times; if still blocked after 3 attempts, return a safe template fallback.

**REQ-FORTUNE-008 [Event-Driven]** — **When** `config.fortune.style` restricts to a subset (e.g., "biorhythm_only"), the generator **shall** omit non-selected sections from both the prompt and the output.

### 4.3 State-Driven

**REQ-FORTUNE-009 [State-Driven]** — **While** `BriefingOrchestrator` (BRIEFING-001) renders the morning briefing, it **shall** call `Generate` only if `config.fortune.enabled == true`; otherwise the fortune section **shall** be omitted without error.

**REQ-FORTUNE-010 [State-Driven]** — **While** IDENTITY-001 backend is unavailable (`ErrBackendUnavailable`), `Generate` **shall** return `ErrDependencyUnavailable` and BRIEFING-001 **shall** gracefully skip the fortune section.

### 4.4 Unwanted

**REQ-FORTUNE-011 [Unwanted]** — The generator **shall not** produce lucky numbers between 1 and 45 (Korean Lotto range) or 6-digit numeric sequences that could be mistaken for lottery; if user requests, redirect to safe disclaimer.

**REQ-FORTUNE-012 [Unwanted]** — The generator **shall not** use mood-manipulative or fear-inducing language (e.g., "불행", "저주", "큰일"); LLM system prompt explicitly forbids such tone.

**REQ-FORTUNE-013 [Unwanted]** — The `SajuCalculator` **shall not** rely on system timezone; all calculations **shall** use the `birth_place_tz` attribute stored in IDENTITY-001; missing value defaults to `Asia/Seoul` with a WARN log.

### 4.5 Optional

**REQ-FORTUNE-014 [Optional]** — **Where** `config.fortune.tone == "playful"`, the LLM **shall** adopt a casual, emoji-allowed style; `"mystical"` uses poetic/antique Korean expressions; `"gentle"` (default) is warm and supportive.

**REQ-FORTUNE-015 [Optional]** — **Where** recent emotion trend (INSIGHTS-001 last 7 days) shows consistent negative Valence (< 0.4), the LLM prompt **shall** include instruction "사용자가 최근 힘들어 보이므로, 운세 톤을 특히 따뜻하고 지지적으로" and never use negative predictions.

---

## 5. 수용 기준

**AC-FORTUNE-001 — Opt-in 기본값**
- **Given** `config.fortune.enabled`가 설정되지 않음 (default false)
- **When** `FortuneGenerator.Generate(ctx, "u1", today)`
- **Then** `ErrFortuneDisabled` 반환, LLM mock 호출 0회.

**AC-FORTUNE-002 — 생년월일 없음**
- **Given** `config.fortune.enabled=true`, IDENTITY-001 Person "u1" 의 Attributes에 birth_date 없음
- **When** `Generate`
- **Then** `ErrBirthDateMissing` 반환, 메시지에 "생년월일 정보가 필요해요".

**AC-FORTUNE-003 — Saju 결정론**
- **Given** birth=1990-05-15 10:30 Asia/Seoul
- **When** `SajuCalculator.Calculate(birth)` 100회 반복
- **Then** 모든 호출 반환값 동일 (day_pillar, five_elements, dominant).

**AC-FORTUNE-004 — Biorhythm 수식**
- **Given** birth=1990-01-01, date=2026-04-22 (경과 13,260일)
- **When** `BiorhythmCalculator.Physical(birth, date)`
- **Then** 반환값 = `round(sin(2π × 13260 / 23) × 100, 2)` 와 일치.

**AC-FORTUNE-005 — Disclaimer 자동 부착**
- **Given** 정상 생성 시나리오
- **When** `Generate`
- **Then** `report.Disclaimer` 비어있지 않고, "엔터테인먼트 목적" 또는 "의학·금융·법률적 조언을 대체할 수 없" 문구 포함.

**AC-FORTUNE-006 — 키워드 차단**
- **Given** LLM mock이 첫 응답에 "이번 주 주식 투자가 좋을 것입니다" 포함
- **When** `Generate`
- **Then** 재생성 1회 이상 시도, 최종 응답에 "주식" "투자" 미포함. Log에 WARN 1건.

**AC-FORTUNE-007 — 복권 번호 거부**
- **Given** LLM mock이 "행운의 번호: 3, 7, 11, 23, 28, 42"
- **When** `Generate`
- **Then** 6개 숫자 sequence 스트립, `report.LuckyNumber` 은 단일 숫자 (1-9) 또는 nil.

**AC-FORTUNE-008 — 동일 날짜 캐시**
- **Given** `Generate(u1, 2026-04-22)` 1회 성공
- **When** 2시간 뒤 동일 호출
- **Then** LLM mock 호출 0회, 첫 번째 결과와 byte-level 동일.

**AC-FORTUNE-009 — 우울 상태 시 톤 조정**
- **Given** INSIGHTS-001 mock이 최근 7일 Valence 평균 0.3 반환
- **When** `Generate`
- **Then** LLM에 전달된 system prompt 문자열에 "지지적" 또는 "따뜻하" 키워드 포함, "불행" "나쁜" 등 negative 키워드 미포함.

**AC-FORTUNE-010 — Style 필터링**
- **Given** `config.fortune.style="biorhythm_only"`
- **When** `Generate`
- **Then** `report.Saju == nil`, `report.Astrology == nil`, `report.Biorhythm != nil`, narrative는 biorhythm 중심.

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── fortune/
        ├── generator.go       # FortuneGenerator
        ├── saju.go            # SajuCalculator
        ├── biorhythm.go       # BiorhythmCalculator
        ├── western.go         # 12 zodiac
        ├── lunar.go           # 양력↔음력 변환
        ├── prompts.go         # LLM system prompt templates
        ├── guards.go          # 키워드 차단 + sanitizer
        ├── cache.go           # per (user,date) 24h cache
        ├── types.go           # FortuneReport, SajuData, Biorhythm, ZodiacSign
        ├── config.go
        └── *_test.go
```

### 6.2 핵심 타입

```go
type FortuneGenerator struct {
    identity  identity.IdentityGraph
    llm       adapter.LLMProvider     // ADAPTER-001
    insights  insights.Reader
    cache     *FortuneCache
    cfg       Config
    logger    *zap.Logger
}

func (g *FortuneGenerator) Generate(ctx context.Context, userID string, date time.Time) (*FortuneReport, error)

type FortuneReport struct {
    Date          time.Time
    UserID        string
    Saju          *SajuData       // nullable per style
    Biorhythm     *Biorhythm
    Astrology     *Astrology
    Narrative     string          // LLM-generated, 200~400 자
    MoodSuggestion string
    LuckyColor    string
    LuckyNumber   int             // 1-9 만
    Disclaimer    string
    GeneratedAt   time.Time
    LLMModelUsed  string
}

type SajuData struct {
    YearPillar   Pillar    // 년주: 천간+지지
    MonthPillar  Pillar
    DayPillar    Pillar    // 일주 = 당일 주도 기운
    HourPillar   Pillar    // 시주 (생시 있을 때만)
    FiveElements map[string]int  // {"木":2, "火":1, ...}
    Dominant     string    // "木" 우세
}

type Pillar struct {
    Heaven string  // 천간: 甲乙丙丁戊己庚辛壬癸
    Earth  string  // 지지: 子丑寅卯辰巳午未申酉戌亥
    KorName string // "갑자" 등
}

type Biorhythm struct {
    PhysicalPct      float64   // -100 ~ +100
    EmotionalPct     float64
    IntellectualPct  float64
    Highlights       []string  // ["감성 고점 주의"]
}
```

### 6.3 Saju 계산 핵심

```go
// 천간 10개, 지지 12개 순환
var stems = []string{"甲","乙","丙","丁","戊","己","庚","辛","壬","癸"}
var branches = []string{"子","丑","寅","卯","辰","巳","午","未","申","酉","戌","亥"}

// 년주: 60갑자 기준일(1984-02-02 갑자년 시작) 로부터 계산
func (c *SajuCalculator) YearPillar(solarYear int) Pillar {
    // 입춘 보정: 입춘 전이면 전년도 년주
    // ...
}

// 일주: 만세력 기준일로부터 경과일 % 60
func (c *SajuCalculator) DayPillar(date time.Time) Pillar {
    days := int(date.Sub(referenceJiaZiDay).Hours() / 24)
    idx := ((days % 60) + 60) % 60
    return Pillar{
        Heaven: stems[idx%10],
        Earth:  branches[idx%12],
        KorName: sixtyJiaZiKor[idx],
    }
}
```

### 6.4 Biorhythm 수식

```go
func (c *BiorhythmCalculator) Calculate(birth, date time.Time) Biorhythm {
    days := int(date.Sub(birth).Hours() / 24)
    return Biorhythm{
        PhysicalPct:     math.Sin(2*math.Pi*float64(days)/23) * 100,
        EmotionalPct:    math.Sin(2*math.Pi*float64(days)/28) * 100,
        IntellectualPct: math.Sin(2*math.Pi*float64(days)/33) * 100,
    }
}
```

### 6.5 LLM System Prompt (한국어 ver)

```
당신은 따뜻하고 신중한 운세 안내자입니다. 다음 제약을 반드시 지키세요:

1. 엔터테인먼트 목적: 확정적 예언 금지, "~할 수 있어요", "~하면 좋을 것 같아요" 어조 사용
2. 금지: 투자·주식·복권 추천, 의학적 진단/치료 언급, "반드시", "100%", "절대"
3. 부정적 표현 회피: "불행", "저주", "큰일", "병", "질병" 사용 금지
4. 길이: 200~400자, JSON 형식 출력
5. 개인화 컨텍스트: {recent_mood_hint} — 사용자 최근 감정 고려
6. 톤: {tone_guidance}

사용자 정보:
- 일주 (오늘의 주도 기운): {day_pillar}
- 오행 우세: {dominant}
- 바이오리듬: 신체 {phys_pct}%, 감성 {emo_pct}%, 지성 {int_pct}%

JSON 스키마:
{
  "narrative": "200~400자",
  "mood_suggestion": "10~30자 짧은 조언",
  "lucky_color": "색상명 (선택)",
  "lucky_number": 1-9 사이 숫자 (선택)
}
```

### 6.6 Guard 알고리즘

```go
var blockedKeywords = []string{
    "투자", "주식", "코인", "복권", "로또",
    "질병", "병", "치료", "처방", "진단",
    "반드시", "100%", "절대", "확실히",
    "불행", "저주", "큰일",
}

var lottoNumberPattern = regexp.MustCompile(`(\d{1,2}[,\s]+){5,}\d{1,2}`)

func (g *Guard) Sanitize(text string) (clean string, blocked []string) {
    // 1. 키워드 포함 문장 스트립
    // 2. 로또 번호 패턴 제거
    // 3. 결과 너무 짧아지면 fallback template 반환
}
```

### 6.7 Cache

Key: `fortune:{user_id}:{YYYYMMDD}`. Storage: MEMORY-001 `facts` (session_id="fortune") 또는 인메모리 map (주기적 cleanup). TTL 24h.

### 6.8 Fallback Template

3회 재생성 실패 시:

```
오늘은 평온한 하루가 될 것 같아요. 자신의 감정에 귀 기울이며, 주변 사람들과 따뜻한 시간을 보내보세요. 감성 바이오리듬이 {emo_pct}%로 {상승|하락}세이니, {...}
```

### 6.9 TDD 진입 순서

1. RED: `TestOptInDefault_Disabled` — AC-FORTUNE-001
2. RED: `TestBirthDateMissing` — AC-FORTUNE-002
3. RED: `TestSaju_Deterministic` — AC-FORTUNE-003
4. RED: `TestBiorhythm_Formula` — AC-FORTUNE-004
5. RED: `TestDisclaimer_AlwaysPresent` — AC-FORTUNE-005
6. RED: `TestGuard_StripInvestment` — AC-FORTUNE-006
7. RED: `TestGuard_StripLottoNumbers` — AC-FORTUNE-007
8. RED: `TestCache_SameDayReturnsSame` — AC-FORTUNE-008
9. RED: `TestTone_LowValenceSupportive` — AC-FORTUNE-009
10. RED: `TestStyle_BiorhythmOnly` — AC-FORTUNE-010
11. GREEN → REFACTOR

### 6.10 라이브러리 결정

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| 양력↔음력 | `github.com/wjk/koreanlunarcalendar` (포팅) 또는 자체 구현 | 만세력 1391-2050 |
| LLM 호출 | ADAPTER-001 | 내부 |
| 정규식 | stdlib `regexp` | |
| JSON | `encoding/json` | |

### 6.11 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| **T**ested | 85%+, Saju 결정론 1000 random 검증, guard 키워드 100+ 코퍼스 |
| **R**eadable | generator/saju/biorhythm/guards 파일 분리 |
| **U**nified | JSON output schema 엄격, disclaimer 템플릿 i18n |
| **S**ecured | opt-in default off, 키워드 guard, 로또 번호 차단, LLM 재시도 3회 cap |
| **T**rackable | 모든 Generate zap 로그 `{user_id, date, model, guard_retries, cache_hit}` |

---

## 7. 의존성

| 타입 | 대상 | 설명 |
|-----|------|-----|
| 선행 SPEC | IDENTITY-001 | Person entity birth_date attribute |
| 선행 SPEC | ADAPTER-001 | LLM 호출 |
| 선행 SPEC | INSIGHTS-001 | 최근 감정 트렌드 |
| 선행 SPEC | MEMORY-001 | cache 영속 |
| 후속 SPEC | BRIEFING-001 | 아침 브리핑 통합 |
| 외부 | `korean-lunar-calendar` or 자체 구현 | 만세력 |
| 외부 | Go 1.22+ | math/big optional |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 사용자가 운세 내용을 진지하게 받아들여 중요 결정에 사용 | 중 | 고 | 모든 응답에 disclaimer, opt-in 기본 off, 의료/금융 키워드 차단 |
| R2 | LLM이 guard를 우회하여 예언적 표현 생성 | 중 | 중 | 3회 재시도 cap, 실패 시 안전 템플릿 fallback |
| R3 | 만세력 계산 오류 (음력 변환) | 낮 | 중 | 주요 날짜 (설날, 추석 등) 10년치 goldenfile 검증 |
| R4 | 생년월일이 개인정보 → 보안 | 중 | 고 | IDENTITY-001의 E2E 원칙 준수, cache도 사용자별 격리 |
| R5 | "lucky number" 를 복권에 사용 | 중 | 중 | 1-9 단일 숫자로 제한, 6자리 시퀀스 차단 |
| R6 | 우울/자살 리스크 사용자에게 negative 운세 | 낮 | 치명적 | INSIGHTS 감정 트렌드 확인, 부정적 키워드 전체 차단, 필요 시 전문가 상담 권고 문구 |
| R7 | 문화적 appropriation (서양 별자리 + 한국 사주 혼용) | 낮 | 낮 | 사용자 style 선택 존중 |

---

## 9. 참고

### 9.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-IDENTITY-001/spec.md` — Person entity attributes
- `.moai/specs/SPEC-GOOSE-BRIEFING-001/spec.md` — consumer
- `.moai/specs/SPEC-GOOSE-INSIGHTS-001/spec.md` — 감정 트렌드
- `.moai/project/adaptation.md` §6.3 Season Mode

### 9.2 외부 참조

- 만세력 알고리즘: https://ko.wikipedia.org/wiki/만세력
- 사주팔자 기본: https://ko.wikipedia.org/wiki/사주팔자
- Biorhythm 이론: https://en.wikipedia.org/wiki/Biorhythm (pseudoscience 명시)
- 12 별자리 날짜: https://en.wikipedia.org/wiki/Astrological_sign

### 9.3 부속 문서

- `./research.md` — Saju 알고리즘 상세, LLM prompt 실험 결과, guard keyword 코퍼스

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **연간 운세·평생 사주 풀이를 포함하지 않는다**. 깊이 있는 상담은 사용자에게 전문가 권장.
- 본 SPEC은 **투자·주식·복권 번호 추천을 명시적으로 금지한다** (guard로 차단).
- 본 SPEC은 **의학적 진단·치료 조언을 포함하지 않는다** (guard 차단).
- 본 SPEC은 **관상·수상·풍수 등 이미지 기반 점술을 포함하지 않는다**.
- 본 SPEC은 **궁합·커플 운세를 포함하지 않는다** (별도 SPEC).
- 본 SPEC은 **타로 카드 78장 풀 덱 해석을 포함하지 않는다** (simplified 22장만 optional).
- 본 SPEC은 **시간대별 운세 (오전/오후/저녁 분리) 를 포함하지 않는다** (1일 1회).
- 본 SPEC은 **다른 사용자와의 비교·랭킹을 포함하지 않는다**.
- 본 SPEC은 **유료 상담 추천 링크를 포함하지 않는다**.
- 본 SPEC은 **자동 생성된 운세를 MEMORY-001의 facts 테이블에 "사실"로 기록하지 않는다** (opinion/generated 로 표시).

---

**End of SPEC-GOOSE-FORTUNE-001**
