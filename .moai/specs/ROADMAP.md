# GENIE-AGENT SPEC 로드맵 v2.0

> **프로젝트**: GENIE-AGENT v4.0 GLOBAL EDITION
> **재작성일**: 2026-04-21
> **이전 버전**: v1.0 (22 SPEC, `/.moai/specs/ROADMAP.md` git history 참조, 폐기)
> **대화 언어**: 한국어 · **코드 식별자**: 영어
> **개발 방법론**: TDD (quality.yaml: `development_mode: tdd`)
> **라이선스**: MIT
> **상태**: 아키텍처 설계 완료, 구현 0%

---

## 0. v2.0 재설계 근거

사용자 지시(2026-04-21):

> "포지셔닝과 경쟁사는 OpenClaw, Hermes Agent와 동일하다. 우리는 사용자의 패턴과 사용자의 니즈를 학습하고 스스로 진화해서 사용자에게 최적화 진화를 스스로 할 수가 있어야 한다. 모든 llm을 api 또는 oauth로 연결이 가능하고, skills, mcp, agents, hooks 개념은 기존의 claude code map 내용과 전반적인 에이전틱 시스템과 프롬프트를 참고해서 재설계를 하자."

### v1.0 vs v2.0 핵심 변화

| 축 | v1.0 (폐기) | **v2.0 (채택)** |
|----|----|----|
| 포지셔닝 | "100% 개인화 평생 동반자" | **OpenClaw/Hermes 동급 agentic coding/task tool + 자율 자기진화** |
| 총 SPEC | 22 | **30** |
| Phase | 7 (파운데이션 우선) | 7 (agentic core 우선) |
| Phase 0 목표 | daemon + hello 응답 | **agentic loop 전체**(QueryEngine + Context + Transport) |
| 학습엔진 | Phase 2~6 deferred | **Phase 4 first-class**(TRAJECTORY/COMPRESSOR/INSIGHTS/ERROR-CLASS/MEMORY) |
| LLM | Ollama 1종 | **15+ provider + OAuth/API + Credential Pool** |
| Skills/MCP/Agents/Hooks | Phase 5+ 일부 | **Phase 2 first-class 4 primitive** |
| Memory | 단일 SQLite | **Pluggable Provider**(Builtin + 외부 1개) |
| Error handling | 미정 | **ErrorClassifier 14-type + retry 전략** |

### 참조 근거

- `.moai/project/research/claude-core.md` — Claude Code agentic loop (QueryEngine + 5 Task 타입 + State 머신)
- `.moai/project/research/claude-primitives.md` — 4 primitive (Skills/MCP/Agents/Hooks) + Plugin host
- `.moai/project/research/hermes-llm.md` — Multi-LLM credential pool + Smart routing + Rate limit + Prompt caching + Context compressor
- `.moai/project/research/hermes-learning.md` — Trajectory 수집→압축→Insights→Memory→Skill 자동생성 파이프라인

---

## 1. 네이밍 규약

- 형식: `SPEC-GENIE-{DOMAIN}-{NNN}`
- DOMAIN: `CORE CONFIG TRANSPORT QUERY CONTEXT CLI COMMAND TOOLS CREDPOOL ROUTER RATELIMIT PROMPT-CACHE COMPRESSOR ADAPTER SKILLS MCP SUBAGENT HOOK PLUGIN TRAJECTORY INSIGHTS MEMORY ERROR-CLASS REFLECT SAFETY ROLLBACK IDENTITY VECTOR LORA A2A BAZAAR GATEWAY PRIVACY`
- NNN: Phase 내 1부터 시작. 동일 DOMAIN 재사용 시 002, 003

---

## 2. 우선순위 정의

| 표기 | 의미 |
|-----|------|
| **P0** | 차단 경로(blocker) |
| **P1** | 제품 가치 핵심, v1.0 이전 필수 |
| **P2** | 차별화 강화, v1.0 이후 또는 병렬 |

