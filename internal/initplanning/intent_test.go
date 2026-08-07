package initplanning

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseInitIntentRequiresExplicitPolicyAndCanonicalSelectedHostSet(t *testing.T) {
	root := canonicalTempRoot(t)
	input := WeakInitIntent{
		InvocationPolicy: string(InvocationExplicit),
		ProjectRoot:      root,
		ProjectID:        "qnt_e3149c17",
		Hosts: []WeakHostSelection{
			{
				Host:       string(HostZed),
				Scope:      string(ScopeUser),
				Components: []string{string(ComponentMCP)},
			},
			{
				Host:       string(HostCodex),
				Scope:      string(ScopeProject),
				Components: []string{string(ComponentSkills), string(ComponentMCP)},
			},
		},
	}
	intent, err := ParseInitIntent(input)
	if err != nil {
		t.Fatalf("ParseInitIntent: %v", err)
	}
	hosts := intent.SelectedHosts().Values()
	if len(hosts) != 2 || hosts[0].Host() != HostCodex || hosts[1].Host() != HostZed {
		t.Fatalf("selected hosts = %+v, want canonical codex/zed order", hosts)
	}
	wantComponents := []Component{ComponentMCP, ComponentSkills}
	if !reflect.DeepEqual(hosts[0].Components().Values(), wantComponents) {
		t.Fatalf(
			"codex components = %v, want %v",
			hosts[0].Components().Values(),
			wantComponents,
		)
	}
	input.Hosts[1].Components[0] = string(ComponentHooks)
	if !reflect.DeepEqual(hosts[0].Components().Values(), wantComponents) {
		t.Fatal("parsed intent retained caller-owned component storage")
	}

	input.InvocationPolicy = ""
	if _, err := ParseInitIntent(input); err == nil {
		t.Fatal("ParseInitIntent accepted an omitted automation policy")
	}
}

func TestParseInitIntentRepresentsCoreOnlyWithoutAHostBooleanProduct(t *testing.T) {
	intent, err := ParseInitIntent(WeakInitIntent{
		InvocationPolicy: string(InvocationInteractive),
		ProjectRoot:      canonicalTempRoot(t),
		ProjectID:        "qnt_e3149c17",
	})
	if err != nil {
		t.Fatalf("ParseInitIntent core-only: %v", err)
	}
	if len(intent.SelectedHosts().Values()) != 0 {
		t.Fatalf("core-only intent selected hosts: %v", intent.SelectedHosts().Values())
	}
}

func TestParseInitIntentRejectsInvalidHostScopeComponentAndDuplicates(t *testing.T) {
	root := canonicalTempRoot(t)
	cases := []struct {
		name  string
		hosts []WeakHostSelection
	}{
		{
			name: "unknown host",
			hosts: []WeakHostSelection{{
				Host:       "agent",
				Scope:      string(ScopeProject),
				Components: []string{string(ComponentMCP)},
			}},
		},
		{
			name: "unknown scope",
			hosts: []WeakHostSelection{{
				Host:       string(HostCodex),
				Scope:      "globalish",
				Components: []string{string(ComponentMCP)},
			}},
		},
		{
			name: "empty component set",
			hosts: []WeakHostSelection{{
				Host:  string(HostCodex),
				Scope: string(ScopeProject),
			}},
		},
		{
			name: "repeated component",
			hosts: []WeakHostSelection{{
				Host:       string(HostCodex),
				Scope:      string(ScopeProject),
				Components: []string{string(ComponentMCP), string(ComponentMCP)},
			}},
		},
		{
			name: "repeated host scope binding",
			hosts: []WeakHostSelection{
				{
					Host:       string(HostCodex),
					Scope:      string(ScopeProject),
					Components: []string{string(ComponentMCP)},
				},
				{
					Host:       string(HostCodex),
					Scope:      string(ScopeProject),
					Components: []string{string(ComponentSkills)},
				},
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := ParseInitIntent(WeakInitIntent{
				InvocationPolicy: string(InvocationExplicit),
				ProjectRoot:      root,
				ProjectID:        "qnt_e3149c17",
				Hosts:            testCase.hosts,
			})
			if err == nil {
				t.Fatalf("ParseInitIntent accepted %s", testCase.name)
			}
		})
	}
}

func TestParseInitIntentAllowsOneHostAtIndependentProjectAndUserScopes(
	t *testing.T,
) {
	intent, err := ParseInitIntent(WeakInitIntent{
		InvocationPolicy: string(InvocationExplicit),
		ProjectRoot:      canonicalTempRoot(t),
		ProjectID:        "qnt_e3149c17",
		Hosts: []WeakHostSelection{
			{
				Host:       string(HostClaude),
				Scope:      string(ScopeProject),
				Components: []string{string(ComponentMCP)},
			},
			{
				Host:       string(HostClaude),
				Scope:      string(ScopeUser),
				Components: []string{string(ComponentSkills)},
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseInitIntent: %v", err)
	}
	bindings := intent.SelectedHosts().Values()
	if len(bindings) != 2 {
		t.Fatalf("binding count = %d, want 2", len(bindings))
	}
	if bindings[0].BindingID().String() != "claude/project" ||
		bindings[1].BindingID().String() != "claude/user" {
		t.Fatalf("bindings = %#v", bindings)
	}
}

func TestDiscoveryObservationCannotBecomeSelectionWithoutIntent(t *testing.T) {
	observation, err := NewDiscoveryObservation(
		HostCodex,
		"binary_found_on_path",
		DiscoveryDetected,
	)
	if err != nil {
		t.Fatalf("NewDiscoveryObservation: %v", err)
	}
	if observation.Host() != HostCodex || observation.Posture() != DiscoveryDetected {
		t.Fatalf("discovery observation = %+v", observation)
	}
	intent, err := ParseInitIntent(WeakInitIntent{
		InvocationPolicy: string(InvocationExplicit),
		ProjectRoot:      canonicalTempRoot(t),
		ProjectID:        "qnt_e3149c17",
	})
	if err != nil {
		t.Fatalf("ParseInitIntent: %v", err)
	}
	if len(intent.SelectedHosts().Values()) != 0 {
		t.Fatal("discovery observation silently entered SelectedHostSet")
	}
}

func canonicalTempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if strings.Contains(root, "..") {
		t.Fatalf("temporary root is not canonical: %s", root)
	}
	return root
}
