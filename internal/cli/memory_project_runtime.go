package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	baseartifacts "github.com/m0n0x41d/haft/data/haft/base-typeenv/artifacts"
	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhoodcache"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

// projectMemoryReadRuntime is the project-bound read-only composition root.
// It has no admission, ProjectTypeEnvHead-selection, Stage-write,
// schema-evolution, decision, commission, or spec-lifecycle port.
type projectMemoryReadRuntime struct {
	projectID  projectidentity.ProjectID
	basis      projectmemory.ProjectBasisSource
	validation projectmemory.ValidationRuntime
	read       projectmemory.CurrentMemoryReadRuntime
}

// projectMemoryRuntime adds the separately composed admission capability used
// only by the guarded CLI admission shell.
type projectMemoryRuntime struct {
	projectMemoryReadRuntime
	admission projectmemory.AdmissionRuntime
	entity    *projectmemory.EntityEstablishmentRuntime
}

type projectMemoryRuntimeBasis struct {
	readOnly         projectMemoryReadRuntime
	selectedRuntime  *projectmemory.ProjectTypeEnvRuntimeResolver
	source           projectmemory.ProjectBasisSource
	snapshotLoader   typedmemorystore.CurrentProjectSnapshotLoader
	target           localpracticeruntime.Target
	targetsByTypeEnv map[string]localpracticeruntime.Target
}

type installedLocalPracticeRuntimeSet struct {
	dispatch         projectmemory.InstalledProjectTypeEnvRuntimeCatalog
	targetsByTypeEnv map[string]localpracticeruntime.Target
}

type projectMemoryAdmissionResponse struct {
	ContractVersion   string                                  `json:"contract_version"`
	Action            string                                  `json:"action"`
	Result            projectmemory.AdmissionResultKind       `json:"result"`
	AuthorityClass    string                                  `json:"authority_class"`
	Validation        any                                     `json:"validation,omitempty"`
	Persistence       projectMemoryAdmissionPersistence       `json:"persistence_disposition"`
	Receipt           *projectMemoryAdmissionReceipt          `json:"receipt,omitempty"`
	PossibleReceipt   *projectMemoryAdmissionReceipt          `json:"possibly_durable_receipt,omitempty"`
	Retry             *projectMemoryAdmissionRetryCoordinates `json:"retry,omitempty"`
	OperationalDetail string                                  `json:"operational_detail,omitempty"`
	Interpretation    projectMemoryAdmissionInterpretation    `json:"interpretation"`
}

type projectMemoryAdmissionPersistence struct {
	Mode             string  `json:"mode"`
	Disposition      string  `json:"disposition,omitempty"`
	RowsWritten      *uint64 `json:"rows_written,omitempty"`
	AuthorityGranted bool    `json:"authority_granted"`
}

type projectMemoryAdmissionReceipt struct {
	EventRef      string `json:"event_ref"`
	CommitRef     string `json:"commit_ref"`
	GraphRevision uint64 `json:"graph_revision"`
	ResultDigest  string `json:"result_digest"`
}

type projectMemoryAdmissionRetryCoordinates struct {
	Kind                  string `json:"kind"`
	ContractVersion       string `json:"contract_version"`
	ProjectID             string `json:"project_id"`
	IdempotencyKey        string `json:"idempotency_key"`
	BasisKind             string `json:"basis_kind"`
	TypeEnvDigest         string `json:"type_env_digest"`
	GraphRevision         uint64 `json:"graph_revision"`
	RequestProvenanceRef  string `json:"request_provenance_ref"`
	CandidateDigest       string `json:"candidate_digest"`
	RequestIdentityDigest string `json:"request_identity_digest"`
	Instruction           string `json:"instruction"`
}

type projectMemoryAdmissionInterpretation struct {
	Establishes      []string `json:"establishes"`
	Omits            []string `json:"omits"`
	DoesNotAuthorize []string `json:"does_not_authorize"`
}

type unavailableProjectMemoryObservableInputProvider struct{}

