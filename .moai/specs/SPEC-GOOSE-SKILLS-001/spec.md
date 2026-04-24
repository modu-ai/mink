---
id: SPEC-GOOSE-SKILLS-001
version: 0.2.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 2
size: 대(L)
lifecycle: spec-anchored
labels: [skills, progressive-disclosure, yaml-frontmatter, trigger-matching, security, go]
---

# SPEC-GOOSE-SKILLS-001 — Progressive Disclosure Skill System (L0~L3, YAML, 4 Trigger)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-primitives §2 + ROADMAP v2.0 Phase 2 기반) | manager-spec |
| 0.2.0 | 2026-04-25 | plan-auditor iteration 1 반영 (Score 0.58 → 재감사 대응): ① labels 채우기 ② §5 format declaration + AC Gherkin 유지 근거 명시 ③ REQ-SK-002/013/014-sec/016/017/018 AC 신설 (AC-SK-011~016) ④ REQ-SK-004 semantics 정정 (error slice / partial success) ⑤ REQ-SK-019 (YAML 15-key schema 승격) ⑥ REQ-SK-020 (L0-L3 effort vs MoAI 3-Level Progressive Disclosure 직교축 관계 명시) ⑦ REQ-SK-021 (4-trigger 스코프 확정 + keyword/agent/phase/language OUT) ⑧ REQ-SK-022 (shell injection threat model 확장) ⑨ §2.4 Standard Alignment (agentskills.io) 신설 ⑩ §5.1 Traceability 테이블 추가 | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 **Skill 시스템**을 정의한다. Claude Code의 Progressive Disclosure(L0~L3 effort 티어) + YAML frontmatter + 4-trigger 활성화 모델을 Go로 포팅하여, `QueryEngine`(SPEC-GOOSE-QUERY-001)이 매 iteration 직전 적절한 Skill을 로드·치환·권한 검증한 후 system/user prompt에 반영하게 한다.

본 SPEC이 통과한 시점에서 `internal/skill` 패키지는:

- `LoadSkillsDir(root)`로 디스크 상의 `SKILL.md` 파일을 walk하여 `SkillDefinition` 레지스트리를 구성하고,
- YAML frontmatter를 allowlist 기반(`SAFE_SKILL_PROPERTIES`)으로 파싱하여 **알 수 없는 속성은 default-deny**하며,
- 4-trigger(inline / fork / conditional / remote) 각각을 결정적으로 활성화하고,
- `${CLAUDE_SKILL_DIR}`, `${CLAUDE_SESSION_ID}` 등 변수 치환을 skill 본문 로드 시점에 적용하며,
- `paths:` 조건부 매칭은 gitignore 문법(`!` 부정 포함)으로 처리하고,
- Progressive Disclosure(L0=50 tokens, L1=200, L2=500, L3=1000+)에 따라 **frontmatter만 먼저 파싱**해 발견 비용을 최소화한다(`estimateSkillFrontmatterTokens`).

본 SPEC은 **skill 자체 정의/저장/버전관리/마켓플레이스**는 다루지 않는다. 그것은 PLUGIN-001 또는 별도 생태계 SPEC.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 2의 4 primitive(Skills/MCP/Agents/Hooks) 중 Skill은 Agent/Hook/Plugin 모두의 의존 대상. `SUBAGENT-001`이 agent system prompt를 forked skill로 구성할 수 있어야 하고, `PLUGIN-001`이 plugin manifest의 `skills:` 배열을 로드할 수 있어야 한다.
- `.moai/project/research/claude-primitives.md` §2가 Claude Code의 Skill 아키텍처(frontmatter schema, 4 trigger, allowlist permission)를 제시한다. 본 SPEC은 그 구조를 Go 이디엄(struct tag + `go-yaml/yaml` + `fsnotify` optional)으로 확정한다.
- QUERY-001의 continue site는 `State.Messages` 외의 프롬프트 구성 요소(system/user prompt)를 주입받기만 한다. Skill이 그 **주입자**가 된다.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript** (`./claude-code-source-map/`): `skills/`, `loadSkillsDir.ts`, `parseSkill*`, `estimateSkillFrontmatterTokens`. Go 포팅 시 `go-yaml/yaml` + 직접 ignore matcher 사용. TS async 동적 로딩은 Go에서는 eager walk + lazy body read.
- **MoAI-ADK `.claude/skills/`**: SKILL.md + `modules/` + `references/` 구조. 본 SPEC의 스키마는 MoAI-ADK의 진화형.
- **Hermes Agent `agent/prompts/`**: 정적 prompt 템플릿. Progressive Disclosure는 Hermes에 없으며, 본 SPEC에서 신규 도입.

### 2.3 범위 경계

- **IN**: `SkillDefinition`/`SkillFrontmatter` 구조체, YAML frontmatter parser(allowlist-default-deny), Progressive Disclosure(effort L0/L1/L2/L3) 토큰 추정, 4-trigger(inline/fork/conditional/remote) 결정 로직, `${VAR}` 치환, `paths:` gitignore 매칭, `disable-model-invocation`/`user-invocable`/`allowed-tools` 권한 게이트, `FileChanged` hook consumer(조건부 활성화 경로), Remote skill(`_canonical_{slug}`) 로더 skeleton.
- **OUT**: 실제 LLM 호출(ADAPTER-001), Forked skill agent spawning(SUBAGENT-001), Hook 이벤트 디스패치(HOOK-001), Plugin manifest 통합(PLUGIN-001), Remote skill의 AKI/GCS 인증 구현(Phase 5+), Skill SDK(유저 작성 도구), Skill editor UI(CLI-001).

### 2.4 표준 정렬 (Standard Alignment)

본 SPEC이 **어떤 외부 표준과 일치·확장·차별화되는지** 명시적으로 기록한다. 잠재 사용자·포팅자가 호환성 가정을 잘못 두지 않도록 한다.

#### 2.4.1 Claude Code Agent Skills (`agentskills.io`)

