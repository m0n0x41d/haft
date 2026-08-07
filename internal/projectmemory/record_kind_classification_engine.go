package projectmemory

import (
	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/kindclassificationengine"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectObjectFamilyFeatureKey  = "haft.project-object.family"
	projectRecordVariantFeatureKey = "haft.project-record.variant"
	projectObjectFamilyGovernor    = "haft.feature.project-object-family/v1"
	projectRecordVariantGovernor   = "haft.feature.project-record-carrier/v1"
	projectRecordFamilyToken       = "project_record"
	entityPresenceFeatureKey       = "haft.entity.identity-present"
	entityPresenceFeatureGovernor  = "haft.feature.entity-visibility/v1"
	entityPresenceFeatureToken     = "present"
)

var (
	ErrRecordKindClassificationRuntimeMissing = kindclassificationengine.ErrRecordKindClassificationRuntimeMissing
	ErrRecordKindClassificationRuntimeInvalid = kindclassificationengine.ErrRecordKindClassificationRuntimeInvalid
)

// ProjectKindClassificationAdmissionEngine remains the stable projectmemory
// surface while the implementation lives below TypeEnv transition/effect
// packages and can therefore be reused without introducing an import cycle.
type ProjectKindClassificationAdmissionEngine = kindclassificationengine.ProjectKindClassificationAdmissionEngine

func NewProjectKindClassificationAdmissionEngine(
	registry kindclassificationruntime.Registry,
) (ProjectKindClassificationAdmissionEngine, error) {
	return kindclassificationengine.NewProjectKindClassificationAdmissionEngine(
		registry,
	)
}

// RecordKindClassificationAdmissionEngine is retained as a source-compatible
// alias while historical call sites move to the project-wide current engine.
type RecordKindClassificationAdmissionEngine = ProjectKindClassificationAdmissionEngine

func NewRecordKindClassificationAdmissionEngine(
	registry kindclassificationruntime.Registry,
) (RecordKindClassificationAdmissionEngine, error) {
	return NewProjectKindClassificationAdmissionEngine(registry)
}

func verifiedRecordFeatureText(
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	raw string,
) (typedmemory.VerifiedTypedValue, error) {
	return kindclassificationengine.VerifyProjectFeatureText(
		environment,
		registry,
		raw,
	)
}
