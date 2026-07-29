package initplanning

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var yamlPlainKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type managedYAMLDocument struct {
	raw        []byte
	root       *yaml.Node
	lineStarts []int
	newline    string
}

type managedYAMLPathStep struct {
	mapping *yaml.Node
	key     *yaml.Node
	value   *yaml.Node
}

type managedYAMLPathLookup struct {
	node    *yaml.Node
	found   bool
	steps   []managedYAMLPathStep
	missing int
}

type managedYAMLByteSpan struct {
	start int
	end   int
}

func canonicalYAMLPointer(
	raw []string,
) ([]string, string, error) {
	path, pointer, err := canonicalJSONPointer(raw)
	if err != nil {
		message := strings.NewReplacer(
			"JSON",
			"YAML",
		).Replace(err.Error())
		return nil, "", fmt.Errorf("%s", message)
	}
	for _, token := range path {
		if !yamlPlainKeyPattern.MatchString(token) {
			return nil, "", fmt.Errorf(
				"managed YAML selector token %q is not a plain key",
				token,
			)
		}
	}
	return path, pointer, nil
}

func canonicalYAMLValue(
	raw []byte,
) ([]byte, error) {
	document, err := parseManagedYAMLDocument(raw)
	if err != nil {
		return nil, err
	}
	if document.root.Kind == yaml.ScalarNode &&
		document.root.Tag == "!!null" {
		return nil, fmt.Errorf("managed YAML fragment cannot be null")
	}
	return canonicalYAMLNodeBytes(document.root)
}

