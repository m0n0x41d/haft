package profiledeclarationpreparation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const (
	ModeExplicitHOnboard = "explicit_h_onboard"
	ModeStrictSpeechAct  = "strict_cli_speech_act"
)

// Policy is the exact project-local declaration-authority policy. It carries
// semantic configuration provenance, never an authority receipt.
type Policy struct {
	mode                string
	configCarrierRef    string
	configCarrierDigest projectprofile.ContentDigest
}

func NewPolicy(mode string, configCarrierRef string, configCarrier []byte) (Policy, error) {
	switch mode {
	case ModeExplicitHOnboard:
		if strings.TrimSpace(configCarrierRef) == "" || len(configCarrier) == 0 {
			return Policy{}, fmt.Errorf(
				"explicit profile declaration requires its effective config carrier",
			)
		}
		digest, err := digestBytes(
			"haft.project-config.profile-declaration-policy/v1",
			configCarrier,
		)
		if err != nil {
			return Policy{}, err
		}
		return Policy{
			mode:                mode,
			configCarrierRef:    configCarrierRef,
			configCarrierDigest: digest,
		}, nil
	case ModeStrictSpeechAct:
		return Policy{mode: mode}, nil
	default:
		return Policy{}, fmt.Errorf(
			"profile declaration authority mode %q is unsupported",
			mode,
		)
	}
}

func (policy Policy) Mode() string { return policy.mode }

func (policy Policy) ConfigCarrier() (string, projectprofile.ContentDigest, bool) {
	if policy.mode != ModeExplicitHOnboard || policy.configCarrierRef == "" {
		return "", projectprofile.ContentDigest{}, false
	}
	return policy.configCarrierRef, policy.configCarrierDigest, true
}

func digestBytes(domain string, value []byte) (projectprofile.ContentDigest, error) {
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(value)
	raw := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	return projectprofile.NewContentDigest(raw)
}
