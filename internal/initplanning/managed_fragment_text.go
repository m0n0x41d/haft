package initplanning

import (
	"bytes"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"unicode/utf8"
)

var htmlCommentSectionNamespacePattern = regexp.MustCompile(
	`^[a-z][a-z0-9_-]*$`,
)

type managedTextSectionSpan struct {
	start  int
	end    int
	digest string
	found  bool
}

// NewHTMLCommentSectionFragment owns one exact section delimited by
// "<!-- namespace:start -->" and "<!-- namespace:end -->". Bytes outside the
// markers remain carrier-owned and are never included in the fragment digest.
func NewHTMLCommentSectionFragment(
	carrierPath string,
	component Component,
	namespace string,
	body []byte,
	createMode fs.FileMode,
	mergeEdition string,
) (ManagedFragment, error) {
	coordinate, err := newHTMLCommentSectionCoordinate(
		carrierPath,
		namespace,
		mergeEdition,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	content, err := canonicalHTMLCommentSection(
		namespace,
		body,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	return newManagedFragment(
		coordinate,
		component,
		content,
		createMode,
	)
}

func newHTMLCommentSectionCoordinate(
	carrierPath string,
	namespace string,
	mergeEdition string,
) (ManagedFragmentCoordinate, error) {
	canonicalPath, err := parseCanonicalAbsolutePath(carrierPath)
	if err != nil {
		return ManagedFragmentCoordinate{}, fmt.Errorf(
			"managed HTML-comment section carrier path: %w",
			err,
		)
	}
	validatedNamespace, err := validateHTMLCommentSectionNamespace(
		namespace,
	)
	if err != nil {
		return ManagedFragmentCoordinate{}, err
	}
	validatedEdition, err := validateManagedMergeEdition(mergeEdition)
	if err != nil {
		return ManagedFragmentCoordinate{}, err
	}
	return ManagedFragmentCoordinate{
		carrierPath:  canonicalPath,
		kind:         ManagedHTMLCommentSection,
		selector:     validatedNamespace,
		mergeEdition: validatedEdition,
	}, nil
}

func validateHTMLCommentSectionNamespace(
	namespace string,
) (string, error) {
	if !htmlCommentSectionNamespacePattern.MatchString(namespace) {
		return "", fmt.Errorf(
			"managed HTML-comment section namespace is invalid",
		)
	}
	return namespace, nil
}

func canonicalHTMLCommentSection(
	namespace string,
	body []byte,
) ([]byte, error) {
	validUTF8 := utf8.Valid(body)
	nulIndex := bytes.IndexByte(body, 0)
	if !validUTF8 || nulIndex >= 0 {
		return nil, fmt.Errorf(
			"managed HTML-comment section body must be UTF-8 text",
		)
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf(
			"managed HTML-comment section body is empty",
		)
	}
	startMarker, endMarker := htmlCommentSectionMarkers(namespace)
	startBytes := []byte(startMarker)
	endBytes := []byte(endMarker)
	hasStartMarker := bytes.Contains(trimmed, startBytes)
	hasEndMarker := bytes.Contains(trimmed, endBytes)
	if hasStartMarker || hasEndMarker {
		return nil, fmt.Errorf(
			"managed HTML-comment section body repeats its boundary marker",
		)
	}
	content := make(
		[]byte,
		0,
		len(startMarker)+len(trimmed)+len(endMarker)+2,
	)
	content = append(content, startMarker...)
	content = append(content, '\n')
	content = append(content, trimmed...)
	content = append(content, '\n')
	content = append(content, endMarker...)
	return content, nil
}

func htmlCommentSectionMarkers(
	namespace string,
) (string, string) {
	start := "<!-- " + namespace + ":start -->"
	end := "<!-- " + namespace + ":end -->"
	return start, end
}

func observeManagedText(
	probes []managedFragmentProbe,
	raw []byte,
) ([]ManagedFragmentObservation, error) {
	observations := make([]ManagedFragmentObservation, len(probes))
	for index, probe := range probes {
		if probe.coordinate.kind != ManagedHTMLCommentSection {
			return nil, fmt.Errorf("managed text probe kind is invalid")
		}
		span, err := locateManagedHTMLCommentSection(
			raw,
			probe.coordinate,
		)
		if err != nil {
			return nil, err
		}
		if !span.found {
			observations[index] = missingManagedFragmentObservation(
				probe.coordinate,
			)
			continue
		}
		observations[index] = presentManagedFragmentObservation(
			probe.coordinate,
			span.digest,
		)
	}
	return observations, nil
}

func locateManagedHTMLCommentSection(
	raw []byte,
	coordinate ManagedFragmentCoordinate,
) (managedTextSectionSpan, error) {
	startMarker, endMarker := htmlCommentSectionMarkers(
		coordinate.selector,
	)
	startBytes := []byte(startMarker)
	endBytes := []byte(endMarker)
	startCount := bytes.Count(raw, startBytes)
	endCount := bytes.Count(raw, endBytes)
	if startCount == 0 && endCount == 0 {
		return managedTextSectionSpan{}, nil
	}
	if startCount != 1 || endCount != 1 {
		return managedTextSectionSpan{}, fmt.Errorf(
			"managed HTML-comment section %s has ambiguous or incomplete markers",
			coordinate.selector,
		)
	}
	start := bytes.Index(raw, startBytes)
	endStart := bytes.Index(raw, endBytes)
	if start < 0 || endStart <= start {
		return managedTextSectionSpan{}, fmt.Errorf(
			"managed HTML-comment section %s has reversed markers",
			coordinate.selector,
		)
	}
	end := endStart + len(endBytes)
	content := raw[start:end]
	digest := managedFragmentDigest(content)
	return managedTextSectionSpan{
		start:  start,
		end:    end,
		digest: digest,
		found:  true,
	}, nil
}

func applyManagedTextEffects(
	effects []ManagedFragmentEffect,
	input ManagedCarrierInput,
) ([]byte, error) {
	content := slices.Clone(input.content)
	for _, effect := range effects {
		if !managedFragmentEffectMutates(effect.kind) {
			continue
		}
		if effect.coordinate.kind != ManagedHTMLCommentSection {
			return nil, fmt.Errorf("managed text effect kind is invalid")
		}
		span, err := locateManagedHTMLCommentSection(
			content,
			effect.coordinate,
		)
		if err != nil {
			return nil, err
		}
		content, err = applyManagedTextEffect(
			content,
			span,
			effect,
		)
		if err != nil {
			return nil, err
		}
	}
	return content, nil
}

func applyManagedTextEffect(
	content []byte,
	span managedTextSectionSpan,
	effect ManagedFragmentEffect,
) ([]byte, error) {
	if effect.kind == ManagedFragmentCreate {
		if span.found {
			return nil, fmt.Errorf(
				"managed HTML-comment section create target is no longer vacant",
			)
		}
		if !effect.hasDesired {
			return nil, fmt.Errorf(
				"managed HTML-comment section create lacks desired fragment",
			)
		}
		return appendManagedTextSection(
			content,
			effect.desired.content,
		), nil
	}
	if !span.found {
		return nil, fmt.Errorf(
			"managed HTML-comment section target is missing",
		)
	}
	if span.digest != effect.expectedDigest {
		return nil, fmt.Errorf(
			"managed HTML-comment section precondition changed",
		)
	}
	if effect.kind == ManagedFragmentRemove {
		return replaceManagedTextSpan(
			content,
			span,
			nil,
		), nil
	}
	if effect.kind != ManagedFragmentReplace || !effect.hasDesired {
		return nil, fmt.Errorf(
			"managed HTML-comment section effect is invalid",
		)
	}
	return replaceManagedTextSpan(
		content,
		span,
		effect.desired.content,
	), nil
}

func appendManagedTextSection(
	content []byte,
	section []byte,
) []byte {
	newline := []byte("\n")
	doubleNewline := []byte("\n\n")
	hasDoubleNewline := bytes.HasSuffix(content, doubleNewline)
	separator := "\n\n"
	if len(content) == 0 || hasDoubleNewline {
		separator = ""
	}
	hasNewline := bytes.HasSuffix(content, newline)
	if len(content) != 0 &&
		separator != "" &&
		hasNewline {
		separator = "\n"
	}
	result := make(
		[]byte,
		0,
		len(content)+len(separator)+len(section)+1,
	)
	result = append(result, content...)
	result = append(result, separator...)
	result = append(result, section...)
	result = append(result, '\n')
	return result
}

func replaceManagedTextSpan(
	content []byte,
	span managedTextSectionSpan,
	replacement []byte,
) []byte {
	result := make(
		[]byte,
		0,
		len(content)-span.end+span.start+len(replacement),
	)
	result = append(result, content[:span.start]...)
	result = append(result, replacement...)
	result = append(result, content[span.end:]...)
	return result
}
