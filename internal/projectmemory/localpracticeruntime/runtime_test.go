package localpracticeruntime

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	basetypeenvartifacts "github.com/m0n0x41d/haft/data/haft/base-typeenv/artifacts"
	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/codeanchoradapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/evidenceworkadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/noteadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/portfoliocomparisonadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/problemcardadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projectmemory/solutionportfolioadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/specsectionadapter"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

func TestCurrentCandidateBuildsAllFiveExactReferenceSchemeRegistries(
	t *testing.T,
) {
	t.Parallel()
	base := loadCurrentBaseArtifact(t)
	target, err := Build(base, typedmemorycandidates.SourceV1_5())
	if err != nil {
		t.Fatalf("Build(current candidate) error = %v", err)
	}
	const wantExtension = "typeenv-extension:haft.typed-memory@sha256:9f55a10e72adbc5f61401692ce9e9a82b20710adaea93a462c38aa957466bac4"
	if got := target.Extension().Ref().String(); got != wantExtension {
		t.Fatalf("current candidate E = %s, want %s", got, wantExtension)
	}
	const wantComposite = "typeenv:sha256:1e23069cc232c1234dfada5d9059ba23f53ab4f420ee9c6f19d4ecf0b1249e49"
	if got := target.Composite().Ref().String(); got != wantComposite {
		t.Fatalf("current candidate C = %s, want %s", got, wantComposite)
	}
	contracts := map[projecttypeenv.RuntimeMechanismInvocationContract]int{}
	for _, requirement := range target.Requirements().Requirements() {
		contract := requirement.InvocationContract()
		if isReferenceSchemeContract(contract) {
			contracts[contract]++
		}
	}
	wantContracts := []projecttypeenv.RuntimeMechanismInvocationContract{
		projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution,
		projecttypeenv.RuntimeMechanismContractClaimInterpretation,
		projecttypeenv.RuntimeMechanismContractClaimMeasurement,
		projecttypeenv.RuntimeMechanismContractClaimEvaluation,
		projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation,
	}
	for _, contract := range wantContracts {
		if contracts[contract] != 1 {
			t.Fatalf(
				"current candidate requirements for %q = %d, want exactly 1",
				contract.String(),
				contracts[contract],
			)
		}
	}
	registry, present := target.ExactRuntimeRegistry()
	if !present || !registry.Valid() {
		t.Fatal("current candidate has no valid exact target runtime registry")
	}
	designation, present := registry.ReferenceDesignationResolutionRegistry()
	if !present || designation.Len() != 1 {
		t.Fatalf("designation registry presence/size = %v/%d, want true/1", present, designation.Len())
	}
	interpretation, present := registry.ClaimInterpretationRegistry()
	if !present || interpretation.Len() != 1 {
		t.Fatalf("interpretation registry presence/size = %v/%d, want true/1", present, interpretation.Len())
	}
	measurement, present := registry.ClaimMeasurementRegistry()
	if !present || measurement.Len() != 1 {
		t.Fatalf("measurement registry presence/size = %v/%d, want true/1", present, measurement.Len())
	}
	evaluation, present := registry.ClaimEvaluationRegistry()
	if !present || evaluation.Len() != 1 {
		t.Fatalf("evaluation registry presence/size = %v/%d, want true/1", present, evaluation.Len())
	}
	constitution, present := registry.EpistemeConstitutionEvaluationRegistry()
	if !present || constitution.Len() != 1 {
		t.Fatalf("constitution registry presence/size = %v/%d, want true/1", present, constitution.Len())
	}
}

