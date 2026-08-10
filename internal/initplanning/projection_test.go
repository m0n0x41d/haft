package initplanning

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHostAdapterProjectionIsCanonicalImmutableAndEffectFree(t *testing.T) {
	root := canonicalTempRoot(t)
	left := buildProjectionInOrder(t, root, false)
	right := buildProjectionInOrder(t, root, true)
	leftOutputs := left.Outputs()
	rightOutputs := right.Outputs()
	if !reflect.DeepEqual(leftOutputs, rightOutputs) {
		t.Fatalf("projection output order is not canonical\nleft: %+v\nright: %+v", leftOutputs, rightOutputs)
	}
	if len(leftOutputs) != 2 || leftOutputs[0].Path() >= leftOutputs[1].Path() {
		t.Fatalf("projection paths are not sorted: %+v", leftOutputs)
	}
	before := left.Outputs()
	changed := left.Outputs()
	changed[0].content[0] = 'x'
	changed[0].path = filepath.Join(root, "changed")
	if !reflect.DeepEqual(left.Outputs(), before) {
		t.Fatal("projection output getter exposed carrier storage")
	}
	rootsBefore := left.TargetRoots()
	rootsChanged := left.TargetRoots()
	rootsChanged[0] = filepath.Join(root, "changed-root")
	if !reflect.DeepEqual(left.TargetRoots(), rootsBefore) {
		t.Fatal("projection root getter exposed carrier storage")
	}
}

func TestHostAdapterProjectionRejectsInvalidBindingAndOutputShape(t *testing.T) {
	root := canonicalTempRoot(t)
	validOutput := mustOutput(
		t,
		filepath.Join(root, "skills", "h-reason.md"),
		ComponentSkills,
		[]byte("skill"),
	)
	outsideOutput := mustOutput(
		t,
		filepath.Join(filepath.Dir(root), "outside.md"),
		ComponentSkills,
		[]byte("outside"),
	)
	wrongComponent := mustOutput(
		t,
		filepath.Join(root, "skills", "config.toml"),
		ComponentMCP,
		[]byte("config"),
	)
	for name, mutate := range map[string]func(HostAdapterProjectionBuilder) HostAdapterProjectionBuilder{
		"unknown host": func(builder HostAdapterProjectionBuilder) HostAdapterProjectionBuilder {
			builder.host = HostID("unknown")
			return builder
		},
		"invalid edition": func(builder HostAdapterProjectionBuilder) HostAdapterProjectionBuilder {
			return builder.AtEdition("bad edition")
		},
		"invalid project": func(builder HostAdapterProjectionBuilder) HostAdapterProjectionBuilder {
			return builder.ForProject(root, "not-a-project-id")
		},
		"outside output": func(builder HostAdapterProjectionBuilder) HostAdapterProjectionBuilder {
			return builder.AddOutput(outsideOutput)
		},
		"unselected component": func(builder HostAdapterProjectionBuilder) HostAdapterProjectionBuilder {
			return builder.AddOutput(wrongComponent)
		},
		"duplicate output": func(builder HostAdapterProjectionBuilder) HostAdapterProjectionBuilder {
			next := builder.AddOutput(validOutput)
			return next.AddOutput(validOutput)
		},
		"missing recovery": func(builder HostAdapterProjectionBuilder) HostAdapterProjectionBuilder {
			builder.recovery = RecoveryOperation{}
			return builder
		},
	} {
		t.Run(name, func(t *testing.T) {
			builder := baseProjectionBuilder(t, root)
			builder = mutate(builder)
			if _, err := builder.Build(); err == nil {
				t.Fatal("projection accepted an illegal state")
			}
		})
	}
}

