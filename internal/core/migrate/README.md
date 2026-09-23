`migrate.Run` stages a bounded v9 capture in a separate project directory. It
requires explicit source paths, an output root, an RFC3339 timestamp, and
`dry_run: true`. It does not select a global database, activate records, switch
projects, install a runtime, or modify source files.

The decoder reads `artifacts`, `artifact_links`, `affected_files`,
`affected_symbols`, `artifact_symbol_bindings`, `evidence_items`, `evidence`,
`spec_section_editions`, and `spec_section_baselines`. Additional columns are
retained in byte-preserving row sidecars. Unknown tables, incompatible schemas,
and incomplete reads have explicit scope dispositions; their row counts are not
reported as zero. Default bounds are 10,000 rows per table/carrier scope and
256 MiB per captured DB file set or carrier scope. SQL extraction also bounds
the retained column bytes per table.

Capture compares two full DB/WAL/SHM manifests, including presence transitions,
with at most three attempts. A nonempty rollback journal is rejected. SQLite
opens only a private temporary copy, uses `mode=ro` and `query_only`, checks
integrity, and reads in one transaction. The temporary copy is removed. This is
a stable observed byte copy with a coherent SQLite interpretation; it cannot
promise an atomic filesystem snapshot against arbitrary concurrent writers.
Original source paths are opened through rooted handles and must be regular
files. A symlinked source file is not followed.

Frontmatter carriers and recognized SQL rows are independent sources. Differences
in body or metadata produce a source conflict with both originals addressable.
Missing about, object, question, choice, rationale, timestamp, or receiving-use
fields are not invented. New IDs have native syntax; `LookupLegacy` returns all
source-qualified aliases for an old short or long ID. No ambiguity is settled by
selecting the first match.

Mapped records retain `origin: migrated_9x`, their original status, and no operator
confirmation. Their projection remains historical. Relations become navigation
with their original type retained. Explicit governance selectors and implementation
footprint roles remain distinct. Historical evidence retains its exact original
verdict, scope, and claim references but receives no use on a converted target
hash. DB section claims require an explicit recognizable native kind; quadrant
labels alone are insufficient.

Only already-valid `haft.terms/1` term maps are copied to `specs/terms.md`. Legacy
fenced sections and term maps have no compiler in this package: their original
bytes and a rewrite queue entry remain available. This is a known compatibility
limit. Unknown source revisions, missing terms, unsupported shapes, and all known
field/record losses remain in the report. MethodRun, RefreshReport, WorkCommission,
and associations from those records have identifiable exclusion entries; their
payloads are not copied into the staging project.

All staged carriers, exact editions, source sidecars, and the report use
`store.Publish`. A retry with the same request and source basis replays the
committed result. Changed requests conflict; existing stage files are never
overwritten. A new source capture may require a new staging directory if it would
change an existing destination. No merge into edited staging data is inferred.

`Queue` reads pending recovery work. `Assist` records one explicit agent proposal,
its source locator, a repair reason, an exact edition, and a queue resolution in
one publication. It checks the expected generation and persisted dependencies,
including full portable source provenance and declared term bytes. It does not
accept, confirm, or supersede a record. Repeating the same assistance request does
not duplicate the proposal. Conflicting queue entries fail explicitly.

The tests create real temporary WAL databases and isolated carrier/staging roots.
They exercise retained and excluded records, both evidence tables, partial losses,
invalid UTF-8, conflicts, source-byte preservation, concurrent WAL writes,
interrupted/replayed assistance, immutable reports, and dependency admission. No
test reads a live `.haft` directory.
