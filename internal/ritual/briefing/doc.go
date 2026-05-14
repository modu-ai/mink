// Package briefing provides morning briefing functionality for the MINK ritual companion.
//
// The morning briefing collects and orchestrates multiple data sources to provide
// a comprehensive daily briefing including:
//
//   - Weather information (current conditions and forecast)
//   - Journal recall (anniversaries, mood trends)
//   - Date calendar (solar terms, Korean holidays)
//   - Daily mantras
//
// The orchestrator runs collectors in parallel with per-module timeouts and
// produces a structured payload that can be rendered to various formats.
package briefing
