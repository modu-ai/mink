# MINK Specs Review Report

Date: 2026-04-21
Scope: `.moai/specs/IMPLEMENTATION-ORDER.md`, `.moai/specs/ROADMAP.md`, active `SPEC-*/spec.md`, `research.md`, retained `DEPRECATED.md`

## 1. Executive Summary

이번 리뷰의 결론은 단순한 "문서 다듬기" 수준이 아니다. 현재 `.moai/specs`는 개별 문서 품질은 전반적으로 높지만, 상위 계획 문서와 하위 스펙이 동시에 진화하면서 다음 4가지 축에서 정합성이 깨져 있다.

1. 상위 문서가 더 이상 단일 소스 오브 트루스가 아니다.
2. Phase 5 계약(REFLECT/SAFETY/ROLLBACK/HOOK/MEMORY)이 서로 맞물리지 않는다.
3. CONFIG-001이 후속 스펙들이 요구하는 설정 표면을 수용하지 못한다.
4. v1 폐기 스펙의 잔존 참조와 누락된 보조 문서 때문에 독자가 현재 아키텍처를 오해하기 쉽다.

가장 시급한 수정은 문서 추가가 아니라, 기존 문서 간 계약을 다시 맞추는 것이다.

## 2. 핵심 발견

### Critical 1. `ROADMAP.md`의 실행 순서가 자체 의존성과 충돌한다

- `ROADMAP.md:205-209`는 MVP Milestone 1에 `TOOLS-001 -> CLI-001`를 넣는다.
- 그러나 같은 문서의 Phase 표는 `TOOLS-001`이 `MCP-001`에 의존하고 (`ROADMAP.md:110`), `CLI-001`이 `COMMAND-001`에 의존한다고 적는다 (`ROADMAP.md:112`).
- 실제 하위 스펙도 이를 확인한다.
  - `SPEC-GOOSE-TOOLS-001/spec.md:519-523`
  - `SPEC-GOOSE-CLI-001/spec.md:727-732`
- 즉, 현재 문서 그대로 구현하면 MVP 1 순서대로는 바로 막힌다.

개선 필요:

- `ROADMAP.md`의 "의존성" 컬럼이 "주요 차단자만 적은 것인지", "실제 선행 조건 전체인지"를 명시해야 한다.
- `ROADMAP.md`의 Milestone 1/2/3를 `IMPLEMENTATION-ORDER.md`와 같은 순서로 재정렬해야 한다.
- `SPEC-GOOSE-CLI-001/spec.md:63`와 `SPEC-GOOSE-TOOLS-001/spec.md:46`에 남아 있는 "MVP Milestone 1 블로커" 표현도 함께 교정해야 한다.

### Critical 2. Phase 5 계약이 스펙 간에 연결되지 않는다

#### 2-1. `SAFETY-001`이 전제하는 HOOK 이벤트가 `HOOK-001`에 없다

- `SPEC-GOOSE-SAFETY-001/spec.md:84`, `:119`, `:489`는 `HookEventApprovalRequest`를 전제한다.
- `SPEC-GOOSE-ROLLBACK-001/spec.md:149`, `:304`, `:456`는 `HookEventRollbackManualIntervention`를 전제한다.
- 그러나 `SPEC-GOOSE-HOOK-001/spec.md`에는 두 이벤트 모두 정의되어 있지 않다. 실제 enum은 24개이며 승인/롤백 수동 개입 이벤트가 없다 (`SPEC-GOOSE-HOOK-001/spec.md:236-260`).

#### 2-2. `SAFETY-001`의 인터페이스가 요구사항을 만족하지 못한다

- 요구사항은 승인 시 `SafetyToken`을 반환해야 한다 (`SPEC-GOOSE-SAFETY-001/spec.md:120`, `:318-319`, `:416-418`).
- 하지만 실제 인터페이스는 `ReceiveDecision(... ) error` 뿐이다 (`SPEC-GOOSE-SAFETY-001/spec.md:179-183`).
- 현재 시그니처로는 "승인 결과를 호출자에게 반환"하는 계약을 표현할 수 없다.

#### 2-3. `REFLECT-001` 내부에도 API 시그니처가 두 번 다르게 나온다

