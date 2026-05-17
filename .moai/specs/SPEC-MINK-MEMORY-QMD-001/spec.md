---
id: SPEC-MINK-MEMORY-QMD-001
version: 0.1.0
status: planned
created_at: 2026-05-16
updated_at: 2026-05-16
author: manager-spec
priority: high
issue_number: null
phase: 3
size: 대(L)
lifecycle: spec-first
labels: [memory, qmd, sqlite-vec, fts5, ollama, sprint-3]
---

# SPEC-MINK-MEMORY-QMD-001 — QMD 기반 메모리 인덱스 (Markdown source-of-truth + sqlite-vec hybrid retrieval)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | 초안 작성 (Sprint 3 진입, ADR-001 QLoRA 폐기 결정 후속). QMD 표준(OpenClaw/ClawMem) 흡수 + Markdown source-of-truth + SQLite (sqlite-vec + FTS5) hybrid retrieval 정의. AGPL-3.0 헌장 위에서 작성. | manager-spec |

---

## 1. 개요 (Overview)

본 SPEC 은 MINK 의 **lifelong memory layer** 를 정의한다. 사용자의 대화·journal·briefing·ritual·weather 등 일상 산출물을 **Markdown source-of-truth + SQLite (sqlite-vec + FTS5) hybrid index** 구조로 누적·검색하여, **모델 가중치 변경 없이** 응답 품질을 점진적으로 향상시킨다.

본 SPEC 의 코드는 **AGPL-3.0-only** (LICENSE-COPYLEFT-001) 라이선스 하에서 작성된다.

핵심 가치:

- **자기진화 ½축**: 외부 GOAT LLM 라우팅 (SPEC-MINK-LLM-ROUTING-V2-AMEND-001) 의 보완 축. 모델 학습 없이 컨텍스트 누적만으로 개인화 달성.
- **Local-first**: 외부 클라우드 임베딩/검색 API 의존 0. 모든 임베딩은 로컬 ollama sidecar.
- **Reindexable**: 인덱스 손상 시 markdown vault 로부터 완전 복원 가능 (`mink memory reindex`).
- **Compatible**: optional `--vault-format=clawmem` 으로 OpenClaw/ClawMem 호환 vault 출력 → 다른 AI agent 와 vault 공유.

### Surface Assumptions (검증 필요)

사용자가 정정할 수 있도록 다음 가정을 명시한다. 본 SPEC 본문은 아래 가정 위에서 작성되었다:

1. **A1**: 사용자는 macOS / Linux / Windows native 중 하나에서 단일 사용자로 MINK 를 운영한다 (multi-user 동시 접근 미지원).
2. **A2**: 사용자의 일일 누적 chunk 수는 100개 미만, 1년 누적 18K chunk 미만이라고 가정한다 (sqlite-vec 단일 파일 범위 내).
3. **A3**: 임베딩 모델은 default `nomic-embed-text` (768-dim) 1종으로 시작한다. 사용자가 default 를 유지하는 한 모델 마이그레이션 비용은 발생하지 않는다. 사용자가 다른 모델 (예: REQ-MEM-035 mxbai-embed-large) 로 명시적으로 변경할 경우 R6 (stale chunk 폭증) 시나리오가 적용되며 `mink memory reindex` 로 전체 재색인이 필요하다.
4. **A4**: ollama 가 설치되지 않은 환경에서도 P0 (memory add / search BM25) 는 정상 동작해야 하며, vsearch/query 는 BM25-only graceful degrade 한다.
5. **A5**: ClawMem 호환 vault 는 launch 직후 사용자가 적극 활용하지 않을 수도 있다 (P2). 다만 vault 마이그레이션 비용이 낮아야 하므로 schema 호환은 launch 시점에 보장한다.
6. **A6**: PII 마스킹 패턴은 한국·영어권 기준으로 충분하다 (다른 언어권 PII 패턴은 후속 SPEC).
7. **A7**: cross-device sync 는 사용자가 markdown vault 디렉토리를 직접 git/Syncthing 등으로 동기화한다는 가정 (별도 sync 서비스 도입 0).

