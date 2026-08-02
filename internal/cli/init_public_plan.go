package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/method"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectledgermigration"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"gopkg.in/yaml.v3"
)

func compilePublicCorePlan(
	ctx context.Context,
	request publicInitRequest,
	userHomeRoot string,
) (initplanning.CoreProjectPlan, error) {
	if ctx == nil {
		return initplanning.CoreProjectPlan{},
			fmt.Errorf("public core planning context is required")
	}
	databasePath, err := project.CanonicalDBPath(
		userHomeRoot,
		request.projectID,
	)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	compiledSchema, err := db.CurrentSchemaVersion()
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	basis, err := initplanning.NewUnavailableBasis(
		"project memory basis is unavailable before core initialization",
	)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	effect := initplanning.CoreInitialize
	beforeSchema := 0
	legacySeedObservationPath := ""
	legacySeedPath := ""
	legacySeedDigest := ""
	rootMigrationSource, rootMigrationTarget, err :=
		observePublicLegacyRootMigration(request.projectRoot)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	carrierRoot := filepath.Join(request.projectRoot, ".haft")
	if rootMigrationSource != "" {
		carrierRoot = rootMigrationSource
	}
	config, err := project.Load(
		carrierRoot,
	)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	databasePresent, err := publicDatabasePresent(
		databasePath,
	)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	if config == nil && databasePresent {
		return initplanning.CoreProjectPlan{}, fmt.Errorf(
			"canonical project database exists without a project identity; initialization requires explicit recovery",
		)
	}
	if config != nil {
		if config.ID != request.projectID {
			return initplanning.CoreProjectPlan{}, fmt.Errorf(
				"project root carries identity %s, request identifies %s",
				config.ID,
				request.projectID,
			)
		}
		if databasePresent {
			requested, requestErr :=
				projectledgermigration.NewRequest(
					request.projectRoot,
					request.projectID,
				)
			if requestErr != nil {
				return initplanning.CoreProjectPlan{}, requestErr
			}
			observation, observationErr :=
				projectledgermigration.Observe(ctx, requested)
			if observationErr != nil {
				return initplanning.CoreProjectPlan{}, observationErr
			}
			if observation.DatabasePath != databasePath {
				return initplanning.CoreProjectPlan{}, fmt.Errorf(
					"observed project database %s differs from canonical plan path %s",
					observation.DatabasePath,
					databasePath,
				)
			}
			beforeSchema = observation.ObservedSchema
			effect, err = selectPublicCoreEffect(
				beforeSchema,
				observation.CompiledSchema,
			)
			if err != nil {
				return initplanning.CoreProjectPlan{}, err
			}
			if effect == initplanning.CoreVerifyCurrent {
				basis, err = observePublicTypeEnvBasis(
					ctx,
					request.projectRoot,
					request.projectID,
				)
				if err != nil {
					return initplanning.CoreProjectPlan{}, err
				}
			} else {
				basis, err = initplanning.NewUnavailableBasis(
					"project memory basis will be re-observed after the planned core migration",
				)
				if err != nil {
					return initplanning.CoreProjectPlan{}, err
				}
			}
		}
	}
	if !databasePresent {
		legacySeedObservationPath,
			legacySeedPath,
			legacySeedDigest,
			_,
			err =
			observePublicLegacyDatabaseSeed(
				request.projectRoot,
				databasePath,
			)
		if err != nil {
			return initplanning.CoreProjectPlan{}, err
		}
	}
	profileFiles, err := compilePublicProfileCoreFileInputs(
		ctx,
		request,
		effect == initplanning.CoreVerifyCurrent,
		carrierRoot,
	)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	profileBootstrap, err := compilePublicInitialProfileBootstrapPlan(
		ctx,
		request,
		databasePresent,
		carrierRoot,
	)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	coreFiles, err := compilePublicCoreFileEffects(
		request,
		config,
		legacySeedPath != "",
		profileFiles,
		carrierRoot,
	)
	if err != nil {
		return initplanning.CoreProjectPlan{}, err
	}
	builder := initplanning.NewCoreProjectPlanBuilder().
		ForProject(request.projectRoot, request.projectID).
		AtDatabase(databasePath).
		WithSchemaTransition(
			effect,
			beforeSchema,
			compiledSchema,
		).
		WithBasis(basis).
		WithFileEffects(coreFiles).
		WithInitialProfileBootstrap(profileBootstrap)
	if legacySeedPath != "" {
		builder = builder.WithLegacyDatabaseSeed(
			legacySeedObservationPath,
			legacySeedPath,
			legacySeedDigest,
		)
	}
	if rootMigrationSource != "" {
		builder = builder.WithLegacyRootMigration(
			rootMigrationSource,
			rootMigrationTarget,
		)
	}
	return builder.Build()
}

