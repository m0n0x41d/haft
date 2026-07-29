package specmigrationv2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
)

const FinalCandidatePacketCarrierSchema = "haft.spec-migration-v2.final-candidate/v1"

const FinalCandidatePacketCarrierKind = "non_binding_final_candidate"

const (
	packetCarrierRepositoryEditionKind  = "repository_edition"
	packetCarrierWorkingTreeEditionKind = "working_tree_edition"
	packetCarrierMapOneKind             = "map_one"
	packetCarrierSplitOneToManyKind     = "split_one_to_many"
	packetCarrierRetireHistoryKind      = "retire_history"
	packetCarrierOutsidePSSKind         = "outside_pss"
)

var lifecycleSectionRefPattern = regexp.MustCompile(`^(?:TS|SS)(?:\.[A-Za-z0-9][A-Za-z0-9_-]*)+\.[0-9]{3}$`)

type ReviewCarrierDigestInput struct {
	Role    ReviewCarrierRole
	Carrier TargetCarrierID
	Digest  SHA256
}

type SemanticZeroPassInput struct {
	Carrier TargetCarrierID
	Digest  SHA256
}

type LifecycleIntentInput struct {
	SectionRef string
	Operation  LifecycleOperation
}

type FinalCandidateReviewBasisInput struct {
	CarrierDigests   []ReviewCarrierDigestInput
	FPFRevision      string
	SemanticZeroPass SemanticZeroPassInput
	LifecycleIntent  []LifecycleIntentInput
}

// SemanticZeroPassBinding identifies the exact non-binding review-evidence
// carrier. It is evidence material, not human acceptance or apply authority.
type SemanticZeroPassBinding struct {
	carrier TargetCarrierID
	digest  SHA256
}

func (binding SemanticZeroPassBinding) Carrier() TargetCarrierID {
	return binding.carrier
}

func (binding SemanticZeroPassBinding) Digest() SHA256 {
	return binding.digest
}

func (binding SemanticZeroPassBinding) valid() bool {
	return binding.carrier.valid() && binding.digest.valid()
}

// FinalCandidateReviewBasis is an exact, non-binding review basis. Its
// existence cannot mint AdmittedMigrationReview or authorize migration work.
type FinalCandidateReviewBasis struct {
	carrierDigests   ReviewCarrierDigestSet
	fpfRevision      FPFRevision
	semanticZeroPass SemanticZeroPassBinding
	lifecycleIntent  LifecycleIntent
}

func NewFinalCandidateReviewBasis(
	input FinalCandidateReviewBasisInput,
) (FinalCandidateReviewBasis, error) {
	carrierDigests := make([]ReviewCarrierDigest, 0, len(input.CarrierDigests))
	for index, candidate := range input.CarrierDigests {
		binding := ReviewCarrierDigest{
			role:    candidate.Role,
			carrier: candidate.Carrier,
			digest:  candidate.Digest,
		}
		if !validReviewCarrierRole(binding.role) ||
			!binding.carrier.valid() ||
			!binding.digest.valid() {
			return FinalCandidateReviewBasis{}, fmt.Errorf("review carrier digest %d is invalid", index)
		}
		carrierDigests = append(carrierDigests, binding)
	}
	carrierSet := ReviewCarrierDigestSet{values: carrierDigests}
	if err := validateReviewCarrierDigestSet(carrierSet); err != nil {
		return FinalCandidateReviewBasis{}, err
	}
	fpfRevision, err := newFPFRevision(input.FPFRevision)
	if err != nil {
		return FinalCandidateReviewBasis{}, err
	}
	semanticZeroPass := SemanticZeroPassBinding{
		carrier: input.SemanticZeroPass.Carrier,
		digest:  input.SemanticZeroPass.Digest,
	}
	if !semanticZeroPass.valid() {
		return FinalCandidateReviewBasis{}, fmt.Errorf("semantic zero-pass binding is invalid")
	}
	lifecycleValues := make([]LifecycleIntentItem, 0, len(input.LifecycleIntent))
	for index, candidate := range input.LifecycleIntent {
		if !lifecycleSectionRefPattern.MatchString(candidate.SectionRef) {
			return FinalCandidateReviewBasis{}, fmt.Errorf("lifecycle intent item %d has invalid section ref %q", index, candidate.SectionRef)
		}
		if candidate.Operation != LifecycleActivate &&
			candidate.Operation != LifecycleRebaseline {
			return FinalCandidateReviewBasis{}, fmt.Errorf("lifecycle intent item %d has unsupported operation %q", index, candidate.Operation)
		}
		item := LifecycleIntentItem{
			sectionRef: candidate.SectionRef,
			operation:  candidate.Operation,
		}
		lifecycleValues = append(lifecycleValues, item)
	}
	lifecycleIntent := LifecycleIntent{values: lifecycleValues}
	if err := validateFinalCandidateLifecycleIntent(lifecycleIntent); err != nil {
		return FinalCandidateReviewBasis{}, err
	}
	basis := FinalCandidateReviewBasis{
		carrierDigests:   carrierSet,
		fpfRevision:      fpfRevision,
		semanticZeroPass: semanticZeroPass,
		lifecycleIntent:  lifecycleIntent,
	}
	return cloneFinalCandidateReviewBasis(basis), nil
}

func (basis FinalCandidateReviewBasis) CarrierDigests() ReviewCarrierDigestSet {
	values := basis.carrierDigests.Values()
	return ReviewCarrierDigestSet{values: values}
}

func (basis FinalCandidateReviewBasis) FPFRevision() FPFRevision {
	return basis.fpfRevision
}

func (basis FinalCandidateReviewBasis) SemanticZeroPass() SemanticZeroPassBinding {
	return basis.semanticZeroPass
}

func (basis FinalCandidateReviewBasis) LifecycleIntent() LifecycleIntent {
	values := basis.lifecycleIntent.Values()
	return LifecycleIntent{values: values}
}

