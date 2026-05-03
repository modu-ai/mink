# 명령 레퍼런스

본 문서는 `goose` 바이너리가 노출하는 최상위 명령 9개와 그 하위 서브커맨드, 그리고 모든 명령에 공통으로 적용되는 전역 플래그를 정리합니다. 동작 코드는 `internal/cli/commands/*.go` 에 1:1 로 대응됩니다.

## 전역 플래그

모든 서브커맨드에서 동일하게 사용할 수 있습니다 (`PersistentFlags`).

| 플래그 | 기본값 | 설명 |
|--------|--------|------|
| `--config <path>` | `""` | 설정 파일 경로. 비어 있으면 기본 검색 규칙을 따릅니다. |
| `--daemon-addr <host:port>` | `127.0.0.1:9005` | daemon 주소. daemon-의존 명령에 모두 전달됩니다. |
| `--format <text\|json>` | `text` | 출력 포맷 힌트. 현재는 텍스트만 채택되며 일부 명령(`audit query`)은 항상 JSON 입니다. |
| `--log-level <debug\|info\|warn\|error>` | `info` | zap 로거 레벨. |
| `--no-color` | `false` | 컬러 출력 비활성. CI/파이프 환경에서 유용. |

## 종료 코드

| 코드 | 의미 |
|------|------|
| `0` | 정상 종료 |
| `1` | 일반 오류 (cobra 가 출력) |
| `2` | 사용 오류 (인자/플래그 누락 등) |
| `69` | daemon unreachable (`REQ-CLI-008`) |

