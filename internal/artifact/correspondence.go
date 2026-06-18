package artifact

import (
	"fmt"
	"strings"
	"time"
)

const (
	CorrespondenceGraphSchemaVersion = 1
	CorrespondenceGraphRecordKind    = "qualified_correspondence_graph"
	CorrespondenceGraphAuthority     = "derived_correspondence_projection"

	CorrespondencePathNotProof = "graph_path_not_proof"

	CorrespondenceBoundaryNotProof        = "not_proof"
	CorrespondenceBoundaryNotEvidence     = "not_evidence"
	CorrespondenceBoundaryNotApproval     = "not_approval"
	CorrespondenceBoundaryNotGateDecision = "not_gate_decision"
	CorrespondenceBoundaryNotGlobalTruth  = "not_global_truth"

	CorrespondenceGapMissingTransformation = "missing_transformation_record"
	CorrespondenceGapMissingAffectedFiles  = "missing_code_correspondence_refs"
	CorrespondenceGapMissingEvidence       = "missing_observation_or_evidence_refs"
	CorrespondenceGapUnboundEvidence       = "evidence_without_claim_binding"
)

type CorrespondenceGraphInput struct {
	DecisionRef string
}

type QualifiedCorrespondenceGraph struct {
	SchemaVersion       int                         `json:"schema_version"`
	RecordKind          string                      `json:"record_kind"`
	Authority           string                      `json:"authority"`
	GraphRef            string                      `json:"graph_ref"`
	DecisionRef         string                      `json:"decision_ref"`
	PathStatus          string                      `json:"path_status"`
	ExpectedRealization []CorrespondenceNode        `json:"expected_realization,omitempty"`
	ObservedRealization []CorrespondenceNode        `json:"observed_realization,omitempty"`
	Edges               []CorrespondenceEdge        `json:"edges,omitempty"`
	Gaps                []CorrespondenceGap         `json:"gaps,omitempty"`
	AuthorityBoundary   CorrespondenceGraphBoundary `json:"authority_boundary"`
	DerivedAt           string                      `json:"derived_at"`
}

type CorrespondenceNode struct {
	Ref    string `json:"ref"`
	Kind   string `json:"kind"`
	Role   string `json:"role"`
	Label  string `json:"label,omitempty"`
	Origin string `json:"origin"`
}

type CorrespondenceEdge struct {
	RelationKind string   `json:"relation_kind"`
	FromRef      string   `json:"from_ref"`
	ToRef        string   `json:"to_ref"`
	Origin       string   `json:"origin"`
	SourceRefs   []string `json:"source_refs,omitempty"`
	PathStatus   string   `json:"path_status"`
}

type CorrespondenceGap struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type CorrespondenceGraphBoundary struct {
	Proof        string `json:"proof"`
	Evidence     string `json:"evidence"`
	Approval     string `json:"approval"`
	GateDecision string `json:"gate_decision"`
	GlobalTruth  string `json:"global_truth"`
}

func BuildQualifiedCorrespondenceGraph(
	input CorrespondenceGraphInput,
	decision *Artifact,
	affectedFiles []AffectedFile,
	evidence []EvidenceItem,
	now time.Time,
) (QualifiedCorrespondenceGraph, error) {
	decisionRef := strings.TrimSpace(input.DecisionRef)
	if decision == nil {
		return QualifiedCorrespondenceGraph{}, fmt.Errorf("decision artifact is required")
	}
	if decision.Meta.Kind != KindDecisionRecord {
		return QualifiedCorrespondenceGraph{}, fmt.Errorf("%s is %s, not DecisionRecord", decision.Meta.ID, decision.Meta.Kind)
	}
	if decisionRef == "" {
		decisionRef = decision.Meta.ID
	}
	if decisionRef != decision.Meta.ID {
		return QualifiedCorrespondenceGraph{}, fmt.Errorf("decision_ref %q does not match artifact %q", decisionRef, decision.Meta.ID)
	}

	fields := decision.UnmarshalDecisionFields()
	expected := correspondenceExpectedNodes(decisionRef, fields)
	observed := correspondenceObservedNodes(affectedFiles, evidence)
	edges := correspondenceEdges(decisionRef, fields, affectedFiles, evidence)

	return QualifiedCorrespondenceGraph{
		SchemaVersion:       CorrespondenceGraphSchemaVersion,
		RecordKind:          CorrespondenceGraphRecordKind,
		Authority:           CorrespondenceGraphAuthority,
		GraphRef:            "correspondence:" + decisionRef,
		DecisionRef:         decisionRef,
		PathStatus:          CorrespondencePathNotProof,
		ExpectedRealization: expected,
		ObservedRealization: observed,
		Edges:               edges,
		Gaps:                correspondenceGaps(fields, affectedFiles, evidence),
		AuthorityBoundary: CorrespondenceGraphBoundary{
			Proof:        CorrespondenceBoundaryNotProof,
			Evidence:     CorrespondenceBoundaryNotEvidence,
			Approval:     CorrespondenceBoundaryNotApproval,
			GateDecision: CorrespondenceBoundaryNotGateDecision,
			GlobalTruth:  CorrespondenceBoundaryNotGlobalTruth,
		},
		DerivedAt: correspondenceDerivedAt(now),
	}, nil
}

