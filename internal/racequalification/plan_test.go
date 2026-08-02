package racequalification

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestIdentifiersRejectEmptyWhitespaceAndSubtestNames(t *testing.T) {
	t.Parallel()

	packageID, err := NewPackageID("github.com/m0n0x41d/haft/internal/cli")
	if err != nil {
		t.Fatal(err)
	}
	if packageID.String() != "github.com/m0n0x41d/haft/internal/cli" {
		t.Fatalf("package ID = %q", packageID)
	}

	for _, invalid := range []string{
		"",
		" ",
		"bad package",
		"bad\npackage",
		string([]byte{'b', 'a', 'd', 0xff}),
	} {
		if _, err := NewPackageID(invalid); err == nil {
			t.Fatalf("NewPackageID(%q) accepted invalid identity", invalid)
		}
	}

	for _, valid := range []string{
		"TestPublicInit",
		"Example",
		"ExampleRaceQualification_plan",
		"FuzzRoundTrip",
	} {
		testID, err := NewTopLevelTestID(valid)
		if err != nil {
			t.Fatalf("NewTopLevelTestID(%q): %v", valid, err)
		}
		if testID.String() != valid {
			t.Fatalf("top-level test ID = %q, want %q", testID, valid)
		}
	}
	for _, invalid := range []string{
		"",
		" ",
		"TestParent/subtest",
		"ExampleParent/subcase",
		"BenchmarkOnly",
		"TestBad\nName",
		string([]byte{'T', 'e', 's', 't', 0xff}),
	} {
		if _, err := NewTopLevelTestID(invalid); err == nil {
			t.Fatalf("NewTopLevelTestID(%q) accepted invalid identity", invalid)
		}
	}
}

func TestValidateRejectsNullDiscoveryArraysAsNonCanonicalJSON(t *testing.T) {
	t.Parallel()

	fixtures := []struct {
		name      string
		discovery Discovery
		mutate    func(*Plan)
		want      string
	}{
		{
			name: "null whole packages",
			discovery: Discovery{
				WholePackages: []PackageID{},
				SplitPackages: []SplitPackageDiscovery{{
					Package: mustPackageID(
						t,
						"github.com/m0n0x41d/haft/internal/split",
					),
					TopLevelTests: []TopLevelTestID{
						mustTopLevelTestID(t, "TestSplit"),
					},
				}},
			},
			mutate: func(plan *Plan) {
				plan.Discovery.WholePackages = nil
			},
			want: "whole_packages must be an explicit array",
		},
		{
			name: "null split packages",
			discovery: Discovery{
				WholePackages: []PackageID{
					mustPackageID(t, "github.com/m0n0x41d/haft/internal/whole"),
				},
				SplitPackages: []SplitPackageDiscovery{},
			},
			mutate: func(plan *Plan) {
				plan.Discovery.SplitPackages = nil
			},
			want: "split_packages must be an explicit array",
		},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			built, err := Build(fixture.discovery, 2)
			if err != nil {
				t.Fatal(err)
			}
			projection := built.Plan()
			fixture.mutate(&projection)
			projection.PlanDigest, err = digestPlan(projection)
			if err != nil {
				t.Fatal(err)
			}
			err = Validate(projection)
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("error = %v, want containing %q", err, fixture.want)
			}
		})
	}
}

