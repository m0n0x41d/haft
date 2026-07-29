package artifact

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDecideInputCanonicalJSONUsesArtifactNormalization(t *testing.T) {
	input := DecideInput{
		ProblemStatement: "  choose a durable boundary  ",
		SelectedTitle:    "  bind the typed boundary  ",
		WhySelected:      "  it preserves the authority split  ",
		AffectedFiles:    []string{" internal/a.go ", "", "internal/b.go"},
		ChoiceResult: &ChoiceResult{
			SubjectRef:    " operator ",
			OptionSet:     []string{" bind the typed boundary ", "", "defer"},
			NextMove:      ChoiceNextMove(" choose_now "),
			VariantRef:    " bind the typed boundary ",
			Reversibility: " supersede with a later decision ",
		},
		SpecBindingPreflight: &SpecBindingPreflight{
			State:               SpecBindingStateProvidedRefsValid,
			SelectedSectionRefs: []string{" SS.contract ", ""},
		},
	}

	canonical, err := EncodeDecideInputCanonicalJSON(input)
	if err != nil {
		t.Fatalf("EncodeDecideInputCanonicalJSON: %v", err)
	}
	decoded, err := DecodeDecideInputCanonicalJSON(canonical)
	if err != nil {
		t.Fatalf("DecodeDecideInputCanonicalJSON: %v", err)
	}
	want := normalizeDecisionInput(input)
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded input differs from artifact normalization\n got: %#v\nwant: %#v", decoded, want)
	}
	if decoded.SelectedTitle != "bind the typed boundary" {
		t.Fatalf("selected title = %q, want normalized value", decoded.SelectedTitle)
	}
	if !reflect.DeepEqual(decoded.SectionRefs, []string{"SS.contract"}) {
		t.Fatalf("section refs = %#v, want preflight-derived canonical refs", decoded.SectionRefs)
	}

	unprepared, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if bytes.Equal(unprepared, canonical) {
		t.Fatal("canonical encoder preserved non-normalized input bytes")
	}
	if _, err := DecodeDecideInputCanonicalJSON(unprepared); err == nil {
		t.Fatal("decoder accepted JSON whose decision semantics still require normalization")
	}
}

func TestDecodeDecideInputCanonicalJSONRejectsAmbiguousOrAlteredBytes(t *testing.T) {
	input := DecideInput{
		ProblemStatement: "choose a boundary",
		SelectedTitle:    "bind it",
	}
	canonical, err := EncodeDecideInputCanonicalJSON(input)
	if err != nil {
		t.Fatalf("EncodeDecideInputCanonicalJSON: %v", err)
	}
	pretty := &bytes.Buffer{}
	if err := json.Indent(pretty, canonical, "", "  "); err != nil {
		t.Fatalf("json.Indent: %v", err)
	}

	cases := map[string][]byte{
		"pretty JSON":        pretty.Bytes(),
		"trailing newline":   append(bytes.Clone(canonical), '\n'),
		"trailing value":     append(bytes.Clone(canonical), []byte(` {}`)...),
		"unknown field":      []byte(`{"problem_statement":"choose a boundary","selected_title":"bind it","unknown":true}`),
		"duplicate field":    []byte(`{"problem_statement":"choose a boundary","selected_title":"bind it","selected_title":"bind it"}`),
		"explicit empty":     []byte(`{"problem_statement":"choose a boundary","selected_title":"bind it","affected_files":[]}`),
		"reordered fields":   []byte(`{"selected_title":"bind it","problem_statement":"choose a boundary"}`),
		"invalid UTF-8":      {'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'},
		"multiple documents": []byte(`{} {}`),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeDecideInputCanonicalJSON(candidate)
			if err == nil {
				t.Fatalf("accepted altered bytes: %q", candidate)
			}
		})
	}
}

func TestDecideInputCanonicalJSONIsStable(t *testing.T) {
	input := DecideInput{
		ProblemStatement: "choose a boundary",
		SelectedTitle:    "bind it",
		PostConditions:   []string{"future effects consume the exact prepared snapshot"},
	}
	first, err := EncodeDecideInputCanonicalJSON(input)
	if err != nil {
		t.Fatalf("first encode: %v", err)
	}
	decoded, err := DecodeDecideInputCanonicalJSON(first)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	second, err := EncodeDecideInputCanonicalJSON(decoded)
	if err != nil {
		t.Fatalf("second encode: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("canonical bytes changed\nfirst:  %s\nsecond: %s", first, second)
	}
	if strings.Contains(string(first), "  ") {
		t.Fatalf("canonical JSON unexpectedly contains formatting whitespace: %s", first)
	}
}
