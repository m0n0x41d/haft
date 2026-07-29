package typedmemorystore

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// KindClassificationSourceBlob is one immutable delivery carrier available to
// a current kind-classification evaluator. It is not Evidence, a judgement,
// or part of the four-input C.3.2 request.
type KindClassificationSourceBlob struct {
	reference typedmemory.CarrierRef
	digest    typedmemory.SHA256Digest
	content   []byte
}

func NewKindClassificationSourceBlob(
	reference typedmemory.CarrierRef,
	digest typedmemory.SHA256Digest,
	content []byte,
) (KindClassificationSourceBlob, error) {
	parsedReference, err := typedmemory.NewCarrierRef(reference.String())
	if err != nil || parsedReference != reference {
		return KindClassificationSourceBlob{}, fmt.Errorf(
			"kind-classification source reference is invalid",
		)
	}
	parsedDigest, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || parsedDigest != digest {
		return KindClassificationSourceBlob{}, fmt.Errorf(
			"kind-classification source digest is invalid",
		)
	}
	if len(content) == 0 {
		return KindClassificationSourceBlob{}, fmt.Errorf(
			"kind-classification source bytes are required",
		)
	}
	contentDigest, err := digestBytes(content)
	if err != nil || contentDigest != digest {
		return KindClassificationSourceBlob{}, fmt.Errorf(
			"kind-classification source digest does not match its exact bytes",
		)
	}
	return KindClassificationSourceBlob{
		reference: reference,
		digest:    digest,
		content:   append([]byte(nil), content...),
	}, nil
}

func (blob KindClassificationSourceBlob) Reference() typedmemory.CarrierRef {
	return blob.reference
}

func (blob KindClassificationSourceBlob) Digest() typedmemory.SHA256Digest {
	return blob.digest
}

func (blob KindClassificationSourceBlob) Bytes() []byte {
	return append([]byte(nil), blob.content...)
}

func (blob KindClassificationSourceBlob) valid() bool {
	rebuilt, err := NewKindClassificationSourceBlob(
		blob.reference,
		blob.digest,
		blob.content,
	)
	return err == nil &&
		rebuilt.reference == blob.reference &&
		rebuilt.digest == blob.digest &&
		bytes.Equal(rebuilt.content, blob.content)
}

// KindClassificationSourceProvider resolves exact source bytes during commit
// revalidation. The caller supplies coordinates recovered from the sealed
// classification basis; providers cannot choose which source is relied on.
type KindClassificationSourceProvider interface {
	LoadKindClassificationSource(
		context.Context,
		projectledger.ProjectID,
		typedmemory.CarrierRef,
		typedmemory.SHA256Digest,
	) (KindClassificationSourceBlob, error)
}

// SnapshotKindClassificationSourceOverlay supplies request-scoped immutable
// sources to initial validation without granting a write or database port.
type SnapshotKindClassificationSourceOverlay interface {
	LoadSnapshotKindClassificationSources(
		context.Context,
		projectledger.ProjectID,
	) ([]KindClassificationSourceBlob, error)
}

// SealedHistoricalKindClassificationSourceAdapter may derive current delivery
// source bytes from an exact immutable historical observable. It cannot use a
// historical judgement as a current classification result, persist a backfill,
// or alter the source observable. The current evaluator still applies its exact
// criterion to the newly derived governed features.
type SealedHistoricalKindClassificationSourceAdapter interface {
	AdaptSealedHistoricalKindClassificationSources(
		projectledger.ProjectID,
		[]ObservableInputBlob,
	) ([]KindClassificationSourceBlob, error)
}

