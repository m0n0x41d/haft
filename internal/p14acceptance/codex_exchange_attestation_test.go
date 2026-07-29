package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	p14CodexMCPExchangeIdentityBefore = "identity_before"
	p14CodexMCPExchangeBasisBefore    = "basis_before"
	p14CodexMCPExchangeTarget         = "target"
	p14CodexMCPExchangeBasisAfter     = "basis_after"
	p14CodexMCPExchangeIdentityAfter  = "identity_after"
)

var p14CodexMCPExchangeRoles = []string{
	p14CodexMCPExchangeIdentityBefore,
	p14CodexMCPExchangeBasisBefore,
	p14CodexMCPExchangeTarget,
	p14CodexMCPExchangeBasisAfter,
	p14CodexMCPExchangeIdentityAfter,
}

type p14CodexMCPDefinitionGroup struct {
	ID          string
	Definitions []p14CodexMCPCallDefinition
}

func buildP14CodexMCPExchangePlannedCalls(
	scenario preparedP14Scenario,
	request preparedP14Request,
	definitions []p14CodexMCPCallDefinition,
	firstSequence int,
) ([]p14CodexMCPPlannedCall, int, error) {
	groups, err := groupP14CodexMCPDefinitions(scenario.ID, definitions)
	if err != nil {
		return nil, 0, err
	}
	calls := make([]p14CodexMCPPlannedCall, 0, len(definitions)*5)
	sequence := firstSequence
	for groupIndex, group := range groups {
		beforeStatus := p14CodexMCPAttestationDefinition(
			fmt.Sprintf(
				"exchange_%03d_identity_before",
				groupIndex+1,
			),
			p14CodexMCPStatusAttestationArgs(),
		)
		beforeStatus.RequiredPredecessors = slices.Clone(
			group.Definitions[0].RequiredPredecessors,
		)
		call, err := p14CodexMCPPlannedExchangeCall(
			scenario,
			request,
			group.ID,
			p14CodexMCPExchangeIdentityBefore,
			beforeStatus,
			sequence,
		)
		if err != nil {
			return nil, 0, err
		}
		calls = append(calls, call)
		sequence++
		guardBasis := p14CodexMCPScenarioNeedsBasisGuard(scenario.ID)
		if guardBasis {
			beforeBasis := p14CodexMCPAttestationDefinition(
				fmt.Sprintf(
					"exchange_%03d_basis_before",
					groupIndex+1,
				),
				p14CodexMCPMemoryBasisProbeArgs(),
			)
			beforeBasis.RequiredPredecessors = slices.Clone(
				group.Definitions[0].RequiredPredecessors,
			)
			call, err = p14CodexMCPPlannedExchangeCall(
				scenario,
				request,
				group.ID,
				p14CodexMCPExchangeBasisBefore,
				beforeBasis,
				sequence,
			)
			if err != nil {
				return nil, 0, err
			}
			calls = append(calls, call)
			sequence++
		}
		for _, definition := range group.Definitions {
			call, err = p14CodexMCPPlannedExchangeCall(
				scenario,
				request,
				group.ID,
				p14CodexMCPExchangeTarget,
				definition,
				sequence,
			)
			if err != nil {
				return nil, 0, err
			}
			calls = append(calls, call)
			sequence++
		}
		if guardBasis {
			afterBasis := p14CodexMCPAttestationDefinition(
				fmt.Sprintf(
					"exchange_%03d_basis_after",
					groupIndex+1,
				),
				p14CodexMCPMemoryBasisProbeArgs(),
			)
			afterBasis.RequiredPredecessors = slices.Clone(
				group.Definitions[0].RequiredPredecessors,
			)
			call, err = p14CodexMCPPlannedExchangeCall(
				scenario,
				request,
				group.ID,
				p14CodexMCPExchangeBasisAfter,
				afterBasis,
				sequence,
			)
			if err != nil {
				return nil, 0, err
			}
			calls = append(calls, call)
			sequence++
		}
		afterStatus := p14CodexMCPAttestationDefinition(
			fmt.Sprintf(
				"exchange_%03d_identity_after",
				groupIndex+1,
			),
			p14CodexMCPStatusAttestationArgs(),
		)
		afterStatus.RequiredPredecessors = slices.Clone(
			group.Definitions[0].RequiredPredecessors,
		)
		call, err = p14CodexMCPPlannedExchangeCall(
			scenario,
			request,
			group.ID,
			p14CodexMCPExchangeIdentityAfter,
			afterStatus,
			sequence,
		)
		if err != nil {
			return nil, 0, err
		}
		calls = append(calls, call)
		sequence++
	}
	return calls, sequence, nil
}

