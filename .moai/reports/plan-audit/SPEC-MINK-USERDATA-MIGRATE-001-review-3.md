# SPEC Review Report: SPEC-MINK-USERDATA-MIGRATE-001
Iteration: 3/3
Verdict: FAIL
Overall Score: 0.82

Reasoning context ignored per M1 Context Isolation. Audit derived solely from `spec.md` v0.1.2, `acceptance.md` v0.1.2, `plan.md` v0.1.2, `spec-compact.md` v0.1.2 at the path `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/`.

---

## Executive Summary

Iter-2 의 8개 defect (ND-1 ~ ND-8) 중 **7개는 완전 해결**, **1개 (ND-1) 는 부분 해결 + 새로운 ND-1 family self-contradiction 도입**. 추가로 ND-4 와 동일 family 의 stale 카운트 1건 잔존을 발견. **iteration 3 가 마지막 허용 이터레이션이지만 신규 major self-contradiction 이 도입되어 PASS 불가**. 단, 두 defect 모두 1-line edit 으로 해소 가능한 trivial 수준이므로 escalation 보다는 사용자 직권으로 즉시 fix-up 권고.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: spec.md:L200-L233. `grep -oE 'REQ-MINK-UDM-[0-9]+' spec.md | sort -u | wc -l = 19`, sequential 001~019, no gaps, no duplicates, 3-digit zero padding consistent. § 4.1 (6 Ubiquitous) + § 4.2 (4 Event-Driven) + § 4.3 (3 State-Driven) + § 4.4 (3 Unwanted) + § 4.5 (2 Optional) + § 4.6 (1 Ubiquitous-Security) = 19. ✓
- **[PASS] MP-2 EARS format compliance**: spec.md:L200~L233. REQ-019 가 ND-7 fix 로 canonical Ubiquitous Security form 채택 — "The system **shall** preserve source file mode bits (never weakened) in the destination across all copy fallback paths when migrating sensitive files." (spec.md:L233). English shall + Korean 병기 + 4 sub-bullets (Scope/Sensitive files/Directory mode/Ownership). 나머지 18 REQ 도 각 tag 에 맞는 canonical template 충실 ("**When** ..., the [system] **shall** ...", "**While** ..., **shall** ...", "**If** ..., **then** ... **shall** ...", "**Where** ..., **shall** ..."). ✓
- **[PASS] MP-3 YAML frontmatter validity**: spec.md:L1-L13. `id: SPEC-MINK-USERDATA-MIGRATE-001`, `version: "0.1.2"`, `status: draft`, `created_at: 2026-05-13`, `updated_at: 2026-05-13`, `author: manager-spec`, `priority: High`, `labels: [brand, userdata, migration, brownfield, cross-cutting, path-resolver]`, `issue_number: null`, `depends_on: [SPEC-MINK-BRAND-RENAME-001, SPEC-MINK-ENV-MIGRATE-001]`, `related_specs: [...]`. 모든 required field 존재, 타입 올바름. ✓
- **[N/A] MP-4 Section 22 language neutrality**: N/A — Go single-language scope (`internal/userpath/**`). Auto-pass.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band | 19 REQ + 12 AC 대부분 unambiguous, 하지만 AC-001 #6 의 gate (case-sensitive `mink`) 와 example (`MINK` uppercase) 사이 case-sensitivity 충돌 (ND3-1, 아래 Defects 참조). 작지만 binary gate 의 정확 invocation 을 막는 inconsistency. |
| Completeness | 0.90 | 0.75-1.0 band | 모든 section 충실, HISTORY v0.1.2 row 가 spec.md/acceptance.md/plan.md/spec-compact.md 4개 문서에 정렬 (spec.md:L23 / acceptance.md:L9 / plan.md:L9 / spec-compact.md:L9). Defect Resolution Map (spec-compact.md:L214-L245) ND-1~ND-8 + D1~D13 통합 enumerate. 단 spec.md L321 "Given/When/Then 5 scenarios" stale (ND3-2). |
| Testability | 0.85 | 0.75-1.0 band | AC-005 #1 ND-8 fix 로 descriptive → binary command 전환 (`... \| wc -l = 0`, acceptance.md:L154). AC-003/009 dual stat notation 보존. Quality gate table 14 rows (acceptance.md:L427-L443). AC-001 #6 의 case-sensitivity 결함 (ND3-1) 이 binary verification 정확성 저해. |
| Traceability | 1.00 | 1.0 band | 19 REQ 모두 acceptance.md 의 `REQ 매핑:` 라인에서 참조. REQ-019 → AC-009 (acceptance.md:L311). REQ-016 → AC-007 (acceptance.md:L210). REQ-018 → AC-008a/AC-008b (acceptance.md:L232, L277). 12 main AC + 4 EC = 16 GIVEN block (`grep -cE '^\*\*Given\*\*' acceptance.md = 16`). |

