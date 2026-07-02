package method

import (
	"fmt"
	"sort"
	"strings"
)

const (
	TaskMechanicalEdit = "mechanical_edit"
	TaskFormattingOnly = "formatting_only"
)

const (
	MethodSourceKind        = "methodpack_card"
	MethodSourceNormativity = "support_carrier_non_normative_fpf"
	MethodAuthorityBoundary = "method_cards_route_work_and_closeout_gates; they do not define normative FPF source material, binding decisions, evidence truth, or gate passage"
)

const (
	LifecycleExperimental = "experimental"
	LifecycleCurrent      = "current"
	LifecycleSuperseded   = "superseded"
	LifecycleDeprecated   = "deprecated"
)

const (
	CatalogReportKind          = "haft_method_catalog"
	CatalogReportSchemaVersion = 1
	MethodCatalogAuthority     = "read_only_method_catalog_not_processpattern_not_enforcement_authority"
)

var builtinMethodSourcePatternRefs = map[string][]string{
	"verification-before-completion":       {"fpf:A.10", "fpf:B.3", "fpf:A.15"},
	"graph-preflight-before-governed-edit": {"fpf:A.10", "fpf:A.15"},
	"problem-closure-hygiene":              {"fpf:E.9", "fpf:A.10", "fpf:A.15"},
	"refactor-only-under-tests":            {"fpf:A.10", "fpf:B.3"},
	"systematic-debugging-before-fix":      {"fpf:B.5.2", "fpf:A.10"},
	"behavior-first-testing":               {"fpf:A.10", "fpf:B.3"},
	"domain-port-before-adapter":           {"fpf:A.6", "fpf:A.15"},
	"functional-core-imperative-shell":     {"fpf:A.15"},
	"make-illegal-states-unrepresentable":  {"fpf:A.6", "fpf:A.17"},
}

type Catalog struct {
	ID      string
	Version string
	Methods []Definition
}

type CatalogReport struct {
	Kind              string         `json:"kind"`
	SchemaVersion     int            `json:"schema_version"`
	CatalogID         string         `json:"catalog_id"`
	CatalogVersion    string         `json:"catalog_version"`
	FilterStatus      string         `json:"filter_status"`
	AuthorityBoundary string         `json:"authority_boundary"`
	Summary           CatalogSummary `json:"summary"`
	Methods           []CatalogEntry `json:"methods"`
	Notes             []string       `json:"notes,omitempty"`
}

type CatalogSummary struct {
	Total        int            `json:"total"`
	Returned     int            `json:"returned"`
	ByLifecycle  map[string]int `json:"by_lifecycle"`
	Current      int            `json:"current"`
	Experimental int            `json:"experimental"`
	Superseded   int            `json:"superseded"`
	Deprecated   int            `json:"deprecated"`
}

type CatalogEntry struct {
	ID                  string        `json:"id"`
	Version             string        `json:"version"`
	Title               string        `json:"title"`
	Summary             string        `json:"summary"`
	ProblemContext      string        `json:"problem_context,omitempty"`
	FirstUsefulMove     string        `json:"first_useful_move,omitempty"`
	ExpectedOutputKinds []string      `json:"expected_output_kinds,omitempty"`
	FitFunctionRefs     []string      `json:"fit_function_refs,omitempty"`
	CarrierRefs         []string      `json:"carrier_refs,omitempty"`
	SourcePatternRefs   []string      `json:"source_pattern_refs,omitempty"`
	Lifecycle           Lifecycle     `json:"lifecycle"`
	SourcePosture       SourcePosture `json:"source_posture"`
}

func BuiltinCatalog() Catalog {
	return Catalog{
		ID:      CatalogID,
		Version: CatalogVersion,
		Methods: withBuiltinCatalogMetadata([]Definition{
			problemClosureHygiene(),
			graphPreflightBeforeGovernedEdit(),
			verificationBeforeCompletion(),
			systematicDebuggingBeforeFix(),
			behaviorFirstTesting(),
			refactorOnlyUnderTests(),
			domainPortBeforeAdapter(),
			functionalCoreImperativeShell(),
			makeIllegalStatesUnrepresentable(),
		}),
	}
}

