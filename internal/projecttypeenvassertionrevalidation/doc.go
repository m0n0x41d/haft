// Package projecttypeenvassertionrevalidation revalidates the exact active
// assertion set from one immutable project-graph observation under one target
// executable TypeEnv and one exact target runtime registry.
//
// The package is a pure semantic core. It performs no storage reads, Stage or
// head selection, authority resolution, or graph mutation. Its report digest
// is derived internally from the target/runtime/graph coordinates and every
// per-assertion result.
package projecttypeenvassertionrevalidation