---

## 2. 배경 (Background)

### 2.1 ADR-001 결정 (2026-05-16): 가중치 학습 노선 폐기

본 프로젝트는 한때 on-device QLoRA / LoRA adapter / RLHF (SFT/DPO/GRPO) 기반 자기진화를 검토했으나, 다음 사유로 **전면 폐기** 결정:

- **하드웨어 제약**: 일반 데스크톱/노트북에서 14B+ 모델의 LoRA fine-tune 은 비현실적
- **운영 복잡도**: 가중치 hot-swap, version registry, eval pipeline 모두 사용자가 감당 불가
- **법적 리스크**: 사용자 데이터로 학습된 가중치 배포는 AGPL 호환성 별도 검토 필요
- **품질 대안 존재**: 외부 GOAT LLM 5종 + lifelong memory context 만으로도 개인화 충분

→ 자기진화는 두 축으로 재정의:

- (a) **외부 GOAT LLM 라우팅** (Claude / DeepSeek / GPT / Codex / GLM-5-Turbo) — 별도 SPEC
- (b) **QMD 메모리 인덱스** — **본 SPEC**

### 2.2 OpenClaw / ClawMem 표준 수렴

2025-2026 시점에 다수 AI agent (Hermes, Claude Code, Goose) 가 **QMD 구조 + ClawMem MCP 인터페이스** 로 수렴 중이다 (research.md §1). MINK 는 자체 vault 를 우선 운영하되, `--vault-format=clawmem` 옵션으로 **표준 호환 vault** 를 부수적으로 출력하여 사용자가 다른 AI agent 와 메모리 공유를 선택할 수 있게 한다.

### 2.3 AGPL-3.0 헌장 위에서

본 SPEC 의 모든 코드는 LICENSE-COPYLEFT-001 의 AGPL-3.0-only 라이선스 위에서 작성된다. 따라서:

- 외부 의존성은 AGPL 양립 라이선스 (MIT/BSD/Apache-2.0) 만 채택. `sqlite-vec` (MIT) 와 `mattn/go-sqlite3` (MIT) 모두 양립.
- 사용자 markdown vault 자체는 사용자 데이터이며 AGPL 적용 대상 아님 — 본 SPEC 은 그 점을 명시한다.

---

## 3. 스코프 (Scope)

### 3.1 IN-SCOPE

- Markdown source-of-truth vault (`~/.mink/memory/markdown/{collection}/*.md`)
- SQLite hybrid index (`~/.mink/memory/{agent}.sqlite`) using sqlite-vec + FTS5
- 6 default collections: `sessions/`, `journal/`, `briefing/`, `ritual/`, `weather/`, `custom/`
- 3 검색 모드: `search` (BM25 only), `vsearch` (vector only), `query` (hybrid + MMR + temporal decay)
- Ollama embed sidecar 통합 (default `nomic-embed-text`, optional `mxbai-embed-large`)
- BM25-only graceful degrade (ollama 미설치 시)
- CLI subcommand 7종: `mink memory {add|search|reindex|export|import|stats|prune}`
- TUI `/memory` slash command (read-only peek)
- 산출물 자동 색인 hook (JOURNAL/BRIEFING/WEATHER/RITUAL publish hook)
- session transcript opt-in export (LLM-ROUTING-V2 응답 → sanitized markdown)
- PII 마스킹 파이프라인 (전화/이메일/카드/OAuth/한국 주민번호)
- `--vault-format=clawmem` ClawMem 호환 출력
- 보안: mode 0600 vault + sqlite, mode 0700 부모 디렉토리
- USERDATA-MIGRATE-001 export 항목 추가 (`memory/markdown/`, `memory/*.sqlite`)
- macOS / Linux / Windows native 지원

