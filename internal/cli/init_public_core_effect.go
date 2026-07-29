package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/initexecution"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

type publicProjectCoreEffect struct {
	request publicInitRequest
	output  io.Writer
}

type publicCoreApplicationError struct {
	cause   error
	receipt publicExactFileReceipt
}

func (failure publicCoreApplicationError) Error() string {
	return failure.cause.Error()
}

func (failure publicCoreApplicationError) Unwrap() error {
	return failure.cause
}

func newPublicProjectCoreEffect(
	request publicInitRequest,
	output io.Writer,
) publicProjectCoreEffect {
	return publicProjectCoreEffect{
		request: request,
		output:  output,
	}
}

func (effect publicProjectCoreEffect) ApplyCore(
	ctx context.Context,
	plan initplanning.CoreProjectPlan,
) (initexecution.CoreEffectReceipt, error) {
	if ctx == nil {
		return initexecution.CoreEffectReceipt{},
			fmt.Errorf("public project core context is required")
	}
	if effect.output == nil {
		return initexecution.CoreEffectReceipt{},
			fmt.Errorf("public project core output is required")
	}
	if plan.ProjectRoot() != effect.request.projectRoot ||
		plan.ProjectID().String() != effect.request.projectID {
		return initexecution.CoreEffectReceipt{},
			fmt.Errorf(
				"public project core plan differs from its exact request",
			)
	}
	recovery := []string{"haft", "init", "--core-only"}
	coreFiles, err := publicExactCoreFileEffects(plan)
	if err != nil {
		return initexecution.CoreEffectReceipt{}, err
	}
	plannedPaths := publicCoreApplicationPaths(plan, coreFiles)
	if err := verifyPublicRootMigrationPrecondition(plan); err != nil {
		return initexecution.CoreEffectReceipt{},
			newPublicCoreApplicationError(
				err,
				nil,
				plan.RootMigration().Target(),
				plannedPaths,
				recovery,
			)
	}
	if err := verifyPublicCorePreconditions(plan); err != nil {
		return initexecution.CoreEffectReceipt{},
			newPublicCoreApplicationError(
				err,
				nil,
				plan.DatabasePath(),
				plannedPaths,
				recovery,
			)
	}
	for _, file := range coreFiles {
		if err := verifyPublicExactFile(file); err != nil {
			return initexecution.CoreEffectReceipt{},
				newPublicCoreApplicationError(
					err,
					nil,
					file.path,
					plannedPaths,
					recovery,
				)
		}
	}
	completedPaths := make([]string, 0, len(plannedPaths))
	var coreReceipt initexecution.CoreEffectReceipt
	if plan.Effect() != initplanning.CoreInitialize {
		coreReceipt, err = effect.applyDatabase(ctx, plan)
		if err != nil {
			return initexecution.CoreEffectReceipt{},
				newPublicCoreApplicationError(
					err,
					completedPaths,
					plan.DatabasePath(),
					plannedPaths,
					recovery,
				)
		}
		if plan.Effect() == initplanning.CoreMigrate {
			completedPaths = append(
				completedPaths,
				plan.DatabasePath(),
			)
		}
	}
	if err := project.EnsureDir(); err != nil {
		return initexecution.CoreEffectReceipt{},
			newPublicCoreApplicationError(
				err,
				completedPaths,
				filepath.Dir(plan.DatabasePath()),
				publicRemainingCorePaths(
					plannedPaths,
					len(completedPaths),
				),
				recovery,
			)
	}
	fileReceipt, err := applyPublicExactFileEffects(
		ctx,
		coreFiles,
		recovery,
	)
	completedPaths = append(
		completedPaths,
		fileReceipt.Completed()...,
	)
	if err != nil {
		return initexecution.CoreEffectReceipt{},
			publicCoreApplicationError{
				cause: err,
				receipt: publicExactFileReceipt{
					completed: completedPaths,
					failed:    fileReceipt.Failed(),
					untouched: fileReceipt.Untouched(),
					retry:     fileReceipt.Retry(),
					recovery:  fileReceipt.Recovery(),
				},
			}
	}
	if err := migratePublicLegacyProjectRoot(plan); err != nil {
		migrationPath := plan.RootMigration().Target()
		return initexecution.CoreEffectReceipt{},
			publicCoreApplicationError{
				cause: err,
				receipt: publicExactFileReceipt{
					completed: completedPaths,
					failed:    migrationPath,
					retry:     []string{migrationPath},
					recovery:  recovery,
				},
			}
	}
	if plan.RootMigration().Kind() ==
		initplanning.CoreRootMigrationQuintToHaft {
		completedPaths = append(
			completedPaths,
			plan.RootMigration().Target(),
		)
	}
	if plan.Effect() == initplanning.CoreInitialize {
		coreReceipt, err = effect.applyDatabase(ctx, plan)
		if err != nil {
			return initexecution.CoreEffectReceipt{},
				publicCoreApplicationError{
					cause: err,
					receipt: publicExactFileReceipt{
						completed: completedPaths,
						failed:    plan.DatabasePath(),
						retry:     []string{plan.DatabasePath()},
						recovery:  recovery,
					},
				}
		}
	}
	return coreReceipt, nil
}

