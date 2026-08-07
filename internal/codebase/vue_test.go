package codebase

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVueScriptSetupAndTemplateUse(t *testing.T) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(root, "helper.ts"), []byte(`export function helper(value: string): string { return value }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	vue := `<template>
  <button @click="submit">{{ label }}</button>
</template>
<script setup lang="ts">
import { helper } from "./helper"
const label = "ready"
function submit(): string {
  return helper(label)
}
</script>
`
	if err := os.WriteFile(filepath.Join(root, "Example.vue"), []byte(vue), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, relPath := range []string{"helper.ts", "Example.vue"} {
		if err := store.IndexFileSymbols(ctx, root, relPath); err != nil {
			t.Fatal(err)
		}
	}
	status := InspectVueParse(root, "Example.vue")
	if status.Status != VueParseIndexed || status.ScriptBlocks != 1 || !status.HasTemplate {
		t.Fatalf("Vue parse status = %+v", status)
	}
	edges, err := (&VueLang{}).ResolveFileEdges(ctx, root, "Example.vue", store)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCallEdge(t, ctx, store, edges, "submit", "helper") {
		t.Fatalf("Vue script call missing: %+v", edges)
	}
	templateUse := false
	for _, edge := range edges {
		if edge.Kind != EdgeTemplateUse {
			continue
		}
		if edgeName(t, ctx, store, edge.SrcID) == "__template__" && edgeName(t, ctx, store, edge.DstID) == "submit" {
			templateUse = true
		}
	}
	if !templateUse {
		t.Fatalf("Vue template-use edge missing: %+v", edges)
	}
}

func TestVueParseStatusDegradesAndDistinguishesEmpty(t *testing.T) {
	root := t.TempDir()
	if status := InspectVueParse(root, "missing.vue"); status.Status != VueParseDegraded {
		t.Fatalf("missing Vue status = %+v", status)
	}
	if err := os.WriteFile(filepath.Join(root, "Empty.vue"), []byte("<!-- empty -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := InspectVueParse(root, "Empty.vue"); status.Status != VueParseEmpty {
		t.Fatalf("empty Vue status = %+v", status)
	}
}

func TestVueTargetParseStatus(t *testing.T) {
	root := os.Getenv("HAFT_VUE_TARGET_ROOT")
	if root == "" {
		t.Skip("set HAFT_VUE_TARGET_ROOT to run the optional real-project Vue corpus")
	}
	counts := map[string]int{}
	total := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".vue" {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		status := InspectVueParse(root, relPath)
		counts[status.Status]++
		total++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total == 0 {
		t.Fatal("Vue target contains no .vue files")
	}
	if counts[VueParseIndexed]+counts[VueParseEmpty]+counts[VueParseDegraded] != total {
		t.Fatalf("Vue parse statuses are incomplete: total=%d counts=%v", total, counts)
	}
	t.Logf("Vue target parse statuses: total=%d counts=%v", total, counts)
}
