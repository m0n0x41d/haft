package projecttypeenv

import "fmt"

func requiredDeclarationFact(
	declaration SymbolicDeclaration,
	path string,
) (SourceScalar, error) {
	values := factsAtPath(declaration.facts, path)
	if len(values) != 1 {
		return SourceScalar{}, fmt.Errorf(
			"lower %q requires exactly one fact %q",
			declaration.Symbol().Value(),
			path,
		)
	}
	return values[0], nil
}

func indexedDeclarationFacts(
	declaration SymbolicDeclaration,
	prefix string,
) ([]SourceScalar, error) {
	byIndex := make(map[int]SourceScalar)
	for _, fact := range declaration.facts {
		index, matches := parseIndexedScalarPath(fact.path, prefix)
		if matches {
			byIndex[index] = fact.value
		}
	}
	result := make([]SourceScalar, 0, len(byIndex))
	for index := 0; index < len(byIndex); index++ {
		value, exists := byIndex[index]
		if !exists {
			return nil, fmt.Errorf(
				"lower %q %s indices are not dense",
				declaration.Symbol().Value(),
				prefix,
			)
		}
		result = append(result, value)
	}
	return result, nil
}
