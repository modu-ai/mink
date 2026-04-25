---
spec_id: SPEC-AGENCY-CLEANUP-002
version: 0.1.0
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
test_format: Given-When-Then
---

# Acceptance Criteria: SPEC-AGENCY-CLEANUP-002

본 문서는 SPEC-AGENCY-CLEANUP-002의 acceptance 시나리오를 Given-When-Then 형식으로 규정한다. 본 SPEC은 파일시스템 상태 검증 중심이므로 실제 테스트는 Bash 명령 관찰로 수행한다.

---

## 1. AC-CL-001: Legacy Manifest 생성 검증

**Requirement**: REQ-CL-001 (Ubiquitous)

### Scenario 1.1: Manifest 파일 존재

- **Given**: SPEC-AGENCY-ABSORB-001의 status가 `completed`이고 legacy 파일 19개가 파일시스템에 존재한다
- **When**: `/moai run SPEC-AGENCY-CLEANUP-002`가 Milestone CL-M1을 완료한다
- **Then**: 다음이 모두 참이어야 한다
  - `.moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml` 파일이 존재
  - YAML root가 `artifacts:` 배열을 포함
  - 배열 항목 수가 정확히 19

### Scenario 1.2: Manifest 필드 완전성

- **Given**: legacy-manifest.yaml이 생성되었다
- **When**: 각 artifact entry를 검사한다
- **Then**: 모든 entry가 다음 필드를 가져야 한다
  - `path`: 프로젝트 루트 기준 상대경로 (예: `.agency/config.yaml`)
  - `type`: "file" 또는 "dir"
  - `size_bytes`: 정수
  - `sha256`: 64자 hex 문자열
  - `action`: "delete" 또는 "archive"
  - `status`: "pending" (run 완료 전), "done" 또는 "failed" (run 완료 후)

### Scenario 1.3: 파일 카테고리 분포

- **Given**: legacy-manifest.yaml의 artifacts 배열
- **When**: path prefix로 그룹화
- **Then**:
  - `.agency/` 시작 entry: 정확히 8개
  - `.claude/skills/agency-` 시작 entry: 5개 디렉터리(각 SKILL.md 1개 + 내부 asset) — 최소 5 entry
  - `.claude/agents/agency/` 시작 entry: 정확히 6개

**검증 명령 예시**:

```bash
# Scenario 1.1
test -f .moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml
yq '.artifacts | length' .moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml
# 기대: 19 (또는 디렉터리 재귀 시 더 많을 수 있음)

# Scenario 1.3
yq '.artifacts[] | select(.path | startswith(".agency/")) | .path' .moai/specs/SPEC-AGENCY-CLEANUP-002/legacy-manifest.yaml | wc -l
# 기대: 8
```

---

## 2. AC-CL-002: Archive 우선 수행 검증

**Requirement**: REQ-CL-002 (Event-Driven)

### Scenario 2.1: Archive 디렉터리 존재

- **Given**: 사용자가 cleanup 실행을 standard mode로 승인했다
- **When**: Milestone CL-M2 완료 시점
- **Then**: `.archive/agency-legacy-{YYYYMMDD}/` 디렉터리가 존재하고 하위에 원본과 동일한 디렉터리 구조가 유지된다

### Scenario 2.2: sha256 일치

- **Given**: archive 디렉터리가 생성되었다
- **When**: 각 복사본의 sha256을 원본 manifest와 비교한다
- **Then**: 모든 artifact의 sha256이 정확히 일치해야 한다 (0 mismatch)

### Scenario 2.3: 원자성 — 실패 시 원본 보존

- **Given**: archive 단계에서 고의로 복사 실패를 유도 (예: `.archive/` 디스크 공간 부족 시뮬레이션)
- **When**: 시스템이 실패를 감지한다
- **Then**:
  - `.archive/agency-legacy-{YYYYMMDD}/`는 rollback되거나 비어있음
  - 19개 원본 파일은 그대로 존재 (한 개도 제거되지 않음)
  - 에러 메시지에 "archive verification failed" 포함

**검증 명령 예시**:

```bash
# Scenario 2.1
DATE=$(date +%Y%m%d)
test -d .archive/agency-legacy-${DATE}
test -f .archive/agency-legacy-${DATE}/.agency/config.yaml
test -d .archive/agency-legacy-${DATE}/.claude/skills/agency-copywriting

# Scenario 2.2
for f in $(yq '.artifacts[] | select(.type == "file") | .path' legacy-manifest.yaml); do
  original_sha=$(yq ".artifacts[] | select(.path == \"$f\") | .sha256" legacy-manifest.yaml)
  archive_sha=$(shasum -a 256 .archive/agency-legacy-${DATE}/$f | cut -d' ' -f1)
  [ "$original_sha" = "$archive_sha" ] || echo "MISMATCH: $f"
done
```

---

## 3. AC-CL-003: Import 파괴 감지 시 중단

**Requirement**: REQ-CL-003 (Unwanted Behavior)

### Scenario 3.1: 위반 발견 시 halt

