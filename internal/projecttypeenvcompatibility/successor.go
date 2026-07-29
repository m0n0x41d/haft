package projecttypeenvcompatibility

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	successorRuleCanonicalDomain = "haft.project-typeenv-successor-rule.v1"
	successorDiffCanonicalDomain = "haft.project-typeenv-successor-diff.v1"
)

// SuccessorRuleClass is the closed transition taxonomy required before a
// candidate TypeEnv may replace a selected predecessor. Additive and removed
// describe coordinate presence. Widened and narrowed are emitted only when
// the compiler can prove set inclusion for the executable rule. CompilerGap
// is an honest refusal to infer an order from two merely different rules.
type SuccessorRuleClass uint8

const (
	SuccessorUnchanged SuccessorRuleClass = iota + 1
	SuccessorAdditive
	SuccessorWidened
	SuccessorNarrowed
	SuccessorRemoved
	SuccessorCompilerGap
)

func (class SuccessorRuleClass) String() string {
	switch class {
	case SuccessorUnchanged:
		return "unchanged"
	case SuccessorAdditive:
		return "additive"
	case SuccessorWidened:
		return "widened"
	case SuccessorNarrowed:
		return "narrowed"
	case SuccessorRemoved:
		return "removed"
	case SuccessorCompilerGap:
		return "compiler_gap"
	default:
		return ""
	}
}

func parseSuccessorRuleClass(raw string) (SuccessorRuleClass, error) {
	classes := []SuccessorRuleClass{
		SuccessorUnchanged,
		SuccessorAdditive,
		SuccessorWidened,
		SuccessorNarrowed,
		SuccessorRemoved,
		SuccessorCompilerGap,
	}
	for _, class := range classes {
		if class.String() == raw {
			return class, nil
		}
	}
	return 0, fmt.Errorf("unsupported successor rule class %q", raw)
}

// SuccessorRuleGround is a closed machine-readable explanation for one
// classification. It is deliberately not free-form compatibility prose.
type SuccessorRuleGround string

const (
	GroundExactSemanticMatch             SuccessorRuleGround = "exact_semantic_match"
	GroundSemanticCoordinateAdded        SuccessorRuleGround = "semantic_coordinate_added"
	GroundSemanticCoordinateRemoved      SuccessorRuleGround = "semantic_coordinate_removed"
	GroundCandidateDomainExpanded        SuccessorRuleGround = "candidate_domain_expanded"
	GroundCandidateDomainRestricted      SuccessorRuleGround = "candidate_domain_restricted"
	GroundContextDomainExpanded          SuccessorRuleGround = "context_domain_expanded"
	GroundContextDomainRestricted        SuccessorRuleGround = "context_domain_restricted"
	GroundContextDomainsIncomparable     SuccessorRuleGround = "context_domains_incomparable"
	GroundCardinalityDomainExpanded      SuccessorRuleGround = "cardinality_domain_expanded"
	GroundCardinalityDomainRestricted    SuccessorRuleGround = "cardinality_domain_restricted"
	GroundDisjointnessDomainExpanded     SuccessorRuleGround = "disjointness_domain_expanded"
	GroundDisjointnessDomainRestricted   SuccessorRuleGround = "disjointness_domain_restricted"
	GroundEnumerationRuleChanged         SuccessorRuleGround = "enumeration_rule_changed"
	GroundSlotTargetChanged              SuccessorRuleGround = "slot_target_changed"
	GroundCardinalityDomainsIncomparable SuccessorRuleGround = "cardinality_domains_incomparable"
	GroundConstraintDomainsIncomparable  SuccessorRuleGround = "constraint_domains_incomparable"
	GroundStoredValueMigrationRequired   SuccessorRuleGround = "stored_value_migration_required"
	GroundSemanticOrderNotImplemented    SuccessorRuleGround = "semantic_order_not_implemented"
)

