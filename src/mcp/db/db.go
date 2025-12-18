package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Holon struct {
	ID        string
	Type      string
	Kind      string
	Layer     string
	Title     string
	Content   string
	ContextID string
	Scope     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	
	// Create tables if not exist (bootstrap)
	schema := `
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
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to init schema: %v", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) GetRawDB() *sql.DB {
	return d.conn
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) CreateHolon(h Holon) error {
	query := `INSERT INTO holons (id, type, kind, layer, title, content, context_id, scope, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var kind, scope interface{}
	if h.Kind == "" {
		kind = nil
	} else {
		kind = h.Kind
	}
	if h.Scope == "" {
		scope = nil
	} else {
		scope = h.Scope
	}
	_, err := d.conn.Exec(query, h.ID, h.Type, kind, h.Layer, h.Title, h.Content, h.ContextID, scope, time.Now(), time.Now())
	return err
}

func (d *DB) RecordWork(id, methodRef, performerRef string, startedAt, endedAt time.Time, ledger string) error {
	query := `INSERT INTO work_records (id, method_ref, performer_ref, started_at, ended_at, resource_ledger, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := d.conn.Exec(query, id, methodRef, performerRef, startedAt, endedAt, ledger, time.Now())
	return err
}

func (d *DB) UpdateHolonLayer(id, layer string) error {
	query := `UPDATE holons SET layer = ?, updated_at = ? WHERE id = ?`
	_, err := d.conn.Exec(query, layer, time.Now(), id)
	return err
}

func (d *DB) AddEvidence(id, holonID, type_, content, verdict, assuranceLevel, carrierRef, validUntil string) error {
	query := `INSERT INTO evidence (id, holon_id, type, content, verdict, assurance_level, carrier_ref, valid_until, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// Handle empty strings as NULL
	var vUntil, aLevel, cRef interface{}
	if validUntil == "" {
		vUntil = nil
	} else {
		vUntil = validUntil
	}
	if assuranceLevel == "" {
		aLevel = nil
	} else {
		aLevel = assuranceLevel
	}
	if carrierRef == "" {
		cRef = nil
	} else {
		cRef = carrierRef
	}
	_, err := d.conn.Exec(query, id, holonID, type_, content, verdict, aLevel, cRef, vUntil, time.Now())
	return err
}

type Evidence struct {
	ID             string
	HolonID        string
	Type           string
	Content        string
	Verdict        string
	AssuranceLevel string
	CarrierRef     string
	CreatedAt      time.Time
}

func (d *DB) GetEvidence(holonID string) ([]Evidence, error) {
	query := `SELECT id, holon_id, type, content, verdict, assurance_level, carrier_ref, created_at FROM evidence WHERE holon_id = ?`
	rows, err := d.conn.Query(query, holonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evidences []Evidence
	for rows.Next() {
		var e Evidence
		var al, cr sql.NullString // Handle potential NULLs for old records
		if err := rows.Scan(&e.ID, &e.HolonID, &e.Type, &e.Content, &e.Verdict, &al, &cr, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.AssuranceLevel = al.String
		e.CarrierRef = cr.String
		evidences = append(evidences, e)
	}
	return evidences, nil
}

func (d *DB) GetEvidenceWithCarrier() ([]Evidence, error) {
	// Query specifically filters for evidence that HAS a carrier reference (NOT NULL and NOT EMPTY)
	query := `SELECT id, holon_id, type, content, verdict, assurance_level, carrier_ref, created_at 
			  FROM evidence 
			  WHERE carrier_ref IS NOT NULL AND carrier_ref != ''`
	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evidences []Evidence
	for rows.Next() {
		var e Evidence
		var al, cr sql.NullString
		if err := rows.Scan(&e.ID, &e.HolonID, &e.Type, &e.Content, &e.Verdict, &al, &cr, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.AssuranceLevel = al.String
		e.CarrierRef = cr.String
		evidences = append(evidences, e)
	}
	return evidences, nil
}

func (d *DB) Link(source, target, relType string) error {
	query := `INSERT INTO relations (source_id, target_id, relation_type, created_at) VALUES (?, ?, ?, ?)`
	_, err := d.conn.Exec(query, source, target, relType, time.Now())
	return err
}