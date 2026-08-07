package initplanning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInstallationManifestIsCanonicalContentAddressedAndRoundTripsStrictly(t *testing.T) {
	root := canonicalTempRoot(t)
	plan := mustManifestAdapterPlan(t, root, false)
	manifest, err := BuildInstallationManifest(plan)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	if !sha256DigestPattern.MatchString(manifest.Digest()) {
		t.Fatalf("manifest digest = %q", manifest.Digest())
	}
	if !strings.HasSuffix(manifest.Ref(), strings.TrimPrefix(manifest.Digest(), "sha256:")) {
		t.Fatalf("manifest ref %q does not bind digest %q", manifest.Ref(), manifest.Digest())
	}
	parsed, err := ParseInstallationManifest(manifest.CanonicalBytes())
	if err != nil {
		t.Fatalf("ParseInstallationManifest: %v", err)
	}
	if parsed.Digest() != manifest.Digest() ||
		!bytes.Equal(parsed.CanonicalBytes(), manifest.CanonicalBytes()) {
		t.Fatal("parsed manifest changed canonical identity")
	}
	if strings.Contains(string(manifest.CanonicalBytes()), "created_at") ||
		strings.Contains(string(manifest.CanonicalBytes()), "updated_at") {
		t.Fatal("manifest deterministic identity includes event time")
	}

	var expanded map[string]any
	if err := json.Unmarshal(manifest.CanonicalBytes(), &expanded); err != nil {
		t.Fatalf("decode manifest fixture: %v", err)
	}
	expanded["unknown"] = "field"
	unknown, err := json.Marshal(expanded)
	if err != nil {
		t.Fatalf("encode unknown-field fixture: %v", err)
	}
	if _, err := ParseInstallationManifest(unknown); err == nil {
		t.Fatal("manifest parser accepted an unknown field")
	}
	pretty, err := json.MarshalIndent(expandedWithoutUnknown(expanded), "", "  ")
	if err != nil {
		t.Fatalf("encode non-canonical fixture: %v", err)
	}
	if _, err := ParseInstallationManifest(pretty); err == nil {
		t.Fatal("manifest parser accepted non-canonical JSON")
	}
}

func TestInstallationManifestIdentityIsIndependentOfBuilderInsertionOrder(t *testing.T) {
	root := canonicalTempRoot(t)
	forward := mustManifestAdapterPlan(t, root, false)
	reverse := mustManifestAdapterPlan(t, root, true)
	left, err := BuildInstallationManifest(forward)
	if err != nil {
		t.Fatalf("BuildInstallationManifest forward: %v", err)
	}
	right, err := BuildInstallationManifest(reverse)
	if err != nil {
		t.Fatalf("BuildInstallationManifest reverse: %v", err)
	}
	if left.Digest() != right.Digest() ||
		!bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatalf(
			"builder order changed manifest identity\nforward: %s\nreverse: %s",
			left.CanonicalBytes(),
			right.CanonicalBytes(),
		)
	}
	paths := left.RenderedPaths()
	if len(paths) != 2 || paths[0].Path >= paths[1].Path {
		t.Fatalf("manifest paths are not canonical: %+v", paths)
	}
}

func TestInstallationManifestRejectsBlockedPlanAndUnselectedOutputComponent(t *testing.T) {
	root := canonicalTempRoot(t)
	components := mustComponents(t, ComponentMCP)
	path := filepath.Join(root, ".codex", "config.toml")
	output := mustOutput(t, path, ComponentSkills, []byte("skill\n"))
	expectation := mustMissing(t, path)
	recovery := mustRecovery(t, HostCodex)
	builder := NewHostAdapterInstallPlanBuilder(HostCodex)
	builder = builder.AtEdition("codex.v1")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(ScopeProject, components)
	builder = builder.AddTargetRoot(root)
	builder = builder.AddOutput(expectation, output)
	builder = builder.RecoverWith(recovery)
	if _, err := builder.Build(); err == nil {
		t.Fatal("adapter plan accepted output from an unselected component")
	}

	blocked := mustManifestAdapterPlan(t, root, false)
	conflict, err := NewForeignConflict(
		filepath.Join(root, ".mcp.json"),
		"foreign config",
	)
	if err != nil {
		t.Fatalf("NewForeignConflict: %v", err)
	}
	blocked.conflicts = []InstallConflict{conflict}
	if _, err := BuildInstallationManifest(blocked); err == nil {
		t.Fatal("manifest was created from a blocked adapter plan")
	}
}

