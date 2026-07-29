package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

var publicHaftSkillContractSourcePattern = regexp.MustCompile(
	`(?m)^<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:[0-9a-f]{64} -->$`,
)

func currentPublicTakeoverDigest(
	output initplanning.RenderedOutput,
) (string, error) {
	digest, recognized, err := observePublicLegacyHaftSkill(output)
	if err != nil {
		return "", err
	}
	if recognized {
		return digest, nil
	}
	return output.Digest(), nil
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
	usesHaftMCP := bytes.Contains(
		observed,
		[]byte("mcp__haft__haft_"),
	)
	namesHaft := bytes.Contains(observed, []byte("Haft"))
	return usesHaftMCP && namesHaft
}
