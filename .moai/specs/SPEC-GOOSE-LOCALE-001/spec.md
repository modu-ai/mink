---
id: SPEC-GOOSE-LOCALE-001
version: 0.1.0
status: planned
created_at: 2026-04-22
updated_at: 2026-04-22
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 소(S)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-LOCALE-001 — Locale Detection + Cultural Context Injection

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성. v5.0 ROADMAP Phase 6 확장에 따른 Localization 4 SPEC 시리즈의 **기반층**. 사용자 최종 지시(2026-04-22): "한국뿐만 아니라 설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게 각 현지화된 스킬들을 추가, hermes-agent 정도의 다국어" 반영. | manager-spec |

---

## 1. 개요 (Overview)

사용자 최종 지시(2026-04-22):

> "한국뿐만 아니라, 설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게 각 현지화된 스킬들을 추가하도록 하자. hermes-agent 정도의 다국어를 제공하자."

본 SPEC은 GOOSE의 **현지화 기반층(Locale Foundation)**을 정의한다. OS 수준에서 사용자의 `country`/`language`/`timezone`/`currency`/`measurement_system`/`calendar_system`을 **감지 + 확인 + 저장**하고, LLM 프롬프트에 **문화권 컨텍스트(CulturalContext)**를 자동 주입한다. 본 SPEC이 통과하면:

- `internal/locale/` 패키지가 OS locale을 감지하고, IP geolocation으로 보조 검증하며,
- `LocaleContext`(ISO 3166-1 country + BCP 47 language + IANA timezone + ISO 4217 currency)를 `CONFIG-001` 저장소에 영속화하고,
- `CulturalContext`(formality_default, honorific_system, measurement_system, calendar_system)를 QueryEngine system prompt에 매 iteration 삽입하며,
- 사용자 override(ONBOARDING-001에서 수집)를 최우선 반영하고,
- 다국적(primary + secondary language) 사용자를 지원한다.

본 SPEC은 **UI 텍스트 번역은 다루지 않는다**(I18N-001). **국가별 Skill 번들은 다루지 않는다**(REGION-SKILLS-001). **온보딩 UX는 다루지 않는다**(ONBOARDING-001).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **Phase 6 의존 루트**: I18N-001(UI 번역), REGION-SKILLS-001(국가별 Skill), ONBOARDING-001(설치 플로우) 모두 `LocaleContext`를 입력으로 소비한다. 이 기반 없이는 후속 3 SPEC 동작 불가.
- **Hermes 수준 다국어**: `hermes-llm.md` §2는 15+ LLM 프로바이더 매트릭스를 보인다. GOOSE는 Hermes가 달성한 "프로바이더 다양성" 수준을 **사용자 다양성**(국가/언어/문화)에서 재현해야 한다.
- **branding.md §3 다국어 페르소나**: 5종 페르소나(집사/친구/선생님/비서/동반자)가 언어별로 다른 존칭 체계(한국 존댓말, 일본 敬語, 중국 敬, 영어 first-name)로 재구성된다. 이 매핑은 `CulturalContext`에 인코딩되어 adapter가 소비한다.
- **adaptation.md §4 Cultural Context**: 4개 주요 언어(en/ko/ja/zh) + 명절/기념일 자동 감지를 명시. 본 SPEC은 그 요구를 **런타임 주입 가능한 구조화 데이터**로 확정한다.
- **법적 제약 분기**: GDPR(EU) / PIPA(한국) / Federal Law 152-FZ(러시아) / ICP(중국)는 country에 따라 다르게 적용된다. `LocaleContext.country`는 PRIVACY 로직의 정식 입력이다.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code `src/localization/`**: OS locale detection 패턴 참조. Tauri `locale` crate로 대체.
- **Hermes `agent/locale/`**: LLM 프롬프트 주입 패턴 참조(직접 포팅 아님).
- **MoAI-ADK `.moai/config/sections/language.yaml`**: `conversation_language` + `agent_prompt_language` 분리 원칙 계승. GOOSE에서는 `LocaleContext.primary_language` + `LocaleContext.secondary_language`로 확장.

### 2.3 범위 경계

