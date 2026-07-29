package projecttypeenv

import (
	"bytes"
	"encoding/binary"
	"sort"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestLinkProjectTypeEnvCompositeIRResolvesExactBaseAndOwnProviders(t *testing.T) {
	base, artifact := compositeStandaloneFixture(t, "alpha.signature", "Alpha")

	ir := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	))
	if ir.BaseTypeEnvRef().String() != baseRef(t, base) {
		t.Fatalf("linked base = %q, want %q", ir.BaseTypeEnvRef().String(), baseRef(t, base))
	}
	if len(ir.Extensions()) != 1 {
		t.Fatalf("linked extension count = %d, want 1", len(ir.Extensions()))
	}
	assertCompositeDependency(t, ir, "kind:U.Entity", CompositeDependencyBase, "base")
	assertCompositeDependency(t, ir, "kind:U.ClaimGraph", CompositeDependencyBase, "base")
	assertCompositeDependency(t, ir, "kind:Alpha.ProjectConcern", CompositeDependencyOwn, "extension")
	assertCompositeDependency(
		t,
		ir,
		"slot_kind:Alpha.ConcernMemory/slot/Alpha.ConcernSlot",
		CompositeDependencyOwn,
		"extension",
	)
	if len(ir.CoverageGaps()) != 1 ||
		ir.CoverageGaps()[0].Code() != CompositeGapStratumDirectionUnresolved {
		t.Fatalf("coverage gaps = %#v; want one unresolved stratum-direction gap", ir.CoverageGaps())
	}
	if !hasCompositeExternalKind(ir.ExternalReferences(), CompositeExternalRule) {
		t.Fatal("source RuleRefs were not retained as explicit non-schema references")
	}
}

func TestCompositeLinkResourceLimitsAreClosedAndOverflowSafe(t *testing.T) {
	atLimit := make([]ProjectTypeEnvExtensionArtifact, maximumCompositeExtensionArtifacts)
	if issues := compositeInputResourceIssues(atLimit); len(issues) != 0 {
		t.Fatalf("extension-count boundary rejected: %#v", issues)
	}
	aboveLimit := make(
		[]ProjectTypeEnvExtensionArtifact,
		maximumCompositeExtensionArtifacts+1,
	)
	issues := compositeInputResourceIssues(aboveLimit)
	assertIssueListContains(t, issues, IssueCompositeResourceLimit)

	tests := []struct {
		name    string
		current uint64
		next    uint64
		limit   uint64
		want    uint64
		exceeds bool
	}{
		{
			name:  "canonical bytes exact boundary",
			next:  maximumCompositeExtensionBytes,
			limit: maximumCompositeExtensionBytes,
			want:  maximumCompositeExtensionBytes,
		},
		{
			name:    "canonical bytes above boundary",
			current: maximumCompositeExtensionBytes,
			next:    1,
			limit:   maximumCompositeExtensionBytes,
			want:    maximumCompositeExtensionBytes,
			exceeds: true,
		},
		{
			name:  "predecessor edges exact boundary",
			next:  maximumCompositePredecessorEdges,
			limit: maximumCompositePredecessorEdges,
			want:  maximumCompositePredecessorEdges,
		},
		{
			name:    "predecessor edges above boundary",
			current: maximumCompositePredecessorEdges,
			next:    1,
			limit:   maximumCompositePredecessorEdges,
			want:    maximumCompositePredecessorEdges,
			exceeds: true,
		},
		{
			name:    "integer overflow saturates",
			current: ^uint64(0),
			next:    1,
			limit:   maximumCompositeExtensionBytes,
			want:    maximumCompositeExtensionBytes,
			exceeds: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, exceeds := addCompositeResourceWithinLimit(
				test.current,
				test.next,
				test.limit,
			)
			if value != test.want || exceeds != test.exceeds {
				t.Fatalf("addCompositeResourceWithinLimit() = (%d, %t), want (%d, %t)", value, exceeds, test.want, test.exceeds)
			}
		})
	}
}

func TestLinkProjectTypeEnvCompositeIRExemptsCarrierRulePolicyEditionAndClaimsFromGhostChecks(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	source := string(carrierFixture(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
	))
	source = strings.Replace(
		source,
		"          kind: persisted_entities_only",
		"          kind: prior_batch_declarations_visible\n"+
			"          evaluation_rule: haft.rule.project-entities-prior-batch/v1",
		1,
	)
	source = strings.Replace(
		source,
		"        assumptions: []",
		"        assumptions:\n"+
			"          - carrier_ref: carrier:domain-model\n"+
			"            edition: 2.0.0\n"+
			"            digest: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		1,
	)
	carrier := parseCarrier(t, []byte(source))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	artifact := compileAndSealExtension(t, bundle.Nodes()[0], nil)

	ir := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	))
	if !hasCompositeExternalKind(ir.ExternalReferences(), CompositeExternalCarrier) {
		t.Fatal("CarrierRef was not retained outside schema-symbol ghost checks")
	}
	if !hasCompositeExternalKind(ir.ExternalReferences(), CompositeExternalRule) {
		t.Fatal("RuleRef was not retained outside schema-symbol ghost checks")
	}
	if !hasCompositeExternalKind(ir.ExternalReferences(), CompositeExternalClaim) {
		t.Fatal("source invariant/claim was not retained outside schema-symbol ghost checks")
	}
	assumptions := ir.CarrierAssumptions()
	if len(assumptions) != 1 {
		t.Fatalf("carrier assumptions = %#v, want one exact structured assumption", assumptions)
	}
	assumption := assumptions[0]
	if assumption.CarrierRef().Value() != "carrier:domain-model" ||
		assumption.Edition().Value() != "2.0.0" ||
		assumption.Digest().Value() != "sha256:"+strings.Repeat("b", 64) {
		t.Fatalf("carrier assumption lost exact identity triple: %#v", assumption)
	}
	if assumption.CarrierRef().Span().Start() == 0 ||
		assumption.Edition().Span().Start() == 0 ||
		assumption.Digest().Span().Start() == 0 {
		t.Fatalf("carrier assumption lost source spans: %#v", assumption)
	}
	if assumption.ConsumerRef() != artifact.Ref() ||
		assumption.ConsumerCoordinate().String() != "alpha.signature@1.0.0" {
		t.Fatalf("carrier assumption lost consumer provenance: %#v", assumption)
	}
}

