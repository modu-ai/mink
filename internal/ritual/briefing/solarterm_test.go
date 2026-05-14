package briefing

import (
	"testing"
)

func TestSolarTermOnDate(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		month    int
		day      int
		wantTerm *SolarTerm
		wantErr  bool
	}{
		{
			name:  "2026-02-04 입추 (Beginning of Spring)",
			year:  2026,
			month: 2,
			day:   4,
			wantTerm: &SolarTerm{
				Name:      "입춘",
				NameHanja: "立春",
				Date:      "2026-02-04",
			},
			wantErr: false,
		},
		{
			name:  "2026-05-06 입하 (Beginning of Summer)",
			year:  2026,
			month: 5,
			day:   6,
			wantTerm: &SolarTerm{
				Name:      "입하",
				NameHanja: "立夏",
				Date:      "2026-05-06",
			},
			wantErr: false,
		},
		{
			name:  "2026-08-08 입추 (Beginning of Autumn)",
			year:  2026,
			month: 8,
			day:   8,
			wantTerm: &SolarTerm{
				Name:      "입추",
				NameHanja: "立秋",
				Date:      "2026-08-08",
			},
			wantErr: false,
		},
		{
			name:  "2026-11-08 입동 (Beginning of Winter)",
			year:  2026,
			month: 11,
			day:   8,
			wantTerm: &SolarTerm{
				Name:      "입동",
				NameHanja: "立冬",
				Date:      "2026-11-08",
			},
			wantErr: false,
		},
		{
			name:     "2026-01-15 no solar term",
			year:     2026,
			month:    1,
			day:      15,
			wantTerm: nil,
			wantErr:  false,
		},
		{
			name:     "1899 out of range",
			year:     1899,
			month:    1,
			day:      1,
			wantTerm: nil,
			wantErr:  true,
		},
		{
			name:     "2101 out of range",
			year:     2101,
			month:    1,
			day:      1,
			wantTerm: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTerm, err := SolarTermOnDate(tt.year, tt.month, tt.day)
			if (err != nil) != tt.wantErr {
				t.Errorf("SolarTermOnDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantTerm == nil {
				if gotTerm != nil {
					t.Errorf("SolarTermOnDate() = %v, want nil", gotTerm)
				}
				return
			}
			if gotTerm == nil {
				t.Errorf("SolarTermOnDate() = nil, want %v", tt.wantTerm)
				return
			}
			if gotTerm.Name != tt.wantTerm.Name {
				t.Errorf("SolarTermOnDate().Name = %v, want %v", gotTerm.Name, tt.wantTerm.Name)
			}
			if gotTerm.NameHanja != tt.wantTerm.NameHanja {
				t.Errorf("SolarTermOnDate().NameHanja = %v, want %v", gotTerm.NameHanja, tt.wantTerm.NameHanja)
			}
			if gotTerm.Date != tt.wantTerm.Date {
				t.Errorf("SolarTermOnDate().Date = %v, want %v", gotTerm.Date, tt.wantTerm.Date)
			}
		})
	}
}
