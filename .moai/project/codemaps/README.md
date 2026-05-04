# AI.GOOSE 코드맵 (Codemaps) — v0.1.0

완벽한 코드 수준의 아키텍처 문서. 미래의 MoAI 에이전트와 개발자를 위한 파일 레벨 인식.

**스캔 날짜**: 2026-05-04  
**커버리지**: 681 Go 파일 + 5 cmd 파일, internal/ 32개 패키지  
**생성 도구**: manager-docs Phase 1-2 코드맵 워크플로우

---

## 읽는 순서

### 1단계: 전체 그림 (5분)
**→ `architecture.md`** 계층식 아키텍처 + mermaid 다이어그램
- 5계층: cmd → core → agent → learning → bridge
- 핵심 모듈 의존성 맵
- 진입점 (goose CLI, goosed gRPC) 확인

### 2단계: 의존성 분석 (10분)
**→ `dependency-graph.md`** 패키지 간 import 관계
- 28개 패키지 × 인/아웃 fan-in/fan-out
- 순환 참조 검사 결과
- 최상위 10개 고-팬인 함수 (@MX:ANCHOR 후보)

### 3단계: 진입점 추적 (5분)
**→ `entry-points.md`** 생명주기 추적
- cmd/goose CLI 부트스트랩
- cmd/goosed 데몬 13단계 시작
- WebSocket 핸드셰이크 흐름

### 4단계: 모듈 깊이 (모듈별)
**→ `modules/*`** 각 패키지 상세 문서
- bridge.md: BRIDGE-001 + AMEND-001 (최신)
- llm-credential.md: Zero-Knowledge 크레덴셜
- llm-provider.md: 6개 프로바이더 라우팅
- query.md: QueryEngine 상태 머신
- command.md: Dispatcher 슬래시 커맨드
- agent.md: AgentRunner Plan-Run-Reflect
- memory.md: SQLite FTS + Qdrant + Graphiti
- audit.md: AUDIT-001 감시
- context.md: ContextAdapter 토큰 최적화
- core.md: Runtime → Session → Drain
- router.md: 라우팅 로직
- permission-sandbox.md: 3-layer permission 시스템
- skills-plugins.md: Skill/Plugin/Subagent/Hook
- transport-config.md: gRPC/WS/SSE 통합
- mcp.md: MCP Server/Client

---

## 스캔 방법론 (Phase 1-2)

### Phase 1: 탐색 (Exploration)
1. **디렉토리 구조**: `ls -la internal/*/` 스냔
2. **파일 개수**: 각 패키지의 .go 파일 count
3. **고팬인 후보**: 3+ 호출자 함수 grep
4. **@MX 인벤토리**: 기존 @MX:ANCHOR/@MX:WARN 스캔
5. **SPEC 참조**: go 주석에서 SPEC-XXX 추출
6. **최근 변경**: PR #82~#98 BRIDGE-001 + AMEND-001 포함

### Phase 2: 분석 + 작성 (Analysis + Writing)
1. **공개 API**: 패키지당 exported type/func 목록
2. **내부 상태**: private field + sync primitive 분석
3. **의존성**: import 스캔 → fan-in/out 계산
4. **생명주기**: 인스턴스 생성/소멸/초기화 흐름
5. **테스트**: *_test.go 파일로 공개 메서드 cross-check
6. **@MX:ANCHOR 제안**: fan_in >= 3 함수 표시

---

## 파일 구조

```
.moai/project/codemaps/
├── README.md                    ← 이 파일
├── architecture.md              ← 5계층 다이어그램
├── dependency-graph.md          ← adjacency 리스트 + 순환 검사
├── entry-points.md              ← CLI/daemon 부트스트랩
└── modules/                     ← 패키지별 상세
    ├── bridge.md                (BRIDGE-001 + AMEND-001 — 최신)
    ├── llm-credential.md        (Zero-Knowledge credential pool)
    ├── llm-provider.md          (6개 provider 라우팅)
    ├── query.md                 (QueryEngine state machine)
    ├── command.md               (Dispatcher slash commands)
    ├── agent.md                 (AgentRunner lifecycle)
    ├── memory.md                (SQLite FTS + Qdrant + Graphiti)
    ├── audit.md                 (AUDIT-001 감시)
    ├── context.md               (ContextAdapter 토큰 최적화)
    ├── core.md                  (Runtime/Session/Drain)
    ├── router.md                (라우팅)
    ├── permission-sandbox.md    (3-layer permission)
    ├── skills-plugins.md        (Skill/Plugin/Subagent/Hook)
    ├── transport-config.md      (gRPC/WS/SSE)
    └── mcp.md                   (MCP Server/Client)
```

