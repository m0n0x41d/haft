package decisionrecordadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type decisionRecordReaderStub struct {
	record *artifact.Artifact
	err    error
}

func (reader decisionRecordReaderStub) Get(
	_ context.Context,
	_ string,
) (*artifact.Artifact, error) {
	return reader.record, reader.err
}

func TestLoadExistingDecisionChoiceSourceRequiresStoredChoice(t *testing.T) {
	record := decisionProjectionTestArtifact(t)
	source, err := LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{record: record},
		record.Meta.ID,
	)
	if err != nil {
		t.Fatalf("LoadExistingDecisionChoiceSource: %v", err)
	}
	if source.DecisionRecordRef() != record.Meta.ID ||
		source.Title() != record.Meta.Title ||
		source.SourceDigest().String() == "" ||
		len(source.CanonicalBytes()) == 0 {
		t.Fatal("loaded DecisionRecord projection source is incomplete")
	}

	withoutChoice := *record
	fields := withoutChoice.UnmarshalDecisionFields()
	fields.ChoiceResult = nil
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal no-choice fixture: %v", err)
	}
	withoutChoice.StructuredData = string(encoded)
	_, err = LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{record: &withoutChoice},
		withoutChoice.Meta.ID,
	)
	if err == nil {
		t.Fatal("loader projected a recommendation-like record without ChoiceResult")
	}
}

func TestLoadExistingDecisionChoiceSourceRejectsHistoricalAndMismatchedIdentity(
	t *testing.T,
) {
	record := decisionProjectionTestArtifact(t)
	historical := *record
	historical.Meta.Status = artifact.StatusSuperseded
	_, err := LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{record: &historical},
		historical.Meta.ID,
	)
	if err == nil {
		t.Fatal("loader projected a superseded DecisionRecord as current choice")
	}

	mismatched := *record
	fields := mismatched.UnmarshalDecisionFields()
	fields.SelectedTitle = "a different selected title"
	encoded, marshalErr := json.Marshal(fields)
	if marshalErr != nil {
		t.Fatalf("marshal mismatched fixture: %v", marshalErr)
	}
	mismatched.StructuredData = string(encoded)
	_, err = LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{record: &mismatched},
		mismatched.Meta.ID,
	)
	if err == nil {
		t.Fatal("loader accepted mismatched DecisionRecord identity")
	}
}

func TestLoadExistingDecisionChoiceSourcePreservesIndependentLegacyChoiceFields(
	t *testing.T,
) {
	record := decisionProjectionTestArtifact(t)
	fields := record.UnmarshalDecisionFields()
	fields.SelectionPolicy = "apply the detailed maximin policy"
	fields.WhySelected = "long-form decision rationale"
	fields.ProblemRefs = []string{"prob-20260718-other-12345678"}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal legacy independent-rule fixture: %v", err)
	}
	record.StructuredData = string(encoded)

	source, err := LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{record: record},
		record.Meta.ID,
	)
	if err != nil {
		t.Fatalf("LoadExistingDecisionChoiceSource: %v", err)
	}
	if source.ChoiceFieldCorrelationMode() !=
		choiceFieldCorrelationLegacyIndependent {
		t.Fatalf(
			"choice correlation mode = %q, want legacy-independent",
			source.ChoiceFieldCorrelationMode(),
		)
	}
	if len(source.ChoiceFieldWarnings()) != 3 {
		t.Fatalf(
			"choice correlation warnings = %#v, want three",
			source.ChoiceFieldWarnings(),
		)
	}
	if !source.valid() {
		t.Fatal("legacy independent ChoiceResult source is not valid")
	}
	if !bytes.Contains(
		source.CanonicalBytes(),
		[]byte(`"choice_rule":"prefer the reversible option"`),
	) {
		t.Fatal("projection source did not preserve the explicit ChoiceResult")
	}
}

func TestLoadExistingDecisionChoiceSourcePropagatesStoreFailure(t *testing.T) {
	_, err := LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{err: fmt.Errorf("store unavailable")},
		"dec-20260718-test-12345678",
	)
	if err == nil {
		t.Fatal("loader hid project-store failure")
	}
}

func TestLegacyContextProjectionDoesNotCollapseCarrierTextIntoTypedContext(
	t *testing.T,
) {
	record := decisionProjectionTestArtifact(t)
	record.Meta.Context = "architecture choice for Haft v9 typed memory"
	source, err := LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{record: record},
		record.Meta.ID,
	)
	if err != nil {
		t.Fatalf("LoadExistingDecisionChoiceSource: %v", err)
	}
	target, err := typedmemory.NewBoundedContextRef("haft-project")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	projection, err := NewLegacyContextProjection(source, target)
	if err != nil {
		t.Fatalf("NewLegacyContextProjection: %v", err)
	}
	if projection.SourceContext() == projection.TargetContext().String() {
		t.Fatal("fixture did not exercise distinct legacy and typed contexts")
	}
	if !projection.validFor(source, target) {
		t.Fatal("exact source-bound legacy context projection is invalid")
	}
	if !bytes.Contains(
		projection.CanonicalBytes(),
		[]byte("legacy_artifact_context_is_preserved_source_description"),
	) {
		t.Fatal("context projection omits its representation boundary")
	}

	otherTarget, err := typedmemory.NewBoundedContextRef("other-project")
	if err != nil {
		t.Fatalf("NewBoundedContextRef other: %v", err)
	}
	if projection.validFor(source, otherTarget) {
		t.Fatal("context projection was reused for a different typed context")
	}

	otherRecord := *record
	otherRecord.Meta.Version++
	otherSource, err := LoadExistingDecisionChoiceSource(
		context.Background(),
		decisionRecordReaderStub{record: &otherRecord},
		otherRecord.Meta.ID,
	)
	if err != nil {
		t.Fatalf("LoadExistingDecisionChoiceSource other: %v", err)
	}
	if projection.validFor(otherSource, target) {
		t.Fatal("context projection was reused for a different source digest")
	}
}

func decisionProjectionTestArtifact(t *testing.T) *artifact.Artifact {
	t.Helper()
	choice := &artifact.ChoiceResult{
		SubjectRef:      "operator",
		OptionSet:       []string{"Option B", "Option A"},
		ComparisonBasis: []string{"Option A satisfies the selected rule"},
		ChoiceRule:      "prefer the reversible option",
		NextMove:        artifact.ChoiceNextMoveChooseNow,
		VariantRef:      "Option A",
		ProblemRefs:     []string{"prob-20260718-test-12345678"},
		PortfolioRef:    "port-20260718-test-12345678",
		Reason:          "Option A is reversible",
		ReopenCondition: "new evidence invalidates the basis",
	}
	fields := artifact.DecisionFields{
		ProblemRefs:        []string{"prob-20260718-test-12345678"},
		SelectedTitle:      "Decision to apply Option A",
		WhySelected:        "Option A is reversible",
		SelectionPolicy:    "prefer the reversible option",
		ChoiceResult:       choice,
		DecisionSubjectRef: "operator",
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal DecisionRecord fixture: %v", err)
	}
	return &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      "dec-20260718-test-12345678",
			Kind:    artifact.KindDecisionRecord,
			Version: 1,
			Status:  artifact.StatusActive,
			Context: "haft-project",
			Title:   "Decision to apply Option A",
		},
		StructuredData: string(encoded),
	}
}
