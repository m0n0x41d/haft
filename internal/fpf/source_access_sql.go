package fpf

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
)

const SourceAccessSchemaVersion = "haft.source-access/v1"

// SourceAccessBasis states why this readable corpus is separate from the
// installed memory compilation. It is never a successor-activation receipt.
type SourceAccessBasis struct {
	SchemaVersion            string          `json:"schema_version"`
	ManifestDigest           string          `json:"manifest_digest"`
	MemoryDatabaseDigest     string          `json:"memory_database_digest"`
	MemorySourceRevision     string          `json:"memory_source_revision"`
	MemoryTypeEnvRef         string          `json:"memory_type_env_ref"`
	MemoryTypeEnvDigest      string          `json:"memory_type_env_digest"`
	CandidateCompilerEdition string          `json:"candidate_compiler_edition"`
	CandidateCompilation     string          `json:"candidate_compilation"`
	CompilerDiagnostics      json.RawMessage `json:"compiler_diagnostics"`
}

func StoreSourceAccessPublications(db *sql.DB, manifest []PublicationManifestEntry) error {
	_, err := db.Exec(`CREATE TABLE source_publications (source_path TEXT PRIMARY KEY, publication_json TEXT NOT NULL)`)
	if err != nil {
		return fmt.Errorf("create source publication table: %w", err)
	}
	for _, entry := range manifest {
		payload, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		_, err = db.Exec(`INSERT INTO source_publications VALUES (?, ?)`, "data/FPF/"+entry.Path, string(payload))
		if err != nil {
			return fmt.Errorf("store publication %s: %w", entry.ID, err)
		}
	}
	return nil
}

func loadSourcePublications(db *sql.DB) (map[string]PublicationManifestEntry, error) {
	var count int
	err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'source_publications'`).Scan(&count)
	if err != nil {
		return nil, err
	}
	entries := map[string]PublicationManifestEntry{}
	if count == 0 {
		return entries, nil
	}
	rows, err := db.Query(`SELECT source_path, publication_json FROM source_publications ORDER BY source_path`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var path, payload string
		if err := rows.Scan(&path, &payload); err != nil {
			return nil, err
		}
		var entry PublicationManifestEntry
		if err := json.Unmarshal([]byte(payload), &entry); err != nil {
			return nil, err
		}
		entries[path] = entry
	}
	return entries, rows.Err()
}

func hydrateSourcePublications(db *sql.DB, units []SourceUnit) error {
	entries, err := loadSourcePublications(db)
	if err != nil {
		return err
	}
	for path, entry := range entries {
		projected := attachPublicationToUnits(units, entry, path)
		copy(units, projected)
	}
	return nil
}

func VerifySourceAccessSnapshot(db *sql.DB, snapshot SourceAccessSnapshot) error {
	if err := VerifySourceQueryIndexReadOnlyDB(db); err != nil {
		return err
	}
	stored, err := loadSourceUnits(db, "", nil)
	if err != nil {
		return err
	}
	expected := snapshot.SourceUnits()
	if len(stored) != len(expected) {
		return fmt.Errorf("source-access count mismatch: %d != %d", len(stored), len(expected))
	}
	byID := map[string]SourceUnit{}
	for _, unit := range expected {
		byID[unit.UnitID] = unit
	}
	for _, unit := range stored {
		want, exists := byID[unit.UnitID]
		if !exists {
			return fmt.Errorf("unexpected source unit %s", unit.UnitID)
		}
		gotJSON, err := json.Marshal(unit)
		if err != nil {
			return err
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			return err
		}
		if !bytes.Equal(gotJSON, wantJSON) {
			return fmt.Errorf("source unit %s differs from exact publication", unit.UnitID)
		}
	}
	return nil
}

// VerifySourceAccessBasisDB binds source access to the unchanged embedded memory
// image. It checks publication identity independently of SQLite/FTS integrity.
func VerifySourceAccessBasisDB(db *sql.DB, memoryDatabaseDigest string) error {
	basisJSON, err := GetSpecMeta(db, "source_access_basis")
	if err != nil {
		return err
	}
	var basis SourceAccessBasis
	if err := json.Unmarshal([]byte(basisJSON), &basis); err != nil {
		return err
	}
	if basis.SchemaVersion != SourceAccessSchemaVersion {
		return fmt.Errorf("unsupported source-access schema %q", basis.SchemaVersion)
	}
	if basis.MemoryDatabaseDigest != memoryDatabaseDigest {
		return fmt.Errorf("source-access memory basis differs from the embedded memory database; rebuild source access without activating a memory successor")
	}
	if !isCanonicalSourceRevision(basis.MemorySourceRevision) || !isCanonicalSHA256Digest(basis.MemoryTypeEnvDigest) {
		return fmt.Errorf("invalid pinned memory basis")
	}
	if basis.MemoryTypeEnvRef != "typeenv:"+basis.MemoryTypeEnvDigest {
		return fmt.Errorf("pinned memory TypeEnv ref and digest differ")
	}
	if basis.CandidateCompilation != "rejected" && basis.CandidateCompilation != "accepted_not_activated" {
		return fmt.Errorf("unknown source compiler posture %q", basis.CandidateCompilation)
	}
	manifestJSON, err := GetSpecMeta(db, "source_publication_manifest")
	if err != nil {
		return err
	}
	if "sha256:"+sourceContentHash(manifestJSON) != basis.ManifestDigest {
		return fmt.Errorf("source-access manifest digest mismatch")
	}
	var manifest []PublicationManifestEntry
	if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
		return err
	}
	entries, err := loadSourcePublications(db)
	if err != nil {
		return err
	}
	if len(entries) != len(manifest) || len(entries) == 0 {
		return fmt.Errorf("source-access publication table differs from its manifest")
	}
	revision, err := GetSpecMeta(db, "fpf_commit")
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, entry := range manifest {
		if seen[entry.ID] {
			return fmt.Errorf("duplicate manifest publication %s", entry.ID)
		}
		seen[entry.ID] = true
		stored, exists := entries["data/FPF/"+entry.Path]
		if !exists || stored != entry {
			return fmt.Errorf("source-access publication %s differs from its manifest", entry.ID)
		}
		if entry.SourceRevision != revision || !isCanonicalSHA256Digest(entry.DocumentDigest) {
			return fmt.Errorf("source-access publication %s has invalid provenance", entry.ID)
		}
	}
	var missing int
	err = db.QueryRow(`SELECT count(*) FROM source_units WHERE source_path NOT IN (SELECT source_path FROM source_publications)`).Scan(&missing)
	if err != nil {
		return err
	}
	if missing != 0 {
		return fmt.Errorf("source-access index has %d units outside its publication manifest", missing)
	}
	return nil
}
