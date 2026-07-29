package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

const (
	specMigrationRecoveryRequiredCode = "migration_recovery_required"
	specMigrationRecoveryRejectedCode = "migration_recovery_rejected"
)

type specMigrationV2RecoveryObservation struct {
	carrier               specmigrationv2.FinalCandidatePacketCarrier
	packet                specmigrationv2.Packet
	structural            specMigrationV2StructuralObservation
	partitionAudit        specmigrationv2.PacketPartitionAudit
	reviewSoftwareCarrier string
}

func recoverSpecMigrationV2(
	ctx context.Context,
	ledger *specMigrationV2Ledger,
	root string,
	packetPath string,
) (specMigrationV2Result, error) {
	if ledger == nil || ledger.handle == nil {
		return specMigrationV2Result{}, fmt.Errorf("checked project ledger is required for migration recovery")
	}
	observation, err := observeSpecMigrationV2Recovery(root, packetPath)
	if err != nil {
		return specMigrationV2Result{}, err
	}
	result := presentSpecMigrationV2RecoveryResult(packetPath, observation)
	if err := ledger.handle.Revalidate(ctx); err != nil {
		return result, fmt.Errorf("revalidate checked project ledger before recovery: %w", err)
	}
	applyRoot, err := specmigrationv2.NewApplyProjectRoot(root)
	if err != nil {
		return result, err
	}
	request, err := specmigrationv2.NewRecoveryRequest(
		specmigrationv2.RecoveryRequestInput{
			ProjectRoot: applyRoot,
			Structural:  observation.structural.request,
		},
	)
	if err != nil {
		return result, err
	}
	recovery := specmigrationv2.RecoverMigration(
		ctx,
		ledger.profile,
		ledger.review,
		request,
	)
	if recovered, ok := recovery.(specmigrationv2.Applied); ok {
		result.State = "recovered"
		result.Applied = true
		result = presentSpecMigrationV2ReceiptCarrier(result, recovered.ReceiptCarrier())
		result.NextAction = "migration recovery completed; inspect the durable receipt and resulting carriers"
		return result, nil
	}
	if replayed, ok := recovery.(specmigrationv2.Replayed); ok {
		result.State = "replayed"
		result.Applied = true
		result = presentSpecMigrationV2ReceiptCarrier(result, replayed.ReceiptCarrier())
		result.NextAction = "completed migration replayed from its exact durable receipt"
		return result, nil
	}
	if required, blocked := recovery.(specmigrationv2.RecoveryRequired); blocked {
		result.State = "recovery_required"
		result.RecoveryPhase = string(required.Phase())
		result.RecoveryReason = required.Reason()
		result.NextAction = "repair the reported recovery precondition, then rerun haft spec migrate"
		return result, specMigrationPreconditionError{
			code: specMigrationRecoveryRequiredCode,
			detail: fmt.Sprintf(
				"migration recovery remains blocked at %s: %s",
				required.Phase(),
				required.Reason(),
			),
		}
	}
	if rejected, blocked := recovery.(specmigrationv2.ApplyRejected); blocked {
		result.State = "recovery_rejected"
		result.RecoveryReason = rejected.Reason()
		result.NextAction = "do not apply; repair the exact packet, journal, or carrier mismatch before recovery"
		return result, specMigrationPreconditionError{
			code:   specMigrationRecoveryRejectedCode,
			detail: fmt.Sprintf("migration recovery rejected (%s): %s", rejected.Code(), rejected.Reason()),
		}
	}
	return result, fmt.Errorf("migration recovery returned an unknown result variant")
}

