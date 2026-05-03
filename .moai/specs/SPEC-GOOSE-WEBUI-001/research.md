# SPEC-GOOSE-WEBUI-001 — Research Notes

> 작성일: 2026-05-04
> 작성자: manager-spec
> 목적: spec.md plan 보강의 근거 자료 및 후속 amendment 의사결정 입력.

본 문서는 SPEC-GOOSE-WEBUI-001 v0.2.0 plan 단계에서 코드베이스, 의존 SPEC, 외부 레퍼런스를 조사한 결과를 모은다. 본 문서의 결정은 spec.md §6/§9/§11 에 반영되었다. 본 문서는 "왜 그렇게 결정했는가"의 evidence layer 이므로 spec.md 가 이미 반영한 결정은 다시 정당화하지 않고 정황만 기록한다.

---

## 1. daemon HTTP/Connect surface 현황

### 1.1 현재 상태

`cmd/goosed/main.go` 의 13-step 생애주기 (SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002) 에서 HTTP listener는 다음 두 종류만 존재한다:

1. **Health endpoint** — `internal/health` 패키지가 `cfg.Transport.HealthPort` 에서 `/healthz`, `/readyz` 류를 노출. step 11 에서 시작.
2. **Slash command + RPC subsystem (placeholder)** — `wireSlashCommandSubsystem` 에서 `dispatcher`, `ctxAdapter` 를 생성하지만 SPEC-GOOSE-TRANSPORT-001 wire 가 미완료라 아직 listener가 없다. main.go 에 `_ = dispatcher; _ = ctxAdapter` 표시로 향후 wire 예정.

→ **결론**: WebUI 는 health listener 와 별도의 새 listener 를 띄워야 한다. step 10.9 위치에 `wireWebUISubsystem` 을 끼우고, listenAndServe 는 step 12 (serving 전환) 직전에 startGoroutine으로 spawn. drain consumer는 `rt.Drain.RegisterDrainConsumer` 패턴으로 등록 — slash command subsystem이 동일 패턴을 사용 중이라 그대로 차용.

### 1.2 Connect-protocol vs plain HTTP for SSE

TRANSPORT-001 가 gRPC + Connect protocol 을 채택하고 있으나 (research.md 결정문 미존재 — TRANSPORT-001 spec.md 직접 참조 필요), SSE 는 plain HTTP 기반이고 Connect 와 충돌하지 않는다. WebUI 가 Connect endpoint 로 마운트될 필요 없음. plain `net/http.ServeMux` + `http.Server` 로 충분. embed.FS 정적 자원도 `http.FileServer(http.FS(staticFS))` 패턴 표준.

### 1.3 Concrete embed 진입점

```go
// internal/webui/static/embed.go (예정)
package static

import "embed"

//go:embed dist/*
var FS embed.FS
```

`dist/` 가 비어 있으면 `go build` 가 `pattern dist/*: no matching files found` 에러. → frontend 빌드를 first-time 진행하지 않으면 Go 빌드 자체가 실패. 대안은 `//go:embed all:dist` 변형 + placeholder `dist/.gitkeep`. run phase Phase 1 에서 결정.

---

## 2. BRIDGE-001 contract 분석

### 2.1 본 SPEC 이 사용하는 BRIDGE-001 표면

| BRIDGE-001 표면 | WEBUI-001 사용처 |
|---|---|
| `/bridge/ws` (WebSocket) | v0.2.0 에서는 사용하지 않음 (SSE 우선) |
| `/bridge/stream` (SSE GET) | chat 응답 streaming, permission_request 수신, status 이벤트 |
| `/bridge/inbound` (HTTP POST) | chat user prompt 전송, attachment, permission_response 전송 |
| `/bridge/login` (POST, 추정) | 24h 쿠키 발급 — install wizard 완료 시점에 호출 |
| `/bridge/logout` (POST) | settings 페이지 logout 버튼 |
| WebSocket close codes (4401/4403/4408/4413/4429/4500) | SSE는 HTTP status + `event: error` 매핑 (REQ-BR-014 단서) |
| Cookie `goose_session` (HttpOnly, SameSite=Strict, 24h) | SPA 자동 포함 (same-origin) |
| CSRF double-submit `X-CSRF-Token` 헤더 | `lib/api.ts` fetch 래퍼가 메모리 변수 + `csrf_token` 쿠키 양측에서 읽어 자동 첨부 |
| `Last-Event-ID` 헤더 (resume) | `useSSE` hook 에서 reconnect 시 lastSequence 전달 |
| Replay buffer 4MB / 500 메시지 (REQ-BR-009) | client 입장에서는 의식하지 않아도 됨 (서버가 자동 replay) |

