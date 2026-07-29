package projecttypeenvheadstore

import "errors"

var (
	ErrContextRequired = errors.New(
		"project TypeEnv head-store context is required",
	)
	ErrStoreRequired = errors.New(
		"project TypeEnv head store is required",
	)
	ErrStoredHeadIntegrity = errors.New(
		"stored project TypeEnv head integrity failure",
	)
	ErrProjectTypeEnvHeadCASConflict = errors.New(
		"project TypeEnv head compare-and-swap conflict",
	)
	ErrHeadRevisionOutOfSQLiteRange = errors.New(
		"project TypeEnv head revision exceeds the SQLite integer range",
	)
)
