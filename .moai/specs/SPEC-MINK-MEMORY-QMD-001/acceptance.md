# Acceptance Criteria — SPEC-MINK-MEMORY-QMD-001

본 문서는 spec.md §6 의 38개 EARS REQ 를 1:1 acceptance criteria 로 풀이한다. 각 AC 는 검증 방법(test 종류, 메트릭) 을 명시한다.

라이선스: AGPL-3.0-only.

---

## Conventions

- ID 형식: `AC-MEM-NNN`
- 매핑: AC ↔ REQ ↔ Milestone
- 검증 방법: `unit` (Go test) / `integration` (multi-package test) / `e2e` (CLI invocation test) / `bench` (latency benchmark) / `manual` (사용자 검증)
- Given-When-Then 또는 Given-Then 형식

---

## Section A — Ubiquitous (13개)

### AC-MEM-001 [REQ-MEM-001 / M1 / P0]

**Given** MINK 의 메모리 서브시스템이 초기화된다
**Then** `~/.mink/memory/markdown/{collection}/*.md` 가 단일 source of truth 로 동작하며, 본 디렉토리 외부의 markdown 은 인덱스 대상이 아니다
**Verify**: unit + e2e — `mink memory add --source /tmp/foo.md` 호출 시 `~/.mink/memory/markdown/custom/foo.md` 로 복사되었는지 확인 + 인덱스 대상 경로 검증

### AC-MEM-002 [REQ-MEM-002 / M1 / P0]

**Given** 메모리 서브시스템이 첫 초기화된다
**Then** `~/.mink/memory/{agent}.sqlite` 파일이 file mode 0600 으로 생성된다
**Verify**: unit — `os.Stat(...).Mode().Perm() == 0600` 단언

### AC-MEM-003 [REQ-MEM-003 / M1 / P0]

**Given** 메모리 서브시스템이 첫 초기화된다
**Then** `~/.mink/memory/` 부모 디렉토리가 mode 0700 으로 생성된다
**Verify**: unit

### AC-MEM-004 [REQ-MEM-004 / M1 / P0]

**Given** vault 에 markdown 파일이 추가된다
**Then** 해당 파일은 mode 0600 으로 저장된다
**Verify**: unit + integration — `mink memory add` 후 `os.Stat` 단언

### AC-MEM-005 [REQ-MEM-005 / M1 / P0]

**Given** 동일 source markdown 의 chunk
**When** model_version 이 변경된다
**Then** chunk_id 도 변경되어, 기존 chunk 가 stale 로 식별 가능하다
**Verify**: unit — `ChunkID(p, s, e, h, "v1") != ChunkID(p, s, e, h, "v2")`

### AC-MEM-006 [REQ-MEM-006 / M1 / P0]

**Given** 메모리 서브시스템이 첫 초기화된다
**Then** 6 default collection 디렉토리가 생성된다: `sessions/`, `journal/`, `briefing/`, `ritual/`, `weather/`, `custom/`
**Verify**: integration — `os.ReadDir` 단언

### AC-MEM-007 [REQ-MEM-007 / M1+M2+M3+M4 / P0]

**Given** 메모리 서브시스템
**Then** 3 retrieval mode 가 모두 지원된다: `search`, `vsearch`, `query`
**Verify**: e2e — 각 mode CLI flag 호출 + 정상 응답

### AC-MEM-008 [REQ-MEM-008 / M4 / P1]

**Given** 사용자가 `mink memory search "..."` 를 mode flag 없이 호출한다
**Then** 기본 mode 는 `query` (hybrid) 가 적용된다
**Verify**: e2e — flag 미지정 시 hybrid 경로 호출 trace

### AC-MEM-009 [REQ-MEM-009 / M4 / P1]

**Given** hybrid score 계산
**Then** spec.md REQ-MEM-009 의 additive 식 `α·cosine(emb(q), emb(c)) + β·bm25_norm(q, c) + γ·temporal_factor(c.created_at)` 가 적용되며 기본값은 α=0.7, β=0.3, γ=0.0 (decay 비활성), temporal_factor 의 half-life=30days (사용자가 γ>0 으로 명시 활성 시)
**Verify**: unit — score 계산 산식 단언 (synthetic 입력)

### AC-MEM-010 [REQ-MEM-010 / M4 / P1]

**Given** hybrid 검색 결과 후보가 30개 이상
**Then** MMR re-ranking (λ=0.7) 후 top-k 가 반환되며, top-10 결과 중 동일 source_path 비율은 30% 이하
**Verify**: unit + integration — MMR diversity metric 검증

