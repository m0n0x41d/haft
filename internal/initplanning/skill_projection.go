package initplanning

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

type SkillRewriteInput struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type skillRewriteSetWire struct {
	ID    string              `json:"id"`
	Rules []SkillRewriteInput `json:"rules"`
}

type SkillRewriteSet struct {
	wire   skillRewriteSetWire
	digest string
}

func NewSkillRewriteSet(
	id string,
	rules []SkillRewriteInput,
) (SkillRewriteSet, error) {
	if !adapterEditionPattern.MatchString(id) {
		return SkillRewriteSet{}, fmt.Errorf("skill rewrite-set ID is invalid")
	}
	seen := make(map[string]struct{}, len(rules))
	validated := slices.Clone(rules)
	for _, rule := range validated {
		if rule.From == "" || rule.To == "" || rule.From == rule.To {
			return SkillRewriteSet{}, fmt.Errorf("skill rewrite rule must be non-empty and changing")
		}
		if !utf8.ValidString(rule.From) || !utf8.ValidString(rule.To) {
			return SkillRewriteSet{}, fmt.Errorf("skill rewrite rule is not UTF-8")
		}
		if _, duplicate := seen[rule.From]; duplicate {
			return SkillRewriteSet{}, fmt.Errorf("skill rewrite set repeats source token %q", rule.From)
		}
		seen[rule.From] = struct{}{}
	}
	wire := skillRewriteSetWire{ID: id, Rules: validated}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return SkillRewriteSet{}, fmt.Errorf("encode skill rewrite set: %w", err)
	}
	return SkillRewriteSet{
		wire:   skillRewriteSetWire{ID: id, Rules: slices.Clone(validated)},
		digest: digestBytesForManifest(canonical),
	}, nil
}

func (rewrite SkillRewriteSet) ID() string {
	return rewrite.wire.ID
}

func (rewrite SkillRewriteSet) Digest() string {
	return rewrite.digest
}

func (rewrite SkillRewriteSet) Rules() []SkillRewriteInput {
	return slices.Clone(rewrite.wire.Rules)
}

func (rewrite SkillRewriteSet) Apply(content []byte) ([]byte, error) {
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("skill rewrite input is not UTF-8")
	}
	if rewrite.wire.ID == "" || !sha256DigestPattern.MatchString(rewrite.digest) {
		return nil, fmt.Errorf("skill rewrite set is invalid")
	}
	arguments := make([]string, 0, len(rewrite.wire.Rules)*2)
	for _, rule := range rewrite.wire.Rules {
		arguments = append(arguments, rule.From, rule.To)
	}
	if len(arguments) == 0 {
		return slices.Clone(content), nil
	}
	replacer := strings.NewReplacer(arguments...)
	return []byte(replacer.Replace(string(content))), nil
}

type SkillPolicyCarrierKind string

const (
	SkillPolicyInSourceFrontmatter SkillPolicyCarrierKind = "source_frontmatter"
	SkillPolicyCodexOpenAIYAML     SkillPolicyCarrierKind = "codex_openai_yaml"
)

type SkillComponentRenderer struct {
	host    HostID
	edition string
	prefix  []string
	policy  SkillPolicyCarrierKind
	rewrite SkillRewriteSet
}

func NewSkillComponentRenderer(
	host HostID,
	edition string,
	prefix []string,
	policy SkillPolicyCarrierKind,
	rewrite SkillRewriteSet,
) (SkillComponentRenderer, error) {
	if _, known := knownHosts[host]; !known {
		return SkillComponentRenderer{}, fmt.Errorf("skill renderer host is invalid")
	}
	if !adapterEditionPattern.MatchString(edition) {
		return SkillComponentRenderer{}, fmt.Errorf("skill renderer edition is invalid")
	}
	if policy != SkillPolicyInSourceFrontmatter && policy != SkillPolicyCodexOpenAIYAML {
		return SkillComponentRenderer{}, fmt.Errorf("skill renderer policy carrier is invalid")
	}
	for _, segment := range prefix {
		if segment == "" || segment != filepath.Base(segment) || segment == "." || segment == ".." {
			return SkillComponentRenderer{}, fmt.Errorf("skill renderer path prefix is invalid")
		}
	}
	if rewrite.wire.ID == "" || !sha256DigestPattern.MatchString(rewrite.digest) {
		return SkillComponentRenderer{}, fmt.Errorf("skill renderer rewrite set is invalid")
	}
	return SkillComponentRenderer{
		host:    host,
		edition: edition,
		prefix:  slices.Clone(prefix),
		policy:  policy,
		rewrite: SkillRewriteSet{
			wire: skillRewriteSetWire{
				ID:    rewrite.wire.ID,
				Rules: rewrite.Rules(),
			},
			digest: rewrite.digest,
		},
	}, nil
}

