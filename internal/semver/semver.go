package semver

import (
	"strconv"
	"strings"
)

func stripPreRelease(s string) string {
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func Greater(a, b string) bool {
	partsA := strings.Split(strings.TrimPrefix(a, "v"), ".")
	partsB := strings.Split(strings.TrimPrefix(b, "v"), ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		var sa, sb string
		if i < len(partsA) {
			sa = partsA[i]
		}
		if i < len(partsB) {
			sb = partsB[i]
		}
		na, errA := strconv.Atoi(stripPreRelease(sa))
		nb, errB := strconv.Atoi(stripPreRelease(sb))
		if errA == nil && errB == nil {
			if na != nb {
				return na > nb
			}
			if sa != sb {
				return sa > sb
			}
		} else if sa != sb {
			return sa > sb
		}
	}
	return false
}
