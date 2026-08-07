package version

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"0.4.0", "0.10.0", -1},
		{"0.10.0", "0.4.0", 1},
		{"dev", "1.0.0", -1},
		{"1.0.0", "dev", 1},
		{"dev", "dev", 0},
		{"", "1.0.0", -1},
		{"1.0.0", "", 1},
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		if got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMajor(t *testing.T) {
	cases := []struct {
		v    string
		want int
	}{
		{"1.0.0", 1},
		{"2.3.4", 2},
		{"0.4.0", 0},
		{"dev", -1},
		{"", -1},
	}
	for _, c := range cases {
		got := Major(c.v)
		if got != c.want {
			t.Errorf("Major(%q) = %d, want %d", c.v, got, c.want)
		}
	}
}
