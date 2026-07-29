// Package projecttypeenvprofilefit deterministically assesses whether one
// exact executable project TypeEnv can discharge an exact current profile.
package projecttypeenvprofilefit

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	AssessmentRuleEditionV1  = "haft.project-typeenv.profile-fit-rules/v1"
	assessmentSchemaV1       = "haft.project-typeenv.profile-fit-assessment/v1"
	assessmentDigestDomain   = "haft.project-typeenv.profile-fit-assessment.v1"
	maximumAssessmentBytes   = 8 << 20
	maximumAssessmentGrounds = 4096
)

type RuleEdition struct{ value string }

func NewRuleEdition(raw string) (RuleEdition, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return RuleEdition{}, fmt.Errorf("profile-fit rule edition is required")
	}
	return RuleEdition{value: raw}, nil
}

func CurrentRuleEdition() RuleEdition {
	edition, _ := NewRuleEdition(AssessmentRuleEditionV1)
	return edition
}

func (edition RuleEdition) String() string { return edition.value }

type GroundKind uint8

const (
	GroundNoCanonicalProfile GroundKind = iota + 1
	GroundSoftwareScope
	GroundKindOrientationUnspecified
	GroundKindCoordinateUnsupported
	GroundKindAvailable
	GroundKindDefinitionWithoutContext
	GroundKindSourceOnly
	GroundKindExplicitlyUnsupported
	GroundKindMissing
	GroundPatternCompiled
	GroundPatternSourceOnly
	GroundPatternExplicitlyUnsupported
	GroundPatternMissing
	GroundContractIndexUnavailable
	GroundAssessorEditionUnavailable
)

func (kind GroundKind) String() string {
	switch kind {
	case GroundNoCanonicalProfile:
		return "no_canonical_project_profile"
	case GroundSoftwareScope:
		return "software_scope"
	case GroundKindOrientationUnspecified:
		return "kind_orientation_unspecified"
	case GroundKindCoordinateUnsupported:
		return "kind_coordinate_unsupported"
	case GroundKindAvailable:
		return "kind_available"
	case GroundKindDefinitionWithoutContext:
		return "kind_definition_without_context"
	case GroundKindSourceOnly:
		return "kind_source_only"
	case GroundKindExplicitlyUnsupported:
		return "kind_explicitly_unsupported"
	case GroundKindMissing:
		return "kind_missing"
	case GroundPatternCompiled:
		return "governing_pattern_compiled"
	case GroundPatternSourceOnly:
		return "governing_pattern_source_only"
	case GroundPatternExplicitlyUnsupported:
		return "governing_pattern_explicitly_unsupported"
	case GroundPatternMissing:
		return "governing_pattern_missing"
	case GroundContractIndexUnavailable:
		return "contract_index_unavailable"
	case GroundAssessorEditionUnavailable:
		return "assessor_edition_unavailable"
	default:
		return ""
	}
}

type GroundPosture uint8

const (
	GroundSatisfied GroundPosture = iota + 1
	GroundContradicted
	GroundMissingBasis
	GroundUnavailable
)

func (posture GroundPosture) String() string {
	switch posture {
	case GroundSatisfied:
		return "satisfied"
	case GroundContradicted:
		return "contradicted"
	case GroundMissingBasis:
		return "missing_basis"
	case GroundUnavailable:
		return "unavailable"
	default:
		return ""
	}
}

// Ground is one typed, exact basis item. Free-form prose is deliberately not
// an input to result selection.
type Ground struct {
	kind       GroundKind
	posture    GroundPosture
	scopeID    string
	coordinate string
	contexts   []string
}

func (ground Ground) Kind() GroundKind       { return ground.kind }
func (ground Ground) Posture() GroundPosture { return ground.posture }
func (ground Ground) ScopeID() string        { return ground.scopeID }
func (ground Ground) Coordinate() string     { return ground.coordinate }
func (ground Ground) Contexts() []string {
	return append([]string(nil), ground.contexts...)
}

func (ground Ground) String() string {
	parts := []string{ground.posture.String(), ground.kind.String()}
	if ground.scopeID != "" {
		parts = append(parts, "scope="+ground.scopeID)
	}
	if ground.coordinate != "" {
		parts = append(parts, "coordinate="+ground.coordinate)
	}
	if len(ground.contexts) > 0 {
		parts = append(parts, "contexts="+strings.Join(ground.contexts, ","))
	}
	return strings.Join(parts, ";")
}

type Assessment interface {
	BasisRef() projecttypeenvprofilebasis.ProjectProfileBasisRef
	BasisDigest() typedmemory.SHA256Digest
	TargetTypeEnvRef() typedmemory.TypeEnvRef
	TargetSnapshotDigest() typedmemory.SHA256Digest
	RuleEdition() RuleEdition
	Digest() typedmemory.SHA256Digest
	FitRef() ProjectTypeEnvProfileFitRef
	Grounds() []Ground
	CanonicalBytes() []byte
	Verify() error
	assessmentVariant()
}

