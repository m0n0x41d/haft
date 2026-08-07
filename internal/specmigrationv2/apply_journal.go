package specmigrationv2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"time"
)

const (
	migrationJournalSchemaV2 = "haft.spec-migration-v2.journal/v2"
	migrationReceiptSchemaV2 = "haft.spec-migration-v2.receipt/v2"
	migrationLineageSchemaV1 = "haft.spec-migration-v2.lineage-record/v1"
	semanticReviewDigestV2   = "haft.spec-migration-v2.semantic-review/v2"
)

type JournalPhase string

const (
	JournalPrepared        JournalPhase = "prepared"
	JournalTargetInstalled JournalPhase = "target_installed"
	JournalSourceArchived  JournalPhase = "source_archived"
	JournalLineageWritten  JournalPhase = "lineage_written"
	JournalReceiptWritten  JournalPhase = "receipt_written"
	JournalCompleted       JournalPhase = "completed"
)

func validJournalPhase(phase JournalPhase) bool {
	return slices.Contains(
		[]JournalPhase{
			JournalPrepared,
			JournalTargetInstalled,
			JournalSourceArchived,
			JournalLineageWritten,
			JournalReceiptWritten,
			JournalCompleted,
		},
		phase,
	)
}

type migrationJournal struct {
	migrationID           MigrationPacketID
	packetDigest          PacketDigest
	projectRoot           ApplyProjectRoot
	sourceCarrier         SourceCarrierID
	sourceDigest          SourceDigest
	targetCarrier         TargetCarrierID
	targetDigest          TargetDigest
	archiveCarrier        ArchiveCarrierID
	lineageDigest         LineagePolicyDigest
	lineageRecordDigest   SHA256
	profileAdmissionRef   string
	profileAdmissionHash  string
	profileLedgerRevision uint64
	semanticReviewRef     ReviewRef
	semanticAdmissionHash SHA256
	semanticReviewDigest  SHA256
	gitWitness            gitSourceProvenanceWitness
	gitWitnessDigest      SHA256
	receiptDigest         SHA256
	phase                 JournalPhase
	startedAt             time.Time
	updatedAt             time.Time
}

type migrationJournalJSONV2 struct {
	Schema                   string           `json:"schema"`
	MigrationID              string           `json:"migration_id"`
	PacketDigest             string           `json:"packet_digest"`
	ProjectRoot              string           `json:"project_root"`
	SourceCarrier            string           `json:"source_carrier"`
	SourceDigest             string           `json:"source_digest"`
	TargetCarrier            string           `json:"target_carrier"`
	TargetDigest             string           `json:"target_digest"`
	ArchiveCarrier           string           `json:"archive_carrier"`
	LineagePolicyDigest      string           `json:"lineage_policy_digest"`
	LineageRecordDigest      string           `json:"lineage_record_digest"`
	ProfileAdmissionRef      string           `json:"profile_admission_ref"`
	ProfileAdmissionDigest   string           `json:"profile_admission_digest"`
	ProfileLedgerRevision    uint64           `json:"profile_ledger_revision"`
	SemanticReviewRef        string           `json:"semantic_review_ref"`
	SemanticAdmissionDigest  string           `json:"semantic_review_admission_digest"`
	SemanticReviewDigest     string           `json:"semantic_review_digest"`
	GitProvenanceWitness     gitWitnessJSONV1 `json:"git_provenance_witness"`
	GitProvenanceWitnessHash string           `json:"git_provenance_witness_digest"`
	ReceiptDigest            string           `json:"receipt_digest"`
	Phase                    string           `json:"phase"`
	StartedAt                string           `json:"started_at"`
	UpdatedAt                string           `json:"updated_at"`
}

