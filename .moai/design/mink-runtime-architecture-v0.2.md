# MINK Runtime Architecture v0.2

- Status: Confirmed (2026-04-24)
- Supersedes: (implicit v0.1 consensus pre-redesign)
- Decision source: 5-round Socratic interview + 2 follow-up rounds (QMD + location) + security redesign round
- Scope: MINK end-user runtime (`./.mink/` + `~/.mink/`). NOT moai-adk build-time (`.moai/`, `.claude/`, `CLAUDE.md`는 불가침).

---

## 0. 설계 원칙

### 승계 원천

| 출처 | 계승 요소 |
|---|---|
| Hermes v0.10 (Nous Research) | 3-layer memory, messenger gateway, Markdown 투명성, SOUL 개념 |
| Claude Code | YAML frontmatter + Progressive Disclosure, Hooks, 선언형 permission, sub-agent + Skill 분리 |
| moai-adk | SPEC-EARS + TRUST 5 + DDD/TDD (빌드 타임), @MX tag, FROZEN constitution |
| PAI (Daniel Miessler) | 11 identity context files |
| Macaron | Deep Memory RL retention, Affective layer |
| Plan-Execute-Reflect (arXiv 2602.10479, 2510.25445) | 5 canonical agent pattern 기반 loop |
| QMD (Tobias Lütke) | Local hybrid search for markdown (BM25+vector+rerank) |
| 2026 sandboxing best practice (NVIDIA/Northflank/agentkernel) | Defense-in-depth, zero-knowledge credential proxy |

### 핵심 경계

- 본 저장소 `.moai/` `.claude/` `CLAUDE.md` = **MINK 개발 도구**, 불가침
- 최종 사용자 런타임 = `./.mink/` (workspace) + `~/.mink/` (secrets only)
- **Build-time SPEC ≠ Run-time Task**: SPEC은 개발자용, Task는 사용자용

---

## 1. 스토리지 2원화 + 디렉토리 구조

### 1.1 `~/.mink/` (user-global, credential/security 전용)

```
~/.mink/
├── secrets/
│   ├── providers.yaml            # {provider, keyring_id} 참조만 저장
│   ├── channels.yaml             # 채널별 keyring 참조
│   └── vault.key.ref             # master key reference
├── config/
│   └── global.yaml               # 전역 사용자 선호
├── state/
│   └── projects.registry         # mink init 된 프로젝트 경로 목록
└── logs/
    └── audit.log                 # append-only 보안 이벤트 추적
```

예상 크기 <100KB. 실제 secret 값은 OS keyring에만 존재, 파일은 참조만.

### 1.2 `./.mink/` (project-local, workspace)

```
<project-root>/.mink/
├── config/
│   ├── mink.yaml                # 프로젝트 설정
│   ├── security.yaml             # allowlist/denylist/sandbox 설정
│   ├── providers.yaml            # LLM 라우팅 규칙 (key는 ~/.mink/ 참조)
│   └── channels.yaml             # 활성 채널 목록
├── persona/
│   ├── soul.md                   # 이 프로젝트에서의 MINK 정체성
│   ├── voice.md                  # 성장 단계별 보이스 규칙
│   └── growth.yaml               # 5 성장 단계 정의
├── context/                      # PAI 11 identity files
│   ├── mission.md  goals.md  projects.md  beliefs.md  models.md
│   ├── strategies.md  narratives.md  learned.md  challenges.md  ideas.md
│   └── growth.md
├── memory/
│   ├── MEMORY.md                 # agent-curated facts
│   ├── USER.md                   # 사용자 프로필 요약
│   └── summaries/                # LLM 요약 캐시 (per session)
├── skills/
│   ├── _builtin/                 # 번들된 기본 skills
│   ├── _auto/                    # 자동 생성 제안 skills (approval 대기)
│   └── */                        # 사용자·import skills
├── agents/                       # (Phase 2+) sub-agent 정의
├── commands/                     # 슬래시 커맨드
├── hooks/                        # event hook 스크립트
├── channels/
│   ├── telegram/config.yaml      # endpoint, bot username
│   ├── web/config.yaml           # localhost port
│   └── cli/                      # (설정 없음)
├── cron/
│   └── rituals.yaml              # 예약 ritual 정의
├── rituals/                      # ritual 실행 이력
├── tasks/                        # task 실행 이력 (plan/trace/reflection/result)
├── specs/                        # (optional) 사용자 custom workflow
├── cache/                        # LLM/tool 응답 캐시
├── logs/
│   ├── minkd.log                # 구조화 JSON 로그
│   └── audit.local.log           # 프로젝트 단위 감사 로그
└── data/
    ├── mink.db                  # 단일 SQLite (WAL)
    ├── mink.db-wal / .shm
    ├── qmd-index/                # QMD 하이브리드 검색 인덱스
    ├── models/                   # GGUF (embedder, reranker)
    ├── kuzu/                     # (Phase 8+) graph DB
    └── trajectory/               # opt-in 자기진화 로그
```

