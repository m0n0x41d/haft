package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/codebase"
)

const (
	BindingTargetSymbol            = "symbol"
	BindingTargetRange             = "range"
	BindingTargetModule            = "module"
	BindingTargetGenerated         = "generated"
	BindingTargetCarrier           = "carrier"
	BindingTargetWholeFileFallback = "whole_file_fallback"
	BindingTargetSpecSection       = "spec_section"
	BindingTargetAPIContract       = "api_contract"
	BindingTargetInvariant         = "invariant"

	BindingScopeAuto      = "auto"
	BindingScopeModule    = "module"
	BindingScopeWholeFile = "whole_file"

	BindingResolutionSourceExplicitTargets       = "explicit_targets"
	BindingResolutionSourcePathClassification    = "path_classification"
	BindingResolutionSourceBindingScope          = "binding_scope"
	BindingResolutionSourceExplicitWholeFile     = "explicit_whole_file_scope"
	BindingResolutionSourceSingleSymbolFile      = "single_symbol_file"
	BindingResolutionSourceOperatorDecisionHint  = "operator_or_decision_hint"
	BindingResolutionSourceLanguageAdapterRange  = "language_adapter_range"
	BindingResolutionSourceWholeFileFallback     = "whole_file_fallback"
	BindingResolutionSourceUnknownAdapterPosture = "unknown_language_adapter_posture"
	BindingResolutionSourceMarkdownSection       = "markdown_section_target"
	BindingResolutionSourceTargetMarker          = "target_marker"
	BindingResolutionSourceYAMLTarget            = "yaml_target"
	BindingResolutionSourceJSONTarget            = "json_target"
)

type BindingResolutionOptions struct {
	ExplicitTargets []BindingTarget
	Hints           []string
	Scope           string
	FallbackReason  string
	DecisionText    string
}

type BindingResolution struct {
	Targets     []BindingTarget
	Symbols     []AffectedSymbol
	Diagnostics []BindingDiagnostic
}

type BindingResolutionError struct {
	Diagnostics []BindingDiagnostic
}

func (err BindingResolutionError) Error() string {
	messages := make([]string, 0, len(err.Diagnostics))
	for _, diagnostic := range err.Diagnostics {
		if diagnostic.Severity != "block" {
			continue
		}
		if diagnostic.FilePath == "" {
			messages = append(messages, diagnostic.Message)
			continue
		}
		messages = append(messages, diagnostic.FilePath+": "+diagnostic.Message)
	}
	if len(messages) == 0 {
		return "binding resolution failed"
	}
	return "binding resolution blocked: " + strings.Join(messages, "; ")
}

func BindingResolutionStrategyOrder() []string {
	return []string{
		BindingResolutionSourceExplicitTargets,
		BindingResolutionSourcePathClassification,
		BindingResolutionSourceBindingScope,
		BindingResolutionSourceExplicitWholeFile,
		BindingResolutionSourceSingleSymbolFile,
		BindingResolutionSourceOperatorDecisionHint,
		BindingResolutionSourceLanguageAdapterRange,
		BindingResolutionSourceWholeFileFallback,
	}
}

func ResolveBindingTargets(
	projectRoot string,
	files []AffectedFile,
	options BindingResolutionOptions,
) (BindingResolution, error) {
	resolution := BindingResolution{}
	scope := strings.TrimSpace(options.Scope)
	if scope == "" {
		scope = BindingScopeAuto
	}

	if len(options.ExplicitTargets) > 0 {
		resolution.Targets = normalizeBindingTargets(attachExplicitTargetEvaluators(projectRoot, options.ExplicitTargets))
		resolution.Symbols = affectedSymbolsFromBindingTargets(resolution.Targets)
		return resolution, nil
	}

	if scope == BindingScopeWholeFile && strings.TrimSpace(options.FallbackReason) == "" {
		diagnostic := BindingDiagnostic{
			Kind:     "missing_fallback_reason",
			Severity: "block",
			Message:  "whole-file binding requires binding_fallback_reason",
		}
		return resolution, BindingResolutionError{Diagnostics: []BindingDiagnostic{diagnostic}}
	}

	for _, file := range files {
		fileResolution := resolveFileBindingTarget(projectRoot, file.Path, options, scope)
		resolution.Targets = append(resolution.Targets, fileResolution.Targets...)
		resolution.Symbols = append(resolution.Symbols, fileResolution.Symbols...)
		resolution.Diagnostics = append(resolution.Diagnostics, fileResolution.Diagnostics...)
	}

	resolution.Targets = normalizeBindingTargets(resolution.Targets)
	resolution.Symbols = normalizeAffectedSymbols(resolution.Symbols)

	if hasBlockingBindingDiagnostics(resolution.Diagnostics) {
		return resolution, BindingResolutionError{Diagnostics: resolution.Diagnostics}
	}

	return resolution, nil
}

