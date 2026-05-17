# Tasks — SPEC-MINK-MEMORY-QMD-001

본 문서는 plan.md 의 5개 마일스톤을 Go 패키지·함수 수준 작업 단위로 분해한다. 각 task 는 owner agent 추정값과 영향 파일 경로를 포함한다.

라이선스: AGPL-3.0-only. 모든 신규 소스 파일 헤더에 AGPL 표기.

---

## M1 — Schema + CRUD foundation [P0]

### T1.1 — config 스켈레톤 신설
- 파일: `.moai/config/sections/memory.yaml`
- 키: `memory.vault_path`, `memory.index_path`, `memory.embedding.{enabled,model,endpoint}`, `memory.retrieval.{alpha,beta,decay_half_life_days,mmr_lambda}`, `memory.session_autoexport.enabled`, `memory.clawmem_compat.{enabled,vault_path}`
- 작업: defaults 정의 + config loader unit test
- agent: expert-backend

### T1.2 — Public API skeleton
- 파일: `internal/memory/memory.go`
- 내용: `Indexer interface { Ingest(ctx, collection, sourcePath) error; Insert(ctx, chunk Chunk) error }` / `Searcher interface { Search(ctx, q, opts) ([]Result, error) }` / `Chunk struct`
- agent: expert-backend

### T1.3 — Chunking 알고리즘
- 파일: `internal/memory/qmd/chunk.go`, `chunk_test.go`
- 함수: `ChunkMarkdown(content string, opts ChunkOpts) []Chunk`
- 알고리즘: heading boundary → paragraph boundary → 512-token hard cap. prev/next pointer 부여
- test: 한국어/영어 markdown 8개 케이스
- agent: expert-backend

### T1.4 — chunk_id hash
- 파일: `internal/memory/qmd/id.go`, `id_test.go`
- 함수: `ChunkID(sourcePath string, startLine, endLine int, contentHash, modelVersion string) string`
- 알고리즘: `sha256(...)[:16]` hex
- test: 동일 입력 → 동일 ID, model_version 변경 → 다른 ID
- agent: expert-backend

### T1.5 — SQLite store + schema
- 파일: `internal/memory/sqlite/store.go`, `schema.sql`, `store_test.go`
- 함수: `Open(path string) (*Store, error)`, `Close()`, `MigrateSchema(ctx)`
- schema: §7.3 의 4 regular + 2 virtual table
- mode 0600 강제 (REQ-MEM-002)
- 부모 디렉토리 mode 0700 강제 (REQ-MEM-003)
- agent: expert-backend

### T1.6 — Single-writer mutex + writer transactions
- 파일: `internal/memory/sqlite/writer.go`, `writer_test.go`
- 함수: `Writer.Insert(ctx, chunk Chunk) error`, `Writer.UpsertFile(ctx, file File) error`
- 동시성: process-level `sync.Mutex` + cross-process file lock (`flock` / `LockFileEx`)
- 100ms timeout (REQ-MEM-030)
- agent: expert-backend

### T1.7 — `mink memory add` CLI subcommand
- 파일: `internal/memory/cli/add.go`, `add_test.go`
- usage: `mink memory add --collection {c} --source {path}`
- 동작: 파일 mode 0600 강제 → vault 에 hardlink 또는 copy → chunk → insert
- REQ-MEM-032: 충돌 시 rejection error
- agent: expert-backend

### T1.8 — `cgo` build tag 가드
- 파일: `internal/memory/sqlite/build_cgo.go`, `build_nocgo.go`
- 효과: pure-Go 빌드 시 명시적 BM25-only 경고 + build fail
- agent: expert-devops

---

## M2 — BM25 search [P0]

### T2.1 — FTS5 reader
- 파일: `internal/memory/sqlite/reader.go`, `reader_test.go`
- 함수: `Reader.SearchBM25(ctx, q string, k int) ([]Hit, error)`
- 토큰화: `tokenize='unicode61'` (한글 안전)
- agent: expert-backend

