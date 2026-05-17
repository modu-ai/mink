---
id: SPEC-GOOSE-ONBOARDING-001
version: 0.2.2
status: superseded
superseded_by: SPEC-MINK-ONBOARDING-001
created_at: 2026-04-22
updated_at: 2026-05-14
author: manager-spec
priority: critical
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-anchored
labels: [onboarding, cli, web-ui, installer, wizard, localization, post-brand-rename, superseded]
---

# SPEC-GOOSE-ONBOARDING-001 — CLI + Web UI Install Wizard

> **POST-BRAND-RENAME NOTICE (2026-05-14)**: 본 SPEC 은 SPEC-MINK-BRAND-RENAME-001 (commit f0f02e4, 2026-05-13) 이전에 작성된 draft 이다. 본문 곳곳에 `mink init` / `./.goose/` 같은 MINK 명칭이 남아 있으며, 후속 implementation 진입 시 다음 중 하나로 처리해야 한다.
>
> 1. **MINK 로 rebrand** — id `SPEC-MINK-ONBOARDING-001` 신설 (CLI 명령 `mink init`, 디렉토리 `~/.mink/`), 본 SPEC 은 status=superseded
> 2. **본문 내 MINK 치환** — id 유지, 본문 goose → mink 치환 (BRAND-RENAME-001 + ENV-MIGRATE-001 + USERDATA-MIGRATE-001 시리즈와 align)
>
> 후속 implementation 진입 직전에 결정. 본 marker 가 추가되기 전까지 본 SPEC 은 "draft, awaiting brand-rename decision" 상태이다.

> **v0.2 Amendment (2026-04-24, propagated 2026-04-25)**: SPEC-GOOSE-ARCH-REDESIGN-v0.2 반영.
> **제거**: 이메일 가입/로그인 플로우, 모바일 디바이스 페어링 단계, Apple Native 초기 설정, Desktop Tauri 8-step 전체 플로우.
> **유지/추가**: **`mink init` CLI 마법사** + **Web UI 설치·설정 마법사** (비개발자 대응).
> **초기 설정 범위**: `./.goose/` 디렉터리 생성 → `persona/soul.md` 입력 → provider key 저장 (OS keyring) → 첫 messenger 채널 활성화.
> 본 v0.2는 Amendment 선언 후 본문 전체를 재구성한 결과이며, 기존 Desktop Tauri 플로우 관련 REQ/AC는 `[DEPRECATED v0.2]` 주석으로 보존한다 (번호 재배치 금지 원칙).

---

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성. v5.0 ROADMAP Phase 6 Localization 시리즈 4번째(최종). 사용자 지시: "설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게". Desktop App 첫 실행 시 8단계 UX로 locale + identity + daily pattern + interests + ritual + LLM provider + privacy 수집. | manager-spec |
| 0.2.1 | 2026-05-14 | POST-BRAND-RENAME marker 추가. BRAND-RENAME-001 (commit f0f02e4) 이후 MINK prefix draft 의 후속 처리 (rebrand vs 본문 치환) 미결정. labels 에 `post-brand-rename` 추가. | manager-spec |
| 0.2.2 | 2026-05-14 | Superseded by SPEC-MINK-ONBOARDING-001 (rebrand 옵션 c 선택). frontmatter status=draft → superseded, superseded_by 추가, labels 에 `superseded` 추가. 본문 body 는 immutable 로 유지 (BRAND-RENAME-001 OUT-scope 정책). 후속 implementation 은 SPEC-MINK-ONBOARDING-001 에서 진행. | manager-spec |
| 0.2.0 | 2026-04-25 | SPEC-GOOSE-ARCH-REDESIGN-v0.2 Amendment를 본문 전반에 전파. 스코프를 Desktop Tauri 8-step → CLI(`mink init`) + Web UI 설치 마법사로 축소. REQ-OB-013(Ritual soft notice)와 REQ-OB-018(Mobile QR 페어링)는 `[DEPRECATED v0.2]`로 표시(번호 보존). §1~§3 및 §6 본문 재작성, §5를 "Test Scenarios"로 재명명 + 각 AC에 REQ 명시 참조 추가. AC-OB-014 로케일 문자열 프랑스어로 수정(REQ-OB-003 정합). 5개 미커버 REQ(014/015-invalid/016/019)에 대응하는 AC 신설(AC-OB-017~020). D8(수동 측정) → Playwright 자동화 참조로 수정. Exclusions L657(CLI 온보딩 금지)을 CLI IN SCOPE로 교체(D11 해소). | manager-spec |

---

## 1. 개요 (Overview)

사용자 최종 지시(2026-04-22):

> "설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게 각 현지화된 스킬들을 추가하도록 하자."

v0.2 Amendment 반영(2026-04-24):

> "Apple Native 초기 설정 · 모바일 디바이스 페어링 · 이메일 가입을 제거하고, **`mink init` CLI 마법사**와 **Web UI 설치 마법사**로 비개발자도 3분 내 설정을 마칠 수 있게 한다."

본 SPEC은 **MINK 첫 설치 시 사용자를 맞이하는 CLI + Web UI 온보딩 마법사**를 정의한다. 두 경로는 동일한 backend 상태 머신(`internal/onboarding`)을 공유하며, 동일한 최종 산출물(`./.goose/` 디렉터리 트리)을 생성한다.

### 1.1 공통 수집 대상 (5-Step 축소)

| Step | 데이터 | 소비 SPEC |
|------|-------|----------|
| 1. Welcome + Locale | country/language/timezone | LOCALE-001, I18N-001, REGION-SKILLS-001 |
| 2. Persona | name, honorific_level, soul.md 본문 | IDENTITY-001, ADAPTER-001 |
| 3. Provider | llm_provider, api key (OS keyring) | CREDPOOL-001, ROUTER-001 |
| 4. Messenger Channel | 첫 채널 선택 (local/terminal, slack, telegram 등) | MESSENGER-*, BRIDGE-001 |
| 5. Privacy & Consent | consent flags | MEMORY-001, LORA-001 (opt-in) |

목표 소요 시간: **3분 이하** (CLI 경로 ≤ 2분, Web UI 경로 ≤ 3분). 각 단계는 **스킵 가능**(기본값 적용) + **뒤로 가기** + **진행 표시** 제공.

### 1.2 두 경로 대비

