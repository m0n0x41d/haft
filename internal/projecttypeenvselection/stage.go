package projecttypeenvselection

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvtransitioncompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectTypeEnvStageDomainV2     = "haft.project-typeenv.stage.v2"
	projectTypeEnvStageDomainV3     = "haft.project-typeenv.stage.v3"
	projectTypeEnvStageDomain       = "haft.project-typeenv.stage.v4"
	projectTypeEnvStageDomainV5     = "haft.project-typeenv.stage.v5"
	maximumProjectTypeEnvStageBytes = 32 << 20
)

// ProjectTypeEnvStage is a pure, immutable, content-addressed preparation over
// one exact project graph snapshot and one exact predecessor posture. It cannot
// create or select ProjectTypeEnvHead, grant authority, or make a signature
// available to admission.
type ProjectTypeEnvStage struct {
	ref                                ProjectTypeEnvStageRef
	project                            projectidentity.ProjectID
	predecessor                        ProjectTypeEnvStagePredecessor
	base                               typedmemory.TypeEnvRef
	extensions                         []typedmemory.TypeEnvExtensionRef
	runtimeBasis                       projecttypeenv.RuntimeEvaluationBasisRef
	verifiedComposite                  typedmemory.TypeEnvRef
	compositeVerificationRef           projecttypeenv.ProjectTypeEnvCompositeVerificationRef
	compositeVerificationDigest        typedmemory.SHA256Digest
	graphSnapshot                      ProjectGraphSnapshotBasisRef
	graphSnapshotDigest                typedmemory.SHA256Digest
	graphRevision                      typedmemory.GraphRevision
	profileLedgerRevision              projectprofile.LedgerRevision
	profileLedgerDigest                typedmemory.SHA256Digest
	compatibility                      ProjectTypeEnvStageCompatibility
	compatibilityRef                   ProjectTypeEnvCompatibilityDiffRef
	compatibilityDigest                typedmemory.SHA256Digest
	revalidation                       projecttypeenvassertionreport.Report
	revalidationRef                    ExistingAssertionRevalidationRef
	revalidationDigest                 typedmemory.SHA256Digest
	profileCompatibility               projecttypeenvprofilefit.Assessment
	profileFitRef                      ProjectTypeEnvProfileFitRef
	profileFitDigest                   typedmemory.SHA256Digest
	transitionProjectionProfiles       projecttypeenvtransitioncompatibility.Set
	transitionProjectionProfilesRef    projecttypeenvtransitioncompatibility.Ref
	transitionProjectionProfilesDigest typedmemory.SHA256Digest
	schemaEdition                      string
	compilerEdition                    StageCompilerEdition
	revalidatorEdition                 StageRevalidatorEdition
	producerEdition                    StageProducerEdition
	historicalProvenance               ProjectTypeEnvStageProvenanceRef
	canonicalBytes                     []byte
}

type ProjectTypeEnvStageInput struct {
	Project                                  projectidentity.ProjectID
	Predecessor                              ProjectTypeEnvStagePredecessor
	Base                                     typedmemory.TypeEnvRef
	OrderedExtensions                        []typedmemory.TypeEnvExtensionRef
	RuntimeBasis                             projecttypeenv.RuntimeEvaluationBasisRef
	VerifiedComposite                        projecttypeenv.ProjectTypeEnvCompositeVerification
	Composite                                typedmemory.TypeEnvRef
	GraphSnapshotBasis                       ProjectGraphSnapshotBasis
	GraphSnapshotBasisRef                    ProjectGraphSnapshotBasisRef
	GraphSnapshotBasisDigest                 typedmemory.SHA256Digest
	GraphRevision                            typedmemory.GraphRevision
	ProfileLedgerRevision                    projectprofile.LedgerRevision
	ProfileLedgerDigest                      typedmemory.SHA256Digest
	Compatibility                            ProjectTypeEnvStageCompatibility
	ExistingAssertionRevalidation            projecttypeenvassertionreport.Report
	ProfileCompatibility                     projecttypeenvprofilefit.Assessment
	TransitionProjectionProfileCompatibility projecttypeenvtransitioncompatibility.Set
}

type projectTypeEnvStageState struct {
	project                            projectidentity.ProjectID
	predecessor                        ProjectTypeEnvStagePredecessor
	base                               typedmemory.TypeEnvRef
	extensions                         []typedmemory.TypeEnvExtensionRef
	runtimeBasis                       projecttypeenv.RuntimeEvaluationBasisRef
	verifiedComposite                  typedmemory.TypeEnvRef
	compositeVerificationRef           projecttypeenv.ProjectTypeEnvCompositeVerificationRef
	compositeVerificationDigest        typedmemory.SHA256Digest
	graphSnapshot                      ProjectGraphSnapshotBasisRef
	graphSnapshotDigest                typedmemory.SHA256Digest
	graphRevision                      typedmemory.GraphRevision
	profileLedgerRevision              projectprofile.LedgerRevision
	profileLedgerDigest                typedmemory.SHA256Digest
	compatibility                      ProjectTypeEnvStageCompatibility
	compatibilityRef                   ProjectTypeEnvCompatibilityDiffRef
	compatibilityDigest                typedmemory.SHA256Digest
	revalidation                       projecttypeenvassertionreport.Report
	revalidationRef                    ExistingAssertionRevalidationRef
	revalidationDigest                 typedmemory.SHA256Digest
	profileCompatibility               projecttypeenvprofilefit.Assessment
	profileFitRef                      ProjectTypeEnvProfileFitRef
	profileFitDigest                   typedmemory.SHA256Digest
	transitionProjectionProfiles       projecttypeenvtransitioncompatibility.Set
	transitionProjectionProfilesRef    projecttypeenvtransitioncompatibility.Ref
	transitionProjectionProfilesDigest typedmemory.SHA256Digest
	schemaEdition                      string
	compilerEdition                    StageCompilerEdition
	revalidatorEdition                 StageRevalidatorEdition
	producerEdition                    StageProducerEdition
	historicalProvenance               ProjectTypeEnvStageProvenanceRef
}

func SealProjectTypeEnvStage(
	input ProjectTypeEnvStageInput,
) (ProjectTypeEnvStage, error) {
	if err := input.VerifiedComposite.Verify(); err != nil {
		return ProjectTypeEnvStage{}, fmt.Errorf(
			"verify Stage composite capability: %w",
			err,
		)
	}
	if err := input.GraphSnapshotBasis.Verify(); err != nil {
		return ProjectTypeEnvStage{}, fmt.Errorf("verify Stage project graph snapshot: %w", err)
	}
	if input.GraphSnapshotBasis.Ref() != input.GraphSnapshotBasisRef {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage graph snapshot reference mismatch")
	}
	if input.GraphSnapshotBasis.Ref().Digest() != input.GraphSnapshotBasisDigest {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage graph snapshot digest mismatch")
	}
	if input.GraphSnapshotBasis.Project() != input.Project {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage graph snapshot project mismatch")
	}
	if input.GraphSnapshotBasis.GraphRevision() != input.GraphRevision {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage graph snapshot revision mismatch")
	}
	schemaEdition, compilerEdition, revalidatorEdition, producerEdition :=
		stageEditionsForNewPredecessor(input.Predecessor)
	state := projectTypeEnvStageState{
		project:                      input.Project,
		predecessor:                  input.Predecessor,
		base:                         input.Base,
		extensions:                   input.OrderedExtensions,
		runtimeBasis:                 input.RuntimeBasis,
		verifiedComposite:            input.Composite,
		compositeVerificationRef:     input.VerifiedComposite.Ref(),
		compositeVerificationDigest:  input.VerifiedComposite.Digest(),
		graphSnapshot:                input.GraphSnapshotBasisRef,
		graphSnapshotDigest:          input.GraphSnapshotBasisDigest,
		graphRevision:                input.GraphRevision,
		profileLedgerRevision:        input.ProfileLedgerRevision,
		profileLedgerDigest:          input.ProfileLedgerDigest,
		compatibility:                input.Compatibility,
		revalidation:                 input.ExistingAssertionRevalidation,
		profileCompatibility:         input.ProfileCompatibility,
		transitionProjectionProfiles: input.TransitionProjectionProfileCompatibility,
		schemaEdition:                schemaEdition,
		compilerEdition:              compilerEdition,
		revalidatorEdition:           revalidatorEdition,
		producerEdition:              producerEdition,
	}
	normalized, err := normalizeProjectTypeEnvStageState(state)
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	if input.VerifiedComposite.BaseTypeEnvRef() != normalized.base {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage B does not match verified composite B")
	}
	verifiedExtensions := input.VerifiedComposite.ExtensionRefs()
	if !orderedExtensionRefsEqual(verifiedExtensions, normalized.extensions) {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage ordered E DAG does not match verified composite E DAG")
	}
	if input.VerifiedComposite.RuntimeEvaluationBasisRef() != normalized.runtimeBasis {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage X does not match verified composite X")
	}
	if input.VerifiedComposite.CompositeRef() != normalized.verifiedComposite {
		return ProjectTypeEnvStage{}, fmt.Errorf("Stage C does not match verified composite C")
	}
	canonical, err := encodeProjectTypeEnvStageState(normalized)
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	return DecodeProjectTypeEnvStage(canonical)
}

