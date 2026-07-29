package projecttypeenv

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const compositeLinkCanonicalDomain = "haft.fpf.projecttypeenv.composite-link-ir.v1"

const (
	maximumCompositeExtensionArtifacts = 1 << 10
	maximumCompositeExtensionBytes     = 64 << 20
	maximumCompositePredecessorEdges   = 64 << 10
)

const (
	IssueExtensionArtifactInvalid      LinkIssueCode = "extension_artifact_invalid"
	IssueExtensionCoordinateMismatch   LinkIssueCode = "extension_coordinate_ref_mismatch"
	IssueDuplicateExtensionRef         LinkIssueCode = "duplicate_extension_ref"
	IssueMissingExtensionPredecessor   LinkIssueCode = "missing_extension_predecessor"
	IssueExtensionPredecessorMismatch  LinkIssueCode = "extension_predecessor_mismatch"
	IssueBaseSymbolRedeclaration       LinkIssueCode = "base_symbol_redeclaration"
	IssueTransitiveSymbolRedeclaration LinkIssueCode = "transitive_symbol_redeclaration"
	IssueBranchSymbolConflict          LinkIssueCode = "branch_symbol_conflict"
	IssueSemanticEffectConflict        LinkIssueCode = "semantic_effect_coordinate_conflict"
	IssueManifestProvideMismatch       LinkIssueCode = "manifest_provide_mismatch"
	IssueGhostDependency               LinkIssueCode = "ghost_dependency"
	IssueNonImportedSiblingDependency  LinkIssueCode = "non_imported_sibling_dependency"
	IssueDependencyKindMismatch        LinkIssueCode = "dependency_kind_mismatch"
	IssueDependencySchemaInvalid       LinkIssueCode = "dependency_schema_invalid"
	IssueCompositeResourceLimit        LinkIssueCode = "composite_resource_limit"
)

// CompositeExtensionLocation points into one verified E artifact. It remains
// an artifact/source coordinate and does not imply graph, causal, or work
// order.
type CompositeExtensionLocation struct {
	coordinate ManifestCoordinate
	ref        typedmemory.TypeEnvExtensionRef
	span       SourceSpan
}

func newCompositeExtensionLocation(
	coordinate ManifestCoordinate,
	ref typedmemory.TypeEnvExtensionRef,
	span SourceSpan,
) CompositeExtensionLocation {
	return CompositeExtensionLocation{
		coordinate: coordinate,
		ref:        ref,
		span:       span,
	}
}

func (location CompositeExtensionLocation) Coordinate() ManifestCoordinate {
	return location.coordinate
}

func (location CompositeExtensionLocation) Ref() typedmemory.TypeEnvExtensionRef {
	return location.ref
}

func (location CompositeExtensionLocation) Span() SourceSpan { return location.span }

func (location CompositeExtensionLocation) String() string {
	return fmt.Sprintf(
		"%s/%s:%d-%d",
		location.coordinate.String(),
		location.ref.String(),
		location.span.Start(),
		location.span.End(),
	)
}

func (CompositeExtensionLocation) issueLocationVariant() {}

// CompositeInputLocation is used only when an untrusted E value fails before
// a source coordinate can be trusted.
type CompositeInputLocation struct {
	index int
}

func (location CompositeInputLocation) Index() int { return location.index }

func (location CompositeInputLocation) String() string {
	return fmt.Sprintf("project-typeenv-extension-input[%d]", location.index)
}

func (CompositeInputLocation) issueLocationVariant() {}

type CompositeLinkInputLocation struct{}

func (CompositeLinkInputLocation) String() string {
	return "project-typeenv-composite-input"
}

func (CompositeLinkInputLocation) issueLocationVariant() {}

type CompositeDependencyScope string

const (
	CompositeDependencyBase     CompositeDependencyScope = "base"
	CompositeDependencyOwn      CompositeDependencyScope = "own_extension"
	CompositeDependencyImported CompositeDependencyScope = "imported_extension"
)

// CompositeSymbolProvider is a closed provenance union. The provider proves
// only where an exact schema symbol was declared; it does not prove that the
// declaration is active in a project.
type CompositeSymbolProvider interface {
	Symbol() typedmemory.SchemaSymbolRef
	RawSymbol() string
	ProviderKind() string
	compositeSymbolProviderVariant()
}

type BaseCompositeSymbolProvider struct {
	symbol typedmemory.SchemaSymbolRef
	raw    string
}

func (provider BaseCompositeSymbolProvider) Symbol() typedmemory.SchemaSymbolRef {
	return provider.symbol
}

func (provider BaseCompositeSymbolProvider) RawSymbol() string { return provider.raw }

func (BaseCompositeSymbolProvider) ProviderKind() string { return "base" }

func (BaseCompositeSymbolProvider) compositeSymbolProviderVariant() {}

type ExtensionCompositeSymbolProvider struct {
	symbol          typedmemory.SchemaSymbolRef
	raw             string
	coordinate      ManifestCoordinate
	ref             typedmemory.TypeEnvExtensionRef
	declarationKind localpractice.DeclarationKind
}

func (provider ExtensionCompositeSymbolProvider) Symbol() typedmemory.SchemaSymbolRef {
	return provider.symbol
}

func (provider ExtensionCompositeSymbolProvider) RawSymbol() string { return provider.raw }

func (ExtensionCompositeSymbolProvider) ProviderKind() string { return "extension" }

func (provider ExtensionCompositeSymbolProvider) Coordinate() ManifestCoordinate {
	return provider.coordinate
}

func (provider ExtensionCompositeSymbolProvider) Ref() typedmemory.TypeEnvExtensionRef {
	return provider.ref
}

func (ExtensionCompositeSymbolProvider) compositeSymbolProviderVariant() {}

type CompositeDependencyResolution struct {
	consumerRef        typedmemory.TypeEnvExtensionRef
	consumerCoordinate ManifestCoordinate
	origin             string
	role               string
	target             typedmemory.SchemaSymbolRef
	source             SourceScalar
	scope              CompositeDependencyScope
	provider           CompositeSymbolProvider
}

func (resolution CompositeDependencyResolution) ConsumerRef() typedmemory.TypeEnvExtensionRef {
	return resolution.consumerRef
}

func (resolution CompositeDependencyResolution) ConsumerCoordinate() ManifestCoordinate {
	return resolution.consumerCoordinate
}

func (resolution CompositeDependencyResolution) Origin() string { return resolution.origin }

func (resolution CompositeDependencyResolution) Role() string { return resolution.role }

func (resolution CompositeDependencyResolution) Target() typedmemory.SchemaSymbolRef {
	return resolution.target
}

func (resolution CompositeDependencyResolution) Source() SourceScalar {
	return resolution.source
}

func (resolution CompositeDependencyResolution) Scope() CompositeDependencyScope {
	return resolution.scope
}

func (resolution CompositeDependencyResolution) Provider() CompositeSymbolProvider {
	return cloneCompositeProvider(resolution.provider)
}

type CompositeExternalReferenceKind string

const (
	CompositeExternalRule       CompositeExternalReferenceKind = "rule_ref"
	CompositeExternalCarrier    CompositeExternalReferenceKind = "carrier_ref"
	CompositeExternalClaim      CompositeExternalReferenceKind = "claim_or_invariant"
	CompositeExternalContext    CompositeExternalReferenceKind = "bounded_context"
	CompositeExternalSourceType CompositeExternalReferenceKind = "source_type"
)

// CompositeExternalReference makes an intentional non-schema dependency
// visible. These references are not eligible for ghost-symbol checks.
type CompositeExternalReference struct {
	consumerRef        typedmemory.TypeEnvExtensionRef
	consumerCoordinate ManifestCoordinate
	origin             string
	role               string
	kind               CompositeExternalReferenceKind
	source             SourceScalar
}

func (reference CompositeExternalReference) ConsumerRef() typedmemory.TypeEnvExtensionRef {
	return reference.consumerRef
}

func (reference CompositeExternalReference) ConsumerCoordinate() ManifestCoordinate {
	return reference.consumerCoordinate
}

func (reference CompositeExternalReference) Origin() string { return reference.origin }

func (reference CompositeExternalReference) Role() string { return reference.role }

func (reference CompositeExternalReference) Kind() CompositeExternalReferenceKind {
	return reference.kind
}

func (reference CompositeExternalReference) Source() SourceScalar { return reference.source }

// CompositeCarrierAssumption retains the exact source-owned carrier identity
// triple from a kind signature. It is unresolved provenance, not a schema
// symbol provider and not evidence that the named carrier was retrieved.
type CompositeCarrierAssumption struct {
	consumerRef        typedmemory.TypeEnvExtensionRef
	consumerCoordinate ManifestCoordinate
	origin             string
	carrierRef         SourceScalar
	edition            SourceScalar
	digest             SourceScalar
}

func (assumption CompositeCarrierAssumption) ConsumerRef() typedmemory.TypeEnvExtensionRef {
	return assumption.consumerRef
}

