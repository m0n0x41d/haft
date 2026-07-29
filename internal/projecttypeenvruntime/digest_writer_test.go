package projecttypeenvruntime

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestTargetRuntimeDigestWriterCountEncodingGolden(t *testing.T) {
	t.Parallel()
	writer := targetRuntimeDigestWriter{}
	if err := writer.addCount(0x01020304); err != nil {
		t.Fatalf("addCount() error = %v", err)
	}

	const wantHex = "00000000000000080000000001020304"
	if got := hex.EncodeToString(writer.bytes()); got != wantHex {
		t.Fatalf("addCount() bytes = %s, want %s", got, wantHex)
	}

	before := writer.bytes()
	if err := writer.addCount(-1); err == nil {
		t.Fatal("addCount() accepted a negative count")
	}
	if !bytes.Equal(writer.bytes(), before) {
		t.Fatal("rejected negative count mutated digest bytes")
	}
}
