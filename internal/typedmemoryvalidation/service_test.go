package typedmemoryvalidation

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestBundledCandidateBindsButNeverReturnsProjectValid(t *testing.T) {
	environment := testEnvironment(t, "1")
	basis, err := NewBundledCandidateOpenWorldBasis(
		environment,
		typedmemory.NewCodecRegistry(),
	)
	if err != nil {
		t.Fatalf("NewBundledCandidateOpenWorldBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: basis}
	service := mustService(t, resolver)
	request := decodeRequest(t, `{"kind":"bundled_candidate_open_world"}`)

	response := service.Validate(request)

	if response.Verdict() != typedmemory.ValidationUnderdetermined {
		t.Fatalf("Verdict() = %q, want underdetermined", response.Verdict())
	}
	assertDiagnosticCode(t, response, DiagnosticCandidateNotProject)
	assertNoAdmissionCapability(t, response)
	assertNoNormalizedDigest(t, response)
	assertNoEffects(t, response)
	projection := response.Basis()
	if projection.ResolutionKind() != BasisResolutionBundledCandidate {
		t.Fatalf("ResolutionKind() = %q", projection.ResolutionKind())
	}
	if _, hasRevision := projection.GraphRevision(); hasRevision {
		t.Fatal("bundled candidate fabricated a graph revision")
	}
	if resolver.lastKind != typedmemorywire.BasisBundledCandidateOpenWorld {
		t.Fatalf("resolver selector = %q", resolver.lastKind)
	}
}

func TestZeroValidateRequestFailsBeforeBasisResolution(t *testing.T) {
	resolver := &fixedResolver{resolution: NewProjectBasisUnavailable()}
	service := mustService(t, resolver)

	response := service.Validate(typedmemorywire.ValidateRequest{})

	if response.Verdict() != typedmemory.ValidationInvalid {
		t.Fatalf("Verdict() = %q, want invalid", response.Verdict())
	}
	assertDiagnosticCode(t, response, DiagnosticMalformedValidationRequest)
	if resolver.callCount != 0 {
		t.Fatalf("zero request reached resolver %d time(s)", resolver.callCount)
	}
}

func TestValidatePreservesDecodedContractVersionInOutcomeAndResponse(t *testing.T) {
	testCases := []struct {
		name    string
		request typedmemorywire.ValidateRequest
		want    string
	}{
		{
			name:    "v1 legacy request",
			request: decodeRequest(t, `{"kind":"project_current"}`),
			want:    typedmemorywire.ContractVersionV1,
		},
		{
			name:    "v2 relational assertion request",
			request: decodeRelationalAssertionRequest(t),
			want:    typedmemorywire.ContractVersionV2,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolution := NewProjectBasisUnavailable()
			service := mustService(t, &fixedResolver{resolution: resolution})

			outcome := service.Evaluate(testCase.request)
			if outcome.ContractVersion() != testCase.want {
				t.Fatalf(
					"outcome contract version = %q, want %q",
					outcome.ContractVersion(),
					testCase.want,
				)
			}

			response := service.Validate(testCase.request)
			if response.ContractVersion() != testCase.want {
				t.Fatalf(
					"response contract version = %q, want %q",
					response.ContractVersion(),
					testCase.want,
				)
			}
			payload, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("json.Marshal(response): %v", err)
			}
			projection := struct {
				ContractVersion string `json:"contract_version"`
			}{}
			if err := json.Unmarshal(payload, &projection); err != nil {
				t.Fatalf("json.Unmarshal(response): %v", err)
			}
			if projection.ContractVersion != testCase.want {
				t.Fatalf(
					"JSON contract version = %q, want %q",
					projection.ContractVersion,
					testCase.want,
				)
			}
		})
	}
}

