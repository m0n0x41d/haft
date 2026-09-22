package initplanning

import (
	"path/filepath"
	"testing"
)

func TestSharedFragmentReceiptRequiresExactOptedInTransition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	tables := []string{"mcp_servers.haft", "mcp_servers.haft.env"}
	previousContent := "[mcp_servers.haft]\nstartup_timeout_sec = 10\n\n[mcp_servers.haft.env]\nHAFT_PROJECT_ROOT = \".\"\n"
	desiredContent := "[mcp_servers.haft]\nstartup_timeout_sec = 20\n\n[mcp_servers.haft.env]\nHAFT_PROJECT_ROOT = \".\"\n"
	customContent := "[mcp_servers.haft]\nstartup_timeout_sec = 45\n\n[mcp_servers.haft.env]\nHAFT_PROJECT_ROOT = \".\"\n"
	nestedApproval := "\n[mcp_servers.haft.tools.haft_method]\napproval_mode = \"prompt\"\n"
	previous := mustTOMLTableSetFragment(t, path, "mcp_servers.haft", tables, previousContent)
	desired := mustTOMLTableSetFragment(t, path, "mcp_servers.haft", tables, desiredContent)
	custom := mustTOMLTableSetFragment(t, path, "mcp_servers.haft", tables, customContent)
	previousRecord := previous.Record()
	desiredRecord := desired.Record()
	customRecord := custom.Record()
	legacyBasis := mustLegacyOwnershipBasis(t)
	manifestBasis := mustManifestOwnershipBasis(t)
	tests := []struct {
		name       string
		receipt    ManagedFragmentRecord
		observed   string
		registered []ManagedFragmentRecord
		optIn      []ManagedFragmentRecord
		want       ManagedFragmentCurrentnessKind
	}{
		{"shared upgrade", previousRecord, desiredContent + nestedApproval, []ManagedFragmentRecord{previousRecord, desiredRecord}, []ManagedFragmentRecord{previousRecord}, ManagedFragmentKnownLegacyExact},
		{"no opt in", previousRecord, desiredContent, []ManagedFragmentRecord{previousRecord, desiredRecord}, nil, ManagedFragmentLocallyModifiedOwned},
		{"custom timeout", previousRecord, customContent, []ManagedFragmentRecord{previousRecord, desiredRecord}, []ManagedFragmentRecord{previousRecord}, ManagedFragmentLocallyModifiedOwned},
		{"custom predecessor", customRecord, desiredContent, []ManagedFragmentRecord{previousRecord, desiredRecord}, []ManagedFragmentRecord{previousRecord}, ManagedFragmentLocallyModifiedOwned},
		{"unregistered desired", previousRecord, desiredContent, []ManagedFragmentRecord{previousRecord}, []ManagedFragmentRecord{previousRecord}, ManagedFragmentLocallyModifiedOwned},
		{"generated downgrade", desiredRecord, previousContent, []ManagedFragmentRecord{previousRecord, desiredRecord}, []ManagedFragmentRecord{previousRecord}, ManagedFragmentLocallyModifiedOwned},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := mustManagedFragmentBaseline(t, []ManagedFragmentRecord{test.receipt}, manifestBasis)
			registry := mustManagedFragmentLegacyRegistry(t, test.registered, legacyBasis)
			registry, err := registry.WithSharedReceiptPredecessors(test.optIn)
			if err != nil {
				t.Fatal(err)
			}
			input := mustPresentManagedCarrier(t, path, test.observed)
			currentness := inspectManagedCarrier(t, []ManagedFragment{desired}, baseline, registry, input)
			assertSingleManagedState(t, currentness, test.want)
			plan, err := CompileManagedCarrierReconciliation(currentness)
			if err != nil {
				t.Fatal(err)
			}
			if test.want == ManagedFragmentLocallyModifiedOwned && plan.Readiness() != ManagedCarrierBlocked {
				t.Fatal("customized or unregistered transition did not block")
			}
			if plan.Readiness() == ManagedCarrierBlocked {
				return
			}
			result, err := ApplyManagedCarrierReconciliation(plan, input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Changed() {
				t.Fatal("receipt adoption rewrote shared carrier or nested approvals")
			}
		})
	}
	registry := mustManagedFragmentLegacyRegistry(t, []ManagedFragmentRecord{desiredRecord}, legacyBasis)
	_, err := registry.WithSharedReceiptPredecessors([]ManagedFragmentRecord{previousRecord})
	if err == nil {
		t.Fatal("accepted an unregistered receipt predecessor")
	}
}
