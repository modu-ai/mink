---
id: SPEC-AGENCY-CLEANUP-002
version: 0.1.0
status: implemented
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
priority: P2
issue_number: null
labels:
  - cleanup
  - agency
  - absorb-followup
  - file-removal
---

# SPEC-AGENCY-CLEANUP-002: Legacy Agency 파일 정리 (ABSORB-001 후속)

## HISTORY

- 2026-04-25 (v0.1.0): 초안 작성. SPEC-AGENCY-ABSORB-001 (v1.0.1, status=completed)의 Open Items 섹션에서 명시한 파일시스템 잔존물 19개(`.agency/` 8 파일 + `.claude/skills/agency-*` 5 디렉터리 + `.claude/agents/agency/` 6 파일)를 정식 정리 SPEC으로 분리. 본 SPEC은 계획 수립만을 목적으로 하며, 실제 파일 삭제는 후속 `/moai run`에서 수행한다.

---

## 1. Overview

SPEC-AGENCY-ABSORB-001은 2026-04-24에 `completed` 상태로 종결되었으며, v3.2 agency 프레임워크를 v3.3 `/moai design` 체계로 흡수하는 작업을 완수했다. 그러나 v1.0.1 audit patch(plan-audit mass-20260425) 결과 **파일시스템 잔존물 19개**가 확인되었다:

| 카테고리 | 경로 | 파일 수 |
|---------|------|--------|
| legacy framework 디렉터리 | `.agency/` | 8 |
| legacy skill 디렉터리 | `.claude/skills/agency-{client-interview,copywriting,design-system,evaluation-criteria,frontend-patterns}/` | 5 디렉터리 (각 SKILL.md 포함) |
| legacy agent 파일 | `.claude/agents/agency/{builder,copywriter,designer,evaluator,learner,planner}.md` | 6 |

ABSORB-001은 이 정리 작업을 자체 scope에서 제외하고 Open Items로 이관했으므로(REQ 번호 재배치 금지 제약), 본 SPEC이 이를 **독립된 cleanup SPEC**으로 공식화한다.

**본 SPEC의 목적**: 19개 파일의 안전한 제거 또는 아카이브를 계획하되, (a) 롤백 가능성을 보장하고 (b) 기존 import/참조 체계를 파괴하지 않으며 (c) ABSORB-001이 수립한 흡수 후 상태(v3.3 `/moai design`)에 부합시킨다.

---

## 2. Background

### 2.1 왜 ABSORB-001 scope에 포함되지 않았는가

ABSORB-001의 최초 설계(2026-04-20)는 **"신규 체계 구축 + 기존 stub 유지"** 방식으로, legacy 파일을 즉시 삭제하지 않고 deprecation redirect stub으로 변환했다. `/agency` 서브커맨드 8개는 stub redirect 방식으로 유지되며, `.claude/rules/agency/constitution.md`는 REDIRECT stub으로 남아있다.

그러나 디렉터리 단위 `.agency/`, `.claude/skills/agency-*`, `.claude/agents/agency/`는 stub 변환 대상이 아닌 **순수 legacy 아티팩트**이므로 제거 또는 아카이브가 필요하다. ABSORB-001 audit patch(v1.0.1, 2026-04-25)에서 이를 결함 D3/D4/D5로 분류하고 Open Items로 이관한 뒤, 후속 SPEC-AGENCY-CLEANUP-002(본 SPEC)의 선행 스펙 CL-1~CL-6을 정의했다.

### 2.2 ABSORB-001 Open Items 인용

ABSORB-001 HISTORY v1.0.1에서 인용:

> "(D3/D4/D5) 실행 정합성 결함(`.agency/` 8 파일 잔존, `.claude/skills/agency-*` 5 디렉터리 잔존, `.claude/agents/agency/` 6 파일 잔존)을 Open Items 테이블로 공식 이관하고 후속 SPEC `SPEC-AGENCY-CLEANUP-002`(가칭)의 선행 스펙(CL-1~CL-6) 정의"

본 SPEC은 그 "가칭"을 공식 SPEC ID로 확정한다.

### 2.3 정리 작업이 왜 별도 SPEC으로 분리되어야 하는가

1. **REQ 번호 안정성**: ABSORB-001은 `completed` 상태이며, 41개 REQ가 고정됨. 정리 작업을 ABSORB-001에 추가하려면 REQ 재배치가 필요한데, 이는 characterization SPEC 원칙(이미 구현된 작업은 재구성 전용)에 위배된다.
2. **작업 성격 차이**: ABSORB-001은 "이전 + 신설"(additive), 본 SPEC은 "제거"(destructive). 롤백 전략과 검증 방법이 근본적으로 다르다.
3. **리스크 경계**: 삭제 작업은 실수 시 복구 비용이 크므로 별도 SPEC으로 격리하여 독립적 승인 절차(user confirmation)를 거친다.