type ProjectTypeEnvProfileFitRef struct{ digest typedmemory.SHA256Digest }

func ParseProjectTypeEnvProfileFitRef(raw string) (ProjectTypeEnvProfileFitRef, error) {
	const prefix = "project-typeenv-profile-fit:"
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, prefix) {
		return ProjectTypeEnvProfileFitRef{}, fmt.Errorf(
			"project TypeEnv profile-fit ref must start with %q",
			prefix,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return ProjectTypeEnvProfileFitRef{}, err
	}
	return ProjectTypeEnvProfileFitRef{digest: digest}, nil
}

func (ref ProjectTypeEnvProfileFitRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref ProjectTypeEnvProfileFitRef) String() string {
	return "project-typeenv-profile-fit:" + ref.digest.String()
}

type assessmentState struct {
	basisRef     projecttypeenvprofilebasis.ProjectProfileBasisRef
	basisDigest  typedmemory.SHA256Digest
	targetRef    typedmemory.TypeEnvRef
	targetDigest typedmemory.SHA256Digest
	edition      RuleEdition
	digest       typedmemory.SHA256Digest
	ref          ProjectTypeEnvProfileFitRef
	grounds      []Ground
	canonical    []byte
}

type Compatible struct{ state assessmentState }
type Incompatible struct{ state assessmentState }
type Underdetermined struct{ state assessmentState }
type Unavailable struct{ state assessmentState }

func (Compatible) assessmentVariant()      {}
func (Incompatible) assessmentVariant()    {}
func (Underdetermined) assessmentVariant() {}
func (Unavailable) assessmentVariant()     {}

func (value Compatible) BasisRef() projecttypeenvprofilebasis.ProjectProfileBasisRef {
	return value.state.basisRef
}
func (value Incompatible) BasisRef() projecttypeenvprofilebasis.ProjectProfileBasisRef {
	return value.state.basisRef
}
func (value Underdetermined) BasisRef() projecttypeenvprofilebasis.ProjectProfileBasisRef {
	return value.state.basisRef
}
func (value Unavailable) BasisRef() projecttypeenvprofilebasis.ProjectProfileBasisRef {
	return value.state.basisRef
}

func (value Compatible) BasisDigest() typedmemory.SHA256Digest {
	return value.state.basisDigest
}
func (value Incompatible) BasisDigest() typedmemory.SHA256Digest {
	return value.state.basisDigest
}
func (value Underdetermined) BasisDigest() typedmemory.SHA256Digest {
	return value.state.basisDigest
}
func (value Unavailable) BasisDigest() typedmemory.SHA256Digest {
	return value.state.basisDigest
}

func (value Compatible) TargetTypeEnvRef() typedmemory.TypeEnvRef {
	return value.state.targetRef
}
func (value Incompatible) TargetTypeEnvRef() typedmemory.TypeEnvRef {
	return value.state.targetRef
}
func (value Underdetermined) TargetTypeEnvRef() typedmemory.TypeEnvRef {
	return value.state.targetRef
}
func (value Unavailable) TargetTypeEnvRef() typedmemory.TypeEnvRef {
	return value.state.targetRef
}

func (value Compatible) TargetSnapshotDigest() typedmemory.SHA256Digest {
	return value.state.targetDigest
}
func (value Incompatible) TargetSnapshotDigest() typedmemory.SHA256Digest {
	return value.state.targetDigest
}
func (value Underdetermined) TargetSnapshotDigest() typedmemory.SHA256Digest {
	return value.state.targetDigest
}
func (value Unavailable) TargetSnapshotDigest() typedmemory.SHA256Digest {
	return value.state.targetDigest
}

func (value Compatible) RuleEdition() RuleEdition      { return value.state.edition }
func (value Incompatible) RuleEdition() RuleEdition    { return value.state.edition }
func (value Underdetermined) RuleEdition() RuleEdition { return value.state.edition }
func (value Unavailable) RuleEdition() RuleEdition     { return value.state.edition }

func (value Compatible) Digest() typedmemory.SHA256Digest      { return value.state.digest }
func (value Incompatible) Digest() typedmemory.SHA256Digest    { return value.state.digest }
func (value Underdetermined) Digest() typedmemory.SHA256Digest { return value.state.digest }
func (value Unavailable) Digest() typedmemory.SHA256Digest     { return value.state.digest }

func (value Compatible) FitRef() ProjectTypeEnvProfileFitRef      { return value.state.ref }
func (value Incompatible) FitRef() ProjectTypeEnvProfileFitRef    { return value.state.ref }
func (value Underdetermined) FitRef() ProjectTypeEnvProfileFitRef { return value.state.ref }
func (value Unavailable) FitRef() ProjectTypeEnvProfileFitRef     { return value.state.ref }