- **정렬 방식**: **부분 호환(compatible subset, with extensions)**.
- **호환 부분**: YAML frontmatter `name` / `description` / `allowed-tools` / `argument-hint` / `disable-model-invocation`, `SKILL.md` 단일 파일 구조, Progressive Disclosure(effort 티어) 개념, 4-trigger 모델(inline/fork/conditional/remote) — 모두 `agentskills.io` 표준의 명시 필드·패턴을 그대로 수용.
- **확장 부분**(본 SPEC 고유): `when-to-use`(MoAI-ADK 진화형), `paths:` gitignore 부정 패턴, `${CLAUDE_SKILL_DIR}` / `${CLAUDE_SESSION_ID}` / `${USER_HOME}` 고정 변수 세트(env var 금지), `context: fork` 필드 사용 제한(감지만, agent spawn은 SUBAGENT-001).
- **차별 부분**: `hooks:` 맵은 **파싱만**(실행은 HOOK-001). Remote skill의 인증은 Phase 5+로 연기. `shell:` 디렉티브는 parse-time 실행 금지(REQ-SK-013).
- **양방향 이식성**: `agentskills.io`의 reference spec에 없는 키(본 SPEC의 `when-to-use`)는 `agentskills.io` 준수 런타임에서 무시 가능하도록 optional로 파싱. 반대로 `agentskills.io` 런타임에서 유효한 SKILL.md는 본 SPEC의 `SAFE_SKILL_PROPERTIES` allowlist에 존재하는 키만 사용하면 그대로 로드 가능.
- **비호환 선언**: `agentskills.io` 표준이 `shell:` parse-time 실행·임의 env var 치환을 허용하더라도, 본 SPEC은 보안 정책상 이를 **거부**한다. 이 정책 차이는 REQ-SK-013 / REQ-SK-014 / REQ-SK-022에 고정된다.

#### 2.4.2 MoAI-ADK 3-Level Progressive Disclosure (CLAUDE.md §13) 와의 관계

CLAUDE.md §13이 정의하는 MoAI-ADK 3-Level Progressive Disclosure와 본 SPEC의 L0~L3 effort 티어는 **서로 다른 축**에서 작동한다. 둘 다 공존하며 충돌하지 않는다.

| 축 | 주체 | 측정 대상 | 값 |
|---|---|---|---|
| MoAI-ADK 3-Level (CLAUDE.md §13) | Skill 로더(어느 내용까지 읽을지) | 디스크 I/O 단계 | Level 1=Metadata(frontmatter), Level 2=Body, Level 3=Bundled files |
| 본 SPEC L0~L3 (REQ-SK-010) | Skill 저자(이 스킬이 얼마나 상세한지) | 토큰 budget 힌트 | L0=~50, L1=~200(default), L2=~500, L3=~1000+ |