func TestLinkProjectTypeEnvCompositeIRCanonicalizesCallerOrder(t *testing.T) {
	base := loadBaseArtifact(t)
	alphaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"alpha-project",
	))
	betaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
		"beta-project",
	))
	gammaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"gamma.signature",
		"1.0.0",
		"Gamma",
		nil,
		"gamma-project",
	))
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{betaCarrier, gammaCarrier, alphaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alpha},
	)
	gamma := compileAndSealExtension(t, nodes["gamma.signature@1.0.0"], nil)

	left := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, gamma, alpha},
	))
	right := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta, gamma},
	))
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("caller permutation changed linked composite IR bytes")
	}
	canonical := left.CanonicalBytes()
	if len(canonical) == 0 {
		t.Fatal("accepted composite IR has empty canonical bytes")
	}
	assertCompositeCanonicalDomain(t, canonical)
	mutated := left.CanonicalBytes()
	mutated[0] ^= 0xff
	if bytes.Equal(mutated, left.CanonicalBytes()) {
		t.Fatal("CanonicalBytes leaked mutable internal storage")
	}
	coordinates := compositeCoordinates(left.Extensions())
	want := []string{"alpha.signature@1.0.0", "beta.signature@1.0.0", "gamma.signature@1.0.0"}
	if strings.Join(coordinates, ",") != strings.Join(want, ",") {
		t.Fatalf("canonical coordinates = %v, want %v", coordinates, want)
	}
}

func TestCanonicalCompositeLinkIRBindsExactBaseAndPredecessor(t *testing.T) {
	base := loadBaseArtifact(t)
	alpha, beta := compositeImportedDependencyFixture(
		t,
		base,
		"Alpha.ProjectConcern",
		false,
	)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	))
	baseline := linked.CanonicalBytes()

	otherBaseDigest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("f", 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	otherBaseRef, err := typedmemory.NewTypeEnvRef(otherBaseDigest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	changedBase := cloneLinkedProjectTypeEnvCompositeIR(linked)
	changedBase.baseRef = otherBaseRef
	if bytes.Equal(baseline, canonicalCompositeLinkIR(changedBase)) {
		t.Fatal("changed exact base TypeEnvRef did not change composite canonical bytes")
	}

	changedPredecessor := cloneLinkedProjectTypeEnvCompositeIR(linked)
	predecessorIndex := -1
	for index := range changedPredecessor.extensions {
		if len(changedPredecessor.extensions[index].predecessors) > 0 {
			predecessorIndex = index
			break
		}
	}
	if predecessorIndex < 0 {
		t.Fatal("test fixture has no predecessor-bearing linked extension")
	}
	changedPredecessor.extensions[predecessorIndex].predecessors[0].ref =
		compositeTestExtensionRef(t, "alpha.signature", "f")
	if bytes.Equal(baseline, canonicalCompositeLinkIR(changedPredecessor)) {
		t.Fatal("changed exact predecessor E-ref did not change composite canonical bytes")
	}
}

func TestLinkProjectTypeEnvCompositeIRCanonicalBytesTrackExactExtension(t *testing.T) {
	base := loadBaseArtifact(t)
	firstSource := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil)
	secondSource := append(
		append([]byte(nil), firstSource...),
		[]byte("\n# changed exact source identity\n")...,
	)
	firstCarrier := parseCarrier(t, firstSource)
	secondCarrier := parseCarrier(t, secondSource)
	firstBundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{firstCarrier},
	)
	secondBundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{secondCarrier},
	)
	first := compileAndSealExtension(t, firstBundle.Nodes()[0], nil)
	second := compileAndSealExtension(t, secondBundle.Nodes()[0], nil)
	firstIR := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{first},
	))
	secondIR := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{second},
	))
	if bytes.Equal(firstIR.CanonicalBytes(), secondIR.CanonicalBytes()) {
		t.Fatal("changed exact E bytes did not change composite canonical bytes")
	}
}

func TestLinkProjectTypeEnvCompositeIRRejectsTamperBaseMismatchAndDuplicates(t *testing.T) {
	base, artifact := compositeStandaloneFixture(t, "alpha.signature", "Alpha")

	t.Run("tampered canonical bytes", func(t *testing.T) {
		forged := artifact
		forged.canonical = append([]byte(nil), artifact.canonical...)
		forged.canonical[len(forged.canonical)-1] ^= 0x01
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(base, []ProjectTypeEnvExtensionArtifact{forged}),
			IssueExtensionArtifactInvalid,
		)
	})

	t.Run("forged coordinate ref", func(t *testing.T) {
		forged := artifact
		forged.ref = compositeTestExtensionRef(t, "other.signature", "e")
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(base, []ProjectTypeEnvExtensionArtifact{forged}),
			IssueExtensionArtifactInvalid,
		)
	})

	t.Run("different exact base", func(t *testing.T) {
		digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat("f", 64))
		if err != nil {
			t.Fatalf("NewSHA256Digest() error = %v", err)
		}
		otherBase, err := typedmemory.NewTypeEnvRef(digest)
		if err != nil {
			t.Fatalf("NewTypeEnvRef() error = %v", err)
		}
		ir := artifact.IR()
		ir.baseTypeEnv = otherBase
		ir.baseSource.value = otherBase.String()
		mismatched := sealExtension(t, ir)
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(base, []ProjectTypeEnvExtensionArtifact{mismatched}),
			IssueBaseRefMismatch,
		)
	})

	t.Run("duplicate input", func(t *testing.T) {
		resolution := LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{artifact, artifact},
		)
		assertCompositeIssue(t, resolution, IssueDuplicateExtensionRef)
		for _, issue := range resolution.Issues() {
			if issue.Code() == IssueDuplicateSignatureID {
				t.Fatalf("same coordinate was mislabeled as version ambiguity: %#v", issue)
			}
		}
	})
}

func TestLinkProjectTypeEnvCompositeIRCanonicalizesDuplicateDiagnostics(t *testing.T) {
	base := loadBaseArtifact(t)
	firstSource := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil)
	secondSource := append(
		append([]byte(nil), firstSource...),
		[]byte("\n# exact-source variant\n")...,
	)
	firstCarrier := parseCarrier(t, firstSource)
	secondCarrier := parseCarrier(t, secondSource)
	firstBundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{firstCarrier},
	)
	secondBundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{secondCarrier},
	)
	first := compileAndSealExtension(t, firstBundle.Nodes()[0], nil)
	second := compileAndSealExtension(t, secondBundle.Nodes()[0], nil)
	if first.Ref() == second.Ref() {
		t.Fatal("changed exact source bytes retained the same E-ref")
	}

	left := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{first, second},
	)
	right := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{second, first},
	)
	if strings.Join(compositeIssueKeys(left.Issues()), "\n") !=
		strings.Join(compositeIssueKeys(right.Issues()), "\n") {
		t.Fatalf("duplicate diagnostics changed under caller permutation:\nleft=%#v\nright=%#v", left.Issues(), right.Issues())
	}
	assertCompositeIssue(t, left, IssueDuplicateManifestCoordinate)
	for _, issue := range left.Issues() {
		if issue.Code() == IssueDuplicateSignatureID {
			t.Fatalf("same coordinate was mislabeled as version ambiguity: %#v", issue)
		}
	}
}

