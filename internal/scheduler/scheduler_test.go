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
			slot := time.Now().UTC().Truncate(interval)
			lastRun := tc.lastRun(slot)
			got := isDue(j, lastRun)
			if got != tc.wantDue {
				t.Errorf("isDue() = %v, want %v (slot=%v, lastRun=%v)", got, tc.wantDue, slot, lastRun)
			}
		})
	}
}
