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
