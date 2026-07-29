package p14acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func runP14InstalledCLIRuntimeIdentity(
	_ context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	surface := p14LiveProtocolSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"runtime identity installed CLI surface",
	); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if surface.Schema != p14LiveProtocolSurfaceSchema ||
		surface.Surface != "installed_cli" ||
		surface.Observer !=
			"restart_checkpoint.verify.installed_cli.v1" ||
		surface.SemanticRequestDigest != scenario.SemanticRequestDigest {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 runtime identity installed CLI protocol differs",
		)
	}
	candidateMatch := verifyP14FileDigest(
		execution.ExecutablePath,
		execution.ExecutableDigest,
	) == nil
	fpfCapture, fpfPresent := execution.PriorCaptures["fpf_query_projection"]
	memoryCapture, memoryPresent := execution.PriorCaptures["exact_profile_neighborhood"]
	basisMatch := fpfPresent &&
		memoryPresent &&
		fpfCapture.SurfaceObservation.Outcome ==
			p14SurfaceOutcomeObserved &&
		memoryCapture.SurfaceObservation.Outcome ==
			p14SurfaceOutcomeObserved &&
		validP14Digest(
			fpfCapture.SurfaceObservation.NormalizedResultDigest,
		) &&
		validP14Digest(
			memoryCapture.SurfaceObservation.NormalizedResultDigest,
		)
	checks := []p14InstalledCLICheckReceipt{
		{
			ID:        "candidate_path_and_digest_match",
			Satisfied: candidateMatch,
		},
		{
			ID:        "frozen_query_and_typeenv_basis_match",
			Satisfied: basisMatch,
		},
	}
	result := p14InstalledCLIFamilyResult{
		Receipt: p14InstalledCLIExecutionReceipt{
			Schema:               p14InstalledCLIReceiptSchema,
			ScenarioID:           scenario.ID,
			Builder:              request.Builder,
			CandidateDigest:      execution.ExecutableDigest,
			RequestPayloadDigest: request.PayloadDigest,
			ObservedAt:           time.Now().UTC().Format(time.RFC3339Nano),
			Checks:               checks,
		},
	}
	if !candidateMatch || !basisMatch {
		result.FailureCode = "runtime_identity_mismatch"
		result.FailureDetail =
			"installed candidate or executed FPF/type-environment basis differs"
	}
	return result, nil
}

func runP14InstalledCLIFPFProjection(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	semantic, err := decodeP14FPFProjectionSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if err := validateP14FPFProjectionSemanticRequest(semantic); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	surface := p14FPFProjectionCLISurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"FPF projection installed CLI surface",
	); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if surface.Schema != p14FPFCLISurfaceSchema ||
		surface.SemanticRequestDigest != scenario.SemanticRequestDigest ||
		len(surface.Cases) != len(semantic.Cases) {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 FPF installed CLI surface differs",
		)
	}
	fixture, err := beginP14InstalledCLIReadOnlyFixture(
		execution,
		scenario.ID,
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	environment := p14InstalledCLIEnvironment(map[string]string{
		"HOME":                     fixture.HomeRoot,
		"HAFT_PROJECT_ROOT":        fixture.ProjectRoot,
		"HAFT_EXPECTED_PROJECT_ID": execution.Prepared.Preparation.FrozenBasis.SelectedProject.ProjectID,
	})
	calls := make([]p14InstalledCLIProcessRequest, 0, len(surface.Cases))
	for index, cliCase := range surface.Cases {
		if cliCase.ID != semantic.Cases[index].ID {
			return p14InstalledCLIFamilyResult{}, fmt.Errorf(
				"P14 FPF installed CLI case order differs",
			)
		}
		calls = append(calls, p14InstalledCLIProcessRequest{
			ID:   cliCase.ID,
			Argv: slices.Clone(cliCase.Argv),
		})
	}
	results := p14ExecuteInstalledCLICalls(
		ctx,
		execution,
		fixture.ProjectRoot,
		environment,
		calls,
	)
	fixtureReceipt, err := finishP14InstalledCLIReadOnlyFixture(fixture)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	receipt := p14InstalledCLIExecutionReceipt{
		Schema:               p14InstalledCLIReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CandidateDigest:      execution.ExecutableDigest,
		RequestPayloadDigest: request.PayloadDigest,
		Fixture:              &fixtureReceipt,
		Commands:             p14InstalledCLICommandReceipts(results),
	}
	if p14InstalledCLIProcessResultsHaveExecutionFailure(results) {
		receipt.Checks = p14InstalledCLIChecks(false,
			"exact_argv_from_sealed_payload",
			"closed_fpf_projection_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:        receipt,
			FailureCode:    "command_execution_failed",
			FailureDetail:  "one or more installed FPF commands could not execute",
			ExecutionError: true,
		}, nil
	}
	observations := make(
		map[string]p14FPFProjectionCommandObservation,
		len(results),
	)
	for _, result := range results {
		observations[result.ID] = p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(result.Stdout),
			Stderr:   slices.Clone(result.Stderr),
			ExitCode: result.ExitCode,
		}
	}
	normalized, _, normalizeErr := normalizeP14FPFProjectionObservations(
		semantic.Cases,
		observations,
		execution.Prepared.Preparation.FrozenBasis.Candidate.FPFRevision,
	)
	if normalizeErr != nil {
		receipt.Checks = p14InstalledCLIChecks(false,
			"exact_argv_from_sealed_payload",
			"closed_fpf_projection_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "normalization_failed",
			FailureDetail: boundedP14InstalledCLIError(normalizeErr),
		}, nil
	}
	if err := validateP14InstalledCLIReadOnlyFixture(
		fixtureReceipt,
	); err != nil {
		receipt.Checks = p14InstalledCLIChecks(false,
			"exact_argv_from_sealed_payload",
			"closed_fpf_projection_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "read_only_fixture_changed",
			FailureDetail: boundedP14InstalledCLIError(err),
		}, nil
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	receipt.Checks = p14InstalledCLIChecks(true,
		"exact_argv_from_sealed_payload",
		"closed_fpf_projection_normalizer",
	)
	return p14InstalledCLIFamilyResult{
		Receipt:    receipt,
		Normalized: normalizedBytes,
	}, nil
}

func runP14InstalledCLIMemoryRead(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	semantic, err := decodeP14MemoryReadSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	surface := p14MemoryReadCLISurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"memory-read installed CLI surface",
	); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if surface.Schema != p14MemoryReadCLISurfaceSchema ||
		surface.SemanticRequestDigest != scenario.SemanticRequestDigest ||
		len(surface.Cases) != len(semantic.Cases) {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 memory-read installed CLI surface differs",
		)
	}
	fixture, err := beginP14InstalledCLIReadOnlyFixture(
		execution,
		scenario.ID,
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	project := execution.Prepared.Preparation.FrozenBasis.SelectedProject
	environment := p14InstalledCLIEnvironment(map[string]string{
		"HOME":                     fixture.HomeRoot,
		"HAFT_PROJECT_ROOT":        fixture.ProjectRoot,
		"HAFT_EXPECTED_PROJECT_ID": project.ProjectID,
	})
	calls := make([]p14InstalledCLIProcessRequest, 0, len(surface.Cases))
	for index, cliCase := range surface.Cases {
		if cliCase.ID != semantic.Cases[index].ID {
			return p14InstalledCLIFamilyResult{}, fmt.Errorf(
				"P14 memory-read installed CLI case order differs",
			)
		}
		calls = append(calls, p14InstalledCLIProcessRequest{
			ID:    cliCase.ID,
			Argv:  slices.Clone(cliCase.Argv),
			Stdin: cliCase.Stdin,
		})
	}
	results := p14ExecuteInstalledCLICalls(
		ctx,
		execution,
		fixture.ProjectRoot,
		environment,
		calls,
	)
	fixtureReceipt, err := finishP14InstalledCLIReadOnlyFixture(fixture)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	receipt := p14InstalledCLIExecutionReceipt{
		Schema:               p14InstalledCLIReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CandidateDigest:      execution.ExecutableDigest,
		RequestPayloadDigest: request.PayloadDigest,
		Fixture:              &fixtureReceipt,
		Commands:             p14InstalledCLICommandReceipts(results),
	}
	if p14InstalledCLIProcessResultsHaveExecutionFailure(results) {
		receipt.Checks = p14InstalledCLIChecks(false,
			"exact_read_request_from_sealed_payload",
			"closed_memory_read_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:        receipt,
			FailureCode:    "command_execution_failed",
			FailureDetail:  "one or more installed memory reads could not execute",
			ExecutionError: true,
		}, nil
	}
	observations := make(
		map[string]p14FPFProjectionCommandObservation,
		len(results),
	)
	for _, result := range results {
		observations[result.ID] = p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(result.Stdout),
			Stderr:   slices.Clone(result.Stderr),
			ExitCode: result.ExitCode,
		}
	}
	normalized, _, normalizeErr := normalizeP14MemoryReadObservations(
		semantic,
		observations,
	)
	if normalizeErr != nil {
		receipt.Checks = p14InstalledCLIChecks(false,
			"exact_read_request_from_sealed_payload",
			"closed_memory_read_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "normalization_failed",
			FailureDetail: boundedP14InstalledCLIError(normalizeErr),
		}, nil
	}
	if err := validateP14InstalledCLIReadOnlyFixture(
		fixtureReceipt,
	); err != nil {
		receipt.Checks = p14InstalledCLIChecks(false,
			"exact_read_request_from_sealed_payload",
			"closed_memory_read_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "read_only_fixture_changed",
			FailureDetail: boundedP14InstalledCLIError(err),
		}, nil
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	receipt.Checks = p14InstalledCLIChecks(true,
		"exact_read_request_from_sealed_payload",
		"closed_memory_read_normalizer",
	)
	return p14InstalledCLIFamilyResult{
		Receipt:    receipt,
		Normalized: normalizedBytes,
	}, nil
}

