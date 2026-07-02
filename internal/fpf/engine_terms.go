package fpf

const EngineTermSheetSchemaVersion = 1

type EngineTermDefinition struct {
	Term             string   `json:"term"`
	ObjectKind       string   `json:"object_kind"`
	Definition       string   `json:"definition"`
	MustNotMean      []string `json:"must_not_mean"`
	Deprecated       bool     `json:"deprecated,omitempty"`
	ReplacementTerms []string `json:"replacement_terms,omitempty"`
}

func EngineTermDefinitions() []EngineTermDefinition {
	return cloneEngineTermDefinitions(engineTermDefinitions)
}

func EngineTermByName(term string) (EngineTermDefinition, bool) {
	for _, definition := range engineTermDefinitions {
		if definition.Term == term {
			return cloneEngineTermDefinition(definition), true
		}
	}
	return EngineTermDefinition{}, false
}

var engineTermDefinitions = []EngineTermDefinition{
	{
		Term:       "PatternSourcePack",
		ObjectKind: "versioned_pattern_source_collection",
		Definition: "Versioned source collection of pattern cards. FPF core is the first source pack; future DPF packs are additional source packs with source paths, edition, hashes, stable pattern IDs, domain scope, and currentness metadata.",
		MustNotMean: []string{
			"runtime route set",
			"evidence pack",
			"approval source",
			"MethodPack",
		},
	},
	{
		Term:       "PatternAtlas",
		ObjectKind: "deterministic_pattern_source_index",
		Definition: "Deterministic source index over PatternSourcePacks. It locates, ranges, hashes, and hydrates pattern cards or subtrees with source metadata.",
		MustNotMean: []string{
			"library of principles as authority",
			"route selector",
			"evidence source",
			"gate passage",
			"approval mechanism",
		},
	},
	{
		Term:       "PatternUse",
		ObjectKind: "umbrella_pattern_use_product_term",
		Definition: "Umbrella product term for gateway, recommendation record, compiled route selection, recall, progressive disclosure, and audit. Prefer narrower terms when one applies.",
		MustNotMean: []string{
			"work authority",
			"MethodPack",
			"DecisionRecord",
			"evidence truth",
		},
	},
	{
		Term:       "PatternUseGateway",
		ObjectKind: "pattern_use_public_entrypoint",
		Definition: "Public CLI/MCP entrypoint that maps one operator concern to a read-only advisory PatternUseRecommendation.",
		MustNotMean: []string{
			"general FPF search",
			"MethodPack pull",
			"work commission",
			"approval",
		},
	},
	{
		Term:       "PatternUseRecommendation",
		ObjectKind: "advisory_pattern_use_record",
		Definition: "Typed advisory record with candidates, recommended use when applicable, wrong boundary, output shape, evidence or SoTA need, blocked stronger use, closeout expectation, support level, and authority boundary.",
		MustNotMean: []string{
			"DecisionRecord",
			"evidence",
			"truth claim",
			"WorkPlan",
			"performed work",
		},
	},
	{
		Term:       "PatternRouteSelector",
		ObjectKind: "compiled_route_selector",
		Definition: "Intent-gated selector over compiled PatternUse route cards. It can return compiled route support only for application or use intents.",
		MustNotMean: []string{
			"keyword matcher",
			"explicit-ref authority",
			"top-k retrieval",
		},
	},
	{
		Term:       "PatternRecall",
		ObjectKind: "pattern_candidate_recall_and_hydration",
		Definition: "Retrieval plus PatternAtlas source hydration path through FPF search. It returns candidate source cards, usually with retrieved_uncompiled support.",
		MustNotMean: []string{
			"compiled support",
			"applicability proof",
			"implementation substrate",
		},
	},
	{
		Term:       "PatternUseAudit",
		ObjectKind: "read_only_pattern_use_transcript_audit",
		Definition: "Read-only transcript or session audit checking gateway-before-reasoning, progressive disclosure, output-shape following, and absence of authority overclaim.",
		MustNotMean: []string{
			"enforcement gate",
			"global proof that PatternUse works",
			"operator approval",
		},
	},
	{
		Term:       "MethodPack",
		ObjectKind: "task_local_work_evidence_harness",
		Definition: "Executable task-local work and evidence harness. It opens and closes MethodRuns, routes work methods, and asks for hard-gate evidence or waivers.",
		MustNotMean: []string{
			"FPF source pack",
			"DPF source pack",
			"PatternUse route",
			"decision authority",
			"evidence truth",
		},
	},
	{
		Term:       "DPF",
		ObjectKind: "domain_pattern_framework_source_pack",
		Definition: "Domain Pattern Framework, represented in Haft as a PatternSourcePack for domain-specific problem situations, principles, and solution moves.",
		MustNotMean: []string{
			"local repo convention",
			"MethodRun card",
			"tool workflow",
		},
	},
	{
		Term:       "LPF",
		ObjectKind: "local_project_practice_context",
		Definition: "Local project or organization practice. In Haft product architecture it remains external host-agent/project context such as AGENTS.md, repo skills, local docs, local decisions, and local specs.",
		MustNotMean: []string{
			"universal domain source pack",
			"Haft-owned public product layer",
		},
	},
	{
		Term:       "TPF",
		ObjectKind: "tool_team_task_process_framework",
		Definition: "Tool, team, or task process framework. In Haft, use MethodPack or workflow mechanics publicly; use TPF only as internal architecture shorthand.",
		MustNotMean: []string{
			"FPF source",
			"DPF normative source",
			"PatternAtlas source pack",
		},
	},
	{
		Term:       "PatternPull",
		ObjectKind: "deprecated_pattern_recall_alias",
		Definition: "Deprecated formal term. Historical or informal alias for pattern recall or source hydration only.",
		MustNotMean: []string{
			"runtime API",
			"artifact kind",
			"schema name",
			"support class",
			"MethodPack pull",
		},
		Deprecated: true,
		ReplacementTerms: []string{
			"PatternUseGateway",
			"PatternRecall",
			"source_hydration",
			"PatternRouteSelector",
		},
	},
}

func cloneEngineTermDefinitions(definitions []EngineTermDefinition) []EngineTermDefinition {
	cloned := make([]EngineTermDefinition, 0, len(definitions))
	for _, definition := range definitions {
		cloned = append(cloned, cloneEngineTermDefinition(definition))
	}
	return cloned
}

func cloneEngineTermDefinition(definition EngineTermDefinition) EngineTermDefinition {
	definition.MustNotMean = append([]string(nil), definition.MustNotMean...)
	definition.ReplacementTerms = append([]string(nil), definition.ReplacementTerms...)
	return definition
}
