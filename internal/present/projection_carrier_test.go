package present

import (
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func fixtureProjectionGraphWithRefs() artifact.ProjectionGraph {
	return artifact.ProjectionGraph{
		Context:     "fixture",
		GeneratedAt: time.Now().UTC(),
		Problems: []artifact.ProblemProjection{
			{Meta: artifact.Meta{ID: "prob-20260513-aaaa1111", Title: "Problem A", Kind: artifact.KindProblemCard, Status: artifact.StatusActive}},
		},
		Portfolios: []artifact.PortfolioProjection{
			{Meta: artifact.Meta{ID: "sol-20260513-bbbb2222", Title: "Portfolio A", Kind: artifact.KindSolutionPortfolio, Status: artifact.StatusActive}, Variants: []artifact.Variant{{ID: "V1", Title: "Variant 1"}}, Comparison: &artifact.ComparisonResult{Dimensions: []string{"speed"}, NonDominatedSet: []string{"V1"}}},
		},
		Decisions: []artifact.DecisionProjection{
			{Meta: artifact.Meta{ID: "dec-20260513-cccc3333", Title: "Decision A", Kind: artifact.KindDecisionRecord, Status: artifact.StatusActive}, SelectedTitle: "Variant 1"},
		},
	}
}

func TestProjection_Brief_IncludesCarrierFootnote(t *testing.T) {
	graph := fixtureProjectionGraphWithRefs()
	out := ProjectionResponse(graph, artifact.ProjectionViewDelegatedAgent)
	assertCarrierFootnoteWithRef(t, out, "dec-20260513-cccc3333")
}

func TestProjection_Rationale_IncludesCarrierFootnote(t *testing.T) {
	graph := fixtureProjectionGraphWithRefs()
	out := ProjectionResponse(graph, artifact.ProjectionViewChangeRationale)
	assertCarrierFootnoteWithRef(t, out, "dec-20260513-cccc3333")
}

func TestProjection_Audit_IncludesCarrierFootnote(t *testing.T) {
	graph := fixtureProjectionGraphWithRefs()
	out := ProjectionResponse(graph, artifact.ProjectionViewAudit)
	assertCarrierFootnoteWithRef(t, out, "dec-20260513-cccc3333")
	if !strings.Contains(out, "prob-20260513-aaaa1111") {
		t.Fatalf("audit projection must list problem source ref:\n%s", out)
	}
}

func TestProjection_Compare_IncludesCarrierFootnote(t *testing.T) {
	graph := fixtureProjectionGraphWithRefs()
	out := ProjectionResponse(graph, artifact.ProjectionViewCompare)
	assertCarrierFootnoteWithRef(t, out, "sol-20260513-bbbb2222")
}

func TestProjection_RendersMissingRelationshipRefsAsUntitledArtifacts(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Context:     "fixture",
		GeneratedAt: time.Now().UTC(),
		Problems: []artifact.ProblemProjection{
			{
				Meta:          artifact.Meta{ID: "prob-1", Title: "Known problem", Kind: artifact.KindProblemCard, Status: artifact.StatusActive},
				PortfolioRefs: []string{"sol-missing"},
			},
		},
	}

	out := ProjectionResponse(graph, artifact.ProjectionViewEngineer)

	if !strings.Contains(out, "Portfolios: **untitled artifact** `sol-missing`") {
		t.Fatalf("missing projection refs should stay explicit, not bare:\n%s", out)
	}
	if strings.Contains(out, "Portfolios: `sol-missing`") {
		t.Fatalf("projection fallback should not render a bare ref:\n%s", out)
	}
}

func TestProjection_NoSources_StillRendersFootnoteWithInformationalNote(t *testing.T) {
	empty := artifact.ProjectionGraph{Context: "empty", GeneratedAt: time.Now().UTC()}
	out := ProjectionResponse(empty, artifact.ProjectionViewDelegatedAgent)

	if !strings.Contains(out, "Carrier — Not Source of Truth (A.15.4)") {
		t.Fatalf("empty projection still must render carrier footer:\n%s", out)
	}
	if !strings.Contains(out, "no underlying source artifacts; this projection is informational only") {
		t.Fatalf("empty projection must render the informational fallback line:\n%s", out)
	}
}

func TestProjection_FootnoteCitesRecoveryPath(t *testing.T) {
	graph := fixtureProjectionGraphWithRefs()
	out := ProjectionResponse(graph, artifact.ProjectionViewDelegatedAgent)
	if !strings.Contains(out, `haft_query(action="get"`) {
		t.Fatalf("footer must cite the haft_query recovery path:\n%s", out)
	}
	if !strings.Contains(out, ".haft/{decisions|problems|solutions|evidence}/") {
		t.Fatalf("footer must point at the on-disk source paths:\n%s", out)
	}
}

func assertCarrierFootnoteWithRef(t *testing.T, out, ref string) {
	t.Helper()
	if !strings.Contains(out, "## Carrier — Not Source of Truth (A.15.4)") {
		t.Fatalf("projection must render carrier footer:\n%s", out)
	}
	if !strings.Contains(out, ref) {
		t.Fatalf("projection footer must list ref %q:\n%s", ref, out)
	}
}