### AC-MEM-011 [REQ-MEM-011 / M1 / P0]

**Given** 본 SPEC 의 코드
**Then** Go 단일 코드베이스로 구현되며, cgo 사용은 sqlite-vec + mattn/go-sqlite3 라이브러리 한정이다
**Verify**: manual — go.mod 검사 + cgo import 그래프 검증

### AC-MEM-012 [REQ-MEM-012 / M1 / P0]

**Given** chunking
**Then** heading boundary → paragraph boundary → 512-token cap 순으로 분할되며 prev/next pointer 가 부여된다
**Verify**: unit — 8개 markdown fixture 에 대해 expected chunks 비교

### AC-MEM-013 [REQ-MEM-013 / M1+M3 / P1]

**Given** vault 의 markdown 파일
**Then** embedding 벡터는 SQLite 안에만 저장되며 markdown 안에는 절대 포함되지 않는다
**Verify**: integration — vault 안의 모든 .md 파일 grep으로 base64/벡터 패턴 부재 검증

---

## Section B — Event-Driven (8개)

### AC-MEM-014 [REQ-MEM-014 / M1 / P0]

**Given** 사용자
**When** `mink memory add --collection journal --source /tmp/note.md` 를 호출한다
**Then** `~/.mink/memory/markdown/journal/note.md` 가 hardlink/copy 로 추가되고 chunks 테이블에 N rows 가 insert 된다 (N = ChunkMarkdown 결과)
**Verify**: e2e + integration

### AC-MEM-015 [REQ-MEM-015 / M2+M3+M4 / P0]

**Given** vault 에 N 개 markdown 이 색인되어 있다
**When** 사용자가 `mink memory search "keyword"` 를 호출한다
**Then** top-k matches 가 chunk_id, source_path, line range, score, 256-char snippet 와 함께 반환된다
**Verify**: e2e — 출력 JSON schema 단언

### AC-MEM-016 [REQ-MEM-016 / M5 / P1]

**Given** chunks 의 일부가 stale model_version 을 가진다
**When** 사용자가 `mink memory reindex` 를 호출한다
**Then** stale chunks 가 drop 되고 (a) 영향 source file 이 re-chunk 되며 (b) embedding backend 가 available 한 경우 re-embed 된다
**Verify**: integration — model_version 변경 시나리오 + reindex 후 stale 0 확인

### AC-MEM-017 [REQ-MEM-017 / M5 / P1]

**Given** JOURNAL/BRIEFING/WEATHER/RITUAL 가 artifact 를 publish 한다
**When** publish event 가 발생한다
**Then** memory 서브시스템이 hook 을 받아 해당 collection 으로 markdown 을 ingest 하며, publish event 발생 1초 이내에 인덱스에 반영된다
**Verify**: integration — 4 subsystem 각각 publish → memory 인덱스 timestamp 단언

### AC-MEM-018 [REQ-MEM-018 / M5 / P2]

**Given** `memory.session_autoexport.enabled = true`
**When** LLM-ROUTING-V2 router 가 session 을 완료한다
**Then** sanitized session transcript 가 `sessions/{YYYY-MM-DD}/{provider}-{session_id}.md` 로 export 되고 색인된다
**Verify**: integration — PII 마스킹 적용 + 파일 생성 단언

### AC-MEM-019 [REQ-MEM-019 / M3 / P1]

**Given** ollama 임베딩 endpoint 가 응답하지 않는다
**When** 사용자가 `mink memory search --mode query "..."` 를 호출한다
**Then** stderr 에 경고가 출력되고 BM25-only fallback 결과가 반환되며 명령은 exit code 0 으로 종료된다
**Verify**: e2e — ollama 미설치 / 임의 endpoint mock 으로 5xx 응답

### AC-MEM-020 [REQ-MEM-020 / M5 / P2]

**Given** vault 가 N 개 markdown + sqlite index 를 보유한다
**When** 사용자가 `mink memory export --format clawmem --dest /tmp/out` 를 호출한다
**Then** `/tmp/out` 에 ClawMem 호환 vault 가 생성된다
**Verify**: manual + integration — ClawMem MCP server mount 검증

### AC-MEM-021 [REQ-MEM-021 / M5 / P1]

