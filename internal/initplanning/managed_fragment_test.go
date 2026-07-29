package initplanning

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

const managedFragmentMergeEdition = "semantic-merge-v1"

func TestManagedJSONObjectEntryFirstInstallPreservesUnrelatedValues(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	desired := mustJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"/opt/haft","args":["serve"]}`,
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{
		  "theme": "dark",
		  "mcpServers": {
		    "context7": {
		      "command": "context7"
		    }
		  }
		}`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	if len(currentness.States()) != 0 {
		t.Fatalf("first-install states = %+v, want no owned/foreign state", currentness.States())
	}
	if got := currentness.VacantTargets(); len(got) != 1 ||
		got[0].Coordinate().Selector() != "/mcpServers/haft" {
		t.Fatalf("first-install vacant targets = %+v", got)
	}

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	if plan.Readiness() != ManagedCarrierReady {
		t.Fatalf("plan readiness = %s, want ready", plan.Readiness())
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	if !result.Changed() || result.Kind() != ManagedCarrierWrite {
		t.Fatalf("result = %s changed=%v, want write/true", result.Kind(), result.Changed())
	}

	document := decodeJSONObjectForTest(t, result.Content())
	if document["theme"] != "dark" {
		t.Fatalf("unrelated theme = %#v, want dark", document["theme"])
	}
	servers := objectFieldForTest(t, document, "mcpServers")
	context7 := objectFieldForTest(t, servers, "context7")
	if context7["command"] != "context7" {
		t.Fatalf("unrelated MCP server changed: %#v", context7)
	}
	haft := objectFieldForTest(t, servers, "haft")
	if haft["command"] != "/opt/haft" {
		t.Fatalf("managed Haft entry = %#v", haft)
	}

	manifestBasis := mustManifestOwnershipBasis(t)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{desired.Record()},
		manifestBasis,
	)
	postInput := mustPresentManagedCarrierBytes(
		t,
		carrierPath,
		result.Content(),
	)
	post := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		postInput,
	)
	assertSingleManagedState(t, post, ManagedFragmentCurrentOwned)
}

func TestManagedFragmentMatchingBytesWithoutReceiptRemainForeign(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	desired := mustJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"mcpServers":{"haft":{"args":["serve"],"command":"haft"}}}`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	assertSingleManagedState(t, currentness, ManagedFragmentForeign)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	if plan.Readiness() != ManagedCarrierBlocked || len(plan.Conflicts()) != 1 {
		t.Fatalf(
			"plan readiness=%s conflicts=%d, want blocked/1",
			plan.Readiness(),
			len(plan.Conflicts()),
		)
	}
	if _, err := ApplyManagedCarrierReconciliation(plan, input); err == nil {
		t.Fatal("blocked fragment plan was applied")
	}
}

func TestKnownLegacyManagedFragmentRecordUsesExactObservedDigest(
	t *testing.T,
) {
	template := mustTOMLTableSetFragment(
		t,
		t.TempDir()+"/config.toml",
		"mcp_servers.haft",
		[]string{"mcp_servers.haft", "mcp_servers.haft.env"},
		`[mcp_servers.haft]
command = "haft"

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
`,
	)
	legacy := []byte(`[mcp_servers.haft]
command = "haft"
env = { HAFT_PROJECT_ROOT = "." }
`)
	record, err := NewKnownLegacyManagedFragmentRecord(
		template,
		legacy,
	)
	if err != nil {
		t.Fatalf("NewKnownLegacyManagedFragmentRecord: %v", err)
	}
	if !reflect.DeepEqual(record.Coordinate(), template.Coordinate()) {
		t.Fatalf(
			"legacy coordinate = %+v, want %+v",
			record.Coordinate(),
			template.Coordinate(),
		)
	}
	if record.Component() != template.Component() {
		t.Fatalf(
			"legacy component = %s, want %s",
			record.Component(),
			template.Component(),
		)
	}
	if record.Digest() != managedFragmentDigest(legacy) ||
		record.Digest() == template.Digest() {
		t.Fatalf(
			"legacy digest = %s, template = %s",
			record.Digest(),
			template.Digest(),
		)
	}
	if _, err := NewKnownLegacyManagedFragmentRecord(
		template,
		nil,
	); err == nil {
		t.Fatal("empty known-legacy content was accepted")
	}
}

func TestManagedFragmentManifestScopesCurrentnessToFragment(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	desired := mustJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{desired.Record()},
		mustManifestOwnershipBasis(t),
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{
		  "theme": "operator-edited-after-install",
		  "mcpServers": {
		    "haft": {
		      "args": ["serve"],
		      "command": "haft"
		    }
		  }
		}`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	assertSingleManagedState(t, currentness, ManagedFragmentCurrentOwned)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	if result.Changed() || result.Kind() != ManagedCarrierUnchanged {
		t.Fatalf("current fragment result = %s changed=%v", result.Kind(), result.Changed())
	}
	if !bytes.Equal(result.Content(), input.Content()) {
		t.Fatal("current fragment reconciliation rewrote unrelated carrier bytes")
	}
}