### 3.2 OUT-OF-SCOPE (본 SPEC 에서 다루지 않음)

- 모델 가중치 학습 (QLoRA / LoRA / RLHF) — **ADR-001 로 폐기**
- 외부 클라우드 임베딩 API (OpenAI / Cohere / Gemini)
- 외부 vector DB (LanceDB / Chroma / Qdrant / Pinecone)
- Web UI 일상 운용 (Web 은 설치 단계에서 ollama 안내만)
- multi-user 동시 접근
- LLM 기반 chunk summarization at ingest path (별도 SPEC)
- cross-device sync 서비스 (markdown 동기화는 사용자의 git/Syncthing 책임)

---

## 4. 목표 (Goals)

본 SPEC 의 완수 시점에 다음 5개가 측정 가능하게 성립해야 한다.

| ID | 목표 | 측정 방법 |
|---|---|---|
| G1 | `mink memory add` 와 BM25 `mink memory search` 가 ollama 없이도 동작 | M1 acceptance 통과 + `ollama` 미설치 환경 CI matrix |
| G2 | ollama embed 활성 시 `mink memory query "..."` 가 top-10 검색 결과를 평균 200ms 이하로 반환 (corpus ≤ 10K chunk) | M3 + M4 의 latency benchmark 통과 |
| G3 | 인덱스 파일 임의 삭제 후 `mink memory reindex` 로 1:1 복원 (chunk 수, 본문 hash 일치) | M5 reindex regression test |
| G4 | journal/briefing/weather/ritual 산출물이 publish 직후 ≤ 1 sec 내 자동 색인 | M5 hook integration test |
| G5 | `--vault-format=clawmem` 으로 출력한 vault 를 ClawMem MCP 서버가 read 모드로 정상 mount | M5 호환성 매뉴얼 검증 (ClawMem v1.x) |

---

## 5. 비목표 (Non-Goals)

- multi-tenant SaaS 운영
- web UI 기반 메모리 브라우징 (CLI/TUI 만)
- 모델 가중치 학습 또는 distillation
- 외부 API 호출 (ollama 외 모든 인터넷 의존성)
- markdown 외 format (PDF, docx, json) source-of-truth 지원 (사용자가 markdown 으로 변환 후 ingest)
- 24/7 서버 데몬 (memory 는 CLI invocation 단위로 sqlite open/close)

---

## 6. 요구사항 (EARS)

본 SPEC 은 **EARS (Easy Approach to Requirements Syntax)** 형식으로 35~50개 acceptance criteria 를 정의한다. 각 REQ 는 priority 라벨 (P0/P1/P2) 을 부여한다.

### 6.1 Ubiquitous (시스템 상시 불변, 13개)

