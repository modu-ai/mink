---
id: SPEC-GOOSE-REGION-SKILLS-001
version: 0.1.0
status: Planned
created: 2026-04-22
updated: 2026-04-22
author: manager-spec
priority: P1
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GOOSE-REGION-SKILLS-001 — Regional Skill Bundles + Locale-aware Activation

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성. v5.0 ROADMAP Phase 6 Localization 시리즈 3번째. 사용자 지시: "설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게 각 현지화된 스킬들을 추가". SKILLS-001의 `locales:` frontmatter 필드 확장 + `.claude/skills/region/{country}/` 디렉토리 계층 + 10+ 국가 번들. | manager-spec |

---

## 1. 개요 (Overview)

사용자 최종 지시(2026-04-22):

> "사용자의 국가와 정보를 수집해서 사용자에 맞게 각 현지화된 스킬들을 추가하도록 하자."

본 SPEC은 **국가별 Skill 번들** 시스템을 정의한다. LOCALE-001이 결정한 `LocaleContext.country`(ISO 3166-1)를 입력으로, 해당 국가에 특화된 Skill 집합(`.claude/skills/region/{country_code}/{skill_name}/SKILL.md`)을 자동으로 활성화한다. Skill은 SKILLS-001이 이미 정의한 YAML frontmatter 스키마를 따르되, **신규 `locales:` 필드**를 추가하여 "이 skill은 country_code in [...]일 때만 활성화된다"는 조건을 선언한다.

초기 10+ 국가 번들:

- **🇰🇷 kr** (한국): 공휴일 · 사주 · 카카오톡 · 네이버 · 배민 · 수능 D-day · 현충일
- **🇯🇵 jp** (일본): 祝日 · LINE · 敬語 · 温泉 · 桜 시즌
- **🇨🇳 cn** (중국): 法定节假日 · WeChat · 春节 · 支付宝 · 高考 · 农历
- **🇺🇸 us** (미국): Thanksgiving · federal holidays · Venmo · tipping · tax season
- **🇪🇺 eu** (유럽연합): GDPR strict mode · SEPA · metric units · Schengen
- **🇻🇳 vn** (베트남): Tết · Zalo · mobile money · 음력 달력
- **🇮🇳 in** (인도): Diwali · Holi · UPI · Hindi mixing · caste-neutral
- **🇸🇦/🇦🇪 ar-region** (아랍권): Ramadan · Hijri calendar · RTL · Halal
- **🇧🇷 br** (브라질): Carnaval · Pix · PT-BR spelling
- **🇩🇪 de** (독일): Feiertage · Sie/Du · Datenschutz

본 SPEC은 **Skill 카탈로그**와 **활성화 엔진**을 정의하고, 각 개별 Skill의 세부 콘텐츠 작성은 Phase 6 이후 커뮤니티 + 팀이 병렬 기여한다(본 SPEC은 10개 국가 × 3개 대표 skill = 30개 스켈레톤을 초기 번들).

---

## 2. 배경 (Background)

### 2.1 왜 Region Skill이 필요한가

- **SKILLS-001 재활용**: Progressive Disclosure + YAML frontmatter 인프라가 이미 존재. country 조건만 추가하면 "국가별 특화 지식"을 Skill로 표현 가능.
- **공휴일/결제/메신저 차이**: 한국 배민 vs 미국 DoorDash, 한국 카카오페이 vs 중국 支付宝. 하드코딩 대신 Skill로 추상화하면 20+ 국가 확장 가능.
- **문화적 에티켓**: adaptation.md §4는 한국 존댓말, 일본 お疲れ様, 중국 체면 등을 나열. 각 문화권의 대화 규칙을 Skill `context: fork` 모드로 agent system prompt에 주입.
- **법적 차이**: EU의 GDPR, 한국의 PIPA, 중국의 PIPL 각각 다른 처리 필요. LOCALE-001의 `legal_flags`를 보조 트리거로 활용.
- **ONBOARDING-001 연계**: 온보딩 Step 2에서 감지된 country로 자동 번들링되고, Step 6(Ritual Preferences)에서 사용자가 필요 없는 skill을 끌 수 있다.

