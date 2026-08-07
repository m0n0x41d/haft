package legacyimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const DryRunReportSchemaVersionV1 = "haft.legacy-import.dry-run/v1"

const DryRunAuthority = "read_only_no_write_no_admission"

var (
	ErrClassificationCollision        = errors.New("legacy import subject has more than one classification")
	ErrObservationUnclassified        = errors.New("legacy import observation subject is unclassified")
	ErrClassificationObservationDrift = errors.New("legacy import classification does not cover the exact source observations")
)

type DryRunSummary struct {
	carrierOnly   uint64
	legacyUnbound uint64
	unresolved    uint64
}

func (summary DryRunSummary) CarrierOnly() uint64 { return summary.carrierOnly }

func (summary DryRunSummary) LegacyUnbound() uint64 { return summary.legacyUnbound }

func (summary DryRunSummary) Unresolved() uint64 { return summary.unresolved }

func (summary DryRunSummary) Total() uint64 {
	return summary.carrierOnly + summary.legacyUnbound + summary.unresolved
}

type DryRunReport struct {
	projectID      projectidentity.ProjectID
	classifier     ClassifierVersion
	source         LegacySourceSnapshot
	items          []SubjectClassification
	summary        DryRunSummary
	canonicalBytes []byte
}

func NewDryRunReport(
	projectID projectidentity.ProjectID,
	classifier ClassifierVersion,
	source LegacySourceSnapshot,
	items []SubjectClassification,
) (DryRunReport, error) {
	if projectID.String() == "" {
		return DryRunReport{}, fmt.Errorf("dry-run report project ID is required")
	}
	if !classifier.valid() {
		return DryRunReport{}, fmt.Errorf("dry-run report classifier version is required")
	}
	if !source.valid() {
		return DryRunReport{}, fmt.Errorf("dry-run report source snapshot is required")
	}
	owned, err := normalizeClassifications(items)
	if err != nil {
		return DryRunReport{}, err
	}
	if err := requireExactObservationPartition(source.observations, owned); err != nil {
		return DryRunReport{}, err
	}
	summary := summarizeClassifications(owned)
	body := dryRunReportBodyDTO{
		SchemaVersion:        DryRunReportSchemaVersionV1,
		Authority:            DryRunAuthority,
		ProjectID:            projectID.String(),
		ClassifierVersion:    classifier.String(),
		SourceSnapshotDigest: source.Digest().String(),
		CarrierCatalog:       carrierCatalogDTOs(source.catalog),
		Items:                classificationDTOs(owned),
		Summary:              dryRunSummaryDTOOf(summary),
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return DryRunReport{}, fmt.Errorf("encode legacy import dry-run report: %w", err)
	}
	return DryRunReport{
		projectID:      projectID,
		classifier:     classifier,
		source:         source,
		items:          owned,
		summary:        summary,
		canonicalBytes: canonical,
	}, nil
}

func (report DryRunReport) SchemaVersion() string { return DryRunReportSchemaVersionV1 }

func (report DryRunReport) Authority() string { return DryRunAuthority }

func (report DryRunReport) ProjectID() projectidentity.ProjectID { return report.projectID }

func (report DryRunReport) ClassifierVersion() ClassifierVersion { return report.classifier }

func (report DryRunReport) SourceSnapshotDigest() typedmemory.SHA256Digest {
	return report.source.Digest()
}

func (report DryRunReport) CarrierCatalog() CarrierCatalog { return report.source.catalog }

func (report DryRunReport) Items() []SubjectClassification {
	return append([]SubjectClassification(nil), report.items...)
}

func (report DryRunReport) Summary() DryRunSummary { return report.summary }

func (report DryRunReport) CanonicalBytes() []byte {
	return append([]byte(nil), report.canonicalBytes...)
}

func (report DryRunReport) Digest() typedmemory.SHA256Digest {
	return digestBytes(report.canonicalBytes)
}

func (report DryRunReport) MarshalJSON() ([]byte, error) {
	var body dryRunReportBodyDTO
	if err := json.Unmarshal(report.canonicalBytes, &body); err != nil {
		return nil, fmt.Errorf("decode canonical dry-run report: %w", err)
	}
	return json.Marshal(dryRunReportEnvelopeDTO{
		dryRunReportBodyDTO: body,
		ReportDigest:        report.Digest().String(),
	})
}

