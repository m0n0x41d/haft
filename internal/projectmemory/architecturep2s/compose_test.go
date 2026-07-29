package architecturep2s

import (
	"bytes"
	"slices"
	"testing"
)

func TestComposePreservesDistinctPositionsAndIsPermutationStable(t *testing.T) {
	basis := p2sTestBasis(t)
	concern := basis.EntityOfConcern()
	problem := p2sTestReference(t, "Haft.ProjectRecordRef", "problem:1")
	portfolio := p2sTestReference(t, "Haft.ProjectRecordRef", "portfolio:1")
	option := p2sTestReference(t, "Haft.ProjectRecordRef", "option:1")
	comparison := p2sTestReference(t, "Haft.ProjectRecordRef", "comparison:1")
	decision := p2sTestReference(t, "Haft.DecisionRecordRef", "decision:1")
	workRecord := p2sTestReference(t, "Haft.WorkRecordRef", "work-record:1")
	work := p2sTestReference(t, "Haft.PerformedWorkOccurrenceRef", "work:1")
	code := p2sTestReference(t, "Haft.CodeAnchorRef", "code:1")
	evidence := p2sTestReference(t, "Haft.EvidenceRecordRef", "evidence:1")
	claim := p2sTestReference(t, "Haft.ProjectClaimRef", "claim:1")
	claims := []ObservedClaim{
		p2sTestClaim(t, basis, "assertion:problem", "Haft.ProblemCardAtConcern", ClaimAffirmsObtaining, []Reference{concern, problem}),
		p2sTestClaim(t, basis, "assertion:portfolio", "Haft.SolutionPortfolioAtConcern", ClaimAffirmsObtaining, []Reference{concern, portfolio, option}),
		p2sTestClaim(t, basis, "assertion:comparison", "Haft.PortfolioComparison", ClaimAffirmsObtaining, []Reference{portfolio, comparison, option}),
		p2sTestClaim(t, basis, "assertion:decision", "Haft.DecisionChoiceAtConcern", ClaimAffirmsObtaining, []Reference{concern, decision, comparison}),
		p2sTestClaim(t, basis, "assertion:work-record", "Haft.WorkOccurrenceRecord", ClaimAffirmsObtaining, []Reference{concern, workRecord, work}),
		p2sTestClaim(t, basis, "assertion:work-change", "Haft.CodeChangedByWork", ClaimAffirmsObtaining, []Reference{work, code}),
		p2sTestClaim(t, basis, "assertion:evidence", "Haft.EvidenceUse", ClaimAffirmsObtaining, []Reference{work, evidence, claim}),
	}
	left, err := Compose(
		ComposeInput{Basis: basis, Claims: claims},
		HaftV9RuleSet(),
	)
	if err != nil {
		t.Fatalf("Compose(left): %v", err)
	}
	reversed := append([]ObservedClaim(nil), claims...)
	slices.Reverse(reversed)
	right, err := Compose(
		ComposeInput{Basis: basis, Claims: reversed},
		HaftV9RuleSet(),
	)
	if err != nil {
		t.Fatalf("Compose(right): %v", err)
	}
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) ||
		left.Digest() != right.Digest() {
		t.Fatal("architecture P2S read changed with non-semantic claim order")
	}

	assertP2SResolution(t, left, PositionAlternatives, ResolutionDirectClaim)
	assertP2SResolution(t, left, PositionComparison, ResolutionDirectClaim)
	assertP2SResolution(t, left, PositionDecision, ResolutionDirectClaim)
	assertP2SResolution(t, left, PositionWorkRecord, ResolutionDirectClaim)
	assertP2SResolution(t, left, PositionWorkToChange, ResolutionDirectClaim)
	assertP2SResolution(t, left, PositionEvidence, ResolutionDirectClaim)

	problemPressure := assertP2SMissing(t, left, PositionProblemPressure)
	if len(problemPressure.SourceDocks()) != 1 {
		t.Fatal("ProblemCard source dock was lost")
	}
	performedWork := assertP2SMissing(t, left, PositionPerformedWork)
	if len(performedWork.SourceDocks()) != 1 {
		t.Fatal("Work record did not remain a source dock for performed Work")
	}
	actualChange := assertP2SMissing(t, left, PositionActualChange)
	if len(actualChange.SourceDocks()) != 1 {
		t.Fatal("work-to-change claim did not remain a source dock for actual change")
	}
	assertP2SResolution(t, left, PositionProductionWork, ResolutionMissing)
	assertP2SResolution(t, left, PositionEntityInception, ResolutionMissing)
	assertP2SResolution(t, left, PositionProductionCompletion, ResolutionMissing)
	assertP2SResolution(t, left, PositionSelectedStructure, ResolutionMissing)
	assertP2SResolution(t, left, PositionActualStructure, ResolutionMissing)
	assertP2SResolution(t, left, PositionTargetEffect, ResolutionMissing)
}