func TestManagedFragmentLocallyModifiedOwnedBlocksReplacement(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	installed := mustJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft-v1","args":["serve"]}`,
	)
	desired := mustJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft-v2","args":["serve"]}`,
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{installed.Record()},
		mustManifestOwnershipBasis(t),
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"mcpServers":{"haft":{"command":"operator-custom","args":["serve"]}}}`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	assertSingleManagedState(t, currentness, ManagedFragmentLocallyModifiedOwned)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	if plan.Readiness() != ManagedCarrierBlocked {
		t.Fatalf("locally modified fragment plan = %s, want blocked", plan.Readiness())
	}
}

func TestManagedTOMLTableFamilyUpdatePreservesOtherSectionsByteExact(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.toml"
	installed := mustTOMLTableFamilyFragment(
		t,
		carrierPath,
		"mcp_servers.haft",
		`[mcp_servers.haft]
command = "haft-v1"
args = ["serve"]

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
`,
	)
	desired := mustTOMLTableFamilyFragment(
		t,
		carrierPath,
		"mcp_servers.haft",
		`[mcp_servers.haft]
command = "haft-v2"
args = ["serve"]

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
HAFT_EXPECTED_PROJECT_ID = "qnt_e3149c17"
`,
	)
	unrelated := `[ui]
theme = "dark"

[mcp_servers.context7]
command = "context7"
`
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		unrelated+"\n"+string(installed.Content()),
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{installed.Record()},
		mustManifestOwnershipBasis(t),
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	assertSingleManagedState(t, currentness, ManagedFragmentOutdatedOwned)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	output := string(result.Content())
	if !strings.HasPrefix(output, unrelated) {
		t.Fatalf("unrelated TOML prefix changed:\n%s", output)
	}
	if strings.Contains(output, `command = "haft-v1"`) {
		t.Fatalf("old managed TOML family survived:\n%s", output)
	}
	if strings.Count(output, "[mcp_servers.haft]") != 1 ||
		!strings.Contains(output, `command = "haft-v2"`) {
		t.Fatalf("desired managed TOML family missing or repeated:\n%s", output)
	}
}

func TestManagedTOMLTableSetMigratesFamilyAndPreservesDescendants(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.toml"
	installed := mustTOMLTableFamilyFragment(
		t,
		carrierPath,
		"mcp_servers.haft",
		`[mcp_servers.haft]
command = "haft-v1"
args = ["serve"]

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
`,
	)
	desired := mustTOMLTableSetFragment(
		t,
		carrierPath,
		"mcp_servers.haft",
		[]string{
			"mcp_servers.haft",
			"mcp_servers.haft.env",
		},
		`[mcp_servers.haft]
command = "haft-v2"
args = ["serve"]

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
HAFT_EXPECTED_PROJECT_ID = "qnt_e3149c17"
`,
	)
	userOwned := `[mcp_servers.haft.tools.haft_query]
approval_mode = "approve"

