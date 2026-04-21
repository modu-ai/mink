# SPEC-GOOSE-VECTOR-001 research.md — Preference Vector Space 구현 자료

> `spec.md` 의 설계 결정을 실제 Go 코드로 옮길 때 참고. 외부 의존, EMA 알고리즘 검증, TDD 전략, 리스크 기록.

---

## 1. 외부 의존 확정

| 의존 | 버전 고정 | 용도 | 근거 |
|-----|---------|----|----|
| `github.com/qdrant/go-client` | v1.10.x | Qdrant gRPC 클라이언트 | spec §3.1, 널리 쓰이는 공식 바인딩 |
| `github.com/coder/hnsw` | v0.5.x | Pure-Go HNSW (fallback) | 단일 패키지, 의존 없음, race-safe |
| `github.com/sashabaranov/go-openai` | v1.32.x | OpenAI embedding API | 기존 ADAPTER-001 과 동일 library |
| `github.com/ollama/ollama-go` (or raw HTTP) | 최신 | 로컬 Ollama 호출 | tech.md §3.2 |
| `github.com/stretchr/testify` | v1.9.x | 테스트 | 프로젝트 표준 |
| `github.com/ory/dockertest/v3` | v3.11.x | Qdrant 통합 테스트 | CI 표준 |

라이브러리 선택 근거:
- `coder/hnsw` 는 stdlib 만 사용(zero-dep), thread-safe, 공식 Go 팀 아님이지만 유지보수 활발.
- `chromem-go` 와 비교 시 coder/hnsw 쪽이 파일 포맷 안정성이 우수(2026-04 기준).

---

## 2. 패키지 디렉터리 계획

```
internal/learning/vector/
├── doc.go
├── types.go              // PreferenceVector, Point, SearchResult, UpdateOptions
├── errors.go
├── interface.go          // VectorSpace, EmbeddingProvider
├── normalize.go          // L2 정규화
├── ema.go                // EMA 공식
├── hashing.go            // Salted SHA-256 (user ID anonymization)
├── provider/
│   ├── openai.go
│   ├── ollama.go
│   ├── dummy.go
│   └── openai_test.go
├── qdrant/
│   ├── backend.go
│   ├── collection.go
│   └── backend_test.go   // -tags=integration
├── hnsw/
│   ├── backend.go
│   ├── persist.go        // 파일 flush / restore
│   └── backend_test.go
├── search.go             // Personalized re-rank
└── contract/
    └── vectorspace_contract_test.go
```

---

## 3. EMA 업데이트 공식 검증

```
v_old: 기존 사용자 벡터 (L2-normalized)
v_int: 새 interaction 벡터 (L2-normalized)
days:  time.Since(v_old.LastUpdated).Hours() / 24
decay: 0.95 ^ days
alpha: 학습률 (기본 0.1)
w:     WeightFunc(v_int) 가 반환 (기본 1.0)

v_new_raw = (1 - alpha) * decay * v_old + alpha * w * v_int
v_new     = v_new_raw / ||v_new_raw||

NumInteractions++
Confidence = min(1.0, NumInteractions / 50)   // 50 interaction 쌓이면 포화
LastUpdated = now
```

### 3.1 불변식

- 업데이트 후에도 `||v_new|| ∈ [1 - 1e-6, 1 + 1e-6]`.
- `NumInteractions` 는 **단조증가** (업데이트 실패 시 롤백).
- `LastUpdated` 는 strict monotone per user.

### 3.2 Concurrency 처리

- 사용자별 `sync.Mutex` 맵 (sync.Map 으로 지연 생성).
- 또는 Qdrant/HNSW 수준의 upsert-with-version 사용.
- 현재 계획: application-level per-user mutex (10k 사용자 수준 충분).

---

## 4. L2 정규화 (normalize.go)

```go
func Normalize(v []float32) []float32 {
    var sumSq float64
    for _, x := range v {
        sumSq += float64(x) * float64(x)
    }
    norm := math.Sqrt(sumSq)
    if norm < 1e-12 {
        return v // zero vector: skip
    }
    out := make([]float32, len(v))
    invNorm := float32(1.0 / norm)
    for i, x := range v {
        out[i] = x * invNorm
    }
    return out
}
```

property-based test: 10000 random vectors, 정규화 후 norm ∈ [1-1e-5, 1+1e-5].

---

## 5. Salted SHA-256 사용자 ID 해싱

