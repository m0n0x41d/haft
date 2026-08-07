package projectidentity

import "testing"

func TestProjectIDUsesCanonicalSyntax(t *testing.T) {
	valid, err := ParseProjectID("qnt_a7f3b2c1")
	if err != nil {
		t.Fatalf("ParseProjectID(valid) error = %v", err)
	}
	if valid.String() != "qnt_a7f3b2c1" {
		t.Fatalf("ProjectID.String() = %q", valid.String())
	}
	for _, invalid := range []string{
		"qnt_A7F3B2C1",
		" qnt_a7f3b2c1",
		"qnt_a7f3b2c1/other",
		"qnt_a7f3\x00b2c1",
	} {
		if _, err := ParseProjectID(invalid); err == nil {
			t.Fatalf("ParseProjectID(%q) accepted a non-canonical ID", invalid)
		}
	}
}
