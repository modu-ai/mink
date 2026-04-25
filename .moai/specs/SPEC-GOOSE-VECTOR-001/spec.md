---
id: SPEC-GOOSE-VECTOR-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P2
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-VECTOR-001 — Preference Vector Space (768-dim, EMA update, cosine search)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (learning-engine.md §4 + tech.md §2.1 goose-vector 기반) | manager-spec |

---

## 1. 개요 (Overview)

모든 상호작용을 **고정 차원 벡터(기본 768-dim)** 로 압축하여 사용자의 "현재 선호도 상태"를 표현하고, 유사도 기반 검색·재정렬·(선택적) 유사 사용자 탐색을 제공한다. 본 SPEC은 다음 네 요소를 하나의 일관된 계약으로 제공한다:

1. **Embedding Provider 추상화** — OpenAI `text-embedding-3-small`(768-dim) / Ollama `nomic-embed-text`(768-dim) / 사용자 지정 모델. 모두 `EmbeddingProvider` 인터페이스를 구현.
2. **Preference Vector 계산·업데이트** — 지수 이동평균(EMA, decay=0.95/day) 기반 증분 업데이트. session-level/user-level 두 scope 모두 지원.
3. **Similarity Search 백엔드** — Qdrant(로컬) 기본, fallback 으로 순수 Go HNSW(`github.com/coder/hnsw` 또는 `github.com/philippgille/chromem-go`).
4. **Personalized Re-ranking** — `SearchResult` 목록을 user vector 기반으로 재정렬.

본 SPEC은 `internal/learning/vector/` 패키지의 인터페이스·불변식·관찰 가능 행동을 규정한다. 엔티티 정의·관계 저장은 IDENTITY-001, LoRA 훈련은 LORA-001, 저장 계층은 MEMORY-001에 위임한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- `learning-engine.md` §4 (Preference Vector Space)는 개인화의 **수치적 기반**으로 768-dim 사용자 임베딩과 cosine 유사도 검색을 규정. §4.3 Personalized Ranking은 검색 결과 재정렬을 요구.
- LORA-001 의 훈련 데이터 필터링은 **"현재 preference vector 와 cosine>=0.7 인 interaction"** 을 입력으로 사용. 따라서 LORA-001 구현 전에 VECTOR-001 의 `UserVector` API 가 반드시 존재해야 함.
- tech.md §2.1 의 `goose-vector` Rust 크레이트는 hnsw/SIMD 가속을 담당. 본 Go SPEC 은 **인터페이스·오케스트레이션**만 정의하고, 성능 크리티컬 hot path 는 **차후 Rust 위임** 가능성을 열어둔다(Non-Goals 참조).

### 2.2 상속 자산 (패턴만 계승)

- **OpenAI `text-embedding-3-small`** (1536→768 dim reducible): GOOSE 는 dimensions=768 고정 호출로 사용.
- **Ollama `nomic-embed-text`** (768-dim native): 로컬·프라이버시 모드 기본값.
- **Qdrant v1.9+** Go client(`github.com/qdrant/go-client`): 로컬 임베디드 모드 지원.
- **chromem-go / coder/hnsw**: Pure Go HNSW, Qdrant 미설치 환경에서 fallback.

### 2.3 범위 경계

