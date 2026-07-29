package typeenv

// BaseTypeEnvCompilerSchemaV2 remains the historical selected compiler edition
// used by already-sealed project stages. A v3 source candidate does not
// silently relabel those stages.
const BaseTypeEnvCompilerSchemaV2 = "fpf-base-typeenv.cov2.v2"

// BaseTypeEnvCompilerSchemaV3 remains the historical compiler edition used by
// the exact 6e7eeb9 Base and already-sealed project stages. Current source does
// not silently relabel those bytes.
const BaseTypeEnvCompilerSchemaV3 = "fpf-base-typeenv.cov2.v3"

// BaseTypeEnvCompilerSchemaV4 remains the historical compiler edition that
// replaced the superseded C.3.1 prose adapter with the C.3.1-C.3.A contract
// families. Current source does not silently relabel its artifacts.
const BaseTypeEnvCompilerSchemaV4 = "fpf-base-typeenv.cov2.v4"

// BaseTypeEnvCompilerSchemaV5 identifies the current source-to-B compiler
// interpretation used by CompileBaseTypeEnv. It recognizes the explicit
// covered-claim semantics of C.2.1 empirical grounding while keeping the
// covered subgraph outside the two participant SlotSpecs. This is an
// implementation edition, not a source revision or a TypeEnv selection act.
const BaseTypeEnvCompilerSchemaV5 = baseTypeEnvCompilerSchema
