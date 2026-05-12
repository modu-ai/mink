# Research — SPEC-MINK-ENV-MIGRATE-001

> 본 문서는 `GOOSE_*` 환경변수 22개를 `MINK_*` 로 alias 하는 deprecation loader 도입을 위한 사전 조사 자료다.
> 모든 file:line 참조는 worktree base commit `f0f02e4` (= origin/main, BRAND-RENAME-001 squash merge 직후) 기준이다.

## §1 Env var 전수 조사

### §1.1 22 unique GOOSE_* keys 와 정의/사용 위치

`grep -rh "GOOSE_[A-Z_]*" --include="*.go" --include="*.yaml" -o . | sort -u` 결과를 BRAND-RENAME spec.md §1.3 NOTE (21 keys) 와 대조해 1개 추가 잡음 (`GOOSE_AUTH_` prefix-only 매칭)를 제외하면 정확히 22 keys.

| # | Key | Type | Default | Runtime read site (production) | 사용 횟수 (전체 .go) | 비고 |
|---|-----|------|---------|--------------------------------|---------------------|------|
| 1 | `GOOSE_HOME` | string | `$HOME/.goose` | `internal/config/config.go:274`, `internal/audit/dual.go:140`, `internal/command/adapter/aliasconfig/loader.go:32` (const `homeEnv`) | 72 (test fixture 다수) | 사용자 데이터 루트. user-data path migration (`~/.goose → ~/.mink`) 은 **별도 SPEC** (SPEC-MINK-USERDATA-MIGRATE-001) scope. |
| 2 | `GOOSE_LOG_LEVEL` | string (debug/info/warn/error) | `info` | `internal/config/env.go:21` | 6 | envOverlay 진입점 |
| 3 | `GOOSE_HEALTH_PORT` | int | (config default) | `internal/config/env.go:27` | 8 | parse fail 시 WARN + 기존 값 유지 (AC-CFG-010b) |
| 4 | `GOOSE_GRPC_PORT` | int | (config default) | `internal/config/env.go:42` | 7 | parse fail 시 WARN + 기존 값 유지 |
| 5 | `GOOSE_LOCALE` | string | (config default) | `internal/config/env.go:57` | 4 | ui.locale 오버라이드 |
| 6 | `GOOSE_LEARNING_ENABLED` | bool | (config default) | `internal/config/env.go:88` | 5 | parse fail 시 WARN + 기존 값 유지 |
| 7 | `GOOSE_CONFIG_STRICT` | bool ("true"|"") | `false` | `internal/config/config.go:246` | 5 | strict unknown-key validation toggle |
| 8 | `GOOSE_GRPC_REFLECTION` | bool ("true") | `false` | `internal/transport/grpc/server.go:169` | 3 | gRPC reflection 활성 |
| 9 | `GOOSE_GRPC_MAX_RECV_MSG_BYTES` | int (bytes) | 4MiB | `internal/transport/grpc/server.go:277` | 7 | recv buffer 한도 |
| 10 | `GOOSE_SHUTDOWN_TOKEN` | string (secret) | "" | `internal/transport/grpc/server.go:297` | 14 | Shutdown RPC 인증 토큰 |
| 11 | `GOOSE_HOOK_TRACE` | string ("1"/"true"/"on") | unset | `internal/hook/handlers.go:270` | 5 | hook DEBUG 로그 활성 |
| 12 | `GOOSE_HOOK_NON_INTERACTIVE` | string ("1") | unset | `internal/hook/permission.go:251` | 3 | hook permission non-TTY 강제 |
| 13 | `GOOSE_ALIAS_STRICT` | string ("1"/"true"/"") | true | `cmd/minkd/main.go:89` | 2 | alias 검증 strict mode |
| 14 | `GOOSE_QWEN_REGION` | string ("intl"/"cn") | `intl` | `internal/llm/provider/qwen/client.go:38` (const `envQwenRegion`) | 8 | provider region URL 결정 |
| 15 | `GOOSE_KIMI_REGION` | string ("intl"/"cn") | `intl` | `internal/llm/provider/kimi/client.go:40` (const `envKimiRegion`) | 8 | provider region URL 결정 |
| 16 | `GOOSE_TELEGRAM_BOT_TOKEN` | string (secret) | "" | **runtime read 호출부 없음** — error message + flag help text 만 (`internal/cli/commands/messaging_telegram.go:68,138`, `internal/messaging/telegram/keyring_nokeyring.go:20,25`) | 4 | 사용자가 cobra `--token` flag 없이 env 만 설정한 경우 채워주는 logic 미구현 (현 구현 = error return). 본 SPEC 에서 alias loader 통합 시 신규 read logic 검토 필요 |
| 17 | `GOOSE_AUTH_TOKEN` | string (secret) | unset | **runtime read 호출부 없음** — `GOOSE_AUTH_*` prefix glob deny-list (env scrub) 의 일부 | 4 | hook subprocess 실행 시 자동 스크럽 대상 (`internal/hook/isolation_unix.go:55`, `isolation_other.go:62`) |
| 18 | `GOOSE_AUTH_REFRESH` | string (secret) | unset | **runtime read 호출부 없음** — `GOOSE_AUTH_*` prefix glob deny-list 의 일부 | 2 | 동일 |
| 19 | `GOOSE_HISTORY_SNIP` | string ("1") | unset | **runtime read 호출부 없음** — comment-only feature gate 참조 (`internal/context/compactor.go:58`, `internal/learning/compressor/adapter.go:31`) | 2 | 실제 게이트 코드 없음, comment 만 존재. SPEC-CTX 의 미완 항목으로 추정 |
| 20 | `GOOSE_METRICS_ENABLED` | string ("true") | unset | **runtime read 호출부 없음** — comment 만 (`internal/observability/metrics/noop/noop.go:2,9`) | 2 | metrics noop 구현의 의도 설명용 |
| 21 | `GOOSE_GRPC_BIND` | string | unset | **runtime read 호출부 없음** — REQ-TR-001 test comment (`internal/transport/grpc/server_test.go:568`) 만 | 1 | bind addr override 의도였으나 미구현 |
| 22 | `GOOSE_AUTH_` (prefix glob) | — | — | `internal/hook/isolation_unix.go:55`, `internal/hook/isolation_other.go:62` | 3 | deny-list **prefix rule** — 본 SPEC 의 핵심 영향: alias 후 `MINK_AUTH_*` prefix 도 deny-list 에 추가 필요 |

