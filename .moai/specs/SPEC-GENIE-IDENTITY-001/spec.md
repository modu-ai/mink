---
id: SPEC-GENIE-IDENTITY-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P2
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GENIE-IDENTITY-001 — Identity Graph (POLE+O 스키마, Kuzu 임베디드, Temporal Context)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (learning-engine.md §3 + adaptation.md §10 + tech.md §2-3 기반) | manager-spec |

---

## 1. 개요 (Overview)

GENIE가 사용자를 "한 사람"으로 이해하기 위한 **Identity Graph** 를 정의한다. 사용자와 주변(사람/조직/위치/사건/사물)의 관계를 **POLE+O 스키마**(Person, Organization, Location, Event, Object)로 표현하고, 모든 사실(fact)에 **temporal validity window**(`valid_from`, `valid_until`)를 부여한다. 저장소는 로컬 임베디드 그래프 DB인 **Kuzu** 를 기본값으로 하고, 협업·다중 사용자 시나리오에서는 **Neo4j 어댑터**를 선택적으로 활성화한다.

본 SPEC은 다음 네 요소를 하나의 일관된 계약으로 제공한다:

1. **POLE+O 스키마 정의** — 5개 엔티티 타입 + 관계(predicate) 어휘 + SHACL 제약.
2. **Temporal fact assertion** — `AssertFact(subject, predicate, object, validFrom, validUntil)` 와 시간 범위 쿼리.
3. **엔티티 자동 추출** — 상호작용 트랜스크립트(SPEC-GENIE-TRAJECTORY-001 결과물)에서 NER + 규칙 기반 추출.
4. **백엔드 어댑터** — `IdentityGraph` 인터페이스 + `KuzuBackend`(기본) + `Neo4jBackend`(옵션).

본 SPEC은 `internal/learning/identity/` 패키지의 인터페이스 계약과 **관찰 가능한 행동**을 규정한다. Vector embedding 연산(§4 sibling)은 SPEC-GENIE-VECTOR-001이, 승격/안전장치는 SPEC-GENIE-REFLECT-001/SAFETY-001이 담당한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- `.moai/project/learning-engine.md` §3 (User Identity Graph)이 POLE+O 스키마와 Kuzu 임베디드 백엔드를 명시. `.moai/project/adaptation.md` §10은 Identity Graph가 persona/style/cultural 데이터의 **정준 저장소**임을 선언.
- Phase 6 Deep Personalization의 첫 번째 의존 항목. VECTOR-001은 "사용자 프로필 벡터"를 구성할 때 Identity Graph의 엔티티 요약을 입력으로 요구하고, LORA-001은 훈련 데이터셋 구성 시 사용자의 **현재 유효한 관계**만 인용해야 함.
- MEMORY-001(Builtin+Plugin)과 SAFETY-001(Frozen zones)가 먼저 존재. 본 SPEC은 그 위에 **그래프형 knowledge layer**를 쌓는다.

### 2.2 상속 자산 (패턴만 계승)

- **Graphiti (Zep, 2024, open-source)**: Temporal Graph RAG 패턴. Entity extraction + episode-based fact tracking. GENIE는 Graphiti의 validity-window 개념만 차용하고, 저장소는 Kuzu 임베디드로 대체.
- **Kuzu 0.11.x**: Embedded property-graph DB. Cypher 서브셋 지원, 단일 바이너리, ACID. `github.com/kuzu-db/kuzu-go` 바인딩.
- **Neo4j 5.26**: 협업·멀티유저 백엔드 옵션. `github.com/neo4j/neo4j-go-driver/v5`.
- **SHACL**(W3C Shapes Constraint Language): 스키마 제약 검증 표준. 본 SPEC은 **서브셋**(class, datatype, min/maxCount, inverseOf)만 구현.

### 2.3 범위 경계

