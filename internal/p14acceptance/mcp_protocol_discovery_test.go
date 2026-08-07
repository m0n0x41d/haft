package p14acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const (
	p14MCPProtocolDiscoverySchema = "haft.p14.mcp-protocol-discovery/v1"
	p14MCPProtocolDiscoveryStatus = "observed_not_final"
	p14MCPProtocolTimeout         = 30 * time.Second
)

var p14ExpectedMCPToolOrder = []string{
	"haft_note",
	"haft_problem",
	"haft_solution",
	"haft_decision",
	"haft_refresh",
	"haft_query",
	"haft_method",
	"haft_commission",
	"haft_spec_section",
	"haft_onboard",
	"haft_entity",
	"haft_memory",
}

type p14MCPProtocolExchange struct {
	Method         string `json:"method"`
	RequestBase64  string `json:"request_base64"`
	RequestDigest  string `json:"request_digest"`
	ResponseBase64 string `json:"response_base64"`
	ResponseDigest string `json:"response_digest"`
}

type p14MCPProtocolDiscovery struct {
	Schema               string                   `json:"schema"`
	Status               string                   `json:"status"`
	RuntimeReceiptDigest string                   `json:"runtime_receipt_digest"`
	ExecutablePath       string                   `json:"executable_path"`
	ExecutableDigest     string                   `json:"executable_digest"`
	ProjectRoot          string                   `json:"project_root"`
	ProcessPID           int                      `json:"process_pid"`
	ProcessLaunchedAt    string                   `json:"process_launched_at"`
	CapturedAt           string                   `json:"captured_at"`
	Exchanges            []p14MCPProtocolExchange `json:"exchanges"`
	EvidenceDigest       string                   `json:"evidence_digest"`
}

type p14MCPJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func captureP14MCPProtocolDiscovery(
	parent context.Context,
	runtime p14RuntimeObservationBinding,
) (p14MCPProtocolDiscovery, error) {
	if err := validateP14RuntimeProtocolBasis(runtime); err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	requests, err := p14MCPProtocolRequests()
	if err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	input := bytes.Join(requests, []byte{'\n'})
	input = append(input, '\n')

	ctx, cancel := context.WithTimeout(parent, p14MCPProtocolTimeout)
	defer cancel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := exec.CommandContext(ctx, runtime.LiveMCPExecutablePath, "serve")
	command.Dir = runtime.LiveMCPProjectRoot
	command.Env = slices.Clone(os.Environ())
	command.Stdin = bytes.NewReader(input)
	command.Stdout = stdout
	command.Stderr = stderr
	launchedAt := time.Now().UTC()
	if err := command.Start(); err != nil {
		return p14MCPProtocolDiscovery{}, fmt.Errorf(
			"start P14 MCP protocol probe: %w",
			err,
		)
	}
	pid := command.Process.Pid
	waitErr := command.Wait()
	capturedAt := time.Now().UTC()
	if ctx.Err() != nil {
		return p14MCPProtocolDiscovery{}, fmt.Errorf(
			"P14 MCP protocol probe timed out: %w",
			ctx.Err(),
		)
	}
	if waitErr != nil {
		return p14MCPProtocolDiscovery{}, fmt.Errorf(
			"P14 MCP protocol probe failed: %w; stderr=%s",
			waitErr,
			boundedP14FPFText(stderr.Bytes()),
		)
	}
	if err := verifyP14FileDigest(
		runtime.LiveMCPExecutablePath,
		runtime.LiveMCPExecutableDigest,
	); err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	responses, err := splitP14MCPProtocolResponses(stdout.Bytes())
	if err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	exchanges := make([]p14MCPProtocolExchange, 0, len(requests))
	for index, request := range requests {
		method := "initialize"
		if index == 1 {
			method = "tools/list"
		}
		response := responses[index]
		exchanges = append(exchanges, p14MCPProtocolExchange{
			Method:         method,
			RequestBase64:  base64.StdEncoding.EncodeToString(request),
			RequestDigest:  p14Digest(request),
			ResponseBase64: base64.StdEncoding.EncodeToString(response),
			ResponseDigest: p14Digest(response),
		})
	}
	discovery := p14MCPProtocolDiscovery{
		Schema:               p14MCPProtocolDiscoverySchema,
		Status:               p14MCPProtocolDiscoveryStatus,
		RuntimeReceiptDigest: runtime.LiveMCPReceiptDigest,
		ExecutablePath:       runtime.LiveMCPExecutablePath,
		ExecutableDigest:     runtime.LiveMCPExecutableDigest,
		ProjectRoot:          runtime.LiveMCPProjectRoot,
		ProcessPID:           pid,
		ProcessLaunchedAt:    launchedAt.Format(time.RFC3339Nano),
		CapturedAt:           capturedAt.Format(time.RFC3339Nano),
		Exchanges:            exchanges,
	}
	digest, err := p14MCPProtocolDiscoveryDigest(discovery)
	if err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	discovery.EvidenceDigest = digest
	if err := validateP14MCPProtocolDiscovery(runtime, discovery); err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	return discovery, nil
}

func p14MCPProtocolRequests() ([][]byte, error) {
	inputs := []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      "p14-initialize",
			"method":  "initialize",
			"params": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{},
				"clientInfo": map[string]any{
					"name":    "haft-p14",
					"version": "v9",
				},
			},
		},
		{
			"jsonrpc": "2.0",
			"id":      "p14-tools-list",
			"method":  "tools/list",
			"params":  map[string]any{},
		},
	}
	result := make([][]byte, 0, len(inputs))
	for _, input := range inputs {
		raw, err := marshalP14CanonicalJSON(input)
		if err != nil {
			return nil, err
		}
		result = append(result, raw)
	}
	return result, nil
}

func splitP14MCPProtocolResponses(raw []byte) ([][]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	lines := bytes.Split(trimmed, []byte{'\n'})
	if len(lines) != 2 {
		return nil, fmt.Errorf(
			"P14 MCP protocol response count = %d, want 2",
			len(lines),
		)
	}
	result := make([][]byte, 0, len(lines))
	for _, line := range lines {
		response := bytes.TrimSpace(line)
		if !canonicalCompactJSON(response) {
			return nil, fmt.Errorf(
				"P14 MCP protocol response is not compact JSON",
			)
		}
		result = append(result, slices.Clone(response))
	}
	return result, nil
}

func p14MCPProtocolDiscoveryDigest(
	discovery p14MCPProtocolDiscovery,
) (string, error) {
	basis := discovery
	basis.EvidenceDigest = ""
	raw, err := marshalP14CanonicalJSON(basis)
	if err != nil {
		return "", err
	}
	return p14Digest(raw), nil
}

