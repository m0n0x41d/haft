// Package profiledetector derives non-binding project-profile suggestions
// from an explicit repository snapshot. The detector is an orientation aid:
// it cannot admit a profile, establish applicability, or mutate project state.
package profiledetector

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	Version       = "haft.project-profile-detector/file-paths-v1"
	PolicyVersion = "haft.project-profile-detector-policy/file-paths-v1"
)

type Classification string

const (
	SoftwareSignals           Classification = "software_signals"
	NonSoftwareSignals        Classification = "non_software_signals"
	MixedSignals              Classification = "mixed_signals"
	InsufficientDetectorBasis Classification = "insufficient_detector_basis"
)

type ConfidencePosture string

const (
	SupportedConfidence    ConfidencePosture = "supported"
	ConflictingConfidence  ConfidencePosture = "conflicting"
	InsufficientConfidence ConfidencePosture = "insufficient"
)

type RealizationKind string

const (
	SoftwareRealization    RealizationKind = "software"
	NonSoftwareRealization RealizationKind = "non_software"
)

type SignalStrength string

const (
	DecisiveSignal   SignalStrength = "decisive"
	SupportingSignal SignalStrength = "supporting"
	ExcludedSignal   SignalStrength = "excluded"
)

type Signal struct {
	ruleID       string
	path         string
	candidateKey string
	kind         RealizationKind
	strength     SignalStrength
}

func (signal Signal) RuleID() string                { return signal.ruleID }
func (signal Signal) Path() string                  { return signal.path }
func (signal Signal) ComponentCandidateKey() string { return signal.candidateKey }
func (signal Signal) RealizationKind() RealizationKind {
	return signal.kind
}
func (signal Signal) Strength() SignalStrength { return signal.strength }

// SuggestedScope is detector output, not a ProfileDeclarationPayload scope.
// Its component reference deliberately is not a stable project ScopeID.
type SuggestedScope struct {
	componentCandidateRef string
	realizationKind       RealizationKind
	orientation           string
	positiveSignals       []Signal
	negativeSignals       []Signal
}

func (scope SuggestedScope) ComponentCandidateRef() string {
	return scope.componentCandidateRef
}
func (scope SuggestedScope) RealizationKind() RealizationKind {
	return scope.realizationKind
}
func (scope SuggestedScope) Orientation() string { return scope.orientation }
func (scope SuggestedScope) PositiveSignals() []Signal {
	return append([]Signal{}, scope.positiveSignals...)
}
func (scope SuggestedScope) NegativeSignals() []Signal {
	return append([]Signal{}, scope.negativeSignals...)
}

// Snapshot is the complete input to the file-path detector v1. It contains no
// declaration, performed Work, or evidence-truth claim.
type Snapshot struct {
	projectRoot       string
	relativeFiles     []string
	scannedFileCount  int
	truncated         bool
	observationDigest string
}

func NewSnapshot(
	projectRoot string,
	relativeFiles []string,
	scannedFileCount int,
	truncated bool,
) (Snapshot, error) {
	root := filepath.Clean(projectRoot)
	if projectRoot == "" || !filepath.IsAbs(root) || root != projectRoot {
		return Snapshot{}, fmt.Errorf("profile detector requires a canonical absolute project root")
	}
	files, err := canonicalRelativeFiles(relativeFiles)
	if err != nil {
		return Snapshot{}, err
	}
	if scannedFileCount < len(files) {
		return Snapshot{}, fmt.Errorf("scanned file count cannot be smaller than the retained file set")
	}
	digest := digestSnapshot(root, files, scannedFileCount, truncated)
	return Snapshot{
		projectRoot:       root,
		relativeFiles:     files,
		scannedFileCount:  scannedFileCount,
		truncated:         truncated,
		observationDigest: digest,
	}, nil
}

func (snapshot Snapshot) ProjectRoot() string { return snapshot.projectRoot }
func (snapshot Snapshot) RelativeFiles() []string {
	return append([]string{}, snapshot.relativeFiles...)
}
func (snapshot Snapshot) ScannedFileCount() int     { return snapshot.scannedFileCount }
func (snapshot Snapshot) Truncated() bool           { return snapshot.truncated }
func (snapshot Snapshot) ObservationDigest() string { return snapshot.observationDigest }

type Suggestion struct {
	classification     Classification
	confidence         ConfidencePosture
	scopes             []SuggestedScope
	conflictingSignals []Signal
	excludedSignals    []Signal
	snapshot           Snapshot
}