func (value Compatible) Grounds() []Ground      { return cloneGrounds(value.state.grounds) }
func (value Incompatible) Grounds() []Ground    { return cloneGrounds(value.state.grounds) }
func (value Underdetermined) Grounds() []Ground { return cloneGrounds(value.state.grounds) }
func (value Unavailable) Grounds() []Ground     { return cloneGrounds(value.state.grounds) }

func (value Compatible) CanonicalBytes() []byte {
	return append([]byte(nil), value.state.canonical...)
}
func (value Incompatible) CanonicalBytes() []byte {
	return append([]byte(nil), value.state.canonical...)
}
func (value Underdetermined) CanonicalBytes() []byte {
	return append([]byte(nil), value.state.canonical...)
}
func (value Unavailable) CanonicalBytes() []byte {
	return append([]byte(nil), value.state.canonical...)
}

func (value Compatible) Verify() error   { return verifyAssessmentState("compatible", value.state) }
func (value Incompatible) Verify() error { return verifyAssessmentState("incompatible", value.state) }
func (value Underdetermined) Verify() error {
	return verifyAssessmentState("underdetermined", value.state)
}
func (value Unavailable) Verify() error { return verifyAssessmentState("unavailable", value.state) }

// AssessProjectTypeEnvProfileFit validates both exact inputs before executing
// the current deterministic rule edition.
func AssessProjectTypeEnvProfileFit(
	basis projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	target projecttypeenv.ProjectTypeEnvExecutableSnapshot,
) (Assessment, error) {
	return AssessProjectTypeEnvProfileFitWithEdition(basis, target, CurrentRuleEdition())
}

func AssessProjectTypeEnvProfileFitWithEdition(
	basis projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	target projecttypeenv.ProjectTypeEnvExecutableSnapshot,
	edition RuleEdition,
) (Assessment, error) {
	if basis == nil {
		return nil, fmt.Errorf("current project-profile basis is required")
	}
	if err := basis.Verify(); err != nil {
		return nil, fmt.Errorf("verify current project-profile basis: %w", err)
	}
	if err := target.Verify(); err != nil {
		return nil, fmt.Errorf("verify target executable project TypeEnv: %w", err)
	}
	if edition.String() == "" {
		return nil, fmt.Errorf("profile-fit rule edition is required")
	}
	return assessVerifiedEnvironment(
		basis,
		target.TypeEnvRef(),
		target.Digest(),
		target.Environment(),
		edition,
	)
}

func assessVerifiedEnvironment(
	basis projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	targetRef typedmemory.TypeEnvRef,
	targetDigest typedmemory.SHA256Digest,
	environment typedmemory.TypeEnv,
	edition RuleEdition,
) (Assessment, error) {
	if edition.String() == "" {
		return nil, fmt.Errorf("profile-fit rule edition is required")
	}
	if edition.String() != AssessmentRuleEditionV1 {
		ground := newGround(
			GroundAssessorEditionUnavailable,
			GroundUnavailable,
			"",
			edition.String(),
			nil,
		)
		return mintAssessment("unavailable", basis, targetRef, targetDigest, edition, []Ground{ground})
	}
	switch value := basis.(type) {
	case projecttypeenvprofilebasis.NoCanonicalProjectProfile:
		ground := newGround(
			GroundNoCanonicalProfile,
			GroundMissingBasis,
			"",
			"current-canonical-project-profile",
			nil,
		)
		return mintAssessment("underdetermined", value, targetRef, targetDigest, edition, []Ground{ground})
	case projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile:
		grounds := assessDeclaredProfile(value.Payload(), environment)
		variant := selectVariant(grounds)
		return mintAssessment(variant, value, targetRef, targetDigest, edition, grounds)
	default:
		return nil, fmt.Errorf("unknown current project-profile basis variant")
	}
}

func assessDeclaredProfile(
	payload projectprofile.ProfileDeclarationPayload,
	environment typedmemory.TypeEnv,
) []Ground {
	grounds := []Ground{}
	for _, scope := range payload.Scopes().Values() {
		switch value := scope.(type) {
		case projectprofile.SoftwareRealization:
			grounds = append(grounds, newGround(
				GroundSoftwareScope,
				GroundSatisfied,
				value.ScopeID().String(),
				"software_realization",
				nil,
			))
		case projectprofile.NonSoftwareRealization:
			grounds = append(grounds, assessKind(value, environment)...)
			grounds = append(grounds, assessPatterns(value, environment)...)
			grounds = append(grounds, assessContracts(value)...)
		}
	}
	return canonicalGrounds(grounds)
}

