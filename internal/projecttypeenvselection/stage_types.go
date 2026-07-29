package projecttypeenvselection

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectTypeEnvStageRefPrefix        = "project-typeenv-stage:"
	noPriorHeadProofRefPrefix           = "project-typeenv-no-prior-head-proof:"
	projectTypeEnvHeadRefPrefix         = "project-typeenv-head:"
	projectTypeEnvStageProvenancePrefix = "project-typeenv-stage-provenance:"
	projectTypeEnvCompatibilityPrefix   = "project-typeenv-compatibility-diff:"
	existingAssertionRevalidationPrefix = "existing-assertion-revalidation:"

	ProjectTypeEnvStageSchemaEditionV2 = "haft.project-typeenv.stage-schema/v2"
	ProjectTypeEnvStageSchemaEditionV3 = "haft.project-typeenv.stage-schema/v3"
	ProjectTypeEnvStageSchemaEditionV4 = "haft.project-typeenv.stage-schema/v4"
	ProjectTypeEnvStageSchemaEditionV5 = "haft.project-typeenv.stage-schema/v5"

	maximumStageCoordinateBytes = 16 << 10
	maximumStageExtensions      = 4096
)

// ProjectTypeEnvStageRef is derived exclusively from exact canonical Stage
// bytes. It is not a database-row identity, authority receipt, or project head.
type ProjectTypeEnvStageRef struct{ digest typedmemory.SHA256Digest }

func ParseProjectTypeEnvStageRef(raw string) (ProjectTypeEnvStageRef, error) {
	digest, err := parseStageDigestRef("Stage", projectTypeEnvStageRefPrefix, raw)
	if err != nil {
		return ProjectTypeEnvStageRef{}, err
	}
	return ProjectTypeEnvStageRef{digest: digest}, nil
}

func (ref ProjectTypeEnvStageRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref ProjectTypeEnvStageRef) String() string {
	return projectTypeEnvStageRefPrefix + ref.digest.String()
}

type ProjectTypeEnvCompatibilityDiffRef struct{ digest typedmemory.SHA256Digest }

func ParseProjectTypeEnvCompatibilityDiffRef(
	raw string,
) (ProjectTypeEnvCompatibilityDiffRef, error) {
	digest, err := parseStageDigestRef(
		"project TypeEnv compatibility diff",
		projectTypeEnvCompatibilityPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvCompatibilityDiffRef{}, err
	}
	return ProjectTypeEnvCompatibilityDiffRef{digest: digest}, nil
}

func (ref ProjectTypeEnvCompatibilityDiffRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvCompatibilityDiffRef) String() string {
	return projectTypeEnvCompatibilityPrefix + ref.digest.String()
}

type ExistingAssertionRevalidationRef struct{ digest typedmemory.SHA256Digest }

func ParseExistingAssertionRevalidationRef(
	raw string,
) (ExistingAssertionRevalidationRef, error) {
	digest, err := parseStageDigestRef(
		"existing-assertion revalidation",
		existingAssertionRevalidationPrefix,
		raw,
	)
	if err != nil {
		return ExistingAssertionRevalidationRef{}, err
	}
	return ExistingAssertionRevalidationRef{digest: digest}, nil
}

func (ref ExistingAssertionRevalidationRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ExistingAssertionRevalidationRef) String() string {
	return existingAssertionRevalidationPrefix + ref.digest.String()
}

type ProjectTypeEnvProfileFitRef = projecttypeenvprofilefit.ProjectTypeEnvProfileFitRef

func ParseProjectTypeEnvProfileFitRef(raw string) (ProjectTypeEnvProfileFitRef, error) {
	return projecttypeenvprofilefit.ParseProjectTypeEnvProfileFitRef(raw)
}

// NoPriorHeadProofRef addresses an exact independently verified absence proof.
// Merely parsing this coordinate does not prove that a project has no head.
type NoPriorHeadProofRef struct{ digest typedmemory.SHA256Digest }

func ParseNoPriorHeadProofRef(raw string) (NoPriorHeadProofRef, error) {
	digest, err := parseStageDigestRef("no-prior-head proof", noPriorHeadProofRefPrefix, raw)
	if err != nil {
		return NoPriorHeadProofRef{}, err
	}
	return NoPriorHeadProofRef{digest: digest}, nil
}

func (ref NoPriorHeadProofRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref NoPriorHeadProofRef) String() string {
	return noPriorHeadProofRefPrefix + ref.digest.String()
}

// ProjectTypeEnvHeadRef is the stable identity of one project's head slot. It
// is intentionally not content-addressed: the slot survives every selection,
// while HeadRevision and the selected composite identify its changing state.
type ProjectTypeEnvHeadRef struct{ project projectidentity.ProjectID }

