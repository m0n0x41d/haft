// Package projecttypeenvassertionreport owns the pure, immutable canonical
// algebra for one complete existing-active-assertion revalidation result.
//
// It intentionally knows neither project TypeEnv selection nor graph
// observation orchestration. Those upper layers lower their exact inputs into
// this package's graph coordinate and strongly validated grounds. The package
// performs no IO, storage, Stage/head mutation, or authority work.
package projecttypeenvassertionreport