type migrationReceiptJSONV2 struct {
	Schema                   string `json:"schema"`
	MigrationID              string `json:"migration_id"`
	PacketDigest             string `json:"packet_digest"`
	SourceDigest             string `json:"source_digest"`
	TargetDigest             string `json:"target_digest"`
	LineagePolicyDigest      string `json:"lineage_policy_digest"`
	ProfileAdmissionRef      string `json:"profile_admission_ref"`
	ProfileAdmissionDigest   string `json:"profile_admission_digest"`
	ProfileLedgerRevision    uint64 `json:"profile_ledger_revision"`
	SemanticReviewRef        string `json:"semantic_review_ref"`
	SemanticAdmissionDigest  string `json:"semantic_review_admission_digest"`
	SemanticReviewDigest     string `json:"semantic_review_digest"`
	GitProvenanceWitnessHash string `json:"git_provenance_witness_digest"`
	AppliedAt                string `json:"applied_at"`
}

type semanticReviewDigestJSONV2 struct {
	Schema                  string                      `json:"schema"`
	ReviewRef               string                      `json:"review_ref"`
	AdmissionDigest         string                      `json:"admission_digest"`
	SpeechActRef            string                      `json:"speech_act_ref"`
	SpeechActDigest         string                      `json:"speech_act_digest"`
	ProjectRoot             string                      `json:"project_root"`
	PacketDigest            string                      `json:"packet_digest"`
	PacketCarrierDigest     string                      `json:"packet_carrier_digest"`
	SourceCarrier           string                      `json:"source_carrier"`
	SourceDigest            string                      `json:"source_digest"`
	TargetCarrierDigests    []reviewCarrierDigestJSONV1 `json:"target_carrier_digests"`
	FPFRevision             string                      `json:"fpf_revision"`
	SemanticZeroPassCarrier string                      `json:"semantic_zero_pass_carrier"`
	SemanticZeroPassDigest  string                      `json:"semantic_zero_pass_digest"`
	LifecycleIntent         []lifecycleIntentJSONV1     `json:"lifecycle_intent"`
}

type reviewCarrierDigestJSONV1 struct {
	Role    string `json:"role"`
	Carrier string `json:"carrier"`
	Digest  string `json:"digest"`
}

type lifecycleIntentJSONV1 struct {
	SectionRef string `json:"section_ref"`
	Operation  string `json:"operation"`
}

type lineageRecordJSONV1 struct {
	Schema              string               `json:"schema"`
	MigrationID         string               `json:"migration_id"`
	PacketDigest        string               `json:"packet_digest"`
	LineagePolicyDigest string               `json:"lineage_policy_digest"`
	Entries             []lineageEntryJSONV1 `json:"entries"`
}

type lineageEntryJSONV1 struct {
	SubjectKind        string                         `json:"subject_kind"`
	SourceSection      string                         `json:"source_section"`
	Start              uint64                         `json:"start"`
	Length             uint64                         `json:"length"`
	FragmentDigest     string                         `json:"fragment_digest"`
	OutcomeKind        string                         `json:"outcome_kind"`
	TargetClaims       []string                       `json:"target_claims,omitempty"`
	ArchiveCarrier     string                         `json:"archive_carrier,omitempty"`
	SourceDigest       string                         `json:"source_digest,omitempty"`
	Reason             string                         `json:"reason,omitempty"`
	Meaning            string                         `json:"meaning,omitempty"`
	OutsideCarriers    []string                       `json:"outside_carriers,omitempty"`
	ResolvedOutsidePSS []resolvedOutsideCarrierJSONV1 `json:"resolved_outside_carriers,omitempty"`
}

type resolvedOutsideCarrierJSONV1 struct {
	ID      string `json:"id"`
	Carrier string `json:"carrier"`
	Digest  string `json:"digest"`
}

func semanticReviewDigest(review admittedMigrationReview) (SHA256, error) {
	encoded, err := encodeSemanticReviewBinding(review)
	if err != nil {
		return SHA256{}, err
	}
	return DigestBytes(encoded), nil
}

