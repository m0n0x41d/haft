package sqlite

import (
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
)

const (
	genesisHeadSelectionMethodRefV1            = "method:project-typeenv-head-compare-and-swap/v1"
	genesisHeadSelectionMethodDescriptionRefV1 = "method-description:project-typeenv-head-compare-and-swap/v1"
	genesisHeadSelectionStatePlaneRefV1        = "state-plane:project-typeenv-head"
	genesisHeadSelectionResourceLedgerRefV1    = "resource-ledger:sqlite-immediate-transaction"
	genesisHeadSelectionOutcomeRefV1           = "outcome:project-typeenv-head-selected"
	genesisHeadSelectionAcceptanceRefV1        = "acceptance:exact-effect-closure-precommit"
	genesisHeadSelectionAuditTraceRefV1        = "audit-trace:project-typeenv-head-selection/v1"
	genesisHeadSelectionVerifierRefV1          = "project-typeenv-head-selection-verifier:genesis/v1"
	transitionHeadSelectionVerifierRefV1       = "project-typeenv-head-selection-verifier:transition/v1"
)

type genesisHeadSelectionMethodContract struct {
	method            authority.MethodRef
	methodDescription authority.MethodDescriptionRef
	statePlane        authority.StatePlaneRef
	resourceLedger    authority.ResourceLedgerRef
	outcome           authority.WorkOutcomeRef
	acceptance        authority.AcceptancePostureRef
	auditTrace        authority.AuditTraceRef
}

func currentGenesisHeadSelectionMethodContract() (
	genesisHeadSelectionMethodContract,
	error,
) {
	method, err := authority.NewMethodRef(genesisHeadSelectionMethodRefV1)
	if err != nil {
		return genesisHeadSelectionMethodContract{}, err
	}
	description, err := authority.NewMethodDescriptionRef(
		genesisHeadSelectionMethodDescriptionRefV1,
	)
	if err != nil {
		return genesisHeadSelectionMethodContract{}, err
	}
	statePlane, err := authority.NewStatePlaneRef(
		genesisHeadSelectionStatePlaneRefV1,
	)
	if err != nil {
		return genesisHeadSelectionMethodContract{}, err
	}
	resourceLedger, err := authority.NewResourceLedgerRef(
		genesisHeadSelectionResourceLedgerRefV1,
	)
	if err != nil {
		return genesisHeadSelectionMethodContract{}, err
	}
	outcome, err := authority.NewWorkOutcomeRef(
		genesisHeadSelectionOutcomeRefV1,
	)
	if err != nil {
		return genesisHeadSelectionMethodContract{}, err
	}
	acceptance, err := authority.NewAcceptancePostureRef(
		genesisHeadSelectionAcceptanceRefV1,
	)
	if err != nil {
		return genesisHeadSelectionMethodContract{}, err
	}
	auditTrace, err := authority.NewAuditTraceRef(
		genesisHeadSelectionAuditTraceRefV1,
	)
	if err != nil {
		return genesisHeadSelectionMethodContract{}, err
	}
	return genesisHeadSelectionMethodContract{
		method:            method,
		methodDescription: description,
		statePlane:        statePlane,
		resourceLedger:    resourceLedger,
		outcome:           outcome,
		acceptance:        acceptance,
		auditTrace:        auditTrace,
	}, nil
}

func (contract genesisHeadSelectionMethodContract) workCoordinates(
	authorityCoordinates projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityCoordinates,
	startedAt time.Time,
	sealedAt time.Time,
) (projecttypeenvselectioneffect.ProjectTypeEnvHeadCASWorkCoordinates, error) {
	window, err := authority.NewTimeWindow(
		startedAt.Round(0).UTC(),
		sealedAt.Round(0).UTC(),
	)
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadCASWorkCoordinates{},
			fmt.Errorf("seal Genesis CAS Work interval: %w", err)
	}
	return projecttypeenvselectioneffect.NewProjectTypeEnvHeadCASWorkCoordinates(
		projecttypeenvselectioneffect.ProjectTypeEnvHeadCASWorkCoordinatesInput{
			Method:            contract.method,
			MethodDescription: contract.methodDescription,
			Authority:         authorityCoordinates,
			WorkInterval:      window,
			StatePlane:        contract.statePlane,
			ResourceLedger:    contract.resourceLedger,
			Outcome:           contract.outcome,
			Acceptance:        contract.acceptance,
			AuditTrace:        contract.auditTrace,
		},
	)
}

func currentHeadSelectionVerifier(
	predecessor projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) (
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierRef,
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierEdition,
	error,
) {
	refText := ""
	switch predecessor.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		refText = genesisHeadSelectionVerifierRefV1
	case projecttypeenvselection.TransitionStagePredecessor:
		refText = transitionHeadSelectionVerifierRefV1
	default:
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierRef{},
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierEdition{},
			fmt.Errorf("head-selection predecessor variant is unsupported")
	}
	ref, err := projecttypeenvselectioneffect.NewProjectTypeEnvHeadSelectionVerifierRef(
		refText,
	)
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierRef{},
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierEdition{},
			err
	}
	edition, err := projecttypeenvselectioneffect.NewProjectTypeEnvHeadSelectionVerifierEdition(1)
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierRef{},
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionVerifierEdition{},
			err
	}
	return ref, edition, nil
}
