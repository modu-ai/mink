---
id: SPEC-GOOSE-WEBUI-001
version: 0.2.1
status: planned
created_at: 2026-04-24
updated_at: 2026-05-04
author: manager-spec
priority: P0
issue_number: null
phase: 6
milestone: M6
size: 대(L)
lifecycle: spec-anchored
labels: [phase-6, milestone-m6, area/webui, area/frontend, type/feature, priority/p0-critical]
---

# SPEC-GOOSE-WEBUI-001 — Localhost Web UI (비개발자 채널)

> v0.2 신규. 비개발자 사용자가 CLI 학습 없이 AI.GOOSE를 설치·운영·대화할 수 있는 localhost 기반 Web UI.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2 §6 메신저 채널.

## HISTORY

| 버전  | 날짜       | 변경 사유 | 담당          |
| ----- | ---------- | --------- | ------------- |
| 0.1.0 | 2026-04-24 | SPEC 초안 작성 (5 REQ + 5 AC 스켈레톤). | architecture-redesign-v0.2 |
| 0.1.0 | 2026-04-27 | HISTORY 섹션 추가 (감사). | GOOS행님 |
| 0.2.0 | 2026-05-04 | 본격 plan 보강 — 18+ REQ + 12+ AC + Tech Approach + Dependencies + Risks. v0.1.0 5 REQ는 의미 보존하며 새 분류 체계로 재흡수: REQ-WEBUI-001(launch)→REQ-WEBUI-104+REQ-WEBUI-201, REQ-WEBUI-002(loopback)→REQ-WEBUI-005, REQ-WEBUI-003(SSE)→REQ-WEBUI-007+REQ-WEBUI-203, REQ-WEBUI-004(install wizard)→REQ-WEBUI-101+REQ-WEBUI-202, REQ-WEBUI-005(approval flow)→REQ-WEBUI-208. v0.1.0 AC-WEBUI-01..05도 동일 의미로 새 식별자에 재배치. labels 채움, lifecycle을 spec-first→spec-anchored로 격상. | manager-spec |
| 0.2.1 | 2026-05-04 | research.md §9 의 3개 Open Question 결정 amendment + §1 의 "별도 포트" 가정 정정. (1) `/bridge/login` schema = 명시 POST 채택 (RESTful, testable). (2) WEBUI listener port = BRIDGE-001 listener 와 **shared port + path 분리** 채택 (`/webui/*` 정적/관리 API + `/bridge/*` wire). 단일 origin 으로 CORS 회피. §1 의 "별도 포트" 표현 정정. (3) channel-aware Confirmer routing = HOOK-001 amendment 의 책임 (별도 SPEC). 본 SPEC 은 hook event 수신 + modal 표시만 담당. REQ/AC 본문 변경 없음 — Tech Approach §6 와 §11 Open Questions Resolution 신규 추가만. | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE의 v0.1 Alpha 채널 3종(`goose CLI/TUI` + Telegram + 본 Web UI) 중 비개발자 진입점이 되는 **localhost 전용 브라우저 GUI**를 정의한다. 사용자는 `goose web` 서브커맨드로 daemon에 동봉된 정적 번들을 띄우고, 기본 브라우저로 `http://127.0.0.1:8787`에 접속해서 다음 흐름을 수행한다:

1. **Install wizard** — 첫 접속 시 provider key 입력 → `~/.goose/secrets/`(키링 참조)·`./.goose/config/`(설정)에 저장 → 첫 대화 smoke test.
2. **Chat 페이지** — SSE 스트림으로 LLM 응답을 토큰 단위로 받는다. AI.GOOSE persona·growth 단계가 voice에 반영된다.
3. **Settings 페이지** — `security.yaml` / `providers.yaml` / `channels.yaml`을 편집하고 daemon 핫리로드를 트리거.
4. **Audit 뷰어** — `~/.goose/logs/audit.log` (AUDIT-001)와 `./.goose/logs/audit.local.log`를 페이지네이션 + 필터로 표시.
5. **Ritual 승인 flow** — Plan 단계의 AskUserQuestion 유발 task가 발생하면 Web UI에서 승인/거절/수정.

본 SPEC이 통과한 시점에서 `goosed` 데몬은:

- `internal/webui/` 패키지를 통해 BRIDGE-001 의 동일 HTTP listener 위에 `/webui/*` 경로(정적 번들 + 설치 wizard / settings / audit 관리 API)를 mount 한다. wire protocol(WebSocket/SSE/POST)은 같은 listener 의 `/bridge/*` 경로(BRIDGE-001 소관). **단일 loopback origin** 으로 CORS 를 회피한다 (v0.2.1 결정, §11 참조),
- `cmd/goose/web.go` 서브커맨드가 `goosed`에 wake 신호를 보내고 OS 기본 브라우저를 연다.
- Frontend 정적 번들(Vite + React + Tailwind v4 + shadcn/ui)이 `internal/webui/static/dist/`에 embed되고 `goosed`가 `embed.FS`로 서빙한다.
- BRIDGE-001 (WebSocket/SSE wire) + PERMISSION-001 (token auth) + AUDIT-001 (audit.log) + HOOK-001 (AskUserQuestion 발화)의 컨트랙트만 소비한다 — 본 SPEC은 wire protocol을 새로 정의하지 않는다.

본 SPEC은 **외부 네트워크 노출, multi-user 인증, 모바일 원격 접속을 포함하지 않는다** (각각 후속 BRIDGE-002 / SECURITY-AUTH-002 / GATEWAY-MOBILE-001 후보).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **ARCH-REDESIGN-v0.2 §6**의 v0.1 Alpha 채널 결정: `CLI / TUI`, `Telegram`, `Web UI (localhost)` 3종이 동시 출시. Email은 v0.1에서 제거됐고, Web UI는 "비개발자 설치·관리 GUI" 위치를 차지한다 (`localhost:8787 SSE 스트림`).
- **비개발자 진입 장벽 해소**: `goose CLI`만 노출하면 친구·가족·동료가 AI.GOOSE를 깔아볼 길이 사실상 없다. 브라우저는 모든 OS의 공통 분모.
- **PERMISSION-001 / HOOK-001 채널 표면화**: 두 SPEC이 정의한 first-call confirm + plan-time approval은 인터페이스(`Confirmer`, AskUserQuestion 발화)만 정의되어 있다. CLI/TUI는 이미 자기 어댑터를 가진 상태고, Web UI 어댑터가 비어 있어 "Web UI 사용자는 항상 deny" 효과가 발생한다.
- **AUDIT-001 가시화**: append-only audit.log가 텍스트 파일로 쌓이지만 비개발자가 grep으로 보지는 않는다. 페이지네이션 뷰어가 채널 가시성을 닫는다.

### 2.2 v0.1.0 → v0.2.0 차이

v0.1.0 (5 REQ + 5 AC) 은 channel 진입점·SSE·loopback bind·install wizard·approval flow의 **5 핵심 axis**만 명시했다. 본 v0.2.0은 동일 5 axis를 의미 보존한 상태에서 다음 차원을 추가한다:

| 차원 | v0.1.0 | v0.2.0 |
|---|---|---|
| Launch | "goose web" 1줄 | port conflict / browser open 실패 / re-launch idempotency 명시 |
| Loopback | "외부 차단" 1줄 | bind 검증 + Origin/Host 검증 + CSRF + DNS rebinding 방어 분리 |
| Streaming | "SSE" 1줄 | SSE event 분류 (chunk/status/notification/permission_request) + Last-Event-ID resume |
| Install wizard | "wizard 표시" 1줄 | provider key entry → keyring write → daemon reload → smoke test 4-step state machine + secret leak 방지 |
| Approval | "승인/거절/수정 UI" 1줄 | HOOK-001 AskUserQuestion 와이어 + 60s timeout + decline reason capture |
| 신규 | — | settings 핫리로드, audit 뷰어 페이지네이션, brand consistency, i18n base, bundle size budget |

### 2.3 상속 자산 (패턴만 계승)

- **BRIDGE-001 v0.2.0**: WebSocket/SSE wire 계약 + 16 close code + flush-gate backpressure + 24h cookie + CSRF double-submit. 본 SPEC은 BRIDGE-001 클라이언트일 뿐, 자체적으로 wire를 정의하지 않는다.
- **PERMISSION-001**: `Confirmer` 인터페이스 + 3-way grant (AlwaysAllow/OnceOnly/Deny) + grant store 영속화. 본 SPEC은 Web UI측 Confirmer 어댑터를 제공한다.
- **HOOK-001**: AskUserQuestion event hook. 본 SPEC은 Web UI 채널에서 AskUserQuestion이 발화될 때의 표시·응답 경로를 정의한다.
- **MoAI ADK shadcn/ui rule**: 본 레포의 frontend 컨벤션은 Tailwind v4 + shadcn/ui v4. 본 SPEC은 동일 컨벤션을 따른다.
- **Open WebUI / Ollama UI** (외부): 패턴 영감만 차용 — code 직접 포팅은 없음. 둘 다 self-hosted local LLM UI이며, install wizard 흐름·SSE handling·dark mode 토글에서 학습.