func TestShippedV1BuildReproducesExactSelectedXAndC(t *testing.T) {
	t.Parallel()
	base := loadShippedV1BaseArtifact(t)
	target, err := Build(base, typedmemorycandidates.SourceV1())
	if err != nil {
		t.Fatalf("Build(shipped 1.0 target) error = %v", err)
	}
	const selectedC = "typeenv:sha256:d6097b7231aee200a0b998bd4146496b796222917e1e16505ac897079b7f29c2"
	if target.Composite().Ref().String() != selectedC {
		t.Fatalf(
			"shipped 1.0 C = %s, want exact selected C %s (X=%s)",
			target.Composite().Ref(),
			selectedC,
			target.RuntimeBasis().Ref(),
		)
	}
	const selectedX = "runtime-evaluation-basis:sha256:d471140ed60af71a7e75b1a2499d06319af6426105646a5518131f9bfa4678dc"
	if target.RuntimeBasis().Ref().String() != selectedX {
		t.Fatalf(
			"shipped 1.0 X = %s, want exact selected X %s",
			target.RuntimeBasis().Ref(),
			selectedX,
		)
	}
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	policy := targetPolicyForRule(t, target, recordRule)
	if got := len(policy.AcceptedMappings()); got != 7 {
		t.Fatalf("shipped 1.0 ProjectRecord mappings = %d, want 7", got)
	}
	assertAcceptedMappings(
		t,
		policy,
		mustShippedV1ProjectRecordMappings(t),
	)
	currentNote, err := noteadapter.CurrentMappingManifestV1()
	if err != nil {
		t.Fatalf("CurrentMappingManifestV1(Note) error = %v", err)
	}
	decision, err := policy.EvaluateMappingPolicy(
		currentNote.Ref(),
		currentNote.AdapterVersion(),
	)
	if err != nil {
		t.Fatalf("EvaluateMappingPolicy(current Note) error = %v", err)
	}
	if decision.Kind() != recordmembershipregistration.MappingManifestNotAccepted {
		t.Fatalf(
			"shipped 1.0 policy decision for current Note = %s, want manifest_not_accepted",
			decision.Kind(),
		)
	}
	codeAnchorRule := carrierfamily.CodeAnchorEvaluatorRuleV1()
	codeAnchorPolicy := targetPolicyForRule(t, target, codeAnchorRule)
	if got := len(codeAnchorPolicy.AcceptedMappings()); got != 2 {
		t.Fatalf("shipped 1.0 CodeAnchor mappings = %d, want 2", got)
	}
	shippedCodeAnchor, err := codeAnchorShippedV1AcceptedMappings()
	if err != nil {
		t.Fatalf("codeAnchorShippedV1AcceptedMappings() error = %v", err)
	}
	assertAcceptedMappings(t, codeAnchorPolicy, shippedCodeAnchor)
}

func TestHistoricalV1_2CandidateAcceptsTargetReviewedShippedV1ProducerCoordinates(
	t *testing.T,
) {
	t.Parallel()
	target := buildHistoricalV1_2Candidate(t)
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	recordPolicy := targetPolicyForRule(t, target, recordRule)
	if got := len(recordPolicy.AcceptedMappings()); got != 14 {
		t.Fatalf(
			"ProjectRecord accepted mappings = %d, want seven current plus seven target-reviewed shipped-v1 coordinates",
			got,
		)
	}
	recordCompatibility, err := ProjectRecordTargetReviewedCompatibilityMappingsV1()
	if err != nil {
		t.Fatalf("ProjectRecordTargetReviewedCompatibilityMappingsV1() error = %v", err)
	}
	assertAcceptedMappings(t, recordPolicy, recordCompatibility)

	codeAnchorRule := carrierfamily.CodeAnchorEvaluatorRuleV1()
	codeAnchorPolicy := targetPolicyForRule(t, target, codeAnchorRule)
	if got := len(codeAnchorPolicy.AcceptedMappings()); got != 3 {
		t.Fatalf(
			"CodeAnchor accepted mappings = %d, want unchanged family plus current and target-reviewed shipped-v1 task coordinates",
			got,
		)
	}
	codeAnchorCompatibility, err := CodeAnchorTargetReviewedCompatibilityMappingsV1()
	if err != nil {
		t.Fatalf("CodeAnchorTargetReviewedCompatibilityMappingsV1() error = %v", err)
	}
	assertAcceptedMappings(t, codeAnchorPolicy, codeAnchorCompatibility)
}