---

## Regression Check (Iteration 3 vs Iteration 2)

Iter-2 의 8 defect 별 해결 상태:

- **ND-1 (AC-001 #6 gate vs example self-contradiction)** — **PARTIALLY RESOLVED + NEW DEFECT INTRODUCED**. 옛 경로 literal (`~/.goose`, `~/.mink`) 인용 제거 → "이전 디렉토리"/"새 MINK 디렉토리" prose 로 재작성 (acceptance.md:L43-L44). gate #1 (`grep -c 'goose' = 0`) 은 이제 정합. **그러나** 새 example 메시지가 gate #2 (`grep -Ec 'mink|밍크' ≥ 1`, case-sensitive) 를 위반함 — "MINK" (uppercase) 는 `mink` (lowercase) regex 와 매치 안 됨. 빈 set 산출. 실증 명령:
  ```
  $ printf 'INFO: 이전 디렉토리에서 새 MINK 디렉토리로\n' | grep -Ec 'mink|밍크'
  0   # gate requires >= 1, FAIL
  $ printf 'Migrated to the new MINK directory.\n' | grep -Ec 'mink|밍크'
  0   # gate requires >= 1, FAIL
  ```
  → ND3-1 신규 defect (아래 Defects Found 참조).

- **ND-2 (plan.md R1 stale "30+ 콜사이트")** — RESOLVED. plan.md:L204 = "R1 | 30+ literal occurrences across 18 distinct files 의 path semantics 비균질 ...". HISTORY v0.1.2 row (plan.md:L9) 가 정정 사실 명시.

- **ND-3 (acceptance.md DoD "11 main scenarios")** — RESOLVED. acceptance.md:L452 = "AC-MINK-UDM-001 ~ AC-MINK-UDM-010 (12 main scenarios — AC-001, 002, 003, 004a, 004b, 005, 006, 007, 008a, 008b, 009, 010; AC-004/008 split 포함) 모두 binary verified". 12 explicit enumerate.

- **ND-4 (spec.md L44 stale "11+ binary-verifiable")** — RESOLVED. spec.md:L45 = "12 binary-verifiable Acceptance Criteria (acceptance.md) + 4 edge cases".

- **ND-5 (acceptance.md DoD "Quality gate 13개")** — RESOLVED. acceptance.md:L454 = "Quality gate 14개 항목 모두 통과 (Coverage전체 / Coverage userpath / Race / Lint / Vet / Build / Brand-lint / LSP / Path-resolver / Test marker / Mode bits / MINK_HOME boundary / TRUST Tested / TRUST Secured; v0.1.1 신규 gate 3개 — test marker, mode bits, MINK_HOME boundary 포함)". 실제 표 (acceptance.md:L429-L443) 데이터 row 14 개 (header 제외) 와 정합.

- **ND-6 (spec.md L314 self-reference v0.1.0)** — RESOLVED. spec.md:L319 = "`.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/spec.md` (본 문서, v0.1.2)". 자체 version 표기 frontmatter 와 정합.

- **ND-7 (REQ-019 EARS form not canonical)** — RESOLVED. spec.md:L233 = "**REQ-MINK-UDM-019 [Ubiquitous]** The system **shall** preserve source file mode bits (never weakened) in the destination across all copy fallback paths when migrating sensitive files." English-first canonical Ubiquitous form. Korean 병기는 parenthetical 로 격하. Scope/Sensitive files/Directory mode/Ownership 가 sub-bullets 로 분리.

- **ND-8 (AC-005 #1 descriptive, not binary)** — RESOLVED. acceptance.md:L154 = "(ND-8 fix — descriptive → binary command) Non-legacy production file 의 `.goose` literal 0건: `grep -rEn '\"\.goose' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/legacy\.go:' | wc -l` 출력 = 0". Binary command.

**Summary**: 7/8 fully resolved. 1/8 (ND-1) partially resolved + new defect ND3-1 introduced (case-sensitivity slip).

**Stagnation check**: ND-1 family 가 iter-2 → iter-3 에 걸쳐 fix 시도 후 새 sibling defect 발생. 동일 defect 자체는 아니지만 같은 AC-001 #6 region 의 정합성 이슈가 2 iteration 연속 발견됨. "blocking defect — manager-spec made no progress" 까진 아니나, **AC-001 #6 region 의 gate-example 정합성을 한 번에 정확히 잡지 못하는 패턴** 관찰됨.

---

## Defects Found (Iteration 3)

### ND3-1. acceptance.md:L42-L44 — AC-001 #6 gate #2 (case-sensitive `mink`) vs base examples (`MINK` uppercase) 자기모순 — Severity: **MAJOR (blocking)**

L42 의 gate:
> 메시지가 단어 `mink` 또는 한국어 `밍크` 중 하나 이상을 **포함한다** (`grep -Ec 'mink|밍크' stderr.log` ≥ 1)

`grep -E` 는 case-sensitive (no `-i` flag). 따라서 패턴 `mink|밍크` 는 lowercase `mink` 또는 정확히 `밍크` 한국어만 매치한다. 대문자 `MINK` 는 매치 안 됨.

L43 의 Korean primary 예시:
> `INFO: 사용자 데이터가 이전 디렉토리에서 새 MINK 디렉토리로 마이그레이션되었습니다.`

L44 의 English subtext 예시:
> `Migrated user data from the legacy directory to the new MINK directory.`

두 예시 모두 `MINK` (대문자) 만 사용하며 lowercase `mink` 또는 `밍크` 가 부재 → gate `grep -Ec 'mink|밍크' ≥ 1` 위반. **실증**:

```
$ printf 'INFO: 사용자 데이터가 이전 디렉토리에서 새 MINK 디렉토리로 마이그레이션되었습니다.\n' | grep -Ec 'mink|밍크'
0   # FAIL — gate 요구치 ≥ 1
$ printf 'Migrated user data from the legacy directory to the new MINK directory.\n' | grep -Ec 'mink|밍크'
0   # FAIL — gate 요구치 ≥ 1
```

이는 ND-1 (iter-2) 의 fix 가 옛 경로 literal 인용 제거 과정에서 brand identifier 표기를 lowercase `mink` 에서 uppercase `MINK` 로 바꾸면서 gate #2 정합성을 깨뜨린 결과다 (이전 v0.1.1 에서는 `~/.mink` 경로 literal 이 lowercase `mink` 를 자동으로 포함). 구현자가 예시를 따르면 gate #2 실패, gate #2 를 따르면 예시와 불일치 — ND-1 과 동일 class 의 self-contradiction 이 다른 컨디션 (case) 에 그대로 재발.

**Remediation 3가지 옵션**:
- (a) 예시에 lowercase `mink` 또는 한국어 `밍크` 토큰 추가. 예: `INFO: 사용자 데이터가 이전 디렉토리에서 새 mink (밍크) 디렉토리로 마이그레이션되었습니다.` — 가장 단순.
- (b) 예시는 uppercase `MINK` 유지하되 gate 를 case-insensitive 로 변경: `grep -Eci 'mink|밍크' stderr.log ≥ 1` (또는 `[Mm][Ii][Nn][Kk]|밍크`).
- (c) 예시 본문에 brand identifier 를 별도 token 으로 명시: `INFO: 사용자 데이터가 이전 디렉토리에서 새 디렉토리로 마이그레이션되었습니다 (mink).`

**권장**: (a) — Korean line 의 "MINK" 를 "mink" (lowercase) 또는 "밍크" 로 치환. English line 도 lowercase `mink` 사용. 1-line edit (또는 2-line).

### ND3-2. spec.md:L321 — `.moai/specs/.../acceptance.md (Given/When/Then 5 scenarios)` stale count — Severity: minor

spec.md:L321 (§ 7.1 본 SPEC 산출물):
> - `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/acceptance.md` (Given/When/Then 5 scenarios)

실제 acceptance.md scenario 수: 12 main + 4 edge = **16 scenarios** (`grep -cE '^\*\*Given\*\*' acceptance.md = 16`).

이는 ND-4 (iter-2) 와 동일 class 의 stale count — ND-4 fix 가 L45 ("12 binary-verifiable") 만 정정하고 L321 의 parallel 표기를 누락. 한 단어 edit 으로 해소 가능: "5 scenarios" → "16 scenarios (12 main + 4 edge)" 또는 단순 "12 main + 4 edge scenarios".

severity 는 minor (구현자가 acceptance.md 를 직접 읽으므로 spec.md §7.1 cross-reference 의 count 오류가 work item 을 막진 않음). 하지만 spec.md HISTORY (L23) 의 "ND-4 fix" 와 모순되는 잔존 inconsistency 이므로 정정 필요.

---

## Chain-of-Verification Pass

Second-look findings (1차 audit 후 재독):

- **AC-001 #6 region (L34-L46) 전체 재독**: gate #1 (`grep -c 'goose' = 0`) 충족 여부 검증 — Korean example, English subtext 모두 literal `goose` 부재 ✓. gate #2 (`grep -Ec 'mink|밍크' ≥ 1`) 검증 시 lowercase 매치 불성립 ← ND3-1 발견. 이 검증은 1차 검토에서 `goose` 부재만 확인하고 `mink` 매치는 자명하다고 가정한 결과로 놓칠 뻔함. case-sensitive grep flag (`-E` no `-i`) 의 함의를 명시 확인하여 발견.
- **REQ 19 개 sequential 재확인**: REQ-001~019 각 라인 (spec.md:L200-L233) 직접 매핑 ✓.
- **AC heading 12개 enumerate 재확인**: `grep -E '^## AC-MINK-UDM-' acceptance.md` 출력으로 AC-001/002/003/004a/004b/005/006/007/008a/008b/009/010 12개 확인 ✓.
- **EC 4개 재확인**: `grep -cE '^### EC-MINK-UDM-' acceptance.md = 4` ✓.
- **Quality gate row 정확 계수**: 1차에서 "14 rows" 라고 acceptance.md HISTORY 가 claim. 표 body row (header 제외) 직접 enumerate → 14 ✓ (Coverage전체/Coverage userpath/Race/Lint/Vet/Build/Brand-lint/LSP/Path-resolver/Test marker/Mode bits/MINK_HOME boundary/TRUST Tested/TRUST Secured).
- **HISTORY v0.1.2 4-file 정렬 재확인**: spec.md:L23, acceptance.md:L9, plan.md:L9, spec-compact.md:L9 모두 v0.1.2 row 존재 ✓.
- **REQ-008 ↔ AC-001 정합 재독**: REQ-008 (spec.md:L210) "shall not contain the legacy brand word 'goose'; it shall contain the new brand identifier 'mink' or its Korean form '밍크' verbatim" — REQ 본문도 lowercase `mink` 와 `밍크` 를 표기. 즉 AC-001 #6 의 example 가 REQ-008 의 "verbatim" 표기와도 불일치 (REQ-008 의 'mink' literal 과 example 의 'MINK' literal 불일치). REQ ↔ AC 일관성 측면에서도 ND3-1 강화 근거.
- **spec.md L321 §7.1 stale count 발견**: ND-4 sweep miss. ND3-2 로 보고.
- **AC-002 #4 / AC-010 #2 / EC-004 #3 의 `goose` 단어 허용**: 각 AC 의 REQ trace (REQ-012, REQ-017 negative, REQ-010) 가 별도이며 명시적으로 AC-001 #6 gate 가 적용 안 된다고 명기. 의도된 exemption, defect 아님.
- **Backtick balance**: spec.md/acceptance.md/plan.md/spec-compact.md 전수 스캔 — D10 의 3개소 (spec.md L52/L178/L292 family) 모두 balanced 유지 ✓. 신규 imbalance 없음.

New defects 발견 횟수: 2 (ND3-1 blocking major, ND3-2 minor cosmetic).

---

## Recommendation

Verdict: **FAIL**. 단 두 defect 모두 1-line edit 으로 해소 가능. 다음 fix 적용 후 v0.1.3 으로 re-submit 또는 사용자 직권 fast-track approval:

### Required fixes (iteration 4 또는 escalation 직권 처리)

1. **(blocking) acceptance.md:L43-L44 — AC-001 #6 base examples 의 `MINK` 대문자 → lowercase `mink` 또는 `밍크` 한국어 병기**

   현재:
   ```
   - 메시지 본문 (Korean primary line) 의 base 예시 ... `INFO: 사용자 데이터가 이전 디렉토리에서 새 MINK 디렉토리로 마이그레이션되었습니다.`
   - REQ-008 에 따라 영문 subtext ... 예: `Migrated user data from the legacy directory to the new MINK directory.`
   ```
   정정 (옵션 a 권장):
   ```
   - 메시지 본문 (Korean primary line) 의 base 예시 ... `INFO: 사용자 데이터가 이전 디렉토리에서 새 mink 디렉토리로 마이그레이션되었습니다.` (또는 "새 밍크 디렉토리" 한국어 표기)
   - REQ-008 에 따라 영문 subtext ... 예: `Migrated user data from the legacy directory to the new mink directory.`
   ```

   대안 (옵션 b): gate 자체를 case-insensitive 로:
   ```
   메시지가 단어 `mink` 또는 한국어 `밍크` 중 하나 이상을 **포함한다** (`grep -Eci 'mink|밍크' stderr.log` ≥ 1)
   ```
   단, 이 경우 REQ-008 spec.md:L210 의 "verbatim" 표기 의미도 같이 점검 필요 (verbatim 이 case-sensitive 인지 ambiguous).

2. **(minor) spec.md:L321 — `acceptance.md (Given/When/Then 5 scenarios)` → `acceptance.md (12 main + 4 edge scenarios)`**

   ND-4 sweep miss. ND-4 가 spec.md L45 만 fix 하고 L321 의 parallel 표기 놓침. 1-token edit.

### Iteration policy 권고

이 SPEC 는 brownfield + security-sensitive 이지만, iter-2 → iter-3 사이 7/8 defect 가 완전 해결되었고 traceability 가 1.00 으로 maximal, must-pass 4 모두 PASS. ND3-1 은 character-class slip 1건, ND3-2 는 stale token 1개로 모두 trivial. 정상적인 process 라면 iter-4 를 요구해야 하나 max_iterations: 3 정책상 escalation 대상.

**Escalation recommendation**: 사용자에게 두 가지 옵션 제시:
- (옵션 A — 권장) 직권 fast-track: 위 2개 fix 를 사용자가 직접 (또는 manager-spec 한 번 더 호출하여) 즉시 적용하고 v0.1.3 발행 후 `/moai run` 진입. ND3-1 의 case-sensitivity 결함은 구현 단계에서 발견되어도 즉시 발견 가능한 표면적 결함이므로 audit overhead 추가 부담은 불필요.
- (옵션 B) 정식 iter-4 허용: harness.yaml `max_iterations` 를 일시적으로 4 로 올리고 정식 audit 재시도.

### Post-merge cleanup recommendation

만약 fast-track 으로 진입 후 Run phase 에서 `internal/userpath/migrate.go` 의 stderr 알림 string literal 을 작성할 때:
- Go const string 으로 표준화: `const migrationNoticeKR = "INFO: 사용자 데이터가 이전 디렉토리에서 새 mink (밍크) 디렉토리로 마이그레이션되었습니다."` 와 `const migrationNoticeEN = "Migrated user data from the legacy directory to the new mink directory."` 로 lowercase `mink` 명시.
- 동일 const 를 AC-001 unit test 의 expected output 으로 재사용하여 spec ↔ test ↔ runtime 3-way 정합 자동 유지.

---

## Defect History Across All Iterations

| Iter | Total | Resolved Next | Carry Forward | New |
|------|-------|---------------|---------------|-----|
| 1 | 13 (D1-D13) | 11 full / 2 partial | D7, D11 | 0 |
| 2 | 8 (ND-1 ~ ND-8) | 7 full / 1 partial | ND-1 (case slip) | 0 |
| 3 | 2 (ND3-1, ND3-2) | TBD | ND-1 family slip | 0 |

**Pattern**: AC-001 #6 region 의 gate-example 정합성 이슈가 3 iteration 누적해서 잔존. iter-1 D7 (weasel "동등 영문" 제거) → iter-2 ND-1 (gate ↔ goose example 자기모순) → iter-3 ND3-1 (gate ↔ mink case 자기모순). 동일 region 에서 fix 가 또 다른 결함을 만드는 mole-whack 패턴. 사용자 직권 fix 시 위 const-based 3-way binding 으로 근원 차단 권고.

---

Verdict: FAIL