### 2.2 SKILLS-001과의 관계

본 SPEC은 **SKILLS-001을 확장**한다:

- SKILLS-001의 `SAFE_SKILL_PROPERTIES` allowlist에 `locales` 추가
- SKILLS-001의 `TriggerMode`에 `TriggerLocale` enum 값 추가 (또는 `TriggerConditional` 확장)
- `LoadSkillsDir`가 `.claude/skills/region/{country_code}/` 하위도 walk
- `LocaleAwareActivator`가 `LocaleContext.country` 기반으로 활성 skill ID 목록 반환

### 2.3 상속 자산

- **MoAI-ADK `.claude/skills/moai/**`**: 네임스페이스 기반 Skill 조직 패턴. 본 SPEC은 `region/` 네임스페이스 추가.
- **SCHEDULER-001 공휴일 DB**: `rickar/cal/v2` 기반. REGION-SKILLS가 국가별 공휴일 Skill로 re-expose.
- **Claude Code 기본 skills**: 영어권 중심. GOOSE는 지리적 확장.

### 2.4 범위 경계

- **IN**: SKILLS-001 스키마 확장, `.claude/skills/region/` 계층, 10+ 국가 번들(30+ skill 스켈레톤), `LocaleAwareActivator` Go 구현, 수동 on/off UI 계약, 공휴일 데이터 소비자 규약.
- **OUT**: 각 Skill의 본문 상세 작성(커뮤니티 기여), 공휴일 DB 자체 구현(SCHEDULER-001), Skill 마켓플레이스 UI(PLUGIN-001), 결제 API 실제 통합(별도 SPEC).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **SKILLS-001 확장**:
   - `SAFE_SKILL_PROPERTIES`에 `locales` 필드 추가
   - `SkillFrontmatter.Locales []string` 필드 추가(ISO 3166-1 또는 `region:eu`/`region:ar-world` 그룹 별칭 허용)
   - `TriggerMode`에 활성화 규칙: `Locales`가 비어있지 않으면 `TriggerLocale` 우선
   - `LoadSkillsDir`가 `.claude/skills/region/{country_code}/` 하위 walk
2. **Locale Group 별칭**:
   - `region:eu` → [DE, FR, IT, ES, NL, BE, AT, PL, SE, DK, FI, IE, PT, GR, CZ, ...] (27개국)
   - `region:ar-world` → [SA, AE, EG, JO, LB, MA, TN, DZ, IQ, ...]
   - `region:latam` → [MX, BR, AR, CO, CL, PE, ...]
   - `region:sea` → [VN, TH, ID, MY, PH, SG, ...]
3. **`internal/skill/region/` 패키지**:
   - `activator.go` — `LocaleAwareActivator.ActiveSkillIDs(LocaleContext) []string`
   - `matcher.go` — country code + group alias 매칭 로직
   - `registry.go` — SKILLS-001 `SkillRegistry`를 래핑해 region skill만 필터
4. **초기 10개 국가 번들** (각각 `SKILL.md` 스켈레톤, 실제 콘텐츠는 후속 PR):
   - **kr** (한국): `holidays`, `kakao-talk`, `naver-services`, `jondaetmal-etiquette`
   - **jp** (일본): `holidays`, `line-messenger`, `keigo-etiquette`
   - **cn** (중국): `lunar-calendar`, `wechat`, `alipay-etiquette`
   - **us** (미국): `federal-holidays`, `tipping-etiquette`, `tax-season`
   - **eu** (그룹: region:eu): `gdpr-strict`, `sepa-payments`, `metric-units`
   - **vn** (베트남): `tet-celebration`, `zalo-messenger`, `lunar-calendar`
   - **in** (인도): `diwali-holi`, `upi-payments`, `caste-neutral-etiquette`
   - **ar-region** (그룹: region:ar-world): `ramadan-awareness`, `hijri-calendar`, `halal-dietary`
   - **br** (브라질): `carnaval`, `pix-payments`, `pt-br-spelling`
   - **de** (독일): `feiertage`, `sie-du-etiquette`, `datenschutz-gdpr`
