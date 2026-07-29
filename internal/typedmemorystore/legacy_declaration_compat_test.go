package typedmemorystore

import "context"

// commitDeclareEntity keeps historical package-internal regression scenarios
// on the current generic admission writer. No second declaration writer is
// compiled into the runtime.
func (adapter *SQLiteAdapter) commitDeclareEntity(
	ctx context.Context,
	request CommitRequest,
) (CommitReceipt, error) {
	return adapter.CommitMemoryChangeSet(ctx, request)
}
