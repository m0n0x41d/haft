package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

type specMigrationV2Operation string

const (
	specMigrationV2InspectOperation     specMigrationV2Operation = "inspect"
	specMigrationV2AdmitReviewOperation specMigrationV2Operation = "admit_review"
	specMigrationV2ApplyOperation       specMigrationV2Operation = "apply"
	specMigrationV2RecoverOperation     specMigrationV2Operation = "recover"
)

type specMigrationPreconditionError struct {
	code   string
	detail string
}

func (failure specMigrationPreconditionError) Error() string {
	return failure.code + ": " + failure.detail
}

func (failure specMigrationPreconditionError) Code() string {
	return failure.code
}

func runSpecMigrate(cmd *cobra.Command, _ []string) error {
	return runSpecMigrateWithReviewCapture(
		cmd,
		specmigrationv2.CaptureVerifiedMigrationReview,
	)
}

func runSpecMigrateWithReviewCapture(
	cmd *cobra.Command,
	capture specMigrationV2ReviewCapture,
) error {
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	candidate, err := resolveCurrentSpecMigrationCandidate(root)
	if err != nil {
		return err
	}
	packetPath := filepath.Join(
		root,
		filepath.FromSlash(candidate.CarrierRef()),
	)
	applyRoot, err := specmigrationv2.NewApplyProjectRoot(root)
	if err != nil {
		return err
	}
	journalState, err := specmigrationv2.InspectMigrationJournalState(
		applyRoot,
		candidate.Carrier().Packet(),
	)
	if err != nil {
		return err
	}
	if completed, ok := journalState.(specmigrationv2.CompletedMigrationJournal); ok {
		return presentCompletedSpecMigration(
			cmd,
			root,
			packetPath,
			completed,
		)
	}
	if pending, ok := journalState.(specmigrationv2.RecoveryPendingMigrationJournal); ok {
		if specMigrateJSON {
			return inspectSpecMigrationRecovery(cmd, root, packetPath, pending)
		}
		return runSpecMigrateV2OperationWithReviewCapture(
			cmd,
			root,
			packetPath,
			specMigrationV2RecoverOperation,
			capture,
		)
	}
	if _, ok := journalState.(specmigrationv2.NoMigrationJournal); !ok {
		return fmt.Errorf("specification migration journal inspection returned an unknown state")
	}
	if specMigrateJSON {
		return runSpecMigrateV2OperationWithReviewCapture(
			cmd,
			root,
			packetPath,
			specMigrationV2InspectOperation,
			capture,
		)
	}
	operation, err := resolveCurrentSpecMigrationOperation(
		cmd.Context(),
		root,
		packetPath,
	)
	if err != nil {
		return err
	}
	return runSpecMigrateV2OperationWithReviewCapture(
		cmd,
		root,
		packetPath,
		operation,
		capture,
	)
}

func resolveCurrentSpecMigrationCandidate(
	root string,
) (specmigrationv2.LocatedFinalCandidatePacket, error) {
	projectRoot, err := specmigrationv2.NewApplyProjectRoot(root)
	if err != nil {
		return specmigrationv2.LocatedFinalCandidatePacket{}, err
	}
	discovery, err := specmigrationv2.LocateFinalCandidatePackets(projectRoot)
	if err != nil {
		return specmigrationv2.LocatedFinalCandidatePacket{}, err
	}
	if one, ok := discovery.(specmigrationv2.OneFinalCandidatePacket); ok {
		return one.Candidate(), nil
	}
	if _, ok := discovery.(specmigrationv2.NoFinalCandidatePackets); ok {
		return specmigrationv2.LocatedFinalCandidatePacket{}, specMigrationPreconditionError{
			code:   "migration_candidate_not_prepared",
			detail: "this project has no prepared specification migration; run h-spec to prepare one",
		}
	}
	if many, ok := discovery.(specmigrationv2.ManyFinalCandidatePackets); ok {
		return specmigrationv2.LocatedFinalCandidatePacket{}, specMigrationPreconditionError{
			code:   "migration_candidate_ambiguous",
			detail: fmt.Sprintf("this project has %d prepared specification migrations; select one through h-spec before continuing", len(many.Candidates())),
		}
	}
	return specmigrationv2.LocatedFinalCandidatePacket{}, fmt.Errorf("specification migration discovery returned an unknown state")
}