- **IN**: POLE+O 엔티티/관계 스키마, Temporal fact struct, `IdentityGraph` 인터페이스, `KuzuBackend` 기본 구현, `Neo4jBackend` 어댑터(옵션), 엔티티 추출 파이프라인(NER + 규칙), SHACL 서브셋 검증, 그래프 쿼리 API(`GraphQuery`), JSON-LD export, 스냅샷/복원.
- **OUT**: Vector embedding 연산(→ VECTOR-001), LoRA 훈련 데이터 구성(→ LORA-001), 5단계 승격/롤백(→ REFLECT-001/ROLLBACK-001), Frozen-path 검증(→ SAFETY-001), Federated 공유(→ Phase 7+ privacy), 외부 지식베이스(Wikipedia/DBpedia) 연동, 다국어 NER 모델 fine-tuning.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/learning/identity/` 패키지 생성.
2. POLE+O 엔티티 타입 정의: `Person`, `Organization`, `Location`, `Event`, `Object` (Go `EntityKind` enum + 공통 `Entity` 구조체).
3. 관계(predicate) 어휘 초기 목록: `KNOWS`, `WORKS_AT`, `REPORTS_TO`, `MEMBER_OF`, `LIVES_AT`, `WORKS_AT_LOC`, `FREQUENTS`, `CELEBRATES`, `ATTENDS`, `PLANS`, `LIKES`, `DISLIKES`, `OWNS`, `WORKS_ON`, `PREFERS` (YAML 파일 `internal/learning/identity/predicates.yaml`, 사용자 확장 가능).
4. `TemporalFact` 구조체: (subject_id, predicate, object_id, valid_from, valid_until?, episode_id, confidence, source).
5. `IdentityGraph` 인터페이스 — AssertFact, InvalidateFact, Query(cypher), GetEntity, PutEntity, ListFacts(entityID, asOf), Snapshot, Restore, Close.
6. `KuzuBackend` 기본 구현: 임베디드 DB 파일 `$GENIE_HOME/identity/graph.kuzu`.
7. `Neo4jBackend` 어댑터: `--identity.backend=neo4j` 설정 시, bolt URL 필요. 본 SPEC은 인터페이스 동등성만 보장.
8. 엔티티 추출 파이프라인 `Extractor`: 입력은 `[]Interaction`(SPEC-GENIE-TRAJECTORY-001 결과), 출력은 `[]ExtractedEntity` + `[]ExtractedRelation`. NER 기본(rule-based+regex) + LLM-assisted(선택).
9. SHACL 서브셋 검증기 `SHACLValidator`: subject class, object class, cardinality (min/max), inverseOf 체크. Fact 저장 전 호출.
10. `GraphQuery` 추상화: Cypher 쿼리 문자열을 backend-agnostic하게 실행. 결과는 `[]map[string]any` 행.
11. JSON-LD export `ExportJSONLD(userID)`: 사용자가 자신의 데이터를 export 할 수 있도록 (adaptation.md §11.3 준수).
12. 스냅샷/복원: `Snapshot(path) error`, `Restore(path) error`. 롤백 가능성(ROLLBACK-001과 연계) 보장.

### 3.2 OUT OF SCOPE (명시적 제외)

- **Vector embedding 계산**: 본 SPEC의 `Entity` 구조체에 `embedding []float32` 필드는 존재하나, **값 계산과 저장 정책**은 VECTOR-001의 책임. 본 SPEC은 "opaque blob"으로만 저장.
- **LLM 기반 entity linking**: 동명이인(“김과장” 여러 명)의 disambiguation은 본 SPEC의 **heuristic만** 지원 (정확한 동명이인 해소는 차기 SPEC).
- **다국어 NER 모델**: 한국어/영어 rule-based만 기본 제공. 일본어·중국어는 사용자 설정으로 `Extractor` 인터페이스 구현체 교체.
- **Frozen-path 보호**: `learning-engine.md`·`user.yaml` 같은 frozen 파일 수정 시도는 SAFETY-001이 차단. 본 SPEC은 graph 내부 구조에만 책임.
- **외부 지식 주입**: Wikipedia/DBpedia 연결, 소셜그래프 import 등은 본 SPEC에서 명시적으로 배제.
- **정합성 제약 자동 수정**: SHACL 위반 사실은 **거부하고 에러 반환**. 자동 수정(infer correct class 등)은 하지 않음.

---

## 4. 목표 (Goals)

- 사용자 1명의 Identity Graph 초기 생성 비용이 100ms 이내 (Kuzu 파일 초기화 포함).
- `AssertFact` p95 레이턴시 3ms 이하 (Kuzu 임베디드, 로컬 SSD).
- SHACL 위반 fact는 저장되지 않는다 (0건 수용).
- 사용자는 자신의 Identity Graph를 **단일 JSON-LD 파일**로 export 할 수 있다.
- 스냅샷 → 7일 이내 임의의 지점으로 복원 가능 (ROLLBACK-001의 30일 정책 준수, 본 SPEC은 복원 API 보장).
- Kuzu ↔ Neo4j 백엔드 전환 시 인터페이스 동등성 100% (동일 테스트 스위트가 두 백엔드 모두에서 통과).

## 5. 비목표 (Non-Goals)

- 100만+ 엔티티 스케일 최적화 (본 SPEC은 단일 사용자 1만 엔티티 수준 가정).
- 분산 그래프 sharding.
- Cross-user graph join (개인정보 격리가 우선).
- 실시간 graph streaming / change-data-capture.

---

## 6. 요구사항 (EARS Requirements)

### REQ-IDENTITY-001 [Ubiquitous]
The Identity Graph service shall expose the `IdentityGraph` Go interface with method set: `AssertFact`, `InvalidateFact`, `Query`, `GetEntity`, `PutEntity`, `ListFacts`, `Snapshot`, `Restore`, `Close`.

### REQ-IDENTITY-002 [Ubiquitous]
The Identity Graph service shall support exactly five entity kinds: `Person`, `Organization`, `Location`, `Event`, `Object`. Any other kind shall be rejected at `PutEntity` with `ErrInvalidEntityKind`.

### REQ-IDENTITY-003 [Ubiquitous]
Every fact stored in the Identity Graph shall carry a `valid_from` timestamp. `valid_until` is optional; a nil value means "currently valid".

### REQ-IDENTITY-004 [Event-Driven]
When `AssertFact(subject, predicate, object, validFrom, validUntil)` is called, the service shall validate the tuple against the SHACL subset (subject class, object class, cardinality, inverseOf) **before** writing. Validation failure shall return `ErrSHACLViolation` and leave the graph unchanged.

### REQ-IDENTITY-005 [Event-Driven]
When `AssertFact` is called with a predicate that already has an active (valid_until = nil) fact for the same subject and the predicate's `maxCardinality = 1`, the service shall close the previous fact by setting its `valid_until` to the new fact's `valid_from - 1ns` (atomic within a transaction).

### REQ-IDENTITY-006 [State-Driven]
While the configuration value `identity.backend` is `kuzu` (default), all persistence operations shall be executed against a local Kuzu database at `$GENIE_HOME/identity/graph.kuzu`.

### REQ-IDENTITY-007 [State-Driven]
While the configuration value `identity.backend` is `neo4j`, the service shall establish a bolt connection using `identity.neo4j.uri` and delegate to the Neo4j driver; all interface contracts (REQ-IDENTITY-001..005) shall remain identical.

### REQ-IDENTITY-008 [Event-Driven]
When the `Extractor.Extract(interactions)` method is called, the service shall return `([]ExtractedEntity, []ExtractedRelation)`, where each extracted entity carries a provisional `confidence` in `[0.0, 1.0]`.

### REQ-IDENTITY-009 [Event-Driven]
When `Query(cypher, params)` is invoked, the service shall execute the query via the active backend and return `[]map[string]any` rows; read-only queries shall not require a transaction handle; write queries shall reject via `ErrWriteQueryNotAllowed` (use `AssertFact`/`InvalidateFact` instead).

### REQ-IDENTITY-010 [Event-Driven]
When `ListFacts(entityID, asOf)` is called, the service shall return only facts where `valid_from <= asOf` and (`valid_until` is nil or `valid_until > asOf`).

### REQ-IDENTITY-011 [Event-Driven]
When `ExportJSONLD(userID)` is called, the service shall emit a valid JSON-LD 1.1 document containing all entities and facts reachable from the user node; the emitted `@context` shall reference a project-local vocabulary URI (`urn:genie:identity:v1`).

### REQ-IDENTITY-012 [Event-Driven]
When `Snapshot(path)` is called, the service shall create a consistent point-in-time copy of the graph suitable for `Restore(path)`; Kuzu backend shall use `EXPORT DATABASE` semantics; Neo4j backend shall use `apoc.export.cypher.all`.

### REQ-IDENTITY-013 [Unwanted]
The Identity Graph service shall not mutate or delete facts whose `valid_until` is strictly before `time.Now() - 30 days`. Such facts are archive-only and shall be retained unless `Restore(path)` replaces them wholesale.

### REQ-IDENTITY-014 [Unwanted]
The Identity Graph service shall not expose a write API that accepts raw Cypher; all writes shall flow through `AssertFact` / `InvalidateFact` / `PutEntity`.

### REQ-IDENTITY-015 [Optional]
Where the configuration value `identity.extractor.llm_assisted = true`, the `Extractor` implementation may additionally invoke an LLM (via SPEC-GENIE-ROUTER-001) to resolve ambiguous entity mentions; the LLM response must still pass `SHACLValidator` before persistence.

### REQ-IDENTITY-016 [Optional]
Where the predicate definition in `predicates.yaml` declares an `inverseOf` relation, `AssertFact` may additionally insert the inverse fact in the same transaction; inverse insertion failure due to SHACL shall abort the entire transaction.

### REQ-IDENTITY-017 [Complex]
While a snapshot is in progress, when `AssertFact` is called concurrently, the service shall queue the write until the snapshot completes; the queue depth shall be bounded (default 1024) and overflow shall return `ErrSnapshotInProgress`.

### REQ-IDENTITY-018 [Ubiquitous]
All persistence errors (backend I/O failure, connection loss) shall be wrapped in `ErrBackendUnavailable` and the caller shall receive a non-nil error; the service shall not return success on silent-drop.

---

## 7. 설계 결정 (Design Decisions)

### DD-IDENTITY-01 — POLE+O 엔티티 5종 고정

5종으로 고정한 이유: Graphiti·Zep·업계 ontology 연구가 수렴한 핵심 카테고리. 확장이 필요하면 `Object` 하위에 사용자 정의 `ObjectSubtype` 태그로 해결하고, 6번째 kind는 추가하지 않는다. 이는 스키마 drift를 막기 위한 **hard constraint**.

### DD-IDENTITY-02 — Kuzu 기본, Neo4j 옵션

로컬 우선 프라이버시(learning-engine.md §7.1)를 지키려면 임베디드 DB가 필수. Kuzu는 Cypher 호환, 단일 바이너리, ACID, 10MB 수준 리소스. Neo4j는 **협업 전용 옵션**으로만 열어두며, 선택 시 사용자에게 "데이터가 로컬을 벗어남" 고지 UI가 필수 (CLI-001에서 확인).

### DD-IDENTITY-03 — SHACL 서브셋만

풀 SHACL은 복잡도 대비 효용 낮음. 본 SPEC은 (class, datatype, min/maxCount, inverseOf) 4요소만 구현. 충분히 표현력 있으면서도 validator 구현이 1~2일 내에 끝나는 수준.

### DD-IDENTITY-04 — 쓰기 API는 Cypher 금지

raw Cypher 쓰기를 열면 SHACL bypass가 가능하고 프라이버시 감사가 어려워진다. 모든 쓰기는 `AssertFact`/`InvalidateFact` 경유 → 감사 로그가 일원화된다.

### DD-IDENTITY-05 — 엔티티 추출은 두 단계

기본은 regex+NER 기반(로컬, LLM 없음). `llm_assisted=true` 설정 시 LLM 호출. 개인정보가 LLM 제공자로 전송되는 것을 기본 OFF로 두는 것이 learning-engine.md §7 프라이버시 약속의 귀결.

### DD-IDENTITY-06 — Temporal invariant: 최신 사실만 valid_until=nil

같은 (subject, predicate, object) 쌍이라도 서로 다른 시점의 truth가 가능 (예: "삼성에서 일한다" → 2024-01-15 ~ nil, 재취업 시 close). maxCardinality=1 predicate는 새 사실 삽입 시 자동 close (REQ-IDENTITY-005).

### DD-IDENTITY-07 — JSON-LD는 Right-to-Portability

GDPR Article 20 (data portability). JSON-LD는 표준 포맷 + 의미 보존 가능. 별도 proprietary format 사용하지 않는다.

---

## 8. 데이터 모델 (Data Model)

```go
// internal/learning/identity/types.go