func parseManagedYAMLDocument(
	raw []byte,
) (managedYAMLDocument, error) {
	newline, err := managedYAMLNewline(raw)
	if err != nil {
		return managedYAMLDocument{}, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		root := &yaml.Node{
			Kind:   yaml.MappingNode,
			Tag:    "!!map",
			Line:   1,
			Column: 1,
		}
		return managedYAMLDocument{
			raw:        slices.Clone(raw),
			root:       root,
			lineStarts: managedYAMLLineStarts(raw),
			newline:    newline,
		}, nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var document yaml.Node
	decodeErr := decoder.Decode(&document)
	if decodeErr == io.EOF {
		root := &yaml.Node{
			Kind:   yaml.MappingNode,
			Tag:    "!!map",
			Line:   1,
			Column: 1,
		}
		return managedYAMLDocument{
			raw:        slices.Clone(raw),
			root:       root,
			lineStarts: managedYAMLLineStarts(raw),
			newline:    newline,
		}, nil
	}
	if decodeErr != nil {
		return managedYAMLDocument{}, fmt.Errorf(
			"decode managed YAML carrier: %w",
			decodeErr,
		)
	}
	var trailing yaml.Node
	trailingErr := decoder.Decode(&trailing)
	if trailingErr == nil {
		return managedYAMLDocument{}, fmt.Errorf(
			"managed YAML carrier must contain exactly one document",
		)
	}
	if trailingErr != io.EOF {
		return managedYAMLDocument{}, fmt.Errorf(
			"decode trailing managed YAML document: %w",
			trailingErr,
		)
	}
	if document.Kind != yaml.DocumentNode {
		return managedYAMLDocument{}, fmt.Errorf(
			"managed YAML carrier must contain exactly one document",
		)
	}
	if len(document.Content) == 0 {
		root := &yaml.Node{
			Kind:   yaml.MappingNode,
			Tag:    "!!map",
			Line:   1,
			Column: 1,
		}
		return managedYAMLDocument{
			raw:        slices.Clone(raw),
			root:       root,
			lineStarts: managedYAMLLineStarts(raw),
			newline:    newline,
		}, nil
	}
	if len(document.Content) != 1 {
		return managedYAMLDocument{}, fmt.Errorf(
			"managed YAML carrier document root is ambiguous",
		)
	}
	root := document.Content[0]
	if err := validateManagedYAMLNode(root); err != nil {
		return managedYAMLDocument{}, err
	}
	return managedYAMLDocument{
		raw:        slices.Clone(raw),
		root:       root,
		lineStarts: managedYAMLLineStarts(raw),
		newline:    newline,
	}, nil
}

func parseManagedYAMLCarrierDocument(
	raw []byte,
) (managedYAMLDocument, error) {
	document, err := parseManagedYAMLDocument(raw)
	if err != nil {
		return managedYAMLDocument{}, err
	}
	if document.root.Kind != yaml.MappingNode {
		return managedYAMLDocument{}, fmt.Errorf(
			"managed YAML carrier root must be a mapping",
		)
	}
	return document, nil
}

func managedYAMLNewline(
	raw []byte,
) (string, error) {
	withoutCRLF := bytes.ReplaceAll(raw, []byte("\r\n"), nil)
	if bytes.Contains(withoutCRLF, []byte{'\r'}) {
		return "", fmt.Errorf(
			"managed YAML carrier uses unsupported bare carriage returns",
		)
	}
	hasCRLF := bytes.Contains(raw, []byte("\r\n"))
	hasLF := bytes.Contains(withoutCRLF, []byte{'\n'})
	if hasCRLF && hasLF {
		return "", fmt.Errorf(
			"managed YAML carrier mixes line-ending editions",
		)
	}
	if hasCRLF {
		return "\r\n", nil
	}
	return "\n", nil
}

func managedYAMLLineStarts(
	raw []byte,
) []int {
	starts := []int{0}
	for index, value := range raw {
		if value == '\n' && index+1 < len(raw) {
			starts = append(starts, index+1)
		}
	}
	return starts
}

func validateManagedYAMLNode(
	node *yaml.Node,
) error {
	if node == nil {
		return fmt.Errorf("managed YAML node is unavailable")
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fmt.Errorf(
			"YAML aliases and anchors are unsupported in managed carriers",
		)
	}
	if node.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf(
			"explicit YAML tags are unsupported in managed carriers",
		)
	}
	if node.Kind == yaml.MappingNode {
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("managed YAML mapping is malformed")
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			value := node.Content[index+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return fmt.Errorf(
					"managed YAML mapping keys must be strings",
				)
			}
			if key.Value == "<<" {
				return fmt.Errorf(
					"YAML merge keys are unsupported in managed carriers",
				)
			}
			if _, duplicate := seen[key.Value]; duplicate {
				return fmt.Errorf(
					"duplicate YAML mapping key %q",
					key.Value,
				)
			}
			seen[key.Value] = struct{}{}
			if err := validateManagedYAMLNode(key); err != nil {
				return err
			}
			if err := validateManagedYAMLNode(value); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			if err := validateManagedYAMLNode(child); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf(
			"managed YAML carrier contains unsupported node kind %d",
			node.Kind,
		)
	}
	return nil
}

func canonicalYAMLNodeBytes(
	node *yaml.Node,
) ([]byte, error) {
	canonical, err := cloneCanonicalYAMLNode(node)
	if err != nil {
		return nil, err
	}
	content, err := yaml.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode canonical YAML value: %w", err)
	}
	return content, nil
}

func cloneCanonicalYAMLNode(
	node *yaml.Node,
) (*yaml.Node, error) {
	if err := validateManagedYAMLNode(node); err != nil {
		return nil, err
	}
	clone := &yaml.Node{
		Kind:        node.Kind,
		Tag:         node.Tag,
		Value:       node.Value,
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
	}
	if node.Kind == yaml.MappingNode {
		type pair struct {
			key   *yaml.Node
			value *yaml.Node
		}
		pairs := make([]pair, 0, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key, err := cloneCanonicalYAMLNode(node.Content[index])
			if err != nil {
				return nil, err
			}
			value, err := cloneCanonicalYAMLNode(node.Content[index+1])
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, pair{key: key, value: value})
		}
		sort.Slice(pairs, func(left int, right int) bool {
			return pairs[left].key.Value < pairs[right].key.Value
		})
		for _, candidate := range pairs {
			clone.Content = append(
				clone.Content,
				candidate.key,
				candidate.value,
			)
		}
		return clone, nil
	}
	for _, child := range node.Content {
		canonical, err := cloneCanonicalYAMLNode(child)
		if err != nil {
			return nil, err
		}
		clone.Content = append(clone.Content, canonical)
	}
	return clone, nil
}

