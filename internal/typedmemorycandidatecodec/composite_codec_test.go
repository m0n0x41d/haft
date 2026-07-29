package typedmemorycandidatecodec

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestEvidenceUseQualifierV1IsExactlyOneCore5Polarity(t *testing.T) {
	suite := testSuite(t)
	codec := suite.EvidenceUseQualifier()
	value, err := NewEvidenceUseQualifier(EvidenceUndercutting)
	if err != nil {
		t.Fatal(err)
	}
	canonical := requireCanonical(t, codec.EncodeInput(value))
	assertIdempotent(t, codec, codec.Shape(), canonical)
	assertRecordFields(t, canonical.Value(), []string{"polarity"})

	malformed := newCanonicalWriter(evidenceUseQualifierCodecDomain)
	malformed = malformed.addBytes(encodeEvidencePolarityWire(EvidenceConfirming))
	malformed = malformed.addBytes(encodeEvidencePolarityWire(EvidenceRebutting))
	requireRejected(t, codec.Canonicalize(codec.Shape(), malformed.result()))

	unknown := newCanonicalWriter(evidencePolarityCodecDomain).addString("supporting").result()
	malformed = newCanonicalWriter(evidenceUseQualifierCodecDomain).addBytes(unknown)
	requireRejected(t, codec.Canonicalize(codec.Shape(), malformed.result()))
}

func TestPerformedIntervalV1PreservesClosedStateAndUTCOrder(t *testing.T) {
	suite := testSuite(t)
	codec := suite.PerformedInterval()
	point := mustCompletedInterval(
		t,
		"2026-01-01T00:00:00Z",
		"2026-01-01T01:00:00+01:00",
	)
	pointCanonical := requireCanonical(t, codec.EncodeInput(point))
	assertIdempotent(t, codec, codec.Shape(), pointCanonical)
	assertIntervalTypedValue(t, pointCanonical.Value(), completedPerformedIntervalVariant)
	equivalentPoint := mustCompletedInterval(
		t,
		"2026-01-01T01:00:00+01:00",
		"2026-01-01T00:00:00Z",
	)
	equivalentCanonical := requireCanonical(t, codec.EncodeInput(equivalentPoint))
	if !bytes.Equal(pointCanonical.CanonicalBytes(), equivalentCanonical.CanonicalBytes()) {
		t.Fatal("equal conceptual intervals have unequal canonical bytes")
	}

	inFlight := mustInFlightInterval(t, "2026-07-17T12:34:56.12Z")
	inFlightCanonical := requireCanonical(t, codec.EncodeInput(inFlight))
	assertIdempotent(t, codec, codec.Shape(), inFlightCanonical)
	assertIntervalTypedValue(t, inFlightCanonical.Value(), inFlightPerformedIntervalVariant)
	if bytes.Equal(pointCanonical.CanonicalBytes(), inFlightCanonical.CanonicalBytes()) {
		t.Fatal("Completed and InFlight variants collide")
	}

	start := mustInstant(t, "2026-01-01T01:00:00+01:00")
	end := mustInstant(t, "2025-12-31T23:59:59Z")
	if _, err := NewCompletedPerformedInterval(start, end); err == nil {
		t.Fatal("completed interval accepted end before start after UTC normalization")
	}
}

func TestPerformedIntervalV1RejectsUnknownVariantAndNestedTrailingBytes(t *testing.T) {
	codec := testSuite(t).PerformedInterval()
	unknown := newCanonicalWriter(performedIntervalCodecDomain)
	unknown = unknown.addString("Planned")
	unknown = unknown.addBytes([]byte("not-a-state"))
	requireRejected(t, codec.Canonicalize(codec.Shape(), unknown.result()))

	start := encodeCanonicalInstantWire(mustInstant(t, "2026-01-01T00:00:00Z"))
	payload := newCanonicalWriter(inFlightPerformedIntervalCodecDomain)
	payload = payload.addBytes(start)
	payload = payload.addBytes(start)
	outer := newCanonicalWriter(performedIntervalCodecDomain)
	outer = outer.addString(inFlightPerformedIntervalVariant)
	outer = outer.addBytes(payload.result())
	requireRejected(t, codec.Canonicalize(codec.Shape(), outer.result()))

	reversedPayload := newCanonicalWriter(completedPerformedIntervalCodecDomain)
	reversedPayload = reversedPayload.addBytes(encodeRawInstantWireForTest("2026-01-01T00:00:01Z"))
	reversedPayload = reversedPayload.addBytes(encodeRawInstantWireForTest("2026-01-01T00:00:00Z"))
	outer = newCanonicalWriter(performedIntervalCodecDomain)
	outer = outer.addString(completedPerformedIntervalVariant)
	outer = outer.addBytes(reversedPayload.result())
	requireRejected(t, codec.Canonicalize(codec.Shape(), outer.result()))
}