func runP14InstalledCLIInitMatrix(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	semantic, err := decodeP14InitMatrixSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	surface := p14InitMatrixCLISurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"init-matrix installed CLI surface",
	); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if surface.Schema != p14InitMatrixCLISurfaceSchema ||
		surface.SemanticRequestDigest != scenario.SemanticRequestDigest ||
		len(surface.Cases) != len(semantic.Cases) {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 init-matrix installed CLI surface differs",
		)
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	commandResults := make([]p14InstalledCLIProcessResult, 0, len(surface.Cases))
	fixtureReceipts := make(
		[]p14InstalledCLIFixtureReceipt,
		0,
		len(surface.Cases),
	)
	normalizedCases := make(
		[]p14InitMatrixNormalizedCaseOutput,
		0,
		len(surface.Cases),
	)
	allSatisfied := true
	semanticFailures := make([]string, 0)
	for index, cliCase := range surface.Cases {
		semanticCase := semantic.Cases[index]
		if cliCase.ID != semanticCase.ID ||
			!slices.Equal(cliCase.Argv, semanticCase.Argv) {
			return p14InstalledCLIFamilyResult{}, fmt.Errorf(
				"P14 init-matrix installed CLI case order differs",
			)
		}
		projectRoot, homeRoot, err := restoreP14InstalledCLIInitFixture(
			execution.WorkspaceRoot,
			scenario.ID,
			cliCase.ID,
			cliCase.ProjectTemplateRoot,
			cliCase.ProjectTemplateDigest,
			cliCase.HomeTemplateRoot,
			cliCase.HomeTemplateDigest,
			cliCase.ProjectExecutionRoot,
			cliCase.HomeExecutionRoot,
		)
		if err != nil {
			return p14InstalledCLIFamilyResult{}, err
		}
		environment := p14InstalledCLIEnvironment(map[string]string{
			"HOME": homeRoot,
		})
		projectBefore, err := observeP14InstalledCLISemanticTree(projectRoot)
		if err != nil {
			return p14InstalledCLIFamilyResult{}, err
		}
		homeBefore, err := observeP14InstalledCLISemanticTree(homeRoot)
		if err != nil {
			return p14InstalledCLIFamilyResult{}, err
		}
		results := p14ExecuteInstalledCLICalls(
			ctx,
			execution,
			projectRoot,
			environment,
			[]p14InstalledCLIProcessRequest{{
				ID:   cliCase.ID,
				Argv: slices.Clone(cliCase.Argv),
			}},
		)
		result := results[0]
		commandResults = append(commandResults, result)
		projectAfter, err := observeP14InstalledCLISemanticTree(projectRoot)
		if err != nil {
			return p14InstalledCLIFamilyResult{}, err
		}
		homeAfter, err := observeP14InstalledCLISemanticTree(homeRoot)
		if err != nil {
			return p14InstalledCLIFamilyResult{}, err
		}
		fixtureReceipts = append(
			fixtureReceipts,
			p14InstalledCLIFixtureReceipt{
				CaseID:              cliCase.ID,
				Isolation:           cliCase.Preparation,
				ProjectBasisDigest:  cliCase.ProjectTemplateDigest,
				HomeTemplateDigest:  cliCase.HomeTemplateDigest,
				ProjectBeforeDigest: projectBefore,
				ProjectAfterDigest:  projectAfter,
				HomeBeforeDigest:    homeBefore,
				HomeAfterDigest:     homeAfter,
			},
		)
		caseSatisfied := p14InstalledCLIInitExitMatches(
			semanticCase,
			result,
		)
		caseSatisfied = caseSatisfied &&
			p14InstalledCLIInitFixtureEffectMatches(
				semanticCase,
				projectBefore,
				projectAfter,
				homeBefore,
				homeAfter,
			)
		caseSatisfied = caseSatisfied &&
			p14InstalledCLIInitCarriersMatch(
				projectRoot,
				homeRoot,
				semanticCase,
			)
		semanticErr := validateP14InstalledCLIInitSemanticEffects(
			repositoryRoot,
			execution,
			projectRoot,
			homeRoot,
			semanticCase,
		)
		if semanticErr != nil {
			semanticFailures = append(
				semanticFailures,
				semanticCase.ID+": "+semanticErr.Error(),
			)
			caseSatisfied = false
		}
		allSatisfied = allSatisfied && caseSatisfied
		normalizedCases = append(
			normalizedCases,
			p14InitMatrixNormalizedCaseOutput{
				ID:                semanticCase.ID,
				Outcome:           semanticCase.Outcome,
				Applicability:     semanticCase.Applicability,
				RequiredCarriers:  slices.Clone(semanticCase.RequiredCarriers),
				ForbiddenCarriers: slices.Clone(semanticCase.ForbiddenCarriers),
				RemovedLegacyCommands: slices.Clone(
					semanticCase.RemovedLegacyCommands,
				),
			},
		)
	}
	receipt := p14InstalledCLIExecutionReceipt{
		Schema:               p14InstalledCLIReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CandidateDigest:      execution.ExecutableDigest,
		RequestPayloadDigest: request.PayloadDigest,
		Fixtures:             fixtureReceipts,
		Commands:             p14InstalledCLICommandReceipts(commandResults),
		Checks: p14InstalledCLIChecks(allSatisfied,
			p14InitMatrixPreparation,
			"profile_applicability_matches",
			"required_and_forbidden_carriers_match",
			"parsed_mcp_contract_matches",
			"managed_instruction_fragment_digest_matches",
			"canonical_skill_bundle_manifest_matches",
			"exact_host_manifest_inventory_matches",
			"legacy_command_cleanup_matches",
		),
	}
	if !allSatisfied {
		failureDetail := "exit, applicability, or semantic carrier contract differs"
		if len(semanticFailures) != 0 {
			failureDetail = strings.Join(semanticFailures, "; ")
		}
		return p14InstalledCLIFamilyResult{
			Receipt:     receipt,
			FailureCode: "init_matrix_mismatch",
			FailureDetail: boundedP14InstalledCLIError(
				fmt.Errorf("%s", failureDetail),
			),
		}, nil
	}
	normalized := p14InitMatrixNormalizedOutput{
		Schema: p14InitMatrixNormalizedSchema,
		Cases:  normalizedCases,
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	return p14InstalledCLIFamilyResult{
		Receipt:    receipt,
		Normalized: normalizedBytes,
	}, nil
}

func p14InstalledCLIInitFixtureEffectMatches(
	semantic p14InitMatrixSemanticCase,
	projectBefore string,
	projectAfter string,
	homeBefore string,
	homeAfter string,
) bool {
	if semantic.Outcome != "scope_choice_required" {
		return true
	}
	return projectBefore == projectAfter && homeBefore == homeAfter
}

func p14InstalledCLIInitExitMatches(
	semantic p14InitMatrixSemanticCase,
	result p14InstalledCLIProcessResult,
) bool {
	if result.ExecutionError != "" || result.OutputLimited {
		return false
	}
	if semantic.Outcome == "initialized" {
		return result.ExitCode == 0 &&
			len(bytes.TrimSpace(result.Stderr)) == 0
	}
	if semantic.Outcome == "scope_choice_required" {
		return result.ExitCode != 0 &&
			strings.Contains(
				string(result.Stderr),
				"multiple realization scopes",
			)
	}
	return false
}