func observeManagedYAML(
	probes []managedFragmentProbe,
	raw []byte,
) ([]ManagedFragmentObservation, error) {
	document, err := parseManagedYAMLCarrierDocument(raw)
	if err != nil {
		return nil, fmt.Errorf("observe managed YAML carrier: %w", err)
	}
	observations := make([]ManagedFragmentObservation, len(probes))
	for index, probe := range probes {
		observation, err := observeManagedYAMLProbe(document, probe)
		if err != nil {
			return nil, err
		}
		observations[index] = observation
	}
	return observations, nil
}

func observeManagedYAMLProbe(
	document managedYAMLDocument,
	probe managedFragmentProbe,
) (ManagedFragmentObservation, error) {
	lookup, err := lookupManagedYAMLPath(
		document.root,
		probe.coordinate.yamlPath,
	)
	if err != nil {
		return ManagedFragmentObservation{}, err
	}
	if !lookup.found {
		return missingManagedFragmentObservation(probe.coordinate), nil
	}
	if probe.coordinate.kind == ManagedYAMLMappingEntry {
		canonical, err := canonicalYAMLNodeBytes(lookup.node)
		if err != nil {
			return ManagedFragmentObservation{}, err
		}
		return presentManagedFragmentObservation(
			probe.coordinate,
			managedFragmentDigest(canonical),
		), nil
	}
	if probe.coordinate.kind == ManagedYAMLSequenceMember {
		if lookup.node.Kind != yaml.SequenceNode {
			return ManagedFragmentObservation{}, fmt.Errorf(
				"managed YAML selector %s must name a sequence",
				probe.coordinate.selector,
			)
		}
		if lookup.node.Style&yaml.FlowStyle != 0 {
			return ManagedFragmentObservation{}, fmt.Errorf(
				"managed YAML path uses flow style at %s",
				probe.coordinate.selector,
			)
		}
		digest, found, err := findKnownYAMLSequenceMember(
			lookup.node.Content,
			probe.candidateDigests,
			probe.coordinate,
		)
		if err != nil {
			return ManagedFragmentObservation{}, err
		}
		if !found {
			return missingManagedFragmentObservation(probe.coordinate), nil
		}
		return presentManagedFragmentObservation(
			probe.coordinate,
			digest,
		), nil
	}
	return ManagedFragmentObservation{}, fmt.Errorf(
		"managed YAML probe kind is invalid",
	)
}

func lookupManagedYAMLPath(
	root *yaml.Node,
	path []string,
) (managedYAMLPathLookup, error) {
	if len(path) == 0 {
		return managedYAMLPathLookup{}, fmt.Errorf(
			"managed YAML selector is empty",
		)
	}
	current := root
	steps := make([]managedYAMLPathStep, 0, len(path))
	for index, token := range path {
		if current.Kind != yaml.MappingNode {
			return managedYAMLPathLookup{}, fmt.Errorf(
				"managed YAML selector crosses non-mapping token %q",
				token,
			)
		}
		if current.Style&yaml.FlowStyle != 0 {
			return managedYAMLPathLookup{}, fmt.Errorf(
				"managed YAML path uses flow style before %q",
				token,
			)
		}
		key, value, found := managedYAMLMappingValue(current, token)
		if !found {
			return managedYAMLPathLookup{
				node:    current,
				steps:   steps,
				missing: index,
			}, nil
		}
		if value.Style&yaml.FlowStyle != 0 {
			return managedYAMLPathLookup{}, fmt.Errorf(
				"managed YAML path uses flow style at %q",
				token,
			)
		}
		steps = append(steps, managedYAMLPathStep{
			mapping: current,
			key:     key,
			value:   value,
		})
		current = value
	}
	return managedYAMLPathLookup{
		node:    current,
		found:   true,
		steps:   steps,
		missing: len(path),
	}, nil
}

