package legacyimport

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const ImportPlanSchemaVersionV1 = "haft.legacy-import.plan/v1"

// ImportPlanPosture is deliberately weaker than semantic admission. The plan
// preserves exact historical carriers and the classifier's explicit
// uncertainty; it never establishes Kind membership, scope, authority, or
// performed Work.
const ImportPlanPosture = "opaque_history_only_no_semantic_admission"

// OpaqueCarrierHistory preserves one exact legacy carrier and its source
// coordinate. It is not a ProjectRecordCarrierV1 and cannot establish the
// carrier's subject, Kind, scope, truth, or authority.
type OpaqueCarrierHistory struct {
	snapshot CarrierSnapshot
}

func newOpaqueCarrierHistory(
	snapshot CarrierSnapshot,
) (OpaqueCarrierHistory, error) {
	if !snapshot.valid() {
		return OpaqueCarrierHistory{}, fmt.Errorf("opaque carrier history requires an exact carrier snapshot")
	}
	return OpaqueCarrierHistory{snapshot: snapshot}, nil
}

func (history OpaqueCarrierHistory) SourceCoordinate() SourceCoordinate {
	return history.snapshot.SourceCoordinate()
}

func (history OpaqueCarrierHistory) CarrierRef() typedmemory.CarrierRef {
	return history.snapshot.Ref()
}

func (history OpaqueCarrierHistory) CarrierEdition() typedmemory.CarrierEdition {
	return history.snapshot.Edition()
}

func (history OpaqueCarrierHistory) CarrierDigest() typedmemory.SHA256Digest {
	return history.snapshot.Digest()
}

func (history OpaqueCarrierHistory) CarrierFormat() CarrierFormat {
	return history.snapshot.Format()
}

func (history OpaqueCarrierHistory) ExactBytes() []byte {
	return history.snapshot.ExactBytes()
}

func (history OpaqueCarrierHistory) LegacyIdentity() CarrierLegacyIdentity {
	return history.snapshot.LegacyIdentity()
}

func (history OpaqueCarrierHistory) valid() bool {
	return history.snapshot.valid()
}

// OpaqueSubjectDisposition preserves the exact dry-run classification of one
// legacy subject. SemanticSubjectRef remains an opaque legacy classifier
// coordinate; it is not promoted to typedmemory.EntityID.
type OpaqueSubjectDisposition struct {
	classification SubjectClassification
}

func newOpaqueSubjectDisposition(
	classification SubjectClassification,
) (OpaqueSubjectDisposition, error) {
	if err := validateClassification(classification); err != nil {
		return OpaqueSubjectDisposition{}, fmt.Errorf(
			"opaque subject disposition: %w",
			err,
		)
	}
	return OpaqueSubjectDisposition{classification: classification}, nil
}

func (disposition OpaqueSubjectDisposition) Subject() SemanticSubjectRef {
	return disposition.classification.Subject()
}

func (disposition OpaqueSubjectDisposition) Kind() ClassificationKind {
	return disposition.classification.Kind()
}

func (disposition OpaqueSubjectDisposition) Observations() []SubjectObservation {
	return disposition.classification.Observations()
}

func (disposition OpaqueSubjectDisposition) UnresolvedReason() (UnresolvedReason, bool) {
	unresolved, exists := disposition.classification.(Unresolved)
	if !exists {
		return UnresolvedReason{}, false
	}
	return unresolved.Reason(), true
}

func (disposition OpaqueSubjectDisposition) valid() bool {
	return validateClassification(disposition.classification) == nil
}

// ImportPlan is a deterministic, effect-free mapping from one sealed dry-run
// report to opaque historical records. It intentionally contains no
// ProjectRecordCarrierV1, EntityRecordCarrierBindingV1, MemberOf judgement,
// relation signature, BoundedContext choice, authority receipt, or Work
// record. Those require an accepted adapter manifest and an exact
// ProjectTypeEnv selected by the project head.
type ImportPlan struct {
	projectID       projectidentity.ProjectID
	classifier      ClassifierVersion
	sourceDigest    typedmemory.SHA256Digest
	reportDigest    typedmemory.SHA256Digest
	reportCanonical []byte
	carriers        []OpaqueCarrierHistory
	dispositions    []OpaqueSubjectDisposition
	canonicalBytes  []byte
}

