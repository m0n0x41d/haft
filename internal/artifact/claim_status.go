package artifact

import "strings"

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

func newDecisionPredictions(inputs []PredictionInput) []DecisionPrediction {
	predictions := make([]DecisionPrediction, 0, len(inputs))

	for _, input := range inputs {
		prediction := DecisionPrediction{
			Claim:      strings.TrimSpace(input.Claim),
			Observable: strings.TrimSpace(input.Observable),
			Threshold:  strings.TrimSpace(input.Threshold),
			Status:     ClaimStatusUnverified,
		}
		if prediction.Claim == "" && prediction.Observable == "" && prediction.Threshold == "" {
			continue
		}

		predictions = append(predictions, prediction)
	}

	if len(predictions) == 0 {
		return nil
	}

	return predictions
}

func normalizeDecisionPredictions(values []DecisionPrediction) []DecisionPrediction {
	predictions := make([]DecisionPrediction, 0, len(values))

	for _, value := range values {
		prediction := DecisionPrediction{
			Claim:      strings.TrimSpace(value.Claim),
			Observable: strings.TrimSpace(value.Observable),
			Threshold:  strings.TrimSpace(value.Threshold),
			Status:     normalizeClaimStatus(value.Status),
		}
		if prediction.Claim == "" && prediction.Observable == "" && prediction.Threshold == "" {
			continue
		}

		predictions = append(predictions, prediction)
	}

	if len(predictions) == 0 {
		return nil
	}

	return predictions
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

func adjudicateDecisionPredictions(
	predictions []DecisionPrediction,
	measurementRecorded bool,
	criteriaMet []string,
	criteriaMetScope []string,
	criteriaNotMet []string,
	criteriaNotMetScope []string,
) []DecisionPrediction {
	normalized := normalizeDecisionPredictions(predictions)
	if len(normalized) == 0 {
		return nil
	}

	aliasIndex := buildPredictionAliasIndex(normalized)
	metMatches := matchPredictionCriteria(aliasIndex, len(normalized), criteriaMet, criteriaMetScope)
	notMetMatches := matchPredictionCriteria(aliasIndex, len(normalized), criteriaNotMet, criteriaNotMetScope)
	updated := make([]DecisionPrediction, 0, len(normalized))

	for index, prediction := range normalized {
		match := PredictionMeasureMatch{
			MeasurementRecorded: measurementRecorded,
			CriteriaMet:         metMatches[index],
			CriteriaNotMet:      notMetMatches[index],
		}
		prediction.Status = ClaimStatusFromPredictionMeasureMatch(match)
		updated = append(updated, prediction)
	}

	return updated
}

func buildPredictionAliasIndex(predictions []DecisionPrediction) map[string]int {
	counts := make(map[string]int)
	aliases := make(map[string]int)

	for index, prediction := range predictions {
		for _, key := range predictionAliasKeys(prediction) {
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

func predictionAliasKeys(prediction DecisionPrediction) []string {
	values := predictionAliasValues(prediction)
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

func predictionAliasValues(prediction DecisionPrediction) []string {
	claim := strings.TrimSpace(prediction.Claim)
	observable := strings.TrimSpace(prediction.Observable)
	threshold := strings.TrimSpace(prediction.Threshold)
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