5. **Skill 예제 frontmatter**:
   ```yaml
   ---
   name: korean-holidays
   description: "한국 공휴일 + 현충일 + 수능 D-day 인식"
   locales: [KR]
   effort: L1
   context: inline
   allowed-tools: [calendar.query, web.search]
   ---
   ```
6. **공휴일 데이터 소비 규약**:
   - SCHEDULER-001의 `rickar/cal/v2` 확장을 본 SPEC이 country별 필터로 래핑
   - Skill은 공휴일 DB 직접 접근 금지, `calendar.query` tool을 경유
7. **수동 on/off API**:
   - `goose skill region enable --country KR`
   - `goose skill region disable --skill korean-holidays`
   - 상태는 CONFIG-001의 `skills.region.disabled` 배열에 저장
8. **LLM prompt 자동 주입**:
   - ONBOARDING-001 완료 시점에 활성 region skills가 QueryEngine system prompt에 자동 포함
   - Skill 본문이 `inline` 모드면 system prompt에 부분 삽입
   - `fork` 모드면 해당 skill을 소비하는 sub-agent spawn 시 전달
9. **사용자 수정 경로**:
   - 사용자가 `~/.goose/skills/region/kr/my-custom/SKILL.md` 작성하면 번들 skill과 병합
   - 충돌 시 사용자 skill 우선

### 3.2 OUT OF SCOPE

- **각 Skill의 상세 본문 작성**: 스켈레톤만 초기 번들, 콘텐츠는 커뮤니티 + 후속 PR.
- **공휴일 DB 자체 구현**: SCHEDULER-001이 `rickar/cal/v2` 확장 담당.
- **결제 API 실제 통합**: Venmo/카카오페이/Pix 등의 API 연동은 별도 SPEC(Phase 9+).
- **메신저 봇 실제 연결**: GATEWAY-001.
- **Skill 마켓플레이스 UI**: PLUGIN-001.
- **Skill 품질 자동 평가**: v2+.
- **번역된 Skill 본문**: 초기 번들은 각 국가 언어로 작성(ko country skill은 한국어, ja는 일본어). 다국어 번역은 I18N-001 책임 외(skill 본문 자체가 문화적).

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-RS-001 [Ubiquitous]** — The `SkillFrontmatter.Locales` field **shall** accept values from the following namespaces: ISO 3166-1 alpha-2 (e.g., `KR`, `JP`), region group aliases with `region:` prefix (e.g., `region:eu`, `region:ar-world`), and the wildcard `*` (all countries). Other values **shall** cause `ErrUnsafeFrontmatterProperty` (SKILLS-001 REQ-SK-001).

**REQ-RS-002 [Ubiquitous]** — When `Locales` is empty or absent on a skill, the activator **shall** treat the skill as country-agnostic (always eligible, subject to other SKILLS-001 triggers like `paths:`).

**REQ-RS-003 [Ubiquitous]** — The region skill bundle directory **shall** follow the path template `.claude/skills/region/{country_code_lowercase}/{skill_name}/SKILL.md` for per-country skills and `.claude/skills/region/_groups/{group_name}/{skill_name}/SKILL.md` for group-aliased skills.

**REQ-RS-004 [Ubiquitous]** — Each region skill **shall** include a `description` field explicitly stating the target country or region in the user's configured language (ko skills describe in 한국어, jp skills in 日本語, etc).

### 4.2 Event-Driven

**REQ-RS-005 [Event-Driven]** — **When** `LocaleAwareActivator.ActiveSkillIDs(loc)` is invoked with `LocaleContext{country: "KR"}`, the activator **shall** return all skill IDs whose `Locales` field includes `KR`, `region:asia-pacific` (if defined), or `*`.

**REQ-RS-006 [Event-Driven]** — **When** ONBOARDING-001 completes with `country = "JP"`, the activator **shall** automatically enable all `locales: [JP]` skills in the user's configuration, unless the user explicitly excluded them in Step 6 (Ritual Preferences).

**REQ-RS-007 [Event-Driven]** — **When** the user changes their country in CONFIG-001 (via `goose locale set --country VN`), the activator **shall** (a) disable skills tied to the old country, (b) enable skills tied to the new country, and (c) notify the user of both changes.

