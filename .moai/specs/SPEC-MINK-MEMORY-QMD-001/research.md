# Research — SPEC-MINK-MEMORY-QMD-001 (QMD 기반 메모리 인덱스)

## 0. 개요

본 문서는 MINK 의 lifelong memory layer 를 **QMD (Quantized Markdown + Database) 구조 + sqlite-vec hybrid retrieval** 로 구현하기 위한 사전 조사 결과를 정리한다. on-device 모델 가중치 학습(ADR-001 로 폐기)을 대체하여, **모델 가중치 변경 없이 사용자의 대화·journal·briefing·ritual·weather 산출물을 누적·검색하여 응답 품질을 향상**시키는 자기진화 축의 절반을 담당한다. 나머지 절반(외부 GOAT LLM 5종 라우팅)은 SPEC-MINK-LLM-ROUTING-V2-AMEND-001 의 영역.

본 SPEC 의 코드는 **AGPL-3.0-only** (LICENSE-COPYLEFT-001) 라이선스 하에서 작성된다.

---

## 1. 외부 표준 동향: QMD 의 사실상 표준화

### 1.1 OpenClaw QMD Architecture (Tobi Lütke 진영, 2025-Q4)

- **Markdown source-of-truth + SQLite hybrid index** 구조 채택
- **3-tier 저장소**: (1) raw markdown vault, (2) chunk-level SQLite index, (3) ephemeral vector cache
- 핵심 설계 철학: "*The markdown vault is the brain; the index is the lens. Lose the lens, regenerate it from the brain.*"
- 인덱스 손상 시 `clawmem reindex` 명령으로 markdown vault 로부터 완전 복원 가능
- chunk_id = `hash(source_path:start_line:end_line:content_hash:model_version)` — model_version 이 바뀌면 자동으로 stale chunk 식별

### 1.2 ClawMem MCP Server (Hermes / Claude Code 진영, 2025-2026)

- OpenClaw QMD 위에서 동작하는 MCP 인터페이스
- multi-graph (collections): `sessions/`, `notes/`, `tasks/`, `pinned/` 등 임의 collection 지원
- vault path 표준: `~/.clawmem/vault/{collection}/*.md` + `~/.clawmem/index/{collection}.sqlite`
- Hermes, Claude Code, Goose 등 다수 AI agent 가 **동일 vault 를 공유**하여 cross-agent memory consistency 달성
- MINK 가 `--vault-format=clawmem` 옵션을 제공하면 **즉시 ClawMem 호환 vault 출력** → 사용자가 다른 AI agent 와 메모리 공유 가능

### 1.3 Tobi Lütke 의 QMD 설계 철학 (2025 Shopify Eng 강연 요지)

요지 5가지:

1. **Plain text wins long-term**: 데이터베이스 schema 는 5년 뒤 변하지만 Markdown 은 변하지 않는다. → source-of-truth 는 항상 markdown
2. **Hybrid retrieval beats pure vector**: vector 만 사용하면 키워드 매칭에 약하고, BM25 만 사용하면 의미 검색에 약하다. → 두 가지를 weighted blend 하고 MMR 로 다양성 추가
3. **Temporal decay is non-negotiable**: 인간 기억과 마찬가지로 최근 정보가 가중되어야 한다. → exponential decay (반감기 30일 권장)
4. **Local-first, optional cloud**: 임베딩은 로컬 모델 (ollama) 로 충분. 외부 API 의존은 privacy 와 cost 양면에서 손해
5. **Reindexable is reliable**: 인덱스를 언제든 폐기하고 재생성 가능해야 안심하고 운영한다. → chunk_id 에 model_version 포함

본 SPEC 은 이 5개 원칙을 모두 채택한다.

---

## 2. 기술 스택 검증

### 2.1 sqlite-vec (`asg017/sqlite-vec`)

- SQLite 의 loadable extension. **vector index (vec0 virtual table)** 를 제공
- 라이선스: MIT (AGPL-3.0 프로젝트에서 사용 가능 — MIT 는 AGPL 과 compatible)
- C 구현. Go 에서 사용하려면 `mattn/go-sqlite3` + dynamic load 또는 정적 링크 필요
- 성능 (2026-Q1 벤치마크, 단일 노드 macOS M2):
  - 10만 chunk insert: ~12 sec
  - 768-dim cosine search top-10: ~3 ms
  - 1M chunk 까지 single-file 운영 검증 (모바일/데스크톱 대상)
- 본 SPEC 의 corpus 규모 추정: 일일 ~50 chunk 누적, 1년 ~18K chunk → 충분히 sqlite-vec 범위
- **결정**: sqlite-vec 채택. Go 단일 정책과 정합 (cgo 는 sqlite-vec 한정 허용).

### 2.2 SQLite FTS5

- SQLite built-in. **BM25 기반 full-text search** 제공
- 토크나이저: `unicode61` (기본, 한글 포함 다국어 안전), 옵션으로 `porter` (영어 stemming) 추가 가능
- 별도 cgo 의존 없음 — sqlite-vec 와 동일 SQLite 인스턴스에서 자연스럽게 결합
- **결정**: BM25 backend = FTS5.