func TestHistoricalV1_2CompatibilityUnionIsTargetPolicyOnlyAndDeterministic(t *testing.T) {
	t.Parallel()
	first := buildHistoricalV1_2Candidate(t)
	second := buildHistoricalV1_2Candidate(t)
	if first.RuntimeBasis().Ref() != second.RuntimeBasis().Ref() {
		t.Fatalf(
			"repeated current builds produced X %s and %s",
			first.RuntimeBasis().Ref(),
			second.RuntimeBasis().Ref(),
		)
	}
	if first.Composite().Ref() != second.Composite().Ref() {
		t.Fatalf(
			"repeated current builds produced C %s and %s",
			first.Composite().Ref(),
			second.Composite().Ref(),
		)
	}
	const reviewedX = "runtime-evaluation-basis:sha256:15229aa012e03837a48d5e5ec12838bf268e49554044dd4e808814749bbb5eb4"
	if first.RuntimeBasis().Ref().String() != reviewedX {
		t.Fatalf(
			"target-reviewed compatibility X = %s, want %s",
			first.RuntimeBasis().Ref(),
			reviewedX,
		)
	}
	const reviewedC = "typeenv:sha256:3ac756f91dca4c86173b8b75d488b86b71214ab97c7e4dc852854f07b4edfd82"
	if first.Composite().Ref().String() != reviewedC {
		t.Fatalf(
			"target-reviewed compatibility C = %s, want %s",
			first.Composite().Ref(),
			reviewedC,
		)
	}

	current := currentTaskProducerMappings(t)
	compatible := allTargetReviewedCompatibilityMappings(t)
	seenManifests := make(map[string]string, len(current)+len(compatible))
	for _, mapping := range append(current, compatible...) {
		manifest := mapping.Manifest().String()
		adapter := mapping.Adapter().String()
		if previous, duplicate := seenManifests[manifest]; duplicate {
			t.Fatalf(
				"current/target-reviewed union repeats manifest %s with adapters %s and %s",
				manifest,
				previous,
				adapter,
			)
		}
		seenManifests[manifest] = adapter
	}

	assertCurrentTaskProducerCoordinates(t, current)
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	firstPolicy := targetPolicyForRule(t, first, recordRule)
	secondPolicy := targetPolicyForRule(t, second, recordRule)
	if !slices.Equal(
		firstPolicy.AcceptedMappings(),
		secondPolicy.AcceptedMappings(),
	) {
		t.Fatal("repeated current builds changed ProjectRecord policy ordering")
	}
	assertAcceptedMappings(t, firstPolicy, current[:7])
	assertAcceptedMappings(t, firstPolicy, compatible[:7])
	codeAnchorPolicy := targetPolicyForRule(
		t,
		first,
		carrierfamily.CodeAnchorEvaluatorRuleV1(),
	)
	assertAcceptedMappings(t, codeAnchorPolicy, current[7:])
	assertAcceptedMappings(t, codeAnchorPolicy, compatible[7:])

	for _, mapping := range current {
		if mapping.Manifest().Version() != "2.0.0" {
			t.Fatalf(
				"current writer %s emits manifest version %s, want 2.0.0",
				mapping.Manifest().ID(),
				mapping.Manifest().Version(),
			)
		}
	}
}

