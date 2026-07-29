package initplanning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const skillSourceBundleSchema = "haft.skill-source-bundle/v1"

type SkillInvocationPolicy string

const (
	SkillInvocationImplicitAllowed SkillInvocationPolicy = "implicit_allowed"
	SkillInvocationManualOnly      SkillInvocationPolicy = "manual_only"
)

type SkillSourceInput struct {
	Name             string
	Description      string
	InvocationPolicy SkillInvocationPolicy
	Content          []byte
}

type skillSourceWire struct {
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	InvocationPolicy SkillInvocationPolicy `json:"invocation_policy"`
	ContentDigest    string                `json:"content_digest"`
	Content          string                `json:"content"`
}

type skillSourceBundleWire struct {
	Schema              string            `json:"schema"`
	Edition             string            `json:"edition"`
	KernelCatalogDigest string            `json:"kernel_catalog_digest"`
	Skills              []skillSourceWire `json:"skills"`
}

type SkillSource struct {
	wire skillSourceWire
}

func (source SkillSource) Name() string {
	return source.wire.Name
}

func (source SkillSource) Description() string {
	return source.wire.Description
}

func (source SkillSource) InvocationPolicy() SkillInvocationPolicy {
	return source.wire.InvocationPolicy
}

func (source SkillSource) ContentDigest() string {
	return source.wire.ContentDigest
}

func (source SkillSource) Content() []byte {
	return []byte(source.wire.Content)
}

type SkillSourceBundle struct {
	wire      skillSourceBundleWire
	canonical []byte
	digest    string
}

func BuildSkillSourceBundle(
	edition string,
	kernelCatalogDigest string,
	inputs []SkillSourceInput,
) (SkillSourceBundle, error) {
	if !adapterEditionPattern.MatchString(edition) {
		return SkillSourceBundle{}, fmt.Errorf("skill-source bundle edition is invalid")
	}
	if !sha256DigestPattern.MatchString(kernelCatalogDigest) {
		return SkillSourceBundle{}, fmt.Errorf("skill-source bundle kernel catalog digest is invalid")
	}
	if len(inputs) == 0 {
		return SkillSourceBundle{}, fmt.Errorf("skill-source bundle has no public skills")
	}
	skills := make([]skillSourceWire, len(inputs))
	for index, input := range inputs {
		skill, err := skillSourceWireFromInput(input)
		if err != nil {
			return SkillSourceBundle{}, err
		}
		skills[index] = skill
	}
	sort.Slice(skills, func(left int, right int) bool {
		return skills[left].Name < skills[right].Name
	})
	wire := skillSourceBundleWire{
		Schema:              skillSourceBundleSchema,
		Edition:             edition,
		KernelCatalogDigest: kernelCatalogDigest,
		Skills:              skills,
	}
	return newSkillSourceBundle(wire)
}

func ParseSkillSourceBundle(raw []byte) (SkillSourceBundle, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var wire skillSourceBundleWire
	if err := decoder.Decode(&wire); err != nil {
		return SkillSourceBundle{}, fmt.Errorf("decode skill-source bundle: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return SkillSourceBundle{}, fmt.Errorf("skill-source bundle has trailing JSON")
	}
	bundle, err := newSkillSourceBundle(wire)
	if err != nil {
		return SkillSourceBundle{}, err
	}
	if !bytes.Equal(raw, bundle.canonical) {
		return SkillSourceBundle{}, fmt.Errorf("skill-source bundle is not canonical JSON")
	}
	return bundle, nil
}

func skillSourceWireFromInput(input SkillSourceInput) (skillSourceWire, error) {
	name := strings.TrimSpace(input.Name)
	if name != input.Name || !skillNamePattern.MatchString(name) {
		return skillSourceWire{}, fmt.Errorf("skill-source name is invalid")
	}
	description := strings.TrimSpace(input.Description)
	if description == "" || description != input.Description {
		return skillSourceWire{}, fmt.Errorf("skill-source description is invalid")
	}
	if input.InvocationPolicy != SkillInvocationImplicitAllowed &&
		input.InvocationPolicy != SkillInvocationManualOnly {
		return skillSourceWire{}, fmt.Errorf("skill-source invocation policy is invalid")
	}
	if len(input.Content) == 0 || !utf8.Valid(input.Content) {
		return skillSourceWire{}, fmt.Errorf("skill-source content is empty or not UTF-8")
	}
	nameMarker := "name: " + name
	if !frontmatterContainsLine(input.Content, nameMarker) {
		return skillSourceWire{}, fmt.Errorf("skill-source content does not carry its exact name")
	}
	manualMarker := frontmatterContainsLine(
		input.Content,
		"disable-model-invocation: true",
	)
	if input.InvocationPolicy == SkillInvocationManualOnly && !manualMarker {
		return skillSourceWire{}, fmt.Errorf("manual skill source lacks its invocation gate")
	}
	if input.InvocationPolicy == SkillInvocationImplicitAllowed && manualMarker {
		return skillSourceWire{}, fmt.Errorf("implicit skill source carries a manual-only gate")
	}
	content := slices.Clone(input.Content)
	return skillSourceWire{
		Name:             name,
		Description:      description,
		InvocationPolicy: input.InvocationPolicy,
		ContentDigest:    digestBytesForManifest(content),
		Content:          string(content),
	}, nil
}

func frontmatterContainsLine(content []byte, marker string) bool {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return false
	}
	frontmatter := text[4 : 4+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.TrimSpace(line) == marker {
			return true
		}
	}
	return false
}

