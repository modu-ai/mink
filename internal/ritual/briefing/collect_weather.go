package briefing

import (
	"context"
	"fmt"
)

// WeatherFetcher defines the interface for fetching weather data.
// This abstraction allows tests to mock the weather data source.
type WeatherFetcher interface {
	Current(ctx context.Context, location string) (*MockWeatherCurrent, error)
	Forecast(ctx context.Context, location string) (*MockWeatherForecast, error)
	AirQuality(ctx context.Context, location string) (*MockAirQuality, error)
}

// WeatherCollector collects weather information for the briefing.
type WeatherCollector struct {
	fetcher WeatherFetcher
}

// NewWeatherCollector creates a new WeatherCollector.
func NewWeatherCollector(fetcher WeatherFetcher) *WeatherCollector {
	return &WeatherCollector{fetcher: fetcher}
}

// Collect fetches weather data and returns a WeatherModule.
// If critical fetches (current/forecast) fail, it sets Offline=true.
// Air quality is optional - failure doesn't trigger offline mode.
// Status is "ok" if critical data available, "offline" on error, "timeout" on context cancel.
func (c *WeatherCollector) Collect(ctx context.Context, location string) (*WeatherModule, string) {
	module := &WeatherModule{
		Current:    nil,
		Forecast:   nil,
		AirQuality: nil,
		Offline:    false,
	}

	// Check if context is already cancelled
	if ctx.Err() != nil {
		module.Offline = true
		return module, "timeout"
	}

	// Fetch current weather (critical)
	current, err := c.fetcher.Current(ctx, location)
	if err != nil {
		module.Offline = true
		return module, "offline"
	}

	// Fetch forecast (critical)
	forecast, err := c.fetcher.Forecast(ctx, location)
	if err != nil {
		module.Offline = true
		return module, "offline"
	}

	// Fetch air quality (optional - don't fail on error)
	airQuality, _ := c.fetcher.AirQuality(ctx, location)
	// If air quality fails, we just don't include it

	// Map mock types to briefing types
	module.Current = &WeatherCurrent{
		Temp:      current.Temp,
		FeelsLike: current.FeelsLike,
		Humidity:  current.Humidity,
		Condition: current.Condition,
		Location:  current.Location,
	}

	if forecast != nil {
		days := make([]WeatherForecastDay, len(forecast.Days))
		for i, d := range forecast.Days {
			days[i] = WeatherForecastDay(d)
		}
		module.Forecast = &WeatherForecast{Days: days}
	}

	if airQuality != nil {
		module.AirQuality = &AirQuality{
			PM25:  airQuality.PM25,
			PM10:  airQuality.PM10,
			AQI:   airQuality.AQI,
			Level: airQuality.Level,
		}
	}

	return module, "ok"
}

// Mock weather data types for testing.
// These are defined in the production package to avoid code duplication
// between test files and production code.

// MockWeatherCurrent represents current weather data (test/mock structure).
type MockWeatherCurrent struct {
	Temp      float64
	FeelsLike float64
	Humidity  float64
	Condition string
	Location  string
}

// MockWeatherForecast represents weather forecast (test/mock structure).
type MockWeatherForecast struct {
	Days []MockForecastDay
}

// MockForecastDay represents a single forecast day (test/mock structure).
type MockForecastDay struct {
	Date      string
	High      float64
	Low       float64
	Condition string
}

// MockAirQuality represents air quality data (test/mock structure).
type MockAirQuality struct {
	PM25  float64
	PM10  float64
	AQI   int
	Level string
}

// WeatherFetcherImpl is the real implementation using web package types.
// This is a placeholder for future integration with actual weather providers.
type WeatherFetcherImpl struct {
	// In future iterations, this would hold a real weather client
}

// Current fetches current weather (placeholder for real implementation).
func (w *WeatherFetcherImpl) Current(ctx context.Context, location string) (*MockWeatherCurrent, error) {
	// TODO: Implement real weather fetching in M2/M3
	return nil, fmt.Errorf("not implemented: use mock for testing")
}

// Forecast fetches weather forecast (placeholder for real implementation).
func (w *WeatherFetcherImpl) Forecast(ctx context.Context, location string) (*MockWeatherForecast, error) {
	// TODO: Implement real weather fetching in M2/M3
	return nil, fmt.Errorf("not implemented: use mock for testing")
}

// AirQuality fetches air quality data (placeholder for real implementation).
func (w *WeatherFetcherImpl) AirQuality(ctx context.Context, location string) (*MockAirQuality, error) {
	// TODO: Implement real air quality fetching in M3
	return nil, fmt.Errorf("not implemented: use mock for testing")
}
