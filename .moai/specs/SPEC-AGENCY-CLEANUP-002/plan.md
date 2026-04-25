---
spec_id: SPEC-AGENCY-CLEANUP-002
version: 0.1.0
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
related: SPEC-AGENCY-ABSORB-001
---

# Implementation Plan: SPEC-AGENCY-CLEANUP-002

ABSORB-001 후속 cleanup 실행 계획. 본 문서는 `/moai run SPEC-AGENCY-CLEANUP-002` 시점의 상세 단계를 규정한다.

---

## 1. Priority & Ordering

본 SPEC은 우선순위 P2 (cleanup/hygiene 작업). ABSORB-001 완료(`completed`) 이후에만 실행 가능하며, 0.1.0 release 이전 cleanup track에 속한다.

**Milestone 순서** (시간 추정 없음, 우선순위·선후관계만):

- **Milestone CL-M1 (Priority High)**: Legacy manifest 생성 및 정적 import 검증 — 차단 조건 확정
- **Milestone CL-M2 (Priority High)**: Archive 디렉터리 구성 및 sha256 검증 — REQ-CL-002 전제
- **Milestone CL-M3 (Priority High)**: 원본 제거 (standard mode) 또는 rename 이동 (archive-only mode) — REQ-CL-002/CL-005
- **Milestone CL-M4 (Priority Medium)**: 단일 커밋 작성 및 rollback 검증 — REQ-CL-004, AC-CL-004
- **Milestone CL-M5 (Priority Medium)**: 회귀 smoke test — `/moai design` 경로 A/B 동작 확인
- **Milestone CL-M6 (Priority Low)**: ABSORB-001 Open Items 해결 확인 및 HISTORY 추가 — 완료 보고

---

## 2. Technical Approach

### 2.1 도구 매트릭스

| 작업 | 주 도구 | 대안 | 비고 |
|-----|--------|------|------|
| 디렉터리 스캔 | `Bash` + `find -type f` | — | 19개 파일 enumerate |
| import 검색 | `Grep` (rg backend) | — | 패턴 기반, 정규식 활용 |
| 해시 계산 | `Bash` + `shasum -a 256` | — | manifest용 sha256 |
| 디렉터리 복사 | `Bash` + `rsync -a` | `cp -a` | 구조 보존, 권한·심볼릭 링크 유지 |
| 파일 제거 | `Bash` + `git rm -r` | — | history 보존 |
| rename 이동 | `Bash` + `git mv` | — | archive-only 모드 |
| 매니페스트 작성 | `Write` | — | YAML 포맷 |

**금지 도구**: `sed`, `awk` (coding-standards.md 준수), 단순 `rm` (git history 손실)

### 2.2 처리 플로우

```
[start]
  │
  ├─ Scan legacy files (.agency, .claude/skills/agency-*, .claude/agents/agency)
  │   └─ Output: list of 19 artifacts
  │
  ├─ Generate legacy-manifest.yaml
  │   └─ Fields: path, type, size_bytes, sha256, action
  │
  ├─ Static import check (rg)
  │   ├─ Violations found? → HALT (REQ-CL-003) → Blocker report
  │   └─ Clean? → proceed
  │
  ├─ Confirm with user (AskUserQuestion: standard | archive-only | abort)
  │
  ├─ Create .archive/agency-legacy-{YYYYMMDD}/
  │   └─ rsync -a from source → verify sha256 parity
  │
  ├─ Branch:
  │   ├─ standard: git rm -r on originals
  │   └─ archive-only: git mv originals → .archive/.../
  │
  ├─ Single commit (chore(cleanup): ABSORB-001 Open Items 정리)
  │   ├─ trailer: SPEC: SPEC-AGENCY-CLEANUP-002
  │   └─ trailer: REQ: REQ-CL-001..005
  │
  ├─ Smoke test: /moai design path A + path B
  │
  └─ HISTORY 추가: ABSORB-001 Open Items 해결 기록 (cross-reference)
```

### 2.3 Commit 메시지 규약

conventional + 한국어 본문 + trailer (CLAUDE.local.md §2.2 준수):