---

## 3. Scope

### 3.1 In Scope

| 대상 | 경로 | 파일 수 | 처리 |
|-----|------|--------|------|
| agency framework 루트 | `.agency/config.yaml`, `.agency/fork-manifest.yaml`, `.agency/context/*.md` (5), `.agency/templates/*.md` (1) | 8 | 삭제 또는 `.archive/agency-legacy/` 이전 |
| agency skill 디렉터리 | `.claude/skills/agency-client-interview/`, `agency-copywriting/`, `agency-design-system/`, `agency-evaluation-criteria/`, `agency-frontend-patterns/` | 5 dir | 삭제 또는 아카이브 |
| agency agent 파일 | `.claude/agents/agency/{builder,copywriter,designer,evaluator,learner,planner}.md` | 6 | 삭제 또는 아카이브 |
| 참조 무결성 검증 | import/참조 깨짐 여부 확인 | - | 필수 선행 |

총 19개 artifact(파일/디렉터리)를 대상으로 한다.

### 3.2 Out of Scope

- **`.claude/rules/agency/constitution.md` REDIRECT stub**: ABSORB-001이 2 minor version cycle 유지 선언. 본 SPEC은 건드리지 않음
- **`/agency` 서브커맨드 redirect stub (8개)**: ABSORB-001 REQ-REDIR-* 범위. 본 SPEC과 무관
- **코드 로직 변경**: 본 SPEC은 순수 파일 정리. Go 코드·skill 본문·agent 프롬프트 수정 금지
- **v3.3 스킬/에이전트 수정**: `moai-domain-brand-design`, `moai-domain-copywriting` 등 흡수 후 신규 artifact는 건드리지 않음

---

## 4. EARS Requirements

본 SPEC은 5가지 EARS 패턴을 각 1개씩 사용하여 정리 작업을 규정한다.

### REQ-CL-001 [Ubiquitous]: Legacy 파일 식별 및 추적

시스템은 Legacy agency 파일 목록(`.agency/`, `.claude/skills/agency-*`, `.claude/agents/agency/`)을 **항상** manifest 형태로 명시적으로 추적해야 한다(shall).

- 매니페스트 위치: `.moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml` (run 단계에서 생성)
- 매니페스트 필드: `path`, `type` (file|dir), `size_bytes`, `sha256`, `action` (delete|archive), `status` (pending|done|failed)

### REQ-CL-002 [Event-Driven]: 백업 후 삭제

사용자가 정리 실행을 승인(AskUserQuestion confirm)하는 **이벤트가 발생하면**(when), 시스템은 대상 파일을 `.archive/agency-legacy-{YYYYMMDD}/`로 복사한 후에만 원본을 제거해야 한다(shall).

- 백업 경로 규약: `.archive/agency-legacy-{YYYYMMDD}/` (run 시점의 날짜)
- 백업 생성 실패 시 원본 제거 금지(atomic: all-or-nothing)
- git rm 사용 (단순 `rm` 금지) — git history에 제거 이력 보존

### REQ-CL-003 [Unwanted]: Import 파괴 감지 시 중단

만약 제거 대상 파일에 대한 외부 import/참조가 발견되면(if detected), 시스템은 즉시 중단하고 사용자에게 blocker 리포트를 반환해야 한다(then shall).

- 검사 대상: `.claude/`, `.moai/`, `internal/`, `cmd/` 디렉터리의 텍스트 파일
- 검사 방법: `rg -l "agency-(client-interview|copywriting|design-system|evaluation-criteria|frontend-patterns)|agency/(builder|copywriter|designer|evaluator|learner|planner)\.md|\.agency/"` 등
- 허용 예외: ABSORB-001 SPEC 문서 자체 내 Open Items 언급(문서 내 역사적 참조는 import가 아님)
- `.claude/rules/agency/constitution.md` REDIRECT stub은 OUT OF SCOPE이므로 그 내부 참조는 검사 대상이 아님

### REQ-CL-004 [State-Driven]: 롤백 가능 상태 유지

정리 작업이 **진행 중인 동안**(while), 시스템은 언제라도 한 번의 git 명령(`git restore .` 또는 `git reset --hard HEAD`)으로 원상 복구 가능한 상태를 유지해야 한다(shall).

- 단일 커밋 원칙: cleanup 전체를 **단 하나의 commit**으로 스테이징 (분할 금지)
- commit message trailer: `SPEC: SPEC-AGENCY-CLEANUP-002` 명기
- archive 디렉터리도 동일 commit에 포함 (별도 commit 금지 — 원자성 확보)

