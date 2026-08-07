package fpfrefresh

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

const IntegrationLockSchemaVersion = "haft.fpf-integration-lock/v1"

// IntegrationCoordinates bind one exact FPF publication, derived index, and
// Base TypeEnv. They contain reproducibility coordinates, not FPF meaning,
// compatibility approval, lifecycle state, or release authority.
type IntegrationCoordinates struct {
	SourceRevision         string `json:"source_revision"`
	ReadmeDocumentDigest   string `json:"readme_document_digest"`
	SpecDocumentDigest     string `json:"spec_document_digest"`
	DatabaseDigest         string `json:"database_digest"`
	SourceUnitCount        int    `json:"source_unit_count"`
	IndexSchemaVersion     string `json:"index_schema_version"`
	BaseTypeEnvRef         string `json:"base_type_env_ref"`
	BaseTypeEnvDigest      string `json:"base_type_env_digest"`
	TypeEnvCompilerEdition string `json:"type_env_compiler_edition"`
}

type TokenGateCoordinates struct {
	FixtureRevision string `json:"fixture_revision"`
	FixtureDigest   string `json:"fixture_digest"`
}

// IntegrationLock is generated from verified source and index bytes. The
// optional token-gate coordinates remain a fixture identity; token thresholds
// and behavioral expectations deliberately stay outside this generated file.
type IntegrationLock struct {
	SchemaVersion string                 `json:"schema_version"`
	GeneratedBy   string                 `json:"generated_by"`
	Coordinates   IntegrationCoordinates `json:"coordinates"`
	TokenGate     *TokenGateCoordinates  `json:"token_gate,omitempty"`
}

type IntegrationCoordinateInput struct {
	SourceRevision string
	ReadmePath     string
	SpecPath       string
	DatabasePath   string
	GeneratedBy    string
	TokenGate      *TokenGateCoordinates
}

// IntegrationByteCoordinateInput verifies exact Git-object publication bytes
// against a derived database without requiring a source checkout.
type IntegrationByteCoordinateInput struct {
	SourceRevision string
	ReadmeBytes    []byte
	SpecBytes      []byte
	DatabasePath   string
	GeneratedBy    string
	TokenGate      *TokenGateCoordinates
}

func BuildIntegrationLock(input IntegrationCoordinateInput) (IntegrationLock, error) {
	coordinates, err := ReadIntegrationCoordinates(input)
	if err != nil {
		return IntegrationLock{}, err
	}
	lock := IntegrationLock{
		SchemaVersion: IntegrationLockSchemaVersion,
		GeneratedBy:   strings.TrimSpace(input.GeneratedBy),
		Coordinates:   coordinates,
		TokenGate:     cloneTokenGateCoordinates(input.TokenGate),
	}
	if err := lock.Validate(); err != nil {
		return IntegrationLock{}, err
	}
	return lock, nil
}

func BuildIntegrationLockFromBytes(
	input IntegrationByteCoordinateInput,
) (IntegrationLock, error) {
	coordinates, err := ReadIntegrationCoordinatesFromBytes(input)
	if err != nil {
		return IntegrationLock{}, err
	}
	lock := IntegrationLock{
		SchemaVersion: IntegrationLockSchemaVersion,
		GeneratedBy:   strings.TrimSpace(input.GeneratedBy),
		Coordinates:   coordinates,
		TokenGate:     cloneTokenGateCoordinates(input.TokenGate),
	}
	if err := lock.Validate(); err != nil {
		return IntegrationLock{}, err
	}
	return lock, nil
}