func p14InstalledCLIInitCarriersMatch(
	projectRoot string,
	homeRoot string,
	semantic p14InitMatrixSemanticCase,
) bool {
	type carrierPath struct {
		Root string
		Path string
	}
	paths := map[string]carrierPath{
		"core:project_identity": {
			Root: projectRoot,
			Path: filepath.Join(".haft", "project.yaml"),
		},
		"host:claude_mcp": {
			Root: projectRoot,
			Path: ".mcp.json",
		},
		"host:claude_instructions": {
			Root: projectRoot,
			Path: "CLAUDE.md",
		},
		"skills:claude_bundle": {
			Root: homeRoot,
			Path: filepath.Join(
				".claude",
				"skills",
				"h-reason",
				"SKILL.md",
			),
		},
		"host:codex_mcp": {
			Root: projectRoot,
			Path: filepath.Join(".codex", "config.toml"),
		},
		"host:codex_instructions": {
			Root: projectRoot,
			Path: "AGENTS.md",
		},
		"skills:agents_bundle": {
			Root: homeRoot,
			Path: filepath.Join(
				".agents",
				"skills",
				"h-reason",
				"SKILL.md",
			),
		},
		"spec:software-system": {
			Root: projectRoot,
			Path: filepath.Join(
				".haft",
				"specs",
				"software-system.md",
			),
		},
		"spec:term-map": {
			Root: projectRoot,
			Path: filepath.Join(".haft", "specs", "term-map.md"),
		},
		"method:swe-core": {
			Root: projectRoot,
			Path: filepath.Join(
				".haft",
				"methods",
				"swe-core",
				"manifest.yaml",
			),
		},
	}
	for _, carrier := range semantic.RequiredCarriers {
		coordinate, present := paths[carrier]
		if !present || !p14InstalledCLIRegularFile(
			filepath.Join(coordinate.Root, coordinate.Path),
		) {
			return false
		}
	}
	for _, carrier := range semantic.ForbiddenCarriers {
		coordinate, present := paths[carrier]
		if !present || p14InstalledCLIPathExists(
			filepath.Join(coordinate.Root, coordinate.Path),
		) {
			return false
		}
	}
	return true
}