func TestSelectorSubstitutionNeverChangesRequestedBasis(t *testing.T) {
	environment := testEnvironment(t, "2")
	bundled, err := NewBundledCandidateOpenWorldBasis(
		environment,
		typedmemory.NewCodecRegistry(),
	)
	if err != nil {
		t.Fatalf("NewBundledCandidateOpenWorldBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: bundled}
	service := mustService(t, resolver)
	wireRequest := decodeRequest(t, `{"kind":"project_current"}`)
	request := &trackingRequest{request: wireRequest}

	response := presentOutcome(service.evaluate(request))

	if response.Verdict() != typedmemory.ValidationUnderdetermined {
		t.Fatalf("Verdict() = %q, want underdetermined", response.Verdict())
	}
	assertDiagnosticCode(t, response, DiagnosticBasisResolutionMismatch)
	if request.bindCount != 0 {
		t.Fatalf("selector substitution bound change set %d time(s)", request.bindCount)
	}
	if response.Basis().RequestedKind() != typedmemorywire.BasisProjectCurrent {
		t.Fatalf("requested basis was substituted: %q", response.Basis().RequestedKind())
	}
}

func TestProjectUnavailableReturnsBothMissingBasesWithoutBinding(t *testing.T) {
	resolver := &fixedResolver{resolution: NewProjectBasisUnavailable()}
	service := mustService(t, resolver)
	wireRequest := decodeRequest(t, `{"kind":"project_current"}`)
	request := &trackingRequest{request: wireRequest}

	response := presentOutcome(service.evaluate(request))

	if response.Verdict() != typedmemory.ValidationUnderdetermined {
		t.Fatalf("Verdict() = %q, want underdetermined", response.Verdict())
	}
	assertDiagnosticCode(t, response, DiagnosticProjectTypeEnvUnavailable)
	assertDiagnosticCode(t, response, DiagnosticProjectSnapshotUnavailable)
	if request.bindCount != 0 {
		t.Fatalf("unavailable project basis bound change set %d time(s)", request.bindCount)
	}
	assertNoEffects(t, response)
}

func TestExactProjectMismatchNeverFallsBackToObservedProject(t *testing.T) {
	environment := testEnvironment(t, "3")
	snapshot := newTestSnapshot(t, environment.Ref(), 17, entityAbsent)
	projectBasis, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)
	if err != nil {
		t.Fatalf("NewResolvedProjectBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: projectBasis}
	service := mustService(t, resolver)
	requestedDigest := strings.Repeat("4", 64)
	basisJSON := fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":"sha256:%s","graph_revision":17}`,
		requestedDigest,
	)
	wireRequest := decodeRequest(t, basisJSON)
	request := &trackingRequest{request: wireRequest}
	snapshot.resetCalls()

	response := presentOutcome(service.evaluate(request))

	if response.Verdict() != typedmemory.ValidationUnderdetermined {
		t.Fatalf("Verdict() = %q, want underdetermined", response.Verdict())
	}
	assertDiagnosticCode(t, response, DiagnosticExactBasisMismatch)
	if request.bindCount != 0 {
		t.Fatalf("exact mismatch bound change set %d time(s)", request.bindCount)
	}
	if snapshot.entityCalls != 0 {
		t.Fatalf("exact mismatch fell back to observed snapshot %d time(s)", snapshot.entityCalls)
	}
	assertNoNormalizedDigest(t, response)
}

func TestExactMismatchResolutionCannotMasqueradeAsMatch(t *testing.T) {
	environment := testEnvironment(t, "5")
	mismatch, err := NewExactProjectBasisMismatch(
		environment.Ref(),
		typedmemory.NewGraphRevision(19),
	)
	if err != nil {
		t.Fatalf("NewExactProjectBasisMismatch() error = %v", err)
	}
	resolver := &fixedResolver{resolution: mismatch}
	service := mustService(t, resolver)
	basisJSON := fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":"%s","graph_revision":19}`,
		environment.Ref().Digest().String(),
	)
	wireRequest := decodeRequest(t, basisJSON)
	request := &trackingRequest{request: wireRequest}

	response := presentOutcome(service.evaluate(request))

	if response.Verdict() != typedmemory.ValidationUnderdetermined {
		t.Fatalf("Verdict() = %q, want underdetermined", response.Verdict())
	}
	assertDiagnosticCode(t, response, DiagnosticBasisResolutionMismatch)
	if request.bindCount != 0 {
		t.Fatalf("mismatch resolution bound change set %d time(s)", request.bindCount)
	}
}

func TestResolvedProjectBasisCanReturnHonestValidWithoutAdmissionLeak(t *testing.T) {
	environment := testEnvironment(t, "6")
	snapshot := newTestSnapshot(t, environment.Ref(), 23, entityAbsent)
	basis, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)
	if err != nil {
		t.Fatalf("NewResolvedProjectBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: basis}
	service := mustService(t, resolver)
	request := decodeRequest(t, `{"kind":"project_current"}`)

	response := service.Validate(request)

	valid, isValid := response.(ValidResponse)
	if !isValid {
		t.Fatalf("response type = %T, want ValidResponse", response)
	}
	if valid.NormalizedDigest().String() == "" {
		t.Fatal("ValidResponse has no normalized digest")
	}
	if len(response.Diagnostics()) != 0 {
		t.Fatalf("ValidResponse diagnostics = %v", response.Diagnostics())
	}
	assertNoAdmissionCapability(t, response)
	assertNoEffects(t, response)
	payload, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		t.Fatalf("json.Marshal() error = %v", marshalErr)
	}
	if strings.Contains(string(payload), "admission") {
		t.Fatalf("public response leaks admission capability: %s", payload)
	}
}

func TestExactMatchingProjectBasisCanReturnValid(t *testing.T) {
	environment := testEnvironment(t, "7")
	snapshot := newTestSnapshot(t, environment.Ref(), 29, entityAbsent)
	basis, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)
	if err != nil {
		t.Fatalf("NewResolvedProjectBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: basis}
	service := mustService(t, resolver)
	basisJSON := fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":"%s","graph_revision":29}`,
		environment.Ref().Digest().String(),
	)
	request := decodeRequest(t, basisJSON)

	response := service.Validate(request)

	if _, isValid := response.(ValidResponse); !isValid {
		t.Fatalf("response type = %T, want ValidResponse", response)
	}
	projection := response.Basis()
	graphRevision, hasRevision := projection.GraphRevision()
	if !hasRevision || graphRevision != 29 {
		t.Fatalf("project graph revision = %d, %v", graphRevision, hasRevision)
	}
}

func TestValidResponseDigestUsesCanonicalAdmittedValue(t *testing.T) {
	fixture := newCanonicalDigestFixture(t)
	basis, err := NewResolvedProjectBasis(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
	)
	if err != nil {
		t.Fatalf("NewResolvedProjectBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: basis}
	service := mustService(t, resolver)
	compact := fixture.decodeRequest(t, []byte("same semantic text"))
	padded := fixture.decodeRequest(t, []byte("  same semantic text  \n"))

	compactCandidate, err := compact.BindChangeSet(fixture.environment.Ref())
	if err != nil {
		t.Fatalf("compact BindChangeSet() error = %v", err)
	}
	paddedCandidate, err := padded.BindChangeSet(fixture.environment.Ref())
	if err != nil {
		t.Fatalf("padded BindChangeSet() error = %v", err)
	}
	compactRawDigest, err := compactCandidate.Digest()
	if err != nil {
		t.Fatalf("compact candidate Digest() error = %v", err)
	}
	paddedRawDigest, err := paddedCandidate.Digest()
	if err != nil {
		t.Fatalf("padded candidate Digest() error = %v", err)
	}
	if compactRawDigest == paddedRawDigest {
		t.Fatal("fixture candidates do not prove pre-canonical byte variance")
	}

	compactResponse := service.Validate(compact)
	paddedResponse := service.Validate(padded)
	compactValid, compactIsValid := compactResponse.(ValidResponse)
	paddedValid, paddedIsValid := paddedResponse.(ValidResponse)
	if !compactIsValid || !paddedIsValid {
		t.Fatalf(
			"response types = %T/%T, want ValidResponse",
			compactResponse,
			paddedResponse,
		)
	}
	if compactValid.NormalizedDigest() != paddedValid.NormalizedDigest() {
		t.Fatalf(
			"normalized semantic digests differ: %s != %s",
			compactValid.NormalizedDigest().String(),
			paddedValid.NormalizedDigest().String(),
		)
	}
	if compactValid.NormalizedDigest().String() == "" {
		t.Fatal("ValidResponse exposes an empty normalized semantic digest")
	}
	if compactValid.NormalizedDigest() == compactRawDigest ||
		paddedValid.NormalizedDigest() == paddedRawDigest {
		t.Fatal("public normalized digest reused a raw candidate digest")
	}
	assertNoAdmissionCapability(t, compactResponse)
	assertNoAdmissionCapability(t, paddedResponse)
}

func TestResolvedProjectBasisProjectsCoreInvalidAndUnderdetermined(t *testing.T) {
	tests := []struct {
		name    string
		posture entityPosture
		kind    typedmemory.ValidationVerdictKind
		code    string
	}{
		{
			name:    "known entity collision",
			posture: entityExact,
			kind:    typedmemory.ValidationInvalid,
			code:    string(typedmemory.DiagnosticEntityAlreadyExists),
		},
		{
			name:    "unsettled identity",
			posture: entityUnsettled,
			kind:    typedmemory.ValidationUnderdetermined,
			code:    string(typedmemory.DiagnosticIdentityBasisMissing),
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			digit := fmt.Sprintf("%x", index+8)
			environment := testEnvironment(t, digit)
			snapshot := newTestSnapshot(t, environment.Ref(), 31, test.posture)
			basis, err := NewResolvedProjectBasis(
				environment,
				typedmemory.NewCodecRegistry(),
				snapshot,
			)
			if err != nil {
				t.Fatalf("NewResolvedProjectBasis() error = %v", err)
			}
			resolver := &fixedResolver{resolution: basis}
			service := mustService(t, resolver)
			request := decodeRequest(t, `{"kind":"project_current"}`)

			response := service.Validate(request)

			if response.Verdict() != test.kind {
				t.Fatalf("Verdict() = %q, want %q", response.Verdict(), test.kind)
			}
			assertDiagnosticCode(t, response, test.code)
			if got := response.Diagnostics()[0].Path(); got != "$.change_set.changes[0].entity_id" {
				t.Fatalf("diagnostic path = %q", got)
			}
			payload, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				t.Fatalf("json.Marshal() error = %v", marshalErr)
			}
			if !strings.Contains(string(payload), `"path":"$.change_set.changes[0].entity_id"`) {
				t.Fatalf("response does not expose exact request JSON path: %s", payload)
			}
			assertNoNormalizedDigest(t, response)
			assertNoEffects(t, response)
		})
	}
}

func TestMalformedBindBecomesStructuredInvalid(t *testing.T) {
	environment := testEnvironment(t, "a")
	basis, err := NewBundledCandidateOpenWorldBasis(
		environment,
		typedmemory.NewCodecRegistry(),
	)
	if err != nil {
		t.Fatalf("NewBundledCandidateOpenWorldBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: basis}
	service := mustService(t, resolver)
	selectorRequest := decodeRequest(t, `{"kind":"bundled_candidate_open_world"}`)
	request := failingBindRequest{
		selector: selectorRequest.Basis(),
		cause:    errors.New("fixture binding contradiction"),
	}

	response := presentOutcome(service.evaluate(request))

	if response.Verdict() != typedmemory.ValidationInvalid {
		t.Fatalf("Verdict() = %q, want invalid", response.Verdict())
	}
	assertDiagnosticCode(t, response, DiagnosticChangeSetBindFailed)
	assertNoEffects(t, response)
}

func TestResponseIsDeterministicAndDefensivelyCopied(t *testing.T) {
	resolver := &fixedResolver{resolution: NewProjectBasisUnavailable()}
	service := mustService(t, resolver)
	request := decodeRequest(t, `{"kind":"project_current"}`)
	response := service.Validate(request)
	first, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal(first) error = %v", err)
	}
	if !strings.Contains(string(first), `"verdict":"underdetermined"`) {
		t.Fatalf("response omits canonical verdict field: %s", first)
	}
	if strings.Contains(string(first), `"kind":"underdetermined"`) {
		t.Fatalf("response exposes a second kind alias: %s", first)
	}

	for iteration := 0; iteration < 50; iteration++ {
		next := service.Validate(request)
		encoded, marshalErr := json.Marshal(next)
		if marshalErr != nil {
			t.Fatalf("json.Marshal(iteration %d) error = %v", iteration, marshalErr)
		}
		if string(encoded) != string(first) {
			t.Fatalf("response is nondeterministic:\nfirst=%s\nnext=%s", first, encoded)
		}
	}

	diagnostics := response.Diagnostics()
	diagnostics[0] = DiagnosticProjection{}
	if response.Diagnostics()[0].Code() == "" {
		t.Fatal("Diagnostics() exposed mutable response state")
	}
}

func TestCoreDiagnosticProjectionPreservesTypedWitnessBasisAndRepairs(t *testing.T) {
	environment := testEnvironment(t, "e")
	contexts := environment.BoundedContexts()
	provenance := contexts[0].Provenance()
	basis, err := typedmemory.NewKnownDeclarationBasis(provenance)
	if err != nil {
		t.Fatalf("NewKnownDeclarationBasis() error = %v", err)
	}
	expected, err := typedmemory.NewDiagnosticStateDatum("active-context")
	if err != nil {
		t.Fatalf("NewDiagnosticStateDatum(expected) error = %v", err)
	}
	actual, err := typedmemory.NewDiagnosticStateDatum("different-context")
	if err != nil {
		t.Fatalf("NewDiagnosticStateDatum(actual) error = %v", err)
	}
	witness, err := typedmemory.NewExpectedActualWitness(expected, actual)
	if err != nil {
		t.Fatalf("NewExpectedActualWitness() error = %v", err)
	}
	path, err := typedmemory.NewDiagnosticPath("changes[0].bounded_context")
	if err != nil {
		t.Fatalf("NewDiagnosticPath() error = %v", err)
	}
	pointer, err := typedmemory.NewRepairPointer("inspect-context-declaration")
	if err != nil {
		t.Fatalf("NewRepairPointer() error = %v", err)
	}
	target, err := typedmemory.NewDiagnosticReferenceDatum("context:fixture")
	if err != nil {
		t.Fatalf("NewDiagnosticReferenceDatum() error = %v", err)
	}
	repair, err := typedmemory.NewRepairCandidate(
		typedmemory.RepairInspectBasis,
		pointer,
		target,
		typedmemory.HumanChoiceRequired,
	)
	if err != nil {
		t.Fatalf("NewRepairCandidate() error = %v", err)
	}
	diagnostic, err := typedmemory.NewInvalidDiagnosticWithDetails(
		typedmemory.DiagnosticSignatureContextMismatch,
		"fixture context mismatch",
		path,
		witness,
		basis,
		[]typedmemory.RepairCandidate{repair},
	)
	if err != nil {
		t.Fatalf("NewInvalidDiagnosticWithDetails() error = %v", err)
	}
	request := decodeRequest(t, `{"kind":"project_current"}`)
	pathProjector, err := newDiagnosticPathProjector(request)
	if err != nil {
		t.Fatalf("newDiagnosticPathProjector() error = %v", err)
	}

	projections, err := projectCoreDiagnostics(
		[]typedmemory.Diagnostic{diagnostic},
		pathProjector,
	)
	if err != nil {
		t.Fatalf("projectCoreDiagnostics() error = %v", err)
	}
	projection := projections[0]
	if projection.Path() != "$.change_set.changes[0].context" {
		t.Fatalf("projected path = %q", projection.Path())
	}
	if projection.PathKind() != DiagnosticPathRequestJSON {
		t.Fatalf("projected path kind = %q", projection.PathKind())
	}
	if projection.Witness().Kind() != DiagnosticWitnessExpectedActual {
		t.Fatalf("witness kind = %q", projection.Witness().Kind())
	}
	if got, present := projection.Witness().Expected().Scalar(); !present || got != "active-context" {
		t.Fatalf("expected witness = %q, %v", got, present)
	}
	governing := projection.GoverningBasis()
	if governing.Kind() != typedmemory.DiagnosticBasisKnownDeclaration {
		t.Fatalf("governing basis kind = %q", governing.Kind())
	}
	if governing.provenance == nil || governing.provenance.kind != DeclarationProvenanceFPFSource {
		t.Fatalf("provenance projection = %#v", governing.provenance)
	}
	if len(governing.provenance.sources) != 1 {
		t.Fatalf("source provenance count = %d", len(governing.provenance.sources))
	}
	repairs := projection.RepairCandidates()
	if len(repairs) != 1 {
		t.Fatalf("repair count = %d", len(repairs))
	}
	if repairs[0].Kind() != typedmemory.RepairInspectBasis {
		t.Fatalf("repair kind = %q", repairs[0].Kind())
	}
	if repairs[0].Pointer() != "inspect-context-declaration" {
		t.Fatalf("repair pointer = %q", repairs[0].Pointer())
	}
	if repairs[0].HumanChoiceRequirement() != typedmemory.HumanChoiceRequired {
		t.Fatalf("human choice = %q", repairs[0].HumanChoiceRequirement())
	}
	if got, present := repairs[0].Target().Scalar(); !present || got != "context:fixture" {
		t.Fatalf("repair target = %q, %v", got, present)
	}
}

func TestDiagnosticDatumProjectionUsesClosedTypedWireShape(t *testing.T) {
	textDatum, err := typedmemory.NewDiagnosticTextDatum("fixture text")
	if err != nil {
		t.Fatalf("NewDiagnosticTextDatum() error = %v", err)
	}
	referenceDatum, err := typedmemory.NewDiagnosticReferenceDatum("entity:fixture")
	if err != nil {
		t.Fatalf("NewDiagnosticReferenceDatum() error = %v", err)
	}
	stateDatum, err := typedmemory.NewDiagnosticStateDatum("active")
	if err != nil {
		t.Fatalf("NewDiagnosticStateDatum() error = %v", err)
	}
	unknownDatum, err := typedmemory.NewUnknownDiagnosticDatum("snapshot unavailable")
	if err != nil {
		t.Fatalf("NewUnknownDiagnosticDatum() error = %v", err)
	}
	setDatum, err := typedmemory.NewDiagnosticSetDatum([]string{"beta", "alpha"})
	if err != nil {
		t.Fatalf("NewDiagnosticSetDatum() error = %v", err)
	}
	tests := []struct {
		name  string
		datum typedmemory.DiagnosticDatum
		want  string
	}{
		{name: "text", datum: textDatum, want: `{"kind":"text","scalar":"fixture text"}`},
		{name: "reference", datum: referenceDatum, want: `{"kind":"reference","scalar":"entity:fixture"}`},
		{name: "state", datum: stateDatum, want: `{"kind":"state","scalar":"active"}`},
		{name: "unknown", datum: unknownDatum, want: `{"kind":"unknown","scalar":"snapshot unavailable"}`},
		{name: "count", datum: typedmemory.NewDiagnosticCountDatum(0), want: `{"kind":"count","count":0}`},
		{name: "set", datum: setDatum, want: `{"kind":"set","set_values":["alpha","beta"]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projection, projectionErr := projectDiagnosticDatum(test.datum)
			if projectionErr != nil {
				t.Fatalf("projectDiagnosticDatum() error = %v", projectionErr)
			}
			payload, marshalErr := json.Marshal(projection)
			if marshalErr != nil {
				t.Fatalf("json.Marshal() error = %v", marshalErr)
			}
			if string(payload) != test.want {
				t.Fatalf("payload = %s, want %s", payload, test.want)
			}
			if strings.Contains(string(payload), `"values"`) {
				t.Fatalf("payload exposes universal string values: %s", payload)
			}
		})
	}
}

func TestResolvedProjectBasisRejectsSnapshotFromAnotherTypeEnv(t *testing.T) {
	environment := testEnvironment(t, "b")
	otherEnvironment := testEnvironment(t, "c")
	snapshot := newTestSnapshot(t, otherEnvironment.Ref(), 1, entityAbsent)

	_, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)

	if err == nil {
		t.Fatal("NewResolvedProjectBasis() accepted a snapshot from another TypeEnv")
	}
}

type canonicalDigestFixture struct {
	environment typedmemory.TypeEnv
	registry    typedmemory.CodecRegistry
	snapshot    *testSnapshot
	context     typedmemory.BoundedContextRef
	valueKind   typedmemory.ValueKindRef
	shape       typedmemory.ValueShapeRef
	codec       typedmemory.CodecRef
	signature   typedmemory.RelationSignatureRef
	slot        typedmemory.SlotKindID
	assertion   typedmemory.AssertionID
}

func newCanonicalDigestFixture(t *testing.T) canonicalDigestFixture {
	t.Helper()
	base := testEnvironment(t, "f")
	context := base.BoundedContexts()[0]
	provenance := context.Provenance()

	kindID, err := typedmemory.NewKindID("Haft.CanonicalText")
	if err != nil {
		t.Fatalf("NewKindID() error = %v", err)
	}
	kind, err := typedmemory.NewKindDefinition(kindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition() error = %v", err)
	}
	valueKind, err := typedmemory.NewValueKindRef(base.Ref(), kindID)
	if err != nil {
		t.Fatalf("NewValueKindRef() error = %v", err)
	}
	admission := testLocalContextKindAvailability(
		t,
		base.Ref(),
		context.Ref(),
		kindID,
		provenance,
		"validation.canonical-digest.availability",
	)

	shapeID, err := typedmemory.NewShapeID("Haft.CanonicalTextShape")
	if err != nil {
		t.Fatalf("NewShapeID() error = %v", err)
	}
	shape, err := typedmemory.NewScalarShape(typedmemory.ScalarText)
	if err != nil {
		t.Fatalf("NewScalarShape() error = %v", err)
	}
	shapeRef, err := typedmemory.DeriveValueShapeRef(shapeID, shape)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef() error = %v", err)
	}
	shapeDeclaration, err := typedmemory.NewValueShapeDeclaration(
		shapeRef,
		shape,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewValueShapeDeclaration() error = %v", err)
	}

	codecID, err := typedmemory.NewCodecID("Haft.TrimmedTextCodec")
	if err != nil {
		t.Fatalf("NewCodecID() error = %v", err)
	}
	codecVersion, err := typedmemory.NewCanonicalizationVersion("v1")
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion() error = %v", err)
	}
	codecDigest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat("2", 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest(codec) error = %v", err)
	}
	codecRef, err := typedmemory.NewCodecRef(codecID, codecVersion, codecDigest)
	if err != nil {
		t.Fatalf("NewCodecRef() error = %v", err)
	}
	binding, err := typedmemory.NewValueBinding(
		valueKind,
		shapeRef,
		codecRef,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewValueBinding() error = %v", err)
	}

	slotID, err := typedmemory.NewSlotKindID("Haft.CanonicalTextSlot")
	if err != nil {
		t.Fatalf("NewSlotKindID() error = %v", err)
	}
	slotTarget, err := typedmemory.NewValueSlotTarget(valueKind)
	if err != nil {
		t.Fatalf("NewValueSlotTarget() error = %v", err)
	}
	slot, err := typedmemory.NewSlotSpec(
		slotID,
		slotTarget,
		typedmemory.ExactlyOneCardinality(),
		provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec() error = %v", err)
	}
	signatureID, err := typedmemory.NewSignatureID("Haft.CanonicalTextRelation")
	if err != nil {
		t.Fatalf("NewSignatureID() error = %v", err)
	}
	signatureRef, err := typedmemory.NewRelationSignatureRef(base.Ref(), signatureID)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef() error = %v", err)
	}
	signature, err := typedmemory.NewRelationSignature(
		signatureRef,
		[]typedmemory.BoundedContextRef{context.Ref()},
		[]typedmemory.SlotSpec{slot},
		provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature() error = %v", err)
	}

	builder := typedmemory.NewTypeEnvBuilder(base.Ref())
	builder = builder.SetSourceRevision(base.SourceRevision())
	builder = builder.SetCompilerSchemaVersion(base.CompilerSchemaVersion())
	builder = builder.SetCoverageManifest(base.CoverageManifest())
	builder = builder.AddBoundedContext(context)
	builder = builder.AddKindDefinition(kind)
	builder = builder.AddContextKindAvailability(admission)
	builder = builder.AddValueShape(shapeDeclaration)
	builder = builder.AddValueBinding(binding)
	builder = builder.AddRelationSignature(signature)
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("TypeEnvBuilder.Build() error = %v", err)
	}
	registry, err := typedmemory.NewCodecRegistry().Register(
		codecRef,
		trimmingTextCodec{shape: shapeRef},
	)
	if err != nil {
		t.Fatalf("CodecRegistry.Register() error = %v", err)
	}
	assertion, err := typedmemory.NewAssertionID("assertion:canonical-text")
	if err != nil {
		t.Fatalf("NewAssertionID() error = %v", err)
	}
	rule, err := typedmemory.NewRuleRef("fixture:assertion-absence")
	if err != nil {
		t.Fatalf("NewRuleRef() error = %v", err)
	}
	absent, err := typedmemory.NewAbsentAssertionState(assertion, rule)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState() error = %v", err)
	}
	snapshot := newTestSnapshot(t, environment.Ref(), 41, entityAbsent)
	snapshot.assertionState = absent

	return canonicalDigestFixture{
		environment: environment,
		registry:    registry,
		snapshot:    snapshot,
		context:     context.Ref(),
		valueKind:   valueKind,
		shape:       shapeRef,
		codec:       codecRef,
		signature:   signatureRef,
		slot:        slotID,
		assertion:   assertion,
	}
}

func (fixture canonicalDigestFixture) decodeRequest(
	t *testing.T,
	input []byte,
) typedmemorywire.ValidateRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": {"kind": "project_current"},
  "change_set": {
    "changes": [{
      "kind": "instantiate_relation",
      "assertion_id": %q,
      "signature_id": %q,
      "context_slice": {
        "context": %q,
        "standard_pins": [],
        "environment_selectors": [],
        "vocabulary_pins": [],
        "role_set_pins": [],
        "gamma_time": {
          "kind": "point",
          "at": "2026-07-16T08:00:00Z"
        }
      },
      "bindings": [{
        "slot_kind": %q,
        "fillers": [{
          "kind": "by_value",
          "value": {
            "value_kind": %q,
            "value_shape": {"id": %q, "digest": %q},
            "codec": {
              "id": %q,
              "version": %q,
              "specification_digest": %q
            },
            "input_base64": %q,
            "asserted_digest": {"kind": "none"}
          }
        }]
      }],
      "provenance": "fixture:canonical-digest"
    }]
  }
}`,
		typedmemorywire.ContractVersion,
		fixture.assertion.String(),
		fixture.signature.ID().String(),
		fixture.context.String(),
		fixture.slot.String(),
		fixture.valueKind.ID().String(),
		fixture.shape.ID().String(),
		fixture.shape.Digest().String(),
		fixture.codec.ID().String(),
		fixture.codec.Version().String(),
		fixture.codec.SpecificationDigest().String(),
		base64.StdEncoding.EncodeToString(input),
	)
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v\npayload=%s", err, payload)
	}
	return request
}

type trimmingTextCodec struct {
	shape typedmemory.ValueShapeRef
}

func (codec trimmingTextCodec) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	input []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return typedmemory.RejectedCodecValue{}
	}
	text := strings.TrimSpace(string(input))
	canonical, err := typedmemory.NewCanonicalizedCodecValue(
		typedmemory.NewTextValue(text),
		[]byte(text),
	)
	if err != nil {
		return typedmemory.RejectedCodecValue{}
	}
	return canonical
}