var knownSuccessorRuleGrounds = map[SuccessorRuleGround]struct{}{
	GroundExactSemanticMatch:             {},
	GroundSemanticCoordinateAdded:        {},
	GroundSemanticCoordinateRemoved:      {},
	GroundCandidateDomainExpanded:        {},
	GroundCandidateDomainRestricted:      {},
	GroundContextDomainExpanded:          {},
	GroundContextDomainRestricted:        {},
	GroundContextDomainsIncomparable:     {},
	GroundCardinalityDomainExpanded:      {},
	GroundCardinalityDomainRestricted:    {},
	GroundDisjointnessDomainExpanded:     {},
	GroundDisjointnessDomainRestricted:   {},
	GroundEnumerationRuleChanged:         {},
	GroundSlotTargetChanged:              {},
	GroundCardinalityDomainsIncomparable: {},
	GroundConstraintDomainsIncomparable:  {},
	GroundStoredValueMigrationRequired:   {},
	GroundSemanticOrderNotImplemented:    {},
}

func (ground SuccessorRuleGround) valid() bool {
	_, found := knownSuccessorRuleGrounds[ground]
	return found
}

// SuccessorRuleAssessment retains the exact semantic coordinate and the
// before/after fingerprints used to classify it. Unchanged rows are retained:
// omitting them would make the successor report unable to prove its examined
// rule surface.
type SuccessorRuleAssessment struct {
	family Family
	key    string
	class  SuccessorRuleClass
	ground SuccessorRuleGround
	before typedmemory.SHA256Digest
	after  typedmemory.SHA256Digest
}

func (rule SuccessorRuleAssessment) Family() Family { return rule.family }

func (rule SuccessorRuleAssessment) Key() string { return rule.key }

func (rule SuccessorRuleAssessment) Class() SuccessorRuleClass { return rule.class }

func (rule SuccessorRuleAssessment) Ground() SuccessorRuleGround { return rule.ground }

func (rule SuccessorRuleAssessment) BeforeDigest() (typedmemory.SHA256Digest, bool) {
	return rule.before, rule.class != SuccessorAdditive
}

func (rule SuccessorRuleAssessment) AfterDigest() (typedmemory.SHA256Digest, bool) {
	return rule.after, rule.class != SuccessorRemoved
}

func (rule SuccessorRuleAssessment) CanonicalBytes() []byte {
	writer := newCanonicalWriter(successorRuleCanonicalDomain)
	writer.addString(rule.family.String())
	writer.addString(rule.key)
	writer.addString(rule.class.String())
	writer.addString(string(rule.ground))
	writer.addString(rule.before.String())
	writer.addString(rule.after.String())
	return writer.bytes()
}

func (rule SuccessorRuleAssessment) verify() error {
	if !rule.family.valid() || rule.key == "" {
		return fmt.Errorf("successor rule coordinate is invalid")
	}
	if rule.class.String() == "" || !rule.ground.valid() {
		return fmt.Errorf("successor rule classification is invalid")
	}
	hasBefore := rule.before.String() != ""
	hasAfter := rule.after.String() != ""
	switch rule.class {
	case SuccessorAdditive:
		if hasBefore || !hasAfter || rule.ground != GroundSemanticCoordinateAdded {
			return fmt.Errorf("additive successor rule has invalid basis")
		}
	case SuccessorRemoved:
		if !hasBefore || hasAfter || rule.ground != GroundSemanticCoordinateRemoved {
			return fmt.Errorf("removed successor rule has invalid basis")
		}
	case SuccessorUnchanged:
		if !hasBefore || !hasAfter || rule.before != rule.after || rule.ground != GroundExactSemanticMatch {
			return fmt.Errorf("unchanged successor rule has invalid basis")
		}
	default:
		if !hasBefore || !hasAfter || rule.before == rule.after {
			return fmt.Errorf("changed successor rule has invalid basis")
		}
	}
	return nil
}

// SuccessorDiff is a complete, immutable rule comparison between two exact
// executable TypeEnvs. It is separate from Diff: the older delta remains a
// compact changed-coordinate identity, while this value is the semantic
// transition review surface and includes unchanged coordinates.
type SuccessorDiff struct {
	base   typedmemory.TypeEnvRef
	target typedmemory.TypeEnvRef
	rules  []SuccessorRuleAssessment
	digest typedmemory.SHA256Digest
}