### 2.2 BRIDGE-001 미정 사항 (gap)

BRIDGE-001 v0.2.0 spec.md 를 정독한 결과 다음 항목이 WEBUI-001 의 입장에서 모호하다:

1. **`/bridge/login` 엔드포인트 정확한 schema** — REQ-BR-002 가 "stub auth or local handshake" 라고만 표기. install wizard 완료 시점에 어떤 payload 로 호출해야 24h 쿠키를 받는가? 
   - **현재 가정 (research.md §9.1 OI-A 로 추적)**: install 완료 시 `goosed` 가 자동으로 first session 쿠키를 set 한다고 가정 (SPA 가 `/install/done` 직후 새로 발급). 단순화를 위해 실제 login 폼은 v0.2.0 에 없음.
2. **AskUserQuestion → SSE event 매핑 schema** — BRIDGE-001 REQ-BR-008 `OutboundPermissionRequest` event 구조는 명시되었지만 PERMISSION-001 의 `PermissionRequest` 와의 정확한 필드 매핑은 BRIDGE-001 §6.3 핵심 타입 시그니처에 `Type: OutboundPermissionRequest, Payload: []byte` 만 있고 payload 스키마는 없다.
   - **현재 가정 (OI-B)**: payload 는 JSON `{request_id, subject_id, subject_type, capability, scope, reason, requested_at}`. 본 SPEC 의 REQ-WEBUI-208 도 동일 schema 가정.
3. **Web UI 가 BRIDGE-001 listener 와 같은 포트를 공유할지 별도 포트인지** — BRIDGE-001 spec.md §1 "단일 머신 내부에서 goosed daemon 과 로컬 브라우저의 Web UI 를 연결" 라는 표현은 있으나 포트 결정 미명시.
   - **현재 결정 (spec.md §6.2 + AC-WEBUI-01)**: 본 SPEC 은 `cfg.WebUI.BindPort = 8787` (default) 별도 포트를 사용한다. BRIDGE-001 은 다른 포트 (예: `cfg.Bridge.BindPort = 8091`). frontend 는 same-host cross-port fetch 를 한다 — 이를 위해 BRIDGE-001 은 CORS 허용 (same-host any-port loopback) 이 필요하다.
   - **OI-C**: 또는 더 단순하게 같은 포트 + path 분리. BRIDGE-001 가 `/bridge/*` 만 차지하고 WEBUI 가 그 외 모든 경로. 이 경우 BRIDGE-001 의 listener 가 ServeMux 에 WEBUI handler를 chain mount. run phase Phase 1 에서 결정.

### 2.3 Web UI 가 BRIDGE-001 amendment 를 강제하는가?

위 gap 을 모두 분석한 결과, **WEBUI-001 v0.2.0 만족을 위해 BRIDGE-001 spec amendment 가 필수인 항목은 없다**. payload 매핑과 포트 결정 모두 BRIDGE-001 implementation 단계에서 자연스럽게 합의 가능. 단 OI-C (같은 포트 vs 다른 포트) 결정은 BRIDGE-001 implementation 시점에 같이 합의해야 한다 — 이 합의를 본 SPEC 의 OI-07 로 추적.

---

## 3. PERMISSION-001 token model 통합

### 3.1 PERMISSION-001 surface 재정리

PERMISSION-001 spec.md §6.2 Public API 에서 본 SPEC 이 사용할 인터페이스:

```go
type Confirmer interface {
    Ask(ctx context.Context, req PermissionRequest) (Decision, error)
}
```

본 SPEC 의 `internal/webui/confirmer/webui_confirmer.go` 가 `Confirmer` 를 구현하고, `Manager.New(store, confirmer, auditor, blocked)` 호출 시 channel 식별자 `"webui"` 와 함께 등록된다 (단, PERMISSION-001 v0.2.0 인터페이스에는 channel 필드가 없으므로 사실상 단일 등록 — multi-channel 분기는 manager 측 로직).

### 3.2 channel-aware Confirmer 분기

