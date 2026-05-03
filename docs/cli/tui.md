# TUI 채팅 모드

`goose` 를 인자 없이 실행하면 Bubble Tea 기반의 풀스크린 TUI 채팅이 시작됩니다. 본 문서는 TUI 진입, 화면 구성, 키 바인딩, slash 명령, 그리고 멀티턴 대화 replay 동작을 정리합니다.

## 진입과 종료

```bash
./goose                       # 기본 daemon 주소(127.0.0.1:9005) 사용
./goose --daemon-addr host:port
./goose --no-color            # 컬러 비활성
```

종료 방법:

| 동작 | 키 |
|------|---|
| 일반 종료 | `Ctrl+C` |
| 스트리밍 중 응답 취소 | `Esc` (대화는 유지) |
| 스트리밍 중 강제 종료 | `Ctrl+C` 두 번 (첫 번째는 확인 프롬프트) |
| slash 명령 종료 | `/quit` |

`Esc` 는 진행 중인 daemon 응답만 끊으며 메시지 히스토리는 유지됩니다. 강제 종료가 아니라면 대화를 이어 갈 수 있습니다.

## 화면 구성

```
┌─────────────────────────────────────────────────┐
│ You: 안녕                                        │
│                                                  │
│ AI: 안녕하세요. 오늘 무엇을 도와드릴까요?         │
│                                                  │
│ You: 어제 얘기 이어서 해줘                        │
│ AI: ▌(스트리밍 중...)                             │
├─────────────────────────────────────────────────┤
│ status bar: streaming · daemon=127.0.0.1:9005   │
│ > _                                             │
└─────────────────────────────────────────────────┘
```

- 상단 영역은 `viewport` 로 자동 스크롤됩니다.
- 하단 입력창에서 메시지를 작성합니다.
- `--no-color` 가 아니면 역할별로 색이 입혀집니다 (`You` 녹색, `AI` 노랑, `system` 회색).

## 키 바인딩

| 키 | 동작 |
|-----|------|
| `Enter` | 입력 메시지 전송 (스트리밍 중이면 무시) |
| `Esc` | 진행 중인 응답 취소 |
| `Ctrl+C` | 종료 (스트리밍 중에는 확인 후 종료) |
| `Ctrl+S` | 세션 저장 (현재 placeholder, Phase E 에 본 구현 예정) |
| `Ctrl+L` | 뷰포트를 맨 위로 이동 |
| 일반 입력 | 입력창 편집 (커서 이동, 백스페이스 등 bubbletea textinput 의 기본 키 매핑 그대로) |

## Slash 명령

입력이 `/` 로 시작하면 daemon 으로 전송하지 않고 로컬에서 처리합니다.

| 명령 | 설명 |
|------|------|
| `/help` | 사용 가능한 slash 명령 목록 |
| `/save <name>` | 현재 세션을 저장 (CLI `goose session save` 는 항상 오류이므로 저장은 반드시 TUI 안에서) |
| `/load <name>` | 세션 파일을 불러와 대화에 병합 |
| `/clear` | 메시지 히스토리 비우기 (daemon 측 상태에는 영향 없음) |
| `/quit` | TUI 종료 |
| `/session` | 현재 세션 이름 표시 (이름이 없으면 `(unnamed)`) |

App dispatcher 가 활성화된 환경에서는 동일한 `/명령` 입력이 dispatcher 의 `ProcessLocal` / `ProcessProceed` / `ProcessExit` / `ProcessAbort` 결과로도 라우팅됩니다 (`internal/cli/tui/dispatch.go`). 그렇지 않은 빌드(테스트, 헤드리스 등)에서는 위 표의 legacy slash handler 가 fallback 으로 사용됩니다.

## 멀티턴 대화 replay

TUI 는 한 번 시작된 대화의 모든 사용자 메시지를 daemon 에 함께 보냅니다. 동작 흐름은 다음과 같습니다 (`internal/cli/rootcmd.go` 의 `askClientAdapter.ChatStream`).

1. 사용자가 `Enter` 를 누르면 현재까지의 메시지 전체(`history []ChatMessage`)가 transport adapter 로 전달됩니다.
2. adapter 는 `transport.SplitMessagesAtLastUser` 로 마지막 사용자 메시지를 분리합니다.
   - `priors` = 마지막 사용자 메시지 직전까지의 (user/assistant 교차) 히스토리.
   - `lastMsg` = 이번에 보낼 새 메시지.
3. `client.ChatStream(ctx, "", lastMsg, transport.WithInitialMessages(priors))` 로 호출되어 daemon 은 priors 를 컨텍스트로 받아 lastMsg 에 응답합니다.
4. 응답은 `transport.ChatStreamFanIn` 으로 단일 채널로 합류한 뒤 `commands.StreamEvent` 로 변환되어 viewport 에 토큰 단위로 그려집니다.

따라서 TUI 안에서는 별도 명령 없이도 자연스러운 멀티턴 흐름이 유지되며, daemon 측 컨텍스트 손실 없이 직전 발화의 의미가 이어집니다. CLI 단발성 `goose ask` 호출과의 차이가 바로 이 부분입니다.

## 상태 메시지

스트리밍 도중 시스템이 자동으로 삽입하는 메시지가 있습니다.

| 메시지 | 의미 |
|--------|------|
| `[Response cancelled]` | 사용자가 `Esc` 로 응답을 취소함 |
| `[Error: ...]` | daemon 또는 LLM 에서 오류 이벤트가 도착함 |
| `[Session save not yet implemented]` | `Ctrl+S` 또는 `/save` 를 눌렀지만 본 구현 전 |

오류 이벤트가 발생해도 TUI 는 종료되지 않으며, 히스토리는 유지된 채 새 입력을 받을 준비가 됩니다.

## 헤드리스 환경에서의 동작

- daemon 이 꺼져 있으면 첫 메시지 전송 시 `[Error: connection refused ...]` 가 system 메시지로 표시됩니다.
- TUI 자체는 종료되지 않으므로 daemon 을 다시 켠 뒤 그대로 입력해 다시 시도할 수 있습니다.
- 색 없는 단순 출력이 필요하다면 `--no-color` 플래그를 사용하세요. CI 빌드 로그나 파이프 캡처에 적합합니다.

## 더 알아보기

- [명령 레퍼런스](commands.md) — 전역 플래그와 daemon 의존도 표.
- [트러블슈팅](troubleshooting.md) — daemon unreachable, exit code, 오류 패턴 진단.
