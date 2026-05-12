# 시작하기

GOOSE CLI를 처음 사용한다면 본 절을 순서대로 따라가면 5분 안에 첫 채팅 응답을 받을 수 있습니다.

## 1. 사전 요구사항

| 항목 | 최소 버전 | 비고 |
|------|-----------|------|
| Go | 1.26 이상 | `go.mod` 의 `go 1.26` 라인과 일치 |
| Git | 임의 최신 | `gh` CLI는 권장 (PR 작업 시) |
| LLM 자격증명 | (선택) | Anthropic / OpenAI / Google / xAI / DeepSeek / Ollama 중 하나. 없어도 daemon 실행은 가능하지만 `ask` 응답은 모의 결과가 됩니다. |

> macOS / Linux / WSL2 모두 동작이 검증되어 있습니다. 순수 Windows 환경은 daemon stop 시그널 처리가 다를 수 있으므로 WSL2를 권장합니다.

## 2. 소스 빌드

```bash
git clone https://github.com/modu-ai/goose.git
cd goose

# 두 바이너리를 동시에 빌드 (CLI + daemon)
go build ./cmd/goose
go build ./cmd/goosed
```

빌드 산출물은 저장소 루트에 `goose`, `goosed` 두 실행 파일로 떨어집니다. 별도의 설치 단계는 없으며 그대로 실행하거나 `$PATH` 에 심볼릭 링크를 걸어 사용합니다.

### 버전 정보 주입(선택)

```bash
go build -ldflags "-X main.version=v0.0.5 -X main.commit=$(git rev-parse --short HEAD) -X main.builtAt=$(date -u +%FT%TZ)" ./cmd/goose
./goose version
# goose version v0.0.5 (commit abc1234, built 2026-05-04T10:00:00Z)
```

ldflags 없이 빌드하면 `version=dev / commit=none / builtAt=unknown` 으로 표시되며 동작에는 영향이 없습니다.

## 3. daemon 기동

CLI는 daemon(`goosed`)에 의존합니다. 새 터미널에서 다음을 실행하세요.

```bash
./goosed
# {"level":"info","msg":"goosed listening","addr":"127.0.0.1:9005"}
```

기본 주소는 `127.0.0.1:9005` 입니다. daemon 의 옵션과 백그라운드 실행 방법은 별도 문서에서 다룹니다 (M0 Foundation 범위).

## 4. 헬스 체크

daemon 이 응답하는지 먼저 확인합니다.

```bash
./goose ping
# pong (version=dev, state=ready, uptime=1234ms)
```

`goose: daemon unreachable at 127.0.0.1:9005` 메시지가 나오면 daemon이 켜져 있지 않거나 주소가 다릅니다. [트러블슈팅](troubleshooting.md) 절을 참고하세요.

## 5. 첫 한 줄 채팅

```bash
./goose ask "안녕, 자기소개 부탁해"
```

응답은 stdout 으로 토큰 단위 스트리밍됩니다. 종료 시 자동 줄바꿈이 추가되며 정상 종료 시 종료 코드는 `0` 입니다.

`stdin` 으로 긴 입력을 넘기려면 `--stdin` 플래그를 사용합니다.

```bash
echo "다음 텍스트를 한 줄로 요약해줘: ..." | ./goose ask --stdin
```

## 6. TUI 채팅 모드

인자 없이 실행하면 Bubble Tea 기반 TUI 채팅이 시작됩니다.

```bash
./goose
```

- 입력창에 메시지를 쓰고 `Enter` 로 전송
- `/help` 로 사용 가능한 slash 명령 확인
- `/quit` 또는 `Ctrl+C` 로 종료

자세한 키 바인딩과 slash 명령은 [TUI 가이드](tui.md) 에서 다룹니다.

## 7. 자격증명 추가(선택)

MINK 저장소 루트에서 다음을 실행하면 환경 변수에서 키를 읽어 안전하게 보관합니다.

```bash
./goose credential add anthropic --from-env ANTHROPIC_API_KEY
```

> `credential` 명령은 본 문서가 다루는 CLI 표면(M3) 외의 별도 영역입니다. 자세한 내용은 [`README.md`](../../README.md) 의 Quick Start 절을 참고하세요.

## 다음 단계

- [명령 레퍼런스](commands.md) — 8개 최상위 명령 전부와 플래그를 확인합니다.
- [TUI 가이드](tui.md) — 채팅 화면과 slash 명령, 멀티턴 대화 흐름을 익힙니다.
- 문제가 생기면 [트러블슈팅](troubleshooting.md) 을 먼저 확인하세요.
