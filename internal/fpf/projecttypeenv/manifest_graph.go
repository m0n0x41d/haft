// Package projecttypeenv compiles source-preserving Haft Local-Practice
// carriers into project TypeEnv artifacts. This file owns only pure source
// manifest graph resolution. It performs no lowering, persistence, staging,
// activation, filesystem access, or project-head mutation.
package projecttypeenv

import (
	"sort"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	// SupportedCompilerVersion is retained as the sealed v1 spelling used by
	// historical carrier fixtures and public compatibility tests.
	SupportedCompilerVersion = "haft.local-practice.compiler/v1"
	// CurrentCompilerVersion owns the current KindClassification declaration
	// family. A v1 carrier cannot opt into that family by changing only fields.
	CurrentCompilerVersion = "haft.local-practice.compiler/v2"
)

type ManifestCoordinate struct {
	id      string
	version string
}

func newManifestCoordinate(id, version string) ManifestCoordinate {
	return ManifestCoordinate{id: id, version: version}
}

func (coordinate ManifestCoordinate) ID() string { return coordinate.id }

func (coordinate ManifestCoordinate) Version() string { return coordinate.version }

func (coordinate ManifestCoordinate) String() string {
	return coordinate.id + "@" + coordinate.version
}

type ResolvedManifestNode struct {
	carrier    localpractice.ParsedCarrier
	coordinate ManifestCoordinate
	imports    []ManifestCoordinate
}

func (node ResolvedManifestNode) Carrier() localpractice.ParsedCarrier { return node.carrier }

func (node ResolvedManifestNode) Coordinate() ManifestCoordinate { return node.coordinate }

func (node ResolvedManifestNode) Imports() []ManifestCoordinate {
	return append([]ManifestCoordinate(nil), node.imports...)
}

type ResolvedManifestBundle struct {
	base  typeenv.BaseTypeEnvArtifact
	nodes []ResolvedManifestNode
}

func (bundle ResolvedManifestBundle) BaseArtifact() typeenv.BaseTypeEnvArtifact {
	return bundle.base
}

func (bundle ResolvedManifestBundle) Nodes() []ResolvedManifestNode {
	return cloneResolvedNodes(bundle.nodes)
}

type BundleResolution interface {
	Rejected() bool
	Issues() []LinkIssue
	Bundle() (ResolvedManifestBundle, bool)
	bundleResolutionVariant()
}

type acceptedBundleResolution struct {
	bundle ResolvedManifestBundle
}

func (acceptedBundleResolution) Rejected() bool { return false }

func (acceptedBundleResolution) Issues() []LinkIssue { return nil }

func (resolution acceptedBundleResolution) Bundle() (ResolvedManifestBundle, bool) {
	bundle := ResolvedManifestBundle{
		base:  resolution.bundle.base,
		nodes: cloneResolvedNodes(resolution.bundle.nodes),
	}
	return bundle, true
}

func (acceptedBundleResolution) bundleResolutionVariant() {}

type rejectedBundleResolution struct {
	issues []LinkIssue
}

func (rejectedBundleResolution) Rejected() bool { return true }

func (resolution rejectedBundleResolution) Issues() []LinkIssue {
	return cloneIssues(resolution.issues)
}

func (rejectedBundleResolution) Bundle() (ResolvedManifestBundle, bool) {
	return ResolvedManifestBundle{}, false
}

func (rejectedBundleResolution) bundleResolutionVariant() {}

type manifestNode struct {
	carrier    localpractice.ParsedCarrier
	coordinate ManifestCoordinate
	carrierID  string
	edition    string
	digest     string
	span       localpractice.SourceLineRange
	importIDs  []string
}

