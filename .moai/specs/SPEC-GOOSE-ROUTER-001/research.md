# SPEC-GOOSE-ROUTER-001 — Research

> Hermes `model_router.py` smart routing 분석 + classifier heuristic 설계 + Provider Registry 15+ 엔트리.

## 1. Hermes 원형 분석 (hermes-llm.md §4 인용)

### 1.1 의사코드 인용

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
```

### 1.2 원문 키워드 set 인용

```python
_COMPLEX_KEYWORDS = {
    "debug", "implement", "refactor", "test", "analyze",
    "design", "architecture", "terminal", "docker", ...
}
```

### 1.3 Route 결과 구조 인용

```python
ROUTE = {
    "model": str,
    "provider": str,
    "base_url": Optional[str],
    "routing_reason": "simple_turn" | "complex_task",
    "signature": (model, provider, base_url, mode, command, args)
}
```

### 1.4 "Conservative by design" (원문 인용)

> 코드/디버그/URL 감지 시 primary 유지.

본 SPEC도 이 원칙을 유지. 애매하면 primary로 보수적.

## 2. 단순성 판정 6 기준 상세

### 2.1 기준 목록

| # | 기준 | 기본값 | 이유 |
|---|------|------|------|
| 1 | `char_count <= 160` | 160 | 트윗 길이 상한, 일반 대화 질문 포괄 |
| 2 | `word_count <= 28` | 28 | Hermes 경험치; 28 단어 이상은 구조적 요청 |
| 3 | `newline_count <= 2` | 2 | 한 단락 내 질문만 허용 |
| 4 | `NOT has_code_block(msg)` | 코드는 복잡 | 코드 리뷰/수정 요청은 primary 필요 |
| 5 | `NOT has_url(msg)` | URL 포함 요청은 복잡 | 자료 기반 작업은 primary |
| 6 | `NOT has_complex_keyword(msg)` | 키워드 set | 명시적 개발/인프라 작업 |

### 2.2 코드 블록 감지 알고리즘

1. ```` ``` ```` 또는 `~~~` 3개 연속 → fenced block
2. Indent 4+ space로 시작하는 **연속 2+ 라인** → code block 추정
3. Backtick inline (```` `x` ````)은 코드 아님(단일 단어 강조)

```go
var codeFencePattern = regexp.MustCompile("(?m)^(`{3,}|~{3,})")
var indentedCodePattern = regexp.MustCompile(`(?m)^(    |\t).+\n(    |\t).+`)
```

### 2.3 URL 감지

```go
var urlPattern = regexp.MustCompile(`https?://\S+`)
```

IP 주소(`http://1.2.3.4`)와 path 포함(`https://x.com/a`) 모두 매치.

### 2.4 복잡 키워드 set (기본 확장안)

```go
var defaultComplexKeywords = []string{
    // 코드 작업
    "debug", "implement", "refactor", "test", "analyze",
    "design", "architecture", "fix", "build", "compile",
    "review", "optimize", "profile",
    // 인프라
    "terminal", "docker", "kubernetes", "deploy", "install",
    "configure", "setup",
    // 파일/검색
    "grep", "search", "find", "read", "write", "edit",
    "create file", "delete",
    // 데이터
    "query", "migrate", "schema", "index",
    // 한국어 기본(v0.1 추가)
    "구현", "디버그", "리팩토링", "테스트", "분석",
    "설계", "배포",
}
```

**word boundary** 매칭:
```go
// "debugger"는 "debug" 키워드 매치 안 함 (whole word).
// "Debug"는 매치(case-insensitive).
pattern := `(?i)\b` + regexp.QuoteMeta(kw) + `\b`
```

한국어는 word boundary 개념이 약하므로 substring 매칭 별도 처리:
```go
// 한국어 키워드는 strings.Contains(lower, kw)로 판정
```

## 3. Provider Registry 15+ 엔트리 (hermes-llm.md §2 인용)

### 3.1 Phase 1 adapter_ready = true (6 provider)