[mcp_servers.haft.tools.haft_method]
approval_mode = "approve"
`
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		string(installed.Content())+"\n"+userOwned,
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{installed.Record()},
		mustManifestOwnershipBasis(t),
	)
	legacy := mustManagedFragmentLegacyRegistry(
		t,
		[]ManagedFragmentRecord{desired.Record()},
		mustLegacyOwnershipBasis(t),
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		baseline,
		legacy,
		input,
	)
	assertSingleManagedState(t, currentness, ManagedFragmentOutdatedOwned)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	output := string(result.Content())
	if strings.Contains(output, `command = "haft-v1"`) ||
		!strings.Contains(output, `command = "haft-v2"`) {
		t.Fatalf("managed exact tables were not updated:\n%s", output)
	}
	if !strings.HasSuffix(output, userOwned) {
		t.Fatalf("user-owned descendant tables changed:\n%s", output)
	}

	firstInstallInput := mustPresentManagedCarrier(
		t,
		carrierPath,
		string(desired.Content())+"\n"+userOwned,
	)
	firstInstallCurrentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		legacy,
		firstInstallInput,
	)
	assertSingleManagedState(
		t,
		firstInstallCurrentness,
		ManagedFragmentKnownLegacyExact,
	)
}

func TestManagedJSONArrayMemberReplacesExactOwnedMemberOnly(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	installed := mustJSONArrayMemberFragment(
		t,
		carrierPath,
		[]string{"packages"},
		"haft-pi-package",
		`"./.haft/pi/haft-pi"`,
	)
	desired := mustJSONArrayMemberFragment(
		t,
		carrierPath,
		[]string{"packages"},
		"haft-pi-package",
		`"../.haft/pi/haft-pi"`,
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"packages":["operator-package","./.haft/pi/haft-pi"],"theme":"dark"}`,
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{installed.Record()},
		mustManifestOwnershipBasis(t),
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	assertSingleManagedState(t, currentness, ManagedFragmentOutdatedOwned)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	document := decodeJSONObjectForTest(t, result.Content())
	packages, ok := document["packages"].([]any)
	if !ok {
		t.Fatalf("packages = %#v, want array", document["packages"])
	}
	wantPackages := []any{"operator-package", "../.haft/pi/haft-pi"}
	if !reflect.DeepEqual(packages, wantPackages) {
		t.Fatalf("packages = %#v, want %#v", packages, wantPackages)
	}
	if document["theme"] != "dark" {
		t.Fatalf("unrelated theme changed: %#v", document["theme"])
	}
}

func TestManagedJSONArrayMemberRecognizesSeveralExactLegacyDigests(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	oldLegacy := mustJSONArrayMemberFragment(
		t,
		carrierPath,
		[]string{"packages"},
		"haft-pi-package",
		`"./.haft/pi/haft-pi"`,
	)
	currentLegacy := mustJSONArrayMemberFragment(
		t,
		carrierPath,
		[]string{"packages"},
		"haft-pi-package",
		`"../.haft/pi/haft-pi"`,
	)
	desired := mustJSONArrayMemberFragment(
		t,
		carrierPath,
		[]string{"packages"},
		"haft-pi-package",
		`{"source":"../.haft/pi/haft-pi","skills":["h-reason"]}`,
	)
	legacy := mustManagedFragmentLegacyRegistry(
		t,
		[]ManagedFragmentRecord{
			oldLegacy.Record(),
			currentLegacy.Record(),
			desired.Record(),
		},
		mustLegacyOwnershipBasis(t),
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"packages":["operator-package","./.haft/pi/haft-pi"],"theme":"dark"}`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		legacy,
		input,
	)
	assertSingleManagedState(
		t,
		currentness,
		ManagedFragmentKnownLegacyExact,
	)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	document := decodeJSONObjectForTest(t, result.Content())
	packages, ok := document["packages"].([]any)
	if !ok {
		t.Fatalf("packages = %#v, want array", document["packages"])
	}
	wantPackages := []any{
		"operator-package",
		map[string]any{
			"source": "../.haft/pi/haft-pi",
			"skills": []any{"h-reason"},
		},
	}
	if !reflect.DeepEqual(packages, wantPackages) {
		t.Fatalf("packages = %#v, want %#v", packages, wantPackages)
	}

	ambiguous := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"packages":["./.haft/pi/haft-pi","../.haft/pi/haft-pi"]}`,
	)
	observationPlan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		legacy,
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	if _, err := ObserveManagedCarrier(
		observationPlan,
		ambiguous,
	); err == nil || !strings.Contains(err.Error(), "is ambiguous") {
		t.Fatalf("ambiguous legacy observation error = %v", err)
	}
}