type fixedResolver struct {
	resolution BasisResolution
	lastKind   typedmemorywire.BasisKind
	callCount  int
}

func (resolver *fixedResolver) Resolve(selector typedmemorywire.BasisSelector) BasisResolution {
	resolver.callCount++
	resolver.lastKind = selector.Kind()
	return resolver.resolution
}

type trackingRequest struct {
	request   typedmemorywire.ValidateRequest
	bindCount int
}

func (request *trackingRequest) ContractVersion() string {
	return request.request.ContractVersion()
}

func (request *trackingRequest) Action() string { return request.request.Action() }

func (request *trackingRequest) Basis() typedmemorywire.BasisSelector {
	return request.request.Basis()
}

func (request *trackingRequest) ChangeCount() int { return request.request.ChangeCount() }

func (request *trackingRequest) DiagnosticCoordinates() typedmemorywire.DiagnosticCoordinateIndex {
	return request.request.DiagnosticCoordinates()
}

func (request *trackingRequest) BindChangeSet(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.MemoryChangeSet, error) {
	request.bindCount++
	return request.request.BindChangeSet(typeEnv)
}

type failingBindRequest struct {
	selector typedmemorywire.BasisSelector
	cause    error
}

func (failingBindRequest) ContractVersion() string { return typedmemorywire.ContractVersion }