func (unavailableProjectMemoryObservableInputProvider) LoadObservableInput(
	context.Context,
	projectidentity.ProjectID,
	typedmemory.ObservableInputRef,
	typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	return typedmemorystore.ObservableInputBlob{}, fmt.Errorf(
		"project-memory observable input is not available from the public generic admission surface",
	)
}

func newProjectMemoryRuntime(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	database *sql.DB,
) (projectMemoryRuntime, error) {
	basis, err := buildProjectMemoryRuntimeBasis(
		ctx,
		projectID,
		database,
	)
	if err != nil {
		return projectMemoryRuntime{}, err
	}
	adapter, err := typedmemorystore.NewProjectExecutableGenericSQLiteAdapterBuilder(
		database,
	).
		SetTypeEnvLoader(projectmemory.NewBaseTypeEnvLoader()).
		SetClock(typedmemorystore.SystemClock{}).
		SetReferenceEngine(typedmemorystore.NewExactPersistedStrongReferenceEngine()).
		SetObservableInputs(unavailableProjectMemoryObservableInputProvider{}).
		SetSelectedProjectRuntime(basis.selectedRuntime).
		Build()
	if err != nil {
		return projectMemoryRuntime{}, fmt.Errorf(
			"construct project-memory admission adapter: %w",
			err,
		)
	}
	admission, err := projectmemory.NewAdmissionRuntime(
		projectID,
		basis.source,
		adapter,
	)
	if err != nil {
		return projectMemoryRuntime{}, fmt.Errorf(
			"construct project-memory admission runtime: %w",
			err,
		)
	}
	entity, err := projectmemory.NewEntityEstablishmentRuntime(
		projectID,
		basis.source,
		admission,
		adapter,
	)
	if err != nil {
		return projectMemoryRuntime{}, fmt.Errorf(
			"construct project entity establishment runtime: %w",
			err,
		)
	}
	return projectMemoryRuntime{
		projectMemoryReadRuntime: basis.readOnly,
		admission:                admission,
		entity:                   entity,
	}, nil
}

func newProjectMemoryReadRuntime(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	database *sql.DB,
) (projectMemoryReadRuntime, error) {
	basis, err := buildProjectMemoryRuntimeBasis(
		ctx,
		projectID,
		database,
	)
	if err != nil {
		return projectMemoryReadRuntime{}, err
	}
	return basis.readOnly, nil
}

func buildProjectMemoryRuntimeBasis(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	database *sql.DB,
) (projectMemoryRuntimeBasis, error) {
	sources, err := installedLocalPracticeSources()
	if err != nil {
		return projectMemoryRuntimeBasis{}, err
	}
	return buildProjectMemoryRuntimeBasisAtSources(
		ctx,
		projectID,
		database,
		sources,
	)
}