func (assumption CompositeCarrierAssumption) ConsumerCoordinate() ManifestCoordinate {
	return assumption.consumerCoordinate
}

func (assumption CompositeCarrierAssumption) Origin() string { return assumption.origin }

func (assumption CompositeCarrierAssumption) CarrierRef() SourceScalar {
	return assumption.carrierRef
}

func (assumption CompositeCarrierAssumption) Edition() SourceScalar {
	return assumption.edition
}

func (assumption CompositeCarrierAssumption) Digest() SourceScalar {
	return assumption.digest
}

type CompositeCoverageGapCode string

const CompositeGapStratumDirectionUnresolved CompositeCoverageGapCode = "stratum_direction_unresolved"

type CompositeCoverageGap struct {
	code       CompositeCoverageGapCode
	coordinate ManifestCoordinate
	ref        typedmemory.TypeEnvExtensionRef
	detail     string
}

func (gap CompositeCoverageGap) Code() CompositeCoverageGapCode { return gap.code }

func (gap CompositeCoverageGap) Coordinate() ManifestCoordinate { return gap.coordinate }

func (gap CompositeCoverageGap) Ref() typedmemory.TypeEnvExtensionRef { return gap.ref }

func (gap CompositeCoverageGap) Detail() string { return gap.detail }

type LinkedCompositeExtension struct {
	artifact     ProjectTypeEnvExtensionArtifact
	predecessors []ResolvedExtensionPredecessor
	ancestors    []typedmemory.TypeEnvExtensionRef
	provides     []typedmemory.SchemaSymbolRef
}

func (extension LinkedCompositeExtension) Artifact() ProjectTypeEnvExtensionArtifact {
	return extension.artifact
}

func (extension LinkedCompositeExtension) Ref() typedmemory.TypeEnvExtensionRef {
	return extension.artifact.Ref()
}

func (extension LinkedCompositeExtension) Coordinate() ManifestCoordinate {
	return extension.artifact.ManifestCoordinate()
}

func (extension LinkedCompositeExtension) DirectPredecessors() []ResolvedExtensionPredecessor {
	return append([]ResolvedExtensionPredecessor(nil), extension.predecessors...)
}

func (extension LinkedCompositeExtension) Ancestors() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), extension.ancestors...)
}

func (extension LinkedCompositeExtension) Provides() []typedmemory.SchemaSymbolRef {
	return append([]typedmemory.SchemaSymbolRef(nil), extension.provides...)
}

// LinkedProjectTypeEnvCompositeIR is a pure, self-reference-free proof of a
// verified B plus a canonically linked E DAG. Its canonical bytes are linker
// evidence only: this type deliberately cannot mint the later C TypeEnvRef.
type LinkedProjectTypeEnvCompositeIR struct {
	base         typeenv.BaseTypeEnvArtifact
	baseRef      typedmemory.TypeEnvRef
	extensions   []LinkedCompositeExtension
	dependencies []CompositeDependencyResolution
	externals    []CompositeExternalReference
	assumptions  []CompositeCarrierAssumption
	gaps         []CompositeCoverageGap
	canonical    []byte
}

func (ir LinkedProjectTypeEnvCompositeIR) BaseArtifact() typeenv.BaseTypeEnvArtifact {
	return ir.base
}

func (ir LinkedProjectTypeEnvCompositeIR) BaseTypeEnvRef() typedmemory.TypeEnvRef {
	return ir.baseRef
}

func (ir LinkedProjectTypeEnvCompositeIR) Extensions() []LinkedCompositeExtension {
	return cloneLinkedCompositeExtensions(ir.extensions)
}

func (ir LinkedProjectTypeEnvCompositeIR) DependencyResolutions() []CompositeDependencyResolution {
	return cloneCompositeDependencyResolutions(ir.dependencies)
}

func (ir LinkedProjectTypeEnvCompositeIR) ExternalReferences() []CompositeExternalReference {
	return append([]CompositeExternalReference(nil), ir.externals...)
}

func (ir LinkedProjectTypeEnvCompositeIR) CarrierAssumptions() []CompositeCarrierAssumption {
	return append([]CompositeCarrierAssumption(nil), ir.assumptions...)
}

func (ir LinkedProjectTypeEnvCompositeIR) CoverageGaps() []CompositeCoverageGap {
	return append([]CompositeCoverageGap(nil), ir.gaps...)
}

func (ir LinkedProjectTypeEnvCompositeIR) CanonicalBytes() []byte {
	return append([]byte(nil), ir.canonical...)
}

type CompositeIRLinkResolution interface {
	Rejected() bool
	Issues() []LinkIssue
	CompositeIR() (LinkedProjectTypeEnvCompositeIR, bool)
	compositeIRLinkResolutionVariant()
}

type acceptedCompositeIRLinkResolution struct {
	ir LinkedProjectTypeEnvCompositeIR
}

func (acceptedCompositeIRLinkResolution) Rejected() bool { return false }

func (acceptedCompositeIRLinkResolution) Issues() []LinkIssue { return nil }

func (resolution acceptedCompositeIRLinkResolution) CompositeIR() (
	LinkedProjectTypeEnvCompositeIR,
	bool,
) {
	return cloneLinkedProjectTypeEnvCompositeIR(resolution.ir), true
}

func (acceptedCompositeIRLinkResolution) compositeIRLinkResolutionVariant() {}

type rejectedCompositeIRLinkResolution struct {
	issues []LinkIssue
}

func (rejectedCompositeIRLinkResolution) Rejected() bool { return true }

func (resolution rejectedCompositeIRLinkResolution) Issues() []LinkIssue {
	return cloneIssues(resolution.issues)
}

func (rejectedCompositeIRLinkResolution) CompositeIR() (
	LinkedProjectTypeEnvCompositeIR,
	bool,
) {
	return LinkedProjectTypeEnvCompositeIR{}, false
}

func (rejectedCompositeIRLinkResolution) compositeIRLinkResolutionVariant() {}

type compositeExtensionNode struct {
	artifact   ProjectTypeEnvExtensionArtifact
	ref        typedmemory.TypeEnvExtensionRef
	coordinate ManifestCoordinate
	ir         ProjectTypeEnvExtensionIR
	ancestors  map[string]struct{}
}

type compositeProviderIndex struct {
	byTyped     map[string]CompositeSymbolProvider
	byRaw       map[string][]CompositeSymbolProvider
	byCollision map[string][]CompositeSymbolProvider
	byNode      map[string][]typedmemory.SchemaSymbolRef
}

// LinkProjectTypeEnvCompositeIR verifies and canonically links exact B/E
// artifacts. It performs no lowering, persistence, staging, activation,
// project-state evaluation, or active-head mutation.
func LinkProjectTypeEnvCompositeIR(
	base typeenv.BaseTypeEnvArtifact,
	extensions []ProjectTypeEnvExtensionArtifact,
) CompositeIRLinkResolution {
	baseRef, baseIssues := resolveBaseArtifact(base)
	if len(baseIssues) > 0 {
		return rejectedCompositeLink(baseIssues)
	}
	resourceIssues := compositeInputResourceIssues(extensions)
	if len(resourceIssues) > 0 {
		return rejectedCompositeLink(resourceIssues)
	}

	nodes, extensionIssues := verifyCompositeExtensionInputs(baseRef, extensions)
	if len(extensionIssues) > 0 {
		return rejectedCompositeLink(extensionIssues)
	}

	ordered, graphIssues := canonicalCompositeDAG(nodes)
	if len(graphIssues) > 0 {
		return rejectedCompositeLink(graphIssues)
	}
	populateCompositeAncestors(ordered)

	providers, providerIssues := buildCompositeProviderIndex(base, ordered)
	if len(providerIssues) > 0 {
		return rejectedCompositeLink(providerIssues)
	}

	dependencies, externals, dependencyIssues := resolveCompositeDependencies(
		ordered,
		providers,
	)
	if len(dependencyIssues) > 0 {
		return rejectedCompositeLink(dependencyIssues)
	}
	assumptions := collectCompositeCarrierAssumptions(ordered)

	linkedExtensions := linkedCompositeExtensions(ordered, providers)
	gaps := compositeCoverageGaps(ordered)
	ir := LinkedProjectTypeEnvCompositeIR{
		base:         base,
		baseRef:      baseRef,
		extensions:   linkedExtensions,
		dependencies: dependencies,
		externals:    externals,
		assumptions:  assumptions,
		gaps:         gaps,
	}
	ir.canonical = canonicalCompositeLinkIR(ir)
	return acceptedCompositeIRLinkResolution{ir: cloneLinkedProjectTypeEnvCompositeIR(ir)}
}