| 측면 | `mink init` CLI | Web UI Wizard |
|-----|-----------------|---------------|
| 진입점 | 쉘에서 `mink init` 실행 | `mink serve --install-wizard` 실행 후 브라우저 열림 (`http://localhost:5173/install`) |
| UI 렌더러 | `charmbracelet/huh` 기반 TUI 폼 | React + shadcn/ui |
| 대상 사용자 | 개발자, 엔지니어 | 비개발자, 가족/친지에게 배포 | 
| progress 표시 | "[2/5]" 텍스트 진행도 | 상단 progress bar (5 단계) |
| API key 입력 | stdin 숨김 입력(tty echo off) + OS keyring | HTML password input + fetch → Go server → OS keyring |
| 비정상 종료 재개 | `mink init --resume` | `/install?resume=1` |

완료 후 공통 산출물:

- `./.goose/config.yaml` (providers, locale, consent flags)
- `./.goose/persona/soul.md`
- OS keyring 항목: `goose.provider.{name}.api_key`
- `./.goose/onboarding-completed` 타임스탬프 파일

---

## 2. 배경 (Background)

### 2.1 왜 CLI + Web UI 이원화인가 (Amendment rationale)

v0.1에서 Desktop Tauri 8-step을 IN SCOPE로 정의했으나, v0.2 Architecture Redesign 결과:

- Desktop Tauri는 설치 마찰이 큼 (macOS 공증, Windows code-signing 비용).
- 이메일/모바일 페어링은 MINK 핵심가치(로컬 우선)와 불일치.
- **가장 단순한 설치 경로**: 터미널 한 줄 또는 브라우저 한 탭.

따라서:
- **CLI(`mink init`)** — 개발자가 `curl | sh` 설치 직후 즉시 실행 가능. 서버 환경에서도 동작.
- **Web UI** — `mink serve --install-wizard`로 로컬 서버 기동 후 자동 브라우저 오픈. 터미널에 익숙지 않은 가족 · 지인도 사용 가능.

양 경로의 backend(`internal/onboarding`)는 **100% 공유**하여 산출물 동일성 보장.

### 2.2 최소 5단계 설계

Amendment가 설정한 scope("`./.goose/` 생성 → persona → provider key → 첫 messenger 채널")를 충족하는 최소 단계:

1. **Welcome + Locale** — 환영 + LOCALE-001 감지 + 필요 시 override
2. **Persona** — name/호칭 + `soul.md` 본문 (prefill 템플릿 제공)
3. **Provider** — LLM 선택 + API key / OAuth (OS keyring 저장)
4. **Messenger Channel** — 첫 채널 (local terminal/chat, 또는 외부 messenger 바인딩)
5. **Privacy & Consent** — 데이터 수집 범위 + telemetry opt-in

한 단계 스킵 시 후속 기능이 저하되지만, **전 단계 모두 스킵해도 `./.goose/` 초기화는 완료**되어야 한다 (MINK를 일단 띄우는 것이 최우선).

### 2.3 branding.md §5.1 First Day with MINK

> "Hatching 🐣 — 알에서 깨어나는 최초의 만남. Goose가 사용자를 발견하고 각인(imprinting)하는 순간."

각인(imprinting) 메타포는 유지: Step 2(Persona) 완료 시 MINK mood가 `calm → imprinting → curious`로 전환. CLI에서는 ASCII 아트 전환, Web UI에서는 간단한 CSS transition (prefers-reduced-motion 시 fade only).

### 2.4 법적 제약

- **GDPR** (EU): 명시적 동의 필수, withdrawal 권리 고지
- **PIPA** (KR): 민감정보 최소 수집
- **CCPA** (US): do-not-sell 옵션
- **LGPD** (BR), **PIPL** (CN), **FZ-152** (RU): country별 legal flags는 LOCALE-001이 제공, Step 5가 그에 따라 consent 문구 조정

### 2.5 범위 경계

- **IN**: 5단계 UI (CLI TUI + Web UI React), Go backend 상태 머신, CONFIG-001 저장, LOCALE/I18N/REGION-SKILLS 초기화 호출, OS keyring 저장.
- **OUT**: LLM provider OAuth 상세(CREDPOOL-001), Identity Graph 완전 구축(IDENTITY-001), LoRA 훈련 자체(LORA-001), 법률 문구 전문(외부 검토), Daily Pattern/Ritual Preferences 수집(v1.0+ Preferences에서).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **CLI 경로 (`mink init`)** — `cmd/goose/cmd/init.go`:
   - `charmbracelet/huh` 기반 TUI 폼 (5-Step)
   - tty echo off 상태에서 API key 입력
   - `--resume` 플래그로 draft 재개
   - `--yes` 플래그로 완전 비대화(모든 기본값 수용 — CI/Docker 환경)
   - 키보드 단축키: Tab(다음 필드), Shift+Tab(이전), Esc(스킵), Ctrl+C(중단 및 draft 저장)
2. **Web UI 경로** — `cmd/goose/cmd/serve.go --install-wizard` + `web/install/`:
   - React + Vite + shadcn/ui (경량, 추가 컴파일 옵션 없이 Go embed)
   - 5개 React 컴포넌트 (Step1Welcome, Step2Persona, Step3Provider, Step4Messenger, Step5Privacy)
   - Zustand store로 수집 데이터 임시 보관 (클라이언트 측)
   - 상단 progress bar (5 단계), 뒤로 가기 / 스킵 / 다음 버튼
   - 완료 시 서버가 종료 신호 수신 → 브라우저에 "설정 완료, 터미널로 돌아가세요" 표시
3. **Backend (Go, 공통)** — `internal/onboarding/`:
   - `flow.go` — `OnboardingFlow` 상태 머신 (CLI/Web UI 공유)
   - `steps.go` — 5-Step 정의 + validation
   - `progress.go` — draft `./.goose/onboarding-draft.yaml` 저장/로드
   - `completion.go` — 완료 처리 (후속 SPEC 초기화 호출)
   - `keyring.go` — OS keyring 추상 (`zalando/go-keyring` 래핑)
4. **Web UI HTTP 엔드포인트** — `internal/server/install/`:
   - `GET /install` — SPA 진입
   - `POST /install/step/:n` — 단계 제출
   - `POST /install/skip/:n` — 단계 스킵
   - `POST /install/back` — 이전 단계
   - `POST /install/complete` — 최종 완료
   - `GET /install/status` — 현재 draft 상태
