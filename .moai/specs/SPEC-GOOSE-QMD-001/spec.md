---
id: SPEC-GOOSE-QMD-001
version: 0.2.0
status: planned
created_at: 2026-04-24
updated_at: 2026-04-24
author: manager-spec
priority: P0
issue_number: null
phase: 1
milestone: M1
size: 대(L)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-QMD-001 — QMD Embedded Hybrid Memory Search

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-24 | 스켈레톤 초안 (아키텍처 재편 v0.2에서 발췌) | architecture-redesign-v0.2 |
| 0.2.0 | 2026-04-24 | M1 진입용 본문 작성: EARS 요구사항 17건, Acceptance Criteria 12건, CGO 빌드/MCP/모델 파이프라인 상세화 | manager-spec |

---

## 1. 개요 (Overview)

GOOSE 런타임은 **로컬 우선(local-first) 하이브리드 검색 엔진**을 통해 Plan Phase의 context retrieval, PAI identity 검색, skill 의미 검색, Task/Ritual 결과 조회를 수행한다. 본 SPEC은 Tobias Lütke가 공개한 [tobi/qmd](https://github.com/tobi/qmd)의 Rust 포트인 [qntx-labs/qmd](https://github.com/qntx-labs/qmd)를 **CGO staticlib로 링크**하여 `goosed` 단일 바이너리 안에 임베드한다.

수락 조건 통과 시 `goosed`는:

- `./.goose/` 하위의 memory / context / skills / tasks / rituals 마크다운을 자동 인덱싱하고,
- BM25 전문검색 → 벡터 검색 → LLM rerank의 3단계 하이브리드 파이프라인으로 top-k 결과를 반환하며,
- 외부 MCP 클라이언트에도 동일한 검색 기능을 stdio JSON-RPC 2.0 프로토콜로 노출한다.

---

## 2. 배경 (Background)

### 2.1 왜 QMD인가

Plan Phase는 Intent classification 직후 **컨텍스트 회수(context retrieval)** 단계에 진입한다(`.moai/design/goose-runtime-architecture-v0.2.md` §3). 여기서 PAI 11개 identity 파일, `./.goose/memory/MEMORY.md`, 기존 task `result.md`를 포괄적으로 훑어야 하는데, 단순 BM25만으로는 의미적 유사성이 무너지고(e.g. "배포 실패"와 "deployment failure"), 순수 벡터 검색만으로는 정확 일치 유실(e.g. task-id, 파일 경로)과 LLM 비용이 문제가 된다.

QMD는 이 두 한계를 **BM25 + dense vector + LLM rerank**의 3-stage pipeline으로 동시에 해결한다. 특히 GGUF 임베더/리랭커를 사용하므로 모든 처리가 로컬 CPU에서 완결되며, Rust 구현체는 메모리 안전성과 단일 바이너리 친화성을 제공한다.

### 2.2 Kuzu와의 역할 분담 (매우 중요)

GOOSE는 두 종류의 지식 표현을 병행한다. 혼동 방지를 위해 명시적으로 분리한다.

| 축 | QMD (본 SPEC, M1) | Kuzu (Phase 8+) |
|----|-------------------|-----------------|
| 대상 | 마크다운 문서 | 엔티티 관계 그래프 |
| 질의 | "이 주제와 비슷한 문서 찾기" | "A와 B는 어떤 관계인가" |
| 결과 | top-k 문서 청크 | 경로, 서브그래프 |
| 저장 | `./.goose/data/qmd-index/` | `./.goose/data/kuzu/` |
| 기술 | BM25 + embedding + rerank | property graph + Cypher |

두 시스템은 서로를 대체하지 않는다. QMD가 "관련 문서가 무엇인가"를 찾고, Kuzu가 "그 문서 안의 엔티티들이 어떻게 엮여 있는가"를 답한다.

### 2.3 상속 자산

- **`qntx-labs/qmd` (MIT, Rust)**: Tobias Lütke의 TypeScript 원본을 포팅. `llama.cpp` 바인딩으로 GGUF 추론, `tantivy` 기반 BM25, `arroy`/`usearch` 계열 ANN 인덱스 사용.
- **`tobi/qmd` (원본, TypeScript, MIT)**: 설계 레퍼런스. 본 SPEC은 TypeScript 포트를 사용하지 않는다(node-llama-cpp 런타임 의존성 회피).
- **MoAI-ADK 설계 원칙**: TRUST 5, SPEC-EARS, @MX tag 적용(빌드 타임만).

### 2.4 범위 경계

- **IN**: `./.goose/`의 마크다운 인덱싱, Go 측 공개 API (`qmd.Index`, `qmd.Query`, `qmd.Reindex`, `qmd.Watch`), MCP stdio 서버, GGUF 모델 자동 다운로드, fsnotify 기반 자동 재인덱스, CLI 서브커맨드(`goose qmd ...`), CGO staticlib 빌드 파이프라인.
- **OUT**: 엔티티 관계 그래프(→ Phase 8 Kuzu), LoRA 미세조정(→ Phase 8 IDENTITY), 분산 인덱스/샤딩, 원격 인덱스 서버, 비-마크다운(.pdf/.docx) 인덱싱(v0.2+).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. **Go 공개 API** (`internal/qmd/`): `qmd.Index(docs)`, `qmd.Query(q, k)`, `qmd.Reindex(path)`, `qmd.Watch(path)`.
2. **CGO staticlib 빌드 파이프라인**: Rust 크레이트(`qntx-labs/qmd`) → `.a`/`.lib` staticlib → Go `// #cgo LDFLAGS`로 링크.
3. **빌드 타깃**: macOS (Intel + ARM universal), Linux x86_64, Linux ARM64. Windows는 v0.2+ 유예.
4. **하이브리드 검색 파이프라인**: BM25(full-text) → dense vector(임베더) → LLM rerank의 3단계.
5. **GGUF 모델 관리**: `bge-small-en-v1.5.gguf`(임베더, ~120MB) + `bge-reranker-base.gguf`(리랭커, ~280MB)를 `./.goose/data/models/`에 자동 다운로드 + SHA256 검증 + 이어받기.
6. **인덱스 영속화**: `./.goose/data/qmd-index/`에 BM25 + vector 인덱스 저장, WAL 기반 크래시 복구.
7. **증분 재인덱스**: 파일 content hash 비교로 변경 감지, fsnotify watcher로 자동 트리거(debounce 500ms).
8. **MCP stdio 서버**: JSON-RPC 2.0 (LSP-style). 메서드: `qmd/query`, `qmd/reindex`, `qmd/stats`.
9. **CLI 서브커맨드**: `goose qmd reindex`, `goose qmd query <text>`, `goose qmd stats`, `goose qmd mcp`(stdio 서버 실행).
10. **보안 경계**: 인덱스 대상 경로는 `./.goose/config/security.yaml`의 `blocked_always`와 교차 검증하여 필터링.
11. **피처 플래그**: `./.goose/config/goose.yaml`에 `qmd.enabled`(기본 true) 제공.

### 3.2 OUT OF SCOPE (명시적 제외)

- Kuzu graph DB (Phase 8 SPEC).
- 모델 파인튜닝 / LoRA 어댑터 (별도 Phase 8 SPEC).
- 비-마크다운 문서 (PDF, DOCX, 소스 코드) — v0.2+.
- 분산/멀티 프로세스 인덱싱. 단일 `goosed` 프로세스 내부.
- 원격 API 기반 임베더/리랭커(OpenAI, Cohere 등). 로컬 GGUF 전용.
- 웹 UI에서의 검색 결과 시각화 — WEBUI-001의 책임.
- Windows 지원(v0.2+로 유예).
- 인덱스 암호화 / at-rest encryption (CREDENTIAL-PROXY-001의 zero-knowledge proxy와 중복 방지).
- TypeScript 원본(`tobi/qmd`) 포팅. Rust 포트만 사용.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-QMD-001 [Ubiquitous]** — The QMD subsystem **shall** be statically linked into `goosed` via CGO, producing a single binary with zero runtime dependencies on external executables, shared libraries, or language runtimes (excluding the two GGUF model files).

**REQ-QMD-002 [Ubiquitous]** — The QMD subsystem **shall** expose a Go API surface consisting of exactly four public functions: `Index(docs []Doc) error`, `Query(q string, k int) ([]Result, error)`, `Reindex(path string) error`, `Watch(path string) (stop func(), error)`.

**REQ-QMD-003 [Ubiquitous]** — The QMD subsystem **shall** persist both BM25 and vector indexes to `./.goose/data/qmd-index/` and survive `goosed` restart without requiring full reindexing.

**REQ-QMD-004 [Ubiquitous]** — The QMD subsystem **shall** record index metadata (last reindex timestamp, document count, index size) in the `qmd_index_status` SQLite table and per-document hashes in `qmd_doc_tracking` (see §4.1 of the architecture doc).

### 4.2 Event-Driven (이벤트 기반)

**REQ-QMD-005 [Event-Driven]** — **When** `goose qmd reindex [path]` is invoked, the system **shall** perform a full reindex of the given path (or all configured roots if omitted) and emit a progress log every 1000 documents processed.

**REQ-QMD-006 [Event-Driven]** — **When** a file under a watched path is created, modified, or deleted, the fsnotify watcher **shall** debounce the event for 500ms and then trigger an incremental reindex of the affected document only.

**REQ-QMD-007 [Event-Driven]** — **When** Agent Core invokes `qmd.Query(q, k)`, the system **shall** execute the 3-stage pipeline (BM25 candidate retrieval → vector re-scoring → LLM rerank) and return up to `k` results ordered by final relevance score, within 10ms p50 and 50ms p99 for an index of up to 10,000 documents.

**REQ-QMD-008 [Event-Driven]** — **When** an external MCP client connects via stdio and sends a JSON-RPC 2.0 request with method `qmd/query`, the server **shall** respond with the same result set that `qmd.Query` returns, plus `jsonrpc: "2.0"` and matching request `id`.

**REQ-QMD-009 [Event-Driven]** — **When** GGUF model files are missing on first invocation, the system **shall** download them from the configured mirror (default: HuggingFace), verify SHA256 checksums against pinned values, and store them at `./.goose/data/models/`. The download **shall** be resumable using HTTP Range requests after network interruption.

### 4.3 State-Driven (상태 기반)

**REQ-QMD-010 [State-Driven]** — **While** a reindex operation is in progress, concurrent `qmd.Query` calls **shall** continue to serve results from the previous snapshot without blocking (reader-writer separation).

**REQ-QMD-011 [State-Driven]** — **While** GGUF model files are being downloaded or verified, any `qmd.Query` call **shall** return `ErrModelNotReady` immediately instead of blocking.

**REQ-QMD-012 [State-Driven]** — **While** `./.goose/config/goose.yaml` has `qmd.enabled: false`, the QMD subsystem **shall not** start the watcher, download models, or accept MCP connections. `qmd.Query` **shall** return `ErrQMDDisabled`.

### 4.4 Unwanted Behavior (방지)

**REQ-QMD-013 [Unwanted]** — **If** a candidate path for indexing matches any pattern in `./.goose/config/security.yaml` `blocked_always` list, **then** the system **shall not** read, tokenize, embed, or index the file and **shall** log the decision to `./.goose/logs/audit.local.log` with reason `qmd_blocked_path`.

**REQ-QMD-014 [Unwanted]** — **If** index corruption is detected on startup (BM25 or vector index fails validation), **then** the system **shall** log an ERROR, rename the corrupt index to `qmd-index.corrupt.{timestamp}/`, and trigger a full reindex rather than serving stale or incorrect results.

**REQ-QMD-015 [Unwanted]** — **If** the combined in-memory index footprint (BM25 + vector) exceeds the configured `qmd.memory_limit_mb` (default 500MB), **then** the system **shall** switch the vector index from in-memory to memory-mapped (mmap) mode and emit a WARN log line.

**REQ-QMD-016 [Unwanted]** — The QMD subsystem **shall not** expose MCP stdio server on any TCP/UDP port. MCP communication is strictly via stdin/stdout of the child process spawned by `goose qmd mcp`.

### 4.5 Optional (선택적)

**REQ-QMD-017 [Optional]** — **Where** environment variable `QMD_MODEL_MIRROR` is defined, the download routine **shall** prefer that URL over the default HuggingFace mirror, falling back to the default on any non-200 response.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then 구조. `*_test.go`로 변환 가능한 수준.

**AC-QMD-001 — 단일 바이너리 + 런타임 의존성 제로**
- **Given** CI 빌드 머신에 Go 1.26 + Rust 1.80 toolchain 설치됨
- **When** `make build-goosed` 실행 후 산출물에 `otool -L goosed` (macOS) / `ldd goosed` (linux) 실행
- **Then** 외부 공유 라이브러리는 시스템 제공(`libSystem`, `libc`, `libpthread`, `libm`, `libdl`)만 표시되고 `qmd`, `tantivy`, `llama.cpp` 관련 항목은 표시되지 않음

**AC-QMD-002 — 10k 문서 인덱싱 성능**
- **Given** `./.goose/memory/`에 10,000개 마크다운 파일이 존재(평균 512바이트)
- **When** `goose qmd reindex` 실행
- **Then** M-series Mac(M2 Pro 이상) 기준 30초 이내에 인덱싱 완료, `qmd_index_status.docs_indexed = 10000`

**AC-QMD-003 — 쿼리 latency p50 < 10ms, p99 < 50ms**
- **Given** 10k 문서 인덱스가 준비되어 있음
- **When** 무작위 영어 자연어 쿼리 1,000개를 순차 실행
- **Then** p50 응답 시간 < 10ms, p99 < 50ms (벤치마크 테스트 `BenchmarkQMDQuery10k`로 증빙)

**AC-QMD-004 — 증분 재인덱스 < 100ms**
- **Given** 10k 인덱스가 준비되어 있고 fsnotify watcher 활성
- **When** 단일 마크다운 파일을 수정
- **Then** debounce 500ms 후 해당 문서만 재인덱싱, 증분 작업 자체는 100ms 이내 완료

**AC-QMD-005 — 3-stage hybrid pipeline 동작 검증**
- **Given** 테스트 코퍼스에 "정확 일치"용 토큰과 "의미 유사"용 문서가 각각 존재
- **When** 쿼리 실행
- **Then** 결과에 BM25 top-k 후보가 포함되고, 벡터 유사도 재정렬이 적용되며, LLM reranker가 최종 순위를 조정했음을 `trace` 플래그 출력으로 확인

**AC-QMD-006 — MCP stdio 프로토콜 적합성**
- **Given** `goose qmd mcp` 를 별도 프로세스로 실행
- **When** 테스트 클라이언트가 `{"jsonrpc":"2.0","id":1,"method":"qmd/query","params":{"q":"hello","k":5}}` 를 stdin에 write
- **Then** stdout에서 `{"jsonrpc":"2.0","id":1,"result":[...]}` 형태의 응답을 수신, JSON Schema 검증 통과

**AC-QMD-007 — MCP 동시 50 클라이언트**
- **Given** MCP 서버 1개 인스턴스
- **When** 50개의 요청을 병렬 stdin multiplex으로 전송
- **Then** 모든 요청이 정확한 `id`로 응답 매칭되어 반환, 누락/중복 없음

**AC-QMD-008 — 모델 다운로드 resumable + SHA256 검증**
- **Given** `./.goose/data/models/bge-small-en-v1.5.gguf` 의 70%가 다운로드된 중단 상태
- **When** `goose qmd reindex` 재실행
- **Then** HTTP Range로 이어받기, 완료 후 SHA256 checksum이 pinned 값과 일치, 불일치 시 파일 삭제 후 재시도(최대 3회)

**AC-QMD-009 — 인덱싱 중 SIGKILL 복구**
- **Given** `goose qmd reindex` 실행 중 (진행률 50%)
- **When** `kill -9 goosed` 로 강제 종료 후 재시작
- **Then** 기존 인덱스 파일이 손상되지 않고 읽기 가능, 미완료된 문서는 content hash 기반으로 자동 재인덱싱

**AC-QMD-010 — 메모리 제한 준수**
- **Given** 10k 문서 인덱스
- **When** `goosed` 프로세스를 정상 부팅 후 10분간 쿼리 부하 인가
- **Then** RSS 메모리 사용량이 500MB 미만 유지 (벤치마크 `TestQMDMemoryBounds`로 검증)

**AC-QMD-011 — `blocked_always` 경로 미인덱싱**
- **Given** `./.goose/config/security.yaml`의 `blocked_always`에 `~/.ssh/**` 포함, 테스트용 심볼릭 링크 `./.goose/test-link` → `~/.ssh/`
- **When** `goose qmd reindex ./.goose/test-link` 실행
- **Then** `~/.ssh/` 하위 파일은 1개도 인덱스에 포함되지 않음, `audit.local.log`에 `qmd_blocked_path` 이벤트 기록

**AC-QMD-012 — 피처 플래그 OFF**
- **Given** `./.goose/config/goose.yaml`에 `qmd.enabled: false` 설정
- **When** `goosed` 부팅
- **Then** watcher 미시작, 모델 미다운로드, `qmd.Query()` 호출 시 `ErrQMDDisabled` 반환

---

## 6. 기술 스택

| 구분 | 항목 | 버전 / 출처 |
|-----|------|-----------|
| Rust 크레이트 | `qntx-labs/qmd` | pinned tag(첫 통합 시 결정), MIT |
| Rust toolchain | `rustc`, `cargo` | 1.80+ (빌드 타임 전용, 런타임 불필요) |
| CGO linker | Go `cgo` + `// #cgo LDFLAGS` | Go 1.26 내장 |
| 임베더 모델 | `bge-small-en-v1.5.gguf` | HuggingFace `qntx-labs/bge-small-en-v1.5-gguf` (~120MB) |
| 리랭커 모델 | `bge-reranker-base.gguf` | HuggingFace `qntx-labs/bge-reranker-base-gguf` (~280MB) |
| 파일 워처 | `github.com/fsnotify/fsnotify` | v1.7+ (CORE-001 의존성과 공유) |
| 다운로더 | stdlib `net/http` + 자체 resume 로직 | — |
| MCP 프레임 | stdlib `encoding/json` (LSP-style framing) | — |
| SQLite 접근 | `modernc.org/sqlite` (CORE-001 확정) | — |

---

## 7. 기술적 접근 (Technical Approach)

### 7.1 패키지 레이아웃

```
/ (repo root)
├── cmd/goose/main.go              # CLI 엔트리, `qmd` 서브커맨드 라우팅
├── cmd/goosed/main.go             # 데몬 엔트리 (CORE-001 확장)
├── internal/qmd/
│   ├── api.go                     # 공개 Go API (Index/Query/Reindex/Watch)
│   ├── cgo/
│   │   ├── bridge.go              # cgo 경계 (// #cgo LDFLAGS)
│   │   ├── bridge.h               # C ABI 선언
│   │   └── qmd_shim.c             # Rust staticlib와의 C shim
│   ├── index/
│   │   ├── bm25.go                # BM25 인덱스 래퍼
│   │   ├── vector.go              # 벡터 인덱스 래퍼
│   │   └── snapshot.go            # 읽기 스냅샷 관리
│   ├── models/
│   │   ├── manifest.go            # 모델 매니페스트 (SHA256, 크기, URL)
│   │   ├── download.go            # resumable 다운로더
│   │   └── verify.go              # SHA256 검증
│   ├── mcp/
│   │   ├── server.go              # stdio JSON-RPC 2.0 서버
│   │   ├── framing.go             # Content-Length 프레이밍 (LSP 호환)
│   │   └── methods.go             # query / reindex / stats 구현
│   ├── watcher/
│   │   └── fsnotify.go            # 파일 변경 감지 + debounce
│   └── security/
│       └── blocked.go             # security.yaml blocked_always 매칭
├── internal/qmd/cgo/rust/         # Rust 크레이트 vendored source
│   ├── Cargo.toml
│   └── build.rs                   # staticlib 산출물 생성
└── build/
    ├── build-qmd-darwin-universal.sh
    ├── build-qmd-linux-amd64.sh
    └── build-qmd-linux-arm64.sh
```

### 7.2 공개 Go API (초안)

```go
// internal/qmd/api.go
package qmd

type Doc struct {
    Path    string
    Content string
    Hash    string   // content_hash (SHA256 of Content)
    Meta    map[string]string
}

type Result struct {
    Path    string
    Snippet string
    Score   float64  // final score after rerank
    Debug   map[string]float64 // BM25 / vector / rerank sub-scores
}

func Index(docs []Doc) error
func Query(q string, k int) ([]Result, error)
func Reindex(root string) error                 // full reindex under root
func Watch(root string) (stop func(), err error) // start fsnotify watcher
```

### 7.3 CGO 빌드 파이프라인

```
┌─────────────────────────────┐
│ Rust source (qntx-labs/qmd) │
│ internal/qmd/cgo/rust/      │
└──────────────┬──────────────┘
               │ cargo build --release --target=<triple> --staticlib
               ▼
┌─────────────────────────────┐
│ libqmd.a (per-triple)       │
│ build/lib/<triple>/libqmd.a │
└──────────────┬──────────────┘
               │ // #cgo LDFLAGS: -L${SRCDIR}/../../build/lib/<triple> -lqmd
               ▼
┌─────────────────────────────┐
│ internal/qmd/cgo/bridge.go  │
│   func cIndex(...)          │
└──────────────┬──────────────┘
               │ go build -tags=qmd
               ▼
┌─────────────────────────────┐
│ goosed (단일 바이너리)       │
└─────────────────────────────┘
```

교차 컴파일 전략:
- **macOS universal binary**: `lipo`로 arm64 + x86_64 `libqmd.a` 병합 후 단일 링크.
- **Linux x86_64 → ARM64**: Docker `buildx`로 멀티 아키 빌드, 각 triple별 staticlib 생성 후 CI에서 교차 링크.
- **CGO 격리**: CGO는 오로지 `internal/qmd` 서브패키지에서만 사용. 그 외 코드베이스(CORE-001 포함)는 `CGO_ENABLED=0` 유지.

### 7.4 메모리 관리

- **소유권**: Rust가 BM25/vector 인덱스 메모리의 소유자. Go 측은 opaque pointer만 보유.
- **수명주기**: `qmd.Open()` → Go 컨텍스트 객체, Go GC 파이널라이저에서 `qmd_close()` 호출.
- **결과 전달**: Rust가 할당한 문자열/슬라이스는 전용 `qmd_free_result()` 함수로 명시적 해제. Go 측 `defer C.qmd_free_result(r)` 패턴.
- **panic 차단**: Rust panic → `catch_unwind` → C 에러 코드 → Go `error` 래핑. panic이 Go 스택으로 전파되지 않도록 FFI 경계에서 완벽 격리.

### 7.5 인덱싱 파이프라인

```
1. scan(root)
   ├─ glob(**/*.md)
   ├─ filter by security.yaml blocked_always
   └─ 각 파일: read + SHA256 hash

2. diff
   ├─ qmd_doc_tracking 테이블과 hash 비교
   ├─ 변경: upsert
   ├─ 미변경: skip
   └─ 삭제: index remove

3. chunk
   ├─ 섹션 기반(H1/H2) 분할
   ├─ 최대 512 토큰 / 최소 64 토큰
   └─ overlap 64 토큰

4. index (Rust 측 CGO 호출)
   ├─ BM25: tantivy에 문서 추가
   ├─ vector: GGUF 임베더로 dense vector 생성 → arroy/usearch 추가
   └─ 원자적 commit (WAL)

5. update status
   └─ qmd_index_status.last_full_reindex_at, docs_indexed
```

### 7.6 MCP stdio 프로토콜

LSP 스타일 Content-Length 프레이밍을 채택한다(별도 파서 구현 불필요).

```
Content-Length: 128\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"qmd/query","params":{"q":"deployment failure","k":5}}
```

메서드:

| 메서드 | Params | Result |
|-------|--------|--------|
| `qmd/query` | `{q: string, k: int, options?: {trace: bool}}` | `{results: Result[]}` |
| `qmd/reindex` | `{path?: string}` | `{docs_indexed: int, duration_ms: int}` |
| `qmd/stats` | `{}` | `{docs: int, index_size_bytes: int, last_reindex_at: string}` |
| `qmd/shutdown` | `{}` | `{ok: true}` |

에러 코드(JSON-RPC 2.0 관례 + QMD 확장):

| 코드 | 의미 |
|-----|------|
| `-32601` | Method not found |
| `-32602` | Invalid params |
| `-32700` | Parse error |
| `-32001` | ErrModelNotReady |
| `-32002` | ErrQMDDisabled |
| `-32003` | ErrIndexCorrupt |

### 7.7 모델 다운로드 + 검증

`internal/qmd/models/manifest.json` (임베드):

```json
{
  "version": "2026-04-24",
  "models": {
    "embedder": {
      "name": "bge-small-en-v1.5",
      "filename": "bge-small-en-v1.5.gguf",
      "size_bytes": 125829120,
      "sha256": "<pinned-hash>",
      "urls": [
        "https://huggingface.co/qntx-labs/bge-small-en-v1.5-gguf/resolve/main/bge-small-en-v1.5.gguf"
      ]
    },
    "reranker": {
      "name": "bge-reranker-base",
      "filename": "bge-reranker-base.gguf",
      "size_bytes": 293601280,
      "sha256": "<pinned-hash>",
      "urls": [
        "https://huggingface.co/qntx-labs/bge-reranker-base-gguf/resolve/main/bge-reranker-base.gguf"
      ]
    }
  }
}
```

절차:
1. 파일 존재 + size + SHA256 일치 → skip
2. 부분 파일 존재 → HTTP Range로 이어받기
3. 전체 다운로드 → SHA256 검증
4. 불일치 → 파일 삭제 후 재시도(최대 3회, 지수 백오프)
5. `QMD_MODEL_MIRROR` env 우선, 실패 시 manifest urls 순차 시도

### 7.8 동시성 / 락

| 작업 | 락 유형 | 비고 |
|-----|--------|------|
| Query (read) | RLock | 스냅샷 기반, writer와 병행 가능 |
| Incremental reindex (write) | Lock | 단일 writer 직렬화 |
| Full reindex | Lock + swap | 새 인덱스를 임시 디렉토리에 구축 → atomic rename |
| MCP handler | goroutine per request | goroutine pool 없음, 50 동시 요청 요구치 여유 |

### 7.9 CLI 서브커맨드

```
goose qmd reindex [path]       # 전체 또는 특정 경로 재인덱스
goose qmd query <text> [-k N]  # 검색 실행 (debug용)
goose qmd stats                # 인덱스 통계 출력
goose qmd mcp                  # stdio MCP 서버 실행 (외부 agent 연결용)
goose qmd models verify        # 모델 SHA256 검증
goose qmd models download      # 모델 강제 재다운로드
```

### 7.10 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | Go 래퍼 단위 테스트 + 실제 staticlib 통합 테스트 + 10k 벤치마크 + SIGKILL 복구 테스트. 85%+ coverage. |
| **R**eadable | 공개 API 4개로 국한, 각 함수 단일 책임. cgo 경계는 bridge.go에 격리. |
| **U**nified | `go fmt` + `golangci-lint` + `rustfmt` + `clippy`. CI에서 모든 lint 통과 필수. |
| **S**ecured | `blocked_always` 교차 검증(REQ-QMD-013), Rust 크레이트 메모리 안전성, FFI 경계 panic 격리, SHA256 모델 무결성. |
| **T**rackable | @MX:ANCHOR를 공개 API 4개 함수에 부여(fan_in 기대 >=3), @MX:WARN을 cgo bridge에 부여, 모든 인덱싱/다운로드 결정은 `audit.local.log` 기록. |

---

## 8. 마이그레이션 & 롤아웃

### 8.1 Phase 1 (M1) — Read-only 통합
- Go API `Index`/`Query`/`Reindex` 제공
- `goose qmd reindex` 수동 명령만 지원
- Agent Core는 Plan Phase에서 `qmd.Query` 호출
- fsnotify watcher **비활성**

### 8.2 Phase 2 (M3)
- `qmd.Watch` 활성화, 파일 변경 자동 증분 재인덱스
- Debounce 500ms, 동시성 테스트 통과

### 8.3 Phase 3 (M4)
- MCP stdio 서버 외부 공개 (`goose qmd mcp`)
- 외부 agent (Claude Code, Cursor 등)에서 검색 결과 참조 가능

### 8.4 피처 플래그

```yaml
# ./.goose/config/goose.yaml
qmd:
  enabled: true              # 기본 true, false로 전체 비활성
  memory_limit_mb: 500
  watch_debounce_ms: 500
  index_roots:
    - "./.goose/memory"
    - "./.goose/context"
    - "./.goose/skills"
    - "./.goose/tasks"
    - "./.goose/rituals"
  models:
    embedder: "bge-small-en-v1.5"
    reranker: "bge-reranker-base"
    mirror_env: "QMD_MODEL_MIRROR"
```

---

## 9. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | runtime, logger, health, exit code 계약 |
| 선행 SPEC | SPEC-GOOSE-FS-ACCESS-001 | `security.yaml` 경로 매트릭스 체크 |
| 동반 SPEC | SPEC-GOOSE-CREDPOOL-001 | provider credential proxy (모델 다운로드 시 keyring 사용 가능성) |
| 외부 | `qntx-labs/qmd` Rust crate | 상류 MIT, pinned version |
| 외부 | Rust 1.80+ toolchain | 빌드 타임 전용 |
| 외부 | HuggingFace CDN | GGUF 모델 다운로드 |
| 외부 | `github.com/fsnotify/fsnotify` | 파일 워처 |
| 내부 | `internal/db` (CORE-001) | `qmd_index_status`, `qmd_doc_tracking`, `fs_access_log` 테이블 |

---

## 10. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Rust toolchain이 사용자 개발 환경에 없음 | 중 | 중 | CI에서 사전 빌드된 바이너리 릴리스 제공. 사용자는 `goose` 배포 바이너리를 받으면 Rust 없이 사용 가능. 소스 빌드는 `make build-from-source` 로 안내. |
| R2 | CGO가 크로스 컴파일을 어렵게 함 | 중 | 상 | CGO는 `internal/qmd` 서브패키지에만 국한. 그 외 전체 코드베이스는 `CGO_ENABLED=0`. Docker buildx로 triple별 staticlib 선(先)빌드. |
| R3 | GGUF 모델 다운로드(400MB+) 가 느린 네트워크에서 실패 | 중 | 중 | HTTP Range resume + SHA256 검증 + mirror fallback + 최대 3회 재시도 (REQ-QMD-009, AC-QMD-008). |
| R4 | 인덱스 파일 손상 (SIGKILL, 디스크 풀) | 중 | 중 | WAL(Write-Ahead Log) + 기동 시 무결성 검증. 손상 감지 → 자동 리인덱스 (REQ-QMD-014). |
| R5 | Rust crate 상류 breaking change | 낮 | 중 | pinned version + vendored source. 업그레이드 시 통합 테스트 스위트 필수 통과(SPEC-LSP-CORE-002 업그레이드 정책 참고). |
| R6 | 10k 문서 쿼리 p99 < 50ms 목표 미달 | 중 | 중 | 벤치마크 테스트 `BenchmarkQMDQuery10k` CI 상시 실행. 초과 시 LLM rerank를 top-20 후보에만 적용하도록 단계 조정. |
| R7 | RSS 메모리 500MB 초과 | 중 | 중 | `qmd.memory_limit_mb` 설정 + 벡터 인덱스 mmap 전환(REQ-QMD-015). |
| R8 | 모델 파일을 실수로 git 저장소에 커밋 | 낮 | 낮 | `.gitignore` 최상단에 `**/data/models/*.gguf` + `**/data/qmd-index/` 추가. |
| R9 | FFI 경계에서 Rust panic이 Go 스택으로 전파 | 낮 | 상 | Rust 측 `catch_unwind` 필수. CI에서 의도적 panic 테스트로 검증. |
| R10 | MCP stdio 서버의 JSON-RPC 파싱이 LSP 프레이밍을 깨뜨림 | 낮 | 중 | Content-Length 프레이밍은 표준 LSP 구현과 동일. `gopls` 테스트 벡터 재사용. |
| R11 | 인덱스가 `blocked_always` 파일을 실수로 포함 | 낮 | 상 | REQ-QMD-013 + AC-QMD-011. 심볼릭 링크 해결(`filepath.EvalSymlinks`) 후 재검증. |

---

## 11. 테스트 계획 (Test Plan)

### 11.1 단위 테스트 (Go 측)

- `internal/qmd/api_test.go`: `Index`/`Query`/`Reindex`/`Watch` 시그니처 검증, 모킹 레벨
- `internal/qmd/cgo/bridge_test.go`: C ABI 바인딩 smoke 테스트 (실제 staticlib 필요)
- `internal/qmd/mcp/framing_test.go`: Content-Length 파싱, 부분 프레임 처리, 잘못된 프레임 거부
- `internal/qmd/mcp/methods_test.go`: 각 RPC 메서드 happy-path + error-path
- `internal/qmd/models/download_test.go`: Range resume, SHA256 불일치 시 재시도
- `internal/qmd/security/blocked_test.go`: glob 패턴 매칭, symlink 이스케이프

### 11.2 통합 테스트 (Rust staticlib 필요)

- `test/integration/qmd_indexing_test.go`: 100 마크다운 픽스처 → 인덱싱 → 쿼리 결과 확인
- `test/integration/qmd_mcp_test.go`: 자식 프로세스로 `goose qmd mcp` 실행 → stdin/stdout 왕복
- `test/integration/qmd_watcher_test.go`: fsnotify + debounce + 증분 재인덱스

### 11.3 성능 / 스트레스 테스트

- `BenchmarkQMDQuery10k`: 10k 문서 + 1k 쿼리, p50/p95/p99 측정
- `BenchmarkQMDReindex10k`: 전체 재인덱스 시간 측정 (목표 <30초 on M2 Pro)
- `BenchmarkQMDIncremental`: 1 파일 변경 시 <100ms 검증
- `TestQMDMemoryBounds`: 10k 인덱스 + 10분 부하 → RSS < 500MB
- `TestQMDConcurrent50MCP`: 50 동시 MCP 요청 → 모두 정상 응답

### 11.4 복구 / 보안 테스트

- `TestQMDRecoverAfterSIGKILL`: 인덱싱 중간 SIGKILL → 재시작 → 인덱스 정상
- `TestQMDIndexCorruptionRecovery`: 인덱스 파일 손상 주입 → 자동 리인덱스
- `TestQMDBlockedPathNotIndexed`: `blocked_always` 하위 파일이 인덱스에 없음
- `TestQMDSymlinkEscape`: symlink로 blocked path 참조 시도 → 차단

### 11.5 CI 행렬

| OS / Arch | 빌드 | 통합 테스트 | 벤치마크 |
|----------|-----|-----------|---------|
| macOS arm64 (M-series) | ✅ | ✅ | ✅ (기준 플랫폼) |
| macOS x86_64 | ✅ | ✅ | — |
| Linux x86_64 | ✅ | ✅ | ✅ |
| Linux arm64 | ✅ | ✅ | — |
| Windows | ❌ (v0.2+) | — | — |

---

## 12. 참고 (References)

### 12.1 프로젝트 문서

- `.moai/design/goose-runtime-architecture-v0.2.md` §8 (QMD Memory Search Integration), §4.1 (qmd_index_status, qmd_doc_tracking), §5 Tier 2 (FS access matrix)
- `.moai/specs/SPEC-GOOSE-CORE-001/spec.md` (선행 런타임 계약)
- `.moai/specs/SPEC-GOOSE-FS-ACCESS-001/spec.md` (경로 매트릭스)
- `.claude/rules/moai/core/lsp-client.md` (외부 Rust crate 상류 버전 고정 정책 레퍼런스)

### 12.2 외부 참조

- **qntx-labs/qmd** (Rust, MIT): https://github.com/qntx-labs/qmd
- **tobi/qmd** (TypeScript, MIT, 원본): https://github.com/tobi/qmd
- **BGE 임베더**: https://huggingface.co/BAAI/bge-small-en-v1.5
- **BGE 리랭커**: https://huggingface.co/BAAI/bge-reranker-base
- **Tantivy** (BM25 엔진): https://github.com/quickwit-oss/tantivy
- **llama.cpp GGUF** (로컬 추론): https://github.com/ggerganov/llama.cpp
- **JSON-RPC 2.0**: https://www.jsonrpc.org/specification
- **LSP Content-Length framing**: https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#baseProtocol

### 12.3 부속 문서

없음. (본 SPEC은 별도 `research.md`를 가지지 않는다. 아키텍처 문서 §8이 연구 노트 역할을 한다.)

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **엔티티 관계 그래프를 구축하지 않는다**. Kuzu는 Phase 8 별도 SPEC.
- 본 SPEC은 **모델 파인튜닝/LoRA**를 포함하지 않는다. 사전 배포된 GGUF 모델만 사용.
- 본 SPEC은 **비-마크다운 문서**(.pdf, .docx, 소스 코드 파일)를 인덱싱하지 않는다. v0.2+ 유예.
- 본 SPEC은 **원격 임베더/리랭커 API**(OpenAI Embeddings, Cohere Rerank)를 호출하지 않는다. 로컬 GGUF 전용.
- 본 SPEC은 **분산 인덱스 / 샤딩 / 멀티 프로세스 공유 인덱스**를 구현하지 않는다. 단일 `goosed` 프로세스 내부에 한정.
- 본 SPEC은 **Windows를 지원하지 않는다**. v0.2+ 유예. 본 Milestone은 macOS(Intel+ARM) + Linux(amd64+arm64)만 타깃.
- 본 SPEC은 **인덱스 암호화 / at-rest encryption**을 제공하지 않는다. Zero-knowledge credential proxy(CREDENTIAL-PROXY-001)와 중복 방지.
- 본 SPEC은 **TypeScript 포트** (`tobi/qmd`) 를 사용하지 않는다. Rust 포트(`qntx-labs/qmd`)만 통합.
- 본 SPEC은 **웹 UI 검색 결과 시각화**를 포함하지 않는다. WEBUI-001의 책임.
- 본 SPEC은 **TCP/UDP 네트워크 노출**을 하지 않는다. MCP 서버는 stdio 전용 (REQ-QMD-016).
- 본 SPEC은 **기존 인덱스 포맷의 마이그레이션**을 책임지지 않는다. 첫 도입이므로 `qmd_index_status.version = 1`만 지원, 이후 버전 업 시 별도 SPEC 추가.

---

**End of SPEC-GOOSE-QMD-001**
