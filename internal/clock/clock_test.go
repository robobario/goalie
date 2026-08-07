package clock

import (
	"testing"
	"time"
)

func TestNow_returns_real_time_when_no_override(t *testing.T) {
	t.Setenv("GOALIE_FIXED_TIME_OVERRIDE", "")
	before := time.Now().UTC()
	got := Now()
	after := time.Now().UTC()
	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %v, want between %v and %v", got, before, after)
	}
}

func TestNow_returns_fixed_time_when_override_set(t *testing.T) {
	t.Setenv("GOALIE_FIXED_TIME_OVERRIDE", "2024-01-08T10:00:00Z")
	got := Now()
	want := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Now() = %v, want %v", got, want)
	}
}

func TestNow_ignores_invalid_override(t *testing.T) {
	t.Setenv("GOALIE_FIXED_TIME_OVERRIDE", "not-a-time")
	before := time.Now().UTC()
	got := Now()
	after := time.Now().UTC()
	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %v, want real time between %v and %v", got, before, after)
	}
}