- **IN**: OS locale detection(3 OS), IP geolocation fallback, `LocaleContext`/`CulturalContext` 타입, LLM prompt 주입 로직, 사용자 override 지원, 다국적(primary+secondary) 사용자 지원, CONFIG-001 스키마 확장.
- **OUT**: UI 번역 리소스(I18N-001), 국가별 Skill 번들링(REGION-SKILLS-001), 온보딩 UX 화면(ONBOARDING-001), GDPR 준수 자체 구현(본 SPEC은 flag만 노출), 공휴일 DB(SCHEDULER-001 + REGION-SKILLS-001 공동).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/locale/` Go 패키지 신설:
   - `detector.go` — OS locale detection (Linux/macOS/Windows)
   - `context.go` — `LocaleContext` 타입 + 직렬화
   - `cultural.go` — `CulturalContext` 타입 + country → cultural 매핑 테이블
   - `prompts.go` — LLM system prompt 주입 빌더
   - `provider.go` — `LocaleDetector` 인터페이스 + 기본 구현
   - `geo.go` — IP geolocation fallback (MaxMind GeoLite2 offline DB 우선, ipapi.co HTTP fallback)
2. OS별 감지 구현:
   - Linux: `LANG`/`LC_ALL`/`LC_MESSAGES` 환경변수 + `/etc/locale.conf`
   - macOS: `defaults read -g AppleLocale` + `AppleLanguages[0]`
   - Windows: `GetUserDefaultLocaleName` Win32 API (CGO 또는 `golang.org/x/sys/windows`)
3. IP geolocation fallback:
   - 우선순위 1: 번들 MaxMind GeoLite2-Country.mmdb (오프라인, ~4MB)
   - 우선순위 2: ipapi.co HTTPS (네트워크 필요, 일일 1000 req free tier)
   - 우선순위 3: 기본값(en-US) + warn 로그
4. `LocaleContext` 구조체:
   - `country` — ISO 3166-1 alpha-2 (KR, JP, CN, US, DE, ...)
   - `primary_language` — BCP 47 (ko-KR, ja-JP, zh-CN, en-US, ...)
   - `secondary_language` — optional BCP 47 (한국 거주 미국인 등)
   - `timezone` — IANA (Asia/Seoul, America/New_York, ...)
   - `currency` — ISO 4217 (KRW, JPY, CNY, USD, EUR, ...)
   - `measurement_system` — `metric` | `imperial`
   - `calendar_system` — `gregorian` | `hijri` | `chinese_lunar` | `hebrew` | `thai_buddhist`
   - `detected_method` — `os`/`ip`/`user_override`/`default`
5. `CulturalContext` 구조체 + country → cultural 매핑 테이블:
   - `formality_default` — `formal` | `casual`
   - `honorific_system` — `korean_jondaetmal` | `japanese_keigo` | `chinese_jing` | `vietnamese_anh_em` | `arabic_formal_familiar` | `none`
   - `name_order` — `given_first` | `family_first`
   - `address_format` — `western` | `east_asian` | `postal_code_prefix`
   - `weekend_days` — `[Sat, Sun]` | `[Fri, Sat]` | `[Sun]` 등
   - `first_day_of_week` — `Sunday` | `Monday` | `Saturday`
   - `legal_flags` — `gdpr`/`ccpa`/`pipa`/`pipl`/`lgpd`/`fz152`
6. LLM prompt 주입 빌더:
   - `BuildSystemPromptAddendum(loc LocaleContext, cul CulturalContext) string`
   - 예시: "User is in KR (Asia/Seoul), speaks ko-KR, uses Korean jondaetmal honorifics. Use 존댓말 by default. Measurement: metric. Legal: PIPA applies."
7. 사용자 override API:
   - `DetectWithOverride(override *LocaleContext) (*LocaleContext, error)` — ONBOARDING-001이 수집한 값이 있으면 OS/IP 감지 결과를 override.
8. CONFIG-001 스키마 확장:
   - `locale:` 섹션을 `~/.goose/config.yaml`에 신설 (REQ-LC-011 참조).
9. 다국적 사용자 지원:
   - `primary_language`(일상 대화) + `secondary_language`(기술 용어 혹은 이중언어) 모두 system prompt에 포함.

### 3.2 OUT OF SCOPE

- UI 번역 문자열, Pluralization, RTL 레이아웃 — I18N-001.
- 국가별 Skill 자동 활성화 — REGION-SKILLS-001.
- 8단계 온보딩 화면 구현 — ONBOARDING-001.
- 공휴일 데이터베이스 — SCHEDULER-001 + REGION-SKILLS-001 공동(REGION-SKILLS-001이 `rickar/cal/v2` 기반으로 캘린더 제공).
- IP geolocation 유료 서비스(MaxMind GeoIP2 commercial, ipstack paid) — 오픈소스 DB만 사용.
- Accept-Language HTTP 헤더 parsing — I18N-001의 CONTENT negotiation 책임.
- 문화권별 UI 페르소나 실제 말투(존댓말/敬語 텍스트) — branding.md의 정의를 본 SPEC이 메타데이터로만 노출하고, 실제 말투 치환은 ADAPTER-001 또는 별도 persona engine.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (상시 불변)

**REQ-LC-001 [Ubiquitous]** — The `LocaleContext` **shall** include the following fields at minimum: `country` (ISO 3166-1 alpha-2), `primary_language` (BCP 47), `timezone` (IANA), `currency` (ISO 4217), `measurement_system`, `calendar_system`, and `detected_method`.

**REQ-LC-002 [Ubiquitous]** — The `LocaleDetector` **shall** return a `LocaleContext` with `detected_method != "default"` on at least 95% of installations where OS locale API is available; the `"default"` fallback **shall** only trigger when every detection path(OS, IP, config file) failed.

**REQ-LC-003 [Ubiquitous]** — The `CulturalContext` for a given country **shall** be deterministic; identical `country` inputs **shall** always produce identical `CulturalContext` output.

**REQ-LC-004 [Ubiquitous]** — The locale package **shall not** perform HTTP requests during `Detect()` unless the MaxMind offline DB lookup fails first; network calls **shall** respect a 2-second timeout and degrade gracefully.

### 4.2 Event-Driven (이벤트 기반)

**REQ-LC-005 [Event-Driven]** — **When** `Detect(ctx)` is invoked, the detector **shall** (a) read OS environment variables/APIs, (b) parse them into `LocaleContext`, (c) if any core field is missing, fall back to IP geolocation, (d) if IP fails, emit default en-US with warn log, and (e) return the resolved context.

**REQ-LC-006 [Event-Driven]** — **When** a user override is stored in `CONFIG-001` (`locale.override` section), the `Detect()` function **shall** return the override values verbatim with `detected_method = "user_override"`, bypassing OS and IP detection entirely.

**REQ-LC-007 [Event-Driven]** — **When** `BuildSystemPromptAddendum(loc, cul)` is called, the function **shall** produce a deterministic UTF-8 string ≤ 400 tokens containing country, language(s), timezone, honorific preference, and applicable legal flags.

**REQ-LC-008 [Event-Driven]** — **When** the user's OS locale and IP geolocation country disagree (e.g., OS=ko_KR but IP=US), the detector **shall** prefer OS and record both values in `LocaleContext.conflict` for ONBOARDING-001 to resolve.

### 4.3 State-Driven (상태 기반)

**REQ-LC-009 [State-Driven]** — **While** `LocaleContext.secondary_language` is set, `BuildSystemPromptAddendum` **shall** include both primary and secondary languages in the prompt ("User speaks ko-KR primary, en-US secondary; code-switching is natural").

**REQ-LC-010 [State-Driven]** — **While** the MaxMind GeoLite2 DB file is absent or older than 90 days, the detector **shall** log a WARN entry on startup suggesting `goose locale update-db` (CLI-001 responsibility) but **shall not** fail `Detect()`.

**REQ-LC-011 [State-Driven]** — **While** `CONFIG-001` is loaded, the `locale:` section **shall** be a typed sub-struct containing: `override` (nullable LocaleContext), `geolocation_enabled` (bool, default true), `geoip_db_path` (string, optional).

### 4.4 Unwanted Behavior (방지)

**REQ-LC-012 [Unwanted]** — The locale package **shall not** transmit the user's IP address or geolocation result to any third party other than the configured ipapi endpoint; telemetry OFF by default.

**REQ-LC-013 [Unwanted]** — **If** the OS locale environment variable contains injected shell syntax (e.g., `LANG="; rm -rf"`), **then** the parser **shall** reject it, log a security event, and fall back to default.

**REQ-LC-014 [Unwanted]** — The detector **shall not** mutate process-level environment variables (`os.Setenv`); all reads are pure.

**REQ-LC-015 [Unwanted]** — **If** the user's `country` matches `CN` (China) and `geolocation_enabled = true`, **then** the detector **shall** skip ipapi.co HTTPS fallback(GFW 차단 가능성) and rely on OS + MaxMind only.

### 4.5 Optional (선택적)

**REQ-LC-016 [Optional]** — **Where** `tz-mapped timezone` is ambiguous (e.g., `Asia/Shanghai` covers mainland China and parts of Xinjiang), the detector **may** expose `timezone_alternatives` for ONBOARDING-001 disambiguation.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-LC-001 — OS 감지 기본 경로 (Linux)**
- **Given** `LANG=ko_KR.UTF-8`, `TZ=Asia/Seoul` 환경변수
- **When** `Detect(ctx)`
- **Then** `LocaleContext{country:"KR", primary_language:"ko-KR", timezone:"Asia/Seoul", currency:"KRW", measurement_system:"metric", calendar_system:"gregorian", detected_method:"os"}` 반환

**AC-LC-002 — OS 감지 기본 경로 (macOS)**
- **Given** `defaults read -g AppleLocale` = `"ja_JP@calendar=gregorian"`, `AppleLanguages[0]` = `"ja"`
- **When** `Detect(ctx)`
- **Then** `primary_language="ja-JP"`, `country="JP"`, `currency="JPY"`, `honorific_system="japanese_keigo"`

**AC-LC-003 — IP fallback**
- **Given** OS locale 환경변수 모두 비어있음(서버 환경), MaxMind DB에서 IP → country=`VN`
- **When** `Detect(ctx)`
- **Then** `country="VN"`, `primary_language="vi-VN"`, `detected_method="ip"`

**AC-LC-004 — User override 최우선**
- **Given** OS=ko_KR 감지됨, 사용자가 `config.yaml`에 `locale.override.country: JP`, `primary_language: ja-JP` 설정
- **When** `Detect(ctx)`
- **Then** `country="JP"`, `primary_language="ja-JP"`, `detected_method="user_override"`; OS 값은 무시

**AC-LC-005 — 다국적 사용자(primary + secondary)**
- **Given** `primary_language="ko-KR"`, `secondary_language="en-US"` override
- **When** `BuildSystemPromptAddendum(loc, cul)`
- **Then** 결과 문자열에 `"primary: ko-KR"`와 `"secondary: en-US"` 모두 포함, `"code-switching is natural"` 지시문 존재

**AC-LC-006 — Cultural context 한국**
- **Given** `country="KR"`
- **When** `ResolveCulturalContext("KR")`
- **Then** `{formality_default:"formal", honorific_system:"korean_jondaetmal", name_order:"family_first", weekend_days:["Sat","Sun"], first_day_of_week:"Monday", legal_flags:["pipa"]}`

**AC-LC-007 — Cultural context 사우디**
- **Given** `country="SA"`
- **When** `ResolveCulturalContext("SA")`
- **Then** `{calendar_system:"hijri", weekend_days:["Fri","Sat"], honorific_system:"arabic_formal_familiar"}`

**AC-LC-008 — OS vs IP 충돌 보존**
- **Given** OS=`ko_KR`, IP geolocation=`US`
- **When** `Detect(ctx)`
- **Then** `country="KR"` (OS 우선), `LocaleContext.conflict={os:"KR", ip:"US"}`, ONBOARDING-001이 이 필드를 읽어 사용자에게 확인 다이얼로그 표시

**AC-LC-009 — 중국에서 ipapi.co 스킵**
- **Given** OS 감지 결과 `country="CN"`, `geolocation_enabled=true`
- **When** OS 결과에 timezone 누락 → ipapi fallback이 호출되려 할 때
- **Then** ipapi HTTP 호출이 발생하지 않고, MaxMind DB만 조회되며, 실패 시 default로 폴백

**AC-LC-010 — 환경변수 injection 거부**
- **Given** `LANG="en_US.UTF-8; curl evil.com"`
- **When** `Detect(ctx)`
- **Then** 파서가 `;` 발견 후 reject, security event 로그, 기본값 en-US로 폴백

**AC-LC-011 — LLM prompt addendum 길이 제한**
- **Given** 한국 + primary/secondary language 포함 최대 케이스
- **When** `BuildSystemPromptAddendum(loc, cul)`
- **Then** 결과 UTF-8 문자열의 토큰 수 ≤ 400 (GPT-4 tokenizer 기준 추정)

**AC-LC-012 — MaxMind DB 노후 경고**
- **Given** `geoip_db_path`가 가리키는 .mmdb 파일의 mtime이 91일 전
- **When** 로더 실행
- **Then** WARN 로그 1건 (`locale.geoip.db.stale`), `Detect()`는 정상 동작

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/locale/
├── detector.go       # Detect(ctx), LocaleDetector interface
├── context.go        # LocaleContext, CulturalContext structs + JSON/YAML tags
├── cultural.go       # country → CulturalContext 매핑 테이블 (정적)
├── prompts.go        # BuildSystemPromptAddendum
├── provider.go       # OSProvider, IPProvider, OverrideProvider
├── geo.go            # MaxMind GeoLite2 reader + ipapi.co HTTP fallback
├── os_linux.go       # //go:build linux
├── os_darwin.go      # //go:build darwin
├── os_windows.go     # //go:build windows
└── *_test.go
```