- Scope 섹션의 `Promoter`에는 `MarkGraduated(id, approver, approvedAt)`만 있다 (`SPEC-GOOSE-REFLECT-001/spec.md:67-74`).
- Go 타입 시그니처에는 `approvalToken SafetyToken`이 추가된다 (`SPEC-GOOSE-REFLECT-001/spec.md:271-277`).
- 요구사항과 AC는 토큰 버전을 기준으로 서술한다 (`SPEC-GOOSE-REFLECT-001/spec.md:150`, `:154`, `:328-334`).

#### 2-4. `ROLLBACK-001`이 잘못된 소유자에게 상태 전이를 호출한다

- 문서 서술은 REFLECT가 `MarkRolledBack`의 소유자라고 적는다 (`SPEC-GOOSE-ROLLBACK-001/spec.md:43`, `:134`, `:451`).
- 그런데 코드 예시는 `e.memory.MarkRolledBack(...)`를 호출한다 (`SPEC-GOOSE-ROLLBACK-001/spec.md:389-391`).
- 이는 `REFLECT-001`의 책임 경계와 충돌한다.

#### 2-5. `REFLECT-001`이 요구하는 MEMORY 인터페이스가 `MEMORY-001`에 없다

- `REFLECT-001`은 `MemoryProvider.SyncLearningEntry`와 `LoadLearningEntries()`를 요구한다 (`SPEC-GOOSE-REFLECT-001/spec.md:94-95`, `:415-416`).
- `MEMORY-001`의 `MemoryProvider`에는 이 메서드들이 없다 (`SPEC-GOOSE-MEMORY-001/spec.md:263-290`).
- 현재 상태로는 REFLECT의 persistence hand-off가 문서상 성립하지 않는다.

개선 필요:

- Phase 5 공통 계약을 별도 "cross-phase contract" 부록 또는 `CONTRACTS.md`로 분리해 한 번만 정의해야 한다.
- `HOOK-001`에 승인/수동개입 이벤트를 추가하거나, SAFETY/ROLLBACK이 HookEmitter의 별도 인터페이스를 쓰는 것으로 명확히 분기해야 한다.
- `SAFETY-001`, `REFLECT-001`, `ROLLBACK-001`, `MEMORY-001`의 인터페이스를 한 번에 맞춰야 한다. 부분 수정은 오히려 혼란을 키운다.

### Critical 3. `IMPLEMENTATION-ORDER.md`의 REQ/AC 집계가 실제 스펙과 맞지 않는다

- 문서는 총합을 `563 REQ / 328 AC`로 주장한다 (`IMPLEMENTATION-ORDER.md:71`).
- 실제 active 30개 `spec.md`에서 REQ/AC 식별자를 집계하면 `551 REQ / 342 AC`다.
- 특히 다음 8개 문서가 상위 집계와 다르다.

| SPEC | `IMPLEMENTATION-ORDER.md` | 실제 `spec.md` 식별자 수 |
|------|----------------------------|----------------------------|
| CLI-001 | 25 REQ / 16 AC | 27 REQ / 17 AC |
| COMMAND-001 | 19 / 13 | 22 / 14 |
| CONTEXT-001 | 16 / 10 | 16 / 12 |
| ROLLBACK-001 | 12 / 7 | 13 / 7 |
| SUBAGENT-001 | 20 / 12 | 22 / 12 |
| TOOLS-001 | 20 / 9 | 24 / 12 |
| TRAJECTORY-001 | 18 / 12 | 19 / 12 |
| TRANSPORT-001 | 14 / 8 | 15 / 8 |

영향:

- 난이도/공수 추정이 왜곡된다.
- 테스트 범위와 milestone exit criteria가 잘못 잡힌다.

개선 필요:

- `IMPLEMENTATION-ORDER.md`의 집계값은 수작업이 아니라 자동 생성으로 바꾸는 것이 좋다.
- 최소한 `REQ/AC count generated at <date>` 같은 출처 메타데이터를 붙여야 한다.

### Critical 4. `CONFIG-001`이 후속 스펙들의 설정 표면을 수용하지 못한다