func (basis FinalCandidateReviewBasis) valid() bool {
	if err := validateReviewCarrierDigestSet(basis.carrierDigests); err != nil {
		return false
	}
	if !fpfRevisionPattern.MatchString(basis.fpfRevision.value) {
		return false
	}
	if !basis.semanticZeroPass.valid() {
		return false
	}
	return validateFinalCandidateLifecycleIntent(basis.lifecycleIntent) == nil
}

type PacketCarrierDigest struct {
	value SHA256
}

func (digest PacketCarrierDigest) String() string {
	return digest.value.String()
}

func (digest PacketCarrierDigest) Equal(other PacketCarrierDigest) bool {
	return digest.value.Equal(other.value)
}

func (digest PacketCarrierDigest) valid() bool {
	return digest.value.valid()
}

// FinalCandidatePacketCarrier is the decoded canonical carrier plus its two
// distinct digests: PacketDigest covers the domain Packet; CarrierDigest
// covers the complete canonical JSON, including the non-binding review basis.
type FinalCandidatePacketCarrier struct {
	packet        Packet
	packetDigest  PacketDigest
	reviewBasis   FinalCandidateReviewBasis
	canonical     []byte
	carrierDigest PacketCarrierDigest
}

func (carrier FinalCandidatePacketCarrier) Packet() Packet {
	return clonePacket(carrier.packet)
}

func (carrier FinalCandidatePacketCarrier) PacketDigest() PacketDigest {
	return carrier.packetDigest
}

func (carrier FinalCandidatePacketCarrier) ReviewBasis() FinalCandidateReviewBasis {
	return cloneFinalCandidateReviewBasis(carrier.reviewBasis)
}

func (carrier FinalCandidatePacketCarrier) CanonicalBytes() []byte {
	return append([]byte{}, carrier.canonical...)
}

func (carrier FinalCandidatePacketCarrier) CarrierDigest() PacketCarrierDigest {
	return carrier.carrierDigest
}

func FinalizePacketCarrier(
	packet Packet,
	reviewBasis FinalCandidateReviewBasis,
) (FinalCandidatePacketCarrier, error) {
	if !reviewBasis.valid() {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("final-candidate review basis is invalid")
	}
	packetDigest, err := PacketDigestOf(packet)
	if err != nil {
		return FinalCandidatePacketCarrier{}, err
	}
	dto, err := newPacketCarrierDTO(packet, packetDigest, reviewBasis)
	if err != nil {
		return FinalCandidatePacketCarrier{}, err
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("encode canonical packet carrier: %w", err)
	}
	carrierDigest := PacketCarrierDigest{value: DigestBytes(canonical)}
	return FinalCandidatePacketCarrier{
		packet:        clonePacket(packet),
		packetDigest:  packetDigest,
		reviewBasis:   cloneFinalCandidateReviewBasis(reviewBasis),
		canonical:     canonical,
		carrierDigest: carrierDigest,
	}, nil
}

func DecodePacketCarrier(value []byte) (FinalCandidatePacketCarrier, error) {
	if err := validateUniqueJSONObject(value); err != nil {
		return FinalCandidatePacketCarrier{}, err
	}
	var dto packetCarrierDTO
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("decode packet carrier: %w", err)
	}
	var trailing any
	trailingErr := decoder.Decode(&trailing)
	if !errors.Is(trailingErr, io.EOF) {
		if trailingErr == nil {
			return FinalCandidatePacketCarrier{}, fmt.Errorf("packet carrier contains more than one JSON root")
		}
		return FinalCandidatePacketCarrier{}, fmt.Errorf("decode trailing packet carrier content: %w", trailingErr)
	}
	packet, err := dto.packet.toDomain()
	if err != nil {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("decode packet domain: %w", err)
	}
	reviewBasis, err := dto.reviewBasis.toDomain()
	if err != nil {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("decode packet review basis: %w", err)
	}
	carrier, err := FinalizePacketCarrier(packet, reviewBasis)
	if err != nil {
		return FinalCandidatePacketCarrier{}, err
	}
	if dto.schema != FinalCandidatePacketCarrierSchema {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("packet carrier schema must be %q", FinalCandidatePacketCarrierSchema)
	}
	if dto.kind != FinalCandidatePacketCarrierKind {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("packet carrier kind must be %q", FinalCandidatePacketCarrierKind)
	}
	declaredDigest, err := NewPacketDigest(dto.packetDigest)
	if err != nil {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("packet carrier packet_digest: %w", err)
	}
	if !declaredDigest.Equal(carrier.packetDigest) {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("packet carrier packet_digest does not match the recomputed domain packet digest")
	}
	if !bytes.Equal(value, carrier.canonical) {
		return FinalCandidatePacketCarrier{}, fmt.Errorf("packet carrier is not in canonical byte form")
	}
	return carrier, nil
}

func validateFinalCandidateLifecycleIntent(intent LifecycleIntent) error {
	if err := validateLifecycleIntent(intent); err != nil {
		return err
	}
	seenSections := make(map[string]struct{}, len(intent.values))
	for _, item := range intent.values {
		if !lifecycleSectionRefPattern.MatchString(item.sectionRef) {
			return fmt.Errorf("final-candidate lifecycle section ref %q is invalid", item.sectionRef)
		}
		if item.operation != LifecycleActivate && item.operation != LifecycleRebaseline {
			return fmt.Errorf("final-candidate lifecycle operation %q is unsupported", item.operation)
		}
		if _, exists := seenSections[item.sectionRef]; exists {
			return fmt.Errorf("final-candidate lifecycle section %q has more than one operation", item.sectionRef)
		}
		seenSections[item.sectionRef] = struct{}{}
	}
	return nil
}

func cloneFinalCandidateReviewBasis(basis FinalCandidateReviewBasis) FinalCandidateReviewBasis {
	carrierDigests := basis.carrierDigests.Values()
	lifecycleIntent := basis.lifecycleIntent.Values()
	return FinalCandidateReviewBasis{
		carrierDigests:   ReviewCarrierDigestSet{values: carrierDigests},
		fpfRevision:      basis.fpfRevision,
		semanticZeroPass: basis.semanticZeroPass,
		lifecycleIntent:  LifecycleIntent{values: lifecycleIntent},
	}
}