func ProjectTypeEnvHeadRefForProject(
	project projectidentity.ProjectID,
) (ProjectTypeEnvHeadRef, error) {
	parsed, err := projectidentity.ParseProjectID(project.String())
	if err != nil || parsed != project {
		return ProjectTypeEnvHeadRef{}, fmt.Errorf("project TypeEnv head project is required")
	}
	return ProjectTypeEnvHeadRef{project: parsed}, nil
}

func ParseProjectTypeEnvHeadRef(raw string) (ProjectTypeEnvHeadRef, error) {
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, projectTypeEnvHeadRefPrefix) {
		return ProjectTypeEnvHeadRef{}, fmt.Errorf(
			"project TypeEnv head ref must start with %q",
			projectTypeEnvHeadRefPrefix,
		)
	}
	projectText := strings.TrimPrefix(raw, projectTypeEnvHeadRefPrefix)
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return ProjectTypeEnvHeadRef{}, fmt.Errorf("project TypeEnv head project: %w", err)
	}
	return ProjectTypeEnvHeadRef{project: project}, nil
}

func (ref ProjectTypeEnvHeadRef) Project() projectidentity.ProjectID { return ref.project }

func (ref ProjectTypeEnvHeadRef) String() string {
	return projectTypeEnvHeadRefPrefix + ref.project.String()
}

// HeadRevision counts successful head selections. It is deliberately not a
// typedmemory.GraphRevision and has no conversion to one.
type HeadRevision struct{ value uint64 }

func NewHeadRevision(value uint64) (HeadRevision, error) {
	if value == 0 {
		return HeadRevision{}, fmt.Errorf("head revision must be greater than zero")
	}
	return HeadRevision{value: value}, nil
}

func (revision HeadRevision) Value() uint64 { return revision.value }

// ProjectTypeEnvStagePredecessor is a closed sum. Genesis is the tag-only
// no-prior-head posture; Transition carries one exact prior head.
type ProjectTypeEnvStagePredecessor interface {
	projectTypeEnvStagePredecessorVariant()
}

type GenesisStagePredecessor struct{}

func NewGenesisStagePredecessor() GenesisStagePredecessor {
	return GenesisStagePredecessor{}
}

func (GenesisStagePredecessor) projectTypeEnvStagePredecessorVariant() {}

// legacyGenesisStagePredecessor exists only to authenticate historical Stage
// v2 and head-selection Request v1 bytes. No public constructor or accessor
// can create a new proof-linked Genesis posture.
type legacyGenesisStagePredecessor struct {
	noPriorHeadProof NoPriorHeadProofRef
}

func newLegacyGenesisStagePredecessor(
	proof NoPriorHeadProofRef,
) (legacyGenesisStagePredecessor, error) {
	parsed, err := ParseNoPriorHeadProofRef(proof.String())
	if err != nil || parsed != proof {
		return legacyGenesisStagePredecessor{}, fmt.Errorf(
			"legacy genesis no-prior-head proof is required",
		)
	}
	return legacyGenesisStagePredecessor{noPriorHeadProof: proof}, nil
}

func (legacyGenesisStagePredecessor) projectTypeEnvStagePredecessorVariant() {}

// LegacyGenesisNoPriorHeadProof exposes the historical proof coordinate needed
// to authenticate decoded Stage v2 and Request v1 records. It cannot construct
// a legacy predecessor and always abstains for current tag-only Genesis.
func LegacyGenesisNoPriorHeadProof(
	predecessor ProjectTypeEnvHeadSelectionPredecessor,
) (NoPriorHeadProofRef, bool) {
	legacy, ok := predecessor.(legacyGenesisStagePredecessor)
	if !ok {
		return NoPriorHeadProofRef{}, false
	}
	return legacy.noPriorHeadProof, true
}

type TransitionStagePredecessor struct {
	project           projectidentity.ProjectID
	head              ProjectTypeEnvHeadRef
	headRevision      HeadRevision
	selectedComposite typedmemory.TypeEnvRef
}

type TransitionStagePredecessorInput struct {
	Project           projectidentity.ProjectID
	Head              ProjectTypeEnvHeadRef
	HeadRevision      HeadRevision
	SelectedComposite typedmemory.TypeEnvRef
}

