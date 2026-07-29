package initplanning

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCompileInitPlanSeparatesCoreAndHostEffectsAndProjectsExactPreview(t *testing.T) {
	root := canonicalTempRoot(t)
	components := mustComponents(t, ComponentMCP, ComponentSkills)
	intent := mustIntent(t, root, []WeakHostSelection{{
		Host:       string(HostCodex),
		Scope:      string(ScopeProject),
		Components: []string{string(ComponentMCP), string(ComponentSkills)},
	}})
	core := mustCorePlan(t, root, CoreMigrate, 53, 54)
	catalog := mustCatalog(t, HostCodex, "codex.v1", ScopeProject, components)
	adapter := mustRichCodexAdapterPlan(t, root, components)

	plan, err := CompileInitPlan(
		intent,
		core,
		[]HostAdapterInstallPlan{adapter},
		catalog,
	)
	if err != nil {
		t.Fatalf("CompileInitPlan: %v", err)
	}
	if plan.Readiness() != PlanBlocked {
		t.Fatalf("plan readiness = %s, want blocked by explicit foreign collision", plan.Readiness())
	}
	preview := plan.Preview()
	if preview.Core.Effect != CoreMigrate ||
		preview.Core.BeforeSchema != 53 ||
		preview.Core.AfterSchema != 54 {
		t.Fatalf("core preview = %+v", preview.Core)
	}
	if !reflect.DeepEqual(
		preview.ApplyOrder,
		[]string{"core_project", "host_adapter_fanout"},
	) {
		t.Fatalf("apply order = %v", preview.ApplyOrder)
	}
	if preview.PartialEffectBoundary != "core_applied_host_incomplete" {
		t.Fatalf("partial-effect boundary = %q", preview.PartialEffectBoundary)
	}
	if len(preview.Hosts) != 1 {
		t.Fatalf("host preview count = %d", len(preview.Hosts))
	}
	effectKinds := make([]FileEffectKind, len(preview.Hosts[0].Effects))
	for index, effect := range preview.Hosts[0].Effects {
		effectKinds[index] = effect.Effect
	}
	sort.Slice(effectKinds, func(left int, right int) bool {
		return effectKinds[left] < effectKinds[right]
	})
	wantKinds := []FileEffectKind{
		FileAdoptLegacy,
		FileConflict,
		FileCreate,
		FilePreserve,
		FileRemoveOwnedOrphan,
		FileUpdate,
		FileUpdateLegacy,
	}
	sort.Slice(wantKinds, func(left int, right int) bool {
		return wantKinds[left] < wantKinds[right]
	})
	if !reflect.DeepEqual(effectKinds, wantKinds) {
		t.Fatalf("preview effects = %v, want %v", effectKinds, wantKinds)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("pure planner created host files: %v", err)
	}
}

func TestCompileInitPlanAllowsExplicitCoreOnlyAndRejectsExtraAdapter(t *testing.T) {
	root := canonicalTempRoot(t)
	intent := mustIntent(t, root, nil)
	core := mustCorePlan(t, root, CoreVerifyCurrent, 54, 54)
	components := mustComponents(t, ComponentMCP)
	catalog := mustCatalog(t, HostCodex, "codex.v1", ScopeProject, components)

	plan, err := CompileInitPlan(intent, core, nil, catalog)
	if err != nil {
		t.Fatalf("CompileInitPlan core-only: %v", err)
	}
	if len(plan.Hosts()) != 0 || plan.Readiness() != PlanReady {
		t.Fatalf("core-only plan = hosts:%d readiness:%s", len(plan.Hosts()), plan.Readiness())
	}
	adapter := mustSimpleAdapterPlan(t, root, HostCodex, "codex.v1", ScopeProject, components)
	_, err = CompileInitPlan(
		intent,
		core,
		[]HostAdapterInstallPlan{adapter},
		catalog,
	)
	if err == nil || !strings.Contains(err.Error(), "set differ") {
		t.Fatalf("discovery-like extra adapter was accepted: %v", err)
	}
}