func assessKind(
	scope projectprofile.NonSoftwareRealization,
	environment typedmemory.TypeEnv,
) []Ground {
	scopeID := scope.ScopeID().String()
	switch orientation := scope.KindOrientation().(type) {
	case projectprofile.UnspecifiedKindOrientation:
		return []Ground{newGround(
			GroundKindOrientationUnspecified,
			GroundMissingBasis,
			scopeID,
			"kind_orientation",
			nil,
		)}
	case projectprofile.ReferencedKindOrientation:
		return []Ground{assessReferencedKind(scopeID, orientation.Ref(), environment)}
	default:
		return []Ground{newGround(
			GroundKindCoordinateUnsupported,
			GroundContradicted,
			scopeID,
			"unknown_kind_orientation_variant",
			nil,
		)}
	}
}

func assessReferencedKind(
	scopeID string,
	ref projectprofile.KindRef,
	environment typedmemory.TypeEnv,
) Ground {
	kindID, err := typedmemory.NewKindID(ref.String())
	if err != nil {
		return newGround(
			GroundKindCoordinateUnsupported,
			GroundContradicted,
			scopeID,
			ref.String(),
			nil,
		)
	}
	_, defined := environment.KindDefinition(kindID)
	contexts := contextsForKind(environment, kindID)
	if defined && len(contexts) > 0 {
		return newGround(GroundKindAvailable, GroundSatisfied, scopeID, kindID.String(), contexts)
	}
	if defined {
		return newGround(
			GroundKindDefinitionWithoutContext,
			GroundContradicted,
			scopeID,
			kindID.String(),
			nil,
		)
	}
	symbol, _ := typedmemory.KindSymbolRef(kindID)
	subject, _ := typedmemory.SchemaSymbolCoverage(symbol)
	entry, found := environment.CoverageManifest().Entry(subject)
	if !found {
		return newGround(GroundKindMissing, GroundMissingBasis, scopeID, kindID.String(), nil)
	}
	switch entry.Posture() {
	case typedmemory.CoverageUnsupported:
		return newGround(
			GroundKindExplicitlyUnsupported,
			GroundContradicted,
			scopeID,
			kindID.String(),
			nil,
		)
	case typedmemory.CoverageSourceOnly:
		return newGround(GroundKindSourceOnly, GroundMissingBasis, scopeID, kindID.String(), nil)
	case typedmemory.CoverageCompiled:
		return newGround(
			GroundKindDefinitionWithoutContext,
			GroundContradicted,
			scopeID,
			kindID.String(),
			nil,
		)
	default:
		return newGround(GroundKindMissing, GroundMissingBasis, scopeID, kindID.String(), nil)
	}
}

func contextsForKind(
	environment typedmemory.TypeEnv,
	kindID typedmemory.KindID,
) []string {
	contexts := []string{}
	for _, availability := range environment.ContextKindAvailabilities() {
		if availability.KindID() == kindID {
			contexts = append(contexts, availability.Context().String())
		}
	}
	slices.Sort(contexts)
	return slices.Compact(contexts)
}

func assessPatterns(
	scope projectprofile.NonSoftwareRealization,
	environment typedmemory.TypeEnv,
) []Ground {
	grounds := []Ground{}
	for _, ref := range scope.GoverningPatternRefs() {
		entries := matchingPatternCoverage(ref, environment.CoverageManifest())
		grounds = append(grounds, patternGround(scope.ScopeID().String(), ref, entries))
	}
	return grounds
}

func matchingPatternCoverage(
	ref projectprofile.SourceUnitRef,
	manifest typedmemory.CoverageManifest,
) []typedmemory.CoverageEntry {
	entries := []typedmemory.CoverageEntry{}
	for _, entry := range manifest.Entries() {
		subjectUnit, sourceUnitSubject := entry.Subject().SourceUnitID()
		if !sourceUnitSubject {
			continue
		}
		source := entry.Source()
		unitMatches := subjectUnit.String() == ref.String()
		pattern, hasPattern := source.PatternID()
		patternMatches := hasPattern && pattern.String() == ref.String()
		if unitMatches || patternMatches {
			entries = append(entries, entry)
		}
	}
	return entries
}

func patternGround(
	scopeID string,
	ref projectprofile.SourceUnitRef,
	entries []typedmemory.CoverageEntry,
) Ground {
	postures := map[typedmemory.CoveragePosture]bool{}
	for _, entry := range entries {
		postures[entry.Posture()] = true
	}
	if postures[typedmemory.CoverageUnsupported] {
		return newGround(
			GroundPatternExplicitlyUnsupported,
			GroundContradicted,
			scopeID,
			ref.String(),
			nil,
		)
	}
	if postures[typedmemory.CoverageSourceOnly] {
		return newGround(
			GroundPatternSourceOnly,
			GroundMissingBasis,
			scopeID,
			ref.String(),
			nil,
		)
	}
	if postures[typedmemory.CoverageCompiled] {
		return newGround(GroundPatternCompiled, GroundSatisfied, scopeID, ref.String(), nil)
	}
	return newGround(GroundPatternMissing, GroundMissingBasis, scopeID, ref.String(), nil)
}