func NewTransitionStagePredecessor(
	input TransitionStagePredecessorInput,
) (TransitionStagePredecessor, error) {
	project, err := projectidentity.ParseProjectID(input.Project.String())
	if err != nil || project != input.Project {
		return TransitionStagePredecessor{}, fmt.Errorf("transition prior-head project is required")
	}
	head, err := ParseProjectTypeEnvHeadRef(input.Head.String())
	if err != nil || head != input.Head {
		return TransitionStagePredecessor{}, fmt.Errorf("transition prior head is required")
	}
	if head.Project() != project {
		return TransitionStagePredecessor{}, fmt.Errorf("transition prior head project mismatch")
	}
	revision, err := NewHeadRevision(input.HeadRevision.Value())
	if err != nil || revision != input.HeadRevision {
		return TransitionStagePredecessor{}, fmt.Errorf("transition head revision is required")
	}
	composite, err := typedmemory.ParseTypeEnvRef(input.SelectedComposite.String())
	if err != nil || composite != input.SelectedComposite {
		return TransitionStagePredecessor{}, fmt.Errorf("transition selected composite is required")
	}
	return TransitionStagePredecessor{
		project:           project,
		head:              head,
		headRevision:      revision,
		selectedComposite: composite,
	}, nil
}

func (predecessor TransitionStagePredecessor) Project() projectidentity.ProjectID {
	return predecessor.project
}

func (predecessor TransitionStagePredecessor) Head() ProjectTypeEnvHeadRef {
	return predecessor.head
}

func (predecessor TransitionStagePredecessor) HeadRevision() HeadRevision {
	return predecessor.headRevision
}

func (predecessor TransitionStagePredecessor) SelectedComposite() typedmemory.TypeEnvRef {
	return predecessor.selectedComposite
}

func (TransitionStagePredecessor) projectTypeEnvStagePredecessorVariant() {}

// ProjectTypeEnvStageCompatibility is aligned with predecessor posture:
// Initial is legal only for Genesis and Compared only for Transition.
type ProjectTypeEnvStageCompatibility interface {
	projectTypeEnvStageCompatibilityVariant()
}

// InitialStageCompatibility is the exact Genesis posture. It binds the target
// executable C even though there is no predecessor C to compare.
type InitialStageCompatibility struct {
	target typedmemory.TypeEnvRef
}

func NewInitialStageCompatibility(
	target typedmemory.TypeEnvRef,
) (InitialStageCompatibility, error) {
	parsed, err := typedmemory.ParseTypeEnvRef(target.String())
	if err != nil || parsed != target {
		return InitialStageCompatibility{}, fmt.Errorf(
			"initial compatibility target TypeEnv is required",
		)
	}
	return InitialStageCompatibility{target: target}, nil
}

func (compatibility InitialStageCompatibility) Target() typedmemory.TypeEnvRef {
	return compatibility.target
}

func (InitialStageCompatibility) projectTypeEnvStageCompatibilityVariant() {}

type ComparedStageCompatibility struct {
	diff projecttypeenvcompatibility.Diff
}

func NewComparedStageCompatibility(
	diff projecttypeenvcompatibility.Diff,
) (ComparedStageCompatibility, error) {
	if err := diff.Verify(); err != nil {
		return ComparedStageCompatibility{}, fmt.Errorf(
			"verify exact executable TypeEnv compatibility diff: %w",
			err,
		)
	}
	decoded, err := projecttypeenvcompatibility.DecodeDiff(diff.CanonicalBytes())
	if err != nil {
		return ComparedStageCompatibility{}, fmt.Errorf(
			"restore exact executable TypeEnv compatibility diff: %w",
			err,
		)
	}
	return ComparedStageCompatibility{diff: decoded}, nil
}

func (compatibility ComparedStageCompatibility) Base() typedmemory.TypeEnvRef {
	return compatibility.diff.Base()
}

func (compatibility ComparedStageCompatibility) Target() typedmemory.TypeEnvRef {
	return compatibility.diff.Target()
}

func (compatibility ComparedStageCompatibility) Diff() projecttypeenvcompatibility.Diff {
	return compatibility.diff
}

func (ComparedStageCompatibility) projectTypeEnvStageCompatibilityVariant() {}

// ProjectProfileBasisRef and ProjectTypeEnvProfileCompatibility are aliases of
// the lower canonical profile contracts. Stage stores the complete assessment
// bytes and never maintains a second posture or free-text reason vocabulary.
type ProjectProfileBasisRef = projecttypeenvprofilebasis.ProjectProfileBasisRef

func ParseProjectProfileBasisRef(raw string) (ProjectProfileBasisRef, error) {
	return projecttypeenvprofilebasis.ParseProjectProfileBasisRef(raw)
}

type ProjectTypeEnvProfileCompatibility = projecttypeenvprofilefit.Assessment

type StageCompilerEdition struct{ value string }

func NewStageCompilerEdition(raw string) (StageCompilerEdition, error) {
	value, err := normalizeStageText("Stage compiler edition", raw)
	if err != nil {
		return StageCompilerEdition{}, err
	}
	return StageCompilerEdition{value: value}, nil
}