func resolveFileBindingTarget(
	projectRoot string,
	relPath string,
	options BindingResolutionOptions,
	scope string,
) BindingResolution {
	if generatedOrIgnoredPath(relPath) {
		return BindingResolution{Targets: []BindingTarget{{
			Kind:             BindingTargetGenerated,
			FilePath:         relPath,
			Reason:           "generated or local runtime path; no code-object binding required",
			ResolutionSource: BindingResolutionSourcePathClassification,
		}}}
	}

	if carrierOnlyPath(relPath) {
		return BindingResolution{Targets: []BindingTarget{{
			Kind:             BindingTargetCarrier,
			FilePath:         relPath,
			Reason:           "carrier path; no code-object binding required",
			ResolutionSource: BindingResolutionSourcePathClassification,
		}}}
	}

	if scope == BindingScopeModule {
		return BindingResolution{Targets: []BindingTarget{moduleBindingTarget(projectRoot, relPath)}}
	}

	if scope == BindingScopeWholeFile {
		return BindingResolution{Targets: []BindingTarget{wholeFileFallbackTarget(relPath, "", options.FallbackReason, BindingResolutionSourceExplicitWholeFile)}}
	}

	support := codebase.InspectFileBindingSupport(projectRoot, relPath)
	switch support.Posture {
	case codebase.BindingSupportSymbols:
		return resolveSymbolFileBinding(support, options)
	case codebase.BindingSupportRangeOnly:
		return BindingResolution{Targets: []BindingTarget{rangeBindingTarget(support.Ranges[0], support.Reason)}}
	case codebase.BindingSupportUnsupportedLanguage, codebase.BindingSupportReadFailed:
		reason := support.Reason
		if reason == "" {
			reason = "symbol and range extraction unavailable"
		}
		return BindingResolution{Targets: []BindingTarget{wholeFileFallbackTarget(relPath, support.Language, reason, support.Posture)}}
	default:
		return BindingResolution{Targets: []BindingTarget{wholeFileFallbackTarget(relPath, support.Language, "unknown language adapter posture", support.Posture)}}
	}
}

func resolveSymbolFileBinding(
	support codebase.FileBindingSupport,
	options BindingResolutionOptions,
) BindingResolution {
	if len(support.Symbols) == 1 && len(options.Hints) == 0 {
		target := symbolBindingTarget(support.Symbols[0], support.Language, BindingResolutionSourceSingleSymbolFile)
		return BindingResolution{
			Targets: []BindingTarget{target},
			Symbols: []AffectedSymbol{affectedSymbolFromSnapshot(support.Symbols[0])},
		}
	}

	matches := matchingBindingSymbols(support.Symbols, append(options.Hints, options.DecisionText))
	if len(matches) > 0 {
		targets := make([]BindingTarget, 0, len(matches))
		symbols := make([]AffectedSymbol, 0, len(matches))
		for _, match := range matches {
			targets = append(targets, symbolBindingTarget(match, support.Language, BindingResolutionSourceOperatorDecisionHint))
			symbols = append(symbols, affectedSymbolFromSnapshot(match))
		}
		return BindingResolution{Targets: targets, Symbols: symbols}
	}

	candidates := make([]string, 0, len(support.Symbols))
	for _, symbol := range support.Symbols {
		candidates = append(candidates, fmt.Sprintf("%s %s:%d", symbol.SymbolKind, symbol.SymbolName, symbol.Line))
	}

	return BindingResolution{
		Diagnostics: []BindingDiagnostic{{
			FilePath: support.FilePath,
			Kind:     "needs_binding_resolution",
			Severity: "block",
			Message:  fmt.Sprintf("multiple parseable symbols found; provide binding_hints or explicit binding_targets (%s)", strings.Join(candidates, ", ")),
		}},
	}
}