type RenderedSkillRecord struct {
	Name                 string
	SourceDescription    string
	InvocationPolicy     SkillInvocationPolicy
	SourceDigest         string
	RenderedSkillPath    string
	RenderedSkillDigest  string
	RenderedPolicyPath   string
	RenderedPolicyDigest string
}

type SkillComponentProjection struct {
	host                HostID
	edition             string
	bundleRef           string
	bundleDigest        string
	kernelCatalogDigest string
	rewriteID           string
	rewriteDigest       string
	policy              SkillPolicyCarrierKind
	root                string
	records             []RenderedSkillRecord
	outputs             []RenderedOutput
}

func (renderer SkillComponentRenderer) Render(
	bundle SkillSourceBundle,
	root string,
) (SkillComponentProjection, error) {
	canonicalRoot, err := parseCanonicalAbsolutePath(root)
	if err != nil {
		return SkillComponentProjection{}, fmt.Errorf("skill projection root: %w", err)
	}
	if len(bundle.canonical) == 0 || !sha256DigestPattern.MatchString(bundle.digest) {
		return SkillComponentProjection{}, fmt.Errorf("skill projection bundle is invalid")
	}
	categoryRoot := filepath.Join(append([]string{canonicalRoot}, renderer.prefix...)...)
	records := make([]RenderedSkillRecord, 0, len(bundle.wire.Skills))
	outputs := make([]RenderedOutput, 0, len(bundle.wire.Skills)*2)
	for _, source := range bundle.Skills() {
		rendered, err := renderer.rewrite.Apply(source.Content())
		if err != nil {
			return SkillComponentProjection{}, err
		}
		skillPath := filepath.Join(categoryRoot, source.Name(), "SKILL.md")
		skillOutput, err := NewRenderedOutput(
			skillPath,
			ComponentSkills,
			rendered,
			0o644,
		)
		if err != nil {
			return SkillComponentProjection{}, err
		}
		record := RenderedSkillRecord{
			Name:                source.Name(),
			SourceDescription:   source.Description(),
			InvocationPolicy:    source.InvocationPolicy(),
			SourceDigest:        source.ContentDigest(),
			RenderedSkillPath:   skillOutput.Path(),
			RenderedSkillDigest: skillOutput.Digest(),
		}
		outputs = append(outputs, skillOutput)
		if renderer.policy == SkillPolicyCodexOpenAIYAML {
			policyOutput, err := renderCodexPolicyOutput(
				categoryRoot,
				source,
			)
			if err != nil {
				return SkillComponentProjection{}, err
			}
			record.RenderedPolicyPath = policyOutput.Path()
			record.RenderedPolicyDigest = policyOutput.Digest()
			outputs = append(outputs, policyOutput)
		}
		records = append(records, record)
	}
	sort.Slice(records, func(left int, right int) bool {
		return records[left].Name < records[right].Name
	})
	sort.Slice(outputs, func(left int, right int) bool {
		return outputs[left].path < outputs[right].path
	})
	return SkillComponentProjection{
		host:                renderer.host,
		edition:             renderer.edition,
		bundleRef:           bundle.Ref(),
		bundleDigest:        bundle.Digest(),
		kernelCatalogDigest: bundle.KernelCatalogDigest(),
		rewriteID:           renderer.rewrite.ID(),
		rewriteDigest:       renderer.rewrite.Digest(),
		policy:              renderer.policy,
		root:                canonicalRoot,
		records:             records,
		outputs:             outputs,
	}, nil
}

func renderCodexPolicyOutput(
	categoryRoot string,
	source SkillSource,
) (RenderedOutput, error) {
	allowImplicit := source.InvocationPolicy() == SkillInvocationImplicitAllowed
	content := fmt.Sprintf(
		"policy:\n  allow_implicit_invocation: %t\n",
		allowImplicit,
	)
	path := filepath.Join(
		categoryRoot,
		source.Name(),
		"agents",
		"openai.yaml",
	)
	return NewRenderedOutput(
		path,
		ComponentSkills,
		[]byte(content),
		0o644,
	)
}

func (projection SkillComponentProjection) Host() HostID {
	return projection.host
}

func (projection SkillComponentProjection) Edition() string {
	return projection.edition
}

func (projection SkillComponentProjection) BundleRef() string {
	return projection.bundleRef
}