func normalizeKindClassificationSourceBlobs(
	values []KindClassificationSourceBlob,
) ([]KindClassificationSourceBlob, error) {
	result := append([]KindClassificationSourceBlob(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(
			kindClassificationSourceBlobCoordinate(result[left]),
			kindClassificationSourceBlobCoordinate(result[right]),
		) < 0
	})
	for index, blob := range result {
		if !blob.valid() {
			return nil, fmt.Errorf(
				"kind-classification source %d is invalid",
				index,
			)
		}
		if index == 0 {
			continue
		}
		previous := result[index-1]
		if previous.reference != blob.reference {
			continue
		}
		if previous.digest != blob.digest ||
			!bytes.Equal(previous.content, blob.content) {
			return nil, fmt.Errorf(
				"kind-classification source reference has conflicting exact content",
			)
		}
		return nil, fmt.Errorf(
			"kind-classification source is repeated",
		)
	}
	return result, nil
}

// coalesceKindClassificationSourceBlobs collapses repeated reliance on the
// same exact source across several classification uses. A repeated coordinate
// is not a second source; any content conflict still fails closed.
func coalesceKindClassificationSourceBlobs(
	values []KindClassificationSourceBlob,
) ([]KindClassificationSourceBlob, error) {
	byReference := make(map[string]KindClassificationSourceBlob, len(values))
	for _, blob := range values {
		if !blob.valid() {
			return nil, fmt.Errorf("kind-classification source is invalid")
		}
		key := blob.Reference().String()
		previous, repeated := byReference[key]
		if !repeated {
			byReference[key] = blob
			continue
		}
		if previous.Digest() != blob.Digest() ||
			!bytes.Equal(previous.Bytes(), blob.Bytes()) {
			return nil, fmt.Errorf(
				"kind-classification source reference has conflicting exact content",
			)
		}
	}
	result := make([]KindClassificationSourceBlob, 0, len(byReference))
	for _, blob := range byReference {
		result = append(result, blob)
	}
	return normalizeKindClassificationSourceBlobs(result)
}

func extendKindClassificationSourceCatalogWithSealedHistorical(
	project projectledger.ProjectID,
	engine KindClassificationAdmissionEngine,
	historical immutableObservableInputCatalog,
	current immutableKindClassificationSourceCatalog,
) (immutableKindClassificationSourceCatalog, error) {
	adapter, available := engine.(SealedHistoricalKindClassificationSourceAdapter)
	if !available {
		return current, nil
	}
	adapted, err := adapter.AdaptSealedHistoricalKindClassificationSources(
		project,
		historical.Blobs(),
	)
	if err != nil {
		return immutableKindClassificationSourceCatalog{}, err
	}
	merged := current.Blobs()
	merged = append(merged, adapted...)
	coalesced, err := coalesceKindClassificationSourceBlobs(merged)
	if err != nil {
		return immutableKindClassificationSourceCatalog{}, err
	}
	return newImmutableKindClassificationSourceCatalog(coalesced)
}

type immutableKindClassificationSourceCatalog struct {
	blobs []KindClassificationSourceBlob
}

func newImmutableKindClassificationSourceCatalog(
	values []KindClassificationSourceBlob,
) (immutableKindClassificationSourceCatalog, error) {
	normalized, err := normalizeKindClassificationSourceBlobs(values)
	if err != nil {
		return immutableKindClassificationSourceCatalog{}, err
	}
	return immutableKindClassificationSourceCatalog{blobs: normalized}, nil
}

func (catalog immutableKindClassificationSourceCatalog) Blobs() []KindClassificationSourceBlob {
	return append([]KindClassificationSourceBlob(nil), catalog.blobs...)
}

func kindClassificationSourceBlobCoordinate(
	blob KindClassificationSourceBlob,
) []byte {
	value := blob.reference.String() + "\x00" + blob.digest.String()
	return []byte(value)
}

func kindClassificationSourceProviderIsPresent(
	provider KindClassificationSourceProvider,
) bool {
	return interfaceCapabilityPresent(provider)
}

func snapshotKindClassificationSourceOverlayIsPresent(
	overlay SnapshotKindClassificationSourceOverlay,
) bool {
	return interfaceCapabilityPresent(overlay)
}

func interfaceCapabilityPresent(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !reflected.IsNil()
	default:
		return true
	}
}
