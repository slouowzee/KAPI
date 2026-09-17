package semver

import (
	"strconv"
	"strings"
)

// Greater reports whether version a is strictly greater than version b.
func Greater(a, b string) bool {
	return Compare(a, b) > 0
}

// Compare returns -1, 0 or 1 depending on whether a is lower than, equal to or
// greater than b. It follows semantic versioning precedence: dot-separated
// core parts are compared first (missing parts count as 0), a release is
// greater than any of its pre-releases, then pre-release identifiers are
// compared one by one. A leading "v" and build metadata are ignored.
func Compare(a, b string) int {
	coreA, preA := split(a)
	coreB, preB := split(b)

	if c := compareIdentifiers(coreA, coreB, "0"); c != 0 {
		return c
	}

	switch {
	case preA == "" && preB == "":
		return 0
	case preA == "":
		return 1
	case preB == "":
		return -1
	}
	return compareIdentifiers(strings.Split(preA, "."), strings.Split(preB, "."), "")
}

func split(v string) (core []string, pre string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '-'); i >= 0 {
		pre = v[i+1:]
		v = v[:i]
	}
	if v == "" {
		return nil, pre
	}
	return strings.Split(v, "."), pre
}

// compareIdentifiers compares two identifier lists. A missing identifier is
// replaced by filler; with an empty filler the shorter list is lower, as
// semver requires for pre-releases.
func compareIdentifiers(a, b []string, filler string) int {
	n := max(len(a), len(b))
	for i := 0; i < n; i++ {
		if i >= len(a) || i >= len(b) {
			if filler == "" {
				if i >= len(a) {
					return -1
				}
				return 1
			}
		}
		sa, sb := filler, filler
		if i < len(a) {
			sa = a[i]
		}
		if i < len(b) {
			sb = b[i]
		}
		if c := compareIdentifier(sa, sb); c != 0 {
			return c
		}
	}
	return 0
}

func compareIdentifier(a, b string) int {
	na, errA := strconv.Atoi(a)
	nb, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		switch {
		case na > nb:
			return 1
		case na < nb:
			return -1
		}
		return 0
	case errA == nil:
		// Numeric identifiers have lower precedence than alphanumeric ones.
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}