문제: AI.GOOSE 는 v0.1 Alpha 에서 3 채널 (CLI/TUI, Telegram, Web UI) 동시 운용. 사용자가 CLI 에서 시작한 task 의 첫 호출 confirm 이 발생하면 어느 channel 에서 prompt 를 띄워야 하는가?

**결정 (spec.md §6.2 + research §3.2)**: 본 SPEC 은 "Web UI 가 active session 일 때" 의 confirm 만 수행한다. CLI 가 활성 세션이면 CLI Confirmer 가 응답. 이는 PERMISSION-001 의 `PermissionRequest.SubjectID` 에 session 정보가 없는 현재 인터페이스로는 단순히 "마지막 active session 의 channel 에 prompt" 휴리스틱으로 진행. 멀티 채널 동시 active 의 결정 로직은 본 SPEC 범위 외 (HOOK-001 + PERMISSION-001 후속 amendment 후보).

### 3.3 install wizard 와 PERMISSION-001 grant store 의 관계

install wizard 시점에는 daemon 이 첫 부팅 직후이며 PERMISSION-001 grant store (`~/.goose/permissions/grants.json`) 가 비어 있다. wizard 의 "smoke test" 단계 (LLM ping) 에서 첫 capability `net api.openai.com/api.anthropic.com` 호출 발생 → Confirmer.Ask 발화 → 본 SPEC 의 webui Confirmer 어댑터가 SSE permission_request 로 frontend 에 표시 → 사용자가 `AlwaysAllow` 선택 → grant 영속화. 즉 install wizard smoke test 가 첫 PERMISSION grant 발생 지점이라는 것이 자연스러운 결과.

---

## 4. brand context summary

### 4.1 design tokens 핵심

- **Primary**: `#FFB800` (밝은 옐로우 — CTA, 강조). 흑색 배경에 대비 4.5:1+ → WCAG AA 통과.
- **Neutral light bg**: `#FAF8F4` (warm off-white, 종이 톤).
- **Neutral dark bg**: `#171513` (deep warm black).
- **Accent**: `#E56B7C` (코랄 핑크 — sparingly).
- **Secondary**: `#6B9E74` (sage green — supportive).
- **Semantic**: success `#4C9B6A`, warning `#E89A3C`, danger `#D45A4F`, info `#5B8FB5`.
- **Typography**: primary `Inter, Pretendard`, display `Fraunces, Pretendard, Georgia`, mono `JetBrains Mono, D2Coding`.
- **Korean letter-spacing**: `-0.01em`.
- **Radius default**: `12px` (md, 버튼/입력).
- **Motion**: acknowledge 180ms, settle 320ms, growth 800ms (chat 메시지 도착은 acknowledge, 페이지 전환은 settle).

### 4.2 brand notation 원칙 (style-guide.md FROZEN)

| 컨텍스트 | 표기 |
|---|---|
| 산문 / brand | `AI.GOOSE` |
| 코드 식별자 (백틱 안) | `goose` (예: `` `goose CLI` ``) |
| URL / repo slug | `ai-goose` (예: `ai-goose.dev`) |

본 SPEC 은 모든 사용자 가시 문자열에서 `AI.GOOSE` 사용. 위반 검증은 `scripts/check-brand.sh` 가 .md 파일 대상으로만 동작 — frontend i18n json 에는 자동 적용되지 않으므로 Phase 1 에서 동일 grep 을 i18n 파일에 적용하는 추가 lint script 가 필요 (research §9 OI 추적).

### 4.3 visual-identity.md 의 _TBD_ 상태

`brand-voice.md` 와 `visual-identity.md` 의 많은 필드가 `_TBD_` 상태. design-tokens.json 은 1.0.0 으로 가득 채워져 있어 해당 토큰을 정본으로 간주. brand-voice.md 의 tone/preferred_terms 는 미정이므로 본 SPEC 의 frontend copy 는 다음 휴리스틱으로 작성:

- **tone**: 친근하지만 신뢰감 있는, 절제된 친밀함 (Daily Companion AI 컨셉)
- **preferred_terms** 잠정: "AI.GOOSE", "함께", "오늘", "기록", "성장"
- **avoided_terms** 잠정: "혁신적인", "잠재력 발휘", "솔루션", "고객"
- **honorific 정책**: 한국어는 "~합니다" (격식체 평어).

이 휴리스틱은 v1.0 정식 brand interview 완료 시 갱신 (별도 SPEC).

---

## 5. frontend tech 평가

### 5.1 후보군