func (projection SkillComponentProjection) BundleDigest() string {
	return projection.bundleDigest
}

func (projection SkillComponentProjection) KernelCatalogDigest() string {
	return projection.kernelCatalogDigest
}

func (projection SkillComponentProjection) RewriteID() string {
	return projection.rewriteID
}

func (projection SkillComponentProjection) RewriteDigest() string {
	return projection.rewriteDigest
}

func (projection SkillComponentProjection) PolicyCarrier() SkillPolicyCarrierKind {
	return projection.policy
}

func (projection SkillComponentProjection) Root() string {
	return projection.root
}

func (projection SkillComponentProjection) Records() []RenderedSkillRecord {
	return slices.Clone(projection.records)
}

func (projection SkillComponentProjection) Outputs() []RenderedOutput {
	return cloneRenderedOutputs(projection.outputs)
}

type SkillDerivationParityReport struct {
	BundleRef           string
	BundleDigest        string
	KernelCatalogDigest string
	Hosts               []HostID
	Projections         []SkillProjectionIdentity
	SkillCount          int
}

type SkillProjectionIdentity struct {
	Host          HostID
	Edition       string
	RewriteID     string
	RewriteDigest string
	PolicyCarrier SkillPolicyCarrierKind
	Root          string
}

func VerifySkillDerivationParity(
	bundle SkillSourceBundle,
	projections []SkillComponentProjection,
) (SkillDerivationParityReport, error) {
	if len(projections) == 0 {
		return SkillDerivationParityReport{}, fmt.Errorf("skill parity needs at least one host projection")
	}
	sources := make(map[string]skillSourceWire, len(bundle.wire.Skills))
	for _, source := range bundle.wire.Skills {
		sources[source.Name] = source
	}
	identities := make([]SkillProjectionIdentity, 0, len(projections))
	seenHosts := make(map[HostID]struct{}, len(projections))
	for _, projection := range projections {
		if _, duplicate := seenHosts[projection.host]; duplicate {
			return SkillDerivationParityReport{}, fmt.Errorf("skill parity repeats host %s", projection.host)
		}
		if err := validateSkillProjection(bundle, sources, projection); err != nil {
			return SkillDerivationParityReport{}, err
		}
		seenHosts[projection.host] = struct{}{}
		identities = append(identities, SkillProjectionIdentity{
			Host:          projection.host,
			Edition:       projection.edition,
			RewriteID:     projection.rewriteID,
			RewriteDigest: projection.rewriteDigest,
			PolicyCarrier: projection.policy,
			Root:          projection.root,
		})
	}
	sort.Slice(identities, func(left int, right int) bool {
		return identities[left].Host < identities[right].Host
	})
	hosts := make([]HostID, len(identities))
	for index, identity := range identities {
		hosts[index] = identity.Host
	}
	return SkillDerivationParityReport{
		BundleRef:           bundle.Ref(),
		BundleDigest:        bundle.Digest(),
		KernelCatalogDigest: bundle.KernelCatalogDigest(),
		Hosts:               hosts,
		Projections:         identities,
		SkillCount:          len(sources),
	}, nil
}

func validateSkillProjection(
	bundle SkillSourceBundle,
	sources map[string]skillSourceWire,
	projection SkillComponentProjection,
) error {
	if err := validateSkillProjectionHeader(bundle, projection); err != nil {
		return err
	}
	outputs, err := indexSkillProjectionOutputs(projection)
	if err != nil {
		return err
	}
	return validateSkillProjectionRecords(sources, projection, outputs)
}

func validateSkillProjectionHeader(
	bundle SkillSourceBundle,
	projection SkillComponentProjection,
) error {
	if _, known := knownHosts[projection.host]; !known {
		return fmt.Errorf("skill projection host is not canonical")
	}
	if !adapterEditionPattern.MatchString(projection.edition) {
		return fmt.Errorf("host %s projection edition is invalid", projection.host)
	}
	if projection.bundleRef != bundle.Ref() ||
		projection.bundleDigest != bundle.Digest() ||
		projection.kernelCatalogDigest != bundle.KernelCatalogDigest() {
		return fmt.Errorf("host %s projection uses another source bundle", projection.host)
	}
	if !adapterEditionPattern.MatchString(projection.rewriteID) ||
		!sha256DigestPattern.MatchString(projection.rewriteDigest) {
		return fmt.Errorf("host %s projection rewrite identity is invalid", projection.host)
	}
	if projection.policy != SkillPolicyInSourceFrontmatter &&
		projection.policy != SkillPolicyCodexOpenAIYAML {
		return fmt.Errorf("host %s projection policy carrier is invalid", projection.host)
	}
	canonicalRoot, err := parseCanonicalAbsolutePath(projection.root)
	if err != nil || canonicalRoot != projection.root {
		return fmt.Errorf("host %s projection root is invalid", projection.host)
	}
	return nil
}

