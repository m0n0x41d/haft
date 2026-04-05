package artifact

import (
	"fmt"
	"strings"
)

var validClaimStatuses = map[ClaimStatus]struct{}{
	ClaimStatusUnverified:   {},
	ClaimStatusSupported:    {},
	ClaimStatusWeakened:     {},
	ClaimStatusRefuted:      {},
	ClaimStatusInconclusive: {},
}

// PredictionMeasureMatch captures how one measurement run touches one prediction.
type PredictionMeasureMatch struct {
	MeasurementRecorded bool
	CriteriaMet         bool
	CriteriaNotMet      bool
}

func normalizeClaimStatus(value ClaimStatus) ClaimStatus {
	normalized := ClaimStatus(strings.TrimSpace(string(value)))

	if _, ok := validClaimStatuses[normalized]; ok {
		return normalized
	}

	return ClaimStatusUnverified
}

func newDecisionClaims(inputs []PredictionInput) []DecisionClaim {
	claims := make([]DecisionClaim, 0, len(inputs))

	for _, input := range inputs {
		claim := DecisionClaim{
			Claim:      strings.TrimSpace(input.Claim),
			Observable: strings.TrimSpace(input.Observable),
			Threshold:  strings.TrimSpace(input.Threshold),
			Status:     ClaimStatusUnverified,
		}
		if claim.Claim == "" && claim.Observable == "" && claim.Threshold == "" {
			continue
		}

		claims = append(claims, claim)
	}

	return normalizeDecisionClaims(claims)
}

func decisionClaimsFromPredictions(values []DecisionPrediction) []DecisionClaim {
	claims := make([]DecisionClaim, 0, len(values))

	for _, value := range values {
		claim := DecisionClaim{
			Claim:      strings.TrimSpace(value.Claim),
			Observable: strings.TrimSpace(value.Observable),
			Threshold:  strings.TrimSpace(value.Threshold),
			Status:     normalizeClaimStatus(value.Status),
		}
		if claim.Claim == "" && claim.Observable == "" && claim.Threshold == "" {
			continue
		}

		claims = append(claims, claim)
	}

	return normalizeDecisionClaims(claims)
}

func normalizeDecisionClaims(values []DecisionClaim) []DecisionClaim {
	claims := make([]DecisionClaim, 0, len(values))
	seenIDs := make(map[string]struct{}, len(values))

	for _, value := range values {
		evidenceRefs := compactStrings(value.EvidenceRefs)
		if len(evidenceRefs) == 0 {
			evidenceRefs = nil
		}

		claim := DecisionClaim{
			ID:           strings.TrimSpace(value.ID),
			Claim:        strings.TrimSpace(value.Claim),
			Observable:   strings.TrimSpace(value.Observable),
			Threshold:    strings.TrimSpace(value.Threshold),
			Status:       normalizeClaimStatus(value.Status),
			EvidenceRefs: evidenceRefs,
		}
		if claim.Claim == "" && claim.Observable == "" && claim.Threshold == "" {
			continue
		}

		claim.ID = uniqueDecisionClaimID(claim.ID, len(claims), seenIDs)
		seenIDs[claim.ID] = struct{}{}
		claims = append(claims, claim)
	}

	if len(claims) == 0 {
		return nil
	}

	return claims
}

func uniqueDecisionClaimID(candidate string, position int, seenIDs map[string]struct{}) string {
	candidate = strings.TrimSpace(candidate)

	if candidate != "" {
		if _, exists := seenIDs[candidate]; !exists {
			return candidate
		}
	}

	next := position
	for {
		generated := canonicalDecisionClaimID(next)
		if _, exists := seenIDs[generated]; !exists {
			return generated
		}
		next++
	}
}

func decisionPredictionsFromClaims(values []DecisionClaim) []DecisionPrediction {
	claims := normalizeDecisionClaims(values)
	if len(claims) == 0 {
		return nil
	}

	predictions := make([]DecisionPrediction, 0, len(claims))

	for _, claim := range claims {
		predictions = append(predictions, DecisionPrediction{
			Claim:      claim.Claim,
			Observable: claim.Observable,
			Threshold:  claim.Threshold,
			Status:     claim.Status,
		})
	}

	return predictions
}

func newDecisionPredictions(inputs []PredictionInput) []DecisionPrediction {
	return decisionPredictionsFromClaims(newDecisionClaims(inputs))
}

