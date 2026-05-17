# Progress — SPEC-MINK-MEMORY-QMD-001

## Status

- Version: 0.1.0
- Lifecycle: spec-first
- Phase: M4 implemented (run 진행 중)
- Overall completion: **~80%** (M1 + M2 + M3 + M4 코드 GREEN, M5 잔여)
- Created: 2026-05-16
- Last updated: 2026-05-17

---

## Milestones

| ID | 제목 | Priority | Status | AC 매핑 | 진척 |
|----|------|----------|--------|---------|------|
| M1 | Schema + CRUD foundation | P0 | ✅ Done | AC-MEM-001..014, 025, 029, 030, 032 | 100% |
| M2 | BM25 search | P0 | ✅ Done | AC-MEM-015 (search mode), 028 | 100% |
| M3 | Ollama embed + vsearch | P1 | ✅ Done | AC-MEM-015 (vsearch), 019, 023, 031, 035 | 100% |
| M4 | Hybrid query + MMR + temporal decay | P1 | ✅ Done | AC-MEM-008, 009, 010, 013 | 100% |
| M5 | Reindex + export + import + ClawMem + publish hooks | P1+P2 | ⏸️ Not started | AC-MEM-016..018, 020, 021, 022, 024, 026, 027, 033, 034, 036, 037, 038 | 0% |

Legend: ⏸️ Not started · 🟡 In progress · ✅ Done · ❌ Blocked

---

## Acceptance Criteria

- Total ACs: 38
- GREEN: 0 (0%)
- IN PROGRESS: 0
- BLOCKED: 0

| Section | AC 수 | GREEN |
|---|---|---|
| A. Ubiquitous | 13 | 0 |
| B. Event-Driven | 8 | 0 |
| C. State-Driven | 6 | 0 |
| D. Unwanted | 7 | 0 |
| E. Optional | 4 | 0 |

---

## Recent Activity

- 2026-05-16: SPEC v0.1.0 plan 작성 (research.md / spec.md / plan.md / tasks.md / acceptance.md / progress.md). Status `planned`.
- 2026-05-17 (M1): PR #247 머지 — schema + chunking + sqlite/flock + `mink memory add` (T1.1~T1.9).
- 2026-05-17 (M2): T2.1 FTS5 reader (`sqlite/reader.go`) + T2.2 search dispatcher (`retrieval/`) + T2.3 `mink memory search` CLI + T2.4 PII redact (`redact/`, 58 table cases, coverage 98.2%) 구현. 8 패키지 `go test -race` GREEN, gofmt/vet/lint 클린. PR #250 머지 (f584d10).
- 2026-05-17 (M3): T3.1 Ollama client + 3-state circuit breaker + T3.2 fallback decision + T3.3 sqlite-vec extension load + 1024d schema migration + T3.4 vsearch retrieval + T3.5 embedding_pending backfill worker + T3.6 `--mode vsearch` CLI wiring. 11 신규 파일 + 6 수정 파일. 9 패키지 race-clean. ollama coverage 88.6%, retrieval 96.7%. PR #251 머지 (670452f).
- 2026-05-17 (M4): T4.1 temporal decay + T4.2 hybrid query (additive α·cosine + β·bm25_norm + γ·decay, ErrFellBackToBM25 graceful degrade) + T4.3 MMR re-ranking (λ=0.7, Jaccard fallback for missing embeddings) + T4.4 `--mode query` default + T4.5 latency benchmark + AC-MEM-013 regression. 10 신규 파일 + 2 수정. **bench 49.9μs (200ms 목표 대비 4000배)**. retrieval coverage **91.7%**.

---

## Blockers

(none)

---

## Next Action

1. M5 진입 — Reindex + export + import + ClawMem + publish hooks (P1+P2)
2. M5 task 12개: cli/{reindex,export,import,stats,prune} + clawmem mirror + hook/publish + LLM-ROUTING session export + USERDATA-MIGRATE amendment + TUI peek + docs
3. AC 매핑 13개: AC-MEM-016~018, 020~022, 024, 026/027, 033, 034, 036~038
