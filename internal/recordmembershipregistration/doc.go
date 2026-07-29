// Package recordmembershipregistration owns the lower-layer, content-addressed
// registration-policy carrier shared by project-memory adapters and TypeEnv X.
//
// Its legacy "candidate" schema/ref tokens are intentionally preserved for
// byte/ref parity with the already-built pre-release carrier. Renaming that
// identity requires an explicit migration decision; X1 consumes the exact
// existing bytes instead of silently creating a successor identity.
//
// Registration identifies declared evaluator, source-delivery boundary, and
// accepted mapping policy. It does not attest executable code, activate a
// TypeEnv, trust a delivered source, or produce a membership judgement.
package recordmembershipregistration