func ResolveManifestGraph(
	base typeenv.BaseTypeEnvArtifact,
	carriers []localpractice.ParsedCarrier,
) BundleResolution {
	baseRef, baseIssues := resolveBaseArtifact(base)
	if len(baseIssues) > 0 {
		return rejectedResolution(baseIssues)
	}

	nodes, carrierIssues := resolveCarrierNodes(baseRef, carriers)
	if len(carrierIssues) > 0 {
		return rejectedResolution(carrierIssues)
	}

	ordered, graphIssues := resolveCanonicalDAG(nodes)
	if len(graphIssues) > 0 {
		return rejectedResolution(graphIssues)
	}

	bundle := ResolvedManifestBundle{base: base, nodes: ordered}
	return acceptedBundleResolution{bundle: bundle}
}

func resolveBaseArtifact(
	base typeenv.BaseTypeEnvArtifact,
) (typedmemory.TypeEnvRef, []LinkIssue) {
	if err := base.Verify(); err != nil {
		issue := newLinkIssue(
			IssueBaseArtifactInvalid,
			BaseArtifactLocation{},
			"base_type_env_ref",
			err.Error(),
			"supply the exact verified P6 base TypeEnv artifact",
		)
		return typedmemory.TypeEnvRef{}, []LinkIssue{issue}
	}
	ref, exists := base.TypeEnvRef()
	if exists {
		return ref, nil
	}
	issue := newLinkIssue(
		IssueBaseNotCompiled,
		BaseArtifactLocation{},
		"base_type_env_ref",
		"the P6 artifact is coverage-only and has no executable TypeEnvRef",
		"compile a complete supported P6 base before linking Local-Practice carriers",
	)
	return typedmemory.TypeEnvRef{}, []LinkIssue{issue}
}

func resolveCarrierNodes(
	baseRef typedmemory.TypeEnvRef,
	carriers []localpractice.ParsedCarrier,
) ([]manifestNode, []LinkIssue) {
	nodes := make([]manifestNode, 0, len(carriers))
	issues := make([]LinkIssue, 0)
	for _, parsed := range carriers {
		node, nodeIssues := resolveCarrierNode(baseRef, parsed)
		nodes = append(nodes, node)
		issues = append(issues, nodeIssues...)
	}
	issues = append(issues, duplicateManifestIssues(nodes)...)
	return nodes, issues
}

func resolveCarrierNode(
	baseRef typedmemory.TypeEnvRef,
	parsed localpractice.ParsedCarrier,
) (manifestNode, []LinkIssue) {
	carrier := parsed.Carrier()
	identity := carrier.Identity()
	manifest := carrier.Manifest()
	carrierID := identity.ID().Value()
	edition := identity.Edition().Value()
	manifestID := manifest.ID().Value()
	manifestVersion := manifest.Version().Value()
	location := newCarrierSourceLocation(carrierID, carrier.Span())
	issues := make([]LinkIssue, 0)

	issues = append(issues, carrierIdentityIssues(
		location,
		carrierID,
		edition,
		manifestID,
		manifestVersion,
	)...)
	issues = append(issues, carrierBaseIssues(location, baseRef, carrier)...)
	issues = append(issues, carrierCompilerIssues(location, carrier)...)

	importIDs := make([]string, 0, len(manifest.Imports()))
	for _, item := range manifest.Imports() {
		importIDs = append(importIDs, item.SignatureID().Value())
	}
	sort.Strings(importIDs)

	node := manifestNode{
		carrier:    parsed,
		coordinate: newManifestCoordinate(manifestID, manifestVersion),
		carrierID:  carrierID,
		edition:    edition,
		digest:     parsed.Digest().String(),
		span:       carrier.Span(),
		importIDs:  importIDs,
	}
	return node, issues
}

