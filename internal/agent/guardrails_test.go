package agent

import (
	"strings"
	"testing"
)

func TestCheckREff_WarnsOnUnsubstantiatedClosure(t *testing.T) {
	err := CheckREff(0.82, 0)
	if err == nil {
		t.Fatal("expected F0 closure warning")
	}
	if !strings.Contains(err.Error(), "F_eff=F0") {
		t.Fatalf("warning = %q, want F_eff=F0 guidance", err.Error())
	}
}
