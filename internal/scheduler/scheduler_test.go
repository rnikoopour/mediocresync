package scheduler

import (
	"testing"
	"time"

	"github.com/rnikoopour/mediocresync/internal/db"
)

func job(unit string, value int) *db.SyncJob {
	return &db.SyncJob{IntervalUnit: unit, IntervalValue: value}
}

func TestIsDue(t *testing.T) {
	// Pin "now" to 01:00:30 UTC so slot boundaries are predictable.
	// We do this by constructing lastRun values relative to a known now,
	// then verifying isDue against the real clock — so we need times that
	// are clearly before or after the current slot.
	//
	// Instead, test using times relative to the actual current slot so the
	// tests are not time-of-day dependent.

	for _, tc := range []struct {
		name     string
		unit     string
		value    int
		lastRun  func(slot time.Time) time.Time // relative to current slot start
		wantDue  bool
	}{
		{
			name:    "no previous run is always due",
			unit:    "minutes", value: 60,
			lastRun: func(_ time.Time) time.Time { return time.Time{} },
			wantDue: true,
		},
		{
			name:    "ran before current 60m slot",
			unit:    "minutes", value: 60,
			lastRun: func(slot time.Time) time.Time { return slot.Add(-1 * time.Second) },
			wantDue: true,
		},
		{
			name:    "ran inside current 60m slot",
			unit:    "minutes", value: 60,
			lastRun: func(slot time.Time) time.Time { return slot.Add(1 * time.Second) },
			wantDue: false,
		},
		{
			name:    "ran exactly at current 60m slot start",
			unit:    "minutes", value: 60,
			lastRun: func(slot time.Time) time.Time { return slot },
			wantDue: false,
		},
		{
			name:    "ran before current 30m slot",
			unit:    "minutes", value: 30,
			lastRun: func(slot time.Time) time.Time { return slot.Add(-1 * time.Second) },
			wantDue: true,
		},
		{
			name:    "ran inside current 30m slot",
			unit:    "minutes", value: 30,
			lastRun: func(slot time.Time) time.Time { return slot.Add(1 * time.Second) },
			wantDue: false,
		},
		{
			name:    "ran before current 1h slot",
			unit:    "hours", value: 1,
			lastRun: func(slot time.Time) time.Time { return slot.Add(-1 * time.Second) },
			wantDue: true,
		},
		{
			name:    "ran inside current 1h slot",
			unit:    "hours", value: 1,
			lastRun: func(slot time.Time) time.Time { return slot.Add(30 * time.Minute) },
			wantDue: false,
		},
		{
			name:    "ran before current 1d slot",
			unit:    "days", value: 1,
			lastRun: func(slot time.Time) time.Time { return slot.Add(-1 * time.Second) },
			wantDue: true,
		},
		{
			name:    "ran inside current 1d slot",
			unit:    "days", value: 1,
			lastRun: func(slot time.Time) time.Time { return slot.Add(1 * time.Hour) },
			wantDue: false,
		},
		{
			name:    "unknown unit is never due",
			unit:    "weeks", value: 1,
			lastRun: func(_ time.Time) time.Time { return time.Time{} },
			wantDue: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := job(tc.unit, tc.value)
			var interval time.Duration
			switch tc.unit {
			case "minutes":
				interval = time.Duration(tc.value) * time.Minute
			case "hours":
				interval = time.Duration(tc.value) * time.Hour
			case "days":
				interval = time.Duration(tc.value) * 24 * time.Hour
			}
			slot := currentSlotStart(time.Now(), interval)
			lastRun := tc.lastRun(slot)
			got := isDue(j, lastRun)
			if got != tc.wantDue {
				t.Errorf("isDue() = %v, want %v (slot=%v, lastRun=%v)", got, tc.wantDue, slot, lastRun)
			}
		})
	}
}

func TestCurrentSlotStart(t *testing.T) {
	// Use a UTC+5 timezone to verify slots anchor to local midnight, not UTC.
	loc := time.FixedZone("UTC+5", 5*60*60)

	for _, tc := range []struct {
		name     string
		now      time.Time
		interval time.Duration
		wantHour int
		wantMin  int
	}{
		{
			// 08:00 is in the 00:00–12:00 slot; current slot start = 00:00
			name:     "12h: 08:00 is in the 00:00 slot",
			now:      time.Date(2026, 3, 28, 8, 0, 0, 0, loc),
			interval: 12 * time.Hour,
			wantHour: 0, wantMin: 0,
		},
		{
			// 13:00 is in the 12:00–00:00 slot; current slot start = 12:00
			name:     "12h: 13:00 is in the 12:00 slot",
			now:      time.Date(2026, 3, 28, 13, 0, 0, 0, loc),
			interval: 12 * time.Hour,
			wantHour: 12, wantMin: 0,
		},
		{
			// 07:30 is in the 06:00–12:00 slot; current slot start = 06:00
			name:     "6h: 07:30 is in the 06:00 slot",
			now:      time.Date(2026, 3, 28, 7, 30, 0, 0, loc),
			interval: 6 * time.Hour,
			wantHour: 6, wantMin: 0,
		},
		{
			// 14:45 is in the 14:30–15:00 slot; current slot start = 14:30
			name:     "30m: 14:45 is in the 14:30 slot",
			now:      time.Date(2026, 3, 28, 14, 45, 0, 0, loc),
			interval: 30 * time.Minute,
			wantHour: 14, wantMin: 30,
		},
		{
			// Any time on a given day is in the 00:00 slot for a 1d interval
			name:     "1d: 09:00 is in the 00:00 slot",
			now:      time.Date(2026, 3, 28, 9, 0, 0, 0, loc),
			interval: 24 * time.Hour,
			wantHour: 0, wantMin: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := currentSlotStart(tc.now, tc.interval)
			gotLocal := got.In(loc)
			if gotLocal.Hour() != tc.wantHour || gotLocal.Minute() != tc.wantMin {
				t.Errorf("currentSlotStart() = %v, want %02d:%02d local", gotLocal, tc.wantHour, tc.wantMin)
			}
		})
	}
}
