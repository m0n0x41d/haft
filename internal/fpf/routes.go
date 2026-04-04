package fpf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type routeArtifactFile struct {
	Routes []Route `json:"routes"`
}

// LoadRoutes reads route definitions from a build-time artifact file.
func LoadRoutes(path string) ([]Route, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open routes file: %w", err)
	}
	defer func() { _ = file.Close() }()

	routes, err := ParseRoutes(file)
	if err != nil {
		return nil, fmt.Errorf("parse routes file: %w", err)
	}

	return routes, nil
}

// ParseRoutes decodes route definitions from JSON.
func ParseRoutes(r io.Reader) ([]Route, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	artifact := routeArtifactFile{}
	if err := decoder.Decode(&artifact); err != nil {
		return nil, err
	}

	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("unexpected trailing content in route artifact")
		}
		return nil, err
	}

	routes := make([]Route, 0, len(artifact.Routes))
	for _, route := range artifact.Routes {
		routes = append(routes, normalizeRoute(route))
	}

	if err := validateRouteShape(routes); err != nil {
		return nil, err
	}

	return routes, nil
}

func normalizeRoute(route Route) Route {
	route.ID = strings.TrimSpace(route.ID)
	route.Title = strings.TrimSpace(route.Title)
	route.Description = strings.TrimSpace(route.Description)
	route.Matchers = dedupeStrings(route.Matchers)
	route.Core = normalizeRoutePatternIDs(route.Core)
	route.Chain = normalizeRoutePatternIDs(route.Chain)
	return route
}

func normalizeRoutePatternIDs(patternIDs []string) []string {
	normalized := make([]string, 0, len(patternIDs))
	for _, patternID := range patternIDs {
		normalized = append(normalized, normalizePatternID(patternID))
	}
	return dedupeStrings(normalized)
}

func validateRouteShape(routes []Route) error {
	issues := make([]string, 0)

	for _, route := range routes {
		routeRef := routeReference(route)
		if len(route.Matchers) == 0 {
			issues = append(issues, fmt.Sprintf("route %q must define at least one matcher", routeRef))
		}

		chainSet := makeStringSet(route.Chain)
		for _, patternID := range route.Core {
			if _, ok := chainSet[patternID]; ok {
				continue
			}
			issues = append(issues, fmt.Sprintf("route %q core pattern %q must also appear in chain", routeRef, patternID))
		}
	}

	return joinRouteValidationIssues(issues)
}

func validateRouteReferences(routes []Route, chunks []SpecChunk) error {
	indexedPatternIDs := collectChunkPatternIDs(chunks)
	issues := make([]string, 0)

	for _, route := range routes {
		routeRef := routeReference(route)
		referencedPatternIDs := append([]string{}, route.Core...)
		referencedPatternIDs = append(referencedPatternIDs, route.Chain...)
		referencedPatternIDs = dedupeStrings(referencedPatternIDs)

		for _, patternID := range referencedPatternIDs {
			if _, ok := indexedPatternIDs[patternID]; ok {
				continue
			}
			issues = append(issues, fmt.Sprintf("route %q references unknown section %q", routeRef, patternID))
		}
	}

	return joinRouteValidationIssues(issues)
}

func routeReference(route Route) string {
	if route.ID != "" {
		return route.ID
	}
	if route.Title != "" {
		return route.Title
	}
	return "<unnamed>"
}

func collectChunkPatternIDs(chunks []SpecChunk) map[string]struct{} {
	patternIDs := make(map[string]struct{}, len(chunks))

	for _, chunk := range chunks {
		if chunk.PatternID == "" {
			continue
		}
		patternIDs[chunk.PatternID] = struct{}{}
	}

	return patternIDs
}

func makeStringSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))

	for _, item := range items {
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}

	return set
}

func joinRouteValidationIssues(issues []string) error {
	if len(issues) == 0 {
		return nil
	}

	sort.Strings(issues)
	return fmt.Errorf("invalid route artifact: %s", strings.Join(issues, "; "))
}