func observePublicLegacyRootMigration(
	projectRoot string,
) (string, string, error) {
	source := filepath.Join(projectRoot, ".quint")
	target := filepath.Join(projectRoot, ".haft")
	sourceInfo, sourceErr := os.Stat(source)
	if os.IsNotExist(sourceErr) {
		return "", "", nil
	}
	if sourceErr != nil {
		return "", "", fmt.Errorf(
			"inspect legacy project root: %w",
			sourceErr,
		)
	}
	if !sourceInfo.IsDir() {
		return "", "", fmt.Errorf(
			"legacy project root is not a directory",
		)
	}
	if _, targetErr := os.Stat(target); targetErr == nil {
		return "", "", nil
	} else if !os.IsNotExist(targetErr) {
		return "", "", fmt.Errorf(
			"inspect current project root: %w",
			targetErr,
		)
	}
	return source, target, nil
}

type publicCoreFileInput struct {
	path             string
	content          []byte
	mode             os.FileMode
	preserveExisting bool
}

func compilePublicCoreFileEffects(
	request publicInitRequest,
	existingIdentity *project.Config,
	hasLegacyDatabaseSeed bool,
	profileFiles []publicCoreFileInput,
	carrierRoot string,
) ([]initplanning.CoreFileEffect, error) {
	inputs, err := publicCoreFileInputs(
		request,
		existingIdentity,
		hasLegacyDatabaseSeed,
		profileFiles,
		carrierRoot,
	)
	if err != nil {
		return nil, err
	}
	effects, err := publicCoreFileEffectsFromInputs(inputs)
	if err != nil {
		return nil, err
	}
	legacyConfig, present, err := planPublicLegacyProjectConfig(carrierRoot)
	if err != nil {
		return nil, err
	}
	if !present {
		return effects, nil
	}
	legacyEffect, err := initplanning.NewCoreFileEffect(
		initplanning.CoreFileEffectKind(legacyConfig.kind),
		legacyConfig.path,
		legacyConfig.content,
		legacyConfig.mode,
		legacyConfig.renderedDigest,
		legacyConfig.expectedDigest,
		legacyConfig.expectedMode,
	)
	if err != nil {
		return nil, err
	}
	return append(effects, legacyEffect), nil
}

func publicCoreFileInputs(
	request publicInitRequest,
	existingIdentity *project.Config,
	hasLegacyDatabaseSeed bool,
	profileFiles []publicCoreFileInput,
	carrierRoot string,
) ([]publicCoreFileInput, error) {
	inputs := make([]publicCoreFileInput, 0, 10)
	for _, directory := range []string{
		"notes",
		"problems",
		"solutions",
		"decisions",
		"evidence",
		"refresh",
	} {
		inputs = append(inputs, publicCoreFileInput{
			path: filepath.Join(
				carrierRoot,
				directory,
				".gitkeep",
			),
			content: []byte{},
			mode:    0o644,
		})
	}
	inputs = append(
		inputs,
		publicCoreFileInput{
			path: filepath.Join(carrierRoot, "workflow.md"),
			content: []byte(
				project.ExampleWorkflowMarkdown(),
			),
			mode:             0o644,
			preserveExisting: true,
		},
	)
	identity := project.Config{
		ID:   request.projectID,
		Name: filepath.Base(request.projectRoot),
	}
	identityBytes, err := yaml.Marshal(identity)
	if err != nil {
		return nil, fmt.Errorf(
			"encode planned project identity: %w",
			err,
		)
	}
	preserveIdentity := existingIdentity != nil
	preserveIdentity = preserveIdentity &&
		existingIdentity.ID == identity.ID &&
		existingIdentity.Name == identity.Name
	inputs = append(inputs, publicCoreFileInput{
		path:             filepath.Join(carrierRoot, "project.yaml"),
		content:          identityBytes,
		mode:             0o644,
		preserveExisting: preserveIdentity,
	})
	gitignorePath := filepath.Join(carrierRoot, ".gitignore")
	gitignore, err := renderPublicCoreGitignore(
		gitignorePath,
		hasLegacyDatabaseSeed,
	)
	if err != nil {
		return nil, err
	}
	inputs = append(inputs, publicCoreFileInput{
		path:    gitignorePath,
		content: gitignore,
		mode:    0o644,
	})
	inputs = append(inputs, profileFiles...)
	return inputs, nil
}