func stageEditionsForNewPredecessor(
	predecessor ProjectTypeEnvStagePredecessor,
) (string, StageCompilerEdition, StageRevalidatorEdition, StageProducerEdition) {
	if _, transition := predecessor.(TransitionStagePredecessor); transition {
		return ProjectTypeEnvStageSchemaEditionV5,
			StageCompilerEditionV5(),
			StageRevalidatorEditionV5(),
			StageProducerEditionV5()
	}
	return ProjectTypeEnvStageSchemaEditionV4,
		StageCompilerEditionV4(),
		StageRevalidatorEditionV4(),
		StageProducerEditionV4()
}

// DecodeProjectTypeEnvStage authenticates the immutable Stage record only. It
// does not recreate the non-serializable final-lowerer capability used to mint
// a new Stage; a reload service must restore that capability from exact
// B/E/X/C inputs before producing another Stage.
func DecodeProjectTypeEnvStage(canonical []byte) (ProjectTypeEnvStage, error) {
	if len(canonical) == 0 {
		return ProjectTypeEnvStage{}, fmt.Errorf("project TypeEnv Stage is empty")
	}
	if len(canonical) > maximumProjectTypeEnvStageBytes {
		return ProjectTypeEnvStage{}, fmt.Errorf(
			"project TypeEnv Stage exceeds %d bytes",
			maximumProjectTypeEnvStageBytes,
		)
	}
	reader := stageReader{value: canonical}
	domain, err := reader.readString("domain")
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	if domain != projectTypeEnvStageDomainV5 &&
		domain != projectTypeEnvStageDomain &&
		domain != projectTypeEnvStageDomainV3 &&
		domain != projectTypeEnvStageDomainV2 {
		return ProjectTypeEnvStage{}, fmt.Errorf("project TypeEnv Stage domain is invalid")
	}
	state, err := decodeProjectTypeEnvStageState(&reader, domain)
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	if reader.remaining() != 0 {
		return ProjectTypeEnvStage{}, fmt.Errorf("project TypeEnv Stage has trailing bytes")
	}
	normalized, err := normalizeProjectTypeEnvStageState(state)
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	reencoded, err := encodeProjectTypeEnvStageState(normalized)
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvStage{}, fmt.Errorf("project TypeEnv Stage is not canonical")
	}
	ref, err := deriveProjectTypeEnvStageRef(canonical)
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	return projectTypeEnvStageFromState(ref, normalized, canonical), nil
}

// VerifyProjectTypeEnvStage checks exact content identity. It is deliberately
// not a substitute for rerunning final lowering when a later effectful service
// needs a fresh trusted production capability.
func VerifyProjectTypeEnvStage(
	expected ProjectTypeEnvStageRef,
	canonical []byte,
) (ProjectTypeEnvStage, error) {
	parsed, err := ParseProjectTypeEnvStageRef(expected.String())
	if err != nil || parsed != expected {
		return ProjectTypeEnvStage{}, fmt.Errorf("expected project TypeEnv Stage reference is invalid")
	}
	stage, err := DecodeProjectTypeEnvStage(canonical)
	if err != nil {
		return ProjectTypeEnvStage{}, err
	}
	if stage.ref != expected {
		return ProjectTypeEnvStage{}, fmt.Errorf("project TypeEnv Stage reference mismatch")
	}
	return stage, nil
}

func (stage ProjectTypeEnvStage) Ref() ProjectTypeEnvStageRef { return stage.ref }

func (stage ProjectTypeEnvStage) Project() projectidentity.ProjectID { return stage.project }

func (stage ProjectTypeEnvStage) Predecessor() ProjectTypeEnvStagePredecessor {
	predecessor, _ := normalizeStagePredecessor(stage.predecessor)
	return predecessor
}

func (stage ProjectTypeEnvStage) Base() typedmemory.TypeEnvRef { return stage.base }

func (stage ProjectTypeEnvStage) OrderedExtensions() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), stage.extensions...)
}

func (stage ProjectTypeEnvStage) RuntimeBasis() projecttypeenv.RuntimeEvaluationBasisRef {
	return stage.runtimeBasis
}

func (stage ProjectTypeEnvStage) VerifiedComposite() typedmemory.TypeEnvRef {
	return stage.verifiedComposite
}

func (stage ProjectTypeEnvStage) CompositeVerificationRef() projecttypeenv.ProjectTypeEnvCompositeVerificationRef {
	return stage.compositeVerificationRef
}

func (stage ProjectTypeEnvStage) CompositeVerificationDigest() typedmemory.SHA256Digest {
	return stage.compositeVerificationDigest
}

func (stage ProjectTypeEnvStage) GraphSnapshotBasis() ProjectGraphSnapshotBasisRef {
	return stage.graphSnapshot
}

func (stage ProjectTypeEnvStage) GraphSnapshotBasisDigest() typedmemory.SHA256Digest {
	return stage.graphSnapshotDigest
}

func (stage ProjectTypeEnvStage) GraphRevision() typedmemory.GraphRevision {
	return stage.graphRevision
}

func (stage ProjectTypeEnvStage) ProfileLedgerRevision() projectprofile.LedgerRevision {
	return stage.profileLedgerRevision
}

func (stage ProjectTypeEnvStage) ProfileLedgerDigest() typedmemory.SHA256Digest {
	return stage.profileLedgerDigest
}

func (stage ProjectTypeEnvStage) Compatibility() ProjectTypeEnvStageCompatibility {
	compatibility, _ := normalizeStageCompatibility(
		stage.predecessor,
		stage.verifiedComposite,
		stage.compatibility,
	)
	return compatibility
}

func (stage ProjectTypeEnvStage) CompatibilityRef() ProjectTypeEnvCompatibilityDiffRef {
	return stage.compatibilityRef
}

func (stage ProjectTypeEnvStage) CompatibilityDigest() typedmemory.SHA256Digest {
	return stage.compatibilityDigest
}

func (stage ProjectTypeEnvStage) ExistingAssertionRevalidation() projecttypeenvassertionreport.Report {
	report, _ := normalizeStageRevalidation(stage.revalidation)
	return report
}

func (stage ProjectTypeEnvStage) ExistingAssertionRevalidationRef() ExistingAssertionRevalidationRef {
	return stage.revalidationRef
}

