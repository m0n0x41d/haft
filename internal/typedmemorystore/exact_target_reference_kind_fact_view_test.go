package typedmemorystore

import "testing"

func TestExactTargetReferenceKindRuntimePostureIsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		memberOf       int
		classification int
		want           exactTargetReferenceKindRuntimePosture
	}{
		{
			name: "explicit none",
			want: exactTargetReferenceKindRuntimeNone,
		},
		{
			name:     "sealed historical",
			memberOf: 1,
			want:     exactTargetReferenceKindRuntimeHistorical,
		},
		{
			name:           "current classification",
			classification: 1,
			want:           exactTargetReferenceKindRuntimeCurrent,
		},
		{
			name:           "mixed is invalid",
			memberOf:       1,
			classification: 1,
			want:           exactTargetReferenceKindRuntimeMixed,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := exactTargetReferenceKindRuntimePostureForCounts(
				test.memberOf,
				test.classification,
			)
			if got != test.want {
				t.Fatalf("posture = %d; want %d", got, test.want)
			}
		})
	}
}