### 1.3 CLI 동작

- `mink init` — 현재 디렉토리에 `./.mink/` 생성 + `~/.mink/state/projects.registry`에 등록
- `mink` — upward traversal로 가장 가까운 `./.mink/` 발견까지 상위 디렉토리 탐색 (git 방식)
- 발견 실패 시 → `mink init` 안내

---

## 2. 4-Layer Primitive Model

```
┌── Layer 1: USER INTERFACE (사용자 가시) ──────────────────┐
│  Conversation  — 단발 turn, 즉시 응답 (Plan bypass)        │
│  Task          — 목표지향 (Plan→Run→Reflect→Sync 전체)     │
│  Ritual        — adaptive scheduled (날씨/기분/요일 반영)  │
│  Reflection    — 모든 Task 후 self-critique + daily       │
└────────────────────────────────────────────────────────────┘
┌── Layer 2: AGENT CORE (내부 엔진) ────────────────────────┐
│  Intent → Plan → Run (ReAct) → Reflect → Sync             │
│  + Re-plan: Reflect 실패 시 Plan으로 loop (max 2)          │
│  + Checkpoint: 각 단계 중간 산출물 검증                    │
└────────────────────────────────────────────────────────────┘
┌── Layer 3: IDENTITY (11 context files + persona) ─────────┐
│  soul / voice / growth                                     │
│  + mission / goals / projects / beliefs / models           │
│  + strategies / narratives / learned / challenges / ideas  │
│  + growth.md (history)                                     │
└────────────────────────────────────────────────────────────┘
┌── Layer 4: MEMORY (Macaron Deep Memory 철학) ─────────────┐
│  Working     — in-memory turn context                      │
│  Episodic    — SQLite FTS5 + QMD 하이브리드 검색          │
│  Semantic    — Kuzu graph (Phase 8+)                       │
│  Procedural  — ./.mink/skills/*.md + QMD 인덱스           │
│  Affective   — mood_log 테이블                             │
│  Identity    — context/*.md + QMD 인덱스                   │
│                                                            │
│  RL-based retention: score = f(recency, emotional weight,  │
│                                recurring goal, feedback)   │
└────────────────────────────────────────────────────────────┘
```

---

## 3. Plan → Run → Sync 상세 (Task 단위)

```
PLAN Phase
├─ Intent classification (conversation/task/ritual/ambiguous)
├─ Context retrieval (QMD hybrid search + Episodic + Identity)
├─ Sub-task DAG generation (Plan-and-Execute pattern)
├─ Tool/skill assignment per node
├─ Risk assessment: irreversible action?
│   → YES: AskUserQuestion 승인 요청
│   → NO: 자율 진행
└─ Artifact: plan.md + DB rows (tasks + plan_nodes)

RUN Phase (ReAct loop per node)
├─ Permission check (declared allowlist)
├─ FS access matrix check (Tier 2 + Tier 3 sandbox)
├─ Tool call / skill invocation (credential = proxy injection)
├─ Observe result
├─ Append to trace.md + trajectories (if opt-in)
├─ Checkpoint validation (schema/bounds)
└─ On failure → retry (max 3) or escalate to reflect

REFLECT Phase (self-critique)
├─ LLM critique: gap / inconsistency / unsupported claim
├─ Score (0.0 ~ 1.0)
├─ If score < 0.7 → re-plan (max 2 cycles)
├─ If score ≥ 0.7 → proceed to Sync
└─ Artifact: reflection.md

SYNC Phase (영속화)
├─ Write result.md
├─ Update episodic memory (messages, summaries)
├─ Update learned.md (new facts about user)
├─ Append affective log if mood detected
├─ Award growth XP (complexity × success × novelty)
├─ Evaluate auto-skill generation: 5+ tool calls?
│   → propose skill via AskUserQuestion
└─ Deliver to channel (Telegram/CLI/Web)
```

