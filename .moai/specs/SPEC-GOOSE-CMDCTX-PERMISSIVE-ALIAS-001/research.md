# Research — SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001

> Permissive alias mode — CMDCTX v0.2 amendment 사전 분석.
>
> 본 문서는 `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` v0.1.1 (implemented, FROZEN status) 의 §6.4 alias 해석 알고리즘과 §Risks R2 를 기반으로 한 amendment 제안의 근거를 정리한다.

---

## 1. 배경 (Why now?)

### 1.1 CMDCTX-001 의 strict-only 알고리즘

`SPEC-GOOSE-CMDCTX-001` v0.1.1 §6.4 의 `ResolveModelAlias` 알고리즘은 다음과 같이 strict allow-list 정책을 가진다:

```
ResolveModelAlias(alias):
  1. if registry == nil: return ErrUnknownModel
  2. canonical := aliasMap.get(alias) or alias
  3. parts := SplitN(canonical, "/", 2)
  4. if len(parts) != 2: return ErrUnknownModel
  5. provider, model := parts[0], parts[1]
  6. meta, ok := registry.Get(provider); if !ok: return ErrUnknownModel
  7. if model NOT in meta.SuggestedModels:
       return ErrUnknownModel    ← 본 SPEC 의 amendment 대상
  8. return &ModelInfo{ID: provider+"/"+model, ...}, nil
```

해당 §6.4 주석은 다음과 같이 미래 옵션을 이미 인지하고 있었다:

> ```
> 7. if model NOT in meta.SuggestedModels:
>      (allow-list strict mode: return ErrUnknownModel)
>      (alternative: permissive mode: still return ModelInfo since registry
>       does not enumerate every model — config-driven)
>      Default: strict.
> ```

즉 strict 가 default 인 점은 의도된 설계이지만, **permissive 분기 자체는 spec 본문에 stub 형태로만 남겨졌다**. 본 SPEC 은 그 stub 을 정식 분기로 승격(promote)한다.

### 1.2 CMDCTX-001 §Risks R2 의 명시적 인지

`spec.md` §9 Risks 표 R2:

| 리스크 | 영향 | 완화 |
|--------|----|------|
| R2 — `ResolveModelAlias` 의 strict mode 가 사용자 친화성을 해침 (SuggestedModels 에 없는 정당한 모델 거부) | 중 | Optional REQ-CMDCTX-017 이 alias map override 경로 제공. 후속 wiring SPEC에서 permissive mode 옵션 추가 가능. |

→ 본 SPEC 이 그 "후속 wiring SPEC" 에 해당한다.

### 1.3 CMDCTX-001 §Exclusions #7 의 placeholder

`spec.md` §Exclusions:

> 7. **Permissive alias mode** — SuggestedModels 에 없는 모델 허용 옵션. 본 SPEC은 strict only. 후속 SPEC (TBD-SPEC-ID, 본 SPEC 머지 후 별도 plan 필요).

→ 본 SPEC 의 ID `SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001` 가 그 TBD-SPEC-ID 자리를 채운다.

---

## 2. 실제 운영상 문제 (User-facing pain)

### 2.1 OpenRouter 같은 multi-tenant provider 의 nested model ID

OpenRouter 는 router 형태의 LLM gateway 로, 단일 provider 명(`openrouter`) 아래 수백~수천 개의 nested model ID 를 노출한다. 예:

- `deepseek/deepseek-r1:free`
- `meta-llama/llama-3.3-70b-instruct:free`
- `google/gemini-2.0-flash-exp:free`
- `anthropic/claude-3.5-sonnet`

이런 모델은 OpenRouter 엔드포인트가 동적으로 routing 하므로, ProviderMeta 정의 시점에 `SuggestedModels` 로 풀 enumerate 하는 것이 비현실적이다. ROUTER-001 의 ProviderMeta 는 "권장(suggested)" 모델만 노출하는 것이 본래 설계 의도이다. 그러나 CMDCTX-001 §6.4 step 7 은 그 SuggestedModels 를 strict allow-list 로 사용해버려, 사용자가 `/model openrouter/deepseek/deepseek-r1:free` 입력 시 `ErrUnknownModel` 로 거부된다.

이는 ROUTER-001 의 SuggestedModels 의미("힌트") 와 CMDCTX-001 의 사용 방식("게이트") 사이의 의미론적 mismatch 이다.

### 2.2 alias map override 만으로는 부족

