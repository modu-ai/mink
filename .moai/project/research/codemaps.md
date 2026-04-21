# Hermes Agent — 코드맵 (codemaps)

> 저장소: **NousResearch/hermes-agent** · 버전: v0.8.0 · 라이선스: MIT
> 주 언어: Python (코어), JavaScript/Node.js (게이트웨이/웹)
> 규모: ~70,000+ LOC, 약 200+ 개 모듈

본 문서는 Hermes Agent 저장소의 **전체 아키텍처, 디렉토리 구조, 모듈 의존성, 데이터 흐름, 진입점**을 한 장에서 조망할 수 있도록 정리한 **코드맵**이다. 각 파일별 상세 설명은 동봉된 `codewiki.md`를 참고한다.

---

## 1. 한 줄 요약

Hermes Agent는 **다중 LLM 공급자를 지원하는 터미널/메신저 기반 AI 에이전트 프레임워크**이다. 중심에는 대화 루프를 관리하는 `run_agent.py`의 `AIAgent` 클래스가 있으며, 그 주위로 (1) 도구 레지스트리 기반의 **플러그인 도구 시스템**, (2) 탈착 가능한 **메모리 공급자**, (3) 6개 이상의 **실행 환경**, (4) 7개 이상의 **메신저 플랫폼 게이트웨이**, (5) 절차적 기억으로서의 **스킬(Skills) 시스템**, (6) **SQLite 기반 세션 상태 저장소**가 모듈형으로 부착되어 있다.

---

## 2. 디렉토리 트리 (핵심 구조)

