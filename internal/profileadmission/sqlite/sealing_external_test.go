package sqlite_test

import (
	"encoding/json"
	"testing"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
)

type foreignCanonicalAdmission struct {
	profileadmissionsqlite.CanonicalProfileAdmission
}

func TestCanonicalProfileAdmissionCannotBeForgedByZeroEmbeddingOrJSON(t *testing.T) {
	zero := profileadmissionsqlite.CanonicalProfileAdmission{}
	if zero.Valid() {
		t.Fatal("zero CanonicalProfileAdmission is valid")
	}
	embedded := foreignCanonicalAdmission{}
	if embedded.Valid() {
		t.Fatal("foreign embedded CanonicalProfileAdmission is valid")
	}
	var raw any = embedded
	if _, ok := raw.(profileadmissionsqlite.CanonicalProfileAdmission); ok {
		t.Fatal("foreign embedding became the concrete canonical token")
	}
	decoded := profileadmissionsqlite.CanonicalProfileAdmission{}
	err := json.Unmarshal([]byte(`{}`), &decoded)
	if err != nil {
		t.Fatalf("generic JSON decoder returned an unexpected syntax error: %v", err)
	}
	if decoded.Valid() {
		t.Fatal("generic JSON decoding minted a canonical token")
	}
}

func TestAdmissionResultZeroValueExposesNoVariant(t *testing.T) {
	result := profileadmissionsqlite.AdmissionResult{}
	if result.Kind() != "" {
		t.Fatalf("zero result kind = %q, want empty", result.Kind())
	}
	if _, ok := result.Admission(); ok {
		t.Fatal("zero result exposed an admitted variant")
	}
	if _, ok := result.Denials(); ok {
		t.Fatal("zero result exposed a denial variant")
	}
	if _, ok := result.Failure(); ok {
		t.Fatal("zero result exposed a failure variant")
	}
}
