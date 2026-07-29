package specmigrationv2

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

// MigrationJournalState is the read-only state of the durable migration saga.
// It reports historical completion independently from later evolution of the
// installed target carrier.
type MigrationJournalState interface {
	migrationJournalStateVariant()
}

type NoMigrationJournal interface {
	MigrationJournalState
	noMigrationJournalVariant()
}

type noMigrationJournal struct{}

func (noMigrationJournal) migrationJournalStateVariant() {}
func (noMigrationJournal) noMigrationJournalVariant()    {}

type RecoveryPendingMigrationJournal interface {
	MigrationJournalState
	Phase() JournalPhase
	Reason() string
	recoveryPendingMigrationJournalVariant()
}

type recoveryPendingMigrationJournal struct {
	phase  JournalPhase
	reason string
}

func (recoveryPendingMigrationJournal) migrationJournalStateVariant()           {}
func (recoveryPendingMigrationJournal) recoveryPendingMigrationJournalVariant() {}

func (state recoveryPendingMigrationJournal) Phase() JournalPhase {
	return state.phase
}

func (state recoveryPendingMigrationJournal) Reason() string {
	return state.reason
}

type CurrentMigrationTargetState string

const (
	CurrentMigrationTargetExact      CurrentMigrationTargetState = "exact_migrated_edition"
	CurrentMigrationTargetEvolved    CurrentMigrationTargetState = "later_edition"
	CurrentMigrationTargetAbsent     CurrentMigrationTargetState = "absent"
	CurrentMigrationTargetUnreadable CurrentMigrationTargetState = "unreadable"
)

type CompletedMigrationJournal interface {
	MigrationJournalState
	Receipt() MigrationEffectReceipt
	ReceiptCarrier() MigrationEffectReceiptCarrier
	CurrentTargetState() CurrentMigrationTargetState
	completedMigrationJournalVariant()
}

type completedMigrationJournal struct {
	receipt            MigrationEffectReceipt
	receiptCarrier     MigrationEffectReceiptCarrier
	currentTargetState CurrentMigrationTargetState
}

func (completedMigrationJournal) migrationJournalStateVariant()     {}
func (completedMigrationJournal) completedMigrationJournalVariant() {}

func (state completedMigrationJournal) Receipt() MigrationEffectReceipt {
	return state.receipt
}

func (state completedMigrationJournal) ReceiptCarrier() MigrationEffectReceiptCarrier {
	return state.receiptCarrier
}

func (state completedMigrationJournal) CurrentTargetState() CurrentMigrationTargetState {
	return state.currentTargetState
}

// InspectMigrationJournalState reads the package-owned saga carriers without
// mutating them. A completed journal remains historical completion when the
// current target later evolves; only the journal-bound lineage and receipt are
// used to prove that completion.
func InspectMigrationJournalState(
	root ApplyProjectRoot,
	packet Packet,
) (MigrationJournalState, error) {
	paths, packetDigest, lineageDigest, err := journalInspectionBasis(root, packet)
	if err != nil {
		return nil, err
	}
	journal, found, err := loadJournal(paths.journal)
	if err != nil {
		return nil, fmt.Errorf("inspect durable migration journal: %w", err)
	}
	if !found {
		return inspectTemporaryMigrationJournal(paths, root, packet, packetDigest, lineageDigest)
	}
	if err := validateInspectionJournal(
		journal,
		root,
		packet,
		packetDigest,
		lineageDigest,
	); err != nil {
		return nil, fmt.Errorf("inspect durable migration journal identity: %w", err)
	}
	if journal.phase != JournalCompleted {
		return recoveryPendingMigrationJournal{
			phase:  journal.phase,
			reason: "durable migration journal has not reached completed",
		}, nil
	}
	return inspectCompletedMigrationJournal(paths, root, journal)
}

func journalInspectionBasis(
	root ApplyProjectRoot,
	packet Packet,
) (migrationEffectPaths, PacketDigest, LineagePolicyDigest, error) {
	if !root.valid() {
		return migrationEffectPaths{}, PacketDigest{}, LineagePolicyDigest{}, fmt.Errorf("migration journal inspection root is invalid")
	}
	packetDigest, err := PacketDigestOf(packet)
	if err != nil {
		return migrationEffectPaths{}, PacketDigest{}, LineagePolicyDigest{}, fmt.Errorf("migration journal inspection packet is invalid: %w", err)
	}
	lineagePolicy := packet.LineagePolicy()
	lineageDigest, err := LineagePolicyDigestOf(lineagePolicy)
	if err != nil {
		return migrationEffectPaths{}, PacketDigest{}, LineagePolicyDigest{}, fmt.Errorf("migration journal inspection lineage is invalid: %w", err)
	}
	source := packet.Source()
	target := packet.Target()
	request := ApplyRequest{
		projectRoot: root,
		analysis: structuralAnalysis{
			packetID:       packet.ID(),
			sourceCarrier:  source.Carrier(),
			targetCarrier:  target.Carrier(),
			archiveCarrier: source.Archive().Carrier(),
		},
	}
	return effectPaths(request), packetDigest, lineageDigest, nil
}

