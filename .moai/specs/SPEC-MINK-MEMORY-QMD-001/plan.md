# Plan — SPEC-MINK-MEMORY-QMD-001

## 0. Overview

본 plan 은 SPEC-MINK-MEMORY-QMD-001 (QMD 기반 메모리 인덱스) 구현을 5개 마일스톤으로 분해한다. 각 마일스톤은 acceptance.md 의 AC ID 와 1:n 매핑된다. 시간 추정은 금지되며 priority 라벨 (P0/P1/P2) 만 사용한다.

라이선스: AGPL-3.0-only (LICENSE-COPYLEFT-001).

---

## 1. Milestones

### M1 — Schema + CRUD foundation [P0]

**목표**: SQLite schema 생성, markdown chunking, `mink memory add` 동작. ollama / vsearch / hybrid 없이도 vault 운영 가능한 최소 경계 확정.

**산출물**:
- `internal/memory/memory.go` — public API skeleton (`Indexer`, `Searcher` interface)
- `internal/memory/config.go` — `memory.yaml` 로딩 + defaults
- `internal/memory/qmd/{chunk.go, id.go, neighbor.go}` — chunking + chunk_id
- `internal/memory/sqlite/{store.go, schema.sql, writer.go}` — open/close, schema migration, single-writer mutex
- `internal/memory/cli/add.go` — `mink memory add` subcommand
- `.moai/config/sections/memory.yaml` — config 신설

**AC 매핑**: AC-MEM-001 .. AC-MEM-010, AC-MEM-014, AC-MEM-025, AC-MEM-029, AC-MEM-030, AC-MEM-032

**완료 조건**:
- markdown 파일 1개를 `mink memory add` 로 vault 에 등록 시 chunks 테이블에 N rows insert
- mode 0600/0700 강제 검증 통과
- single-writer mutex 동시 invocation race test 통과

---

### M2 — BM25 search [P0]

**목표**: FTS5 기반 BM25-only `mink memory search` 동작. ollama 미설치 환경에서도 launch ready.

**산출물**:
- `internal/memory/sqlite/reader.go` — FTS5 query builder
- `internal/memory/retrieval/search.go` — BM25-only mode 구현
- `internal/memory/cli/search.go` — `mink memory search` (default mode = search 일 때)
- `internal/memory/redact/{pii.go, pii_test.go}` — PII 마스킹 파이프라인 (이후 M3 session export 에서 사용)

**AC 매핑**: AC-MEM-015 (search mode), AC-MEM-019, AC-MEM-028, AC-MEM-031, AC-MEM-033

**완료 조건**:
- `mink memory search "keyword"` BM25 결과 top-10 반환
- snippet 256-char 절단 동작
- PII 마스킹 unit test 50개 이상 (한국 전화/주민/이메일/카드/OAuth)

---

### M3 — Ollama embed + vsearch [P1]

**목표**: ollama embed sidecar 통합, vector-only `vsearch` 모드 동작. graceful degrade 검증.

**산출물**:
- `internal/memory/ollama/{client.go, fallback.go}` — POST `/api/embeddings`, healthcheck, BM25 fallback decision
- `internal/memory/sqlite/schema.sql` — `embeddings vec0` virtual table 추가 (sqlite-vec extension load)
- `internal/memory/retrieval/vsearch.go` — k-NN 검색
- `internal/memory/cli/search.go` — `--mode vsearch` 옵션
- embedding_pending 플래그 backfill worker (best-effort, foreground 100ms 이내 반환)

**AC 매핑**: AC-MEM-015 (vsearch mode), AC-MEM-019, AC-MEM-023, AC-MEM-035

**완료 조건**:
- ollama 미설치 환경에서 `mink memory search --mode vsearch` 호출 시 stderr 경고 + BM25 fallback 반환
- ollama 설치 환경에서 vsearch 가 의미 기반 매칭 (테스트 corpus 검증)
- `mxbai-embed-large` 모델 교체 시 stale chunk 자동 식별

---

### M4 — Hybrid query + MMR + temporal decay [P1]

**목표**: 3-mode 검색 완성. `query` (hybrid) 가 default mode. MMR 다양성, temporal decay 반감기 30일.

**산출물**:
- `internal/memory/retrieval/query.go` — hybrid score 계산
- `internal/memory/retrieval/mmr.go` — MMR re-ranking
- `internal/memory/retrieval/decay.go` — exponential temporal decay
- `internal/memory/cli/search.go` — `--mode query` default 적용
- latency benchmark test (corpus 10K chunk, top-10 ≤ 200ms)