func (suggestion Suggestion) DetectorVersion() string { return Version }
func (suggestion Suggestion) SuggestionRef() string {
	writer := newDigestWriter("haft.project-profile-suggestion/v1")
	writer.add(suggestion.snapshot.observationDigest)
	writer.add(string(suggestion.classification))
	return "profile-suggestion:" + writer.digest()
}
func (suggestion Suggestion) Classification() Classification {
	return suggestion.classification
}
func (suggestion Suggestion) ConfidencePosture() ConfidencePosture {
	return suggestion.confidence
}
func (suggestion Suggestion) SuggestedScopes() []SuggestedScope {
	return append([]SuggestedScope{}, suggestion.scopes...)
}
func (suggestion Suggestion) ConflictingSignals() []Signal {
	return append([]Signal{}, suggestion.conflictingSignals...)
}
func (suggestion Suggestion) ExcludedSignals() []Signal {
	return append([]Signal{}, suggestion.excludedSignals...)
}
func (suggestion Suggestion) Snapshot() Snapshot { return suggestion.snapshot }

type signalRule struct {
	id           string
	candidateKey string
	kind         RealizationKind
	strength     SignalStrength
	matches      func(string) bool
}

var signalRules = []signalRule{
	{id: "software_manifest", candidateKey: "software", kind: SoftwareRealization, strength: SupportingSignal, matches: matchesSoftwareManifest},
	{id: "production_source", candidateKey: "software", kind: SoftwareRealization, strength: DecisiveSignal, matches: matchesProductionSource},
	{id: "document_primary_manifest", candidateKey: "documents", kind: NonSoftwareRealization, strength: DecisiveSignal, matches: matchesDocumentPrimaryManifest},
	{id: "document_tool_manifest", candidateKey: "documents", kind: NonSoftwareRealization, strength: SupportingSignal, matches: matchesDocumentToolManifest},
	{id: "document_content", candidateKey: "documents", kind: NonSoftwareRealization, strength: SupportingSignal, matches: matchesDocumentContent},
	{id: "model_artifact", candidateKey: "models", kind: NonSoftwareRealization, strength: DecisiveSignal, matches: matchesModelArtifact},
	{id: "helper_code", candidateKey: "", kind: "", strength: ExcludedSignal, matches: matchesHelperCode},
}

func Detect(snapshot Snapshot) Suggestion {
	signals := detectSignals(snapshot.relativeFiles)
	excluded := signalsWithStrength(signals, ExcludedSignal)
	software := componentEvidenceFor("software", signals)
	documents := componentEvidenceFor("documents", signals)
	models := componentEvidenceFor("models", signals)
	softwareReady := hasRule(software, "software_manifest") && hasRule(software, "production_source")
	documentsReady := documentScopeReady(documents, softwareReady)
	modelsReady := hasRule(models, "model_artifact")
	return buildSuggestion(
		snapshot,
		software,
		documents,
		models,
		excluded,
		softwareReady,
		documentsReady,
		modelsReady,
	)
}

func buildSuggestion(
	snapshot Snapshot,
	software []Signal,
	documents []Signal,
	models []Signal,
	excluded []Signal,
	softwareReady bool,
	documentsReady bool,
	modelsReady bool,
) Suggestion {
	if snapshot.truncated {
		return insufficientSuggestion(snapshot, excluded)
	}
	nonSoftwareReady := documentsReady || modelsReady
	if softwareReady && nonSoftwareReady {
		scopes := suggestedScopes(
			snapshot.observationDigest,
			software,
			documents,
			models,
			documentsReady,
			modelsReady,
		)
		conflicts := signalsForSuggestedScopes(scopes)
		return Suggestion{
			classification:     MixedSignals,
			confidence:         ConflictingConfidence,
			scopes:             scopes,
			conflictingSignals: conflicts,
			excludedSignals:    excluded,
			snapshot:           snapshot,
		}
	}
	if softwareReady {
		scope := newSuggestedScope(
			snapshot.observationDigest,
			"software",
			SoftwareRealization,
			software,
			nil,
		)
		return Suggestion{
			classification:  SoftwareSignals,
			confidence:      SupportedConfidence,
			scopes:          []SuggestedScope{scope},
			excludedSignals: excluded,
			snapshot:        snapshot,
		}
	}
	if nonSoftwareReady {
		scopes := suggestedScopes(
			snapshot.observationDigest,
			nil,
			documents,
			models,
			documentsReady,
			modelsReady,
		)
		return Suggestion{
			classification:  NonSoftwareSignals,
			confidence:      SupportedConfidence,
			scopes:          scopes,
			excludedSignals: excluded,
			snapshot:        snapshot,
		}
	}
	return insufficientSuggestion(snapshot, excluded)
}

