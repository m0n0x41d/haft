package projecttypeenvstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
)

const runtimeMechanismCanonicalSchema = "haft.runtime-mechanism-artifact/v1"

type runtimeMechanismRecord struct {
	artifactRef     string
	edition         string
	digest          string
	canonicalSchema string
	canonical       []byte
}

func (record runtimeMechanismRecord) clone() runtimeMechanismRecord {
	result := record
	result.canonical = append([]byte(nil), record.canonical...)
	return result
}

func (record runtimeMechanismRecord) coordinate() string {
	return record.artifactRef + "@" + record.edition
}

func (record runtimeMechanismRecord) exactEqual(other runtimeMechanismRecord) bool {
	return record.artifactRef == other.artifactRef &&
		record.edition == other.edition &&
		record.digest == other.digest &&
		record.canonicalSchema == other.canonicalSchema &&
		bytes.Equal(record.canonical, other.canonical)
}

func prepareRuntimeMechanismArtifacts(
	artifacts []runtimemechanism.RuntimeMechanismArtifactV1,
) ([]runtimeMechanismRecord, []runtimemechanism.RuntimeMechanismArtifactV1, error) {
	type prepared struct {
		record   runtimeMechanismRecord
		artifact runtimemechanism.RuntimeMechanismArtifactV1
	}
	values := make([]prepared, 0, len(artifacts))
	for index, artifact := range artifacts {
		if err := artifact.Verify(); err != nil {
			return nil, nil, fmt.Errorf(
				"%w: verify runtime mechanism artifact[%d]: %v",
				ErrClosureInconsistent,
				index,
				err,
			)
		}
		canonical := artifact.CanonicalBytes()
		decoded, err := runtimemechanism.DecodeRuntimeMechanismArtifactV1(canonical)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"%w: decode runtime mechanism artifact[%d]: %v",
				ErrClosureInconsistent,
				index,
				err,
			)
		}
		if decoded.Identity() != artifact.Identity() ||
			!bytes.Equal(decoded.CanonicalBytes(), canonical) {
			return nil, nil, fmt.Errorf(
				"%w: runtime mechanism artifact[%d] metadata does not match canonical bytes",
				ErrClosureInconsistent,
				index,
			)
		}
		identity := decoded.Identity()
		values = append(values, prepared{
			record: runtimeMechanismRecord{
				artifactRef:     identity.Artifact().String(),
				edition:         identity.Edition().String(),
				digest:          identity.Digest().String(),
				canonicalSchema: runtimeMechanismCanonicalSchema,
				canonical:       decoded.CanonicalBytes(),
			},
			artifact: decoded,
		})
	}
	sort.Slice(values, func(left int, right int) bool {
		return values[left].record.coordinate() < values[right].record.coordinate()
	})
	records := make([]runtimeMechanismRecord, 0, len(values))
	verified := make([]runtimemechanism.RuntimeMechanismArtifactV1, 0, len(values))
	for _, value := range values {
		if len(records) != 0 && records[len(records)-1].coordinate() == value.record.coordinate() {
			prior := records[len(records)-1]
			if !prior.exactEqual(value.record) {
				return nil, nil, fmt.Errorf(
					"%w: runtime mechanism coordinate %q has different exact bytes",
					ErrClosureInconsistent,
					value.record.coordinate(),
				)
			}
			continue
		}
		records = append(records, value.record.clone())
		verified = append(verified, value.artifact)
	}
	return records, verified, nil
}

