// Package neighborhood owns the pure, snapshot-bound read grammar for one
// EntityOfConcern neighborhood. It does not load a graph, read carriers,
// choose an EntityOfConcern, mutate typed memory, or grant authority.
package neighborhood

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectmemory/projectionprofile"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const ProjectionProfileSchemaV1 = "haft.projection-profile/v1"

// FacetKind is a presentation grouping in a read projection. Facet order is
// profile-owned presentation order only; it is never relation, applicability,
// priority, causality, method order, or Work order.
type FacetKind = projectionprofile.FacetKind

const (
	FacetEpistemes      = projectionprofile.FacetEpistemes
	FacetProblems       = projectionprofile.FacetProblems
	FacetAlternatives   = projectionprofile.FacetAlternatives
	FacetDecisions      = projectionprofile.FacetDecisions
	FacetSpecifications = projectionprofile.FacetSpecifications
	FacetEvidence       = projectionprofile.FacetEvidence
	FacetWork           = projectionprofile.FacetWork
	FacetImplementation = projectionprofile.FacetImplementation
	FacetUnresolved     = projectionprofile.FacetUnresolved
)

func KnownFacetKinds() []FacetKind {
	return projectionprofile.KnownFacetKinds()
}

// ItemKind is a projection-local classification of an already-admitted item.
// It does not mint or replace the item's U.Kind.
type ItemKind string

const (
	ItemProblemCard             ItemKind = "problem_card"
	ItemSolutionOption          ItemKind = "solution_option"
	ItemSolutionPortfolio       ItemKind = "solution_portfolio"
	ItemPortfolioComparison     ItemKind = "portfolio_comparison"
	ItemDecisionRecord          ItemKind = "decision_record"
	ItemSpecSection             ItemKind = "spec_section"
	ItemProjectClaim            ItemKind = "project_claim"
	ItemEvidenceRecord          ItemKind = "evidence_record"
	ItemNoteRecord              ItemKind = "note_record"
	ItemSupportingEpisteme      ItemKind = "supporting_episteme_record"
	ItemWorkRecord              ItemKind = "work_record"
	ItemPerformedWorkOccurrence ItemKind = "performed_work_occurrence"
	ItemCodeAnchor              ItemKind = "code_anchor"
	ItemUnresolvedLegacyRecord  ItemKind = "unresolved_legacy_record"
)

var knownItemKinds = []ItemKind{
	ItemCodeAnchor,
	ItemDecisionRecord,
	ItemEvidenceRecord,
	ItemNoteRecord,
	ItemPerformedWorkOccurrence,
	ItemPortfolioComparison,
	ItemProblemCard,
	ItemProjectClaim,
	ItemSolutionOption,
	ItemSolutionPortfolio,
	ItemSpecSection,
	ItemSupportingEpisteme,
	ItemUnresolvedLegacyRecord,
	ItemWorkRecord,
}

func (kind ItemKind) Valid() bool {
	return slices.Contains(knownItemKinds, kind)
}

// DetailLevel is a closed projection density. It never changes canonical
// state or the truth/authority posture of an item.
type DetailLevel string

const (
	DetailOverview DetailLevel = "overview"
	DetailStandard DetailLevel = "standard"
	DetailEvidence DetailLevel = "evidence"
)

var knownDetailLevels = []DetailLevel{
	DetailOverview,
	DetailStandard,
	DetailEvidence,
}

func (level DetailLevel) Valid() bool {
	return slices.Contains(knownDetailLevels, level)
}

type QuestionFamily string

const (
	QuestionAgentOrientation    QuestionFamily = "agent_orientation"
	QuestionDecisionRationale   QuestionFamily = "decision_rationale"
	QuestionSpecImpact          QuestionFamily = "spec_impact"
	QuestionEvidenceCurrentness QuestionFamily = "evidence_currentness"
	QuestionImplementationTrace QuestionFamily = "implementation_trace"
)

type ReceivingUse string