func TestLinkProjectTypeEnvCompositeIRDistinguishesVersionAmbiguity(t *testing.T) {
	base := loadBaseArtifact(t)
	firstCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"AlphaV1",
		nil,
		"alpha-v1-project",
	))
	secondCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"2.0.0",
		"AlphaV2",
		nil,
		"alpha-v2-project",
	))
	firstBundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{firstCarrier},
	)
	secondBundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{secondCarrier},
	)
	first := compileAndSealExtension(t, firstBundle.Nodes()[0], nil)
	second := compileAndSealExtension(t, secondBundle.Nodes()[0], nil)

	left := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{first, second},
	)
	right := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{second, first},
	)
	assertCompositeIssueListsEqual(t, left.Issues(), right.Issues())
	assertCompositeIssue(t, left, IssueDuplicateSignatureID)
	if got := countIssueSubject(
		left.Issues(),
		IssueDuplicateSignatureID,
		"alpha.signature",
	); got != 2 {
		t.Fatalf("version-ambiguity issue count = %d, want one symmetric issue per version", got)
	}
	for _, issue := range left.Issues() {
		if issue.Code() == IssueDuplicateManifestCoordinate {
			t.Fatalf("different versions were mislabeled as one exact coordinate: %#v", issue)
		}
	}
}

func TestLinkProjectTypeEnvCompositeIRRejectsMissingPredecessor(t *testing.T) {
	base := loadBaseArtifact(t)
	alphaCarrier := parseCarrier(t, carrierFixture(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
	))
	betaCarrier := parseCarrier(t, carrierFixture(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
	))
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{alphaCarrier, betaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alpha},
	)
	assertCompositeIssue(
		t,
		LinkProjectTypeEnvCompositeIR(base, []ProjectTypeEnvExtensionArtifact{beta}),
		IssueMissingExtensionPredecessor,
	)
}

func TestLinkProjectTypeEnvCompositeIRRejectsPredecessorIdentityMismatches(t *testing.T) {
	base := loadBaseArtifact(t)

	t.Run("same coordinate with different exact E-ref", func(t *testing.T) {
		alphaSource := carrierFixtureInContext(
			t,
			base,
			"alpha.signature",
			"1.0.0",
			"Alpha",
			nil,
			"alpha-project",
		)
		betaSource := carrierFixtureInContext(
			t,
			base,
			"beta.signature",
			"1.0.0",
			"Beta",
			[]string{"alpha.signature"},
			"beta-project",
		)
		alphaCarrier := parseCarrier(t, alphaSource)
		betaCarrier := parseCarrier(t, betaSource)
		bundle := acceptedManifestBundle(
			t,
			base,
			[]localpractice.ParsedCarrier{alphaCarrier, betaCarrier},
		)
		nodes := nodesByCoordinate(bundle.Nodes())
		alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
		beta := compileAndSealExtension(
			t,
			nodes["beta.signature@1.0.0"],
			[]ProjectTypeEnvExtensionArtifact{alpha},
		)

		changedSource := append(
			append([]byte(nil), alphaSource...),
			[]byte("\n# changed exact predecessor bytes\n")...,
		)
		changedCarrier := parseCarrier(t, changedSource)
		changedBundle := acceptedManifestBundle(
			t,
			base,
			[]localpractice.ParsedCarrier{changedCarrier},
		)
		changedAlpha := compileAndSealExtension(t, changedBundle.Nodes()[0], nil)
		if changedAlpha.Ref() == alpha.Ref() {
			t.Fatal("changed exact predecessor retained the same E-ref")
		}
		left := LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{changedAlpha, beta},
		)
		right := LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{beta, changedAlpha},
		)
		assertCompositeIssueListsEqual(t, left.Issues(), right.Issues())
		assertCompositeIssue(
			t,
			left,
			IssueExtensionPredecessorMismatch,
		)
	})

	t.Run("same exact E-ref asserted with different coordinate", func(t *testing.T) {
		alphaRef := compositeTestExtensionRef(t, "alpha.signature", "a")
		betaRef := compositeTestExtensionRef(t, "beta.signature", "b")
		alphaCoordinate := newManifestCoordinate("alpha.signature", "1.0.0")
		betaCoordinate := newManifestCoordinate("beta.signature", "1.0.0")
		wrongCoordinate := newManifestCoordinate("alpha.signature", "2.0.0")
		alpha := compositeGraphTestNode(alphaCoordinate, alphaRef, nil)
		beta := compositeGraphTestNode(
			betaCoordinate,
			betaRef,
			[]compositeGraphTestPredecessor{{coordinate: wrongCoordinate, ref: alphaRef}},
		)
		_, leftIssues := canonicalCompositeDAG([]compositeExtensionNode{beta, alpha})
		_, rightIssues := canonicalCompositeDAG([]compositeExtensionNode{alpha, beta})
		assertCompositeIssueListsEqual(t, leftIssues, rightIssues)
		assertIssueListContains(t, leftIssues, IssueExtensionPredecessorMismatch)
	})
}