---

## 4. 데이터베이스 스키마 (`mink.db`, SQLite WAL)

### 4.1 Core tables

```sql
sessions (id PK, channel, started_at, ended_at, summary_md)
messages (id PK, session_id FK, role, content, created_at, tokens_in, tokens_out, metadata)
messages_fts USING fts5(content, content_rowid=id)
tasks (id PK, session_id, intent, status, plan_path, result_path, started_at, finished_at, score)
plan_nodes (id PK, task_id FK, seq, description, tool_call, depends_on, status, result_summary)
trajectories (id PK, task_id, step_seq, observation, reasoning, action, created_at)
rituals (id PK, name, schedule, adaptive_rules_yaml, channel, template_md_path, enabled, last_run_at)
ritual_runs (id PK, ritual_id FK, task_id FK, ran_at, adaptations JSONB)
growth_state (id CHECK(id=1), stage, stage_entered_at, xp, mood, mood_updated_at, streak_days)
growth_history (id PK AUTO, event_type, delta_json, occurred_at)
mood_log (id PK AUTO, session_id, inferred_mood, confidence, evidence_text, detected_at)
skills_meta (id PK, path, name, description, origin, created_at, last_used_at, usage_count, success_rate)
permissions (id PK AUTO, subject, capability, scope, granted_at, granted_by)
credentials_refs (id PK, provider, keyring_id, created_at)   -- 값 아닌 참조만
qmd_index_status (id CHECK(id=1), last_full_reindex_at, docs_indexed, index_size_bytes)
qmd_doc_tracking (path PK, content_hash, last_indexed_at, chunk_count)
fs_access_log (id PK AUTO, operation, path, allowed, reason, at)
```

### 4.2 Indexes
- `messages(session_id, created_at)`, `messages_fts(content)`
- `tasks(status)`, `plan_nodes(task_id, seq)`
- `ritual_runs(ritual_id, ran_at DESC)`
- `trajectories(task_id, step_seq)`
- `mood_log(session_id, detected_at DESC)`
- `fs_access_log(at DESC)`

### 4.3 Migration 전략
Embedded golang-migrate + `internal/db/migrations/*.up.sql + *.down.sql` (Go `embed` 번들). `mink migrate up/down/status` 서브커맨드.

---

## 5. 보안 아키텍처 (5-Tier Defense-in-Depth)

### Tier 1 — Storage Partition
- `~/.mink/secrets/*` = credential 참조만 (키링 ID)
- `./.mink/**` = workspace, 쓰기 자유
- 물리적 분리로 정책 단순화

### Tier 2 — Filesystem Access Matrix

| 경로 범주 | Read | Write/Delete/Create |
|---|---|---|
| `./.mink/**` | ✅ | ✅ |
| 사용자 write_paths allowlist | ✅ | ✅ |
| `./` 프로젝트 루트 (기본) | ✅ | ❌ |
| `~/.mink/config/` | ✅ | ❌ |
| 사용자 read_paths allowlist | ✅ | ❌ |
| `/etc` `/var` `/usr` `/bin` `/sbin` | ❌ | ❌ |
| `~/.ssh` `~/.aws` `~/.gnupg` `~/.env*` `~/.netrc` | ❌ | ❌ |
| `/proc` `/sys` `/dev` | ❌ | ❌ |
| `~` (home, allowlist 미매칭) | ❌ | ❌ |

### Tier 3 — OS-Level Sandbox
- macOS: Seatbelt (`sandbox-exec` profile)
- Linux: Landlock LSM + Seccomp-BPF
- Windows: AppContainer (v1.1+)
- `fallback_behavior: refuse` — 샌드박스 활성화 불가 시 실행 거부