### 6.2 핵심 Go 타입 시그니처

```go
// LocaleContext — 사용자의 지리/언어/시간 컨텍스트.
// 모든 필드는 감지 시점에 결정되며 이후 CONFIG-001을 거쳐 영속화된다.
type LocaleContext struct {
    Country           string            `yaml:"country" json:"country"`              // ISO 3166-1 alpha-2
    PrimaryLanguage   string            `yaml:"primary_language" json:"primary_language"` // BCP 47
    SecondaryLanguage string            `yaml:"secondary_language,omitempty" json:"secondary_language,omitempty"`
    Timezone          string            `yaml:"timezone" json:"timezone"`            // IANA
    Currency          string            `yaml:"currency" json:"currency"`            // ISO 4217
    MeasurementSystem string            `yaml:"measurement_system" json:"measurement_system"` // "metric"|"imperial"
    CalendarSystem    string            `yaml:"calendar_system" json:"calendar_system"`       // "gregorian"|"hijri"|...
    DetectedMethod    string            `yaml:"detected_method" json:"detected_method"`       // "os"|"ip"|"user_override"|"default"
    Conflict          *LocaleConflict   `yaml:"conflict,omitempty" json:"conflict,omitempty"`
}

// LocaleConflict — OS와 IP 감지가 불일치할 때 기록되는 진단 정보.
type LocaleConflict struct {
    OSCountry string `yaml:"os" json:"os"`
    IPCountry string `yaml:"ip" json:"ip"`
}

// CulturalContext — country 기반 정적 매핑으로 결정되는 문화권 속성.
type CulturalContext struct {
    FormalityDefault FormalityMode `yaml:"formality_default" json:"formality_default"`
    HonorificSystem  string        `yaml:"honorific_system" json:"honorific_system"`  // "korean_jondaetmal"|...
    NameOrder        string        `yaml:"name_order" json:"name_order"`              // "given_first"|"family_first"
    AddressFormat    string        `yaml:"address_format" json:"address_format"`      // "western"|"east_asian"|...
    WeekendDays      []string      `yaml:"weekend_days" json:"weekend_days"`
    FirstDayOfWeek   string        `yaml:"first_day_of_week" json:"first_day_of_week"`
    LegalFlags       []string      `yaml:"legal_flags" json:"legal_flags"`            // "gdpr"|"pipa"|...
}

// FormalityMode — 말투의 기본 격식 수준.
type FormalityMode string
const (
    FormalityFormal FormalityMode = "formal"
    FormalityCasual FormalityMode = "casual"
)

// LocaleDetector — OS/IP/override 소스를 조합해 LocaleContext를 반환.
type LocaleDetector interface {
    Detect(ctx context.Context) (*LocaleContext, error)
}

// IPGeolocator — MaxMind 우선, ipapi.co fallback.
type IPGeolocator interface {
    ResolveCountry(ctx context.Context, ip net.IP) (country string, err error)
}

// 공개 함수:
func NewDetector(cfg LocaleConfig, logger *zap.Logger) LocaleDetector
func ResolveCulturalContext(country string) CulturalContext
func BuildSystemPromptAddendum(loc LocaleContext, cul CulturalContext) string
```

