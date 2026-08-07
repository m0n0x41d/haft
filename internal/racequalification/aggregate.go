package racequalification

import (
	"fmt"
	"slices"
	"strings"
)

const (
	// AggregateSchema identifies the machine-readable aggregate result.
	AggregateSchema = "haft.race-qualification.aggregate/v1"
)

// ShardStatus is the terminal execution status of one explicit shard.
type ShardStatus string

const (
	ShardPassed      ShardStatus = "passed"
	ShardFailed      ShardStatus = "failed"
	ShardTimedOut    ShardStatus = "timed_out"
	ShardInterrupted ShardStatus = "interrupted"
)

// ShardObservation binds one terminal shard result to one exact plan.
type ShardObservation struct {
	PlanDigest PlanDigest  `json:"plan_digest"`
	ShardIndex int         `json:"shard_index"`
	Status     ShardStatus `json:"status"`
}

// AggregateStatus is the terminal posture of the complete shard set.
type AggregateStatus string

const (
	AggregatePassed  AggregateStatus = "passed"
	AggregateFailed  AggregateStatus = "failed"
	AggregateInvalid AggregateStatus = "invalid"
)

// AggregateResult is a deterministic machine-readable projection. Invalid
// input also returns an error explaining why it could not be a complete result.
type AggregateResult struct {
	Schema       string             `json:"schema"`
	PlanDigest   PlanDigest         `json:"plan_digest"`
	Status       AggregateStatus    `json:"status"`
	Observations []ShardObservation `json:"observations"`
	FailedShards []int              `json:"failed_shards"`
}

// Aggregate accepts a result only when every explicit shard appears exactly
// once and is bound to the same validated plan. Failed, timed-out, and
// interrupted shards produce a valid failed aggregate, never a pass.
func Aggregate(plan Plan, observations []ShardObservation) (AggregateResult, error) {
	result := AggregateResult{
		Schema:       AggregateSchema,
		PlanDigest:   plan.PlanDigest,
		Status:       AggregateInvalid,
		Observations: canonicalObservations(observations),
		FailedShards: []int{},
	}
	if err := Validate(plan); err != nil {
		return result, fmt.Errorf("aggregate plan: %w", err)
	}
	if len(observations) != plan.ShardCount {
		return result, fmt.Errorf(
			"aggregate has %d shard observations, want %d",
			len(observations),
			plan.ShardCount,
		)
	}

	byIndex := make(map[int]ShardObservation, plan.ShardCount)
	for _, observation := range observations {
		if observation.PlanDigest != plan.PlanDigest {
			return result, fmt.Errorf(
				"shard %d is bound to plan %q, want %q",
				observation.ShardIndex,
				observation.PlanDigest,
				plan.PlanDigest,
			)
		}
		if observation.ShardIndex < 0 ||
			observation.ShardIndex >= plan.ShardCount {
			return result, fmt.Errorf(
				"shard observation index %d is outside [0,%d)",
				observation.ShardIndex,
				plan.ShardCount,
			)
		}
		if !validShardStatus(observation.Status) {
			return result, fmt.Errorf(
				"shard %d has unknown terminal status %q",
				observation.ShardIndex,
				observation.Status,
			)
		}
		if _, exists := byIndex[observation.ShardIndex]; exists {
			return result, fmt.Errorf(
				"shard %d has duplicate observations",
				observation.ShardIndex,
			)
		}
		byIndex[observation.ShardIndex] = observation
	}

	for index := 0; index < plan.ShardCount; index++ {
		observation, exists := byIndex[index]
		if !exists {
			return result, fmt.Errorf("shard %d observation is missing", index)
		}
		if observation.Status != ShardPassed {
			result.FailedShards = append(result.FailedShards, index)
		}
	}
	if len(result.FailedShards) > 0 {
		result.Status = AggregateFailed
		return result, nil
	}
	result.Status = AggregatePassed
	return result, nil
}

func validShardStatus(status ShardStatus) bool {
	switch status {
	case ShardPassed, ShardFailed, ShardTimedOut, ShardInterrupted:
		return true
	default:
		return false
	}
}

func canonicalObservations(observations []ShardObservation) []ShardObservation {
	canonical := append([]ShardObservation{}, observations...)
	slices.SortFunc(canonical, func(left, right ShardObservation) int {
		if left.ShardIndex < right.ShardIndex {
			return -1
		}
		if left.ShardIndex > right.ShardIndex {
			return 1
		}
		if compared := strings.Compare(string(left.Status), string(right.Status)); compared != 0 {
			return compared
		}
		return strings.Compare(left.PlanDigest.String(), right.PlanDigest.String())
	})
	return canonical
}