### 2.4 v0.1 Alpha scope guardrail

- **MUST**: install wizard, chat (SSE), settings, audit viewer, ritual approval flow, brand-aligned UI.
- **OUT**: 외부 네트워크 bind, multi-user auth, 모바일 원격 bridge, realtime collab, plugin marketplace UI, browser extension, PWA offline, Service Worker push notification.
- **Tech stack 결정**: Vite + React 18 + TypeScript 5 + Tailwind v4 + shadcn/ui v4. **Next.js 15 RSC는 채택하지 않는다** (이유: §6.5 + research.md §5).
- **Bundle size budget**: gzip 후 500KB 이하 (cold start 1초 이내 first paint 목표).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/webui/` Go 패키지 — Web UI 전용 HTTP listener, install wizard handler, settings handler, audit viewer handler, embed.FS 정적 자원 서빙.
2. `cmd/goose/web.go` 서브커맨드 — `goose web [--port N] [--no-browser] [--bind 127.0.0.1]`. daemon이 안 떠 있으면 `goosed` 자동 spawn (별도 프로세스).
3. Frontend 정적 번들 — Vite + React + TS + Tailwind v4 + shadcn/ui로 빌드, `internal/webui/static/dist/`에 emit, `embed.FS`로 번들.
4. **5 페이지**: `/` (chat), `/install` (wizard), `/settings`, `/audit`, `/approve/{request_id}` (modal-friendly).
5. Install wizard state machine: `intro → provider-select → key-entry → keyring-write → daemon-reload → smoke-test → done` (7-state).
6. Settings 핫리로드: `PUT /webui/settings/{file}` → daemon `SIGHUP` 또는 file watcher → 응답 200 + 적용 결과.
7. Audit viewer: 시간 역순 페이지네이션 (50건/페이지), filter (event_type / subject_id / capability), 검색.
8. Approval flow: AskUserQuestion 발화 → SSE event `permission_request` → Web UI modal → 응답을 `POST /webui/approve` → BRIDGE-001 inbound → HOOK-001 처리.
9. Brand consistency: 표기 `AI.GOOSE` (사용자 가시), `goose` (코드 식별자 = 백틱), 색상은 `.moai/project/brand/design-tokens.json`의 primary `#FFB800`, neutral scale, fontFamily Inter+Pretendard.
10. **Loopback-only 보장 (REQ-WEBUI-005)** — bind 검증 + Origin/Host 검증 + DNS rebinding 방어 (`Host` 헤더가 `127.0.0.1` 또는 `[::1]`만 허용; `localhost`는 허용하되 DNS resolution이 loopback인지 검증).
11. **Dark mode** — `prefers-color-scheme` + manual toggle.
12. **Korean primary + English fallback i18n** — `i18next` 기반, 모든 UI 문자열 외부화.

### 3.2 OUT OF SCOPE (명시적 제외)

- WebSocket/SSE wire protocol 자체 — BRIDGE-001 담당.
- Token auth grant store — PERMISSION-001 담당.
- Audit log 본체 (rotation / append-only) — AUDIT-001 담당.
- AskUserQuestion event hook 자체 — HOOK-001 담당.
- 외부 네트워크 bind / 원격 mobile bridge — BRIDGE-002 후보.
- Multi-user 인증 / federated identity (OIDC/SAML) — 후속 SPEC.
- Realtime collab (다중 탭 동시 편집) — 후속 SPEC.
- Plugin marketplace UI — PLUGIN-001 후속.
- Browser extension / PWA offline / Service Worker — 후속 SPEC.
- E2EE — loopback이므로 불필요.
- Mobile native shell (React Native / Capacitor) — v0.1 OUT.
- A11y compliance audit (WCAG AA 인증) — 후속 SPEC. 본 SPEC은 keyboard navigation + 충분한 색상 대비만 보장.
- Internationalization 일본어/중국어 — Korean + English만, 다른 언어는 후속.
- Browser DevTools extension integration — 후속.
- 사용자 정의 테마 (custom CSS) — 후속.
- 멀티 워크스페이스 (한 브라우저에서 여러 `./.goose/` 동시 관리) — 후속.

---

## 4. 의존성 (Dependencies)

| 타입 | 대상 | 상태 | 설명 |
|-----|------|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | completed | daemon lifecycle, root context, drain consumer 등록 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | completed | `~/.goose/` + `./.goose/` 경로 의미, security.yaml 스키마 |
| 선행 SPEC | SPEC-GOOSE-DAEMON-WIRE-001 | completed | wire-up 헬퍼 패턴 (본 SPEC은 동일 패턴으로 webui consumer 등록) |
| 선행 SPEC | SPEC-GOOSE-PERMISSION-001 | completed | `Confirmer` 인터페이스, grant store, first-call confirm flow |
| 선행 SPEC | SPEC-GOOSE-AUDIT-001 | completed | append-only audit.log 본체, 본 SPEC은 read-only consumer |
| 선행 SPEC | SPEC-GOOSE-HOOK-001 | completed | AskUserQuestion event 발화 + 응답 dispatch |
| 동일 단계 SPEC | SPEC-GOOSE-BRIDGE-001 | planned | WebSocket/SSE wire 계약 — 본 SPEC은 클라이언트 |
| 동일 단계 SPEC | SPEC-GOOSE-CREDENTIAL-PROXY-001 | planned | install wizard에서 키 입력 시 keyring 저장 경로 |
| 후속 SPEC | SPEC-GOOSE-CHANNEL-TG-001 | planned | Telegram bot이 본 Web UI와 동일 PERMISSION-001 / HOOK-001 어댑터 패턴 사용 |
| 외부 | Go 1.23+ | available | embed.FS, net/http, context |
| 외부 | Vite 5.x + React 18 + TS 5 | available | frontend 빌드 |
| 외부 | Tailwind v4 + shadcn/ui v4 | available | 프로젝트 컨벤션 |
| 외부 | i18next 23.x + react-i18next | available | i18n |
| 외부 | EventSource Polyfill (옵션) | available | SSE 안정성 (모던 브라우저는 native) |

**라이브러리 결정 의도**:
- **Next.js 15 / RSC 미채택 이유**: 본 UI는 daemon이 서빙하는 정적 SPA. RSC server bundle을 Go 데몬에 끼울 이유가 없고, build complexity와 cold-start 비용 (gzip 후 1.2MB+ baseline) 이 cold start <1s 목표를 위협한다 (research.md §5에서 정량 비교).
- **HTMX 미채택 이유**: SSE chunk가 마크다운 + 코드블록을 점진 렌더링해야 하므로 클라이언트 상태가 불가피. HTMX의 swap 모델은 chat에 부적합.
- **Astro 미채택 이유**: 동일 — 동적 chat이 정적 + island 모델보다 SPA가 단순.

---

## 5. EARS 요구사항 (Requirements)

> §5의 REQ-WEBUI-NNN은 다음 분류 체계로 100/200/300/400/500 스킴: 100 = Ubiquitous, 200 = Event-Driven, 300 = State-Driven, 400 = Optional, 500 = Unwanted. 각 카테고리 내부에서 단조 증가, 결번/중복 없음. v0.1.0의 REQ-WEBUI-001..005는 카테고리 재배치되었으며 의미는 보존된다.

### 5.1 Ubiquitous (시스템 상시 불변)

**REQ-WEBUI-101 [Ubiquitous]** — The Web UI Go package `internal/webui` **shall** expose a single HTTP listener that mounts five route groups: `/` (SPA shell), `/webui/install/*` (wizard API), `/webui/settings/*` (settings API), `/webui/audit/*` (audit viewer API), `/webui/approve/*` (approval API), and **shall not** expose any route under `/bridge/*` (BRIDGE-001 owns that namespace).

**REQ-WEBUI-102 [Ubiquitous]** — Every HTML response served by `internal/webui` **shall** include a `Content-Security-Policy` header with `default-src 'self'; connect-src 'self' http://127.0.0.1:* http://[::1]:*; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'`. The CSP **shall not** include `unsafe-eval` and **shall not** allow remote script origins.

**REQ-WEBUI-103 [Ubiquitous]** — Every API response under `/webui/*` **shall** be `application/json` with explicit error schema `{error: {code: string, message: string, details?: object}}` on non-2xx; **shall not** return HTML error pages on API endpoints.

