package fpfrefresh

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// CandidateTokenGateDatabasePathEnvironment selects the exact database
	// image exercised by the query-token acceptance test.
	CandidateTokenGateDatabasePathEnvironment = "HAFT_QUERY_TOKEN_GATE_DATABASE_PATH" // #nosec G101 -- environment-variable name, not a credential.

	// CandidateTokenGateLockPathEnvironment selects the canonical generated
	// integration lock that binds the candidate database and fixture.
	CandidateTokenGateLockPathEnvironment = "HAFT_QUERY_TOKEN_GATE_LOCK_PATH" // #nosec G101 -- environment-variable name, not a credential.

	// CandidateTokenGateFixturePathEnvironment selects the exact
	// human-reviewed behavior corpus exercised by the query-token acceptance
	// test.
	CandidateTokenGateFixturePathEnvironment = "HAFT_QUERY_TOKEN_GATE_FIXTURE_PATH" // #nosec G101 -- environment-variable name, not a credential.
)

// CandidateTokenGateInput names the complete candidate byte set exercised by
// one token-budget acceptance run. The paths are explicit adapter inputs; the
// gate must not discover or substitute repository-current artifacts.
type CandidateTokenGateInput struct {
	DatabasePath        string
	IntegrationLockPath string
	FixturePath         string
}

// CandidateTokenGate is the domain port for exercising query behavior and
// token-budget acceptance against one exact candidate artifact.
type CandidateTokenGate interface {
	VerifyCandidate(ctx context.Context, input CandidateTokenGateInput) error
}

// CandidateTokenGateFunc adapts a function to CandidateTokenGate.
type CandidateTokenGateFunc func(
	ctx context.Context,
	input CandidateTokenGateInput,
) error

func (function CandidateTokenGateFunc) VerifyCandidate(
	ctx context.Context,
	input CandidateTokenGateInput,
) error {
	if function == nil {
		return fmt.Errorf("candidate token-gate function is required")
	}
	return function(ctx, input)
}

func normalizeCandidateTokenGateInput(
	input CandidateTokenGateInput,
) (CandidateTokenGateInput, error) {
	normalized := input
	fields := []struct {
		name  string
		value *string
	}{
		{name: "database path", value: &normalized.DatabasePath},
		{name: "integration-lock path", value: &normalized.IntegrationLockPath},
		{name: "fixture path", value: &normalized.FixturePath},
	}
	for _, field := range fields {
		trimmed := strings.TrimSpace(*field.value)
		if trimmed == "" {
			return CandidateTokenGateInput{}, fmt.Errorf(
				"candidate token-gate %s is required",
				field.name,
			)
		}
		if trimmed != *field.value || !filepath.IsAbs(trimmed) {
			return CandidateTokenGateInput{}, fmt.Errorf(
				"candidate token-gate %s must be a trimmed absolute path",
				field.name,
			)
		}
		cleaned := filepath.Clean(trimmed)
		if cleaned != trimmed {
			return CandidateTokenGateInput{}, fmt.Errorf(
				"candidate token-gate %s must be canonical",
				field.name,
			)
		}
		*field.value = cleaned
	}
	return normalized, nil
}