// EntityKind — POLE+O 5종 고정
type EntityKind int

const (
    EntityPerson EntityKind = iota
    EntityOrganization
    EntityLocation
    EntityEvent
    EntityObject
)

// Entity — 5종 공통 구조
type Entity struct {
    ID         string            // UUID v7 권장 (시간정렬 + 유일)
    Kind       EntityKind
    Name       string            // 사람의 이름, 조직명 등
    Aliases    []string          // "김과장", "Kim Manager"
    Attributes map[string]any    // kind 별 추가 속성 (예: Person.age_bracket)
    Embedding  []float32         // VECTOR-001이 채움, 본 SPEC은 opaque blob
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// TemporalFact — 모든 관계는 시간 범위를 가진다
type TemporalFact struct {
    ID         string            // UUID v7
    SubjectID  string            // Entity.ID
    Predicate  string            // e.g., "WORKS_AT"
    ObjectID   string            // Entity.ID
    ValidFrom  time.Time         // 필수
    ValidUntil *time.Time        // nil = currently valid
    EpisodeID  string            // Trajectory episode 참조 (SPEC-GENIE-TRAJECTORY-001)
    Confidence float64           // [0, 1]
    Source     FactSource        // user_statement | inference | fact_extraction | llm_assisted
}

type FactSource string

const (
    SourceUserStatement  FactSource = "user_statement"
    SourceInference      FactSource = "inference"
    SourceFactExtraction FactSource = "fact_extraction"
    SourceLLMAssisted    FactSource = "llm_assisted"
)

// PredicateDefinition — predicates.yaml 에서 로드
type PredicateDefinition struct {
    Name           string       `yaml:"name"`            // "WORKS_AT"
    SubjectClass   EntityKind   `yaml:"subject_class"`   // EntityPerson
    ObjectClass    EntityKind   `yaml:"object_class"`    // EntityOrganization
    MinCardinality int          `yaml:"min_cardinality"` // 0 또는 1
    MaxCardinality int          `yaml:"max_cardinality"` // 1 (-1 = unbounded)
    InverseOf      string       `yaml:"inverse_of"`      // "EMPLOYS"
    Description    string       `yaml:"description"`
}

// IdentityGraph — 외부 사용자가 보는 유일한 인터페이스
type IdentityGraph interface {
    // 쓰기
    PutEntity(ctx context.Context, e Entity) error
    AssertFact(ctx context.Context, f TemporalFact) error
    InvalidateFact(ctx context.Context, factID string, at time.Time) error

    // 읽기
    GetEntity(ctx context.Context, id string) (Entity, error)
    ListFacts(ctx context.Context, entityID string, asOf time.Time) ([]TemporalFact, error)
    Query(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)

    // 스냅샷/복원
    Snapshot(ctx context.Context, path string) error
    Restore(ctx context.Context, path string) error

    // Export
    ExportJSONLD(ctx context.Context, userID string) ([]byte, error)

    // 생명주기
    Close(ctx context.Context) error
}

// Extractor — 상호작용에서 엔티티·관계 추출
type Extractor interface {
    Extract(ctx context.Context, interactions []trajectory.Interaction) ([]ExtractedEntity, []ExtractedRelation, error)
}

type ExtractedEntity struct {
    Kind       EntityKind
    Name       string
    Mentions   []TextSpan        // 원문 위치 (감사 로그)
    Confidence float64
}

type ExtractedRelation struct {
    SubjectName string
    Predicate   string
    ObjectName  string
    ValidFrom   time.Time
    Confidence  float64
    Source      FactSource
}

type TextSpan struct {
    EpisodeID string
    Start     int
    End       int
}

// SHACLValidator — 서브셋 검증기
type SHACLValidator interface {
    ValidateFact(f TemporalFact, subjectKind, objectKind EntityKind, existing []TemporalFact) error
}
```

### 8.1 `predicates.yaml` 초기 엔트리 예시

```yaml
- name: WORKS_AT
  subject_class: person
  object_class: organization
  min_cardinality: 0
  max_cardinality: -1        # 동시에 여러 조직 근무 가능
  inverse_of: EMPLOYS
  description: "주어(사람)가 목적어(조직)에 소속되어 근무 중"

- name: REPORTS_TO
  subject_class: person
  object_class: person
  min_cardinality: 0
  max_cardinality: 1         # 한 시점에 한 명의 상급자
  inverse_of: MANAGES
  description: "주어가 목적어에게 업무 보고"

- name: CELEBRATES
  subject_class: person
  object_class: event
  min_cardinality: 0
  max_cardinality: -1
  description: "주어가 목적어(생일·기념일)를 기린다"
```

---

## 9. API/인터페이스 (Public Surface)

패키지 `internal/learning/identity`는 다음 심볼만 공개한다:

- `EntityKind`, `Entity`, `TemporalFact`, `PredicateDefinition`, `FactSource`, `TextSpan`, `ExtractedEntity`, `ExtractedRelation`
- `IdentityGraph` 인터페이스
- `Extractor` 인터페이스 + `NewRuleBasedExtractor()` 생성자
- `SHACLValidator` 인터페이스 + `NewDefaultValidator(preds []PredicateDefinition)` 생성자
- `NewKuzuBackend(path string) (IdentityGraph, error)`
- `NewNeo4jBackend(cfg Neo4jConfig) (IdentityGraph, error)`
- `LoadPredicates(path string) ([]PredicateDefinition, error)`

에러 심볼: `ErrInvalidEntityKind`, `ErrSHACLViolation`, `ErrWriteQueryNotAllowed`, `ErrBackendUnavailable`, `ErrSnapshotInProgress`, `ErrPredicateUnknown`.

---

## 10. Exclusions (What NOT to Build)

- LoRA 훈련 데이터 빌더 (→ LORA-001 `DatasetBuilder`).
- Preference vector 계산·업데이트 (→ VECTOR-001).
- 승격 파이프라인·냉각 기간 관리 (→ REFLECT-001/ROLLBACK-001).
- Frozen-path enforcement, rate limiter, approval flow (→ SAFETY-001).
- A2A 외부 에이전트와의 graph 공유 (→ A2A-001).
- ML 기반 동명이인 해소(고급).
- Voice biometric / family mode 구현 (adaptation.md §9.2).
- 외부 KG 연결 (Wikipedia, DBpedia, Wikidata).

---

## 11. Acceptance Criteria

Acceptance 상세는 `acceptance.md` 부재 시 본 섹션이 단일 출처이다. research.md에서 확장 시나리오를 제공한다.

### AC-IDENTITY-001 — 인터페이스 계약
Given `NewKuzuBackend(tmpDir)` 로 서비스를 초기화한 뒤, When `PutEntity(Entity{Kind: EntityPerson, Name: "Goos"})` → `AssertFact(...)` → `GetEntity(...)` → `ListFacts(...)` 를 순차 호출하면, Then 모든 호출이 nil 에러를 반환하고 조회 결과가 입력과 일치한다.

### AC-IDENTITY-002 — POLE+O 5종 고정
When `PutEntity` 에 `EntityKind(99)` 값을 전달하면, Then 즉시 `ErrInvalidEntityKind` 가 반환되고 그래프 상태는 변하지 않는다.

### AC-IDENTITY-003 — SHACL 위반 차단
Given predicate `WORKS_AT`(subject=Person, object=Organization) 이 로드되고, When `AssertFact(Person→WORKS_AT→Person)` 를 호출하면, Then `ErrSHACLViolation` 이 반환되고 해당 fact 는 저장되지 않는다.

### AC-IDENTITY-004 — maxCardinality=1 자동 close
Given predicate `REPORTS_TO`(maxCardinality=1), When 동일 subject 에 대해 두 번째 `AssertFact` 를 호출하면, Then 첫 번째 fact 의 `valid_until` 이 두 번째 fact 의 `valid_from - 1ns` 로 자동 설정되고, 동시에 `ListFacts(entity, now)` 는 두 번째 fact 만 반환한다.

### AC-IDENTITY-005 — Temporal 쿼리 정확성
Given 하나의 subject 에 대해 `WORKS_AT` 관계가 2023-01-01~2024-12-31 (close) 과 2025-01-01~nil (open) 두 건 존재할 때, When `ListFacts(subjectID, 2024-06-15)` 를 호출하면, Then 첫 번째 fact 만 반환되고 두 번째는 제외된다.

### AC-IDENTITY-006 — 쓰기 Cypher 차단
When `Query("CREATE (n:Person {name:'x'})", nil)` 를 호출하면, Then `ErrWriteQueryNotAllowed` 가 반환되고 그래프 상태는 변하지 않는다.

### AC-IDENTITY-007 — Kuzu↔Neo4j 인터페이스 동등성
동일 테스트 스위트(ACIDS-001..006)가 `NewKuzuBackend` 와 `NewNeo4jBackend`(테스트 컨테이너) 두 백엔드 모두에서 100% 통과한다.

### AC-IDENTITY-008 — 엔티티 추출 신뢰도
Given 상호작용 `"우리 팀장 김과장이 삼성에서 다음주 회의 소집했어"` 를 `NewRuleBasedExtractor().Extract(...)` 에 전달할 때, Then 최소 3 엔티티(Person: 김과장, Organization: 삼성, Event: 다음주 회의) 가 반환되고 각 엔티티의 `Confidence >= 0.5` 이다.

### AC-IDENTITY-009 — 스냅샷/복원 라운드트립
Given 10 entity + 20 fact 가 저장된 그래프에서, When `Snapshot("/tmp/s1.kuzu")` → `Restore("/tmp/s1.kuzu")` 를 수행하면, Then `ExportJSONLD(userID)` 결과가 스냅샷 전후로 byte-level 동일하다.

### AC-IDENTITY-010 — JSON-LD 유효성
When `ExportJSONLD(userID)` 를 호출하면, Then 반환된 바이트열이 유효한 JSON-LD 1.1 문서(`@context` 존재, 모든 엔티티가 `@id`/`@type` 를 가짐)이다. 외부 JSON-LD playground(로컬 goldenfile) 와 매칭된다.

### AC-IDENTITY-011 — 에러 비은폐
Given Kuzu DB 파일이 읽기전용으로 마운트되어 쓰기 실패 상태일 때, When `AssertFact` 를 호출하면, Then `ErrBackendUnavailable` 로 래핑된 에러가 반환되고 호출자는 성공으로 오인하지 않는다.

### AC-IDENTITY-012 — 30일 이상된 fact 불변
Given `valid_until = time.Now() - 31 days` 인 과거 fact 가 있을 때, When `InvalidateFact(oldID, now)` 를 호출하면, Then `ErrArchivedFact` 가 반환되고 그 fact 는 수정되지 않는다.

---

## 12. 테스트 전략 (요약, 상세는 research.md)

- 단위: `predicates_test.go`(YAML 로드), `shacl_test.go`(위반 12종), `temporal_test.go`(valid_until 계산).
- 통합(Kuzu): `kuzu_backend_test.go` 임시 디렉터리, race 모드.
- 통합(Neo4j): `neo4j_backend_test.go` dockertest 기반, `-tags=integration`.
- 계약: 동일 `graph_contract_test.go` 를 두 백엔드 모두에 적용.
- 골든파일: JSON-LD export 바이트열 매칭.
- 커버리지: 85%+ (quality.yaml 기본값 유지).

---

## 13. 의존성 & 영향 (Dependencies & Impact)

- **상위 의존**: MEMORY-001(episode_id 참조), SAFETY-001(frozen-path 보호 호출 경로).
- **하위 소비자**: VECTOR-001(엔티티 embedding), LORA-001(훈련 데이터 엔티티 인용), proactive engine(Phase 6+), A2A-001(외부 에이전트에게 Agent Card 표시 시 graph 발췌).
- **CLI 영향**: `genie identity dump`, `genie identity export --format=jsonld`, `genie identity snapshot <path>` 세 명령 추가 (CLI-001 후속 PR).

---

## 14. 오픈 이슈 (Open Issues)

1. **Kuzu 바인딩 Go 1.26 호환성**: `github.com/kuzu-db/kuzu-go` 마지막 릴리즈(2026-02)가 Go 1.25로 빌드됨. 1.26+ 빌드 가능 여부 CI 확인 필요.
2. **NER 모델 라이선스**: rule-based 기본 구현은 의존 없음. LLM-assisted 모드의 프롬프트 템플릿은 예시 저장소에 commit할지 별도 플러그인으로 둘지 결정.
3. **Cypher 서브셋 호환성**: Kuzu 와 Neo4j 가 지원하는 Cypher 서브셋이 미묘하게 다름(e.g., `WITH` 옵션). `GraphQuery` 가 양쪽 모두에서 동일하게 동작하는 **Safe Cypher** 서브셋을 DSL 로 정리할지 여부.
4. **PROF(profile) 속성의 Attribute 키**: `Entity.Attributes` 키 네이밍 convention. snake_case vs camelCase. 기본은 snake_case.
5. **Embedding 저장 전략**: `Entity.Embedding` 을 Kuzu list 타입으로 저장할지, 별도 sidecar(VECTOR-001의 Qdrant)에 위임할지. 후자가 분리 원칙상 권장되나 join 비용이 증가.