func CompareSuccessor(
	previous typedmemory.TypeEnv,
	current typedmemory.TypeEnv,
) (SuccessorDiff, error) {
	before, err := projectTypeEnv(previous)
	if err != nil {
		return SuccessorDiff{}, fmt.Errorf("project previous executable TypeEnv: %w", err)
	}
	after, err := projectTypeEnv(current)
	if err != nil {
		return SuccessorDiff{}, fmt.Errorf("project current executable TypeEnv: %w", err)
	}
	rules, err := compareSuccessorEntries(before, after)
	if err != nil {
		return SuccessorDiff{}, err
	}
	if previous.Ref() == current.Ref() && hasSuccessorSemanticChange(rules) {
		return SuccessorDiff{}, fmt.Errorf(
			"one TypeEnvRef cannot identify two executable semantic surfaces",
		)
	}
	diff := SuccessorDiff{
		base:   previous.Ref(),
		target: current.Ref(),
		rules:  rules,
	}
	digest, err := digestBytes(diff.CanonicalBytes())
	if err != nil {
		return SuccessorDiff{}, err
	}
	diff.digest = digest
	if err := diff.Verify(); err != nil {
		return SuccessorDiff{}, err
	}
	return diff, nil
}

func hasSuccessorSemanticChange(rules []SuccessorRuleAssessment) bool {
	for _, rule := range rules {
		if rule.class != SuccessorUnchanged {
			return true
		}
	}
	return false
}

func (diff SuccessorDiff) Base() typedmemory.TypeEnvRef { return diff.base }

func (diff SuccessorDiff) Target() typedmemory.TypeEnvRef { return diff.target }

func (diff SuccessorDiff) Rules() []SuccessorRuleAssessment {
	return append([]SuccessorRuleAssessment(nil), diff.rules...)
}

func (diff SuccessorDiff) Digest() typedmemory.SHA256Digest { return diff.digest }

func (diff SuccessorDiff) HasCompilerGap() bool {
	for _, rule := range diff.rules {
		if rule.class == SuccessorCompilerGap {
			return true
		}
	}
	return false
}

func (diff SuccessorDiff) CanonicalBytes() []byte {
	writer := newCanonicalWriter(successorDiffCanonicalDomain)
	writer.addString(diff.base.String())
	writer.addString(diff.target.String())
	writer.addUint64(uint64(len(diff.rules)))
	for _, rule := range diff.rules {
		writer.addBytes(rule.CanonicalBytes())
	}
	return writer.bytes()
}

func (diff SuccessorDiff) Verify() error {
	base, err := typedmemory.ParseTypeEnvRef(diff.base.String())
	if err != nil || base != diff.base {
		return fmt.Errorf("successor diff base is invalid")
	}
	target, err := typedmemory.ParseTypeEnvRef(diff.target.String())
	if err != nil || target != diff.target {
		return fmt.Errorf("successor diff target is invalid")
	}
	if len(diff.rules) == 0 || len(diff.rules) > maximumDiffChanges {
		return fmt.Errorf("successor diff rule count is invalid")
	}
	for index, rule := range diff.rules {
		if err := rule.verify(); err != nil {
			return fmt.Errorf("successor rule %d: %w", index, err)
		}
		decoded, err := decodeSuccessorRule(rule.CanonicalBytes())
		if err != nil {
			return fmt.Errorf("decode successor rule %d: %w", index, err)
		}
		if !bytes.Equal(decoded.CanonicalBytes(), rule.CanonicalBytes()) {
			return fmt.Errorf("successor rule %d is not exact", index)
		}
	}
	if err := verifySuccessorRuleOrder(diff.rules); err != nil {
		return err
	}
	canonical := diff.CanonicalBytes()
	if len(canonical) > maximumDiffCanonicalBytes {
		return fmt.Errorf("successor diff exceeds %d bytes", maximumDiffCanonicalBytes)
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return err
	}
	if digest != diff.digest {
		return fmt.Errorf("successor diff digest mismatch")
	}
	return nil
}

