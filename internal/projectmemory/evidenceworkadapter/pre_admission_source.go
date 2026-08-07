package evidenceworkadapter

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var (
	ErrPreAdmissionSourceStageInvalid = errors.New(
		"Evidence/Work pre-admission observable-source stage is invalid",
	)
	ErrPreAdmissionSourceUnavailable = errors.New(
		"Evidence/Work pre-admission observable source is unavailable",
	)
)

// PreAdmissionSourceStage exposes exactly the three record sources and one
// performed-occurrence source sealed by one unforgeable candidate. It grants no
// membership, admission, storage, evidence, work, or authority effect.
type PreAdmissionSourceStage struct {
	project             projectidentity.ProjectID
	observableBlobs     map[string]typedmemorystore.ObservableInputBlob
	observableOrder     []string
	classificationBlobs map[string]typedmemorystore.KindClassificationSourceBlob
	classificationOrder []string
}

var _ typedmemorystore.ObservableInputContentProvider = PreAdmissionSourceStage{}
var _ typedmemorystore.SnapshotObservableInputOverlay = PreAdmissionSourceStage{}
var _ typedmemorystore.KindClassificationSourceProvider = PreAdmissionSourceStage{}
var _ typedmemorystore.SnapshotKindClassificationSourceOverlay = PreAdmissionSourceStage{}

func SealPreAdmissionSourceStage(
	candidate ValidCandidate,
) (PreAdmissionSourceStage, error) {
	if candidate == nil {
		return PreAdmissionSourceStage{}, ErrPreAdmissionSourceStageInvalid
	}
	recordSources := candidate.RecordMembershipSources()
	if len(recordSources) != 3 {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: record source count = %d, want 3",
			ErrPreAdmissionSourceStageInvalid,
			len(recordSources),
		)
	}
	occurrence := candidate.OccurrenceMembershipSource()
	project := occurrence.ProjectID()
	observableBlobs := make(map[string]typedmemorystore.ObservableInputBlob, 4)
	for _, source := range recordSources {
		if source.ProjectID() != project {
			return PreAdmissionSourceStage{}, fmt.Errorf(
				"%w: sources cross project boundaries",
				ErrPreAdmissionSourceStageInvalid,
			)
		}
		observable := source.ObservableInput()
		verified, err := recordcarrier.VerifyRecordMembershipSourceV1(
			observable,
			source.CanonicalBytes(),
		)
		if err != nil ||
			verified.EntityID() != source.EntityID() {
			return PreAdmissionSourceStage{}, fmt.Errorf(
				"%w: record source verification failed",
				ErrPreAdmissionSourceStageInvalid,
			)
		}
		if err := addSourceBlob(
			observableBlobs,
			observable,
			source.CanonicalBytes(),
		); err != nil {
			return PreAdmissionSourceStage{}, err
		}
	}
	observable := occurrence.ObservableInput()
	verified, err := carrierfamily.VerifyMembershipSourceV1(
		observable,
		occurrence.CanonicalBytes(),
	)
	if err != nil ||
		verified.EntityID() != occurrence.EntityID() {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: occurrence source verification failed",
			ErrPreAdmissionSourceStageInvalid,
		)
	}
	if err := addSourceBlob(
		observableBlobs,
		observable,
		occurrence.CanonicalBytes(),
	); err != nil {
		return PreAdmissionSourceStage{}, err
	}
	classificationBlobs := make(
		map[string]typedmemorystore.KindClassificationSourceBlob,
		4,
	)
	recordClassificationSources := candidate.RecordClassificationSources()
	if len(recordClassificationSources) != 3 {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: record classification source count = %d, want 3",
			ErrPreAdmissionSourceStageInvalid,
			len(recordClassificationSources),
		)
	}
	for _, source := range recordClassificationSources {
		verified, err := recordcarrier.VerifyRecordClassificationSourceV1(
			source.Ref(),
			source.Digest(),
			source.CanonicalBytes(),
		)
		if err != nil ||
			verified.ProjectID() != project ||
			verified.EntityID() != source.EntityID() {
			return PreAdmissionSourceStage{}, fmt.Errorf(
				"%w: record classification source verification failed",
				ErrPreAdmissionSourceStageInvalid,
			)
		}
		if err := addClassificationSourceBlob(
			classificationBlobs,
			source.Ref(),
			source.Digest(),
			source.CanonicalBytes(),
		); err != nil {
			return PreAdmissionSourceStage{}, err
		}
	}
	occurrenceClassification := candidate.OccurrenceClassificationSource()
	verifiedOccurrenceClassification, err := carrierfamily.VerifyClassificationSourceV1(
		occurrenceClassification.Ref(),
		occurrenceClassification.Digest(),
		occurrenceClassification.CanonicalBytes(),
	)
	if err != nil ||
		verifiedOccurrenceClassification.ProjectID() != project ||
		verifiedOccurrenceClassification.EntityID() != occurrence.EntityID() {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: occurrence classification source verification failed",
			ErrPreAdmissionSourceStageInvalid,
		)
	}
	if err := addClassificationSourceBlob(
		classificationBlobs,
		occurrenceClassification.Ref(),
		occurrenceClassification.Digest(),
		occurrenceClassification.CanonicalBytes(),
	); err != nil {
		return PreAdmissionSourceStage{}, err
	}
	observableOrder := make([]string, 0, len(observableBlobs))
	for raw := range observableBlobs {
		observableOrder = append(observableOrder, raw)
	}
	sort.Strings(observableOrder)
	classificationOrder := make([]string, 0, len(classificationBlobs))
	for raw := range classificationBlobs {
		classificationOrder = append(classificationOrder, raw)
	}
	sort.Strings(classificationOrder)
	return PreAdmissionSourceStage{
		project:             project,
		observableBlobs:     observableBlobs,
		observableOrder:     observableOrder,
		classificationBlobs: classificationBlobs,
		classificationOrder: classificationOrder,
	}, nil
}