**REQ-WEBUI-104 [Ubiquitous]** — The Web UI bundle **shall** be embedded into the `goosed` binary via `embed.FS` and **shall not** require any runtime filesystem access to source assets; the `goose web` subcommand **shall** start successfully on a freshly-built binary placed in any writable directory without auxiliary files. (v0.1.0 REQ-WEBUI-001 부분 흡수: launch.)

**REQ-WEBUI-105 [Ubiquitous]** — All user-facing strings rendered by the SPA **shall** be sourced from `i18n/locales/{ko,en}.json` and **shall not** be hardcoded inline; default language is Korean (`ko`), fallback `en`. Brand notation **shall** follow `.moai/project/brand/style-guide.md`: `AI.GOOSE` for prose, `` `goose` `` (backticked) for identifiers.

**REQ-WEBUI-106 [Ubiquitous]** — The frontend bundle gzipped size **shall** stay under 500 KB at build time; `npm run build` **shall** fail (non-zero exit) if the cumulative gzipped JS bundle exceeds the threshold.

### 5.2 Event-Driven (이벤트 기반)

**REQ-WEBUI-201 [Event-Driven]** — **When** `goose web` is invoked, the CLI **shall** (a) check whether `goosed` is reachable on `cfg.Transport.HealthPort`, (b) if not, fork-spawn `goosed` and wait up to 5 s for `state=serving`, (c) resolve the configured `webui.bind_port` (default 8787), (d) open the user's default browser to `http://127.0.0.1:{port}/` unless `--no-browser` is set, and (e) print the URL to stdout regardless of `--no-browser`. (v0.1.0 REQ-WEBUI-001 launch behavior 흡수.)

**REQ-WEBUI-202 [Event-Driven]** — **When** the Web UI receives the first request to any path under `/webui/*` and the install wizard has never completed (signal: `~/.goose/state/install.json` missing or `completed=false`), the server **shall** redirect the SPA to `/install` and **shall** reject all non-install API endpoints with HTTP 412 `{error.code:"install_required"}`. (v0.1.0 REQ-WEBUI-004 흡수.)

**REQ-WEBUI-203 [Event-Driven]** — **When** an LLM streaming response is in progress for the active session, the Web UI **shall** subscribe to BRIDGE-001's SSE endpoint (`/bridge/stream`) and **shall** render `chunk` events as incremental markdown using a streaming markdown parser; first-token-to-paint latency **shall** be ≤ 100 ms p95 measured from BRIDGE-001 chunk emit to DOM mutation, on a 4-core x86_64 host with 8 GB RAM. (v0.1.0 REQ-WEBUI-003 + AC-WEBUI-03 흡수.)

**REQ-WEBUI-204 [Event-Driven]** — **When** the user submits the install wizard's provider-key form, the Web UI **shall** (a) validate the key shape client-side (length and prefix), (b) `POST /webui/install/credentials` with the key in the request body over the loopback connection, (c) the server-side handler **shall** invoke `CredentialProxy.Store(provider, key)` (CREDENTIAL-PROXY-001) which writes a keyring reference, (d) the server **shall not** log the raw key (zap field redacted, only `key_id` logged), (e) on success the wizard advances to `daemon-reload` state.

**REQ-WEBUI-205 [Event-Driven]** — **When** the user edits a settings file (`security.yaml` / `providers.yaml` / `channels.yaml` / `aliases.yaml`) via the Web UI and submits, the server **shall** (a) parse and validate the YAML with the same schema validator the daemon uses at boot, (b) write atomically (temp + rename) with mode 0600, (c) trigger daemon reload via the existing config reload hook (CONFIG-001 reload path), (d) respond 200 with the parsed snapshot, or 400 with validation errors per field. (v0.1.0 AC-WEBUI-05 immediate apply 흡수.)

**REQ-WEBUI-206 [Event-Driven]** — **When** the user opens `/audit`, the Web UI **shall** request `GET /webui/audit?cursor={ts}&limit=50&filter={...}` which **shall** return a paginated JSON list of audit events from `~/.goose/logs/audit.log` (and optionally `./.goose/logs/audit.local.log`), sorted by descending timestamp; events older than the cursor are excluded. The handler **shall not** return event records whose `subject_type=skill` and whose declared `requires.fs_read` includes paths the current OS user cannot read (defense-in-depth read filter).

**REQ-WEBUI-207 [Event-Driven]** — **When** the SSE connection drops (network blip, tab backgrounded), the Web UI client **shall** reconnect using the `Last-Event-ID` header carrying the last received `Sequence`, per BRIDGE-001 REQ-BR-009 / REQ-BR-018; reconnection backoff **shall** match the BRIDGE-001 §6.2 schedule (1s → 30s ± 20% jitter, max 10 attempts before requiring fresh cookie).

**REQ-WEBUI-208 [Event-Driven]** — **When** an `OutboundPermissionRequest` event arrives via the SSE stream (HOOK-001 AskUserQuestion fan-out), the Web UI **shall** display a modal containing the request `(subject_id, capability, scope, reason)`, **shall** offer four actions `{AlwaysAllow, OnceOnly, Deny, ModifyScope}`, and **shall** `POST /webui/approve/{request_id}` with the user's choice within 60 s; if no action within 60 s the client **shall** emit an `inboundPermissionResponse{decision: "deny", reason: "timeout"}` (matching BRIDGE-001 REQ-BR-008 default-deny-on-timeout). (v0.1.0 REQ-WEBUI-005 흡수.)

### 5.3 State-Driven (상태 기반)

**REQ-WEBUI-301 [State-Driven]** — **While** the install wizard is in any state other than `done`, the Web UI **shall not** allow navigation to `/`, `/settings`, `/audit`; explicit navigation attempts **shall** redirect to `/install` and resume from the recorded wizard state (persisted in `~/.goose/state/install.json`).

**REQ-WEBUI-302 [State-Driven]** — **While** the daemon's state is `bootstrap` or `draining` (CORE-001 state machine), the Web UI server **shall** respond `503` with `Retry-After: 1` to any `/webui/*` API request and **shall** display a "starting" or "shutting down" overlay in the SPA.

**REQ-WEBUI-303 [State-Driven]** — **While** the chat session is awaiting a permission response (an `OutboundPermissionRequest` is open, a corresponding `permission_response` has not yet arrived from BRIDGE-001), the Web UI **shall** disable the chat input field, **shall** show a non-dismissible (visually clear) modal, and **shall not** allow new chat messages to be POSTed.

**REQ-WEBUI-304 [State-Driven]** — **While** dark mode is active (user preference stored in `localStorage.theme=dark` or `prefers-color-scheme: dark` at first visit), the SPA **shall** use the `neutral.900`/`950` background tokens and primary `#FFB800` accent retains WCAG AA contrast against the dark background; the toggle **shall** persist across page reloads.

### 5.4 Optional (선택적)

**REQ-WEBUI-401 [Optional]** — **Where** the user supplies `--port N` to `goose web`, the server **shall** bind to `127.0.0.1:N` and `[::1]:N`; if the port is occupied the server **shall** fail with `webui.port_in_use` error (no auto-increment, explicit failure).

**REQ-WEBUI-402 [Optional]** — **Where** the user has multiple LLM provider keys configured, the chat input **shall** offer a model picker dropdown that updates `providers.yaml` `default` field via `PUT /webui/settings/providers.yaml`; the picker is hidden when only one provider is configured.

**REQ-WEBUI-403 [Optional]** — **Where** the audit viewer query string includes `subject_id=<id>`, the response **shall** highlight matching event rows and link to the corresponding skill/MCP/agent definition file (read-only path display, no file content rendering).

### 5.5 Unwanted Behavior (방지)

**REQ-WEBUI-501 [Unwanted]** — The Web UI server **shall not** bind to any address other than `127.0.0.1`, `::1`, or `localhost` (resolved to loopback); attempts to start with a non-loopback bind **shall** fail with `webui.non_loopback_bind` and **shall** log a security warning. (v0.1.0 REQ-WEBUI-002 흡수.)

**REQ-WEBUI-502 [Unwanted]** — The Web UI server **shall not** accept any HTTP request whose `Host` header resolves to a non-loopback IP (DNS rebinding defense); requests with `Host: evil.com` even when arriving on the loopback socket **shall** be rejected with HTTP 421 `misdirected_request` and logged.

**REQ-WEBUI-503 [Unwanted]** — The Web UI server **shall not** include raw provider-key values in any HTTP response body, log line, audit event, or SSE event; only `{key_id, last4}` projection is permitted. Test suite **shall** assert by greping responses for any value that matches a known fixture key.