func suggestedScopes(
	observationDigest string,
	software []Signal,
	documents []Signal,
	models []Signal,
	documentsReady bool,
	modelsReady bool,
) []SuggestedScope {
	allReadySignals := append([]Signal{}, software...)
	if documentsReady {
		allReadySignals = append(allReadySignals, documents...)
	}
	if modelsReady {
		allReadySignals = append(allReadySignals, models...)
	}
	result := []SuggestedScope{}
	if len(software) > 0 {
		negative := signalsForOtherKinds(allReadySignals, SoftwareRealization)
		result = append(result, newSuggestedScope(observationDigest, "software", SoftwareRealization, software, negative))
	}
	if documentsReady {
		negative := signalsForOtherComponents(allReadySignals, "documents")
		result = append(result, newSuggestedScope(observationDigest, "documents", NonSoftwareRealization, documents, negative))
	}
	if modelsReady {
		negative := signalsForOtherComponents(allReadySignals, "models")
		result = append(result, newSuggestedScope(observationDigest, "models", NonSoftwareRealization, models, negative))
	}
	slices.SortFunc(result, func(left SuggestedScope, right SuggestedScope) int {
		return cmp.Compare(left.orientation, right.orientation)
	})
	return result
}

func newSuggestedScope(
	observationDigest string,
	orientation string,
	kind RealizationKind,
	positive []Signal,
	negative []Signal,
) SuggestedScope {
	return SuggestedScope{
		componentCandidateRef: componentCandidateReference(observationDigest, orientation, positive),
		realizationKind:       kind,
		orientation:           orientation,
		positiveSignals:       canonicalSignals(positive),
		negativeSignals:       canonicalSignals(negative),
	}
}

func insufficientSuggestion(snapshot Snapshot, excluded []Signal) Suggestion {
	return Suggestion{
		classification:  InsufficientDetectorBasis,
		confidence:      InsufficientConfidence,
		excludedSignals: excluded,
		snapshot:        snapshot,
	}
}

func documentScopeReady(signals []Signal, softwareReady bool) bool {
	if hasRule(signals, "document_primary_manifest") {
		return true
	}
	if softwareReady {
		return false
	}
	contentCount := countRule(signals, "document_content")
	toolAndContent := hasRule(signals, "document_tool_manifest") && contentCount > 0
	return toolAndContent || contentCount >= 3
}

func detectSignals(files []string) []Signal {
	groups := make([][]Signal, len(files))
	for index, path := range files {
		groups[index] = signalsForPath(path)
	}
	return canonicalSignals(slices.Concat(groups...))
}

func signalsForPath(path string) []Signal {
	result := []Signal{}
	for _, rule := range signalRules {
		if rule.matches(path) {
			result = append(result, Signal{
				ruleID:       rule.id,
				path:         path,
				candidateKey: rule.candidateKey,
				kind:         rule.kind,
				strength:     rule.strength,
			})
		}
	}
	return result
}

func componentEvidenceFor(candidateKey string, values []Signal) []Signal {
	result := []Signal{}
	for _, value := range values {
		if value.candidateKey == candidateKey {
			result = append(result, value)
		}
	}
	return result
}

func signalsWithStrength(values []Signal, strength SignalStrength) []Signal {
	result := []Signal{}
	for _, value := range values {
		if value.strength == strength {
			result = append(result, value)
		}
	}
	return result
}

func signalsForOtherKinds(values []Signal, kind RealizationKind) []Signal {
	result := []Signal{}
	for _, value := range values {
		if value.kind != "" && value.kind != kind {
			result = append(result, value)
		}
	}
	return result
}

func signalsForOtherComponents(values []Signal, candidateKey string) []Signal {
	result := []Signal{}
	for _, value := range values {
		if value.candidateKey != "" && value.candidateKey != candidateKey {
			result = append(result, value)
		}
	}
	return result
}

func signalsForSuggestedScopes(scopes []SuggestedScope) []Signal {
	groups := make([][]Signal, len(scopes))
	for index, scope := range scopes {
		groups[index] = scope.positiveSignals
	}
	return canonicalSignals(slices.Concat(groups...))
}

func hasRule(values []Signal, ruleID string) bool {
	return countRule(values, ruleID) > 0
}

func countRule(values []Signal, ruleID string) int {
	count := 0
	for _, value := range values {
		if value.ruleID == ruleID {
			count++
		}
	}
	return count
}

func canonicalSignals(values []Signal) []Signal {
	result := append([]Signal{}, values...)
	slices.SortFunc(result, compareSignal)
	return slices.CompactFunc(result, func(left Signal, right Signal) bool {
		return compareSignal(left, right) == 0
	})
}