### Tier 4 — Zero-Knowledge Credential Proxy
- OS Keyring에만 secret 값 저장 (macOS Keychain / libsecret / Windows Credential Vault)
- `mink-proxy` 별도 프로세스 = 네트워크 경계에서 Authorization header 주입
- agent 프로세스 메모리에 secret value 절대 진입하지 않음
- 프롬프트 인젝션으로 "토큰 출력" 시도해도 agent가 값을 모름

### Tier 5 — Declared Permission + Confirm
- Skill/MCP frontmatter `requires: {net, fs_read, fs_write, exec}` 선언
- 첫 호출 시 AskUserQuestion (항상/한번만/거절)
- Grant → `permissions` 테이블 기록
- 모든 결정 → `audit.log`

### `./.mink/config/security.yaml` 예시

```yaml
workspace:
  write_paths:
    - "./.mink/**"
    # 사용자 추가 예시
    # - "./output/**"
    # - "./drafts/**.md"
  read_paths:
    - "./"
    - "~/.mink/config/global.yaml"
  blocked_always:
    - "/etc/**" "/var/**" "/usr/**" "/bin/**" "/sbin/**"
    - "~/.ssh/**" "~/.aws/**" "~/.gnupg/**" "~/.env*" "~/.netrc"
    - "/proc/**" "/sys/**" "/dev/**"

sandbox:
  enabled: true
  implementation: auto
  fallback_behavior: refuse

secrets:
  backend: os_keyring
  injection: transport_proxy
  proxy_port: 8788
```

---

## 6. 메신저 채널 (v0.1 Alpha)

| 채널 | 상태 | 구현 |
|---|---|---|
| CLI / TUI | ✅ | Bubble Tea REPL |
| Telegram | ✅ | Bot API long-poll → gateway |
| Web UI (localhost) | ✅ | localhost:8787 SSE 스트림, **비개발자 설치·관리 GUI** |
| Email (IMAP+SMTP) | ❌ **제거** | — |

Gateway 공통 규약: `IncomingMessage{channel, user_id, text, attachments}` → `OutgoingMessage{channel, text, attachments}`. Agent Core는 채널을 모른다.

---

## 7. LLM Provider 전략

```yaml
# ./.mink/config/providers.yaml
default: claude-sonnet-4.6
fallback_chain:
  - claude-sonnet-4.6
  - gpt-5
  - gemini-2.5-pro
  - ollama:qwen3-32b

routing:
  by_task_complexity:
    simple: ollama:qwen3-8b
    medium: claude-sonnet-4.6
    complex: claude-opus-4.7

auto_fallback_triggers:
  - 429 rate-limited
  - 503 unavailable
  - timeout > 30s
```

Secret은 OS keyring, 이름은 `mink-provider-{name}` 규약.

---

## 8. QMD Memory Search 통합

- **구현**: `qntx-labs/qmd` (Rust) → CGO staticlib, 단일 바이너리
- **인덱싱 대상**:
  - `./.mink/memory/MEMORY.md USER.md summaries/*.md`
  - `./.mink/context/*.md` (11개)
  - `./.mink/skills/**/*.md`
  - `./.mink/tasks/*/result.md reflection.md`
  - `./.mink/rituals/*/runs/*.md`
- **모델**: `bge-small-en-v1.5.gguf` (embedder, ~120MB) + `bge-reranker-base.gguf` (~280MB)
- **재인덱스**: `on_file_change` 자동, `mink qmd reindex` 수동
- **M1부터 편입** — Phase 8 Kuzu와 공존 (QMD=문서 검색, Kuzu=관계 추론)

---

## 9. Ritual Adaptive Engine

```yaml
# ./.mink/cron/rituals.yaml
rituals:
  - id: morning-briefing
    name: 아침 브리핑
    schedule: "0 7 * * *"
    adaptive:
      if_weather: rain
        add: 우산 리마인드
      if_day_of_week: monday
        add: 주간 목표 복기
      if_mood: sleepy
        tone: gentle
      if_growth_stage: sage
        depth: deeper_insight
    channel: telegram
    skill: morning-briefing
```

---

## 10. Milestone 정렬 (최종)