func groupP14CodexMCPDefinitions(
	scenarioID string,
	definitions []p14CodexMCPCallDefinition,
) ([]p14CodexMCPDefinitionGroup, error) {
	if len(definitions) == 0 {
		return nil, fmt.Errorf(
			"P14 Codex MCP scenario %q has no target calls",
			scenarioID,
		)
	}
	result := make([]p14CodexMCPDefinitionGroup, 0, len(definitions))
	closedParallelGroups := make(map[string]struct{})
	for index := 0; index < len(definitions); {
		definition := definitions[index]
		if definition.ParallelGroup == "" {
			result = append(result, p14CodexMCPDefinitionGroup{
				ID: scenarioID + ":" + definition.CaseID,
				Definitions: []p14CodexMCPCallDefinition{
					definition,
				},
			})
			index++
			continue
		}
		if _, duplicate := closedParallelGroups[definition.ParallelGroup]; duplicate {
			return nil, fmt.Errorf(
				"P14 Codex MCP parallel group %q is not contiguous",
				definition.ParallelGroup,
			)
		}
		groupEnd := index + 1
		for groupEnd < len(definitions) &&
			definitions[groupEnd].ParallelGroup ==
				definition.ParallelGroup {
			groupEnd++
		}
		members := slices.Clone(definitions[index:groupEnd])
		if len(members) < 2 {
			return nil, fmt.Errorf(
				"P14 Codex MCP parallel group %q has fewer than two calls",
				definition.ParallelGroup,
			)
		}
		result = append(result, p14CodexMCPDefinitionGroup{
			ID:          scenarioID + ":parallel:" + definition.ParallelGroup,
			Definitions: members,
		})
		closedParallelGroups[definition.ParallelGroup] = struct{}{}
		index = groupEnd
	}
	return result, nil
}

func p14CodexMCPAttestationDefinition(
	caseID string,
	args map[string]any,
) p14CodexMCPCallDefinition {
	return p14CodexMCPCallDefinition{
		CaseID: caseID,
		Tool:   "haft_query",
		Args:   args,
	}
}

func p14CodexMCPStatusAttestationArgs() map[string]any {
	return map[string]any{
		"action": "status",
		"full":   false,
	}
}

func p14CodexMCPMemoryBasisProbeArgs() map[string]any {
	return map[string]any{
		"action": "memory",
		"memory_request": map[string]any{
			"mode":             "resolve",
			"contract_version": "haft.memory.v1",
			"basis": map[string]any{
				"kind": "project_current",
			},
			"query":          "Haft v9 typed project memory",
			"max_candidates": 5,
		},
	}
}

func p14CodexMCPScenarioNeedsBasisGuard(scenarioID string) bool {
	return slices.Contains(
		[]string{
			"invalid",
			"underdetermined",
			"authority_rejection",
		},
		scenarioID,
	)
}

func p14CodexMCPPlannedExchangeCall(
	scenario preparedP14Scenario,
	request preparedP14Request,
	exchangeID string,
	exchangeRole string,
	definition p14CodexMCPCallDefinition,
	sequence int,
) (p14CodexMCPPlannedCall, error) {
	args, err := marshalP14CanonicalJSON(definition.Args)
	if err != nil {
		return p14CodexMCPPlannedCall{}, err
	}
	var prompt *p14CodexMCPPlannedAgentPrompt
	if definition.AgentPrompt != nil {
		copied := *definition.AgentPrompt
		prompt = &copied
	}
	return p14CodexMCPPlannedCall{
		Sequence:             sequence,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CaseID:               definition.CaseID,
		ExchangeID:           exchangeID,
		ExchangeRole:         exchangeRole,
		ParallelGroup:        definition.ParallelGroup,
		RequiredPredecessors: slices.Clone(definition.RequiredPredecessors),
		Tool:                 definition.Tool,
		ArgsCanonical:        string(args),
		ArgsDigest:           p14Digest(args),
		RequestPayloadDigest: request.PayloadDigest,
		AgentPrompt:          prompt,
	}, nil
}

func p14CodexMCPTargetCalls(
	calls []p14CodexMCPCallEvidence,
) []p14CodexMCPCallEvidence {
	result := make([]p14CodexMCPCallEvidence, 0, len(calls))
	for _, call := range calls {
		if call.ExchangeRole == p14CodexMCPExchangeTarget {
			result = append(result, call)
		}
	}
	return result
}

func validateP14CodexMCPExchangePlan(
	calls []p14CodexMCPPlannedCall,
) error {
	for index := 0; index < len(calls); {
		exchangeID := calls[index].ExchangeID
		end := index + 1
		for end < len(calls) &&
			calls[end].ExchangeID == exchangeID {
			end++
		}
		exchange := calls[index:end]
		if err := validateP14CodexMCPPlannedExchange(exchange); err != nil {
			return err
		}
		index = end
	}
	return nil
}

