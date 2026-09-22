package fpfrefresh

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
)

const SourceAccessArchiveRelativePath = "internal/cli/fpf-source.db.gz"

// SourceAccessReport is a source-access lock/receipt. The canonical lock is
// stored inside the atomically replaced archive, alongside its exact corpus.
// Neither this report nor the archive can activate a memory environment.
type SourceAccessReport struct {
	SchemaVersion   string                         `json:"schema_version"`
	SourceRevision  string                         `json:"source_revision"`
	Basis           fpf.SourceAccessBasis          `json:"basis"`
	Publications    []fpf.PublicationManifestEntry `json:"publications"`
	SourceUnitCount int                            `json:"source_unit_count"`
	DatabaseDigest  string                         `json:"database_digest"`
	ArchiveDigest   string                         `json:"archive_digest"`
}

type SourceAccessRequest struct {
	Source             GitSourceRequest
	MemoryDatabasePath string
	ArchivePath        string
}

func acquireSourceAccessSnapshot(ctx context.Context, request GitSourceRequest) (fpf.SourceAccessSnapshot, error) {
	if err := validateSourceObjectRepository(ctx, request.RepositoryPath); err != nil {
		return fpf.SourceAccessSnapshot{}, err
	}
	source, err := AcquireGitSource(ctx, request)
	if err != nil {
		return fpf.SourceAccessSnapshot{}, err
	}
	core, err := buildLogicalPublicationSnapshot(source.ReadmeBytes(), source.SpecificationBytes(), source.CommitSHA())
	if err != nil {
		return fpf.SourceAccessSnapshot{}, err
	}
	inputs := make([]fpf.PublicationInput, 0)
	for _, descriptor := range fpf.SourceAccessPublications() {
		payload, err := readCandidatePublication(ctx, commandGitSourceRunner{}, request.RepositoryPath, source.CommitSHA(), descriptor.Path)
		if err != nil {
			return fpf.SourceAccessSnapshot{}, err
		}
		if descriptor.ID == "engineering-suite" {
			if err := fpf.ValidateEngineeringSuiteMembership(payload); err != nil {
				return fpf.SourceAccessSnapshot{}, err
			}
		}
		input := fpf.PublicationInput{
			Descriptor: descriptor,
			Document:   fpf.SourceDocument{Path: "data/FPF/" + descriptor.Path, SourceRevision: source.CommitSHA(), Markdown: payload},
		}
		inputs = append(inputs, input)
	}
	return fpf.BuildSourceAccessSnapshot(core, inputs)
}