- **정합**: MoAI-ADK Level 1(Metadata-only) 단계에서 본 SPEC의 `EstimateSkillFrontmatterTokens`가 `effort` 값과 무관하게 호출된다. Level 2 승격 시에 비로소 body를 읽고 L0~L3이 consumer(`QueryEngine`) 의사결정에 영향을 준다.
- **직교성**: Level 1/2/3 = **"지금 어디까지 로드했는가"**, L0/L1/L2/L3 = **"저자가 선언한 스킬의 분량"**. 두 축은 독립적이며 상호 매핑되지 않는다.
- **REQ 고정**: 이 관계는 REQ-SK-020에 behavioral contract로 고정된다.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/skill/` 패키지 생성.
2. `SkillDefinition`, `SkillFrontmatter`, `TriggerMode`, `EffortLevel` 타입.
3. `SkillRegistry` 컨테이너(map + RWMutex, atomic replace).
4. `LoadSkillsDir(root string, opts ...LoadOption) (*SkillRegistry, error)` walker.
5. `ParseSkillFile(path string, content []byte) (*SkillDefinition, error)` — YAML frontmatter + body 분리.
6. `ValidateFrontmatter(fm SkillFrontmatter) error` — `SAFE_SKILL_PROPERTIES` allowlist 기반.
7. `EstimateSkillFrontmatterTokens(fm SkillFrontmatter) int` — name + description + when-to-use 길이 기반 휴리스틱.
8. `ResolveEffort(fm SkillFrontmatter) EffortLevel` — `effort:` 필드 → L0/L1/L2/L3 매핑 (숫자 또는 알파넘).
9. 4-trigger 결정자:
   - `IsInline(fm)` / `IsForked(fm)` / `IsConditional(fm)` / `IsRemote(fm)`.
10. Conditional 활성화 — `paths:` 패턴 gitignore 매칭(`github.com/sabhiram/go-gitignore` 또는 자체 구현).
11. 변수 치환 — `${CLAUDE_SKILL_DIR}`, `${CLAUDE_SESSION_ID}`, `${USER_HOME}` 등 body/hooks 로드 시점 치환.
12. 권한 게이트 — `disableModelInvocation`, `userInvocable`, `allowedTools`의 런타임 검증 함수.
13. `FileChangedConsumer(paths []string) []string` — 변경된 파일 경로를 활성화 대상 skill ID 목록으로 변환(HOOK-001에서 FileChanged 이벤트를 본 패키지로 라우팅).
14. Remote skill loader skeleton — `LoadRemoteSkill(uri string) (*SkillDefinition, error)` (HTTP fetch만; 인증은 Phase 5+ TODO).

### 3.2 OUT OF SCOPE

- **Forked skill 실행**: `context: fork` 플래그 감지는 본 SPEC 범위이나, 실제 sub-agent spawn은 SUBAGENT-001.
- **Slash command 전개**: `!command` + `$ARG` 치환은 COMMAND-001.
- **Hook 이벤트 dispatcher**: `SessionStart`/`PostToolUse` 등 frontmatter의 `hooks:` 필드는 본 SPEC이 파싱만. 실제 라우팅은 HOOK-001.
- **Remote skill 인증/캐시**: 본 SPEC은 `http.Get`까지만. AKI/GCS/OAuth는 Phase 5+.
- **Skill hot-reload**: `fsnotify` 기반 watcher는 PLUGIN-001에서 통합(atomic clearThenRegister).
- **Skill 편집 UI**: CLI-001 / 별도 SPEC.
- **MCP prompts/list로부터 Skill 변환**: MCP-001의 책임.
- **Keyword/Agent/Phase/Language 기반 trigger matching**: MoAI-ADK `.claude/skills/` 관례의 `triggers: keywords`, `triggers: agents`, `triggers: phases`, `triggers: languages` 다차원 활성화는 본 SPEC에서 지원하지 않는다. 본 SPEC은 Claude Code 상류의 4-trigger(inline/fork/conditional/remote) 모델만 수용한다(REQ-SK-021). MoAI-style 다차원 trigger가 필요하면 별도 후속 SPEC(`SPEC-GOOSE-SKILLS-MULTIDIM-XXX`)에서 다룬다.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-SK-001 [Ubiquitous]** — The `SkillRegistry` **shall** reject any frontmatter key that is not in `SAFE_SKILL_PROPERTIES` allowlist; unknown keys **shall** cause `ParseSkillFile` to return `ErrUnsafeFrontmatterProperty` without partial registration.

**REQ-SK-002 [Ubiquitous]** — The `SkillFrontmatter` parser **shall** preserve exact YAML ordering for `hooks:` arrays; event handler order **shall** match source document order (no sorting, no dedup).

**REQ-SK-003 [Ubiquitous]** — The `EstimateSkillFrontmatterTokens` function **shall** compute token count from `name + description + whenToUse` fields only, without loading the skill body file contents.

**REQ-SK-004 [Ubiquitous]** — Every `SkillDefinition` in the registry **shall** have a unique `ID` (path-derived slug). When a duplicate ID is encountered, `LoadSkillsDir` **shall** (a) preserve the first-registered entry unchanged, (b) append `ErrDuplicateSkillID` (wrapping the offending file path) to the returned error slice, and (c) continue loading remaining files without aborting — consistent with the partial-success semantics defined in REQ-SK-005(e). The loader **shall not** return `ErrDuplicateSkillID` as the top-level `error` return value; duplicate-ID reporting travels exclusively through the error slice channel.

### 4.2 Event-Driven (이벤트 기반)

**REQ-SK-005 [Event-Driven]** — **When** `LoadSkillsDir(root)` is invoked, the walker **shall** (a) recursively scan `root` for files named `SKILL.md`, (b) parse each file's YAML frontmatter, (c) validate against `SAFE_SKILL_PROPERTIES`, (d) populate `SkillRegistry` atomically, and (e) return the populated registry plus an error slice (one entry per skipped file; no partial failure aborts the full load).

**REQ-SK-006 [Event-Driven]** — **When** a skill body contains `${CLAUDE_SKILL_DIR}`, the `ResolveBody(session)` function **shall** substitute with the absolute path of the skill's parent directory; **when** it contains `${CLAUDE_SESSION_ID}`, substitute with `session.ID`; unknown variables **shall** remain literal.

**REQ-SK-007 [Event-Driven]** — **When** `FileChangedConsumer(changedPaths)` is invoked, the function **shall** iterate the registry, match each conditional skill's `paths:` patterns against `changedPaths` using gitignore semantics, and return the list of skill IDs whose patterns matched.

**REQ-SK-008 [Event-Driven]** — **When** a skill's frontmatter sets `context: fork`, `IsForked(fm)` **shall** return `true` and the skill **shall not** be eligible for inline prompt-body injection; instead the consumer(SUBAGENT-001) **shall** route it through `runAgent`.

**REQ-SK-009 [Event-Driven]** — **When** a skill's frontmatter sets `disable-model-invocation: true`, the registry's `CanInvoke(skill, actor)` gate **shall** return `false` for `actor = "model"` and `true` only for `actor = "user"` or `actor = "hook"`.

### 4.3 State-Driven (상태 기반)

**REQ-SK-010 [State-Driven]** — **While** the `effort:` frontmatter field is absent, the resolved `EffortLevel` **shall** default to `L1` (200 tokens budget); explicit `L0`/`L1`/`L2`/`L3` strings override; integer values 0/1/2/3 **shall** map to `L0`/`L1`/`L2`/`L3`.

**REQ-SK-011 [State-Driven]** — **While** `paths:` frontmatter is absent, `IsConditional(fm)` **shall** return `false`; the skill **shall not** be included in any `FileChangedConsumer` result.

**REQ-SK-012 [State-Driven]** — **While** a skill is remote(`ID` has `_canonical_` prefix), `LoadRemoteSkill` **shall** bypass the disk walker and fetch the `SKILL.md` over HTTP; cached remote skills **shall** be re-validated on every `LoadSkillsDir` call (no disk persistence in this SPEC).

### 4.4 Unwanted Behavior (방지)

**REQ-SK-013 [Unwanted]** — The parser **shall not** execute any `shell:` directive at parse time; `shell:` is parsed as metadata only, and actual shell invocation is deferred to the consumer (hook dispatcher / agent runtime).

**REQ-SK-014 [Unwanted]** — **If** a skill body contains `${` followed by an unknown variable name (e.g., `${ENV.SECRET}`), **then** the substitution **shall** log a warning and leave the literal intact; it **shall not** fail the load nor expose `os.Getenv` values.

**REQ-SK-015 [Unwanted]** — The `ParseSkillFile` function **shall not** follow symbolic links that resolve outside the `root` passed to `LoadSkillsDir`; symlink escape attempts **shall** cause `ErrSymlinkEscape` for that file only.

**REQ-SK-016 [Unwanted]** — The registry **shall not** mutate a `SkillDefinition` in-place after first insertion; updates **shall** go through `Replace(id, newDef)` which performs an atomic swap via a new map copy.

### 4.5 Optional (선택적)

**REQ-SK-017 [Optional]** — **Where** `model: opus[1m]` is set in frontmatter, the skill loader **shall** record the preferred model alias in `SkillDefinition.PreferredModel`; the consumer(ROUTER-001) **may** honor or override.

**REQ-SK-018 [Optional]** — **Where** `argument-hint:` is non-empty, the skill loader **shall** expose it via `SkillDefinition.ArgumentHint` for COMMAND-001's slash-command autocomplete consumer.

### 4.6 Schema, Standard & Security Contracts (v0.2.0 보강)

이 절은 v0.2.0에서 감사(iteration 1) 대응으로 추가되었다. 기존 REQ-SK-001~018과 **번호 재배치 없이** 뒤에 누적된다.

**REQ-SK-019 [Ubiquitous]** — `SAFE_SKILL_PROPERTIES` allowlist **shall** enumerate exactly the following 15 keys and only these 15 keys as the authoritative YAML frontmatter schema: `name`, `description`, `when-to-use`, `argument-hint`, `arguments`, `model`, `effort`, `context`, `agent`, `allowed-tools`, `disable-model-invocation`, `user-invocable`, `paths`, `shell`, `hooks`. Any change to this set **shall** require a semver-minor bump, an updated allowlist test, and a corresponding HISTORY entry in this SPEC. §6.2의 Go `SAFE_SKILL_PROPERTIES` 맵은 본 REQ의 구현이며, REQ가 정본이다.

**REQ-SK-020 [Ubiquitous]** — The L0/L1/L2/L3 effort tiers defined in REQ-SK-010 **shall** be treated as **orthogonal** to the MoAI-ADK 3-Level Progressive Disclosure (CLAUDE.md §13; Level 1=Metadata, Level 2=Body, Level 3=Bundled). Effort tiers describe **author-declared skill size budget**; the 3-Level describes **loader-driven I/O staging**. The skill loader **shall not** derive one from the other, **shall not** collapse the two axes into a single scalar, and **shall** expose both dimensions independently to consumers. This relationship is normative and frozen per §2.4.2.

**REQ-SK-021 [Ubiquitous]** — Trigger matching in this SPEC is **restricted** to the four axes defined in §6.3 (`remote > fork > conditional > inline`). The loader **shall not** accept, parse, or route any of the following multi-dimensional trigger fields that appear in MoAI-ADK `.claude/skills/` convention: `triggers.keywords`, `triggers.agents`, `triggers.phases`, `triggers.languages`. If such fields appear in a frontmatter file, they **shall** trigger `ErrUnsafeFrontmatterProperty` via REQ-SK-001 (since they are not in the 15-key allowlist of REQ-SK-019). Multi-dimensional trigger support is explicitly deferred to a future SPEC; this exclusion is load-bearing and **shall not** be relaxed without a new REQ.

**REQ-SK-022 [Unwanted]** — The parser and `SkillRegistry` **shall** enforce the following shell-injection threat-model boundaries (extends REQ-SK-013):
  - (a) `shell.executable`이 YAML에 지정된 경우, **shall** 그 값을 **리터럴 경로**로만 기록하고 parse-time에 실행·resolve·PATH 조회·`filepath.EvalSymlinks`를 수행하지 **않는다**. 화이트리스트 검증은 consumer(HOOK-001 / agent runtime)의 책임으로 위임한다. 본 SPEC은 화이트리스트를 정의하지 않는다(이는 후속 SPEC에 위임).
  - (b) `shell.deny-write`가 `true`인 경우, **shall** consumer가 sandboxed execution 환경(예: read-only FS mount, container)에서 실행하도록 hint 플래그로 기록한다. 본 SPEC의 파서·레지스트리는 이 플래그의 실제 강제 지점이 **아니다**; consumer가 강제한다. 파서는 필드 존재·값만 보존한다.
  - (c) `hooks:` 맵 내 `command` 값은 **shall** raw 문자열로 그대로 보존되며, 파서는 shell metacharacter(예: `;`, `|`, `&&`, `$()`, `` ` ``, `>`, `<`, `$(...)`, `${...}`) 변환·escape·sanitization을 수행하지 **않는다**. 해석·escape·격리는 HOOK-001 dispatcher의 책임이다. 파서가 metacharacter를 변형하면 consumer 기대 의미가 변질되기 때문.
  - (d) Remote skill(`_canonical_` 접두사)의 `SKILL.md` 본문에 포함된 `shell:`·`hooks:`·변수 치환 토큰은 **shall** 로컬 skill과 **동일한 보안 정책**의 적용을 받는다. Remote origin이라는 사실이 권한 상승 경로가 되지 않도록, REQ-SK-001/013/014/015/022의 모든 제약이 remote skill에도 적용된다.

