package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var ErrSelectedProjectTypeEnvRuntimeResolverRequired = errors.New(
	"typed-memory selected project TypeEnv runtime resolver is required",
)

var ErrSelectedMemberOfRuntimeNotSerializable = errors.New(
	"selected MemberOf runtime is an in-process capability and cannot be serialized",
)

var ErrSelectedKindClassificationRuntimeNotSerializable = errors.New(
	"selected kind-classification runtime is an in-process capability and cannot be serialized",
)

type typeEnvRepresentationKind uint8

const (
	typeEnvRepresentationGenericSnapshot typeEnvRepresentationKind = iota + 1
	typeEnvRepresentationProjectExecutable
)

// SelectedProjectTypeEnvRuntimeRequest is minted from the exact graph head
// observed by typedmemorystore inside the caller-owned transaction. The
// resolver may inspect the coordinates but cannot construct or alter them.
type SelectedProjectTypeEnvRuntimeRequest struct {
	project       projectledger.ProjectID
	graphRevision typedmemory.GraphRevision
	selected      typedmemory.TypeEnvRef
}

func (request SelectedProjectTypeEnvRuntimeRequest) Project() projectledger.ProjectID {
	return request.project
}

func (request SelectedProjectTypeEnvRuntimeRequest) GraphRevision() typedmemory.GraphRevision {
	return request.graphRevision
}

func (request SelectedProjectTypeEnvRuntimeRequest) SelectedTypeEnv() typedmemory.TypeEnvRef {
	return request.selected
}

// SelectedMemberOfRuntime is the closed transaction-time membership posture
// of one selected project TypeEnv runtime. A selected C either requires no
// executable MemberOf mechanism or carries one exact evaluator correlated to
// both C's X and the matched target-registry coordinate.
type SelectedMemberOfRuntime interface {
	selectedMemberOfRuntimeVariant()
}

// MemberOfNotRequired states that the selected X contains neither a MemberOf
// mechanism pin nor a record-membership registration-policy pin.
type MemberOfNotRequired struct{}

func NewMemberOfNotRequired() MemberOfNotRequired {
	return MemberOfNotRequired{}
}

func (MemberOfNotRequired) selectedMemberOfRuntimeVariant() {}

func (MemberOfNotRequired) MarshalJSON() ([]byte, error) {
	return nil, ErrSelectedMemberOfRuntimeNotSerializable
}

func (*MemberOfNotRequired) UnmarshalJSON([]byte) error {
	return ErrSelectedMemberOfRuntimeNotSerializable
}

// ExactMemberOfRuntimeInput is the complete proof material required to bind a
// callable evaluator to one selected X and one exact target-runtime registry.
type ExactMemberOfRuntimeInput struct {
	Engine                   MemberOfEvaluationEngine
	RuntimeBasisDigest       SelectedRuntimeBasisDigest
	RegistryCoordinateDigest ExactTargetRegistryCoordinateDigest
}

// SelectedRuntimeBasisDigest is the typed identity coordinate of the exact X
// used to construct a selected runtime. It cannot be exchanged accidentally
// with the matched registry coordinate.
type SelectedRuntimeBasisDigest struct {
	digest typedmemory.SHA256Digest
}

func NewSelectedRuntimeBasisDigest(
	digest typedmemory.SHA256Digest,
) (SelectedRuntimeBasisDigest, error) {
	if err := verifyExactDigest(digest, "runtime-basis X"); err != nil {
		return SelectedRuntimeBasisDigest{}, err
	}
	return SelectedRuntimeBasisDigest{digest: digest}, nil
}

func (digest SelectedRuntimeBasisDigest) Digest() typedmemory.SHA256Digest {
	return digest.digest
}

// ExactTargetRegistryCoordinateDigest is the typed identity coordinate of the
// target runtime registry matched to the selected X.
type ExactTargetRegistryCoordinateDigest struct {
	digest typedmemory.SHA256Digest
}

