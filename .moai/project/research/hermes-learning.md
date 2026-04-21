# Hermes Agent Self-Evolution 파이프라인 심층 분석

> **분석일**: 2026-04-21 · **대상**: `hermes-agent-main/{agent, trajectory_compressor.py, skills, mcp_serve.py, acp_*}` · **용도**: SPEC-GENIE-TRAJECTORY-001 / COMPRESSOR-001 / INSIGHTS-001 / ERROR-CLASS-001 / MEMORY-001 근거

## 1. 학습 파이프라인 E2E

```
사용자 상호작용 (CLI/Gateway/Batch)
  ↓
Trajectory 수집 (agent/trajectory.py)
  - ShareGPT JSON-L 포맷
  - 타임스탬프, tool calls, LLM resp
  ↓
Trajectory 압축 (trajectory_compressor.py, 1517 LoC)
  ① Protected head/tail
  ② Middle 샘플링 + 요약 (Gemini 3 Flash)
  ③ Token budget (Target: 15,250)
  ④ 메트릭 수집
  ↓
Insights 추출 (agent/insights.py, 34KB)
+ Error 분류 (agent/error_classifier.py, 28KB)
  ① Pattern/Preference
  ② Error 유형 (14 FailoverReason)
  ③ Opportunity
  ④ 신뢰도 계산
  ↓
Memory 저장 (agent/memory_manager.py)
+ Sync/Prefetch (agent/memory_provider.py)
  ① Builtin (MEMORY.md / USER.md)
  ② External Plugin (Honcho, Hindsight, Mem0)
  ↓
  ├─ Skill 자동 생성
  ├─ Prompt 개선 (시스템 프롬프트 업데이트)
  └─ LoRA 준비 (pre-training)
```

## 2. Trajectory 스키마

**ShareGPT 호환 JSON-L**:
```python
entry = {
    "conversations": [
        {"from": "system"|"human"|"gpt"|"tool", "value": str},
        ...
    ],
    "timestamp": datetime.isoformat(),
    "model": str,           # e.g., "anthropic/claude-opus"
    "completed": bool,
}

# 저장
trajectory_samples.jsonl      # 성공
failed_trajectories.jsonl     # 실패
```

**TrajectoryMetrics**:
```python
@dataclass
class TrajectoryMetrics:
    original_tokens: int
    compressed_tokens: int
    tokens_saved: int
    compression_ratio: float
    original_turns: int
    compressed_turns: int
    turns_compressed_start_idx: int
    turns_in_compressed_region: int
    was_compressed: bool
    still_over_limit: bool
    skipped_under_target: bool
    summarization_api_calls: int
    summarization_errors: int
```

## 3. Trajectory Compressor 알고리즘

```python
def compress_trajectory(trajectory) -> (trajectory, metrics):
    """첫 N턴 + 마지막 M턴 보호, 중간 압축"""
    
    # 1. 토큰 계산
    turn_tokens = [count_tokens(t) for t in trajectory]
    total = sum(turn_tokens)
    
    # 2. 목표 미만이면 스킵
    if total <= TARGET_MAX_TOKENS (15_250):
        metrics.skipped_under_target = True
        return trajectory, metrics
    
    # 3. 보호된 인덱스 (첫 system/human/gpt/tool, 마지막 4턴)
    protected = find_protected_indices(trajectory)
    compress_start, compress_end = find_compressible_region(protected)
    
    # 4. 압축량 계산
    tokens_to_save = total - TARGET_MAX_TOKENS
    target_compress = tokens_to_save + SUMMARY_TARGET_TOKENS (750)
    
    # 5. 누적해서 충분한 토큰 모일 때까지 턴 수집
    accumulated = 0
    compress_until = compress_start
    for i in range(compress_start, compress_end):
        accumulated += turn_tokens[i]
        compress_until = i + 1
        if accumulated >= target_compress:
            break
    
    # 6. 요약 생성 (Gemini 3 Flash, async)
    content = extract_turn_content(trajectory, compress_start, compress_until)
    summary = generate_summary_async(
        content, model="google/gemini-3-flash-preview"
    )
    
    # 7. 재구성
    compressed = []
    compressed.extend(trajectory[:compress_start])  # head
    compressed.append({"from": "human", "value": summary})
    compressed.extend(trajectory[compress_until:])  # tail
    
    return compressed, metrics
```

