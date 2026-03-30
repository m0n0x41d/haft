package project

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// IndexEntry is a decision summary stored in the cross-project index.
type IndexEntry struct {
	ProjectID     string
	ProjectName   string
	DecisionID    string
	Title         string
	SelectedTitle string
	WhySelected   string
	WeakestLink   string
	PrimaryLang   string // for CL matching
	CreatedAt     string
}

// IndexRecall is a cross-project decision surfaced during framing.
type IndexRecall struct {
	ProjectName string
	DecisionID  string
	Title       string
	WhySelected string
	WeakestLink string
	CL          int     // 2 = similar context, 1 = different context
	Similarity  float64 // FTS5 rank
}

// IndexStore manages the cross-project index at ~/.haft/index.db.
type IndexStore struct {
	db *sql.DB
}

// OpenIndex opens or creates the cross-project index.
func OpenIndex() (*IndexStore, error) {
	path, err := IndexDBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open index DB: %w", err)
	}

	// Create schema if needed
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS global_decisions (
			project_id TEXT NOT NULL,
			project_name TEXT NOT NULL,
			decision_id TEXT NOT NULL,
			title TEXT NOT NULL,
			selected_title TEXT,
			why_selected TEXT,
			weakest_link TEXT,
			primary_lang TEXT,
			created_at TEXT NOT NULL,
			PRIMARY KEY (project_id, decision_id)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS global_fts USING fts5(
			title, selected_title, why_selected, weakest_link,
			content='global_decisions',
			content_rowid='rowid',
			tokenize='porter unicode61'
		)`,
		// Triggers to keep FTS5 in sync
		`CREATE TRIGGER IF NOT EXISTS global_fts_insert AFTER INSERT ON global_decisions BEGIN
			INSERT INTO global_fts(rowid, title, selected_title, why_selected, weakest_link)
			VALUES (new.rowid, new.title, new.selected_title, new.why_selected, new.weakest_link);
		END`,
		`CREATE TRIGGER IF NOT EXISTS global_fts_delete BEFORE DELETE ON global_decisions BEGIN
			DELETE FROM global_fts WHERE rowid = old.rowid;
		END`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				_ = db.Close()
				return nil, fmt.Errorf("index schema: %w", err)
			}
		}
	}

	return &IndexStore{db: db}, nil
}

// Close closes the index DB.
func (s *IndexStore) Close() error {
	return s.db.Close()
}

// WriteDecision writes or updates a decision summary in the index.
func (s *IndexStore) WriteDecision(ctx context.Context, entry IndexEntry) error {
	// Delete old entry first (for FTS5 trigger)
	_, _ = s.db.ExecContext(ctx,
		`DELETE FROM global_decisions WHERE project_id = ? AND decision_id = ?`,
		entry.ProjectID, entry.DecisionID)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_decisions (project_id, project_name, decision_id, title, selected_title, why_selected, weakest_link, primary_lang, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ProjectID, entry.ProjectName, entry.DecisionID,
		entry.Title, entry.SelectedTitle, entry.WhySelected,
		entry.WeakestLink, entry.PrimaryLang, entry.CreatedAt)
	return err
}

// Search finds decisions across all OTHER projects matching the query.
// Returns results sorted by FTS5 relevance, with CL assigned based on language match.
func (s *IndexStore) Search(ctx context.Context, query string, currentProjectID string, currentLang string, limit int) ([]IndexRecall, error) {
	if limit <= 0 {
		limit = 5
	}

	// Sanitize query for FTS5
	query = sanitizeFTS(query)
	if query == "" {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT gd.project_name, gd.decision_id, gd.title, gd.why_selected, gd.weakest_link, gd.primary_lang,
		       rank
		FROM global_fts
		JOIN global_decisions gd ON global_fts.rowid = gd.rowid
		WHERE global_fts MATCH ?
		  AND gd.project_id != ?
		ORDER BY rank
		LIMIT ?`,
		query, currentProjectID, limit)
	if err != nil {
		return nil, fmt.Errorf("index search: %w", err)
	}
	defer rows.Close()

	var results []IndexRecall
	for rows.Next() {
		var r IndexRecall
		var lang string
		var rank float64
		if err := rows.Scan(&r.ProjectName, &r.DecisionID, &r.Title, &r.WhySelected, &r.WeakestLink, &lang, &rank); err != nil {
			continue
		}

		// CL matching: same primary language = CL2, different = CL1
		if currentLang != "" && lang != "" && currentLang == lang {
			r.CL = 2
		} else {
			r.CL = 1
		}
		r.Similarity = -rank // FTS5 rank is negative, higher is better
		results = append(results, r)
	}

	return results, rows.Err()
}

// DetectPrimaryLanguage reads the codebase_modules table to find the dominant language.
func DetectPrimaryLanguage(db *sql.DB) string {
	var lang string
	err := db.QueryRow(`
		SELECT lang FROM codebase_modules
		GROUP BY lang ORDER BY COUNT(*) DESC LIMIT 1`).Scan(&lang)
	if err != nil {
		return ""
	}
	return lang
}