func assessContracts(scope projectprofile.NonSoftwareRealization) []Ground {
	grounds := []Ground{}
	for _, ref := range scope.ContractRefs() {
		grounds = append(grounds, newGround(
			GroundContractIndexUnavailable,
			GroundMissingBasis,
			scope.ScopeID().String(),
			ref.String(),
			nil,
		))
	}
	return grounds
}

func selectVariant(grounds []Ground) string {
	for _, ground := range grounds {
		if ground.posture == GroundContradicted {
			return "incompatible"
		}
	}
	for _, ground := range grounds {
		if ground.posture == GroundMissingBasis {
			return "underdetermined"
		}
	}
	return "compatible"
}

type groundJSON struct {
	Kind       string   `json:"kind"`
	Posture    string   `json:"posture"`
	ScopeID    string   `json:"scope_id,omitempty"`
	Coordinate string   `json:"coordinate,omitempty"`
	Contexts   []string `json:"contexts,omitempty"`
}

type assessmentJSON struct {
	Schema               string       `json:"schema"`
	Variant              string       `json:"variant"`
	BasisRef             string       `json:"basis_ref"`
	BasisDigest          string       `json:"basis_digest"`
	TargetTypeEnvRef     string       `json:"target_typeenv_ref"`
	TargetSnapshotDigest string       `json:"target_snapshot_digest"`
	RuleEdition          string       `json:"rule_edition"`
	Grounds              []groundJSON `json:"grounds"`
}

// DecodeCanonicalAssessment reconstructs one persisted exact assessment. It
// rejects unknown fields, trailing input, unsupported variants, non-canonical
// ground order, and any byte sequence that does not round-trip exactly.
func DecodeCanonicalAssessment(raw []byte) (Assessment, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("canonical profile-fit assessment is required")
	}
	if len(raw) > maximumAssessmentBytes {
		return nil, fmt.Errorf("canonical profile-fit assessment exceeds the supported bound")
	}
	dto := assessmentJSON{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return nil, fmt.Errorf("decode canonical profile-fit assessment: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("canonical profile-fit assessment has trailing JSON")
		}
		return nil, fmt.Errorf("decode trailing profile-fit assessment input: %w", err)
	}
	if dto.Schema != assessmentSchemaV1 {
		return nil, fmt.Errorf("unsupported profile-fit assessment schema %q", dto.Schema)
	}
	if !validAssessmentVariant(dto.Variant) {
		return nil, fmt.Errorf("unsupported profile-fit assessment variant %q", dto.Variant)
	}
	if len(dto.Grounds) == 0 || len(dto.Grounds) > maximumAssessmentGrounds {
		return nil, fmt.Errorf("canonical profile-fit assessment ground count is invalid")
	}
	basisRef, err := projecttypeenvprofilebasis.ParseProjectProfileBasisRef(dto.BasisRef)
	if err != nil {
		return nil, fmt.Errorf("parse profile-fit basis ref: %w", err)
	}
	basisDigest, err := typedmemory.NewSHA256Digest(dto.BasisDigest)
	if err != nil {
		return nil, fmt.Errorf("parse profile-fit basis digest: %w", err)
	}
	if basisRef.Digest() != basisDigest {
		return nil, fmt.Errorf("profile-fit basis ref and digest differ")
	}
	targetRef, err := typedmemory.ParseTypeEnvRef(dto.TargetTypeEnvRef)
	if err != nil {
		return nil, fmt.Errorf("parse profile-fit target C: %w", err)
	}
	targetDigest, err := typedmemory.NewSHA256Digest(dto.TargetSnapshotDigest)
	if err != nil {
		return nil, fmt.Errorf("parse profile-fit target snapshot digest: %w", err)
	}
	edition, err := NewRuleEdition(dto.RuleEdition)
	if err != nil {
		return nil, err
	}
	grounds, err := decodeGrounds(dto.Grounds, edition)
	if err != nil {
		return nil, err
	}
	if !validVariantGrounds(dto.Variant, grounds) {
		return nil, fmt.Errorf(
			"profile-fit assessment variant contradicts its exact grounds",
		)
	}
	canonicalDTO, err := json.Marshal(dto)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonicalDTO, raw) {
		return nil, fmt.Errorf("profile-fit assessment is not canonical")
	}
	digest, err := digestBytes(raw)
	if err != nil {
		return nil, err
	}
	ref, err := ParseProjectTypeEnvProfileFitRef(
		"project-typeenv-profile-fit:" + digest.String(),
	)
	if err != nil {
		return nil, err
	}
	state := assessmentState{
		basisRef:     basisRef,
		basisDigest:  basisDigest,
		targetRef:    targetRef,
		targetDigest: targetDigest,
		edition:      edition,
		digest:       digest,
		ref:          ref,
		grounds:      grounds,
		canonical:    append([]byte(nil), raw...),
	}
	result, err := assessmentForVariant(dto.Variant, state)
	if err != nil {
		return nil, err
	}
	if err := result.Verify(); err != nil {
		return nil, err
	}
	return result, nil
}