- **REQ-MEM-001 [P0]**: The MINK memory subsystem **shall** treat markdown files under `~/.mink/memory/markdown/{collection}/*.md` as the single source of truth.
- **REQ-MEM-002 [P0]**: The MINK memory subsystem **shall** create the SQLite index file at `~/.mink/memory/{agent}.sqlite` with file mode 0600.
- **REQ-MEM-003 [P0]**: The MINK memory subsystem **shall** create the markdown vault parent directory `~/.mink/memory/` with file mode 0700.
- **REQ-MEM-004 [P0]**: The MINK memory subsystem **shall** create every markdown file under the vault with file mode 0600.
- **REQ-MEM-005 [P0]**: The MINK memory subsystem **shall** compute each chunk_id as `hash(source_path:start_line:end_line:content_hash:model_version)` so that model_version changes mark all derived chunks as stale.
- **REQ-MEM-006 [P0]**: The MINK memory subsystem **shall** support six default collections: `sessions/`, `journal/`, `briefing/`, `ritual/`, `weather/`, `custom/`.
- **REQ-MEM-007 [P0]**: The MINK memory subsystem **shall** expose three retrieval modes: `search` (BM25 only), `vsearch` (vector only), `query` (hybrid).
- **REQ-MEM-008 [P1]**: The MINK memory subsystem **shall** use `query` as the default retrieval mode when the user issues `mink memory search` without an explicit mode flag.
- **REQ-MEM-009 [P1]**: The MINK memory subsystem **shall** compute the hybrid score as `α * cosine(emb(q), emb(c)) + β * bm25_normalized(q, c) + γ * temporal_factor(c.created_at)` with defaults α=0.7, β=0.3, γ=exp(-Δt / 30days).
- **REQ-MEM-010 [P1]**: The MINK memory subsystem **shall** apply MMR (Maximal Marginal Relevance) re-ranking with λ=0.7 (relevance) and 1-λ=0.3 (diversity) before returning top-k results.
- **REQ-MEM-011 [P0]**: The MINK memory subsystem **shall** be implementable in Go-only modulo a single permitted cgo dependency on the SQLite + sqlite-vec stack.
- **REQ-MEM-012 [P0]**: The MINK memory subsystem **shall** chunk markdown content by heading boundaries first, paragraph boundaries second, and a 512-token hard cap last, preserving prev/next chunk neighbor pointers.
- **REQ-MEM-013 [P1]**: The MINK memory subsystem **shall** persist embedding vectors only inside the SQLite index, never inside the markdown vault.

### 6.2 Event-Driven (8개)

- **REQ-MEM-014 [P0]**: **When** the user invokes `mink memory add --collection {c} --source {path}`, the subsystem **shall** copy or hardlink the markdown file under `~/.mink/memory/markdown/{c}/`, chunk it, and index every chunk in the SQLite store.
- **REQ-MEM-015 [P0]**: **When** the user invokes `mink memory search "{query}"`, the subsystem **shall** execute the configured retrieval mode and return top-k matches with chunk_id, source_path, line range, score, and a 256-char snippet.
- **REQ-MEM-016 [P1]**: **When** the user invokes `mink memory reindex`, the subsystem **shall** drop all index rows whose model_version does not equal the current model, re-chunk affected source files, and re-embed when an embedding backend is available.
- **REQ-MEM-017 [P1]**: **When** the JOURNAL/BRIEFING/WEATHER/RITUAL subsystem publishes an artifact, the memory subsystem **shall** receive the publish hook and ingest the new markdown into the corresponding collection within 1 second of the publish event.
- **REQ-MEM-018 [P2]**: **When** the LLM-ROUTING-V2 router completes a session and `memory.session_autoexport.enabled = true`, the subsystem **shall** export the sanitized session transcript to `sessions/{YYYY-MM-DD}/{provider}-{session_id}.md` and index it.
- **REQ-MEM-019 [P1]**: **When** the embedding backend (ollama) returns an error or is unreachable, the subsystem **shall** emit a warning to stderr and fall back to BM25-only retrieval without aborting the command.
- **REQ-MEM-020 [P2]**: **When** the user invokes `mink memory export --format clawmem --dest {path}`, the subsystem **shall** produce a ClawMem-compatible vault at the destination path within the same agent process.
- **REQ-MEM-021 [P1]**: **When** the user invokes `mink memory prune --before {date}`, the subsystem **shall** remove markdown source files older than `{date}` from the vault and drop their corresponding chunks from the SQLite index in the same transaction.

### 6.3 State-Driven (6개)