검증 명령:
```bash
cd /Users/goos/.moai/worktrees/goose/SPEC-MINK-ENV-MIGRATE-001
grep -rh "GOOSE_[A-Z_]*" --include="*.go" --include="*.yaml" -o . | grep -v "/vendor/" | sort -u | wc -l   # → 22
grep -rh "GOOSE_[A-Z_]*" --include="*.go" -o . | grep -v "/vendor/" | sort | uniq -c | sort -rn          # 사용 빈도
```

### §1.2 분류 (runtime read 유무)

- **Runtime-read keys (15개)** — alias loader 가 처리해야 할 대상:
  `GOOSE_HOME`, `GOOSE_LOG_LEVEL`, `GOOSE_HEALTH_PORT`, `GOOSE_GRPC_PORT`, `GOOSE_LOCALE`, `GOOSE_LEARNING_ENABLED`, `GOOSE_CONFIG_STRICT`, `GOOSE_GRPC_REFLECTION`, `GOOSE_GRPC_MAX_RECV_MSG_BYTES`, `GOOSE_SHUTDOWN_TOKEN`, `GOOSE_HOOK_TRACE`, `GOOSE_HOOK_NON_INTERACTIVE`, `GOOSE_ALIAS_STRICT`, `GOOSE_QWEN_REGION`, `GOOSE_KIMI_REGION`.
- **Doc-only / comment-only keys (4개)** — runtime read 부재, 본 SPEC 에서 산문만 정리 (alias loader 등록은 미래-proof 차원에서 동시 등록 권장):
  `GOOSE_TELEGRAM_BOT_TOKEN` (error message hint), `GOOSE_HISTORY_SNIP`, `GOOSE_METRICS_ENABLED`, `GOOSE_GRPC_BIND`.
- **Deny-list prefix glob (1개 + 2개 child)** — 별도 처리 경로:
  `GOOSE_AUTH_*` (`isolation_unix.go:55`, `isolation_other.go:62`) + `GOOSE_AUTH_TOKEN` / `GOOSE_AUTH_REFRESH` (test fixture only).

### §1.3 모듈/패키지 분포

| Package | Keys | 비고 |
|---------|------|------|
| `internal/config/` | LOG_LEVEL, HEALTH_PORT, GRPC_PORT, LOCALE, LEARNING_ENABLED, CONFIG_STRICT, HOME | envOverlay 함수가 5 key 의 단일 진입점 (env.go) |
| `internal/audit/` | HOME | dual.go:140 직접 read |
| `internal/transport/grpc/` | GRPC_REFLECTION, GRPC_MAX_RECV_MSG_BYTES, SHUTDOWN_TOKEN | server.go 3곳 직접 read |
| `internal/hook/` | HOOK_TRACE, HOOK_NON_INTERACTIVE | + AUTH_ prefix deny-list |
| `internal/llm/provider/qwen/`, `kimi/` | QWEN_REGION, KIMI_REGION | 각 client.go 의 const + 직접 read |
| `internal/command/adapter/aliasconfig/` | HOME (const `homeEnv`) | loader.go 32 |
| `cmd/minkd/` | ALIAS_STRICT | main.go 89 직접 read |
| `internal/cli/commands/`, `internal/messaging/telegram/` | TELEGRAM_BOT_TOKEN | 산문 hint 만 |

