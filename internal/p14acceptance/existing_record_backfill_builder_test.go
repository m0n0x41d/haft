package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"testing"
)

const (
	p14ExistingRecordBackfillBuilderID         = "memory.existing-record-backfill.v1"
	p14ExistingRecordBackfillSemanticSchema    = "haft.p14.existing-record-backfill-semantic/v2"
	p14ExistingRecordBackfillCLISchema         = "haft.p14.existing-record-backfill-cli/v2"
	p14ExistingRecordBackfillOutputSchema      = "haft.p14.existing-record-backfill-output/v1"
	p14ExistingRecordBackfillLocalOracleSchema = "haft.p14.existing-record-backfill-local-oracle/v1"
	p14ExistingRecordBackfillNormalizationID   = "p14.existing-record-backfill.semantic-outcome.v1"
	p14ExistingRecordBackfillContractVersion   = "haft.memory.backfill.v1"
	p14ExistingRecordBackfillArtifactRef       = "prob-20260713-haft-v9-source-native-query-implementation-010656a8"
	p14ExistingRecordBackfillEntityKind        = "U.EntityRef"
	p14ExistingRecordBackfillEntityID          = "entity:haft-v9-typed-memory"
	p14ExistingRecordBackfillContext           = "haft-project"
)

var p14ExistingRecordBackfillOracleTests = []string{
	"github.com/m0n0x41d/haft/internal/cli::TestMemoryBackfillDryRunThenApplyUsesSourceOwnedProjection",
	"github.com/m0n0x41d/haft/internal/cli::TestMemoryBackfillInterfaceIsCLIOnlyAndNamesAuthorityBoundary",
	"github.com/m0n0x41d/haft/internal/p14acceptance::TestP14HistoricalBackfillPrettyJSONUsesDedicatedDocumentParser",
}

type p14ExistingRecordBackfillSemanticRequest struct {
	Schema                     string                                   `json:"schema"`
	ScenarioID                 string                                   `json:"scenario_id"`
	Protocol                   string                                   `json:"protocol"`
	FixtureIsolation           string                                   `json:"fixture_isolation"`
	SelectedProjectRoot        string                                   `json:"selected_project_root"`
	SelectedProjectBasisDigest string                                   `json:"selected_project_basis_digest"`
	HomeTemplateRoot           string                                   `json:"home_template_root"`
	HomeTemplateDigest         string                                   `json:"home_template_digest"`
	Calls                      []p14ExistingRecordBackfillSemanticCall  `json:"calls"`
	Expected                   p14ExistingRecordBackfillExpectedOutcome `json:"expected"`
}

type p14ExistingRecordBackfillSemanticCall struct {
	ID      string                           `json:"id"`
	Request p14ExistingRecordBackfillRequest `json:"request"`
}

type p14ExistingRecordBackfillRequest struct {
	ContractVersion      string                               `json:"contract_version"`
	Mode                 string                               `json:"mode"`
	RequestProvenanceRef string                               `json:"request_provenance_ref"`
	Items                []p14ExistingRecordBackfillSelection `json:"items"`
}

type p14ExistingRecordBackfillSelection struct {
	ArtifactRef      string                             `json:"artifact_ref"`
	EntityRef        p14ExistingRecordBackfillEntityRef `json:"entity_ref"`
	BoundedContextID string                             `json:"bounded_context_ref"`
}

type p14ExistingRecordBackfillEntityRef struct {
	RefKindID   string `json:"ref_kind_id"`
	ReferenceID string `json:"reference_id"`
}

type p14ExistingRecordBackfillExpectedOutcome struct {
	Calls                    []p14ExistingRecordBackfillExpectedCall `json:"calls"`
	CommitCount              int                                     `json:"commit_count"`
	GraphRevisionDelta       int64                                   `json:"graph_revision_delta"`
	SourceCarrierDisposition string                                  `json:"source_carrier_disposition"`
	AuthorityGranted         bool                                    `json:"authority_granted"`
}

type p14ExistingRecordBackfillExpectedCall struct {
	ID                 string `json:"id"`
	Result             string `json:"result"`
	GraphRevisionDelta int64  `json:"graph_revision_delta"`
	DurableChangeCount int    `json:"durable_change_count"`
}

