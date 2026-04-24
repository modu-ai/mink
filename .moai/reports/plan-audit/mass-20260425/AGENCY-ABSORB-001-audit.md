# SPEC Review Report: SPEC-AGENCY-ABSORB-001

Iteration: 1/3
Verdict: **FAIL** (status=completed이나 완성도 불일치 + MP-1 REQ 번호 결함)
Overall Score: 0.62

Reasoning context ignored per M1 Context Isolation. Audit based solely on spec.md/plan.md/acceptance.md + filesystem evidence.

---

## Must-Pass Results

- **[FAIL] MP-1 REQ number consistency**
  - REQ-DETECT 계열에 -001/-002 없이 `REQ-DETECT-003`만 단독 존재 (spec.md:L236). 동일 category 내 sequential 원칙 위반.
  - spec.md 본문 REQ footer는 "총 41건"이라 집계하지만, acceptance.md:L287 Quality Gate Checklist는 "35개 REQ 모두 증거 파일 인용 포함"이라 기록 → 내부 불일치.
  - REQ-PENCIL-001이 spec.md:L153(REQ-ABSORB-008)의 하위 리스트에 언급되나 REQ coverage footer(spec.md:L334)에는 비포함. "참조만 하며 재정의하지 않음"이라는 주석(spec.md:L336)과 실제 텍스트 사용이 모호.
  - 판정: 단일 gap(DETECT-001/002 부재) 하나만으로도 MP-1 FAIL.

- **[PASS] MP-2 EARS format compliance**
  - 조사한 REQ 모두 EARS 패턴에 부합: Ubiquitous ("The system SHALL …"), Event-Driven (WHEN…), State-Driven (WHILE…), Unwanted Behavior (IF…THEN) 모두 일관 사용 (spec.md:L109,113,127,131,180,184,204,212,232,280 등).

- **[PASS] MP-3 YAML frontmatter validity**
  - spec.md:L1-18: id(string), version(string), status(string), created/updated/completed(ISO date), author(string), priority(string), labels(array) 전 필드 존재 및 type 유효.
  - acceptance.md:L1-8, plan.md:L1-8 동일 유효.

- **[N/A] MP-4 Section 22 language neutrality**
  - 본 SPEC은 design workflow 흡수 범위로, 16-language 다국어 툴링 주제가 아님. N/A 적용.

---

## Category Scores (rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band — 1-2개 요구사항에 경미한 해석 필요 | spec.md:L289 REQ-DOC-001 "확인 필요" 박스가 본문 요구사항에 섞여 있어 해석 모호 |
| Completeness | 0.75 | 0.75 band — 한 비핵심 섹션 sparse | HISTORY/WHY(Purpose)/WHAT(Scope)/REQ/AC/Exclusions 모두 존재. Open Items 섹션 풍부. 단 spec.md에 명시적 "ACCEPTANCE CRITERIA" 섹션 없이 acceptance.md로 분리. 글로벌 AC 요약(spec.md:L301-308)은 포함 |
| Testability | 0.80 | 0.75 band 상단 | 41 REQ 대부분이 grep/Read로 binary-testable. acceptance.md:L119-202 per-REQ verification matrix로 executable. 단 AC-GLOBAL-4 "Learner 자동 수정 차단"은 정적 grep으로만 검증, 실제 런타임 Frozen Guard 실행 증거 없음 |
| Traceability | 0.70 | 0.75 band 하단 | 대부분 REQ→evidence 매핑 완비. 그러나 REQ-DETECT 계열 001/002 부재가 trace table 파손. REQ-PENCIL-001 본문 언급(L153) 대비 coverage footer 누락 |

---

## Defects Found

**D1. spec.md:L236 — REQ-DETECT category에 001/002 없이 -003만 단독 존재 — Severity: major**
  MP-1 sequential 원칙 위반. "REQ-DETECT-003 (State-Driven)"이라는 이름 자체가 선행 번호의 존재를 함의하나 해당 SPEC에 정의되어 있지 않음. 독자가 외부 SPEC 참조로 오해할 여지 있음.

**D2. acceptance.md:L287 vs spec.md:L336 — REQ 총 개수 불일치 (35 vs 41) — Severity: minor**
  Quality Gate Checklist는 "35개 REQ 모두 증거 파일 인용"으로 기록, spec.md footer는 "총 EARS 요구사항 수: 41건"로 기록. 동일 SPEC 내 자기모순.

**D3. 실행 정합성 — `.agency/` legacy 디렉터리(8 file) 여전히 존재 — Severity: major (실행 결함, spec는 Exclusions로 허용)**
  `.agency/config.yaml`, `.agency/fork-manifest.yaml`, `.agency/context/*.md` 5개, `.agency/templates/brief-template.md` 확인됨. CLAUDE.md에서 "Legacy .agency/ directories are archived via moai migrate agency"라 선언(CLAUDE.md line 417)했으나 실제 archive/migration 미수행. SPEC의 Exclusions 조항(spec.md:L86)은 삭제 금지를 명시하므로 spec 문서상 결함은 아니나, "status=completed" 선언과 CLAUDE.md 문구가 filesystem 현실과 불일치.