func TestManagedJSONArraySourceMemberPreservesUserOwnedObjectFields(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	newSource, err := NewJSONArrayMemberFragment(
		carrierPath,
		ComponentPackage,
		[]string{"packages"},
		"haft-pi-package",
		[]byte(`"../.haft/pi/haft-pi"`),
		fs.FileMode(0o644),
		ManagedJSONArraySourceMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONArrayMemberFragment(new): %v", err)
	}
	oldSource, err := NewJSONArrayMemberFragment(
		carrierPath,
		ComponentPackage,
		[]string{"packages"},
		"haft-pi-package",
		[]byte(`"./.haft/pi/haft-pi"`),
		fs.FileMode(0o644),
		ManagedJSONArraySourceMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONArrayMemberFragment(old): %v", err)
	}
	legacy := mustManagedFragmentLegacyRegistry(
		t,
		[]ManagedFragmentRecord{
			newSource.Record(),
			oldSource.Record(),
		},
		mustLegacyOwnershipBasis(t),
	)
	currentObject := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"packages":[{"source":"../.haft/pi/haft-pi","skills":["h-reason"],"prompts":false}],"theme":"dark"}`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{newSource},
		NoPriorManagedFragmentBaseline(),
		legacy,
		currentObject,
	)
	assertSingleManagedState(
		t,
		currentness,
		ManagedFragmentKnownLegacyExact,
	)
	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation(current): %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(
		plan,
		currentObject,
	)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation(current): %v", err)
	}
	if result.Changed() ||
		!bytes.Equal(result.Content(), currentObject.Content()) {
		t.Fatal("current object-form source rewrote user-owned filters")
	}

	changedFilters := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"packages":[{"source":"../.haft/pi/haft-pi","skills":[],"prompts":true}],"theme":"dark"}`,
	)
	manifest := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{newSource.Record()},
		mustManifestOwnershipBasis(t),
	)
	filterCurrentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{newSource},
		manifest,
		NoManagedFragmentLegacyRegistry(),
		changedFilters,
	)
	assertSingleManagedState(
		t,
		filterCurrentness,
		ManagedFragmentCurrentOwned,
	)

	oldObject := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"packages":[{"source":"./.haft/pi/haft-pi","skills":["h-reason"],"prompts":false}],"theme":"dark"}`,
	)
	oldCurrentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{newSource},
		NoPriorManagedFragmentBaseline(),
		legacy,
		oldObject,
	)
	assertSingleManagedState(
		t,
		oldCurrentness,
		ManagedFragmentKnownLegacyExact,
	)
	oldPlan, err := CompileManagedCarrierReconciliation(oldCurrentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation(old): %v", err)
	}
	migrated, err := ApplyManagedCarrierReconciliation(
		oldPlan,
		oldObject,
	)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation(old): %v", err)
	}
	document := decodeJSONObjectForTest(t, migrated.Content())
	packages, ok := document["packages"].([]any)
	if !ok || len(packages) != 1 {
		t.Fatalf("packages = %#v, want one object", document["packages"])
	}
	object, ok := packages[0].(map[string]any)
	if !ok ||
		object["source"] != "../.haft/pi/haft-pi" ||
		object["prompts"] != false ||
		!reflect.DeepEqual(object["skills"], []any{"h-reason"}) {
		t.Fatalf("migrated package object = %#v", packages[0])
	}
}

func TestManagedJSONArraySourceMemberRejectsAmbiguousIdentity(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	newSource, err := NewJSONArrayMemberFragment(
		carrierPath,
		ComponentPackage,
		[]string{"packages"},
		"haft-pi-package",
		[]byte(`"../.haft/pi/haft-pi"`),
		fs.FileMode(0o644),
		ManagedJSONArraySourceMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONArrayMemberFragment(new): %v", err)
	}
	oldSource, err := NewJSONArrayMemberFragment(
		carrierPath,
		ComponentPackage,
		[]string{"packages"},
		"haft-pi-package",
		[]byte(`"./.haft/pi/haft-pi"`),
		fs.FileMode(0o644),
		ManagedJSONArraySourceMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONArrayMemberFragment(old): %v", err)
	}
	legacy := mustManagedFragmentLegacyRegistry(
		t,
		[]ManagedFragmentRecord{
			newSource.Record(),
			oldSource.Record(),
		},
		mustLegacyOwnershipBasis(t),
	)
	plan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{newSource},
		NoPriorManagedFragmentBaseline(),
		legacy,
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"packages":["../.haft/pi/haft-pi",{"source":"./.haft/pi/haft-pi","skills":[]}]}`,
	)
	if _, err := ObserveManagedCarrier(
		plan,
		input,
	); err == nil || !strings.Contains(
		err.Error(),
		"source member /packages/haft-pi-package is ambiguous",
	) {
		t.Fatalf("ambiguous source observation error = %v", err)
	}
}

