package facebook

import (
	"regexp"
)

const (
	V12 = "v12.0"
	V13 = "v13.0"
	V14 = "v14.0"
	V15 = "v15.0"
	V16 = "v16.0"
	V17 = "v17.0" // 2023-07-17
	// API Version; Latest; Default
	Latest = V17
)

var (
	// https://semver.org/
	semVer = regexp.MustCompile(`^v([1-9]\d*)(\.(0|[1-9]\d*))$`)
)

// IsVersion indicates whether given s string represents an Grapth-API Version.
func IsVersion(s string) bool {
	return semVer.MatchString(s)
}