func TestCompatibilityCatalogReturnsOwnedDeterministicValues(t *testing.T) {
	t.Parallel()
	first := allTargetReviewedCompatibilityMappings(t)
	expected := []struct {
		manifest string
		adapter  string
	}{
		{
			manifest: "mapping-manifest:18:haft.evidence-work5:1.0.0sha256:bb010a20f5c691e1a615b8a7e891bef1a4087e89695099b124df2a4e8b22069e",
			adapter:  "haft-evidence-work-adapter/1.0.0",
		},
		{
			manifest: "mapping-manifest:20:haft.note-at-concern5:1.0.0sha256:c22309ff58a1f1be7474841f5232d43ec0024f423f8fe22336330b2700ba6f53",
			adapter:  "haft-note-adapter/1.0.0",
		},
		{
			manifest: "mapping-manifest:25:haft.portfolio-comparison5:1.0.0sha256:28d0f994ab80bb8ff70d63a9361d8d737da1d177a334edf5def8c31ad3ac68a1",
			adapter:  "haft-portfolio-comparison-adapter/1.0.0",
		},
		{
			manifest: "mapping-manifest:28:haft.problem-card-at-concern5:1.0.0sha256:0ea0ed8ac6340eb7a3c4857480a2494423df1763dad403430671f226849ab3a7",
			adapter:  "haft-problem-card-adapter/1.0.0",
		},
		{
			manifest: "mapping-manifest:28:haft.spec-section-at-concern5:1.0.0sha256:92bbceedf8989609775f279fb37aa641de050dd4335ad4ba1efab47d6fefc412",
			adapter:  "haft-spec-section-adapter/1.0.0",
		},
		{
			manifest: "mapping-manifest:31:haft.decision-choice-at-concern5:1.1.0sha256:85de30a8ab311ae9254131f2b2c08b39744a1512879a7233aaf8c3d84fc82545",
			adapter:  "haft-decision-record-adapter/1.1.0",
		},
		{
			manifest: "mapping-manifest:34:haft.solution-portfolio-at-concern5:1.0.0sha256:eb3b9729e37f93cd93608611910e41bac66d98462b318c20b09b54a7d93d8b65",
			adapter:  "haft-solution-portfolio-adapter/1.0.0",
		},
		{
			manifest: "mapping-manifest:16:haft.code-anchor5:1.0.0sha256:a4e5dad8db3dd94922f45585d23b3f3acb6790689ac06ceac2db00636d2a9a7d",
			adapter:  "haft-code-anchor-adapter/1.0.0",
		},
	}
	if len(first) != len(expected) {
		t.Fatalf(
			"target-reviewed compatibility mappings = %d, want exact set of %d",
			len(first),
			len(expected),
		)
	}
	for index, mapping := range first {
		if mapping.Manifest().String() != expected[index].manifest ||
			mapping.Adapter().String() != expected[index].adapter {
			t.Fatalf(
				"target-reviewed compatibility mapping %d = (%s, %s), want (%s, %s)",
				index,
				mapping.Manifest(),
				mapping.Adapter(),
				expected[index].manifest,
				expected[index].adapter,
			)
		}
	}
	baseline := append(
		[]recordmembershipregistration.AcceptedMapping(nil),
		first...,
	)
	first[0] = recordmembershipregistration.AcceptedMapping{}
	first = append(first, recordmembershipregistration.AcceptedMapping{})
	second := allTargetReviewedCompatibilityMappings(t)
	if !slices.Equal(second, baseline) {
		t.Fatal("mutating returned compatibility mappings changed package catalog")
	}
}

