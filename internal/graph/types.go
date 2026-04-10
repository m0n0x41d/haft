// Package graph provides a unified query interface over Haft's
// architectural knowledge: artifacts, codebase modules, dependencies,
// affected files, and affected symbols — all stored in the project's
// SQLite database.
//
// The graph is not a separate data structure. It is a query layer
// over existing tables (artifacts, artifact_links, codebase_modules,
// module_dependencies, affected_files, affected_symbols).
package graph

// NodeKind identifies the type of entity in the knowledge graph.
type NodeKind string

const (
	KindDecision  NodeKind = "decision"
	KindProblem   NodeKind = "problem"
	KindPortfolio NodeKind = "portfolio"
	KindModule    NodeKind = "module"
	KindFile      NodeKind = "file"
	KindSymbol    NodeKind = "symbol"
	KindEvidence  NodeKind = "evidence"
)

// Node represents any entity in the knowledge graph.
type Node struct {
	ID   string   `json:"id"`
	Kind NodeKind `json:"kind"`
	Name string   `json:"name"`
	Path string   `json:"path,omitempty"` // file path or module path
}

// Edge represents a relationship between two nodes.
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // constrains, implements, contains, depends_on
}

// Invariant is a constraint extracted from a decision record.
type Invariant struct {
	Text       string `json:"text"`
	DecisionID string `json:"decision_id"`
	DecisionTitle string `json:"decision_title"`
}

// ImpactItem describes a module affected by a change, with distance from the source.
type ImpactItem struct {
	ModuleID      string `json:"module_id"`
	ModulePath    string `json:"module_path"`
	DecisionID    string `json:"decision_id"`
	DecisionTitle string `json:"decision_title"`
	Distance      int    `json:"distance"` // hops from changed module
	IsDirect      bool   `json:"is_direct"`
}