- **IN**: `EmbeddingProvider` 인터페이스, `VectorSpace` 인터페이스, EMA 증분 업데이트, cosine similarity 계산, Qdrant/HNSW 어댑터, Personalized Re-ranking API, 사용자 간 유사도 탐색(opt-in + 익명화), 벡터 스냅샷.
- **OUT**: Embedding model 자체 훈련/튜닝, Collaborative filtering 추천 생성(본 SPEC 은 "유사 사용자 ID 리스트"만 반환), Federated learning(→ Phase 7+ privacy), ANN 알고리즘 교체(HNSW 외), SIMD 최적화(→ Rust goose-vector).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/learning/vector/` 패키지 생성.
2. `EmbeddingProvider` 인터페이스: `Embed(ctx, text) ([]float32, error)`, `Dimension() int`, `Name() string`.
3. 기본 provider 3종:
   - `NewOpenAIEmbedder(apiKey, "text-embedding-3-small", dim=768)` — CREDPOOL-001 경유 호출.
   - `NewOllamaEmbedder(host, "nomic-embed-text", dim=768)`.
   - `NewDummyEmbedder(dim)` — 테스트용(고정 벡터 반환, deterministic).
4. `PreferenceVector` 구조체: dimension, vector []float32, timestamp, decay_factor, confidence, num_interactions, last_updated.
5. `VectorSpace` 인터페이스 — ComputeUserVector, UpdateUserVector(EMA), GetUserVector, Upsert, Search(topK, threshold), FindSimilarUsers(opt-in), Delete, Snapshot, Restore, Close.
6. EMA 업데이트 공식: `v_new = (1-α) * v_old * decay + α * v_interaction`, 기본 α=0.1, decay=0.95^(days_since_last_update).
7. `QdrantBackend` 기본 구현: gRPC 클라이언트, collection 자동 생성(dim=768, distance=Cosine).
8. `HNSWBackend` fallback 구현: pure Go, 메모리 인덱스 + 주기적 파일 flush.
9. `Personalized Re-ranker`: 입력 `[]SearchResult`, 출력 `[]RankedResult` (original_rank, personalized_rank, score).
10. 유사 사용자 탐색 `FindSimilarUsers(userID, topK)`: **opt-in 플래그** + **사용자 ID 익명화**(해시) + confidence threshold(>=0.6).
11. 벡터 정규화(L2 norm=1) — 저장/조회 시 자동.
12. Incremental batch embedding: `EmbedBatch(ctx, []string) ([][]float32, error)` (provider가 native batch 지원 시 활용).

### 3.2 OUT OF SCOPE (명시적 제외)

- **Embedding 모델 fine-tuning**: LoRA training 은 LORA-001, 본 SPEC 은 pre-trained embedding 사용만.
- **Recommendation 생성**: 유사 사용자를 바탕으로 "이 스킬 추천" 같은 결과 생성은 Proactive Engine (Phase 6+).
- **SIMD/AVX2 최적화**: 768-dim cosine 계산의 SIMD 구현은 Rust `goose-vector` crate 로 위임.
- **Cross-language 사용자 벡터 join**: en/ko/ja/zh 각각의 embedding space 차이는 본 SPEC 에서 처리 안 함 (Phase 7+).
- **Qdrant 클러스터 모드**: 단일 노드만.
- **Embedding 버전 migration**: embedding model 이 바뀌면 **전체 재계산**만. auto-migration 없음.

---

## 4. 목표 (Goals)

- 사용자 1명의 첫 vector 계산은 interaction 3개 축적 후 200ms 이내 (OpenAI 네트워크 제외).
- `Search(topK=10)` p95 레이턴시 10ms 이하 (Qdrant 로컬, 1M 포인트 기준).
- EMA 업데이트는 atomic (lost update 없음, 10 concurrent writer 에서 검증).
- Qdrant↔HNSW 백엔드 전환 시 동일 계약 100% 통과.
- 유사 사용자 탐색 opt-out 이 기본값이고, opt-in 시에도 사용자 ID 는 salted SHA-256 해시로만 외부 노출.

## 5. 비목표 (Non-Goals)

- 1M+ 사용자 규모의 cross-user 유사도 그래프 구축.
- 768-dim 외의 dimension 동적 지원(본 SPEC 은 dim=768 고정, 바꾸려면 새 SPEC).
- Embedding provider 의 rate limit 관리(→ RATELIMIT-001 경유).
- 벡터 암호화 저장(→ Phase 7+ privacy).

---

## 6. 요구사항 (EARS Requirements)

### REQ-VECTOR-001 [Ubiquitous]
The Preference Vector service shall expose the `VectorSpace` Go interface with method set: `Upsert`, `ComputeUserVector`, `UpdateUserVector`, `GetUserVector`, `Search`, `FindSimilarUsers`, `Delete`, `Snapshot`, `Restore`, `Close`.

### REQ-VECTOR-002 [Ubiquitous]
All vectors stored or returned by the service shall be L2-normalized (`||v|| = 1.0 ± 1e-6`). Un-normalized inputs to `Upsert` shall be normalized automatically before persistence.

### REQ-VECTOR-003 [Ubiquitous]
The service shall enforce a fixed embedding dimension of 768. Any `Upsert` with a vector of length != 768 shall return `ErrDimensionMismatch`.

### REQ-VECTOR-004 [Event-Driven]
When `UpdateUserVector(userID, interactionVector)` is called, the service shall apply the EMA formula `v_new = (1-α) * decay(v_old) * decay + α * interactionVector` atomically, where α defaults to 0.1 and decay = 0.95^(days_since_last_update).

### REQ-VECTOR-005 [Event-Driven]
When `Search(userVector, topK, threshold)` is called, the service shall return at most `topK` results whose cosine similarity with `userVector` is `>= threshold` (default threshold = 0.0), sorted by similarity descending.

### REQ-VECTOR-006 [State-Driven]
While the configuration value `vector.backend` is `qdrant` (default), all persistence operations shall be executed against a local Qdrant instance using the `github.com/qdrant/go-client` gRPC driver.

### REQ-VECTOR-007 [State-Driven]
While the configuration value `vector.backend` is `hnsw`, the service shall use the pure-Go HNSW fallback with a periodic file flush to `$GOOSE_HOME/vector/hnsw.idx`.

### REQ-VECTOR-008 [Event-Driven]
When `FindSimilarUsers(userID, topK)` is called and the configuration value `vector.share_similarity = false` (default), the service shall return `ErrSimilarityOptOut` without any computation.

### REQ-VECTOR-009 [State-Driven]
While `vector.share_similarity = true`, `FindSimilarUsers` shall compute similarities using **salted SHA-256 hashed user IDs** for the return payload; raw user IDs shall never leave the service boundary.

### REQ-VECTOR-010 [Event-Driven]
When `EmbedBatch(ctx, texts)` is called and the active provider exposes a batch API (e.g., OpenAI), the service shall invoke that batch API once; otherwise the service shall parallelize single-text calls with a bounded worker pool (default 4).

### REQ-VECTOR-011 [Unwanted]
The service shall not persist interaction-level embeddings tied to PII (raw user message text) unless the caller sets `SaveInteractionEmbedding = true` in the options; by default only the aggregated user vector is persisted.

### REQ-VECTOR-012 [Unwanted]
The service shall not silently switch embedding providers mid-session. Provider change shall require an explicit `NewVectorSpace(cfg)` invocation.

### REQ-VECTOR-013 [Optional]
Where the caller provides a `WeightFunc(interaction)` in `UpdateUserVectorOptions`, the service shall apply that weight to the interaction vector before EMA folding; absent weight defaults to 1.0.

### REQ-VECTOR-014 [Complex]
While a snapshot is in progress, when `Upsert` is called concurrently, the service shall queue the upsert (default queue depth 4096) until the snapshot completes; overflow shall return `ErrSnapshotInProgress`.

---

## 7. 설계 결정 (Design Decisions)

### DD-VECTOR-01 — 768-dim 고정

`nomic-embed-text`(로컬) 와 `text-embedding-3-small`(dim=768 옵션) 둘 다 지원 가능. 384/1024/1536 는 interop 비용이 크다. 고정으로 단순성 확보.

### DD-VECTOR-02 — EMA 증분 업데이트

Full re-embedding 을 주기적으로 하지 않고 interaction 단위로 증분 업데이트. decay=0.95/day 로 1개월 지나면 초기 기여도가 ~20%로 감소.

### DD-VECTOR-03 — Qdrant 기본 + HNSW fallback

Qdrant 는 filtering/payload 지원이 우수하고 gRPC 인터페이스로 backpressure 관리가 쉽다. 그러나 "설치 없는 Go 바이너리" 요구가 있을 때를 대비해 pure-Go HNSW 를 **동등한 인터페이스**로 제공.

### DD-VECTOR-04 — 사용자 간 유사도는 opt-in + 해시

프라이버시 관점에서 raw user ID를 join 키로 사용하지 않는다. salted SHA-256 (`salt` 는 `$GOOSE_HOME` 에 저장된 device-local 값). 사용자가 명시적 opt-in 한 경우에만 연산 자체가 실행됨.

### DD-VECTOR-05 — Interaction-level embedding 은 기본 미저장

Raw message → embedding 은 PII 로 간주. 기본은 **user vector (aggregate)** 만 저장. 세밀한 debug/분석 필요 시 `SaveInteractionEmbedding = true` (기본 false).

### DD-VECTOR-06 — Dimension migration 은 수동

Embedding model 이 바뀌면 dim 이 달라질 가능성 높음. 본 SPEC 은 dim=768 고정, migration 은 별도 CLI 명령 (`goose vector reindex --from=<old> --to=<new>`)으로만 제공.

### DD-VECTOR-07 — Go native + Rust 위임 경계

768-dim cosine 계산은 Go 에서도 ~0.8µs 수준이라 성능 병목이 아니다. 그러나 "10만 벡터 브루트포스 search" 같은 상황에서는 `goose-vector` Rust crate 로 위임 가능하도록 `SimilarityEngine` 인터페이스를 분리한다. 본 SPEC 의 기본 구현은 pure Go.

---

## 8. 데이터 모델 (Data Model)

```go
// internal/learning/vector/types.go