const (
	ReceivingUseAgentOrientation    ReceivingUse = "agent_orientation"
	ReceivingUseDecisionRationale   ReceivingUse = "decision_rationale"
	ReceivingUseSpecImpact          ReceivingUse = "spec_impact"
	ReceivingUseEvidenceCurrentness ReceivingUse = "evidence_currentness"
	ReceivingUseImplementationTrace ReceivingUse = "implementation_trace"
)

type AudienceClass string

const (
	AudienceAgent             AudienceClass = "agent"
	AudienceEngineerOrManager AudienceClass = "engineer_or_manager"
	AudienceReviewer          AudienceClass = "reviewer"
)

// EntityKindPolicy is explicit even when the v1 profile accepts every entity
// already admitted by the selected TypeEnv. It is not a repository classifier
// and does not infer a U.Kind from labels or source order.
type EntityKindPolicy string

const EntityKindAnyAdmitted EntityKindPolicy = "any_admitted_entity"

// ProfileInputKind names an input family a pure assembler may consume after an
// outer shell has pinned and loaded it. The profile itself performs no IO.
type ProfileInputKind string

const (
	InputCanonicalTypedMemory ProfileInputKind = "canonical_typed_memory"
	InputAdapterMappings      ProfileInputKind = "adapter_mapping_manifests"
	InputSpecProjection       ProfileInputKind = "spec_carrier_projection"
	InputEvidenceProjection   ProfileInputKind = "evidence_projection"
	InputCodeProjection       ProfileInputKind = "code_symbol_projection"
)

type IntentionalLossKind string

const (
	LossProfileFacetFiltering IntentionalLossKind = "profile_facet_filtering"
	LossUnrequestedFacets     IntentionalLossKind = "unrequested_facets"
	LossNoGeneratedSummary    IntentionalLossKind = "no_generated_summary"
	LossNoInferredRelation    IntentionalLossKind = "no_inferred_relation"
	LossNoWorkOrder           IntentionalLossKind = "no_work_order"
)

// ProjectionProfileRef includes its immutable edition in the canonical token.
type ProjectionProfileRef = projectionprofile.Ref

func ParseProjectionProfileRef(raw string) (ProjectionProfileRef, error) {
	return projectionprofile.ParseRef(raw)
}

type itemFacetRule struct {
	item  ItemKind
	facet FacetKind
}

// ProjectionProfileDefinition is immutable kernel-owned configuration. Its
// digest covers every semantic field; callers receive defensive copies.
type ProjectionProfileDefinition struct {
	descriptor       projectionprofile.Descriptor
	purpose          string
	receivingUse     ReceivingUse
	audience         AudienceClass
	questions        []QuestionFamily
	entityKindPolicy EntityKindPolicy
	itemFacetRules   []itemFacetRule
	details          []DetailLevel
	inputs           []ProfileInputKind
	intentionalLoss  []IntentionalLossKind
	schemaVersion    string
}

func (definition ProjectionProfileDefinition) Ref() ProjectionProfileRef {
	return definition.descriptor.Ref()
}

func (definition ProjectionProfileDefinition) Edition() uint32 {
	return definition.descriptor.Edition()
}

func (definition ProjectionProfileDefinition) Digest() typedmemory.SHA256Digest {
	return definition.descriptor.Digest()
}

func (definition ProjectionProfileDefinition) Purpose() string {
	return definition.purpose
}

func (definition ProjectionProfileDefinition) ReceivingUse() ReceivingUse {
	return definition.receivingUse
}

func (definition ProjectionProfileDefinition) Audience() AudienceClass {
	return definition.audience
}

func (definition ProjectionProfileDefinition) QuestionFamilies() []QuestionFamily {
	return append([]QuestionFamily{}, definition.questions...)
}

func (definition ProjectionProfileDefinition) EntityKindPolicy() EntityKindPolicy {
	return definition.entityKindPolicy
}

func (definition ProjectionProfileDefinition) Facets() []FacetKind {
	return definition.descriptor.Facets()
}

func (definition ProjectionProfileDefinition) DetailLevels() []DetailLevel {
	return append([]DetailLevel{}, definition.details...)
}