func carrierIdentityIssues(
	location CarrierSourceLocation,
	carrierID string,
	edition string,
	manifestID string,
	manifestVersion string,
) []LinkIssue {
	issues := make([]LinkIssue, 0)
	if carrierID != manifestID {
		issues = append(issues, newLinkIssue(
			IssueCarrierManifestMismatch,
			location,
			manifestID,
			"carrier.id and signature_manifest.id differ",
			"make the carrier and manifest identify the same local-practice signature",
		))
	}
	if edition != manifestVersion {
		issues = append(issues, newLinkIssue(
			IssueCarrierManifestMismatch,
			location,
			manifestID,
			"carrier.edition and signature_manifest.version differ",
			"make carrier edition and manifest version exactly equal",
		))
	}
	if !isCanonicalSemVer(edition) || !isCanonicalSemVer(manifestVersion) {
		issues = append(issues, newLinkIssue(
			IssueCarrierManifestMismatch,
			location,
			manifestID,
			"carrier edition and manifest version must be canonical SemVer",
			"use one exact MAJOR.MINOR.PATCH SemVer value in both fields",
		))
	}
	return issues
}

func carrierBaseIssues(
	location CarrierSourceLocation,
	baseRef typedmemory.TypeEnvRef,
	carrier localpractice.Carrier,
) []LinkIssue {
	parsed, err := typedmemory.ParseTypeEnvRef(carrier.BaseTypeEnvRef().Value())
	if err == nil && parsed.String() == baseRef.String() {
		return nil
	}
	detail := "carrier base_type_env_ref does not equal the verified P6 base"
	if err != nil {
		detail = "carrier base_type_env_ref is malformed: " + err.Error()
	}
	issue := newLinkIssue(
		IssueBaseRefMismatch,
		location,
		carrier.BaseTypeEnvRef().Value(),
		detail,
		"set base_type_env_ref to "+baseRef.String()+"; do not encode it as a manifest import",
	)
	return []LinkIssue{issue}
}

func carrierCompilerIssues(
	location CarrierSourceLocation,
	carrier localpractice.Carrier,
) []LinkIssue {
	actual := carrier.CompilerVersion().Value()
	if compilerVersionSupportsDeclarationFamily(actual, carrier) {
		return nil
	}
	issue := newLinkIssue(
		IssueCompilerVersionMismatch,
		location,
		actual,
		"the Local-Practice compiler version is unsupported",
		"use "+SupportedCompilerVersion+" only for sealed EntitySet/MemberOf declarations or "+CurrentCompilerVersion+" for current KindClassification declarations",
	)
	return []LinkIssue{issue}
}

func compilerVersionSupportsDeclarationFamily(
	version string,
	carrier localpractice.Carrier,
) bool {
	if version != SupportedCompilerVersion && version != CurrentCompilerVersion {
		return false
	}
	hasHistoricalKindSurface := false
	hasCurrentKindSurface := false
	for _, declaration := range carrier.Signature().Vocabulary().Declarations() {
		switch declaration.Kind() {
		case localpractice.DeclarationEntitySet,
			localpractice.DeclarationKindSignature:
			hasHistoricalKindSurface = true
		case localpractice.DeclarationKindClassificationSignature:
			hasCurrentKindSurface = true
		}
	}
	if hasHistoricalKindSurface && hasCurrentKindSurface {
		return false
	}
	if hasHistoricalKindSurface {
		return version == SupportedCompilerVersion
	}
	if hasCurrentKindSurface {
		return version == CurrentCompilerVersion
	}
	return true
}

func duplicateManifestIssues(nodes []manifestNode) []LinkIssue {
	byCoordinate := make(map[string][]manifestNode)
	byID := make(map[string][]manifestNode)
	for _, node := range nodes {
		coordinate := node.coordinate.String()
		byCoordinate[coordinate] = append(byCoordinate[coordinate], node)
		byID[node.coordinate.ID()] = append(byID[node.coordinate.ID()], node)
	}
	issues := make([]LinkIssue, 0)
	for coordinate, matches := range byCoordinate {
		if len(matches) < 2 {
			continue
		}
		issues = append(issues, manifestMultiplicityIssues(
			IssueDuplicateManifestCoordinate,
			coordinate,
			matches,
			"retain exactly one carrier for this manifest coordinate",
		)...)
	}
	for signatureID, matches := range byID {
		if len(matches) < 2 || allSameCoordinate(matches) {
			continue
		}
		issues = append(issues, manifestMultiplicityIssues(
			IssueDuplicateSignatureID,
			signatureID,
			matches,
			"select one exact signature version before linking; imports never select latest",
		)...)
	}
	return issues
}

