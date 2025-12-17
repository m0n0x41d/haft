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
		layer TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		context_id TEXT NOT NULL,
		scope TEXT,
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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (source_id, target_id, relation_type)
	);
	`
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to init schema: %v", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) CreateHolon(h Holon) error {
	query := `INSERT INTO holons (id, type, kind, layer, title, content, context_id, scope, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := d.conn.Exec(query, h.ID, h.Type, h.Kind, h.Layer, h.Title, h.Content, h.ContextID, h.Scope, time.Now(), time.Now())
	return err
}

func (d *DB) UpdateHolonLayer(id, layer string) error {
	query := `UPDATE holons SET layer = ?, updated_at = ? WHERE id = ?`
	_, err := d.conn.Exec(query, layer, time.Now(), id)
	return err
}

func (d *DB) AddEvidence(id, holonID, type_, content, verdict, assuranceLevel, carrierRef, validUntil string) error {
	query := `INSERT INTO evidence (id, holon_id, type, content, verdict, assurance_level, carrier_ref, valid_until, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := d.conn.Exec(query, id, holonID, type_, content, verdict, assuranceLevel, carrierRef, validUntil, time.Now())
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

func (d *DB) Link(source, target, relType string) error {
	query := `INSERT INTO relations (source_id, target_id, relation_type, created_at) VALUES (?, ?, ?, ?)`
	_, err := d.conn.Exec(query, source, target, relType, time.Now())
	return err
}
