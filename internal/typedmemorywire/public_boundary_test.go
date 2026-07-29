package typedmemorywire

import (
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestValidateStrictJSONRejectsDuplicateFields(t *testing.T) {
	t.Parallel()

	err := ValidateStrictJSON(
		[]byte(`{"action":"establish","action":"establish"}`),
	)
	var decodeError *DecodeError
	if !errors.As(err, &decodeError) {
		t.Fatalf("ValidateStrictJSON() error = %v, want DecodeError", err)
	}
	if decodeError.Code() != ErrorInvalidContract &&
		decodeError.Code() != ErrorMalformedJSON {
		t.Fatalf("duplicate-field code = %q", decodeError.Code())
	}
}

func TestNewExactProjectSelectorRetainsObservedCoordinates(t *testing.T) {
	t.Parallel()

	digest, err := typedmemory.NewSHA256Digest(
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatal(err)
	}
	revision := typedmemory.NewGraphRevision(17)
	selector, err := NewExactProjectSelector(digest, revision)
	if err != nil {
		t.Fatalf("NewExactProjectSelector() error = %v", err)
	}
	if selector.RequestedTypeEnvDigest() != digest ||
		selector.RequestedGraphRevision() != revision {
		t.Fatalf("selector = %#v", selector)
	}
}
