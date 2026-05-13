package envalias

// keyPair holds the canonical MINK_* key name and the legacy GOOSE_* alias.
type keyPair struct {
	Mink  string
	Goose string
}

// keyMappings maps the short newKey (e.g. "LOG_LEVEL") to the full MINK_/GOOSE_ pair.
// All 21 single-key entries from spec.md §7.3 inventory are registered here.
//
// The 22nd entry (GOOSE_AUTH_* prefix glob) is NOT a single-key alias; it is handled
// separately by the env-scrub deny-list in internal/hook/isolation_*.go.
// Use [MinkPrefixes] to obtain the prefix list for deny-list extension.
//
// @MX:ANCHOR: [AUTO] primary 22-key alias mapping table — all env var read sites depend on this
// @MX:REASON: fan_in >= 5 (Phase 2-6 adoption sites); modification changes alias semantics globally
var keyMappings = map[string]keyPair{
	"HOME":                    {Mink: "MINK_HOME", Goose: "GOOSE_HOME"},
	"LOG_LEVEL":               {Mink: "MINK_LOG_LEVEL", Goose: "GOOSE_LOG_LEVEL"},
	"HEALTH_PORT":             {Mink: "MINK_HEALTH_PORT", Goose: "GOOSE_HEALTH_PORT"},
	"GRPC_PORT":               {Mink: "MINK_GRPC_PORT", Goose: "GOOSE_GRPC_PORT"},
	"LOCALE":                  {Mink: "MINK_LOCALE", Goose: "GOOSE_LOCALE"},
	"LEARNING_ENABLED":        {Mink: "MINK_LEARNING_ENABLED", Goose: "GOOSE_LEARNING_ENABLED"},
	"CONFIG_STRICT":           {Mink: "MINK_CONFIG_STRICT", Goose: "GOOSE_CONFIG_STRICT"},
	"GRPC_REFLECTION":         {Mink: "MINK_GRPC_REFLECTION", Goose: "GOOSE_GRPC_REFLECTION"},
	"GRPC_MAX_RECV_MSG_BYTES": {Mink: "MINK_GRPC_MAX_RECV_MSG_BYTES", Goose: "GOOSE_GRPC_MAX_RECV_MSG_BYTES"},
	"SHUTDOWN_TOKEN":          {Mink: "MINK_SHUTDOWN_TOKEN", Goose: "GOOSE_SHUTDOWN_TOKEN"},
	"HOOK_TRACE":              {Mink: "MINK_HOOK_TRACE", Goose: "GOOSE_HOOK_TRACE"},
	"HOOK_NON_INTERACTIVE":    {Mink: "MINK_HOOK_NON_INTERACTIVE", Goose: "GOOSE_HOOK_NON_INTERACTIVE"},
	"ALIAS_STRICT":            {Mink: "MINK_ALIAS_STRICT", Goose: "GOOSE_ALIAS_STRICT"},
	"QWEN_REGION":             {Mink: "MINK_QWEN_REGION", Goose: "GOOSE_QWEN_REGION"},
	"KIMI_REGION":             {Mink: "MINK_KIMI_REGION", Goose: "GOOSE_KIMI_REGION"},
	"TELEGRAM_BOT_TOKEN":      {Mink: "MINK_TELEGRAM_BOT_TOKEN", Goose: "GOOSE_TELEGRAM_BOT_TOKEN"},
	"AUTH_TOKEN":              {Mink: "MINK_AUTH_TOKEN", Goose: "GOOSE_AUTH_TOKEN"},
	"AUTH_REFRESH":            {Mink: "MINK_AUTH_REFRESH", Goose: "GOOSE_AUTH_REFRESH"},
	"HISTORY_SNIP":            {Mink: "MINK_HISTORY_SNIP", Goose: "GOOSE_HISTORY_SNIP"},
	"METRICS_ENABLED":         {Mink: "MINK_METRICS_ENABLED", Goose: "GOOSE_METRICS_ENABLED"},
	"GRPC_BIND":               {Mink: "MINK_GRPC_BIND", Goose: "GOOSE_GRPC_BIND"},
}

// minkPrefixes는 MINK_* 접두어 목록을 반환한다.
// internal/hook/isolation_*.go 의 env scrub deny-list 확장 용도 (Phase 4).
var minkPrefixes = []string{
	"MINK_AUTH_",
}

// MinkPrefixes returns the list of MINK_* prefix strings that must be added to
// the env-scrub deny-list in internal/hook/isolation_*.go.
// This is the Phase 4 preparation hook described in plan.md §5.
func MinkPrefixes() []string {
	result := make([]string, len(minkPrefixes))
	copy(result, minkPrefixes)
	return result
}
