// Package recordmembershipcandidate is the compatibility facade for the
// lower-layer canonical registration contract now owned by
// recordmembershipregistration.
//
// The package deliberately cannot bind the registration into X, mint a trusted
// record-membership source delivery, activate a TypeEnv, or produce a MemberOf
// judgement. It closes only the pure data contract and accepted
// mapping-manifest/adapter policy needed before those effects can be designed.
package recordmembershipcandidate