**Given** vault 에 6개월 이전 markdown 파일 30개가 있다
**When** 사용자가 `mink memory prune --before 2026-01-01` 을 호출한다
**Then** 해당 markdown 파일 및 chunks 가 동일 transaction 으로 삭제된다
**Verify**: integration — markdown 삭제 + chunks count delta 단언

---

## Section C — State-Driven (6개)

### AC-MEM-022 [REQ-MEM-022 / M5 / P1]

**Given** `mink memory reindex` 가 background 에서 실행 중이다
**While** reindex 진행 중
**Then** read-only `search` / `query` 호출이 정상 응답한다 (read lock 차단 없음)
**Verify**: integration — concurrent goroutine: 1개 reindex + 5개 search

### AC-MEM-023 [REQ-MEM-023 / M3 / P1]

**Given** ollama endpoint 가 unreachable 이다
**While** ollama 가 down
**Then** `mink memory add` 가 정상 동작하며 새 chunks 가 `embedding_pending = 1` 로 마크된다
**Verify**: integration — ollama mock 5xx + chunks WHERE embedding_pending=1 count 단언

### AC-MEM-024 [REQ-MEM-024 / M5 / P2]

**Given** `memory.clawmem_compat.enabled = true`
**While** mirror 모드 활성
**Then** 모든 markdown write 가 `~/.mink/memory/markdown/` 와 `~/.clawmem/vault/` 양쪽에 미러되고, content hash 가 동일하다
**Verify**: integration — diff 후 hash 비교

### AC-MEM-025 [REQ-MEM-025 / M1 / P0]

**Given** 두 CLI invocation 이 동시에 `mink memory add` 를 호출한다
**While** 첫 invocation 의 write transaction 이 open
**Then** 두 번째 invocation 은 single-writer mutex 로 serialize 되어 정상 완료한다 (race 없음)
**Verify**: integration — race detector (`go test -race`) + 10회 concurrent invocation

### AC-MEM-026 [REQ-MEM-026 / M5 / P1]

**Given** vault 의 markdown 파일 수와 chunks 의 file_id count 가 불일치한다
**While** 검사가 실행된다
**Then** stderr 또는 log 에 warning 이 1회 출력되고 `mink memory reindex --collection {c}` 권장 메시지가 표시된다
**Verify**: integration — 임의로 chunks row 삭제 후 검사 호출

### AC-MEM-027 [REQ-MEM-027 / M5 / P1]

**Given** vault 의 markdown 디렉토리에 6 default collection + `custom/` 외 디렉토리 (예: `~/.mink/memory/markdown/garbage/`) 가 존재한다
**While** ingest scan 이 실행된다
**Then** `garbage/` 는 ignore 되며 한 줄 warning 이 출력된다
**Verify**: integration

---

## Section D — Unwanted Behavior (7개)

### AC-MEM-028 [REQ-MEM-028 / M2 / P0]

**Given** session transcript export 흐름
**When** 마스킹 파이프라인 적용 없이 직접 ingest 가 시도된다 (예: 코드 bypass)
**Then** ingest 가 명시적으로 reject 되고 error 반환
**Verify**: unit — redact 미적용 path 호출 시 sentinel error 단언

### AC-MEM-029 [REQ-MEM-029 / M1 / P0]

**Given** vault 또는 sqlite 파일 생성
**When** 파일 mode 를 0644 이상으로 변경 시도가 일어난다 (테스트 코드 시뮬레이션)
**Then** subsystem 이 mode 0600 으로 강제 재설정하거나 거부한다
**Verify**: unit — `os.Chmod(file, 0644)` 후 다음 vault touch 호출에서 mode 0600 복원 단언

### AC-MEM-030 [REQ-MEM-030 / M1 / P0]

**Given** 사용자가 `mink memory add` 를 호출한다
**Then** CLI 반환 시간은 100ms 이하이며, chunking/임베딩이 길어질 경우 background task 로 defer 된다
**Verify**: bench — 10회 호출 평균 + p95 ≤ 100ms

### AC-MEM-031 [REQ-MEM-031 / M3 / P0]

**Given** subsystem 의 네트워크 호출
**Then** 외부 endpoint 는 `http://localhost:11434` (ollama) 외에는 호출되지 않는다
**Verify**: integration — http test recorder 로 outbound URL 캡처 + allowlist 검증

### AC-MEM-032 [REQ-MEM-032 / M1 / P0]

**Given** vault 에 동일 source_path 의 파일이 이미 존재한다
**When** 사용자가 동일 source_path 로 `mink memory add` 를 호출한다
**Then** explicit error 가 반환되고 기존 vault 파일은 변경되지 않는다
**Verify**: e2e — 두 번째 add 호출 exit code != 0 + 파일 hash 단언