**REQ-RS-008 [Event-Driven]** — **When** a skill's `Locales` uses a `region:` group alias, the matcher **shall** expand the alias at load time using the canonical group table and cache the expansion; runtime lookups use the cached map.

### 4.3 State-Driven

**REQ-RS-009 [State-Driven]** — **While** a user-authored skill in `~/.goose/skills/region/{country}/` shares the same `name` as a bundled skill, the user-authored version **shall** override the bundled version; a WARN log **shall** record the override.

**REQ-RS-010 [State-Driven]** — **While** `LocaleContext.country` is empty (detection failed, no override), the activator **shall** fall back to loading only `Locales: [*]` skills (wildcard); no country-specific skills activate.

**REQ-RS-011 [State-Driven]** — **While** CONFIG-001 `skills.region.disabled` contains a skill ID, the activator **shall** exclude that skill from the returned active list regardless of `Locales` match.

### 4.4 Unwanted Behavior

**REQ-RS-012 [Unwanted]** — **If** a region skill references tools not in `allowed-tools` (e.g., attempts `kakao.send_message` without declaration), **then** the skill loader **shall** reject that skill with `ErrInvalidAllowedTools` at parse time.

**REQ-RS-013 [Unwanted]** — The activator **shall not** leak the user's `LocaleContext.country` to any remote Skill (`_canonical_` prefix) unless the user has explicitly opted in via `skills.region.share_country_remote = true`.

**REQ-RS-014 [Unwanted]** — **If** two bundled region skills have the same `name` (e.g., both `kr/holidays` and a duplicate in `jp/holidays` with `Locales: [KR]`), **then** the loader **shall** log a duplicate-name warning and use the one whose directory matches `Locales` entry (path-based disambiguation).

### 4.5 Optional

**REQ-RS-015 [Optional]** — **Where** a user's `LocaleContext.secondary_language` implies a second country (e.g., ko-KR primary + en-US secondary), the activator **may** offer to enable skills from the secondary country with explicit user confirmation.

**REQ-RS-016 [Optional]** — **Where** SCHEDULER-001 queries for country-specific holidays, the region skill registry **may** return a pre-filtered calendar object to reduce SCHEDULER-001's matching work.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-RS-001 — 한국 사용자의 자동 활성화**
- **Given** `LocaleContext{country: "KR"}`, 번들에 `region/kr/korean-holidays` 및 `region/kr/kakao-talk`
- **When** `ActiveSkillIDs(loc)`
- **Then** 결과 배열에 `korean-holidays`, `kakao-talk` 포함

**AC-RS-002 — 그룹 별칭 (region:eu)**
- **Given** 번들에 `region/_groups/eu/gdpr-strict` (`Locales: [region:eu]`), 사용자 `country="DE"`
- **When** `ActiveSkillIDs(loc)`
- **Then** `gdpr-strict` 포함 (DE는 region:eu 그룹 멤버)

**AC-RS-003 — 국가 변경 시 자동 전환**
- **Given** 초기 `country="KR"`, `korean-holidays` 활성. 사용자가 `goose locale set --country JP` 실행
- **When** activator 재평가
- **Then** `korean-holidays` 비활성, `japanese-holidays` 활성, 알림 표시

**AC-RS-004 — 사용자 override (disabled)**
- **Given** `country="KR"`, CONFIG에 `skills.region.disabled: ["kakao-talk"]`
- **When** `ActiveSkillIDs(loc)`
- **Then** 결과에 `kakao-talk` 제외, `korean-holidays`는 포함

**AC-RS-005 — 사용자 authored skill override**
- **Given** 번들에 `region/kr/korean-holidays`, 사용자가 `~/.goose/skills/region/kr/korean-holidays/SKILL.md` 작성
- **When** `LoadSkillsDir`
- **Then** 사용자 버전이 레지스트리 진입, 번들 버전은 skip, WARN 1건

