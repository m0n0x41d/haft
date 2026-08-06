package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestRunCarrierManifestText(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restore := stubCarrierManifestJSON(t, false)
	defer restore()

	if err := runCarrierManifest(cmd, nil); err != nil {
		t.Fatalf("runCarrierManifest returned error: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Carrier Authority Manifest v1",
		"agent-skill-carriers",
		"desktop-standalone-code",
		"cli-interactive-presentation-code",
		"dead_surface_policy:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("carrier manifest output missing %q:\n%s", want, text)
		}
	}
}

func TestRunCarrierManifestJSON(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restore := stubCarrierManifestJSON(t, true)
	defer restore()

	if err := runCarrierManifest(cmd, nil); err != nil {
		t.Fatalf("runCarrierManifest returned error: %v", err)
	}

	var manifest project.CarrierAuthorityManifest
	if err := json.Unmarshal(output.Bytes(), &manifest); err != nil {
		t.Fatalf("decode manifest JSON: %v\n%s", err, output.String())
	}
	if findings := project.ValidateCarrierAuthorityManifest(manifest); len(findings) > 0 {
		t.Fatalf("manifest findings = %#v", findings)
	}
}

func TestCarrierManifestHelpNamesAuthorityBoundaries(t *testing.T) {
	t.Parallel()

	normalized := strings.ToLower(strings.Join(strings.Fields(carrierManifestCmd.Long), " "))
	for _, want := range []string{
		"review/discovery metadata",
		"not binding authority",
		"default status",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("carrier manifest help missing %q:\n%s", want, carrierManifestCmd.Long)
		}
	}
}

func TestRunCarrierCheckText(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restore := stubCarrierCheckJSON(t, false)
	defer restore()

	if err := runCarrierCheck(cmd, nil); err != nil {
		t.Fatalf("runCarrierCheck returned error: %v", err)
	}
	if !strings.Contains(output.String(), "carrier semio check: clean") {
		t.Fatalf("output = %q", output.String())
	}
	if !strings.Contains(output.String(), "generated surface") {
		t.Fatalf("output should mention generated surface count: %q", output.String())
	}
}

func TestCarrierCheckHelpNamesAuthorityBoundaries(t *testing.T) {
	t.Parallel()

	normalized := strings.ToLower(strings.Join(strings.Fields(carrierCheckCmd.Long), " "))
	for _, want := range []string{
		"review inputs",
		"not evidence",
		"approval",
		"gate",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("carrier check help missing %q:\n%s", want, carrierCheckCmd.Long)
		}
	}
}

func TestRunCarrierCheckJSON(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restore := stubCarrierCheckJSON(t, true)
	defer restore()

	if err := runCarrierCheck(cmd, nil); err != nil {
		t.Fatalf("runCarrierCheck returned error: %v", err)
	}

	var result project.CarrierSemioCheckResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode check JSON: %v\n%s", err, output.String())
	}
	if len(result.Findings) > 0 {
		t.Fatalf("findings = %#v", result.Findings)
	}
	if len(result.CheckedGeneratedSurfaces) == 0 {
		t.Fatal("expected generated interface surfaces to be checked")
	}
}

func TestHandleQuintQueryCarrierManifest(t *testing.T) {
	t.Parallel()

	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "carrier_manifest",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery carrier_manifest returned error: %v", err)
	}

	var manifest project.CarrierAuthorityManifest
	if err := json.Unmarshal([]byte(result), &manifest); err != nil {
		t.Fatalf("decode carrier manifest JSON: %v\n%s", err, result)
	}
	if findings := project.ValidateCarrierAuthorityManifest(manifest); len(findings) > 0 {
		t.Fatalf("manifest findings = %#v", findings)
	}
}