func TestCompileInitPlanKeysAdaptersByHostAndScope(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	mcp := mustComponents(t, ComponentMCP)
	skills := mustComponents(t, ComponentSkills)
	intent := mustIntent(t, root, []WeakHostSelection{
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
	})
	projectCapability, err := NewAdapterCapabilityBuilder(HostClaude).
		AtEdition("claude.project.v1").
		Allow(ScopeProject, mcp).
		Build()
	if err != nil {
		t.Fatalf("build project capability: %v", err)
	}
	userCapability, err := NewAdapterCapabilityBuilder(HostClaude).
		AtEdition("claude.user.v1").
		Allow(ScopeUser, skills).
		Build()
	if err != nil {
		t.Fatalf("build user capability: %v", err)
	}
	catalog, err := NewAdapterCatalog(
		[]AdapterCapability{
			userCapability,
			projectCapability,
		},
	)
	if err != nil {
		t.Fatalf("NewAdapterCatalog: %v", err)
	}
	projectPlan := mustSimpleAdapterPlan(
		t,
		root,
		HostClaude,
		"claude.project.v1",
		ScopeProject,
		mcp,
	)
	userPath := filepath.Join(
		root,
		".host",
		"claude.user.config",
	)
	userPlan := mustAdapterAtPath(
		t,
		root,
		HostClaude,
		"claude.user.v1",
		ScopeUser,
		skills,
		userPath,
	)
	core := mustCorePlan(t, root, CoreVerifyCurrent, 54, 54)

	plan, err := CompileInitPlan(
		intent,
		core,
		[]HostAdapterInstallPlan{
			userPlan,
			projectPlan,
		},
		catalog,
	)
	if err != nil {
		t.Fatalf("CompileInitPlan: %v", err)
	}
	hosts := plan.Hosts()
	if len(hosts) != 2 ||
		hosts[0].BindingID().String() != "claude/project" ||
		hosts[1].BindingID().String() != "claude/user" {
		t.Fatalf("compiled bindings = %#v", hosts)
	}
}

