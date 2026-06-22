package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	methodpkg "github.com/m0n0x41d/haft/internal/method"
)

func handleHaftMethod(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, string, error) {
	action, _ := args["action"].(string)
	switch strings.TrimSpace(action) {
	case "pull":
		return handleHaftMethodPull(ctx, store, haftDir, args)
	case "close":
		result, err := handleHaftMethodClose(ctx, store, haftDir, args)
		return result, "", err
	case "show":
		result, err := handleHaftMethodShow(ctx, store, args)
		return result, "", err
	case "status":
		result, err := handleHaftMethodStatus(ctx, store, args)
		return result, "", err
	case "detail":
		result, err := handleHaftMethodDetail(args)
		return result, "", err
	default:
		return "", "", fmt.Errorf("haft_method action must be pull, close, show, status, or detail")
	}
}

func handleHaftMethodPull(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, string, error) {
	input, err := parseMethodPullInput(args)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(input.Task) == "" {
		return "", "", fmt.Errorf("task is required for method pull")
	}

	a, filePath, run, err := methodpkg.CreateRun(ctx, store, haftDir, input)
	if err != nil {
		return "", "", err
	}
	return renderMethodPullResponse(run, filePath), a.Meta.ID, nil
}

func handleHaftMethodClose(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
	input, err := parseMethodCloseInput(args)
	if err != nil {
		return "", err
	}
	_, filePath, run, err := methodpkg.CloseRun(ctx, store, haftDir, input)
	if err != nil {
		return "", err
	}
	return renderMethodCloseResponse(run, filePath), nil
}

func handleHaftMethodShow(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	pullID, _ := args["pull_id"].(string)
	if strings.TrimSpace(pullID) == "" {
		return "", fmt.Errorf("pull_id is required for method show")
	}
	a, err := store.Get(ctx, pullID)
	if err != nil {
		return "", err
	}
	if a.Meta.Kind != artifact.KindMethodRun {
		return "", fmt.Errorf("%s is %s, not MethodRun", pullID, a.Meta.Kind)
	}
	run, err := methodpkg.DecodeRun(a)
	if err != nil {
		return "", err
	}
	return renderMethodRunSummary(run), nil
}

func handleHaftMethodStatus(ctx context.Context, store *artifact.Store, args map[string]any) (string, error) {
	limit := intArg(args, "limit", 10)
	runs, err := methodpkg.OpenRuns(ctx, store, limit)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "No open method runs.", nil
	}
	var b strings.Builder
	b.WriteString("Open method runs:\n")
	for _, run := range runs {
		b.WriteString(fmt.Sprintf("- `%s` — %s (%s)\n",
			run.ID,
			run.TaskSignature.Task,
			run.TaskSignature.Ceremony,
		))
	}
	return b.String(), nil
}