### 6.3 Country → CulturalContext 매핑 (초안 20+ 국가)

| Country | Lang | Honorific | Name Order | Calendar | Weekend | Legal |
|---------|------|-----------|------------|----------|---------|-------|
| KR | ko-KR | korean_jondaetmal | family_first | gregorian | Sat,Sun | pipa |
| JP | ja-JP | japanese_keigo | family_first | gregorian | Sat,Sun | appi |
| CN | zh-CN | chinese_jing | family_first | gregorian(+chinese_lunar) | Sat,Sun | pipl |
| US | en-US | none | given_first | gregorian | Sat,Sun | ccpa |
| DE | de-DE | german_sie_du | given_first | gregorian | Sat,Sun | gdpr |
| FR | fr-FR | french_tu_vous | given_first | gregorian | Sat,Sun | gdpr |
| GB | en-GB | none | given_first | gregorian | Sat,Sun | ukgdpr |
| BR | pt-BR | portuguese_senhor | given_first | gregorian | Sat,Sun | lgpd |
| RU | ru-RU | russian_vy | given_first | gregorian | Sat,Sun | fz152 |
| SA | ar-SA | arabic_formal_familiar | family_first | hijri | Fri,Sat | sa_pdpl |
| AE | ar-AE | arabic_formal_familiar | family_first | gregorian(+hijri) | Sat,Sun | uae_pdpl |
| IN | hi-IN | hindi_aap_tum | family_first | gregorian(+hindu) | Sun | in_dpdp |
| VN | vi-VN | vietnamese_anh_em | family_first | gregorian | Sat,Sun | vn_pdpd |
| ID | id-ID | none | given_first | gregorian | Sat,Sun | id_pdp |
| TH | th-TH | thai_khun | given_first | thai_buddhist | Sat,Sun | th_pdpa |
| TR | tr-TR | turkish_siz | given_first | gregorian | Sat,Sun | kvkk |
| MX | es-MX | spanish_usted_tu | given_first | gregorian | Sat,Sun | mx_lfpdppp |
| ES | es-ES | spanish_usted_tu | given_first | gregorian | Sat,Sun | gdpr |
| IT | it-IT | italian_lei_tu | given_first | gregorian | Sat,Sun | gdpr |
| PL | pl-PL | polish_pan_pani | given_first | gregorian | Sat,Sun | gdpr |