| Provider | Default Model | Base URL | Auth | Tools | Vision | Embed |
|----------|--------------|----------|------|-------|--------|-------|
| anthropic | claude-opus-4-7 | https://api.anthropic.com/v1 | oauth/api_key | ✅ | ✅ | ❌ |
| openai | gpt-4o | https://api.openai.com/v1 | oauth/api_key | ✅ | ✅ | ✅ |
| google | gemini-2.0-flash | https://generativelanguage.googleapis.com/v1beta | api_key | ✅ | ✅ | ✅ |
| xai | grok-2 | https://api.x.ai/v1 | api_key | ✅ | ✅ | ❌ |
| deepseek | deepseek-chat | https://api.deepseek.com/v1 | api_key | ✅ | ❌ | ❌ |
| ollama | llama3.2 | http://localhost:11434 | none | ✅ | ✅ | ✅ |

### 3.2 Phase 1 metadata-only (adapter_ready = false, 9+ provider)

| Provider | Default Model | Base URL | Auth | Notes |
|----------|--------------|----------|------|-------|
| openrouter | (aggregator) | https://openrouter.ai/api/v1 | api_key | 100+ 모델 |
| nous | hermes-3 | (Portal URL) | oauth | Agent Key TTL |
| mistral | mistral-large | https://api.mistral.ai/v1 | api_key | |
| groq | llama3.2-70b | https://api.groq.com/openai/v1 | api_key | 500 RPM+ |
| qwen | qwen3 | (local or cloud) | oauth | |
| kimi | moonshot-v1 | https://api.moonshot.cn/v1 | api_key | |
| glm | glm-4 | https://open.bigmodel.cn/api/paas/v4 | api_key | ZhipuAI |
| minimax | abab6 | https://api.minimax.chat/v1 | api_key | |
| custom | (user-defined) | (user-defined) | api_key | custom_providers[] |

### 3.3 Registry Register 검증

```go
func (r *ProviderRegistry) Register(meta *ProviderMeta) error {
    if meta.Name == "" { return errors.New("name required") }
    if meta.DefaultBaseURL == "" && meta.Name != "custom" {
        return errors.New("base_url required unless custom")
    }
    if meta.AuthType != "oauth" && meta.AuthType != "api_key" && meta.AuthType != "none" {
        return errors.New("invalid auth_type")
    }
    r.providers[meta.Name] = meta
    return nil
}
```

## 4. Signature 설계

### 4.1 목적

- 동일 routing 결정이 여러 번 발생할 때 trace ID로 사용
- PROMPT-CACHE-001의 cache key 일부로 사용 가능
- INSIGHTS-001이 라우팅 패턴 분석 시 그룹핑 키

### 4.2 Canonical format

```
<model>|<provider>|<base_url>|<mode>|<command>|<args_hash_12>
```

예:
```
claude-opus-4-7|anthropic|https://api.anthropic.com/v1|chat|messages.create|a1b2c3d4e5f6
```

args_hash_12 = sha256(canonicalJSON(args))의 앞 12자. args에는 `max_tokens`, `temperature` 등 결정적 파라미터만. user-identifying 값 금지.

### 4.3 Canonical JSON

```go
func canonicalJSON(m map[string]any) string {
    // 키 정렬 후 json.Marshal
    keys := sortedKeys(m)
    var buf bytes.Buffer
    buf.WriteByte('{')
    for i, k := range keys {
        if i > 0 { buf.WriteByte(',') }
        kv, _ := json.Marshal(k)
        vv, _ := json.Marshal(m[k])
        buf.Write(kv); buf.WriteByte(':'); buf.Write(vv)
    }
    buf.WriteByte('}')
    return buf.String()
}
```

## 5. 테스트 전략

### 5.1 Classifier 테이블 테스트