type p14ExistingRecordBackfillCLISurface struct {
	Schema                     string                             `json:"schema"`
	SemanticRequestDigest      string                             `json:"semantic_request_digest"`
	FixtureIsolation           string                             `json:"fixture_isolation"`
	SelectedProjectRoot        string                             `json:"selected_project_root"`
	SelectedProjectBasisDigest string                             `json:"selected_project_basis_digest"`
	HomeTemplateRoot           string                             `json:"home_template_root"`
	HomeTemplateDigest         string                             `json:"home_template_digest"`
	Calls                      []p14ExistingRecordBackfillCLICall `json:"calls"`
}

type p14ExistingRecordBackfillCLICall struct {
	ID    string   `json:"id"`
	Argv  []string `json:"argv"`
	Stdin string   `json:"stdin"`
}

type p14ExistingRecordBackfillNormalizedOutput struct {
	Schema     string                                   `json:"schema"`
	ScenarioID string                                   `json:"scenario_id"`
	Expected   p14ExistingRecordBackfillExpectedOutcome `json:"expected"`
}

type p14ExistingRecordBackfillLocalOracle struct {
	Schema                string   `json:"schema"`
	SemanticRequestDigest string   `json:"semantic_request_digest"`
	ExpectedResultDigest  string   `json:"expected_result_digest"`
	LocalOracleTests      []string `json:"local_oracle_tests"`
}