func DecodeSuccessorDiff(canonical []byte) (SuccessorDiff, error) {
	if len(canonical) == 0 || len(canonical) > maximumDiffCanonicalBytes {
		return SuccessorDiff{}, fmt.Errorf("successor diff byte length is invalid")
	}
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("successor diff domain")
	if err != nil {
		return SuccessorDiff{}, err
	}
	if domain != successorDiffCanonicalDomain {
		return SuccessorDiff{}, fmt.Errorf("successor diff domain is invalid")
	}
	baseText, err := reader.readString("successor base")
	if err != nil {
		return SuccessorDiff{}, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return SuccessorDiff{}, err
	}
	targetText, err := reader.readString("successor target")
	if err != nil {
		return SuccessorDiff{}, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetText)
	if err != nil {
		return SuccessorDiff{}, err
	}
	count, err := reader.readCount("successor rules", maximumDiffChanges)
	if err != nil || count == 0 {
		return SuccessorDiff{}, fmt.Errorf("successor rule count is invalid")
	}
	rules := make([]SuccessorRuleAssessment, 0, count)
	for index := 0; index < count; index++ {
		raw, readErr := reader.readBytes("successor rule")
		if readErr != nil {
			return SuccessorDiff{}, readErr
		}
		rule, decodeErr := decodeSuccessorRule(raw)
		if decodeErr != nil {
			return SuccessorDiff{}, decodeErr
		}
		rules = append(rules, rule)
	}
	if reader.remaining() != 0 {
		return SuccessorDiff{}, fmt.Errorf("successor diff has trailing bytes")
	}
	diff := SuccessorDiff{base: base, target: target, rules: rules}
	digest, err := digestBytes(diff.CanonicalBytes())
	if err != nil {
		return SuccessorDiff{}, err
	}
	diff.digest = digest
	if !bytes.Equal(diff.CanonicalBytes(), canonical) {
		return SuccessorDiff{}, fmt.Errorf("successor diff is not canonical")
	}
	if err := diff.Verify(); err != nil {
		return SuccessorDiff{}, err
	}
	return diff, nil
}

func decodeSuccessorRule(canonical []byte) (SuccessorRuleAssessment, error) {
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("successor rule domain")
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	if domain != successorRuleCanonicalDomain {
		return SuccessorRuleAssessment{}, fmt.Errorf("successor rule domain is invalid")
	}
	familyText, err := reader.readString("successor rule family")
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	family, err := parseFamily(familyText)
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	key, err := reader.readString("successor rule key")
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	classText, err := reader.readString("successor rule class")
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	class, err := parseSuccessorRuleClass(classText)
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	groundText, err := reader.readString("successor rule ground")
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	beforeText, err := reader.readString("successor rule before digest")
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	afterText, err := reader.readString("successor rule after digest")
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	before, err := optionalDigest(beforeText)
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	after, err := optionalDigest(afterText)
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	if reader.remaining() != 0 {
		return SuccessorRuleAssessment{}, fmt.Errorf("successor rule has trailing bytes")
	}
	rule := SuccessorRuleAssessment{
		family: family,
		key:    key,
		class:  class,
		ground: SuccessorRuleGround(groundText),
		before: before,
		after:  after,
	}
	if err := rule.verify(); err != nil {
		return SuccessorRuleAssessment{}, err
	}
	if !bytes.Equal(rule.CanonicalBytes(), canonical) {
		return SuccessorRuleAssessment{}, fmt.Errorf("successor rule is not canonical")
	}
	return rule, nil
}

func optionalDigest(raw string) (typedmemory.SHA256Digest, error) {
	if raw == "" {
		return typedmemory.SHA256Digest{}, nil
	}
	return typedmemory.NewSHA256Digest(raw)
}

func compareSuccessorEntries(
	before []semanticEntry,
	after []semanticEntry,
) ([]SuccessorRuleAssessment, error) {
	rules := make([]SuccessorRuleAssessment, 0, len(before)+len(after))
	beforeIndex := 0
	afterIndex := 0
	for beforeIndex < len(before) || afterIndex < len(after) {
		comparison := compareEntryPositions(before, beforeIndex, after, afterIndex)
		if comparison < 0 {
			entry := before[beforeIndex]
			rules = append(rules, removedSuccessorRule(entry))
			beforeIndex++
			continue
		}
		if comparison > 0 {
			entry := after[afterIndex]
			rules = append(rules, additiveSuccessorRule(entry))
			afterIndex++
			continue
		}
		left := before[beforeIndex]
		right := after[afterIndex]
		rule, err := classifyMatchedSuccessorRule(left, right)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
		beforeIndex++
		afterIndex++
	}
	return rules, nil
}

func additiveSuccessorRule(entry semanticEntry) SuccessorRuleAssessment {
	return SuccessorRuleAssessment{
		family: entry.family,
		key:    entry.key,
		class:  SuccessorAdditive,
		ground: GroundSemanticCoordinateAdded,
		after:  entry.fingerprint,
	}
}