```
chore(cleanup): SPEC-AGENCY-CLEANUP-002 legacy agency 파일 19개 정리

- .agency/ 디렉터리 8개 파일 제거 (config.yaml, fork-manifest.yaml, context/5, templates/1)
- .claude/skills/agency-* 5개 디렉터리 제거
- .claude/agents/agency/ 6개 파일 제거
- .archive/agency-legacy-2026MMDD/ 전체 백업 보존 (단일 commit 원자성)
- ABSORB-001 Open Items (D3/D4/D5) 해결

SPEC: SPEC-AGENCY-CLEANUP-002
REQ: REQ-CL-001, REQ-CL-002, REQ-CL-003, REQ-CL-004, REQ-CL-005
AC: AC-CL-001, AC-CL-002, AC-CL-003, AC-CL-004, AC-CL-005
```

---

## 3. Milestone Detail

### 3.1 Milestone CL-M1: Manifest & Import Check (Priority High)

**목표**: 제거 대상 19개 artifact 확정 + 외부 참조 여부 판정

**단계**:

1. `Bash`: `find .agency .claude/skills/agency-client-interview .claude/skills/agency-copywriting .claude/skills/agency-design-system .claude/skills/agency-evaluation-criteria .claude/skills/agency-frontend-patterns .claude/agents/agency -type f` 실행
2. 결과를 `legacy-manifest.yaml`의 `artifacts[]` 배열로 구성
3. 각 파일에 대해 `shasum -a 256` 계산 후 manifest에 기록
4. `Grep`로 import 참조 검사:
   - 패턴 1: `agency-(client-interview|copywriting|design-system|evaluation-criteria|frontend-patterns)`
   - 패턴 2: `agents/agency/(builder|copywriter|designer|evaluator|learner|planner)`
   - 패턴 3: `\.agency/`
   - 검색 범위: `.claude/`, `.moai/`, `internal/`, `cmd/`, `docs/` (단 `.moai/specs/SPEC-AGENCY-ABSORB-001/`, `.moai/specs/SPEC-AGENCY-CLEANUP-002/`, `.claude/rules/agency/constitution.md`는 예외)
5. 위반 0건 시 M2 진입, 1건 이상 시 blocker report 반환

**산출물**: `.moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml`

**완료 조건**: manifest에 19개 entry 존재 + import violation = 0 (또는 사용자 예외 승인)

### 3.2 Milestone CL-M2: Archive Creation (Priority High)

**목표**: 원본 제거 전에 복구 가능한 백업 확보

**단계**:

1. `.archive/agency-legacy-{YYYYMMDD}/` 디렉터리 생성 (YYYYMMDD는 run 실행 날짜)
2. `.gitignore`에 `.archive/` 제외 규칙이 있는지 확인 → 있으면 `!.archive/agency-legacy-*` 예외 추가 권장 (manager-git에 위임)
3. `rsync -a` 또는 `cp -a`로 19개 artifact를 구조 그대로 복사
   - `.agency/` → `.archive/agency-legacy-{DATE}/.agency/`
   - `.claude/skills/agency-*` → `.archive/agency-legacy-{DATE}/.claude/skills/agency-*`
   - `.claude/agents/agency/` → `.archive/agency-legacy-{DATE}/.claude/agents/agency/`
4. 복사 후 sha256 재계산하여 manifest와 대조
5. 불일치 1건 이상 시 archive 롤백 (rm -rf `.archive/agency-legacy-{DATE}/`) 후 abort

**완료 조건**: 19개 artifact의 sha256 일치 확인 완료

### 3.3 Milestone CL-M3: Source Removal (Priority High)

**목표**: 원본 legacy 파일을 제거하거나 archive로 이동

**모드 분기**:

- **Standard mode** (기본):
  - `git rm -r .agency/` (8 파일 일괄)
  - `git rm -r .claude/skills/agency-client-interview/` × 5 (각 디렉터리)
  - `git rm .claude/agents/agency/*.md` (6 파일)
- **Archive-only mode** (`--archive-only`):
  - `git mv .agency/ .archive/agency-legacy-{DATE}/.agency/` 형태의 rename
  - rsync 단계를 생략하고 git이 rename으로 인식하도록 `git config diff.renames true` 확인

**완료 조건**: 원본 위치에 19개 artifact가 더 이상 존재하지 않음 (archive 제외)

### 3.4 Milestone CL-M4: Commit & Rollback Check (Priority Medium)

**목표**: 단일 commit 원자성 확보 + 롤백 가능성 검증

**단계**:

1. `git status` 확인: staged changes가 archive 생성 + 원본 제거를 모두 포함하는지
2. 단일 commit 작성 (§2.3 메시지 규약)
3. **Rollback dry-run 검증**: 별도 branch(`rollback-test/CL-002-DATE`) 생성 → `git reset --hard HEAD~1` → 19개 파일 원위치 복원 확인 → branch 삭제 (원 branch 영향 없음)

