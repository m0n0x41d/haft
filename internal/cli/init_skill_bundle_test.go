package cli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestCurrentSkillSourceBundleBindsExactPublicCatalogAndKernelDigest(
	t *testing.T,
) {
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	if bundle.KernelCatalogDigest() != report.SourceDigest {
		t.Fatalf(
			"bundle kernel digest = %s, want %s",
			bundle.KernelCatalogDigest(),
			report.SourceDigest,
		)
	}
	if bundle.Edition() != currentSkillSourceBundleEdition {
		t.Fatalf("bundle edition = %s", bundle.Edition())
	}
	if len(bundle.Skills()) != len(allSkills) {
		t.Fatalf("bundle skill count = %d, want %d", len(bundle.Skills()), len(allSkills))
	}
	t.Logf(
		"bundle=%s kernel=%s skills=%d",
		bundle.Digest(),
		bundle.KernelCatalogDigest(),
		len(bundle.Skills()),
	)
	manifestByName := make(map[string]skillManifest, len(allSkills))
	for _, skill := range allSkills {
		manifestByName[skill.Name] = skill
	}
	for _, source := range bundle.Skills() {
		manifest, exists := manifestByName[source.Name()]
		if !exists {
			t.Fatalf("bundle added unknown public skill %s", source.Name())
		}
		digest := sha256.Sum256(manifest.Content)
		wantDigest := fmt.Sprintf("sha256:%x", digest)
		if source.ContentDigest() != wantDigest {
			t.Fatalf("skill %s digest = %s, want %s", source.Name(), source.ContentDigest(), wantDigest)
		}
		wantPolicy := initplanning.SkillInvocationImplicitAllowed
		if !manifest.AllowImplicit {
			wantPolicy = initplanning.SkillInvocationManualOnly
		}
		if source.InvocationPolicy() != wantPolicy {
			t.Fatalf("skill %s policy = %s, want %s", source.Name(), source.InvocationPolicy(), wantPolicy)
		}
		if source.Description() == "" {
			t.Fatalf("skill %s lost its source description", source.Name())
		}
		if !bytes.Equal(source.Content(), manifest.Content) {
			t.Fatalf("skill %s bundle content is not the exact canonical source bytes", source.Name())
		}
	}
}

func TestCurrentSkillSourceBundlePublishesTaskLevelMemoryUX(t *testing.T) {
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	contentByName := make(map[string]string, len(bundle.Skills()))
	for _, skill := range bundle.Skills() {
		content := string(skill.Content())
		contentByName[skill.Name()] = content
		for _, forbidden := range []string{
			"haft memory typeenv",
			`haft_memory(action="admit")`,
			"preparation is read-only",
			"read-only preparation",
		} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("public skill %s exposes low-level memory UX %q", skill.Name(), forbidden)
			}
		}
	}
	required := map[string][]string{
		"h-onboard": {
			"mcp__haft__haft_onboard",
			`action="status"`,
			`action="profile_prepare"`,
			"haft onboard profile apply",
			"installs default project memory",
			"ask the operator to enable, defer, select, or understand a memory schema",
		},
		"h-decide": {
			"## DecisionRecord route",
			"Routes one direct, unambiguous operator request",
			"host_routed_operator_request",
			"disable-model-invocation: false",
		},
		"h-reason": {
			"mcp__haft__haft_entity",
			"mcp__haft__haft_onboard",
			"`known_absent` says only",
			"operator-named or agent-inferred",
			"establish the minimum EntityOfConcern without asking for separate permission",
			"same idempotency key",
		},
		"h-note": {
			"mcp__haft__haft_entity",
			"mcp__haft__haft_onboard",
			"canonical order",
		},
	}
	for skillName, fragments := range required {
		content, exists := contentByName[skillName]
		if !exists {
			t.Fatalf("public skill %s is missing", skillName)
		}
		for _, fragment := range fragments {
			if !strings.Contains(content, fragment) {
				t.Errorf("public skill %s missing %q", skillName, fragment)
			}
		}
	}
}

func TestSkillSourceInputRejectsManifestFrontmatterPolicyDrift(t *testing.T) {
	implicitSource := []byte("---\nname: h-test\ndescription: test\n---\nbody\n")
	manualSource := []byte("---\nname: h-test\ndescription: test\ndisable-model-invocation: true\n---\nbody\n")
	for name, manifest := range map[string]skillManifest{
		"wrong name": {
			Name: "h-other", Content: implicitSource, AllowImplicit: true,
		},
		"manual source marked implicit": {
			Name: "h-test", Content: manualSource, AllowImplicit: true,
		},
		"implicit source marked manual": {
			Name: "h-test", Content: implicitSource, AllowImplicit: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := skillSourceInputFromManifest(manifest); err == nil {
				t.Fatal("skill manifest/source drift was accepted")
			}
		})
	}
}