func buildProjectMemoryRuntimeBasisAtSources(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	database *sql.DB,
	installedSources [][]byte,
) (projectMemoryRuntimeBasis, error) {
	if ctx == nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct project-memory runtime: context is required",
		)
	}
	if database == nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct project-memory runtime: database is required",
		)
	}
	canonicalProjectID, err := projectidentity.ParseProjectID(projectID.String())
	if err != nil || canonicalProjectID != projectID {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct project-memory runtime: exact project identity is required",
		)
	}
	if err := db.RequireCurrentSchemaReadOnly(ctx, database); err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct project-memory runtime: %w",
			err,
		)
	}
	base, err := loadEmbeddedMemoryRuntime(ctx)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct project-memory base runtime: %w",
			err,
		)
	}
	target, err := localpracticeruntime.Build(
		base.Artifact(),
		typedmemorycandidates.SourceV1_4(),
	)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct installed typed-memory Local-Practice runtime: %w",
			err,
		)
	}
	artifacts, err := projecttypeenvstore.OpenReadOnly(ctx, database)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct installed typed-memory artifact reader: %w",
			err,
		)
	}
	installedRuntimes, err := buildInstalledLocalPracticeRuntimeCatalog(
		ctx,
		artifacts,
		target,
		installedSources,
	)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct installed typed-memory runtime catalog: %w",
			err,
		)
	}
	stages, err := projecttypeenvstage.OpenReadOnly(ctx, database)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct existing project-memory Stage store: %w",
			err,
		)
	}
	heads, err := projecttypeenvheadstore.OpenReadOnly(ctx, database)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct existing project-memory head store: %w",
			err,
		)
	}
	selectedRuntime, err := projectmemory.NewProjectTypeEnvRuntimeResolver(
		stages,
		heads,
		sqlite.NewCurrentCommittedClosureLoader(),
		installedRuntimes.dispatch,
	)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct selected project TypeEnv runtime resolver: %w",
			err,
		)
	}
	ledgerProjectID, err := projectledger.ParseProjectID(projectID.String())
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct project-memory read identity: %w",
			err,
		)
	}
	readFrameLoader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectReadFrameLoader(
			database,
			projectmemory.NewBaseTypeEnvLoader(),
			selectedRuntime,
		)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct current project-memory read frame: %w",
			err,
		)
	}
	cacheStore := neighborhoodcache.NewAtomicStore()
	readCache, err := neighborhoodcache.NewShell(cacheStore)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct current project-memory neighborhood cache: %w",
			err,
		)
	}
	identitySource, err :=
		identityreconciliation.NewSQLiteCommittedResolutionStateSource(database)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct current project-memory identity resolution source: %w",
			err,
		)
	}
	readRuntime, err :=
		projectmemory.NewCurrentMemoryReadRuntimeWithNeighborhoodCacheAndIdentityReconciliation(
			ledgerProjectID,
			readFrameLoader,
			readCache,
			identitySource,
		)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct current project-memory read runtime: %w",
			err,
		)
	}
	loader, err := typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		database,
		projectmemory.NewBaseTypeEnvLoader(),
		selectedRuntime,
	)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct current project-memory snapshot loader: %w",
			err,
		)
	}
	source, err := projectmemory.NewCurrentProjectBasisSource(loader)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct current project-memory basis source: %w",
			err,
		)
	}
	validation, err := projectmemory.NewValidationRuntime(projectID, source)
	if err != nil {
		return projectMemoryRuntimeBasis{}, fmt.Errorf(
			"construct project-memory validation runtime: %w",
			err,
		)
	}
	return projectMemoryRuntimeBasis{
		readOnly: projectMemoryReadRuntime{
			projectID:  projectID,
			basis:      source,
			validation: validation,
			read:       readRuntime,
		},
		selectedRuntime:  selectedRuntime,
		source:           source,
		snapshotLoader:   loader,
		target:           target,
		targetsByTypeEnv: installedRuntimes.targetsByTypeEnv,
	}, nil
}

func installedLocalPracticeSources() ([][]byte, error) {
	known := [][]byte{
		typedmemorycandidates.SourceV1(),
		typedmemorycandidates.SourceV1_1(),
		typedmemorycandidates.SourceV1_2(),
		typedmemorycandidates.SourceV1_3(),
		typedmemorycandidates.SourceV1_4(),
	}
	result := make([][]byte, 0, len(known))
	for index, source := range known {
		parsed, err := localpractice.Parse(source)
		if err != nil {
			return nil, fmt.Errorf(
				"parse installed Local-Practice source %d: %w",
				index,
				err,
			)
		}
		base := parsed.Carrier().BaseTypeEnvRef().Value()
		matched := typedmemorycandidates.SourcesForExactBaseTypeEnvRef(base)
		if len(matched) != 1 {
			return nil, fmt.Errorf(
				"installed Local-Practice base %q resolves %d sources; want exactly 1",
				base,
				len(matched),
			)
		}
		result = append(result, matched[0])
	}
	return result, nil
}

