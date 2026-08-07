package specmigrationv2

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var fpfRevisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type ReviewMissingBasis string

const (
	MissingHumanSemanticZeroReview ReviewMissingBasis = "human_semantic_zero_pass"
	MissingExactReviewBinding      ReviewMissingBasis = "exact_review_binding"
	MissingLifecycleResolution     ReviewMissingBasis = "lifecycle_resolution"
)

type ReviewMissingBasisSet struct {
	values []ReviewMissingBasis
}

func NewReviewMissingBasisSet(values []ReviewMissingBasis) (ReviewMissingBasisSet, error) {
	if len(values) == 0 {
		return ReviewMissingBasisSet{}, fmt.Errorf("pending migration review requires missing basis")
	}
	seen := make(map[ReviewMissingBasis]struct{}, len(values))
	result := make([]ReviewMissingBasis, 0, len(values))
	for _, value := range values {
		if !validReviewMissingBasis(value) {
			return ReviewMissingBasisSet{}, fmt.Errorf("unknown migration review missing basis %q", value)
		}
		if _, exists := seen[value]; exists {
			return ReviewMissingBasisSet{}, fmt.Errorf("duplicate migration review missing basis %q", value)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left] < result[right]
	})
	return ReviewMissingBasisSet{values: result}, nil
}

func (set ReviewMissingBasisSet) Values() []ReviewMissingBasis {
	return append([]ReviewMissingBasis{}, set.values...)
}

func (set ReviewMissingBasisSet) valid() bool {
	_, err := NewReviewMissingBasisSet(set.values)
	return err == nil
}

type MigrationReviewResolution interface {
	migrationReviewResolutionVariant()
}

// AdmittedMigrationReview is the sealed semantic-review result required by
// the migration effect boundary. Callers can consume a value returned by
// ReviewAdmissionService, but cannot implement or construct one themselves.
// The review is not project-profile authority and does not perform migration
// Work merely by existing.
type AdmittedMigrationReview interface {
	MigrationReviewResolution
	ReviewRef() ReviewRef
	ReviewAdmissionDigest() SHA256
	SpeechActRef() SemanticReviewSpeechActRef
	SpeechActDigest() SHA256
	ProjectRoot() ApplyProjectRoot
	PacketDigest() PacketDigest
	PacketCarrierDigest() PacketCarrierDigest
	PartitionAudit() PacketPartitionAuditBinding
	SourceCarrier() SourceCarrierID
	SourceDigest() SourceDigest
	TargetCarrierDigests() ReviewCarrierDigestSet
	FPFRevision() FPFRevision
	SemanticZeroPass() SemanticZeroPassBinding
	LifecycleIntent() LifecycleIntent
	admittedMigrationReviewVariant()
}

type PendingMigrationReview interface {
	MigrationReviewResolution
	MissingBasis() ReviewMissingBasisSet
	pendingMigrationReviewVariant()
}

type pendingMigrationReview struct {
	missing ReviewMissingBasisSet
}

func NewPendingMigrationReview(
	missing ReviewMissingBasisSet,
) (PendingMigrationReview, error) {
	if !missing.valid() {
		return nil, fmt.Errorf("pending migration review missing basis is invalid")
	}
	values := missing.Values()
	copy := ReviewMissingBasisSet{values: values}
	return pendingMigrationReview{missing: copy}, nil
}

func (pendingMigrationReview) migrationReviewResolutionVariant() {}
func (pendingMigrationReview) pendingMigrationReviewVariant()    {}

func (review pendingMigrationReview) MissingBasis() ReviewMissingBasisSet {
	values := review.missing.Values()
	return ReviewMissingBasisSet{values: values}
}

type ReviewRef struct {
	value string
}

func newReviewRef(raw string) (ReviewRef, error) {
	if raw != strings.TrimSpace(raw) {
		return ReviewRef{}, fmt.Errorf("migration review ref must be canonical without surrounding whitespace")
	}
	value, err := requireNarrative("migration review ref", raw)
	if err != nil {
		return ReviewRef{}, err
	}
	return ReviewRef{value: value}, nil
}

func (ref ReviewRef) String() string {
	return ref.value
}

type SemanticReviewSpeechActRef struct {
	value string
}

func NewSemanticReviewSpeechActRef(raw string) (SemanticReviewSpeechActRef, error) {
	value, err := requireCanonicalReviewReference("semantic-review SpeechAct ref", raw)
	if err != nil {
		return SemanticReviewSpeechActRef{}, err
	}
	return SemanticReviewSpeechActRef{value: value}, nil
}

func (ref SemanticReviewSpeechActRef) String() string {
	return ref.value
}