5. **Step 1 (Welcome + Locale)** 상세:
   - LOCALE-001의 `Detect()` 결과 표시 (country/language/timezone)
   - Dropdown 또는 텍스트 입력(CLI)으로 override 가능
   - Conflict 존재 시(OS vs IP) 명시 표시 + 사용자 선택 강제
   - "Apply and continue" 시 `LocaleContext` 저장 + I18N 즉시 적용
6. **Step 2 (Persona)** 상세:
   - Name (필수), Preferred Honorific Level (한국어/일본어일 때: 존댓말/해요체/반말)
   - `soul.md` 본문 (CLI: `$EDITOR` 기동 / Web UI: `<textarea>` multi-line, 템플릿 prefill)
   - LOCALE-001의 `CulturalContext.formality_default`를 기본값으로 제안
7. **Step 3 (Provider)** 상세:
   - 리스트 UI: Anthropic / OpenAI / Google / Ollama / DeepSeek / Custom
   - Anthropic / OpenAI: OAuth 버튼(Web UI: 브라우저 새 탭 / CLI: URL 복사 + pastebin) + API key 대안
   - Ollama: localhost 감지, 모델 자동 나열
   - Custom: URL + API key + model name
   - API key 저장: OS keyring (`zalando/go-keyring` — macOS Keychain / Windows Credential Manager / Linux Secret Service)
   - "Skip and configure later" 옵션 → `llm.default_provider: "unset"`
8. **Step 4 (Messenger Channel)** 상세:
   - 첫 채널 선택: Local Terminal / Slack / Telegram / Discord / Custom
   - Local Terminal은 즉시 활성화(추가 입력 없음).
   - 외부 messenger는 bot token 입력 (OS keyring 저장).
   - "Skip" 시 Local Terminal만 활성.
9. **Step 5 (Privacy & Consent)** 상세:
   - 체크박스:
     - [x] 대화 기록 로컬 저장 (필수, 기본 ON)
     - [ ] LoRA 개인 모델 훈련에 사용 (opt-in, 기본 OFF)
     - [ ] Anonymous telemetry (opt-in, 기본 OFF)
     - [ ] 오류 보고 자동 전송 (opt-in, 기본 OFF)
   - GDPR 국가이면 "I explicitly consent" 명시적 체크 강제
   - Submit 시 `ConsentFlags`를 `./.goose/config.yaml`에 저장
10. **완료 처리**:
    - CLI: ASCII "🐣 → 🐥" 아트 전환 + "Goose is ready. Run `mink start`." 메시지
    - Web UI: 간단한 egg→mink transition (prefers-reduced-motion 존중), "터미널로 돌아가세요" 메시지
    - REGION-SKILLS-001 `ActivateForCountry(country)` 호출
    - Identity Graph 초기 노드 seed (IDENTITY-001 public API)
    - `./.goose/onboarding-completed` 타임스탬프 기록
11. **3분 목표 수행 측정**:
    - 각 단계 소요 시간 이벤트 로깅 (telemetry opt-in 후에만)
    - 완료율, 중도 이탈율 추적

### 3.2 OUT OF SCOPE

- **LLM OAuth 상세 플로우**: CREDPOOL-001 전담. 본 SPEC은 "OAuth 시작" 이벤트만 발신.
- **Identity Graph 완전 구축**: 본 SPEC은 seed 노드만 생성.
- **LoRA 훈련 자체**: LORA-001. 본 SPEC은 consent flag 저장에 한정.
- **Messenger 연결 상세 프로토콜**: 각 MESSENGER-* SPEC. 본 SPEC은 bot token 저장 + "첫 채널 활성" 신호까지.
- **법률 문구 전문**: 외부 법률 검토 후 `docs/privacy-policy.md`에 주입. 본 SPEC은 링크만.
- **Desktop Tauri 경로**: v0.1에서 IN SCOPE였으나 v0.2 Amendment로 제거. Tauri 패키징은 v1.0+에서 재평가.
- **모바일 디바이스 페어링**: v0.1에서 Step 7/8에 포함되었으나 v0.2 Amendment로 제거. BRIDGE-001 별도 SPEC에서 다룸.
- **이메일 가입/로그인**: v0.1에 없었고 v0.2 Amendment에서도 명시적으로 제외.
- **재온보딩 플로우**: 1회성. 추후 "Preferences > Re-run onboarding"(v1.0+).
- **Daily Pattern/Interests/Ritual 수집**: v0.1 Step 4/5/6이었으나 v0.2 Amendment에서 제거. 추후 Preferences 화면에서 선택적 수집.
- **A/B 테스트**: v2+.

---

## 4. EARS 요구사항 (Requirements)

> 주: REQ-OB-001~019 번호는 v0.1의 것을 보존한다. v0.2 Amendment로 스코프에서 제거된 REQ는 `[DEPRECATED v0.2]` 주석으로 표시하며 본문에서 제거하지 않는다(번호 재배치 금지 원칙).

### 4.1 Ubiquitous

**REQ-OB-001 [Ubiquitous]** — The onboarding flow **shall** complete in 5 steps (CLI and Web UI identically) and **shall** present a progress indicator (CLI: "[n/5]" text, Web UI: top progress bar) at all times.

**REQ-OB-002 [Ubiquitous]** — Every onboarding step **shall** provide three actions: `Next` (submit + advance), `Back` (return to previous step, disabled on Step 1), and `Skip` (apply default values + advance).

**REQ-OB-003 [Ubiquitous]** — The onboarding UI **shall** be rendered in the language determined by LOCALE-001's `primary_language` as soon as Step 1 (Locale) completes; prior to Step 1, the UI **shall** use the OS-detected language.

**REQ-OB-004 [Ubiquitous]** — All user inputs **shall** be validated before advancing to the next step; validation errors **shall** display inline with field-specific messages in the user's language (matching LOCALE-001 `primary_language`).

### 4.2 Event-Driven

**REQ-OB-005 [Event-Driven]** — **When** the user invokes `mink init` in a fresh environment (no `./.goose/config.yaml` exists or `onboarding_completed` is absent) **or** opens the Web UI install wizard (`mink serve --install-wizard`), the system **shall** start the onboarding flow at Step 1 rather than the main application shell.

**REQ-OB-006 [Event-Driven]** — **When** Step 1 (Locale) is submitted, the backend **shall** call LOCALE-001's override API to persist the user's choice and **shall** immediately reload the I18N bundles for the new language (CLI re-renders current screen; Web UI fetches new locale bundle).

