# SPEC-GOOSE-WEBUI-001 — Tasks (TDD-mode)

> 작성일: 2026-05-04
> 작성자: manager-spec
> 상태: planned (run phase 진입 시 manager-tdd 가 in-progress 로 전환)
>
> Methodology: **TDD** (RED → GREEN → REFACTOR), per `quality.development_mode`.
> Harness: **standard** (sprint contract optional but recommended; LSP zero-error gate per phase).
> Drift guard: applies to both Go (`internal/webui/`, `cmd/goose/`) and frontend (`frontend/`).

---

## 1. AC → Task Mapping

| AC               | Primary REQ                       | Owner Phase | Task IDs                                       |
| ---------------- | --------------------------------- | ----------- | ---------------------------------------------- |
| AC-WEBUI-01      | REQ-WEBUI-104, REQ-WEBUI-201      | M1          | T-101, T-102, T-103, T-104                     |
| AC-WEBUI-02      | REQ-WEBUI-501, REQ-WEBUI-502      | M1          | T-105, T-106                                   |
| AC-WEBUI-03      | REQ-WEBUI-202, REQ-WEBUI-301      | M2          | T-201, T-202, T-203, T-204                     |
| AC-WEBUI-04      | REQ-WEBUI-204, REQ-WEBUI-503      | M2          | T-205, T-206, T-207                            |
| AC-WEBUI-05      | REQ-WEBUI-203                     | M3          | T-301, T-302, T-303                            |
| AC-WEBUI-06      | REQ-WEBUI-207, REQ-WEBUI-505      | M3          | T-304, T-305                                   |
| AC-WEBUI-07      | REQ-WEBUI-205, REQ-WEBUI-506      | M4          | T-401, T-402, T-403                            |
| AC-WEBUI-08      | REQ-WEBUI-206                     | M4          | T-404, T-405                                   |
| AC-WEBUI-09      | REQ-WEBUI-208, REQ-WEBUI-303      | M5          | T-501, T-502, T-503, T-504                     |
| AC-WEBUI-10      | REQ-WEBUI-102, REQ-WEBUI-503, REQ-WEBUI-504 | cross | T-110, T-505 (security gate, run across all phases) |
| AC-WEBUI-11      | REQ-WEBUI-106                     | M1          | T-107                                          |
| AC-WEBUI-12      | REQ-WEBUI-105                     | M1          | T-108                                          |
| AC-WEBUI-13      | REQ-WEBUI-304                     | M5          | T-506                                          |
| AC-WEBUI-14      | REQ-WEBUI-302                     | M1          | T-109                                          |

REQ-WEBUI-401 (--port flag), REQ-WEBUI-402 (model picker), REQ-WEBUI-403 (audit highlight) 는 자동 AC 가 없으므로 Phase 6 의 manual smoke 체크리스트로 검증한다.

---

## 2. Milestones

### M0 — Foundation & Repo Wiring (precondition)

> Prerequisite. 모든 후속 milestone 진입 전 1회만.

| Task   | 설명 |
| ------ | --- |
| T-001  | (확인만) BRIDGE-001 implementation 진행 상태 점검. 본 SPEC 의 SSE consumer가 의존하는 `/bridge/stream` `/bridge/inbound` `/bridge/login` 가 stub 으로라도 응답하도록 합의. 실 구현 미완료 시 `internal/bridge/` 의 stub harness 작성을 BRIDGE-001 에 요청. |
| T-002  | `frontend/` 디렉토리 셋업: `npm init -y`, Vite + React + TS template, Tailwind v4 설치, shadcn/ui v4 init, `vite.config.ts` 의 `build.outDir` 를 `../internal/webui/static/dist` 로 지정. |
| T-003  | `internal/webui/static/embed.go` 작성, `//go:embed dist/*` + placeholder `dist/.gitkeep` (frontend 미빌드 환경에서도 Go 컴파일 통과). |
| T-004  | `internal/webui/server.go` 의 `webui.Server` skeleton + `cmd/goosed/wire.go` 에 `wireWebUISubsystem` 추가, main.go step 10.9 삽입. listener 는 stub (실 listen 은 M1 에서). |
| T-005  | `cfg.WebUI` config 섹션 신설 (`.moai/config/sections/webui.yaml` 또는 `internal/config/` 하위), `BindHost`, `BindPort` (default 8787), `Enabled`. CONFIG-001 reload path 와 호환. |