## 3. 범위 정의

- **S (소)**: 단일 패키지, ~500~1500 LoC, 1주
- **M (중)**: 2~3 패키지, ~1500~4000 LoC, 2~3주
- **L (대)**: 3+ 패키지, ~4000~8000 LoC, 외부 의존성 통합

---

## 4. 전체 SPEC 목록 (30건)

### Phase 0 — Agentic Core (5 SPEC, P0)

> 목표: async generator streaming query loop + context window + transport. Claude Code QueryEngine 구조 직접 이식.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 01 | **SPEC-GENIE-CORE-001** | genied 데몬 부트스트랩 + graceful shutdown | P0 | S | — | claude-core §1 + 기존 SPEC 재활용 |
| 02 | **SPEC-GENIE-QUERY-001** | QueryEngine + queryLoop (async streaming, state machine) ★ | P0 | L | CORE-001 | claude-core §1-2 |
| 03 | **SPEC-GENIE-CONTEXT-001** | Context Window 관리 + compaction (autoCompact/reactive/snip) | P0 | M | QUERY-001 | claude-core §7 |
| 04 | **SPEC-GENIE-TRANSPORT-001** | gRPC 서버 + proto 스키마 | P0 | M | CORE-001 | 기존 SPEC 재활용 |
| 05 | **SPEC-GENIE-CONFIG-001** | 계층형 설정 로더 (project/user/runtime YAML) | P0 | S | CORE-001 | 기존 SPEC 재활용 |

### Phase 1 — Multi-LLM Infrastructure (5 SPEC, P0) ★

> 목표: 모든 LLM을 API/OAuth로 연결. 15+ provider credential pool + smart routing + rate limit + prompt caching. Hermes credential_pool.py 원형 이식.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 06 | **SPEC-GENIE-CREDPOOL-001** | Credential Pool (OAuth/API, 4 strategy, rotation) ★ | P0 | L | CONFIG-001 | hermes-llm §1-3 |
| 07 | **SPEC-GENIE-ROUTER-001** | Smart Model Routing + Provider Registry | P0 | M | CREDPOOL-001 | hermes-llm §4 |
| 08 | **SPEC-GENIE-RATELIMIT-001** | Rate Limit Tracker (RPM/TPM/RPH/TPH 4 bucket) | P0 | S | CREDPOOL-001 | hermes-llm §5 |
| 09 | **SPEC-GENIE-PROMPT-CACHE-001** | Prompt Caching (system_and_3, 4 breakpoint, TTL) | P1 | S | ROUTER-001 | hermes-llm §6 |
| 10 | **SPEC-GENIE-ADAPTER-001** | 6 Provider 어댑터 (Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama) | P0 | L | ROUTER-001 | hermes-llm §8 |

### Phase 2 — 4 Primitives (5 SPEC, P0) ★

> 목표: Skills/MCP/Agents/Hooks first-class 구현. Claude Code 4 primitive 직접 포팅.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 11 | **SPEC-GENIE-SKILLS-001** | Progressive Disclosure Skill System (L0-L3, YAML, 4 trigger) ★ | P0 | L | QUERY-001 | claude-primitives §2 |
| 12 | **SPEC-GENIE-MCP-001** | MCP Client/Server (stdio/WS/SSE, OAuth, deferred loading) ★ | P0 | L | TRANSPORT-001 | claude-primitives §3 |
| 13 | **SPEC-GENIE-SUBAGENT-001** | Sub-agent Runtime (fork/worktree/bg isolation, 3 memory scope) ★ | P0 | L | QUERY-001, SKILLS-001 | claude-primitives §4 |
| 14 | **SPEC-GENIE-HOOK-001** | Lifecycle Hook System (24 events + useCanUseTool 권한 플로우) ★ | P0 | M | QUERY-001 | claude-primitives §5 |
| 15 | **SPEC-GENIE-PLUGIN-001** | Plugin Host (manifest.json + MCPB + 4 primitive 패키징) | P1 | M | SKILLS-001, MCP-001, SUBAGENT-001, HOOK-001 | claude-primitives §6 |