func NewImportPlan(report DryRunReport) (ImportPlan, error) {
	verified, err := verifyDryRunReport(report)
	if err != nil {
		return ImportPlan{}, err
	}
	carriers, err := mapOpaqueCarrierHistories(verified.source.catalog.values)
	if err != nil {
		return ImportPlan{}, err
	}
	dispositions, err := mapOpaqueSubjectDispositions(verified.items)
	if err != nil {
		return ImportPlan{}, err
	}
	body := importPlanBodyDTO{
		SchemaVersion:        ImportPlanSchemaVersionV1,
		Posture:              ImportPlanPosture,
		ProjectID:            verified.projectID.String(),
		ClassifierVersion:    verified.classifier.String(),
		SourceSnapshotDigest: verified.source.Digest().String(),
		DryRunReportDigest:   verified.Digest().String(),
		DryRunReport:         json.RawMessage(verified.CanonicalBytes()),
		Carriers:             carrierHistoryDTOs(carriers),
		Dispositions:         dispositionDTOs(dispositions),
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return ImportPlan{}, fmt.Errorf("encode legacy import plan: %w", err)
	}
	return ImportPlan{
		projectID:       verified.projectID,
		classifier:      verified.classifier,
		sourceDigest:    verified.source.Digest(),
		reportDigest:    verified.Digest(),
		reportCanonical: verified.CanonicalBytes(),
		carriers:        carriers,
		dispositions:    dispositions,
		canonicalBytes:  canonical,
	}, nil
}

func (plan ImportPlan) SchemaVersion() string { return ImportPlanSchemaVersionV1 }

func (plan ImportPlan) Posture() string { return ImportPlanPosture }

func (plan ImportPlan) ProjectID() projectidentity.ProjectID { return plan.projectID }

func (plan ImportPlan) ClassifierVersion() ClassifierVersion { return plan.classifier }

func (plan ImportPlan) SourceSnapshotDigest() typedmemory.SHA256Digest {
	return plan.sourceDigest
}

func (plan ImportPlan) DryRunReportDigest() typedmemory.SHA256Digest {
	return plan.reportDigest
}

func (plan ImportPlan) DryRunReportCanonicalBytes() []byte {
	return append([]byte(nil), plan.reportCanonical...)
}

func (plan ImportPlan) CarrierHistories() []OpaqueCarrierHistory {
	return append([]OpaqueCarrierHistory(nil), plan.carriers...)
}

func (plan ImportPlan) SubjectDispositions() []OpaqueSubjectDisposition {
	return append([]OpaqueSubjectDisposition(nil), plan.dispositions...)
}

func (plan ImportPlan) CanonicalBytes() []byte {
	return append([]byte(nil), plan.canonicalBytes...)
}

func (plan ImportPlan) Digest() typedmemory.SHA256Digest {
	return digestBytes(plan.canonicalBytes)
}

// Verify rechecks the sealed plan before an outer effect shell relies on it.
// It does not admit any semantic claim or authorize a write.
func (plan ImportPlan) Verify() error {
	if !plan.valid() {
		return fmt.Errorf("legacy import plan is incomplete")
	}
	body := importPlanBodyDTO{
		SchemaVersion:        ImportPlanSchemaVersionV1,
		Posture:              ImportPlanPosture,
		ProjectID:            plan.projectID.String(),
		ClassifierVersion:    plan.classifier.String(),
		SourceSnapshotDigest: plan.sourceDigest.String(),
		DryRunReportDigest:   plan.reportDigest.String(),
		DryRunReport:         json.RawMessage(plan.reportCanonical),
		Carriers:             carrierHistoryDTOs(plan.carriers),
		Dispositions:         dispositionDTOs(plan.dispositions),
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode legacy import plan verification body: %w", err)
	}
	if !bytes.Equal(canonical, plan.canonicalBytes) {
		return fmt.Errorf("legacy import plan differs from its canonical reconstruction")
	}
	return nil
}

