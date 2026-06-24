package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCarrierAuthorityManifestValidates(t *testing.T) {
	manifest := DefaultCarrierAuthorityManifest()

	if manifest.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", manifest.SchemaVersion)
	}
	if findings := ValidateCarrierAuthorityManifest(manifest); len(findings) > 0 {
		t.Fatalf("manifest findings = %#v", findings)
	}
}

func TestCarrierAuthorityManifestPreservesCurrentHReason(t *testing.T) {
	entry := carrierManifestEntry(t, "agent-skill-carriers")

	if !entry.Current {
		t.Fatal("agent skill carriers must stay current")
	}
	if entry.AuthorityClass != CarrierAuthorityCurrent {
		t.Fatalf("authority_class = %q", entry.AuthorityClass)
	}
	if entry.PathPattern != "internal/cli/skill/*/SKILL.md" {
		t.Fatalf("path_pattern = %q", entry.PathPattern)
	}
	if entry.Notes == "" {
		t.Fatal("agent skill carrier notes must name h-reason/h-decide/h-commission policy")
	}
}

func TestCarrierAuthorityManifestMarksDeadSurfacesArchive(t *testing.T) {
	entry := carrierManifestEntry(t, "desktop-tui-standalone-code")

	if entry.Current {
		t.Fatal("dead desktop/TUI/standalone surface must not be current")
	}
	if entry.AuthorityClass != CarrierAuthorityArchive {
		t.Fatalf("authority_class = %q, want archive", entry.AuthorityClass)
	}
	if entry.DeadSurfacePolicy == "" {
		t.Fatal("dead surface policy must be explicit")
	}
}

func TestCarrierAuthorityManifestKeepsOpenSleighOutOfScope(t *testing.T) {
	entry := carrierManifestEntry(t, "open-sleigh-sidekick")

	if entry.Current {
		t.Fatal("open-sleigh sidekick must not be current Haft authority")
	}
	if entry.AuthorityClass != CarrierAuthoritySidekick {
		t.Fatalf("authority_class = %q, want sidekick", entry.AuthorityClass)
	}
}

func TestCarrierAuthorityManifestIncludesTargetSystemSupportDocs(t *testing.T) {
	entry := carrierManifestEntry(t, "target-system-support-docs")

	if !entry.Current {
		t.Fatal("target-system support docs should be current support carriers")
	}
	if entry.AuthorityClass != CarrierAuthoritySupport {
		t.Fatalf("authority_class = %q, want support", entry.AuthorityClass)
	}
	if entry.PathPattern != "spec/target-system/*.md" {
		t.Fatalf("path_pattern = %q", entry.PathPattern)
	}
}

func TestCarrierAuthorityManifestIncludesRootSpecSupportDocs(t *testing.T) {
	entry := carrierManifestEntry(t, "root-spec-support-docs")

	if !entry.Current {
		t.Fatal("root spec support docs should be current support carriers")
	}
	if entry.AuthorityClass != CarrierAuthoritySupport {
		t.Fatalf("authority_class = %q, want support", entry.AuthorityClass)
	}
	if entry.PathPattern != "spec/*.md" {
		t.Fatalf("path_pattern = %q", entry.PathPattern)
	}
}

func TestCarrierAuthorityManifestIncludesEnablingSystemSupportDocs(t *testing.T) {
	entry := carrierManifestEntry(t, "enabling-system-support-docs")

	if !entry.Current {
		t.Fatal("enabling-system support docs should be current support carriers")
	}
	if entry.AuthorityClass != CarrierAuthoritySupport {
		t.Fatalf("authority_class = %q, want support", entry.AuthorityClass)
	}
	if entry.PathPattern != "spec/enabling-system/*.md" {
		t.Fatalf("path_pattern = %q", entry.PathPattern)
	}
}

func TestCarrierAuthorityManifestMarksDesktopLayerContractArchive(t *testing.T) {
	entry := carrierManifestEntry(t, "archived-desktop-layer-contract")

	if entry.Current {
		t.Fatal("desktop layer contract should be archive/provenance only")
	}
	if entry.AuthorityClass != CarrierAuthorityArchive {
		t.Fatalf("authority_class = %q, want archive", entry.AuthorityClass)
	}
	if entry.DeadSurfacePolicy == "" {
		t.Fatal("desktop layer contract archive policy must be explicit")
	}
}