func indexSkillProjectionOutputs(
	projection SkillComponentProjection,
) (map[string]RenderedOutput, error) {
	outputs := make(map[string]RenderedOutput, len(projection.outputs))
	for _, output := range projection.outputs {
		if _, duplicate := outputs[output.path]; duplicate {
			return nil, fmt.Errorf("host %s projection repeats output %s", projection.host, output.path)
		}
		if !skillProjectionOutputIsValid(projection.root, output) {
			return nil, fmt.Errorf("host %s projection output %s is invalid", projection.host, output.path)
		}
		outputs[output.path] = output
	}
	return outputs, nil
}

func skillProjectionOutputIsValid(root string, output RenderedOutput) bool {
	canonical, err := parseCanonicalAbsolutePath(output.path)
	if err != nil || canonical != output.path {
		return false
	}
	relative, err := filepath.Rel(root, output.path)
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return output.Component() == ComponentSkills &&
		output.mode == 0o644 &&
		len(output.content) > 0 &&
		output.digest == digestBytesForManifest(output.content)
}

func validateSkillProjectionRecords(
	sources map[string]skillSourceWire,
	projection SkillComponentProjection,
	outputs map[string]RenderedOutput,
) error {
	if len(projection.records) != len(sources) {
		return fmt.Errorf("host %s projection has incomplete public skill coverage", projection.host)
	}
	expectedOutputCount := len(sources)
	if projection.policy == SkillPolicyCodexOpenAIYAML {
		expectedOutputCount *= 2
	}
	if len(outputs) != expectedOutputCount {
		return fmt.Errorf("host %s projection output coverage is incomplete", projection.host)
	}
	seenRecords := make(map[string]struct{}, len(projection.records))
	for _, record := range projection.records {
		if _, duplicate := seenRecords[record.Name]; duplicate {
			return fmt.Errorf("host %s projection repeats skill %s", projection.host, record.Name)
		}
		source, exists := sources[record.Name]
		if !exists {
			return fmt.Errorf("host %s projection includes unknown skill %s", projection.host, record.Name)
		}
		if err := validateSkillProjectionRecord(projection, source, record, outputs); err != nil {
			return err
		}
		seenRecords[record.Name] = struct{}{}
	}
	return nil
}

func validateSkillProjectionRecord(
	projection SkillComponentProjection,
	source skillSourceWire,
	record RenderedSkillRecord,
	outputs map[string]RenderedOutput,
) error {
	if record.SourceDigest != source.ContentDigest ||
		record.SourceDescription != source.Description ||
		record.InvocationPolicy != source.InvocationPolicy {
		return fmt.Errorf("host %s skill %s differs from bundle identity", projection.host, record.Name)
	}
	skillOutput, exists := outputs[record.RenderedSkillPath]
	if !exists ||
		record.RenderedSkillDigest != skillOutput.digest ||
		!strings.HasSuffix(record.RenderedSkillPath, filepath.Join(record.Name, "SKILL.md")) {
		return fmt.Errorf("host %s skill %s output identity is invalid", projection.host, record.Name)
	}
	if projection.policy == SkillPolicyInSourceFrontmatter {
		if record.RenderedPolicyPath != "" || record.RenderedPolicyDigest != "" {
			return fmt.Errorf("host %s skill %s has an unexpected policy carrier", projection.host, record.Name)
		}
		return nil
	}
	policyOutput, exists := outputs[record.RenderedPolicyPath]
	if !exists ||
		record.RenderedPolicyDigest != policyOutput.digest ||
		!strings.HasSuffix(record.RenderedPolicyPath, filepath.Join(record.Name, "agents", "openai.yaml")) {
		return fmt.Errorf("host %s skill %s policy identity is invalid", projection.host, record.Name)
	}
	allowImplicit := source.InvocationPolicy == SkillInvocationImplicitAllowed
	expectedPolicy := fmt.Sprintf(
		"policy:\n  allow_implicit_invocation: %t\n",
		allowImplicit,
	)
	if string(policyOutput.content) != expectedPolicy {
		return fmt.Errorf("host %s skill %s policy content is invalid", projection.host, record.Name)
	}
	return nil
}
