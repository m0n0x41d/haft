package db

const evidenceCarrierSchemaVersion59 = 59

// evidenceCarrierMigration59 is additive but snapshot-backed because a 9.0.3
// binary treats schema 59 as future. Restoring the verified schema-58 snapshot
// is the explicit binary-downgrade path.
var evidenceCarrierMigration59 = Migration{
	Version:         evidenceCarrierSchemaVersion59,
	ServeActivation: ServeActivationAutomaticWithSnapshot,
	Description:     "Persist evidence carrier state and queue legacy carrier backfill",
	Statements: []string{
		"ALTER TABLE evidence_items ADD COLUMN causal_support_basis TEXT DEFAULT ''",
		"ALTER TABLE evidence_items ADD COLUMN updated_at TEXT",
		"UPDATE evidence_items SET updated_at = created_at WHERE updated_at IS NULL OR trim(updated_at) = ''",
		`CREATE TABLE IF NOT EXISTS evidence_carrier_projection_debt (
			evidence_id TEXT PRIMARY KEY,
			artifact_ref TEXT NOT NULL,
			carrier_path TEXT NOT NULL,
			desired_digest TEXT NOT NULL,
			last_error TEXT NOT NULL,
			opened_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(evidence_id) REFERENCES evidence_items(id) ON DELETE CASCADE,
			FOREIGN KEY(artifact_ref) REFERENCES artifacts(id)
		)`,
		"CREATE INDEX IF NOT EXISTS idx_evidence_carrier_projection_debt_parent ON evidence_carrier_projection_debt(artifact_ref)",
		`INSERT OR IGNORE INTO evidence_carrier_projection_debt (
			evidence_id, artifact_ref, carrier_path, desired_digest,
			last_error, opened_at, updated_at
		)
		SELECT
			id,
			artifact_ref,
			'.haft/evidence/' || id || '.md',
			'',
			'legacy EvidenceRecord carrier backfill required; run haft sync',
			strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		FROM evidence_items`,
	},
}
