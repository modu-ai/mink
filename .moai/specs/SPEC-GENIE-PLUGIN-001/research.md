# SPEC-GENIE-PLUGIN-001 — Research & Porting Analysis

> **목적**: Claude Code의 `manifest.json` 기반 plugin host + MCPB 번들 → GENIE Go 포팅 계약. `.moai/project/research/claude-primitives.md` §6을 본 SPEC REQ와 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/plugin/` 단일 패키지.

---

## 1. 레포 현재 상태 스캔

```
/Users/goos/MoAI/AgentOS/
├── claude-code-source-map/    # utils/plugins/ TS 참조
├── .moai/specs/               # 4 primitive SPEC(SKILLS/MCP/SUBAGENT/HOOK-001) 확정
└── claude-code-source-map/commands/  # 참조용 (slash command 연계)
```

- `internal/plugin/` → **전부 부재**. Phase 2 마지막 퍼즐.
- 4 primitive SPEC의 entry point 계약이 본 SPEC의 orchestrator 코드 작성 전제조건.
- MCPB 실물 샘플 없음 — DXT 공개 스펙 기반 fixture 자체 제작.

**결론**: GREEN 단계는 `internal/plugin/` 8개 파일 + 4 primitive의 `LoadPluginXxx` 인터페이스 연동.

---

## 2. claude-primitives.md §6 원문 인용 → REQ 매핑

### 2.1 manifest 스키마 (§6)

원문:

```json
{
  "name": "plugin-name",
  "description": "...",
  "version": "1.0.0",
  "skills": [{"id": "...", "path": "skills/.../SKILL.md"}],
  "agents": [{"id": "...", "path": "agents/....md"}],
  "mcpServers": [{"name": "...", "command": "node", "args": [...]}],
  "commands": [{"name": "...", "description": "..."}],
  "hooks": {
    "SessionStart": [
      {"matcher": "**/*.ts", "hooks": [{"command": "setup.sh"}]}
    ]
  }
}
```

→ **본 SPEC §6.2의 `PluginManifest` 타입이 1:1 포팅**. REQ-PL-001, REQ-PL-004에 필드 검증 규칙 수록.

### 2.2 Plugin Loading Pipeline (§6)

원문:

```
1. Discovery (~/.claude/plugins/, .claude/plugins/, 마켓플레이스)
2. Validation (manifest 스키마, 이름 예약어, hook 스키마)
3. Primitive 로드 (loadPlugin{Skills,Agents,Commands,Hooks})
4. Runtime Registration (atomic swap)
```

| 원문 단계 | 본 SPEC REQ / 구현 |
|---|---|
| Discovery | 3-level (user/project/marketplace); REQ-PL-008 (enabled flag) |
| Validation | REQ-PL-001, REQ-PL-004, REQ-PL-013, REQ-PL-016 |
| Primitive 로드 | REQ-PL-005 (walker), 4 primitive entry point 호출 |
| Atomic Registration | REQ-PL-003, REQ-PL-007 (ClearThenRegister) |

### 2.3 MCPB 파일 (§6)

원문:

> **MCPB 파일** (복합 서버): DXT 매니페스트 + user config variables 치환.

→ **REQ-PL-006, REQ-PL-009, REQ-PL-014**.

MCPB의 DXT 매니페스트 상세:

- `userConfigVariables`: 사용자가 설정할 값 정의 (API 키, 엔드포인트 등).
- `${VAR}` 치환: manifest + text 파일에서 lookup.
- Zip 해제 시 zip slip 방지 필수.

### 2.4 설계 원칙 (§10) — Composable Plugins

원문:

> **Composable Plugins**: 4 primitive 각각 plugin-loadable, manifest.json 중심.

→ 본 SPEC의 orchestrator 구조. 각 primitive는 `Load` entry point만 노출하고, plugin의 존재 여부에 무관하게 standalone 동작 가능.

---

## 3. Go 포팅 매핑표 (claude-primitives.md §7)

| Claude Code (TS) | GENIE (Go) | 결정 |
|---|---|---|
| `utils/plugins/` (Zod validator) | `internal/plugin/validator.go` | Go struct tag + custom validator |
| `utils/plugins/walker` | `internal/plugin/walker.go` | stdlib `filepath.Walk` |
| `utils/plugins/loader` | `internal/plugin/loader.go` | staged/commit/rollback 패턴 |
| `utils/plugins/mcpb` | `internal/plugin/mcpb.go` | stdlib `archive/zip` |
| Atomic swap | `atomic.Pointer[map]` | Go 1.22+ |

---

## 4. Go 이디엄 선택 (상세 근거)

### 4.1 Staged/Commit/Rollback 트랜잭션 패턴

**문제**: `LoadPlugin`이 5개 primitive(skills, agents, mcpServers, commands, hooks)를 순차 로드. 3번째 primitive에서 실패 시 1·2번째 등록된 내용을 되돌려야 함.

**대안 1 — 순수 함수형 staged**: 각 primitive의 Load가 단순히 "staged map"을 반환. Orchestrator가 모두 성공 시 한 번에 merge.
- **문제**: primitive 내부 dependency(e.g., agent가 등록된 skill을 참조)는 staged 상태로 검증 불가.

**대안 2 — 3-phase 인터페이스**: 각 primitive가 `(staged, commit, rollback)` 3 함수 반환.
- **장점**: staged 상태에서 primitive가 자신의 내부 검증 수행, commit은 atomic swap, rollback은 staged 폐기.
- **채택**.

**인터페이스 서명 합의(본 SPEC의 요구사항, 4 primitive SPEC이 후속 구현)**:

```go
type PrimitiveLoad interface {
    Stage(manifest PluginManifest, baseDir string) (stagedHandle, error)
    Commit(handle stagedHandle) error
    Rollback(handle stagedHandle)
}
```

### 4.2 Atomic `ClearThenRegister` 조정

각 primitive registry는 자체적으로 `atomic.Pointer[map]` 기반 swap 제공(HOOK-001의 REQ-HK-003와 동일). 본 SPEC은 **순서 보장**만 담당:

```
snapshot_old_all = snapshot of all primitive registries
new_plugins = scan + load (staged)
if all staged successfully:
  for each primitive in (Skills, Agents, Hooks, MCP, Commands):
    primitive.Commit()  // atomic swap per-primitive
  plugin_registry.swap(new_instances)