func (definition ProjectionProfileDefinition) Inputs() []ProfileInputKind {
	return append([]ProfileInputKind{}, definition.inputs...)
}

func (definition ProjectionProfileDefinition) SlotReads() []typedmemory.SlotKindID {
	return definition.descriptor.SlotReads()
}

func (definition ProjectionProfileDefinition) IntentionalLosses() []IntentionalLossKind {
	return append([]IntentionalLossKind{}, definition.intentionalLoss...)
}

func (definition ProjectionProfileDefinition) SchemaVersion() string {
	return definition.schemaVersion
}

func (definition ProjectionProfileDefinition) AllowsFacet(facet FacetKind) bool {
	return definition.Valid() && definition.descriptor.AllowsFacet(facet)
}

func (definition ProjectionProfileDefinition) AllowsDetail(level DetailLevel) bool {
	return definition.Valid() && slices.Contains(definition.details, level)
}

func (definition ProjectionProfileDefinition) FacetForItem(
	item ItemKind,
) (FacetKind, bool) {
	if !definition.Valid() || !item.Valid() {
		return "", false
	}
	index, found := slices.BinarySearchFunc(
		definition.itemFacetRules,
		item,
		func(rule itemFacetRule, candidate ItemKind) int {
			return strings.Compare(string(rule.item), string(candidate))
		},
	)
	if !found {
		return "", false
	}
	return definition.itemFacetRules[index].facet, true
}

func (definition ProjectionProfileDefinition) Valid() bool {
	return validateProjectionProfileDefinition(definition) == nil
}

type projectionProfileBuilder struct {
	definition ProjectionProfileDefinition
}

func newProjectionProfileBuilder() *projectionProfileBuilder {
	return &projectionProfileBuilder{}
}

func (builder *projectionProfileBuilder) setDescriptor(
	descriptor projectionprofile.Descriptor,
	schemaVersion string,
) *projectionProfileBuilder {
	builder.definition.descriptor = descriptor
	builder.definition.schemaVersion = schemaVersion
	return builder
}

func (builder *projectionProfileBuilder) setPurpose(
	purpose string,
	receivingUse ReceivingUse,
	audience AudienceClass,
) *projectionProfileBuilder {
	builder.definition.purpose = purpose
	builder.definition.receivingUse = receivingUse
	builder.definition.audience = audience
	return builder
}

func (builder *projectionProfileBuilder) setQuestions(
	values []QuestionFamily,
) *projectionProfileBuilder {
	builder.definition.questions = append([]QuestionFamily{}, values...)
	return builder
}

func (builder *projectionProfileBuilder) setEntityKindPolicy(
	policy EntityKindPolicy,
) *projectionProfileBuilder {
	builder.definition.entityKindPolicy = policy
	return builder
}

func (builder *projectionProfileBuilder) setDetails(
	values []DetailLevel,
) *projectionProfileBuilder {
	builder.definition.details = append([]DetailLevel{}, values...)
	return builder
}

func (builder *projectionProfileBuilder) setInputs(
	values []ProfileInputKind,
) *projectionProfileBuilder {
	builder.definition.inputs = append([]ProfileInputKind{}, values...)
	return builder
}

func (builder *projectionProfileBuilder) setIntentionalLosses(
	values []IntentionalLossKind,
) *projectionProfileBuilder {
	builder.definition.intentionalLoss = append(
		[]IntentionalLossKind{},
		values...,
	)
	return builder
}

func (builder *projectionProfileBuilder) setItemFacetRules(
	values []itemFacetRule,
) *projectionProfileBuilder {
	builder.definition.itemFacetRules = append(
		[]itemFacetRule{},
		values...,
	)
	return builder
}