func TestCompileInitPlanSharesOnlyUserScopedSkillOwnershipAcrossProjects(
	t *testing.T,
) {
	projectRoot := canonicalTempRoot(t)
	ownerRoot := canonicalTempRoot(t)
	targetRoot := canonicalTempRoot(t)
	core := mustCorePlan(
		t,
		projectRoot,
		CoreVerifyCurrent,
		54,
		54,
	)
	buildPlan := func(
		scope InstallScope,
		components ComponentSet,
	) HostAdapterInstallPlan {
		component := components.Values()[0]
		path := filepath.Join(
			targetRoot,
			string(scope)+"-"+string(component),
		)
		output := mustOutput(
			t,
			path,
			component,
			[]byte("shared host projection\n"),
		)
		expectation, err := ExpectMissing(path)
		if err != nil {
			t.Fatalf("ExpectMissing: %v", err)
		}
		builder := NewHostAdapterInstallPlanBuilder(HostCodex)
		builder = builder.AtEdition("codex.v1")
		builder = builder.PublishedFrom(
			mustPublicationIdentity(t, projectRoot),
		)
		builder = builder.ForProject(ownerRoot, "qnt_34f7b96f")
		builder = builder.WithSelection(scope, components)
		builder = builder.AddTargetRoot(targetRoot)
		builder = builder.AddOutput(expectation, output)
		builder = builder.RecoverWith(mustRecovery(t, HostCodex))
		plan, err := builder.Build()
		if err != nil {
			t.Fatalf("build cross-project adapter plan: %v", err)
		}
		return plan
	}

	skills := mustComponents(t, ComponentSkills)
	skillIntent := mustIntent(t, projectRoot, []WeakHostSelection{{
		Host:       string(HostCodex),
		Scope:      string(ScopeUser),
		Components: []string{string(ComponentSkills)},
	}})
	skillCatalog := mustCatalog(
		t,
		HostCodex,
		"codex.v1",
		ScopeUser,
		skills,
	)
	if _, err := CompileInitPlan(
		skillIntent,
		core,
		[]HostAdapterInstallPlan{
			buildPlan(ScopeUser, skills),
		},
		skillCatalog,
	); err != nil {
		t.Fatalf(
			"shared user skill owner was rejected: %v",
			err,
		)
	}

	mcp := mustComponents(t, ComponentMCP)
	mcpIntent := mustIntent(t, projectRoot, []WeakHostSelection{{
		Host:       string(HostCodex),
		Scope:      string(ScopeUser),
		Components: []string{string(ComponentMCP)},
	}})
	mcpCatalog := mustCatalog(
		t,
		HostCodex,
		"codex.v1",
		ScopeUser,
		mcp,
	)
	if _, err := CompileInitPlan(
		mcpIntent,
		core,
		[]HostAdapterInstallPlan{
			buildPlan(ScopeUser, mcp),
		},
		mcpCatalog,
	); err == nil || !strings.Contains(
		err.Error(),
		"belongs to another project",
	) {
		t.Fatalf(
			"cross-project user MCP owner was accepted: %v",
			err,
		)
	}

	projectSkillIntent := mustIntent(
		t,
		projectRoot,
		[]WeakHostSelection{{
			Host:       string(HostCodex),
			Scope:      string(ScopeProject),
			Components: []string{string(ComponentSkills)},
		}},
	)
	projectSkillCatalog := mustCatalog(
		t,
		HostCodex,
		"codex.v1",
		ScopeProject,
		skills,
	)
	if _, err := CompileInitPlan(
		projectSkillIntent,
		core,
		[]HostAdapterInstallPlan{
			buildPlan(ScopeProject, skills),
		},
		projectSkillCatalog,
	); err == nil || !strings.Contains(
		err.Error(),
		"belongs to another project",
	) {
		t.Fatalf(
			"cross-project project skill owner was accepted: %v",
			err,
		)
	}

	for _, test := range []struct {
		name       string
		components ComponentSet
	}{
		{
			name: "skills_and_mcp",
			components: mustComponents(
				t,
				ComponentSkills,
				ComponentMCP,
			),
		},
		{
			name: "skills_and_instructions",
			components: mustComponents(
				t,
				ComponentSkills,
				ComponentInstructions,
			),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := test.components.Values()
			raw := make([]string, len(values))
			for index, component := range values {
				raw[index] = string(component)
			}
			intent := mustIntent(
				t,
				projectRoot,
				[]WeakHostSelection{{
					Host:       string(HostCodex),
					Scope:      string(ScopeUser),
					Components: raw,
				}},
			)
			catalog := mustCatalog(
				t,
				HostCodex,
				"codex.v1",
				ScopeUser,
				test.components,
			)
			if _, err := CompileInitPlan(
				intent,
				core,
				[]HostAdapterInstallPlan{
					buildPlan(ScopeUser, test.components),
				},
				catalog,
			); err == nil || !strings.Contains(
				err.Error(),
				"belongs to another project",
			) {
				t.Fatalf(
					"cross-project mixed user owner was accepted: %v",
					err,
				)
			}
		})
	}

	wrongHostIntent := mustIntent(
		t,
		projectRoot,
		[]WeakHostSelection{{
			Host:       string(HostClaude),
			Scope:      string(ScopeUser),
			Components: []string{string(ComponentSkills)},
		}},
	)
	wrongHostCatalog := mustCatalog(
		t,
		HostClaude,
		"claude.v1",
		ScopeUser,
		skills,
	)
	if _, err := CompileInitPlan(
		wrongHostIntent,
		core,
		[]HostAdapterInstallPlan{
			buildPlan(ScopeUser, skills),
		},
		wrongHostCatalog,
	); err == nil || !strings.Contains(
		err.Error(),
		"has no adapter plan",
	) {
		t.Fatalf(
			"cross-project plan from another host was accepted: %v",
			err,
		)
	}

	if _, err := ParseComponentSet([]string{
		string(ComponentSkills),
		"unknown-component",
	}); err == nil {
		t.Fatal("malformed shared component set was accepted")
	}
}