- `CONFIG-001`이 정의하는 스키마는 `Log`, `Transport`, `LLM`, `Learning`, `UI` 중심이다 (`SPEC-GOOSE-CONFIG-001/spec.md:207-234`).
- 그러나 후속 스펙은 이미 아래 설정을 요구한다.
  - `reflect.confidence_decay_enabled`, `reflect.max_active_entries` (`SPEC-GOOSE-REFLECT-001/spec.md:144-145`)
  - `safety.frozen_paths`, `safety.canary_threshold` (`SPEC-GOOSE-SAFETY-001/spec.md:131-132`)
  - `rollback.regression_threshold`, `rollback.evaluation_interval` (`SPEC-GOOSE-ROLLBACK-001/spec.md:103-106`, `:143-144`)
  - `config.insights.pricing` (`SPEC-GOOSE-INSIGHTS-001/spec.md:75`, `:180`)
  - `a2a.transport`, `a2a.allow_multihop`, `a2a.*`, `acp.*` (`SPEC-GOOSE-A2A-001/spec.md:146`, `:167`, `:490`)
  - `memory.*` (`SPEC-GOOSE-MEMORY-001/spec.md:576-584`)

개선 필요:

- `CONFIG-001`을 "Phase 0 최소 스키마"와 "전체 제품 스키마" 중 무엇으로 볼지 먼저 정의해야 한다.
- 전체 제품 스키마라면 후속 스펙의 설정 키들을 수용하도록 확장해야 한다.
- Phase 0 최소 스키마라면, `ROADMAP.md`와 `IMPLEMENTATION-ORDER.md`에 "후속 SPEC이 `Config` 서브트리를 증설한다"는 규칙을 명문화해야 한다.

## 3. 상위 문서 차원의 개선 필요 사항

### High 1. 물리적 문서 수와 활성 스펙 수가 분리되어 있는데, 문서가 이를 명확히 설명하지 않는다

- 디스크상 `SPEC-*` 디렉터리는 32개다.
- 활성 스펙은 30개이며, `SPEC-GOOSE-AGENT-001`, `SPEC-GOOSE-LLM-001`은 보존된 deprecated 디렉터리다 (`ROADMAP.md:232-241`).
- 지금 구조에서는 단순히 폴더 목록만 본 사람이 "30개냐 32개냐"를 바로 이해하기 어렵다.

개선 필요:

- `.moai/specs/INDEX.md`를 추가해 `active / deprecated / rewritten / external roadmap`을 구분해야 한다.
- 혹은 deprecated 디렉터리를 `.moai/specs/deprecated/`로 옮기는 편이 더 안전하다.

### High 2. `ROADMAP.md`와 `IMPLEMENTATION-ORDER.md`가 같은 정보를 다른 방식으로 유지하며 드리프트한다

대표 사례:

- Ecosystem 수량 표기:
  - `ROADMAP.md:146-154`
  - `IMPLEMENTATION-ORDER.md:68-69`
- Critical path 계산:
  - `IMPLEMENTATION-ORDER.md:320-330`
  - 같은 줄의 "나머지 11 SPEC" 서술은 실제로 10개 명시 + deprecated 3개를 함께 적어 산술이 어긋난다.
- QUERY-001의 차기 버전 계획:
  - `IMPLEMENTATION-ORDER.md:204`, `:232`
  - 실제 `SPEC-GOOSE-QUERY-001/spec.md`는 아직 `version: 0.1.0`이다 (`SPEC-GOOSE-QUERY-001/spec.md:2-21`).

개선 필요:

- `ROADMAP.md`는 "무엇을 할 것인가", `IMPLEMENTATION-ORDER.md`는 "어떤 순서와 병렬도로 갈 것인가"로 역할을 더 명확히 분리해야 한다.
- 반복 집계값(REQ/AC 수, blocker 수, phase totals)은 한 문서에서만 관리하거나 자동 생성해야 한다.

### High 3. 누락된 참조 문서가 여러 군데 존재한다

실제 누락 확인:

- `ROADMAP-ECOSYSTEM.md`
- `ROADMAP-RUST.md`
- `ROADMAP-CLIENTS.md`
- `.moai/project/security.md`
- `run.md`

참조 위치 예:

- `ROADMAP.md:154`, `:273-275`
- `IMPLEMENTATION-ORDER.md:307`, `:369`, `:438`
- `SPEC-GOOSE-LORA-001/spec.md:36`, `:59`, `:103`, `:453`
- `SPEC-GOOSE-TRAJECTORY-001/spec.md:466`