func TestCanonicalCompositeDAGRejectsSelfImportAndCycle(t *testing.T) {
	alphaRef := compositeTestExtensionRef(t, "alpha.signature", "a")
	betaRef := compositeTestExtensionRef(t, "beta.signature", "b")
	alphaCoordinate := newManifestCoordinate("alpha.signature", "1.0.0")
	betaCoordinate := newManifestCoordinate("beta.signature", "1.0.0")

	t.Run("self import", func(t *testing.T) {
		alpha := compositeGraphTestNode(
			alphaCoordinate,
			alphaRef,
			[]compositeGraphTestPredecessor{{coordinate: alphaCoordinate, ref: alphaRef}},
		)
		_, issues := canonicalCompositeDAG([]compositeExtensionNode{alpha})
		assertIssueListContains(t, issues, IssueSelfImport)
	})

	t.Run("cycle", func(t *testing.T) {
		alpha := compositeGraphTestNode(
			alphaCoordinate,
			alphaRef,
			[]compositeGraphTestPredecessor{{coordinate: betaCoordinate, ref: betaRef}},
		)
		beta := compositeGraphTestNode(
			betaCoordinate,
			betaRef,
			[]compositeGraphTestPredecessor{{coordinate: alphaCoordinate, ref: alphaRef}},
		)
		_, issues := canonicalCompositeDAG([]compositeExtensionNode{beta, alpha})
		assertIssueListContains(t, issues, IssueImportCycle)
	})

	t.Run("acyclic descendant of cycle is not a cycle member", func(t *testing.T) {
		gammaRef := compositeTestExtensionRef(t, "gamma.signature", "c")
		gammaCoordinate := newManifestCoordinate("gamma.signature", "1.0.0")
		alpha := compositeGraphTestNode(
			alphaCoordinate,
			alphaRef,
			[]compositeGraphTestPredecessor{{coordinate: betaCoordinate, ref: betaRef}},
		)
		beta := compositeGraphTestNode(
			betaCoordinate,
			betaRef,
			[]compositeGraphTestPredecessor{{coordinate: alphaCoordinate, ref: alphaRef}},
		)
		gamma := compositeGraphTestNode(
			gammaCoordinate,
			gammaRef,
			[]compositeGraphTestPredecessor{{coordinate: alphaCoordinate, ref: alphaRef}},
		)
		_, leftIssues := canonicalCompositeDAG(
			[]compositeExtensionNode{gamma, beta, alpha},
		)
		_, rightIssues := canonicalCompositeDAG(
			[]compositeExtensionNode{alpha, gamma, beta},
		)
		leftKeys := compositeIssueKeys(leftIssues)
		rightKeys := compositeIssueKeys(rightIssues)
		if strings.Join(leftKeys, "\n") != strings.Join(rightKeys, "\n") {
			t.Fatalf(
				"cycle diagnostics changed under caller permutation:\nleft=%#v\nright=%#v",
				leftIssues,
				rightIssues,
			)
		}
		cycleSubjects := issueSubjects(leftIssues, IssueImportCycle)
		want := []string{alphaRef.String(), betaRef.String()}
		if strings.Join(cycleSubjects, "\n") != strings.Join(want, "\n") {
			t.Fatalf("cycle subjects = %v, want exact cycle members %v", cycleSubjects, want)
		}
	})
}

func TestLinkProjectTypeEnvCompositeIRRejectsTransitiveAndBranchRedeclarations(t *testing.T) {
	base := loadBaseArtifact(t)

	t.Run("transitive import redeclaration", func(t *testing.T) {
		alphaCarrier := parseCarrier(t, carrierFixture(
			t,
			base,
			"alpha.signature",
			"1.0.0",
			"Shared",
			nil,
		))
		betaCarrier := parseCarrier(t, carrierFixture(
			t,
			base,
			"beta.signature",
			"1.0.0",
			"Shared",
			[]string{"alpha.signature"},
		))
		bundle := acceptedManifestBundle(
			t,
			base,
			[]localpractice.ParsedCarrier{alphaCarrier, betaCarrier},
		)
		nodes := nodesByCoordinate(bundle.Nodes())
		alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
		beta := compileAndSealExtension(
			t,
			nodes["beta.signature@1.0.0"],
			[]ProjectTypeEnvExtensionArtifact{alpha},
		)
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(
				base,
				[]ProjectTypeEnvExtensionArtifact{beta, alpha},
			),
			IssueTransitiveSymbolRedeclaration,
		)
	})

	t.Run("unrelated branch conflict", func(t *testing.T) {
		alphaCarrier := parseCarrier(t, carrierFixture(
			t,
			base,
			"alpha.signature",
			"1.0.0",
			"Shared",
			nil,
		))
		gammaCarrier := parseCarrier(t, carrierFixture(
			t,
			base,
			"gamma.signature",
			"1.0.0",
			"Shared",
			nil,
		))
		bundle := acceptedManifestBundle(
			t,
			base,
			[]localpractice.ParsedCarrier{gammaCarrier, alphaCarrier},
		)
		nodes := nodesByCoordinate(bundle.Nodes())
		alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
		gamma := compileAndSealExtension(t, nodes["gamma.signature@1.0.0"], nil)
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(
				base,
				[]ProjectTypeEnvExtensionArtifact{gamma, alpha},
			),
			IssueBranchSymbolConflict,
		)
	})
}

func TestLinkProjectTypeEnvCompositeIRScopesSlotKindIdentityToItsRelation(t *testing.T) {
	base := loadBaseArtifact(t)
	alphaSource := carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"alpha-project",
	)
	alphaSource = bytes.ReplaceAll(
		alphaSource,
		[]byte("Alpha.ConcernSlot"),
		[]byte("SharedSlot"),
	)
	betaSource := carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		nil,
		"beta-project",
	)
	betaSource = bytes.ReplaceAll(
		betaSource,
		[]byte("Beta.ConcernSlot"),
		[]byte("SharedSlot"),
	)
	alphaCarrier := parseCarrier(t, alphaSource)
	betaCarrier := parseCarrier(t, betaSource)
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{betaCarrier, alphaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(t, nodes["beta.signature@1.0.0"], nil)

	ir := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	))
	assertCompositeDependency(
		t,
		ir,
		"slot_kind:Alpha.ConcernMemory/slot/SharedSlot",
		CompositeDependencyOwn,
		"extension",
	)
	assertCompositeDependency(
		t,
		ir,
		"slot_kind:Beta.ConcernMemory/slot/SharedSlot",
		CompositeDependencyOwn,
		"extension",
	)
}

func TestLinkProjectTypeEnvCompositeIRDoesNotReportDifferentRelationSlotAsKindMismatch(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	alphaSource := carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"alpha-project",
	)
	alphaSource = bytes.Replace(
		alphaSource,
		[]byte("          slot: Alpha.ConcernSlot"),
		[]byte("          slot: SharedSlot"),
		1,
	)
	betaSource := carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		nil,
		"beta-project",
	)
	betaSource = bytes.ReplaceAll(
		betaSource,
		[]byte("Beta.ConcernSlot"),
		[]byte("SharedSlot"),
	)
	alphaCarrier := parseCarrier(t, alphaSource)
	betaCarrier := parseCarrier(t, betaSource)
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{betaCarrier, alphaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(t, nodes["beta.signature@1.0.0"], nil)

	resolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	)
	assertCompositeIssue(t, resolution, IssueGhostDependency)
	for _, issue := range resolution.Issues() {
		if issue.Code() == IssueDependencyKindMismatch {
			t.Fatalf("different relation-local SlotKind produced kind mismatch: %#v", issue)
		}
	}
}

func TestLinkProjectTypeEnvCompositeIRRejectsBaseSymbolRedeclaration(t *testing.T) {
	base := loadBaseArtifact(t)
	source := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil)
	source = bytes.ReplaceAll(source, []byte("Alpha.ProjectConcern"), []byte("U.Entity"))
	carrier := parseCarrier(t, source)
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	artifact := compileAndSealExtension(t, bundle.Nodes()[0], nil)
	assertCompositeIssue(
		t,
		LinkProjectTypeEnvCompositeIR(base, []ProjectTypeEnvExtensionArtifact{artifact}),
		IssueBaseSymbolRedeclaration,
	)
}

