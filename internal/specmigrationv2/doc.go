// Package specmigrationv2 contains the pure validation core and guarded effect
// shell for the reviewed EnablingSystemSpec-to-SoftwareSystemSpec migration.
//
// It admits only exact source inventories, total tagged dispositions, exact
// split fragments, digest-pinned target claim catalogs, resolved OutsidePSS
// carriers, and canonical projectprofile applicability. DryRun owns no
// filesystem, database, lock, journal, or mutation capability. ApplyMigration
// is a separate outer shell whose filesystem mechanics recheck Git and carrier
// bytes and record a recoverable saga before installing reviewed bytes. Its
// public request constructor remains deliberately sealed: the current
// rehydrated project-profile value does not prove canonical SQLite origin or
// binding applicability. No caller can reach effects until P0PA supplies that
// sealed proof and the semantic-review adapter admits the exact packet.
package specmigrationv2