**AC-RS-006 — 빈 Locales (country-agnostic)**
- **Given** 번들 skill `generic/weather-tips`에 `Locales`가 없음
- **When** 어떤 country든
- **Then** 해당 skill은 항상 `ActiveSkillIDs` 결과에 포함(다른 트리거는 별도)

**AC-RS-007 — Country 미감지 (fallback to wildcard)**
- **Given** `LocaleContext{country: ""}` (감지 실패), 번들에 `generic/*` (Locales: [*]) 및 `region/kr/korean-holidays`
- **When** `ActiveSkillIDs(loc)`
- **Then** `generic/*` 포함, `korean-holidays` 제외

**AC-RS-008 — 알 수 없는 Locale 거부**
- **Given** SKILL.md에 `locales: [XZ]` (존재하지 않는 country)
- **When** 파싱
- **Then** `ErrUnsafeFrontmatterProperty` 또는 신규 `ErrUnknownCountryCode` 반환, skill 미등록

**AC-RS-009 — Tool allowlist 위반 감지**
- **Given** `kakao-talk` skill이 본문에서 `kakao.send_message`를 호출하려 하나 `allowed-tools`에 미선언
- **When** 파싱
- **Then** `ErrInvalidAllowedTools` 반환 (주의: 실제 실행 차단은 TOOLS-001 책임, 본 SPEC은 정적 검증까지)

**AC-RS-010 — Remote skill에 country 미전송**
- **Given** Remote skill `_canonical_weather`, `skills.region.share_country_remote = false` (기본)
- **When** Remote skill 호출
- **Then** HTTP 요청 헤더/본문에 `country` 필드 없음

**AC-RS-011 — EU 그룹 확장**
- **Given** 그룹 테이블에 `region:eu` = [DE, FR, IT, ES, NL, BE, AT, PL, SE, DK, FI, IE, PT, GR, CZ, HU, RO, BG, HR, SK, SI, LT, LV, EE, CY, MT, LU]
- **When** `matcher.expandAlias("region:eu")`
- **Then** 위 27개 country code 모두 반환

**AC-RS-012 — 10+ 국가 스켈레톤 존재**
- **Given** 릴리스 빌드
- **When** `.claude/skills/region/` 스캔
- **Then** kr/, jp/, cn/, us/, vn/, in/, br/, de/ 최소 8개 country 디렉토리 + `_groups/eu/`, `_groups/ar-world/` 존재, 각각 최소 3개 skill 포함

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 SKILLS-001 확장 지점

`SAFE_SKILL_PROPERTIES` 갱신:
```go
var SAFE_SKILL_PROPERTIES = map[string]struct{}{
    // ... 기존 15개 ...
    "locales": {}, // NEW — ISO 3166-1 alpha-2 또는 "region:*" 별칭 또는 "*"
}
```

`SkillFrontmatter` 확장:
```go
type SkillFrontmatter struct {
    // ... 기존 필드 ...
    Locales []string `yaml:"locales,omitempty"` // ["KR"], ["region:eu"], ["*"]
}
```

### 6.2 패키지 레이아웃

```
internal/skill/region/
├── activator.go     # LocaleAwareActivator, ActiveSkillIDs(loc)
├── matcher.go       # country code + group alias 매칭
├── groups.go        # region:eu, region:ar-world 정적 테이블
├── registry.go      # SkillRegistry 래퍼 (region 필터)
└── *_test.go
```

### 6.3 핵심 Go 타입

```go
// LocaleAwareActivator — LOCALE-001의 LocaleContext와 SKILLS-001의 SkillRegistry를 결합.
type LocaleAwareActivator interface {
    ActiveSkillIDs(loc locale.LocaleContext) []string
    IsActive(skillID string, loc locale.LocaleContext) bool
}

// Matcher — locales 필드를 country code로 확장.
type Matcher interface {
    Matches(skillLocales []string, country string) bool
    ExpandAlias(alias string) []string // "region:eu" → [DE, FR, ...]
}

// 공개 함수:
func NewActivator(reg *skill.SkillRegistry, matcher Matcher) LocaleAwareActivator
func DefaultMatcher() Matcher // 그룹 테이블 + 직접 매칭
```

### 6.4 Group Alias 테이블 (정적)

