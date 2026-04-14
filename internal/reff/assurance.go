package reff

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInadmissibleEvidence = errors.New("CL0 evidence cannot support")

// Evidence is the minimal evidence shape needed for claim-scoped assurance
// decomposition in the evidence engine.
type Evidence struct {
	ClaimRefs       []string
	Type            string
	Verdict         string
	CongruenceLevel int
	FormalityLevel  int
	HasFormality    bool
	ValidUntil      string
}

// ClaimAssurance is the per-claim F/G/R decomposition defined in the evidence
// ontology.
type ClaimAssurance struct {
	ClaimRef      string
	EvidenceCount int
	HasEvidence   bool
	FEff          int
	GEff          int
	REff          float64
}

// DecisionAssurance is the decision-level weakest-link aggregation over claims.
type DecisionAssurance struct {
	Claims      []ClaimAssurance
	HasEvidence bool
	FEff        int
	GEff        int
	REff        float64
}

// ComputeDecisionAssurance computes claim-scoped assurance and then applies the
// weakest-link rule across claims. When claimRefs is empty, evidence is scored
// directly as a backward-compatible pre-claim decision.
func ComputeDecisionAssurance(claimRefs []string, items []Evidence, now time.Time) (DecisionAssurance, error) {
	activeItems := filterActiveEvidence(items)
	normalizedClaims := normalizeClaimRefs(claimRefs)

	if len(normalizedClaims) == 0 {
		claim, err := ComputeClaimAssurance("", activeItems, now)
		if err != nil {
			return DecisionAssurance{}, err
		}

		return DecisionAssurance{
			HasEvidence: claim.HasEvidence,
			FEff:        claim.FEff,
			GEff:        claim.GEff,
			REff:        claim.REff,
		}, nil
	}

	result := DecisionAssurance{
		Claims: make([]ClaimAssurance, 0, len(normalizedClaims)),
		FEff:   3,
		GEff:   3,
		REff:   1.0,
	}

	for _, claimRef := range normalizedClaims {
		claimItems := filterClaimEvidence(activeItems, claimRef)
		claim, err := ComputeClaimAssurance(claimRef, claimItems, now)
		if err != nil {
			return DecisionAssurance{}, err
		}

		result.Claims = append(result.Claims, claim)
		result.HasEvidence = result.HasEvidence || claim.HasEvidence
		result.FEff = minInt(result.FEff, claim.FEff)
		result.GEff = minInt(result.GEff, claim.GEff)
		result.REff = minFloat(result.REff, claim.REff)
	}

	return result, nil
}

// ComputeClaimAssurance computes F/G/R for one claim by applying the
// weakest-link rule to all active evidence that references it.
func ComputeClaimAssurance(claimRef string, items []Evidence, now time.Time) (ClaimAssurance, error) {
	result := ClaimAssurance{
		ClaimRef: claimRef,
	}

	if len(items) == 0 {
		return result, nil
	}

	result.EvidenceCount = len(items)
	result.HasEvidence = true
	result.FEff = 3
	result.GEff = 3
	result.REff = 1.0

	for _, item := range items {
		err := ValidateEvidence(item)
		if err != nil {
			return ClaimAssurance{}, err
		}

		formality := DeriveFormality(item)
		groundedness := GroundednessFromCL(item.CongruenceLevel)
		reliability := ScoreTypedEvidence(item.Type, item.Verdict, item.CongruenceLevel, item.ValidUntil, now)

		result.FEff = minInt(result.FEff, formality)
		result.GEff = minInt(result.GEff, groundedness)
		result.REff = minFloat(result.REff, reliability)
	}

	return result, nil
}

// ValidateEvidence rejects evidence that the target-system contract marks as
// inadmissible before it enters R_eff computation.
func ValidateEvidence(item Evidence) error {
	verdict := CanonicalVerdict(item.Verdict)
	groundedness := GroundednessFromCL(item.CongruenceLevel)

	if verdict != "supports" || groundedness != 0 {
		return nil
	}

	return fmt.Errorf("%w — re-evaluate or change verdict", ErrInadmissibleEvidence)
}

// CanonicalVerdict maps measurement aliases onto the canonical evidence
// vocabulary used by the evidence engine.
func CanonicalVerdict(verdict string) string {
	normalized := strings.ToLower(strings.TrimSpace(verdict))

	switch normalized {
	case "supports", "accepted", "pass":
		return "supports"
	case "weakens", "partial", "degrade":
		return "weakens"
	case "refutes", "failed", "fail":
		return "refutes"
	case "superseded":
		return "superseded"
	default:
		return "weakens"
	}
}

// DeriveFormality returns the normalized explicit formality when present.
// Otherwise it derives formality from evidence type.
func DeriveFormality(item Evidence) int {
	if item.HasFormality {
		return NormalizeFormalityLevel(item.FormalityLevel)
	}

	return inferFormalityFromType(item.Type)
}

// NormalizeFormalityLevel preserves F0-F3 and folds legacy 0-9 values into the
// same normalized formality scale used by the artifact layer.
func NormalizeFormalityLevel(level int) int {
	switch {
	case level < 0:
		return 0
	case level <= 3:
		return level
	case level <= 5:
		return 1
	case level <= 8:
		return 2
	default:
		return 3
	}
}

// GroundednessFromCL preserves CL semantics for view decomposition:
// CL3=direct, CL2=similar, CL1=indirect, CL0=inadmissible.
func GroundednessFromCL(cl int) int {
	switch cl {
	case 3, 2, 1, 0:
		return cl
	default:
		return 0
	}
}

func inferFormalityFromType(evidenceType string) int {
	normalized := strings.ToLower(strings.TrimSpace(evidenceType))

	switch normalized {
	case "formal_proof", "proof", "proof_grade":
		return 3
	case "test_result", "test", "measurement", "explicit_measure", "partial_measure", "benchmark", "audit":
		return 2
	case "anecdote", "obs_no_verify":
		return 0
	case "documentation", "expert_opinion", "cross_project", "research", "code_review", "user_feedback", "attached", "observation", "obs_file_review", "obs_external":
		return 1
	case "obs_test_pass", "obs_lint_pass":
		return 2
	default:
		return 1
	}
}

func filterActiveEvidence(items []Evidence) []Evidence {
	active := make([]Evidence, 0, len(items))

	for _, item := range items {
		if CanonicalVerdict(item.Verdict) == "superseded" {
			continue
		}

		active = append(active, item)
	}

	return active
}

func filterClaimEvidence(items []Evidence, claimRef string) []Evidence {
	filtered := make([]Evidence, 0, len(items))

	for _, item := range items {
		if !containsClaimRef(item.ClaimRefs, claimRef) {
			continue
		}

		filtered = append(filtered, item)
	}

	return filtered
}

func normalizeClaimRefs(values []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}

		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func containsClaimRef(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}

	return false
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}

	return right
}

func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}

	return right
}