func TestHandleQuintQueryCarrierCheck(t *testing.T) {
	t.Parallel()

	store := setupCLIArtifactStore(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "README.md"),
		[]byte("v8 dropped the standalone interactive agent, the TUI, and desktop wrappers.\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(context.Background(), store, nil, filepath.Join(root, ".haft"), map[string]any{
		"action": "carrier_check",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery carrier_check returned error: %v", err)
	}

	var check project.CarrierSemioCheckResult
	if err := json.Unmarshal([]byte(result), &check); err != nil {
		t.Fatalf("decode carrier check JSON: %v\n%s", err, result)
	}
	if len(check.CheckedFiles) != 1 || check.CheckedFiles[0] != "README.md" {
		t.Fatalf("checked_files = %#v, want README.md only", check.CheckedFiles)
	}
	if len(check.CheckedGeneratedSurfaces) == 0 {
		t.Fatal("expected generated interface surfaces in carrier_check")
	}
	if len(check.Findings) > 0 {
		t.Fatalf("findings = %#v", check.Findings)
	}
}

func TestCarrierCheckGeneratedSurfacesIncludeInterfaceCatalog(t *testing.T) {
	t.Parallel()

	surfaces := carrierCheckGeneratedSurfaces()
	if len(surfaces) == 0 {
		t.Fatal("expected generated interface surfaces")
	}
	found := false
	for _, surface := range surfaces {
		if surface.Path == "generated/interface/query.carrier_check" {
			found = true
			if !strings.Contains(surface.Content, "carrier_check") {
				t.Fatalf("carrier_check generated surface content = %q", surface.Content)
			}
		}
	}
	if !found {
		t.Fatalf("generated surfaces missing query.carrier_check: %#v", surfaces)
	}
}

func TestCarrierCheckGeneratedSurfacesIncludeMCPToolsListCatalog(t *testing.T) {
	t.Parallel()

	surfaces := carrierCheckGeneratedSurfaces()
	if len(surfaces) == 0 {
		t.Fatal("expected generated surfaces")
	}
	found := false
	for _, surface := range surfaces {
		if surface.Path == "generated/mcp-tools/haft_query" {
			found = true
			if !strings.Contains(surface.Content, "haft_query") {
				t.Fatalf("haft_query generated MCP surface content = %q", surface.Content)
			}
		}
	}
	if !found {
		t.Fatalf("generated surfaces missing generated/mcp-tools/haft_query: %#v", surfaces)
	}
}

func TestCarrierCheckGeneratedSurfacesIncludeContractGenerationFragments(t *testing.T) {
	t.Parallel()

	surfaces := carrierCheckGeneratedSurfaces()
	previewFound := false
	schemaFound := false

	for _, surface := range surfaces {
		switch surface.Path {
		case "generated/contract-generation/preview/query.contract_generation":
			previewFound = true
			if !strings.Contains(surface.Content, "read-only/generated text is discovery only") {
				t.Fatalf("contract_generation preview surface missing authority boundary: %q", surface.Content)
			}
		case "generated/contract-generation/schema/query.contract_generation":
			schemaFound = true
			for _, want := range []string{
				"schema fragment is read-only validation material",
				"contract_generation",
				"sha256:",
			} {
				if !strings.Contains(surface.Content, want) {
					t.Fatalf("contract_generation schema surface missing %q: %q", want, surface.Content)
				}
			}
		}
	}

	if !previewFound || !schemaFound {
		t.Fatalf("generated contract-generation surfaces missing preview=%v schema=%v: %#v", previewFound, schemaFound, surfaces)
	}
}

func TestCarrierCheckMCPToolSurfaceFlagsAuthorityGrantWording(t *testing.T) {
	t.Parallel()

	text := carrierCheckMCPToolSurfaceText(fpf.Tool{
		Name:        "haft_bad",
		Description: "Tool description authorizes operator approval.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"payload": map[string]interface{}{
					"type":        "string",
					"description": "MCP schema visibility is evidence for binding.",
				},
			},
		},
	})
	result, err := project.CheckCarrierSemioWithVirtualTexts(t.TempDir(), []project.CarrierSemioVirtualText{{
		Path:    "generated/mcp-tools/haft_bad",
		Content: text,
	}})
	if err != nil {
		t.Fatalf("CheckCarrierSemioWithVirtualTexts: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("findings = %#v, want two authority-boundary findings", result.Findings)
	}
}

func stubCarrierManifestJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := carrierManifestJSON
	carrierManifestJSON = value
	return func() { carrierManifestJSON = prev }
}

func stubCarrierCheckJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := carrierCheckJSON
	carrierCheckJSON = value
	return func() { carrierCheckJSON = prev }
}
