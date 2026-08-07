package codeanchoradapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var (
	ErrPreAdmissionSourceStageInvalid = errors.New(
		"CodeAnchor pre-admission observable-source stage is invalid",
	)
	ErrPreAdmissionSourceUnavailable = errors.New(
		"CodeAnchor pre-admission observable source is unavailable for the requested coordinate",
	)
	ErrPreAdmissionFallbackProviderMissing = errors.New(
		"CodeAnchor pre-admission observable-source fallback provider is missing",
	)
)

// PreAdmissionSourceStage supplies the exact CodeAnchor family source for
// read-only validation and transaction-time revalidation. It neither asserts
// membership nor makes the source durable.
type PreAdmissionSourceStage struct {
	project        projectledger.ProjectID
	entity         typedmemory.EntityID
	context        typedmemory.BoundedContextRef
	observable     typedmemory.MemberOfObservableInput
	observableBlob typedmemorystore.ObservableInputBlob
	classification typedmemorystore.KindClassificationSourceBlob
}

var _ typedmemorystore.ObservableInputContentProvider = PreAdmissionSourceStage{}
var _ typedmemorystore.SnapshotObservableInputOverlay = PreAdmissionSourceStage{}
var _ typedmemorystore.KindClassificationSourceProvider = PreAdmissionSourceStage{}
var _ typedmemorystore.SnapshotKindClassificationSourceOverlay = PreAdmissionSourceStage{}

type PreAdmissionObservableInputProvider struct {
	stage    PreAdmissionSourceStage
	fallback typedmemorystore.ObservableInputContentProvider
}

var _ typedmemorystore.ObservableInputContentProvider = PreAdmissionObservableInputProvider{}
var _ typedmemorystore.SnapshotObservableInputOverlay = PreAdmissionObservableInputProvider{}

func SealPreAdmissionSourceStage(
	candidate ValidCandidate,
) (PreAdmissionSourceStage, error) {
	if candidate == nil {
		return PreAdmissionSourceStage{}, ErrPreAdmissionSourceStageInvalid
	}
	manifest := candidate.MappingManifestRef()
	if err := manifest.Verify(); err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: mapping manifest: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	adapter := candidate.AdapterVersion()
	if err := adapter.Verify(); err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: adapter version: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	source := candidate.MembershipSource()
	observable := source.ObservableInput()
	verified, err := carrierfamily.VerifyMembershipSourceV1(
		observable,
		source.CanonicalBytes(),
	)
	if err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: carrier-family source: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	if err := requireCandidateSourceCorrelation(
		candidate,
		verified,
	); err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	observableBlob, err := typedmemorystore.NewObservableInputBlob(
		observable.Reference(),
		observable.Digest(),
		verified.CanonicalBytes(),
	)
	if err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: immutable source blob: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	classificationSource := candidate.ClassificationSource()
	verifiedClassification, err := carrierfamily.VerifyClassificationSourceV1(
		classificationSource.Ref(),
		classificationSource.Digest(),
		classificationSource.CanonicalBytes(),
	)
	if err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: carrier-family classification source: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	if err := requireCandidateClassificationSourceCorrelation(
		candidate,
		verifiedClassification,
	); err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	classificationBlob, err := typedmemorystore.NewKindClassificationSourceBlob(
		verifiedClassification.Ref(),
		verifiedClassification.Digest(),
		verifiedClassification.CanonicalBytes(),
	)
	if err != nil {
		return PreAdmissionSourceStage{}, fmt.Errorf(
			"%w: immutable classification source blob: %v",
			ErrPreAdmissionSourceStageInvalid,
			err,
		)
	}
	return PreAdmissionSourceStage{
		project:        verified.ProjectID(),
		entity:         verified.EntityID(),
		context:        verified.BoundedContext(),
		observable:     observable,
		observableBlob: observableBlob,
		classification: classificationBlob,
	}, nil
}

func NewPreAdmissionObservableInputProvider(
	stage PreAdmissionSourceStage,
	fallback typedmemorystore.ObservableInputContentProvider,
) (PreAdmissionObservableInputProvider, error) {
	if !stage.valid() {
		return PreAdmissionObservableInputProvider{},
			ErrPreAdmissionSourceStageInvalid
	}
	if !observableInputContentProviderPresent(fallback) {
		return PreAdmissionObservableInputProvider{},
			ErrPreAdmissionFallbackProviderMissing
	}
	return PreAdmissionObservableInputProvider{
		stage:    stage,
		fallback: fallback,
	}, nil
}