**특성**:
- 병렬 처리: `max_concurrent_requests: 50` (asyncio semaphore)
- Tokenizer: `moonshotai/Kimi-K2-Thinking` (trust_remote_code=True)
- 요약 모델: Google Gemini 3 Flash (temp 0.3)
- 재시도: max_retries=3, retry_delay=2s (jittered)
- 타임아웃: 300s/trajectory

## 4. Insights 분류 체계

**InsightsEngine**:
```python
overview = {
    "total_sessions": int,
    "total_tokens": int,
    "estimated_cost": float,
    "total_hours": float,
    "avg_session_duration": float,
}

models = [{
    "model": str,
    "sessions": int,
    "input_tokens": int,
    "output_tokens": int,
    "cache_read_tokens": int,
    "cache_write_tokens": int,
    "total_tokens": int,
    "tool_calls": int,
    "cost": float,
    "has_pricing": bool,
}]

tools = [{"tool": str, "count": int, "percentage": float}]

activity = {
    "by_day": [{"day": str, "count": int}, ...],    # Mon-Sun
    "by_hour": [{"hour": int, "count": int}, ...],  # 0-23
    "busiest_day": {"day": str, "count": int},
    "busiest_hour": {"hour": int, "count": int},
    "active_days": int,
    "max_streak": int,
}
```

## 5. Error Classifier (14 FailoverReason)

```python
class FailoverReason(Enum):
    # 인증/인가
    auth = "auth"                      # 401/403 일시적
    auth_permanent = "auth_permanent"
    
    # 과금/할당량
    billing = "billing"                # 402, 신용 소진
    rate_limit = "rate_limit"          # 429
    
    # 서버
    overloaded = "overloaded"          # 503/529
    server_error = "server_error"      # 500/502
    
    # 컨텍스트/payload
    context_overflow = "context_overflow"
    payload_too_large = "payload_too_large"  # 413
    
    # 모델
    model_not_found = "model_not_found"  # 404
    
    # 기타
    timeout = "timeout"
    format_error = "format_error"
    thinking_signature = "thinking_signature"  # Anthropic
    unknown = "unknown"

@dataclass
class ClassifiedError:
    reason: FailoverReason
    status_code: Optional[int]
    retryable: bool
    should_compress: bool
    should_rotate_credential: bool
    should_fallback: bool
    message: str
```

**분류 파이프라인**:
```
1. Provider 특화 패턴 (Anthropic thinking_signature, long_context_tier)
2. HTTP 상태 코드 + 메시지 정제
   401→auth, 402→billing/rate_limit, 403→auth/billing, 404→model_not_found,
   413→payload_too_large, 429→rate_limit, 400→context_overflow/format,
   500/502→server_error, 503/529→overloaded
3. Error code 분류 (body.error.code)
   resource_exhausted→rate_limit, insufficient_quota→billing,
   context_length_exceeded→context_overflow
4. Message 패턴 매칭 (case-insensitive)
   _BILLING_PATTERNS, _RATE_LIMIT_PATTERNS, _CONTEXT_OVERFLOW_PATTERNS,
   _AUTH_PATTERNS
5. Transport 휴리스틱
   ReadTimeout/ConnectTimeout→timeout
   Server disconnect + (tokens>60% OR >120K OR msgs>200)→context_overflow
6. Fallback: unknown (retryable=True)
```

## 6. Memory Provider 인터페이스