func verifyCompositeExtensionInputs(
	baseRef typedmemory.TypeEnvRef,
	extensions []ProjectTypeEnvExtensionArtifact,
) ([]compositeExtensionNode, []LinkIssue) {
	nodes := make([]compositeExtensionNode, 0, len(extensions))
	issues := make([]LinkIssue, 0)
	predecessorEdges := uint64(0)
	predecessorLimitExceeded := false
	for index, artifact := range extensions {
		if err := artifact.Verify(); err != nil {
			issues = append(issues, newLinkIssue(
				IssueExtensionArtifactInvalid,
				CompositeInputLocation{index: index},
				fmt.Sprintf("extension[%d]", index),
				err.Error(),
				"supply the exact verified ProjectTypeEnvExtensionArtifact bytes",
			))
			continue
		}
		ir := artifact.IR()
		nextPredecessorEdges, limitExceeded := addCompositeResourceWithinLimit(
			predecessorEdges,
			uint64(len(ir.Manifest().DirectPredecessors())),
			maximumCompositePredecessorEdges,
		)
		predecessorEdges = nextPredecessorEdges
		predecessorLimitExceeded = predecessorLimitExceeded || limitExceeded
		coordinate := artifact.ManifestCoordinate()
		location := newCompositeExtensionLocation(
			coordinate,
			artifact.Ref(),
			ir.Manifest().Span(),
		)
		if ir.BaseTypeEnvRef() != baseRef {
			issues = append(issues, newLinkIssue(
				IssueBaseRefMismatch,
				location,
				ir.BaseTypeEnvRef().String(),
				"extension base TypeEnvRef does not equal the exact verified B artifact",
				"recompile the extension against "+baseRef.String(),
			))
		}
		if artifact.Ref().ID().String() != coordinate.ID() {
			issues = append(issues, newLinkIssue(
				IssueExtensionCoordinateMismatch,
				location,
				artifact.Ref().String(),
				"E-ref ID does not equal SignatureManifest id",
				"reseal the exact source-compiled extension; never substitute an E-ref",
			))
		}
		nodes = append(nodes, compositeExtensionNode{
			artifact:   artifact,
			ref:        artifact.Ref(),
			coordinate: coordinate,
			ir:         ir,
			ancestors:  make(map[string]struct{}),
		})
	}
	if predecessorLimitExceeded {
		issues = append(issues, compositeResourceIssue(
			"direct-predecessor-edges",
			maximumCompositePredecessorEdges,
		))
	}
	issues = append(issues, compositeMultiplicityIssues(nodes)...)
	sortIssues(issues)
	return nodes, issues
}

func compositeInputResourceIssues(
	extensions []ProjectTypeEnvExtensionArtifact,
) []LinkIssue {
	issues := make([]LinkIssue, 0)
	if len(extensions) > maximumCompositeExtensionArtifacts {
		issues = append(issues, compositeResourceIssue(
			"extension-artifacts",
			maximumCompositeExtensionArtifacts,
		))
		return issues
	}
	totalBytes := uint64(0)
	limitExceeded := false
	for _, artifact := range extensions {
		totalBytes, limitExceeded = addCompositeResourceWithinLimit(
			totalBytes,
			uint64(len(artifact.canonical)),
			maximumCompositeExtensionBytes,
		)
		if limitExceeded {
			issues = append(issues, compositeResourceIssue(
				"aggregate-canonical-bytes",
				maximumCompositeExtensionBytes,
			))
			break
		}
	}
	return issues
}

func addCompositeResourceWithinLimit(
	current uint64,
	next uint64,
	limit uint64,
) (uint64, bool) {
	if current > limit || next > limit-current {
		return limit, true
	}
	return current + next, false
}

func compositeResourceIssue(subject string, limit uint64) LinkIssue {
	return newLinkIssue(
		IssueCompositeResourceLimit,
		CompositeLinkInputLocation{},
		subject,
		fmt.Sprintf("composite input exceeds the exact aggregate limit of %d", limit),
		"split the composite into a bounded exact input before linking",
	)
}

func compositeMultiplicityIssues(nodes []compositeExtensionNode) []LinkIssue {
	byRef := make(map[string][]compositeExtensionNode)
	byCoordinate := make(map[string][]compositeExtensionNode)
	byID := make(map[string][]compositeExtensionNode)
	for _, node := range nodes {
		byRef[node.ref.String()] = append(byRef[node.ref.String()], node)
		byCoordinate[node.coordinate.String()] = append(
			byCoordinate[node.coordinate.String()],
			node,
		)
		byID[node.coordinate.ID()] = append(byID[node.coordinate.ID()], node)
	}
	issues := make([]LinkIssue, 0)
	issues = append(issues, compositeMultiplicityGroupIssues(
		byRef,
		IssueDuplicateExtensionRef,
		"the link input repeats one exact E-ref",
	)...)
	issues = append(issues, compositeMultiplicityGroupIssues(
		byCoordinate,
		IssueDuplicateManifestCoordinate,
		"the link input contains more than one E artifact for one manifest coordinate",
	)...)
	for id, matches := range byID {
		if len(matches) < 2 || compositeNodesShareCoordinate(matches) {
			continue
		}
		issues = append(issues, compositeMultiplicityIssuesForMatches(
			matches,
			IssueDuplicateSignatureID,
			id,
			"the link input contains more than one version of one SignatureID",
		)...)
	}
	sortIssues(issues)
	return issues
}

func compositeMultiplicityGroupIssues(
	groups map[string][]compositeExtensionNode,
	code LinkIssueCode,
	detail string,
) []LinkIssue {
	issues := make([]LinkIssue, 0)
	for subject, matches := range groups {
		if len(matches) < 2 {
			continue
		}
		issues = append(issues, compositeMultiplicityIssuesForMatches(
			matches,
			code,
			subject,
			detail,
		)...)
	}
	return issues
}

func compositeMultiplicityIssuesForMatches(
	matches []compositeExtensionNode,
	code LinkIssueCode,
	subject string,
	detail string,
) []LinkIssue {
	issues := make([]LinkIssue, 0, len(matches))
	for _, match := range matches {
		issues = append(issues, newLinkIssue(
			code,
			compositeNodeLocation(match),
			subject,
			detail,
			"retain exactly one exact artifact for this identity",
		))
	}
	return issues
}

func compositeNodesShareCoordinate(nodes []compositeExtensionNode) bool {
	first := nodes[0].coordinate.String()
	for _, node := range nodes[1:] {
		if node.coordinate.String() != first {
			return false
		}
	}
	return true
}

func canonicalCompositeDAG(
	nodes []compositeExtensionNode,
) ([]compositeExtensionNode, []LinkIssue) {
	byRef := make(map[string]compositeExtensionNode, len(nodes))
	byCoordinate := make(map[string]compositeExtensionNode, len(nodes))
	for _, node := range nodes {
		byRef[node.ref.String()] = node
		byCoordinate[node.coordinate.String()] = node
	}
	issues := validateCompositePredecessors(nodes, byRef, byCoordinate)
	if len(issues) > 0 {
		return nil, issues
	}

	indegree := make(map[string]int, len(nodes))
	children := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		key := node.ref.String()
		predecessors := node.ir.Manifest().DirectPredecessors()
		indegree[key] = len(predecessors)
		for _, predecessor := range predecessors {
			parentKey := predecessor.Ref().String()
			children[parentKey] = append(children[parentKey], key)
		}
	}
	for key := range children {
		sort.Strings(children[key])
	}

	ready := make([]compositeExtensionNode, 0)
	for _, node := range nodes {
		if indegree[node.ref.String()] == 0 {
			ready = append(ready, node)
		}
	}
	sortCompositeNodes(ready)
	ordered := make([]compositeExtensionNode, 0, len(nodes))
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		ordered = append(ordered, current)
		for _, childKey := range children[current.ref.String()] {
			indegree[childKey]--
			if indegree[childKey] != 0 {
				continue
			}
			ready = append(ready, byRef[childKey])
			sortCompositeNodes(ready)
		}
	}
	if len(ordered) == len(nodes) {
		return ordered, nil
	}
	cycleIssues := compositeCycleIssues(nodes, indegree)
	return nil, cycleIssues
}

func validateCompositePredecessors(
	nodes []compositeExtensionNode,
	byRef map[string]compositeExtensionNode,
	byCoordinate map[string]compositeExtensionNode,
) []LinkIssue {
	issues := make([]LinkIssue, 0)
	for _, node := range nodes {
		manifest := node.ir.Manifest()
		location := compositeNodeLocation(node)
		for _, predecessor := range manifest.DirectPredecessors() {
			if predecessor.Ref() == node.ref {
				issues = append(issues, newLinkIssue(
					IssueSelfImport,
					location,
					predecessor.Ref().String(),
					"extension imports its own exact E-ref",
					"remove the self import and reseal the extension",
				))
				continue
			}
			actual, exists := byRef[predecessor.Ref().String()]
			if !exists {
				_, coordinateExists := byCoordinate[predecessor.Coordinate().String()]
				code := IssueMissingExtensionPredecessor
				detail := "exact predecessor E-ref is absent from the link input"
				if coordinateExists {
					code = IssueExtensionPredecessorMismatch
					detail = "manifest coordinate is present with different exact E bytes/ref"
				}
				issues = append(issues, newLinkIssue(
					code,
					location,
					predecessor.Ref().String(),
					detail,
					"supply the exact predecessor artifact named by the sealed child",
				))
				continue
			}
			if actual.coordinate.String() == predecessor.Coordinate().String() {
				continue
			}
			issues = append(issues, newLinkIssue(
				IssueExtensionPredecessorMismatch,
				location,
				predecessor.Ref().String(),
				"predecessor E-ref resolves to a different manifest coordinate",
				"recompile and reseal the child from the exact predecessor coordinate",
			))
		}
	}
	sortIssues(issues)
	return issues
}

