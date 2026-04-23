---
spec_id: AGENCY-ABSORB-001
document: plan
version: 1.0.0
status: completed
created: 2026-04-20
updated: 2026-04-24
---

# Implementation Plan — SPEC-AGENCY-ABSORB-001

본 문서는 회고적(characterization) 이행 계획이다. 모든 milestone은 이미 commit·deploy되었으며, 본 plan.md는 "어떤 순서로 무엇이 변경되었는가"를 기록하여 향후 audit·rollback 시 참조 자료로 활용한다.

---

## Overall Strategy

흡수 작업은 다음 5단계 순차 진행을 기본으로 하되, M0(GOOSE 브랜드 파운데이션)는 선행 트랙으로 외부에서 완료된 입력으로 간주했다.

```
M0 (선행 트랙) → M1 (헌법 이전) → M2 (스킬 신설) → M3 (워크플로우 통합) → M4 (설정 통합) → M5 (deprecation)
                                                                                              ↓
                                                                              M6 (sync — 본 SPEC 작성, 회고)
```

핵심 원칙:

- **Verbatim 이전 우선**: 헌법 본문은 의미론적 변경 없이 위치만 이동 (M1 핵심).
- **흡수는 신규 파일로**: 구 스킬을 직접 수정하지 않고 신규 `moai-domain-*`, `moai-workflow-*` 트리에 새로 작성하여 충돌 위험 제거 (M2).
- **Deprecation window 보장**: 즉시 삭제 없이 redirect stub 유지로 외부 참조 호환성 보존 (M5, REQ-DEPRECATE-003).
- **Configuration 외부화**: 모든 임계값을 design.yaml로 추출하여 스킬 본문에서 하드코딩 제거 (M4, REQ-CONFIG-002).

---

## M0 — GOOSE 브랜드 파운데이션 수립 (선행 트랙)

**Status**: 완료 (커밋 d02f512 — 본 SPEC 외부)

**목적**: `.moai/project/brand/` 디렉터리 구조와 GOOSE 브랜드 페르소나 정립. 본 SPEC의 입력 조건이며, 결과물은 아니다.

**Deliverables (입력으로 전제)**:
- `.moai/project/brand/brand-voice.md` 스캐폴드
- `.moai/project/brand/visual-identity.md` 스캐폴드
- `.moai/project/brand/target-audience.md` 스캐폴드

**왜 별도 트랙인가**: 브랜드 정의는 사용자 의사결정 영역이며, agency 흡수의 기술적 작업과 분리되어야 변경 조정이 용이하다.

**본 SPEC과의 인터페이스**: REQ-ROUTE-001, REQ-CONST-004, REQ-BRIEF-002가 이 디렉터리 구조에 의존한다.

---

## M1 — 헌법 이전 (Constitution Migration)

**Status**: 완료 (2026-04-20)

**Priority**: High (모든 후속 milestone의 거버넌스 baseline)

**범위 / 변경 파일**:

| 파일 | 변경 유형 | 비고 |
|------|----------|------|
| `.claude/rules/moai/design/constitution.md` | 신규 (404줄) | v3.3.0, agency v3.2.0 verbatim + Section 3 tripartite 확장 |
| `.claude/rules/agency/constitution.md` | 축소 → redirect stub | 21줄로 축소, REQ-DEPRECATE-003 deprecation window 명시 |

**기술적 접근**:
1. 원본 v3.2.0 본문을 신 위치로 복사.
2. Section 3을 3.1 (Brand Context, constitutional parent), 3.2 (Design Brief, execution scope), 3.3 (Relationship)로 분할하여 SPEC-DESIGN-CONST-AMEND-001 의도 반영.
3. FROZEN zone에 각 subsection을 개별 항목으로 등재 (Section 2 항목 추가).
4. HISTORY 섹션에 이전 사실과 SPEC 참조 기록.
5. 원본 파일은 21줄 redirect stub으로 교체 (외부 참조 보존).