### REQ-CL-005 [Optional]: Archive-Only 모드 (대안 경로)

가능한 경우(where feasible), 시스템은 "삭제 대신 archive만" 수행하는 대안 모드를 지원해야 한다(shall).

- 트리거: `/moai run SPEC-AGENCY-CLEANUP-002 --archive-only` 또는 AskUserQuestion에서 "archive only" 옵션 선택
- 동작: 원본 파일을 `.archive/agency-legacy-{YYYYMMDD}/`로 **이동(move)**. 원본 위치에서는 제거되나 git은 rename으로 인식
- 이점: import 참조가 남아있는 경우에도 git log를 통한 경로 추적 가능
- 기본 동작 아님: `--archive-only` 플래그 부재 시 REQ-CL-002의 "백업 후 삭제" 적용

---

## 5. Acceptance Criteria

Given-When-Then 형식으로 명시. 상세 시나리오는 `acceptance.md`에 기술.

### AC-CL-001: Legacy manifest 생성

- **Given**: ABSORB-001이 `completed` 상태이고 legacy 파일 19개가 파일시스템에 존재한다
- **When**: `/moai run SPEC-AGENCY-CLEANUP-002` 시작 시점
- **Then**: `.moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml`이 생성되어야 하며 19개 entry 각각이 `path`, `type`, `size_bytes`, `sha256`, `action` 필드를 포함해야 한다

### AC-CL-002: Archive 우선 수행

- **Given**: legacy manifest가 확정되고 user가 cleanup을 승인했다
- **When**: 파일 제거가 실행된다
- **Then**: `.archive/agency-legacy-{YYYYMMDD}/`에 19개 artifact가 구조 그대로 복사된 후에만 원본이 제거되어야 하며, 복사 실패 시 원본은 그대로 보존되어야 한다

### AC-CL-003: Import 파괴 감지 시 중단

- **Given**: cleanup 대기 중 `.claude/rules/moai/` 하위 어떤 파일이 `agency-copywriting/SKILL.md`를 참조한다고 가정
- **When**: REQ-CL-003 검사가 수행된다
- **Then**: 시스템은 중단되고 사용자에게 "Blocker: N개의 외부 참조가 발견됨" 리포트를 반환해야 하며, 어떤 파일도 제거되지 않아야 한다

### AC-CL-004: 단일 커밋 롤백

- **Given**: cleanup이 성공적으로 커밋되었다
- **When**: `git reset --hard HEAD~1` 실행
- **Then**: 19개 legacy 파일이 원위치로 복원되고, `.archive/agency-legacy-{YYYYMMDD}/` 디렉터리도 제거되어야 한다

### AC-CL-005: Archive-only 모드 경로 보존

- **Given**: user가 `--archive-only` 옵션을 선택했다
- **When**: cleanup이 실행된다
- **Then**: 원본 위치의 파일은 제거되나 `git log --follow` 추적 시 파일 rename 이력이 `.archive/agency-legacy-{YYYYMMDD}/` 하위 경로로 연결되어야 한다

---

## 6. Technical Approach

### 6.1 실행 순서

1. **manifest 생성**: `.agency/`, `.claude/skills/agency-*`, `.claude/agents/agency/` 스캔 → `legacy-manifest.yaml` 작성
2. **import 검사**: `rg` 기반 패턴 검색 → 위반 발견 시 중단 (REQ-CL-003)
3. **archive**: `.archive/agency-legacy-{YYYYMMDD}/` 디렉터리 생성 → rsync 또는 `cp -a`로 구조 복사 → sha256 재검증
4. **원본 제거**: `git rm -r` (standard mode) 또는 `git mv` (archive-only mode)
5. **커밋**: 단일 commit, trailer `SPEC: SPEC-AGENCY-CLEANUP-002`, conventional type `chore(cleanup)`

### 6.2 검증 전략

- **정적 검증**: `rg -l "\.agency/|agency-(client-interview|copywriting|design-system|evaluation-criteria|frontend-patterns)|agents/agency/"` 결과가 허용 예외만 남아있어야 함
- **동적 검증**: `/moai design` 경로 A/B 양쪽 smoke test — 흡수 후 체계만 사용하는지 확인
- **회귀 검증**: ABSORB-001 acceptance 시나리오 재실행하여 깨진 항목 없는지 확인

### 6.3 도구 선택

- `Bash`로 `find`, `rg`, `git rm`, `rsync` 수행 (sed/awk 금지)
- `Read`로 legacy 파일 메타 확인
- `Write`로 `legacy-manifest.yaml` 생성
- `Edit` 사용 금지 (본 SPEC은 순수 삭제/이동)