func p14InstalledCLIRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func p14InstalledCLIPathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func runP14InstalledCLIMemoryOperation(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	semantic, err := decodeP14MemoryOperationSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	surface := p14MemoryOperationCLISurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"memory-operation installed CLI surface",
	); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if surface.Schema != p14MemoryOperationCLISchema ||
		surface.SemanticRequestDigest != scenario.SemanticRequestDigest ||
		surface.FixtureIsolation !=
			semantic.InstalledCLIFixtureIsolation ||
		surface.SelectedProjectRoot != semantic.SelectedProjectRoot ||
		surface.SelectedProjectBasisDigest !=
			semantic.SelectedProjectBasisDigest ||
		surface.HomeTemplateRoot != semantic.HomeTemplateRoot ||
		surface.HomeTemplateDigest != semantic.HomeTemplateDigest ||
		len(surface.Calls) != len(semantic.Steps) {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 memory-operation installed CLI surface differs",
		)
	}
	projectRoot := surface.SelectedProjectRoot
	projectBasis, err := observeP14SelectedProjectMemoryBasis(projectRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if projectBasis != surface.SelectedProjectBasisDigest {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 memory-operation selected-project basis changed",
		)
	}
	homeRoot, err := cloneP14InstalledCLIHomeFixture(
		execution.WorkspaceRoot,
		scenario.ID,
		"scenario",
		surface.HomeTemplateRoot,
		surface.HomeTemplateDigest,
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	projectBefore, err := observeP14SelectedProjectMemoryBasis(projectRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	homeBefore, err := observeP14InstalledCLISemanticTree(homeRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	calls := make([]p14InstalledCLIProcessRequest, 0, len(surface.Calls))
	for index, call := range surface.Calls {
		step := semantic.Steps[index]
		if call.ID != step.ID ||
			call.ParallelGroup != step.ParallelGroup {
			return p14InstalledCLIFamilyResult{}, fmt.Errorf(
				"P14 memory-operation installed CLI call order differs",
			)
		}
		calls = append(calls, p14InstalledCLIProcessRequest{
			ID:            call.ID,
			ParallelGroup: call.ParallelGroup,
			Argv:          slices.Clone(call.Argv),
			Stdin:         call.Stdin,
		})
	}
	environment := p14InstalledCLIEnvironment(map[string]string{
		"HOME":              homeRoot,
		"HAFT_PROJECT_ROOT": projectRoot,
	})
	results := p14ExecuteInstalledCLICalls(
		ctx,
		execution,
		projectRoot,
		environment,
		calls,
	)
	projectAfter, err := observeP14SelectedProjectMemoryBasis(projectRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	homeAfter, err := observeP14InstalledCLISemanticTree(homeRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	fixtureReceipt := &p14InstalledCLIFixtureReceipt{
		Isolation:           semantic.InstalledCLIFixtureIsolation,
		ProjectBasisDigest:  semantic.SelectedProjectBasisDigest,
		HomeTemplateDigest:  semantic.HomeTemplateDigest,
		ProjectBeforeDigest: projectBefore,
		ProjectAfterDigest:  projectAfter,
		HomeBeforeDigest:    homeBefore,
		HomeAfterDigest:     homeAfter,
	}
	receipt := p14InstalledCLIExecutionReceipt{
		Schema:               p14InstalledCLIReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CandidateDigest:      execution.ExecutableDigest,
		RequestPayloadDigest: request.PayloadDigest,
		Fixture:              fixtureReceipt,
		Commands:             p14InstalledCLICommandReceipts(results),
	}
	if p14InstalledCLIProcessResultsHaveExecutionFailure(results) {
		receipt.Checks = p14InstalledCLIChecks(false,
			"selected_project_read_only_and_fresh_home_clone",
			"closed_memory_operation_protocol",
			"expected_semantic_effect_only",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:        receipt,
			FailureCode:    "command_execution_failed",
			FailureDetail:  "one or more installed memory operations could not execute",
			ExecutionError: true,
		}, nil
	}
	normalizationErr := normalizeP14InstalledCLIMemoryOperation(
		semantic,
		results,
		projectBefore,
		projectAfter,
		homeBefore,
		homeAfter,
	)
	if normalizationErr != nil {
		receipt.Checks = p14InstalledCLIChecks(false,
			"selected_project_read_only_and_fresh_home_clone",
			"closed_memory_operation_protocol",
			"expected_semantic_effect_only",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "normalization_failed",
			FailureDetail: boundedP14InstalledCLIError(normalizationErr),
		}, nil
	}
	receipt.Checks = p14InstalledCLIChecks(true,
		"selected_project_read_only_and_fresh_home_clone",
		"closed_memory_operation_protocol",
		"expected_semantic_effect_only",
	)
	normalized := p14MemoryOperationNormalizedOutput{
		Schema:     p14MemoryOperationOutputSchema,
		ScenarioID: semantic.ScenarioID,
		Expected:   semantic.Expected,
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	return p14InstalledCLIFamilyResult{
		Receipt:    receipt,
		Normalized: normalizedBytes,
	}, nil
}

func normalizeP14InstalledCLIMemoryOperation(
	semantic p14MemoryOperationSemanticRequest,
	results []p14InstalledCLIProcessResult,
	projectBefore string,
	projectAfter string,
	homeBefore string,
	homeAfter string,
) error {
	if len(results) != len(semantic.Steps) {
		return fmt.Errorf("memory-operation result count differs")
	}
	if err := validateP14InstalledCLIMemoryOperationFixtureEffect(
		semantic,
		projectBefore,
		projectAfter,
		homeBefore,
		homeAfter,
	); err != nil {
		return err
	}
	normalizers := map[string]func(
		p14MemoryOperationSemanticRequest,
		[]p14InstalledCLIProcessResult,
	) error{
		"positive_typed_write":    normalizeP14InstalledCLIPositiveWrite,
		"invalid":                 normalizeP14InstalledCLIValidationOnly,
		"underdetermined":         normalizeP14InstalledCLIValidationOnly,
		"authority_rejection":     normalizeP14InstalledCLIAuthorityRejection,
		"concurrency_idempotency": normalizeP14InstalledCLIConcurrency,
	}
	normalizer := normalizers[semantic.ScenarioID]
	if normalizer == nil {
		return fmt.Errorf(
			"memory-operation scenario %q is open",
			semantic.ScenarioID,
		)
	}
	if err := normalizer(semantic, results); err != nil {
		return err
	}
	return nil
}

func validateP14InstalledCLIMemoryOperationFixtureEffect(
	semantic p14MemoryOperationSemanticRequest,
	projectBefore string,
	projectAfter string,
	homeBefore string,
	homeAfter string,
) error {
	if projectBefore != projectAfter {
		return fmt.Errorf("memory-operation changed project carriers")
	}
	writes := semantic.Expected.GraphRevisionDelta > 0
	if !writes && homeBefore != homeAfter {
		return fmt.Errorf(
			"memory-operation no-write scenario changed HOME state",
		)
	}
	if writes && homeBefore == homeAfter {
		return fmt.Errorf("memory-operation expected a durable home-store change")
	}
	return nil
}

func normalizeP14InstalledCLIPositiveWrite(
	semantic p14MemoryOperationSemanticRequest,
	results []p14InstalledCLIProcessResult,
) error {
	if len(results) != 4 {
		return fmt.Errorf("positive-write result count differs")
	}
	if err := assertP14InstalledCLIValidationResult(
		results[0],
		semantic.Expected.Verdict,
	); err != nil {
		return err
	}
	committed, err := p14InstalledCLIAdmissionResult(results[1])
	if err != nil {
		return err
	}
	replayed, err := p14InstalledCLIAdmissionResult(results[2])
	if err != nil {
		return err
	}
	if committed.Disposition != "committed" ||
		replayed.Disposition != "replay" ||
		!bytes.Equal(committed.Receipt, replayed.Receipt) {
		return fmt.Errorf("positive-write commit/replay receipts differ")
	}
	if committed.GraphRevision !=
		uint64(semantic.Expected.GraphRevisionDelta)+
			p14MemoryOperationInitialRevision(semantic) {
		return fmt.Errorf("positive-write graph revision differs")
	}
	payload, _, err := p14InstalledCLIJSONResult(results[3])
	if err != nil {
		return err
	}
	if payload["action"] != "neighborhood" ||
		payload["result_kind"] != "exact_neighborhood" {
		return fmt.Errorf("positive-write reread did not recover exact neighborhood")
	}
	result, valid := payload["result"].(map[string]any)
	if !valid {
		return fmt.Errorf("positive-write reread result is absent")
	}
	root, valid := result["root"].(map[string]any)
	if !valid {
		return fmt.Errorf("positive-write reread root is absent")
	}
	coordinate, valid := root["coordinate"].(map[string]any)
	if !valid || coordinate["reference_id"] != "entity:positive-write" {
		return fmt.Errorf("positive-write reread recovered another entity")
	}
	return nil
}

func normalizeP14InstalledCLIValidationOnly(
	semantic p14MemoryOperationSemanticRequest,
	results []p14InstalledCLIProcessResult,
) error {
	if len(results) != 1 {
		return fmt.Errorf("validation-only result count differs")
	}
	return assertP14InstalledCLIValidationResult(
		results[0],
		semantic.Expected.Verdict,
	)
}

func normalizeP14InstalledCLIAuthorityRejection(
	semantic p14MemoryOperationSemanticRequest,
	results []p14InstalledCLIProcessResult,
) error {
	if len(results) != 1 {
		return fmt.Errorf("authority-rejection result count differs")
	}
	result := results[0]
	if result.ExitCode == 0 ||
		len(bytes.TrimSpace(result.Stdout)) != 0 ||
		!strings.Contains(
			string(result.Stderr),
			semantic.Expected.BoundaryErrorCode+
				" at "+
				semantic.Expected.BoundaryErrorPath,
		) {
		return fmt.Errorf("authority rejection differs")
	}
	return nil
}

func normalizeP14InstalledCLIConcurrency(
	semantic p14MemoryOperationSemanticRequest,
	results []p14InstalledCLIProcessResult,
) error {
	if len(results) != 4 {
		return fmt.Errorf("concurrency result count differs")
	}
	admissions := make([]p14InstalledCLIAdmissionObservation, 0, 3)
	for _, result := range results[:3] {
		admission, err := p14InstalledCLIAdmissionResult(result)
		if err != nil {
			return err
		}
		admissions = append(admissions, admission)
	}
	commitCount := 0
	for _, admission := range admissions {
		if admission.Disposition == "committed" {
			commitCount++
		}
		if admission.Disposition != "committed" &&
			admission.Disposition != "replay" {
			return fmt.Errorf("concurrency admission disposition differs")
		}
		if !bytes.Equal(admission.Receipt, admissions[0].Receipt) {
			return fmt.Errorf("concurrency receipts differ")
		}
	}
	if commitCount != semantic.Expected.CommitCount {
		return fmt.Errorf("concurrency commit count differs")
	}
	conflict := results[3]
	if conflict.ExitCode == 0 ||
		len(bytes.TrimSpace(conflict.Stdout)) != 0 ||
		!strings.Contains(
			string(conflict.Stderr),
			"typed-memory idempotency key was already consumed",
		) {
		return fmt.Errorf("concurrency conflicting request was not rejected")
	}
	return nil
}

func assertP14InstalledCLIValidationResult(
	result p14InstalledCLIProcessResult,
	expectedVerdict string,
) error {
	payload, _, err := p14InstalledCLIJSONResult(result)
	if err != nil {
		return err
	}
	persistence, valid := payload["persistence_disposition"].(map[string]any)
	if !valid ||
		payload["action"] != "validate" ||
		payload["verdict"] != expectedVerdict ||
		persistence["mode"] != "validation_only_no_write" ||
		persistence["authority_granted"] != false {
		return fmt.Errorf("memory validation response differs")
	}
	rows, valid := p14InstalledCLIJSONUint64(persistence["rows_written"])
	if !valid || rows != 0 {
		return fmt.Errorf("memory validation wrote rows")
	}
	return nil
}

type p14InstalledCLIAdmissionObservation struct {
	Disposition   string
	GraphRevision uint64
	Receipt       []byte
}

func p14InstalledCLIAdmissionResult(
	result p14InstalledCLIProcessResult,
) (p14InstalledCLIAdmissionObservation, error) {
	payload, _, err := p14InstalledCLIJSONResult(result)
	if err != nil {
		return p14InstalledCLIAdmissionObservation{}, err
	}
	persistence, valid := payload["persistence_disposition"].(map[string]any)
	if !valid ||
		payload["action"] != "admit" ||
		payload["result"] != "committed" ||
		payload["authority_class"] !=
			"non_binding_semantic_assertion" ||
		persistence["mode"] != "transactional_project_memory_commit" ||
		persistence["authority_granted"] != false {
		return p14InstalledCLIAdmissionObservation{}, fmt.Errorf(
			"memory admission response differs",
		)
	}
	disposition, valid := persistence["disposition"].(string)
	if !valid {
		return p14InstalledCLIAdmissionObservation{}, fmt.Errorf(
			"memory admission disposition is absent",
		)
	}
	disposition, valid = p14InstalledCLIAdmissionSemanticDisposition(
		disposition,
	)
	if !valid {
		return p14InstalledCLIAdmissionObservation{}, fmt.Errorf(
			"memory admission disposition is unsupported",
		)
	}
	receipt, valid := payload["receipt"].(map[string]any)
	if !valid {
		return p14InstalledCLIAdmissionObservation{}, fmt.Errorf(
			"memory admission receipt is absent",
		)
	}
	graphRevision, valid := p14InstalledCLIJSONUint64(
		receipt["graph_revision"],
	)
	if !valid ||
		!validP14Digest(p14InstalledCLIString(receipt["result_digest"])) ||
		p14InstalledCLIString(receipt["event_ref"]) == "" ||
		p14InstalledCLIString(receipt["commit_ref"]) == "" {
		return p14InstalledCLIAdmissionObservation{}, fmt.Errorf(
			"memory admission receipt is incomplete",
		)
	}
	receiptBytes, err := marshalP14CanonicalJSON(receipt)
	if err != nil {
		return p14InstalledCLIAdmissionObservation{}, err
	}
	return p14InstalledCLIAdmissionObservation{
		Disposition:   disposition,
		GraphRevision: graphRevision,
		Receipt:       receiptBytes,
	}, nil
}

func p14InstalledCLIAdmissionSemanticDisposition(
	runtimeDisposition string,
) (string, bool) {
	switch runtimeDisposition {
	case "applied":
		return "committed", true
	case "replay":
		return "replay", true
	default:
		return "", false
	}
}

func p14MemoryOperationInitialRevision(
	semantic p14MemoryOperationSemanticRequest,
) uint64 {
	if len(semantic.Steps) == 0 {
		return 0
	}
	request := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(semantic.Steps[0].Request))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		return 0
	}
	basis, valid := request["basis"].(map[string]any)
	if !valid {
		return 0
	}
	revision, _ := p14InstalledCLIJSONUint64(basis["graph_revision"])
	return revision
}

func runP14InstalledCLIExistingRecordBackfill(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	semantic, err := decodeP14ExistingRecordBackfillSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	surface, err := decodeP14ExistingRecordBackfillCLISurface(
		[]byte(request.CanonicalPayload),
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if surface.SemanticRequestDigest != scenario.SemanticRequestDigest ||
		surface.FixtureIsolation != semantic.FixtureIsolation ||
		surface.SelectedProjectRoot != semantic.SelectedProjectRoot ||
		surface.SelectedProjectBasisDigest !=
			semantic.SelectedProjectBasisDigest ||
		surface.HomeTemplateRoot != semantic.HomeTemplateRoot ||
		surface.HomeTemplateDigest != semantic.HomeTemplateDigest ||
		len(surface.Calls) != len(semantic.Calls) {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 existing-record backfill installed CLI surface differs",
		)
	}
	projectRoot := surface.SelectedProjectRoot
	projectBasis, err := observeP14SelectedProjectMemoryBasis(projectRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if projectBasis != surface.SelectedProjectBasisDigest {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 backfill selected-project basis changed",
		)
	}
	homeRoot, err := cloneP14InstalledCLIHomeFixture(
		execution.WorkspaceRoot,
		scenario.ID,
		"scenario",
		surface.HomeTemplateRoot,
		surface.HomeTemplateDigest,
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	projectBefore, err := observeP14SelectedProjectMemoryBasis(projectRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	homeBefore, err := observeP14InstalledCLISemanticTree(homeRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	calls := make([]p14InstalledCLIProcessRequest, 0, len(surface.Calls))
	for index, call := range surface.Calls {
		if call.ID != semantic.Calls[index].ID {
			return p14InstalledCLIFamilyResult{}, fmt.Errorf(
				"P14 existing-record backfill call order differs",
			)
		}
		calls = append(calls, p14InstalledCLIProcessRequest{
			ID:    call.ID,
			Argv:  slices.Clone(call.Argv),
			Stdin: call.Stdin,
		})
	}
	environment := p14InstalledCLIEnvironment(map[string]string{
		"HOME":              homeRoot,
		"HAFT_PROJECT_ROOT": projectRoot,
	})
	results := p14ExecuteInstalledCLICalls(
		ctx,
		execution,
		projectRoot,
		environment,
		calls,
	)
	projectAfter, err := observeP14SelectedProjectMemoryBasis(projectRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	homeAfter, err := observeP14InstalledCLISemanticTree(homeRoot)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	receipt := p14InstalledCLIExecutionReceipt{
		Schema:               p14InstalledCLIReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CandidateDigest:      execution.ExecutableDigest,
		RequestPayloadDigest: request.PayloadDigest,
		Fixture: &p14InstalledCLIFixtureReceipt{
			Isolation:           semantic.FixtureIsolation,
			ProjectBasisDigest:  semantic.SelectedProjectBasisDigest,
			HomeTemplateDigest:  semantic.HomeTemplateDigest,
			ProjectBeforeDigest: projectBefore,
			ProjectAfterDigest:  projectAfter,
			HomeBeforeDigest:    homeBefore,
			HomeAfterDigest:     homeAfter,
		},
		Commands: p14InstalledCLICommandReceipts(results),
	}
	if p14InstalledCLIProcessResultsHaveExecutionFailure(results) {
		receipt.Checks = p14InstalledCLIChecks(false,
			"selected_project_read_only_and_fresh_home_clone",
			"dry_run_apply_replay_protocol",
			"source_carriers_byte_identical",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:        receipt,
			FailureCode:    "command_execution_failed",
			FailureDetail:  "one or more installed backfill calls could not execute",
			ExecutionError: true,
		}, nil
	}
	normalizeErr := normalizeP14InstalledCLIExistingRecordBackfill(
		semantic,
		results,
		projectBefore,
		projectAfter,
		homeBefore,
		homeAfter,
	)
	if normalizeErr != nil {
		receipt.Checks = p14InstalledCLIChecks(false,
			"selected_project_read_only_and_fresh_home_clone",
			"dry_run_apply_replay_protocol",
			"source_carriers_byte_identical",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "normalization_failed",
			FailureDetail: boundedP14InstalledCLIError(normalizeErr),
		}, nil
	}
	receipt.Checks = p14InstalledCLIChecks(true,
		"selected_project_read_only_and_fresh_home_clone",
		"dry_run_apply_replay_protocol",
		"source_carriers_byte_identical",
	)
	normalized := p14ExistingRecordBackfillNormalizedOutput{
		Schema:     p14ExistingRecordBackfillOutputSchema,
		ScenarioID: semantic.ScenarioID,
		Expected:   semantic.Expected,
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	return p14InstalledCLIFamilyResult{
		Receipt:    receipt,
		Normalized: normalizedBytes,
	}, nil
}

func normalizeP14InstalledCLIExistingRecordBackfill(
	semantic p14ExistingRecordBackfillSemanticRequest,
	results []p14InstalledCLIProcessResult,
	projectBefore string,
	projectAfter string,
	homeBefore string,
	homeAfter string,
) error {
	if len(results) != len(semantic.Expected.Calls) ||
		projectBefore != projectAfter ||
		homeBefore == homeAfter {
		return fmt.Errorf("backfill fixture effect differs")
	}
	for index, result := range results {
		payload, _, err := p14InstalledCLIJSONDocumentResult(result)
		if err != nil {
			return err
		}
		expected := semantic.Expected.Calls[index]
		if payload["contract_version"] !=
			p14ExistingRecordBackfillContractVersion ||
			payload["mode"] != semantic.Calls[index].Request.Mode ||
			payload["request_provenance_ref"] !=
				semantic.Calls[index].Request.RequestProvenanceRef {
			return fmt.Errorf("backfill response envelope differs")
		}
		before, validBefore := p14InstalledCLIJSONUint64(
			payload["graph_revision_before"],
		)
		after, validAfter := p14InstalledCLIJSONUint64(
			payload["graph_revision_after"],
		)
		if !validBefore || !validAfter ||
			int64(after)-int64(before) != expected.GraphRevisionDelta {
			return fmt.Errorf("backfill graph revision delta differs")
		}
		routes, valid := payload["routes"].([]any)
		if !valid || len(routes) != 1 {
			return fmt.Errorf("backfill route count differs")
		}
		route, valid := routes[0].(map[string]any)
		if !valid || route["result"] != expected.Result {
			return fmt.Errorf("backfill route result differs")
		}
		if expected.DurableChangeCount > 0 {
			report, valid := route["projection_report"].(map[string]any)
			if !valid {
				return fmt.Errorf("backfill projection report is absent")
			}
			durable, valid := p14InstalledCLIJSONUint64(
				report["durable_change_count"],
			)
			if !valid || durable != uint64(expected.DurableChangeCount) {
				return fmt.Errorf("backfill durable change count differs")
			}
		}
		if !p14InstalledCLIBackfillAuthorityBoundary(
			payload["authority_boundary"],
			semantic.Calls[index].Request.Mode,
		) {
			return fmt.Errorf("backfill authority boundary differs")
		}
	}
	return nil
}

func p14InstalledCLIBackfillAuthorityBoundary(
	value any,
	mode string,
) bool {
	boundary, valid := value.(map[string]any)
	if !valid {
		return false
	}
	mutation := "validation_only_zero_write"
	if mode == "apply" {
		mutation = "explicit_selected_non_binding_projection_admission"
	}
	expected := map[string]string{
		"mutation":       mutation,
		"schema":         "not_schema_declaration_or_activation",
		"type_env_head":  "not_typeenv_head_selection_or_mutation",
		"decision":       "not_decision_binding_or_supersession",
		"specification":  "not_specification_approval_reopen_or_rebaseline",
		"evidence_truth": "not_evidence_truth_or_quality",
		"performed_work": "not_performed_work_or_completion",
		"publication":    "not_publication_or_release",
	}
	for key, value := range expected {
		if boundary[key] != value {
			return false
		}
	}
	return true
}

func p14InstalledCLIJSONResult(
	result p14InstalledCLIProcessResult,
) (map[string]any, []byte, error) {
	return decodeP14CandidateJSONObservation(
		p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(result.Stdout),
			Stderr:   slices.Clone(result.Stderr),
			ExitCode: result.ExitCode,
		},
	)
}

func p14InstalledCLIJSONDocumentResult(
	result p14InstalledCLIProcessResult,
) (map[string]any, []byte, error) {
	stderr := boundedP14FPFText(result.Stderr)
	if result.ExitCode != 0 {
		return nil, nil, fmt.Errorf(
			"candidate exit code = %d; stderr=%s",
			result.ExitCode,
			stderr,
		)
	}
	if len(bytes.TrimSpace(result.Stderr)) != 0 {
		return nil, nil, fmt.Errorf(
			"candidate emitted stderr on success: %s",
			stderr,
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(result.Stdout))
	decoder.UseNumber()
	payload := make(map[string]any)
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf(
			"decode candidate response document: %w",
			err,
		)
	}
	trailing := any(nil)
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, nil, fmt.Errorf(
			"candidate response document has trailing JSON",
		)
	}
	canonical, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		return nil, nil, err
	}
	return payload, canonical, nil
}

func p14InstalledCLIJSONUint64(value any) (uint64, bool) {
	number, valid := value.(json.Number)
	if !valid {
		return 0, false
	}
	parsed, err := number.Int64()
	if err != nil || parsed < 0 {
		return 0, false
	}
	return uint64(parsed), true
}

func p14InstalledCLIString(value any) string {
	text, _ := value.(string)
	return text
}

type p14InstalledCLIExpectedCommand struct {
	ID            string
	ParallelGroup string
	Argv          []string
	StdinDigest   string
}

func validateP14InstalledCLIReceiptEvidence(
	scenario preparedP14Scenario,
	request preparedP14Request,
	basis frozenP14Basis,
	prior map[string]p14InstalledCLIScenarioCapture,
	receipt p14InstalledCLIExecutionReceipt,
	observation p14InstalledSurfaceObservation,
) error {
	expectedCommands, err := p14InstalledCLIExpectedCommands(
		request,
	)
	if err != nil {
		return err
	}
	if err := validateP14InstalledCLIExactCommands(
		expectedCommands,
		receipt.Commands,
	); err != nil {
		return err
	}
	expectedChecks, err := p14InstalledCLIExpectedCheckIDs(
		scenario,
		request,
	)
	if err != nil {
		return err
	}
	actualChecks := make([]string, 0, len(receipt.Checks))
	for _, check := range receipt.Checks {
		actualChecks = append(actualChecks, check.ID)
	}
	if !slices.Equal(actualChecks, expectedChecks) {
		return fmt.Errorf(
			"P14 installed CLI receipt check IDs differ: got %v, want %v",
			actualChecks,
			expectedChecks,
		)
	}
	if scenario.Oracle.Kind == "live_predicate" {
		return validateP14InstalledCLICommandlessPredicate(
			scenario,
			prior,
			receipt,
			observation,
		)
	}
	if scenario.Oracle.Kind != "normalized_digest" {
		return fmt.Errorf(
			"P14 installed CLI oracle kind %q is open",
			scenario.Oracle.Kind,
		)
	}
	if observation.Outcome != p14SurfaceOutcomeObserved {
		if observation.NormalizedResultDigest != "" {
			return fmt.Errorf(
				"P14 failed installed CLI observation carries a normalized digest",
			)
		}
		return nil
	}
	normalized, err := recomputeP14InstalledCLINormalizedResult(
		scenario,
		request,
		basis,
		receipt,
	)
	if err != nil {
		return fmt.Errorf(
			"recompute P14 installed CLI normalized result: %w",
			err,
		)
	}
	normalizedDigest := p14Digest(normalized)
	if normalizedDigest != scenario.Oracle.ExpectedResultDigest ||
		normalizedDigest != observation.NormalizedResultDigest {
		return fmt.Errorf(
			"P14 installed CLI normalized result is not derived from captured evidence",
		)
	}
	return nil
}

func p14InstalledCLIExpectedCommands(
	request preparedP14Request,
) ([]p14InstalledCLIExpectedCommand, error) {
	builder := request.Builder
	if builder == p14RuntimeIdentityBuilderID ||
		slices.Contains(p14AgentOrientationBuilderIDs, builder) {
		return []p14InstalledCLIExpectedCommand{}, nil
	}
	if builder == p14FPFProjectionBuilderID {
		surface := p14FPFProjectionCLISurface{}
		if err := decodeP14StrictCompactJSON(
			request.CanonicalPayload,
			&surface,
			"FPF installed CLI command plan",
		); err != nil {
			return nil, err
		}
		commands := make(
			[]p14InstalledCLIExpectedCommand,
			0,
			len(surface.Cases),
		)
		for _, testCase := range surface.Cases {
			commands = append(commands, p14InstalledCLIExpectedCommand{
				ID:          testCase.ID,
				Argv:        slices.Clone(testCase.Argv),
				StdinDigest: p14Digest(nil),
			})
		}
		return commands, nil
	}
	if slices.Contains(p14MemoryReadBuilderIDs, builder) {
		surface := p14MemoryReadCLISurface{}
		if err := decodeP14StrictCompactJSON(
			request.CanonicalPayload,
			&surface,
			"memory-read installed CLI command plan",
		); err != nil {
			return nil, err
		}
		commands := make(
			[]p14InstalledCLIExpectedCommand,
			0,
			len(surface.Cases),
		)
		for _, testCase := range surface.Cases {
			commands = append(commands, p14InstalledCLIExpectedCommand{
				ID:          testCase.ID,
				Argv:        slices.Clone(testCase.Argv),
				StdinDigest: p14Digest([]byte(testCase.Stdin)),
			})
		}
		return commands, nil
	}
	if builder == p14InitMatrixBuilderID {
		surface := p14InitMatrixCLISurface{}
		if err := decodeP14StrictCompactJSON(
			request.CanonicalPayload,
			&surface,
			"init-matrix installed CLI command plan",
		); err != nil {
			return nil, err
		}
		commands := make(
			[]p14InstalledCLIExpectedCommand,
			0,
			len(surface.Cases),
		)
		for _, testCase := range surface.Cases {
			commands = append(commands, p14InstalledCLIExpectedCommand{
				ID:          testCase.ID,
				Argv:        slices.Clone(testCase.Argv),
				StdinDigest: p14Digest(nil),
			})
		}
		return commands, nil
	}
	if slices.Contains(p14MemoryOperationBuilderIDs, builder) {
		surface := p14MemoryOperationCLISurface{}
		if err := decodeP14StrictCompactJSON(
			request.CanonicalPayload,
			&surface,
			"memory-operation installed CLI command plan",
		); err != nil {
			return nil, err
		}
		commands := make(
			[]p14InstalledCLIExpectedCommand,
			0,
			len(surface.Calls),
		)
		for _, call := range surface.Calls {
			commands = append(commands, p14InstalledCLIExpectedCommand{
				ID:            call.ID,
				ParallelGroup: call.ParallelGroup,
				Argv:          slices.Clone(call.Argv),
				StdinDigest:   p14Digest([]byte(call.Stdin)),
			})
		}
		return commands, nil
	}
	if builder == p14ExistingRecordBackfillBuilderID {
		surface, err := decodeP14ExistingRecordBackfillCLISurface(
			[]byte(request.CanonicalPayload),
		)
		if err != nil {
			return nil, err
		}
		commands := make(
			[]p14InstalledCLIExpectedCommand,
			0,
			len(surface.Calls),
		)
		for _, call := range surface.Calls {
			commands = append(commands, p14InstalledCLIExpectedCommand{
				ID:          call.ID,
				Argv:        slices.Clone(call.Argv),
				StdinDigest: p14Digest([]byte(call.Stdin)),
			})
		}
		return commands, nil
	}
	if slices.Contains(p14CodeExploreBuilderIDs, builder) {
		surface := p14CodeExploreCLISurface{}
		if err := decodeP14StrictCompactJSON(
			request.CanonicalPayload,
			&surface,
			"code Explore installed CLI command plan",
		); err != nil {
			return nil, err
		}
		return []p14InstalledCLIExpectedCommand{{
			ID:          "explore",
			Argv:        slices.Clone(surface.Argv),
			StdinDigest: p14Digest(nil),
		}}, nil
	}
	return nil, fmt.Errorf(
		"P14 installed CLI builder %q has no command plan",
		builder,
	)
}

func validateP14InstalledCLIExactCommands(
	expected []p14InstalledCLIExpectedCommand,
	actual []p14InstalledCLICommandReceipt,
) error {
	if len(actual) != len(expected) {
		return fmt.Errorf(
			"P14 installed CLI command count = %d, want %d",
			len(actual),
			len(expected),
		)
	}
	for index, command := range actual {
		want := expected[index]
		if command.ID != want.ID ||
			command.ParallelGroup != want.ParallelGroup ||
			!slices.Equal(command.Argv, want.Argv) ||
			command.StdinDigest != want.StdinDigest {
			return fmt.Errorf(
				"P14 installed CLI command %d differs from sealed payload",
				index,
			)
		}
	}
	return nil
}

func p14InstalledCLIExpectedCheckIDs(
	scenario preparedP14Scenario,
	request preparedP14Request,
) ([]string, error) {
	builder := request.Builder
	if builder == p14RuntimeIdentityBuilderID {
		return []string{
			"candidate_path_and_digest_match",
			"frozen_query_and_typeenv_basis_match",
		}, nil
	}
	if slices.Contains(p14AgentOrientationBuilderIDs, builder) {
		policy, err := p14LiveProtocolPolicyForScenario(scenario.ID)
		if err != nil {
			return nil, err
		}
		return slices.Clone(policy.SurfaceChecks["installed_cli"]), nil
	}
	values := map[string][]string{
		p14FPFProjectionBuilderID: {
			"exact_argv_from_sealed_payload",
			"closed_fpf_projection_normalizer",
		},
		p14InitMatrixBuilderID: {
			p14InitMatrixPreparation,
			"profile_applicability_matches",
			"required_and_forbidden_carriers_match",
			"parsed_mcp_contract_matches",
			"managed_instruction_fragment_digest_matches",
			"canonical_skill_bundle_manifest_matches",
			"exact_host_manifest_inventory_matches",
		},
		p14ExistingRecordBackfillBuilderID: {
			"selected_project_read_only_and_fresh_home_clone",
			"dry_run_apply_replay_protocol",
			"source_carriers_byte_identical",
		},
	}
	if checks := values[builder]; len(checks) != 0 {
		return slices.Clone(checks), nil
	}
	if slices.Contains(p14MemoryReadBuilderIDs, builder) {
		return []string{
			"exact_read_request_from_sealed_payload",
			"closed_memory_read_normalizer",
		}, nil
	}
	if slices.Contains(p14MemoryOperationBuilderIDs, builder) {
		return []string{
			"selected_project_read_only_and_fresh_home_clone",
			"closed_memory_operation_protocol",
			"expected_semantic_effect_only",
		}, nil
	}
	if slices.Contains(p14CodeExploreBuilderIDs, builder) {
		return []string{
			"exact_graph_argv_from_sealed_payload",
			"closed_code_explore_normalizer",
		}, nil
	}
	return nil, fmt.Errorf(
		"P14 installed CLI builder %q has no check plan",
		builder,
	)
}

func validateP14InstalledCLICommandlessPredicate(
	scenario preparedP14Scenario,
	prior map[string]p14InstalledCLIScenarioCapture,
	receipt p14InstalledCLIExecutionReceipt,
	observation p14InstalledSurfaceObservation,
) error {
	if len(receipt.Commands) != 0 ||
		receipt.Fixture != nil ||
		len(receipt.Fixtures) != 0 {
		return fmt.Errorf(
			"P14 commandless installed CLI predicate carries command evidence",
		)
	}
	satisfied := false
	if scenario.ID == "runtime_identity" {
		fpf, fpfPresent := prior["fpf_query_projection"]
		memory, memoryPresent := prior["exact_profile_neighborhood"]
		satisfied = fpfPresent &&
			memoryPresent &&
			fpf.SurfaceObservation.Outcome ==
				p14SurfaceOutcomeObserved &&
			memory.SurfaceObservation.Outcome ==
				p14SurfaceOutcomeObserved &&
			validP14Digest(
				fpf.SurfaceObservation.NormalizedResultDigest,
			) &&
			validP14Digest(
				memory.SurfaceObservation.NormalizedResultDigest,
			)
	} else {
		priorScenario, err := p14AgentOrientationPriorScenario(
			scenario.ID,
		)
		if err != nil {
			return err
		}
		dependency, present := prior[priorScenario]
		satisfied = present &&
			dependency.SurfaceObservation.Outcome ==
				p14SurfaceOutcomeObserved &&
			validP14Digest(
				dependency.SurfaceObservation.NormalizedResultDigest,
			)
	}
	for _, check := range receipt.Checks {
		if check.Satisfied != satisfied {
			return fmt.Errorf(
				"P14 commandless installed CLI predicate is self-attested",
			)
		}
	}
	if observation.Outcome == p14SurfaceOutcomeObserved && !satisfied {
		return fmt.Errorf(
			"P14 commandless installed CLI predicate lacks its exact dependency",
		)
	}
	return nil
}

func recomputeP14InstalledCLINormalizedResult(
	scenario preparedP14Scenario,
	request preparedP14Request,
	basis frozenP14Basis,
	receipt p14InstalledCLIExecutionReceipt,
) ([]byte, error) {
	results, err := p14InstalledCLIResultsFromReceipts(receipt.Commands)
	if err != nil {
		return nil, err
	}
	builder := request.Builder
	if builder == p14FPFProjectionBuilderID {
		return recomputeP14InstalledCLIFPFProjection(
			scenario,
			basis,
			results,
			receipt,
		)
	}
	if slices.Contains(p14MemoryReadBuilderIDs, builder) {
		return recomputeP14InstalledCLIMemoryRead(
			scenario,
			results,
			receipt,
		)
	}
	if builder == p14InitMatrixBuilderID {
		return recomputeP14InstalledCLIInitMatrix(
			scenario,
			request,
			results,
			receipt,
		)
	}
	if slices.Contains(p14MemoryOperationBuilderIDs, builder) {
		return recomputeP14InstalledCLIMemoryOperation(
			scenario,
			results,
			receipt,
		)
	}
	if builder == p14ExistingRecordBackfillBuilderID {
		return recomputeP14InstalledCLIBackfill(
			scenario,
			results,
			receipt,
		)
	}
	if slices.Contains(p14CodeExploreBuilderIDs, builder) {
		return recomputeP14InstalledCLICodeExplore(
			scenario,
			results,
			receipt,
		)
	}
	return nil, fmt.Errorf(
		"P14 installed CLI builder %q has no evidence normalizer",
		builder,
	)
}

func p14InstalledCLIResultsFromReceipts(
	receipts []p14InstalledCLICommandReceipt,
) ([]p14InstalledCLIProcessResult, error) {
	results := make([]p14InstalledCLIProcessResult, 0, len(receipts))
	for _, receipt := range receipts {
		stdout, err := base64.StdEncoding.DecodeString(receipt.StdoutBase64)
		if err != nil {
			return nil, err
		}
		stderr, err := base64.StdEncoding.DecodeString(receipt.StderrBase64)
		if err != nil {
			return nil, err
		}
		results = append(results, p14InstalledCLIProcessResult{
			ID:             receipt.ID,
			ParallelGroup:  receipt.ParallelGroup,
			Argv:           slices.Clone(receipt.Argv),
			StdinDigest:    receipt.StdinDigest,
			ExitCode:       receipt.ExitCode,
			Stdout:         stdout,
			Stderr:         stderr,
			OutputLimited:  receipt.OutputLimited,
			ExecutionError: receipt.ExecutionError,
		})
	}
	return results, nil
}

func recomputeP14InstalledCLIFPFProjection(
	scenario preparedP14Scenario,
	basis frozenP14Basis,
	results []p14InstalledCLIProcessResult,
	receipt p14InstalledCLIExecutionReceipt,
) ([]byte, error) {
	if receipt.Fixture == nil || len(receipt.Fixtures) != 0 {
		return nil, fmt.Errorf("P14 FPF receipt omits its read-only fixture")
	}
	if err := validateP14InstalledCLIReadOnlyFixture(
		*receipt.Fixture,
	); err != nil {
		return nil, err
	}
	semantic, err := decodeP14FPFProjectionSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return nil, err
	}
	observations := make(
		map[string]p14FPFProjectionCommandObservation,
		len(results),
	)
	for _, result := range results {
		observations[result.ID] = p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(result.Stdout),
			Stderr:   slices.Clone(result.Stderr),
			ExitCode: result.ExitCode,
		}
	}
	normalized, _, err := normalizeP14FPFProjectionObservations(
		semantic.Cases,
		observations,
		basis.Candidate.FPFRevision,
	)
	if err != nil {
		return nil, err
	}
	return marshalP14CanonicalJSON(normalized)
}

func recomputeP14InstalledCLIMemoryRead(
	scenario preparedP14Scenario,
	results []p14InstalledCLIProcessResult,
	receipt p14InstalledCLIExecutionReceipt,
) ([]byte, error) {
	if receipt.Fixture == nil || len(receipt.Fixtures) != 0 {
		return nil, fmt.Errorf(
			"P14 memory-read receipt omits its read-only fixture",
		)
	}
	if err := validateP14InstalledCLIReadOnlyFixture(
		*receipt.Fixture,
	); err != nil {
		return nil, err
	}
	semantic, err := decodeP14MemoryReadSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return nil, err
	}
	observations := make(
		map[string]p14FPFProjectionCommandObservation,
		len(results),
	)
	for _, result := range results {
		observations[result.ID] = p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(result.Stdout),
			Stderr:   slices.Clone(result.Stderr),
			ExitCode: result.ExitCode,
		}
	}
	normalized, _, err := normalizeP14MemoryReadObservations(
		semantic,
		observations,
	)
	if err != nil {
		return nil, err
	}
	return marshalP14CanonicalJSON(normalized)
}

func recomputeP14InstalledCLIInitMatrix(
	scenario preparedP14Scenario,
	request preparedP14Request,
	results []p14InstalledCLIProcessResult,
	receipt p14InstalledCLIExecutionReceipt,
) ([]byte, error) {
	semantic, err := decodeP14InitMatrixSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return nil, err
	}
	surface := p14InitMatrixCLISurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"init-matrix recomputation surface",
	); err != nil {
		return nil, err
	}
	if receipt.Fixture != nil ||
		len(receipt.Fixtures) != len(semantic.Cases) ||
		len(results) != len(semantic.Cases) {
		return nil, fmt.Errorf(
			"P14 init-matrix receipt evidence count differs",
		)
	}
	normalizedCases := make(
		[]p14InitMatrixNormalizedCaseOutput,
		0,
		len(semantic.Cases),
	)
	for index, semanticCase := range semantic.Cases {
		fixture := receipt.Fixtures[index]
		surfaceCase := surface.Cases[index]
		if fixture.CaseID != semanticCase.ID ||
			fixture.CaseID != surfaceCase.ID ||
			fixture.Isolation != surfaceCase.Preparation ||
			fixture.ProjectBasisDigest !=
				surfaceCase.ProjectTemplateDigest ||
			fixture.HomeTemplateDigest !=
				surfaceCase.HomeTemplateDigest ||
			fixture.ProjectBeforeDigest !=
				fixture.ProjectBasisDigest ||
			fixture.HomeBeforeDigest !=
				fixture.HomeTemplateDigest ||
			!p14InstalledCLIInitExitMatches(
				semanticCase,
				results[index],
			) ||
			!p14InstalledCLIInitFixtureEffectMatches(
				semanticCase,
				fixture.ProjectBeforeDigest,
				fixture.ProjectAfterDigest,
				fixture.HomeBeforeDigest,
				fixture.HomeAfterDigest,
			) {
			return nil, fmt.Errorf(
				"P14 init-matrix case %q evidence differs",
				semanticCase.ID,
			)
		}
		normalizedCases = append(
			normalizedCases,
			p14InitMatrixNormalizedCaseOutput{
				ID:                semanticCase.ID,
				Outcome:           semanticCase.Outcome,
				Applicability:     semanticCase.Applicability,
				RequiredCarriers:  slices.Clone(semanticCase.RequiredCarriers),
				ForbiddenCarriers: slices.Clone(semanticCase.ForbiddenCarriers),
				RemovedLegacyCommands: slices.Clone(
					semanticCase.RemovedLegacyCommands,
				),
			},
		)
	}
	normalized := p14InitMatrixNormalizedOutput{
		Schema: p14InitMatrixNormalizedSchema,
		Cases:  normalizedCases,
	}
	return marshalP14CanonicalJSON(normalized)
}

