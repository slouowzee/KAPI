package packages

import (
	"strconv"
	"strings"
)

func semverGreater(a, b string) bool {
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

		na, errA := strconv.Atoi(sa)
		nb, errB := strconv.Atoi(sb)

		if errA == nil && errB == nil {
			if na != nb {
				return na > nb
			}
		} else {
			if sa != sb {
				return sa > sb
			}
		}
	}
	return false
}
