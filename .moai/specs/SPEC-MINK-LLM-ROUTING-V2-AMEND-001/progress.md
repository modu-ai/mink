# Progress — SPEC-MINK-LLM-ROUTING-V2-AMEND-001

진행률 추적. 마일스톤별 status + AC 진행 + 누적 학습.

---

## Summary

- **Status**: planned (plan 종결, run 미개시)
- **Version**: 0.2.0
- **Overall Progress**: 0% (0/41 AC GREEN, audit B1 fix 반영)
- **Last Updated**: 2026-05-16

---

## Milestone Status

| Milestone | Status | AC 범위 | GREEN / 총 | Coverage |
|---|---|---|---|---|
| M1 — Provider 어댑터 5종 | ⏸️ planned | AC-001 ~ AC-006, AC-001a, AC-005a | 0 / 8 | — |
| M2 — 인증 흐름 (key paste + OAuth) | ⏸️ planned | AC-007 ~ AC-016, AC-016a | 0 / 11 | — |
| M3 — 라우팅 정책 + Fallback chain | ⏸️ planned | AC-017 ~ AC-026 | 0 / 10 | — |
| M4 — MEMORY-QMD Export Hook | ⏸️ planned | AC-027 ~ AC-030 | 0 / 4 | — |
| M5 — CLI/TUI 통합 + E2E | ⏸️ planned | AC-031 ~ AC-038 | 0 / 8 | — |

Legend: ⏸️ planned · 🔵 in-progress · 🟢 GREEN · 🔴 BLOCKED

---

## Tasks Status (26 atomic)

| Task | Milestone | Status | PR | Notes |
|------|----|---|---|---|
| T-001 Provider types | M1 | ⏸️ | — | — |
| T-002 Anthropic adapter | M1 | ⏸️ | — | — |
| T-003 DeepSeek adapter | M1 | ⏸️ | — | — |
| T-004 OpenAI adapter | M1 | ⏸️ | — | — |
| T-005 Codex adapter (client) | M1 | ⏸️ | — | — |
| T-006 z.ai GLM adapter | M1 | ⏸️ | — | — |
| T-007 Custom adapter | M1 | ⏸️ | — | — |
| T-008 ProviderRegistry | M1 | ⏸️ | — | — |
| T-009 AUTH consumer adapter | M2 | ⏸️ | — | A1 freeze 대기 |
| T-010 Key paste flow | M2 | ⏸️ | — | — |
| T-011 OAuth PKCE 생성 | M2 | ⏸️ | — | — |
| T-012 OAuth callback server | M2 | ⏸️ | — | — |
| T-013 Device-code flow | M2 | ⏸️ | — | — |
| T-014 `mink login` 명령 | M2 | ⏸️ | — | — |
| T-015 Web onboarding 통합 | M2 | ⏸️ | — | ONBOARDING-001 v0.3.1 wiring |
| T-016 Codex 7d warning | M2 | ⏸️ | — | — |
| T-017 RoutingCategory + YAML | M3 | ⏸️ | — | — |
| T-018 Chain 정의 | M3 | ⏸️ | — | — |
| T-019 Fallback executor | M3 | ⏸️ | — | — |
| T-020 Rate-limit filter 재활용 | M3 | ⏸️ | — | v0.2.1 코드 재활용 |
| T-021 `mink routing` 명령 | M3 | ⏸️ | — | — |
| T-022 SessionExporter 인터페이스 | M4 | ⏸️ | — | A2 freeze 대기 |
| T-023 Stream tap 비동기 | M4 | ⏸️ | — | — |
| T-024 `--no-export` flag | M4 | ⏸️ | — | — |
| T-025 `mink model` + template | M5 | ⏸️ | — | — |
| T-026 E2E 4 시나리오 | M5 | ⏸️ | — | — |

---

## External Dependencies (Plan 단계 Freeze 필요)

