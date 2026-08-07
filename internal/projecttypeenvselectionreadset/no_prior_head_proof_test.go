package projecttypeenvselectionreadset

import (
	"bytes"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

func TestNoPriorHeadProofIsCanonicalContentAddressedAuditRecord(
	t *testing.T,
) {
	project := readSetProject(t, "qnt_89abcdef")
	graph := readSetGraphBasis(t, project, 23, "a")
	head, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(
		project,
	)
	if err != nil {
		t.Fatalf("ProjectTypeEnvHeadRefForProject(): %v", err)
	}
	observedAt := time.Date(
		2026,
		time.July,
		17,
		11,
		23,
		45,
		123456789,
		time.FixedZone("fixture", 4*60*60),
	)
	first, err := sealNoPriorHeadProofRecord(noPriorHeadProofInput{
		project:      project,
		head:         head,
		currentGraph: graph,
		observedAt:   observedAt,
	})
	if err != nil {
		t.Fatalf("sealNoPriorHeadProofRecord(first): %v", err)
	}
	if err := first.Verify(); err != nil {
		t.Fatalf("first.Verify(): %v", err)
	}
	wantObservedAt := observedAt.Round(0).UTC()
	if !first.ObservedAt().Equal(wantObservedAt) ||
		first.ObservedAt().Location() != time.UTC {
		t.Fatalf(
			"ObservedAt() = %s (%s); want %s (UTC)",
			first.ObservedAt(),
			first.ObservedAt().Location(),
			wantObservedAt,
		)
	}
	decoded, err := VerifyNoPriorHeadProof(
		first.Ref(),
		first.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("VerifyNoPriorHeadProof(): %v", err)
	}
	if decoded.Ref() != first.Ref() ||
		!bytes.Equal(decoded.CanonicalBytes(), first.CanonicalBytes()) {
		t.Fatal("canonical round trip changed proof identity")
	}
	if first.Digest() != first.Ref().Digest() {
		t.Fatal("proof digest differs from its content-address reference")
	}
	substitutedGraph := readSetGraphBasis(
		t,
		project,
		graph.GraphRevision().Value(),
		"b",
	)
	if err := VerifyNoPriorHeadProofAgainstGraphSnapshot(
		first,
		substitutedGraph,
	); err == nil {
		t.Fatal("proof verified against a substituted graph basis")
	}

	second, err := sealNoPriorHeadProofRecord(noPriorHeadProofInput{
		project:      project,
		head:         head,
		currentGraph: graph,
		observedAt:   observedAt.Add(time.Nanosecond),
	})
	if err != nil {
		t.Fatalf("sealNoPriorHeadProofRecord(second): %v", err)
	}
	if second.Ref() == first.Ref() {
		t.Fatal("observed_at change did not change proof content address")
	}
	if _, err := VerifyNoPriorHeadProof(
		second.Ref(),
		first.CanonicalBytes(),
	); err == nil {
		t.Fatal("proof bytes verified under a different content address")
	}
	if _, err := DecodeNoPriorHeadProof(
		append(first.CanonicalBytes(), 0x00),
	); err == nil {
		t.Fatal("proof decoder accepted trailing bytes")
	}
	legacy := noPriorHeadProofWriter{}
	legacy.addString("haft.project-typeenv.no-prior-head-proof.v1")
	legacy.addString(project.String())
	legacy.addString(head.String())
	legacy.addString(graph.Ref().String())
	legacy.addString(graph.Ref().Digest().String())
	legacy.addUint64(graph.GraphRevision().Value())
	if _, err := DecodeNoPriorHeadProof(legacy.bytes()); err == nil {
		t.Fatal("v2 proof decoder accepted legacy v1 proof bytes")
	}

	returned := first.CanonicalBytes()
	returned[len(returned)-1] ^= 0xff
	if err := first.Verify(); err != nil {
		t.Fatalf("caller mutation changed immutable proof: %v", err)
	}
}

func TestNoPriorHeadProofRejectsMissingObservationTime(t *testing.T) {
	project := readSetProject(t, "qnt_89abcdef")
	graph := readSetGraphBasis(t, project, 1, "a")
	head, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(
		project,
	)
	if err != nil {
		t.Fatalf("ProjectTypeEnvHeadRefForProject(): %v", err)
	}
	_, err = sealNoPriorHeadProofRecord(noPriorHeadProofInput{
		project:      project,
		head:         head,
		currentGraph: graph,
		observedAt:   time.Time{},
	})
	if err == nil {
		t.Fatal("sealNoPriorHeadProofRecord accepted missing observed_at")
	}
}