func TestManifestProvidesExactOwnershipReceiptBasis(t *testing.T) {
	root := canonicalTempRoot(t)
	manifest, err := BuildInstallationManifest(
		mustManifestAdapterPlan(t, root, false),
	)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	basis, err := NewOwnershipBasis(
		OwnershipManifestReceipt,
		manifest.Ref(),
		manifest.Digest(),
	)
	if err != nil {
		t.Fatalf("NewOwnershipBasis: %v", err)
	}
	if basis.Ref() != manifest.Ref() || basis.Digest() != manifest.Digest() {
		t.Fatalf("ownership basis = %+v", basis)
	}
}

func mustManifestAdapterPlan(
	t *testing.T,
	root string,
	reverse bool,
) HostAdapterInstallPlan {
	t.Helper()
	components := mustComponents(t, ComponentMCP, ComponentSkills)
	mcpPath := filepath.Join(root, ".codex", "config.toml")
	skillPath := filepath.Join(root, ".agents", "skills", "h-reason", "SKILL.md")
	mcpOutput := mustOutput(t, mcpPath, ComponentMCP, []byte("mcp\n"))
	skillOutput := mustOutput(t, skillPath, ComponentSkills, []byte("skill\n"))
	mcpExpectation := mustMissing(t, mcpPath)
	skillExpectation := mustMissing(t, skillPath)
	recovery := mustRecovery(t, HostCodex)
	builder := NewHostAdapterInstallPlanBuilder(HostCodex)
	builder = builder.AtEdition("codex.v1")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(ScopeProject, components)
	builder = builder.AddTargetRoot(root)
	if reverse {
		builder = builder.AddOutput(skillExpectation, skillOutput)
		builder = builder.AddOutput(mcpExpectation, mcpOutput)
	} else {
		builder = builder.AddOutput(mcpExpectation, mcpOutput)
		builder = builder.AddOutput(skillExpectation, skillOutput)
	}
	builder = builder.RecoverWith(recovery)
	plan, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterInstallPlanBuilder.Build: %v", err)
	}
	return plan
}

func mustPublicationIdentity(t *testing.T, root string) PublicationIdentity {
	t.Helper()
	identity, err := NewPublicationIdentity(PublicationIdentityInput{
		HaftVersion:         "v9.dev",
		ExecutablePath:      filepath.Join(root, "bin", "haft"),
		ExecutableDigest:    digestBytes([]byte("haft binary")),
		SkillBundleDigest:   digestBytes([]byte("skill bundle")),
		KernelCatalogDigest: digestBytes([]byte("kernel catalog")),
	})
	if err != nil {
		t.Fatalf("NewPublicationIdentity: %v", err)
	}
	return identity
}

func expandedWithoutUnknown(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		if key != "unknown" {
			result[key] = value
		}
	}
	return result
}

func TestInstallationManifestGetterCopiesCarrierSlices(t *testing.T) {
	root := canonicalTempRoot(t)
	manifest, err := BuildInstallationManifest(
		mustManifestAdapterPlan(t, root, false),
	)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	before := manifest.RenderedPaths()
	mutated := manifest.RenderedPaths()
	mutated[0].Path = filepath.Join(root, "changed")
	after := manifest.RenderedPaths()
	if !reflect.DeepEqual(after, before) {
		t.Fatal("manifest getter exposed mutable carrier storage")
	}
}