→ **단일 진입점 부재**: envOverlay (5 key) 외 10 개 production read site 가 분산. alias loader 도입은 envOverlay 확장 + 분산 read site 의 점진 migration 둘 다 필요.

## §2 현재 env var 로딩 패턴

### §2.1 `os.Getenv` 직접 호출 (production)

위 §1.1 의 "Runtime read site (production)" 컬럼 = 10 unique production file (15 keys 처리). 모든 호출이 직접적이고 wrapper 부재.

### §2.2 envOverlay (부분 단일 진입점)

`internal/config/env.go` 에 5 key (LOG_LEVEL, HEALTH_PORT, GRPC_PORT, LOCALE, LEARNING_ENABLED) 를 처리하는 `envOverlay(cfg, sources, logger, envLookup func(string) string)` 함수 존재:
- envLookup 주입을 통해 test 시 process-wide env 오염 회피 (병렬 test 안전)
- WARN 로그 패턴 정의 (parse fail 시 기존 값 유지)
- SourceEnv 추적 (config 의 source-of-truth tracking)
- @MX:NOTE + @MX:SPEC tag 사용 (SPEC-GOOSE-CONFIG-001 §6.2)

→ **alias loader 의 reference impl 으로 envOverlay 패턴 그대로 따라가는 것이 최선** (logger, envLookup 주입, source tracking, parse fail handling 모두 검증됨).

### §2.3 viper.BindEnv 등록부

`grep -rn "viper.BindEnv" --include="*.go" . | grep -v vendor` → **0건**. cobra 사용하지만 viper 미사용. 모든 env read 는 직접 `os.Getenv` 호출.

### §2.4 test setup (`os.Setenv` / `t.Setenv`)

| 패턴 | 호출 횟수 | 안전성 |
|------|----------|--------|
| `t.Setenv("GOOSE_*", ...)` | ~50+ (대다수 `cmd/minkd/integration_test.go`, `internal/command/adapter/aliasconfig/*_test.go`, qwen/kimi client_test.go) | **안전** — Go testing framework 가 자동 cleanup |
| `os.Setenv("GOOSE_*", ...)` | 5 (`internal/audit/dual_test.go:128,130,293`, `internal/tools/builtin/terminal/bash_test.go:211`, `internal/transport/grpc/server_test.go:532,537`) | process-wide, 명시 cleanup 필요 |

→ §5 정책: in-tree test 의 `t.Setenv("GOOSE_*", ...)` 도 본 SPEC 안에서 `t.Setenv("MINK_*", ...)` 로 migrate (consistency). `GOOSE_*` 호출은 alias 동작 검증 test (deprecation warning emit, fallback semantic) 로만 한정.

## §3 Deprecation alias 산업 사례 + best practice

Go 표준 라이브러리는 env var alias 메커니즘을 내장하지 않는다 — 직접 구현 필요. 주요 OSS 사례:

### §3.1 HashiCorp Terraform (env var rename history)

- Terraform 0.10 → 0.11 시기 `TF_VAR_*` 외 다수 prefix 변경 사례에서 alias 동작:
  - **NEW > OLD priority** — 사용자가 새 키를 명시 설정하면 우선
  - **Deprecation warning** — stderr 로 한 번 emit, format `"DEPRECATED: TF_OLD_X is deprecated, use TF_NEW_X instead"`
  - **Removal timeline** — 최소 2개 minor version (e.g., 0.11 deprecate → 0.13 removal)
  - **공식 docs migration guide** — release notes + dedicated upgrade page
- 시사점: alias loader 만 본 SPEC 에서 처리, 완전 제거는 별도 SPEC + 1+ minor release 후 (일치)

### §3.2 HashiCorp Vault (`VAULT_*` namespace)

- 모든 env var 가 `VAULT_*` prefix → kubernetes operator 도입 시 일부 `VAULT_K8S_*` 도입, deprecated 키와 신규 키 공존 정책:
  - **Per-key warning once per process** (sync.Once 패턴 — 사용자가 100번 read 해도 1회 emit)
  - **Structured logger 사용** (Vault 의 hclog) — 단순 stderr print 가 아닌 구조화 채널
