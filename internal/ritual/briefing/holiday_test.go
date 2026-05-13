package briefing

import (
	"testing"
)

func TestLookupKoreanHoliday(t *testing.T) {
	tests := []struct {
		name      string
		year      int
		month     int
		day       int
		wantHoliday *KoreanHoliday
		wantErr   bool
	}{
		{
			name:  "2026-02-17 설날 (Lunar New Year)",
			year:  2026,
			month: 2,
			day:   17,
			wantHoliday: &KoreanHoliday{
				Name:      "설날",
				NameHanja: "春節",
				Date:      "2026-02-17",
				IsHoliday: true,
			},
			wantErr: false,
		},
		{
			name:  "2026-03-01 삼일절 (Independence Day)",
			year:  2026,
			month: 3,
			day:   1,
			wantHoliday: &KoreanHoliday{
				Name:      "삼일절",
				NameHanja: "三一節",
				Date:      "2026-03-01",
				IsHoliday: true,
			},
			wantErr: false,
		},
		{
			name:  "2026-05-05 어린이날 (Children's Day)",
			year:  2026,
			month: 5,
			day:   5,
			wantHoliday: &KoreanHoliday{
				Name:      "어린이날",
				NameHanja: "兒童節",
				Date:      "2026-05-05",
				IsHoliday: true,
			},
			wantErr: false,
		},
		{
			name:  "2026-09-21 추석 (Chuseok)",
			year:  2026,
			month: 9,
			day:   21,
			wantHoliday: &KoreanHoliday{
				Name:      "추석",
				NameHanja: "秋夕",
				Date:      "2026-09-21",
				IsHoliday: true,
			},
			wantErr: false,
		},
		{
			name:    "2026-01-15 no holiday",
			year:    2026,
			month:   1,
			day:     15,
			wantHoliday: nil,
			wantErr: false,
		},
		{
			name:    "1899 out of range",
			year:    1899,
			month:   1,
			day:     1,
			wantHoliday: nil,
			wantErr: true,
		},
		{
			name:    "2101 out of range",
			year:    2101,
			month:   1,
			day:     1,
			wantHoliday: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHoliday, err := LookupKoreanHoliday(tt.year, tt.month, tt.day)
			if (err != nil) != tt.wantErr {
				t.Errorf("LookupKoreanHoliday() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantHoliday == nil {
				if gotHoliday != nil {
					t.Errorf("LookupKoreanHoliday() = %v, want nil", gotHoliday)
				}
				return
			}
			if gotHoliday == nil {
				t.Errorf("LookupKoreanHoliday() = nil, want %v", tt.wantHoliday)
				return
			}
			if gotHoliday.Name != tt.wantHoliday.Name {
				t.Errorf("LookupKoreanHoliday().Name = %v, want %v", gotHoliday.Name, tt.wantHoliday.Name)
			}
			if gotHoliday.NameHanja != tt.wantHoliday.NameHanja {
				t.Errorf("LookupKoreanHoliday().NameHanja = %v, want %v", gotHoliday.NameHanja, tt.wantHoliday.NameHanja)
			}
			if gotHoliday.Date != tt.wantHoliday.Date {
				t.Errorf("LookupKoreanHoliday().Date = %v, want %v", gotHoliday.Date, tt.wantHoliday.Date)
			}
			if gotHoliday.IsHoliday != tt.wantHoliday.IsHoliday {
				t.Errorf("LookupKoreanHoliday().IsHoliday = %v, want %v", gotHoliday.IsHoliday, tt.wantHoliday.IsHoliday)
			}
		})
	}
}
