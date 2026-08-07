package decisionrecordadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	decisionProjectionSourceSchemaV1 = "haft.decision-record-projection-source/v1"
	legacyContextProjectionSchemaV1  = "haft.decision-record-legacy-context-projection/v1"

	choiceFieldCorrelationExact             = "exact_duplicated_fields"
	choiceFieldCorrelationLegacyIndependent = "legacy_independent_choice_fields"
)

type DecisionRecordReader interface {
	Get(context.Context, string) (*artifact.Artifact, error)
}

// ExistingDecisionChoiceSource can only be obtained by loading an existing
// project artifact through LoadExistingDecisionChoiceSource. Its existence is
// not a second manual act: the project authority policy governs creation of
// the source DecisionRecord before this read-only projection begins.
type ExistingDecisionChoiceSource struct {
	source                     recordatconcern.DecisionChoiceSource
	title                      string
	context                    string
	choiceFieldCorrelationMode string
	choiceFieldWarnings        []string
	canonical                  []byte
	provenance                 typedmemory.ProvenanceRef
}

type decisionProjectionSourceCanonicalV1 struct {
	Schema                string          `json:"schema"`
	DecisionRecordRef     string          `json:"decision_record_ref"`
	DecisionRecordVersion int             `json:"decision_record_version"`
	Status                string          `json:"status"`
	Context               string          `json:"context"`
	Title                 string          `json:"title"`
	StructuredDataDigest  string          `json:"structured_data_digest"`
	ChoiceResult          json.RawMessage `json:"choice_result"`
}

func LoadExistingDecisionChoiceSource(
	ctx context.Context,
	reader DecisionRecordReader,
	decisionRef string,
) (ExistingDecisionChoiceSource, error) {
	if ctx == nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"load DecisionRecord projection source requires a context",
		)
	}
	if err := ctx.Err(); err != nil {
		return ExistingDecisionChoiceSource{}, err
	}
	if reader == nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"load DecisionRecord projection source requires a project artifact reader",
		)
	}
	exactRef := strings.TrimSpace(decisionRef)
	if exactRef == "" || exactRef != decisionRef {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"load DecisionRecord projection source requires an exact reference",
		)
	}
	record, err := reader.Get(ctx, exactRef)
	if err != nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"load DecisionRecord %s: %w",
			exactRef,
			err,
		)
	}
	return projectExistingDecisionChoiceSource(record)
}