| Dep | SPEC | 본 SPEC 의 가정 | 상태 | Freeze 시점 |
|---|---|---|---|---|
| AUTH-CREDENTIAL-001 인터페이스 | SPEC-MINK-AUTH-CREDENTIAL-001 | A1: Store/Load/Delete/List 4 메서드 stable (research.md §6) | ⏸️ pending | M2 진입 전 |
| MEMORY-QMD-001 SessionExporter | SPEC-MINK-MEMORY-QMD-001 | A2: SessionExporter 인터페이스 + sessions/ collection 수용 (research.md §7) | ⏸️ pending | M4 진입 전 (없으면 NoopExporter 로 임시 wire) |
| ERROR-CLASS-001 14 FailoverReason | SPEC-GOOSE-ERROR-CLASS-001 | v0.2.x completed (사용 가능) | 🟢 ready | — |
| RATELIMIT-001 4-bucket tracker | SPEC-GOOSE-RATELIMIT-001 | completed (사용 가능) | 🟢 ready | — |

---

## 보수적 결정 / 합의

(plan 단계에서 미리 합의된 사항. run 단계 진입 시 본 섹션 갱신.)

| # | 결정 | 사유 |
|---|----|----|
| D1 | 15-direct adapter 코드는 *유지* (백워드 호환), default 활성 pool 만 5 로 전환 | 사용자가 OpenRouter / Together / Groq 등 잔여 12 를 custom 으로 명시 추가 시 즉시 사용 가능 |
| D2 | OAuth state CSRF token 은 16 byte 무작위 (32 byte hex string) | RFC 6749 §10.12 권고 충족 |
| D3 | Codex 7d warning 임계는 7.5일 (mock test 안정성) | 8일 직전 자투리 시간 마진 |
| D4 | Provider regex 검증은 prefix 만 (전체 형식 검증은 server-side 호출 시점) | 사용자 paste 오타 1차 catch 목적, 형식 변경 위험 격리 |
| D5 | `--no-stream` batch 모드는 chunk 누적 후 한 번에 출력 | Provider 어댑터는 항상 stream — CLI 만 batch 변환 |

---

## 위험 모니터링

| Risk | 등급 | 상태 | 다음 점검 |
|---|---|---|---|
| R1 callback port 충돌 | M | 디자인 완료 (auto-port + device-code 폴백) | T-012 / T-013 GREEN 시점 |
| R2 provider rate-limit 비대칭 | M | RATELIMIT-001 4-bucket 재사용 | T-020 GREEN |
| R3 Codex 비공식 API 변동 | H | 격리 (`internal/llm/provider/codex/`) | T-005 / T-026 시나리오 2 |
| R4 Custom SSE 미세 차이 | M | 5 golden fixture | T-007 GREEN |
| R5 MEMORY hook 동기 차단 | M | errgroup + buffered channel | T-023 race-test |
| R6 keyring 미가용 평문 fallback silent | M | AUTH-001 측 협조 + 본 SPEC login flow surface warning | T-014 GREEN |
| R7 8일 idle refresh-token 만료 | M | REQ-RV2A-025 (7d warning) | T-016 GREEN |
| R8 AUTH 인터페이스 drift | H | plan 단계 freeze 합의 | M2 진입 전 |

---

## Drift Guard

- 계획 파일: 9 (`research.md`, `spec.md`, `plan.md`, `tasks.md`, `acceptance.md`, `progress.md`, `internal/llm/provider/types.go`, `internal/llm/auth/store_adapter.go`, `internal/llm/router/v2/policy.go` 등)
- 실 수정 파일: — (plan 단계, 0)
- 누적 drift: 0%

---

## 다음 진입점

본 plan (v0.2.0) 머지 후:

1. **외부 dependency freeze**: AUTH-CREDENTIAL-001 plan 진행 → 인터페이스 freeze
2. **/moai run SPEC-MINK-LLM-ROUTING-V2-AMEND-001**: M1 진입 (Provider 어댑터 5종 병렬 구현)

---

Version: 1.0.0
Last Updated: 2026-05-16
