package projecttypeenvselection

import "math"

func sliceIndexFromUint64(value uint64) (int, bool) {
	if value > math.MaxInt {
		return 0, false
	}
	return int(value), true
}
