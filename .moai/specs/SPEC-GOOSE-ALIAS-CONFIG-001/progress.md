## SPEC-GOOSE-ALIAS-CONFIG-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd 가정 — run phase 진입 시 재확인)
- Harness: standard (file_count<10 예상, 단일 Go domain, 보안/결제 영역 아님)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/command/adapter/aliasconfig/` 신규, 의존 SPEC 인터페이스 모두 implemented FROZEN
- Branch base: main (의존 SPEC 모두 main 머지됨, PR #52 c018ec5 기준)

### 의존 SPEC 상태 확인

| SPEC | Status | 본 SPEC 와의 관계 |
|------|--------|----------------|
| SPEC-GOOSE-CMDCTX-001 | implemented (FROZEN, PR #52 c018ec5) | `adapter.Options.AliasMap` 필드 consumer — 본 SPEC 가 채워줌. surface 변경 없음. |
| SPEC-GOOSE-ROUTER-001 | implemented (FROZEN) | `*router.ProviderRegistry`, `ProviderMeta.SuggestedModels` read-only validation 사용. |
| SPEC-GOOSE-CONFIG-001 | implemented (FROZEN) | `resolveGooseHome` 패턴 차용 (env GOOSE_HOME → $HOME/.goose). 별도 패키지로 분리. |

### Phase Log

- 2026-04-27 plan phase 시작
  - 부모 SPEC: SPEC-GOOSE-CMDCTX-001 implemented 직후, alias 데이터 소스 wiring 결손 식별
  - 본 SPEC 산출물:
    - `research.md` — 패키지 위치, 스키마 결정, validation 모드, filesystem layout, 8개 결정 명세
    - `spec.md` — 24 REQ / 27 AC / EARS 5 카테고리 (Ubiquitous 6, Event 3, State 5, Unwanted 8, Optional 3) + 10 Exclusions + 8 §Data Model 절
    - `progress.md` — 본 파일
  - 다음 단계 후보:
    - (a) plan-auditor 사이클 (independent SPEC 검토, EARS 컴플라이언스, 의존 SPEC 영향 확인)
    - (b) 사용자 검토 → /moai run 분기 결정
  - Open questions (사용자 결정 필요):
    1. 패키지 위치 최종 확정: research §2.1 결정 6 의 `internal/command/adapter/aliasconfig/` 가 권장 — 변경 시 spec.md §6.1 / §11 / AC-ALIAS-051 수정 필요.
    2. lenient 모드 의미론: `Validate` 가 in-place 삭제 vs 호출자가 errors 보고 직접 삭제. 권장: in-place 삭제 (REQ-ALIAS-024 §6.5 구현 노트). 결정 시 spec.md §6.2 godoc 명시 필요.
    3. `GOOSE_ALIAS_STRICT` env 기본값 — 본 SPEC 은 strict=true 기본. fail-fast 안전 default 채택.

### 산출 파일

- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/research.md`
- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md`
- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/progress.md`

### Next-step Hand-off Notes

- Run phase 시 manager-tdd 또는 manager-ddd 가 본 SPEC 으로 진입할 때 다음 자료 우선 로딩:
  - `internal/command/adapter/adapter.go:49-62` (Options.AliasMap 필드 — consumer)
  - `internal/command/adapter/alias.go` (resolveAlias 알고리즘 — 본 SPEC 변경 금지)
  - `internal/config/config.go:256-266` (`resolveGooseHome` — 차용 패턴)
  - `internal/llm/router/registry.go` (ProviderRegistry API — validation 시 read-only 사용)
- 검증 필수 정적 체크:
  - AC-ALIAS-050: `go.mod` diff CI gate (신규 외부 의존성 0건)
  - AC-ALIAS-051: 패키지 import 화이트리스트 (`go list -deps`)
- 신규 작성 파일 (예상):
  - `internal/command/adapter/aliasconfig/loader.go`
  - `internal/command/adapter/aliasconfig/validate.go`
  - `internal/command/adapter/aliasconfig/errors.go`
  - `internal/command/adapter/aliasconfig/loader_test.go`
  - `internal/command/adapter/aliasconfig/validate_test.go`
- 수정 예상 파일:
  - `cmd/goosed/main.go` 또는 등가 부트스트랩 — `aliasconfig.LoadDefault` + `aliasconfig.Validate` + `adapter.New(Options{AliasMap: ...})` wiring 1회 추가