func TestCompileInitPlanRejectsUnsupportedScopeComponentAndEdition(t *testing.T) {
	root := canonicalTempRoot(t)
	mcp := mustComponents(t, ComponentMCP)
	catalog := mustCatalog(t, HostZed, "zed.v1", ScopeUser, mcp)
	core := mustCorePlan(t, root, CoreVerifyCurrent, 54, 54)

	projectIntent := mustIntent(t, root, []WeakHostSelection{{
		Host:       string(HostZed),
		Scope:      string(ScopeProject),
		Components: []string{string(ComponentMCP)},
	}})
	projectAdapter := mustSimpleAdapterPlan(t, root, HostZed, "zed.v1", ScopeProject, mcp)
	_, err := CompileInitPlan(
		projectIntent,
		core,
		[]HostAdapterInstallPlan{projectAdapter},
		catalog,
	)
	if err == nil || !strings.Contains(err.Error(), "does not support project scope") {
		t.Fatalf("unsupported Zed project scope was accepted: %v", err)
	}

	packageComponent := mustComponents(t, ComponentPackage)
	packageIntent := mustIntent(t, root, []WeakHostSelection{{
		Host:       string(HostZed),
		Scope:      string(ScopeUser),
		Components: []string{string(ComponentPackage)},
	}})
	packageAdapter := mustSimpleAdapterPlan(
		t,
		root,
		HostZed,
		"zed.v1",
		ScopeUser,
		packageComponent,
	)
	_, err = CompileInitPlan(
		packageIntent,
		core,
		[]HostAdapterInstallPlan{packageAdapter},
		catalog,
	)
	if err == nil || !strings.Contains(err.Error(), "does not support component package") {
		t.Fatalf("unsupported Zed package component was accepted: %v", err)
	}

	userIntent := mustIntent(t, root, []WeakHostSelection{{
		Host:       string(HostZed),
		Scope:      string(ScopeUser),
		Components: []string{string(ComponentMCP)},
	}})
	wrongEdition := mustSimpleAdapterPlan(t, root, HostZed, "zed.v2", ScopeUser, mcp)
	_, err = CompileInitPlan(
		userIntent,
		core,
		[]HostAdapterInstallPlan{wrongEdition},
		catalog,
	)
	if err == nil || !strings.Contains(err.Error(), "differs from catalog edition") {
		t.Fatalf("unregistered adapter edition was accepted: %v", err)
	}
}

func TestCompileInitPlanRejectsCrossHostPathOwnership(t *testing.T) {
	root := canonicalTempRoot(t)
	mcp := mustComponents(t, ComponentMCP)
	intent := mustIntent(t, root, []WeakHostSelection{
		{
			Host:       string(HostCodex),
			Scope:      string(ScopeProject),
			Components: []string{string(ComponentMCP)},
		},
		{
			Host:       string(HostClaude),
			Scope:      string(ScopeProject),
			Components: []string{string(ComponentMCP)},
		},
	})
	core := mustCorePlan(t, root, CoreVerifyCurrent, 54, 54)
	codexCapability := mustCapability(t, HostCodex, "codex.v1", ScopeProject, mcp)
	claudeCapability := mustCapability(t, HostClaude, "claude.v1", ScopeProject, mcp)
	catalog, err := NewAdapterCatalog([]AdapterCapability{codexCapability, claudeCapability})
	if err != nil {
		t.Fatalf("NewAdapterCatalog: %v", err)
	}
	shared := filepath.Join(root, ".mcp.json")
	codex := mustAdapterAtPath(t, root, HostCodex, "codex.v1", ScopeProject, mcp, shared)
	claude := mustAdapterAtPath(t, root, HostClaude, "claude.v1", ScopeProject, mcp, shared)

	_, err = CompileInitPlan(
		intent,
		core,
		[]HostAdapterInstallPlan{claude, codex},
		catalog,
	)
	if err == nil || !strings.Contains(err.Error(), "both plan path") {
		t.Fatalf("cross-host path ownership was accepted: %v", err)
	}
}