func TestLinkProjectTypeEnvCompositeIRRejectsSemanticEffectAliases(t *testing.T) {
	base := loadBaseArtifact(t)

	t.Run("distinct EntitySet names cannot conceal one context coordinate", func(t *testing.T) {
		alphaCarrier := parseCarrier(t, carrierFixture(
			t,
			base,
			"alpha.signature",
			"1.0.0",
			"Alpha",
			nil,
		))
		betaCarrier := parseCarrier(t, carrierFixture(
			t,
			base,
			"beta.signature",
			"1.0.0",
			"Beta",
			nil,
		))
		bundle := acceptedManifestBundle(
			t,
			base,
			[]localpractice.ParsedCarrier{alphaCarrier, betaCarrier},
		)
		nodes := nodesByCoordinate(bundle.Nodes())
		alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
		beta := compileAndSealExtension(t, nodes["beta.signature@1.0.0"], nil)
		left := LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{beta, alpha},
		)
		right := LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{alpha, beta},
		)
		leftKeys := compositeIssueKeys(left.Issues())
		rightKeys := compositeIssueKeys(right.Issues())
		if strings.Join(leftKeys, "\n") != strings.Join(rightKeys, "\n") {
			t.Fatalf(
				"EntitySet conflict diagnostics changed under caller permutation:\nleft=%#v\nright=%#v",
				left.Issues(),
				right.Issues(),
			)
		}
		assertCompositeIssue(
			t,
			left,
			IssueSemanticEffectConflict,
		)
		assertCompositeIssueSubject(
			t,
			left,
			IssueSemanticEffectConflict,
			"entity_set\x00haft-project",
		)
		if got := countIssueSubject(
			left.Issues(),
			IssueSemanticEffectConflict,
			"entity_set\x00haft-project",
		); got != 2 {
			t.Fatalf("EntitySet semantic-coordinate issue count = %d, want 2", got)
		}
	})

	t.Run("distinct KindSignature names cannot conceal one kind and context coordinate", func(t *testing.T) {
		alphaSource := carrierFixtureInContext(
			t,
			base,
			"alpha.signature",
			"1.0.0",
			"Alpha",
			nil,
			"shared-project",
		)
		betaSource := carrierFixtureInContext(
			t,
			base,
			"beta.signature",
			"1.0.0",
			"Beta",
			[]string{"alpha.signature"},
			"shared-project",
		)
		old := []byte(
			"symbol: Beta.ProjectConcern.Signature\n        value_kind: Beta.ProjectConcern",
		)
		replacement := []byte(
			"symbol: Beta.ProjectConcern.Signature\n        value_kind: Alpha.ProjectConcern",
		)
		if !bytes.Contains(betaSource, old) {
			t.Fatal("test fixture kind-signature value kind was not found")
		}
		betaSource = bytes.Replace(betaSource, old, replacement, 1)
		alphaCarrier := parseCarrier(t, alphaSource)
		betaCarrier := parseCarrier(t, betaSource)
		bundle := acceptedManifestBundle(
			t,
			base,
			[]localpractice.ParsedCarrier{betaCarrier, alphaCarrier},
		)
		nodes := nodesByCoordinate(bundle.Nodes())
		alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
		beta := compileAndSealExtension(
			t,
			nodes["beta.signature@1.0.0"],
			[]ProjectTypeEnvExtensionArtifact{alpha},
		)
		left := LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{beta, alpha},
		)
		right := LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{alpha, beta},
		)
		leftKeys := compositeIssueKeys(left.Issues())
		rightKeys := compositeIssueKeys(right.Issues())
		if strings.Join(leftKeys, "\n") != strings.Join(rightKeys, "\n") {
			t.Fatalf(
				"KindSignature conflict diagnostics changed under caller permutation:\nleft=%#v\nright=%#v",
				left.Issues(),
				right.Issues(),
			)
		}
		assertCompositeIssueSubject(
			t,
			left,
			IssueSemanticEffectConflict,
			"kind_signature\x00Alpha.ProjectConcern\x00shared-project",
		)
		if got := countIssueSubject(
			left.Issues(),
			IssueSemanticEffectConflict,
			"kind_signature\x00Alpha.ProjectConcern\x00shared-project",
		); got != 2 {
			t.Fatalf("KindSignature semantic-coordinate issue count = %d, want 2", got)
		}
	})

	t.Run("different codec name cannot shadow exact base kind binding", func(t *testing.T) {
		source := carrierFixtureInContext(
			t,
			base,
			"alpha.signature",
			"1.0.0",
			"Alpha",
			nil,
			"alpha-project",
		)
		old := []byte("symbol: Alpha.Codec.Text\n        value_kind: Alpha.ProjectConcern")
		replacement := []byte("symbol: Alpha.Codec.Text\n        value_kind: U.ClaimGraph")
		if !bytes.Contains(source, old) {
			t.Fatal("test fixture codec binding was not found")
		}
		source = bytes.Replace(source, old, replacement, 1)
		carrier := parseCarrier(t, source)
		bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
		artifact := compileAndSealExtension(t, bundle.Nodes()[0], nil)
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(
				base,
				[]ProjectTypeEnvExtensionArtifact{artifact},
			),
			IssueSemanticEffectConflict,
		)
	})
}

func TestLinkProjectTypeEnvCompositeIRLinksKindBridgeWithExactSchemaDependencies(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	artifact := compositeKindBridgeArtifact(
		t,
		base,
		"alpha.bridge",
		"Alpha.Bridge.EntityToEntity",
		"U.Entity",
		"U.Entity",
	)

	ir := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	))
	extension := ir.Extensions()[0]
	provided := extension.Provides()
	if len(provided) != 1 ||
		provided[0].Kind() != typedmemory.BridgeSymbol ||
		provided[0].Key() != "Alpha.Bridge.EntityToEntity" {
		t.Fatalf("KindBridge provided symbols = %#v", provided)
	}
	for _, role := range []string{"mapping.source_kind", "mapping.target_kind"} {
		dependency := findCompositeDependency(
			t,
			ir,
			"declaration:Alpha.Bridge.EntityToEntity",
			role,
		)
		if dependency.Target().String() != "kind:U.Entity" ||
			dependency.Scope() != CompositeDependencyBase ||
			dependency.Provider().ProviderKind() != "base" {
			t.Fatalf("KindBridge dependency %q = %#v", role, dependency)
		}
	}
}