func (stage PreAdmissionSourceStage) ProjectID() projectledger.ProjectID {
	return stage.project
}

func (stage PreAdmissionSourceStage) EntityID() typedmemory.EntityID {
	return stage.entity
}

func (stage PreAdmissionSourceStage) BoundedContext() typedmemory.BoundedContextRef {
	return stage.context
}

func (stage PreAdmissionSourceStage) ObservableInput() typedmemory.MemberOfObservableInput {
	return stage.observable
}

func (stage PreAdmissionSourceStage) LoadObservableInput(
	ctx context.Context,
	project projectledger.ProjectID,
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	if err := preAdmissionSourceContextError(ctx); err != nil {
		return typedmemorystore.ObservableInputBlob{}, err
	}
	if !stage.valid() ||
		project != stage.project ||
		reference != stage.observable.Reference() ||
		digest != stage.observable.Digest() {
		return typedmemorystore.ObservableInputBlob{}, fmt.Errorf(
			"%w: project=%s reference=%s digest=%s",
			ErrPreAdmissionSourceUnavailable,
			project.String(),
			reference.String(),
			digest.String(),
		)
	}
	return typedmemorystore.NewObservableInputBlob(
		stage.observableBlob.Reference(),
		stage.observableBlob.Digest(),
		stage.observableBlob.Bytes(),
	)
}

func (stage PreAdmissionSourceStage) LoadSnapshotObservableInputs(
	ctx context.Context,
	project projectledger.ProjectID,
) ([]typedmemorystore.ObservableInputBlob, error) {
	if err := preAdmissionSourceContextError(ctx); err != nil {
		return nil, err
	}
	if !stage.valid() || project != stage.project {
		return nil, fmt.Errorf(
			"%w: project=%s",
			ErrPreAdmissionSourceUnavailable,
			project.String(),
		)
	}
	blob, err := stage.LoadObservableInput(
		ctx,
		project,
		stage.observable.Reference(),
		stage.observable.Digest(),
	)
	if err != nil {
		return nil, err
	}
	return []typedmemorystore.ObservableInputBlob{blob}, nil
}

func (provider PreAdmissionObservableInputProvider) LoadObservableInput(
	ctx context.Context,
	project projectledger.ProjectID,
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	if err := preAdmissionSourceContextError(ctx); err != nil {
		return typedmemorystore.ObservableInputBlob{}, err
	}
	if !provider.stage.valid() ||
		!observableInputContentProviderPresent(provider.fallback) {
		return typedmemorystore.ObservableInputBlob{},
			ErrPreAdmissionSourceStageInvalid
	}
	if reference == provider.stage.observable.Reference() {
		return provider.stage.LoadObservableInput(
			ctx,
			project,
			reference,
			digest,
		)
	}
	blob, err := provider.fallback.LoadObservableInput(
		ctx,
		project,
		reference,
		digest,
	)
	if err != nil {
		return typedmemorystore.ObservableInputBlob{}, err
	}
	verified, err := typedmemorystore.NewObservableInputBlob(
		blob.Reference(),
		blob.Digest(),
		blob.Bytes(),
	)
	if err != nil {
		return typedmemorystore.ObservableInputBlob{}, err
	}
	if verified.Reference() != reference || verified.Digest() != digest {
		return typedmemorystore.ObservableInputBlob{},
			ErrPreAdmissionSourceUnavailable
	}
	return verified, nil
}

func (provider PreAdmissionObservableInputProvider) LoadSnapshotObservableInputs(
	ctx context.Context,
	project projectledger.ProjectID,
) ([]typedmemorystore.ObservableInputBlob, error) {
	return provider.stage.LoadSnapshotObservableInputs(ctx, project)
}

