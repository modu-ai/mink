# Hermes Agent Multi-LLM Credential Pool & Routing 심층 분석

> **분석일**: 2026-04-21 · **대상**: `hermes-agent-main/agent/` · **용도**: SPEC-GOOSE-CREDPOOL-001 / ROUTER-001 / RATELIMIT-001 / PROMPT-CACHE-001 / COMPRESSOR-001 근거

## 1. Credential Pool 아키텍처

```
┌─────────────────────────────────────────────────┐
│ CredentialPool(provider, entries)               │
└─────────────────────────────────────────────────┘
          │
    ┌─────┴──────┐
    ▼            ▼              ▼
  [OK] ◄────[EXHAUSTED]    [PENDING]
               │                │
    [clear_expired]      [_refresh_entry]
    (1시간 cooldown)       (OAuth token)
```

**Selection Strategies** (4종):
- FILL_FIRST (우선순위 순서)
- ROUND_ROBIN (공평)
- RANDOM (부하 분산)
- LEAST_USED (사용 빈도 기반)

**흐름**:
1. select: STRATEGY 기반 + availability filtering + refresh
2. refresh: OAuth entry 자동 갱신 (expiring 감지)
3. Exhaustion: HTTP 429/402 → STATUS_EXHAUSTED + cooldown 기록
4. Rotation: 현재 entry 소진 → 다음 가용 entry
5. Persistence: JSON 상태 저장

**Storage 위치**:
- Anthropic Claude Code: `~/.claude/.credentials.json`
- OpenAI Codex: `~/.codex/auth.json`
- Nous: `~/.hermes/auth.json`

## 2. 지원 LLM 프로바이더 매트릭스 (15+)

| Provider | 인증 | 출처 | Stream | Tools | Vision | Embed | 특기 |
|---|---|---|---|---|---|---|---|
| **Anthropic** | OAuth/API | Pool | ✅ | ✅ | ✅ | ❌ | Prompt Caching (4 breakpoint) |
| **OpenAI (Codex)** | OAuth | Pool | ✅ | ✅ | ✅ | ✅ | Refresh token rotation |
| **OpenRouter** | API | Pool | ✅ | ✅ | ✅ | ✅ | 100+ 모델 aggregator |
| **Nous Portal** | OAuth | auth.json | ✅ | ✅ | ✅ | ❌ | Agent Key refresh (TTL) |
| **Google Gemini** | API | env | ✅ | ✅ | ✅ | ✅ | GOOGLE_API_KEY |
| **xAI Grok** | API | env | ✅ | ✅ | ✅ | ❌ | OpenAI-compatible |
| **DeepSeek** | API | env | ✅ | ✅ | ❌ | ❌ | cost-effective |
| **Ollama** | Local | CLI | ✅ | ✅ | ✅ | ✅ | localhost:11434 |
| **Mistral** | API | env | ✅ | ✅ | ❌ | ✅ | MISTRAL_API_KEY |
| **Groq** | API | env | ✅ | ✅ | ❌ | ❌ | Fastest (~500 RPM) |
| **Qwen** | OAuth | local | ✅ | ✅ | ✅ | ✅ | ~/.qwen/oauth_creds.json |
| **Kimi/Moonshot** | API | env | ✅ | ✅ | ✅ | ❌ | 코딩 특화 |
| **GLM (z.ai)** | API | env | ✅ | ✅ | ✅ | ❌ | ZhipuAI OpenAI-compat |
| **MiniMax** | API | env | ✅ | ✅ | ✅ | ❌ | 글로벌 + CN |
| **Custom** | API | config | ✅ | ✅ | ✅ | ✅ | custom_providers[] |

## 3. 핵심 Python 인터페이스

### 3.1 PooledCredential

```python
@dataclass
class PooledCredential:
    provider: str
    id: str                         # UUID hex[:6]
    label: str
    auth_type: str                  # "oauth" | "api_key"
    priority: int
    source: str
    access_token: str
    refresh_token: Optional[str]
    last_status: Optional[str]      # "ok" | "exhausted"
    last_status_at: Optional[float]
    last_error_code: Optional[int]  # HTTP status
    last_error_reset_at: Optional[float]
    base_url: Optional[str]
    expires_at: Optional[str]
    expires_at_ms: Optional[int]
    agent_key: Optional[str]        # Nous-specific
    request_count: int = 0
    extra: Dict[str, Any] = None

    @property
    def runtime_api_key(self) -> str  # agent_key 우선 fallback access_token
    @property
    def runtime_base_url(self) -> Optional[str]
```

### 3.2 CredentialPool

```python
class CredentialPool:
    def __init__(self, provider: str, entries: List[PooledCredential])
    
    def select(self) -> Optional[PooledCredential]
        # Strategy + availability + refresh
    
    def mark_exhausted_and_rotate(
        self, *, status_code, error_context
    ) -> Optional[PooledCredential]
    
    def _refresh_entry(self, entry, *, force) -> Optional[PooledCredential]
    
    def _available_entries(
        self, *, clear_expired=False, refresh=False
    ) -> List[PooledCredential]
    
    def acquire_lease(self, credential_id=None) -> Optional[str]
    def release_lease(self, credential_id: str) -> None
```

