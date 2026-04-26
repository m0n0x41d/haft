package scopeauth

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type Verdict string

const (
	Allowed      Verdict = "allowed"
	OutOfScope   Verdict = "out_of_scope"
	Forbidden    Verdict = "forbidden"
	UnknownScope Verdict = "unknown_scope"
)

type ReasonCode string

const (
	ReasonOutOfScope   ReasonCode = "out_of_scope_paths"
	ReasonForbidden    ReasonCode = "forbidden_paths"
	ReasonUnknownScope ReasonCode = "unknown_scope_paths"
)

type CommissionScope struct {
	AllowedPaths   []string
	ForbiddenPaths []string
	AffectedFiles  []string
	Lockset        []string
}

type PathFacts struct {
	WorkspaceRoot string
	ProjectRoot   string
}

type Summary struct {
	Verdict      Verdict
	Allowed      []string
	OutOfScope   []string
	Forbidden    []string
	UnknownScope []string
}

type BlockingReason struct {
	Code    ReasonCode
	Verdict Verdict
	Paths   []string
}

func (summary Summary) CanApply() bool {
	return summary.Verdict == Allowed && len(summary.Allowed) > 0
}

func (summary Summary) BlockingReason() BlockingReason {
	switch summary.Verdict {
	case Forbidden:
		return BlockingReason{
			Code:    ReasonForbidden,
			Verdict: Forbidden,
			Paths:   summary.Forbidden,
		}
	case UnknownScope:
		return BlockingReason{
			Code:    ReasonUnknownScope,
			Verdict: UnknownScope,
			Paths:   summary.UnknownScope,
		}
	case OutOfScope:
		return BlockingReason{
			Code:    ReasonOutOfScope,
			Verdict: OutOfScope,
			Paths:   summary.OutOfScope,
		}
	default:
		return BlockingReason{}
	}
}

func AuthorizeWorkspaceDiff(
	scope CommissionScope,
	changedPaths []string,
	facts PathFacts,
) Summary {
	summary := Summary{}

	for _, changedPath := range changedPaths {
		summary = summary.authorizePath(scope, changedPath, facts)
	}

	return summary.withVerdict(scope)
}

func (summary Summary) authorizePath(
	scope CommissionScope,
	changedPath string,
	facts PathFacts,
) Summary {
	normalizedPath, ok := normalizeChangedPath(changedPath, facts)
	if !ok {
		return summary.appendUnknown(changedPath)
	}

	if pathMatchesAny(scope.ForbiddenPaths, normalizedPath) {
		return summary.appendForbidden(normalizedPath)
	}

	if pathMatchesAny(scope.AllowedPaths, normalizedPath) {
		return summary.appendAllowed(normalizedPath)
	}

	if scope.hasAnyCarrier() {
		return summary.appendOutOfScope(normalizedPath)
	}

	return summary.appendUnknown(normalizedPath)
}

func (summary Summary) withVerdict(scope CommissionScope) Summary {
	switch {
	case len(summary.Forbidden) > 0:
		summary.Verdict = Forbidden
	case len(summary.UnknownScope) > 0:
		summary.Verdict = UnknownScope
	case len(summary.OutOfScope) > 0:
		summary.Verdict = OutOfScope
	case len(summary.Allowed) > 0:
		summary.Verdict = Allowed
	case scope.hasAnyCarrier():
		summary.Verdict = OutOfScope
	default:
		summary.Verdict = UnknownScope
	}

	return summary
}

func (summary Summary) appendAllowed(path string) Summary {
	summary.Allowed = appendUnique(summary.Allowed, path)
	return summary
}

func (summary Summary) appendOutOfScope(path string) Summary {
	summary.OutOfScope = appendUnique(summary.OutOfScope, path)
	return summary
}

func (summary Summary) appendForbidden(path string) Summary {
	summary.Forbidden = appendUnique(summary.Forbidden, path)
	return summary
}