func correspondenceExpectedNodes(decisionRef string, fields DecisionFields) []CorrespondenceNode {
	nodes := []CorrespondenceNode{{
		Ref:    decisionRef,
		Kind:   "decision_speech_act",
		Role:   "expected",
		Label:  fields.SelectedTitle,
		Origin: "declared",
	}}

	if transformation := NormalizeTransformationRecord(fields.TransformationRecord); transformation != nil {
		nodes = append(nodes, CorrespondenceNode{
			Ref:    decisionRef + "#transformation_record",
			Kind:   "transformation",
			Role:   "expected",
			Label:  transformation.TransformedEntity,
			Origin: "declared",
		})
	}

	for _, claim := range normalizeDecisionClaims(fields.Claims) {
		nodes = append(nodes, CorrespondenceNode{
			Ref:    claim.ID,
			Kind:   "claim",
			Role:   "expected",
			Label:  claim.Claim,
			Origin: "declared",
		})
	}

	return nodes
}

func correspondenceObservedNodes(
	affectedFiles []AffectedFile,
	evidence []EvidenceItem,
) []CorrespondenceNode {
	nodes := make([]CorrespondenceNode, 0, len(affectedFiles)+len(evidence))
	for _, file := range affectedFiles {
		path := strings.TrimSpace(file.Path)
		if path == "" {
			continue
		}
		nodes = append(nodes, CorrespondenceNode{
			Ref:    "file:" + path,
			Kind:   "code_entity",
			Role:   "observed",
			Label:  path,
			Origin: "declared",
		})
	}
	for _, item := range evidence {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		nodes = append(nodes, CorrespondenceNode{
			Ref:    id,
			Kind:   "evidence_item",
			Role:   "observed",
			Label:  strings.TrimSpace(item.Type),
			Origin: "declared",
		})
	}

	return nodes
}

func correspondenceEdges(
	decisionRef string,
	fields DecisionFields,
	affectedFiles []AffectedFile,
	evidence []EvidenceItem,
) []CorrespondenceEdge {
	edges := []CorrespondenceEdge{}
	transformationRef := ""
	if NormalizeTransformationRecord(fields.TransformationRecord) != nil {
		transformationRef = decisionRef + "#transformation_record"
		edges = append(edges, CorrespondenceEdge{
			RelationKind: "DecisionRationale--justifies-->Transformation",
			FromRef:      decisionRef,
			ToRef:        transformationRef,
			Origin:       "declared",
			SourceRefs:   []string{decisionRef},
			PathStatus:   CorrespondencePathNotProof,
		})
	}

	for _, claim := range normalizeDecisionClaims(fields.Claims) {
		targetRef := decisionRef
		if transformationRef != "" {
			targetRef = transformationRef
		}
		edges = append(edges, CorrespondenceEdge{
			RelationKind: "Claim--constrains-->TransformationOrDecision",
			FromRef:      claim.ID,
			ToRef:        targetRef,
			Origin:       "declared",
			SourceRefs:   []string{decisionRef},
			PathStatus:   CorrespondencePathNotProof,
		})
	}

	for _, file := range affectedFiles {
		path := strings.TrimSpace(file.Path)
		if path == "" || transformationRef == "" {
			continue
		}
		edges = append(edges, CorrespondenceEdge{
			RelationKind: "CodeEntity--claimedToRealize-->Transformation",
			FromRef:      "file:" + path,
			ToRef:        transformationRef,
			Origin:       "declared",
			SourceRefs:   []string{decisionRef},
			PathStatus:   CorrespondencePathNotProof,
		})
	}

	for _, item := range evidence {
		edges = append(edges, correspondenceEvidenceEdges(decisionRef, item)...)
	}

	return edges
}

func correspondenceEvidenceEdges(decisionRef string, item EvidenceItem) []CorrespondenceEdge {
	edges := []CorrespondenceEdge{}
	for _, claimRef := range compactStrings(item.ClaimRefs) {
		edges = append(edges, CorrespondenceEdge{
			RelationKind: "Observation--supportsViaEvidencePath-->Claim",
			FromRef:      item.ID,
			ToRef:        claimRef,
			Origin:       "declared",
			SourceRefs:   []string{decisionRef, item.ID},
			PathStatus:   CorrespondencePathNotProof,
		})
	}

	return edges
}

func correspondenceGaps(
	fields DecisionFields,
	affectedFiles []AffectedFile,
	evidence []EvidenceItem,
) []CorrespondenceGap {
	gaps := []CorrespondenceGap{}
	if NormalizeTransformationRecord(fields.TransformationRecord) == nil {
		gaps = append(gaps, CorrespondenceGap{
			Kind:   CorrespondenceGapMissingTransformation,
			Detail: "No explicit TransformationRecord is available for this DecisionRecord.",
		})
	}
	if len(correspondenceObservedNodes(affectedFiles, nil)) == 0 {
		gaps = append(gaps, CorrespondenceGap{
			Kind:   CorrespondenceGapMissingAffectedFiles,
			Detail: "No affected_files/code refs are available for correspondence projection.",
		})
	}
	if len(evidence) == 0 {
		gaps = append(gaps, CorrespondenceGap{
			Kind:   CorrespondenceGapMissingEvidence,
			Detail: "No evidence items are attached to the DecisionRecord.",
		})
	}
	for _, item := range evidence {
		if len(compactStrings(item.ClaimRefs)) != 0 {
			continue
		}
		gaps = append(gaps, CorrespondenceGap{
			Kind:   CorrespondenceGapUnboundEvidence,
			Detail: "Evidence item " + item.ID + " has no claim_refs binding.",
		})
	}

	return gaps
}

func correspondenceDerivedAt(now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return now.UTC().Format(time.RFC3339)
}
