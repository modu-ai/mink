package briefing

import (
	"testing"
	"time"
)

func TestMantraCollector_Collect(t *testing.T) {
	t.Run("single mantra", func(t *testing.T) {
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		cfg := DefaultConfig()
		cfg.Mantra = "오늘도 좋은 하루!"

		collector := NewMantraCollector(cfg)
		module, status := collector.Collect(today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		if module.Text != "오늘도 좋은 하루!" {
			t.Errorf("expected Text='오늘도 좋은 하루!', got '%s'", module.Text)
		}

		if module.Index != 0 {
			t.Errorf("expected Index=0 for single mantra, got %d", module.Index)
		}

		if module.Total != 1 {
			t.Errorf("expected Total=1, got %d", module.Total)
		}
	})

	t.Run("mantra rotation - week 20", func(t *testing.T) {
		// May 14, 2026 is in ISO week 20
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		cfg := &Config{
			Mantras: []string{
				"mantra1",
				"mantra2",
				"mantra3",
				"mantra4",
			},
		}

		collector := NewMantraCollector(cfg)
		module, status := collector.Collect(today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		// Week 20, 4 mantras: (20-1) % 4 = 3, so should be mantra4 (index 3)
		expectedIndex := 3
		expectedText := "mantra4"

		if module.Index != expectedIndex {
			t.Errorf("expected Index=%d, got %d", expectedIndex, module.Index)
		}

		if module.Text != expectedText {
			t.Errorf("expected Text='%s', got '%s'", expectedText, module.Text)
		}

		if module.Total != 4 {
			t.Errorf("expected Total=4, got %d", module.Total)
		}
	})

	t.Run("mantra rotation - week 1", func(t *testing.T) {
		// January 1, 2026 is in ISO week 1
		today := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		cfg := &Config{
			Mantras: []string{
				"first",
				"second",
				"third",
			},
		}

		collector := NewMantraCollector(cfg)
		module, status := collector.Collect(today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		// Week 1, 3 mantras: (1-1) % 3 = 0, so should be "first"
		if module.Index != 0 {
			t.Errorf("expected Index=0, got %d", module.Index)
		}

		if module.Text != "first" {
			t.Errorf("expected Text='first', got '%s'", module.Text)
		}
	})

	t.Run("clinical vocabulary detection", func(t *testing.T) {
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		cfg := DefaultConfig()
		cfg.Mantra = "우울증과 불안감을 극복하세요" // Contains clinical terms

		collector := NewMantraCollector(cfg)
		_, status := collector.Collect(today)

		if status != "skipped" {
			t.Errorf("expected status 'skipped' for clinical vocabulary, got '%s'", status)
		}
	})

	t.Run("empty mantra config", func(t *testing.T) {
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		cfg := &Config{
			Mantra:  "",
			Mantras: []string{},
		}

		collector := NewMantraCollector(cfg)
		module, status := collector.Collect(today)

		// Empty config should not fail, just return empty text
		if status != "ok" {
			t.Errorf("expected status 'ok' with empty config, got '%s'", status)
		}

		if module.Text != "" {
			t.Errorf("expected empty Text with empty config, got '%s'", module.Text)
		}
	})

	t.Run("mantra priority - single over array", func(t *testing.T) {
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		cfg := DefaultConfig()
		cfg.Mantra = "single"
		cfg.Mantras = []string{"array1", "array2"} // Should be ignored

		collector := NewMantraCollector(cfg)
		module, status := collector.Collect(today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		if module.Text != "single" {
			t.Errorf("expected single mantra to take priority, got '%s'", module.Text)
		}

		if module.Total != 1 {
			t.Errorf("expected Total=1 for single mantra, got %d", module.Total)
		}
	})
}