func (summary Summary) appendUnknown(path string) Summary {
	summary.UnknownScope = appendUnique(summary.UnknownScope, path)
	return summary
}

func (scope CommissionScope) hasAnyCarrier() bool {
	return hasAny(scope.AllowedPaths) ||
		hasAny(scope.ForbiddenPaths) ||
		hasAny(scope.AffectedFiles) ||
		hasAny(scope.Lockset)
}

func normalizeChangedPath(changedPath string, facts PathFacts) (string, bool) {
	cleanedPath := strings.TrimSpace(changedPath)
	if cleanedPath == "" {
		return "", false
	}

	if filepath.IsAbs(cleanedPath) {
		return normalizeAbsoluteChangedPath(cleanedPath, facts)
	}

	return normalizeRelativeChangedPath(cleanedPath)
}

func normalizeAbsoluteChangedPath(changedPath string, facts PathFacts) (string, bool) {
	for _, root := range []string{facts.WorkspaceRoot, facts.ProjectRoot} {
		normalizedPath, ok := normalizeUnderRoot(changedPath, root)
		if ok {
			return normalizedPath, true
		}
	}

	return "", false
}

func normalizeUnderRoot(changedPath string, root string) (string, bool) {
	cleanedRoot := strings.TrimSpace(root)
	if cleanedRoot == "" {
		return "", false
	}

	relativePath, err := filepath.Rel(filepath.Clean(cleanedRoot), filepath.Clean(changedPath))
	if err != nil {
		return "", false
	}

	return normalizeRelativeChangedPath(relativePath)
}

func normalizeRelativeChangedPath(changedPath string) (string, bool) {
	cleanedPath := filepath.Clean(changedPath)
	slashPath := filepath.ToSlash(cleanedPath)
	normalizedPath := path.Clean(slashPath)

	if normalizedPath == "." {
		return "", false
	}

	if normalizedPath == ".." || strings.HasPrefix(normalizedPath, "../") {
		return "", false
	}

	if path.IsAbs(normalizedPath) {
		return "", false
	}

	return normalizedPath, true
}

func normalizeScopePattern(pattern string) (string, bool) {
	trimmedPattern := strings.TrimSpace(pattern)
	if trimmedPattern == "" {
		return "", false
	}

	slashPattern := filepath.ToSlash(trimmedPattern)
	normalizedPattern := path.Clean(slashPattern)
	if normalizedPattern == "." {
		return ".", true
	}

	if normalizedPattern == ".." || strings.HasPrefix(normalizedPattern, "../") {
		return "", false
	}

	if path.IsAbs(normalizedPattern) {
		return "", false
	}

	return normalizedPattern, true
}

func pathMatchesAny(patterns []string, changedPath string) bool {
	for _, pattern := range patterns {
		if pathMatches(pattern, changedPath) {
			return true
		}
	}

	return false
}

func pathMatches(pattern string, changedPath string) bool {
	normalizedPattern, ok := normalizeScopePattern(pattern)
	if !ok {
		return false
	}

	switch {
	case normalizedPattern == ".":
		return true
	case normalizedPattern == "**/*":
		return true
	case strings.HasSuffix(normalizedPattern, "/**"):
		prefix := strings.TrimSuffix(normalizedPattern, "/**")
		return changedPath == prefix || strings.HasPrefix(changedPath, prefix+"/")
	case strings.Contains(normalizedPattern, "*"):
		return globRegex(normalizedPattern).MatchString(changedPath)
	default:
		return changedPath == normalizedPattern
	}
}

func globRegex(pattern string) *regexp.Regexp {
	quotedPattern := regexp.QuoteMeta(pattern)
	doubleStarPattern := strings.ReplaceAll(quotedPattern, "\\*\\*", ".*")
	regexPattern := strings.ReplaceAll(doubleStarPattern, "\\*", "[^/]*")

	return regexp.MustCompile("^" + regexPattern + "$")
}

func hasAny(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}

	return false
}

func appendUnique(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}

	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}
