// Package projecttypeenvstore persists immutable, content-addressed project
// TypeEnv artifacts.
//
// The package stores the exact canonical B, E, X, and C bytes. Its artifact
// closure proves recipe identity and resolved X pins, not successful final
// lowering into an executable TypeEnv. It owns no project Stage, active head,
// authority receipt, or mutable "current" row. Every read replays the owning
// artifact decoder before returning a value.
package projecttypeenvstore
