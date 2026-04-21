# SPEC-GOOSE-PROMPT-CACHE-001 — Research

> Hermes `prompt_cache.py` 알고리즘 상세 + Anthropic 제약 + 테스트 fixture.

## 1. Hermes 원형 분석 (hermes-llm.md §6 인용)

### 1.1 원문 시각화 인용

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

Wait, 원문을 정확히 다시 읽어봄:

> **알고리즘**:
> 1. System prompt에 cache marker (1/4)
> 2. 최대 4 breakpoint (Anthropic 제한)
> 3. Non-system 메시지 중 마지막 3개에 cache marker
> 4. 타입: `{"type": "ephemeral"}` (5분) 또는 `{"type": "ephemeral", "ttl": "1h"}`

즉 총 **1 (system) + 3 (last 3 non-system) = 4** breakpoint. 위 다이어그램은 예시용이며 실제로는 messages[5], messages[6], messages[7]의 마지막 3개 non-system 메시지에 marker가 와야 한다.

### 1.2 효과 인용

> **효과**: Multi-turn에서 입력 토큰 비용 ~75% 절감.

## 2. Anthropic API 제약 (외부 문서)

### 2.1 Breakpoint 개수

> You can define up to 4 cache breakpoints per request.

### 2.2 TTL 옵션

- `ephemeral` (5분, 기본)
- `ephemeral` + `ttl: "1h"` (1시간, beta 기능)

### 2.3 Cache block 위치

> Any content block (text, image, tool_use, tool_result, document) can contain a `cache_control` parameter. The cached prefix extends from the beginning of the prompt to the content block with `cache_control`, inclusive.

즉 `cache_control`이 붙은 **마지막 block까지** 캐시됨.

### 2.4 순서 제약

Breakpoint들은 prompt 순서대로 "증가하는" 관계여야 함. 뒤 breakpoint는 앞 breakpoint의 캐시를 재사용.

## 3. 알고리즘 edge case

### 3.1 Non-system < 3

messages = `[system, user]`:
- System marker: index 0
- Last non-system: index 1
- 총 2 marker

messages = `[system]`:
- System marker: index 0
- 총 1 marker

### 3.2 System 없음

messages = `[user, assistant, user]`:
- Last 3 non-system: `[0, 1, 2]`
- 총 3 marker

messages = `[user]`:
- 총 1 marker

### 3.3 마지막 메시지가 system일 가능성?

일반적으로 없음. 하지만 방어적으로:

```go
if messages[0].Role != "system" {
    // system marker 생략
}
// last 3 non-system은 Role != "system" 필터링
```

### 3.4 중복 인덱스 방지

messages = `[system, user]` + strategy=SystemAnd3:
- System marker: index 0
- Last non-system: index 1
- 중복 없음

messages = `[user]`:
- System marker 없음
- Last non-system: index 0
- 총 1 marker

### 3.5 Multi content block

Anthropic 메시지는 `Content: []ContentBlock`일 수 있음. 예시:
```go
Message{
    Role: "user",
    Content: []ContentBlock{
        {Type: "tool_result", ToolUseID: "abc", Content: "..."},
        {Type: "text", Text: "and also..."},
    },
}
```

이 경우 marker는 **마지막 ContentBlock**(index 1)에 적용(REQ-PC-012). Anthropic 문서 "cached prefix extends ... inclusive" 해석.

## 4. TTL 선택 가이드

### 4.1 5분 vs 1시간

| 요소 | 5분 (기본) | 1시간 |
|------|----------|------|
| 비용 | 5분 기본 가격 | 1시간 1.5x ~ 2x |
| Hit 기대 | 짧은 대화 | 장기 세션 |
| Cache write cost | 쓰기 비용 1x | 쓰기 비용 2x |
| 권장 | multi-turn chat | IDE agent, 장시간 작업 |

### 4.2 Phase 1 정책

본 SPEC은 TTL을 **호출자가 명시**. ADAPTER-001의 Anthropic 어댑터가 다음 heuristic으로 결정 가능(후속 SPEC):
- conversation_length < 5 turns → `5m`
- conversation_length >= 5 turns → `1h`

Phase 1은 `5m` 기본.

## 5. 테스트 fixture 설계

### 5.1 message 구성 케이스