**REQ-WEBUI-504 [Unwanted]** — The Web UI **shall not** expose a `/webui/exec` or `/webui/eval` endpoint, **shall not** allow users to upload arbitrary skills or MCP server configs through the UI in v0.2.0 (config edit limited to enumerated YAML files), and **shall not** allow file system browse beyond `~/.goose/` and `./.goose/` paths.

**REQ-WEBUI-505 [Unwanted]** — If an SSE reconnect storm is detected (≥ 10 reconnection attempts in 30 s for the same session cookie), the Web UI client **shall** stop retrying and **shall** display a banner instructing the user to reload manually; the server **shall** rate-limit reconnect attempts at 1/s per session via BRIDGE-001 close code 4429 enforcement.

**REQ-WEBUI-506 [Unwanted]** — The Web UI **shall not** silently overwrite settings YAML files when the daemon's in-memory snapshot disagrees with the on-disk file (concurrent edit detected via mtime+hash check); on disagreement the server **shall** respond 409 `conflict` with both versions and require user merge confirmation.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/webui/
├── server.go              # HTTP listener + route mux
├── server_test.go
├── handlers/
│   ├── install.go         # /webui/install/*
│   ├── settings.go        # /webui/settings/*
│   ├── audit.go           # /webui/audit/*
│   ├── approve.go         # /webui/approve/*
│   └── static.go          # embed.FS 서빙 + SPA fallback
├── installer/
│   ├── state.go           # ~/.goose/state/install.json 영속
│   ├── state_machine.go   # 7-state transitions
│   └── smoketest.go       # daemon에 첫 ping
├── confirmer/             # PERMISSION-001 Confirmer 어댑터
│   └── webui_confirmer.go # SSE permission_request fan-out
├── auditviewer/
│   ├── reader.go          # audit.log 역방향 페이지네이션
│   └── filter.go
├── settingsedit/
│   ├── validator.go       # YAML schema 검증 (CONFIG-001 재사용)
│   └── reload.go          # daemon hot-reload trigger
├── static/
│   ├── dist/              # Vite build output (embed)
│   └── embed.go           # //go:embed dist/*
└── errors.go

cmd/goose/
└── web.go                 # goose web 서브커맨드

frontend/                  # 별도 모듈, npm run build → internal/webui/static/dist/
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── pages/
│   │   ├── Chat.tsx
│   │   ├── Install.tsx        # 7-step wizard
│   │   ├── Settings.tsx
│   │   ├── Audit.tsx
│   │   └── ApproveModal.tsx
│   ├── components/
│   │   ├── ui/                # shadcn/ui
│   │   ├── chat/              # MessageStream, MarkdownStream, ChatInput
│   │   └── brand/             # Logo, Wordmark (AI.GOOSE)
│   ├── hooks/
│   │   ├── useSSE.ts          # EventSource + Last-Event-ID
│   │   ├── useApproval.ts
│   │   └── useDarkMode.ts
│   ├── lib/
│   │   ├── api.ts             # /webui/* fetch 래퍼
│   │   └── markdownStream.ts  # incremental markdown parser
│   ├── i18n/
│   │   ├── ko.json
│   │   └── en.json
│   └── styles/
│       └── tokens.css         # design-tokens.json → CSS variables
├── package.json
├── vite.config.ts
├── tailwind.config.ts          # design-tokens.json import
└── tsconfig.json
```

### 6.2 wire 통합 (cmd/goosed/wire.go)

본 SPEC은 `wireSlashCommandSubsystem` 패턴을 그대로 따라 새로운 `wireWebUISubsystem(rt, hookRegistry, permissionMgr, auditReader, logger) (*webui.Server, error)`를 추가한다. main.go의 13-step 생애주기에서 step 10.9로 삽입:

```
Step 10.9: WebUI subsystem wiring
- webui.Server 생성 (handlers + Confirmer 어댑터)
- PERMISSION-001 Manager에 webui Confirmer 등록 (channel = "webui")
- HOOK-001 hookRegistry에 webui AskUserQuestion handler 등록
- AUDIT-001 reader 인터페이스 binding
- rt.Drain에 webui.Server.Shutdown 등록 (timeout 5s)
- listenAndServe는 cfg.WebUI.BindPort (default 8787) 에서
```

별도 listener를 사용한다 (BRIDGE-001 / TRANSPORT-001과 포트 분리). BRIDGE-001 endpoint는 같은 daemon process이므로 cross-origin이 아닌 same-origin (다른 path만 사용) — 본 SPEC의 frontend는 `fetch('/bridge/...')` 로 호출.

### 6.3 Install wizard state machine

```
intro
  ├─[Continue]─→ provider-select
provider-select
  ├─[Anthropic]──→ key-entry(provider=anthropic)
  ├─[OpenAI]─────→ key-entry(provider=openai)
  ├─[Google]─────→ key-entry(provider=google)
  └─[Ollama]─────→ key-entry(provider=ollama, type=endpoint-url)
key-entry
  ├─[Submit Valid]──→ keyring-write
  └─[Cancel]────────→ provider-select
keyring-write (auto)
  ├─[Success]───→ daemon-reload
  └─[Failure]───→ key-entry (with error)
daemon-reload (auto, ~2s)
  └─→ smoke-test
smoke-test
  ├─[LLM ping success]──→ done
  └─[Failure]───────────→ key-entry (with provider error)
done
  └─→ redirect to /
```

상태는 `~/.goose/state/install.json` 에 atomic write (temp + rename, mode 0600). 각 트랜지션은 `installer.state_machine.go`의 단일 함수 `Transition(current State, event Event) (next State, error)` 로 표현.

### 6.4 SSE consumption 모델

Web UI frontend의 `useSSE` hook:

```typescript
// frontend/src/hooks/useSSE.ts (의사코드)
//
// EventSource('/bridge/stream') 을 열고, 4 SSE event type을 dispatch:
//   - chunk: 마크다운 incremental render
//   - status: top bar 상태 표시
//   - permission_request: ApproveModal 띄움
//   - error: toast + reconnect logic 시작
//
// reconnect 시 lastEventId 헤더로 sequence 전달 (BRIDGE-001 REQ-BR-009)
```

incremental markdown rendering은 `streaming-markdown` 라이브러리 또는 자체 파서 (chunk 단위로 token 누적, code fence/list/table 경계 감지) 사용.

### 6.5 Frontend tech stack 결정 근거

| 차원 | Vite + React (선택) | Next.js 15 RSC (기각) | Astro (기각) | HTMX (기각) |
|---|---|---|---|---|
| Daemon embed 호환 | ✅ static SPA, embed.FS 단일 dir | ⚠️ server bundle 별도, Go에 끼우려면 SSR runtime 필요 | ⚠️ partial hydration runtime 필요 | ⚠️ SSE chunk를 swap으로 처리하기 부적합 |
| Bundle size (gzip) | ~280 KB target | ~1.2 MB baseline | ~600 KB | ~50 KB (but rebuild 부담) |
| Cold start to first paint | <800 ms | 1.5–2s | ~1s | <300 ms (but chat 안 됨) |
| Streaming markdown chat | 표준 SPA 패턴 | 가능하나 RSC streaming 복잡 | island 모델로 어색 | 불가능 (clientside state 불가피) |
| 프로젝트 컨벤션 정합 | ✅ shadcn/ui v4 + Tailwind v4 | ✅ 정합되나 RSC가 표준 컨벤션 변형 | ⚠️ 학습 비용 | ❌ 컨벤션 불일치 |
| 결정 | **선택** | 기각 | 기각 | 기각 |

### 6.6 Brand integration

- 색상: design-tokens.json의 primary `#FFB800` (CTA), neutral light `#FAF8F4` / dark `#171513` (background), accent `#E56B7C`. Tailwind v4 `@theme` 블록에 토큰을 import.
- 타이포그래피: primary `Inter, Pretendard`, display `Fraunces, Pretendard`, mono `JetBrains Mono, D2Coding`. font-display: swap. Korean letter-spacing -0.01em.
- 모션: `motion.duration` 토큰 (`acknowledge: 180ms`, `settle: 320ms`, `growth: 800ms`) 사용. 메시지 도착 = acknowledge, 페이지 트랜지션 = settle.
- 표기: `<h1>AI.GOOSE</h1>` (산문 위치), `<code>goose CLI</code>` (백틱). brand-lint 검증은 spec.md / research.md 자체에 적용 — frontend 정적 자원에는 적용되지 않으나 사용자 가시 문자열은 동일 컨벤션을 따른다 (i18n ko.json/en.json).

### 6.7 5-Phase 구현 계획 (run phase에서 수행)