func validateP14CodexMCPPlannedExchange(
	exchange []p14CodexMCPPlannedCall,
) error {
	if len(exchange) < 3 {
		return fmt.Errorf("P14 Codex MCP exchange is incomplete")
	}
	first := exchange[0]
	last := exchange[len(exchange)-1]
	if first.ExchangeRole != p14CodexMCPExchangeIdentityBefore ||
		last.ExchangeRole != p14CodexMCPExchangeIdentityAfter {
		return fmt.Errorf(
			"P14 Codex MCP exchange %q lacks identity brackets",
			first.ExchangeID,
		)
	}
	statusArgs, err := marshalP14CanonicalJSON(
		p14CodexMCPStatusAttestationArgs(),
	)
	if err != nil {
		return err
	}
	if first.Tool != "haft_query" ||
		last.Tool != "haft_query" ||
		first.ArgsCanonical != string(statusArgs) ||
		last.ArgsCanonical != string(statusArgs) {
		return fmt.Errorf(
			"P14 Codex MCP exchange %q status bracket differs",
			first.ExchangeID,
		)
	}
	targets := make([]p14CodexMCPPlannedCall, 0)
	basisBefore := 0
	basisAfter := 0
	for _, call := range exchange {
		if call.ExchangeID != first.ExchangeID ||
			call.ScenarioID != first.ScenarioID ||
			call.Builder != first.Builder ||
			call.RequestPayloadDigest != first.RequestPayloadDigest ||
			!slices.Equal(
				call.RequiredPredecessors,
				first.RequiredPredecessors,
			) {
			return fmt.Errorf(
				"P14 Codex MCP exchange %q changes basis",
				first.ExchangeID,
			)
		}
		switch call.ExchangeRole {
		case p14CodexMCPExchangeIdentityBefore,
			p14CodexMCPExchangeIdentityAfter:
			if call.ParallelGroup != "" || call.AgentPrompt != nil {
				return fmt.Errorf(
					"P14 Codex MCP identity bracket carries target fields",
				)
			}
		case p14CodexMCPExchangeBasisBefore:
			basisBefore++
		case p14CodexMCPExchangeBasisAfter:
			basisAfter++
		case p14CodexMCPExchangeTarget:
			targets = append(targets, call)
		default:
			return fmt.Errorf(
				"P14 Codex MCP exchange %q role is open",
				first.ExchangeID,
			)
		}
	}
	if len(targets) == 0 ||
		basisBefore != basisAfter ||
		basisBefore > 1 {
		return fmt.Errorf(
			"P14 Codex MCP exchange %q target or basis bracket differs",
			first.ExchangeID,
		)
	}
	if p14CodexMCPScenarioNeedsBasisGuard(first.ScenarioID) !=
		(basisBefore == 1) {
		return fmt.Errorf(
			"P14 Codex MCP exchange %q no-write basis guard differs",
			first.ExchangeID,
		)
	}
	if basisBefore == 1 {
		basisArgs, err := marshalP14CanonicalJSON(
			p14CodexMCPMemoryBasisProbeArgs(),
		)
		if err != nil {
			return err
		}
		for _, call := range exchange {
			if call.ExchangeRole != p14CodexMCPExchangeBasisBefore &&
				call.ExchangeRole != p14CodexMCPExchangeBasisAfter {
				continue
			}
			if call.Tool != "haft_query" ||
				call.ArgsCanonical != string(basisArgs) ||
				call.ParallelGroup != "" ||
				call.AgentPrompt != nil {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q basis probe differs",
					first.ExchangeID,
				)
			}
		}
	}
	if len(targets) == 1 && targets[0].ParallelGroup != "" {
		return fmt.Errorf(
			"P14 Codex MCP singleton exchange %q claims parallelism",
			first.ExchangeID,
		)
	}
	if len(targets) > 1 {
		group := targets[0].ParallelGroup
		if group == "" {
			return fmt.Errorf(
				"P14 Codex MCP exchange %q has ungrouped targets",
				first.ExchangeID,
			)
		}
		for _, target := range targets {
			if target.ParallelGroup != group {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q mixes parallel groups",
					first.ExchangeID,
				)
			}
		}
	}
	return nil
}

func deriveP14CodexMCPExchangeBindings(
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	callBindings []p14CodexSessionHistoryCallBinding,
	begins map[string]p14CodexSessionToolBegin,
	ends map[string]p14CodexSessionToolEvent,
) (
	[]p14CodexMCPExchangeBinding,
	map[string]string,
	error,
) {
	runtimeDigest, err := p14CodexMCPRuntimeIdentityDigest(
		packet.Packet.Runtime,
	)
	if err != nil {
		return nil, nil, err
	}
	evidenceBySequence := make(
		map[int]p14CodexMCPCallEvidence,
		len(input.Calls),
	)
	bindingBySequence := make(
		map[int]p14CodexSessionHistoryCallBinding,
		len(callBindings),
	)
	for _, evidence := range input.Calls {
		evidenceBySequence[evidence.Sequence] = evidence
	}
	for _, binding := range callBindings {
		bindingBySequence[binding.Sequence] = binding
	}
	result := make([]p14CodexMCPExchangeBinding, 0)
	digests := make(map[string]string)
	for index := 0; index < len(packet.Packet.Calls); {
		exchangeID := packet.Packet.Calls[index].ExchangeID
		end := index + 1
		for end < len(packet.Packet.Calls) &&
			packet.Packet.Calls[end].ExchangeID == exchangeID {
			end++
		}
		planned := packet.Packet.Calls[index:end]
		binding, err := deriveP14CodexMCPExchangeBinding(
			packet.Packet.Runtime,
			runtimeDigest,
			planned,
			evidenceBySequence,
			bindingBySequence,
			begins,
			ends,
		)
		if err != nil {
			return nil, nil, err
		}
		if _, duplicate := digests[binding.ExchangeID]; duplicate {
			return nil, nil, fmt.Errorf(
				"P14 Codex MCP exchange %q is duplicated",
				binding.ExchangeID,
			)
		}
		digests[binding.ExchangeID] = binding.EvidenceDigest
		result = append(result, binding)
		index = end
	}
	return result, digests, nil
}

