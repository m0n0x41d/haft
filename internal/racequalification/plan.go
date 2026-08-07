package racequalification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// PlanSchema identifies the machine-readable plan projection.
	PlanSchema = "haft.race-qualification.plan/v1"

	// ConsolidatedP13SkipPattern is the only deliberate skip in the ordinary
	// race-qualification contour.
	ConsolidatedP13SkipPattern = "^TestP13ConsolidatedAcceptance$"
)

// PackageID identifies one Go package in the discovered test closure.
type PackageID string

// NewPackageID constructs a validated package identity.
func NewPackageID(raw string) (PackageID, error) {
	id := PackageID(raw)
	if err := validatePackageID(id); err != nil {
		return "", err
	}
	return id, nil
}

// String returns the exact package identity.
func (id PackageID) String() string {
	return string(id)
}

// TopLevelTestID identifies one platform-visible top-level Test, Example, or
// Fuzz target. Subtests and fuzz seed cases are deliberately not representable
// as independent work.
type TopLevelTestID string

// NewTopLevelTestID constructs a validated top-level Test, Example, or Fuzz
// target identity.
func NewTopLevelTestID(raw string) (TopLevelTestID, error) {
	id := TopLevelTestID(raw)
	if err := validateTopLevelTestID(id); err != nil {
		return "", err
	}
	return id, nil
}

// String returns the exact top-level test identity.
func (id TopLevelTestID) String() string {
	return string(id)
}

// PlanDigest identifies the exact canonical plan projection.
type PlanDigest string

// String returns the sha256-prefixed digest.
func (digest PlanDigest) String() string {
	return string(digest)
}

// SplitPackageDiscovery contains the top-level tests that may be partitioned
// independently for one package.
type SplitPackageDiscovery struct {
	Package       PackageID        `json:"package_id"`
	TopLevelTests []TopLevelTestID `json:"top_level_tests"`
}

// Discovery is the platform/build-tag-aware work discovered by an imperative
// adapter. WholePackages execute as indivisible units. SplitPackages execute
// each top-level Test, Example, or Fuzz target exactly once.
type Discovery struct {
	WholePackages []PackageID             `json:"whole_packages"`
	SplitPackages []SplitPackageDiscovery `json:"split_packages"`
}

// WorkKind identifies the execution granularity of one assignment.
type WorkKind string

const (
	// WholePackageWork executes every ordinary test in one package.
	WholePackageWork WorkKind = "whole_package"

	// SplitTopLevelTestWork executes one exact top-level Test, Example, or Fuzz
	// target while retaining all subtests and seed cases under the same parent.
	SplitTopLevelTestWork WorkKind = "split_top_level_test"
)

// WorkItem is one indivisible race-qualification assignment.
type WorkItem struct {
	Kind         WorkKind       `json:"kind"`
	Package      PackageID      `json:"package_id"`
	TopLevelTest TopLevelTestID `json:"top_level_test_id,omitempty"`
}

// Shard is one explicit partition member. Work is always a JSON array; an
// empty array is a valid no-work shard, while an absent shard is invalid.
type Shard struct {
	Index int        `json:"index"`
	Work  []WorkItem `json:"work"`
}

// Plan is the deterministic machine-readable projection returned by Plan().
// Call Validate after decoding a plan from JSON.
type Plan struct {
	Schema       string     `json:"schema"`
	PlanDigest   PlanDigest `json:"plan_digest"`
	ShardCount   int        `json:"shard_count"`
	SkipPatterns []string   `json:"skip_patterns"`
	Discovery    Discovery  `json:"discovery"`
	Shards       []Shard    `json:"shards"`
}

// RaceQualificationPlan is a validated immutable plan. Its accessors return
// defensive projections suitable for command and workflow adapters.
type RaceQualificationPlan struct {
	projection Plan
}

// Build canonicalizes discovered work, assigns it deterministically, and
// returns a validated immutable plan.
func Build(discovery Discovery, shardCount int) (RaceQualificationPlan, error) {
	if shardCount <= 0 {
		return RaceQualificationPlan{}, fmt.Errorf("shard count must be positive")
	}

	canonicalDiscovery, work, err := canonicalizeDiscovery(discovery)
	if err != nil {
		return RaceQualificationPlan{}, err
	}

	shards := make([]Shard, shardCount)
	for index := range shards {
		shards[index] = Shard{
			Index: index,
			Work:  []WorkItem{},
		}
	}
	for index, item := range work {
		shardIndex := index % shardCount
		shards[shardIndex].Work = append(shards[shardIndex].Work, item)
	}

	projection := Plan{
		Schema:       PlanSchema,
		ShardCount:   shardCount,
		SkipPatterns: []string{ConsolidatedP13SkipPattern},
		Discovery:    canonicalDiscovery,
		Shards:       shards,
	}
	if err := validateStructure(projection); err != nil {
		return RaceQualificationPlan{}, err
	}
	digest, err := digestPlan(projection)
	if err != nil {
		return RaceQualificationPlan{}, err
	}
	projection.PlanDigest = digest
	if err := Validate(projection); err != nil {
		return RaceQualificationPlan{}, err
	}

	return RaceQualificationPlan{projection: projection}, nil
}

