package briefing

import (
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config with single mantra",
			cfg: Config{
				Mantra: "Start your day with purpose",
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple mantras",
			cfg: Config{
				Mantras: []string{"Mantra 1", "Mantra 2", "Mantra 3"},
			},
			wantErr: false,
		},
		{
			name: "empty config is invalid",
			cfg:  Config{
				// No mantra or mantras
			},
			wantErr: true,
		},
		{
			name: "config with both single and array mantras is invalid",
			cfg: Config{
				Mantra:  "Single",
				Mantras: []string{"Array 1", "Array 2"},
			},
			wantErr: true,
		},
		{
			name: "config with empty mantra string is invalid",
			cfg: Config{
				Mantra: "",
			},
			wantErr: true,
		},
		{
			name: "config with empty mantras array is invalid",
			cfg: Config{
				Mantras: []string{},
			},
			wantErr: true,
		},
		{
			name: "config with empty string in mantras array is invalid",
			cfg: Config{
				Mantras: []string{"Valid", ""},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("DefaultConfig() produced invalid config: %v", err)
	}

	// Verify default mantra is not empty
	if cfg.Mantra == "" && len(cfg.Mantras) == 0 {
		t.Error("DefaultConfig() has no mantra or mantras")
	}
}