- **REQ-MEM-022 [P1]**: **While** an `mink memory reindex` operation is in progress, the subsystem **shall** continue to serve read-only `search` and `query` requests against the previous index snapshot (no read lock held by reindex worker).
- **REQ-MEM-023 [P1]**: **While** the ollama embedding endpoint is unreachable, the subsystem **shall** keep accepting `mink memory add` invocations and mark new chunks as `embedding_pending = 1` for later backfill.
- **REQ-MEM-024 [P2]**: **While** `--vault-format=clawmem` mode is active, the subsystem **shall** mirror every markdown write to both `~/.mink/memory/markdown/` and `~/.clawmem/vault/` paths and keep their content hashes identical.
- **REQ-MEM-025 [P0]**: **While** a write transaction is open on the SQLite index, the subsystem **shall** hold a single-writer mutex so that concurrent CLI invocations serialize their writes safely.
- **REQ-MEM-026 [P1]**: **While** the subsystem detects a chunk count mismatch between markdown and index, it **shall** log a warning with the suggested remediation `mink memory reindex --collection {c}`.
- **REQ-MEM-027 [P1]**: **While** the markdown vault contains files outside the 6 default collections plus `custom/`, the subsystem **shall** treat them as ignored and not index them, emitting a one-line warning per scan.

### 6.4 Unwanted Behavior (7개)

- **REQ-MEM-028 [P0]**: The MINK memory subsystem **shall not** store raw chat transcripts without first running the PII redaction pipeline.
- **REQ-MEM-029 [P0]**: The MINK memory subsystem **shall not** create any vault file or sqlite file with permission broader than mode 0600.
- **REQ-MEM-030 [P0]**: The MINK memory subsystem **shall not** block the CLI return for more than 100ms on a `mink memory add` call when chunking can be deferred to a background reindex task.
- **REQ-MEM-031 [P0]**: The MINK memory subsystem **shall not** transmit any markdown content or embedding vector to any external endpoint other than `http://localhost:11434` (ollama).
- **REQ-MEM-032 [P0]**: The MINK memory subsystem **shall not** silently overwrite a markdown source file in the vault; conflicting `add` operations **shall** be rejected with an explicit error.
- **REQ-MEM-033 [P1]**: The MINK memory subsystem **shall not** include embedding vectors in the markdown vault or in any `export` output that is not explicitly the SQLite file.
- **REQ-MEM-034 [P1]**: The MINK memory subsystem **shall not** allow `mink memory prune` to delete a markdown source file without a corresponding index row deletion in the same transaction.

### 6.5 Optional Features (4개)

- **REQ-MEM-035 [P1]**: **Where** the user installs `mxbai-embed-large` in ollama, the subsystem **shall** allow `memory.embedding.model = mxbai-embed-large` and persist the new model_version in chunk_id derivations.
- **REQ-MEM-036 [P2]**: **Where** the user enables `memory.clawmem_compat.enabled = true`, the subsystem **shall** mirror writes to the ClawMem vault as defined in REQ-MEM-024.
- **REQ-MEM-037 [P2]**: **Where** the user invokes `mink memory stats`, the subsystem **shall** report per-collection chunk counts, total embedding vector size on disk, and the oldest/newest chunk timestamps.
- **REQ-MEM-038 [P2]**: **Where** the user invokes `mink memory import --source {path}`, the subsystem **shall** validate that the source vault has a compatible schema and merge it into the local vault, refusing on schema mismatch.

**REQ totals**: Ubiquitous 13 + Event-Driven 8 + State-Driven 6 + Unwanted 7 + Optional 4 = **38 REQs**.
**Priority distribution**: P0 17, P1 14, P2 7.

---

## 7. 아키텍처

### 7.1 Go 패키지 트리