### T2.2 — search mode dispatcher
- 파일: `internal/memory/retrieval/search.go`, `search_test.go`
- 함수: `RunBM25(ctx, q, opts) ([]Result, error)`
- 256-char snippet 생성
- agent: expert-backend

### T2.3 — `mink memory search` CLI (BM25 path)
- 파일: `internal/memory/cli/search.go` (M3/M4 에서 확장)
- 기본 동작: ollama 미설치 시 자동 BM25 fallback
- agent: expert-backend

### T2.4 — PII 마스킹 파이프라인
- 파일: `internal/memory/redact/pii.go`, `pii_test.go`
- 패턴: 한국 전화/주민/이메일/카드번호/OAuth token
- test corpus: 50+ 케이스 (한국 + 영어)
- agent: expert-security

---

## M3 — Ollama embed + vsearch [P1]

### T3.1 — Ollama client
- 파일: `internal/memory/ollama/client.go`, `client_test.go`
- 함수: `Client.Embed(ctx, model string, text string) ([]float32, error)`
- timeout: 5초, circuit breaker (3회 연속 실패 시 30초 disable)
- agent: expert-backend

### T3.2 — Fallback decision
- 파일: `internal/memory/ollama/fallback.go`, `fallback_test.go`
- 함수: `ShouldFallbackToBM25(err error) bool`
- 동작: ollama 미설치/응답 없음/timeout → BM25 fallback + stderr warn (REQ-MEM-019)
- agent: expert-backend

### T3.3 — sqlite-vec extension load + embeddings vec0 table
- 파일: `internal/memory/sqlite/schema.sql` (extension load), `store.go` (init)
- 동작: `SELECT load_extension('vec0')` (mattn/go-sqlite3 의 `_load_extension` opt)
- macOS/Linux/Windows native 검증
- agent: expert-devops

### T3.4 — vsearch retrieval
- 파일: `internal/memory/retrieval/vsearch.go`, `vsearch_test.go`
- 함수: `RunVector(ctx, q, opts) ([]Result, error)`
- sqlite-vec k-NN: `SELECT ... FROM embeddings WHERE embedding MATCH ? AND k=K`
- agent: expert-backend

### T3.5 — embedding_pending backfill worker
- 파일: `internal/memory/sqlite/backfill.go`, `backfill_test.go`
- 동작: ingest path 가 ollama 응답 5초+ 시 `embedding_pending=1` 로 마크하고 background goroutine 으로 backfill (CLI 호출 반환 100ms 이내 보장)
- agent: expert-backend

### T3.6 — `--mode vsearch` CLI flag
- 파일: `internal/memory/cli/search.go` (확장)
- agent: expert-backend

---

## M4 — Hybrid query + MMR + temporal decay [P1]

### T4.1 — temporal decay 계산
- 파일: `internal/memory/retrieval/decay.go`, `decay_test.go`
- 함수: `DecayFactor(createdAt time.Time, halfLife time.Duration) float64`
- 식: `exp(-Δt / halfLife)` (halfLife 30 days default)
- agent: expert-backend

### T4.2 — hybrid score
- 파일: `internal/memory/retrieval/query.go`, `query_test.go`
- 함수: `RunHybrid(ctx, q, opts) ([]Result, error)`
- 식: `α·cosine + β·bm25_norm * γ_decay`
- agent: expert-backend

### T4.3 — MMR re-ranking
- 파일: `internal/memory/retrieval/mmr.go`, `mmr_test.go`
- 함수: `MMRRerank(candidates, queryEmbed, lambda) []Result`
- λ=0.7 default
- agent: expert-backend

### T4.4 — `--mode query` default 적용
- 파일: `internal/memory/cli/search.go` (default mode = query)
- 변경: `mink memory search "..."` (no flag) → query 모드 (REQ-MEM-008)
- agent: expert-backend

