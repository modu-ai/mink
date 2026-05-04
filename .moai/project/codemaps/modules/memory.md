# memory 패키지 — Multi-Backend Memory (SQLite FTS + Qdrant + Graphiti)

**위치**: internal/memory/  
**파일**: 19개 (provider.go, sqlite_fts.go, vector_store.go, graph_store.go, checkpoint.go, reranker.go)  
**상태**: ✅ Active (SPEC-GOOSE-MEMORY-001)

---

## 목적

다중 백엔드 메모리: 텍스트 검색 (SQLite FTS) + 벡터 검색 (Qdrant) + 그래프 (Graphiti Identity).

---

## 공개 API

### MemoryProvider Interface
```go
type MemoryProvider interface {
    // @MX:ANCHOR [AUTO] Core memory recall
    // @MX:REASON: Fan-in ≥3 (Agent, Learning, Query)
    Recall(ctx context.Context, query string) (ContextBlock, error)
    // Returns: relevant history + preference vector + identity edges
    
    Memorize(ctx context.Context, content string) error
    // Store interaction: parse entities, index text, embed vector
    
    Search(ctx context.Context, query string, limit int) ([]Result, error)
    // Full-text search (SQLite FTS)
    
    SearchVector(ctx context.Context, embedding []float32, limit int) ([]Result, error)
    // Vector search (Qdrant)
}

type ContextBlock struct {
    RecentMessages  []Message
    RelatedMemories []string
    PreferenceVector []float32
    IdentityFacts   []string  // From Graphiti
}
```

---

## 3-Backend Architecture

### 1. SQLite Full-Text Search
```go
type SQLiteFTS struct {
    db *sql.DB  // Schema: CREATE VIRTUAL TABLE memory USING fts5(...)
}

// Stores: text interactions, audit logs, user facts
// Indexes: word tokenization, phrase search
// Query: SELECT * FROM memory WHERE memory MATCH 'query'
```

### 2. Qdrant Vector Store
```go
type QdrantVectorStore struct {
    client *qdrant.Client
    collection string  // "user_preferences"
}

// Stores: preference embeddings (768-dim)
// Queries: Cosine similarity search
// Use: "Find similar interactions"
```

### 3. Graphiti Identity Graph
```go
type GraphitiGraphStore struct {
    client *graphiti.Client
}

// Stores: POLE+O entities (Person, Organization, Location, Event, Object)
// Indexes: User relationships, interaction history
// Queries: "Find contacts who mentioned X", "Temporal edges"
```

---

## Recall Flow

```
Agent.Memory.Recall(query)
  │
  ├─ Step 1: Parse query intent
  │   └─ Is query asking for facts vs. style vs. context?
  │
  ├─ Step 2: Hybrid search
  │   ├─ SQLite FTS: exact/phrase match
  │   ├─ Qdrant: semantic similarity
  │   └─ Graphiti: entity relationships
  │
  ├─ Step 3: Rerank results (LLM-based)
  │   └─ Score each result for relevance to current task
  │
  └─ Step 4: Return ContextBlock
      ├─ Top 5 recent messages
      ├─ Top 3 related memories
      ├─ Preference vector (embed query)
      └─ Identity facts (3+ entities)
```

---

## Memorize Flow

```
Agent.Memory.Memorize(interaction)
  │
  ├─ Step 1: Extract entities
  │   ├─ Named entity recognition
  │   ├─ User mentions, topics, tools used
  │   └─ Temporal anchors (day/time)
  │
  ├─ Step 2: Index text (SQLite FTS)
  │   └─ Tokenize, insert into memory table
  │
  ├─ Step 3: Embed vector (OpenAI/Ollama)
  │   └─ 768-dim embedding for semantic search
  │
  ├─ Step 4: Store vector (Qdrant)
  │   └─ upsert with payload (timestamp, entities)
  │
  └─ Step 5: Store graph (Graphiti)
      ├─ Add entities (POLE+O nodes)
      ├─ Add edges (interaction, temporal)
      └─ Link to user profile
```

---

## Checkpoint/Resume (Persistence)

```go
func (m *MemoryProvider) Checkpoint(ctx context.Context) error
    // 1. SQLite PRAGMA integrity_check
    // 2. Qdrant snapshot export
    // 3. Graphiti dump (JSON export)
    // 4. Save to ~/.goose/checkpoints/
    // 5. Audit log checkpoint event

func (m *MemoryProvider) Resume(ctx context.Context, path string) error
    // 1. Load SQLite from file
    // 2. Load Qdrant snapshot
    // 3. Restore Graphiti edges
    // 4. Verify checksums
```

---

## Reranker (LLM-based)

```go
type Reranker struct {
    llm llm.Provider
}

// Before returning search results:
// 1. Score each candidate: relevance to current task
// 2. Keep top 3-5 results
// 3. Return ordered list

// Uses LLM to understand context (not just embeddings)
// Slow but accurate (used only for Recall, not every query)
```

---

## SPEC 参考

| SPEC | 状态 |
|------|------|
| SPEC-MEMORY-001 | ✅ 3-backend design |
| SPEC-IDENTITY-001 | ✅ Graphiti integration |

---

**Version**: memory v0.1.0  
**Backends**: 3 (SQLite FTS, Qdrant, Graphiti)  
**LOC**: ~320  
**Generated**: 2026-05-04