완전 매핑은 `cultural.go`의 `countryToCultural` 정적 맵으로 인코딩. 누락 국가는 en-US + `none` honorific + `gdpr` 플래그 없음으로 폴백.

### 6.4 System Prompt Addendum 템플릿

```
# Locale Context
- Country: {{country}} ({{country_full_name}})
- Languages: primary={{primary_language}}{{#if secondary}}, secondary={{secondary_language}} — code-switching is natural{{/if}}
- Timezone: {{timezone}} (currently {{current_local_time}})
- Currency: {{currency}}
- Measurement: {{measurement_system}}
- Calendar: {{calendar_system}}

# Cultural Context
- Formality: {{formality_default}} by default
- Honorific system: {{honorific_system}}
- Name order: {{name_order}}
- Weekend: {{weekend_days}}
- Legal framework: {{legal_flags}}

Apply these conventions unless the user's conversational style overrides them.
```

렌더링 토큰 수 ≤ 400. 각 필드는 짧은 키워드만 사용.

### 6.5 라이브러리 결정

| 용도 | 라이브러리 | 버전 | 결정 근거 |
|------|----------|-----|---------|
| BCP 47 파싱 | `golang.org/x/text/language` | v0.14+ | 표준 라이브러리급, `language.Parse` 검증 견고 |
| IP geolocation offline | `github.com/oschwald/maxminddb-golang` | v2.x | MaxMind 공식, 메모리 매핑 빠름 |
| IP geolocation HTTP fallback | stdlib `net/http` | — | 경량, ipapi.co 전용 |
| IANA timezone 검증 | stdlib `time.LoadLocation` | — | 표준 |
| YAML | `gopkg.in/yaml.v3` | — | CONFIG-001 공유 |
| 로깅 | `go.uber.org/zap` | — | CORE-001 공유 |
| Windows locale API | `golang.org/x/sys/windows` | — | CGO 회피 |

