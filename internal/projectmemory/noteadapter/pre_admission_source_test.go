package noteadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type recordingObservableInputProvider struct {
	blob  typedmemorystore.ObservableInputBlob
	loads int
}

func TestCurrentClassificationSourceStageIsDisjointFromHistoricalObservable(t *testing.T) {
	t.Parallel()

	fixture := newAdapterFixture(t)
	currentRuntime := mustValueTest(
		NewExactRuntimeBasisBuilder(fixture.runtime.ProjectID()).
			SetGraphRevision(fixture.runtime.GraphRevision()).
			SetEnvironment(fixture.runtime.Environment()).
			SetCodecs(fixture.runtime.Codecs()).
			SetSelectedRuntimeCoordinates(
				fixture.runtime.SelectedRuntimeBasis(),
				fixture.runtime.RegistryCoordinate(),
			).
			SetCurrentKindClassification().
			Build(),
	)
	candidate := validCandidate(
		t,
		Adapt(fixture.draft, currentRuntime, fixture.concern),
	)
	stage, err := SealPreAdmissionSourceStage(candidate)
	if err != nil {
		t.Fatalf("SealPreAdmissionSourceStage() error = %v", err)
	}
	source := candidate.ClassificationSource()
	blob, err := stage.LoadKindClassificationSource(
		context.Background(),
		stage.ProjectID(),
		source.Ref(),
		source.Digest(),
	)
	if err != nil {
		t.Fatalf("LoadKindClassificationSource() error = %v", err)
	}
	if blob.Reference() != source.Ref() ||
		blob.Digest() != source.Digest() ||
		!bytes.Equal(blob.Bytes(), source.CanonicalBytes()) {
		t.Fatal("current classification stage changed exact source identity")
	}
	if source.Ref().String() == candidate.MembershipSource().ObservableInput().Reference().String() {
		t.Fatal("current classification and historical MemberOf sources share a reference domain")
	}
}

func (provider *recordingObservableInputProvider) LoadObservableInput(
	_ context.Context,
	_ projectledger.ProjectID,
	_ typedmemory.ObservableInputRef,
	_ typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	provider.loads++
	return provider.blob, nil
}

func TestPreAdmissionSourceStageServesOneExactImmutableCoordinate(
	t *testing.T,
) {
	fixture := newAdapterFixture(t)
	candidate := validCandidate(
		t,
		Adapt(fixture.draft, fixture.runtime, fixture.concern),
	)
	stage, err := SealPreAdmissionSourceStage(candidate)
	if err != nil {
		t.Fatalf("SealPreAdmissionSourceStage() error = %v", err)
	}
	observable := candidate.MembershipSource().ObservableInput()
	blob, err := stage.LoadObservableInput(
		context.Background(),
		stage.ProjectID(),
		observable.Reference(),
		observable.Digest(),
	)
	if err != nil {
		t.Fatalf("LoadObservableInput() error = %v", err)
	}
	if blob.Reference() != observable.Reference() ||
		blob.Digest() != observable.Digest() ||
		!bytes.Equal(blob.Bytes(), candidate.MembershipSource().CanonicalBytes()) {
		t.Fatal("staged blob lost the exact record-membership source coordinate")
	}

	mutated := blob.Bytes()
	mutated[0] ^= 0xff
	reloaded, err := stage.LoadObservableInput(
		context.Background(),
		stage.ProjectID(),
		observable.Reference(),
		observable.Digest(),
	)
	if err != nil {
		t.Fatalf("reload staged source: %v", err)
	}
	if bytes.Equal(mutated, reloaded.Bytes()) {
		t.Fatal("caller mutation changed staged observable bytes")
	}

	overlay, err := stage.LoadSnapshotObservableInputs(
		context.Background(),
		stage.ProjectID(),
	)
	if err != nil {
		t.Fatalf("LoadSnapshotObservableInputs() error = %v", err)
	}
	if len(overlay) != 1 ||
		overlay[0].Reference() != observable.Reference() ||
		overlay[0].Digest() != observable.Digest() {
		t.Fatalf("snapshot overlay = %#v, want one exact staged source", overlay)
	}
}