func managedYAMLMappingValue(
	mapping *yaml.Node,
	token string,
) (*yaml.Node, *yaml.Node, bool) {
	for index := 0; index < len(mapping.Content); index += 2 {
		key := mapping.Content[index]
		if key.Value == token {
			return key, mapping.Content[index+1], true
		}
	}
	return nil, nil, false
}

func findKnownYAMLSequenceMember(
	members []*yaml.Node,
	candidates []string,
	coordinate ManagedFragmentCoordinate,
) (string, bool, error) {
	candidateSet := make(map[string]struct{}, len(candidates))
	for _, digest := range candidates {
		candidateSet[digest] = struct{}{}
	}
	matches := make([]string, 0, 1)
	for _, member := range members {
		canonical, err := canonicalYAMLNodeBytes(member)
		if err != nil {
			return "", false, err
		}
		digest := managedFragmentDigest(canonical)
		if _, known := candidateSet[digest]; known {
			matches = append(matches, digest)
		}
	}
	if len(matches) > 1 {
		return "", false, fmt.Errorf(
			"managed YAML sequence member %s/%s is ambiguous",
			coordinate.selector,
			coordinate.memberID,
		)
	}
	if len(matches) == 0 {
		return "", false, nil
	}
	return matches[0], true, nil
}

func applyManagedYAMLEffects(
	effects []ManagedFragmentEffect,
	input ManagedCarrierInput,
) ([]byte, error) {
	content := slices.Clone(input.content)
	for _, effect := range effects {
		if !managedFragmentEffectMutates(effect.kind) {
			continue
		}
		document, err := parseManagedYAMLCarrierDocument(content)
		if err != nil {
			return nil, fmt.Errorf("apply managed YAML carrier: %w", err)
		}
		if effect.coordinate.kind == ManagedYAMLMappingEntry {
			content, err = applyYAMLMappingEntryEffect(document, effect)
		}
		if effect.coordinate.kind == ManagedYAMLSequenceMember {
			content, err = applyYAMLSequenceMemberEffect(document, effect)
		}
		if err != nil {
			return nil, err
		}
		if content == nil {
			return nil, fmt.Errorf("managed YAML effect kind is invalid")
		}
	}
	return content, nil
}

func applyYAMLMappingEntryEffect(
	document managedYAMLDocument,
	effect ManagedFragmentEffect,
) ([]byte, error) {
	lookup, err := lookupManagedYAMLPath(
		document.root,
		effect.coordinate.yamlPath,
	)
	if err != nil {
		return nil, err
	}
	if effect.kind == ManagedFragmentCreate {
		if lookup.found {
			return nil, fmt.Errorf(
				"managed YAML create target is no longer vacant",
			)
		}
		if !effect.hasDesired {
			return nil, fmt.Errorf(
				"managed YAML create lacks desired fragment",
			)
		}
		return insertMissingYAMLMappingEntry(
			document,
			lookup,
			effect.coordinate.yamlPath,
			effect.desired.content,
		)
	}
	if !lookup.found || len(lookup.steps) == 0 {
		return nil, fmt.Errorf("managed YAML mapping target is missing")
	}
	if err := requireYAMLNodeDigest(
		lookup.node,
		effect.expectedDigest,
	); err != nil {
		return nil, err
	}
	if effect.kind == ManagedFragmentRemove {
		span, err := managedYAMLMappingRemovalSpan(document, lookup)
		if err != nil {
			return nil, err
		}
		return spliceManagedYAML(document.raw, span, nil), nil
	}
	if effect.kind != ManagedFragmentReplace || !effect.hasDesired {
		return nil, fmt.Errorf("managed YAML mapping effect is invalid")
	}
	key := lookup.steps[len(lookup.steps)-1].key
	span, err := managedYAMLMappingEntrySpan(document, key)
	if err != nil {
		return nil, err
	}
	replacement, err := renderYAMLMappingEntry(
		key.Value,
		effect.desired.content,
		key.Column-1,
		document.newline,
	)
	if err != nil {
		return nil, err
	}
	return spliceManagedYAML(document.raw, span, replacement), nil
}

