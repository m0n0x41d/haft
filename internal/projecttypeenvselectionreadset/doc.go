// Package projecttypeenvselectionreadset owns the transaction-local,
// read-only Genesis head-observation boundary.
//
// It turns an exact dedicated-head absence observed through one active
// BEGIN IMMEDIATE capability into an opaque in-process read set and privately
// issues its immutable content-addressed NoPriorHeadProofRecord. The audit
// record is inspectable and serializable; only the read set retains the
// transaction capability. Neither value is a Stage-currentness, profile,
// assertion-revalidation, authority, Work, CAS, graph-mutation, or selection
// capability.
package projecttypeenvselectionreadset
