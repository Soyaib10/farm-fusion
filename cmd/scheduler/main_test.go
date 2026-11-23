package main

import (
	"testing"
	"time"
)

func TestNextDailyRun(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before the hour runs today", time.Date(2026, 6, 1, 4, 30, 0, 0, ny), time.Date(2026, 6, 1, 5, 0, 0, 0, ny)},
		{"exactly on the hour is due now", time.Date(2026, 6, 1, 5, 0, 0, 0, ny), time.Date(2026, 6, 1, 5, 0, 0, 0, ny)},
		{"after the hour runs tomorrow", time.Date(2026, 6, 1, 5, 0, 1, 0, ny), time.Date(2026, 6, 2, 5, 0, 0, 0, ny)},
		// DST starts 2026-03-08 in New York; a fixed 24h step would land on 06:00.
		{"spring forward keeps 05:00", time.Date(2026, 3, 7, 6, 0, 0, 0, ny), time.Date(2026, 3, 8, 5, 0, 0, 0, ny)},
		// DST ends 2026-11-01; a fixed 24h step would land on 04:00.
		{"fall back keeps 05:00", time.Date(2026, 10, 31, 6, 0, 0, 0, ny), time.Date(2026, 11, 1, 5, 0, 0, 0, ny)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nextDailyRun(tc.now, 5)
			if !got.Equal(tc.want) || got.Hour() != 5 {
				t.Fatalf("nextDailyRun(%s) = %s, want %s", tc.now, got, tc.want)
			}
		})
	}
}