func removedSuccessorRule(entry semanticEntry) SuccessorRuleAssessment {
	return SuccessorRuleAssessment{
		family: entry.family,
		key:    entry.key,
		class:  SuccessorRemoved,
		ground: GroundSemanticCoordinateRemoved,
		before: entry.fingerprint,
	}
}

func classifyMatchedSuccessorRule(
	before semanticEntry,
	after semanticEntry,
) (SuccessorRuleAssessment, error) {
	if before.fingerprint == after.fingerprint {
		return SuccessorRuleAssessment{
			family: before.family,
			key:    before.key,
			class:  SuccessorUnchanged,
			ground: GroundExactSemanticMatch,
			before: before.fingerprint,
			after:  after.fingerprint,
		}, nil
	}
	class, ground, err := classifyChangedSemanticRule(before, after)
	if err != nil {
		return SuccessorRuleAssessment{}, err
	}
	return SuccessorRuleAssessment{
		family: before.family,
		key:    before.key,
		class:  class,
		ground: ground,
		before: before.fingerprint,
		after:  after.fingerprint,
	}, nil
}

func classifyChangedSemanticRule(
	before semanticEntry,
	after semanticEntry,
) (SuccessorRuleClass, SuccessorRuleGround, error) {
	if before.family != after.family || before.key != after.key {
		return 0, "", fmt.Errorf("successor semantic coordinates disagree")
	}
	switch before.family {
	case EntitySetDefinitionFamily:
		return classifyEntitySetChange(before.material, after.material)
	case TypedRelationDeclarationFragmentFamily, legacyRelationSignatureFamily:
		return classifyTypedRelationDeclarationFragmentChange(
			before.material,
			after.material,
		)
	case RelationSlotFamily:
		return classifyRelationSlotChange(before.material, after.material)
	case ConstraintFamily:
		return classifyConstraintChange(before.material, after.material)
	case ValueBindingFamily, ValueShapeFamily:
		return SuccessorCompilerGap, GroundStoredValueMigrationRequired, nil
	default:
		return SuccessorCompilerGap, GroundSemanticOrderNotImplemented, nil
	}
}

type entitySetSemantic struct {
	context     string
	enumeration string
	policy      string
	evaluator   string
}

func classifyEntitySetChange(
	beforeRaw []byte,
	afterRaw []byte,
) (SuccessorRuleClass, SuccessorRuleGround, error) {
	before, err := decodeEntitySetSemantic(beforeRaw)
	if err != nil {
		return 0, "", err
	}
	after, err := decodeEntitySetSemantic(afterRaw)
	if err != nil {
		return 0, "", err
	}
	if before.context != after.context || before.enumeration != after.enumeration {
		return SuccessorCompilerGap, GroundEnumerationRuleChanged, nil
	}
	if before.policy == "persisted_entities_only" && after.policy == "prior_batch_declarations_visible" {
		return SuccessorWidened, GroundCandidateDomainExpanded, nil
	}
	if before.policy == "prior_batch_declarations_visible" && after.policy == "persisted_entities_only" {
		return SuccessorNarrowed, GroundCandidateDomainRestricted, nil
	}
	return SuccessorCompilerGap, GroundSemanticOrderNotImplemented, nil
}

func decodeEntitySetSemantic(raw []byte) (entitySetSemantic, error) {
	reader := newCanonicalReader(raw)
	domain, err := reader.readString("entity-set domain")
	if err != nil {
		return entitySetSemantic{}, err
	}
	if domain != "executable-typeenv.entity-set-definition.v1" {
		return entitySetSemantic{}, fmt.Errorf("entity-set semantic domain is invalid")
	}
	context, err := reader.readString("entity-set context")
	if err != nil {
		return entitySetSemantic{}, err
	}
	enumeration, err := reader.readString("entity-set enumeration")
	if err != nil {
		return entitySetSemantic{}, err
	}
	policy, err := reader.readString("entity-set candidate policy")
	if err != nil {
		return entitySetSemantic{}, err
	}
	evaluator := ""
	if policy == "prior_batch_declarations_visible" {
		evaluator, err = reader.readString("entity-set candidate evaluator")
		if err != nil {
			return entitySetSemantic{}, err
		}
	}
	if reader.remaining() != 0 {
		return entitySetSemantic{}, fmt.Errorf("entity-set semantic has trailing bytes")
	}
	return entitySetSemantic{
		context:     context,
		enumeration: enumeration,
		policy:      policy,
		evaluator:   evaluator,
	}, nil
}