func inspectTemporaryMigrationJournal(
	paths migrationEffectPaths,
	root ApplyProjectRoot,
	packet Packet,
	packetDigest PacketDigest,
	lineageDigest LineagePolicyDigest,
) (MigrationJournalState, error) {
	temporary, found, err := loadJournal(paths.journal + ".tmp")
	if err != nil {
		return nil, fmt.Errorf("inspect temporary migration journal: %w", err)
	}
	if !found {
		return noMigrationJournal{}, nil
	}
	if err := validateInspectionJournal(
		temporary,
		root,
		packet,
		packetDigest,
		lineageDigest,
	); err != nil {
		return nil, fmt.Errorf("inspect temporary migration journal identity: %w", err)
	}
	return recoveryPendingMigrationJournal{
		phase:  temporary.phase,
		reason: "prepared temporary migration journal requires recovery",
	}, nil
}

func validateInspectionJournal(
	journal migrationJournal,
	root ApplyProjectRoot,
	packet Packet,
	packetDigest PacketDigest,
	lineageDigest LineagePolicyDigest,
) error {
	source := packet.Source()
	target := packet.Target()
	archive := source.Archive()
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: journal.migrationID.String() == packet.ID().String(), name: "migration ID"},
		{matches: journal.packetDigest.Equal(packetDigest), name: "packet digest"},
		{matches: journal.projectRoot.String() == root.String(), name: "project root"},
		{matches: journal.sourceCarrier.String() == source.Carrier().String(), name: "source carrier"},
		{matches: journal.sourceDigest.Equal(source.Digest()), name: "source digest"},
		{matches: journal.targetCarrier.String() == target.Carrier().String(), name: "target carrier"},
		{matches: journal.targetDigest.Equal(target.Digest()), name: "target digest"},
		{matches: journal.archiveCarrier.String() == archive.Carrier().String(), name: "archive carrier"},
		{matches: journal.lineageDigest.String() == lineageDigest.String(), name: "lineage-policy digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("journal %s does not match the final-candidate packet", check.name)
		}
	}
	return nil
}

func inspectCompletedMigrationJournal(
	paths migrationEffectPaths,
	root ApplyProjectRoot,
	journal migrationJournal,
) (MigrationJournalState, error) {
	lineageExact, lineageReason := exactHistoricalCarrier(
		paths.lineage,
		journal.lineageRecordDigest,
		"lineage",
	)
	if !lineageExact {
		return recoveryPendingMigrationJournal{
			phase:  journal.phase,
			reason: lineageReason,
		}, nil
	}
	receiptBytes, err := readRegularFileNoFollow(paths.receipt)
	if err != nil {
		return recoveryPendingMigrationJournal{
			phase:  journal.phase,
			reason: "completed journal receipt is unavailable: " + err.Error(),
		}, nil
	}
	observedReceiptDigest := DigestBytes(receiptBytes)
	if !observedReceiptDigest.Equal(journal.receiptDigest) {
		return recoveryPendingMigrationJournal{
			phase:  journal.phase,
			reason: "completed journal receipt digest does not match",
		}, nil
	}
	receipt, err := decodeReceipt(receiptBytes)
	if err != nil {
		return recoveryPendingMigrationJournal{
			phase:  journal.phase,
			reason: "completed journal receipt is invalid: " + err.Error(),
		}, nil
	}
	expectedReceiptBytes, err := encodeReceipt(receiptFromJournal(journal))
	if err != nil {
		return nil, fmt.Errorf("reconstruct completed journal receipt: %w", err)
	}
	if !bytes.Equal(receiptBytes, expectedReceiptBytes) {
		return recoveryPendingMigrationJournal{
			phase:  journal.phase,
			reason: "completed journal receipt does not match journal provenance",
		}, nil
	}
	receiptCarrier, err := newMigrationEffectReceiptCarrier(
		root,
		paths.receipt,
		journal.receiptDigest,
	)
	if err != nil {
		return nil, fmt.Errorf("bind completed migration receipt carrier: %w", err)
	}
	targetState := inspectCurrentMigrationTarget(paths.target, journal.targetDigest)
	return completedMigrationJournal{
		receipt:            receipt,
		receiptCarrier:     receiptCarrier,
		currentTargetState: targetState,
	}, nil
}

func exactHistoricalCarrier(
	path string,
	expected SHA256,
	label string,
) (bool, string) {
	content, err := readRegularFileNoFollow(path)
	if err != nil {
		return false, "completed journal " + label + " carrier is unavailable: " + err.Error()
	}
	observed := DigestBytes(content)
	if !observed.Equal(expected) {
		return false, "completed journal " + label + " carrier digest does not match"
	}
	return true, ""
}

func inspectCurrentMigrationTarget(
	path string,
	expected TargetDigest,
) CurrentMigrationTargetState {
	content, err := readRegularFileNoFollow(path)
	if errors.Is(err, os.ErrNotExist) {
		return CurrentMigrationTargetAbsent
	}
	if err != nil {
		return CurrentMigrationTargetUnreadable
	}
	observed := TargetDigestOf(content)
	if observed.Equal(expected) {
		return CurrentMigrationTargetExact
	}
	return CurrentMigrationTargetEvolved
}
