package sqlite

import (
	"fmt"
	"math"
)

// exactSQLiteInteger admits only values represented exactly by SQLite's
// signed 64-bit INTEGER storage class.
func exactSQLiteInteger(label string, value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("%s exceeds SQLite INTEGER range", label)
	}
	return int64(value), nil
}