func (stage ProjectTypeEnvStage) ExistingAssertionRevalidationDigest() typedmemory.SHA256Digest {
	return stage.revalidationDigest
}

func (stage ProjectTypeEnvStage) ProfileCompatibility() projecttypeenvprofilefit.Assessment {
	profile, _ := normalizeStageProfileCompatibility(stage.profileCompatibility)
	return profile
}

func (stage ProjectTypeEnvStage) ProfileFitRef() ProjectTypeEnvProfileFitRef {
	return stage.profileFitRef
}

func (stage ProjectTypeEnvStage) ProfileFitDigest() typedmemory.SHA256Digest {
	return stage.profileFitDigest
}

// TransitionProjectionProfileCompatibility returns the exact successor-diff
// and installed-profile assessment bound by a Transition Stage v5. Genesis
// and historical Stage editions have no such artifact.
func (stage ProjectTypeEnvStage) TransitionProjectionProfileCompatibility() (
	projecttypeenvtransitioncompatibility.Set,
	bool,
) {
	if stage.schemaEdition != ProjectTypeEnvStageSchemaEditionV5 {
		return projecttypeenvtransitioncompatibility.Set{}, false
	}
	decoded, err := projecttypeenvtransitioncompatibility.Decode(
		stage.transitionProjectionProfiles.CanonicalBytes(),
	)
	if err != nil {
		return projecttypeenvtransitioncompatibility.Set{}, false
	}
	return decoded, true
}

func (stage ProjectTypeEnvStage) TransitionProjectionProfileCompatibilityRef() (
	projecttypeenvtransitioncompatibility.Ref,
	bool,
) {
	if _, exists := stage.TransitionProjectionProfileCompatibility(); !exists {
		return projecttypeenvtransitioncompatibility.Ref{}, false
	}
	return stage.transitionProjectionProfilesRef, true
}

func (stage ProjectTypeEnvStage) TransitionProjectionProfileCompatibilityDigest() (
	typedmemory.SHA256Digest,
	bool,
) {
	if _, exists := stage.TransitionProjectionProfileCompatibility(); !exists {
		return typedmemory.SHA256Digest{}, false
	}
	return stage.transitionProjectionProfilesDigest, true
}

func (stage ProjectTypeEnvStage) SchemaEdition() string { return stage.schemaEdition }

func (stage ProjectTypeEnvStage) CompilerEdition() StageCompilerEdition {
	return stage.compilerEdition
}

func (stage ProjectTypeEnvStage) RevalidatorEdition() StageRevalidatorEdition {
	return stage.revalidatorEdition
}

func (stage ProjectTypeEnvStage) ProducerEdition() StageProducerEdition {
	return stage.producerEdition
}

// HistoricalProvenance returns the uninterpreted provenance coordinate carried
// by exact v2/v3 Stage bytes. It is absent from current v4 Stages and is not a
// source-lineage record, Work, evidence, or authority.
func (stage ProjectTypeEnvStage) HistoricalProvenance() (ProjectTypeEnvStageProvenanceRef, bool) {
	if stage.schemaEdition != ProjectTypeEnvStageSchemaEditionV2 &&
		stage.schemaEdition != ProjectTypeEnvStageSchemaEditionV3 {
		return ProjectTypeEnvStageProvenanceRef{}, false
	}
	return stage.historicalProvenance, true
}

func (stage ProjectTypeEnvStage) CanonicalBytes() []byte {
	return append([]byte(nil), stage.canonicalBytes...)
}

func (stage ProjectTypeEnvStage) Verify() error {
	verified, err := VerifyProjectTypeEnvStage(stage.ref, stage.canonicalBytes)
	if err != nil {
		return err
	}
	stored := projectTypeEnvStageState{
		project:                            stage.project,
		predecessor:                        stage.predecessor,
		base:                               stage.base,
		extensions:                         stage.extensions,
		runtimeBasis:                       stage.runtimeBasis,
		verifiedComposite:                  stage.verifiedComposite,
		compositeVerificationRef:           stage.compositeVerificationRef,
		compositeVerificationDigest:        stage.compositeVerificationDigest,
		graphSnapshot:                      stage.graphSnapshot,
		graphSnapshotDigest:                stage.graphSnapshotDigest,
		graphRevision:                      stage.graphRevision,
		profileLedgerRevision:              stage.profileLedgerRevision,
		profileLedgerDigest:                stage.profileLedgerDigest,
		compatibility:                      stage.compatibility,
		compatibilityRef:                   stage.compatibilityRef,
		compatibilityDigest:                stage.compatibilityDigest,
		revalidation:                       stage.revalidation,
		revalidationRef:                    stage.revalidationRef,
		revalidationDigest:                 stage.revalidationDigest,
		profileCompatibility:               stage.profileCompatibility,
		profileFitRef:                      stage.profileFitRef,
		profileFitDigest:                   stage.profileFitDigest,
		transitionProjectionProfiles:       stage.transitionProjectionProfiles,
		transitionProjectionProfilesRef:    stage.transitionProjectionProfilesRef,
		transitionProjectionProfilesDigest: stage.transitionProjectionProfilesDigest,
		schemaEdition:                      stage.schemaEdition,
		compilerEdition:                    stage.compilerEdition,
		revalidatorEdition:                 stage.revalidatorEdition,
		producerEdition:                    stage.producerEdition,
		historicalProvenance:               stage.historicalProvenance,
	}
	encoded, err := encodeProjectTypeEnvStageState(stored)
	if err != nil {
		return fmt.Errorf("verify stored Stage state: %w", err)
	}
	if verified.ref != stage.ref || !bytes.Equal(encoded, stage.canonicalBytes) {
		return fmt.Errorf("project TypeEnv Stage stored state differs from canonical bytes")
	}
	return nil
}

