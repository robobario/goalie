package schema

import (
	"regexp"
	"testing"
)

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestVersionIsSemver(t *testing.T) {
	if !semverRe.MatchString(Version) {
		t.Errorf("Version %q is not valid X.Y.Z semver", Version)
	}
}