- **Given**: 가상의 파일 `.claude/rules/moai/some-rule.md`에 `agency-copywriting/SKILL.md`을 참조하는 문자열이 존재한다
- **When**: Milestone CL-M1 import 검사가 수행된다
- **Then**:
  - 시스템이 cleanup을 중단한다
  - `.archive/` 디렉터리가 생성되지 않는다
  - 19개 원본 파일이 모두 그대로 유지된다
  - blocker 리포트가 stdout에 출력된다: "Import violation detected: {파일:라인} references legacy artifact"

### Scenario 3.2: 허용 예외는 halt 트리거 안 함

- **Given**: SPEC-AGENCY-ABSORB-001 자체 문서에 "agency-copywriting" 문자열이 존재한다 (역사적 참조)
- **When**: import 검사가 수행된다
- **Then**: 해당 참조는 위반으로 카운트되지 않고 cleanup이 정상 진행된다 (예외 목록: `.moai/specs/SPEC-AGENCY-ABSORB-001/`, `.moai/specs/SPEC-AGENCY-CLEANUP-002/`, `.claude/rules/agency/constitution.md`)

### Scenario 3.3: 모든 검색 패턴 커버리지

- **Given**: 시스템에 3가지 패턴 violation이 각각 1개씩 삽입되어 있다
  - 패턴 1: `agency-frontend-patterns`
  - 패턴 2: `.claude/agents/agency/planner.md`
  - 패턴 3: `.agency/templates/brief-template.md`
- **When**: import 검사가 수행된다
- **Then**: 3개 violation 모두 리포트되고 cleanup은 중단된다

**검증 명령 예시**:

```bash
# Scenario 3.1 재현
echo "See agency-copywriting/SKILL.md for details" >> .claude/rules/moai/test-violation.md
# Run SPEC-AGENCY-CLEANUP-002 → 기대: halt with blocker
rm .claude/rules/moai/test-violation.md  # cleanup after test

# Scenario 3.2 허용 예외 확인
rg -l "agency-copywriting" .moai/specs/SPEC-AGENCY-ABSORB-001/
# 기대: 최소 1개 히트 — 그러나 violation으로 처리되지 않음
```

---

## 4. AC-CL-004: 단일 커밋 롤백 검증

**Requirement**: REQ-CL-004 (State-Driven)

### Scenario 4.1: 단일 commit 작성

- **Given**: Milestone CL-M3가 완료되어 원본이 제거되었다
- **When**: Milestone CL-M4 commit이 수행된다
- **Then**:
  - `git log -1` 출력이 `chore(cleanup): SPEC-AGENCY-CLEANUP-002` 패턴에 매치
  - commit message trailer에 `SPEC: SPEC-AGENCY-CLEANUP-002` 포함
  - commit message trailer에 `REQ: REQ-CL-001, REQ-CL-002, REQ-CL-003, REQ-CL-004, REQ-CL-005` 포함
  - `git show --stat HEAD`에 19개 artifact 제거 + archive 파일 추가가 **하나의 commit**에 모두 반영

### Scenario 4.2: Rollback 성공

- **Given**: cleanup commit이 HEAD에 있다
- **When**: `git reset --hard HEAD~1`이 실행된다
- **Then**:
  - 19개 legacy 파일이 원위치에 복원됨 (file existence check)
  - 모든 복원 파일의 sha256이 manifest 원본 값과 일치
  - `.archive/agency-legacy-{YYYYMMDD}/` 디렉터리가 제거됨 (git reset으로 untracked 상태가 되지 않음 — archive도 tracked이므로 함께 제거)

### Scenario 4.3: Archive 파일도 같은 commit에 포함 (원자성)

- **Given**: commit이 작성되었다
- **When**: `git show --name-only HEAD` 실행
- **Then**: 출력에 (a) 제거된 legacy 파일 + (b) `.archive/agency-legacy-{DATE}/` 하위 추가 파일이 **모두 포함**된다 (별도 commit 분할되지 않음)

**검증 명령 예시**:

```bash
# Scenario 4.1
git log -1 --pretty=%B | grep -q "SPEC-AGENCY-CLEANUP-002"
git log -1 --pretty=%B | grep -q "REQ: REQ-CL-001"

# Scenario 4.2 (dry-run branch)
git checkout -b rollback-test/CL-002-$(date +%Y%m%d)
git reset --hard HEAD~1
test -f .agency/config.yaml && echo "restored OK"
test -f .claude/agents/agency/planner.md && echo "restored OK"
git checkout main
git branch -D rollback-test/CL-002-$(date +%Y%m%d)  # cleanup

# Scenario 4.3
git show --name-only HEAD | grep -c "^\.archive/" # > 0
git show --name-only HEAD | grep -c "^\.agency/" # > 0 (as deletions)
```

---

## 5. AC-CL-005: Archive-only 모드 경로 보존 검증

**Requirement**: REQ-CL-005 (Optional)

### Scenario 5.1: `--archive-only` 플래그 수용