func TestHostAdapterProjectionCanonicalizesManagedFragmentsWithoutOwningCarrier(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	carrierPath := filepath.Join(root, ".codex", "settings.json")
	first := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
		"semantic-merge-v1",
	)
	second := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"hooks", "haft"},
		`{"command":"haft","args":["hook"]}`,
		"semantic-merge-v1",
	)

	builder := baseManagedProjectionBuilder(t, root)
	builder = builder.AddManagedFragment(second)
	builder = builder.AddManagedFragment(first)
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterProjectionBuilder.Build: %v", err)
	}

	fragments := projection.ManagedFragments()
	if len(fragments) != 2 {
		t.Fatalf("managed fragments = %d, want 2", len(fragments))
	}
	leftKey := managedFragmentCoordinateKey(fragments[0].coordinate)
	rightKey := managedFragmentCoordinateKey(fragments[1].coordinate)
	if leftKey >= rightKey {
		t.Fatalf("managed fragments are not canonical: %q >= %q", leftKey, rightKey)
	}
	before := projection.ManagedFragments()
	changed := projection.ManagedFragments()
	changed[0].content[0] = 'x'
	changed[0].coordinate.carrierPath = filepath.Join(root, "changed.json")
	if !reflect.DeepEqual(projection.ManagedFragments(), before) {
		t.Fatal("managed fragment getter exposed projection storage")
	}
	if bytes.Equal(fragments[0].Content(), []byte(`{"command":"operator"}`)) {
		t.Fatal("projection unexpectedly treated shared carrier bytes as owned output")
	}
}

func TestHostAdapterProjectionRetainsOnlySelectedManagedFragmentComponents(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fragment := mustProjectionJSONObjectEntryFragment(
		t,
		filepath.Join(root, ".codex", "settings.json"),
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
		"semantic-merge-v1",
	)
	builder := baseManagedProjectionBuilder(t, root).
		AddManagedFragment(fragment).
		RetainInstalledManagedFragments(ComponentMCP)
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterProjectionBuilder.Build: %v", err)
	}
	want := []Component{ComponentMCP}
	if !reflect.DeepEqual(
		projection.RetainedManagedFragmentComponents(),
		want,
	) {
		t.Fatalf(
			"retained managed-fragment components = %v, want %v",
			projection.RetainedManagedFragmentComponents(),
			want,
		)
	}
	changed := projection.RetainedManagedFragmentComponents()
	changed[0] = ComponentSkills
	if !reflect.DeepEqual(
		projection.RetainedManagedFragmentComponents(),
		want,
	) {
		t.Fatal("retained component getter exposed projection storage")
	}
	if _, err := BuildProjectionInstallationManifest(projection); err == nil {
		t.Fatal("unresolved retained projection produced a manifest")
	}
	if _, err := BuildFirstCoherentManagedCarrierObservationPlans(
		projection,
		NoManagedFragmentLegacyRegistry(),
	); err == nil {
		t.Fatal("retained installed fragments accepted a first-install basis")
	}

	if _, err := baseManagedProjectionBuilder(t, root).
		AddManagedFragment(fragment).
		RetainInstalledManagedFragments(ComponentSkills).
		Build(); err == nil {
		t.Fatal("projection retained managed fragments for an unselected component")
	}
	if _, err := builder.
		RetainInstalledManagedFragments(ComponentMCP).
		Build(); err == nil {
		t.Fatal("projection repeated a retained managed-fragment component")
	}
}