**검증**: REQ-CONST-001~004 (verbatim 이전 + tripartite 확장 + redirect stub + 브랜드 [HARD] 룰).

**Risks Mitigated**:
- 외부 도구가 `.claude/rules/agency/constitution.md`를 참조해도 redirect stub이 새 위치를 안내한다.
- v3.2.0 → v3.3.0 버전 bump가 호환성 변경(Section 3 분할)을 명확히 신호한다.

---

## M2 — Design 스킬 신설 (Skill Absorption)

**Status**: 완료 (2026-04-20)

**Priority**: High (M3 워크플로우의 의존 스킬)

**범위 / 변경 파일**:

| 신규 스킬 | 흡수 출처 | 분류 | user-invocable |
|-----------|---------|------|----------------|
| `moai-domain-brand-design/SKILL.md` | agency-design-system v1.0.0 | domain | true |
| `moai-domain-copywriting/SKILL.md` | agency-copywriting v3.2.0 | domain | true |
| `moai-workflow-design-context/SKILL.md` | (신규) | workflow | false |
| `moai-workflow-design-import/SKILL.md` | (신규) | workflow | false |
| `moai-workflow-gan-loop/SKILL.md` | agency constitution §11/§12 | workflow | false |
| `moai-workflow-pencil-integration/SKILL.md` | (신규) | workflow | false |

**기술적 접근**:
1. **흡수 패턴**: 구 agency 스킬의 본문을 새 스킬에 옮기되, frontmatter는 MoAI 표준(YAML folded scalar description, metadata 문자열, progressive_disclosure 블록, triggers 블록)으로 재작성.
2. **REQ 참조 삽입**: 각 스킬 본문 단계마다 본 SPEC의 REQ-ID를 인라인 명시 (예: GAN Loop SKILL.md "REQ coverage: REQ-SKILL-011, ... REQ-CONST-004").
3. **하드코딩 제거**: GAN Loop 임계값(`pass_threshold`, `max_iterations` 등)을 모두 design.yaml에서 읽도록 분리 (REQ-CONFIG-002).
4. **Pencil은 옵션**: `moai-workflow-pencil-integration`은 phase B2.6 precondition 미충족 시 graceful skip (REQ-PENCIL-002).

**검증**: REQ-ABSORB-005~008.

**Risks Mitigated**:
- 구 agency 스킬은 보존(`.claude/skills/agency-design-system/` 잔존)하여 흡수 직후 회귀 비교 가능.
- copywriter/designer agent 정의는 fallback 참조용으로 유지 (REQ-ABSORB-009).

---

## M3 — 명령어·워크플로우 통합 (Routing Integration)

**Status**: 완료 (2026-04-20)

**Priority**: High (사용자 진입점)

**범위 / 변경 파일**:

| 파일 | 변경 유형 | 비고 |
|------|----------|------|
| `.claude/commands/moai/design.md` | 신규 (8줄) | Thin command wrapper, `Skill("moai")` 라우팅 |
| `.claude/skills/moai/workflows/design.md` | 신규 (220줄) | Phase 0/1/A/B/C 전체 로직 |

**기술적 접근**:
1. **Thin command 패턴 준수** (SPEC-THIN-CMDS-001): `.claude/commands/moai/design.md`는 frontmatter + `Use Skill("moai") with arguments: design $ARGUMENTS` 한 줄.
2. **워크플로우 본문 위치**: 모든 라우팅·검증 로직은 `.claude/skills/moai/workflows/design.md`에 작성.
3. **Phase 정의**:
   - Phase 0: Pre-flight (agency detection, brand context check) — REQ-DETECT-003, REQ-ROUTE-001.
   - Phase 1: Route selection via AskUserQuestion — REQ-ROUTE-002~007.
   - Phase A: Claude Design import — REQ-ROUTE-004, REQ-FALLBACK-001.
   - Phase B: Code-based design (B1 skills load → B2.5 design context → B2.6 Pencil → B3 BRIEF → B4 expert-frontend) — REQ-ROUTE-005, REQ-PENCIL-001~003.
   - Phase C: Quality gate via GAN Loop — REQ-ROUTE-008.
