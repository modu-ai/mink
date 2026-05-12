package envalias

import (
	"os"
	"sync"

	"go.uber.org/zap"
)

// EnvSource는 Get 이 값을 가져온 출처를 나타낸다.
type EnvSource int

const (
	// SourceDefault는 등록된 키가 없거나 두 키 모두 미설정 시 반환된다.
	SourceDefault EnvSource = iota
	// SourceMink는 MINK_X 키에서 값을 가져왔을 때 반환된다.
	SourceMink
	// SourceGoose는 GOOSE_X 키에서 값을 가져왔을 때 반환된다 (deprecated 경로).
	SourceGoose
)

// String은 EnvSource 의 사람이 읽기 좋은 표현을 반환한다.
func (s EnvSource) String() string {
	switch s {
	case SourceMink:
		return "mink"
	case SourceGoose:
		return "goose"
	default:
		return "default"
	}
}

// Options는 Loader 생성 시 주입 가능한 옵션을 담는다.
// 테스트에서는 EnvLookup 과 Logger 를 교체해 os.Getenv 호출 없이 검증할 수 있다.
type Options struct {
	// Logger는 deprecation/conflict warning 을 출력할 zap 로거.
	// nil 이면 warning 은 무음 처리된다.
	Logger *zap.Logger
	// EnvLookup은 환경변수 값을 조회하는 함수. nil 이면 os.Getenv 로 대체된다.
	EnvLookup func(string) string
	// StrictMode가 true 이면 미등록 키 접근 시 warning 을 출력한다.
	// 기본값은 false. SPEC-MINK-ENV-CLEANUP-001 에서 true 로 전환 예정.
	StrictMode bool
}

// Loader는 GOOSE_* ↔ MINK_* alias 해석의 단일 진입점이다.
//
// @MX:ANCHOR: [AUTO] alias loader entry point — all env var reads in MINK binary route here
// @MX:REASON: fan_in >= 11 (config/env.go + 10 distributed read sites in Phase 3); central routing point
type Loader struct {
	opts       Options
	warnedOnce map[string]*sync.Once
	warnedMu   sync.Mutex
}

// New 는 주어진 Options 로 새 Loader 를 반환한다.
// opts.EnvLookup 이 nil 이면 os.Getenv 가 사용된다.
func New(opts Options) *Loader {
	if opts.EnvLookup == nil {
		opts.EnvLookup = os.Getenv
	}
	return &Loader{
		opts:       opts,
		warnedOnce: make(map[string]*sync.Once),
	}
}

// Get은 alias loader 의 단일 조회 API.
//
// shortKey 는 keys.go 에 등록된 짧은 키 이름이다 (예: "LOG_LEVEL", "HOME").
//
// 반환 규칙 (spec.md §7.2):
//   - MINK_X 설정: (MINK_X값, SourceMink, true) — warning 없음
//   - GOOSE_X 만 설정: (GOOSE_X값, SourceGoose, true) — deprecation warning 1회
//   - 두 키 모두 설정: (MINK_X값, SourceMink, true) — conflict warning 1회
//   - 두 키 모두 미설정: ("", SourceDefault, false)
//   - 미등록 키 (StrictMode=false): ("", SourceDefault, false) — 무음
//   - 미등록 키 (StrictMode=true): ("", SourceDefault, false) — warning 1회
func (l *Loader) Get(shortKey string) (value string, source EnvSource, ok bool) {
	pair, registered := keyMappings[shortKey]
	if !registered {
		if l.opts.StrictMode {
			l.logUnknownKey(shortKey)
		}
		return "", SourceDefault, false
	}

	minkVal := l.opts.EnvLookup(pair.Mink)
	gooseVal := l.opts.EnvLookup(pair.Goose)

	switch {
	case minkVal != "" && gooseVal != "":
		// 두 키 동시 설정 — MINK_X 우선, conflict warning 1회
		l.emitConflictWarning(pair.Mink, pair.Goose)
		return minkVal, SourceMink, true
	case minkVal != "":
		// MINK_X 단독 설정 — warning 없음
		return minkVal, SourceMink, true
	case gooseVal != "":
		// GOOSE_X 단독 설정 — deprecation warning 1회
		l.emitDeprecationWarning(pair.Mink, pair.Goose)
		return gooseVal, SourceGoose, true
	default:
		// 미설정
		return "", SourceDefault, false
	}
}

// emitDeprecationWarning 은 GOOSE_X 단독 사용 시 한 번만 경고를 출력한다.
func (l *Loader) emitDeprecationWarning(newFullKey, oldFullKey string) {
	once := l.onceFor(newFullKey)
	once.Do(func() {
		if l.opts.Logger == nil {
			return
		}
		l.opts.Logger.Warn("deprecated env var, please rename",
			zap.String("old", oldFullKey),
			zap.String("new", newFullKey),
			zap.String("spec", "SPEC-MINK-ENV-MIGRATE-001"),
		)
	})
}

// emitConflictWarning 은 두 키 동시 설정 시 한 번만 경고를 출력한다.
func (l *Loader) emitConflictWarning(newFullKey, oldFullKey string) {
	once := l.onceFor(newFullKey + "::conflict")
	once.Do(func() {
		if l.opts.Logger == nil {
			return
		}
		l.opts.Logger.Warn("both legacy and new env var set; using new key",
			zap.String("new", newFullKey),
			zap.String("old", oldFullKey),
			zap.String("value_source", newFullKey),
			zap.String("spec", "SPEC-MINK-ENV-MIGRATE-001"),
		)
	})
}

// onceFor는 token 에 대응하는 sync.Once 를 반환한다. 없으면 생성한다.
func (l *Loader) onceFor(token string) *sync.Once {
	l.warnedMu.Lock()
	defer l.warnedMu.Unlock()
	once, ok := l.warnedOnce[token]
	if !ok {
		once = &sync.Once{}
		l.warnedOnce[token] = once
	}
	return once
}

// logUnknownKey 는 strict mode 에서 미등록 키 접근 시 경고를 출력한다.
func (l *Loader) logUnknownKey(shortKey string) {
	if l.opts.Logger != nil {
		l.opts.Logger.Warn("envalias.Get called with unregistered key",
			zap.String("key", shortKey),
			zap.String("spec", "SPEC-MINK-ENV-MIGRATE-001"),
		)
	}
}