else:
  for each staged: Rollback()
```

**Cross-registry gap**: commit 순서 중간에 observer가 "Skills는 new, Agents는 old"를 볼 수 있음. 이는 REQ-PL-003에서 per-registry consistency만 보장으로 명시.

### 4.3 MCPB zip 해제 보안

```go
func extractMCPB(zipPath, targetDir string) error {
    r, err := zip.OpenReader(zipPath)
    if err != nil { return err }
    defer r.Close()

    absTarget, _ := filepath.Abs(targetDir)
    for _, f := range r.File {
        // 1. symlink 거부
        if f.Mode()&os.ModeSymlink != 0 {
            return fmt.Errorf("%w: symlink in mcpb", ErrZipSlip)
        }
        // 2. path escape 검사
        dest := filepath.Join(targetDir, f.Name)
        absDest, _ := filepath.Abs(dest)
        if !strings.HasPrefix(absDest, absTarget+string(os.PathSeparator)) && absDest != absTarget {
            return fmt.Errorf("%w: %s escapes target", ErrZipSlip, f.Name)
        }
        // 3. 크기 제한 (각 파일 10MB, 총 100MB)
        if f.UncompressedSize64 > 10*1024*1024 {
            return ErrMCPBFileTooLarge
        }
        // ... 실제 추출
    }
    return nil
}
```

### 4.4 `${VAR}` 치환 정책

```go
var varPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

func substituteUserConfig(content string, vars map[string]string) (string, error) {
    var missing []string
    result := varPattern.ReplaceAllStringFunc(content, func(match string) string {
        name := match[2:len(match)-1]  // strip ${ }
        if val, ok := vars[name]; ok {
            return val
        }
        missing = append(missing, name)
        return match  // 유지 (에러 후 반환)
    })
    if len(missing) > 0 {
        return "", fmt.Errorf("%w: %v", ErrMissingUserConfigVariable, missing)
    }
    return result, nil
}
```

치환 대상 파일 타입 제한 (R3 완화):

- `*.json` (manifest)
- `*.yaml`, `*.yml`
- `*.md` (SKILL.md, agent 정의)

바이너리 / `*.zip` / `*.mcpb` 내부는 미치환.

### 4.5 Semver 검증

```go
import "github.com/Masterminds/semver/v3"