func TestHistoricalV1_2CandidateRevalidatesExactLiveShippedV1NoteSourceUnderTargetC(
	t *testing.T,
) {
	t.Parallel()
	target := buildHistoricalV1_2Candidate(t)
	snapshot, present := target.Preparation().ExecutableSnapshot()
	if !present {
		t.Fatal("current target has no executable snapshot")
	}
	registry, present := target.ExactRuntimeRegistry()
	if !present {
		t.Fatal("current target has no exact runtime registry")
	}
	engine, err := projectmemory.NewRecordMembershipAdmissionEngine(registry)
	if err != nil {
		t.Fatalf("NewRecordMembershipAdmissionEngine() error = %v", err)
	}
	project, err := projectidentity.ParseProjectID("qnt_e3149c17")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	entity, err := typedmemory.NewEntityID("record:note-20260719-cdb06186")
	if err != nil {
		t.Fatalf("NewEntityID() error = %v", err)
	}
	compatible, err := ProjectRecordTargetReviewedCompatibilityMappingsV1()
	if err != nil {
		t.Fatalf("ProjectRecordTargetReviewedCompatibilityMappingsV1() error = %v", err)
	}
	noteMapping := mappingForManifestID(
		t,
		compatible,
		noteadapter.MappingManifestIDV1,
	)
	carrier, err := recordcarrier.SealProjectRecordCarrierV1(
		entity,
		contextRef,
		recordcarrier.GenericProjectRecordVariantV1{},
	)
	if err != nil {
		t.Fatalf("SealProjectRecordCarrierV1() error = %v", err)
	}
	binding, err := recordcarrier.SealEntityRecordCarrierBindingV1(
		project,
		carrier,
		noteMapping.Manifest(),
		noteMapping.Adapter(),
	)
	if err != nil {
		t.Fatalf("SealEntityRecordCarrierBindingV1() error = %v", err)
	}
	source, err := recordcarrier.SealRecordMembershipSourceV1(
		project,
		entity,
		contextRef,
		carrier,
		binding,
	)
	if err != nil {
		t.Fatalf("SealRecordMembershipSourceV1() error = %v", err)
	}
	const liveSourceRef = "record-membership-source:sha256:35f0366e5d37cd34d6ac0e07d4a8f5062658b09feed6f44cec43f95b48aac296"
	if source.ObservableInput().Reference().String() != liveSourceRef {
		t.Fatalf(
			"recreated shipped-v1 Note source = %s, want exact live durable source %s",
			source.ObservableInput().Reference().String(),
			liveSourceRef,
		)
	}
	blob, err := memberofevaluation.NewObservableInputBlob(
		source.ObservableInput().Reference(),
		source.ObservableInput().Digest(),
		source.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("NewObservableInputBlob() error = %v", err)
	}
	environment := snapshot.Environment()
	kindID, err := typedmemory.NewKindID("Haft.ProjectRecord")
	if err != nil {
		t.Fatalf("NewKindID() error = %v", err)
	}
	valueKind, err := typedmemory.NewValueKindRef(environment.Ref(), kindID)
	if err != nil {
		t.Fatalf("NewValueKindRef() error = %v", err)
	}
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewGammaPoint() error = %v", err)
	}
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   contextRef,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice() error = %v", err)
	}
	query, err := typedmemory.NewMemberOfQuery(entity, valueKind, contextSlice)
	if err != nil {
		t.Fatalf("NewMemberOfQuery() error = %v", err)
	}
	revision := typedmemory.NewGraphRevision(3)
	view, err := typedmemory.NewPersistedSnapshotView(environment.Ref(), revision)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView() error = %v", err)
	}
	request, err := typedmemory.NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest() error = %v", err)
	}
	universe, err := memberofevaluation.NewExactPersistedEntityUniverse(
		project,
		contextRef,
		revision,
		[]typedmemory.EntityID{entity},
	)
	if err != nil {
		t.Fatalf("NewExactPersistedEntityUniverse() error = %v", err)
	}
	input, err := memberofevaluation.NewMemberOfEvaluationInput(
		project,
		environment,
		request,
		[]memberofevaluation.ObservableInputBlob{blob},
		universe,
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationInput() error = %v", err)
	}
	selection := engine.SelectSnapshotObservableInputs(input)
	if _, selected := selection.(memberofevaluation.SnapshotObservableInputsSelected); !selected {
		t.Fatalf("shipped-v1 Note source selection = %T, want selected", selection)
	}
	judgement, err := engine.EvaluateMemberOf(context.Background(), input)
	if err != nil {
		t.Fatalf("EvaluateMemberOf(shipped-v1 Note source) error = %v", err)
	}
	if _, member := judgement.(typedmemory.MemberOfMember); !member {
		t.Fatalf("shipped-v1 Note judgement = %T, want MemberOfMember", judgement)
	}
}

func buildHistoricalV1_2Candidate(t *testing.T) Target {
	t.Helper()
	base := loadHistoricalV1_2BaseArtifact(t)
	target, err := Build(base, typedmemorycandidates.SourceV1_2())
	if err != nil {
		t.Fatalf("Build(historical 1.2 candidate) error = %v", err)
	}
	return target
}

func targetPolicyForRule(
	t *testing.T,
	target Target,
	rule typedmemory.RuleRef,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	for _, policy := range target.RegistrationPolicies() {
		if policy.Evaluator().Rule() == rule {
			return policy
		}
	}
	t.Fatalf("target has no policy for %s", rule.String())
	return recordmembershipregistration.RegistrationArtifactV1{}
}