### Phase 3 — Agentic Primitives (3 SPEC, P0)

> 목표: Tool Registry + Slash Command + CLI. QueryEngine 소비자 계층.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 16 | **SPEC-GENIE-TOOLS-001** | Tool Registry + ToolSearch (deferred loading, inventory) | P0 | M | QUERY-001, MCP-001 | claude-primitives + Hermes model_tools.py |
| 17 | **SPEC-GENIE-COMMAND-001** | Slash Command System (/moai, /agency 등 custom) | P1 | S | QUERY-001 | Claude Code commands/ |
| 18 | **SPEC-GENIE-CLI-001** | genie CLI (cobra + Connect-gRPC, TUI) | P0 | M | TRANSPORT-001, COMMAND-001 | 기존 CLI-001 재작성 |

### Phase 4 — Self-Evolution (5 SPEC, P0) ★

> 목표: 자율 학습 파이프라인. Trajectory→Compressor→Insights→ErrorClass→Memory. Hermes 학습 파이프라인 원형 이식.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 19 | **SPEC-GENIE-TRAJECTORY-001** | Trajectory 수집 + 익명화 (ShareGPT JSON-L) ★ | P0 | S | QUERY-001 | hermes-learning §2 |
| 20 | **SPEC-GENIE-COMPRESSOR-001** | Trajectory Compressor (protected head/tail + LLM summary) ★ | P0 | M | TRAJECTORY-001, ROUTER-001 | hermes-learning §3 |
| 21 | **SPEC-GENIE-INSIGHTS-001** | Insights 추출 (Pattern/Preference/Error/Opportunity) ★ | P1 | M | TRAJECTORY-001 | hermes-learning §4 |
| 22 | **SPEC-GENIE-ERROR-CLASS-001** | Error Classifier (14 FailoverReason, retry 전략) | P0 | S | ADAPTER-001 | hermes-learning §5 |
| 23 | **SPEC-GENIE-MEMORY-001** | Pluggable Memory Provider (Builtin + 외부 1개) ★ | P0 | M | CORE-001 | hermes-learning §6 |

### Phase 5 — Promotion & Safety (3 SPEC, P1) ★

> 목표: MoAI SPEC-REFLECT-001 5단계 승격 + Safety 5-layer. 실제 사용자 상태 변경은 이 게이트 통과 필수.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 24 | **SPEC-GENIE-REFLECT-001** | 5단계 승격 (Observation→Heuristic→Rule→HighConf→Graduated) ★ | P1 | L | INSIGHTS-001, MEMORY-001 | learning-engine §2 |
| 25 | **SPEC-GENIE-SAFETY-001** | FrozenGuard + RateLimiter + Approval + Canary + Contradiction | P1 | M | REFLECT-001 | learning-engine §2.3 |
| 26 | **SPEC-GENIE-ROLLBACK-001** | 성능 저하 자동 감지 + 롤백 (30일 쿨다운) | P1 | S | REFLECT-001, SAFETY-001 | constitution §15 |

### Phase 6 — Deep Personalization (3 SPEC, P2)

> 목표: Identity Graph + Preference Vector + User LoRA. 사용자의 디지털 쌍둥이.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 27 | **SPEC-GENIE-IDENTITY-001** | Identity Graph POLE+O 스키마 + Kuzu 임베디드 | P2 | L | MEMORY-001, SAFETY-001 | learning-engine §3 |
| 28 | **SPEC-GENIE-VECTOR-001** | Preference Vector Space (768-dim, cosine) | P2 | M | MEMORY-001 | learning-engine §4 |
| 29 | **SPEC-GENIE-LORA-001** | User-specific QLoRA Trainer (ONNX Runtime GenAI) | P2 | L | VECTOR-001, SAFETY-001 | learning-engine §5 |