func encodeSemanticReviewBinding(review admittedMigrationReview) ([]byte, error) {
	if err := validateAdmittedMigrationReview(review); err != nil {
		return nil, err
	}
	carriers := review.targetCarrierDigests.Values()
	sort.Slice(carriers, func(left, right int) bool {
		return carriers[left].role < carriers[right].role
	})
	carrierDTOs := make([]reviewCarrierDigestJSONV1, 0, len(carriers))
	for _, carrier := range carriers {
		carrierDTOs = append(carrierDTOs, reviewCarrierDigestJSONV1{
			Role:    string(carrier.role),
			Carrier: carrier.carrier.String(),
			Digest:  carrier.digest.String(),
		})
	}
	intent := review.lifecycleIntent.Values()
	sort.Slice(intent, func(left, right int) bool {
		leftKey := intent[left].sectionRef + "\x00" + string(intent[left].operation)
		rightKey := intent[right].sectionRef + "\x00" + string(intent[right].operation)
		return leftKey < rightKey
	})
	intentDTOs := make([]lifecycleIntentJSONV1, 0, len(intent))
	for _, item := range intent {
		intentDTOs = append(intentDTOs, lifecycleIntentJSONV1{
			SectionRef: item.sectionRef,
			Operation:  string(item.operation),
		})
	}
	dto := semanticReviewDigestJSONV2{
		Schema:                  semanticReviewDigestV2,
		ReviewRef:               review.reviewRef.String(),
		AdmissionDigest:         review.admissionDigest.String(),
		SpeechActRef:            review.speechActRef.String(),
		SpeechActDigest:         review.speechActDigest.String(),
		ProjectRoot:             review.projectRoot.String(),
		PacketDigest:            review.packetDigest.String(),
		PacketCarrierDigest:     review.packetCarrierDigest.String(),
		SourceCarrier:           review.sourceCarrier.String(),
		SourceDigest:            review.sourceDigest.String(),
		TargetCarrierDigests:    carrierDTOs,
		FPFRevision:             review.fpfRevision.String(),
		SemanticZeroPassCarrier: review.semanticZeroPass.Carrier().String(),
		SemanticZeroPassDigest:  review.semanticZeroPass.Digest().String(),
		LifecycleIntent:         intentDTOs,
	}
	return marshalCanonicalJSON(dto)
}

func encodeLineageRecord(
	migrationID MigrationPacketID,
	packetDigest PacketDigest,
	policy LineagePolicy,
	policyDigest LineagePolicyDigest,
) ([]byte, error) {
	entries := policy.Entries()
	sort.Slice(entries, func(left, right int) bool {
		return lineageEntrySortKey(entries[left]) < lineageEntrySortKey(entries[right])
	})
	encodedEntries := make([]lineageEntryJSONV1, 0, len(entries))
	for _, entry := range entries {
		encoded, err := lineageEntryToJSON(entry)
		if err != nil {
			return nil, err
		}
		encodedEntries = append(encodedEntries, encoded)
	}
	dto := lineageRecordJSONV1{
		Schema:              migrationLineageSchemaV1,
		MigrationID:         migrationID.String(),
		PacketDigest:        packetDigest.String(),
		LineagePolicyDigest: policyDigest.String(),
		Entries:             encodedEntries,
	}
	return marshalCanonicalJSON(dto)
}

func lineageEntryToJSON(entry LineageEntry) (lineageEntryJSONV1, error) {
	subject := entry.Subject()
	span := subject.Span()
	source := subject.Source()
	length := span.Length()
	fragmentDigest := span.Digest()
	dto := lineageEntryJSONV1{
		SourceSection:  source.String(),
		Start:          span.Start(),
		Length:         length.Value(),
		FragmentDigest: fragmentDigest.String(),
	}
	switch subject.(type) {
	case wholeSourceSection:
		dto.SubjectKind = "whole_source_section"
	case sourceFragment:
		dto.SubjectKind = "source_fragment"
	default:
		return lineageEntryJSONV1{}, fmt.Errorf("cannot encode unknown lineage subject")
	}
	switch outcome := entry.Outcome().(type) {
	case MeaningMappedToTargetClaims:
		dto.OutcomeKind = "meaning_mapped_to_target_claims"
		claims := outcome.TargetClaims().Values()
		dto.TargetClaims = mapClaimStrings(claims)
	case RetainedAsHistoryOnly:
		archiveCarrier := outcome.ArchiveCarrier()
		sourceDigest := outcome.SourceEditionDigest()
		dto.OutcomeKind = "retained_as_history_only"
		dto.ArchiveCarrier = archiveCarrier.String()
		dto.SourceDigest = sourceDigest.String()
		dto.Reason = outcome.Reason()
	case ContinuesOutsidePSS:
		carriers := outcome.Carriers()
		carrierValues := carriers.Values()
		resolved := outcome.ResolvedCarriers()
		dto.OutcomeKind = "continues_outside_pss"
		dto.Meaning = outcome.Meaning()
		dto.OutsideCarriers = mapOutsideCarrierStrings(carrierValues)
		dto.ResolvedOutsidePSS = mapResolvedOutsideCarriers(resolved)
	default:
		return lineageEntryJSONV1{}, fmt.Errorf("cannot encode unknown lineage outcome")
	}
	return dto, nil
}