func applyYAMLSequenceMemberEffect(
	document managedYAMLDocument,
	effect ManagedFragmentEffect,
) ([]byte, error) {
	lookup, err := lookupManagedYAMLPath(
		document.root,
		effect.coordinate.yamlPath,
	)
	if err != nil {
		return nil, err
	}
	if effect.kind == ManagedFragmentCreate {
		if !effect.hasDesired {
			return nil, fmt.Errorf(
				"managed YAML sequence create lacks desired fragment",
			)
		}
		if !lookup.found {
			return insertMissingYAMLSequenceMember(
				document,
				lookup,
				effect.coordinate.yamlPath,
				effect.desired.content,
			)
		}
		if lookup.node.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf(
				"managed YAML selector %s must name a sequence",
				effect.coordinate.selector,
			)
		}
		indent, err := managedYAMLSequenceIndent(lookup.node)
		if err != nil {
			return nil, err
		}
		member, err := renderYAMLSequenceMember(
			effect.desired.content,
			indent,
			document.newline,
		)
		if err != nil {
			return nil, err
		}
		offset, err := managedYAMLSequenceAppendOffset(
			document,
			lookup.node,
		)
		if err != nil {
			return nil, err
		}
		return insertManagedYAMLBlock(
			document.raw,
			offset,
			member,
			document.newline,
		), nil
	}
	if !lookup.found || lookup.node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("managed YAML sequence target is missing")
	}
	member, err := exactYAMLSequenceMember(
		lookup.node,
		effect.expectedDigest,
	)
	if err != nil {
		return nil, err
	}
	span, err := managedYAMLSequenceMemberSpan(document, member)
	if err != nil {
		return nil, err
	}
	if effect.kind == ManagedFragmentRemove {
		if len(lookup.node.Content) == 1 {
			span, err = managedYAMLMappingRemovalSpan(document, lookup)
			if err != nil {
				return nil, err
			}
		}
		return spliceManagedYAML(document.raw, span, nil), nil
	}
	if effect.kind != ManagedFragmentReplace || !effect.hasDesired {
		return nil, fmt.Errorf("managed YAML sequence effect is invalid")
	}
	indent, err := managedYAMLSequenceMemberIndent(document, member)
	if err != nil {
		return nil, err
	}
	replacement, err := renderYAMLSequenceMember(
		effect.desired.content,
		indent,
		document.newline,
	)
	if err != nil {
		return nil, err
	}
	return spliceManagedYAML(document.raw, span, replacement), nil
}

func requireYAMLNodeDigest(
	node *yaml.Node,
	expected string,
) error {
	canonical, err := canonicalYAMLNodeBytes(node)
	if err != nil {
		return err
	}
	if managedFragmentDigest(canonical) != expected {
		return fmt.Errorf("managed YAML fragment precondition changed")
	}
	return nil
}

func exactYAMLSequenceMember(
	sequence *yaml.Node,
	expectedDigest string,
) (*yaml.Node, error) {
	var found *yaml.Node
	for _, member := range sequence.Content {
		canonical, err := canonicalYAMLNodeBytes(member)
		if err != nil {
			return nil, err
		}
		if managedFragmentDigest(canonical) != expectedDigest {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf(
				"managed YAML sequence member is ambiguous",
			)
		}
		found = member
	}
	if found == nil {
		return nil, fmt.Errorf(
			"managed YAML sequence member precondition changed",
		)
	}
	return found, nil
}

func insertMissingYAMLMappingEntry(
	document managedYAMLDocument,
	lookup managedYAMLPathLookup,
	path []string,
	desired []byte,
) ([]byte, error) {
	parent, remainder, err := managedYAMLInsertionParent(
		document.root,
		lookup,
		path,
	)
	if err != nil {
		return nil, err
	}
	indent, offset, err := managedYAMLMappingInsertionPoint(
		document,
		parent,
	)
	if err != nil {
		return nil, err
	}
	block, err := renderYAMLNestedMappingEntry(
		remainder,
		desired,
		indent,
		document.newline,
	)
	if err != nil {
		return nil, err
	}
	return insertManagedYAMLBlock(
		document.raw,
		offset,
		block,
		document.newline,
	), nil
}