### M1 — Server Skeleton + Embed + `goose web` Subcommand

> AC-WEBUI-01, AC-WEBUI-02, AC-WEBUI-11, AC-WEBUI-12, AC-WEBUI-14 그린.

| Task   | TDD Order | 설명 |
| ------ | --------- | --- |
| T-101  | RED #1    | `internal/webui/server_test.go` 에 `TestServer_BindLoopbackOnly` 작성 — `0.0.0.0` bind 시도가 `webui.non_loopback_bind` 에러로 실패하는 케이스. (REQ-WEBUI-501) |
| T-102  | RED #2    | 동일 파일에 `TestServer_HostHeaderRebindingRejected` — bind 는 loopback 이지만 `Host: evil.com` 헤더로 요청 시 421 응답. (REQ-WEBUI-502) |
| T-103  | RED #3    | `TestServer_StaticEmbed_ServesIndexHTML` — embed.FS 의 `dist/index.html` 이 `GET /` 에서 HTML 응답. CSP 헤더 포함. (REQ-WEBUI-101, REQ-WEBUI-102) |
| T-104  | RED #4    | `cmd/goose/web_test.go` 에 `TestGooseWeb_LaunchSpawnsDaemon_OpensBrowser` — `--no-browser` 플래그 시 stdout에 URL 출력 검증, daemon spawn 후 healthz 200 검증. (REQ-WEBUI-104, REQ-WEBUI-201) |
| T-105  | GREEN     | `internal/webui/server.go` 구현: `http.Server` + ServeMux + bind 검증 + Host 검증 미들웨어 + embed.FS handler + CSP 미들웨어. |
| T-106  | GREEN     | `cmd/goose/web.go` 구현: cobra 서브커맨드, daemon spawn (없을 때), 브라우저 open (`open` / `xdg-open` / `start`), `--no-browser` flag. |
| T-107  | RED #5 → GREEN | `frontend/scripts/check-bundle-size.mjs` — `npm run build` 후 `dist/assets/*.js` gzip 누적 ≤ 500 KB 검증. 초과 시 non-zero exit. `package.json` `scripts.postbuild` 에 hook. (REQ-WEBUI-106) |
| T-108  | RED #6 → GREEN | `frontend/i18n/locales/{ko,en}.json` 셋업 + `react-i18next` 통합 + `frontend/scripts/check-i18n-keys.mjs` (양쪽 set diff 검증). 초기 키: `app.title`, `chat.placeholder`, `install.welcome.title`, `nav.chat`, `nav.settings`, `nav.audit`, `theme.toggle`. brand notation 검증 (i18n 파일을 `scripts/check-brand.sh` 와 동일 알고리즘으로 grep, 부적합 brand 패턴 0건). (REQ-WEBUI-105) |
| T-109  | RED #7 → GREEN | `internal/webui/server_test.go` 에 `TestServer_DaemonStateAware_503OnDraining` — daemon state 가 draining/bootstrap 일 때 503 + Retry-After 헤더 응답 검증. (REQ-WEBUI-302) |
| T-110  | (cross-cut) RED → GREEN | CSP regression test: 모든 HTML/JSON 응답에 `Content-Security-Policy` 헤더 포함. `unsafe-eval` 미포함. (REQ-WEBUI-102, AC-WEBUI-10 부분) |
| T-111  | REFACTOR  | M1 코드 리뷰: server.go 의 미들웨어 chain 추출, errors.go 정리, embed handler 의 SPA fallback (`/install` 등 SPA 라우트가 index.html 로 resolve) 별도 함수로 분리. |
| T-112  | LSP gate  | `go vet ./internal/webui/... ./cmd/goose/...` zero issue, `golangci-lint run` zero issue, `npm run lint` zero issue. M2 진입 전 확인. |

### M2 — Install Wizard (state machine + provider key entry)

> AC-WEBUI-03, AC-WEBUI-04 그린. CREDENTIAL-PROXY-001 stub 사용 (실 구현 대기 시).