func TestCodeAnchorLocatorV1AcceptsOnlyClosedTargetsAndRepositoryRelativePaths(t *testing.T) {
	suite := testSuite(t)
	codec := suite.CodeAnchorLocator()
	file, err := NewFileCodeAnchorTarget("internal/auth/service.go")
	if err != nil {
		t.Fatal(err)
	}
	fileLocator, err := NewCodeAnchorLocator("github.com/acme/project", "0123456789abcdef", file)
	if err != nil {
		t.Fatal(err)
	}
	fileCanonical := requireCanonical(t, codec.EncodeInput(fileLocator))
	assertIdempotent(t, codec, codec.Shape(), fileCanonical)
	assertAnchorTypedValue(t, fileCanonical.Value(), fileCodeAnchorTargetVariant)

	symbol, err := NewSymbolCodeAnchorTarget("internal/auth/service.go", "AuthService.ValidateToken")
	if err != nil {
		t.Fatal(err)
	}
	symbolLocator, err := NewCodeAnchorLocator("github.com/acme/project", "0123456789abcdef", symbol)
	if err != nil {
		t.Fatal(err)
	}
	symbolCanonical := requireCanonical(t, codec.EncodeInput(symbolLocator))
	assertIdempotent(t, codec, codec.Shape(), symbolCanonical)
	assertAnchorTypedValue(t, symbolCanonical.Value(), symbolCodeAnchorTargetVariant)
	if bytes.Equal(fileCanonical.CanonicalBytes(), symbolCanonical.CanonicalBytes()) {
		t.Fatal("File and Symbol targets collide")
	}

	validPaths := []string{"README.md", "a/b/c.go", "документы/модель.md"}
	for _, path := range validPaths {
		if _, err := NewFileCodeAnchorTarget(path); err != nil {
			t.Errorf("valid path %q rejected: %v", path, err)
		}
	}
	invalidPaths := []string{"", "/absolute", "C:/absolute", ".", "..", "a/../b", "a/./b", "a//b", "a/", `a\b`, "a\x00b"}
	for _, path := range invalidPaths {
		if _, err := NewFileCodeAnchorTarget(path); err == nil {
			t.Errorf("invalid path %q accepted", path)
		}
	}
	if _, err := NewCodeAnchorLocator("", "revision", file); err == nil {
		t.Fatal("empty repository identity accepted")
	}
	if _, err := NewCodeAnchorLocator("repository", "", file); err == nil {
		t.Fatal("empty revision identity accepted")
	}
	if _, err := NewSymbolCodeAnchorTarget("file.go", ""); err == nil {
		t.Fatal("empty source symbol identity accepted")
	}
}

func TestCodeAnchorLocatorV1RejectsUnknownVariantAndNestedEOFViolation(t *testing.T) {
	codec := testSuite(t).CodeAnchorLocator()
	repository := encodeTextWire("repository")
	revision := encodeTextWire("revision")
	unknown := newCanonicalWriter(codeAnchorLocatorCodecDomain)
	unknown = unknown.addBytes(repository)
	unknown = unknown.addBytes(revision)
	unknown = unknown.addString("LineRange")
	unknown = unknown.addBytes([]byte("unknown"))
	requireRejected(t, codec.Canonicalize(codec.Shape(), unknown.result()))

	filePayload := newCanonicalWriter(fileCodeAnchorTargetCodecDomain)
	filePayload = filePayload.addBytes(encodeTextWire("file.go"))
	filePayload = filePayload.addBytes(encodeTextWire("extra"))
	outer := newCanonicalWriter(codeAnchorLocatorCodecDomain)
	outer = outer.addBytes(repository)
	outer = outer.addBytes(revision)
	outer = outer.addString(fileCodeAnchorTargetVariant)
	outer = outer.addBytes(filePayload.result())
	requireRejected(t, codec.Canonicalize(codec.Shape(), outer.result()))
}