func normalizeProjectTypeEnvStageState(
	state projectTypeEnvStageState,
) (projectTypeEnvStageState, error) {
	project, err := projectidentity.ParseProjectID(state.project.String())
	if err != nil || project != state.project {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage project is required")
	}
	predecessor, err := normalizeStagePredecessor(state.predecessor)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if transition, ok := predecessor.(TransitionStagePredecessor); ok &&
		transition.project != project {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage prior-head project mismatch")
	}
	base, err := normalizeTypeEnvRef("Stage base B", state.base)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	extensions, err := normalizeOrderedExtensionRefs(state.extensions)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	runtimeBasis, err := normalizeRuntimeBasisRef(state.runtimeBasis)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	composite, err := normalizeTypeEnvRef("Stage verified composite C", state.verifiedComposite)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	verificationRef, err := projecttypeenv.ParseProjectTypeEnvCompositeVerificationRef(
		state.compositeVerificationRef.String(),
	)
	if err != nil || verificationRef != state.compositeVerificationRef {
		return projectTypeEnvStageState{}, fmt.Errorf(
			"Stage composite verification reference is required",
		)
	}
	verificationDigest, err := normalizeDigest(
		"Stage composite verification digest",
		state.compositeVerificationDigest,
	)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if verificationRef.Digest() != verificationDigest {
		return projectTypeEnvStageState{}, fmt.Errorf(
			"Stage composite verification ref and digest mismatch",
		)
	}
	graphSnapshot, err := ParseProjectGraphSnapshotBasisRef(state.graphSnapshot.String())
	if err != nil || graphSnapshot != state.graphSnapshot {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage graph snapshot basis is required")
	}
	graphSnapshotDigest, err := normalizeDigest("Stage graph snapshot digest", state.graphSnapshotDigest)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if graphSnapshot.Digest() != graphSnapshotDigest {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage graph snapshot ref and digest mismatch")
	}
	profileLedgerDigest, err := normalizeDigest("Stage profile-ledger digest", state.profileLedgerDigest)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compatibility, err := normalizeStageCompatibility(
		predecessor,
		composite,
		state.compatibility,
	)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compatibilityRef, compatibilityDigest, err := deriveStageCompatibilityIdentity(compatibility)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if stageCompatibilityIdentityWasSupplied(state) &&
		(state.compatibilityRef != compatibilityRef || state.compatibilityDigest != compatibilityDigest) {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage compatibility ref or digest mismatch")
	}
	revalidation, err := normalizeStageRevalidation(state.revalidation)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if revalidation.TargetTypeEnv() != composite {
		return projectTypeEnvStageState{}, fmt.Errorf(
			"Stage revalidation target does not match Stage C",
		)
	}
	if revalidation.GraphSnapshotRef().String() != graphSnapshot.String() ||
		revalidation.GraphSnapshot().BasisDigest() != graphSnapshotDigest {
		return projectTypeEnvStageState{}, fmt.Errorf(
			"Stage revalidation graph snapshot does not match Stage graph basis",
		)
	}
	if revalidation.GraphRevision() != state.graphRevision {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage revalidation graph revision mismatch")
	}
	if revalidation.RuntimeBasisRef() != runtimeBasis {
		return projectTypeEnvStageState{}, fmt.Errorf(
			"Stage revalidation runtime basis does not match Stage X",
		)
	}
	revalidationRef, revalidationDigest, err := deriveStageRevalidationIdentity(revalidation)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if stageRevalidationIdentityWasSupplied(state) &&
		(state.revalidationRef != revalidationRef || state.revalidationDigest != revalidationDigest) {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage revalidation ref or digest mismatch")
	}
	profile, err := normalizeStageProfileCompatibility(state.profileCompatibility)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if profile.TargetTypeEnvRef() != composite {
		return projectTypeEnvStageState{}, fmt.Errorf(
			"Stage profile-fit target does not match Stage C",
		)
	}
	profileFitRef, profileFitDigest, err := deriveStageProfileFitIdentity(profile)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	if stageProfileFitIdentityWasSupplied(state) &&
		(state.profileFitRef != profileFitRef || state.profileFitDigest != profileFitDigest) {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage profile-fit ref or digest mismatch")
	}
	if state.schemaEdition != ProjectTypeEnvStageSchemaEditionV2 &&
		state.schemaEdition != ProjectTypeEnvStageSchemaEditionV3 &&
		state.schemaEdition != ProjectTypeEnvStageSchemaEditionV4 &&
		state.schemaEdition != ProjectTypeEnvStageSchemaEditionV5 {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage schema edition is unsupported")
	}
	if err := validateStagePredecessorSchema(predecessor, state.schemaEdition); err != nil {
		return projectTypeEnvStageState{}, err
	}
	transitionProjectionProfiles,
		transitionProjectionProfilesRef,
		transitionProjectionProfilesDigest,
		err := normalizeTransitionProjectionProfiles(
		predecessor,
		composite,
		state.schemaEdition,
		state.transitionProjectionProfiles,
		state.transitionProjectionProfilesRef,
		state.transitionProjectionProfilesDigest,
	)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compiler, err := NewStageCompilerEdition(state.compilerEdition.String())
	if err != nil || compiler != state.compilerEdition {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage compiler edition is required")
	}
	revalidator, err := NewStageRevalidatorEdition(state.revalidatorEdition.String())
	if err != nil || revalidator != state.revalidatorEdition {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage revalidator edition is required")
	}
	producer, err := NewStageProducerEdition(state.producerEdition.String())
	if err != nil || producer != state.producerEdition {
		return projectTypeEnvStageState{}, fmt.Errorf("Stage producer edition is required")
	}
	historicalProvenance, err := normalizeHistoricalStageProvenance(
		state.schemaEdition,
		state.historicalProvenance,
	)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	return projectTypeEnvStageState{
		project:                            project,
		predecessor:                        predecessor,
		base:                               base,
		extensions:                         extensions,
		runtimeBasis:                       runtimeBasis,
		verifiedComposite:                  composite,
		compositeVerificationRef:           verificationRef,
		compositeVerificationDigest:        verificationDigest,
		graphSnapshot:                      graphSnapshot,
		graphSnapshotDigest:                graphSnapshotDigest,
		graphRevision:                      state.graphRevision,
		profileLedgerRevision:              state.profileLedgerRevision,
		profileLedgerDigest:                profileLedgerDigest,
		compatibility:                      compatibility,
		compatibilityRef:                   compatibilityRef,
		compatibilityDigest:                compatibilityDigest,
		revalidation:                       revalidation,
		revalidationRef:                    revalidationRef,
		revalidationDigest:                 revalidationDigest,
		profileCompatibility:               profile,
		profileFitRef:                      profileFitRef,
		profileFitDigest:                   profileFitDigest,
		transitionProjectionProfiles:       transitionProjectionProfiles,
		transitionProjectionProfilesRef:    transitionProjectionProfilesRef,
		transitionProjectionProfilesDigest: transitionProjectionProfilesDigest,
		schemaEdition:                      state.schemaEdition,
		compilerEdition:                    compiler,
		revalidatorEdition:                 revalidator,
		producerEdition:                    producer,
		historicalProvenance:               historicalProvenance,
	}, nil
}

func normalizeHistoricalStageProvenance(
	schemaEdition string,
	provenance ProjectTypeEnvStageProvenanceRef,
) (ProjectTypeEnvStageProvenanceRef, error) {
	switch schemaEdition {
	case ProjectTypeEnvStageSchemaEditionV2, ProjectTypeEnvStageSchemaEditionV3:
		parsed, err := ParseProjectTypeEnvStageProvenanceRef(provenance.String())
		if err != nil || parsed != provenance {
			return ProjectTypeEnvStageProvenanceRef{}, fmt.Errorf(
				"historical Stage provenance is required",
			)
		}
		return parsed, nil
	case ProjectTypeEnvStageSchemaEditionV4, ProjectTypeEnvStageSchemaEditionV5:
		if provenance.Digest().String() != "" {
			return ProjectTypeEnvStageProvenanceRef{}, fmt.Errorf(
				"current Stage cannot carry historical caller-supplied provenance",
			)
		}
		return ProjectTypeEnvStageProvenanceRef{}, nil
	default:
		return ProjectTypeEnvStageProvenanceRef{}, fmt.Errorf(
			"Stage schema edition is unsupported",
		)
	}
}

func normalizeStagePredecessor(
	predecessor ProjectTypeEnvStagePredecessor,
) (ProjectTypeEnvStagePredecessor, error) {
	switch value := predecessor.(type) {
	case GenesisStagePredecessor:
		return NewGenesisStagePredecessor(), nil
	case legacyGenesisStagePredecessor:
		return newLegacyGenesisStagePredecessor(value.noPriorHeadProof)
	case TransitionStagePredecessor:
		return NewTransitionStagePredecessor(TransitionStagePredecessorInput{
			Project:           value.project,
			Head:              value.head,
			HeadRevision:      value.headRevision,
			SelectedComposite: value.selectedComposite,
		})
	default:
		return nil, fmt.Errorf("Stage predecessor posture is required")
	}
}

func validateStagePredecessorSchema(
	predecessor ProjectTypeEnvStagePredecessor,
	schemaEdition string,
) error {
	switch schemaEdition {
	case ProjectTypeEnvStageSchemaEditionV2:
		switch predecessor.(type) {
		case legacyGenesisStagePredecessor, TransitionStagePredecessor:
			return nil
		default:
			return fmt.Errorf("Stage v2 predecessor posture is invalid")
		}
	case ProjectTypeEnvStageSchemaEditionV3:
		switch predecessor.(type) {
		case GenesisStagePredecessor, TransitionStagePredecessor:
			return nil
		default:
			return fmt.Errorf("Stage v3 predecessor posture is invalid")
		}
	case ProjectTypeEnvStageSchemaEditionV4:
		switch predecessor.(type) {
		case GenesisStagePredecessor, TransitionStagePredecessor:
			return nil
		default:
			return fmt.Errorf("Stage v4 predecessor posture is invalid")
		}
	case ProjectTypeEnvStageSchemaEditionV5:
		if _, ok := predecessor.(TransitionStagePredecessor); !ok {
			return fmt.Errorf("Stage v5 requires Transition predecessor posture")
		}
		return nil
	default:
		return fmt.Errorf("Stage schema edition is unsupported")
	}
}

