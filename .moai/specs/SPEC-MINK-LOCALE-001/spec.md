---
id: SPEC-MINK-LOCALE-001
version: 0.3.0
status: in-progress
created_at: 2026-04-22
updated_at: 2026-05-16
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 소(S)
lifecycle: spec-anchored
labels: ["phase-6", "localization", "foundation", "locale-detection", "cultural-context"]
---

# SPEC-MINK-LOCALE-001 — Locale Detection + Cultural Context Injection

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성. v5.0 ROADMAP Phase 6 확장에 따른 Localization 4 SPEC 시리즈의 **기반층**. 사용자 최종 지시(2026-04-22): "한국뿐만 아니라 설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게 각 현지화된 스킬들을 추가, hermes-agent 정도의 다국어" 반영. | manager-spec |
| 0.1.1 | 2026-04-25 | Iteration 1 감사(mass-20260425/LOCALE-001-audit) Must-Pass + Major 결함 대응. (1) frontmatter `labels` 채움(D1, MP-3). (2) AC-LC-001~012에 `Covers REQ-LC-XXX` 트레이서빌리티 추가(D3). (3) 누락 REQ 6개(002/003/011/012/014/016)에 대해 AC-LC-013~018 신설(D4). (4) number/date/time format + collation 스코프 분기를 Exclusions 및 Technical Approach §6.7에 명시하여 I18N-001로 위임(D5). (5) Country→Currency 매핑을 "CLDR-inspired manual map, ~20개 우선 + ISO 3166↔4217 확장 240개"로 확정(D6, §6.8). (6) 다중 타임존 국가(US/RU/BR/CA/AU)의 기본 TZ 선택 정책을 `OS TZ env > CLDR primary zone > conflict 기록`으로 확정하고 AC-LC-018로 커버(D7, §6.9). TRUST 5는 §6.10으로 이동. REQ 번호 재배치 없음. research.md 변경 없음. | manager-spec |
| 0.2.0 | 2026-05-16 | Phase 1 구현 완료. `internal/locale/` 패키지 신설 (7개 소스 파일 + 4개 테스트 파일). Detect()/DetectWithOverride()/ResolveCulturalContext()/CountryToCurrency()/PrimaryTimezone()/TimezoneAlternatives()/BuildSystemPromptAddendum()/Load()/Save() 구현. 30개 우선 국가 currency 맵, 5개 다중 타임존 국가 CLDR 테이블, 20+ 국가 cultural 매핑 테이블 인코딩. CLI wiring: `defaultLocaleIndex()` 헬퍼로 `runStep1Locale` 초기 선택값을 OS 감지 결과로 pre-select. 테스트 커버리지 92.7% (목표 85% 초과). 모든 기존 테스트 통과. IP geolocation HTTP 프로브(MaxMind/ipapi.co) 및 Web wiring은 Phase 2 follow-up PR로 위임. | expert-backend |
| 0.3.0 | 2026-05-16 | **amendment-v0.2 — 자동 감지 (browser GPS + IP geolocation + 수동 폴백) 요구사항 증설**. ONBOARDING-001 Phase 4 web-speedrun hotfix 세션 (2026-05-16) 에서 사용자 결정 반영: "Step 1 지역 선택은 브라우저 GPS / IP geolocation 으로 자동 감지하고 수동 입력은 폴백" → 본 amendment 로 편입. 신규 REQ 6개 (REQ-LC-040~045: 자동 감지 entry / Web Geolocation API / IP fallback / 권한 거부 처리 / 정확도 표기 / 프라이버시 고지), 신규 AC 6개 (AC-LC-020~025, 모두 binary verifiable), §3.1 IN SCOPE 확장 (browser GPS 경로 명시), §6 Technical Approach 신규 §6.11 (자동 감지 흐름 + Web/CLI 분기 + 폴백 trees), §6.12 (프라이버시 고지 정책 — PIPA/GDPR/CCPA 부합 텍스트 가이드), §10 영향 범위 신설. 기존 REQ-LC-001~016 / AC-LC-001~018 변경 0 (Phase 1 종결물 보존). Web 구현 의존성: navigator.geolocation + IP geolocation HTTP 프로브 (ipapi.co 1차 / Nominatim 2차 / MaxMind GeoLite2 로컬 옵션 — §6.5 와 일치). CLI 구현은 IP geolocation only (TUI 환경, OS timezone 보조). 새 npm/Go 모듈 도입 0. frontmatter version 0.1.1 → 0.3.0, status planned → in-progress, updated_at 2026-04-25 → 2026-05-16 정합화. | manager-spec |

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
10. **자동 감지 entry point (Web + CLI) — amendment-v0.2**:
    - Web 측은 `navigator.geolocation.getCurrentPosition` + reverse geocoding (백엔드 hop) 을 1차로 사용. 권한 거부 / 실패 시 IP geolocation HTTP 프로브로 자동 전환.
    - CLI 측은 IP geolocation HTTP 프로브 + OS timezone 을 1차로 사용 (browser GPS 는 TUI 환경 불가).
    - 두 경로 모두 실패하면 수동 폴백 (Web: 4-preset radio / CLI: free-form text prompt) 으로 진입.
11. **정확도 등급 표기 — amendment-v0.2**:
    - GPS = city-level (`accuracy: "high"`)
    - IP = country-level (`accuracy: "medium"`)
    - 수동 = `accuracy: "manual"`
    - `LocaleContext.accuracy` 필드로 영속화. LLM system prompt addendum 의 `Detection: {accuracy}` 라인으로 노출 (REQ-LC-044).
12. **프라이버시 고지 — amendment-v0.2**:
    - 자동 감지 호출 전 사용자 명시 동의 절차 (Web: Step 1 inline 텍스트 + Geolocation permission prompt / CLI: `--auto-detect` default-on + stderr 1-line 고지).
    - GDPR Art. 13 / PIPA Art. 15 / CCPA §1798.100 의 데이터 수집 고지 의무를 충족.
    - 사용자는 언제든 `--no-auto-detect` 또는 브라우저 권한 거부로 비활성화 가능 (REQ-LC-045).

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

### 4.6 자동 감지 (Auto-Detection — amendment-v0.2)

amendment-v0.2 (2026-05-16) 에서 신설된 자동 감지 entry 흐름. ONBOARDING-001 Phase 4 web-speedrun hotfix 세션의 사용자 결정 "Step 1 지역 선택은 브라우저 GPS / IP geolocation 으로 자동 감지하고 수동 입력은 폴백" 을 본 §에서 EARS 요구사항으로 확정한다. REQ 번호 042~ 가 아닌 040~ 로 점프하는 것은 향후 보조 detect 모듈 (017~039 예약 슬롯) 확장 여지를 보존하기 위함이다.