func insertMissingYAMLSequenceMember(
	document managedYAMLDocument,
	lookup managedYAMLPathLookup,
	path []string,
	desired []byte,
) ([]byte, error) {
	parent, remainder, err := managedYAMLInsertionParent(
		document.root,
		lookup,
		path,
	)
	if err != nil {
		return nil, err
	}
	indent, offset, err := managedYAMLMappingInsertionPoint(
		document,
		parent,
	)
	if err != nil {
		return nil, err
	}
	block, err := renderYAMLNestedSequenceMember(
		remainder,
		desired,
		indent,
		document.newline,
	)
	if err != nil {
		return nil, err
	}
	return insertManagedYAMLBlock(
		document.raw,
		offset,
		block,
		document.newline,
	), nil
}

func managedYAMLInsertionParent(
	root *yaml.Node,
	lookup managedYAMLPathLookup,
	path []string,
) (*yaml.Node, []string, error) {
	if lookup.found || lookup.missing >= len(path) {
		return nil, nil, fmt.Errorf(
			"managed YAML insertion target is not vacant",
		)
	}
	parent := lookup.node
	if parent == nil {
		parent = root
	}
	if parent.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf(
			"managed YAML insertion parent is not a mapping",
		)
	}
	if parent.Style&yaml.FlowStyle != 0 {
		return nil, nil, fmt.Errorf(
			"managed YAML path uses flow style at insertion parent",
		)
	}
	remainder := slices.Clone(path[lookup.missing:])
	if len(remainder) == 0 {
		return nil, nil, fmt.Errorf(
			"managed YAML insertion path is empty",
		)
	}
	return parent, remainder, nil
}

func managedYAMLMappingInsertionPoint(
	document managedYAMLDocument,
	mapping *yaml.Node,
) (int, int, error) {
	if len(mapping.Content) == 0 {
		if mapping != document.root {
			return 0, 0, fmt.Errorf(
				"empty nested YAML mappings are unsupported insertion parents",
			)
		}
		return 0, len(document.raw), nil
	}
	firstKey := mapping.Content[0]
	lastKey := mapping.Content[len(mapping.Content)-2]
	span, err := managedYAMLMappingEntrySpan(document, lastKey)
	if err != nil {
		return 0, 0, err
	}
	return firstKey.Column - 1, span.end, nil
}

func renderYAMLNestedMappingEntry(
	path []string,
	desired []byte,
	indent int,
	newline string,
) ([]byte, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("managed YAML mapping path is empty")
	}
	var output bytes.Buffer
	for index, token := range path {
		if !yamlPlainKeyPattern.MatchString(token) {
			return nil, fmt.Errorf(
				"managed YAML mapping key %q is not renderable",
				token,
			)
		}
		output.WriteString(strings.Repeat(" ", indent+index*2))
		output.WriteString(token)
		output.WriteString(":")
		output.WriteString(newline)
	}
	valueIndent := indent + len(path)*2
	value, err := indentCanonicalYAMLValue(
		desired,
		valueIndent,
		newline,
	)
	if err != nil {
		return nil, err
	}
	output.Write(value)
	return output.Bytes(), nil
}

func renderYAMLNestedSequenceMember(
	path []string,
	desired []byte,
	indent int,
	newline string,
) ([]byte, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("managed YAML sequence path is empty")
	}
	var output bytes.Buffer
	for index, token := range path {
		if !yamlPlainKeyPattern.MatchString(token) {
			return nil, fmt.Errorf(
				"managed YAML mapping key %q is not renderable",
				token,
			)
		}
		output.WriteString(strings.Repeat(" ", indent+index*2))
		output.WriteString(token)
		output.WriteString(":")
		output.WriteString(newline)
	}
	member, err := renderYAMLSequenceMember(
		desired,
		indent+len(path)*2,
		newline,
	)
	if err != nil {
		return nil, err
	}
	output.Write(member)
	return output.Bytes(), nil
}

func renderYAMLMappingEntry(
	key string,
	desired []byte,
	indent int,
	newline string,
) ([]byte, error) {
	return renderYAMLNestedMappingEntry(
		[]string{key},
		desired,
		indent,
		newline,
	)
}