func TestProjectionInstallationManifestV2OwnsOnlyExactManagedFragments(
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

	manifest, err := BuildProjectionInstallationManifest(projection)
	if err != nil {
		t.Fatalf("BuildProjectionInstallationManifest: %v", err)
	}
	if manifest.Schema() != installationManifestSchemaV2 {
		t.Fatalf("manifest schema = %q, want v2", manifest.Schema())
	}
	if len(manifest.RenderedPaths()) != 0 {
		t.Fatalf("fragment-only manifest owns whole paths: %+v", manifest.RenderedPaths())
	}
	fragments := manifest.ManagedFragments()
	if len(fragments) != 2 {
		t.Fatalf("manifest managed fragments = %d, want 2", len(fragments))
	}
	if fragments[0].CarrierPath >= fragments[1].CarrierPath &&
		fragments[0].Selector >= fragments[1].Selector {
		t.Fatalf("manifest fragments are not canonical: %+v", fragments)
	}
	canonical := string(manifest.CanonicalBytes())
	for _, forbidden := range []string{
		`"carrier_digest"`,
		`"carrier_mode"`,
		`"create_mode"`,
		`"mode"`,
	} {
		if strings.Contains(canonical, forbidden) {
			t.Fatalf("fragment manifest claims shared carrier field %s: %s", forbidden, canonical)
		}
	}

	parsed, err := ParseInstallationManifest(manifest.CanonicalBytes())
	if err != nil {
		t.Fatalf("ParseInstallationManifest(v2): %v", err)
	}
	if parsed.Digest() != manifest.Digest() ||
		!bytes.Equal(parsed.CanonicalBytes(), manifest.CanonicalBytes()) {
		t.Fatal("parsed v2 manifest changed canonical identity")
	}
	baseline, err := parsed.ManagedFragmentBaseline()
	if err != nil {
		t.Fatalf("ManagedFragmentBaseline: %v", err)
	}
	if len(baseline.Records()) != 2 ||
		baseline.OwnershipBasis().Ref() != parsed.Ref() ||
		baseline.OwnershipBasis().Digest() != parsed.Digest() {
		t.Fatalf("managed fragment baseline is not manifest-backed: %+v", baseline)
	}
}

func TestInstallationManifestV1CanonicalBytesRemainBackwardCompatible(
	t *testing.T,
) {
	root := "/tmp/haft-v9-manifest-v1-fixture"
	manifest, err := BuildInstallationManifest(
		mustManifestAdapterPlan(t, root, false),
	)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	quotedRoot, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("quote fixture root: %v", err)
	}
	quotedExecutable, err := json.Marshal(filepath.Join(root, "bin", "haft"))
	if err != nil {
		t.Fatalf("quote fixture executable: %v", err)
	}
	quotedMCPPath, err := json.Marshal(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("quote fixture MCP path: %v", err)
	}
	quotedSkillPath, err := json.Marshal(
		filepath.Join(root, ".agents", "skills", "h-reason", "SKILL.md"),
	)
	if err != nil {
		t.Fatalf("quote fixture skill path: %v", err)
	}
	expected := fmt.Sprintf(
		`{"schema":"haft.host-installation-manifest/v1","project_root":%s,"project_id":"qnt_e3149c17","host":"codex","adapter_edition":"codex.v1","install_scope":"project","components":["mcp","skills"],"target_roots":[%s],"haft_version":"v9.dev","executable_path":%s,"executable_digest":%q,"skill_bundle_digest":%q,"kernel_catalog_digest":%q,"rendered_paths":[{"path":%s,"component":"skills","digest":%q,"mode":420},{"path":%s,"component":"mcp","digest":%q,"mode":420}]}`,
		quotedRoot,
		quotedRoot,
		quotedExecutable,
		digestBytes([]byte("haft binary")),
		digestBytes([]byte("skill bundle")),
		digestBytes([]byte("kernel catalog")),
		quotedSkillPath,
		digestBytes([]byte("skill\n")),
		quotedMCPPath,
		digestBytes([]byte("mcp\n")),
	)
	if !bytes.Equal(manifest.CanonicalBytes(), []byte(expected)) {
		t.Fatalf(
			"v1 manifest bytes changed\n got: %s\nwant: %s",
			manifest.CanonicalBytes(),
			expected,
		)
	}
	if manifest.Schema() != installationManifestSchemaV1 ||
		len(manifest.ManagedFragments()) != 0 {
		t.Fatalf("v1 manifest unexpectedly exposes managed fragments: %s", manifest.CanonicalBytes())
	}
	if _, err := ParseInstallationManifest([]byte(expected)); err != nil {
		t.Fatalf("ParseInstallationManifest(v1 fixture): %v", err)
	}
}