**REQ-LC-040 [Event-Driven]** — **When** 사용자가 onboarding Step 1 에 진입하면 (Web UI: 컴포넌트 mount / CLI: `mink init` 진입), the system **shall** 자동 감지를 1차로 시도하고 결과를 4-preset 또는 free-form UI 에 pre-select 한다. 자동 감지 entry 는 default-on 이며, 사용자는 명시적 비활성화 경로 (Web: 권한 거부 / CLI: `--no-auto-detect`) 로만 우회할 수 있다.

**REQ-LC-041 [State-Driven]** — **While** 사용자가 Web UI 에서 자동 감지를 활성화한 상태, the system **shall** 다음 순서로 시도한다: (1) `navigator.geolocation.getCurrentPosition` (city-level, `accuracy="high"`, timeout 5s), (2) IP geolocation HTTP 프로브 (country-level, `accuracy="medium"`, timeout 3s), (3) 모두 실패 시 수동 폴백 (`accuracy="manual"`). 각 단계는 비차단(non-blocking) 이며 사용자 진행을 막지 않는다.

**REQ-LC-042 [State-Driven]** — **While** 사용자가 CLI 환경에서 `mink init --auto-detect` 또는 `mink init` (기본 default-on) 을 실행한 상태, the system **shall** IP geolocation HTTP 프로브 + OS timezone 을 1차로 시도하고, 실패 시 OS env 만으로 detect 한다 (browser GPS 는 CLI 에서 사용 불가, REQ-LC-001 의 기존 OS detect 경로와 통합된다). `--no-auto-detect` flag 가 set 이면 자동 감지를 건너뛰고 기존 OS env 경로만 사용한다.

**REQ-LC-043 [Unwanted]** — **If** Web Geolocation API 권한이 거부되면, **then** the system **shall** 자동으로 IP geolocation 경로로 전환하고, 사용자에게 차단된 단계를 알리는 비차단 UI 메시지 (toast 또는 inline notice) 를 표시한다. **The system shall not** 사용자 진행을 차단하거나 modal dialog 로 응답을 강요한다.

**REQ-LC-044 [Ubiquitous]** — The system **shall** 모든 `LocaleContext` 결과에 `accuracy` 필드 (`"high"` | `"medium"` | `"manual"`) 를 포함하고, 이를 `BuildSystemPromptAddendum` 출력의 `Detection: {accuracy}` 라인으로 노출한다. `accuracy` 필드는 CONFIG-001 의 `locale:` 섹션에 영속화되며, 사용자 override 가 적용된 경우 `accuracy="manual"` 로 기록된다.

**REQ-LC-045 [Ubiquitous, Privacy]** — The system **shall** 자동 감지 호출 직전 사용자에게 명시적 고지를 제시한다: (a) Web — Step 1 진입 시 inline 텍스트 + Geolocation permission prompt 의 brower-native 동의 절차, (b) CLI — stderr 1-line 고지 (`"Detecting your location for personalisation. Use --no-auto-detect to skip."`). 고지 텍스트는 GDPR Art. 13 (데이터 수집 직전 고지) / PIPA Art. 15 (수집 동의) / CCPA §1798.100 (소비자 권리 고지) 의 의무를 충족하는 표현으로 작성한다. **The system shall not** 사용자 동의 없이 위치 정보를 외부 제3자에게 전송한다 (REQ-LC-012 와 결합).

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-LC-001 — OS 감지 기본 경로 (Linux)**
- **Covers**: REQ-LC-001, REQ-LC-005
- **Given** `LANG=ko_KR.UTF-8`, `TZ=Asia/Seoul` 환경변수
- **When** `Detect(ctx)`
- **Then** `LocaleContext{country:"KR", primary_language:"ko-KR", timezone:"Asia/Seoul", currency:"KRW", measurement_system:"metric", calendar_system:"gregorian", detected_method:"os"}` 반환

**AC-LC-002 — OS 감지 기본 경로 (macOS)**
- **Covers**: REQ-LC-001, REQ-LC-005
- **Given** `defaults read -g AppleLocale` = `"ja_JP@calendar=gregorian"`, `AppleLanguages[0]` = `"ja"`
- **When** `Detect(ctx)`
- **Then** `primary_language="ja-JP"`, `country="JP"`, `currency="JPY"`, `honorific_system="japanese_keigo"`

**AC-LC-003 — IP fallback**
- **Covers**: REQ-LC-004, REQ-LC-005
- **Given** OS locale 환경변수 모두 비어있음(서버 환경), MaxMind DB에서 IP → country=`VN`
- **When** `Detect(ctx)`
- **Then** `country="VN"`, `primary_language="vi-VN"`, `detected_method="ip"`

**AC-LC-004 — User override 최우선**
- **Covers**: REQ-LC-006
- **Given** OS=ko_KR 감지됨, 사용자가 `config.yaml`에 `locale.override.country: JP`, `primary_language: ja-JP` 설정
- **When** `Detect(ctx)`
- **Then** `country="JP"`, `primary_language="ja-JP"`, `detected_method="user_override"`; OS 값은 무시

**AC-LC-005 — 다국적 사용자(primary + secondary)**
- **Covers**: REQ-LC-009
- **Given** `primary_language="ko-KR"`, `secondary_language="en-US"` override
- **When** `BuildSystemPromptAddendum(loc, cul)`
- **Then** 결과 문자열에 `"primary: ko-KR"`와 `"secondary: en-US"` 모두 포함, `"code-switching is natural"` 지시문 존재

**AC-LC-006 — Cultural context 한국**
- **Covers**: REQ-LC-001, REQ-LC-003
- **Given** `country="KR"`
- **When** `ResolveCulturalContext("KR")`
- **Then** `{formality_default:"formal", honorific_system:"korean_jondaetmal", name_order:"family_first", weekend_days:["Sat","Sun"], first_day_of_week:"Monday", legal_flags:["pipa"]}`

**AC-LC-007 — Cultural context 사우디**
- **Covers**: REQ-LC-001, REQ-LC-003
- **Given** `country="SA"`
- **When** `ResolveCulturalContext("SA")`
- **Then** `{calendar_system:"hijri", weekend_days:["Fri","Sat"], honorific_system:"arabic_formal_familiar"}`

**AC-LC-008 — OS vs IP 충돌 보존**
- **Covers**: REQ-LC-008
- **Given** OS=`ko_KR`, IP geolocation=`US`
- **When** `Detect(ctx)`
- **Then** `country="KR"` (OS 우선), `LocaleContext.conflict={os:"KR", ip:"US"}`, ONBOARDING-001이 이 필드를 읽어 사용자에게 확인 다이얼로그 표시

**AC-LC-009 — 중국에서 ipapi.co 스킵**
- **Covers**: REQ-LC-015
- **Given** OS 감지 결과 `country="CN"`, `geolocation_enabled=true`
- **When** OS 결과에 timezone 누락 → ipapi fallback이 호출되려 할 때
- **Then** ipapi HTTP 호출이 발생하지 않고, MaxMind DB만 조회되며, 실패 시 default로 폴백