4. **Fallback 명시**: Phase A 실패 시 path B로 폴백 옵션 제시, Pencil 오류는 Phase B 내부에서 흡수 (REQ-FALLBACK-001~002).

**검증**: REQ-ROUTE-001~008, REQ-FALLBACK-001~002, REQ-BRIEF-001~003, REQ-DETECT-003.

---

## M4 — 설정 통합 (Configuration Centralization)

**Status**: 완료 (2026-04-20)

**Priority**: Medium (M2/M3가 참조하는 파라미터 source of truth)

**범위 / 변경 파일**:

| 파일 | 변경 유형 | 비고 |
|------|----------|------|
| `.moai/config/sections/design.yaml` | 신규 (57줄) | gan_loop, evolution, adaptation, brand_context, design_docs, claude_design, figma 섹션 |
| `.moai/design/README.md` | 신규 | Auto-load 우선순위, reserved filename, update policy |
| `.moai/design/research.md` | 신규 (스캐폴드) | _TBD_ 마커 — human research 입력 대기 |
| `.moai/design/spec.md` | 신규 (스캐폴드) | _TBD_ 마커 — IA·frame spec 입력 대기 |
| `.moai/design/system.md` | 수정 | design tokens·typography·spacing 섹션 (스캐폴드) |
| `.moai/design/wireframes/`, `.moai/design/screenshots/` | 디렉터리 신규 | human reference 자료 보관소 |

**기술적 접근**:
1. **모든 임계값 외부화**: `gan_loop.pass_threshold` 0.75, `max_iterations` 5, `evolution.max_evolution_rate_per_week` 3 등.
2. **Brand context 디렉터리 명시**: `brand_context.dir: .moai/project/brand` + `interview_on_first_run: true`.
3. **Auto-load 정책**: `design_docs.auto_load_on_design_command: true`, `priority: [spec, system, research, pencil-plan]`, `token_budget: 20000`.
4. **Claude Design 활성화**: `claude_design.enabled: true`, `fallback_path: code_based`, `supported_bundle_versions: ["1.0"]`.
5. **Sprint Contract**: `sprint_contract.required_harness_levels: [thorough]`, `optional_harness_levels: [standard]`, `artifact_dir: .moai/sprints`.

**검증**: REQ-CONFIG-001~003, REQ-DESIGN-DOCS-001~003.

**Risks Mitigated**:
- 임계값을 단일 파일로 모아 추후 evolution 시 canary check 대상이 명확해진다 (헌법 §5 Layer 2).
- README의 reserved filename 표가 자동생성 산출물과 사용자 파일의 충돌을 사전 방지한다.

---

## M5 — /agency Deprecation (Migration Cleanup)

**Status**: 완료 (2026-04-20 ~ 2026-04-23)

**Priority**: Medium (사용자 호환성)

**범위 / 변경 파일**:

| 파일 | 변경 유형 | 비고 |
|------|----------|------|
| `.claude/commands/agency/agency.md` | 축소 → redirect + migration table | 27줄, 매핑 표 7행 |
| `.claude/commands/agency/brief.md` | 축소 → redirect | `Skill("moai") plan` 라우팅 |
| `.claude/commands/agency/build.md` | 축소 → redirect | `Skill("moai") design` 라우팅 |
| `.claude/commands/agency/review.md` | 축소 → redirect | `Skill("moai") e2e` 라우팅 |
| `.claude/commands/agency/profile.md` | 축소 → redirect | `Skill("moai") project` 라우팅 |
| `.claude/commands/agency/resume.md` | 축소 → redirect | `Skill("moai") run` 라우팅 |
| `.claude/commands/agency/learn.md` | 축소 → AGENCY_SUBCOMMAND_UNSUPPORTED + research 라우팅 | 직접 등가물 없음 |
| `.claude/commands/agency/evolve.md` | 축소 → AGENCY_SUBCOMMAND_UNSUPPORTED + research 라우팅 | 직접 등가물 없음 |
| `CLAUDE.md` | 수정 | §3 /agency DEPRECATED 섹션, §4 agency agents 흡수 공지, §9 design system 설정 위치 |