func withBuiltinCatalogMetadata(definitions []Definition) []Definition {
	enriched := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		enriched = append(enriched, withDefinitionCatalogMetadata(definition))
	}
	return enriched
}

func withDefinitionCatalogMetadata(definition Definition) Definition {
	definition.SourcePosture = SourcePosture{
		SourceKind:        MethodSourceKind,
		SourceEdition:     CatalogID + "@" + CatalogVersion,
		Normativity:       MethodSourceNormativity,
		AuthorityBoundary: MethodAuthorityBoundary,
	}

	if strings.TrimSpace(definition.Lifecycle.Status) == "" {
		definition.Lifecycle.Status = LifecycleCurrent
	}
	if strings.TrimSpace(definition.Lifecycle.ValidFrom) == "" {
		definition.Lifecycle.ValidFrom = "2026-06-25"
	}
	if strings.TrimSpace(definition.ProblemContext) == "" {
		definition.ProblemContext = "Non-trivial software engineering work where a task-local MethodPack card can reduce omissions."
	}
	if strings.TrimSpace(definition.FirstUsefulMove) == "" && len(definition.Procedure) > 0 {
		definition.FirstUsefulMove = definition.Procedure[0]
	}
	if len(definition.ExpectedOutputKinds) == 0 {
		definition.ExpectedOutputKinds = []string{"method_run_closeout", "verification_evidence"}
	}
	if len(definition.FitFunctionRefs) == 0 {
		definition.FitFunctionRefs = []string{"process_check:method_run_hard_gates"}
	}
	if len(definition.CarrierRefs) == 0 {
		definition.CarrierRefs = []string{
			"internal/method/builtin.go",
			".haft/methods/" + CatalogID + "/" + definition.ID + ".yaml",
		}
	}
	if len(definition.SourcePatternRefs) == 0 {
		definition.SourcePatternRefs = sourcePatternRefsForMethod(definition.ID)
	}

	return definition
}

func sourcePatternRefsForMethod(methodID string) []string {
	return append([]string(nil), builtinMethodSourcePatternRefs[methodID]...)
}

func ValidateCatalog(catalog Catalog) error {
	seen := map[string]bool{}
	for _, definition := range catalog.Methods {
		if strings.TrimSpace(definition.ID) == "" {
			return fmt.Errorf("method id is required")
		}
		if seen[definition.ID] {
			return fmt.Errorf("duplicate method id %q", definition.ID)
		}
		seen[definition.ID] = true
		if strings.TrimSpace(definition.Title) == "" {
			return fmt.Errorf("method %s title is required", definition.ID)
		}
		if err := validateSourcePosture(definition); err != nil {
			return err
		}
		if err := validateSourcePatternRefs(definition); err != nil {
			return err
		}
		if len(definition.HardGates) == 0 {
			return fmt.Errorf("method %s needs at least one hard gate", definition.ID)
		}
		for _, gate := range definition.HardGates {
			if strings.TrimSpace(gate.ID) == "" {
				return fmt.Errorf("method %s has hard gate without id", definition.ID)
			}
			if strings.TrimSpace(gate.Kind) == "" {
				return fmt.Errorf("method %s gate %s needs gate_kind", definition.ID, gate.ID)
			}
			if strings.TrimSpace(gate.CheckLevel) == "" {
				return fmt.Errorf("method %s gate %s needs check_level", definition.ID, gate.ID)
			}
		}
		if err := validateLifecycle(definition); err != nil {
			return err
		}
	}
	return nil
}

func validateSourcePatternRefs(definition Definition) error {
	for _, ref := range definition.SourcePatternRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return fmt.Errorf("method %s has empty source_pattern_refs item", definition.ID)
		}
		if !validSourcePatternRefPrefix(ref) {
			return fmt.Errorf("method %s source_pattern_refs item %q must start with fpf:, swe-dpf:, or methodpack:", definition.ID, ref)
		}
	}
	return nil
}

