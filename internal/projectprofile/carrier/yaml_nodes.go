package carrier

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func rejectUnsafeYAML(node *yaml.Node) error {
	if node == nil {
		return fmt.Errorf("project profile YAML contains an absent node")
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fmt.Errorf("project profile YAML aliases and anchors are not supported")
	}
	if node.Tag == "!!null" {
		return fmt.Errorf("project profile YAML null values are not supported")
	}
	for _, child := range node.Content {
		if err := rejectUnsafeYAML(child); err != nil {
			return err
		}
	}
	return nil
}

func mappingFields(
	node *yaml.Node,
	context string,
	required []string,
	optional []string,
) (map[string]*yaml.Node, error) {
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" {
		return nil, fmt.Errorf("%s must be a mapping", context)
	}
	allowed := make(map[string]struct{}, len(required)+len(optional))
	for _, name := range required {
		allowed[name] = struct{}{}
	}
	for _, name := range optional {
		allowed[name] = struct{}{}
	}
	result := make(map[string]*yaml.Node, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
			return nil, fmt.Errorf("%s contains a non-string field name", context)
		}
		name := keyNode.Value
		if _, exists := allowed[name]; !exists {
			return nil, fmt.Errorf("%s contains unknown field %q", context, name)
		}
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("%s contains duplicate field %q", context, name)
		}
		result[name] = valueNode
	}
	for _, name := range required {
		if _, exists := result[name]; !exists {
			return nil, fmt.Errorf("%s is missing required field %q", context, name)
		}
	}
	return result, nil
}

func stringScalar(node *yaml.Node, context string) (string, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", fmt.Errorf("%s must be a string", context)
	}
	if node.Value == "" || node.Value != strings.TrimSpace(node.Value) {
		return "", fmt.Errorf("%s must be a non-empty string without surrounding whitespace", context)
	}
	return node.Value, nil
}

func sequenceItems(node *yaml.Node, context string) ([]*yaml.Node, error) {
	if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" {
		return nil, fmt.Errorf("%s must be a sequence", context)
	}
	return append([]*yaml.Node{}, node.Content...), nil
}
