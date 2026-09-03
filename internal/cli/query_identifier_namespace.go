package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/fpf"
)

const (
	wrongIdentifierNamespaceCode     = "wrong_identifier_namespace"
	exactMemoryRecoveryMaxCandidates = uint32(8)
)

type exactQueryRecoveryArguments struct {
	Action        string                           `json:"action"`
	Mode          string                           `json:"mode,omitempty"`
	ArtifactRef   string                           `json:"artifact_ref,omitempty"`
	Identifier    string                           `json:"identifier,omitempty"`
	Symbol        string                           `json:"symbol,omitempty"`
	AnchorID      string                           `json:"anchor_id,omitempty"`
	MemoryRequest *exactQueryMemoryRecoveryRequest `json:"memory_request,omitempty"`
}

type exactQueryMemoryRecoveryRequest struct {
	ContractVersion string                  `json:"contract_version"`
	Mode            string                  `json:"mode"`
	Basis           exactQueryRecoveryBasis `json:"basis"`
	Query           string                  `json:"query"`
	MaxCandidates   uint32                  `json:"max_candidates"`
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

type exactMemoryIdentifierResolver interface {
	ResolveExactMemoryIdentifier(context.Context, string) (bool, error)
}

type exactFPFIdentifierResolver interface {
	ResolveExactFPFIdentifier(context.Context, string) (bool, error)
}

type exactFPFIdentifierDBOpener func() (*sql.DB, func(), error)

// cachedExactFPFIdentifierResolver materializes the immutable embedded source
// index once per long-lived handler. Typed-memory resolution intentionally
// remains uncached because project memory can change while the server runs.
type cachedExactFPFIdentifierResolver struct {
	mu          sync.RWMutex
	open        exactFPFIdentifierDBOpener
	identifiers map[string]struct{}
}

func newCachedExactFPFIdentifierResolver(
	open exactFPFIdentifierDBOpener,
) exactFPFIdentifierResolver {
	if open == nil {
		return nil
	}
	return &cachedExactFPFIdentifierResolver{open: open}
}

func (resolver *cachedExactFPFIdentifierResolver) ResolveExactFPFIdentifier(
	ctx context.Context,
	identifier string,
) (bool, error) {
	if resolver == nil || resolver.open == nil {
		return false, fmt.Errorf("FPF source identifier resolver is unavailable")
	}
	key := strings.ToLower(identifier)
	resolver.mu.RLock()
	identifiers := resolver.identifiers
	_, exact := identifiers[key]
	resolver.mu.RUnlock()
	if identifiers != nil {
		return exact, nil
	}

	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	if resolver.identifiers == nil {
		loaded, err := loadExactFPFIdentifiers(ctx, resolver.open)
		if err != nil {
			// Do not make a transient extraction or verification failure sticky.
			// Namespace recovery is best-effort and may retry on a later request.
			return false, err
		}
		resolver.identifiers = loaded
	}
	_, exact = resolver.identifiers[key]
	return exact, nil
}

func loadExactFPFIdentifiers(
	ctx context.Context,
	open exactFPFIdentifierDBOpener,
) (map[string]struct{}, error) {
	database, cleanup, err := open()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := fpf.VerifySourceQueryIndexReadOnlyDB(database); err != nil {
		return nil, fmt.Errorf("verify canonical FPF source index: %w", err)
	}
	rows, err := database.QueryContext(
		ctx,
		`SELECT unit_id, source_id, pattern_id FROM source_units`,
	)
	if err != nil {
		return nil, fmt.Errorf("read exact FPF source identifiers: %w", err)
	}
	defer rows.Close()

	identifiers := make(map[string]struct{})
	for rows.Next() {
		var unitID string
		var sourceID string
		var patternID string
		if err := rows.Scan(&unitID, &sourceID, &patternID); err != nil {
			return nil, fmt.Errorf("scan exact FPF source identifiers: %w", err)
		}
		for _, identifier := range []string{unitID, sourceID, patternID} {
			if identifier != "" {
				identifiers[strings.ToLower(identifier)] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exact FPF source identifiers: %w", err)
	}
	return identifiers, nil
}

type exactMemoryIdentifierResolverFunc func(
	context.Context,
	string,
) (bool, error)

func (resolve exactMemoryIdentifierResolverFunc) ResolveExactMemoryIdentifier(
	ctx context.Context,
	identifier string,
) (bool, error) {
	return resolve(ctx, identifier)
}

type queryMemoryExactIdentifierResolver struct {
	handler fpf.MemoryToolHandler
}

func newQueryMemoryExactIdentifierResolver(
	handler fpf.MemoryToolHandler,
) exactMemoryIdentifierResolver {
	if handler == nil {
		return nil
	}
	return queryMemoryExactIdentifierResolver{handler: handler}
}

func (resolver queryMemoryExactIdentifierResolver) ResolveExactMemoryIdentifier(
	ctx context.Context,
	identifier string,
) (bool, error) {
	arguments := memoryRecoveryArguments(identifier)
	payload, err := json.Marshal(arguments)
	if err != nil {
		return false, fmt.Errorf("encode exact memory namespace probe: %w", err)
	}
	result, err := resolver.handler(ctx, payload)
	if err != nil {
		// Memory readiness, snapshot, or runtime failure leaves this alternative
		// namespace unverifiable. It must not block the caller's own read route
		// and must never be converted into a guessed recovery.
		return false, nil
	}
	return exactMemoryResolutionResponse(identifier, []byte(result))
}

type exactIdentifierNamespace string

const (
	exactIdentifierNamespaceArtifact exactIdentifierNamespace = "haft_artifact_id"
	exactIdentifierNamespaceFPF      exactIdentifierNamespace = "fpf_source_identifier"
	exactIdentifierNamespaceCode     exactIdentifierNamespace = "code_symbol"
	exactIdentifierNamespaceMemory   exactIdentifierNamespace = "typed_memory_entity_id"
)

type exactIdentifierNamespaceMatch struct {
	namespace exactIdentifierNamespace
	recovery  exactQueryRecoveryCall
}

type exactCodeIdentifierRoute string

const (
	exactCodeIdentifierSymbol exactCodeIdentifierRoute = "symbol"
	exactCodeIdentifierAnchor exactCodeIdentifierRoute = "anchor_id"
)

func rejectWrongIdentifierNamespaceForQueryAction(
	ctx context.Context,
	store *artifact.Store,
	action string,
	args map[string]any,
	memoryResolver exactMemoryIdentifierResolver,
	fpfResolver exactFPFIdentifierResolver,
) error {
	switch action {
	case "node", "callees", "callers", "impact", "explore", "code_context":
		identifier, parameter := firstPresentStringArg(
			args,
			"symbol",
			"name",
			"anchor_id",
		)
		return rejectNonCodeIdentifierAsCode(
			ctx,
			store,
			action,
			parameter,
			identifier,
			memoryResolver,
			fpfResolver,
		)
	case "related":
		identifier, parameter := firstPresentStringArg(
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
			memoryResolver,
			fpfResolver,
		)
	case "fpf":
		mode := strings.TrimSpace(stringArg(args, "mode"))
		if mode != string(fpf.QueryModeLookup) &&
			mode != string(fpf.QueryModeInspect) {
			return nil
		}
		identifier, parameter := firstPresentStringArg(args, "identifier")
		return rejectNonFPFIdentifierAsSource(
			ctx,
			store,
			action,
			parameter,
			identifier,
			memoryResolver,
			fpfResolver,
		)
	default:
		return nil
	}
}

func rejectNonCodeIdentifierAsCode(
	ctx context.Context,
	store *artifact.Store,
	action string,
	parameter string,
	identifier string,
	memoryResolver exactMemoryIdentifierResolver,
	fpfResolver exactFPFIdentifierResolver,
) error {
	if identifier == "" {
		return nil
	}
	if _, exact := isExactIndexedCodeIdentifier(ctx, store, identifier); exact {
		return nil
	}

	matches := make([]exactIdentifierNamespaceMatch, 0, 3)
	if isExactCanonicalArtifactIdentifier(identifier) {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceArtifact,
			recovery:  artifactRecoveryCall(identifier),
		})
	}
	if exact, _ := resolveExactFPFSourceIdentifier(ctx, fpfResolver, identifier); exact {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceFPF,
			recovery:  fpfRecoveryCall(identifier),
		})
	}
	if exactMemoryIdentifier(ctx, memoryResolver, identifier) {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceMemory,
			recovery:  memoryRecoveryCall(identifier),
		})
	}
	return uniqueWrongIdentifierNamespace(
		action,
		parameter,
		identifier,
		exactIdentifierNamespaceCode,
		matches,
	)
}