func planPublicLegacyProjectConfig(
	carrierRoot string,
) (publicExactFileEffect, bool, error) {
	path := project.ProjectConfigPath(carrierRoot)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return publicExactFileEffect{}, false, nil
	}
	if err != nil {
		return publicExactFileEffect{}, false,
			fmt.Errorf("inspect legacy project config %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return publicExactFileEffect{}, false,
			fmt.Errorf("legacy project config %s is not a regular file", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return publicExactFileEffect{}, false,
			fmt.Errorf("read legacy project config %s: %w", path, err)
	}
	digest := publicContentDigest(content)
	if project.IsKnownGeneratedLegacyProjectConfigDigest(digest) {
		return publicExactFileEffect{
			kind:           publicExactFileRemove,
			path:           path,
			expectedDigest: digest,
			expectedMode:   info.Mode().Perm(),
		}, true, nil
	}
	preserved, err := planPublicExactFile(
		path,
		content,
		info.Mode().Perm(),
	)
	if err != nil {
		return publicExactFileEffect{}, false, err
	}
	return preserved, true, nil
}

func compilePublicProfileCoreFileInputs(
	ctx context.Context,
	request publicInitRequest,
	ledgerCurrent bool,
	carrierRoot string,
) ([]publicCoreFileInput, error) {
	if !ledgerCurrent {
		return nil, nil
	}
	rawScopeID := ""
	if request.profileScope.kind == publicProfileScopeExact {
		rawScopeID = request.profileScope.scopeID
	}
	scopeRequest, err := projectSpecificationScopeRequestFromFlag(
		rawScopeID,
	)
	if err != nil {
		return nil, err
	}
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		ctx,
		request.projectRoot,
		scopeRequest,
	)
	if err != nil {
		return nil, err
	}
	application, err := planInitProfileApplication(
		resolution,
		scopeRequest,
	)
	if err != nil {
		return nil, err
	}
	if selectionFailure, failed := initProfileSelectionFailure(
		application,
	); failed {
		return nil, selectionFailure
	}
	scopeApplicability, _, resolved := resolution.Resolved()
	if !resolved {
		return nil, nil
	}
	inputs, err := publicRequiredSpecCoreFileInputs(
		carrierRoot,
		scopeApplicability,
	)
	if err != nil {
		return nil, err
	}
	if application.SWEMethodPackKind !=
		projectprofile.CapabilityRequired {
		return inputs, nil
	}
	methodInputs, err := publicMethodCoreFileInputs(
		carrierRoot,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	return append(inputs, methodInputs...), nil
}

func planInitProfileApplication(
	resolution projectSpecificationApplicabilityResolution,
	request projectSpecificationScopeRequest,
) (initProfileApplication, error) {
	publicApplicability, err :=
		publicProjectSpecificationApplicabilityFrom(
			resolution,
			request,
		)
	if err != nil {
		return initProfileApplication{}, err
	}
	application := initProfileApplication{
		Authority:            initProfileApplicationAuthority,
		ProfileApplicability: publicApplicability,
	}
	scopeApplicability, _, resolved := resolution.Resolved()
	if !resolved {
		if !application.valid() {
			return initProfileApplication{}, fmt.Errorf(
				"planned unresolved init profile application is invalid",
			)
		}
		return application, nil
	}
	methodApplicability, err :=
		scopeApplicability.ScopedCapabilityApplicability(
			projectprofile.SWEMethodPackCapability,
		)
	if err != nil {
		return initProfileApplication{}, err
	}
	application.RequiredSpecKinds =
		scopeApplicability.ApplicableDocumentKinds()
	application.SWEMethodPackKind = methodApplicability.Kind()
	if !application.valid() {
		return initProfileApplication{}, fmt.Errorf(
			"planned resolved init profile application is invalid",
		)
	}
	return application, nil
}

func publicRequiredSpecCoreFileInputs(
	haftDir string,
	applicability project.ProjectSpecificationSetApplicability,
) ([]publicCoreFileInput, error) {
	carriers, err := project.RequiredSpecCarriers(applicability)
	if err != nil {
		return nil, err
	}
	inputs := make(
		[]publicCoreFileInput,
		len(carriers),
	)
	for index, carrier := range carriers {
		inputs[index] = publicCoreFileInput{
			path: filepath.Join(
				haftDir,
				carrier.RelativePath,
			),
			content:          []byte(carrier.Content),
			mode:             0o644,
			preserveExisting: true,
		}
	}
	return inputs, nil
}

func publicMethodCoreFileInputs(
	haftDir string,
	materializedAt time.Time,
) ([]publicCoreFileInput, error) {
	catalog := method.BuiltinCatalog()
	root := filepath.Join(
		haftDir,
		"methods",
		catalog.ID,
	)
	methodIDs := make([]string, len(catalog.Methods))
	for index, definition := range catalog.Methods {
		methodIDs[index] = definition.ID
	}
	manifest := method.Manifest{
		CatalogID:           catalog.ID,
		CatalogVersion:      catalog.Version,
		HaftFeature:         "methodpack-v1",
		MaterializedAt:      materializedAt.Format(time.RFC3339),
		LocalOverridePolicy: "preserve_existing_files",
		Methods:             methodIDs,
	}
	manifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	inputs := []publicCoreFileInput{{
		path:             filepath.Join(root, "manifest.yaml"),
		content:          manifestBytes,
		mode:             0o644,
		preserveExisting: true,
	}}
	for _, definition := range catalog.Methods {
		content, err := yaml.Marshal(definition)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, publicCoreFileInput{
			path: filepath.Join(
				root,
				definition.ID+".yaml",
			),
			content:          content,
			mode:             0o644,
			preserveExisting: true,
		})
	}
	return inputs, nil
}

