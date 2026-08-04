package projecttypeenvselectioneffect

import (
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	casWorkRecordDomain = "haft.project-typeenv.head-cas-work-record.v1"

	predecessorComparisonGenesis    = "genesis_absence_matched"
	predecessorComparisonTransition = "transition_head_matched"

	projectGraphUWorkNotAssertedP8GV1 = "not_asserted_p8g_v1"
)

// ProjectTypeEnvHeadCASWorkCoordinates are A.15.1-shaped coordinates carried
// by the local record. They do not by themselves admit anything into the
// project graph as U.Work.
type ProjectTypeEnvHeadCASWorkCoordinates struct {
	method             authority.MethodRef
	methodDescription  authority.MethodDescriptionRef
	coveringAssignment authority.RoleAssignmentRef
	actualPerformer    authority.SystemRef
	boundedContext     authority.BoundedContextRef
	workInterval       authority.TimeWindow
	statePlane         authority.StatePlaneRef
	resourceLedger     authority.ResourceLedgerRef
	outcome            authority.WorkOutcomeRef
	acceptance         authority.AcceptancePostureRef
	auditTrace         authority.AuditTraceRef
}

type ProjectTypeEnvHeadCASWorkCoordinatesInput struct {
	Method            authority.MethodRef
	MethodDescription authority.MethodDescriptionRef
	Authority         ProjectTypeEnvHeadSelectionAuthorityCoordinates
	WorkInterval      authority.TimeWindow
	StatePlane        authority.StatePlaneRef
	ResourceLedger    authority.ResourceLedgerRef
	Outcome           authority.WorkOutcomeRef
	Acceptance        authority.AcceptancePostureRef
	AuditTrace        authority.AuditTraceRef
}

func NewProjectTypeEnvHeadCASWorkCoordinates(
	input ProjectTypeEnvHeadCASWorkCoordinatesInput,
) (ProjectTypeEnvHeadCASWorkCoordinates, error) {
	subject := input.Authority.ExecutionSubject()
	if err := verifyCASWorkExecutionSubject(subject, input.WorkInterval); err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	return newProjectTypeEnvHeadCASWorkCoordinates(
		projectTypeEnvHeadCASWorkCoordinatesRawInput{
			Method:             input.Method,
			MethodDescription:  input.MethodDescription,
			CoveringAssignment: subject.Ref(),
			ActualPerformer:    subject.HolderSystemRef(),
			BoundedContext:     subject.BoundedContext(),
			WorkInterval:       input.WorkInterval,
			StatePlane:         input.StatePlane,
			ResourceLedger:     input.ResourceLedger,
			Outcome:            input.Outcome,
			Acceptance:         input.Acceptance,
			AuditTrace:         input.AuditTrace,
		},
	)
}

type projectTypeEnvHeadCASWorkCoordinatesRawInput struct {
	Method             authority.MethodRef
	MethodDescription  authority.MethodDescriptionRef
	CoveringAssignment authority.RoleAssignmentRef
	ActualPerformer    authority.SystemRef
	BoundedContext     authority.BoundedContextRef
	WorkInterval       authority.TimeWindow
	StatePlane         authority.StatePlaneRef
	ResourceLedger     authority.ResourceLedgerRef
	Outcome            authority.WorkOutcomeRef
	Acceptance         authority.AcceptancePostureRef
	AuditTrace         authority.AuditTraceRef
}

