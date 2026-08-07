package typedmemory

import "testing"

func TestSHA256DigestRequiresExactLowercaseEncoding(t *testing.T) {
	valid := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := NewSHA256Digest(valid); err != nil {
		t.Fatalf("valid digest: %v", err)
	}

	uppercase := "sha256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if _, err := NewSHA256Digest(uppercase); err == nil {
		t.Fatal("uppercase digest succeeded")
	}
	if _, err := NewSHA256Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err == nil {
		t.Fatal("digest without domain prefix succeeded")
	}
}

func TestParseTypeEnvRefRequiresCanonicalExternalForm(t *testing.T) {
	raw := "typeenv:sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	ref, err := ParseTypeEnvRef(raw)
	if err != nil {
		t.Fatalf("ParseTypeEnvRef(valid) error = %v", err)
	}
	if ref.String() != raw {
		t.Fatalf("ParseTypeEnvRef(valid) = %q; want %q", ref.String(), raw)
	}
	for _, malformed := range []string{
		"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"typeenv: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"typeenv:sha256:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
	} {
		if _, err := ParseTypeEnvRef(malformed); err == nil {
			t.Fatalf("ParseTypeEnvRef(%q) succeeded", malformed)
		}
	}
}

func TestStrongReferenceKeepsRefKindAndIdentitySeparate(t *testing.T) {
	digest, err := NewSHA256Digest("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	typeEnv, err := NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("TypeEnv ref: %v", err)
	}
	refKindID, err := NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("RefKind ID: %v", err)
	}
	refKind, err := NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatalf("reference kind: %v", err)
	}
	referenceID, err := NewReferenceID("entity:haft")
	if err != nil {
		t.Fatalf("reference ID: %v", err)
	}

	ref, err := NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("persisted ref: %v", err)
	}
	if ref.RefKind() != refKind {
		t.Fatal("reference kind changed")
	}
	if ref.ReferenceKey() != "persisted:entity:haft" {
		t.Fatalf("reference key = %q", ref.ReferenceKey())
	}
}

func TestLocalReferenceExposesStrongBatchLocalIdentityWithoutParsingReferenceKey(t *testing.T) {
	digest, err := NewSHA256Digest("sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	typeEnv, err := NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("TypeEnv ref: %v", err)
	}
	refKindID, err := NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("RefKind ID: %v", err)
	}
	refKind, err := NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatalf("reference kind: %v", err)
	}
	localID, err := NewBatchLocalRef("batch-entity")
	if err != nil {
		t.Fatalf("batch-local ID: %v", err)
	}

	ref, err := NewLocalRef(refKind, localID)
	if err != nil {
		t.Fatalf("local ref: %v", err)
	}
	if ref.BatchLocalRef() != localID {
		t.Fatalf("batch-local identity changed: %q", ref.BatchLocalRef().String())
	}
}

func TestCodecRefStringIsInjectiveAcrossIDVersionBoundaries(t *testing.T) {
	digest, err := NewSHA256Digest("sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	firstID, err := NewCodecID("codec@v1")
	if err != nil {
		t.Fatalf("NewCodecID(first): %v", err)
	}
	firstVersion, err := NewCanonicalizationVersion("canonical")
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion(first): %v", err)
	}
	secondID, err := NewCodecID("codec")
	if err != nil {
		t.Fatalf("NewCodecID(second): %v", err)
	}
	secondVersion, err := NewCanonicalizationVersion("v1@canonical")
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion(second): %v", err)
	}
	first, err := NewCodecRef(firstID, firstVersion, digest)
	if err != nil {
		t.Fatalf("NewCodecRef(first): %v", err)
	}
	second, err := NewCodecRef(secondID, secondVersion, digest)
	if err != nil {
		t.Fatalf("NewCodecRef(second): %v", err)
	}
	if first.String() == second.String() {
		t.Fatalf("distinct CodecRefs share String key %q", first.String())
	}
}
