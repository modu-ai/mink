# GENIE-AGENT 구현 순서 종합 보고서

> **작성일**: 2026-04-21
> **대상**: 30 SPEC (Phase 0~7 전체)
> **근거**: ROADMAP v2.0 + 6 Phase 작성 에이전트 종합 보고 + 의존성 그래프 분석
> **방법론**: TDD (RED→GREEN→REFACTOR)
> **목적**: 의존성 그래프 기반 최적 구현 순서 + 병렬화 지점 + Milestone + 리스크 매핑

---

## 0. 요약

- **전체**: 30 active SPEC, 총 **563 REQ / 328 AC**, 예상 **~30,000 Go LoC** (+ Rust LoRA crate 위임)
- **Critical Path**: CORE → QUERY → CONTEXT → CREDPOOL → ADAPTER → HOOK → SUBAGENT → TOOLS → CLI → TRAJECTORY → MEMORY → INSIGHTS → REFLECT → SAFETY → LORA
- **6 Milestone** 구분: M0 파운데이션 → M1 멀티LLM → M2 4 Primitive → M3 MVP 동작 → M4 자기진화 → M5 Safety → M6 개인화
- **최대 병렬도**: M1 3개 + M2 3개 + M4 2~3개 + M6 2개 = 평균 2~3 SPEC 동시 진행
- **총 기간 (팀 2명, TDD 엄격)**: **~5개월** (20~22주)

---

## 1. 30 SPEC 전체 목록 + REQ/AC

### Phase 0 — Agentic Core (5)
| # | SPEC-ID | 우선 | 범위 | REQ | AC | 상태 |
|---|---------|----|----|-----|----|----|
| 01 | SPEC-GENIE-CORE-001 | P0 | S | 12 | 6 | v1.0 유지 |
| 02 | SPEC-GENIE-CONFIG-001 | P0 | S | 14 | 8 | v1.0 유지 |
| 03 | SPEC-GENIE-TRANSPORT-001 | P0 | M | 14 | 8 | v1.0 유지 |
| 04 | **SPEC-GENIE-QUERY-001** ★ | P0 | L | 20 | 12 | v2.0 신규 |
| 05 | **SPEC-GENIE-CONTEXT-001** | P0 | M | 16 | 10 | v2.0 신규 |

### Phase 1 — Multi-LLM Infrastructure (5)
| 06 | **SPEC-GENIE-CREDPOOL-001** ★ | P0 | L | 18 | 10 |
| 07 | SPEC-GENIE-ROUTER-001 | P0 | M | 16 | 8 |
| 08 | SPEC-GENIE-RATELIMIT-001 | P0 | S | 13 | 7 |
| 09 | SPEC-GENIE-PROMPT-CACHE-001 | P1 | S | 13 | 8 |
| 10 | **SPEC-GENIE-ADAPTER-001** ★ | P0 | L | 20 | 12 |

### Phase 2 — 4 Primitives (5)
| 11 | SPEC-GENIE-SKILLS-001 | P0 | L | 18 | 10 |
| 12 | SPEC-GENIE-MCP-001 | P0 | L | 20 | 12 |
| 13 | **SPEC-GENIE-HOOK-001** ★ | P0 | M | 20 | 10 |
| 14 | SPEC-GENIE-SUBAGENT-001 | P0 | L | 20 | 12 |
| 15 | SPEC-GENIE-PLUGIN-001 | P1 | M | 18 | 12 |

### Phase 3 — Agentic Primitives (3)
| 16 | SPEC-GENIE-TOOLS-001 | P0 | M | 20 | 9 |
| 17 | SPEC-GENIE-COMMAND-001 | P1 | S | 19 | 13 |
| 18 | SPEC-GENIE-CLI-001 | P0 | M | 25 | 16 |

