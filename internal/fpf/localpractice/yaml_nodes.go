package localpractice

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const maximumCarrierBytes = 4 << 20

func decodeSingleDocument(source []byte) (*yaml.Node, error) {
	if len(source) > maximumCarrierBytes {
		return nil, fmt.Errorf("local-practice carrier exceeds %d bytes", maximumCarrierBytes)
	}
	if len(bytes.TrimSpace(source)) == 0 {
		return nil, fmt.Errorf("local-practice carrier is empty")
	}
	if !utf8.Valid(source) {
		return nil, fmt.Errorf("local-practice carrier must be valid UTF-8")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(source))
	document := yaml.Node{}
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode local-practice YAML: %w", err)
	}

	trailing := yaml.Node{}
	err := decoder.Decode(&trailing)
	if err == nil {
		return nil, fmt.Errorf("local-practice carrier must contain exactly one YAML document")
	}
	if err != io.EOF {
		return nil, fmt.Errorf("decode trailing local-practice YAML: %w", err)
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("local-practice carrier has no document root")
	}
	return document.Content[0], nil
}

func rejectUnsafeYAML(node *yaml.Node, source []byte) error {
	lines := bytes.Split(source, []byte("\n"))
	return rejectUnsafeYAMLNode(node, lines)
}

func rejectUnsafeYAMLNode(node *yaml.Node, sourceLines [][]byte) error {
	if node == nil {
		return fmt.Errorf("local-practice YAML contains an absent node")
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fmt.Errorf("local-practice YAML aliases and anchors are not supported")
	}
	if node.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf("local-practice YAML explicit tags are not supported at line %d", node.Line)
	}
	if node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return fmt.Errorf("local-practice YAML block scalar styles are not supported at line %d", node.Line)
	}
	if node.Tag == "!!null" {
		return fmt.Errorf("local-practice YAML null values are not supported at line %d", node.Line)
	}
	if node.Kind == yaml.ScalarNode {
		if err := rejectMultilineScalar(node, sourceLines); err != nil {
			return err
		}
	}
	for _, child := range node.Content {
		if err := rejectUnsafeYAMLNode(child, sourceLines); err != nil {
			return err
		}
	}
	return nil
}

func rejectMultilineScalar(node *yaml.Node, sourceLines [][]byte) error {
	line, err := scalarSourceLine(node, sourceLines)
	if err != nil {
		return err
	}
	if scalarEndsOnSourceLine(node, line) {
		return nil
	}
	return fmt.Errorf(
		"local-practice YAML semantic scalars must occupy one physical line at line %d",
		node.Line,
	)
}

func scalarSourceLine(node *yaml.Node, sourceLines [][]byte) (string, error) {
	if node.Line <= 0 || node.Line > len(sourceLines) {
		return "", fmt.Errorf("local-practice YAML scalar has an invalid source line %d", node.Line)
	}
	line := strings.TrimSuffix(string(sourceLines[node.Line-1]), "\r")
	runes := []rune(line)
	column := node.Column - 1
	if column < 0 || column > len(runes) {
		return "", fmt.Errorf(
			"local-practice YAML scalar has an invalid source column %d at line %d",
			node.Column,
			node.Line,
		)
	}
	return string(runes[column:]), nil
}

func scalarEndsOnSourceLine(node *yaml.Node, line string) bool {
	switch {
	case node.Style&yaml.DoubleQuotedStyle != 0:
		return doubleQuotedScalarCloses(line)
	case node.Style&yaml.SingleQuotedStyle != 0:
		return singleQuotedScalarCloses(line)
	default:
		return strings.Contains(line, node.Value)
	}
}

func doubleQuotedScalarCloses(line string) bool {
	if !strings.HasPrefix(line, "\"") {
		return false
	}
	escaped := false
	for index := 1; index < len(line); index++ {
		character := line[index]
		if character == '\\' {
			escaped = !escaped
			continue
		}
		if character == '"' && !escaped {
			return true
		}
		escaped = false
	}
	return false
}

func singleQuotedScalarCloses(line string) bool {
	if !strings.HasPrefix(line, "'") {
		return false
	}
	for index := 1; index < len(line); index++ {
		if line[index] != '\'' {
			continue
		}
		if index+1 < len(line) && line[index+1] == '\'' {
			index++
			continue
		}
		return true
	}
	return false
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

	fields := make(map[string]*yaml.Node, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
			return nil, fmt.Errorf("%s contains a non-string field name", context)
		}
		name := keyNode.Value
		if !utf8.ValidString(name) {
			return nil, fmt.Errorf("%s contains an invalid UTF-8 field name", context)
		}
		if strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("%s contains a control character in a field name", context)
		}
		if _, exists := allowed[name]; !exists {
			return nil, fmt.Errorf("%s contains unknown field %q", context, name)
		}
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("%s contains duplicate field %q", context, name)
		}
		fields[name] = valueNode
	}
	for _, name := range required {
		if _, exists := fields[name]; !exists {
			return nil, fmt.Errorf("%s is missing required field %q", context, name)
		}
	}
	return fields, nil
}