func validateP14MCPProtocolDiscovery(
	runtime p14RuntimeObservationBinding,
	discovery p14MCPProtocolDiscovery,
) error {
	if err := validateP14RuntimeProtocolBasis(runtime); err != nil {
		return err
	}
	if discovery.Schema != p14MCPProtocolDiscoverySchema ||
		discovery.Status != p14MCPProtocolDiscoveryStatus ||
		discovery.RuntimeReceiptDigest != runtime.LiveMCPReceiptDigest ||
		discovery.ExecutablePath != runtime.LiveMCPExecutablePath ||
		discovery.ExecutableDigest != runtime.LiveMCPExecutableDigest ||
		discovery.ProjectRoot != runtime.LiveMCPProjectRoot ||
		discovery.ProcessPID <= 0 ||
		len(discovery.Exchanges) != 2 {
		return fmt.Errorf("P14 MCP protocol discovery basis differs")
	}
	launchedAt, err := time.Parse(
		time.RFC3339Nano,
		discovery.ProcessLaunchedAt,
	)
	if err != nil {
		return fmt.Errorf("P14 MCP protocol launch time is invalid: %w", err)
	}
	capturedAt, err := time.Parse(time.RFC3339Nano, discovery.CapturedAt)
	if err != nil || !capturedAt.After(launchedAt) {
		return fmt.Errorf("P14 MCP protocol capture chronology differs")
	}
	liveFulfilledAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPFulfilledAt,
	)
	if err != nil || launchedAt.Before(liveFulfilledAt) {
		return fmt.Errorf(
			"P14 MCP protocol probe predates verified live MCP fulfillment",
		)
	}
	digest, err := p14MCPProtocolDiscoveryDigest(discovery)
	if err != nil {
		return err
	}
	if discovery.EvidenceDigest != digest {
		return fmt.Errorf("P14 MCP protocol discovery digest differs")
	}
	if err := verifyP14FileDigest(
		discovery.ExecutablePath,
		discovery.ExecutableDigest,
	); err != nil {
		return err
	}
	requests, err := p14MCPProtocolRequests()
	if err != nil {
		return err
	}
	for index, exchange := range discovery.Exchanges {
		method := "initialize"
		if index == 1 {
			method = "tools/list"
		}
		request, response, err := validateP14MCPProtocolExchange(
			exchange,
			method,
		)
		if err != nil {
			return err
		}
		if !bytes.Equal(request, requests[index]) {
			return fmt.Errorf(
				"P14 MCP %s request bytes differ",
				method,
			)
		}
		if method == "initialize" {
			if err := validateP14MCPInitializeResponse(response); err != nil {
				return err
			}
			continue
		}
		if err := validateP14MCPToolsListResponse(response); err != nil {
			return err
		}
	}
	return nil
}

func validateP14RuntimeProtocolBasis(
	runtime p14RuntimeObservationBinding,
) error {
	if !filepath.IsAbs(runtime.LiveMCPExecutablePath) ||
		!filepath.IsAbs(runtime.LiveMCPProjectRoot) ||
		!validP14Digest(runtime.LiveMCPExecutableDigest) ||
		!validP14Digest(runtime.LiveMCPReceiptDigest) {
		return fmt.Errorf("P14 MCP protocol runtime basis is invalid")
	}
	return nil
}

func validateP14MCPProtocolExchange(
	exchange p14MCPProtocolExchange,
	method string,
) ([]byte, []byte, error) {
	if exchange.Method != method ||
		!validP14Digest(exchange.RequestDigest) ||
		!validP14Digest(exchange.ResponseDigest) {
		return nil, nil, fmt.Errorf("P14 MCP %s exchange basis differs", method)
	}
	request, err := base64.StdEncoding.DecodeString(exchange.RequestBase64)
	if err != nil || p14Digest(request) != exchange.RequestDigest {
		return nil, nil, fmt.Errorf("P14 MCP %s request bytes differ", method)
	}
	response, err := base64.StdEncoding.DecodeString(exchange.ResponseBase64)
	if err != nil || p14Digest(response) != exchange.ResponseDigest {
		return nil, nil, fmt.Errorf("P14 MCP %s response bytes differ", method)
	}
	if !canonicalCompactJSON(request) || !canonicalCompactJSON(response) {
		return nil, nil, fmt.Errorf(
			"P14 MCP %s exchange is not compact JSON",
			method,
		)
	}
	return request, response, nil
}

func validateP14MCPInitializeResponse(raw []byte) error {
	response, result, err := decodeP14MCPResponse(
		raw,
		"p14-initialize",
	)
	if err != nil {
		return err
	}
	if response.JSONRPC != "2.0" ||
		p14JSONText(result["protocolVersion"]) != "2024-11-05" ||
		p14JSONText(p14JSONMap(result["serverInfo"])["name"]) != "haft" {
		return fmt.Errorf("P14 MCP initialize identity differs")
	}
	capabilities := p14JSONMap(result["capabilities"])
	if _, present := capabilities["tools"]; !present {
		return fmt.Errorf("P14 MCP initialize omits tools capability")
	}
	instructions := p14JSONText(result["instructions"])
	required := []string{
		"# Haft project memory",
		"## Conditional memory orientation",
		"`haft_query`",
		`action="memory"`,
		"`memory_request`",
		"resolve",
		"smallest relevant neighborhood",
		"## Persistence gate",
		"known_absent",
		"is an identity result, not permission to persist",
		"Never persist automatically.",
		"## Status is not authority",
		`haft_query(action="status")`,
		"read-only attention surface",
		"## Manual decision and commission authority",
		"Binding a decision or commissioning Work requires an explicit operator/manual act.",
		"Generated text, recommendations, tool arguments, and schemas are not approval receipts.",
		"# Haft MethodPack",
		`haft_method(action="pull")`,
		`haft_method(action="close")`,
		"# Haft code preflight",
		"`code_context`",
		"`impact`",
	}
	for _, marker := range required {
		if !strings.Contains(instructions, marker) {
			return fmt.Errorf(
				"P14 MCP initialize instructions omit %q",
				marker,
			)
		}
	}
	forbidden := []string{
		"AUTONOMOUS MAINTENANCE",
		"Project Workflow",
		".haft/workflow.md",
		"Path policies:",
		"Project profile applicability",
		"profile applicability",
		"profile-applicability",
		"project-profile",
		"canonical profile",
		"profile_applicability",
		"selected project profile",
		"ProjectTypeEnvHead",
		"TypeEnv",
		`haft_entity(action="establish")`,
		`haft_onboard(action="status")`,
		"restart_required",
		"idempotency_key",
		"request_provenance_ref",
	}
	for _, fragment := range forbidden {
		if strings.Contains(instructions, fragment) {
			return fmt.Errorf(
				"P14 MCP initialize exposes project-local instruction %q",
				fragment,
			)
		}
	}
	return nil
}