**REQ-OB-007 [Event-Driven]** — **When** Step 3 (Provider) is submitted with an API key, the backend **shall** store the secret in the OS keyring via `zalando/go-keyring` under entry `goose.provider.{name}.api_key` and **shall not** write plaintext secrets to `./.goose/config.yaml`.

**REQ-OB-008 [Event-Driven]** — **When** Step 5 (Privacy) is submitted and the user's country is in the EU (per LOCALE-001 `legal_flags`), the form **shall** require an explicit `I consent` checkbox (Web UI) or explicit `y` confirmation (CLI) before allowing completion; `Skip` is not permitted in GDPR regions for consent-required fields.

**REQ-OB-009 [Event-Driven]** — **When** the user completes Step 5, the backend **shall** (a) persist all collected data to `./.goose/config.yaml` (CONFIG-001), (b) call REGION-SKILLS-001 to activate country skills, (c) seed Identity Graph initial nodes, (d) write `./.goose/onboarding-completed` timestamp file, and (e) exit the wizard (CLI returns to shell; Web UI signals server shutdown and displays "return to terminal").

**REQ-OB-010 [Event-Driven]** — **When** the user clicks `Skip` on Step 3 (Provider), the backend **shall** record `llm.default_provider: "unset"`, and the first invocation of the main application **shall** display a non-blocking notice prompting the user to configure a provider later.

### 4.3 State-Driven

**REQ-OB-011 [State-Driven]** — **While** the user is mid-onboarding and quits (CLI: Ctrl+C; Web UI: tab closed), the collected data (partial) **shall** be persisted to `./.goose/onboarding-draft.yaml`, and re-running `mink init --resume` or reopening the Web UI **shall** resume from the last completed step.

**REQ-OB-012 [State-Driven]** — **While** LOCALE-001's `Detect()` returned `LocaleConflict` (OS vs IP mismatch), Step 1 **shall** display both values and require the user to choose one before advancing.

**REQ-OB-013 [State-Driven]** `[DEPRECATED v0.2]` — (v0.1: "While the user is on Step 6 (Rituals) and has unchecked all three ritual options, the UI shall display a soft notice.") v0.2 Amendment에서 Ritual 수집 단계 자체가 제거되었으므로 본 요구는 비활성. 번호는 보존한다. 추후 v1.0 Preferences 화면 SPEC에서 재도입될 수 있음.

### 4.4 Unwanted Behavior

**REQ-OB-014 [Unwanted]** — The onboarding flow **shall not** transmit any user input to external servers **except**: (a) Step 3 LLM OAuth redirect (user-initiated, visible browser URL), (b) Step 5 telemetry `POST` (only after explicit opt-in checkbox). All other network I/O during onboarding is prohibited.

**REQ-OB-015 [Unwanted]** — **If** the user enters an API key that fails validation (malformed prefix for the selected provider — e.g., Anthropic requires `sk-ant-`, OpenAI requires `sk-`), **then** the form **shall** display an error and **shall not** store the invalid key in the OS keyring or anywhere else.

**REQ-OB-016 [Unwanted]** — The onboarding flow **shall not** request sensitive data fields outside the enumerated whitelist. Allowed fields: `name`, `honorific_level`, `pronouns` (optional), `soul.md` (free text), `locale choice`, `provider choice`, `api_key` (keyring-only), `messenger channel`, `consent flags`. Prohibited: SSN, biometric templates, medical records, government ID numbers, phone number, physical address, email (per v0.2 Amendment).

**REQ-OB-017 [Unwanted]** — **If** Step 2 (Persona) is submitted with a `name` field containing 500+ characters or shell/HTML injection patterns (per regex `[<>&{};|$]` or length > 500), **then** the backend **shall** reject the submission and log a security event to `./.goose/security-events.log`.

### 4.5 Optional

**REQ-OB-018 [Optional]** `[DEPRECATED v0.2]` — (v0.1: "Where the user has successfully completed Step 7 (LLM provider) with a valid provider, the onboarding flow may offer to pair with a mobile device at the end via QR code.") v0.2 Amendment에서 모바일 디바이스 페어링이 스코프에서 제거되었으므로 본 요구는 비활성. 번호는 보존한다. 모바일 페어링은 BRIDGE-001에서 별도 SPEC으로 다룬다.

**REQ-OB-019 [Optional]** — **Where** accessibility features are enabled (Web UI: `prefers-reduced-motion` media query true, or OS high-contrast setting / CLI: `NO_COLOR` env var set or `$TERM=dumb`), the onboarding UI **shall** respect those preferences: no autoplay animations, WCAG 2.1 AA contrast (Web UI), plain text output without ANSI escapes (CLI).

---

## 5. Test Scenarios (Given-When-Then)

> **Format declaration**: 본 섹션은 §4 EARS 요구사항을 검증하기 위한 **테스트 시나리오**를 Given/When/Then 형식으로 작성한다. EARS 규범적 요구는 §4에서 이미 선언되었으며, 본 섹션의 각 시나리오는 해당 REQ를 **verifies** 관계로 명시 참조한다. (v0.1에서는 본 섹션이 "Acceptance Criteria"였으나 MP-2 rubric 준수를 위해 v0.2에서 "Test Scenarios"로 재명명함. 수용 기준 자체는 §4 EARS 블록이 수행한다.)

**AC-OB-001 — 첫 실행 시 온보딩 시작 (verifies REQ-OB-005)**
- **Given** fresh install, `./.goose/config.yaml` 없음
- **When** 사용자가 `mink init` 실행 또는 `mink serve --install-wizard` 후 `http://localhost:5173/install` 접근
- **Then** 메인 앱이 시작되는 대신 Step 1 Welcome + Locale 화면이 표시됨 (CLI: huh 폼 / Web UI: React 컴포넌트)

**AC-OB-002 — 진행 표시 (verifies REQ-OB-001, REQ-OB-002)**
- **Given** 온보딩 진행 중
- **When** Step 2 (Persona) 도착
- **Then** CLI는 "[2/5]" 텍스트 표시, Web UI는 progress bar가 40% 위치로 이동, Back/Skip/Next 3 버튼이 활성 (Step 1에서는 Back 비활성)