| Task   | TDD Order | 설명 |
| ------ | --------- | --- |
| T-201  | RED #8    | `internal/webui/installer/state_machine_test.go` — 7-state 전체 transition table 테이블 테스트. Valid/Invalid event 커버. (REQ-WEBUI-301) |
| T-202  | RED #9    | `internal/webui/handlers/install_test.go` — `GET /webui/install/state` 가 `~/.goose/state/install.json` 의 현재 state 반환. install 미완료 시 `/` 와 `/settings` `GET` 이 `/install` 로 redirect. (REQ-WEBUI-202, REQ-WEBUI-301) |
| T-203  | GREEN     | `internal/webui/installer/state.go` 구현: atomic write + mode 0600. `installer.state_machine.go` 구현. |
| T-204  | GREEN     | `frontend/src/pages/Install.tsx` + 7-step wizard 컴포넌트 (intro / provider-select / key-entry / keyring-write 진행 표시 / daemon-reload 진행 표시 / smoke-test / done). brand 색상 + Inter typography. `react-i18next` 사용. |
| T-205  | RED #10   | `internal/webui/handlers/install_test.go` — `POST /webui/install/credentials {provider, key}` 시나리오: (a) raw key 가 응답 body 미포함, (b) keyring stub 호출, (c) zap log 에 raw key 매치 0건, (d) audit event 에 raw key 미포함. fixture key `sk-ant-fixture-test-001`. (REQ-WEBUI-204, REQ-WEBUI-503) |
| T-206  | GREEN     | `internal/webui/handlers/install.go` 의 credentials handler 구현. `CredentialProxy` 인터페이스 (CREDENTIAL-PROXY-001 stub) 호출. response schema `{key_id, last4}`. zap.String("key_id", id) 만 로깅, key 자체는 절대 zap field 에 안 넣음. |
| T-207  | RED #11 → GREEN | smoke-test step 의 첫 LLM ping → PERMISSION-001 첫 grant 발생 시나리오. webui Confirmer 어댑터 stub 으로 `AlwaysAllow` 자동 응답. install done 까지 도달 검증. |
| T-208  | REFACTOR  | M2 코드 리뷰: state machine 의 transition 함수들을 단일 진입점으로 통합, install handler 의 redaction 로직을 미들웨어로 추출. |
| T-209  | LSP gate  | M2 종료 검증. coverage `internal/webui/installer/...` ≥ 85%. |

### M3 — Chat (SSE consume + reconnect)

> AC-WEBUI-05, AC-WEBUI-06 그린. BRIDGE-001 stub harness 사용.

| Task   | TDD Order | 설명 |
| ------ | --------- | --- |
| T-301  | RED #12   | `frontend/src/hooks/useSSE.test.ts` (Vitest) — EventSource 기반 hook 의 chunk dispatching, lastEventId 추적, reconnect 백오프. (REQ-WEBUI-203, REQ-WEBUI-207) |
| T-302  | GREEN     | `frontend/src/hooks/useSSE.ts` 구현: EventSource 래퍼 + Last-Event-ID + 4 event type 분기 (chunk/status/permission_request/error). 백오프는 BRIDGE-001 §6.2 준수. |
| T-303  | GREEN     | `frontend/src/components/chat/MessageStream.tsx` + `MarkdownStream.tsx` — chunk incremental render. e2e Playwright 테스트로 first-token-to-paint p95 측정. (REQ-WEBUI-203 + AC-WEBUI-05) |
| T-304  | RED #13   | Playwright e2e `tests/e2e/chat-resume.spec.ts` — 청크 5개 후 네트워크 단절 → 1.2s 후 reconnect → 청크 #6 부터 정상 도착. (REQ-WEBUI-207, AC-WEBUI-06) |
| T-305  | RED #14 → GREEN | `useSSE` 의 reconnect storm 방어: 11번째 시도 시 stop + reload banner 표시. (REQ-WEBUI-505) |
| T-306  | REFACTOR  | M3 코드 리뷰: incremental markdown parser 결정 (자체 vs 외부 lib, OI-02 종결). DOMPurify 적용 검증. |
| T-307  | LSP gate  | M3 종료 검증. frontend coverage Vitest ≥ 80%, Playwright e2e 1+ scenario 통과. |

### M4 — Settings + Audit Viewer

> AC-WEBUI-07, AC-WEBUI-08 그린.

