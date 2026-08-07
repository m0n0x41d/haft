package projecttypeenvassertionrevalidation

import (
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// These aliases preserve the evaluator package's read API while the canonical
// algebra lives below it in projecttypeenvassertionreport.
type Report = projecttypeenvassertionreport.Report
type AssertionOutcome = projecttypeenvassertionreport.AssertionOutcome
type AssertionOutcomeKind = projecttypeenvassertionreport.AssertionOutcomeKind
type Ground = projecttypeenvassertionreport.Ground
type GroundCode = projecttypeenvassertionreport.GroundCode
type GroundPosture = projecttypeenvassertionreport.GroundPosture
type GroundDetail = projecttypeenvassertionreport.GroundDetail

const (
	AssertionValid           = projecttypeenvassertionreport.AssertionValid
	AssertionInvalid         = projecttypeenvassertionreport.AssertionInvalid
	AssertionUnderdetermined = projecttypeenvassertionreport.AssertionUnderdetermined

	GroundInvalid      = projecttypeenvassertionreport.GroundInvalid
	GroundMissingBasis = projecttypeenvassertionreport.GroundMissingBasis

	CodeTargetRelationFragmentUnavailable  = projecttypeenvassertionreport.CodeTargetRelationFragmentUnavailable
	CodeRelationFragmentContextMismatch    = projecttypeenvassertionreport.CodeRelationFragmentContextMismatch
	CodeTargetSignatureUnavailable         = projecttypeenvassertionreport.CodeTargetSignatureUnavailable
	CodeTargetContextUnavailable           = projecttypeenvassertionreport.CodeTargetContextUnavailable
	CodeSignatureContextMismatch           = projecttypeenvassertionreport.CodeSignatureContextMismatch
	CodeUnknownSlot                        = projecttypeenvassertionreport.CodeUnknownSlot
	CodeMissingSlot                        = projecttypeenvassertionreport.CodeMissingSlot
	CodeCardinalityMismatch                = projecttypeenvassertionreport.CodeCardinalityMismatch
	CodeReferenceModeMismatch              = projecttypeenvassertionreport.CodeReferenceModeMismatch
	CodeReferenceKindMismatch              = projecttypeenvassertionreport.CodeReferenceKindMismatch
	CodeValueKindMismatch                  = projecttypeenvassertionreport.CodeValueKindMismatch
	CodeTargetKindUnavailable              = projecttypeenvassertionreport.CodeTargetKindUnavailable
	CodeValueBindingUnavailable            = projecttypeenvassertionreport.CodeValueBindingUnavailable
	CodeValueMigrationRequired             = projecttypeenvassertionreport.CodeValueMigrationRequired
	CodeValueCanonicalBytesChanged         = projecttypeenvassertionreport.CodeValueCanonicalBytesChanged
	CodeRefKindDefinitionMissing           = projecttypeenvassertionreport.CodeRefKindDefinitionMissing
	CodeRefKindReferentMismatch            = projecttypeenvassertionreport.CodeRefKindReferentMismatch
	CodeStaticKindDisjointness             = projecttypeenvassertionreport.CodeStaticKindDisjointness
	CodeSlotGroupMismatch                  = projecttypeenvassertionreport.CodeSlotGroupMismatch
	CodeKindSignatureUnavailable           = projecttypeenvassertionreport.CodeKindSignatureUnavailable
	CodeEntitySetUnavailable               = projecttypeenvassertionreport.CodeEntitySetUnavailable
	CodeMemberOfEvaluatorMissing           = projecttypeenvassertionreport.CodeMemberOfEvaluatorMissing
	CodeMemberOfObservableMissing          = projecttypeenvassertionreport.CodeMemberOfObservableMissing
	CodeMemberOfNotMember                  = projecttypeenvassertionreport.CodeMemberOfNotMember
	CodeKindClassificationEvaluatorMissing = projecttypeenvassertionreport.CodeKindClassificationEvaluatorMissing
	CodeKindClassificationBasisMissing     = projecttypeenvassertionreport.CodeKindClassificationBasisMissing
	CodeKindClassificationFalse            = projecttypeenvassertionreport.CodeKindClassificationFalse
)

func DecodeCanonicalReport(raw []byte) (Report, error) {
	return projecttypeenvassertionreport.DecodeCanonicalReport(raw)
}

func newAssertionOutcome(
	assertion typedmemory.AssertionID,
	relationDigest typedmemory.SHA256Digest,
	grounds []Ground,
) (AssertionOutcome, error) {
	return projecttypeenvassertionreport.NewAssertionOutcome(
		assertion,
		relationDigest,
		grounds,
	)
}

func newReport(
	targetTypeEnv typedmemory.TypeEnvRef,
	graphSnapshot projecttypeenvselection.ProjectGraphSnapshotBasis,
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisRef,
	runtimeCoordinate typedmemory.SHA256Digest,
	outcomes []AssertionOutcome,
) (Report, error) {
	if err := graphSnapshot.Verify(); err != nil {
		return Report{}, err
	}
	ref, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		graphSnapshot.Ref().String(),
	)
	if err != nil {
		return Report{}, err
	}
	coordinate, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		ref,
		graphSnapshot.GraphRevision(),
		graphSnapshot.Ref().Digest(),
	)
	if err != nil {
		return Report{}, err
	}
	return projecttypeenvassertionreport.NewReport(
		targetTypeEnv,
		coordinate,
		runtimeBasis,
		runtimeCoordinate,
		outcomes,
	)
}
