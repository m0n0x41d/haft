// Package projecttypeenvruntime compares one exact project TypeEnv runtime
// basis X with the immutable registries configured in the current process.
//
// The package is deliberately narrower than TypeEnv compilation and Stage
// revalidation. X declares exact mechanism and registration-policy identities;
// this package checks that callable codec/evaluator registries and the
// configured registration policy resolve those exact identities. It neither
// attests executable bytes nor selects a project TypeEnv head.
//
// A future head-selection effect must invoke ObserveCurrentTargetRuntime from
// its own package-owned process configuration. It must not accept a prebuilt
// ExactTargetRuntimeRegistry from CLI, MCP, JSON, SQLite, or another caller as
// proof of current runtime configuration.
package projecttypeenvruntime
