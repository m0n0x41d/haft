package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/onboarding"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestOnboardMCPPreInitReturnsReadableRecoveryWithoutEffects(
	t *testing.T,
) {
	t.Parallel()

	handler, err := newOnboardingRequiredMCPHandler()
	if err != nil {
		t.Fatal(err)
	}
	output, err := handler(
		context.Background(),
		json.RawMessage(`{"action":"status"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	response := decodeOnboardResponse(t, output)
	if response.Action != "status" ||
		response.Result != "onboarding_required" ||
		response.Status != "needs_init" {
		t.Fatalf(
			"pre-init response = %#v",
			response,
		)
	}
	if response.Effects.RepositoryInspected ||
		response.Effects.ReviewCarrierCreated ||
		response.Effects.ReviewCarrierReused ||
		response.Effects.CanonicalProfileChanged ||
		response.Effects.StructuredMemoryEnabled ||
		response.Effects.AuthorityGranted {
		t.Fatalf(
			"pre-init effects = %#v",
			response.Effects,
		)
	}
	assertOnboardOutputHasNoInternalJargon(t, output)
}

func TestOnboardMCPStrictDecoderRejectsInvalidVariantsBeforeWork(
	t *testing.T,
) {
	t.Parallel()

	handler, err := newOnboardingRequiredMCPHandler()
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []string{
		`{"action":"status","action":"profile_prepare"}`,
		`{"action":"status","type_env":"internal"}`,
		`{"action":"status","basis":"not applicable"}`,
		`{"action":"memory_prepare","scopes":[]}`,
		`{"action":"profile_prepare","basis":"Missing scopes."}`,
		`{"action":"profile_prepare","scopes":[]}`,
		`{"action":"profile_change_prepare"}`,
		`{"action":"profile_change_prepare","scope_id":"app"}`,
		`{"action":"profile_change_prepare","entity_ref":"entity:target"}`,
		`{"action":"status"}{"action":"status"}`,
	}
	for index, fixture := range fixtures {
		if output, callErr := handler(
			context.Background(),
			json.RawMessage(fixture),
		); callErr == nil {
			t.Fatalf(
				"invalid request %d returned %q",
				index,
				output,
			)
		}
	}
}

func TestOnboardPublishedSchemaBoundsReachStrictHandler(t *testing.T) {
	t.Parallel()

	schema := onboardPublishedInputSchema(t)
	properties := onboardPublishedProperties(t, schema, "haft_onboard")
	scopesSchema, ok := properties["scopes"].(map[string]interface{})
	if !ok {
		t.Fatalf("scopes schema = %#v", properties["scopes"])
	}
	scopeSchema, ok := scopesSchema["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("scope item schema = %#v", scopesSchema["items"])
	}
	scopeProperties := onboardPublishedProperties(
		t,
		scopeSchema,
		"haft_onboard.scopes.items",
	)
	evidenceSchema, ok := scopeProperties["evidence_paths"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"evidence_paths schema = %#v",
			scopeProperties["evidence_paths"],
		)
	}
	evidenceItemSchema, ok := evidenceSchema["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("evidence path item schema = %#v", evidenceSchema["items"])
	}

	basisLimit := onboardPublishedInteger(t, properties["basis"], "maxLength")
	scopeLimit := onboardPublishedInteger(
		t,
		scopeProperties["scope_id"],
		"maxLength",
	)
	labelLimit := onboardPublishedInteger(
		t,
		scopeProperties["label"],
		"maxLength",
	)
	evidenceCountLimit := onboardPublishedInteger(
		t,
		evidenceSchema,
		"maxItems",
	)
	evidencePathLimit := onboardPublishedInteger(
		t,
		evidenceItemSchema,
		"maxLength",
	)
	changeScopeLimit := onboardPublishedInteger(
		t,
		properties["scope_id"],
		"maxLength",
	)
	entityRefLimit := onboardPublishedInteger(
		t,
		properties["entity_ref"],
		"maxLength",
	)
	if basisLimit != onboarding.MaximumProfileBasisBytes ||
		scopeLimit != onboarding.MaximumScopeIDBytes ||
		changeScopeLimit != onboarding.MaximumScopeIDBytes ||
		entityRefLimit != onboarding.MaximumEntityRefBytes ||
		labelLimit != onboarding.MaximumScopeLabelBytes ||
		evidenceCountLimit != onboarding.MaximumEvidencePaths ||
		evidencePathLimit != onboarding.MaximumEvidencePathBytes {
		t.Fatalf(
			"published bounds = basis:%d scope:%d change_scope:%d entity:%d label:%d evidence:%d/%d",
			basisLimit,
			scopeLimit,
			changeScopeLimit,
			entityRefLimit,
			labelLimit,
			evidenceCountLimit,
			evidencePathLimit,
		)
	}

	handler, err := newOnboardingRequiredMCPHandler()
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{
		"status",
		"profile_prepare",
		"memory_prepare",
	} {
		payload := json.RawMessage(`{"action":"` + action + `"}`)
		if _, callErr := handler(
			context.Background(),
			payload,
		); callErr != nil {
			t.Fatalf("published %s variant failed: %v", action, callErr)
		}
	}
	if _, callErr := handler(
		context.Background(),
		json.RawMessage(
			`{"action":"profile_change_prepare","scope_id":"app","entity_ref":"entity:target"}`,
		),
	); callErr != nil {
		t.Fatalf("published profile_change_prepare variant failed: %v", callErr)
	}

	evidencePaths := make([]string, evidenceCountLimit)
	evidencePaths[0] = strings.Repeat("p", evidencePathLimit)
	for index := 1; index < evidenceCountLimit; index++ {
		evidencePaths[index] = fmt.Sprintf("evidence-%03d", index)
	}
	scopes := []onboardScopeWire{{
		ScopeID:         strings.Repeat("s", scopeLimit),
		Label:           strings.Repeat("l", labelLimit),
		RealizationKind: "software",
		EvidencePaths:   evidencePaths,
	}}
	request := onboardRequestWire{
		Action: "profile_prepare",
		Basis:  stringPointer(strings.Repeat("b", basisLimit)),
		Scopes: &scopes,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	output, err := handler(context.Background(), payload)
	if err != nil {
		t.Fatalf("published maximum-bound request failed: %v", err)
	}
	response := decodeOnboardResponse(t, output)
	if response.Result != "onboarding_required" ||
		response.Status != "needs_init" {
		t.Fatalf("maximum-bound response = %#v", response)
	}
}

type toggledOnboardRuntime struct {
	observation  onboarding.Observation
	profileCalls int
	memoryCalls  int
}

func (runtime *toggledOnboardRuntime) Observe(
	context.Context,
) (onboarding.Observation, error) {
	return runtime.observation, nil
}

func (runtime *toggledOnboardRuntime) PrepareProfile(
	context.Context,
	onboarding.Request,
) (onboarding.Preparation, error) {
	runtime.profileCalls++
	return onboarding.Preparation{}, fmt.Errorf(
		"unexpected profile preparation",
	)
}

func (runtime *toggledOnboardRuntime) PrepareProfileChange(
	context.Context,
	onboarding.Request,
) (onboarding.Preparation, error) {
	return onboarding.Preparation{}, fmt.Errorf(
		"unexpected profile change preparation",
	)
}

func (runtime *toggledOnboardRuntime) PrepareMemory(
	context.Context,
) (onboarding.Preparation, error) {
	runtime.memoryCalls++
	return onboarding.Preparation{}, fmt.Errorf(
		"unexpected memory preparation",
	)
}

func TestOnboardMCPStaleProcessReturnsRestartRequiredWithoutPreparation(
	t *testing.T,
) {
	t.Parallel()

	ready, err := onboarding.NewObservation(
		onboarding.ObservationInput{
			Initialized:     true,
			ProfileDeclared: true,
			MemoryReady:     true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &toggledOnboardRuntime{
		observation: ready,
	}
	service, err := onboarding.NewService(runtime, false)
	if err != nil {
		t.Fatal(err)
	}
	output, err := executeOnboardMCP(
		context.Background(),
		service,
		json.RawMessage(`{"action":"memory_prepare"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	response := decodeOnboardResponse(t, output)
	if response.Result != "restart_required" ||
		response.Status != "ready" {
		t.Fatalf(
			"restart response = %#v",
			response,
		)
	}
	if runtime.profileCalls != 0 ||
		runtime.memoryCalls != 0 {
		t.Fatalf(
			"stale process invoked preparation: %d/%d",
			runtime.profileCalls,
			runtime.memoryCalls,
		)
	}
	if response.Effects.ReviewCarrierCreated ||
		response.Effects.ReviewCarrierReused ||
		response.Effects.CanonicalProfileChanged ||
		response.Effects.StructuredMemoryEnabled ||
		response.Effects.AuthorityGranted {
		t.Fatalf(
			"restart effects = %#v",
			response.Effects,
		)
	}
	assertOnboardOutputHasNoInternalJargon(t, output)
}

func TestOnboardMCPManualFallbackCoversUnsupportedDocsEmptyAndMultipleScopes(
	t *testing.T,
) {
	fixtures := []struct {
		name   string
		files  []string
		scopes []onboardScopeWire
	}{
		{
			name:  "zig",
			files: []string{"build.zig", "src/main.zig"},
			scopes: []onboardScopeWire{{
				ScopeID:         "zig-app",
				Label:           "Zig application",
				RealizationKind: "software",
				EvidencePaths:   []string{"build.zig"},
			}},
		},
		{
			name:  "elixir",
			files: []string{"lib/app.ex", "mix.exs"},
			scopes: []onboardScopeWire{{
				ScopeID:         "elixir-app",
				Label:           "Elixir application",
				RealizationKind: "software",
				EvidencePaths:   []string{"mix.exs"},
			}},
		},
		{
			name:  "dart",
			files: []string{"lib/main.dart", "pubspec.yaml"},
			scopes: []onboardScopeWire{{
				ScopeID:         "dart-app",
				Label:           "Dart application",
				RealizationKind: "software",
				EvidencePaths:   []string{"pubspec.yaml"},
			}},
		},
		{
			name:  "docs only",
			files: []string{"README.md"},
			scopes: []onboardScopeWire{{
				ScopeID:         "handbook",
				Label:           "Project handbook",
				RealizationKind: "non_software",
				EvidencePaths:   []string{"README.md"},
			}},
		},
		{
			name: "empty mixed fallback",
			scopes: []onboardScopeWire{
				{
					ScopeID:         "future-app",
					Label:           "Future application",
					RealizationKind: "software",
					EvidencePaths:   []string{},
				},
				{
					ScopeID:         "future-guide",
					Label:           "Future guide",
					RealizationKind: "non_software",
					EvidencePaths:   []string{},
				},
			},
		},
	}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			project := newCLIProfileOnboardLedgerFixture(t)
			for _, relative := range fixture.files {
				writeProfileInspectionFixture(
					t,
					project.root,
					relative,
				)
			}
			binding := mustOnboardProjectBinding(
				t,
				project.root,
			)
			surface, err := openSealedProjectOnboardSurface(
				context.Background(),
				binding,
			)
			if err != nil {
				t.Fatal(err)
			}
			defer surface.Close()
			handler := surface.Handler()

			statusOutput := callOnboardHandler(
				t,
				handler,
				`{"action":"status"}`,
			)
			status := decodeOnboardResponse(
				t,
				statusOutput,
			)
			if status.Result != "needs_profile" ||
				status.Status != "needs_profile" {
				t.Fatalf(
					"initial status = %#v",
					status,
				)
			}
			assertOnboardOutputHasNoInternalJargon(
				t,
				statusOutput,
			)
			assertCLIProfileOnboardMutationCounts(
				t,
				project.root,
				0,
			)

			autoOutput := callOnboardHandler(
				t,
				handler,
				`{"action":"profile_prepare"}`,
			)
			auto := decodeOnboardResponse(t, autoOutput)
			if auto.Result != "needs_scope_review" ||
				auto.Status != "needs_profile" {
				t.Fatalf(
					"automatic fallback = %#v",
					auto,
				)
			}
			if _, statErr := os.Lstat(
				profileDeclarationReviewPath(
					project.root,
				),
			); !os.IsNotExist(statErr) {
				t.Fatalf(
					"insufficient detector wrote review: %v",
					statErr,
				)
			}

			request := onboardRequestWire{
				Action: "profile_prepare",
				Basis: stringPointer(
					"Explicit project scope for an unsupported or empty repository.",
				),
				Scopes: &fixture.scopes,
			}
			requestBytes, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			preparedOutput := callOnboardHandler(
				t,
				handler,
				string(requestBytes),
			)
			prepared := decodeOnboardResponse(
				t,
				preparedOutput,
			)
			if prepared.Result !=
				"profile_review_prepared" ||
				prepared.Status !=
					"profile_review_ready" {
				t.Fatalf(
					"manual prepare = %#v",
					prepared,
				)
			}
			if prepared.ReviewRef !=
				"review:onboard-profile" ||
				len(prepared.Scopes) !=
					len(fixture.scopes) {
				t.Fatalf(
					"manual review projection = %#v",
					prepared,
				)
			}
			if !prepared.Effects.ReviewCarrierCreated ||
				prepared.Effects.CanonicalProfileChanged ||
				prepared.Effects.StructuredMemoryEnabled ||
				prepared.Effects.AuthorityGranted {
				t.Fatalf(
					"manual prepare effects = %#v",
					prepared.Effects,
				)
			}
			assertOnboardOutputHasNoInternalJargon(
				t,
				preparedOutput,
			)
			assertCLIProfileOnboardMutationCounts(
				t,
				project.root,
				0,
			)

			reusedOutput := callOnboardHandler(
				t,
				handler,
				string(requestBytes),
			)
			reused := decodeOnboardResponse(
				t,
				reusedOutput,
			)
			if reused.Result !=
				"profile_review_reused" ||
				!reused.Effects.ReviewCarrierReused ||
				reused.Effects.ReviewCarrierCreated {
				t.Fatalf(
					"manual reuse = %#v",
					reused,
				)
			}

			readyOutput := callOnboardHandler(
				t,
				handler,
				`{"action":"status"}`,
			)
			ready := decodeOnboardResponse(
				t,
				readyOutput,
			)
			if ready.Result !=
				"profile_review_ready" ||
				ready.Status !=
					"profile_review_ready" ||
				len(ready.Scopes) !=
					len(fixture.scopes) {
				t.Fatalf(
					"review-ready status = %#v",
					ready,
				)
			}
			readyByID := make(
				map[string]onboardScopeResponseWire,
				len(ready.Scopes),
			)
			for _, scope := range ready.Scopes {
				readyByID[scope.ScopeID] = scope
			}
			for _, expected := range fixture.scopes {
				actual, present := readyByID[expected.ScopeID]
				if !present ||
					actual.Label != expected.Label ||
					actual.RealizationKind != expected.RealizationKind ||
					!reflect.DeepEqual(
						actual.EvidencePaths,
						expected.EvidencePaths,
					) {
					t.Fatalf(
						"status scope %q = %#v, want %#v",
						expected.ScopeID,
						actual,
						expected,
					)
				}
			}
		})
	}
}