func (failingBindRequest) Action() string { return typedmemorywire.ActionValidate }

func (request failingBindRequest) Basis() typedmemorywire.BasisSelector {
	return request.selector
}

func (failingBindRequest) ChangeCount() int { return 1 }

func (request failingBindRequest) BindChangeSet(
	typedmemory.TypeEnvRef,
) (typedmemory.MemoryChangeSet, error) {
	return typedmemory.MemoryChangeSet{}, request.cause
}

type entityPosture uint8

const (
	entityAbsent entityPosture = iota + 1
	entityExact
	entityUnsettled
)

type testSnapshot struct {
	testing        *testing.T
	typeEnv        typedmemory.TypeEnvRef
	revision       typedmemory.GraphRevision
	posture        entityPosture
	assertionState typedmemory.AssertionState
	entityCalls    int
	referenceCalls int
}

func newTestSnapshot(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	revision uint64,
	posture entityPosture,
) *testSnapshot {
	return &testSnapshot{
		testing:  t,
		typeEnv:  typeEnv,
		revision: typedmemory.NewGraphRevision(revision),
		posture:  posture,
	}
}

func (snapshot *testSnapshot) resetCalls() {
	snapshot.entityCalls = 0
	snapshot.referenceCalls = 0
}

func (snapshot *testSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot *testSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot *testSnapshot) ResolveEntity(
	entity typedmemory.EntityID,
	context typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	snapshot.entityCalls++
	switch snapshot.posture {
	case entityAbsent:
		basis, err := typedmemory.NewResolutionBasisRef("fixture:absence")
		if err != nil {
			snapshot.testing.Fatalf("NewResolutionBasisRef() error = %v", err)
		}
		resolution, err := typedmemory.NewAbsentEntityResolution(entity, context, basis)
		if err != nil {
			snapshot.testing.Fatalf("NewAbsentEntityResolution() error = %v", err)
		}
		return resolution
	case entityExact:
		basis, err := typedmemory.NewResolutionBasisRef("fixture:exact")
		if err != nil {
			snapshot.testing.Fatalf("NewResolutionBasisRef() error = %v", err)
		}
		resolution, err := typedmemory.NewExactEntityResolution(entity, context, basis)
		if err != nil {
			snapshot.testing.Fatalf("NewExactEntityResolution() error = %v", err)
		}
		return resolution
	case entityUnsettled:
		missing, err := typedmemory.NewMissingBasis("fixture:snapshot")
		if err != nil {
			snapshot.testing.Fatalf("NewMissingBasis() error = %v", err)
		}
		values := []typedmemory.MissingBasis{missing}
		resolution, err := typedmemory.NewUnsettledEntityResolution("fixture entity", values)
		if err != nil {
			snapshot.testing.Fatalf("NewUnsettledEntityResolution() error = %v", err)
		}
		return resolution
	default:
		return nil
	}
}