**dariubs/locales 및 unicode-org/icu 언급 재평가**:
- `dariubs/locales`: 활성 유지보수 불확실. 대신 `golang.org/x/text/language` 채택.
- `unicode-org/icu`: C/C++ 라이브러리. GOOSE는 I18N-001에서 Go 네이티브 `go-i18n/v2`를 선호하므로 본 SPEC은 ICU 의존 미포함.

### 6.6 TDD 진입 순서

1. **RED #1** — `TestResolveCulturalContext_KR` → AC-LC-006
2. **RED #2** — `TestResolveCulturalContext_SA_Hijri` → AC-LC-007
3. **RED #3** — `TestDetect_Linux_OSEnv` → AC-LC-001 (fake OS provider)
4. **RED #4** — `TestDetect_UserOverride_Wins` → AC-LC-004
5. **RED #5** — `TestDetect_OSvsIP_ConflictRecorded` → AC-LC-008
6. **RED #6** — `TestDetect_EnvInjection_Rejected` → AC-LC-010
7. **RED #7** — `TestBuildSystemPromptAddendum_Bilingual` → AC-LC-005
8. **RED #8** — `TestDetect_CN_SkipsIPAPI` → AC-LC-009
9. **RED #9** — `TestDetect_IPFallback_VN` → AC-LC-003 (mock MaxMind reader)
10. **GREEN** — 최소 구현
11. **REFACTOR** — provider 체인(`OSProvider → IPProvider → DefaultProvider`) 추상화