func decodeGrounds(
	values []groundJSON,
	edition RuleEdition,
) ([]Ground, error) {
	grounds := make([]Ground, 0, len(values))
	for index, value := range values {
		ground, err := decodeGround(value, edition)
		if err != nil {
			return nil, fmt.Errorf("profile-fit ground %d: %w", index, err)
		}
		grounds = append(grounds, ground)
	}
	canonical := canonicalGrounds(grounds)
	if !bytes.Equal(
		mustMarshalGrounds(values),
		mustMarshalGrounds(groundJSONs(canonical)),
	) {
		return nil, fmt.Errorf("profile-fit grounds are not canonical")
	}
	return canonical, nil
}

func decodeGround(value groundJSON, edition RuleEdition) (Ground, error) {
	kind, err := parseGroundKind(value.Kind)
	if err != nil {
		return Ground{}, err
	}
	posture, err := parseGroundPosture(value.Posture)
	if err != nil {
		return Ground{}, err
	}
	ground := newGround(
		kind,
		posture,
		value.ScopeID,
		value.Coordinate,
		value.Contexts,
	)
	if err := validateDecodedGround(ground, edition); err != nil {
		return Ground{}, err
	}
	if !bytes.Equal(
		mustMarshalGrounds([]groundJSON{value}),
		mustMarshalGrounds(groundJSONs([]Ground{ground})),
	) {
		return Ground{}, fmt.Errorf("ground is not canonical")
	}
	return ground, nil
}

func parseGroundKind(raw string) (GroundKind, error) {
	for candidate := GroundNoCanonicalProfile; candidate <= GroundAssessorEditionUnavailable; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("profile-fit ground kind %q is invalid", raw)
}

func parseGroundPosture(raw string) (GroundPosture, error) {
	for candidate := GroundSatisfied; candidate <= GroundUnavailable; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("profile-fit ground posture %q is invalid", raw)
}

func validateDecodedGround(ground Ground, edition RuleEdition) error {
	if expectedPosture(ground.kind) != ground.posture {
		return fmt.Errorf("ground kind and posture are inconsistent")
	}
	switch ground.kind {
	case GroundNoCanonicalProfile:
		if ground.scopeID != "" ||
			ground.coordinate != "current-canonical-project-profile" ||
			len(ground.contexts) != 0 {
			return fmt.Errorf("exact profile-absence ground is malformed")
		}
	case GroundAssessorEditionUnavailable:
		if ground.scopeID != "" ||
			ground.coordinate != edition.String() ||
			len(ground.contexts) != 0 {
			return fmt.Errorf("assessor-edition ground is malformed")
		}
	case GroundSoftwareScope:
		if err := validateScopeGround(ground, false); err != nil {
			return err
		}
		if ground.coordinate != "software_realization" {
			return fmt.Errorf("software-scope ground coordinate is invalid")
		}
	case GroundKindOrientationUnspecified:
		if err := validateScopeGround(ground, false); err != nil {
			return err
		}
		if ground.coordinate != "kind_orientation" {
			return fmt.Errorf("unspecified-kind ground coordinate is invalid")
		}
	case GroundKindAvailable:
		if err := validateKindGround(ground, true); err != nil {
			return err
		}
	case GroundKindCoordinateUnsupported,
		GroundKindDefinitionWithoutContext,
		GroundKindSourceOnly,
		GroundKindExplicitlyUnsupported,
		GroundKindMissing:
		if err := validateKindGround(ground, false); err != nil {
			return err
		}
	case GroundPatternCompiled,
		GroundPatternSourceOnly,
		GroundPatternExplicitlyUnsupported,
		GroundPatternMissing:
		if err := validateScopeGround(ground, false); err != nil {
			return err
		}
		if _, err := projectprofile.NewSourceUnitRef(ground.coordinate); err != nil {
			return fmt.Errorf("governing-pattern ground coordinate: %w", err)
		}
	case GroundContractIndexUnavailable:
		if err := validateScopeGround(ground, false); err != nil {
			return err
		}
		if _, err := projectprofile.NewSpecSectionRef(ground.coordinate); err != nil {
			return fmt.Errorf("contract ground coordinate: %w", err)
		}
	default:
		return fmt.Errorf("unknown profile-fit ground kind")
	}
	return nil
}