**AC-LC-010 — 환경변수 injection 거부**
- **Covers**: REQ-LC-013
- **Given** `LANG="en_US.UTF-8; curl evil.com"`
- **When** `Detect(ctx)`
- **Then** 파서가 `;` 발견 후 reject, security event 로그, 기본값 en-US로 폴백

**AC-LC-011 — LLM prompt addendum 길이 제한**
- **Covers**: REQ-LC-007
- **Given** 한국 + primary/secondary language 포함 최대 케이스
- **When** `BuildSystemPromptAddendum(loc, cul)`
- **Then** 결과 UTF-8 문자열의 토큰 수 ≤ 400 (`cl100k_base` tokenizer 기준 정확 측정 — 테스트는 tiktoken-go 라이브러리를 사용해 exact count 단언)

**AC-LC-012 — MaxMind DB 노후 경고**
- **Covers**: REQ-LC-010
- **Given** `geoip_db_path`가 가리키는 .mmdb 파일의 mtime이 91일 전
- **When** 로더 실행
- **Then** WARN 로그 1건 (`locale.geoip.db.stale`), `Detect()`는 정상 동작

**AC-LC-013 — OS 감지 성공률(REQ-LC-002 단위 테스트화)**
- **Covers**: REQ-LC-002
- **Given** 3 OS matrix에서 각각 유효한 locale 조합(Linux: `LANG=ko_KR.UTF-8`, macOS: `AppleLocale=ja_JP`, Windows: `GetUserDefaultLocaleName=en-US`)
- **When** 각 OS에서 `Detect(ctx)` 호출
- **Then** 3 OS 모두 `detected_method ∈ {"os","ip","user_override"}`이며 `"default"`가 아님. CI는 `ubuntu-latest`, `macos-latest`, `windows-latest`에서 모두 통과해야 한다. REQ-LC-002의 "95%" 표현은 본 AC로 운영상 deterministic 검증으로 대체한다.

**AC-LC-014 — CulturalContext 결정론(REQ-LC-003)**
- **Covers**: REQ-LC-003
- **Given** 동일한 country 입력 `"KR"`
- **When** `ResolveCulturalContext("KR")` 을 100회 반복 호출
- **Then** 반환된 모든 `CulturalContext` 구조체가 deep-equal (직렬화된 YAML 바이트가 바이트 단위로 동일). 순서가 있는 필드(`weekend_days`, `legal_flags`)도 순서까지 동일.

**AC-LC-015 — CONFIG-001 `locale:` 스키마 라운드트립(REQ-LC-011)**
- **Covers**: REQ-LC-011
- **Given** `~/.goose/config.yaml`에 `locale.override.country=JP`, `locale.geolocation_enabled=false`, `locale.geoip_db_path="/tmp/geo.mmdb"` 작성
- **When** CONFIG-001 loader 실행 → struct 역직렬화 → 다시 YAML 직렬화
- **Then** 타입 필드 모두 존재(`override` is `*LocaleContext`, `geolocation_enabled` is `bool`, `geoip_db_path` is `string`), 원본과 재직렬화 결과가 의미적 동일(key order 무시), 알 수 없는 필드는 역직렬화 시 rejected.

**AC-LC-016 — Telemetry OFF 기본 & 제3자 전송 금지(REQ-LC-012)**
- **Covers**: REQ-LC-012
- **Given** 기본 설정(`geolocation_enabled` 미지정 또는 `false`)에서 `Detect(ctx)` 호출
- **When** 네트워크 모니터(테스트용 httptest round-tripper)로 모든 outbound HTTP 관측
- **Then** 관측된 outbound 요청 수 = 0. `geolocation_enabled=true`로 명시하더라도, 호스트 allow-list는 `ipapi.co`만 포함하며 그 외 도메인으로의 전송이 발생하면 테스트 실패.

**AC-LC-017 — 환경변수 순수성(REQ-LC-014)**
- **Covers**: REQ-LC-014
- **Given** 테스트 시작 시점에 `os.Environ()` 스냅샷 캡처
- **When** `Detect(ctx)` 호출 완료 후
- **Then** `os.Environ()` 재캡처 값이 스냅샷과 바이트 단위로 동일. `os.Setenv`/`os.Unsetenv` 호출 0건(reflection-free 검증은 `LANG`, `LC_ALL`, `LC_MESSAGES`, `TZ` 키에 대해 값 비교로 대체 가능).

**AC-LC-018 — 다중 타임존 국가 및 timezone_alternatives(REQ-LC-016 + §6.9)**
- **Covers**: REQ-LC-001, REQ-LC-016
- **Given** 사용자 country=`US`, OS `TZ` 환경변수 미설정(ambiguous case)
- **When** `Detect(ctx)` 호출
- **Then** `LocaleContext.timezone`은 CLDR likelySubtags 기반 대표 존(미국=`America/New_York`)으로 결정되고, `LocaleContext.timezone_alternatives`에 `["America/New_York","America/Chicago","America/Denver","America/Los_Angeles","America/Anchorage","Pacific/Honolulu"]` 6개 IANA zone이 포함되며, `LocaleContext.conflict`에는 기록되지 않는다(다중존은 conflict가 아닌 ambiguity로 분류). 동일 케이스에서 OS `TZ=America/Los_Angeles`가 설정되어 있으면 `timezone="America/Los_Angeles"`가 우선하고 `timezone_alternatives`는 생략된다.

> **참고**: AC-LC-019 는 차후 보조 detect 모듈 (REQ-LC-017~039 예약 슬롯) 용으로 예약되어 amendment-v0.2 에서는 의도적으로 빈 슬롯으로 둔다. AC-LC-020~025 는 amendment-v0.2 의 신규 자동 감지 요구사항 (REQ-LC-040~045) 을 검증한다.

**AC-LC-020 — Web Step 1 진입 시 자동 감지 시도 (amendment-v0.2)**
- **Covers**: REQ-LC-040, REQ-LC-041
- **Given** Web UI Step 1 컴포넌트가 mount 되고, 브라우저가 Geolocation API 권한을 허용한 상태 (test fixture: `navigator.geolocation.getCurrentPosition` mock 이 `{coords: {latitude: 37.5665, longitude: 126.9780}}` 반환)
- **When** `Step1Locale` 컴포넌트의 자동 감지 effect 가 실행되어 `InstallApi.probeLocale(sessionId)` 가 호출되고, 백엔드가 reverse geocoding 결과를 반환
- **Then** `country="KR"`, `primary_language="ko-KR"`, `timezone="Asia/Seoul"` 이 pre-select 되고, `LocaleContext.accuracy="high"` 가 기록되며, UI 의 4-preset radio 가 자동 감지 결과 항목으로 표시된다. 백엔드 응답 시간이 `5s` 를 초과하면 timeout 으로 IP fallback 으로 전환 (AC-LC-021 참조).

