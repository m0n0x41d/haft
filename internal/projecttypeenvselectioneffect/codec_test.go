package projecttypeenvselectioneffect

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestCanonicalReaderRejectsMaxUint64LengthBeforeIndexArithmetic(t *testing.T) {
	t.Parallel()

	canonical := make([]byte, 8)
	binary.BigEndian.PutUint64(canonical, math.MaxUint64)
	reader := &canonicalReader{value: canonical}

	if _, err := reader.readBytes("hostile field"); err == nil {
		t.Fatal("readBytes(MaxUint64 length) succeeded")
	} else if !strings.Contains(err.Error(), "exceeds canonical record limit") {
		t.Fatalf("readBytes(MaxUint64 length) error = %q", err)
	}
	if reader.offset != 8 {
		t.Fatalf("readBytes(MaxUint64 length) offset = %d, want prefix length 8", reader.offset)
	}
}