CMDCTX-001 REQ-CMDCTX-017 이 제공하는 `aliasMap` 은 **명시적으로 등록된 단일 단어 alias** (예: `opus` → `anthropic/claude-opus-4-7`) 만 처리한다. OpenRouter 처럼 모델 ID 자체가 nested 인 경우, 모든 모델을 aliasMap 에 사전 등록하는 것은:

1. 동적 모델 추가에 대응 불가 (OpenRouter 신규 모델 지속 출시).
2. config 파일이 비대해짐 (수천 개 항목).
3. typo 시에도 등록되지 않은 모델로 인식되어 `ErrUnknownModel` 로 fallback (개선되지 않음).

따라서 **provider-level opt-in 또는 global permissive flag** 가 별도 필요하다.

---

## 3. 결정 옵션 (Design alternatives)

### 3.1 옵션 A — `Options.AliasResolveMode` enum 필드 (선택)

ContextAdapter `Options` 에 enum 필드 추가:

```go
type AliasResolveMode int

const (
    AliasResolveStrict             AliasResolveMode = iota // 기본값. CMDCTX-001 v0.1.1 동작과 동일.
    AliasResolveModePermissiveProvider                     // provider lookup 성공 시 model 무조건 허용 + warn-log.
    AliasResolveModePermissive                             // provider lookup 실패 시에도 ErrUnknownModel 반환은 유지하되, model 검증은 생략.
)

type Options struct {
    Registry        *router.ProviderRegistry
    LoopController  LoopController
    AliasMap        map[string]string
    AliasResolveMode AliasResolveMode  // ⬅︎ 신규
    GetwdFn         func() (string, error)
    Logger          Logger
}
```

장점:

- 단일 진입점, 명확한 계약.
- backward compatibility: zero-value `AliasResolveStrict` 가 기본 → 기존 코드 동작 유지.
- enum 으로 미래 mode 추가 용이 (예: `AliasResolveExperimental`, `AliasResolveDevPreview` 등).

단점:

- 모든 provider 에 동일 정책 적용. provider-별 정책 차이 표현 불가 (예: openrouter 만 permissive, anthropic 은 strict).

### 3.2 옵션 B — `ProviderMeta.AllowUnsuggestedModels` per-provider flag (대안)

`internal/llm/router/registry.go` 의 `ProviderMeta` 에 boolean 추가:

```go
type ProviderMeta struct {
    Name             string
    DisplayName      string
    SuggestedModels  []string
    AllowUnsuggestedModels bool  // ⬅︎ 신규
    // ...
}
```

장점:

- per-provider 세밀 제어. openrouter 만 `true` 로 설정 가능.
- ROUTER-001 의 의미론적 정합성 회복 (SuggestedModels 가 "힌트" 임을 provider 자신이 선언).

단점:

- ROUTER-001 SPEC 의 implemented status 를 침범 (FROZEN 자산 변경 필요).
- ROUTER-001 SPEC 의 별도 amendment 가 추가로 필요해져서 변경 surface 가 두 SPEC 으로 확장됨.
- ProviderRegistry 는 `internal/llm/router/` 에 거주, ContextAdapter 의 책임 경계를 흐림.

### 3.3 옵션 C — adapter `aliasMap` 의 wildcard syntax `provider/*` (보조)

`aliasMap` 의 키에 wildcard 허용:

```yaml
# ~/.goose/aliases.yaml (CMDCTX-001 REQ-CMDCTX-017 의 후속 config)
openrouter/*: passthrough
opus: anthropic/claude-opus-4-7
```

→ adapter 가 `openrouter/deepseek/deepseek-r1:free` 입력 받으면 `openrouter/*` rule 매칭 → strict 검증 우회.

장점:

- aliasMap 자체의 확장이라 ContextAdapter 의 기존 자산 재활용.
- per-provider 제어 가능.

단점:

- aliasMap 의 의미론을 "alias 등록" 에서 "정책 등록" 으로 확대 → 혼란.
- wildcard 매칭 규칙이 복잡해짐 (specificity, ordering).

### 3.4 결정 — 옵션 A 채택, 옵션 C 는 Optional 보조

**채택: 옵션 A (`AliasResolveMode` enum on Options)**.

근거:

1. CMDCTX-001 v0.1.1 의 변경 surface 를 adapter 패키지로 한정 — ROUTER-001 (FROZEN) 변경 불필요.
2. ContextAdapter 의 책임 경계 (registry 를 read-only 로 사용) 유지.
3. enum default 값이 strict 이므로 backward compat 자동 보장.
4. 향후 옵션 C (wildcard aliasMap) 는 Optional REQ 로 추가하여 보완 가능.

옵션 B 는 ROUTER-001 변경이 필수라 본 SPEC scope 를 초과한다. 별도 후속 SPEC 으로 분리 가능하지만 본 amendment 의 scope 외.

---

## 4. CMDCTX-001 amendment 형태 (v0.2.0)

### 4.1 변경 surface 요약

CMDCTX-001 본문에 가해지는 변경 (본 SPEC implementation 시점에만 — 지금 plan 단계에서는 변경 금지):

1. **frontmatter**: `version: 0.1.1` → `0.2.0`. status 는 implemented 유지(또는 별도 정책 결정).
2. **HISTORY**: v0.2.0 항목 추가 — "permissive alias mode opt-in via `Options.AliasResolveMode` 필드 추가. SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 에 의한 amendment."
3. **§6.2 핵심 타입**: `Options` struct 에 `AliasResolveMode` 필드 추가. `AliasResolveMode` enum 타입 + 상수 3개 정의 추가.
4. **§6.4 alias 해석 알고리즘**: step 7 분기 추가:
   ```
   7. if model NOT in meta.SuggestedModels:
        switch a.aliasResolveMode:
          case AliasResolveStrict (default):
            return nil, ErrUnknownModel
          case AliasResolveModePermissiveProvider, AliasResolveModePermissive:
            if a.logger != nil:
              a.logger.Warn("model not in SuggestedModels, permissive mode allows", ...)
            // fall through to step 8
   8. return &ModelInfo{...}, nil
   ```
   `AliasResolveModePermissive` 는 step 6 의 provider lookup 실패 시에도 동작이 다를 수 있는가? — **본 SPEC 은 NO 로 결정**. provider unknown 은 여전히 hard fail. mode 별 차이는 step 7 에서만.
5. **신규 enum 타입 godoc**: warn-log 메시지 형식 명시.
6. **§9 Risks**: R2 완화에 본 SPEC 참조 추가.
7. **AC 신규**: permissive 모드의 happy path / unknown provider 경계 / strict default backward compat 검증 AC 3~5 개 추가.

### 4.2 변경 strategy

CMDCTX-001 SPEC 본문 변경은 본 SPEC 의 **implementation phase** 에만 수행한다. 지금은 plan 단계이므로 CMDCTX-001 본문은 건드리지 않는다.

본 SPEC 머지 → run phase 진입 시점:
- 본 SPEC 의 spec.md 가 CMDCTX-001 v0.2.0 amendment 내용을 정의.
- 실제 코드 변경은 `internal/command/adapter/adapter.go`, `options.go` (또는 등가) 에 가해진다.
- CMDCTX-001 spec.md 본문도 같은 PR 에서 v0.1.1 → v0.2.0 으로 갱신 (HISTORY 항목 추가, §6.2/§6.4 본문 갱신).

본 SPEC 자체는 그 amendment 의 spec-level 합의서이다. 코드 + CMDCTX-001 v0.2.0 본문 = 본 SPEC 의 산출물.

---

## 5. permissive 모드의 위험 (Risks of relaxation)

### 5.1 typo 로 인한 잘못된 모델 호출

가장 명백한 리스크. 사용자가 `/model anthropic/claude-opuss-4-7` (오타) 입력 시 strict 모드에서는 `ErrUnknownModel` 로 즉시 거부되지만, permissive 모드에서는 그대로 ModelInfo 생성 → API 호출 → provider 단의 HTTP 4xx 응답 → loop 단의 에러 처리로 fallback. 사용자 경험상 "왜 안 되지" 까지의 latency 가 증가한다.

완화책:

- **Warn-log 의무화**: permissive 모드에서 SuggestedModels 미포함 모델 사용 시 logger.Warn 으로 알림. 사용자가 로그 확인 시 typo 의심 가능.
- **Telemetry counter 연계**: 후속 SPEC-GOOSE-TELEMETRY-001 (TBD) 가 도입되면 `cmdctx_permissive_unsuggested_model_total` counter 증가. 운영자가 provider 별 typo 빈도 모니터링 가능. (본 SPEC 의 §Exclusions 참조.)
- **error-class 표준화**: provider HTTP 4xx 응답이 SPEC-GOOSE-ERROR-CLASS-001 의 통일된 에러 분류로 전파되어 user-facing message 가 "Model not recognized by provider, check spelling" 형태로 표준화될 가능성 (해당 SPEC 의 책임).