```python
class MemoryProvider(ABC):
    """Swappable 메모리 백엔드"""
    
    # 필수
    @abstractmethod
    def name(self) -> str:           # "builtin", "honcho", "hindsight", "mem0"
    @abstractmethod
    def is_available(self) -> bool:  # 설정 + 자격증명 (네트워크 X)
    @abstractmethod
    def initialize(self, session_id, **kwargs):
        # kwargs: hermes_home, platform, agent_context, agent_identity
    @abstractmethod
    def get_tool_schemas(self) -> List[Dict]:
    
    # 선택
    def system_prompt_block(self) -> str
    def prefetch(self, query, *, session_id) -> str  # 회상 (빠름)
    def queue_prefetch(self, query, *, session_id)   # 백그라운드
    def sync_turn(self, user_content, assistant_content, *, session_id)
    def handle_tool_call(self, tool_name, args, **kwargs) -> str  # JSON
    
    # Lifecycle hooks
    def on_turn_start(self, turn_number, message, **kwargs)
    def on_session_end(self, messages)
    def on_pre_compress(self, messages) -> str
    def on_delegation(self, task, result, **kwargs)
```

**MemoryManager**:
- Builtin 항상 첫 번째 (제거 불가)
- 최대 1개 외부 plugin provider
- Tool schema 수집 (이름 충돌 검출)
- 실패 격리 (한 provider 오류 ≠ 차단)

## 7. Skill 카탈로그 상위 10

**구조**:
```
skills/                     optional-skills/
├─ autonomous-ai-agents/    ├─ autonomous-ai-agents/
├─ creative/                ├─ blockchain/
├─ data-science/            ├─ communication/
├─ devops/                  ├─ creative/
├─ diagramming/             ├─ devops/
├─ domain/                  ├─ email/
├─ email/                   ├─ health/
├─ feeds/                   ├─ migration/
├─ gaming/                  ├─ mlops/
├─ github/                  ├─ productivity/
└─ inference-sh/            └─ research/
```

**Frontmatter 스키마**:
```yaml
name: "Skill Display Name"
description: "One-line"
platforms: ["linux", "darwin", "windows"]
metadata:
  hermes:
    config:
      KEY: "default_value"
```

**자동 생성** (skill_commands.py):
- `scan_skill_commands()`: SKILL.md 재귀 스캔
- 비활성화 필터링
- 플랫폼 호환성 체크
- 정규화: 공백/_ → -

## 8. MCP Server 번들 (mcp_serve.py, 30KB)

**10 tool 노출** (stdio 기반):
```python
# 대화 관리
conversations_list(platform?, limit?, search?)
conversation_get(session_key)
messages_read(session_key, limit?)
attachments_fetch(session_key, message_id)

# 이벤트
events_poll(after_cursor?, session_key?, limit?)
events_wait(after_cursor?, session_key?, timeout_ms?)

# 전송 & 승인
messages_send(target, message)
permissions_list_open()
permissions_respond(approval_id, decision)

# Hermes 특화
channels_list(platform?)
```

**EventBridge**:
- SessionDB → mtime 캐시 (200ms 주기)
- 새 메시지 → QueueEvent (in-memory, 1000 제한)
- Thread-safe: threading.Event + lock

## 9. ACP Adapter (IDE 프로토콜)

```
acp_adapter/
├─ server.py          # ACP 엔드포인트
├─ events.py          # 이벤트 정의
├─ tools.py           # Tool 라우팅
├─ session.py         # Session 상태
├─ auth.py            # OAuth, JWT
├─ permissions.py
└─ entry.py
```

**Google A2A v0.3과 차이**:
- Hermes = 메시징 서버 (대화 중심)
- ACP = IDE 프로토콜 (VS Code/Zed/JetBrains)
- 둘다 tool schema 노출 (OpenAI 형식)

## 10. GENIE Go 포팅 매핑

```
internal/
├── learning/
│   ├── trajectory.go    ← agent/trajectory.py (~100 LoC)
│   ├── compressor.go    ← trajectory_compressor.py (1517→800 LoC)
│   └── insights.go      ← agent/insights.py (34KB→600 LoC)
├── memory/
│   ├── provider.go      ← agent/memory_provider.py (interface)
│   ├── manager.go       ← agent/memory_manager.py
│   ├── sqlite.go        ← agent/builtin_memory_provider.py
│   └── plugin.go        (Honcho, Hindsight, Mem0)
├── skill/
│   ├── catalog.go       ← tools/skills_tool.py
│   ├── command.go       ← agent/skill_commands.py
│   └── runtime.go
├── evolve/
│   ├── reflect.go       ← agent/insights.py
│   ├── safety.go        ← ErrorClassifier interface
│   └── compress.go      ← PreCompressHook
└── mcp/
    ├── server.go        ← mcp_serve.py
    └── registry.go      ← EventBridge, ConversationTools
```