```go
// internal/skill/region/groups.go
var RegionGroups = map[string][]string{
    "region:eu": {
        "AT", "BE", "BG", "HR", "CY", "CZ", "DK", "EE", "FI", "FR",
        "DE", "GR", "HU", "IE", "IT", "LV", "LT", "LU", "MT", "NL",
        "PL", "PT", "RO", "SK", "SI", "ES", "SE",
    },
    "region:ar-world": {
        "SA", "AE", "EG", "JO", "LB", "MA", "TN", "DZ", "IQ", "OM",
        "QA", "KW", "BH", "YE", "SY", "LY", "SD", "PS",
    },
    "region:latam": {
        "MX", "BR", "AR", "CO", "CL", "PE", "VE", "EC", "GT", "CU",
        "BO", "DO", "HN", "PY", "SV", "NI", "CR", "PA", "UY",
    },
    "region:sea": {
        "VN", "TH", "ID", "MY", "PH", "SG", "KH", "LA", "MM", "BN", "TL",
    },
    "region:east-asia": {"KR", "JP", "CN", "TW", "HK", "MO", "MN"},
}
```

### 6.5 Matcher 알고리즘

```go
func (m *matcher) Matches(skillLocales []string, country string) bool {
    if len(skillLocales) == 0 {
        return true // country-agnostic skill
    }
    if country == "" {
        for _, l := range skillLocales {
            if l == "*" {
                return true // wildcard only matches when country unknown
            }
        }
        return false
    }
    for _, l := range skillLocales {
        switch {
        case l == "*":
            return true
        case l == country:
            return true
        case strings.HasPrefix(l, "region:"):
            if contains(RegionGroups[l], country) {
                return true
            }
        }
    }
    return false
}
```

### 6.6 Skeleton SKILL.md 예시 (kr/korean-holidays)

```markdown
---
name: korean-holidays
description: "한국 공휴일 + 현충일 + 수능 D-day 인식 및 안내"
locales: [KR]
effort: L1
context: inline
allowed-tools: [calendar.query, web.search]
---

# 한국 공휴일 스킬

사용자가 한국에 거주할 때 GOOSE가 알아야 할 공휴일·기념일 정보.

## 국경일 (5개)

- 3월 1일: 삼일절
- 7월 17일: 제헌절 (공휴일 아님, 기념일)
- 8월 15일: 광복절
- 10월 3일: 개천절
- 10월 9일: 한글날

## 법정공휴일

- 1월 1일: 신정
- 설날 3일간 (음력 기준, `calendar.query(kind: "lunar")`로 조회)
- 석가탄신일 (음력 4월 8일)
- 어린이날 (5월 5일, 대체공휴일 적용)
- 현충일 (6월 6일)
- 추석 3일간 (음력 기준)
- 크리스마스 (12월 25일)

## 준공휴일 및 기념일

- 어버이날 (5월 8일)
- 스승의 날 (5월 15일)
- 수능일 (11월 셋째 주 목요일, 매년 `calendar.query(kind: "national_exam")` 조회)
- 연인의 날: 밸런타인(2/14), 화이트데이(3/14), 빼빼로데이(11/11)

## 행동 규칙

- 공휴일 전날 저녁에 "내일 쉬는 날이네요" 인사
- 수능 D-day 100일부터 카운트다운 (수험생 사용자에 한함)
- 현충일은 조용한 어조(조기 게양일)
- 명절 1주 전부터 귀성/교통 안내 제안
```

### 6.7 초기 번들 스켈레톤 목록 (30+ skills)

| 국가 | Skill 이름 (3종) |
|------|------------------|
| kr | korean-holidays, kakao-talk, jondaetmal-etiquette |
| jp | japanese-holidays, line-messenger, keigo-etiquette |
| cn | chinese-lunar-calendar, wechat-etiquette, chunjie-traditions |
| us | us-federal-holidays, tipping-culture, tax-season-aware |
| vn | tet-lunar-new-year, zalo-messenger, vn-honorifics |
| in | diwali-holi, upi-payments, caste-neutral-etiquette |
| br | carnaval, pix-payments, pt-br-spelling |
| de | feiertage, sie-du-etiquette, datenschutz-aware |
| `_groups/eu` | gdpr-strict, sepa-payments, metric-units |
| `_groups/ar-world` | ramadan-awareness, hijri-calendar, halal-dietary |

