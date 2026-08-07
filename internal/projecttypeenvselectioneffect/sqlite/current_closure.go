package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	ErrCurrentCommittedClosureUnavailable = errors.New(
		"current committed project TypeEnv selection closure is unavailable",
	)
	ErrCurrentCommittedClosureUncorrelated = errors.New(
		"current committed project TypeEnv selection closure is uncorrelated",
	)
)

// CurrentCommittedClosureLoader recovers one already-committed selection
// closure through a caller-owned read transaction. It exposes no transaction,
// head, Stage, or mutation capability.
type CurrentCommittedClosureLoader struct{}

func NewCurrentCommittedClosureLoader() CurrentCommittedClosureLoader {
	return CurrentCommittedClosureLoader{}
}

func (CurrentCommittedClosureLoader) LoadCommittedClosureForCurrentHeadTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectidentity.ProjectID,
	graphRevision typedmemory.GraphRevision,
	head projecttypeenvselection.ProjectTypeEnvHeadRef,
	headRevision projecttypeenvselection.HeadRevision,
	selected typedmemory.TypeEnvRef,
) (
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
	error,
) {
	if ctx == nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			sqlitetransaction.ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}
	if transaction == nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}
	if err := verifyCurrentCommittedClosureCoordinates(
		project,
		graphRevision,
		head,
		headRevision,
		selected,
	); err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}
	storedGraphRevision, err := exactSQLiteInteger(
		"current committed closure graph revision",
		graphRevision.Value(),
	)
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}
	storedHeadRevision, err := exactSQLiteInteger(
		"current committed closure head revision",
		headRevision.Value(),
	)
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}

	row := currentCommittedClosureRow{}
	err = transaction.ScanOne(
		ctx,
		currentCommittedClosureSQL,
		[]any{
			project.String(),
			storedGraphRevision,
			head.String(),
			storedHeadRevision,
			selected.String(),
		},
		[]any{
			&row.ref,
			&row.digest,
			&row.canonical,
			&row.idempotencyKey,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf(
				"%w: no closure matches the active project/head/C at or before the current graph revision",
				ErrCurrentCommittedClosureUnavailable,
			)
	}
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf("load current committed TypeEnv selection closure: %w", err)
	}

	closure, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvHeadSelectionClosureV1(
			row.canonical,
		)
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			corruptReplay("current closure canonical bytes", err)
	}
	key, err :=
		projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
			row.idempotencyKey,
		)
	if err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			corruptReplay("current closure idempotency key", err)
	}
	if err := verifyReplayClosure(
		closure,
		project,
		key,
		closure.RequestDigest(),
		closure.AuthorityCoordinates().ContentDigest(),
		row.ref,
		row.digest,
	); err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}
	if err := verifyCurrentCommittedClosureResult(
		closure,
		graphRevision,
		head,
		headRevision,
		selected,
	); err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}
	if err := verifyStoredReplayDAGTx(
		ctx,
		transaction,
		project,
		key,
		closure,
	); err != nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1{},
			err
	}
	return closure, nil
}

type currentCommittedClosureRow struct {
	ref            string
	digest         string
	canonical      []byte
	idempotencyKey string
}

const currentCommittedClosureSQL = `
SELECT
	closure.closure_ref,
	closure.closure_digest,
	closure.canonical_bytes,
	authority_use.original_idempotency_key
FROM project_typeenv_head_selection_closures closure
JOIN project_typeenv_head_selection_authority_uses authority_use
	ON authority_use.authority_use_ref = closure.authority_use_ref
	AND authority_use.authority_use_digest = closure.authority_use_digest
JOIN project_typeenv_head_history history
	ON history.project_id = closure.project_id
	AND history.head_ref = closure.head_ref
	AND history.head_revision = closure.head_revision
	AND history.head_state_digest = closure.head_state_digest
	AND history.graph_revision = closure.graph_revision
	AND history.graph_event_ref = closure.graph_event_ref
	AND history.graph_commit_ref = closure.graph_commit_ref
WHERE closure.project_id = ?
	AND closure.graph_revision <= ?
	AND closure.head_ref = ?
	AND closure.head_revision = ?
	AND history.selected_composite_ref = ?
ORDER BY closure.graph_revision DESC
LIMIT 1
`

func verifyCurrentCommittedClosureCoordinates(
	project projectidentity.ProjectID,
	graphRevision typedmemory.GraphRevision,
	head projecttypeenvselection.ProjectTypeEnvHeadRef,
	headRevision projecttypeenvselection.HeadRevision,
	selected typedmemory.TypeEnvRef,
) error {
	parsedProject, err := projectidentity.ParseProjectID(project.String())
	if err != nil || parsedProject != project {
		return fmt.Errorf("current closure project identity is required")
	}
	if graphRevision.Value() == 0 {
		return fmt.Errorf("current closure graph revision is required")
	}
	parsedHead, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(
		head.String(),
	)
	if err != nil || parsedHead != head || head.Project() != project {
		return fmt.Errorf("current closure project head is required")
	}
	if headRevision.Value() == 0 {
		return fmt.Errorf("current closure head revision is required")
	}
	parsedSelected, err := typedmemory.ParseTypeEnvRef(selected.String())
	if err != nil || parsedSelected != selected {
		return fmt.Errorf("current closure selected TypeEnv is required")
	}
	return nil
}

func verifyCurrentCommittedClosureResult(
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
	graphRevision typedmemory.GraphRevision,
	head projecttypeenvselection.ProjectTypeEnvHeadRef,
	headRevision projecttypeenvselection.HeadRevision,
	selected typedmemory.TypeEnvRef,
) error {
	successor := closure.SuccessorHead()
	matches := closure.CommittedGraphRevision().Value() <= graphRevision.Value() &&
		successor.Ref() == head &&
		successor.Revision() == headRevision &&
		successor.SelectedComposite() == selected &&
		closure.Target().Composite() == selected
	if !matches {
		return fmt.Errorf(
			"%w: decoded closure differs from active graph/head/C coordinates",
			ErrCurrentCommittedClosureUncorrelated,
		)
	}
	return nil
}
