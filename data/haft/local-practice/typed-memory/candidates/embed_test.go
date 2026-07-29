package candidates

import (
	"bytes"
	"testing"
)

func TestSourcesForExactBaseTypeEnvRefPreserveEveryShippedEdition(t *testing.T) {
	tests := []struct {
		base   string
		source []byte
	}{
		{base: baseTypeEnvRefV1, source: SourceV1()},
		{base: baseTypeEnvRefV1_1, source: SourceV1_1()},
		{base: baseTypeEnvRefV1_2, source: SourceV1_2()},
		{base: baseTypeEnvRefV1_3, source: SourceV1_3()},
		{base: baseTypeEnvRefV1_4, source: SourceV1_4()},
	}
	for _, test := range tests {
		resolved := SourcesForExactBaseTypeEnvRef(test.base)
		if len(resolved) != 1 || !bytes.Equal(resolved[0], test.source) {
			t.Fatalf("source for %q was not resolved byte-for-byte", test.base)
		}
		declaredBase := []byte("base_type_env_ref: " + test.base + "\n")
		if !bytes.Contains(resolved[0], declaredBase) {
			t.Fatalf("source for %q does not declare that exact base", test.base)
		}
		resolved[0][0] ^= 0xff
		reread := SourcesForExactBaseTypeEnvRef(test.base)
		if len(reread) != 1 || !bytes.Equal(reread[0], test.source) {
			t.Fatalf("source for %q leaked mutable backing bytes", test.base)
		}
	}
}

func TestSourcesForExactBaseTypeEnvRefRejectUnknownCoordinate(t *testing.T) {
	resolved := SourcesForExactBaseTypeEnvRef(
		"typeenv:sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	)
	if len(resolved) != 0 {
		t.Fatal("unknown Base TypeEnv coordinate resolved to a shipped source")
	}
}
