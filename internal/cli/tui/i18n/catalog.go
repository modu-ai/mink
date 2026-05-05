// Package i18n provides locale-aware string catalogs for the TUI.
package i18n

// Catalog holds all user-facing strings for a given locale.
// Add new fields here when new user-facing strings are introduced.
type Catalog struct {
	Lang                  string // BCP-47 language code, e.g. "en", "ko"
	StatusbarIdle         string // format: "Session: %s | Daemon: %s | Messages: %d"
	SessionMenuHeader     string
	SessionMenuEmpty      string
	EditPrompt            string
	SlashHelpHeader       string
	PermissionPrompt      string
	PermissionAllowOnce   string
	PermissionAllowAlways string
	PermissionDenyOnce    string
	PermissionDenyAlways  string
	Saved                 string // format: "[saved: %s]"
	Loaded                string // format: "[loaded: %s, %d messages]"
}

// enCatalog is the English catalog.
// Strings must produce byte-identical output to the previous hardcoded strings.
var enCatalog = Catalog{
	Lang:                  "en",
	StatusbarIdle:         "Session: %s | Daemon: %s | Messages: %d",
	SessionMenuHeader:     "Recent sessions",
	SessionMenuEmpty:      "[no recent sessions]",
	EditPrompt:            "(edit)> ",
	SlashHelpHeader:       "Conversation commands",
	PermissionPrompt:      "Allow this tool call?",
	PermissionAllowOnce:   "Allow once",
	PermissionAllowAlways: "Allow always (this tool)",
	PermissionDenyOnce:    "Deny once",
	PermissionDenyAlways:  "Deny always (this tool)",
	Saved:                 "[saved: %s]",
	Loaded:                "[loaded: %s, %d messages]",
}

// koCatalog is the Korean catalog.
var koCatalog = Catalog{
	Lang:                  "ko",
	StatusbarIdle:         "세션: %s | 데몬: %s | 메시지: %d",
	SessionMenuHeader:     "최근 세션",
	SessionMenuEmpty:      "최근 세션 없음",
	EditPrompt:            "(편집)> ",
	SlashHelpHeader:       "대화 명령어",
	PermissionPrompt:      "이 도구 호출을 허용하시겠습니까?",
	PermissionAllowOnce:   "이번만 허용",
	PermissionAllowAlways: "항상 허용 (이 도구)",
	PermissionDenyOnce:    "이번만 거부",
	PermissionDenyAlways:  "항상 거부 (이 도구)",
	Saved:                 "[저장됨: %s]",
	Loaded:                "[불러옴: %s, %d 메시지]",
}

// Catalogs maps BCP-47 language codes to their catalogs.
// Add new language entries here to extend locale support.
//
// @MX:ANCHOR Catalogs is the primary locale registry; fan_in >= 3: loader.go, catalog_test.go, model.go
// @MX:REASON fan_in >= 3: Load() in loader.go references it, catalog_test.go checks it, Default() in catalog.go exposes it
var Catalogs = map[string]Catalog{
	"en": enCatalog,
	"ko": koCatalog,
}

// Default returns the English catalog.
// Used as the fallback when no locale is configured or the configured locale
// is not present in Catalogs.
func Default() Catalog {
	return enCatalog
}
