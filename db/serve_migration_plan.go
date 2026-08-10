package db

// ServeMigrationPlanKind is the closed startup posture for an observed schema.
type ServeMigrationPlanKind string

const (
	ServeMigrationCurrent        ServeMigrationPlanKind = "current"
	ServeMigrationAutomatic      ServeMigrationPlanKind = "automatic"
	ServeMigrationManualRequired ServeMigrationPlanKind = "manual_required"
	ServeMigrationFutureSchema   ServeMigrationPlanKind = "future_schema"
	ServeMigrationInvalidCatalog ServeMigrationPlanKind = "invalid_catalog"
)

// ServeMigrationPlan is the domain port consumed by startup coordination.
// PendingVersions is always the exact contiguous suffix after ObservedSchema.
// FirstBlockedVersion is set only when the suffix cannot run automatically.
type ServeMigrationPlan struct {
	Kind                ServeMigrationPlanKind
	ObservedSchema      int
	CompiledSchema      int
	PendingVersions     []int
	FirstBlockedVersion int
	SnapshotRequired    bool
}

// CompileServeMigrationPlan classifies one already validated project schema
// against the compiled kernel catalog. It does not inspect or mutate SQLite.
func CompileServeMigrationPlan(observedSchema int) (ServeMigrationPlan, error) {
	compiledSchema, err := CurrentSchemaVersion()
	if err != nil {
		return ServeMigrationPlan{}, err
	}
	return compileServeMigrationPlan(
		observedSchema,
		compiledSchema,
		kernelMigrations,
	), nil
}

func compileServeMigrationPlan(
	observedSchema int,
	compiledSchema int,
	migrations []Migration,
) ServeMigrationPlan {
	plan := ServeMigrationPlan{
		ObservedSchema: observedSchema,
		CompiledSchema: compiledSchema,
	}
	if observedSchema <= 0 || compiledSchema <= 0 {
		plan.Kind = ServeMigrationInvalidCatalog
		return plan
	}
	if observedSchema > compiledSchema {
		plan.Kind = ServeMigrationFutureSchema
		return plan
	}
	if observedSchema == compiledSchema {
		plan.Kind = ServeMigrationCurrent
		return plan
	}
	pending := migrationsAfterSchema(migrations, observedSchema)
	plan.PendingVersions = migrationVersions(pending)
	if !isExactMigrationSuffix(
		pending,
		observedSchema,
		compiledSchema,
	) {
		plan.Kind = ServeMigrationInvalidCatalog
		return plan
	}
	for _, migration := range pending {
		switch migration.ServeActivation {
		case ServeActivationAutomatic:
		case ServeActivationAutomaticWithSnapshot:
			plan.SnapshotRequired = true
		case ServeActivationManualWithSnapshot:
			plan.SnapshotRequired = true
			plan.Kind = ServeMigrationManualRequired
			plan.FirstBlockedVersion = migration.Version
			return plan
		case ServeActivationManual:
			plan.Kind = ServeMigrationManualRequired
			plan.FirstBlockedVersion = migration.Version
			return plan
		default:
			plan.Kind = ServeMigrationInvalidCatalog
			plan.FirstBlockedVersion = migration.Version
			return plan
		}
	}
	plan.Kind = ServeMigrationAutomatic
	return plan
}

func migrationsAfterSchema(
	migrations []Migration,
	observedSchema int,
) []Migration {
	pending := make([]Migration, 0)
	for _, migration := range migrations {
		if migration.Version <= observedSchema {
			continue
		}
		pending = append(pending, migration)
	}
	return pending
}

func migrationVersions(migrations []Migration) []int {
	versions := make([]int, 0, len(migrations))
	for _, migration := range migrations {
		versions = append(versions, migration.Version)
	}
	return versions
}

func isExactMigrationSuffix(
	migrations []Migration,
	observedSchema int,
	compiledSchema int,
) bool {
	if len(migrations) != compiledSchema-observedSchema {
		return false
	}
	expected := observedSchema + 1
	for _, migration := range migrations {
		if migration.Version != expected {
			return false
		}
		expected++
	}
	return expected == compiledSchema+1
}
