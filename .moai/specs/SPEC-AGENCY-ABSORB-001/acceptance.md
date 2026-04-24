---
spec_id: AGENCY-ABSORB-001
document: acceptance
version: 1.0.0
status: completed
created_at: 2026-04-20
updated_at: 2026-04-24
labels: []
---

# Acceptance Criteria — SPEC-AGENCY-ABSORB-001

본 문서는 spec.md의 41개 EARS 요구사항에 대한 검증 절차와 글로벌 수용 기준을 정의한다. characterization SPEC이므로 모든 검증은 **현재 디스크 상태**를 대상으로 수행되며, 통과 여부가 곧 흡수 작업의 성공을 의미한다.

---

## Global Acceptance Criteria

### AC-GLOBAL-1: /agency 명령 라우팅 일관성

**Given**: `.claude/commands/agency/` 아래 8개 deprecation stub이 존재한다.
**When**: 사용자가 `/agency`, `/agency brief`, `/agency build`, `/agency review`, `/agency profile`, `/agency resume`, `/agency learn`, `/agency evolve` 중 하나를 호출한다.
**Then**:
1. `Skill("moai")` 또는 `Skill("moai-workflow-research")`로 라우팅되어 silent fail 없이 실행된다.
2. Frontmatter `description`에 "(Deprecated)" 표기가 노출된다.
3. 본문에 SPEC-AGENCY-ABSORB-001 참조 라인이 포함된다.
4. `/agency learn` 또는 `/agency evolve` 호출 시 `AGENCY_SUBCOMMAND_UNSUPPORTED` 에러 코드 + 마이그레이션 가이드 URL 출력.

**Verification**:
```bash
# 8개 파일 존재 확인
ls .claude/commands/agency/*.md | wc -l   # expect: 8
# 모든 파일에 SPEC 참조 존재
grep -l "SPEC-AGENCY-ABSORB-001" .claude/commands/agency/*.md | wc -l   # expect: 8
# 모든 파일이 Skill() 라우팅으로 종료
grep -l "Use Skill(" .claude/commands/agency/*.md | wc -l   # expect: 8
# learn/evolve의 ERROR 코드
grep -l "AGENCY_SUBCOMMAND_UNSUPPORTED" .claude/commands/agency/{learn,evolve}.md | wc -l   # expect: 2
```

---

### AC-GLOBAL-2: /moai design Phase 0 → Phase 1 분기 작동

**Given**: `.claude/commands/moai/design.md` 및 `.claude/skills/moai/workflows/design.md`가 존재한다.
**When**: 사용자가 `/moai design`을 호출한다.
**Then**:
1. Phase 0 Check 1에서 `.agency/` 디렉터리 존재 여부 검사 (REQ-DETECT-003).
2. Phase 0 Check 2에서 `.moai/project/brand/` 3개 파일 존재 + `_TBD_` 마커 부재 검증 (REQ-ROUTE-001).
3. 검증 통과 시 Phase 1 진입, AskUserQuestion으로 두 옵션 제시 (REQ-ROUTE-002).
4. 옵션 1(Recommended): "Claude Design import" + 설명 + 구독 요건 명시.
5. 옵션 2: "Code-based brand design" + 설명 + visual-identity.md 요건 명시.
6. 사용자 미응답 시 최대 3회 재제시 후 안전 종료 (REQ-ROUTE-007).

**Verification**:
```bash
# Phase 0/1/A/B/C 구조 확인
grep -n "^## Phase" .claude/skills/moai/workflows/design.md
# AskUserQuestion 호출 명시 확인
grep "AskUserQuestion" .claude/skills/moai/workflows/design.md
# Subscription override 로직 확인 (REQ-ROUTE-006)
grep "subscription.tier" .claude/skills/moai/workflows/design.md
```

---

### AC-GLOBAL-3: .moai/design/ 자동 로딩 토큰 예산 준수