func (snapshot *testSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	snapshot.referenceCalls++
	return nil
}

func (snapshot *testSnapshot) EvaluateMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	query := request.Query()
	missing, err := typedmemory.MissingKindSignatureForMemberOf(query)
	if err != nil {
		snapshot.testing.Fatalf("MissingKindSignatureForMemberOf() error = %v", err)
	}
	repair, err := typedmemory.NewRepairPointer("repair:resolve-member-of-kind-signature")
	if err != nil {
		snapshot.testing.Fatalf("NewRepairPointer() error = %v", err)
	}
	judgement, err := typedmemory.NewMemberOfUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		snapshot.testing.Fatalf("NewMemberOfUndefined() error = %v", err)
	}
	return judgement
}

func (snapshot *testSnapshot) AssertionState(typedmemory.AssertionID) typedmemory.AssertionState {
	return snapshot.assertionState
}

func (*testSnapshot) ResolveAlias(
	typedmemory.EntityAlias,
	typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return nil
}

func (snapshot *testSnapshot) ResolveReconciliationBasis(
	basis typedmemory.ReconciliationBasisRef,
	context typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	resolution, err := typedmemory.NewMissingReconciliationBasis(basis, context)
	if err != nil {
		snapshot.testing.Fatalf("NewMissingReconciliationBasis() error = %v", err)
	}
	return resolution
}

