package version

import (
	"strconv"
	"strings"
)

// Compare returns -1, 0, or 1 if a is less than, equal to, or greater than b.
// Non-release strings (e.g. "dev", "") sort below any X.Y.Z version.
func Compare(a, b string) int {
	pa, aOk := parse(a)
	pb, bOk := parse(b)
	switch {
	case !aOk && !bOk:
		return 0
	case !aOk:
		return -1
	case !bOk:
		return 1
	}
	for i := range pa {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// Major returns the major component of a version string, or -1 if unparseable.
func Major(v string) int {
	parts, ok := parse(v)
	if !ok {
		return -1
	}
	return parts[0]
}

func parse(v string) ([3]int, bool) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