func TestCorePlanMakesInvalidSchemaAndBasisStatesInexpressible(t *testing.T) {
	root := canonicalTempRoot(t)
	unavailable, err := NewUnavailableBasis("genesis_required")
	if err != nil {
		t.Fatalf("NewUnavailableBasis: %v", err)
	}
	builder := NewCoreProjectPlanBuilder()
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.AtDatabase(filepath.Join(root, "haft.db"))
	builder = builder.WithBasis(unavailable)
	cases := []struct {
		effect CoreEffectKind
		before int
		after  int
	}{
		{effect: CoreInitialize, before: 1, after: 54},
		{effect: CoreMigrate, before: 54, after: 53},
		{effect: CoreVerifyCurrent, before: 53, after: 54},
	}
	for _, testCase := range cases {
		candidate := builder.WithSchemaTransition(
			testCase.effect,
			testCase.before,
			testCase.after,
		)
		if _, err := candidate.Build(); err == nil {
			t.Fatalf(
				"core plan accepted %s %d -> %d",
				testCase.effect,
				testCase.before,
				testCase.after,
			)
		}
	}
	if _, err := NewSelectedBasis("", 0, ""); err == nil {
		t.Fatal("selected basis accepted missing coordinates")
	}
}

func TestOwnershipClassificationRequiresItsExactReceiptKind(t *testing.T) {
	path := filepath.Join(canonicalTempRoot(t), "skill.md")
	digest := digestBytes([]byte("skill\n"))
	manifestRef := "host-installation-manifest:fixture"
	manifest, err := NewOwnershipBasis(
		OwnershipManifestReceipt,
		manifestRef,
		digestBytes([]byte(manifestRef)),
	)
	if err != nil {
		t.Fatalf("NewOwnershipBasis manifest: %v", err)
	}
	legacyRef := "known-legacy-registry:fixture"
	legacy, err := NewOwnershipBasis(
		OwnershipLegacyRegistry,
		legacyRef,
		digestBytes([]byte(legacyRef)),
	)
	if err != nil {
		t.Fatalf("NewOwnershipBasis legacy: %v", err)
	}
	if _, err := ExpectCurrentOwned(path, digest, 0o644, legacy); err == nil {
		t.Fatal("current-owned predecessor accepted a legacy registry as ownership")
	}
	if _, err := ExpectKnownLegacyExact(path, digest, 0o644, manifest); err == nil {
		t.Fatal("known-legacy predecessor accepted an installation receipt")
	}
	if _, err := NewLocallyModifiedOwnedConflict(
		path,
		"locally changed",
		OwnershipBasis{},
	); err == nil {
		t.Fatal("locally modified owned conflict accepted no installation receipt")
	}
}

func mustIntent(
	t *testing.T,
	root string,
	hosts []WeakHostSelection,
) InitIntent {
	t.Helper()
	intent, err := ParseInitIntent(WeakInitIntent{
		InvocationPolicy: string(InvocationExplicit),
		ProjectRoot:      root,
		ProjectID:        "qnt_e3149c17",
		Hosts:            hosts,
	})
	if err != nil {
		t.Fatalf("ParseInitIntent: %v", err)
	}
	return intent
}

func mustCorePlan(
	t *testing.T,
	root string,
	effect CoreEffectKind,
	before int,
	after int,
) CoreProjectPlan {
	t.Helper()
	basis, err := NewSelectedBasis(
		"project-typeenv-head:qnt_e3149c17",
		1,
		"typeenv:sha256:"+strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("NewSelectedBasis: %v", err)
	}
	builder := NewCoreProjectPlanBuilder()
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.AtDatabase(filepath.Join(root, ".haft-home", "haft.db"))
	builder = builder.WithSchemaTransition(effect, before, after)
	builder = builder.WithBasis(basis)
	plan, err := builder.Build()
	if err != nil {
		t.Fatalf("CoreProjectPlanBuilder.Build: %v", err)
	}
	return plan
}