func classifyTypedRelationDeclarationFragmentChange(
	beforeRaw []byte,
	afterRaw []byte,
) (SuccessorRuleClass, SuccessorRuleGround, error) {
	beforeID, beforeContexts, err := decodeRelationDeclarationSemantic(beforeRaw)
	if err != nil {
		return 0, "", err
	}
	afterID, afterContexts, err := decodeRelationDeclarationSemantic(afterRaw)
	if err != nil {
		return 0, "", err
	}
	if beforeID != afterID {
		return SuccessorCompilerGap, GroundSemanticOrderNotImplemented, nil
	}
	if equalStrings(beforeContexts, afterContexts) {
		return SuccessorUnchanged, GroundExactSemanticMatch, nil
	}
	return classifySetExpansion(
		beforeContexts,
		afterContexts,
		GroundContextDomainExpanded,
		GroundContextDomainRestricted,
		GroundContextDomainsIncomparable,
	)
}

func decodeRelationDeclarationSemantic(raw []byte) (string, []string, error) {
	reader := newCanonicalReader(raw)
	domain, err := reader.readString("relation declaration domain")
	if err != nil {
		return "", nil, err
	}
	legacy := domain == "executable-typeenv.relation-signature.v1"
	current := domain == "executable-typeenv.typed-relation-declaration-fragment.v1"
	if !legacy && !current {
		return "", nil, fmt.Errorf("relation declaration semantic domain is invalid")
	}
	id, err := reader.readString("relation declaration id")
	if err != nil {
		return "", nil, err
	}
	if current {
		posture, postureErr := reader.readString("relation declaration posture")
		if postureErr != nil {
			return "", nil, postureErr
		}
		if posture != typedmemory.RelationDeclarationTypedFragment.String() {
			return "", nil, fmt.Errorf("relation declaration posture is invalid")
		}
	}
	count, err := reader.readCount("relation declaration contexts", maximumDiffChanges)
	if err != nil {
		return "", nil, err
	}
	contexts := make([]string, 0, count)
	for index := 0; index < count; index++ {
		context, readErr := reader.readString("relation declaration context")
		if readErr != nil {
			return "", nil, readErr
		}
		contexts = append(contexts, context)
	}
	if reader.remaining() != 0 {
		return "", nil, fmt.Errorf("relation declaration semantic has trailing bytes")
	}
	sort.Strings(contexts)
	return id, contexts, nil
}

type cardinalitySemantic struct {
	minimum uint64
	maximum uint64
	bounded bool
}

type relationSlotSemantic struct {
	signature   string
	slot        string
	targetParts []string
	cardinality cardinalitySemantic
}

func classifyRelationSlotChange(
	beforeRaw []byte,
	afterRaw []byte,
) (SuccessorRuleClass, SuccessorRuleGround, error) {
	before, err := decodeRelationSlotSemantic(beforeRaw)
	if err != nil {
		return 0, "", err
	}
	after, err := decodeRelationSlotSemantic(afterRaw)
	if err != nil {
		return 0, "", err
	}
	if before.signature != after.signature || before.slot != after.slot || !equalStrings(before.targetParts, after.targetParts) {
		return SuccessorCompilerGap, GroundSlotTargetChanged, nil
	}
	return classifyCardinalityOrder(before.cardinality, after.cardinality)
}

func decodeRelationSlotSemantic(raw []byte) (relationSlotSemantic, error) {
	reader := newCanonicalReader(raw)
	domain, err := reader.readString("relation slot domain")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	if domain != "executable-typeenv.relation-slot.v1" {
		return relationSlotSemantic{}, fmt.Errorf("relation slot semantic domain is invalid")
	}
	signature, err := reader.readString("relation slot signature")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	slot, err := reader.readString("relation slot kind")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	mode, err := reader.readString("relation slot target mode")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	targetParts := []string{mode}
	valueKind, err := reader.readString("relation slot value kind")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	targetParts = append(targetParts, valueKind)
	if mode == "reference" {
		refKind, readErr := reader.readString("relation slot reference kind")
		if readErr != nil {
			return relationSlotSemantic{}, readErr
		}
		targetParts = append(targetParts, refKind)
	}
	minimum, err := readCanonicalUint64(reader, "relation slot minimum")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	boundedText, err := reader.readString("relation slot bounded posture")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	bounded, err := strconv.ParseBool(boundedText)
	if err != nil {
		return relationSlotSemantic{}, fmt.Errorf("relation slot bounded posture is invalid")
	}
	maximum, err := readCanonicalUint64(reader, "relation slot maximum")
	if err != nil {
		return relationSlotSemantic{}, err
	}
	if reader.remaining() != 0 {
		return relationSlotSemantic{}, fmt.Errorf("relation slot semantic has trailing bytes")
	}
	return relationSlotSemantic{
		signature:   signature,
		slot:        slot,
		targetParts: targetParts,
		cardinality: cardinalitySemantic{
			minimum: minimum,
			maximum: maximum,
			bounded: bounded,
		},
	}, nil
}