func validateVersion(v string) error {
    _, err := semver.StrictNewVersion(v)
    if err != nil { return fmt.Errorf("%w: %s", ErrInvalidManifest, err) }
    return nil
}
```

`StrictNewVersion`은 `v1.0` 같은 단축 형을 거부하고 `1.0.0` 강제.

---

## 5. 참조 가능한 외부 자산 분석

### 5.1 Claude Code TypeScript

- `utils/plugins/` 디렉토리: Zod validator + async loader 조합. 직접 포트 대상 아님.
- `components/plugins/` UI 컴포넌트: 본 SPEC 비관련.
- **직접 포트 대상 없음**.

### 5.2 Anthropic DXT (외부 공개 스펙)

https://github.com/anthropics/dxt 참조 (가정 경로; 실제 스펙은 MCPB 공식 문서 참조):

- `dxt-manifest.json`의 `userConfigVariables` 스키마: `[{"name": string, "required": bool, "type": "string|number|secret", "default": any?, "description": string?}]`.
- 본 SPEC은 v0.1에서 `name`, `required`, `default`만 지원. `type`과 `description`은 metadata로 저장하되 활용 안 함.

### 5.3 4 Primitive SPEC의 entry point 계약

각 primitive SPEC이 본 SPEC에 약속하는 인터페이스 (4 primitive SPEC의 §3 IN SCOPE 또는 §7 의존성에서 확인):

- SKILLS-001: plugin 로드 시 `LoadSkillsDir`의 `root`를 plugin path로 사용 가능(§3.1). 별도 staged API는 본 SPEC에서 명시 필요 → **추가 통합 포인트**(4 primitive SPEC v0.2에서 staged 인터페이스 확정).
- MCP-001: `mcpServers` 목록을 `ConnectToServer` 호출 대상으로 위임.
- SUBAGENT-001: `LoadAgentsDir`의 root를 plugin의 `agents/` 경로로.
- HOOK-001: `PluginHookLoader` 인터페이스(§3.1 IN SCOPE) — 본 SPEC이 이 인터페이스 구현체를 제공하는 것으로 해석.

→ **오픈 이슈 3**: staged/commit/rollback 인터페이스가 4 primitive SPEC의 v0.1에 없음. v0.2에서 확정 필요. 본 SPEC은 이를 가정하고 진행.

---

## 6. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 결정 근거 |
|------|-----|-----------|---------|
| `github.com/Masterminds/semver/v3` | v3.2+ | ✅ 버전 검증 | 업계 표준 |
| `gopkg.in/yaml.v3` | v3.0+ | ✅ plugins.yaml | CONFIG-001 공유 |
| `go.uber.org/zap` | v1.27+ | ✅ 로깅 | CORE-001 공유 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 | CORE-001 공유 |
| Go stdlib `encoding/json` | 1.22+ | ✅ manifest | 표준 |
| Go stdlib `archive/zip` | 1.22+ | ✅ MCPB 추출 | 표준 |
| Go stdlib `regexp` | 1.22+ | ✅ ${VAR} 치환 | 표준 |
| Go stdlib `sync/atomic` | 1.22+ | ✅ atomic.Pointer | 표준 |

**의도적 미사용**:

- `hashicorp/go-plugin`: gRPC IPC 기반. 본 SPEC의 plugin은 in-process(primitive 등록).
- `go-github/go-github`: 마켓플레이스는 v0.1에서 stub.
- `go-digest`: 코드 서명 검증은 Phase 5+.

---

## 7. 테스트 전략 (TDD RED → GREEN)

### 7.1 Unit 테스트 (22~28개)

**Manifest parser**:
- `TestPluginManifest_ParseMinimal`
- `TestPluginManifest_MissingName_Error`
- `TestPluginManifest_InvalidSemver_Error`
- `TestPluginManifest_UnknownField_StrictReject`

**Validator**:
- `TestValidator_ReservedName_{genie,claude,mcp,_prefix}` (4 case)
- `TestValidator_UnknownHookEvent`
- `TestValidator_CredentialsInURI_{http,https,ws,wss}` (4 case)
- `TestValidator_PathTraversal_Rejected`
- `TestValidator_AllHookEventsValid`

**MCPB loader**:
- `TestMCPB_Extract_ValidZip`
- `TestMCPB_Extract_ZipSlip_Rejected`
- `TestMCPB_Extract_Symlink_Rejected`
- `TestMCPB_Extract_FileTooLarge_Rejected`
- `TestMCPB_Substitute_AllVars`
- `TestMCPB_Substitute_MissingRequired_Error`
- `TestMCPB_Substitute_UnknownVar_NoChange`

**Registry**:
- `TestRegistry_Load_Unique_Success`
- `TestRegistry_Load_Duplicate_Error`
- `TestRegistry_Unload_Removes`
- `TestRegistry_ClearThenRegister_AtomicRace` (race detector)

**Config**:
- `TestPluginsYAML_ParseValid`
- `TestPluginsYAML_DisabledSkip`
- `TestPluginsYAML_UserConfigVars_PerPlugin`

### 7.2 Integration 테스트 (AC 1:1, `integration`)

| AC | Test | 특수 요구 |
|---|---|---|
| AC-PL-001 | `TestPlugin_LoadMinimalManifest` | fixture 디렉토리 |
| AC-PL-002 | `TestPlugin_LoadAllPrimitives` | stub primitives registered |
| AC-PL-003 | `TestPlugin_ReservedName_Rejected` | |
| AC-PL-004 | `TestPlugin_UnknownHookEvent_Rejected` | |
| AC-PL-005 | `TestPlugin_DuplicateName_Rejected` | |
| AC-PL-006 | `TestPlugin_MCPB_UserConfigSubstitution` | 실제 zip fixture |
| AC-PL-007 | `TestPlugin_MCPB_MissingRequiredVar` | |
| AC-PL-008 | `TestPlugin_MCPB_ZipSlip_Rejected` | 악의 fixture |
| AC-PL-009 | `TestPlugin_ReloadAll_AtomicObservability` | goroutine stress |
| AC-PL-010 | `TestPlugin_CredentialsInURI_Rejected` | |
| AC-PL-011 | `TestPlugin_EnabledFalse_Skipped` | |
| AC-PL-012 | `TestPlugin_PartialFailure_Rollback` | primitive fail inject |

### 7.3 Fixture 전략

- `testdata/plugins/` — 실제 디렉토리 형태 mini plugin 세트.
- `testdata/mcpb/` — fixture `.mcpb` 파일 (생성 스크립트 제공).
- `testdata/evil-mcpb/` — zip slip 공격 fixture (보안 테스트).
- Stub primitive Load: `internal/plugin/testsupport/stub_primitives.go`로 4 primitive의 fake 구현.

### 7.4 커버리지 목표

- `internal/plugin/`: 90%+.
- `mcpb.go`: 95%+ (보안 크리티컬).
- `-race` 필수 (ClearThenRegister).

---

## 8. 오픈 이슈

1. **4 primitive entry point 인터페이스**: `Stage/Commit/Rollback` 3-phase 패턴이 각 primitive SPEC v0.1에 미포함. v0.2에서 확정해야 본 SPEC GREEN 가능 — 또는 본 SPEC이 primitive별 adapter 작성 책임.
2. **Cross-registry 2PC 필요성**: REQ-PL-003이 per-registry 원자성만 약속. 사용자 시나리오에서 cross-registry gap이 실제 문제되는지 측정 후 Phase 5+에서 2PC 도입 판단.
3. **Permissions 필드 enforcement**: `manifest.permissions: ["network", "filesystem:project"]`의 집행 주체는 각 primitive 런타임. 본 SPEC은 저장만. 실제 enforcement 스펙은 v0.2 또는 별도 SAFETY SPEC.
4. **MCPB 저작 지원**: 본 SPEC은 reader만. writer(MCPB 생성 CLI)는 별도 도구.
5. **마켓플레이스 인증**: `LoadPluginFromMarketplace`는 Phase 5+ OAuth + 서명 검증 스텁. 현재는 `ErrNotImplemented`.
6. **plugins.yaml vs manifest 우선순위**: `enabled` 외의 필드(예: userConfigVariables)는 plugins.yaml 우선. manifest의 `userConfigVariables` 기본값은 fallback.
7. **병렬 로드**: 50+ plugin 시 ReloadAll 지연. `errgroup`으로 primitive 로드 병렬화 검토 (v0.2).

---

## 9. 구현 규모 예상

| 영역 | 파일 수 | 신규 LoC | 테스트 LoC |
|---|---|---|---|
| `manifest.go` | 1 | 200 | 250 |
| `validator.go` | 1 | 220 | 300 |
| `walker.go` | 1 | 150 | 200 |
| `loader.go` (orchestrator) | 1 | 280 | 400 |
| `mcpb.go` | 1 | 300 | 450 |
| `mcp_integration.go` | 1 | 120 | 180 |
| `registry.go` | 1 | 150 | 280 |
| `config.go` (plugins.yaml) | 1 | 120 | 200 |
| **합계** | **8** | **~1,540** | **~2,260** |

테스트 비율: 59%.

---

## 10. 결론

- **상속 자산**: TypeScript source map 설계 참조만. DXT 공개 스펙 기반 MCPB 구현.
- **핵심 결정**:
  - 3-phase(Stage/Commit/Rollback) 트랜잭션 패턴 for primitive loading.
  - `atomic.Pointer[map]` 기반 per-registry atomic swap; cross-registry 일관성은 약속 안 함.
  - MCPB는 zip slip / symlink / 파일 크기 3중 방어.
  - `${VAR}` 치환은 manifest + `.md`/`.yaml`/`.yml`/`.json` 텍스트 파일로 한정.
  - Reserved plugin name strict enforcement.
  - Marketplace는 v0.1 stub.
- **Go 버전**: 1.22+ (CORE-001 정합).
- **다음 단계 선행 요건**:
  - 4 primitive SPEC이 각각 `Stage/Commit/Rollback` 인터페이스 합의 필요 (v0.2 업데이트).
  - HOOK-001의 `HookEventNames()` 공개 (REQ-HK-001 기반 보장됨).
  - CONFIG-001이 `plugins.yaml` schema를 primary config에서 분리된 section으로 인지.