func planPublicCoreFile(
	input publicCoreFileInput,
) (publicExactFileEffect, error) {
	if !input.preserveExisting {
		return planPublicExactFile(
			input.path,
			input.content,
			input.mode,
		)
	}
	info, err := os.Stat(input.path)
	if os.IsNotExist(err) {
		return planPublicExactFile(
			input.path,
			input.content,
			input.mode,
		)
	}
	if err != nil {
		return publicExactFileEffect{}, err
	}
	if !info.Mode().IsRegular() {
		return publicExactFileEffect{}, fmt.Errorf(
			"preserved core carrier %s is not a regular file",
			input.path,
		)
	}
	content, err := os.ReadFile(input.path)
	if err != nil {
		return publicExactFileEffect{}, err
	}
	return planPublicExactFile(
		input.path,
		content,
		info.Mode().Perm(),
	)
}

func renderPublicCoreGitignore(
	path string,
	hasLegacyDatabaseSeed bool,
) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	entries := []string{
		"spec-migration-v2/",
		profileDeclarationReviewFileName,
		projectTypeEnvGenesisReviewFileName,
		".project-typeenv-genesis-review-*",
	}
	if hasLegacyDatabaseSeed {
		entries = append(entries, "haft.db", "quint.db")
	}
	rendered := string(content)
	for _, entry := range entries {
		if strings.Contains(rendered, entry) {
			continue
		}
		if rendered != "" && !strings.HasSuffix(rendered, "\n") {
			rendered += "\n"
		}
		rendered += entry + "\n"
	}
	return []byte(rendered), nil
}

func observePublicLegacyDatabaseSeed(
	projectRoot string,
	canonicalDatabasePath string,
) (string, string, string, bool, error) {
	haftDir := filepath.Join(projectRoot, ".haft")
	quintDir := filepath.Join(projectRoot, ".quint")
	_, haftErr := os.Stat(haftDir)
	haftMissing := os.IsNotExist(haftErr)
	if haftErr != nil && !haftMissing {
		return "", "", "", false, fmt.Errorf(
			"inspect current project carrier root: %w",
			haftErr,
		)
	}
	type candidate struct {
		observed string
		planned  string
	}
	legacy := []candidate{
		{
			observed: filepath.Join(haftDir, "haft.db"),
			planned:  filepath.Join(haftDir, "haft.db"),
		},
		{
			observed: filepath.Join(haftDir, "quint.db"),
			planned:  filepath.Join(haftDir, "quint.db"),
		},
		{
			observed: filepath.Join(quintDir, "quint.db"),
			planned:  filepath.Join(quintDir, "quint.db"),
		},
	}
	if haftMissing {
		legacy[2].planned = filepath.Join(haftDir, "quint.db")
	}
	for _, current := range legacy {
		if current.observed == canonicalDatabasePath {
			continue
		}
		info, err := os.Stat(current.observed)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", "", "", false, fmt.Errorf(
				"inspect legacy project database %s: %w",
				current.observed,
				err,
			)
		}
		if !info.Mode().IsRegular() || info.Size() <= 4096 {
			continue
		}
		digest, err := digestRegularFile(current.observed)
		if err != nil {
			return "", "", "", false, err
		}
		return current.observed,
			current.planned,
			digest,
			true,
			nil
	}
	return "", "", "", false, nil
}

func publicDatabasePresent(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf(
			"inspect public init database path: %w",
			err,
		)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf(
			"public init database path is not a regular file",
		)
	}
	return true, nil
}

func selectPublicCoreEffect(
	observed int,
	compiled int,
) (initplanning.CoreEffectKind, error) {
	switch {
	case observed <= 0:
		return "", fmt.Errorf(
			"existing project schema frontier must be positive",
		)
	case observed < compiled:
		return initplanning.CoreMigrate, nil
	case observed == compiled:
		return initplanning.CoreVerifyCurrent, nil
	default:
		return "", fmt.Errorf(
			"project schema %d is newer than compiled schema %d",
			observed,
			compiled,
		)
	}
}