func moduleBindingTarget(projectRoot, relPath string) BindingTarget {
	modulePath := filepath.ToSlash(filepath.Dir(relPath))
	if modulePath == "." {
		modulePath = relPath
	}

	hash := sha256.Sum256([]byte(projectRoot + ":" + modulePath))
	return BindingTarget{
		Kind:             BindingTargetModule,
		FilePath:         relPath,
		ModulePath:       modulePath,
		ModuleHash:       hex.EncodeToString(hash[:]),
		Reason:           "explicit module binding scope",
		ResolutionSource: BindingResolutionSourceBindingScope,
	}
}

func symbolBindingTarget(symbol codebase.SymbolSnapshot, language string, source string) BindingTarget {
	anchor := codebase.BuildSymbolAnchor(symbol, language)
	return BindingTarget{
		Kind:             BindingTargetSymbol,
		AnchorID:         anchor.ID,
		AnchorVersion:    anchor.Version,
		FilePath:         symbol.FilePath,
		Language:         language,
		SymbolName:       symbol.SymbolName,
		SymbolKind:       symbol.SymbolKind,
		Receiver:         symbol.Receiver,
		QualifiedName:    anchor.QualifiedName,
		SignatureHash:    anchor.SignatureHash,
		Line:             symbol.Line,
		EndLine:          symbol.EndLine,
		BodyHash:         symbol.Hash,
		Confidence:       "high",
		ResolutionSource: source,
	}
}

func rangeBindingTarget(snapshot codebase.StableRangeSnapshot, reason string) BindingTarget {
	return BindingTarget{
		Kind:             BindingTargetRange,
		FilePath:         snapshot.FilePath,
		Language:         snapshot.Language,
		Line:             snapshot.StartLine,
		EndLine:          snapshot.EndLine,
		AnchorHash:       snapshot.AnchorHash,
		TextHash:         snapshot.TextHash,
		NearestSymbol:    snapshot.NearestName,
		Reason:           reason,
		Confidence:       "medium",
		ResolutionSource: BindingResolutionSourceLanguageAdapterRange,
	}
}

func wholeFileFallbackTarget(relPath, language, reason, posture string) BindingTarget {
	source := BindingResolutionSourceWholeFileFallback
	if posture == BindingResolutionSourceExplicitWholeFile {
		source = BindingResolutionSourceExplicitWholeFile
	}
	return BindingTarget{
		Kind:             BindingTargetWholeFileFallback,
		FilePath:         relPath,
		Language:         language,
		Reason:           reason,
		WhySymbolFailed:  reason,
		WhyRangeFailed:   reason,
		LanguageSupport:  posture,
		Confidence:       "low",
		ResolutionSource: source,
	}
}

func affectedSymbolFromSnapshot(symbol codebase.SymbolSnapshot) AffectedSymbol {
	language, _ := codebase.LanguageForPath(symbol.FilePath)
	anchor := codebase.BuildSymbolAnchor(symbol, language)
	return AffectedSymbol{
		AnchorID:      anchor.ID,
		AnchorVersion: anchor.Version,
		FilePath:      symbol.FilePath,
		Language:      language,
		SymbolName:    symbol.SymbolName,
		SymbolKind:    symbol.SymbolKind,
		Receiver:      symbol.Receiver,
		QualifiedName: anchor.QualifiedName,
		SignatureHash: anchor.SignatureHash,
		Line:          symbol.Line,
		EndLine:       symbol.EndLine,
		Hash:          symbol.Hash,
	}
}

// ResolveLegacyAffectedSymbolAnchor is the review-time compatibility bridge.
// It never writes and never guesses: exactly one current declaration upgrades
// to a durable anchor; zero or multiple matches return a typed
// needs_binding_resolution diagnostic.
func ResolveLegacyAffectedSymbolAnchor(projectRoot string, legacy AffectedSymbol) (AffectedSymbol, *BindingDiagnostic) {
	snapshots, err := codebase.ExtractSymbolSnapshots(projectRoot, legacy.FilePath)
	if err != nil {
		return legacy, &BindingDiagnostic{
			FilePath: legacy.FilePath,
			Kind:     SymbolBindingNeedsBindingResolution,
			Severity: "block",
			Message:  "legacy symbol binding cannot be resolved: " + err.Error(),
		}
	}
	matches := make([]codebase.SymbolSnapshot, 0)
	for _, snapshot := range snapshots {
		if snapshot.SymbolName != legacy.SymbolName || snapshot.SymbolKind != legacy.SymbolKind {
			continue
		}
		if legacy.Receiver != "" && snapshot.Receiver != legacy.Receiver {
			continue
		}
		matches = append(matches, snapshot)
	}
	if len(matches) == 1 {
		return affectedSymbolFromSnapshot(matches[0]), nil
	}
	message := fmt.Sprintf("legacy symbol binding matched %d current declarations; operator rebind selection required", len(matches))
	return legacy, &BindingDiagnostic{
		FilePath: legacy.FilePath,
		Kind:     SymbolBindingNeedsBindingResolution,
		Severity: "block",
		Message:  message,
	}
}

