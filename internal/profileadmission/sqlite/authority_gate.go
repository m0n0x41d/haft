package sqlite

import (
	"context"

	"github.com/m0n0x41d/haft/internal/profileauthority"
	profileauthoritysqlite "github.com/m0n0x41d/haft/internal/profileauthority/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// profileAuthorityGate is the only authority dependency admitted by the
// profile writer. Implementations must resolve the exact source-native basis
// inside the caller's BEGIN IMMEDIATE transaction and return only a sealed
// package-minted use.
type profileAuthorityGate interface {
	PrepareClosureSnapshotForBasis(
		context.Context,
		profileauthority.BasisRef,
	) (profileauthoritysqlite.ClosureSnapshot, error)
	ResolveForAdmission(
		context.Context,
		*sqlitetransaction.Transaction,
		profileauthoritysqlite.ClosureSnapshot,
	) (profileauthority.AdmittedUse, []profileauthority.ResolutionDenial, error)
	ResolveAuthorityUseForBasisInTransaction(
		context.Context,
		*sqlitetransaction.Transaction,
		profileauthoritysqlite.ClosureSnapshot,
	) (profileauthority.AuthorityUseRecord, bool, error)
}