func normalizeClassifications(
	items []SubjectClassification,
) ([]SubjectClassification, error) {
	owned := append([]SubjectClassification(nil), items...)
	for index, item := range owned {
		if err := validateClassification(item); err != nil {
			return nil, fmt.Errorf("classification %d: %w", index, err)
		}
	}
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].canonicalBytes(), owned[right].canonicalBytes()) < 0
	})
	seen := map[string]struct{}{}
	for _, item := range owned {
		subject := item.Subject().String()
		if _, exists := seen[subject]; exists {
			return nil, fmt.Errorf("%w: %s", ErrClassificationCollision, subject)
		}
		seen[subject] = struct{}{}
	}
	return owned, nil
}

func requireExactObservationPartition(
	observations ObservationSet,
	classifications []SubjectClassification,
) error {
	sourceBySubject := observationKeysBySubject(observations.values)
	classifiedBySubject := map[string]map[string]struct{}{}
	for _, classification := range classifications {
		subject := classification.Subject().String()
		classifiedBySubject[subject] = observationKeys(classification.Observations())
	}
	for subject, sourceKeys := range sourceBySubject {
		classifiedKeys, exists := classifiedBySubject[subject]
		if !exists {
			return fmt.Errorf("%w: %s", ErrObservationUnclassified, subject)
		}
		if !sameStringSet(sourceKeys, classifiedKeys) {
			return fmt.Errorf("%w: %s", ErrClassificationObservationDrift, subject)
		}
	}
	for subject := range classifiedBySubject {
		if _, exists := sourceBySubject[subject]; exists {
			continue
		}
		return fmt.Errorf("%w: %s has no source observation", ErrClassificationObservationDrift, subject)
	}
	return nil
}

func observationKeysBySubject(values []SubjectObservation) map[string]map[string]struct{} {
	result := map[string]map[string]struct{}{}
	for _, observation := range values {
		subject := observation.Subject().String()
		if result[subject] == nil {
			result[subject] = map[string]struct{}{}
		}
		result[subject][string(observation.canonicalBytes())] = struct{}{}
	}
	return result
}

func observationKeys(values []SubjectObservation) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, observation := range values {
		result[string(observation.canonicalBytes())] = struct{}{}
	}
	return result
}

func sameStringSet(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if _, exists := right[key]; !exists {
			return false
		}
	}
	return true
}

func summarizeClassifications(items []SubjectClassification) DryRunSummary {
	summary := DryRunSummary{}
	for _, item := range items {
		switch item.Kind() {
		case ClassificationCarrierOnly:
			summary.carrierOnly++
		case ClassificationLegacyUnbound:
			summary.legacyUnbound++
		case ClassificationUnresolved:
			summary.unresolved++
		}
	}
	return summary
}

type dryRunSummaryDTO struct {
	CarrierOnly   uint64 `json:"carrier_only"`
	LegacyUnbound uint64 `json:"legacy_unbound"`
	Unresolved    uint64 `json:"unresolved"`
	Total         uint64 `json:"total"`
}

func dryRunSummaryDTOOf(summary DryRunSummary) dryRunSummaryDTO {
	return dryRunSummaryDTO{
		CarrierOnly:   summary.CarrierOnly(),
		LegacyUnbound: summary.LegacyUnbound(),
		Unresolved:    summary.Unresolved(),
		Total:         summary.Total(),
	}
}

type dryRunReportBodyDTO struct {
	SchemaVersion        string               `json:"schema_version"`
	Authority            string               `json:"authority"`
	ProjectID            string               `json:"project_id"`
	ClassifierVersion    string               `json:"classifier_version"`
	SourceSnapshotDigest string               `json:"source_snapshot_digest"`
	CarrierCatalog       []carrierSnapshotDTO `json:"carrier_catalog"`
	Items                []classificationDTO  `json:"items"`
	Summary              dryRunSummaryDTO     `json:"summary"`
}

type dryRunReportEnvelopeDTO struct {
	dryRunReportBodyDTO
	ReportDigest string `json:"report_digest"`
}

func carrierCatalogDTOs(catalog CarrierCatalog) []carrierSnapshotDTO {
	result := make([]carrierSnapshotDTO, 0, len(catalog.values))
	for _, snapshot := range catalog.values {
		result = append(result, carrierSnapshotDTOOf(snapshot))
	}
	return result
}

func classificationDTOs(items []SubjectClassification) []classificationDTO {
	result := make([]classificationDTO, 0, len(items))
	for _, item := range items {
		result = append(result, classificationDTOOf(item))
	}
	return result
}