```
internal/memory/
├── memory.go                  # public API: Indexer, Searcher 인터페이스
├── config.go                  # memory.yaml 로딩, defaults
├── qmd/
│   ├── chunk.go               # ChunkMarkdown(content, opts) []Chunk
│   ├── chunk_test.go
│   ├── id.go                  # chunk_id hash 함수
│   └── neighbor.go            # prev/next pointer 관리
├── sqlite/
│   ├── store.go               # SQLite open/close, schema migration
│   ├── store_test.go
│   ├── schema.sql             # CREATE TABLE / vec0 / fts5 definitions
│   ├── writer.go              # 단일 writer mutex + transaction
│   └── reader.go              # 검색 query 빌더
├── ollama/
│   ├── client.go              # POST /api/embeddings, healthcheck
│   ├── client_test.go
│   └── fallback.go            # BM25-only graceful degrade decision
├── retrieval/
│   ├── search.go              # BM25-only mode
│   ├── vsearch.go             # vector-only mode
│   ├── query.go               # hybrid mode (α·vector + β·BM25 + γ·decay)
│   ├── mmr.go                 # MMR re-ranking
│   └── decay.go               # temporal decay factor
├── redact/
│   ├── pii.go                 # 마스킹 패턴 + 적용
│   └── pii_test.go
├── clawmem/
│   ├── mirror.go              # --vault-format=clawmem mirror writer
│   └── schema.go              # ClawMem vault 호환 schema 변환
├── hook/
│   ├── publish.go             # JOURNAL/BRIEFING/WEATHER/RITUAL publish hook
│   └── publish_test.go
└── cli/
    ├── add.go
    ├── search.go
    ├── reindex.go
    ├── export.go
    ├── import.go
    ├── stats.go
    └── prune.go
```

### 7.2 데이터 흐름 (ASCII)

```
                ┌──────────────────────────────────────────────┐
                │           ~/.mink/memory/markdown/           │
                │  sessions/  journal/  briefing/  ritual/     │
                │  weather/   custom/                          │  ← source of truth
                └────────────┬─────────────────────────────────┘
                             │ ChunkMarkdown()
                             ▼
                ┌─────────────────────────────────────────────┐
                │  qmd.Chunk[]                                 │
                │  - id (hash(path:start:end:hash:model_ver))  │
                │  - content / source_path / line_range        │
                │  - prev_id / next_id                          │
                │  - embedding_pending flag                     │
                └────────────┬─────────────────────────────────┘
                             │ store.Writer.Insert()
                             ▼
       ┌──────────────────────────────────────────────────────────┐
       │       ~/.mink/memory/{agent}.sqlite (mode 0600)          │
       │                                                          │
       │   chunks (regular table)     ─┐                          │
       │   files (regular table)      ─┤  same SQLite instance     │
       │   chunks_fts (FTS5 vtab)     ─┤  single-writer mutex      │
       │   embeddings (vec0 vtab)     ─┤                          │
       │   metadata (key/value)       ─┘                          │
       └────────────┬─────────────────────────────────────────────┘
                    │
                    ▼
   ┌─── retrieval.search (BM25)  ────────────────┐
   │                                              │
   │    retrieval.vsearch (vector only)           │
   │       ▲                                      │
   │       │ ollama.Client.Embed(q)               │
   │       │                                      │
   │    retrieval.query (hybrid + MMR + decay)    │
   │       │                                      │
   └───────┴──────────────────────────────────────┘
                    │
                    ▼
              top-k results → CLI / TUI / JSON
```

### 7.3 SQLite Schema (4 핵심 + 2 virtual)

```sql
-- regular tables
CREATE TABLE IF NOT EXISTS files (
    file_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    collection     TEXT NOT NULL,
    source_path    TEXT NOT NULL UNIQUE,
    content_hash   TEXT NOT NULL,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
    chunk_id       TEXT PRIMARY KEY,            -- hash(path:start:end:hash:model_ver)
    file_id        INTEGER NOT NULL REFERENCES files(file_id) ON DELETE CASCADE,
    start_line     INTEGER NOT NULL,
    end_line       INTEGER NOT NULL,
    content        TEXT NOT NULL,
    prev_chunk_id  TEXT,
    next_chunk_id  TEXT,
    embedding_pending INTEGER NOT NULL DEFAULT 1,
    model_version  TEXT NOT NULL,
    created_at     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- virtual tables (sqlite-vec + FTS5)
CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0(
    chunk_id TEXT PRIMARY KEY,
    embedding FLOAT[768]                         -- nomic-embed-text default
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    chunk_id UNINDEXED,
    content,
    tokenize = 'unicode61'
);

CREATE INDEX IF NOT EXISTS idx_chunks_file_id   ON chunks(file_id);
CREATE INDEX IF NOT EXISTS idx_chunks_model_ver ON chunks(model_version);
CREATE INDEX IF NOT EXISTS idx_files_collection ON files(collection);
```