func buildP14ExistingRecordBackfillScenario(
	declared scenarioContract,
	fixture p14MemoryOperationFixture,
) (preparedP14Scenario, error) {
	if err := validateP14ExistingRecordBackfillContract(declared); err != nil {
		return preparedP14Scenario{}, err
	}
	if err := validateP14MemoryOperationFixtureShape(fixture); err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := p14ExistingRecordBackfillSemanticRequest{
		Schema:                     p14ExistingRecordBackfillSemanticSchema,
		ScenarioID:                 declared.ID,
		Protocol:                   "dry_run_then_apply_then_idempotent_dry_run",
		FixtureIsolation:           p14MemoryOperationFixtureIsolation,
		SelectedProjectRoot:        fixture.SelectedProjectRoot,
		SelectedProjectBasisDigest: fixture.SelectedProjectBasisDigest,
		HomeTemplateRoot:           fixture.HomeTemplateRoot,
		HomeTemplateDigest:         fixture.HomeTemplateDigest,
		Calls: []p14ExistingRecordBackfillSemanticCall{
			p14ExistingRecordBackfillCall(
				"dry_run",
				"dry_run",
			),
			p14ExistingRecordBackfillCall(
				"apply",
				"apply",
			),
			p14ExistingRecordBackfillCall(
				"replay",
				"dry_run",
			),
		},
		Expected: p14ExistingRecordBackfillExpectedOutcome{
			Calls: []p14ExistingRecordBackfillExpectedCall{
				{
					ID:                 "dry_run",
					Result:             "validated_only",
					GraphRevisionDelta: 0,
					DurableChangeCount: 0,
				},
				{
					ID:                 "apply",
					Result:             "committed",
					GraphRevisionDelta: 1,
					DurableChangeCount: 2,
				},
				{
					ID:                 "replay",
					Result:             "already_projected",
					GraphRevisionDelta: 0,
					DurableChangeCount: 0,
				},
			},
			CommitCount:              1,
			GraphRevisionDelta:       1,
			SourceCarrierDisposition: "byte_identical_before_after",
			AuthorityGranted:         false,
		},
	}
	if err := validateP14ExistingRecordBackfillSemantic(semantic); err != nil {
		return preparedP14Scenario{}, err
	}
	semanticBytes, err := json.Marshal(semantic)
	if err != nil {
		return preparedP14Scenario{}, fmt.Errorf(
			"encode P14 existing-record backfill semantic request: %w",
			err,
		)
	}
	semanticDigest := p14Digest(semanticBytes)
	surface, err := buildP14ExistingRecordBackfillCLISurface(
		semantic,
		semanticDigest,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	surfaceBytes, err := json.Marshal(surface)
	if err != nil {
		return preparedP14Scenario{}, fmt.Errorf(
			"encode P14 existing-record backfill CLI surface: %w",
			err,
		)
	}
	normalized := p14ExistingRecordBackfillNormalizedOutput{
		Schema:     p14ExistingRecordBackfillOutputSchema,
		ScenarioID: semantic.ScenarioID,
		Expected:   semantic.Expected,
	}
	normalizedBytes, err := json.Marshal(normalized)
	if err != nil {
		return preparedP14Scenario{}, fmt.Errorf(
			"encode P14 existing-record backfill normalized output: %w",
			err,
		)
	}
	expectedResultDigest := p14Digest(normalizedBytes)
	localOracle := p14ExistingRecordBackfillLocalOracle{
		Schema:                p14ExistingRecordBackfillLocalOracleSchema,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  expectedResultDigest,
		LocalOracleTests:      slices.Clone(declared.LocalOracleTests),
	}
	localOracleBytes, err := json.Marshal(localOracle)
	if err != nil {
		return preparedP14Scenario{}, fmt.Errorf(
			"encode P14 existing-record backfill local oracle: %w",
			err,
		)
	}
	scenario := preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests: []preparedP14Request{{
			Surface:               "installed_cli",
			Builder:               declared.RequestBuilder,
			Encoding:              "argv_json",
			CanonicalPayload:      string(surfaceBytes),
			PayloadDigest:         p14Digest(surfaceBytes),
			SemanticRequestDigest: semanticDigest,
		}},
		Oracle: preparedP14Oracle{
			Kind:                    declared.OracleKind,
			NormalizationID:         p14ExistingRecordBackfillNormalizationID,
			ExpectedResultDigest:    expectedResultDigest,
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}
	if err := validateP14ExistingRecordBackfillPreparedScenario(
		declared,
		scenario,
	); err != nil {
		return preparedP14Scenario{}, err
	}
	return scenario, nil
}

func p14ExistingRecordBackfillCall(
	id string,
	mode string,
) p14ExistingRecordBackfillSemanticCall {
	return p14ExistingRecordBackfillSemanticCall{
		ID: id,
		Request: p14ExistingRecordBackfillRequest{
			ContractVersion: p14ExistingRecordBackfillContractVersion,
			Mode:            mode,
			RequestProvenanceRef: "p14:existing-record-backfill:" +
				id,
			Items: []p14ExistingRecordBackfillSelection{{
				ArtifactRef: p14ExistingRecordBackfillArtifactRef,
				EntityRef: p14ExistingRecordBackfillEntityRef{
					RefKindID:   p14ExistingRecordBackfillEntityKind,
					ReferenceID: p14ExistingRecordBackfillEntityID,
				},
				BoundedContextID: p14ExistingRecordBackfillContext,
			}},
		},
	}
}

func buildP14ExistingRecordBackfillCLISurface(
	semantic p14ExistingRecordBackfillSemanticRequest,
	semanticDigest string,
) (p14ExistingRecordBackfillCLISurface, error) {
	calls := make(
		[]p14ExistingRecordBackfillCLICall,
		0,
		len(semantic.Calls),
	)
	for _, semanticCall := range semantic.Calls {
		requestBytes, err := json.Marshal(semanticCall.Request)
		if err != nil {
			return p14ExistingRecordBackfillCLISurface{}, fmt.Errorf(
				"encode P14 existing-record backfill request %s: %w",
				semanticCall.ID,
				err,
			)
		}
		calls = append(calls, p14ExistingRecordBackfillCLICall{
			ID: semanticCall.ID,
			Argv: []string{
				"memory",
				"backfill",
				"--input-file",
				"-",
			},
			Stdin: string(requestBytes),
		})
	}
	return p14ExistingRecordBackfillCLISurface{
		Schema:                     p14ExistingRecordBackfillCLISchema,
		SemanticRequestDigest:      semanticDigest,
		FixtureIsolation:           semantic.FixtureIsolation,
		SelectedProjectRoot:        semantic.SelectedProjectRoot,
		SelectedProjectBasisDigest: semantic.SelectedProjectBasisDigest,
		HomeTemplateRoot:           semantic.HomeTemplateRoot,
		HomeTemplateDigest:         semantic.HomeTemplateDigest,
		Calls:                      calls,
	}, nil
}

func validateP14ExistingRecordBackfillPreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	if err := validateP14ExistingRecordBackfillContract(declared); err != nil {
		return err
	}
	semantic, err := decodeP14ExistingRecordBackfillSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	if err := validateP14ExistingRecordBackfillSemantic(semantic); err != nil {
		return err
	}
	semanticBytes, err := json.Marshal(semantic)
	if err != nil {
		return err
	}
	if !bytes.Equal(
		semanticBytes,
		[]byte(scenario.SemanticRequestCanonical),
	) ||
		p14Digest(semanticBytes) != scenario.SemanticRequestDigest ||
		len(scenario.Requests) != 1 {
		return fmt.Errorf(
			"P14 existing-record backfill semantic request differs",
		)
	}
	request := scenario.Requests[0]
	if request.Surface != "installed_cli" ||
		request.Builder != p14ExistingRecordBackfillBuilderID ||
		request.Encoding != "argv_json" ||
		request.SemanticRequestDigest != scenario.SemanticRequestDigest {
		return fmt.Errorf(
			"P14 existing-record backfill CLI request identity differs",
		)
	}
	surface, err := decodeP14ExistingRecordBackfillCLISurface(
		[]byte(request.CanonicalPayload),
	)
	if err != nil {
		return err
	}
	expectedSurface, err := buildP14ExistingRecordBackfillCLISurface(
		semantic,
		scenario.SemanticRequestDigest,
	)
	if err != nil {
		return err
	}
	expectedSurfaceBytes, err := json.Marshal(expectedSurface)
	if err != nil {
		return err
	}
	if !bytes.Equal(
		expectedSurfaceBytes,
		[]byte(request.CanonicalPayload),
	) ||
		p14Digest(expectedSurfaceBytes) != request.PayloadDigest ||
		surface.Schema != expectedSurface.Schema {
		return fmt.Errorf(
			"P14 existing-record backfill CLI surface differs",
		)
	}
	normalized := p14ExistingRecordBackfillNormalizedOutput{
		Schema:     p14ExistingRecordBackfillOutputSchema,
		ScenarioID: semantic.ScenarioID,
		Expected:   semantic.Expected,
	}
	normalizedBytes, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	localOracle := p14ExistingRecordBackfillLocalOracle{
		Schema:                p14ExistingRecordBackfillLocalOracleSchema,
		SemanticRequestDigest: scenario.SemanticRequestDigest,
		ExpectedResultDigest:  p14Digest(normalizedBytes),
		LocalOracleTests:      slices.Clone(declared.LocalOracleTests),
	}
	localOracleBytes, err := json.Marshal(localOracle)
	if err != nil {
		return err
	}
	if scenario.Oracle.Kind != "normalized_digest" ||
		scenario.Oracle.NormalizationID !=
			p14ExistingRecordBackfillNormalizationID ||
		scenario.Oracle.ExpectedResultDigest !=
			p14Digest(normalizedBytes) ||
		scenario.Oracle.ExpectedEffect !=
			"fixture_semantic_write" ||
		scenario.Oracle.LocalOracleOutputDigest !=
			p14Digest(localOracleBytes) {
		return fmt.Errorf(
			"P14 existing-record backfill oracle differs",
		)
	}
	return nil
}

