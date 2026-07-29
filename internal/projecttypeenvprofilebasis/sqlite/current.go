// Package sqlite adapts the canonical project-profile ledger to the immutable
// project-TypeEnv profile basis used by Stage preparation and revalidation.
package sqlite

import (
	"context"
	"fmt"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// LoadCurrentWithin resolves the exact current canonical profile inside the
// caller-owned SQLite snapshot. It neither starts nor finishes a transaction.
func LoadCurrentWithin(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	root projectprofile.ProjectRootV1,
) (projecttypeenvprofilebasis.CurrentProjectProfileBasis, error) {
	observation, err := profileadmissionsqlite.ResolveCurrentWithin(
		ctx,
		transaction,
		root,
	)
	if err != nil {
		return nil, fmt.Errorf("reload current project profile: %w", err)
	}
	switch current := observation.(type) {
	case profileadmissionsqlite.NoCurrentCanonicalProfile:
		return projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(
			current.ProjectRoot(),
		)
	case profileadmissionsqlite.DeclaredCurrentCanonicalProfile:
		return FromCanonicalAdmission(current.Admission())
	default:
		return nil, fmt.Errorf(
			"current project-profile observation variant is invalid: %T",
			observation,
		)
	}
}

// FromCanonicalAdmission preserves the complete durable support DAG of the
// opaque admission token. It grants no profile or project-TypeEnv authority.
func FromCanonicalAdmission(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile, error) {
	if !admission.Valid() {
		return projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{},
			fmt.Errorf("canonical project-profile admission is required")
	}
	return projecttypeenvprofilebasis.NewDeclaredCanonicalProjectProfile(
		projecttypeenvprofilebasis.DeclaredProjectProfileBasisInput{
			ProjectRoot:                       admission.ProjectRoot(),
			LedgerRevision:                    admission.LedgerRevision(),
			Payload:                           admission.Payload(),
			AdmissionRecordRef:                admission.AdmissionRecordRef(),
			AdmissionRecordDigest:             admission.AdmissionRecordDigest(),
			AdmissionRecordCanonicalJSON:      admission.AdmissionRecordCanonicalJSON(),
			ReceiptDigest:                     admission.ReceiptDigest(),
			ReceiptCanonicalJSON:              admission.ReceiptCanonicalJSON(),
			CandidateProvenanceDigest:         admission.CandidateProvenanceDigest(),
			WorkRecordRef:                     admission.WorkRecordRef(),
			WorkRecordDigest:                  admission.WorkRecordDigest(),
			AuthorityBasisRef:                 admission.AuthorityBasisRef(),
			AuthorityBasisDigest:              admission.AuthorityBasisDigest(),
			AuthorityResolutionRef:            admission.AuthorityResolutionRef(),
			AuthorityResolutionDigest:         admission.AuthorityResolutionDigest(),
			ProfileAuthorRoleAssignmentRef:    admission.ProfileAuthorRoleAssignmentRef(),
			ProfileAuthorRoleAssignmentDigest: admission.ProfileAuthorRoleAssignmentDigest(),
			ObservedProjectBasisRef:           admission.ObservedProjectBasisRef(),
			ObservedProjectBasisDigest:        admission.ObservedProjectBasisDigest(),
			OutcomeAssessmentRef:              admission.OutcomeAssessmentRef(),
			OutcomeAssessmentDigest:           admission.OutcomeAssessmentDigest(),
		},
	)
}