## 4. Smart Routing 의사코드

```python
def choose_cheap_model_route(user_message, routing_config):
    """간단 휴리스틱으로 저비용 모델 라우팅"""
    # 조건 (ALL true):
    if len(msg) <= 160 chars AND
       len(msg.split()) <= 28 words AND
       NOT has_code_block(msg) AND
       NOT has_url(msg) AND
       NOT has_complex_keyword(msg):
        return cheap_model_route
    return None

# Complex keywords (제외): 
_COMPLEX_KEYWORDS = {
    "debug", "implement", "refactor", "test", "analyze",
    "design", "architecture", "terminal", "docker", ...
}

# Route 결과:
ROUTE = {
    "model": str,
    "provider": str,
    "base_url": Optional[str],
    "routing_reason": "simple_turn" | "complex_task",
    "signature": (model, provider, base_url, mode, command, args)
}
```

**Conservative by design**: 코드/디버그/URL 감지 시 primary 유지.

## 5. Rate Limit Tracker

```python
@dataclass
class RateLimitBucket:
    limit: int = 0
    remaining: int = 0
    reset_seconds: float = 0.0
    captured_at: float = 0.0
    
    @property
    def used(self) -> int: return max(0, self.limit - self.remaining)
    @property
    def usage_pct(self) -> float: return (self.used / self.limit) * 100.0
    @property
    def remaining_seconds_now(self) -> float:
        elapsed = time.time() - self.captured_at
        return max(0.0, self.reset_seconds - elapsed)

@dataclass
class RateLimitState:
    requests_min: RateLimitBucket
    requests_hour: RateLimitBucket
    tokens_min: RateLimitBucket
    tokens_hour: RateLimitBucket
    captured_at: float
    provider: str
```

**수집**: `x-ratelimit-limit-{requests,tokens}`, `x-ratelimit-remaining-*`, `x-ratelimit-reset-*` 헤더 (OpenAI, Anthropic, OpenRouter, Nous Portal 호환).

**경고**: 80% 사용률 도달 시 리셋 시간과 함께 표시.

## 6. Prompt Caching: system_and_3 Strategy

```
Messages:
  [0] System Prompt ← [cache_control: ephemeral] (1/4)
  [1] User
  [2] Assistant
  [3] User
  [4] Tool Results
  [5] Assistant
  [6] User           ← [cache_control] (2/4)
  [7] Assistant      ← [cache_control] (3/4)
                        (4/4 slot unused)
```

**알고리즘**:
1. System prompt에 cache marker (1/4)
2. 최대 4 breakpoint (Anthropic 제한)
3. Non-system 메시지 중 마지막 3개에 cache marker
4. 타입: `{"type": "ephemeral"}` (5분) 또는 `{"type": "ephemeral", "ttl": "1h"}`

**효과**: Multi-turn에서 입력 토큰 비용 ~75% 절감.

## 7. Context Compressor (4 Phase)

```
Long conversation (100K tokens)
  ├─ Phase 1: Preflight check (estimate_messages_tokens_rough)
  │   IF tokens >= threshold_tokens (50% context_length) → compress
  │
  ├─ Phase 2: Tool Result Pruning (cheap, no LLM)
  │   └─ Old tool output → "[Old tool output cleared...]"
  │
  ├─ Phase 3: Head/Tail Protection
  │   ├─ Head: first N messages (기본 3)
  │   └─ Tail: token budget 기반 (기본 20% threshold)
  │
  ├─ Phase 4: LLM Summarization
  │   ├─ Target: _SUMMARY_RATIO * threshold (기본 20%)
  │   ├─ Budget: _SUMMARY_TOKENS_CEILING (12K max)
  │   ├─ Template: Goal, Progress, Decisions, Files, Next Steps
  │   └─ Iterative: 이전 요약 보존 + 증분 업데이트
  │
  └─ Phase 5: Replacement
      └─ [CONTEXT COMPACTION] + summary + remaining messages
```

**트리거**:
```python
should_compress(prompt_tokens) IF prompt_tokens >= threshold_tokens
should_compress_preflight(messages) IF estimate_tokens_rough(messages) >= threshold
```

**실패 보호**: 600초 cooldown (LLM 실패 시).

## 8. Anthropic Adapter 특수 기능 (58KB)

**큰 이유**:
1. OAuth 관리 (PKCE 2.1 + single-use refresh token)
2. Token Sync (`~/.claude/.credentials.json`, `~/.hermes/.../hermes-oauth.json`)
3. Model Normalization (점, 버전, 별칭)
4. Tool Conversion (OpenAI → Anthropic schema)
5. Content Conversion (이미지, 코드, thinking 블록)
6. Thinking Mode (Adaptive thinking, o1-style)