```
hermes-agent/
├── README.md                     프로젝트 개요/설치 가이드
├── cli.py                        대화형 터미널 UI (≈9,185 LOC)
├── run_agent.py                  에이전트 대화 루프 오케스트레이터 (≈9,845 LOC)
├── batch_runner.py               RL 트래젝토리 배치 생성기
├── mcp_serve.py                  MCP(Model Context Protocol) 서버
├── mini_swe_runner.py            SWE 특화 에이전트 러너
├── model_tools.py                도구 discovery · dispatch 파사드
├── toolsets.py                   도구 그룹(toolset) 정의/해석
├── hermes_state.py               SQLite 세션/메시지 영속화 (+FTS5)
├── hermes_constants.py           전역 상수/경로
├── hermes_logging.py             로깅 설정
├── hermes_time.py                시간 유틸
├── rl_cli.py                     강화학습 CLI
├── trajectory_compressor.py      RL 트래젝토리 압축
├── utils.py                      범용 유틸
│
├── agent/                        에이전트 코어 유틸 (24개 파일)
│   ├── anthropic_adapter.py      Anthropic API 어댑터
│   ├── auxiliary_client.py       OpenAI/서드파티 클라이언트
│   ├── builtin_memory_provider.py 내장 JSON 메모리
│   ├── context_compressor.py     컨텍스트 토큰 압축
│   ├── context_references.py     .hermes.md 등 참조 파일 관리
│   ├── copilot_acp_client.py     Copilot ACP 클라이언트
│   ├── credential_pool.py        API 키 풀 관리
│   ├── display.py                터미널 포맷팅/스피너
│   ├── error_classifier.py       API 에러 분류/페일오버 판단
│   ├── insights.py               사용량 분석
│   ├── memory_manager.py         메모리 오케스트레이터
│   ├── memory_provider.py        메모리 플러그인 추상 인터페이스
│   ├── model_metadata.py         모델 메타/토큰 추정
│   ├── models_dev.py             개발용 모델 설정
│   ├── prompt_builder.py         시스템 프롬프트 빌더
│   ├── prompt_caching.py         Anthropic 프롬프트 캐싱
│   ├── rate_limit_tracker.py     레이트리밋 추적
│   ├── redact.py                 민감정보 마스킹
│   ├── retry_utils.py            지수 백오프/지터
│   ├── skill_commands.py         스킬 명령 핸들러
│   ├── skill_utils.py            스킬 discovery/파싱
│   ├── smart_model_routing.py    자동 모델 라우팅
│   ├── subdirectory_hints.py     디렉토리별 컨텍스트 힌트
│   ├── title_generator.py        세션 제목 자동 생성
│   ├── trajectory.py             RL 트래젝토리 변환
│   └── usage_pricing.py          토큰 비용 계산
│
├── tools/                        도구 구현 (53+ 파일)
│   ├── registry.py               중앙 도구 레지스트리 (싱글턴)
│   ├── web_tools.py              웹 검색/추출
│   ├── terminal_tool.py          셸 명령 실행 (다중 백엔드)
│   ├── file_tools.py             파일 read/write/patch
│   ├── vision_tools.py           이미지 분석
│   ├── browser_tool.py           브라우저 자동화 파사드
│   ├── browser_camofox.py        Anti-detection 브라우저
│   ├── browser_providers/        브라우저 백엔드 (Playwright/Browserbase/Firecrawl/BrowserUse)
│   ├── skills_tool.py            스킬 list/view/manage 도구
│   ├── memory_tool.py            MemoryManager 브릿지 도구
│   ├── session_search_tool.py    세션 FTS5 검색
│   ├── todo_tool.py              할 일 관리
│   ├── delegate_tool.py          서브에이전트 spawn
│   ├── code_execution_tool.py    Python 코드 실행
│   ├── image_generation_tool.py  이미지 생성 (Fal/Replicate/SD)
│   ├── tts_tool.py               TTS (Google/OpenAI/Neu/로컬)
│   ├── cronjob_tools.py          크론 스케줄링
│   ├── mcp_tool.py               외부 MCP 서버 도구 동적 로드
│   ├── homeassistant_tool.py     Home Assistant 연동
│   ├── mixture_of_agents_tool.py MoA 앙상블
│   ├── send_message_tool.py      게이트웨이 메시지 송신
│   ├── clarify_tool.py           사용자 되묻기
│   ├── approval.py               민감 명령 승인
│   ├── process_registry.py       프로세스 PID 관리
│   └── environments/             실행 환경(샌드박스) 백엔드
│       ├── base.py               추상 인터페이스
│       ├── local.py              로컬 셸
│       ├── docker.py             도커 컨테이너
│       ├── ssh.py                원격 SSH
│       ├── modal.py              Modal serverless
│       ├── managed_modal.py      영속 Modal 세션
│       ├── daytona.py            Daytona IDE
│       ├── singularity.py        Singularity HPC 컨테이너
│       └── modal_utils.py        Modal 유틸
│
├── hermes_cli/                   CLI 애플리케이션 레이어 (44 모듈)
│   ├── main.py                   `hermes` 커맨드 엔트리
│   ├── banner.py                 ASCII 배너
│   ├── commands.py               슬래시 명령 핸들러
│   ├── auth.py / auth_commands.py OAuth/인증 플로우
│   ├── config.py                 config.yaml 로더
│   ├── model_switch.py / models.py 모델 전환 UI
│   ├── cron.py                   크론잡 CLI
│   ├── doctor.py                 헬스 체크
│   ├── gateway.py                게이트웨이 설정 마법사
│   ├── curses_ui.py              curses 기반 UI 컴포넌트
│   ├── callbacks.py              이벤트 콜백
│   ├── pairing.py                DM 페어링
│   ├── plugins.py                플러그인 discovery/loader
│   └── mcp_config.py             MCP 서버 설정
│
├── gateway/                      멀티플랫폼 메신저 게이트웨이 (11 모듈)
│   ├── run.py                    게이트웨이 오케스트레이터
│   ├── session.py                게이트웨이 세션
│   ├── config.py                 게이트웨이 설정
│   ├── delivery.py               응답 전송/청크 분할
│   ├── pairing.py                사용자 페어링
│   ├── status.py                 상태 추적
│   ├── hooks.py                  before/after 이벤트 훅
│   ├── builtin_hooks/            기본 훅 구현
│   ├── channel_directory.py      채널 레지스트리
│   └── platforms/                텔레그램/디스코드/슬랙/왓츠앱/시그널/이메일/매트릭스
│
├── acp_adapter/                  Anthropic Copilot Protocol 어댑터 (8 모듈)
│   └── entry.py / server.py / auth.py / session.py / events.py / tools.py / permissions.py
│
├── skills/                       번들 스킬 라이브러리 (28+ 카테고리)
│   └── apple/ github/ software-development/ mlops/ research/ productivity/ creative/ …
│
├── optional-skills/              선택 스킬 팩 (16 디렉토리)
├── environments/                 RL 학습 환경 (14 모듈, ai_town/atropos 등)
├── tests/                        테스트 스위트 (43+ 파일)
├── scripts/                      설치/유틸 스크립트
├── docs/ website/ landingpage/   문서/사이트
├── docker/ nix/                  배포 설정
└── pyproject.toml / requirements.txt / flake.nix
```

---

## 3. 진입점 (Entry Points)