func validateP14ExistingRecordBackfillContract(
	declared scenarioContract,
) error {
	if declared.ID != "existing_record_backfill" ||
		declared.Family != declared.ID ||
		declared.RequestBuilder !=
			p14ExistingRecordBackfillBuilderID ||
		!slices.Equal(
			declared.Surfaces,
			[]string{"installed_cli"},
		) ||
		declared.OracleKind != "normalized_digest" ||
		declared.ExpectedEffect != "fixture_semantic_write" ||
		!slices.Equal(
			declared.RequiredBindings,
			[]string{
				"candidate_basis",
				"p13_evidence",
				"selected_project_basis",
				"golden_memory_fixture",
			},
		) ||
		!slices.Equal(
			declared.LocalOracleTests,
			p14ExistingRecordBackfillOracleTests,
		) {
		return fmt.Errorf(
			"P14 existing-record backfill contract differs",
		)
	}
	return nil
}

func validateP14ExistingRecordBackfillSemantic(
	semantic p14ExistingRecordBackfillSemanticRequest,
) error {
	if semantic.Schema != p14ExistingRecordBackfillSemanticSchema ||
		semantic.ScenarioID != "existing_record_backfill" ||
		semantic.Protocol !=
			"dry_run_then_apply_then_idempotent_dry_run" ||
		semantic.FixtureIsolation !=
			p14MemoryOperationFixtureIsolation ||
		!filepath.IsAbs(semantic.SelectedProjectRoot) ||
		!validP14Digest(semantic.SelectedProjectBasisDigest) ||
		!filepath.IsAbs(semantic.HomeTemplateRoot) ||
		!validP14Digest(semantic.HomeTemplateDigest) ||
		semantic.SelectedProjectRoot == semantic.HomeTemplateRoot ||
		len(semantic.Calls) != 3 {
		return fmt.Errorf(
			"P14 existing-record backfill semantic shape differs",
		)
	}
	expectedIDs := []string{"dry_run", "apply", "replay"}
	expectedModes := []string{"dry_run", "apply", "dry_run"}
	for index, call := range semantic.Calls {
		if call.ID != expectedIDs[index] ||
			call.Request.Mode != expectedModes[index] ||
			call.Request.ContractVersion !=
				p14ExistingRecordBackfillContractVersion ||
			call.Request.RequestProvenanceRef !=
				"p14:existing-record-backfill:"+call.ID ||
			len(call.Request.Items) != 1 {
			return fmt.Errorf(
				"P14 existing-record backfill call %d differs",
				index,
			)
		}
		item := call.Request.Items[0]
		if item.ArtifactRef !=
			p14ExistingRecordBackfillArtifactRef ||
			item.EntityRef.RefKindID !=
				p14ExistingRecordBackfillEntityKind ||
			item.EntityRef.ReferenceID !=
				p14ExistingRecordBackfillEntityID ||
			item.BoundedContextID !=
				p14ExistingRecordBackfillContext {
			return fmt.Errorf(
				"P14 existing-record backfill selection differs",
			)
		}
	}
	expected := p14ExistingRecordBackfillExpectedOutcome{
		Calls: []p14ExistingRecordBackfillExpectedCall{
			{
				ID:                 "dry_run",
				Result:             "validated_only",
				GraphRevisionDelta: 0,
				DurableChangeCount: 0,
			},
			{
				ID:                 "apply",
				Result:             "committed",
				GraphRevisionDelta: 1,
				DurableChangeCount: 2,
			},
			{
				ID:                 "replay",
				Result:             "already_projected",
				GraphRevisionDelta: 0,
				DurableChangeCount: 0,
			},
		},
		CommitCount:              1,
		GraphRevisionDelta:       1,
		SourceCarrierDisposition: "byte_identical_before_after",
		AuthorityGranted:         false,
	}
	expectedBytes, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	observedBytes, err := json.Marshal(semantic.Expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedBytes, observedBytes) {
		return fmt.Errorf(
			"P14 existing-record backfill expected outcome differs",
		)
	}
	return nil
}