func mustComponents(t *testing.T, components ...Component) ComponentSet {
	t.Helper()
	raw := make([]string, len(components))
	for index, component := range components {
		raw[index] = string(component)
	}
	set, err := ParseComponentSet(raw)
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	return set
}

func mustCapability(
	t *testing.T,
	host HostID,
	edition string,
	scope InstallScope,
	components ComponentSet,
) AdapterCapability {
	t.Helper()
	builder := NewAdapterCapabilityBuilder(host)
	builder = builder.AtEdition(edition)
	builder = builder.Allow(scope, components)
	capability, err := builder.Build()
	if err != nil {
		t.Fatalf("AdapterCapabilityBuilder.Build: %v", err)
	}
	return capability
}

func mustCatalog(
	t *testing.T,
	host HostID,
	edition string,
	scope InstallScope,
	components ComponentSet,
) AdapterCatalog {
	t.Helper()
	capability := mustCapability(t, host, edition, scope, components)
	catalog, err := NewAdapterCatalog([]AdapterCapability{capability})
	if err != nil {
		t.Fatalf("NewAdapterCatalog: %v", err)
	}
	return catalog
}

func mustSimpleAdapterPlan(
	t *testing.T,
	root string,
	host HostID,
	edition string,
	scope InstallScope,
	components ComponentSet,
) HostAdapterInstallPlan {
	t.Helper()
	path := filepath.Join(root, ".host", string(host)+".config")
	return mustAdapterAtPath(t, root, host, edition, scope, components, path)
}

func mustAdapterAtPath(
	t *testing.T,
	root string,
	host HostID,
	edition string,
	scope InstallScope,
	components ComponentSet,
	path string,
) HostAdapterInstallPlan {
	t.Helper()
	component := components.Values()[0]
	output := mustOutput(t, path, component, []byte("host projection\n"))
	expectation, err := ExpectMissing(path)
	if err != nil {
		t.Fatalf("ExpectMissing: %v", err)
	}
	recovery := mustRecovery(t, host)
	builder := NewHostAdapterInstallPlanBuilder(host)
	builder = builder.AtEdition(edition)
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(scope, components)
	builder = builder.AddTargetRoot(root)
	builder = builder.AddOutput(expectation, output)
	builder = builder.RecoverWith(recovery)
	plan, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterInstallPlanBuilder.Build: %v", err)
	}
	return plan
}