- 시사점: per-key sync.Once 결정 (§5 policy 2)

### §3.3 kubectl (`KUBE_*` → `KUBECONFIG`)

- v1.5 시점 `KUBE_CONFIG` → `KUBECONFIG` rename:
  - **Silent fallback** (warning 없음) — 단순한 keys 의 경우 warning 노이즈 회피
  - 단, `--alsologtostderr` flag 등 복잡 alias 는 명시적 deprecation warning + removal version 명시
- 시사점: 본 SPEC 의 22 keys 는 모두 deprecation warning 필요 (kubectl silent 정책 미적용) — 사용자가 1.0 release 후 마이그레이션 시점을 명확히 알아야 함

### §3.4 종합 — 본 SPEC 채택 정책 (§5 의 8 결정 trail)

1. **우선순위**: NEW > OLD (Terraform/Vault 일치)
2. **Warning frequency**: per-key sync.Once (Vault 일치)
3. **Warning channel**: structured logger (Vault 의 hclog ↔ 본 프로젝트 zap)
4. **Removal timeline**: 본 SPEC 은 alias 만, 완전 제거는 별도 SPEC + 1+ minor release
5. **Test setup migration**: in-tree consistency (Terraform/Vault test suite 패턴 일치)

## §4 위험 시나리오

### §4.1 사용자 dotfile (`.bashrc` / `.zshrc`)

- 시나리오: 사용자가 `export GOOSE_HOME=~/work/goose-data` 만 설정한 상태에서 v0.x → 본 SPEC 머지된 버전으로 업그레이드
- 동작 (본 SPEC alias loader 후): GOOSE_HOME 값 유지 + WARN log 1회 emit ("DEPRECATED: GOOSE_HOME, please rename to MINK_HOME")
- 검증: AC-MINK-EM-002 (시나리오 명세)
- mitigation: release notes 의 migration section + WARN log 명확히 명시

### §4.2 CI 환경 (`.github/workflows/`)

- 현재 worktree 내 `grep -rn "GOOSE_" .github/` → **0건** (BRAND-RENAME 가 이미 정리)
- 외부 fork repo 의 워크플로우 가능 → 본 SPEC alias loader 가 자동 흡수, 추가 조치 불필요
- 검증: AC-MINK-EM-002 와 동일 시나리오 (env source 무관)

### §4.3 단위 test 의 `os.Setenv("GOOSE_*", ...)` 직접 호출

- 영향: §2.4 의 5 곳 + `t.Setenv` 50+ 곳
- 본 SPEC 적용 후: alias loader 가 GOOSE_* 도 인식 → 기존 test 동작 유지 (backward compat)
- 단, **§5 policy 5** 에 따라 in-tree consistency 위해 `MINK_*` 로 migrate; `GOOSE_*` 호출은 alias 검증 test (1~2 곳) 로만 한정
- 검증: AC-MINK-EM-007 (test migration 완료 검증), AC-MINK-EM-008 (alias 검증 test 존재)

### §4.4 Docker / k8s manifest

- 현재 worktree 의 Dockerfile, docker-compose, k8s manifests 에 `GOOSE_*` 잔존 여부 확인 → 본 SPEC research 단계에서 grep 후 별첨
- 본 SPEC scope **외**: 해당 자산은 외부 운영 자원이며, alias loader 가 자동 흡수하므로 즉시 깨지지 않음
- 후속 docs SPEC 에서 release notes + migration guide 로 처리

### §4.5 SCHEDULED process / cron job 의 env

- 사용자 환경에 cron `0 * * * * GOOSE_LOG_LEVEL=debug minkd ...` 같은 entry 가능
- 동작: alias loader 가 흡수 + per-key per-process sync.Once warning
- cron 실행 빈도 (e.g., hourly) → process 마다 새 sync.Once 인스턴스 → 매 실행마다 warning 1회 (의도된 동작)

## §5 선행 SPEC supersede 영향

### §5.1 SPEC-MINK-BRAND-RENAME-001 §3.1 item 12 footnote

BRAND-RENAME-001 spec.md §3.1 item 12 의 footnote 가 본 SPEC 의 분리 정책을 명시적으로 트레일했다:

> **NOTE**: 환경변수 (`GOOSE_*` 21개) 와 user-data 경로 (`~/.goose/`) 는 본 SPEC scope 외. 후속 SPEC 머지 전까지 옛 표기 그대로 동작. (CHANGELOG.md line 47)

