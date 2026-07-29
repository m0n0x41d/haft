package artifact

import (
	"regexp"
	"strings"
)

// artifactIDPattern matches the leading prefix-date shape shared by haft
// artifact IDs and artifact-like search tokens.
var artifactIDPattern = regexp.MustCompile(`(?i)^[a-z]+-\d{6,}`)

var canonicalArtifactIDPrefixes = map[string]struct{}{
	KindNote.IDPrefix():              {},
	KindProblemCard.IDPrefix():       {},
	KindSolutionPortfolio.IDPrefix(): {},
	KindDecisionRecord.IDPrefix():    {},
	KindWorkCommission.IDPrefix():    {},
	KindMethodRun.IDPrefix():         {},
	KindEvidencePack.IDPrefix():      {},
	KindRefreshReport.IDPrefix():     {},
}

// IsArtifactID reports whether value is a single artifact ID token. Exact ID
// queries use direct lookup and must never fall through to lexical or semantic
// recall.
func IsArtifactID(value string) bool {
	value = strings.TrimSpace(value)
	return !strings.ContainsAny(value, " \t\n") && artifactIDPattern.MatchString(value)
}

// IsCanonicalArtifactID reports whether value has both the artifact ID shape
// and a prefix owned by a current Haft artifact kind. Namespace routing must
// use this narrower predicate: an arbitrary hyphenated code symbol is not a
// Haft artifact merely because it happens to contain a date-like segment.
func IsCanonicalArtifactID(value string) bool {
	value = strings.TrimSpace(value)
	if !IsArtifactID(value) {
		return false
	}

	prefix, _, found := strings.Cut(value, "-")
	if !found {
		return false
	}
	_, canonical := canonicalArtifactIDPrefixes[strings.ToLower(prefix)]
	return canonical
}