func classifyCardinalityOrder(
	before cardinalitySemantic,
	after cardinalitySemantic,
) (SuccessorRuleClass, SuccessorRuleGround, error) {
	beforeWithinAfter := after.minimum <= before.minimum && upperContains(after, before)
	afterWithinBefore := before.minimum <= after.minimum && upperContains(before, after)
	if beforeWithinAfter && !afterWithinBefore {
		return SuccessorWidened, GroundCardinalityDomainExpanded, nil
	}
	if afterWithinBefore && !beforeWithinAfter {
		return SuccessorNarrowed, GroundCardinalityDomainRestricted, nil
	}
	return SuccessorCompilerGap, GroundCardinalityDomainsIncomparable, nil
}

func upperContains(container, member cardinalitySemantic) bool {
	if !container.bounded {
		return true
	}
	if !member.bounded {
		return false
	}
	return container.maximum >= member.maximum
}

type constraintSemantic struct {
	variant     string
	coordinate  []string
	values      []string
	cardinality cardinalitySemantic
}

func classifyConstraintChange(
	beforeRaw []byte,
	afterRaw []byte,
) (SuccessorRuleClass, SuccessorRuleGround, error) {
	beforeVariant, err := decodeConstraintVariant(beforeRaw)
	if err != nil {
		return 0, "", err
	}
	afterVariant, err := decodeConstraintVariant(afterRaw)
	if err != nil {
		return 0, "", err
	}
	if beforeVariant != afterVariant {
		return SuccessorCompilerGap, GroundConstraintDomainsIncomparable, nil
	}
	if beforeVariant != "kind_disjoint" && beforeVariant != "slot_cardinality" {
		return SuccessorCompilerGap, GroundConstraintDomainsIncomparable, nil
	}
	before, err := decodeConstraintSemantic(beforeRaw)
	if err != nil {
		return 0, "", err
	}
	after, err := decodeConstraintSemantic(afterRaw)
	if err != nil {
		return 0, "", err
	}
	if before.variant != after.variant || !equalStrings(before.coordinate, after.coordinate) {
		return SuccessorCompilerGap, GroundConstraintDomainsIncomparable, nil
	}
	switch before.variant {
	case "kind_disjoint":
		beforeSubsetAfter := isStringSubset(before.values, after.values)
		afterSubsetBefore := isStringSubset(after.values, before.values)
		if beforeSubsetAfter && !afterSubsetBefore {
			return SuccessorNarrowed, GroundDisjointnessDomainRestricted, nil
		}
		if afterSubsetBefore && !beforeSubsetAfter {
			return SuccessorWidened, GroundDisjointnessDomainExpanded, nil
		}
		return SuccessorCompilerGap, GroundConstraintDomainsIncomparable, nil
	case "slot_cardinality":
		return classifyCardinalityOrder(before.cardinality, after.cardinality)
	default:
		return SuccessorCompilerGap, GroundConstraintDomainsIncomparable, nil
	}
}

func decodeConstraintVariant(raw []byte) (string, error) {
	reader := newCanonicalReader(raw)
	domain, err := reader.readString("constraint domain")
	if err != nil {
		return "", err
	}
	if domain != "executable-typeenv.constraint.v1" {
		return "", fmt.Errorf("constraint semantic domain is invalid")
	}
	if _, err := reader.readString("constraint id"); err != nil {
		return "", err
	}
	return reader.readString("constraint variant")
}