func indentCanonicalYAMLValue(
	desired []byte,
	indent int,
	newline string,
) ([]byte, error) {
	document, err := parseManagedYAMLDocument(desired)
	if err != nil {
		return nil, err
	}
	canonical, err := canonicalYAMLNodeBytes(document.root)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(string(canonical), "\n")
	lines := strings.Split(text, "\n")
	var output bytes.Buffer
	for _, line := range lines {
		output.WriteString(strings.Repeat(" ", indent))
		output.WriteString(line)
		output.WriteString(newline)
	}
	return output.Bytes(), nil
}

func renderYAMLSequenceMember(
	desired []byte,
	indent int,
	newline string,
) ([]byte, error) {
	document, err := parseManagedYAMLDocument(desired)
	if err != nil {
		return nil, err
	}
	canonical, err := canonicalYAMLNodeBytes(document.root)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(string(canonical), "\n")
	if document.root.Kind == yaml.ScalarNode &&
		!strings.Contains(text, "\n") {
		return []byte(
			strings.Repeat(" ", indent) +
				"- " +
				text +
				newline,
		), nil
	}
	value, err := indentCanonicalYAMLValue(
		desired,
		indent+2,
		newline,
	)
	if err != nil {
		return nil, err
	}
	prefix := strings.Repeat(" ", indent) + "-" + newline
	return append([]byte(prefix), value...), nil
}

func managedYAMLMappingRemovalSpan(
	document managedYAMLDocument,
	lookup managedYAMLPathLookup,
) (managedYAMLByteSpan, error) {
	if len(lookup.steps) == 0 {
		return managedYAMLByteSpan{}, fmt.Errorf(
			"managed YAML mapping path has no removable entry",
		)
	}
	selected := len(lookup.steps) - 1
	for selected > 0 {
		parent := lookup.steps[selected].mapping
		if len(parent.Content) != 2 {
			break
		}
		selected--
	}
	return managedYAMLMappingEntrySpan(
		document,
		lookup.steps[selected].key,
	)
}

func managedYAMLMappingEntrySpan(
	document managedYAMLDocument,
	key *yaml.Node,
) (managedYAMLByteSpan, error) {
	if key == nil || key.Line < 1 || key.Column < 1 {
		return managedYAMLByteSpan{}, fmt.Errorf(
			"managed YAML mapping entry has no source position",
		)
	}
	line := key.Line - 1
	if line >= len(document.lineStarts) {
		return managedYAMLByteSpan{}, fmt.Errorf(
			"managed YAML mapping entry line is outside the carrier",
		)
	}
	start := document.lineStarts[line]
	end, err := managedYAMLBlockEnd(
		document,
		line+1,
		key.Column-1,
	)
	if err != nil {
		return managedYAMLByteSpan{}, err
	}
	return managedYAMLByteSpan{start: start, end: end}, nil
}

func managedYAMLSequenceMemberSpan(
	document managedYAMLDocument,
	member *yaml.Node,
) (managedYAMLByteSpan, error) {
	if member == nil || member.Line < 1 {
		return managedYAMLByteSpan{}, fmt.Errorf(
			"managed YAML sequence member has no source position",
		)
	}
	line := member.Line - 1
	if line >= len(document.lineStarts) {
		return managedYAMLByteSpan{}, fmt.Errorf(
			"managed YAML sequence member line is outside the carrier",
		)
	}
	rawLine := managedYAMLLine(document, line)
	indent, content, err := managedYAMLLineIndent(rawLine)
	if err != nil {
		return managedYAMLByteSpan{}, err
	}
	if !strings.HasPrefix(content, "-") {
		return managedYAMLByteSpan{}, fmt.Errorf(
			"managed YAML sequence member does not start at a block item",
		)
	}
	end, err := managedYAMLBlockEnd(document, line+1, indent)
	if err != nil {
		return managedYAMLByteSpan{}, err
	}
	return managedYAMLByteSpan{
		start: document.lineStarts[line],
		end:   end,
	}, nil
}