func TestCompositeCodecGoldenDigests(t *testing.T) {
	suite := testSuite(t)
	qualifier, _ := NewEvidenceUseQualifier(EvidenceConstraining)
	interval := mustCompletedInterval(
		t,
		"2026-07-17T12:34:56Z",
		"2026-07-17T12:35:56Z",
	)
	target, _ := NewSymbolCodeAnchorTarget("internal/auth.go", "ValidateToken")
	locator, _ := NewCodeAnchorLocator("repo-id", "deadbeef", target)
	values := map[string]typedmemory.CanonicalizedCodecValue{
		"qualifier": requireCanonical(t, suite.EvidenceUseQualifier().EncodeInput(qualifier)),
		"interval":  requireCanonical(t, suite.PerformedInterval().EncodeInput(interval)),
		"anchor":    requireCanonical(t, suite.CodeAnchorLocator().EncodeInput(locator)),
	}
	want := map[string]string{
		"qualifier": "b02f1d5908bda1f08efd6805ff015a4624d7ce3c22aae96fb5aa0350dfd3f08b",
		"interval":  "e1558edb34304b8afd308282cb8a9a254d9b91fb290e0d8408d9fbfb0831f037",
		"anchor":    "a62c644635adb707286cffb96e6a105a7c61fa3cca88d3126b11164f61b38426",
	}
	for label, value := range values {
		if got := canonicalDigest(value); got != want[label] {
			t.Errorf("%s golden digest = %s, want %s", label, got, want[label])
		}
	}
}

func mustInstant(t *testing.T, raw string) CanonicalInstant {
	t.Helper()
	value, err := ParseCanonicalInstant(raw)
	if err != nil {
		t.Fatalf("ParseCanonicalInstant(%q): %v", raw, err)
	}
	return value
}

func mustCompletedInterval(
	t *testing.T,
	startRaw string,
	endRaw string,
) CompletedPerformedInterval {
	t.Helper()
	value, err := NewCompletedPerformedInterval(
		mustInstant(t, startRaw),
		mustInstant(t, endRaw),
	)
	if err != nil {
		t.Fatalf("NewCompletedPerformedInterval(): %v", err)
	}
	return value
}

func mustInFlightInterval(
	t *testing.T,
	startRaw string,
) InFlightPerformedInterval {
	t.Helper()
	value, err := NewInFlightPerformedInterval(mustInstant(t, startRaw))
	if err != nil {
		t.Fatalf("NewInFlightPerformedInterval(): %v", err)
	}
	return value
}

func assertRecordFields(
	t *testing.T,
	value typedmemory.TypedValue,
	want []string,
) {
	t.Helper()
	record, ok := value.(typedmemory.RecordTypedValue)
	if !ok {
		t.Fatalf("value = %T, want RecordTypedValue", value)
	}
	fields := record.Fields()
	if len(fields) != len(want) {
		t.Fatalf("record field count = %d, want %d", len(fields), len(want))
	}
	for index, field := range fields {
		if got := field.Name().String(); got != want[index] {
			t.Fatalf("record field %d = %q, want %q", index, got, want[index])
		}
	}
}

func assertIntervalTypedValue(
	t *testing.T,
	value typedmemory.TypedValue,
	wantVariant string,
) {
	t.Helper()
	record, ok := value.(typedmemory.RecordTypedValue)
	if !ok || len(record.Fields()) != 1 {
		t.Fatalf("interval value = %T, want one-field record", value)
	}
	state, ok := record.Fields()[0].Value().(typedmemory.SumTypedValue)
	if !ok {
		t.Fatalf("interval state = %T, want sum", record.Fields()[0].Value())
	}
	if got := state.Variant().String(); got != wantVariant {
		t.Fatalf("interval variant = %q, want %q", got, wantVariant)
	}
}

func assertAnchorTypedValue(
	t *testing.T,
	value typedmemory.TypedValue,
	wantVariant string,
) {
	t.Helper()
	record, ok := value.(typedmemory.RecordTypedValue)
	if !ok || len(record.Fields()) != 3 {
		t.Fatalf("anchor value = %T, want three-field record", value)
	}
	var target typedmemory.TypedValue
	for _, field := range record.Fields() {
		if field.Name().String() == "target" {
			target = field.Value()
		}
	}
	sum, ok := target.(typedmemory.SumTypedValue)
	if !ok {
		t.Fatalf("anchor target = %T, want sum", target)
	}
	if got := sum.Variant().String(); got != wantVariant {
		t.Fatalf("anchor target variant = %q, want %q", got, wantVariant)
	}
}
