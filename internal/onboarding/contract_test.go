package onboarding

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

type scriptedRuntime struct {
	observation       Observation
	profile           Preparation
	memory            Preparation
	observeCalls      int
	profileCalls      int
	memoryCalls       int
	observationUpdate func() Observation
}

func (runtime *scriptedRuntime) Observe(
	context.Context,
) (Observation, error) {
	runtime.observeCalls++
	if runtime.observationUpdate != nil {
		return runtime.observationUpdate(), nil
	}
	return runtime.observation, nil
}

func (runtime *scriptedRuntime) PrepareProfile(
	context.Context,
	Request,
) (Preparation, error) {
	runtime.profileCalls++
	return runtime.profile, nil
}

func (runtime *scriptedRuntime) PrepareMemory(
	context.Context,
) (Preparation, error) {
	runtime.memoryCalls++
	return runtime.memory, nil
}

func TestStatusProjectsClosedReadableStatesWithoutAuthorityEffects(
	t *testing.T,
) {
	t.Parallel()

	scope := mustScope(t, "app", "Application", Software, nil)
	fixtures := []struct {
		name        string
		observation ObservationInput
		result      Result
		status      Status
		choices     []string
	}{
		{
			name: "needs init",
			observation: ObservationInput{
				Initialized: false,
			},
			result: ResultOnboardingRequired,
			status: StatusNeedsInit,
		},
		{
			name: "needs profile",
			observation: ObservationInput{
				Initialized: true,
				Scopes:      []Scope{scope},
			},
			result: ResultNeedsProfile,
			status: StatusNeedsProfile,
		},
		{
			name: "profile review",
			observation: ObservationInput{
				Initialized:        true,
				ProfileReviewReady: true,
				Scopes:             []Scope{scope},
			},
			result: ResultProfileReviewReady,
			status: StatusProfileReviewReady,
		},
		{
			name: "needs memory",
			observation: ObservationInput{
				Initialized:     true,
				ProfileDeclared: true,
				Scopes:          []Scope{scope},
			},
			result:  ResultNeedsMemory,
			status:  StatusNeedsMemory,
			choices: memoryChoices(),
		},
		{
			name: "memory review",
			observation: ObservationInput{
				Initialized:       true,
				ProfileDeclared:   true,
				MemoryReviewReady: true,
				Scopes:            []Scope{scope},
			},
			result:  ResultMemoryReviewReady,
			status:  StatusMemoryReviewReady,
			choices: memoryChoices(),
		},
		{
			name: "deferred",
			observation: ObservationInput{
				Initialized:     true,
				ProfileDeclared: true,
				MemoryDeferred:  true,
				Scopes:          []Scope{scope},
			},
			result: ResultMemoryDeferred,
			status: StatusMemoryDeferred,
		},
		{
			name: "ready",
			observation: ObservationInput{
				Initialized:     true,
				ProfileDeclared: true,
				MemoryReady:     true,
				Scopes:          []Scope{scope},
			},
			result: ResultReady,
			status: StatusReady,
		},
	}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			observation, err := NewObservation(
				fixture.observation,
			)
			if err != nil {
				t.Fatal(err)
			}
			runtime := &scriptedRuntime{
				observation: observation,
			}
			service, err := NewService(
				runtime,
				fixture.observation.MemoryReady,
			)
			if err != nil {
				t.Fatal(err)
			}
			request, err := NewRequest(
				RequestInput{
					Action: string(ActionStatus),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := service.Execute(
				context.Background(),
				request,
			)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Result() != fixture.result ||
				outcome.Status() != fixture.status {
				t.Fatalf(
					"result/status = %q/%q, want %q/%q",
					outcome.Result(),
					outcome.Status(),
					fixture.result,
					fixture.status,
				)
			}
			if !slices.Equal(
				outcome.Choices(),
				fixture.choices,
			) {
				t.Fatalf(
					"choices = %#v, want %#v",
					outcome.Choices(),
					fixture.choices,
				)
			}
			effects := outcome.Effects()
			if effects.CanonicalProfileChanged ||
				effects.StructuredMemoryEnabled ||
				effects.AuthorityGranted ||
				effects.ReviewCarrierCreated ||
				effects.ReviewCarrierReused {
				t.Fatalf(
					"status crossed effect boundary: %#v",
					effects,
				)
			}
			if runtime.profileCalls != 0 ||
				runtime.memoryCalls != 0 {
				t.Fatalf(
					"status invoked preparation: %d/%d",
					runtime.profileCalls,
					runtime.memoryCalls,
				)
			}
		})
	}
}