func rejectNonArtifactIdentifierAsArtifact(
	ctx context.Context,
	store *artifact.Store,
	action string,
	parameter string,
	identifier string,
	memoryResolver exactMemoryIdentifierResolver,
	fpfResolver exactFPFIdentifierResolver,
) error {
	if identifier == "" || isExactCanonicalArtifactIdentifier(identifier) {
		return nil
	}

	matches := make([]exactIdentifierNamespaceMatch, 0, 3)
	if exact, _ := resolveExactFPFSourceIdentifier(ctx, fpfResolver, identifier); exact {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceFPF,
			recovery:  fpfRecoveryCall(identifier),
		})
	}
	if route, exact := isExactIndexedCodeIdentifier(ctx, store, identifier); exact {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceCode,
			recovery:  codeRecoveryCall(identifier, route),
		})
	}
	if exactMemoryIdentifier(ctx, memoryResolver, identifier) {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceMemory,
			recovery:  memoryRecoveryCall(identifier),
		})
	}
	return uniqueWrongIdentifierNamespace(
		action,
		parameter,
		identifier,
		exactIdentifierNamespaceArtifact,
		matches,
	)
}

func rejectNonFPFIdentifierAsSource(
	ctx context.Context,
	store *artifact.Store,
	action string,
	parameter string,
	identifier string,
	memoryResolver exactMemoryIdentifierResolver,
	fpfResolver exactFPFIdentifierResolver,
) error {
	if identifier == "" {
		return nil
	}
	if exact, _ := resolveExactFPFSourceIdentifier(ctx, fpfResolver, identifier); exact {
		return nil
	}

	matches := make([]exactIdentifierNamespaceMatch, 0, 3)
	if isExactCanonicalArtifactIdentifier(identifier) {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceArtifact,
			recovery:  artifactRecoveryCall(identifier),
		})
	}
	if route, exact := isExactIndexedCodeIdentifier(ctx, store, identifier); exact {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceCode,
			recovery:  codeRecoveryCall(identifier, route),
		})
	}
	if exactMemoryIdentifier(ctx, memoryResolver, identifier) {
		matches = append(matches, exactIdentifierNamespaceMatch{
			namespace: exactIdentifierNamespaceMemory,
			recovery:  memoryRecoveryCall(identifier),
		})
	}
	return uniqueWrongIdentifierNamespace(
		action,
		parameter,
		identifier,
		exactIdentifierNamespaceFPF,
		matches,
	)
}