func validSourcePatternRefPrefix(ref string) bool {
	return strings.HasPrefix(ref, "fpf:") ||
		strings.HasPrefix(ref, "swe-dpf:") ||
		strings.HasPrefix(ref, "methodpack:")
}

func validateSourcePosture(definition Definition) error {
	if strings.TrimSpace(definition.SourcePosture.SourceKind) != MethodSourceKind {
		return fmt.Errorf("method %s source_posture.source_kind must be %q", definition.ID, MethodSourceKind)
	}
	if strings.TrimSpace(definition.SourcePosture.Normativity) != MethodSourceNormativity {
		return fmt.Errorf("method %s source_posture.normativity must be %q", definition.ID, MethodSourceNormativity)
	}
	if strings.TrimSpace(definition.SourcePosture.SourceEdition) == "" {
		return fmt.Errorf("method %s source_posture.source_edition is required", definition.ID)
	}
	if strings.TrimSpace(definition.SourcePosture.AuthorityBoundary) == "" {
		return fmt.Errorf("method %s source_posture.authority_boundary is required", definition.ID)
	}
	return nil
}

func validateLifecycle(definition Definition) error {
	status := normalizeToken(definition.Lifecycle.Status)
	switch status {
	case LifecycleExperimental, LifecycleCurrent, LifecycleSuperseded, LifecycleDeprecated:
	default:
		return fmt.Errorf("method %s lifecycle.status must be experimental, current, superseded, or deprecated", definition.ID)
	}
	if status == LifecycleCurrent && strings.TrimSpace(definition.Lifecycle.RetirementReason) != "" {
		return fmt.Errorf("method %s current lifecycle cannot carry retirement_reason", definition.ID)
	}
	if status == LifecycleSuperseded && len(definition.Lifecycle.SuccessorRefs) == 0 {
		return fmt.Errorf("method %s superseded lifecycle needs successor_refs", definition.ID)
	}
	if status == LifecycleDeprecated && strings.TrimSpace(definition.Lifecycle.RetirementReason) == "" {
		return fmt.Errorf("method %s deprecated lifecycle needs retirement_reason", definition.ID)
	}
	return nil
}

func Pull(input PullInput) (MethodRun, error) {
	catalog := BuiltinCatalog()
	if err := ValidateCatalog(catalog); err != nil {
		return MethodRun{}, err
	}

	normalized := normalizePullInput(input)
	if err := validatePullCarryThrough(normalized.CarryThrough); err != nil {
		return MethodRun{}, err
	}
	ceremony, reason := ceremonyFor(normalized)
	if ceremony == "low" || ceremony == "none" {
		return MethodRun{
			CatalogID:      catalog.ID,
			CatalogVersion: catalog.Version,
			Status:         "open",
			TaskSignature: TaskSignature{
				Task:                 strings.TrimSpace(input.Task),
				NormalizedTaskKind:   normalized.DeclaredTaskKind,
				ChangeIntent:         normalized.ChangeIntent,
				IntendedFiles:        normalized.IntendedFiles,
				RiskSignals:          normalized.RiskSignals,
				UserScopeConstraints: normalized.UserScopeConstraints,
				Ceremony:             ceremony,
				CeremonyReason:       reason,
			},
			DeterministicContext: ContextSnapshot{},
			CarryThrough:         normalized.CarryThrough,
		}, nil
	}

	cards := matchCards(catalog.Methods, normalized)
	cards = withFallbackVerificationCard(cards)
	maxMethods := normalized.ResponseBudget.MaxMethods
	if maxMethods <= 0 || maxMethods > 3 {
		maxMethods = 3
	}
	if len(cards) > maxMethods {
		cards = cards[:maxMethods]
	}

	return MethodRun{
		CatalogID:      catalog.ID,
		CatalogVersion: catalog.Version,
		Status:         "open",
		TaskSignature: TaskSignature{
			Task:                 strings.TrimSpace(input.Task),
			NormalizedTaskKind:   normalized.DeclaredTaskKind,
			ChangeIntent:         normalized.ChangeIntent,
			IntendedFiles:        normalized.IntendedFiles,
			RiskSignals:          normalized.RiskSignals,
			UserScopeConstraints: normalized.UserScopeConstraints,
			Ceremony:             ceremony,
			CeremonyReason:       reason,
		},
		DeterministicContext: ContextSnapshot{
			PathPolicyMatches: pathPolicyMatches(normalized.IntendedFiles),
		},
		Methods:      cards,
		CarryThrough: normalized.CarryThrough,
	}, nil
}