합계: 8국 × 3 + 2그룹 × 3 = **30 스켈레톤**.

### 6.8 LLM Prompt 자동 주입 규약

ONBOARDING-001 완료 후 QueryEngine의 system prompt 빌드 로직:

```
<locale-context>
{LOCALE-001의 BuildSystemPromptAddendum 결과}
</locale-context>

<region-skills-active>
- skill: korean-holidays
  summary: "한국 공휴일 및 기념일 인식"
- skill: kakao-talk
  summary: "카카오톡 메시지 스타일 이해"
- skill: jondaetmal-etiquette
  summary: "존댓말 기본 + 반말 허용 시점 판단"
</region-skills-active>
```

각 inline skill의 full body는 effort level(L1 = ~200 tokens)에 따라 부분만 주입. L2+ 이상은 tool search 경로로 lazy 로드.

### 6.9 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| Skill 로더 확장 | SKILLS-001 자체 | 변경 최소화 |
| Group alias | Go map 정적 테이블 | 외부 라이브러리 불필요 |
| 공휴일 DB | `github.com/rickar/cal/v2` | SCHEDULER-001 공유 |
| ISO 3166 검증 | `github.com/biter777/countries` v1.7+ | code validity 확인 |
| 로깅 | zap | 공유 |

### 6.10 TDD 진입 순서

1. **RED #1** — `TestMatcher_DirectCountry` → AC-RS-001
2. **RED #2** — `TestMatcher_GroupAlias_EU` → AC-RS-002, AC-RS-011
3. **RED #3** — `TestMatcher_Wildcard` → AC-RS-006
4. **RED #4** — `TestMatcher_EmptyCountry_WildcardOnly` → AC-RS-007
5. **RED #5** — `TestActivator_UserOverride_Disabled` → AC-RS-004
6. **RED #6** — `TestActivator_UserAuthored_OverridesBundle` → AC-RS-005
7. **RED #7** — `TestFrontmatter_UnknownCountry_Rejected` → AC-RS-008
8. **RED #8** — `TestCountrySwitch_SkillsToggled` → AC-RS-003
9. **RED #9** — `TestRemoteSkill_NoCountryLeakage` → AC-RS-010
10. **GREEN** — 최소 구현
11. **REFACTOR** — group 확장 결과 memoize