| 후보 | 베이스라인 gzip JS | RSC/SSR 의존 | embed.FS 호환 | 학습곡선 (팀) | chat streaming 친화 |
|---|---|---|---|---|---|
| Vite + React 18 SPA | ~280 KB | 없음 | 매우 우수 (단일 dir 정적 파일) | 낮음 (기존 컨벤션) | 우수 (EventSource + setState) |
| Next.js 15 App Router (RSC + streaming) | ~1.2 MB | RSC server bundle 필요 | 어려움 (Go가 SSR runtime 흉내내야 함, 또는 Next.js export 모드 — 그러면 RSC 의의 상실) | 중 (RSC 학습) | RSC streaming 가능하나 server-side 가 Go 인 환경에서 부적합 |
| Astro + React island | ~600 KB | partial hydration runtime | 가능 (정적 build) | 중 | island 모델이 SSE chat 에 어색 |
| HTMX + 서버 partial render | ~50 KB | 서버 template 필요 | Go template 필요 | 중 | swap 모델로 chunk-by-chunk render 어려움 |
| Solid.js | ~150 KB | 없음 | 우수 | 높음 (팀 미숙) | 우수 |
| SvelteKit | ~300 KB | adapter 필요 | adapter-static 시 가능 | 중-높음 | 우수 |

### 5.2 결정: Vite + React 18 SPA

이유:
1. 프로젝트 컨벤션 (shadcn/ui v4 + Tailwind v4) 와 가장 정합.
2. embed.FS 단일 dir 모델이 daemon 빌드를 단순화.
3. cold-start <1초 first paint 목표가 대부분 후보 중 가장 안전 (gzip ~280 KB baseline + lazy /audit /settings 으로 초기 진입 더 가벼움).
4. SSE chat streaming 은 표준 SPA 패턴 (`useSSE` hook + setState 점진 누적).
5. Next.js 15 RSC 의 server bundle 을 Go daemon 에 끼우는 것은 "Node.js runtime 동봉" 또는 "Next.js export 모드 (RSC 의의 무력화)" 양쪽 다 부적절.

### 5.3 컴포넌트 선택

- shadcn/ui v4: Button, Dialog, Input, Select, Tabs, Toast, Skeleton, Tooltip, ScrollArea — 사용한 것만 install (전체 install 안 함, bundle 절약).
- 자체 컴포넌트: `MessageStream`, `MarkdownStream`, `ApproveModal`, `WizardStep`, `SettingsForm`, `AuditTable`, `BrandLogo`, `ThemeToggle`.
- 외부 라이브러리:
  - `i18next` + `react-i18next` (i18n)
  - 마크다운 streaming: `streaming-markdown` (검토) 또는 자체 구현 (chunk 누적 후 `marked` parse + DOMPurify)
  - icons: `lucide-react` (shadcn 표준)

---

## 6. SSE vs WebSocket — Web UI 측 선택

### 6.1 BRIDGE-001 의 양쪽 지원

BRIDGE-001 v0.2.0 은 WebSocket (`/bridge/ws`) 과 SSE (`/bridge/stream` + `/bridge/inbound`) 를 동시에 제공. 클라이언트 선택 가능.

### 6.2 Web UI 의 결정: SSE 우선

이유:
1. **단방향 streaming + 명확한 inbound endpoint**: chat 응답이 server → client 일방향이고 user prompt 는 별도 POST 로 분리되는 것이 react state 관리에 단순.
2. **Last-Event-ID resume 표준**: EventSource 가 자동으로 lastEventId 를 헤더에 포함 → reconnect 로직이 단순.
3. **HTTP/1.1 호환**: 기업 보안 소프트웨어가 WebSocket 을 막는 경우에도 SSE 통과 (loopback 환경에서는 거의 무관하지만 패턴 일관성).
4. **EventSource 표준 reconnect**: 자동 reconnect (간격은 server `retry:` 필드 또는 default 3s) 가 BRIDGE-001 §6.2 reconnect 정책과 호환되도록 클라이언트 wrapper 에서 lifecycle 제어.

### 6.3 EventSource 표준 vs fetch+ReadableStream (OI-03)