func buildInstalledLocalPracticeRuntimeCatalog(
	ctx context.Context,
	artifacts *projecttypeenvstore.Store,
	current localpracticeruntime.Target,
	sources [][]byte,
) (installedLocalPracticeRuntimeSet, error) {
	currentEntry, err := projectmemory.NewInstalledProjectTypeEnvRuntimeEntry(
		current.RuntimeBasis(),
		current.InstalledRuntime(),
	)
	if err != nil {
		return installedLocalPracticeRuntimeSet{}, err
	}
	entries := []projectmemory.InstalledProjectTypeEnvRuntimeEntry{currentEntry}
	seen := map[string]struct{}{
		current.RuntimeBasis().Ref().String(): {},
	}
	targetsByTypeEnv := map[string]localpracticeruntime.Target{
		current.Composite().Ref().String(): current,
	}
	currentBase, present := current.Base().TypeEnvRef()
	if !present {
		return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
			"current Local-Practice target has no executable B",
		)
	}
	for index, source := range sources {
		parsed, parseErr := localpractice.Parse(source)
		if parseErr != nil {
			return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
				"parse installed Local-Practice runtime source %d: %w",
				index,
				parseErr,
			)
		}
		baseRef, parseRefErr := typedmemory.ParseTypeEnvRef(
			parsed.Carrier().BaseTypeEnvRef().Value(),
		)
		if parseRefErr != nil {
			return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
				"parse installed Local-Practice runtime source %d base: %w",
				index,
				parseRefErr,
			)
		}
		base, available, loadErr := loadInstalledLocalPracticeBase(
			ctx,
			artifacts,
			current,
			currentBase,
			baseRef,
		)
		if loadErr != nil {
			return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
				"load installed Local-Practice runtime source %d B: %w",
				index,
				loadErr,
			)
		}
		if !available {
			continue
		}
		candidate, buildErr := localpracticeruntime.Build(base, source)
		if buildErr != nil {
			return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
				"build installed Local-Practice runtime source %d: %w",
				index,
				buildErr,
			)
		}
		ref := candidate.RuntimeBasis().Ref().String()
		if _, duplicate := seen[ref]; duplicate {
			compositeRef := candidate.Composite().Ref().String()
			existing, exact := targetsByTypeEnv[compositeRef]
			if !exact || existing.RuntimeBasis().Ref().String() != ref {
				return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
					"installed Local-Practice runtime source %d duplicates X %q with a different C",
					index,
					ref,
				)
			}
			continue
		}
		entry, entryErr := projectmemory.NewInstalledProjectTypeEnvRuntimeEntry(
			candidate.RuntimeBasis(),
			candidate.InstalledRuntime(),
		)
		if entryErr != nil {
			return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
				"bind installed Local-Practice runtime source %d: %w",
				index,
				entryErr,
			)
		}
		compositeRef := candidate.Composite().Ref().String()
		if _, duplicate := targetsByTypeEnv[compositeRef]; duplicate {
			return installedLocalPracticeRuntimeSet{}, fmt.Errorf(
				"installed Local-Practice runtime source %d duplicates C %q",
				index,
				compositeRef,
			)
		}
		seen[ref] = struct{}{}
		entries = append(entries, entry)
		targetsByTypeEnv[compositeRef] = candidate
	}
	dispatch, err := projectmemory.NewInstalledProjectTypeEnvRuntimeCatalog(entries)
	if err != nil {
		return installedLocalPracticeRuntimeSet{}, err
	}
	return installedLocalPracticeRuntimeSet{
		dispatch:         dispatch,
		targetsByTypeEnv: targetsByTypeEnv,
	}, nil
}

func loadInstalledLocalPracticeBase(
	ctx context.Context,
	artifacts *projecttypeenvstore.Store,
	current localpracticeruntime.Target,
	currentBase typedmemory.TypeEnvRef,
	requested typedmemory.TypeEnvRef,
) (typeenv.BaseTypeEnvArtifact, bool, error) {
	if requested == currentBase {
		return current.Base(), true, nil
	}
	stored, storedErr := artifacts.GetBaseTypeEnvArtifact(ctx, requested)
	if storedErr != nil && !errors.Is(
		storedErr,
		projecttypeenvstore.ErrArtifactNotFound,
	) {
		return typeenv.BaseTypeEnvArtifact{}, false, storedErr
	}
	archived, archivedErr := baseartifacts.LoadExact(requested)
	if archivedErr != nil && !errors.Is(
		archivedErr,
		baseartifacts.ErrExactArtifactNotFound,
	) {
		return typeenv.BaseTypeEnvArtifact{}, false, archivedErr
	}
	if storedErr == nil {
		if archivedErr == nil && !bytes.Equal(
			stored.CanonicalBytes(),
			archived.CanonicalBytes(),
		) {
			return typeenv.BaseTypeEnvArtifact{}, false, fmt.Errorf(
				"project and archived Base TypeEnv bytes differ for %s",
				requested.String(),
			)
		}
		return stored, true, nil
	}
	if archivedErr == nil {
		return archived, true, nil
	}
	return typeenv.BaseTypeEnvArtifact{}, false, nil
}

