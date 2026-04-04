package artifact

import (
	"context"
	"time"
)

// ArtifactStore defines the persistence contract for artifacts.
// Domain functions depend on this interface, not the concrete Store.
// This enables swapping implementations (SQLite, server-backed, in-memory for tests).
type ArtifactStore interface {
	// CRUD
	Create(ctx context.Context, a *Artifact) error
	Get(ctx context.Context, id string) (*Artifact, error)
	Update(ctx context.Context, a *Artifact) error

	// Listing
	ListByKind(ctx context.Context, kind Kind, limit int) ([]*Artifact, error)
	ListActiveByKind(ctx context.Context, kind Kind, limit int) ([]*Artifact, error)
	ListByContext(ctx context.Context, contextName string) ([]*Artifact, error)
	ListActive(ctx context.Context, limit int) ([]*Artifact, error)

	// Search
	Search(ctx context.Context, query string, limit int) ([]*Artifact, error)
	SearchByAffectedFile(ctx context.Context, filePath string) ([]*Artifact, error)

	// Staleness
	FindStaleDecisions(ctx context.Context) ([]*Artifact, error)
	FindStaleArtifacts(ctx context.Context) ([]*Artifact, error)

	// Sequences
	NextSequence(ctx context.Context, kind Kind) (int, error)

	// Links
	AddLink(ctx context.Context, sourceID, targetID, linkType string) error
	GetLinks(ctx context.Context, artifactID string) ([]Link, error)
	GetBacklinks(ctx context.Context, artifactID string) ([]Link, error)

	// Affected files
	SetAffectedFiles(ctx context.Context, artifactID string, files []AffectedFile) error
	GetAffectedFiles(ctx context.Context, artifactID string) ([]AffectedFile, error)

	// Affected symbols (tree-sitter powered, symbol-level baselines)
	SetAffectedSymbols(ctx context.Context, artifactID string, symbols []AffectedSymbol) error
	GetAffectedSymbols(ctx context.Context, artifactID string) ([]AffectedSymbol, error)

	// Evidence
	AddEvidenceItem(ctx context.Context, item *EvidenceItem, artifactRef string) error
	GetEvidenceItems(ctx context.Context, artifactRef string) ([]EvidenceItem, error)
	SupersedeEvidenceByType(ctx context.Context, artifactRef string, evidenceType string) error

	// Timing
	LastRefreshScan(ctx context.Context) time.Time
	EpistemicDebtBudget(ctx context.Context) (float64, error)
}