| Task   | TDD Order | 설명 |
| ------ | --------- | --- |
| T-401  | RED #15   | `internal/webui/handlers/settings_test.go` — `PUT /webui/settings/providers.yaml` 정상 케이스 + 잘못된 YAML 케이스 + mtime conflict 케이스 (409). (REQ-WEBUI-205, REQ-WEBUI-506) |
| T-402  | GREEN     | `internal/webui/handlers/settings.go` 구현: YAML validator (CONFIG-001 schema 재사용), atomic write mode 0600, daemon reload trigger (CONFIG-001 reload hook 호출). |
| T-403  | GREEN     | `frontend/src/pages/Settings.tsx` — `providers.yaml`, `security.yaml`, `channels.yaml`, `aliases.yaml` 4개 탭. 각 탭은 textarea + validate 버튼 + save 버튼. error 표시. shadcn `Tabs`, `Form`, `Toast` 사용. |
| T-404  | RED #16   | `internal/webui/auditviewer/reader_test.go` — fixture audit.log (200 event) 에 대해 페이지네이션 + filter (capability) + 응답 100ms p95 검증. (REQ-WEBUI-206, AC-WEBUI-08) |
| T-405  | GREEN     | `internal/webui/auditviewer/reader.go` 구현 + `handlers/audit.go` 의 `GET /webui/audit?cursor=&limit=&filter[]` 핸들러. `frontend/src/pages/Audit.tsx` 의 페이지네이션 UI. shadcn `ScrollArea`, `Skeleton`. |
| T-406  | REFACTOR  | M4 코드 리뷰: settings YAML schema 검증 로직을 `settingsedit/validator.go` 로 격리, audit reader 의 reverse-iteration 알고리즘 (대용량 파일 처리) 검토. |
| T-407  | LSP gate  | M4 종료 검증. coverage `internal/webui/auditviewer/...` ≥ 85%. |

### M5 — Approval Modal + Dark Mode + Brand Polish

> AC-WEBUI-09, AC-WEBUI-13 그린. AC-WEBUI-10 (security gate) 최종 검증.

| Task   | TDD Order | 설명 |
| ------ | --------- | --- |
| T-501  | RED #17   | `internal/webui/confirmer/webui_confirmer_test.go` — webui Confirmer 의 SSE permission_request 발화 → `POST /webui/approve/{id}` 응답 → Decision 반환. timeout 60s 시 default-deny. (REQ-WEBUI-208, AC-WEBUI-09) |
| T-502  | GREEN     | `internal/webui/confirmer/webui_confirmer.go` 구현: PERMISSION-001 `Confirmer` 인터페이스 충족, in-flight request map (`request_id` → channel), 60s timeout. |
| T-503  | GREEN     | `frontend/src/components/ApproveModal.tsx` + `useApproval` hook. shadcn `Dialog` (non-dismissible), 4 action button, 60s countdown 표시. modal 열림 동안 chat input disabled (REQ-WEBUI-303). |
| T-504  | RED #18 → GREEN | end-to-end Playwright `tests/e2e/approval-roundtrip.spec.ts` — chat 1턴 + tool 호출 (mock) → permission_request 발화 → AlwaysAllow 클릭 → grant 영속화 → 응답 chunk 정상 도착. timeout 별도 케이스. |
| T-505  | RED #19   | `internal/webui/server_test.go` 에 `TestServer_NoExecOrEvalEndpoints` — `GET /webui/exec`, `GET /webui/eval`, `POST /webui/exec` 모두 404. fixture provider key 가 `/webui/audit`, `/webui/settings/*`, SSE event 응답에 누출 0건 (회귀 테스트). (REQ-WEBUI-503, REQ-WEBUI-504, AC-WEBUI-10) |
| T-506  | RED #20 → GREEN | `frontend/src/hooks/useDarkMode.test.ts` + Playwright e2e `tests/e2e/dark-mode.spec.ts` — `prefers-color-scheme: dark` 첫 방문 dark, toggle 후 light, 새로고침 유지. WCAG AA contrast 검증 (axe-core `contrast-ratio` rule). (REQ-WEBUI-304, AC-WEBUI-13) |
| T-507  | REFACTOR  | M5 코드 리뷰: Confirmer in-flight map 의 메모리 누수 방어 (timeout 후 cleanup), ApproveModal 의 a11y (focus trap, ESC 차단). brand notation 일괄 검증. |
| T-508  | LSP gate + final | M5 종료. 전체 `go test -race ./internal/webui/... ./cmd/goose/...` 통과 + frontend `npm run build && npm test` 통과 + Playwright e2e 모든 시나리오 통과. coverage report 작성. |