**추정 LoC**:
- learning/trajectory: 100
- learning/compressor: 800
- learning/insights: 600
- memory/{provider,manager,sqlite}: 800
- skill/{catalog,command,runtime}: 700
- evolve/{reflect,safety,compress}: 500
- mcp/{server,registry}: 600
- **합계: ~4,000 Go LoC** (Python 원본 ~3,500)

## 11. GENIE SPEC 도출

### SPEC-GENIE-TRAJECTORY-001
- `SaveTrajectory(trajectory, model, completed) -> error`
- ShareGPT JSON-L 포맷
- 저장소: `~/.genie/trajectories/{success,failed}/YYYY-MM-DD.jsonl`
- 익명화 (redact pipeline, 선택적)

### SPEC-GENIE-COMPRESSOR-001
- 기간별 메트릭
- Protected head/tail 전략
- 중간 LLM 요약 (Gemini 3 Flash 또는 구성 가능)
- TrajectoryMetrics 수집

### SPEC-GENIE-INSIGHTS-001
- overview/models/tools/activity 다차원
- busiest_day, max_streak, active_days
- JSON 또는 terminal UI (테이블)

### SPEC-GENIE-MEMORY-001 (재작성)
- MemoryProvider interface
- Builtin (SQLite, 파일) + 외부 1개 (Honcho/Hindsight/Mem0)
- Session 격리 (session_id)

### SPEC-GENIE-ERROR-CLASS-001
- 14 FailoverReason enum
- 입력: error, provider, model, approx_tokens, context_length
- 출력: ClassifiedError
- 파이프라인: 상태코드→에러코드→메시지→휴리스틱

## 12. Hermes SPEC-REFLECT 연계

**SPEC-REFLECT-001의 5단계 승격**:
```
Layer 1: Trajectory 수집 → internal/learning/trajectory
Layer 2: 압축 → internal/learning/compressor
Layer 3: Insights 추출 → internal/evolve/{reflect, safety}
Layer 4: Memory 저장 → internal/memory/{provider, manager, sqlite}
Layer 5: Skill/Prompt 자동 진화 → internal/skill/{catalog, command, runtime}
```

LoRA 파인튜닝 준비는 별도 (internal/learning/lora — Rust).

## 13. 재사용 vs 재작성

| 모듈 | 원본 | 이행 | Go LoC |
|---|---|---|---|
| Trajectory | trajectory.py | 재작성 | 100 |
| Compressor | 1517 LoC | 60% 재사용 | 800 |
| Insights | 34KB | 80% 재사용 | 600 |
| Error Classifier | 28KB | 90% 재사용 | 500 |
| Memory Provider | interface | 재사용 | 150 |
| Memory Manager | 368 LoC | 재사용 | 400 |
| Skill Catalog | 200+ LoC | 80% 재사용 | 250 |
| MCP Server | 867 LoC | 30% 재사용 | 500 |
| **합계** | ~3,500 | **55%** | **~4,000** |

## 14. 최우선 구현

1. Trajectory 수집 (기반)
2. Compressor + Insights (데이터 품질)
3. Memory provider interface (확장성)
4. Error classifier (복구 전략)
5. MCP gateway (IDE 통합)

---

**결론**: Hermes 자기개선 파이프라인 = **Trajectory 수집 → 지능형 압축 → 다층 Insights → Swappable Memory → Skill 자동 발견 → MCP 게이트웨이** 6단. GENIE 포팅 **재사용 55%**, ~4,000 Go LoC (Python 3,500). Tokenizer 의존성 제거 (Go unicode counting), async → goroutine+channel.
