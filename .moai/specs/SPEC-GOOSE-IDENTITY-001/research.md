# SPEC-GOOSE-IDENTITY-001 research.md — Identity Graph 구현 자료

> 본 문서는 `spec.md` 의 설계 결정과 AC를 실구현으로 옮길 때 참고하는 기술 자료이다. 외부 의존 버전, 스키마 예시, TDD 테스트 전략, 리스크/완화를 기록한다.

---

## 1. 외부 의존 확정

| 의존 | 버전 고정 | 용도 | 근거 |
|-----|---------|----|----|
| `github.com/kuzu-db/kuzu-go` | v0.11.0 | 임베디드 그래프 DB (기본) | learning-engine.md §3.5, tech.md §2 |
| `github.com/neo4j/neo4j-go-driver/v5` | v5.26.x | 협업용 옵션 백엔드 | learning-engine.md §3.5 |
| `gopkg.in/yaml.v3` | v3.0.1 | `predicates.yaml` 로더 | 프로젝트 표준 |
| `github.com/google/uuid` | v1.6.0 | Entity/Fact ID (UUID v7) | 프로젝트 표준 |
| `github.com/piprate/json-gold` | v0.5.0 | JSON-LD 직렬화/검증 | 표준 구현체, 활발한 maintenance |
| `github.com/stretchr/testify` | v1.9.x | 테스트 프레임워크 | 프로젝트 표준 |
| `github.com/ory/dockertest/v3` | v3.11.x | Neo4j 통합 테스트 | CI-in-docker 패턴 |

고정 정책:
- Kuzu 0.11 미만은 Cypher 서브셋 상이로 제외.
- Neo4j 5.x 는 `apoc.export.cypher.all` 을 기본 번들에 포함한다는 가정(Aura 및 5.x OSS 모두 해당).

---

## 2. 패키지 디렉터리 계획

```
internal/learning/identity/
├── doc.go                 // 패키지 개요
├── types.go               // Entity, TemporalFact, PredicateDefinition
├── errors.go              // ErrInvalidEntityKind 등 모든 에러 심볼
├── interface.go           // IdentityGraph, Extractor, SHACLValidator
├── predicates.go          // LoadPredicates(path) + 내장 기본값
├── predicates.yaml        // 초기 어휘 (테스트/배포에서 복사)
├── shacl.go               // DefaultValidator 구현 (4-요소 서브셋)
├── shacl_test.go
├── kuzu/
│   ├── backend.go         // NewKuzuBackend + CRUD + Query
│   ├── schema.go          // DDL (node/rel tables)
│   ├── snapshot.go        // EXPORT DATABASE wrap
│   └── backend_test.go
├── neo4j/
│   ├── backend.go         // NewNeo4jBackend
│   ├── snapshot.go        // apoc.export.cypher.all wrap
│   └── backend_test.go    // -tags=integration
├── extractor/
│   ├── rule_based.go      // NewRuleBasedExtractor()
│   ├── llm_assisted.go    // ROUTER-001 호출 (옵션)
│   └── rule_based_test.go
├── jsonld/
│   ├── export.go          // ExportJSONLD 구현
│   ├── context.go         // urn:goose:identity:v1 vocabulary
│   └── export_test.go
└── contract/
    └── graph_contract_test.go   // Kuzu & Neo4j 공통 계약 테스트
```

---

## 3. Kuzu 스키마 DDL (초안)

```cypher
// Node tables: POLE+O 5종
CREATE NODE TABLE Person (
    id UUID PRIMARY KEY,
    name STRING,
    aliases STRING[],
    attributes_json STRING,
    embedding FLOAT[] DEFAULT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
CREATE NODE TABLE Organization (...same shape...);
CREATE NODE TABLE Location     (...);
CREATE NODE TABLE Event        (...);
CREATE NODE TABLE Object       (...);

// Rel table: 하나의 테이블로 모든 predicate 표현
CREATE REL TABLE FACT (
    FROM Person TO Person,
    FROM Person TO Organization,
    FROM Person TO Location,
    FROM Person TO Event,
    FROM Person TO Object,
    FROM Organization TO Location,
    -- 조합은 PredicateDefinition 에서 제한
    id UUID,
    predicate STRING,
    valid_from TIMESTAMP,
    valid_until TIMESTAMP,
    episode_id STRING,
    confidence DOUBLE,
    source STRING
);
```