```
M0 Foundation (✅ CORE-001 완료)
M1 Multi-LLM + QMD         CREDPOOL · ROUTER · ADAPTER · QMD-001 · PROVIDER-FALLBACK
M2 4 Primitives            SKILLS · MCP · HOOK · SUBAGENT · PERMISSION-001
M3 Core Workflow           COMMAND · CLI · TUI · Plan-Run-Sync · REFLECT-001
M4 Self-Evolution          TRAJECTORY · COMPRESSOR · INSIGHTS · auto-skill
M5 Safety ★ 대폭 확장
  ├─ SAFETY-001
  ├─ ROLLBACK-001
  ├─ SECURITY-SANDBOX-001 (OS-level)
  ├─ CREDENTIAL-PROXY-001 (zero-knowledge)
  ├─ FS-ACCESS-001         (allowlist/denylist matrix)
  └─ AUDIT-001             (append-only audit.log)
M6 Channels                TELEGRAM-001 · WEBUI-001 (Email 제거)
M7 Daily Companion v1.0    RITUAL-001 · BRIEFING · JOURNAL · CONTEXT-001 · GROWTH
M8 Deep Personalization    IDENTITY · VECTOR · LORA · Kuzu · Affective
M9 Ecosystem v2.0          plugin marketplace · additional channels
```

SPEC 총량: 58 → 약 48 (삭제 5 / 재편 5 / 신규 9).

---

## 11. 파일 포맷

### 11.1 Skill (Claude Code 스타일 frontmatter)

```markdown
---
name: morning-briefing
description: 아침 브리핑 생성 (날씨/일정/뉴스)
trigger_keywords: ["아침 브리핑", "오늘 시작"]
requires:
  net: ["openweathermap.org/*", "googleapis.com/*"]
  fs_read: ["./.mink/context/goals.md"]
  fs_write: []
  exec: []
estimated_tokens: 800
origin: builtin
version: 1.0.0
trust_level: user-confirm-once
---

# 본문: step-by-step 지시
```

### 11.2 Ritual (YAML)
위 §9 참조.

### 11.3 Plan (Markdown + DB)
- 파일: `./.mink/tasks/{task-id}/plan.md` (사람 읽기용)
- DB: `tasks` + `plan_nodes` 테이블 (검색/집계용)
- 중요 task만 AskUserQuestion 승인 요청

---

## 12. 모듈 & 바이너리

- Module path: `github.com/modu-ai/mink`
- Binaries:
  - `minkd` — 데몬 (long-running, channel gateway)
  - `mink` — 사용자 CLI (init, status, migrate, provider, qmd, forget, …)
  - `mink-proxy` — zero-knowledge credential proxy (별도 프로세스)
- Go 1.26 + Rust crate (CGO staticlib, LoRA + QMD)

---

## 13. Appendix — 폐기된 설계 결정

| 항목 | 폐기 사유 |
|---|---|
| Mobile 네이티브 앱 (iOS/Android) | Hermes 패턴과 충돌, 과잉 |
| Apple Native (Siri/Share/Live Activity) | self-hosted agent 패턴과 충돌 |
| Cloud Free Tier 3-Tier 구조 | self-hosted only 철학에 집중 |
| Email 채널 (IMAP+SMTP) | v0.1 범위 축소 |
| `~/.mink/` 전체 워크스페이스 | project-local로 전환 (team 공유·격리) |
| 전체 DB 암호화 (SQLCipher) | CGO 부담, zero-knowledge proxy가 더 효과적 |
| path-string only denylist | bypass 가능 (Ona 사례), OS sandbox로 보강 |
| 환경변수 credential 저장 | 프롬프트 인젝션 취약, OS keyring으로 전환 |

---

## References

- arXiv 2602.10479 — Goal-Directed Agentic AI Architecture
- arXiv 2510.25445 — Agentic AI Comprehensive Survey
- NousResearch/hermes-agent v0.10
- tobi/qmd + qntx-labs/qmd (Rust)
- Claude Code Permission Docs
- Northflank — How to sandbox AI agents in 2026
- NVIDIA AI Red Team — Practical Security Guidance for Agentic Workflows
- agentkernel — Secrets & Zero-Knowledge Proxy
- Daniel Miessler — Personal AI Infrastructure (PAI)