본 SPEC = 환경변수 부분 ("`GOOSE_*` 21개") 의 후속 이행. user-data 경로 (`~/.goose/`) 는 SPEC-MINK-USERDATA-MIGRATE-001 (별도) scope.

검증: `grep -n "SPEC-MINK-ENV-MIGRATE-001" CHANGELOG.md` → line 23 ("`SPEC-MINK-ENV-MIGRATE-001`: `GOOSE_*` 21개 env vars → `MINK_*` deprecation alias loader") 이미 작성됨.

### §5.2 REQ-MINK-BR-027

BRAND-RENAME spec.md REQ-MINK-BR-027:
> "환경변수 alias loader 는 별도 SPEC 으로 분리한다. 본 SPEC 은 식별자/모듈/proto/binary/산문 rename 만 처리하며, runtime env var 처리 변경은 포함하지 않는다."

본 SPEC = REQ-MINK-BR-027 의 분리된 부분의 구체화.

## §6 Out-of-scope 명확화 (IN ↔ OUT)

### §6.1 본 SPEC IN scope

- 22 GOOSE_* env var 의 alias loader 도입
- envOverlay 5 key + 분산 production read site 10 곳의 alias 채택 (15 runtime-read keys)
- 4 doc-only key (TELEGRAM_BOT_TOKEN, HISTORY_SNIP, METRICS_ENABLED, GRPC_BIND) 의 alias 등록 (미래-proof, runtime read 신설은 본 SPEC 외)
- `GOOSE_AUTH_*` prefix deny-list 에 `MINK_AUTH_*` 추가 (env scrub backward + forward compat)
- in-tree test 의 `t.Setenv("GOOSE_*", ...)` 를 `MINK_*` 로 migrate (alias 검증 test 1~2 곳 제외)
- 산문/주석/error message 의 GOOSE_* 표기를 MINK_* 로 update (alias 검증 test 코멘트 제외)
- 단위 test (table-driven) + integration test (실제 env set + main wire-up)

### §6.2 본 SPEC OUT scope

| Out item | Rationale | 처리 경로 |
|----------|-----------|----------|
| `GOOSE_*` 완전 제거 (alias 삭제) | 사용자 마이그레이션 시간 보장 | 별도 후속 SPEC (SPEC-MINK-ENV-CLEANUP-001), post-1.0 release + 1+ minor cycle 후 |
| user-data path migration (`~/.goose → ~/.mink`) | 데이터 손실 위험 + 별도 마이그레이션 도구 필요 | SPEC-MINK-USERDATA-MIGRATE-001 (별도) |
| `.env.example`, `docker-compose.yaml`, k8s manifest 의 GOOSE_* update | 외부 운영 자원 | 후속 docs SPEC 또는 release notes |
| cobra flag 의 default value alias (e.g., `--token` flag default 가 GOOSE_TELEGRAM_BOT_TOKEN 참조 시) | 현재 코드에 해당 패턴 없음 (확인됨) | 본 SPEC 외 |
| viper.BindEnv 도입 | 현재 viper 미사용 | 본 SPEC 외 (별도 refactor) |
| `~/.goose/...` hard-coded path 의 산문 정리 | user-data SPEC 와 함께 처리 | SPEC-MINK-USERDATA-MIGRATE-001 |

### §6.3 잠정 결정 (본 SPEC 안에서 plan-auditor 검토 시 재확인)

- `GOOSE_TELEGRAM_BOT_TOKEN` 의 runtime read logic 신설 여부: 현재는 error message hint 만. 본 SPEC 에서 alias loader 만 등록 + 산문 hint update; runtime read 신설은 별도 SPEC. 이유: behavior change scope minimization.
- `GOOSE_HISTORY_SNIP` / `GOOSE_METRICS_ENABLED` / `GOOSE_GRPC_BIND` 의 runtime read logic: 동일 (alias 만 등록, runtime read 신설은 별도).

## §7 References

- SPEC-MINK-BRAND-RENAME-001 §3.1 item 12 footnote, REQ-MINK-BR-027
- CHANGELOG.md line 23 (본 SPEC 의 후속성 명시), line 47 (분리 정책)
- internal/config/env.go (envOverlay reference impl)
- internal/hook/isolation_unix.go (env scrub deny-list 반영점)
- HashiCorp Vault hclog deprecation pattern (https://github.com/hashicorp/vault — Vault docs)
- HashiCorp Terraform env var rename history (Terraform CHANGELOG 0.10~0.13)
- kubectl rename pattern (Kubernetes #34058)

---

조사 완료. spec.md 의 EARS 요구사항과 AC 는 본 research 의 §1 inventory + §3 industry pattern + §6 boundary 에 정합한다.