func (stage PreAdmissionSourceStage) LoadKindClassificationSource(
	ctx context.Context,
	project projectledger.ProjectID,
	reference typedmemory.CarrierRef,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.KindClassificationSourceBlob, error) {
	if err := preAdmissionSourceContextError(ctx); err != nil {
		return typedmemorystore.KindClassificationSourceBlob{}, err
	}
	if !stage.valid() ||
		project != stage.project ||
		reference != stage.classification.Reference() ||
		digest != stage.classification.Digest() {
		return typedmemorystore.KindClassificationSourceBlob{}, fmt.Errorf(
			"%w: project=%s classification_reference=%s digest=%s",
			ErrPreAdmissionSourceUnavailable,
			project.String(),
			reference.String(),
			digest.String(),
		)
	}
	return typedmemorystore.NewKindClassificationSourceBlob(
		stage.classification.Reference(),
		stage.classification.Digest(),
		stage.classification.Bytes(),
	)
}

func (stage PreAdmissionSourceStage) LoadSnapshotKindClassificationSources(
	ctx context.Context,
	project projectledger.ProjectID,
) ([]typedmemorystore.KindClassificationSourceBlob, error) {
	if err := preAdmissionSourceContextError(ctx); err != nil {
		return nil, err
	}
	blob, err := stage.LoadKindClassificationSource(
		ctx,
		project,
		stage.classification.Reference(),
		stage.classification.Digest(),
	)
	if err != nil {
		return nil, err
	}
	return []typedmemorystore.KindClassificationSourceBlob{blob}, nil
}

func (stage PreAdmissionSourceStage) valid() bool {
	if stage.project.String() == "" ||
		stage.entity.String() == "" ||
		stage.context.String() == "" ||
		stage.observable.Reference().String() == "" ||
		stage.observable.Digest().String() == "" {
		return false
	}
	verifiedObservable, err := typedmemorystore.NewObservableInputBlob(
		stage.observableBlob.Reference(),
		stage.observableBlob.Digest(),
		stage.observableBlob.Bytes(),
	)
	if err != nil {
		return false
	}
	verifiedClassification, err := typedmemorystore.NewKindClassificationSourceBlob(
		stage.classification.Reference(),
		stage.classification.Digest(),
		stage.classification.Bytes(),
	)
	if err != nil {
		return false
	}
	return verifiedObservable.Reference() == stage.observable.Reference() &&
		verifiedObservable.Digest() == stage.observable.Digest() &&
		verifiedClassification.Reference() == stage.classification.Reference() &&
		verifiedClassification.Digest() == stage.classification.Digest()
}

func requireCandidateSourceCorrelation(
	candidate ValidCandidate,
	source carrierfamily.MembershipSourceV1,
) error {
	carrier := candidate.Carrier()
	binding := candidate.CarrierBinding()
	checks := []struct {
		name  string
		match bool
	}{
		{
			name: "candidate/source carrier",
			match: bytes.Equal(
				carrier.CanonicalBytes(),
				source.Carrier().CanonicalBytes(),
			),
		},
		{
			name: "candidate/source binding",
			match: bytes.Equal(
				binding.CanonicalBytes(),
				source.Binding().CanonicalBytes(),
			),
		},
		{
			name:  "candidate/source project",
			match: binding.ProjectID() == source.ProjectID(),
		},
		{
			name:  "candidate/source entity",
			match: carrier.EntityID() == source.EntityID(),
		},
		{
			name:  "candidate/source context",
			match: carrier.BoundedContext() == source.BoundedContext(),
		},
		{
			name: "candidate mapping manifest",
			match: binding.MappingManifestRef() ==
				candidate.MappingManifestRef(),
		},
		{
			name: "candidate adapter version",
			match: binding.AdapterVersion() ==
				candidate.AdapterVersion(),
		},
	}
	for _, check := range checks {
		if !check.match {
			return fmt.Errorf("%s mismatch", check.name)
		}
	}
	return requireCandidateChangeCorrelation(candidate, source)
}

func requireCandidateClassificationSourceCorrelation(
	candidate ValidCandidate,
	source carrierfamily.ClassificationSourceV1,
) error {
	carrier := candidate.Carrier()
	binding := candidate.CarrierBinding()
	checks := []struct {
		name  string
		match bool
	}{
		{
			name: "candidate/classification source carrier",
			match: bytes.Equal(
				carrier.CanonicalBytes(),
				source.Carrier().CanonicalBytes(),
			),
		},
		{
			name: "candidate/classification source binding",
			match: bytes.Equal(
				binding.CanonicalBytes(),
				source.Binding().CanonicalBytes(),
			),
		},
		{
			name:  "candidate/classification source project",
			match: binding.ProjectID() == source.ProjectID(),
		},
		{
			name:  "candidate/classification source entity",
			match: carrier.EntityID() == source.EntityID(),
		},
		{
			name:  "candidate/classification source context",
			match: carrier.BoundedContext() == source.BoundedContext(),
		},
		{
			name: "candidate classification mapping manifest",
			match: binding.MappingManifestRef() ==
				candidate.MappingManifestRef(),
		},
		{
			name: "candidate classification adapter version",
			match: binding.AdapterVersion() ==
				candidate.AdapterVersion(),
		},
	}
	for _, check := range checks {
		if !check.match {
			return fmt.Errorf("%s mismatch", check.name)
		}
	}
	return requireCandidateClassificationChangeCorrelation(candidate, source)
}