### AC-MEM-033 [REQ-MEM-033 / M5 / P1]

**Given** `mink memory export --format markdown`
**When** export 실행
**Then** export 출력 markdown 안에 embedding 벡터 또는 base64 인코딩된 벡터가 절대 포함되지 않는다
**Verify**: integration — export 결과 grep 부재 단언

### AC-MEM-034 [REQ-MEM-034 / M5 / P1]

**Given** `mink memory prune --before {date}` 가 실행 중 중간에 process kill 시뮬레이션 발생
**Then** markdown 삭제와 index 삭제가 동일 transaction 안에서 atomically 처리되어 partial state 가 남지 않는다 (markdown 만 삭제되고 index 가 잔존하는 상태 0)
**Verify**: integration — chaos test: kill -9 mid-prune + 후속 검사

---

## Section E — Optional Features (4개)

### AC-MEM-035 [REQ-MEM-035 / M3 / P1]

**Given** 사용자가 `memory.embedding.model = mxbai-embed-large` 로 설정한다
**Then** 새 chunks 의 chunk_id 가 갱신된 model_version 으로 derive 되며, 기존 chunks 는 stale 로 식별된다
**Verify**: integration — config switch 후 reindex 전/후 chunk_id 비교

### AC-MEM-036 [REQ-MEM-036 / M5 / P2]

**Given** 사용자가 `memory.clawmem_compat.enabled = true` 로 설정한다
**Then** vault write 가 `~/.mink/memory/markdown/` 와 `~/.clawmem/vault/` 에 동시에 mirror 된다 (REQ-MEM-024)
**Verify**: integration — AC-MEM-024 동일

### AC-MEM-037 [REQ-MEM-037 / M5 / P2]

**Given** 사용자가 `mink memory stats` 를 호출한다
**Then** per-collection chunk count + total embedding size on disk + oldest/newest chunk timestamp 가 사람이 읽기 쉬운 표 형식으로 출력된다
**Verify**: e2e — 출력 형식 검증 (헤더 컬럼 6개)

### AC-MEM-038 [REQ-MEM-038 / M5 / P2]

**Given** 사용자가 `mink memory import --source /path/to/other-vault` 를 호출한다
**When** import 대상 vault 의 schema version 이 현재와 호환된다
**Then** 두 vault 가 merge 되고, 충돌 (동일 chunk_id) 시 source 우선 정책 + warning 출력
**When** schema 가 비호환이면
**Then** import 가 reject 되고 exit code != 0 + diagnostic message 반환
**Verify**: integration — 호환/비호환 두 vault fixture 로 양쪽 path 검증

---

## Coverage Summary

| 섹션 | REQ 수 | AC 수 | 매핑 |
|---|---|---|---|
| A. Ubiquitous | 13 | 13 | 1:1 |
| B. Event-Driven | 8 | 8 | 1:1 |
| C. State-Driven | 6 | 6 | 1:1 |
| D. Unwanted | 7 | 7 | 1:1 |
| E. Optional | 4 | 4 | 1:1 |
| **Total** | **38** | **38** | **1:1** |

| Priority | REQ 수 | AC 수 |
|---|---|---|
| P0 | 17 | 17 |
| P1 | 14 | 14 |
| P2 | 7 | 7 |

| Verify type | AC 수 |
|---|---|
| unit | 8 |
| integration | 17 |
| e2e | 8 |
| bench | 2 |
| manual | 3 |

---

## Definition of Done (SPEC 전체)

- [ ] AC-MEM-001 .. AC-MEM-038 모두 GREEN
- [ ] coverage ≥ 85% (TRUST 5 Tested)
- [ ] golangci-lint clean (TRUST 5 Unified)
- [ ] CROSSPLAT-001 §5.1 매트릭스 통과 (macOS / Linux / Windows native)
- [ ] USERDATA-MIGRATE-001 amendment 머지 (export 항목에 `memory/` 추가)
- [ ] CLI-TUI-003 의 `mink memory` 7-subcommand 패리티
- [ ] ClawMem MCP 서버 vault mount 매뉴얼 검증 캡쳐
- [ ] README + `.moai/docs/memory.md` 작성 (ko)
- [ ] 모든 신규 소스 AGPL-3.0-only 헤더 존재
- [ ] sync PR merge → status `implemented`
