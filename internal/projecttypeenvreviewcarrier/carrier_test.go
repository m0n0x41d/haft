package projecttypeenvreviewcarrier

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
)

func TestCarrierOwnsBytesAndRoundTripsDigest(t *testing.T) {
	source := []byte("{\"schema\":\"v1\"}\n")
	carrier, err := NewCarrier(source)
	if err != nil {
		t.Fatalf("new carrier: %v", err)
	}
	source[0] = '!'
	if bytes.Equal(carrier.Bytes(), source) {
		t.Fatal("carrier retained caller-owned mutable bytes")
	}
	parsed, err := ParseDigest(carrier.Digest().String())
	if err != nil {
		t.Fatalf("parse digest: %v", err)
	}
	if parsed != carrier.Digest() {
		t.Fatalf("parsed digest = %s, want %s", parsed, carrier.Digest())
	}
}

func TestNewCarrierRejectsOversizedContent(t *testing.T) {
	_, err := NewCarrier(make([]byte, MaximumBytes+1))
	if err == nil {
		t.Fatal("oversized carrier was accepted")
	}
}

func TestParseDigestRejectsWeakOrMalformedValues(t *testing.T) {
	values := []string{
		"",
		"sha1:abcd",
		"sha256:abcd",
		"sha256:not-hex",
	}
	for _, value := range values {
		if _, err := ParseDigest(value); err == nil {
			t.Fatalf("digest %q was accepted", value)
		}
	}
}

func TestParseDigestAcceptsSyntacticallyValidAllZeroDigest(t *testing.T) {
	value := "sha256:" + strings.Repeat("0", sha256.Size*2)
	digest, err := ParseDigest(value)
	if err != nil {
		t.Fatalf("parse all-zero digest: %v", err)
	}
	if digest.String() != value {
		t.Fatalf("digest string = %q, want %q", digest.String(), value)
	}
}
