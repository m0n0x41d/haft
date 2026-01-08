package fpf

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrCodeDatabaseNotInitialized ErrorCode = "DATABASE_NOT_INITIALIZED"
	ErrCodeHolonNotFound          ErrorCode = "HOLON_NOT_FOUND"
	ErrCodeContextClosed          ErrorCode = "CONTEXT_CLOSED"
	ErrCodeHypothesisInUse        ErrorCode = "HYPOTHESIS_IN_USE"
	ErrCodeInvalidVerdict         ErrorCode = "INVALID_VERDICT"
	ErrCodeMissingRequired        ErrorCode = "MISSING_REQUIRED"
	ErrCodeInvalidResolution      ErrorCode = "INVALID_RESOLUTION"
	ErrCodeCyclicDependency       ErrorCode = "CYCLIC_DEPENDENCY"
)

type QuintError struct {
	Code       ErrorCode
	Tool       string
	Message    string
	Suggestion string
}

func (e *QuintError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s: %s. %s", e.Code, e.Message, e.Suggestion)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *QuintError) Is(target error) bool {
	var qe *QuintError
	if errors.As(target, &qe) {
		return e.Code == qe.Code
	}
	return false
}

var ErrDatabaseNotInitialized = &QuintError{
	Code:       ErrCodeDatabaseNotInitialized,
	Message:    "database not initialized",
	Suggestion: "run quint_internalize first",
}

func ErrHolonNotFound(tool, id string) *QuintError {
	return &QuintError{
		Code:    ErrCodeHolonNotFound,
		Tool:    tool,
		Message: fmt.Sprintf("holon '%s' not found", id),
	}
}

func ErrContextClosed(tool, dcID, drrID string) *QuintError {
	return &QuintError{
		Code:       ErrCodeContextClosed,
		Tool:       tool,
		Message:    fmt.Sprintf("decision_context '%s' already closed by DRR '%s'", dcID, drrID),
		Suggestion: "use a different context or create a new one",
	}
}

func ErrHypothesisInUse(tool, hypID, drrID string) *QuintError {
	return &QuintError{
		Code:    ErrCodeHypothesisInUse,
		Tool:    tool,
		Message: fmt.Sprintf("hypothesis '%s' already used in open DRR '%s'", hypID, drrID),
	}
}

func ErrInvalidVerdict(tool, verdict string) *QuintError {
	return &QuintError{
		Code:       ErrCodeInvalidVerdict,
		Tool:       tool,
		Message:    fmt.Sprintf("invalid verdict: %s", verdict),
		Suggestion: "must be PASS, FAIL, or REFINE",
	}
}

func ErrMissingRequired(tool, field string) *QuintError {
	return &QuintError{
		Code:    ErrCodeMissingRequired,
		Tool:    tool,
		Message: fmt.Sprintf("%s is required", field),
	}
}

func ErrCyclicDependency(tool string) *QuintError {
	return &QuintError{
		Code:    ErrCodeCyclicDependency,
		Tool:    tool,
		Message: "link would create dependency cycle",
	}
}