**Given**: `.moai/design/` 디렉터리에 spec.md, system.md, research.md, pencil-plan.md 중 하나 이상이 _TBD_ 외 콘텐츠를 포함한다.
**And**: `.moai/config/sections/design.yaml`에서 `design_docs.token_budget: 20000`이 설정되어 있다.
**When**: `/moai design` Phase B2.5에서 `moai-workflow-design-context` 스킬이 호출된다.
**Then**:
1. priority 순서(spec > system > research > pencil-plan)대로 파일을 읽는다 (REQ-DESIGN-DOCS-001).
2. _TBD_-only 파일은 스킵된다 (REQ-DESIGN-DOCS-003).
3. 토큰 추정 알고리즘 `ceiling(char_count / 4) * 1.10`으로 누적 계산.
4. 누적이 20000을 초과하면 reverse priority로 truncation: pencil-plan → research → system 순으로 drop. spec은 항상 보존.
5. 결과물은 Markdown 컨텍스트 블록으로 다음 subagent prompt에 prepend된다.

**Verification**:
```bash
# token_budget 설정 확인
grep "token_budget:" .moai/config/sections/design.yaml   # expect: 20000
# priority 배열 확인
grep -A5 "priority:" .moai/config/sections/design.yaml
# 스킬 본문의 truncation 로직
grep "Truncation order" .claude/skills/moai-workflow-design-context/SKILL.md
```

---

### AC-GLOBAL-4: FROZEN zone 보호 (Learner 자동 수정 차단)

**Given**: `.claude/rules/moai/design/constitution.md` Section 2에 FROZEN zone이 정의되어 있다.
**When**: Learner 또는 evolution 메커니즘이 다음 항목 수정을 시도한다:
- 헌법 파일 자체
- Section 3.1/3.2/3.3 본문
- Safety Architecture (Section 5)
- GAN Loop contract (Section 11)
- Evaluator leniency prevention (Section 12)
- Pipeline phase ordering
- Pass threshold floor (0.60)
- `require_approval` 설정
**Then**:
1. Frozen Guard (헌법 §5 Layer 1)이 write 작업을 차단한다.
2. 차단 로그가 기록되고 사용자에게 통지된다.
3. EVOLVABLE zone 항목(스킬 body, adaptation weights, 평가 rubric 등)은 정상 evolution 경로로 진행 가능.

**Verification**:
```bash
# FROZEN zone 항목 수
grep "^- \[FROZEN\]" .claude/rules/moai/design/constitution.md | wc -l   # expect: >= 9
# EVOLVABLE zone 항목 수
grep "^- \[EVOLVABLE\]" .claude/rules/moai/design/constitution.md | wc -l   # expect: >= 5
# Pass threshold floor 명시
grep "Pass threshold floor" .claude/rules/moai/design/constitution.md   # expect: 0.60
```

---

## Per-REQ Verification Matrix

### Category 1: Command Routing & Migration

| REQ-ID | Verification Method | Expected Outcome |
|--------|--------------------|--------------------|
| REQ-ABSORB-001 | `ls .claude/commands/agency/*.md \| wc -l` | 8 files |
| REQ-ABSORB-002 | `grep "Use Skill(\"moai\")" .claude/commands/agency/brief.md` | matches `plan $ARGUMENTS` |
| REQ-ABSORB-003 | `grep "AGENCY_SUBCOMMAND_UNSUPPORTED" .claude/commands/agency/{learn,evolve}.md` | matches both files |
| REQ-ABSORB-004 | `Read .claude/commands/moai/design.md` | exists, frontmatter description matches "Hybrid design workflow" |

### Category 2: Skill Absorption

| REQ-ID | Verification Method | Expected Outcome |
|--------|--------------------|--------------------|
| REQ-ABSORB-005 | `grep "Absorbed from agency-copywriting" .claude/skills/moai-domain-copywriting/SKILL.md` | line found, version v3.2.0 |
| REQ-ABSORB-006 | `grep "Absorbed from agency-design-system" .claude/skills/moai-domain-brand-design/SKILL.md` | line found, version v1.0.0 |
| REQ-ABSORB-007 | `grep "Absorbed from agency constitution Section 11" .claude/skills/moai-workflow-gan-loop/SKILL.md` | line found |
| REQ-ABSORB-007 (config) | `grep "All loop parameters are read from .moai/config/sections/design.yaml" .claude/skills/moai-workflow-gan-loop/SKILL.md` | line found |
| REQ-ABSORB-008 | `ls .claude/skills/moai-workflow-{design-context,design-import,pencil-integration}/SKILL.md` | 3 files exist |
| REQ-ABSORB-009 | `ls .claude/agents/agency/{copywriter,designer}.md` | both files exist (deprecation window) |