---

## 스코프 범위

### ✅ 포함 (In Scope)
- `internal/` 32개 패키지 (28개 매뉴얼 스캔, 4개 deferred)
- `cmd/goose/`, `cmd/goosed/` 진입점
- 공개 API surface 모든 exported type/func
- 대형 패키지 (bridge 43파일, agent 24파일, llm 16파일 등)
- @MX:ANCHOR 후보 (fan_in >= 3)
- SPEC 참조 (BRIDGE-001, BRIDGE-AMEND-001 등)
- 최근 변경 이력 (PR #82~#98 merge)

### ⏸️ Deferred (Out of Scope — Next Cycle)
- `internal/cli/` — CLI 클라이언트 상세 (covered via cmd/goose entry-point)
- `internal/observability/` — observability 통합
- `internal/evolve/` — learning evolution 세부
- `internal/fsaccess/` — filesystem 추상화
- `internal/credproxy/` — credential proxy
- `internal/health/` — health check
- `internal/learning/` — learning engine 초기 구현 (structure.md로 충분)
- `internal/qmd/` — QMD chunk 분석

---

## 발견된 주요 항목

### 고팬인 함수 (High Fan-in — @MX:ANCHOR 후보)

1. **QueryEngine.SubmitMessage** (fan_in ≥3)
   - CLI, Transport, Subagent에서 호출
   - 10ms deadline, streaming response
   - State 단독 소유 contract

2. **Dispatcher.ProcessUserInput** (fan_in ≥3)
   - QUERY-001, 테스트, 통합 테스트
   - 모든 슬래시 커맨드의 dispatch 진입점

3. **outboundBuffer.Replay** (fan_in ≥2+)
   - BRIDGE-001 reconnect replay
   - Sequence gap 불가능 invariant

4. **Agent.Execute** (fan_in ≥3)
   - Core, Subagent, Test
   - Tool 실행 + learning 피드백

5. **LLMProvider interface** (fan_in ≥2+ impls)
   - Provider router가 선택해서 호출
   - 6개 구현체 (Ollama, OpenAI, Claude, Google, etc)

### 아키텍처 위반 없음
- 순환 참조: **0개** 검출 (acyclic DAG)
- 계층 침해: 없음 (5계층 원칙 준수)
- goroutine 안전성: @MX:WARN 4개 (permInbox, pendingPerms, buffer, etc)

### 최근 변경 (BRIDGE-001, AMEND-001)
- **M1**: DeriveLogicalID + WebUISession.LogicalID (PR #94)
- **M2**: Registry.LogicalID lookup + ws/sse LogicalID fill (PR #95)
- **M3**: dispatcher buffer rekey (LogicalID) + logout eager-drop (PR #96)
- **M4**: resumer LogicalID lookup + multi-tab integration (PR #97)
- **Status**: completed + 5 PRs mapped (PR #98 종결)

---

## 토큰 비용

| 파일 | LOC | 추정 Token |
|-----|-----|-----------|
| architecture.md | 250 | 1,250 |
| dependency-graph.md | 150 | 750 |
| entry-points.md | 120 | 600 |
| bridge.md | 250 | 1,250 |
| llm-credential.md | 200 | 1,000 |
| llm-provider.md | 200 | 1,000 |
| query.md | 200 | 1,000 |
| command.md | 180 | 900 |
| agent.md | 180 | 900 |
| memory.md | 180 | 900 |
| (나머지 6개) | ~1,100 | ~5,500 |
| **합계** | ~3,900 | ~19,500 |

---

## 향후 갱신

- Deferred 패키지 4개 추가 스캔 (500+ LOC)
- 테스트 커버리지 분석 (integration tests)
- Performance profile 추가 (goroutine 개수, heap 할당)
- TypeScript 클라이언트 mapping (packages/)

---

**Version**: 0.1.0 codemap  
**Generated**: 2026-05-04 by manager-docs  
**Source**: SPEC-CODEMAPS-001 Phase 1-2  
**Quality**: TRUST 5 Readable + @MX:ANCHOR candidates identified