개선 필요:

- 문서가 아직 없으면 "planned, not created" 상태를 명시해야 한다.
- 빈 placeholder라도 만들어 링크를 살리는 것이 낫다.

## 4. 활성 스펙 본문 차원의 개선 필요 사항

### High 4. v1 아키텍처 참조가 active v2 스펙에 남아 있다

대표 사례:

- `SPEC-GOOSE-CORE-001/spec.md:42`, `:54`, `:297`가 `LLM-001`, `AGENT-001`을 후속으로 적는다.
- `SPEC-GOOSE-CONFIG-001/spec.md:82`, `:234`, `:285-286`, `:334`가 `LLM-001`, `AGENT-001`을 전제로 한다.
- `SPEC-GOOSE-TRANSPORT-001/spec.md:77`, `:309-310`가 `AGENT-001`, `LLM-001`, `TOOL-001`을 사용한다.

영향:

- 새 기여자가 v2 구조를 학습할 때 폐기된 스펙이 아직 살아 있는 것으로 보인다.
- `TOOL-001` vs `TOOLS-001`처럼 ID 체계까지 혼동된다.

개선 필요:

- active 스펙에서 deprecated ID를 직접 참조하지 않도록 정리해야 한다.
- 필요한 경우 "historical note" 섹션으로만 격리해야 한다.

### High 5. dependency 섹션이 "핵심 선행"과 "실제 전 의존"을 혼용한다

예시:

- `ROADMAP.md:89`는 `PROMPT-CACHE-001` 의존성을 `ROUTER-001` 하나만 적지만, 실제 스펙은 `QUERY-001`도 선행으로 둔다 (`SPEC-GOOSE-PROMPT-CACHE-001/spec.md:339-340`).
- `ROADMAP.md:90`는 `ADAPTER-001`을 `ROUTER-001`만 선행으로 적지만, 실제 스펙은 `CONFIG/CORE/CREDPOOL/RATELIMIT/PROMPT-CACHE/QUERY/ROUTER`를 모두 소비한다 (`SPEC-GOOSE-ADAPTER-001/spec.md:591-603`).
- `ROADMAP.md:100`는 `SUBAGENT-001` 의존성에 HOOK-001이 빠져 있지만 실제 스펙은 HOOK를 선행으로 둔다 (`SPEC-GOOSE-SUBAGENT-001/spec.md:478-480`).
- `ROADMAP.md:152`는 `A2A-001` 의존성을 `MCP-001, SUBAGENT-001`만 적지만 실제 스펙은 `RATELIMIT-001`, `CONFIG-001`도 요구한다 (`SPEC-GOOSE-A2A-001/spec.md:490`).

개선 필요:

- 상위 문서에 `primary blockers` 컬럼과 `all compile-time/runtime deps` 컬럼을 분리하는 것이 좋다.
- dependency graph와 phase table의 기준을 통일해야 한다.

### High 6. `HOOK-001` 문서가 자기 자신과 모순된다

- AC는 "정확히 24개"라고 하면서 24개가 아닌 긴 목록을 적는다 (`SPEC-GOOSE-HOOK-001/spec.md:162-165`).
- enum 상수는 실제로 24개뿐이며 `Elicitation`, `ElicitationResult`, `InstructionsLoaded`는 없다 (`SPEC-GOOSE-HOOK-001/spec.md:236-260`).
- 그런데 리스크/제외 항목에서는 이 이벤트들이 "추후 추가" 혹은 "이벤트 상수 + 기본 dispatcher만" 있다고 적는다 (`SPEC-GOOSE-HOOK-001/spec.md:509`, `:547`).

개선 필요:

- event catalog를 표 하나로 고정하고 `implemented / declared only / out of scope`를 명시해야 한다.
- SAFETY/ROLLBACK이 요구하는 이벤트까지 포함해 전체 catalog를 재정의하는 편이 낫다.

### Medium 1. 템플릿 드리프트가 크다

현재 최소 3개의 문서 패턴이 섞여 있다.