func (builder *projectionProfileBuilder) build() (
	ProjectionProfileDefinition,
	error,
) {
	definition := builder.definition
	definition.questions = sortedUnique(definition.questions)
	definition.details = canonicalDetailLevels(definition.details)
	definition.inputs = sortedUnique(definition.inputs)
	definition.intentionalLoss = sortedUnique(definition.intentionalLoss)
	definition.itemFacetRules = itemFacetRulesFor(
		definition.Facets(),
		definition.itemFacetRules,
	)
	if err := validateProjectionProfileDefinition(definition); err != nil {
		return ProjectionProfileDefinition{}, err
	}
	return definition, nil
}

var canonicalItemFacetRulesV1 = []itemFacetRule{
	{item: ItemCodeAnchor, facet: FacetImplementation},
	{item: ItemDecisionRecord, facet: FacetDecisions},
	{item: ItemEvidenceRecord, facet: FacetEvidence},
	{item: ItemPerformedWorkOccurrence, facet: FacetWork},
	{item: ItemPortfolioComparison, facet: FacetAlternatives},
	{item: ItemProblemCard, facet: FacetProblems},
	{item: ItemProjectClaim, facet: FacetEpistemes},
	{item: ItemSolutionOption, facet: FacetAlternatives},
	{item: ItemSolutionPortfolio, facet: FacetAlternatives},
	{item: ItemSpecSection, facet: FacetSpecifications},
	{item: ItemSupportingEpisteme, facet: FacetEpistemes},
	{item: ItemUnresolvedLegacyRecord, facet: FacetUnresolved},
	{item: ItemWorkRecord, facet: FacetWork},
}

var canonicalItemFacetRulesV2 = []itemFacetRule{
	{item: ItemCodeAnchor, facet: FacetImplementation},
	{item: ItemDecisionRecord, facet: FacetDecisions},
	{item: ItemEvidenceRecord, facet: FacetEvidence},
	// Epistemes is a presentation grouping here, not a claim that a local
	// NoteAtConcern carrier has U.Episteme as its exact U.Kind.
	{item: ItemNoteRecord, facet: FacetEpistemes},
	{item: ItemPerformedWorkOccurrence, facet: FacetWork},
	{item: ItemPortfolioComparison, facet: FacetAlternatives},
	{item: ItemProblemCard, facet: FacetProblems},
	{item: ItemProjectClaim, facet: FacetEpistemes},
	{item: ItemSolutionOption, facet: FacetAlternatives},
	{item: ItemSolutionPortfolio, facet: FacetAlternatives},
	{item: ItemSpecSection, facet: FacetSpecifications},
	{item: ItemSupportingEpisteme, facet: FacetEpistemes},
	{item: ItemUnresolvedLegacyRecord, facet: FacetUnresolved},
	{item: ItemWorkRecord, facet: FacetWork},
}

func itemFacetRulesFor(
	facets []FacetKind,
	rules []itemFacetRule,
) []itemFacetRule {
	result := make([]itemFacetRule, 0, len(rules))
	for _, rule := range rules {
		if !slices.Contains(facets, rule.facet) {
			continue
		}
		result = append(result, rule)
	}
	sort.Slice(result, func(left int, right int) bool {
		return strings.Compare(
			string(result[left].item),
			string(result[right].item),
		) < 0
	})
	return result
}

