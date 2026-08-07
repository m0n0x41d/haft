package racequalification

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestAggregatePassesOnlyWithEveryShardObservedExactlyOnce(t *testing.T) {
	t.Parallel()

	built, err := Build(
		Discovery{
			WholePackages: []PackageID{
				mustPackageID(t, "github.com/m0n0x41d/haft/internal/only"),
			},
			SplitPackages: []SplitPackageDiscovery{},
		},
		4,
	)
	if err != nil {
		t.Fatal(err)
	}
	observations := passingObservations(built.Plan())
	slices.Reverse(observations)

	result, err := Aggregate(built.Plan(), observations)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != AggregatePassed {
		t.Fatalf("status = %q, want %q", result.Status, AggregatePassed)
	}
	if len(result.FailedShards) != 0 {
		t.Fatalf("failed shards = %#v, want none", result.FailedShards)
	}
	for index, observation := range result.Observations {
		if observation.ShardIndex != index {
			t.Fatalf(
				"observation position %d has shard %d",
				index,
				observation.ShardIndex,
			)
		}
	}
}

func TestAggregateFailsForFailedTimedOutOrInterruptedShard(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []ShardStatus{
		ShardFailed,
		ShardTimedOut,
		ShardInterrupted,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			observations := passingObservations(built.Plan())
			observations[1].Status = status

			result, err := Aggregate(built.Plan(), observations)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != AggregateFailed {
				t.Fatalf("status = %q, want %q", result.Status, AggregateFailed)
			}
			if !slices.Equal(result.FailedShards, []int{1}) {
				t.Fatalf("failed shards = %#v, want [1]", result.FailedShards)
			}
		})
	}
}

func TestAggregateRejectsMissingObservationEvenForEmptyShard(t *testing.T) {
	t.Parallel()

	built, err := Build(
		Discovery{
			WholePackages: []PackageID{
				mustPackageID(t, "github.com/m0n0x41d/haft/internal/only"),
			},
			SplitPackages: []SplitPackageDiscovery{},
		},
		3,
	)
	if err != nil {
		t.Fatal(err)
	}
	observations := passingObservations(built.Plan())
	observations = observations[:2]

	result, err := Aggregate(built.Plan(), observations)
	if err == nil || !strings.Contains(err.Error(), "want 3") {
		t.Fatalf("error = %v, want missing observation rejection", err)
	}
	if result.Status != AggregateInvalid {
		t.Fatalf("status = %q, want %q", result.Status, AggregateInvalid)
	}
}

func TestAggregateRejectsDuplicateObservation(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	observations := passingObservations(built.Plan())
	observations[2] = observations[0]

	result, err := Aggregate(built.Plan(), observations)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want duplicate rejection", err)
	}
	if result.Status != AggregateInvalid {
		t.Fatalf("status = %q, want %q", result.Status, AggregateInvalid)
	}
}

func TestAggregateRejectsOutOfRangeMismatchedAndUnknownObservations(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		name   string
		mutate func([]ShardObservation)
		want   string
	}{
		{
			name: "out of range",
			mutate: func(observations []ShardObservation) {
				observations[0].ShardIndex = 3
			},
			want: "outside",
		},
		{
			name: "wrong plan",
			mutate: func(observations []ShardObservation) {
				observations[0].PlanDigest =
					"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			},
			want: "bound to plan",
		},
		{
			name: "unknown status",
			mutate: func(observations []ShardObservation) {
				observations[0].Status = "cancelled"
			},
			want: "unknown terminal status",
		},
	}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			observations := passingObservations(built.Plan())
			fixture.mutate(observations)
			result, err := Aggregate(built.Plan(), observations)
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("error = %v, want containing %q", err, fixture.want)
			}
			if result.Status != AggregateInvalid {
				t.Fatalf("status = %q, want %q", result.Status, AggregateInvalid)
			}
		})
	}
}

func TestAggregateRejectsInvalidPlan(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 2)
	if err != nil {
		t.Fatal(err)
	}
	projection := built.Plan()
	observations := passingObservations(projection)
	projection.PlanDigest =
		"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	result, err := Aggregate(projection, observations)
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("error = %v, want invalid plan rejection", err)
	}
	if result.Status != AggregateInvalid {
		t.Fatalf("status = %q, want %q", result.Status, AggregateInvalid)
	}
}

func TestAggregateJSONIsDeterministicAndMachineReadable(t *testing.T) {
	t.Parallel()

	built, err := Build(sampleDiscovery(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	forward := passingObservations(built.Plan())
	reversed := append([]ShardObservation{}, forward...)
	slices.Reverse(reversed)

	first, err := Aggregate(built.Plan(), forward)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Aggregate(built.Plan(), reversed)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf(
			"input ordering changed aggregate JSON\nfirst:  %s\nsecond: %s",
			firstJSON,
			secondJSON,
		)
	}

	var projection AggregateResult
	if err := json.Unmarshal(firstJSON, &projection); err != nil {
		t.Fatal(err)
	}
	if projection.Schema != AggregateSchema ||
		projection.PlanDigest != built.Digest() ||
		projection.Status != AggregatePassed ||
		projection.Observations == nil ||
		projection.FailedShards == nil {
		t.Fatalf("aggregate JSON projection = %#v", projection)
	}
}

func passingObservations(plan Plan) []ShardObservation {
	observations := make([]ShardObservation, plan.ShardCount)
	for index := range observations {
		observations[index] = ShardObservation{
			PlanDigest: plan.PlanDigest,
			ShardIndex: index,
			Status:     ShardPassed,
		}
	}
	return observations
}
