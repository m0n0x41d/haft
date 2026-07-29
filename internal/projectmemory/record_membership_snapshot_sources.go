package projectmemory

import (
	"strings"

	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

const recordMembershipObservablePrefix = "record-membership-source:"

var _ typedmemorystore.SnapshotObservableInputSelector = RecordMembershipAdmissionEngine{}

// SelectSnapshotObservableInputs interprets only the record-membership source
// format owned by projectmemory. The lower storage layer supplies immutable
// bytes but never learns this format. Exactly one verified source for the
// query's project/entity/context is required; absence and ambiguity remain
// unavailable and therefore fail closed as MemberOfUndefined.
func (engine RecordMembershipAdmissionEngine) SelectSnapshotObservableInputs(
	input typedmemorystore.MemberOfEvaluationInput,
) typedmemorystore.SnapshotObservableInputSelection {
	query := input.Request().Query()
	selected := make([]typedmemorystore.ObservableInputBlob, 0, 1)
	for _, blob := range input.ObservableInputs() {
		if !strings.HasPrefix(
			blob.Reference().String(),
			recordMembershipObservablePrefix,
		) {
			continue
		}
		expected, err := typedmemory.NewMemberOfObservableInput(
			blob.Reference(),
			blob.Digest(),
		)
		if err != nil {
			return typedmemorystore.NewSnapshotObservableInputsUnavailable()
		}
		source, err := recordcarrier.VerifyRecordMembershipSourceV1(
			expected,
			blob.Bytes(),
		)
		if err != nil {
			return typedmemorystore.NewSnapshotObservableInputsUnavailable()
		}
		if source.ProjectID().String() != input.ProjectID().String() ||
			source.EntityID() != query.EntityID() ||
			source.BoundedContext() != query.ContextSlice().Context() {
			continue
		}
		selected = append(selected, blob)
	}
	if len(selected) != 1 {
		return typedmemorystore.NewSnapshotObservableInputsUnavailable()
	}
	result, err := typedmemorystore.NewSnapshotObservableInputsSelected(selected)
	if err != nil {
		return typedmemorystore.NewSnapshotObservableInputsUnavailable()
	}
	return result
}