### Category 3: Constitution Migration

| REQ-ID | Verification Method | Expected Outcome |
|--------|--------------------|--------------------|
| REQ-CONST-001 | `grep "Relocated from" .claude/rules/moai/design/constitution.md` | HISTORY entry found, mentions verbatim preservation |
| REQ-CONST-002 | `grep "tripartite structure (3.1/3.2/3.3)" .claude/rules/moai/design/constitution.md` | line found in HISTORY |
| REQ-CONST-002 (subsections) | `grep -E "^### 3\.(1\|2\|3)" .claude/rules/moai/design/constitution.md` | 3 subsections present |
| REQ-CONST-003 | `wc -l .claude/rules/agency/constitution.md` | <= 25 lines (stub) |
| REQ-CONST-003 (redirect) | `grep "REDIRECT" .claude/rules/agency/constitution.md` | redirect notice present |
| REQ-CONST-004 | `grep -c "^- \[HARD\]" .claude/rules/moai/design/constitution.md \| head -1` | section 3.1 has >= 5 [HARD] rules |

### Category 4: Pipeline & Workflow

| REQ-ID | Verification Method | Expected Outcome |
|--------|--------------------|--------------------|
| REQ-ROUTE-001 | `grep "Brand context is incomplete" .claude/skills/moai/workflows/design.md` | brand interview proposal present |
| REQ-ROUTE-002 | `grep -A10 "## Phase 1" .claude/skills/moai/workflows/design.md` | two options listed |
| REQ-ROUTE-003 | `grep "Recommended.*Claude Design import" .claude/skills/moai/workflows/design.md` | option 1 marked Recommended |
| REQ-ROUTE-004 | `grep "Phase A: Claude Design Import" .claude/skills/moai/workflows/design.md` | Phase A defined |
| REQ-ROUTE-005 | `grep "Phase B: Code-Based Design" .claude/skills/moai/workflows/design.md` | Phase B defined |
| REQ-ROUTE-006 | `grep "Subscription override" .claude/skills/moai/workflows/design.md` | reversal logic present |
| REQ-ROUTE-007 | `grep "No-response handling" .claude/skills/moai/workflows/design.md` | 3-attempt rule present |
| REQ-ROUTE-008 | `grep "moai-workflow-gan-loop" .claude/skills/moai/workflows/design.md` | Phase C invokes GAN loop |
| REQ-FALLBACK-001 | `grep "switch to path B" .claude/skills/moai/workflows/design.md` | path A failure offers path B |
| REQ-FALLBACK-002 | `grep "Do NOT abort the overall.*design" .claude/skills/moai/workflows/design.md` | Pencil error continues to B3 |
| REQ-FALLBACK-003 | `grep "_TBD_ markers" .claude/skills/moai-domain-brand-design/SKILL.md` | brand interview gate present |
| REQ-BRIEF-001 | `grep -E "^## (Goal\|Audience\|Brand)" .claude/skills/moai/workflows/design.md` | 3 required sections defined in BRIEF template |
| REQ-BRIEF-002 | `grep "auto-inject key content" .claude/skills/moai/workflows/design.md` | auto-injection logic present |
| REQ-BRIEF-003 | `grep "BRIEF_SECTION_INCOMPLETE" .claude/skills/moai/workflows/design.md` | error code defined |
| REQ-DETECT-003 | `grep ".agency/ detection" .claude/skills/moai/workflows/design.md` | Phase 0 Check 1 present |

### Category 5: Configuration & Brief Directory

