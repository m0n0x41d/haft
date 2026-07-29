// Package authority contains two deliberately distinct layers during the v9
// migration: a reusable manual-terminal SpeechAct source kernel and a legacy
// profile-declaration authority adapter. PreparedSpeechActIntent,
// VerifiedSpeechActSource, and SpeechActSourceWriter are the reusable seam.
// PreparedAuthorityIntent, VerifiedAuthorityAct, AuthorityBasisWriter, and the
// profile admission gate are profile-specific compatibility code, not the
// authorization interface for decisions or commissions.
//
// VerifiedSpeechActSourceV2 is the additive, pure reliance-bearing source
// description. It keeps SpeechActRef, WorkRef, DescriptionRef, observable
// content carriers, and the terminal-capture carrier distinct while adding the
// resource-ledger, outcome, acceptance, and audit anchors required of the
// communicative Work record. Its strict decoder can rehydrate canonical v2
// material against an exact verified or recorded v1 source basis. There is no
// v2 persistence or downgrade API in this package: the v1 speech_acts table
// remains read-only compatible, and a consuming protocol must add a distinct
// durable v2 store before relying on persistence.
//
// The remainder of this note describes the legacy receipt/profile layer.
//
// EvaluateReceipt and EvaluateReceiptWithHostVerifiers are legacy orientation
// helpers. ReceiptStatusValid is not a permission, an admitted use, or a token
// convertible to the new gate result. The P0PA KernelGate loads only immutable
// canonical SQLite presentations and AuthorityResolutionRecords. It
// intentionally provides no production writer: missing trusted presentation
// provenance fails closed.
//
// Resolve is read-only and does not reserve or consume a single-use key. A
// later project-profile admission transaction must re-load and revalidate the
// canonical records, insert authority_uses, apply the expected-revision profile
// CAS, and persist the admission record in one transaction. It must never trust
// a previous Resolve result as effect authority.
package authority