func projectExistingDecisionChoiceSource(
	record *artifact.Artifact,
) (ExistingDecisionChoiceSource, error) {
	if record == nil || record.Meta.Kind != artifact.KindDecisionRecord {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"DecisionRecord projection source must be an existing DecisionRecord",
		)
	}
	if record.Meta.Status != artifact.StatusActive &&
		record.Meta.Status != artifact.StatusRefreshDue {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"DecisionRecord %s status %s is historical or non-current; v1 projects only active or refresh-due choices",
			record.Meta.ID,
			record.Meta.Status,
		)
	}
	if record.Meta.Version < 1 {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"DecisionRecord %s has no positive artifact version",
			record.Meta.ID,
		)
	}
	fields, err := decodeExactDecisionFields(record.StructuredData)
	if err != nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"DecisionRecord %s structured data: %w",
			record.Meta.ID,
			err,
		)
	}
	choice := artifact.NormalizeChoiceResult(fields.ChoiceResult)
	if choice == nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"DecisionRecord %s has no stored ChoiceResult; a comparison or recommendation cannot be projected as a decision",
			record.Meta.ID,
		)
	}
	if err := artifact.ValidateChoiceResult(choice); err != nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"DecisionRecord %s ChoiceResult: %w",
			record.Meta.ID,
			err,
		)
	}
	if choice.NextMove != artifact.ChoiceNextMoveChooseNow {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"DecisionRecord %s ChoiceResult next_move=%s does not institute one chosen option",
			record.Meta.ID,
			choice.NextMove,
		)
	}
	choiceFieldCorrelationMode, choiceFieldWarnings, err :=
		requireDecisionChoiceFieldCorrelation(
			record,
			fields,
			choice,
		)
	if err != nil {
		return ExistingDecisionChoiceSource{}, err
	}
	choiceJSON, err := json.Marshal(choice)
	if err != nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"encode DecisionRecord %s ChoiceResult: %w",
			record.Meta.ID,
			err,
		)
	}
	structuredDigest := sha256.Sum256([]byte(record.StructuredData))
	sourceCanonical, err := json.Marshal(
		decisionProjectionSourceCanonicalV1{
			Schema:                decisionProjectionSourceSchemaV1,
			DecisionRecordRef:     record.Meta.ID,
			DecisionRecordVersion: record.Meta.Version,
			Status:                string(record.Meta.Status),
			Context:               record.Meta.Context,
			Title:                 record.Meta.Title,
			StructuredDataDigest: "sha256:" +
				hex.EncodeToString(structuredDigest[:]),
			ChoiceResult: append(json.RawMessage(nil), choiceJSON...),
		},
	)
	if err != nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"encode DecisionRecord %s projection source: %w",
			record.Meta.ID,
			err,
		)
	}
	sum := sha256.Sum256(sourceCanonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return ExistingDecisionChoiceSource{}, err
	}
	source, err := recordatconcern.NewDecisionChoiceSource(
		recordatconcern.DecisionChoiceSourceInput{
			RecordRef:     record.Meta.ID,
			RecordVersion: record.Meta.Version,
			RecordDigest:  digest,
			ChoiceJSON:    choiceJSON,
			Subject:       choice.SubjectRef,
			Options:       choice.OptionSet,
			Chosen:        choice.VariantRef,
			ProblemRefs:   choice.ProblemRefs,
			PortfolioRef:  choice.PortfolioRef,
		},
	)
	if err != nil {
		return ExistingDecisionChoiceSource{}, err
	}
	provenance, err := typedmemory.NewProvenanceRef(
		"decision-record-projection:" +
			record.Meta.ID +
			"@" +
			digest.String(),
	)
	if err != nil {
		return ExistingDecisionChoiceSource{}, fmt.Errorf(
			"derive DecisionRecord projection provenance: %w",
			err,
		)
	}
	return ExistingDecisionChoiceSource{
		source:                     source,
		title:                      record.Meta.Title,
		context:                    record.Meta.Context,
		choiceFieldCorrelationMode: choiceFieldCorrelationMode,
		choiceFieldWarnings:        choiceFieldWarnings,
		canonical:                  sourceCanonical,
		provenance:                 provenance,
	}, nil
}

func (source ExistingDecisionChoiceSource) DecisionRecordRef() string {
	return source.source.RecordRef()
}

func (source ExistingDecisionChoiceSource) Title() string {
	return source.title
}

func (source ExistingDecisionChoiceSource) Context() string {
	return source.context
}

func (source ExistingDecisionChoiceSource) ChoiceFieldCorrelationMode() string {
	return source.choiceFieldCorrelationMode
}

func (source ExistingDecisionChoiceSource) ChoiceFieldWarnings() []string {
	return append([]string(nil), source.choiceFieldWarnings...)
}

func (source ExistingDecisionChoiceSource) CanonicalBytes() []byte {
	return append([]byte(nil), source.canonical...)
}

func (source ExistingDecisionChoiceSource) SourceDigest() typedmemory.SHA256Digest {
	return source.source.RecordDigest()
}

