package artifact

import (
	"regexp"
	"strings"
)

// artifactIDPattern matches the leading prefix-date shape shared by haft
// artifact IDs (prob-/dec-/sol-/note-/evid-/wc-/rr- followed by a date).
var artifactIDPattern = regexp.MustCompile(`(?i)^[a-z]+-\d{6,}`)

// IsArtifactID reports whether value is a single artifact ID token. Exact ID
// queries use direct lookup and must never fall through to lexical or semantic
// recall.
func IsArtifactID(value string) bool {
	value = strings.TrimSpace(value)
	return !strings.ContainsAny(value, " \t\n") && artifactIDPattern.MatchString(value)
}