func ReadIntegrationCoordinates(input IntegrationCoordinateInput) (IntegrationCoordinates, error) {
	readmeDigest, err := digestFile(input.ReadmePath)
	if err != nil {
		return IntegrationCoordinates{}, fmt.Errorf("digest FPF Readme.md: %w", err)
	}
	specDigest, err := digestFile(input.SpecPath)
	if err != nil {
		return IntegrationCoordinates{}, fmt.Errorf("digest FPF-Spec.md: %w", err)
	}
	databaseDigest, err := digestFile(input.DatabasePath)
	if err != nil {
		return IntegrationCoordinates{}, fmt.Errorf("digest FPF index: %w", err)
	}
	return readIntegrationCoordinates(
		input.SourceRevision,
		readmeDigest,
		specDigest,
		databaseDigest,
		input.DatabasePath,
	)
}

func ReadIntegrationCoordinatesFromBytes(
	input IntegrationByteCoordinateInput,
) (IntegrationCoordinates, error) {
	readmeDigest := digestBytesSHA256(input.ReadmeBytes)
	specDigest := digestBytesSHA256(input.SpecBytes)
	databaseDigest, err := digestFile(input.DatabasePath)
	if err != nil {
		return IntegrationCoordinates{}, fmt.Errorf("digest FPF index: %w", err)
	}
	return readIntegrationCoordinates(
		input.SourceRevision,
		readmeDigest,
		specDigest,
		databaseDigest,
		input.DatabasePath,
	)
}

func readIntegrationCoordinates(
	sourceRevision string,
	readmeDigest string,
	specDigest string,
	databaseDigest string,
	databasePath string,
) (IntegrationCoordinates, error) {
	revision, err := normalizeCommitSHA(sourceRevision)
	if err != nil {
		return IntegrationCoordinates{}, fmt.Errorf("source revision: %w", err)
	}

	database, err := openIntegrationDatabaseReadOnly(databasePath)
	if err != nil {
		return IntegrationCoordinates{}, err
	}
	defer func() { _ = database.Close() }()

	meta, err := readRequiredIntegrationMeta(database)
	if err != nil {
		return IntegrationCoordinates{}, err
	}
	if meta["fpf_commit"] != revision {
		return IntegrationCoordinates{}, fmt.Errorf(
			"snapshot_pin_stale: index source revision %q differs from exact source revision %q",
			meta["fpf_commit"],
			revision,
		)
	}
	if meta["readme_document_digest"] != readmeDigest {
		return IntegrationCoordinates{}, fmt.Errorf(
			"snapshot_pin_stale: index Readme.md digest %q differs from source bytes %q",
			meta["readme_document_digest"],
			readmeDigest,
		)
	}
	if meta["spec_document_digest"] != specDigest {
		return IntegrationCoordinates{}, fmt.Errorf(
			"snapshot_pin_stale: index FPF-Spec.md digest %q differs from source bytes %q",
			meta["spec_document_digest"],
			specDigest,
		)
	}
	if meta["typeenv_source_revision"] != revision {
		return IntegrationCoordinates{}, fmt.Errorf(
			"snapshot_pin_stale: TypeEnv source revision %q differs from exact source revision %q",
			meta["typeenv_source_revision"],
			revision,
		)
	}
	sourceUnitCount, err := strconv.Atoi(meta["indexed_source_units"])
	if err != nil || sourceUnitCount <= 0 {
		return IntegrationCoordinates{}, fmt.Errorf(
			"snapshot_pin_stale: indexed_source_units=%q must be a positive integer",
			meta["indexed_source_units"],
		)
	}

	return IntegrationCoordinates{
		SourceRevision:         revision,
		ReadmeDocumentDigest:   readmeDigest,
		SpecDocumentDigest:     specDigest,
		DatabaseDigest:         databaseDigest,
		SourceUnitCount:        sourceUnitCount,
		IndexSchemaVersion:     meta["schema_version"],
		BaseTypeEnvRef:         meta["typeenv_ref"],
		BaseTypeEnvDigest:      meta["typeenv_artifact_digest"],
		TypeEnvCompilerEdition: meta["typeenv_compiler_schema_version"],
	}, nil
}