func TestBuildIsCanonicalDeterministicAndInputOrderIndependent(t *testing.T) {
	t.Parallel()

	first, err := Build(sampleDiscovery(t), 12)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(shuffledSampleDiscovery(t), 12)
	if err != nil {
		t.Fatal(err)
	}

	firstProjection := first.Plan()
	secondProjection := second.Plan()
	if !plansEqual(firstProjection, secondProjection) {
		t.Fatalf(
			"input ordering changed plan\nfirst:  %#v\nsecond: %#v",
			firstProjection,
			secondProjection,
		)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf(
			"input ordering changed digest: %q != %q",
			first.Digest(),
			second.Digest(),
		)
	}

	wantWhole := []PackageID{
		mustPackageID(t, "github.com/m0n0x41d/haft/internal/alpha"),
		mustPackageID(t, "github.com/m0n0x41d/haft/internal/middle"),
		mustPackageID(t, "github.com/m0n0x41d/haft/internal/zeta"),
	}
	if !slices.Equal(firstProjection.Discovery.WholePackages, wantWhole) {
		t.Fatalf(
			"whole packages = %#v, want %#v",
			firstProjection.Discovery.WholePackages,
			wantWhole,
		)
	}
	if got := firstProjection.Discovery.SplitPackages; len(got) != 2 ||
		got[0].Package.String() != "github.com/m0n0x41d/haft/internal/cli" ||
		got[1].Package.String() != "github.com/m0n0x41d/haft/internal/store" ||
		!slices.IsSorted(got[0].TopLevelTests) ||
		!slices.IsSorted(got[1].TopLevelTests) {
		t.Fatalf("split discovery is not canonical: %#v", got)
	}
	if len(firstProjection.SkipPatterns) != 1 ||
		firstProjection.SkipPatterns[0] != ConsolidatedP13SkipPattern {
		t.Fatalf("skip policy = %#v", firstProjection.SkipPatterns)
	}
	if err := Validate(firstProjection); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestBuildPartitionsEveryWorkItemExactlyOnceAcrossShardCounts(t *testing.T) {
	t.Parallel()

	for _, shardCount := range []int{1, 2, 12} {
		shardCount := shardCount
		t.Run(string(rune('A'+shardCount)), func(t *testing.T) {
			t.Parallel()

			built, err := Build(sampleDiscovery(t), shardCount)
			if err != nil {
				t.Fatal(err)
			}
			projection := built.Plan()
			if len(projection.Shards) != shardCount {
				t.Fatalf("shards = %d, want %d", len(projection.Shards), shardCount)
			}

			observed := map[string]int{}
			for index, shard := range projection.Shards {
				if shard.Index != index {
					t.Fatalf("shard position %d has index %d", index, shard.Index)
				}
				if shard.Work == nil {
					t.Fatalf("shard %d work is nil", shard.Index)
				}
				for _, item := range shard.Work {
					observed[workItemKey(item)]++
				}
			}

			expected := expectedSampleWorkKeys(t)
			if len(observed) != len(expected) {
				t.Fatalf("observed %d work items, want %d", len(observed), len(expected))
			}
			for _, key := range expected {
				if observed[key] != 1 {
					t.Fatalf("work item %q occurs %d times, want exactly once", key, observed[key])
				}
			}
			if err := Validate(projection); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func TestBuildRepresentsEmptyShardsAndJSONArraysExplicitly(t *testing.T) {
	t.Parallel()

	built, err := Build(
		Discovery{
			WholePackages: []PackageID{
				mustPackageID(t, "github.com/m0n0x41d/haft/internal/only"),
			},
			SplitPackages: []SplitPackageDiscovery{},
		},
		4,
	)
	if err != nil {
		t.Fatal(err)
	}
	projection := built.Plan()
	for index := 1; index < 4; index++ {
		if projection.Shards[index].Work == nil ||
			len(projection.Shards[index].Work) != 0 {
			t.Fatalf(
				"empty shard %d work = %#v, want explicit empty array",
				index,
				projection.Shards[index].Work,
			)
		}
	}

	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"whole_packages":["github.com/m0n0x41d/haft/internal/only"]`,
		`"split_packages":[]`,
		`"index":1,"work":[]`,
		`"index":2,"work":[]`,
		`"index":3,"work":[]`,
	} {
		if !bytes.Contains(encoded, []byte(want)) {
			t.Fatalf("plan JSON missing %s: %s", want, encoded)
		}
	}
}

func TestBuildRejectsInvalidDiscoveryBeforeAssignment(t *testing.T) {
	t.Parallel()

	validPackage := mustPackageID(t, "github.com/m0n0x41d/haft/internal/valid")
	validTest := mustTopLevelTestID(t, "TestValid")
	fixtures := []struct {
		name       string
		discovery  Discovery
		shardCount int
		want       string
	}{
		{
			name:       "zero shard count",
			discovery:  Discovery{WholePackages: []PackageID{validPackage}},
			shardCount: 0,
			want:       "positive",
		},
		{
			name:       "negative shard count",
			discovery:  Discovery{WholePackages: []PackageID{validPackage}},
			shardCount: -1,
			want:       "positive",
		},
		{
			name:       "empty discovery",
			discovery:  Discovery{},
			shardCount: 1,
			want:       "at least one package",
		},
		{
			name: "empty whole package ID",
			discovery: Discovery{
				WholePackages: []PackageID{""},
			},
			shardCount: 1,
			want:       "non-empty",
		},
		{
			name: "duplicate whole package",
			discovery: Discovery{
				WholePackages: []PackageID{validPackage, validPackage},
			},
			shardCount: 1,
			want:       "more than once",
		},
		{
			name: "empty split package ID",
			discovery: Discovery{
				SplitPackages: []SplitPackageDiscovery{{
					Package:       "",
					TopLevelTests: []TopLevelTestID{validTest},
				}},
			},
			shardCount: 1,
			want:       "non-empty",
		},
		{
			name: "split package without tests",
			discovery: Discovery{
				SplitPackages: []SplitPackageDiscovery{{
					Package:       validPackage,
					TopLevelTests: []TopLevelTestID{},
				}},
			},
			shardCount: 1,
			want:       "at least one top-level",
		},
		{
			name: "split package with empty test",
			discovery: Discovery{
				SplitPackages: []SplitPackageDiscovery{{
					Package:       validPackage,
					TopLevelTests: []TopLevelTestID{""},
				}},
			},
			shardCount: 1,
			want:       "non-empty",
		},
		{
			name: "split package with subtest",
			discovery: Discovery{
				SplitPackages: []SplitPackageDiscovery{{
					Package:       validPackage,
					TopLevelTests: []TopLevelTestID{"TestParent/subtest"},
				}},
			},
			shardCount: 1,
			want:       "subtests remain with their parent",
		},
		{
			name: "split package repeats test",
			discovery: Discovery{
				SplitPackages: []SplitPackageDiscovery{{
					Package:       validPackage,
					TopLevelTests: []TopLevelTestID{validTest, validTest},
				}},
			},
			shardCount: 1,
			want:       "repeats top-level test",
		},
		{
			name: "split package repeats",
			discovery: Discovery{
				SplitPackages: []SplitPackageDiscovery{
					{Package: validPackage, TopLevelTests: []TopLevelTestID{validTest}},
					{Package: validPackage, TopLevelTests: []TopLevelTestID{"ExampleValid"}},
				},
			},
			shardCount: 1,
			want:       "discovered more than once",
		},
		{
			name: "package is both whole and split",
			discovery: Discovery{
				WholePackages: []PackageID{validPackage},
				SplitPackages: []SplitPackageDiscovery{{
					Package:       validPackage,
					TopLevelTests: []TopLevelTestID{validTest},
				}},
			},
			shardCount: 1,
			want:       "both",
		},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			_, err := Build(fixture.discovery, fixture.shardCount)
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("error = %v, want containing %q", err, fixture.want)
			}
		})
	}
}

func TestValidateRejectsCorruptIncompleteAndNonCanonicalPlans(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 4)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		name   string
		mutate func(*Plan)
		want   string
	}{
		{
			name:   "schema",
			mutate: func(plan *Plan) { plan.Schema = "future" },
			want:   "schema",
		},
		{
			name:   "zero shard count",
			mutate: func(plan *Plan) { plan.ShardCount = 0 },
			want:   "positive",
		},
		{
			name: "additional skip",
			mutate: func(plan *Plan) {
				plan.SkipPatterns = append(plan.SkipPatterns, "^TestOther$")
			},
			want: "skip policy",
		},
		{
			name: "missing skip",
			mutate: func(plan *Plan) {
				plan.SkipPatterns = []string{}
			},
			want: "skip policy",
		},
		{
			name: "unsorted discovery",
			mutate: func(plan *Plan) {
				plan.Discovery.WholePackages[0], plan.Discovery.WholePackages[1] =
					plan.Discovery.WholePackages[1], plan.Discovery.WholePackages[0]
			},
			want: "canonically sorted",
		},
		{
			name: "split package without tests",
			mutate: func(plan *Plan) {
				plan.Discovery.SplitPackages[0].TopLevelTests = []TopLevelTestID{}
			},
			want: "at least one top-level",
		},
		{
			name: "missing shard",
			mutate: func(plan *Plan) {
				plan.Shards = plan.Shards[:len(plan.Shards)-1]
			},
			want: "explicit shards",
		},
		{
			name: "duplicate shard index",
			mutate: func(plan *Plan) {
				plan.Shards[1].Index = 0
			},
			want: "canonical index order",
		},
		{
			name: "out of range shard index",
			mutate: func(plan *Plan) {
				plan.Shards[1].Index = plan.ShardCount
			},
			want: "outside",
		},
		{
			name: "missing work item",
			mutate: func(plan *Plan) {
				source := firstNonEmptyShard(plan.Shards)
				plan.Shards[source].Work = plan.Shards[source].Work[1:]
			},
			want: "missing",
		},
		{
			name: "duplicate work item",
			mutate: func(plan *Plan) {
				source := firstNonEmptyShard(plan.Shards)
				item := plan.Shards[source].Work[0]
				plan.Shards[source].Work = append(plan.Shards[source].Work, item)
				slices.SortFunc(plan.Shards[source].Work, compareWorkItems)
			},
			want: "more than once",
		},
		{
			name: "wrong deterministic shard",
			mutate: func(plan *Plan) {
				source := firstNonEmptyShard(plan.Shards)
				item := plan.Shards[source].Work[0]
				plan.Shards[source].Work = plan.Shards[source].Work[1:]
				target := (source + 1) % plan.ShardCount
				plan.Shards[target].Work = append(plan.Shards[target].Work, item)
				slices.SortFunc(plan.Shards[target].Work, compareWorkItems)
			},
			want: "deterministic shard",
		},
		{
			name: "empty package in work item",
			mutate: func(plan *Plan) {
				source := firstNonEmptyShard(plan.Shards)
				plan.Shards[source].Work[0].Package = ""
			},
			want: "non-empty",
		},
		{
			name: "subtest in work item",
			mutate: func(plan *Plan) {
				shardIndex, workIndex := firstSplitWork(plan.Shards)
				plan.Shards[shardIndex].Work[workIndex].TopLevelTest =
					"TestParent/subtest"
			},
			want: "subtests remain with their parent",
		},
		{
			name: "unknown work kind",
			mutate: func(plan *Plan) {
				source := firstNonEmptyShard(plan.Shards)
				plan.Shards[source].Work[0].Kind = "future"
			},
			want: "unknown kind",
		},
		{
			name: "digest mismatch",
			mutate: func(plan *Plan) {
				plan.PlanDigest = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			},
			want: "digest mismatch",
		},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			projection := built.Plan()
			fixture.mutate(&projection)
			err := Validate(projection)
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("error = %v, want containing %q", err, fixture.want)
			}
		})
	}
}

func TestValidateRejectsNilWorkInsteadOfTreatingItAsExplicitEmptyShard(t *testing.T) {
	t.Parallel()

	built, err := Build(
		Discovery{
			WholePackages: []PackageID{
				mustPackageID(t, "github.com/m0n0x41d/haft/internal/only"),
			},
			SplitPackages: []SplitPackageDiscovery{},
		},
		3,
	)
	if err != nil {
		t.Fatal(err)
	}
	projection := built.Plan()
	projection.Shards[2].Work = nil
	err = Validate(projection)
	if err == nil || !strings.Contains(err.Error(), "explicitly carry an empty work array") {
		t.Fatalf("error = %v, want explicit empty work rejection", err)
	}
}

func TestPlanShardAndDiscoveryAccessorsAreDefensive(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 2)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := built.Digest()

	projection := built.Plan()
	projection.SkipPatterns[0] = "^TestChanged$"
	projection.Discovery.WholePackages[0] = "changed"
	projection.Discovery.SplitPackages[0].TopLevelTests[0] = "TestChanged"
	projection.Shards[0].Work[0].Package = "changed"

	shard, err := built.Shard(0)
	if err != nil {
		t.Fatal(err)
	}
	shard.Work[0].Package = "changed-again"

	fresh := built.Plan()
	if fresh.PlanDigest != originalDigest ||
		fresh.SkipPatterns[0] != ConsolidatedP13SkipPattern ||
		fresh.Discovery.WholePackages[0] == "changed" ||
		fresh.Discovery.SplitPackages[0].TopLevelTests[0] == "TestChanged" ||
		fresh.Shards[0].Work[0].Package == "changed" ||
		fresh.Shards[0].Work[0].Package == "changed-again" {
		t.Fatalf("external mutation changed immutable plan: %#v", fresh)
	}

	for _, invalid := range []int{-1, fresh.ShardCount} {
		if _, err := built.Shard(invalid); err == nil {
			t.Fatalf("Shard(%d) accepted invalid index", invalid)
		}
	}
}

func TestJSONProjectionRoundTripsDeterministicallyAndBindsDigest(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 12)
	if err != nil {
		t.Fatal(err)
	}
	encodedPlan, err := json.Marshal(built.Plan())
	if err != nil {
		t.Fatal(err)
	}
	encodedCapability, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodedPlan, encodedCapability) {
		t.Fatalf(
			"capability JSON differs from Plan projection\nplan: %s\ncap:  %s",
			encodedPlan,
			encodedCapability,
		)
	}

	var decoded Plan
	if err := json.Unmarshal(encodedPlan, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := Validate(decoded); err != nil {
		t.Fatalf("round-trip Validate: %v", err)
	}
	if decoded.PlanDigest != built.Digest() ||
		!strings.HasPrefix(decoded.PlanDigest.String(), "sha256:") ||
		len(decoded.PlanDigest.String()) != len("sha256:")+64 {
		t.Fatalf("round-trip digest = %q", decoded.PlanDigest)
	}

	reencoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodedPlan, reencoded) {
		t.Fatalf("JSON projection is not stable\nfirst:  %s\nsecond: %s", encodedPlan, reencoded)
	}

	otherCount, err := Build(sampleDiscovery(t), 2)
	if err != nil {
		t.Fatal(err)
	}
	if otherCount.Digest() == built.Digest() {
		t.Fatal("different shard count retained the same plan digest")
	}
}

func sampleDiscovery(t *testing.T) Discovery {
	t.Helper()
	return Discovery{
		WholePackages: []PackageID{
			mustPackageID(t, "github.com/m0n0x41d/haft/internal/zeta"),
			mustPackageID(t, "github.com/m0n0x41d/haft/internal/alpha"),
			mustPackageID(t, "github.com/m0n0x41d/haft/internal/middle"),
		},
		SplitPackages: []SplitPackageDiscovery{
			{
				Package: mustPackageID(
					t,
					"github.com/m0n0x41d/haft/internal/store",
				),
				TopLevelTests: []TopLevelTestID{
					mustTopLevelTestID(t, "TestStoreZulu"),
					mustTopLevelTestID(t, "TestStoreAlpha"),
					mustTopLevelTestID(t, "ExampleStore"),
				},
			},
			{
				Package: mustPackageID(
					t,
					"github.com/m0n0x41d/haft/internal/cli",
				),
				TopLevelTests: []TopLevelTestID{
					mustTopLevelTestID(t, "TestPublicInit"),
					mustTopLevelTestID(t, "ExampleCLI"),
					mustTopLevelTestID(t, "TestRunSpec"),
					mustTopLevelTestID(t, "TestHostStatus"),
					mustTopLevelTestID(t, "TestOnboard"),
					mustTopLevelTestID(t, "TestServe"),
					mustTopLevelTestID(t, "TestDoctor"),
				},
			},
		},
	}
}

func shuffledSampleDiscovery(t *testing.T) Discovery {
	t.Helper()
	discovery := sampleDiscovery(t)
	slices.Reverse(discovery.WholePackages)
	slices.Reverse(discovery.SplitPackages)
	for index := range discovery.SplitPackages {
		slices.Reverse(discovery.SplitPackages[index].TopLevelTests)
	}
	return discovery
}

func expectedSampleWorkKeys(t *testing.T) []string {
	t.Helper()
	discovery := sampleDiscovery(t)
	keys := make([]string, 0, 13)
	for _, packageID := range discovery.WholePackages {
		keys = append(keys, workItemKey(WorkItem{
			Kind:    WholePackageWork,
			Package: packageID,
		}))
	}
	for _, split := range discovery.SplitPackages {
		for _, testID := range split.TopLevelTests {
			keys = append(keys, workItemKey(WorkItem{
				Kind:         SplitTopLevelTestWork,
				Package:      split.Package,
				TopLevelTest: testID,
			}))
		}
	}
	return keys
}

func mustPackageID(t *testing.T, raw string) PackageID {
	t.Helper()
	id, err := NewPackageID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustTopLevelTestID(t *testing.T, raw string) TopLevelTestID {
	t.Helper()
	id, err := NewTopLevelTestID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func firstNonEmptyShard(shards []Shard) int {
	for index, shard := range shards {
		if len(shard.Work) > 0 {
			return index
		}
	}
	panic("test plan has no non-empty shard")
}

func firstSplitWork(shards []Shard) (int, int) {
	for shardIndex, shard := range shards {
		for workIndex, item := range shard.Work {
			if item.Kind == SplitTopLevelTestWork {
				return shardIndex, workIndex
			}
		}
	}
	panic("test plan has no split work")
}

func plansEqual(left, right Plan) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		panic(err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		panic(err)
	}
	return bytes.Equal(leftJSON, rightJSON)
}