func recomputeP14InstalledCLIMemoryOperation(
	scenario preparedP14Scenario,
	results []p14InstalledCLIProcessResult,
	receipt p14InstalledCLIExecutionReceipt,
) ([]byte, error) {
	if receipt.Fixture == nil || len(receipt.Fixtures) != 0 {
		return nil, fmt.Errorf(
			"P14 memory-operation receipt fixture differs",
		)
	}
	semantic, err := decodeP14MemoryOperationSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return nil, err
	}
	fixture := *receipt.Fixture
	if fixture.Isolation != semantic.InstalledCLIFixtureIsolation ||
		fixture.ProjectBasisDigest !=
			semantic.SelectedProjectBasisDigest ||
		fixture.HomeTemplateDigest != semantic.HomeTemplateDigest ||
		fixture.ProjectBeforeDigest != fixture.ProjectBasisDigest ||
		fixture.HomeBeforeDigest != fixture.HomeTemplateDigest {
		return nil, fmt.Errorf(
			"P14 memory-operation receipt basis differs",
		)
	}
	if err := normalizeP14InstalledCLIMemoryOperation(
		semantic,
		results,
		fixture.ProjectBeforeDigest,
		fixture.ProjectAfterDigest,
		fixture.HomeBeforeDigest,
		fixture.HomeAfterDigest,
	); err != nil {
		return nil, err
	}
	normalized := p14MemoryOperationNormalizedOutput{
		Schema:     p14MemoryOperationOutputSchema,
		ScenarioID: semantic.ScenarioID,
		Expected:   semantic.Expected,
	}
	return marshalP14CanonicalJSON(normalized)
}