func addSourceBlob(
	blobs map[string]typedmemorystore.ObservableInputBlob,
	observable typedmemory.MemberOfObservableInput,
	canonical []byte,
) error {
	key := observable.Reference().String()
	if _, exists := blobs[key]; exists {
		return fmt.Errorf(
			"%w: duplicate source reference %s",
			ErrPreAdmissionSourceStageInvalid,
			key,
		)
	}
	blob, err := typedmemorystore.NewObservableInputBlob(
		observable.Reference(),
		observable.Digest(),
		canonical,
	)
	if err != nil {
		return fmt.Errorf(
			"%w: immutable source blob: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	blobs[key] = blob
	return nil
}

func addClassificationSourceBlob(
	blobs map[string]typedmemorystore.KindClassificationSourceBlob,
	reference typedmemory.CarrierRef,
	digest typedmemory.SHA256Digest,
	canonical []byte,
) error {
	key := reference.String()
	if _, exists := blobs[key]; exists {
		return fmt.Errorf(
			"%w: duplicate classification source reference %s",
			ErrPreAdmissionSourceStageInvalid,
			key,
		)
	}
	blob, err := typedmemorystore.NewKindClassificationSourceBlob(
		reference,
		digest,
		canonical,
	)
	if err != nil {
		return fmt.Errorf(
			"%w: immutable classification source blob: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	blobs[key] = blob
	return nil
}

func (stage PreAdmissionSourceStage) LoadObservableInput(
	ctx context.Context,
	project projectidentity.ProjectID,
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	if ctx == nil {
		return typedmemorystore.ObservableInputBlob{}, fmt.Errorf(
			"Evidence/Work pre-admission observable source requires context",
		)
	}
	if err := ctx.Err(); err != nil {
		return typedmemorystore.ObservableInputBlob{}, err
	}
	if project != stage.project {
		return typedmemorystore.ObservableInputBlob{},
			ErrPreAdmissionSourceUnavailable
	}
	blob, found := stage.observableBlobs[reference.String()]
	if !found || blob.Digest() != digest {
		return typedmemorystore.ObservableInputBlob{},
			ErrPreAdmissionSourceUnavailable
	}
	return typedmemorystore.NewObservableInputBlob(
		blob.Reference(),
		blob.Digest(),
		blob.Bytes(),
	)
}

func (stage PreAdmissionSourceStage) LoadSnapshotObservableInputs(
	ctx context.Context,
	project projectidentity.ProjectID,
) ([]typedmemorystore.ObservableInputBlob, error) {
	if ctx == nil {
		return nil, fmt.Errorf(
			"Evidence/Work pre-admission observable source requires context",
		)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if project != stage.project || len(stage.observableOrder) != 4 {
		return nil, ErrPreAdmissionSourceUnavailable
	}
	result := make(
		[]typedmemorystore.ObservableInputBlob,
		0,
		len(stage.observableOrder),
	)
	for _, raw := range stage.observableOrder {
		blob := stage.observableBlobs[raw]
		exact, err := typedmemorystore.NewObservableInputBlob(
			blob.Reference(),
			blob.Digest(),
			blob.Bytes(),
		)
		if err != nil {
			return nil, err
		}
		result = append(result, exact)
	}
	return result, nil
}

func (stage PreAdmissionSourceStage) LoadKindClassificationSource(
	ctx context.Context,
	project projectidentity.ProjectID,
	reference typedmemory.CarrierRef,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.KindClassificationSourceBlob, error) {
	if ctx == nil {
		return typedmemorystore.KindClassificationSourceBlob{}, fmt.Errorf(
			"Evidence/Work pre-admission classification source requires context",
		)
	}
	if err := ctx.Err(); err != nil {
		return typedmemorystore.KindClassificationSourceBlob{}, err
	}
	if project != stage.project {
		return typedmemorystore.KindClassificationSourceBlob{},
			ErrPreAdmissionSourceUnavailable
	}
	blob, found := stage.classificationBlobs[reference.String()]
	if !found || blob.Digest() != digest {
		return typedmemorystore.KindClassificationSourceBlob{},
			ErrPreAdmissionSourceUnavailable
	}
	return typedmemorystore.NewKindClassificationSourceBlob(
		blob.Reference(),
		blob.Digest(),
		blob.Bytes(),
	)
}

func (stage PreAdmissionSourceStage) LoadSnapshotKindClassificationSources(
	ctx context.Context,
	project projectidentity.ProjectID,
) ([]typedmemorystore.KindClassificationSourceBlob, error) {
	if ctx == nil {
		return nil, fmt.Errorf(
			"Evidence/Work pre-admission classification source requires context",
		)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if project != stage.project || len(stage.classificationOrder) != 4 {
		return nil, ErrPreAdmissionSourceUnavailable
	}
	result := make(
		[]typedmemorystore.KindClassificationSourceBlob,
		0,
		len(stage.classificationOrder),
	)
	for _, raw := range stage.classificationOrder {
		blob := stage.classificationBlobs[raw]
		exact, err := typedmemorystore.NewKindClassificationSourceBlob(
			blob.Reference(),
			blob.Digest(),
			blob.Bytes(),
		)
		if err != nil {
			return nil, err
		}
		result = append(result, exact)
	}
	return result, nil
}