func deriveP14CodexMCPExchangeBinding(
	runtime p14RuntimeObservationBinding,
	runtimeDigest string,
	planned []p14CodexMCPPlannedCall,
	evidenceBySequence map[int]p14CodexMCPCallEvidence,
	bindingBySequence map[int]p14CodexSessionHistoryCallBinding,
	begins map[string]p14CodexSessionToolBegin,
	ends map[string]p14CodexSessionToolEvent,
) (p14CodexMCPExchangeBinding, error) {
	if err := validateP14CodexMCPPlannedExchange(planned); err != nil {
		return p14CodexMCPExchangeBinding{}, err
	}
	first := planned[0]
	result := p14CodexMCPExchangeBinding{
		ExchangeID:            first.ExchangeID,
		ScenarioID:            first.ScenarioID,
		RuntimeIdentityDigest: runtimeDigest,
		TargetSequences:       make([]int, 0),
	}
	exchangeCalls := make([]p14CodexMCPExchangeCallDigestBasis, 0, len(planned))
	targetBegins := make([]int, 0)
	targetEnds := make([]int, 0)
	for _, call := range planned {
		evidence, present := evidenceBySequence[call.Sequence]
		if !present {
			return p14CodexMCPExchangeBinding{}, fmt.Errorf(
				"P14 Codex MCP exchange %q omits sequence %d",
				first.ExchangeID,
				call.Sequence,
			)
		}
		history, present := bindingBySequence[call.Sequence]
		if !present {
			return p14CodexMCPExchangeBinding{}, fmt.Errorf(
				"P14 Codex MCP exchange %q omits history sequence %d",
				first.ExchangeID,
				call.Sequence,
			)
		}
		begin, beginPresent := begins[evidence.Transcript.ToolCallID]
		end, endPresent := ends[evidence.Transcript.ToolCallID]
		if !beginPresent || !endPresent ||
			begin.Line != history.BeginLine ||
			end.Line != history.EndLine {
			return p14CodexMCPExchangeBinding{}, fmt.Errorf(
				"P14 Codex MCP exchange %q begin/end binding differs",
				first.ExchangeID,
			)
		}
		if begin.Line >= end.Line {
			return p14CodexMCPExchangeBinding{}, fmt.Errorf(
				"P14 Codex MCP exchange %q contains an interrupted call",
				first.ExchangeID,
			)
		}
		switch call.ExchangeRole {
		case p14CodexMCPExchangeIdentityBefore:
			result.IdentityBeforeSequence = call.Sequence
			if err := validateP14CodexMCPStatusAttestation(
				runtime,
				evidence,
			); err != nil {
				return p14CodexMCPExchangeBinding{}, err
			}
		case p14CodexMCPExchangeIdentityAfter:
			result.IdentityAfterSequence = call.Sequence
			if err := validateP14CodexMCPStatusAttestation(
				runtime,
				evidence,
			); err != nil {
				return p14CodexMCPExchangeBinding{}, err
			}
		case p14CodexMCPExchangeBasisBefore:
			result.BasisBeforeSequence = call.Sequence
			basis, err := p14CodexMCPMemoryBasisProofFromEvidence(
				evidence,
			)
			if err != nil {
				return p14CodexMCPExchangeBinding{}, err
			}
			result.BasisBefore = &basis
		case p14CodexMCPExchangeBasisAfter:
			result.BasisAfterSequence = call.Sequence
			basis, err := p14CodexMCPMemoryBasisProofFromEvidence(
				evidence,
			)
			if err != nil {
				return p14CodexMCPExchangeBinding{}, err
			}
			result.BasisAfter = &basis
		case p14CodexMCPExchangeTarget:
			result.TargetSequences = append(
				result.TargetSequences,
				call.Sequence,
			)
			result.ParallelGroup = call.ParallelGroup
			targetBegins = append(targetBegins, begin.Line)
			targetEnds = append(targetEnds, end.Line)
		default:
			return p14CodexMCPExchangeBinding{}, fmt.Errorf(
				"P14 Codex MCP exchange role is open",
			)
		}
		exchangeCalls = append(
			exchangeCalls,
			p14CodexMCPExchangeCallDigestBasis{
				Sequence:        call.Sequence,
				ExchangeRole:    call.ExchangeRole,
				ToolCallID:      evidence.Transcript.ToolCallID,
				ArgsDigest:      call.ArgsDigest,
				ResponseDigest:  evidence.Response.BodyDigest,
				BeginLine:       history.BeginLine,
				BeginLineDigest: history.BeginLineDigest,
				EndLine:         history.EndLine,
				EndLineDigest:   history.EndLineDigest,
			},
		)
	}
	if err := validateP14CodexMCPExchangeEventOrder(
		planned,
		bindingBySequence,
		targetBegins,
		targetEnds,
	); err != nil {
		return p14CodexMCPExchangeBinding{}, err
	}
	if (result.BasisBefore == nil) != (result.BasisAfter == nil) {
		return p14CodexMCPExchangeBinding{}, fmt.Errorf(
			"P14 Codex MCP exchange %q has a partial basis guard",
			first.ExchangeID,
		)
	}
	if result.BasisBefore != nil &&
		(result.BasisBefore.TypeEnvRef != result.BasisAfter.TypeEnvRef ||
			result.BasisBefore.TypeEnvDigest !=
				result.BasisAfter.TypeEnvDigest ||
			result.BasisBefore.GraphRevision !=
				result.BasisAfter.GraphRevision ||
			result.BasisBefore.ResponseDigest !=
				result.BasisAfter.ResponseDigest) {
		// This is equality of two independently executed semantic store-read
		// projections. It is not a raw filesystem or SQLite byte snapshot.
		return p14CodexMCPExchangeBinding{}, fmt.Errorf(
			"P14 Codex MCP no-write exchange %q changed its semantic store-read projection or graph CAS frontier",
			first.ExchangeID,
		)
	}
	digestBasis := p14CodexMCPExchangeDigestBasis{
		ExchangeID:            result.ExchangeID,
		ScenarioID:            result.ScenarioID,
		ParallelGroup:         result.ParallelGroup,
		RuntimeIdentityDigest: runtimeDigest,
		RuntimeReceiptDigest:  runtime.LiveMCPReceiptDigest,
		Calls:                 exchangeCalls,
		BasisBefore:           result.BasisBefore,
		BasisAfter:            result.BasisAfter,
	}
	raw, err := marshalP14CanonicalJSON(digestBasis)
	if err != nil {
		return p14CodexMCPExchangeBinding{}, err
	}
	result.EvidenceDigest = p14Digest(raw)
	return result, nil
}