func handleHaftMethodDetail(args map[string]any) (string, error) {
	methodRef, _ := args["method_ref"].(string)
	if methodRef == "" {
		methodRef, _ = args["method_id"].(string)
	}
	if strings.TrimSpace(methodRef) == "" {
		return "", fmt.Errorf("method_ref is required for method detail")
	}
	definition, err := methodpkg.Detail(methodRef)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseMethodPullInput(args map[string]any) (methodpkg.PullInput, error) {
	input := methodpkg.PullInput{}
	input.Task, _ = args["task"].(string)
	input.DeclaredTaskKind, _ = args["declared_task_kind"].(string)
	input.ChangeIntent, _ = args["change_intent"].(string)
	input.CeremonyRequest, _ = args["ceremony_request"].(string)
	input.Context, _ = args["context"].(string)
	input.IntendedFiles = parseStringArrayFromArgs(args, "intended_files")
	input.UserScopeConstraints = parseStringArrayFromArgs(args, "user_scope_constraints")

	riskSignals, err := parseMethodRiskSignals(args, "risk_signals")
	if err != nil {
		return input, err
	}
	input.RiskSignals = riskSignals

	var refs methodpkg.ArtifactRefs
	if present, err := decodeStrictArgFromArgs(args, "artifact_refs", &refs); err != nil {
		return input, fmt.Errorf("artifact_refs must be an object")
	} else if present {
		input.ArtifactRefs = refs
	}

	var budget methodpkg.ResponseBudget
	if present, err := decodeStrictArgFromArgs(args, "response_budget", &budget); err != nil {
		return input, fmt.Errorf("response_budget must be an object")
	} else if present {
		input.ResponseBudget = budget
	}

	return input, nil
}

func parseMethodCloseInput(args map[string]any) (methodpkg.CloseInput, error) {
	input := methodpkg.CloseInput{}
	input.PullID, _ = args["pull_id"].(string)
	input.ChangedFiles = parseStringArrayFromArgs(args, "changed_files")
	if present, err := decodeStrictArgFromArgs(args, "gate_results", &input.GateResults); err != nil {
		return input, fmt.Errorf("gate_results must be an array of gate result objects")
	} else if !present {
		input.GateResults = nil
	}
	if present, err := decodeStrictArgFromArgs(args, "verification", &input.Verification); err != nil {
		return input, fmt.Errorf("verification must be an object")
	} else if !present {
		input.Verification = methodpkg.Verification{}
	}
	if present, err := decodeStrictArgFromArgs(args, "waivers", &input.Waivers); err != nil {
		return input, fmt.Errorf("waivers must be an array of waiver objects")
	} else if !present {
		input.Waivers = nil
	}
	return input, nil
}

func parseMethodRiskSignals(args map[string]any, key string) ([]methodpkg.RiskSignal, error) {
	raw, ok := args[key]
	if !ok {
		return nil, nil
	}
	if values, ok := raw.([]any); ok {
		var signals []methodpkg.RiskSignal
		for _, value := range values {
			switch typed := value.(type) {
			case string:
				signals = append(signals, methodpkg.RiskSignal{ID: typed, Source: "agent_declared"})
			case map[string]any:
				data, _ := json.Marshal(typed)
				var signal methodpkg.RiskSignal
				if err := json.Unmarshal(data, &signal); err != nil {
					return nil, fmt.Errorf("%s contains invalid risk signal object", key)
				}
				signals = append(signals, signal)
			default:
				return nil, fmt.Errorf("%s must contain strings or objects", key)
			}
		}
		return signals, nil
	}
	var signals []methodpkg.RiskSignal
	if _, err := decodeStrictArgFromArgs(args, key, &signals); err == nil {
		return signals, nil
	}
	var names []string
	if _, err := decodeStrictArgFromArgs(args, key, &names); err != nil {
		return nil, fmt.Errorf("%s must be an array of strings or objects", key)
	}
	for _, name := range names {
		signals = append(signals, methodpkg.RiskSignal{ID: name, Source: "agent_declared"})
	}
	return signals, nil
}

func renderMethodPullResponse(run methodpkg.MethodRun, filePath string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Method pull recorded: `%s`\n", run.ID))
	b.WriteString(fmt.Sprintf("Ceremony: %s — %s\n", run.TaskSignature.Ceremony, run.TaskSignature.CeremonyReason))
	if filePath != "" {
		b.WriteString(fmt.Sprintf("File: %s\n", filePath))
	}
	if len(run.Methods) == 0 {
		b.WriteString("No method gates applied. Use normal verification before completion.\n")
	} else {
		b.WriteString("\nMethods:\n")
		for _, card := range run.Methods {
			b.WriteString(fmt.Sprintf("- %s `%s` — %s\n", card.Title, card.ID, card.WhyApplies))
			if posture := methodpkg.RenderSourcePosture(card.SourcePosture); posture != "" {
				b.WriteString(fmt.Sprintf("  - %s\n", posture))
			}
			for _, gate := range card.HardGates {
				b.WriteString(fmt.Sprintf("  - hard gate `%s`: %s\n", gate.ID, gate.PassCondition))
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(renderMethodCloseTemplate(run))
	b.WriteString("Before claiming completion, call `haft_method(action=\"close\", pull_id=\"")
	b.WriteString(run.ID)
	b.WriteString("\", ...)` using this shape, with evidence_refs or waivers for hard gates.\n")
	return b.String()
}

func renderMethodCloseResponse(run methodpkg.MethodRun, filePath string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Method run closed: `%s`\n", run.ID))
	if filePath != "" {
		b.WriteString(fmt.Sprintf("File: %s\n", filePath))
	}
	if run.Closeout != nil && run.Closeout.Verification.Result != "" {
		b.WriteString(fmt.Sprintf("Verification: %s\n", run.Closeout.Verification.Result))
	}
	return b.String()
}

func renderMethodRunSummary(run methodpkg.MethodRun) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Method run `%s`\n", run.ID))
	b.WriteString(fmt.Sprintf("- Status: %s\n", run.Status))
	b.WriteString(fmt.Sprintf("- Ceremony: %s\n", run.TaskSignature.Ceremony))
	b.WriteString(fmt.Sprintf("- Task: %s\n", run.TaskSignature.Task))
	if len(run.Methods) == 0 {
		b.WriteString("- Methods: none\n")
		if run.Status == "open" {
			b.WriteString("\n")
			b.WriteString(renderMethodCloseTemplate(run))
		}
		return b.String()
	}
	b.WriteString("- Methods:\n")
	for _, card := range run.Methods {
		b.WriteString(fmt.Sprintf("  - %s `%s`\n", card.Title, card.ID))
		if posture := methodpkg.RenderSourcePosture(card.SourcePosture); posture != "" {
			b.WriteString(fmt.Sprintf("    - %s\n", posture))
		}
	}
	if run.Status == "open" {
		b.WriteString("\n")
		b.WriteString(renderMethodCloseTemplate(run))
	}
	return b.String()
}

func renderMethodCloseTemplate(run methodpkg.MethodRun) string {
	data, err := json.MarshalIndent(methodpkg.BuildCloseTemplate(run), "", "  ")
	if err != nil {
		return ""
	}
	return "Close template:\n```json\n" + string(data) + "\n```\n"
}

func intArg(args map[string]any, key string, fallback int) int {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		next, err := typed.Int64()
		if err == nil {
			return int(next)
		}
	}
	return fallback
}
