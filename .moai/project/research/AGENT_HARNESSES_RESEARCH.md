# AgentOS 에이전트 하네스 설계 연구
## 기존 에이전트 런타임 및 하네스 상세 분석

**작성 일자**: 2026년 4월 10일  
**목적**: AgentOS 데스크톱 에이전트 런타임 설계를 위한 기존 에이전트 하네스 시스템 분석

---

## 1. Hermes Agent (NousResearch)

### 개요
NousResearch에서 개발한 Hermes Agent는 지속적인 메모리, 스킬 시스템, 다중 플랫폼 메시징 게이트웨이를 갖춘 오픈소스 자율 에이전트입니다.

### 아키텍처 결정사항

**핵심 컴포넌트:**
- **AIAgent 클래스**: 중앙 집중식 오케스트레이션 계층으로, LLM 상호작용, 도구 실행, 상태 관리를 담당
- **다중 인터페이스**: CLI, Gateway(메시징 플랫폼), ACP(에디터 통합), Cron(예약 작업)
- **도구 레지스트리 패턴**: 임포트 시점에서의 자체 등록으로 느슨한 결합 구현
- **5개 메모리 계층**: 계층적 메모리 아키텍처로 유연성 제공
- **6개 실행 백엔드**: 다양한 실행 환경 지원

**상태 관리:**
- SQLite 기반 세션/상태 데이터베이스 (FTS5 전문 검색 포함)
- 계층화된 프롬프트 캐싱으로 토큰 최적화
- `hermes_state.py`를 통한 상태 지속성 관리

### 주요 아키텍처 특징

1. **중앙집중식 오케스트레이션**: 단일 AIAgent 클래스가 모든 상호작용 조정
2. **도구 시스템 인프라**: 자동 발견 및 디스패치로 확장성 확보
3. **보안-우선 설계**: 격리된 실행 환경과 플러그형 백엔드
4. **다층 프롬프팅**: 프롬프트 빌더, 컨텍스트 압축기, 프롬프트 캐싱 포함
5. **스킬 시스템**: 포탈 스킬을 통한 기능 확장

### 강점
- 광범위한 백엔드 지원(6개)으로 유연한 배포
- 포탄 플랫폼 지원(CLI, 메시징, 에디터, Cron)
- 체계적인 상태 관리 및 세션 추적
- 상대적으로 간단한 아키텍처

### 약점
- 학습 곡선: 6개 백엔드와 5개 메모리 계층으로 인한 복잡성
- 오픈소스 생태계 내 사용자 기반이 상대적으로 제한적

