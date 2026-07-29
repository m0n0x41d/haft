// Package typedmemorystore owns the transactional SQLite effect boundary for
// typed project memory. The exact v46 writer revalidates and atomically stores
// non-binding entity declarations, context-local alias changes, typed relation
// instances, and assertion retractions together with their sealed admission
// basis and independently verified materialization. Merge and split remain
// outside generic admission. Product activation and host wiring belong to the
// outer project-memory shell.
package typedmemorystore