func manifestMultiplicityIssues(
	code LinkIssueCode,
	subject string,
	matches []manifestNode,
	repair string,
) []LinkIssue {
	issues := make([]LinkIssue, 0, len(matches))
	for _, match := range matches {
		location := newCarrierSourceLocation(match.carrierID, match.span)
		issues = append(issues, newLinkIssue(
			code,
			location,
			subject,
			"the source bundle contains more than one matching signature carrier",
			repair,
		))
	}
	return issues
}

func allSameCoordinate(nodes []manifestNode) bool {
	first := nodes[0].coordinate.String()
	for _, node := range nodes[1:] {
		if node.coordinate.String() != first {
			return false
		}
	}
	return true
}

func resolveCanonicalDAG(
	nodes []manifestNode,
) ([]ResolvedManifestNode, []LinkIssue) {
	byID := make(map[string]manifestNode, len(nodes))
	for _, node := range nodes {
		byID[node.coordinate.ID()] = node
	}
	issues := validateImports(nodes, byID)
	if len(issues) > 0 {
		return nil, issues
	}
	return topologicalNodes(nodes, byID)
}

func validateImports(
	nodes []manifestNode,
	byID map[string]manifestNode,
) []LinkIssue {
	issues := make([]LinkIssue, 0)
	for _, node := range nodes {
		location := newCarrierSourceLocation(node.carrierID, node.span)
		for _, importID := range node.importIDs {
			if importID == node.coordinate.ID() {
				issues = append(issues, newLinkIssue(
					IssueSelfImport,
					location,
					importID,
					"a SignatureManifest cannot import itself",
					"remove the self-import and declare only real predecessor signatures",
				))
				continue
			}
			if _, exists := byID[importID]; exists {
				continue
			}
			issues = append(issues, newLinkIssue(
				IssueMissingImport,
				location,
				importID,
				"no exact carrier in the bundle provides the imported SignatureID",
				"supply the exact imported carrier; do not fabricate a base fpf.core manifest",
			))
		}
	}
	return issues
}

func topologicalNodes(
	nodes []manifestNode,
	byID map[string]manifestNode,
) ([]ResolvedManifestNode, []LinkIssue) {
	indegree := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		id := node.coordinate.ID()
		indegree[id] = len(node.importIDs)
		for _, importID := range node.importIDs {
			dependents[importID] = append(dependents[importID], id)
		}
	}
	for id := range dependents {
		sort.Slice(dependents[id], func(left, right int) bool {
			return nodeStructuralKey(byID[dependents[id][left]]) < nodeStructuralKey(byID[dependents[id][right]])
		})
	}

	ready := readyNodeIDs(nodes, indegree)
	ordered := make([]ResolvedManifestNode, 0, len(nodes))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		node := byID[id]
		ordered = append(ordered, resolvedNode(node, byID))
		for _, dependent := range dependents[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = insertReadyNode(ready, dependent, byID)
			}
		}
	}
	if len(ordered) == len(nodes) {
		return ordered, nil
	}
	return nil, importCycleIssues(nodes, indegree, byID)
}

func readyNodeIDs(nodes []manifestNode, indegree map[string]int) []string {
	ready := make([]string, 0)
	byID := make(map[string]manifestNode, len(nodes))
	for _, node := range nodes {
		byID[node.coordinate.ID()] = node
		if indegree[node.coordinate.ID()] == 0 {
			ready = append(ready, node.coordinate.ID())
		}
	}
	sort.Slice(ready, func(left, right int) bool {
		return nodeStructuralKey(byID[ready[left]]) < nodeStructuralKey(byID[ready[right]])
	})
	return ready
}