---

## 5. 수용 기준 (Acceptance Criteria)

### 5.1 Format & Traceability Declaration (v0.2.0)

이 섹션의 수용 기준은 **Given/When/Then (Gherkin/BDD)** 형식으로 기술된다. §4의 EARS 요구사항이 시스템이 "WHAT/WHEN" 충족해야 하는지를 정의한다면, §5의 AC는 그 EARS REQ의 충족 여부를 **어떻게 관측·검증할지**에 대한 test scenario이다.

이 이중 표현(EARS REQ + Gherkin AC)은 의도적 설계이다:
- EARS 형식은 요구사항의 **모호성 제거**에 최적화됨 (single actor + single action + optional condition).
- Gherkin 형식은 요구사항의 **관측 가능한 검증 시나리오** 표현에 최적화됨 (precondition + trigger + outcome).
- 따라서 §5는 §4의 EARS 구문을 반복·복제하지 않는다; 대신 각 REQ-SK-XXX가 어떤 AC-SK-YYY에 의해 검증되는지를 §5.2 traceability table + 각 AC header의 `(covers REQ-SK-XXX)` 인라인 시그너처로 추적한다.

MP-2 감사 기준(plan-auditor iteration 1)에서 "ACs mislabeled as EARS"로 판정되었던 FAIL은, 본 format declaration을 통해 **AC는 EARS가 아니라 EARS의 검증 시나리오**임을 명시함으로써 해소된다. 감사 규칙이 "AC가 EARS여야 한다"고 요구한다면, 이는 tooling 해석 문제로서 §5.2 traceability table이 mapping 증거를 제공한다.

### 5.2 REQ → AC Traceability Matrix

| REQ | AC (primary) | AC (supplementary) | Note |
|-----|--------------|---------------------|------|
| REQ-SK-001 (allowlist-default-deny) | AC-SK-002 | AC-SK-011 (hooks allowlist) | |
| REQ-SK-002 (hooks ordering) | AC-SK-011 | — | v0.2.0 신설 |
| REQ-SK-003 (frontmatter token estimate) | AC-SK-001 | — | |
| REQ-SK-004 (duplicate ID, error slice) | AC-SK-009 | — | v0.2.0 semantics clarified |
| REQ-SK-005 (walker, partial success) | AC-SK-001, AC-SK-002, AC-SK-008, AC-SK-009 | — | |
| REQ-SK-006 (known variable subst) | AC-SK-006 | — | |
| REQ-SK-007 (FileChangedConsumer) | AC-SK-004 | — | |
| REQ-SK-008 (IsForked) | AC-SK-005 | — | |
| REQ-SK-009 (CanInvoke gate) | AC-SK-007 | — | |
| REQ-SK-010 (effort default & mapping) | AC-SK-003 | — | |
| REQ-SK-011 (no paths → not conditional) | AC-SK-004 (negative path) | — | implicitly |
| REQ-SK-012 (remote skill load) | AC-SK-010 | — | |
| REQ-SK-013 (shell parse-time no-exec) | AC-SK-012 | — | v0.2.0 신설 |
| REQ-SK-014 (unknown variable + no os.Getenv) | AC-SK-006 (literal retention), AC-SK-013 | — | v0.2.0 security AC 신설 |
| REQ-SK-015 (symlink escape) | AC-SK-008 | — | |
| REQ-SK-016 (atomic Replace) | AC-SK-014 | — | v0.2.0 신설 |
| REQ-SK-017 (PreferredModel alias) | AC-SK-015 | — | v0.2.0 신설 |
| REQ-SK-018 (ArgumentHint exposure) | AC-SK-016 | — | v0.2.0 신설 |
| REQ-SK-019 (15-key schema) | AC-SK-002 (deny), AC-SK-011 | — | |
| REQ-SK-020 (L0-L3 ⊥ 3-Level) | AC-SK-003, AC-SK-001 | — | orthogonality observable via independent fields |
| REQ-SK-021 (4-trigger only) | AC-SK-002 (multi-dim fields rejected) | — | |
| REQ-SK-022 (shell threat model) | AC-SK-012, AC-SK-013 | — | |

