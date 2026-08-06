package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
)

func TestFPFCommandRegistersOnlyCanonicalQueryCommands(t *testing.T) {
	t.Parallel()

	names := make([]string, 0)
	for _, command := range fpfCmd.Commands() {
		names = append(names, command.Name())
	}
	slices.Sort(names)

	want := []string{"inspect", "lookup", "query"}
	if !slices.Equal(names, want) {
		t.Fatalf("haft fpf commands = %#v, want %#v", names, want)
	}
	for _, retired := range []string{"search", "section", "info"} {
		if slices.Contains(names, retired) {
			t.Fatalf("retired haft fpf %s alias remains registered", retired)
		}
	}
}

func TestFPFConcernQueryDoesNotExposeSourceRoleSelection(t *testing.T) {
	t.Parallel()

	if fpfQueryCmd.Flags().Lookup("role") != nil {
		t.Fatal("haft fpf query exposes --role; concern retrieval must remain navigation-only")
	}
	if fpfLookupCmd.Flags().Lookup("role") == nil {
		t.Fatal("haft fpf lookup lost exact source-role selection")
	}
	if fpfInspectCmd.Flags().Lookup("role") == nil {
		t.Fatal("haft fpf inspect lost exact source-role selection")
	}
}

func TestRunFPFQueryEmitsSourceNativeCandidateSetJSON(t *testing.T) {
	t.Parallel()

	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubFPFQueryFlags(t)
	defer restoreFlags()

	fpfQueryKnownContext = []string{"system boundaries"}
	fpfQueryMaxTotalCandidates = 2
	fpfQueryMaxRelationsPerCandidate = 1
	fpfQueryJSON = true

	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	if err := runFPFQuery(command, []string{"strict", "distinctions"}); err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("query output is not JSON: %v\n%s", err, output.String())
	}
	if payload["kind"] != string(fpf.QueryResultKindCandidateSet) {
		t.Fatalf("kind = %#v, want candidate_set: %s", payload["kind"], output.String())
	}
	truncation := payload["truncation"].(map[string]any)
	budget := truncation["budget"].(map[string]any)
	if budget["max_relations_per_candidate"] != float64(1) {
		t.Fatalf("relation budget did not reach query core: %#v", budget)
	}
	if strings.Contains(output.String(), "── Haft") {
		t.Fatalf("query output contains project navigation footer: %s", output.String())
	}
}

func TestRunFPFInspectRejectsUnknownSourceRole(t *testing.T) {
	t.Parallel()

	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubFPFQueryFlags(t)
	defer restoreFlags()

	fpfInspectRoles = []string{"recommended_pattern"}
	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	err := runFPFInspect(command, []string{"A.7"})
	if err == nil || !strings.Contains(err.Error(), "unsupported source unit role") {
		t.Fatalf("unknown role error = %v", err)
	}
}

func stubFPFQueryFlags(t *testing.T) func() {
	t.Helper()
	originalQueryEntity := fpfQueryEntityOfConcern
	originalQueryContext := append([]string(nil), fpfQueryKnownContext...)
	originalQueryUse := fpfQueryIntendedUse
	originalQueryPerRole := fpfQueryMaxCandidatesPerRole
	originalQueryTotal := fpfQueryMaxTotalCandidates
	originalQueryExcerpt := fpfQueryMaxExcerptCharacters
	originalQueryRelations := fpfQueryMaxRelationsPerCandidate
	originalQueryJSON := fpfQueryJSON
	originalLookupRoles := append([]string(nil), fpfLookupRoles...)
	originalLookupPerRole := fpfLookupMaxCandidatesPerRole
	originalLookupTotal := fpfLookupMaxTotalCandidates
	originalLookupExcerpt := fpfLookupMaxExcerptCharacters
	originalLookupRelations := fpfLookupMaxRelationsPerCandidate
	originalLookupJSON := fpfLookupJSON
	originalInspectRoles := append([]string(nil), fpfInspectRoles...)
	originalInspectJSON := fpfInspectJSON
	originalPublicationView := fpfPublicationView
	originalReplayRef := fpfReplayRef

	fpfQueryEntityOfConcern = ""
	fpfQueryKnownContext = nil
	fpfQueryIntendedUse = ""
	fpfQueryMaxCandidatesPerRole = 0
	fpfQueryMaxTotalCandidates = 0
	fpfQueryMaxExcerptCharacters = 0
	fpfQueryMaxRelationsPerCandidate = 0
	fpfQueryJSON = false
	fpfLookupRoles = nil
	fpfLookupMaxCandidatesPerRole = 0
	fpfLookupMaxTotalCandidates = 0
	fpfLookupMaxExcerptCharacters = 0
	fpfLookupMaxRelationsPerCandidate = 0
	fpfLookupJSON = false
	fpfInspectRoles = nil
	fpfInspectJSON = false
	fpfPublicationView = "working"
	fpfReplayRef = ""

	return func() {
		fpfQueryEntityOfConcern = originalQueryEntity
		fpfQueryKnownContext = originalQueryContext
		fpfQueryIntendedUse = originalQueryUse
		fpfQueryMaxCandidatesPerRole = originalQueryPerRole
		fpfQueryMaxTotalCandidates = originalQueryTotal
		fpfQueryMaxExcerptCharacters = originalQueryExcerpt
		fpfQueryMaxRelationsPerCandidate = originalQueryRelations
		fpfQueryJSON = originalQueryJSON
		fpfLookupRoles = originalLookupRoles
		fpfLookupMaxCandidatesPerRole = originalLookupPerRole
		fpfLookupMaxTotalCandidates = originalLookupTotal
		fpfLookupMaxExcerptCharacters = originalLookupExcerpt
		fpfLookupMaxRelationsPerCandidate = originalLookupRelations
		fpfLookupJSON = originalLookupJSON
		fpfInspectRoles = originalInspectRoles
		fpfInspectJSON = originalInspectJSON
		fpfPublicationView = originalPublicationView
		fpfReplayRef = originalReplayRef
	}
}
