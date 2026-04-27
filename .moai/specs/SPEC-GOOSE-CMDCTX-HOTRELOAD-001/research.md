# SPEC-GOOSE-CMDCTX-HOTRELOAD-001 — Research

> ContextAdapter 의 `registry` / `aliasMap` 필드를 atomic.Pointer 기반 hot-reload 가능 형태로 전환하기 위한 사전 조사. CMDCTX-001 v0.1.1 implemented 시점 코드를 분석하고, alias config 파일 watcher 트리거 (ALIAS-CONFIG-001) 와의 연결, fsnotify cross-platform 일관성, debounce 정책, 다른 CMDCTX-001 amendment SPEC (PERMISSIVE-ALIAS, TELEMETRY) 와의 amendment 충돌 가능성을 정리한다.

---

## 1. 목적과 범위

### 1.1 본 연구가 해결하려는 결손

`SPEC-GOOSE-CMDCTX-001` v0.1.1 (implemented, FROZEN, PR #52 c018ec5) 의 `ContextAdapter` 는 다음 두 필드가 `New(opts)` 시점에 immutable 하다:

- `registry *router.ProviderRegistry` — 포인터 자체는 가변이지만, 필드는 `New(...)` 한 번 할당 후 swap 경로 부재.
- `aliasMap map[string]string` — read-only by convention, immutable map after construction.

결과적으로, 사용자가 다음 행동을 했을 때 변경이 즉시 반영되지 않는다:

- `~/.goose/aliases.yaml` 파일을 수정 (alias 추가/삭제)
- 새 provider 를 registry 에 등록 (ROUTER-001 후속 dynamic registration SPEC)
- registry 내 SuggestedModels 갱신

현재로서는 daemon 재시작이 유일한 반영 수단이며, 이는 다음 UX 결손을 유발:

- `/model new-alias` 추가 직후 daemon 재시작 부담
- 멀티세션 동시 실행 환경에서 alias 변경의 propagation 불가
- 운영 환경에서 alias 갱신을 위한 scheduled restart 필요

### 1.2 본 연구가 정의하려는 출력

- ContextAdapter 의 `registry` / `aliasMap` 필드를 `atomic.Pointer[T]` 로 전환하는 amendment 의 구체 surface
- WithContext children 와 부모/자식 간 일관된 visibility 보장 알고리즘
- Hot-reload 트리거 옵션 비교 (fsnotify watcher / SIGHUP / 명시적 reload command)
- debounce 정책 / cross-platform fsnotify 일관성 / reload 실패 시 fallback 전략
- 본 SPEC 이 CMDCTX-001 v0.2.0 amendment (PERMISSIVE-ALIAS-001 와 동시 amendment) 와 충돌할 가능성과 완화 방안

### 1.3 본 연구가 다루지 않는 범위

- alias config 파일 schema / loader 자체 — `SPEC-GOOSE-ALIAS-CONFIG-001` (planned) 책임
- registry hot-reload via SIGHUP / HTTP API — 별도 SPEC (TBD)
- provider plugin hot-load (런타임 .so / Go plugin 시스템) — out of scope
- `LoopController` / `loop.State` 의 hot-reload — 본 SPEC 비대상 (loop state 는 single-owner, REQ-CMDCTX-016 invariant)

---

## 2. CMDCTX-001 v0.1.1 의 ContextAdapter 분석

### 2.1 현 ContextAdapter struct (FROZEN reference)

CMDCTX-001 spec.md §6.2 에서 정의된 implemented surface:

```go
type ContextAdapter struct {
    registry   *router.ProviderRegistry  // immutable after New
    loopCtrl   LoopController            // immutable after New
    aliasMap   map[string]string         // immutable after New (by convention)
    planMode   *atomic.Bool              // shared via pointer (already hot-swappable per WithContext)
    getwdFn    func() (string, error)
    logger     Logger
    ctxHook    context.Context
}
```

### 2.2 변경 가능한 필드 vs 불가능한 필드

| 필드 | v0.1.1 가변성 | 본 SPEC 의 변경 의도 |
|------|-------------|--------------------|
| `registry` | 포인터, but 한 번 할당 후 미변경 | `*atomic.Pointer[router.ProviderRegistry]` 로 전환, ReloadRegistry API 신설 |
| `aliasMap` | 맵 reference, but 한 번 할당 후 미변경 | `*atomic.Pointer[map[string]string]` 로 전환, ReloadAliases API 신설 |
| `loopCtrl` | interface, immutable | **변경 없음** — loop hot-reload 는 본 SPEC 비대상 |
| `planMode` | `*atomic.Bool` (이미 atomic) | **변경 없음** — 기존 패턴 차용의 모범 |
| `getwdFn` | 함수 포인터, immutable | **변경 없음** |
| `logger` | interface, immutable | **변경 없음** |
| `ctxHook` | context, per-WithContext-clone | **변경 없음** |

핵심 통찰: `planMode *atomic.Bool` 패턴 (CMDCTX-001 §6.5) 가 이미 본 SPEC 이 적용하려는 atomic-pointer-shared 패턴의 reference 구현이다. WithContext shallow copy 가 부모/자식 간 atomic 포인터를 공유하여 SetPlanMode 호출이 즉시 visibility 됨이 이미 검증된다 (REQ-CMDCTX-005, AC-CMDCTX-014).

### 2.3 WithContext shallow copy 와의 상호작용

CMDCTX-001 §6.5:

```go
func (a *ContextAdapter) WithContext(ctx context.Context) *ContextAdapter {
    clone := *a              // shallow copy: planMode pointer 공유
    clone.ctxHook = ctx
    return &clone
}
```

shallow copy 후 `clone.registry` 와 `clone.aliasMap` 도 부모와 동일 포인터/리퍼런스를 공유한다. 본 SPEC 이 두 필드를 `*atomic.Pointer[T]` 로 전환하면:

- shallow copy 시 atomic.Pointer 자체의 포인터(즉 `*atomic.Pointer[T]`)가 복사되어 부모/자식이 동일 atomic instance 를 공유
- 부모의 `ReloadRegistry(newReg)` 호출은 자식의 `ResolveModelAlias` 호출에서 즉시 (atomic ordering 보장 내) 관찰됨
- 이는 planMode 의 invariant 와 동일한 단일 진실 공급원 (single source of truth) 보장

**중요**: atomic.Pointer 를 값 타입으로 두면 (즉 `atomic.Pointer[T]` 비-포인터), shallow copy 시 `noCopy` 가드 위반 (`go vet copylocks`). 따라서 반드시 `*atomic.Pointer[T]` 포인터 indirection 으로 사용해야 한다 — planMode 와 정확히 동일한 이유.

---

## 3. Hot-reload 트리거 옵션 비교

### 3.1 옵션 A — Filesystem Watcher (fsnotify)

`github.com/fsnotify/fsnotify` 라이브러리로 alias 파일을 감시하고 변경 이벤트 발생 시 reload.

**장점**:
- 사용자가 파일 편집기로 저장하면 즉시 반영 (UX 우수)
- daemon 재시작 / SIGHUP 송신 필요 없음
- macOS / Linux / Windows 지원 (fsnotify cross-platform abstraction)

**단점**:
- fsnotify cross-platform 의미론 불일치 (macOS FSEvents → coalescing, Linux inotify → 개별 이벤트, Windows ReadDirectoryChangesW → atomicity 차이)
- 빠른 연속 저장 (vim swap → atomic rename) 시 다중 이벤트 발생 → debounce 필요
- 외부 의존성 신규 추가 (`go.mod` 신규 require)

**결정 기준**: 외부 의존성 추가 부담 vs UX 자동성. ALIAS-CONFIG-001 의 §10 Exclusions #1 이 본 SPEC 에 hot-reload 책임을 위임하면서 "file watching" 을 명시했으므로, fsnotify 를 1차 선택지로 채택.

### 3.2 옵션 B — 명시적 reload command (`/reload aliases`)

`/reload aliases` 같은 슬래시 명령으로 명시적 reload 트리거.

**장점**:
- 외부 의존성 0
- reload 시점 사용자가 통제 — 의도하지 않은 reload 회피
- 새 builtin command 추가만으로 구현

**단점**:
- 사용자 경험: 파일 편집 후 매번 `/reload aliases` 입력 필요
- watcher 비대비 → 반자동화

**결정 기준**: 옵션 A 의 secondary 또는 fallback 채널로 동시 채택. fsnotify 가 unavailable / disabled 인 환경 (예: 일부 Docker volume mount, NFS) 에서 graceful fallback.

### 3.3 옵션 C — SIGHUP signal handler

POSIX SIGHUP 을 reload 시그널로 사용 (전통적 unix daemon 패턴).

**장점**:
- 외부 의존성 0
- `kill -HUP <pid>` 같은 ops-friendly 패턴
- shell script / systemd / launchd 통합 용이

**단점**:
- Windows 미지원 (SIGHUP 부재)
- daemon 외 CLI mode (직접 실행) 에서 비실용

**결정 기준**: 옵션 A 와 보완적 (cross-platform 우선). 본 SPEC v0.1.0 은 옵션 A (fsnotify) + 옵션 B (slash command) 만 채택, SIGHUP 은 후속 SPEC.

### 3.4 결정: Hybrid (A + B)

| 트리거 | 본 SPEC v0.1.0 채택 | 근거 |
|--------|-------------------|------|
| fsnotify watcher | Yes (REQ-HOTRELOAD-010) | 자동성 / UX 우수 |
| `/reload aliases` slash command | Yes (REQ-HOTRELOAD-013, Optional) | fallback / explicit control |
| SIGHUP | No — out of scope (Exclusions #2) | Windows 미지원 / 본 SPEC 1차 범위 외 |

옵션 A 와 B 는 모두 동일한 `ReloadAliases(newMap)` 내부 API 를 호출하므로 구현 비용 중복은 minimal.

---

## 4. atomic swap 패턴 설계

### 4.1 registry / aliasMap atomic.Pointer 전환

CMDCTX-001 v0.1.1 의 ContextAdapter 를 본 SPEC 이 다음과 같이 amendment 한다 (CMDCTX-001 v0.2.0 amendment governance):

```go
// AFTER amendment (v0.2.0)
type ContextAdapter struct {
    registry   *atomic.Pointer[router.ProviderRegistry]  // ← changed
    aliasMap   *atomic.Pointer[map[string]string]        // ← changed
    loopCtrl   LoopController                            // unchanged
    planMode   *atomic.Bool                              // unchanged
    getwdFn    func() (string, error)                    // unchanged
    logger     Logger                                    // unchanged
    ctxHook    context.Context                           // unchanged
}
```

**왜 둘 다 포인터 indirection 인가**:
- `atomic.Pointer[T]` 는 `noCopy` 가드를 가진다 (`go vet copylocks`). WithContext shallow copy 가 값 타입을 복사하면 경고 발생.
- planMode `*atomic.Bool` 와 동일 패턴 (CMDCTX-001 §6.6) — 일관성 + 검증된 패턴.

**왜 aliasMap 도 atomic.Pointer 인가**:
- map 자체는 동시 read 안전이지만 read 와 write 동시는 race.
- reload 는 새 map 을 생성하여 기존 map 을 통째로 교체하는 패턴 (copy-on-write).
- atomic.Pointer 의 `Store(newMap)` 는 단일 포인터 swap 으로 atomic.

### 4.2 New(...) 에서 atomic.Pointer 초기화

```go
func New(opts Options) *ContextAdapter {
    regPtr := new(atomic.Pointer[router.ProviderRegistry])
    if opts.Registry != nil {
        regPtr.Store(opts.Registry)
    }

    aliasPtr := new(atomic.Pointer[map[string]string])
    aliasMap := opts.AliasMap
    if aliasMap == nil {
        aliasMap = map[string]string{}
    }
    aliasPtr.Store(&aliasMap)

    return &ContextAdapter{
        registry: regPtr,
        aliasMap: aliasPtr,
        // ...
    }
}
```

핵심:
- `*atomic.Pointer[router.ProviderRegistry]` 는 항상 non-nil (nil dependency 라도 atomic.Pointer 자체는 존재)
- `Load()` 가 nil 반환 가능 → 호출자가 nil 검증 필요 (REQ-CMDCTX-014 의 graceful 의미론 유지)
- aliasMap 은 항상 non-nil 빈 맵으로 초기화하여 nil-pointer dereference 회피

### 4.3 ReloadAliases / ReloadRegistry API

```go
// ReloadAliases atomically swaps the alias map. Must be called with a fresh map
// (copy-on-write) — modifying the previous map after this call races with concurrent
// ResolveModelAlias readers.
//
// Returns nil on success. Returns an error if newMap fails Validate (caller's
// responsibility to invoke aliasconfig.Validate before this call; this method
// does NOT re-validate — separation of concerns).
func (a *ContextAdapter) ReloadAliases(newMap map[string]string) error {
    if newMap == nil {
        return ErrNilAliasMap
    }
    // Copy to ensure caller cannot mutate the live map post-swap.
    cloned := make(map[string]string, len(newMap))
    for k, v := range newMap {
        cloned[k] = v
    }
    a.aliasMap.Store(&cloned)
    return nil
}

// ReloadRegistry atomically swaps the provider registry pointer. The old
// registry is not torn down; callers are responsible for any cleanup
// (typically there is none — registry is a read-only data structure).
func (a *ContextAdapter) ReloadRegistry(newReg *router.ProviderRegistry) error {
    if newReg == nil {
        return ErrNilRegistry
    }
    a.registry.Store(newReg)
    return nil
}
```

### 4.4 ResolveModelAlias 의 read-side adaptation

CMDCTX-001 §6.4 알고리즘은 `a.registry` 와 `a.aliasMap` 을 직접 참조한다. amendment 후:

```go
func (a *ContextAdapter) ResolveModelAlias(alias string) (*command.ModelInfo, error) {
    reg := a.registry.Load()  // atomic snapshot
    if reg == nil {
        return nil, command.ErrUnknownModel
    }
    aliasMapPtr := a.aliasMap.Load()  // atomic snapshot
    var aliasMap map[string]string
    if aliasMapPtr != nil {
        aliasMap = *aliasMapPtr
    }
    // ... rest of algorithm uses local reg / aliasMap variables
}
```

핵심: read-side 는 atomic.Load() 한 번만 호출하여 snapshot 확보. 동일 함수 호출 내에서 reload 가 일어나도 snapshot 은 일관됨 (중간에 reload 되어도 함수 실행은 이전 값 사용 — eventual consistency).

### 4.5 WithContext children 와의 일관성

WithContext shallow copy 가 부모와 동일한 `*atomic.Pointer[T]` 포인터를 공유하므로:

```
Parent:  &ContextAdapter{registry: regPtr1, aliasMap: aliasPtr1, planMode: planPtr1, ...}
                                  ↓ shallow copy
Child:   &ContextAdapter{registry: regPtr1, aliasMap: aliasPtr1, planMode: planPtr1, ctxHook: ctx, ...}

Parent.ReloadAliases(newMap)
  → aliasPtr1.Store(&newMap)
  → Child.ResolveModelAlias() 의 a.aliasMap.Load() 가 newMap 즉시 관찰
```

planMode 와 정확히 동일한 invariant. CMDCTX-001 §6.6 의 "상태 공유 invariant" 절이 그대로 본 SPEC 에 적용된다.

---

## 5. fsnotify cross-platform 일관성

### 5.1 macOS / Linux / Windows 의 의미론 차이

| OS | Backend | Coalescing | rename 처리 |
|----|---------|-----------|------------|
| macOS | FSEvents (kqueue fallback) | Yes (시스템 차원 coalescing) | rename → CREATE+REMOVE 페어, atomicity 다름 |
| Linux | inotify | No (이벤트 단위 발생) | atomic rename → MOVED_TO 단일 이벤트 또는 CREATE+DELETE 페어 (mv 구현 의존) |
| Windows | ReadDirectoryChangesW | 부분 coalescing | rename → RENAMED_OLD_NAME + RENAMED_NEW_NAME |

### 5.2 vim atomic save 의 함정

대부분의 텍스트 에디터 (vim, neovim, helix, VSCode) 는 atomic-write 패턴 사용:

```
1. 임시 파일 ~/.goose/.aliases.yaml.swp 작성
2. fsync
3. rename(swp → aliases.yaml)
```

이 결과 fsnotify 가 다음 순서로 이벤트를 emit:

- macOS: `Remove(aliases.yaml) → Create(aliases.yaml)`
- Linux: `Rename(aliases.yaml) → Create(aliases.yaml)` 또는 단순 `Write`

watcher 가 `Create` 시점에 file open 을 시도하면 데이터가 아직 fsync 되지 않았을 수 있음 (race). 따라서 `Write` 이벤트 수신 후 일정 시간 (debounce window) 대기 후 read 가 안전.

### 5.3 watcher 디렉토리 단위 vs 파일 단위

**파일 단위 watch** (`watcher.Add("~/.goose/aliases.yaml")`):
- atomic rename 시 watch 가 풀림 (파일 inode 가 바뀜)
- 첫 reload 후 더 이상 이벤트 미수신

**디렉토리 단위 watch** (`watcher.Add("~/.goose/")`):
- 파일 생성/삭제/수정 모두 수신
- atomic rename 도 디렉토리 차원에서 보임 — 권장

결정: 디렉토리 단위 watch + 파일명 필터링 (`event.Name == "aliases.yaml"`).

### 5.4 debounce 정책

fsnotify burst 이벤트 (1 회 저장에 5+ 이벤트) 를 처리하기 위한 debounce:

| 옵션 | window | 트레이드오프 |
|------|--------|-------------|
| 50ms | 짧음 | 빠른 반영, 일부 burst 미합쳐짐 |
| 100ms | 중간 | 균형 (권장) |
| 250ms | 길음 | 느린 반영, atomic rename safety 증가 |

결정: **debounce 100ms** (REQ-HOTRELOAD-021).

알고리즘:
1. fsnotify 이벤트 수신
2. 100ms 타이머 reset (이전 타이머 cancel)
3. 100ms 동안 추가 이벤트 없으면 reload 실행
4. 추가 이벤트 발생 시 step 2 반복

---

## 6. Reload 실패 시 Fallback 전략

### 6.1 실패 케이스 분류

| 케이스 | 발생 사유 | 처리 |
|-------|--------|------|
| YAML malformed | 사용자가 syntax 오류 입력 | 기존 map 유지, error 반환, warn-log |
| Validate 실패 (strict) | unknown provider/model | 기존 map 유지, error 반환, warn-log |
| Validate 실패 (lenient) | unknown provider/model 일부 | 유효 entry 만 반영, invalid entry 제외, info-log |
| 파일 부재 (rename swap 직후) | atomic rename race | debounce 후 retry, 그래도 부재 시 빈 map 반영 (ALIAS-CONFIG-001 §4.1 의 graceful default) |
| 파일 권한 오류 | chmod 0000 | 기존 map 유지, warn-log |
| oversize file | 1 MiB cap 초과 | 기존 map 유지, error 반환 |

### 6.2 in-flight 호출 안전성

reload 도중 (`Validate` 가 실행 중일 때) 다른 goroutine 의 `ResolveModelAlias` 호출이 발생해도:

- read-side 는 `aliasMap.Load()` 로 atomic snapshot 확보 (구 map 또는 신 map 둘 중 하나, 일관성 보장)
- swap 은 단일 atomic.Store(...)
- race detector 는 atomic 연산을 감지하지 않음 → 0건 보장

### 6.3 active model fallback

reload 후 active model (현재 사용 중인 alias 의 canonical) 이 새 registry 에 부재하는 경우:

- v0.1.0 본 SPEC 은 fallback 정책을 **명시적으로 정의하지 않는다** — Optional REQ (REQ-HOTRELOAD-040) 로 후속 검토.
- 기본 동작: 이전 alias 의 cached canonical 을 그대로 사용 — `ResolveModelAlias` 가 새 lookup 을 강제하지 않음.
- 사용자가 명시적으로 `/model <alias>` 재호출 시 새 registry 기준 lookup 수행.

---

## 7. CMDCTX-001 amendment 충돌 가능성

### 7.1 PERMISSIVE-ALIAS-001 와의 충돌

`SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001` (planned, P4) 은 CMDCTX-001 v0.1.1 → v0.2.0 amendment 를 명시. 본 SPEC 도 동일 amendment 를 요구하므로 다음 충돌 가능:

| 충돌 영역 | PERMISSIVE-ALIAS-001 변경 | 본 SPEC 변경 | 충돌 여부 |
|----------|--------------------------|-------------|---------|
| `Options` struct | `AliasResolveMode` 필드 추가 | (변경 없음 — Options 외 ContextAdapter struct 만 변경) | 비충돌 |
| `ContextAdapter` struct | (변경 없음) | `registry`, `aliasMap` 을 atomic.Pointer 로 전환 | 비충돌 (서로 다른 필드) |
| `ResolveModelAlias` 알고리즘 | step 7 분기 추가 | step 1, 2 read-side 를 atomic.Load() 로 변경 | **부분 충돌** — 같은 함수 본문 변경, merge 시 conflict 가능 |
| spec.md frontmatter version | 0.1.1 → 0.2.0 | 0.1.1 → 0.2.0 (또는 0.3.0) | **버전 충돌** — 둘 중 하나가 먼저 머지되면 다른 한쪽이 0.2.0 → 0.3.0 으로 bump 필요 |
| HISTORY 항목 | v0.2.0 1줄 추가 | v0.2.0 또는 v0.3.0 1줄 추가 | 머지 순서 의존 |
| §6 Technical Approach | §6.4 step 7 본문 갱신 | §6.2 struct 갱신 + §6.4 step 1, 2 갱신 + §6.6 race 안전성 갱신 | **부분 충돌** — 같은 절 동시 수정 |

### 7.2 TELEMETRY (가칭) 와의 충돌

CMDCTX-001 §Exclusions #6 "Telemetry / metrics emission" 후속 SPEC (TBD) 도 amendment 가능성 있음:

- ResolveModelAlias 호출 카운터 / latency emission 추가
- §6.4 알고리즘 본문 후처리 추가
- 본 SPEC 과의 충돌은 ResolveModelAlias 함수 본문 동시 수정 — merge 시 conflict 가능

### 7.3 Amendment 충돌 완화 전략

**전략 A — 직렬 머지**: 본 SPEC, PERMISSIVE-ALIAS-001, TELEMETRY 를 정해진 순서로 머지. 후속 amendment 가 이전 amendment 의 본문 수정을 그대로 read-base 로 인정.

**전략 B — 통합 amendment 트랙**: 본 SPEC 머지 시 v0.2.0, PERMISSIVE-ALIAS-001 머지 시 v0.3.0, TELEMETRY 머지 시 v0.4.0 — 직렬 버전 bump.

**전략 C — 각 amendment 가 spec.md 의 서로 다른 절 수정**: 본 SPEC 은 §6.2 struct + §6.4 step 1-2 + §6.6 race; PERMISSIVE-ALIAS-001 은 §6.4 step 7 + §9 Risks R2; TELEMETRY 는 §6.4 후처리. 가능하면 절 분리.

**결정**: 전략 B + 전략 C 결합. 본 SPEC implementation 시점에 다른 amendment SPEC 과 동시 진행 여부를 사용자가 결정. 본 SPEC v0.1.0 은 "CMDCTX-001 v0.2.0 또는 그 이후 amendment 버전을 요구한다" 고만 명시 (정확한 버전 숫자는 머지 순서에 의해 결정).

### 7.4 spec.md 본문 amendment 의 governance

본 SPEC implementation 시점에 CMDCTX-001 spec.md 의 다음 절들이 동시 갱신된다:

- frontmatter `version: 0.1.1 → 0.X.0` (X 는 머지 순서 결정)
- HISTORY 항목 1줄 추가
- §1 개요: hot-reload 지원 명시
- §6.2 struct: `registry`, `aliasMap` 을 atomic.Pointer 로 전환
- §6.4 ResolveModelAlias 알고리즘: step 1-2 read-side adaptation
- §6.6 race 안전성: atomic.Pointer 사용 추가
- §Exclusions #8 "Hot-reload of registry / aliasMap" 항목 → 본 SPEC 으로 이전 (Exclusions 에서 삭제)

---

## 8. 결정 요약 (Design Decisions)

| # | 결정 | 근거 |
|---|------|------|
| D-1 | `*atomic.Pointer[T]` 포인터 indirection 으로 registry/aliasMap 전환 | planMode `*atomic.Bool` 패턴과 일관, `noCopy` 가드 회피 (CMDCTX-001 §6.6) |
| D-2 | reload 트리거: fsnotify watcher (1차) + `/reload aliases` slash (2차) | 자동성 + fallback. SIGHUP 은 후속 SPEC. |
| D-3 | fsnotify 디렉토리 단위 watch + 파일명 필터링 | atomic rename 시 file-level watch 풀림 회피 |
| D-4 | debounce 100ms | 이벤트 burst 합치기 + atomic rename safety 균형 |
| D-5 | reload 실패 시 기존 map 유지 + warn-log | fail-safe — 잘못된 입력이 daemon 을 망가뜨리지 않음 |
| D-6 | active model 새 registry 에 부재 시 cached canonical 유지 (REQ Optional) | v0.1.0 비대상, 후속 SPEC |
| D-7 | aliasconfig.Validate 는 본 SPEC 외부 — caller 가 ReloadAliases 호출 전 검증 | 책임 분리 (ALIAS-CONFIG-001 의 Validate 재사용) |
| D-8 | ReloadAliases 가 받은 map 을 deep copy | caller 가 post-swap mutation 으로 race 유발 가능성 차단 |
| D-9 | fsnotify 외부 의존성 신규 추가 허용 | v0.1.0 의 자동성 가치 > 의존성 부담 (단, build tag 로 optional 화 검토 — Open Question) |
| D-10 | CMDCTX-001 amendment 버전은 머지 순서에 의해 결정 (v0.2.0 또는 그 이후) | PERMISSIVE-ALIAS-001 / TELEMETRY 와의 직렬 amendment 트랙 |

---

## 9. Open Questions

다음은 본 SPEC plan 단계에서 사용자 결정을 권장하는 항목:

1. **fsnotify build tag 분기**: fsnotify 를 default 의존성으로 둘지, `// +build fsnotify` 빌드 태그로 optional 화할지. optional 화하면 비-fsnotify 빌드는 `/reload aliases` 슬래시 명령만 사용. 권장: default 채택 (단순성), optional 화는 후속 결정.

2. **CMDCTX-001 amendment 버전 숫자**: PERMISSIVE-ALIAS-001 (작성 중) 과 본 SPEC 의 머지 순서. 둘 다 v0.2.0 을 차지하려 함. 사용자 결정 필요. 권장: PERMISSIVE-ALIAS-001 가 P4, 본 SPEC 도 P4 — 우선순위 동등. 머지 순서는 implementation 시점 사용자가 결정.

3. **ReloadRegistry 필요성**: registry hot-reload 는 dynamic provider registration SPEC (TBD) 가 없으면 무용. v0.1.0 에서 ReloadRegistry API 만 정의 (실제 호출자 없음, future-proofing) vs aliasMap 만 hot-reload (registry 는 후속 SPEC). 권장: v0.1.0 에 둘 다 atomic.Pointer 로 전환하되, `ReloadRegistry` 호출자는 후속 SPEC 책임. 본 SPEC 은 surface 만 노출.

4. **active model invalidation 정책**: reload 후 현재 active model 이 새 registry 에 없을 때 자동으로 default model 로 fallback 할지, 다음 user input 까지 대기할지. v0.1.0 비대상 (REQ Optional, 후속 SPEC).

5. **다중 watcher 인스턴스**: 한 daemon 에 다중 ContextAdapter 가 있는 경우 (multi-session — CMDCTX-001 §Exclusions #9) watcher 도 다중일지 단일 broadcast 일지. v0.1.0 은 단일 ContextAdapter 가정 (CMDCTX-001 §Exclusions #9 와 동일 가정).

---

## 10. 의존 SPEC / 코드 anchor

### 10.1 의존 SPEC

- **SPEC-GOOSE-CMDCTX-001** (implemented, FROZEN, PR #52 c018ec5): 본 SPEC 의 amendment 대상. v0.1.1 → v0.2.0 (또는 그 이후) bump.
- **SPEC-GOOSE-ALIAS-CONFIG-001** (planned, batch A): alias config 파일 watcher 트리거. 본 SPEC 의 reload 데이터 소스.
- **SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001** (planned, P4): CMDCTX-001 amendment 동시 진행 시 §6.4 본문 충돌. governance 필요.
- **SPEC-GOOSE-ROUTER-001** (implemented, FROZEN): `*router.ProviderRegistry` API. read-only 사용.

### 10.2 코드 anchor (CMDCTX-001 implementation 기준, PR #52 c018ec5)

- `internal/command/adapter/adapter.go` — ContextAdapter struct 정의 위치
- `internal/command/adapter/alias.go` — resolveAlias 알고리즘 (REQ-CMDCTX-002 / 009 구현)
- `internal/command/adapter/adapter_test.go` — race detector 검증 패턴 (AC-CMDCTX-014)
- `internal/command/adapter/controller.go` — LoopController interface (변경 없음)

### 10.3 신규 패키지 (예상)

- `internal/command/adapter/hotreload/` — fsnotify watcher / debounce / reload 트리거 entry
- 또는 `internal/command/adapter/aliasconfig/watcher.go` — ALIAS-CONFIG-001 패키지에 watcher 추가 (research §2.1 결정 필요)

본 SPEC v0.1.0 권장: `internal/command/adapter/hotreload/` 별도 패키지. 이유: ALIAS-CONFIG-001 의 패키지 격리 invariant (AC-ALIAS-051 의 import 화이트리스트) 와 fsnotify 신규 의존성 분리.

---

## 11. 참고 (References)

- `internal/command/adapter/adapter.go` — ContextAdapter struct (v0.1.1 implementation)
- `internal/command/adapter/alias.go` — resolveAlias 알고리즘
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` — 부모 SPEC, FROZEN reference
- `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` — alias config loader, §10 Exclusions #1 hot-reload 위임
- `.moai/specs/SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001/spec.md` — amendment 충돌 분석 대상
- `https://pkg.go.dev/sync/atomic#Pointer` — atomic.Pointer Go stdlib doc
- `https://pkg.go.dev/github.com/fsnotify/fsnotify` — fsnotify cross-platform watcher
- `CLAUDE.local.md §2.5` — 코드 주석 영어 정책

---

Version: 0.1.0 (research)
Last Updated: 2026-04-27