### Phase 4 — Self-Evolution (5)
| 19 | SPEC-GENIE-TRAJECTORY-001 | P0 | S | 18 | 12 |
| 20 | SPEC-GENIE-COMPRESSOR-001 | P0 | M | 18 | 13 |
| 21 | **SPEC-GENIE-ERROR-CLASS-001** ★ | P0 | S | 24 | 18 |
| 22 | **SPEC-GENIE-MEMORY-001** ★ | P0 | M | 20 | 16 |
| 23 | SPEC-GENIE-INSIGHTS-001 | P1 | M | 19 | 19 |

### Phase 5 — Promotion & Safety (3)
| 24 | **SPEC-GENIE-REFLECT-001** ★ | P1 | L | 20 | 12 |
| 25 | SPEC-GENIE-SAFETY-001 | P1 | M | 16 | 9 |
| 26 | SPEC-GENIE-ROLLBACK-001 | P1 | S | 12 | 7 |

### Phase 6 — Deep Personalization (3)
| 27 | SPEC-GENIE-IDENTITY-001 | P2 | L | 18 | 12 |
| 28 | SPEC-GENIE-VECTOR-001 | P2 | M | 14 | 8 |
| 29 | SPEC-GENIE-LORA-001 | P2 | L | 20 | 12 |

### Phase 7 — Ecosystem (1)
| 30 | SPEC-GENIE-A2A-001 | P2 | L | 20 | 12 |

**합계**: **563 REQ / 328 AC**. ★ = critical path.

---

## 2. 상세 의존성 그래프 (Cross-Phase)

```
                          ┌────────── CORE-001 ──────────┐
                          │                              │
                  CONFIG-001                      TRANSPORT-001
                          │                              │
                          │                              │
            ┌─── CREDPOOL-001 ────┐                      │
            │         │           │                      │
      ROUTER-001  RATELIMIT-001   │                      │
            │                     │                      │
      PROMPT-CACHE-001            │                      │
            └─────────┬───────────┘                      │
                      │                                  │
                 ADAPTER-001 ──────┬── ERROR-CLASS-001   │
                      │            │                      │
                      └──────→ QUERY-001 ←────────────────┤
                                    │                     │
                           ┌────────┼────────┬────────┐   │
                           │        │        │        │   │
                       CONTEXT-001  │        │        │   │
                           │        │        │        │   │
                        SKILLS-001  │        │        │   │
                           │        │        │        │   │
                        HOOK-001 ───┤        │        │   │
                           │        │        │        │   │
                      SUBAGENT-001  │        │        │   │
                           │        │        │        │   │
                           └──── MCP-001 ←───┴────────┴───┘
                                    │
                              PLUGIN-001
                                    │
                                    ├──→ TOOLS-001 ──→ COMMAND-001 ──→ CLI-001
                                    │
              TRAJECTORY-001 ←──────┤
                    │                │
             COMPRESSOR-001          │
                    │                │
                    ├── MEMORY-001 ──┤
                    │                │
             INSIGHTS-001            │
                    │                │
                REFLECT-001          │
                    │                │
             SAFETY-001              │
                    │                │
             ROLLBACK-001            │
                    │                │
                    ├── IDENTITY-001 │
                    ├── VECTOR-001   │
                    └── LORA-001     │
                                     │
                                  A2A-001
```

---

## 3. 최적 구현 순서 (Milestone별)

### Milestone 0: Agentic Foundation (M0) — 2주
**목표**: 데몬 뜨고 단일 턴 LLM 호출 성공

**순차 구현**:
```
01. CORE-001      (기존 v1.0, 최소 수정) ────┐
02. CONFIG-001    (기존 v1.0) ────────────────┤  병렬 가능
03. TRANSPORT-001 (기존 v1.0) ────────────────┘
04. QUERY-001     (신규 ★ — 가장 복잡)
05. CONTEXT-001   (QUERY의 Compactor 인터페이스 구현체)
```

**완료 기준**: QueryEngine이 mock LLM으로 `<-chan SDKMessage` streaming 성공. AC-QUERY-01~10, AC-CONTEXT-01~08 모두 GREEN.