func decodeConstraintSemantic(raw []byte) (constraintSemantic, error) {
	reader := newCanonicalReader(raw)
	domain, err := reader.readString("constraint domain")
	if err != nil {
		return constraintSemantic{}, err
	}
	if domain != "executable-typeenv.constraint.v1" {
		return constraintSemantic{}, fmt.Errorf("constraint semantic domain is invalid")
	}
	id, err := reader.readString("constraint id")
	if err != nil {
		return constraintSemantic{}, err
	}
	variant, err := reader.readString("constraint variant")
	if err != nil {
		return constraintSemantic{}, err
	}
	result := constraintSemantic{variant: variant, coordinate: []string{id}}
	switch variant {
	case "kind_disjoint":
		count, readErr := reader.readCount("disjoint kinds", maximumDiffChanges)
		if readErr != nil {
			return constraintSemantic{}, readErr
		}
		for index := 0; index < count; index++ {
			kind, valueErr := reader.readString("disjoint kind")
			if valueErr != nil {
				return constraintSemantic{}, valueErr
			}
			result.values = append(result.values, kind)
		}
		sort.Strings(result.values)
	case "slot_cardinality":
		signature, readErr := reader.readString("constraint signature")
		if readErr != nil {
			return constraintSemantic{}, readErr
		}
		slot, readErr := reader.readString("constraint slot")
		if readErr != nil {
			return constraintSemantic{}, readErr
		}
		minimum, readErr := readCanonicalUint64(reader, "constraint minimum")
		if readErr != nil {
			return constraintSemantic{}, readErr
		}
		boundedText, readErr := reader.readString("constraint bounded posture")
		if readErr != nil {
			return constraintSemantic{}, readErr
		}
		bounded, parseErr := strconv.ParseBool(boundedText)
		if parseErr != nil {
			return constraintSemantic{}, parseErr
		}
		maximum, readErr := readCanonicalUint64(reader, "constraint maximum")
		if readErr != nil {
			return constraintSemantic{}, readErr
		}
		result.coordinate = append(result.coordinate, signature, slot)
		result.cardinality = cardinalitySemantic{
			minimum: minimum,
			maximum: maximum,
			bounded: bounded,
		}
	default:
		return constraintSemantic{variant: variant, coordinate: []string{id}}, nil
	}
	if reader.remaining() != 0 {
		return constraintSemantic{}, fmt.Errorf("constraint semantic has trailing bytes")
	}
	return result, nil
}

func classifySetExpansion(
	before []string,
	after []string,
	supersetGround SuccessorRuleGround,
	subsetGround SuccessorRuleGround,
	incomparableGround SuccessorRuleGround,
) (SuccessorRuleClass, SuccessorRuleGround, error) {
	beforeSubsetAfter := isStringSubset(before, after)
	afterSubsetBefore := isStringSubset(after, before)
	if beforeSubsetAfter && !afterSubsetBefore {
		return SuccessorWidened, supersetGround, nil
	}
	if afterSubsetBefore && !beforeSubsetAfter {
		return SuccessorNarrowed, subsetGround, nil
	}
	return SuccessorCompilerGap, incomparableGround, nil
}

func isStringSubset(left, right []string) bool {
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	for _, value := range left {
		if _, found := rightSet[value]; !found {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	return bytes.Equal([]byte(joinCanonicalStrings(left)), []byte(joinCanonicalStrings(right)))
}

func joinCanonicalStrings(values []string) string {
	writer := newCanonicalWriter("string-sequence.v1")
	for _, value := range values {
		writer.addString(value)
	}
	return string(writer.bytes())
}

func readCanonicalUint64(reader *canonicalReader, label string) (uint64, error) {
	raw, err := reader.readBytes(label)
	if err != nil {
		return 0, err
	}
	if len(raw) != 8 {
		return 0, fmt.Errorf("%s is not an encoded uint64", label)
	}
	return bytesToUint64(raw), nil
}

func bytesToUint64(raw []byte) uint64 {
	var result uint64
	for _, value := range raw {
		result = result<<8 | uint64(value)
	}
	return result
}

func verifySuccessorRuleOrder(rules []SuccessorRuleAssessment) error {
	for index := 1; index < len(rules); index++ {
		left := semanticEntry{family: rules[index-1].family, key: rules[index-1].key}
		right := semanticEntry{family: rules[index].family, key: rules[index].key}
		if compareSemanticEntryCoordinates(left, right) >= 0 {
			return fmt.Errorf("successor rules are not canonically ordered")
		}
	}
	return nil
}
