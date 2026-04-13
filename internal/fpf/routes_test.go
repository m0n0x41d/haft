package fpf

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestParseRoutes_NormalizesRouteArtifact(t *testing.T) {
	artifact := `{
		"routes": [
			{
				"id": " boundary-unpacking ",
				"title": " Boundary discipline and routing ",
				"description": " Boundary statements. ",
				"matchers": [" boundary ", "boundary", "contract"],
				"core": ["a6", "A.6"],
				"chain": [" a.6 ", "a.6.b", "A.6.B"]
			}
		]
	}`

	routes, err := ParseRoutes(strings.NewReader(artifact))
	if err != nil {
		t.Fatalf("ParseRoutes failed: %v", err)
	}

	want := []Route{
		{
			ID:          "boundary-unpacking",
			Title:       "Boundary discipline and routing",
			Description: "Boundary statements.",
			Matchers:    []string{"boundary", "contract"},
			Core:        []string{"A.6"},
			Chain:       []string{"A.6", "A.6.B"},
		},
	}
	if !reflect.DeepEqual(routes, want) {
		t.Fatalf("unexpected routes: got %#v want %#v", routes, want)
	}
}

func TestParseRoutes_RejectsInvalidRouteShape(t *testing.T) {
	tests := []struct {
		name     string
		artifact string
		wantErr  string
	}{
		{
			name: "missing matcher",
			artifact: `{
				"routes": [
					{
						"id": "boundary-unpacking",
						"title": "Boundary discipline and routing",
						"description": "Boundary statements.",
						"matchers": [],
						"core": ["A.6"],
						"chain": ["A.6", "A.6.B"]
					}
				]
			}`,
			wantErr: `route "boundary-unpacking" must define at least one matcher`,
		},
		{
			name: "core outside chain",
			artifact: `{
				"routes": [
					{
						"id": "boundary-unpacking",
						"title": "Boundary discipline and routing",
						"description": "Boundary statements.",
						"matchers": ["boundary"],
						"core": ["A.6.C"],
						"chain": ["A.6", "A.6.B"]
					}
				]
			}`,
			wantErr: `route "boundary-unpacking" core pattern "A.6.C" must also appear in chain`,
		},
		{
			name: "invalid core pattern id",
			artifact: `{
				"routes": [
					{
						"id": "boundary-unpacking",
						"title": "Boundary discipline and routing",
						"description": "Boundary statements.",
						"matchers": ["boundary"],
						"core": ["A..6"],
						"chain": ["A.6", "A.6.B"]
					}
				]
			}`,
			wantErr: `route "boundary-unpacking" has invalid core pattern "A..6"`,
		},
		{
			name: "invalid chain pattern id",
			artifact: `{
				"routes": [
					{
						"id": "boundary-unpacking",
						"title": "Boundary discipline and routing",
						"description": "Boundary statements.",
						"matchers": ["boundary"],
						"core": ["A.6"],
						"chain": ["A.6", "Z.1"]
					}
				]
			}`,
			wantErr: `route "boundary-unpacking" has invalid chain pattern "Z.1"`,
		},
		{
			name: "missing resolved sections",
			artifact: `{
				"routes": [
					{
						"id": "boundary-unpacking",
						"title": "Boundary discipline and routing",
						"description": "Boundary statements.",
						"matchers": ["boundary"],
						"core": [],
						"chain": []
					}
				]
			}`,
			wantErr: `route "boundary-unpacking" must define at least one resolved core or chain pattern`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseRoutes(strings.NewReader(test.artifact))
			if err == nil {
				t.Fatal("ParseRoutes unexpectedly succeeded")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("unexpected error: got %q want substring %q", err.Error(), test.wantErr)
			}
		})
	}
}

