package journal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIdentityClient is a stub IdentityClient for tests.
type mockIdentityClient struct {
	dates []ImportantDate
	err   error
}

func (m *mockIdentityClient) GetImportantDates(_ context.Context, _ string) ([]ImportantDate, error) {
	return m.dates, m.err
}

// TestAnniversaryDetector_DateMatch_PlusMinus1Day verifies that dates within ±1 day match.
func TestAnniversaryDetector_DateMatch_PlusMinus1Day(t *testing.T) {
	t.Parallel()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		dateMonth time.Month
		dateDay   int
		wantMatch bool
	}{
		{"exact match", time.April, 22, true},
		{"day - 1", time.April, 21, true},
		{"day + 1", time.April, 23, true},
		{"day - 2 out of window", time.April, 20, false},
		{"day + 2 out of window", time.April, 24, false},
		{"different month", time.March, 22, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			identity := &mockIdentityClient{
				dates: []ImportantDate{
					{
						Type: "wedding",
						Name: "결혼기념일",
						Date: time.Date(2020, tc.dateMonth, tc.dateDay, 0, 0, 0, 0, time.UTC),
					},
				},
			}
			d := NewAnniversaryDetector(identity)
			matches, err := d.Check(context.Background(), "u1", today)
			require.NoError(t, err)

			if tc.wantMatch {
				require.Len(t, matches, 1, "expected a match for %s", tc.name)
				assert.Equal(t, "결혼기념일", matches[0].Name)
			} else {
				assert.Empty(t, matches, "expected no match for %s", tc.name)
			}
		})
	}
}

// TestAnniversaryDetector_DateOutOfWindow verifies that dates outside ±1 day window
// do not match.
func TestAnniversaryDetector_DateOutOfWindow(t *testing.T) {
	t.Parallel()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	identity := &mockIdentityClient{
		dates: []ImportantDate{
			{
				Type: "wedding",
				Name: "결혼기념일",
				Date: time.Date(2020, 4, 25, 0, 0, 0, 0, time.UTC), // 3 days away
			},
		},
	}
	d := NewAnniversaryDetector(identity)
	matches, err := d.Check(context.Background(), "u1", today)
	require.NoError(t, err)
	assert.Empty(t, matches)
}

// TestAnniversaryDetector_NoImportantDates_Empty verifies that no matches are
// returned when the identity client returns an empty list.
func TestAnniversaryDetector_NoImportantDates_Empty(t *testing.T) {
	t.Parallel()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	identity := &mockIdentityClient{dates: nil}
	d := NewAnniversaryDetector(identity)

	matches, err := d.Check(context.Background(), "u1", today)
	require.NoError(t, err)
	assert.Empty(t, matches)
}

// TestAnniversaryDetector_EmptyUserID verifies that ErrInvalidUserID is returned.
func TestAnniversaryDetector_EmptyUserID(t *testing.T) {
	t.Parallel()

	identity := &mockIdentityClient{}
	d := NewAnniversaryDetector(identity)
	_, err := d.Check(context.Background(), "", time.Now())
	assert.ErrorIs(t, err, ErrInvalidUserID)
}

// TestAnniversaryDetector_MultipleMatches verifies that multiple dates in the window
// are all returned.
func TestAnniversaryDetector_MultipleMatches(t *testing.T) {
	t.Parallel()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	identity := &mockIdentityClient{
		dates: []ImportantDate{
			{Type: "wedding", Name: "결혼기념일", Date: time.Date(2020, 4, 22, 0, 0, 0, 0, time.UTC)},
			{Type: "birthday", Name: "생일", Date: time.Date(1990, 4, 21, 0, 0, 0, 0, time.UTC)},
			{Type: "other", Name: "기타", Date: time.Date(2010, 3, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	d := NewAnniversaryDetector(identity)
	matches, err := d.Check(context.Background(), "u1", today)
	require.NoError(t, err)
	assert.Len(t, matches, 2, "wedding and birthday should both match")
}

// TestDateInWindow_YearWrapAround verifies that December 31 / January 1 boundary works.
func TestDateInWindow_YearWrapAround(t *testing.T) {
	t.Parallel()

	today := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candidate := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)

	assert.True(t, dateInWindow(candidate, today, 1), "Dec 31 and Jan 1 are within ±1 day")
}
