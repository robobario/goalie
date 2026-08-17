package timeutil

import (
	"testing"
	"time"
)

func TestAgeString(t *testing.T) {
	base := time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		ts   time.Time
		want string
	}{
		{"minutes", base.Add(-30 * time.Minute), "30m ago"},
		{"hours same day", base.Add(-5 * time.Hour), "5h ago"},
		{"yesterday", base.Add(-24 * time.Hour), "yesterday"},
		{"two days", base.Add(-48 * time.Hour), "2d ago"},
		{"invalid ts", time.Time{}, "?d ago"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ts string
			if tc.ts.IsZero() {
				ts = "not-a-timestamp"
			} else {
				ts = tc.ts.Format(time.RFC3339)
			}
			got := AgeString(ts, base)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAgeStringDSTSpringForward guards against a day-comparison bug on the day
// after a DST spring-forward transition. In New York, March 10 midnight is still
// EST (UTC-5 = 05:00 UTC) while March 11 midnight is EDT (UTC-4 = 04:00 UTC),
// so the two consecutive midnight boundaries are only 23h apart. A Sub()==24h
// check would return "1d ago" instead of "yesterday". Calendar-based AddDate
// comparison is correct in all cases.
func TestAgeStringDSTSpringForward(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("America/New_York timezone not available")
	}
	now := time.Date(2024, 3, 11, 12, 0, 0, 0, loc)
	parsed := time.Date(2024, 3, 10, 12, 0, 0, 0, loc)
	got := AgeString(parsed.Format(time.RFC3339), now)
	if got != "yesterday" {
		t.Errorf("got %q, want %q", got, "yesterday")
	}
}

// TestAgeStringTimezone guards against the UTC-truncation bug where a task
// updated early in the morning local time (but on the previous UTC day) was
// incorrectly shown as "yesterday" instead of "Xh ago".
//
// Scenario: user is in UTC+5. Current local time is 06:00 (01:00 UTC on Mar 10).
// Task updated at 03:00 local (22:00 UTC on Mar 9) — 3 hours ago, same local calendar day.
// UTC-based truncation sees Mar 10 vs Mar 9 → 24h gap → wrong "yesterday".
// Location-aware comparison sees Mar 10 vs Mar 10 → correct "3h ago".
func TestAgeStringTimezone(t *testing.T) {
	loc := time.FixedZone("UTC+5", 5*60*60)
	now := time.Date(2024, 3, 10, 6, 0, 0, 0, loc)    // 01:00 UTC Mar 10
	parsed := time.Date(2024, 3, 10, 3, 0, 0, 0, loc)  // 22:00 UTC Mar 9, 3h before now
	ts := parsed.Format(time.RFC3339)

	got := AgeString(ts, now)
	if got != "3h ago" {
		t.Errorf("got %q, want %q", got, "3h ago")
	}
}

// TestAgeStringCalendarDaysNotRawDuration guards against issue #152, where
// a raw elapsed.Hours()/24 truncation undercounted local calendar midnights
// crossed. In NZST (UTC+12), an entry logged at 13:08 local two calendar days
// before "now" at 09:16 local only has ~44 raw elapsed hours (44/24 truncates
// to 1), but two local midnights have passed, so it should read "2d ago".
func TestAgeStringCalendarDaysNotRawDuration(t *testing.T) {
	loc := time.FixedZone("NZST", 12*60*60)
	parsed := time.Date(2026, 8, 12, 13, 8, 48, 0, loc)
	now := time.Date(2026, 8, 14, 9, 16, 56, 0, loc)
	ts := parsed.Format(time.RFC3339)

	got := AgeString(ts, now)
	if got != "2d ago" {
		t.Errorf("got %q, want %q", got, "2d ago")
	}
}
