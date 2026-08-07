package typedmemorystore

import (
	"fmt"
	"math"
)

// sqliteIntegerFromUint64 is the pure platform boundary for values persisted
// in SQLite INTEGER columns. Callers retain ownership of domain-specific error
// classification and must fail closed when exact conversion is unavailable.
func sqliteIntegerFromUint64(value uint64) (int64, bool) {
	if value > math.MaxInt64 {
		return 0, false
	}
	return int64(value), true
}

// uint64FromSQLiteInteger rejects negative SQLite INTEGER values instead of
// allowing their two's-complement representation to enter the domain model.
func uint64FromSQLiteInteger(value int64) (uint64, bool) {
	if value < 0 {
		return 0, false
	}
	return uint64(value), true
}

// sliceIndexFromUint64 admits only values exactly representable by this
// process's slice/index type.
func sliceIndexFromUint64(value uint64) (int, bool) {
	if value > math.MaxInt {
		return 0, false
	}
	return int(value), true
}

func exactSQLiteCoordinate(value uint64, detail string) (int64, error) {
	converted, exact := sqliteIntegerFromUint64(value)
	if !exact {
		return 0, fmt.Errorf("%s exceeds SQLite INTEGER", detail)
	}
	return converted, nil
}
