// Package update looks for newer Nota releases and installs them.
//
// It does exactly what install.sh does — ask GitHub for the latest tag, fetch
// the release asset, check its SHA-256 against the published checksums, and put
// the binary in place — with no dependency on Wails or the services layer, so
// every part of it can be tested against a local HTTP server.
//
// Nothing here runs unless the user has said yes. The app made no network
// requests at all before this package existed, and that stays true until the
// preference in settings says otherwise.
package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a release version: the MAJOR.MINOR.PATCH the tags use.
type Version struct {
	Major, Minor, Patch int
}

// ParseVersion reads "4.1.0" or "v4.1.0". A locally built binary reports "dev",
// which is not a version and is rejected — that is how a dev build opts out of
// update checks without a special case anywhere else.
func ParseVersion(s string) (Version, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("not a version: %q", s)
	}
	var v Version
	for i, into := range []*int{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("not a version: %q", s)
		}
		*into = n
	}
	return v, nil
}

// Less reports whether v is older than other. Comparison is numeric per field,
// so 4.10.0 is correctly newer than 4.9.0.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