func decodeRequest(t *testing.T, basisJSON string) typedmemorywire.ValidateRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": %s,
  "change_set": {
    "changes": [{
      "kind": "declare_entity",
      "entity_id": "entity:fixture",
      "local_ref": "local:fixture",
      "context": "context:fixture",
      "label": "Fixture entity",
      "provenance": "provenance:fixture"
    }]
  }
}`, typedmemorywire.ContractVersion, basisJSON)
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v\npayload=%s", err, payload)
	}
	return request
}

func decodeRelationalAssertionRequest(t *testing.T) typedmemorywire.ValidateRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": {"kind": "project_current"},
  "change_set": {
    "changes": [{
      "kind": "assert_relation",
      "assertion_id": "assertion:v2-response-version",
      "signature_id": "Local.Relation",
      "context_slice": {
        "context": "context:fixture",
        "standard_pins": [],
        "environment_selectors": [],
        "vocabulary_pins": [],
        "role_set_pins": [],
        "gamma_time": {
          "kind": "point",
          "at": "2026-07-20T00:00:00Z"
        }
      },
      "modality": {"kind": "obtaining_unknown"},
      "bindings": [{
        "slot_kind": "Local.EntitySlot",
        "fillers": [{
          "kind": "by_reference",
          "reference": {
            "kind": "persisted",
            "ref_kind": "U.EntityRef",
            "id": "entity:fixture"
          }
        }]
      }],
      "provenance": "provenance:v2-response-version"
    }]
  }
}`, typedmemorywire.ContractVersionV2)
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest(v2 assertion): %v\npayload=%s", err, payload)
	}
	return request
}

