package artifact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRecoverPortfolioVariantIdentities_UsesStructuredAndBodyAliases(t *testing.T) {
	fields, err := json.Marshal(PortfolioFields{
		Variants: []Variant{
			{ID: "V1", Title: "REST", WeakestLink: "chatty payloads"},
			{Title: "gRPC", WeakestLink: "tooling overhead"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	portfolio := &Artifact{
		Meta: Meta{
			ID:    "sol-transport",
			Kind:  KindSolutionPortfolio,
			Title: "Transport options",
		},
		Body: `# Transport options

## Variants (2)

### V1. REST

**Weakest link:** chatty payloads

### V2. gRPC

**Weakest link:** tooling overhead
`,
		StructuredData: string(fields),
	}

	identities, err := RecoverPortfolioVariantIdentities(portfolio)
	if err != nil {
		t.Fatal(err)
	}
	if len(identities) != 2 {
		t.Fatalf("expected 2 identities, got %+v", identities)
	}
	if identities[1].Key != "V2" {
		t.Fatalf("expected fallback body ID V2, got %+v", identities[1])
	}
	if !containsString(identities[1].Aliases, "gRPC") {
		t.Fatalf("expected gRPC title alias, got %+v", identities[1])
	}

	byID, ok, err := ResolvePortfolioVariantIdentity(portfolio, "V2")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected V2 lookup to resolve")
	}
	if byID.Label != "gRPC" {
		t.Fatalf("expected V2 label gRPC, got %+v", byID)
	}

	byTitle, ok, err := ResolvePortfolioVariantIdentity(portfolio, "gRPC")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected title lookup to resolve")
	}
	if byTitle.Key != "V2" {
		t.Fatalf("expected title lookup to resolve canonical key V2, got %+v", byTitle)
	}
}

func TestResolvePortfolioVariantIdentity_LegacyBodyOnly(t *testing.T) {
	portfolio := &Artifact{
		Meta: Meta{
			ID:    "sol-legacy-transport",
			Kind:  KindSolutionPortfolio,
			Title: "Legacy transport options",
		},
		Body: `# Legacy transport options

## Variants (2)

### V1. REST

**Weakest link:** chatty payloads

### V2. gRPC

**Weakest link:** tooling overhead
`,
	}

	identity, ok, err := ResolvePortfolioVariantIdentity(portfolio, "gRPC")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected legacy title lookup to resolve")
	}
	if identity.Key != "V2" || identity.Label != "gRPC" {
		t.Fatalf("unexpected legacy identity: %+v", identity)
	}
}

func TestRecoverPortfolioVariantIdentities_RejectsAmbiguousAliases(t *testing.T) {
	portfolio := &Artifact{
		Meta: Meta{
			ID:    "sol-ambiguous",
			Kind:  KindSolutionPortfolio,
			Title: "Ambiguous variants",
		},
		Body: `# Ambiguous variants

## Variants (2)

### V1. REST

**Weakest link:** chatty payloads

### V1. gRPC

**Weakest link:** tooling overhead
`,
	}

	_, err := RecoverPortfolioVariantIdentities(portfolio)
	if err == nil {
		t.Fatal("expected ambiguous identity error")
	}
	if !strings.Contains(err.Error(), `variant identity "V1" is duplicated`) {
		t.Fatalf("unexpected ambiguity error: %v", err)
	}
}

func TestPreviewPortfolioVariantIdentities_AllowsAmbiguousLegacyPortfolio(t *testing.T) {
	portfolio := &Artifact{
		Meta: Meta{
			ID:    "sol-ambiguous",
			Kind:  KindSolutionPortfolio,
			Title: "Ambiguous variants",
		},
		Body: `# Ambiguous variants

## Variants (2)

### V1. REST

**Weakest link:** chatty payloads

### V1. gRPC

**Weakest link:** tooling overhead
`,
	}

	identities := PreviewPortfolioVariantIdentities(portfolio)
	if len(identities) != 2 {
		t.Fatalf("expected preview identities without validation failure, got %+v", identities)
	}
}