Note: Kuzu 의 REL TABLE 은 다중 (FROM, TO) 조합을 지원하지만, 조합이 25+ 이면 유지보수가 어려워진다. 대안으로 **generic Entity 단일 node table + type discriminator** 를 고려 (오픈 이슈 §5).

---

## 4. SHACL 서브셋 알고리즘 (의사코드)

```
ValidateFact(fact, subjectKind, objectKind, existing):
    pred = PredicateRegistry[fact.Predicate]
    if pred == nil: return ErrPredicateUnknown

    // 1. Class 제약
    if subjectKind != pred.SubjectClass: return ErrSHACLViolation{"subject class"}
    if objectKind  != pred.ObjectClass:  return ErrSHACLViolation{"object class"}

    // 2. Cardinality 제약
    active = filter(existing, f -> f.Subject == fact.Subject && f.Predicate == fact.Predicate && f.ValidUntil == nil)
    if pred.MaxCardinality != -1 && len(active) >= pred.MaxCardinality:
        // 자동 close 또는 reject — REQ-IDENTITY-005 에 따라 maxCardinality=1 은 자동 close
        if pred.MaxCardinality == 1:
            return CloseExisting(active[0], fact.ValidFrom - 1ns)
        return ErrSHACLViolation{"cardinality"}

    // 3. MinCardinality 는 삭제 시 검증 (본 SPEC 범위 외, 추후)
    // 4. InverseOf 는 caller 가 트랜잭션 내에서 별도 insert
    return nil
```

---

## 5. 엔티티 추출 파이프라인

### 5.1 Rule-based (기본)

- **Person**: `[가-힣]{2,4}(씨|과장|대리|팀장|부장|이사|대표)` + 영어 `[A-Z][a-z]+(\s[A-Z][a-z]+){1,3}`.
- **Organization**: 고유명사 사전(samsung, google, ...) + 접미어 규칙("~사", "~팀", "Inc.", "LLC").
- **Location**: "서울", "강남", "San Francisco" 사전 + zip code 패턴.
- **Event**: 시간 표현("다음주 회의", "2026-05-15 생일") + trigger words ("생일", "회의", "휴가").
- **Object**: 남은 명사구. 기본 confidence 0.5 이하.

각 추출은 `TextSpan{EpisodeID, Start, End}` 로 원문 위치를 기록하여 감사 로그로 남긴다.

### 5.2 LLM-Assisted (옵션)

- 프롬프트: "다음 대화에서 POLE+O 엔티티만 JSON 배열로 추출하라. 각 항목은 {kind, name, confidence}."
- `identity.extractor.llm_assisted = true` 이고 `router.model` 이 지정되면 호출.
- 결과는 `SHACLValidator` 를 통과해야만 저장.
- 호출당 비용을 제한하기 위해 세션당 `max_llm_calls=10` (CONFIG-001 기본값).

---

## 6. JSON-LD Export 예시

```json
{
  "@context": {
    "@vocab": "urn:goose:identity:v1#",
    "name": "http://xmlns.com/foaf/0.1/name",
    "validFrom": "urn:goose:identity:v1#validFrom",
    "validUntil": "urn:goose:identity:v1#validUntil"
  },
  "@graph": [
    {
      "@id": "urn:goose:entity:user-goos",
      "@type": "Person",
      "name": "Goos",
      "aliases": ["행님"],
      "WORKS_AT": {
        "@id": "urn:goose:entity:org-samsung",
        "validFrom": "2024-01-15T00:00:00Z"
      }
    },
    {
      "@id": "urn:goose:entity:org-samsung",
      "@type": "Organization",
      "name": "Samsung"
    }
  ]
}
```

---

## 7. TDD 테스트 전략

### 7.1 RED 단계 (TDD 모드 — quality.yaml)

첫 RED 커밋은 `shacl_test.go` 에서 12개 위반 케이스를 작성한다. 통과 시점에는 `DefaultValidator` 는 존재만 하고 구현 없음.