func TestPreAdmissionSourceStageFailsClosedAcrossCoordinatesAndCancellation(
	t *testing.T,
) {
	fixture := newAdapterFixture(t)
	candidate := validCandidate(
		t,
		Adapt(fixture.draft, fixture.runtime, fixture.concern),
	)
	stage, err := SealPreAdmissionSourceStage(candidate)
	if err != nil {
		t.Fatalf("SealPreAdmissionSourceStage() error = %v", err)
	}
	observable := stage.ObservableInput()
	foreignProject := mustValueTest(
		projectidentity.ParseProjectID("qnt_1234abcd"),
	)
	foreignReference := mustValueTest(
		typedmemory.NewObservableInputRef("record-membership-source:foreign"),
	)
	foreignDigest := mustDigestTest(t, 'f')

	requests := []struct {
		name      string
		project   projectidentity.ProjectID
		reference typedmemory.ObservableInputRef
		digest    typedmemory.SHA256Digest
	}{
		{
			name:      "project",
			project:   foreignProject,
			reference: observable.Reference(),
			digest:    observable.Digest(),
		},
		{
			name:      "reference",
			project:   stage.ProjectID(),
			reference: foreignReference,
			digest:    observable.Digest(),
		},
		{
			name:      "digest",
			project:   stage.ProjectID(),
			reference: observable.Reference(),
			digest:    foreignDigest,
		},
	}
	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			_, loadErr := stage.LoadObservableInput(
				context.Background(),
				request.project,
				request.reference,
				request.digest,
			)
			if !errors.Is(loadErr, ErrPreAdmissionSourceUnavailable) {
				t.Fatalf("LoadObservableInput() error = %v, want unavailable", loadErr)
			}
		})
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = stage.LoadObservableInput(
		canceled,
		stage.ProjectID(),
		observable.Reference(),
		observable.Digest(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled LoadObservableInput() error = %v", err)
	}
	_, err = stage.LoadSnapshotObservableInputs(context.Background(), foreignProject)
	if !errors.Is(err, ErrPreAdmissionSourceUnavailable) {
		t.Fatalf("foreign snapshot overlay error = %v", err)
	}
}

func TestPreAdmissionSourceStageRejectsUnsealedAndZeroStage(
	t *testing.T,
) {
	if _, err := SealPreAdmissionSourceStage(nil); !errors.Is(
		err,
		ErrPreAdmissionSourceStageInvalid,
	) {
		t.Fatalf("nil candidate error = %v", err)
	}

	fixture := newAdapterFixture(t)
	var zero PreAdmissionSourceStage
	_, err := zero.LoadSnapshotObservableInputs(
		context.Background(),
		fixture.draft.ProjectID(),
	)
	if !errors.Is(err, ErrPreAdmissionSourceUnavailable) {
		t.Fatalf("zero stage error = %v", err)
	}
}

func TestPreAdmissionObservableInputProviderKeepsStageAuthoritativeAndDelegatesOthers(
	t *testing.T,
) {
	fixture := newAdapterFixture(t)
	candidate := validCandidate(
		t,
		Adapt(fixture.draft, fixture.runtime, fixture.concern),
	)
	stage, err := SealPreAdmissionSourceStage(candidate)
	if err != nil {
		t.Fatalf("SealPreAdmissionSourceStage() error = %v", err)
	}
	fallbackReference := mustValueTest(
		typedmemory.NewObservableInputRef("observable:entity-set-enumeration"),
	)
	fallbackBytes := []byte("exact entity-set enumeration")
	fallbackDigest := digestBytesTest(t, fallbackBytes)
	fallbackBlob := mustValueTest(typedmemorystore.NewObservableInputBlob(
		fallbackReference,
		fallbackDigest,
		fallbackBytes,
	))
	fallback := &recordingObservableInputProvider{blob: fallbackBlob}
	provider, err := NewPreAdmissionObservableInputProvider(stage, fallback)
	if err != nil {
		t.Fatalf("NewPreAdmissionObservableInputProvider() error = %v", err)
	}

	loaded, err := provider.LoadObservableInput(
		context.Background(),
		stage.ProjectID(),
		fallbackReference,
		fallbackDigest,
	)
	if err != nil {
		t.Fatalf("load fallback observable: %v", err)
	}
	if loaded.Reference() != fallbackReference || loaded.Digest() != fallbackDigest {
		t.Fatal("fallback provider lost the requested exact coordinate")
	}
	if fallback.loads != 1 {
		t.Fatalf("fallback loads = %d, want 1", fallback.loads)
	}

	_, err = provider.LoadObservableInput(
		context.Background(),
		stage.ProjectID(),
		stage.ObservableInput().Reference(),
		fallbackDigest,
	)
	if !errors.Is(err, ErrPreAdmissionSourceUnavailable) {
		t.Fatalf("substituted staged digest error = %v", err)
	}
	if fallback.loads != 1 {
		t.Fatal("staged-reference mismatch fell through to the fallback provider")
	}

	overlay, err := provider.LoadSnapshotObservableInputs(
		context.Background(),
		stage.ProjectID(),
	)
	if err != nil {
		t.Fatalf("load provider snapshot overlay: %v", err)
	}
	if len(overlay) != 1 ||
		overlay[0].Reference() != stage.ObservableInput().Reference() {
		t.Fatalf("provider snapshot overlay = %#v, want only staged Note source", overlay)
	}

	if _, err := NewPreAdmissionObservableInputProvider(stage, nil); !errors.Is(
		err,
		ErrPreAdmissionFallbackProviderMissing,
	) {
		t.Fatalf("nil fallback error = %v", err)
	}
}

func digestBytesTest(
	t *testing.T,
	content []byte,
) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256(content)
	return mustValueTest(typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	))
}
