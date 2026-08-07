package clock

import (
	"os"
	"time"
)

// Now returns the current UTC time. If GOALIE_FIXED_TIME_OVERRIDE is set to a
// valid RFC3339 timestamp, that value is returned instead. Intended for use in
// test fixture generation so that stored timestamps are deterministic.
func Now() time.Time {
	if v := os.Getenv("GOALIE_FIXED_TIME_OVERRIDE"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}