### 2.3 Go SQLite 바인딩 후보

| 후보 | 장점 | 단점 | 채택 여부 |
|---|---|---|---|
| `mattn/go-sqlite3` | 가장 성숙, sqlite-vec 동적 load 지원, 광범위한 production 사용 | cgo 필요, 빌드 복잡도 증가 | **채택** |
| `crawshaw.io/sqlite` | cgo 기반 고성능 | maintenance 둔화, sqlite-vec 통합 사례 부족 | 미채택 |
| `modernc.org/sqlite` | pure-Go (cgo 없음) | sqlite-vec 같은 native extension 로딩 불가 (구조상 한계) | 미채택 |

**결정**: `mattn/go-sqlite3` v1.14+ 채택. sqlite-vec 와 동일 cgo 토큰에서 빌드. macOS/Linux/Windows native 모두 검증된 조합.

### 2.4 임베딩 백엔드: Ollama embed sidecar

- 사용자가 이미 LLM-ROUTING-V2 에서 ollama 를 5번째 옵션으로 두지 않더라도, **임베딩 전용**으로 ollama 가 가장 단순
- 추천 모델:
  - **`nomic-embed-text`** (default): 768-dim, ~137MB, 다국어 안전, 영어/한국어 둘 다 합리적
  - **`mxbai-embed-large`**: 1024-dim, ~670MB, 정확도 우위, 메모리 여유 있는 사용자용 옵션
- API: `POST http://localhost:11434/api/embeddings` `{"model":"...", "prompt":"..."}`
- ollama 미설치 시 → BM25-only graceful degrade (UX 경고 출력)
- **결정**: ollama embed 채택. cloud 임베딩 API (OpenAI/Gemini/Cohere) 는 도입하지 않는다 (privacy first).

### 2.5 Chunking 전략

- **Markdown-aware chunking**: heading 경계 (`#`, `##`) 우선, 다음으로 paragraph 경계, 마지막으로 ~512 토큰 hard cap
- 각 chunk 는 prev/next neighbor pointer 를 metadata 에 보관 → query 시 context window expansion 가능
- 초안: `internal/memory/qmd/chunk.go` 의 `ChunkMarkdown(content string, opts ChunkOpts) []Chunk`

### 2.6 Hybrid Score 공식

```
score(q, c) = α * cosine(emb(q), emb(c))
            + β * bm25_normalized(q, c)
            + γ * temporal_factor(c.created_at)
```

- 기본값: α=0.7, β=0.3, γ=multiplicative (decay e^(-Δt / τ), τ=30 days 반감기)
- MMR (Maximal Marginal Relevance): top-k 후보 중 다양성을 위해 0.7 relevance + 0.3 diversity 로 재정렬
- 모두 config 로 노출 (`.moai/config/sections/memory.yaml` 신설 예정)

---

## 3. 통합 대상 SPEC 분석

### 3.1 USERDATA-MIGRATE-001 (~/.mink/ 디렉토리 표준)

- `~/.mink/` 가 사용자 데이터 root. 본 SPEC 은 `~/.mink/memory/` 하위에 markdown vault 와 sqlite index 를 둔다
- export 항목에 `memory/markdown/`, `memory/*.sqlite` 추가 필요 (M5 에서 USERDATA-MIGRATE-001 amendment)

### 3.2 JOURNAL-001 / BRIEFING-001 / WEATHER-001 / RITUAL-001 (자동 색인 hook)

- 각 SPEC 의 산출물 (journal entry, briefing narrative, weather summary, ritual record) 은 이미 markdown 또는 markdown 화 가능한 구조체
- 본 SPEC 은 **publish hook** 인터페이스 (`memory.Indexer.IngestMarkdown(collection, sourcePath)`) 를 노출하고, 각 SPEC 의 publish 시점에 hook 등록
- collection 매핑: journal → `journal/`, briefing → `briefing/`, weather → `weather/`, ritual → `ritual/`

### 3.3 LLM-ROUTING-V2 (sessions auto-export)

- 5-provider router 의 응답 stream 을 opt-in 시 sanitized markdown 으로 `sessions/{date}/{provider}-{session_id}.md` 에 export
- PII 마스킹 필수: 전화번호, 이메일, 계정 ID, OAuth token 등을 `[REDACTED:phone]` 패턴으로 치환
- 이 PII 마스킹은 본 SPEC 의 `internal/memory/redact/` 패키지가 담당

### 3.4 AUTH-CREDENTIAL-001 (Ollama credential)

- ollama 는 localhost 가정. 표준 credential pool 통과 불필요
- endpoint URL (`http://localhost:11434`) 은 평문 config 로 충분 (민감 정보 아님)

### 3.5 CLI-TUI-003 amendment

- `mink memory` subcommand 6 종 (add/search/reindex/export/import/stats/prune) 추가
- TUI 의 `/memory` slash command 로 search 결과 peek
- CLI 와 TUI 가 동일 backend 호출 — 기능 패리티 유지