**AC-OB-003 — Locale 감지 + 수정 (verifies REQ-OB-006, REQ-OB-003)**
- **Given** LOCALE-001이 `country="KR"` 감지
- **When** Step 1 (Welcome + Locale) 표시
- **Then** "거주 국가: 대한민국" + dropdown(Web UI) 또는 selection list(CLI)으로 변경 가능. 사용자가 "일본"으로 변경 후 Next → CONFIG-001에 `country="JP"` 저장, UI 언어는 한국어→일본어로 즉시 전환

**AC-OB-004 — OS vs IP 충돌 해결 (verifies REQ-OB-012)**
- **Given** `LocaleConflict{os:"KR", ip:"US"}` 감지
- **When** Step 1 표시
- **Then** "OS 설정: 한국 / IP 위치: 미국" 두 라디오 버튼(Web UI) 또는 두 선택지(CLI), 하나 선택 강제 후에만 Next 진행

**AC-OB-005 — 이름 유효성 (verifies REQ-OB-004, REQ-OB-017)**
- **Given** Step 2에서 name 필드 비어있음
- **When** Next 실행
- **Then** 인라인 에러 "이름을 입력해주세요"(사용자 언어 적용), 전진 불가

**AC-OB-006 — Skip 기본값 적용 (Step 4 Messenger) (verifies REQ-OB-002)**
- **Given** Step 4, 사용자가 messenger 선택 안 함
- **When** Skip 실행
- **Then** default `local_terminal` 채널만 활성, Step 5로 진행

**AC-OB-007 — API key 키링 저장 (valid key path) (verifies REQ-OB-007)**
- **Given** Step 3, 사용자가 Anthropic 선택 + 유효한 `sk-ant-xxxx` 입력
- **When** Next 실행
- **Then** OS keyring에 `goose.provider.anthropic.api_key` 저장, `./.goose/config.yaml`에는 `providers.anthropic.api_key_source: keyring` 만 저장(평문 없음)

**AC-OB-008 — GDPR 명시적 동의 (verifies REQ-OB-008)**
- **Given** `country="DE"` (EU), Step 5
- **When** 사용자가 "I consent" 체크(또는 CLI 'y' 확인) 없이 Submit
- **Then** 에러 "Explicit consent required", Submit 차단

**AC-OB-009 — 완료 후 Region Skills 활성화 (verifies REQ-OB-009)**
- **Given** 사용자가 `country="KR"`로 온보딩 완료
- **When** Step 5 Submit
- **Then** REGION-SKILLS-001이 `korean-holidays`, `kakao-talk`, `jondaetmal-etiquette` 자동 활성. CONFIG-001 `skills.region.active` 배열에 3개 ID 등록. `./.goose/onboarding-completed` 파일 생성

**AC-OB-010 — 완료 메시지 + Imprinting 이벤트 (verifies REQ-OB-009, REQ-OB-019)**
- **Given** 온보딩 완료
- **When** Step 5 Submit 후
- **Then** CLI는 ASCII 아트 "🐣 → 🐥" 전환과 "Goose is ready. Run `mink start`." 메시지(prefers reduced motion 시 ASCII 전환 생략 텍스트만). Web UI는 egg→mink transition과 "터미널로 돌아가세요" 메시지(reduced motion 시 fade only). 한국어 사용자에게는 "안녕하세요, [이름]님!" 인사 적용

**AC-OB-011 — 중도 이탈 후 재개 (verifies REQ-OB-011)**
- **Given** Step 2 (Persona) 완료 후 사용자가 Ctrl+C(CLI) 또는 탭 닫기(Web UI)
- **When** `mink init --resume` 또는 `/install?resume=1`
- **Then** `./.goose/onboarding-draft.yaml`에 Step 1~2 데이터 존재, UI는 Step 3부터 재시작

**AC-OB-012 — LLM provider 스킵 (verifies REQ-OB-010)**
- **Given** Step 3에서 Skip
- **When** Step 4로 진행
- **Then** CONFIG-001 `llm.default_provider: "unset"` 저장. 온보딩 완료 후 첫 `mink start` 실행 시 "LLM 공급자를 설정해주세요" 비차단 배너/알림 표시

**AC-OB-013** `[DEPRECATED v0.2]` — (v0.1: "Mobile 페어링 제안") v0.2 Amendment로 모바일 페어링 스코프 제거. 번호 보존, 비활성. 추후 BRIDGE-001에서 다룸.

**AC-OB-014 — Skip 불허 (GDPR consent, French locale) (verifies REQ-OB-008, REQ-OB-003)**
- **Given** `country="FR"`, `primary_language="fr"`, Step 5
- **When** Skip 실행
- **Then** 에러 메시지 프랑스어 "Le consentement explicite ne peut pas être ignoré." 표시, 전진 불가. (REQ-OB-003에 따라 UI 언어가 프랑스어이므로 에러 메시지도 프랑스어.)

**AC-OB-015 — 입력 사니타이징 (이름) (verifies REQ-OB-017)**
- **Given** Step 2 name = `"Hacker<script>alert(1)</script>"`
- **When** Next 실행
- **Then** 정규식 검증 실패, 에러 표시, 저장 안 됨, `./.goose/security-events.log`에 security event 기록

**AC-OB-016 — 3분 이내 완료 가능성 (Playwright 자동화) (verifies REQ-OB-001)**
- **Given** 모든 Skip 사용 + 기본값 수용 (Step 2 name은 필수이므로 "TestUser" 입력)
- **When** Playwright 스크립트 `e2e/install-wizard-speedrun.spec.ts`가 Web UI Step 1 → Step 5 전체를 자동 진행하고, 별도 `scripts/cli-install-speedrun.sh`가 `mink init --yes` 경로를 실행
- **Then** Playwright가 측정한 Web UI 총 소요 시간 ≤ 3분, `scripts/cli-install-speedrun.sh` 측정한 CLI 총 소요 시간 ≤ 2분. CI 파이프라인(`.github/workflows/install-wizard-e2e.yml`)에서 양 측정 모두 통과해야 PR merge 가능

**AC-OB-017 — 외부 전송 차단 invariant (verifies REQ-OB-014)**
- **Given** onboarding 진행 중, Step 5에서 telemetry opt-in 비선택, Step 3에서 OAuth 버튼 미클릭
- **When** 전 과정 완료
- **Then** 테스트 하네스(`httptest.NewServer` + `dnsproxy-test`)가 기록한 외부 네트워크 I/O 건수 = 0. 허용 건수: Step 3 OAuth 클릭 시 1건, Step 5 telemetry opt-in 후 1건 (본 시나리오에서는 모두 비활성화 상태)