| 진입점 | 파일 | 용도 |
|---|---|---|
| `hermes` CLI | `hermes_cli/main.py` → `cli.py` | 대화형 터미널 UI |
| 에이전트 루프 | `run_agent.py : AIAgent.run_conversation()` | 실제 대화 처리 루프 |
| 배치 생성 | `batch_runner.py` | RL 트래젝토리 대량 생성 |
| MCP 서버 | `mcp_serve.py` | Hermes 도구를 MCP로 노출 |
| 게이트웨이 데몬 | `gateway/run.py` (`hermes gateway start`) | 메신저 플랫폼 브릿지 |
| SWE 러너 | `mini_swe_runner.py` | 코드베이스 수정 태스크 전용 |
| ACP 어댑터 | `acp_adapter/entry.py` | Copilot ACP 호환 |
| RL CLI | `rl_cli.py` | 강화학습 워크플로 |

---

## 4. 레이어드 아키텍처 한눈에 보기

```
┌────────────────────────────────────────────────────────────────┐
│                        사용자 인터페이스                        │
│   cli.py (TUI)   gateway/ (Telegram/Discord/…)   acp_adapter/  │
│   mcp_serve.py   batch_runner.py    mini_swe_runner.py         │
└───────────────────────────────┬────────────────────────────────┘
                                │
┌───────────────────────────────▼────────────────────────────────┐
│                      에이전트 오케스트레이션                    │
│            run_agent.py :: AIAgent.run_conversation()          │
│        (대화 루프, 모델 호출, 툴 콜 루프, 상태 동기화)          │
└──┬───────────────┬───────────────┬───────────────┬─────────────┘
   │               │               │               │
   ▼               ▼               ▼               ▼
┌──────────┐  ┌──────────┐  ┌──────────────┐  ┌───────────────┐
│  agent/  │  │  tools/  │  │hermes_state  │  │memory_manager │
│프롬프트/ │  │레지스트리│  │.py (SQLite+  │  │ (+provider)   │
│에러/메모리│  │+53 tools │  │  FTS5)       │  │               │
└────┬─────┘  └────┬─────┘  └──────┬───────┘  └──────┬────────┘
     │             │               │                 │
     ▼             ▼               ▼                 ▼
┌──────────┐ ┌──────────────┐ ┌──────────┐   ┌──────────────┐
│ LLM API  │ │environments/ │ │ ~/.hermes│   │ builtin JSON │
│(OpenAI,  │ │local/docker/ │ │ /state.db│   │ + 외부 플러그│
│Anthropic,│ │ssh/modal/... │ │          │   │인 (honcho 등)│
│OpenRouter│ └──────────────┘ └──────────┘   └──────────────┘
└──────────┘
```

---

## 5. 대화 한 턴의 데이터 흐름

```
사용자 입력 (CLI/게이트웨이)
        │
        ▼
┌──────────────────────────────────────────────────────────┐
│ ① 턴 준비                                                 │
│  · HermesState에서 세션/히스토리 로드                    │
│  · MemoryManager.prefetch_all(user_input) → 기억 회상    │
│  · prompt_builder.build_system_prompt(identity +         │
│       memory + skills + .hermes.md + SOUL.md)            │
│  · context_compressor로 토큰 예산 맞춤                   │
└──────────────────────────────────────────────────────────┘
        │
        ▼
┌──────────────────────────────────────────────────────────┐
│ ② LLM 호출                                                │
│  · messages = [system, …history, user]                    │
│  · auxiliary_client/anthropic_adapter로 공급자 디스패치  │
│  · 스트리밍 + Anthropic 프롬프트 캐시 적용               │
│  · usage_pricing/rate_limit_tracker가 토큰·비용·한도 추적 │
│  · error_classifier가 실패 시 페일오버 판단              │
└──────────────────────────────────────────────────────────┘
        │
        ▼
┌──────────────────────────────────────────────────────────┐
│ ③ 툴 콜 루프 (LLM이 tool_call을 요청할 때)                │
│  model_tools.handle_function_call(name, args)             │
│     → tools.registry.dispatch() → ToolEntry.handler       │
│         · terminal   → environments/* 백엔드로 실행        │
│         · browser    → browser_providers/*                │
│         · file       → read/write/patch                    │
│         · memory     → MemoryManager.handle_tool_call()    │
│         · skills     → skill_utils로 로드                  │
│         · delegate   → ThreadPoolExecutor 서브에이전트     │
│         · mcp        → 외부 MCP 서버로 릴레이               │
│  결과를 tool_result 메시지로 히스토리에 추가              │
│  → ② 로 돌아감, 툴 콜이 없을 때까지                       │
└──────────────────────────────────────────────────────────┘
        │
        ▼
┌──────────────────────────────────────────────────────────┐
│ ④ 턴 마무리                                               │
│  · HermesState.save_message() 로 영속화                  │
│  · MemoryManager.sync_all(user, response) 로 기억 저장   │
│  · insights 업데이트, 리소스 정리 (브라우저/서브에이전트)│
└──────────────────────────────────────────────────────────┘
        │
        ▼
최종 assistant 응답 → CLI 또는 게이트웨이 delivery로 송신
```