func (runtime projectMemoryReadRuntime) Validate(
	ctx context.Context,
	payload []byte,
) ([]byte, error) {
	request, err := typedmemorywire.DecodeValidateRequest(payload)
	if err != nil {
		return nil, err
	}
	return runtime.validateDecoded(ctx, request)
}

func (runtime projectMemoryReadRuntime) validateDecoded(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) ([]byte, error) {
	response, err := runtime.validation.Validate(ctx, request)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("encode project-memory validation response: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (runtime projectMemoryRuntime) Admit(
	ctx context.Context,
	request typedmemorywire.AdmitRequest,
) ([]byte, error) {
	result, err := runtime.admission.Admit(ctx, request)
	if err != nil {
		return nil, err
	}
	response, err := presentProjectMemoryAdmission(
		result,
		request.AuthorityClass(),
	)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("encode project-memory admission response: %w", err)
	}
	return append(encoded, '\n'), nil
}

// ValidationMCPHandler exposes only read-only validation. Admission is a
// separate capability and cannot be reached by hiding an action from the MCP
// schema while retaining a broader handler behind it.
func (runtime projectMemoryReadRuntime) ValidationMCPHandler() fpf.MemoryToolHandler {
	return func(ctx context.Context, arguments json.RawMessage) (string, error) {
		result, err := runtime.Validate(ctx, arguments)
		if err != nil {
			return "", err
		}
		return string(result), nil
	}
}

// ReadOnlyMCPHandler exposes the internal typed-memory action union used by
// dedicated CLI reads. The public MCP adapter is ReadOnlyQueryMCPHandler.
func (runtime projectMemoryReadRuntime) ReadOnlyMCPHandler() fpf.MemoryToolHandler {
	return func(ctx context.Context, arguments json.RawMessage) (string, error) {
		request, err := typedmemorywire.DecodeRequest(arguments)
		if err != nil {
			return "", err
		}
		result, err := runtime.ExecuteReadOnlyRequest(ctx, request)
		if err != nil {
			return "", err
		}
		return string(result), nil
	}
}

// ReadOnlyQueryMCPHandler exposes the same sealed read result union through the
// public haft_query(action="memory", memory_request={...}) wrapper. The strict
// query decoder receives the original JSON bytes and rejects duplicate,
// legacy-flat, unknown, or cross-variant fields before translation.
func (runtime projectMemoryReadRuntime) ReadOnlyQueryMCPHandler() fpf.MemoryToolHandler {
	return func(ctx context.Context, arguments json.RawMessage) (string, error) {
		request, err := typedmemorywire.DecodeQueryReadRequest(arguments)
		if err != nil {
			return "", err
		}
		result, err := runtime.ExecuteReadOnlyRequest(ctx, request)
		if err != nil {
			return "", err
		}
		return string(result), nil
	}
}