**AC-OB-018 — 잘못된 API key 거부 (invalid key path) (verifies REQ-OB-015)**
- **Given** Step 3에서 Anthropic 선택 + API key `"sk-INVALID-123"` 입력 (Anthropic 접두어 `sk-ant-` 아님)
- **When** Next 실행
- **Then** 폼 검증 에러 "Invalid API key format for Anthropic (expected prefix `sk-ant-`)" 표시, OS keyring에 미기록(keyring 항목 조회 시 `ErrNotFound`), `./.goose/config.yaml`에도 미기록

**AC-OB-019 — 민감정보 필드 부재 (verifies REQ-OB-016)**
- **Given** onboarding UI 스냅샷 (CLI TUI 전체 + Web UI DOM 전체)
- **When** static audit 스크립트 `test/audit_no_sensitive_fields.go`가 각 단계 스키마 순회
- **Then** 필드 whitelist(name, honorific_level, pronouns, soul.md, locale, provider, api_key, messenger, consent flags)에 없는 필드는 0건. 금지 필드(SSN, biometric, medical, government_id, phone_number, physical_address, email)의 스키마 key 존재 시 테스트 실패

**AC-OB-020 — 접근성 preferences 존중 (verifies REQ-OB-019)**
- **Given-1 (Web UI)** 브라우저 `prefers-reduced-motion: reduce` 설정
- **Given-2 (CLI)** `NO_COLOR=1 mink init` 실행
- **When** 온보딩 각 단계 렌더
- **Then-1** Web UI: Framer Motion 전환이 `initial={false}` + `transition={{ duration: 0 }}` 로 축소됨 (DOM snapshot 비교). 모든 텍스트/배경 조합이 WCAG AA contrast (>= 4.5:1) 통과 (axe-core 자동 검사)
- **Then-2** CLI: ANSI escape 시퀀스가 출력되지 않음 (stdout capture 후 regex `\x1b\[` match 없음)

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃 (v0.2)

```
cmd/goose/cmd/
├── init.go                         # `mink init` CLI entry
└── serve.go                        # `mink serve --install-wizard` mode

internal/onboarding/                # Backend 공통 상태 머신 (CLI + Web UI)
├── flow.go                         # OnboardingFlow 상태 머신
├── steps.go                        # 5-Step 정의 + validators
├── progress.go                     # draft 저장/로드 (atomic write)
├── completion.go                   # 완료 처리 + 후속 SPEC 초기화 호출
├── keyring.go                      # OS keyring 추상 (zalando/go-keyring)
├── validators.go                   # API key prefix 검증 등
└── *_test.go

internal/cli/install/               # CLI TUI 레이어
├── tui.go                          # charmbracelet/huh 기반 form orchestration
└── tui_test.go

internal/server/install/            # Web UI HTTP 레이어
├── handler.go                      # /install/* HTTP handlers
├── embed.go                        # web/install dist embed
└── handler_test.go

web/install/                        # Web UI 프런트
├── src/
│   ├── OnboardingApp.tsx
│   ├── steps/
│   │   ├── Step1WelcomeLocale.tsx
│   │   ├── Step2Persona.tsx
│   │   ├── Step3Provider.tsx
│   │   ├── Step4Messenger.tsx
│   │   └── Step5Privacy.tsx
│   ├── ProgressBar.tsx
│   ├── store.ts                    # zustand
│   └── types.ts
├── index.html
└── vite.config.ts
```

### 6.2 핵심 타입 (Go)

```go
// internal/onboarding/flow.go

type OnboardingFlow struct {
    SessionID   string
    CurrentStep int              // 1..5
    Data        OnboardingData
    StartedAt   time.Time
    CompletedAt *time.Time
}

type OnboardingData struct {
    Locale    LocaleChoice       // Step 1
    Persona   PersonaProfile     // Step 2
    Provider  ProviderChoice     // Step 3
    Messenger MessengerChannel   // Step 4
    Consent   ConsentFlags       // Step 5
}

type PersonaProfile struct {
    Name            string   // required, 1..500 chars, no HTML/shell injection
    HonorificLevel  string   // "formal" | "casual" | "intimate"
    Pronouns        string   // optional
    SoulMarkdown    string   // soul.md body
}

type ProviderChoice struct {
    Provider       string  // "anthropic" | "openai" | "google" | "ollama" | "deepseek" | "custom" | "unset"
    AuthMethod     string  // "oauth" | "api_key" | "env"
    APIKeyStored   bool    // keyring에 저장되었는지
    CustomEndpoint string
    PreferredModel string
}

type MessengerChannel struct {
    Type        string  // "local_terminal" | "slack" | "telegram" | "discord" | "custom"
    BotTokenKey string  // keyring 엔트리 키 (저장되었을 때만)
}

type ConsentFlags struct {
    ConversationStorageLocal bool   // default true (필수)
    LoRATrainingAllowed      bool   // default false
    TelemetryEnabled         bool   // default false
    CrashReportingEnabled    bool   // default false
    GDPRExplicitConsent      *bool  // EU 사용자만 non-nil
}

// 공개 함수
func StartFlow(ctx context.Context, locale *locale.LocaleContext) (*OnboardingFlow, error)
func (f *OnboardingFlow) SubmitStep(step int, data any) error
func (f *OnboardingFlow) SkipStep(step int) error
func (f *OnboardingFlow) Back() error
func (f *OnboardingFlow) Complete() (*UserProfile, error)
```

### 6.3 CLI TUI 흐름 (`charmbracelet/huh`)

```
mink init
  → huh.NewForm() with 5 huh.Group (1 group per step)
  → Each group: title + fields + Next/Back/Skip key bindings
  → On Ctrl+C: flow.Save() to draft, exit code 130
  → On final submit: flow.Complete() + print success message
```

API key 입력 시:
- `huh.NewInput().Password(true)` — stdin echo off
- 서브미트 직후 `validators.ValidateProviderKey(provider, key)` 호출
- 검증 통과 시에만 `keyring.Set("goose.provider."+provider+".api_key", key)`

### 6.4 Web UI 흐름 (React + Vite)