func recomputeP14InstalledCLIBackfill(
	scenario preparedP14Scenario,
	results []p14InstalledCLIProcessResult,
	receipt p14InstalledCLIExecutionReceipt,
) ([]byte, error) {
	if receipt.Fixture == nil || len(receipt.Fixtures) != 0 {
		return nil, fmt.Errorf("P14 backfill receipt fixture differs")
	}
	semantic, err := decodeP14ExistingRecordBackfillSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return nil, err
	}
	fixture := *receipt.Fixture
	if fixture.Isolation != semantic.FixtureIsolation ||
		fixture.ProjectBasisDigest !=
			semantic.SelectedProjectBasisDigest ||
		fixture.HomeTemplateDigest != semantic.HomeTemplateDigest ||
		fixture.ProjectBeforeDigest != fixture.ProjectBasisDigest ||
		fixture.HomeBeforeDigest != fixture.HomeTemplateDigest {
		return nil, fmt.Errorf("P14 backfill receipt basis differs")
	}
	if err := normalizeP14InstalledCLIExistingRecordBackfill(
		semantic,
		results,
		fixture.ProjectBeforeDigest,
		fixture.ProjectAfterDigest,
		fixture.HomeBeforeDigest,
		fixture.HomeAfterDigest,
	); err != nil {
		return nil, err
	}
	normalized := p14ExistingRecordBackfillNormalizedOutput{
		Schema:     p14ExistingRecordBackfillOutputSchema,
		ScenarioID: semantic.ScenarioID,
		Expected:   semantic.Expected,
	}
	return marshalP14CanonicalJSON(normalized)
}