func (ref SemanticReviewSpeechActRef) valid() bool {
	_, err := requireCanonicalReviewReference("semantic-review SpeechAct ref", ref.value)
	return err == nil
}

type FPFRevision struct {
	value string
}

func newFPFRevision(raw string) (FPFRevision, error) {
	if raw != strings.TrimSpace(raw) || !fpfRevisionPattern.MatchString(raw) {
		return FPFRevision{}, fmt.Errorf("FPF revision must be an exact 40-character lowercase commit SHA")
	}
	return FPFRevision{value: raw}, nil
}

func (revision FPFRevision) String() string {
	return revision.value
}

type ReviewCarrierRole string

const (
	ReviewTargetSystemCarrier   ReviewCarrierRole = "target_system"
	ReviewSoftwareSystemCarrier ReviewCarrierRole = "software_system"
	ReviewTermMapCarrier        ReviewCarrierRole = "term_map"
)

type ReviewCarrierDigest struct {
	role    ReviewCarrierRole
	carrier TargetCarrierID
	digest  SHA256
}

func (binding ReviewCarrierDigest) Role() ReviewCarrierRole {
	return binding.role
}

func (binding ReviewCarrierDigest) Carrier() TargetCarrierID {
	return binding.carrier
}

func (binding ReviewCarrierDigest) Digest() SHA256 {
	return binding.digest
}

type ReviewCarrierDigestSet struct {
	values []ReviewCarrierDigest
}

func (set ReviewCarrierDigestSet) Values() []ReviewCarrierDigest {
	return append([]ReviewCarrierDigest{}, set.values...)
}

type LifecycleOperation string

const (
	LifecycleReopen     LifecycleOperation = "reopen"
	LifecycleRebaseline LifecycleOperation = "rebaseline"
	LifecycleActivate   LifecycleOperation = "activate"
)

type LifecycleIntentItem struct {
	sectionRef string
	operation  LifecycleOperation
}

func (item LifecycleIntentItem) SectionRef() string {
	return item.sectionRef
}

func (item LifecycleIntentItem) Operation() LifecycleOperation {
	return item.operation
}

type LifecycleIntent struct {
	values []LifecycleIntentItem
}

func (intent LifecycleIntent) Values() []LifecycleIntentItem {
	return append([]LifecycleIntentItem{}, intent.values...)
}

// admittedMigrationReview is deliberately unexported. Only the durable review
// admission service mints it after an exact package-captured SpeechAct and the
// current packet, source, review carriers, semantic zero-pass, FPF revision,
// and lifecycle intent have all been revalidated.
type admittedMigrationReview struct {
	reviewRef            ReviewRef
	admissionDigest      SHA256
	speechActRef         SemanticReviewSpeechActRef
	speechActDigest      SHA256
	projectRoot          ApplyProjectRoot
	packetDigest         PacketDigest
	packetCarrierDigest  PacketCarrierDigest
	partitionAudit       PacketPartitionAuditBinding
	sourceCarrier        SourceCarrierID
	sourceDigest         SourceDigest
	targetCarrierDigests ReviewCarrierDigestSet
	fpfRevision          FPFRevision
	semanticZeroPass     SemanticZeroPassBinding
	lifecycleIntent      LifecycleIntent
}

func (admittedMigrationReview) migrationReviewResolutionVariant() {}
func (admittedMigrationReview) admittedMigrationReviewVariant()   {}

func (review admittedMigrationReview) ReviewRef() ReviewRef {
	return review.reviewRef
}

func (review admittedMigrationReview) ReviewAdmissionDigest() SHA256 {
	return review.admissionDigest
}

func (review admittedMigrationReview) SpeechActRef() SemanticReviewSpeechActRef {
	return review.speechActRef
}

func (review admittedMigrationReview) SpeechActDigest() SHA256 {
	return review.speechActDigest
}

func (review admittedMigrationReview) ProjectRoot() ApplyProjectRoot {
	return review.projectRoot
}

func (review admittedMigrationReview) PacketDigest() PacketDigest {
	return review.packetDigest
}

func (review admittedMigrationReview) PacketCarrierDigest() PacketCarrierDigest {
	return review.packetCarrierDigest
}

func (review admittedMigrationReview) PartitionAudit() PacketPartitionAuditBinding {
	return review.partitionAudit
}

func (review admittedMigrationReview) SourceCarrier() SourceCarrierID {
	return review.sourceCarrier
}

func (review admittedMigrationReview) SourceDigest() SourceDigest {
	return review.sourceDigest
}

func (review admittedMigrationReview) TargetCarrierDigests() ReviewCarrierDigestSet {
	values := review.targetCarrierDigests.Values()
	return ReviewCarrierDigestSet{values: values}
}

func (review admittedMigrationReview) FPFRevision() FPFRevision {
	return review.fpfRevision
}

