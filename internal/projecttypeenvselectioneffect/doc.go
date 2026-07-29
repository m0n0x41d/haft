// Package projecttypeenvselectioneffect owns the pure, immutable records for
// one ProjectTypeEnv-head selection effect.
//
// The package performs no SQLite work and cannot establish that a transaction
// committed. Its canonical values describe the exact effect that an outer
// transaction shell may write atomically.
//
// A ProjectTypeEnvHeadCASWorkRecord is a Haft-local authority/effect carrier.
// It describes the real CAS occurrence through A.15.1-shaped coordinates, but
// it deliberately makes no project-graph U.Work membership claim. Such
// membership requires a separate typed-memory admission under the current
// project TypeEnv.
package projecttypeenvselectioneffect