**D4. 실행 정합성 — `.claude/skills/agency-*` 5개 디렉터리(agency-client-interview, agency-copywriting, agency-design-system, agency-evaluation-criteria, agency-frontend-patterns) 잔존 — Severity: minor (Open Items #3에 self-disclosure됨)**
  SPEC의 Open Items #3에서 "구 디렉터리 정리는 후속 sync 작업으로 권고"라 자인(spec.md:L329). 자기 공개는 긍정 요소이나 completed 상태 선언과 상충.

**D5. 실행 정합성 — `.claude/agents/agency/{planner,builder,evaluator,learner,copywriter,designer}.md` 6 파일 전체 잔존 — Severity: minor (REQ-DOC-001 "확인 필요" 섹션에 공개됨)**
  CLAUDE.md는 "planner, builder, evaluator, learner removed"라 선언하지만 6개 파일 모두 `.claude/agents/agency/` 디렉터리에 존재. spec.md:L289가 "active catalog 등록 해제 = removed"로 재해석하여 봉합 시도 — 어휘 모호성 있음.

**D6. spec.md:L336 REQ-PENCIL-001 참조 취급 — Severity: minor**
  REQ-ABSORB-008 본문(spec.md:L153)에는 `moai-workflow-pencil-integration` 명시되나 footer REQ coverage 목록엔 REQ-PENCIL 계열 비포함. "참조만 하며 재정의하지 않음"이라 했으나 실제 REQ-FALLBACK-002(spec.md:L216)가 "structured Pencil error code"를 정의하므로 경계 불명확.

**D7. `.claude/commands/moai/design.md` 268 byte — Severity: info (정보성)**
  Thin Command Pattern 준수 여부 spot-check로는 양호(268 bytes, under 20 LOC 가능). 본 감사 범위 내 결함 아님.

---

## Chain-of-Verification Pass

Second-pass findings:
- REQ 번호 끝-to-끝 재확인: ABSORB(1-9 연속 OK), CONST(1-4 OK), ROUTE(1-8 OK), FALLBACK(1-3 OK), BRIEF(1-3 OK), **DETECT(-003만)** ← 확정 gap, CONFIG(1-3 OK), DESIGN-DOCS(1-3 OK), DEPRECATE(1-4 OK), DOC(1-3 OK). 첫 패스에서 감지한 DETECT gap 외 추가 gap 없음을 재확인.
- Exclusions 구체성: spec.md:L80-89에 6개 구체 항목(신규 코드 금지, 파일 수정 금지, agency 즉시 삭제 금지 등) 나열, 모두 명확. PASS.
- 요구사항 간 모순: REQ-DEPRECATE-003(삭제 금지, deprecation window 유지) vs REQ-DOC-001(removed 선언) 간 "어휘 모호" — 기 감지(D5)됨. 추가 모순 없음.
- Traceability 전수: 41 REQ 중 ad-hoc 5개 샘플(REQ-ABSORB-005, REQ-CONST-003, REQ-ROUTE-006, REQ-CONFIG-002, REQ-DEPRECATE-001) 모두 evidence 파일 인용 확인. Traceability 파손 없음.
- 2차 패스에서 신규 결함 없음. 첫 패스 판정 유지.

---

## Regression Check

N/A (iteration 1 — 이전 iteration 없음).

---

## Recommendation

**FAIL 사유 요약**:
- MP-1 violation (REQ-DETECT 계열 번호 gap) 단 하나로 must-pass 실패 → 전체 FAIL.
- 부차적으로 실행 정합성(D3-D5)이 status=completed 선언과 불일치.

**완료율 평가**: 문서 품질 기준 ~90%, 실행 정합성(filesystem vs 선언) 기준 ~75%, 종합 **약 82% 완료**. "spec-first characterization" 성격을 감안하여 SPEC 문서 자체는 사용 가능 수준이나, 번호 gap 정리 + 실행 불일치 공식화 필요.

**수정 권고 (manager-spec 이관 사항)**:

1. **[필수] REQ-DETECT 번호 정합성 해결 (spec.md:L236)**:
   - 옵션 A: `REQ-DETECT-003`을 `REQ-DETECT-001`로 rename.
   - 옵션 B: REQ-DETECT-001/002의 의도된 정의가 외부 SPEC에 존재한다면 그 출처를 footer REQ coverage 주석에 명시.

2. **[필수] REQ 총 개수 통일 (acceptance.md:L287)**:
   - "35개 REQ" → "41개 REQ"로 정정. spec.md footer와 일치.

3. **[권장] REQ-PENCIL 경계 명확화 (spec.md:L153, L336)**:
   - REQ-ABSORB-008에서 pencil 스킬 언급 시 "자체 SPEC: SPEC-PENCIL-*" 참조 명시.
   - REQ-FALLBACK-002의 Pencil error code는 본 SPEC 고유로 유지.

4. **[권장] Open Items → 후속 SPEC 공식화**:
   - spec.md Open Items #1(agent 파일 잔존), #3(구 스킬 잔존)은 SPEC-AGENCY-CLEANUP-002로 명시 이관 (spec.md:L307 Cross-References에 이미 언급됨 — acceptance.md에 체크리스트로 추가).

5. **[선택] 실행 정합성 vs completed 선언**:
   - status=completed를 유지할 경우 "Implementation Scope" 문서를 추가하여 "physical cleanup은 deprecation window 이후"임을 명시.
   - 또는 status를 `implemented-partial` 등 새 값으로 변경하여 SPEC-AGENCY-CLEANUP-002 완료 시 `completed`로 승격.

---

Report written: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/AGENCY-ABSORB-001-audit.md`
