# SPEC-GENIE-SKILLS-001 — Research & Porting Analysis

> **목적**: Claude Code의 Skill 시스템(TypeScript 원문) → GENIE Go 포팅 계약을 명확히 한다. `.moai/project/research/claude-primitives.md` §2의 설계를 본 SPEC REQ와 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/skill/` 단일 패키지.

---

## 1. 레포 현재 상태 스캔

```
/Users/goos/MoAI/AgentOS/
├── claude-code-source-map/  # TypeScript 참조 (commands/, components/skill/ 포함)
├── hermes-agent-main/       # Python 참조 (agent/prompts/)
└── .moai/specs/             # 기존 SPEC (QUERY-001 합의된 인터페이스)
```

- `internal/skill/` → **전부 부재**. Phase 2에서 신규 작성.
- Claude Code source map 내 `skills/` 디렉토리 + `parseSkill*.ts` 파일 확인. **언어 상이로 직접 포트 대상이 아니다** — 스키마와 trigger 결정 로직만 번역.

**결론**: GREEN 단계는 `internal/skill/` 7개 파일 **zero-to-one** 신규 작성.

---

## 2. claude-primitives.md §2 원문 인용 → REQ 매핑

### 2.1 YAML Frontmatter (§2.1)

원문 발췌:

```yaml
---
name: custom-name                # 선택, 기본 디렉토리명
description: |                   # 선택, markdown
when-to-use: |                   # 선택, 모델 노출
argument-hint: "--flag value"
arguments: [arg1, arg2]
model: opus[1m]                  # 선택
effort: L2                       # L0/L1/L2/L3 또는 정수
context: fork                    # "fork" | inline default
agent: expert
allowed-tools: [bash:readonly, read]
disable-model-invocation: false
user-invocable: true
paths: [src/**/*.ts, "!**/test/**"]
shell: {executable: bash, deny-write: true}
hooks:
  SessionStart: [{command: setup.sh}]
  PostToolUse: [{command: log.sh}]