자세한 표는 [트러블슈팅](troubleshooting.md#종료-코드) 에 정리되어 있습니다.

---

## `goose ask` — LLM 한 줄 질의

LLM 에 메시지를 보내고 응답을 토큰 단위로 stdout 에 스트리밍합니다.

```
goose ask <message> [--stdin] [--timeout 30s]
```

| 입력 방식 | 예시 |
|-----------|------|
| 인자 1개 | `goose ask "안녕"` |
| `--stdin` | `echo "..." \| goose ask --stdin` |

플래그:

- `--timeout <duration>` (기본 `30s`, `REQ-CLI-009`) — daemon 응답을 기다리는 최대 시간.
- `--stdin` — stdin 에서 메시지 본문을 읽어옵니다. 인자와 동시에 사용할 수 없습니다.

오류:

- 인자도 없고 `--stdin` 도 없으면 종료 코드 `2` (`requires a message argument or --stdin flag`).
- daemon 미가동 시 종료 코드 `69` 와 `goose: daemon unreachable at <addr>` (stderr).

> 다중 메시지를 넘기는 멀티턴 모드는 TUI 의 자동 replay 로 처리됩니다 ([TUI 가이드](tui.md#멀티턴-대화-replay) 참조).

## `goose ping` — 데몬 헬스 체크

```
goose ping
```

성공 시 stdout 에 `pong (version=..., state=..., uptime=...ms)` 한 줄을 출력합니다. 실패 시 종료 코드 `69` 와 함께 stderr 에 `goose: daemon unreachable at <addr>: <err>` 를 출력합니다.

## `goose daemon` — daemon 운영

```
goose daemon status
goose daemon shutdown   # 미구현 (M3 후속)
```

- `status` — `ping` 과 동일한 헬스체크에 추가로 latency 를 보여줍니다 (5초 timeout).
- `shutdown` — 현재 명령은 `daemon shutdown not yet implemented` 오류를 반환합니다. 향후 SPEC amendment 로 RPC 가 추가될 예정입니다.

## `goose session` — 세션 관리

```
goose session list
goose session load <name>
goose session save <name>     # 항상 오류 (TUI 의 /save 사용)
goose session rm <name> [-y]
```

- `list` — `~/.goose/sessions/` 아래 저장된 세션 이름을 한 줄씩 출력합니다. 비어 있으면 `No sessions found`.
- `load <name>` — 세션 파일을 읽어 메시지 수를 출력합니다 (TUI 진입 wiring 은 `dispatch.go` 에서 이뤄집니다).
- `save <name>` — CLI 단독으로는 항상 오류입니다. TUI 안에서 `/save <name>` 으로 실행하세요 (`REQ-CLI-012`).
- `rm <name>` — 삭제 전 `[y/N]` 으로 확인합니다. `-y` / `--yes` 플래그로 즉시 삭제할 수 있습니다.

## `goose config` — 설정 키-값 조회/저장

```
goose config get <key>
goose config set <key> <value>
goose config list
```

저장소는 `internal/cli/commands/connect_config_store.go` 의 `ConnectConfigStore` 가 daemon 의 `ConfigService` 로 위임합니다. daemon 이 꺼져 있으면 모든 서브커맨드가 종료 코드 `69` 를 반환합니다.

테스트에서는 `MemoryConfigStore` 가 동일 인터페이스를 구현해 in-process 로 동작합니다.

오류:

- `config get` 에서 키가 없으면 `goose: config key not found: <key>` 메시지와 함께 종료 코드 `1`.

## `goose tool` — 도구 카탈로그

```
goose tool list
```

daemon 의 `ToolService` 에 등록된 도구 이름과 설명을 출력합니다 (`internal/cli/commands/connect_tool_registry.go`). daemon 미가동 시 종료 코드 `69`.

오프라인/테스트 환경에서는 `StaticToolRegistry` 가 `read / write / edit / bash / browse / grep` 6개를 하드코딩 카탈로그로 제공합니다.

## `goose audit` — 감사 로그 검색

```
goose audit query [--since RFC3339] [--until RFC3339] [--type fs.write,permission.grant] [--log-dir PATH]
```

`~/.goose/logs/` (기본) 의 감사 로그를 읽어 JSON 배열로 stdout 에 출력합니다. 옵션:

- `--since`, `--until` — RFC3339 형식 (`2026-04-29T12:00:00Z`).
- `--type` — 쉼표 구분 이벤트 타입 필터.
- `--log-dir` — 기본 경로 대신 다른 디렉터리를 가리킬 때 사용.

본 명령은 daemon 을 거치지 않고 파일을 직접 읽습니다. daemon 가동 여부와 무관하게 동작합니다 (`SPEC-GOOSE-AUDIT-001 REQ-AUDIT-004`).

## `goose plugin` — 플러그인 (스텁)

```
goose plugin list           # 항상 "No plugins installed."
goose plugin install <name> # 미구현
goose plugin remove <name>  # 미구현
```

플러그인 시스템은 추후 SPEC 에서 구현됩니다. `install` / `remove` 는 현재 `goose: plugin system not yet implemented` 오류를 반환합니다.

## `goose version` — 버전 출력

```
goose version
# goose version v0.0.5 (commit abc1234, built 2026-05-04T10:00:00Z)
```

빌드 시 ldflags 로 주입된 `version` / `commit` / `builtAt` 를 그대로 출력합니다. 인자가 없으면 항상 종료 코드 `0`.

## (인자 없음) — TUI 채팅 진입

```
goose
```

서브커맨드 없이 실행하면 daemon 에 연결한 뒤 Bubble Tea 기반 TUI 채팅이 시작됩니다. 자세한 사용법은 [TUI 가이드](tui.md) 를 참조하세요.

---

## 명령별 daemon 의존도 한눈에 보기

| 명령 | daemon 필요 | 비고 |
|------|:---------:|------|
| `ask` | ✅ | `ChatStream` 호출 |
| `ping` | ✅ | 헬스체크 자체가 ping |
| `daemon status` | ✅ | latency 측정 |
| `daemon shutdown` | ⏸️ | 미구현 |
| `session list / rm` | ❌ | 로컬 파일만 사용 |
| `session load` | ✅ | TUI 진입 시 daemon 필요 |
| `session save` | n/a | 항상 오류, TUI 의 `/save` 사용 |
| `config *` | ✅ | `ConfigService` |
| `tool list` | ✅ | `ToolService` |
| `audit query` | ❌ | 파일 직접 읽기 |
| `plugin *` | ❌ | 모두 스텁 |
| `version` | ❌ | ldflags 만 출력 |
| (인자 없음, TUI) | ✅ | `ChatStream` + 멀티턴 replay |
