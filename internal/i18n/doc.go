// Package i18n provides a lightweight internationalization layer for MINK backend code.
//
// Architecture:
//
//	Bundle   — loads YAML message files from an embedded catalog directory.
//	Translator — per-language runtime that resolves keys, applies Go template
//	             parameter substitution, and handles CLDR plural selection via
//	             go-i18n/v2.
//	Default()  — process-wide singleton that auto-detects the active language
//	             from locale.Detect and exposes a ready-made Translator.
//
// Usage:
//
//	tr := i18n.Default()
//	msg := tr.Translate("install.welcome", nil)
//	// → "Welcome to MINK" (en) or "MINK에 오신 것을 환영합니다" (ko)
//
//	msg = tr.Translate("install.step_count", map[string]any{"Count": 3})
//	// → "3 steps remaining" (en) or "3단계 남음" (ko)
//
// Thread safety: Bundle is safe after construction; Translator instances are
// read-only and may be shared across goroutines.
//
// SPEC: SPEC-MINK-I18N-001 Phase 1 (Foundation Layer)
//
// @MX:ANCHOR: [AUTO] Bundle is the package entry point — caller count 3+ (Default, LoadDirectory, tests).
// @MX:REASON: Signature changes propagate to every consumer of the Default() singleton and all test harnesses.
//
// @MX:ANCHOR: [AUTO] Translator is the per-language translation interface — consumed by Default, CLI init, CLI tui.
// @MX:REASON: Interface change breaks the Default() wrapper and all mock implementations in test doubles.
package i18n