func TestComposeKeepsUnknownDeniedAndLegacyDirectClaimsUnderdetermined(
	t *testing.T,
) {
	basis := p2sTestBasis(t)
	concern := basis.EntityOfConcern()
	comparison := p2sTestReference(
		t,
		"Haft.ProjectRecordRef",
		"comparison:1",
	)
	modalities := []ClaimModality{
		ClaimObtainingUnknown,
		ClaimDeniesObtaining,
		ClaimLegacyUnqualified,
	}
	for _, modality := range modalities {
		t.Run(string(modality), func(t *testing.T) {
			claim := p2sTestClaim(
				t,
				basis,
				"assertion:"+string(modality),
				"Haft.PortfolioComparison",
				modality,
				[]Reference{concern, comparison},
			)
			model, err := Compose(
				ComposeInput{Basis: basis, Claims: []ObservedClaim{claim}},
				HaftV9RuleSet(),
			)
			if err != nil {
				t.Fatalf("Compose(): %v", err)
			}
			position, found := model.Position(PositionComparison)
			if !found {
				t.Fatal("comparison position is missing")
			}
			underdetermined, ok := position.(UnderdeterminedPosition)
			if !ok || len(underdetermined.Candidates()) != 1 {
				t.Fatalf("comparison position = %#v", position)
			}
		})
	}
}

func TestComposeDoesNotBridgeConcernsThroughUnmappedCarrierRelations(
	t *testing.T,
) {
	basis := p2sTestBasis(t)
	firstConcern := basis.EntityOfConcern()
	secondConcern := p2sTestReference(t, "U.EntityRef", "system:other")
	firstPortfolio := p2sTestReference(
		t,
		"Haft.ProjectRecordRef",
		"portfolio:first",
	)
	secondPortfolio := p2sTestReference(
		t,
		"Haft.ProjectRecordRef",
		"portfolio:second",
	)
	carrier := p2sTestReference(
		t,
		"Haft.CarrierEditionRef",
		"carrier:shared",
	)
	claims := []ObservedClaim{
		p2sTestClaim(t, basis, "assertion:first", "Haft.SolutionPortfolioAtConcern", ClaimAffirmsObtaining, []Reference{firstConcern, firstPortfolio}),
		p2sTestClaim(t, basis, "assertion:second", "Haft.SolutionPortfolioAtConcern", ClaimAffirmsObtaining, []Reference{secondConcern, secondPortfolio}),
		p2sTestClaim(t, basis, "assertion:first-carrier", "Haft.CarrierPresentsRecord", ClaimAffirmsObtaining, []Reference{firstPortfolio, carrier}),
		p2sTestClaim(t, basis, "assertion:second-carrier", "Haft.CarrierPresentsRecord", ClaimAffirmsObtaining, []Reference{secondPortfolio, carrier}),
	}
	model, err := Compose(
		ComposeInput{Basis: basis, Claims: claims},
		HaftV9RuleSet(),
	)
	if err != nil {
		t.Fatalf("Compose(): %v", err)
	}
	position, found := model.Position(PositionAlternatives)
	if !found {
		t.Fatal("alternatives position is missing")
	}
	direct, ok := position.(DirectClaimPosition)
	if !ok || len(direct.Claims()) != 1 {
		t.Fatalf("alternatives position = %#v", position)
	}
	if direct.Claims()[0].AssertionID() != "assertion:first" {
		t.Fatal("unmapped shared-carrier relation broadened the concern")
	}
}