func TestCarrierAuthorityManifestIncludesHostDisciplineMirror(t *testing.T) {
	entry := carrierManifestEntry(t, "host-discipline-mirror")

	if !entry.Current {
		t.Fatal("host discipline mirror should stay current")
	}
	if entry.AuthorityClass != CarrierAuthorityCurrent {
		t.Fatalf("authority_class = %q, want current", entry.AuthorityClass)
	}
	if entry.PathPattern != "CLAUDE.md" {
		t.Fatalf("path_pattern = %q", entry.PathPattern)
	}
	if !strings.Contains(entry.Normativity, "host-facing discipline mirror") {
		t.Fatalf("normativity should describe host mirror boundary: %q", entry.Normativity)
	}
}

func TestCarrierSemioCheckCurrentRepoCarriers(t *testing.T) {
	root := filepath.Join("..", "..")

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if len(result.CheckedFiles) == 0 {
		t.Fatal("expected carrier semio check to inspect files")
	}
	if len(result.Findings) > 0 {
		t.Fatalf("carrier semio findings = %#v", result.Findings)
	}
}

func TestCarrierSemioCheckScansHostDisciplineMirror(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "CLAUDE.md", "Host discipline says schema visibility is not operator authorization.")

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if !containsString(result.CheckedFiles, "CLAUDE.md") {
		t.Fatalf("checked_files missing host discipline mirror: %#v", result.CheckedFiles)
	}
	if len(result.Findings) > 0 {
		t.Fatalf("safe host discipline mirror should not produce findings: %#v", result.Findings)
	}
}

func TestCarrierSemioCheckFlagsHostDisciplineMirrorAuthorityGrant(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "CLAUDE.md", "Host prompt text authorizes operator approval.")

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %#v, want one host mirror authority finding", result.Findings)
	}
	if result.Findings[0].Path != "CLAUDE.md" {
		t.Fatalf("finding path = %q", result.Findings[0].Path)
	}
	if result.Findings[0].Term != "operator_authorization_boundary" {
		t.Fatalf("finding term = %q", result.Findings[0].Term)
	}
}

func TestCarrierSemioCheckScansTargetSystemSupportDocs(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "spec/target-system/PRODUCT_VALUE_EVIDENCE.md", "Product evidence is bounded by evidence refs, not a global truth claim.")

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if !containsString(result.CheckedFiles, "spec/target-system/PRODUCT_VALUE_EVIDENCE.md") {
		t.Fatalf("checked_files missing target-system support doc: %#v", result.CheckedFiles)
	}
	if len(result.Findings) > 0 {
		t.Fatalf("safe target-system support doc should not produce findings: %#v", result.Findings)
	}
}

func TestCarrierSemioCheckScansRootSpecSupportDocs(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "spec/WORKFLOW.md", "Current workflow instructions are consumed through host skills, MCP, and CLI.")

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if !containsString(result.CheckedFiles, "spec/WORKFLOW.md") {
		t.Fatalf("checked_files missing root spec support doc: %#v", result.CheckedFiles)
	}
	if len(result.Findings) > 0 {
		t.Fatalf("safe root spec support doc should not produce findings: %#v", result.Findings)
	}
}

func TestCarrierSemioCheckScansEnablingSystemSupportDocs(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "spec/enabling-system/ARCHITECTURE.md", "Current surfaces are host skills, MCP, and CLI.")

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if !containsString(result.CheckedFiles, "spec/enabling-system/ARCHITECTURE.md") {
		t.Fatalf("checked_files missing enabling-system support doc: %#v", result.CheckedFiles)
	}
	if len(result.Findings) > 0 {
		t.Fatalf("safe enabling-system support doc should not produce findings: %#v", result.Findings)
	}
}