### Phase 7 — Ecosystem (옵션, 4 SPEC, P2)

> 목표: A2A · Marketplace · Multi-platform Gateway · Privacy. 상용화 레이어.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 | 근거 |
|---|---------|-----|----|----|----|-----|
| 30 | **SPEC-GENIE-A2A-001** | Agent Communication Protocol (Hermes ACP + Google A2A v0.3) | P2 | L | MCP-001, SUBAGENT-001 | hermes-learning §9 + ecosystem §2.3 |

> Phase 7의 나머지 3 SPEC(BAZAAR-001, GATEWAY-001, PRIVACY-001)은 별도 `ROADMAP-ECOSYSTEM.md`에서 관리. 본 로드맵에서는 A2A-001 하나만 최소 남김.

---

## 5. Phase별 SPEC 개수 및 핵심 가치

| Phase | 이름 | SPEC 수 | 핵심 가치 | 주요 산출물 |
|-------|-----|--------|----------|-------------|
| 0 | Agentic Core | 5 | async streaming query loop | QueryEngine + Context + Transport |
| 1 | Multi-LLM Infrastructure | 5 | 모든 LLM 연결 (API/OAuth) | 15+ provider credential pool |
| 2 | 4 Primitives | 5 | Skills/MCP/Agents/Hooks | plugin-loadable 4 primitive |
| 3 | Agentic Primitives | 3 | Tool Registry + CLI | 사용자 인터페이스 |
| 4 | Self-Evolution | 5 | 자율 학습 파이프라인 | Trajectory→Insights→Memory |
| 5 | Promotion & Safety | 3 | 5-tier + safety gates | SPEC-REFLECT-001 계승 |
| 6 | Deep Personalization | 3 | Identity + LoRA | 디지털 쌍둥이 |
| 7 | Ecosystem (옵션) | 1 (+3 별도) | A2A | IDE 통합 |
| **합계** | — | **30** | — | — |

---

## 6. 의존성 그래프 (상위)

```
CORE-001 ─┬─ CONFIG-001 ─┬─ CREDPOOL-001 ─┬─ ROUTER-001 ─┬─ ADAPTER-001
          │              │                 │              ├─ RATELIMIT-001
          │              │                 │              └─ PROMPT-CACHE-001
          │              │                 │                        │
          ├─ TRANSPORT-001 ─ MCP-001 ─┬── TOOLS-001                  │
          │                           └── SUBAGENT-001 ← SKILLS-001  │
          │                                                          │
          ├─ QUERY-001 ─┬─ CONTEXT-001                               │
          │             ├─ SKILLS-001 ─── PLUGIN-001 ←──── HOOK-001  │
          │             ├─ HOOK-001                                   │
          │             └─ COMMAND-001 ─ CLI-001                      │
          │                                                           │
          └─ MEMORY-001 ─┬─ TRAJECTORY-001 ─ COMPRESSOR-001 ──────────┤
                         ├─ INSIGHTS-001 ─ REFLECT-001 ─ SAFETY-001   │
                         │                   │                        │
                         │                   └─ ROLLBACK-001          │
                         │                                             │
                         └─ ERROR-CLASS-001 ←───────────────────────── ┘
                                                      │
                           IDENTITY-001 ─── VECTOR-001 ─── LORA-001
                                                      │
                                              A2A-001 ─┘
```

---

## 7. 실행 순서 권장

### MVP Milestone 1 — 동작하는 에이전트 (Phase 0+1+3 일부)
- CORE-001 → CONFIG-001 → TRANSPORT-001 → QUERY-001 → CONTEXT-001
- CREDPOOL-001 → ROUTER-001 → ADAPTER-001 (Anthropic + OpenAI 먼저)
- TOOLS-001 → CLI-001
- **동작 확인**: `genie ask "hello"` → Claude/GPT 경유 응답