### 6.11 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | 테이블 테스트 (20+ country × 30+ skill), CI에서 초기 번들 30개 스켈레톤 전부 parse 성공 검증 |
| **R**eadable | region skills은 각 country 언어로 작성(한국 skill은 한국어) → 로컬 검토자 이해도↑ |
| **U**nified | SKILLS-001 스키마 상속, `locales` 필드 명명 규칙 문서화 |
| **S**ecured | country → remote 전송 차단(REQ-RS-013), allowed-tools 정적 검증(REQ-RS-012) |
| **T**rackable | `region:` 그룹 별칭 확장 결과를 DEBUG 로그에 출력, skill load 이벤트에 country 태그 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-SKILLS-001** | frontmatter 스키마 상속 + allowlist 확장 |
| 선행 SPEC | **SPEC-GOOSE-LOCALE-001** | `LocaleContext.country` 소비 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `skills.region.*` 섹션 |
| 선행 SPEC | SPEC-GOOSE-I18N-001 | skill description의 언어별 번역 지원(선택) |
| 후속 SPEC | SPEC-GOOSE-SCHEDULER-001 | 공휴일 DB country 기반 선택 |
| 후속 SPEC | SPEC-GOOSE-ONBOARDING-001 | Step 2 완료 시 자동 활성화 |
| 후속 SPEC | SPEC-GOOSE-TOOLS-001 | allowed-tools 정적 검증 연계 |
| 외부 | `rickar/cal/v2` | 공휴일 DB |
| 외부 | `biter777/countries` v1.7+ | ISO 3166-1 검증 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 10+ 국가 스켈레톤 콘텐츠가 평균적으로 부실하면 사용자 체감 품질 저하 | 고 | 고 | 초기 번들은 kr/jp/cn/us 4개국만 "완성", 나머지는 "베타" 배지 노출 |
| R2 | region:eu 그룹에 Brexit 후 영국(GB) 미포함 논란 | 중 | 낮 | `region:eu` = EU 회원국만, 영국은 `region:uk-commonwealth` 별도 |
| R3 | 문화적 편향(예: 중국 `wechat-etiquette`가 정치 민감 주제 회피) | 중 | 중 | skill 본문 리뷰어 CODEOWNERS, 정치 주제 guard 선언 |
| R4 | 사용자 country 원격 전송 의도치 않은 누출 | 중 | 고 | REQ-RS-013 + AC-RS-010 자동 테스트 |
| R5 | ISO 3166 코드 변경(예: 러시아 영토 분쟁) | 낮 | 낮 | `biter777/countries` 업데이트 주기 모니터 |
| R6 | 공휴일 DB 라이센스(rickar/cal/v2는 MIT) 변경 | 낮 | 중 | MIT 고정 commit pin |
| R7 | 사용자 authored skill이 번들 skill을 악의적으로 override | 중 | 중 | WARN 로그 필수(REQ-RS-009), CLI `goose skill region list --source`로 노출 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/adaptation.md` §4.2 문화별 뉘앙스, §4.3 명절 & 기념일
- `.moai/project/branding.md` §3.3 문화적 뉘앙스
- `.moai/specs/SPEC-GOOSE-SKILLS-001/spec.md` — 기반 스키마
- `.moai/specs/SPEC-GOOSE-LOCALE-001/spec.md` — country 소스
- `.moai/specs/SPEC-GOOSE-SCHEDULER-001/spec.md` — 공휴일 DB 소비자

### 9.2 외부 참조

- ISO 3166-1 alpha-2 country codes
- Unicode CLDR territory groups (UN M.49 regions)
- EU official member states list
- Arab League member states
- `rickar/cal/v2` documentation
- `biter777/countries` documentation

### 9.3 부속 문서

- `./research.md` — 10+ 국가별 문화 원천 자료, 공휴일 매핑 검증, skill 콘텐츠 규칙
- `../SPEC-GOOSE-SKILLS-001/spec.md`
- `../SPEC-GOOSE-LOCALE-001/spec.md`
- `../SPEC-GOOSE-ONBOARDING-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **각 Skill의 상세 본문을 완성하지 않는다**. 30+ 스켈레톤만 제공, 완성은 커뮤니티 PR.
- 본 SPEC은 **공휴일 DB 자체를 구현하지 않는다**(SCHEDULER-001).
- 본 SPEC은 **결제 API(Venmo/카카오페이/Pix)를 실제 연동하지 않는다**. 스킬은 인식/안내만.
- 본 SPEC은 **메신저 API(카카오톡/WeChat/LINE)를 실제 연결하지 않는다**(GATEWAY-001).
- 본 SPEC은 **Skill 마켓플레이스 UI를 포함하지 않는다**(PLUGIN-001).
- 본 SPEC은 **Skill 품질 자동 평가를 수행하지 않는다**.
- 본 SPEC은 **PIPL/GDPR/PIPA 실제 준수 로직을 구현하지 않는다**. `legal_flags`가 설정된 skill이 "이 법률을 고려하라"고 LLM에 지시할 뿐.
- 본 SPEC은 **SaaS 번역 관리 툴과 통합하지 않는다**.
- 본 SPEC은 **정치/종교/민감 주제에 대한 자동 회피 로직을 강제하지 않는다**. 각 skill이 자체 guard 문구로 선언.
- 본 SPEC은 **사용자의 country를 원격 서버로 전송하지 않는다**(REQ-RS-013).
- 본 SPEC은 **hot-reload로 skill 본문을 자동 번역하지 않는다**. skill 본문은 국가 언어 원본 유지.

---

**End of SPEC-GOOSE-REGION-SKILLS-001**