**AC-LC-021 — Web Geolocation 권한 거부 시 IP fallback (amendment-v0.2)**
- **Covers**: REQ-LC-041, REQ-LC-043
- **Given** Web UI Step 1 진입, Geolocation API 권한이 사용자에 의해 거부됨 (`navigator.geolocation.getCurrentPosition` mock 이 `PERMISSION_DENIED` error 콜백 호출)
- **When** Step1Locale 의 fallback 로직이 IP geolocation HTTP 프로브를 트리거 (`POST /install/api/locale/probe` 가 IP fallback 분기를 응답)
- **Then** country-level 결과로 pre-select, `LocaleContext.accuracy="medium"`, inline notice 가 DOM 에 렌더링됨 (예: `"위치 권한이 거부되어 IP 기반으로 감지했습니다"`). 사용자 진행을 차단하는 modal 은 표시되지 않는다 (REQ-LC-043). `data-testid="locale-fallback-notice"` 로 테스트가 검증 가능해야 한다.

**AC-LC-022 — CLI 자동 감지 default + --no-auto-detect 비활성화 (amendment-v0.2)**
- **Covers**: REQ-LC-042, REQ-LC-045
- **Given** `mink init` 실행 (no flag, default-on)
- **When** 자동 감지 entry point 가 호출됨
- **Then** stderr 에 정확히 1줄의 고지가 출력됨 (`"Detecting your location for personalisation. Use --no-auto-detect to skip. (locally stored only)"` 정규식 매칭), IP geolocation HTTP 프로브 + OS timezone 시도. `mink init --no-auto-detect` 변형 실행 시에는 stderr 고지가 출력되지 않고 자동 감지가 건너뛰어지며 기존 OS env detect 경로만 실행된다 (REQ-LC-001 의 기존 동작 보존).

**AC-LC-023 — accuracy 필드 LLM prompt 노출 (amendment-v0.2)**
- **Covers**: REQ-LC-044
- **Given** `LocaleContext.accuracy ∈ {"high", "medium", "manual"}` 중 하나가 설정됨
- **When** `BuildSystemPromptAddendum(loc, cul)` 호출
- **Then** 결과 UTF-8 문자열에 `Detection: high` (또는 `Detection: medium`, `Detection: manual`) 라인이 정확히 포함됨. `strings.Contains(addendum, "Detection: " + loc.Accuracy)` 단언이 PASS. accuracy 가 비어 있으면 (`""`) 해당 라인은 생략되고 backward compatibility 가 유지된다 (Phase 1 기존 호출자 보호).

**AC-LC-024 — 자동 감지 모든 경로 실패 시 수동 폴백 진입 (amendment-v0.2)**
- **Covers**: REQ-LC-040, REQ-LC-041
- **Given** GPS 권한 거부 + IP 프로브 timeout (3s 경과) / 네트워크 차단 / VPN 의심 모두 발생 (Web: `geolocation` denied + `fetch` 가 timeout. CLI: HTTP probe 가 context.DeadlineExceeded 반환)
- **When** 자동 감지 entry 흐름이 종료됨
- **Then** Step 1 UI 가 4-preset radio (Web — KR/US/FR/DE) 또는 free-form text prompt (CLI) 폴백 모드로 전환되고, `LocaleContext.accuracy="manual"` 이 기록되며, 사용자가 명시적으로 선택할 때까지 다음 Step 으로 진행되지 않는다 (Step 1 의 "수동 입력" 요구사항). 폴백 전환 자체는 자동이며 사용자에게 별도 확인을 요구하지 않는다.

**AC-LC-025 — 프라이버시 고지 텍스트 존재 + 사용자 동의 경로 (amendment-v0.2)**
- **Covers**: REQ-LC-045
- **Given** Web Step 1 mount 또는 CLI `mink init` 진입
- **When** 자동 감지 호출 직전
- **Then**:
  - **Web** 측: inline 고지 텍스트가 DOM 에 렌더링됨 (`data-testid="locale-privacy-notice"`). 텍스트는 (1) 수집 대상 (위치/국가), (2) 수집 방법 (Geolocation API + IP), (3) 저장 위치 (locally only, no telemetry), (4) 거부 방법 (브라우저 권한 거부) 의 4 가지 핵심 정보를 모두 포함. Geolocation permission prompt 가 그 직후에 트리거된다.
  - **CLI** 측: stderr 에 1줄 고지 (AC-LC-022 의 정규식과 동일) 가 출력됨.
  - **두 경우 모두**: 사용자가 `--no-auto-detect` (CLI) 또는 브라우저 권한 거부 (Web) 로 자동 감지를 비활성화할 수 있어야 한다 (AC-LC-021, AC-LC-022 와 결합).

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

### 6.7 Format 필드 스코프 분기 (number / date / time / collation)

감사 iteration 1(D5)에서 지적된 포맷 dimension의 경계를 본 SPEC에서 **명시적으로 I18N-001로 위임**한다.

| Format 차원 | 소유 SPEC | 본 SPEC 역할 |
|-----------|---------|-----------|
| `number_format` (decimal separator, thousand separator, grouping) | I18N-001 | LocaleContext에 필드 없음. `primary_language`만 노출하고 I18N-001이 CLDR `supplemental/numberingSystems` 테이블로 해석. |
| `date_format` (DD/MM/YYYY, MM/DD/YYYY, YYYY-MM-DD, 기타) | I18N-001 | LocaleContext에 필드 없음. I18N-001이 CLDR `main/{locale}/dates/calendars/{cal}/dateFormats`로 해석. |
| `time_format` (12h vs 24h, AM/PM markers) | I18N-001 | LocaleContext에 필드 없음. I18N-001이 CLDR `main/{locale}/dates/calendars/{cal}/timeFormats`로 해석. |
| `collation` (CLDR UCA sort order — 독일 phonebook vs dictionary, 스웨덴 å placement, 중국 pinyin vs stroke) | I18N-001 | LocaleContext에 필드 없음. I18N-001이 `golang.org/x/text/collate`로 해석. |

**이유**:
- LOCALE-001은 **"사용자가 누구인가"**(country/language/timezone/currency/cultural context)를 결정하는 기반층이다.
- 포맷팅(number/date/time/collation)은 **"사용자에게 무엇을 어떻게 보여줄 것인가"**의 문제로, 소비 시점(UI 렌더링, 문서 생성)에 결정되어야 한다.
- 두 관심사를 분리하면 (a) LocaleContext struct가 flat 유지되어 YAML 직렬화가 단순해지고, (b) I18N-001이 CLDR 데이터 파일을 번들링하는 책임을 일원화할 수 있다.

**소비 경로**: I18N-001은 LOCALE-001이 노출한 `LocaleContext.primary_language`(BCP 47)를 입력으로 받아 CLDR 테이블을 조회한다. LOCALE-001은 format 자체를 계산하지 않는다.

### 6.8 Country → Currency 매핑 결정