func (source ExistingDecisionChoiceSource) valid() bool {
	if len(source.canonical) == 0 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(source.canonical))
	decoder.DisallowUnknownFields()
	encoded := decisionProjectionSourceCanonicalV1{}
	if err := decoder.Decode(&encoded); err != nil {
		return false
	}
	if err := requireProjectionJSONEnd(decoder); err != nil {
		return false
	}
	sum := sha256.Sum256(source.canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	return err == nil &&
		digest == source.source.RecordDigest() &&
		validChoiceFieldCorrelation(
			source.choiceFieldCorrelationMode,
			source.choiceFieldWarnings,
		) &&
		encoded.DecisionRecordRef == source.source.RecordRef() &&
		encoded.DecisionRecordVersion == source.source.RecordVersion() &&
		encoded.Context == source.context &&
		encoded.Title == source.title &&
		bytes.Equal(encoded.ChoiceResult, source.source.ChoiceJSON()) &&
		source.provenance.String() ==
			"decision-record-projection:"+
				source.source.RecordRef()+
				"@"+
				digest.String()
}

// LegacyContextProjection keeps a legacy artifact's free-text Context field
// distinct from the typed BoundedContextRef selected for this projection. It
// records the exact source digest and both values; it does not claim that the
// legacy description and the typed context reference are the same kind.
type LegacyContextProjection struct {
	sourceDigest  typedmemory.SHA256Digest
	sourceContext string
	targetContext typedmemory.BoundedContextRef
	canonical     []byte
}

type legacyContextProjectionCanonicalV1 struct {
	Schema                   string `json:"schema"`
	DecisionProjectionDigest string `json:"decision_projection_digest"`
	LegacyArtifactContext    string `json:"legacy_artifact_context"`
	TypedBoundedContextRef   string `json:"typed_bounded_context_ref"`
	RepresentationBoundaryV1 string `json:"representation_boundary"`
}

func NewLegacyContextProjection(
	source ExistingDecisionChoiceSource,
	target typedmemory.BoundedContextRef,
) (LegacyContextProjection, error) {
	if !source.valid() {
		return LegacyContextProjection{}, fmt.Errorf(
			"legacy context projection requires a loaded exact DecisionRecord source",
		)
	}
	if target.String() == "" {
		return LegacyContextProjection{}, fmt.Errorf(
			"legacy context projection requires a typed bounded-context reference",
		)
	}
	encoded := legacyContextProjectionCanonicalV1{
		Schema:                   legacyContextProjectionSchemaV1,
		DecisionProjectionDigest: source.SourceDigest().String(),
		LegacyArtifactContext:    source.Context(),
		TypedBoundedContextRef:   target.String(),
		RepresentationBoundaryV1: "legacy_artifact_context_is_preserved_source_description_" +
			"not_typed_bounded_context_identity",
	}
	canonical, err := json.Marshal(encoded)
	if err != nil {
		return LegacyContextProjection{}, fmt.Errorf(
			"encode DecisionRecord legacy context projection: %w",
			err,
		)
	}
	return LegacyContextProjection{
		sourceDigest:  source.SourceDigest(),
		sourceContext: source.Context(),
		targetContext: target,
		canonical:     canonical,
	}, nil
}

func (projection LegacyContextProjection) SourceContext() string {
	return projection.sourceContext
}

func (projection LegacyContextProjection) TargetContext() typedmemory.BoundedContextRef {
	return projection.targetContext
}

func (projection LegacyContextProjection) CanonicalBytes() []byte {
	return append([]byte(nil), projection.canonical...)
}

func (projection LegacyContextProjection) validFor(
	source ExistingDecisionChoiceSource,
	target typedmemory.BoundedContextRef,
) bool {
	if !source.valid() || len(projection.canonical) == 0 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(projection.canonical))
	decoder.DisallowUnknownFields()
	encoded := legacyContextProjectionCanonicalV1{}
	if err := decoder.Decode(&encoded); err != nil {
		return false
	}
	if err := requireProjectionJSONEnd(decoder); err != nil {
		return false
	}
	expected, err := NewLegacyContextProjection(source, target)
	if err != nil {
		return false
	}
	return projection.sourceDigest == source.SourceDigest() &&
		projection.sourceContext == source.Context() &&
		projection.targetContext == target &&
		bytes.Equal(projection.canonical, expected.canonical) &&
		encoded.DecisionProjectionDigest == source.SourceDigest().String() &&
		encoded.LegacyArtifactContext == source.Context() &&
		encoded.TypedBoundedContextRef == target.String()
}