---
```

| 원문 키 | 본 SPEC REQ | Go 타입 필드 |
|---|---|---|
| `name` | REQ-SK-004 (ID uniqueness) | `Name string` |
| `description`, `when-to-use` | REQ-SK-003 (tokens) | `Description string`, `WhenToUse string` |
| `effort` | REQ-SK-010 (L0~L3 매핑) | `Effort string` → `EffortLevel` |
| `context: fork` | REQ-SK-008 | `Context string` → `TriggerFork` |
| `paths` | REQ-SK-011, REQ-SK-007 | `Paths []string` |
| `disable-model-invocation` | REQ-SK-009 | `DisableModelInvocation bool` |
| `shell` | REQ-SK-013 (실행 금지) | `Shell *SkillShellConfig` |
| 미등록 키 | REQ-SK-001 (allowlist-deny) | `SAFE_SKILL_PROPERTIES` |

### 2.2 Progressive Disclosure (§2.2)

원문 표:

| Level | 토큰 | 용도 |
|-------|-----|------|
| L0 | ~50 | 매우 경량, 빠른 응답 |
| L1 | ~200 | 표준 (default) |
| L2 | ~500 | 중급 |
| L3 | ~1000+ | 고급 |

`estimateSkillFrontmatterTokens()`: frontmatter만 파싱(name + description + whenToUse), 전체 콘텐츠는 호출 시만 로드.

→ **REQ-SK-003**이 이 휴리스틱을 Go로 포팅. 토큰 정확도는 soft guarantee(실제 tokenizer는 ADAPTER-001/COMPRESSOR-001).

### 2.3 Trigger 4종 (§2.3)

원문:

1. **Inline Skill** (기본): processPromptSlashCommand로 전개
2. **Forked Skill** (`context:fork`): executeForkedSkill → runAgent
3. **Conditional Skill** (`paths:`): FileChanged hook → activateConditionalSkillsForPaths
4. **Remote Skill** (`EXPERIMENTAL_SKILL_SEARCH`): `_canonical_{slug}` 접두사 → AKI/GCS에서 SKILL.md 로드

→ **REQ-SK-005~REQ-SK-012**가 각각을 구현. 우선순위는 remote > fork > conditional > inline (§6.3).

### 2.4 Model Invocation 제약 (§2.4)

원문 `SkillTool.checkPermissions` 알고리즘:

1. deny 규칙 검사 ("skill-name" 또는 "prefix:*")
2. `disableModelInvocation` 플래그 체크
3. `SAFE_SKILL_PROPERTIES` allowlist (새 프로퍼티 기본 deny)
4. allow 규칙 또는 자동 허가

→ **REQ-SK-001, REQ-SK-009**가 2·3단계를 구현. 1·4단계(explicit deny/allow 규칙)는 PLUGIN-001에서 plugin-level 정책으로 확장.

---

## 3. Go 포팅 매핑표 (claude-primitives.md §7)

| Claude Code (TS) | GENIE (Go) 패키지 | 결정 |
|---|---|---|
| `skills/` 디렉토리 walker | `internal/skill/loader.go:LoadSkillsDir` | stdlib `io/fs.WalkDir` |
| `parseSkill*` | `internal/skill/parser.go:ParseSkillFile` | `yaml.v3` + 2-pass loose→strict |
| Trigger 결정 | `internal/skill/schema.go` helpers | `IsInline`/`IsForked`/`IsConditional`/`IsRemote` |
| `activateConditionalSkillsForPaths` | `FileChangedConsumer` | 순수 함수, HOOK-001이 dispatch |
| `estimateSkillFrontmatterTokens` | `EstimateSkillFrontmatterTokens` | 휴리스틱 근사 |
| `_canonical_{slug}` remote loader | `internal/skill/remote.go:LoadRemoteSkill` | HTTP fetch (인증은 Phase 5+) |
| `SAFE_SKILL_PROPERTIES` allowlist | `schema.go`의 package-level `var` map | default-deny |

---

## 4. Go 이디엄 선택 (상세 근거)

### 4.1 AsyncIterator → eager walk

Claude Code의 `loadSkillsDir`은 async iterator로 점진 로드. Go는 고루틴 없이 `io/fs.WalkDir` 동기 호출로 충분. 5000개 skill 가정 시 평균 <200ms (SSD 기준).

### 4.2 Allowlist-default-deny 2-pass 파싱

`yaml.v3`의 `KnownFields(true)`만 사용하면 알 수 없는 키 존재 시 에러 메시지가 모호. 2-pass:

1. Pass 1: `map[string]any` loose unmarshal.
2. Pass 2: key 순회 + allowlist 검증 + 에러 key 이름 포함하여 반환.
3. Pass 3: 검증된 map을 `SkillFrontmatter` struct로 2차 marshal→unmarshal.

3-pass가 다소 비효율이지만 skill 파일은 소량(<=10KB), 로드는 초기화 1회. 진단 메시지 품질 우선.

### 4.3 gitignore 매칭 라이브러리 결정

후보:

| 라이브러리 | Stars | 최근 커밋 | 부정 패턴(`!`) | 결정 |
|---|---|---|---|---|
| `github.com/denormal/go-gitignore` | ~50 | 2023 | 지원 | **채택** |
| `github.com/sabhiram/go-gitignore` | ~200 | 2022 | 지원 | 백업 |
| 자체 구현 | — | — | — | 시간 투자 불필요 |

두 라이브러리 모두 Apache 2.0 또는 MIT. `denormal`이 `Matcher` 인터페이스가 깔끔해 채택. 추후 교체 가능하도록 `internal/skill/conditional.go`에 `type PathMatcher interface { Match(string) bool }` 추상화.

### 4.4 Registry concurrency 모델

- 기본: `sync.RWMutex` + `map[string]*SkillDefinition`.
- Replace는 map 복사 + full lock swap(atomic swap 효과).
- 읽기는 RLock.
- `atomic.Pointer[map[...]]`로 lock-free 최적화는 REFACTOR 단계에서 필요 시 적용.

### 4.5 Remote skill loader (최소 기능)

- `net/http` `http.Client{Timeout: 10s}`로 GET.
- 응답 body 크기 제한: 1MB (`http.MaxBytesReader`).
- 인증 헤더 주입 hook만 제공: `RemoteAuthProvider interface { Headers(uri string) http.Header }`.
- 실제 인증(OAuth, Bearer, AKI)은 PLUGIN-001 / Phase 5+ 책임.

---

## 5. 참조 가능한 외부 자산 분석

### 5.1 Claude Code TypeScript (`./claude-code-source-map/`)

```
claude-code-source-map/
├── commands/                # slash command (COMMAND-001 관련)
├── components/              # UI (본 SPEC 비관련)
```

- Skill 로딩 함수는 grep 시 다수 파일에 분산(`parseSkillFrontmatter.ts`, `loadSkillsDir.ts` 등). 원문 구조를 그대로 이식하기보다는 `claude-primitives.md` §2의 요약을 primary source로 삼는다.
- **직접 포트 대상 없음**. 패턴 + 스키마만 번역.

### 5.2 MoAI-ADK `.claude/skills/` (본 레포)

- 실제 SKILL.md 예제: `/.claude/skills/moai-foundation-core/SKILL.md` 등.
- frontmatter 최소 세트(name/description) 사용 중. 본 SPEC의 full 스키마는 MoAI-ADK의 진화형.
- 테스트 fixture로 재활용: `internal/skill/testdata/`에 축소 사본을 복제하여 AC-SK-001 등에 사용.

### 5.3 Hermes Agent (`./hermes-agent-main/`)

- `agent/prompts/` 내 Python 문자열 템플릿. Progressive Disclosure 개념 **없음**.
- 본 SPEC은 Hermes 자산 **재사용 없음**. 설계 참조만.

---

## 6. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 결정 근거 |
|------|-----|-----------|---------|
| `gopkg.in/yaml.v3` | v3.0.1+ | ✅ YAML 파싱 | stdlib 대체 없음 |
| `github.com/denormal/go-gitignore` | v0.3+ | ✅ paths 매칭 | 부정 패턴 지원 |
| `go.uber.org/zap` | v1.27+ | ✅ 로깅 | CORE-001 계승 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 | CORE-001 계승 |
| Go stdlib `io/fs` | 1.22+ | ✅ WalkDir | 표준 |
| Go stdlib `net/http` | 1.22+ | ✅ remote fetch | 표준 |

**의도적 미사용**:

- `fsnotify`: hot-reload watcher는 PLUGIN-001.
- `github.com/golang/protobuf`: SKILL.md는 YAML, proto 무관.
- tokenizer(`tiktoken-go`): Progressive Disclosure는 길이 근사. 정확 토큰은 ADAPTER-001.

---

## 7. 테스트 전략 (TDD RED → GREEN)

### 7.1 Unit 테스트 (20~28개)

**Schema 레이어**:
- `TestSAFE_SKILL_PROPERTIES_Exact` — 15개 allowlist 확정
- `TestSkillFrontmatter_YAMLRoundtrip` — 모든 필드 marshal/unmarshal 일관성
- `TestResolveEffort_Mappings` — L0/L1/L2/L3/정수/기본값 5 케이스

**Parser 레이어**:
- `TestParseSkillFile_NoFrontmatter_Empty` — `---`가 없는 파일은 빈 frontmatter
- `TestParseSkillFile_UnsafeProperty_Rejected` — REQ-SK-001
- `TestParseSkillFile_InvalidYAML_Error` — 손상된 YAML
- `TestParseSkillFile_PreservesHooksOrder` — REQ-SK-002

**Runtime 레이어**:
- `TestResolveBody_KnownVariables` — REQ-SK-006
- `TestResolveBody_UnknownVariable_Warn` — REQ-SK-014
- `TestCanInvoke_ActorMatrix` — model/user/hook × true/false
- `TestEstimateSkillFrontmatterTokens_LengthBased` — REQ-SK-003

**Conditional 레이어**:
- `TestFileChangedConsumer_PositivePatterns`
- `TestFileChangedConsumer_NegationPatterns` — `!**/test/**`
- `TestFileChangedConsumer_NoPaths_NoMatch` — REQ-SK-011

**Loader 레이어**:
- `TestLoadSkillsDir_MultipleFiles_Ordered`
- `TestLoadSkillsDir_SymlinkEscape_Detected` — REQ-SK-015
- `TestLoadSkillsDir_DuplicateID_FirstWins` — REQ-SK-004

**Registry 레이어**:
- `TestRegistry_Replace_AtomicSwap` — race detector
- `TestRegistry_Get_Concurrent` — RWMutex 검증

### 7.2 Integration 테스트 (AC 1:1, build tag `integration`)

| AC | Test |
|---|---|
| AC-SK-001 | `TestSkills_LoadDir_MinimalValid` |
| AC-SK-002 | `TestSkills_LoadDir_UnknownPropertyRejected` |
| AC-SK-003 | `TestSkills_ResolveEffort_AllMappings` |
| AC-SK-004 | `TestSkills_FileChanged_GitignoreMatching` |
| AC-SK-005 | `TestSkills_IsForked_ContextFork` |
| AC-SK-006 | `TestSkills_ResolveBody_VariableSubstitution` |
| AC-SK-007 | `TestSkills_CanInvoke_DisableModelInvocation` |
| AC-SK-008 | `TestSkills_LoadDir_SymlinkEscapeReported` |
| AC-SK-009 | `TestSkills_LoadDir_DuplicateID` |
| AC-SK-010 | `TestSkills_LoadRemoteSkill_HTTP` — httptest.Server 사용 |

### 7.3 커버리지 목표

- `internal/skill/`: 90%+.
- `-race` 필수(Registry concurrent access 검증).

---

## 8. 오픈 이슈

1. **Remote skill 인증**: 본 SPEC은 `RemoteAuthProvider` 인터페이스만. 실제 OAuth/AKI는 PLUGIN-001 통합 시 결정.
2. **Skill hot-reload**: `fsnotify` 통합은 PLUGIN-001의 책임. 본 SPEC은 `Replace` API만 제공.
3. **MCP prompts/list → Skill 변환**: 스키마 차이(MCP prompt는 argument 중심, Skill은 context/effort 중심). MCP-001에서 어댑터 레이어 제공.
4. **`model: inherit` 의미**: 원문은 "부모 에이전트 모델 계승"이나, inline skill은 부모가 없음. 본 SPEC은 `PreferredModel=""`로 기록, ROUTER-001이 최종 해석.
5. **Progressive Disclosure 토큰 휴리스틱 정확도**: 실제 tokenizer(COMPRESSOR-001 결정)와 오차 발생 시 L1→L2 승격 기준 재조정 여부.

---

## 9. 구현 규모 예상

| 영역 | 파일 수 | 신규 LoC | 테스트 LoC |
|---|---|---|---|
| `loader.go` + walker | 1 | 200 | 300 |
| `parser.go` (2-pass) | 1 | 180 | 280 |
| `schema.go` (allowlist) | 1 | 150 | 200 |
| `conditional.go` | 1 | 120 | 250 |
| `remote.go` | 1 | 100 | 150 |
| `runtime.go` | 1 | 180 | 280 |
| `registry.go` | 1 | 120 | 240 |
| **합계** | **7** | **~1,050** | **~1,700** |

테스트 비율: 62% (TDD 1:1+ 충족).

---

## 10. 결론

- **상속 자산**: TypeScript source map은 설계 참조, 직접 포트 없음. `claude-primitives.md` §2가 primary source.
- **핵심 결정**:
  - Allowlist-default-deny 2-pass parsing (진단 품질 우선).
  - Trigger 우선순위: remote > fork > conditional > inline.
  - Variable substitution은 safelist(`CLAUDE_SKILL_DIR`, `CLAUDE_SESSION_ID`, `USER_HOME`)만 — env var 절대 금지.
  - gitignore 매칭: `denormal/go-gitignore` 채택.
- **Go 버전**: 1.22+ (CORE-001과 정합).
- **다음 단계 선행 요건**: 없음(QUERY-001이 완료되면 본 SPEC 착수 가능). HOOK-001의 `FileChanged` 계약과 본 SPEC의 `FileChangedConsumer` 서명 교차 검증이 GREEN 전 최종 체크.