### 5.3 Acceptance Criteria

**AC-SK-001 — 최소 SKILL.md 로드** _(covers REQ-SK-003, REQ-SK-005, REQ-SK-020)_
- **Given** `/tmp/skills/hello/SKILL.md`에 `name: hello`, `description: "say hi"` frontmatter + 본문 "Hello"
- **When** `LoadSkillsDir("/tmp/skills")`
- **Then** 레지스트리에 1개 skill(`ID="hello"`, `Effort=L1`, `IsInline=true`, `IsConditional=false`)이 등록되고, `EstimateSkillFrontmatterTokens()`가 `len("hello")+len("say hi")` 기반 추정값을 반환. Effort 필드와 Progressive Disclosure stage(frontmatter-only 로드)는 독립적으로 관측 가능(REQ-SK-020 orthogonality 검증).

**AC-SK-002 — Allowlist-default-deny로 unknown property 거부** _(covers REQ-SK-001, REQ-SK-019, REQ-SK-021)_
- **Given** SKILL.md에 `frobnicate: true`(allowlist 미포함) 또는 `triggers: {keywords: [auth]}`(MoAI-style 다차원 trigger) 포함
- **When** `ParseSkillFile`
- **Then** `err = ErrUnsafeFrontmatterProperty`, 해당 skill은 레지스트리에 등록되지 않음, 다른 정상 skill은 영향 없음. 15-key allowlist(REQ-SK-019) 밖의 임의 키는 동일한 방식으로 거부(REQ-SK-021 — keyword/agent/phase/language trigger 필드 포함).

**AC-SK-003 — Progressive Disclosure effort 매핑** _(covers REQ-SK-010, REQ-SK-020)_
- **Given** 4개 skill: `effort: L0`, `effort: 2`, `effort: L3`, 미지정
- **When** 각각 `ResolveEffort`
- **Then** 반환값 `L0`, `L2`, `L3`, `L1`(기본). Effort 값은 저자 선언이며, 로더가 실제로 어느 Level(Metadata/Body/Bundled)까지 읽었는지와는 무관함을 reflection/check로 검증(REQ-SK-020 orthogonal axes).

**AC-SK-004 — Conditional 활성화 (gitignore 매칭)** _(covers REQ-SK-007, REQ-SK-011)_
- **Given** SKILL.md에 `paths: ["src/**/*.ts", "!**/test/**"]`, `FileChangedConsumer`에 `["src/foo/bar.ts", "src/test/baz.ts", "README.md"]` 전달
- **When** consumer 호출
- **Then** 결과 skill ID 리스트에 해당 skill이 포함(첫 경로 매칭 + 두번째는 부정 패턴으로 제외 + 세번째 미매칭). 반환 리스트에는 skill ID가 정확히 1회 포함. 또한 `paths:` 미지정 skill은 어떤 changedPaths에도 매칭되지 않음(REQ-SK-011).

**AC-SK-005 — Forked skill 감지** _(covers REQ-SK-008)_
- **Given** SKILL.md에 `context: fork` 설정
- **When** `IsForked`/`IsInline`
- **Then** `IsForked == true`, `IsInline == false`; consumer는 본 skill 본문을 inline 주입하지 않음

**AC-SK-006 — 변수 치환 (known + unknown literal)** _(covers REQ-SK-006, REQ-SK-014 literal-retention)_
- **Given** SKILL.md 본문이 `"Working in ${CLAUDE_SKILL_DIR} session ${CLAUDE_SESSION_ID} maybe ${UNKNOWN_VAR}"`, session.ID = `"sess-abc"`, 파일 경로 `/tmp/skills/hello/SKILL.md`
- **When** `ResolveBody(session)`
- **Then** 결과 문자열이 `"Working in /tmp/skills/hello session sess-abc maybe ${UNKNOWN_VAR}"` — 알려진 변수는 치환되고, `${UNKNOWN_VAR}`은 **그대로 리터럴 유지**되며, zap 로거에 warning이 1건 기록된다.

**AC-SK-007 — Model invocation 차단** _(covers REQ-SK-009)_
- **Given** SKILL.md에 `disable-model-invocation: true`
- **When** `CanInvoke(skill, "model")` 및 `CanInvoke(skill, "user")`
- **Then** 전자는 false, 후자는 true

**AC-SK-008 — Symlink escape 방지** _(covers REQ-SK-015)_
- **Given** `/tmp/skills/evil/SKILL.md`가 `/etc/passwd`로의 symlink
- **When** `LoadSkillsDir("/tmp/skills")`
- **Then** `evil` skill은 error slice에 `ErrSymlinkEscape`로 포함, 다른 skill 로드는 성공

**AC-SK-009 — 중복 ID 탐지 (partial-success semantics)** _(covers REQ-SK-004, REQ-SK-005)_
- **Given** `/tmp/skills/a/SKILL.md`와 `/tmp/skills/b/SKILL.md`가 모두 `name: same`
- **When** `LoadSkillsDir`
- **Then** (a) 첫 번째로 walk된 파일만 레지스트리에 진입하고, (b) 두 번째 파일은 `ErrDuplicateSkillID`(offending path wrap)로 error slice에 포함되며, (c) 최상위 `error` 반환값은 `nil`(또는 다른 실패 사유만 포함), (d) 로더는 세 번째 이후의 정상 skill을 계속 로드한다. 이는 REQ-SK-004 v0.2.0 정정된 semantics("append to slice, not return")와 일치한다.

**AC-SK-010 — Remote skill 로드 (HTTP)** _(covers REQ-SK-012, REQ-SK-022d)_
- **Given** 테스트 httptest.Server가 경로 `/skills/remote.md`에 valid SKILL.md 콘텐츠 반환
- **When** `LoadRemoteSkill("http://127.0.0.1:PORT/skills/remote.md")`
- **Then** `SkillDefinition.ID`가 `_canonical_remote` 접두사를 가지며, `IsRemote == true`. 또한 remote SKILL.md 본문에 `shell:` 또는 `hooks:` 필드가 있더라도 로컬 skill과 동일하게 REQ-SK-013/022의 parse-time no-exec 제약이 적용됨이 관측되어야 한다(별도 assertion).