func TestManagedFragmentExactLegacyCanBeAdoptedWithoutCarrierRewrite(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	desired := mustJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
	)
	legacy := mustManagedFragmentLegacyRegistry(
		t,
		[]ManagedFragmentRecord{desired.Record()},
		mustLegacyOwnershipBasis(t),
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"mcpServers":{"haft":{"args":["serve"],"command":"haft"}},"theme":"dark"}`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		legacy,
		input,
	)
	assertSingleManagedState(t, currentness, ManagedFragmentKnownLegacyExact)

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	if result.Changed() || !bytes.Equal(result.Content(), input.Content()) {
		t.Fatal("exact legacy adoption rewrote the shared carrier")
	}
}

func TestManagedFragmentObservationFailsClosedOnAmbiguousCarrier(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/settings.json"
	desired := mustJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
	)
	plan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	duplicateJSON := mustPresentManagedCarrier(
		t,
		carrierPath,
		`{"mcpServers":{"haft":{"command":"one"},"haft":{"command":"two"}}}`,
	)
	if _, err := ObserveManagedCarrier(plan, duplicateJSON); err == nil ||
		!strings.Contains(err.Error(), "duplicate JSON object key") {
		t.Fatalf("duplicate JSON observation error = %v", err)
	}

	tomlPath := t.TempDir() + "/config.toml"
	toml := mustTOMLTableFamilyFragment(
		t,
		tomlPath,
		"mcp_servers.haft",
		"[mcp_servers.haft]\ncommand = \"haft\"\n",
	)
	tomlPlan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{toml},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan(TOML): %v", err)
	}
	unsupportedTOML := mustPresentManagedCarrier(
		t,
		tomlPath,
		"[\"mcp_servers\".haft]\ncommand = \"haft\"\n",
	)
	if _, err := ObserveManagedCarrier(tomlPlan, unsupportedTOML); err == nil ||
		!strings.Contains(err.Error(), "unsupported TOML table header") {
		t.Fatalf("unsupported TOML observation error = %v", err)
	}
}

func TestManagedYAMLFragmentsFirstInstallPreservesUnrelatedCarrierBytes(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.yaml"
	server := mustYAMLMappingEntryFragment(
		t,
		carrierPath,
		[]string{"mcp_servers", "haft"},
		`command: /opt/haft/bin/haft
args:
  - serve
env:
  HAFT_PROJECT_ROOT: /work/project
  HAFT_EXPECTED_PROJECT_ID: qnt_e3149c17
enabled: true
`,
	)
	skills := mustYAMLSequenceMemberFragment(
		t,
		carrierPath,
		[]string{"skills", "external_dirs"},
		"haft-skills-root",
		`/opt/haft/hermes/skills`,
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`# operator header
theme: dark
skills:
  external_dirs:
    - /foreign/skills
mcp_servers:
  other:
    command: other-server
# operator footer
telemetry:
  enabled: false
