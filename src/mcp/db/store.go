package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS holons (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	kind TEXT,
	layer TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	context_id TEXT NOT NULL,
	scope TEXT,
	cached_r_score REAL DEFAULT 0.0 CHECK(cached_r_score BETWEEN 0.0 AND 1.0),
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS evidence (
	id TEXT PRIMARY KEY,
	holon_id TEXT NOT NULL,
	type TEXT NOT NULL,
	content TEXT NOT NULL,
	verdict TEXT NOT NULL,
	assurance_level TEXT,
	carrier_ref TEXT,
	valid_until DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS relations (
	source_id TEXT NOT NULL,
	target_id TEXT NOT NULL,
	relation_type TEXT NOT NULL,
	congruence_level INTEGER DEFAULT 3 CHECK(congruence_level BETWEEN 0 AND 3),
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (source_id, target_id, relation_type)
);
CREATE TABLE IF NOT EXISTS characteristics (
	id TEXT PRIMARY KEY,
	holon_id TEXT NOT NULL,
	name TEXT NOT NULL,
	scale TEXT NOT NULL,
	value TEXT NOT NULL,
	unit TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(holon_id) REFERENCES holons(id)
);
CREATE TABLE IF NOT EXISTS work_records (
	id TEXT PRIMARY KEY,
	method_ref TEXT NOT NULL,
	performer_ref TEXT NOT NULL,
	started_at DATETIME NOT NULL,
	ended_at DATETIME,
	resource_ledger TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

type Store struct {
	conn *sql.DB
	q    *Queries
}

func NewStore(dbPath string) (*Store, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to init schema: %v", err)
	}

	return &Store{
		conn: conn,
		q:    New(),
	}, nil
}

func (s *Store) GetRawDB() *sql.DB {
	return s.conn
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) CreateHolon(ctx context.Context, id, typ, kind, layer, title, content, contextID, scope string) error {
	now := sql.NullTime{Time: time.Now(), Valid: true}
	return s.q.CreateHolon(ctx, s.conn, CreateHolonParams{
		ID:        id,
		Type:      typ,
		Kind:      toNullString(kind),
		Layer:     layer,
		Title:     title,
		Content:   content,
		ContextID: contextID,
		Scope:     toNullString(scope),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *Store) GetHolon(ctx context.Context, id string) (Holon, error) {
	return s.q.GetHolon(ctx, s.conn, id)
}

func (s *Store) GetHolonTitle(ctx context.Context, id string) (string, error) {
	return s.q.GetHolonTitle(ctx, s.conn, id)
}

func (s *Store) ListAllHolonIDs(ctx context.Context) ([]string, error) {
	return s.q.ListAllHolonIDs(ctx, s.conn)
}

func (s *Store) UpdateHolonLayer(ctx context.Context, id, layer string) error {
	return s.q.UpdateHolonLayer(ctx, s.conn, UpdateHolonLayerParams{
		ID:        id,
		Layer:     layer,
		UpdatedAt: sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) RecordWork(ctx context.Context, id, methodRef, performerRef string, startedAt, endedAt time.Time, ledger string) error {
	return s.q.RecordWork(ctx, s.conn, RecordWorkParams{
		ID:             id,
		MethodRef:      methodRef,
		PerformerRef:   performerRef,
		StartedAt:      startedAt,
		EndedAt:        sql.NullTime{Time: endedAt, Valid: true},
		ResourceLedger: toNullString(ledger),
		CreatedAt:      sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) AddEvidence(ctx context.Context, id, holonID, typ, content, verdict, assuranceLevel, carrierRef, validUntil string) error {
	var vUntil sql.NullTime
	if validUntil != "" {
		t, err := time.Parse(time.RFC3339, validUntil)
		if err != nil {
			t, err = time.Parse("2006-01-02", validUntil)
		}
		if err == nil {
			vUntil = sql.NullTime{Time: t, Valid: true}
		}
	}

	return s.q.AddEvidence(ctx, s.conn, AddEvidenceParams{
		ID:             id,
		HolonID:        holonID,
		Type:           typ,
		Content:        content,
		Verdict:        verdict,
		AssuranceLevel: toNullString(assuranceLevel),
		CarrierRef:     toNullString(carrierRef),
		ValidUntil:     vUntil,
		CreatedAt:      sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) GetEvidence(ctx context.Context, holonID string) ([]Evidence, error) {
	return s.q.GetEvidenceByHolon(ctx, s.conn, holonID)
}

func (s *Store) GetEvidenceWithCarrier(ctx context.Context) ([]Evidence, error) {
	return s.q.GetEvidenceWithCarrier(ctx, s.conn)
}

func (s *Store) Link(ctx context.Context, source, target, relType string) error {
	return s.q.AddRelation(ctx, s.conn, AddRelationParams{
		SourceID:     source,
		TargetID:     target,
		RelationType: relType,
		CreatedAt:    sql.NullTime{Time: time.Now(), Valid: true},
	})
}

func (s *Store) GetComponentsOf(ctx context.Context, targetID string) ([]GetComponentsOfRow, error) {
	return s.q.GetComponentsOf(ctx, s.conn, targetID)
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
