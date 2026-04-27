package implementationplan

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type ID string

type Revision string

type DecisionRef string

type DependencyEdge struct {
	DecisionRef DecisionRef
	DependsOn   DecisionRef
}

type Lockset []string

type DecisionNode struct {
	Ref       DecisionRef
	DependsOn []DecisionRef
	Lockset   Lockset
}

type Plan struct {
	ID        ID
	Revision  Revision
	Decisions []DecisionNode
}

func ParsePayload(payload map[string]any) (Plan, error) {
	id, err := requiredID(payload)
	if err != nil {
		return Plan{}, err
	}

	revision, err := requiredRevision(payload)
	if err != nil {
		return Plan{}, err
	}

	decisions, err := decisionNodes(payload)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		ID:        id,
		Revision:  revision,
		Decisions: decisions,
	}
	if err := plan.ValidateDAG(); err != nil {
		return Plan{}, err
	}

	return plan, nil
}

func (plan Plan) DecisionRefs() []DecisionRef {
	refs := make([]DecisionRef, 0, len(plan.Decisions))
	for _, decision := range plan.Decisions {
		refs = append(refs, decision.Ref)
	}
	return refs
}

func (plan Plan) DependencyEdges() []DependencyEdge {
	edges := make([]DependencyEdge, 0)
	for _, decision := range plan.Decisions {
		for _, dependency := range decision.DependsOn {
			edges = append(edges, DependencyEdge{
				DecisionRef: decision.Ref,
				DependsOn:   dependency,
			})
		}
	}
	return edges
}

func (plan Plan) ValidateDAG() error {
	dependenciesByRef, err := plan.dependenciesByRef()
	if err != nil {
		return err
	}

	if err := validateKnownDependencies(dependenciesByRef); err != nil {
		return err
	}

	return validateAcyclicDependencies(dependenciesByRef)
}

func DependenciesSatisfied(dependencyIDs []string, satisfiedByID map[string]bool) bool {
	dependencies := cleanStringSlice(dependencyIDs)
	for _, dependencyID := range dependencies {
		if !satisfiedByID[dependencyID] {
			return false
		}
	}
	return true
}

func NormalizeLockset(values []string) Lockset {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		lockPath := normalizeLockPath(value)
		if lockPath == "" {
			continue
		}
		cleaned = append(cleaned, lockPath)
	}

	sort.Strings(cleaned)
	return Lockset(uniqueSortedStrings(cleaned))
}

func LocksetsOverlap(left []string, right []string) bool {
	leftLockset := NormalizeLockset(left)
	rightLockset := NormalizeLockset(right)

	return leftLockset.ConflictsWith(rightLockset)
}

func (lockset Lockset) ConflictsWith(other Lockset) bool {
	for _, leftPath := range lockset {
		for _, rightPath := range other {
			if lockPathsOverlap(leftPath, rightPath) {
				return true
			}
		}
	}
	return false
}

func requiredID(payload map[string]any) (ID, error) {
	value := stringField(payload, "id")
	if value == "" {
		return "", fmt.Errorf("plan.id is required")
	}
	return ID(value), nil
}

func requiredRevision(payload map[string]any) (Revision, error) {
	value := stringField(payload, "revision")
	if value == "" {
		return "", fmt.Errorf("plan.revision is required")
	}
	return Revision(value), nil
}

func decisionNodes(payload map[string]any) ([]DecisionNode, error) {
	raw, ok := payload["decisions"].([]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("plan.decisions is required")
	}

	decisions := make([]DecisionNode, 0, len(raw))
	for index, value := range raw {
		decision, err := decisionNode(value, index)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}

	return decisions, nil
}

func decisionNode(value any, index int) (DecisionNode, error) {
	switch entry := value.(type) {
	case string:
		ref, err := requiredDecisionRef(entry, fmt.Sprintf("plan.decisions[%d]", index))
		if err != nil {
			return DecisionNode{}, err
		}
		return DecisionNode{Ref: ref}, nil
	case map[string]any:
		return decisionNodeFromMap(entry, index)
	default:
		return DecisionNode{}, fmt.Errorf("plan.decisions[%d] must be a decision ref string or object", index)
	}
}

func decisionNodeFromMap(entry map[string]any, index int) (DecisionNode, error) {
	ref, err := decisionRefField(entry, index)
	if err != nil {
		return DecisionNode{}, err
	}

	dependencies, err := decisionRefSliceField(entry, "depends_on", fmt.Sprintf("plan.decisions[%d].depends_on", index))
	if err != nil {
		return DecisionNode{}, err
	}

	lockset, err := locksetField(entry, "lockset", fmt.Sprintf("plan.decisions[%d].lockset", index))
	if err != nil {
		return DecisionNode{}, err
	}

	return DecisionNode{
		Ref:       ref,
		DependsOn: dependencies,
		Lockset:   lockset,
	}, nil
}

func decisionRefField(entry map[string]any, index int) (DecisionRef, error) {
	ref := stringField(entry, "ref")
	alternate := stringField(entry, "decision_ref")
	if ref != "" && alternate != "" && ref != alternate {
		return "", fmt.Errorf("plan.decisions[%d].ref and decision_ref disagree", index)
	}
	if ref == "" {
		ref = alternate
	}
	return requiredDecisionRef(ref, fmt.Sprintf("plan.decisions[%d].ref", index))
}

