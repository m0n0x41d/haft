package memberofevaluation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ObservableInputBlob is immutable content addressed by the exact reference
// and digest carried by a MemberOf basis. The blob is source evidence; it is
// not itself a membership judgement.
type ObservableInputBlob struct {
	reference typedmemory.ObservableInputRef
	digest    typedmemory.SHA256Digest
	bytes     []byte
}

func NewObservableInputBlob(
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
	content []byte,
) (ObservableInputBlob, error) {
	parsedReference, err := typedmemory.NewObservableInputRef(reference.String())
	if err != nil || parsedReference != reference {
		return ObservableInputBlob{}, fmt.Errorf("observable input reference is required")
	}
	if len(content) == 0 {
		return ObservableInputBlob{}, fmt.Errorf("observable input bytes are required")
	}
	actual, err := digestBytes(content)
	if err != nil {
		return ObservableInputBlob{}, err
	}
	if actual != digest {
		return ObservableInputBlob{}, fmt.Errorf(
			"observable input bytes do not match the exact digest",
		)
	}
	return ObservableInputBlob{
		reference: reference,
		digest:    digest,
		bytes:     append([]byte(nil), content...),
	}, nil
}

func (blob ObservableInputBlob) Reference() typedmemory.ObservableInputRef {
	return blob.reference
}

func (blob ObservableInputBlob) Digest() typedmemory.SHA256Digest { return blob.digest }

func (blob ObservableInputBlob) Bytes() []byte {
	return append([]byte(nil), blob.bytes...)
}

func (blob ObservableInputBlob) Valid() bool {
	rebuilt, err := NewObservableInputBlob(
		blob.reference,
		blob.digest,
		blob.bytes,
	)
	return err == nil &&
		rebuilt.reference == blob.reference &&
		rebuilt.digest == blob.digest &&
		bytes.Equal(rebuilt.bytes, blob.bytes)
}

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
		if found && !bytes.Equal(existing.Bytes(), copy.Bytes()) {
			return immutableObservableInputCatalog{}, fmt.Errorf(
				"immutable observable input identity has conflicting bytes",
			)
		}
		if found {
			continue
		}
		seen[key] = copy
		verified = append(verified, copy)
	}
	slices.SortFunc(verified, compareObservableInputBlobs)
	return immutableObservableInputCatalog{blobs: verified}, nil
}

func (catalog immutableObservableInputCatalog) Len() int {
	return len(catalog.blobs)
}

func (catalog immutableObservableInputCatalog) Blobs() []ObservableInputBlob {
	return cloneObservableInputBlobs(catalog.blobs)
}

func cloneObservableInputBlobs(blobs []ObservableInputBlob) []ObservableInputBlob {
	result := make([]ObservableInputBlob, 0, len(blobs))
	for _, blob := range blobs {
		result = append(result, ObservableInputBlob{
			reference: blob.reference,
			digest:    blob.digest,
			bytes:     blob.Bytes(),
		})
	}
	return result
}

func compareObservableInputBlobs(left, right ObservableInputBlob) int {
	if left.Reference().String() < right.Reference().String() {
		return -1
	}
	if left.Reference().String() > right.Reference().String() {
		return 1
	}
	if left.Digest().String() < right.Digest().String() {
		return -1
	}
	if left.Digest().String() > right.Digest().String() {
		return 1
	}
	return 0
}

func observableInputIdentity(blob ObservableInputBlob) string {
	return blob.Reference().String() + "\x00" + blob.Digest().String()
}

func digestBytes(value []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(value)
	encoded := hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest("sha256:" + encoded)
}