func Detail(methodID string) (Definition, error) {
	for _, definition := range BuiltinCatalog().Methods {
		if definition.ID == methodID {
			return definition, nil
		}
	}
	return Definition{}, fmt.Errorf("unknown method %q", methodID)
}

func DiscoverCatalog(status string) (CatalogReport, error) {
	catalog := BuiltinCatalog()
	if err := ValidateCatalog(catalog); err != nil {
		return CatalogReport{}, err
	}

	filterStatus := normalizeCatalogStatus(status)
	if !catalogStatusSupported(filterStatus) {
		return CatalogReport{}, fmt.Errorf("unsupported method catalog status %q; supported: current, experimental, superseded, deprecated, all", status)
	}

	entries := make([]CatalogEntry, 0, len(catalog.Methods))
	for _, definition := range catalog.Methods {
		if !catalogStatusMatches(definition, filterStatus) {
			continue
		}
		entries = append(entries, catalogEntry(definition))
	}

	return CatalogReport{
		Kind:              CatalogReportKind,
		SchemaVersion:     CatalogReportSchemaVersion,
		CatalogID:         catalog.ID,
		CatalogVersion:    catalog.Version,
		FilterStatus:      filterStatus,
		AuthorityBoundary: MethodCatalogAuthority,
		Summary:           catalogSummary(catalog.Methods, len(entries)),
		Methods:           entries,
		Notes: []string{
			"MethodPack catalog discovery is read-only and does not create ProcessPattern authority.",
			"Current methods are eligible for pull matching; superseded/deprecated methods are history and detail-only.",
		},
	}, nil
}

func normalizePullInput(input PullInput) PullInput {
	input.DeclaredTaskKind = normalizeToken(input.DeclaredTaskKind)
	input.ChangeIntent = normalizeToken(input.ChangeIntent)
	input.CeremonyRequest = normalizeToken(input.CeremonyRequest)
	input.IntendedFiles = dedupeStrings(input.IntendedFiles)
	input.UserScopeConstraints = dedupeStrings(input.UserScopeConstraints)
	input.RiskSignals = normalizeRiskSignals(input.RiskSignals)
	input.CarryThrough = normalizeCarryThroughItems(input.CarryThrough, true)
	return input
}

func ceremonyFor(input PullInput) (string, string) {
	mechanical := isMechanicalPull(input)
	if mechanical && (input.CeremonyRequest == "none" || input.CeremonyRequest == "low") {
		return input.CeremonyRequest, "agent requested low ceremony for mechanical edit"
	}
	if mechanical {
		return "low", "mechanical edit"
	}
	if hasAnyRiskSignal(input, "external_io", "domain_boundary", "persistence", "public_api", "governed_file", "failing_test") {
		return "medium", "risk signals require method gates"
	}
	if input.CeremonyRequest == "none" || input.CeremonyRequest == "low" {
		return "medium", "low ceremony request ignored for non-mechanical code work"
	}
	if input.DeclaredTaskKind == "architecture" {
		return "deep", "architecture task"
	}
	return "medium", "non-trivial code work"
}