func newSkillSourceBundle(
	wire skillSourceBundleWire,
) (SkillSourceBundle, error) {
	if err := validateSkillSourceBundleWire(wire); err != nil {
		return SkillSourceBundle{}, err
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return SkillSourceBundle{}, fmt.Errorf("encode skill-source bundle: %w", err)
	}
	return SkillSourceBundle{
		wire:      cloneSkillSourceBundleWire(wire),
		canonical: canonical,
		digest:    digestBytesForManifest(canonical),
	}, nil
}

func validateSkillSourceBundleWire(wire skillSourceBundleWire) error {
	if wire.Schema != skillSourceBundleSchema {
		return fmt.Errorf("skill-source bundle schema is not current")
	}
	if !adapterEditionPattern.MatchString(wire.Edition) {
		return fmt.Errorf("skill-source bundle edition is invalid")
	}
	if !sha256DigestPattern.MatchString(wire.KernelCatalogDigest) {
		return fmt.Errorf("skill-source bundle kernel catalog digest is invalid")
	}
	if len(wire.Skills) == 0 {
		return fmt.Errorf("skill-source bundle has no public skills")
	}
	previous := ""
	for _, skill := range wire.Skills {
		if skill.Name <= previous {
			return fmt.Errorf("skill-source bundle skills are not unique and sorted")
		}
		rebuilt, err := skillSourceWireFromInput(SkillSourceInput{
			Name:             skill.Name,
			Description:      skill.Description,
			InvocationPolicy: skill.InvocationPolicy,
			Content:          []byte(skill.Content),
		})
		if err != nil {
			return err
		}
		if rebuilt.ContentDigest != skill.ContentDigest {
			return fmt.Errorf("skill-source %s content digest is invalid", skill.Name)
		}
		previous = skill.Name
	}
	return nil
}

func cloneSkillSourceBundleWire(
	wire skillSourceBundleWire,
) skillSourceBundleWire {
	return skillSourceBundleWire{
		Schema:              wire.Schema,
		Edition:             wire.Edition,
		KernelCatalogDigest: wire.KernelCatalogDigest,
		Skills:              slices.Clone(wire.Skills),
	}
}

func (bundle SkillSourceBundle) CanonicalBytes() []byte {
	return slices.Clone(bundle.canonical)
}

func (bundle SkillSourceBundle) Digest() string {
	return bundle.digest
}

func (bundle SkillSourceBundle) Ref() string {
	digest := strings.TrimPrefix(bundle.digest, "sha256:")
	return "skill-source-bundle:" + digest
}

func (bundle SkillSourceBundle) Edition() string {
	return bundle.wire.Edition
}

func (bundle SkillSourceBundle) KernelCatalogDigest() string {
	return bundle.wire.KernelCatalogDigest
}

func (bundle SkillSourceBundle) Skills() []SkillSource {
	result := make([]SkillSource, len(bundle.wire.Skills))
	for index, wire := range bundle.wire.Skills {
		result[index] = SkillSource{wire: wire}
	}
	return result
}