### 7.4 검색 모드 흐름

| Mode | 흐름 |
|---|---|
| `search` (BM25) | `chunks_fts MATCH q` → BM25 score 기준 정렬 → top-k → snippet 생성 → 반환 |
| `vsearch` | ollama.Embed(q) → `embeddings WHERE k=K MATCH ?` (sqlite-vec k-NN) → cosine score → 반환 |
| `query` (hybrid) | parallel: (a) BM25 top-3K, (b) vec top-3K → 후보 union → score = α·cosine + β·bm25_norm + γ·decay → MMR top-k → 반환 |

---

## 8. 의존성·통합

| 의존 SPEC | 인터페이스 | 본 SPEC 에서의 영향 |
|---|---|---|
| **USERDATA-MIGRATE-001** | export 항목 추가 | M5 에서 USERDATA-MIGRATE-001 amendment 발행: `memory/markdown/`, `memory/*.sqlite` 추가 |
| **JOURNAL-001** | publish hook | `internal/memory/hook/publish.go` 에 `OnJournalPublished(entry)` 등록 |
| **BRIEFING-001** | publish hook | `OnBriefingPublished(narrative)` |
| **WEATHER-001** | publish hook | `OnWeatherPublished(summary)` |
| **RITUAL-001** | publish hook | `OnRitualPublished(record)` |
| **LLM-ROUTING-V2** | session export | opt-in 시 응답 stream → sanitized markdown → `sessions/` collection |
| **AUTH-CREDENTIAL-001** | ollama endpoint | localhost 가정, 평문 config 로 충분 |
| **CLI-TUI-003** | subcommand 추가 | `mink memory` 7-subcommand wiring + TUI `/memory` slash command |
| **CROSSPLAT-001** | OS 가드 | macOS/Linux/Windows 모두 sqlite-vec 빌드 정상성 검증 (§5.1) |
| **LICENSE-COPYLEFT-001** | AGPL 헌장 | 모든 신규 소스 파일 헤더 AGPL-3.0-only 표기 |

---

## 9. 위험 (Risks)

| 위험 | 가능성 | 영향 | 완화 |
|---|---|---|---|
| **R1**: ollama 미설치 사용자의 query 모드 UX 저하 | 高 | 中 | BM25-only graceful degrade + UX 경고 + ollama 설치 가이드 (CLI/Web) |
| **R2**: sqlite-vec 인덱스 파일 손상 | 低 | 高 | markdown source-of-truth + `mink memory reindex` 완전 복원 |
| **R3**: corpus 1M chunk 초과 시 성능 저하 | 低 | 中 | 1M chunk 까지 검증된 sqlite-vec. 초과 시 prune 또는 별도 SPEC 으로 sharding |
| **R4**: PII 누출 (한국·영어 외 언어권) | 中 | 高 | M5 에서 redact 패턴 다국어 확장 SPEC 후속 발행. launch 직후 사용자 명시적 opt-in 강제 |
| **R5**: ClawMem vault schema 변경 | 中 | 低 | `--vault-format=clawmem` 은 옵션 (P2). schema 변경 시 read-only fallback |
| **R6**: ollama 임베딩 모델 변경 시 stale chunk 폭증 | 中 | 中 | chunk_id 에 model_version 포함 → 자동 식별 + `reindex --collection` 점진 마이그레이션 |
| **R7**: cgo 빌드 복잡도 (Windows native) | 中 | 中 | CROSSPLAT-001 §5.1 가드 + CI Windows runner 에서 sqlite-vec 빌드 검증 |
| **R8**: 단일 writer mutex 로 인한 concurrent CLI 지연 | 低 | 低 | 사용자 단일이므로 동시 invocation 드묾. 100ms 이내 release 보장 (REQ-MEM-030) |

