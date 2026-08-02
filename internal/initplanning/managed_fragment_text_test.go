package initplanning

import (
	"bytes"
	"strings"
	"testing"
)

const managedTextFragmentMergeEdition = "html-comment-section-merge-v1"

func TestManagedHTMLCommentSectionFirstInstallPreservesOutsideBytes(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/CLAUDE.md"
	desired := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"current instructions",
	)
	input := mustPresentManagedCarrier(
		t,
		carrierPath,
		"# Operator rules\n\nKeep this text.\n",
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
		input,
	)
	if len(currentness.States()) != 0 ||
		len(currentness.VacantTargets()) != 1 {
		t.Fatalf(
			"first-install text currentness states=%+v vacant=%+v",
			currentness.States(),
			currentness.VacantTargets(),
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
	if !result.Changed() ||
		!bytes.HasPrefix(result.Content(), input.Content()) ||
		!bytes.Contains(result.Content(), desired.Content()) {
		t.Fatalf("first-install text result = %q", result.Content())
	}

	operatorSuffix := []byte("\n## Operator suffix\n\nPreserve this too.\n")
	postBytes := append(result.Content(), operatorSuffix...)
	postInput := mustPresentManagedCarrierBytes(
		t,
		carrierPath,
		postBytes,
	)
	baseline := mustManagedFragmentBaseline(
		t,
		[]ManagedFragmentRecord{desired.Record()},
		mustManifestOwnershipBasis(t),
	)
	post := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		baseline,
		NoManagedFragmentLegacyRegistry(),
		postInput,
	)
	assertSingleManagedState(t, post, ManagedFragmentCurrentOwned)
	postPlan, err := CompileManagedCarrierReconciliation(post)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation(post): %v", err)
	}
	postResult, err := ApplyManagedCarrierReconciliation(
		postPlan,
		postInput,
	)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation(post): %v", err)
	}
	if postResult.Changed() ||
		!bytes.Equal(postResult.Content(), postBytes) {
		t.Fatal("current text fragment rewrote operator-owned bytes")
	}
}

func TestManagedHTMLCommentSectionReplacesOwnedSectionOnly(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/CLAUDE.md"
	installed := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"old instructions",
	)
	desired := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"new instructions",
	)
	prefix := []byte("# Operator prefix\n\n")
	suffix := []byte("\n\n## Operator suffix\n")
	content := make([]byte, 0)
	content = append(content, prefix...)
	content = append(content, installed.Content()...)
	content = append(content, suffix...)
	input := mustPresentManagedCarrierBytes(
		t,
		carrierPath,
		content,
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
	expected := make([]byte, 0)
	expected = append(expected, prefix...)
	expected = append(expected, desired.Content()...)
	expected = append(expected, suffix...)
	if !bytes.Equal(result.Content(), expected) {
		t.Fatalf(
			"owned text replacement changed outside bytes\n got: %q\nwant: %q",
			result.Content(),
			expected,
		)
	}
}

func TestManagedHTMLCommentSectionLocalEditIsReplacedInsideMarkersOnly(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/CLAUDE.md"
	installed := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"old instructions",
	)
	locallyEdited := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"operator changed the owned section",
	)
	desired := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"new instructions",
	)
	prefix := []byte("# Operator prefix\n\n")
	suffix := []byte("\n\n## Operator suffix\n")
	content := make([]byte, 0)
	content = append(content, prefix...)
	content = append(content, locallyEdited.Content()...)
	content = append(content, suffix...)
	input := mustPresentManagedCarrierBytes(
		t,
		carrierPath,
		content,
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
	assertSingleManagedState(
		t,
		currentness,
		ManagedFragmentOutdatedOwned,
	)
	plan, err := CompileManagedCarrierReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileManagedCarrierReconciliation: %v", err)
	}
	if plan.Readiness() != ManagedCarrierReady {
		t.Fatalf("locally edited text readiness=%s", plan.Readiness())
	}
	result, err := ApplyManagedCarrierReconciliation(plan, input)
	if err != nil {
		t.Fatalf("ApplyManagedCarrierReconciliation: %v", err)
	}
	expected := make([]byte, 0)
	expected = append(expected, prefix...)
	expected = append(expected, desired.Content()...)
	expected = append(expected, suffix...)
	if !bytes.Equal(result.Content(), expected) {
		t.Fatalf(
			"marker-owned replacement changed outside bytes\n got: %q\nwant: %q",
			result.Content(),
			expected,
		)
	}
}

