package code

import (
	"path"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func (index Index) Resolve(selector string) Resolution {
	r := Resolution{Kind: "unresolved", Selector: selector, Basis: index.Basis, Complete: index.Complete}
	if err := carrier.ValidateSelector(selector); err != nil {
		r.Kind = "unsupported"
		r.Complete = false
		r.Diagnostics = append(r.Diagnostics, diagnostic("invalid_or_unsupported_selector", selector, err.Error()))
		return r
	}
	kind, value, _ := strings.Cut(selector, ":")
	p := value
	name := ""
	if kind == "sym" {
		p, name, _ = strings.Cut(value, "::")
		if path.Ext(p) != ".go" {
			r.Kind = "unsupported"
			r.Complete = false
			r.Diagnostics = append(r.Diagnostics, diagnostic("unsupported_language", p, "B1 declaration resolution supports Go only"))
			return r
		}
	}
	for _, file := range index.Files {
		if file.Path != p && !(kind == "dir" && (p == "." || strings.HasPrefix(file.Path, p+"/"))) {
			continue
		}
		if file.Status == "excluded" || file.Status == "unsupported" || file.Status == "unresolved" {
			r.Diagnostics = append(r.Diagnostics, diagnostic(file.Status, file.Path, file.Reason))
			if file.Path == p && file.Status == "unsupported" && kind != "dir" {
				r.Kind = "unsupported"
				r.Complete = false
				return r
			}
			continue
		}
		r.Files = append(r.Files, file.Path)
	}
	if kind != "sym" {
		if len(r.Files) > 0 {
			r.Kind = "exact"
		}
		return r
	}
	for _, symbol := range index.Symbols {
		if symbol.Path == p && (symbol.QualifiedName == name || symbol.Name == name) {
			r.Candidates = append(r.Candidates, symbol)
		}
	}
	switch len(r.Candidates) {
	case 0:
		r.Kind = "unresolved"
	case 1:
		r.Kind = "exact"
	default:
		r.Kind = "ambiguous"
	}
	return r
}

// ResolveCheck maps only the supported Go test/property adapters to their
// declaration anchor. The authored selector remains distinct from the locator.
func (index Index) ResolveCheck(selector string) Resolution {
	kind, value, _ := strings.Cut(selector, ":")
	if kind != "test" && kind != "pbt" {
		return Resolution{Kind: "unsupported", Selector: selector, Basis: index.Basis, Complete: false, Diagnostics: []Diagnostic{diagnostic("unsupported_check_adapter", selector, "B1 resolves Go test: and pbt: check declarations only")}}
	}
	p, name, hasSymbol := strings.Cut(value, "::")
	if !hasSymbol {
		return Resolution{Kind: "unresolved", Selector: selector, Basis: index.Basis, Complete: false, Diagnostics: []Diagnostic{diagnostic("exact_check_symbol_required", selector, "Go check requires a path::test declaration")}}
	}
	base, _, subtest := strings.Cut(name, "/")
	codeSelector := "sym:" + p + "::" + base
	resolved := index.Resolve(codeSelector)
	resolved.Selector = selector
	resolved.CodeSelector = codeSelector
	if subtest {
		resolved.Complete = false
		resolved.Diagnostics = append(resolved.Diagnostics, diagnostic("subtest_requires_runner_resolution", selector, "Code anchor identifies the harness; exact dynamic case selection belongs to the runner observation"))
	}
	return resolved
}

// Related uses a conservative Go package closure. Same-package helpers and
// reverse importers participate even if an individual function body is unchanged.
func (index Index) Related(selector string) RelatedResult {
	r := RelatedResult{Resolution: index.Resolve(selector), Basis: index.Basis, Complete: index.Complete, DependencyFiles: []string{}, AffectedFiles: []string{}, ExternalDependencies: []string{}, Limits: []string{"Go package-level closure; dynamic call dispatch and semantic oracle fitness are not inferred"}}
	if r.Resolution.Kind != "exact" {
		r.Complete = false
		return r
	}
	starts := map[string]bool{}
	packages := map[string]Package{}
	for _, pkg := range index.Packages {
		packages[pkg.ID] = pkg
		for _, selected := range r.Resolution.Files {
			base := path.Base(selected)
			if base == "go.mod" || base == "go.sum" || base == "go.work" || base == ".gitignore" || base == ".haftignore" {
				directory := path.Dir(selected)
				if directory == "." || pkg.Directory == directory || strings.HasPrefix(pkg.Directory, directory+"/") {
					starts[pkg.ID] = true
				}
			}
		}
		for _, f := range pkg.Files {
			for _, selected := range r.Resolution.Files {
				if f == selected {
					starts[pkg.ID] = true
				}
			}
		}
	}
	walk := func(reverse bool) []string {
		seen := map[string]bool{}
		queue := []string{}
		for id := range starts {
			queue = append(queue, id)
		}
		files := []string{}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			pkg := packages[id]
			files = append(files, pkg.Files...)
			if reverse {
				queue = append(queue, pkg.ReverseImports...)
			} else {
				queue = append(queue, pkg.LocalDependencies...)
				r.ExternalDependencies = append(r.ExternalDependencies, pkg.ExternalDependencies...)
			}
		}
		return sortedUnique(files)
	}
	r.DependencyFiles = walk(false)
	r.AffectedFiles = walk(true)
	r.ExternalDependencies = sortedUnique(r.ExternalDependencies)
	for _, file := range index.Files {
		if file.Status != "excluded" || file.Reason != "ignore_rule" || path.Ext(file.Path) != ".go" {
			continue
		}
		for _, included := range r.DependencyFiles {
			if path.Dir(included) == path.Dir(file.Path) {
				r.Complete = false
				r.Limits = append(r.Limits, "Ignored package sibling: "+file.Path)
				break
			}
		}
	}
	if len(r.ExternalDependencies) > 0 {
		r.Complete = false
		r.Limits = append(r.Limits, "External imports require the pinned toolchain/module/runner dependency basis; their implementation bytes are outside this index")
	}
	for _, d := range index.Diagnostics {
		if strings.Contains(d.Code, "partial") || d.Code == "module_basis_absent" {
			r.Complete = false
			r.Limits = append(r.Limits, d.Code+": "+d.Path)
		}
	}
	return r
}