func TestLinkProjectTypeEnvCompositeIRRejectsKindBridgeSemanticEffectConflict(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	first := compositeKindBridgeArtifact(
		t,
		base,
		"alpha.bridge",
		"Shared.Bridge.EntityToEntity",
		"U.Entity",
		"U.Entity",
	)
	second := compositeKindBridgeArtifact(
		t,
		base,
		"beta.bridge",
		"Shared.Bridge.EntityToEntity",
		"U.Entity",
		"U.Entity",
	)

	resolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{second, first},
	)
	assertCompositeIssue(t, resolution, IssueSemanticEffectConflict)
}

func TestLinkProjectTypeEnvCompositeIRRejectsKindBridgeGhostKind(t *testing.T) {
	base := loadBaseArtifact(t)
	artifact := compositeKindBridgeArtifact(
		t,
		base,
		"alpha.bridge",
		"Alpha.Bridge.EntityToGhost",
		"U.Entity",
		"Ghost.Kind",
	)

	resolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	)
	assertCompositeIssue(t, resolution, IssueGhostDependency)
}

func TestLinkProjectTypeEnvCompositeIRCanonicalizesKindBridgePermutation(t *testing.T) {
	base := loadBaseArtifact(t)
	alpha := compositeKindBridgeArtifact(
		t,
		base,
		"alpha.bridge",
		"Alpha.Bridge.EntityToEntity",
		"U.Entity",
		"U.Entity",
	)
	beta := compositeKindBridgeArtifact(
		t,
		base,
		"beta.bridge",
		"Beta.Bridge.EntityToEntity",
		"U.Entity",
		"U.Entity",
	)

	left := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	))
	right := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta},
	))
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("KindBridge caller permutation changed linked composite IR bytes")
	}
}

func TestLinkProjectTypeEnvCompositeIRLinksBoundedContextAndSubkind(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	artifact := compositeContextSubkindArtifact(
		t,
		base,
		"alpha.context",
		"Alpha",
		"alpha-project",
		"Alpha.ProjectConcern",
		"U.Entity",
	)

	ir := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	))
	provided := ir.Extensions()[0].Provides()
	if !hasProvidedSymbol(provided, typedmemory.ContextSymbol, "alpha-project") {
		t.Fatalf("bounded-context symbol is absent from provides: %#v", provided)
	}
	if hasProvidedKey(provided, "Alpha.Subkind.ProjectConcernEntity") {
		t.Fatalf("subkind declaration identity leaked into provides: %#v", provided)
	}
	child := findCompositeDependency(
		t,
		ir,
		"declaration:Alpha.Subkind.ProjectConcernEntity",
		"child_kind",
	)
	if child.Target().String() != "kind:Alpha.ProjectConcern" ||
		child.Scope() != CompositeDependencyOwn {
		t.Fatalf("subkind child dependency = %#v", child)
	}
	super := findCompositeDependency(
		t,
		ir,
		"declaration:Alpha.Subkind.ProjectConcernEntity",
		"super_kind",
	)
	if super.Target().String() != "kind:U.Entity" ||
		super.Scope() != CompositeDependencyBase {
		t.Fatalf("subkind super dependency = %#v", super)
	}
}

func TestLinkProjectTypeEnvCompositeIRRejectsBoundedContextSemanticConflict(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	alpha := compositeContextSubkindArtifact(
		t,
		base,
		"alpha.context",
		"Alpha",
		"shared-project",
		"Alpha.ProjectConcern",
		"U.Entity",
	)
	beta := compositeContextSubkindArtifact(
		t,
		base,
		"beta.context",
		"Beta",
		"shared-project",
		"Beta.ProjectConcern",
		"U.Entity",
	)

	resolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	)
	assertCompositeIssue(t, resolution, IssueSemanticEffectConflict)
	assertCompositeIssueSubject(
		t,
		resolution,
		IssueSemanticEffectConflict,
		"bounded_context\x00shared-project",
	)
}

func TestLinkProjectTypeEnvCompositeIRRejectsSubkindSemanticConflict(t *testing.T) {
	base := loadBaseArtifact(t)
	alpha := compositeContextSubkindArtifact(
		t,
		base,
		"alpha.context",
		"Alpha",
		"alpha-project",
		"U.Holon",
		"U.Entity",
	)
	beta := compositeContextSubkindArtifact(
		t,
		base,
		"beta.context",
		"Beta",
		"beta-project",
		"U.Holon",
		"U.Entity",
	)

	resolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta},
	)
	assertCompositeIssue(t, resolution, IssueSemanticEffectConflict)
	assertCompositeIssueSubject(
		t,
		resolution,
		IssueSemanticEffectConflict,
		"subkind\x00U.Holon\x00U.Entity",
	)
}

func TestLinkProjectTypeEnvCompositeIRRejectsSubkindGhostKind(t *testing.T) {
	base := loadBaseArtifact(t)
	artifact := compositeContextSubkindArtifact(
		t,
		base,
		"alpha.context",
		"Alpha",
		"alpha-project",
		"Alpha.ProjectConcern",
		"Ghost.Kind",
	)

	resolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{artifact},
	)
	assertCompositeIssue(t, resolution, IssueGhostDependency)
	assertCompositeIssueSubject(t, resolution, IssueGhostDependency, "kind:Ghost.Kind")
}

func TestLinkProjectTypeEnvCompositeIRCanonicalizesContextSubkindPermutation(
	t *testing.T,
) {
	base := loadBaseArtifact(t)
	alpha := compositeContextSubkindArtifact(
		t,
		base,
		"alpha.context",
		"Alpha",
		"alpha-project",
		"Alpha.ProjectConcern",
		"U.Entity",
	)
	beta := compositeContextSubkindArtifact(
		t,
		base,
		"beta.context",
		"Beta",
		"beta-project",
		"Beta.ProjectConcern",
		"U.Entity",
	)

	left := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	))
	right := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta},
	))
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("bounded-context/subkind caller permutation changed linked bytes")
	}
}