func (edition StageCompilerEdition) String() string { return edition.value }

type StageProducerEdition struct{ value string }

func NewStageProducerEdition(raw string) (StageProducerEdition, error) {
	value, err := normalizeStageText("Stage producer edition", raw)
	if err != nil {
		return StageProducerEdition{}, err
	}
	return StageProducerEdition{value: value}, nil
}

func (edition StageProducerEdition) String() string { return edition.value }

type StageRevalidatorEdition struct{ value string }

func NewStageRevalidatorEdition(raw string) (StageRevalidatorEdition, error) {
	value, err := normalizeStageText("Stage revalidator edition", raw)
	if err != nil {
		return StageRevalidatorEdition{}, err
	}
	return StageRevalidatorEdition{value: value}, nil
}

func (edition StageRevalidatorEdition) String() string { return edition.value }

// ProjectTypeEnvStageProvenanceRef is retained only to decode and inspect the
// orphan provenance coordinate carried by historical v2/v3 Stage bytes. New
// Stage sealing does not accept or emit this caller-supplied coordinate.
type ProjectTypeEnvStageProvenanceRef struct{ digest typedmemory.SHA256Digest }

func ParseProjectTypeEnvStageProvenanceRef(
	raw string,
) (ProjectTypeEnvStageProvenanceRef, error) {
	digest, err := parseStageDigestRef("Stage provenance", projectTypeEnvStageProvenancePrefix, raw)
	if err != nil {
		return ProjectTypeEnvStageProvenanceRef{}, err
	}
	return ProjectTypeEnvStageProvenanceRef{digest: digest}, nil
}

func (ref ProjectTypeEnvStageProvenanceRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvStageProvenanceRef) String() string {
	return projectTypeEnvStageProvenancePrefix + ref.digest.String()
}

func parseStageDigestRef(
	label string,
	prefix string,
	raw string,
) (typedmemory.SHA256Digest, error) {
	digestText, found := strings.CutPrefix(raw, prefix)
	if !found {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s reference is malformed", label)
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s reference: %w", label, err)
	}
	if prefix+digest.String() != raw {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s reference is not canonical", label)
	}
	return digest, nil
}

func normalizeTypeEnvRef(
	label string,
	ref typedmemory.TypeEnvRef,
) (typedmemory.TypeEnvRef, error) {
	parsed, err := typedmemory.ParseTypeEnvRef(ref.String())
	if err != nil || parsed != ref {
		return typedmemory.TypeEnvRef{}, fmt.Errorf("%s is required", label)
	}
	return parsed, nil
}

func normalizeRuntimeBasisRef(
	ref projecttypeenv.RuntimeEvaluationBasisRef,
) (projecttypeenv.RuntimeEvaluationBasisRef, error) {
	parsed, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(ref.String())
	if err != nil || parsed != ref {
		return projecttypeenv.RuntimeEvaluationBasisRef{}, fmt.Errorf("runtime evaluation basis is required")
	}
	return parsed, nil
}

func normalizeOrderedExtensionRefs(
	refs []typedmemory.TypeEnvExtensionRef,
) ([]typedmemory.TypeEnvExtensionRef, error) {
	if len(refs) > maximumStageExtensions {
		return nil, fmt.Errorf("ordered E DAG exceeds %d extensions", maximumStageExtensions)
	}
	owned := append([]typedmemory.TypeEnvExtensionRef(nil), refs...)
	seen := make(map[string]struct{}, len(owned))
	for index, ref := range owned {
		parsed, err := typedmemory.ParseTypeEnvExtensionRef(ref.String())
		if err != nil || parsed != ref {
			return nil, fmt.Errorf("ordered E DAG extension %d is invalid", index)
		}
		key := ref.String()
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("ordered E DAG contains duplicate extension %q", key)
		}
		seen[key] = struct{}{}
	}
	return owned, nil
}

func normalizeDigest(
	label string,
	digest typedmemory.SHA256Digest,
) (typedmemory.SHA256Digest, error) {
	parsed, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || parsed != digest {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s is required", label)
	}
	return parsed, nil
}

func normalizeStageText(label string, raw string) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("%s is required in canonical form", label)
	}
	if !utf8.ValidString(raw) || strings.ContainsFunc(raw, unicode.IsControl) {
		return "", fmt.Errorf("%s contains invalid text", label)
	}
	if len(raw) > maximumStageCoordinateBytes {
		return "", fmt.Errorf("%s exceeds %d bytes", label, maximumStageCoordinateBytes)
	}
	return raw, nil
}

func orderedExtensionRefsEqual(
	left []typedmemory.TypeEnvExtensionRef,
	right []typedmemory.TypeEnvExtensionRef,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