- **EventSource 장점**: 자동 reconnect + Last-Event-ID + 표준화.
- **EventSource 단점**: 헤더 커스터마이징 불가 (예: Authorization 헤더 못 보냄 — 단 same-origin cookie 는 자동 포함).
- **fetch + ReadableStream 장점**: 모든 헤더 + body 커스터마이징, AbortController 연동.
- **fetch + ReadableStream 단점**: SSE 파싱 (`event:` `data:` `id:`) 직접 구현, reconnect 직접 구현.

본 SPEC 의 동작은 cookie 기반 same-origin 이므로 EventSource 의 헤더 제약이 문제 안 됨. **EventSource 우선, 필요 시 wrapper 에서 fetch fallback** 으로 결정. OI-03 으로 run phase 검증.

---

## 7. install wizard state model

### 7.1 7-state 흐름

```
intro → provider-select → key-entry → keyring-write → daemon-reload → smoke-test → done
```

### 7.2 각 state 의 사이드이펙트

| State | 사이드이펙트 | persistence |
|---|---|---|
| intro | 없음 | install.json `state="intro"` |
| provider-select | 없음 | install.json `state="provider-select", provider=null` |
| key-entry | 폼 입력 (메모리만) | install.json `state="key-entry", provider="anthropic"` |
| keyring-write | OS keyring 에 entry 작성 + `~/.goose/secrets/providers.yaml` 에 keyring_id 참조 작성 | install.json `state="keyring-write", key_id=<uuid>` |
| daemon-reload | daemon 에 SIGHUP 또는 file watcher 트리거 | install.json `state="daemon-reload"` |
| smoke-test | LLM ping ("hello") 1회 + 결과 검증 (status=ok 응답) — 이 시점에 PERMISSION-001 첫 grant 발생 (smoke-test 흐름이 자동 AlwaysAllow 선택을 표시) | install.json `state="smoke-test", attempt=N` |
| done | install.json `completed=true` | 영속 |

### 7.3 atomic write + concurrency

`install.json` 은 mode 0600. 동시 다중 탭 시도 → 첫 탭이 `keyring-write` 진행 중에 두 번째 탭이 `key-entry` 를 fresh start? → 첫 탭의 결과가 더 진척된 상태이므로 두 번째 탭은 `state` 를 다시 읽어 resume. SPA 는 5초마다 `GET /webui/install/state` polling 으로 상태 동기화 (install 단계 한정).

### 7.4 실패 처리

- keyring-write 실패 (OS keyring backend unavailable): error 상태 + key-entry 로 retry 옵션 제공.
- smoke-test 실패 (provider key invalid / network error): key-entry 로 retry. 3 retry 후에는 사용자에게 manual 검증 안내.

---

## 8. 외부 레퍼런스 — 패턴 차용 (코드 미차용)

### 8.1 Open WebUI

- 자체 호스팅 LLM UI, Ollama/OpenAI 호환.
- 패턴 차용: install wizard 의 provider 선택 UX (단일 화면 카드 선택).
- 차용하지 않음: Python backend, multi-user auth, plugin marketplace.

### 8.2 Ollama

- 로컬 LLM runtime + 단순 web UI.
- 패턴 차용: localhost 전용 + 단일 사용자 가정의 단순함.
- 차용하지 않음: Go 의 `cmd/ollama` 코드 — 본 SPEC 은 자체 frontend.

### 8.3 Claude Code (참고)

- VSCode/터미널 통합 AI agent.
- 패턴 차용: AskUserQuestion 의 3-way decision (AlwaysAllow / OnceOnly / Deny) UI — modal 디자인 영감.
- 차용하지 않음: Claude Code 의 코드 자체 (직접 포팅 없음).

### 8.4 Hermes Agent v0.10

- ARCH-REDESIGN 의 영감원.
- 차용: messenger gateway 패턴, persona/voice 분리.
- 차용 안 함: Web UI 가 없음 (Hermes 는 messenger 우선).

### 8.5 본 SPEC 이 채택하지 않은 외부 패턴

- ChatGPT WebUI: multi-user / 외부 노출 / OAuth — 모두 본 SPEC 외.
- LangChain LangServe Playground: dev tool 위주, 비개발자 UX 미흡.
- Streamlit: Python-only, embed.FS 모델 부적합.

---

## 9. 알려진 미지수 / open question (annotation cycle 후보)

> 이 섹션은 spec.md §11 의 OI-* 와 별개로, **annotation cycle** 에서 사용자에게 묻고자 하는 의사결정 항목을 모은다.

### 9.1 `/bridge/login` 정확한 schema (spec.md OI-A 추적)

