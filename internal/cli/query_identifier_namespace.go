package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/fpf"
)

const wrongIdentifierNamespaceCode = "wrong_identifier_namespace"

type exactQueryRecoveryArguments struct {
	Action          string                   `json:"action"`
	Mode            string                   `json:"mode,omitempty"`
	ContractVersion string                   `json:"contract_version,omitempty"`
	Basis           *exactQueryRecoveryBasis `json:"basis,omitempty"`
	ArtifactRef     string                   `json:"artifact_ref,omitempty"`
	Identifier      string                   `json:"identifier,omitempty"`
	Query           string                   `json:"query,omitempty"`
	Symbol          string                   `json:"symbol,omitempty"`
}

type exactQueryRecoveryBasis struct {
	Kind string `json:"kind"`
}

type exactQueryRecoveryCall struct {
	Tool      string                      `json:"tool"`
	Arguments exactQueryRecoveryArguments `json:"arguments"`
}

type wrongIdentifierNamespace struct {
	Code              string                 `json:"code"`
	Tool              string                 `json:"tool"`
	Action            string                 `json:"action"`
	Parameter         string                 `json:"parameter"`
	Identifier        string                 `json:"identifier"`
	ReceivedNamespace string                 `json:"received_namespace"`
	ExpectedNamespace string                 `json:"expected_namespace"`
	SameCallRetryable bool                   `json:"same_call_retryable"`
	Message           string                 `json:"message"`
	RecoveryCall      exactQueryRecoveryCall `json:"recovery_call"`
}

type wrongIdentifierNamespaceError struct {
	payload wrongIdentifierNamespace
}

func (failure wrongIdentifierNamespaceError) Error() string {
	encoded, err := json.Marshal(failure.payload)
	if err != nil {
		return fmt.Sprintf(
			`{"code":"wrong_identifier_namespace","tool":"haft_query","action":%q,"same_call_retryable":false}`,
			failure.payload.Action,
		)
	}
	return string(encoded)
}

func rejectArtifactIDAsCodeSymbol(
	action string,
	parameter string,
	identifier string,
) error {
	if !artifact.IsCanonicalArtifactID(identifier) {
		return nil
	}

	return wrongIdentifierNamespaceError{
		payload: wrongIdentifierNamespace{
			Code:              wrongIdentifierNamespaceCode,
			Tool:              "haft_query",
			Action:            action,
			Parameter:         parameter,
			Identifier:        identifier,
			ReceivedNamespace: "haft_artifact_id",
			ExpectedNamespace: "code_symbol",
			SameCallRetryable: false,
			Message:           "A Haft artifact ID cannot be resolved by a code-symbol action. Use the exact artifact recovery call; retrying this action with another artifact ID cannot succeed.",
			RecoveryCall: exactQueryRecoveryCall{
				Tool: "haft_query",
				Arguments: exactQueryRecoveryArguments{
					Action:      "related",
					ArtifactRef: identifier,
				},
			},
		},
	}
}

func rejectWrongIdentifierNamespaceForQueryAction(
	ctx context.Context,
	store *artifact.Store,
	action string,
	args map[string]any,
) error {
	var identifier string
	var parameter string
	switch action {
	case "node", "callees", "callers", "impact", "explore":
		identifier, parameter = firstPresentStringArg(args, "symbol", "name")
	case "code_context":
		identifier, parameter = firstPresentStringArg(args, "symbol")
	case "related":
		identifier, parameter = firstPresentStringArg(
			args,
			"artifact_ref",
			"ref",
			"artifact_id",
		)
		return rejectNonArtifactIdentifierAsArtifact(
			ctx,
			store,
			action,
			parameter,
			identifier,
		)
	default:
		return nil
	}
	if identifier == "" {
		return nil
	}
	if err := rejectArtifactIDAsCodeSymbol(action, parameter, identifier); err != nil {
		return err
	}
	return rejectEntityIDAsCodeSymbol(action, parameter, identifier)
}

func rejectEntityIDAsCodeSymbol(
	action string,
	parameter string,
	identifier string,
) error {
	if !isTypedMemoryEntityID(identifier) {
		return nil
	}

	basis := exactQueryRecoveryBasis{Kind: "project_current"}
	return wrongIdentifierNamespaceError{
		payload: wrongIdentifierNamespace{
			Code:              wrongIdentifierNamespaceCode,
			Tool:              "haft_query",
			Action:            action,
			Parameter:         parameter,
			Identifier:        identifier,
			ReceivedNamespace: "typed_memory_entity_id",
			ExpectedNamespace: "code_symbol",
			SameCallRetryable: false,
			Message:           "A typed-memory EntityID cannot be resolved by a code-symbol action. Resolve the unchanged identifier through project memory; retrying this code action cannot succeed.",
			RecoveryCall: exactQueryRecoveryCall{
				Tool: "haft_query",
				Arguments: exactQueryRecoveryArguments{
					Action:          "memory",
					Mode:            "resolve",
					ContractVersion: "haft.memory.v1",
					Basis:           &basis,
					Query:           identifier,
				},
			},
		},
	}
}

