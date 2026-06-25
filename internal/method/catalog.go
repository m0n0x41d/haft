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

type Catalog struct {
	ID      string
	Version string
	Methods []Definition
}

func BuiltinCatalog() Catalog {
	return Catalog{
		ID:      CatalogID,
		Version: CatalogVersion,
		Methods: withBuiltinSourcePosture([]Definition{
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

func withBuiltinSourcePosture(definitions []Definition) []Definition {
	posture := SourcePosture{
		SourceKind:        MethodSourceKind,
		SourceEdition:     CatalogID + "@" + CatalogVersion,
		Normativity:       MethodSourceNormativity,
		AuthorityBoundary: MethodAuthorityBoundary,
	}
	enriched := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		definition.SourcePosture = posture
		enriched = append(enriched, definition)
	}
	return enriched
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
	}
	return nil
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

func Pull(input PullInput) (MethodRun, error) {
	catalog := BuiltinCatalog()
	if err := ValidateCatalog(catalog); err != nil {
		return MethodRun{}, err
	}

	normalized := normalizePullInput(input)
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
		Methods: cards,
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

func normalizePullInput(input PullInput) PullInput {
	input.DeclaredTaskKind = normalizeToken(input.DeclaredTaskKind)
	input.ChangeIntent = normalizeToken(input.ChangeIntent)
	input.CeremonyRequest = normalizeToken(input.CeremonyRequest)
	input.IntendedFiles = dedupeStrings(input.IntendedFiles)
	input.UserScopeConstraints = dedupeStrings(input.UserScopeConstraints)
	input.RiskSignals = normalizeRiskSignals(input.RiskSignals)
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
		verificationBeforeCompletion(),
		"fallback for unmatched non-trivial code work",
	)
	return append(cards, card)
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
		ID:               definition.ID,
		Version:          definition.Version,
		Title:            definition.Title,
		WhyApplies:       why,
		Intent:           definition.Intent,
		SourcePosture:    definition.SourcePosture,
		HardGates:        hardGates,
		SoftGates:        firstN(definition.SoftGates, 2),
		Procedure:        procedure,
		AntiPatterns:     firstN(definition.AntiPatterns, 3),
		RequiredEvidence: requiredEvidence,
		Waiver:           definition.Waiver,
		RequiredCloseout: true,
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