func isMechanicalPull(input PullInput) bool {
	if input.DeclaredTaskKind == TaskMechanicalEdit || input.DeclaredTaskKind == TaskFormattingOnly {
		return true
	}
	return input.ChangeIntent == TaskMechanicalEdit || input.ChangeIntent == TaskFormattingOnly
}

func matchCards(definitions []Definition, input PullInput) []MethodCard {
	var selected []Definition
	for _, definition := range definitions {
		if !methodSelectable(definition) {
			continue
		}
		if methodExcluded(definition, input) {
			continue
		}
		if !methodApplies(definition, input) {
			continue
		}
		selected = append(selected, definition)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Priority < selected[j].Priority
	})

	cards := make([]MethodCard, 0, len(selected))
	for _, definition := range selected {
		cards = append(cards, compactCard(definition, whyApplies(definition, input)))
	}
	return cards
}

func withFallbackVerificationCard(cards []MethodCard) []MethodCard {
	if len(cards) > 0 {
		return cards
	}
	card := compactCard(
		withDefinitionCatalogMetadata(verificationBeforeCompletion()),
		"fallback for unmatched non-trivial code work",
	)
	return append(cards, card)
}

func methodSelectable(definition Definition) bool {
	return normalizeToken(definition.Lifecycle.Status) == LifecycleCurrent
}

func methodExcluded(definition Definition, input PullInput) bool {
	return applicabilityMatches(definition.DoesNotApplyTo, input)
}

func methodApplies(definition Definition, input PullInput) bool {
	if definition.ID == "problem-closure-hygiene" && strings.TrimSpace(input.ArtifactRefs.ProblemRef) != "" {
		return true
	}
	return applicabilityMatches(definition.AppliesTo, input)
}

func applicabilityMatches(app Applicability, input PullInput) bool {
	if contains(app.TaskKinds, input.DeclaredTaskKind) {
		return true
	}
	if contains(app.ChangeIntents, input.ChangeIntent) {
		return true
	}
	for _, signal := range input.RiskSignals {
		if contains(app.RiskSignals, signal.ID) {
			return true
		}
	}
	for _, file := range input.IntendedFiles {
		for _, fragment := range app.PathContains {
			if strings.Contains(strings.ToLower(file), strings.ToLower(fragment)) {
				return true
			}
		}
	}
	return false
}

func compactCard(definition Definition, why string) MethodCard {
	hardGates := append([]Gate(nil), definition.HardGates...)
	if len(hardGates) > 3 {
		hardGates = hardGates[:3]
	}
	procedure := append([]string(nil), definition.Procedure...)
	if len(procedure) > 3 {
		procedure = procedure[:3]
	}
	requiredEvidence := append([]string(nil), definition.RequiredEvidence...)
	if len(requiredEvidence) > 3 {
		requiredEvidence = requiredEvidence[:3]
	}

	return MethodCard{
		ID:                definition.ID,
		Version:           definition.Version,
		Title:             definition.Title,
		WhyApplies:        why,
		Intent:            definition.Intent,
		Lifecycle:         definition.Lifecycle,
		SourcePosture:     definition.SourcePosture,
		SourcePatternRefs: append([]string(nil), definition.SourcePatternRefs...),
		HardGates:         hardGates,
		SoftGates:         firstN(definition.SoftGates, 2),
		Procedure:         procedure,
		AntiPatterns:      firstN(definition.AntiPatterns, 3),
		RequiredEvidence:  requiredEvidence,
		Waiver:            definition.Waiver,
		RequiredCloseout:  true,
	}
}

