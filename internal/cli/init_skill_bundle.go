package cli

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"gopkg.in/yaml.v3"
)

const currentSkillSourceBundleEdition = "haft-public-skills.v2"

type skillSourceFrontmatter struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
}

func currentSkillSourceBundle() (initplanning.SkillSourceBundle, error) {
	catalog := haftInterfaceCatalog()
	report := buildInterfaceContractGenerationReport(catalog)
	inputs := make([]initplanning.SkillSourceInput, len(allSkills))
	for index, skill := range allSkills {
		input, err := skillSourceInputFromManifest(skill)
		if err != nil {
			return initplanning.SkillSourceBundle{}, err
		}
		inputs[index] = input
	}
	return initplanning.BuildSkillSourceBundle(
		currentSkillSourceBundleEdition,
		report.SourceDigest,
		inputs,
	)
}

func skillSourceInputFromManifest(
	skill skillManifest,
) (initplanning.SkillSourceInput, error) {
	frontmatter, err := parseSkillSourceFrontmatter(skill.Content)
	if err != nil {
		return initplanning.SkillSourceInput{}, fmt.Errorf("skill %s frontmatter: %w", skill.Name, err)
	}
	if frontmatter.Name != skill.Name {
		return initplanning.SkillSourceInput{}, fmt.Errorf(
			"skill manifest name %s differs from source name %s",
			skill.Name,
			frontmatter.Name,
		)
	}
	manualOnly := !skill.AllowImplicit
	if manualOnly != frontmatter.DisableModelInvocation {
		return initplanning.SkillSourceInput{}, fmt.Errorf(
			"skill %s invocation policy differs from its source frontmatter",
			skill.Name,
		)
	}
	policy := initplanning.SkillInvocationImplicitAllowed
	if manualOnly {
		policy = initplanning.SkillInvocationManualOnly
	}
	return initplanning.SkillSourceInput{
		Name:             skill.Name,
		Description:      strings.TrimSpace(frontmatter.Description),
		InvocationPolicy: policy,
		Content:          bytes.Clone(skill.Content),
	}, nil
}

func parseSkillSourceFrontmatter(
	content []byte,
) (skillSourceFrontmatter, error) {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return skillSourceFrontmatter{}, fmt.Errorf("opening delimiter is missing")
	}
	remainder := content[4:]
	end := bytes.Index(remainder, []byte("\n---"))
	if end < 0 {
		return skillSourceFrontmatter{}, fmt.Errorf("closing delimiter is missing")
	}
	var frontmatter skillSourceFrontmatter
	if err := yaml.Unmarshal(remainder[:end], &frontmatter); err != nil {
		return skillSourceFrontmatter{}, fmt.Errorf("decode YAML: %w", err)
	}
	if strings.TrimSpace(frontmatter.Name) == "" ||
		strings.TrimSpace(frontmatter.Description) == "" {
		return skillSourceFrontmatter{}, fmt.Errorf("name or description is missing")
	}
	return frontmatter, nil
}