func observeSpecMigrationV2Recovery(
	root string,
	packetPath string,
) (specMigrationV2RecoveryObservation, error) {
	carrierBytes, resolvedPacketPath, err := readPacketCarrier(packetPath)
	if err != nil {
		return specMigrationV2RecoveryObservation{}, err
	}
	carrier, err := specmigrationv2.DecodePacketCarrier(carrierBytes)
	if err != nil {
		return specMigrationV2RecoveryObservation{}, fmt.Errorf(
			"decode strict migration-v2 recovery candidate %s: %w",
			resolvedPacketPath,
			err,
		)
	}
	packet := carrier.Packet()
	reviewSoftwareCarrier, candidateBytes, err := observeReviewBasis(root, carrier.ReviewBasis())
	if err != nil {
		return specMigrationV2RecoveryObservation{}, err
	}
	if err := validateCLISoftwareMigrationPacket(packet, reviewSoftwareCarrier); err != nil {
		return specMigrationV2RecoveryObservation{}, err
	}
	sourceBytes, err := readSpecMigrationV2RecoverySource(root, packet)
	if err != nil {
		return specMigrationV2RecoveryObservation{}, err
	}
	structural, _, err := buildSpecMigrationV2StructuralRequestFromSourceBytes(
		root,
		packet,
		candidateBytes,
		sourceBytes,
	)
	if err != nil {
		return specMigrationV2RecoveryObservation{}, err
	}
	audit, err := specmigrationv2.AuditPacketCandidate(carrier, structural.request)
	if err != nil {
		return specMigrationV2RecoveryObservation{}, fmt.Errorf("audit migration-v2 recovery candidate: %w", err)
	}
	if audit.Status() != specmigrationv2.PacketPartitionAuditVerified {
		return specMigrationV2RecoveryObservation{}, fmt.Errorf(
			"migration-v2 recovery partition audit is %s with %d diagnostics",
			audit.Status(),
			len(audit.Diagnostics()),
		)
	}
	return specMigrationV2RecoveryObservation{
		carrier:               carrier,
		packet:                packet,
		structural:            structural,
		partitionAudit:        audit,
		reviewSoftwareCarrier: reviewSoftwareCarrier,
	}, nil
}

func readSpecMigrationV2RecoverySource(
	root string,
	packet specmigrationv2.Packet,
) ([]byte, error) {
	sourceCarrier := packet.Source().Carrier().String()
	archiveCarrier := packet.Source().Archive().Carrier().String()
	sourceBytes, sourceErr := readProjectRelativeCarrier(root, sourceCarrier)
	archiveBytes, archiveErr := readProjectRelativeCarrier(root, archiveCarrier)
	sourceFound := sourceErr == nil
	archiveFound := archiveErr == nil
	if sourceErr != nil && !errors.Is(sourceErr, os.ErrNotExist) {
		return nil, fmt.Errorf("read migration recovery source carrier: %w", sourceErr)
	}
	if archiveErr != nil && !errors.Is(archiveErr, os.ErrNotExist) {
		return nil, fmt.Errorf("read migration recovery archive carrier: %w", archiveErr)
	}
	if sourceFound && archiveFound {
		return nil, fmt.Errorf("migration recovery found both live source and designated archive carriers")
	}
	if !sourceFound && !archiveFound {
		return nil, fmt.Errorf("migration recovery found neither live source nor designated archive carrier")
	}
	selected := sourceBytes
	selectedCarrier := sourceCarrier
	if archiveFound {
		selected = archiveBytes
		selectedCarrier = archiveCarrier
	}
	observed := specmigrationv2.SourceDigestOf(selected)
	expected := packet.Source().Digest()
	if !observed.Equal(expected) {
		return nil, fmt.Errorf(
			"migration recovery source bytes at %s have digest %s, expected %s",
			filepath.ToSlash(selectedCarrier),
			observed.String(),
			expected.String(),
		)
	}
	return selected, nil
}

func presentSpecMigrationV2RecoveryResult(
	packetPath string,
	observation specMigrationV2RecoveryObservation,
) specMigrationV2Result {
	packet := observation.packet
	counts := observation.partitionAudit.Counts()
	return specMigrationV2Result{
		RecordKind:            specMigrationV2RecordKind,
		SchemaVersion:         packet.SchemaVersion(),
		State:                 "recovering",
		PacketID:              packet.ID().String(),
		PacketDigest:          observation.carrier.PacketDigest().String(),
		PacketCarrier:         filepath.Clean(packetPath),
		PacketCarrierDigest:   observation.carrier.CarrierDigest().String(),
		SourceCarrier:         packet.Source().Carrier().String(),
		SourceDigest:          packet.Source().Digest().String(),
		ReviewSoftwareCarrier: observation.reviewSoftwareCarrier,
		FinalTargetCarrier:    packet.Target().Carrier().String(),
		TargetDigest:          packet.Target().Digest().String(),
		PartitionAuditStatus:  string(observation.partitionAudit.Status()),
		PartitionAuditDigest:  observation.partitionAudit.Digest().String(),
		PartitionAuditCounts: specMigrationV2AuditCounts{
			SourceSections:       counts.SourceSections(),
			TopLevelDispositions: counts.TopLevelDispositions(),
			SplitSections:        counts.SplitSections(),
			SplitLeaves:          counts.SplitLeaves(),
			WholeSectionOutcomes: counts.WholeSectionOutcomes(),
			LineageEntries:       counts.LineageEntries(),
		},
		ProfileApplicability: "historical_journal_bound",
		RecoveryRequested:    true,
		Applied:              false,
		NextAction:           "continue the exact journal-bound migration effect",
	}
}
