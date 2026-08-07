package typedmemorystore

import (
	"bytes"
	"fmt"
	"sort"
)

// immutableObservableInputCatalog is a copied, content-verified set of exact
// observable bytes. It deliberately has no source-format knowledge; the
// selected evaluator owns query-specific source interpretation.
type immutableObservableInputCatalog struct {
	blobs []ObservableInputBlob
}

func newImmutableObservableInputCatalog(
	blobs []ObservableInputBlob,
) (immutableObservableInputCatalog, error) {
	verified := make([]ObservableInputBlob, 0, len(blobs))
	seen := make(map[string]ObservableInputBlob, len(blobs))
	for _, blob := range blobs {
		copy, err := NewObservableInputBlob(
			blob.Reference(),
			blob.Digest(),
			blob.Bytes(),
		)
		if err != nil {
			return immutableObservableInputCatalog{}, fmt.Errorf(
				"verify immutable observable input catalog: %w",
				err,
			)
		}
		key := observableInputIdentity(copy)
		existing, found := seen[key]
		if found {
			if !bytes.Equal(existing.Bytes(), copy.Bytes()) {
				return immutableObservableInputCatalog{}, fmt.Errorf(
					"immutable observable input identity has conflicting bytes",
				)
			}
			continue
		}
		seen[key] = copy
		verified = append(verified, copy)
	}
	sort.Slice(verified, func(left, right int) bool {
		leftBlob := verified[left]
		rightBlob := verified[right]
		if leftBlob.Reference().String() != rightBlob.Reference().String() {
			return leftBlob.Reference().String() < rightBlob.Reference().String()
		}
		return leftBlob.Digest().String() < rightBlob.Digest().String()
	})
	return immutableObservableInputCatalog{blobs: verified}, nil
}

func (catalog immutableObservableInputCatalog) Len() int {
	return len(catalog.blobs)
}

func (catalog immutableObservableInputCatalog) Blobs() []ObservableInputBlob {
	return cloneObservableInputBlobs(catalog.blobs)
}

func (catalog immutableObservableInputCatalog) ContainsAll(
	selected []ObservableInputBlob,
) bool {
	if len(selected) == 0 {
		return false
	}
	available := make(map[string]ObservableInputBlob, len(catalog.blobs))
	for _, blob := range catalog.blobs {
		available[observableInputIdentity(blob)] = blob
	}
	for _, blob := range selected {
		candidate, found := available[observableInputIdentity(blob)]
		if !found || !bytes.Equal(candidate.Bytes(), blob.Bytes()) {
			return false
		}
	}
	return true
}

func observableInputIdentity(blob ObservableInputBlob) string {
	return blob.Reference().String() + "\x00" + blob.Digest().String()
}

func cloneObservableInputBlobs(blobs []ObservableInputBlob) []ObservableInputBlob {
	return append([]ObservableInputBlob(nil), blobs...)
}