func validateProjectionProfileDefinition(
	definition ProjectionProfileDefinition,
) error {
	if !definition.descriptor.Valid() {
		return fmt.Errorf("projection profile identity is invalid")
	}
	installed, found := projectionprofile.Lookup(definition.Ref())
	if !found ||
		installed.Edition() != definition.Edition() ||
		installed.Digest() != definition.Digest() ||
		!slices.Equal(installed.Facets(), definition.Facets()) ||
		!slices.Equal(installed.SlotReads(), definition.SlotReads()) {
		return fmt.Errorf("projection profile descriptor is not installed")
	}
	if definition.schemaVersion != ProjectionProfileSchemaV1 {
		return fmt.Errorf("projection profile schema version is unsupported")
	}
	if definition.purpose == "" ||
		definition.purpose != strings.TrimSpace(definition.purpose) {
		return fmt.Errorf("projection profile purpose is invalid")
	}
	if definition.receivingUse == "" || definition.audience == "" {
		return fmt.Errorf("projection profile use and audience are required")
	}
	if !knownReceivingUse(definition.receivingUse) ||
		!knownAudienceClass(definition.audience) {
		return fmt.Errorf("projection profile use or audience is unsupported")
	}
	if definition.entityKindPolicy != EntityKindAnyAdmitted {
		return fmt.Errorf("projection profile entity-kind policy is unsupported")
	}
	if err := validatePresentationFacets(definition.Facets()); err != nil {
		return err
	}
	if err := validateDetails(definition.details); err != nil {
		return err
	}
	if len(definition.questions) == 0 ||
		len(definition.inputs) == 0 ||
		len(definition.SlotReads()) == 0 ||
		len(definition.intentionalLoss) == 0 {
		return fmt.Errorf("projection profile semantic declarations are incomplete")
	}
	if !allQuestionFamiliesKnown(definition.questions) ||
		!allProfileInputsKnown(definition.inputs) ||
		!allIntentionalLossesKnown(definition.intentionalLoss) {
		return fmt.Errorf("projection profile semantic declaration is unsupported")
	}
	slotReads := definition.SlotReads()
	canonicalSlotReads := canonicalSlotKindReads(slotReads)
	if !slices.Equal(slotReads, canonicalSlotReads) {
		return fmt.Errorf("projection profile slot-read set is not canonical")
	}
	expectedRules := itemFacetRulesFor(
		definition.Facets(),
		definition.itemFacetRules,
	)
	if len(definition.itemFacetRules) == 0 ||
		!slices.Equal(definition.itemFacetRules, expectedRules) {
		return fmt.Errorf("projection profile item/facet rules are not canonical")
	}
	for _, rule := range definition.itemFacetRules {
		if !rule.item.Valid() || !rule.facet.Valid() {
			return fmt.Errorf("projection profile item/facet rule is unsupported")
		}
	}
	digest, err := projectionProfileDigest(definition)
	if err != nil {
		return err
	}
	if digest != definition.Digest() {
		return fmt.Errorf("projection profile digest is not canonical")
	}
	return nil
}

