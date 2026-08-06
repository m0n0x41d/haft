package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestSpecSectionDraftContractActionPublishesCanonicalValidationCall(
	t *testing.T,
) {
	haftDir := filepath.Join(t.TempDir(), ".haft")
	raw, _, err := handleHaftSpecSectionWithProjectionRef(
		t.Context(),
		haftDir,
		map[string]any{"action": "draft_contract"},
	)
	if err != nil {
		t.Fatalf("draft_contract: %v", err)
	}
	contract := specflow.DraftContract{}
	if err := json.Unmarshal([]byte(raw), &contract); err != nil {
		t.Fatalf("decode draft contract: %v\n%s", err, raw)
	}
	if contract.ValidationCall.Tool != "haft_query" ||
		contract.ValidationCall.Arguments["action"] != "spec_validate" {
		t.Fatalf("validation call = %#v", contract.ValidationCall)
	}
}