func NewExactTargetRegistryCoordinateDigest(
	digest typedmemory.SHA256Digest,
) (ExactTargetRegistryCoordinateDigest, error) {
	if err := verifyExactDigest(digest, "target-registry coordinate"); err != nil {
		return ExactTargetRegistryCoordinateDigest{}, err
	}
	return ExactTargetRegistryCoordinateDigest{digest: digest}, nil
}

func (digest ExactTargetRegistryCoordinateDigest) Digest() typedmemory.SHA256Digest {
	return digest.digest
}

// ExactMemberOfRuntime is an executable, non-serializable selected-C
// capability. The digests are identity coordinates, not evidence that an
// evaluation has already been performed.
type ExactMemberOfRuntime struct {
	engine                   MemberOfEvaluationEngine
	runtimeBasisDigest       SelectedRuntimeBasisDigest
	registryCoordinateDigest ExactTargetRegistryCoordinateDigest
}

func NewExactMemberOfRuntime(
	input ExactMemberOfRuntimeInput,
) (ExactMemberOfRuntime, error) {
	if !memberOfEvaluationEngineIsPresent(input.Engine) {
		return ExactMemberOfRuntime{}, fmt.Errorf(
			"exact MemberOf runtime requires an executable evaluator",
		)
	}
	if err := verifyExactDigest(
		input.RuntimeBasisDigest.Digest(),
		"runtime-basis X",
	); err != nil {
		return ExactMemberOfRuntime{}, err
	}
	if err := verifyExactDigest(
		input.RegistryCoordinateDigest.Digest(),
		"target-registry coordinate",
	); err != nil {
		return ExactMemberOfRuntime{}, err
	}
	return ExactMemberOfRuntime{
		engine:                   input.Engine,
		runtimeBasisDigest:       input.RuntimeBasisDigest,
		registryCoordinateDigest: input.RegistryCoordinateDigest,
	}, nil
}

func (ExactMemberOfRuntime) selectedMemberOfRuntimeVariant() {}

func (ExactMemberOfRuntime) MarshalJSON() ([]byte, error) {
	return nil, ErrSelectedMemberOfRuntimeNotSerializable
}

func (*ExactMemberOfRuntime) UnmarshalJSON([]byte) error {
	return ErrSelectedMemberOfRuntimeNotSerializable
}

func (runtime ExactMemberOfRuntime) RuntimeBasisDigest() SelectedRuntimeBasisDigest {
	return runtime.runtimeBasisDigest
}

func (runtime ExactMemberOfRuntime) RegistryCoordinateDigest() ExactTargetRegistryCoordinateDigest {
	return runtime.registryCoordinateDigest
}

// SelectedKindClassificationRuntime is the current C.3.2 transaction-time
// posture. It remains separate from the historical MemberOf runtime so a
// selected C cannot silently substitute one semantics for the other.
type SelectedKindClassificationRuntime interface {
	selectedKindClassificationRuntimeVariant()
}

type KindClassificationNotRequired struct{}

func NewKindClassificationNotRequired() KindClassificationNotRequired {
	return KindClassificationNotRequired{}
}

func (KindClassificationNotRequired) selectedKindClassificationRuntimeVariant() {}

func (KindClassificationNotRequired) MarshalJSON() ([]byte, error) {
	return nil, ErrSelectedKindClassificationRuntimeNotSerializable
}

func (*KindClassificationNotRequired) UnmarshalJSON([]byte) error {
	return ErrSelectedKindClassificationRuntimeNotSerializable
}

type ExactKindClassificationRuntimeInput struct {
	Engine                   KindClassificationAdmissionEngine
	RuntimeBasisDigest       SelectedRuntimeBasisDigest
	RegistryCoordinateDigest ExactTargetRegistryCoordinateDigest
}

type ExactKindClassificationRuntime struct {
	engine                   KindClassificationAdmissionEngine
	runtimeBasisDigest       SelectedRuntimeBasisDigest
	registryCoordinateDigest ExactTargetRegistryCoordinateDigest
}