---

## 3. TDD 진입 순서 (RED → GREEN → REFACTOR)

순서는 milestone 우선이며, 한 milestone 안에서 RED #N 의 번호 순서. 다음 RED 가 이전 GREEN 에 의존할 때만 명시적 dependency.

```
RED #1  T-101 (loopback bind)              → GREEN T-105
RED #2  T-102 (Host rebind)                → GREEN T-105
RED #3  T-103 (embed serve)                → GREEN T-105
RED #4  T-104 (goose web subcommand)       → GREEN T-106
RED #5  T-107 (bundle size)                → GREEN (build script)
RED #6  T-108 (i18n)                       → GREEN (i18n setup)
RED #7  T-109 (daemon state 503)           → GREEN T-105 확장
                                            → REFACTOR T-111 → LSP T-112
RED #8  T-201 (state machine)              → GREEN T-203
RED #9  T-202 (install state API)          → GREEN T-203
RED #10 T-205 (key redaction)              → GREEN T-206
RED #11 T-207 (smoke-test grant flow)      → GREEN T-207
                                            → REFACTOR T-208 → LSP T-209
RED #12 T-301 (useSSE hook)                → GREEN T-302
RED #13 T-304 (chat resume e2e)            → GREEN T-302/T-303
RED #14 T-305 (reconnect storm)            → GREEN T-302
                                            → REFACTOR T-306 → LSP T-307
RED #15 T-401 (settings PUT)               → GREEN T-402
RED #16 T-404 (audit pagination)           → GREEN T-405
                                            → REFACTOR T-406 → LSP T-407
RED #17 T-501 (Confirmer adapter)          → GREEN T-502
RED #18 T-504 (approval e2e)               → GREEN T-502/T-503
RED #19 T-505 (no exec/eval, key non-leak) → GREEN (regression)
RED #20 T-506 (dark mode)                  → GREEN T-506
                                            → REFACTOR T-507 → LSP T-508
```

---

## 4. Cross-cutting Tasks

| Task   | 설명 |
| ------ | --- |
| T-CC-1 | brand-lint: 모든 milestone 종료 시 `bash scripts/check-brand.sh .moai/specs/SPEC-GOOSE-WEBUI-001/spec.md` 통과 + 추가로 `frontend/src/**/*.tsx`, `frontend/i18n/locales/*.json` 에 동일 grep 적용 (custom script). |
| T-CC-2 | @MX 태그: `internal/webui/server.go` 의 listener startup 진입점에 `@MX:ANCHOR` (fan_in: 본 SPEC 의 5개 handler group 모두 통과), `internal/webui/confirmer/webui_confirmer.go` 의 in-flight map mutate 위치에 `@MX:WARN`(REASON: race-prone 동시성). |
| T-CC-3 | Manual smoke checklist (REQ-WEBUI-401/402/403): `--port 9000` 으로 띄워서 8787 대신 9000 응답 확인, 다중 provider 시 model picker 동작, audit `subject_id=skill:summary` highlight. M5 종료 후 1회. |
| T-CC-4 | OI 종결: research.md §9 의 9개 open question 을 milestone 별 적절한 시점에 결정. OI-A/B/C 는 M1 ~ M3 내에서 BRIDGE-001 implementation 협의로, OI-D 는 M5 내에서 결정. |
| T-CC-5 | 문서 sync: `/moai sync` 진입 시 `~/.goose/` 사용자 가이드 (별도 SPEC 후보) 또는 README.md 의 `goose web` 섹션 추가 보고. 본 task 는 sync phase 에서 수행. |

---

## 5. Definition of Done (전체 SPEC)