```
mink serve --install-wizard
  → Go HTTP server embeds web/install dist
  → /install GET returns SPA
  → React app fetches /install/status → 현재 step 확인
  → Step별 컴포넌트 렌더, Next 시 POST /install/step/:n
  → Step 5 Submit 시 POST /install/complete → 서버는 응답 후 graceful shutdown
  → 브라우저는 "Setup complete. Return to terminal." 표시
```

### 6.5 국가별 Consent 문구 차이

| Country | GDPR | 문구 차이 |
|---------|------|---------|
| EU (DE, FR, IT, ...) | ✅ | "I explicitly consent" 체크박스 필수 |
| UK | ✅ (UK GDPR) | EU와 동일 |
| KR | PIPA | "개인정보 수집·이용 동의" 문구 + 민감정보 미수집 명시 |
| US (CA) | CCPA | "Do not sell my personal information" 옵션 |
| BR | LGPD | EU 유사 |
| CN | PIPL | "境内数据处理" 명시, 데이터 해외 전송 동의 |
| JP | APPI | "個人情報の利用目的" 명시 |
| 기타 | — | 일반 문구 |

LOCALE-001 `legal_flags` 기반 Step 5 UI 조건부 분기.

### 6.6 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| CLI TUI | `charmbracelet/huh` | 폼 중심 대화형, Bubble Tea 기반 |
| OS Keyring (Go) | `zalando/go-keyring` | macOS/Windows/Linux 통일 API, 외부 deps 없음 |
| Web UI React | `react` 19.x | 경량 SPA |
| Web UI State | `zustand` 5.x | 간결한 store |
| Web UI Animation | `framer-motion` 11.x (conditional) | reduced-motion 존중 |
| Web UI 컴포넌트 | `shadcn/ui` | 일관 디자인 |
| 폼 검증 (Web UI) | `zod` 3.x + `react-hook-form` 7.x | 선언적 검증 |
| i18n | `go-i18n` (backend) + `react-i18next` (Web UI) | I18N-001 공유 |
| Web UI 빌드 | `vite` 5.x | 빠른 dev server, embed-friendly |

**제거된 v0.1 의존성**: `tauri-plugin-keyring`, Tauri 전체 스택, Mobile 페어링 QR 생성기.

### 6.7 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | §5 Test Scenarios 20개(AC-OB-001~020 중 AC-OB-013 deprecated) + Playwright e2e + CLI speedrun 스크립트. Go unit tests for flow state machine, validators, keyring abstraction |
| **R**eadable | 각 Step 컴포넌트 분리 (React/TUI 공통), Go validator 선언적 테이블, i18n 모든 사용자 메시지 |
| **U**nified | 공통 backend (`internal/onboarding`)가 CLI와 Web UI 상태 머신 공유. 단일 소스 데이터 모델 |
| **S**ecured | API key keyring 전용(REQ-OB-007), 입력 사니타이징(REQ-OB-017), GDPR 명시적 동의 강제(REQ-OB-008), 외부 전송 차단(REQ-OB-014, AC-OB-017), 민감정보 필드 whitelist(REQ-OB-016, AC-OB-019) |
| **T**rackable | 각 step 소요 시간 익명 로그(opt-in 후에만), 완료율 추적, `./.goose/onboarding-draft.yaml` atomic write로 상태 복원 |

### 6.8 TDD 진입 순서 (v0.2)

1. **RED #1** — `TestOnboardingFlow_FirstLaunch_StartsStep1` → AC-OB-001
2. **RED #2** — `TestStep2Persona_EmptyName_Rejected` → AC-OB-005
3. **RED #3** — `TestStep4_Skip_AppliesLocalTerminalDefault` → AC-OB-006
4. **RED #4** — `TestStep3_APIKey_StoredInKeyring` (valid path) → AC-OB-007
5. **RED #5** — `TestStep3_InvalidAPIKey_Rejected` → AC-OB-018 (NEW, REQ-OB-015)
6. **RED #6** — `TestStep5_GDPR_RequiresExplicitConsent_FR` → AC-OB-008, AC-OB-014
7. **RED #7** — `TestComplete_ActivatesRegionSkills` → AC-OB-009
8. **RED #8** — `TestDraftResume_AfterCtrlC` → AC-OB-011
9. **RED #9** — `TestStep2_NameInjection_Rejected` → AC-OB-015
10. **RED #10** — `TestLocaleConflict_ForcesChoice` → AC-OB-004
11. **RED #11** — `TestNoExternalNetwork_WhenAllOptOut` → AC-OB-017 (NEW, REQ-OB-014)
12. **RED #12** — `TestFieldWhitelist_NoSensitiveFields` → AC-OB-019 (NEW, REQ-OB-016)
13. **RED #13** — `TestPrefersReducedMotion_DisablesAnimation` → AC-OB-020 (NEW, REQ-OB-019)
14. **GREEN** — 최소 구현
15. **REFACTOR** — step validator를 선언적 테이블로
16. **E2E (Playwright + CLI speedrun)** — 3분/2분 완주 검증 → AC-OB-016

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-LOCALE-001** | Detect() 결과 + override API + CulturalContext |
| 선행 SPEC | **SPEC-GOOSE-I18N-001** | UI 번역 + RTL + 국가별 consent 문구 |
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | 최종 UserProfile 저장소 (`./.goose/config.yaml`) |
| 동시 | SPEC-GOOSE-REGION-SKILLS-001 | 완료 시 자동 활성화 호출 |
| 동시 | SPEC-GOOSE-IDENTITY-001 (최소) | Initial node seed (이름 + persona) |
| 후속 SPEC | SPEC-GOOSE-CREDPOOL-001 | ProviderChoice + API key 이관 |
| 후속 SPEC | SPEC-MESSENGER-* | 첫 messenger 채널 연결 이관 |
| 외부 | `charmbracelet/huh` | CLI TUI 폼 |
| 외부 | `zalando/go-keyring` | OS keyring 추상 |
| 외부 | `framer-motion` 11.x | Web UI 애니메이션 (conditional on prefers-reduced-motion) |
| 외부 | `zod` 3.x + `react-hook-form` 7.x | Web UI 폼 검증 |
| 외부 | `vite` 5.x | Web UI 빌드 |

