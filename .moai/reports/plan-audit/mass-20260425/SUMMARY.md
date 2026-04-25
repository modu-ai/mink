# Mass Audit SUMMARY — 2026-04-25

30 SPEC 독립 감사 + 권고 조치 실행 결과 종합.

---

## 1. 감사 개요

- **일시**: 2026-04-25
- **범위**: Tier A (구현 SPEC 6건) + Tier B (P0 Plan-only SPEC 24건) = 총 30 SPEC
- **감사 도구**: `plan-auditor` subagent 30회 실행 (병렬 4 라운드 × 6-7 agent)
- **컨텍스트 격리**: 모든 감사가 M1 준수 — orchestrator reasoning 참조 없이 SPEC 파일만으로 판정

---

## 2. 감사 결과 (iteration 1)

### 2.1 통계

- **30 / 30 SPEC FAIL** (100% FAIL rate)
- **총 386 결함**:
  - Critical: 약 25건
  - Major: 약 170건
  - Minor: 약 191건
- **평균 12.9 결함 / SPEC**

### 2.2 SPEC별 상세 (개별 리포트 링크)

#### Tier A — 구현 SPEC

| SPEC | 결함 | Verdict | 구현 정합률 | 리포트 |
|------|------|---------|-------------|--------|
| AGENCY-ABSORB-001 | 7 | FAIL | 82% | [AGENCY-ABSORB-001-audit.md](AGENCY-ABSORB-001-audit.md) |
| CREDPOOL-001 | 13 | FAIL | 45% | [CREDPOOL-001-audit.md](CREDPOOL-001-audit.md) |
| ROUTER-001 | 9 | FAIL | 100% | [ROUTER-001-audit.md](ROUTER-001-audit.md) |
| CORE-001 | 17 | FAIL | 83% | [CORE-001-audit.md](CORE-001-audit.md) |
| ADAPTER-001 | 14 | FAIL(doc)/PASS(impl) | 80-90% | [ADAPTER-001-audit.md](ADAPTER-001-audit.md) |
| ADAPTER-002 | 11 | FAIL | 81.8% | [ADAPTER-002-audit.md](ADAPTER-002-audit.md) |

#### Tier B Round 1 — Core Runtime Infrastructure

| SPEC | 결함 | Verdict | 리포트 |
|------|------|---------|--------|
| CONTEXT-001 | 9 | FAIL | [CONTEXT-001-audit.md](CONTEXT-001-audit.md) |
| TOOLS-001 | 8 | FAIL (Score 0.58) | [TOOLS-001-audit.md](TOOLS-001-audit.md) |
| HOOK-001 | 13 | FAIL (Score 0.62) | [HOOK-001-audit.md](HOOK-001-audit.md) |
| SKILLS-001 | 19 | FAIL (Score 0.58) | [SKILLS-001-audit.md](SKILLS-001-audit.md) |
| MCP-001 | 13 | FAIL | [MCP-001-audit.md](MCP-001-audit.md) |
| SUBAGENT-001 | 17 | FAIL | [SUBAGENT-001-audit.md](SUBAGENT-001-audit.md) |

#### Tier B Round 2 — Protocol/Infra

| SPEC | 결함 | Verdict | 리포트 |
|------|------|---------|--------|
| BRIDGE-001 | 16 | FAIL (Score 0.28) | [BRIDGE-001-audit.md](BRIDGE-001-audit.md) |
| TRANSPORT-001 | 13 | FAIL | [TRANSPORT-001-audit.md](TRANSPORT-001-audit.md) |
| COMPRESSOR-001 | 19 | FAIL | [COMPRESSOR-001-audit.md](COMPRESSOR-001-audit.md) |
| RATELIMIT-001 | 20 | FAIL | [RATELIMIT-001-audit.md](RATELIMIT-001-audit.md) |
| ERROR-CLASS-001 | 7 | FAIL | [ERROR-CLASS-001-audit.md](ERROR-CLASS-001-audit.md) |
| CONFIG-001 | 20 | FAIL (Score 0.62) | [CONFIG-001-audit.md](CONFIG-001-audit.md) |

#### Tier B Round 3 — Feature SPECs

| SPEC | 결함 | Verdict | 리포트 |
|------|------|---------|--------|
| MEMORY-001 | 12 | FAIL | [MEMORY-001-audit.md](MEMORY-001-audit.md) |
| TRAJECTORY-001 | 12 | FAIL (Score 0.68) | [TRAJECTORY-001-audit.md](TRAJECTORY-001-audit.md) |
| SCHEDULER-001 | 21 | FAIL | [SCHEDULER-001-audit.md](SCHEDULER-001-audit.md) |
| JOURNAL-001 | 11 | FAIL | [JOURNAL-001-audit.md](JOURNAL-001-audit.md) |
| RITUAL-001 | 13 | FAIL (Score 0.48, 최저) | [RITUAL-001-audit.md](RITUAL-001-audit.md) |
| I18N-001 | 15 | FAIL (Score 0.58) | [I18N-001-audit.md](I18N-001-audit.md) |