func managedYAMLBlockEnd(
	document managedYAMLDocument,
	fromLine int,
	indent int,
) (int, error) {
	trailingStart := -1
	for line := fromLine; line < len(document.lineStarts); line++ {
		rawLine := managedYAMLLine(document, line)
		lineIndent, content, err := managedYAMLLineIndent(rawLine)
		if err != nil {
			return 0, err
		}
		if content == "" {
			if trailingStart < 0 {
				trailingStart = document.lineStarts[line]
			}
			continue
		}
		if strings.HasPrefix(content, "#") && lineIndent <= indent {
			if trailingStart >= 0 {
				return trailingStart, nil
			}
			return document.lineStarts[line], nil
		}
		if lineIndent <= indent {
			if trailingStart >= 0 {
				return trailingStart, nil
			}
			return document.lineStarts[line], nil
		}
		trailingStart = -1
	}
	if trailingStart >= 0 {
		return trailingStart, nil
	}
	return len(document.raw), nil
}

func managedYAMLLine(
	document managedYAMLDocument,
	line int,
) []byte {
	start := document.lineStarts[line]
	end := len(document.raw)
	if line+1 < len(document.lineStarts) {
		end = document.lineStarts[line+1]
	}
	return document.raw[start:end]
}

func managedYAMLLineIndent(
	raw []byte,
) (int, string, error) {
	line := strings.TrimSuffix(string(raw), "\n")
	line = strings.TrimSuffix(line, "\r")
	indent := 0
	for _, value := range line {
		if value == ' ' {
			indent++
			continue
		}
		if value == '\t' {
			return 0, "", fmt.Errorf(
				"managed YAML carrier uses tab indentation",
			)
		}
		break
	}
	return indent, strings.TrimSpace(line), nil
}

func managedYAMLSequenceAppendOffset(
	document managedYAMLDocument,
	sequence *yaml.Node,
) (int, error) {
	if len(sequence.Content) == 0 {
		return 0, fmt.Errorf(
			"empty block YAML sequences are unsupported insertion parents",
		)
	}
	span, err := managedYAMLSequenceMemberSpan(
		document,
		sequence.Content[len(sequence.Content)-1],
	)
	if err != nil {
		return 0, err
	}
	return span.end, nil
}

func managedYAMLSequenceIndent(
	sequence *yaml.Node,
) (int, error) {
	if len(sequence.Content) == 0 {
		if sequence.Column < 1 {
			return 0, fmt.Errorf(
				"managed YAML sequence has no source indentation",
			)
		}
		return sequence.Column - 1, nil
	}
	member := sequence.Content[0]
	indent := member.Column - 3
	if indent < 0 {
		return 0, fmt.Errorf(
			"managed YAML sequence member indentation is invalid",
		)
	}
	return indent, nil
}

func managedYAMLSequenceMemberIndent(
	document managedYAMLDocument,
	member *yaml.Node,
) (int, error) {
	line := managedYAMLLine(document, member.Line-1)
	indent, _, err := managedYAMLLineIndent(line)
	if err != nil {
		return 0, err
	}
	return indent, nil
}

func insertManagedYAMLBlock(
	raw []byte,
	offset int,
	block []byte,
	newline string,
) []byte {
	prefix := slices.Clone(raw[:offset])
	suffix := slices.Clone(raw[offset:])
	var output bytes.Buffer
	output.Write(prefix)
	if len(prefix) > 0 && !bytes.HasSuffix(prefix, []byte{'\n'}) {
		output.WriteString(newline)
	}
	output.Write(block)
	if len(block) > 0 && !bytes.HasSuffix(block, []byte(newline)) {
		output.WriteString(newline)
	}
	output.Write(suffix)
	return output.Bytes()
}

func spliceManagedYAML(
	raw []byte,
	span managedYAMLByteSpan,
	replacement []byte,
) []byte {
	result := make([]byte, 0, len(raw)-(span.end-span.start)+len(replacement))
	result = append(result, raw[:span.start]...)
	result = append(result, replacement...)
	result = append(result, raw[span.end:]...)
	return result
}
