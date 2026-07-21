package slugify

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"alice@example.com", "alice-example-com"},
		{"Alice@Example.COM", "alice-example-com"},
		{"user+tag@example.com", "user-tag-example-com"},
		{"@example.com", "example-com"},
		{"john.doe@company.org", "john-doe-company-org"},
		{"", ""},
	}
	for _, c := range cases {
		got := Slugify(c.input)
		if got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