```go
tests := []struct{
    name     string
    msg      string
    expected bool // isSimple
    reasons  []string
}{
    {"simple greeting", "hello", true, nil},
    {"single Korean sentence", "안녕하세요", true, nil},
    {"with complex keyword debug", "debug this", false, []string{"has_complex_keyword"}},
    {"with code fence", "```go\nfunc x(){}\n```", false, []string{"has_code_block"}},
    {"with URL", "check https://x.com", false, []string{"has_url"}},
    {"exceeds 161 chars", strings.Repeat("a", 161), false, []string{"exceeds_char_limit"}},
    {"exactly 160 chars", strings.Repeat("a", 160), true, nil},
    {"29 words", strings.Repeat("w ", 29), false, []string{"exceeds_word_limit"}},
    {"exactly 28 words", strings.Repeat("w ", 28), true, nil},
    {"3 newlines", "a\nb\nc\nd", false, []string{"exceeds_newline_limit"}},
    {"2 newlines", "a\nb\nc", true, nil},
    // CJK
    {"Korean implement keyword", "이 함수 구현해줘", false, []string{"has_complex_keyword"}},
    // mixed
    {"URL + keyword", "debug https://x.com", false, []string{"has_url", "has_complex_keyword"}},
    // boundary
    {"debugger (not keyword match)", "the debugger runs", true, nil}, // whole word
}
```

### 5.2 Router 통합 테스트

- Primary only (cheap nil) → 모든 입력에서 primary
- Force primary → classifier 결과 무시
- Force cheap + cheap nil → ErrCheapRouteUndefined
- Registry에 provider 없음 → ErrProviderNotRegistered
- RoutingDecisionHook 호출 확인(spy)

### 5.3 Signature 테스트

- 동일 Route → 동일 signature
- Args 순서만 다름 → 동일 signature (canonical JSON)
- Args 값 변경 → 다른 signature
- Signature에 timestamp/user ID 미포함 검증 (정규식 검사)

## 6. Go 라이브러리 결정

| 역할 | 라이브러리 | 근거 |
|-----|----------|------|
| 정규식 | stdlib `regexp` (RE2) | 성능 충분, 외부 의존 없음 |
| 해시 | stdlib `crypto/sha256` | signature hash |
| JSON | stdlib `encoding/json` | canonical 직렬화 |
| 로깅 | `go.uber.org/zap` | CORE-001 상속 |
| 테스트 | `stretchr/testify` | CORE-001 상속 |

**본 SPEC에서 채택 명시**:
- 외부 라이브러리 **추가 없음**. 순수 stdlib + zap.

## 7. 성능 목표

| 메트릭 | 목표 |
|-----|------|
| `Classifier.Classify()` p99 | < 500μs |
| `Router.Route()` p99 | < 1ms |
| Registry lookup | O(1) map access |

## 8. 오픈 이슈

- **Q1**: tool call이 있는 대화의 cheap 라우팅? 현 설계는 Phase 1 단순 heuristic만, provider capability 참조는 후속.
- **Q2**: Streaming 선호 시 non-streaming cheap 모델 스킵? 후속 SPEC의 dynamic routing.
- **Q3**: User의 locale 기반 한국어 키워드 활성화? 현 설계는 default set에 한국어 포함. config override로 조정.
- **Q4**: Signature에 conversation_length 포함? 제외(고유 결정 재현성 훼손). Hook에서 별도 로그.

## 9. 구현 순서 (TDD)

SPEC §6.6과 일치:

| # | Test | Focus |
|---|---|---|
| 1 | TestClassifier_SimpleGreeting | 기본 단순 판정 |
| 2 | TestClassifier_ComplexKeyword | 키워드 매칭 |
| 3 | TestClassifier_CodeBlock | code fence |
| 4 | TestClassifier_URL | URL 정규식 |
| 5 | TestClassifier_LongMessage | threshold |
| 6 | TestRouter_CheapRouteNil | fallback |
| 7 | TestRouter_Signature_Reproducible | canonical hash |
| 8 | TestRouter_UnregisteredProvider | registry 검증 |

GREEN 후:
- `ProviderRegistry.DefaultRegistry()` 15개 엔트리 단위 테스트
- RoutingDecisionHook 호출 확인
- Race detector 통과

---

**End of Research (SPEC-GOOSE-ROUTER-001)**