func newPublicCoreApplicationError(
	cause error,
	completed []string,
	failed string,
	remaining []string,
	recovery []string,
) publicCoreApplicationError {
	return publicCoreApplicationError{
		cause: cause,
		receipt: publicExactFileReceipt{
			completed: slices.Clone(completed),
			failed:    failed,
			untouched: slices.Clone(remaining),
			retry:     slices.Clone(remaining),
			recovery:  slices.Clone(recovery),
		},
	}
}

func publicCoreApplicationPaths(
	plan initplanning.CoreProjectPlan,
	files []publicExactFileEffect,
) []string {
	paths := make(
		[]string,
		0,
		len(files)+2,
	)
	if plan.Effect() == initplanning.CoreMigrate {
		paths = append(paths, plan.DatabasePath())
	}
	paths = append(paths, publicExactFilePaths(files)...)
	if plan.RootMigration().Kind() ==
		initplanning.CoreRootMigrationQuintToHaft {
		paths = append(paths, plan.RootMigration().Target())
	}
	if plan.Effect() == initplanning.CoreInitialize {
		paths = append(paths, plan.DatabasePath())
	}
	return paths
}

func publicRemainingCorePaths(
	paths []string,
	completed int,
) []string {
	if completed >= len(paths) {
		return nil
	}
	return slices.Clone(paths[completed:])
}

func verifyPublicRootMigrationPrecondition(
	plan initplanning.CoreProjectPlan,
) error {
	migration := plan.RootMigration()
	switch migration.Kind() {
	case initplanning.CoreRootMigrationNone:
		return nil
	case initplanning.CoreRootMigrationQuintToHaft:
		info, err := os.Stat(migration.Source())
		if err != nil {
			return fmt.Errorf(
				"verify planned legacy project root: %w",
				err,
			)
		}
		if !info.IsDir() {
			return fmt.Errorf(
				"planned legacy project root is not a directory",
			)
		}
		if _, err := os.Stat(migration.Target()); err == nil {
			return fmt.Errorf(
				"planned legacy root migration target now exists",
			)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf(
				"verify planned legacy root target: %w",
				err,
			)
		}
		return nil
	default:
		return fmt.Errorf("planned legacy root migration is invalid")
	}
}

func publicExactCoreFileEffects(
	plan initplanning.CoreProjectPlan,
) ([]publicExactFileEffect, error) {
	planned := plan.FileEffects()
	effects := make(
		[]publicExactFileEffect,
		len(planned),
	)
	for index, effect := range planned {
		kind := publicExactFileEffectKind(effect.Kind())
		if kind != publicExactFileCreate &&
			kind != publicExactFilePreserve &&
			kind != publicExactFileReplace {
			return nil, fmt.Errorf(
				"core file effect %s has invalid kind %s",
				effect.Path(),
				effect.Kind(),
			)
		}
		effects[index] = publicExactFileEffect{
			kind:           kind,
			path:           effect.Path(),
			content:        effect.Content(),
			mode:           effect.Mode(),
			renderedDigest: effect.RenderedDigest(),
			expectedDigest: effect.ExpectedDigest(),
			expectedMode:   effect.ExpectedMode(),
		}
	}
	return effects, nil
}