func newProjectTypeEnvHeadCASWorkCoordinates(
	input projectTypeEnvHeadCASWorkCoordinatesRawInput,
) (ProjectTypeEnvHeadCASWorkCoordinates, error) {
	method, err := authority.NewMethodRef(input.Method.String())
	if err != nil || method != input.Method {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, fmt.Errorf("CAS Work Method ref is required")
	}
	description, err := authority.NewMethodDescriptionRef(
		input.MethodDescription.String(),
	)
	if err != nil || description != input.MethodDescription {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work MethodDescription ref is required")
	}
	coveringAssignment, err := authority.NewRoleAssignmentRef(
		input.CoveringAssignment.String(),
	)
	if err != nil || coveringAssignment != input.CoveringAssignment {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work covering RoleAssignment ref is required")
	}
	actualPerformer, err := authority.NewSystemRef(input.ActualPerformer.String())
	if err != nil || actualPerformer != input.ActualPerformer {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work actual performer System ref is required")
	}
	context, err := authority.NewBoundedContextRef(input.BoundedContext.String())
	if err != nil || context != input.BoundedContext {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work bounded context is required")
	}
	window, err := authority.NewTimeWindow(
		input.WorkInterval.From(),
		input.WorkInterval.Until(),
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	statePlane, err := authority.NewStatePlaneRef(input.StatePlane.String())
	if err != nil || statePlane != input.StatePlane {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work StatePlane ref is required")
	}
	ledger, err := authority.NewResourceLedgerRef(input.ResourceLedger.String())
	if err != nil || ledger != input.ResourceLedger {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work resource-ledger ref is required")
	}
	outcome, err := authority.NewWorkOutcomeRef(input.Outcome.String())
	if err != nil || outcome != input.Outcome {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work outcome ref is required")
	}
	acceptance, err := authority.NewAcceptancePostureRef(
		input.Acceptance.String(),
	)
	if err != nil || acceptance != input.Acceptance {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work acceptance posture is required")
	}
	audit, err := authority.NewAuditTraceRef(input.AuditTrace.String())
	if err != nil || audit != input.AuditTrace {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work audit-trace ref is required")
	}
	return ProjectTypeEnvHeadCASWorkCoordinates{
		method:             method,
		methodDescription:  description,
		coveringAssignment: coveringAssignment,
		actualPerformer:    actualPerformer,
		boundedContext:     context,
		workInterval:       window,
		statePlane:         statePlane,
		resourceLedger:     ledger,
		outcome:            outcome,
		acceptance:         acceptance,
		auditTrace:         audit,
	}, nil
}

func verifyCASWorkExecutionSubject(
	subject projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionSubject,
	workInterval authority.TimeWindow,
) error {
	decoded, err :=
		projecttypeenvselectionauthority.DecodeProjectTypeEnvHeadSelectionPermissionSubject(
			subject.CanonicalJSON(),
		)
	if err != nil || !executionSubjectsEqual(decoded, subject) {
		return fmt.Errorf("CAS Work execution subject is invalid")
	}
	window, err := authority.NewTimeWindow(
		workInterval.From(),
		workInterval.Until(),
	)
	if err != nil {
		return fmt.Errorf("CAS Work interval: %w", err)
	}
	assignment := subject.AssignmentWindow()
	inside := !window.From().Before(assignment.From()) &&
		!window.Until().After(assignment.Until())
	if !inside {
		return fmt.Errorf("CAS Work interval is outside execution RoleAssignment")
	}
	return nil
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) Method() authority.MethodRef {
	return value.method
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) MethodDescription() authority.MethodDescriptionRef {
	return value.methodDescription
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) CoveringRoleAssignment() authority.RoleAssignmentRef {
	return value.coveringAssignment
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) ActualPerformerSystem() authority.SystemRef {
	return value.actualPerformer
}

// PerformedBy preserves the pre-v9 accessor for callers reading frozen
// records. Its value is a covering assignment, not an actor.
//
// Deprecated: use CoveringRoleAssignment.
func (value ProjectTypeEnvHeadCASWorkCoordinates) PerformedBy() authority.RoleAssignmentRef {
	return value.CoveringRoleAssignment()
}

// ExecutedWithin preserves the pre-v9 accessor for callers reading frozen
// records. For this record the value is the actual performer system.
//
// Deprecated: use ActualPerformerSystem.
func (value ProjectTypeEnvHeadCASWorkCoordinates) ExecutedWithin() authority.SystemRef {
	return value.ActualPerformerSystem()
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) BoundedContext() authority.BoundedContextRef {
	return value.boundedContext
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) WorkInterval() authority.TimeWindow {
	return value.workInterval
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) StatePlane() authority.StatePlaneRef {
	return value.statePlane
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) ResourceLedger() authority.ResourceLedgerRef {
	return value.resourceLedger
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) Outcome() authority.WorkOutcomeRef {
	return value.outcome
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) Acceptance() authority.AcceptancePostureRef {
	return value.acceptance
}

func (value ProjectTypeEnvHeadCASWorkCoordinates) AuditTrace() authority.AuditTraceRef {
	return value.auditTrace
}

func encodeCASWorkCoordinates(
	writer *canonicalWriter,
	value ProjectTypeEnvHeadCASWorkCoordinates,
) {
	writer.writeString(value.method.String())
	writer.writeString(value.methodDescription.String())
	// Keep the v1 canonical byte order stable: covering assignment first,
	// actual performer system second.
	writer.writeString(value.coveringAssignment.String())
	writer.writeString(value.actualPerformer.String())
	writer.writeString(value.boundedContext.String())
	writer.writeString(value.workInterval.From().UTC().Format(time.RFC3339Nano))
	writer.writeString(value.workInterval.Until().UTC().Format(time.RFC3339Nano))
	writer.writeString(value.statePlane.String())
	writer.writeString(value.resourceLedger.String())
	writer.writeString(value.outcome.String())
	writer.writeString(value.acceptance.String())
	writer.writeString(value.auditTrace.String())
}

func decodeCASWorkCoordinates(
	reader *canonicalReader,
) (ProjectTypeEnvHeadCASWorkCoordinates, error) {
	methodText, err := reader.readString("CAS Work Method ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	method, err := authority.NewMethodRef(methodText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	descriptionText, err := reader.readString("CAS Work MethodDescription ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	description, err := authority.NewMethodDescriptionRef(descriptionText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	coveringAssignmentText, err := reader.readString("CAS Work covering RoleAssignment ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	coveringAssignment, err := authority.NewRoleAssignmentRef(coveringAssignmentText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	actualPerformerText, err := reader.readString("CAS Work actual performer System ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	actualPerformer, err := authority.NewSystemRef(actualPerformerText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	contextText, err := reader.readString("CAS Work bounded context")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	context, err := authority.NewBoundedContextRef(contextText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	fromText, err := reader.readString("CAS Work interval start")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	from, err := time.Parse(time.RFC3339Nano, fromText)
	if err != nil || from.UTC().Format(time.RFC3339Nano) != fromText {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work interval start is not canonical UTC")
	}
	untilText, err := reader.readString("CAS Work interval end")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	until, err := time.Parse(time.RFC3339Nano, untilText)
	if err != nil || until.UTC().Format(time.RFC3339Nano) != untilText {
		return ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("CAS Work interval end is not canonical UTC")
	}
	window, err := authority.NewTimeWindow(from, until)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	statePlaneText, err := reader.readString("CAS Work StatePlane ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	statePlane, err := authority.NewStatePlaneRef(statePlaneText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	ledgerText, err := reader.readString("CAS Work resource-ledger ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	ledger, err := authority.NewResourceLedgerRef(ledgerText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	outcomeText, err := reader.readString("CAS Work outcome ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	outcome, err := authority.NewWorkOutcomeRef(outcomeText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	acceptanceText, err := reader.readString("CAS Work acceptance posture")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	acceptance, err := authority.NewAcceptancePostureRef(acceptanceText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	auditText, err := reader.readString("CAS Work audit-trace ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	audit, err := authority.NewAuditTraceRef(auditText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkCoordinates{}, err
	}
	return newProjectTypeEnvHeadCASWorkCoordinates(
		projectTypeEnvHeadCASWorkCoordinatesRawInput{
			Method:             method,
			MethodDescription:  description,
			CoveringAssignment: coveringAssignment,
			ActualPerformer:    actualPerformer,
			BoundedContext:     context,
			WorkInterval:       window,
			StatePlane:         statePlane,
			ResourceLedger:     ledger,
			Outcome:            outcome,
			Acceptance:         acceptance,
			AuditTrace:         audit,
		},
	)
}

// ProjectTypeEnvHeadPredecessorComparison is a closed, derived comparison
// result. It cannot be provided independently of the closed predecessor.
type ProjectTypeEnvHeadPredecessorComparison interface {
	projectTypeEnvHeadPredecessorComparisonVariant()
}

type GenesisHeadAbsenceMatched struct {
	proof projecttypeenvselection.NoPriorHeadProofRef
}

func (GenesisHeadAbsenceMatched) projectTypeEnvHeadPredecessorComparisonVariant() {}

func (value GenesisHeadAbsenceMatched) Proof() projecttypeenvselection.NoPriorHeadProofRef {
	return value.proof
}

type TransitionHeadMatched struct {
	prior projecttypeenvselection.TransitionStagePredecessor
}

func (TransitionHeadMatched) projectTypeEnvHeadPredecessorComparisonVariant() {}

func (value TransitionHeadMatched) Prior() projecttypeenvselection.TransitionStagePredecessor {
	return value.prior
}

func derivePredecessorComparison(
	predecessor projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
	genesisProof projecttypeenvselection.NoPriorHeadProofRef,
) (ProjectTypeEnvHeadPredecessorComparison, error) {
	switch exact := predecessor.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		proof, err := projecttypeenvselection.ParseNoPriorHeadProofRef(
			genesisProof.String(),
		)
		if err != nil || proof != genesisProof {
			return nil, fmt.Errorf(
				"genesis predecessor comparison requires an effect-owned absence proof",
			)
		}
		return GenesisHeadAbsenceMatched{proof: proof}, nil
	case projecttypeenvselection.TransitionStagePredecessor:
		return TransitionHeadMatched{prior: exact}, nil
	default:
		return nil, fmt.Errorf("head predecessor comparison variant is unsupported")
	}
}

func encodePredecessorComparison(
	writer *canonicalWriter,
	value ProjectTypeEnvHeadPredecessorComparison,
) {
	switch exact := value.(type) {
	case GenesisHeadAbsenceMatched:
		writer.writeString(predecessorComparisonGenesis)
		writer.writeString(exact.Proof().String())
	case TransitionHeadMatched:
		writer.writeString(predecessorComparisonTransition)
		writer.writeString(exact.Prior().Head().String())
		writer.writeUint64(exact.Prior().HeadRevision().Value())
		writer.writeString(exact.Prior().SelectedComposite().String())
	}
}

func decodePredecessorComparison(
	reader *canonicalReader,
	project projectidentity.ProjectID,
	predecessor projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) (ProjectTypeEnvHeadPredecessorComparison, error) {
	variant, err := reader.readString("head predecessor comparison variant")
	if err != nil {
		return nil, err
	}
	switch variant {
	case predecessorComparisonGenesis:
		proofText, readErr := reader.readString("matched absence proof")
		if readErr != nil {
			return nil, readErr
		}
		proof, parseErr := projecttypeenvselection.ParseNoPriorHeadProofRef(
			proofText,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		if _, ok := predecessor.(projecttypeenvselection.GenesisStagePredecessor); !ok {
			return nil, fmt.Errorf("genesis predecessor comparison mismatch")
		}
		return GenesisHeadAbsenceMatched{proof: proof}, nil
	case predecessorComparisonTransition:
		headText, readErr := reader.readString("matched prior head")
		if readErr != nil {
			return nil, readErr
		}
		head, parseErr := projecttypeenvselection.ParseProjectTypeEnvHeadRef(
			headText,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		revisionValue, readErr := reader.readUint64("matched prior HeadRevision")
		if readErr != nil {
			return nil, readErr
		}
		revision, parseErr := projecttypeenvselection.NewHeadRevision(revisionValue)
		if parseErr != nil {
			return nil, parseErr
		}
		compositeText, readErr := reader.readString("matched prior composite")
		if readErr != nil {
			return nil, readErr
		}
		composite, parseErr := typedmemory.ParseTypeEnvRef(compositeText)
		if parseErr != nil {
			return nil, parseErr
		}
		prior, parseErr := projecttypeenvselection.NewTransitionStagePredecessor(
			projecttypeenvselection.TransitionStagePredecessorInput{
				Project:           project,
				Head:              head,
				HeadRevision:      revision,
				SelectedComposite: composite,
			},
		)
		if parseErr != nil {
			return nil, parseErr
		}
		exact, ok := predecessor.(projecttypeenvselection.TransitionStagePredecessor)
		if !ok ||
			exact.Head() != prior.Head() ||
			exact.HeadRevision() != prior.HeadRevision() ||
			exact.SelectedComposite() != prior.SelectedComposite() {
			return nil, fmt.Errorf("transition predecessor comparison mismatch")
		}
	default:
		return nil, fmt.Errorf("head predecessor comparison variant is invalid")
	}
	return derivePredecessorComparison(
		predecessor,
		projecttypeenvselection.NoPriorHeadProofRef{},
	)
}

// ProjectGraphUWorkAdmissionPosture is closed in P8G v1. It makes the
// non-admission claim inspectable without accepting a caller-supplied bool.
type ProjectGraphUWorkAdmissionPosture struct {
	value string
}

func ProjectGraphUWorkMembershipNotAssertedP8GV1() ProjectGraphUWorkAdmissionPosture {
	return ProjectGraphUWorkAdmissionPosture{
		value: projectGraphUWorkNotAssertedP8GV1,
	}
}

func (value ProjectGraphUWorkAdmissionPosture) String() string {
	return value.value
}

// ProjectTypeEnvHeadCASWorkRecord describes the successful system CAS effect
// through its stable WorkRef and exact local coordinates. It is not the Work
// occurrence and does not assert project-graph U.Work membership.
type ProjectTypeEnvHeadCASWorkRecord struct {
	ref                    ProjectTypeEnvHeadCASWorkRecordRef
	digest                 typedmemory.SHA256Digest
	transactionRef         ProjectTypeEnvHeadSelectionTransactionRef
	transactionDigest      typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	workRef                authority.WorkRef
	coordinates            ProjectTypeEnvHeadCASWorkCoordinates
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	authority              ProjectTypeEnvHeadSelectionAuthorityCoordinates
	authorityUseRef        ProjectTypeEnvHeadSelectionAuthorityUseRecordRef
	authorityUseDigest     typedmemory.SHA256Digest
	receiptRef             ProjectTypeEnvHeadSelectionReceiptRef
	receiptDigest          typedmemory.SHA256Digest
	predecessor            projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	predecessorComparison  ProjectTypeEnvHeadPredecessorComparison
	target                 ProjectTypeEnvHeadSelectionTarget
	expectedGraphRevision  typedmemory.GraphRevision
	committedGraphRevision typedmemory.GraphRevision
	successorHead          projecttypeenvselection.ProjectTypeEnvHeadState
	successorHeadDigest    typedmemory.SHA256Digest
	event                  projecttypeenvselection.GraphEventRef
	commit                 projecttypeenvselection.GraphCommitRef
	committedResultRef     ProjectTypeEnvHeadSelectionCommittedResultRef
	committedResultDigest  typedmemory.SHA256Digest
	canonicalBytes         []byte
}

type ProjectTypeEnvHeadCASWorkRecordInput struct {
	Identity     ProjectTypeEnvHeadSelectionTransactionIdentity
	ReferenceDAG ProjectTypeEnvHeadSelectionReferenceDAG
	Receipt      ProjectTypeEnvHeadSelectionReceiptV1
	AuthorityUse ProjectTypeEnvHeadSelectionAuthorityUseRecord
	Result       ProjectTypeEnvHeadSelectionCommittedResult
	Coordinates  ProjectTypeEnvHeadCASWorkCoordinates
	GenesisProof projecttypeenvselection.NoPriorHeadProofRef
}

func SealProjectTypeEnvHeadCASWorkRecord(
	input ProjectTypeEnvHeadCASWorkRecordInput,
) (ProjectTypeEnvHeadCASWorkRecord, error) {
	if err := input.Identity.Verify(); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	if err := input.ReferenceDAG.Verify(input.Identity); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	if err := input.Receipt.Verify(); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	if err := input.AuthorityUse.Verify(); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	if err := input.Result.Verify(); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	if err := verifyCASWorkCoordinatesAgainstAuthority(
		input.Coordinates,
		input.Receipt.AuthorityCoordinates(),
	); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	coordinates, err := NewProjectTypeEnvHeadCASWorkCoordinates(
		ProjectTypeEnvHeadCASWorkCoordinatesInput{
			Method:            input.Coordinates.Method(),
			MethodDescription: input.Coordinates.MethodDescription(),
			Authority:         input.Receipt.AuthorityCoordinates(),
			WorkInterval:      input.Coordinates.WorkInterval(),
			StatePlane:        input.Coordinates.StatePlane(),
			ResourceLedger:    input.Coordinates.ResourceLedger(),
			Outcome:           input.Coordinates.Outcome(),
			Acceptance:        input.Coordinates.Acceptance(),
			AuditTrace:        input.Coordinates.AuditTrace(),
		},
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	if input.Receipt.TransactionRef() != input.Identity.Ref() ||
		input.Receipt.WorkRef() != input.ReferenceDAG.WorkRef() ||
		input.Receipt.CASWorkRecordRef() !=
			input.ReferenceDAG.CASWorkRecordRef() ||
		input.AuthorityUse.Ref() !=
			input.ReferenceDAG.AuthorityUseRecordRef() ||
		input.AuthorityUse.WorkRef() != input.ReferenceDAG.WorkRef() ||
		input.AuthorityUse.ReceiptRef() != input.Receipt.Ref() ||
		input.Result.Ref() != input.Receipt.CommittedResultRef() {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work members do not describe one exact transaction")
	}
	comparison, err := derivePredecessorComparison(
		input.Receipt.Predecessor(),
		input.GenesisProof,
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	writer := newCanonicalWriter(casWorkRecordDomain)
	writer.writeString(input.ReferenceDAG.CASWorkRecordRef().String())
	writer.writeString(input.Identity.Ref().String())
	writer.writeString(input.Identity.Digest().String())
	writer.writeString(input.Identity.Project().String())
	writer.writeString(input.ReferenceDAG.WorkRef().String())
	encodeCASWorkCoordinates(&writer, coordinates)
	writer.writeString(input.Identity.RequestRef().String())
	writer.writeString(input.Identity.RequestDigest().String())
	encodeAuthorityCoordinates(&writer, input.Receipt.AuthorityCoordinates())
	writer.writeString(input.AuthorityUse.Ref().String())
	writer.writeString(input.AuthorityUse.Digest().String())
	writer.writeString(input.Receipt.Ref().String())
	writer.writeString(input.Receipt.Digest().String())
	encodePredecessor(&writer, input.Receipt.Predecessor())
	encodePredecessorComparison(&writer, comparison)
	encodeTarget(&writer, input.Receipt.Target())
	writer.writeUint64(input.Receipt.ExpectedGraphRevision().Value())
	writer.writeUint64(input.Receipt.CommittedGraphRevision().Value())
	writer.writeBytes(input.Receipt.SuccessorHead().CanonicalBytes())
	writer.writeString(input.Receipt.SuccessorHeadDigest().String())
	writer.writeString(input.Receipt.EventRef().String())
	writer.writeString(input.Receipt.CommitRef().String())
	writer.writeString(input.Result.Ref().String())
	writer.writeString(input.Result.Digest().String())
	return DecodeProjectTypeEnvHeadCASWorkRecord(writer.bytes())
}

func DecodeProjectTypeEnvHeadCASWorkRecord(
	canonical []byte,
) (ProjectTypeEnvHeadCASWorkRecord, error) {
	reader, err := newCanonicalReader(canonical, casWorkRecordDomain)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	refText, err := reader.readString("CAS Work-record ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	ref, err := ParseProjectTypeEnvHeadCASWorkRecordRef(refText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	transactionText, err := reader.readString("CAS Work transaction ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	transactionRef, err := ParseProjectTypeEnvHeadSelectionTransactionRef(
		transactionText,
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	transactionDigest, err := readTypedDigest(
		reader,
		"CAS Work transaction digest",
	)
	if err != nil || transactionRef.Digest() != transactionDigest {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work transaction ref/digest mismatch")
	}
	projectText, err := reader.readString("CAS Work project")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	workText, err := reader.readString("CAS Work occurrence ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	workRef, err := authority.NewWorkRef(workText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	coordinates, err := decodeCASWorkCoordinates(reader)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	requestText, err := reader.readString("CAS Work request ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	requestDigest, err := readTypedDigest(reader, "CAS Work request digest")
	if err != nil || requestRef.Digest() != requestDigest {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work request ref/digest mismatch")
	}
	coordinatesAuthority, err := decodeAuthorityCoordinates(reader)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	if err := verifyCASWorkCoordinatesAgainstAuthority(
		coordinates,
		coordinatesAuthority,
	); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	useText, err := reader.readString("CAS Work authority-use ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	useRef, err := ParseProjectTypeEnvHeadSelectionAuthorityUseRecordRef(useText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	useDigest, err := readTypedDigest(reader, "CAS Work authority-use digest")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	receiptText, err := reader.readString("CAS Work receipt ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	receiptRef, err := ParseProjectTypeEnvHeadSelectionReceiptRef(receiptText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	receiptDigest, err := readTypedDigest(reader, "CAS Work receipt digest")
	if err != nil || receiptRef.Digest() != receiptDigest {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work receipt ref/digest mismatch")
	}
	predecessor, err := decodePredecessor(reader, project)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	comparison, err := decodePredecessorComparison(
		reader,
		project,
		predecessor,
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	target, err := decodeTarget(reader)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	expected, err := reader.readUint64("CAS Work expected GraphRevision")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	committed, err := reader.readUint64("CAS Work committed GraphRevision")
	if err != nil || committed == 0 || committed-1 != expected {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work GraphRevision pair is not contiguous")
	}
	headBytes, err := reader.readBytes("CAS Work successor head")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	head, err := projecttypeenvselection.DecodeProjectTypeEnvHeadState(headBytes)
	if err != nil || head.Project() != project ||
		head.SelectedComposite() != target.Composite() {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work successor head is invalid")
	}
	headDigest, err := readTypedDigest(reader, "CAS Work successor head digest")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	recomputedHeadDigest, err := digestRaw(headBytes)
	if err != nil || recomputedHeadDigest != headDigest {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work successor head digest mismatch")
	}
	eventText, err := reader.readString("CAS Work event ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(eventText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	commitText, err := reader.readString("CAS Work commit ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(commitText)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	resultText, err := reader.readString("CAS Work committed-result ref")
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	resultRef, err := ParseProjectTypeEnvHeadSelectionCommittedResultRef(
		resultText,
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	resultDigest, err := readTypedDigest(
		reader,
		"CAS Work committed-result digest",
	)
	if err != nil || resultRef.Digest() != resultDigest {
		return ProjectTypeEnvHeadCASWorkRecord{},
			fmt.Errorf("CAS Work committed-result ref/digest mismatch")
	}
	if err := reader.requireEnd("head CAS Work record"); err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	digest, err := digestCanonical(casWorkRecordDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecord{}, err
	}
	return ProjectTypeEnvHeadCASWorkRecord{
		ref:                    ref,
		digest:                 digest,
		transactionRef:         transactionRef,
		transactionDigest:      transactionDigest,
		project:                project,
		workRef:                workRef,
		coordinates:            coordinates,
		requestRef:             requestRef,
		requestDigest:          requestDigest,
		authority:              coordinatesAuthority,
		authorityUseRef:        useRef,
		authorityUseDigest:     useDigest,
		receiptRef:             receiptRef,
		receiptDigest:          receiptDigest,
		predecessor:            predecessor,
		predecessorComparison:  comparison,
		target:                 target,
		expectedGraphRevision:  typedmemory.NewGraphRevision(expected),
		committedGraphRevision: typedmemory.NewGraphRevision(committed),
		successorHead:          head,
		successorHeadDigest:    headDigest,
		event:                  event,
		commit:                 commit,
		committedResultRef:     resultRef,
		committedResultDigest:  resultDigest,
		canonicalBytes:         append([]byte(nil), canonical...),
	}, nil
}

func verifyCASWorkCoordinatesAgainstAuthority(
	coordinates ProjectTypeEnvHeadCASWorkCoordinates,
	authorityCoordinates ProjectTypeEnvHeadSelectionAuthorityCoordinates,
) error {
	subject := authorityCoordinates.ExecutionSubject()
	if err := verifyCASWorkExecutionSubject(
		subject,
		coordinates.WorkInterval(),
	); err != nil {
		return err
	}
	matches := coordinates.CoveringRoleAssignment() == subject.Ref() &&
		coordinates.ActualPerformerSystem() == subject.HolderSystemRef() &&
		coordinates.BoundedContext() == subject.BoundedContext()
	if !matches {
		return fmt.Errorf(
			"CAS Work execution coordinates differ from authority subject",
		)
	}
	return nil
}

func (record ProjectTypeEnvHeadCASWorkRecord) Ref() ProjectTypeEnvHeadCASWorkRecordRef {
	return record.ref
}

func (record ProjectTypeEnvHeadCASWorkRecord) Digest() typedmemory.SHA256Digest {
	return record.digest
}

func (record ProjectTypeEnvHeadCASWorkRecord) TransactionRef() ProjectTypeEnvHeadSelectionTransactionRef {
	return record.transactionRef
}

func (record ProjectTypeEnvHeadCASWorkRecord) TransactionDigest() typedmemory.SHA256Digest {
	return record.transactionDigest
}

func (record ProjectTypeEnvHeadCASWorkRecord) Project() projectidentity.ProjectID {
	return record.project
}

func (record ProjectTypeEnvHeadCASWorkRecord) WorkRef() authority.WorkRef {
	return record.workRef
}

func (record ProjectTypeEnvHeadCASWorkRecord) Coordinates() ProjectTypeEnvHeadCASWorkCoordinates {
	return record.coordinates
}

func (record ProjectTypeEnvHeadCASWorkRecord) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return record.requestRef
}

func (record ProjectTypeEnvHeadCASWorkRecord) RequestDigest() typedmemory.SHA256Digest {
	return record.requestDigest
}

func (record ProjectTypeEnvHeadCASWorkRecord) AuthorityCoordinates() ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	return record.authority
}

func (record ProjectTypeEnvHeadCASWorkRecord) AuthorityUseRecordRef() ProjectTypeEnvHeadSelectionAuthorityUseRecordRef {
	return record.authorityUseRef
}

func (record ProjectTypeEnvHeadCASWorkRecord) AuthorityUseRecordDigest() typedmemory.SHA256Digest {
	return record.authorityUseDigest
}

func (record ProjectTypeEnvHeadCASWorkRecord) ReceiptRef() ProjectTypeEnvHeadSelectionReceiptRef {
	return record.receiptRef
}

func (record ProjectTypeEnvHeadCASWorkRecord) ReceiptDigest() typedmemory.SHA256Digest {
	return record.receiptDigest
}

func (record ProjectTypeEnvHeadCASWorkRecord) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return record.predecessor
}

func (record ProjectTypeEnvHeadCASWorkRecord) PredecessorComparison() ProjectTypeEnvHeadPredecessorComparison {
	return record.predecessorComparison
}

func (record ProjectTypeEnvHeadCASWorkRecord) Target() ProjectTypeEnvHeadSelectionTarget {
	return record.target
}

func (record ProjectTypeEnvHeadCASWorkRecord) ExpectedGraphRevision() typedmemory.GraphRevision {
	return record.expectedGraphRevision
}

func (record ProjectTypeEnvHeadCASWorkRecord) CommittedGraphRevision() typedmemory.GraphRevision {
	return record.committedGraphRevision
}

func (record ProjectTypeEnvHeadCASWorkRecord) SuccessorHead() projecttypeenvselection.ProjectTypeEnvHeadState {
	return record.successorHead
}

func (record ProjectTypeEnvHeadCASWorkRecord) SuccessorHeadDigest() typedmemory.SHA256Digest {
	return record.successorHeadDigest
}

func (record ProjectTypeEnvHeadCASWorkRecord) EventRef() projecttypeenvselection.GraphEventRef {
	return record.event
}

func (record ProjectTypeEnvHeadCASWorkRecord) CommitRef() projecttypeenvselection.GraphCommitRef {
	return record.commit
}

func (record ProjectTypeEnvHeadCASWorkRecord) CommittedResultRef() ProjectTypeEnvHeadSelectionCommittedResultRef {
	return record.committedResultRef
}

func (record ProjectTypeEnvHeadCASWorkRecord) CommittedResultDigest() typedmemory.SHA256Digest {
	return record.committedResultDigest
}

func (record ProjectTypeEnvHeadCASWorkRecord) ProjectGraphUWorkAdmission() ProjectGraphUWorkAdmissionPosture {
	return ProjectGraphUWorkMembershipNotAssertedP8GV1()
}

func (record ProjectTypeEnvHeadCASWorkRecord) CanonicalBytes() []byte {
	return append([]byte(nil), record.canonicalBytes...)
}

func (record ProjectTypeEnvHeadCASWorkRecord) Verify() error {
	decoded, err := DecodeProjectTypeEnvHeadCASWorkRecord(record.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != record.ref || decoded.digest != record.digest {
		return fmt.Errorf("head CAS Work record differs from canonical bytes")
	}
	return nil
}