### 6.7 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | 20+ country 테이블 테스트(AC-LC-006/007 확장), 3 OS matrix CI, MaxMind mock, 커버리지 90%+ |
| **R**eadable | country 매핑 테이블을 별도 파일(`cultural.go`)로 분리, 한국어 주석 + 영어 식별자 |
| **U**nified | `golangci-lint` + CONFIG-001 동일 yaml struct tag 규칙 |
| **S**ecured | env var injection 거부(REQ-LC-013), 중국 geolocation HTTP 차단(REQ-LC-015), telemetry OFF 기본 |
| **T**rackable | `detected_method` 필드 + zap 구조화 로그(`locale.detect.completed` with source/country/lang) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, context 루트 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `locale:` 섹션 스키마 확장 대상 |
| 후속 SPEC | SPEC-GOOSE-I18N-001 | LocaleContext.primary_language 소비 |
| 후속 SPEC | SPEC-GOOSE-REGION-SKILLS-001 | LocaleContext.country 기반 Skill 활성화 |
| 후속 SPEC | SPEC-GOOSE-ONBOARDING-001 | Detect() 결과를 초기 화면에 표시 + override 수집 |
| 후속 SPEC | SPEC-GOOSE-SCHEDULER-001 | LocaleContext.timezone 소비, REGION-SKILLS-001 통해 공휴일 DB 선택 |
| 외부 | Go 1.22+ | `//go:build` tag |
| 외부 | `golang.org/x/text/language` v0.14+ | BCP 47 파싱 |
| 외부 | `github.com/oschwald/maxminddb-golang` v2.x | GeoLite2 DB reader |
| 외부 | MaxMind GeoLite2-Country.mmdb | ~4MB offline DB, 번들 또는 CLI로 다운로드 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | MaxMind GeoLite2 라이센스(CC BY-SA 4.0) 변경 | 낮 | 중 | 대안 DB(`ipapi.co` offline, `db-ip.com`) 후보 리스트 유지 |
| R2 | Country → CulturalContext 매핑이 문화적 편향 내포 | 중 | 중 | 매핑 테이블 공개 + PR 리뷰 필수, REGION-SKILLS-001에서 사용자가 덮어쓸 수 있게 설계 |
| R3 | IP geolocation이 VPN/Tor 사용자에게 부정확 | 고 | 낮 | OS 우선(REQ-LC-005), 충돌 시 사용자 확인(ONBOARDING-001 AC) |
| R4 | Windows `GetUserDefaultLocaleName` CGO 문제 | 중 | 중 | `golang.org/x/sys/windows` 순수 Go로 호출 |
| R5 | `CulturalContext`에 담은 legal_flags가 법률 변경에 뒤처짐 | 중 | 고 | 플래그는 힌트(hint)이며 실제 준수 로직은 각 SPEC(예: JOURNAL-001 PIPA)에서 최종 결정 |
| R6 | 이슬람 달력(hijri)/중국 음력 혼용 사용자(예: 말레이시아) | 중 | 낮 | `CalendarSystem`을 `gregorian+hijri` 복합 문자열 허용, REGION-SKILLS-001이 세부 처리 |
| R7 | GeoLite2 DB의 90일 노후화 미감지 | 낮 | 중 | `goose locale update-db` CLI 명령 + 시작 시 WARN(REQ-LC-010) |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/branding.md` §3 다국어 페르소나 시스템 (4개 언어 × 5개 페르소나 매트릭스)
- `.moai/project/adaptation.md` §4 Cultural Context (4 언어 자동 감지 + 문화별 뉘앙스 + 명절)
- `.moai/project/research/hermes-llm.md` §2 15+ LLM 프로바이더 매트릭스 (다양성 벤치마크)
- `.moai/specs/ROADMAP.md` §3 Phase 6 (v5.0 확장 9 SPEC 중 Localization 4 SPEC의 기반)
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` §6.2 ENV 매핑(`GOOSE_LOCALE`)

