package web_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOfflineSaveLoad_RoundTrip verifies that a WeatherReport saved by
// SaveLatest can be recovered intact by LoadLatest.
func TestOfflineSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := web.NewDiskOfflineStore(dir)

	report := &web.WeatherReport{
		Location:       web.Location{Lat: 37.57, Lon: 126.98, DisplayName: "Seoul", Country: "KR"},
		Timestamp:      time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
		TemperatureC:   22.5,
		Condition:      "clear",
		SourceProvider: "openweathermap",
	}

	err := store.SaveLatest("openweathermap", 37.57, 126.98, report)
	require.NoError(t, err)

	loaded, err := store.LoadLatest("openweathermap", 37.57, 126.98)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, report.TemperatureC, loaded.TemperatureC)
	assert.Equal(t, report.Condition, loaded.Condition)
	assert.Equal(t, report.SourceProvider, loaded.SourceProvider)
	assert.Equal(t, report.Location.Lat, loaded.Location.Lat)
	assert.Equal(t, report.Location.Country, loaded.Location.Country)
	assert.True(t, report.Timestamp.Equal(loaded.Timestamp), "timestamps must match")
}

// TestOfflineLoad_FileMissing verifies that LoadLatest returns
// ErrNoFallbackAvailable when no file exists for the given slot.
func TestOfflineLoad_FileMissing(t *testing.T) {
	dir := t.TempDir()
	store := web.NewDiskOfflineStore(dir)

	_, err := store.LoadLatest("openweathermap", 37.57, 126.98)
	assert.ErrorIs(t, err, web.ErrNoFallbackAvailable,
		"missing file must return ErrNoFallbackAvailable")
}

// TestOfflineLoad_CorruptJSON_Evict verifies that a corrupt JSON file is
// evicted and ErrNoFallbackAvailable is returned.
func TestOfflineLoad_CorruptJSON_Evict(t *testing.T) {
	dir := t.TempDir()
	store := web.NewDiskOfflineStore(dir)

	// Write a syntactically invalid JSON file at the expected path.
	path := filepath.Join(dir, "latest-openweathermap-37.57-126.98.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0600))

	_, err := store.LoadLatest("openweathermap", 37.57, 126.98)
	assert.ErrorIs(t, err, web.ErrNoFallbackAvailable,
		"corrupt JSON must return ErrNoFallbackAvailable")

	// The file must have been evicted.
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "corrupt file must be deleted after eviction")
}

// TestOfflineLoad_AtomicWrite_NoPartial verifies that the file is not visible
// in a partial state during a SaveLatest call. We do this by saving, then
// immediately loading — the loaded file must be a complete, parseable report.
func TestOfflineLoad_AtomicWrite_NoPartial(t *testing.T) {
	dir := t.TempDir()
	store := web.NewDiskOfflineStore(dir)

	report := &web.WeatherReport{
		Location:       web.Location{Lat: 35.68, Lon: 139.69, Country: "JP"},
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		TemperatureC:   18.3,
		Condition:      "rain",
		SourceProvider: "openweathermap",
	}

	require.NoError(t, store.SaveLatest("openweathermap", 35.68, 139.69, report))

	// The file must be present and contain valid JSON.
	path := filepath.Join(dir, "latest-openweathermap-35.68-139.69.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded web.WeatherReport
	require.NoError(t, json.Unmarshal(data, &loaded), "saved file must be valid JSON")
	assert.Equal(t, report.TemperatureC, loaded.TemperatureC)

	// File permissions must be 0600.
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "file permissions must be 0600")
}

// TestOfflineSave_OverwritesPrevious verifies that a second SaveLatest call
// replaces the first file (idempotent overwrite via atomic rename).
func TestOfflineSave_OverwritesPrevious(t *testing.T) {
	dir := t.TempDir()
	store := web.NewDiskOfflineStore(dir)

	r1 := &web.WeatherReport{
		Timestamp:    time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
		TemperatureC: 10.0, SourceProvider: "openweathermap",
	}
	r2 := &web.WeatherReport{
		Timestamp:    time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		TemperatureC: 15.0, SourceProvider: "openweathermap",
	}

	require.NoError(t, store.SaveLatest("openweathermap", 37.57, 126.98, r1))
	require.NoError(t, store.SaveLatest("openweathermap", 37.57, 126.98, r2))

	loaded, err := store.LoadLatest("openweathermap", 37.57, 126.98)
	require.NoError(t, err)
	assert.Equal(t, r2.TemperatureC, loaded.TemperatureC, "second save must overwrite first")
}

// TestOfflineLoad_StaleFlagPreserved verifies that Stale and Message fields
// are preserved through a save/load round-trip.
func TestOfflineLoad_StaleFlagPreserved(t *testing.T) {
	dir := t.TempDir()
	store := web.NewDiskOfflineStore(dir)

	report := &web.WeatherReport{
		Timestamp:      time.Now().UTC(),
		TemperatureC:   20.0,
		Stale:          true,
		Message:        "offline",
		SourceProvider: "openweathermap",
	}
	require.NoError(t, store.SaveLatest("openweathermap", 37.57, 126.98, report))

	loaded, err := store.LoadLatest("openweathermap", 37.57, 126.98)
	require.NoError(t, err)
	assert.True(t, loaded.Stale)
	assert.Equal(t, "offline", loaded.Message)
}