### 참고자료
- [Hermes Agent 아키텍처](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture/)
- [DEV Community: Hermes Agent 분석](https://dev.to/crabtalk/hermes-agent-what-nous-research-built-m5b)
- [GitHub: NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent)

---

## 2. OpenAI Codex CLI

### 개요
OpenAI의 Codex CLI는 로컬 터미널에서 실행되는 코딩 에이전트로, Rust로 작성되어 빠른 성능을 제공합니다.

### 아키텍처 결정사항

**보안 샌드박싱:**
- macOS seatbelt 및 Linux Landlock 기본 제공 샌드박스
- 선택적 bubblewrap 파이프라인
- 위험한 바이패스 옵션(외부 강화 환경에서만 사용 권장)

**인증 메커니즘:**
- ChatGPT OAuth 지원
- 디바이스 인증
- stdin을 통한 API 키 파이핑
- MCP OAuth 로그인을 위한 선택적 고정 포트

**실행 모델:**
- 사용자가 지정한 디렉토리 내에서 임의의 명령 실행 가능
- 파일 및 네트워크 접근 제어를 통한 정책 기반 샌드박싱

### 강점
- Rust 기반의 높은 성능
- 내장된 샌드박싱으로 보안 제공
- ChatGPT 생태계와의 통합
- 간단한 로컬 실행 모델

### 약점
- 공개된 아키텍처 정보 부족
- 커스터마이징 옵션 제한적
- 오픈소스가 아닌 상용 서비스

### 참고자료
- [Codex CLI 명령줄 옵션](https://developers.openai.com/codex/cli/reference)
- [Codex 인증](https://developers.openai.com/codex/auth)
- [Codex MCP 통합](https://developers.openai.com/codex/mcp)

---

## 3. Anthropic Claude Code

### 개요
Claude Code는 서브에이전트 모델, 슬래시 커맨드, 스킬, MCP 통합을 갖춘 코드 작업을 위한 플랫폼입니다.

### 아키텍처 결정사항

**서브에이전트 모델:**
- 각 서브에이전트는 독립적인 컨텍스트 윈도우 실행
- 커스텀 시스템 프롬프트 및 도구 접근 제어
- 주요 대화 흐름에서 자동 위임
- 무한 중첩 방지(서브에이전트는 다른 서브에이전트 생성 불가)

**MCP 통합:**
- Model Context Protocol 표준화로 AI 도구 통합
- mcpServers 필드를 통한 MCP 서버 연결
- 인라인 서버는 서브에이전트 시작 시 연결, 종료 시 해제

**도구 접근 제어:**
- 기본적으로 부모 세션의 모든 도구 상속
- 도구 필드(허용 목록) 또는 disallowedTools 필드(거부 목록)를 통한 제한
- 플러그인 서브에이전트는 보안상 hooks, mcpServers, permissionMode 미지원

**스킬 vs 서브에이전트 vs MCP:**
- 스킬: 재사용 가능한 전문 지식으로 최소 토큰 오버헤드
- 서브에이전트: 복잡한 다단계 워크플로우를 위한 컨텍스트 격리
- MCP 서버: 외부 API 및 실시간 데이터 소스 연결

### 강점
- 표준화된 MCP 프로토콜 채택
- 명확한 도구 격리 및 권한 관리
- 플러그인 생태계 지원
- 컨텍스트 격리로 안정성 향상

### 약점
- 서브에이전트 간 직접 정보 공유 불가
- "thinking" 모드 미지원
- 중첩 서브에이전트 불가능

### 참고자료
- [Claude Code: 커스텀 서브에이전트 생성](https://code.claude.com/docs/en/sub-agents)
- [alexop.dev: Claude Code 풀스택 이해](https://alexop.dev/posts/understanding-claude-code-full-stack/)
- [Penligent: Claude Code 아키텍처](https://www.penligent.ai/hackinglabs/inside-claude-code-the-architecture-behind-tools-memory-hooks-and-mcp/)

---

## 4. Devin / Cognition Labs

### 개요
Devin은 SWE-bench에서 13.86% 성공률을 달성한 "세계 최초 AI 소프트웨어 엔지니어"입니다(이전 최고 기록 1.96%).

### 아키텍처 결정사항

**장기 추론 및 계획:**
- 수천 개의 결정이 필요한 복잡한 엔지니어링 작업 실행 가능
- 매 단계에서 관련 컨텍스트 회상
- 시간 경과에 따른 학습 및 자기 오류 수정

**실행 환경:**
- 샌드박스된 셸, 에디터, 브라우저 포함
- 지속적 작업공간으로 세션 간 상태 유지
- 자체 CLI 기반 도구 구성

**성능 및 확장:**
- 2024년 9월: ARR $1M → 2025년 6월: $73M
- Devin 2.0: 에이전트-네이티브 IDE 경험 도입
- 새로운 플랜: $20부터 시작

### 강점
- 현존하는 공개 벤치마크에서 최고의 성능
- 장기 계획 및 실행 능력
- 자율 소프트웨어 엔지니어링 태스크 처리

### 약점
- 상용 서비스로 완전 공개되지 않은 아키텍처
- API/SDK 커스터마이징 제한적
- 높은 비용(엔터프라이즈 플랜)

### 참고자료
- [Cognition: Devin 2.0 출시](https://cognition.ai/blog/devin-2)
- [Cognition: Devin 성능 리뷰 2025](https://cognition.ai/blog/devin-annual-performance-review-2025)
- [Medium: Agent-Native Development Deep Dive](https://medium.com/@takafumi.endo/agent-native-development-a-deep-dive-into-devin-2-0-s-technical-design-3451587d23c0)

---

## 5. OpenHands / OpenDevin

### 개요
OpenHands(이전명 OpenDevin)는 ICLR 2025에서 발표된 오픈플랫폼으로, 이벤트 스트림 기반 아키텍처를 특징으로 합니다.

### 아키텍처 결정사항

**이벤트 스트림 아키텍처:**
- 모든 에이전트-환경 상호작용이 타입화된 이벤트로 흐름:
  - User Message → Agent → LLM → Action → Runtime → Observation → Agent
- 사용자 인터페이스, 에이전트, 환경 간의 강력하고 유연한 상호작용

**Docker 기반 샌드박시:**
- 각 작업 세션마다 별도의 Docker 컨테이너 생성
- REST API 서버를 통한 통신
- 임의의 코드 실행을 안전하게 격리

**런타임 환경:**
- 셸, 웹 브라우저, IPython 서버 포함
- 클라이언트-서버 모델
- 사용자 정의 베이스 이미지 지원

**런타임 이미지 빌드:**
- 사용자 제공 이미지 기반으로 OH 런타임 이미지 생성
- OpenHands 특화 코드 및 런타임 클라이언트 포함

### 강점
- 이벤트 스트림으로 명확한 분리
- Docker 기반으로 기존 도구 활용 용이
- 커뮤니티 드리븐 오픈소스
- 학술 출판으로 검증된 아키텍처

### 약점
- Docker 오버헤드로 인한 성능
- 복잡한 런타임 설정
- 대규모 프로덕션 배포 시 리소스 집약적

### 참고자료
- [OpenHands: An Open Platform for AI Software Developers (arXiv)](https://arxiv.org/html/2407.16741v3)
- [OpenHands 런타임 아키텍처 문서](https://docs.openhands.dev/openhands/usage/architecture/runtime)
- [OpenHands 소프트웨어 에이전트 SDK](https://arxiv.org/html/2511.03690v1)

---

## 6. SWE-agent / Mini-SWE-agent (Princeton)

### 개요
Princeton 연구팀에서 NeurIPS 2024에 발표한 "Agent-Computer Interface" 개념의 선구자입니다.

### 아키텍처 결정사항

**Agent-Computer Interface (ACI):**
- 에이전트의 능력을 증진시키는 추상화 계층
- 파일 보기, 검색, 편집을 위한 소수의 간단한 액션
- 가드레일로 일반적 실수 방지
- 매 턴마다 구체적이고 간결한 피드백 제공

**설계 원칙:**
- 한 가지 원자적 작업만 수행하는 도구
- snake_case 일관성
- 명시적 제약 조건 문서화
- 강타이핑과 열거형

**성능 최적화:**
- 감정 구조 인지를 줄인 간단한 인터페이스
- 효율적인 도구 디자인으로 토큰 사용 최소화

### 강점
- **Mini-SWE-Agent**: 100줄의 Python으로 65% SWE-bench 달성
- 최소한의 설정, 단순한 구조
- 학술적으로 검증된 원리
- 광범위한 산업 채택(Meta, NVIDIA, IBM 등)

### 약점
- 소프트웨어 엔지니어링 작업 특화
- 다른 도메인으로의 일반화 제한적

### 참고자료
- [SWE-agent: Agent-Computer Interfaces (arXiv)](https://arxiv.org/abs/2405.15793)
- [GitHub: SWE-agent](https://github.com/SWE-agent/SWE-agent)
- [GitHub: Mini-SWE-agent](https://github.com/SWE-agent/mini-swe-agent/)

---

## 7. Aider

### 개요
Aider는 git 저장소 맵, diff 기반 편집, 모델별 편집 형식 지원을 특징으로 하는 대화형 코딩 도구입니다.

### 아키텍처 결정사항

**저장소 맵 (Repo Map):**
- ctags 기반으로 자동 생성되는 저장소 전체 맵
- 모든 선언된 변수 및 함수의 호출 시그니처 포함
- Tree Sitter 기반 향상된 파싱

**편집 형식:**
1. **Diff 형식**: 검색/교체 블록으로 파일 수정 지정 (효율적)
2. **Whole 형식**: 전체 업데이트된 파일 반환 (느리지만 단순)
3. **Diff-Fenced 형식**: Gemini 모델 대응
4. **Unified Diff 형식**: GPT-4 Turbo의 게으른 코딩 문제 해결

**모델별 최적화:**
- gpt-4o-2024-11-20 및 GPT-5 전체 지원
- Cohere Command-R+ 지원 개선
- 모델별 강점에 맞춘 편집 형식 선택

### 강점
- 저장소 맵으로 컨텍스트 최적화
- 모델별 맞춤 편집 형식
- 성숙한 오픈소스 프로젝트
- Unified diff로 3배 게으름 감소

### 약점
- 저장소 크기에 따른 스케일링 문제
- 비교적 로우레벨 인터페이스

### 참고자료
- [Aider: Repository Map](https://aider.chat/docs/repomap.html)
- [Aider: Edit Formats](https://aider.chat/docs/more/edit-formats.html)
- [Aider: Unified Diffs](https://aider.chat/docs/unified-diffs.html)

---

## 8. Continue.dev

### 개요
Continue는 IDE(VS Code, IntelliJ) 통합으로 컨텍스트 프로바이더 기반 아키텍처를 제공합니다.

### 아키텍처 결정사항

**컨텍스트 프로바이더:**
- 표준화된 인터페이스를 통한 외부 데이터 소스 통합
- "@" 심볼로 접근 가능한 내장 프로바이더:
  - 현재 작업공간의 파일
  - 프로젝트 전체 함수/클래스
  - 현재 브랜치의 모든 변경사항
  - 현재 열린 파일
  - 마지막 터미널 명령 및 출력
  - 모든 열린 파일의 내용

**IDE 통합 패턴:**
- 공통 IDE 인터페이스(약 40개 메서드)
- 플랫폼별 구현: TypeScript(VS Code), Kotlin(IntelliJ)
- JSON 기반 메시지 프로토콜
- 크로스플랫폼 코어

**MCP 지원:**
- Model Context Protocol 표준 지원
- 임의의 MCP 서버 통합 가능
- HTTP 기반 커스텀 프로바이더

### 강점
- IDE 네이티브 통합으로 자연스러운 UX
- 확장 가능한 컨텍스트 프로바이더 시스템
- MCP 표준 채택
- 크로스 IDE 호환성

### 약점
- IDE 플러그인 아키텍처의 복잡성
- 플랫폼별 유지보수 부담

### 참고자료
- [Continue: Context Providers](https://docs.continue.dev/customize/deep-dives/custom-providers)
- [DeepWiki: Continue Architecture](https://deepwiki.com/continuedev/continue/5.1-plugin-architecture)
- [Continue: IDE Integration Patterns](https://deepwiki.com/continuedev/continue/2.4-communication-flow)

---

## 9. Cursor / Windsurf

### 개요
Cursor와 Windsurf는 2025년 에이전트 IDE 경쟁의 핵심 플레이어입니다.

### Cursor 2.0 아키텍처

**에이전트 중심 설계:**
- Composer 모델: 강화학습으로 훈련된 목적 제작 코딩 모델
- 4배 빠른 성능, MoE 아키텍처로 특화된 서브모델 라우팅
- 멀티파일 리팩토링 및 리포지토리 변경 자동화

**실행 모델:**
- Git worktree 격리로 최대 8개 에이전트 동시 실행
- 개발자가 제어할 수 있는 세밀한 체크포인트
- 의도적 설계: 기술적 한계가 아닌 사용성 선택

**컨텍스트 관리:**
- 수동 컨텍스트 추가 또는 코드베이스 태깅 필요
- 개발자 중심의 접근

### Windsurf 아키텍처

**Cascade 기술:**
- 신경망처럼 코드베이스를 매핑
- 멀티파일 편집 및 깊은 컨텍스트 인식 지원
- 자동 파일 영향도 분석

**자동 에이전트:**
- 전체 저장소 자동 스캔
- 영향받는 파일 자동 선택
- 테스트/명령 자동 실행 및 패치
- 확인 대화 없음

**모델 전략:**
- 특화된 소프트웨어 엔지니어링 모델 개발
- SWE-1.5(복잡 작업), SWE-1-mini(저지연 자동완성)
- 범용 모델 대신 자체 훈련 모델 사용

### 강점 및 차이점

| 측면 | Cursor | Windsurf |
|------|--------|----------|
| 컨텍스트 관리 | 수동(개발자 제어) | 자동(AI 선택) |
| 실행 제어 | 세밀한 체크포인트 | 자동 완전 실행 |
| 모델 | 기본 모델 활용 | 자체 훈련 SWE 모델 |
| 접근성 | 개발자 중심 | 자동화 중심 |

### 참고자료
- [Codecademy: Agentic IDE Comparison](https://www.codecademy.com/article/agentic-ide-comparison-cursor-vs-windsurf-vs-antigravity)
- [Cursor 2.0: Agent-First Architecture Guide](https://www.digitalapplied.com/blog/cursor-2-0-agent-first-architecture-guide)
- [Medium: Windsurf vs Cursor 전투](https://medium.com/@lad.jai/windsurf-vs-cursor-the-battle-of-ai-powered-ides-in-2025-57d78729900c)

---

## 10. Smolagents (HuggingFace)

### 개요
Smolagents는 1,000줄 이하의 코드로 강력한 AI 에이전트를 구축 가능한 미니 에이전트 프레임워크입니다.

### 아키텍처 결정사항

**Code Agents vs Tool Calling:**
- **CodeAgent**: Python 코드 스니펫으로 도구 호출 생성
- **ToolCallingAgent**: JSON 기반 도구 호출

**CodeAgent의 강점:**
- 동적 코드 생성으로 높은 표현력
- 복잡한 로직 및 제어 흐름 지원
- 함수 중첩 및 재사용 가능
- 루프, 변환, 추론 결합 가능
- 새로운 액션 동적 생성

**성능:**
- ToolCallingAgent 대비 약 30% 적은 단계 사용
- 복잡한 벤치마크에서 우수한 성능

**편의성:**
- Python 함수로 도구 노출
- JSON 대비 객체 관리 용이
- 호스팅 또는 로컬 샌드박스에서 실행 가능

### 강점
- 1,000줄의 미니멀한 구현
- 높은 표현력과 유연성
- 토큰 효율성(30% 절감)
- 무료 오픈소스

### 약점
- 보안: 로컬 실행 시 위험
- 코드 생성의 불확실성

### 참고자료
- [HuggingFace: Smolagents 소개](https://huggingface.co/blog/smolagents)
- [Smolagents 공식 문서](https://smolagents.org/)
- [Medium: Smolagents Deep Dive](https://kargarisaac.medium.com/exploring-the-smolagents-library-a-deep-dive-into-multistepagent-codeagent-and-toolcallingagent-03482a6ea18c)

---

## 11. 에이전트 하네스 학술 연구

### 하네스란 무엇인가?

**정의:**
하네스 계층은 모델 외부 컴포넌트로, 다음을 결정합니다:
- 어떤 지침이 구속력 있는지
- 어떤 액션이 가능한지
- 상태를 어떻게 외부화할 것인지
- 다단계 실행을 어떻게 경계/복구/검사할 것인지

### 핵심 설계 원칙

**분리된 관심사 (Separation of Concerns):**
- 각 아키텍처 결정이 독립적으로 설정 및 교체 가능해야 함

**점진적 성능 저하 (Progressive Degradation):**
- 리소스 고갈 시에도 우아하게 기능 저하
- 한 컴포넌트 실패가 전체 시스템 중단 초래하지 않도록

**투명성 우선 (Transparency over Magic):**
- 모든 시스템 액션이 관찰 및 재정의 가능해야 함

**진화 기반 설계 (Evolution-Based Design):**
- 모델이 향상됨에 따라 하네스는 축소되어야 함
- 제한을 보완하는 컴포넌트는 제거 가능해야 함

### 하네스 설계 고려사항

**도구 범위 (Tool Scoping):**
- 더 많은 도구 = 더 나쁜 성능
- 현재 단계에 필요한 최소 도구 세트만 노출

**휴먼-인-더-루프 제어:**
- 중요한 결정 시점에서 일시 중지
- 주요 액션에 대한 승인 필요

**점진적 공개 및 보안 (Progressive Disclosure):**
- 제한된 도구/권한으로 시작
- 필요에 따라 확대
- 기본값으로 최소 권한(least privilege)

### 벤치마킹 및 평가

**주요 벤치마크:**
- **SWE-bench Verified**: 실제 GitHub 이슈 해결 능력
- **WebArena**: 웹 네비게이션 에이전트 평가
- **GAIA**: 일반 AI 어시스턴트 평가
- **τ²-bench (2025)**: 커스터머 서비스(소매, 항공, 통신)
- **HAL (Holistic Agent Leaderboard)**: 포괄적 멀티벤치마크 평가

**평가 문제점:**
- do-nothing 에이전트가 38% τ-bench 항공사 작업 통과
- LLM-as-judge의 산술 오류
- 테스트 확대로 SWE-bench 순위 41% 변경

### HarnessCard 제안

최근 연구는 HarnessCard를 가벼운 보고 아티팩트로 제안:
- 하네스 설계 문서화
- 시스템 액션 투명성 제공

### 참고자료
- [Preprints.org: Harness Engineering for Language Agents](https://www.preprints.org/manuscript/202603.1756)
- [arXiv: Building AI Coding Agents for the Terminal](https://arxiv.org/html/2603.05344v1)
- [Medium: 2026은 Agent Harnesses의 해](https://aakashgupta.medium.com/2025-was-agents-2026-is-agent-harnesses-heres-why-that-changes-everything-073e9877655e)
- [arXiv: Best Practices for Agentic Benchmarks](https://arxiv.org/html/2507.02825v1)
- [arXiv: Holistic Agent Leaderboard](https://arxiv.org/pdf/2510.11977)

---

## 12. 샌드박싱 기술 비교

### 격리 기술 개요

에이전트용 샌드박싱은 다양한 기술로 구현됩니다.

### MicroVM (Firecracker & Kata Containers)

**특성:**
- 하드웨어 가상화로 커널 익스플로잇 방지
- E2B: Firecracker 선택, 125ms 콜드스타트
- 바이너리 크기: 3MB
- VM 수준 격리를 컨테이너 속도로 제공
- 메모리 오버헤드: 전통 VM의 GB 대비 microVM당 5MB

**장점:**
- 강력한 격리
- 빠른 부팅
- 가벼운 리소스 사용

### gVisor (User-Space Kernel)

**특성:**
- 유저스페이스 커널(Sentry)이 시스템콜 가로챔
- Modal이 선택한 기술
- 유효한 시스템콜 부분집합만 호스트에 프록시

**장점:**
- 상대적으로 가벼운 설정
- GPU 지원(Modal은 T4~H200)
- 서버리스 오토스케일링

### WebAssembly & WASI

**특성:**
- 런타임 샌드박스로 WASI 인터페이스를 통한 기능 제한
- 콜드스타트: 마이크로초
- 디스크 풋프린트: 수 MB(컨테이너 대비 수 GB)

**장점:**
- 극도의 경량성
- 빠른 부팅
- 제한된 기능으로 안전

### 플랫폼별 구현 비교

| 플랫폼 | 기술 | 콜드스타트 | 격리 수준 | 특성 |
|--------|------|---------|---------|------|
| **E2B** | Firecracker | 125ms | VM급 | AI 전문화 |
| **Modal** | gVisor | 중간 | 커널 프록시 | GPU 지원, 파이썬 최적화 |
| **Daytona** | Docker/Kata | <90ms | 중~강 | Docker 호환, LSP 지원 |
| **Northflank** | Kata/gVisor | 강함 | 강함 | BYOC 지원, 무제한 세션 |

### 참고자료
- [SoftwareSeni: Sandboxing Isolation Technologies](https://www.softwareseni.com/firecracker-gvisor-containers-and-webassembly-comparing-isolation-technologies-for-ai-agents/)
- [Manveer: AI Agent Sandboxing Guide 2026](https://manveerc.substack.com/p/ai-agent-sandboxing-guide)
- [Northflank: Daytona vs E2B](https://northflank.com/blog/daytona-vs-e2b-ai-code-execution-sandboxes)
- [Northflank: Best Code Execution Sandbox 2026](https://northflank.com/blog/best-code-execution-sandbox-for-ai-agents)

---

## 13. 대화 루프 설계

### 에이전트 루프의 기본 구조

표준적 에이전트 루프는 다음과 같은 반복입니다:

1. **프롬프트 평가**: 시스템 프롬프트 + 도구 정의 + 대화 기록
2. **도구 호출**: LLM이 도구 선택 및 호출
3. **결과 수집**: 도구 실행 결과
4. **피드백 통합**: 결과를 대화 기록에 추가
5. **반복**: 작업 완료까지 1-4 반복

### 컨텍스트 윈도우 관리

**문제:**
- 복잡한 작업은 많은 도구 호출 필요
- 각 루프 반복마다 기록 누적
- 컨텍스트 윈도우 초과 가능

**해결책 (SDK 자동 처리):**
- **NullConversationManager**: 수정 없음
- **SlidingWindowConversationManager**: 최근 메시지만 유지
- **SummarizingConversationManager**: 오래된 메시지 지능형 요약

### 스트리밍과 인터럽션

**인터럽션 처리:**
- API 호출이 백그라운드 스레드에서 실행
- 메인 스레드가 인터럽션 요청 모니터링
- 인터럽션 시 조기 종료, 네트워크 차단하지 않음
- 활성 자식 에이전트에 인터럽션 신호 전파

**스트리밍의 장점:**
- 응답 지연 감소
- 사용자 경험 향상
- 토큰 사용 시가시성

### 도구 설계의 영향

**좋은 도구 피드백:**
- 구체적이고 간결함
- 에이전트가 다음 단계를 명확히 이해 가능
- 오류 정보 포함

**도구 스코핑:**
- 많은 도구 = 더 나쁜 성능
- 현재 단계별 필요한 도구만 제공

### 참고자료
- [Strands Agents: Agent Loop](https://strandsagents.com/docs/user-guide/concepts/agents/agent-loop/)
- [Claude API: Agent Loop](https://platform.claude.com/docs/en/agent-sdk/agent-loop)
- [Braintrust: The Canonical Agent Architecture](https://www.braintrust.dev/blog/agent-while-loop/)
- [Restate: Durable AI Loops](https://www.restate.dev/blog/durable-ai-loops-fault-tolerance-across-frameworks-and-without-handcuffs)

---

## 14. 장기 실행 에이전트 상태 관리

### 상태 지속성의 중요성

장기 실행 에이전트(수시간~수일)의 경우:
- 메모리만으로는 불충분
- 계획된/계획되지 않은 중단 복구 필요
- 여러 병렬 작업 조정 필요

### 체크포인팅 아키텍처

**기본 개념:**
- 정기적 또는 각 단계 후 워크플로우 상태 저장
- 지속적 스토리지(SQLite, PostgreSQL, S3 등)에 저장
- 실패 시 해당 지점에서 복구 가능

**LangGraph 접근법:**
- **Explicit State Schemas**: TypedDict와 Annotated 타입 사용
- **Reducer Functions**: 정의된 리듀서로 안전한 상태 업데이트
- **Robust Checkpointing**: 병렬 실행 시에도 일관성 유지

### 타임 트래블 (Time Travel) 및 리플레이

**개념:**
- 이전 실행 상태 저장
- 검사 및 분석 가능
- 해당 지점에서 분기 가능

**비결정성 디버깅:**
- 체크포인트된 상태로 재현 가능
- 단계별 검사 가능
- 트래젝토리 변경 가능

**구현:**
- LangGraph Time Travel: 실행 간 상태 비교
- OpenReplay: 비결정성 원인 분석
- 분기: 대체 경로 탐색 가능

### 프로덕션 사례

**AWS DynamoDB + LangGraph:**
- DynamoDBSaver 커넥터 제공
- 크기 기반 페이로드 지능형 처리
- 프로덕션 레디

**LangGraph GA (2025년 5월):**
- 거의 400개 회사의 프로덕션 에이전트에서 사용
- 안정적인 상태 관리 제공

### 참고자료
- [LangGraph Persistence Documentation](https://docs.langchain.com/oss/javascript/langgraph/persistence)
- [Mastering LangGraph State Management 2025](https://sparkco.ai/blog/mastering-langgraph-state-management-in-2025)
- [AWS: Build Durable AI Agents with LangGraph and DynamoDB](https://aws.amazon.com/blogs/database/build-durable-ai-agents-with-langgraph-and-dynamodb/)
- [Checkpoint/Restore Systems Evolution](https://eunomia.dev/blog/2025/05/11/checkpointrestore-systems-evolution-techniques-and-applications-in-ai-agents/)

---

## 15. 컨텍스트 압축 기술

### 장기 실행 에이전트의 컨텍스트 문제

**현황(2025):**
- 엔터프라이즈 AI 실패의 ~65%가 컨텍스트 드리프트/메모리 손실
- 단순 토큰 부족이 아닌 컨텍스트 품질 문제

### ACON 프레임워크 (October 2025)

**개념:**
- Agent Context Optimization
- 환경 관찰과 상호작용 기록 모두 압축

**알고리즘:**
1. 두 개 트래젝토리 비교: 전체 컨텍스트(성공) vs 압축 컨텍스트(실패)
2. 강력한 LLM이 실패 원인 분석
3. 자연어로 압축 가이드라인 업데이트
4. 반복

**성능:**
- 메모리 사용 26-54% 감소
- 95% 이상 정확도 유지
- 소형 모델 성능 46% 향상

### 프로바이더 네이티브 솔루션

**OpenAI (GPT-4o):**
- `/responses/compact` 엔드포인트
- 불투명한 압축 표현
- 최고 압축률(99.3%)
- 해석 불가능성

**Anthropic (Claude):**
- Claude SDK 내장 압축
- 구조화된 요약(7-12k 자)
- 섹션: 분석, 파일, 대기 작업, 현재 상태
- 해석 가능

### 권장 기법

**2025-2026 수렴한 기법:**
1. Anchored Iterative Summarization: 중요 포인트 유지
2. Failure-Driven Guideline Optimization (ACON)
3. Provider-native Compaction APIs

### 참고자료
- [ACON: Optimizing Context Compression (arXiv)](https://arxiv.org/html/2510.00615v1)
- [Factory.ai: Evaluating Context Compression](https://factory.ai/news/evaluating-compression)
- [Zylos Research: AI Agent Context Compression](https://zylos.ai/research/2026-02-28-ai-agent-context-compression-strategies)

---

## AgentOS 하네스 설계 교훈

### 핵심 아키텍처 선택

#### 1. 다층 인터페이스 전략 (Hermes 모델)
**배우기:**
- 단일 핵심 오케스트레이션 (AIAgent 클래스)
- 다중 진입점(CLI, SDK, IDE, 예약): 유연성 제공
- 각 인터페이스가 동일한 핵심 엔진 사용: 일관성 보장

**AgentOS 적용:**
```
AgentOS Core (Event-based State Machine)
├── CLI Interface
├── SDK Interface (Python/JS)
├── Desktop App
├── IDE Plugin
└── Daemon Mode (백그라운드)
```

#### 2. 이벤트 스트림 기반 설계 (OpenHands 모델)
**배우기:**
- 명확한 입출력 타입화
- User → Agent → LLM → Action → Runtime → Observation
- UI, 에이전트, 런타임 간 느슨한 결합

**AgentOS 적용:**
- 모든 상호작용을 TypedEvent로 정의
- 이벤트 핸들러로 플러그형 확장
- 완전한 이벤트 기록(리플레이 가능)

#### 3. Agent-Computer Interface 최소화 (SWE-agent 모델)
**배우기:**
- 원자적 액션(atomic actions)만 노출
- 각 피드백이 구체적이고 간결
- 가드레일로 일반적 실수 방지

**AgentOS 적용:**
```
Minimal ACI:
- fs.read_file(path)
- fs.write_file(path, content)
- exec.run_command(cmd)
- editor.open(path)
- editor.position_cursor(line, col)
```

#### 4. 컨텍스트 격리 (Claude Code 모델)
**배우기:**
- 서브에이전트의 독립적 컨텍스트 윈도우
- 명시적 도구 격리(allowlist/denylist)
- 무한 중첩 방지

**AgentOS 적용:**
- Task-specific Sub-agents (분석, 코딩, 테스팅)
- 각 서브에이전트는 최소 필요 도구만 접근
- 1단계 위임만 허용

#### 5. 빠른 샌드박싱 (E2B/Daytona)
**배우기:**
- Firecracker microVM: 125ms 콜드스타트
- Kata Containers: <90ms
- Docker 오버헤드: 초 단위

**AgentOS 적용:**
```
런타임 선택:
- 로컬 개발: Docker (빠른 반복)
- 프로덕션: Firecracker/Kata (보안)
- 엣지/경량: WASI (극도 경량)
```

#### 6. 명시적 상태 관리 (LangGraph)
**배우기:**
- TypedDict 기반 상태 스키마
- Reducer 함수로 안전한 업데이트
- 체크포인팅으로 복구 가능성

**AgentOS 적용:**
```python
class AgentState(TypedDict):
    task: str
    context_files: list[str]
    execution_log: list[Event]
    current_step: int
    result: Optional[str]

# 각 단계 후 자동 체크포인팅
# 실패 시 마지막 체크포인트에서 복구
```

#### 7. 장기 실행 상태 지속성
**배우기:**
- 메모리 + 영구 스토리지 (SQLite/DynamoDB)
- 타임 트래블: 이전 상태 검사 및 분기
- 비결정성 디버깅: 체크포인트 기반 재현

**AgentOS 적용:**
- 세션 메타데이터: SQLite에 저장
- 도구 호출 로그: 완전 기록
- 체크포인트: 각 에이전트 단계마다

#### 8. 적응형 컨텍스트 압축
**배우기:**
- ACON: 실패 기반 가이드라인 최적화
- 구조화된 요약: 해석 가능성 유지
- 65% 기업 실패가 컨텍스트 드리프트

**AgentOS 적용:**
```
Compression Strategy:
1. 최근 10 이벤트: 전체 유지
2. 이전 50 이벤트: 요약
3. 이전 모든 이벤트: 메타데이터만
4. 중요 변수: 항상 유지
```

#### 9. MCP 표준 채택
**배우기:**
- Claude Code의 표준화된 MCP 통합
- Continue의 컨텍스트 프로바이더
- 도구 등록/디스커버리 표준화

**AgentOS 적용:**
- MCP 서버 자동 발견
- 도구 능력 분석 및 등록
- IDE/에디터 플러그인과의 통합

#### 10. 최소 하네스 설계 (Mini-SWE-agent)
**배우기:**
- 100줄로 65% 벤치마크 달성
- 복잡성보다 명확성
- 진화 기반: 모델 향상에 따라 하네스 축소

**AgentOS 아키텍처:**
```
핵심 (~500줄):
- Event loop
- Tool dispatch
- State management

확장 (1000줄+):
- Caching
- Context compression
- Multi-agent coordination

제거 가능:
- 특정 샌드박스 구현
- UI 어댑터
- 확장 기능
```

### 설계 원칙 요약

**투명성 (Transparency)**
- 모든 에이전트 액션이 사용자에게 가시화
- 도구 호출 및 결과 완전 기록
- 비결정성 추적 가능

**제어성 (Control)**
- 중요 결정에서 사용자 개입 가능
- 각 단계 이후 일시 중지 가능
- 경로 수정/취소 가능

**복구 가능성 (Recoverability)**
- 체크포인트 기반 상태 저장
- 실패 지점에서 재개 가능
- 타임 트래블로 디버깅 가능

**확장성 (Extensibility)**
- 플러그형 도구 시스템
- MCP 표준 채택
- 이벤트 기반 아키텍처로 후킹 용이

**효율성 (Efficiency)**
- 최소 도구 노출 (도구 범위 최적화)
- 적응형 컨텍스트 압축
- 필요시에만 리소스 할당

**보안 (Security)**
- 격리된 샌드박스 (Firecracker/Kata)
- 최소 권한 원칙 (Least privilege)
- 도구별 권한 제어

### 피해야 할 안티패턴

1. **과도한 도구 노출**
   - 성능 저하
   - 에이전트 혼란
   - → 필요한 도구만 노출

2. **투명성 부족**
   - 사용자가 에이전트 이해 불가
   - 디버깅 어려움
   - → 모든 액션 기록 및 표시

3. **상태 지속성 부재**
   - 장기 작업 불가능
   - 복구 불가능
   - → SQLite + 체크포인팅

4. **무제한 컨텍스트 누적**
   - 토큰 낭비 및 성능 저하
   - 중요 정보 상실
   - → 적응형 압축 + 슬라이딩 윈도우

5. **강한 결합 아키텍처**
   - 인터페이스별 다른 구현
   - 유지보수 어려움
   - → 이벤트 기반 코어로 분리

### 권장 기술 스택

**코어 런타임:**
- 언어: Python (빠른 반복) 또는 Rust (성능)
- 프레임워크: LangGraph 호환 설계
- 상태 관리: TypedDict + Reducer 패턴

**샌드박싱:**
- 기본: Docker (개발)
- 프로덕션: Firecracker microVM (E2B/Daytona)
- 옵션: gVisor (GPU 워크로드)

**저장소:**
- 상태: SQLite (로컬) / DynamoDB (클라우드)
- 로그: 이벤트 스트림 (재현 가능)

**통합:**
- MCP 표준 채택
- IDE 플러그인 지원
- CLI + GUI 모두 제공

**평가:**
- HAL 기반 멀티벤치마크
- SWE-bench Verified로 검증
- 비용/성능 트레이드오프 추적

---

## 요약 비교표

| 요소 | Hermes | Codex | Claude Code | Devin | OpenHands | SWE-agent | Cursor | Windsurf | Smolagents | LangGraph |
|------|--------|-------|-------------|-------|-----------|-----------|--------|----------|-----------|-----------|
| **오픈소스** | ✓ | ✗ | ✓ | ✗ | ✓ | ✓ | ✗ | ✗ | ✓ | ✓ |
| **다중 인터페이스** | ✓✓ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| **이벤트 기반** | ○ | ○ | ○ | ○ | ✓✓ | ○ | ✗ | ✗ | ○ | ✓ |
| **상태 지속성** | ✓ | ✗ | ○ | ✓ | ○ | ○ | ○ | ✓ | ✗ | ✓✓ |
| **컨텍스트 압축** | ✓ | ○ | ✓ | ○ | ○ | ✗ | ○ | ○ | ✗ | ✓ |
| **MCP 지원** | ○ | ✓ | ✓✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ○ |
| **샌드박싱** | ✓ | ✓✓ | ✓ | ✓ | ✓✓ | ✓ | ✓ | ✓ | ✗ | ✗ |
| **간결성** | ○ | ✓ | ✓ | ✗ | ○ | ✓✓ | ✓ | ○ | ✓✓ | ○ |

(✓✓: 강점, ✓: 지원, ○: 부분 지원, ✗: 미지원)

---

## 결론: AgentOS를 위한 설계 권장사항

### 1단계: 핵심 구축 (MVP)
1. **이벤트 기반 상태 머신** (OpenHands 모델)
   - 모든 상호작용 타입화
   - 완전 기록 가능

2. **최소 ACI** (SWE-agent 원칙)
   - 5-8개 핵심 도구만 시작
   - 각 도구는 단일 책임

3. **로컬 SQLite 저장소**
   - 세션 메타데이터
   - 도구 호출 로그

### 2단계: 프로덕션 강화
1. **적응형 컨텍스트 압축** (ACON)
2. **다중 샌드박싱 지원** (Docker → Firecracker)
3. **MCP 통합** (에이전트 확장성)

### 3단계: 다중 에이전트 조율
1. **서브에이전트 격리** (Claude Code 모델)
2. **타임 트래블 디버깅** (LangGraph 기법)
3. **병렬 실행 안전성** (LangGraph State)

### AgentOS의 경쟁 우위

기존 솔루션들의 장점을 결합:
- **Hermes의 다중 인터페이스** + **OpenHands의 이벤트 기반** + **Claude Code의 MCP** + **SWE-agent의 단순성** + **LangGraph의 상태 관리** + **Daytona의 빠른 샌드박싱**

결과:
- 개발자 친화적 (CLI + SDK + IDE)
- 프로덕션 견고성 (체크포인팅 + 복구)
- 투명성 (완전 기록)
- 확장성 (MCP 표준)
- 성능 (빠른 샌드박싱 + 컨텍스트 압축)

---

## 참고 자료 총목록

### 공식 문서 및 아키텍처
1. [Hermes Agent 아키텍처](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture/)
2. [OpenAI Codex CLI](https://developers.openai.com/codex/cli)
3. [Claude Code 서브에이전트](https://code.claude.com/docs/en/sub-agents)
4. [OpenHands 런타임 아키텍처](https://docs.openhands.dev/openhands/usage/architecture/runtime)
5. [Continue.dev 컨텍스트 프로바이더](https://docs.continue.dev/customize/deep-dives/custom-providers)

### 학술 논문
6. [SWE-agent: Agent-Computer Interfaces (arXiv 2405.15793)](https://arxiv.org/abs/2405.15793)
7. [OpenHands: Open Platform for AI Software (arXiv 2407.16741)](https://arxiv.org/html/2407.16741v3)
8. [ACON: Context Compression (arXiv 2510.00615)](https://arxiv.org/html/2510.00615v1)
9. [Best Practices for Agentic Benchmarks (arXiv 2507.02825)](https://arxiv.org/html/2507.02825v1)
10. [Holistic Agent Leaderboard (arXiv 2510.11977)](https://arxiv.org/pdf/2510.11977)

### 기술 블로그 및 분석
11. [Medium: Devin 2.0 Deep Dive](https://medium.com/@takafumi.endo/agent-native-development-a-deep-dive-into-devin-2-0-s-technical-design-3451587d23c0)
12. [alexop.dev: Claude Code 풀스택](https://alexop.dev/posts/understanding-claude-code-full-stack/)
13. [SoftwareSeni: Sandboxing Isolation Technologies](https://www.softwareseni.com/firecracker-gvisor-containers-and-webassembly-comparing-isolation-technologies-for-ai-agents/)
14. [Medium: 2026은 Agent Harnesses의 해](https://aakashgupta.medium.com/2025-was-agents-2026-is-agent-harnesses-heres-why-that-changes-everything-073e9877655e)
15. [Braintrust: Canonical Agent Architecture](https://www.braintrust.dev/blog/agent-while-loop/)

### 프레임워크 및 도구
16. [HuggingFace Smolagents](https://huggingface.co/blog/smolagents)
17. [LangGraph State Management](https://sparkco.ai/blog/mastering-langgraph-state-management-in-2025)
18. [Aider Repository Map](https://aider.chat/docs/repomap.html)

### 샌드박싱 및 인프라
19. [Northflank: Daytona vs E2B](https://northflank.com/blog/daytona-vs-e2b-ai-code-execution-sandboxes)
20. [E2B 문서](https://e2b.dev)

---

**최종 작성**: 2026년 4월 10일  
**상태**: AgentOS 설계 연구 완료  
**다음 단계**: 핵심 런타임 프로토타입 구축
