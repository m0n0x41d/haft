# Haft Typed-Memory Local-Practice carrier

**Contract truth: V9 CONTRACT.** This candidate source carrier contributes to
the typed-memory contract; its presence is not installed-runtime proof. A
readiness claim requires current **EXACT-CANDIDATE EVIDENCE** from P14 tied to
the exact candidate bytes. RC or release status additionally requires release
authority; neither follows from this carrier alone.

This directory contains repo-owned, versioned source carriers for the Haft
Typed-Memory local practice.

[`candidates/1.0.0.yaml`](candidates/1.0.0.yaml) is the byte-stable historical
candidate. `SourceV1()` continues to return those exact bytes for replay.

[`candidates/1.1.0.yaml`](candidates/1.1.0.yaml) is the byte-stable additive
successor candidate based on aligned FPF Base TypeEnv
`typeenv:sha256:a5223d5018230095652543f0378a1fc3f64175f21d01309e6f4084088d5d2804`.
`SourceV1_1()` continues to return those exact bytes for replay.

[`candidates/1.2.0.yaml`](candidates/1.2.0.yaml) carries the same additive
declaration set on the current FPF Base TypeEnv
`typeenv:sha256:973eeeed8e234b4ff0194662d80e204fe27ad5ba92c87840a6d1ed3a9d5d742d`.
It is a new carrier edition rather than a rewrite of either historical byte
stream. The additive declaration set contains:

- the local structural shape and codec binding
  `Haft.Shape.ProjectMemoryReferenceSchemeV1` /
  `Haft.Codec.ProjectMemoryReferenceSchemeV1` for inherited
  `U.ReferenceScheme`;
- `Haft.ProjectEpistemeConstitutionBasis`, a non-relational
  `runtime_evaluator_input` carrier for a ClaimGraph by value, an
  EntityOfConcern reference, and a ReferenceScheme by value; and
- five source-declared evaluator requirements covering reference designation
  resolution, claim interpretation, claim measurement, claim evaluation, and
  episteme-constitution evaluation.

All manifests remain `candidate`. They are suitable for deterministic
parsing, symbolic compilation, sealing, composite preparation, and read-only
Stage work. Neither is an active or selected project TypeEnv, and neither
authorizes a `ProjectTypeEnvHead` effect.

The sealed carrier token `kind: relation_signature` and the Go/wire spellings
`RelationSignature`, `RelationSignatureRef`, and
`define_relation_signature` are historical edition-compatibility names. The
current compiler classifies their actual closed payload as
`TypedRelationDeclarationFragment`: symbol/ref, BoundedContext set, named
SlotSpecs, separately declared structural cardinality/constraint rules, and
provenance. Current TypeEnv values, compiler metadata, diagnostics, and read
postures expose `typed_relation_declaration_fragment`; exact 1.0.0/1.1.0
carrier bytes are not rewritten, and 1.2.0 has its own coordinate.

That fragment posture is intentionally weaker than an FPF
`RelationSignature`. These carriers do not contain or execute the direct
obtaining predicate/laws, applicability, occurrence-identity rule, or the
complete declaration-episteme ClaimGraph/ReferenceScheme basis. Structural
slot validation therefore cannot establish truth, create an obtaining
occurrence, or admit a durable FPF relation kind. Full RelationSignature
lowering remains outside v9 until a named receiving use and a separate human
choice justify its schema and runtime fanout.

The candidate deliberately implements Haft-local record/use/occurrence
contracts. It does not export fake `U.*` symbols and does not claim that
`Haft.EvidenceUse` is exact FPF Evidence or that
`Haft.WorkOccurrenceRecord` is performed `U.Work`. The Core5 polarity
tokens, Completed/InFlight interval algebra, and canonical-instant contract
remain candidate choices pending their reviewed specification decision and
the separate human-gated head selection.

The 1.1.0/1.2.0 basis carriers are not the source C.2.1
`EpistemeConstitutionRelationSignature`, an obtaining relation occurrence, or
proof that the constitution predicate obtains. Its
`runtime_evaluator_requirement` declarations state required E-to-X
coordinates only. They do not identify callable mechanism implementations,
record an invocation, or provide positive `Satisfied` evidence. Missing exact
runtime basis remains `Underdetermined(reference_scheme_runtime_basis_missing)`.
