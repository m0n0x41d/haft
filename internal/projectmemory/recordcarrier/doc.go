// Package recordcarrier defines the pure carrier-first observation used to
// establish project-record membership. It deliberately has no persistence,
// evaluator registration, semantic admission, activation, or authority
// operation.
//
// A ProjectRecordCarrierV1 is a classification/identity carrier. It is not the
// ProjectRecord itself, proof that the record's claims are true, or evidence of
// approval or binding authority. In particular, a DecisionRecord carrier
// variant never authorizes or performs a decision.
//
// Sealing these inert values is not membership authority. A later composition
// boundary must accept observables only from its trusted adapter, immutable
// store, and exact evaluator registration. Candidate relations, same-batch
// participation, labels, filenames, and caller-supplied KindIDs cannot
// substitute for that producer boundary.
package recordcarrier