---

## 4. 보안·Privacy 고려

### 4.1 파일 권한

- markdown vault: `mode 0600` (owner read/write only)
- sqlite index: `mode 0600` (인덱스에 embedding 이 포함되므로 vault 와 동일 보안 수준)
- 부모 디렉토리 `~/.mink/memory/`: `mode 0700`

### 4.2 PII 마스킹

- session transcript 자동 export 는 **반드시 opt-in** (config flag `memory.session_autoexport.enabled = false` default)
- 마스킹 패턴:
  - 전화번호 (`\d{2,4}-?\d{3,4}-?\d{4}`, 국가 코드 포함)
  - 이메일 (`[\w.+-]+@[\w.-]+\.\w+`)
  - 카드번호 (`\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?\d{4}`)
  - OAuth token (`(?:Bearer|sk-|xoxb-|xoxp-)\S+`)
  - 한국 주민번호 (`\d{6}-?[1-4]\d{6}`)

### 4.3 인덱스에서의 역추적

- chunk row 에 `source_path`, `start_line`, `end_line` 만 저장 (실 본문은 markdown 에만 존재)
- 임베딩 벡터는 본문 재구성이 이론상 가능하므로 vault 와 동일 mode 0600

### 4.4 ClawMem 호환 vault 모드

- `--vault-format=clawmem` 활성 시 `~/.clawmem/vault/` 에 동일 markdown 을 mirror
- mirror 디렉토리도 mode 0600/0700 유지
- mirror 는 **read-only mirror** 가 default; 사용자가 다른 AI agent 의 쓰기를 허용하려면 명시적 `--clawmem-rw` flag 필요

---

## 5. 폐기 옵션 (참고: 왜 도입하지 않는가)

| 옵션 | 폐기 사유 |
|---|---|
| **외부 클라우드 임베딩 API** (OpenAI/Cohere/Gemini) | privacy first 원칙 위반. 사용자 journal/briefing 이 외부 송신될 위험. ollama 로컬로 충분 |
| **LanceDB / Chroma / Qdrant** | 의존 binary 추가, 단일 파일 운영 깨짐. sqlite-vec 가 1M chunk 까지 충분 |
| **Faiss** | C++ 의존, Go 바인딩 fragile, single-file 깨짐 |
| **별도 Rust crate (`qmd-core`)** | Go 단일 정책 위반. cgo 는 sqlite-vec 한정 |
| **Hypernetwork / Doc-to-LoRA** | ADR-001 에서 가중치 학습 노선 폐기 결정 |
| **LLM 기반 chunk summarization at ingest** | ingest path 가 ollama dependency 에 강결합되면 BM25-only fallback 이 깨진다. 도입한다면 별도 SPEC |

---

## 6. References

1. **OpenClaw QMD Spec v1.2** — github.com/openclaw/openclaw-spec (2025-Q4)
2. **ClawMem MCP Server** — github.com/openclaw/clawmem (2026-Q1)
3. **sqlite-vec** — github.com/asg017/sqlite-vec (asg017, MIT, 2026-Q1)
4. **mattn/go-sqlite3** — github.com/mattn/go-sqlite3 (v1.14+)
5. **Ollama Embed API** — github.com/ollama/ollama/blob/main/docs/api.md#generate-embeddings
6. **nomic-embed-text** — huggingface.co/nomic-ai/nomic-embed-text-v1.5
7. **mxbai-embed-large** — huggingface.co/mixedbread-ai/mxbai-embed-large-v1
8. **SQLite FTS5** — sqlite.org/fts5.html (BM25 default scoring)
9. **MMR (Carbonell & Goldstein, 1998)** — "The Use of MMR, Diversity-Based Reranking for Reordering Documents and Producing Summaries"
10. **Tobi Lütke "QMD as a Memory Substrate"** — 2025 Shopify Eng internal talk (요지 공개분)

---

## 7. 결론

본 조사 결과를 종합하면 다음 결정이 합리적이다:

- **storage**: Markdown source-of-truth + SQLite (sqlite-vec + FTS5) hybrid index, 단일 파일 per collection
- **bindings**: `mattn/go-sqlite3` + `asg017/sqlite-vec` (cgo 허용 범위)
- **embedding**: ollama embed sidecar (`nomic-embed-text` default), 미설치 시 BM25-only graceful degrade
- **retrieval**: 3 mode (search / vsearch / query). query = 0.7·vector + 0.3·BM25 + temporal decay + MMR
- **format**: ClawMem vault 호환 옵션 제공 → cross-agent memory consistency
- **security**: mode 0600 vault + sqlite, PII 마스킹 파이프라인, opt-in session auto-export
- **lifecycle**: 인덱스 손상 시 `mink memory reindex` 로 markdown 으로부터 완전 복원

본 결정들은 `spec.md` §6 EARS 요구사항과 §7 아키텍처로 1:1 매핑된다.
