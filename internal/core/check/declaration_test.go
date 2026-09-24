package check

import "testing"

func TestDeclaredIdentityRejectsContradictionsButNotRawHistory(t *testing.T) {
	declared := loadProbe(t, "property_pass").Expected
	cases := []struct {
		name string
		edit func(*Contract)
	}{
		{"package", func(c *Contract) { c.Selector.Package += "/other" }},
		{"test", func(c *Contract) { c.Selector.Test = "TestOther" }},
		{"oracle", func(c *Contract) { c.Ref = "pbt:other_test.go::TestCancelPreservesTotal" }},
		{"scope", func(c *Contract) { c.Scope = "broader scope" }},
		{"failure-contract", func(c *Contract) { c.FailureContract = "other meaning" }},
		{"failure-pattern", func(c *Contract) { c.FailurePattern = "different failure" }},
		{"seed", func(c *Contract) { v := int64(123); c.Basis.Seed = &v }},
		{"claim", func(c *Contract) { c.Basis.Claim += "-other" }},
		{"binding-condition", func(c *Contract) { c.Basis.Conditions = []string{"broader condition"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expected := loadProbe(t, "property_pass").Expected
			tc.edit(&expected)
			if DeclarationMismatch(expected, declared) == "" {
				t.Fatal("contradiction matched declared identity")
			}
		})
	}
	historical := loadProbe(t, "property_pass").Expected
	historical.Basis.Code = "old raw implementation"
	historical.Basis.Check = "old raw oracle"
	historical.Basis.Dependencies = "old raw dependencies"
	historical.Basis.Environment["toolchain"] = "old toolchain"
	if got := DeclarationMismatch(historical, declared); got != "" {
		t.Fatal("historical raw basis was treated as an identity forgery", got)
	}
}