### MVP Milestone 2 — 4 Primitive 완성 (Phase 2)
- SKILLS-001 → HOOK-001 → SUBAGENT-001 → MCP-001 → PLUGIN-001
- **동작 확인**: 외부 MCP 서버 연결, 서브에이전트 fork, skill 로드

### MVP Milestone 3 — 자기진화 코어 (Phase 4)
- TRAJECTORY-001 → ERROR-CLASS-001 → MEMORY-001 → COMPRESSOR-001 → INSIGHTS-001
- **동작 확인**: 주간 활동 리포트 생성, 에러 자동 분류/재시도

### MVP Milestone 4 — Promotion (Phase 5)
- REFLECT-001 → SAFETY-001 → ROLLBACK-001
- **동작 확인**: 관찰→승격→승인→적용→(저하 시)롤백 cycle

### v1.0 Release Milestone (Phase 6+7 일부)
- RATELIMIT-001, PROMPT-CACHE-001, COMMAND-001 (미착수 P1)
- IDENTITY-001 → VECTOR-001 → LORA-001 (로컬 개인화)
- A2A-001 (IDE 통합)

---

## 8. 기존 v1.0 SPEC 처리

`.moai/specs/` 하위 기존 6 SPEC 디렉토리 처리:

| 기존 SPEC | 처리 방안 |
|----------|---------|
| `SPEC-GENIE-CORE-001/` | **유지** — v2.0에서도 동일 역할. 필요 시 minor 수정 |
| `SPEC-GENIE-CONFIG-001/` | **유지** — v2.0에서도 동일 |
| `SPEC-GENIE-TRANSPORT-001/` | **유지** — v2.0에서도 동일 |
| `SPEC-GENIE-LLM-001/` | **폐기** — v2.0에서 CREDPOOL-001 + ROUTER-001 + ADAPTER-001으로 분할. 디렉토리는 보존하되 `DEPRECATED.md` 추가 |
| `SPEC-GENIE-AGENT-001/` | **폐기** — v2.0에서 QUERY-001 + SUBAGENT-001로 대체. `DEPRECATED.md` 추가 |
| `SPEC-GENIE-CLI-001/` | **재작성** — Phase 3으로 이동. cobra + Connect-gRPC + TUI로 확장 |

폐기 SPEC에는 다음 노트를 추가:
```markdown
# DEPRECATED
> 본 SPEC은 ROADMAP v2.0 재설계로 폐기됨.
> 대체 SPEC: {SPEC-GENIE-XXX-001}, {SPEC-GENIE-YYY-001}
> 이전 내용은 git history 참조.
```

---

## 9. 예상 구현 규모

| Phase | 영역 | Go LoC | Python 원본 대비 |
|-------|-----|--------|---------------|
| 0 | Agentic Core | ~3,000 | Claude Code TS의 80% 재사용 |
| 1 | Multi-LLM Infra | ~6,900 | Hermes Python 16,000 → 43% |
| 2 | 4 Primitives | ~5,000~7,000 | Claude Code TS의 80% 재사용 |
| 3 | Agentic Primitives | ~2,000 | 신규 |
| 4 | Self-Evolution | ~4,000 | Hermes Python 3,500 → 114% |
| 5 | Promotion & Safety | ~2,500 | MoAI SPEC-REFLECT-001 계승 |
| 6 | Deep Personalization | ~5,000 | 신규 (Rust + Go) |
| 7 | Ecosystem | ~2,000 | 신규 |
| **합계** | — | **~30,000** | MoAI-ADK-Go 38,700 + Hermes 16,000 + Claude 수천 → 30K로 축약 |

---

## 10. OUT OF SCOPE (본 로드맵에서 제외)

본 로드맵이 **다루지 않는** 항목. 별도 로드맵 또는 후속 분기에서 관리:

- **Rust 크리티컬 레이어** (`genie-ml`, `genie-wasm`, `genie-crypto`, `genie-vector`): Go Phase 6 안정화 이후 `ROADMAP-RUST.md`
- **TypeScript 클라이언트 패키지** (desktop/Tauri, mobile/RN, web/Next.js): CLI(Phase 3)를 제외한 나머지는 `ROADMAP-CLIENTS.md`
- **생태계·결제·토큰 경제** (Bazaar, Stripe, x402, 구독 티어): `ROADMAP-ECOSYSTEM.md`
- **Multi-platform Gateway** (Telegram/Discord/Slack/KakaoTalk/WeChat): 별도 로드맵
- **Hypernetwork 즉시 개인화** (Sakana AI Doc-to-LoRA): LORA-001 안정화 이후 별도 SPEC
- **Federated Learning / Secure Aggregation**: Phase 6+ P3 이후
- **Agent Teams, 다언어 LSP 18개**: MoAI-ADK-Go 직접 link 가능 시점에 재평가
- **한국 전용 기능** (KT, KakaoPay 등): 완전 제거 또는 옵션 플러그인

---

## 11. 오픈 질문 / 다음 결정 포인트

1. **Go 버전 고정**: tech.md는 1.26+. 실제 최신 안정 버전 교차검증 후 go.mod 확정
2. **Kuzu vs Neo4j** (IDENTITY-001): Kuzu 임베디드 우선, 클라우드 협업 요구 시 Neo4j 추가
3. **LLM 기본값** (ROUTER-001): Phase 0에서 Ollama 기본. BYOK는 Anthropic/OpenAI
4. **LoRA 베이스 모델** (LORA-001): Qwen3-0.6B vs Gemma-1B 라이선스·메모리
5. **LLM Stream 인터페이스** (ADAPTER-001): `StreamReader` vs `<-chan Chunk`
6. **proto 생성물 commit 정책**: repo에 commit vs CI 생성
7. **TDD 엄격도**: quality.yaml TDD 설정됨. Phase 0부터 RED-first 강제, 85%+ 커버리지
8. **Tokenizer 선택** (COMPRESSOR-001): Python Hermes는 Kimi tokenizer. Go는 tiktoken-go 또는 직접 구현

---

## 12. 첫 번째 실행 SPEC

**Phase 0, 순번 01**: [`SPEC-GENIE-CORE-001`](./SPEC-GENIE-CORE-001/spec.md) — genied 데몬 부트스트랩 (v1.0에서 유지).

**권장 실행 순서**:
```
1. CORE-001 (유지) → RED→GREEN→REFACTOR
2. CONFIG-001 (유지) + TRANSPORT-001 (유지) 병렬
3. QUERY-001 (신규) ★ 핵심 agentic loop
4. CONTEXT-001 (신규) + CREDPOOL-001 (신규) 병렬
5. ROUTER-001 (신규) + ADAPTER-001 (신규, Anthropic부터)
6. TOOLS-001 + CLI-001 (MVP Milestone 1 완성)
7. Phase 2 4 primitive
8. Phase 4 자기진화
9. Phase 5 safety
```

---

## 13. 핵심 설계 원칙 (5가지)

1. **One QueryEngine per conversation** — 세션 생명주기 = engine 생명주기 (Claude Code)
2. **Streaming mandatory** — buffering 금지, async channel (Claude Code)
3. **Credential Pool first** — 모든 LLM은 pool 경유, OAuth 자동 갱신, exhausted rotation (Hermes)
4. **4 Primitive first-class** — Skills/MCP/Agents/Hooks는 Phase 2 필수 (Claude Code)
5. **Self-evolution with safety gates** — 모든 학습 결과는 5-tier 승격 + 5-layer safety 통과 필수 (MoAI SPEC-REFLECT-001)

---

**Version**: 2.0.0
**License**: MIT (본 문서 포함)
**Next action**: 본 ROADMAP 사용자 승인 → Phase 0 CORE-001 `DEPRECATED` 대상 업데이트 → 신규 SPEC(QUERY-001부터) 작성 착수
