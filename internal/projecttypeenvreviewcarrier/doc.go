// Package projecttypeenvreviewcarrier owns the filesystem boundary for the
// sealed, human-readable ProjectTypeEnv Genesis review carrier.
//
// Install and Replace serialize every package writer with an identity-checked
// advisory lock. Replace checks the exact reviewed digest immediately before
// the atomic rename. Arbitrary processes that bypass this boundary can mutate
// ordinary project files without honoring advisory locks; detected identity
// changes fail closed, but hostile filesystem isolation is not this package's
// contract.
//
// A failure after Linkat or Renameat is an OutcomeUnknown, not a bare error.
// Retrying the recorded operation with the exact same proposal and, for
// Replace, its original expected digest converges to Reused when the namespace
// effect occurred. Mutations also reconcile a bounded set of reserved,
// non-canonical stage files left by an interrupted prior attempt.
package projecttypeenvreviewcarrier