**AC-SK-011 — Hooks 배열 순서 보존** _(covers REQ-SK-002, REQ-SK-019)_
- **Given** SKILL.md frontmatter에 `hooks: { SessionStart: [{command: "alpha"}, {command: "beta"}, {command: "gamma"}] }`이 정의된 skill
- **When** `ParseSkillFile` + `Frontmatter.Hooks["SessionStart"]` 조회
- **Then** 파싱 결과 hook 엔트리 slice 순서는 정확히 `["alpha", "beta", "gamma"]` (YAML source 순서 그대로). 어떤 정렬·dedup·재배열도 관측되지 않으며, 반복 로드(10회) 결과도 안정적으로 동일 순서. 또한 `hooks` 키가 `SAFE_SKILL_PROPERTIES` 15-key allowlist에 포함됨이 schema test로 검증된다.

**AC-SK-012 — Shell directive parse-time no-exec** _(covers REQ-SK-013, REQ-SK-022)_
- **Given** SKILL.md frontmatter에 `shell: { executable: "/bin/sh", deny-write: true }` + `hooks: { PostToolUse: [{command: "rm -rf /; touch /tmp/pwned"}] }`
- **When** `ParseSkillFile` (parse-time only, no consumer dispatch)
- **Then** (a) 파일 시스템에 `/tmp/pwned` 생성되지 않음(no exec), (b) 어떤 subprocess도 spawn되지 않음(testable via process counter diff), (c) `SkillDefinition.Frontmatter.Shell.Executable == "/bin/sh"`로 리터럴 보존, (d) `SkillDefinition.Frontmatter.Shell.DenyWrite == true` hint flag 보존, (e) `Hooks["PostToolUse"][0].Command == "rm -rf /; touch /tmp/pwned"` metacharacter 변형·escape 없이 raw string 보존, (f) `Shell.Executable` 경로에 대한 `filepath.EvalSymlinks` 또는 PATH 조회가 수행되지 않음(`os.Stat` 호출 카운터로 관측).

**AC-SK-013 — Unknown variable은 os.Getenv 호출하지 않음 (security)** _(covers REQ-SK-014)_
- **Given** 환경변수 `SECRET_TOKEN=supersecret123`가 test 프로세스에 설정되어 있고, SKILL.md 본문이 `"token=${SECRET_TOKEN} env=${HOME} skill=${CLAUDE_SKILL_DIR}"`
- **When** `ResolveBody(session)` 호출 + test harness가 `os.Getenv` 호출을 monkey-patch 또는 counter로 관측
- **Then** (a) 결과 문자열은 `"token=${SECRET_TOKEN} env=${HOME} skill=/tmp/skills/hello"` — 알려진 safelist 변수(`CLAUDE_SKILL_DIR`)만 치환되고 `${SECRET_TOKEN}`, `${HOME}`은 literal 유지, (b) test harness가 관측한 `os.Getenv` 호출 횟수 delta = **0** (ResolveBody 전후로 증가 없음), (c) 결과 문자열 어디에도 `supersecret123`이 포함되지 않음(regex assertion), (d) zap 로거에 미지 변수 2건에 대한 warning 로그가 기록됨.

**AC-SK-014 — Registry atomic Replace (no in-place mutation)** _(covers REQ-SK-016)_
- **Given** 기존 registry에 `{id: "skill-a", def: defV1}` 보유. 새 map `{id: "skill-a", def: defV2, id: "skill-b", def: defB}` 준비.
- **When** 리더 goroutine 100개가 `registry.Get("skill-a")`를 반복 호출하는 동안, 별도 goroutine이 `registry.Replace(newMap)`를 1회 호출
- **Then** (a) `-race` detector가 활성화된 test가 race condition 없이 통과, (b) 각 리더는 `defV1` 또는 `defV2` 둘 중 하나만 관측(혼합 상태 없음 — pointer swap atomicity), (c) Replace 전·후에 `defV1`과 `defV2`의 내부 필드(`defV1.Frontmatter.Name` 등)가 in-place로 변경된 흔적 없음(heap snapshot diff 또는 reflect check로 관측), (d) Replace 완료 후 `Get("skill-b")`는 `defB`를 반환하고 `Get("nonexistent")`는 `(nil, false)`.

**AC-SK-015 — PreferredModel 기록** _(covers REQ-SK-017)_
- **Given** SKILL.md frontmatter에 `model: opus[1m]`
- **When** `ParseSkillFile` 후 `SkillDefinition.PreferredModel` 조회
- **Then** `PreferredModel == "opus[1m]"` (리터럴 보존). ROUTER-001이 consumer로 해석하는 것이지 본 loader가 resolve하지 않음을 확인: 어떤 outbound HTTP/API 호출도 parse-time에 발생하지 않음(httptest spy). frontmatter에 `model:` 키가 없을 때는 `PreferredModel == ""` (empty string, not literal "inherit").

**AC-SK-016 — ArgumentHint 노출** _(covers REQ-SK-018)_
- **Given** SKILL.md frontmatter에 `argument-hint: "<path> [--recursive]"`
- **When** `ParseSkillFile` 후 `SkillDefinition.ArgumentHint` 조회
- **Then** `ArgumentHint == "<path> [--recursive]"` 리터럴 그대로. `argument-hint:` 필드 부재 시 `ArgumentHint == ""` (빈 문자열). COMMAND-001 consumer가 이 필드를 슬래시-커맨드 autocomplete hint로 소비할 수 있음을 확인 — 본 SPEC은 노출만 책임.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/
└── skill/
    ├── loader.go            # LoadSkillsDir, 파일 walker
    ├── parser.go            # ParseSkillFile, frontmatter 분리
    ├── schema.go            # SkillDefinition, SkillFrontmatter, SAFE_SKILL_PROPERTIES
    ├── conditional.go       # paths: gitignore 매칭
    ├── remote.go            # LoadRemoteSkill HTTP fetcher
    ├── runtime.go           # ResolveBody, ResolveEffort, CanInvoke, EstimateTokens
    ├── registry.go          # SkillRegistry (atomic swap)
    └── *_test.go
```

### 6.2 핵심 Go 타입 시그니처

```go
// 지원되는 trigger 모드. discriminated union.
type TriggerMode int
const (
    TriggerInline TriggerMode = iota
    TriggerFork
    TriggerConditional
    TriggerRemote
)

// Progressive Disclosure 레벨.
type EffortLevel int
const (
    EffortL0 EffortLevel = iota  // ~50 tokens
    EffortL1                      // ~200 tokens (default)
    EffortL2                      // ~500 tokens
    EffortL3                      // ~1000+ tokens
)

