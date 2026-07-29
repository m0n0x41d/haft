package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// Revalidator is the read-only project-ledger check between the committed
// authority resolution and performed Work. The shell invokes it while no SQL
// transaction is open; production must not pass a ceremonial no-op.
type Revalidator func(context.Context) error

// PrepareBeforeAdmission commits source-native v3 authority, revalidates the
// project ledger after that commit, and commits the exact v2 Work/value DAG in
// a second transaction. It deliberately stops before admission and exposes no
// database handle or mutation capability through its result.
func PrepareBeforeAdmission(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	input profiledeclarationpreparation.ProfileOnboardingWorkInput,
	policy profiledeclarationpreparation.Policy,
	clock func() time.Time,
	revalidate Revalidator,
) (Outcome, error) {
	if ctx == nil || database == nil || clock == nil || revalidate == nil {
		return nil, fmt.Errorf(
			"profile declaration preparation requires context, database, clock, and revalidator",
		)
	}
	checkedAt := clock().UTC().Round(0)
	probe, err := profiledeclarationpreparation.NewPlan(
		projectRoot,
		input,
		policy,
		checkedAt,
	)
	if err != nil {
		return nil, err
	}
	selected, authorityNew, conflict, err := prepareAuthorityStage(
		ctx,
		database,
		probe,
	)
	if err != nil {
		return nil, err
	}
	if conflict != "" {
		return Conflict{detail: conflict}, nil
	}
	if err := revalidate(ctx); err != nil {
		return nil, fmt.Errorf(
			"revalidate project ledger after profile authority commit: %w",
			err,
		)
	}
	prepared, workNew, conflict, err := prepareWorkStage(
		ctx,
		database,
		selected,
		clock,
	)
	if err != nil {
		return nil, err
	}
	if conflict != "" {
		return Conflict{detail: conflict}, nil
	}
	if authorityNew || workNew {
		return PreparedNew{prepared: prepared}, nil
	}
	return ExactExisting{prepared: prepared}, nil
}

func prepareAuthorityStage(
	ctx context.Context,
	database *sql.DB,
	probe profiledeclarationpreparation.Plan,
) (profiledeclarationpreparation.Plan, bool, string, error) {
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		return profiledeclarationpreparation.Plan{}, false, "", err
	}
	existing, presence, detail, err := recoverExistingAuthority(
		ctx,
		transaction,
		probe,
	)
	if err != nil {
		return profiledeclarationpreparation.Plan{}, false, "",
			rollbackPreparation(transaction, err)
	}
	if presence == authorityConflict {
		finish := transaction.Rollback(context.Background())
		return profiledeclarationpreparation.Plan{}, false, detail, finish.Err()
	}
	if presence == authorityExact {
		finish := transaction.Rollback(ctx)
		if !finish.Succeeded() {
			return profiledeclarationpreparation.Plan{}, false, "", finish.Err()
		}
		return existing.plan, false, "", nil
	}
	durableInput, err := StoreAndReloadProfileOnboardingWorkInput(
		ctx,
		transaction,
		probe.Input(),
		probe.PreparedAt(),
	)
	if err != nil {
		return profiledeclarationpreparation.Plan{}, false, "",
			rollbackPreparation(transaction, err)
	}
	if string(durableInput.CanonicalJSON()) != string(probe.Input().CanonicalJSON()) {
		err = fmt.Errorf("durable profile Work input differs from the reviewed input")
		return profiledeclarationpreparation.Plan{}, false, "",
			rollbackPreparation(transaction, err)
	}
	if err := storeAuthoritySupport(ctx, transaction, probe); err != nil {
		return profiledeclarationpreparation.Plan{}, false, "",
			rollbackPreparation(transaction, err)
	}
	bindingDigest, err := loadProjectBindingDigest(ctx, transaction, probe.Root())
	if err != nil {
		return profiledeclarationpreparation.Plan{}, false, "",
			rollbackPreparation(transaction, err)
	}
	basis, resolution, err := buildAuthorityRowsV3(probe, bindingDigest)
	if err != nil {
		return profiledeclarationpreparation.Plan{}, false, "",
			rollbackPreparation(transaction, err)
	}
	if err := persistAuthorityRowsV3(ctx, transaction, basis, resolution); err != nil {
		return profiledeclarationpreparation.Plan{}, false, "",
			rollbackPreparation(transaction, err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return profiledeclarationpreparation.Plan{}, false, "", fmt.Errorf(
			"commit v3 profile authority preparation: %w",
			finish.Err(),
		)
	}
	return probe, true, "", nil
}