감사 iteration 1(D6)에서 제기된 오픈 이슈(research.md §12 item #4)를 확정한다.

**결정**: `internal/locale/cultural.go`의 정적 맵 `countryToCurrency map[string]string`에 **CLDR-inspired manual mapping**을 인코딩한다. 외부 라이브러리(`golang.org/x/text/currency`) 의존 추가 금지.

**근거**:
1. **의존성 최소화**: `x/text/language`는 이미 BCP 47 파싱에 필요. `x/text/currency`는 ISO 4217 메타데이터(통화명, 소수점 자릿수 등)를 제공하지만, 본 SPEC은 단순 country→currency lookup만 필요.
2. **커버리지 충분**: §6.3의 20개국 + research.md §1.2에 따른 Hermes Tier 1(en/ko/ja/zh) 중심 → v0.1.1 범위는 20개국으로 확정. 나머지 ISO 3166 알파-2 국가(약 240개)는 **CLDR `supplemental/supplementalData.xml`의 `<currencyData>` 섹션**에서 추출한 매핑을 manual 테이블에 추가하는 방식으로 확장 가능.
3. **테이블 위치**: `internal/locale/cultural.go`에 `countryToCurrency` (string→string, ISO 3166-1 alpha-2 → ISO 4217) 정적 맵으로 정의. 누락 국가는 `USD`로 폴백하고 `resolved_via_fallback=true` 진단 플래그(§6.10의 D11 제안과 연계; v0.1.1은 hint만 기록)를 WARN 로그에 기록.
4. **확장 절차**: 추가 국가 필요 시 PR로 `countryToCurrency` 엔트리를 추가(문화적 정확성 검증은 research.md §12 item #5와 공동 처리). CLDR 버전을 commit 메시지에 명시.

**매핑 소스 고정**: 초기 20개국은 Unicode CLDR 44.1 (2024-04 release) `supplemental/supplementalData.xml` `<region iso3166="XX">`의 `<currency>` 엔트리에서 발행 중(`iso4217="XXX"` tender="true") currency를 채택.

### 6.9 Multi-Timezone 국가 해석 정책

감사 iteration 1(D7)에서 지적된 US(6 zones), RU(11), BR(4), AU(5), CA(6) 등 다중 타임존 국가의 기본 TZ 선택 규칙을 확정한다.

**결정 규칙 (우선순위 순)**:

1. **OS TZ env 우선**: `$TZ` 환경변수 또는 macOS `systemsetup -gettimezone` / Windows `GetDynamicTimeZoneInformation`이 유효한 IANA zone을 반환하면 해당 값을 `LocaleContext.timezone`으로 채택. 이 경로에서는 `timezone_alternatives`를 생략(OS가 이미 사용자 선택을 표현).
2. **CLDR likelySubtags primary zone**: OS 정보가 없거나 빈 값이면 CLDR `supplemental/likelySubtags.xml`의 country→primary timezone 매핑(예: US→`America/New_York`, RU→`Europe/Moscow`, BR→`America/Sao_Paulo`, AU→`Australia/Sydney`, CA→`America/Toronto`)을 기본값으로 사용. 동시에 `LocaleContext.timezone_alternatives`에 해당 country의 모든 IANA zone을 기록.
3. **Conflict vs Ambiguity 구분**:
   - OS 값이 IP geolocation 값과 *country 수준에서 다르면* → `LocaleContext.conflict` 기록(REQ-LC-008, AC-LC-008). 예: OS=KR, IP=US.
   - OS 값이 없고 country만 결정된 경우 다중존 국가 → `LocaleContext.timezone_alternatives` 기록, conflict 없음. 예: OS `TZ` 미설정, IP=US → timezone=`America/New_York`, timezone_alternatives=6개 zone.
4. **ONBOARDING-001 위임**: `timezone_alternatives`가 비어 있지 않으면 ONBOARDING-001이 사용자에게 드롭다운 선택을 제공(본 SPEC은 데이터만 노출, UX는 위임).

**`LocaleContext`에 필드 추가**:
- `timezone_alternatives []string` — IANA zone 리스트, 다중존 국가에서만 채워짐. 단일존 국가(대부분)는 nil 또는 빈 슬라이스. YAML `omitempty`로 직렬화.
- 기존 REQ-LC-016의 `timezone_alternatives`는 본 필드로 구현되며, AC-LC-018로 검증.

**매핑 테이블**: `internal/locale/cultural.go`에 `countryToTimezones map[string][]string` 추가. 첫 원소가 primary, 나머지가 alternatives. 20개국 중 다중존: `US`, `RU`, `BR`, `AU`, `CA`. 단일존: 나머지 15개. 확장은 CLDR `metaZones.xml` + `timezone.xml` 기반.

**REQ 번호 재배치 없음**: 본 정책은 기존 REQ-LC-016(Optional)을 구체화하고 REQ-LC-001(IANA timezone 필드)을 보강한다. 신규 REQ 추가하지 않는다.

### 6.10 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | 20+ country 테이블 테스트(AC-LC-006/007 확장), 3 OS matrix CI, MaxMind mock, 커버리지 90%+ |
| **R**eadable | country 매핑 테이블을 별도 파일(`cultural.go`)로 분리, 한국어 주석 + 영어 식별자 |
| **U**nified | `golangci-lint` + CONFIG-001 동일 yaml struct tag 규칙 |
| **S**ecured | env var injection 거부(REQ-LC-013), 중국 geolocation HTTP 차단(REQ-LC-015), telemetry OFF 기본 |
| **T**rackable | `detected_method` 필드 + zap 구조화 로그(`locale.detect.completed` with source/country/lang) |

### 6.11 자동 감지 흐름 + Web/CLI 분기 (amendment-v0.2)

자동 감지는 두 entry point (Web onboarding Step 1 / CLI `mink init`) 에서 호출되며, 각 환경의 능력 차이를 반영한 분기 트리를 사용한다.

#### 6.11.1 Web 흐름

```
1. Step 1 컴포넌트 mount
   ├─ Privacy notice 렌더 (inline 텍스트, data-testid="locale-privacy-notice")
   └─ probeLocale(sessionId) effect 실행

2. navigator.geolocation.getCurrentPosition (timeout 5s)
   ├─ 권한 허용 + 성공 → POST /install/api/locale/probe (body: {lat, lng})
   │                  → backend reverse geocoding → {country, language, timezone}
   │                  → LocaleContext.accuracy = "high"
   │                  → 4-preset UI 에 자동 감지 결과 pre-select + 정확도 배지 표시
   │
   ├─ 권한 거부 (PERMISSION_DENIED) → IP fallback 분기로 자동 전환
   ├─ Timeout (5s) → IP fallback 분기로 자동 전환
   └─ 기타 error (POSITION_UNAVAILABLE 등) → IP fallback 분기로 자동 전환

3. IP fallback 분기
   ├─ POST /install/api/locale/probe (body: {}) → backend 가 X-Forwarded-For / RemoteAddr 로 IP 추출
   ├─ backend → ipapi.co HTTPS (timeout 3s) → {country, language, timezone}
   │            └─ rate-limit / outage → Nominatim 2차 fallback
   │            └─ 둘 다 실패 → manual fallback 분기로 전환
   ├─ 성공 → LocaleContext.accuracy = "medium"
   │       → 4-preset UI 에 결과 pre-select + 정확도 배지 + inline notice ("위치 권한 거부됨, IP 기반 감지")
   │       → data-testid="locale-fallback-notice" 로 검증 가능

4. Manual fallback 분기
   ├─ Step 1 UI 가 4-preset radio (KR/US/FR/DE) 만 표시
   ├─ LocaleContext.accuracy = "manual"
   └─ 사용자가 명시적으로 선택할 때까지 다음 Step 으로 진행 차단

5. LocaleContext 영속화
   └─ CONFIG-001 `locale:` 섹션에 country/language/timezone/accuracy 저장
```

#### 6.11.2 CLI 흐름 (`mink init`)

```
1. CLI 시작 (default = auto-detect on)
   ├─ stderr 1-line 고지 출력 (REQ-LC-045 / AC-LC-025):
   │   "Detecting your location for personalisation. Use --no-auto-detect to skip. (locally stored only)"
   └─ --no-auto-detect 가 set 이면 OS env detect (REQ-LC-001) 만 실행 + 고지 미출력

2. IP geolocation HTTP 프로브 (timeout 3s, internal/locale/iplookup.go 신규)
   ├─ ipapi.co HTTPS GET https://ipapi.co/json/
   │   ├─ 응답 정상 → {country, language, timezone}
   │   ├─ 5xx / rate-limit (429) → Nominatim 2차 fallback (optional, 호스트 allow-list 등록 후)
   │   └─ China 차단 (REQ-LC-015) → OS env detect 로 폴백 (해당 경로 자체 skip)
   ├─ context.DeadlineExceeded (3s) → OS env detect 로 폴백
   └─ 네트워크 오류 (DNS / connection refused 등) → OS env detect 로 폴백

3. OS env detect 와 결과 병합
   ├─ IP 결과의 country 가 OS env 결과 country 와 일치 → 그대로 채택, accuracy="medium"
   ├─ 두 country 가 불일치 → REQ-LC-008 의 conflict 분기 (LocaleContext.conflict 기록)
   └─ IP 만 가능 / OS 만 가능 → 가능한 쪽 채택

4. LocaleContext 영속화
   └─ accuracy 필드 = "medium" (IP) | "high" 는 CLI 에서 불가 (browser GPS 없음) | "manual" (override)
```

#### 6.11.3 라이브러리 결정 (§6.5 보강)

amendment-v0.2 는 새 npm/Go 모듈을 도입하지 않는다. 자동 감지에 필요한 의존성은 모두 표준 라이브러리 또는 §6.5 에 이미 등록된 항목으로 충족된다.

| 용도 | 라이브러리 | 환경 | 비고 |
|------|----------|-----|------|
| Web Geolocation API | `navigator.geolocation` | 브라우저 표준 | 추가 의존성 없음 |
| Web reverse geocoding 백엔드 호출 | 표준 `fetch` | Web | 추가 의존성 없음 |
| Web/CLI IP geolocation 1차 | `net/http` stdlib + ipapi.co HTTPS | 백엔드 | §6.5 항목 (기존) |
| Web/CLI IP geolocation 2차 fallback | `net/http` stdlib + Nominatim (OpenStreetMap) | 백엔드 | 사용자 선택 (rate-limit 1/sec, attribution 의무). 기본 disabled. |
| CLI offline DB (옵션) | `github.com/oschwald/maxminddb-golang` v2.x | CLI | §6.5 항목 (기존, GeoLite2 사용 시) |

**vendor 우선순위 결정**: 1차 ipapi.co (단순, IP+reverse geocoding 통합, free tier 30/min). 2차 Nominatim (OpenStreetMap, rate-limit 1/sec + attribution 의무, opt-in). 3차 MaxMind GeoLite2 로컬 DB (오프라인, ~4MB, CLI 전용 옵션). 모든 vendor 는 §6.5 의 기존 의존성 범위에 포함되며 새 추가는 없음.

### 6.12 프라이버시 고지 정책 (amendment-v0.2)

자동 감지 호출 전 사용자 명시 동의를 위한 고지 텍스트 가이드. GDPR Art. 13 (정보 수집 직전 고지) / PIPA Art. 15 (수집 동의) / CCPA §1798.100 (소비자 권리 고지) 의 데이터 수집 고지 의무를 단일 텍스트로 충족한다.

#### 6.12.1 4 핵심 원칙

모든 고지 텍스트 변형은 다음 4 가지 정보를 명시적으로 포함해야 한다.

1. **무엇을 수집하는가** — location / country / approximate position
2. **어떻게 수집하는가** — browser Geolocation API / IP address lookup
3. **어디에 저장하는가** — locally only (no external telemetry)
4. **어떻게 거부하는가** — browser permission denial / `--no-auto-detect` flag

#### 6.12.2 Web Step 1 inline 고지 텍스트 (예시)

```
영문 (canonical):
  To personalise your experience, MINK detects your location.
  You can decline the browser prompt or skip detection at any time.
  Detection result is stored locally only. No external telemetry.

한국어 병기 (옵션, I18N-001 catalog 소비 시):
  지역 정보 자동 감지: 브라우저 권한 거부 또는 건너뛰기 가능.
  결과는 로컬에만 저장됩니다.
```

- DOM 위치: Step 1 컴포넌트 상단 (`data-testid="locale-privacy-notice"`)
- 시각적 강조: 본문 텍스트보다 작지만 가독성 보장 (font-size ≥ 12px, color contrast ratio ≥ 4.5:1 — WCAG AA)
- I18N-001 통합 시 catalog 키 `locale.privacy.notice.web` 로 등록 (amendment-v0.2 범위 외, 후속 PR)

#### 6.12.3 CLI stderr 고지 텍스트

```
canonical (영문):
  Detecting your location for personalisation. Use --no-auto-detect to skip. (locally stored only)
```

- 출력 스트림: stderr (stdout 오염 방지, 파이프 친화)
- 1 줄, 80 자 이내 (전형적 터미널 폭)
- ANSI escape 없음 (NO_COLOR 환경 변수 준수)
- 출력 타이밍: HTTP 프로브 호출 *직전* (사용자가 차단 결정 가능한 시점)

#### 6.12.4 컴플라이언스 매핑

| 법령 | 조항 | 요구사항 | 본 고지의 충족 방식 |
|-----|-----|---------|------------------|
| GDPR | Art. 13 | 수집 직전 정보 주체에게 (a) 데이터 controller, (b) 수집 목적, (c) 법적 근거, (d) 저장 기간 고지 | 본문 (a) MINK 명시, (b) personalisation, (c) consent (사용자 권한 허용 = lawful basis), (d) locally only |
| PIPA | Art. 15 | 개인정보 수집 시 (a) 수집·이용 목적, (b) 수집 항목, (c) 보유·이용 기간, (d) 거부 권리 동의 | 본문 (a) personalisation, (b) location/country, (c) locally only, (d) `--no-auto-detect` / 권한 거부 |
| CCPA | §1798.100 | 소비자의 (a) 알 권리, (b) 거부 권리 보장 | 본문 (a) "detects your location", (b) "skip" / "decline" |

#### 6.12.5 사용자 거부 경로 (불가역적이지 않음)

- Web: 권한 거부 후에도 사용자가 추후 브라우저 설정에서 권한을 재허용하면 다음 onboarding 진입 시 다시 감지 가능
- CLI: `--no-auto-detect` 는 1회 실행 한정. 다음 `mink init` 호출 시 다시 default-on
- 영속화된 자동 감지 결과는 사용자가 ONBOARDING-001 또는 `mink config locale set` 으로 언제든 manual override 가능 (REQ-LC-006 의 user override 경로와 통합)

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, context 루트 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `locale:` 섹션 스키마 확장 대상 |
| 후속 SPEC | SPEC-MINK-I18N-001 | LocaleContext.primary_language 소비 |
| 후속 SPEC | SPEC-GOOSE-REGION-SKILLS-001 | LocaleContext.country 기반 Skill 활성화 |
| 후속 SPEC | SPEC-MINK-ONBOARDING-001 | Detect() 결과를 초기 화면에 표시 + override 수집 |
| 후속 SPEC | SPEC-GOOSE-SCHEDULER-001 | LocaleContext.timezone 소비, REGION-SKILLS-001 통해 공휴일 DB 선택 |
| 외부 | Go 1.22+ | `//go:build` tag |
| 외부 | `golang.org/x/text/language` v0.14+ | BCP 47 파싱 |
| 외부 | `github.com/oschwald/maxminddb-golang` v2.x | GeoLite2 DB reader |
| 외부 | MaxMind GeoLite2-Country.mmdb | ~4MB offline DB, 번들 또는 CLI로 다운로드 |
| 외부 (Web, amendment-v0.2) | `navigator.geolocation` | 브라우저 표준 API. 새 npm 패키지 도입 없음. |
| 외부 (Web/CLI, amendment-v0.2) | ipapi.co HTTPS | 1차 IP geolocation. 무료 tier 30 req/min. §6.5 기존 항목. |
| 외부 (Web/CLI, amendment-v0.2, optional) | Nominatim (OpenStreetMap) | 2차 IP geolocation fallback. rate-limit 1/sec, attribution 의무. 기본 disabled. |

> **amendment-v0.2 보강**: 자동 감지 entry 도입에도 불구하고 새 npm 또는 Go 모듈은 추가되지 않는다. 모든 의존성은 표준 `fetch` / `net/http` + 기존 §6.5 항목으로 충족된다.

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
| R8 (amendment-v0.2) | VPN / Tor 사용자에 대한 IP geolocation 부정확 | 고 | 낮 | `accuracy="medium"` 표기로 사용자에게 정확도 한계 노출 + manual override 수정 가능 (기존 R3 보강). LocaleContext.conflict 가 OS 결과와 IP 결과 불일치를 기록하므로 ONBOARDING-001 가 확인 UI 표시 |
| R9 (amendment-v0.2) | 외부 HTTP 의존성 (ipapi.co) rate-limit / outage | 중 | 중 | 2차 fallback Nominatim (opt-in) + 최종 fallback OS env detect. timeout 3s + circuit-break 패턴으로 사용자 진행 차단 방지 |
| R10 (amendment-v0.2) | GPS city-level 정확도가 프라이버시 위험 | 중 | 중 | lat/lng raw 값은 `LocaleContext` 에 저장하지 않음 (백엔드 reverse geocoding 직후 폐기). country/language/timezone 만 영속화. §6.12 4-원칙 고지로 사용자 명시 동의 보장 |
| R11 (amendment-v0.2) | accuracy 필드 미설정 시 backward compatibility 손상 | 낮 | 중 | `BuildSystemPromptAddendum` 이 `accuracy == ""` 인 경우 `Detection:` 라인을 생략하여 Phase 1 기존 호출자 보호 (AC-LC-023 의 단언 포함) |
| R12 (amendment-v0.2) | Web inline 고지가 권한 prompt 와 시각적으로 분리되어 사용자 혼동 | 낮 | 낮 | 고지 텍스트가 권한 prompt 직전에 렌더링되고, `data-testid="locale-privacy-notice"` 가 prompt 트리거 버튼과 동일 영역에 위치하도록 컴포넌트 설계 (REQ-LC-045) |

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
- `../SPEC-MINK-I18N-001/spec.md` — UI 번역 소비자
- `../SPEC-GOOSE-REGION-SKILLS-001/spec.md` — country 기반 Skill 활성화
- `../SPEC-MINK-ONBOARDING-001/spec.md` — Detect() 결과 표시 + override 수집

---

## 10. 영향 범위 (Impact Analysis — amendment-v0.2)

amendment-v0.2 에서 신설된 자동 감지 (browser GPS + IP fallback + 수동 폴백) 요구사항이 영향을 미치는 파일 목록. Phase 2 구현 PR 진행 시 본 표를 체크리스트로 활용한다.

### 10.1 백엔드 영향 (Go)

| 컴포넌트 | 변경 유형 | 비고 |
|---------|----------|------|
| `internal/server/install/handler.go` | 확장 | POST `/install/api/locale/probe` 엔드포인트 추가. Geolocation 결과 (lat/lng 수신) 또는 IP fallback (X-Forwarded-For / RemoteAddr) 분기 처리. CSRF / Origin allowlist 는 기존 install handler 패턴 재사용. |
| `internal/locale/detect.go` | 확장 | `DetectWithGeolocation(ctx, lat, lng) (*LocaleContext, error)` + `DetectFromIP(ctx, remoteAddr string) (*LocaleContext, error)` helper 추가. `LocaleContext.Accuracy` 필드 영속화. |
| `internal/locale/iplookup.go` | **신규** | ipapi.co (1차) + Nominatim (2차, opt-in) HTTP 클라이언트. timeout 3s. RFC 1918 / loopback / link-local 차단. 호스트 allow-list 강제 (REQ-LC-012 와 결합). |
| `internal/locale/context.go` | 확장 | `LocaleContext` 구조체에 `Accuracy string` (`"high"` | `"medium"` | `"manual"`) 필드 추가. YAML `omitempty` 로 backward compat 보장. |
| `internal/locale/prompts.go` | 확장 | `BuildSystemPromptAddendum` 에 `Detection: {accuracy}` 라인 조건부 포함 (accuracy 가 비어있으면 라인 생략). AC-LC-023 검증 대상. |
| `internal/cli/commands/init.go` | 확장 | `--no-auto-detect` flag 추가 (default false). 자동 감지 default-on. stderr 1-line 고지 (`fmt.Fprintln(os.Stderr, ...)`). NO_COLOR 환경 변수 준수. |
| `internal/server/install/handler_test.go` | 확장 | 신규 probe 엔드포인트 단위 테스트 (정상 / 권한 거부 / IP fallback / timeout 4 경로). |
| `internal/locale/iplookup_test.go` | **신규** | httptest 기반 mock vendor 응답 단위 테스트. ipapi.co 정상 / 5xx / 429 / Nominatim fallback / 모두 실패 5 경로. |
| `internal/locale/detect_test.go` | 확장 | `DetectWithGeolocation` / `DetectFromIP` 단위 테스트 + accuracy 필드 단언. |

### 10.2 프런트엔드 영향 (TypeScript / React)

| 컴포넌트 | 변경 유형 | 비고 |
|---------|----------|------|
| `web/install/src/components/steps/Step1Locale.tsx` | **재설계** | 기존 4-preset radio UI 를 "자동 감지 → 결과 표기 + 정확도 배지 + 수동 폴백 radio" 로 변경. PRESETS 4개는 폴백 경로로 유지. mount effect 에서 `probeLocale(sessionId)` 호출. |
| `web/install/src/lib/api.ts` | 확장 | `InstallApi.probeLocale(sessionId: string): Promise<LocaleProbeResult>` 메서드 추가. CSRF token + sessionId 헤더 자동 첨부. |
| `web/install/src/types/onboarding.ts` | 확장 | `LocaleChoice` 인터페이스에 `Accuracy: "high" \| "medium" \| "manual"` 필드 추가. `LocaleProbeResult` 신규 타입 (country/language/timezone/accuracy). |
| `web/install/src/components/steps/Step1Locale.test.tsx` | **신규 또는 확장** | Vitest + Testing Library 기반. `navigator.geolocation` mock + fetch mock 으로 6 시나리오 (성공 / 거부 → IP fallback / timeout → IP fallback / 모두 실패 → manual / 정확도 배지 / privacy notice DOM) 검증. AC-LC-020~025 매핑. |
| `web/install/src/lib/geolocation.ts` | **신규 (선택)** | `getCurrentPositionWithTimeout(timeoutMs: number)` 헬퍼. browser-native API 의 timeout 처리 + Promise 래핑. 단위 테스트 가능성을 위해 격리. |

### 10.3 통합 측 영향

| 컴포넌트 | 변경 유형 | 비고 |
|---------|----------|------|
| `internal/onboarding/types.go` | 확장 | `LocaleChoice` Go 측 동일 필드 (`Accuracy string`) 추가. JSON tag 로 web 측과 wire-compat. |
| `internal/onboarding/state.go` | 확장 (검증) | Step 1 완료 조건이 `accuracy` 필드 set 까지 포함하도록 검증. manual fallback 시 사용자 명시 선택 필수. |
| `.moai/specs/SPEC-MINK-ONBOARDING-001/spec.md` | 참조 갱신 (별도 PR 권장) | 본 SPEC 의 amendment-v0.2 결과를 소비. 별도 amendment 로 ONBOARDING-001 Step 1 acceptance criteria 갱신 필요. |
| `.moai/specs/SPEC-MINK-I18N-001/spec.md` | 참조 (별도 PR 권장) | 6.12.2 의 catalog 키 `locale.privacy.notice.web` 등록 (amendment-v0.2 범위 외). |
| `web/install/playwright/` (E2E) | 확장 (별도 PR) | 자동 감지 6 시나리오 E2E. amendment-v0.2 의 직접 산출물은 아니며, 후속 Phase 4 hotfix PR 에 포함 권장. |

### 10.4 영향 받지 않는 컴포넌트 (보존)

다음 컴포넌트는 amendment-v0.2 로 인해 *변경되지 않으며*, Phase 1 종결물의 동작이 그대로 보존된다. Phase 2 구현 PR 진행 시 본 컴포넌트들의 회귀를 차단하는 unit test 필수.

- `internal/locale/cultural.go` — 20+ country cultural 매핑 테이블 (변경 없음)
- `internal/locale/cultural_test.go` — 결정론 테스트 (변경 없음)
- `internal/locale/os_linux.go` / `os_darwin.go` / `os_windows.go` — OS env detect 경로 (REQ-LC-001 보존)
- `internal/locale/geo.go` — MaxMind GeoLite2 reader (변경 없음; iplookup.go 와는 별개 경로)
- AC-LC-001 ~ AC-LC-018 — Phase 1 종결 AC 18 개 모두 그대로 PASS 유지 의무

### 10.5 마이그레이션 / 데이터 호환성

- 기존 사용자의 `~/.mink/config.yaml` 의 `locale:` 섹션에 `accuracy` 필드가 없는 경우, 로더는 `accuracy: ""` (빈 문자열) 로 역직렬화하고 `BuildSystemPromptAddendum` 은 `Detection:` 라인을 생략하여 backward compat 유지 (R11 완화).
- 기존 onboarding flow 를 거친 사용자는 다음 `mink init --reset locale` 또는 `mink config locale set accuracy=manual` 으로 명시적 marker 부착 가능 (amendment-v0.2 의 직접 산출물 외).

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
- 본 SPEC은 **number format(소수점/천단위 구분자/그룹핑)을 `LocaleContext`에 포함하지 않는다**. I18N-001이 CLDR 기반으로 해석(§6.7).
- 본 SPEC은 **date format 템플릿(DD/MM/YYYY vs MM/DD/YYYY vs YYYY-MM-DD)을 정의하지 않는다**. I18N-001 CLDR 소비(§6.7).
- 본 SPEC은 **time format(12h vs 24h, AM/PM marker)을 정의하지 않는다**. I18N-001 CLDR 소비(§6.7).
- 본 SPEC은 **sort collation(CLDR UCA, 독일 phonebook/dictionary, 스웨덴 å 위치, 중국 pinyin/stroke)을 구현하지 않는다**. I18N-001이 `golang.org/x/text/collate`로 해석(§6.7).
- 본 SPEC은 **country→currency 매핑에 `golang.org/x/text/currency` 의존성을 추가하지 않는다**. `internal/locale/cultural.go`의 manual map `countryToCurrency`만 사용(§6.8).
- 본 SPEC은 **전 세계 모든 ISO 3166 국가를 v0.1.1 범위에 포함하지 않는다**. 초기 20개국 + USD 폴백. 나머지 ~240개 국가는 후속 PR로 CLDR 44.1 `supplementalData.xml` 기반 확장(§6.8).
- 본 SPEC은 **다중 타임존 국가(US/RU/BR/CA/AU)의 UX 선택 다이얼로그를 제공하지 않는다**. `timezone_alternatives` 필드로 데이터만 노출하고 UX는 ONBOARDING-001(§6.9).
- 본 SPEC은 **CLDR 데이터 파일(xml/json)을 번들 포함하지 않는다**. 정적 Go 맵으로 필요한 서브셋만 인코딩(`cultural.go`의 `countryToCurrency`, `countryToTimezones`, `countryToCultural`). CLDR 전체 번들링은 I18N-001 책임.

---

**End of SPEC-MINK-LOCALE-001**