func (review admittedMigrationReview) SemanticZeroPass() SemanticZeroPassBinding {
	return review.semanticZeroPass
}

func (review admittedMigrationReview) LifecycleIntent() LifecycleIntent {
	values := review.lifecycleIntent.Values()
	return LifecycleIntent{values: values}
}

func validReviewMissingBasis(value ReviewMissingBasis) bool {
	switch value {
	case MissingHumanSemanticZeroReview:
		return true
	case MissingExactReviewBinding:
		return true
	case MissingLifecycleResolution:
		return true
	default:
		return false
	}
}

func validateAdmittedMigrationReview(review admittedMigrationReview) error {
	if strings.TrimSpace(review.reviewRef.value) == "" {
		return fmt.Errorf("admitted migration review ref is required")
	}
	if !review.admissionDigest.valid() ||
		!review.speechActRef.valid() ||
		!review.speechActDigest.valid() {
		return fmt.Errorf("admitted migration review admission and SpeechAct bindings are required")
	}
	if !review.projectRoot.valid() ||
		!review.packetDigest.valid() ||
		!review.packetCarrierDigest.valid() ||
		!review.partitionAudit.valid() ||
		!review.sourceCarrier.valid() ||
		!review.sourceDigest.valid() {
		return fmt.Errorf("admitted migration review root, packet, carrier, and source bindings are required")
	}
	if err := validateReviewCarrierDigestSet(review.targetCarrierDigests); err != nil {
		return err
	}
	if !fpfRevisionPattern.MatchString(review.fpfRevision.value) {
		return fmt.Errorf("admitted migration review FPF revision is invalid")
	}
	if !review.semanticZeroPass.valid() {
		return fmt.Errorf("admitted migration review semantic zero-pass binding is invalid")
	}
	if err := validateLifecycleIntent(review.lifecycleIntent); err != nil {
		return err
	}
	return nil
}

func requireCanonicalReviewReference(name string, raw string) (string, error) {
	if raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("%s must be canonical without surrounding whitespace", name)
	}
	return requireNarrative(name, raw)
}

func canonicalReviewTime(value time.Time) time.Time {
	return value.Round(0).UTC()
}

func formatReviewTime(value time.Time) string {
	return canonicalReviewTime(value).Format(time.RFC3339Nano)
}

func parseReviewTime(raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse semantic-review time: %w", err)
	}
	canonical := canonicalReviewTime(value)
	if raw != formatReviewTime(canonical) {
		return time.Time{}, fmt.Errorf("semantic-review time must use canonical UTC RFC3339Nano form")
	}
	return canonical, nil
}

func validateReviewCarrierDigestSet(set ReviewCarrierDigestSet) error {
	if len(set.values) != 3 {
		return fmt.Errorf("admitted migration review requires target-system, software-system, and term-map digests")
	}
	seen := map[ReviewCarrierRole]struct{}{}
	seenCarriers := map[string]struct{}{}
	for _, value := range set.values {
		if !validReviewCarrierRole(value.role) {
			return fmt.Errorf("admitted migration review carrier role %q is invalid", value.role)
		}
		if _, exists := seen[value.role]; exists {
			return fmt.Errorf("admitted migration review carrier role %q is duplicated", value.role)
		}
		if !value.carrier.valid() || !value.digest.valid() {
			return fmt.Errorf("admitted migration review carrier %q is invalid", value.role)
		}
		if _, exists := seenCarriers[value.carrier.String()]; exists {
			return fmt.Errorf("admitted migration review carrier %q is reused across roles", value.carrier.String())
		}
		seen[value.role] = struct{}{}
		seenCarriers[value.carrier.String()] = struct{}{}
	}
	return nil
}

func validateLifecycleIntent(intent LifecycleIntent) error {
	if len(intent.values) == 0 {
		return fmt.Errorf("admitted migration review lifecycle intent is required")
	}
	seen := map[string]struct{}{}
	for _, value := range intent.values {
		if value.sectionRef != strings.TrimSpace(value.sectionRef) ||
			value.sectionRef == "" ||
			!validLifecycleOperation(value.operation) {
			return fmt.Errorf("admitted migration review lifecycle intent item is invalid")
		}
		key := value.sectionRef + "\x00" + string(value.operation)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("admitted migration review lifecycle intent item is duplicated")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validReviewCarrierRole(role ReviewCarrierRole) bool {
	return role == ReviewTargetSystemCarrier ||
		role == ReviewSoftwareSystemCarrier ||
		role == ReviewTermMapCarrier
}

func validLifecycleOperation(operation LifecycleOperation) bool {
	return operation == LifecycleReopen ||
		operation == LifecycleRebaseline ||
		operation == LifecycleActivate
}