`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{server, skills},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	if got := currentness.VacantTargets(); len(got) != 2 {
		t.Fatalf("first-install vacant YAML targets = %+v, want 2", got)
	}

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	output := string(result.Content())
	unrelated := []string{
		"# operator header\ntheme: dark\n",
		"    - /foreign/skills\n",
		"  other:\n    command: other-server\n",
		"# operator footer\ntelemetry:\n  enabled: false\n",
	}
	for _, exact := range unrelated {
		if !strings.Contains(output, exact) {
			t.Fatalf("unrelated YAML bytes %q changed:\n%s", exact, output)
		}
	}
	if !strings.Contains(output, "  haft:\n") ||
		!strings.Contains(output, "    command: /opt/haft/bin/haft\n") ||
		!strings.Contains(output, "    - /opt/haft/hermes/skills\n") {
		t.Fatalf("managed YAML fragments are missing:\n%s", output)
	}

	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{server.Record(), skills.Record()},
		mustManifestOwnershipBasis(t),
	)
	post := inspectManagedCarrier(
		t,
		[]ManagedFragment{server, skills},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		mustPresentManagedCarrierBytes(t, carrierPath, result.Content()),
	)
	states := post.States()
	if len(states) != 2 {
		t.Fatalf("post-install YAML states = %+v, want 2", states)
	}
	for _, state := range states {
		if state.Kind() != ManagedFragmentCurrentOwned {
			t.Fatalf("post-install YAML state = %s, want current_owned", state.Kind())
		}
	}
}

func TestManagedYAMLFragmentsFirstInstallPreservesCommentOnlyCarrier(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.yaml"
	server := mustYAMLMappingEntryFragment(
		t,
		carrierPath,
		[]string{"mcp_servers", "haft"},
		"command: haft\nargs:\n  - serve\n",
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		"# operator owns this carrier\n# keep both lines exactly\n",
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{server},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	if len(currentness.States()) != 0 ||
		len(currentness.VacantTargets()) != 1 {
		t.Fatalf(
			"comment-only YAML currentness states=%d vacant=%d, want 0/1",
			len(currentness.States()),
			len(currentness.VacantTargets()),
		)
	}

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	output := string(result.Content())
	if !strings.HasPrefix(
		output,
		"# operator owns this carrier\n# keep both lines exactly\n",
	) {
		t.Fatalf("comment-only YAML carrier bytes changed:\n%s", output)
	}
	if !strings.Contains(output, "mcp_servers:\n  haft:\n") {
		t.Fatalf("managed YAML fragment is missing:\n%s", output)
	}
}

func TestManagedYAMLCommentInsideOwnedFragmentIsLocalModification(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.yaml"
	server := mustYAMLMappingEntryFragment(
		t,
		carrierPath,
		[]string{"mcp_servers", "haft"},
		"command: haft\nargs:\n  - serve\n",
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{server.Record()},
		mustManifestOwnershipBasis(t),
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`mcp_servers:
  haft:
    # operator changed the owned fragment
    command: haft
    args:
      - serve
`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{server},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	assertSingleManagedState(
		t,
		currentness,
		ManagedFragmentLocallyModifiedOwned,
	)
	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	if plan.Readiness() != ManagedCarrierBlocked ||
		len(plan.Conflicts()) != 1 {
		t.Fatalf(
			"comment-modified YAML plan = %s conflicts=%d, want blocked/1",
			plan.Readiness(),
			len(plan.Conflicts()),
		)
	}
}

func TestManagedYAMLFragmentsReplaceOnlyExactOwnedCoordinates(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.yaml"
	installedServer := mustYAMLMappingEntryFragment(
		t,
		carrierPath,
		[]string{"mcp_servers", "haft"},
		"command: haft-v1\nargs:\n  - serve\nenabled: true\n",
	)
	desiredServer := mustYAMLMappingEntryFragment(
		t,
		carrierPath,
		[]string{"mcp_servers", "haft"},
		"command: haft-v2\nargs:\n  - serve\nenabled: true\n",
	)
	installedSkills := mustYAMLSequenceMemberFragment(
		t,
		carrierPath,
		[]string{"skills", "external_dirs"},
		"haft-skills-root",
		`/old/haft/skills`,
	)
	desiredSkills := mustYAMLSequenceMemberFragment(
		t,
		carrierPath,
		[]string{"skills", "external_dirs"},
		"haft-skills-root",
		`/new/haft/skills`,
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`# preserve exactly
theme: dark
skills:
  external_dirs:
    - /foreign/skills
    - /old/haft/skills
mcp_servers:
  other:
    command: other-server
  haft:
    command: haft-v1
    args:
      - serve
    enabled: true
telemetry: false
`,
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{
			installedServer.Record(),
			installedSkills.Record(),
		},
		mustManifestOwnershipBasis(t),
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desiredServer, desiredSkills},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	states := currentness.States()
	if len(states) != 2 {
		t.Fatalf("pre-update YAML states = %+v, want 2", states)
	}
	for _, state := range states {
		if state.Kind() != ManagedFragmentOutdatedOwned {
			t.Fatalf("pre-update YAML state = %s, want outdated_owned", state.Kind())
		}
	}

	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	output := string(result.Content())
	if !strings.Contains(output, "# preserve exactly\ntheme: dark\n") ||
		!strings.Contains(output, "    - /foreign/skills\n") ||
		!strings.Contains(output, "  other:\n    command: other-server\n") ||
		!strings.HasSuffix(output, "telemetry: false\n") {
		t.Fatalf("unrelated YAML bytes changed:\n%s", output)
	}
	if strings.Contains(output, "haft-v1") ||
		strings.Contains(output, "/old/haft/skills") ||
		strings.Count(output, "haft-v2") != 1 ||
		strings.Count(output, "/new/haft/skills") != 1 {
		t.Fatalf("owned YAML fragments were not replaced exactly:\n%s", output)
	}
}

func TestManagedYAMLMatchingValuesWithoutReceiptRemainForeign(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.yaml"
	server := mustYAMLMappingEntryFragment(
		t,
		carrierPath,
		[]string{"mcp_servers", "haft"},
		"command: haft\nargs:\n  - serve\nenabled: true\n",
	)
	skills := mustYAMLSequenceMemberFragment(
		t,
		carrierPath,
		[]string{"skills", "external_dirs"},
		"haft-skills-root",
		`/opt/haft/skills`,
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		`skills:
  external_dirs:
    - /opt/haft/skills