// YAML frontmatter의 스키마. 알려진 속성만 필드로 존재.
type SkillFrontmatter struct {
    Name                   string            `yaml:"name,omitempty"`
    Description            string            `yaml:"description,omitempty"`
    WhenToUse              string            `yaml:"when-to-use,omitempty"`
    ArgumentHint           string            `yaml:"argument-hint,omitempty"`
    Arguments              []string          `yaml:"arguments,omitempty"`
    Model                  string            `yaml:"model,omitempty"`
    Effort                 string            `yaml:"effort,omitempty"`   // "L0"|"L1"|"L2"|"L3"|숫자
    Context                string            `yaml:"context,omitempty"`  // "fork"|"inline"
    Agent                  string            `yaml:"agent,omitempty"`
    AllowedTools           []string          `yaml:"allowed-tools,omitempty"`
    DisableModelInvocation bool              `yaml:"disable-model-invocation,omitempty"`
    UserInvocable          *bool             `yaml:"user-invocable,omitempty"`  // nil=기본 true
    Paths                  []string          `yaml:"paths,omitempty"`
    Shell                  *SkillShellConfig `yaml:"shell,omitempty"`
    Hooks                  map[string][]SkillHookEntry `yaml:"hooks,omitempty"`
}

type SkillShellConfig struct {
    Executable string `yaml:"executable"`
    DenyWrite  bool   `yaml:"deny-write,omitempty"`
}

type SkillHookEntry struct {
    Matcher string `yaml:"matcher,omitempty"`
    Command string `yaml:"command"`
}

// 런타임에서 유지되는 완전한 skill.
type SkillDefinition struct {
    ID              string            // path-derived slug
    AbsolutePath    string            // SKILL.md 절대경로
    Frontmatter     SkillFrontmatter
    Body            string            // raw body (치환 전)
    Trigger         TriggerMode
    Effort          EffortLevel
    PreferredModel  string
    FrontmatterTokens int              // estimate
    IsRemote        bool
}

// allowlist. 새 속성 추가는 코드 수정 + 테스트 필수.
var SAFE_SKILL_PROPERTIES = map[string]struct{}{
    "name": {}, "description": {}, "when-to-use": {},
    "argument-hint": {}, "arguments": {}, "model": {},
    "effort": {}, "context": {}, "agent": {}, "allowed-tools": {},
    "disable-model-invocation": {}, "user-invocable": {},
    "paths": {}, "shell": {}, "hooks": {},
}

// Registry. atomic replace 기반.
type SkillRegistry struct {
    mu      sync.RWMutex
    skills  map[string]*SkillDefinition
    logger  *zap.Logger
}