func prepareWorkStage(
	ctx context.Context,
	database *sql.DB,
	plan profiledeclarationpreparation.Plan,
	clock func() time.Time,
) (Prepared, bool, string, error) {
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		return Prepared{}, false, "", err
	}
	existing, presence, detail, err := recoverExistingAuthority(
		ctx,
		transaction,
		plan,
	)
	if err != nil {
		return Prepared{}, false, "", rollbackPreparation(transaction, err)
	}
	if presence != authorityExact {
		finish := transaction.Rollback(context.Background())
		if presence == authorityAbsent {
			detail = "profile authority disappeared before performed Work"
		}
		return Prepared{}, false, detail, finish.Err()
	}
	plan = existing.plan
	times, existed, err := loadExistingOccurrenceTimes(ctx, transaction, plan)
	if err != nil {
		return Prepared{}, false, "", rollbackPreparation(transaction, err)
	}
	if !existed {
		times, err = observeOccurrenceTimes(clock, plan.PreparedAt())
		if err != nil {
			return Prepared{}, false, "", rollbackPreparation(transaction, err)
		}
	}
	values, err := plan.BuildValueSet(times)
	if err != nil {
		return Prepared{}, false, "", rollbackPreparation(transaction, err)
	}
	workWindow, err := times.WorkWindow()
	if err != nil {
		return Prepared{}, false, "", rollbackPreparation(transaction, err)
	}
	if err := storeValueSet(
		ctx,
		transaction,
		plan,
		values,
		workWindow.Until(),
	); err != nil {
		return Prepared{}, false, "", rollbackPreparation(transaction, err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return Prepared{}, false, "", fmt.Errorf(
			"commit v2 profile-onboarding Work: %w",
			finish.Err(),
		)
	}
	basisRef, err := plan.AuthorityBasisRef()
	if err != nil {
		return Prepared{}, false, "", err
	}
	candidate, err := plan.Candidate(values, basisRef)
	if err != nil {
		return Prepared{}, false, "", err
	}
	prepared, err := newPrepared(
		plan,
		values,
		candidate,
		existing.basis.digest,
		existing.resolution.digest,
	)
	if err != nil {
		return Prepared{}, false, "", err
	}
	return prepared, !existed, "", nil
}

func observeOccurrenceTimes(
	clock func() time.Time,
	checkedAt time.Time,
) (profiledeclarationpreparation.OccurrenceTimes, error) {
	workFrom, err := nextObservedTime(clock, checkedAt, 64)
	if err != nil {
		return profiledeclarationpreparation.OccurrenceTimes{}, err
	}
	basisFrom, err := nextObservedTime(clock, workFrom, 64)
	if err != nil {
		return profiledeclarationpreparation.OccurrenceTimes{}, err
	}
	basisUntil, err := nextObservedTime(clock, basisFrom, 64)
	if err != nil {
		return profiledeclarationpreparation.OccurrenceTimes{}, err
	}
	workUntil, err := nextObservedTime(clock, basisUntil, 64)
	if err != nil {
		return profiledeclarationpreparation.OccurrenceTimes{}, err
	}
	return profiledeclarationpreparation.NewOccurrenceTimes(
		workFrom,
		basisFrom,
		basisUntil,
		workUntil,
	)
}

func nextObservedTime(
	clock func() time.Time,
	previous time.Time,
	remaining int,
) (time.Time, error) {
	if remaining == 0 {
		return time.Time{}, fmt.Errorf(
			"profile declaration clock did not advance after %s",
			formatCanonicalTime(previous),
		)
	}
	observed := clock().UTC().Round(0)
	if observed.After(previous) {
		return observed, nil
	}
	return nextObservedTime(clock, previous, remaining-1)
}

func rollbackPreparation(
	transaction *sqlitetransaction.Transaction,
	cause error,
) error {
	finish := transaction.Rollback(context.Background())
	return errors.Join(cause, finish.Err())
}