func clonePacket(packet Packet) Packet {
	dispositions := packet.SourceDispositions()
	return Packet{
		id:                 packet.id,
		schemaVersion:      packet.schemaVersion,
		source:             packet.source,
		target:             packet.target,
		outsideRegistry:    packet.outsideRegistry,
		sourceDispositions: dispositions,
		lineagePolicy:      packet.LineagePolicy(),
	}
}

type packetCarrierDTO struct {
	schema       string
	kind         string
	packetDigest string
	packet       packetDTO
	reviewBasis  reviewBasisDTO
}

func (dto packetCarrierDTO) MarshalJSON() ([]byte, error) {
	type wire struct {
		Schema       string         `json:"schema"`
		Kind         string         `json:"kind"`
		PacketDigest string         `json:"packet_digest"`
		Packet       packetDTO      `json:"packet"`
		ReviewBasis  reviewBasisDTO `json:"review_basis"`
	}
	value := wire{
		Schema:       dto.schema,
		Kind:         dto.kind,
		PacketDigest: dto.packetDigest,
		Packet:       dto.packet,
		ReviewBasis:  dto.reviewBasis,
	}
	return json.Marshal(value)
}

func (dto *packetCarrierDTO) UnmarshalJSON(value []byte) error {
	type wire struct {
		Schema       string         `json:"schema"`
		Kind         string         `json:"kind"`
		PacketDigest string         `json:"packet_digest"`
		Packet       packetDTO      `json:"packet"`
		ReviewBasis  reviewBasisDTO `json:"review_basis"`
	}
	var decoded wire
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	dto.schema = decoded.Schema
	dto.kind = decoded.Kind
	dto.packetDigest = decoded.PacketDigest
	dto.packet = decoded.Packet
	dto.reviewBasis = decoded.ReviewBasis
	return nil
}

type packetDTO struct {
	ID                 string                 `json:"id"`
	SchemaVersion      uint32                 `json:"schema_version"`
	Source             sourceManifestDTO      `json:"source"`
	Target             targetManifestDTO      `json:"target"`
	OutsideCarriers    []outsideCarrierDTO    `json:"outside_carriers"`
	SourceDispositions []sourceDispositionDTO `json:"source_dispositions"`
}

type sourceManifestDTO struct {
	Carrier    string              `json:"carrier"`
	Digest     string              `json:"digest"`
	ByteLength uint64              `json:"byte_length"`
	Archive    archiveManifestDTO  `json:"archive"`
	Provenance sourceProvenanceDTO `json:"provenance"`
	Sections   []sourceSectionDTO  `json:"sections"`
}

type archiveManifestDTO struct {
	Carrier      string `json:"carrier"`
	SourceDigest string `json:"source_digest"`
}

type sourceSectionDTO struct {
	ID   string       `json:"id"`
	Span exactSpanDTO `json:"span"`
}

type exactSpanDTO struct {
	Start  uint64 `json:"start"`
	Length uint64 `json:"length"`
	Digest string `json:"digest"`
}

type sourceProvenanceDTO struct {
	Kind                   string                     `json:"kind"`
	ProjectRoot            string                     `json:"project_root"`
	Carrier                string                     `json:"carrier"`
	Commit                 *string                    `json:"commit,omitempty"`
	ParentCommit           *string                    `json:"parent_commit,omitempty"`
	ParentSourceDigest     *string                    `json:"parent_source_digest,omitempty"`
	DesignatedSourceDigest string                     `json:"designated_source_digest"`
	Delta                  *worktreeDeltaDTO          `json:"delta,omitempty"`
	ResolutionRecord       provenanceRecordBindingDTO `json:"resolution_record"`
}

type worktreeDeltaDTO struct {
	Format string `json:"format"`
	Digest string `json:"digest"`
}