func observePublicTypeEnvBasis(
	ctx context.Context,
	projectRoot string,
	projectID string,
) (initplanning.BasisReadiness, error) {
	ledger, err := openCurrentProjectLedger(
		ctx,
		projectRoot,
		projectledger.ReadOnly,
		"plan public initialization",
	)
	if err != nil {
		return initplanning.BasisReadiness{}, err
	}
	var headRef string
	var headRevision int64
	var compositeRef string
	queryErr := ledger.Database().
		QueryRowContext(
			ctx,
			`SELECT head_ref, head_revision, selected_composite_ref
			 FROM project_typeenv_heads
			 WHERE project_id = ?`,
			projectID,
		).
		Scan(
			&headRef,
			&headRevision,
			&compositeRef,
		)
	closeErr := ledger.Close()
	if errors.Is(queryErr, sql.ErrNoRows) {
		if closeErr != nil {
			return initplanning.BasisReadiness{}, closeErr
		}
		return initplanning.NewUnavailableBasis(
			"project_basis_unavailable: project memory is not ready",
		)
	}
	if err := errors.Join(queryErr, closeErr); err != nil {
		return initplanning.BasisReadiness{}, fmt.Errorf(
			"observe current project memory basis: %w",
			err,
		)
	}
	return initplanning.NewSelectedBasis(
		headRef,
		headRevision,
		compositeRef,
	)
}

func compilePublicHostInitPlan(
	request publicInitRequest,
	core initplanning.CoreProjectPlan,
	runtime currentHostPublicationRuntime,
	maxCarrierBytes int64,
) (initplanning.InitPlan, error) {
	if len(request.hostBindings) == 0 {
		return initplanning.InitPlan{}, fmt.Errorf(
			"public host initialization requires at least one host binding",
		)
	}
	if request.projectRoot != core.ProjectRoot() ||
		request.projectID != core.ProjectID().String() {
		return initplanning.InitPlan{}, fmt.Errorf(
			"public initialization request and core plan identify different projects",
		)
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		return initplanning.InitPlan{}, err
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		return initplanning.InitPlan{}, err
	}
	candidates, err := currentStandardSkillCandidates(
		request.projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		return initplanning.InitPlan{}, err
	}
	inspector, err := initfs.NewHostStatusInspector(maxCarrierBytes)
	if err != nil {
		return initplanning.InitPlan{}, err
	}
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  request.projectRoot,
			ProjectID:    request.projectID,
			UserHomeRoot: runtime.userHomeRoot,
		},
	)
	if err != nil {
		return initplanning.InitPlan{}, err
	}

	hostPlans := make(
		[]initplanning.HostAdapterInstallPlan,
		0,
		len(request.hostBindings),
	)
	capabilities := make(
		[]initplanning.AdapterCapability,
		0,
		len(request.hostBindings),
	)
	weakHosts := make(
		[]initplanning.WeakHostSelection,
		0,
		len(request.hostBindings),
	)
	for _, binding := range request.hostBindings {
		location, locationErr := layout.ManifestLocation(
			binding.host,
			binding.scope,
		)
		if locationErr != nil {
			return initplanning.InitPlan{}, locationErr
		}
		store, storeErr := initfs.NewManifestStore(
			location.Root(),
			location.Path(),
			maxCarrierBytes,
		)
		if storeErr != nil {
			return initplanning.InitPlan{}, storeErr
		}
		bindingRoot, bindingID, bindingErr :=
			resolvePublicHostBindingOwner(
				request,
				binding,
				store,
			)
		if bindingErr != nil {
			return initplanning.InitPlan{}, bindingErr
		}
		projection, projectionErr := buildPublicHostProjection(
			bindingRoot,
			bindingID,
			binding,
			candidates,
			bundle,
			publication,
			runtime,
		)
		if projectionErr != nil {
			return initplanning.InitPlan{}, projectionErr
		}
		legacy, managedLegacy, legacyErr :=
			currentPublicTakeoverRegistries(projection)
		if legacyErr != nil {
			return initplanning.InitPlan{}, legacyErr
		}
		hostPlan, hostPlanErr := compilePublicHostBindingPlan(
			inspector,
			store,
			projection,
			legacy,
			managedLegacy,
		)
		if hostPlanErr != nil {
			return initplanning.InitPlan{}, hostPlanErr
		}
		capability, capabilityErr :=
			initplanning.NewAdapterCapabilityBuilder(binding.host).
				AtEdition(projection.Edition()).
				Allow(binding.scope, binding.components).
				Build()
		if capabilityErr != nil {
			return initplanning.InitPlan{}, capabilityErr
		}
		hostPlans = append(hostPlans, hostPlan)
		capabilities = append(capabilities, capability)
		weakHosts = append(
			weakHosts,
			weakPublicHostSelection(binding),
		)
	}
	intent, err := initplanning.ParseInitIntent(
		initplanning.WeakInitIntent{
			InvocationPolicy: string(request.invocation),
			ProjectRoot:      request.projectRoot,
			ProjectID:        request.projectID,
			Hosts:            weakHosts,
		},
	)
	if err != nil {
		return initplanning.InitPlan{}, err
	}
	catalog, err := initplanning.NewAdapterCatalog(capabilities)
	if err != nil {
		return initplanning.InitPlan{}, err
	}
	return initplanning.CompileInitPlan(
		intent,
		core,
		hostPlans,
		catalog,
	)
}