func mapClaimStrings(values []TargetAtomicClaimID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	sort.Strings(result)
	return result
}

func mapOutsideCarrierStrings(values []OutsideCarrierID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	sort.Strings(result)
	return result
}

func mapResolvedOutsideCarriers(
	values []ResolvedOutsideCarrierBinding,
) []resolvedOutsideCarrierJSONV1 {
	result := make([]resolvedOutsideCarrierJSONV1, 0, len(values))
	for _, value := range values {
		id := value.ID()
		carrier := value.Carrier()
		digest := value.Digest()
		result = append(result, resolvedOutsideCarrierJSONV1{
			ID:      id.String(),
			Carrier: carrier.String(),
			Digest:  digest.String(),
		})
	}
	sort.Slice(result, func(left, right int) bool {
		leftKey := result[left].ID + "\x00" + result[left].Carrier + "\x00" + result[left].Digest
		rightKey := result[right].ID + "\x00" + result[right].Carrier + "\x00" + result[right].Digest
		return leftKey < rightKey
	})
	return result
}

func encodeJournal(journal migrationJournal) ([]byte, error) {
	if err := validateJournal(journal); err != nil {
		return nil, err
	}
	witnessDTO, err := gitWitnessToJSON(journal.gitWitness)
	if err != nil {
		return nil, err
	}
	dto := migrationJournalJSONV2{
		Schema:                   migrationJournalSchemaV2,
		MigrationID:              journal.migrationID.String(),
		PacketDigest:             journal.packetDigest.String(),
		ProjectRoot:              journal.projectRoot.String(),
		SourceCarrier:            journal.sourceCarrier.String(),
		SourceDigest:             journal.sourceDigest.String(),
		TargetCarrier:            journal.targetCarrier.String(),
		TargetDigest:             journal.targetDigest.String(),
		ArchiveCarrier:           journal.archiveCarrier.String(),
		LineagePolicyDigest:      journal.lineageDigest.String(),
		LineageRecordDigest:      journal.lineageRecordDigest.String(),
		ProfileAdmissionRef:      journal.profileAdmissionRef,
		ProfileAdmissionDigest:   journal.profileAdmissionHash,
		ProfileLedgerRevision:    journal.profileLedgerRevision,
		SemanticReviewRef:        journal.semanticReviewRef.String(),
		SemanticAdmissionDigest:  journal.semanticAdmissionHash.String(),
		SemanticReviewDigest:     journal.semanticReviewDigest.String(),
		GitProvenanceWitness:     witnessDTO,
		GitProvenanceWitnessHash: journal.gitWitnessDigest.String(),
		ReceiptDigest:            journal.receiptDigest.String(),
		Phase:                    string(journal.phase),
		StartedAt:                canonicalEffectTime(journal.startedAt),
		UpdatedAt:                canonicalEffectTime(journal.updatedAt),
	}
	return marshalCanonicalJSON(dto)
}

func decodeJournal(content []byte) (migrationJournal, error) {
	var dto migrationJournalJSONV2
	if err := unmarshalCanonicalJSON(content, &dto); err != nil {
		return migrationJournal{}, err
	}
	journal, err := journalFromJSON(dto)
	if err != nil {
		return migrationJournal{}, err
	}
	reencoded, err := encodeJournal(journal)
	if err != nil {
		return migrationJournal{}, err
	}
	if !bytes.Equal(content, reencoded) {
		return migrationJournal{}, fmt.Errorf("migration journal is not canonical")
	}
	return journal, nil
}