func sequenceItems(node *yaml.Node, context string, allowEmpty bool) ([]*yaml.Node, error) {
	if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" {
		return nil, fmt.Errorf("%s must be a sequence", context)
	}
	if !allowEmpty && len(node.Content) == 0 {
		return nil, fmt.Errorf("%s must contain at least one item", context)
	}
	return append([]*yaml.Node(nil), node.Content...), nil
}

func sourceText(node *yaml.Node, context string) (SourceText, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return SourceText{}, fmt.Errorf("%s must be a string", context)
	}
	value := node.Value
	if !utf8.ValidString(value) {
		return SourceText{}, fmt.Errorf("%s must be valid UTF-8", context)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return SourceText{}, fmt.Errorf("%s must not contain control characters", context)
	}
	if value == "" || value != strings.TrimSpace(value) {
		return SourceText{}, fmt.Errorf("%s must be a non-empty string without surrounding whitespace", context)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return SourceText{}, fmt.Errorf("%s source span: %w", context, err)
	}
	return SourceText{value: value, span: span}, nil
}

func qualifiedSourceText(node *yaml.Node, context string) (SourceText, error) {
	value, err := sourceText(node, context)
	if err != nil {
		return SourceText{}, err
	}
	if strings.ContainsAny(value.value, " /\\") {
		return SourceText{}, fmt.Errorf("%s must not contain whitespace, slash, or backslash", context)
	}
	return value, nil
}

func unsignedScalar(node *yaml.Node, context string) (uint64, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return 0, fmt.Errorf("%s must be an unsigned decimal integer", context)
	}
	value := node.Value
	if value == "" {
		return 0, fmt.Errorf("%s must be an unsigned decimal integer", context)
	}
	if value != "0" && strings.HasPrefix(value, "0") {
		return 0, fmt.Errorf("%s must use canonical decimal notation", context)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("%s must use canonical decimal notation", context)
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", context, err)
	}
	return parsed, nil
}

func nodeLineRange(node *yaml.Node) (SourceLineRange, error) {
	if node == nil || node.Line <= 0 {
		return SourceLineRange{}, fmt.Errorf("source node has no positive start line")
	}
	end := nodeEndLine(node)
	startLine, err := positiveSourceLine(node.Line)
	if err != nil {
		return SourceLineRange{}, fmt.Errorf("source node start line: %w", err)
	}
	endLine, err := positiveSourceLine(end)
	if err != nil {
		return SourceLineRange{}, fmt.Errorf("source node end line: %w", err)
	}
	return newSourceLineRange(startLine, endLine)
}

func mappingFieldLineRange(mapping *yaml.Node, name string) (SourceLineRange, error) {
	if mapping == nil || mapping.Kind != yaml.MappingNode || mapping.Tag != "!!map" {
		return SourceLineRange{}, fmt.Errorf("source parent for field %q is not a mapping", name)
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		keyNode := mapping.Content[index]
		if keyNode.Value != name {
			continue
		}
		valueNode := mapping.Content[index+1]
		end := nodeEndLine(valueNode)
		startLine, err := positiveSourceLine(keyNode.Line)
		if err != nil {
			return SourceLineRange{}, fmt.Errorf("source field %q start line: %w", name, err)
		}
		endLine, err := positiveSourceLine(end)
		if err != nil {
			return SourceLineRange{}, fmt.Errorf("source field %q end line: %w", name, err)
		}
		return newSourceLineRange(startLine, endLine)
	}
	return SourceLineRange{}, fmt.Errorf("source parent has no field %q", name)
}

func positiveSourceLine(value int) (uint64, error) {
	if value <= 0 {
		return 0, fmt.Errorf("line must be positive")
	}
	encoded := strconv.Itoa(value)
	line, err := strconv.ParseUint(encoded, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("line %q does not fit the canonical source range: %w", encoded, err)
	}
	return line, nil
}

func nodeEndLine(node *yaml.Node) int {
	end := node.Line
	if node.Kind == yaml.ScalarNode {
		end += strings.Count(node.Value, "\n")
	}
	for _, child := range node.Content {
		childEnd := nodeEndLine(child)
		if childEnd > end {
			end = childEnd
		}
	}
	return end
}

func digestSource(source []byte) SourceDigest {
	digest := sha256.Sum256(source)
	encoded := hex.EncodeToString(digest[:])
	return SourceDigest{value: "sha256:" + encoded}
}