---

## 6. 모듈 의존성 그래프 (요약)

```
cli.py ──────────────┐
                     ├──► run_agent.py ──► agent/* ──► LLM API
hermes_cli/main.py ──┤                │
                     │                ├──► model_tools.py ──► tools/registry.py
gateway/run.py ──────┤                │                        │
                     │                │                        ├──► tools/*.py
mcp_serve.py ────────┤                │                        └──► tools/environments/*
                     │                ├──► hermes_state.py (SQLite)
batch_runner.py ─────┤                ├──► toolsets.py
                     │                └──► agent/memory_manager.py
acp_adapter/entry ───┘                             │
                                                   ├──► agent/memory_provider.py (ABC)
                                                   ├──► agent/builtin_memory_provider.py
                                                   └──► 외부 플러그인 (honcho 등)
```

**순환 방지**: `tools/registry.py`는 다른 `tools/*` 모듈을 import 하지 않는다. 각 도구가 import 시점에 `registry.register()`를 호출하고, `model_tools.py`가 모든 도구를 import 한 뒤 레지스트리에서 조회한다.

---

## 7. 핵심 디자인 패턴

**7.1 Tool Registry 패턴** — 싱글턴 `ToolRegistry`에 각 도구가 self-register. `ToolEntry`에 schema·handler·`check_fn`(가용성 확인)이 포함.

**7.2 Memory Provider 패턴** — `MemoryProvider` ABC 기반 플러그인, 내장 provider + 최대 1개의 외부 provider를 `MemoryManager`가 통합.

**7.3 Execution Environment 패턴** — `environments/base.py`를 상속한 6+개의 백엔드(local/docker/ssh/modal/daytona/singularity)로 도구 실행을 추상화.

**7.4 Async Bridging 패턴** — 스레드별 영속 이벤트 루프로 "Event loop is closed" 오류 방지. 캐시된 `httpx`/`AsyncOpenAI` 클라이언트가 재사용됨.

**7.5 Prompt Injection Defense** — `_scan_context_content()`가 SOUL.md, .hermes.md, AGENTS.md 등에서 주입 패턴/제로폭 문자를 탐지, 의심 시 자리표시자로 대체.

**7.6 Fenced Memory Context** — 회상된 메모리는 `<memory-context>` 펜스로 감싸서 모델이 사용자 지시와 혼동하지 않도록 함.

**7.7 Smart Failover** — `error_classifier.FailoverReason`을 기반으로 context_limit/rate_limit/auth/service 등 원인별 재시도·모델 전환 정책 적용.

---

## 8. 설정·경로·환경변수

### 8.1 `~/.hermes/` 파일 트리
```
~/.hermes/
├── config.yaml        전역 설정
├── state.db           SQLite 세션/메시지 DB (+FTS5)
├── memory.json        내장 메모리 저장
├── skills/            사용자 학습 스킬
├── plugins/           사용자 플러그인
├── credentials/       암호화된 API 키
├── logs/              활동 로그
└── .env               환경 오버라이드
```

### 8.2 `config.yaml` 스키마 (요약)
```yaml
model: openrouter:nous-hermes-2
provider_config:
  openrouter: {api_key: sk-...}
  openai:     {api_key: sk-...}
  anthropic:  {api_key: sk-...}
tools:
  enabled:  [web, terminal, file, vision, browser]
  disabled: []
skills:  {enabled: true, auto_create: true, search: true}
memory:  {provider: builtin, enabled: true}
gateway:
  telegram: {token: ...}
  discord:  {token: ...}
  slack:    {token: ...}
mcp_servers: [...]
```