func recomputeP14InstalledCLICodeExplore(
	scenario preparedP14Scenario,
	results []p14InstalledCLIProcessResult,
	receipt p14InstalledCLIExecutionReceipt,
) ([]byte, error) {
	if len(results) != 1 ||
		receipt.Fixture == nil ||
		len(receipt.Fixtures) != 0 {
		return nil, fmt.Errorf(
			"P14 code Explore receipt evidence count differs",
		)
	}
	if err := validateP14InstalledCLIReadOnlyFixture(
		*receipt.Fixture,
	); err != nil {
		return nil, err
	}
	semantic, err := decodeP14CodeExploreSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return nil, err
	}
	result := results[0]
	normalized, _, err := normalizeP14CodeExploreObservation(
		semantic,
		p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(result.Stdout),
			Stderr:   slices.Clone(result.Stderr),
			ExitCode: result.ExitCode,
		},
	)
	if err != nil {
		return nil, err
	}
	return marshalP14CanonicalJSON(normalized)
}

func p14InstalledCLIChecks(
	satisfied bool,
	ids ...string,
) []p14InstalledCLICheckReceipt {
	checks := make([]p14InstalledCLICheckReceipt, 0, len(ids))
	for _, id := range ids {
		checks = append(checks, p14InstalledCLICheckReceipt{
			ID:        id,
			Satisfied: satisfied,
		})
	}
	return checks
}