func TestLinkProjectTypeEnvCompositeIRDistinguishesImportedGhostAndSiblingDependencies(t *testing.T) {
	base := loadBaseArtifact(t)

	t.Run("imported provider", func(t *testing.T) {
		alpha, beta := compositeImportedDependencyFixture(t, base, "Alpha.ProjectConcern", false)
		ir := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{beta, alpha},
		))
		resolution := findCompositeDependency(
			t,
			ir,
			"declaration:Beta.ProjectConcernRef",
			"value_kind",
		)
		if resolution.Scope() != CompositeDependencyImported {
			t.Fatalf("imported dependency scope = %q", resolution.Scope())
		}
		provider, ok := resolution.Provider().(ExtensionCompositeSymbolProvider)
		if !ok || provider.Coordinate().ID() != "alpha.signature" {
			t.Fatalf("imported provider = %#v, want alpha.signature", resolution.Provider())
		}
	})

	t.Run("ghost", func(t *testing.T) {
		alpha, beta := compositeImportedDependencyFixture(t, base, "Missing.ProjectConcern", false)
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(
				base,
				[]ProjectTypeEnvExtensionArtifact{beta, alpha},
			),
			IssueGhostDependency,
		)
	})

	t.Run("non-imported sibling", func(t *testing.T) {
		alpha, beta := compositeImportedDependencyFixture(t, base, "Gamma.ProjectConcern", true)
		gammaCarrier := parseCarrier(t, carrierFixture(
			t,
			base,
			"gamma.signature",
			"1.0.0",
			"Gamma",
			nil,
		))
		gammaBundle := acceptedManifestBundle(
			t,
			base,
			[]localpractice.ParsedCarrier{gammaCarrier},
		)
		gamma := compileAndSealExtension(t, gammaBundle.Nodes()[0], nil)
		assertCompositeIssue(
			t,
			LinkProjectTypeEnvCompositeIR(
				base,
				[]ProjectTypeEnvExtensionArtifact{beta, gamma, alpha},
			),
			IssueNonImportedSiblingDependency,
		)
	})
}

func compositeKindBridgeArtifact(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	carrierID string,
	bridgeID string,
	sourceKind string,
	targetKind string,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	source := kindBridgeSourceForBase(t, base, "one_way")
	replacements := []struct {
		from string
		to   string
	}{
		{from: "haft.auth-bridge", to: carrierID},
		{from: "subject_kind: U.KindBridge", to: "subject_kind: U.Entity"},
		{from: "ranged_value_kind: U.Kind", to: "ranged_value_kind: U.Entity"},
		{
			from: "Haft.Bridge.AuthenticatedRequestToFrontendRequest",
			to:   bridgeID,
		},
		{from: "Auth.AuthenticatedRequest", to: sourceKind},
		{from: "Frontend.VerifiedRequest", to: targetKind},
	}
	for _, replacement := range replacements {
		if !bytes.Contains(source, []byte(replacement.from)) {
			t.Fatalf("KindBridge fixture has no %q", replacement.from)
		}
		source = bytes.ReplaceAll(
			source,
			[]byte(replacement.from),
			[]byte(replacement.to),
		)
	}
	carrier := parseCarrier(t, source)
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	return compileAndSealExtension(t, bundle.Nodes()[0], nil)
}

func compositeContextSubkindArtifact(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	carrierID string,
	prefix string,
	context string,
	childKind string,
	superKind string,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	source := string(contextSubkindExtensionSource(t, base, prefix))
	replacements := []struct {
		from string
		to   string
	}{
		{from: "context.signature", to: carrierID},
		{from: "haft-project", to: context},
		{from: "child_kind: " + prefix + ".ProjectConcern", to: "child_kind: " + childKind},
		{from: "super_kind: U.Entity", to: "super_kind: " + superKind},
	}
	for _, replacement := range replacements {
		if !strings.Contains(source, replacement.from) {
			t.Fatalf("context/subkind fixture has no %q", replacement.from)
		}
		source = strings.ReplaceAll(source, replacement.from, replacement.to)
	}
	carrier := parseCarrier(t, []byte(source))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	return compileAndSealExtension(t, bundle.Nodes()[0], nil)
}

func hasProvidedSymbol(
	provided []typedmemory.SchemaSymbolRef,
	kind typedmemory.SchemaSymbolKind,
	key string,
) bool {
	for _, symbol := range provided {
		if symbol.Kind() == kind && symbol.Key() == key {
			return true
		}
	}
	return false
}

func hasProvidedKey(provided []typedmemory.SchemaSymbolRef, key string) bool {
	for _, symbol := range provided {
		if symbol.Key() == key {
			return true
		}
	}
	return false
}

func compositeStandaloneFixture(
	t *testing.T,
	id string,
	prefix string,
) (typeenv.BaseTypeEnvArtifact, ProjectTypeEnvExtensionArtifact) {
	t.Helper()
	base := loadBaseArtifact(t)
	carrier := parseCarrier(t, carrierFixture(t, base, id, "1.0.0", prefix, nil))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	artifact := compileAndSealExtension(t, bundle.Nodes()[0], nil)
	return base, artifact
}

func carrierFixtureInContext(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	id string,
	version string,
	prefix string,
	imports []string,
	context string,
) []byte {
	t.Helper()
	source := carrierFixture(t, base, id, version, prefix, imports)
	old := []byte("bounded_context_ref: haft-project")
	replacement := []byte("bounded_context_ref: " + context)
	if !bytes.Contains(source, old) {
		t.Fatal("test fixture bounded_context_ref was not found")
	}
	return bytes.ReplaceAll(source, old, replacement)
}

func compositeImportedDependencyFixture(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	target string,
	includeGammaInSourceResolution bool,
) (ProjectTypeEnvExtensionArtifact, ProjectTypeEnvExtensionArtifact) {
	t.Helper()
	alphaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"alpha-project",
	))
	betaSource := carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
		"beta-project",
	)
	betaSource = replaceCompositeRefKindValue(t, betaSource, target)
	betaCarrier := parseCarrier(t, betaSource)
	carriers := []localpractice.ParsedCarrier{alphaCarrier, betaCarrier}
	if includeGammaInSourceResolution {
		gammaCarrier := parseCarrier(t, carrierFixtureInContext(
			t,
			base,
			"gamma.signature",
			"1.0.0",
			"Gamma",
			nil,
			"gamma-project",
		))
		carriers = append(carriers, gammaCarrier)
	}
	bundle := acceptedManifestBundle(t, base, carriers)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alpha},
	)
	return alpha, beta
}

func replaceCompositeRefKindValue(
	t *testing.T,
	source []byte,
	target string,
) []byte {
	t.Helper()
	old := []byte("symbol: Beta.ProjectConcernRef\n        value_kind: Beta.ProjectConcern")
	replacement := []byte("symbol: Beta.ProjectConcernRef\n        value_kind: " + target)
	if !bytes.Contains(source, old) {
		t.Fatal("test fixture ref-kind declaration was not found")
	}
	return bytes.Replace(source, old, replacement, 1)
}

type compositeGraphTestPredecessor struct {
	coordinate ManifestCoordinate
	ref        typedmemory.TypeEnvExtensionRef
}