// ftsStopWords is a set of common English function words that add noise to FTS5 queries.
// Lowercase-compared — technical 2-char terms like "Go", "CI", "DB" pass through.
var ftsStopWords = map[string]bool{
	"a": true, "an": true, "am": true, "as": true, "at": true,
	"be": true, "by": true, "do": true, "go": false, // "go" is a language name — keep it
	"he": true, "if": true, "in": true, "is": true, "it": true,
	"me": true, "my": true, "no": true, "of": true, "on": true,
	"or": true, "so": true, "to": true, "up": true, "us": true,
	"we": true, "the": true, "and": true, "but": true, "for": true,
	"not": true, "you": true, "are": true, "was": true, "has": true,
	"had": true, "its": true, "our": true, "can": true, "did": true,
}

func sanitizeFTS(query string) string {
	// Remove FTS5 special characters
	replacer := strings.NewReplacer(
		"+", " ", "-", " ", "~", " ", "*", " ",
		":", " ", "^", " ", "(", " ", ")", " ",
		"\"", " ", "'", " ", ".", " ", ",", " ",
		";", " ", "!", " ", "?", " ", "/", " ",
		"—", " ", "–", " ",
	)
	cleaned := replacer.Replace(query)

	// Split into words, filter 1-char tokens and English stop words
	var words []string
	for _, w := range strings.Fields(cleaned) {
		if len(w) < 2 {
			continue
		}
		if ftsStopWords[strings.ToLower(w)] {
			continue
		}
		words = append(words, w)
	}
	if len(words) == 0 {
		return ""
	}

	// AND-default: implicit AND in FTS5 (space-separated = all terms required)
	return strings.Join(words, " ")
}

// PopulateContextFacts writes project fingerprint data to context_facts table.
func PopulateContextFacts(ctx context.Context, db *sql.DB, projectName string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	facts := map[string]string{
		"project_name": projectName,
	}

	// Primary language from codebase_modules
	lang := DetectPrimaryLanguage(db)
	if lang != "" {
		facts["primary_language"] = lang
	}

	// Module count
	var moduleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM codebase_modules`).Scan(&moduleCount); err == nil {
		facts["module_count"] = fmt.Sprintf("%d", moduleCount)
	}

	// Decision count
	var decCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifacts WHERE kind = 'DecisionRecord' AND status = 'active'`).Scan(&decCount); err == nil {
		facts["decision_count"] = fmt.Sprintf("%d", decCount)
	}

	// Domains from decision contexts
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT context FROM artifacts WHERE context != '' AND status = 'active'`)
	if err == nil {
		var domains []string
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil && d != "" {
				domains = append(domains, d)
			}
		}
		_ = rows.Close()
		if len(domains) > 0 {
			facts["domains"] = strings.Join(domains, ",")
		}
	}

	// Write facts
	for k, v := range facts {
		_, err := db.ExecContext(ctx,
			`INSERT OR REPLACE INTO context_facts (category, content, updated_at) VALUES (?, ?, ?)`,
			k, v, now)
		if err != nil {
			// Non-fatal, table might not exist in old DBs
			if !strings.Contains(err.Error(), "no such table") {
				return fmt.Errorf("write context fact %s: %w", k, err)
			}
		}
	}

	return nil
}

// EnsureDir creates ~/.haft/ if needed, migrating data from ~/.quint-code/ if it exists.
func EnsureDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	haftDir := filepath.Join(homeDir, ".haft")
	oldDir := filepath.Join(homeDir, ".quint-code")

	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		return err
	}

	// Migration: move contents from ~/.quint-code/ → ~/.haft/
	if _, err := os.Stat(oldDir); err == nil {
		migrateHomeDir(oldDir, haftDir)
	}

	return nil
}

// migrateHomeDir moves data from old ~/.quint-code/ to ~/.haft/.
// For the projects/ subdirectory, merges at project level (each project is a subdir).
// For other files/dirs, moves if not already present.
func migrateHomeDir(oldDir, newDir string) {
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return
	}

	migrated := 0
	for _, entry := range entries {
		oldPath := filepath.Join(oldDir, entry.Name())
		newPath := filepath.Join(newDir, entry.Name())

		// Special handling for projects/ — merge at project level
		if entry.Name() == "projects" && entry.IsDir() {
			migrated += mergeProjectDirs(oldPath, newPath)
			continue
		}

		// Skip if already exists in new location
		if _, err := os.Stat(newPath); err == nil {
			continue
		}

		if err := os.Rename(oldPath, newPath); err == nil {
			migrated++
		}
	}

	if migrated > 0 {
		fmt.Fprintf(os.Stderr, "haft: migrated %d items from ~/.quint-code/ → ~/.haft/\n", migrated)
	}

	// Remove old dir if empty (recursive check)
	removeIfEmpty(oldDir)
}

// mergeProjectDirs moves individual project dirs from old to new.
func mergeProjectDirs(oldProjects, newProjects string) int {
	_ = os.MkdirAll(newProjects, 0o755)

	entries, err := os.ReadDir(oldProjects)
	if err != nil {
		return 0
	}

	moved := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		oldPath := filepath.Join(oldProjects, entry.Name())
		newPath := filepath.Join(newProjects, entry.Name())

		if _, err := os.Stat(newPath); err == nil {
			continue // project already exists in new location
		}
		if err := os.Rename(oldPath, newPath); err == nil {
			moved++
		}
	}
	return moved
}

func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	// Remove empty subdirs first
	for _, e := range entries {
		if e.IsDir() {
			sub := filepath.Join(dir, e.Name())
			removeIfEmpty(sub)
		}
	}
	// Re-read after cleaning subdirs
	entries, _ = os.ReadDir(dir)
	if len(entries) == 0 {
		_ = os.Remove(dir)
	}
}