type p14CodexMCPExchangeCallDigestBasis struct {
	Sequence        int    `json:"sequence"`
	ExchangeRole    string `json:"exchange_role"`
	ToolCallID      string `json:"tool_call_id"`
	ArgsDigest      string `json:"args_digest"`
	ResponseDigest  string `json:"response_digest"`
	BeginLine       int    `json:"begin_line"`
	BeginLineDigest string `json:"begin_line_digest"`
	EndLine         int    `json:"end_line"`
	EndLineDigest   string `json:"end_line_digest"`
}

type p14CodexMCPExchangeDigestBasis struct {
	ExchangeID            string                               `json:"exchange_id"`
	ScenarioID            string                               `json:"scenario_id"`
	ParallelGroup         string                               `json:"parallel_group,omitempty"`
	RuntimeIdentityDigest string                               `json:"runtime_identity_digest"`
	RuntimeReceiptDigest  string                               `json:"runtime_receipt_digest"`
	Calls                 []p14CodexMCPExchangeCallDigestBasis `json:"calls"`
	BasisBefore           *p14CodexMemoryBasisProof            `json:"basis_before,omitempty"`
	BasisAfter            *p14CodexMemoryBasisProof            `json:"basis_after,omitempty"`
}

func p14CodexMCPRuntimeIdentityDigest(
	runtime p14RuntimeObservationBinding,
) (string, error) {
	basis := struct {
		RestartID                 string `json:"restart_id"`
		ThreadID                  string `json:"thread_id"`
		RestartCheckpointDigest   string `json:"restart_checkpoint_digest"`
		LiveMCPReceiptDigest      string `json:"live_mcp_receipt_digest"`
		LiveMCPPID                int    `json:"live_mcp_pid"`
		LiveMCPStartedAt          string `json:"live_mcp_started_at"`
		LiveMCPExecutablePath     string `json:"live_mcp_executable_path"`
		LiveMCPExecutableDigest   string `json:"live_mcp_executable_digest"`
		LiveMCPProjectRoot        string `json:"live_mcp_project_root"`
		PreparedTaskRuntimePID    int    `json:"prepared_task_runtime_pid"`
		PreparedTaskRuntimeStart  string `json:"prepared_task_runtime_started_at"`
		PreparedTaskRuntimeBinary string `json:"prepared_task_runtime_executable"`
		PreparedTaskRuntimeArgs   string `json:"prepared_task_runtime_args_digest"`
		CodexSessionRoot          string `json:"codex_session_root"`
	}{
		RestartID:                 runtime.RestartID,
		ThreadID:                  runtime.ThreadID,
		RestartCheckpointDigest:   runtime.RestartCheckpointDigest,
		LiveMCPReceiptDigest:      runtime.LiveMCPReceiptDigest,
		LiveMCPPID:                runtime.LiveMCPPID,
		LiveMCPStartedAt:          runtime.LiveMCPStartedAt,
		LiveMCPExecutablePath:     runtime.LiveMCPExecutablePath,
		LiveMCPExecutableDigest:   runtime.LiveMCPExecutableDigest,
		LiveMCPProjectRoot:        runtime.LiveMCPProjectRoot,
		PreparedTaskRuntimePID:    runtime.PreparedTaskRuntimePID,
		PreparedTaskRuntimeStart:  runtime.PreparedTaskRuntimeStartedAt,
		PreparedTaskRuntimeBinary: runtime.PreparedTaskRuntimeExecutable,
		PreparedTaskRuntimeArgs:   runtime.PreparedTaskRuntimeArgsDigest,
		CodexSessionRoot:          runtime.CodexSessionRoot,
	}
	raw, err := marshalP14CanonicalJSON(basis)
	if err != nil {
		return "", err
	}
	return p14Digest(raw), nil
}