```go
// $GOOSE_HOME/.salt 에서 64-byte random salt 로드
func HashUserID(salt, userID string) string {
    h := sha256.New()
    h.Write([]byte(salt))
    h.Write([]byte(":"))
    h.Write([]byte(userID))
    return hex.EncodeToString(h.Sum(nil))
}
```

salt 가 기기마다 다르기 때문에 **서로 다른 기기의 동일 사용자 ID 가 다른 해시**를 생성한다. 이는 cross-device linkability 방지의 trade-off (일관성 vs privacy). 본 SPEC 은 privacy 우선.

---

## 6. Qdrant Collection 스키마

```yaml
collection_name: goose_preferences
vectors:
  size: 768
  distance: Cosine
hnsw_config:
  m: 16
  ef_construct: 128
  full_scan_threshold: 10000
optimizers_config:
  default_segment_number: 2
payload_schema:
  type: keyword
  created_at: datetime
```

---

## 7. TDD 테스트 전략

### 7.1 RED 단계

- `ema_test.go`: EMA 공식의 수치 정확성 (고정 입력 → 기대 출력, 1e-6 허용).
- `normalize_test.go`: property-based (10000 random).
- `dimension_test.go`: `ErrDimensionMismatch` 경로.

### 7.2 GREEN 단계

1. `types.go`, `errors.go`, `normalize.go`, `ema.go`, `hashing.go`.
2. `DummyEmbedder` (테스트 전용) 구현.
3. `HNSWBackend` 최소 구현 → AC-VECTOR-001..005.
4. Opt-out/opt-in (AC-006, AC-007).
5. Qdrant backend 및 계약 테스트 (AC-008).

### 7.3 REFACTOR 단계

- Qdrant/HNSW 공통 로직(예: upsert retry, metrics collection) 을 `backend/shared/` 로 추출.
- `SearchResult` 변환 헬퍼 통합.

### 7.4 커버리지

- 전체 85%+; `ema.go`, `normalize.go`, `hashing.go` 는 95%+.

---

## 8. 성능 벤치마크 목표

| 작업 | p50 | p95 | 백엔드 |
|----|----|----|----|
| `Upsert(1 point)` | 0.8 ms | 2.5 ms | Qdrant local |
| `Search(topK=10, N=1M)` | 4 ms | 10 ms | Qdrant HNSW m=16 |
| `Search(topK=10, N=100K)` | 2 ms | 5 ms | coder/hnsw |
| `UpdateUserVector` | 0.3 ms | 1.0 ms | in-memory EMA + Qdrant upsert |
| `Normalize(768)` | 0.8 µs | 1.5 µs | pure Go |
| `FindSimilarUsers(topK=5, users=10K)` | 8 ms | 25 ms | Qdrant filter + rerank |

---

## 9. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|------|----|----|
| OpenAI dim=768 tier 제한 | Medium | 호출 시 fallback: 1536→768 PCA 로컬 (`gonum`) |
| Qdrant 임베디드 모드 미성숙 | High | stand-alone docker 필수, embed 는 Phase 7+ 로 연기 |
| HNSW 파일 포맷 migration | Low | 포맷 버전 헤더(`v1`) 필드, 향후 변경 시 reindex |
| salt 유실로 cross-session 불연속 | Medium | `goose vector regenerate-salt` 명령 + 사용자 고지 |
| EMA 파라미터 튜닝 (α, decay) | Low | 초기 값은 learning-engine.md §4 근거; 향후 REFLECT-001 이 사용자별 최적화 |
| 모델 차원 변경 시 migration | Medium | 수동 reindex CLI, auto-migration 없음 (DD-VECTOR-06) |
| Opt-in 유사 사용자 오용 | High | `share_similarity=true` 시 사용자에게 Warning 표시 (CLI-001 책임) |

---

## 10. 관측·운영

- 메트릭 (Prometheus naming):
  - `goose_vector_upsert_total{backend}`
  - `goose_vector_search_duration_seconds{backend}`
  - `goose_vector_user_update_total`
  - `goose_vector_similarity_opted_in_users`
- 로그: provider 호출 실패는 warn 이상, EMA conflict(version mismatch)는 info.

---

## 11. 추적성

- spec.md REQ-VECTOR-001..014 → 본 문서 §3 (EMA), §5 (hashing), §7 (TDD).
- AC-VECTOR-001..008 → 본 문서 §7 (테스트), §8 (성능).
- learning-engine.md §4 → spec.md DD-VECTOR-01..07.
- adaptation.md §10.2 → spec.md §10 Exclusions (LoRA 위임).