func NewExactKindClassificationRuntime(
	input ExactKindClassificationRuntimeInput,
) (ExactKindClassificationRuntime, error) {
	if !kindClassificationAdmissionEngineIsPresent(input.Engine) {
		return ExactKindClassificationRuntime{}, fmt.Errorf(
			"exact kind-classification runtime requires an executable evaluator",
		)
	}
	if err := verifyExactDigest(
		input.RuntimeBasisDigest.Digest(),
		"runtime-basis X",
	); err != nil {
		return ExactKindClassificationRuntime{}, err
	}
	if err := verifyExactDigest(
		input.RegistryCoordinateDigest.Digest(),
		"target-registry coordinate",
	); err != nil {
		return ExactKindClassificationRuntime{}, err
	}
	return ExactKindClassificationRuntime{
		engine:                   input.Engine,
		runtimeBasisDigest:       input.RuntimeBasisDigest,
		registryCoordinateDigest: input.RegistryCoordinateDigest,
	}, nil
}

func (ExactKindClassificationRuntime) selectedKindClassificationRuntimeVariant() {}

func (ExactKindClassificationRuntime) MarshalJSON() ([]byte, error) {
	return nil, ErrSelectedKindClassificationRuntimeNotSerializable
}

func (*ExactKindClassificationRuntime) UnmarshalJSON([]byte) error {
	return ErrSelectedKindClassificationRuntimeNotSerializable
}

func (runtime ExactKindClassificationRuntime) RuntimeBasisDigest() SelectedRuntimeBasisDigest {
	return runtime.runtimeBasisDigest
}

func (runtime ExactKindClassificationRuntime) RegistryCoordinateDigest() ExactTargetRegistryCoordinateDigest {
	return runtime.registryCoordinateDigest
}

// SelectedProjectTypeEnvRuntime is the exact executable result returned by a
// trusted outer adapter after it has reconstructed the selected C and matched
// C's X against the installed process runtime. Construction binds the result
// back to the transaction-local request; it does not select a head or grant
// admission authority.
type SelectedProjectTypeEnvRuntime struct {
	request        SelectedProjectTypeEnvRuntimeRequest
	environment    typedmemory.TypeEnv
	codecs         typedmemory.CodecRegistry
	memberOf       SelectedMemberOfRuntime
	classification SelectedKindClassificationRuntime
}

// SelectedProjectTypeEnvRuntimeBuilder starts a typestate construction chain.
// Each required component advances to a distinct builder type, so Build is
// unavailable until request, environment, codecs, and membership posture are
// all explicit.
type SelectedProjectTypeEnvRuntimeBuilder struct {
	request SelectedProjectTypeEnvRuntimeRequest
}

func NewSelectedProjectTypeEnvRuntimeBuilder(
	request SelectedProjectTypeEnvRuntimeRequest,
) SelectedProjectTypeEnvRuntimeBuilder {
	return SelectedProjectTypeEnvRuntimeBuilder{request: request}
}

type selectedProjectTypeEnvRuntimeEnvironmentBuilder struct {
	request     SelectedProjectTypeEnvRuntimeRequest
	environment typedmemory.TypeEnv
}

func (builder SelectedProjectTypeEnvRuntimeBuilder) SetEnvironment(
	environment typedmemory.TypeEnv,
) selectedProjectTypeEnvRuntimeEnvironmentBuilder {
	return selectedProjectTypeEnvRuntimeEnvironmentBuilder{
		request:     builder.request,
		environment: environment,
	}
}

type selectedProjectTypeEnvRuntimeCodecsBuilder struct {
	request     SelectedProjectTypeEnvRuntimeRequest
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
}