// PrepareSourceAccess evaluates the current compiler, records its exact result,
// and builds a read-only corpus in a disposable workspace. It never changes
// the memory index, source checkout, project ledger or integrated-refresh lock.
func PrepareSourceAccess(ctx context.Context, request SourceAccessRequest) (SourceAccessReport, []byte, error) {
	snapshot, err := acquireSourceAccessSnapshot(ctx, request.Source)
	if err != nil {
		return SourceAccessReport{}, nil, err
	}
	basis, err := sourceAccessBasis(ctx, snapshot, request.MemoryDatabasePath)
	if err != nil {
		return SourceAccessReport{}, nil, err
	}
	workspace, err := os.MkdirTemp("", "haft-source-access-")
	if err != nil {
		return SourceAccessReport{}, nil, err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	databasePath := filepath.Join(workspace, "source.db")
	if err := buildSourceAccessDatabase(databasePath, snapshot, basis); err != nil {
		return SourceAccessReport{}, nil, err
	}
	payload, err := os.ReadFile(databasePath)
	if err != nil {
		return SourceAccessReport{}, nil, err
	}
	archive, err := compressSourceAccess(payload)
	if err != nil {
		return SourceAccessReport{}, nil, err
	}
	units := snapshot.SourceUnits()
	report := SourceAccessReport{
		SchemaVersion:   fpf.SourceAccessSchemaVersion,
		SourceRevision:  snapshot.Core().Revision(),
		Basis:           basis,
		Publications:    snapshot.Manifest(),
		SourceUnitCount: len(units),
		DatabaseDigest:  digestBytesSHA256(payload),
		ArchiveDigest:   digestBytesSHA256(archive),
	}
	return report, archive, nil
}

func sourceAccessBasis(ctx context.Context, snapshot fpf.SourceAccessSnapshot, memoryPath string) (fpf.SourceAccessBasis, error) {
	memoryDigest, err := digestFile(memoryPath)
	if err != nil {
		return fpf.SourceAccessBasis{}, err
	}
	database, err := openIntegrationDatabaseReadOnly(memoryPath)
	if err != nil {
		return fpf.SourceAccessBasis{}, err
	}
	defer func() { _ = database.Close() }()
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(ctx, database)
	if err != nil {
		return fpf.SourceAccessBasis{}, fmt.Errorf("load pinned memory compilation: %w", err)
	}
	meta, err := readRequiredIntegrationMeta(database)
	if err != nil {
		return fpf.SourceAccessBasis{}, err
	}
	ref, exists := artifact.TypeEnvRef()
	if !exists || meta["fpf_commit"] != artifact.SourceRevision().String() || meta["typeenv_artifact_digest"] != artifact.Digest().String() || meta["typeenv_ref"] != ref.String() {
		return fpf.SourceAccessBasis{}, fmt.Errorf("memory compilation metadata is not coherent")
	}
	manifestJSON, err := snapshot.ManifestJSON()
	if err != nil {
		return fpf.SourceAccessBasis{}, err
	}
	compilation, err := typeenv.CompileBaseTypeEnv(snapshot.Core())
	if err != nil {
		return fpf.SourceAccessBasis{}, fmt.Errorf("evaluate current Core compiler: %w", err)
	}
	diagnostics, err := marshalSourceCompilerDiagnostics(compilation.Diagnostics())
	if err != nil {
		return fpf.SourceAccessBasis{}, err
	}
	posture := "accepted_not_activated"
	if compilation.Rejected() {
		posture = "rejected"
	}
	finalMemoryDigest, err := digestFile(memoryPath)
	if err != nil {
		return fpf.SourceAccessBasis{}, err
	}
	if memoryDigest != finalMemoryDigest {
		return fpf.SourceAccessBasis{}, fmt.Errorf("memory database changed while pinning source-access basis")
	}
	return fpf.SourceAccessBasis{
		SchemaVersion:            fpf.SourceAccessSchemaVersion,
		ManifestDigest:           digestBytesSHA256(manifestJSON),
		MemoryDatabaseDigest:     memoryDigest,
		MemorySourceRevision:     artifact.SourceRevision().String(),
		MemoryTypeEnvRef:         ref.String(),
		MemoryTypeEnvDigest:      artifact.Digest().String(),
		CandidateCompilation:     posture,
		CandidateCompilerEdition: compilation.CompilerSchemaVersion().String(),
		CompilerDiagnostics:      diagnostics,
	}, nil
}

func buildSourceAccessDatabase(path string, snapshot fpf.SourceAccessSnapshot, basis fpf.SourceAccessBasis) error {
	units := snapshot.SourceUnits()
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	if _, err := database.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	if err := fpf.StoreSourceAccessPublications(database, snapshot.Manifest()); err != nil {
		return err
	}
	if err := fpf.StoreSourceUnitsDB(database, units); err != nil {
		return err
	}
	basisJSON, err := json.Marshal(basis)
	if err != nil {
		return err
	}
	manifestJSON, err := snapshot.ManifestJSON()
	if err != nil {
		return err
	}
	core := snapshot.Core()
	entries := map[string]string{
		"schema_version":              fpf.SpecIndexSchemaVersion,
		"fpf_commit":                  core.Revision(),
		"readme_document_digest":      core.ReadmeDigest().String(),
		"spec_document_digest":        core.SpecDigest().String(),
		"indexed_source_units":        strconv.Itoa(len(units)),
		"source_access_basis":         string(basisJSON),
		"source_publication_manifest": string(manifestJSON),
	}
	if err := fpf.SetSpecMetaEntries(path, entries); err != nil {
		return err
	}
	return fpf.VerifySourceAccessSnapshot(database, snapshot)
}

func compressSourceAccess(payload []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ApplySourceAccess replaces exactly one archive after preparation. Keeping
// source data and its manifest/basis lock in one file avoids partial publication.
func ApplySourceAccess(request SourceAccessRequest, report SourceAccessReport, archive []byte) error {
	if !filepath.IsAbs(request.ArchivePath) || filepath.Base(request.ArchivePath) != "fpf-source.db.gz" {
		return fmt.Errorf("source publication requires the explicit fpf-source.db.gz archive path")
	}
	if filepath.Clean(request.ArchivePath) == filepath.Clean(request.MemoryDatabasePath) {
		return fmt.Errorf("source publication cannot replace the memory database")
	}

	if digestBytesSHA256(archive) != report.ArchiveDigest {
		return fmt.Errorf("source archive does not match prepared report")
	}
	memoryDigest, err := digestFile(request.MemoryDatabasePath)
	if err != nil {
		return err
	}
	if memoryDigest != report.Basis.MemoryDatabaseDigest {
		return fmt.Errorf("pinned memory database changed before source publication")
	}
	directory := filepath.Dir(request.ArchivePath)
	file, err := os.CreateTemp(directory, ".fpf-source-*")
	if err != nil {
		return err
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := file.Write(archive); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(path, 0644); err != nil {
		return err
	}
	return os.Rename(path, request.ArchivePath)
}

func VerifySourceAccess(ctx context.Context, request SourceAccessRequest) (SourceAccessReport, error) {
	if request.Source.Fetch != nil {
		return SourceAccessReport{}, fmt.Errorf("source verification is read-only; fetch pinned objects separately with source-fetch")
	}
	archive, payload, err := readSourceAccessArchive(request.ArchivePath)
	if err != nil {
		return SourceAccessReport{}, err
	}
	revision, err := sourceAccessRevision(payload)
	if err != nil {
		return SourceAccessReport{}, err
	}
	pinned, err := pinSourceVerification(ctx, request.Source, revision)
	if err != nil {
		return SourceAccessReport{}, err
	}
	request.Source = pinned
	// A deterministic rebuild verifies every source unit, publication, compiler
	// diagnostic and pinned memory coordinate against exact Git objects.
	report, expectedArchive, err := PrepareSourceAccess(ctx, request)
	if err != nil {
		return SourceAccessReport{}, err
	}
	if !bytes.Equal(archive, expectedArchive) || digestBytesSHA256(payload) != report.DatabaseDigest {
		return SourceAccessReport{}, fmt.Errorf("source-access archive differs from exact deterministic publication")
	}
	return report, nil
}

func marshalSourceCompilerDiagnostics(diagnostics []typeenv.CompilerDiagnostic) ([]byte, error) {
	type diagnosticView struct {
		Code    string `json:"code"`
		UnitID  string `json:"unit_id"`
		Message string `json:"message"`
	}
	views := make([]diagnosticView, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		view := diagnosticView{Code: diagnostic.Code(), UnitID: diagnostic.UnitID(), Message: diagnostic.Message()}
		views = append(views, view)
	}
	return json.Marshal(views)
}