### 5.2 CMDCTX-001 SPEC 본문 변경의 governance

CMDCTX-001 v0.1.1 은 status: implemented 이다. FROZEN 자산을 변경하려면:

- HISTORY 에 v0.2.0 amendment 항목 명시.
- amendment 가 backward compatible (default behavior 동일) 임을 spec 자체에서 증명.
- AC 추가는 기존 18 REQ / 19 AC 에 누적 (제거/변경 금지).

본 SPEC 은 위 governance 를 §Dependencies 와 §HISTORY 에 명시하여 변경 추적성을 보장한다.

### 5.3 mode 폭주 (mode proliferation)

Strict / PermissiveProvider / Permissive 3 종 도입 후 향후 `Experimental`, `DevPreview`, `BetaModel` 등 mode 무한 추가 가능성. 본 SPEC 은:

- enum 정의 시 godoc 에 "신규 mode 추가는 별도 SPEC 필요" 명기.
- 각 mode 별 명확한 의미론 정의 (어떤 step 에서 어떤 분기 차이를 가지는지).

→ enum 추가 자체에 amendment 문턱을 두어 폭주 방지.

---

## 6. 구현 영역 요약 (Implementation surface preview)

### 6.1 코드 변경 영역

- `internal/command/adapter/options.go` (또는 `adapter.go` 내) — `AliasResolveMode` enum + 상수 정의.
- `internal/command/adapter/adapter.go` — `Options` struct 에 필드 추가, `New(...)` 가 그것을 `*ContextAdapter` 에 저장.
- `internal/command/adapter/alias.go` — `ResolveModelAlias` step 7 분기 추가.
- `internal/command/adapter/adapter_test.go` (or `alias_test.go`) — 신규 AC 검증 table cases 추가.

### 6.2 CMDCTX-001 SPEC 본문 변경 영역 (run phase에서)

- frontmatter: `version 0.1.1 → 0.2.0`, `updated_at` 갱신.
- HISTORY: v0.2.0 항목 1줄.
- §6.2 / §6.4 본문 갱신.
- §9 R2 완화에 본 SPEC 링크.
- (선택) AC 표 갱신 — 신규 AC 가 본 SPEC 의 spec.md 에 거주하므로 CMDCTX-001 본문엔 cross-reference 만 추가.

### 6.3 신규 코드 검증

- `go test -race ./internal/command/adapter/...` — race detector clean.
- table-driven test: strict default, PermissiveProvider hit/miss, Permissive provider-unknown, Permissive model-suggested.
- coverage: 기존 ≥ 90% 유지 (감소 금지).
- golangci-lint 0 issues 유지.

---

## 7. 본 SPEC 의 IN/OUT scope (한 줄)

- **IN**: `Options.AliasResolveMode` enum 신설 + `ResolveModelAlias` step 7 분기 + warn-log + AC 추가 + CMDCTX-001 v0.2.0 amendment governance.
- **OUT**: ProviderMeta `AllowUnsuggestedModels` 필드 (옵션 B 미채택), aliasMap wildcard syntax 의 적극 구현(Optional REQ 로 stub 만), telemetry counter (SPEC-GOOSE-TELEMETRY-001), config hot-reload (SPEC-GOOSE-HOTRELOAD-001), ProviderRegistry mutation.

---

## 8. 참고 (References)

- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` v0.1.1 — 부모 SPEC. §6.4, §9 R2, §Exclusions #7.
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` — `command.ErrUnknownModel` 정의 위치(`internal/command/errors.go:23-25`).
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — ProviderMeta 의미론, SuggestedModels 의 본래 의도("권장").
- `.moai/specs/SPEC-GOOSE-ERROR-CLASS-001/spec.md` — provider HTTP 4xx 응답의 통일 에러 분류(완화책 #3).
- (TBD) `SPEC-GOOSE-TELEMETRY-001` — counter 연계 후속 SPEC.
- (TBD) `SPEC-GOOSE-HOTRELOAD-001` — registry/aliasMap hot-reload 후속 SPEC.

---

Version: 0.1.0
Last Updated: 2026-04-27