func (runtime projectMemoryReadRuntime) ExecuteReadOnlyRequest(
	ctx context.Context,
	request typedmemorywire.Request,
) ([]byte, error) {
	if request == nil {
		return nil, fmt.Errorf("decoded read-only memory request is required")
	}
	switch exact := request.(type) {
	case typedmemorywire.ValidateRequest:
		return runtime.validateDecoded(ctx, exact)
	case typedmemorywire.ResolveReadRequest:
		recovery, unavailable, err :=
			runtime.projectMemoryReadRecovery(ctx, exact.Action())
		if err != nil || unavailable {
			return recovery, err
		}
		result, err := runtime.ResolveRead(ctx, exact)
		if err != nil {
			return nil, err
		}
		return projectmemory.EncodeResolutionReadResponse(result)
	case typedmemorywire.NeighborhoodReadRequest:
		recovery, unavailable, err :=
			runtime.projectMemoryReadRecovery(ctx, exact.Action())
		if err != nil || unavailable {
			return recovery, err
		}
		result, err := runtime.NeighborhoodRead(ctx, exact)
		if err != nil {
			return nil, err
		}
		return projectmemory.EncodeNeighborhoodReadResponse(result)
	case typedmemorywire.RecallReadRequest:
		recovery, unavailable, err :=
			runtime.projectMemoryReadRecovery(ctx, exact.Action())
		if err != nil || unavailable {
			return recovery, err
		}
		result, err := runtime.RecallRead(ctx, exact)
		if err != nil {
			return nil, err
		}
		return projectmemory.EncodeScopedRecallReadResponse(result)
	default:
		return nil, fmt.Errorf(
			"haft_memory action %q is not available on the read-only boundary",
			request.Action(),
		)
	}
}

func (runtime projectMemoryReadRuntime) projectMemoryReadRecovery(
	ctx context.Context,
	mode string,
) ([]byte, bool, error) {
	available, err := runtime.ProjectBasisAvailable(ctx)
	if err != nil {
		return nil, false, err
	}
	if available {
		return nil, false, nil
	}
	response, err := projectMemoryUnavailableReadResponse(mode)
	return response, true, err
}