func verifyPublicCorePreconditions(
	plan initplanning.CoreProjectPlan,
) error {
	if plan.Effect() != initplanning.CoreInitialize {
		return nil
	}
	if _, err := os.Stat(plan.DatabasePath()); err == nil {
		return fmt.Errorf(
			"planned project database initialization found an existing database",
		)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf(
			"inspect planned project database initialization: %w",
			err,
		)
	}
	seed := plan.DatabaseSeed()
	if seed.Kind() == initplanning.CoreDatabaseSeedEmpty {
		return nil
	}
	if seed.Kind() != initplanning.CoreDatabaseSeedLegacyCopy {
		return fmt.Errorf("planned project database seed is invalid")
	}
	digest, err := digestRegularFile(seed.ObservationPath())
	if err != nil {
		return fmt.Errorf(
			"verify observed legacy project database: %w",
			err,
		)
	}
	if digest != seed.Digest() {
		return fmt.Errorf(
			"legacy project database %s changed after preview; no project files were written",
			seed.ObservationPath(),
		)
	}
	return nil
}

func (effect publicProjectCoreEffect) applyDatabase(
	ctx context.Context,
	plan initplanning.CoreProjectPlan,
) (initexecution.CoreEffectReceipt, error) {
	if plan.Effect() != initplanning.CoreInitialize {
		return (initexecution.ExistingProjectCoreEffect{}).
			ApplyCore(ctx, plan)
	}
	switch plan.DatabaseSeed().Kind() {
	case initplanning.CoreDatabaseSeedEmpty:
	case initplanning.CoreDatabaseSeedLegacyCopy:
		if err := materializePublicLegacyDatabaseSeed(plan); err != nil {
			return initexecution.CoreEffectReceipt{}, err
		}
	default:
		return initexecution.CoreEffectReceipt{}, fmt.Errorf(
			"planned project database seed is invalid",
		)
	}
	if err := initializeDatabase(plan.DatabasePath()); err != nil {
		return initexecution.CoreEffectReceipt{}, err
	}
	if err := projectledger.BindInitialized(
		ctx,
		plan.ProjectRoot(),
		time.Now().UTC(),
	); err != nil {
		return initexecution.CoreEffectReceipt{}, err
	}
	return initexecution.NewCoreEffectReceipt(
		initexecution.CoreEffectApplied,
		initplanning.CoreInitialize,
		plan.ProjectRoot(),
		plan.ProjectID().String(),
		plan.DatabasePath(),
		plan.BeforeSchema(),
		plan.AfterSchema(),
	)
}

func materializePublicLegacyDatabaseSeed(
	plan initplanning.CoreProjectPlan,
) error {
	seed := plan.DatabaseSeed()
	digest, err := digestRegularFile(seed.SourcePath())
	if err != nil {
		return fmt.Errorf(
			"verify planned legacy project database: %w",
			err,
		)
	}
	if digest != seed.Digest() {
		return fmt.Errorf(
			"legacy project database %s changed after preview; canonical database was not written",
			seed.SourcePath(),
		)
	}
	source, err := os.Open(seed.SourcePath())
	if err != nil {
		return fmt.Errorf(
			"open planned legacy project database: %w",
			err,
		)
	}
	parent := filepath.Dir(plan.DatabasePath())
	if err := os.MkdirAll(parent, 0o755); err != nil {
		_ = source.Close()
		return fmt.Errorf(
			"create canonical project database parent: %w",
			err,
		)
	}
	stage, err := os.CreateTemp(
		parent,
		".haft-legacy-database-*",
	)
	if err != nil {
		_ = source.Close()
		return fmt.Errorf(
			"stage legacy project database: %w",
			err,
		)
	}
	stagePath := stage.Name()
	copyErr := stage.Chmod(0o600)
	if copyErr == nil {
		_, copyErr = io.Copy(stage, source)
	}
	if copyErr == nil {
		copyErr = stage.Sync()
	}
	sourceCloseErr := source.Close()
	stageCloseErr := stage.Close()
	if copyErr != nil || sourceCloseErr != nil || stageCloseErr != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf(
			"stage legacy project database: %w",
			errors.Join(copyErr, sourceCloseErr, stageCloseErr),
		)
	}
	if err := os.Rename(stagePath, plan.DatabasePath()); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf(
			"publish legacy project database: %w",
			err,
		)
	}
	return nil
}

func migratePublicLegacyProjectRoot(
	plan initplanning.CoreProjectPlan,
) error {
	migration := plan.RootMigration()
	if migration.Kind() == initplanning.CoreRootMigrationNone {
		return nil
	}
	if migration.Kind() !=
		initplanning.CoreRootMigrationQuintToHaft {
		return fmt.Errorf("planned legacy root migration is invalid")
	}
	if err := os.Rename(
		migration.Source(),
		migration.Target(),
	); err != nil {
		return fmt.Errorf("migrate legacy project root: %w", err)
	}
	return nil
}