- **Given**: 사용자가 `/moai run SPEC-AGENCY-CLEANUP-002 --archive-only`를 실행한다
- **When**: cleanup이 수행된다
- **Then**:
  - 원본 위치에서 legacy 파일이 제거됨
  - `.archive/agency-legacy-{YYYYMMDD}/` 하위에 파일이 존재
  - git은 delete + add가 아닌 **rename**으로 인식

### Scenario 5.2: `git log --follow`로 rename 추적

- **Given**: archive-only 모드 commit이 HEAD에 있다
- **When**: `git log --follow .archive/agency-legacy-{DATE}/.agency/config.yaml` 실행
- **Then**:
  - log 출력에 원본 경로 `.agency/config.yaml`의 이력이 포함됨
  - rename 이력이 시각적으로 연결 가능

### Scenario 5.3: Standard mode와 archive-only mode 구분

- **Given**: 동일 SPEC이 두 모드로 실행 가능
- **When**: 모드 플래그가 없는 경우
- **Then**: 기본 동작은 standard mode (git rm) — archive-only는 opt-in

**검증 명령 예시**:

```bash
# Scenario 5.1 (archive-only 실행 후)
DATE=$(date +%Y%m%d)
test ! -f .agency/config.yaml   # 원본 제거 확인
test -f .archive/agency-legacy-${DATE}/.agency/config.yaml  # archive 존재

# Scenario 5.2
git log --follow --oneline .archive/agency-legacy-${DATE}/.agency/config.yaml
# 기대 출력에 rename 이전 commit 포함
```

---

## 6. Edge Cases

### EC-1: `.archive/` 가 `.gitignore`에 포함된 경우

- **Given**: `.gitignore`에 `.archive/` 라인이 있다
- **When**: cleanup 실행
- **Then**: `.archive/agency-legacy-{DATE}/`가 git에 tracked되지 않아 commit에 포함되지 않음 → REQ-CL-004 원자성 위반
- **기대 동작**: manager-git에 위임하여 `!.archive/agency-legacy-*` 예외 규칙 추가 권고 (run 단계에서 자동 감지)

### EC-2: 디렉터리 권한 문제

- **Given**: `.claude/agents/agency/planner.md`의 권한이 읽기 전용(0444)
- **When**: git rm 실행
- **Then**: git rm은 권한에 상관없이 성공. 그러나 rsync 복사 시 권한 보존 필요 (`rsync -a` 사용)

### EC-3: YYYYMMDD 충돌

- **Given**: 같은 날 두 번째 cleanup 시도 (실수로 인한 재실행)
- **When**: `.archive/agency-legacy-{DATE}/` 가 이미 존재
- **Then**: cleanup halt — user에게 "archive already exists, abort or rename?" 옵션 제시 (AskUserQuestion — 단, 이는 orchestrator 책임)

### EC-4: manifest와 실제 파일 불일치

- **Given**: CL-M1 완료 후 M2 진입 전에 사용자가 수동으로 `.agency/config.yaml` 수정
- **When**: CL-M2 sha256 검증
- **Then**: 불일치 감지 → halt + "manifest stale, re-run CL-M1" 권고

---

## 7. Definition of Done

본 SPEC이 `completed` 상태로 전환되기 위해 모두 충족되어야 할 조건:

- [ ] `AC-CL-001` 모든 scenario (1.1, 1.2, 1.3) pass
- [ ] `AC-CL-002` 모든 scenario (2.1, 2.2, 2.3) pass
- [ ] `AC-CL-003` 모든 scenario (3.1, 3.2, 3.3) pass
- [ ] `AC-CL-004` 모든 scenario (4.1, 4.2, 4.3) pass
- [ ] `AC-CL-005` 모든 scenario (5.1, 5.2, 5.3) pass (archive-only 모드 사용 시)
- [ ] edge case EC-1~EC-4 중 관찰된 경우 처리 완료
- [ ] 회귀 smoke test: `/moai design` path A/B 동작 확인
- [ ] `spec.md` HISTORY에 완료 엔트리 추가 (v0.1.0 → v0.1.1)
- [ ] 사용자에게 완료 리포트 제출 (제거 파일 수, archive 경로, rollback 명령, smoke test 결과)

---

## 8. Quality Gate Checklist (Plan 단계)

본 acceptance.md 문서 자체의 완성도 기준:

- [x] 5개 REQ 모두 AC 섹션 매핑 (REQ-CL-001 → AC-CL-001, ..., REQ-CL-005 → AC-CL-005)
- [x] 각 AC가 최소 2개 scenario 포함 (AC-CL-001: 3개, AC-CL-002: 3개, AC-CL-003: 3개, AC-CL-004: 3개, AC-CL-005: 3개)
- [x] Given-When-Then 형식 명시적 사용
- [x] 검증 명령 예시 제공 (bash one-liner 기반)
- [x] Edge case 섹션 최소 3개 (EC-1~EC-4)
- [x] Definition of Done 체크리스트 제공

---

Version: 0.1.0
Companion: spec.md §5, plan.md §3
REQ coverage: REQ-CL-001, REQ-CL-002, REQ-CL-003, REQ-CL-004, REQ-CL-005
AC coverage: AC-CL-001, AC-CL-002, AC-CL-003, AC-CL-004, AC-CL-005
