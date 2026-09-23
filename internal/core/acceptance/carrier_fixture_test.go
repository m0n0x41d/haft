package acceptance_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func TestOrderClaimAndTermFixtures(t *testing.T) {
	raw, err := os.ReadFile("../testdata/order/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	d := carrier.Parse(raw)
	if !d.Valid() {
		t.Fatalf("fixture invalid: %+v", d.Diagnostics)
	}
	if ds := carrier.Validate(d.Record, true); carrier.HasErrors(ds) {
		t.Fatal(ds)
	}
	terms, err := os.ReadFile("../testdata/order/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	td := carrier.ParseTerms(terms)
	if carrier.HasErrors(td.Diagnostics) {
		t.Fatal(td.Diagnostics)
	}
	if ds := carrier.ValidateTermRefs(d.Record, td.Terms); carrier.HasErrors(ds) {
		t.Fatal(ds)
	}
	if !bytes.Equal(d.Bytes(), raw) {
		t.Fatal("lossless read changed fixture bytes")
	}
	if d.Record.OperatorConfirmed || d.Record.Status != "proposed" {
		t.Fatal("fixture manufactured authority")
	}
	// Structural success does not supply any observed evidence or accepted norm.
	if len(d.Record.Uses) != 0 {
		t.Fatal("fixture contains unobserved evidence")
	}
}

func TestHistoricalInterpretationDoesNotFollowLiveTerms(t *testing.T) {
	raw, err := os.ReadFile("../testdata/order/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	terms, err := os.ReadFile("../testdata/order/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	basis := carrier.InterpretationBasis{Terms: &carrier.BasisFile{Name: "terms.md", Bytes: terms}}
	_, oldBytes, oldDigest, err := carrier.NewSnapshot(raw, basis)
	if err != nil {
		t.Fatal(err)
	}
	next := bytes.Replace(terms, []byte("new or paid"), []byte("new only"), 1)
	basis.Terms = &carrier.BasisFile{Name: "terms.md", Bytes: next}
	_, _, newDigest, err := carrier.NewSnapshot(raw, basis)
	if err != nil {
		t.Fatal(err)
	}
	if oldDigest == newDigest {
		t.Fatal("term interpretation change kept old edition")
	}
	old, err := carrier.ReadSnapshot(oldBytes, oldDigest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(old.Interpretation.Terms.Bytes, terms) {
		t.Fatal("historical terms were reinterpreted")
	}
	if !strings.Contains(string(old.Interpretation.Terms.Bytes), "new or paid") {
		t.Fatal("old term definition missing")
	}
	if _, err := carrier.ReadSnapshot(append(oldBytes, ' '), oldDigest); err == nil {
		t.Fatal("altered bytes accepted under old digest")
	}
}