| Phase | 범위 | 주요 산출물 | 의존 |
|-------|-----|----------|------|
| Phase 1 — Server skeleton + embed | `internal/webui/server.go` + embed.FS + SPA shell + `goose web` 서브커맨드 | REQ-WEBUI-101/104/201/501/502 그린 | DAEMON-WIRE-001 |
| Phase 2 — Install wizard | state machine + 5 핸들러 + frontend Install.tsx | REQ-WEBUI-202/204/301 그린 | CREDENTIAL-PROXY-001 |
| Phase 3 — Chat (SSE consume) | useSSE + MessageStream + ChatInput | REQ-WEBUI-203/207/303/505 그린 | BRIDGE-001 |
| Phase 4 — Settings 핫리로드 | settings handler + validator + reload | REQ-WEBUI-205/506 그린 | CONFIG-001 reload path |
| Phase 5 — Audit + Approval | audit reader + ApproveModal + Confirmer 어댑터 | REQ-WEBUI-206/208/304 그린 | AUDIT-001, PERMISSION-001, HOOK-001 |

각 Phase는 별도 commit + 독립 검증. 각 Phase 종료 시 `scripts/check-brand.sh`, frontend `npm run lint && npm run build && npm test`, Go `go test -race ./internal/webui/... ./cmd/goose/...` 통과 확인.

### 6.8 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | Go `internal/webui/` ≥ 85% coverage, frontend Vitest + React Testing Library, Playwright e2e (install wizard + chat 1턴 + audit pagination), characterization test for AskUserQuestion roundtrip |
| **R**eadable | Korean-first UI copy (ko.json), AI.GOOSE notation enforcement, page-per-route 컴포넌트 단일 책임, shadcn/ui 표준 컴포넌트 |
| **U**nified | 동일 daemon 프로세스 내 listener (별도 binary 없음), CONFIG-001 schema 재사용, BRIDGE-001 wire 재사용, design tokens 단일 소스 |
| **S**ecured | Loopback bind enforcement (REQ-WEBUI-501/502), CSP (REQ-WEBUI-102), provider key redaction (REQ-WEBUI-503), settings 파일 mode 0600, install wizard JSON mode 0600, no `/exec` `/eval` 엔드포인트, DNS rebinding 방어 |
| **T**rackable | 모든 settings 변경에 audit event (CONFIG-001 reload hook 경유), 모든 approval 결정 audit, install wizard 단계 audit, frontend client-side error는 `/webui/telemetry/error` (옵션) 로 daemon zap 로그 |

---

## 7. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. 마지막 줄에 `Covers: REQ-WEBUI-XXX, ...` 메타라인을 포함하여 RTM 자동 추적을 보장한다.

**AC-WEBUI-01 — `goose web` launch end-to-end**
- **Given** `goose web` 명령 실행, daemon 미실행, port 8787 free, default browser 설정
- **When** 명령 실행 후 5 s 대기
- **Then** (a) `goosed` 프로세스 spawn 됨 (`pgrep goosed` 1건), (b) 헬스체크 `GET http://127.0.0.1:{health_port}/healthz` 200, (c) Web UI 응답 `GET http://127.0.0.1:8787/` 200 + HTML, (d) `Content-Security-Policy` 헤더 검증 통과, (e) 기본 브라우저 새 창에서 URL 표시 (macOS `open` / Linux `xdg-open` / Windows `start`), (f) `--no-browser` 플래그 시 (e) 생략되고 stdout에 URL만 출력
- **Covers**: REQ-WEBUI-104, REQ-WEBUI-201

**AC-WEBUI-02 — 외부 IP bind 거부 + DNS rebinding 방어**
- **Given** `cfg.WebUI.Bind = "0.0.0.0"` (외부 인터페이스)
- **When** `goosed` 시작
- **Then** (a) `internal/webui` 시작 단계에서 `webui.non_loopback_bind` 에러로 fail-fast, (b) `127.0.0.1`/`::1`/`localhost` 모두 정상. 별도 케이스: bind는 loopback이지만 요청 헤더 `Host: evil.com` 으로 도달하면 421 응답 + zap WARN 로그 1건
- **Covers**: REQ-WEBUI-501, REQ-WEBUI-502

**AC-WEBUI-03 — Install wizard 7-state 진행 + 미완료 redirect**
- **Given** 첫 실행 (`~/.goose/state/install.json` 미존재)
- **When** 사용자가 `/`, `/settings`, `/audit` 어느 경로로 접근하든
- **Then** (a) 모두 `/install` 로 redirect, (b) wizard intro → provider-select → key-entry (Anthropic 선택, 유효 형태 키 입력) → keyring-write (자동 성공) → daemon-reload (자동) → smoke-test (LLM ping 성공) → done 까지 진행, (c) 각 트랜지션에서 `~/.goose/state/install.json` 에 현재 state 저장, (d) 도중에 새 탭으로 동일 URL 열어도 wizard가 같은 state로 resume, (e) done 직후 `/` 로 redirect, (f) 모든 응답에 `application/json` content-type (API)
- **Covers**: REQ-WEBUI-202, REQ-WEBUI-301

**AC-WEBUI-04 — Provider key 키링 저장 + redaction**
- **Given** install wizard key-entry 단계, fixture key `sk-ant-fixture-XXX-YYY`
- **When** `POST /webui/install/credentials {provider:"anthropic", key:"sk-ant-fixture-XXX-YYY"}`
- **Then** (a) 응답 200 `{key_id: "<uuid>", last4: "Y-YYY"}`, raw key 미포함, (b) `~/.goose/secrets/providers.yaml` 에 `{provider: anthropic, keyring_id: <uuid>}` 만 저장 (값 미저장), (c) OS keyring (macOS Keychain / libsecret / Windows Credential Vault) 에 entry name `goose-provider-anthropic` 으로 raw key 저장 (수동 inspect로 검증), (d) `~/.goose/logs/audit.log` 의 새 event entry에 raw key 미포함, (e) goosed zap log 검색 (last 100 lines) 에 raw key 매치 0건. CREDENTIAL-PROXY-001 가 실 구현될 때까지는 stub injection으로 검증
- **Covers**: REQ-WEBUI-204, REQ-WEBUI-503

**AC-WEBUI-05 — SSE 스트리밍 chat first-token latency**
- **Given** install wizard 완료, BRIDGE-001 SSE 엔드포인트 stub (chunk 1개를 mock LLM에서 50 ms 후 emit)
- **When** Web UI chat에 "hello" 전송
- **Then** (a) `POST /bridge/inbound {type:"chat", payload:"hello"}` 200, (b) SSE event `chunk` 도착 후 DOM mutation까지 p95 ≤ 100 ms (100회 측정), (c) 화면에 incremental markdown 으로 렌더, (d) chunk 끝나면 `status: completed` 이벤트 1건. 측정 host: 4-core x86_64, 8 GB RAM
- **Covers**: REQ-WEBUI-203

**AC-WEBUI-06 — SSE 끊김 + Last-Event-ID resume**
- **Given** active chat, 진행 중 SSE 청크 5개 도착, 청크 #6 emit 직전 네트워크 일시 단절
- **When** 1.2 s 후 자동 reconnect 발생
- **Then** (a) reconnect 요청에 `Last-Event-ID: 5` 헤더 포함, (b) 서버는 청크 #6 부터 emit, (c) 사용자 화면에 누락 0건, (d) 11번째 reconnect 실패 시 client는 시도 중단 + 사용자에게 reload 안내 banner. 백오프 스케줄 1s → 30s ± 20% jitter 검증
- **Covers**: REQ-WEBUI-207, REQ-WEBUI-505

**AC-WEBUI-07 — Settings 즉시 적용 (핫리로드)**
- **Given** `/settings` 페이지 열림, `providers.yaml` `default: claude-sonnet-4.6` 상태
- **When** 사용자가 `default: gpt-5` 로 변경 후 Save
- **Then** (a) `PUT /webui/settings/providers.yaml` 200, (b) `~/.goose/config/providers.yaml` 또는 `./.goose/config/providers.yaml` 갱신 (atomic write, mode 0600), (c) daemon CONFIG-001 reload path 트리거 → in-memory snapshot 갱신 (다음 chat 호출이 gpt-5로 라우팅됨), (d) UI에 "Saved + applied" toast. mtime 충돌 케이스 (외부 편집 동시 발생) 는 409 conflict 응답 + 양쪽 버전 표시
- **Covers**: REQ-WEBUI-205, REQ-WEBUI-506

