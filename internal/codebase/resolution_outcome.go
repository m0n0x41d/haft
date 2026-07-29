package codebase

type ResolutionStatus string

const (
	ResolutionResolved   ResolutionStatus = "resolved"
	ResolutionAmbiguous  ResolutionStatus = "ambiguous"
	ResolutionUnresolved ResolutionStatus = "unresolved"
)

type ResolutionReason string

const (
	ResolutionReasonNoCandidate        ResolutionReason = "no_candidate"
	ResolutionReasonMultipleCandidates ResolutionReason = "multiple_candidates"
	ResolutionReasonExternalDependency ResolutionReason = "external_dependency"
	ResolutionReasonUnsupportedForm    ResolutionReason = "unsupported_form"
	ResolutionReasonShadowedBinding    ResolutionReason = "shadowed_binding"
)

type EdgeOrigin string

const (
	EdgeOriginASTCall              EdgeOrigin = "ast_call"
	EdgeOriginASTNew               EdgeOrigin = "ast_new"
	EdgeOriginASTValueReference    EdgeOrigin = "ast_value_reference"
	EdgeOriginASTTypeReference     EdgeOrigin = "ast_type_reference"
	EdgeOriginVueTemplate          EdgeOrigin = "vue_template"
	EdgeOriginNamedImport          EdgeOrigin = "named_import"
	EdgeOriginNamespaceImport      EdgeOrigin = "namespace_import"
	EdgeOriginReceiverType         EdgeOrigin = "receiver_type"
	EdgeOriginHeritage             EdgeOrigin = "heritage"
	EdgeOriginCallbackRegistration EdgeOrigin = "callback_registration"
	EdgeOriginEmitterPair          EdgeOrigin = "emitter_pair"
	EdgeOriginLegacyStatic         EdgeOrigin = "legacy_static"
	EdgeOriginHeuristicSynthesis   EdgeOrigin = "heuristic_synthesis"
)

type ResolutionMethod string

const (
	ResolutionMethodExactSymbol ResolutionMethod = "exact_symbol"
	ResolutionMethodImportMap   ResolutionMethod = "import_map"
	ResolutionMethodTypeFacts   ResolutionMethod = "type_facts"
	ResolutionMethodHeuristic   ResolutionMethod = "heuristic"
	ResolutionMethodLegacy      ResolutionMethod = "legacy"
)

type ConfidenceClass string

const (
	ConfidenceExact ConfidenceClass = "exact"
	ConfidenceHigh  ConfidenceClass = "high"
	ConfidenceLow   ConfidenceClass = "low"
)

// EdgeResolution is a closed outcome family: a resolver must say resolved,
// ambiguous, or unresolved. Only ResolvedEdge can be admitted to traversal.
type EdgeResolution interface {
	edgeResolutionStatus() ResolutionStatus
}

type ResolvedEdge struct {
	Edge CodeEdge
}

func (ResolvedEdge) edgeResolutionStatus() ResolutionStatus { return ResolutionResolved }

type AmbiguousEdge struct {
	SourceID           string
	Kind               EdgeKind
	FilePath           string
	Line               int
	Reason             ResolutionReason
	CandidateIDs       []string
	Origin             EdgeOrigin
	ResolverVersion    string
	SourceSnapshotHash string
}

func (AmbiguousEdge) edgeResolutionStatus() ResolutionStatus { return ResolutionAmbiguous }

type UnresolvedEdge struct {
	SourceID           string
	Kind               EdgeKind
	FilePath           string
	Line               int
	Reason             ResolutionReason
	Origin             EdgeOrigin
	ResolverVersion    string
	SourceSnapshotHash string
}

func (UnresolvedEdge) edgeResolutionStatus() ResolutionStatus { return ResolutionUnresolved }

type ResolutionDiagnostic struct {
	SourceID           string
	Kind               EdgeKind
	FilePath           string
	Line               int
	Status             ResolutionStatus
	Reason             ResolutionReason
	CandidateIDs       []string
	Origin             EdgeOrigin
	ResolverVersion    string
	SourceSnapshotHash string
}

// PartitionEdgeResolutions is the pure admission boundary. Resolved edges flow
// into code traversal; every non-resolved outcome becomes an inspectable
// diagnostic and is inexpressible as a traversal edge.
func PartitionEdgeResolutions(outcomes []EdgeResolution) ([]CodeEdge, []ResolutionDiagnostic) {
	edges := make([]CodeEdge, 0)
	diagnostics := make([]ResolutionDiagnostic, 0)
	for _, outcome := range outcomes {
		switch resolved := outcome.(type) {
		case ResolvedEdge:
			edges = append(edges, normalizeCodeEdge(resolved.Edge))
		case AmbiguousEdge:
			diagnostics = append(diagnostics, ResolutionDiagnostic{
				SourceID:           resolved.SourceID,
				Kind:               resolved.Kind,
				FilePath:           resolved.FilePath,
				Line:               resolved.Line,
				Status:             ResolutionAmbiguous,
				Reason:             resolved.Reason,
				CandidateIDs:       append([]string(nil), resolved.CandidateIDs...),
				Origin:             resolved.Origin,
				ResolverVersion:    resolved.ResolverVersion,
				SourceSnapshotHash: resolved.SourceSnapshotHash,
			})
		case UnresolvedEdge:
			diagnostics = append(diagnostics, ResolutionDiagnostic{
				SourceID:           resolved.SourceID,
				Kind:               resolved.Kind,
				FilePath:           resolved.FilePath,
				Line:               resolved.Line,
				Status:             ResolutionUnresolved,
				Reason:             resolved.Reason,
				Origin:             resolved.Origin,
				ResolverVersion:    resolved.ResolverVersion,
				SourceSnapshotHash: resolved.SourceSnapshotHash,
			})
		}
	}
	return edges, diagnostics
}

func normalizeCodeEdge(edge CodeEdge) CodeEdge {
	if edge.Origin == "" {
		edge.Origin = legacyEdgeOrigin(edge.Provenance)
	}
	if edge.ResolutionMethod == "" {
		edge.ResolutionMethod = legacyResolutionMethod(edge.Provenance)
	}
	if edge.Confidence == "" {
		edge.Confidence = legacyConfidence(edge.Provenance)
	}
	if edge.ResolverVersion == "" {
		edge.ResolverVersion = "legacy-v1"
	}
	return edge
}

func legacyEdgeOrigin(provenance Provenance) EdgeOrigin {
	if provenance == ProvenanceHeuristic {
		return EdgeOriginHeuristicSynthesis
	}
	return EdgeOriginLegacyStatic
}

func legacyResolutionMethod(provenance Provenance) ResolutionMethod {
	if provenance == ProvenanceHeuristic {
		return ResolutionMethodHeuristic
	}
	return ResolutionMethodLegacy
}

func legacyConfidence(provenance Provenance) ConfidenceClass {
	if provenance == ProvenanceHeuristic {
		return ConfidenceLow
	}
	return ConfidenceHigh
}
