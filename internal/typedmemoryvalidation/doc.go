// Package typedmemoryvalidation orchestrates the read-only typed-memory
// validation boundary.
//
// A wire BasisSelector is only an untrusted request. A server-owned
// BasisResolver supplies the environment, codec mechanisms, and immutable
// snapshot which may support an honest project verdict. The bundled FPF base
// is deliberately different: it can bind TypeEnv-dependent references, but it
// is not a project snapshot or an admission basis. Its P7 result therefore
// remains Underdetermined and never exposes typedmemory.AdmissionBatch.
//
// This package has no database, filesystem, transaction, or admission port.
package typedmemoryvalidation
