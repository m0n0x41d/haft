package projectpath

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

// Path is one canonical, project-relative, slash-separated path. It is purely
// lexical: constructing a Path does not touch the filesystem.
type Path struct {
	value string
}

// ModulePath is one canonical module path. The empty value is the explicit
// project-root module; non-root values obey the same contract as Path.
type ModulePath struct {
	value string
}

// ModuleRef pairs one indexed module identity with its canonical path. The
// fields stay private so callers cannot construct an invalid module path.
type ModuleRef struct {
	id   string
	path ModulePath
}

// Parse canonicalizes one weak path at the project boundary.
func Parse(raw string) (Path, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	canonical := path.Clean(normalized)
	if invalidProjectPath(normalized, canonical) {
		return Path{}, fmt.Errorf(
			"path %q must be a canonical project-relative path",
			raw,
		)
	}
	return Path{value: canonical}, nil
}

// ParseModule canonicalizes a module path. An empty string denotes the root
// module and is never a valid ordinary Path.
func ParseModule(raw string) (ModulePath, error) {
	if strings.TrimSpace(raw) == "" {
		return ModulePath{}, nil
	}
	parsed, err := Parse(raw)
	if err != nil {
		return ModulePath{}, err
	}
	return ModulePath(parsed), nil
}

// NewModuleRef parses one weak indexed-module row at the shared path boundary.
func NewModuleRef(id string, rawPath string) (ModuleRef, error) {
	canonicalID := strings.TrimSpace(id)
	if canonicalID == "" {
		return ModuleRef{}, fmt.Errorf("module identity is required")
	}
	modulePath, err := ParseModule(rawPath)
	if err != nil {
		return ModuleRef{}, err
	}
	return ModuleRef{
		id:   canonicalID,
		path: modulePath,
	}, nil
}

func (p Path) String() string {
	return p.value
}

func (p ModulePath) String() string {
	return p.value
}

func (m ModuleRef) ID() string {
	return m.id
}

func (m ModuleRef) Path() ModulePath {
	return m.path
}

// Contains reports segment-safe module membership. The root module contains
// every valid project path; "internal/cli" does not contain "internal/client".
func (m ModulePath) Contains(candidate Path) bool {
	if m.value == "" {
		return true
	}
	return candidate.value == m.value ||
		strings.HasPrefix(candidate.value, m.value+"/")
}

// ResolveMostSpecificModule returns the longest segment-safe containing
// module. Duplicate identities for the same canonical module path fail closed
// instead of making query order an authority selector.
func ResolveMostSpecificModule(
	modules []ModuleRef,
	candidate Path,
) (ModuleRef, bool, error) {
	var result ModuleRef
	bestLength := -1
	for _, module := range modules {
		if module.id == "" {
			return ModuleRef{}, false, fmt.Errorf(
				"module reference is not initialized",
			)
		}
		if !module.path.Contains(candidate) {
			continue
		}
		pathLength := len(module.path.String())
		if pathLength < bestLength {
			continue
		}
		if pathLength == bestLength {
			if module.path.String() == result.path.String() &&
				module.id != result.id {
				return ModuleRef{}, false, fmt.Errorf(
					"canonical module path %q has ambiguous identities %q and %q",
					module.path.String(),
					result.id,
					module.id,
				)
			}
			continue
		}
		result = module
		bestLength = pathLength
	}
	return result, bestLength >= 0, nil
}

// ResolveExisting resolves symlinks for an existing project path and rejects a
// target outside the physical project root. Callers should use the returned
// path for the immediate effect instead of joining the weak input again.
func ResolveExisting(root string, candidate Path) (string, error) {
	resolvedRoot, err := resolveRoot(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(resolvedRoot, filepath.FromSlash(candidate.value))
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	if !within(resolvedRoot, resolved) {
		return "", fmt.Errorf(
			"project path %q resolves outside project root",
			candidate.value,
		)
	}
	return resolved, nil
}

// ResolvePotential resolves the nearest existing ancestor before accepting a
// path that may not exist yet. This prevents a missing child beneath an
// escaping symlink from being treated as project-contained.
func ResolvePotential(root string, candidate Path) (string, error) {
	resolvedRoot, err := resolveRoot(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(resolvedRoot, filepath.FromSlash(candidate.value))
	existing, suffix, err := nearestExisting(joined)
	if err != nil {
		return "", err
	}
	resolvedAncestor, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	resolved := filepath.Join(append([]string{resolvedAncestor}, suffix...)...)
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	if !within(resolvedRoot, resolved) {
		return "", fmt.Errorf(
			"project path %q resolves outside project root",
			candidate.value,
		)
	}
	return resolved, nil
}

func invalidProjectPath(normalized, canonical string) bool {
	if normalized == "" || canonical == "." {
		return true
	}
	if path.IsAbs(canonical) || windowsDriveAbsolute(normalized) {
		return true
	}
	if canonical == ".." || strings.HasPrefix(canonical, "../") {
		return true
	}
	return strings.ContainsFunc(canonical, unicode.IsControl)
}

func windowsDriveAbsolute(value string) bool {
	if len(value) < 2 {
		return false
	}
	drive := value[0]
	isLetter := drive >= 'a' && drive <= 'z' || drive >= 'A' && drive <= 'Z'
	return isLetter && value[1] == ':'
}

func resolveRoot(root string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(strings.TrimSpace(root)))
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

func nearestExisting(candidate string) (string, []string, error) {
	current := candidate
	suffix := make([]string, 0)
	for {
		_, err := os.Lstat(current)
		if err == nil {
			return current, suffix, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil, fmt.Errorf(
				"no existing ancestor for %s",
				candidate,
			)
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		current = parent
	}
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