mcp_servers:
  haft:
    enabled: true
    args: [serve]
    command: haft
`,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{server, skills},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	states := currentness.States()
	if len(states) != 2 {
		t.Fatalf("matching unowned YAML states = %+v, want 2", states)
	}
	for _, state := range states {
		if state.Kind() != ManagedFragmentForeign {
			t.Fatalf("matching unowned YAML state = %s, want foreign", state.Kind())
		}
	}
	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	if plan.Readiness() != ManagedCarrierBlocked ||
		len(plan.Conflicts()) != 2 {
		t.Fatalf(
			"matching unowned YAML plan = %s conflicts=%d, want blocked/2",
			plan.Readiness(),
			len(plan.Conflicts()),
		)
	}
}

func TestManagedYAMLObservationFailsClosedOnAmbiguousSyntax(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/config.yaml"
	server := mustYAMLMappingEntryFragment(
		t,
		carrierPath,
		[]string{"mcp_servers", "haft"},
		"command: haft\n",
	)
	plan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{server},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "duplicate key",
			content: `mcp_servers:
  haft:
    command: one
  haft:
    command: two
`,
			want: "duplicate YAML mapping key",
		},
		{
			name: "alias",
			content: `defaults: &defaults
  command: haft
mcp_servers:
  haft: *defaults
`,
			want: "YAML aliases and anchors are unsupported",
		},
		{
			name: "merge key",
			content: `defaults: &defaults
  command: haft
mcp_servers:
  haft:
    <<: *defaults
`,
			want: "YAML aliases and anchors are unsupported",
		},
		{
			name: "multiple documents",
			content: `mcp_servers:
  haft:
    command: haft