func (builder selectedProjectTypeEnvRuntimeEnvironmentBuilder) SetCodecs(
	codecs typedmemory.CodecRegistry,
) selectedProjectTypeEnvRuntimeCodecsBuilder {
	return selectedProjectTypeEnvRuntimeCodecsBuilder{
		request:     builder.request,
		environment: builder.environment,
		codecs:      codecs,
	}
}

type selectedProjectTypeEnvRuntimeReadyBuilder struct {
	request        SelectedProjectTypeEnvRuntimeRequest
	environment    typedmemory.TypeEnv
	codecs         typedmemory.CodecRegistry
	memberOf       SelectedMemberOfRuntime
	classification SelectedKindClassificationRuntime
}

func (builder selectedProjectTypeEnvRuntimeCodecsBuilder) SetMemberOfRuntime(
	memberOf SelectedMemberOfRuntime,
) selectedProjectTypeEnvRuntimeReadyBuilder {
	return selectedProjectTypeEnvRuntimeReadyBuilder{
		request:        builder.request,
		environment:    builder.environment,
		codecs:         builder.codecs,
		memberOf:       memberOf,
		classification: NewKindClassificationNotRequired(),
	}
}

func (builder selectedProjectTypeEnvRuntimeReadyBuilder) SetKindClassificationRuntime(
	classification SelectedKindClassificationRuntime,
) selectedProjectTypeEnvRuntimeReadyBuilder {
	builder.classification = classification
	return builder
}

func (builder selectedProjectTypeEnvRuntimeReadyBuilder) Build() (
	SelectedProjectTypeEnvRuntime,
	error,
) {
	if err := verifySelectedProjectTypeEnvRuntimeRequest(builder.request); err != nil {
		return SelectedProjectTypeEnvRuntime{}, err
	}
	if builder.environment.Ref() != builder.request.selected {
		return SelectedProjectTypeEnvRuntime{}, fmt.Errorf(
			"selected project TypeEnv runtime ref %q differs from requested C %q",
			builder.environment.Ref().String(),
			builder.request.selected.String(),
		)
	}
	if _, err := selectedMemberOfRuntimeEngine(builder.memberOf); err != nil {
		return SelectedProjectTypeEnvRuntime{}, err
	}
	if _, err := selectedKindClassificationRuntimeEngine(builder.classification); err != nil {
		return SelectedProjectTypeEnvRuntime{}, err
	}
	runtime := SelectedProjectTypeEnvRuntime(builder)
	return runtime, nil
}

func (runtime SelectedProjectTypeEnvRuntime) Environment() typedmemory.TypeEnv {
	return runtime.environment
}

func (runtime SelectedProjectTypeEnvRuntime) Codecs() typedmemory.CodecRegistry {
	return runtime.codecs
}

// MemberOfRuntime returns the closed posture correlated to this selected C/X
// observation. It never turns a missing evaluator into an implicit fallback.
func (runtime SelectedProjectTypeEnvRuntime) MemberOfRuntime() SelectedMemberOfRuntime {
	return runtime.memberOf
}

func (runtime SelectedProjectTypeEnvRuntime) KindClassificationRuntime() SelectedKindClassificationRuntime {
	return runtime.classification
}

// SelectedProjectTypeEnvRuntimeResolver is the dependency-inverted project-C
// boundary. It receives the same opaque SQLite transaction used by validation
// or admission. Implementations reconstruct exact B/E/X/C through the Stage
// owner, match X to installed runtime registries, and return no database or
// head-mutation capability.
type SelectedProjectTypeEnvRuntimeResolver interface {
	ResolveSelectedProjectTypeEnvRuntime(
		context.Context,
		*sqlitetransaction.Transaction,
		SelectedProjectTypeEnvRuntimeRequest,
	) (SelectedProjectTypeEnvRuntime, error)
}

type resolvedTypeEnvRuntime struct {
	environment    typedmemory.TypeEnv
	codecs         typedmemory.CodecRegistry
	memberOf       MemberOfEvaluationEngine
	classification KindClassificationAdmissionEngine
}