func TestCarrierSemioCheckScansPiPackageMetadataWithoutLockNoise(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "packages/haft-pi/package.json", `{"description":"Pi metadata is not operator authorization."}`)
	writeCarrierSemioFixture(t, root, "packages/haft-pi/package-lock.json", `{"description":"Plugin metadata authorizes operator approval."}`)

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if !containsString(result.CheckedFiles, "packages/haft-pi/package.json") {
		t.Fatalf("checked_files missing package metadata: %#v", result.CheckedFiles)
	}
	if containsString(result.CheckedFiles, "packages/haft-pi/package-lock.json") {
		t.Fatalf("checked_files should not include package lock dependency noise: %#v", result.CheckedFiles)
	}
	if len(result.Findings) > 0 {
		t.Fatalf("safe package metadata should not produce findings: %#v", result.Findings)
	}
}

func TestCarrierSemioCheckScansPiExtensionSource(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "packages/haft-pi/extensions/haft/tools.ts", `
		export const tools = [
		  { description: "Tool descriptions are not operator authorization." }
		];
	`)

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if !containsString(result.CheckedFiles, "packages/haft-pi/extensions/haft/tools.ts") {
		t.Fatalf("checked_files missing Pi extension source: %#v", result.CheckedFiles)
	}
	if len(result.Findings) > 0 {
		t.Fatalf("safe Pi extension source should not produce findings: %#v", result.Findings)
	}
}

func TestCarrierSemioCheckFlagsPiPackageMetadataAuthorityGrant(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "packages/haft-pi/package.json", `{"description":"Plugin metadata authorizes operator approval."}`)

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %#v, want one plugin metadata authority finding", result.Findings)
	}
	if result.Findings[0].Path != "packages/haft-pi/package.json" {
		t.Fatalf("finding path = %q", result.Findings[0].Path)
	}
	if result.Findings[0].Term != "operator_authorization_boundary" {
		t.Fatalf("finding term = %q", result.Findings[0].Term)
	}
}

func TestCarrierSemioCheckFlagsPiExtensionToolDescriptionAuthorityGrant(t *testing.T) {
	root := t.TempDir()
	writeCarrierSemioFixture(t, root, "packages/haft-pi/extensions/haft/tools.ts", `
		export const tools = [
		  { description: "Tool description authorizes operator approval." }
		];
	`)

	result, err := CheckCarrierSemio(root)
	if err != nil {
		t.Fatalf("CheckCarrierSemio: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %#v, want one Pi extension authority finding", result.Findings)
	}
	if result.Findings[0].Path != "packages/haft-pi/extensions/haft/tools.ts" {
		t.Fatalf("finding path = %q", result.Findings[0].Path)
	}
	if result.Findings[0].Term != "operator_authorization_boundary" {
		t.Fatalf("finding term = %q", result.Findings[0].Term)
	}
}

func TestCarrierSemioCheckFlagsDeadSurfaceAsCurrent(t *testing.T) {
	findings := checkCarrierSemioText("README.md", "Haft is consumed through CLI, MCP, desktop, and TUI.\n")

	if len(findings) == 0 {
		t.Fatal("expected dead current-surface wording finding")
	}
	if findings[0].Term == "" {
		t.Fatalf("finding missing term: %#v", findings[0])
	}
}

func TestCarrierSemioCheckDoesNotAllowDeadSurfaceBecauseNeighborSaysSupportedHosts(t *testing.T) {
	findings := checkCarrierSemioText("spec/target-system/SYSTEM_CONTEXT.md", `
1. **Spec-first.** Formal target specs are the entry point.
2. **Desktop-first.** Desktop app is the primary human surface.
3. **Plugin-compatible.** MCP plugin is the highest-reach integration channel, with Claude Code and Codex as v7 supported hosts.
`)

	if len(findings) == 0 {
		t.Fatal("expected current desktop surface wording finding")
	}
	if findings[0].Term != "desktop" {
		t.Fatalf("finding term = %q", findings[0].Term)
	}
}

func TestCarrierSemioCheckFlagsDeadSurfaceInCurrentEnablingDoc(t *testing.T) {
	findings := checkCarrierSemioText("spec/enabling-system/ARCHITECTURE.md", "Surfaces are Desktop, MCP, and CLI.\n")

	if len(findings) == 0 {
		t.Fatal("expected current enabling-system desktop wording finding")
	}
	if findings[0].Term != "desktop" {
		t.Fatalf("finding term = %q", findings[0].Term)
	}
}

