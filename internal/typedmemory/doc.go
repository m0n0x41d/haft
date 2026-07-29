// Package typedmemory defines Haft's pure, effect-free typed project-memory
// algebra. It deliberately contains no source parser, storage adapter,
// transport schema, artifact store, authority gate, or graph projection.
//
// The package models instance changes against an already active TypeEnv.
// Project schema proposals and activations are a separate algebra and cannot be
// smuggled through MemoryChangeSet.
package typedmemory