- 패턴 A: `개요 -> 배경 -> 스코프 -> EARS -> 수용 기준 -> 기술적 접근 -> 의존성 -> 리스크 -> 참고 -> Exclusions`
- 패턴 B: `Goals/Non-Goals`, `Design Decisions`, `Data Model`, `Dependencies & Impact`, `Open Issues`
- 패턴 C: Phase 5 계열처럼 `Go 타입 시그니처`가 앞쪽으로 올라가 있고 dependencies 섹션이 별도 없다

이 자체가 잘못은 아니지만, 현재는 생성 시점에 따라 템플릿이 바뀐 흔적처럼 보여 자동 리뷰와 비교가 어렵다.

개선 필요:

- active 스펙의 표준 템플릿을 하나로 고정해야 한다.
- 최소 공통 섹션은 강제하는 것이 좋다:
  - Scope
  - Requirements
  - Acceptance Criteria
  - Public Interfaces
  - Dependencies
  - Risks
  - Open Issues

### Medium 2. open issue가 많지만 owner, due date, resolution rule이 없다

예시:

- `SPEC-GOOSE-A2A-001/spec.md` open issues 8개
- `SPEC-GOOSE-IDENTITY-001/spec.md` open issues 5개
- `SPEC-GOOSE-LORA-001/spec.md` open issues 7개
- 다수 `research.md`도 4~8개의 open issue를 가진다

현재는 "남아 있는 고민"은 잘 기록되지만, 무엇이 구현 차단인지/추후 개선인지의 우선순위 정보가 없다.

개선 필요:

- open issue 항목마다 `blocking / non-blocking / post-MVP` 라벨을 붙여야 한다.
- `ROADMAP.md`의 결정 포인트와 각 spec/research의 open issues를 연결하는 추적 표가 있으면 좋다.

### Medium 3. 일부 AC/문구가 아직 설계 검토용 표현에 머문다

예시:

- `SPEC-GOOSE-INSIGHTS-001/spec.md:179`의 AC 이름이 아직 "`모델 가격 미정`"이다.
- `SPEC-GOOSE-ROUTER-001/spec.md:167`도 "`Cheap route 미정의 시`"라는 제목이라, 설계 상태와 테스트 의도를 분리해서 쓰는 편이 더 명확하다.

개선 필요:

- AC 이름은 "검증 대상 행동" 중심으로 바꾸는 것이 좋다.
- 미정 여부는 Open Issues나 Design Decisions로 빼는 편이 낫다.

## 5. Phase별 세부 코멘트

### Phase 0

- `CORE-001`, `CONFIG-001`, `TRANSPORT-001`이 아직 deprecated v1 분해 전 구조를 일부 전제한다.
- Go 버전도 상위 문서(1.26+)와 하위 문서(1.22+)가 섞여 있다.
  - `ROADMAP.md:286`
  - `IMPLEMENTATION-ORDER.md:433`, `:445`
  - `SPEC-GOOSE-CORE-001/spec.md:262`

권고:

- Phase 0 문서부터 "현재 canonical dependency names"를 먼저 정리해야 한다.
- Go 버전은 별도 ADR이나 `go-version.md` 한 장으로 고정하는 것이 낫다.

### Phase 1

- Multi-LLM 계열은 세부 설계가 좋지만 상위 문서의 dependency 요약이 실제보다 많이 축약돼 있다.
- 가격표/pricing ownership, tokenizer ownership 같은 운영성 의사결정이 아직 흩어져 있다.

권고:

- `ADAPTER/ROUTER/RATELIMIT/PROMPT-CACHE/INSIGHTS`가 공유하는 config/pricing/tokenizer 책임을 한 표로 묶어야 한다.

### Phase 2-3

- `HOOK-001`, `SUBAGENT-001`, `TOOLS-001`, `CLI-001`는 서로의 consumer이기도 하고 blocker이기도 해서 상위 문서의 단일 선형 순서로 설명하기 어렵다.
- 지금은 `ROADMAP.md`와 `IMPLEMENTATION-ORDER.md`가 서로 다른 기준으로 설명한다.

권고:

- "compile-time dependency", "feature dependency", "MVP dependency"를 분리해 표현해야 한다.

### Phase 4-5

- 학습 파이프라인은 문서량은 충분하지만 계약 소유권이 모호하다.
- `MEMORY`가 툴 provider인지 learning state store인지 역할이 두 갈래로 커져 있다.
- `REFLECT`와 `ROLLBACK`의 상태 전이 API 소유권을 다시 정리해야 한다.

