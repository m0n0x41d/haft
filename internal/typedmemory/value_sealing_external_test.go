package typedmemory_test

import (
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type embeddedMemoryChange struct {
	typedmemory.MemoryChange
}

type embeddedIdentityChange struct {
	typedmemory.IdentityChange
}

type embeddedCandidateSlotFiller struct {
	typedmemory.CandidateSlotFiller
}

type embeddedClaimGraphValue struct {
	typedmemory.ClaimGraphValue
}

func TestExternalCodeCannotForgeVerifiedTypedValue(t *testing.T) {
	source := `package probe

import "github.com/m0n0x41d/haft/internal/typedmemory"

type forged struct{}

func (forged) ValueKind() typedmemory.ValueKindRef { return typedmemory.ValueKindRef{} }
func (forged) ValueShape() typedmemory.ValueShapeRef { return typedmemory.ValueShapeRef{} }
func (forged) Codec() typedmemory.CodecRef { return typedmemory.CodecRef{} }
func (forged) CanonicalBytes() []byte { return []byte("forged") }
func (forged) Digest() typedmemory.SHA256Digest { return typedmemory.SHA256Digest{} }

var _ typedmemory.VerifiedTypedValue = forged{}
`
	output := compileTypedMemoryExternalProbe(t, source)
	if !strings.Contains(output, "missing method verifiedTypedValueVariant") {
		t.Fatalf("compiler did not reject external VerifiedTypedValue forgery:\n%s", output)
	}
}

func TestExternalCodeCannotPutSchemaChangeInMemoryChangeSet(t *testing.T) {
	source := `package probe

import "github.com/m0n0x41d/haft/internal/typedmemory"

var _ typedmemory.MemoryChange = typedmemory.DefineKindSchemaChange{}
`
	output := compileTypedMemoryExternalProbe(t, source)
	if !strings.Contains(output, "missing method memoryChangeVariant") {
		t.Fatalf("compiler did not keep schema and memory changes disjoint:\n%s", output)
	}
}

func TestExternalCodeCannotForgeMemoryChange(t *testing.T) {
	source := `package probe

import "github.com/m0n0x41d/haft/internal/typedmemory"

type forged struct{}

func (forged) validMemoryChange() bool { return true }

var _ typedmemory.MemoryChange = forged{}
`
	output := compileTypedMemoryExternalProbe(t, source)
	if !strings.Contains(output, "missing method memoryChangeVariant") {
		t.Fatalf("compiler did not reject external MemoryChange forgery:\n%s", output)
	}
}

func TestExternalCodeCannotForgeValidationVerdict(t *testing.T) {
	source := `package probe

import "github.com/m0n0x41d/haft/internal/typedmemory"

type forged struct{}

func (forged) Kind() typedmemory.ValidationVerdictKind { return typedmemory.ValidationValid }

var _ typedmemory.ValidationVerdict = forged{}
`
	output := compileTypedMemoryExternalProbe(t, source)
	if !strings.Contains(output, "missing method validationVerdictVariant") {
		t.Fatalf("compiler did not reject external ValidationVerdict forgery:\n%s", output)
	}
}

func TestEntityResolutionAndAliasAvailabilityRemainDisjoint(t *testing.T) {
	aliasAsEntity := `package probe

import "github.com/m0n0x41d/haft/internal/typedmemory"

var _ typedmemory.EntityResolution = typedmemory.UnboundAliasResolution{}
`
	output := compileTypedMemoryExternalProbe(t, aliasAsEntity)
	if !strings.Contains(output, "missing method entityResolutionVariant") {
		t.Fatalf("compiler did not separate alias availability from entity resolution:\n%s", output)
	}

	entityAsAlias := `package probe

import "github.com/m0n0x41d/haft/internal/typedmemory"

var _ typedmemory.AliasAvailability = typedmemory.AbsentEntityResolution{}
`
	output = compileTypedMemoryExternalProbe(t, entityAsAlias)
	if !strings.Contains(output, "missing method aliasAvailabilityVariant") {
		t.Fatalf("compiler did not separate entity resolution from alias availability:\n%s", output)
	}
}

func TestExternalCodeCannotForgeEntityResolution(t *testing.T) {
	source := `package probe

import "github.com/m0n0x41d/haft/internal/typedmemory"

type forged struct{}

var _ typedmemory.EntityResolution = forged{}
`
	output := compileTypedMemoryExternalProbe(t, source)
	if !strings.Contains(output, "missing method entityResolutionVariant") {
		t.Fatalf("compiler did not reject external EntityResolution forgery:\n%s", output)
	}
}

func TestEmbeddedClosedVariantsAreRejectedAtRuntimeBoundaries(t *testing.T) {
	entity, err := typedmemory.NewEntityID("entity:embedded-attack")
	if err != nil {
		t.Fatalf("NewEntityID: %v", err)
	}
	local, err := typedmemory.NewBatchLocalRef("local:embedded-attack")
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("ctx:embedded-attack")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	label, err := typedmemory.NewEntityLabel("Embedded attack")
	if err != nil {
		t.Fatalf("NewEntityLabel: %v", err)
	}
	provenance, err := typedmemory.NewProvenanceRef("test:embedded-attack")
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(entity, local, contextRef, label, provenance)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	wrappedMemory := embeddedMemoryChange{MemoryChange: declaration}
	if _, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{wrappedMemory}); err == nil {
		t.Fatal("MemoryChangeSet accepted an embedded non-exact variant")
	}

	alias, err := typedmemory.NewEntityAlias("embedded alias")
	if err != nil {
		t.Fatalf("NewEntityAlias: %v", err)
	}
	aliasChange, err := typedmemory.NewAdmitAlias(entity, alias, contextRef, provenance)
	if err != nil {
		t.Fatalf("NewAdmitAlias: %v", err)
	}
	wrappedIdentity := embeddedIdentityChange{IdentityChange: aliasChange}
	if _, err := typedmemory.NewApplyIdentityChange(wrappedIdentity); err == nil {
		t.Fatal("ApplyIdentityChange accepted an embedded non-exact variant")
	}

	digest, err := typedmemory.NewSHA256Digest(
		"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("NewRefKindID: %v", err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef: %v", err)
	}
	referenceID, err := typedmemory.NewReferenceID("entity:embedded-attack")
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	filler, err := typedmemory.NewByReferenceCandidate(reference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate: %v", err)
	}
	wrappedFiller := embeddedCandidateSlotFiller{CandidateSlotFiller: filler}
	slot, err := typedmemory.NewSlotKindID("EntitySlot")
	if err != nil {
		t.Fatalf("NewSlotKindID: %v", err)
	}
	if _, err := typedmemory.NewCandidateSlotBinding(
		slot,
		[]typedmemory.CandidateSlotFiller{wrappedFiller},
	); err == nil {
		t.Fatal("CandidateSlotBinding accepted an embedded non-exact variant")
	}

	graph, err := typedmemory.NewClaimGraphValue(nil, nil)
	if err != nil {
		t.Fatalf("NewClaimGraphValue: %v", err)
	}
	shapeID, err := typedmemory.NewShapeID("U.ClaimGraphShape")
	if err != nil {
		t.Fatalf("NewShapeID: %v", err)
	}
	shape, err := typedmemory.NewValueShapeRef(shapeID, digest)
	if err != nil {
		t.Fatalf("NewValueShapeRef: %v", err)
	}
	codec, err := typedmemory.NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	wrappedGraph := embeddedClaimGraphValue{ClaimGraphValue: graph}
	if _, ok := codec.EncodeInput(wrappedGraph).(typedmemory.RejectedCodecValue); !ok {
		t.Fatal("ClaimGraphCodecV1 accepted an embedded non-exact ClaimGraphValue")
	}
}

func compileTypedMemoryExternalProbe(t *testing.T, source string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve typed-memory test source location")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	probeRoot, err := os.MkdirTemp(repositoryRoot, ".typedmemory-probe-")
	if err != nil {
		t.Fatalf("create probe directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(probeRoot) })
	probeFile := filepath.Join(probeRoot, "probe.go")
	if err := os.WriteFile(probeFile, []byte(source), 0o600); err != nil {
		t.Fatalf("write probe source: %v", err)
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(goBinary, "test", ".")
	command.Dir = probeRoot
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("external typed-memory forgery probe compiled successfully:\n%s", output)
	}
	return string(output)
}
