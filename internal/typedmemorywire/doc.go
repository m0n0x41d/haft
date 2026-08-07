// Package typedmemorywire decodes the read-only Haft typed-memory validation
// request. It owns only the protocol boundary: strict JSON, resource limits,
// closed discriminators, and lowering into a sealed unbound candidate.
//
// A request basis is an untrusted selector. The bundled open-world candidate
// is not project-active and cannot authorize persistence or establish a
// project-level Valid verdict. The package neither loads project state nor
// claims that a requested TypeEnv digest or graph revision is active.
// The outer validation service resolves the selector, supplies the resulting
// TypeEnvRef to BindChangeSet, and then invokes typedmemory's pure semantic
// validator. This package performs no I/O and exposes no admission operation.
package typedmemorywire
