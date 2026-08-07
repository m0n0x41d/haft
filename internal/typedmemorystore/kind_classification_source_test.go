package typedmemorystore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSealedHistoricalClassificationSourcesRemainEphemeralAndDeduplicated(
	t *testing.T,
) {
	t.Parallel()
	project := mustKindClassificationSourceValue(
		projectledger.ParseProjectID("qnt_deadbeef"),
	)
	historicalBytes := []byte("sealed-historical-observable")
	historicalDigest := kindClassificationSourceDigest(t, historicalBytes)
	historicalReference := mustKindClassificationSourceValue(
		typedmemory.NewObservableInputRef(
			"record-membership-source:" + historicalDigest.String(),
		),
	)
	historicalBlob := mustKindClassificationSourceValue(
		NewObservableInputBlob(
			historicalReference,
			historicalDigest,
			historicalBytes,
		),
	)
	historical := mustKindClassificationSourceValue(
		newImmutableObservableInputCatalog(
			[]ObservableInputBlob{historicalBlob},
		),
	)
	currentBytes := []byte("derived-current-classification-source")
	currentDigest := kindClassificationSourceDigest(t, currentBytes)
	currentReference := mustKindClassificationSourceValue(
		typedmemory.NewCarrierRef(
			"record-classification-source:" + currentDigest.String(),
		),
	)
	currentBlob := mustKindClassificationSourceValue(
		NewKindClassificationSourceBlob(
			currentReference,
			currentDigest,
			currentBytes,
		),
	)
	current := mustKindClassificationSourceValue(
		newImmutableKindClassificationSourceCatalog(
			[]KindClassificationSourceBlob{currentBlob},
		),
	)
	engine := sealedHistoricalSourceAdapterFixture{source: currentBlob}
	merged := mustKindClassificationSourceValue(
		extendKindClassificationSourceCatalogWithSealedHistorical(
			project,
			engine,
			historical,
			current,
		),
	)
	if len(merged.Blobs()) != 1 {
		t.Fatalf("merged source count = %d, want 1", len(merged.Blobs()))
	}
	if !bytes.Equal(historical.Blobs()[0].Bytes(), historicalBytes) {
		t.Fatal("historical observable bytes changed during source extension")
	}
}

type sealedHistoricalSourceAdapterFixture struct {
	source KindClassificationSourceBlob
}

func (sealedHistoricalSourceAdapterFixture) EvaluateKindClassification(
	context.Context,
	KindClassificationAdmissionInput,
) (typedmemory.KindClassificationJudgement, error) {
	return nil, fmt.Errorf("not used by source-extension test")
}

func (fixture sealedHistoricalSourceAdapterFixture) AdaptSealedHistoricalKindClassificationSources(
	project projectledger.ProjectID,
	historical []ObservableInputBlob,
) ([]KindClassificationSourceBlob, error) {
	if project.String() == "" || len(historical) != 1 {
		return nil, fmt.Errorf("fixture received incomplete historical coordinates")
	}
	return []KindClassificationSourceBlob{fixture.source}, nil
}

func kindClassificationSourceDigest(
	t *testing.T,
	value []byte,
) typedmemory.SHA256Digest {
	t.Helper()
	digest := sha256.Sum256(value)
	return mustKindClassificationSourceValue(
		typedmemory.NewSHA256Digest(fmt.Sprintf("sha256:%x", digest)),
	)
}

func mustKindClassificationSourceValue[T any](value T, err error) T {
	if err != nil {
		panic("construct kind-classification source fixture: " + err.Error())
	}
	return value
}