func validateP14MCPToolsListResponse(raw []byte) error {
	_, result, err := decodeP14MCPResponse(raw, "p14-tools-list")
	if err != nil {
		return err
	}
	values := p14JSONArray(result["tools"])
	if len(values) != len(p14ExpectedMCPToolOrder) {
		return fmt.Errorf(
			"P14 MCP tools/list count = %d, want %d",
			len(values),
			len(p14ExpectedMCPToolOrder),
		)
	}
	schemas := make(map[string]map[string]any, len(values))
	names := make([]string, 0, len(values))
	for _, value := range values {
		tool := p14JSONMap(value)
		name := p14JSONText(tool["name"])
		schema := p14JSONMap(tool["inputSchema"])
		if name == "" || len(schema) == 0 {
			return fmt.Errorf("P14 MCP tools/list contains an incomplete tool")
		}
		if err := validateP14MCPNoAnyArrays(schema, name); err != nil {
			return err
		}
		names = append(names, name)
		schemas[name] = schema
	}
	if !slices.Equal(names, p14ExpectedMCPToolOrder) {
		return fmt.Errorf("P14 MCP tools/list order differs: %v", names)
	}
	if err := validateP14MCPOnboardSchema(schemas["haft_onboard"]); err != nil {
		return err
	}
	if err := validateP14MCPEntitySchema(schemas["haft_entity"]); err != nil {
		return err
	}
	if err := validateP14MCPQueryMemorySchema(schemas["haft_query"]); err != nil {
		return err
	}
	return validateP14MCPRawMemorySchema(schemas["haft_memory"])
}

func decodeP14MCPResponse(
	raw []byte,
	expectedID string,
) (p14MCPJSONRPCResponse, map[string]any, error) {
	response := p14MCPJSONRPCResponse{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return p14MCPJSONRPCResponse{}, nil, fmt.Errorf(
			"decode P14 MCP response: %w",
			err,
		)
	}
	if response.ID != expectedID ||
		len(response.Error) != 0 ||
		len(response.Result) == 0 {
		return p14MCPJSONRPCResponse{}, nil, fmt.Errorf(
			"P14 MCP response %q differs",
			expectedID,
		)
	}
	result := map[string]any{}
	resultDecoder := json.NewDecoder(bytes.NewReader(response.Result))
	resultDecoder.UseNumber()
	if err := resultDecoder.Decode(&result); err != nil {
		return p14MCPJSONRPCResponse{}, nil, fmt.Errorf(
			"decode P14 MCP response result: %w",
			err,
		)
	}
	return response, result, nil
}