### 9.2 외부 참조

- ISO 3166-1 alpha-2 country codes
- BCP 47 language tags (RFC 5646)
- IANA Time Zone Database
- ISO 4217 currency codes
- MaxMind GeoLite2 Country DB: https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
- ipapi.co HTTPS API documentation
- Unicode CLDR supplemental data (week info, measurement, calendar preference)

### 9.3 부속 문서

- `./research.md` — OS API 비교 + country mapping 결정 근거 + Hermes gateway 다국어 패턴 분석
- `../SPEC-GOOSE-I18N-001/spec.md` — UI 번역 소비자
- `../SPEC-GOOSE-REGION-SKILLS-001/spec.md` — country 기반 Skill 활성화
- `../SPEC-GOOSE-ONBOARDING-001/spec.md` — Detect() 결과 표시 + override 수집

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **UI 번역 리소스를 포함하지 않는다**(I18N-001 전담).
- 본 SPEC은 **국가별 Skill 자동 활성화를 구현하지 않는다**(REGION-SKILLS-001).
- 본 SPEC은 **온보딩 8단계 화면을 렌더링하지 않는다**(ONBOARDING-001). `Detect()` 결과를 반환할 뿐.
- 본 SPEC은 **공휴일 데이터베이스를 관리하지 않는다**. `CalendarSystem` 메타데이터만 노출.
- 본 SPEC은 **GDPR/PIPA/PIPL 실제 준수 로직을 포함하지 않는다**. `legal_flags`는 힌트이며 각 소비 SPEC이 준수.
- 본 SPEC은 **문화권별 실제 말투(존댓말/敬語 텍스트)를 생성하지 않는다**. branding.md의 메타데이터만 노출, 말투 치환은 ADAPTER-001 또는 persona engine.
- 본 SPEC은 **Accept-Language HTTP 헤더 파싱을 수행하지 않는다**(I18N-001).
- 본 SPEC은 **유료 geolocation 서비스(MaxMind GeoIP2 commercial, ipstack)를 통합하지 않는다**.
- 본 SPEC은 **IP 주소를 영속 저장하지 않는다**. country 결과만 저장.
- 본 SPEC은 **hot reload / 파일 감시를 구현하지 않는다**(CONFIG-001과 동일 원칙).
- 본 SPEC은 **다국적 대응 말투 mixing 엔진(예: 영어 + 존댓말 혼용 생성)을 구현하지 않는다**. `BuildSystemPromptAddendum`은 지시만 내리고, 실제 생성은 LLM.

---

**End of SPEC-GOOSE-LOCALE-001**