func presentProjectMemoryAdmission(
	result projectmemory.AdmissionResult,
	authorityClass string,
) (projectMemoryAdmissionResponse, error) {
	if result == nil {
		return projectMemoryAdmissionResponse{}, fmt.Errorf(
			"present project-memory admission: unsupported result variant <nil>",
		)
	}
	contractVersion := result.ContractVersion()
	if contractVersion != typedmemorywire.ContractVersionV1 &&
		contractVersion != typedmemorywire.ContractVersionV2 {
		return projectMemoryAdmissionResponse{}, fmt.Errorf(
			"present project-memory admission: unsupported contract version %q",
			contractVersion,
		)
	}
	switch exact := result.(type) {
	case projectmemory.AdmissionNotAdmitted:
		rows := uint64(0)
		return projectMemoryAdmissionResponse{
			ContractVersion: contractVersion,
			Action:          typedmemorywire.ActionAdmit,
			Result:          exact.Kind(),
			AuthorityClass:  authorityClass,
			Validation:      exact.ValidationResponse(),
			Persistence: projectMemoryAdmissionPersistence{
				Mode:             "not_admitted_no_write",
				RowsWritten:      &rows,
				AuthorityGranted: false,
			},
			Interpretation: projectMemoryAdmissionInterpretation{
				Establishes: []string{
					"the supplied candidate was not committed",
					"the nested validation response explains the current semantic result",
				},
				Omits: []string{
					"no project-memory mutation occurred",
					"no applicability, truth, evidence, completion, or recommendation claim was established",
				},
				DoesNotAuthorize: []string{
					"changing the exact basis or semantic request under the same idempotency key",
					"decision, commission, specification lifecycle, schema, or ProjectTypeEnvHead mutation",
				},
			},
		}, nil
	case projectmemory.AdmissionCommitted:
		receipt := exact.Receipt()
		return projectMemoryAdmissionResponse{
			ContractVersion: contractVersion,
			Action:          typedmemorywire.ActionAdmit,
			Result:          exact.Kind(),
			AuthorityClass:  authorityClass,
			Persistence: projectMemoryAdmissionPersistence{
				Mode:             "transactional_project_memory_commit",
				Disposition:      string(receipt.Disposition()),
				AuthorityGranted: false,
			},
			Receipt: &projectMemoryAdmissionReceipt{
				EventRef:      receipt.EventRef(),
				CommitRef:     receipt.CommitRef(),
				GraphRevision: receipt.GraphRevision().Value(),
				ResultDigest:  receipt.ResultDigest().String(),
			},
			Interpretation: projectMemoryAdmissionInterpretation{
				Establishes: []string{
					"the exact non-binding semantic assertion batch is durable at the receipt coordinates",
					"replay means the same durable result was recovered without a second write",
				},
				Omits: []string{
					"validity does not establish truth, evidence quality, applicability outside the exact ContextSlice, completion, or recommendation",
					"the receipt does not say that a graph node or document acts",
				},
				DoesNotAuthorize: []string{
					"decision, commission, specification lifecycle, schema, or ProjectTypeEnvHead mutation",
					"reinterpretation of retrieval rank, graph direction, or timestamp as causal or work order",
				},
			},
		}, nil
	case projectmemory.AdmissionCommitOutcomeUnknown:
		retry := presentProjectMemoryAdmissionRetry(
			exact.RetryCoordinates(),
		)
		response := projectMemoryAdmissionResponse{
			ContractVersion: contractVersion,
			Action:          typedmemorywire.ActionAdmit,
			Result:          exact.Kind(),
			AuthorityClass:  authorityClass,
			Persistence: projectMemoryAdmissionPersistence{
				Mode:             "commit_outcome_unknown",
				AuthorityGranted: false,
			},
			Retry:             &retry,
			OperationalDetail: exact.OperationalDetail(),
			Interpretation: projectMemoryAdmissionInterpretation{
				Establishes: []string{
					"the admission runtime could not prove whether the exact semantic transaction committed or rolled back",
					"the retry coordinates identify the same canonical admission request independently of JSON whitespace or object-field order",
				},
				Omits: []string{
					"the result does not establish that the semantic transaction committed",
					"the result does not establish that the semantic transaction rolled back",
					"the result does not establish truth, evidence quality, applicability outside the exact ContextSlice, completion, or recommendation",
				},
				DoesNotAuthorize: []string{
					"retrying with changed project, basis, content, provenance, or idempotency coordinates",
					"decision, commission, specification lifecycle, schema, or ProjectTypeEnvHead mutation",
				},
			},
		}
		possibleReceipt, present := exact.PossibleReceipt()
		if present {
			presented := presentProjectMemoryAdmissionReceipt(possibleReceipt)
			response.PossibleReceipt = &presented
			response.Interpretation.Omits = append(
				response.Interpretation.Omits,
				"the possibly durable receipt is preserved as returned data but does not resolve the unknown outcome",
			)
		}
		return response, nil
	default:
		return projectMemoryAdmissionResponse{}, fmt.Errorf(
			"present project-memory admission: unsupported result variant %T",
			result,
		)
	}
}

func presentProjectMemoryAdmissionReceipt(
	receipt typedmemorystore.CommitReceipt,
) projectMemoryAdmissionReceipt {
	return projectMemoryAdmissionReceipt{
		EventRef:      receipt.EventRef(),
		CommitRef:     receipt.CommitRef(),
		GraphRevision: receipt.GraphRevision().Value(),
		ResultDigest:  receipt.ResultDigest().String(),
	}
}

func presentProjectMemoryAdmissionRetry(
	coordinates projectmemory.AdmissionRetryCoordinates,
) projectMemoryAdmissionRetryCoordinates {
	return projectMemoryAdmissionRetryCoordinates{
		Kind:                  "replay_exact_admission",
		ContractVersion:       coordinates.ContractVersion().String(),
		ProjectID:             coordinates.ProjectID().String(),
		IdempotencyKey:        coordinates.IdempotencyKey().String(),
		BasisKind:             string(coordinates.BasisKind()),
		TypeEnvDigest:         coordinates.TypeEnv().String(),
		GraphRevision:         coordinates.GraphRevision().Value(),
		RequestProvenanceRef:  coordinates.RequestProvenance().String(),
		CandidateDigest:       coordinates.CandidateDigest().String(),
		RequestIdentityDigest: coordinates.RequestIdentityDigest().String(),
		Instruction:           "replay the unchanged exact admission request with the same project and idempotency key",
	}
}