**AC-WEBUI-08 — Audit viewer pagination + filter**
- **Given** `~/.goose/logs/audit.log` 에 fixture event 200건 (timestamp 균등 분포, 다양한 capability)
- **When** (a) `GET /webui/audit?cursor=&limit=50` 첫 페이지, (b) `GET /webui/audit?cursor=<50번째 ts>&limit=50` 두번째 페이지, (c) `GET /webui/audit?filter[capability]=net&limit=50`
- **Then** (a) 50건 반환, descending by timestamp, (b) 그 이전 50건, (c) `capability=net` 만 필터링되어 반환, (d) 응답 100 ms p95 이내 (200 event scale), (e) 비정상 event row (skill의 declared `requires.fs_read`가 OS user 권한 외) 는 응답에서 제외됨
- **Covers**: REQ-WEBUI-206

**AC-WEBUI-09 — Approval modal roundtrip (60s timeout 포함)**
- **Given** active chat, daemon이 tool 호출 (capability=net, scope=api.openai.com)을 요청, PERMISSION-001 Manager.Check 가 첫 호출이라 Confirmer.Ask 발화
- **When** SSE event `permission_request {request_id, subject_id:"skill:summary", capability:"net", scope:"api.openai.com", reason:"summarization tool"}` 도착
- **Then** (a) Web UI modal 표시 (subject_id, capability, scope, reason 4 필드), (b) chat input field disabled, (c) AlwaysAllow 클릭 → `POST /webui/approve/<request_id> {choice:"AlwaysAllow"}` → BRIDGE-001 inbound → HOOK-001 응답 dispatch → PERMISSION-001 grant 영속화, (d) modal 닫힘, chat 재개. 별도 케이스: 60 s 동안 응답 없으면 client가 자동 `{choice:"Deny", reason:"timeout"}` emit, modal 닫힘, audit event `grant_denied/timeout` 1건
- **Covers**: REQ-WEBUI-208, REQ-WEBUI-303

**AC-WEBUI-10 — CSP + provider key non-leak**
- **Given** Web UI 모든 페이지
- **When** (a) 응답 헤더 검증, (b) 모든 API/SSE 응답 body grep "sk-ant-fixture", (c) zap log 검색
- **Then** (a) `Content-Security-Policy` 헤더 모든 HTML 응답에 존재, `script-src 'self'` 포함 + `unsafe-eval` 미포함, (b) 응답 0건 매치, (c) zap log 0건 매치 (last 1000 lines). 추가: `/webui/exec` `/webui/eval` 라우트 404 확인
- **Covers**: REQ-WEBUI-102, REQ-WEBUI-503, REQ-WEBUI-504

**AC-WEBUI-11 — Bundle size budget enforcement**
- **Given** frontend `npm run build`
- **When** Vite build 완료
- **Then** (a) `dist/assets/*.js` gzip 누적 ≤ 500 KB, (b) 초과 시 build 스크립트 non-zero exit + 어떤 파일이 budget 초과시켰는지 보고, (c) `internal/webui/static/embed.go` `//go:embed dist/*` 가 여전히 컴파일 통과
- **Covers**: REQ-WEBUI-106

**AC-WEBUI-12 — Brand notation + i18n base**
- **Given** Web UI 모든 페이지의 사용자 가시 문자열
- **When** (a) `i18n/locales/ko.json` / `en.json` 키 grep, (b) 페이지 렌더링 후 DOM grep
- **Then** (a) 두 파일에서 `AI.GOOSE` 표기 사용, 부적합 패턴 (`Goose project`, `goose project`, `GOOSE-AGENT` 백틱 외부) 매치 0건, (b) 코드 식별자(`goose CLI`, `goose web`, `goosed daemon`) 는 `<code>` 태그 안에 위치, (c) `localStorage.locale=en` 설정 후 새로고침 시 모든 문자열 영문, (d) `localStorage.locale=ko` (default) 시 한국어
- **Covers**: REQ-WEBUI-105

**AC-WEBUI-13 — Dark mode persistence + WCAG AA contrast**
- **Given** 사용자가 첫 방문, OS `prefers-color-scheme: dark`
- **When** (a) 첫 방문 시 자동 dark, (b) toggle 버튼으로 light 전환, (c) 새로고침
- **Then** (a) 첫 방문 dark, (b) toggle 후 light, `localStorage.theme=light`, (c) 새로고침 후 light 유지. 추가: dark 모드에서 primary `#FFB800` 가 background `#171513` 에 대해 WCAG AA contrast (≥ 4.5:1) 통과, light 모드에서 `#FFB800` 가 `#FAF8F4` 에 대해 동일 검증 (text는 neutral.700 이상 사용)
- **Covers**: REQ-WEBUI-304

**AC-WEBUI-14 — Daemon state-aware overlay**
- **Given** daemon이 graceful shutdown 진행 중 (state=draining)
- **When** Web UI 가 `GET /webui/audit` 호출
- **Then** (a) 503 응답 + `Retry-After: 1` 헤더, (b) SPA가 "shutting down" overlay 표시, (c) 이후 daemon 재시작 + state=serving 으로 전환되면 overlay 자동 사라지고 정상 동작 복귀. 별도 케이스: state=bootstrap 동일 503 + "starting" overlay
- **Covers**: REQ-WEBUI-302

---

## 8. REQ → AC Traceability

| REQ                | AC                                | 비고 |
| ------------------ | --------------------------------- | ---- |
| REQ-WEBUI-101      | AC-WEBUI-01 (route mount), AC-WEBUI-10 (no exec/eval) | route group 정의 |
| REQ-WEBUI-102      | AC-WEBUI-10                       | CSP 헤더 |
| REQ-WEBUI-103      | AC-WEBUI-03 (API content-type)    | JSON 에러 스키마 |
| REQ-WEBUI-104      | AC-WEBUI-01                       | embed.FS launch |
| REQ-WEBUI-105      | AC-WEBUI-12                       | i18n + brand |
| REQ-WEBUI-106      | AC-WEBUI-11                       | bundle budget |
| REQ-WEBUI-201      | AC-WEBUI-01                       | goose web launch |
| REQ-WEBUI-202      | AC-WEBUI-03                       | install required redirect |
| REQ-WEBUI-203      | AC-WEBUI-05                       | SSE chat |
| REQ-WEBUI-204      | AC-WEBUI-04                       | provider key store |
| REQ-WEBUI-205      | AC-WEBUI-07                       | settings reload |
| REQ-WEBUI-206      | AC-WEBUI-08                       | audit pagination |
| REQ-WEBUI-207      | AC-WEBUI-06                       | SSE resume |
| REQ-WEBUI-208      | AC-WEBUI-09                       | approval modal |
| REQ-WEBUI-301      | AC-WEBUI-03                       | wizard guard |
| REQ-WEBUI-302      | AC-WEBUI-14                       | daemon state overlay |
| REQ-WEBUI-303      | AC-WEBUI-09                       | input disable |
| REQ-WEBUI-304      | AC-WEBUI-13                       | dark mode |
| REQ-WEBUI-401      | AC-WEBUI-01 (--port 변종)         | port flag |
| REQ-WEBUI-402      | (run phase manual smoke)          | model picker |
| REQ-WEBUI-403      | (run phase manual smoke)          | audit highlight |
| REQ-WEBUI-501      | AC-WEBUI-02                       | non-loopback bind |
| REQ-WEBUI-502      | AC-WEBUI-02                       | DNS rebind |
| REQ-WEBUI-503      | AC-WEBUI-04, AC-WEBUI-10          | key redaction |
| REQ-WEBUI-504      | AC-WEBUI-10                       | endpoint surface limit |
| REQ-WEBUI-505      | AC-WEBUI-06                       | reconnect storm |
| REQ-WEBUI-506      | AC-WEBUI-07                       | concurrent edit |

REQ-WEBUI-402 / REQ-WEBUI-403 는 v0.2.0에서 "Optional" 카테고리로 명시되며, 자동 AC가 없고 run phase의 수동 smoke 체크리스트로 검증한다 (tasks.md 참조).

---