| REQ-ID | Verification Method | Expected Outcome |
|--------|--------------------|--------------------|
| REQ-CONFIG-001 | `Read .moai/config/sections/design.yaml` | all 8 sections (gan_loop, evolution, adaptation, brand_context, design_docs, claude_design, figma, default_framework) present |
| REQ-CONFIG-002 | `grep "pass_threshold: 0.75" .moai/config/sections/design.yaml` | line found |
| REQ-CONFIG-002 (no hardcode) | `grep "Do not hardcode thresholds" .claude/skills/moai-workflow-gan-loop/SKILL.md` | line found |
| REQ-CONFIG-003 | `grep -A3 "sprint_contract:" .moai/config/sections/design.yaml` | required_harness_levels and optional_harness_levels present |
| REQ-DESIGN-DOCS-001 | `grep "Auto-load priority" .moai/design/README.md` | spec > system > research > pencil-plan order |
| REQ-DESIGN-DOCS-002 | `grep "Reserved Filenames" .moai/design/README.md` | tokens.json, components.json, import-warnings.json, brief/BRIEF-*.md listed |
| REQ-DESIGN-DOCS-003 | `grep -A5 "Token Budget Enforcement" .claude/skills/moai-workflow-design-context/SKILL.md` | reverse truncation algorithm documented |

### Category 6: Deprecation Policy

| REQ-ID | Verification Method | Expected Outcome |
|--------|--------------------|--------------------|
| REQ-DEPRECATE-001 | `grep -l "DEPRECATED" .claude/commands/agency/*.md \| wc -l` | 8 |
| REQ-DEPRECATE-002 | `grep "DEPRECATED.*SPEC-AGENCY-ABSORB-001" CLAUDE.md` | line found in §3 |
| REQ-DEPRECATE-003 | `grep "next minor version" .claude/commands/agency/agency.md` | line found |
| REQ-DEPRECATE-003 (stub) | `grep "2 minor version cycles" .claude/rules/agency/constitution.md` | window noted |
| REQ-DEPRECATE-004 | `grep "AGENCY_SUBCOMMAND_UNSUPPORTED" .claude/commands/agency/{learn,evolve}.md` | both files contain error code + URL |

### Category 7: Documentation Sync

| REQ-ID | Verification Method | Expected Outcome |
|--------|--------------------|--------------------|
| REQ-DOC-001 | `grep "Agency Agents (2)" CLAUDE.md` | catalog updated |
| REQ-DOC-001 (planner removal) | `grep "planner, builder, evaluator, learner removed" CLAUDE.md` | line found |
| REQ-DOC-002 | `grep "Design System Configuration (absorbed from agency" CLAUDE.md` | §9 section present |
| REQ-DOC-003 | `grep "Legacy .agency/ directories are archived" CLAUDE.md` | migration note present |

---

## Edge Cases & Negative Tests

### Edge Case 1: 빈 brand 디렉터리

**Given**: `.moai/project/brand/` 디렉터리가 존재하나 모든 파일이 _TBD_-only 상태이다.
**When**: `/moai design`이 호출된다.
**Then**:
1. Phase 0 Check 2에서 incomplete 판정.
2. Route selection 스킵.
3. brand interview 자동 제안.
4. 사용자가 interview 거부 시 `/moai design` 안전 종료 (irreversible 작업 전).

**Verification**: `.moai/project/brand/brand-voice.md` 현 상태(_TBD_ 마커 포함)에서 워크플로우 시뮬레이션 시 interview 트리거 확인.

---

### Edge Case 2: design.yaml 누락

**Given**: `.moai/config/sections/design.yaml` 파일이 부재한다.
**When**: GAN Loop 또는 design context 스킬이 호출된다.
**Then**:
1. `moai-workflow-design-context`는 compiled-in defaults 사용 + 경고 로그 (REQ-DESIGN-DOCS-003 default fallback).
2. `moai-workflow-gan-loop`는 임계값을 읽지 못해 default 또는 에러 (REQ-CONFIG-002 위반).
3. 사용자에게 `moai init` 또는 design.yaml 복원 안내.

---

### Edge Case 3: Pencil MCP 부재

**Given**: Pencil MCP 서버가 설치되지 않았고, `.moai/design/pencil-plan.md`도 존재하지 않는다.
**When**: `/moai design` Phase B2.6에 도달한다.
**Then**:
1. Precondition 검사 실패 → graceful skip (REQ-PENCIL-002).
2. 사용자 가시 에러 없음.
3. Phase B3로 자동 진행.

**Verification**: `.moai/design/`에 .pen 파일 부재 + pencil-plan.md 부재 시 정상 흐름.

---

### Edge Case 4: 깨진 Claude Design 번들