func compositeCycleIssues(
	nodes []compositeExtensionNode,
	indegree map[string]int,
) []LinkIssue {
	parents := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		for _, predecessor := range node.ir.Manifest().DirectPredecessors() {
			parents[node.ref.String()] = append(
				parents[node.ref.String()],
				predecessor.Ref().String(),
			)
		}
	}
	issues := make([]LinkIssue, 0)
	for _, node := range nodes {
		if indegree[node.ref.String()] == 0 {
			continue
		}
		if !compositeNodeParticipatesInCycle(node.ref.String(), parents) {
			continue
		}
		issues = append(issues, newLinkIssue(
			IssueImportCycle,
			compositeNodeLocation(node),
			node.ref.String(),
			"extension remains in the cyclic predecessor subgraph",
			"remove the cycle and reseal every affected content-addressed extension",
		))
	}
	sortIssues(issues)
	return issues
}

func compositeNodeParticipatesInCycle(
	start string,
	parents map[string][]string,
) bool {
	visited := make(map[string]struct{})
	pending := append([]string(nil), parents[start]...)
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		if current == start {
			return true
		}
		if _, seen := visited[current]; seen {
			continue
		}
		visited[current] = struct{}{}
		pending = append(pending, parents[current]...)
	}
	return false
}

func sortCompositeNodes(nodes []compositeExtensionNode) {
	sort.Slice(nodes, func(left, right int) bool {
		return compositeNodeKey(nodes[left]) < compositeNodeKey(nodes[right])
	})
}

func compositeNodeKey(node compositeExtensionNode) string {
	return node.coordinate.ID() + "\x00" +
		node.coordinate.Version() + "\x00" +
		node.ref.String()
}

func compositeNodeLocation(node compositeExtensionNode) CompositeExtensionLocation {
	return newCompositeExtensionLocation(
		node.coordinate,
		node.ref,
		node.ir.Manifest().Span(),
	)
}

func populateCompositeAncestors(nodes []compositeExtensionNode) {
	byRef := make(map[string]int, len(nodes))
	for index := range nodes {
		byRef[nodes[index].ref.String()] = index
	}
	for index := range nodes {
		ancestors := make(map[string]struct{})
		for _, predecessor := range nodes[index].ir.Manifest().DirectPredecessors() {
			parentKey := predecessor.Ref().String()
			ancestors[parentKey] = struct{}{}
			parentIndex := byRef[parentKey]
			for ancestor := range nodes[parentIndex].ancestors {
				ancestors[ancestor] = struct{}{}
			}
		}
		nodes[index].ancestors = ancestors
	}
}

func buildCompositeProviderIndex(
	base typeenv.BaseTypeEnvArtifact,
	nodes []compositeExtensionNode,
) (compositeProviderIndex, []LinkIssue) {
	index := compositeProviderIndex{
		byTyped:     make(map[string]CompositeSymbolProvider),
		byRaw:       make(map[string][]CompositeSymbolProvider),
		byCollision: make(map[string][]CompositeSymbolProvider),
		byNode:      make(map[string][]typedmemory.SchemaSymbolRef),
	}
	for _, entry := range base.SymbolManifest().Entries() {
		provider := BaseCompositeSymbolProvider{
			symbol: entry.Symbol(),
			raw:    entry.Symbol().Key(),
		}
		index.add(provider)
	}

	issues := semanticEffectConflictIssues(base, nodes)
	for _, node := range nodes {
		providers, declarationIssues := extensionProviders(node)
		issues = append(issues, declarationIssues...)
		for _, provider := range providers {
			collisionKey := compositeProviderCollisionKey(provider)
			conflicts := index.byCollision[collisionKey]
			issues = append(
				issues,
				compositeProviderConflictIssues(node, provider, conflicts)...,
			)
			index.add(provider)
			index.byNode[node.ref.String()] = append(
				index.byNode[node.ref.String()],
				provider.Symbol(),
			)
		}
	}
	for key := range index.byNode {
		sort.Slice(index.byNode[key], func(left, right int) bool {
			return index.byNode[key][left].String() < index.byNode[key][right].String()
		})
	}
	sortIssues(issues)
	return index, issues
}

type compositeSemanticEffect struct {
	location   IssueLocation
	coordinate string
	symbol     string
}

func semanticEffectConflictIssues(
	base typeenv.BaseTypeEnvArtifact,
	nodes []compositeExtensionNode,
) []LinkIssue {
	byCoordinate := make(map[string][]compositeSemanticEffect)
	for _, projection := range base.SymbolManifest().Entries() {
		coordinate, exists := baseSemanticEffectCoordinate(projection)
		if !exists {
			continue
		}
		byCoordinate[coordinate] = append(
			byCoordinate[coordinate],
			compositeSemanticEffect{
				location:   BaseArtifactLocation{},
				coordinate: coordinate,
				symbol:     projection.Symbol().String(),
			},
		)
	}
	for _, node := range nodes {
		context := node.ir.BoundedContext().Value()
		declarations := node.ir.Signature().Vocabulary().Declarations()
		for _, declaration := range declarations {
			coordinate, exists := semanticEffectCoordinate(declaration, context)
			if !exists {
				continue
			}
			byCoordinate[coordinate] = append(
				byCoordinate[coordinate],
				compositeSemanticEffect{
					location:   compositeNodeLocation(node),
					coordinate: coordinate,
					symbol:     declaration.Symbol().Value(),
				},
			)
		}
	}
	issues := make([]LinkIssue, 0)
	for coordinate, effects := range byCoordinate {
		if len(effects) < 2 {
			continue
		}
		for _, effect := range effects {
			issues = append(issues, newLinkIssue(
				IssueSemanticEffectConflict,
				effect.location,
				coordinate,
				"declarations including "+effect.symbol+
					" target one runtime semantic coordinate",
				"retain one definition for this coordinate; aliases cannot conceal duplicate effects",
			))
		}
	}
	sortIssues(issues)
	return issues
}

func baseSemanticEffectCoordinate(
	projection typeenv.DeclarationProjection,
) (string, bool) {
	dependencies := projection.Dependencies()
	switch projection.Symbol().Kind() {
	case typedmemory.ContextSymbol:
		return "bounded_context\x00" + projection.Symbol().Key(), true
	case typedmemory.EntitySetSymbol:
		context, exists := dependencyBySchemaKind(dependencies, typedmemory.ContextSymbol)
		if !exists {
			return "", false
		}
		return "entity_set\x00" + context.Key(), true
	case typedmemory.KindSignatureSymbol:
		kind, kindExists := dependencyBySchemaKind(dependencies, typedmemory.KindSymbol)
		context, contextExists := dependencyBySchemaKind(dependencies, typedmemory.ContextSymbol)
		if !kindExists || !contextExists {
			return "", false
		}
		return "kind_signature\x00" + kind.Key() + "\x00" + context.Key(), true
	case typedmemory.CodecSymbol:
		kind, exists := dependencyBySchemaKind(dependencies, typedmemory.KindSymbol)
		if !exists {
			return "", false
		}
		return "codec_binding\x00" + kind.Key(), true
	case typedmemory.BridgeSymbol:
		return "context_bridge\x00" + projection.Symbol().Key(), true
	default:
		return "", false
	}
}

func dependencyBySchemaKind(
	dependencies []typedmemory.SchemaSymbolRef,
	kind typedmemory.SchemaSymbolKind,
) (typedmemory.SchemaSymbolRef, bool) {
	for _, dependency := range dependencies {
		if dependency.Kind() == kind {
			return dependency, true
		}
	}
	return typedmemory.SchemaSymbolRef{}, false
}

