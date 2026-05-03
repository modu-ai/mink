package insights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeriod_BoundsHonored verifies that Extract only scans files within the period.
// AC-INSIGHTS-008
func TestPeriod_BoundsHonored(t *testing.T) {
	t.Parallel()

	// Create test trajectory files at 4 dates.
	dir := t.TempDir()
	dates := []string{"2026-04-10", "2026-04-15", "2026-04-20", "2026-04-25"}
	for _, d := range dates {
		writeTestJSONLFile(t, dir, "success", d+".jsonl", 2) // 2 sessions per file
	}

	cfg := InsightsConfig{
		TelemetryEnabled: true,
		GooseHome:        dir,
	}
	engine := New(cfg, nil)

	// Only 04-15 and 04-20 are within [04-14, 04-22].
	from := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	period := Between(from, to)

	report, err := engine.Extract(t.Context(), period)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Should have 4 sessions (2 from 04-15 + 2 from 04-20).
	assert.Equal(t, 4, report.Overview.TotalSessions,
		"only sessions in [04-15, 04-20] should be counted")
	assert.Equal(t, from, report.Period.From)
	assert.Equal(t, to, report.Period.To)
}

// TestPeriod_Invalid_ErrInvalidPeriod verifies that From > To returns ErrInvalidPeriod.
// AC-INSIGHTS-009
func TestPeriod_Invalid_ErrInvalidPeriod(t *testing.T) {
	t.Parallel()

	cfg := InsightsConfig{TelemetryEnabled: true}
	engine := New(cfg, nil)

	from := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	period := Between(from, to)

	report, err := engine.Extract(t.Context(), period)
	assert.Nil(t, report)
	assert.ErrorIs(t, err, ErrInvalidPeriod)
}

// TestPeriod_Last returns a valid period covering the last N days.
func TestPeriod_Last(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	p := Last(7)
	after := time.Now().UTC()

	assert.True(t, p.From.Before(p.To))
	assert.WithinDuration(t, before.AddDate(0, 0, -7), p.From, 2*time.Second)
	assert.WithinDuration(t, after, p.To, 2*time.Second)
}

// TestPeriod_AllTime contains zero From and far-future To.
func TestPeriod_AllTime(t *testing.T) {
	t.Parallel()

	p := AllTime()
	assert.True(t, p.From.IsZero())
	assert.Equal(t, time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), p.To)
}

// TestPeriod_Validate_EqualFromTo is valid (same From and To).
func TestPeriod_Validate_EqualFromTo(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	p := Between(now, now)
	assert.NoError(t, p.Validate())
}
