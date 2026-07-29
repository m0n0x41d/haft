package projecttypeenvstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
)

const registrationPolicyCanonicalSchema = recordmembershipregistration.RegistrationSchemaV1

type registrationPolicyRecord struct {
	ref             string
	digest          string
	canonicalSchema string
	canonical       []byte
}

func (record registrationPolicyRecord) clone() registrationPolicyRecord {
	result := record
	result.canonical = append([]byte(nil), record.canonical...)
	return result
}

func (record registrationPolicyRecord) exactEqual(other registrationPolicyRecord) bool {
	return record.ref == other.ref &&
		record.digest == other.digest &&
		record.canonicalSchema == other.canonicalSchema &&
		bytes.Equal(record.canonical, other.canonical)
}

func prepareRegistrationPolicyArtifacts(
	artifacts []projecttypeenv.RegistrationPolicyArtifact,
) ([]registrationPolicyRecord, []projecttypeenv.RegistrationPolicyArtifact, error) {
	type prepared struct {
		record   registrationPolicyRecord
		artifact projecttypeenv.RegistrationPolicyArtifact
	}
	values := make([]prepared, 0, len(artifacts))
	for index, artifact := range artifacts {
		if err := artifact.Verify(); err != nil {
			return nil, nil, fmt.Errorf(
				"%w: verify registration-policy artifact[%d]: %v",
				ErrClosureInconsistent,
				index,
				err,
			)
		}
		canonical := artifact.CanonicalBytes()
		decoded, err := projecttypeenv.DecodeRegistrationPolicyArtifact(canonical)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"%w: decode registration-policy artifact[%d]: %v",
				ErrClosureInconsistent,
				index,
				err,
			)
		}
		if decoded.Ref() != artifact.Ref() ||
			!bytes.Equal(decoded.CanonicalBytes(), canonical) {
			return nil, nil, fmt.Errorf(
				"%w: registration-policy artifact[%d] metadata does not match canonical bytes",
				ErrClosureInconsistent,
				index,
			)
		}
		values = append(values, prepared{
			record: registrationPolicyRecord{
				ref:             decoded.Ref().String(),
				digest:          decoded.Ref().Digest().String(),
				canonicalSchema: registrationPolicyCanonicalSchema,
				canonical:       decoded.CanonicalBytes(),
			},
			artifact: decoded,
		})
	}
	sort.Slice(values, func(left int, right int) bool {
		return values[left].record.ref < values[right].record.ref
	})
	records := make([]registrationPolicyRecord, 0, len(values))
	verified := make([]projecttypeenv.RegistrationPolicyArtifact, 0, len(values))
	for _, value := range values {
		if len(records) != 0 && records[len(records)-1].ref == value.record.ref {
			return nil, nil, fmt.Errorf(
				"%w: duplicate registration-policy artifact %q",
				ErrClosureInconsistent,
				value.record.ref,
			)
		}
		records = append(records, value.record.clone())
		verified = append(verified, value.artifact)
	}
	return records, verified, nil
}

func putRegistrationPolicyRecord(
	ctx context.Context,
	transaction *sql.Tx,
	record registrationPolicyRecord,
) error {
	if _, err := decodeRegistrationPolicyRecord(record); err != nil {
		return integrityError(
			ArtifactRuntimeBasis,
			record.ref,
			fmt.Errorf("prepare registration-policy write: %w", err),
		)
	}
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_registration_policies (
			registration_ref,
			artifact_digest,
			canonical_schema_version,
			canonical_bytes
		)
		SELECT ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM project_typeenv_registration_policies
			WHERE registration_ref = ? OR artifact_digest = ?
		)`,
		record.ref,
		record.digest,
		record.canonicalSchema,
		record.canonical,
		record.ref,
		record.digest,
	)
	if err != nil {
		return fmt.Errorf("insert registration-policy %q: %w", record.ref, err)
	}
	stored, err := loadRegistrationPolicyRecord(
		ctx,
		sqlOneRowScanner{query: transaction},
		record.ref,
	)
	if err != nil {
		if errors.Is(err, ErrArtifactNotFound) {
			return fmt.Errorf(
				"%w: registration-policy %q",
				ErrArtifactConflict,
				record.ref,
			)
		}
		return err
	}
	if !stored.exactEqual(record) {
		return fmt.Errorf("%w: registration-policy %q", ErrArtifactConflict, record.ref)
	}
	return nil
}

func decodeRegistrationPolicyRecord(
	record registrationPolicyRecord,
) (projecttypeenv.RegistrationPolicyArtifact, error) {
	decoded, err := projecttypeenv.DecodeRegistrationPolicyArtifact(record.canonical)
	if err != nil {
		return projecttypeenv.RegistrationPolicyArtifact{}, err
	}
	if decoded.Ref().String() != record.ref ||
		decoded.Ref().Digest().String() != record.digest ||
		record.canonicalSchema != registrationPolicyCanonicalSchema ||
		!bytes.Equal(decoded.CanonicalBytes(), record.canonical) {
		return projecttypeenv.RegistrationPolicyArtifact{}, fmt.Errorf(
			"registration-policy metadata does not match canonical bytes",
		)
	}
	return decoded, nil
}

func loadRegistrationPolicyRecord(
	ctx context.Context,
	scanner oneRowScanner,
	ref string,
) (registrationPolicyRecord, error) {
	record := registrationPolicyRecord{}
	err := scanner.ScanOne(
		ctx,
		`SELECT
			registration_ref,
			artifact_digest,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_registration_policies
		 WHERE registration_ref = ?`,
		[]any{ref},
		[]any{
			&record.ref,
			&record.digest,
			&record.canonicalSchema,
			&record.canonical,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return registrationPolicyRecord{}, fmt.Errorf(
			"%w: registration-policy %q",
			ErrArtifactNotFound,
			ref,
		)
	}
	if err != nil {
		return registrationPolicyRecord{}, fmt.Errorf(
			"load registration-policy %q: %w",
			ref,
			err,
		)
	}
	return record.clone(), nil
}