func semanticEffectCoordinate(
	declaration SymbolicDeclaration,
	context string,
) (string, bool) {
	switch declaration.Kind() {
	case localpractice.DeclarationBoundedContext:
		return "bounded_context\x00" + declaration.Symbol().Value(), true
	case localpractice.DeclarationSubkind:
		child, childExists := declarationDependencyByRole(declaration, "child_kind")
		super, superExists := declarationDependencyByRole(declaration, "super_kind")
		if !childExists || !superExists {
			return "", false
		}
		return "subkind\x00" + child.Target().Value() + "\x00" + super.Target().Value(), true
	case localpractice.DeclarationEntitySet:
		return "entity_set\x00" + context, true
	case localpractice.DeclarationKindSignature:
		valueKind, exists := declarationDependencyByRole(declaration, "value_kind")
		if !exists {
			return "", false
		}
		return "kind_signature\x00" + valueKind.Target().Value() + "\x00" + context, true
	case localpractice.DeclarationKindClassificationSignature:
		localKind, exists := declarationDependencyByRole(declaration, "local_kind")
		if !exists {
			return "", false
		}
		return "kind_classification_signature\x00" +
			localKind.Target().Value() + "\x00" + context, true
	case localpractice.DeclarationCodecBinding:
		valueKind, exists := declarationDependencyByRole(declaration, "value_kind")
		if !exists {
			return "", false
		}
		return "codec_binding\x00" + valueKind.Target().Value(), true
	case localpractice.DeclarationRuntimeEvaluatorRequirement:
		rule := factsAtPath(declaration.Facts(), "rule_ref")
		contract := factsAtPath(declaration.Facts(), "invocation_contract")
		if len(rule) != 1 || len(contract) != 1 {
			return "", false
		}
		return "runtime_evaluator_requirement\x00" +
			contract[0].Value() + "\x00" + rule[0].Value(), true
	case localpractice.DeclarationKindBridge:
		return "context_bridge\x00" + declaration.Symbol().Value(), true
	default:
		return "", false
	}
}

func declarationDependencyByRole(
	declaration SymbolicDeclaration,
	role string,
) (SymbolicDependency, bool) {
	for _, dependency := range declaration.Dependencies() {
		if dependency.Role() == role {
			return dependency, true
		}
	}
	return SymbolicDependency{}, false
}

func (index *compositeProviderIndex) add(provider CompositeSymbolProvider) {
	typedKey := provider.Symbol().String()
	if _, exists := index.byTyped[typedKey]; !exists {
		index.byTyped[typedKey] = cloneCompositeProvider(provider)
	}
	index.byRaw[provider.RawSymbol()] = append(
		index.byRaw[provider.RawSymbol()],
		cloneCompositeProvider(provider),
	)
	collisionKey := compositeProviderCollisionKey(provider)
	index.byCollision[collisionKey] = append(
		index.byCollision[collisionKey],
		cloneCompositeProvider(provider),
	)
}

func compositeProviderCollisionKey(provider CompositeSymbolProvider) string {
	if provider.Symbol().Kind() == typedmemory.SlotKindSymbol {
		return "relation-local-slot\x00" + provider.Symbol().String()
	}
	return "manifest-symbol\x00" + provider.RawSymbol()
}

func extensionProviders(
	node compositeExtensionNode,
) ([]ExtensionCompositeSymbolProvider, []LinkIssue) {
	providers := make([]ExtensionCompositeSymbolProvider, 0)
	issues := make([]LinkIssue, 0)
	derivedProvides := make(map[string]struct{})
	declarations := node.ir.Signature().Vocabulary().Declarations()
	for _, declaration := range declarations {
		for _, exported := range declaration.Exports() {
			symbol, err := extensionExportSymbol(declaration, exported.Value())
			if err != nil {
				issues = append(issues, newLinkIssue(
					IssueDependencySchemaInvalid,
					compositeNodeLocation(node),
					exported.Value(),
					err.Error(),
					"repair and reseal the source declaration",
				))
				continue
			}
			providers = append(providers, ExtensionCompositeSymbolProvider{
				symbol:          symbol,
				raw:             exported.Value(),
				coordinate:      node.coordinate,
				ref:             node.ref,
				declarationKind: declaration.Kind(),
			})
			derivedProvides[exported.Value()] = struct{}{}
		}
	}
	manifestProvides := make(map[string]struct{})
	for _, provided := range node.ir.Manifest().Provides() {
		manifestProvides[provided.Value()] = struct{}{}
	}
	if !sameStringSet(derivedProvides, manifestProvides) {
		issues = append(issues, newLinkIssue(
			IssueManifestProvideMismatch,
			compositeNodeLocation(node),
			node.coordinate.String(),
			"SignatureManifest provides do not exactly equal declaration exports",
			"repair and reseal the source-compiled extension",
		))
	}
	sort.Slice(providers, func(left, right int) bool {
		return providers[left].symbol.String() < providers[right].symbol.String()
	})
	return providers, issues
}

