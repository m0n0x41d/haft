package artifact

import "strings"

// PortfolioVariantIdentity exposes a recoverable variant identity for
// comparison, selection capture, and decision validation.
type PortfolioVariantIdentity struct {
	Key     string
	Label   string
	Aliases []string
}

// PreviewPortfolioVariantIdentities returns the recoverable variant identities
// without validating ambiguity. Use this only for lightweight intent detection
// ahead of stricter validation.
func PreviewPortfolioVariantIdentities(portfolio *Artifact) []PortfolioVariantIdentity {
	identities := portfolioVariantIdentities(portfolio)
	return materializePortfolioVariantIdentities(identities)
}

// RecoverPortfolioVariantIdentities returns the canonical variant identities
// that can be recovered from a portfolio's structured data and legacy body.
func RecoverPortfolioVariantIdentities(portfolio *Artifact) ([]PortfolioVariantIdentity, error) {
	identities := portfolioVariantIdentities(portfolio)
	if len(identities) == 0 {
		return nil, nil
	}
	if err := validatePortfolioVariantIdentities(identities); err != nil {
		return nil, err
	}

	return materializePortfolioVariantIdentities(identities), nil
}

// ResolvePortfolioVariantIdentity maps a stored or user-provided variant
// reference back to the portfolio's canonical variant identity.
func ResolvePortfolioVariantIdentity(portfolio *Artifact, ref string) (PortfolioVariantIdentity, bool, error) {
	normalizedRef := strings.TrimSpace(ref)
	if normalizedRef == "" {
		return PortfolioVariantIdentity{}, false, nil
	}

	identities, err := RecoverPortfolioVariantIdentities(portfolio)
	if err != nil {
		return PortfolioVariantIdentity{}, false, err
	}

	for _, identity := range identities {
		if strings.TrimSpace(identity.Key) == normalizedRef {
			return identity, true, nil
		}
		if strings.TrimSpace(identity.Label) == normalizedRef {
			return identity, true, nil
		}
		for _, alias := range identity.Aliases {
			if strings.TrimSpace(alias) == normalizedRef {
				return identity, true, nil
			}
		}
	}

	return PortfolioVariantIdentity{}, false, nil
}

func materializePortfolioVariantIdentities(identities []portfolioVariantIdentity) []PortfolioVariantIdentity {
	recovered := make([]PortfolioVariantIdentity, 0, len(identities))
	for _, identity := range identities {
		recovered = append(recovered, PortfolioVariantIdentity{
			Key:     strings.TrimSpace(identity.Key),
			Label:   strings.TrimSpace(identity.Label),
			Aliases: append([]string(nil), identity.Aliases...),
		})
	}

	return recovered
}
