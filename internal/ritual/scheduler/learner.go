// Package scheduler — PatternLearner for SPEC-GOOSE-SCHEDULER-001 P4a T-020.
package scheduler

import (
	"fmt"
	"math"
	"sync"
)

// driftCapMinutes is the maximum displacement (in minutes) PatternLearner is
// allowed to propose in a single adjustment cycle. REQ-SCHED-016 caps abrupt
// jumps at ±2 hours.
const driftCapMinutes = 120

// peakProximityMinutes is the window inside which two daily peaks are
// considered the "same" peak for the SupportingDays counter.
const peakProximityMinutes = 15

// PatternLearner converts raw ActivityPatterns into RitualTimeProposals.
// It maintains a small per-kind history of recent peak hours so consecutive
// observations can be counted toward the 3-day commit threshold
// (REQ-SCHED-016).
//
// @MX:ANCHOR: [AUTO] Central learner — fan_in >= 3 (Scheduler.runDailyLearner, Predict, Observe callers)
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-006, REQ-SCHED-016
type PatternLearner struct {
	cfg PatternLearnerConfig

	mu      sync.Mutex
	history map[RitualKind][]int // ring buffer of recent observed peak hours
}

// NewPatternLearner constructs a PatternLearner with the given config.
// Defaults are applied via cfg.effective().
func NewPatternLearner(cfg PatternLearnerConfig) *PatternLearner {
	return &PatternLearner{
		cfg:     cfg.effective(),
		history: make(map[RitualKind][]int),
	}
}

// Predict returns the ritual clock string the learner believes the user
// prefers, based on the ActivityPattern aggregate.
//
// REQ-SCHED-006: confidence = min(1.0, DaysObserved / RollingWindow). When
// DaysObserved < RollingWindow the configured fallback default for the kind
// is returned with confidence proportional to days observed (REQ-SCHED-012).
//
// Edge case: when ByHour is all zero, fallback default is returned with
// confidence 0.
func (l *PatternLearner) Predict(kind RitualKind, pat ActivityPattern) (clock string, confidence float64, err error) {
	if pat.DaysObserved < l.cfg.RollingWindowDays {
		return l.fallbackDefault(kind), float64(pat.DaysObserved) / float64(l.cfg.RollingWindowDays), nil
	}

	peak, found := dominantHour(pat.ByHour)
	if !found {
		// All zero counts — return fallback with confidence 0.
		return l.fallbackDefault(kind), 0.0, nil
	}

	confidence = math.Min(1.0, float64(pat.DaysObserved)/float64(l.cfg.RollingWindowDays))
	return fmt.Sprintf("%02d:00", peak), confidence, nil
}

// Observe ingests a new daily ActivityPattern, updates the learner's history,
// and returns a RitualTimeProposal when the drift exceeds the configured
// threshold. Returns nil when no proposal is warranted.
//
// REQ-SCHED-006: drift gate at DriftThresholdMinutes (default 30 min).
// REQ-SCHED-016: cap proposed displacement at ±driftCapMinutes (120 min).
func (l *PatternLearner) Observe(kind RitualKind, currentClock string, pat ActivityPattern) (*RitualTimeProposal, error) {
	peak, found := dominantHour(pat.ByHour)
	if !found {
		return nil, nil
	}

	currentMinutes, err := clockToMinutes(currentClock)
	if err != nil {
		return nil, err
	}
	peakMinutes := peak * 60

	driftRaw := peakMinutes - currentMinutes
	driftAbs := absInt(driftRaw)

	if driftAbs <= l.cfg.DriftThresholdMinutes {
		// Within the no-op band — no proposal, but still record the peak so
		// future SupportingDays bookkeeping is accurate.
		l.recordPeak(kind, peak)
		return nil, nil
	}

	// Cap at ±driftCapMinutes preserving direction.
	cappedDrift := driftRaw
	if cappedDrift > driftCapMinutes {
		cappedDrift = driftCapMinutes
	} else if cappedDrift < -driftCapMinutes {
		cappedDrift = -driftCapMinutes
	}
	newClock := minutesToClock(currentMinutes + cappedDrift)

	supporting := l.recordPeak(kind, peak)

	confidence := math.Min(1.0, float64(pat.DaysObserved)/float64(l.cfg.RollingWindowDays))

	return &RitualTimeProposal{
		Kind:            kind,
		OldLocalClock:   currentClock,
		NewLocalClock:   newClock,
		DriftMinutes:    driftAbs,
		SupportingDays:  supporting,
		Confidence:      confidence,
		ConfirmRequired: true,
	}, nil
}

// recordPeak appends peakHour to the kind's ring buffer (capped at
// RollingWindowDays) and returns the count of recent consecutive peaks within
// peakProximityMinutes of the new observation — i.e. the SupportingDays value.
func (l *PatternLearner) recordPeak(kind RitualKind, peakHour int) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	hist := l.history[kind]
	hist = append(hist, peakHour)
	if len(hist) > l.cfg.RollingWindowDays {
		hist = hist[len(hist)-l.cfg.RollingWindowDays:]
	}
	l.history[kind] = hist

	// Count contiguous trailing entries within peakProximity of the latest.
	supporting := 0
	for i := len(hist) - 1; i >= 0; i-- {
		if absInt((hist[i]-peakHour)*60) <= peakProximityMinutes {
			supporting++
		} else {
			break
		}
	}
	return supporting
}

// fallbackDefault returns the configured default clock for the given kind.
// All defaults are validated as non-empty in PatternLearnerConfig.effective().
func (l *PatternLearner) fallbackDefault(kind RitualKind) string {
	switch kind {
	case KindMorning:
		return l.cfg.DefaultMorning
	case KindBreakfast:
		return l.cfg.DefaultBreakfast
	case KindLunch:
		return l.cfg.DefaultLunch
	case KindDinner:
		return l.cfg.DefaultDinner
	case KindEvening:
		return l.cfg.DefaultEvening
	}
	return ""
}

// dominantHour returns the index of the highest-count bucket in the 24-hour
// histogram. The second return value is false when all counts are zero.
func dominantHour(byHour [24]int) (int, bool) {
	bestIdx := -1
	bestCount := 0
	for h, c := range byHour {
		if c > bestCount {
			bestCount = c
			bestIdx = h
		}
	}
	return bestIdx, bestIdx >= 0
}

func clockToMinutes(clock string) (int, error) {
	h, m, err := parseClock(clock)
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

func minutesToClock(total int) string {
	// Wrap into [0, 24*60) just in case the cap pushes the value out of range.
	for total < 0 {
		total += 24 * 60
	}
	total %= 24 * 60
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
