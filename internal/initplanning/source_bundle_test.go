package initplanning

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestSkillSourceBundleIsCanonicalContentAddressedAndStrict(t *testing.T) {
	kernelDigest := digestBytes([]byte("kernel catalog"))
	left, err := BuildSkillSourceBundle(
		"skills.v1",
		kernelDigest,
		[]SkillSourceInput{
			implicitSkillInput("h-reason", "Reason from exact source"),
			manualSkillInput("h-decide", "Bind an operator choice"),
		},
	)
	if err != nil {
		t.Fatalf("BuildSkillSourceBundle: %v", err)
	}
	right, err := BuildSkillSourceBundle(
		"skills.v1",
		kernelDigest,
		[]SkillSourceInput{
			manualSkillInput("h-decide", "Bind an operator choice"),
			implicitSkillInput("h-reason", "Reason from exact source"),
		},
	)
	if err != nil {
		t.Fatalf("BuildSkillSourceBundle reversed: %v", err)
	}
	if left.Digest() != right.Digest() ||
		!bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("skill input order changed bundle identity")
	}
	parsed, err := ParseSkillSourceBundle(left.CanonicalBytes())
	if err != nil {
		t.Fatalf("ParseSkillSourceBundle: %v", err)
	}
	if parsed.Ref() != left.Ref() || parsed.Digest() != left.Digest() {
		t.Fatal("parsed skill bundle changed identity")
	}
	if len(parsed.Skills()) != 2 || parsed.Skills()[0].Name() != "h-decide" {
		t.Fatalf("canonical skill set = %+v", parsed.Skills())
	}
	var expanded map[string]any
	if err := json.Unmarshal(left.CanonicalBytes(), &expanded); err != nil {
		t.Fatalf("decode bundle fixture: %v", err)
	}
	expanded["unknown"] = "field"
	unknown, err := json.Marshal(expanded)
	if err != nil {
		t.Fatalf("encode bundle fixture: %v", err)
	}
	if _, err := ParseSkillSourceBundle(unknown); err == nil {
		t.Fatal("skill bundle accepted an unknown field")
	}
	pretty, err := json.MarshalIndent(expandedWithoutUnknown(expanded), "", "  ")
	if err != nil {
		t.Fatalf("encode pretty bundle fixture: %v", err)
	}
	if _, err := ParseSkillSourceBundle(pretty); err == nil {
		t.Fatal("skill bundle accepted non-canonical JSON")
	}
}

func TestSkillSourceBundleRejectsInvocationAndContentIdentityDrift(t *testing.T) {
	kernelDigest := digestBytes([]byte("kernel catalog"))
	manualWithoutGate := implicitSkillInput("h-decide", "Bind a choice")
	manualWithoutGate.InvocationPolicy = SkillInvocationManualOnly
	implicitWithGate := manualSkillInput("h-reason", "Reason")
	implicitWithGate.InvocationPolicy = SkillInvocationImplicitAllowed
	wrongName := implicitSkillInput("h-reason", "Reason")
	wrongName.Content = []byte("---\nname: h-other\n---\nbody\n")
	for name, input := range map[string]SkillSourceInput{
		"manual without gate": manualWithoutGate,
		"implicit with gate":  implicitWithGate,
		"wrong content name":  wrongName,
		"empty description": {
			Name:             "h-reason",
			Description:      "",
			InvocationPolicy: SkillInvocationImplicitAllowed,
			Content:          []byte("---\nname: h-reason\n---\nbody\n"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildSkillSourceBundle(
				"skills.v1",
				kernelDigest,
				[]SkillSourceInput{input},
			); err == nil {
				t.Fatal("skill bundle accepted semantic identity drift")
			}
		})
	}
}

func TestSkillSourceBundleGettersCopyContent(t *testing.T) {
	bundle, err := BuildSkillSourceBundle(
		"skills.v1",
		digestBytes([]byte("kernel catalog")),
		[]SkillSourceInput{implicitSkillInput("h-reason", "Reason")},
	)
	if err != nil {
		t.Fatalf("BuildSkillSourceBundle: %v", err)
	}
	before := bundle.Skills()[0].Content()
	changed := bundle.Skills()[0].Content()
	changed[0] = 'x'
	if !reflect.DeepEqual(bundle.Skills()[0].Content(), before) {
		t.Fatal("skill bundle exposed mutable source content")
	}
}

func implicitSkillInput(name string, description string) SkillSourceInput {
	return SkillSourceInput{
		Name:             name,
		Description:      description,
		InvocationPolicy: SkillInvocationImplicitAllowed,
		Content: []byte(
			"---\nname: " + name + "\ndescription: source\n---\nbody\n",
		),
	}
}

func manualSkillInput(name string, description string) SkillSourceInput {
	return SkillSourceInput{
		Name:             name,
		Description:      description,
		InvocationPolicy: SkillInvocationManualOnly,
		Content: []byte(
			"---\nname: " + name + "\ndescription: source\ndisable-model-invocation: true\n---\nbody\n",
		),
	}
}