**AC 매핑**: AC-MEM-008, AC-MEM-009, AC-MEM-010, AC-MEM-013

**완료 조건**:
- query 모드 top-10 평균 latency ≤ 200ms (corpus ≤ 10K chunk)
- MMR diversity 검증 (top-10 결과 중 동일 source_path 비율 ≤ 30%)
- temporal decay 검증 (동일 score 시 최근 chunk 우선)

---

### M5 — Reindex + export + import + ClawMem 호환 + publish hooks [P1+P2]

**목표**: 운영 라이프사이클 완성. 인덱스 손상 복원, vault 백업/마이그레이션, ClawMem 호환, JOURNAL/BRIEFING/WEATHER/RITUAL publish hook 통합.

**산출물**:
- `internal/memory/cli/reindex.go` — markdown 으로부터 인덱스 완전 재생성
- `internal/memory/cli/export.go` — vault tarball + sqlite snapshot
- `internal/memory/cli/import.go` — schema 호환 검증 후 merge
- `internal/memory/cli/stats.go` — per-collection 통계
- `internal/memory/cli/prune.go` — `--before` 기준 마크다운+인덱스 transactional 삭제
- `internal/memory/clawmem/{mirror.go, schema.go}` — `--vault-format=clawmem` mirror writer
- `internal/memory/hook/{publish.go, publish_test.go}` — JOURNAL/BRIEFING/WEATHER/RITUAL publish hook
- LLM-ROUTING-V2 session auto-export (opt-in)
- USERDATA-MIGRATE-001 amendment (export 항목에 `memory/` 추가)
- TUI `/memory` slash command (read-only peek)

**AC 매핑**: AC-MEM-016, AC-MEM-017, AC-MEM-018, AC-MEM-020, AC-MEM-021, AC-MEM-022, AC-MEM-024, AC-MEM-026, AC-MEM-027, AC-MEM-034, AC-MEM-036, AC-MEM-037, AC-MEM-038

**완료 조건**:
- 인덱스 임의 삭제 후 `mink memory reindex` 1:1 복원 (chunk 수, content hash 일치)
- JOURNAL/BRIEFING/WEATHER/RITUAL publish → ≤ 1 sec 내 색인 (integration test)
- ClawMem MCP 서버에서 본 SPEC 출력 vault 를 read 모드로 mount 성공 (매뉴얼 검증)
- USERDATA-MIGRATE-001 amendment PR 발행

---

## 2. Milestone Priority Order

```
M1 (P0) ─▶ M2 (P0) ─▶ M3 (P1) ─▶ M4 (P1) ─▶ M5 (P1+P2)
```

P0 마일스톤 (M1+M2) 완수 시점에 **launch 차단 해제**. P1+P2 (M3~M5) 는 launch 권장 / post-launch enhancement.

---

## 3. Technical Approach

### 3.1 cgo 정책

- 본 SPEC 의 cgo 사용은 sqlite-vec + mattn/go-sqlite3 **한정**.
- Go 단일 정책은 유지 (Rust crate 도입 없음).
- 빌드 태그: `//go:build cgo` 로 보호. pure-Go 빌드 시도는 CI 에서 명시적으로 BM25-only 경고 + build fail 처리.

### 3.2 SQLite 파일 분리 전략

- 인덱스 파일 1개 per agent profile (`~/.mink/memory/{agent}.sqlite`)
- 멀티 collection 은 동일 sqlite 파일 안에서 `files.collection` column 으로 구분
- 향후 collection 분리가 필요하면 별도 SPEC

### 3.3 단일 writer mutex

- `internal/memory/sqlite/writer.go` 의 `sync.Mutex` (process-level)
- 동시 CLI invocation 은 file lock (`flock` on Unix, `LockFileEx` on Windows) 으로 process-간 serialize
- 100ms 이내 release 강제 (REQ-MEM-030)

### 3.4 Chunking 알고리즘

```
1. Markdown parse → AST
2. Walk AST, segment by:
   - heading boundary (H1/H2/H3)
   - paragraph boundary
   - 512-token hard cap
3. 각 segment 에 prev/next pointer 부여
4. chunk_id = sha256(source_path:start_line:end_line:content_hash:model_version)[:16]
```

### 3.5 Hybrid score 계산