#### Tier B Round 4 — User-Facing Features

| SPEC | 결함 | Verdict | 리포트 |
|------|------|---------|--------|
| LOCALE-001 | 12 | FAIL (Score 0.68) | [LOCALE-001-audit.md](LOCALE-001-audit.md) |
| ONBOARDING-001 | 11 | FAIL (Score 0.48) | [ONBOARDING-001-audit.md](ONBOARDING-001-audit.md) |
| QMD-001 | 10 | FAIL (Score 0.68) | [QMD-001-audit.md](QMD-001-audit.md) |
| CALENDAR-001 | 12 | FAIL | [CALENDAR-001-audit.md](CALENDAR-001-audit.md) |
| BRIEFING-001 | 12 | FAIL | [BRIEFING-001-audit.md](BRIEFING-001-audit.md) |
| DESKTOP-001 | 17 | FAIL (Score 0.52) | [DESKTOP-001-audit.md](DESKTOP-001-audit.md) |

---

## 3. 근본 원인 분석

### 3.1 Smoking Gun Timeline

```
2026-04-21 18:27:44  commit 29882c1  ROADMAP v2.0 + 30 SPEC 일괄 생성
                                      ↑ plan-auditor 게이트 없이 생성
2026-04-21 18:28:02  commit 16c4d78  plan-auditor Phase 2.3 통합
                                      (18초 뒤 — race condition)
```

30 SPEC이 품질 게이트 도입 **18초 전에** 만들어져 감사를 전혀 거치지 않음.

### 3.2 Schema Conflict

- `manager-spec.md:113` — "YAML frontmatter (**8 fields: id, version, status, created, updated, author, priority, issue_number**)"
- `plan-auditor.md:127` — Required: `created_at`, `labels` (다른 이름)

**두 agent 정의가 상호 모순** → 모든 SPEC이 구조적으로 MP-3 FAIL.

### 3.3 Scope Creep

사용자가 사용한 명령:
- `/moai project ... --team ultrathink` (2026-04-21) — 본래는 product.md/structure.md/tech.md만 생성해야 하는데 30 SPEC + ROADMAP 생성 (scope 이탈)
- 자연어: "UI/UX 시뮬레이션..." (2026-04-22) — `/moai plan` 미사용, 추가 SPEC 생성
- "spec과 프로젝트 문서 분석해서 본격적인 개발 진행" (2026-04-22)

모두 `/moai plan`을 명시적으로 사용하지 않았고, `/moai project`는 SPEC 생성 금지 규칙이 없었음.

### 3.4 책임 귀속

| 원인 | 책임 | 비중 |
|------|------|------|
| `/moai project` scope creep 허용 | 워크플로우 설계 결함 | 40% |
| plan-auditor Phase 2.3 도입 타이밍 race | 타이밍 결함 | 30% |
| manager-spec ↔ plan-auditor schema 불일치 | 내부 정의 모순 | 20% |
| 사용자가 `/moai plan`을 각 SPEC별 호출하지 않음 | 사용법 안내 부족 | 10% |

**사용자 프롬프트 자체에는 잘못이 없음.** 워크플로우가 `/moai project` 범위 경계를 HARD rule로 지키지 못했고, plan-auditor 게이트가 생성 시점에 존재하지 않았음.

---

## 4. 권고 조치 이행 현황

### 4.1 Phase A — 예방 (완료 ✅)

commit `35a3f18` — 4개 파일 수정으로 재발 방지

1. **`.claude/skills/moai/workflows/project.md`**: [HARD] SPEC 생성 금지 규칙 명시
2. **`.claude/agents/moai/manager-spec.md`**: canonical frontmatter schema 9 필드 확정 + 12항목 Verification Checklist
3. **`.claude/skills/moai/workflows/plan.md`**: Phase 2 schema 인용 + enum 명시
4. **`.moai/config/sections/harness.yaml`**: `spec_batch_size >= 3` 감지 규칙 추가

### 4.2 Phase B — Frontmatter 일괄 마이그레이션 (완료 ✅)

commit `8b5150c` — Python 스크립트로 50 SPEC 파일 일괄 수정

- `created` → `created_at` (50 파일)
- `updated` → `updated_at` (50 파일)
- `labels: []` 추가 (모든 파일)
- `status` enum 정규화: "Planned" → "planned" 등
- 30 SPEC 중 MP-3 관련 결함 약 **90건 해소**