func normalizeStageCompatibility(
	predecessor ProjectTypeEnvStagePredecessor,
	target typedmemory.TypeEnvRef,
	compatibility ProjectTypeEnvStageCompatibility,
) (ProjectTypeEnvStageCompatibility, error) {
	switch predecessorValue := predecessor.(type) {
	case GenesisStagePredecessor, legacyGenesisStagePredecessor:
		value, ok := compatibility.(InitialStageCompatibility)
		if !ok {
			return nil, fmt.Errorf("genesis Stage requires initial compatibility posture")
		}
		normalized, err := NewInitialStageCompatibility(value.target)
		if err != nil {
			return nil, err
		}
		if normalized.target != target {
			return nil, fmt.Errorf("initial compatibility target does not match Stage C")
		}
		return normalized, nil
	case TransitionStagePredecessor:
		value, ok := compatibility.(ComparedStageCompatibility)
		if !ok {
			return nil, fmt.Errorf("transition Stage requires compared compatibility posture")
		}
		normalized, err := NewComparedStageCompatibility(value.diff)
		if err != nil {
			return nil, err
		}
		if normalized.Base() != predecessorValue.selectedComposite {
			return nil, fmt.Errorf("compatibility base does not match prior selected C")
		}
		if normalized.Target() != target {
			return nil, fmt.Errorf("compatibility target does not match Stage C")
		}
		return normalized, nil
	default:
		return nil, fmt.Errorf("Stage predecessor posture is required")
	}
}

func normalizeStageRevalidation(
	report projecttypeenvassertionreport.Report,
) (projecttypeenvassertionreport.Report, error) {
	if err := report.Verify(); err != nil {
		return projecttypeenvassertionreport.Report{}, fmt.Errorf(
			"Stage assertion revalidation report: %w",
			err,
		)
	}
	return projecttypeenvassertionreport.DecodeCanonicalReport(
		report.CanonicalBytes(),
	)
}

func normalizeStageProfileCompatibility(
	profile projecttypeenvprofilefit.Assessment,
) (projecttypeenvprofilefit.Assessment, error) {
	if profile == nil {
		return nil, fmt.Errorf("Stage profile-fit assessment is required")
	}
	if err := profile.Verify(); err != nil {
		return nil, fmt.Errorf(
			"Stage profile-fit assessment: %w",
			err,
		)
	}
	return projecttypeenvprofilefit.DecodeCanonicalAssessment(
		profile.CanonicalBytes(),
	)
}

func normalizeTransitionProjectionProfiles(
	predecessor ProjectTypeEnvStagePredecessor,
	target typedmemory.TypeEnvRef,
	schemaEdition string,
	artifact projecttypeenvtransitioncompatibility.Set,
	artifactRef projecttypeenvtransitioncompatibility.Ref,
	artifactDigest typedmemory.SHA256Digest,
) (
	projecttypeenvtransitioncompatibility.Set,
	projecttypeenvtransitioncompatibility.Ref,
	typedmemory.SHA256Digest,
	error,
) {
	present := artifact.Ref().Digest().String() != "" ||
		artifactRef.Digest().String() != "" ||
		artifactDigest.String() != ""
	if schemaEdition != ProjectTypeEnvStageSchemaEditionV5 {
		if present {
			return projecttypeenvtransitioncompatibility.Set{},
				projecttypeenvtransitioncompatibility.Ref{},
				typedmemory.SHA256Digest{},
				fmt.Errorf("historical or Genesis Stage cannot carry Transition projection-profile compatibility")
		}
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			nil
	}
	transition, ok := predecessor.(TransitionStagePredecessor)
	if !ok {
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			fmt.Errorf("Stage v5 requires Transition projection-profile compatibility")
	}
	if !present {
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			fmt.Errorf("transition Stage v5 requires projection-profile compatibility")
	}
	if err := artifact.Verify(); err != nil {
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			fmt.Errorf("transition Stage projection-profile compatibility: %w", err)
	}
	owned, err := projecttypeenvtransitioncompatibility.Decode(
		artifact.CanonicalBytes(),
	)
	if err != nil {
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			err
	}
	diff := owned.SuccessorDiff()
	if diff.Base() != transition.SelectedComposite() {
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			fmt.Errorf("transition projection-profile compatibility base does not match prior selected C")
	}
	if diff.Target() != target {
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			fmt.Errorf("transition projection-profile compatibility target does not match Stage C")
	}
	derivedRef := owned.Ref()
	derivedDigest := owned.Digest()
	identitySupplied := artifactRef.Digest().String() != "" || artifactDigest.String() != ""
	if identitySupplied && (artifactRef != derivedRef || artifactDigest != derivedDigest) {
		return projecttypeenvtransitioncompatibility.Set{},
			projecttypeenvtransitioncompatibility.Ref{},
			typedmemory.SHA256Digest{},
			fmt.Errorf("transition projection-profile compatibility ref or digest mismatch")
	}
	return owned, derivedRef, derivedDigest, nil
}

func deriveStageCompatibilityIdentity(
	compatibility ProjectTypeEnvStageCompatibility,
) (ProjectTypeEnvCompatibilityDiffRef, typedmemory.SHA256Digest, error) {
	switch value := compatibility.(type) {
	case InitialStageCompatibility:
		writer := stageWriter{}
		writer.addString("haft.project-typeenv.initial-compatibility.v2")
		writer.addString(value.target.String())
		digest, err := deriveStageProjectionDigest(writer.bytes())
		if err != nil {
			return ProjectTypeEnvCompatibilityDiffRef{}, typedmemory.SHA256Digest{}, err
		}
		return ProjectTypeEnvCompatibilityDiffRef{digest: digest}, digest, nil
	case ComparedStageCompatibility:
		if err := value.diff.Verify(); err != nil {
			return ProjectTypeEnvCompatibilityDiffRef{}, typedmemory.SHA256Digest{}, err
		}
		digest := value.diff.Digest()
		return ProjectTypeEnvCompatibilityDiffRef{digest: digest}, digest, nil
	default:
		return ProjectTypeEnvCompatibilityDiffRef{}, typedmemory.SHA256Digest{},
			fmt.Errorf("Stage compatibility posture is required")
	}
}

func deriveStageRevalidationIdentity(
	revalidation projecttypeenvassertionreport.Report,
) (ExistingAssertionRevalidationRef, typedmemory.SHA256Digest, error) {
	if err := revalidation.Verify(); err != nil {
		return ExistingAssertionRevalidationRef{}, typedmemory.SHA256Digest{}, err
	}
	digest := revalidation.Digest()
	return ExistingAssertionRevalidationRef{digest: digest}, digest, nil
}

func deriveStageProfileFitIdentity(
	profile projecttypeenvprofilefit.Assessment,
) (ProjectTypeEnvProfileFitRef, typedmemory.SHA256Digest, error) {
	if profile == nil {
		return ProjectTypeEnvProfileFitRef{}, typedmemory.SHA256Digest{},
			fmt.Errorf("Stage profile-fit assessment is required")
	}
	if err := profile.Verify(); err != nil {
		return ProjectTypeEnvProfileFitRef{}, typedmemory.SHA256Digest{}, err
	}
	return profile.FitRef(), profile.Digest(), nil
}