func whyApplies(definition Definition, input PullInput) string {
	var reasons []string
	if contains(definition.AppliesTo.TaskKinds, input.DeclaredTaskKind) && input.DeclaredTaskKind != "" {
		reasons = append(reasons, "task_kind="+input.DeclaredTaskKind)
	}
	if contains(definition.AppliesTo.ChangeIntents, input.ChangeIntent) && input.ChangeIntent != "" {
		reasons = append(reasons, "change_intent="+input.ChangeIntent)
	}
	for _, signal := range input.RiskSignals {
		if contains(definition.AppliesTo.RiskSignals, signal.ID) {
			reasons = append(reasons, "risk="+signal.ID)
		}
	}
	for _, file := range input.IntendedFiles {
		for _, fragment := range definition.AppliesTo.PathContains {
			if strings.Contains(strings.ToLower(file), strings.ToLower(fragment)) {
				reasons = append(reasons, "path contains "+fragment)
			}
		}
	}
	if len(reasons) == 0 {
		return "matched declared work shape"
	}
	return strings.Join(dedupeStrings(reasons), " + ")
}

func normalizeCatalogStatus(status string) string {
	status = normalizeToken(status)
	if status == "" {
		return LifecycleCurrent
	}
	return status
}

func catalogStatusSupported(status string) bool {
	switch status {
	case LifecycleCurrent, LifecycleExperimental, LifecycleSuperseded, LifecycleDeprecated, "all":
		return true
	default:
		return false
	}
}

func catalogStatusMatches(definition Definition, status string) bool {
	if status == "all" {
		return true
	}
	return normalizeToken(definition.Lifecycle.Status) == status
}

func catalogEntry(definition Definition) CatalogEntry {
	return CatalogEntry{
		ID:                  definition.ID,
		Version:             definition.Version,
		Title:               definition.Title,
		Summary:             definition.Summary,
		ProblemContext:      definition.ProblemContext,
		FirstUsefulMove:     definition.FirstUsefulMove,
		ExpectedOutputKinds: append([]string(nil), definition.ExpectedOutputKinds...),
		FitFunctionRefs:     append([]string(nil), definition.FitFunctionRefs...),
		CarrierRefs:         append([]string(nil), definition.CarrierRefs...),
		SourcePatternRefs:   append([]string(nil), definition.SourcePatternRefs...),
		Lifecycle:           definition.Lifecycle,
		SourcePosture:       definition.SourcePosture,
	}
}

func catalogSummary(definitions []Definition, returned int) CatalogSummary {
	counts := map[string]int{
		LifecycleExperimental: 0,
		LifecycleCurrent:      0,
		LifecycleSuperseded:   0,
		LifecycleDeprecated:   0,
	}
	for _, definition := range definitions {
		status := normalizeCatalogStatus(definition.Lifecycle.Status)
		counts[status]++
	}
	return CatalogSummary{
		Total:        len(definitions),
		Returned:     returned,
		ByLifecycle:  counts,
		Current:      counts[LifecycleCurrent],
		Experimental: counts[LifecycleExperimental],
		Superseded:   counts[LifecycleSuperseded],
		Deprecated:   counts[LifecycleDeprecated],
	}
}

func pathPolicyMatches(files []string) []string {
	var matches []string
	for _, file := range files {
		lowered := strings.ToLower(file)
		switch {
		case strings.Contains(lowered, "internal/mcp"):
			matches = append(matches, file+":mcp")
		case strings.Contains(lowered, "internal/cli"):
			matches = append(matches, file+":cli")
		case strings.Contains(lowered, "db/"):
			matches = append(matches, file+":db")
		}
	}
	return matches
}

func normalizeRiskSignals(signals []RiskSignal) []RiskSignal {
	seen := map[string]bool{}
	var out []RiskSignal
	for _, signal := range signals {
		signal.ID = normalizeToken(signal.ID)
		if signal.ID == "" || seen[signal.ID] {
			continue
		}
		seen[signal.ID] = true
		out = append(out, signal)
	}
	return out
}

func normalizeToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func hasAnyRiskSignal(input PullInput, ids ...string) bool {
	for _, signal := range input.RiskSignals {
		if contains(ids, signal.ID) {
			return true
		}
	}
	return false
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if normalizeToken(value) == needle && needle != "" {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func firstN(values []string, max int) []string {
	values = append([]string(nil), values...)
	if len(values) > max {
		return values[:max]
	}
	return values
}
