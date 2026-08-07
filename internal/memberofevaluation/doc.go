// Package memberofevaluation owns the neutral immutable input and capability
// contracts shared by store-facing MemberOf evaluators.
//
// These values describe an exact evaluation input. They do not attest that a
// caller obtained the input from a transaction-correlated store read; that
// authority belongs to the outer storage shell that constructs them.
package memberofevaluation