func validateP14CodexMCPStatusAttestation(
	runtime p14RuntimeObservationBinding,
	evidence p14CodexMCPCallEvidence,
) error {
	body, err := p14CodexMCPResponseBody(evidence)
	if err != nil {
		return err
	}
	if evidence.Transcript.Tool != "haft_query" ||
		evidence.Transcript.Status != "completed" ||
		evidence.Response.IsError ||
		len(bytes.TrimSpace(body)) == 0 {
		return fmt.Errorf(
			"P14 Codex MCP same-server status challenge failed",
		)
	}
	observed, err := parseP14CodexMCPRuntimeStatusLine(body)
	if err != nil {
		return err
	}
	expectedStartedAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPStartedAt,
	)
	if err != nil {
		return err
	}
	if observed.PID != runtime.LiveMCPPID ||
		!observed.StartedAt.Equal(
			expectedStartedAt.UTC().Truncate(time.Second),
		) ||
		!p14CodexSamePhysicalExecutable(
			observed.ExecutablePath,
			runtime.LiveMCPExecutablePath,
		) {
		return fmt.Errorf(
			"P14 Codex MCP status runtime identity differs from the verified live MCP receipt",
		)
	}
	return nil
}

type p14CodexMCPRuntimeStatusIdentity struct {
	PID            int
	StartedAt      time.Time
	ExecutablePath string
}

func parseP14CodexMCPRuntimeStatusLine(
	body []byte,
) (p14CodexMCPRuntimeStatusIdentity, error) {
	const section = "### Runtime\n\n"
	if bytes.Count(body, []byte(section)) != 1 {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status has no unique Runtime section",
		)
	}
	after, present := strings.CutPrefix(
		string(body[bytes.Index(body, []byte(section))+len(section):]),
		"- `haft serve`: pid=",
	)
	if !present {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status Runtime line differs",
		)
	}
	line, _, _ := strings.Cut(after, "\n")
	pidText, rest, present := strings.Cut(line, " started=")
	if !present {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status Runtime pid differs",
		)
	}
	startedText, rest, present := strings.Cut(rest, " executable=`")
	if !present {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status Runtime start differs",
		)
	}
	executable, mtime, present := strings.Cut(
		rest,
		"` executable_mtime=",
	)
	if !present ||
		strings.TrimSpace(mtime) == "" ||
		strings.Contains(mtime, " ") {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status Runtime executable differs",
		)
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status Runtime pid is invalid",
		)
	}
	startedAt, err := time.Parse(time.RFC3339, startedText)
	if err != nil {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status Runtime start is invalid",
		)
	}
	if !filepath.IsAbs(executable) {
		return p14CodexMCPRuntimeStatusIdentity{}, fmt.Errorf(
			"P14 Codex MCP status Runtime executable is not absolute",
		)
	}
	return p14CodexMCPRuntimeStatusIdentity{
		PID:            pid,
		StartedAt:      startedAt.UTC(),
		ExecutablePath: filepath.Clean(executable),
	}, nil
}

func p14CodexSamePhysicalExecutable(
	left string,
	right string,
) bool {
	cleanLeft := filepath.Clean(left)
	cleanRight := filepath.Clean(right)
	if cleanLeft == cleanRight {
		return true
	}
	physicalLeft, leftErr := filepath.EvalSymlinks(cleanLeft)
	physicalRight, rightErr := filepath.EvalSymlinks(cleanRight)
	return leftErr == nil &&
		rightErr == nil &&
		filepath.Clean(physicalLeft) == filepath.Clean(physicalRight)
}