**리스크**: QUERY-001의 state machine + continue sites 구현이 M0 전체 일정의 60% 차지.

---

### Milestone 1: Multi-LLM + Error Handling (M1) — 3~4주
**목표**: 실제 Anthropic/OpenAI/Ollama 호출 + 에러 분류/재시도

**구현 경로** (병렬 가능 표시):
```
06. CREDPOOL-001 (독립, 선행)
    │
    ├─→ 07. ROUTER-001         ─┐
    ├─→ 08. RATELIMIT-001      │  [A그룹: 병렬 3개]
    │                          │
    └─→ 21. ERROR-CLASS-001 ───┘  ← ADAPTER 이전에 독립 진행 가능
           │
    07, 08 완료 후:
           │
           09. PROMPT-CACHE-001 (Anthropic 특화)
                      │
           10. ADAPTER-001 ★   (06+07+08+09+21 모두 소비, QUERY-001의 LLMCall 구현)
```

**완료 기준**: `genie ask "hello"` → Anthropic 또는 OpenAI 응답. 429 rate limit 시 auto rotation. 컨텍스트 초과 감지.

**병렬화**: 팀 2명 → ROUTER/RATELIMIT/ERROR-CLASS 3개 동시 진행 가능. 단일 개발자는 순차.

**리스크**: ADAPTER-001이 Phase 1의 통합점. OAuth PKCE(Anthropic)와 tool conversion(OpenAI-format → Anthropic schema)이 까다로움.

---

### Milestone 2: 4 Primitives (M2) — 4주
**목표**: Skills/MCP/Agents/Hooks 4 primitive 작동

**구현 경로**:
```
      SKILLS-001    HOOK-001    MCP-001
      (QUERY 소비자) (QUERY 통합) (TRANSPORT만)
          │            │            │
          │            │            │  ← [B그룹: 3개 완전 병렬 가능]
          │            │            │
          └────────────┴────────────┘
                       │
                SUBAGENT-001 (SKILLS+HOOK 필요)
                       │
                 PLUGIN-001 (4 primitive 모두 필요, 마지막)
```

**완료 기준**: 
- Skill 로드 및 fork 실행
- MCP 외부 서버 연결 (Context7, Sequential-Thinking)
- 24 hook 이벤트 발화 + PreToolUse permission gate
- Sub-agent fork/worktree/background 3 isolation
- Plugin manifest.json 로드

**중요 주의**: **QUERY-001 v0.2.0 업데이트 필요**. SKILLS/HOOK이 QueryEngine의 `SubmitMessage`에 ProcessUserInput + HookDispatcher 훅 추가. Phase 2 완료 시점에 QUERY-001.md `HISTORY` 섹션에 `0.2.0` 추가 예정.

**병렬화**: SKILLS/HOOK/MCP 3개는 완전 독립. 팀 3명이면 주 단위 압축 가능.

**리스크**: `modelcontextprotocol/go-sdk`가 v0.x alpha — wrapper layer 필수. MoAI-ADK-Go 기존 26 agent 호환성 검증 필요.

---

### Milestone 3: MVP 동작 (M3) — 2주
**목표**: **사용자가 `genie` 실행 → TUI 대화 가능** (MVP Release 후보)

**구현 경로**:
```
16. TOOLS-001    (QUERY+MCP 소비, builtin 6개 + MCP 통합)
     │
17. COMMAND-001  (QUERY 소비, slash command)
     │
18. CLI-001      (v0.2.0 재작성: cobra + bubbletea TUI + Connect-gRPC)
```

**완료 기준**: 
```bash
$ genie                    # TUI 대화 시작
$ genie ask "fix bug in main.go"  # non-interactive
$ genie session list              # session 관리
$ genie tool list                 # tool 목록
```

**중요 주의**: QUERY-001 v0.2.0의 `Dispatcher.ProcessUserInput` 호출 확장 필요 (COMMAND-001 연계).