func affectedSymbolsFromBindingTargets(targets []BindingTarget) []AffectedSymbol {
	symbols := make([]AffectedSymbol, 0, len(targets))
	for _, target := range targets {
		if target.Kind != BindingTargetSymbol {
			continue
		}
		symbols = append(symbols, AffectedSymbol{
			AnchorID:      target.AnchorID,
			AnchorVersion: target.AnchorVersion,
			FilePath:      target.FilePath,
			Language:      target.Language,
			SymbolName:    target.SymbolName,
			SymbolKind:    target.SymbolKind,
			Receiver:      target.Receiver,
			QualifiedName: target.QualifiedName,
			SignatureHash: target.SignatureHash,
			Line:          target.Line,
			EndLine:       target.EndLine,
			Hash:          target.BodyHash,
		})
	}
	return normalizeAffectedSymbols(symbols)
}

func normalizeBindingTargets(targets []BindingTarget) []BindingTarget {
	out := make([]BindingTarget, 0, len(targets))
	seen := make(map[string]struct{})
	for _, target := range targets {
		target.Kind = strings.TrimSpace(target.Kind)
		target.TargetRef = normalizeSemanticTargetRef(target.TargetRef)
		target.AnchorID = strings.TrimSpace(target.AnchorID)
		if strings.TrimSpace(target.FilePath) != "" {
			target.FilePath = normalizeDecisionProjectPath(target.FilePath)
		}
		target.Language = strings.TrimSpace(target.Language)
		target.SymbolName = strings.TrimSpace(target.SymbolName)
		target.SymbolKind = strings.TrimSpace(target.SymbolKind)
		target.Receiver = strings.TrimSpace(target.Receiver)
		target.QualifiedName = strings.TrimSpace(target.QualifiedName)
		target.SignatureHash = strings.TrimSpace(target.SignatureHash)
		target.BodyHash = strings.TrimSpace(target.BodyHash)
		target.AnchorHash = strings.TrimSpace(target.AnchorHash)
		target.TextHash = strings.TrimSpace(target.TextHash)
		if strings.TrimSpace(target.ModulePath) != "" {
			target.ModulePath = normalizeDecisionModulePath(
				target.ModulePath,
			)
		}
		target.ModuleHash = strings.TrimSpace(target.ModuleHash)
		target.Reason = strings.TrimSpace(target.Reason)
		target.WhySymbolFailed = strings.TrimSpace(target.WhySymbolFailed)
		target.WhyRangeFailed = strings.TrimSpace(target.WhyRangeFailed)
		target.LanguageSupport = strings.TrimSpace(target.LanguageSupport)
		target.Confidence = strings.TrimSpace(target.Confidence)
		target.ResolutionSource = strings.TrimSpace(target.ResolutionSource)
		if target.ResolutionSource == "" && semanticBindingTargetKind(target.Kind) {
			target.ResolutionSource = BindingResolutionSourceExplicitTargets
		}
		if target.Kind == "" ||
			target.FilePath == "" &&
				target.TargetRef == "" &&
				target.ModulePath == "" {
			continue
		}
		key := strings.Join([]string{
			target.Kind,
			target.TargetRef,
			target.AnchorID,
			target.FilePath,
			target.SymbolKind,
			target.Receiver,
			target.SymbolName,
			target.ModulePath,
			target.TextHash,
		}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FilePath != out[j].FilePath {
			return out[i].FilePath < out[j].FilePath
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].SymbolName < out[j].SymbolName
	})
	return out
}

func semanticBindingTargetKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case BindingTargetSpecSection, BindingTargetAPIContract, BindingTargetInvariant:
		return true
	default:
		return false
	}
}

func normalizeSemanticTargetRef(ref string) string {
	return strings.TrimSpace(ref)
}