func (plan ImportPlan) valid() bool {
	if plan.projectID.String() == "" || !plan.classifier.valid() {
		return false
	}
	if plan.sourceDigest.String() == "" || plan.reportDigest.String() == "" {
		return false
	}
	if len(plan.reportCanonical) == 0 || len(plan.canonicalBytes) == 0 {
		return false
	}
	if len(plan.carriers) == 0 || len(plan.dispositions) == 0 {
		return false
	}
	return allOpaqueHistoryValid(plan.carriers, plan.dispositions)
}

func verifyDryRunReport(report DryRunReport) (DryRunReport, error) {
	if report.projectID.String() == "" || !report.classifier.valid() || !report.source.valid() {
		return DryRunReport{}, fmt.Errorf("legacy import plan requires a complete dry-run report")
	}
	verified, err := NewDryRunReport(
		report.projectID,
		report.classifier,
		report.source,
		report.items,
	)
	if err != nil {
		return DryRunReport{}, fmt.Errorf("verify legacy import dry-run report: %w", err)
	}
	if string(verified.CanonicalBytes()) != string(report.CanonicalBytes()) {
		return DryRunReport{}, fmt.Errorf("legacy import dry-run report differs from its canonical reconstruction")
	}
	if verified.Digest().String() != report.Digest().String() {
		return DryRunReport{}, fmt.Errorf("legacy import dry-run report digest is inconsistent")
	}
	return verified, nil
}

func mapOpaqueCarrierHistories(
	snapshots []CarrierSnapshot,
) ([]OpaqueCarrierHistory, error) {
	result := make([]OpaqueCarrierHistory, 0, len(snapshots))
	for index, snapshot := range snapshots {
		history, err := newOpaqueCarrierHistory(snapshot)
		if err != nil {
			return nil, fmt.Errorf("map carrier history %d: %w", index, err)
		}
		result = append(result, history)
	}
	return result, nil
}

func mapOpaqueSubjectDispositions(
	classifications []SubjectClassification,
) ([]OpaqueSubjectDisposition, error) {
	result := make([]OpaqueSubjectDisposition, 0, len(classifications))
	for index, classification := range classifications {
		disposition, err := newOpaqueSubjectDisposition(classification)
		if err != nil {
			return nil, fmt.Errorf("map subject disposition %d: %w", index, err)
		}
		result = append(result, disposition)
	}
	return result, nil
}

func allOpaqueHistoryValid(
	carriers []OpaqueCarrierHistory,
	dispositions []OpaqueSubjectDisposition,
) bool {
	for _, carrier := range carriers {
		if !carrier.valid() {
			return false
		}
	}
	for _, disposition := range dispositions {
		if !disposition.valid() {
			return false
		}
	}
	return true
}

type importPlanBodyDTO struct {
	SchemaVersion        string               `json:"schema_version"`
	Posture              string               `json:"posture"`
	ProjectID            string               `json:"project_id"`
	ClassifierVersion    string               `json:"classifier_version"`
	SourceSnapshotDigest string               `json:"source_snapshot_digest"`
	DryRunReportDigest   string               `json:"dry_run_report_digest"`
	DryRunReport         json.RawMessage      `json:"dry_run_report"`
	Carriers             []carrierSnapshotDTO `json:"opaque_carrier_history"`
	Dispositions         []classificationDTO  `json:"opaque_subject_dispositions"`
}

func carrierHistoryDTOs(histories []OpaqueCarrierHistory) []carrierSnapshotDTO {
	result := make([]carrierSnapshotDTO, 0, len(histories))
	for _, history := range histories {
		result = append(result, carrierSnapshotDTOOf(history.snapshot))
	}
	return result
}

func dispositionDTOs(
	dispositions []OpaqueSubjectDisposition,
) []classificationDTO {
	result := make([]classificationDTO, 0, len(dispositions))
	for _, disposition := range dispositions {
		result = append(
			result,
			classificationDTOOf(disposition.classification),
		)
	}
	return result
}