func insertReadyNode(
	ready []string,
	id string,
	byID map[string]manifestNode,
) []string {
	ready = append(ready, id)
	sort.Slice(ready, func(left, right int) bool {
		return nodeStructuralKey(byID[ready[left]]) < nodeStructuralKey(byID[ready[right]])
	})
	return ready
}

func resolvedNode(
	node manifestNode,
	byID map[string]manifestNode,
) ResolvedManifestNode {
	imports := make([]ManifestCoordinate, 0, len(node.importIDs))
	for _, importID := range node.importIDs {
		imports = append(imports, byID[importID].coordinate)
	}
	sort.Slice(imports, func(left, right int) bool {
		return imports[left].String() < imports[right].String()
	})
	return ResolvedManifestNode{
		carrier:    node.carrier,
		coordinate: node.coordinate,
		imports:    imports,
	}
}

func importCycleIssues(
	nodes []manifestNode,
	indegree map[string]int,
	byID map[string]manifestNode,
) []LinkIssue {
	issues := make([]LinkIssue, 0)
	for _, node := range nodes {
		id := node.coordinate.ID()
		if indegree[id] == 0 || !participatesInImportCycle(id, byID) {
			continue
		}
		location := newCarrierSourceLocation(node.carrierID, node.span)
		issues = append(issues, newLinkIssue(
			IssueImportCycle,
			location,
			id,
			"the SignatureManifest import graph is cyclic",
			"remove at least one import edge so declarations have an acyclic source dependency graph",
		))
	}
	return issues
}

func participatesInImportCycle(
	start string,
	byID map[string]manifestNode,
) bool {
	startNode, exists := byID[start]
	if !exists {
		return false
	}
	pending := append([]string(nil), startNode.importIDs...)
	visited := make(map[string]struct{}, len(byID))
	for len(pending) > 0 {
		candidate := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if candidate == start {
			return true
		}
		if _, seen := visited[candidate]; seen {
			continue
		}
		visited[candidate] = struct{}{}
		node, found := byID[candidate]
		if found {
			pending = append(pending, node.importIDs...)
		}
	}
	return false
}

func rejectedResolution(issues []LinkIssue) BundleResolution {
	owned := cloneIssues(issues)
	sortIssues(owned)
	return rejectedBundleResolution{issues: owned}
}

func cloneResolvedNodes(nodes []ResolvedManifestNode) []ResolvedManifestNode {
	owned := append([]ResolvedManifestNode(nil), nodes...)
	for index := range owned {
		owned[index].imports = owned[index].Imports()
	}
	return owned
}

func nodeStructuralKey(node manifestNode) string {
	parts := []string{
		node.coordinate.ID(),
		node.coordinate.Version(),
		node.carrierID,
		node.edition,
		node.digest,
	}
	return strings.Join(parts, "\x00")
}

func isCanonicalSemVer(value string) bool {
	coreAndPrerelease, build, buildPresent := strings.Cut(value, "+")
	if buildPresent && (!validSemVerIdentifiers(build, false) || strings.Contains(build, "+")) {
		return false
	}
	core, prerelease, prereleasePresent := strings.Cut(coreAndPrerelease, "-")
	if prereleasePresent && !validSemVerIdentifiers(prerelease, true) {
		return false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !validSemVerNumber(part) {
			return false
		}
	}
	return true
}

func validSemVerNumber(value string) bool {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return false
	}
	for _, character := range value {
		if !unicode.IsDigit(character) || character > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func validSemVerIdentifiers(value string, rejectNumericLeadingZero bool) bool {
	parts := strings.Split(value, ".")
	for _, part := range parts {
		if !validSemVerIdentifier(part) {
			return false
		}
		if rejectNumericLeadingZero && numericIdentifier(part) && len(part) > 1 && part[0] == '0' {
			return false
		}
	}
	return true
}

func validSemVerIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		letter := character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
		digit := character >= '0' && character <= '9'
		if !letter && !digit && character != '-' {
			return false
		}
	}
	return true
}

func numericIdentifier(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