func decodeP14ExistingRecordBackfillSemantic(
	raw []byte,
) (p14ExistingRecordBackfillSemanticRequest, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	semantic := p14ExistingRecordBackfillSemanticRequest{}
	if err := decoder.Decode(&semantic); err != nil {
		return p14ExistingRecordBackfillSemanticRequest{}, fmt.Errorf(
			"decode P14 existing-record backfill semantic request: %w",
			err,
		)
	}
	if err := requireP14ExistingRecordBackfillEOF(decoder); err != nil {
		return p14ExistingRecordBackfillSemanticRequest{}, err
	}
	return semantic, nil
}

func decodeP14ExistingRecordBackfillCLISurface(
	raw []byte,
) (p14ExistingRecordBackfillCLISurface, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	surface := p14ExistingRecordBackfillCLISurface{}
	if err := decoder.Decode(&surface); err != nil {
		return p14ExistingRecordBackfillCLISurface{}, fmt.Errorf(
			"decode P14 existing-record backfill CLI surface: %w",
			err,
		)
	}
	if err := requireP14ExistingRecordBackfillEOF(decoder); err != nil {
		return p14ExistingRecordBackfillCLISurface{}, err
	}
	return surface, nil
}

func requireP14ExistingRecordBackfillEOF(
	decoder *json.Decoder,
) error {
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf(
		"P14 existing-record backfill carrier has trailing JSON",
	)
}

func TestP14ExistingRecordBackfillBuilderClosesCLIOnlyDryRunApplyReplay(
	t *testing.T,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := findP14ScenarioContractByBuilder(
		contract,
		p14ExistingRecordBackfillBuilderID,
	)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := buildP14ExistingRecordBackfillScenario(
		declared,
		syntheticP14MemoryReadFixture().Operations,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14ExistingRecordBackfillPreparedScenario(
		declared,
		scenario,
	); err != nil {
		t.Fatal(err)
	}
	if len(scenario.Requests) != 1 ||
		scenario.Requests[0].Surface != "installed_cli" {
		t.Fatal(
			"P14 existing-record backfill exposed a non-CLI surface",
		)
	}
	semantic, err := decodeP14ExistingRecordBackfillSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		t.Fatal(err)
	}
	semantic.Calls[0].Request.Mode = "apply"
	tamperedBytes, err := json.Marshal(semantic)
	if err != nil {
		t.Fatal(err)
	}
	tampered := scenario
	tampered.SemanticRequestCanonical = string(tamperedBytes)
	tampered.SemanticRequestDigest = p14Digest(tamperedBytes)
	if err := validateP14ExistingRecordBackfillPreparedScenario(
		declared,
		tampered,
	); err == nil {
		t.Fatal(
			"P14 existing-record backfill accepted a dry-run/apply substitution",
		)
	}
}
