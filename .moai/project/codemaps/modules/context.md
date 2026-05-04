# context 패키지 — Token Budget Management

**위치**: internal/context/  
**파일**: 20개 (budget.go, adapter.go, compactor.go, summarizer.go)  
**상태**: ✅ Active (SPEC-GOOSE-CONTEXT-001)

---

## 목적

토큰 예산 추적: 입력/출력 개수, 비용, 압축 전략.

---

## 공개 API

### ContextAdapter
```go
type ContextAdapter struct {
    tokenBudget int64
    used        int64
    cost        float64
}

func (ca *ContextAdapter) Remaining() int64
    // Return: budget - used

func (ca *ContextAdapter) Estimate(text string) int64
    // Estimate tokens for text (tiktoken approximation)

func (ca *ContextAdapter) CanAfford(estimate int64) bool
    // Check: used + estimate <= budget
```

### Compaction
```go
type Compactor struct {
    // Strategy: summarize old messages to recover tokens
}

func (c *Compactor) Compact(messages []Message, targetTokens int64) ([]Message, error)
    // 1. Group by time window
    // 2. Summarize each group
    // 3. Keep recent messages intact
    // 4. Return reduced message list
```

---

## Budget Policy

```
Session budget: 100,000 tokens (configurable)

Usage:
  - Input messages: counted
  - LLM API calls: counted  
  - Tool execution: not counted (external)
  - Internal: not counted

When exhausted:
  - Agent gets error: BudgetExhausted
  - Graceful fallback: summarize history
  - User notified: "Context full"
```

---

**Version**: context v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~240
