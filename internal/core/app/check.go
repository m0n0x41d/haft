package app

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/check"
	"github.com/m0n0x41d/haft/internal/core/code"
	"github.com/m0n0x41d/haft/internal/core/store"
)

// CheckBasisCapture exposes the exact preimages already used by prepare. It
// enables an independent client check without reconstructing hidden conventions.
type CheckBasisCapture struct {
	Format                 string             `json:"format"`
	HashAlgorithm          string             `json:"hash_algorithm"`
	ImplementationFiles    map[string]string  `json:"implementation_files"`
	ImplementationPreimage []byte             `json:"implementation_preimage_base64"`
	Oracle                 CheckOracleCapture `json:"oracle"`
	DependencyFiles        map[string]string  `json:"dependency_files"`
	DependencyConfig       code.Config        `json:"dependency_config"`
	DependencyPreimage     []byte             `json:"dependency_preimage_base64"`
}
type CheckOracleCapture struct {
	Path      string `json:"path"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	RawDigest string `json:"raw_digest"`
}

func (s Service) check(q Request, r Result, v store.View) Result {
	if q.Action == "" || q.Action == "structural" {
		docs := v.Documents
		if q.Carrier != "" {
			docs = []carrier.Document{carrier.Parse([]byte(q.Carrier))}
		}
		if q.Ref != "" {
			res := v.Resolve(q.Ref)
			if res.Kind != "found" || res.Document == nil {
				return conflict(r, "target_unresolved", res.Kind)
			}
			docs = []carrier.Document{*res.Document}
		}
		ds := []carrier.Diagnostic{}
		for _, doc := range docs {
			ds = append(ds, doc.Diagnostics...)
			ds = append(ds, carrier.Validate(doc.Record, q.Strict)...)
			ds = append(ds, v.Projection.ValidateReferences(doc.Record)...)
		}
		r.Kind = "structurally_valid"
		if carrier.HasErrors(ds) {
			r.Kind = "invalid"
		}
		r.Diagnostics = append(r.Diagnostics, ds...)
		r.Data = map[string]any{"records": len(docs), "strict": q.Strict, "checks_executed": false}
		r.Limits = append(r.Limits, "Structural checking neither runs an oracle nor establishes implementation correctness")
		return r
	}
	if q.Action == "observe" {
		if q.Observation == nil {
			return failure(r, "observation_required", "Provide expected contract and captured actual runner observation")
		}
		observation := check.GoTestObservation(*q.Observation)
		r.Kind = observation.Status
		probe := q
		probe.Ref = q.Observation.Expected.Basis.Claim
		probe.CheckRef = q.Observation.Expected.Ref
		probe.Scope = q.Observation.Expected.Scope
		probe.FailureContract = q.Observation.Expected.FailureContract
		probe.FailurePattern = q.Observation.Expected.FailurePattern
		probe.Seed = q.Observation.Expected.Basis.Seed
		contract, _, _, _, err := s.prepareCheck(probe, v)
		currentness := "unknown"
		if err == nil {
			currentness = "changed"
			if sameJSON(contract.Basis, q.Observation.Expected.Basis) {
				currentness = "same"
			}
		}
		r.Data = map[string]any{"observation": observation, "current_basis": currentness, "declared_check_binding": err == nil}
		if err != nil {
			r.Diagnostics = append(r.Diagnostics, carrier.Diagnostic{Code: "binding_unresolved", Message: err.Error(), Severity: "warning"})
		}
		r.Limits = append(r.Limits, observation.Limits...)
		return r
	}
	if q.Action != "prepare" {
		return failure(r, "unsupported_action", "Check action is structural, prepare or observe")
	}
	contract, command, index, captured, err := s.prepareCheck(q, v)
	if err != nil {
		return conflict(r, "check_basis_unresolved", err.Error())
	}
	r.Kind = "prepared"
	r.Basis["code_basis"] = index.Basis
	r.Basis["claim_ref"] = contract.Basis.Claim
	if !index.Complete {
		r.Coverage = "degraded"
	}
	r.Diagnostics = append(r.Diagnostics, index.Diagnostics...)
	cgo := "0"
	if index.Config.CGOEnabled {
		cgo = "1"
	}
	r.Data = map[string]any{"expected": contract, "command": command, "code_complete": index.Complete, "runner_started": false, "basis_capture": captured,
		"run_environment":        map[string]string{"GOOS": index.Config.GOOS, "GOARCH": index.Config.GOARCH, "CGO_ENABLED": cgo, "GOTOOLCHAIN": "local", "GOWORK": "off", "GOFLAGS": ""},
		"toolchain_verification": map[string]any{"command": []string{"go", "version"}, "expected": index.Config.Toolchain}}
	r.Limits = append(r.Limits, "Caller reviews whether the declared oracle covers the claim, runs the exact command, and supplies captured output to observe")
	r.Limits = append(r.Limits, "External imports rely on the declared toolchain and captured module manifests; external implementation bytes and arbitrary runtime inputs are outside local inspection")
	return r
}
func checkSymbol(ref string) (string, error) {
	kind, value, ok := strings.Cut(ref, ":")
	if !ok {
		return "", fmt.Errorf("invalid check selector")
	}
	switch kind {
	case "test", "pbt":
		return "sym:" + value, nil
	case "sym":
		return ref, nil
	default:
		return "", fmt.Errorf("unsupported executable check selector %s", kind)
	}
}
func (s Service) prepareCheck(q Request, v store.View) (check.Contract, []string, code.Index, CheckBasisCapture, error) {
	var contract check.Contract
	var captured CheckBasisCapture
	found := v.Projection.Resolve(q.Ref)
	if found.Kind != "found" || found.Document == nil || found.Claim == nil {
		return contract, nil, code.Index{}, captured, fmt.Errorf("one resolvable spec claim is required: %s", found.Kind)
	}
	if q.Scope == "" {
		return contract, nil, code.Index{}, captured, fmt.Errorf("explicit check scope is required")
	}
	symbol, err := checkSymbol(q.CheckRef)
	if err != nil {
		return contract, nil, code.Index{}, captured, err
	}
	var binding *carrier.Binding
	for _, b := range found.Claim.Checks {
		normalized, e := checkSymbol(b.Ref)
		if e == nil && normalized == symbol {
			copy := b
			binding = &copy
			break
		}
	}
	if binding == nil {
		return contract, nil, code.Index{}, captured, fmt.Errorf("check is not declared for this exact claim")
	}
	index, _, err := s.codeIndex(q)
	if err != nil {
		return contract, nil, index, captured, err
	}
	if !index.Complete || len(index.Config.ToolTags) > 0 {
		return contract, nil, index, captured, fmt.Errorf("complete Go capture without custom compiler tool tags is required for this runner adapter")
	}
	for _, d := range index.Diagnostics {
		if strings.Contains(d.Code, "partial") || d.Code == "module_basis_absent" || d.Code == "unresolved_local_import" {
			return contract, nil, index, captured, fmt.Errorf("runner local dependency basis incomplete: %s", d.Code)
		}
	}
	resolved := index.Resolve(symbol)
	if resolved.Kind != "exact" || len(resolved.Candidates) != 1 {
		return contract, nil, index, captured, fmt.Errorf("oracle symbol %s", resolved.Kind)
	}
	oracle := resolved.Candidates[0]
	if !strings.HasSuffix(oracle.Path, "_test.go") || !strings.HasPrefix(oracle.Name, "Test") {
		return contract, nil, index, captured, fmt.Errorf("selected declaration is not a Go test")
	}
	implementations := map[string]string{}
	dependencyFiles := map[string]bool{}
	externalImports := map[string]bool{}
	selectors := []string{symbol}
	for _, impl := range found.Claim.ImplementedBy {
		selectors = append(selectors, impl.Ref)
		res := index.Resolve(impl.Ref)
		if res.Kind != "exact" {
			return contract, nil, index, captured, fmt.Errorf("implementation %s: %s", impl.Ref, res.Kind)
		}
		for _, p := range res.Files {
			for _, f := range index.Files {
				if f.Path == p {
					implementations[p] = f.Digest
				}
			}
		}
	}
	if len(implementations) == 0 {
		return contract, nil, index, captured, fmt.Errorf("claim has no resolved implementation basis")
	}
	for _, selector := range selectors {
		related := index.Related(selector)
		for _, imported := range related.ExternalDependencies {
			externalImports[imported] = true
		}
		for _, p := range related.DependencyFiles {
			dependencyFiles[p] = true
		}
	}
	dependencies := map[string]string{}
	for _, file := range index.Files {
		if dependencyFiles[file.Path] {
			if strings.Contains(string(file.Raw), "//go:embed ") {
				return contract, nil, index, captured, fmt.Errorf("embedded asset basis requires a separate adapter declaration")
			}
			dependencies[file.Path] = file.Digest
		}
		base := path.Base(file.Path)
		if base == "go.mod" || base == "go.sum" || base == "go.work" || base == "go.work.sum" || base == ".gitignore" || base == ".haftignore" {
			dependencies[file.Path] = file.Digest
		}
	}
	dependencyBytes, _ := json.Marshal(struct {
		Config code.Config       `json:"config"`
		Files  map[string]string `json:"files"`
	}{index.Config, dependencies})
	external := []string{}
	for imported := range externalImports {
		external = append(external, imported)
	}
	sort.Strings(external)
	encoded, _ := json.Marshal(implementations)
	pkg := ""
	for _, p := range index.Packages {
		for _, f := range p.Files {
			if f == oracle.Path {
				pkg = p.ImportPath
			}
		}
	}
	if pkg == "" {
		return contract, nil, index, captured, fmt.Errorf("test package identity unresolved")
	}
	conditions := []string{binding.Covers}
	if binding.Conditions != "" {
		conditions = append(conditions, binding.Conditions)
	}
	contract = check.Contract{Ref: binding.Ref, Selector: check.Selector{Package: pkg, Test: oracle.Name}, Scope: q.Scope, FailureContract: q.FailureContract, FailurePattern: q.FailurePattern,
		Basis: check.Basis{Claim: found.Document.Record.ID + "@" + found.Document.Edition + "#" + found.Claim.ID, Code: carrier.Digest(encoded), Check: oracle.RawDigest, Dependencies: carrier.Digest(dependencyBytes), Conditions: conditions, Seed: q.Seed,
			Environment: map[string]string{"build_tags": strings.Join(index.Config.BuildTags, ","), "goos": index.Config.GOOS, "goarch": index.Config.GOARCH, "toolchain": index.Config.Toolchain, "cgo_enabled": fmt.Sprint(index.Config.CGOEnabled), "external_imports": strings.Join(external, ","), "external_basis": "declared toolchain and captured module manifests; transitive bytes not inspected"}}}
	command, err := check.GoTestCommand(contract.Selector, index.Config.BuildTags)
	if err != nil {
		return contract, nil, index, captured, err
	}
	captured = CheckBasisCapture{Format: "haft.check-basis-capture/1",
		HashAlgorithm:       "sha256:<lowercase hex> of exact bytes. File digests hash entire raw file bytes; oracle raw_digest hashes file[start_byte:end_byte] (zero-based, end-exclusive). Code and Dependencies hash the exact UTF-8 JSON preimage bytes supplied in the base64 fields, without adding a newline or reserializing.",
		ImplementationFiles: implementations, ImplementationPreimage: encoded,
		Oracle:          CheckOracleCapture{Path: oracle.Path, StartByte: oracle.StartByte, EndByte: oracle.EndByte, RawDigest: oracle.RawDigest},
		DependencyFiles: dependencies, DependencyConfig: index.Config, DependencyPreimage: dependencyBytes}
	return contract, command, index, captured, nil
}