// Plan returns a defensive machine-readable projection.
func (plan RaceQualificationPlan) Plan() Plan {
	return clonePlan(plan.projection)
}

// Shard returns one defensive shard projection.
func (plan RaceQualificationPlan) Shard(index int) (Shard, error) {
	if index < 0 || index >= plan.projection.ShardCount {
		return Shard{}, fmt.Errorf(
			"shard index %d is outside [0,%d)",
			index,
			plan.projection.ShardCount,
		)
	}
	return cloneShard(plan.projection.Shards[index]), nil
}

// Digest returns the exact plan digest.
func (plan RaceQualificationPlan) Digest() PlanDigest {
	return plan.projection.PlanDigest
}

// MarshalJSON emits the same deterministic projection as Plan().
func (plan RaceQualificationPlan) MarshalJSON() ([]byte, error) {
	if err := Validate(plan.projection); err != nil {
		return nil, err
	}
	return json.Marshal(plan.projection)
}

// Validate proves that a decoded projection is canonical, complete, unique,
// deterministically assigned, and bound to its declared digest.
func Validate(plan Plan) error {
	if err := validateStructure(plan); err != nil {
		return err
	}
	digest, err := digestPlan(plan)
	if err != nil {
		return err
	}
	if plan.PlanDigest == "" {
		return fmt.Errorf("plan digest is required")
	}
	if plan.PlanDigest != digest {
		return fmt.Errorf(
			"plan digest mismatch: got %q, want %q",
			plan.PlanDigest,
			digest,
		)
	}
	return nil
}

func validateStructure(plan Plan) error {
	if plan.Schema != PlanSchema {
		return fmt.Errorf("plan schema must be %q", PlanSchema)
	}
	if plan.ShardCount <= 0 {
		return fmt.Errorf("shard count must be positive")
	}
	if len(plan.SkipPatterns) != 1 ||
		plan.SkipPatterns[0] != ConsolidatedP13SkipPattern {
		return fmt.Errorf(
			"skip policy must contain only %q",
			ConsolidatedP13SkipPattern,
		)
	}
	if plan.Discovery.WholePackages == nil {
		return fmt.Errorf("discovery whole_packages must be an explicit array")
	}
	if plan.Discovery.SplitPackages == nil {
		return fmt.Errorf("discovery split_packages must be an explicit array")
	}

	canonicalDiscovery, expectedWork, err := canonicalizeDiscovery(plan.Discovery)
	if err != nil {
		return err
	}
	if !equalDiscovery(plan.Discovery, canonicalDiscovery) {
		return fmt.Errorf("discovery is not canonically sorted")
	}
	if len(plan.Shards) != plan.ShardCount {
		return fmt.Errorf(
			"plan has %d shards, want %d explicit shards",
			len(plan.Shards),
			plan.ShardCount,
		)
	}

	expectedByKey := make(map[string]WorkItem, len(expectedWork))
	expectedShardByKey := make(map[string]int, len(expectedWork))
	for index, item := range expectedWork {
		key := workItemKey(item)
		expectedByKey[key] = item
		expectedShardByKey[key] = index % plan.ShardCount
	}

	observedByKey := make(map[string]int, len(expectedWork))
	for position, shard := range plan.Shards {
		if shard.Index < 0 || shard.Index >= plan.ShardCount {
			return fmt.Errorf(
				"shard index %d is outside [0,%d)",
				shard.Index,
				plan.ShardCount,
			)
		}
		if shard.Index != position {
			return fmt.Errorf(
				"shards must be in canonical index order: position %d has index %d",
				position,
				shard.Index,
			)
		}
		if shard.Work == nil {
			return fmt.Errorf(
				"shard %d must explicitly carry an empty work array",
				shard.Index,
			)
		}
		if !slices.IsSortedFunc(shard.Work, compareWorkItems) {
			return fmt.Errorf("shard %d work is not canonically sorted", shard.Index)
		}
		for _, item := range shard.Work {
			if err := validateWorkItem(item); err != nil {
				return fmt.Errorf("shard %d: %w", shard.Index, err)
			}
			key := workItemKey(item)
			if _, exists := observedByKey[key]; exists {
				return fmt.Errorf("work item %q is assigned more than once", key)
			}
			expected, exists := expectedByKey[key]
			if !exists || expected != item {
				return fmt.Errorf("shard %d contains unexpected work item %q", shard.Index, key)
			}
			expectedShard := expectedShardByKey[key]
			if shard.Index != expectedShard {
				return fmt.Errorf(
					"work item %q is assigned to shard %d, want deterministic shard %d",
					key,
					shard.Index,
					expectedShard,
				)
			}
			observedByKey[key] = shard.Index
		}
	}

	for _, item := range expectedWork {
		key := workItemKey(item)
		if _, exists := observedByKey[key]; !exists {
			return fmt.Errorf("work item %q is missing from the shard partition", key)
		}
	}
	return nil
}