type storedTypeEnvCoordinate struct {
	ref            typedmemory.TypeEnvRef
	representation typeEnvRepresentationKind
}

func (adapter *SQLiteAdapter) resolveTypeEnvRuntimeTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	graphRevision typedmemory.GraphRevision,
	selected typedmemory.TypeEnvRef,
) (resolvedTypeEnvRuntime, error) {
	if transaction == nil {
		return resolvedTypeEnvRuntime{}, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return resolvedTypeEnvRuntime{}, err
	}
	coordinate, err := loadTypeEnvCoordinateTx(ctx, transaction, selected)
	if err != nil {
		return resolvedTypeEnvRuntime{}, err
	}
	switch coordinate.representation {
	case typeEnvRepresentationGenericSnapshot:
		return adapter.resolveGenericSnapshotRuntimeTx(
			ctx,
			transaction,
			coordinate.ref,
		)
	case typeEnvRepresentationProjectExecutable:
		return adapter.resolveSelectedProjectRuntimeTx(
			ctx,
			transaction,
			project,
			graphRevision,
			coordinate.ref,
		)
	default:
		return resolvedTypeEnvRuntime{}, storedAdmissionIntegrity(
			"active TypeEnv coordinate has an unsupported representation",
			nil,
		)
	}
}

func (adapter *SQLiteAdapter) resolveGenericSnapshotRuntimeTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref typedmemory.TypeEnvRef,
) (resolvedTypeEnvRuntime, error) {
	if adapter == nil || !typeEnvLoaderIsPresent(adapter.loader) {
		return resolvedTypeEnvRuntime{}, ErrTypeEnvLoaderRequired
	}
	stored, found, err := loadTypeEnvSnapshotWithScanner(
		ctx,
		transaction,
		ref,
	)
	if err != nil {
		return resolvedTypeEnvRuntime{}, err
	}
	if !found {
		return resolvedTypeEnvRuntime{}, storedAdmissionIntegrity(
			"generic TypeEnv coordinate owner is missing",
			nil,
		)
	}
	environment, codecs, err := adapter.loader.LoadTypeEnv(stored)
	if err != nil {
		return resolvedTypeEnvRuntime{}, storedAdmissionIntegrity(
			"generic TypeEnv snapshot cannot be loaded",
			err,
		)
	}
	if !loadedEnvironmentMatchesSnapshot(environment, stored) {
		return resolvedTypeEnvRuntime{}, storedAdmissionIntegrity(
			"loaded generic TypeEnv differs from immutable snapshot metadata",
			nil,
		)
	}
	return resolvedTypeEnvRuntime{
		environment:    environment,
		codecs:         codecs,
		memberOf:       adapter.memberOfEngine,
		classification: adapter.kindClassificationEngine,
	}, nil
}

func (adapter *SQLiteAdapter) resolveSelectedProjectRuntimeTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	graphRevision typedmemory.GraphRevision,
	selected typedmemory.TypeEnvRef,
) (resolvedTypeEnvRuntime, error) {
	if adapter == nil ||
		!selectedProjectTypeEnvRuntimeResolverIsPresent(
			adapter.selectedProjectRuntime,
		) {
		return resolvedTypeEnvRuntime{},
			ErrSelectedProjectTypeEnvRuntimeResolverRequired
	}
	request := SelectedProjectTypeEnvRuntimeRequest{
		project:       project,
		graphRevision: graphRevision,
		selected:      selected,
	}
	if err := verifySelectedProjectTypeEnvRuntimeRequest(request); err != nil {
		return resolvedTypeEnvRuntime{}, err
	}
	resolved, err :=
		adapter.selectedProjectRuntime.ResolveSelectedProjectTypeEnvRuntime(
			ctx,
			transaction,
			request,
		)
	if err != nil {
		return resolvedTypeEnvRuntime{}, fmt.Errorf(
			"resolve selected project TypeEnv runtime: %w",
			err,
		)
	}
	if err := verifySelectedProjectTypeEnvRuntime(resolved, request); err != nil {
		return resolvedTypeEnvRuntime{}, err
	}
	memberOf, err := selectedMemberOfRuntimeEngine(resolved.MemberOfRuntime())
	if err != nil {
		return resolvedTypeEnvRuntime{}, err
	}
	classification, err := selectedKindClassificationRuntimeEngine(
		resolved.KindClassificationRuntime(),
	)
	if err != nil {
		return resolvedTypeEnvRuntime{}, err
	}
	return resolvedTypeEnvRuntime{
		environment:    resolved.environment,
		codecs:         resolved.codecs,
		memberOf:       memberOf,
		classification: classification,
	}, nil
}

func loadTypeEnvCoordinateTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	expected typedmemory.TypeEnvRef,
) (storedTypeEnvCoordinate, error) {
	if ctx == nil {
		return storedTypeEnvCoordinate{}, fmt.Errorf(
			"load TypeEnv coordinate: context is required",
		)
	}
	if transaction == nil {
		return storedTypeEnvCoordinate{},
			sqlitetransaction.ErrTransactionInvalid
	}
	var refText string
	var representation string
	var genericRef sql.NullString
	var projectRef sql.NullString
	err := transaction.ScanOne(
		ctx,
		`SELECT
			type_env_ref,
			representation_kind,
			generic_snapshot_ref,
			project_executable_ref
		FROM typed_memory_type_env_coordinates
		WHERE type_env_ref = ?`,
		[]any{expected.String()},
		[]any{&refText, &representation, &genericRef, &projectRef},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedTypeEnvCoordinate{}, storedAdmissionIntegrity(
			"active TypeEnv coordinate is missing",
			nil,
		)
	}
	if err != nil {
		return storedTypeEnvCoordinate{}, fmt.Errorf(
			"load active TypeEnv coordinate: %w",
			err,
		)
	}
	ref, err := typedmemory.ParseTypeEnvRef(refText)
	if err != nil || ref != expected {
		return storedTypeEnvCoordinate{}, storedAdmissionIntegrity(
			"active TypeEnv coordinate ref differs from its lookup key",
			err,
		)
	}
	kind, err := verifyTypeEnvCoordinateOwner(
		ref,
		representation,
		genericRef,
		projectRef,
	)
	if err != nil {
		return storedTypeEnvCoordinate{}, storedAdmissionIntegrity(
			"active TypeEnv coordinate owner is malformed",
			err,
		)
	}
	return storedTypeEnvCoordinate{
		ref:            ref,
		representation: kind,
	}, nil
}

func verifyTypeEnvCoordinateOwner(
	ref typedmemory.TypeEnvRef,
	representation string,
	genericRef sql.NullString,
	projectRef sql.NullString,
) (typeEnvRepresentationKind, error) {
	switch representation {
	case "generic_snapshot":
		if !genericRef.Valid ||
			genericRef.String != ref.String() ||
			projectRef.Valid {
			return 0, fmt.Errorf(
				"generic snapshot ownership does not point exclusively to itself",
			)
		}
		return typeEnvRepresentationGenericSnapshot, nil
	case "project_executable":
		if genericRef.Valid ||
			!projectRef.Valid ||
			projectRef.String != ref.String() {
			return 0, fmt.Errorf(
				"project executable ownership does not point exclusively to itself",
			)
		}
		return typeEnvRepresentationProjectExecutable, nil
	default:
		return 0, fmt.Errorf(
			"unsupported TypeEnv representation %q",
			representation,
		)
	}
}