func p14CodexMCPMemoryBasisProofFromEvidence(
	evidence p14CodexMCPCallEvidence,
) (p14CodexMemoryBasisProof, error) {
	body, err := p14CodexMCPResponseBody(evidence)
	if err != nil {
		return p14CodexMemoryBasisProof{}, err
	}
	if evidence.Transcript.Status != "completed" ||
		evidence.Response.IsError {
		return p14CodexMemoryBasisProof{}, fmt.Errorf(
			"P14 Codex MCP basis guard failed",
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	payload := map[string]any{}
	if err := decoder.Decode(&payload); err != nil {
		return p14CodexMemoryBasisProof{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return p14CodexMemoryBasisProof{}, fmt.Errorf(
			"P14 Codex MCP basis response has trailing JSON",
		)
	}
	result := p14JSONMap(payload["result"])
	snapshot := p14JSONMap(result["snapshot_basis"])
	typeEnvRef := p14JSONText(snapshot["type_env_ref"])
	typeEnvDigest := p14JSONText(snapshot["type_env_digest"])
	graphRevision, graphOK := p14JSONInt64(
		snapshot["graph_revision"],
	)
	if typeEnvRef == "" ||
		!validP14Digest(typeEnvDigest) ||
		!graphOK ||
		graphRevision < 0 {
		return p14CodexMemoryBasisProof{}, fmt.Errorf(
			"P14 Codex MCP basis response omits snapshot_basis",
		)
	}
	return p14CodexMemoryBasisProof{
		TypeEnvRef:     typeEnvRef,
		TypeEnvDigest:  typeEnvDigest,
		GraphRevision:  graphRevision,
		ResponseDigest: evidence.Response.BodyDigest,
	}, nil
}

func validateP14CodexMCPExchangeEventOrder(
	planned []p14CodexMCPPlannedCall,
	bindings map[int]p14CodexSessionHistoryCallBinding,
	targetBegins []int,
	targetEnds []int,
) error {
	if len(planned) == 0 ||
		len(targetBegins) == 0 ||
		len(targetBegins) != len(targetEnds) {
		return fmt.Errorf("P14 Codex MCP exchange event order is incomplete")
	}
	previousEnd := 0
	for _, call := range planned {
		if call.ExchangeRole == p14CodexMCPExchangeTarget &&
			call.ParallelGroup != "" {
			continue
		}
		binding := bindings[call.Sequence]
		if binding.BeginLine <= previousEnd ||
			binding.EndLine <= binding.BeginLine {
			return fmt.Errorf(
				"P14 Codex MCP exchange contains intervening or reordered Haft events",
			)
		}
		previousEnd = binding.EndLine
	}
	if len(targetBegins) == 1 {
		return nil
	}
	maxBegin := slices.Max(targetBegins)
	minEnd := slices.Min(targetEnds)
	if maxBegin >= minEnd {
		return fmt.Errorf(
			"P14 Codex MCP parallel exchange did not overlap",
		)
	}
	beforeEnd := bindings[planned[0].Sequence].EndLine
	afterBegin := bindings[planned[len(planned)-1].Sequence].BeginLine
	if slices.Min(targetBegins) <= beforeEnd ||
		slices.Max(targetEnds) >= afterBegin {
		return fmt.Errorf(
			"P14 Codex MCP parallel exchange escaped its status bracket",
		)
	}
	return nil
}

func p14CodexMCPExchangeCount(
	calls []p14CodexMCPPlannedCall,
) int {
	count := 0
	previous := ""
	for _, call := range calls {
		if call.ExchangeID == previous {
			continue
		}
		count++
		previous = call.ExchangeID
	}
	return count
}

func validateP14CodexMCPExchangeEvidenceShape(
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	callBindings []p14CodexSessionHistoryCallBinding,
	exchanges []p14CodexMCPExchangeBinding,
) error {
	runtimeDigest, err := p14CodexMCPRuntimeIdentityDigest(
		packet.Packet.Runtime,
	)
	if err != nil {
		return err
	}
	evidenceBySequence := make(
		map[int]p14CodexMCPCallEvidence,
		len(input.Calls),
	)
	historyBySequence := make(
		map[int]p14CodexSessionHistoryCallBinding,
		len(callBindings),
	)
	for _, evidence := range input.Calls {
		evidenceBySequence[evidence.Sequence] = evidence
	}
	for _, binding := range callBindings {
		historyBySequence[binding.Sequence] = binding
	}
	exchangeIndex := 0
	for index := 0; index < len(packet.Packet.Calls); {
		exchangeID := packet.Packet.Calls[index].ExchangeID
		end := index + 1
		for end < len(packet.Packet.Calls) &&
			packet.Packet.Calls[end].ExchangeID == exchangeID {
			end++
		}
		if exchangeIndex >= len(exchanges) {
			return fmt.Errorf(
				"P14 Codex MCP exchange evidence is incomplete",
			)
		}
		planned := packet.Packet.Calls[index:end]
		exchange := exchanges[exchangeIndex]
		if err := validateP14CodexMCPExchangeBindingShape(
			packet.Packet.Runtime,
			runtimeDigest,
			planned,
			evidenceBySequence,
			historyBySequence,
			exchange,
		); err != nil {
			return err
		}
		exchangeIndex++
		index = end
	}
	if exchangeIndex != len(exchanges) {
		return fmt.Errorf(
			"P14 Codex MCP exchange evidence contains extra brackets",
		)
	}
	return nil
}

func validateP14CodexMCPExchangeBindingShape(
	runtime p14RuntimeObservationBinding,
	runtimeDigest string,
	planned []p14CodexMCPPlannedCall,
	evidenceBySequence map[int]p14CodexMCPCallEvidence,
	historyBySequence map[int]p14CodexSessionHistoryCallBinding,
	exchange p14CodexMCPExchangeBinding,
) error {
	first := planned[0]
	if exchange.ExchangeID != first.ExchangeID ||
		exchange.ScenarioID != first.ScenarioID ||
		exchange.RuntimeIdentityDigest != runtimeDigest ||
		!validP14Digest(exchange.EvidenceDigest) {
		return fmt.Errorf(
			"P14 Codex MCP exchange %q evidence basis differs",
			first.ExchangeID,
		)
	}
	targetSequences := make([]int, 0)
	callDigestBasis := make(
		[]p14CodexMCPExchangeCallDigestBasis,
		0,
		len(planned),
	)
	for _, call := range planned {
		evidence, evidencePresent := evidenceBySequence[call.Sequence]
		history, historyPresent := historyBySequence[call.Sequence]
		if !evidencePresent || !historyPresent ||
			history.ExchangeEvidenceDigest != exchange.EvidenceDigest {
			return fmt.Errorf(
				"P14 Codex MCP exchange %q call binding differs",
				first.ExchangeID,
			)
		}
		switch call.ExchangeRole {
		case p14CodexMCPExchangeIdentityBefore:
			if exchange.IdentityBeforeSequence != call.Sequence {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q before status differs",
					first.ExchangeID,
				)
			}
			if err := validateP14CodexMCPStatusAttestation(
				runtime,
				evidence,
			); err != nil {
				return err
			}
		case p14CodexMCPExchangeIdentityAfter:
			if exchange.IdentityAfterSequence != call.Sequence {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q after status differs",
					first.ExchangeID,
				)
			}
			if err := validateP14CodexMCPStatusAttestation(
				runtime,
				evidence,
			); err != nil {
				return err
			}
		case p14CodexMCPExchangeBasisBefore:
			if exchange.BasisBeforeSequence != call.Sequence {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q before basis differs",
					first.ExchangeID,
				)
			}
			basis, err := p14CodexMCPMemoryBasisProofFromEvidence(
				evidence,
			)
			if err != nil ||
				exchange.BasisBefore == nil ||
				basis != *exchange.BasisBefore {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q before basis proof differs",
					first.ExchangeID,
				)
			}
		case p14CodexMCPExchangeBasisAfter:
			if exchange.BasisAfterSequence != call.Sequence {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q after basis differs",
					first.ExchangeID,
				)
			}
			basis, err := p14CodexMCPMemoryBasisProofFromEvidence(
				evidence,
			)
			if err != nil ||
				exchange.BasisAfter == nil ||
				basis != *exchange.BasisAfter {
				return fmt.Errorf(
					"P14 Codex MCP exchange %q after basis proof differs",
					first.ExchangeID,
				)
			}
		case p14CodexMCPExchangeTarget:
			targetSequences = append(targetSequences, call.Sequence)
		}
		callDigestBasis = append(
			callDigestBasis,
			p14CodexMCPExchangeCallDigestBasis{
				Sequence:        call.Sequence,
				ExchangeRole:    call.ExchangeRole,
				ToolCallID:      evidence.Transcript.ToolCallID,
				ArgsDigest:      call.ArgsDigest,
				ResponseDigest:  evidence.Response.BodyDigest,
				BeginLine:       history.BeginLine,
				BeginLineDigest: history.BeginLineDigest,
				EndLine:         history.EndLine,
				EndLineDigest:   history.EndLineDigest,
			},
		)
	}
	if !slices.Equal(exchange.TargetSequences, targetSequences) ||
		(exchange.BasisBefore == nil) != (exchange.BasisAfter == nil) {
		return fmt.Errorf(
			"P14 Codex MCP exchange %q target or basis evidence differs",
			first.ExchangeID,
		)
	}
	if exchange.BasisBefore != nil &&
		(exchange.BasisBefore.TypeEnvRef !=
			exchange.BasisAfter.TypeEnvRef ||
			exchange.BasisBefore.TypeEnvDigest !=
				exchange.BasisAfter.TypeEnvDigest ||
			exchange.BasisBefore.GraphRevision !=
				exchange.BasisAfter.GraphRevision ||
			exchange.BasisBefore.ResponseDigest !=
				exchange.BasisAfter.ResponseDigest) {
		return fmt.Errorf(
			"P14 Codex MCP no-write exchange %q changed its semantic store-read projection or graph CAS frontier",
			first.ExchangeID,
		)
	}
	digestBasis := p14CodexMCPExchangeDigestBasis{
		ExchangeID:            exchange.ExchangeID,
		ScenarioID:            exchange.ScenarioID,
		ParallelGroup:         exchange.ParallelGroup,
		RuntimeIdentityDigest: runtimeDigest,
		RuntimeReceiptDigest:  runtime.LiveMCPReceiptDigest,
		Calls:                 callDigestBasis,
		BasisBefore:           exchange.BasisBefore,
		BasisAfter:            exchange.BasisAfter,
	}
	raw, err := marshalP14CanonicalJSON(digestBasis)
	if err != nil {
		return err
	}
	if p14Digest(raw) != exchange.EvidenceDigest {
		return fmt.Errorf(
			"P14 Codex MCP exchange %q digest differs",
			first.ExchangeID,
		)
	}
	return nil
}