func deriveStageProjectionDigest(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	digestText := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(digestText)
}

func stageCompatibilityIdentityWasSupplied(state projectTypeEnvStageState) bool {
	return state.compatibilityRef.digest.String() != "" || state.compatibilityDigest.String() != ""
}

func stageRevalidationIdentityWasSupplied(state projectTypeEnvStageState) bool {
	return state.revalidationRef.digest.String() != "" || state.revalidationDigest.String() != ""
}

func stageProfileFitIdentityWasSupplied(state projectTypeEnvStageState) bool {
	return state.profileFitRef.Digest().String() != "" ||
		state.profileFitDigest.String() != ""
}

func encodeProjectTypeEnvStageState(
	state projectTypeEnvStageState,
) ([]byte, error) {
	normalized, err := normalizeProjectTypeEnvStageState(state)
	if err != nil {
		return nil, err
	}
	domain, err := stageDomainForSchema(normalized.schemaEdition)
	if err != nil {
		return nil, err
	}
	writer := stageWriter{}
	writer.addString(domain)
	writer.addString(normalized.project.String())
	if err := encodeStagePredecessor(
		&writer,
		normalized.predecessor,
		normalized.schemaEdition,
	); err != nil {
		return nil, err
	}
	writer.addString(normalized.base.String())
	writer.addUint64(uint64(len(normalized.extensions)))
	for _, extension := range normalized.extensions {
		writer.addString(extension.String())
	}
	writer.addString(normalized.runtimeBasis.String())
	writer.addString(normalized.verifiedComposite.String())
	writer.addString(normalized.compositeVerificationRef.String())
	writer.addString(normalized.compositeVerificationDigest.String())
	writer.addString(normalized.graphSnapshot.String())
	writer.addString(normalized.graphSnapshotDigest.String())
	writer.addUint64(normalized.graphRevision.Value())
	writer.addUint64(normalized.profileLedgerRevision.Value())
	writer.addString(normalized.profileLedgerDigest.String())
	encodeStageCompatibility(&writer, normalized.compatibility)
	writer.addString(normalized.compatibilityRef.String())
	writer.addString(normalized.compatibilityDigest.String())
	writer.addBytes(normalized.revalidation.CanonicalBytes())
	writer.addString(normalized.revalidationRef.String())
	writer.addString(normalized.revalidationDigest.String())
	writer.addBytes(normalized.profileCompatibility.CanonicalBytes())
	writer.addString(normalized.profileFitRef.String())
	writer.addString(normalized.profileFitDigest.String())
	if normalized.schemaEdition == ProjectTypeEnvStageSchemaEditionV5 {
		writer.addBytes(normalized.transitionProjectionProfiles.CanonicalBytes())
		writer.addString(normalized.transitionProjectionProfilesRef.String())
		writer.addString(normalized.transitionProjectionProfilesDigest.String())
	}
	writer.addString(normalized.schemaEdition)
	writer.addString(normalized.compilerEdition.String())
	writer.addString(normalized.revalidatorEdition.String())
	writer.addString(normalized.producerEdition.String())
	if normalized.schemaEdition == ProjectTypeEnvStageSchemaEditionV2 ||
		normalized.schemaEdition == ProjectTypeEnvStageSchemaEditionV3 {
		writer.addString(normalized.historicalProvenance.String())
	}
	result := writer.bytes()
	if len(result) > maximumProjectTypeEnvStageBytes {
		return nil, fmt.Errorf(
			"project TypeEnv Stage exceeds %d bytes",
			maximumProjectTypeEnvStageBytes,
		)
	}
	return result, nil
}