func compareSignal(left Signal, right Signal) int {
	leftKey := left.candidateKey + "\x00" + left.ruleID + "\x00" + left.path
	rightKey := right.candidateKey + "\x00" + right.ruleID + "\x00" + right.path
	return cmp.Compare(leftKey, rightKey)
}

func componentCandidateReference(
	observationDigest string,
	orientation string,
	signals []Signal,
) string {
	writer := newDigestWriter("haft.project-profile-component-suggestion/v1")
	writer.add(observationDigest)
	writer.add(orientation)
	for _, signal := range canonicalSignals(signals) {
		writer.add(signal.ruleID)
		writer.add(signal.path)
	}
	return "profile-component-suggestion:" + writer.digest()
}

func digestSnapshot(root string, files []string, scanned int, truncated bool) string {
	writer := newDigestWriter("haft.project-profile-observation/file-paths-v1")
	writer.add(root)
	writer.add(strconv.Itoa(scanned))
	writer.add(strconv.FormatBool(truncated))
	for _, path := range files {
		writer.add(path)
	}
	return writer.digest()
}

type digestAccumulator struct{ bytes []byte }

func newDigestWriter(domain string) digestAccumulator {
	writer := digestAccumulator{}
	writer.add(domain)
	return writer
}

func (writer *digestAccumulator) add(value string) {
	length := strconv.Itoa(len(value))
	writer.bytes = append(writer.bytes, []byte(length+":"+value)...)
}

func (writer digestAccumulator) digest() string {
	sum := sha256.Sum256(writer.bytes)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func canonicalRelativeFiles(values []string) ([]string, error) {
	result := make([]string, len(values))
	for index, value := range values {
		canonical, err := canonicalRelativeFile(value)
		if err != nil {
			return nil, fmt.Errorf("relative file %d: %w", index, err)
		}
		result[index] = canonical
	}
	slices.Sort(result)
	result = slices.Compact(result)
	return result, nil
}

func canonicalRelativeFile(value string) (string, error) {
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if value == "" || filepath.IsAbs(value) || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path %q must stay inside the project root", value)
	}
	return cleaned, nil
}

func matchesSoftwareManifest(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return slices.Contains([]string{
		"build.gradle",
		"build.gradle.kts",
		"cargo.toml",
		"go.mod",
		"package.json",
		"pom.xml",
		"pyproject.toml",
	}, base) || strings.HasSuffix(base, ".csproj")
}

func matchesProductionSource(path string) bool {
	if matchesHelperCode(path) || matchesDocumentContent(path) {
		return false
	}
	extension := strings.ToLower(filepath.Ext(path))
	if !slices.Contains([]string{
		".c", ".cc", ".cpp", ".cs", ".go", ".h", ".hpp", ".java", ".js",
		".jsx", ".kt", ".kts", ".php", ".py", ".rb", ".rs", ".swift", ".ts", ".tsx",
	}, extension) {
		return false
	}
	segments := strings.Split(filepath.ToSlash(path), "/")
	if len(segments) == 1 {
		return true
	}
	return slices.Contains([]string{"app", "cmd", "internal", "lib", "pkg", "src"}, segments[0])
}

func matchesDocumentPrimaryManifest(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return slices.Contains([]string{
		"book.toml",
		"quarto.yml",
		"quarto.yaml",
		"typst.toml",
	}, base)
}

func matchesDocumentToolManifest(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return slices.Contains([]string{
		"mkdocs.yml",
		"mkdocs.yaml",
		"docusaurus.config.js",
		"docusaurus.config.ts",
	}, base)
}

func matchesDocumentContent(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	if !slices.Contains([]string{".adoc", ".bib", ".docx", ".md", ".pdf", ".rst", ".tex", ".typ"}, extension) {
		return false
	}
	segments := strings.Split(filepath.ToSlash(path), "/")
	if len(segments) < 2 {
		return false
	}
	return slices.Contains([]string{"content", "docs", "manuscript", "papers"}, segments[0])
}

func matchesModelArtifact(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return slices.Contains([]string{
		".gguf",
		".joblib",
		".onnx",
		".pkl",
		".pt",
		".pth",
		".safetensors",
	}, extension)
}

func matchesHelperCode(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	if !slices.Contains([]string{".js", ".py", ".rb", ".sh", ".ts"}, extension) {
		return false
	}
	segments := strings.Split(filepath.ToSlash(path), "/")
	if len(segments) < 2 {
		return false
	}
	return slices.Contains([]string{"bin", "scripts", "tools"}, segments[0])
}
