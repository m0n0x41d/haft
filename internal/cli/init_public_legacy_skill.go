package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

var publicHaftSkillContractSourcePattern = regexp.MustCompile(
	`(?m)^<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:[0-9a-f]{64} -->$`,
)

var publicHaftSkillToolNamespaces = []string{
	"mcp__haft__haft_",
	"haft__haft_",
	"haft_",
}

var publicHaftSkillToolNames = []string{
	"problem",
	"solution",
	"decision",
	"query",
	"note",
	"refresh",
	"commission",
	"spec_section",
	"onboard",
	"entity",
	"method",
}

func observePublicLegacyCodexSkillPolicy(
	output initplanning.RenderedOutput,
	recognizedSkillRoots map[string]struct{},
) (string, bool, error) {
	if output.Component() != initplanning.ComponentSkills ||
		filepath.Base(output.Path()) != "openai.yaml" ||
		filepath.Base(filepath.Dir(output.Path())) != "agents" {
		return "", false, nil
	}
	skillRoot := filepath.Dir(filepath.Dir(output.Path()))
	if _, recognized := recognizedSkillRoots[skillRoot]; !recognized ||
		!isCanonicalPublicCodexSkillPolicy(output.Content()) {
		return "", false, nil
	}
	info, err := os.Lstat(output.Path())
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf(
			"inspect possible legacy Codex skill policy %s: %w",
			output.Path(),
			err,
		)
	}
	if !info.Mode().IsRegular() ||
		info.Size() > publicInitMaxCarrierBytes {
		return "", false, nil
	}
	observed, err := os.ReadFile(output.Path())
	if err != nil {
		return "", false, fmt.Errorf(
			"read possible legacy Codex skill policy %s: %w",
			output.Path(),
			err,
		)
	}
	if !isCanonicalPublicCodexSkillPolicy(observed) {
		return "", false, nil
	}
	return publicContentDigest(observed), true, nil
}

func isCanonicalPublicCodexSkillPolicy(content []byte) bool {
	return bytes.Equal(
		content,
		[]byte("policy:\n  allow_implicit_invocation: true\n"),
	) || bytes.Equal(
		content,
		[]byte("policy:\n  allow_implicit_invocation: false\n"),
	)
}

func observePublicLegacyHaftSkill(
	output initplanning.RenderedOutput,
) (string, bool, error) {
	if output.Component() != initplanning.ComponentSkills {
		return "", false, nil
	}
	info, err := os.Lstat(output.Path())
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf(
			"inspect possible legacy Haft skill %s: %w",
			output.Path(),
			err,
		)
	}
	if !info.Mode().IsRegular() ||
		info.Size() > publicInitMaxCarrierBytes {
		return "", false, nil
	}
	observed, err := os.ReadFile(output.Path())
	if err != nil {
		return "", false, fmt.Errorf(
			"read possible legacy Haft skill %s: %w",
			output.Path(),
			err,
		)
	}
	if !isPublicLegacyHaftSkill(output, observed) {
		return "", false, nil
	}
	return publicContentDigest(observed), true, nil
}

func isPublicLegacyHaftSkill(
	output initplanning.RenderedOutput,
	observed []byte,
) bool {
	if filepath.Base(output.Path()) != "SKILL.md" {
		return false
	}
	expected, err := parseSkillSourceFrontmatter(output.Content())
	if err != nil {
		return false
	}
	actual, err := parseSkillSourceFrontmatter(observed)
	if err != nil {
		return false
	}
	directoryName := filepath.Base(filepath.Dir(output.Path()))
	if actual.Name != expected.Name ||
		actual.Name != directoryName {
		return false
	}
	heading := []byte("\n# " + actual.Name + " ")
	if !bytes.Contains(observed, heading) {
		return false
	}
	if publicHaftSkillContractSourcePattern.Match(observed) {
		return true
	}
	usesHaftToolNamespace := sharesPublicHaftToolNamespace(
		output.Content(),
		observed,
	)
	// The exact Haft MCP namespace is the product identity witness. Published
	// v8.x skills used that namespace but did not necessarily spell the product
	// name in prose, so a case-sensitive prose token cannot be an ownership
	// prerequisite.
	return usesHaftToolNamespace
}

func sharesPublicHaftToolNamespace(
	expected []byte,
	observed []byte,
) bool {
	expectedNamespace := publicHaftSkillToolNamespace(expected)
	observedNamespace := publicHaftSkillToolNamespace(observed)
	hasExpectedNamespace := expectedNamespace != ""
	usesExpectedNamespace := observedNamespace == expectedNamespace
	return hasExpectedNamespace && usesExpectedNamespace
}

func publicHaftSkillToolNamespace(content []byte) string {
	namespaceIndex := slices.IndexFunc(
		publicHaftSkillToolNamespaces,
		func(namespace string) bool {
			return publicHaftSkillUsesToolNamespace(
				content,
				namespace,
			)
		},
	)
	if namespaceIndex < 0 {
		return ""
	}
	return publicHaftSkillToolNamespaces[namespaceIndex]
}

func publicHaftSkillUsesToolNamespace(
	content []byte,
	namespace string,
) bool {
	return slices.ContainsFunc(
		publicHaftSkillToolNames,
		func(toolName string) bool {
			signatureText := namespace + toolName
			signature := []byte(signatureText)
			return bytes.Contains(content, signature)
		},
	)
}
