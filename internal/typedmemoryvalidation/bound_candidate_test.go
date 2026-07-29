package typedmemoryvalidation

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestEvaluateCandidatePreservesWireValidationSemanticsWithoutJSONRoundTrip(
	t *testing.T,
) {
	environment := testEnvironment(t, "d")
	snapshot := newTestSnapshot(t, environment.Ref(), 47, entityAbsent)
	resolved, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	request := decodeRequest(t, `{"kind":"project_current"}`)
	candidate, err := request.BindChangeSet(environment.Ref())
	if err != nil {
		t.Fatal(err)
	}
	wireService := mustService(t, &fixedResolver{resolution: resolved})
	candidateService := mustService(t, &fixedResolver{resolution: resolved})

	wireOutcome := wireService.Evaluate(request)
	candidateOutcome := candidateService.EvaluateCandidate(
		typedmemorywire.ProjectCurrentSelector{},
		candidate,
	)
	if candidateOutcome.ContractVersion() != typedmemorywire.ContractVersionV2 {
		t.Fatalf(
			"candidate contract version = %q; want v2",
			candidateOutcome.ContractVersion(),
		)
	}
	wireValid, wireReady := wireOutcome.(ValidOutcome)
	candidateValid, candidateReady := candidateOutcome.(ValidOutcome)
	if !wireReady || !candidateReady {
		t.Fatalf(
			"outcomes = %T/%T, want two ValidOutcome values",
			wireOutcome,
			candidateOutcome,
		)
	}
	if wireValid.SemanticChangeDigest() != candidateValid.SemanticChangeDigest() {
		t.Fatal("bound candidate semantic digest differs from strict wire validation")
	}
	wireBasis := wireValid.AdmissionBasis()
	candidateBasis := candidateValid.AdmissionBasis()
	if wireBasis.Digest() != candidateBasis.Digest() ||
		!bytes.Equal(wireBasis.CanonicalBytes(), candidateBasis.CanonicalBytes()) {
		t.Fatal("bound candidate admission basis differs from strict wire validation")
	}
	if !candidateValid.AdmissionBatch().IsValid() {
		t.Fatal("bound candidate did not retain a sealed AdmissionBatch")
	}
}

func TestEvaluateCandidateLabelsDiagnosticsAsSemanticWithoutFabricatingJSONPath(
	t *testing.T,
) {
	environment := testEnvironment(t, "e")
	snapshot := newTestSnapshot(t, environment.Ref(), 53, entityExact)
	resolved, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	request := decodeRequest(t, `{"kind":"project_current"}`)
	candidate, err := request.BindChangeSet(environment.Ref())
	if err != nil {
		t.Fatal(err)
	}
	service := mustService(t, &fixedResolver{resolution: resolved})

	outcome := service.EvaluateCandidate(
		typedmemorywire.ProjectCurrentSelector{},
		candidate,
	)
	if outcome.ContractVersion() != typedmemorywire.ContractVersionV2 {
		t.Fatalf("candidate contract version = %q; want v2", outcome.ContractVersion())
	}
	if outcome.Verdict() != typedmemory.ValidationInvalid {
		t.Fatalf("verdict = %s, want invalid", outcome.Verdict())
	}
	diagnostics := outcome.Diagnostics()
	if len(diagnostics) == 0 {
		t.Fatal("invalid bound candidate returned no diagnostics")
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.PathKind() != DiagnosticPathTypedMemorySemantic {
			t.Fatalf(
				"diagnostic path kind = %s, want %s",
				diagnostic.PathKind(),
				DiagnosticPathTypedMemorySemantic,
			)
		}
	}
}

func TestEvaluateCandidateRejectsZeroCandidateBeforeBasisResolution(t *testing.T) {
	resolver := &fixedResolver{resolution: NewProjectBasisUnavailable()}
	service := mustService(t, resolver)

	outcome := service.EvaluateCandidate(
		typedmemorywire.ProjectCurrentSelector{},
		typedmemory.MemoryChangeSet{},
	)
	if outcome.ContractVersion() != typedmemorywire.ContractVersionV2 {
		t.Fatalf("invalid candidate contract version = %q; want v2", outcome.ContractVersion())
	}
	if outcome.Verdict() != typedmemory.ValidationInvalid {
		t.Fatalf("verdict = %s, want invalid", outcome.Verdict())
	}
	if resolver.callCount != 0 {
		t.Fatalf("invalid candidate reached basis resolver %d time(s)", resolver.callCount)
	}
}