- [ ] 모든 14 AC 자동 검증 통과 (Go test + Vitest + Playwright)
- [ ] coverage `internal/webui/...` ≥ 85%, `cmd/goose/web.go` ≥ 85%, frontend Vitest ≥ 80%
- [ ] `go test -race ./internal/webui/... ./cmd/goose/...` race condition 0건
- [ ] `golangci-lint run --enable gosec` 0 issue
- [ ] `npm run build` 통과 + bundle size ≤ 500 KB gzip
- [ ] `npm run lint` 0 error
- [ ] Playwright e2e 5+ scenario 통과 (install / chat / chat-resume / approval / dark-mode)
- [ ] `bash scripts/check-brand.sh .moai/specs/SPEC-GOOSE-WEBUI-001/{spec,research,tasks}.md` exit 0
- [ ] frontend i18n 키 누락 0건 (build-time 검증)
- [ ] PERMISSION-001 / HOOK-001 / AUDIT-001 / CONFIG-001 의 인터페이스 컨트랙트 위반 0건
- [ ] BRIDGE-001 의 SSE consumer 컨트랙트 호환 (close code, Last-Event-ID, CSRF, Cookie) 검증 통과
- [ ] research.md §9 의 9개 OI 모두 종결 (결정 또는 후속 SPEC 으로 위임)
- [ ] manual smoke checklist (T-CC-3) 통과
- [ ] CHANGELOG entry 추가 (sync phase) — 본 task 는 sync 에서

---

## 6. Risks per Milestone

| Milestone | 주요 리스크 | 완화 |
| --------- | --------- | ---- |
| M0 / M1   | embed.FS 가 frontend 미빌드 환경에서 컴파일 실패 | T-003 의 `dist/.gitkeep` placeholder + CI 에서 `npm run build` 선행 |
| M1        | `cfg.WebUI` 추가가 CONFIG-001 reload 와 충돌 | T-005 에서 reload hook 통합 검증 |
| M2        | CREDENTIAL-PROXY-001 가 미완료 상태로 stub 만 존재 | webui 의 `CredentialProxy` 인터페이스 정의 + stub 구현 → CREDENTIAL-PROXY-001 완료 시 stub 만 교체 |
| M3        | BRIDGE-001 의 `/bridge/stream` `/bridge/inbound` `/bridge/login` 가 미구현 | T-001 에서 stub harness 합의. M3 시작 시점에 BRIDGE-001 status 재확인 |
| M3        | first-token-to-paint p95 ≤ 100 ms 미달 | incremental markdown parser 선택 (T-306 OI-02) 에 영향 |
| M4        | settings YAML schema 가 daemon 과 disagree (concurrent edit 401/409) | T-402 + T-401 의 mtime+hash 비교 |
| M5        | Confirmer in-flight map 의 메모리 누수 (request 가 timeout 없이 영원히 대기) | T-507 에서 cleanup goroutine 검증 |
| M5        | dark mode WCAG AA contrast 미달 (특히 primary `#FFB800` 위 텍스트) | T-506 의 axe-core 검증 + design tokens 의 contrast 보장 색상 사용 |
| cross     | brand-lint 위반 (i18n json) | T-CC-1 의 custom script 적용 |
| cross     | bundle size 폭증 | T-107 size budget gate + lazy-load `/audit` |

---

## 7. Run Phase Entry Conditions

run phase 진입 시 manager-tdd 가 다음을 1회 검증:

1. `.moai/specs/SPEC-GOOSE-WEBUI-001/{spec,research,tasks}.md` 모두 존재
2. spec.md frontmatter `version: 0.2.0`, `status: planned`
3. annotation cycle 종결 마커 (사용자의 "Proceed" 명시)
4. dependency SPEC 상태 확인:
   - PERMISSION-001 / AUDIT-001 / HOOK-001 / CONFIG-001 / DAEMON-WIRE-001 / CORE-001: completed
   - BRIDGE-001 / CREDENTIAL-PROXY-001: planned (stub harness 또는 실 구현 결정)
5. `feature/SPEC-GOOSE-WEBUI-001-run` 브랜치 분기 (CLAUDE.local.md §1.2)

run phase 종료 시 (`/moai sync` 호출 직전):

- `.moai/specs/SPEC-GOOSE-WEBUI-001/status.txt` 가 `completed` 로 갱신
- frontmatter `status: completed`, `completed: <date>`
- HISTORY 0.3.0 entry 추가 ("v0.2.0 plan 의 모든 AC 그린 처리, run phase 종결")

---

**End of tasks.md**