**리스크**: TRANSPORT-001의 proto 확장 3개 (`AgentService/ChatStream`, `ToolService/List`, `ConfigService/*`) — CLI-001 범위 내 proto 추가로 문서화.

---

### Milestone 4: Self-Evolution Core (M4) — 3주
**목표**: 자율 학습 파이프라인 작동 (trajectory 수집 → 압축 → 통찰 → 메모리)

**구현 경로**:
```
21. ERROR-CLASS-001 ✅ (M1에서 이미 완료)
   │
19. TRAJECTORY-001 (QUERY의 PostToolUse/SessionEnd hook 소비)    ─┐
   │                                                              │
22. MEMORY-001 (독립, Pluggable Provider)                          ├─ 병렬 가능
   │                                                              │
20. COMPRESSOR-001 (ROUTER의 Summarizer 필요)  ←──────────────────┘
   │
23. INSIGHTS-001 (TRAJECTORY + MEMORY 소비, 마지막)
```

**완료 기준**:
- 매 세션 JSONL trajectory 저장 (success/failed 분리)
- 15K+ token trajectory 자동 압축 (Gemini 3 Flash summarizer)
- Weekly insight report: "가장 바쁜 요일 화요일, 최근 error rate 8% 증가"
- Memory provider 인터페이스로 Honcho/Mem0 plug-in 가능

**병렬화**: TRAJECTORY ↔ MEMORY 병렬. COMPRESSOR는 둘 다 소비하므로 후속.

**리스크**: tokenizer 라이브러리(tiktoken-go vs 자체 구현) 결정. Pricing YAML 데이터 유지보수 주체 미정.

---

### Milestone 5: Promotion & Safety (M5) — 2주
**목표**: 학습 결과가 실제 사용자 상태를 변경하기 전 안전 게이트 통과

**구현 경로 (순차 필수)**:
```
24. REFLECT-001  (INSIGHTS+MEMORY 소비, 5-tier 상태 머신)
     │
25. SAFETY-001   (REFLECT의 MarkGraduated 게이트, 5-layer)
     │
26. ROLLBACK-001 (REFLECT의 MarkRolledBack 호출자, regression 감지)
```

**완료 기준**:
- Observation(1회) → Heuristic(3회) → Rule(5회+0.80) → HighConfidence(10회+0.95) → Graduated(사용자 승인 via Hook) state machine 작동
- 5-layer safety (FrozenGuard/Canary/Contradiction/RateLimiter/HumanOversight) 통과 검증
- 0.10 이상 score 하락 시 자동 rollback + 30일 쿨다운

**리스크**: AskUserQuestion은 HOOK-001의 `HookEventApprovalRequest` 이벤트로만 간접 호출 (직접 호출 금지). 현 세션 orchestrator가 이 이벤트를 AskUserQuestion으로 변환해야 함.

---

### Milestone 6: Deep Personalization + Ecosystem (M6) — 4~5주
**목표**: Identity Graph + Preference Vector + User LoRA (사용자의 디지털 쌍둥이)

**구현 경로**:
```
27. IDENTITY-001 (MEMORY+SAFETY 소비, POLE+O Kuzu) ─┐
                                                     │ 병렬
28. VECTOR-001   (MEMORY 소비, 768-dim Qdrant)     ─┘
    │
    └─→ 29. LORA-001 (VECTOR+SAFETY 소비, Go 인터페이스 + Rust genie-ml crate 위임)
                │
                └── A2A-001 병렬 가능 (M2 완료 시 언제든 착수)
                    (MCP+SUBAGENT 소비)
```

**완료 기준**:
- Identity Graph: 대화에서 엔티티 자동 추출 + 시간 추적
- User LoRA: 매주 auto 재훈련 + 이전 버전 롤백
- A2A: 외부 에이전트 Agent Card 디스커버리 + escrow 결제

**중요 주의**: **LORA-001은 Go 인터페이스만 정의. 실제 Tensor/Gradient/ONNX/QLoRA 4-bit는 Rust `crates/genie-ml/`** (별도 `ROADMAP-RUST.md`). Go ↔ Rust gRPC 기본 (`unix:///tmp/genie-ml.sock`), CGO는 선택.