func TestBuildSpecIndex_RejectsRoutesWithMalformedPatternIDs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	routes := []Route{
		{
			ID:          "boundary-unpacking",
			Title:       "Boundary discipline and routing",
			Description: "Boundary statements.",
			Matchers:    []string{"boundary", "contract"},
			Core:        []string{"A..6"},
			Chain:       []string{"A.6", "A.6.B"},
		},
	}

	chunks := []SpecChunk{
		{ID: 0, Heading: "A.6 - Boundary", Level: 2, Body: "Boundary statements.", PatternID: "A.6"},
		{ID: 1, Heading: "A.6.B - Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
	}

	err := BuildSpecIndex(dbPath, chunks, routes)
	if err == nil {
		t.Fatal("BuildSpecIndex unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), `route "boundary-unpacking" has invalid core pattern "A..6"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildSpecIndex_PersistsRoutesFromArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	routePath := filepath.Join(tmpDir, "routes.json")
	dbPath := filepath.Join(tmpDir, "test.db")

	artifact := `{
		"routes": [
			{
				"id": "boundary-unpacking",
				"title": "Boundary discipline and routing",
				"description": "Boundary statements.",
				"matchers": ["boundary", "contract"],
				"core": ["A.6", "A.6.B"],
				"chain": ["A.6", "A.6.B", "A.6.C"]
			}
		]
	}`
	if err := os.WriteFile(routePath, []byte(artifact), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	routes, err := LoadRoutes(routePath)
	if err != nil {
		t.Fatalf("LoadRoutes failed: %v", err)
	}

	chunks := []SpecChunk{
		{ID: 0, Heading: "A.6 - Boundary", Level: 2, Body: "Boundary statements.", PatternID: "A.6"},
		{ID: 1, Heading: "A.6.B - Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
		{ID: 2, Heading: "A.6.C - Promise Clause", Level: 2, Body: "Promise clauses.", PatternID: "A.6.C"},
	}
	if err := BuildSpecIndex(dbPath, chunks, routes); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	var routeID string
	var title string
	var description string
	var matchersJSON string
	var coreJSON string
	var chainJSON string
	err = db.QueryRow(`
		SELECT route_id, title, description, matchers_json, core_json, chain_json
		FROM routes
		WHERE route_id = ?
	`, "boundary-unpacking").Scan(&routeID, &title, &description, &matchersJSON, &coreJSON, &chainJSON)
	if err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}

	if routeID != "boundary-unpacking" {
		t.Fatalf("unexpected route id: %q", routeID)
	}
	if title != "Boundary discipline and routing" {
		t.Fatalf("unexpected title: %q", title)
	}
	if description != "Boundary statements." {
		t.Fatalf("unexpected description: %q", description)
	}
	if matchersJSON != `["boundary","contract"]` {
		t.Fatalf("unexpected matchers json: %q", matchersJSON)
	}
	if coreJSON != `["A.6","A.6.B"]` {
		t.Fatalf("unexpected core json: %q", coreJSON)
	}
	if chainJSON != `["A.6","A.6.B","A.6.C"]` {
		t.Fatalf("unexpected chain json: %q", chainJSON)
	}
}

func TestBuildSpecIndex_RejectsRoutesReferencingUnknownSections(t *testing.T) {
	tmpDir := t.TempDir()
	routePath := filepath.Join(tmpDir, "routes.json")
	dbPath := filepath.Join(tmpDir, "test.db")

	artifact := `{
		"routes": [
			{
				"id": "boundary-unpacking",
				"title": "Boundary discipline and routing",
				"description": "Boundary statements.",
				"matchers": ["boundary", "contract"],
				"core": ["A.6", "A.6.X"],
				"chain": ["A.6", "A.6.B", "A.6.X"]
			}
		]
	}`
	if err := os.WriteFile(routePath, []byte(artifact), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	routes, err := LoadRoutes(routePath)
	if err != nil {
		t.Fatalf("LoadRoutes failed: %v", err)
	}

	chunks := []SpecChunk{
		{ID: 0, Heading: "A.6 - Boundary", Level: 2, Body: "Boundary statements.", PatternID: "A.6"},
		{ID: 1, Heading: "A.6.B - Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
	}

	err = BuildSpecIndex(dbPath, chunks, routes)
	if err == nil {
		t.Fatal("BuildSpecIndex unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), `route "boundary-unpacking" references unknown section "A.6.X"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