func TestComposeRejectsMixedReadCoordinatesAndConflictingNotApplicable(
	t *testing.T,
) {
	basis := p2sTestBasis(t)
	concern := basis.EntityOfConcern()
	comparison := p2sTestReference(
		t,
		"Haft.ProjectRecordRef",
		"comparison:1",
	)
	wrongContext := p2sTestClaim(
		t,
		basis,
		"assertion:wrong-context",
		"Haft.PortfolioComparison",
		ClaimAffirmsObtaining,
		[]Reference{concern, comparison},
	)
	wrongContext.context = "another-context"
	if _, err := Compose(
		ComposeInput{Basis: basis, Claims: []ObservedClaim{wrongContext}},
		HaftV9RuleSet(),
	); err == nil {
		t.Fatal("architecture P2S accepted a claim from another read coordinate")
	}

	direct := p2sTestClaim(
		t,
		basis,
		"assertion:direct",
		"Haft.PortfolioComparison",
		ClaimAffirmsObtaining,
		[]Reference{concern, comparison},
	)
	notApplicable, err := NewNotApplicableBasis(
		PositionComparison,
		"basis:not-applicable",
		"the comparison relation is explicitly out of scope",
	)
	if err != nil {
		t.Fatalf("NewNotApplicableBasis(): %v", err)
	}
	if _, err := Compose(
		ComposeInput{
			Basis:         basis,
			Claims:        []ObservedClaim{direct},
			NotApplicable: []NotApplicableBasis{notApplicable},
		},
		HaftV9RuleSet(),
	); err == nil {
		t.Fatal("architecture P2S accepted direct and not-applicable postures together")
	}
}

func p2sTestBasis(t *testing.T) ProjectionBasis {
	t.Helper()
	concern := p2sTestReference(t, "U.EntityRef", "system:haft")
	basis, err := NewProjectionBasis(ProjectionBasisInput{
		Project:         "qnt_1234abcd",
		EntityOfConcern: concern,
		Context:         "haft-project",
		TypeEnv:         "typeenv:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		GraphSnapshot:   "project-graph-snapshot:sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		GraphRevision:   42,
	})
	if err != nil {
		t.Fatalf("NewProjectionBasis(): %v", err)
	}
	return basis
}

func p2sTestReference(t *testing.T, kind string, id string) Reference {
	t.Helper()
	reference, err := NewReference(kind, id)
	if err != nil {
		t.Fatalf("NewReference(): %v", err)
	}
	return reference
}

func p2sTestClaim(
	t *testing.T,
	basis ProjectionBasis,
	assertionID string,
	signature string,
	modality ClaimModality,
	references []Reference,
) ObservedClaim {
	t.Helper()
	claim, err := NewObservedClaim(ObservedClaimInput{
		AssertionID: assertionID,
		Signature:   signature,
		Context:     basis.Context(),
		TypeEnv:     basis.TypeEnv(),
		Modality:    modality,
		Provenance:  "provenance:" + assertionID,
		OriginEvent: "event:" + assertionID,
		References:  references,
	})
	if err != nil {
		t.Fatalf("NewObservedClaim(): %v", err)
	}
	return claim
}

func assertP2SResolution(
	t *testing.T,
	model ReadModel,
	kind PositionKind,
	want ResolutionKind,
) Position {
	t.Helper()
	position, found := model.Position(kind)
	if !found || position.Resolution() != want {
		t.Fatalf(
			"position %s = (%T, %q), want %q",
			kind,
			position,
			positionResolution(position),
			want,
		)
	}
	return position
}

func assertP2SMissing(
	t *testing.T,
	model ReadModel,
	kind PositionKind,
) MissingPosition {
	t.Helper()
	position := assertP2SResolution(t, model, kind, ResolutionMissing)
	missing, ok := position.(MissingPosition)
	if !ok {
		t.Fatalf("position %s = %T, want MissingPosition", kind, position)
	}
	return missing
}

func positionResolution(position Position) ResolutionKind {
	if position == nil {
		return ""
	}
	return position.Resolution()
}