### 7.2 GREEN 단계

1. `types.go`, `errors.go`, `predicates.go`(기본값) 먼저 통과.
2. `DefaultValidator` 로 SHACL 테스트 통과.
3. `KuzuBackend` 최소 구현 (PutEntity/AssertFact/GetEntity/ListFacts) 로 AC-001..005 통과.
4. Cypher write-guard (AC-006), Kuzu↔Neo4j 계약(AC-007) 순.
5. Extractor → JSON-LD → Snapshot/Restore → 에러 래핑 순.

### 7.3 REFACTOR 단계

- Kuzu 와 Neo4j 백엔드의 중복 로직은 `internal/learning/identity/common/` 으로 추출.
- `Normalize()` 헬퍼(이름 trim, case fold) 를 shared 위치로.

### 7.4 커버리지 목표

- `identity/` 패키지 전체 85% (quality.yaml 기본).
- `shacl.go`, `extractor/rule_based.go` 는 90%+.

### 7.5 통합 테스트

- Neo4j: `-tags=integration`, GitHub Actions 의 `services: neo4j` 사용.
- Kuzu: 인메모리 임시 디렉터리, OS-agnostic (다만 windows CGO 이슈는 오픈 이슈 §1).

---

## 8. 성능 벤치마크 목표

| 작업 | p50 | p95 | 비고 |
|----|----|----|----|
| `PutEntity` | 1.2 ms | 3.0 ms | Kuzu 로컬 SSD |
| `AssertFact` | 1.5 ms | 3.0 ms | SHACL 검증 포함 |
| `ListFacts(N=100)` | 2.0 ms | 5.0 ms | asOf 시간 필터 |
| `Query(cypher, hops=2)` | 5.0 ms | 20.0 ms | 2-hop path |
| `Snapshot(1만 fact)` | 400 ms | 1.5 s | EXPORT DATABASE |
| `ExportJSONLD(1만 fact)` | 600 ms | 2.0 s | json-gold |

---

## 9. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|------|----|----|
| Kuzu Go 바인딩이 Go 1.26 미지원 | **High** | CI matrix 에 1.25/1.26 모두 넣고 실패 시 커뮤니티 PR 또는 1.25 LTS 유지 |
| Neo4j bolt 연결 끊김 | Medium | 재연결 back-off (250ms..5s, max 5회), `ErrBackendUnavailable` 로 wrap |
| SHACL 서브셋이 표현력 부족 | Low | `predicates.yaml` 에 `custom_validator` 확장 포인트 예약 |
| 동명이인 "김과장" 2명 | Medium | 초기 구현은 heuristic(가장 최근 언급), 차기 SPEC에서 disambiguation |
| `maxCardinality=1` 레이스 | Medium | Kuzu/Neo4j 모두 단일 write transaction 으로 감싸 원자성 보장 |
| JSON-LD playground 의존 | Low | 골든파일은 로컬 고정, 외부 서비스 네트워크 호출 금지 |

---

## 10. 사용자 CLI (참고용 스케치)

```bash
$ goose identity dump
Person: Goos (aliases: 행님)
  WORKS_AT → Samsung (since 2024-01-15)
  REPORTS_TO → Kim (Manager) (since 2026-04-01)
  LIKES → Rust Programming
Event: Mother's Birthday (2026-05-15)

$ goose identity export --format=jsonld --out=me.jsonld
Exported 14 entities, 27 facts to me.jsonld

$ goose identity snapshot ~/.goose/snapshots/2026-04-21.kuzu
Snapshot written (1.1 MB)
```

본 CLI 는 CLI-001 의 후속 PR에서 구현한다. 본 SPEC은 인터페이스·에러·출력 포맷만 고정.

---

## 11. 추적성

- spec.md REQ-IDENTITY-001..018 → 본 문서 §4 (SHACL), §5 (Extractor), §7 (TDD).
- AC-IDENTITY-001..012 → 본 문서 §7 (테스트 전략), §8 (성능).
- learning-engine.md §3.1..3.5 → spec.md DD-IDENTITY-01..07.
- adaptation.md §10, §11 → spec.md §10 Exclusions + AC-IDENTITY-010 (export).