func TestOnboardMCPDetectedMixedProjectPreparesNormalMultiScopeReview(
	t *testing.T,
) {
	project := newCLIProfileOnboardLedgerFixture(t)
	for _, relative := range []string{
		"go.mod",
		"internal/kernel.go",
		"models/current.onnx",
	} {
		writeProfileInspectionFixture(
			t,
			project.root,
			relative,
		)
	}
	suggestion, err := profiledetector.Inspect(project.root)
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.Classification() !=
		profiledetector.MixedSignals {
		t.Fatalf(
			"fixture classification = %q",
			suggestion.Classification(),
		)
	}
	surface, err := openSealedProjectOnboardSurface(
		context.Background(),
		mustOnboardProjectBinding(t, project.root),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	output := callOnboardHandler(
		t,
		surface.Handler(),
		`{"action":"profile_prepare"}`,
	)
	response := decodeOnboardResponse(t, output)
	if response.Result != "profile_review_prepared" ||
		response.Status != "profile_review_ready" ||
		len(response.Scopes) != 2 {
		t.Fatalf(
			"mixed prepare = %#v",
			response,
		)
	}
	kinds := []string{
		response.Scopes[0].RealizationKind,
		response.Scopes[1].RealizationKind,
	}
	if !reflect.DeepEqual(
		kinds,
		[]string{"non_software", "software"},
	) && !reflect.DeepEqual(
		kinds,
		[]string{"software", "non_software"},
	) {
		t.Fatalf(
			"mixed scope kinds = %#v",
			kinds,
		)
	}
	assertCLIProfileOnboardMutationCounts(
		t,
		project.root,
		0,
	)
	assertOnboardOutputHasNoInternalJargon(t, output)
}

func TestOnboardLegacyMemoryPreparationRemainsNonBindingButStatusRequiresInitRepair(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(
		t,
		"onboard-memory-review",
	)
	binding := mustOnboardProjectBinding(
		t,
		harness.Root().String(),
	)
	surface, err := openSealedProjectOnboardSurface(
		context.Background(),
		binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	handler := surface.Handler()

	statusOutput := callOnboardHandler(
		t,
		handler,
		`{"action":"status"}`,
	)
	status := decodeOnboardResponse(t, statusOutput)
	if status.Result != "onboarding_required" ||
		status.Status != "needs_init" ||
		!strings.Contains(status.NextAction, "haft init") {
		t.Fatalf(
			"preparation status = %#v",
			status,
		)
	}
	wantChoices := []string{
		"Enable structured project memory",
		"Not now",
	}
	if len(status.Choices) != 0 {
		t.Fatalf(
			"legacy repair status exposed choices = %#v",
			status.Choices,
		)
	}
	assertOnboardOutputHasNoInternalJargon(
		t,
		statusOutput,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_typeenv_heads",
		0,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		0,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_profile_admissions_v3",
		0,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_profile_admissions_v5",
		1,
	)

	preparedOutput := callOnboardHandler(
		t,
		handler,
		`{"action":"memory_prepare"}`,
	)
	prepared := decodeOnboardResponse(
		t,
		preparedOutput,
	)
	if prepared.Result !=
		"memory_review_prepared" ||
		prepared.Status !=
			"memory_review_ready" ||
		prepared.ReviewRef !=
			"review:onboard-memory" {
		t.Fatalf(
			"memory prepare = %#v",
			prepared,
		)
	}
	if !reflect.DeepEqual(
		prepared.Choices,
		wantChoices,
	) {
		t.Fatalf(
			"prepare choices = %#v",
			prepared.Choices,
		)
	}
	if !prepared.Effects.ReviewCarrierCreated ||
		prepared.Effects.CanonicalProfileChanged ||
		prepared.Effects.StructuredMemoryEnabled ||
		prepared.Effects.AuthorityGranted {
		t.Fatalf(
			"memory prepare effects = %#v",
			prepared.Effects,
		)
	}
	assertOnboardOutputHasNoInternalJargon(
		t,
		preparedOutput,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_typeenv_heads",
		0,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		0,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_profile_admissions_v3",
		0,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_profile_admissions_v5",
		1,
	)

	reusedOutput := callOnboardHandler(
		t,
		handler,
		`{"action":"memory_prepare"}`,
	)
	reused := decodeOnboardResponse(
		t,
		reusedOutput,
	)
	if reused.Result != "memory_review_reused" ||
		!reused.Effects.ReviewCarrierReused ||
		reused.Effects.ReviewCarrierCreated {
		t.Fatalf(
			"memory reuse = %#v",
			reused,
		)
	}
	assertOnboardTableCount(
		t,
		harness,
		"project_typeenv_heads",
		0,
	)
	assertOnboardTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		0,
	)

	readyOutput := callOnboardHandler(
		t,
		handler,
		`{"action":"status"}`,
	)
	ready := decodeOnboardResponse(
		t,
		readyOutput,
	)
	if ready.Result != "onboarding_required" ||
		ready.Status != "needs_init" ||
		len(ready.Choices) != 0 {
		t.Fatalf(
			"memory review status = %#v",
			ready,
		)
	}
	assertOnboardOutputHasNoInternalJargon(
		t,
		readyOutput,
	)
}

func mustOnboardProjectBinding(
	t *testing.T,
	root string,
) ProjectBinding {
	t.Helper()
	binding, err := resolveProjectBindingFromInput(
		projectRootInput{
			Path:   root,
			Source: "onboard-test",
		},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func callOnboardHandler(
	t *testing.T,
	handler func(
		context.Context,
		json.RawMessage,
	) (string, error),
	request string,
) string {
	t.Helper()
	if handler == nil {
		t.Fatal("onboarding handler is unavailable")
	}
	output, err := handler(
		context.Background(),
		json.RawMessage(request),
	)
	if err != nil {
		t.Fatalf(
			"onboarding request %s: %v",
			request,
			err,
		)
	}
	return output
}

func decodeOnboardResponse(
	t *testing.T,
	output string,
) onboardResponseWire {
	t.Helper()
	response := onboardResponseWire{}
	if err := json.Unmarshal(
		[]byte(output),
		&response,
	); err != nil {
		t.Fatalf(
			"decode onboarding response: %v\n%s",
			err,
			output,
		)
	}
	return response
}

func onboardPublishedInputSchema(
	t *testing.T,
) map[string]interface{} {
	t.Helper()
	for _, tool := range fpf.NewServer("test").ToolCatalog() {
		if tool.Name != "haft_onboard" {
			continue
		}
		schema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			t.Fatalf("haft_onboard input schema = %#v", tool.InputSchema)
		}
		return schema
	}
	t.Fatal("haft_onboard is absent from the published tool catalog")
	return nil
}

func onboardPublishedProperties(
	t *testing.T,
	schema map[string]interface{},
	label string,
) map[string]interface{} {
	t.Helper()
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s properties = %#v", label, schema["properties"])
	}
	return properties
}

func onboardPublishedInteger(
	t *testing.T,
	raw interface{},
	field string,
) int {
	t.Helper()
	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema = %#v", field, raw)
	}
	value, ok := schema[field].(int)
	if !ok {
		t.Fatalf("%s = %#v", field, schema[field])
	}
	return value
}