func testEnvironment(t *testing.T, digit string) typedmemory.TypeEnv {
	t.Helper()
	digestText := "sha256:" + strings.Repeat(digit, 64)
	digest, err := typedmemory.NewSHA256Digest(digestText)
	mustNoError(t, err)
	typeEnv, err := typedmemory.NewTypeEnvRef(digest)
	mustNoError(t, err)
	revision, err := typedmemory.NewSourceRevision("revision:fixture:" + digit)
	mustNoError(t, err)
	compiler, err := typedmemory.NewCompilerSchemaVersion("compiler:fixture:v1")
	mustNoError(t, err)
	unitID, err := typedmemory.NewSourceUnitID("source:fixture:" + digit)
	mustNoError(t, err)
	contentText := "sha256:" + strings.Repeat("d", 64)
	contentHash, err := typedmemory.NewSHA256Digest(contentText)
	mustNoError(t, err)
	lineRange, err := typedmemory.NewSourceLineRange(1, 1)
	mustNoError(t, err)
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unitID,
		revision,
		contentHash,
		lineRange,
	)
	mustNoError(t, err)
	subject, err := typedmemory.SourceUnitCoverage(unitID)
	mustNoError(t, err)
	entry, err := typedmemory.NewCompiledCoverageEntry(subject, location)
	mustNoError(t, err)
	entries := []typedmemory.CoverageEntry{entry}
	manifest, err := typedmemory.NewCoverageManifest(entries)
	mustNoError(t, err)
	provenanceRef, err := typedmemory.NewProvenanceRef("provenance:context:" + digit)
	mustNoError(t, err)
	ruleID, err := typedmemory.NewCompilerRuleID("fixture.context")
	mustNoError(t, err)
	provenance, err := typedmemory.NewFPFSourceProvenance(provenanceRef, location, ruleID)
	mustNoError(t, err)
	contextRef, err := typedmemory.NewBoundedContextRef("context:fixture")
	mustNoError(t, err)
	context, err := typedmemory.NewBoundedContext(contextRef, provenance)
	mustNoError(t, err)
	builder := typedmemory.NewTypeEnvBuilder(typeEnv)
	builder = builder.SetSourceRevision(revision)
	builder = builder.SetCompilerSchemaVersion(compiler)
	builder = builder.SetCoverageManifest(manifest)
	builder = builder.AddBoundedContext(context)
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("TypeEnvBuilder.Build() error = %v", err)
	}
	return environment
}