func normalizeAffectedSymbols(symbols []AffectedSymbol) []AffectedSymbol {
	out := make([]AffectedSymbol, 0, len(symbols))
	seen := make(map[string]struct{})
	for _, symbol := range symbols {
		symbol.FilePath = normalizeProjectPath(symbol.FilePath)
		symbol.AnchorID = strings.TrimSpace(symbol.AnchorID)
		symbol.Language = strings.TrimSpace(symbol.Language)
		symbol.SymbolName = strings.TrimSpace(symbol.SymbolName)
		symbol.SymbolKind = strings.TrimSpace(symbol.SymbolKind)
		symbol.Receiver = strings.TrimSpace(symbol.Receiver)
		symbol.QualifiedName = strings.TrimSpace(symbol.QualifiedName)
		symbol.SignatureHash = strings.TrimSpace(symbol.SignatureHash)
		symbol.Hash = strings.TrimSpace(symbol.Hash)
		if symbol.FilePath == "" || symbol.SymbolName == "" {
			continue
		}
		key := symbol.AnchorID
		if key == "" {
			key = symbol.FilePath + "\x00" + symbol.SymbolKind + "\x00" + symbol.Receiver + "\x00" + symbol.SymbolName
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, symbol)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FilePath != out[j].FilePath {
			return out[i].FilePath < out[j].FilePath
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].SymbolName < out[j].SymbolName
	})
	return out
}

func attachExplicitTargetEvaluators(projectRoot string, targets []BindingTarget) []BindingTarget {
	out := make([]BindingTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, attachExplicitTargetEvaluator(projectRoot, target))
	}
	return out
}

func attachExplicitTargetEvaluator(projectRoot string, target BindingTarget) BindingTarget {
	if !semanticBindingTargetKind(target.Kind) {
		return target
	}
	if strings.TrimSpace(target.FilePath) == "" {
		return target
	}
	if strings.TrimSpace(target.TextHash) != "" {
		return target
	}

	section, ok := extractMarkedTargetRange(projectRoot, target.FilePath, target.TargetRef)
	if !ok {
		jsonSection, jsonOK := extractJSONTargetRange(projectRoot, target.FilePath, target.TargetRef)
		if jsonOK {
			section = jsonSection
		} else {
			yamlSection, yamlOK := extractYAMLTargetRange(projectRoot, target.FilePath, target.TargetRef)
			if yamlOK {
				section = yamlSection
			} else {
				markdownSection, markdownOK := extractMarkdownTargetRange(projectRoot, target.FilePath, target.TargetRef)
				if !markdownOK {
					return target
				}
				section = markdownSection
			}
		}
	}
	target.Line = section.StartLine
	target.EndLine = section.EndLine
	target.AnchorHash = section.AnchorHash
	target.TextHash = section.TextHash
	if language, ok := codebase.LanguageForPath(target.FilePath); ok && strings.TrimSpace(target.Language) == "" {
		target.Language = language
	}
	if strings.TrimSpace(target.Confidence) == "" {
		target.Confidence = "medium"
	}
	if strings.TrimSpace(target.ResolutionSource) == "" {
		target.ResolutionSource = section.Source
	}
	return target
}

type markdownTargetRange struct {
	StartLine  int
	EndLine    int
	AnchorHash string
	TextHash   string
	Source     string
}

func extractMarkdownTargetRange(projectRoot, relPath, targetRef string) (markdownTargetRange, bool) {
	token := semanticTargetLookupToken(targetRef)
	if token == "" {
		return markdownTargetRange{}, false
	}

	content, err := readProjectFile(projectRoot, relPath)
	if err != nil {
		return markdownTargetRange{}, false
	}
	lines := splitBindingTextLines(string(content))
	start, end, ok := markdownSemanticTargetFenceRange(lines, targetRef, token)
	if !ok {
		start, end, ok = markdownSpecSectionRange(lines, token)
	}
	if !ok {
		start, end, ok = markdownHeadingRange(lines, token)
	}
	if !ok {
		return markdownTargetRange{}, false
	}
	text := strings.Join(lines[start-1:end], "\n")
	normalized := normalizeBindingRangeText(text)
	anchorHash := sha256.Sum256([]byte(firstBindingNonEmptyLine(normalized)))
	textHash := sha256.Sum256([]byte(normalized))
	return markdownTargetRange{
		StartLine:  start,
		EndLine:    end,
		AnchorHash: hex.EncodeToString(anchorHash[:]),
		TextHash:   hex.EncodeToString(textHash[:]),
		Source:     BindingResolutionSourceMarkdownSection,
	}, true
}