func canonicalizeDiscovery(input Discovery) (Discovery, []WorkItem, error) {
	wholePackages := append([]PackageID{}, input.WholePackages...)
	slices.Sort(wholePackages)
	for index, packageID := range wholePackages {
		if err := validatePackageID(packageID); err != nil {
			return Discovery{}, nil, fmt.Errorf("whole package %d: %w", index, err)
		}
		if index > 0 && wholePackages[index-1] == packageID {
			return Discovery{}, nil, fmt.Errorf(
				"whole package %q is discovered more than once",
				packageID,
			)
		}
	}

	splitPackages := make([]SplitPackageDiscovery, len(input.SplitPackages))
	for index, split := range input.SplitPackages {
		if err := validatePackageID(split.Package); err != nil {
			return Discovery{}, nil, fmt.Errorf("split package %d: %w", index, err)
		}
		if len(split.TopLevelTests) == 0 {
			return Discovery{}, nil, fmt.Errorf(
				"split package %q must contain at least one top-level Test, Example, or Fuzz target",
				split.Package,
			)
		}
		tests := append([]TopLevelTestID{}, split.TopLevelTests...)
		slices.Sort(tests)
		for testIndex, testID := range tests {
			if err := validateTopLevelTestID(testID); err != nil {
				return Discovery{}, nil, fmt.Errorf(
					"split package %q test %d: %w",
					split.Package,
					testIndex,
					err,
				)
			}
			if testIndex > 0 && tests[testIndex-1] == testID {
				return Discovery{}, nil, fmt.Errorf(
					"split package %q repeats top-level test %q",
					split.Package,
					testID,
				)
			}
		}
		splitPackages[index] = SplitPackageDiscovery{
			Package:       split.Package,
			TopLevelTests: tests,
		}
	}
	slices.SortFunc(splitPackages, func(left, right SplitPackageDiscovery) int {
		return strings.Compare(left.Package.String(), right.Package.String())
	})
	for index, split := range splitPackages {
		if index > 0 && splitPackages[index-1].Package == split.Package {
			return Discovery{}, nil, fmt.Errorf(
				"split package %q is discovered more than once",
				split.Package,
			)
		}
	}

	packageKinds := make(map[PackageID]WorkKind, len(wholePackages)+len(splitPackages))
	for _, packageID := range wholePackages {
		packageKinds[packageID] = WholePackageWork
	}
	for _, split := range splitPackages {
		if prior, exists := packageKinds[split.Package]; exists {
			return Discovery{}, nil, fmt.Errorf(
				"package %q is discovered as both %q and %q",
				split.Package,
				prior,
				SplitTopLevelTestWork,
			)
		}
		packageKinds[split.Package] = SplitTopLevelTestWork
	}
	if len(packageKinds) == 0 {
		return Discovery{}, nil, fmt.Errorf("discovery must contain at least one package")
	}

	canonical := Discovery{
		WholePackages: wholePackages,
		SplitPackages: splitPackages,
	}
	work := make([]WorkItem, 0, len(wholePackages))
	for _, packageID := range wholePackages {
		work = append(work, WorkItem{
			Kind:    WholePackageWork,
			Package: packageID,
		})
	}
	for _, split := range splitPackages {
		for _, testID := range split.TopLevelTests {
			work = append(work, WorkItem{
				Kind:         SplitTopLevelTestWork,
				Package:      split.Package,
				TopLevelTest: testID,
			})
		}
	}
	slices.SortFunc(work, compareWorkItems)
	return canonical, work, nil
}

func validatePackageID(id PackageID) error {
	if err := validateIdentifier("package ID", id.String()); err != nil {
		return err
	}
	return nil
}

