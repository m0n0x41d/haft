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
