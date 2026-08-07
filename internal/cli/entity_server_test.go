package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type recordingEntityEstablishmentPort struct {
	calls   int
	request projectmemory.EntityEstablishmentRequest
}

func (port *recordingEntityEstablishmentPort) Establish(
	_ context.Context,
	request projectmemory.EntityEstablishmentRequest,
) (projectmemory.EntityEstablishmentResult, error) {
	port.calls++
	port.request = request
	return projectmemory.NewAlreadyExactEntityEstablished(request)
}

func TestEntityMCPHandlerStrictlyDecodesAndReturnsCanonicalReference(
	t *testing.T,
) {
	t.Parallel()

	port := &recordingEntityEstablishmentPort{}
	handler := newEntityMCPHandler(port.Establish)
	output, err := handler(context.Background(), json.RawMessage(
		entityServerValidRequest,
	))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if port.calls != 1 ||
		port.request.EntityID().String() != "service:auth" {
		t.Fatalf("port calls/request = %d/%#v", port.calls, port.request)
	}
	for _, expected := range []string{
		`"result":"established"`,
		`"delivery_kind":"already_exact"`,
		`"ref_kind_id":"U.EntityRef"`,
		`"reference_id":"service:auth"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output lacks %s: %s", expected, output)
		}
	}
	for _, forbidden := range []string{
		"type_env",
		"graph_revision",
		"change_set",
		"authority_class",
	} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("output leaked %q: %s", forbidden, output)
		}
	}
}

func TestEntityMCPHandlerRejectsUnknownFieldBeforeEffect(t *testing.T) {
	t.Parallel()

	port := &recordingEntityEstablishmentPort{}
	handler := newEntityMCPHandler(port.Establish)
	payload := strings.Replace(
		entityServerValidRequest,
		`"action":"establish",`,
		`"action":"establish","basis":{"kind":"project_current"},`,
		1,
	)
	if _, err := handler(
		context.Background(),
		json.RawMessage(payload),
	); err == nil {
		t.Fatal("low-level basis input was accepted")
	}
	if port.calls != 0 {
		t.Fatalf("invalid input reached effect %d time(s)", port.calls)
	}
}

func TestStartupAwareEntityPortRequiresRestartAfterEnablement(
	t *testing.T,
) {
	t.Parallel()

	inner := &recordingEntityEstablishmentPort{}
	ready := false
	port, err := newStartupAwareEntityEstablishmentPort(
		inner,
		func(context.Context) (bool, error) {
			return ready, nil
		},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	request, err := projectmemory.DecodeEntityEstablishmentRequest(
		[]byte(entityServerValidRequest),
	)
	if err != nil {
		t.Fatal(err)
	}
	first, err := port.Establish(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind() != projectmemory.EntityEstablishedResult ||
		inner.calls != 1 {
		t.Fatalf("pre-enablement delegation = %s/%d", first.Kind(), inner.calls)
	}

	ready = true
	second, err := port.Establish(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Kind() != projectmemory.EntityRestartRequiredResult ||
		inner.calls != 1 {
		t.Fatalf("post-enablement result/calls = %s/%d", second.Kind(), inner.calls)
	}
}

func TestOnboardingEntityHandlerIsAlwaysCallableAndNeverWrites(
	t *testing.T,
) {
	t.Parallel()

	handler, err := newEntityOnboardingRequiredMCPHandler(
		"Haft is not initialized in this project.",
	)
	if err != nil {
		t.Fatal(err)
	}
	output, err := handler(
		context.Background(),
		json.RawMessage(entityServerValidRequest),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"result":"onboarding_required"`) ||
		!strings.Contains(output, `"performed":false`) ||
		!strings.Contains(output, `"authority_granted":false`) {
		t.Fatalf("onboarding output = %s", output)
	}
}

func TestEntityMCPHandlerMapsEveryClosedDomainOutcome(t *testing.T) {
	t.Parallel()

	request, err := projectmemory.DecodeEntityEstablishmentRequest(
		[]byte(entityServerValidRequest),
	)
	if err != nil {
		t.Fatal(err)
	}
	other, err := typedmemory.NewEntityID("service:other")
	if err != nil {
		t.Fatal(err)
	}
	results := make([]projectmemory.EntityEstablishmentResult, 0, 9)
	results = append(
		results,
		mustEntityServerResult(
			projectmemory.NewEntityOnboardingRequired("Haft is not initialized."),
		),
		mustEntityServerResult(
			projectmemory.NewEntityEnablementChoiceRequired(
				"Typed project memory is not enabled.",
			),
		),
		mustEntityServerResult(
			projectmemory.NewEntityRestartRequired(
				"Project memory became ready after startup.",
			),
		),
		mustEntityServerResult(
			projectmemory.NewAlreadyExactEntityEstablished(request),
		),
		mustEntityServerResult(
			projectmemory.NewEntityIdentityConflict(
				request.EntityID(),
				"EntityID already names another exact identity.",
			),
		),
		mustEntityServerResult(
			projectmemory.NewEntityAliasConflict(
				request.Aliases()[0],
				other,
				"Alias is already bound.",
			),
		),
		mustEntityServerResult(
			projectmemory.NewEntityIdempotencyConflict(
				"Idempotency key is occupied.",
			),
		),
		mustEntityServerResult(
			projectmemory.NewEntityEstablishmentRejected(
				[]string{"Request is not semantically admissible."},
			),
		),
		mustEntityServerResult(
			projectmemory.NewEntityEstablishmentCommitOutcomeUnknown(
				request,
				"Durability confirmation is pending.",
			),
		),
	)
	for _, result := range results {
		result := result
		t.Run(string(result.Kind()), func(t *testing.T) {
			t.Parallel()
			port := fixedEntityEstablishmentPort{result: result}
			handler := newEntityMCPHandler(port.Establish)
			output, handlerErr := handler(
				context.Background(),
				json.RawMessage(entityServerValidRequest),
			)
			if handlerErr != nil {
				t.Fatalf("handler error = %v", handlerErr)
			}
			if !strings.Contains(
				output,
				`"result":"`+string(result.Kind())+`"`,
			) {
				t.Fatalf("closed result %s output = %s", result.Kind(), output)
			}
			lower := strings.ToLower(output)
			for _, forbidden := range []string{
				"typeenv",
				"memorychangeset",
			} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf(
						"closed result %s leaked %q: %s",
						result.Kind(),
						forbidden,
						output,
					)
				}
			}
		})
	}
}

func mustEntityServerResult[T projectmemory.EntityEstablishmentResult](
	result T,
	err error,
) projectmemory.EntityEstablishmentResult {
	if err != nil {
		panic(err)
	}
	return result
}

const entityServerValidRequest = `{
	"action":"establish",
	"entity_id":"service:auth",
	"label":"Authentication service",
	"bounded_context_ref":"context:software-system",
	"aliases":["auth"],
	"persistence_reason":"explicit_operator_request",
	"request_provenance_ref":"operator:chat",
	"idempotency_key":"entity:service:auth:v1"
}`