func TestP14InstalledCLINegativeMemoryScenariosBindBothNoWriteSnapshots(
	t *testing.T,
) {
	scenarios := []string{
		"invalid",
		"underdetermined",
		"authority_rejection",
	}
	for _, scenarioID := range scenarios {
		t.Run(scenarioID+"/project", func(t *testing.T) {
			semantic := p14MemoryOperationSemanticRequest{
				ScenarioID: scenarioID,
				Expected: p14MemoryOperationExpected{
					GraphRevisionDelta: 0,
				},
			}
			err := validateP14InstalledCLIMemoryOperationFixtureEffect(
				semantic,
				p14TestDigest("project-before"),
				p14TestDigest("project-after"),
				p14TestDigest("home"),
				p14TestDigest("home"),
			)
			if err == nil {
				t.Fatal(
					"P14 negative memory scenario accepted a project mutation",
				)
			}
		})
		t.Run(scenarioID+"/HOME", func(t *testing.T) {
			semantic := p14MemoryOperationSemanticRequest{
				ScenarioID: scenarioID,
				Expected: p14MemoryOperationExpected{
					GraphRevisionDelta: 0,
				},
			}
			err := validateP14InstalledCLIMemoryOperationFixtureEffect(
				semantic,
				p14TestDigest("project"),
				p14TestDigest("project"),
				p14TestDigest("home-before"),
				p14TestDigest("home-after"),
			)
			if err == nil {
				t.Fatal(
					"P14 negative memory scenario accepted a HOME mutation",
				)
			}
		})
	}
}

func TestP14InstalledCLIInitScopeChoiceBindsBothNoWriteSnapshots(
	t *testing.T,
) {
	semantic := p14InitMatrixSemanticCase{
		Outcome: "scope_choice_required",
	}
	stable := p14TestDigest("stable")
	if !p14InstalledCLIInitFixtureEffectMatches(
		semantic,
		stable,
		stable,
		stable,
		stable,
	) {
		t.Fatal("P14 init scope choice rejected unchanged fixture snapshots")
	}
	if p14InstalledCLIInitFixtureEffectMatches(
		semantic,
		p14TestDigest("project-before"),
		p14TestDigest("project-after"),
		stable,
		stable,
	) {
		t.Fatal("P14 init scope choice accepted a project mutation")
	}
	if p14InstalledCLIInitFixtureEffectMatches(
		semantic,
		stable,
		stable,
		p14TestDigest("home-before"),
		p14TestDigest("home-after"),
	) {
		t.Fatal("P14 init scope choice accepted a HOME mutation")
	}
}