func extensionExportSymbol(
	declaration SymbolicDeclaration,
	raw string,
) (typedmemory.SchemaSymbolRef, error) {
	switch declaration.Kind() {
	case localpractice.DeclarationBoundedContext:
		ref, err := typedmemory.NewBoundedContextRef(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.BoundedContextSymbolRef(ref)
	case localpractice.DeclarationValueKind:
		id, err := typedmemory.NewKindID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.KindSymbolRef(id)
	case localpractice.DeclarationRefKind:
		id, err := typedmemory.NewRefKindID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.RefKindSymbolRef(id)
	case localpractice.DeclarationEntitySet:
		id, err := typedmemory.NewEntitySetSymbolID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.EntitySetSymbolRef(id)
	case localpractice.DeclarationKindSignature,
		localpractice.DeclarationKindClassificationSignature:
		id, err := typedmemory.NewKindSignatureSymbolID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.KindSignatureSymbolRef(id)
	case localpractice.DeclarationRelationSignature:
		return slottedSignatureExportSymbol(declaration, raw)
	case localpractice.DeclarationRuntimeEvaluatorInput:
		return slottedSignatureExportSymbol(declaration, raw)
	case localpractice.DeclarationValueShape:
		id, err := typedmemory.NewShapeID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ValueShapeSymbolRef(id)
	case localpractice.DeclarationCodecBinding:
		id, err := typedmemory.NewCodecID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.CodecSymbolRef(id)
	case localpractice.DeclarationRuntimeEvaluatorRequirement:
		id, err := typedmemory.NewConstraintID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ConstraintSymbolRef(id)
	case localpractice.DeclarationConstraint:
		id, err := typedmemory.NewConstraintID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ConstraintSymbolRef(id)
	case localpractice.DeclarationKindBridge:
		id, err := typedmemory.NewContextBridgeID(raw)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ContextBridgeSymbolRef(id)
	default:
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf(
			"unsupported declaration kind %q",
			declaration.Kind(),
		)
	}
}

// slottedSignatureExportSymbol gives linker identity to a declaration that
// exports one signature-like carrier plus SlotKinds. It does not lower the
// declaration to a typedmemory.TypedRelationDeclarationFragment or assert that
// any predicate obtains; only the historical DeclarationRelationSignature
// source token is consumed by fragment lowering.
func slottedSignatureExportSymbol(
	declaration SymbolicDeclaration,
	raw string,
) (typedmemory.SchemaSymbolRef, error) {
	relationID, err := typedmemory.NewSignatureID(declaration.Symbol().Value())
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	if raw == declaration.Symbol().Value() {
		return typedmemory.RelationSymbolRef(relationID)
	}
	slotID, err := typedmemory.NewSlotKindID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	return typedmemory.SlotKindSymbolRef(relationID, slotID)
}

func compositeProviderConflictIssues(
	node compositeExtensionNode,
	provider ExtensionCompositeSymbolProvider,
	conflicts []CompositeSymbolProvider,
) []LinkIssue {
	issues := make([]LinkIssue, 0)
	for _, conflict := range conflicts {
		code := IssueBranchSymbolConflict
		detail := "two unrelated extension branches provide the same SymbolID"
		switch existing := conflict.(type) {
		case BaseCompositeSymbolProvider:
			code = IssueBaseSymbolRedeclaration
			detail = "extension redeclares a symbol from the exact B manifest"
		case ExtensionCompositeSymbolProvider:
			if _, imported := node.ancestors[existing.ref.String()]; imported {
				code = IssueTransitiveSymbolRedeclaration
				detail = "extension redeclares a symbol provided by a transitive import"
			}
			if existing.ref == node.ref {
				code = IssueManifestProvideMismatch
				detail = "one extension provides the same SymbolID more than once"
			}
		}
		issues = append(issues, newLinkIssue(
			code,
			compositeNodeLocation(node),
			provider.RawSymbol(),
			detail,
			"use one non-shadowing SymbolID and reseal the affected extension",
		))
	}
	return issues
}

func resolveCompositeDependencies(
	nodes []compositeExtensionNode,
	providers compositeProviderIndex,
) ([]CompositeDependencyResolution, []CompositeExternalReference, []LinkIssue) {
	dependencies := make([]CompositeDependencyResolution, 0)
	externals := make([]CompositeExternalReference, 0)
	issues := make([]LinkIssue, 0)
	for _, node := range nodes {
		uses, nodeExternals, useIssues := compositeDependencyUses(node)
		externals = append(externals, nodeExternals...)
		issues = append(issues, useIssues...)
		for _, use := range uses {
			resolution, issue := resolveCompositeDependency(node, use, providers)
			if issue != nil {
				issues = append(issues, *issue)
				continue
			}
			dependencies = append(dependencies, resolution)
		}
	}
	sort.Slice(dependencies, func(left, right int) bool {
		return compositeDependencyResolutionKey(dependencies[left]) <
			compositeDependencyResolutionKey(dependencies[right])
	})
	sort.Slice(externals, func(left, right int) bool {
		return compositeExternalReferenceKey(externals[left]) <
			compositeExternalReferenceKey(externals[right])
	})
	sortIssues(issues)
	return dependencies, externals, issues
}

func collectCompositeCarrierAssumptions(
	nodes []compositeExtensionNode,
) []CompositeCarrierAssumption {
	result := make([]CompositeCarrierAssumption, 0)
	for _, node := range nodes {
		declarations := node.ir.Signature().Vocabulary().Declarations()
		for _, declaration := range declarations {
			if declaration.Kind() != localpractice.DeclarationKindSignature &&
				declaration.Kind() != localpractice.DeclarationKindClassificationSignature {
				continue
			}
			result = append(
				result,
				carrierAssumptionsFromDeclaration(node, declaration)...,
			)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return compositeCarrierAssumptionKey(result[left]) <
			compositeCarrierAssumptionKey(result[right])
	})
	return result
}

type compositeCarrierAssumptionParts struct {
	carrierRef SourceScalar
	edition    SourceScalar
	digest     SourceScalar
}

func carrierAssumptionsFromDeclaration(
	node compositeExtensionNode,
	declaration SymbolicDeclaration,
) []CompositeCarrierAssumption {
	byPrefix := make(map[string]compositeCarrierAssumptionParts)
	for _, fact := range declaration.Facts() {
		prefix, field, found := splitCarrierAssumptionPath(fact.Path())
		if !found {
			continue
		}
		parts := byPrefix[prefix]
		switch field {
		case "carrier_ref":
			parts.carrierRef = fact.Value()
		case "edition":
			parts.edition = fact.Value()
		case "digest":
			parts.digest = fact.Value()
		}
		byPrefix[prefix] = parts
	}
	result := make([]CompositeCarrierAssumption, 0, len(byPrefix))
	for prefix, parts := range byPrefix {
		result = append(result, CompositeCarrierAssumption{
			consumerRef:        node.ref,
			consumerCoordinate: node.coordinate,
			origin:             "declaration:" + declaration.Symbol().Value() + "/" + prefix,
			carrierRef:         parts.carrierRef,
			edition:            parts.edition,
			digest:             parts.digest,
		})
	}
	return result
}

func splitCarrierAssumptionPath(path string) (string, string, bool) {
	for _, field := range []string{"carrier_ref", "edition", "digest"} {
		suffix := "." + field
		if path == "reference_scheme"+suffix {
			return "reference_scheme", field, true
		}
		indexed := strings.HasPrefix(path, "assumptions[") ||
			strings.HasPrefix(path, "dependencies[")
		if indexed && strings.HasSuffix(path, suffix) {
			return strings.TrimSuffix(path, suffix), field, true
		}
	}
	return "", "", false
}

type compositeDependencyUse struct {
	origin                  string
	role                    string
	raw                     string
	target                  typedmemory.SchemaSymbolRef
	source                  SourceScalar
	expectedDeclarationKind localpractice.DeclarationKind
}

func compositeDependencyUses(
	node compositeExtensionNode,
) ([]compositeDependencyUse, []CompositeExternalReference, []LinkIssue) {
	uses := make([]compositeDependencyUse, 0)
	externals := make([]CompositeExternalReference, 0)
	issues := make([]LinkIssue, 0)
	rows := node.ir.Signature()

	for _, fact := range rows.SubjectBlock().Facts() {
		switch fact.Path() {
		case "subject_kind", "ranged_value_kind", "result_kind":
			use, err := newKindDependencyUse("signature.subject_block", fact.Path(), fact.Value())
			if err != nil {
				issues = append(issues, compositeDependencySchemaIssue(node, fact.Value(), err))
				continue
			}
			uses = append(uses, use)
		case "slice_set":
			externals = append(externals, newCompositeExternalReference(
				node,
				"signature.subject_block",
				fact.Path(),
				CompositeExternalSourceType,
				fact.Value(),
			))
		case "extent_rule":
			externals = append(externals, newCompositeExternalReference(
				node,
				"signature.subject_block",
				fact.Path(),
				CompositeExternalRule,
				fact.Value(),
			))
		}
	}

	for _, fact := range rows.Laws().Facts() {
		if strings.HasPrefix(fact.Path(), "constraint_refs[") {
			use, err := newConstraintDependencyUse("signature.laws", fact.Path(), fact.Value())
			if err != nil {
				issues = append(issues, compositeDependencySchemaIssue(node, fact.Value(), err))
				continue
			}
			uses = append(uses, use)
			continue
		}
		externals = append(externals, newCompositeExternalReference(
			node,
			"signature.laws",
			fact.Path(),
			CompositeExternalClaim,
			fact.Value(),
		))
	}

	for _, fact := range rows.Applicability().Facts() {
		kind := CompositeExternalClaim
		if fact.Path() == "bounded_context_ref" {
			kind = CompositeExternalContext
		}
		externals = append(externals, newCompositeExternalReference(
			node,
			"signature.applicability",
			fact.Path(),
			kind,
			fact.Value(),
		))
	}

	for _, declaration := range rows.Vocabulary().Declarations() {
		declarationUses, declarationExternals, err := declarationDependencyUses(
			node,
			declaration,
		)
		if err != nil {
			issues = append(issues, compositeDependencySchemaIssue(node, declaration.Symbol(), err))
			continue
		}
		uses = append(uses, declarationUses...)
		externals = append(externals, declarationExternals...)
	}
	return uses, externals, issues
}

func declarationDependencyUses(
	node compositeExtensionNode,
	declaration SymbolicDeclaration,
) ([]compositeDependencyUse, []CompositeExternalReference, error) {
	uses := make([]compositeDependencyUse, 0)
	externals := make([]CompositeExternalReference, 0)
	if declaration.Kind() == localpractice.DeclarationRuntimeEvaluatorRequirement {
		rules := factsAtPath(declaration.Facts(), "rule_ref")
		if len(rules) != 1 {
			return nil, nil, fmt.Errorf(
				"runtime evaluator requirement %q requires one rule_ref fact",
				declaration.Symbol().Value(),
			)
		}
		externals = append(externals, newCompositeExternalReference(
			node,
			"declaration:"+declaration.Symbol().Value(),
			"rule_ref",
			CompositeExternalRule,
			rules[0],
		))
	}
	relationRaw := declarationConstraintRelation(declaration)
	for _, dependency := range declaration.Dependencies() {
		origin := "declaration:" + declaration.Symbol().Value()
		target, externalKind, semantic, err := declarationDependencyTarget(
			declaration,
			dependency,
			relationRaw,
		)
		if err != nil {
			return nil, nil, err
		}
		if !semantic {
			externals = append(externals, newCompositeExternalReference(
				node,
				origin,
				dependency.Role(),
				externalKind,
				dependency.Target(),
			))
			continue
		}
		use := compositeDependencyUse{
			origin: origin,
			role:   dependency.Role(),
			raw:    dependency.Target().Value(),
			target: target,
			source: dependency.Target(),
		}
		if declaration.Kind() == localpractice.DeclarationRuntimeEvaluatorInput &&
			dependency.Role() == "evaluator_requirement" {
			use.expectedDeclarationKind = localpractice.DeclarationRuntimeEvaluatorRequirement
		}
		uses = append(uses, use)
	}
	return uses, externals, nil
}

func declarationDependencyTarget(
	declaration SymbolicDeclaration,
	dependency SymbolicDependency,
	relationRaw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	role := dependency.Role()
	raw := dependency.Target().Value()
	switch declaration.Kind() {
	case localpractice.DeclarationBoundedContext:
		return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
			"bounded-context declaration contains unexpected dependency role %q",
			role,
		)
	case localpractice.DeclarationSubkind:
		return subkindDeclarationDependencyTarget(role, raw)
	case localpractice.DeclarationRefKind:
		return schemaKindSymbol(raw)
	case localpractice.DeclarationEntitySet:
		return typedmemory.SchemaSymbolRef{}, CompositeExternalRule, false, nil
	case localpractice.DeclarationKindSignature:
		return kindSignatureDependencyTarget(role, raw)
	case localpractice.DeclarationKindClassificationSignature:
		return kindClassificationSignatureDependencyTarget(role, raw)
	case localpractice.DeclarationRelationSignature:
		return relationDeclarationDependencyTarget(role, raw)
	case localpractice.DeclarationRuntimeEvaluatorInput:
		return runtimeEvaluatorInputDependencyTarget(role, raw)
	case localpractice.DeclarationValueShape:
		return schemaShapeSymbol(raw)
	case localpractice.DeclarationCodecBinding:
		return codecDeclarationDependencyTarget(role, raw)
	case localpractice.DeclarationRuntimeEvaluatorRequirement:
		return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
			"runtime evaluator requirement contains unexpected dependency role %q",
			role,
		)
	case localpractice.DeclarationConstraint:
		return constraintDeclarationDependencyTarget(role, raw, relationRaw)
	case localpractice.DeclarationKindBridge:
		return kindBridgeDeclarationDependencyTarget(role, raw)
	default:
		return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
			"declaration %q contains unsupported dependency role %q",
			declaration.Symbol().Value(),
			role,
		)
	}
}

