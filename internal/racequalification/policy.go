package racequalification

const (
	// CurrentCommandGOMAXPROCS keeps each race-instrumented test process from
	// inheriting the host-wide CPU count while several processes run together.
	CurrentCommandGOMAXPROCS = 2

	// CurrentSubtestParallelism bounds t.Parallel work inside each package
	// process while independent shard processes consume the machine-wide
	// concurrency budget.
	CurrentSubtestParallelism = 2
)

// CurrentSplitPackagePaths returns the packages whose top-level tests are
// independently partitioned by the complete race qualification. The list is
// the measured packages where partitioning wins despite per-process package
// setup. Every child subtest remains attached to its top-level parent.
func CurrentSplitPackagePaths() []string {
	return []string{
		"github.com/m0n0x41d/haft/db",
		"github.com/m0n0x41d/haft/internal/cli",
		"github.com/m0n0x41d/haft/internal/fpf",
		"github.com/m0n0x41d/haft/internal/fpf/typeenv",
		"github.com/m0n0x41d/haft/internal/fpfrefresh",
		"github.com/m0n0x41d/haft/internal/projectledgermigration",
		"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite",
		"github.com/m0n0x41d/haft/internal/typedmemorystore",
	}
}

// CurrentSplitPackageExecutionPriority returns the exact tail-first execution
// priority for the current split-package set. Discovery remains canonically
// sorted through CurrentSplitPackagePaths; this separate policy only controls
// which already-planned split commands are dispatched first.
func CurrentSplitPackageExecutionPriority() []string {
	return []string{
		"github.com/m0n0x41d/haft/internal/fpfrefresh",
		"github.com/m0n0x41d/haft/internal/cli",
		"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite",
		"github.com/m0n0x41d/haft/db",
		"github.com/m0n0x41d/haft/internal/fpf",
		"github.com/m0n0x41d/haft/internal/typedmemorystore",
		"github.com/m0n0x41d/haft/internal/fpf/typeenv",
		"github.com/m0n0x41d/haft/internal/projectledgermigration",
	}
}
