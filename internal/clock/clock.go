package clock

import (
	"os"
	"time"
)

// Now returns the current UTC time. If GOALIE_FIXED_TIME_OVERRIDE is set to a
// valid RFC3339 timestamp, that value is returned instead. Intended for use by
// tests/compat/generate-data.sh, which drives the compiled binary as a
// subprocess and so can only control time through the environment. In-process
// Go tests should prefer injecting a Clock (e.g. FakeClock) instead.
func Now() time.Time {
	if v := os.Getenv("GOALIE_FIXED_TIME_OVERRIDE"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

// Clock abstracts the current time so production code can depend on an
// interface and tests can substitute a fixed value instead of racing the
// wall clock.
type Clock interface {
	Now() time.Time
}

// RealClock is the production Clock, backed by Now (and therefore still
// honoring GOALIE_FIXED_TIME_OVERRIDE for subprocess-driven fixture generation).
type RealClock struct{}

func (RealClock) Now() time.Time { return Now() }

// FakeClock is a Clock that always returns a fixed time. Intended for tests.
type FakeClock struct {
	T time.Time
}

func (f FakeClock) Now() time.Time { return f.T }