**기술적 접근**:
1. **Redirect stub 통일 패턴**: 모든 deprecated 명령은 ① frontmatter `description` "(Deprecated)" 접두사, ② 본문 `> DEPRECATED: ... SPEC-AGENCY-ABSORB-001` 라인, ③ `Use Skill("moai") with arguments: <subcommand> $ARGUMENTS` 종료.
2. **Unsupported 처리**: learn/evolve는 추가로 `> ERROR: AGENCY_SUBCOMMAND_UNSUPPORTED — ... has no direct equivalent.` 안내 + 마이그레이션 가이드 URL 제공.
3. **CLAUDE.md 갱신**:
   - §3에 `/agency (DEPRECATED — use /moai design)` 절 추가.
   - §4 Agency Agents 항목을 "(2) — copywriter and designer retained as fallback path B skills" + "planner, builder, evaluator, learner removed in SPEC-AGENCY-ABSORB-001 M5"로 축소.
   - §9에 "Design System Configuration (absorbed from agency, SPEC-AGENCY-ABSORB-001)" 절 추가.
4. **Deprecation window 명시**: `.claude/rules/agency/constitution.md` stub 본문에 "2 minor version cycles" 명문화 (REQ-DEPRECATE-003).

**검증**: REQ-ABSORB-001~003, REQ-DEPRECATE-001~004, REQ-DOC-001~003.

**Risks Mitigated**:
- 외부 사용자가 `/agency` 호출 시 silent fail 대신 명시적 마이그레이션 안내를 받는다.
- CLAUDE.md 갱신으로 향후 agent 작업이 흡수된 구조를 인지한다.

---

## M6 — 회고 SPEC 작성 (본 작업)

**Status**: 진행 중 (2026-04-24)

**Priority**: Low (메타 거버넌스, 산출물 무영향)

**범위**:
- `.moai/specs/SPEC-AGENCY-ABSORB-001/spec.md` 신규 (이미 작성)
- `.moai/specs/SPEC-AGENCY-ABSORB-001/plan.md` 신규 (본 파일)
- `.moai/specs/SPEC-AGENCY-ABSORB-001/acceptance.md` 신규 (다음 단계)

**기술적 접근**:
1. M1~M5의 산출물을 Read/Grep으로 직접 확인하여 EARS 요구사항 35건 추출.
2. 외부 SPEC(SPEC-PENCIL, SPEC-SKILL 등)에서 정의된 REQ는 참조만 하고 재정의하지 않음.
3. "확인 필요" 모순 사항(agency agent 파일 잔존 등)은 spec.md Open Items에 명시.

**Risks Mitigated**:
- characterization SPEC이 등록됨으로써 다수 코드·문서가 "존재하지 않는 SPEC을 참조하는" 모순 상태가 해소된다.
- 향후 cleanup SPEC(예: SPEC-AGENCY-CLEANUP-002) 작성 시 본 SPEC을 baseline으로 차이를 측정할 수 있다.

---

## Technical Approach Summary

### Architectural Decisions

1. **수직 도메인 흡수 vs 별도 시스템 유지**: 흡수 선택. 근거 — TRUST 5·헌법·MX 태그 등 거버넌스 인프라 단일화가 이중 운영 비용보다 압도적으로 우월.
2. **신규 스킬 트리 vs 구 스킬 in-place 수정**: 신규 트리 선택. 근거 — `moai update` 시 upstream 동기화 비용 감소, agency 사용자 호환성 보존.
3. **Verbatim 헌법 이전 vs 재작성**: Verbatim 선택. 근거 — Section 3 외 의미 변경 없음을 명확히 하여 audit 비용 절감.
4. **즉시 삭제 vs Deprecation window**: Deprecation window 선택. 근거 — 외부 참조(문서·블로그·교육 자료) 호환성과 사용자 학습 비용.