func (lock IntegrationLock) Validate() error {
	if lock.SchemaVersion != IntegrationLockSchemaVersion {
		return fmt.Errorf(
			"integration lock schema_version=%q, want %q",
			lock.SchemaVersion,
			IntegrationLockSchemaVersion,
		)
	}
	if strings.TrimSpace(lock.GeneratedBy) == "" {
		return fmt.Errorf("integration lock generated_by is required")
	}
	if err := validateIntegrationCoordinates(lock.Coordinates); err != nil {
		return err
	}
	if lock.TokenGate != nil {
		if strings.TrimSpace(lock.TokenGate.FixtureRevision) == "" {
			return fmt.Errorf("integration lock token_gate.fixture_revision is required")
		}
		if err := validateSHA256Digest(lock.TokenGate.FixtureDigest); err != nil {
			return fmt.Errorf("integration lock token_gate.fixture_digest: %w", err)
		}
	}
	return nil
}

func MarshalIntegrationLock(lock IntegrationLock) ([]byte, error) {
	if err := lock.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode integration lock: %w", err)
	}
	return append(payload, '\n'), nil
}

func ParseIntegrationLock(payload []byte) (IntegrationLock, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var lock IntegrationLock
	if err := decoder.Decode(&lock); err != nil {
		return IntegrationLock{}, fmt.Errorf("decode integration lock: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return IntegrationLock{}, fmt.Errorf("decode integration lock: trailing JSON value")
		}
		return IntegrationLock{}, fmt.Errorf("decode integration lock trailing data: %w", err)
	}
	if err := lock.Validate(); err != nil {
		return IntegrationLock{}, err
	}
	canonical, err := MarshalIntegrationLock(lock)
	if err != nil {
		return IntegrationLock{}, err
	}
	if !bytes.Equal(payload, canonical) {
		return IntegrationLock{}, fmt.Errorf(
			"snapshot_pin_stale: integration lock is not canonical generated bytes; regenerate it",
		)
	}
	return lock, nil
}

func VerifyIntegrationLock(lock IntegrationLock, input IntegrationCoordinateInput) error {
	actual, err := BuildIntegrationLock(input)
	if err != nil {
		return err
	}
	expectedBytes, err := MarshalIntegrationLock(lock)
	if err != nil {
		return err
	}
	actualBytes, err := MarshalIntegrationLock(actual)
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedBytes, actualBytes) {
		return fmt.Errorf(
			"snapshot_pin_stale: generated integration coordinates differ from source, DB, TypeEnv, or token fixture bytes",
		)
	}
	return nil
}

func VerifyIntegrationLockFromBytes(
	lock IntegrationLock,
	input IntegrationByteCoordinateInput,
) error {
	actual, err := BuildIntegrationLockFromBytes(input)
	if err != nil {
		return err
	}
	expectedBytes, err := MarshalIntegrationLock(lock)
	if err != nil {
		return err
	}
	actualBytes, err := MarshalIntegrationLock(actual)
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedBytes, actualBytes) {
		return fmt.Errorf(
			"snapshot_pin_stale: generated integration coordinates differ from source, DB, TypeEnv, or token fixture bytes",
		)
	}
	return nil
}