type provenanceRecordBindingDTO struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type targetManifestDTO struct {
	Carrier    string `json:"carrier"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
}

type outsideCarrierDTO struct {
	ID      string `json:"id"`
	Carrier string `json:"carrier"`
	Digest  string `json:"digest"`
}

type sourceDispositionDTO struct {
	SourceSectionID string         `json:"source_section_id"`
	Disposition     dispositionDTO `json:"disposition"`
}

type dispositionDTO struct {
	Kind           string           `json:"kind"`
	TargetClaimIDs []string         `json:"target_claim_ids,omitempty"`
	Branches       []splitBranchDTO `json:"branches,omitempty"`
	Reason         *string          `json:"reason,omitempty"`
	Meaning        *string          `json:"meaning,omitempty"`
	CarrierRefs    []string         `json:"carrier_refs,omitempty"`
}

type splitBranchDTO struct {
	Fragment    exactSpanDTO   `json:"fragment"`
	Disposition dispositionDTO `json:"disposition"`
}

type reviewBasisDTO struct {
	CarrierDigests   []reviewCarrierDigestDTO `json:"carrier_digests"`
	FPFRevision      string                   `json:"fpf_revision"`
	SemanticZeroPass semanticZeroPassDTO      `json:"semantic_zero_pass"`
	LifecycleIntent  []lifecycleIntentDTO     `json:"lifecycle_intent"`
}

type reviewCarrierDigestDTO struct {
	Role    ReviewCarrierRole `json:"role"`
	Carrier string            `json:"carrier"`
	Digest  string            `json:"digest"`
}

type semanticZeroPassDTO struct {
	Carrier string `json:"carrier"`
	Digest  string `json:"digest"`
}

type lifecycleIntentDTO struct {
	SectionRef string             `json:"section_ref"`
	Operation  LifecycleOperation `json:"operation"`
}

func newPacketCarrierDTO(
	packet Packet,
	packetDigest PacketDigest,
	reviewBasis FinalCandidateReviewBasis,
) (packetCarrierDTO, error) {
	packetValue, err := packetDTOFromDomain(packet)
	if err != nil {
		return packetCarrierDTO{}, err
	}
	reviewBasisValue := reviewBasisDTOFromDomain(reviewBasis)
	return packetCarrierDTO{
		schema:       FinalCandidatePacketCarrierSchema,
		kind:         FinalCandidatePacketCarrierKind,
		packetDigest: packetDigest.String(),
		packet:       packetValue,
		reviewBasis:  reviewBasisValue,
	}, nil
}

func packetDTOFromDomain(packet Packet) (packetDTO, error) {
	provenance, err := sourceProvenanceDTOFromDomain(packet.source.provenance)
	if err != nil {
		return packetDTO{}, err
	}
	sourceSections := append([]SourceSection{}, packet.source.sections...)
	sort.Slice(sourceSections, func(left, right int) bool {
		leftKey := sourceSectionDigestSortKey(sourceSections[left])
		rightKey := sourceSectionDigestSortKey(sourceSections[right])
		return leftKey < rightKey
	})
	sections := make([]sourceSectionDTO, 0, len(sourceSections))
	for _, section := range sourceSections {
		sections = append(sections, sourceSectionDTO{
			ID:   section.id.String(),
			Span: exactSpanDTOFromDomain(section.span),
		})
	}
	outsideCarriers := make([]outsideCarrierDTO, 0, len(packet.outsideRegistry.values))
	for _, registration := range packet.outsideRegistry.values {
		outsideCarriers = append(outsideCarriers, outsideCarrierDTO{
			ID:      registration.id.String(),
			Carrier: registration.carrier.String(),
			Digest:  registration.digest.String(),
		})
	}
	sort.Slice(outsideCarriers, func(left, right int) bool {
		return outsideCarriers[left].ID < outsideCarriers[right].ID
	})
	sourceDispositions := append([]SourceDisposition{}, packet.sourceDispositions...)
	sort.Slice(sourceDispositions, func(left, right int) bool {
		leftKey := sourceDispositionDigestSortKey(sourceDispositions[left])
		rightKey := sourceDispositionDigestSortKey(sourceDispositions[right])
		return leftKey < rightKey
	})
	dispositions := make([]sourceDispositionDTO, 0, len(sourceDispositions))
	for index, sourceDisposition := range sourceDispositions {
		disposition, dispositionErr := dispositionDTOFromDomain(sourceDisposition.disposition)
		if dispositionErr != nil {
			return packetDTO{}, fmt.Errorf("encode source disposition %d: %w", index, dispositionErr)
		}
		dispositions = append(dispositions, sourceDispositionDTO{
			SourceSectionID: sourceDisposition.source.String(),
			Disposition:     disposition,
		})
	}
	return packetDTO{
		ID:            packet.id.String(),
		SchemaVersion: packet.schemaVersion,
		Source: sourceManifestDTO{
			Carrier:    packet.source.carrier.String(),
			Digest:     packet.source.digest.String(),
			ByteLength: packet.source.byteLength.Value(),
			Archive: archiveManifestDTO{
				Carrier:      packet.source.archive.carrier.String(),
				SourceDigest: packet.source.archive.sourceDigest.String(),
			},
			Provenance: provenance,
			Sections:   sections,
		},
		Target: targetManifestDTO{
			Carrier:    packet.target.carrier.String(),
			Digest:     packet.target.digest.String(),
			ByteLength: packet.target.byteLength.Value(),
		},
		OutsideCarriers:    outsideCarriers,
		SourceDispositions: dispositions,
	}, nil
}

func sourceProvenanceDTOFromDomain(
	provenance DesignatedSourceProvenance,
) (sourceProvenanceDTO, error) {
	record := provenance.resolutionRecord
	recordDTO := provenanceRecordBindingDTO{
		Ref:    record.ref.String(),
		Digest: record.digest.String(),
	}
	switch origin := provenance.origin.(type) {
	case RepositoryEdition:
		if !filepath.IsAbs(origin.projectRoot.String()) {
			return sourceProvenanceDTO{}, fmt.Errorf("packet carrier provenance project_root must be absolute")
		}
		commit := origin.commitOID.String()
		return sourceProvenanceDTO{
			Kind:                   packetCarrierRepositoryEditionKind,
			ProjectRoot:            origin.projectRoot.String(),
			Carrier:                origin.carrier.String(),
			Commit:                 &commit,
			DesignatedSourceDigest: origin.bytesDigest.String(),
			ResolutionRecord:       recordDTO,
		}, nil
	case WorkingTreeEdition:
		if !filepath.IsAbs(origin.parent.projectRoot.String()) {
			return sourceProvenanceDTO{}, fmt.Errorf("packet carrier provenance project_root must be absolute")
		}
		parentCommit := origin.parent.commitOID.String()
		parentDigest := origin.parent.bytesDigest.String()
		delta := worktreeDeltaDTO{
			Format: string(origin.delta.format),
			Digest: origin.delta.digest.String(),
		}
		return sourceProvenanceDTO{
			Kind:                   packetCarrierWorkingTreeEditionKind,
			ProjectRoot:            origin.parent.projectRoot.String(),
			Carrier:                origin.parent.carrier.String(),
			ParentCommit:           &parentCommit,
			ParentSourceDigest:     &parentDigest,
			DesignatedSourceDigest: origin.designatedDigest.String(),
			Delta:                  &delta,
			ResolutionRecord:       recordDTO,
		}, nil
	default:
		return sourceProvenanceDTO{}, fmt.Errorf("encode unknown designated-source provenance variant")
	}
}

func exactSpanDTOFromDomain(span ExactByteSpan) exactSpanDTO {
	return exactSpanDTO{
		Start:  span.Start(),
		Length: span.Length().Value(),
		Digest: span.Digest().String(),
	}
}

func dispositionDTOFromDomain(disposition Disposition) (dispositionDTO, error) {
	switch value := disposition.(type) {
	case MapOne:
		claims := value.targetClaims.Values()
		claimIDs := make([]string, 0, len(claims))
		for _, claim := range claims {
			claimIDs = append(claimIDs, claim.String())
		}
		sort.Strings(claimIDs)
		return dispositionDTO{Kind: packetCarrierMapOneKind, TargetClaimIDs: claimIDs}, nil
	case SplitOneToMany:
		sourceBranches := append([]SplitBranch{}, value.branches...)
		sort.Slice(sourceBranches, func(left, right int) bool {
			leftSpan := sourceBranches[left].fragment
			rightSpan := sourceBranches[right].fragment
			leftKey := fmt.Sprintf(
				"%020d\x00%020d\x00%s\x00%s",
				leftSpan.Start(),
				leftSpan.Length().Value(),
				leftSpan.Digest().String(),
				branchDispositionDigestSortKey(sourceBranches[left].disposition),
			)
			rightKey := fmt.Sprintf(
				"%020d\x00%020d\x00%s\x00%s",
				rightSpan.Start(),
				rightSpan.Length().Value(),
				rightSpan.Digest().String(),
				branchDispositionDigestSortKey(sourceBranches[right].disposition),
			)
			return leftKey < rightKey
		})
		branches := make([]splitBranchDTO, 0, len(sourceBranches))
		for index, branch := range sourceBranches {
			branchDisposition, err := branchDispositionDTOFromDomain(branch.disposition)
			if err != nil {
				return dispositionDTO{}, fmt.Errorf("encode split branch %d: %w", index, err)
			}
			branches = append(branches, splitBranchDTO{
				Fragment:    exactSpanDTOFromDomain(branch.fragment),
				Disposition: branchDisposition,
			})
		}
		return dispositionDTO{Kind: packetCarrierSplitOneToManyKind, Branches: branches}, nil
	case RetireHistory:
		reason := value.reason
		return dispositionDTO{Kind: packetCarrierRetireHistoryKind, Reason: &reason}, nil
	case OutsidePSS:
		carrierIDs := value.carriers.Values()
		carrierRefs := make([]string, 0, len(carrierIDs))
		for _, carrierID := range carrierIDs {
			carrierRefs = append(carrierRefs, carrierID.String())
		}
		sort.Strings(carrierRefs)
		meaning := value.meaning
		return dispositionDTO{
			Kind:        packetCarrierOutsidePSSKind,
			Meaning:     &meaning,
			CarrierRefs: carrierRefs,
		}, nil
	default:
		return dispositionDTO{}, fmt.Errorf("encode unknown disposition variant")
	}
}

func branchDispositionDTOFromDomain(
	disposition BranchDisposition,
) (dispositionDTO, error) {
	switch value := disposition.(type) {
	case MapOne:
		return dispositionDTOFromDomain(value)
	case RetireHistory:
		return dispositionDTOFromDomain(value)
	case OutsidePSS:
		return dispositionDTOFromDomain(value)
	default:
		return dispositionDTO{}, fmt.Errorf("encode unknown branch disposition variant")
	}
}

func reviewBasisDTOFromDomain(basis FinalCandidateReviewBasis) reviewBasisDTO {
	carrierDigests := basis.carrierDigests.Values()
	carrierDTOs := make([]reviewCarrierDigestDTO, 0, len(carrierDigests))
	for _, binding := range carrierDigests {
		carrierDTOs = append(carrierDTOs, reviewCarrierDigestDTO{
			Role:    binding.role,
			Carrier: binding.carrier.String(),
			Digest:  binding.digest.String(),
		})
	}
	sort.Slice(carrierDTOs, func(left, right int) bool {
		return carrierDTOs[left].Role < carrierDTOs[right].Role
	})
	lifecycleItems := basis.lifecycleIntent.Values()
	lifecycleDTOs := make([]lifecycleIntentDTO, 0, len(lifecycleItems))
	for _, item := range lifecycleItems {
		lifecycleDTOs = append(lifecycleDTOs, lifecycleIntentDTO{
			SectionRef: item.sectionRef,
			Operation:  item.operation,
		})
	}
	sort.Slice(lifecycleDTOs, func(left, right int) bool {
		leftKey := lifecycleDTOs[left].SectionRef + "\x00" + string(lifecycleDTOs[left].Operation)
		rightKey := lifecycleDTOs[right].SectionRef + "\x00" + string(lifecycleDTOs[right].Operation)
		return leftKey < rightKey
	})
	return reviewBasisDTO{
		CarrierDigests: carrierDTOs,
		FPFRevision:    basis.fpfRevision.String(),
		SemanticZeroPass: semanticZeroPassDTO{
			Carrier: basis.semanticZeroPass.carrier.String(),
			Digest:  basis.semanticZeroPass.digest.String(),
		},
		LifecycleIntent: lifecycleDTOs,
	}
}

func (dto packetDTO) toDomain() (Packet, error) {
	id, err := NewMigrationPacketID(dto.ID)
	if err != nil {
		return Packet{}, err
	}
	source, err := dto.Source.toDomain()
	if err != nil {
		return Packet{}, err
	}
	target, err := dto.Target.toDomain()
	if err != nil {
		return Packet{}, err
	}
	outsideRegistry, err := outsideRegistryToDomain(dto.OutsideCarriers)
	if err != nil {
		return Packet{}, err
	}
	dispositions := make([]SourceDisposition, 0, len(dto.SourceDispositions))
	for index, candidate := range dto.SourceDispositions {
		sourceID, sourceErr := NewSourceSectionID(candidate.SourceSectionID)
		if sourceErr != nil {
			return Packet{}, fmt.Errorf("source disposition %d: %w", index, sourceErr)
		}
		disposition, dispositionErr := candidate.Disposition.toDomain()
		if dispositionErr != nil {
			return Packet{}, fmt.Errorf("source disposition %d: %w", index, dispositionErr)
		}
		value, valueErr := NewSourceDisposition(sourceID, disposition)
		if valueErr != nil {
			return Packet{}, fmt.Errorf("source disposition %d: %w", index, valueErr)
		}
		dispositions = append(dispositions, value)
	}
	packet, err := NewPacket(PacketInput{
		ID:                 id,
		SchemaVersion:      dto.SchemaVersion,
		Source:             source,
		Target:             target,
		OutsideRegistry:    outsideRegistry,
		SourceDispositions: dispositions,
	})
	if err != nil {
		return Packet{}, err
	}
	return packet, nil
}

func (dto sourceManifestDTO) toDomain() (SourceManifest, error) {
	carrier, err := NewSourceCarrierID(dto.Carrier)
	if err != nil {
		return SourceManifest{}, err
	}
	digest, err := NewSourceDigest(dto.Digest)
	if err != nil {
		return SourceManifest{}, err
	}
	byteLength, err := NewByteLength(dto.ByteLength)
	if err != nil {
		return SourceManifest{}, err
	}
	archiveCarrier, err := NewArchiveCarrierID(dto.Archive.Carrier)
	if err != nil {
		return SourceManifest{}, err
	}
	archiveDigest, err := NewSourceDigest(dto.Archive.SourceDigest)
	if err != nil {
		return SourceManifest{}, err
	}
	archive, err := NewArchiveManifest(archiveCarrier, archiveDigest)
	if err != nil {
		return SourceManifest{}, err
	}
	provenance, err := dto.Provenance.toDomain()
	if err != nil {
		return SourceManifest{}, err
	}
	sections := make([]SourceSection, 0, len(dto.Sections))
	for index, candidate := range dto.Sections {
		sectionID, sectionErr := NewSourceSectionID(candidate.ID)
		if sectionErr != nil {
			return SourceManifest{}, fmt.Errorf("source section %d: %w", index, sectionErr)
		}
		span, spanErr := candidate.Span.toDomain()
		if spanErr != nil {
			return SourceManifest{}, fmt.Errorf("source section %d: %w", index, spanErr)
		}
		section, sectionErr := NewSourceSection(sectionID, span)
		if sectionErr != nil {
			return SourceManifest{}, fmt.Errorf("source section %d: %w", index, sectionErr)
		}
		sections = append(sections, section)
	}
	manifest, err := NewSourceManifest(SourceManifestInput{
		Carrier:    carrier,
		Digest:     digest,
		ByteLength: byteLength,
		Archive:    archive,
		Provenance: provenance,
		Sections:   sections,
	})
	if err != nil {
		return SourceManifest{}, err
	}
	return manifest, nil
}

func (dto sourceProvenanceDTO) toDomain() (DesignatedSourceProvenance, error) {
	if !filepath.IsAbs(dto.ProjectRoot) {
		return DesignatedSourceProvenance{}, fmt.Errorf("packet carrier provenance project_root must be absolute")
	}
	projectRoot, err := NewProjectRootRef(dto.ProjectRoot)
	if err != nil {
		return DesignatedSourceProvenance{}, err
	}
	carrier, err := NewSourceCarrierID(dto.Carrier)
	if err != nil {
		return DesignatedSourceProvenance{}, err
	}
	designatedDigest, err := NewSourceDigest(dto.DesignatedSourceDigest)
	if err != nil {
		return DesignatedSourceProvenance{}, err
	}
	resolutionRecord, err := dto.ResolutionRecord.toDomain()
	if err != nil {
		return DesignatedSourceProvenance{}, err
	}
	var origin SourceEditionOrigin
	switch dto.Kind {
	case packetCarrierRepositoryEditionKind:
		value, originErr := dto.repositoryOrigin(projectRoot, carrier, designatedDigest)
		if originErr != nil {
			return DesignatedSourceProvenance{}, originErr
		}
		origin = value
	case packetCarrierWorkingTreeEditionKind:
		value, originErr := dto.workingTreeOrigin(projectRoot, carrier, designatedDigest)
		if originErr != nil {
			return DesignatedSourceProvenance{}, originErr
		}
		origin = value
	default:
		return DesignatedSourceProvenance{}, fmt.Errorf("unknown source provenance kind %q", dto.Kind)
	}
	provenance, err := NewDesignatedSourceProvenance(origin, resolutionRecord)
	if err != nil {
		return DesignatedSourceProvenance{}, err
	}
	return provenance, nil
}

func (dto sourceProvenanceDTO) repositoryOrigin(
	projectRoot ProjectRootRef,
	carrier SourceCarrierID,
	designatedDigest SourceDigest,
) (RepositoryEdition, error) {
	if dto.Commit == nil {
		return RepositoryEdition{}, fmt.Errorf("repository provenance requires commit")
	}
	if dto.ParentCommit != nil || dto.ParentSourceDigest != nil || dto.Delta != nil {
		return RepositoryEdition{}, fmt.Errorf("repository provenance cannot contain working-tree fields")
	}
	commit, err := NewGitCommitOID(*dto.Commit)
	if err != nil {
		return RepositoryEdition{}, err
	}
	edition, err := NewRepositoryEdition(projectRoot, commit, carrier, designatedDigest)
	if err != nil {
		return RepositoryEdition{}, err
	}
	return edition, nil
}

func (dto sourceProvenanceDTO) workingTreeOrigin(
	projectRoot ProjectRootRef,
	carrier SourceCarrierID,
	designatedDigest SourceDigest,
) (WorkingTreeEdition, error) {
	if dto.Commit != nil {
		return WorkingTreeEdition{}, fmt.Errorf("working-tree provenance cannot contain repository commit")
	}
	if dto.ParentCommit == nil || dto.ParentSourceDigest == nil || dto.Delta == nil {
		return WorkingTreeEdition{}, fmt.Errorf("working-tree provenance requires parent commit, parent digest, and delta")
	}
	parentCommit, err := NewGitCommitOID(*dto.ParentCommit)
	if err != nil {
		return WorkingTreeEdition{}, err
	}
	parentDigest, err := NewSourceDigest(*dto.ParentSourceDigest)
	if err != nil {
		return WorkingTreeEdition{}, err
	}
	parent, err := NewRepositoryEdition(projectRoot, parentCommit, carrier, parentDigest)
	if err != nil {
		return WorkingTreeEdition{}, err
	}
	deltaDigest, err := NewWorktreeDeltaDigest(dto.Delta.Digest)
	if err != nil {
		return WorkingTreeEdition{}, err
	}
	delta, err := NewWorktreeDeltaBinding(WorktreeDeltaFormat(dto.Delta.Format), deltaDigest)
	if err != nil {
		return WorkingTreeEdition{}, err
	}
	edition, err := NewWorkingTreeEdition(parent, designatedDigest, delta)
	if err != nil {
		return WorkingTreeEdition{}, err
	}
	return edition, nil
}

func (dto provenanceRecordBindingDTO) toDomain() (ProvenanceRecordBinding, error) {
	ref, err := NewProvenanceRecordRef(dto.Ref)
	if err != nil {
		return ProvenanceRecordBinding{}, err
	}
	digest, err := NewProvenanceRecordDigest(dto.Digest)
	if err != nil {
		return ProvenanceRecordBinding{}, err
	}
	binding, err := NewProvenanceRecordBinding(ref, digest)
	if err != nil {
		return ProvenanceRecordBinding{}, err
	}
	return binding, nil
}

func (dto exactSpanDTO) toDomain() (ExactByteSpan, error) {
	length, err := NewByteLength(dto.Length)
	if err != nil {
		return ExactByteSpan{}, err
	}
	digest, err := NewFragmentDigest(dto.Digest)
	if err != nil {
		return ExactByteSpan{}, err
	}
	span, err := NewExactByteSpan(dto.Start, length, digest)
	if err != nil {
		return ExactByteSpan{}, err
	}
	return span, nil
}

func (dto targetManifestDTO) toDomain() (TargetManifest, error) {
	carrier, err := NewTargetCarrierID(dto.Carrier)
	if err != nil {
		return TargetManifest{}, err
	}
	digest, err := NewTargetDigest(dto.Digest)
	if err != nil {
		return TargetManifest{}, err
	}
	byteLength, err := NewByteLength(dto.ByteLength)
	if err != nil {
		return TargetManifest{}, err
	}
	manifest, err := NewTargetManifest(TargetManifestInput{
		Carrier:    carrier,
		Digest:     digest,
		ByteLength: byteLength,
	})
	if err != nil {
		return TargetManifest{}, err
	}
	return manifest, nil
}

func outsideRegistryToDomain(
	values []outsideCarrierDTO,
) (OutsideCarrierRegistry, error) {
	registrations := make([]OutsideCarrierRegistration, 0, len(values))
	for index, candidate := range values {
		id, err := NewOutsideCarrierID(candidate.ID)
		if err != nil {
			return OutsideCarrierRegistry{}, fmt.Errorf("outside carrier %d: %w", index, err)
		}
		carrier, err := NewSourceCarrierID(candidate.Carrier)
		if err != nil {
			return OutsideCarrierRegistry{}, fmt.Errorf("outside carrier %d: %w", index, err)
		}
		digest, err := NewOutsideCarrierDigest(candidate.Digest)
		if err != nil {
			return OutsideCarrierRegistry{}, fmt.Errorf("outside carrier %d: %w", index, err)
		}
		registration, err := NewOutsideCarrierRegistration(OutsideCarrierRegistrationInput{
			ID:      id,
			Carrier: carrier,
			Digest:  digest,
		})
		if err != nil {
			return OutsideCarrierRegistry{}, fmt.Errorf("outside carrier %d: %w", index, err)
		}
		registrations = append(registrations, registration)
	}
	registry, err := NewOutsideCarrierRegistry(registrations)
	if err != nil {
		return OutsideCarrierRegistry{}, err
	}
	return registry, nil
}

func (dto dispositionDTO) toDomain() (Disposition, error) {
	switch dto.Kind {
	case packetCarrierMapOneKind:
		return dto.mapOne()
	case packetCarrierSplitOneToManyKind:
		return dto.splitOneToMany()
	case packetCarrierRetireHistoryKind:
		return dto.retireHistory()
	case packetCarrierOutsidePSSKind:
		return dto.outsidePSS()
	default:
		return nil, fmt.Errorf("unknown disposition kind %q", dto.Kind)
	}
}

func (dto dispositionDTO) toBranchDomain() (BranchDisposition, error) {
	switch dto.Kind {
	case packetCarrierMapOneKind:
		return dto.mapOne()
	case packetCarrierRetireHistoryKind:
		return dto.retireHistory()
	case packetCarrierOutsidePSSKind:
		return dto.outsidePSS()
	case packetCarrierSplitOneToManyKind:
		return nil, fmt.Errorf("split branch disposition cannot recursively split")
	default:
		return nil, fmt.Errorf("unknown split branch disposition kind %q", dto.Kind)
	}
}

func (dto dispositionDTO) mapOne() (MapOne, error) {
	if len(dto.TargetClaimIDs) == 0 {
		return MapOne{}, fmt.Errorf("map_one requires target_claim_ids")
	}
	if dto.Branches != nil || dto.Reason != nil || dto.Meaning != nil || dto.CarrierRefs != nil {
		return MapOne{}, fmt.Errorf("map_one contains fields from another disposition variant")
	}
	claims := make([]TargetAtomicClaimID, 0, len(dto.TargetClaimIDs))
	for index, raw := range dto.TargetClaimIDs {
		claim, err := NewTargetAtomicClaimID(raw)
		if err != nil {
			return MapOne{}, fmt.Errorf("map_one target claim %d: %w", index, err)
		}
		claims = append(claims, claim)
	}
	claimSet, err := NewTargetClaimSet(claims)
	if err != nil {
		return MapOne{}, err
	}
	mapping, err := NewMapOne(claimSet)
	if err != nil {
		return MapOne{}, err
	}
	return mapping, nil
}

func (dto dispositionDTO) splitOneToMany() (SplitOneToMany, error) {
	if len(dto.Branches) < 2 {
		return SplitOneToMany{}, fmt.Errorf("split_one_to_many requires at least two branches")
	}
	if dto.TargetClaimIDs != nil || dto.Reason != nil || dto.Meaning != nil || dto.CarrierRefs != nil {
		return SplitOneToMany{}, fmt.Errorf("split_one_to_many contains fields from another disposition variant")
	}
	branches := make([]SplitBranch, 0, len(dto.Branches))
	for index, candidate := range dto.Branches {
		fragment, err := candidate.Fragment.toDomain()
		if err != nil {
			return SplitOneToMany{}, fmt.Errorf("split branch %d fragment: %w", index, err)
		}
		disposition, err := candidate.Disposition.toBranchDomain()
		if err != nil {
			return SplitOneToMany{}, fmt.Errorf("split branch %d disposition: %w", index, err)
		}
		branch, err := NewSplitBranch(fragment, disposition)
		if err != nil {
			return SplitOneToMany{}, fmt.Errorf("split branch %d: %w", index, err)
		}
		branches = append(branches, branch)
	}
	split, err := NewSplitOneToMany(branches)
	if err != nil {
		return SplitOneToMany{}, err
	}
	return split, nil
}

func (dto dispositionDTO) retireHistory() (RetireHistory, error) {
	if dto.Reason == nil {
		return RetireHistory{}, fmt.Errorf("retire_history requires reason")
	}
	if dto.TargetClaimIDs != nil || dto.Branches != nil || dto.Meaning != nil || dto.CarrierRefs != nil {
		return RetireHistory{}, fmt.Errorf("retire_history contains fields from another disposition variant")
	}
	retirement, err := NewRetireHistory(*dto.Reason)
	if err != nil {
		return RetireHistory{}, err
	}
	return retirement, nil
}

func (dto dispositionDTO) outsidePSS() (OutsidePSS, error) {
	if dto.Meaning == nil || len(dto.CarrierRefs) == 0 {
		return OutsidePSS{}, fmt.Errorf("outside_pss requires meaning and carrier_refs")
	}
	if dto.TargetClaimIDs != nil || dto.Branches != nil || dto.Reason != nil {
		return OutsidePSS{}, fmt.Errorf("outside_pss contains fields from another disposition variant")
	}
	carrierIDs := make([]OutsideCarrierID, 0, len(dto.CarrierRefs))
	for index, raw := range dto.CarrierRefs {
		carrierID, err := NewOutsideCarrierID(raw)
		if err != nil {
			return OutsidePSS{}, fmt.Errorf("outside_pss carrier ref %d: %w", index, err)
		}
		carrierIDs = append(carrierIDs, carrierID)
	}
	carrierSet, err := NewOutsideCarrierSet(carrierIDs)
	if err != nil {
		return OutsidePSS{}, err
	}
	outside, err := NewOutsidePSS(*dto.Meaning, carrierSet)
	if err != nil {
		return OutsidePSS{}, err
	}
	return outside, nil
}

func (dto reviewBasisDTO) toDomain() (FinalCandidateReviewBasis, error) {
	carrierDigests := make([]ReviewCarrierDigestInput, 0, len(dto.CarrierDigests))
	for index, candidate := range dto.CarrierDigests {
		carrier, err := NewTargetCarrierID(candidate.Carrier)
		if err != nil {
			return FinalCandidateReviewBasis{}, fmt.Errorf("review carrier digest %d: %w", index, err)
		}
		digest, err := NewSHA256(candidate.Digest)
		if err != nil {
			return FinalCandidateReviewBasis{}, fmt.Errorf("review carrier digest %d: %w", index, err)
		}
		carrierDigests = append(carrierDigests, ReviewCarrierDigestInput{
			Role:    candidate.Role,
			Carrier: carrier,
			Digest:  digest,
		})
	}
	semanticCarrier, err := NewTargetCarrierID(dto.SemanticZeroPass.Carrier)
	if err != nil {
		return FinalCandidateReviewBasis{}, err
	}
	semanticDigest, err := NewSHA256(dto.SemanticZeroPass.Digest)
	if err != nil {
		return FinalCandidateReviewBasis{}, err
	}
	lifecycleIntent := make([]LifecycleIntentInput, 0, len(dto.LifecycleIntent))
	for _, item := range dto.LifecycleIntent {
		lifecycleIntent = append(
			lifecycleIntent,
			LifecycleIntentInput(item),
		)
	}
	basis, err := NewFinalCandidateReviewBasis(FinalCandidateReviewBasisInput{
		CarrierDigests: carrierDigests,
		FPFRevision:    dto.FPFRevision,
		SemanticZeroPass: SemanticZeroPassInput{
			Carrier: semanticCarrier,
			Digest:  semanticDigest,
		},
		LifecycleIntent: lifecycleIntent,
	})
	if err != nil {
		return FinalCandidateReviewBasis{}, err
	}
	return basis, nil
}

func validateUniqueJSONObject(value []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	first, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode packet carrier JSON root: %w", err)
	}
	opening, ok := first.(json.Delim)
	if !ok || opening != '{' {
		return fmt.Errorf("packet carrier JSON root must be one object")
	}
	if err := scanUniqueJSONObject(decoder); err != nil {
		return err
	}
	trailing, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode trailing packet carrier content: %w", err)
	}
	return fmt.Errorf("packet carrier contains trailing JSON token %v", trailing)
}

func scanUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		return scanUniqueJSONObject(decoder)
	case '[':
		return scanUniqueJSONArray(decoder)
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

func scanUniqueJSONObject(decoder *json.Decoder) error {
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("JSON object key is not a string")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("packet carrier contains duplicate JSON object key %q", key)
		}
		seen[key] = struct{}{}
		if err := scanUniqueJSONValue(decoder); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := closing.(json.Delim)
	if !ok || delimiter != '}' {
		return fmt.Errorf("JSON object has invalid closing delimiter")
	}
	return nil
}

func scanUniqueJSONArray(decoder *json.Decoder) error {
	for decoder.More() {
		if err := scanUniqueJSONValue(decoder); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := closing.(json.Delim)
	if !ok || delimiter != ']' {
		return fmt.Errorf("JSON array has invalid closing delimiter")
	}
	return nil
}
