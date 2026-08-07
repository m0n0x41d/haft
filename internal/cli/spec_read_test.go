package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestSpecRuntimeReadPathsDoNotBypassSQLEditionSource(t *testing.T) {
	t.Parallel()

	root := testProjectRoot(t)
	cliDir := filepath.Join(root, "internal", "cli")

	allowed := map[string]map[string]string{
		"internal/cli/spec_read.go": {
			"project.LoadProjectSpecificationSet(": "SQL-first helper may read carriers only for compatibility fallback and term-map support",
		},
		"internal/cli/spec_sync.go": {
			"project.LoadProjectSpecificationSet(": "spec sync is the explicit carrier import path into SQL editions",
		},
		"internal/cli/spec_validate.go": {
			"project.LoadProjectSpecificationSet(": "spec validate intentionally reads authored draft and active carriers before profile applicability or lifecycle admission; it is read-only and keeps lifecycle observations separate",
		},
		"internal/cli/spec_classify_change.go": {
			"project.SpecSectionsFromDocuments(": "spec classify-change parses explicit before/after carrier files as read-only review input",
		},
		"internal/cli/overseer.go": {
			"project.SpecSectionsFromDocuments(": "overseer parses the changed carrier file as review-scoping input, not as the runtime spec source of truth",
		},
	}
	tokens := []string{
		"project.LoadProjectSpecificationSet(",
		"project.ProjectSpecificationSetFromDocuments(",
		"project.SpecSectionsFromDocuments(",
	}

	entries, err := os.ReadDir(cliDir)
	if err != nil {
		t.Fatalf("read %s: %v", cliDir, err)
	}
	seenAllowed := map[string]map[string]bool{}
	for path, pathAllowed := range allowed {
		seenAllowed[path] = map[string]bool{}
		for token := range pathAllowed {
			seenAllowed[path][token] = false
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		absPath := filepath.Join(cliDir, entry.Name())
		relPath := filepath.ToSlash(filepath.Join("internal", "cli", entry.Name()))
		content := readSpecRuntimeReadPathSource(t, absPath)
		for _, token := range tokens {
			if !strings.Contains(content, token) {
				continue
			}

			reason, ok := allowed[relPath][token]
			if !ok {
				t.Fatalf("%s bypasses SQL-first SpecSection reads via %q; use loadProjectSpecificationSetSQLFirst or add an explicit carrier-only exception", relPath, token)
			}
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("%s allowlist for %q must explain the carrier boundary", relPath, token)
			}
			seenAllowed[relPath][token] = true
		}
	}

	for relPath, tokenSeen := range seenAllowed {
		for token, seen := range tokenSeen {
			if !seen {
				t.Fatalf("stale SQL-first read-path allowlist: %s no longer contains %q", relPath, token)
			}
		}
	}
}

func TestLoadProjectSpecificationSetFromSQLEditionsPropagatesStoreOpenFailure(t *testing.T) {
	root := setupSpecSyncProject(t)
	homeFile := filepath.Join(t.TempDir(), "home-file")
	if err := os.WriteFile(homeFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write home file: %v", err)
	}
	t.Setenv("HOME", homeFile)

	_, _, err := loadProjectSpecificationSetFromSQLEditions(root)
	if err == nil {
		t.Fatal("expected SQL edition store open failure")
	}
	for _, want := range []string{
		"haft project database is not ready",
		"haft project migrate",
		"no migration was attempted",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want %q", err, want)
		}
	}
}