---

## 10. 결정 사항 (ADR cross-link)

- **ADR-001 (2026-05-16)**: on-device 가중치 학습 (QLoRA / LoRA / RLHF) 전면 폐기. 자기진화는 (a) 외부 GOAT LLM 라우팅, (b) QMD 메모리 인덱스 두 축으로 재정의. → 본 SPEC §2.1
- **ADR-MEM-001 (본 SPEC 신설)**: storage = Markdown source-of-truth + SQLite (sqlite-vec + FTS5) hybrid. 대안 (LanceDB/Chroma/Qdrant/Faiss) 모두 single-file 운영 깨짐으로 폐기.
- **ADR-MEM-002 (본 SPEC 신설)**: 임베딩 backend = ollama local sidecar. cloud API 폐기 (privacy first).
- **ADR-MEM-003 (본 SPEC 신설)**: ClawMem 호환은 P2 optional. launch 차단 요소 아님.

---

## 11. TRUST 5 정합

- **Tested**: M1~M5 각 마일스톤별 unit + integration test. 목표 coverage ≥ 85%. characterization test 는 SQLite schema 변환 시점에만 적용 (legacy 코드 없음 → 주로 spec test).
- **Readable**: Go 표준 명명 (`PascalCase` exported, `camelCase` unexported). 모든 export 함수에 godoc (영문, language.yaml `code_comments: en`).
- **Unified**: `gofmt` + `go vet` + `golangci-lint`. import order: stdlib → 3rd party → internal.
- **Secured**: REQ-MEM-028/029/031 unwanted behavior 가드. file mode 0600/0700 강제 테스트. PII 마스킹 unit test 한국·영어 케이스 50개 이상.
- **Trackable**: 모든 commit conventional commit + `SPEC: SPEC-MINK-MEMORY-QMD-001`, `REQ: REQ-MEM-NNN`, `AC: AC-MEM-NNN` trailer. @MX:ANCHOR 는 `Indexer`, `Searcher` 같은 public API. @MX:WARN 은 writer mutex 영역.

---

## 12. References

- `.moai/specs/SPEC-MINK-MEMORY-QMD-001/research.md` (본 SPEC 의 사전 조사 결과)
- `.moai/adr/ADR-001-mink-self-evolution-pivot.md` (가중치 학습 폐기 결정)
- `.moai/specs/SPEC-MINK-LLM-ROUTING-V2-AMEND-001/spec.md` (자기진화 ½축, 외부 LLM 라우팅)
- `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/spec.md` (~/.mink/ 디렉토리 표준)
- `.moai/specs/SPEC-MINK-JOURNAL-001/spec.md`
- `.moai/specs/SPEC-MINK-BRIEFING-001/spec.md`
- `.moai/specs/SPEC-MINK-WEATHER-001/spec.md`
- `.moai/specs/SPEC-MINK-RITUAL-001/spec.md`
- `.moai/specs/SPEC-MINK-CLI-TUI-003/spec.md`
- `.moai/specs/SPEC-MINK-CROSSPLAT-001/spec.md` §5.1 OS 가드
- `.moai/specs/SPEC-MINK-LICENSE-COPYLEFT-001/spec.md` (AGPL-3.0 헌장)
- OpenClaw QMD Spec v1.2 (github.com/openclaw/openclaw-spec, 2025-Q4)
- ClawMem MCP Server (github.com/openclaw/clawmem, 2026-Q1)
- sqlite-vec (github.com/asg017/sqlite-vec, MIT)
- mattn/go-sqlite3 v1.14+ (MIT)
- Ollama Embed API (`/api/embeddings`)
- Carbonell & Goldstein 1998 (MMR 원논문)