func validateTopLevelTestID(id TopLevelTestID) error {
	raw := id.String()
	if err := validateIdentifier("top-level test ID", raw); err != nil {
		return err
	}
	if strings.Contains(raw, "/") {
		return fmt.Errorf(
			"top-level test ID %q contains '/'; subtests remain with their parent",
			raw,
		)
	}
	if !strings.HasPrefix(raw, "Test") &&
		!strings.HasPrefix(raw, "Example") &&
		!strings.HasPrefix(raw, "Fuzz") {
		return fmt.Errorf(
			"top-level test ID %q must name a Test, Example, or Fuzz target",
			raw,
		)
	}
	return nil
}

func validateIdentifier(label string, raw string) error {
	if raw == "" {
		return fmt.Errorf("%s must be non-empty", label)
	}
	if !utf8.ValidString(raw) {
		return fmt.Errorf("%s contains invalid UTF-8", label)
	}
	for _, character := range raw {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return fmt.Errorf("%s %q contains whitespace or control characters", label, raw)
		}
	}
	return nil
}

func validateWorkItem(item WorkItem) error {
	if err := validatePackageID(item.Package); err != nil {
		return err
	}
	switch item.Kind {
	case WholePackageWork:
		if item.TopLevelTest != "" {
			return fmt.Errorf(
				"whole package %q must not name a top-level test",
				item.Package,
			)
		}
	case SplitTopLevelTestWork:
		if err := validateTopLevelTestID(item.TopLevelTest); err != nil {
			return err
		}
	default:
		return fmt.Errorf("work item for package %q has unknown kind %q", item.Package, item.Kind)
	}
	return nil
}

func compareWorkItems(left, right WorkItem) int {
	if compared := strings.Compare(left.Package.String(), right.Package.String()); compared != 0 {
		return compared
	}
	if compared := strings.Compare(string(left.Kind), string(right.Kind)); compared != 0 {
		return compared
	}
	return strings.Compare(left.TopLevelTest.String(), right.TopLevelTest.String())
}

func workItemKey(item WorkItem) string {
	return string(item.Kind) + "\x00" +
		item.Package.String() + "\x00" +
		item.TopLevelTest.String()
}

func equalDiscovery(left, right Discovery) bool {
	if !slices.Equal(left.WholePackages, right.WholePackages) ||
		len(left.SplitPackages) != len(right.SplitPackages) {
		return false
	}
	for index := range left.SplitPackages {
		if left.SplitPackages[index].Package != right.SplitPackages[index].Package ||
			!slices.Equal(
				left.SplitPackages[index].TopLevelTests,
				right.SplitPackages[index].TopLevelTests,
			) {
			return false
		}
	}
	return true
}

func clonePlan(plan Plan) Plan {
	cloned := plan
	cloned.SkipPatterns = append([]string{}, plan.SkipPatterns...)
	cloned.Discovery = cloneDiscovery(plan.Discovery)
	cloned.Shards = make([]Shard, len(plan.Shards))
	for index, shard := range plan.Shards {
		cloned.Shards[index] = cloneShard(shard)
	}
	return cloned
}

func cloneDiscovery(discovery Discovery) Discovery {
	cloned := Discovery{
		WholePackages: append([]PackageID{}, discovery.WholePackages...),
		SplitPackages: make([]SplitPackageDiscovery, len(discovery.SplitPackages)),
	}
	for index, split := range discovery.SplitPackages {
		cloned.SplitPackages[index] = SplitPackageDiscovery{
			Package: split.Package,
			TopLevelTests: append(
				[]TopLevelTestID{},
				split.TopLevelTests...,
			),
		}
	}
	return cloned
}

func cloneShard(shard Shard) Shard {
	return Shard{
		Index: shard.Index,
		Work:  append([]WorkItem{}, shard.Work...),
	}
}

type planDigestProjection struct {
	Schema       string    `json:"schema"`
	ShardCount   int       `json:"shard_count"`
	SkipPatterns []string  `json:"skip_patterns"`
	Discovery    Discovery `json:"discovery"`
	Shards       []Shard   `json:"shards"`
}

func digestPlan(plan Plan) (PlanDigest, error) {
	encoded, err := json.Marshal(planDigestProjection{
		Schema:       plan.Schema,
		ShardCount:   plan.ShardCount,
		SkipPatterns: plan.SkipPatterns,
		Discovery:    plan.Discovery,
		Shards:       plan.Shards,
	})
	if err != nil {
		return "", fmt.Errorf("encode race qualification plan digest input: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return PlanDigest("sha256:" + hex.EncodeToString(digest[:])), nil
}
