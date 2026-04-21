# Hermes Agent — 코드위키 (codewiki)

> 저장소: **NousResearch/hermes-agent**
> 본 문서는 주요 모듈/파일/클래스/함수를 **위키 스타일**로 정리한 상세 레퍼런스이다.
> 전체 아키텍처 조망은 동봉된 `codemaps.md`를 먼저 참고할 것을 권장한다.

문서는 다음 순서로 구성된다.

1. [프로젝트 루트 파일](#1-프로젝트-루트-파일)
2. [`agent/` — 에이전트 코어 유틸](#2-agent--에이전트-코어-유틸)
3. [`tools/` — 도구 시스템](#3-tools--도구-시스템)
4. [`tools/environments/` — 실행 환경 백엔드](#4-toolsenvironments--실행-환경-백엔드)
5. [`tools/browser_providers/` — 브라우저 백엔드](#5-toolsbrowser_providers--브라우저-백엔드)
6. [`hermes_cli/` — CLI 애플리케이션 레이어](#6-hermes_cli--cli-애플리케이션-레이어)
7. [`gateway/` — 멀티플랫폼 게이트웨이](#7-gateway--멀티플랫폼-게이트웨이)
8. [`acp_adapter/` — Anthropic Copilot Protocol 어댑터](#8-acp_adapter--anthropic-copilot-protocol-어댑터)
9. [`skills/` — 절차적 기억 라이브러리](#9-skills--절차적-기억-라이브러리)
10. [설정·저장소·보안](#10-설정저장소보안)
11. [자주 쓰는 사용 예시](#11-자주-쓰는-사용-예시)
12. [검증·감사 메모](#12-검증감사-메모)

---

## 1. 프로젝트 루트 파일

### 1.1 `cli.py` — 대화형 터미널 UI
- **규모**: ≈9,185 LOC
- **역할**: `prompt_toolkit` 기반의 고정 입력창 TUI, 스트리밍 응답 렌더링, 슬래시 명령 자동완성, 인터럽트 처리.
- **핵심 구성요소**
  - `KeyBindings` — 입력창 단축키와 `/명령` 자동완성.
  - `FileHistory` — 명령/입력 히스토리 영속화.
  - 설정 로더 — `~/.hermes/config.yaml` 또는 프로젝트의 `./cli-config.yaml`.
  - 실시간 스피너·도구 프리뷰 · 대화 중단(`Ctrl-C`) 처리.
- **진입**: `hermes_cli/main.py`가 본 파일의 `main()`을 호출한다.
- **상호작용**: 사용자 입력이 들어오면 `run_agent.AIAgent.run_conversation()`을 호출한다.

### 1.2 `run_agent.py` — 에이전트 오케스트레이터
- **규모**: ≈9,845 LOC (저장소에서 가장 큰 파일)
- **주요 클래스**: `AIAgent`
- **핵심 메서드**
  - `run_conversation(user_input: str) -> str` — 턴 준비, 모델 호출, 툴 콜 루프, 마무리까지 전 과정 수행.
  - 내부 단계: (1) 세션/히스토리 로드, (2) `MemoryManager.prefetch_all`, (3) `prompt_builder.build_system_prompt`, (4) `context_compressor.compress`, (5) LLM 호출, (6) 툴 콜 루프, (7) `HermesState.save_message`, (8) `MemoryManager.sync_all`.
- **담당 업무**
  - 대화 상태 관리 · 모델 공급자 디스패치 · 프롬프트 캐싱 통합.
  - 토큰·비용 트래킹 (`usage_pricing`) · 에러 분류 및 페일오버.
  - 툴 콜 오케스트레이션 (`model_tools.handle_function_call`).
  - 서브에이전트 위임(ThreadPoolExecutor 기반).
- **주의점**: 이벤트 루프 관리를 위해 스레드별 영속 루프 전략을 사용하며, `httpx` / `AsyncOpenAI` 클라이언트는 캐시된 상태로 재사용된다.

### 1.3 `model_tools.py` — 도구 discovery·dispatch 파사드
- **주요 함수**
  - `get_tool_definitions(enabled_toolsets, disabled_toolsets) -> list[dict]` — OpenAI 호환 스키마 반환.
  - `handle_function_call(tool_name, args, task_id, user_task) -> str` — 실제 실행 엔트리.
  - `get_toolset_for_tool(tool_name) -> str` — 역조회.
  - `check_toolset_requirements() -> dict` — 의존성 검사.
- **특징**: 본 파일 import 시점에 모든 `tools/*` 모듈을 선로드해 레지스트리를 채우고 비동기 브리징 헬퍼(`_run_async`)를 제공한다.

### 1.4 `toolsets.py`
- 도구들을 `toolset` 단위로 묶어(`web`, `terminal`, `browser`, `skills` 등) 활성/비활성 토글을 가능하게 한다.
- `resolve_toolset(name)` — 토큰 매칭, 별칭, 기본값 결정.

### 1.5 `hermes_state.py` — SQLite 영속 저장소
- **테이블**
  - `sessions`(id, source, user_id, model, system_prompt, parent_session_id, started_at, ended_at, message/tool 카운트, 토큰/비용 트래킹, title)
  - `messages`(id, session_id, role, content, tool_calls, tool_name, timestamp, token_count, finish_reason, reasoning)
  - `messages_fts` — SQLite FTS5 가상 테이블 (풀텍스트 검색 인덱스)
- **주요 메서드**: `create_session`, `save_message`, `load_messages`, `search_sessions_fts`, `end_session`.

### 1.6 기타 루트 파일
- `hermes_constants.py` — 경로/기본값 상수.
- `hermes_logging.py` — 표준 로깅 설정.
- `hermes_time.py` — 시간·타임존 유틸.
- `utils.py` — 범용 유틸 (문자열, 파일, 해시 등).
- `batch_runner.py` — RL 트래젝토리 배치 생성기, YAML config 입력 → `.jsonl` 출력.
- `mcp_serve.py` — Hermes 도구를 MCP(Model Context Protocol)로 노출.
- `mini_swe_runner.py` — SWE 전용 에이전트 러너 (레포 패치 태스크).
- `rl_cli.py` — 강화학습 워크플로 CLI.
- `trajectory_compressor.py` — RL 트래젝토리 압축 로직.

---

## 2. `agent/` — 에이전트 코어 유틸

### 2.1 `memory_manager.py`
- **클래스**: `MemoryManager`
- **책임**: 내장 메모리 공급자 + 최대 1개의 외부 공급자를 통합 관리.
- **주요 메서드**
  - `add_provider(provider: MemoryProvider)` — 공급자 등록.
  - `build_system_prompt() -> str` — 시스템 프롬프트에 메모리 블록 삽입.
  - `prefetch_all(user_message)` — 턴 시작 시 기억 회상.
  - `sync_all(user_msg, assistant_response)` — 턴 종료 후 기억 저장.
  - `handle_tool_call(name, args)` — 메모리 관련 툴 콜 라우팅.
- **안전장치**: 회상된 메모리는 `<memory-context>` 펜스로 감싸 사용자 입력과 분리.

### 2.2 `memory_provider.py`
- **추상 베이스**: `MemoryProvider`
- **요구 메서드**: `get_tool_schemas`, `prefetch`, `sync`, `handle_tool_call`.
- 외부 공급자(예: Honcho)는 이 인터페이스를 구현해 `plugins/`에 배치.

### 2.3 `builtin_memory_provider.py`
- JSON 파일 기반 내장 메모리 (`~/.hermes/memory.json`).
- 외부 의존성 없이 기본 동작 보장.

### 2.4 `prompt_builder.py`
- **상수**: `DEFAULT_AGENT_IDENTITY`, `MEMORY_GUIDANCE`, `SKILLS_GUIDANCE`, `TOOL_USE_ENFORCEMENT_GUIDANCE`.
- **주요 함수**
  - `build_system_prompt(...)` — 정체성 + 메모리 + 스킬 인덱스 + 컨텍스트 파일을 결합.
  - `_scan_context_content(text)` — SOUL.md/.cursorrules 등의 주입 공격 탐지(정규식 + 제로폭 문자 검사).
  - `load_soul_md()` — 성격 오버라이드 로더.
  - `build_skills_system_prompt()` — 스킬 인덱스 생성.
  - `build_context_files_prompt()` — `.hermes.md`, `AGENTS.md` 결합.

### 2.5 `context_compressor.py`
- **클래스**: `ContextCompressor`
- **전략**: 토큰 추정 → 가장 오래된 비-툴콜 메시지부터 드랍. 최근 툴 상호작용은 우선 보존.
- `compress(messages, max_tokens, model) -> list[dict]`.

### 2.6 `model_metadata.py`
- `fetch_model_metadata(provider, model)` — 컨텍스트 한계, 입·출력 단가.
- `estimate_tokens_rough(messages)` — 빠른 토큰 추정.
- `query_ollama_num_ctx()` — 로컬 Ollama 컨텍스트 감지.
- `save_context_length()` — 발견된 값 캐시.

### 2.7 `error_classifier.py`
- **열거형**: `FailoverReason` (context_limit / rate_limit / auth_error / service_error / model_unavailable 등)
- `classify_api_error(status, message) -> FailoverReason` — 예외/메시지를 분류하여 재시도·페일오버 의사결정에 사용.

### 2.8 `usage_pricing.py`
- **데이터클래스**: `CanonicalUsage` (input/output/cache/reasoning 토큰).
- `estimate_usage_cost(provider, model, tokens)` — 비용 추정.
- `format_token_count_compact()`, `format_duration_compact()` — 표시 헬퍼.

### 2.9 `trajectory.py`
- `convert_scratchpad_to_think()` — Claude thinking 태그 변환.
- `save_trajectory_to_file()` — RL 훈련 데이터 저장.

### 2.10 `skill_utils.py`
- `get_all_skills_dirs()` — 스킬 경로 탐색 (번들 → 프로젝트 → 사용자).
- `iter_skill_index_files()` — `SKILL.md` 목록.
- `parse_frontmatter()` — YAML frontmatter 파서.
- `extract_skill_description()` — 스킬 요약 추출.
- `skill_matches_platform()` — 플랫폼·도구 조건 필터링.

### 2.11 `credential_pool.py`
- 환경 변수 / credential 파일 / `~/.hermes/credentials/` / OS 키체인 등을 순차적으로 조회해 API 키를 제공.

### 2.12 `retry_utils.py`
- `jittered_backoff(attempt, base_delay)` — 지수 백오프 + 지터.

### 2.13 `smart_model_routing.py`
- 비용·지연시간·가용성·기능(비전, 툴 호출 지원 등)을 매칭해 태스크에 적합한 모델 자동 선택.

### 2.14 `anthropic_adapter.py`
- Anthropic Claude 전용 어댑터. 프롬프트 캐시 제어, 비전, thinking, `max_tokens` 관리.

### 2.15 `auxiliary_client.py`
- OpenAI/호환 공급자(OpenRouter, Groq, vLLM 등)에 대한 HTTP 클라이언트 관리. 영속 이벤트 루프에 바인딩된 `AsyncOpenAI` 재사용.

### 2.16 기타 모듈
| 모듈 | 역할 |
|---|---|
| `display.py` | 터미널 포맷팅, 스피너, 도구 프리뷰 렌더링 |
| `title_generator.py` | 세션 제목 자동 생성 |
| `insights.py` | 사용량 분석 (세션별 통계) |
| `redact.py` | 로그에서 민감정보 마스킹 |
| `rate_limit_tracker.py` | 공급자별 레이트리밋 추적 |
| `subdirectory_hints.py` | 하위 디렉토리별 컨텍스트 힌트 |
| `prompt_caching.py` | Anthropic `cache_control` 적용 |
| `context_references.py` | `.hermes.md` 참조 관리 |
| `copilot_acp_client.py` | Copilot ACP 클라이언트 스텁 |
| `models_dev.py` | 개발용 모델 설정 |
| `skill_commands.py` | 스킬 관련 CLI 명령 로직 |

---

## 3. `tools/` — 도구 시스템

### 3.1 `tools/registry.py` — 중앙 레지스트리
- **클래스**: `ToolRegistry` (싱글턴), `ToolEntry`
- `ToolEntry(name, toolset, schema, handler, check_fn, is_async, ...)`
- **주요 메서드**
  - `register(name, toolset, schema, handler, check_fn=None, ...)` — 도구 등록 (각 도구 모듈이 import 시점에 호출).
  - `get_tool_definitions(enabled, disabled)` — 필터링된 스키마 리스트.
  - `dispatch(tool_name, args)` — 가용성 확인 후 handler 실행.
  - `get_tool_to_toolset_map()` — 역매핑.
- **순환 방지**: 본 파일은 `tools/*`의 어떤 구체 도구도 import 하지 않는다.

### 3.2 웹 도구 — `web_tools.py`
- `web_search(query, max_results)` — 검색엔진 질의.
- `web_extract(url)` — URL 본문 추출 (Readability/커스텀 파서).

### 3.3 터미널 도구 — `terminal_tool.py`
- `terminal(command, env_id=None, timeout=...)` — 활성 실행 환경에서 셸 명령 실행.
- 6+ 백엔드를 지원: local / docker / ssh / modal / daytona / singularity.
- 영속 세션(cwd·환경변수 유지) 지원.

### 3.4 파일 도구 — `file_tools.py`
- `read_file(path, offset, limit)`, `write_file(path, content)`, `patch(path, diff)`, `search_files(pattern, glob)`.
- 프로젝트 루트를 베이스로 경로 샌드박싱.

### 3.5 브라우저 도구 — `browser_tool.py`, `browser_camofox.py`
- `browser_navigate(url)`, `browser_snapshot()`, `browser_click(selector)`, `browser_type(selector, text)`, `browser_scroll`, `browser_vision`, `browser_console(js)`.
- 백엔드 프로바이더(`tools/browser_providers/`) 중 하나를 선택.
- `browser_camofox.py` — anti-detection 모드.

### 3.6 비전 도구 — `vision_tools.py`
- `vision_analyze(image_path, question)` — 멀티모달 모델 호출로 이미지 이해.

### 3.7 코드 실행 — `code_execution_tool.py`
- `execute_code(language, code)` — 실행 환경 위에서 Python/셸 코드 실행.

### 3.8 스킬 도구 — `skills_tool.py`
- `skills_list`, `skill_view(name)`, `skill_manage(action, ...)`.
- `skill_utils` 기반으로 스킬 discovery.

### 3.9 메모리 도구 — `memory_tool.py`
- 실제 로직은 `MemoryManager.handle_tool_call`에 위임.

### 3.10 세션 검색 — `session_search_tool.py`
- `session_search(query)` — `HermesState.search_sessions_fts`로 FTS5 검색.

### 3.11 Todo 도구 — `todo_tool.py`
- 인메모리·SQLite 혼합 할일 관리(`todo create/update/list/complete`).

### 3.12 위임 도구 — `delegate_tool.py`
- `delegate_task(prompt, toolset=...)` — 격리된 서브에이전트 spawn.
- `ThreadPoolExecutor` + 전용 이벤트 루프에서 실행, 결과만 부모 에이전트로 반환.

### 3.13 이미지 생성 — `image_generation_tool.py`
- 제공자: Fal, Replicate, 로컬 Stable Diffusion.

### 3.14 TTS — `tts_tool.py`
- 제공자: Google TTS, OpenAI TTS, NeuTTS, 로컬 TTS.

### 3.15 크론 — `cronjob_tools.py`
- 크론 생성/수정/삭제/목록. SQLite 영속, 데몬이 실행.

### 3.16 MCP 도구 — `mcp_tool.py`
- `discover_mcp_tools()` — 외부 MCP 서버의 툴 스키마 동적 임포트.
- 각 서버 툴을 레지스트리에 프록시 등록.

### 3.17 Home Assistant — `homeassistant_tool.py`
- `ha_list_entities`, `ha_get_state`, `ha_call_service`. `HASS_TOKEN`·`HASS_URL` 필요.

### 3.18 Mixture of Agents — `mixture_of_agents_tool.py`
- `mixture_of_agents(prompt, models=[...])` — 여러 모델에 동시 쿼리 후 앙상블 합성.

### 3.19 메시징 — `send_message_tool.py`
- 플랫폼 독립적인 `send_message(platform, target, text)`. 게이트웨이로 릴레이.

### 3.20 Clarify — `clarify_tool.py`
- `clarify(question, options?)` — 사용자에게 되묻기.

### 3.21 승인 — `approval.py`
- 민감 명령(파일 삭제, 외부 결제 등) 실행 전 확인 절차. DM 페어링으로 원격 승인 가능.

### 3.22 프로세스 — `process_registry.py`
- 실행 중인 서브프로세스의 PID/상태 추적, graceful shutdown.

---

## 4. `tools/environments/` — 실행 환경 백엔드

### 4.1 `base.py`
- **추상 클래스**: `ExecutionEnvironment`
- 메서드: `start`, `stop`, `run_command`, `upload_file`, `download_file`, `is_alive` 등.

### 4.2 백엔드 목록
| 파일 | 백엔드 | 요약 |
|---|---|---|
| `local.py` | 호스트 셸 | 개발용 기본값 |
| `docker.py` | Docker 컨테이너 | 이미지 지정, 볼륨 마운트 |
| `ssh.py` | 원격 SSH | 키 인증 기반 원격 머신 |
| `modal.py` | Modal serverless | GPU/CPU 동적 할당 |
| `managed_modal.py` | 영속 Modal | 세션 영속화 레이어 |
| `daytona.py` | Daytona 워크스페이스 | IDE 연동 |
| `singularity.py` | Singularity | HPC 클러스터 |
| `modal_utils.py` | Modal 공용 유틸 | 이미지·볼륨 헬퍼 |

각 백엔드는 **타임아웃·리소스 한도·파일 전송**을 공통 API로 제공한다.

---

## 5. `tools/browser_providers/` — 브라우저 백엔드

| 파일 | 제공자 | 특징 |
|---|---|---|
| `base.py` | 추상 인터페이스 | `navigate/click/type/snapshot/close` |
| `browser_use.py` | BrowserUse | 에이전틱 브라우저 제어 |
| `browserbase.py` | Browserbase | 클라우드 호스팅 Chromium |
| `firecrawl.py` | Firecrawl | AI 기반 스크래핑/크롤링 |
| (Playwright) | 로컬 | `browser_tool.py`가 직접 임포트 |
| `browser_camofox.py` | Camoufox | anti-detection (stealth) |

---

## 6. `hermes_cli/` — CLI 애플리케이션 레이어

### 6.1 `main.py`
- `hermes` 커맨드의 실제 엔트리. `fire.Fire()`로 서브커맨드 라우팅.
- 호출 가능: `hermes`, `hermes model`, `hermes gateway start`, `hermes doctor`, `hermes cron …` 등.

### 6.2 `commands.py`
- 슬래시 명령 구현: `/new`, `/reset`, `/model`, `/skills`, `/retry`, `/cost`, `/memory`, `/tools` 등.

### 6.3 `config.py`
- `~/.hermes/config.yaml` 로드, CLI 기본값 병합, 환경 변수 오버라이드.

### 6.4 `auth.py` / `auth_commands.py`
- 각 공급자/플랫폼 OAuth 및 토큰 저장. 로그인/로그아웃 플로우.

### 6.5 `model_switch.py` / `models.py`
- 인터랙티브 모델 선택 UI, 공급자 discovery(OpenRouter, OpenAI, Anthropic, z.ai, Groq 등).

### 6.6 `doctor.py`
- 시스템 헬스 체크: 의존성, 환경변수, API 키 유효성, 스킬 디렉토리, DB 무결성.

### 6.7 `gateway.py`
- 게이트웨이 설정 마법사. 플랫폼 토큰 입력, 데몬 시작/종료.

### 6.8 `curses_ui.py`
- `curses` 기반 보조 UI 컴포넌트 (리스트 선택, 모달 등).

### 6.9 `callbacks.py`
- 대화 이벤트(토큰 수신, 툴 시작/종료 등) 콜백 훅.

### 6.10 `pairing.py`
- DM 페어링 (명령 승인용) — 로컬 세션과 메신저 사용자를 링크.

### 6.11 `plugins.py`
- 플러그인 discovery/loader: `~/.hermes/plugins/`, 프로젝트 `./.hermes/plugins/`, pip 패키지.

### 6.12 `mcp_config.py`
- MCP 서버 설정 파일 읽기/쓰기.

### 6.13 `cron.py`
- 크론잡 CLI 명령.

### 6.14 `banner.py`
- ASCII 배너, 버전 정보 렌더링.

---

## 7. `gateway/` — 멀티플랫폼 게이트웨이

### 7.1 `run.py`
- 게이트웨이 오케스트레이터. `hermes gateway start`에서 호출.
- 플랫폼 커넥터 초기화, 인바운드 큐 처리, 세션 라이프사이클 관리.

### 7.2 `session.py`
- 플랫폼 사용자 ↔ 에이전트 태스크 바인딩. 멀티턴 상태 유지.

### 7.3 `config.py`
- 게이트웨이별 설정 (플랫폼 토큰, 기본 모델, 승인 정책 등).

### 7.4 `delivery.py`
- 아웃바운드 메시지 전송. 긴 응답 청킹·스레딩·미디어 업로드 처리.

### 7.5 `pairing.py`
- 플랫폼 사용자 식별 및 승인용 DM 페어링.

### 7.6 `status.py`
- 게이트웨이 실행 상태, 플랫폼별 연결 상태.

### 7.7 `hooks.py`
- 이벤트 훅 시스템: `before_tool_call`, `after_tool_call`, `on_message_received` 등.
- `builtin_hooks/` 하위에 기본 로깅/분석 훅 구현.

### 7.8 `channel_directory.py`
- 채널/그룹 레지스트리: 이름·ID·설정 매핑.

### 7.9 `platforms/`
| 파일 | 플랫폼 | SDK |
|---|---|---|
| `telegram.py` | Telegram | Telegram Bot API |
| `discord.py` | Discord | discord.py |
| `slack.py` | Slack | Slack Bolt |
| `whatsapp.py` | WhatsApp | Business API |
| `signal.py` | Signal | signal-cli |
| `email.py` | 이메일 | SMTP/IMAP |
| `matrix.py` | Matrix.org | matrix-nio |

모든 플랫폼 핸들러는 공통 `Platform` 인터페이스(`async send_message`, `async receive_messages`)를 구현한다.

---

## 8. `acp_adapter/` — Anthropic Copilot Protocol 어댑터

Hermes를 Copilot ACP(Agent Coding Protocol) 호환 에이전트로 노출한다.

| 파일 | 역할 |
|---|---|
| `entry.py` | 진입점 (ACP 서버 실행) |
| `server.py` | JSON-RPC 서버 구현 |
| `auth.py` | 인증 핸들링 |
| `session.py` | ACP 세션 관리 |
| `events.py` | 이벤트 스트리밍 |
| `tools.py` | Hermes 도구를 ACP 툴 스키마로 변환 |
| `permissions.py` | 파일/명령 권한 검사 |

---

## 9. `skills/` — 절차적 기억 라이브러리

- 번들 스킬(28+ 카테고리) + `optional-skills/`(16 팩) + 사용자 스킬(`~/.hermes/skills/`).
- 각 스킬은 `SKILL.md` 파일 하나로 표현되며, YAML frontmatter로 메타(플랫폼 조건, 필요 도구 등)를 선언할 수 있다.
- **자가개선**: 복잡한 태스크 수행 중 에이전트가 새 스킬을 자동 생성하거나 기존 스킬을 개선할 수 있다.
- **탐색 순서**: `~/.hermes/skills/` → 프로젝트 `./.hermes/skills/` → 번들 `skills/`.
- **agentskills.io 호환**: 오픈 표준을 지향한다.

---

## 10. 설정·저장소·보안

### 10.1 `~/.hermes/` 디렉토리
`config.yaml`, `state.db`, `memory.json`, `skills/`, `plugins/`, `credentials/`, `logs/`, `.env`.

### 10.2 주요 환경 변수
`HERMES_HOME`, `HERMES_QUIET`, `OPENROUTER_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `HASS_TOKEN`, `HASS_URL`, `MCP_SERVERS`.

### 10.3 보안 장치
- **자격증명 관리**: 환경변수 → 파일 → 키체인 우선순위.
- **명령 승인**: 민감 커맨드에 대해 DM 또는 터미널에서 확인, 모든 명령은 `state.db`에 감사 로깅.
- **샌드박스 격리**: 터미널 실행은 선택된 environment 백엔드에서 격리.
- **프롬프트 인젝션 방어**: `_scan_context_content`가 SOUL.md/.hermes.md 주입 패턴·제로폭 유니코드 탐지.
- **메모리 펜싱**: `<memory-context>` 블록으로 기억과 사용자 지시를 분리.

### 10.4 에러 핸들링
| 분류 | 대응 |
|---|---|
| context_limit | `ContextCompressor`로 히스토리 축소 |
| rate_limit | `jittered_backoff`로 재시도 |
| auth_error | 사용자에게 자격증명 재입력 요청 |
| service_error | 백오프 재시도 |
| model_unavailable | `smart_model_routing`으로 대체 모델 페일오버 |

---

## 11. 자주 쓰는 사용 예시

### 11.1 설치 및 첫 실행
```bash
pip install hermes-agent
hermes doctor        # 환경 점검
hermes               # 대화형 TUI 실행
```

### 11.2 커스텀 도구 등록
```python
# ~/.hermes/plugins/my_tool.py
from tools.registry import registry

def handler(args):
    return f"Hello, {args['name']}"

registry.register(
    name="hello",
    toolset="custom",
    schema={
        "name": "hello",
        "description": "Greets a person",
        "parameters": {
            "type": "object",
            "properties": {"name": {"type": "string"}},
            "required": ["name"],
        },
    },
    handler=handler,
    check_fn=lambda: True,
)
```

### 11.3 게이트웨이 시작 (Telegram)
```bash
hermes gateway setup          # 토큰 입력 마법사
hermes gateway start --platform telegram
```

### 11.4 크론잡 생성
```
/cron add "매일 오전 9시 일정 요약" "0 9 * * *"
```

### 11.5 스킬 작성
```markdown
<!-- ~/.hermes/skills/morning-briefing/SKILL.md -->
---
name: morning-briefing
platforms: [cli, telegram]
tools: [web_search, send_message]
---

# 아침 브리핑

1. 최신 뉴스 3건을 검색한다.
2. 사용자 캘린더 일정을 확인한다.
3. 결과를 마크다운 블록으로 정리한다.
```

### 11.6 세션 FTS5 검색
```
/search "프로덕션 배포 이슈"
```

---

## 12. 검증·감사 메모

본 위키는 다음 감사 루프를 거쳐 작성되었다.

1. **파일 커버리지 체크** — 루트·`agent/`·`tools/`·`tools/environments/`·`tools/browser_providers/`·`hermes_cli/`·`gateway/`·`acp_adapter/`·`skills/` 전 디렉토리를 1회 이상 순회.
2. **클래스·함수 이름 재확인** — `MemoryManager`, `ContextCompressor`, `ToolRegistry`, `ToolEntry`, `ExecutionEnvironment`, `FailoverReason`, `CanonicalUsage` 등 주요 식별자가 실제 파일과 1:1 매칭되는지 확인.
3. **툴 카테고리 누락 점검** — 21개 카테고리(web/terminal/file/browser/vision/code/skills/memory/search/todo/delegate/image_gen/tts/cron/mcp/homeassistant/MoA/send_message/clarify/approval/process) 반영.
4. **환경 백엔드 누락 점검** — local/docker/ssh/modal/managed_modal/daytona/singularity 7개 반영.
5. **게이트웨이 플랫폼 누락 점검** — telegram/discord/slack/whatsapp/signal/email/matrix 7개 반영.
6. **보안 항목** — 자격증명/승인/샌드박스/주입 방어/메모리 펜싱 5개 항목 반영.
7. **데이터 흐름 상호참조** — `codemaps.md` §5 턴 루프와 본 문서의 `run_agent.py` 설명이 일치함을 확인.
8. **불확실성 표기 원칙** — 파일 내부의 구체 함수 시그니처 중 문서화가 불완전할 수 있는 부분은 "요약" 또는 "주요 메서드" 단위로 기재하여 과장 서술을 방지.

추가로 필요한 상세 정보(개별 함수 시그니처 · 테스트 케이스 · RL 환경 구조 등)는 실제 저장소의 `tests/`와 `environments/`를 별도로 참고할 것을 권장한다.

---

**끝.** 본 위키와 `codemaps.md`를 함께 참조하면 Hermes Agent 저장소 전체 구조와 세부 모듈을 모두 파악할 수 있다.