func rejectNonArtifactIdentifierAsArtifact(
	ctx context.Context,
	store *artifact.Store,
	action string,
	parameter string,
	identifier string,
) error {
	if identifier == "" || artifact.IsCanonicalArtifactID(identifier) {
		return nil
	}

	sourceIdentifier, err := isExactFPFSourceIdentifier(identifier)
	if err != nil {
		return fmt.Errorf("inspect FPF identifier namespace: %w", err)
	}
	if sourceIdentifier {
		return wrongSourceIdentifierAsArtifact(action, parameter, identifier)
	}

	codeSymbol, err := isExactIndexedCodeSymbol(ctx, store, identifier)
	if err != nil {
		return fmt.Errorf("inspect code-symbol identifier namespace: %w", err)
	}
	if codeSymbol {
		return wrongCodeSymbolAsArtifact(action, parameter, identifier)
	}
	return nil
}

func wrongSourceIdentifierAsArtifact(
	action string,
	parameter string,
	identifier string,
) error {
	return wrongIdentifierNamespaceError{
		payload: wrongIdentifierNamespace{
			Code:              wrongIdentifierNamespaceCode,
			Tool:              "haft_query",
			Action:            action,
			Parameter:         parameter,
			Identifier:        identifier,
			ReceivedNamespace: "fpf_source_identifier",
			ExpectedNamespace: "haft_artifact_id",
			SameCallRetryable: false,
			Message:           "An exact FPF source identifier cannot be recovered as a Haft artifact. Inspect the unchanged source identifier through the source-native FPF surface; artifact lookup and semantic fallback are not used.",
			RecoveryCall: exactQueryRecoveryCall{
				Tool: "haft_query",
				Arguments: exactQueryRecoveryArguments{
					Action:     "fpf",
					Mode:       "inspect",
					Identifier: identifier,
				},
			},
		},
	}
}

func wrongCodeSymbolAsArtifact(
	action string,
	parameter string,
	identifier string,
) error {
	return wrongIdentifierNamespaceError{
		payload: wrongIdentifierNamespace{
			Code:              wrongIdentifierNamespaceCode,
			Tool:              "haft_query",
			Action:            action,
			Parameter:         parameter,
			Identifier:        identifier,
			ReceivedNamespace: "code_symbol",
			ExpectedNamespace: "haft_artifact_id",
			SameCallRetryable: false,
			Message:           "An exact indexed code symbol cannot be recovered as a Haft artifact. Resolve the unchanged symbol through the code node action; artifact lookup and semantic fallback are not used.",
			RecoveryCall: exactQueryRecoveryCall{
				Tool: "haft_query",
				Arguments: exactQueryRecoveryArguments{
					Action: "node",
					Symbol: identifier,
				},
			},
		},
	}
}

func isTypedMemoryEntityID(identifier string) bool {
	value := strings.TrimSpace(identifier)
	return value == identifier &&
		strings.HasPrefix(value, "entity:") &&
		len(value) > len("entity:") &&
		!strings.ContainsAny(value, "\r\n\t ")
}

func isExactFPFSourceIdentifier(identifier string) (bool, error) {
	result, err := queryEmbeddedFPF(fpf.InspectQuery{Identifier: identifier})
	if err != nil {
		return false, err
	}
	hit, exact := result.(fpf.ExactHit)
	if !exact {
		return false, nil
	}
	identifiers := []string{
		hit.Unit.UnitID,
		hit.Unit.SourceID,
		hit.Unit.PatternID,
	}
	for _, candidate := range identifiers {
		if candidate != "" && strings.EqualFold(candidate, identifier) {
			return true, nil
		}
	}
	return false, nil
}

func isExactIndexedCodeSymbol(
	ctx context.Context,
	store *artifact.Store,
	identifier string,
) (bool, error) {
	if store == nil {
		return false, nil
	}
	count := 0
	err := store.DB().QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'code_symbols'`,
	).Scan(&count)
	if err != nil || count == 0 {
		return false, err
	}

	bareName := identifier
	receiver := ""
	if separator := strings.LastIndex(identifier, "."); separator > 0 {
		receiver = identifier[:separator]
		bareName = identifier[separator+1:]
	}

	query := `SELECT COUNT(*) FROM code_symbols WHERE name = ?`
	arguments := []any{bareName}
	if receiver != "" {
		query += ` AND receiver = ?`
		arguments = append(arguments, receiver)
	}
	err = store.DB().QueryRowContext(
		ctx,
		query,
		arguments...,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func firstPresentStringArg(
	args map[string]any,
	keys ...string,
) (string, string) {
	for _, key := range keys {
		value, present := args[key].(string)
		if !present || value == "" {
			continue
		}
		return value, key
	}
	return "", ""
}