BRIDGE-001 v0.2.0 spec 은 `/bridge/login` POST 로 24h 쿠키를 발급한다고 명시했으나 payload schema 는 미정. 본 SPEC 은 install wizard 완료 시점에 daemon 이 자동으로 first session 쿠키를 발급한다고 가정. 이 가정이 BRIDGE-001 implementation 결정과 충돌할 가능성이 있다.

→ **annotation question**: install 완료 시점의 자동 cookie 발급은 BRIDGE-001 의 separate `/bridge/login` 호출로 모델링할 것인가, 또는 install wizard 의 `done` 상태 전환에 daemon 이 직접 cookie 를 set 하는 비대칭 경로로 모델링할 것인가?

### 9.2 `OutboundPermissionRequest` payload schema (spec.md OI-B 추적)

BRIDGE-001 spec 의 `OutboundMessage.Payload []byte` 가 정확히 어떤 JSON 으로 직렬화되는지 미명세. 본 SPEC REQ-WEBUI-208 은 `{request_id, subject_id, subject_type, capability, scope, reason, requested_at}` 가정.

→ **annotation question**: 이 schema 를 본 SPEC 에서 normative 로 명시할 것인가, 또는 BRIDGE-001 amendment 후보로 외부화할 것인가?

### 9.3 같은 포트 vs 별도 포트 (spec.md OI-07 추적)

본 SPEC 은 `cfg.WebUI.BindPort = 8787` 별도 포트를 default 로 했으나 BRIDGE-001 과 같은 포트 (path 분리) 도 옵션. 후자가 단순하지만 BRIDGE-001 의 listener 구조와 ServeMux mount point 가 무엇인지 BRIDGE-001 implementation 단계에서 결정.

→ **annotation question**: WEBUI listener 와 BRIDGE listener 를 같은 포트에 마운트할 것인가, 별도 포트로 둘 것인가? (단순성 vs CORS 회피 trade-off)

### 9.4 channel-aware Confirmer 라우팅

PERMISSION-001 v0.2.0 의 Confirmer 인터페이스는 단일. 멀티 채널 동시 운용 시 어느 채널이 prompt 를 받을지에 대한 결정 로직은 PERMISSION-001 + HOOK-001 후속 amendment 에서 정해야 한다.

→ **annotation question**: 본 SPEC 은 "Web UI 가 active session 일 때만 webui Confirmer 사용" 가정인데, "마지막 active session" 정의 + tracking 메커니즘이 어디에 위치해야 하는가? (HOOK-001? PERMISSION-001? 본 SPEC?)

### 9.5 frontend monorepo vs 독립 모듈 (spec.md OI-08 추적)

`frontend/` 디렉토리를 npm workspace 로 두고 root `package.json` 추가할지, `internal/webui/static/dist/` 산출물만 git track 하고 frontend 는 별도 repo 로 분리할지.

→ **annotation question**: 본 SPEC 은 frontend monorepo (root level `frontend/`) 가정. 운영 부담 vs 통합도 tradeoff 검토 필요.

### 9.6 install wizard provider 목록 baseline

- v0.2.0 baseline: Anthropic / OpenAI / Google / Ollama (4종).
- 후보 추가: xAI Grok, DeepSeek, Mistral, Together, Groq.

→ **annotation question**: v0.2.0 baseline 4종 충분한가? Ollama 의 endpoint URL 입력 모드는 키 입력과 다른 폼 분기인데 같이 묶어도 되는가?

### 9.7 dark mode WCAG AA 자동 검증 도구

axe-core CLI vs Playwright + axe vs Lighthouse CI. 자동화 비용 vs 신뢰도.

→ **annotation question**: 자동화 검증을 e2e 테스트에 포함할 것인가, 또는 manual smoke 로 충분한가? (REQ-WEBUI-304 + AC-WEBUI-13 단일 케이스만 자동 검증?)

### 9.8 i18n 키 누락 검증 시점

- runtime: 페이지 렌더링 시 `t(missing.key)` 호출이 fallback 영문 + console.warn.
- build-time: `npm run build` 가 ko/en 양쪽 키 set diff 검증, 누락 시 fail.

→ **annotation question**: build-time 검증을 강제할 것인가? (CI gate)

### 9.9 incremental markdown parser 선택 (spec.md OI-02 추적)