func testLocalContextKindAvailability(
	t *testing.T,
	base typedmemory.TypeEnvRef,
	context typedmemory.BoundedContextRef,
	kind typedmemory.KindID,
	upstream typedmemory.DeclarationProvenance,
	fixtureID string,
) typedmemory.ContextKindAvailability {
	t.Helper()
	symbol, err := typedmemory.KindSymbolRef(kind)
	mustNoError(t, err)
	manifest, err := typedmemory.NewSignatureManifestRef(fixtureID, "1.0.0")
	mustNoError(t, err)
	basis, err := typedmemory.NewManifestSymbolBasis(
		manifest,
		typedmemory.ManifestProvide,
		symbol,
	)
	mustNoError(t, err)
	digestSource := append(
		[]byte(fixtureID+"\x00"+context.String()+"\x00"+kind.String()+"\x00"),
		upstream.CanonicalBytes()...,
	)
	sum := sha256.Sum256(digestSource)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	mustNoError(t, err)
	reference, err := typedmemory.NewProvenanceRef("prov:" + fixtureID)
	mustNoError(t, err)
	carrier, err := typedmemory.NewCarrierRef("carrier:" + fixtureID)
	mustNoError(t, err)
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	mustNoError(t, err)
	lineRange, err := typedmemory.NewSourceLineRange(1, 1)
	mustNoError(t, err)
	rule, err := typedmemory.NewCompilerRuleID("fixture.context-kind-availability.v1")
	mustNoError(t, err)
	projectProvenance, err := typedmemory.NewProjectSourceProvenanceBuilder(
		reference,
		carrier,
		edition,
		digest,
	).
		SetDeclarationRange(lineRange).
		SetCompilerRule(rule).
		SetBoundedContext(context).
		SetBaseTypeEnv(base).
		SetSignatureBlockRow(typedmemory.VocabularyRow).
		SetManifestBasis(basis).
		Build()
	mustNoError(t, err)
	contextSource, err := typedmemory.NewContextKindAvailabilitySource(
		context.String(),
		projectProvenance,
	)
	mustNoError(t, err)
	declarationSource, err := typedmemory.NewContextKindAvailabilitySource(
		kind.String(),
		projectProvenance,
	)
	mustNoError(t, err)
	extensionRef, err := typedmemory.ParseTypeEnvExtensionRef(
		"typeenv-extension:" + manifest.ID() + "@" + digest.String(),
	)
	mustNoError(t, err)
	provider, err := typedmemory.NewExtensionKindAvailabilityProvider(
		typedmemory.ExtensionKindAvailabilityProviderInput{
			ExtensionRef:      extensionRef,
			Context:           context,
			ContextSource:     contextSource,
			Symbol:            symbol,
			DeclarationSource: declarationSource,
		},
	)
	mustNoError(t, err)
	ground, err := typedmemory.NewLocalContextKindAvailabilityGround(
		typedmemory.LocalContextKindAvailabilityGroundInput{
			Context:             context,
			KindID:              kind,
			ContextSource:       contextSource,
			ApplicabilitySource: contextSource,
			Provider:            provider,
		},
	)
	mustNoError(t, err)
	grounds, err := typedmemory.NewContextKindAvailabilityGroundSet(
		[]typedmemory.ContextKindAvailabilityGround{ground},
	)
	mustNoError(t, err)
	availability, err := typedmemory.NewContextKindAvailability(context, kind, grounds)
	mustNoError(t, err)
	return availability
}

func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("fixture constructor error = %v", err)
	}
}

func mustService(t *testing.T, resolver BasisResolver) Service {
	t.Helper()
	service, err := NewService(resolver)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func assertDiagnosticCode(t *testing.T, response Response, code string) {
	t.Helper()
	for _, diagnostic := range response.Diagnostics() {
		if diagnostic.Code() == code {
			return
		}
	}
	t.Fatalf("diagnostic %q missing from %#v", code, response.Diagnostics())
}

func assertNoEffects(t *testing.T, response Response) {
	t.Helper()
	disposition := response.PersistenceDisposition()
	if disposition.Mode() != PersistenceValidationOnlyNoWrite {
		t.Fatalf("persistence mode = %q", disposition.Mode())
	}
	if disposition.RowsWritten() != 0 {
		t.Fatalf("rows written = %d", disposition.RowsWritten())
	}
	if disposition.AuthorityGranted() {
		t.Fatal("validation response granted authority")
	}
}

func assertNoAdmissionCapability(t *testing.T, response Response) {
	t.Helper()
	typeOf := reflect.TypeOf(response)
	if _, exists := typeOf.MethodByName("AdmissionBatch"); exists {
		t.Fatalf("response type %T exposes AdmissionBatch", response)
	}
	if _, exists := typeOf.MethodByName("ChangeSet"); exists {
		t.Fatalf("response type %T exposes validated ChangeSet", response)
	}
	if _, exists := typeOf.MethodByName("AdmissionEnvelopeDigest"); exists {
		t.Fatalf("response type %T exposes an admission-envelope identity", response)
	}
	if _, exists := typeOf.MethodByName("CanonicalEnvelopeBytes"); exists {
		t.Fatalf("response type %T exposes admission-envelope bytes", response)
	}
}

func assertNoNormalizedDigest(t *testing.T, response Response) {
	t.Helper()
	if _, exists := response.(ValidResponse); exists {
		t.Fatalf("response type %T exposes normalized digest", response)
	}
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(payload), "normalized_digest") {
		t.Fatalf("non-Valid response contains normalized digest: %s", payload)
	}
}
