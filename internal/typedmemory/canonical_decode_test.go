package typedmemory

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeCanonicalRelationInstanceRoundTripsExactStrongValue(t *testing.T) {
	relation := canonicalDecodeRelationFixture(t)
	raw, err := relation.CanonicalBytes()
	if err != nil {
		t.Fatalf("RelationInstance.CanonicalBytes(): %v", err)
	}
	want := append([]byte(nil), raw...)

	decoded, err := DecodeCanonicalRelationInstance(raw)
	if err != nil {
		t.Fatalf("DecodeCanonicalRelationInstance(): %v", err)
	}
	got, err := decoded.CanonicalBytes()
	if err != nil {
		t.Fatalf("decoded RelationInstance.CanonicalBytes(): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("decoded relation did not preserve exact canonical bytes")
	}
	if decoded.Assertion() != relation.Assertion() ||
		decoded.Signature() != relation.Signature() ||
		decoded.Slice().Ref() != relation.Slice().Ref() ||
		decoded.Provenance() != relation.Provenance() {
		t.Fatal("decoded relation changed a strong identity field")
	}

	foundReference := false
	foundValue := false
	for _, binding := range decoded.Bindings() {
		for _, filler := range binding.Fillers() {
			switch filler.(type) {
			case ReferenceFiller:
				foundReference = true
			case ValueFiller:
				foundValue = true
			}
		}
	}
	if !foundReference || !foundValue {
		t.Fatal("decoded relation did not recover both closed filler variants")
	}

	raw[0] ^= 0xff
	afterMutation, err := decoded.CanonicalBytes()
	if err != nil {
		t.Fatalf("decoded RelationInstance.CanonicalBytes() after input mutation: %v", err)
	}
	if !bytes.Equal(afterMutation, want) {
		t.Fatal("decoded relation retained mutable input bytes")
	}
}

func TestDecodeCanonicalContextSliceRoundTripsAllClosedGammaVariants(t *testing.T) {
	point := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	window, err := NewGammaWindow(
		mustContextSlicePoint(t, "2026-07-15T08:00:00Z").At(),
		point.At(),
		GammaBoundaryInclusive,
		GammaBoundaryExclusive,
	)
	if err != nil {
		t.Fatalf("NewGammaWindow(): %v", err)
	}
	policy, err := NewGammaPolicyApplication(
		mustContextSliceCarrierRef(t, "policy:canonical-decode"),
		mustContextSliceEdition(t, "2026-07"),
		mustContextSliceDigest(t, "canonical-decode-policy"),
		point,
		window,
	)
	if err != nil {
		t.Fatalf("NewGammaPolicyApplication(): %v", err)
	}

	for name, gamma := range map[string]GammaTimeSelector{
		"point":  point,
		"window": window,
		"policy": policy,
	} {
		t.Run(name, func(t *testing.T) {
			slice := canonicalDecodeContextSliceFixture(t, gamma)
			raw := slice.CanonicalBytes()
			want := append([]byte(nil), raw...)

			decoded, decodeErr := DecodeCanonicalContextSlice(raw)
			if decodeErr != nil {
				t.Fatalf("DecodeCanonicalContextSlice(): %v", decodeErr)
			}
			if decoded.Ref() != slice.Ref() {
				t.Fatalf(
					"decoded ref = %s, want %s",
					decoded.Ref().String(),
					slice.Ref().String(),
				)
			}
			if !bytes.Equal(decoded.CanonicalBytes(), want) {
				t.Fatal("decoded ContextSlice did not preserve exact canonical bytes")
			}
			if len(decoded.StandardPins()) != 2 ||
				len(decoded.EnvironmentSelectors()) != 2 ||
				len(decoded.VocabularyPins()) != 1 ||
				len(decoded.RoleSetPins()) != 1 {
				t.Fatal("decoded ContextSlice changed an exact selection set")
			}

			raw[len(raw)-1] ^= 0xff
			if !bytes.Equal(decoded.CanonicalBytes(), want) {
				t.Fatal("decoded ContextSlice retained mutable input bytes")
			}
		})
	}
}

func TestDecodeCanonicalRelationInstanceRejectsIntegrityAndCanonicalityFailures(t *testing.T) {
	relation := canonicalDecodeRelationFixture(t)
	raw, err := relation.CanonicalBytes()
	if err != nil {
		t.Fatalf("RelationInstance.CanonicalBytes(): %v", err)
	}

	t.Run("trailing bytes", func(t *testing.T) {
		mutated := append(append([]byte(nil), raw...), 0)
		expectCanonicalRelationDecodeError(t, mutated, "tail")
	})

	t.Run("invalid UTF-8", func(t *testing.T) {
		mutated := append([]byte(nil), raw...)
		offset := bytes.Index(mutated, []byte(relation.Assertion().String()))
		if offset < 0 {
			t.Fatal("assertion bytes not found")
		}
		mutated[offset] = 0xff
		expectCanonicalRelationDecodeError(t, mutated, "UTF-8")
	})

	t.Run("ContextSlice ref mismatch", func(t *testing.T) {
		mutated := append([]byte(nil), raw...)
		mutateLastDigestNibble(t, mutated, relation.Slice().Ref().String())
		expectCanonicalRelationDecodeError(t, mutated, "does not match")
	})

	t.Run("typed-value digest mismatch", func(t *testing.T) {
		digest := canonicalDecodeValueDigest(t, relation)
		mutated := append([]byte(nil), raw...)
		mutateLastDigestNibble(t, mutated, digest.String())
		expectCanonicalRelationDecodeError(t, mutated, "content-derived digest")
	})

	t.Run("noncanonical binding order", func(t *testing.T) {
		mutated := canonicalDecodeReverseRelationBindings(t, raw)
		expectCanonicalRelationDecodeError(t, mutated, "normalized")
	})
}

func TestDecodeCanonicalContextSliceRejectsMalformedAndNoncanonicalInput(t *testing.T) {
	slice := canonicalDecodeContextSliceFixture(
		t,
		mustContextSlicePoint(t, "2026-07-16T08:00:00Z"),
	)
	raw := slice.CanonicalBytes()

	t.Run("wrong domain", func(t *testing.T) {
		writer := newCanonicalWriter("not-a-context-slice.v1")
		writer.addString("context:test")
		expectCanonicalContextSliceDecodeError(t, writer.bytes(), "domain")
	})

	t.Run("truncated field", func(t *testing.T) {
		mutated := append([]byte(nil), raw[:len(raw)-1]...)
		expectCanonicalContextSliceDecodeError(t, mutated, "length")
	})

	t.Run("collection count limit", func(t *testing.T) {
		writer := newCanonicalWriter(contextSliceCanonicalDomain)
		writer.addString("context:test")
		writer.addUint64(maxCollectionItems + 1)
		expectCanonicalContextSliceDecodeError(t, writer.bytes(), "count")
	})

	t.Run("noncanonical set order", func(t *testing.T) {
		mutated := canonicalDecodeReverseContextStandards(t, raw)
		expectCanonicalContextSliceDecodeError(t, mutated, "normalized")
	})

	t.Run("unsupported Gamma domain", func(t *testing.T) {
		mutated := canonicalDecodeReplaceContextGamma(
			t,
			raw,
			newCanonicalWriter("context-slice-gamma-implicit-now.v1").bytes(),
		)
		expectCanonicalContextSliceDecodeError(t, mutated, "unsupported Gamma")
	})
}

func TestDecodeCanonicalRelationInstanceRejectsNoncanonicalReferenceGrammar(t *testing.T) {
	relation := canonicalDecodeRelationFixture(t)
	raw, err := relation.CanonicalBytes()
	if err != nil {
		t.Fatalf("RelationInstance.CanonicalBytes(): %v", err)
	}
	reference := canonicalDecodeReferenceFiller(t, relation)
	mutated := append([]byte(nil), raw...)
	key := reference.Reference().ReferenceKey()
	offset := bytes.Index(mutated, []byte(key))
	if offset < 0 {
		t.Fatalf("reference key %q not found", key)
	}
	copy(mutated[offset:offset+len("persisted:")], []byte("local:xxxx"))
	expectCanonicalRelationDecodeError(t, mutated, "not persisted")
}

func canonicalDecodeRelationFixture(t *testing.T) RelationInstance {
	t.Helper()
	fixture := newValidationFixture(t)
	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("validation verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	changes := valid.ChangeSet().Changes()
	if len(changes) != 1 {
		t.Fatalf("validated changes = %d, want 1", len(changes))
	}
	validated, ok := changes[0].(ValidatedRelationInstance)
	if !ok {
		t.Fatalf("validated change = %T; want ValidatedRelationInstance", changes[0])
	}
	return validated.Relation()
}

func canonicalDecodeContextSliceFixture(
	t *testing.T,
	gamma GammaTimeSelector,
) ContextSlice {
	t.Helper()
	return mustContextSliceBuild(t, ContextSliceInput{
		Context: mustContextSliceContext(t, "context:canonical-decode"),
		StandardPins: []StandardPin{
			mustContextSliceStandardPin(t, "standard:a", "v1", "standard-a"),
			mustContextSliceStandardPin(t, "standard:b", "v2", "standard-b"),
		},
		EnvironmentSelectors: []EnvironmentSelector{
			mustContextSliceEnvironment(t, "jurisdiction", "AM", "env-am"),
			mustContextSliceEnvironment(t, "platform", "linux-arm64", "env-linux"),
		},
		VocabularyPins: []VocabularyPin{
			mustContextSliceVocabularyPin(t, "vocabulary:a", "v3", "vocabulary-a"),
		},
		RoleSetPins: []RoleSetPin{
			mustContextSliceRoleSetPin(t, "roles:a", "v4", "roles-a"),
		},
		GammaTime: gamma,
	})
}

func canonicalDecodeValueDigest(t *testing.T, relation RelationInstance) SHA256Digest {
	t.Helper()
	for _, binding := range relation.Bindings() {
		for _, filler := range binding.Fillers() {
			value, ok := filler.(ValueFiller)
			if ok {
				return value.Value().Digest()
			}
		}
	}
	t.Fatal("relation fixture has no ValueFiller")
	return SHA256Digest{}
}

func canonicalDecodeReferenceFiller(t *testing.T, relation RelationInstance) ReferenceFiller {
	t.Helper()
	for _, binding := range relation.Bindings() {
		for _, filler := range binding.Fillers() {
			reference, ok := filler.(ReferenceFiller)
			if ok {
				return reference
			}
		}
	}
	t.Fatal("relation fixture has no ReferenceFiller")
	return ReferenceFiller{}
}

func canonicalDecodeReverseRelationBindings(t *testing.T, raw []byte) []byte {
	t.Helper()
	reader, err := newStrictCanonicalReader(raw, "validated-relation-instance.v2")
	if err != nil {
		t.Fatalf("newStrictCanonicalReader(): %v", err)
	}
	assertion, _ := reader.readString()
	signature, _ := reader.readString()
	sliceRef, _ := reader.readString()
	sliceRaw, _ := reader.readBytes()
	tail, err := reader.readRemainingBytes()
	if err != nil {
		t.Fatalf("readRemainingBytes(): %v", err)
	}
	if len(tail) < 3 {
		t.Fatalf("relation tail = %d fields; need two bindings and provenance", len(tail))
	}

	writer := newCanonicalWriter("validated-relation-instance.v2")
	writer.addString(assertion)
	writer.addString(signature)
	writer.addString(sliceRef)
	writer.addBytes(sliceRaw)
	for index := len(tail) - 2; index >= 0; index-- {
		writer.addBytes(tail[index])
	}
	provenance, err := strictUTF8String(tail[len(tail)-1])
	if err != nil {
		t.Fatalf("strictUTF8String(provenance): %v", err)
	}
	writer.addString(provenance)
	return writer.bytes()
}

func canonicalDecodeReverseContextStandards(t *testing.T, raw []byte) []byte {
	t.Helper()
	reader, err := newStrictCanonicalReader(raw, contextSliceCanonicalDomain)
	if err != nil {
		t.Fatalf("newStrictCanonicalReader(): %v", err)
	}
	context, _ := reader.readString()
	standardCount, _ := reader.readCount()
	standards := make([][]byte, 0, standardCount)
	for index := uint64(0); index < standardCount; index++ {
		value, readErr := reader.readBytes()
		if readErr != nil {
			t.Fatalf("read standard %d: %v", index, readErr)
		}
		standards = append(standards, value)
	}
	if len(standards) < 2 {
		t.Fatal("ContextSlice fixture needs at least two Standard pins")
	}
	remaining, err := reader.readRemainingBytes()
	if err != nil {
		t.Fatalf("readRemainingBytes(): %v", err)
	}

	writer := newCanonicalWriter(contextSliceCanonicalDomain)
	writer.addString(context)
	writer.addUint64(standardCount)
	for index := len(standards) - 1; index >= 0; index-- {
		writer.addBytes(standards[index])
	}
	for _, value := range remaining {
		writer.addBytes(value)
	}
	return writer.bytes()
}

func canonicalDecodeReplaceContextGamma(
	t *testing.T,
	raw []byte,
	gamma []byte,
) []byte {
	t.Helper()
	reader, err := newStrictCanonicalReader(raw, contextSliceCanonicalDomain)
	if err != nil {
		t.Fatalf("newStrictCanonicalReader(): %v", err)
	}
	fields, err := reader.readRemainingBytes()
	if err != nil {
		t.Fatalf("readRemainingBytes(): %v", err)
	}
	if len(fields) == 0 {
		t.Fatal("ContextSlice has no fields")
	}

	writer := newCanonicalWriter(contextSliceCanonicalDomain)
	for _, field := range fields[:len(fields)-1] {
		writer.addBytes(field)
	}
	writer.addBytes(gamma)
	return writer.bytes()
}

func mutateLastDigestNibble(t *testing.T, raw []byte, digestBearingValue string) {
	t.Helper()
	offset := bytes.Index(raw, []byte(digestBearingValue))
	if offset < 0 {
		t.Fatalf("digest-bearing value %q not found", digestBearingValue)
	}
	last := offset + len(digestBearingValue) - 1
	if raw[last] == '0' {
		raw[last] = '1'
		return
	}
	raw[last] = '0'
}

func expectCanonicalRelationDecodeError(t *testing.T, raw []byte, contains string) {
	t.Helper()
	_, err := DecodeCanonicalRelationInstance(raw)
	if err == nil {
		t.Fatalf("DecodeCanonicalRelationInstance() accepted malformed bytes")
	}
	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("decode error = %q, want substring %q", err, contains)
	}
}

func expectCanonicalContextSliceDecodeError(t *testing.T, raw []byte, contains string) {
	t.Helper()
	_, err := DecodeCanonicalContextSlice(raw)
	if err == nil {
		t.Fatalf("DecodeCanonicalContextSlice() accepted malformed bytes")
	}
	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("decode error = %q, want substring %q", err, contains)
	}
}
