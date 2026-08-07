package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/specmigrationv2"
	"github.com/spf13/cobra"
)

const (
	specMigrationV2RecordKind               = "haft_spec_migration_v2_dry_run"
	specMigrationPendingReviewCode          = "pending_review"
	specMigrationProfileUnderdeterminedCode = "profile_applicability_underdetermined"
	specMigrationProfileNotApplicableCode   = "profile_not_applicable"
	specMigrationInvalidCode                = "migration_invalid"
	specMigrationGitProvenanceInvalidCode   = "git_source_provenance_invalid"
	specMigrationFPFSourceInvalidCode       = "fpf_source_revision_invalid"
	specMigrationV2SourceCarrier            = ".haft/specs/enabling-system.md"
	specMigrationV2TargetCarrier            = ".haft/specs/software-system.md"
	specMigrationV2ArchiveCarrier           = ".haft/migration-archive/enabling-system.md"
)

type specMigrationV2Diagnostic struct {
	Code    string `json:"code"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}

type specMigrationV2Result struct {
	RecordKind            string                      `json:"record_kind"`
	SchemaVersion         uint32                      `json:"schema_version"`
	State                 string                      `json:"state"`
	PacketID              string                      `json:"packet_id"`
	PacketDigest          string                      `json:"packet_digest"`
	PacketCarrier         string                      `json:"packet_carrier"`
	PacketCarrierDigest   string                      `json:"packet_carrier_digest"`
	SourceCarrier         string                      `json:"source_carrier"`
	SourceDigest          string                      `json:"source_digest"`
	ReviewSoftwareCarrier string                      `json:"review_software_carrier"`
	FinalTargetCarrier    string                      `json:"final_target_carrier"`
	TargetDigest          string                      `json:"target_digest"`
	PartitionAuditStatus  string                      `json:"partition_audit_status"`
	PartitionAuditDigest  string                      `json:"partition_audit_digest"`
	PartitionAuditCounts  specMigrationV2AuditCounts  `json:"partition_audit_counts"`
	ProfileApplicability  string                      `json:"profile_applicability"`
	ProfileMissingBasis   []string                    `json:"profile_missing_basis,omitempty"`
	ReviewMissingBasis    []string                    `json:"review_missing_basis,omitempty"`
	Diagnostics           []specMigrationV2Diagnostic `json:"diagnostics,omitempty"`
	ApplyRequested        bool                        `json:"apply_requested"`
	RecoveryRequested     bool                        `json:"recovery_requested,omitempty"`
	RecoveryPhase         string                      `json:"recovery_phase,omitempty"`
	RecoveryReason        string                      `json:"recovery_reason,omitempty"`
	Applied               bool                        `json:"applied"`
	ReceiptCarrier        string                      `json:"receipt_carrier,omitempty"`
	ReceiptCarrierDigest  string                      `json:"receipt_carrier_digest,omitempty"`
	NextAction            string                      `json:"next_action"`
}

type specMigrationV2AuditCounts struct {
	SourceSections       int `json:"source_sections"`
	TopLevelDispositions int `json:"top_level_dispositions"`
	SplitSections        int `json:"split_sections"`
	SplitLeaves          int `json:"split_leaves"`
	WholeSectionOutcomes int `json:"whole_section_outcomes"`
	LineageEntries       int `json:"lineage_entries"`
}

type specMigrationV2Observation struct {
	carrier               specmigrationv2.FinalCandidatePacketCarrier
	packet                specmigrationv2.Packet
	projectRoot           specmigrationv2.ProjectRootRef
	gitWitness            specmigrationv2.GitSourceProvenanceWitness
	profileApplicability  profileadmissionsqlite.SoftwareSystemSpecMigrationApplicability
	structural            specMigrationV2StructuralObservation
	partitionAudit        specmigrationv2.PacketPartitionAudit
	reviewSoftwareCarrier string
}

type specMigrationV2ObservationBasis struct {
	PacketCarrier         []byte                               `json:"packet_carrier"`
	ProjectRoot           string                               `json:"project_root"`
	GitWitness            []byte                               `json:"git_witness"`
	Profile               specMigrationV2ProfileBasis          `json:"profile"`
	Source                specMigrationV2CarrierBasis          `json:"source"`
	Target                specMigrationV2CarrierBasis          `json:"target"`
	TargetClaims          specMigrationV2TargetClaimBasis      `json:"target_claims"`
	Outside               []specMigrationV2OutsideCarrierBasis `json:"outside"`
	PartitionAudit        []byte                               `json:"partition_audit"`
	ReviewSoftwareCarrier string                               `json:"review_software_carrier"`
}

type specMigrationV2CarrierBasis struct {
	Carrier string `json:"carrier"`
	Bytes   []byte `json:"bytes"`
}

type specMigrationV2TargetClaimBasis struct {
	Carrier string   `json:"carrier"`
	Digest  string   `json:"digest"`
	Claims  []string `json:"claims"`
}

type specMigrationV2OutsideCarrierBasis struct {
	ID      string `json:"id"`
	Carrier string `json:"carrier"`
	Bytes   []byte `json:"bytes"`
}

type specMigrationV2ProfileBasis struct {
	Kind                  string   `json:"kind"`
	Valid                 bool     `json:"valid"`
	ProjectRoot           string   `json:"project_root"`
	SoftwareScopeIDs      []string `json:"software_scope_ids,omitempty"`
	AdmissionRecordRef    string   `json:"admission_record_ref,omitempty"`
	AdmissionRecordDigest string   `json:"admission_record_digest,omitempty"`
	ProfilePayloadDigest  string   `json:"profile_payload_digest,omitempty"`
	LedgerRevision        uint64   `json:"ledger_revision,omitempty"`
	UnderdeterminedBasis  string   `json:"underdetermined_basis,omitempty"`
}

type specMigrationV2InvalidPreflight struct {
	result specMigrationV2Result
}

func (failure *specMigrationV2InvalidPreflight) Error() string {
	diagnostic := failure.result.Diagnostics[0]
	return diagnostic.Code + ": " + diagnostic.Detail
}

type specMigrationV2StructuralObservation struct {
	request specmigrationv2.StructuralRequest
	source  specmigrationv2.SourceSnapshot
	target  specmigrationv2.TargetSnapshot
	claims  specmigrationv2.TargetClaimCatalog
	outside specmigrationv2.OutsideCarrierSnapshots
}

type specMigrationV2Ledger struct {
	handle  *projectledger.Handle
	profile profileadmissionsqlite.Service
	review  specmigrationv2.ReviewAdmissionService
}

func openSpecMigrationV2Ledger(
	ctx context.Context,
	root string,
	access projectledger.Access,
) (*specMigrationV2Ledger, error) {
	handle, err := projectledger.OpenExisting(ctx, root, access)
	if err != nil {
		return nil, fmt.Errorf("open checked project ledger: %w", err)
	}
	profileService, err := profileadmissionsqlite.NewService(handle.Database())
	if err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("construct profile ledger service: %w", err)
	}
	reviewService, err := specmigrationv2.NewReviewAdmissionService(handle.Database())
	if err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("construct semantic-review ledger service: %w", err)
	}
	return &specMigrationV2Ledger{
		handle:  handle,
		profile: profileService,
		review:  reviewService,
	}, nil
}

func (ledger *specMigrationV2Ledger) Close() error {
	if ledger == nil || ledger.handle == nil {
		return nil
	}
	return ledger.handle.Close()
}

func runSpecMigrateV2OperationWithReviewCapture(
	cmd *cobra.Command,
	root string,
	packetPath string,
	operation specMigrationV2Operation,
	capture specMigrationV2ReviewCapture,
) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	apply := operation == specMigrationV2ApplyOperation
	admitReview := operation == specMigrationV2AdmitReviewOperation
	access := projectledger.ReadOnly
	if operation != specMigrationV2InspectOperation {
		access = projectledger.ReadWrite
	}
	ledger, err := openSpecMigrationV2Ledger(ctx, root, access)
	if err != nil {
		return err
	}
	defer ledger.Close()
	if operation == specMigrationV2RecoverOperation {
		result, recoverErr := recoverSpecMigrationV2(
			ctx,
			ledger,
			root,
			packetPath,
		)
		if result.RecordKind == "" {
			return recoverErr
		}
		if writeErr := writeSpecMigrationV2Result(cmd.OutOrStdout(), result, specMigrateJSON); writeErr != nil {
			return writeErr
		}
		return recoverErr
	}
	observation, err := observeSpecMigrationV2WithProfile(
		ctx,
		root,
		packetPath,
		ledger.profile,
	)
	if err != nil {
		var invalidPreflight *specMigrationV2InvalidPreflight
		if errors.As(err, &invalidPreflight) {
			result := invalidPreflight.result
			result.ApplyRequested = apply
			asJSON := specMigrateJSON && !admitReview
			if writeErr := writeSpecMigrationV2Result(cmd.OutOrStdout(), result, asJSON); writeErr != nil {
				return writeErr
			}
			if admitReview {
				return specMigrationV2ReviewAdmissionPrecondition(result)
			}
			return specMigrationV2ApplyPrecondition(result, apply)
		}
		return err
	}
	review, err := resolveSpecMigrationV2Review(
		ctx,
		ledger.review,
		observation,
	)
	if err != nil {
		return err
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
		return fmt.Errorf("construct canonical migration-v2 dry-run: %w", err)
	}
	result := presentSpecMigrationV2Result(
		packetPath,
		observation,
		specmigrationv2.DryRun(dryRunRequest),
		apply,
	)
	if admitReview {
		return runSpecMigrationV2ReviewAdmissionWithCapture(
			ctx,
			cmd.OutOrStdout(),
			ledger,
			root,
			packetPath,
			observation,
			result,
			capture,
		)
	}
	if apply && result.State == "applicable" {
		result, err = applySpecMigrationV2(
			ctx,
			ledger,
			root,
			observation,
			review,
			result,
		)
		if err != nil {
			return err
		}
	}
	if err := writeSpecMigrationV2Result(cmd.OutOrStdout(), result, specMigrateJSON); err != nil {
		return err
	}
	return specMigrationV2ApplyPrecondition(result, apply)
}

type specMigrationV2ReviewAdmissionResult struct {
	reviewRef             string
	reviewAdmissionDigest string
	speechActRef          string
	speechActDigest       string
	projectRoot           string
	packetDigest          string
	packetCarrierDigest   string
	partitionAuditStatus  string
	partitionAuditDigest  string
}

type specMigrationV2ReviewCapture func(
	context.Context,
	specmigrationv2.PreparedMigrationReviewAdmission,
) (authority.VerifiedSpeechActSource, error)

func runSpecMigrationV2ReviewAdmissionWithCapture(
	ctx context.Context,
	writer io.Writer,
	ledger *specMigrationV2Ledger,
	root string,
	packetPath string,
	observation specMigrationV2Observation,
	preflight specMigrationV2Result,
	capture specMigrationV2ReviewCapture,
) error {
	admissibleState := preflight.State == "pending_review" || preflight.State == "applicable"
	if !admissibleState {
		if err := writeSpecMigrationV2Result(writer, preflight, false); err != nil {
			return err
		}
		return specMigrationV2ReviewAdmissionPrecondition(preflight)
	}
	if capture == nil {
		return fmt.Errorf("semantic-review terminal capture is unavailable")
	}
	prepared, err := specmigrationv2.PrepareMigrationReviewAdmission(
		observation.carrier,
		observation.partitionAudit,
	)
	if err != nil {
		return err
	}
	review, err := ledger.review.Resume(ctx, prepared)
	if err == nil {
		result := presentSpecMigrationV2ReviewAdmissionResult(
			review,
			observation.partitionAudit,
		)
		return writeSpecMigrationV2ReviewAdmissionResult(writer, result)
	}
	var unavailable *specmigrationv2.NoDurableMigrationReviewSpeechActSourceError
	if !errors.As(err, &unavailable) {
		return fmt.Errorf("resume exact semantic review before terminal capture: %w", err)
	}
	speechAct, err := capture(ctx, prepared)
	if err != nil {
		return err
	}
	if err := ledger.review.RecordSource(ctx, prepared, speechAct); err != nil {
		return fmt.Errorf("record exact semantic-review SpeechAct source: %w", err)
	}
	if err := ledger.handle.Revalidate(ctx); err != nil {
		return fmt.Errorf("revalidate checked project ledger after terminal capture: %w", err)
	}
	freshObservation, err := observeSpecMigrationV2WithProfile(
		ctx,
		root,
		packetPath,
		ledger.profile,
	)
	if err != nil {
		return fmt.Errorf("repeat complete migration observation after terminal capture: %w", err)
	}
	if err := compareSpecMigrationV2ObservationBasis(observation, freshObservation); err != nil {
		return err
	}
	freshPrepared, err := specmigrationv2.PrepareMigrationReviewAdmission(
		freshObservation.carrier,
		freshObservation.partitionAudit,
	)
	if err != nil {
		return fmt.Errorf("prepare exact migration review after terminal capture: %w", err)
	}
	if err := comparePreparedMigrationReviewAdmission(prepared, freshPrepared); err != nil {
		return err
	}
	review, err = ledger.review.Resume(ctx, freshPrepared)
	if err != nil {
		return fmt.Errorf("resume exact semantic-review effect after durable source closure: %w", err)
	}
	result := presentSpecMigrationV2ReviewAdmissionResult(
		review,
		freshObservation.partitionAudit,
	)
	return writeSpecMigrationV2ReviewAdmissionResult(writer, result)
}

func presentSpecMigrationV2ReviewAdmissionResult(
	review specmigrationv2.AdmittedMigrationReview,
	audit specmigrationv2.PacketPartitionAudit,
) specMigrationV2ReviewAdmissionResult {
	return specMigrationV2ReviewAdmissionResult{
		reviewRef:             review.ReviewRef().String(),
		reviewAdmissionDigest: review.ReviewAdmissionDigest().String(),
		speechActRef:          review.SpeechActRef().String(),
		speechActDigest:       review.SpeechActDigest().String(),
		projectRoot:           review.ProjectRoot().String(),
		packetDigest:          review.PacketDigest().String(),
		packetCarrierDigest:   review.PacketCarrierDigest().String(),
		partitionAuditStatus:  string(audit.Status()),
		partitionAuditDigest:  audit.Digest().String(),
	}
}

func resolveSpecMigrationV2Review(
	ctx context.Context,
	service specmigrationv2.ReviewAdmissionService,
	observation specMigrationV2Observation,
) (specmigrationv2.MigrationReviewResolution, error) {
	if observation.partitionAudit.Status() != specmigrationv2.PacketPartitionAuditVerified {
		return pendingSpecMigrationReview()
	}
	review, err := service.ResolveCurrentForAudit(
		ctx,
		observation.carrier,
		observation.partitionAudit,
	)
	if err == nil {
		return review, nil
	}
	if !errors.Is(err, specmigrationv2.ErrNoCurrentSemanticReviewAdmission) {
		return nil, fmt.Errorf("resolve current exact semantic review: %w", err)
	}
	return pendingSpecMigrationReview()
}

func comparePreparedMigrationReviewAdmission(
	before specmigrationv2.PreparedMigrationReviewAdmission,
	after specmigrationv2.PreparedMigrationReviewAdmission,
) error {
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: before.ReviewContentRef() == after.ReviewContentRef(), name: "review-content ref"},
		{matches: before.ReviewContentDigest().Equal(after.ReviewContentDigest()), name: "review-content digest"},
		{matches: before.AdmissionRef().String() == after.AdmissionRef().String(), name: "admission ref"},
		{matches: before.SpeechActRef() == after.SpeechActRef(), name: "SpeechAct ref"},
		{matches: before.ReviewDigest() == after.ReviewDigest(), name: "review digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("migration observation changed after terminal capture: prepared %s differs", check.name)
		}
	}
	return nil
}

func specMigrationV2ReviewAdmissionPrecondition(
	result specMigrationV2Result,
) error {
	preconditions := map[string]specMigrationPreconditionError{
		"underdetermined": {
			code:   specMigrationProfileUnderdeterminedCode,
			detail: "a current integrity-valid canonical project profile is required before semantic-review admission",
		},
		"not_applicable": {
			code:   specMigrationProfileNotApplicableCode,
			detail: "semantic-review admission is not applicable because the current profile has no software realization scope",
		},
		"invalid": {
			code:   specMigrationInvalidCode,
			detail: "the exact migration packet or observed carrier set is invalid",
		},
	}
	precondition, found := preconditions[result.State]
	if found {
		return precondition
	}
	return specMigrationPreconditionError{
		code:   specMigrationInvalidCode,
		detail: "semantic-review admission received an unexpected preflight state",
	}
}

func writeSpecMigrationV2ReviewAdmissionResult(
	writer io.Writer,
	_ specMigrationV2ReviewAdmissionResult,
) error {
	_, err := fmt.Fprintf(
		writer,
		"Semantic review recorded.\nNo specification files were changed.\nRun `haft spec migrate` again to perform this exact reviewed migration.\n",
	)
	return err
}

func observeSpecMigrationV2WithProfile(
	ctx context.Context,
	root string,
	packetPath string,
	profileService profileadmissionsqlite.Service,
) (specMigrationV2Observation, error) {
	carrierBytes, resolvedPacketPath, err := readPacketCarrier(packetPath)
	if err != nil {
		return specMigrationV2Observation{}, err
	}
	carrier, err := specmigrationv2.DecodePacketCarrier(carrierBytes)
	if err != nil {
		return specMigrationV2Observation{}, fmt.Errorf(
			"decode strict migration-v2 final candidate %s: %w",
			resolvedPacketPath,
			err,
		)
	}
	packet := carrier.Packet()
	reviewSoftwareCarrier, candidateBytes, err := observeReviewBasis(root, carrier.ReviewBasis())
	if err != nil {
		return specMigrationV2Observation{}, err
	}
	if err := validateCLISoftwareMigrationPacket(packet, reviewSoftwareCarrier); err != nil {
		return specMigrationV2Observation{}, err
	}
	if err := verifySpecMigrationV2FPFSource(ctx, root, carrier.ReviewBasis()); err != nil {
		failure := newSpecMigrationV2InvalidPreflight(
			resolvedPacketPath,
			carrier,
			reviewSoftwareCarrier,
			specMigrationFPFSourceInvalidCode,
			"data/FPF",
			err.Error(),
		)
		return specMigrationV2Observation{}, failure
	}
	applyRoot, err := specmigrationv2.NewApplyProjectRoot(root)
	if err != nil {
		failure := newSpecMigrationV2InvalidPreflight(
			resolvedPacketPath,
			carrier,
			reviewSoftwareCarrier,
			specMigrationGitProvenanceInvalidCode,
			packet.Source().Carrier().String(),
			err.Error(),
		)
		return specMigrationV2Observation{}, failure
	}
	gitWitness, err := specmigrationv2.VerifyGitSourceProvenance(
		ctx,
		applyRoot,
		packet.Source().Provenance(),
	)
	if err != nil {
		failure := newSpecMigrationV2InvalidPreflight(
			resolvedPacketPath,
			carrier,
			reviewSoftwareCarrier,
			specMigrationGitProvenanceInvalidCode,
			packet.Source().Carrier().String(),
			err.Error(),
		)
		return specMigrationV2Observation{}, failure
	}
	structural, projectRoot, err := buildSpecMigrationV2StructuralRequest(
		root,
		packet,
		candidateBytes,
	)
	if err != nil {
		return specMigrationV2Observation{}, err
	}
	partitionAudit, err := specmigrationv2.AuditPacketCandidate(carrier, structural.request)
	if err != nil {
		return specMigrationV2Observation{}, fmt.Errorf("audit migration-v2 final candidate: %w", err)
	}
	profileRoot, err := projectprofile.NewProjectRootV1(root)
	if err != nil {
		return specMigrationV2Observation{}, fmt.Errorf("construct profile project root: %w", err)
	}
	applicability := profileService.ResolveSoftwareSystemSpecMigration(ctx, profileRoot)
	return specMigrationV2Observation{
		carrier:               carrier,
		packet:                packet,
		projectRoot:           projectRoot,
		gitWitness:            gitWitness,
		profileApplicability:  applicability,
		structural:            structural,
		partitionAudit:        partitionAudit,
		reviewSoftwareCarrier: reviewSoftwareCarrier,
	}, nil
}

func compareSpecMigrationV2ObservationBasis(
	before specMigrationV2Observation,
	after specMigrationV2Observation,
) error {
	beforeCanonical, err := encodeSpecMigrationV2ObservationBasis(before)
	if err != nil {
		return fmt.Errorf("encode pre-capture migration observation: %w", err)
	}
	afterCanonical, err := encodeSpecMigrationV2ObservationBasis(after)
	if err != nil {
		return fmt.Errorf("encode post-capture migration observation: %w", err)
	}
	return compareSpecMigrationV2CanonicalObservationBasis(beforeCanonical, afterCanonical)
}

func compareSpecMigrationV2CanonicalObservationBasis(
	beforeCanonical []byte,
	afterCanonical []byte,
) error {
	if string(beforeCanonical) == string(afterCanonical) {
		return nil
	}
	return fmt.Errorf(
		"migration observation changed after terminal capture: before=%s after=%s; the SpeechAct source is durable but no review effect was instituted",
		specMigrationV2ObservationDigest(beforeCanonical),
		specMigrationV2ObservationDigest(afterCanonical),
	)
}

func encodeSpecMigrationV2ObservationBasis(
	observation specMigrationV2Observation,
) ([]byte, error) {
	claimIDs := make([]string, 0, len(observation.structural.claims.Claims()))
	for _, claim := range observation.structural.claims.Claims() {
		claimIDs = append(claimIDs, claim.String())
	}
	outside := make(
		[]specMigrationV2OutsideCarrierBasis,
		0,
		len(observation.structural.outside.Values()),
	)
	for _, snapshot := range observation.structural.outside.Values() {
		outside = append(outside, specMigrationV2OutsideCarrierBasis{
			ID:      snapshot.ID().String(),
			Carrier: snapshot.Carrier().String(),
			Bytes:   snapshot.Bytes(),
		})
	}
	gitWitness := []byte(nil)
	if observation.gitWitness != nil {
		gitWitness = observation.gitWitness.CanonicalBytes()
	}
	basis := specMigrationV2ObservationBasis{
		PacketCarrier: observation.carrier.CanonicalBytes(),
		ProjectRoot:   observation.projectRoot.String(),
		GitWitness:    gitWitness,
		Profile:       specMigrationV2ProfileObservationBasis(observation.profileApplicability),
		Source: specMigrationV2CarrierBasis{
			Carrier: observation.structural.source.Carrier().String(),
			Bytes:   observation.structural.source.Bytes(),
		},
		Target: specMigrationV2CarrierBasis{
			Carrier: observation.structural.target.Carrier().String(),
			Bytes:   observation.structural.target.Bytes(),
		},
		TargetClaims: specMigrationV2TargetClaimBasis{
			Carrier: observation.structural.claims.Carrier().String(),
			Digest:  observation.structural.claims.Digest().String(),
			Claims:  claimIDs,
		},
		Outside:               outside,
		PartitionAudit:        observation.partitionAudit.CanonicalBytes(),
		ReviewSoftwareCarrier: observation.reviewSoftwareCarrier,
	}
	return json.Marshal(basis)
}

func specMigrationV2ProfileObservationBasis(
	applicability profileadmissionsqlite.SoftwareSystemSpecMigrationApplicability,
) specMigrationV2ProfileBasis {
	basis := specMigrationV2ProfileBasis{
		Kind:  string(applicability.Kind()),
		Valid: applicability.Valid(),
	}
	if required, ok := applicability.Required(); ok {
		basis.ProjectRoot = required.ProjectRoot().String()
		basis.AdmissionRecordRef = required.AdmissionRecordRef().String()
		basis.AdmissionRecordDigest = required.AdmissionRecordDigest().String()
		basis.ProfilePayloadDigest = required.ProfilePayloadDigest().String()
		basis.LedgerRevision = required.LedgerRevision().Value()
		for _, scope := range required.SoftwareScopeIDs() {
			basis.SoftwareScopeIDs = append(basis.SoftwareScopeIDs, scope.String())
		}
		return basis
	}
	if notApplicable, ok := applicability.NotApplicable(); ok {
		basis.ProjectRoot = notApplicable.ProjectRoot().String()
		basis.AdmissionRecordRef = notApplicable.AdmissionRecordRef().String()
		basis.AdmissionRecordDigest = notApplicable.AdmissionRecordDigest().String()
		basis.ProfilePayloadDigest = notApplicable.ProfilePayloadDigest().String()
		basis.LedgerRevision = notApplicable.LedgerRevision().Value()
		return basis
	}
	if underdetermined, ok := applicability.Underdetermined(); ok {
		basis.ProjectRoot = underdetermined.ProjectRoot().String()
		basis.UnderdeterminedBasis = string(underdetermined.MissingBasis())
	}
	return basis
}

func specMigrationV2ObservationDigest(canonical []byte) string {
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func newSpecMigrationV2InvalidPreflight(
	packetPath string,
	carrier specmigrationv2.FinalCandidatePacketCarrier,
	reviewSoftwareCarrier string,
	code string,
	subject string,
	detail string,
) *specMigrationV2InvalidPreflight {
	packet := carrier.Packet()
	result := specMigrationV2Result{
		RecordKind:            specMigrationV2RecordKind,
		SchemaVersion:         packet.SchemaVersion(),
		State:                 "invalid",
		PacketID:              packet.ID().String(),
		PacketDigest:          carrier.PacketDigest().String(),
		PacketCarrier:         filepath.Clean(packetPath),
		PacketCarrierDigest:   carrier.CarrierDigest().String(),
		SourceCarrier:         packet.Source().Carrier().String(),
		SourceDigest:          packet.Source().Digest().String(),
		ReviewSoftwareCarrier: reviewSoftwareCarrier,
		FinalTargetCarrier:    packet.Target().Carrier().String(),
		TargetDigest:          packet.Target().Digest().String(),
		PartitionAuditStatus:  "not_run",
		ProfileApplicability:  "not_observed",
		Diagnostics: []specMigrationV2Diagnostic{{
			Code:    code,
			Subject: subject,
			Detail:  detail,
		}},
		Applied:    false,
		NextAction: "repair the exact source witness and rerun the same packet; partition audit and apply were not attempted",
	}
	return &specMigrationV2InvalidPreflight{result: result}
}

func verifySpecMigrationV2FPFSource(
	ctx context.Context,
	root string,
	basis specmigrationv2.FinalCandidateReviewBasis,
) error {
	fpfRoot := filepath.Join(root, "data", "FPF")
	canonicalFPFRoot, err := filepath.EvalSymlinks(fpfRoot)
	if err != nil {
		return fmt.Errorf("resolve current FPF source root: %w", err)
	}
	repositoryRoot, err := runSpecMigrationV2Git(ctx, fpfRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	observedRootText := string(repositoryRoot)
	observedRoot := strings.TrimSpace(observedRootText)
	canonicalObservedRoot, err := filepath.EvalSymlinks(observedRoot)
	if err != nil {
		return fmt.Errorf("resolve observed FPF Git root: %w", err)
	}
	if canonicalObservedRoot != canonicalFPFRoot {
		return fmt.Errorf("data/FPF is not the root of the reviewed FPF repository")
	}
	headBytes, err := runSpecMigrationV2Git(ctx, fpfRoot, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return err
	}
	headText := string(headBytes)
	head := strings.TrimSpace(headText)
	expected := basis.FPFRevision().String()
	if head != expected {
		return fmt.Errorf("current FPF HEAD %s does not match reviewed revision %s", head, expected)
	}
	status, err := runSpecMigrationV2Git(
		ctx,
		fpfRoot,
		"status",
		"--porcelain=v1",
		"--untracked-files=normal",
	)
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return fmt.Errorf("current FPF source has uncommitted or untracked changes")
	}
	return nil
}

func runSpecMigrationV2Git(
	ctx context.Context,
	root string,
	args ...string,
) ([]byte, error) {
	commandArgs := []string{"-C", root}
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		outputText := string(output)
		detail := strings.TrimSpace(outputText)
		return nil, fmt.Errorf("read FPF Git state: %w: %s", err, detail)
	}
	return output, nil
}

func validateCLISoftwareMigrationPacket(
	packet specmigrationv2.Packet,
	reviewSoftwareCarrier string,
) error {
	if packet.Source().Carrier().String() != specMigrationV2SourceCarrier {
		return fmt.Errorf("software-system migration-v2 source must be %s", specMigrationV2SourceCarrier)
	}
	if packet.Target().Carrier().String() != specMigrationV2TargetCarrier {
		return fmt.Errorf("software-system migration-v2 final target must be %s", specMigrationV2TargetCarrier)
	}
	if packet.Source().Archive().Carrier().String() != specMigrationV2ArchiveCarrier {
		return fmt.Errorf("software-system migration-v2 archive must be %s", specMigrationV2ArchiveCarrier)
	}
	if reviewSoftwareCarrier == specMigrationV2TargetCarrier {
		return fmt.Errorf("SoftwareSystemSpec review-basis carrier must remain distinct from the final install target")
	}
	return nil
}

func readPacketCarrier(rawPath string) ([]byte, string, error) {
	absolute, err := filepath.Abs(rawPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve migration-v2 packet path: %w", err)
	}
	canonical := filepath.Clean(absolute)
	bytes, err := os.ReadFile(canonical)
	if err != nil {
		return nil, "", fmt.Errorf("read migration-v2 packet carrier %s: %w", canonical, err)
	}
	return bytes, canonical, nil
}

func observeReviewBasis(
	root string,
	basis specmigrationv2.FinalCandidateReviewBasis,
) (string, []byte, error) {
	softwareCarrier := ""
	softwareBytes := []byte(nil)
	for _, binding := range basis.CarrierDigests().Values() {
		bytes, err := readProjectRelativeCarrier(root, binding.Carrier().String())
		if err != nil {
			return "", nil, fmt.Errorf("read %s review-basis carrier: %w", binding.Role(), err)
		}
		observed := specmigrationv2.DigestBytes(bytes)
		if !observed.Equal(binding.Digest()) {
			return "", nil, fmt.Errorf(
				"review-basis carrier digest mismatch for %s: observed %s, expected %s",
				binding.Carrier().String(),
				observed.String(),
				binding.Digest().String(),
			)
		}
		if binding.Role() == specmigrationv2.ReviewSoftwareSystemCarrier {
			softwareCarrier = binding.Carrier().String()
			softwareBytes = bytes
		}
	}
	semanticZeroPass := basis.SemanticZeroPass()
	semanticBytes, err := readProjectRelativeCarrier(root, semanticZeroPass.Carrier().String())
	if err != nil {
		return "", nil, fmt.Errorf("read semantic zero-pass carrier: %w", err)
	}
	observedSemantic := specmigrationv2.DigestBytes(semanticBytes)
	if !observedSemantic.Equal(semanticZeroPass.Digest()) {
		return "", nil, fmt.Errorf(
			"semantic zero-pass digest mismatch for %s: observed %s, expected %s",
			semanticZeroPass.Carrier().String(),
			observedSemantic.String(),
			semanticZeroPass.Digest().String(),
		)
	}
	if softwareCarrier == "" || len(softwareBytes) == 0 {
		return "", nil, fmt.Errorf("strict final candidate has no readable SoftwareSystemSpec review-basis carrier")
	}
	return softwareCarrier, softwareBytes, nil
}

func buildSpecMigrationV2StructuralRequest(
	root string,
	packet specmigrationv2.Packet,
	candidateBytes []byte,
) (specMigrationV2StructuralObservation, specmigrationv2.ProjectRootRef, error) {
	sourceBytes, err := readProjectRelativeCarrier(root, packet.Source().Carrier().String())
	if err != nil {
		return specMigrationV2StructuralObservation{}, specmigrationv2.ProjectRootRef{}, fmt.Errorf("read designated source carrier: %w", err)
	}
	return buildSpecMigrationV2StructuralRequestFromSourceBytes(
		root,
		packet,
		candidateBytes,
		sourceBytes,
	)
}

func buildSpecMigrationV2StructuralRequestFromSourceBytes(
	root string,
	packet specmigrationv2.Packet,
	candidateBytes []byte,
	sourceBytes []byte,
) (specMigrationV2StructuralObservation, specmigrationv2.ProjectRootRef, error) {
	projectRoot, err := specmigrationv2.NewProjectRootRef(root)
	if err != nil {
		return specMigrationV2StructuralObservation{}, specmigrationv2.ProjectRootRef{}, err
	}
	source, err := specmigrationv2.NewSourceSnapshot(specmigrationv2.SourceSnapshotInput{
		Carrier: packet.Source().Carrier(),
		Bytes:   sourceBytes,
	})
	if err != nil {
		return specMigrationV2StructuralObservation{}, specmigrationv2.ProjectRootRef{}, err
	}
	// The packet target is the final installation identity. The observed bytes
	// come from the separately bound SoftwareSystemSpec review carrier; using
	// that review path as the target would turn a draft carrier into authority.
	target, err := specmigrationv2.NewTargetSnapshot(specmigrationv2.TargetSnapshotInput{
		Carrier: packet.Target().Carrier(),
		Bytes:   candidateBytes,
	})
	if err != nil {
		return specMigrationV2StructuralObservation{}, specmigrationv2.ProjectRootRef{}, err
	}
	targetClaims, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: packet.Target().Carrier(),
		Bytes:   candidateBytes,
	})
	if err != nil {
		return specMigrationV2StructuralObservation{}, specmigrationv2.ProjectRootRef{}, err
	}
	outside, err := observeOutsideCarriers(root, packet.OutsideRegistry())
	if err != nil {
		return specMigrationV2StructuralObservation{}, specmigrationv2.ProjectRootRef{}, err
	}
	request, err := specmigrationv2.NewStructuralRequest(specmigrationv2.StructuralRequestInput{
		Packet:           packet,
		ProjectRoot:      projectRoot,
		Source:           source,
		Target:           target,
		TargetClaims:     targetClaims,
		OutsideSnapshots: outside,
	})
	if err != nil {
		return specMigrationV2StructuralObservation{}, specmigrationv2.ProjectRootRef{}, err
	}
	return specMigrationV2StructuralObservation{
		request: request,
		source:  source,
		target:  target,
		claims:  targetClaims,
		outside: outside,
	}, projectRoot, nil
}

func observeOutsideCarriers(
	root string,
	registry specmigrationv2.OutsideCarrierRegistry,
) (specmigrationv2.OutsideCarrierSnapshots, error) {
	values := make([]specmigrationv2.OutsideCarrierSnapshot, 0, len(registry.Values()))
	for _, registration := range registry.Values() {
		bytes, err := readProjectRelativeCarrier(root, registration.Carrier().String())
		if err != nil {
			return specmigrationv2.OutsideCarrierSnapshots{}, fmt.Errorf(
				"read outside-PSS carrier %s: %w",
				registration.ID().String(),
				err,
			)
		}
		snapshot, err := specmigrationv2.NewOutsideCarrierSnapshot(
			specmigrationv2.OutsideCarrierSnapshotInput{
				ID:      registration.ID(),
				Carrier: registration.Carrier(),
				Bytes:   bytes,
			},
		)
		if err != nil {
			return specmigrationv2.OutsideCarrierSnapshots{}, err
		}
		values = append(values, snapshot)
	}
	return specmigrationv2.NewOutsideCarrierSnapshots(values)
}

func readProjectRelativeCarrier(root string, carrier string) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(carrier))
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("carrier path escapes project root")
	}
	return os.ReadFile(path)
}

func pendingSpecMigrationReview() (specmigrationv2.PendingMigrationReview, error) {
	missing, err := specmigrationv2.NewReviewMissingBasisSet(
		[]specmigrationv2.ReviewMissingBasis{specmigrationv2.MissingExactReviewBinding},
	)
	if err != nil {
		return nil, err
	}
	return specmigrationv2.NewPendingMigrationReview(missing)
}

func newSpecMigrationV2ApplyRequest(
	ctx context.Context,
	ledger *specMigrationV2Ledger,
	root string,
	observation specMigrationV2Observation,
	review specmigrationv2.AdmittedMigrationReview,
	requestedAt time.Time,
) (specmigrationv2.ApplyRequest, error) {
	if ledger == nil || ledger.handle == nil {
		return specmigrationv2.ApplyRequest{}, fmt.Errorf("checked project ledger is required for migration apply")
	}
	if err := ledger.handle.Revalidate(ctx); err != nil {
		return specmigrationv2.ApplyRequest{}, fmt.Errorf("revalidate checked project ledger before apply: %w", err)
	}
	required, ok := observation.profileApplicability.Required()
	if !ok {
		return specmigrationv2.ApplyRequest{}, fmt.Errorf("migration apply requires current SoftwareSystemSpec applicability")
	}
	applyRoot, err := specmigrationv2.NewApplyProjectRoot(root)
	if err != nil {
		return specmigrationv2.ApplyRequest{}, err
	}
	return specmigrationv2.NewApplyRequest(
		ctx,
		ledger.profile,
		specmigrationv2.ApplyRequestInput{
			ProjectRoot:          applyRoot,
			Structural:           observation.structural.request,
			ProfileApplicability: required,
			Review:               review,
			RequestedAt:          requestedAt,
		},
	)
}

func applySpecMigrationV2(
	ctx context.Context,
	ledger *specMigrationV2Ledger,
	root string,
	observation specMigrationV2Observation,
	resolution specmigrationv2.MigrationReviewResolution,
	result specMigrationV2Result,
) (specMigrationV2Result, error) {
	review, ok := resolution.(specmigrationv2.AdmittedMigrationReview)
	if !ok {
		return result, fmt.Errorf("applicable migration dry-run did not carry an admitted semantic review")
	}
	request, err := newSpecMigrationV2ApplyRequest(
		ctx,
		ledger,
		root,
		observation,
		review,
		time.Now().UTC(),
	)
	if err != nil {
		return result, err
	}
	applyResult := specmigrationv2.ApplyMigration(ctx, ledger.profile, request)
	if applied, ok := applyResult.(specmigrationv2.Applied); ok {
		result.State = "applied"
		result.Applied = true
		result = presentSpecMigrationV2ReceiptCarrier(result, applied.ReceiptCarrier())
		result.NextAction = "migration effect completed; inspect the durable receipt and resulting carriers"
		return result, nil
	}
	if replayed, ok := applyResult.(specmigrationv2.Replayed); ok {
		result.State = "replayed"
		result.Applied = true
		result = presentSpecMigrationV2ReceiptCarrier(result, replayed.ReceiptCarrier())
		result.NextAction = "existing completed migration effect replayed from its durable receipt"
		return result, nil
	}
	if recovery, blocked := applyResult.(specmigrationv2.RecoveryRequired); blocked {
		return result, fmt.Errorf(
			"migration recovery required at %s: %s",
			recovery.Phase(),
			recovery.Reason(),
		)
	}
	if rejected, blocked := applyResult.(specmigrationv2.ApplyRejected); blocked {
		return result, fmt.Errorf(
			"migration apply rejected (%s): %s",
			rejected.Code(),
			rejected.Reason(),
		)
	}
	return result, fmt.Errorf("migration apply returned an unknown result variant")
}

func presentSpecMigrationV2ReceiptCarrier(
	result specMigrationV2Result,
	carrier specmigrationv2.MigrationEffectReceiptCarrier,
) specMigrationV2Result {
	result.ReceiptCarrier = carrier.Ref().String()
	result.ReceiptCarrierDigest = carrier.Digest().String()
	return result
}

func presentSpecMigrationV2Result(
	packetPath string,
	observation specMigrationV2Observation,
	dryRun specmigrationv2.DryRunResult,
	apply bool,
) specMigrationV2Result {
	packet := observation.packet
	auditCounts := observation.partitionAudit.Counts()
	result := specMigrationV2Result{
		RecordKind:            specMigrationV2RecordKind,
		SchemaVersion:         packet.SchemaVersion(),
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
			SourceSections:       auditCounts.SourceSections(),
			TopLevelDispositions: auditCounts.TopLevelDispositions(),
			SplitSections:        auditCounts.SplitSections(),
			SplitLeaves:          auditCounts.SplitLeaves(),
			WholeSectionOutcomes: auditCounts.WholeSectionOutcomes(),
			LineageEntries:       auditCounts.LineageEntries(),
		},
		ProfileApplicability: string(observation.profileApplicability.Kind()),
		ApplyRequested:       apply,
		Applied:              false,
	}
	return presentSpecMigrationV2DryRunVariant(result, dryRun)
}

func presentSpecMigrationV2DryRunVariant(
	result specMigrationV2Result,
	dryRun specmigrationv2.DryRunResult,
) specMigrationV2Result {
	if value, ok := dryRun.(specmigrationv2.CanonicalUnderdetermined); ok {
		result.State = "underdetermined"
		result.ProfileApplicability = "underdetermined"
		result.ProfileMissingBasis = []string{string(value.Applicability().MissingBasis())}
		result.NextAction = "complete and admit the project profile, then rerun the same packet"
		return result
	}
	if _, ok := dryRun.(specmigrationv2.NotApplicable); ok {
		result.State = "not_applicable"
		result.ProfileApplicability = "not_applicable"
		result.NextAction = "no SoftwareSystemSpec migration is applicable to the admitted project profile"
		return result
	}
	if value, ok := dryRun.(specmigrationv2.PendingReview); ok {
		result.State = "pending_review"
		result.ProfileApplicability = "required"
		for _, basis := range value.MissingBasis().Values() {
			result.ReviewMissingBasis = append(result.ReviewMissingBasis, string(basis))
		}
		result.NextAction = "run haft spec migrate interactively to review the prepared semantic change"
		return result
	}
	if value, ok := dryRun.(specmigrationv2.Invalid); ok {
		result.State = "invalid"
		for _, diagnostic := range value.Diagnostics().Values() {
			result.Diagnostics = append(result.Diagnostics, specMigrationV2Diagnostic{
				Code:    string(diagnostic.Code()),
				Subject: diagnostic.Subject(),
				Detail:  diagnostic.Detail(),
			})
		}
		result.NextAction = "repair the exact packet or observed carrier mismatch; no apply is possible"
		return result
	}
	if _, ok := dryRun.(specmigrationv2.Applicable); ok {
		result.State = "applicable"
		result.ProfileApplicability = "required"
		result.NextAction = "run haft spec migrate again to perform the exact reviewed migration"
		return result
	}
	result.State = "invalid"
	result.ProfileApplicability = "unknown"
	result.Diagnostics = []specMigrationV2Diagnostic{{
		Code:    "unknown_dry_run_variant",
		Subject: result.PacketID,
		Detail:  "migration-v2 dry-run returned an unknown result variant",
	}}
	result.NextAction = "stop: the migration-v2 result contract is not understood"
	return result
}

func writeSpecMigrationV2Result(writer io.Writer, result specMigrationV2Result, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	_, err := fmt.Fprintf(
		writer,
		"Specification migration: %s\nprofile applicability: %s\nsemantic partition audit: %s\nsource sections: %d; target lineage entries: %d\nnext: %s\n",
		result.State,
		result.ProfileApplicability,
		result.PartitionAuditStatus,
		result.PartitionAuditCounts.SourceSections,
		result.PartitionAuditCounts.LineageEntries,
		result.NextAction,
	)
	if err != nil {
		return err
	}
	if result.RecoveryRequested {
		_, err = fmt.Fprintf(
			writer,
			"recovery_phase: %s\nrecovery_reason: %s\n",
			result.RecoveryPhase,
			result.RecoveryReason,
		)
		if err != nil {
			return err
		}
	}
	if result.ReceiptCarrier == "" {
		return nil
	}
	_, err = fmt.Fprintln(writer, "durable receipt: recorded")
	return err
}

func specMigrationV2ApplyPrecondition(result specMigrationV2Result, apply bool) error {
	if !apply {
		return nil
	}
	if result.Applied {
		return nil
	}
	if result.State == "applicable" {
		return nil
	}
	preconditions := map[string]specMigrationPreconditionError{
		"pending_review": {
			code:   specMigrationPendingReviewCode,
			detail: "an admitted human semantic review is required; packet JSON cannot mint review authority",
		},
		"underdetermined": {
			code:   specMigrationProfileUnderdeterminedCode,
			detail: "a current integrity-valid canonical project profile is required",
		},
		"not_applicable": {
			code:   specMigrationProfileNotApplicableCode,
			detail: "the current canonical project profile has no software realization scope",
		},
		"invalid": {
			code:   specMigrationInvalidCode,
			detail: "the exact migration packet or observed carrier set is invalid",
		},
	}
	precondition, found := preconditions[result.State]
	if found {
		return precondition
	}
	return specMigrationPreconditionError{
		code:   specMigrationInvalidCode,
		detail: "migration-v2 apply received an unknown dry-run state",
	}
}