// Bindings projects only authored claim-local links. Document validity and exact
// editions remain visible; invalid carriers do not silently create traceability.
func (index Index) Bindings(documents []carrier.Document) []Binding {
	result := []Binding{}
	for _, document := range documents {
		if !document.Valid() {
			continue
		}
		edition := document.Edition
		if edition == "" {
			_, _, edition, _ = carrier.NewSnapshot(document.Raw, carrier.InterpretationBasis{})
		}
		for _, claim := range document.Record.Claims {
			ref := document.Record.ID + "@" + edition + "#" + claim.ID
			for _, kind := range []string{"implemented_by", "checks"} {
				links := claim.ImplementedBy
				if kind == "checks" {
					links = claim.Checks
				}
				for _, link := range links {
					resolution := index.Resolve(link.Ref)
					if kind == "checks" {
						resolution = index.ResolveCheck(link.Ref)
					}
					result = append(result, Binding{Claim: ref, Kind: kind, Ref: link.Ref, Covers: link.Covers, Conditions: link.Conditions, Resolution: resolution})
				}
			}
		}
	}
	sort.Slice(result, func(a, b int) bool {
		x, y := result[a], result[b]
		return x.Claim+"\x00"+x.Kind+"\x00"+x.Ref < y.Claim+"\x00"+y.Kind+"\x00"+y.Ref
	})
	return result
}
func (index Index) BindingsFor(selector string, documents []carrier.Document) []Binding {
	affected := index.Related(selector)
	paths := map[string]bool{}
	for _, p := range affected.AffectedFiles {
		paths[p] = true
	}
	result := []Binding{}
	for _, binding := range index.Bindings(documents) {
		if binding.Ref == selector || binding.Resolution.CodeSelector == selector {
			binding.MatchKind = "authored_exact_selector"
			result = append(result, binding)
			continue
		}
		for _, p := range binding.Resolution.Files {
			if paths[p] {
				binding.MatchKind = "package_dependency"
				result = append(result, binding)
				break
			}
		}
	}
	return result
}

func Compare(before, after Index, selector string) Change {
	c := Change{Selector: selector, Before: before.Resolve(selector), After: after.Resolve(selector), EvidenceBasisChanged: before.Basis != after.Basis, Affected: after.Related(selector)}
	if c.Before.Kind != "exact" || c.After.Kind != "exact" {
		c.Kind = c.After.Kind
		return c
	}
	if before.Basis == after.Basis {
		c.Kind = "unchanged"
		return c
	}
	if len(c.Before.Candidates) == 1 && len(c.After.Candidates) == 1 {
		x, y := c.Before.Candidates[0], c.After.Candidates[0]
		if x.Anchor != y.Anchor || x.SyntaxDigest != y.SyntaxDigest {
			c.Kind = "implementation_changed"
			return c
		}
	}
	old := map[string]File{}
	now := map[string]File{}
	for _, f := range before.Files {
		old[f.Path] = f
	}
	for _, f := range after.Files {
		now[f.Path] = f
	}
	changed := false
	navigation := true
	for _, p := range sortedUnique(append(before.Related(selector).DependencyFiles, c.Affected.DependencyFiles...)) {
		a, aok := old[p]
		b, bok := now[p]
		if !aok || !bok || a.Digest != b.Digest {
			changed = true
			if !aok || !bok || a.SyntaxDigest == "" || a.SyntaxDigest != b.SyntaxDigest {
				navigation = false
			}
		}
	}
	// Ignore/module/build metadata are semantic basis, never presentation.
	for p, a := range old {
		if path.Ext(p) != ".go" {
			b, ok := now[p]
			if !ok || a.Digest != b.Digest {
				navigation = false
				changed = true
			}
		}
	}
	for p := range now {
		if _, ok := old[p]; !ok && path.Ext(p) != ".go" {
			navigation = false
			changed = true
		}
	}
	if !changed {
		c.Kind = "basis_changed"
	} else if navigation {
		c.Kind = "navigation_only"
	} else {
		c.Kind = "dependency_or_configuration_changed"
	}
	return c
}