func WriteIntegrationLock(path string, lock IntegrationLock) error {
	payload, err := MarshalIntegrationLock(lock)
	if err != nil {
		return err
	}
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return fmt.Errorf("create integration lock directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(cleanPath), ".fpf-integration-lock-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary integration lock: %w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(payload); err != nil {
		return fmt.Errorf("write temporary integration lock: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("fsync temporary integration lock: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary integration lock: %w", err)
	}
	if err := os.Rename(temporaryPath, cleanPath); err != nil {
		return fmt.Errorf("replace integration lock: %w", err)
	}
	keepTemporary = false
	directory, err := os.Open(filepath.Dir(cleanPath))
	if err != nil {
		return fmt.Errorf("open integration lock directory for fsync: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("fsync integration lock directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close integration lock directory: %w", closeErr)
	}
	return nil
}

func validateIntegrationCoordinates(coordinates IntegrationCoordinates) error {
	if _, err := normalizeCommitSHA(coordinates.SourceRevision); err != nil {
		return fmt.Errorf("integration lock source_revision: %w", err)
	}
	for field, value := range map[string]string{
		"readme_document_digest": coordinates.ReadmeDocumentDigest,
		"spec_document_digest":   coordinates.SpecDocumentDigest,
		"database_digest":        coordinates.DatabaseDigest,
		"base_type_env_digest":   coordinates.BaseTypeEnvDigest,
	} {
		if err := validateSHA256Digest(value); err != nil {
			return fmt.Errorf("integration lock %s: %w", field, err)
		}
	}
	if coordinates.SourceUnitCount <= 0 {
		return fmt.Errorf("integration lock source_unit_count must be positive")
	}
	if strings.TrimSpace(coordinates.IndexSchemaVersion) == "" {
		return fmt.Errorf("integration lock index_schema_version is required")
	}
	if !strings.HasPrefix(coordinates.BaseTypeEnvRef, "typeenv:sha256:") {
		return fmt.Errorf("integration lock base_type_env_ref must be a typeenv:sha256 reference")
	}
	if strings.TrimPrefix(coordinates.BaseTypeEnvRef, "typeenv:") != coordinates.BaseTypeEnvDigest {
		return fmt.Errorf("integration lock Base TypeEnv ref and digest differ")
	}
	if strings.TrimSpace(coordinates.TypeEnvCompilerEdition) == "" {
		return fmt.Errorf("integration lock type_env_compiler_edition is required")
	}
	return nil
}

func readRequiredIntegrationMeta(database *sql.DB) (map[string]string, error) {
	keys := []string{
		"fpf_commit",
		"indexed_source_units",
		"readme_document_digest",
		"schema_version",
		"spec_document_digest",
		"typeenv_artifact_digest",
		"typeenv_compiler_schema_version",
		"typeenv_ref",
		"typeenv_source_revision",
	}
	meta := make(map[string]string, len(keys))
	for _, key := range keys {
		var value string
		if err := database.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value); err != nil {
			return nil, fmt.Errorf("read FPF index metadata %s: %w", key, err)
		}
		meta[key] = strings.TrimSpace(value)
	}
	return meta, nil
}

func openIntegrationDatabaseReadOnly(path string) (*sql.DB, error) {
	absolutePath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("resolve FPF index path: %w", err)
	}
	readOnlyURI := url.URL{Scheme: "file", Path: absolutePath}
	query := readOnlyURI.Query()
	query.Set("mode", "ro")
	readOnlyURI.RawQuery = query.Encode()
	database, err := sql.Open("sqlite", readOnlyURI.String())
	if err != nil {
		return nil, fmt.Errorf("open FPF index read-only: %w", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("open FPF index read-only: %w", err)
	}
	return database, nil
}

func digestFile(path string) (string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

func digestBytesSHA256(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func normalizeCommitSHA(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if len(normalized) != 40 {
		return "", fmt.Errorf("must be one full 40-character commit SHA")
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", fmt.Errorf("must be lowercase hexadecimal: %w", err)
	}
	return normalized, nil
}

func validateSHA256Digest(value string) error {
	hexDigest := strings.TrimPrefix(strings.TrimSpace(value), "sha256:")
	if len(hexDigest) != sha256.Size*2 || "sha256:"+hexDigest != strings.TrimSpace(value) {
		return fmt.Errorf("must be sha256:<64 lowercase hexadecimal characters>")
	}
	decoded, err := hex.DecodeString(hexDigest)
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("must be sha256:<64 lowercase hexadecimal characters>")
	}
	if hex.EncodeToString(decoded) != hexDigest {
		return fmt.Errorf("must use lowercase hexadecimal")
	}
	return nil
}

func cloneTokenGateCoordinates(input *TokenGateCoordinates) *TokenGateCoordinates {
	if input == nil {
		return nil
	}
	cloned := *input
	return &cloned
}