func putRuntimeMechanismRecord(
	ctx context.Context,
	transaction *sql.Tx,
	record runtimeMechanismRecord,
) error {
	if _, err := decodeRuntimeMechanismRecord(record); err != nil {
		return integrityError(
			ArtifactRuntimeBasis,
			record.coordinate(),
			fmt.Errorf("prepare runtime mechanism write: %w", err),
		)
	}
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_runtime_mechanisms (
			artifact_ref,
			edition,
			artifact_digest,
			canonical_schema_version,
			canonical_bytes
		)
		SELECT ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM project_typeenv_runtime_mechanisms
			WHERE artifact_ref = ? AND edition = ?
		)`,
		record.artifactRef,
		record.edition,
		record.digest,
		record.canonicalSchema,
		record.canonical,
		record.artifactRef,
		record.edition,
	)
	if err != nil {
		return fmt.Errorf("insert runtime mechanism %q: %w", record.coordinate(), err)
	}
	stored, err := loadRuntimeMechanismRecord(
		ctx,
		sqlOneRowScanner{query: transaction},
		record.artifactRef,
		record.edition,
	)
	if err != nil {
		if errors.Is(err, ErrArtifactNotFound) {
			return fmt.Errorf(
				"%w: runtime mechanism %q",
				ErrArtifactConflict,
				record.coordinate(),
			)
		}
		return err
	}
	if !stored.exactEqual(record) {
		return fmt.Errorf(
			"%w: runtime mechanism %q",
			ErrArtifactConflict,
			record.coordinate(),
		)
	}
	return nil
}

func decodeRuntimeMechanismRecord(
	record runtimeMechanismRecord,
) (runtimemechanism.RuntimeMechanismArtifactV1, error) {
	decoded, err := runtimemechanism.DecodeRuntimeMechanismArtifactV1(record.canonical)
	if err != nil {
		return runtimemechanism.RuntimeMechanismArtifactV1{}, err
	}
	identity := decoded.Identity()
	if identity.Artifact().String() != record.artifactRef ||
		identity.Edition().String() != record.edition ||
		identity.Digest().String() != record.digest ||
		record.canonicalSchema != runtimeMechanismCanonicalSchema ||
		!bytes.Equal(decoded.CanonicalBytes(), record.canonical) {
		return runtimemechanism.RuntimeMechanismArtifactV1{}, fmt.Errorf(
			"runtime mechanism metadata does not match canonical bytes",
		)
	}
	return decoded, nil
}

func loadRuntimeMechanismRecord(
	ctx context.Context,
	scanner oneRowScanner,
	artifactRef string,
	edition string,
) (runtimeMechanismRecord, error) {
	record := runtimeMechanismRecord{}
	err := scanner.ScanOne(
		ctx,
		`SELECT
			artifact_ref,
			edition,
			artifact_digest,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_runtime_mechanisms
		 WHERE artifact_ref = ? AND edition = ?`,
		[]any{artifactRef, edition},
		[]any{
			&record.artifactRef,
			&record.edition,
			&record.digest,
			&record.canonicalSchema,
			&record.canonical,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return runtimeMechanismRecord{}, fmt.Errorf(
			"%w: runtime mechanism %q@%q",
			ErrArtifactNotFound,
			artifactRef,
			edition,
		)
	}
	if err != nil {
		return runtimeMechanismRecord{}, fmt.Errorf(
			"load runtime mechanism %q@%q: %w",
			artifactRef,
			edition,
			err,
		)
	}
	return record.clone(), nil
}

func resolveStoredRuntimeBasis(
	ctx context.Context,
	scanner oneRowScanner,
	basis projecttypeenv.RuntimeEvaluationBasisArtifact,
) (projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	identities, err := runtimeMechanismIdentitiesForPins(basis.Pins())
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			basis.Ref().String(),
			err,
		)
	}
	artifacts := make([]runtimemechanism.RuntimeMechanismArtifactV1, 0, len(identities))
	for _, identity := range identities {
		record, loadErr := loadRuntimeMechanismRecord(
			ctx,
			scanner,
			identity.artifactRef,
			identity.edition,
		)
		if loadErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
				ArtifactRuntimeBasis,
				basis.Ref().String(),
				loadErr,
			)
		}
		if record.digest != identity.digest {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
				ArtifactRuntimeBasis,
				basis.Ref().String(),
				fmt.Errorf(
					"runtime mechanism %q digest is %q; X pins %q",
					record.coordinate(),
					record.digest,
					identity.digest,
				),
			)
		}
		artifact, decodeErr := decodeRuntimeMechanismRecord(record)
		if decodeErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
				ArtifactRuntimeBasis,
				basis.Ref().String(),
				decodeErr,
			)
		}
		artifacts = append(artifacts, artifact)
	}
	policyPins := basis.RegistrationPolicyPins()
	policies := make([]projecttypeenv.RegistrationPolicyArtifact, 0, len(policyPins))
	for _, pin := range policyPins {
		record, loadErr := loadRegistrationPolicyRecord(
			ctx,
			scanner,
			pin.Registration().String(),
		)
		if loadErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
				ArtifactRuntimeBasis,
				basis.Ref().String(),
				loadErr,
			)
		}
		policy, decodeErr := decodeRegistrationPolicyRecord(record)
		if decodeErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
				ArtifactRuntimeBasis,
				basis.Ref().String(),
				decodeErr,
			)
		}
		policies = append(policies, policy)
	}
	resolved, err := projecttypeenv.ResolveRuntimeEvaluationBasisClosure(
		basis,
		artifacts,
		policies,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			basis.Ref().String(),
			fmt.Errorf("resolve stored X closure: %w", err),
		)
	}
	if err := resolved.VerifyResolvedClosure(); err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			basis.Ref().String(),
			fmt.Errorf("verify stored X closure: %w", err),
		)
	}
	return resolved, nil
}

type runtimeMechanismIdentity struct {
	artifactRef string
	edition     string
	digest      string
}

type pinWithMechanism interface {
	Mechanism() projecttypeenv.RuntimeMechanismArtifactPin
}

func runtimeMechanismIdentitiesForPins(
	pins []projecttypeenv.RuntimeEvaluationMechanismPin,
) ([]runtimeMechanismIdentity, error) {
	byCoordinate := make(map[string]runtimeMechanismIdentity)
	for index, pin := range pins {
		withMechanism, ok := pin.(pinWithMechanism)
		if !ok {
			return nil, fmt.Errorf("runtime X pin[%d] type %T exposes no exact mechanism identity", index, pin)
		}
		mechanism := withMechanism.Mechanism()
		identity := runtimeMechanismIdentity{
			artifactRef: mechanism.Artifact().String(),
			edition:     mechanism.Edition().String(),
			digest:      mechanism.Digest().String(),
		}
		coordinate := identity.artifactRef + "@" + identity.edition
		prior, exists := byCoordinate[coordinate]
		if exists && prior.digest != identity.digest {
			return nil, fmt.Errorf(
				"runtime X pins conflicting digests for mechanism coordinate %q",
				coordinate,
			)
		}
		byCoordinate[coordinate] = identity
	}
	keys := make([]string, 0, len(byCoordinate))
	for key := range byCoordinate {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]runtimeMechanismIdentity, 0, len(keys))
	for _, key := range keys {
		result = append(result, byCoordinate[key])
	}
	return result, nil
}