**병렬화**: IDENTITY ↔ VECTOR 완전 병렬. A2A는 M2 이후 언제든 별도 트랙. LORA는 최후 (Rust 의존).

**리스크**:
- Kuzu Go 바인딩 1.26+ 호환성 미확인
- Rust genie-ml 별도 팀/일정 필요
- ONNX Runtime GenAI CGO 빌드 복잡도

---

## 4. Critical Path 분석

### 4.1 최단 경로 (순차 필수)
```
CORE → CONFIG → TRANSPORT → QUERY → CONTEXT
→ CREDPOOL → ROUTER → ADAPTER
→ HOOK → SUBAGENT
→ TOOLS → CLI                       [MVP Release 시점]
→ TRAJECTORY → MEMORY → INSIGHTS
→ REFLECT → SAFETY → ROLLBACK
→ VECTOR → LORA                     [v1.0 Release 시점]
```
총 **19 SPEC**. 나머지 11 SPEC(RATELIMIT, PROMPT-CACHE, ERROR-CLASS, SKILLS, MCP, PLUGIN, COMMAND, COMPRESSOR, IDENTITY, A2A, 3 DEPRECATED)은 critical path 외.

### 4.2 병렬화 가능 지점
| Milestone | 병렬 그룹 | 병렬 SPEC 수 | 절감 예상 |
|-----------|---------|-------|--------|
| M0 | CONFIG/TRANSPORT 병렬 | 2 | 30% |
| M1 | ROUTER/RATELIMIT/ERROR-CLASS 병렬 | 3 | 40% |
| M2 | SKILLS/HOOK/MCP 병렬 | 3 | 50% |
| M4 | TRAJECTORY/MEMORY 병렬 | 2 | 30% |
| M6 | IDENTITY/VECTOR 병렬 + A2A 별도 트랙 | 2+1 | 40% |

팀 2~3명 + 병렬화 최대 활용 시 **5개월 → 3.5개월** 단축 가능.

### 4.3 Blocker SPEC (후속 대거 차단)
1. **QUERY-001** — 19 후속 SPEC 직접 의존
2. **MEMORY-001** — 6 후속 SPEC 직접 의존 (INSIGHTS/IDENTITY/VECTOR/LORA/REFLECT)
3. **ADAPTER-001** — 4 후속 (QUERY 내부 + ERROR-CLASS + COMPRESSOR)
4. **HOOK-001** — 3 후속 (SUBAGENT + PLUGIN + SAFETY 승인 플로우)

→ 이 4개에 리소스 집중 권장.

---

## 5. TDD 엄격도 적용 전략

모든 SPEC은 quality.yaml `development_mode: tdd` 기준:

### 5.1 RED 단계 우선 작성
각 SPEC의 AC(Given-When-Then)를 **실패 테스트**로 먼저 작성. 예:
```go
// TestQueryEngine_SubmitMessage_StreamsImmediately(t *testing.T)
// TestCredPool_Select_RoundRobin(t *testing.T)  
// TestSkillLoader_AllowlistDefaultDeny(t *testing.T)
```

### 5.2 GREEN 최소 구현
테스트 통과 최소 코드만. 과도 추상화 금지.

### 5.3 REFACTOR 후 Skill("simplify") 강제
`run.md` Phase 2.10 — simplify skill 자동 실행 (MoAI 정책).

### 5.4 Coverage 게이트
Per-commit 85% minimum. Phase 4~6은 90%+ 권장 (복잡도 높음).

### 5.5 Characterization Test (brownfield)
MoAI-ADK-Go 상속 코드는 DDD 모드 추가 고려 (quality.yaml 임시 override). Phase 2 SUBAGENT-001이 해당 가능성 높음.

---

## 6. 인터페이스 계약 (Cross-Phase 공유 타입)

Phase 간 계약이 일관되어야 컴파일 + 통합 테스트 성공:

| 인터페이스 | 정의 SPEC | 구현 SPEC |
|----------|---------|---------|
| `LLMCall` | QUERY-001 | ADAPTER-001 |
| `Executor` (tool runner) | QUERY-001 | TOOLS-001 |
| `Compactor` | CONTEXT-001 | COMPRESSOR-001 (offline) + 자체 (in-session) |
| `Summarizer` | COMPRESSOR-001 | ADAPTER-001(어댑터 경유) |
| `MemoryProvider` | MEMORY-001 | BuiltinProvider + Plugin |
| `HookHandler` | HOOK-001 | QUERY-001 호출 + plugin 등록 |
| `PermissionMatcher` | HOOK-001 | TOOLS-001 + SKILLS-001 |
| `SafetyGate` | SAFETY-001 | REFLECT-001이 호출 |
| `Restorer` | ROLLBACK-001 | LORA-001 (LoRA hot-swap) |
| `SkillRegistry` | SKILLS-001 | COMMAND-001이 provider로 등록 |

**권장**: M1 초반에 `internal/contracts/` 패키지 생성하여 순수 interface만 먼저 정의 (SPEC 구현 전). 각 SPEC은 이 contract를 implement. 순환 의존 방지.

---

## 7. 예상 공수 및 Release Milestone

### 7.1 인력별 일정 (TDD + Skill("simplify") 포함)

| Milestone | 순차 기간 | 팀 2명 | 팀 3명 |
|-----------|--------|--------|--------|
| M0 Foundation | 3주 | 2주 | 1.5주 |
| M1 Multi-LLM | 4주 | 3주 | 2주 |
| M2 4 Primitive | 5주 | 4주 | 2.5주 |
| M3 MVP CLI | 2주 | 2주 | 1.5주 |
| M4 Self-Evolution | 4주 | 3주 | 2주 |
| M5 Safety | 2주 | 2주 | 1.5주 |
| M6 Personalization | 5주 | 4.5주 | 3주 |
| **합계** | **25주** | **20.5주** | **14주** |

Rust LoRA crate는 별도 트랙. Go 팀 1명 + Rust 팀 1명 필요.

### 7.2 Release Milestone
- **v0.1 Alpha** (M0+M1 완료, ~5주): genie ask 동작
- **v0.2 Beta** (M0~M3 완료, ~11주): MVP TUI + 4 primitive
- **v0.5 RC** (M0~M5 완료, ~17주): 자기진화 + Safety gate
- **v1.0 Release** (M0~M6 완료, ~22주): Personalization + A2A
- **v1.5** (+ Rust LoRA 안정화, ~28주): Full polyglot 성능

---

## 8. 즉시 실행 가능한 다음 액션

### 8.1 권장: Phase 0 CORE-001부터 TDD RED
```
/moai run SPEC-GENIE-CORE-001
```
- manager-tdd 서브에이전트가 AC-CORE-01~06 실패 테스트 작성
- go.mod 초기화 (Go 버전 확정 필요: tech.md 1.26+ 명시, 실제 최신 확인)
- cmd/genied + internal/core + internal/health 스켈레톤

### 8.2 병행 준비 작업
- **`internal/contracts/` 인터페이스 패키지** 선제 생성 (LLMCall, MemoryProvider, HookHandler, Executor, Compactor, Summarizer, PermissionMatcher, SafetyGate 순수 interface)
- `.moai/project/security.md` 작성 (redact 규칙 거버넌스)
- Rust genie-ml crate 별도 리포 초기화 (LORA-001 준비)
- `proto/` 디렉토리 초기 스키마 (TRANSPORT-001 + CLI-001 확장 3개)