func compositeGraphTestNode(
	coordinate ManifestCoordinate,
	ref typedmemory.TypeEnvExtensionRef,
	predecessors []compositeGraphTestPredecessor,
) compositeExtensionNode {
	resolved := make([]ResolvedExtensionPredecessor, 0, len(predecessors))
	for _, predecessor := range predecessors {
		resolved = append(resolved, ResolvedExtensionPredecessor{
			coordinate: predecessor.coordinate,
			ref:        predecessor.ref,
			source: SourceScalar{
				value: predecessor.coordinate.ID(),
				span:  SourceSpan{start: 1, end: 1},
			},
		})
	}
	ir := ProjectTypeEnvExtensionIR{
		manifest: ProjectSignatureManifestIR{
			coordinate:   coordinate,
			predecessors: resolved,
			span:         SourceSpan{start: 1, end: 1},
		},
	}
	return compositeExtensionNode{
		ref:        ref,
		coordinate: coordinate,
		ir:         ir,
		ancestors:  make(map[string]struct{}),
	}
}

func compositeTestExtensionRef(
	t *testing.T,
	id string,
	fill string,
) typedmemory.TypeEnvExtensionRef {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvExtensionRef(
		"typeenv-extension:" + id + "@sha256:" + strings.Repeat(fill, 64),
	)
	if err != nil {
		t.Fatalf("ParseTypeEnvExtensionRef() error = %v", err)
	}
	return ref
}

func acceptedCompositeIR(
	t *testing.T,
	resolution CompositeIRLinkResolution,
) LinkedProjectTypeEnvCompositeIR {
	t.Helper()
	if resolution.Rejected() {
		t.Fatalf("LinkProjectTypeEnvCompositeIR() rejected: %#v", resolution.Issues())
	}
	ir, exists := resolution.CompositeIR()
	if !exists {
		t.Fatal("accepted composite link has no IR")
	}
	return ir
}

func assertCompositeIssue(
	t *testing.T,
	resolution CompositeIRLinkResolution,
	want LinkIssueCode,
) {
	t.Helper()
	if !resolution.Rejected() {
		t.Fatalf("LinkProjectTypeEnvCompositeIR() accepted; want %q", want)
	}
	assertIssueListContains(t, resolution.Issues(), want)
}

func assertIssueListContains(t *testing.T, issues []LinkIssue, want LinkIssueCode) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code() == want {
			return
		}
	}
	t.Fatalf("issues = %#v; want code %q", issues, want)
}

func assertCompositeIssueSubject(
	t *testing.T,
	resolution CompositeIRLinkResolution,
	code LinkIssueCode,
	subject string,
) {
	t.Helper()
	for _, issue := range resolution.Issues() {
		if issue.Code() == code && issue.Subject() == subject {
			return
		}
	}
	t.Fatalf("issues = %#v; want %q subject %q", resolution.Issues(), code, subject)
}

func assertCompositeDependency(
	t *testing.T,
	ir LinkedProjectTypeEnvCompositeIR,
	target string,
	scope CompositeDependencyScope,
	providerKind string,
) {
	t.Helper()
	for _, dependency := range ir.DependencyResolutions() {
		if dependency.Target().String() != target {
			continue
		}
		if dependency.Scope() != scope {
			t.Fatalf("dependency %q scope = %q, want %q", target, dependency.Scope(), scope)
		}
		if dependency.Provider().ProviderKind() != providerKind {
			t.Fatalf(
				"dependency %q provider = %q, want %q",
				target,
				dependency.Provider().ProviderKind(),
				providerKind,
			)
		}
		return
	}
	t.Fatalf("dependency target %q was not resolved", target)
}

func findCompositeDependency(
	t *testing.T,
	ir LinkedProjectTypeEnvCompositeIR,
	origin string,
	role string,
) CompositeDependencyResolution {
	t.Helper()
	for _, dependency := range ir.DependencyResolutions() {
		if dependency.Origin() == origin && dependency.Role() == role {
			return dependency
		}
	}
	t.Fatalf("dependency %s/%s was not found", origin, role)
	return CompositeDependencyResolution{}
}

func hasCompositeExternalKind(
	references []CompositeExternalReference,
	kind CompositeExternalReferenceKind,
) bool {
	for _, reference := range references {
		if reference.Kind() == kind {
			return true
		}
	}
	return false
}

func compositeIssueKeys(issues []LinkIssue) []string {
	result := make([]string, 0, len(issues))
	for _, issue := range issues {
		result = append(result, string(issue.Code())+"\x00"+
			issue.Location().String()+"\x00"+
			issue.Subject()+"\x00"+
			issue.Detail()+"\x00"+
			issue.Repair())
	}
	return result
}

func assertCompositeIssueListsEqual(t *testing.T, left []LinkIssue, right []LinkIssue) {
	t.Helper()
	leftKeys := compositeIssueKeys(left)
	rightKeys := compositeIssueKeys(right)
	if strings.Join(leftKeys, "\n") != strings.Join(rightKeys, "\n") {
		t.Fatalf(
			"link diagnostics changed under caller permutation:\nleft=%#v\nright=%#v",
			left,
			right,
		)
	}
}

func countIssueSubject(
	issues []LinkIssue,
	code LinkIssueCode,
	subject string,
) int {
	count := 0
	for _, issue := range issues {
		if issue.Code() == code && issue.Subject() == subject {
			count++
		}
	}
	return count
}

func issueSubjects(issues []LinkIssue, code LinkIssueCode) []string {
	result := make([]string, 0)
	for _, issue := range issues {
		if issue.Code() == code {
			result = append(result, issue.Subject())
		}
	}
	sort.Strings(result)
	return result
}

func assertCompositeCanonicalDomain(t *testing.T, canonical []byte) {
	t.Helper()
	if len(canonical) < 8 {
		t.Fatalf("composite canonical bytes are shorter than a domain length prefix: %d", len(canonical))
	}
	domainLength := binary.BigEndian.Uint64(canonical[:8])
	if domainLength != uint64(len(compositeLinkCanonicalDomain)) {
		t.Fatalf(
			"composite canonical domain length = %d, want %d",
			domainLength,
			len(compositeLinkCanonicalDomain),
		)
	}
	domainEnd := 8 + int(domainLength)
	if domainEnd > len(canonical) {
		t.Fatalf("composite canonical domain exceeds payload: end=%d size=%d", domainEnd, len(canonical))
	}
	if got := string(canonical[8:domainEnd]); got != compositeLinkCanonicalDomain {
		t.Fatalf("composite canonical domain = %q, want %q", got, compositeLinkCanonicalDomain)
	}
}

func compositeCoordinates(extensions []LinkedCompositeExtension) []string {
	result := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		result = append(result, extension.Coordinate().String())
	}
	return result
}