func requiredDecisionRef(value string, label string) (DecisionRef, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return DecisionRef(cleaned), nil
}

func decisionRefSliceField(entry map[string]any, key string, label string) ([]DecisionRef, error) {
	values, err := stringSliceField(entry, key, label)
	if err != nil {
		return nil, err
	}

	refs := make([]DecisionRef, 0, len(values))
	for _, value := range values {
		refs = append(refs, DecisionRef(value))
	}
	return uniqueDecisionRefs(refs), nil
}

func locksetField(entry map[string]any, key string, label string) (Lockset, error) {
	values, err := stringSliceField(entry, key, label)
	if err != nil {
		return nil, err
	}
	return NormalizeLockset(values), nil
}

func stringSliceField(entry map[string]any, key string, label string) ([]string, error) {
	value, ok := entry[key]
	if !ok {
		return nil, nil
	}

	switch typed := value.(type) {
	case []string:
		return cleanStringSlice(typed), nil
	case []any:
		return anyStringSlice(typed, label)
	default:
		return nil, fmt.Errorf("%s must be an array of strings", label)
	}
}

func anyStringSlice(values []any, label string) ([]string, error) {
	result := make([]string, 0, len(values))
	for index, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a string", label, index)
		}

		cleaned := strings.TrimSpace(text)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result, nil
}

func (plan Plan) dependenciesByRef() (map[DecisionRef][]DecisionRef, error) {
	dependenciesByRef := make(map[DecisionRef][]DecisionRef, len(plan.Decisions))
	for _, decision := range plan.Decisions {
		if _, exists := dependenciesByRef[decision.Ref]; exists {
			return nil, fmt.Errorf("plan decision %s is duplicated", decision.Ref)
		}
		dependenciesByRef[decision.Ref] = uniqueDecisionRefs(decision.DependsOn)
	}
	return dependenciesByRef, nil
}

func validateKnownDependencies(dependenciesByRef map[DecisionRef][]DecisionRef) error {
	for ref, dependencies := range dependenciesByRef {
		for _, dependency := range dependencies {
			if dependency == ref {
				return fmt.Errorf("plan decision %s depends on itself", ref)
			}
			if _, exists := dependenciesByRef[dependency]; !exists {
				return fmt.Errorf("plan decision %s depends on unknown decision %s", ref, dependency)
			}
		}
	}
	return nil
}

func validateAcyclicDependencies(dependenciesByRef map[DecisionRef][]DecisionRef) error {
	visiting := map[DecisionRef]bool{}
	visited := map[DecisionRef]bool{}

	for ref := range dependenciesByRef {
		if err := detectCycle(ref, dependenciesByRef, visiting, visited); err != nil {
			return err
		}
	}
	return nil
}

func detectCycle(
	ref DecisionRef,
	dependenciesByRef map[DecisionRef][]DecisionRef,
	visiting map[DecisionRef]bool,
	visited map[DecisionRef]bool,
) error {
	if visited[ref] {
		return nil
	}
	if visiting[ref] {
		return fmt.Errorf("plan decision dependency cycle includes %s", ref)
	}

	visiting[ref] = true
	for _, dependency := range dependenciesByRef[ref] {
		if err := detectCycle(dependency, dependenciesByRef, visiting, visited); err != nil {
			return err
		}
	}
	delete(visiting, ref)
	visited[ref] = true

	return nil
}

func uniqueDecisionRefs(values []DecisionRef) []DecisionRef {
	seen := make(map[DecisionRef]struct{}, len(values))
	result := make([]DecisionRef, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func cleanStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func uniqueSortedStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}

	unique := values[:1]
	for _, value := range values[1:] {
		if value != unique[len(unique)-1] {
			unique = append(unique, value)
		}
	}
	return unique
}

func lockPathsOverlap(left string, right string) bool {
	left = normalizeLockPath(left)
	right = normalizeLockPath(right)

	switch {
	case left == "" || right == "":
		return false
	case left == "**/*" || right == "**/*":
		return true
	case left == "*" || right == "*":
		return true
	case left == right:
		return true
	case lockPrefixContains(left, right):
		return true
	case lockPrefixContains(right, left):
		return true
	}

	leftMatchesRight, _ := path.Match(left, right)
	rightMatchesLeft, _ := path.Match(right, left)
	return leftMatchesRight || rightMatchesLeft
}

func lockPrefixContains(pattern string, candidate string) bool {
	prefix, ok := lockPrefix(pattern)
	if !ok {
		return false
	}
	return candidate == prefix || strings.HasPrefix(candidate, prefix+"/")
}

func lockPrefix(pattern string) (string, bool) {
	switch {
	case strings.HasSuffix(pattern, "/**/*"):
		return strings.TrimSuffix(pattern, "/**/*"), true
	case strings.HasSuffix(pattern, "/**"):
		return strings.TrimSuffix(pattern, "/**"), true
	default:
		return "", false
	}
}

func normalizeLockPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	value = strings.TrimPrefix(value, "./")
	if value == "" {
		return ""
	}

	return path.Clean(value)
}