### 4.3 Phase C1 — Critical 코드 결함 수정 (완료 ✅)

commit `79d92ff` — expert-backend 3 parallel agents로 6건 수정

| 결함 | 파일 | 테스트 추가 |
|------|------|-------------|
| CREDPOOL D13 (보안) | `internal/llm/credential/pool.go` | 4 tests |
| CORE B4-1 (SIGTERM) | `cmd/goosed/main.go` + `internal/core/runtime.go` | 1 test |
| CORE B4-2 (parentCtx) | `internal/core/shutdown.go` | 1 test |
| ADAPTER I2 (보안) | `internal/llm/provider/google/gemini.go` | 3 subtests |
| ADAPTER I3 (dead code) | `internal/llm/provider/llm_call.go` | 1 test |
| ADAPTER I1 (tracker) | `google/gemini.go` + `ollama/local.go` | 2 tests |
| ADAPTER2 D1 (13→15) | `internal/llm/factory/registry_builder.go` | 3 subtests |

**검증**: `go test -race -count=5 ./internal/core/...` PASS, `go vet ./internal/...` clean, 17 provider 패키지 전체 테스트 통과.

### 4.4 Phase C2 — 29 SPEC 나머지 결함 수정 (대기 📋)

- **범위**: 29 SPEC × 평균 10-15 결함/SPEC (frontmatter 해소 후 잔여)
- **성격**: EARS 라벨, AC Gherkin→EARS, REQ↔AC 매핑, Amendment 일관성
- **예상**: manager-spec 29회 iteration, 2-5 시간
- **상태**: 별도 세션에서 batch 수행 권장 (token volume 고려)

### 4.5 Phase D — 최종 검증 (이 보고서 ✅)

- mass audit 리포트 30건 아카이브 (commit `881ced6`)
- SUMMARY.md (이 파일) 생성
- Phase A/B/C1 커밋 링크

---

## 5. 잔존 리스크

### 5.1 Phase C2 미완

29 SPEC 각각에 대해 EARS/AC/traceability 차원 수정이 필요. MP-3만 해소된 상태이므로 audit 재실행 시 여전히 대다수 결함 잔존.

**권고**: `/moai plan --resume SPEC-XXX` 플로우로 SPEC별 plan-auditor iteration 수행. 이번 세션에서 확립된 Phase A 규칙에 의해 재생성은 올바른 schema로 진행됨.

### 5.2 구현 결함 미완 (Major 수준)

- Kimi 장문 context INFO 로그 미구현 (ADAPTER-002 D3)
- GLM `budget_tokens` 미구현 (ADAPTER-002 D2)
- OpenRouter PreferredProviders 미구현 (ADAPTER-002 D4)
- Google 429 rotation (genai SDK HTTP 상태 코드 미노출)

**권고**: ADAPTER-002 SPEC을 올바른 schema로 재작성 후 `/moai run` 재실행.

### 5.3 SPEC 간 경계 모호

- BRIDGE-001 vs TRANSPORT-001 (amendment vs 본문 충돌)
- ONBOARDING-001 CLI vs Desktop scope 모순
- JOURNAL vs MEMORY vs TRAJECTORY 일부 중복
- LOCALE vs I18N의 number/date format gray zone

**권고**: `/moai design` 또는 `/moai plan` annotation cycle로 경계 명확화.

---

## 6. 성공 지표

이번 세션 단일 성과:

- ✅ 30 SPEC 독립 감사 완료 (이전에 한 번도 없었음)
- ✅ 386 결함 식별 및 분류
- ✅ 근본 원인 5개 축으로 명시화
- ✅ 4개 파일 수정으로 재발 방지 메커니즘 확립
- ✅ 50 SPEC frontmatter 일괄 정합화 (90 결함 해소)
- ✅ 6 Critical 코드 결함 수정 + 11 테스트 추가
- ✅ 0.1.0 릴리스 전 구조적 결함 대부분 수면 위로

## 7. 참조

- **커밋 체인**:
  - `35a3f18` Phase A — 워크플로우 결함 방지 4건
  - `8b5150c` Phase B — 50 SPEC frontmatter 마이그레이션
  - `79d92ff` Phase C1 — 6 Critical 코드 결함 수정
  - `881ced6` mass audit 리포트 30건 아카이브
- **감사 리포트**: `.moai/reports/plan-audit/mass-20260425/*.md` (30개)
- **자료**: CLAUDE.local.md §1 (Enhanced GitHub Flow), .moai/config/sections/harness.yaml

---

Generated: 2026-04-25
Author: MoAI orchestrator (Opus 4.7) via plan-auditor + expert-backend delegation