func markdownSemanticTargetFenceRange(lines []string, targetRef string, token string) (int, int, bool) {
	for index := 0; index < len(lines); index++ {
		info, ok := markdownFenceInfo(lines[index])
		if !ok {
			continue
		}
		if !semanticMarkdownFenceInfo(info) {
			continue
		}

		fenceStart := index + 1
		fenceEnd := len(lines)
		for closeIndex := index + 1; closeIndex < len(lines); closeIndex++ {
			if _, ok := markdownFenceInfo(lines[closeIndex]); ok {
				fenceEnd = closeIndex + 1
				break
			}
		}
		if !semanticMarkdownFenceContainsTarget(lines[index+1:fenceEnd-1], targetRef, token) {
			continue
		}
		start := precedingMarkdownHeadingLine(lines, fenceStart)
		if start == 0 {
			start = fenceStart
		}
		end := nextMarkdownHeadingLine(lines, start)
		if end == 0 {
			end = len(lines)
		} else {
			end--
		}
		return start, end, true
	}
	return 0, 0, false
}

func semanticMarkdownFenceInfo(info string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(info)))
	for _, field := range fields {
		switch strings.ReplaceAll(field, "-", "_") {
		case "spec_section", "api_contract", "invariant":
			return true
		}
	}
	return false
}

func semanticMarkdownFenceContainsTarget(lines []string, targetRef string, token string) bool {
	targetRef = normalizeSemanticTargetRef(targetRef)
	token = normalizeSemanticTargetRef(token)
	for _, line := range lines {
		for _, key := range []string{"id", "target_ref", "section_id"} {
			value := yamlLineScalarValue(line, key)
			if value == "" {
				continue
			}
			if value == targetRef {
				return true
			}
			if key != "target_ref" && value == token {
				return true
			}
		}
	}
	return false
}

func extractMarkedTargetRange(projectRoot, relPath, targetRef string) (markdownTargetRange, bool) {
	targetRef = normalizeSemanticTargetRef(targetRef)
	if targetRef == "" {
		return markdownTargetRange{}, false
	}

	content, err := readProjectFile(projectRoot, relPath)
	if err != nil {
		return markdownTargetRange{}, false
	}
	lines := splitBindingTextLines(string(content))
	start := 0
	for index, line := range lines {
		if markedTargetRef(line) != targetRef {
			continue
		}
		start = index + 1
		break
	}
	if start == 0 {
		return markdownTargetRange{}, false
	}

	end := len(lines)
	for index := start; index < len(lines); index++ {
		if markedTargetRef(lines[index]) != "" {
			end = index
			break
		}
	}
	if end < start {
		end = start
	}

	text := strings.Join(lines[start-1:end], "\n")
	normalized := normalizeBindingRangeText(text)
	anchorHash := sha256.Sum256([]byte(firstBindingNonEmptyLine(normalized)))
	textHash := sha256.Sum256([]byte(normalized))
	return markdownTargetRange{
		StartLine:  start,
		EndLine:    end,
		AnchorHash: hex.EncodeToString(anchorHash[:]),
		TextHash:   hex.EncodeToString(textHash[:]),
		Source:     BindingResolutionSourceTargetMarker,
	}, true
}

func extractYAMLTargetRange(projectRoot, relPath, targetRef string) (markdownTargetRange, bool) {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".yaml", ".yml":
	default:
		return markdownTargetRange{}, false
	}
	targetRef = normalizeSemanticTargetRef(targetRef)
	if targetRef == "" {
		return markdownTargetRange{}, false
	}
	token := semanticTargetLookupToken(targetRef)
	content, err := readProjectFile(projectRoot, relPath)
	if err != nil {
		return markdownTargetRange{}, false
	}
	lines := splitBindingTextLines(string(content))
	start := 0
	startIndent := 0
	for index, line := range lines {
		if !yamlLineMatchesTarget(line, targetRef, token) {
			continue
		}
		start = index + 1
		startIndent = lineIndent(line)
		break
	}
	if start == 0 {
		return markdownTargetRange{}, false
	}

	end := len(lines)
	for index := start; index < len(lines); index++ {
		line := lines[index]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if lineIndent(line) <= startIndent && yamlLineStartsSibling(line) {
			end = index
			break
		}
	}
	if end < start {
		end = start
	}

	text := strings.Join(lines[start-1:end], "\n")
	normalized := normalizeBindingRangeText(text)
	anchorHash := sha256.Sum256([]byte(firstBindingNonEmptyLine(normalized)))
	textHash := sha256.Sum256([]byte(normalized))
	return markdownTargetRange{
		StartLine:  start,
		EndLine:    end,
		AnchorHash: hex.EncodeToString(anchorHash[:]),
		TextHash:   hex.EncodeToString(textHash[:]),
		Source:     BindingResolutionSourceYAMLTarget,
	}, true
}