func subkindDeclarationDependencyTarget(
	role string,
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	switch role {
	case "child_kind", "super_kind":
		return schemaKindSymbol(raw)
	default:
		return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
			"subkind dependency role %q is unsupported",
			role,
		)
	}
}

func kindBridgeDeclarationDependencyTarget(
	role string,
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	switch role {
	case "mapping.source_kind", "mapping.target_kind":
		return schemaKindSymbol(raw)
	default:
		return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
			"kind-bridge dependency role %q is unsupported",
			role,
		)
	}
}

func kindSignatureDependencyTarget(
	role string,
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	if role == "value_kind" {
		return schemaKindSymbol(raw)
	}
	if role == "entity_set" {
		return schemaEntitySetSymbol(raw)
	}
	if strings.HasSuffix(role, ".carrier_ref") {
		return typedmemory.SchemaSymbolRef{}, CompositeExternalCarrier, false, nil
	}
	return typedmemory.SchemaSymbolRef{}, CompositeExternalRule, false, nil
}

func kindClassificationSignatureDependencyTarget(
	role string,
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	switch role {
	case "local_kind", "candidate_value_kind":
		return schemaKindSymbol(raw)
	case "criterion_rule", "slice_conditions_rule", "extent_rule.rule_ref":
		return typedmemory.SchemaSymbolRef{}, CompositeExternalRule, false, nil
	case "reference_scheme.carrier_ref":
		return typedmemory.SchemaSymbolRef{}, CompositeExternalCarrier, false, nil
	default:
		if strings.HasPrefix(role, "dependencies[") &&
			strings.HasSuffix(role, "].carrier_ref") {
			return typedmemory.SchemaSymbolRef{}, CompositeExternalCarrier, false, nil
		}
		return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
			"kind-classification signature dependency role %q is unsupported",
			role,
		)
	}
}

func relationDeclarationDependencyTarget(
	role string,
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	if strings.HasSuffix(role, ".value_kind") {
		return schemaKindSymbol(raw)
	}
	if strings.HasSuffix(role, ".ref_mode.ref_kind") {
		return schemaRefKindSymbol(raw)
	}
	return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
		"relation dependency role %q is unsupported",
		role,
	)
}

func runtimeEvaluatorInputDependencyTarget(
	role string,
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	if role == "evaluator_requirement" {
		return schemaConstraintSymbol(raw)
	}
	return relationDeclarationDependencyTarget(role, raw)
}

func codecDeclarationDependencyTarget(
	role string,
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	if role == "value_kind" {
		return schemaKindSymbol(raw)
	}
	if role == "value_shape" {
		return schemaShapeSymbol(raw)
	}
	return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
		"codec dependency role %q is unsupported",
		role,
	)
}

func constraintDeclarationDependencyTarget(
	role string,
	raw string,
	relationRaw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	if role == "constraint.relation" {
		return schemaRelationSymbol(raw)
	}
	if strings.HasPrefix(role, "rule.disjoint_kinds[") {
		return schemaKindSymbol(raw)
	}
	if isConstraintSlotRole(role) {
		return schemaSlotSymbol(relationRaw, raw)
	}
	return typedmemory.SchemaSymbolRef{}, "", false, fmt.Errorf(
		"constraint dependency role %q is unsupported",
		role,
	)
}

func isConstraintSlotRole(role string) bool {
	return role == "constraint.slot" ||
		role == "constraint.subset" ||
		role == "constraint.superset" ||
		role == "constraint.whole" ||
		strings.HasPrefix(role, "rule.slots[") ||
		strings.HasPrefix(role, "rule.parts[")
}

func declarationConstraintRelation(declaration SymbolicDeclaration) string {
	for _, dependency := range declaration.Dependencies() {
		if dependency.Role() == "constraint.relation" {
			return dependency.Target().Value()
		}
	}
	return ""
}

func newKindDependencyUse(
	origin string,
	role string,
	source SourceScalar,
) (compositeDependencyUse, error) {
	target, _, _, err := schemaKindSymbol(source.Value())
	return compositeDependencyUse{
		origin: origin,
		role:   role,
		raw:    source.Value(),
		target: target,
		source: source,
	}, err
}

func newConstraintDependencyUse(
	origin string,
	role string,
	source SourceScalar,
) (compositeDependencyUse, error) {
	target, _, _, err := schemaConstraintSymbol(source.Value())
	return compositeDependencyUse{
		origin: origin,
		role:   role,
		raw:    source.Value(),
		target: target,
		source: source,
	}, err
}

func schemaKindSymbol(
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	id, err := typedmemory.NewKindID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	symbol, err := typedmemory.KindSymbolRef(id)
	return symbol, "", true, err
}

func schemaRefKindSymbol(
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	id, err := typedmemory.NewRefKindID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	symbol, err := typedmemory.RefKindSymbolRef(id)
	return symbol, "", true, err
}

func schemaEntitySetSymbol(
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	id, err := typedmemory.NewEntitySetSymbolID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	symbol, err := typedmemory.EntitySetSymbolRef(id)
	return symbol, "", true, err
}

func schemaRelationSymbol(
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	id, err := typedmemory.NewSignatureID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	symbol, err := typedmemory.RelationSymbolRef(id)
	return symbol, "", true, err
}

func schemaSlotSymbol(
	relationRaw string,
	slotRaw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	relation, err := typedmemory.NewSignatureID(relationRaw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	slot, err := typedmemory.NewSlotKindID(slotRaw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	symbol, err := typedmemory.SlotKindSymbolRef(relation, slot)
	return symbol, "", true, err
}

func schemaShapeSymbol(
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	id, err := typedmemory.NewShapeID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	symbol, err := typedmemory.ValueShapeSymbolRef(id)
	return symbol, "", true, err
}

func schemaConstraintSymbol(
	raw string,
) (typedmemory.SchemaSymbolRef, CompositeExternalReferenceKind, bool, error) {
	id, err := typedmemory.NewConstraintID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, "", false, err
	}
	symbol, err := typedmemory.ConstraintSymbolRef(id)
	return symbol, "", true, err
}

func resolveCompositeDependency(
	node compositeExtensionNode,
	use compositeDependencyUse,
	providers compositeProviderIndex,
) (CompositeDependencyResolution, *LinkIssue) {
	provider, exists := providers.byTyped[use.target.String()]
	if !exists {
		return CompositeDependencyResolution{}, unresolvedCompositeDependencyIssue(
			node,
			use,
			providers.byRaw[use.raw],
		)
	}
	if use.expectedDeclarationKind != "" {
		extensionProvider, fromExtension := provider.(ExtensionCompositeSymbolProvider)
		if !fromExtension || extensionProvider.declarationKind != use.expectedDeclarationKind {
			issue := newLinkIssue(
				IssueDependencyKindMismatch,
				compositeNodeLocation(node),
				use.target.String(),
				"dependency resolves to a declaration outside the required source variant",
				"point evaluator_requirement at a runtime_evaluator_requirement declaration",
			)
			return CompositeDependencyResolution{}, &issue
		}
	}
	scope, accessible := compositeProviderScope(node, provider)
	if !accessible {
		issue := newLinkIssue(
			IssueNonImportedSiblingDependency,
			compositeNodeLocation(node),
			use.target.String(),
			"exact dependency is provided only by a non-imported extension branch",
			"add the provider SignatureID to imports, recompile, and reseal the consumer",
		)
		return CompositeDependencyResolution{}, &issue
	}
	return CompositeDependencyResolution{
		consumerRef:        node.ref,
		consumerCoordinate: node.coordinate,
		origin:             use.origin,
		role:               use.role,
		target:             use.target,
		source:             use.source,
		scope:              scope,
		provider:           cloneCompositeProvider(provider),
	}, nil
}

func unresolvedCompositeDependencyIssue(
	node compositeExtensionNode,
	use compositeDependencyUse,
	rawProviders []CompositeSymbolProvider,
) *LinkIssue {
	code := IssueGhostDependency
	detail := "dependency has no exact provider in B, this extension, or its imports"
	repair := "declare or import the exact typed symbol, then recompile and reseal"
	if rawProvidersContainOnlyDifferentKinds(use.target, rawProviders) {
		code = IssueDependencyKindMismatch
		detail = "the raw SymbolID exists, but not with the required schema-symbol kind"
		repair = "use the source-declared symbol kind required by this dependency role"
	}
	issue := newLinkIssue(
		code,
		compositeNodeLocation(node),
		use.target.String(),
		detail,
		repair,
	)
	return &issue
}

func rawProvidersContainOnlyDifferentKinds(
	target typedmemory.SchemaSymbolRef,
	providers []CompositeSymbolProvider,
) bool {
	if len(providers) == 0 {
		return false
	}
	for _, provider := range providers {
		if provider.Symbol().Kind() == target.Kind() {
			return false
		}
	}
	return true
}

func compositeProviderScope(
	node compositeExtensionNode,
	provider CompositeSymbolProvider,
) (CompositeDependencyScope, bool) {
	switch value := provider.(type) {
	case BaseCompositeSymbolProvider:
		return CompositeDependencyBase, true
	case ExtensionCompositeSymbolProvider:
		if value.ref == node.ref {
			return CompositeDependencyOwn, true
		}
		_, imported := node.ancestors[value.ref.String()]
		return CompositeDependencyImported, imported
	default:
		return "", false
	}
}

func newCompositeExternalReference(
	node compositeExtensionNode,
	origin string,
	role string,
	kind CompositeExternalReferenceKind,
	source SourceScalar,
) CompositeExternalReference {
	return CompositeExternalReference{
		consumerRef:        node.ref,
		consumerCoordinate: node.coordinate,
		origin:             origin,
		role:               role,
		kind:               kind,
		source:             source,
	}
}

func compositeDependencySchemaIssue(
	node compositeExtensionNode,
	source SourceScalar,
	err error,
) LinkIssue {
	return newLinkIssue(
		IssueDependencySchemaInvalid,
		compositeNodeLocation(node),
		source.Value(),
		err.Error(),
		"repair and reseal the source declaration",
	)
}

func linkedCompositeExtensions(
	nodes []compositeExtensionNode,
	providers compositeProviderIndex,
) []LinkedCompositeExtension {
	result := make([]LinkedCompositeExtension, 0, len(nodes))
	for _, node := range nodes {
		ancestors := make([]typedmemory.TypeEnvExtensionRef, 0, len(node.ancestors))
		for ancestor := range node.ancestors {
			providerNodeRef, err := typedmemory.ParseTypeEnvExtensionRef(ancestor)
			if err == nil {
				ancestors = append(ancestors, providerNodeRef)
			}
		}
		sort.Slice(ancestors, func(left, right int) bool {
			return ancestors[left].String() < ancestors[right].String()
		})
		result = append(result, LinkedCompositeExtension{
			artifact:     node.artifact,
			predecessors: node.ir.Manifest().DirectPredecessors(),
			ancestors:    ancestors,
			provides:     append([]typedmemory.SchemaSymbolRef(nil), providers.byNode[node.ref.String()]...),
		})
	}
	return result
}

func compositeCoverageGaps(nodes []compositeExtensionNode) []CompositeCoverageGap {
	result := make([]CompositeCoverageGap, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, CompositeCoverageGap{
			code:       CompositeGapStratumDirectionUnresolved,
			coordinate: node.coordinate,
			ref:        node.ref,
			detail: "the exact carrier declares imports/provides but does not expose a " +
				"machine-checkable LEX stratum direction; the linker did not infer one",
		})
	}
	return result
}

