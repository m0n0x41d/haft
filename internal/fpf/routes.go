package fpf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