**핵심 함수**:
```python
def read_claude_code_credentials() -> Optional[Dict]
def refresh_anthropic_oauth_pure(refresh_token, *, use_json) -> Dict
def resolve_anthropic_token() -> Optional[str]
def run_hermes_oauth_login_pure() -> Optional[Dict]
def build_anthropic_client(api_key, base_url=None)
def convert_messages_to_anthropic(messages) -> List
def convert_tools_to_anthropic(tools) -> List
def normalize_anthropic_response(response) -> Dict
def _supports_adaptive_thinking(model) -> bool
def _get_anthropic_max_output(model) -> int
```

## 9. GOOSE Go 포팅 매핑

```
internal/llm/
├── credential/
│   ├── pool.go           ← CredentialPool + PooledCredential
│   ├── oauth.go          ← OAuth 2.1 + PKCE
│   └── storage.go        ← Persistent storage
├── router/
│   ├── smart.go          ← Smart routing
│   └── selector.go       ← Strategy-based selection
├── ratelimit/
│   ├── tracker.go        ← RateLimitState + Bucket
│   ├── parser.go         ← Header parsing
│   └── display.go        ← Formatting
├── cache/
│   ├── prompt.go         ← system_and_3 caching
│   └── context.go        ← Context compression
├── provider/
│   ├── registry.go       ← Provider registry
│   ├── anthropic/{adapter, token, tools}.go
│   ├── openai/{oauth, adapter}.go
│   ├── google/gemini.go
│   ├── xai/grok.go
│   ├── deepseek/client.go
│   ├── ollama/local.go
│   ├── nous/{agent_key, portal}.go
│   ├── openrouter/client.go
│   ├── mistral/client.go
│   ├── groq/client.go
│   ├── qwen/oauth.go
│   ├── kimi/client.go
│   ├── glm/client.go
│   └── minimax/client.go
├── metrics/
│   ├── pricing.go        ← Usage + cost
│   └── usage.go
└── retry/
    └── backoff.go        ← Jittered exponential
```

## 10. GOOSE SPEC 도출

### SPEC-GOOSE-CREDPOOL-001
R1. Pool entry 선택 (4 strategy)
R2. OAuth auto-refresh (expiring 감지)
R3. Exhausted cooldown (HTTP 429/402)
R4. Soft lease (동시성)
R5. ~/ 폴더 자동 동기화 (Anthropic/OpenAI/Nous)

### SPEC-GOOSE-ROUTER-001
R1. 메시지 단순성 판정 (길이/단어/개행)
R2. 코드/URL/복잡 키워드 감지
R3. Primary ↔ Cheap 결정
R4. Route signature (trace용)

### SPEC-GOOSE-RATELIMIT-001
R1. x-ratelimit-* 헤더 파싱
R2. 4 bucket (requests_min/hour, tokens_min/hour)
R3. Usage % + 임계값(80%) 경고
R4. Human-readable display

### SPEC-GOOSE-PROMPT-CACHE-001
R1. system_and_3 strategy (max 4 breakpoints)
R2. TTL 설정 (5m default, 1h option)
R3. 다양한 메시지 포맷 지원

### SPEC-GOOSE-COMPRESSOR-001
R1. Preflight check (rough estimate)
R2. Tool result pruning (no LLM)
R3. Head/tail protection (token budget)
R4. Structured LLM summary (Goal/Progress/Decisions/Files/Next)
R5. Iterative summary updates

## 11. 재사용 vs 재작성 + Go LoC

| 모듈 | Python | 판정 | Go LoC 예상 |
|---|---|---|---|
| Credential Pool | 1.3K | ✅ 재사용 | 700 |
| Rate Limit Tracker | 243 | ✅ 재사용 | 150 |
| Prompt Caching | 73 | ✅ 재사용 | 100 |
| Usage Pricing | 500+ | 🔶 부분 | 300 |
| Retry Utils | 58 | ✅ 재사용 | 80 |
| Model Metadata | 1K | 🔶 부분 | 500 |
| Smart Routing | 160 | 🔶 부분 | 150 |
| Anthropic Adapter | 1.45K | 🔶 부분 | 800 |
| Auxiliary Client | 2.25K | ❌ 재작성 | 1200 |
| Context Compressor | 33KB | ❌ 재작성 | 2000 |
| Prompt Builder | 42KB | ❌ 재작성 | 2500 |
| **합계** | **~16,000** | **55%** | **~6,900** |

**포팅 기간**: 2-3개월 (팀 2명)

## 12. 고리스크

- Anthropic OAuth PKCE 재구현
- 여러 provider async client 통합 (Go concurrency 설계)
- Rate limit header 정규화 (provider별 차이)

---

**결론**: Hermes Multi-LLM = **엔터프라이즈급 credential management + 지능형 라우팅 + 자동 최적화** 3층 구조. GOOSE Go 포팅 **재사용 55%**, Credential Pool 원형 유지, anthropic-go/openai-go SDK 활용.
