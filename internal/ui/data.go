// Package ui implements the interactive terminal dashboard for haft.
// Read-only — never writes to DB.
package ui

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/reff"
)

// BoardData holds all data needed by the dashboard, loaded once at startup.
type BoardData struct {
	ProjectName string
	ProjectRoot string

	// Decisions
	Decisions       []*artifact.Artifact
	ShippedCount    int
	PendingCount    int
	DecisionREff    map[string]float64 // decision ID → R_eff
	DecisionDrift   map[string]string  // decision ID → "clean"/"drift"/"no baseline"
	DecisionShipped map[string]bool    // decision ID → true if has measurement

	// Problems
	BacklogProblems    []*artifact.Artifact
	InProgressProblems []*artifact.Artifact
	AddressedCount     int

	// Notes
	RecentNotes []*artifact.Artifact

	// Modules
	CoverageReport *codebase.CoverageReport

	// Health
	StaleItems    []artifact.StaleItem
	CriticalCount int // exit code = 1 if > 0

	// Activity
	RecentActivity []ActivityItem

	// Expiring
	ExpiringSoon []ExpiringItem

	// Depth distribution
	TacticalCount int
	StandardCount int
	DeepCount     int

	// Evidence stats
	EvidenceTotal   int
	EvidenceAvgAge  int // days
	EvidenceOldest  int // days
	EvidenceExpired int

	// Context groups
	ContextGroups map[string]int // context name → decision count
}

// ActivityItem is a recent event for the timeline.
type ActivityItem struct {
	Date  time.Time
	Kind  string
	Title string
	ID    string
}

// ExpiringItem is a decision expiring within 30 days.
type ExpiringItem struct {
	ID        string
	Title     string
	ExpiresIn int // days
}

// LoadBoardData populates all dashboard data from the Store.
func LoadBoardData(store *artifact.Store, db *sql.DB, projectName, projectRoot string) (*BoardData, error) {
	ctx := context.Background()
	data := &BoardData{
		ProjectName:     projectName,
		ProjectRoot:     projectRoot,
		DecisionREff:    make(map[string]float64),
		DecisionDrift:   make(map[string]string),
		DecisionShipped: make(map[string]bool),
		ContextGroups:   make(map[string]int),
	}

	// Decisions
	decisions, err := store.ListByKind(ctx, artifact.KindDecisionRecord, 100)
	if err != nil {
		return nil, fmt.Errorf("load decisions: %w", err)
	}
	data.Decisions = filterActive(decisions)

	for _, d := range data.Decisions {
		shipped := hasMeasurement(ctx, store, d.Meta.ID)
		data.DecisionShipped[d.Meta.ID] = shipped
		if shipped {
			data.ShippedCount++
		} else {
			data.PendingCount++
		}

		// R_eff
		data.DecisionREff[d.Meta.ID] = computeREff(ctx, store, d.Meta.ID)

		// Context groups
		ctx_ := d.Meta.Context
		if ctx_ == "" {
			ctx_ = "(no context)"
		}
		data.ContextGroups[ctx_]++

		// Depth distribution
		switch d.Meta.Mode {
		case artifact.ModeTactical:
			data.TacticalCount++
		case artifact.ModeStandard:
			data.StandardCount++
		case artifact.ModeDeep:
			data.DeepCount++
		}
	}

	// Drift status
	driftReports, _ := artifact.CheckDrift(ctx, store, projectRoot)
	for _, r := range driftReports {
		if !r.HasBaseline {
			data.DecisionDrift[r.DecisionID] = "no baseline"
			continue
		}
		hasDrift := false
		for _, f := range r.Files {
			if f.Status == artifact.DriftModified || f.Status == artifact.DriftMissing {
				hasDrift = true
				break
			}
		}
		if hasDrift {
			data.DecisionDrift[r.DecisionID] = "drift"
		} else {
			data.DecisionDrift[r.DecisionID] = "clean"
		}
	}

	// Problems
	problems, _ := store.ListByKind(ctx, artifact.KindProblemCard, 100)
	for _, p := range problems {
		if p.Meta.Status != artifact.StatusActive {
			continue
		}
		hasPortfolio := hasLinkedKind(ctx, store, p.Meta.ID, artifact.KindSolutionPortfolio)
		hasDecision := hasLinkedKind(ctx, store, p.Meta.ID, artifact.KindDecisionRecord)
		switch {
		case hasDecision:
			data.AddressedCount++
		case hasPortfolio:
			data.InProgressProblems = append(data.InProgressProblems, p)
		default:
			data.BacklogProblems = append(data.BacklogProblems, p)
		}
	}

	// Notes
	notes, _ := store.ListByKind(ctx, artifact.KindNote, 5)
	data.RecentNotes = filterActive(notes)

	// Stale items
	data.StaleItems, _ = artifact.ScanStale(ctx, store, projectRoot)
	for _, item := range data.StaleItems {
		if item.DaysStale > 30 {
			data.CriticalCount++
		}
	}

	// Check R_eff < 0.3
	for _, r := range data.DecisionREff {
		if r > 0 && r < 0.3 {
			data.CriticalCount++
		}
	}

	// Coverage
	scanner := codebase.NewScanner(db)
	if !scanner.ModulesLastScanned(ctx).IsZero() {
		report, err := codebase.ComputeCoverage(ctx, db)
		if err == nil && report.TotalModules > 0 {
			data.CoverageReport = report
		}
	}

	// Expiring soon (within 30 days)
	now := time.Now()
	for _, d := range data.Decisions {
		if d.Meta.ValidUntil == "" {
			continue
		}
		exp, err := time.Parse(time.RFC3339, d.Meta.ValidUntil)
		if err != nil {
			continue
		}
		daysLeft := int(exp.Sub(now).Hours() / 24)
		if daysLeft > 0 && daysLeft <= 30 {
			data.ExpiringSoon = append(data.ExpiringSoon, ExpiringItem{
				ID:        d.Meta.ID,
				Title:     d.Meta.Title,
				ExpiresIn: daysLeft,
			})
		}
	}

	// Activity (last 7 days)
	sevenDaysAgo := now.Add(-7 * 24 * time.Hour)
	allArtifacts, _ := store.ListByKind(ctx, "", 50) // all kinds
	for _, a := range allArtifacts {
		if a.Meta.CreatedAt.After(sevenDaysAgo) {
			data.RecentActivity = append(data.RecentActivity, ActivityItem{
				Date:  a.Meta.CreatedAt,
				Kind:  string(a.Meta.Kind),
				Title: a.Meta.Title,
				ID:    a.Meta.ID,
			})
		}
	}

	// Evidence stats
	loadEvidenceStats(ctx, db, data)

	return data, nil
}