```
α=0.7, β=0.3 (config 노출)
γ_decay(t) = exp(- (now - t) / (30 days in seconds))
score(q, c) = α·cosine(emb(q), emb(c)) + β·bm25_norm(q, c) + γ·γ_decay(c.created_at)
# additive form per spec.md REQ-MEM-009. γ 는 별도 weight (multiplicative 아님). γ default 는 0.0 (decay 비활성) 이며 사용자가 명시 활성 시 0<γ≤1. M4 진입 시 default 재검토.
```

bm25_norm 은 corpus-level normalize: `bm25_norm = bm25_raw / max(bm25_raw_in_top_K)`.

### 3.6 MMR re-ranking

```
선택 집합 S 가 비어있을 때:
  s1 = argmax_c score(q, c)
이후 k-1 번 반복:
  s_next = argmax_c [ λ·score(q, c) - (1-λ)·max_{s in S} cosine(c, s) ]
λ = 0.7 (config)
```

### 3.7 ClawMem mirror 전략

- `--vault-format=clawmem` 활성화 시 모든 `Indexer.Insert` 가 `~/.clawmem/vault/` 에 동일 markdown 을 추가 write
- write 순서: vault 먼저 → mirror 후. mirror 실패는 stderr warn, 메인 write 는 commit
- ClawMem schema 변경 detect 시 mirror 모드 자동 disable + warn

### 3.8 USERDATA-MIGRATE-001 amendment 필요 항목

M5 에서 발행할 USERDATA-MIGRATE-001-AMEND-NNN:
- export 매니페스트에 `~/.mink/memory/markdown/` 추가
- export 매니페스트에 `~/.mink/memory/*.sqlite` 추가
- import 시 schema 검증 + merge 정책

---

## 4. Cross-SPEC Dependencies (구현 순서 제약)

```
LICENSE-COPYLEFT-001 (헌장)         ─▶ 본 SPEC 모든 소스 헤더
USERDATA-MIGRATE-001                 ─▶ M5 amendment 발행
CROSSPLAT-001 §5.1                   ─▶ M1~M5 모든 CI matrix
AUTH-CREDENTIAL-001                  ─▶ M3 ollama endpoint (평문 충분)
JOURNAL-001 / BRIEFING-001 /
WEATHER-001 / RITUAL-001             ─▶ M5 publish hook wiring
LLM-ROUTING-V2 (AMEND-001 status)    ─▶ M5 session auto-export (opt-in)
CLI-TUI-003                          ─▶ M1~M5 subcommand + TUI peek
```

---

## 5. Risks (구현 측면)

| 위험 | 대응 |
|---|---|
| sqlite-vec extension 동적 로딩이 Windows 에서 실패 | M3 진입 전 CROSSPLAT-001 §5.1 CI matrix 에서 사전 검증 |
| cgo 빌드 속도 저하로 dev loop 지연 | `go build -tags=cgo` 캐시 + `-trimpath` 활성 |
| ollama API 응답 5초+ 지연 | M3 의 client.go 에 5-sec timeout + circuit breaker |
| `chunks_fts MATCH` 한글 토큰 분리 부정확 | `tokenize='unicode61'` 검증 + 한글 corpus test |
| ClawMem schema spec 갱신 시 mirror 깨짐 | M5 의 mirror writer 에 schema version 감지 + fallback |
| 단일 writer mutex 가 1초+ 잡힘 (대량 ingest) | reindex worker 는 별도 process / read snapshot 보호 (REQ-MEM-022) |

---

## 6. Definition of Done (SPEC 전체)

- [ ] M1~M5 모든 AC GREEN
- [ ] coverage ≥ 85%
- [ ] golangci-lint clean
- [ ] CROSSPLAT-001 CI matrix (macOS/Linux/Windows) 통과
- [ ] USERDATA-MIGRATE-001-AMEND-NNN 머지
- [ ] CLI-TUI-003 의 `mink memory` 7-subcommand 패리티
- [ ] ClawMem MCP 서버에서 vault mount 매뉴얼 검증 캡쳐
- [ ] README + `.moai/docs/memory.md` 작성
- [ ] AGPL-3.0 헤더 모든 신규 소스에 존재
- [ ] sync PR merge → status `implemented`

---

## 7. Out-of-Plan (이 plan 에서 다루지 않음)

- LLM 기반 chunk summarization at ingest path
- 외부 cloud embedding API
- web UI 일상 운용
- multi-user 동시 접근
- 다국어 PII 마스킹 확장 (한국어/영어 외)
- cross-device sync 자동화 (사용자가 markdown vault 를 직접 git/Syncthing 동기화)

→ 위 항목들은 본 SPEC 완료 후 별도 SPEC 발행.
