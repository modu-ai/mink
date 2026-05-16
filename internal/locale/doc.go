// Package locale implements OS-level locale detection and cultural context
// derivation for MINK's localization foundation layer.
//
// # Overview
//
// SPEC-MINK-LOCALE-001 defines this package as the foundation layer that all
// subsequent localization SPECs (I18N-001, REGION-SKILLS-001, ONBOARDING-001)
// depend upon. It detects who the user is (country/language/timezone/currency/
// cultural context) without implementing any UI translation, skill bundling, or
// onboarding UX.
//
// # Package Structure
//
//   - context.go    — LocaleContext, CulturalContext, related types
//   - detect.go     — Detect() orchestration + env parsing helpers
//   - cultural.go   — Country→CulturalContext static mapping table + currency/timezone maps
//   - prompts.go    — BuildSystemPromptAddendum() LLM prompt builder
//   - persistence.go — YAML round-trip helpers (Load/Save)
//   - os_linux.go   — Linux-specific OS locale detection
//   - os_darwin.go  — macOS-specific OS locale detection
//   - os_windows.go — Windows-specific OS locale detection
//
// # Key Design Decisions
//
// Injectable indirections (getEnv, statFile, execCommand) allow tests
// to substitute fakes without real OS dependencies. The Detect() function never
// mutates process-level environment variables (REQ-LC-014).
//
// Number/date/time format and collation are intentionally OUT OF SCOPE — those
// dimensions belong to I18N-001 which consumes LocaleContext.PrimaryLanguage
// (BCP 47) and resolves formatting via CLDR tables.
//
// Currency mapping uses a CLDR-inspired manual table (~30 priority countries)
// with USD as fallback. Multi-timezone countries (US/RU/BR/CA/AU) expose
// TimezoneAlternatives for ONBOARDING-001 disambiguation.
//
// # @MX Contract
//
// @MX:ANCHOR: [AUTO] Detect() is the single source of truth for OS locale resolution across MINK.
// @MX:REASON: ONBOARDING-001, REGION-SKILLS-001, and SCHEDULER-001 all consume the returned
// LocaleContext; any signature change breaks the entire localization dependency chain (REQ-LC-005).
//
// @MX:ANCHOR: [AUTO] LocaleContext is the canonical cross-package data type for user locale state.
// @MX:REASON: Serialized to ~/.mink/config.yaml locale: section; field renames break existing
// persisted configurations and require a CONFIG-001 migration.
//
// @MX:NOTE: [AUTO] IP geolocation (MaxMind + ipapi.co) is a Phase 2 follow-up. Phase 1 provides
// the stub interface and injectable indirections; real HTTP/DB calls are wired in the next PR.
package locale
