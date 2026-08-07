package racequalification

import (
	"slices"
	"testing"
)

func TestCurrentSplitPackagePathsAreCanonicalAndDefensive(t *testing.T) {
	t.Parallel()

	if CurrentSubtestParallelism != 2 {
		t.Fatalf(
			"subtest parallelism = %d, want bounded concurrency 2",
			CurrentSubtestParallelism,
		)
	}
	if CurrentCommandGOMAXPROCS != 2 {
		t.Fatalf(
			"command GOMAXPROCS = %d, want bounded concurrency 2",
			CurrentCommandGOMAXPROCS,
		)
	}
	want := []string{
		"github.com/m0n0x41d/haft/db",
		"github.com/m0n0x41d/haft/internal/cli",
		"github.com/m0n0x41d/haft/internal/fpf",
		"github.com/m0n0x41d/haft/internal/fpf/typeenv",
		"github.com/m0n0x41d/haft/internal/fpfrefresh",
		"github.com/m0n0x41d/haft/internal/projectledgermigration",
		"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite",
		"github.com/m0n0x41d/haft/internal/typedmemorystore",
	}
	first := CurrentSplitPackagePaths()
	if !slices.Equal(first, want) {
		t.Fatalf("split packages = %#v, want exact ordered policy %#v", first, want)
	}
	first[0] = "changed"
	second := CurrentSplitPackagePaths()
	if !slices.Equal(second, want) {
		t.Fatalf(
			"split packages after caller mutation = %#v, want defensive copy %#v",
			second,
			want,
		)
	}
}

func TestCurrentSplitPackageExecutionPriorityIsExactDefensivePermutation(
	t *testing.T,
) {
	t.Parallel()

	want := []string{
		"github.com/m0n0x41d/haft/internal/fpfrefresh",
		"github.com/m0n0x41d/haft/internal/cli",
		"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite",
		"github.com/m0n0x41d/haft/db",
		"github.com/m0n0x41d/haft/internal/fpf",
		"github.com/m0n0x41d/haft/internal/typedmemorystore",
		"github.com/m0n0x41d/haft/internal/fpf/typeenv",
		"github.com/m0n0x41d/haft/internal/projectledgermigration",
	}
	first := CurrentSplitPackageExecutionPriority()
	if !slices.Equal(first, want) {
		t.Fatalf("split execution priority = %#v, want %#v", first, want)
	}

	canonicalPriority := append([]string{}, first...)
	slices.Sort(canonicalPriority)
	if !slices.Equal(canonicalPriority, CurrentSplitPackagePaths()) {
		t.Fatalf(
			"split execution priority is not an exact permutation of the split-package set: %#v",
			first,
		)
	}

	first[0] = "changed"
	if second := CurrentSplitPackageExecutionPriority(); !slices.Equal(second, want) {
		t.Fatalf(
			"split execution priority after caller mutation = %#v, want defensive copy %#v",
			second,
			want,
		)
	}
}