```go
tests := []struct{
    name      string
    messages  []Message
    strategy  CacheStrategy
    ttl       TTL
    wantMarkers []CacheMarker
}{
    {
        name: "full 6 messages, SystemAnd3",
        messages: []Message{
            {Role:"system", Content:[]Block{{Type:"text",Text:"sys"}}},
            {Role:"user",      Content:[]Block{{Type:"text",Text:"u1"}}},
            {Role:"assistant", Content:[]Block{{Type:"text",Text:"a1"}}},
            {Role:"user",      Content:[]Block{{Type:"text",Text:"u2"}}},
            {Role:"assistant", Content:[]Block{{Type:"text",Text:"a2"}}},
            {Role:"user",      Content:[]Block{{Type:"text",Text:"u3"}}},
        },
        strategy: SystemAnd3,
        ttl:      TTLDefault,
        wantMarkers: []CacheMarker{
            {0, 0, "5m"},  // system
            {3, 0, "5m"},  // user (3 from end)
            {4, 0, "5m"},  // assistant
            {5, 0, "5m"},  // user (last)
        },
    },
    {
        name: "system + 2 non-system",
        messages: []Message{
            {Role:"system", ...},
            {Role:"user", ...},
            {Role:"assistant", ...},
        },
        strategy: SystemAnd3,
        ttl:      TTLDefault,
        wantMarkers: []CacheMarker{
            {0, 0, "5m"},
            {1, 0, "5m"},
            {2, 0, "5m"},
        },
    },
    {
        name: "no system, 3 non-system",
        messages: []Message{
            {Role:"user", ...},
            {Role:"assistant", ...},
            {Role:"user", ...},
        },
        strategy: SystemAnd3,
        ttl:      TTL1Hour,
        wantMarkers: []CacheMarker{
            {0, 0, "1h"},
            {1, 0, "1h"},
            {2, 0, "1h"},
        },
    },
    {
        name: "SystemOnly",
        messages: []Message{
            {Role:"system", ...},
            {Role:"user", ...},
            {Role:"assistant", ...},
        },
        strategy: SystemOnly,
        ttl:      TTLDefault,
        wantMarkers: []CacheMarker{{0, 0, "5m"}},
    },
    {
        name:        "None strategy",
        messages:    []Message{{Role:"system"}, {Role:"user"}},
        strategy:    None,
        ttl:         TTLDefault,
        wantMarkers: nil,
    },
    {
        name:        "empty messages",
        messages:    nil,
        strategy:    SystemAnd3,
        ttl:         TTLDefault,
        wantMarkers: nil,
    },
    {
        name: "multi content block message",
        messages: []Message{
            {Role:"system", Content:[]Block{{Type:"text"}}},
            {Role:"user", Content:[]Block{
                {Type:"tool_result"},
                {Type:"text"},
                {Type:"text"},
            }},
        },
        strategy: SystemAnd3,
        ttl:      TTLDefault,
        wantMarkers: []CacheMarker{
            {0, 0, "5m"},
            {1, 2, "5m"}, // last content block index 2
        },
    },
}
```

### 5.2 결정성 테스트

동일 입력 두 번 → 동일 plan:
```go
func TestPlanner_Deterministic(t *testing.T) {
    p := NewPlanner()
    msgs := buildMessages()
    plan1, _ := p.Plan(msgs, SystemAnd3, TTLDefault)
    plan2, _ := p.Plan(msgs, SystemAnd3, TTLDefault)
    assert.Equal(t, plan1.Markers, plan2.Markers)
}
```

### 5.3 Race 테스트

순수 함수이므로 race 없음. 방어적 테스트:
```go
func TestPlanner_Concurrent(t *testing.T) {
    p := NewPlanner()
    msgs := buildMessages()
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            p.Plan(msgs, SystemAnd3, TTLDefault)
        }()
    }
    wg.Wait()
}
```

## 6. Go 라이브러리 결정

| 역할 | 라이브러리 | 근거 |
|-----|----------|------|
| 테스트 | `stretchr/testify` | 기존 SPEC과 동일 |

**외부 라이브러리 추가 없음**. 순수 stdlib.

## 7. 성능 목표

| 메트릭 | 목표 |
|-----|------|
| `Plan()` p99 (6 메시지) | < 10μs |
| `Plan()` p99 (100 메시지) | < 50μs |

순수 인덱스 연산이므로 매우 빠름.

## 8. ADAPTER-001 인터페이스

Anthropic 어댑터가 소비하는 방식:

```go
// ADAPTER-001의 Anthropic 어댑터 가상 코드
plan, err := cachePlanner.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
if err != nil {
    return nil, err
}

for _, m := range plan.Markers {
    applyMarker(&apiRequest.Messages[m.MessageIndex], m.ContentBlockIndex, m.TTL)
}
```

본 SPEC은 `Plan`만 반환. 실제 JSON에 `cache_control: {"type":"ephemeral","ttl":"1h"}` 삽입은 ADAPTER-001.

## 9. 오픈 이슈

- **Q1**: messages가 100+개일 때 성능? 순수 slice iteration 2회로 O(n). 문제 없음.
- **Q2**: Anthropic이 tool_use 전용 cache 분리? 현 알고리즘은 block 단위 중립. 변경 없음.
- **Q3**: `MinMessageTokens` 옵션 활성화 시 token counter 의존. Phase 1은 미활성 가정.
- **Q4**: 1h TTL이 지역/account별로 미지원인 경우? ADAPTER-001이 fallback 처리 (본 SPEC 범위 외).

## 10. 구현 순서 (TDD)

| # | Test | Focus |
|---|---|---|
| 1 | TestPlanner_SystemAnd3_FullMessages | 기본 6 msg |
| 2 | TestPlanner_SystemAnd3_FewMessages | 축소 |
| 3 | TestPlanner_SystemAnd3_NoSystem | system 없음 |
| 4 | TestPlanner_SystemOnly | strategy |
| 5 | TestPlanner_None | empty plan |
| 6 | TestPlanner_EmptyMessages | 경계 |
| 7 | TestPlanner_MultipleContentBlocks_UsesLastIndex | block index |
| 8 | TestPlanner_TTL1Hour_Propagates | TTL 전파 |

---

**End of Research (SPEC-GOOSE-PROMPT-CACHE-001)**