func expectedPosture(kind GroundKind) GroundPosture {
	switch kind {
	case GroundSoftwareScope, GroundKindAvailable, GroundPatternCompiled:
		return GroundSatisfied
	case GroundKindCoordinateUnsupported,
		GroundKindDefinitionWithoutContext,
		GroundKindExplicitlyUnsupported,
		GroundPatternExplicitlyUnsupported:
		return GroundContradicted
	case GroundNoCanonicalProfile,
		GroundKindOrientationUnspecified,
		GroundKindSourceOnly,
		GroundKindMissing,
		GroundPatternSourceOnly,
		GroundPatternMissing,
		GroundContractIndexUnavailable:
		return GroundMissingBasis
	case GroundAssessorEditionUnavailable:
		return GroundUnavailable
	default:
		return 0
	}
}

func validateScopeGround(ground Ground, contextsRequired bool) error {
	if _, err := projectprofile.NewScopeID(ground.scopeID); err != nil {
		return fmt.Errorf("profile-fit ground scope: %w", err)
	}
	if ground.coordinate == "" {
		return fmt.Errorf("profile-fit ground coordinate is required")
	}
	if contextsRequired && len(ground.contexts) == 0 {
		return fmt.Errorf("profile-fit ground contexts are required")
	}
	if !contextsRequired && len(ground.contexts) != 0 {
		return fmt.Errorf("profile-fit ground carries unexpected contexts")
	}
	for _, raw := range ground.contexts {
		contextRef, err := typedmemory.NewBoundedContextRef(raw)
		if err != nil || contextRef.String() != raw {
			return fmt.Errorf("profile-fit ground context %q is invalid", raw)
		}
	}
	return nil
}

func validateKindGround(ground Ground, contextsRequired bool) error {
	if err := validateScopeGround(ground, contextsRequired); err != nil {
		return err
	}
	if ground.kind == GroundKindCoordinateUnsupported {
		if _, err := projectprofile.NewKindRef(ground.coordinate); err != nil {
			return fmt.Errorf("unsupported kind coordinate is not a profile KindRef: %w", err)
		}
		return nil
	}
	kindID, err := typedmemory.NewKindID(ground.coordinate)
	if err != nil || kindID.String() != ground.coordinate {
		return fmt.Errorf("profile-fit ground kind coordinate %q is invalid", ground.coordinate)
	}
	return nil
}

func validAssessmentVariant(variant string) bool {
	return variant == "compatible" ||
		variant == "incompatible" ||
		variant == "underdetermined" ||
		variant == "unavailable"
}

func mintAssessment(
	variant string,
	basis projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	targetRef typedmemory.TypeEnvRef,
	targetDigest typedmemory.SHA256Digest,
	edition RuleEdition,
	grounds []Ground,
) (Assessment, error) {
	canonical := canonicalGrounds(grounds)
	dtoGrounds := make([]groundJSON, 0, len(canonical))
	for _, ground := range canonical {
		dtoGrounds = append(dtoGrounds, groundJSON{
			Kind:       ground.kind.String(),
			Posture:    ground.posture.String(),
			ScopeID:    ground.scopeID,
			Coordinate: ground.coordinate,
			Contexts:   append([]string(nil), ground.contexts...),
		})
	}
	dto := assessmentJSON{
		Schema:               assessmentSchemaV1,
		Variant:              variant,
		BasisRef:             basis.ProfileBasisRef().String(),
		BasisDigest:          basis.Digest().String(),
		TargetTypeEnvRef:     targetRef.String(),
		TargetSnapshotDigest: targetDigest.String(),
		RuleEdition:          edition.String(),
		Grounds:              dtoGrounds,
	}
	canonicalBytes, err := json.Marshal(dto)
	if err != nil {
		return nil, err
	}
	digest, err := digestBytes(canonicalBytes)
	if err != nil {
		return nil, err
	}
	ref, err := ParseProjectTypeEnvProfileFitRef(
		"project-typeenv-profile-fit:" + digest.String(),
	)
	if err != nil {
		return nil, err
	}
	state := assessmentState{
		basisRef:     basis.ProfileBasisRef(),
		basisDigest:  basis.Digest(),
		targetRef:    targetRef,
		targetDigest: targetDigest,
		edition:      edition,
		digest:       digest,
		ref:          ref,
		grounds:      canonical,
		canonical:    canonicalBytes,
	}
	return assessmentForVariant(variant, state)
}

func assessmentForVariant(
	variant string,
	state assessmentState,
) (Assessment, error) {
	switch variant {
	case "compatible":
		return Compatible{state: state}, nil
	case "incompatible":
		return Incompatible{state: state}, nil
	case "underdetermined":
		return Underdetermined{state: state}, nil
	case "unavailable":
		return Unavailable{state: state}, nil
	default:
		return nil, fmt.Errorf("unknown profile-fit assessment variant %q", variant)
	}
}