**완료 조건**: single commit 작성 + rollback dry-run 성공

### 3.5 Milestone CL-M5: Regression Smoke Test (Priority Medium)

**목표**: cleanup이 `/moai design` 흐름을 깨지 않았는지 확인

**단계**:

1. `/moai design --help` 또는 router 호출 경로 확인 (path A/path B 분기 존재)
2. `moai-domain-brand-design` 스킬 로드 시도 (frontmatter 기반)
3. `moai-domain-copywriting` 스킬 로드 시도
4. ABSORB-001 acceptance.md의 통과 기준 중 파일 존재 기반 체크만 재실행 (실제 디자인 생성은 scope 밖)

**완료 조건**: v3.3 `/moai design` 체계가 cleanup 이전과 동일하게 동작

### 3.6 Milestone CL-M6: History & Reporting (Priority Low)

**목표**: ABSORB-001 Open Items 해결 공식 기록

**단계**:

1. 본 SPEC `spec.md` HISTORY에 완료 엔트리 추가 (v0.1.0 → v0.1.1)
2. ABSORB-001 `spec.md`에는 별도 편집하지 않음 (characterization SPEC 원칙: completed 상태 불변)
3. 사용자에게 완료 리포트 제출: (a) 제거된 19 artifact 요약 (b) archive 경로 (c) rollback 명령 (d) smoke test 결과

**완료 조건**: SPEC 상태 `planned` → `completed` 전환 + 완료 리포트 제출

---

## 4. Risk Mitigation Matrix

| Risk (spec.md §8) | Milestone | 완화 조치 | Fallback |
|------------------|-----------|----------|---------|
| R-CL-01 Import 파괴 | CL-M1 | rg 정적 검사 | halt + blocker report |
| R-CL-02 accidental deletion | CL-M2, CL-M3 | archive-first | `git reset --hard HEAD~1` |
| R-CL-03 `.archive/` gitignore 제외 | CL-M2 | pre-check, `!.archive/...` 추가 | manager-git 협의 |
| R-CL-04 sha256 불일치 | CL-M2 | post-copy 재검증 | archive 롤백 후 abort |
| R-CL-05 rename 미감지 | CL-M3 | `diff.renames=true` 설정 | split move (파일별 git mv) |
| R-CL-06 legacy 복원 필요 | 전 단계 | `.archive/` 보존 | archive-only mode 권장 |

---

## 5. Test Strategy

본 SPEC은 코드 변경이 없으므로 전통적 unit test는 없다. 검증은 파일시스템 관찰 기반:

- **AC-CL-001**: `test -f .moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml` + entry 개수 == 19
- **AC-CL-002**: `test -d .archive/agency-legacy-{DATE}` + 19 artifact sha256 일치
- **AC-CL-003**: 의도적 violation 주입 시 halt 동작 확인 (수동 smoke)
- **AC-CL-004**: rollback dry-run branch에서 파일 복원 확인
- **AC-CL-005**: archive-only 모드에서 `git log --follow .archive/agency-legacy-{DATE}/.agency/config.yaml` 이 원본 `.agency/config.yaml`까지 추적

상세 시나리오는 `acceptance.md` 참조.

---

## 6. Completion Definition

- 6개 milestone (CL-M1~CL-M6) 전체 완료
- 19개 artifact가 원본 위치에서 제거되거나 archive로 이동됨
- single commit 작성 + rollback dry-run 성공
- `/moai design` path A/B smoke test 통과
- SPEC HISTORY에 완료 엔트리 기록
- ABSORB-001 Open Items (D3/D4/D5) 해결 명시적 기록 (본 SPEC HISTORY)

---

## 7. Out-of-Plan Items (명시적 배제)

`spec.md` §Exclusions와 중복되지만 plan 수준 재확인:

- 실제 파일 삭제는 본 plan.md 생성 시점에서는 수행하지 않음 (plan 단계)
- `/agency` 서브커맨드 stub 8개는 본 SPEC이 건드리지 않음
- `.claude/rules/agency/constitution.md` REDIRECT stub 유지
- ABSORB-001 SPEC 문서 수정 금지
- v3.3 흡수 후 신규 스킬/에이전트 수정 금지

---

Version: 0.1.0
Companion: SPEC-AGENCY-ABSORB-001 (v1.0.1, completed) Open Items D3/D4/D5
