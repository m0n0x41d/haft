package codebase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTSSymbolAdapter_ExtractsIdiomaticDeclarationsCanonically(t *testing.T) {
	root := t.TempDir()
	rel := "app.ts"
	source := `export type ApplicationCaseState = {
  readonly status: string
  advance(next: string): void
}

export enum Phase {
  Draft,
  Active = "active",
}

export const createApiApp = (port: number) => ({ port })
export const wrapped = memo(() => 42)
export const workflowStateLabel: Record<string, string> = { active: "Active" }

export const apiClient = {
  async me(): Promise<string> { return "me" },
  signOut: async () => "ok",
}

export abstract class Base {
  public run = () => createApiApp(1)
  count = 0
  private secret() { return 0 }
  load() { return this.run() }
}

const localExpression = function () { return 1 }
function classic() { return localExpression() }
`
	if err := os.WriteFile(filepath.Join(root, rel), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshots, err := ExtractSymbolSnapshots(root, rel)
	if err != nil {
		t.Fatal(err)
	}
	byIdentity := snapshotIdentities(snapshots)

	want := map[string]string{
		"createApiApp":                 "func",
		"wrapped":                      "func",
		"workflowStateLabel":           "constant",
		"apiClient":                    "constant",
		"apiClient.me":                 "method",
		"apiClient.signOut":            "method",
		"ApplicationCaseState":         "type_alias",
		"ApplicationCaseState.status":  "property",
		"ApplicationCaseState.advance": "method",
		"Phase":                        "enum",
		"Phase.Draft":                  "enum_member",
		"Phase.Active":                 "enum_member",
		"Base":                         "class",
		"Base.run":                     "method",
		"Base.count":                   "property",
		"Base.secret":                  "method",
		"Base.load":                    "method",
		"localExpression":              "func",
		"classic":                      "func",
	}
	for identity, kind := range want {
		snapshot, ok := byIdentity[identity]
		if !ok {
			t.Errorf("missing %s; got %v", identity, snapshotIdentityList(snapshots))
			continue
		}
		if snapshot.SymbolKind != kind {
			t.Errorf("%s kind = %q, want %q", identity, snapshot.SymbolKind, kind)
		}
		body := source[snapshot.StartByte:snapshot.EndByte]
		if !strings.Contains(body, snapshot.SymbolName) {
			t.Errorf("%s byte-exact body does not contain its name: %q", identity, body)
		}
	}

	if countSnapshots(snapshots, "createApiApp", "") != 1 {
		t.Fatalf("arrow declaration must produce one canonical node: %+v", snapshots)
	}
	if !byIdentity["createApiApp"].Exported || !byIdentity["apiClient.me"].Exported {
		t.Fatal("export ancestry must propagate to arrows and exported object members")
	}
	if byIdentity["Base.secret"].Exported {
		t.Fatal("private class method must not be marked exported")
	}
	if byIdentity["localExpression"].Exported || byIdentity["classic"].Exported {
		t.Fatal("non-exported TypeScript declarations must not be ranked as exported")
	}
}

func TestTSSymbolAdapter_TSXAndMTSShareRegistryCoverage(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"view.tsx":  `export const View = () => <main>Hello</main>`,
		"entry.mts": `export const boot = async () => 1`,
	}
	store, storeRoot := newSymbolStore(t)
	_ = storeRoot
	ctx := context.Background()
	for rel, source := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := store.IndexFileSymbols(ctx, root, rel); err != nil {
			t.Fatal(err)
		}
	}

	view, err := store.GetByName(ctx, "View")
	if err != nil || len(view) != 1 || view[0].Lang != "typescript" {
		t.Fatalf("TSX arrow not indexed with TypeScript grammar: %+v / %v", view, err)
	}
	boot, err := store.GetByName(ctx, "boot")
	if err != nil || len(boot) != 1 || boot[0].Lang != "typescript" {
		t.Fatalf("MTS arrow not indexed through registry: %+v / %v", boot, err)
	}

	scanner := NewScanner(store.db)
	fingerprint, err := scanner.SourceFingerprint(root)
	if err != nil || fingerprint == "" {
		t.Fatalf("TSX/MTS source fingerprint missing: %q / %v", fingerprint, err)
	}
	language, ok := LanguageForPath("entry.mts")
	if !ok || language != "typescript" {
		t.Fatalf("MTS binding support = %q/%v, want typescript/true", language, ok)
	}
}

func TestBuildRepoMap_UsesCanonicalTSSnapshots(t *testing.T) {
	root := t.TempDir()
	source := `export const createApiApp = () => 1
const internalHelper = () => 2
`
	if err := os.WriteFile(filepath.Join(root, "app.ts"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	repoMap, err := BuildRepoMap(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(repoMap.Files) != 1 {
		t.Fatalf("expected one TS file, got %+v", repoMap.Files)
	}
	byName := map[string]Symbol{}
	for _, symbol := range repoMap.Files[0].Symbols {
		byName[symbol.Name] = symbol
	}
	if byName["createApiApp"].Kind != "func" || !byName["createApiApp"].Exported {
		t.Fatalf("repo map lost canonical exported arrow: %+v", byName)
	}
	if byName["internalHelper"].Kind != "func" || byName["internalHelper"].Exported {
		t.Fatalf("repo map disagrees with canonical local arrow: %+v", byName)
	}
}

func TestBuildRepoMap_DoesNotRepresentOversizedSourceAsEmpty(t *testing.T) {
	root := t.TempDir()
	content := strings.Repeat("x", int(defaultMaxFileBytes)+1)
	if err := os.WriteFile(
		filepath.Join(root, "oversized.ts"),
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	repoMap, err := BuildRepoMap(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(repoMap.Files) != 0 {
		t.Fatalf(
			"oversized source masqueraded as repository-map file: %+v",
			repoMap.Files,
		)
	}
}

func TestSupportedBindingLanguages_UsesJSTSSymbolAdapterExtensions(t *testing.T) {
	support := SupportedBindingLanguages()
	byLanguage := map[string]map[string]bool{}
	for _, item := range support {
		exts := map[string]bool{}
		for _, ext := range item.Extensions {
			exts[ext] = true
		}
		byLanguage[item.Language] = exts
	}
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
		if !byLanguage["typescript"][ext] {
			t.Errorf("TypeScript binding support missing %s: %+v", ext, byLanguage["typescript"])
		}
	}
	for _, ext := range []string{".js", ".jsx", ".mjs", ".cjs"} {
		if !byLanguage["javascript"][ext] {
			t.Errorf("JavaScript binding support missing %s: %+v", ext, byLanguage["javascript"])
		}
	}
}

func snapshotIdentities(snapshots []SymbolSnapshot) map[string]SymbolSnapshot {
	out := make(map[string]SymbolSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		identity := snapshot.SymbolName
		if snapshot.Receiver != "" {
			identity = snapshot.Receiver + "." + snapshot.SymbolName
		}
		out[identity] = snapshot
	}
	return out
}

func snapshotIdentityList(snapshots []SymbolSnapshot) []string {
	identities := snapshotIdentities(snapshots)
	out := make([]string, 0, len(identities))
	for identity := range identities {
		out = append(out, identity)
	}
	return out
}

func countSnapshots(snapshots []SymbolSnapshot, name, receiver string) int {
	count := 0
	for _, snapshot := range snapshots {
		if snapshot.SymbolName == name && snapshot.Receiver == receiver {
			count++
		}
	}
	return count
}
