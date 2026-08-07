package fpfrefresh

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTokenGateCoordinatesBindsExactFixtureBytes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "corpus.json")
	payload := []byte(`{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": "fixture-308edac-v1",
  "cases": [
    {"case_id": "one", "minimum_reduction": 0.3}
  ]
}
`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	coordinates, err := ReadTokenGateCoordinates(path)
	if err != nil {
		t.Fatalf("ReadTokenGateCoordinates() error = %v", err)
	}
	sum := sha256.Sum256(payload)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if coordinates.FixtureRevision != "fixture-308edac-v1" ||
		coordinates.FixtureDigest != wantDigest {
		t.Fatalf("coordinates = %#v, want revision and digest %s", coordinates, wantDigest)
	}

	tampered := append(append([]byte(nil), payload...), ' ')
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := ReadTokenGateCoordinates(path)
	if err != nil {
		t.Fatalf("ReadTokenGateCoordinates(tampered) error = %v", err)
	}
	if changed.FixtureRevision != coordinates.FixtureRevision ||
		changed.FixtureDigest == coordinates.FixtureDigest {
		t.Fatalf("exact-byte tamper was not isolated to fixture digest: %#v", changed)
	}
}

func TestReadTokenGateCoordinatesRejectsMalformedEnvelope(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"wrong schema": `{
  "schema_version": "wrong",
  "fixture_revision": "fixture-v1",
  "cases": [{}]
}`,
		"blank revision": `{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": " ",
  "cases": [{}]
}`,
		"missing cases": `{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": "fixture-v1",
  "cases": []
}`,
		"non-object case": `{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": "fixture-v1",
  "cases": ["not-an-object"]
}`,
		"trailing value": `{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": "fixture-v1",
  "cases": [{}]
} {}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "corpus.json")
			if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadTokenGateCoordinates(path); err == nil {
				t.Fatal("ReadTokenGateCoordinates() error = nil")
			}
		})
	}
}

func TestReadTokenGateCoordinatesRejectsOversizedFixture(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(
		path,
		[]byte(strings.Repeat("x", maxTokenGateFixtureBytes+1)),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTokenGateCoordinates(path); err == nil {
		t.Fatal("ReadTokenGateCoordinates() error = nil")
	}
}

func TestReadBoundedTokenGateFixtureRejectsOversizedStream(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(strings.Repeat("x", maxTokenGateFixtureBytes+1))
	if _, err := readBoundedTokenGateFixture(reader); err == nil {
		t.Fatal("readBoundedTokenGateFixture() error = nil")
	}
}

func TestTokenGateCompatibilityDeltasDistinguishUnboundAndChangedFixture(t *testing.T) {
	t.Parallel()

	const digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	candidate := &TokenGateCoordinates{
		FixtureRevision: "fixture-v2",
		FixtureDigest:   digestB,
	}

	deltas, err := tokenGateCompatibilityDeltas(nil, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 ||
		deltas[0].Family() != DeltaTokenBudgetCorpus ||
		deltas[0].Kind() != DeltaTokenBudgetCorpusChanged ||
		deltas[0].Before() != "unbound" ||
		deltas[0].After() != "fixture-v2@"+digestB {
		t.Fatalf("unbound fixture delta = %#v", deltas)
	}

	predecessor := &IntegrationLock{TokenGate: &TokenGateCoordinates{
		FixtureRevision: "fixture-v1",
		FixtureDigest:   digestA,
	}}
	deltas, err = tokenGateCompatibilityDeltas(predecessor, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 ||
		deltas[0].Before() != "fixture-v1@"+digestA ||
		deltas[0].After() != "fixture-v2@"+digestB {
		t.Fatalf("changed fixture delta = %#v", deltas)
	}

	predecessor.TokenGate = candidate
	deltas, err = tokenGateCompatibilityDeltas(predecessor, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 0 {
		t.Fatalf("unchanged fixture deltas = %#v, want none", deltas)
	}
}

func TestVerifyRepositoryTokenGateFixtureFailsClosedOnByteDrift(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "corpus.json")
	payload := []byte(`{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": "fixture-v1",
  "cases": [{"case_id": "one"}]
}
`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	coordinates, err := ReadTokenGateCoordinates(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyRepositoryTokenGateFixture(path, &coordinates); err != nil {
		t.Fatalf("verifyRepositoryTokenGateFixture(exact) error = %v", err)
	}
	if err := verifyRepositoryTokenGateFixture(filepath.Join(t.TempDir(), "missing"), nil); err != nil {
		t.Fatalf("nil token fixture should be inapplicable: %v", err)
	}

	changed := append(append([]byte(nil), payload...), ' ')
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyRepositoryTokenGateFixture(path, &coordinates); err == nil {
		t.Fatal("verifyRepositoryTokenGateFixture(drifted) error = nil")
	}
}
