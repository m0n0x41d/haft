// Package projecttypeenvheadstore owns the narrow durable projection of the
// current project TypeEnv head and its append-only immutable semantic states.
//
// The package does not authorize, select, or commit a project TypeEnv. Its
// mutation primitives require a caller-owned BEGIN IMMEDIATE transaction,
// compare exact prior state, write only the head projection, and leave
// transaction finish and the wider authority/Work/receipt closure to the P8G
// effect shell.
package projecttypeenvheadstore
