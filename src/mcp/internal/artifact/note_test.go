package artifact

import (
	"context"
	"testing"
)

func TestValidateNote_MissingRationale(t *testing.T) {
	v := ValidateNote(context.Background(), nil, NoteInput{
		Title:     "Switch to gRPC",
		Rationale: "",
	})
	if v.OK {
		t.Error("expected validation to fail for missing rationale")
	}
	if len(v.Warnings) == 0 {
		t.Error("expected warnings")
	}
}

func TestValidateNote_WhitespaceRationale(t *testing.T) {
	v := ValidateNote(context.Background(), nil, NoteInput{
		Title:     "Use Redis",
		Rationale: "   ",
	})
	if v.OK {
		t.Error("expected validation to fail for whitespace-only rationale")
	}
}

func TestValidateNote_ShortRationaleWithFiles(t *testing.T) {
	v := ValidateNote(context.Background(), nil, NoteInput{
		Title:         "Use mutex",
		Rationale:     "faster",
		AffectedFiles: []string{"cache.go"},
	})
	if !v.OK {
		t.Error("should pass but with warning")
	}
	if len(v.Warnings) == 0 {
		t.Error("expected warning about short rationale")
	}
}

func TestValidateNote_ValidNote(t *testing.T) {
	v := ValidateNote(context.Background(), nil, NoteInput{
		Title:     "RWMutex for session cache",
		Rationale: "Contention is below 0.1% based on load test results from last week",
	})
	if !v.OK {
		t.Error("expected validation to pass")
	}
	if len(v.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", v.Warnings)
	}
}

func TestValidateNote_ArchitecturalKeywords(t *testing.T) {
	tests := []struct {
		title     string
		rationale string
	}{
		{"migrate to microservices", "better scalability"},
		{"Use new lib", "need to rewrite the auth module"},
		{"Architecture change", "redesign the data layer for performance"},
	}

	for _, tt := range tests {
		v := ValidateNote(context.Background(), nil, NoteInput{
			Title:     tt.title,
			Rationale: tt.rationale,
		})
		if v.Suggest != "/q-frame" {
			t.Errorf("expected /q-frame suggestion for %q, got %q", tt.title, v.Suggest)
		}
	}
}

func TestValidateNote_TooManyFiles(t *testing.T) {
	v := ValidateNote(context.Background(), nil, NoteInput{
		Title:         "Refactor logging",
		Rationale:     "Switching from fmt to zerolog for structured logging",
		AffectedFiles: []string{"a.go", "b.go", "c.go", "d.go"},
	})
	if v.Suggest != "/q-frame" {
		t.Errorf("expected /q-frame suggestion for >3 files, got %q", v.Suggest)
	}
}

func TestValidateNote_ConflictDetection(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Create an active decision about PostgreSQL
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "PostgreSQL as single data store", Status: StatusActive},
		Body: "All persistent state in PostgreSQL",
	})
	store.SetAffectedFiles(ctx, "dec-001", []AffectedFile{{Path: "internal/db/store.go"}})

	// Try to note something that conflicts by file
	v := ValidateNote(ctx, store, NoteInput{
		Title:         "Use MongoDB for sessions",
		Rationale:     "Need document store for flexible session data",
		AffectedFiles: []string{"internal/db/store.go"},
	})

	if len(v.Conflicts) == 0 {
		t.Error("expected conflict with PostgreSQL decision")
	}
}

func TestCreateNote_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	input := NoteInput{
		Title:         "RWMutex for session cache",
		Rationale:     "Contention below 0.1 percent based on load test",
		AffectedFiles: []string{"internal/auth/cache.go"},
		Context:       "auth",
		Evidence:      "Load test: p99 lock wait <2us at 5k RPS",
	}

	a, filePath, err := CreateNote(ctx, store, quintDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Kind != KindNote {
		t.Errorf("kind = %q, want Note", a.Meta.Kind)
	}
	if a.Meta.Mode != ModeNote {
		t.Errorf("mode = %q, want note", a.Meta.Mode)
	}
	if filePath == "" {
		t.Error("file path should not be empty")
	}

	// Verify DB
	got, err := store.Get(ctx, a.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.Title != "RWMutex for session cache" {
		t.Errorf("title = %q", got.Meta.Title)
	}

	// Verify affected files
	files, err := store.GetAffectedFiles(ctx, a.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "internal/auth/cache.go" {
		t.Errorf("affected files = %+v", files)
	}

	// Verify searchable
	results, err := store.Search(ctx, "RWMutex cache", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("note not found via search")
	}
}
