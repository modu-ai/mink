package web

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// WeatherConfig holds runtime configuration for all weather tools.
// It is loaded from ~/.goose/config/weather.yaml at startup (or from the
// path passed to LoadWeatherConfig). Missing keys fall back to safe defaults.
type WeatherConfig struct {
	// Provider selects the weather data source.
	// "auto"           – Korean coordinates route to KMA, others to OWM (M2+)
	// "openweathermap" – always use OWM (M1 default)
	// "kma"            – always use KMA (M2; ErrMissingAPIKey if key absent)
	Provider string `yaml:"provider"`

	// OpenWeatherMap holds OWM-specific settings.
	OpenWeatherMap OWMConfig `yaml:"openweathermap"`

	// KMA holds Korean Meteorological Administration settings (M2+).
	KMA KMAConfig `yaml:"kma"`

	// AirKorea holds AirKorea API settings (M3+).
	AirKorea AirKoreaConfig `yaml:"airkorea"`

	// DefaultLocation is used when no location input is provided and IP
	// geolocation is disabled. Format: "City,CountryCode" (e.g. "Seoul,KR").
	DefaultLocation string `yaml:"default_location"`

	// CacheTTL is the duration string for the bbolt live-cache TTL.
	// Defaults to "10m". Parsed by time.ParseDuration.
	CacheTTL string `yaml:"cache_ttl"`

	// AllowIPGeolocation controls whether the tool may call ipapi.co to
	// resolve the caller's location when no explicit coordinates are given.
	// Defaults to true.
	AllowIPGeolocation bool `yaml:"allow_ip_geolocation"`

	// parsedCacheTTL is the pre-parsed CacheTTL duration (not exported).
	parsedCacheTTL time.Duration
}

// OWMConfig contains OpenWeatherMap-specific settings.
type OWMConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"` // override for testing; defaults to "https://api.openweathermap.org"
}

// KMAConfig contains Korea Meteorological Administration settings (M2+).
type KMAConfig struct {
	APIKey string `yaml:"api_key"`
}

// AirKoreaConfig contains AirKorea API settings (M3+).
type AirKoreaConfig struct {
	APIKey string `yaml:"api_key"`
}

// CacheTTLDuration returns the parsed cache TTL. Falls back to 10 minutes when
// the string is empty or unparseable.
func (c *WeatherConfig) CacheTTLDuration() time.Duration {
	if c.parsedCacheTTL > 0 {
		return c.parsedCacheTTL
	}
	return 10 * time.Minute
}

// defaultWeatherConfig returns a safe zero-configuration with production defaults.
func defaultWeatherConfig() WeatherConfig {
	return WeatherConfig{
		Provider:           "openweathermap",
		DefaultLocation:    "Seoul,KR",
		CacheTTL:           "10m",
		AllowIPGeolocation: true,
		parsedCacheTTL:     10 * time.Minute,
	}
}

// LoadWeatherConfig reads weather.yaml from path and merges it over the default
// config. If path is empty, the file does not exist, or the file is empty, the
// default config is returned without error. Parse errors in individual fields
// (e.g. invalid CacheTTL) fall back to their default values rather than failing
// fast, mirroring the TOOLS-WEB-001 LoadWebConfig pattern.
func LoadWeatherConfig(path string) (*WeatherConfig, error) {
	cfg := defaultWeatherConfig()
	if path == "" {
		return &cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return &cfg, nil // non-fatal: return defaults
	}
	if len(data) == 0 {
		return &cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// YAML parse failure is non-fatal; return defaults.
		d := defaultWeatherConfig()
		return &d, nil
	}

	// Apply defaults for missing fields.
	if cfg.Provider == "" {
		cfg.Provider = "openweathermap"
	}
	if cfg.DefaultLocation == "" {
		cfg.DefaultLocation = "Seoul,KR"
	}
	if cfg.CacheTTL == "" {
		cfg.CacheTTL = "10m"
	}

	// Parse CacheTTL; fall back to 10 minutes on error.
	if d, parseErr := time.ParseDuration(cfg.CacheTTL); parseErr == nil {
		cfg.parsedCacheTTL = d
	} else {
		cfg.parsedCacheTTL = 10 * time.Minute
	}

	return &cfg, nil
}
