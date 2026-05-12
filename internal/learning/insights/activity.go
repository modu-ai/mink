package insights

import (
	"sort"
	"time"

	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// dayLabels maps the Mon=0 index to weekday abbreviations (spec §6.4).
var dayLabels = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// aggregateActivity computes the ActivityPattern from a set of trajectories.
// ByDay is indexed Mon=0 to Sun=6 (not Go's time.Weekday Sunday=0).
// ByHour is indexed 0-23.
func aggregateActivity(trajectories []*trajectory.Trajectory) ActivityPattern {
	byDay := make([]DayBucket, 7)
	byHour := make([]HourBucket, 24)
	daySet := make(map[string]struct{})

	for i, label := range dayLabels {
		byDay[i].Day = label
	}
	for i := range byHour {
		byHour[i].Hour = i
	}

	for _, t := range trajectories {
		ts := t.Timestamp.UTC()
		// Remap from Go's Sunday=0 to Mon=0.
		// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6
		// Our mapping: Monday=0, Tuesday=1, ..., Sunday=6
		// Formula: (int(weekday) + 6) % 7
		dayIdx := (int(ts.Weekday()) + 6) % 7
		byDay[dayIdx].Count++
		byHour[ts.Hour()].Count++
		daySet[ts.Format("2006-01-02")] = struct{}{}
	}

	activeDays := len(daySet)
	maxStreak := computeMaxStreak(daySet)

	busyDay := maxDayBucket(byDay)
	busyHour := maxHourBucket(byHour)

	return ActivityPattern{
		ByDay:       byDay,
		ByHour:      byHour,
		BusiestDay:  busyDay,
		BusiestHour: busyHour,
		ActiveDays:  activeDays,
		MaxStreak:   maxStreak,
	}
}

// computeMaxStreak finds the longest consecutive-day run in daySet.
// Days with zero sessions break the streak.
func computeMaxStreak(daySet map[string]struct{}) int {
	if len(daySet) == 0 {
		return 0
	}
	days := make([]time.Time, 0, len(daySet))
	for s := range daySet {
		d, _ := time.Parse("2006-01-02", s)
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Before(days[j])
	})

	streak := 1
	maxS := 1
	for i := 1; i < len(days); i++ {
		diff := days[i].Sub(days[i-1])
		if diff == 24*time.Hour {
			streak++
			if streak > maxS {
				maxS = streak
			}
		} else {
			streak = 1
		}
	}
	return maxS
}

// maxDayBucket returns the DayBucket with the highest Count.
// Ties broken by the lower index (earlier weekday).
func maxDayBucket(byDay []DayBucket) DayBucket {
	if len(byDay) == 0 {
		return DayBucket{}
	}
	best := byDay[0]
	for _, b := range byDay[1:] {
		if b.Count > best.Count {
			best = b
		}
	}
	return best
}

// maxHourBucket returns the HourBucket with the highest Count.
// Ties broken by the lower hour (earlier in the day).
func maxHourBucket(byHour []HourBucket) HourBucket {
	if len(byHour) == 0 {
		return HourBucket{}
	}
	best := byHour[0]
	for _, b := range byHour[1:] {
		if b.Count > best.Count {
			best = b
		}
	}
	return best
}