func decodeProjectTypeEnvStageState(
	reader *stageReader,
	domain string,
) (projectTypeEnvStageState, error) {
	projectText, err := reader.readString("project")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return projectTypeEnvStageState{}, fmt.Errorf("decode Stage project: %w", err)
	}
	predecessor, err := decodeStagePredecessor(reader, domain)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	baseText, err := reader.readString("base B")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return projectTypeEnvStageState{}, fmt.Errorf("decode Stage base B: %w", err)
	}
	extensionCount, err := reader.readCount("ordered E DAG", maximumStageExtensions)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, extensionCount)
	for index := 0; index < extensionCount; index++ {
		text, readErr := reader.readString("extension reference")
		if readErr != nil {
			return projectTypeEnvStageState{}, readErr
		}
		ref, parseErr := typedmemory.ParseTypeEnvExtensionRef(text)
		if parseErr != nil {
			return projectTypeEnvStageState{}, fmt.Errorf("decode Stage extension %d: %w", index, parseErr)
		}
		extensions = append(extensions, ref)
	}
	runtimeText, err := reader.readString("runtime basis X")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(runtimeText)
	if err != nil {
		return projectTypeEnvStageState{}, fmt.Errorf("decode Stage runtime basis X: %w", err)
	}
	compositeText, err := reader.readString("verified composite C")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	composite, err := typedmemory.ParseTypeEnvRef(compositeText)
	if err != nil {
		return projectTypeEnvStageState{}, fmt.Errorf("decode Stage composite C: %w", err)
	}
	verificationRefText, err := reader.readString("composite verification reference")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	verificationRef, err := projecttypeenv.ParseProjectTypeEnvCompositeVerificationRef(
		verificationRefText,
	)
	if err != nil {
		return projectTypeEnvStageState{}, fmt.Errorf(
			"decode composite verification reference: %w",
			err,
		)
	}
	verificationDigestText, err := reader.readString("composite verification digest")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	verificationDigest, err := typedmemory.NewSHA256Digest(verificationDigestText)
	if err != nil {
		return projectTypeEnvStageState{}, fmt.Errorf("decode composite verification digest: %w", err)
	}
	snapshotText, err := reader.readString("graph snapshot basis")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	snapshot, err := ParseProjectGraphSnapshotBasisRef(snapshotText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	snapshotDigestText, err := reader.readString("graph snapshot digest")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	snapshotDigest, err := typedmemory.NewSHA256Digest(snapshotDigestText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revisionValue, err := reader.readUint64("graph revision")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revision := typedmemory.NewGraphRevision(revisionValue)
	profileLedgerRevisionValue, err := reader.readUint64("profile-ledger revision")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	profileLedgerRevision := projectprofile.NewLedgerRevision(profileLedgerRevisionValue)
	profileLedgerDigestText, err := reader.readString("profile-ledger digest")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	profileLedgerDigest, err := typedmemory.NewSHA256Digest(profileLedgerDigestText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compatibility, err := decodeStageCompatibility(reader)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compatibilityRefText, err := reader.readString("compatibility ref")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compatibilityRef, err := ParseProjectTypeEnvCompatibilityDiffRef(compatibilityRefText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compatibilityDigestText, err := reader.readString("compatibility digest")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compatibilityDigest, err := typedmemory.NewSHA256Digest(compatibilityDigestText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revalidation, err := decodeStageRevalidation(reader)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revalidationRefText, err := reader.readString("revalidation ref")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revalidationRef, err := ParseExistingAssertionRevalidationRef(revalidationRefText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revalidationDigestText, err := reader.readString("revalidation digest identity")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revalidationDigest, err := typedmemory.NewSHA256Digest(revalidationDigestText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	profile, err := decodeStageProfileCompatibility(reader)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	profileFitRefText, err := reader.readString("profile-fit ref")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	profileFitRef, err := ParseProjectTypeEnvProfileFitRef(profileFitRefText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	profileFitDigestText, err := reader.readString("profile-fit digest")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	profileFitDigest, err := typedmemory.NewSHA256Digest(profileFitDigestText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	transitionProjectionProfiles := projecttypeenvtransitioncompatibility.Set{}
	transitionProjectionProfilesRef := projecttypeenvtransitioncompatibility.Ref{}
	transitionProjectionProfilesDigest := typedmemory.SHA256Digest{}
	if domain == projectTypeEnvStageDomainV5 {
		artifactBytes, readErr := reader.readBytes(
			"Transition projection-profile compatibility",
			maximumProjectTypeEnvStageBytes,
		)
		if readErr != nil {
			return projectTypeEnvStageState{}, readErr
		}
		transitionProjectionProfiles, err = projecttypeenvtransitioncompatibility.Decode(
			artifactBytes,
		)
		if err != nil {
			return projectTypeEnvStageState{}, err
		}
		artifactRefText, readErr := reader.readString(
			"Transition projection-profile compatibility ref",
		)
		if readErr != nil {
			return projectTypeEnvStageState{}, readErr
		}
		transitionProjectionProfilesRef, err = projecttypeenvtransitioncompatibility.ParseRef(
			artifactRefText,
		)
		if err != nil {
			return projectTypeEnvStageState{}, err
		}
		artifactDigestText, readErr := reader.readString(
			"Transition projection-profile compatibility digest",
		)
		if readErr != nil {
			return projectTypeEnvStageState{}, readErr
		}
		transitionProjectionProfilesDigest, err = typedmemory.NewSHA256Digest(
			artifactDigestText,
		)
		if err != nil {
			return projectTypeEnvStageState{}, err
		}
	}
	schemaEdition, err := reader.readString("schema edition")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compilerText, err := reader.readString("compiler edition")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	compiler, err := NewStageCompilerEdition(compilerText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revalidatorText, err := reader.readString("revalidator edition")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	revalidator, err := NewStageRevalidatorEdition(revalidatorText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	producerText, err := reader.readString("producer edition")
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	producer, err := NewStageProducerEdition(producerText)
	if err != nil {
		return projectTypeEnvStageState{}, err
	}
	historicalProvenance := ProjectTypeEnvStageProvenanceRef{}
	if schemaEdition == ProjectTypeEnvStageSchemaEditionV2 ||
		schemaEdition == ProjectTypeEnvStageSchemaEditionV3 {
		provenanceText, readErr := reader.readString("historical provenance")
		if readErr != nil {
			return projectTypeEnvStageState{}, readErr
		}
		historicalProvenance, err = ParseProjectTypeEnvStageProvenanceRef(provenanceText)
		if err != nil {
			return projectTypeEnvStageState{}, err
		}
	}
	return projectTypeEnvStageState{
		project:                            project,
		predecessor:                        predecessor,
		base:                               base,
		extensions:                         extensions,
		runtimeBasis:                       runtimeBasis,
		verifiedComposite:                  composite,
		compositeVerificationRef:           verificationRef,
		compositeVerificationDigest:        verificationDigest,
		graphSnapshot:                      snapshot,
		graphSnapshotDigest:                snapshotDigest,
		graphRevision:                      revision,
		profileLedgerRevision:              profileLedgerRevision,
		profileLedgerDigest:                profileLedgerDigest,
		compatibility:                      compatibility,
		compatibilityRef:                   compatibilityRef,
		compatibilityDigest:                compatibilityDigest,
		revalidation:                       revalidation,
		revalidationRef:                    revalidationRef,
		revalidationDigest:                 revalidationDigest,
		profileCompatibility:               profile,
		profileFitRef:                      profileFitRef,
		profileFitDigest:                   profileFitDigest,
		transitionProjectionProfiles:       transitionProjectionProfiles,
		transitionProjectionProfilesRef:    transitionProjectionProfilesRef,
		transitionProjectionProfilesDigest: transitionProjectionProfilesDigest,
		schemaEdition:                      schemaEdition,
		compilerEdition:                    compiler,
		revalidatorEdition:                 revalidator,
		producerEdition:                    producer,
		historicalProvenance:               historicalProvenance,
	}, nil
}

func stageDomainForSchema(schemaEdition string) (string, error) {
	switch schemaEdition {
	case ProjectTypeEnvStageSchemaEditionV2:
		return projectTypeEnvStageDomainV2, nil
	case ProjectTypeEnvStageSchemaEditionV3:
		return projectTypeEnvStageDomainV3, nil
	case ProjectTypeEnvStageSchemaEditionV4:
		return projectTypeEnvStageDomain, nil
	case ProjectTypeEnvStageSchemaEditionV5:
		return projectTypeEnvStageDomainV5, nil
	default:
		return "", fmt.Errorf("Stage schema edition is unsupported")
	}
}

func encodeStagePredecessor(
	writer *stageWriter,
	predecessor ProjectTypeEnvStagePredecessor,
	wireEdition string,
) error {
	legacy, err := predecessorWireIsLegacy(wireEdition)
	if err != nil {
		return err
	}
	switch value := predecessor.(type) {
	case GenesisStagePredecessor:
		if legacy {
			return fmt.Errorf("legacy predecessor encoding requires legacy Genesis posture")
		}
		writer.addString("genesis")
		return nil
	case legacyGenesisStagePredecessor:
		if !legacy {
			return fmt.Errorf("current predecessor encoding rejects legacy Genesis posture")
		}
		writer.addString("genesis")
		writer.addString(value.noPriorHeadProof.String())
		return nil
	case TransitionStagePredecessor:
		writer.addString("transition")
		writer.addString(value.project.String())
		writer.addString(value.head.String())
		writer.addUint64(value.headRevision.Value())
		writer.addString(value.selectedComposite.String())
		return nil
	default:
		return fmt.Errorf("Stage predecessor posture is required")
	}
}

func decodeStagePredecessor(
	reader *stageReader,
	wireEdition string,
) (ProjectTypeEnvStagePredecessor, error) {
	legacy, err := predecessorWireIsLegacy(wireEdition)
	if err != nil {
		return nil, err
	}
	tag, err := reader.readString("predecessor posture")
	if err != nil {
		return nil, err
	}
	switch tag {
	case "genesis":
		if !legacy {
			return NewGenesisStagePredecessor(), nil
		}
		proofText, readErr := reader.readString("no-prior-head proof")
		if readErr != nil {
			return nil, readErr
		}
		proof, parseErr := ParseNoPriorHeadProofRef(proofText)
		if parseErr != nil {
			return nil, parseErr
		}
		return newLegacyGenesisStagePredecessor(proof)
	case "transition":
		projectText, readErr := reader.readString("prior-head project")
		if readErr != nil {
			return nil, readErr
		}
		project, parseErr := projectidentity.ParseProjectID(projectText)
		if parseErr != nil {
			return nil, parseErr
		}
		headText, readErr := reader.readString("prior head")
		if readErr != nil {
			return nil, readErr
		}
		head, parseErr := ParseProjectTypeEnvHeadRef(headText)
		if parseErr != nil {
			return nil, parseErr
		}
		revisionValue, readErr := reader.readUint64("head revision")
		if readErr != nil {
			return nil, readErr
		}
		revision, parseErr := NewHeadRevision(revisionValue)
		if parseErr != nil {
			return nil, parseErr
		}
		compositeText, readErr := reader.readString("prior selected composite")
		if readErr != nil {
			return nil, readErr
		}
		composite, parseErr := typedmemory.ParseTypeEnvRef(compositeText)
		if parseErr != nil {
			return nil, parseErr
		}
		return NewTransitionStagePredecessor(TransitionStagePredecessorInput{
			Project:           project,
			Head:              head,
			HeadRevision:      revision,
			SelectedComposite: composite,
		})
	default:
		return nil, fmt.Errorf("project TypeEnv Stage predecessor %q is unsupported", tag)
	}
}

func predecessorWireIsLegacy(wireEdition string) (bool, error) {
	switch wireEdition {
	case ProjectTypeEnvStageSchemaEditionV2,
		projectTypeEnvStageDomainV2,
		projectTypeEnvHeadSelectionRequestDomainV1:
		return true, nil
	case ProjectTypeEnvStageSchemaEditionV3,
		projectTypeEnvStageDomainV3,
		ProjectTypeEnvStageSchemaEditionV4,
		projectTypeEnvStageDomain,
		ProjectTypeEnvStageSchemaEditionV5,
		projectTypeEnvStageDomainV5,
		projectTypeEnvHeadSelectionRequestDomain:
		return false, nil
	default:
		return false, fmt.Errorf("predecessor wire edition is unsupported")
	}
}

func encodeStageCompatibility(
	writer *stageWriter,
	compatibility ProjectTypeEnvStageCompatibility,
) {
	switch value := compatibility.(type) {
	case InitialStageCompatibility:
		writer.addString("initial")
		writer.addString(value.target.String())
	case ComparedStageCompatibility:
		writer.addString("compared")
		writer.addBytes(value.diff.CanonicalBytes())
	}
}

func decodeStageCompatibility(
	reader *stageReader,
) (ProjectTypeEnvStageCompatibility, error) {
	tag, err := reader.readString("compatibility posture")
	if err != nil {
		return nil, err
	}
	switch tag {
	case "initial":
		targetText, readErr := reader.readString("initial compatibility target")
		if readErr != nil {
			return nil, readErr
		}
		target, parseErr := typedmemory.ParseTypeEnvRef(targetText)
		if parseErr != nil {
			return nil, parseErr
		}
		return NewInitialStageCompatibility(target)
	case "compared":
		canonical, readErr := reader.readBytes(
			"exact executable TypeEnv compatibility diff",
			maximumProjectTypeEnvStageBytes,
		)
		if readErr != nil {
			return nil, readErr
		}
		diff, parseErr := projecttypeenvcompatibility.DecodeDiff(canonical)
		if parseErr != nil {
			return nil, parseErr
		}
		return NewComparedStageCompatibility(diff)
	default:
		return nil, fmt.Errorf("project TypeEnv Stage compatibility %q is unsupported", tag)
	}
}

func decodeStageRevalidation(
	reader *stageReader,
) (projecttypeenvassertionreport.Report, error) {
	canonical, err := reader.readBytes(
		"exact assertion revalidation report",
		maximumProjectTypeEnvStageBytes,
	)
	if err != nil {
		return projecttypeenvassertionreport.Report{}, err
	}
	return projecttypeenvassertionreport.DecodeCanonicalReport(canonical)
}

func decodeStageProfileCompatibility(
	reader *stageReader,
) (projecttypeenvprofilefit.Assessment, error) {
	canonical, err := reader.readBytes(
		"exact project TypeEnv profile-fit assessment",
		maximumProjectTypeEnvStageBytes,
	)
	if err != nil {
		return nil, err
	}
	return projecttypeenvprofilefit.DecodeCanonicalAssessment(canonical)
}

func projectTypeEnvStageFromState(
	ref ProjectTypeEnvStageRef,
	state projectTypeEnvStageState,
	canonical []byte,
) ProjectTypeEnvStage {
	return ProjectTypeEnvStage{
		ref:                                ref,
		project:                            state.project,
		predecessor:                        state.predecessor,
		base:                               state.base,
		extensions:                         append([]typedmemory.TypeEnvExtensionRef(nil), state.extensions...),
		runtimeBasis:                       state.runtimeBasis,
		verifiedComposite:                  state.verifiedComposite,
		compositeVerificationRef:           state.compositeVerificationRef,
		compositeVerificationDigest:        state.compositeVerificationDigest,
		graphSnapshot:                      state.graphSnapshot,
		graphSnapshotDigest:                state.graphSnapshotDigest,
		graphRevision:                      state.graphRevision,
		profileLedgerRevision:              state.profileLedgerRevision,
		profileLedgerDigest:                state.profileLedgerDigest,
		compatibility:                      state.compatibility,
		compatibilityRef:                   state.compatibilityRef,
		compatibilityDigest:                state.compatibilityDigest,
		revalidation:                       state.revalidation,
		revalidationRef:                    state.revalidationRef,
		revalidationDigest:                 state.revalidationDigest,
		profileCompatibility:               state.profileCompatibility,
		profileFitRef:                      state.profileFitRef,
		profileFitDigest:                   state.profileFitDigest,
		transitionProjectionProfiles:       state.transitionProjectionProfiles,
		transitionProjectionProfilesRef:    state.transitionProjectionProfilesRef,
		transitionProjectionProfilesDigest: state.transitionProjectionProfilesDigest,
		schemaEdition:                      state.schemaEdition,
		compilerEdition:                    state.compilerEdition,
		revalidatorEdition:                 state.revalidatorEdition,
		producerEdition:                    state.producerEdition,
		historicalProvenance:               state.historicalProvenance,
		canonicalBytes:                     append([]byte(nil), canonical...),
	}
}

func deriveProjectTypeEnvStageRef(
	canonical []byte,
) (ProjectTypeEnvStageRef, error) {
	sum := sha256.Sum256(canonical)
	digestText := "sha256:" + hex.EncodeToString(sum[:])
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return ProjectTypeEnvStageRef{}, err
	}
	return ProjectTypeEnvStageRef{digest: digest}, nil
}

type stageWriter struct{ buffer bytes.Buffer }

func (writer *stageWriter) addString(value string) { writer.addBytes([]byte(value)) }

func (writer *stageWriter) addBytes(value []byte) {
	writer.addUint64(uint64(len(value)))
	writer.buffer.Write(value)
}

func (writer *stageWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.buffer.Write(encoded[:])
}

func (writer stageWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type stageReader struct {
	value  []byte
	offset int
}

func (reader *stageReader) readUint64(label string) (uint64, error) {
	if len(reader.value)-reader.offset < 8 {
		return 0, fmt.Errorf("project TypeEnv Stage %s is truncated", label)
	}
	value := binary.BigEndian.Uint64(reader.value[reader.offset : reader.offset+8])
	reader.offset += 8
	return value, nil
}

func (reader *stageReader) readCount(label string, maximum int) (int, error) {
	value, err := reader.readUint64(label + " count")
	if err != nil {
		return 0, err
	}
	valueInt, exact := sliceIndexFromUint64(value)
	if !exact {
		return 0, fmt.Errorf(
			"project TypeEnv Stage %s count does not fit this runtime",
			label,
		)
	}
	if valueInt > maximum {
		return 0, fmt.Errorf("project TypeEnv Stage %s exceeds %d", label, maximum)
	}
	return valueInt, nil
}

func (reader *stageReader) readString(label string) (string, error) {
	value, err := reader.readBytes(label, maximumStageCoordinateBytes)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("project TypeEnv Stage %s contains invalid UTF-8", label)
	}
	return string(value), nil
}

func (reader *stageReader) readBytes(label string, maximum int) ([]byte, error) {
	length, err := reader.readUint64(label + " length")
	if err != nil {
		return nil, err
	}
	lengthValue, exact := sliceIndexFromUint64(length)
	if !exact {
		return nil, fmt.Errorf(
			"project TypeEnv Stage %s length does not fit this runtime",
			label,
		)
	}
	if lengthValue > maximum {
		return nil, fmt.Errorf(
			"project TypeEnv Stage %s exceeds %d bytes",
			label,
			maximum,
		)
	}
	remaining := len(reader.value) - reader.offset
	if lengthValue > remaining {
		return nil, fmt.Errorf("project TypeEnv Stage %s is truncated", label)
	}
	end := reader.offset + lengthValue
	value := append([]byte(nil), reader.value[reader.offset:end]...)
	reader.offset = end
	return value, nil
}

func (reader stageReader) remaining() int { return len(reader.value) - reader.offset }
