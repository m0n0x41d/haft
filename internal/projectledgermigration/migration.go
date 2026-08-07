package projectledgermigration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

type Outcome string

const (
	OutcomeApplied        Outcome = "applied"
	OutcomeAlreadyCurrent Outcome = "already_current"
)

type Request struct {
	root    projectledger.ProjectRoot
	project projectledger.ProjectID
}

func NewRequest(rawRoot string, rawProjectID string) (Request, error) {
	root, err := canonicalProjectRoot(rawRoot)
	if err != nil {
		return Request{}, err
	}
	projectID, err := projectledger.ParseProjectID(rawProjectID)
	if err != nil {
		return Request{}, fmt.Errorf("parse expected project identity: %w", err)
	}
	return Request{
		root:    root,
		project: projectID,
	}, nil
}

func canonicalProjectRoot(raw string) (projectledger.ProjectRoot, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return projectledger.ProjectRoot{}, fmt.Errorf("project root is required")
	}
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return projectledger.ProjectRoot{}, fmt.Errorf("resolve project root: %w", err)
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return projectledger.ProjectRoot{}, fmt.Errorf("resolve physical project root: %w", err)
	}
	root, err := projectledger.NewProjectRoot(filepath.Clean(physical))
	if err != nil {
		return projectledger.ProjectRoot{}, err
	}
	return root, nil
}

type Result struct {
	ProjectRoot  string
	ProjectID    string
	DatabasePath string
	BeforeSchema int
	AfterSchema  int
	Outcome      Outcome
}

type SchemaObservation struct {
	ProjectRoot    string
	ProjectID      string
	DatabasePath   string
	ObservedSchema int
	CompiledSchema int
}

func Observe(
	ctx context.Context,
	request Request,
) (SchemaObservation, error) {
	if ctx == nil {
		return SchemaObservation{},
			fmt.Errorf("observe project ledger: context is required")
	}
	identity, err := projectledger.LoadIdentity(request.root.String())
	if err != nil {
		return SchemaObservation{},
			fmt.Errorf("load exact project identity: %w", err)
	}
	if identity.ProjectID() != request.project {
		return SchemaObservation{}, fmt.Errorf(
			"project identity mismatch: root carries %s, expected %s",
			identity.ProjectID().String(),
			request.project.String(),
		)
	}
	handle, err := projectledger.OpenForExplicitMigration(
		ctx,
		request.root.String(),
		projectledger.ReadOnly,
	)
	if err != nil {
		return SchemaObservation{},
			fmt.Errorf("open exact project ledger read-only: %w", err)
	}
	observed, observeErr := observeSchemaFrontier(
		ctx,
		handle.Database(),
	)
	var prefixErr error
	var bindingErr error
	if observeErr == nil {
		prefixErr = db.RequireSchemaPrefixReadOnly(
			ctx,
			handle.Database(),
			observed,
		)
		if prefixErr == nil {
			bindingErr = requireBindingForObservedSchema(
				ctx,
				handle,
				observed,
			)
		}
	}
	compiled, compiledErr := db.CurrentSchemaVersion()
	closeErr := handle.Close()
	if err := errors.Join(
		observeErr,
		prefixErr,
		bindingErr,
		compiledErr,
		closeErr,
	); err != nil {
		return SchemaObservation{}, err
	}
	return SchemaObservation{
		ProjectRoot:    request.root.String(),
		ProjectID:      request.project.String(),
		DatabasePath:   handle.DatabasePath(),
		ObservedSchema: observed,
		CompiledSchema: compiled,
	}, nil
}

func Apply(ctx context.Context, request Request) (Result, error) {
	return apply(
		ctx,
		request,
		transitionExpectation{kind: transitionAnyAdditive},
	)
}

type ExactTransition struct {
	before int
	after  int
}

func NewExactTransition(
	before int,
	after int,
) (ExactTransition, error) {
	if before <= 0 || after <= before {
		return ExactTransition{}, fmt.Errorf(
			"exact additive transition is invalid: %d -> %d",
			before,
			after,
		)
	}
	return ExactTransition{before: before, after: after}, nil
}

func (transition ExactTransition) BeforeSchema() int {
	return transition.before
}

func (transition ExactTransition) AfterSchema() int {
	return transition.after
}

func ApplyExact(
	ctx context.Context,
	request Request,
	transition ExactTransition,
) (Result, error) {
	if transition.before <= 0 || transition.after <= transition.before {
		return Result{}, fmt.Errorf("migrate project ledger: exact transition is invalid")
	}
	return apply(
		ctx,
		request,
		transitionExpectation{
			kind:   transitionExactAdditive,
			before: transition.before,
			after:  transition.after,
		},
	)
}

type transitionExpectationKind string

const (
	transitionAnyAdditive   transitionExpectationKind = "any_additive"
	transitionExactAdditive transitionExpectationKind = "exact_additive"
)

type transitionExpectation struct {
	kind   transitionExpectationKind
	before int
	after  int
}

func apply(
	ctx context.Context,
	request Request,
	expectation transitionExpectation,
) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("migrate project ledger: context is required")
	}
	identity, err := projectledger.LoadIdentity(request.root.String())
	if err != nil {
		return Result{}, fmt.Errorf("load exact project identity: %w", err)
	}
	if identity.ProjectID() != request.project {
		return Result{}, fmt.Errorf(
			"project identity mismatch: root carries %s, expected %s",
			identity.ProjectID().String(),
			request.project.String(),
		)
	}
	handle, err := projectledger.OpenForExplicitMigration(
		ctx,
		request.root.String(),
		projectledger.ReadWrite,
	)
	if err != nil {
		return Result{}, fmt.Errorf("open exact project ledger: %w", err)
	}
	result, applyErr := applyToHandle(
		ctx,
		request,
		handle,
		expectation,
	)
	closeErr := handle.Close()
	if applyErr != nil {
		return Result{}, errors.Join(applyErr, closeErr)
	}
	if closeErr != nil {
		return Result{}, fmt.Errorf("close migrated project ledger: %w", closeErr)
	}
	return result, nil
}

