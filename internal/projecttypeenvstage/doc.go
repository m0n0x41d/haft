// Package projecttypeenvstage persists immutable project TypeEnv Stage records
// and restores selection-ready in-process capabilities by replaying the exact
// final-lowering proof from the artifact store.
//
// The package owns no active head, selection, authorization, or mutable
// "current" pointer. Plain reads return data-only Stage and verification
// records. A selection-ready read is a stronger operation: it reloads the
// exact B/E/X/C closure, resolves the runtime-mechanism catalogs bound by X,
// reruns final lowering, and byte-compares the newly minted verification
// receipt with the persisted receipt.
package projecttypeenvstage