func TestServiceRequiresRestartWhenMemoryBecameReadyAfterStartup(
	t *testing.T,
) {
	t.Parallel()

	observation, err := NewObservation(
		ObservationInput{
			Initialized:     true,
			ProfileDeclared: true,
			MemoryReady:     true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &scriptedRuntime{
		observation: observation,
	}
	service, err := NewService(runtime, false)
	if err != nil {
		t.Fatal(err)
	}
	request, err := NewRequest(
		RequestInput{
			Action: string(ActionMemoryPrepare),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := service.Execute(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result() != ResultRestartRequired ||
		outcome.Status() != StatusReady {
		t.Fatalf(
			"result/status = %q/%q",
			outcome.Result(),
			outcome.Status(),
		)
	}
	if runtime.memoryCalls != 0 {
		t.Fatalf(
			"stale process invoked preparation %d time(s)",
			runtime.memoryCalls,
		)
	}
	effects := outcome.Effects()
	if effects.StructuredMemoryEnabled ||
		effects.AuthorityGranted ||
		effects.ReviewCarrierCreated ||
		effects.ReviewCarrierReused {
		t.Fatalf(
			"restart result crossed effect boundary: %#v",
			effects,
		)
	}
}

func TestPreparationResultsKeepBindingEffectsFalse(t *testing.T) {
	t.Parallel()

	scope := mustScope(
		t,
		"docs",
		"Documentation",
		NonSoftware,
		[]string{"README.md"},
	)
	observation, err := NewObservation(
		ObservationInput{
			Initialized: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := NewPreparation(
		PreparationCreated,
		"review:onboard-profile",
		[]Scope{scope},
		"Prepared.",
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &scriptedRuntime{
		observation: observation,
		profile:     profile,
	}
	service, err := NewService(runtime, false)
	if err != nil {
		t.Fatal(err)
	}
	request, err := NewRequest(
		RequestInput{
			Action: string(ActionProfilePrepare),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := service.Execute(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result() != ResultProfileReviewCreated ||
		outcome.Status() != StatusProfileReviewReady {
		t.Fatalf(
			"result/status = %q/%q",
			outcome.Result(),
			outcome.Status(),
		)
	}
	effects := outcome.Effects()
	if !effects.ReviewCarrierCreated ||
		effects.CanonicalProfileChanged ||
		effects.StructuredMemoryEnabled ||
		effects.AuthorityGranted {
		t.Fatalf(
			"preparation effects = %#v",
			effects,
		)
	}
}

func TestRequestRejectsCrossActionAndIncompleteFallbackFields(
	t *testing.T,
) {
	t.Parallel()

	scope := mustScope(
		t,
		"app",
		"Application",
		Software,
		nil,
	)
	fixtures := []RequestInput{
		{
			Action:       string(ActionStatus),
			BasisPresent: true,
			Basis:        "not applicable",
		},
		{
			Action:        string(ActionMemoryPrepare),
			ScopesPresent: true,
			Scopes:        []Scope{scope},
		},
		{
			Action:        string(ActionProfilePrepare),
			ScopesPresent: true,
			Scopes:        []Scope{scope},
		},
		{
			Action:       string(ActionProfilePrepare),
			BasisPresent: true,
			Basis:        "Manual classification.",
		},
	}
	for index, fixture := range fixtures {
		if _, err := NewRequest(fixture); err == nil {
			t.Fatalf(
				"invalid request %d was accepted: %#v",
				index,
				fixture,
			)
		}
	}
}

func TestScopeEvidencePathCountMatchesPublicSchemaBound(t *testing.T) {
	t.Parallel()

	atLimit := make([]string, MaximumEvidencePaths)
	for index := range atLimit {
		atLimit[index] = fmt.Sprintf("docs/evidence-%03d.md", index)
	}
	if _, err := NewScope(
		"docs",
		"Documentation",
		NonSoftware,
		atLimit,
	); err != nil {
		t.Fatalf("scope at evidence-path limit: %v", err)
	}

	overLimit := append(
		append([]string{}, atLimit...),
		"docs/evidence-over-limit.md",
	)
	if _, err := NewScope(
		"docs",
		"Documentation",
		NonSoftware,
		overLimit,
	); err == nil {
		t.Fatal("scope above evidence-path limit was accepted")
	}
}

func mustScope(
	t *testing.T,
	scopeID string,
	label string,
	kind RealizationKind,
	evidence []string,
) Scope {
	t.Helper()
	scope, err := NewScope(
		scopeID,
		label,
		kind,
		evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}