## 9. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Loopback binding 우회 (잘못된 bind 설정 / IPv6 dual-stack 누수 / DNS rebinding) | 중 | 매우 고 | (a) start 단계 strict bind 검증 (REQ-WEBUI-501), (b) Host 헤더 검증 + 421 응답 (REQ-WEBUI-502), (c) CSP `connect-src 'self' http://127.0.0.1:* http://[::1]:*` 로 외부 origin fetch 차단, (d) 통합 테스트로 Host:evil.com 포함 fixture 검증, (e) IPv6 dual-stack은 `::1` 만 명시 — `::ffff:127.0.0.1` mapped 형태도 허용하되 외부 IP는 거부 |
| R2 | Install wizard에서 raw provider key가 어디로든 leak (로그 / 응답 body / audit / SSE) | 중 | 매우 고 | (a) zap field redaction (key는 절대 field에 포함 안 함, `key_id` + `last4` 만), (b) 응답 schema 강제 (testdata fixture key를 grep하는 회귀 테스트), (c) audit event 스키마에 raw key 필드 부재 보장, (d) frontend Form은 `type="password"` + autocomplete=off, (e) 브라우저 history에 입력값 기록 안 되도록 `/install` SPA 라우트는 입력 후 즉시 다음 state로 redirect |
| R3 | SSE reconnect storm (네트워크 불안 + max-attempts 누락) | 중 | 중 | (a) BRIDGE-001 의 reconnect 정책 (REQ-BR-018) 준수: 1s→30s exponential, max 10, (b) client 가 11번째에서 stop + 사용자 banner, (c) 서버측 BRIDGE-001 close 4429 rate-limit, (d) 통합 테스트로 10회 실패 시나리오 검증 |
| R4 | Bundle size 폭증 (의존성 추가로 cold start 1초 초과) | 중 | 중 | (a) `npm run build` 후 size budget gate (REQ-WEBUI-106), (b) shadcn/ui 컴포넌트는 사용한 것만 import, (c) Tailwind v4 JIT, (d) 이미지/폰트는 가능한 경우 system-ui 우선 + Inter/Pretendard self-host (CDN 의존 없음), (e) lazy-load `/audit` 페이지 (lazy boundary) — 초기 진입은 chat + install 만 |
| R5 | i18n 미흡 — Korean primary 결정으로 영어권 OSS 기여자가 사용 어려움 | 중 | 중 | (a) `localStorage.locale=en` 으로 즉시 전환 가능 (REQ-WEBUI-105), (b) 모든 키는 ko/en 동시 작성 의무, (c) `npm run build` 시 locale 키 누락 검증 (CI 단계), (d) 후속 SPEC에서 일본어/중국어 추가 시 동일 패턴 |
| R6 | Settings 핫리로드가 daemon in-memory snapshot과 disagree (외부 편집 / 동시 다중 탭) | 중 | 고 | (a) mtime + content hash 비교 (REQ-WEBUI-506), (b) 409 conflict 응답 + 양쪽 버전 표시, (c) settings 저장은 atomic write (temp + rename), (d) reload 후 `audit.log` 에 결정 이벤트 기록 |
| R7 | Approval modal 60s timeout 후 default-deny가 사용자 의도와 다름 (사용자가 자리비움) | 중 | 중 | (a) timeout 동작은 BRIDGE-001 REQ-BR-008 의 default-deny와 일관, (b) modal 에 60s countdown 표시, (c) timeout 결과를 audit `grant_denied/timeout` 으로 기록해서 사용자가 history에서 확인 가능, (d) 후속 호출 시 다시 confirm flow (PERMISSION-001 의 `OnceOnly` 패턴 활용) |
| R8 | Brand notation 위반 — frontend i18n 파일에 부적합 brand 패턴 (예: 백틱 외부 `Goose project`) 누수 | 낮 | 중 | (a) `scripts/check-brand.sh` 가 `.md` 파일만 검사하지만, build 단계에서 `i18n/locales/*.json` 도 grep 하는 추가 lint 단계 (REQ-WEBUI-105 + AC-WEBUI-12), (b) 코드 리뷰 시 brand-lint pre-commit |
| R9 | shadcn/ui v4 / Tailwind v4 메이저 변경으로 디자인 토큰 매핑 break | 낮 | 중 | (a) 토큰을 단일 소스 `design-tokens.json` 에 두고 Tailwind config가 import (단일 변경 지점), (b) shadcn 컴포넌트 wrap layer (`components/ui/`) 로 upgrade impact 차단, (c) 후속 SPEC 에서 shadcn 5 메이저 시 별도 amendment |
| R10 | 같은 브라우저의 다른 origin (예: localhost:3000 dev server) 이 fetch로 Web UI 데이터 빼냄 (CSRF) | 중 | 고 | (a) BRIDGE-001 CSRF double-submit 토큰 재사용, (b) `SameSite=Strict` 쿠키, (c) Origin 헤더 검증 (REQ-WEBUI-502 와 동일 메커니즘), (d) `/webui/*` 도 동일하게 CSRF 검증 (BRIDGE-001 의 패턴 차용) |

---

## 10. Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **WebSocket/SSE wire protocol 자체를 정의하지 않는다**. 본 SPEC은 BRIDGE-001 클라이언트일 뿐이며, frame 스키마/close code/CSRF 메커니즘은 BRIDGE-001 §6에 위임.
- 본 SPEC은 **token auth grant store, first-call confirm 정책 자체를 구현하지 않는다**. PERMISSION-001 담당. 본 SPEC은 Web UI Confirmer 어댑터만 제공.
- 본 SPEC은 **audit.log 본체 (rotation, append-only attribute) 를 구현하지 않는다**. AUDIT-001 담당. 본 SPEC은 read-only paginated viewer만.
- 본 SPEC은 **AskUserQuestion event 발화 자체를 구현하지 않는다**. HOOK-001 담당. 본 SPEC은 발화된 event를 SSE로 받아 modal로 표시하는 부분만.
- 본 SPEC은 **외부 네트워크 노출 / 모바일 원격 접속을 포함하지 않는다**. BRIDGE-002 후보.
- 본 SPEC은 **multi-user authentication / federated identity (OIDC/SAML/SSO) 를 포함하지 않는다**. 단일 OS 사용자 가정. 후속 SPEC.
- 본 SPEC은 **realtime 다중 탭 collab (한 chat을 여러 탭에서 동시 편집) 을 보장하지 않는다**. 단일 active session 가정.
- 본 SPEC은 **plugin marketplace UI / 사용자 정의 skill upload UI 를 포함하지 않는다**. v0.2.0 settings 페이지는 enumerated YAML 파일 편집만.
- 본 SPEC은 **browser extension / PWA offline / Service Worker push notification을 포함하지 않는다**.
- 본 SPEC은 **TLS / HTTPS를 적용하지 않는다**. loopback 전용이므로 불필요. 외부 노출은 §3.2와 REQ-WEBUI-501 에서 금지.
- 본 SPEC은 **mobile native shell (React Native / Capacitor / Tauri)을 포함하지 않는다**. 후속 SPEC.
- 본 SPEC은 **WCAG AA 인증을 포함하지 않는다**. keyboard navigation + 충분한 색상 대비만 보장.
- 본 SPEC은 **일본어/중국어 i18n을 포함하지 않는다**. ko + en 만.
- 본 SPEC은 **사용자 정의 테마 (custom CSS / 외부 폰트 import) 를 포함하지 않는다**.
- 본 SPEC은 **멀티 워크스페이스 (한 브라우저에서 여러 `./.goose/` 동시 관리) 를 포함하지 않는다**.
- 본 SPEC은 **AI.GOOSE persona의 voice/tone tuning UI를 포함하지 않는다**. persona/voice.md 편집은 후속 SPEC (CONTEXT-001 후속 amendment 후보).

---

## 11. Open Items (run phase / 후속 SPEC 이관)

| ID | 항목 | 관련 REQ/AC | 이관 사유 |
|----|-----|-----------|---------|
| OI-01 | Bundle size 측정의 정확한 기준 (Brotli 포함? CSS 포함?) | REQ-WEBUI-106 | research.md §9 에 후보 알고리즘 3종 비교, run phase에서 결정 |
| OI-02 | Incremental markdown parser — 자체 구현 vs `streaming-markdown` lib | §6.4 | run phase에서 prototype 후 결정 |
| OI-03 | `useSSE` hook — `EventSource` 표준 vs `fetch` + `ReadableStream` | §6.4 | EventSource 가 reconnect 자동이지만 헤더 (Last-Event-ID 외) 커스터마이징 제약 — research.md §6 |
| OI-04 | Dark mode WCAG AA contrast 자동화 검증 (axe-core / Lighthouse) | REQ-WEBUI-304 | run phase Phase 5 에서 e2e 테스트 |
| OI-05 | install wizard provider 목록 (Anthropic/OpenAI/Google/Ollama 외 추가?) | §6.3 | research.md §7, run phase에서 v0.2 baseline 확정 |
| OI-06 | i18n 키 누락 검증 — runtime 검사 vs build-time | REQ-WEBUI-105 | run phase 에서 결정 |
| OI-07 | BRIDGE-001 amendment 필요 여부 (Web UI 가 새 SSE event type을 요구하나?) | §2.3 | research.md §9 — 현재로는 BRIDGE-001 v0.2.0 contract 로 충분해 보이지만 prototype 단계에서 재확인 |
| OI-08 | Frontend monorepo vs 독립 모듈 (npm workspace) | §6.1 | run phase Phase 1 에서 결정 |

