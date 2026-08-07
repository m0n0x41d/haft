// Package memberofruntime composes the store-facing MemberOf evaluators that
// are callable in one process.
//
// A registration binds an exact RuleRef to an engine and to a separately
// supplied MechanismIdentity. This is process composition, not byte
// attestation: the registry does not prove that the Go value implements the
// artifact named by that identity. Project TypeEnv/runtime admission remains
// responsible for verifying the selected X pins before this dispatcher is
// installed.
package memberofruntime
