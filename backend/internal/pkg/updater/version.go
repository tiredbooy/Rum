// Package updater checks GitHub Releases for a newer Rum, with mirror fallback,
// a once-a-day throttle and optional checksum-verified asset download. It never
// installs anything or replaces the running binary.
package updater

import (
	"strconv"
	"strings"
)

type semver struct {
	nums [3]int
	pre  []string
}

func parseSemver(s string) (semver, bool) {
	var v semver
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i] // build metadata never affects precedence
	}
	core := s
	if i := strings.IndexByte(s, '-'); i >= 0 {
		core = s[:i]
		v.pre = strings.Split(s[i+1:], ".")
	}
	parts := strings.Split(core, ".")
	if core == "" || len(parts) > 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p[0] == '+' || p[0] == '-' {
			return v, false
		}
		v.nums[i] = n
	}
	return v, true
}

// CompareVersions orders two semver strings (leading "v" and missing minor or
// patch parts allowed). It returns -1, 0 or 1; ok is false if either side is
// not a version.
func CompareVersions(a, b string) (cmp int, ok bool) {
	va, okA := parseSemver(a)
	vb, okB := parseSemver(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range va.nums {
		if va.nums[i] != vb.nums[i] {
			return sign(va.nums[i] - vb.nums[i]), true
		}
	}
	return comparePre(va.pre, vb.pre), true
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	if n > 0 {
		return 1
	}
	return 0
}

// comparePre follows semver: a release outranks any prerelease; identifiers
// compare numerically when both are numbers, otherwise as text.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		na, errA := strconv.Atoi(a[i])
		nb, errB := strconv.Atoi(b[i])
		switch {
		case errA == nil && errB == nil && na != nb:
			return sign(na - nb)
		case errA == nil && errB != nil:
			return -1
		case errA != nil && errB == nil:
			return 1
		case errA != nil && errB != nil && a[i] != b[i]:
			return sign(strings.Compare(a[i], b[i]))
		}
	}
	return sign(len(a) - len(b))
}

// IsNewer reports whether latest is a strictly higher version than current.
// Anything unparseable is "not newer", so a bad payload never raises a notice.
func IsNewer(latest, current string) bool {
	c, ok := CompareVersions(latest, current)
	return ok && c > 0
}