func resolvePublicHostBindingOwner(
	request publicInitRequest,
	binding publicHostBinding,
	store initfs.ManifestStore,
) (string, string, error) {
	projectRoot := request.projectRoot
	projectID := request.projectID
	if !isSharedPublicUserSkillBinding(binding) {
		return projectRoot, projectID, nil
	}
	read, err := store.Read()
	if err != nil {
		return "", "", err
	}
	if read.Kind() == initfs.ManifestReadMissing {
		return projectRoot, projectID, nil
	}
	manifest := read.Manifest()
	sameSharedBinding := manifest.Host() == binding.host
	sameSharedBinding = sameSharedBinding &&
		manifest.Scope() == initplanning.ScopeUser
	sameSharedBinding = sameSharedBinding &&
		onlySkillsComponent(manifest.Components())
	if !sameSharedBinding {
		return projectRoot, projectID, nil
	}
	return manifest.ProjectRoot(), manifest.ProjectID(), nil
}

func isSharedPublicUserSkillBinding(
	binding publicHostBinding,
) bool {
	return binding.scope == initplanning.ScopeUser &&
		onlySkillsComponent(binding.components.Values())
}

func compilePublicHostBindingPlan(
	inspector initfs.HostStatusInspector,
	store initfs.ManifestStore,
	projection initplanning.HostAdapterProjection,
	legacy initplanning.LegacyRegistrySelection,
	managedLegacy initplanning.ManagedFragmentLegacyRegistry,
) (initplanning.HostAdapterInstallPlan, error) {
	if len(projection.ManagedFragments()) == 0 {
		currentness, err := inspector.InspectCurrentness(
			store,
			projection,
			legacy,
		)
		if err != nil {
			return initplanning.HostAdapterInstallPlan{}, err
		}
		return initplanning.CompileHostAdapterReconciliation(
			currentness,
		)
	}
	currentness, err := inspector.InspectCoherentCurrentness(
		store,
		projection,
		legacy,
		managedLegacy,
	)
	if err != nil {
		return initplanning.HostAdapterInstallPlan{}, err
	}
	return initplanning.CompileCoherentHostAdapterReconciliation(
		currentness,
	)
}

func buildPublicHostProjection(
	projectRoot string,
	projectID string,
	binding publicHostBinding,
	candidates []currentStandardSkillCandidate,
	bundle initplanning.SkillSourceBundle,
	publication initplanning.PublicationIdentity,
	runtime currentHostPublicationRuntime,
) (initplanning.HostAdapterProjection, error) {
	values := binding.components.Values()
	skillsOnly := len(values) == 1 &&
		values[0] == initplanning.ComponentSkills
	if skillsOnly {
		candidate, found := findCurrentStandardSkillCandidate(
			candidates,
			binding.host,
			binding.scope,
		)
		if !found {
			return initplanning.HostAdapterProjection{}, fmt.Errorf(
				"host binding %s/%s has no standard skill projection",
				binding.host,
				binding.scope,
			)
		}
		return buildCurrentStandardSkillHostProjection(
			projectRoot,
			projectID,
			candidate,
			publication,
		)
	}
	return buildSelectedCurrentCoherentHostProjection(
		projectRoot,
		projectID,
		binding.host,
		binding.scope,
		binding.components,
		candidates,
		bundle,
		publication,
		runtime,
	)
}

func weakPublicHostSelection(
	binding publicHostBinding,
) initplanning.WeakHostSelection {
	components := binding.components.Values()
	names := make([]string, len(components))
	for index, component := range components {
		names[index] = string(component)
	}
	return initplanning.WeakHostSelection{
		Host:       string(binding.host),
		Scope:      string(binding.scope),
		Components: names,
	}
}

