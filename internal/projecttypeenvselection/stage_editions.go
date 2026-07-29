package projecttypeenvselection

const (
	// ProjectTypeEnvStageCompilerEditionV2 identifies the exact Stage
	// compilation interpretation used by the lossless v2 producer. It is not the
	// Base-TypeEnv compiler schema stored in the executable snapshot.
	ProjectTypeEnvStageCompilerEditionV2 = "project-typeenv-stage-compiler/v2"

	// ProjectTypeEnvStageProducerEditionV2 identifies the package-owned v2
	// Stage producer. It is provenance for preparation, not selection
	// authority.
	ProjectTypeEnvStageProducerEditionV2 = "project-typeenv-stage-producer/v2"

	// ProjectTypeEnvStageRevalidatorEditionV2 identifies the package-owned v2
	// Stage revalidation interpretation. Equality with this edition does not
	// prove that mutable Stage bases are current.
	ProjectTypeEnvStageRevalidatorEditionV2 = "project-typeenv-stage-revalidator/v2"

	// V3 editions identify the historical tag-only Genesis Stage contract.
	// V2 and V3 remain available solely for exact historical decode and replay.
	ProjectTypeEnvStageCompilerEditionV3    = "project-typeenv-stage-compiler/v3"
	ProjectTypeEnvStageProducerEditionV3    = "project-typeenv-stage-producer/v3"
	ProjectTypeEnvStageRevalidatorEditionV3 = "project-typeenv-stage-revalidator/v3"

	// V4 editions identify the first Stage contract whose source lineage is the
	// exact B/E/X/C closure bound by Stage itself. It carries no caller-supplied
	// shadow provenance coordinate.
	ProjectTypeEnvStageCompilerEditionV4    = "project-typeenv-stage-compiler/v4"
	ProjectTypeEnvStageProducerEditionV4    = "project-typeenv-stage-producer/v4"
	ProjectTypeEnvStageRevalidatorEditionV4 = "project-typeenv-stage-revalidator/v4"

	// V5 editions identify Transition Stages that bind the complete successor
	// diff together with compatibility results for every installed immutable
	// ProjectionProfile edition. Genesis remains byte-identical v4.
	ProjectTypeEnvStageCompilerEditionV5    = "project-typeenv-stage-compiler/v5"
	ProjectTypeEnvStageProducerEditionV5    = "project-typeenv-stage-producer/v5"
	ProjectTypeEnvStageRevalidatorEditionV5 = "project-typeenv-stage-revalidator/v5"
)

func StageCompilerEditionV2() StageCompilerEdition {
	return StageCompilerEdition{value: ProjectTypeEnvStageCompilerEditionV2}
}

func StageProducerEditionV2() StageProducerEdition {
	return StageProducerEdition{value: ProjectTypeEnvStageProducerEditionV2}
}

func StageRevalidatorEditionV2() StageRevalidatorEdition {
	return StageRevalidatorEdition{value: ProjectTypeEnvStageRevalidatorEditionV2}
}

func StageCompilerEditionV3() StageCompilerEdition {
	return StageCompilerEdition{value: ProjectTypeEnvStageCompilerEditionV3}
}

func StageProducerEditionV3() StageProducerEdition {
	return StageProducerEdition{value: ProjectTypeEnvStageProducerEditionV3}
}

func StageRevalidatorEditionV3() StageRevalidatorEdition {
	return StageRevalidatorEdition{value: ProjectTypeEnvStageRevalidatorEditionV3}
}

func StageCompilerEditionV4() StageCompilerEdition {
	return StageCompilerEdition{value: ProjectTypeEnvStageCompilerEditionV4}
}

func StageProducerEditionV4() StageProducerEdition {
	return StageProducerEdition{value: ProjectTypeEnvStageProducerEditionV4}
}

func StageRevalidatorEditionV4() StageRevalidatorEdition {
	return StageRevalidatorEdition{value: ProjectTypeEnvStageRevalidatorEditionV4}
}

func StageCompilerEditionV5() StageCompilerEdition {
	return StageCompilerEdition{value: ProjectTypeEnvStageCompilerEditionV5}
}

func StageProducerEditionV5() StageProducerEdition {
	return StageProducerEdition{value: ProjectTypeEnvStageProducerEditionV5}
}

func StageRevalidatorEditionV5() StageRevalidatorEdition {
	return StageRevalidatorEdition{value: ProjectTypeEnvStageRevalidatorEditionV5}
}