// PreferenceVector — 사용자의 현재 선호도 상태 요약
type PreferenceVector struct {
    UserID          string     // salted hash 가 아님, raw ID (내부용)
    Dimension       int        // 고정 768
    Vector          []float32  // L2-normalized
    DecayFactor     float64    // 기본 0.95
    Confidence      float64    // [0, 1], interaction 누적에 따라 증가
    NumInteractions int        // 누적 interaction 수
    LastUpdated     time.Time
    Timestamp       time.Time  // 벡터 초기 생성 시점
}

// EmbeddingProvider — OpenAI / Ollama / Dummy
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int                // 반드시 768
    Name() string                  // "openai:text-embedding-3-small", "ollama:nomic-embed-text"
}

// VectorSpace — 상위 소비자 인터페이스
type VectorSpace interface {
    // Upsert/Search
    Upsert(ctx context.Context, point Point) error
    Search(ctx context.Context, queryVec []float32, topK int, threshold float64) ([]SearchResult, error)

    // User vector 특화
    ComputeUserVector(ctx context.Context, userID string, interactions []Interaction) (PreferenceVector, error)
    UpdateUserVector(ctx context.Context, userID string, interactionVec []float32, opts UpdateOptions) error
    GetUserVector(ctx context.Context, userID string) (PreferenceVector, error)

    // 유사 사용자
    FindSimilarUsers(ctx context.Context, userID string, topK int) ([]UserSimilarity, error)

    // 생명주기
    Delete(ctx context.Context, id string) error
    Snapshot(ctx context.Context, path string) error
    Restore(ctx context.Context, path string) error
    Close(ctx context.Context) error
}