func TestLoadProjectSpecificationSetSQLFirstForNonSoftwareScopeFiltersSQLSoftware(
	t *testing.T,
) {
	root := setupSpecSyncProject(t)
	specDir := filepath.Join(root, ".haft", "specs")
	writeSpecCheckCLIFile(
		t,
		filepath.Join(specDir, "enabling-system.md"),
		validCLISpecSectionCarrier("ES.legacy.001", "enabling.role"),
	)

	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	software := cliScopedSpecSection(
		"SS.sql.001",
		project.SpecDocumentKindSoftwareSystem,
		"software.role",
	)
	legacy := cliScopedSpecSection(
		"ES.sql.001",
		project.SpecDocumentKindEnablingSystem,
		"enabling.role",
	)
	if err := store.PutCurrent(specflow.NewSpecSectionEdition(
		"qnt_5eec5eec",
		software,
		specflow.SpecSectionSourceSQL,
		time.Now().UTC(),
	)); err != nil {
		t.Fatalf("seed software edition: %v", err)
	}
	if err := store.PutCurrent(specflow.NewSpecSectionEdition(
		"qnt_5eec5eec",
		legacy,
		specflow.SpecSectionSourceSQL,
		time.Now().UTC(),
	)); err != nil {
		t.Fatalf("seed legacy edition: %v", err)
	}
	applicability := mustCLISpecificationApplicability(
		t,
		false,
		"documents",
	)

	specSet, err := loadProjectSpecificationSetSQLFirstForScope(
		root,
		applicability,
	)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirstForScope: %v", err)
	}
	if len(specSet.Sections) != 0 {
		t.Fatalf("non-software SQL sections = %#v, want none", specSet.Sections)
	}
	if containsSpecFindingCode(
		specSet.Findings,
		project.SpecMigrationRequiredFindingCode,
	) {
		t.Fatalf("non-software SQL set retained migration pressure: %#v", specSet.Findings)
	}
	if containsSpecFindingCode(
		specSet.Findings,
		"profile_capability_applicability_underdetermined",
	) {
		t.Fatalf("non-software SQL set retained resolved target-relation uncertainty: %#v", specSet.Findings)
	}
	if len(specSet.TermMapEntries) != 1 {
		t.Fatalf("term-map entries = %#v, want carrier support", specSet.TermMapEntries)
	}
}

func TestLoadProjectSpecificationSetSQLFirstForSoftwareScopeKeepsMigration(
	t *testing.T,
) {
	root := setupSpecSyncProject(t)
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.Remove(filepath.Join(specDir, "software-system.md")); err != nil {
		t.Fatal(err)
	}
	writeSpecCheckCLIFile(
		t,
		filepath.Join(specDir, "enabling-system.md"),
		validCLISpecSectionCarrier("ES.legacy.001", "enabling.role"),
	)
	applicability := mustCLISpecificationApplicability(
		t,
		true,
		"software",
	)

	specSet, err := loadProjectSpecificationSetSQLFirstForScope(
		root,
		applicability,
	)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirstForScope: %v", err)
	}
	if !containsSpecFindingCode(
		specSet.Findings,
		project.SpecMigrationRequiredFindingCode,
	) {
		t.Fatalf("software SQL set omitted migration pressure: %#v", specSet.Findings)
	}
}

func cliScopedSpecSection(
	id string,
	documentKind project.SpecDocumentKind,
	sectionKind string,
) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          string(documentKind),
		Kind:          sectionKind,
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  string(documentKind),
		Path: filepath.ToSlash(
			filepath.Join(".haft", "specs", string(documentKind)+".md"),
		),
	}
}

func mustCLISpecificationApplicability(
	t *testing.T,
	software bool,
	rawScopeID string,
) project.ProjectSpecificationSetApplicability {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	var scope projectprofile.RealizationScope
	if software {
		scope, err = projectprofile.NewSoftwareRealization(
			scopeID,
			projectprofile.NoEntityReference{},
		)
	}
	if !software {
		scope, err = projectprofile.NewNonSoftwareRealization(
			scopeID,
			projectprofile.NoEntityReference{},
			projectprofile.UnspecifiedKindOrientation{},
			nil,
			nil,
		)
	}
	if err != nil {
		t.Fatalf("construct realization scope: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{scope},
	)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	applicability, err := project.DeriveProjectSpecificationSetApplicability(
		matrix,
		scopeID,
	)
	if err != nil {
		t.Fatalf("DeriveProjectSpecificationSetApplicability: %v", err)
	}
	return applicability
}

func readSpecRuntimeReadPathSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