func filterActive(artifacts []*artifact.Artifact) []*artifact.Artifact {
	var result []*artifact.Artifact
	for _, a := range artifacts {
		if a.Meta.Status == artifact.StatusActive || a.Meta.Status == artifact.StatusRefreshDue {
			result = append(result, a)
		}
	}
	return result
}

func hasMeasurement(ctx context.Context, store *artifact.Store, decisionID string) bool {
	evidence, err := store.GetEvidenceItems(ctx, decisionID)
	if err != nil {
		return false
	}
	for _, e := range evidence {
		if e.Type == "measurement" && e.Verdict != "superseded" {
			return true
		}
	}
	return false
}

func computeREff(ctx context.Context, store *artifact.Store, artifactID string) float64 {
	evidence, err := store.GetEvidenceItems(ctx, artifactID)
	if err != nil || len(evidence) == 0 {
		return 0
	}
	now := time.Now()
	minScore := 1.0
	hasActive := false
	for _, e := range evidence {
		if e.Verdict == "superseded" {
			continue
		}
		score := reff.ScoreEvidence(e.Verdict, e.CongruenceLevel, e.ValidUntil, now)
		if score < minScore {
			minScore = score
		}
		hasActive = true
	}
	if !hasActive {
		return 0
	}
	return minScore
}

func hasLinkedKind(ctx context.Context, store *artifact.Store, artifactID string, kind artifact.Kind) bool {
	backlinks, err := store.GetBacklinks(ctx, artifactID)
	if err != nil {
		return false
	}
	for _, link := range backlinks {
		a, err := store.Get(ctx, link.Ref)
		if err != nil {
			continue
		}
		if a.Meta.Kind == kind && (a.Meta.Status == artifact.StatusActive || a.Meta.Status == artifact.StatusRefreshDue) {
			return true
		}
	}
	return false
}

func loadEvidenceStats(ctx context.Context, db *sql.DB, data *BoardData) {
	now := time.Now()

	rows, err := db.QueryContext(ctx, `SELECT created_at, valid_until, verdict FROM evidence_items WHERE verdict != 'superseded'`)
	if err != nil {
		return
	}
	defer rows.Close()

	var totalAge float64
	oldest := 0
	for rows.Next() {
		var createdAt, validUntil, verdict string
		if rows.Scan(&createdAt, &validUntil, &verdict) != nil {
			continue
		}
		data.EvidenceTotal++

		created, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			continue
		}
		age := int(now.Sub(created).Hours() / 24)
		totalAge += float64(age)
		if age > oldest {
			oldest = age
		}

		if validUntil != "" {
			exp, err := time.Parse(time.RFC3339, validUntil)
			if err == nil && exp.Before(now) {
				data.EvidenceExpired++
			}
		}
	}

	data.EvidenceOldest = oldest
	if data.EvidenceTotal > 0 {
		data.EvidenceAvgAge = int(totalAge / float64(data.EvidenceTotal))
	}
}