func assertOnboardOutputHasNoInternalJargon(
	t *testing.T,
	output string,
) {
	t.Helper()
	lower := strings.ToLower(output)
	for _, forbidden := range []string{
		"typeenv",
		"type_env",
		"projecttypeenv",
		"memorychangeset",
		"memory_change_set",
		"change_set",
		"graph_revision",
		"basis_digest",
		"basis_revision",
		"stage_ref",
		"b/e/x/c",
		"p8",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf(
				"public onboarding output leaked %q:\n%s",
				forbidden,
				output,
			)
		}
	}
}

func assertOnboardTableCount(
	t *testing.T,
	harness *profileadmissionfixture.Harness,
	table string,
	want int,
) {
	t.Helper()
	var got int
	query := "SELECT COUNT(*) FROM " + table
	if err := harness.Database().
		QueryRow(query).
		Scan(&got); err != nil {
		t.Fatalf(
			"count %s: %v",
			table,
			err,
		)
	}
	if got != want {
		t.Fatalf(
			"%s rows = %d, want %d",
			table,
			got,
			want,
		)
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestOnboardStatusRoutesSupportedSingletonLegacyProjectThroughInit(
	t *testing.T,
) {
	project := newCLIProfileOnboardLedgerFixture(t)
	writeProfileInspectionFixture(t, project.root, "go.mod")
	writeProfileInspectionFixture(t, project.root, "internal/kernel.go")
	binding := mustOnboardProjectBinding(t, project.root)
	surface, err := openSealedProjectOnboardSurface(
		context.Background(),
		binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	output := callOnboardHandler(
		t,
		surface.Handler(),
		`{"action":"status"}`,
	)
	response := decodeOnboardResponse(t, output)
	if response.Result != "needs_profile" ||
		response.Status != "needs_profile" ||
		!response.AutomaticBootstrapEligible ||
		!strings.Contains(response.NextAction, "haft init --core-only") {
		t.Fatalf("supported singleton onboarding status = %#v", response)
	}
	assertCLIProfileOnboardMutationCounts(t, project.root, 0)
}

func TestOnboardStatusBoundsLargeDetectorEvidenceWithoutLosingTotal(
	t *testing.T,
) {
	project := newCLIProfileOnboardLedgerFixture(t)
	writeProfileInspectionFixture(t, project.root, "go.mod")
	for index := 0; index < onboarding.MaximumEvidencePaths+19; index++ {
		writeProfileInspectionFixture(
			t,
			project.root,
			fmt.Sprintf("internal/component-%03d.go", index),
		)
	}
	surface, err := openSealedProjectOnboardSurface(
		context.Background(),
		mustOnboardProjectBinding(t, project.root),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	output := callOnboardHandler(
		t,
		surface.Handler(),
		`{"action":"status"}`,
	)
	response := decodeOnboardResponse(t, output)
	if response.Result != "needs_profile" ||
		len(response.Scopes) != 1 {
		t.Fatalf("large status = %#v", response)
	}
	scope := response.Scopes[0]
	wantTotal := onboarding.MaximumEvidencePaths + 20
	if scope.ScopeID != "software" ||
		scope.EvidencePathCount != wantTotal ||
		!scope.EvidencePathsTruncated ||
		len(scope.EvidencePaths) != onboarding.MaximumEvidencePaths {
		t.Fatalf("large evidence scope = %#v, want total %d", scope, wantTotal)
	}
}