### 8.3 의사결정 필요 항목 (구현 진입 전 확정)
| # | 결정 항목 | 영향 SPEC | 시점 |
|---|---------|---------|------|
| 1 | Go 버전 고정 | 전체 | CORE-001 RED 직전 |
| 2 | sqlite 드라이버 (modernc.org vs mattn) | MEMORY-001 | M4 진입 전 |
| 3 | Tokenizer 라이브러리 | COMPRESSOR, RATELIMIT, CONTEXT | M1 진입 전 |
| 4 | Graph DB (Kuzu 임베디드 vs Neo4j) | IDENTITY-001 | M6 진입 전 |
| 5 | LoRA Base Model (Qwen3-0.6B vs Gemma-1B) | LORA-001 | M6 진입 전 |
| 6 | LLM Stream 인터페이스 (`<-chan Chunk` 확정) | ADAPTER-001 | M1 진입 전 |
| 7 | proto 생성물 commit 정책 | TRANSPORT-001 | CORE-001 직후 |
| 8 | Rust genie-ml 배포 방식 (embedded vs 별도 바이너리) | LORA-001 | M6 진입 전 |

---

## 9. 주요 리스크 및 완화

| 리스크 | 영향 SPEC | 완화 방안 |
|------|---------|---------|
| `modelcontextprotocol/go-sdk` v0.x alpha 불안정 | MCP-001 | Wrapper layer 필수, SDK 버전 pinning |
| Anthropic OAuth PKCE 수동 구현 | ADAPTER-001, CREDPOOL-001 | anthropic-sdk-go 기능 확인 후 fallback HTTP client |
| 토큰 카운팅 정확도 | CONTEXT-001, COMPRESSOR-001, RATELIMIT-001 | MVP는 `/4 + overhead` 근사, v0.5+ tiktoken-go 정밀화 |
| QUERY-001 state machine 복잡도 | QUERY-001 | Claude Code `query.ts` 68KB 원문 라인-by-라인 포팅 |
| Rust genie-ml 팀 공수 | LORA-001 | Go 인터페이스 먼저 + mock gRPC로 end-to-end → Rust 후속 |
| AskUserQuestion 간접 호출 복잡도 | SAFETY-001, REFLECT-001 | HOOK-001의 Approval 이벤트 표준화 + 오케스트레이터 layer 문서화 |
| 기존 MoAI-ADK-Go 호환성 | SUBAGENT-001, PLUGIN-001 | 초기 로드 테스트 + legacy adapter 도입 판단 |

---

## 10. 최종 권장 순서 요약

```
[M0 Foundation ─ 2주]
    CORE ═══▶ CONFIG + TRANSPORT (병렬) ═══▶ QUERY ★ ═══▶ CONTEXT

[M1 Multi-LLM ─ 3주]
    CREDPOOL ═══▶ [ROUTER + RATELIMIT + ERROR-CLASS] (병렬 3개) ═══▶
    PROMPT-CACHE ═══▶ ADAPTER ★

[M2 4 Primitive ─ 4주]
    [SKILLS + HOOK + MCP] (병렬 3개) ═══▶ SUBAGENT ═══▶ PLUGIN

[M3 MVP CLI ─ 2주]  ← v0.2 Beta Release
    TOOLS ═══▶ COMMAND ═══▶ CLI

[M4 Self-Evolution ─ 3주]
    [TRAJECTORY + MEMORY] (병렬) ═══▶ COMPRESSOR ═══▶ INSIGHTS

[M5 Safety ─ 2주]  ← v0.5 RC Release
    REFLECT ═══▶ SAFETY ═══▶ ROLLBACK

[M6 Personalization ─ 4주]  ← v1.0 Release
    [IDENTITY + VECTOR] (병렬) ═══▶ LORA (+ Rust 위임)
    A2A (별도 트랙, M2 완료 시 언제든)
```

**첫 실행 커맨드**:
```bash
# (Go 버전 확정 후)
cd /Users/goos/MoAI/AgentOS
/moai run SPEC-GENIE-CORE-001
```

---

**Version**: 1.0.0
**License**: MIT (본 문서 포함)
**다음 단계**: 사용자 최종 승인 → Go 버전 확정 → `/moai run SPEC-GENIE-CORE-001` TDD RED 진입