func verifySelectedProjectTypeEnvRuntimeRequest(
	request SelectedProjectTypeEnvRuntimeRequest,
) error {
	canonicalProject, err := projectledger.ParseProjectID(
		request.project.String(),
	)
	if err != nil || canonicalProject != request.project {
		return fmt.Errorf(
			"selected project TypeEnv runtime requires an exact project",
		)
	}
	if request.graphRevision.Value() == 0 {
		return fmt.Errorf(
			"selected project TypeEnv runtime requires a positive graph revision",
		)
	}
	if request.selected.Digest().String() == "" {
		return fmt.Errorf(
			"selected project TypeEnv runtime requires an exact composite C",
		)
	}
	return nil
}

func verifySelectedProjectTypeEnvRuntime(
	runtime SelectedProjectTypeEnvRuntime,
	request SelectedProjectTypeEnvRuntimeRequest,
) error {
	if runtime.request != request {
		return fmt.Errorf(
			"selected project TypeEnv runtime is uncorrelated with its transaction request",
		)
	}
	if runtime.environment.Ref() != request.selected {
		return fmt.Errorf(
			"selected project TypeEnv runtime differs from requested C",
		)
	}
	if _, err := selectedMemberOfRuntimeEngine(runtime.memberOf); err != nil {
		return err
	}
	_, err := selectedKindClassificationRuntimeEngine(runtime.classification)
	return err
}

func selectedKindClassificationRuntimeEngine(
	runtime SelectedKindClassificationRuntime,
) (KindClassificationAdmissionEngine, error) {
	switch exact := runtime.(type) {
	case KindClassificationNotRequired:
		return nil, nil
	case ExactKindClassificationRuntime:
		if !kindClassificationAdmissionEngineIsPresent(exact.engine) {
			return nil, fmt.Errorf(
				"selected project TypeEnv runtime carries an invalid kind-classification evaluator",
			)
		}
		if err := verifyExactDigest(
			exact.runtimeBasisDigest.Digest(),
			"runtime-basis X",
		); err != nil {
			return nil, err
		}
		if err := verifyExactDigest(
			exact.registryCoordinateDigest.Digest(),
			"target-registry coordinate",
		); err != nil {
			return nil, err
		}
		return exact.engine, nil
	default:
		return nil, fmt.Errorf(
			"selected project TypeEnv runtime requires an explicit kind-classification posture",
		)
	}
}

func selectedMemberOfRuntimeEngine(
	runtime SelectedMemberOfRuntime,
) (MemberOfEvaluationEngine, error) {
	switch exact := runtime.(type) {
	case MemberOfNotRequired:
		return nil, nil
	case ExactMemberOfRuntime:
		if !memberOfEvaluationEngineIsPresent(exact.engine) {
			return nil, fmt.Errorf(
				"selected project TypeEnv runtime carries an invalid MemberOf evaluator",
			)
		}
		if err := verifyExactDigest(
			exact.runtimeBasisDigest.Digest(),
			"runtime-basis X",
		); err != nil {
			return nil, err
		}
		if err := verifyExactDigest(
			exact.registryCoordinateDigest.Digest(),
			"target-registry coordinate",
		); err != nil {
			return nil, err
		}
		return exact.engine, nil
	default:
		return nil, fmt.Errorf(
			"selected project TypeEnv runtime requires an explicit MemberOf posture",
		)
	}
}

func verifyExactDigest(
	digest typedmemory.SHA256Digest,
	label string,
) error {
	parsed, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || parsed != digest {
		return fmt.Errorf("exact selected runtime requires a canonical %s digest", label)
	}
	return nil
}

func memberOfEvaluationEngineIsPresent(
	engine MemberOfEvaluationEngine,
) bool {
	if engine == nil {
		return false
	}
	value := reflect.ValueOf(engine)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}

func kindClassificationAdmissionEngineIsPresent(
	engine KindClassificationAdmissionEngine,
) bool {
	if engine == nil {
		return false
	}
	value := reflect.ValueOf(engine)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}

func selectedProjectTypeEnvRuntimeResolverIsPresent(
	resolver SelectedProjectTypeEnvRuntimeResolver,
) bool {
	if resolver == nil {
		return false
	}
	value := reflect.ValueOf(resolver)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}
