package fpf

import (
	"strings"
	"testing"
)

func TestEngineTermDefinitionsAreComplete(t *testing.T) {
	definitions := EngineTermDefinitions()
	if len(definitions) < 10 {
		t.Fatalf("term definitions = %d, want canonical C0 term set", len(definitions))
	}

	objectKinds := map[string]string{}
	for _, definition := range definitions {
		if strings.TrimSpace(definition.Term) == "" {
			t.Fatalf("term has empty name: %#v", definition)
		}
		if strings.TrimSpace(definition.ObjectKind) == "" {
			t.Fatalf("%s object_kind missing", definition.Term)
		}
		if strings.TrimSpace(definition.Definition) == "" {
			t.Fatalf("%s definition missing", definition.Term)
		}
		if len(definition.MustNotMean) == 0 {
			t.Fatalf("%s must_not_mean missing", definition.Term)
		}
		if previous, ok := objectKinds[definition.ObjectKind]; ok {
			t.Fatalf("%s and %s share object_kind %q", previous, definition.Term, definition.ObjectKind)
		}
		objectKinds[definition.ObjectKind] = definition.Term
	}
}

func TestPatternPullIsDeprecatedFormalTerm(t *testing.T) {
	definition, ok := EngineTermByName("PatternPull")
	if !ok {
		t.Fatal("PatternPull deprecation term missing")
	}
	if !definition.Deprecated {
		t.Fatal("PatternPull must be marked deprecated")
	}
	if len(definition.ReplacementTerms) == 0 {
		t.Fatal("PatternPull needs replacement terms")
	}
	for _, forbidden := range []string{"runtime API", "support class", "MethodPack pull"} {
		if !stringSliceContains(definition.MustNotMean, forbidden) {
			t.Fatalf("PatternPull must_not_mean missing %q: %#v", forbidden, definition.MustNotMean)
		}
	}
}

func TestPatternAtlasTermBlocksAuthorityMeanings(t *testing.T) {
	definition, ok := EngineTermByName("PatternAtlas")
	if !ok {
		t.Fatal("PatternAtlas term missing")
	}
	for _, forbidden := range []string{"route selector", "evidence source", "gate passage", "approval mechanism"} {
		if !stringSliceContains(definition.MustNotMean, forbidden) {
			t.Fatalf("PatternAtlas must_not_mean missing %q: %#v", forbidden, definition.MustNotMean)
		}
	}
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