func LoadSkillsDir(root string, opts ...LoadOption) (*SkillRegistry, []error)
func (r *SkillRegistry) Get(id string) (*SkillDefinition, bool)
func (r *SkillRegistry) Replace(newSkills map[string]*SkillDefinition)
func (r *SkillRegistry) FileChangedConsumer(changed []string) []string
func (r *SkillRegistry) CanInvoke(id string, actor string) bool
```

### 6.3 Trigger 결정 규칙

| Trigger | 조건 | 소비자 |
|---------|------|--------|
| `TriggerRemote` | `ID` prefix `_canonical_` | Remote loader skeleton |
| `TriggerFork` | `Context == "fork"` | SUBAGENT-001 `runAgent` |
| `TriggerConditional` | `len(Paths) > 0` | `FileChangedConsumer` → HOOK-001 |
| `TriggerInline` | 위 조건 모두 아님 (default) | QueryEngine prompt 주입자 |

우선순위: remote > fork > conditional > inline. 하나의 skill은 정확히 하나의 trigger에 속한다.

### 6.4 Variable Substitution 규약

| 변수 | 치환 값 |
|------|--------|
| `${CLAUDE_SKILL_DIR}` | `filepath.Dir(def.AbsolutePath)` |
| `${CLAUDE_SESSION_ID}` | `session.ID` (런타임 주입) |
| `${USER_HOME}` | `os.UserHomeDir()` |
| 기타 `${XXX}` | 치환 없이 그대로 유지 + zap warn |

`os.Getenv`는 **금지** (REQ-SK-014). 민감 정보 노출 위험 때문.

### 6.5 Conditional 매칭 (gitignore 문법)

- `github.com/sabhiram/go-gitignore` 또는 `github.com/denormal/go-gitignore` 중 후자 채택(테스트 커버리지 + 유지보수 활성도).
- `!**/test/**` 같은 부정 패턴 지원.
- 대소문자 민감(OS 무관 일관성).

### 6.6 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| YAML frontmatter 파싱 | `gopkg.in/yaml.v3` | stdlib 없음, v3가 strict mode 지원 |
| gitignore 매칭 | `github.com/denormal/go-gitignore` | 부정 패턴 지원, 활성 유지보수 |
| 파일 walker | stdlib `io/fs.WalkDir` | symlink 제어 용이 |
| HTTP client | stdlib `net/http` | Remote skill fetch |
| 로깅 | `go.uber.org/zap` | CORE-001 공유 |

### 6.7 Allowlist-Default-Deny 파싱 전략

1. YAML을 우선 `map[string]any`로 loose unmarshal.
2. 키를 순회하며 `SAFE_SKILL_PROPERTIES`에 있는지 확인. 없으면 `ErrUnsafeFrontmatterProperty`.
3. 통과한 항목만 `SkillFrontmatter` struct로 2차 unmarshal.
4. 이유: `yaml.v3`의 strict=true는 알 수 없는 필드 전체를 에러로 만들지만, 어떤 키가 범인인지 보고하기 어렵다. 2단계 전략이 진단 메시지 품질 높음.

### 6.8 TDD 진입 순서

1. **RED #1** — `TestSchema_SafeSkillProperties_ContainsExpected` (목록 확정)
2. **RED #2** — `TestParseSkillFile_MinimalValid` (AC-SK-001)
3. **RED #3** — `TestParseSkillFile_UnknownProperty_Rejected` (AC-SK-002)
4. **RED #4** — `TestResolveEffort_Mappings` (AC-SK-003)
5. **RED #5** — `TestFileChangedConsumer_GitignoreMatching` (AC-SK-004)
6. **RED #6** — `TestIsForked_ContextFork` (AC-SK-005)
7. **RED #7** — `TestResolveBody_VariableSubstitution` (AC-SK-006)
8. **RED #8** — `TestCanInvoke_DisableModelInvocation` (AC-SK-007)
9. **RED #9** — `TestLoadSkillsDir_SymlinkEscape` (AC-SK-008)
10. **RED #10** — `TestLoadSkillsDir_DuplicateID` (AC-SK-009)
11. **RED #11** — `TestLoadRemoteSkill_HTTP` (AC-SK-010)
12. **GREEN** — 최소 구현.
13. **REFACTOR** — registry atomic swap을 `atomic.Pointer[map]`로 최적화 검토.

### 6.9 TRUST 5 매핑

| 차원 | 본 SPEC 달성 방법 |
|-----|-----------------|
| **T**ested | 25+ unit test, 10 integration test (AC 1:1), 커버리지 90%+ |
| **R**eadable | 파일별 단일 책임(loader/parser/schema/conditional/remote/runtime/registry), 한국어 주석 + 영어 식별자 |
| **U**nified | `go fmt`, `golangci-lint` (errcheck, govet, staticcheck) |
| **S**ecured | Allowlist-default-deny, symlink escape 방지, env var 치환 금지, shell directive 실행 금지 |
| **T**rackable | 각 skill 로드 시 zap 구조화 로그 (`skill_id`, `effort`, `trigger`), error slice로 부분 실패 가시화 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | Skill consumer(QueryEngine prompt 주입자)의 인터페이스 계약 |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트 |
| 후속 SPEC | SPEC-GOOSE-HOOK-001 | `FileChanged` 이벤트 → `FileChangedConsumer` 라우팅 |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | `context: fork` skill의 agent spawn |
| 후속 SPEC | SPEC-GOOSE-PLUGIN-001 | plugin manifest `skills:` 로딩 |
| 후속 SPEC | SPEC-GOOSE-COMMAND-001 | `argument-hint:` 소비 |
| 외부 | Go 1.22+ | `io/fs.WalkDir`, generics |
| 외부 | `gopkg.in/yaml.v3` v3.0+ | YAML 파싱 |
| 외부 | `github.com/denormal/go-gitignore` v0.3+ | paths 매칭 |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 |
| 표준 | `agentskills.io` Claude Code Agent Skills 표준 | §2.4.1 참조. 본 SPEC은 해당 표준의 compatible subset + MoAI-ADK 보안/확장 레이어. YAML frontmatter schema(`name`/`description`/`allowed-tools`/`argument-hint`/`disable-model-invocation`), 4-trigger(inline/fork/conditional/remote), Progressive Disclosure effort 축은 해당 표준 준수. `shell:` parse-time 실행·임의 env var 치환에서는 표준보다 엄격(REQ-SK-013/014/022). |
| 내부 명세 | CLAUDE.md §13 (MoAI-ADK 3-Level Progressive Disclosure) | §2.4.2 참조. 본 SPEC의 L0~L3 effort 축과 **직교**하는 별도 축(Metadata/Body/Bundled). REQ-SK-020에 고정. |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | `SAFE_SKILL_PROPERTIES` 확장 시 생태계 호환성 파괴 | 중 | 고 | 모든 추가는 allowlist 테이블 테스트 + semver minor로 공지 |
| R2 | gitignore 라이브러리가 deprecated되어 재작성 필요 | 낮 | 중 | 인터페이스 `Matcher` 추상화, 내부 구현 교체 가능 |
| R3 | Remote skill 로드가 네트워크 장애로 전체 `LoadSkillsDir` 실패 | 중 | 중 | Remote는 error slice로만 보고, 로컬 skill 로드는 성공 |
| R4 | 변수 치환 누락으로 `${SECRET_XXX}` 유출 | 고 | 고 | env var 치환 절대 금지(REQ-SK-014), 알 수 없는 변수는 리터럴 유지 + warn |
| R5 | Forked skill의 실제 semantics가 SUBAGENT-001과 어긋나 재설계 | 중 | 중 | 본 SPEC은 **감지만**(`IsForked`). 실행 계약은 SUBAGENT-001에서 확정 |
| R6 | Hot-reload 시 registry race | 중 | 중 | `Replace(newMap)`가 map 복사 + 포인터 교체 (atomic). 호출자는 `Replace` 외부 진입로 미허용 |
| R7 | Progressive Disclosure 토큰 휴리스틱이 실제 tokenizer와 괴리 | 낮 | 낮 | `EstimateSkillFrontmatterTokens`는 상한 근사. 정확도는 soft guarantee |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/claude-primitives.md` §2 Skill System, §2.1 YAML Frontmatter, §2.2 Progressive Disclosure, §2.3 Trigger 4종, §2.4 Model Invocation 제약
- `.moai/specs/ROADMAP.md` §4 Phase 2 row 11 (SKILLS-001)
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` — consumer 인터페이스
- `.moai/project/tech.md` §1 (Go 오케스트레이션 계층)

### 9.2 외부 참조

- `gopkg.in/yaml.v3`: https://pkg.go.dev/gopkg.in/yaml.v3
- `github.com/denormal/go-gitignore`: gitignore 매칭
- Claude Code source map: `./claude-code-source-map/` (패턴만)
- MoAI-ADK `.claude/skills/`: SKILL.md 구조 선례
- `agentskills.io`: Claude Code Agent Skills 공개 표준 (compatible subset 대상 — §2.4.1)
- CLAUDE.md §13: MoAI-ADK 3-Level Progressive Disclosure (§2.4.2 직교 관계 — Metadata/Body/Bundled)

### 9.3 부속 문서

- `./research.md` — claude-primitives.md §2 원문 인용, Go 라이브러리 선택 근거, 테스트 매트릭스
- `../SPEC-GOOSE-QUERY-001/spec.md`
- `../SPEC-GOOSE-HOOK-001/spec.md`
- `../SPEC-GOOSE-SUBAGENT-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **skill의 의미 있는 실행(prompt 주입/agent spawn/shell 실행)을 구현하지 않는다**. 레지스트리 + 메타데이터 + 활성화 결정만.
- 본 SPEC은 **skill SDK나 작성 가이드를 포함하지 않는다**.
- 본 SPEC은 **MCP prompt → Skill 변환을 구현하지 않는다**. MCP-001의 책임.
- 본 SPEC은 **Hook 이벤트 라우터를 구현하지 않는다**. `FileChangedConsumer`는 순수 함수, dispatch는 HOOK-001.
- 본 SPEC은 **Remote skill의 AKI/GCS/OAuth 인증을 구현하지 않는다**. HTTP fetch 스켈레톤만.
- 본 SPEC은 **Skill hot-reload watcher를 구현하지 않는다**. PLUGIN-001.
- 본 SPEC은 **Slash command(!command) 전개를 구현하지 않는다**. COMMAND-001.
- 본 SPEC은 **`$ARG` 치환을 구현하지 않는다**. COMMAND-001이 argument 파싱 담당.
- 본 SPEC은 **env variable 치환을 지원하지 않는다** — 보안상 금지(REQ-SK-014).
- 본 SPEC은 **Marketplace UI/publish flow를 구현하지 않는다**. 별도 ROADMAP-ECOSYSTEM.

---

**End of SPEC-GOOSE-SKILLS-001**