func uniqueWrongIdentifierNamespace(
	action string,
	parameter string,
	identifier string,
	expected exactIdentifierNamespace,
	matches []exactIdentifierNamespaceMatch,
) error {
	if len(matches) != 1 {
		return nil
	}
	match := matches[0]
	return wrongIdentifierNamespaceError{
		payload: wrongIdentifierNamespace{
			Code:              wrongIdentifierNamespaceCode,
			Tool:              "haft_query",
			Action:            action,
			Parameter:         parameter,
			Identifier:        identifier,
			ReceivedNamespace: string(match.namespace),
			ExpectedNamespace: string(expected),
			SameCallRetryable: false,
			Message: fmt.Sprintf(
				"The unchanged identifier is proven in the %s namespace, not the %s namespace. Execute the exact read-only recovery_call; retrying the same action cannot succeed.",
				match.namespace,
				expected,
			),
			RecoveryCall: match.recovery,
		},
	}
}

func artifactRecoveryCall(identifier string) exactQueryRecoveryCall {
	return exactQueryRecoveryCall{
		Tool: "haft_query",
		Arguments: exactQueryRecoveryArguments{
			Action:      "related",
			ArtifactRef: identifier,
		},
	}
}

func fpfRecoveryCall(identifier string) exactQueryRecoveryCall {
	return exactQueryRecoveryCall{
		Tool: "haft_query",
		Arguments: exactQueryRecoveryArguments{
			Action:     "fpf",
			Mode:       "inspect",
			Identifier: identifier,
		},
	}
}

func codeRecoveryCall(
	identifier string,
	route exactCodeIdentifierRoute,
) exactQueryRecoveryCall {
	arguments := exactQueryRecoveryArguments{Action: "node"}
	if route == exactCodeIdentifierAnchor {
		arguments.AnchorID = identifier
	} else {
		arguments.Symbol = identifier
	}
	return exactQueryRecoveryCall{
		Tool:      "haft_query",
		Arguments: arguments,
	}
}

func memoryRecoveryCall(identifier string) exactQueryRecoveryCall {
	return exactQueryRecoveryCall{
		Tool:      "haft_query",
		Arguments: memoryRecoveryArguments(identifier),
	}
}

func memoryRecoveryArguments(identifier string) exactQueryRecoveryArguments {
	return exactQueryRecoveryArguments{
		Action: "memory",
		MemoryRequest: &exactQueryMemoryRecoveryRequest{
			ContractVersion: "haft.memory.v1",
			Mode:            "resolve",
			Basis: exactQueryRecoveryBasis{
				Kind: "project_current",
			},
			Query:         identifier,
			MaxCandidates: exactMemoryRecoveryMaxCandidates,
		},
	}
}