func currentPublicTakeoverRegistries(
	projection initplanning.HostAdapterProjection,
) (
	initplanning.LegacyRegistrySelection,
	initplanning.ManagedFragmentLegacyRegistry,
	error,
) {
	whole, basis, err := currentPublicWholeTakeoverRegistry(projection)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.ManagedFragmentLegacyRegistry{},
			err
	}
	fragments := projection.ManagedFragments()
	if len(fragments) == 0 {
		return whole,
			initplanning.NoManagedFragmentLegacyRegistry(),
			nil
	}
	if basis.Kind() == "" {
		basis, err = initplanning.NewOwnershipBasis(
			initplanning.OwnershipLegacyRegistry,
			"known-legacy-managed-fragments:"+
				string(projection.Host())+"/"+
				string(projection.Scope())+":"+
				projection.Edition(),
			projection.Publication().ExecutableDigest(),
		)
		if err != nil {
			return initplanning.LegacyRegistrySelection{},
				initplanning.ManagedFragmentLegacyRegistry{},
				err
		}
	}
	records := make(
		[]initplanning.ManagedFragmentRecord,
		0,
		len(fragments)+1,
	)
	for _, fragment := range fragments {
		records = append(records, fragment.Record())
	}
	legacyCodex, present, err :=
		currentPublicLegacyCodexQuintFragment(projection)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.ManagedFragmentLegacyRegistry{},
			err
	}
	if present {
		records = append(records, legacyCodex.Record())
	}
	legacyPortableCodex, present, err :=
		currentPublicLegacyPortableCodexHaftRecord(projection)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.ManagedFragmentLegacyRegistry{},
			err
	}
	if present {
		records = append(records, legacyPortableCodex)
	}
	legacyPi, err := currentPublicLegacyPiFragments(projection)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.ManagedFragmentLegacyRegistry{},
			err
	}
	for _, fragment := range legacyPi {
		records = append(records, fragment.Record())
	}
	legacyJSON, err := currentPublicLegacyJSONMCPFragments(
		projection,
	)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.ManagedFragmentLegacyRegistry{},
			err
	}
	for _, fragment := range legacyJSON {
		records = append(records, fragment.Record())
	}
	managed, err := initplanning.NewManagedFragmentLegacyRegistry(
		records,
		basis,
	)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.ManagedFragmentLegacyRegistry{},
			err
	}
	return whole, managed, nil
}

func currentPublicLegacyPiFragments(
	projection initplanning.HostAdapterProjection,
) ([]initplanning.ManagedFragment, error) {
	if projection.Host() != initplanning.HostPi ||
		projection.Scope() != initplanning.ScopeProject {
		return nil, nil
	}
	var desired initplanning.ManagedFragment
	found := false
	for _, fragment := range projection.ManagedFragments() {
		coordinate := fragment.Coordinate()
		if coordinate.Kind() != initplanning.ManagedJSONArrayMember ||
			coordinate.Selector() != "/packages" ||
			coordinate.MemberID() != "haft-pi-package" {
			continue
		}
		if found {
			return nil, fmt.Errorf(
				"pi projection repeats its package setting fragment",
			)
		}
		desired = fragment
		found = true
	}
	if !found {
		return nil, fmt.Errorf(
			"pi projection lacks its package setting fragment",
		)
	}
	values := []string{
		piSettingsEntry,
		piLegacySettingsEntry,
	}
	result := make([]initplanning.ManagedFragment, 0, len(values))
	for _, value := range values {
		content, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf(
				"encode legacy Pi package setting: %w",
				err,
			)
		}
		fragment, err := initplanning.NewJSONArrayMemberFragment(
			desired.Coordinate().CarrierPath(),
			desired.Component(),
			[]string{"packages"},
			"haft-pi-package",
			content,
			desired.CreateMode(),
			desired.Coordinate().MergeEdition(),
		)
		if err != nil {
			return nil, err
		}
		if fragment.Digest() == desired.Digest() {
			continue
		}
		result = append(result, fragment)
	}
	return result, nil
}

var publicLegacyCodexQuintFamilyPattern = regexp.MustCompile(
	`(?s)^\[mcp_servers\.quint-code\]\n` +
		`command = "([^"\n]+)"\n` +
		`args = \["serve"\]\n` +
		`startup_timeout_sec = 10\n` +
		`tool_timeout_sec = 60\n\n` +
		`\[mcp_servers\.quint-code\.env\]\n` +
		`QUINT_PROJECT_ROOT = "([^"\n]+)"\n$`,
)

func currentPublicLegacyCodexQuintFragment(
	projection initplanning.HostAdapterProjection,
) (initplanning.ManagedFragment, bool, error) {
	if (projection.Host() != initplanning.HostCodex &&
		projection.Host() != initplanning.HostAir) ||
		projection.Scope() != initplanning.ScopeProject {
		return initplanning.ManagedFragment{}, false, nil
	}
	path := filepath.Join(
		projection.ProjectRoot(),
		".codex",
		"config.toml",
	)
	content := publicLegacyCodexQuintContent(
		"quint-code",
		projection.ProjectRoot(),
	)
	raw, err := os.ReadFile(path)
	if err == nil {
		observed, found, extractErr :=
			initplanning.ExtractTOMLTableFamily(
				raw,
				"mcp_servers.quint-code",
			)
		if extractErr != nil {
			return initplanning.ManagedFragment{}, false,
				fmt.Errorf(
					"inspect legacy Codex MCP configuration: %w",
					extractErr,
				)
		}
		if found && isPublicLegacyCodexQuintFamily(
			observed,
			projection.ProjectRoot(),
		) {
			content = observed
		}
	} else if !os.IsNotExist(err) {
		return initplanning.ManagedFragment{}, false,
			fmt.Errorf(
				"read legacy Codex MCP configuration: %w",
				err,
			)
	}
	fragment, err := initplanning.NewTOMLTableFamilyFragment(
		path,
		initplanning.ComponentMCP,
		"mcp_servers.quint-code",
		content,
		0o644,
		currentTOMLFragmentMergeEdition,
	)
	if err != nil {
		return initplanning.ManagedFragment{}, false, err
	}
	return fragment, true, nil
}