`streaming-markdown` (외부 lib) vs 자체 구현 (chunk 누적 + `marked` parse + DOMPurify). 자체 구현이 안전하나 코드 분량이 늘어남. 외부 lib 의 maintainership 검토 필요.

→ **annotation question**: prototype 단계 (run phase Phase 3) 에서 외부 lib 를 시도하고 결과를 보고 결정할 것인가?

---

## 10. 검증 — plan 종료 체크리스트

### 10.1 spec.md 형식 검증 (자동화)

- [x] HISTORY 0.2.0 entry 추가
- [x] frontmatter version 0.1.0 → 0.2.0
- [x] frontmatter status 유지 (`planned`, completed 아님)
- [x] frontmatter updated_at 2026-05-04
- [x] frontmatter labels 채움 (phase-6 / milestone-m6 / area/webui / area/frontend / type/feature / priority/p0-critical)
- [x] EARS REQ-WEBUI-NNN ≥ 18 — 본 SPEC 은 21 (101..106 + 201..208 + 301..304 + 401..403 + 501..506)
- [x] AC-WEBUI-NN ≥ 12 — 본 SPEC 은 14
- [x] Exclusions 섹션 존재 (§10)
- [x] Dependencies 섹션 존재 (§4)
- [x] Risks 섹션 존재 (§9)
- [x] References 섹션 존재 (§12)
- [x] §6 Technical Approach: package layout + wire integration + SSE handler + install wizard state machine + settings/audit/brand 모두 포함
- [x] AI.GOOSE 표기 (산문) + `goose` (백틱 식별자) — brand-lint 통과 예상

### 10.2 brand-lint 검증

- spec.md / research.md / tasks.md 본문에서 brand-lint 가 검출하는 부적합 패턴 (`AI.GOOSE` 가 아닌 brand 위치 표기, 백틱 외부의 도메인-용어형 표기 등 — 정확한 패턴 정의는 `.moai/project/brand/style-guide.md` §5 참조) 매치 0건을 자체 검토 완료. 기계 검증은 `bash scripts/check-brand.sh .moai/specs/SPEC-GOOSE-WEBUI-001/spec.md .moai/specs/SPEC-GOOSE-WEBUI-001/research.md .moai/specs/SPEC-GOOSE-WEBUI-001/tasks.md` 로 수행.

### 10.3 plan 종료 후 후속 동작

- annotation cycle: 본 research §9 의 9개 open question 을 사용자에게 묻는다 (1회차에 대표 4개, 후속 회차에 나머지).
- run phase 진입 조건: 사용자가 "Proceed" 명시.
- run phase 시작 시 `.moai/specs/SPEC-GOOSE-WEBUI-001/status.txt` 는 그대로 `planned` 유지 (run 시작 시 manager-ddd/tdd 가 in-progress 로 전환).

---

## 11. 참고 — 본 research.md 가 직접 읽은 파일 목록

- `.moai/specs/SPEC-GOOSE-WEBUI-001/spec.md` (v0.1.0 scaffold)
- `.moai/specs/SPEC-GOOSE-PERMISSION-001/spec.md` (v0.2.0)
- `.moai/specs/SPEC-GOOSE-BRIDGE-001/spec.md` (v0.2.0)
- `.moai/design/goose-runtime-architecture-v0.2.md` (§0~§13)
- `.moai/project/brand/brand-voice.md` (TBD 상태)
- `.moai/project/brand/style-guide.md` (FROZEN v1.0.0)
- `.moai/project/brand/visual-identity.md` (TBD 상태)
- `.moai/project/brand/design-tokens.json` (v1.0.0)
- `cmd/goosed/main.go` (13-step wire 생애주기)
- `cmd/goosed/wire.go` (wire 헬퍼 패턴)
- `scripts/check-brand.sh` (brand-lint 검증 스크립트 첫 80줄)
- `.claude/rules/moai/design/constitution.md` (FROZEN/EVOLVABLE zone)

읽지 않은 (그러나 본 SPEC 결정에 잠재적 영향 있는) 파일:

- BRIDGE-001 의 실 구현 진행 상태 (현재 status=planned)
- HOOK-001 spec 본문 (status=completed 만 확인)
- AUDIT-001 spec 본문 (status=completed 만 확인)
- TRANSPORT-001 spec 본문 (gRPC vs Connect 결정)

이들 미독파일은 spec.md §11 의 OI-* 또는 본 research §9 open question 의 후보로만 추적.

---

**End of research.md**