func extractJSONTargetRange(projectRoot, relPath, targetRef string) (markdownTargetRange, bool) {
	if strings.ToLower(filepath.Ext(relPath)) != ".json" {
		return markdownTargetRange{}, false
	}
	targetRef = normalizeSemanticTargetRef(targetRef)
	if targetRef == "" {
		return markdownTargetRange{}, false
	}
	content, err := readProjectFile(projectRoot, relPath)
	if err != nil {
		return markdownTargetRange{}, false
	}
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.UseNumber()
	start, end, ok := findJSONTargetObject(decoder, targetRef, semanticTargetLookupToken(targetRef))
	if !ok || start < 0 || end > len(content) || start >= end {
		return markdownTargetRange{}, false
	}
	startLine := bindingLineForOffset(content, start)
	endLine := bindingLineForOffset(content, end)
	lines := splitBindingTextLines(string(content))
	text := strings.Join(lines[startLine-1:endLine], "\n")
	normalized := normalizeBindingRangeText(text)
	anchorHash := sha256.Sum256([]byte(firstBindingNonEmptyLine(normalized)))
	textHash := sha256.Sum256([]byte(normalized))
	return markdownTargetRange{
		StartLine:  startLine,
		EndLine:    endLine,
		AnchorHash: hex.EncodeToString(anchorHash[:]),
		TextHash:   hex.EncodeToString(textHash[:]),
		Source:     BindingResolutionSourceJSONTarget,
	}, true
}

func findJSONTargetObject(decoder *json.Decoder, targetRef string, token string) (int, int, bool) {
	valueStart := int(decoder.InputOffset())
	value, err := decoder.Token()
	if err != nil {
		return 0, 0, false
	}
	delimiter, ok := value.(json.Delim)
	if !ok {
		return 0, 0, false
	}
	switch delimiter {
	case '{':
		return findJSONTargetObjectBody(decoder, valueStart, targetRef, token)
	case '[':
		for decoder.More() {
			start, end, ok := findJSONTargetObject(decoder, targetRef, token)
			if ok {
				return start, end, true
			}
		}
		_, _ = decoder.Token()
	}
	return 0, 0, false
}

func findJSONTargetObjectBody(
	decoder *json.Decoder,
	objectStart int,
	targetRef string,
	token string,
) (int, int, bool) {
	matches := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, 0, false
		}
		key, ok := keyToken.(string)
		if !ok {
			return 0, 0, false
		}
		if key == "id" || key == "target_ref" || key == "section_id" {
			valueToken, err := decoder.Token()
			if err != nil {
				return 0, 0, false
			}
			value, ok := valueToken.(string)
			if ok && jsonTargetValueMatches(key, value, targetRef, token) {
				matches = true
			}
			continue
		}
		start, end, ok := findJSONTargetObject(decoder, targetRef, token)
		if ok {
			return start, end, true
		}
	}
	_, err := decoder.Token()
	if err != nil {
		return 0, 0, false
	}
	if matches {
		return objectStart, int(decoder.InputOffset()), true
	}
	return 0, 0, false
}

func jsonTargetValueMatches(key string, value string, targetRef string, token string) bool {
	value = normalizeSemanticTargetRef(value)
	if value == targetRef {
		return true
	}
	return (key == "id" || key == "section_id") && value == token
}

func bindingLineForOffset(content []byte, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	return strings.Count(string(content[:offset]), "\n") + 1
}

func yamlLineMatchesTarget(line string, targetRef string, token string) bool {
	for _, key := range []string{"id", "target_ref", "section_id"} {
		value := yamlLineScalarValue(line, key)
		if value == "" {
			continue
		}
		if value == targetRef {
			return true
		}
		if (key == "id" || key == "section_id") && value == token {
			return true
		}
	}
	return false
}

func yamlLineScalarValue(line string, key string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "-")
	trimmed = strings.TrimSpace(trimmed)
	prefix := key + ":"
	if !strings.HasPrefix(trimmed, prefix) {
		return ""
	}
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if index := strings.Index(value, "#"); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	return normalizeSemanticTargetRef(strings.Trim(value, `"'`))
}