### T4.5 — latency benchmark
- 파일: `internal/memory/retrieval/bench_test.go`
- corpus: 10K chunk synthetic
- 목표: top-10 평균 ≤ 200ms (G2)
- agent: expert-performance

---

## M5 — Reindex + export + import + ClawMem 호환 + publish hooks [P1+P2]

### T5.1 — `mink memory reindex`
- 파일: `internal/memory/cli/reindex.go`, `reindex_test.go`
- 동작: chunks WHERE model_version != current → drop → re-chunk → re-embed
- regression test: 인덱스 임의 삭제 후 1:1 복원 (chunk 수, content hash 일치)
- agent: expert-backend

### T5.2 — `mink memory export`
- 파일: `internal/memory/cli/export.go`, `export_test.go`
- format: tarball (markdown + sqlite snapshot)
- `--format=clawmem` 옵션: ClawMem vault 호환
- agent: expert-backend

### T5.3 — `mink memory import`
- 파일: `internal/memory/cli/import.go`, `import_test.go`
- 동작: schema 호환 검증 → merge
- 충돌 시 explicit error (REQ-MEM-038)
- agent: expert-backend

### T5.4 — `mink memory stats`
- 파일: `internal/memory/cli/stats.go`
- 출력: per-collection chunk count + total embedding size + oldest/newest timestamp
- agent: expert-backend

### T5.5 — `mink memory prune`
- 파일: `internal/memory/cli/prune.go`, `prune_test.go`
- 동작: `--before {date}` → markdown 삭제 + chunks 삭제 (transactional)
- REQ-MEM-034: markdown 삭제와 index 삭제 동일 transaction
- agent: expert-backend

### T5.6 — ClawMem mirror writer
- 파일: `internal/memory/clawmem/mirror.go`, `mirror_test.go`
- 동작: `--vault-format=clawmem` 활성 시 mirror write to `~/.clawmem/vault/`
- agent: expert-backend

### T5.7 — ClawMem schema 변환
- 파일: `internal/memory/clawmem/schema.go`, `schema_test.go`
- schema version detect + fallback to read-only mirror on mismatch
- agent: expert-backend

### T5.8 — Publish hooks (JOURNAL / BRIEFING / WEATHER / RITUAL)
- 파일: `internal/memory/hook/publish.go`, `publish_test.go`
- 함수: `OnJournalPublished`, `OnBriefingPublished`, `OnWeatherPublished`, `OnRitualPublished`
- 각 SPEC 의 publish 시점 wiring: 별도 PR 로 wiring
- integration test: publish → ≤ 1 sec 내 색인
- agent: expert-backend

### T5.9 — LLM-ROUTING-V2 session auto-export
- 파일: `internal/memory/hook/session.go`, `session_test.go`
- opt-in: `memory.session_autoexport.enabled = true`
- 출력: `sessions/{YYYY-MM-DD}/{provider}-{session_id}.md` (PII 마스킹 후)
- agent: expert-backend

### T5.10 — USERDATA-MIGRATE-001 amendment
- 별도 SPEC 디렉토리: `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001-AMEND-NNN/`
- 내용: export 매니페스트에 `memory/markdown/`, `memory/*.sqlite` 추가
- agent: manager-spec

### T5.11 — TUI `/memory` slash command (peek)
- 파일: `internal/tui/slash/memory.go`, `memory_test.go`
- 동작: read-only search peek (top-5 결과)
- CLI-TUI-003 wiring
- agent: expert-frontend

### T5.12 — README + `.moai/docs/memory.md`
- 작성: ko (documentation language). 사용자용 가이드.
- agent: manager-docs

---

## Summary

| 마일스톤 | task 수 | priority |
|---|---|---|
| M1 | 8 | P0 |
| M2 | 4 | P0 |
| M3 | 6 | P1 |
| M4 | 5 | P1 |
| M5 | 12 | P1+P2 |
| **Total** | **35** | — |