### Configuration-Driven Design

모든 GAN Loop·evolution·adaptation 임계값을 design.yaml로 외부화하여:
- 스킬 본문에는 하드코딩 없음 (REQ-CONFIG-002).
- 향후 evolution 시 canary check 대상이 명확.
- 사용자 환경별 override가 한 파일 수정으로 가능.

### Hybrid Path A/B

Claude Design import (path A)와 코드 기반 brand design (path B)를 양립시켜:
- 구독 등급에 따라 기본 추천 옵션을 동적 변경 (REQ-ROUTE-006).
- Path A 실패 시 path B로 graceful fallback (REQ-FALLBACK-001).
- 두 경로 모두 동일한 Phase C (GAN Loop) 품질 게이트로 수렴.

---

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| 구 agency 스킬·에이전트 파일 잔존으로 혼란 | Medium | REQ-DEPRECATE-003 deprecation window + 후속 cleanup SPEC 권고 |
| brand 파일 _TBD_ 미해소로 첫 `/moai design` 실패 | Medium | REQ-ROUTE-001이 brand interview를 강제 트리거하도록 설계 |
| design.yaml 파라미터 변경이 evolution에 의해 약화될 위험 | High | 헌법 §2 FROZEN — pass threshold floor 0.60 + require_approval=true |
| Claude Design 번들 형식 변경 시 path A 단절 | Medium | `claude_design.supported_bundle_versions` 화이트리스트 + REQ-FALLBACK-001 path B 폴백 |
| Pencil MCP 부재 환경에서 Phase B2.6 오작동 | Low | REQ-PENCIL-002 graceful skip + 구조화된 오류 코드 + Phase B3 자동 진행 |
| 외부 참조가 `.claude/rules/agency/constitution.md`를 직접 인용 | Low | redirect stub 유지 (REQ-CONST-003) + deprecation window |

---

## Verification Strategy

자세한 검증 방법은 `acceptance.md`에 정의된다. 본 plan.md 차원의 verification 전략:

1. **파일 존재성**: 모든 milestone 산출물 파일이 디스크에 존재함을 Glob/Read로 확인.
2. **REQ 추적성**: 각 REQ-ID가 적어도 한 개의 코드/문서 위치에 명시적으로 인용되는지 grep 검증.
3. **CLAUDE.md 갱신**: 4개 agency 관련 라인이 모두 업데이트 상태인지 확인.
4. **Deprecation 일관성**: 8개 agency 명령 모두 동일 패턴(frontmatter + body redirect) 적용 여부.
5. **모순 보고**: spec.md Open Items에 등록된 4건은 "확인 필요"로 명시되어 후속 SPEC의 input이 됨.

---

## Post-Completion Recommendations

본 SPEC 종료 후 다음 후속 트랙을 권고:

- **SPEC-AGENCY-CLEANUP-002** (가칭): REQ-DEPRECATE-003 만료 시점에 `.claude/rules/agency/`, `.claude/commands/agency/`, `.claude/agents/agency/`, `.claude/skills/agency-*/` 디렉터리 물리적 삭제.
- **SPEC-BRAND-ONBOARDING-001** (가칭): brand interview 자동 실행 트리거와 `_TBD_` 마커 자동 검출·치유 워크플로우 정의.
- **SPEC-AGENCY-LEARN-EVOLVE-MIGRATION** (가칭): `/agency learn`·`/agency evolve` 사용자 워크플로우의 정식 등가물 정의 (현재는 moai-workflow-research로 임시 라우팅).