func TestHostAdapterProjectionRejectsIllegalManagedFragmentShape(t *testing.T) {
	root := canonicalTempRoot(t)
	carrierPath := filepath.Join(root, ".codex", "settings.json")
	valid := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
		"semantic-merge-v1",
	)
	outside := mustProjectionJSONObjectEntryFragment(
		t,
		filepath.Join(filepath.Dir(root), "outside.json"),
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
		"semantic-merge-v1",
	)
	wrongComponent, err := NewJSONObjectEntryFragment(
		carrierPath,
		ComponentSkills,
		[]string{"mcpServers", "haft"},
		[]byte(`{"command":"haft","args":["serve"]}`),
		0o600,
		"semantic-merge-v1",
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment(wrong component): %v", err)
	}
	differentEdition := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"hooks", "haft"},
		`{"command":"haft","args":["hook"]}`,
		"semantic-merge-v2",
	)
	wholeCarrier := mustOutput(
		t,
		carrierPath,
		ComponentMCP,
		[]byte(`{"mcpServers":{}}`),
	)

	for name, mutate := range map[string]func(
		HostAdapterProjectionBuilder,
	) HostAdapterProjectionBuilder{
		"outside target root": func(
			builder HostAdapterProjectionBuilder,
		) HostAdapterProjectionBuilder {
			return builder.AddManagedFragment(outside)
		},
		"unselected component": func(
			builder HostAdapterProjectionBuilder,
		) HostAdapterProjectionBuilder {
			return builder.AddManagedFragment(wrongComponent)
		},
		"duplicate coordinate": func(
			builder HostAdapterProjectionBuilder,
		) HostAdapterProjectionBuilder {
			next := builder.AddManagedFragment(valid)
			return next.AddManagedFragment(valid)
		},
		"mixed merge editions in one carrier": func(
			builder HostAdapterProjectionBuilder,
		) HostAdapterProjectionBuilder {
			next := builder.AddManagedFragment(valid)
			return next.AddManagedFragment(differentEdition)
		},
		"whole file and fragment share carrier": func(
			builder HostAdapterProjectionBuilder,
		) HostAdapterProjectionBuilder {
			next := builder.AddOutput(wholeCarrier)
			return next.AddManagedFragment(valid)
		},
	} {
		t.Run(name, func(t *testing.T) {
			builder := baseManagedProjectionBuilder(t, root)
			builder = mutate(builder)
			if _, err := builder.Build(); err == nil {
				t.Fatal("projection accepted an illegal managed fragment state")
			}
		})
	}
}

func buildProjectionInOrder(
	t *testing.T,
	root string,
	reverse bool,
) HostAdapterProjection {
	t.Helper()
	first := mustOutput(
		t,
		filepath.Join(root, "skills", "a.md"),
		ComponentSkills,
		[]byte("a"),
	)
	second := mustOutput(
		t,
		filepath.Join(root, "skills", "b.md"),
		ComponentSkills,
		[]byte("b"),
	)
	builder := baseProjectionBuilder(t, root)
	if reverse {
		builder = builder.AddOutput(second)
		builder = builder.AddOutput(first)
	} else {
		builder = builder.AddOutput(first)
		builder = builder.AddOutput(second)
	}
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterProjectionBuilder.Build: %v", err)
	}
	return projection
}

func baseProjectionBuilder(
	t *testing.T,
	root string,
) HostAdapterProjectionBuilder {
	t.Helper()
	builder := NewHostAdapterProjectionBuilder(HostCodex)
	builder = builder.AtEdition("codex.v2")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(
		ScopeProject,
		mustComponents(t, ComponentSkills),
	)
	builder = builder.AddTargetRoot(root)
	builder = builder.RecoverWith(mustRecovery(t, HostCodex))
	return builder
}

func baseManagedProjectionBuilder(
	t *testing.T,
	root string,
) HostAdapterProjectionBuilder {
	t.Helper()
	builder := NewHostAdapterProjectionBuilder(HostCodex)
	builder = builder.AtEdition("codex.v3")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(
		ScopeProject,
		mustComponents(t, ComponentMCP),
	)
	builder = builder.AddTargetRoot(root)
	builder = builder.RecoverWith(mustRecovery(t, HostCodex))
	return builder
}

func mustProjectionJSONObjectEntryFragment(
	t *testing.T,
	carrierPath string,
	selector []string,
	value string,
	mergeEdition string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewJSONObjectEntryFragment(
		carrierPath,
		ComponentMCP,
		selector,
		[]byte(value),
		0o600,
		mergeEdition,
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment: %v", err)
	}
	return fragment
}