func applyToHandle(
	ctx context.Context,
	request Request,
	handle *projectledger.Handle,
	expectation transitionExpectation,
) (Result, error) {
	before, err := observeSchemaFrontier(ctx, handle.Database())
	if err != nil {
		return Result{}, err
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		return Result{}, fmt.Errorf("resolve compiled schema frontier: %w", err)
	}
	if before > current {
		return Result{}, fmt.Errorf(
			"project schema %d is newer than this Haft binary schema %d",
			before,
			current,
		)
	}
	if err := requireTransitionExpectation(
		expectation,
		before,
		current,
	); err != nil {
		return Result{}, err
	}
	if err := db.RequireSchemaPrefixReadOnly(
		ctx,
		handle.Database(),
		before,
	); err != nil {
		return Result{}, err
	}
	if err := requireBindingForObservedSchema(
		ctx,
		handle,
		before,
	); err != nil {
		return Result{}, err
	}
	if before < db.ProjectLedgerBindingSchemaVersion {
		bindingAt := time.Now().UTC()
		bindingTransitionRan := false
		if err := db.RunMigrationsThroughProjectLedgerBinding(
			handle.Database(),
			func(transaction db.MigrationTransaction) error {
				if err := handle.BindDuringFirstDurableSchemaMigration(
					ctx,
					transaction,
					bindingAt,
				); err != nil {
					return err
				}
				bindingTransitionRan = true
				return nil
			},
		); err != nil {
			return Result{}, fmt.Errorf(
				"apply migrations through project binding: %w",
				err,
			)
		}
		if err := handle.Revalidate(ctx); err != nil {
			if bindingTransitionRan {
				return Result{}, errors.Join(
					projectledger.ErrBindingCommittedTopologyChanged,
					err,
				)
			}
			return Result{}, fmt.Errorf(
				"verify concurrently migrated project identity: %w",
				err,
			)
		}
	}
	if err := db.RunMigrations(handle.Database()); err != nil {
		return Result{}, fmt.Errorf("apply additive project migrations: %w", err)
	}
	if err := db.RequireCurrentSchemaReadOnly(ctx, handle.Database()); err != nil {
		return Result{}, fmt.Errorf("verify migrated project schema: %w", err)
	}
	if err := handle.RequireAttachedIdentity(ctx); err != nil {
		return Result{}, fmt.Errorf("verify migrated project identity: %w", err)
	}
	if err := handle.Revalidate(ctx); err != nil {
		return Result{}, fmt.Errorf("revalidate migrated project topology: %w", err)
	}
	after, err := observeSchemaFrontier(ctx, handle.Database())
	if err != nil {
		return Result{}, err
	}
	return newResult(
		request,
		handle.DatabasePath(),
		before,
		after,
		current,
	)
}

func requireBindingForObservedSchema(
	ctx context.Context,
	handle *projectledger.Handle,
	observedSchema int,
) error {
	if observedSchema < db.ProjectLedgerBindingSchemaVersion {
		return nil
	}
	if err := handle.RequireAttachedIdentity(ctx); err != nil {
		recoveryCommand := fmt.Sprintf(
			"haft project recover-binding --project-root %q --project-id %s",
			handle.ProjectRoot().String(),
			handle.ProjectID().String(),
		)
		return fmt.Errorf(
			"schema %d requires a durable project identity binding: %w; run `%s` for explicit backed-up recovery",
			observedSchema,
			err,
			recoveryCommand,
		)
	}
	return nil
}

func requireTransitionExpectation(
	expectation transitionExpectation,
	observedBefore int,
	compiledAfter int,
) error {
	switch expectation.kind {
	case transitionAnyAdditive:
		if expectation.before == 0 && expectation.after == 0 {
			return nil
		}
	case transitionExactAdditive:
		if expectation.before == observedBefore &&
			expectation.after == compiledAfter {
			return nil
		}
		return fmt.Errorf(
			"project schema transition differs from exact plan: observed %d -> compiled %d, expected %d -> %d; no migration was attempted",
			observedBefore,
			compiledAfter,
			expectation.before,
			expectation.after,
		)
	}
	return fmt.Errorf("project schema transition expectation is invalid")
}

func observeSchemaFrontier(ctx context.Context, database *sql.DB) (int, error) {
	frontier := 0
	err := database.QueryRowContext(
		ctx,
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&frontier)
	if err != nil {
		return 0, fmt.Errorf("read project schema frontier: %w", err)
	}
	return frontier, nil
}

func newResult(
	request Request,
	databasePath string,
	before int,
	after int,
	current int,
) (Result, error) {
	canonicalDatabasePath := filepath.Clean(databasePath)
	if databasePath == "" ||
		!filepath.IsAbs(databasePath) ||
		canonicalDatabasePath != databasePath {
		return Result{}, fmt.Errorf("invalid project database path in migration result")
	}
	if before <= 0 || after != current || before > after {
		return Result{}, fmt.Errorf(
			"invalid project schema transition %d -> %d with current %d",
			before,
			after,
			current,
		)
	}
	outcome := OutcomeApplied
	if before == after {
		outcome = OutcomeAlreadyCurrent
	}
	return Result{
		ProjectRoot:  request.root.String(),
		ProjectID:    request.project.String(),
		DatabasePath: databasePath,
		BeforeSchema: before,
		AfterSchema:  after,
		Outcome:      outcome,
	}, nil
}
