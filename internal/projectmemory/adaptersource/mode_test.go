package adaptersource

import "testing"

func TestModeKeepsCurrentAndHistoricalSourcePosturesDisjoint(t *testing.T) {
	t.Parallel()

	historical := HistoricalMembership()
	current := CurrentKindClassification()
	if err := historical.Verify(); err != nil {
		t.Fatalf("HistoricalMembership().Verify() error = %v", err)
	}
	if err := current.Verify(); err != nil {
		t.Fatalf("CurrentKindClassification().Verify() error = %v", err)
	}
	if !historical.IsHistoricalMembership() || historical.IsCurrentKindClassification() {
		t.Fatal("historical mode overlaps current classification")
	}
	if !current.IsCurrentKindClassification() || current.IsHistoricalMembership() {
		t.Fatal("current classification mode overlaps historical membership")
	}
	if err := (Mode{}).Verify(); err == nil {
		t.Fatal("zero source mode verified")
	}
}