func rejectedCompositeLink(issues []LinkIssue) CompositeIRLinkResolution {
	owned := cloneIssues(issues)
	sortIssues(owned)
	return rejectedCompositeIRLinkResolution{issues: owned}
}

func canonicalCompositeLinkIR(ir LinkedProjectTypeEnvCompositeIR) []byte {
	writer := compositeLinkWriter{}
	writer.addString(compositeLinkCanonicalDomain)
	writer.addString(ir.baseRef.String())
	writer.addString(ir.base.Digest().String())
	writer.addUint(uint64(len(ir.extensions)))
	for _, extension := range ir.extensions {
		writer.addString(extension.Coordinate().String())
		writer.addString(extension.Ref().String())
		writer.addUint(uint64(len(extension.predecessors)))
		for _, predecessor := range extension.predecessors {
			writer.addString(predecessor.Coordinate().String())
			writer.addString(predecessor.Ref().String())
		}
		writer.addUint(uint64(len(extension.provides)))
		for _, provided := range extension.provides {
			writer.addString(provided.String())
		}
	}
	writer.addUint(uint64(len(ir.dependencies)))
	for _, dependency := range ir.dependencies {
		writer.addString(compositeDependencyResolutionKey(dependency))
	}
	writer.addUint(uint64(len(ir.externals)))
	for _, external := range ir.externals {
		writer.addString(compositeExternalReferenceKey(external))
	}
	writer.addUint(uint64(len(ir.assumptions)))
	for _, assumption := range ir.assumptions {
		writer.addString(compositeCarrierAssumptionKey(assumption))
	}
	writer.addUint(uint64(len(ir.gaps)))
	for _, gap := range ir.gaps {
		writer.addString(string(gap.code))
		writer.addString(gap.coordinate.String())
		writer.addString(gap.ref.String())
		writer.addString(gap.detail)
	}
	return writer.bytes()
}

type compositeLinkWriter struct {
	buffer bytes.Buffer
}

func (writer *compositeLinkWriter) addString(value string) {
	writer.addUint(uint64(len(value)))
	writer.buffer.WriteString(value)
}

func (writer *compositeLinkWriter) addUint(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.buffer.Write(encoded[:])
}

func (writer compositeLinkWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

func compositeDependencyResolutionKey(resolution CompositeDependencyResolution) string {
	return resolution.consumerRef.String() + "\x00" +
		resolution.origin + "\x00" +
		resolution.role + "\x00" +
		resolution.target.String() + "\x00" +
		string(resolution.scope) + "\x00" +
		compositeProviderKey(resolution.provider) + "\x00" +
		fmt.Sprintf("%020d:%020d", resolution.source.Span().Start(), resolution.source.Span().End())
}

func compositeExternalReferenceKey(reference CompositeExternalReference) string {
	return reference.consumerRef.String() + "\x00" +
		reference.origin + "\x00" +
		reference.role + "\x00" +
		string(reference.kind) + "\x00" +
		reference.source.Value() + "\x00" +
		fmt.Sprintf("%020d:%020d", reference.source.Span().Start(), reference.source.Span().End())
}

func compositeCarrierAssumptionKey(assumption CompositeCarrierAssumption) string {
	return assumption.consumerRef.String() + "\x00" +
		assumption.consumerCoordinate.String() + "\x00" +
		assumption.origin + "\x00" +
		compositeSourceScalarKey(assumption.carrierRef) + "\x00" +
		compositeSourceScalarKey(assumption.edition) + "\x00" +
		compositeSourceScalarKey(assumption.digest)
}

func compositeSourceScalarKey(source SourceScalar) string {
	return source.Value() + "\x00" +
		fmt.Sprintf("%020d:%020d", source.Span().Start(), source.Span().End())
}

func compositeProviderKey(provider CompositeSymbolProvider) string {
	switch value := provider.(type) {
	case BaseCompositeSymbolProvider:
		return "base\x00" + value.symbol.String()
	case ExtensionCompositeSymbolProvider:
		return "extension\x00" + value.ref.String() + "\x00" + value.symbol.String()
	default:
		return "invalid"
	}
}

func cloneCompositeProvider(provider CompositeSymbolProvider) CompositeSymbolProvider {
	switch value := provider.(type) {
	case BaseCompositeSymbolProvider:
		return value
	case ExtensionCompositeSymbolProvider:
		return value
	default:
		return nil
	}
}

func cloneCompositeDependencyResolutions(
	values []CompositeDependencyResolution,
) []CompositeDependencyResolution {
	result := append([]CompositeDependencyResolution(nil), values...)
	for index := range result {
		result[index].provider = cloneCompositeProvider(result[index].provider)
	}
	return result
}

func cloneLinkedCompositeExtensions(
	values []LinkedCompositeExtension,
) []LinkedCompositeExtension {
	result := append([]LinkedCompositeExtension(nil), values...)
	for index := range result {
		result[index].predecessors = append(
			[]ResolvedExtensionPredecessor(nil),
			result[index].predecessors...,
		)
		result[index].ancestors = append(
			[]typedmemory.TypeEnvExtensionRef(nil),
			result[index].ancestors...,
		)
		result[index].provides = append(
			[]typedmemory.SchemaSymbolRef(nil),
			result[index].provides...,
		)
	}
	return result
}

func cloneLinkedProjectTypeEnvCompositeIR(
	ir LinkedProjectTypeEnvCompositeIR,
) LinkedProjectTypeEnvCompositeIR {
	ir.extensions = cloneLinkedCompositeExtensions(ir.extensions)
	ir.dependencies = cloneCompositeDependencyResolutions(ir.dependencies)
	ir.externals = append([]CompositeExternalReference(nil), ir.externals...)
	ir.assumptions = append([]CompositeCarrierAssumption(nil), ir.assumptions...)
	ir.gaps = append([]CompositeCoverageGap(nil), ir.gaps...)
	ir.canonical = append([]byte(nil), ir.canonical...)
	return ir
}

func sameStringSet(left map[string]struct{}, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if _, exists := right[value]; !exists {
			return false
		}
	}
	return true
}