func currentPublicLegacyPortableCodexHaftRecord(
	projection initplanning.HostAdapterProjection,
) (initplanning.ManagedFragmentRecord, bool, error) {
	if (projection.Host() != initplanning.HostCodex &&
		projection.Host() != initplanning.HostAir) ||
		projection.Scope() != initplanning.ScopeProject {
		return initplanning.ManagedFragmentRecord{}, false, nil
	}
	var desired initplanning.ManagedFragment
	found := false
	for _, fragment := range projection.ManagedFragments() {
		coordinate := fragment.Coordinate()
		if coordinate.Kind() != initplanning.ManagedTOMLTableSet ||
			coordinate.Selector() != "mcp_servers.haft" {
			continue
		}
		if found {
			return initplanning.ManagedFragmentRecord{}, false,
				fmt.Errorf(
					"codex projection repeats its Haft MCP table set",
				)
		}
		desired = fragment
		found = true
	}
	if !found {
		return initplanning.ManagedFragmentRecord{}, false,
			fmt.Errorf(
				"codex projection lacks its Haft MCP table set",
			)
	}
	record, err := initplanning.NewKnownLegacyManagedFragmentRecord(
		desired,
		publicLegacyPortableCodexHaftContent(),
	)
	if err != nil {
		return initplanning.ManagedFragmentRecord{}, false, err
	}
	return record, true, nil
}

func publicLegacyPortableCodexHaftContent() []byte {
	return []byte(`[mcp_servers.haft]
command = "haft"
args = ["serve"]
cwd = ".."
env = { HAFT_PROJECT_ROOT = "." }
startup_timeout_sec = 15
tool_timeout_sec = 120
default_tools_approval_mode = "prompt"
`)
}

func publicLegacyCodexQuintContent(
	command string,
	projectRoot string,
) []byte {
	return []byte(fmt.Sprintf(
		`[mcp_servers.quint-code]
command = %q
args = ["serve"]
startup_timeout_sec = 10
tool_timeout_sec = 60

[mcp_servers.quint-code.env]
QUINT_PROJECT_ROOT = %q
`,
		command,
		projectRoot,
	))
}

func isPublicLegacyCodexQuintFamily(
	content []byte,
	projectRoot string,
) bool {
	matches := publicLegacyCodexQuintFamilyPattern.FindSubmatch(
		content,
	)
	if len(matches) != 3 {
		return false
	}
	command := filepath.Clean(string(matches[1]))
	if filepath.Base(command) != "quint-code" {
		return false
	}
	observedRoot := filepath.Clean(string(matches[2]))
	if observedRoot == filepath.Clean(projectRoot) {
		return true
	}
	resolved, err := filepath.EvalSymlinks(observedRoot)
	return err == nil &&
		filepath.Clean(resolved) == filepath.Clean(projectRoot)
}

func currentPublicWholeTakeoverRegistry(
	projection initplanning.HostAdapterProjection,
) (
	initplanning.LegacyRegistrySelection,
	initplanning.OwnershipBasis,
	error,
) {
	outputs := projection.Outputs()
	if len(outputs) == 0 {
		return initplanning.WithoutKnownLegacyRegistry(),
			initplanning.OwnershipBasis{},
			nil
	}
	paths := make(
		[]initplanning.KnownLegacyPath,
		len(outputs),
	)
	for index, output := range outputs {
		digest, err := currentPublicTakeoverDigest(output)
		if err != nil {
			return initplanning.LegacyRegistrySelection{},
				initplanning.OwnershipBasis{},
				err
		}
		paths[index] = initplanning.KnownLegacyPath{
			Path:      output.Path(),
			Component: output.Component(),
			Digest:    digest,
		}
	}
	registry, err := initplanning.BuildKnownLegacyDigestRegistry(
		initplanning.KnownLegacyDigestRegistryInput{
			Edition:     projection.Edition(),
			ProjectRoot: projection.ProjectRoot(),
			ProjectID:   projection.ProjectID().String(),
			Host:        projection.Host(),
			Scope:       projection.Scope(),
			TargetRoots: projection.TargetRoots(),
			Paths:       paths,
		},
	)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.OwnershipBasis{},
			err
	}
	selection, err := initplanning.WithKnownLegacyRegistry(registry)
	if err != nil {
		return initplanning.LegacyRegistrySelection{},
			initplanning.OwnershipBasis{},
			err
	}
	return selection, registry.OwnershipBasis(), nil
}