권고:

- `MEMORY-001`을 그대로 확장할지, `learning store`를 별도 인터페이스로 분리할지 결정이 필요하다.

### Phase 6-7

- `IDENTITY/VECTOR/LORA/A2A`는 개별 스펙 완성도는 높지만, 상위 보조 로드맵이 실제 파일로 존재하지 않는다.
- 상용화/생태계 영역은 "문서상 분리"가 되어 있으나 "아티팩트상 분리"는 아직 안 됐다.

권고:

- `ROADMAP-RUST.md`, `ROADMAP-ECOSYSTEM.md`, `ROADMAP-CLIENTS.md` placeholder라도 만들어야 한다.

## 6. 우선순위 권고

1. `ROADMAP.md`와 `IMPLEMENTATION-ORDER.md`의 순서/의존성/REQ-AC 집계를 동기화한다.
2. Phase 5 계약을 한 번에 정리한다.
3. `CONFIG-001`의 스키마 범위를 명확히 재선언한다.
4. active 스펙에서 deprecated ID(`LLM-001`, `AGENT-001`, `TOOL-001`)를 제거한다.
5. `HOOK-001`의 event catalog를 재작성한다.
6. 누락된 참조 문서 placeholder를 만든다.
7. `.moai/specs/INDEX.md`를 추가해 active/deprecated/external roadmap를 구분한다.

## 7. 문서별 빠른 액션 매트릭스

| 우선 | 문서 | 필요한 수정 |
|------|------|-------------|
| P0 | `ROADMAP.md` | Milestone 재정렬, dependency 컬럼 의미 명시, ecosystem 설명 정리 |
| P0 | `IMPLEMENTATION-ORDER.md` | REQ/AC 집계 재생성, critical path 산술 교정, QUERY v0.2 계획 분리 |
| P0 | `SPEC-GOOSE-HOOK-001/spec.md` | event catalog 재정의, SAFETY/ROLLBACK 이벤트 반영 |
| P0 | `SPEC-GOOSE-SAFETY-001/spec.md` | `ReceiveDecision` 반환 계약 수정, HOOK 연동 방식 명확화 |
| P0 | `SPEC-GOOSE-REFLECT-001/spec.md` | `Promoter` 시그니처 중복 제거, MEMORY hand-off 계약 정식화 |
| P0 | `SPEC-GOOSE-ROLLBACK-001/spec.md` | `MarkRolledBack` 소유자 수정, manual intervention event 명세 정합화 |
| P0 | `SPEC-GOOSE-MEMORY-001/spec.md` | learning-state persistence를 포함할지 여부 결정 |
| P1 | `SPEC-GOOSE-CONFIG-001/spec.md` | Phase 4-7 설정 키 수용 규칙 추가 |
| P1 | `SPEC-GOOSE-CORE-001/spec.md` | deprecated 후속 SPEC 명칭 제거 |
| P1 | `SPEC-GOOSE-TRANSPORT-001/spec.md` | `TOOL-001` -> `TOOLS-001`, deprecated 후속 교정 |
| P1 | `SPEC-GOOSE-CONFIG-001/spec.md` | `LLM-001`/`AGENT-001` 기반 설명을 v2 구조로 교체 |
| P2 | `SPEC-GOOSE-INSIGHTS-001/spec.md` | AC 명명 개선, pricing ownership 문구 정리 |

## 8. 결론

현재 스펙 세트는 "아이디어 부족"이 아니라 "정합성 유지 장치 부족"이 문제다. 다시 말해 개별 문서는 충분히 깊지만, 상위 계획 문서, cross-phase 계약, 설정 스키마, 이벤트 카탈로그가 중앙에서 잠기지 않아 드리프트가 발생했다.

문서 품질을 가장 빠르게 끌어올리는 방법은 새 스펙을 더 쓰는 것이 아니라, 이미 존재하는 문서 사이의 인터페이스와 소유권을 먼저 잠그는 것이다. 특히 `ROADMAP/IMPLEMENTATION-ORDER`, `HOOK/SAFETY/REFLECT/ROLLBACK`, `CONFIG` 이 세 축을 먼저 정리하면 이후 구현 단계의 혼란이 크게 줄어든다.