**Given**: 사용자가 path A 선택 후 손상된 또는 지원되지 않는 형식의 번들 경로를 제공한다.
**When**: `moai-workflow-design-import`가 파싱을 시도한다.
**Then**:
1. 구조화된 에러 코드(`DESIGN_IMPORT_INVALID_BUNDLE` 또는 `DESIGN_IMPORT_UNSUPPORTED_FORMAT`) 반환.
2. AskUserQuestion으로 path B 전환 옵션 제시 (REQ-FALLBACK-001).
3. 사용자가 거부 시 워크플로우 정지, 정정된 번들 입력 대기.

---

### Edge Case 5: 동시 SPEC 작성 vs 본 SPEC

**Given**: SPEC-DB-SYNC-RELOC-001이 동시기에 진행되었으며 일부 인프라(예: `.moai/config/sections/`)를 공유한다.
**When**: 양 SPEC이 모두 design.yaml과 db.yaml을 신설한다.
**Then**:
1. design.yaml과 db.yaml은 별개 파일이므로 충돌 없음.
2. 본 SPEC의 Out of Scope 명시(Non-Goals)로 책임 경계가 명확.
3. 두 SPEC의 acceptance가 독립적으로 검증 가능.

---

## Definition of Done

본 SPEC의 "완료(completed)" 정의:

1. **모든 41개 REQ가 검증 가능**: spec.md의 각 REQ에 대응하는 코드/문서 위치가 grep/Read로 확인된다.
2. **4개 글로벌 AC 통과**: AC-GLOBAL-1 ~ AC-GLOBAL-4가 모두 위 검증 명령으로 통과.
3. **3-파일 SPEC 구조 완비**: `.moai/specs/SPEC-AGENCY-ABSORB-001/{spec,plan,acceptance}.md` 모두 존재.
4. **YAML frontmatter 유효**: 각 파일의 frontmatter가 metadata schema 준수 (id, version, status, dates, author 등).
5. **REQ 추적성**: 모든 REQ-ID가 적어도 한 개의 외부 파일(스킬·커맨드·헌법·CLAUDE.md)에 인용된다.
6. **Open Items 등록**: spec.md "확인 필요" 4건이 후속 SPEC의 input으로 명시되어 있다.
7. **회귀 비교 가능**: 본 SPEC을 baseline으로 향후 SPEC-AGENCY-CLEANUP-002가 수행할 변경의 차이가 정량화 가능하다.

본 작성 시점(2026-04-24)에 위 7개 조건이 모두 만족되며, status: completed로 기록한다.

---

## Quality Gate Checklist

- [x] EARS 형식 준수 (WHEN/WHILE/IF/THE SHALL 키워드 사용)
- [x] 35개 REQ 모두 증거 파일 인용 포함
- [x] Exclusions 섹션 존재 (spec.md "What NOT to Build")
- [x] 3-파일 구조 (spec.md / plan.md / acceptance.md)
- [x] Frontmatter 유효 (id, version, status, dates)
- [x] HISTORY 섹션 존재 (spec.md)
- [x] Non-Goals/Out of Scope 명시
- [x] Open Items / 확인 필요 항목 명시
- [x] Verification 명령 제공 (grep/Read 명령으로 즉시 실행 가능)
- [x] characterization SPEC 패턴 준수 (신규 코드 작성 없음)

---

## Cross-References

- 외부 SPEC 의존:
  - SPEC-THIN-CMDS-001 (thin command 패턴, 선행)
  - SPEC-DB-SYNC-RELOC-001 (병렬 트랙, 본 SPEC와 독립)
- 본 SPEC이 참조만 하고 재정의하지 않는 REQ:
  - REQ-PENCIL-001 ~ REQ-PENCIL-016 (Pencil 통합 스킬 자체의 SPEC)
  - REQ-SKILL-* (각 흡수 스킬 자체 SPEC의 요구사항)
- 후속 권고 SPEC:
  - SPEC-AGENCY-CLEANUP-002 (deprecation window 만료 시 물리적 삭제)
  - SPEC-BRAND-ONBOARDING-001 (brand interview 자동 트리거)
  - SPEC-AGENCY-LEARN-EVOLVE-MIGRATION (learn/evolve 정식 등가물 정의)