type Point struct {
    ID       string
    Vector   []float32
    Payload  map[string]any          // ex: {"type":"user","created":...}
}

type SearchResult struct {
    ID         string
    Score      float64                // cosine similarity
    Payload    map[string]any
}

type UserSimilarity struct {
    HashedUserID string              // salted SHA-256
    Score        float64              // [0, 1]
}

type UpdateOptions struct {
    Alpha                   float64   // EMA 학습률 (기본 0.1)
    WeightFunc              func(interactionVec []float32) float64
    SaveInteractionEmbedding bool     // 기본 false (PII 보호)
}

type Interaction struct {
    Text      string
    Timestamp time.Time
    Weight    float64                 // default 1.0
}
```

---

## 9. API/인터페이스 (Public Surface)

공개 심볼:

- `EmbeddingProvider`, `VectorSpace`, `PreferenceVector`, `Point`, `SearchResult`, `UserSimilarity`, `UpdateOptions`, `Interaction`
- `NewOpenAIEmbedder(cfg OpenAIConfig) (EmbeddingProvider, error)`
- `NewOllamaEmbedder(cfg OllamaConfig) (EmbeddingProvider, error)`
- `NewDummyEmbedder(seed int64) EmbeddingProvider`
- `NewQdrantBackend(cfg QdrantConfig) (VectorSpace, error)`
- `NewHNSWBackend(cfg HNSWConfig) (VectorSpace, error)`
- `NewVectorSpace(provider EmbeddingProvider, backend VectorSpace) VectorSpace` — decorator (embedding 자동 호출 포함)

에러 심볼: `ErrDimensionMismatch`, `ErrSimilarityOptOut`, `ErrBackendUnavailable`, `ErrSnapshotInProgress`, `ErrUserNotFound`.

---

## 10. Exclusions (What NOT to Build)

- LoRA 학습 데이터셋 구성 (→ LORA-001).
- 사용자 페르소나 태그 추론 (→ IDENTITY-001).
- Federated / secure aggregation (→ Phase 7+).
- Embedding model 의 한국어 성능 교차검증 (model comparison 은 상위 task).
- 벡터 DB 의 정량적 인덱스 튜닝 (M, ef_construct 등) UI — 본 SPEC 은 default 값 고정.
- A/B 실험 infrastructure (recommendation efficacy 측정은 별도 SPEC).
- Cross-user recommendation 생성 (Proactive Engine).

---

## 11. Acceptance Criteria

### AC-VECTOR-001 — 인터페이스 계약
Given `NewHNSWBackend(...)` 로 서비스를 초기화하면, When `Upsert` → `Search(topK=5)` → `Delete` 를 순차 호출할 때, Then 모든 호출이 nil 에러를 반환하고 조회 결과는 해당 ID를 포함/제외한다.

### AC-VECTOR-002 — Dimension 고정
When `Upsert` 에 길이 512 벡터를 전달하면, Then `ErrDimensionMismatch` 가 반환되고 저장되지 않는다.

### AC-VECTOR-003 — L2 정규화
Given 임의의 길이의 벡터 `v`(||v||=3.0)를 `Upsert` 한 뒤, When `Search` 로 해당 ID 를 조회하면, Then 반환 벡터의 L2 norm 이 `1.0 ± 1e-6` 이다.

### AC-VECTOR-004 — EMA 업데이트 원자성
Given 동일 사용자에 대해 10개의 concurrent `UpdateUserVector` 가 실행될 때, Then 모든 업데이트가 반영되고 최종 `NumInteractions` 은 정확히 10 이다(lost update 없음).

### AC-VECTOR-005 — Cosine 검색 정확성
Given 100개의 랜덤 포인트가 업서트된 후 특정 포인트 `p*` 와 유사한 쿼리를 보낼 때, When `Search(topK=1)` 를 호출하면, Then 반환 첫 결과의 ID == `p*.ID` 이다.

### AC-VECTOR-006 — Opt-out 기본
When `vector.share_similarity = false` 상태로 `FindSimilarUsers("user123", 5)` 를 호출하면, Then `ErrSimilarityOptOut` 이 반환되고 아무런 유사도 계산도 수행되지 않는다 (Qdrant 통계 미증가).

### AC-VECTOR-007 — Opt-in 시 해시 ID
Given `vector.share_similarity = true`, When `FindSimilarUsers("user123", 5)` 를 호출하면, Then 반환된 각 `UserSimilarity.HashedUserID` 는 salted SHA-256 hex (길이 64)이고 원본 "user123" 문자열은 포함되지 않는다.

### AC-VECTOR-008 — 백엔드 동등성
동일 계약 테스트 스위트가 `NewQdrantBackend`(dockertest) 와 `NewHNSWBackend` 두 백엔드 모두에서 100% 통과한다.

---

## 12. 테스트 전략 (요약, 상세는 research.md)

- 단위: `ema_test.go`(공식 정확성), `normalize_test.go`, `hashing_test.go`(salt 고정 시 재현성).
- 통합(Qdrant): dockertest 기반 `-tags=integration`.
- 통합(HNSW): 인메모리, race 모드.
- 계약: `vectorspace_contract_test.go` 를 양쪽에.
- Property-based: 100 random vectors → L2 norm invariant.
- 커버리지: 85%+ (quality.yaml 기본).

---

## 13. 의존성 & 영향 (Dependencies & Impact)

- **상위 의존**: MEMORY-001 (episode·interaction 원본은 MEMORY의 Trajectory 에서 읽는다).
- **하위 소비자**: LORA-001 (훈련 샘플 유사도 필터), IDENTITY-001 (`Entity.Embedding` 저장을 본 SPEC 의 HashedID 전략과 합의), Proactive Engine.
- **CLI 영향**: `goose vector status`, `goose vector reindex`, `goose vector export <userID>`.

---

## 14. 오픈 이슈 (Open Issues)

1. **OpenAI `text-embedding-3-small` dim=768 지원**: API 파라미터 `dimensions=768` 이 account tier 에 따라 제한이 있을 수 있음. fallback 시 1536→768 PCA 수행 여부 결정.
2. **Qdrant 임베디드 모드 안정성**: 0.11.x 까지는 embedded 지원 불완전. 완전한 embedded 지원이 준비될 때까지 localhost:6334 stand-alone 프로세스 가정.
3. **HNSW 파일 포맷**: chromem-go vs coder/hnsw 의 마이그레이션 호환성. 선택 후 포맷 lock-in.
4. **Salt 배포 정책**: 기기별 salt 를 `$GOOSE_HOME/.salt` 에 저장. 기기 이전 시 FindSimilarUsers 결과가 불연속. 사용자에게 고지 필요.
5. **Weight func 의 직렬화**: `WeightFunc` 는 Go 함수 값이라 config 에 저장 불가. 프리셋(`"recency"`, `"length"`, `"satisfaction"`) 문자열로 간접 참조할지 결정.