func validateP14MCPNoAnyArrays(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		if p14JSONText(typed["type"]) == "array" {
			items := p14JSONMap(typed["items"])
			if len(items) == 0 {
				return fmt.Errorf(
					"P14 MCP schema %s degrades an array to any[]",
					path,
				)
			}
		}
		for key, nested := range typed {
			child := path + "." + key
			if key == "oneOf" || key == "anyOf" {
				variants := p14JSONArray(nested)
				if len(variants) == 0 {
					return fmt.Errorf(
						"P14 MCP schema variants %s are empty",
						child,
					)
				}
				for index, variant := range variants {
					if len(p14JSONMap(variant)) == 0 {
						return fmt.Errorf(
							"P14 MCP schema variant %s[%d] is unconstrained",
							child,
							index,
						)
					}
				}
			}
			if err := validateP14MCPNoAnyArrays(nested, child); err != nil {
				return err
			}
		}
	case []any:
		for index, nested := range typed {
			child := fmt.Sprintf("%s[%d]", path, index)
			switch nested.(type) {
			case map[string]any, []any:
				if err := validateP14MCPNoAnyArrays(
					nested,
					child,
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateP14MCPOnboardSchema(schema map[string]any) error {
	if err := validateP14MCPClosedObject(
		schema,
		[]string{"action"},
		[]string{"action", "basis", "scopes"},
		"haft_onboard",
	); err != nil {
		return err
	}
	properties := p14JSONMap(schema["properties"])
	if !p14SchemaEnumEquals(
		p14JSONMap(properties["action"]),
		[]string{"memory_prepare", "profile_prepare", "status"},
	) {
		return fmt.Errorf("P14 haft_onboard action schema differs")
	}
	scopes := p14JSONMap(properties["scopes"])
	items := p14JSONMap(scopes["items"])
	if maximum, valid := p14JSONInt(scopes["maxItems"]); !valid || maximum != 32 {
		return fmt.Errorf("P14 haft_onboard scopes bound differs")
	}
	if err := validateP14MCPClosedObject(
		items,
		[]string{"evidence_paths", "label", "realization_kind", "scope_id"},
		[]string{"evidence_paths", "label", "realization_kind", "scope_id"},
		"haft_onboard.scopes.items",
	); err != nil {
		return err
	}
	scopeProperties := p14JSONMap(items["properties"])
	if !p14SchemaEnumEquals(
		p14JSONMap(scopeProperties["realization_kind"]),
		[]string{"non_software", "software"},
	) {
		return fmt.Errorf("P14 haft_onboard realization schema differs")
	}
	return nil
}

func validateP14MCPEntitySchema(schema map[string]any) error {
	required := []string{
		"action",
		"aliases",
		"bounded_context_ref",
		"entity_id",
		"idempotency_key",
		"label",
		"persistence_reason",
		"request_provenance_ref",
	}
	if err := validateP14MCPClosedObject(
		schema,
		required,
		required,
		"haft_entity",
	); err != nil {
		return err
	}
	properties := p14JSONMap(schema["properties"])
	if !p14SchemaLiteralEquals(
		p14JSONMap(properties["action"]),
		"establish",
	) || !p14SchemaEnumEquals(
		p14JSONMap(properties["persistence_reason"]),
		[]string{"explicit_operator_request", "named_receiving_use"},
	) {
		return fmt.Errorf("P14 haft_entity literal schema differs")
	}
	aliases := p14JSONMap(properties["aliases"])
	maximum, valid := p14JSONInt(aliases["maxItems"])
	if !valid || maximum != 63 || !p14JSONBool(aliases["uniqueItems"]) {
		return fmt.Errorf("P14 haft_entity alias schema differs")
	}
	return nil
}

func validateP14MCPQueryMemorySchema(schema map[string]any) error {
	if err := validateP14MCPClosedObject(
		schema,
		[]string{"action"},
		nil,
		"haft_query",
	); err != nil {
		return err
	}
	properties := p14JSONMap(schema["properties"])
	if !p14SchemaEnumContains(
		p14JSONMap(properties["action"]),
		[]string{"memory"},
	) {
		return fmt.Errorf("P14 haft_query memory schema differs")
	}
	envelope := p14JSONMap(properties["memory_request"])
	variants := p14JSONArray(envelope["oneOf"])
	if len(variants) != 3 {
		return fmt.Errorf(
			"P14 haft_query memory_request variants differ",
		)
	}
	expectedRequired := map[string][]string{
		"resolve": {
			"basis",
			"contract_version",
			"max_candidates",
			"mode",
			"query",
		},
		"neighborhood": {
			"basis",
			"bounded_context_ref",
			"contract_version",
			"entity_ref",
			"mode",
			"read_budget",
			"view",
		},
		"recall": {
			"basis",
			"bounded_context_ref",
			"candidate_budget",
			"contract_version",
			"entity_ref",
			"mode",
			"query",
			"read_budget",
			"view",
		},
	}
	seen := make(map[string]struct{}, len(variants))
	for _, rawVariant := range variants {
		variant := p14JSONMap(rawVariant)
		variantProperties := p14JSONMap(variant["properties"])
		mode := p14SchemaSingleLiteral(
			p14JSONMap(variantProperties["mode"]),
		)
		required := expectedRequired[mode]
		if required == nil {
			return fmt.Errorf(
				"P14 haft_query memory_request exposes unsupported mode",
			)
		}
		exactProperties := slices.Clone(required)
		if mode == "resolve" {
			exactProperties = append(
				exactProperties,
				"bounded_context_ref",
			)
		}
		if err := validateP14MCPClosedObject(
			variant,
			required,
			exactProperties,
			"haft_query.memory_request."+mode,
		); err != nil {
			return err
		}
		if !p14SchemaLiteralEquals(
			p14JSONMap(variantProperties["contract_version"]),
			"haft.memory.v1",
		) {
			return fmt.Errorf(
				"P14 haft_query memory contract version differs",
			)
		}
		if err := validateP14MCPMemoryReadBasisSchema(
			p14JSONMap(variantProperties["basis"]),
			mode,
		); err != nil {
			return err
		}
		if mode != "resolve" {
			if err := validateP14MCPMemoryEntityRefSchema(
				p14JSONMap(variantProperties["entity_ref"]),
				mode,
			); err != nil {
				return err
			}
			if err := validateP14MCPMemoryReadBudgetSchema(
				p14JSONMap(variantProperties["read_budget"]),
				mode,
			); err != nil {
				return err
			}
			if err := validateP14MCPMemoryViewSchema(
				p14JSONMap(variantProperties["view"]),
				mode,
			); err != nil {
				return err
			}
		}
		if mode == "recall" {
			candidateBudget := p14JSONMap(
				variantProperties["candidate_budget"],
			)
			if err := validateP14MCPClosedObject(
				candidateBudget,
				[]string{"max_candidates"},
				[]string{"max_candidates"},
				"haft_query.memory_request.recall.candidate_budget",
			); err != nil {
				return err
			}
			if err := validateP14MCPPositiveUint32Schema(
				p14JSONMap(
					p14JSONMap(candidateBudget["properties"])["max_candidates"],
				),
				"haft_query.memory_request.recall.candidate_budget.max_candidates",
			); err != nil {
				return err
			}
		}
		seen[mode] = struct{}{}
	}
	if len(seen) != len(expectedRequired) {
		return fmt.Errorf(
			"P14 haft_query memory_request modes are duplicated",
		)
	}
	return nil
}

func validateP14MCPMemoryReadBasisSchema(
	schema map[string]any,
	mode string,
) error {
	variants := p14JSONArray(schema["oneOf"])
	if len(variants) != 2 {
		return fmt.Errorf(
			"P14 haft_query %s basis variants differ",
			mode,
		)
	}
	expected := map[string][]string{
		"project_current": {"kind"},
		"exact_project": {
			"graph_revision",
			"kind",
			"type_env_digest",
		},
	}
	seen := make(map[string]struct{}, len(variants))
	for _, rawVariant := range variants {
		variant := p14JSONMap(rawVariant)
		properties := p14JSONMap(variant["properties"])
		kind := p14SchemaSingleLiteral(
			p14JSONMap(properties["kind"]),
		)
		required := expected[kind]
		if required == nil {
			return fmt.Errorf(
				"P14 haft_query %s basis kind differs",
				mode,
			)
		}
		if err := validateP14MCPClosedObject(
			variant,
			required,
			required,
			"haft_query.memory_request."+mode+".basis."+kind,
		); err != nil {
			return err
		}
		seen[kind] = struct{}{}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf(
			"P14 haft_query %s basis variants are duplicated",
			mode,
		)
	}
	return nil
}

func validateP14MCPMemoryEntityRefSchema(
	schema map[string]any,
	mode string,
) error {
	if err := validateP14MCPClosedObject(
		schema,
		[]string{"ref_kind_id", "reference_id"},
		[]string{"ref_kind_id", "reference_id"},
		"haft_query.memory_request."+mode+".entity_ref",
	); err != nil {
		return err
	}
	properties := p14JSONMap(schema["properties"])
	for _, field := range []string{"ref_kind_id", "reference_id"} {
		if err := validateP14MCPBoundedStringSchema(
			p14JSONMap(properties[field]),
			typedmemorywire.MaximumIdentifierBytes,
			"haft_query.memory_request."+mode+".entity_ref."+field,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateP14MCPMemoryReadBudgetSchema(
	schema map[string]any,
	mode string,
) error {
	fields := []string{
		"max_carrier_excerpt_characters",
		"max_facets",
		"max_items_per_facet",
		"max_provenance_depth",
		"max_relation_paths_per_item",
	}
	if err := validateP14MCPClosedObject(
		schema,
		fields,
		fields,
		"haft_query.memory_request."+mode+".read_budget",
	); err != nil {
		return err
	}
	properties := p14JSONMap(schema["properties"])
	for _, field := range fields {
		if err := validateP14MCPPositiveUint32Schema(
			p14JSONMap(properties[field]),
			"haft_query.memory_request."+mode+".read_budget."+field,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateP14MCPMemoryViewSchema(
	schema map[string]any,
	mode string,
) error {
	fields := []string{
		"detail",
		"include_history",
		"projection_profile_ref",
		"requested_facets",
	}
	if err := validateP14MCPClosedObject(
		schema,
		fields,
		fields,
		"haft_query.memory_request."+mode+".view",
	); err != nil {
		return err
	}
	properties := p14JSONMap(schema["properties"])
	if !p14SchemaEnumEquals(
		p14JSONMap(properties["projection_profile_ref"]),
		[]string{
			"agent_orientation.v1",
			"agent_orientation.v2",
			"decision_rationale.v1",
			"evidence_currentness.v1",
			"implementation_trace.v1",
			"spec_impact.v1",
		},
	) || !p14SchemaEnumEquals(
		p14JSONMap(properties["detail"]),
		[]string{"evidence", "overview", "standard"},
	) || p14JSONText(
		p14JSONMap(properties["include_history"])["type"],
	) != "boolean" {
		return fmt.Errorf(
			"P14 haft_query %s view scalar schemas differ",
			mode,
		)
	}
	facets := p14JSONMap(properties["requested_facets"])
	minimum, validMinimum := p14JSONInt(facets["minItems"])
	maximum, validMaximum := p14JSONInt(facets["maxItems"])
	if p14JSONText(facets["type"]) != "array" ||
		!validMinimum ||
		minimum != 1 ||
		!validMaximum ||
		maximum != typedmemorywire.MaximumArrayItems ||
		!p14SchemaEnumEquals(
			p14JSONMap(facets["items"]),
			[]string{
				"alternatives",
				"decisions",
				"epistemes",
				"evidence",
				"implementation",
				"problems",
				"specifications",
				"unresolved",
				"work",
			},
		) {
		return fmt.Errorf(
			"P14 haft_query %s requested-facet schema differs",
			mode,
		)
	}
	return nil
}

func validateP14MCPRawMemorySchema(schema map[string]any) error {
	if err := validateP14MCPClosedObject(
		schema,
		[]string{"request"},
		[]string{"request"},
		"haft_memory",
	); err != nil {
		return err
	}
	properties := p14JSONMap(schema["properties"])
	request := p14JSONMap(properties["request"])
	variants := p14JSONArray(request["oneOf"])
	if len(variants) != 2 {
		return fmt.Errorf("P14 haft_memory request variants differ")
	}
	expected := map[string][]string{
		"validate": {
			"action",
			"basis",
			"change_set",
			"contract_version",
		},
		"admit": {
			"action",
			"authority_class",
			"basis",
			"change_set",
			"contract_version",
			"idempotency_key",
			"request_provenance_ref",
		},
	}
	seen := make(map[string]struct{}, len(variants))
	for _, raw := range variants {
		variant := p14JSONMap(raw)
		properties := p14JSONMap(variant["properties"])
		actionSchema := p14JSONMap(properties["action"])
		action := p14SchemaSingleLiteral(actionSchema)
		required := expected[action]
		if required == nil {
			return fmt.Errorf("P14 haft_memory exposes an unsupported action")
		}
		if err := validateP14MCPClosedObject(
			variant,
			required,
			required,
			"haft_memory.request."+action,
		); err != nil {
			return err
		}
		if !p14SchemaLiteralEquals(
			p14JSONMap(properties["contract_version"]),
			"haft.memory.v2",
		) {
			return fmt.Errorf("P14 haft_memory contract version differs")
		}
		if err := validateP14MCPRawMemoryChangeSetSchema(
			p14JSONMap(properties["change_set"]),
			action,
		); err != nil {
			return err
		}
		seen[action] = struct{}{}
	}
	if len(seen) != 2 {
		return fmt.Errorf("P14 haft_memory action variants are duplicated")
	}
	return nil
}

func validateP14MCPRawMemoryChangeSetSchema(
	schema map[string]any,
	action string,
) error {
	if err := validateP14MCPClosedObject(
		schema,
		[]string{"changes"},
		[]string{"changes"},
		"haft_memory.request."+action+".change_set",
	); err != nil {
		return err
	}
	changes := p14JSONMap(p14JSONMap(schema["properties"])["changes"])
	minimum, validMinimum := p14JSONInt(changes["minItems"])
	maximum, validMaximum := p14JSONInt(changes["maxItems"])
	if p14JSONText(changes["type"]) != "array" ||
		!validMinimum ||
		minimum != 1 ||
		!validMaximum ||
		maximum != typedmemorywire.MaximumChanges {
		return fmt.Errorf(
			"P14 haft_memory %s change-set array schema differs",
			action,
		)
	}
	variants := p14JSONArray(p14JSONMap(changes["items"])["oneOf"])
	if len(variants) != 4 {
		return fmt.Errorf(
			"P14 haft_memory %s change variants differ",
			action,
		)
	}
	expectedKinds := []string{
		"assert_relation",
		"declare_entity",
		"identity_change",
		"retract_assertion",
	}
	actualKinds := make([]string, 0, len(variants))
	for _, raw := range variants {
		variant := p14JSONMap(raw)
		properties := p14JSONMap(variant["properties"])
		kind := p14SchemaSingleLiteral(p14JSONMap(properties["kind"]))
		if kind == "" {
			return fmt.Errorf(
				"P14 haft_memory %s change kind is open",
				action,
			)
		}
		actualKinds = append(actualKinds, kind)
		if kind != "identity_change" {
			continue
		}
		identityVariants := p14JSONArray(
			p14JSONMap(properties["change"])["oneOf"],
		)
		identityKinds := make([]string, 0, len(identityVariants))
		for _, rawIdentity := range identityVariants {
			identity := p14JSONMap(rawIdentity)
			identityProperties := p14JSONMap(identity["properties"])
			identityKinds = append(
				identityKinds,
				p14SchemaSingleLiteral(
					p14JSONMap(identityProperties["kind"]),
				),
			)
		}
		identityKinds = p14SortedUniqueStrings(identityKinds)
		if !slices.Equal(
			identityKinds,
			[]string{"admit_alias", "supersede_alias"},
		) {
			return fmt.Errorf(
				"P14 haft_memory %s identity variants expose merge/split or another unsupported kind",
				action,
			)
		}
	}
	actualKinds = p14SortedUniqueStrings(actualKinds)
	if !slices.Equal(actualKinds, expectedKinds) {
		return fmt.Errorf(
			"P14 haft_memory %s change kinds differ",
			action,
		)
	}
	return nil
}

func validateP14MCPBoundedStringSchema(
	schema map[string]any,
	maximum int,
	path string,
) error {
	minimum, validMinimum := p14JSONInt(schema["minLength"])
	actualMaximum, validMaximum := p14JSONInt(schema["maxLength"])
	if p14JSONText(schema["type"]) != "string" ||
		!validMinimum ||
		minimum != 1 ||
		!validMaximum ||
		actualMaximum != maximum {
		return fmt.Errorf("P14 MCP schema %s string bounds differ", path)
	}
	return nil
}

func validateP14MCPPositiveUint32Schema(
	schema map[string]any,
	path string,
) error {
	minimum, validMinimum := p14JSONInt(schema["minimum"])
	maximum, validMaximum := p14JSONInt(schema["maximum"])
	if p14JSONText(schema["type"]) != "integer" ||
		!validMinimum ||
		minimum != 1 ||
		!validMaximum ||
		uint64(maximum) != uint64(^uint32(0)) {
		return fmt.Errorf("P14 MCP schema %s integer bounds differ", path)
	}
	return nil
}

func validateP14MCPClosedObject(
	schema map[string]any,
	required []string,
	exactProperties []string,
	path string,
) error {
	if p14JSONText(schema["type"]) != "object" ||
		schema["additionalProperties"] != false {
		return fmt.Errorf("P14 MCP schema %s is not a closed object", path)
	}
	actualRequired := p14JSONStrings(schema["required"])
	expectedRequired := slices.Clone(required)
	slices.Sort(expectedRequired)
	if !slices.Equal(actualRequired, expectedRequired) {
		return fmt.Errorf("P14 MCP schema %s required fields differ", path)
	}
	if exactProperties == nil {
		return nil
	}
	properties := p14JSONMap(schema["properties"])
	actualProperties := make([]string, 0, len(properties))
	for key := range properties {
		actualProperties = append(actualProperties, key)
	}
	slices.Sort(actualProperties)
	expectedProperties := slices.Clone(exactProperties)
	slices.Sort(expectedProperties)
	if !slices.Equal(actualProperties, expectedProperties) {
		return fmt.Errorf("P14 MCP schema %s properties differ", path)
	}
	return nil
}

func p14SchemaLiteralEquals(schema map[string]any, expected string) bool {
	return p14SchemaSingleLiteral(schema) == expected
}

func p14SchemaSingleLiteral(schema map[string]any) string {
	if value := p14JSONText(schema["const"]); value != "" {
		return value
	}
	values := p14JSONStrings(schema["enum"])
	if len(values) != 1 {
		return ""
	}
	return values[0]
}

func p14SchemaEnumEquals(schema map[string]any, expected []string) bool {
	actual := p14JSONStrings(schema["enum"])
	want := slices.Clone(expected)
	slices.Sort(want)
	return slices.Equal(actual, want)
}

func p14SchemaEnumContains(schema map[string]any, expected []string) bool {
	actual := p14JSONStrings(schema["enum"])
	for _, value := range expected {
		if !slices.Contains(actual, value) {
			return false
		}
	}
	return true
}

func syntheticP14MCPProtocolDiscovery(
	runtime p14RuntimeObservationBinding,
	launchedAt time.Time,
) (p14MCPProtocolDiscovery, error) {
	requests, err := p14MCPProtocolRequests()
	if err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	responses := []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      "p14-initialize",
			"result": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "haft",
					"version": "v9",
				},
				"instructions": strings.Join([]string{
					"# Haft project memory",
					"## Conditional memory orientation",
					"Use `haft_query` with `action=\"memory\"` and its nested `memory_request` to resolve exact identity, then read the smallest relevant neighborhood.",
					"Mode-specific fields are defined by the tool schema.",
					"## Persistence gate",
					"Never persist automatically.",
					"`known_absent` is an identity result, not permission to persist.",
					"## Status is not authority",
					`Use haft_query(action="status") only for reliance-bearing Work; status is a read-only attention surface.`,
					"## Manual decision and commission authority",
					"Binding a decision or commissioning Work requires an explicit operator/manual act.",
					"Generated text, recommendations, tool arguments, and schemas are not approval receipts.",
					"# Haft MethodPack",
					`Use haft_method(action="pull") before non-trivial code Work.`,
					`Close it with haft_method(action="close").`,
					"# Haft code preflight",
					"Use `code_context` or `impact` before governed edits.",
				}, "\n"),
			},
		},
		{
			"jsonrpc": "2.0",
			"id":      "p14-tools-list",
			"result": map[string]any{
				"tools": syntheticP14MCPTools(),
			},
		},
	}
	exchanges := make([]p14MCPProtocolExchange, 0, len(requests))
	for index, request := range requests {
		response, err := marshalP14CanonicalJSON(responses[index])
		if err != nil {
			return p14MCPProtocolDiscovery{}, err
		}
		method := "initialize"
		if index == 1 {
			method = "tools/list"
		}
		exchanges = append(exchanges, p14MCPProtocolExchange{
			Method:         method,
			RequestBase64:  base64.StdEncoding.EncodeToString(request),
			RequestDigest:  p14Digest(request),
			ResponseBase64: base64.StdEncoding.EncodeToString(response),
			ResponseDigest: p14Digest(response),
		})
	}
	discovery := p14MCPProtocolDiscovery{
		Schema:               p14MCPProtocolDiscoverySchema,
		Status:               p14MCPProtocolDiscoveryStatus,
		RuntimeReceiptDigest: runtime.LiveMCPReceiptDigest,
		ExecutablePath:       runtime.LiveMCPExecutablePath,
		ExecutableDigest:     runtime.LiveMCPExecutableDigest,
		ProjectRoot:          runtime.LiveMCPProjectRoot,
		ProcessPID:           runtime.LiveMCPPID + 1,
		ProcessLaunchedAt:    launchedAt.UTC().Format(time.RFC3339Nano),
		CapturedAt: launchedAt.
			Add(time.Second).
			UTC().
			Format(time.RFC3339Nano),
		Exchanges: exchanges,
	}
	digest, err := p14MCPProtocolDiscoveryDigest(discovery)
	if err != nil {
		return p14MCPProtocolDiscovery{}, err
	}
	discovery.EvidenceDigest = digest
	return discovery, nil
}

func syntheticP14MCPTools() []any {
	tools := make([]any, 0, len(p14ExpectedMCPToolOrder))
	for _, name := range p14ExpectedMCPToolOrder {
		schema := p14MCPClosedSchema(map[string]any{}, nil)
		switch name {
		case "haft_onboard":
			schema = syntheticP14MCPOnboardSchema()
		case "haft_entity":
			schema = syntheticP14MCPEntitySchema()
		case "haft_query":
			schema = syntheticP14MCPQuerySchema()
		case "haft_memory":
			schema = syntheticP14MCPRawMemorySchema()
		}
		tools = append(tools, map[string]any{
			"name":        name,
			"description": name,
			"inputSchema": schema,
		})
	}
	return tools
}

func syntheticP14MCPOnboardSchema() map[string]any {
	scopeProperties := map[string]any{
		"scope_id":         map[string]any{"type": "string"},
		"label":            map[string]any{"type": "string"},
		"realization_kind": p14MCPEnumSchema("software", "non_software"),
		"evidence_paths": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	}
	scope := p14MCPClosedSchema(
		scopeProperties,
		[]string{
			"scope_id",
			"label",
			"realization_kind",
			"evidence_paths",
		},
	)
	properties := map[string]any{
		"action": p14MCPEnumSchema(
			"status",
			"profile_prepare",
			"memory_prepare",
		),
		"basis": map[string]any{"type": "object"},
		"scopes": map[string]any{
			"type":     "array",
			"maxItems": 32,
			"items":    scope,
		},
	}
	return p14MCPClosedSchema(properties, []string{"action"})
}

func syntheticP14MCPEntitySchema() map[string]any {
	properties := map[string]any{
		"action":              p14MCPEnumSchema("establish"),
		"entity_id":           map[string]any{"type": "string"},
		"label":               map[string]any{"type": "string"},
		"bounded_context_ref": map[string]any{"type": "string"},
		"aliases": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"maxItems":    63,
			"uniqueItems": true,
		},
		"persistence_reason": p14MCPEnumSchema(
			"explicit_operator_request",
			"named_receiving_use",
		),
		"request_provenance_ref": map[string]any{"type": "string"},
		"idempotency_key":        map[string]any{"type": "string"},
	}
	required := []string{
		"action",
		"entity_id",
		"label",
		"bounded_context_ref",
		"aliases",
		"persistence_reason",
		"request_provenance_ref",
		"idempotency_key",
	}
	return p14MCPClosedSchema(properties, required)
}

func syntheticP14MCPQuerySchema() map[string]any {
	boundedIdentifier := func() map[string]any {
		return map[string]any{
			"type":      "string",
			"minLength": 1,
			"maxLength": typedmemorywire.MaximumIdentifierBytes,
		}
	}
	positiveUint32 := func() map[string]any {
		return map[string]any{
			"type":    "integer",
			"minimum": 1,
			"maximum": uint64(^uint32(0)),
		}
	}
	projectCurrent := p14MCPClosedSchema(
		map[string]any{
			"kind": p14MCPEnumSchema("project_current"),
		},
		[]string{"kind"},
	)
	exactProject := p14MCPClosedSchema(
		map[string]any{
			"kind":            p14MCPEnumSchema("exact_project"),
			"type_env_digest": map[string]any{"type": "string"},
			"graph_revision":  map[string]any{"type": "integer"},
		},
		[]string{"kind", "type_env_digest", "graph_revision"},
	)
	readBudgetProperties := map[string]any{
		"max_facets":                     positiveUint32(),
		"max_items_per_facet":            positiveUint32(),
		"max_relation_paths_per_item":    positiveUint32(),
		"max_carrier_excerpt_characters": positiveUint32(),
		"max_provenance_depth":           positiveUint32(),
	}
	readBudgetRequired := []string{
		"max_facets",
		"max_items_per_facet",
		"max_relation_paths_per_item",
		"max_carrier_excerpt_characters",
		"max_provenance_depth",
	}
	basis := map[string]any{
		"oneOf": []any{projectCurrent, exactProject},
	}
	entityRef := p14MCPClosedSchema(
		map[string]any{
			"ref_kind_id":  boundedIdentifier(),
			"reference_id": boundedIdentifier(),
		},
		[]string{"ref_kind_id", "reference_id"},
	)
	view := p14MCPClosedSchema(
		map[string]any{
			"projection_profile_ref": p14MCPEnumSchema(
				"agent_orientation.v1",
				"agent_orientation.v2",
				"decision_rationale.v1",
				"spec_impact.v1",
				"evidence_currentness.v1",
				"implementation_trace.v1",
			),
			"requested_facets": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": typedmemorywire.MaximumArrayItems,
				"items": p14MCPEnumSchema(
					"epistemes",
					"problems",
					"alternatives",
					"decisions",
					"specifications",
					"evidence",
					"work",
					"implementation",
					"unresolved",
				),
			},
			"detail": p14MCPEnumSchema(
				"overview",
				"standard",
				"evidence",
			),
			"include_history": map[string]any{"type": "boolean"},
		},
		[]string{
			"projection_profile_ref",
			"requested_facets",
			"detail",
			"include_history",
		},
	)
	readBudget := p14MCPClosedSchema(
		readBudgetProperties,
		readBudgetRequired,
	)
	resolve := p14MCPClosedSchema(
		map[string]any{
			"mode":                p14MCPEnumSchema("resolve"),
			"contract_version":    p14MCPEnumSchema("haft.memory.v1"),
			"basis":               basis,
			"query":               map[string]any{"type": "string"},
			"bounded_context_ref": map[string]any{"type": "string"},
			"max_candidates":      map[string]any{"type": "integer"},
		},
		[]string{
			"mode",
			"contract_version",
			"basis",
			"query",
			"max_candidates",
		},
	)
	neighborhood := p14MCPClosedSchema(
		map[string]any{
			"mode":                p14MCPEnumSchema("neighborhood"),
			"contract_version":    p14MCPEnumSchema("haft.memory.v1"),
			"basis":               basis,
			"entity_ref":          entityRef,
			"bounded_context_ref": map[string]any{"type": "string"},
			"view":                view,
			"read_budget":         readBudget,
		},
		[]string{
			"mode",
			"contract_version",
			"basis",
			"entity_ref",
			"bounded_context_ref",
			"view",
			"read_budget",
		},
	)
	recall := p14MCPClosedSchema(
		map[string]any{
			"mode":                p14MCPEnumSchema("recall"),
			"contract_version":    p14MCPEnumSchema("haft.memory.v1"),
			"basis":               basis,
			"entity_ref":          entityRef,
			"bounded_context_ref": map[string]any{"type": "string"},
			"view":                view,
			"read_budget":         readBudget,
			"query":               map[string]any{"type": "string"},
			"candidate_budget": p14MCPClosedSchema(
				map[string]any{
					"max_candidates": positiveUint32(),
				},
				[]string{"max_candidates"},
			),
		},
		[]string{
			"mode",
			"contract_version",
			"basis",
			"entity_ref",
			"bounded_context_ref",
			"view",
			"read_budget",
			"query",
			"candidate_budget",
		},
	)
	properties := map[string]any{
		"action": p14MCPEnumSchema("status", "memory"),
		"memory_request": map[string]any{
			"oneOf": []any{resolve, neighborhood, recall},
		},
	}
	return p14MCPClosedSchema(properties, []string{"action"})
}

func syntheticP14MCPRawMemorySchema() map[string]any {
	changeSet := syntheticP14MCPRawMemoryChangeSetSchema()
	commonProperties := map[string]any{
		"contract_version": p14MCPEnumSchema("haft.memory.v2"),
		"basis": map[string]any{
			"type": "object",
		},
		"change_set": changeSet,
	}
	validateProperties := p14MCPCloneMap(commonProperties)
	validateProperties["action"] = p14MCPEnumSchema("validate")
	validateVariant := p14MCPClosedSchema(
		validateProperties,
		[]string{"contract_version", "action", "basis", "change_set"},
	)
	admitProperties := p14MCPCloneMap(commonProperties)
	admitProperties["action"] = p14MCPEnumSchema("admit")
	admitProperties["authority_class"] = p14MCPEnumSchema(
		"non_binding_semantic_assertion",
	)
	admitProperties["idempotency_key"] = map[string]any{"type": "string"}
	admitProperties["request_provenance_ref"] = map[string]any{"type": "string"}
	admitVariant := p14MCPClosedSchema(
		admitProperties,
		[]string{
			"contract_version",
			"action",
			"basis",
			"authority_class",
			"idempotency_key",
			"request_provenance_ref",
			"change_set",
		},
	)
	request := map[string]any{
		"oneOf": []any{validateVariant, admitVariant},
	}
	return p14MCPClosedSchema(
		map[string]any{"request": request},
		[]string{"request"},
	)
}

func syntheticP14MCPRawMemoryChangeSetSchema() map[string]any {
	declareEntity := p14MCPClosedSchema(
		map[string]any{
			"kind": p14MCPEnumSchema("declare_entity"),
		},
		[]string{"kind"},
	)
	identityChange := p14MCPClosedSchema(
		map[string]any{
			"kind": p14MCPEnumSchema("identity_change"),
			"change": map[string]any{
				"oneOf": []any{
					p14MCPClosedSchema(
						map[string]any{
							"kind": p14MCPEnumSchema("admit_alias"),
						},
						[]string{"kind"},
					),
					p14MCPClosedSchema(
						map[string]any{
							"kind": p14MCPEnumSchema("supersede_alias"),
						},
						[]string{"kind"},
					),
				},
			},
		},
		[]string{"kind", "change"},
	)
	assertRelation := p14MCPClosedSchema(
		map[string]any{
			"kind": p14MCPEnumSchema("assert_relation"),
		},
		[]string{"kind"},
	)
	retractAssertion := p14MCPClosedSchema(
		map[string]any{
			"kind": p14MCPEnumSchema("retract_assertion"),
		},
		[]string{"kind"},
	)
	return p14MCPClosedSchema(
		map[string]any{
			"changes": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": typedmemorywire.MaximumChanges,
				"items": map[string]any{
					"oneOf": []any{
						declareEntity,
						identityChange,
						assertRelation,
						retractAssertion,
					},
				},
			},
		},
		[]string{"changes"},
	)
}

func p14MCPClosedSchema(
	properties map[string]any,
	required []string,
) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             required,
	}
}

func p14MCPEnumSchema(values ...string) map[string]any {
	enums := make([]any, 0, len(values))
	for _, value := range values {
		enums = append(enums, value)
	}
	return map[string]any{
		"type": "string",
		"enum": enums,
	}
}

func p14MCPCloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func TestP14MCPProtocolDiscoveryRejectsAnyArrayAndFlatMemorySchema(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "haft")
	content := []byte("synthetic installed haft")
	if err := os.WriteFile(executable, content, 0o755); err != nil {
		t.Fatal(err)
	}
	runtime := p14RuntimeObservationBinding{
		LiveMCPPID:              41,
		LiveMCPFulfilledAt:      "2026-07-28T10:00:00Z",
		LiveMCPExecutablePath:   executable,
		LiveMCPExecutableDigest: p14Digest(content),
		LiveMCPProjectRoot:      root,
		LiveMCPReceiptDigest:    p14TestDigest("protocol-live-receipt"),
	}
	discovery, err := syntheticP14MCPProtocolDiscovery(
		runtime,
		time.Date(2026, 7, 28, 10, 1, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14MCPProtocolDiscovery(
		runtime,
		discovery,
	); err != nil {
		t.Fatal(err)
	}

	flatMemory := mutateP14MCPToolsResponseForTest(
		t,
		discovery,
		func(tools []any) {
			schema := p14MCPClosedSchema(
				map[string]any{
					"action": p14MCPEnumSchema("validate", "admit"),
				},
				[]string{"action"},
			)
			p14MCPToolForTest(t, tools, "haft_memory")["inputSchema"] = schema
		},
	)
	if err := validateP14MCPProtocolDiscovery(
		runtime,
		flatMemory,
	); err == nil {
		t.Fatal("P14 MCP discovery accepted the old flat haft_memory schema")
	}

	flatMemoryQuery := mutateP14MCPToolsResponseForTest(
		t,
		discovery,
		func(tools []any) {
			tool := p14MCPToolForTest(t, tools, "haft_query")
			schema := p14JSONMap(tool["inputSchema"])
			properties := p14JSONMap(schema["properties"])
			delete(properties, "memory_request")
			properties["contract_version"] =
				p14MCPEnumSchema("haft.memory.v1")
			properties["basis"] = map[string]any{"type": "object"}
		},
	)
	if err := validateP14MCPProtocolDiscovery(
		runtime,
		flatMemoryQuery,
	); err == nil {
		t.Fatal(
			"P14 MCP discovery accepted the legacy flat haft_query memory schema",
		)
	}

	anyArray := mutateP14MCPToolsResponseForTest(
		t,
		discovery,
		func(tools []any) {
			tool := p14MCPToolForTest(t, tools, "haft_entity")
			schema := p14JSONMap(tool["inputSchema"])
			properties := p14JSONMap(schema["properties"])
			aliases := p14JSONMap(properties["aliases"])
			delete(aliases, "items")
		},
	)
	if err := validateP14MCPProtocolDiscovery(
		runtime,
		anyArray,
	); err == nil {
		t.Fatal("P14 MCP discovery accepted an any[] alias schema")
	}

	mergeSplit := mutateP14MCPToolsResponseForTest(
		t,
		discovery,
		func(tools []any) {
			tool := p14MCPToolForTest(t, tools, "haft_memory")
			schema := p14JSONMap(tool["inputSchema"])
			request := p14JSONMap(
				p14JSONMap(schema["properties"])["request"],
			)
			variants := p14JSONArray(request["oneOf"])
			validateVariant := p14JSONMap(variants[0])
			changeSet := p14JSONMap(
				p14JSONMap(validateVariant["properties"])["change_set"],
			)
			changes := p14JSONMap(
				p14JSONMap(changeSet["properties"])["changes"],
			)
			items := p14JSONMap(changes["items"])
			changeVariants := p14JSONArray(items["oneOf"])
			items["oneOf"] = append(
				changeVariants,
				p14MCPClosedSchema(
					map[string]any{
						"kind": p14MCPEnumSchema("merge_entity"),
					},
					[]string{"kind"},
				),
			)
		},
	)
	if err := validateP14MCPProtocolDiscovery(
		runtime,
		mergeSplit,
	); err == nil {
		t.Fatal(
			"P14 MCP discovery accepted unsupported raw-memory merge/split",
		)
	}

	unboundedEntityRef := mutateP14MCPToolsResponseForTest(
		t,
		discovery,
		func(tools []any) {
			tool := p14MCPToolForTest(t, tools, "haft_query")
			schema := p14JSONMap(tool["inputSchema"])
			envelope := p14JSONMap(
				p14JSONMap(schema["properties"])["memory_request"],
			)
			variants := p14JSONArray(envelope["oneOf"])
			neighborhood := p14JSONMap(variants[1])
			entityRef := p14JSONMap(
				p14JSONMap(neighborhood["properties"])["entity_ref"],
			)
			referenceID := p14JSONMap(
				p14JSONMap(entityRef["properties"])["reference_id"],
			)
			delete(referenceID, "maxLength")
		},
	)
	if err := validateP14MCPProtocolDiscovery(
		runtime,
		unboundedEntityRef,
	); err == nil {
		t.Fatal(
			"P14 MCP discovery accepted an unbounded EntityRef field",
		)
	}
}

func mutateP14MCPToolsResponseForTest(
	t *testing.T,
	discovery p14MCPProtocolDiscovery,
	mutate func([]any),
) p14MCPProtocolDiscovery {
	t.Helper()
	cloned := discovery
	cloned.Exchanges = slices.Clone(discovery.Exchanges)
	exchange := cloned.Exchanges[1]
	raw, err := base64.StdEncoding.DecodeString(exchange.ResponseBase64)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	result := p14JSONMap(payload["result"])
	tools := p14JSONArray(result["tools"])
	mutate(tools)
	changed, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	exchange.ResponseBase64 = base64.StdEncoding.EncodeToString(changed)
	exchange.ResponseDigest = p14Digest(changed)
	cloned.Exchanges[1] = exchange
	cloned.EvidenceDigest = ""
	digest, err := p14MCPProtocolDiscoveryDigest(cloned)
	if err != nil {
		t.Fatal(err)
	}
	cloned.EvidenceDigest = digest
	return cloned
}

func p14MCPToolForTest(
	t *testing.T,
	tools []any,
	name string,
) map[string]any {
	t.Helper()
	for _, raw := range tools {
		tool := p14JSONMap(raw)
		if p14JSONText(tool["name"]) == name {
			return tool
		}
	}
	t.Fatalf("synthetic P14 MCP tools/list omits %q", name)
	return nil
}