func assertAcceptedMappings(
	t *testing.T,
	policy recordmembershipregistration.RegistrationArtifactV1,
	mappings []recordmembershipregistration.AcceptedMapping,
) {
	t.Helper()
	for _, mapping := range mappings {
		decision, err := policy.EvaluateMappingPolicy(
			mapping.Manifest(),
			mapping.Adapter(),
		)
		if err != nil {
			t.Fatalf("EvaluateMappingPolicy(%s) error = %v", mapping.Manifest(), err)
		}
		if decision.Kind() != recordmembershipregistration.MappingAccepted {
			t.Fatalf(
				"mapping policy for %s = %s, want accepted",
				mapping.Manifest(),
				decision.Kind().String(),
			)
		}
	}
}

func currentTaskProducerMappings(
	t *testing.T,
) []recordmembershipregistration.AcceptedMapping {
	t.Helper()
	noteManifest, noteErr := noteadapter.CurrentMappingManifestV1()
	note := mustAcceptedMappingFromManifest(t, noteManifest, noteErr)
	problemManifest, problemErr := problemcardadapter.CurrentMappingManifestV1()
	problem := mustAcceptedMappingFromManifest(
		t,
		problemManifest,
		problemErr,
	)
	portfolioManifest, portfolioErr :=
		solutionportfolioadapter.CurrentMappingManifestV1()
	portfolio := mustAcceptedMappingFromManifest(
		t,
		portfolioManifest,
		portfolioErr,
	)
	comparisonManifest, comparisonErr :=
		portfoliocomparisonadapter.CurrentMappingManifestV1()
	comparison := mustAcceptedMappingFromManifest(
		t,
		comparisonManifest,
		comparisonErr,
	)
	specSectionManifest, specSectionErr :=
		specsectionadapter.CurrentMappingManifestV1()
	specSection := mustAcceptedMappingFromManifest(
		t,
		specSectionManifest,
		specSectionErr,
	)
	evidenceWorkManifest, evidenceWorkErr :=
		evidenceworkadapter.CurrentMappingManifestV1()
	evidenceWork := mustAcceptedMappingFromManifest(
		t,
		evidenceWorkManifest,
		evidenceWorkErr,
	)
	decisionManifest, decisionErr := decisionrecordadapter.CurrentMappingManifestV1()
	decision := mustAcceptedMappingFromManifest(
		t,
		decisionManifest,
		decisionErr,
	)
	codeAnchorManifest, codeAnchorErr := codeanchoradapter.CurrentMappingManifestV1()
	codeAnchor := mustAcceptedMappingFromManifest(
		t,
		codeAnchorManifest,
		codeAnchorErr,
	)
	return []recordmembershipregistration.AcceptedMapping{
		note,
		problem,
		portfolio,
		comparison,
		specSection,
		evidenceWork,
		decision,
		codeAnchor,
	}
}

type currentManifest interface {
	Ref() recordmapping.MappingManifestRef
	AdapterVersion() recordmapping.AdapterVersion
}

func mustAcceptedMappingFromManifest[T currentManifest](
	t *testing.T,
	manifest T,
	err error,
) recordmembershipregistration.AcceptedMapping {
	t.Helper()
	if err != nil {
		t.Fatalf("load current task-producer mapping manifest: %v", err)
	}
	mapping, err := recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest.Ref(),
			Adapter:  manifest.AdapterVersion(),
		},
	)
	if err != nil {
		t.Fatalf("construct current task-producer mapping: %v", err)
	}
	return mapping
}

func allTargetReviewedCompatibilityMappings(
	t *testing.T,
) []recordmembershipregistration.AcceptedMapping {
	t.Helper()
	recordMappings, err := ProjectRecordTargetReviewedCompatibilityMappingsV1()
	if err != nil {
		t.Fatalf("ProjectRecordTargetReviewedCompatibilityMappingsV1() error = %v", err)
	}
	codeAnchorMappings, err := CodeAnchorTargetReviewedCompatibilityMappingsV1()
	if err != nil {
		t.Fatalf("CodeAnchorTargetReviewedCompatibilityMappingsV1() error = %v", err)
	}
	return append(recordMappings, codeAnchorMappings...)
}