func requireCandidateClassificationChangeCorrelation(
	candidate ValidCandidate,
	source carrierfamily.ClassificationSourceV1,
) error {
	changes := candidate.ChangeSet().Changes()
	signatures := candidate.RelationDeclarationFragmentIDs()
	if len(changes) < 3 || len(signatures) != len(changes)-1 {
		return fmt.Errorf(
			"CodeAnchor candidate requires one declaration, one definition, and at least one semantic link",
		)
	}
	declaration, ok := changes[0].(typedmemory.DeclareEntity)
	if !ok ||
		declaration.Entity() != source.EntityID() ||
		declaration.Context() != source.BoundedContext() {
		return fmt.Errorf(
			"CodeAnchor declaration and classification source differ",
		)
	}
	for index, change := range changes[1:] {
		assertRelation, ok := change.(typedmemory.AssertRelation)
		if !ok {
			return fmt.Errorf(
				"CodeAnchor candidate change %d is not AssertRelation",
				index+1,
			)
		}
		relation := assertRelation.Assertion()
		if relation.Modality().Kind() != typedmemory.AssertionModalityAffirmsObtaining {
			return fmt.Errorf(
				"CodeAnchor relation %d does not affirm obtaining",
				index,
			)
		}
		if relation.Signature().ID() != signatures[index] ||
			relation.Context() != source.BoundedContext() {
			return fmt.Errorf(
				"CodeAnchor relation %d and classification source differ",
				index,
			)
		}
	}
	if signatures[0].String() != codeAnchorDefinitionSignatureID {
		return fmt.Errorf(
			"CodeAnchor first relation is not its exact definition",
		)
	}
	return nil
}

func requireCandidateChangeCorrelation(
	candidate ValidCandidate,
	source carrierfamily.MembershipSourceV1,
) error {
	changes := candidate.ChangeSet().Changes()
	signatures := candidate.RelationDeclarationFragmentIDs()
	if len(changes) < 3 || len(signatures) != len(changes)-1 {
		return fmt.Errorf(
			"CodeAnchor candidate requires one declaration, one definition, and at least one semantic link",
		)
	}
	declaration, ok := changes[0].(typedmemory.DeclareEntity)
	if !ok ||
		declaration.Entity() != source.EntityID() ||
		declaration.Context() != source.BoundedContext() {
		return fmt.Errorf(
			"CodeAnchor declaration and membership source differ",
		)
	}
	for index, change := range changes[1:] {
		assertRelation, ok := change.(typedmemory.AssertRelation)
		if !ok {
			return fmt.Errorf(
				"CodeAnchor candidate change %d is not AssertRelation",
				index+1,
			)
		}
		relation := assertRelation.Assertion()
		if relation.Modality().Kind() != typedmemory.AssertionModalityAffirmsObtaining {
			return fmt.Errorf(
				"CodeAnchor relation %d does not affirm obtaining",
				index,
			)
		}
		if relation.Signature().ID() != signatures[index] ||
			relation.Context() != source.BoundedContext() {
			return fmt.Errorf(
				"CodeAnchor relation %d and membership source differ",
				index,
			)
		}
	}
	if signatures[0].String() != codeAnchorDefinitionSignatureID {
		return fmt.Errorf(
			"CodeAnchor first relation is not its exact definition",
		)
	}
	return nil
}

func preAdmissionSourceContextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf(
			"CodeAnchor pre-admission observable source requires context",
		)
	}
	return ctx.Err()
}

func observableInputContentProviderPresent(
	provider typedmemorystore.ObservableInputContentProvider,
) bool {
	if provider == nil {
		return false
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}