---

## 7. Dependencies

### 7.1 선행 SPEC

- **SPEC-AGENCY-ABSORB-001** (status=completed, v1.0.1): 흡수 작업 완료 전제. 본 SPEC은 그 Open Items를 수행한다
- ABSORB-001의 v3.3 `/moai design` 체계가 정상 작동 중이어야 함

### 7.2 참조 문서

- `.claude/rules/moai/design/constitution.md` (v3.3.0): 흡수 후 헌법
- `.claude/rules/agency/constitution.md` (REDIRECT stub): 본 SPEC에서는 건드리지 않음
- ABSORB-001 `acceptance.md`: Quality Gate Checklist "41개 REQ" 일관성 유지

### 7.3 후속 SPEC (예상)

- 본 SPEC 완료 후, `/agency` 서브커맨드 redirect stub 8개 제거를 다루는 `SPEC-AGENCY-STUB-REMOVAL-003`(가칭)이 2 minor version cycle 후 작성될 수 있다 (ABSORB-001 REQ-DEPRECATE-003 참조).

---

## 8. Risks

| ID | 리스크 | 영향 | 완화 |
|----|-------|------|------|
| R-CL-01 | 숨은 import/참조로 인한 빌드 파괴 | High | REQ-CL-003 강제 검사, 위반 시 halt |
| R-CL-02 | accidental deletion (잘못된 glob) | Critical | REQ-CL-002 archive-first, REQ-CL-004 단일 commit 롤백 |
| R-CL-03 | `.archive/` 경로가 `.gitignore`에 포함되어 백업 미커밋 | Medium | run 단계에서 `.gitignore` 사전 확인, 필요 시 `!.archive/` 예외 추가 |
| R-CL-04 | sha256 불일치로 인한 복사 실패 | Low | REQ-CL-002 원자성 확보, 실패 시 archive 롤백 |
| R-CL-05 | git mv가 rename으로 감지 실패 (50% 임계) | Low | `git config diff.renames 100%` 또는 large-file 분할 이동 |
| R-CL-06 | 향후 ABSORB-001 복원 필요 시 legacy 자료 소실 | Medium | REQ-CL-005 archive-only 모드로 `.archive/` 보존 |

---

## Exclusions (What NOT to Build)

[HARD] 본 SPEC은 **계획**만 수립한다. 실제 파일 제거는 후속 `/moai run`에서 수행.

구체적 배제 항목:

- **실제 파일 삭제 작업 수행 금지**: 본 SPEC(Plan 단계)은 `spec.md`/`plan.md`/`acceptance.md`/`spec-compact.md` 4개 문서만 생성. `.agency/`, `.claude/skills/agency-*`, `.claude/agents/agency/` 실파일은 건드리지 않음
- **코드(.go) 변경 금지**: `internal/`, `cmd/` 이하 Go 소스 수정 불가
- **`.claude/rules/agency/constitution.md` REDIRECT stub 건드림 금지**: ABSORB-001 REQ-DEPRECATE-003 관할
- **`/agency` 서브커맨드 stub 제거 금지**: 2 minor version cycle 유지 약속 존중
- **흡수 후 신규 artifact 수정 금지**: `moai-domain-brand-design`, `moai-domain-copywriting`, `/moai design` router 등 v3.3 자산은 out of scope
- **research.md 생성 금지**: 본 SPEC은 소량·단순(19 파일 정리)이므로 연구 artifact 없이 직접 plan 단계로 진입
- **REQ 재배치 또는 41개 REQ footer 수정 금지**: ABSORB-001의 REQ 번호 안정성 계약 준수

---

## 9. Quality Gate Checklist

본 SPEC이 `planned` → `in-progress` → `completed`로 전환되기 위한 gate:

- [ ] `spec.md` / `plan.md` / `acceptance.md` / `spec-compact.md` 4개 파일 생성 (Plan 단계 완료 기준)
- [ ] EARS 5종 패턴 각 1회 이상 사용 (REQ-CL-001~005)
- [ ] Exclusions 섹션 최소 1개 항목 명시 (HARD 요구)
- [ ] ABSORB-001 참조 명시적 (Section 2, 7)
- [ ] research.md 미생성 (본 SPEC 특성)
- [ ] Rollback 전략 명시 (REQ-CL-004, AC-CL-004)

Run 단계 진입 시 추가 gate는 `plan.md` Section 3을 참조.

---

Version: 0.1.0
Classification: cleanup-followup
Companion SPEC: SPEC-AGENCY-ABSORB-001 (v1.0.1, completed)
REQ coverage: REQ-CL-001, REQ-CL-002, REQ-CL-003, REQ-CL-004, REQ-CL-005