func canonicalDecisionClaimID(index int) string {
	return fmt.Sprintf("claim-%03d", index+1)
}

func normalizeDecisionPredictions(values []DecisionPrediction) []DecisionPrediction {
	return decisionPredictionsFromClaims(decisionClaimsFromPredictions(values))
}

// ClaimStatusFromPredictionMeasureMatch maps one measurement-to-prediction relation
// into the smallest claim status vocabulary used by the runtime.
func ClaimStatusFromPredictionMeasureMatch(match PredictionMeasureMatch) ClaimStatus {
	if match.CriteriaMet && match.CriteriaNotMet {
		return ClaimStatusWeakened
	}
	if match.CriteriaMet {
		return ClaimStatusSupported
	}
	if match.CriteriaNotMet {
		return ClaimStatusRefuted
	}
	if match.MeasurementRecorded {
		return ClaimStatusInconclusive
	}

	return ClaimStatusUnverified
}

func adjudicateDecisionClaims(
	claims []DecisionClaim,
	measurementRecorded bool,
	criteriaMet []string,
	criteriaMetScope []string,
	criteriaNotMet []string,
	criteriaNotMetScope []string,
) []DecisionClaim {
	normalized := normalizeDecisionClaims(claims)
	if len(normalized) == 0 {
		return nil
	}

	aliasIndex := buildDecisionClaimAliasIndex(normalized)
	metMatches := matchPredictionCriteria(aliasIndex, len(normalized), criteriaMet, criteriaMetScope)
	notMetMatches := matchPredictionCriteria(aliasIndex, len(normalized), criteriaNotMet, criteriaNotMetScope)
	updated := make([]DecisionClaim, 0, len(normalized))

	for index, claim := range normalized {
		match := PredictionMeasureMatch{
			MeasurementRecorded: measurementRecorded,
			CriteriaMet:         metMatches[index],
			CriteriaNotMet:      notMetMatches[index],
		}
		claim.Status = ClaimStatusFromPredictionMeasureMatch(match)
		updated = append(updated, claim)
	}

	return updated
}

func buildDecisionClaimAliasIndex(claims []DecisionClaim) map[string]int {
	counts := make(map[string]int)
	aliases := make(map[string]int)

	for index, claim := range claims {
		for _, key := range decisionClaimAliasKeys(claim) {
			counts[key]++
			if _, exists := aliases[key]; exists {
				continue
			}
			aliases[key] = index
		}
	}

	index := make(map[string]int)

	for key, predictionIndex := range aliases {
		if counts[key] != 1 {
			continue
		}
		index[key] = predictionIndex
	}

	return index
}

func matchPredictionCriteria(
	aliasIndex map[string]int,
	predictionCount int,
	criteria []string,
	scope []string,
) []bool {
	matches := make([]bool, predictionCount)
	criteriaValues := make([]string, 0, len(criteria)+len(scope))
	criteriaValues = append(criteriaValues, criteria...)
	criteriaValues = append(criteriaValues, scope...)

	for _, value := range criteriaValues {
		predictionIndex, ok := resolvePredictionAlias(value, aliasIndex)
		if !ok {
			continue
		}
		matches[predictionIndex] = true
	}

	return matches
}

func resolvePredictionAlias(value string, aliasIndex map[string]int) (int, bool) {
	for _, key := range criterionAliasKeys(value) {
		predictionIndex, ok := aliasIndex[key]
		if ok {
			return predictionIndex, true
		}
	}

	return 0, false
}

func decisionClaimAliasKeys(claim DecisionClaim) []string {
	values := decisionClaimAliasValues(claim)
	keys := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		key := criterionMatchKey(value)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	return keys
}

func decisionClaimAliasValues(node DecisionClaim) []string {
	claim := strings.TrimSpace(node.Claim)
	observable := strings.TrimSpace(node.Observable)
	threshold := strings.TrimSpace(node.Threshold)
	values := make([]string, 0, 6)

	if claim != "" {
		values = append(values, claim)
	}
	if observable != "" {
		values = append(values, observable)
	}
	if claim != "" && observable != "" {
		values = append(values, claim+" "+observable)
	}
	if claim != "" && threshold != "" {
		values = append(values, claim+" "+threshold)
	}
	if observable != "" && threshold != "" {
		values = append(values, observable+" "+threshold)
	}
	if claim != "" && observable != "" && threshold != "" {
		values = append(values, claim+" "+observable+" "+threshold)
	}

	return values
}