func validatePresentationFacets(values []FacetKind) error {
	if len(values) == 0 {
		return fmt.Errorf("projection profile requires at least one facet")
	}
	seen := make(map[FacetKind]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return fmt.Errorf("projection profile facet %q is invalid", value)
		}
		if _, found := seen[value]; found {
			return fmt.Errorf("projection profile facet %q is duplicated", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateDetails(values []DetailLevel) error {
	if len(values) == 0 {
		return fmt.Errorf("projection profile requires at least one detail level")
	}
	seen := make(map[DetailLevel]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return fmt.Errorf("projection profile detail %q is invalid", value)
		}
		if _, found := seen[value]; found {
			return fmt.Errorf("projection profile detail %q is duplicated", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func canonicalDetailLevels(values []DetailLevel) []DetailLevel {
	result := make([]DetailLevel, 0, len(values))
	for _, candidate := range knownDetailLevels {
		if !slices.Contains(values, candidate) {
			continue
		}
		result = append(result, candidate)
	}
	return result
}

func knownReceivingUse(value ReceivingUse) bool {
	values := []ReceivingUse{
		ReceivingUseAgentOrientation,
		ReceivingUseDecisionRationale,
		ReceivingUseSpecImpact,
		ReceivingUseEvidenceCurrentness,
		ReceivingUseImplementationTrace,
	}
	return slices.Contains(values, value)
}

func knownAudienceClass(value AudienceClass) bool {
	values := []AudienceClass{
		AudienceAgent,
		AudienceEngineerOrManager,
		AudienceReviewer,
	}
	return slices.Contains(values, value)
}

func allQuestionFamiliesKnown(values []QuestionFamily) bool {
	known := []QuestionFamily{
		QuestionAgentOrientation,
		QuestionDecisionRationale,
		QuestionSpecImpact,
		QuestionEvidenceCurrentness,
		QuestionImplementationTrace,
	}
	for _, value := range values {
		if !slices.Contains(known, value) {
			return false
		}
	}
	return true
}

func allProfileInputsKnown(values []ProfileInputKind) bool {
	known := []ProfileInputKind{
		InputCanonicalTypedMemory,
		InputAdapterMappings,
		InputSpecProjection,
		InputEvidenceProjection,
		InputCodeProjection,
	}
	for _, value := range values {
		if !slices.Contains(known, value) {
			return false
		}
	}
	return true
}

func allIntentionalLossesKnown(values []IntentionalLossKind) bool {
	known := []IntentionalLossKind{
		LossProfileFacetFiltering,
		LossUnrequestedFacets,
		LossNoGeneratedSummary,
		LossNoInferredRelation,
		LossNoWorkOrder,
	}
	for _, value := range values {
		if !slices.Contains(known, value) {
			return false
		}
	}
	return true
}

type projectionProfileCanonicalV1 struct {
	Ref              string                `json:"profile_ref"`
	Edition          uint32                `json:"edition"`
	Purpose          string                `json:"purpose"`
	ReceivingUse     ReceivingUse          `json:"receiving_use"`
	Audience         AudienceClass         `json:"audience"`
	Questions        []QuestionFamily      `json:"question_families"`
	EntityKindPolicy EntityKindPolicy      `json:"entity_kind_policy"`
	Facets           []FacetKind           `json:"facet_presentation_order"`
	ItemFacetRules   []itemFacetCanonical  `json:"item_facet_rules"`
	Details          []DetailLevel         `json:"allowed_detail_levels"`
	Inputs           []ProfileInputKind    `json:"declared_inputs"`
	SlotReads        []string              `json:"declared_slot_reads"`
	IntentionalLoss  []IntentionalLossKind `json:"intentional_loss"`
	SchemaVersion    string                `json:"projection_schema_version"`
}

type itemFacetCanonical struct {
	Item  ItemKind  `json:"item_kind"`
	Facet FacetKind `json:"facet"`
}

func projectionProfileDigest(
	definition ProjectionProfileDefinition,
) (typedmemory.SHA256Digest, error) {
	rules := make([]itemFacetCanonical, 0, len(definition.itemFacetRules))
	for _, rule := range definition.itemFacetRules {
		rules = append(rules, itemFacetCanonical{
			Item:  rule.item,
			Facet: rule.facet,
		})
	}
	carrier := projectionProfileCanonicalV1{
		Ref:              definition.Ref().String(),
		Edition:          definition.Edition(),
		Purpose:          definition.purpose,
		ReceivingUse:     definition.receivingUse,
		Audience:         definition.audience,
		Questions:        append([]QuestionFamily{}, definition.questions...),
		EntityKindPolicy: definition.entityKindPolicy,
		Facets:           definition.Facets(),
		ItemFacetRules:   rules,
		Details:          append([]DetailLevel{}, definition.details...),
		Inputs:           append([]ProfileInputKind{}, definition.inputs...),
		SlotReads:        slotReadStrings(definition.SlotReads()),
		IntentionalLoss: append(
			[]IntentionalLossKind{},
			definition.intentionalLoss...,
		),
		SchemaVersion: definition.schemaVersion,
	}
	canonical, err := json.Marshal(carrier)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"encode projection profile: %w",
			err,
		)
	}
	sum := sha256.Sum256(canonical)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(raw)
}

func sortedUnique[T ~string](values []T) []T {
	result := append([]T{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left] < result[right]
	})
	return slices.Compact(result)
}

func canonicalSlotKindReads(
	values []typedmemory.SlotKindID,
) []typedmemory.SlotKindID {
	result := append([]typedmemory.SlotKindID{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return slices.CompactFunc(
		result,
		func(left typedmemory.SlotKindID, right typedmemory.SlotKindID) bool {
			return left.String() == right.String()
		},
	)
}

func slotReadStrings(values []typedmemory.SlotKindID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func mustProjectionProfile(
	builder *projectionProfileBuilder,
) ProjectionProfileDefinition {
	definition, err := builder.build()
	if err != nil {
		panic(err)
	}
	return definition
}

var builtinProjectionProfiles = buildBuiltinProjectionProfiles()

func BuiltinProjectionProfiles() []ProjectionProfileDefinition {
	return append(
		[]ProjectionProfileDefinition{},
		builtinProjectionProfiles...,
	)
}

func LookupProjectionProfile(
	ref ProjectionProfileRef,
) (ProjectionProfileDefinition, bool) {
	if !ref.Valid() {
		return ProjectionProfileDefinition{}, false
	}
	index, found := slices.BinarySearchFunc(
		builtinProjectionProfiles,
		ref,
		func(
			definition ProjectionProfileDefinition,
			candidate ProjectionProfileRef,
		) int {
			return strings.Compare(
				definition.Ref().String(),
				candidate.String(),
			)
		},
	)
	if !found {
		return ProjectionProfileDefinition{}, false
	}
	return builtinProjectionProfiles[index], true
}

func buildBuiltinProjectionProfiles() []ProjectionProfileDefinition {
	builders := map[string]func(projectionprofile.Descriptor) ProjectionProfileDefinition{
		"agent_orientation.v1":    buildAgentOrientationProfileV1,
		"agent_orientation.v2":    buildAgentOrientationProfileV2,
		"decision_rationale.v1":   buildDecisionRationaleProfile,
		"evidence_currentness.v1": buildEvidenceCurrentnessProfile,
		"implementation_trace.v1": buildImplementationTraceProfile,
		"spec_impact.v1":          buildSpecImpactProfile,
	}
	descriptors := projectionprofile.Installed()
	values := make([]ProjectionProfileDefinition, 0, len(descriptors))
	for _, descriptor := range descriptors {
		builder, found := builders[descriptor.Ref().String()]
		if !found {
			panic("installed projection profile has no neighborhood runtime builder")
		}
		values = append(values, builder(descriptor))
	}
	sort.Slice(values, func(left int, right int) bool {
		return values[left].Ref().String() < values[right].Ref().String()
	})
	return values
}

func buildAgentOrientationProfileV1(
	descriptor projectionprofile.Descriptor,
) ProjectionProfileDefinition {
	builder := newAgentOrientationProfileBuilder(descriptor)
	builder.setItemFacetRules(canonicalItemFacetRulesV1)
	return mustProjectionProfile(builder)
}

func buildAgentOrientationProfileV2(
	descriptor projectionprofile.Descriptor,
) ProjectionProfileDefinition {
	builder := newAgentOrientationProfileBuilder(descriptor)
	builder.setItemFacetRules(canonicalItemFacetRulesV2)
	return mustProjectionProfile(builder)
}

func newAgentOrientationProfileBuilder(
	descriptor projectionprofile.Descriptor,
) *projectionProfileBuilder {
	builder := newProjectionProfileBuilder()
	builder.setDescriptor(descriptor, ProjectionProfileSchemaV1)
	builder.setPurpose(
		"Orient an agent to the exact typed story around one EntityOfConcern",
		ReceivingUseAgentOrientation,
		AudienceAgent,
	)
	builder.setQuestions([]QuestionFamily{QuestionAgentOrientation})
	builder.setEntityKindPolicy(EntityKindAnyAdmitted)
	builder.setDetails(append([]DetailLevel{}, knownDetailLevels...))
	builder.setInputs([]ProfileInputKind{
		InputCanonicalTypedMemory,
		InputAdapterMappings,
		InputSpecProjection,
		InputEvidenceProjection,
		InputCodeProjection,
	})
	builder.setIntentionalLosses(commonProfileLosses())
	return builder
}

func buildDecisionRationaleProfile(
	descriptor projectionprofile.Descriptor,
) ProjectionProfileDefinition {
	builder := newProjectionProfileBuilder()
	builder.setDescriptor(descriptor, ProjectionProfileSchemaV1)
	builder.setPurpose(
		"Recover the exact problem, alternatives, decision, and supporting basis",
		ReceivingUseDecisionRationale,
		AudienceEngineerOrManager,
	)
	builder.setQuestions([]QuestionFamily{QuestionDecisionRationale})
	builder.setEntityKindPolicy(EntityKindAnyAdmitted)
	builder.setDetails(append([]DetailLevel{}, knownDetailLevels...))
	builder.setInputs([]ProfileInputKind{
		InputCanonicalTypedMemory,
		InputAdapterMappings,
		InputEvidenceProjection,
	})
	builder.setIntentionalLosses(commonProfileLosses())
	builder.setItemFacetRules(canonicalItemFacetRulesV1)
	return mustProjectionProfile(builder)
}

func buildEvidenceCurrentnessProfile(
	descriptor projectionprofile.Descriptor,
) ProjectionProfileDefinition {
	builder := newProjectionProfileBuilder()
	builder.setDescriptor(descriptor, ProjectionProfileSchemaV1)
	builder.setPurpose(
		"Inspect evidence and performed-work currentness without refreshing it",
		ReceivingUseEvidenceCurrentness,
		AudienceReviewer,
	)
	builder.setQuestions([]QuestionFamily{QuestionEvidenceCurrentness})
	builder.setEntityKindPolicy(EntityKindAnyAdmitted)
	builder.setDetails([]DetailLevel{DetailStandard, DetailEvidence})
	builder.setInputs([]ProfileInputKind{
		InputCanonicalTypedMemory,
		InputAdapterMappings,
		InputEvidenceProjection,
		InputSpecProjection,
	})
	builder.setIntentionalLosses(commonProfileLosses())
	builder.setItemFacetRules(canonicalItemFacetRulesV1)
	return mustProjectionProfile(builder)
}

func buildImplementationTraceProfile(
	descriptor projectionprofile.Descriptor,
) ProjectionProfileDefinition {
	builder := newProjectionProfileBuilder()
	builder.setDescriptor(descriptor, ProjectionProfileSchemaV1)
	builder.setPurpose(
		"Trace admitted claims and specifications through Work to exact code anchors",
		ReceivingUseImplementationTrace,
		AudienceEngineerOrManager,
	)
	builder.setQuestions([]QuestionFamily{QuestionImplementationTrace})
	builder.setEntityKindPolicy(EntityKindAnyAdmitted)
	builder.setDetails(append([]DetailLevel{}, knownDetailLevels...))
	builder.setInputs([]ProfileInputKind{
		InputCanonicalTypedMemory,
		InputAdapterMappings,
		InputSpecProjection,
		InputEvidenceProjection,
		InputCodeProjection,
	})
	builder.setIntentionalLosses(commonProfileLosses())
	builder.setItemFacetRules(canonicalItemFacetRulesV1)
	return mustProjectionProfile(builder)
}

func buildSpecImpactProfile(
	descriptor projectionprofile.Descriptor,
) ProjectionProfileDefinition {
	builder := newProjectionProfileBuilder()
	builder.setDescriptor(descriptor, ProjectionProfileSchemaV1)
	builder.setPurpose(
		"Inspect exact specification, claim, decision, evidence, and implementation relations",
		ReceivingUseSpecImpact,
		AudienceEngineerOrManager,
	)
	builder.setQuestions([]QuestionFamily{QuestionSpecImpact})
	builder.setEntityKindPolicy(EntityKindAnyAdmitted)
	builder.setDetails(append([]DetailLevel{}, knownDetailLevels...))
	builder.setInputs([]ProfileInputKind{
		InputCanonicalTypedMemory,
		InputAdapterMappings,
		InputSpecProjection,
		InputEvidenceProjection,
		InputCodeProjection,
	})
	builder.setIntentionalLosses(commonProfileLosses())
	builder.setItemFacetRules(canonicalItemFacetRulesV1)
	return mustProjectionProfile(builder)
}

func commonProfileLosses() []IntentionalLossKind {
	return []IntentionalLossKind{
		LossProfileFacetFiltering,
		LossUnrequestedFacets,
		LossNoGeneratedSummary,
		LossNoInferredRelation,
		LossNoWorkOrder,
	}
}