func assertCurrentTaskProducerCoordinates(
	t *testing.T,
	mappings []recordmembershipregistration.AcceptedMapping,
) {
	t.Helper()
	expectedAdapters := []string{
		noteadapter.AdapterVersionV1,
		problemcardadapter.AdapterVersionV1,
		solutionportfolioadapter.AdapterVersionV1,
		portfoliocomparisonadapter.AdapterVersionV1,
		specsectionadapter.AdapterVersionV1,
		evidenceworkadapter.AdapterVersionV1,
		decisionrecordadapter.AdapterVersionV1,
		codeanchoradapter.AdapterVersionV1,
	}
	if len(mappings) != len(expectedAdapters) {
		t.Fatalf(
			"current task-producer mappings = %d, want %d",
			len(mappings),
			len(expectedAdapters),
		)
	}
	for index, mapping := range mappings {
		if mapping.Adapter().String() != expectedAdapters[index] {
			t.Fatalf(
				"current task-producer adapter %d = %s, want %s",
				index,
				mapping.Adapter(),
				expectedAdapters[index],
			)
		}
	}
}

func mappingForManifestID(
	t *testing.T,
	mappings []recordmembershipregistration.AcceptedMapping,
	manifestID string,
) recordmembershipregistration.AcceptedMapping {
	t.Helper()
	for _, mapping := range mappings {
		if mapping.Manifest().ID() == manifestID {
			return mapping
		}
	}
	t.Fatalf("target-reviewed compatibility catalog has no %s mapping", manifestID)
	return recordmembershipregistration.AcceptedMapping{}
}

func isReferenceSchemeContract(
	contract projecttypeenv.RuntimeMechanismInvocationContract,
) bool {
	switch contract {
	case projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution,
		projecttypeenv.RuntimeMechanismContractClaimInterpretation,
		projecttypeenv.RuntimeMechanismContractClaimMeasurement,
		projecttypeenv.RuntimeMechanismContractClaimEvaluation,
		projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation:
		return true
	default:
		return false
	}
}

func loadCurrentBaseArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	path := filepath.Join("..", "..", "cli", "fpf.db")
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro&immutable=1")
	if err != nil {
		t.Fatalf("open embedded FPF database read-only: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(
		context.Background(),
		database,
	)
	if err != nil {
		t.Fatalf("LoadArtifactReadOnlyDB() error = %v", err)
	}
	return artifact
}

func loadHistoricalV1_2BaseArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef(basetypeenvartifacts.HistoricalV3Ref)
	if err != nil {
		t.Fatalf("ParseTypeEnvRef(historical 1.2 Base) error = %v", err)
	}
	artifact, err := basetypeenvartifacts.LoadExact(ref)
	if err != nil {
		t.Fatalf("LoadExact(historical 1.2 Base) error = %v", err)
	}
	return artifact
}

func loadShippedV1BaseArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	path := filepath.Join(
		"testdata",
		"fpf-44dd881-base.canonical.gz.b64",
	)
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped 44dd881 Base fixture: %v", err)
	}
	compact := strings.Join(strings.Fields(string(encoded)), "")
	compressed, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		t.Fatalf("decode shipped 44dd881 Base fixture: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open shipped 44dd881 Base fixture: %v", err)
	}
	canonical, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("inflate shipped 44dd881 Base fixture: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close shipped 44dd881 Base fixture: %v", err)
	}
	artifact, err := typeenv.DecodeBaseTypeEnvArtifact(canonical)
	if err != nil {
		t.Fatalf("decode shipped 44dd881 Base artifact: %v", err)
	}
	const expected = "typeenv:sha256:aa1eec077868e611108810f1e4bc187d55eb38e3bc705cc149a098008b58cd1a"
	ref, present := artifact.TypeEnvRef()
	if !present || ref.String() != expected {
		t.Fatalf(
			"shipped 44dd881 Base = %s (present=%v), want %s",
			ref,
			present,
			expected,
		)
	}
	return artifact
}

func mustShippedV1ProjectRecordMappings(
	t *testing.T,
) []recordmembershipregistration.AcceptedMapping {
	t.Helper()
	mappings, err := projectRecordShippedV1AcceptedMappings()
	if err != nil {
		t.Fatalf("projectRecordShippedV1AcceptedMappings() error = %v", err)
	}
	return mappings
}
