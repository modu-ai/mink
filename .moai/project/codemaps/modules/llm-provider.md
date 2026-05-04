# llm 패키지 — LLM Provider Routing (6 프로바이더)

**위치**: internal/llm/, internal/llm/provider/  
**파일**: 16개 (provider/* + credential/ + ratelimit/)  
**상태**: ✅ Active (SPEC-GOOSE-LLM-001)

---

## 목적

6개 LLM 프로바이더 추상화 (Ollama, OpenAI, Claude, Google, etc). 비용 추적, Rate limit, Fallback 라우팅.

---

## 공개 API

### LLMProvider Interface
```go
type LLMProvider interface {
    // REQ-LLM-001: All methods must support context.Context
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (<-chan Chunk, error)
    
    // REQ-LLM-002: Model discovery
    Models() []Model
    
    // REQ-LLM-003: Cost tracking (opt-in)
    Cost(usage Usage) float64
}

type Request struct {
    Model      string
    Messages   []Message
    Tools      []Tool
    Temperature float32
    MaxTokens  int
}

type Response struct {
    Content   string
    ToolUses  []ToolUse
    Usage     Usage  // tokens, cost
}

type Chunk struct {
    Content string
    ToolUses []ToolUse
}
```

### Router
```go
type Router interface {
    // Select provider for model
    Route(model string) (LLMProvider, error)
    
    // Register provider
    Register(name string, provider LLMProvider) error
}

// REQ-LLM-004: Auto-fallback on provider error
func (r *router) Complete(ctx context.Context, model string, req Request) (Response, error)
    // 1. Route to primary provider
    // 2. On error, try fallback providers (list from config)
    // 3. Return first success or last error
```

---

## 6개 프로바이더 구현

### 1. Ollama (Local, Free)
```go
type OllamaProvider struct {
    baseURL string  // http://localhost:11434
    client  *http.Client
}

// Models(): ollama list (local models)
// Cost(): $0 (local inference)
// Benefits: Privacy, no API key, no rate limit
// Use case: Development, private deployments
```

### 2. OpenAI (GPT-4, o1)
```go
type OpenAIProvider struct {
    apiKey string
    client *openai.Client
}

// Models(): gpt-4, gpt-4o, o1, o1-mini
// Cost(): Per-token pricing (input/output)
// Features: Function calling, Vision, Structured output
// Rate limits: RPM, TPM (REQ-LLM-RATELIMIT-001)
```

### 3. Anthropic (Claude)
```go
type AnthropicProvider struct {
    apiKey string
    client *anthropic.Client
}

// Models(): claude-opus-4, claude-sonnet-4, claude-haiku-4
// Cost(): Per-token pricing
// Features: Extended thinking (opus), Prompt caching
// Rate limits: RPM, TPM
```

### 4. Google (Gemini)
```go
type GoogleProvider struct {
    apiKey string
    client *genai.Client
}

// Models(): gemini-2.0-flash, gemini-pro
// Cost(): Per-1M token pricing
// Features: Multimodal, Long context (1M tokens)
// Rate limits: RPM, QPM (queries per minute)
```

### 5. Groq (Fast Inference)
```go
type GroqProvider struct {
    apiKey string
    client *groq.Client
}

// Models(): mixtral-8x7b, llama2-70b
// Cost(): Cheaper than OpenAI
// Features: Low latency (<100ms)
// Use case: Real-time applications
```

### 6. Azure OpenAI (Enterprise)
```go
type AzureOpenAIProvider struct {
    endpoint string
    apiKey string
    deployment string
    client *openai.Client
}

// Models(): Custom deployments
// Cost(): Enterprise pricing
// Features: VNET integration, SOC 2
// Use case: Enterprise deployments
```

---

## Rate Limiting (REQ-LLM-RATELIMIT-001)

### 4-Bucket Strategy
```go
type RateLimiter struct {
    rpm int  // Requests per minute
    tpm int  // Tokens per minute
    rph int  // Requests per hour (daily cap)
    tph int  // Tokens per hour
}

func (r *RateLimiter) CheckLimits(tokens int) (canProceed bool, waitTime time.Duration)
    // Check all 4 buckets
    // Return first exceeded bucket wait time
    // 80% threshold → warning
    // 100% threshold → reject (return error)
```

### Header Parsing
```go
// Extract limits from provider response headers
func parseRateLimitHeaders(headers http.Header) RateLimitInfo
    // OpenAI: x-ratelimit-limit-requests, x-ratelimit-limit-tokens
    // Anthropic: anthropic-ratelimit-*
    // Google: quota, rateLimitInfo
```

---

## Credential Pool (REQ-CREDPOOL-001)

### Zero-Knowledge Storage
```go
type CredentialPool struct {
    vault      EncryptedVault  // AES-256
    rotation   AutoRotation    // 90 days
}

func (p *CredentialPool) Get(provider string) (credential string, error)
    // 1. Check vault (encrypted)
    // 2. Decrypt only when needed
    // 3. Audit log access
    // 4. Return never-logged plaintext

// @MX:WARN: Credential in memory (plaintext during call)
// @MX:REASON: Must decrypt for API call. Use context deadline.
```

### Rotation Policy
```
Every 90 days:
  1. Generate new API key (request from provider)
  2. Test new key (warm-up call)
  3. Rotate in vault (atomic)
  4. Audit log rotation event
  5. Keep old key (7 days fallback)
```

---

## Cost Tracking

### Per-Request
```go
type Usage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
    CostUSD      float64  // Calculated per provider
}

// OpenAI GPT-4:
//   Input: $0.03 / 1K tokens
//   Output: $0.06 / 1K tokens
cost := (inputTokens / 1000.0) * 0.03 + (outputTokens / 1000.0) * 0.06

// Ollama: cost = 0 (local)
```

### Budget Tracking
```go
type BudgetTracker struct {
    dailyLimitUSD  float64
    monthlyLimitUSD float64
    usedToday      float64
    usedThisMonth  float64
}

func (bt *BudgetTracker) CanAfford(estimatedCost float64) bool
    return (usedToday + estimatedCost) <= dailyLimitUSD &&
           (usedThisMonth + estimatedCost) <= monthlyLimitUSD
```

---

## Fallback Strategy

### Provider Selection
```go
func (r *router) Complete(ctx context.Context, model string, req Request) {
    // 1. Map model → preferred provider
    //    "gpt-4" → OpenAI
    //    "claude-*" → Anthropic
    //    "mixtral" → Groq
    
    // 2. Get fallback list (from config)
    //    primary: OpenAI
    //    fallback: [Groq, Ollama]
    
    // 3. Try each in order
    for provider := range [primary] + fallbacks {
        resp, err := provider.Complete(ctx, req)
        if err == nil {
            return resp
        }
        log.Warnf("Provider %s failed: %v, trying fallback", provider, err)
    }
    
    // 4. Return last error if all failed
    return nil, lastErr
}
```

### Error Recovery
```
Provider error types:
  1. Rate limit → wait + retry same provider
  2. Quota exceeded → skip to fallback
  3. Auth invalid → skip to fallback
  4. Network timeout → retry with backoff
  5. Model not found → try different model on same provider
```

---

## 동시성 안전성

### Provider Interface (Stateless)
```
Each method (Complete, Stream, Models, Cost) is:
  ✓ Thread-safe (no shared state)
  ✓ Concurrent callers OK
  ✗ Sequential within same request (respect context deadline)
```

### Router (Synchronized)
```go
type router struct {
    mu sync.RWMutex
    providers map[string]LLMProvider
    budgetTracker *BudgetTracker  // @MX:WARN: shared state
}

// @MX:WARN [AUTO] budgetTracker concurrent updates
// @MX:REASON: Multiple SubmitMessage → Complete calls
// Solution: atomic.Add or sync.Mutex on cost tracking
```

---

## @MX:ANCHOR 함수

| 함수 | 팬인 | 이유 |
|------|------|------|
| `Router.Route()` | 3+ | Agent, Tool, Test |
| `LLMProvider.Complete()` | Interface impl | All agents |
| `RateLimiter.CheckLimits()` | 2+ | Pre-call, middleware |
| `CostTracker.CanAfford()` | 2+ | Budget enforcement |

---

## SPEC 참조

| SPEC | 상태 |
|------|------|
| SPEC-GOOSE-LLM-001 | ✅ 6 providers |
| SPEC-CREDPOOL-001 | ✅ Zero-knowledge vault |
| SPEC-RATELIMIT-001 | ✅ 4-bucket tracking |

---

**Version**: llm v0.1.0  
**Providers**: 6 (Ollama, OpenAI, Claude, Google, Groq, Azure)  
**LOC**: ~280  
**Generated**: 2026-05-04