func yamlLineStartsSibling(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "- ") {
		return true
	}
	return yamlLineScalarValue(line, "id") != "" ||
		yamlLineScalarValue(line, "target_ref") != "" ||
		yamlLineScalarValue(line, "section_id") != ""
}

func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func markedTargetRef(line string) string {
	trimmed := strings.TrimSpace(line)
	const marker = "haft-target:"
	index := strings.Index(trimmed, marker)
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(trimmed[index+len(marker):])
	if cut := strings.IndexAny(value, " \t"); cut >= 0 {
		value = strings.TrimSpace(value[:cut])
	}
	return normalizeSemanticTargetRef(strings.Trim(value, `"'`))
}

func semanticTargetLookupToken(targetRef string) string {
	ref := strings.TrimSpace(targetRef)
	if ref == "" {
		return ""
	}
	if index := strings.LastIndex(ref, ":"); index >= 0 && index < len(ref)-1 {
		return strings.TrimSpace(ref[index+1:])
	}
	return ref
}

func markdownSpecSectionRange(lines []string, token string) (int, int, bool) {
	for index := 0; index < len(lines); index++ {
		info, ok := markdownFenceInfo(lines[index])
		if !ok || !strings.Contains(info, "spec-section") {
			continue
		}
		fenceStart := index + 1
		fenceEnd := len(lines)
		for closeIndex := index + 1; closeIndex < len(lines); closeIndex++ {
			if _, ok := markdownFenceInfo(lines[closeIndex]); ok {
				fenceEnd = closeIndex + 1
				break
			}
		}
		if !specSectionFenceContainsID(lines[index+1:fenceEnd-1], token) {
			continue
		}
		start := precedingMarkdownHeadingLine(lines, fenceStart)
		if start == 0 {
			start = fenceStart
		}
		end := nextMarkdownHeadingLine(lines, start)
		if end == 0 {
			end = len(lines)
		} else {
			end--
		}
		return start, end, true
	}
	return 0, 0, false
}

func markdownFenceInfo(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "```")), true
}

func specSectionFenceContainsID(lines []string, token string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"id:", "section_id:"} {
			if !strings.HasPrefix(trimmed, prefix) {
				continue
			}
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), `"'`)
			if value == token {
				return true
			}
		}
	}
	return false
}

func markdownHeadingRange(lines []string, token string) (int, int, bool) {
	for index, line := range lines {
		level, heading, ok := parseBindingMarkdownHeading(line)
		if !ok {
			continue
		}
		if !strings.Contains(heading, token) {
			continue
		}
		start := index + 1
		end := len(lines)
		for next := index + 1; next < len(lines); next++ {
			nextLevel, _, ok := parseBindingMarkdownHeading(lines[next])
			if ok && nextLevel <= level {
				end = next
				break
			}
		}
		return start, end, true
	}
	return 0, 0, false
}

func precedingMarkdownHeadingLine(lines []string, beforeLine int) int {
	for index := beforeLine - 2; index >= 0; index-- {
		if _, _, ok := parseBindingMarkdownHeading(lines[index]); ok {
			return index + 1
		}
	}
	return 0
}

func nextMarkdownHeadingLine(lines []string, afterLine int) int {
	afterLevel, _, ok := parseBindingMarkdownHeading(lines[afterLine-1])
	if !ok {
		afterLevel = 1
	}
	for index := afterLine; index < len(lines); index++ {
		level, _, ok := parseBindingMarkdownHeading(lines[index])
		if ok && level <= afterLevel {
			return index + 1
		}
	}
	return 0
}

func parseBindingMarkdownHeading(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(trimmed[level+1:]), true
}

func splitBindingTextLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func normalizeBindingRangeText(text string) string {
	lines := splitBindingTextLines(text)
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func firstBindingNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func matchingBindingSymbols(symbols []codebase.SymbolSnapshot, texts []string) []codebase.SymbolSnapshot {
	haystack := strings.ToLower(strings.Join(texts, "\n"))
	if strings.TrimSpace(haystack) == "" {
		return nil
	}
	matches := make([]codebase.SymbolSnapshot, 0, len(symbols))
	for _, symbol := range symbols {
		name := strings.ToLower(strings.TrimSpace(symbol.SymbolName))
		if name == "" {
			continue
		}
		if strings.Contains(haystack, name) {
			matches = append(matches, symbol)
		}
	}
	return matches
}

func hasBlockingBindingDiagnostics(diagnostics []BindingDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "block" {
			return true
		}
	}
	return false
}
