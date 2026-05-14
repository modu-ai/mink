package briefing

import (
	"context"
	"time"
)

// weatherCollectorAdapter adapts WeatherCollector to the Collector interface.
type weatherCollectorAdapter struct {
	collector *WeatherCollector
	location  string
}

func (a *weatherCollectorAdapter) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return a.collector.Collect(ctx, a.location)
}

// NewWeatherCollectorAdapter creates a Collector adapter for WeatherCollector.
func NewWeatherCollectorAdapter(collector *WeatherCollector, location string) Collector {
	return &weatherCollectorAdapter{
		collector: collector,
		location:  location,
	}
}

// journalCollectorAdapter adapts JournalCollector to the Collector interface.
type journalCollectorAdapter struct {
	collector *JournalCollector
}

func (a *journalCollectorAdapter) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return a.collector.Collect(ctx, userID, today)
}

// NewJournalCollectorAdapter creates a Collector adapter for JournalCollector.
func NewJournalCollectorAdapter(collector *JournalCollector) Collector {
	return &journalCollectorAdapter{collector: collector}
}

// dateCollectorAdapter adapts DateCollector to the Collector interface.
type dateCollectorAdapter struct {
	collector *DateCollector
}

func (a *dateCollectorAdapter) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return a.collector.Collect(today)
}

// NewDateCollectorAdapter creates a Collector adapter for DateCollector.
func NewDateCollectorAdapter(collector *DateCollector) Collector {
	return &dateCollectorAdapter{collector: collector}
}

// mantraCollectorAdapter adapts MantraCollector to the Collector interface.
type mantraCollectorAdapter struct {
	collector *MantraCollector
}

func (a *mantraCollectorAdapter) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return a.collector.Collect(today)
}

// NewMantraCollectorAdapter creates a Collector adapter for MantraCollector.
func NewMantraCollectorAdapter(collector *MantraCollector) Collector {
	return &mantraCollectorAdapter{collector: collector}
}