### 8.3 주요 환경 변수
`HERMES_HOME`, `HERMES_QUIET`, `OPENROUTER_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `HASS_TOKEN`, `HASS_URL`, `MCP_SERVERS`.

### 8.4 프로젝트 수준 컨텍스트
- `cli-config.yaml` — 프로젝트 설정 오버라이드
- `SOUL.md` — 에이전트 성격 오버라이드 (주입 스캔 대상)
- `.hermes.md` / `AGENTS.md` — 자동 주입되는 컨텍스트 파일

---

## 9. 저장소 통계 (요약)

| 컴포넌트 | 위치 | 역할 | LOC |
|---|---|---|---|
| CLI UI | `cli.py` | 대화형 터미널 | 9,185 |
| 에이전트 코어 | `run_agent.py` | 오케스트레이션 | 9,845 |
| 도구 시스템 | `tools/registry.py`, `model_tools.py` | discovery/dispatch | 2,000+ |
| 에이전트 유틸 | `agent/` (24 파일) | 메모리·프롬프트·에러 등 | 5,000+ |
| CLI 명령 | `hermes_cli/` (44 파일) | 명령·설정·인증 | 8,000+ |
| 도구 구현 | `tools/` (53+ 파일) | 개별 도구 | 15,000+ |
| 실행 환경 | `tools/environments/` | 샌드박스 백엔드 | 3,000+ |
| 게이트웨이 | `gateway/` (11 모듈) | 메신저 | 3,500+ |
| ACP 어댑터 | `acp_adapter/` | Copilot 프로토콜 | 1,500+ |
| 스킬 | `skills/` + `optional-skills/` | 절차적 기억 | ─ |
| RL 환경 | `environments/` | 학습 환경 | 2,500+ |
| 테스트 | `tests/` | 검증 | 5,000+ |

---

## 10. 확장 포인트 (Extension Points)

1. **커스텀 도구** — `~/.hermes/plugins/my_tool.py`에서 `tools.registry.register(...)` 호출.
2. **메모리 공급자** — `MemoryProvider` ABC 상속 후 `config.yaml`에 등록.
3. **스킬** — `~/.hermes/skills/my-skill/SKILL.md` 작성 (YAML frontmatter 선택).
4. **게이트웨이 플랫폼** — `gateway/platforms/base.Platform` 상속 후 핸들러 등록.
5. **MCP 서버** — 독립 프로세스로 실행 후 `mcp_servers` 설정으로 연결.
6. **실행 환경** — `tools/environments/base.ExecutionEnvironment` 상속.
7. **브라우저 제공자** — `tools/browser_providers/base.py` 상속.

---

## 11. 주목할 만한 기능

- **다중 공급자 모델** — OpenRouter(200+ 모델), OpenAI, Anthropic(thinking+캐싱), Ollama, vLLM, 스마트 라우팅.
- **6+ 실행 환경** — local/docker/ssh/modal/daytona/singularity, 각 세션 영속 가능.
- **멀티 브라우저 백엔드** — Playwright, Browserbase, Firecrawl, BrowserUse, CamoFox.
- **자가개선 스킬 시스템** — 복잡한 태스크에서 스킬 자동 생성, FTS5로 검색.
- **멀티플랫폼 게이트웨이** — Telegram/Discord/Slack/WhatsApp/Signal/Email/Matrix를 단일 데몬으로.
- **RL 인프라** — 트래젝토리 압축, Atropos 환경, Mini-SWE 러너.
- **MCP 양방향** — 외부 MCP 도구 임포트 + Hermes 도구를 MCP로 노출.
- **프롬프트 캐싱** — Anthropic 캐시 제어로 캐시된 토큰 90% 할인.
- **FTS5 세션 검색** — 모든 과거 대화를 자연어로 검색.

---

## 12. 검증·감사 노트

본 코드맵은 저장소 전수 탐색(디렉토리 트리, 핵심 파일 리딩, import 그래프 분석)을 통해 작성되었으며 아래 관점에서 재검토되었다.

1. **진입점 누락 확인** — `cli.py`, `run_agent.py`, `batch_runner.py`, `mcp_serve.py`, `mini_swe_runner.py`, `gateway/run.py`, `rl_cli.py`, `acp_adapter/entry.py` 8개 모두 반영.
2. **도구 카테고리 누락 확인** — web/terminal/file/browser/vision/code/skills/memory/search/todo/delegate/image_gen/tts/cron/mcp/homeassistant/MoA/send_message/clarify/approval/process 21개 카테고리 반영.
3. **환경 백엔드 누락 확인** — local/docker/ssh/modal/managed_modal/daytona/singularity 7개 반영.
4. **게이트웨이 플랫폼 누락 확인** — telegram/discord/slack/whatsapp/signal/email/matrix 7개 반영.
5. **데이터 흐름 일관성** — 턴 준비 → LLM 호출 → 툴 루프 → 마무리의 4단계가 `run_agent.py`의 실제 루프 구조와 일치.
6. **순환 의존 주의** — `tools/registry.py`가 도구 모듈을 import 하지 않는 패턴 재확인.

상세 모듈별 설명은 `codewiki.md`를 참조.