type ProjectionDraftInput struct {
	ProjectID         projectidentity.ProjectID
	RecordEntity      typedmemory.EntityID
	RecordLocalRef    typedmemory.BatchLocalRef
	RecordLabel       typedmemory.EntityLabel
	AssertionID       typedmemory.AssertionID
	ContextSlice      typedmemory.ContextSlice
	Source            ExistingDecisionChoiceSource
	ContextProjection LegacyContextProjection
	Concern           ExactConcernBinding
	Problem           OptionalProjectRecordReference
	Portfolio         OptionalProjectRecordReference
	Options           []DecisionOptionBinding
	Comparison        OptionalProjectRecordReference
}

type Draft = recordatconcern.DecisionProjectionDraft

func NewDraft(input ProjectionDraftInput) (Draft, error) {
	if !input.Source.valid() {
		return Draft{}, fmt.Errorf(
			"DecisionRecord projection requires a loaded exact source",
		)
	}
	if input.RecordEntity.String() != input.Source.DecisionRecordRef() ||
		input.RecordLocalRef.String() != input.Source.DecisionRecordRef() {
		return Draft{}, fmt.Errorf(
			"DecisionRecord typed entity and batch-local reference must exactly reuse source identity %s",
			input.Source.DecisionRecordRef(),
		)
	}
	if input.RecordLabel.String() != input.Source.Title() {
		return Draft{}, fmt.Errorf(
			"DecisionRecord typed label %q differs from source title %q",
			input.RecordLabel,
			input.Source.Title(),
		)
	}
	if !input.ContextProjection.validFor(
		input.Source,
		input.ContextSlice.Context(),
	) {
		return Draft{}, fmt.Errorf(
			"DecisionRecord legacy context %q has no exact projection to typed ContextSlice %q",
			input.Source.Context(),
			input.ContextSlice.Context(),
		)
	}
	return recordatconcern.NewDecisionProjectionDraft(
		recordatconcern.DecisionProjectionDraftInput{
			ProjectID:      input.ProjectID,
			RecordEntity:   input.RecordEntity,
			RecordLocalRef: input.RecordLocalRef,
			RecordLabel:    input.RecordLabel,
			AssertionID:    input.AssertionID,
			ContextSlice:   input.ContextSlice,
			Source:         input.Source.source,
			Concern:        input.Concern,
			Problem:        input.Problem,
			Portfolio:      input.Portfolio,
			Options:        input.Options,
			Comparison:     input.Comparison,
			Provenance:     input.Source.provenance,
		},
	)
}

type DecisionOptionBinding = recordatconcern.DecisionOptionBinding

func NewDecisionOptionBinding(
	label string,
	reference typedmemory.PersistedRef,
) (DecisionOptionBinding, error) {
	return recordatconcern.NewDecisionOptionBinding(label, reference)
}

type OptionalProjectRecordReference = recordatconcern.OptionalProjectRecordReference
type OptionalProjectRecordReferenceKind = recordatconcern.OptionalProjectRecordReferenceKind

const (
	OptionalProjectRecordAbsent = recordatconcern.OptionalProjectRecordAbsent
	OptionalProjectRecordExact  = recordatconcern.OptionalProjectRecordExact
)

func NoProjectRecordReference() OptionalProjectRecordReference {
	return recordatconcern.NoProjectRecordReference()
}

func NewExactProjectRecordReference(
	sourceRef string,
	reference typedmemory.PersistedRef,
) (OptionalProjectRecordReference, error) {
	return recordatconcern.NewExactProjectRecordReference(
		sourceRef,
		reference,
	)
}

