// Package typedmemorycandidatecodec implements the immutable candidate codecs
// declared by the Haft typed-memory local-practice candidate carrier.
//
// The package deliberately does not register a runtime mechanism, activate a
// TypeEnv, or assign FPF Core semantics to its Haft-local values. A caller must
// first bind a Suite to the exact content-derived ValueShape declarations and
// may later use the resulting pure mechanisms as input to a separately
// authorized runtime-catalog and ProjectTypeEnv selection process.
package typedmemorycandidatecodec
