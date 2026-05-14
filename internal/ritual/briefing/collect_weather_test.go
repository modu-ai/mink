package briefing

import (
	"context"
	"errors"
	"testing"
)

// mockWeatherFetcher is a test double for WeatherFetcher.
type mockWeatherFetcher struct {
	current    *MockWeatherCurrent
	forecast   *MockWeatherForecast
	airQuality *MockAirQuality
	err        error
}

func (m *mockWeatherFetcher) Current(ctx context.Context, location string) (*MockWeatherCurrent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.current, nil
}

func (m *mockWeatherFetcher) Forecast(ctx context.Context, location string) (*MockWeatherForecast, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.forecast, nil
}

func (m *mockWeatherFetcher) AirQuality(ctx context.Context, location string) (*MockAirQuality, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.airQuality, nil
}

func TestWeatherCollector_Collect(t *testing.T) {
	t.Run("happy path - all data available", func(t *testing.T) {
		ctx := context.Background()
		location := "Seoul"

		fetcher := &mockWeatherFetcher{
			current: &MockWeatherCurrent{
				Temp:      18.5,
				FeelsLike: 17.0,
				Humidity:  65.0,
				Condition: "clear",
				Location:  "Seoul",
			},
			forecast: &MockWeatherForecast{
				Days: []MockForecastDay{
					{Date: "2026-05-14", High: 22.0, Low: 15.0, Condition: "sunny"},
					{Date: "2026-05-15", High: 23.0, Low: 16.0, Condition: "cloudy"},
				},
			},
			airQuality: &MockAirQuality{
				PM25:  25.5,
				PM10:  45.0,
				AQI:   55,
				Level: "moderate",
			},
		}

		collector := NewWeatherCollector(fetcher)
		module, status := collector.Collect(ctx, location)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		if module.Offline {
			t.Error("expected Offline=false, got true")
		}

		if module.Current == nil {
			t.Fatal("expected Current to be populated")
		}
		if module.Current.Temp != 18.5 {
			t.Errorf("expected Temp=18.5, got %f", module.Current.Temp)
		}
		if module.Current.Condition != "clear" {
			t.Errorf("expected Condition='clear', got '%s'", module.Current.Condition)
		}

		if module.Forecast == nil {
			t.Fatal("expected Forecast to be populated")
		}
		if len(module.Forecast.Days) != 2 {
			t.Errorf("expected 2 forecast days, got %d", len(module.Forecast.Days))
		}

		if module.AirQuality == nil {
			t.Fatal("expected AirQuality to be populated")
		}
		if module.AirQuality.AQI != 55 {
			t.Errorf("expected AQI=55, got %d", module.AirQuality.AQI)
		}
	})

	t.Run("offline - fetch error", func(t *testing.T) {
		ctx := context.Background()
		location := "Seoul"

		fetcher := &mockWeatherFetcher{
			err: errors.New("network timeout"),
		}

		collector := NewWeatherCollector(fetcher)
		module, status := collector.Collect(ctx, location)

		if status != "offline" {
			t.Errorf("expected status 'offline', got '%s'", status)
		}

		if !module.Offline {
			t.Error("expected Offline=true on error")
		}

		if module.Current != nil || module.Forecast != nil || module.AirQuality != nil {
			t.Error("expected all data to be nil on error")
		}
	})

	t.Run("offline - fetch error", func(t *testing.T) {
		ctx := context.Background()
		location := "Seoul"

		fetcher := &mockWeatherFetcher{
			err: errors.New("network timeout"),
		}

		collector := NewWeatherCollector(fetcher)
		module, status := collector.Collect(ctx, location)

		if status != "offline" {
			t.Errorf("expected status 'offline', got '%s'", status)
		}

		if !module.Offline {
			t.Error("expected Offline=true on error")
		}

		if module.Current != nil || module.Forecast != nil || module.AirQuality != nil {
			t.Error("expected all data to be nil on error")
		}
	})

	t.Run("partial failure - air quality missing", func(t *testing.T) {
		ctx := context.Background()
		location := "Seoul"

		// Air quality is optional, so missing it should still return "ok"
		fetcher := &partialMockFetcher{
			hasCurrent:    true,
			hasForecast:   true,
			hasAirQuality: false,
		}

		collector := NewWeatherCollector(fetcher)
		module, status := collector.Collect(ctx, location)

		// Critical data is available, so status should be "ok"
		if status != "ok" {
			t.Errorf("expected status 'ok' when only air quality missing, got '%s'", status)
		}
		if module.Current == nil {
			t.Error("expected Current to be populated")
		}
		if module.Forecast == nil {
			t.Error("expected Forecast to be populated")
		}
		if module.AirQuality != nil {
			t.Error("expected AirQuality to be nil when fetch fails")
		}
	})

	t.Run("critical failure - current missing", func(t *testing.T) {
		ctx := context.Background()
		location := "Seoul"

		fetcher := &partialMockFetcher{
			hasCurrent:    false,
			hasForecast:   true,
			hasAirQuality: true,
		}

		collector := NewWeatherCollector(fetcher)
		module, status := collector.Collect(ctx, location)

		// Missing critical data should return "offline"
		if status != "offline" {
			t.Errorf("expected status 'offline' when current missing, got '%s'", status)
		}
		if !module.Offline {
			t.Error("expected Offline=true when critical data missing")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		fetcher := &mockWeatherFetcher{
			current: &MockWeatherCurrent{},
		}

		collector := NewWeatherCollector(fetcher)
		_, status := collector.Collect(ctx, "Seoul")

		if status != "timeout" {
			t.Errorf("expected status 'timeout' on cancelled context, got '%s'", status)
		}
	})
}

// partialMockFetcher simulates partial data availability
type partialMockFetcher struct {
	hasCurrent    bool
	hasForecast   bool
	hasAirQuality bool
}

func (p *partialMockFetcher) Current(ctx context.Context, location string) (*MockWeatherCurrent, error) {
	if !p.hasCurrent {
		return nil, errors.New("current unavailable")
	}
	return &MockWeatherCurrent{Temp: 20.0, Condition: "clear", Location: location}, nil
}

func (p *partialMockFetcher) Forecast(ctx context.Context, location string) (*MockWeatherForecast, error) {
	if !p.hasForecast {
		return nil, errors.New("forecast unavailable")
	}
	return &MockWeatherForecast{
		Days: []MockForecastDay{
			{Date: "2026-05-14", High: 22.0, Low: 15.0, Condition: "sunny"},
		},
	}, nil
}

func (p *partialMockFetcher) AirQuality(ctx context.Context, location string) (*MockAirQuality, error) {
	if !p.hasAirQuality {
		return nil, errors.New("air quality unavailable")
	}
	return &MockAirQuality{PM25: 30.0, PM10: 50.0, AQI: 60, Level: "moderate"}, nil
}