func exactMemoryIdentifier(
	ctx context.Context,
	resolver exactMemoryIdentifierResolver,
	identifier string,
) bool {
	if resolver == nil {
		return false
	}
	exact, err := resolver.ResolveExactMemoryIdentifier(ctx, identifier)
	return err == nil && exact
}

func exactMemoryResolutionResponse(
	identifier string,
	payload []byte,
) (bool, error) {
	var envelope struct {
		ResultKind string `json:"result_kind"`
		Result     struct {
			ResolutionWitnesses []struct {
				Kind    string `json:"kind"`
				Matched string `json:"matched"`
			} `json:"resolution_witnesses"`
			Candidates []struct {
				ResolutionWitnesses []struct {
					Kind    string `json:"kind"`
					Matched string `json:"matched"`
				} `json:"resolution_witnesses"`
			} `json:"candidates"`
			Issues []struct {
				Kind               string `json:"kind"`
				Alias              string `json:"alias"`
				HistoricalEntityID string `json:"historical_entity_id"`
				QueryContext       string `json:"query_context"`
			} `json:"issues"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return false, fmt.Errorf("decode exact memory namespace probe: %w", err)
	}
	switch envelope.ResultKind {
	case "exact_entity":
		for _, witness := range envelope.Result.ResolutionWitnesses {
			if exactMemoryWitness(identifier, witness.Kind, witness.Matched) {
				return true, nil
			}
		}
	case "entity_candidates":
		for _, candidate := range envelope.Result.Candidates {
			for _, witness := range candidate.ResolutionWitnesses {
				if exactMemoryWitness(identifier, witness.Kind, witness.Matched) {
					return true, nil
				}
			}
		}
	case "resolution_unsettled":
		for _, issue := range envelope.Result.Issues {
			switch issue.Kind {
			case "alias_conflict":
				if issue.Alias == identifier {
					return true, nil
				}
			case "reviewed_split_candidates":
				if issue.HistoricalEntityID == identifier {
					return true, nil
				}
			case "context_not_resolved":
				if issue.QueryContext == identifier {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func exactMemoryWitness(identifier string, kind string, matched string) bool {
	if matched != identifier {
		return false
	}
	switch kind {
	case "exact_identifier", "exact_alias", "reviewed_identity_merge":
		return true
	default:
		return false
	}
}

func isExactCanonicalArtifactIdentifier(identifier string) bool {
	return strings.TrimSpace(identifier) == identifier &&
		artifact.IsCanonicalArtifactID(identifier)
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

func resolveExactFPFSourceIdentifier(
	ctx context.Context,
	resolver exactFPFIdentifierResolver,
	identifier string,
) (bool, error) {
	if resolver != nil {
		return resolver.ResolveExactFPFIdentifier(ctx, identifier)
	}
	return isExactFPFSourceIdentifier(identifier)
}

func isExactIndexedCodeIdentifier(
	ctx context.Context,
	store *artifact.Store,
	identifier string,
) (exactCodeIdentifierRoute, bool) {
	if store == nil {
		return "", false
	}
	service := codeintel.NewService(store)
	state, err := service.CurrentIndexState(ctx)
	if err != nil || state.Epoch < 1 || state.Degraded {
		return "", false
	}

	var anchorCount int
	err = store.DB().QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM code_symbols
		 WHERE anchor_id = ?`,
		identifier,
	).Scan(&anchorCount)
	if err != nil {
		return "", false
	}
	if err := service.ConfirmIndexState(ctx, state); err != nil {
		return "", false
	}
	if anchorCount > 0 {
		return exactCodeIdentifierAnchor, true
	}

	bareName := identifier
	receiver := ""
	if separator := strings.LastIndex(identifier, "."); separator > 0 {
		receiver = identifier[:separator]
		bareName = identifier[separator+1:]
	}
	query := `SELECT COUNT(*) FROM code_symbols
		WHERE (name = ? OR qualified_name = ?)`
	arguments := []any{bareName, identifier}
	if receiver != "" {
		query += ` AND (receiver = ? OR qualified_name = ?)`
		arguments = append(arguments, receiver, identifier)
	}
	var symbolCount int
	if err := store.DB().QueryRowContext(
		ctx,
		query,
		arguments...,
	).Scan(&symbolCount); err != nil {
		return "", false
	}
	if err := service.ConfirmIndexState(ctx, state); err != nil {
		return "", false
	}
	return exactCodeIdentifierSymbol, symbolCount > 0
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