**v0.2에서 제거된 의존성**: SPEC-GOOSE-DESKTOP-001 (Tauri 호스트 환경), SPEC-GOOSE-BRIDGE-001 (Mobile 페어링; BRIDGE는 여전히 존재하지만 본 SPEC은 더 이상 호출하지 않음), `tauri-plugin-keyring`.

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 5단계가 여전히 길게 느껴져 중도 이탈 | 중 | 고 | Skip 적극 강조, 3분 내 완료 보장, 각 단계 평균 30초 목표, CLI `--yes`로 완전 비대화 가능 |
| R2 | GDPR 명시적 동의 미준수 시 법적 리스크 | 중 | 고 | 외부 법률 검토 + LOCALE-001 `legal_flags` 조건부 분기 + audit 로그 |
| R3 | LLM provider 스킵 후 영영 미설정 | 중 | 중 | 메인 앱 첫 실행 시 persistent notice + `mink config provider` 단축 명령 |
| R4 | API key 검증이 provider별로 다름 | 중 | 중 | provider별 prefix regex 테이블 + "Test connection" 선택 가능 (opt-in, 1 request 소비) |
| R5 | 중도 이탈 시 draft 파일 손상 | 낮 | 중 | atomic write(`tmp + rename`) + YAML schema 검증 |
| R6 | Web UI 애니메이션이 저사양 PC에서 끊김 | 중 | 낮 | `prefers-reduced-motion` 존중(AC-OB-020) + CLI 대안 경로 항상 존재 |
| R7 | 이름 입력의 이모지/특수문자 | 중 | 낮 | Unicode Name 규격 허용, HTML/shell injection만 차단 |
| R8 | RTL 언어(ar)에서 진행 방향 혼란 | 중 | 중 | Back/Next 시각적 방향을 RTL 자동 대응 (logical properties, Web UI), CLI는 방향 중립 |
| R9 | Web UI 서버 포트 충돌 (5173 사용 중) | 중 | 낮 | 포트 auto-increment, 환경변수 `MINK_INSTALL_PORT` 지원 |
| R10 | 온보딩 반복 버그 (완료 후에도 다시 실행) | 낮 | 고 | `./.goose/onboarding-completed` 타임스탬프 + 파일 존재 검증, Force-reset은 `mink init --force`에서만 |
| R11 | CLI TUI가 특정 터미널(cmd.exe, dumb term)에서 깨짐 | 중 | 중 | `$TERM=dumb` / `NO_COLOR=1` 감지 → 단순 prompt 모드 fallback, AC-OB-020으로 검증 |
| R12 | OS keyring 접근 불가 환경 (Linux headless, WSL without libsecret) | 중 | 중 | `keyring.Set` 실패 시 폴백 `~/.goose/.keyring-fallback.yaml`(0600) + 보안 경고 표시 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/branding.md` §5.1 First Day with MINK (Hatching)
- `.moai/project/branding.md` §3 다국어 페르소나
- `.moai/project/adaptation.md` §2.1 명시적 Persona
- `.moai/specs/SPEC-GOOSE-ARCH-REDESIGN-v0.2/spec.md` — Amendment 근거
- `.moai/specs/SPEC-GOOSE-LOCALE-001/spec.md` — Step 1 데이터 소스
- `.moai/specs/SPEC-GOOSE-I18N-001/spec.md` — UI 언어
- `.moai/specs/SPEC-GOOSE-REGION-SKILLS-001/spec.md` — 완료 시 활성화
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` — 저장소

### 9.2 외부 참조

- GDPR Article 6, 7 (동의 요건)
- CCPA §1798.120 (do-not-sell)
- PIPA 제15조 (수집 동의)
- WCAG 2.1 Level AA (접근성)
- Material Design: Onboarding patterns (참고, Desktop Tauri 스코프 제거 후 참조만 유지)
- `charmbracelet/huh` documentation
- `zalando/go-keyring` documentation

### 9.3 부속 문서

- `./research.md` — 온보딩 UX 경쟁사 분석, 목표 시간 검증, GDPR/PIPA/PIPL 문구 매핑. (주: research.md는 v0.1 시점 작성되어 Desktop 8-step 전제를 많이 포함한다. v0.2 Amendment 후 일부 내용은 참조만 하되 최종 스코프는 본 spec.md §1~§3을 따른다.)
- `../SPEC-GOOSE-LOCALE-001/spec.md`
- `../SPEC-GOOSE-I18N-001/spec.md`
- `../SPEC-GOOSE-REGION-SKILLS-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **LLM OAuth 플로우 상세를 구현하지 않는다**(CREDPOOL-001 전담).
- 본 SPEC은 **Identity Graph 완전 구축을 수행하지 않는다**. seed 노드만(IDENTITY-001이 확장).
- 본 SPEC은 **LoRA 훈련을 실행하지 않는다**(LORA-001). 동의 flag만 저장.
- 본 SPEC은 **Messenger 연결 프로토콜 상세를 구현하지 않는다**(MESSENGER-* SPECs). 첫 채널 선택과 bot token keyring 저장까지만.
- 본 SPEC은 **법률 문구 전문을 작성하지 않는다**. `docs/privacy-policy.md` 링크만. 실제 문구는 외부 법률 검토 후 주입.
- 본 SPEC은 **재온보딩 플로우를 구현하지 않는다**. v1.0+ Preferences 메뉴에서 별도.
- 본 SPEC은 **A/B 테스트를 수행하지 않는다**.
- 본 SPEC은 **민감정보(SSN, biometric, medical, government ID, phone number, physical address, email)를 수집하지 않는다**(REQ-OB-016).
- 본 SPEC은 **사용자 입력을 외부 서버로 전송하지 않는다**(REQ-OB-014 + Step 3 OAuth / Step 5 telemetry opt-in 예외).
- 본 SPEC은 **온보딩 완료 후 데이터를 자동 수정하지 않는다**. 수정은 Preferences 별도 화면.
- 본 SPEC은 **Desktop Tauri 앱 패키징 및 Mobile 디바이스 페어링을 구현하지 않는다**(v0.2 Amendment로 제거; 각각 v1.0+ 재평가 및 BRIDGE-001 참조).
- 본 SPEC은 **이메일 가입/로그인 플로우를 제공하지 않는다**(v0.2 Amendment).
- v0.2 Amendment: **CLI 온보딩은 본 SPEC의 IN SCOPE이다**. (v0.1에서는 "CLI 환경에서 온보딩을 강제하지 않는다" 조항이 있었으나, v0.2 Amendment에서 `mink init` CLI 마법사가 핵심 경로로 지정되었으므로 해당 조항은 철회한다.)

---

**End of SPEC-GOOSE-ONBOARDING-001 v0.2.0**