type RuntimeBasis = recordatconcern.RuntimeBasis
type ExactRuntimeBasis = recordatconcern.ExactRuntimeBasis
type ExactRuntimeBasisBuilder = recordatconcern.ExactRuntimeBasisBuilder
type MissingRuntimeBasis = recordatconcern.MissingRuntimeBasis

func NewExactRuntimeBasisBuilder(
	project projectidentity.ProjectID,
) ExactRuntimeBasisBuilder {
	return recordatconcern.NewExactRuntimeBasisBuilder(project)
}

type ExactConcernBinding = recordatconcern.ExactConcernBinding

func NewExactConcernBinding(
	resolution typedmemory.ResolvedStrongReference,
) (ExactConcernBinding, error) {
	return recordatconcern.NewExactConcernBinding(resolution)
}

type MissingBasis = recordatconcern.MissingBasis
type Result = recordatconcern.Result
type ValidCandidate = recordatconcern.ValidCandidate
type Invalid = recordatconcern.Invalid
type Underdetermined = recordatconcern.Underdetermined

func decodeExactDecisionFields(
	raw string,
) (artifact.DecisionFields, error) {
	if strings.TrimSpace(raw) == "" {
		return artifact.DecisionFields{}, fmt.Errorf(
			"stored structured data is required",
		)
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	fields := artifact.DecisionFields{}
	if err := decoder.Decode(&fields); err != nil {
		return artifact.DecisionFields{}, err
	}
	if err := requireProjectionJSONEnd(decoder); err != nil {
		return artifact.DecisionFields{}, err
	}
	return fields, nil
}

func requireDecisionChoiceFieldCorrelation(
	record *artifact.Artifact,
	fields artifact.DecisionFields,
	choice *artifact.ChoiceResult,
) (string, []string, error) {
	if strings.TrimSpace(record.Meta.ID) == "" ||
		record.Meta.ID != strings.TrimSpace(record.Meta.ID) {
		return "", nil, fmt.Errorf(
			"DecisionRecord identity is missing or noncanonical",
		)
	}
	if strings.TrimSpace(record.Meta.Title) == "" ||
		record.Meta.Title != strings.TrimSpace(record.Meta.Title) ||
		record.Meta.Title != fields.SelectedTitle {
		return "", nil, fmt.Errorf(
			"DecisionRecord %s title and selected_title differ or are noncanonical",
			record.Meta.ID,
		)
	}
	warnings := make([]string, 0, 3)
	selectionPolicy := strings.TrimSpace(fields.SelectionPolicy)
	if choice.ChoiceRule != selectionPolicy {
		warnings = append(
			warnings,
			"choice_result.choice_rule differs from selection_policy",
		)
	}
	if choice.Reason != strings.TrimSpace(fields.WhySelected) {
		warnings = append(
			warnings,
			"choice_result.reason differs from why_selected",
		)
	}
	choiceProblemRefs := canonicalStrings(choice.ProblemRefs)
	decisionProblemRefs := canonicalStrings(fields.ProblemRefs)
	if !slices.Equal(choiceProblemRefs, decisionProblemRefs) {
		warnings = append(
			warnings,
			"choice_result.problem_refs differ from decision problem_refs",
		)
	}
	if len(warnings) == 0 {
		return choiceFieldCorrelationExact, nil, nil
	}
	return choiceFieldCorrelationLegacyIndependent, warnings, nil
}

func validChoiceFieldCorrelation(
	mode string,
	warnings []string,
) bool {
	if mode == choiceFieldCorrelationExact {
		return len(warnings) == 0
	}
	return mode == choiceFieldCorrelationLegacyIndependent &&
		len(warnings) > 0
}

func canonicalStrings(values []string) []string {
	owned := make([]string, 0, len(values))
	for _, value := range values {
		canonical := strings.TrimSpace(value)
		if canonical == "" {
			continue
		}
		owned = append(owned, canonical)
	}
	sort.Strings(owned)
	return slices.Compact(owned)
}

func requireProjectionJSONEnd(decoder *json.Decoder) error {
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("expected exactly one JSON value")
}
