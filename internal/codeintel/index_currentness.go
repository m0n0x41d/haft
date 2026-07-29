package codeintel

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/codebase"
)

const indexQueryAttemptLimit = 2

// IndexBasisChangedError is the typed retry boundary for a query whose several
// store reads did not all belong to one published code-index basis.
type IndexBasisChangedError struct {
	Before codebase.IndexState
	After  codebase.IndexState
}

func (e *IndexBasisChangedError) Error() string {
	return fmt.Sprintf(
		"code_index_basis_changed: retry required (epoch %d basis %s -> epoch %d basis %s)",
		e.Before.Epoch,
		e.Before.Basis.CoverageRef(),
		e.After.Epoch,
		e.After.Basis.CoverageRef(),
	)
}

func (e *IndexBasisChangedError) retryableIndexQuery() {}

// ConcernGraphBasisChangedError rejects a fused result whose reasoning graph or
// lexical artifact seed set changed while the query was being assembled.
type ConcernGraphBasisChangedError struct {
	Before string
	After  string
}

func (e *ConcernGraphBasisChangedError) Error() string {
	return fmt.Sprintf(
		"concern_graph_basis_changed: retry required (%s -> %s)",
		e.Before,
		e.After,
	)
}

func (e *ConcernGraphBasisChangedError) retryableIndexQuery() {}

type retryableIndexQueryError interface {
	error
	retryableIndexQuery()
}

// CurrentIndexState returns one transactionally read code-index state.
func (s *Service) CurrentIndexState(
	ctx context.Context,
) (codebase.IndexState, error) {
	return s.scanner.CurrentIndexState(ctx)
}

// ConfirmIndexState rejects a result assembled while another process published
// or degraded the current code-index basis.
func (s *Service) ConfirmIndexState(
	ctx context.Context,
	before codebase.IndexState,
) error {
	if s.beforeBasisConfirm != nil {
		if err := s.beforeBasisConfirm(ctx); err != nil {
			return err
		}
	}
	after, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return err
	}
	if before.SameCurrentBasis(after) {
		return nil
	}
	return &IndexBasisChangedError{
		Before: before,
		After:  after,
	}
}

func retryIndexQuery[T any](
	run func() (T, error),
) (T, error) {
	var zero T
	var lastErr error
	for range indexQueryAttemptLimit {
		result, err := run()
		if err == nil {
			return result, nil
		}
		var changed retryableIndexQueryError
		if !errors.As(err, &changed) {
			return zero, err
		}
		lastErr = err
	}
	return zero, lastErr
}