func verifyAssessmentState(variant string, state assessmentState) error {
	dto := assessmentJSON{}
	if err := json.Unmarshal(state.canonical, &dto); err != nil {
		return fmt.Errorf("decode canonical project TypeEnv profile-fit assessment: %w", err)
	}
	if dto.Schema != assessmentSchemaV1 || dto.Variant != variant {
		return fmt.Errorf("project TypeEnv profile-fit assessment schema or variant mismatch")
	}
	digest, err := digestBytes(state.canonical)
	if err != nil {
		return err
	}
	ref, err := ParseProjectTypeEnvProfileFitRef(
		"project-typeenv-profile-fit:" + digest.String(),
	)
	if err != nil {
		return err
	}
	valid := dto.BasisRef == state.basisRef.String()
	valid = valid && dto.BasisDigest == state.basisDigest.String()
	valid = valid && dto.TargetTypeEnvRef == state.targetRef.String()
	valid = valid && dto.TargetSnapshotDigest == state.targetDigest.String()
	valid = valid && dto.RuleEdition == state.edition.String()
	valid = valid && digest == state.digest && ref == state.ref
	valid = valid && bytes.Equal(state.canonical, mustMarshal(dto))
	valid = valid && bytes.Equal(
		mustMarshalGrounds(dto.Grounds),
		mustMarshalGrounds(groundJSONs(state.grounds)),
	)
	valid = valid && validVariantGrounds(variant, state.grounds)
	if !valid {
		return fmt.Errorf("project TypeEnv profile-fit assessment fields differ from canonical bytes")
	}
	return nil
}

func validVariantGrounds(variant string, grounds []Ground) bool {
	hasContradicted := false
	hasMissing := false
	hasUnavailable := false
	for _, ground := range grounds {
		hasContradicted = hasContradicted || ground.posture == GroundContradicted
		hasMissing = hasMissing || ground.posture == GroundMissingBasis
		hasUnavailable = hasUnavailable || ground.posture == GroundUnavailable
	}
	switch variant {
	case "compatible":
		return !hasContradicted && !hasMissing && !hasUnavailable
	case "incompatible":
		return hasContradicted && !hasUnavailable
	case "underdetermined":
		return !hasContradicted && hasMissing && !hasUnavailable
	case "unavailable":
		return !hasContradicted && !hasMissing && hasUnavailable
	default:
		return false
	}
}

func groundJSONs(grounds []Ground) []groundJSON {
	values := make([]groundJSON, 0, len(grounds))
	for _, ground := range grounds {
		values = append(values, groundJSON{
			Kind:       ground.kind.String(),
			Posture:    ground.posture.String(),
			ScopeID:    ground.scopeID,
			Coordinate: ground.coordinate,
			Contexts:   append([]string(nil), ground.contexts...),
		})
	}
	return values
}

func newGround(
	kind GroundKind,
	posture GroundPosture,
	scopeID string,
	coordinate string,
	contexts []string,
) Ground {
	ownedContexts := append([]string(nil), contexts...)
	slices.Sort(ownedContexts)
	ownedContexts = slices.Compact(ownedContexts)
	return Ground{
		kind:       kind,
		posture:    posture,
		scopeID:    scopeID,
		coordinate: coordinate,
		contexts:   ownedContexts,
	}
}

func canonicalGrounds(values []Ground) []Ground {
	result := cloneGrounds(values)
	sort.Slice(result, func(left int, right int) bool {
		return groundKey(result[left]) < groundKey(result[right])
	})
	return slices.CompactFunc(result, func(left Ground, right Ground) bool {
		return groundKey(left) == groundKey(right)
	})
}

func cloneGrounds(values []Ground) []Ground {
	result := make([]Ground, 0, len(values))
	for _, value := range values {
		clone := value
		clone.contexts = append([]string(nil), value.contexts...)
		result = append(result, clone)
	}
	return result
}

func groundKey(ground Ground) string {
	return strings.Join([]string{
		ground.posture.String(),
		ground.kind.String(),
		ground.scopeID,
		ground.coordinate,
		strings.Join(ground.contexts, "\x1f"),
	}, "\x1e")
}

func digestBytes(canonical []byte) (typedmemory.SHA256Digest, error) {
	hasher := sha256.New()
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(assessmentDigestDomain)))
	_, _ = hasher.Write(size[:])
	_, _ = hasher.Write([]byte(assessmentDigestDomain))
	binary.BigEndian.PutUint64(size[:], uint64(len(canonical)))
	_, _ = hasher.Write(size[:])
	_, _ = hasher.Write(canonical)
	raw := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	return typedmemory.NewSHA256Digest(raw)
}

func mustMarshal(value assessmentJSON) []byte {
	data, _ := json.Marshal(value)
	return data
}

func mustMarshalGrounds(value []groundJSON) []byte {
	data, _ := json.Marshal(value)
	return data
}
