package timeutil

import (
	"fmt"
	"time"
)

// AgeString returns a human-readable description of how long ago ts occurred relative to now.
// Day boundaries are compared in now's location so that "yesterday" reflects the caller's
// local calendar rather than UTC midnight.
func AgeString(ts string, now time.Time) string {
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "?d ago"
	}
	elapsed := now.Sub(parsed)
	if elapsed < time.Hour {
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	}
	loc := now.Location()
	ny, nm, nd := now.Date()
	py, pm, pd := parsed.In(loc).Date()
	nowDay := time.Date(ny, nm, nd, 0, 0, 0, 0, loc)
	parsedDay := time.Date(py, pm, pd, 0, 0, 0, 0, loc)
	if parsedDay.AddDate(0, 0, 1).Equal(nowDay) {
		return "yesterday"
	}
	if elapsed < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
}
