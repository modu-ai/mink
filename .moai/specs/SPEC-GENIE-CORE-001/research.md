# SPEC-GENIE-CORE-001 — Research & Inheritance Analysis

> **목적**: `genied` 데몬 부트스트랩 구현에 재활용 가능한 자산과 재작성이 필요한 영역을 식별한다. Explore 에이전트를 사용하지 않고 본 작성자가 직접 grep/glob/ls로 레포를 스캔한 결과이다.
> **작성일**: 2026-04-21
> **범위**: darwin/linux Go 1.22+ 가정. Windows는 OUT OF SCOPE.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/
.agency  .claude  .gitignore  .mcp.json  .moai  .moai-backups
AGENT_HARNESSES_RESEARCH.md  CLAUDE.md  codemaps.md  codewiki.md
claude-code-source-map/   # TypeScript 참조 (Claude Code)
hermes-agent-main/        # Python 참조 (Hermes Agent)
HERMES_AGENT_COMPREHENSIVE_INVENTORY.md
LLM_ROUTING_RESEARCH.md   README.md
docs-site/
```

- `cmd/`, `internal/`, `pkg/`, `go.mod`, `go.sum` → **전부 부재**. Go 소스는 현재 0 LoC.
- `.moai/specs/` → `ROADMAP.md`와 본 SPEC 디렉토리만 존재 (지금 신규 생성됨).
- MoAI-ADK-Go (`structure.md` §9가 계승 대상으로 지목) → **본 레포에는 미러되지 않았음**. 외부 레포(`moai-adk-go`)가 존재한다고 문서는 가정하나, 물리적으로 확인 불가.

**결론**: Phase 0 모든 Go 코드는 **신규 작성**이며, "직접 포트" 시나리오는 없다. 설계 패턴만 참고한다.

---

## 2. 참조 가능한 자산별 분석

### 2.1 MoAI-ADK-Go (외부, 미러 없음)

- `structure.md` §9.1 Table: "`internal/evolution/observe/`", "`internal/research/patterns/`", "`internal/telemetry/`", "`internal/lsp/`" 등이 "MoAI 계승"으로 명시됨.
- 본 SPEC은 부트스트랩/shutdown 범위이므로 `internal/core/` 디렉토리 구조만 참고하면 충분하다 — 이는 일반적 Go daemon 패턴이므로 외부 레포 없이도 독립 구현 가능.
- **행동**: 본 SPEC에서는 MoAI-ADK-Go 소스를 복사하지 않고, `go.uber.org/zap` + `context` + `os/signal` 표준 조합으로 독립 작성. 이후 CONFIG/TRANSPORT SPEC에서 MoAI-ADK-Go 미러링 여부를 재평가한다.

### 2.2 Claude Code TypeScript (`./claude-code-source-map/`)

파일 스캔 결과:

```
claude-code-source-map/
├── bootstrap/state.ts           # 56KB — 세션 부트스트랩 상태 관리
├── entrypoints/
│   ├── cli.tsx                  # 39KB — CLI 진입점 (Ink React)
│   ├── init.ts                  # 13KB — 초기화 시퀀스
│   └── sdk/                     # SDK 엔트리
└── bridge/ (33 files)           # Remote bridge (Phase 5+용)
```

`entrypoints/init.ts`에서 확인된 graceful shutdown 시그니처:

```
9:  import { shutdownLspServerManager } from '../services/lsp/manager.js'
32: gracefulShutdownSync,
34: } from '../utils/gracefulShutdown.js'
189: registerCleanup(shutdownLspServerManager)
224: gracefulShutdownSync(1)
```

**교훈**:
- `registerCleanup(fn)` 패턴 — 본 SPEC에서 `CleanupHook` 구조체로 Go화.
- `gracefulShutdownSync(exitCode)` — exit code 명시 전달. 본 SPEC의 REQ-CORE-009(panic 후 exit 1)와 일치.

**직접 포트 대상**: 없음 (언어 상이). 패턴 계승만.

### 2.3 Hermes Agent Python (`./hermes-agent-main/`)

디렉토리 스캔:

```
hermes-agent-main/
├── agent/                       # 29 files
│   ├── memory_manager.py       # SQLite FTS 기반 메모리 (→ MEMORY-001 참고)
│   ├── memory_provider.py      # Provider 인터페이스 (→ MEMORY-001 참고)
│   ├── trajectory.py           # 2KB — trajectory collection (→ TELEM-001)
│   └── ... (context_compressor, insights, skill_utils 등)
├── tools/                       # 58 sub-dirs — tool 정의
├── cli.py                       # 409KB 단일 파일 (!!)
└── toolsets.py, trajectory_compressor.py
```

`cli.py` L8872에서 `signal.SIGTERM` 핸들러 확인:

```python
_signal.signal(_signal.SIGTERM, _signal_handler)
```

- **부트스트랩 관점에서 참조할 가치는 제한적**. `cli.py`가 409KB 단일 파일이며, Python asyncio + signal 패턴은 Go의 `context.Context` + `signal.NotifyContext` 관용구로 재작성 필요.
- `agent/memory_provider.py`, `agent/trajectory.py`는 후속 SPEC(MEMORY-001, TELEM-001)에서 분석 가치 있음. 본 SPEC에서는 미사용.

**직접 포트 대상**: 없음. Python → Go 재작성 필요.

---

## 3. Go 이디엄 선택 (독립 작성 기준)

### 3.1 시그널 처리 — `signal.NotifyContext` (Go 1.16+)

표준 라이브러리만으로 graceful shutdown 구현:

```
rootCtx, cancel := signal.NotifyContext(
    context.Background(), syscall.SIGINT, syscall.SIGTERM,
)
defer cancel()
<-rootCtx.Done()  // blocks until signal
```

- `os/signal.Notify` + 채널 loop 패턴보다 간결.
- AC-CORE-002(SIGTERM → exit 0)의 직접 매핑.

### 3.2 Cleanup hook 등록 — 역순 실행 슬라이스

`defer` 스택 대신 명시적 `[]CleanupHook`를 사용하는 이유:
- 등록 순서와 독립적으로 **이름으로 식별 가능한 hook**이 필요 (로그 추적, AC-CORE-005 판단).
- Timeout을 hook별로 관리 가능 (향후 LoRA 훈련 hook 등은 긴 timeout 필요).

### 3.3 State machine — `atomic.Value`

`sync.Mutex` + `state ProcessState` 변수보다 `atomic.Value`가:
- 헬스 endpoint는 **읽기 경로**가 빈번 (REQ-CORE-005: 50ms 이내). 락 획득 오버헤드 회피.
- 쓰기 경로(부트스트랩 중 1회, shutdown 중 1회)는 경합 없음.

### 3.4 로거 — `go.uber.org/zap`

`tech.md` §3.1이 zap을 지정. `slog` (Go 1.21+ stdlib) 대안도 있으나:
- zap은 JSON 포맷 성숙도·필드 재사용(`With()`)·caller 자동 주입 우수.
- MoAI-ADK-Go도 zap 사용(tech.md 명시).

---

## 4. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 후속 SPEC 확장 |
|------|-----|-----------|--------------|
| `go.uber.org/zap` | v1.27+ | ✅ 로거 초기화 | 모든 패키지에서 공유 |
| `gopkg.in/yaml.v3` | v3.x | ✅ config.yaml 파싱 | CONFIG-001이 viper로 흡수 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 assertion | 전 SPEC 공용 |
| 표준 `net/http`, `os/signal`, `context`, `sync/atomic` | stdlib | ✅ | — |

**의도적 미사용**(Phase 0):
- `gin` / `chi` / `echo` — 헬스 endpoint 하나에 HTTP 프레임워크 불필요.
- `viper` — 단일 파일 파싱에 overkill. CONFIG-001에서 도입.
- `cobra` — CLI subcommand 없음 (genied는 데몬 단일 진입점). CLI-001(genie CLI)에서 도입.
- `grpc-go` / `connect-go` — TRANSPORT-001의 범위.

---

## 5. 예상 테스트 전략 (TDD RED → GREEN)

### 5.1 Unit 테스트 (7~10개)

- `TestState_Transitions` — atomic state transitions (init→bootstrap→serving→draining→stopped).
- `TestExitCodes_Constants` — 상수 값 정합성 (0, 1, 78).
- `TestCleanupHook_RegisterAndRunInOrder` — 등록 순서와 역순 실행.
- `TestCleanupHook_PanicIsolation` — AC-CORE-005.
- `TestBootstrapConfig_MissingFile_ReturnsDefaults`.
- `TestBootstrapConfig_MalformedYaml_ReturnsError`.

### 5.2 Integration 테스트 (`_integration_test.go` + build tag `integration`)

- `TestGenied_BootstrapsAndServesHealthz` — AC-CORE-001.
- `TestGenied_SigtermTriggersGracefulExit` — AC-CORE-002.
- `TestGenied_MalformedConfigExits78` — AC-CORE-003.
- `TestGenied_PortInUseExits78` — AC-CORE-004 (pre-bound listener 주입).
- `TestGenied_DrainingState_Returns503` — AC-CORE-006.

### 5.3 Race detector

`go test -race ./...`로 state machine의 atomic 연산과 cleanup hook fan-out 경합 없음을 검증.

### 5.4 커버리지 목표

- `internal/core/`: 90%+
- `internal/health/`: 85%+
- 전체 가중 평균: 85%+ (quality.yaml TDD 요구 충족)

---

## 6. 오픈 이슈

1. **Go 버전 최종 결정**: `tech.md` §1.2가 "Go 1.26+"를 명시하나, 2026-04 기준 Go 1.26은 아직 미출시 가능성. 본 SPEC은 **1.22+ 최소**로 잠정 결정 (signal.NotifyContext, generic 지원). 1차 구현자가 `go.mod`에 실제 가용 최신 안정 버전으로 잠금.
2. **헬스 endpoint 포트 기본값**: 제안 `17890` (genie의 모바일 키패드 "GOOS" 매핑). 다른 GENIE 프로세스와 충돌 시 `GENIE_HEALTH_PORT` env로 override.
3. **build metadata 주입 방법**: `-ldflags "-X main.version=..."`이 Go 관용이나, `runtime/debug.ReadBuildInfo()` 대안 검토. 본 SPEC은 `ldflags` 전제.
4. **Observability 도구**: OpenTelemetry은 Phase 5+에서 `internal/telemetry/exporter.go`로 편입 예정. 본 SPEC의 헬스 endpoint는 OTel과 독립.

---

## 7. 결론

- **상속 자산**: MoAI-ADK-Go 코드 미러 없음 → 독립 작성.
- **참조 자산**: Claude Code `entrypoints/init.ts`의 `registerCleanup` 패턴 (패턴만).
- **기술 스택 결정**: `zap` + stdlib `context`/`os/signal`/`atomic.Value` + `yaml.v3` + `net/http`.
- **구현 규모 예상**: 600~1,000 LoC (테스트 포함 1,200~1,800 LoC).
- **리스크**: R1(Go 버전), R5(포트 충돌)는 구성으로 해결 가능. 나머지는 기술적 미해결 항목 없음.

본 SPEC의 GREEN 단계는 **파운데이션 위의 다음 5개 SPEC(CONFIG/TRANSPORT/LLM/AGENT/CLI)이 같은 프로세스에 hook을 등록할 수 있는 인터페이스**를 확정한다는 의미가 있다.

---

**End of research.md**