---

## 12. 참고 (References)

### 12.1 프로젝트 문서

- `.moai/design/goose-runtime-architecture-v0.2.md` §6 메신저 채널 (v0.1 Alpha 채널 정의)
- `.moai/specs/SPEC-GOOSE-ARCH-REDESIGN-v0.2/spec.md` (parent meta-SPEC)
- `.moai/specs/SPEC-GOOSE-BRIDGE-001/spec.md` (WebSocket/SSE wire 계약)
- `.moai/specs/SPEC-GOOSE-PERMISSION-001/spec.md` (Confirmer 인터페이스 + grant flow)
- `.moai/specs/SPEC-GOOSE-AUDIT-001/spec.md` (append-only audit.log)
- `.moai/specs/SPEC-GOOSE-HOOK-001/spec.md` (AskUserQuestion event hook)
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` (security.yaml / providers.yaml / channels.yaml 스키마)
- `.moai/specs/SPEC-GOOSE-DAEMON-WIRE-001/spec.md` (wire-up 패턴)
- `.moai/specs/SPEC-GOOSE-CREDENTIAL-PROXY-001/spec.md` (keyring 저장 경로)
- `.moai/project/brand/style-guide.md` (brand notation)
- `.moai/project/brand/design-tokens.json` (색상/타이포/모션 토큰)
- `cmd/goosed/main.go`, `cmd/goosed/wire.go` (wire 통합 진입점)
- `scripts/check-brand.sh` (brand-lint 검증)

### 12.2 외부 참조

- Vite Docs: https://vitejs.dev/
- React 18 streaming: https://react.dev/
- Tailwind v4 announce: https://tailwindcss.com/blog/tailwindcss-v4-alpha
- shadcn/ui v4: https://ui.shadcn.com/
- i18next: https://www.i18next.com/
- MDN EventSource (SSE): https://developer.mozilla.org/en-US/docs/Web/API/EventSource
- MDN Last-Event-ID: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#receiving_events_from_the_server
- OWASP Cheat Sheet — Cross-Site Request Forgery Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- Open WebUI (참고 패턴, 코드 미차용): https://github.com/open-webui/open-webui
- Ollama UI (참고 패턴, 코드 미차용): https://github.com/ollama/ollama

### 12.3 부속 문서 (본 SPEC 디렉토리)

- `research.md` — 코드베이스 분석, frontend tech 평가, BRIDGE-001 contract gap 분석.
- `tasks.md` — TDD-mode task decomposition, M0..M5 milestone, AC → task 매핑.

---

## 11. Open Questions Resolution (v0.2.1)

본 절은 `research.md §9` 에 기록된 3 개 Open Question 의 최종 결정을 명문화한다. v0.2.1 amendment 에서 추가됨. REQ/AC 본문은 v0.2.0 그대로 유지되며, 본 결정은 run phase 진입 시 manager-tdd 의 implementation 가이드로 사용된다.

### 11.1 OI-A — `/bridge/login` schema

**결정**: **명시 POST 채택**.

| 항목 | 내용 |
|------|------|
| Endpoint | `POST /bridge/login` |
| Request body | `{"intent": "first_install" \| "resume"}` (install wizard 마지막 스텝 또는 chat 페이지 진입 직전) |
| Response | `200 OK` + `Set-Cookie: goose_session=...; HttpOnly; SameSite=Strict; Path=/; Max-Age=86400` + body `{"csrf_token": "..."}` |
| 트리거 | install wizard 의 "save key" 스텝 완료 후 자동 호출 / chat 페이지 진입 시 쿠키 부재 감지 시 자동 redirect |

**거절된 대안**: 자동 cookie set (install wizard 완료 시 daemon 이 비대칭으로 cookie 발급).
**근거**: REST 컨벤션 + 명시적 인증 트랜지션 + 통합 테스트 작성 용이 + 브라우저 보안 모델(POST 만 cookie set 허용 일반 패턴) 정렬.
**관련**: BRIDGE-001 REQ-BR-002 (cookie lifecycle) + REQ-BR-006 (CSRF 검증) + WEBUI REQ-WEBUI-202 (install wizard redirect) + REQ-WEBUI-204 (provider key entry) + REQ-WEBUI-301 (auth guard).

### 11.2 OI-B — WEBUI listener 포트 정책

**결정**: **shared port + path 분리 채택** (BRIDGE-001 의 단일 listener 위에 `/webui/*` mount).

| 항목 | 내용 |
|------|------|
| BRIDGE-001 listener | `127.0.0.1:8091` (또는 사용자 설정) |
| WEBUI 정적 번들 | `GET /webui/*` (embed.FS 서빙) |
| WEBUI 관리 API | `POST /webui/install/*`, `POST /webui/settings/*`, `GET /webui/audit/*` |
| BRIDGE wire | `/bridge/ws`, `/bridge/stream`, `/bridge/inbound`, `/bridge/login`, `/bridge/logout` |
| Origin | 단일 loopback origin (`http://127.0.0.1:8091`) — CORS 불필요 |

**거절된 대안**: 별도 포트(예: `8787` for WebUI) — CORS preflight 와 cookie domain 분리 비용 큼.
**근거**: 단일 origin = browser security model 단순화, CORS 불필요, 사용자 친화 (포트 1 개만 기억). BRIDGE-001 §3.1 item 5 ("두 path 는 같은 listener 에서 제공") 와 정합. §1 의 v0.2.0 초안의 "별도 포트" 표현은 본 amendment 에서 정정됨.
**관련**: WEBUI REQ-WEBUI-005 (loopback bind) + REQ-WEBUI-104 (embed) + BRIDGE-001 REQ-BR-003 (동일 listener 의 WS+SSE). REQ-WEBUI-501/502 의 외부 bind reject 정책은 BRIDGE-001 REQ-BR-005 와 공유.

### 11.3 OI-C — channel-aware Confirmer routing

**결정**: **HOOK-001 의 책임으로 분리**. 본 SPEC 범위 외.

| 항목 | 내용 |
|------|------|
| 책임 위치 | HOOK-001 (별도 amendment 후속 작업) |
| WEBUI 책임 | hook event 수신 + permission_request modal 표시 + 사용자 선택 응답 |
| 결정 알고리즘 | "active session 우선 채널 선택" — HOOK-001 amendment 가 정의 (예: 마지막 inbound 가 webui 면 webui Confirmer 사용, 그렇지 않으면 CLI/Telegram 우선순위 표 적용) |
| 멀티 채널 동시 활성 | HOOK-001 가 channel registry + activity timestamp 기반 선택 |

**거절된 대안**:
- WEBUI-001 가 직접 라우팅: 채널 별 책임 분산 → orchestration 복잡도 증가, single-source-of-truth 위배.
- PERMISSION-001 에 routing 추가: PERMISSION-001 은 grant/revoke decision 만 담당, routing 은 별도 관심사.

**근거**: HOOK-001 의 사용자 prompt event 가 발화 위치에 가장 가깝고, channel registry 도 HOOK-001 가 보유하기에 자연스러운 책임. WEBUI 는 receiving end 만 담당.
**관련**: WEBUI REQ-WEBUI-208 (modal) + REQ-WEBUI-303 (input disable during pending response) + HOOK-001 (별도 amendment 후속).
**Follow-up**: HOOK-001 amendment SPEC 작성 시 본 결정을 reference. amendment ID 미정 — `/moai plan` 트리거로 별도 작성.

### 11.4 결정 영향 요약

| Open Question | 결정 | 영향 받는 REQ | run phase 영향 |
|---------------|------|---------------|----------------|
| OI-A | 명시 POST `/bridge/login` | REQ-WEBUI-202, 204, 301 + BRIDGE-001 REQ-BR-002, 006 | install wizard 마지막 스텝의 fetch 호출 + cookie/CSRF 응답 처리 |
| OI-B | shared port + path 분리 | REQ-WEBUI-005, 104, 501, 502 + BRIDGE-001 REQ-BR-003, 005 | `cmd/goose/web.go` 가 BRIDGE listener URL 만 사용. WEBUI mount 가 `internal/bridge/` 의 mux 에 add |
| OI-C | HOOK-001 amendment 책임 | REQ-WEBUI-208, 303 | WEBUI 는 hook event subscriber 로 단순화. HOOK-001 amendment 별도 진행 |

3 결정 모두 v0.2.0 의 REQ/AC 본문을 변경하지 않는다. Implementation 단계 manager-tdd 가 위 결정을 그대로 따른다.

---

**End of SPEC-GOOSE-WEBUI-001**
