// Package profileadmission defines the non-binding host request for one
// profile-declaration admission. It cannot execute an admission or mint an
// admitted result.
//
// The concrete SQLite boundary owns authority resolution, durable Work
// recovery, ledger CAS, canonical writes, COMMIT recovery, and strict durable
// reread. Keeping the request here lets callers name the intended effect
// without exposing a replaceable transaction port or a structurally mintable
// admitted token.
package profileadmission