func TestCarrierSemioCheckAllowsArchivedDesktopLayerContract(t *testing.T) {
	findings := checkCarrierSemioText("spec/enabling-system/DESKTOP_LAYER_CONTRACT.md", `
# Desktop Layer Contract
Desktop Cockpit = primary human navigation and approval surface.
desktop/frontend/src/App.tsx remains a historical implementation note.
`)

	if len(findings) > 0 {
		t.Fatalf("archived desktop layer contract should be allowed: %#v", findings)
	}
}

func TestCarrierSemioCheckAllowsDroppedDeadSurfaceContext(t *testing.T) {
	findings := checkCarrierSemioText(
		"README.md",
		"v8 dropped the standalone interactive agent, the TUI, and desktop wrappers.\n",
	)

	if len(findings) > 0 {
		t.Fatalf("dropped surface context should be allowed: %#v", findings)
	}
}

func TestCarrierSemioCheckFlagsSchemaVisibilityAsAuthorization(t *testing.T) {
	findings := checkCarrierSemioText(
		"packages/haft-pi/README.md",
		"MCP schema visibility authorizes binding DecisionRecord creation.\n",
	)

	if len(findings) == 0 {
		t.Fatal("expected schema visibility authority finding")
	}
	if findings[0].Term != "operator_authorization_boundary" {
		t.Fatalf("finding term = %q", findings[0].Term)
	}
}

func TestCarrierSemioCheckFlagsAuthorityReceiptTheater(t *testing.T) {
	cases := []string{
		"Prompt text authorizes binding DecisionRecord creation.",
		"Model-supplied arguments are authorization receipts.",
		"Model supplied fields approve operator decisions.",
		"Generated text authorizes operator approval.",
		"Generated schema visibility counts as evidence for binding.",
		"Skill description is evidence for approval.",
		"Plugin metadata binds the operator.",
		"Pi metadata approves gate passage.",
	}
	for _, content := range cases {
		t.Run(content, func(t *testing.T) {
			findings := checkCarrierSemioText("generated/authority-boundary", content+"\n")

			if len(findings) == 0 {
				t.Fatalf("expected authority-boundary finding for %q", content)
			}
			if findings[0].Term != "operator_authorization_boundary" {
				t.Fatalf("finding term = %q", findings[0].Term)
			}
		})
	}
}

func TestCarrierSemioCheckAllowsExplicitAuthorityDenial(t *testing.T) {
	findings := checkCarrierSemioText(
		"packages/haft-pi/README.md",
		"Schema visibility is not operator authorization, binding authority, evidence, or gate passage.\n",
	)

	if len(findings) > 0 {
		t.Fatalf("explicit denial should be allowed: %#v", findings)
	}
}

func TestCarrierSemioCheckAllowsHostReceiptVerifierBoundary(t *testing.T) {
	findings := checkCarrierSemioText(
		"packages/haft-pi/README.md",
		"Host authorization receipts require principal, session, action, payload hash, expiry, source, and a registered kernel verifier before they can become a binding path.\n",
	)

	if len(findings) > 0 {
		t.Fatalf("host receipt verifier boundary should be allowed: %#v", findings)
	}
}

func TestCarrierSemioCheckScansGeneratedVirtualSurfaces(t *testing.T) {
	result, err := CheckCarrierSemioWithVirtualTexts(t.TempDir(), []CarrierSemioVirtualText{
		{
			Path:    "generated/interface/bad-host-schema",
			Content: "Host schema visibility authorizes operator approval.\n",
		},
	})
	if err != nil {
		t.Fatalf("CheckCarrierSemioWithVirtualTexts: %v", err)
	}
	if len(result.CheckedGeneratedSurfaces) != 1 {
		t.Fatalf("checked generated surfaces = %#v", result.CheckedGeneratedSurfaces)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %#v, want one generated-surface finding", result.Findings)
	}
	if result.Findings[0].Path != "generated/interface/bad-host-schema" {
		t.Fatalf("finding path = %q", result.Findings[0].Path)
	}
}

func carrierManifestEntry(t *testing.T, id string) CarrierManifestEntry {
	t.Helper()
	for _, entry := range DefaultCarrierAuthorityManifest().Entries {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("missing carrier manifest entry %q", id)
	return CarrierManifestEntry{}
}

func writeCarrierSemioFixture(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