func TestManagedHTMLCommentSectionPreManifestVersionIsReplaced(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/AGENTS.md"
	installed := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"instructions from an older Haft installation",
	)
	desired := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"current Haft instructions",
	)
	prefix := []byte("# Project rules\n\n")
	suffix := []byte("\n\n## Local rules\n")
	content := make([]byte, 0)
	content = append(content, prefix...)
	content = append(content, installed.Content()...)
	content = append(content, suffix...)
	input := mustPresentManagedCarrierBytes(t, carrierPath, content)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
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
	expected := make([]byte, 0)
	expected = append(expected, prefix...)
	expected = append(expected, desired.Content()...)
	expected = append(expected, suffix...)
	if !bytes.Equal(result.Content(), expected) {
		t.Fatalf(
			"pre-manifest replacement changed outside bytes\n got: %q\nwant: %q",
			result.Content(),
			expected,
		)
	}
}

func TestManagedHTMLCommentSectionExactPreManifestVersionIsCurrent(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/CLAUDE.md"
	desired := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"legacy exact instructions",
	)
	legacy := mustManagedFragmentLegacyRegistry(
		t,
		[]ManagedFragmentRecord{desired.Record()},
		mustLegacyOwnershipBasis(t),
	)
	content := []byte(
		"# Operator\n\n" +
			string(desired.Content()) +
			"\n\n## More operator text\n",
	)
	input := mustPresentManagedCarrierBytes(
		t,
		carrierPath,
		content,
	)
	currentness := inspectManagedCarrier(
		t,
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		legacy,
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
	if result.Changed() || !bytes.Equal(result.Content(), content) {
		t.Fatal("exact legacy text adoption rewrote the carrier")
	}
}

func TestManagedHTMLCommentSectionFailsClosedOnMalformedMarkers(
	t *testing.T,
) {
	carrierPath := t.TempDir() + "/CLAUDE.md"
	desired := mustHTMLCommentSectionFragment(
		t,
		carrierPath,
		ComponentInstructions,
		"instructions",
	)
	plan, err := BuildManagedFragmentObservationPlan(
		[]ManagedFragment{desired},
		NoPriorManagedFragmentBaseline(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	start, end := htmlCommentSectionMarkers("haft")
	for name, raw := range map[string]string{
		"missing end":      start + "\nbody\n",
		"end before start": end + "\n" + start,
		"duplicate start":  start + "\n" + start + "\n" + end,
	} {
		t.Run(name, func(t *testing.T) {
			input := mustPresentManagedCarrier(
				t,
				carrierPath,
				raw,
			)
			if _, err := ObserveManagedCarrier(plan, input); err == nil {
				t.Fatalf("malformed markers were admitted: %q", raw)
			}
		})
	}
}

func TestManagedHTMLCommentSectionManifestRoundTrip(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fragment := mustHTMLCommentSectionFragment(
		t,
		root+"/CLAUDE.md",
		ComponentMCP,
		"manifest instructions",
	)
	builder := baseManagedProjectionBuilder(t, root)
	builder = builder.AddManagedFragment(fragment)
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterProjectionBuilder.Build: %v", err)
	}
	manifest, err := BuildProjectionInstallationManifest(projection)
	if err != nil {
		t.Fatalf("BuildProjectionInstallationManifest: %v", err)
	}
	parsed, err := ParseInstallationManifest(manifest.CanonicalBytes())
	if err != nil {
		t.Fatalf("ParseInstallationManifest: %v", err)
	}
	records, err := parsed.ManagedFragmentRecords()
	if err != nil {
		t.Fatalf("ManagedFragmentRecords: %v", err)
	}
	if len(records) != 1 ||
		records[0].Coordinate().Kind() != ManagedHTMLCommentSection ||
		records[0].Coordinate().Selector() != "haft" {
		t.Fatalf("parsed text fragment records = %+v", records)
	}
}

func mustHTMLCommentSectionFragment(
	t *testing.T,
	carrierPath string,
	component Component,
	body string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewHTMLCommentSectionFragment(
		carrierPath,
		component,
		"haft",
		[]byte(strings.TrimSpace(body)),
		0o644,
		managedTextFragmentMergeEdition,
	)
	if err != nil {
		t.Fatalf("NewHTMLCommentSectionFragment: %v", err)
	}
	return fragment
}
