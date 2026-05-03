# 트러블슈팅

CLI 사용 중 자주 마주치는 문제와 진단 절차를 모았습니다. 코드 레벨에서의 출처는 모두 `internal/cli/` 트리 안에 있습니다.

## 종료 코드

| 코드 | 의미 | 출처 |
|------|------|------|
| `0` | 정상 종료 | `ExitOK` |
| `1` | 일반 오류 (cobra 가 stderr 에 메시지를 출력) | `ExitError` |
| `2` | 사용 오류 (인자/플래그 누락 등) | cobra 표준 동작 |
| `69` | daemon unreachable (`REQ-CLI-008`) | transport `connection refused` 매핑 |

`echo $?` 로 직접 확인할 수 있습니다.

```bash
./goose ping; echo "exit=$?"
```

## "goose: daemon unreachable at 127.0.0.1:9005"

daemon-의존 명령 (`ask`, `ping`, `daemon status`, `session load`, `config *`, `tool list`, TUI) 이 daemon 에 연결하지 못했을 때 발생합니다.

체크리스트:

1. `goosed` 가 실제로 켜져 있는지 확인
   ```bash
   pgrep -lf goosed
   # 또는
   lsof -iTCP:9005 -sTCP:LISTEN
   ```
2. 다른 포트로 띄웠다면 `--daemon-addr` 로 동일하게 지정
   ```bash
   ./goose --daemon-addr 127.0.0.1:19005 ping
   ```
3. 방화벽/macOS 의 "수신 연결 허용" 다이얼로그가 차단하지 않았는지 확인.
4. 원격 daemon 이라면 `127.0.0.1` 이 아니라 실제 IP 와 포트가 LISTEN 중인지 점검.

오류 메시지의 정확한 패턴(소문자 비교)은 다음과 같이 매칭됩니다 (`isUnreachableError` 참조).

- `connection refused`
- `daemon unreachable`
- `no such host`
- `connect: connection refused`

## "requires a message argument or --stdin flag"

`goose ask` 를 인자도, `--stdin` 도 없이 실행했을 때 종료 코드 `2` 와 함께 출력됩니다.

```bash
./goose ask "안녕"           # OK
echo "안녕" | ./goose ask --stdin   # OK
./goose ask                  # exit 2, 위 메시지 출력
```

## 응답이 멈추거나 오래 걸려요

기본 timeout 은 `30s` 입니다 (`REQ-CLI-009`). 긴 응답이 필요하면 늘려 주세요.

```bash
./goose ask --timeout 2m "긴 분석을 수행해줘"
```

TUI 에서는 `Esc` 로 진행 중인 응답을 취소하고 새 메시지를 보낼 수 있습니다. timeout 을 늘리는 별도 옵션은 없으며, 영구적으로 늘리려면 daemon 측 LLM 호출 timeout 을 점검하세요.

## "config key not found: <key>"

`goose config get <key>` 가 키를 찾지 못한 경우입니다. `goose config list` 로 등록된 키를 먼저 확인하고, 필요하면 `goose config set <key> <value>` 로 추가하세요.

## "save command only works in chat mode"

`goose session save <name>` 은 의도적으로 항상 오류를 반환합니다 (`REQ-CLI-012`). 세션 저장은 반드시 TUI 안에서 `/save <name>` 으로 수행해야 합니다.

## TUI 가 깨져 보일 때

대부분 터미널 호환성 문제입니다.

- `--no-color` 로 색 코드 출력을 끄고 다시 시도해 보세요.
- macOS 의 기본 Terminal.app 보다는 iTerm2, Alacritty, WezTerm, VS Code 통합 터미널 등에서 안정적으로 동작합니다.
- CI 환경(`tty` 가 아닌 stdout) 에서는 TUI 진입을 피하고 `goose ask` 를 사용하세요.

## "daemon shutdown not yet implemented"

`goose daemon shutdown` 은 현재 스텁입니다. 데몬을 종료하려면 `Ctrl+C` 또는 `pkill goosed` 로 직접 신호를 보내세요. SPEC amendment 가 필요하며 `SPEC-GOOSE-CLI-001` 후속 후보 E 항목입니다.

## "plugin system not yet implemented"

`goose plugin install <name>` / `remove <name>` 도 동일하게 스텁입니다. `goose plugin list` 만 동작하며 항상 `No plugins installed.` 를 반환합니다. 플러그인 시스템 SPEC 이 머지되면 본 동작이 갱신됩니다.

## 자주 묻는 질문

**Q. `goose` 와 `goosed` 둘 다 띄워야 하나요?**
A. 네. `goose` 는 사용자 인터페이스(CLI/TUI), `goosed` 는 LLM 호출과 상태를 담당하는 데몬입니다. 두 프로세스가 동일 머신에서 분리 동작합니다.

**Q. 컬러 출력을 영구적으로 끄려면?**
A. 매 호출에 `--no-color` 를 붙이거나 alias 를 만들어 사용하세요. 환경 변수 기반 비활성은 아직 제공되지 않습니다.

**Q. 로그를 더 자세히 보고 싶어요.**
A. `--log-level debug` 를 추가하면 zap logger 가 debug 레벨로 출력합니다. CLI 측 로그는 데몬 응답을 가공하기 직전 / 직후 단계에서만 의미를 가집니다 (대부분의 LLM 디테일은 daemon 로그에 있습니다).

**Q. 멀티턴 대화 시 priors 가 너무 많아 token cost 가 큰 것 같아요.**
A. 현재는 모든 히스토리를 daemon 에 전달합니다. 압축은 `SPEC-GOOSE-COMPRESSOR-001` (M4) 의 책임이며 본 CLI SPEC 범위 밖입니다. 임시로는 `/clear` 로 히스토리를 비우거나 새 TUI 세션을 시작하세요.

## 그래도 해결되지 않는다면

- daemon 로그를 함께 첨부해 GitHub Discussions 또는 이슈 트래커에 보고해 주세요.
- 재현 절차에 다음을 포함하면 진단이 빠릅니다.
  - `goose version` 출력 (commit / builtAt)
  - `goosed` 시작 명령과 환경
  - 실패 직전 마지막 입력
  - 종료 코드 (`echo $?`)
