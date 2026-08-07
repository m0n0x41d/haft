// Package profileprojection maintains the human-readable project-profile
// projection derived from the canonical SQLite admission ledger.
//
// The YAML carrier is deliberately downstream of CanonicalProfileAdmission.
// It cannot admit a profile, recover authority, or mint an effect capability.
// SQLite and the filesystem are not one atomic store: ordinary effect failures
// commit append-only projection debt, while a hard crash is reconciled by the
// next Rebuild from the still-canonical ledger revision.
// Historical v1 debt remains readable. Every new debt event is appended to the
// tagged v2 event sum, including a v2 resolution of an unresolved v1 event.
package profileprojection