func mustRichCodexAdapterPlan(
	t *testing.T,
	root string,
	components ComponentSet,
) HostAdapterInstallPlan {
	t.Helper()
	createPath := filepath.Join(root, ".codex", "config.toml")
	preservePath := filepath.Join(root, ".agents", "skills", "h-reason", "SKILL.md")
	updatePath := filepath.Join(root, ".agents", "skills", "h-spec", "SKILL.md")
	adoptPath := filepath.Join(root, ".agents", "skills", "h-frame", "SKILL.md")
	legacyUpdatePath := filepath.Join(root, ".agents", "skills", "h-note", "SKILL.md")
	removePath := filepath.Join(root, ".agents", "commands", "q-reason.md")
	conflictPath := filepath.Join(root, ".mcp.json")

	createOutput := mustOutput(t, createPath, ComponentMCP, []byte("mcp config\n"))
	preserveOutput := mustOutput(t, preservePath, ComponentSkills, []byte("current skill\n"))
	updateOutput := mustOutput(t, updatePath, ComponentSkills, []byte("new skill\n"))
	adoptOutput := mustOutput(t, adoptPath, ComponentSkills, []byte("legacy exact\n"))
	legacyUpdateOutput := mustOutput(t, legacyUpdatePath, ComponentSkills, []byte("new note skill\n"))

	createExpectation := mustMissing(t, createPath)
	preserveExpectation := mustExact(
		t,
		preservePath,
		PredecessorCurrentOwned,
		preserveOutput.Digest(),
	)
	updateExpectation := mustExact(
		t,
		updatePath,
		PredecessorOutdatedOwned,
		digestBytes([]byte("old skill\n")),
	)
	adoptExpectation := mustExact(
		t,
		adoptPath,
		PredecessorKnownLegacyExact,
		adoptOutput.Digest(),
	)
	legacyUpdateExpectation := mustExact(
		t,
		legacyUpdatePath,
		PredecessorKnownLegacyExact,
		digestBytes([]byte("old note skill\n")),
	)
	removeExpectation := mustExact(
		t,
		removePath,
		PredecessorOrphanedOwned,
		digestBytes([]byte("owned legacy command\n")),
	)
	removal, err := NewPlannedRemoval(removeExpectation, ComponentSkills)
	if err != nil {
		t.Fatalf("NewPlannedRemoval: %v", err)
	}
	conflict, err := NewForeignConflict(
		conflictPath,
		"existing MCP config has no Haft ownership receipt",
	)
	if err != nil {
		t.Fatalf("NewForeignConflict: %v", err)
	}
	recovery := mustRecovery(t, HostCodex)

	builder := NewHostAdapterInstallPlanBuilder(HostCodex)
	builder = builder.AtEdition("codex.v1")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(ScopeProject, components)
	builder = builder.AddTargetRoot(root)
	builder = builder.AddOutput(createExpectation, createOutput)
	builder = builder.AddOutput(preserveExpectation, preserveOutput)
	builder = builder.AddOutput(updateExpectation, updateOutput)
	builder = builder.AddOutput(adoptExpectation, adoptOutput)
	builder = builder.AddOutput(legacyUpdateExpectation, legacyUpdateOutput)
	builder = builder.AddRemoval(removal)
	builder = builder.AddConflict(conflict)
	builder = builder.RecoverWith(recovery)
	plan, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterInstallPlanBuilder.Build: %v", err)
	}
	return plan
}

func mustOutput(
	t *testing.T,
	path string,
	component Component,
	content []byte,
) RenderedOutput {
	t.Helper()
	output, err := NewRenderedOutput(path, component, content, 0o644)
	if err != nil {
		t.Fatalf("NewRenderedOutput: %v", err)
	}
	return output
}

func mustMissing(t *testing.T, path string) PathExpectation {
	t.Helper()
	expectation, err := ExpectMissing(path)
	if err != nil {
		t.Fatalf("ExpectMissing: %v", err)
	}
	return expectation
}

func mustExact(
	t *testing.T,
	path string,
	kind PredecessorKind,
	digest string,
) PathExpectation {
	t.Helper()
	basisKind := OwnershipManifestReceipt
	if kind == PredecessorKnownLegacyExact {
		basisKind = OwnershipLegacyRegistry
	}
	basisRef := string(basisKind) + ":fixture"
	basis, err := NewOwnershipBasis(
		basisKind,
		basisRef,
		digestBytes([]byte(basisRef)),
	)
	if err != nil {
		t.Fatalf("NewOwnershipBasis: %v", err)
	}
	var expectation PathExpectation
	switch kind {
	case PredecessorCurrentOwned:
		expectation, err = ExpectCurrentOwned(path, digest, 0o644, basis)
	case PredecessorOutdatedOwned:
		expectation, err = ExpectOutdatedOwned(path, digest, 0o644, basis)
	case PredecessorKnownLegacyExact:
		expectation, err = ExpectKnownLegacyExact(path, digest, 0o644, basis)
	case PredecessorOrphanedOwned:
		expectation, err = ExpectOrphanedOwned(path, digest, 0o644, basis)
	default:
		t.Fatalf("mustExact received unsupported predecessor kind %s", kind)
	}
	if err != nil {
		t.Fatalf("construct exact expectation: %v", err)
	}
	return expectation
}

func mustRecovery(t *testing.T, host HostID) RecoveryOperation {
	t.Helper()
	recovery, err := NewRecoveryOperation([]string{
		"haft",
		"host",
		"reconcile",
		"--host",
		string(host),
	})
	if err != nil {
		t.Fatalf("NewRecoveryOperation: %v", err)
	}
	return recovery
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return fmt.Sprintf("sha256:%x", digest)
}