func resolveCurrentSpecMigrationOperation(
	ctx context.Context,
	root string,
	packetPath string,
) (specMigrationV2Operation, error) {
	ledger, err := openSpecMigrationV2Ledger(ctx, root, projectledger.ReadOnly)
	if err != nil {
		return "", err
	}
	defer ledger.Close()
	observation, err := observeSpecMigrationV2WithProfile(
		ctx,
		root,
		packetPath,
		ledger.profile,
	)
	if err != nil {
		return "", err
	}
	review, err := resolveSpecMigrationV2Review(ctx, ledger.review, observation)
	if err != nil {
		return "", err
	}
	dryRunRequest, err := specmigrationv2.NewCanonicalDryRunRequest(
		specmigrationv2.CanonicalDryRunRequestInput{
			Packet:               observation.packet,
			ProjectRoot:          observation.projectRoot,
			ProfileApplicability: observation.profileApplicability,
			Review:               review,
			Source:               observation.structural.source,
			Target:               observation.structural.target,
			TargetClaims:         observation.structural.claims,
			OutsideSnapshots:     observation.structural.outside,
		},
	)
	if err != nil {
		return "", fmt.Errorf("construct canonical migration-v2 state probe: %w", err)
	}
	state := specmigrationv2.DryRun(dryRunRequest)
	if _, ok := state.(specmigrationv2.PendingReview); ok {
		return specMigrationV2AdmitReviewOperation, nil
	}
	if _, ok := state.(specmigrationv2.Applicable); ok {
		return specMigrationV2ApplyOperation, nil
	}
	return specMigrationV2InspectOperation, nil
}

func inspectSpecMigrationRecovery(
	cmd *cobra.Command,
	root string,
	packetPath string,
	pending specmigrationv2.RecoveryPendingMigrationJournal,
) error {
	observation, err := observeSpecMigrationV2Recovery(root, packetPath)
	if err != nil {
		return err
	}
	result := presentSpecMigrationV2RecoveryResult(packetPath, observation)
	result.State = "recovery_pending"
	result.RecoveryRequested = false
	result.RecoveryPhase = string(pending.Phase())
	result.RecoveryReason = pending.Reason()
	result.NextAction = "run haft spec migrate without --json to continue the sealed migration work"
	return writeSpecMigrationV2Result(cmd.OutOrStdout(), result, true)
}

func presentCompletedSpecMigration(
	cmd *cobra.Command,
	root string,
	packetPath string,
	completed specmigrationv2.CompletedMigrationJournal,
) error {
	observation, err := observeSpecMigrationV2Recovery(root, packetPath)
	if err != nil {
		return err
	}
	result := presentSpecMigrationV2RecoveryResult(packetPath, observation)
	result.State = "replayed"
	result.RecoveryRequested = false
	result.Applied = true
	result = presentSpecMigrationV2ReceiptCarrier(result, completed.ReceiptCarrier())
	result.NextAction = completedMigrationNextAction(completed.CurrentTargetState())
	return writeSpecMigrationV2Result(cmd.OutOrStdout(), result, specMigrateJSON)
}

func completedMigrationNextAction(
	state specmigrationv2.CurrentMigrationTargetState,
) string {
	if state == specmigrationv2.CurrentMigrationTargetExact {
		return "historical migration is complete; no migration action is required"
	}
	if state == specmigrationv2.CurrentMigrationTargetEvolved {
		return "historical migration is complete; the current target is a later lifecycle edition and no migration rerun is required"
	}
	if state == specmigrationv2.CurrentMigrationTargetAbsent {
		return "historical migration is complete; the current target is absent and should be handled through its current lifecycle, not by rerunning migration"
	}
	return "historical migration is complete; inspect the current target through its lifecycle without rerunning migration"
}