func journalFromJSON(dto migrationJournalJSONV2) (migrationJournal, error) {
	if dto.Schema != migrationJournalSchemaV2 {
		return migrationJournal{}, fmt.Errorf("unsupported migration journal schema %q", dto.Schema)
	}
	migrationID, err := NewMigrationPacketID(dto.MigrationID)
	if err != nil {
		return migrationJournal{}, err
	}
	packetDigest, err := NewPacketDigest(dto.PacketDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	projectRoot, err := NewApplyProjectRoot(dto.ProjectRoot)
	if err != nil {
		return migrationJournal{}, err
	}
	sourceCarrier, err := NewSourceCarrierID(dto.SourceCarrier)
	if err != nil {
		return migrationJournal{}, err
	}
	sourceDigest, err := NewSourceDigest(dto.SourceDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	targetCarrier, err := NewTargetCarrierID(dto.TargetCarrier)
	if err != nil {
		return migrationJournal{}, err
	}
	targetDigest, err := NewTargetDigest(dto.TargetDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	archiveCarrier, err := NewArchiveCarrierID(dto.ArchiveCarrier)
	if err != nil {
		return migrationJournal{}, err
	}
	lineageValue, err := NewSHA256(dto.LineagePolicyDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	lineageRecordDigest, err := NewSHA256(dto.LineageRecordDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	reviewRef, err := newReviewRef(dto.SemanticReviewRef)
	if err != nil {
		return migrationJournal{}, err
	}
	reviewDigest, err := NewSHA256(dto.SemanticReviewDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	admissionDigest, err := NewSHA256(dto.SemanticAdmissionDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	witnessDigest, err := NewSHA256(dto.GitProvenanceWitnessHash)
	if err != nil {
		return migrationJournal{}, err
	}
	witnessJSON, err := json.Marshal(dto.GitProvenanceWitness)
	if err != nil {
		return migrationJournal{}, err
	}
	witnessJSON = append(witnessJSON, '\n')
	witness, err := decodeGitWitness(witnessJSON)
	if err != nil {
		return migrationJournal{}, err
	}
	if !witness.digest.Equal(witnessDigest) {
		return migrationJournal{}, fmt.Errorf("git witness digest does not match its canonical journal record")
	}
	receiptDigest, err := NewSHA256(dto.ReceiptDigest)
	if err != nil {
		return migrationJournal{}, err
	}
	startedAt, err := parseCanonicalEffectTime(dto.StartedAt)
	if err != nil {
		return migrationJournal{}, err
	}
	updatedAt, err := parseCanonicalEffectTime(dto.UpdatedAt)
	if err != nil {
		return migrationJournal{}, err
	}
	journal := migrationJournal{
		migrationID:           migrationID,
		packetDigest:          packetDigest,
		projectRoot:           projectRoot,
		sourceCarrier:         sourceCarrier,
		sourceDigest:          sourceDigest,
		targetCarrier:         targetCarrier,
		targetDigest:          targetDigest,
		archiveCarrier:        archiveCarrier,
		lineageDigest:         LineagePolicyDigest{value: lineageValue},
		lineageRecordDigest:   lineageRecordDigest,
		profileAdmissionRef:   dto.ProfileAdmissionRef,
		profileAdmissionHash:  dto.ProfileAdmissionDigest,
		profileLedgerRevision: dto.ProfileLedgerRevision,
		semanticReviewRef:     reviewRef,
		semanticAdmissionHash: admissionDigest,
		semanticReviewDigest:  reviewDigest,
		gitWitness:            witness,
		gitWitnessDigest:      witnessDigest,
		receiptDigest:         receiptDigest,
		phase:                 JournalPhase(dto.Phase),
		startedAt:             startedAt,
		updatedAt:             updatedAt,
	}
	return journal, validateJournal(journal)
}

func validateJournal(journal migrationJournal) error {
	witnessValidationErr := validateGitWitness(journal.gitWitness)
	witnessBytes, witnessErr := encodeGitWitness(journal.gitWitness)
	witnessDigest := DigestBytes(witnessBytes)
	profileBindingErr := validateOpaqueProfileBindingShape(
		journal.profileAdmissionRef,
		journal.profileAdmissionHash,
		journal.profileLedgerRevision,
	)
	checks := []struct {
		valid  bool
		reason string
	}{
		{valid: journal.migrationID.valid(), reason: "journal migration ID is invalid"},
		{valid: journal.packetDigest.valid(), reason: "journal packet digest is invalid"},
		{valid: journal.projectRoot.valid(), reason: "journal project root is invalid"},
		{valid: journal.sourceCarrier.valid(), reason: "journal source carrier is invalid"},
		{valid: journal.sourceDigest.valid(), reason: "journal source digest is invalid"},
		{valid: journal.targetCarrier.valid(), reason: "journal target carrier is invalid"},
		{valid: journal.targetDigest.valid(), reason: "journal target digest is invalid"},
		{valid: journal.archiveCarrier.valid(), reason: "journal archive carrier is invalid"},
		{valid: journal.lineageDigest.valid(), reason: "journal lineage-policy digest is invalid"},
		{valid: journal.lineageRecordDigest.valid(), reason: "journal lineage-record digest is invalid"},
		{valid: profileBindingErr == nil, reason: "journal profile-admission binding is invalid"},
		{valid: journal.semanticReviewRef.String() != "", reason: "journal semantic-review ref is missing"},
		{valid: journal.semanticAdmissionHash.valid(), reason: "journal semantic-review admission digest is invalid"},
		{valid: journal.semanticReviewDigest.valid(), reason: "journal semantic-review digest is invalid"},
		{valid: witnessValidationErr == nil && witnessErr == nil, reason: "journal Git witness record is invalid"},
		{valid: witnessValidationErr == nil && witnessErr == nil && witnessDigest.Equal(journal.gitWitnessDigest), reason: "journal Git witness digest does not bind its record"},
		{valid: journal.gitWitnessDigest.valid(), reason: "journal Git witness digest is invalid"},
		{valid: journal.receiptDigest.valid(), reason: "journal receipt digest is invalid"},
		{valid: validJournalPhase(journal.phase), reason: "journal phase is invalid"},
		{valid: !journal.startedAt.IsZero(), reason: "journal start time is missing"},
		{valid: !journal.updatedAt.Before(journal.startedAt), reason: "journal update time precedes start time"},
	}
	for _, check := range checks {
		if !check.valid {
			return fmt.Errorf("%s", check.reason)
		}
	}
	return nil
}

func encodeReceipt(receipt MigrationEffectReceipt) ([]byte, error) {
	if err := validateReceipt(receipt); err != nil {
		return nil, err
	}
	dto := migrationReceiptJSONV2{
		Schema:                   migrationReceiptSchemaV2,
		MigrationID:              receipt.migrationID.String(),
		PacketDigest:             receipt.packetDigest.String(),
		SourceDigest:             receipt.sourceDigest.String(),
		TargetDigest:             receipt.targetDigest.String(),
		LineagePolicyDigest:      receipt.lineageDigest.String(),
		ProfileAdmissionRef:      receipt.profileAdmissionRef,
		ProfileAdmissionDigest:   receipt.profileAdmissionHash,
		ProfileLedgerRevision:    receipt.profileLedgerRevision,
		SemanticReviewRef:        receipt.semanticReviewRef.String(),
		SemanticAdmissionDigest:  receipt.semanticReviewAdmissionDigest.String(),
		SemanticReviewDigest:     receipt.semanticReviewDigest.String(),
		GitProvenanceWitnessHash: receipt.gitWitnessDigest.String(),
		AppliedAt:                canonicalEffectTime(receipt.appliedAt),
	}
	return marshalCanonicalJSON(dto)
}

func decodeReceipt(content []byte) (MigrationEffectReceipt, error) {
	var dto migrationReceiptJSONV2
	if err := unmarshalCanonicalJSON(content, &dto); err != nil {
		return MigrationEffectReceipt{}, err
	}
	if dto.Schema != migrationReceiptSchemaV2 {
		return MigrationEffectReceipt{}, fmt.Errorf("unsupported migration receipt schema %q", dto.Schema)
	}
	migrationID, err := NewMigrationPacketID(dto.MigrationID)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	packetDigest, err := NewPacketDigest(dto.PacketDigest)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	sourceDigest, err := NewSourceDigest(dto.SourceDigest)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	targetDigest, err := NewTargetDigest(dto.TargetDigest)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	lineageValue, err := NewSHA256(dto.LineagePolicyDigest)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	reviewRef, err := newReviewRef(dto.SemanticReviewRef)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	reviewDigest, err := NewSHA256(dto.SemanticReviewDigest)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	admissionDigest, err := NewSHA256(dto.SemanticAdmissionDigest)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	witnessDigest, err := NewSHA256(dto.GitProvenanceWitnessHash)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	appliedAt, err := parseCanonicalEffectTime(dto.AppliedAt)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	receipt := MigrationEffectReceipt{
		migrationID:                   migrationID,
		packetDigest:                  packetDigest,
		sourceDigest:                  sourceDigest,
		targetDigest:                  targetDigest,
		lineageDigest:                 LineagePolicyDigest{value: lineageValue},
		profileAdmissionRef:           dto.ProfileAdmissionRef,
		profileAdmissionHash:          dto.ProfileAdmissionDigest,
		profileLedgerRevision:         dto.ProfileLedgerRevision,
		semanticReviewRef:             reviewRef,
		semanticReviewAdmissionDigest: admissionDigest,
		semanticReviewDigest:          reviewDigest,
		gitWitnessDigest:              witnessDigest,
		appliedAt:                     appliedAt,
	}
	if err := validateReceipt(receipt); err != nil {
		return MigrationEffectReceipt{}, err
	}
	reencoded, err := encodeReceipt(receipt)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	if !bytes.Equal(content, reencoded) {
		return MigrationEffectReceipt{}, fmt.Errorf("migration receipt is not canonical")
	}
	return receipt, nil
}

func validateReceipt(receipt MigrationEffectReceipt) error {
	if !receipt.migrationID.valid() || !receipt.packetDigest.valid() {
		return fmt.Errorf("migration receipt identity is invalid")
	}
	if !receipt.sourceDigest.valid() || !receipt.targetDigest.valid() || !receipt.lineageDigest.valid() {
		return fmt.Errorf("migration receipt carrier or lineage digest is invalid")
	}
	profileBindingErr := validateOpaqueProfileBindingShape(
		receipt.profileAdmissionRef,
		receipt.profileAdmissionHash,
		receipt.profileLedgerRevision,
	)
	if profileBindingErr != nil {
		return fmt.Errorf("migration receipt P0PA binding is invalid")
	}
	reviewRef := receipt.semanticReviewRef.String()
	if reviewRef == "" ||
		!receipt.semanticReviewAdmissionDigest.valid() ||
		!receipt.semanticReviewDigest.valid() {
		return fmt.Errorf("migration receipt semantic-review binding is invalid")
	}
	if !receipt.gitWitnessDigest.valid() || receipt.appliedAt.IsZero() {
		return fmt.Errorf("migration receipt Git witness or apply time is invalid")
	}
	return nil
}

// validateOpaqueProfileBindingShape protects canonical journal decoding only.
// It does not prove SQLite origin, COMMIT, applicability, or authority. The
// public effect constructor remains sealed until P0PA supplies that proof.
func validateOpaqueProfileBindingShape(ref string, digest string, revision uint64) error {
	if _, err := requireNarrative("opaque profile-admission ref", ref); err != nil {
		return err
	}
	if _, err := NewSHA256(digest); err != nil {
		return err
	}
	if revision == 0 {
		return fmt.Errorf("profile-admission ledger revision is required")
	}
	return nil
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func unmarshalCanonicalJSON(content []byte, target any) error {
	reader := bytes.NewReader(content)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("migration JSON has trailing content")
	}
	return nil
}

func canonicalEffectTime(value time.Time) string {
	utc := value.UTC()
	return utc.Format(time.RFC3339Nano)
}

func parseCanonicalEffectTime(raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, err
	}
	if canonicalEffectTime(value) != raw {
		return time.Time{}, fmt.Errorf("migration time is not canonical UTC RFC3339Nano")
	}
	return value, nil
}