---
theme: dark
`,
			want: "exactly one document",
		},
		{
			name:    "flow managed parent",
			content: "mcp_servers: {haft: {command: haft}}\n",
			want:    "managed YAML path uses flow style",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := mustPresentManagedCarrier(
				t,
				carrierPath,
				test.content,
			)
			_, err := ObserveManagedCarrier(plan, input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ObserveManagedCarrier error = %v, want %q", err, test.want)
			}
		})
	}
}

func inspectManagedCarrier(
	t *testing.T,
	desired []ManagedFragment,
	baseline ManagedFragmentBaseline,
	legacy ManagedFragmentLegacyRegistry,
	input ManagedCarrierInput,
) ManagedCarrierCurrentness {
	t.Helper()
	plan, err := BuildManagedFragmentObservationPlan(
		desired,
		baseline,
		legacy,
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	observation, err := ObserveManagedCarrier(plan, input)
	if err != nil {
		t.Fatalf("ObserveManagedCarrier: %v", err)
	}
	currentness, err := ClassifyManagedFragmentCurrentness(
		plan,
		observation,
	)
	if err != nil {
		t.Fatalf("ClassifyManagedFragmentCurrentness: %v", err)
	}
	return currentness
}

func mustJSONObjectEntryFragment(
	t *testing.T,
	carrierPath string,
	selector []string,
	value string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewJSONObjectEntryFragment(
		carrierPath,
		ComponentMCP,
		selector,
		[]byte(value),
		fs.FileMode(0o644),
		managedFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment: %v", err)
	}
	return fragment
}

func mustJSONArrayMemberFragment(
	t *testing.T,
	carrierPath string,
	selector []string,
	memberID string,
	value string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewJSONArrayMemberFragment(
		carrierPath,
		ComponentPackage,
		selector,
		memberID,
		[]byte(value),
		fs.FileMode(0o644),
		managedFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONArrayMemberFragment: %v", err)
	}
	return fragment
}

func mustTOMLTableFamilyFragment(
	t *testing.T,
	carrierPath string,
	prefix string,
	value string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewTOMLTableFamilyFragment(
		carrierPath,
		ComponentMCP,
		prefix,
		[]byte(value),
		fs.FileMode(0o644),
		managedFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewTOMLTableFamilyFragment: %v", err)
	}
	return fragment
}

func mustTOMLTableSetFragment(
	t *testing.T,
	carrierPath string,
	prefix string,
	tables []string,
	value string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewTOMLTableSetFragment(
		carrierPath,
		ComponentMCP,
		prefix,
		tables,
		[]byte(value),
		fs.FileMode(0o644),
		managedFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewTOMLTableSetFragment: %v", err)
	}
	return fragment
}

func mustYAMLMappingEntryFragment(
	t *testing.T,
	carrierPath string,
	selector []string,
	value string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewYAMLMappingEntryFragment(
		carrierPath,
		ComponentMCP,
		selector,
		[]byte(value),
		fs.FileMode(0o644),
		managedFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewYAMLMappingEntryFragment: %v", err)
	}
	return fragment
}

func mustYAMLSequenceMemberFragment(
	t *testing.T,
	carrierPath string,
	selector []string,
	memberID string,
	value string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewYAMLSequenceMemberFragment(
		carrierPath,
		ComponentMCP,
		selector,
		memberID,
		[]byte(value),
		fs.FileMode(0o644),
		managedFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewYAMLSequenceMemberFragment: %v", err)
	}
	return fragment
}

func mustPresentManagedCarrier(
	t *testing.T,
	path string,
	content string,
) ManagedCarrierInput {
	t.Helper()
	return mustPresentManagedCarrierBytes(t, path, []byte(content))
}

func mustPresentManagedCarrierBytes(
	t *testing.T,
	path string,
	content []byte,
) ManagedCarrierInput {
	t.Helper()
	input, err := NewPresentManagedCarrier(
		path,
		content,
		fs.FileMode(0o644),
	)
	if err != nil {
		t.Fatalf("NewPresentManagedCarrier: %v", err)
	}
	return input
}

func mustManagedFragmentBaseline(
	t *testing.T,
	records []ManagedFragmentRecord,
	basis OwnershipBasis,
) ManagedFragmentBaseline {
	t.Helper()
	baseline, err := NewManagedFragmentManifestBaseline(records, basis)
	if err != nil {
		t.Fatalf("NewManagedFragmentManifestBaseline: %v", err)
	}
	return baseline
}

func mustManagedFragmentLegacyRegistry(
	t *testing.T,
	records []ManagedFragmentRecord,
	basis OwnershipBasis,
) ManagedFragmentLegacyRegistry {
	t.Helper()
	registry, err := NewManagedFragmentLegacyRegistry(records, basis)
	if err != nil {
		t.Fatalf("NewManagedFragmentLegacyRegistry: %v", err)
	}
	return registry
}

func mustManifestOwnershipBasis(t *testing.T) OwnershipBasis {
	t.Helper()
	basis, err := NewOwnershipBasis(
		OwnershipManifestReceipt,
		"host-installation-manifest:test",
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatalf("NewOwnershipBasis(manifest): %v", err)
	}
	return basis
}

func mustLegacyOwnershipBasis(t *testing.T) OwnershipBasis {
	t.Helper()
	basis, err := NewOwnershipBasis(
		OwnershipLegacyRegistry,
		"host-legacy-registry:test",
		"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	)
	if err != nil {
		t.Fatalf("NewOwnershipBasis(legacy): %v", err)
	}
	return basis
}

func assertSingleManagedState(
	t *testing.T,
	currentness ManagedCarrierCurrentness,
	want ManagedFragmentCurrentnessKind,
) {
	t.Helper()
	states := currentness.States()
	if len(states) != 1 || states[0].Kind() != want {
		t.Fatalf("managed fragment states = %+v, want one %s", states, want)
	}
}

func decodeJSONObjectForTest(
	t *testing.T,
	raw []byte,
) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode JSON object: %v\n%s", err, raw)
	}
	return result
}

func objectFieldForTest(
	t *testing.T,
	parent map[string]any,
	key string,
) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want JSON object", key, parent[key])
	}
	return value
}
