package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
	"github.com/spf13/cobra"
)

func TestMemoryValidationCLIAndMCPShareStableReadOnlyPresentation(t *testing.T) {
	runtime, err := newReadOnlyMemoryValidation(context.Background())
	if err != nil {
		t.Fatalf("newReadOnlyMemoryValidation() error = %v", err)
	}
	payload := bundledMemoryValidationFixture()

	direct, err := runtime.Validate(payload)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	repeated, err := runtime.Validate(payload)
	if err != nil {
		t.Fatalf("repeated Validate() error = %v", err)
	}
	if !bytes.Equal(direct, repeated) {
		t.Fatalf("validation output is not stable\nfirst:  %s\nsecond: %s", direct, repeated)
	}

	mcp, err := runtime.MCPHandler()(context.Background(), json.RawMessage(payload))
	if err != nil {
		t.Fatalf("MCPHandler() error = %v", err)
	}
	if !bytes.Equal([]byte(mcp), direct) {
		t.Fatalf("MCP presentation differs from core presentation\nMCP:  %s\ncore: %s", mcp, direct)
	}

	input := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(input, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	cli := runMemoryValidateForTest(t, input, nil)
	if !bytes.Equal(cli, direct) {
		t.Fatalf("CLI presentation differs from MCP presentation\nCLI: %s\nMCP: %s", cli, direct)
	}

	response := struct {
		ContractVersion string `json:"contract_version"`
		Action          string `json:"action"`
		Verdict         string `json:"verdict"`
		Basis           struct {
			RequestedKind  string  `json:"requested_kind"`
			ResolutionKind string  `json:"resolution_kind"`
			GraphRevision  *uint64 `json:"graph_revision"`
		} `json:"basis"`
		Persistence struct {
			Mode             string `json:"mode"`
			RowsWritten      uint64 `json:"rows_written"`
			AuthorityGranted bool   `json:"authority_granted"`
		} `json:"persistence_disposition"`
		NormalizedDigest string `json:"normalized_digest"`
	}{}
	if err := json.Unmarshal(direct, &response); err != nil {
		t.Fatalf("unmarshal validation response: %v\n%s", err, direct)
	}
	if response.ContractVersion != typedmemorywire.ContractVersion {
		t.Fatalf("contract_version = %q", response.ContractVersion)
	}
	if response.Action != typedmemorywire.ActionValidate {
		t.Fatalf("action = %q", response.Action)
	}
	if response.Verdict != "underdetermined" {
		t.Fatalf("verdict = %q, want underdetermined", response.Verdict)
	}
	if response.Basis.RequestedKind != string(typedmemorywire.BasisBundledCandidateOpenWorld) {
		t.Fatalf("requested basis = %q", response.Basis.RequestedKind)
	}
	if response.Basis.ResolutionKind != "bundled_candidate_open_world" {
		t.Fatalf("resolution basis = %q", response.Basis.ResolutionKind)
	}
	if response.Basis.GraphRevision != nil {
		t.Fatalf("bundled candidate fabricated graph revision %d", *response.Basis.GraphRevision)
	}
	if response.Persistence.Mode != "validation_only_no_write" ||
		response.Persistence.RowsWritten != 0 ||
		response.Persistence.AuthorityGranted {
		t.Fatalf("persistence disposition = %#v", response.Persistence)
	}
	if response.NormalizedDigest != "" {
		t.Fatalf("bundled candidate exposed normalized digest %q", response.NormalizedDigest)
	}
}

func TestMemoryValidateReadsStdinBytesStrictly(t *testing.T) {
	payload := bundledMemoryValidationFixture()
	fromStdin := runMemoryValidateForTest(t, "-", bytes.NewReader(payload))

	input := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(input, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := runMemoryValidateForTest(t, input, nil)
	if !bytes.Equal(fromStdin, fromFile) {
		t.Fatalf("stdin and file results differ\nstdin: %s\nfile:  %s", fromStdin, fromFile)
	}

	duplicate := bytes.Replace(
		payload,
		[]byte(`"action":"validate"`),
		[]byte(`"action":"validate","action":"validate"`),
		1,
	)
	runtime, err := newReadOnlyMemoryValidation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.Validate(duplicate)
	var decodeError *typedmemorywire.DecodeError
	if !errors.As(err, &decodeError) {
		t.Fatalf("duplicate field error = %T %v, want DecodeError", err, err)
	}
	if decodeError.Code() != typedmemorywire.ErrorInvalidContract {
		t.Fatalf("duplicate field code = %q", decodeError.Code())
	}

	previousInputFile := memoryValidateInputFile
	t.Cleanup(func() { memoryValidateInputFile = previousInputFile })
	memoryValidateInputFile = "-"
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetIn(bytes.NewReader(duplicate))
	output := bytes.Buffer{}
	command.SetOut(&output)
	err = runMemoryValidate(command, nil)
	if !errors.As(err, &decodeError) {
		t.Fatalf("CLI duplicate field error = %T %v, want DecodeError", err, err)
	}
	if output.Len() != 0 {
		t.Fatalf("malformed CLI input produced semantic output: %s", output.Bytes())
	}
}

func TestMemoryValidationProductionResolverNeverFabricatesProjectBasis(t *testing.T) {
	runtime, err := newReadOnlyMemoryValidation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name          string
		basis         string
		requestedKind string
	}{
		{
			name:          "current project",
			basis:         `{"kind":"project_current"}`,
			requestedKind: "project_current",
		},
		{
			name:          "exact project",
			basis:         `{"kind":"exact_project","type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","graph_revision":17}`,
			requestedKind: "exact_project",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			payload := bytes.Replace(
				bundledMemoryValidationFixture(),
				[]byte(`{"kind":"bundled_candidate_open_world"}`),
				[]byte(testCase.basis),
				1,
			)
			result, err := runtime.Validate(payload)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			response := struct {
				Verdict string `json:"verdict"`
				Basis   struct {
					RequestedKind  string  `json:"requested_kind"`
					ResolutionKind string  `json:"resolution_kind"`
					TypeEnvRef     string  `json:"type_env_ref"`
					GraphRevision  *uint64 `json:"graph_revision"`
				} `json:"basis"`
				Diagnostics []struct {
					Code string `json:"code"`
				} `json:"diagnostics"`
			}{}
			if err := json.Unmarshal(result, &response); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if response.Verdict != "underdetermined" {
				t.Fatalf("verdict = %q", response.Verdict)
			}
			if response.Basis.RequestedKind != testCase.requestedKind ||
				response.Basis.ResolutionKind != "project_basis_unavailable" {
				t.Fatalf("basis = %#v", response.Basis)
			}
			if response.Basis.TypeEnvRef != "" || response.Basis.GraphRevision != nil {
				t.Fatalf("production resolver fabricated project basis %#v", response.Basis)
			}
			codes := make([]string, 0, len(response.Diagnostics))
			for _, diagnostic := range response.Diagnostics {
				codes = append(codes, diagnostic.Code)
			}
			wantCodes := []string{
				"project_active_typeenv_unavailable",
				"project_snapshot_unavailable",
			}
			if !slices.Equal(codes, wantCodes) {
				t.Fatalf("diagnostic codes = %v, want %v", codes, wantCodes)
			}
		})
	}
}

func TestPublicMemoryValidateRoutesProjectBasisThroughCheckedReadSession(
	t *testing.T,
) {
	cases := []struct {
		name  string
		basis []byte
	}{
		{
			name:  "current project",
			basis: []byte(`{"kind":"project_current"}`),
		},
		{
			name: "exact project",
			basis: []byte(
				`{"kind":"exact_project","type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","graph_revision":17}`,
			),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			want := []byte(
				`{"contract_version":"haft.memory.v1","action":"validate","verdict":"underdetermined"}` + "\n",
			)
			session := &fixedProjectMemoryReadSession{result: want}
			previousOpener := openProjectMemoryReadSession
			openProjectMemoryReadSession = func(
				context.Context,
			) (projectMemoryReadSession, error) {
				return session, nil
			}
			t.Cleanup(func() {
				openProjectMemoryReadSession = previousOpener
			})
			payload := bytes.Replace(
				bundledMemoryValidationFixture(),
				[]byte(`{"kind":"bundled_candidate_open_world"}`),
				testCase.basis,
				1,
			)
			request, err := typedmemorywire.DecodeValidateRequest(payload)
			if err != nil {
				t.Fatalf("DecodeValidateRequest() error = %v", err)
			}

			result, err := validateMemoryRequest(
				context.Background(),
				request,
			)
			if err != nil {
				t.Fatalf("validateMemoryRequest() error = %v", err)
			}
			if !bytes.Equal(result, want) {
				t.Fatalf("validate result = %q, want %q", result, want)
			}
			if session.calls != 1 ||
				session.action != typedmemorywire.ActionValidate ||
				!session.closed {
				t.Fatalf("project validation session = %#v", session)
			}
		})
	}
}

func TestMemoryValidateCallDoesNotMutateProjectTree(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectDB := filepath.Join(haftDir, "haft.db")
	if err := os.WriteFile(projectDB, []byte("project-db-sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "request.json")
	if err := os.WriteFile(input, bundledMemoryValidationFixture(), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotMemoryValidationTree(t, root)

	t.Chdir(root)
	_ = runMemoryValidateForTest(t, input, nil)

	after := snapshotMemoryValidationTree(t, root)
	if !slices.Equal(before, after) {
		t.Fatalf("memory validate changed project tree\nbefore=%v\nafter=%v", before, after)
	}
}

func TestConfigureMemoryValidationAdvertisesDedicatedToolWithoutProjectBinding(t *testing.T) {
	server := fpf.NewServer("test")
	if err := configureMemoryValidation(context.Background(), server); err != nil {
		t.Fatalf("configureMemoryValidation() error = %v", err)
	}
	for _, tool := range server.ToolCatalog() {
		if tool.Name == "haft_memory" {
			return
		}
	}
	t.Fatal("configured server did not advertise haft_memory")
}

func TestPreProjectMemoryHandlersReturnTypedRecovery(t *testing.T) {
	runtime, err := newReadOnlyMemoryValidation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	admitResult, err := runtime.PreProjectMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryAdmissionTestPayload),
	)
	if err != nil {
		t.Fatalf("pre-project admit recovery: %v", err)
	}
	for _, expected := range []string{
		`"contract_version":"haft.memory.v2"`,
		`"action":"admit"`,
		`"result_kind":"project_basis_unavailable"`,
		`"performed":false`,
		`"tool":"haft_onboard"`,
	} {
		if !strings.Contains(admitResult, expected) {
			t.Fatalf(
				"pre-project admit recovery lacks %s: %s",
				expected,
				admitResult,
			)
		}
	}

	readResult, err := newProjectMemoryUnavailableReadMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryFullResolveQueryPayload()),
	)
	if err != nil {
		t.Fatalf("pre-project read recovery: %v", err)
	}
	for _, expected := range []string{
		`"contract_version":"haft.memory.v1"`,
		`"action":"memory"`,
		`"mode":"resolve"`,
		`"result_kind":"project_basis_unavailable"`,
		`"performed":false`,
		`"tool":"haft_onboard"`,
	} {
		if !strings.Contains(readResult, expected) {
			t.Fatalf(
				"pre-project read recovery lacks %s: %s",
				expected,
				readResult,
			)
		}
	}
	for _, output := range []string{admitResult, readResult} {
		lower := strings.ToLower(output)
		for _, forbidden := range []string{
			"typeenv",
			"memorychangeset",
			"graph_revision",
		} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("typed recovery leaked %q: %s", forbidden, output)
			}
		}
	}
}

func TestReadOnlyMemoryValidationSurfaceRejectsAdmissionWhileCLIRegistersIt(
	t *testing.T,
) {
	registered := false
	for _, command := range memoryCmd.Commands() {
		if command.Name() == memoryAdmitCmd.Name() {
			registered = true
		}
	}
	if !registered {
		t.Fatal("memory admit command is not publicly registered")
	}
	runtime, err := newReadOnlyMemoryValidation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.MCPHandler()(
		context.Background(),
		json.RawMessage(`{
  "contract_version":"haft.memory.v1",
  "action":"admit",
  "basis":{
    "kind":"exact_project",
    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "graph_revision":0
  },
  "authority_class":"non_binding_semantic_assertion",
  "idempotency_key":"sealed-public-admit",
  "request_provenance_ref":"provenance:sealed-public-admit",
  "change_set":{"changes":[{
    "kind":"declare_entity",
    "entity_id":"entity:sealed-public-admit",
    "local_ref":"local:sealed-public-admit",
    "context":"haft-project",
    "label":"sealed public admit",
    "provenance":"provenance:sealed-public-admit-change"
  }]}
}`),
	)
	if err == nil {
		t.Fatal("read-only public MCP handler accepted admission action")
	}
	if result != "" {
		t.Fatalf("rejected public admission produced result %q", result)
	}
}

func TestMemoryValidateInterfaceContractNamesReadOnlyBoundary(t *testing.T) {
	capability, found := findInterfaceCapability(haftInterfaceCatalog(), "memory.validate")
	if !found {
		t.Fatal("memory.validate interface capability missing")
	}
	if capability.CurrentExecution.MCPTool != "haft_memory" ||
		capability.CurrentExecution.MCPAction != "validate" {
		t.Fatalf("memory.validate execution = %#v", capability.CurrentExecution)
	}
	if capability.CurrentExecution.CLICommand != "haft memory validate --input-file request.json" {
		t.Fatalf("memory.validate CLI = %q", capability.CurrentExecution.CLICommand)
	}
	wantRequired := []string{"action", "basis", "change_set", "contract_version"}
	gotRequired := append([]string(nil), capability.InputContract.RequiredFields...)
	slices.Sort(gotRequired)
	if !slices.Equal(gotRequired, wantRequired) {
		t.Fatalf("memory.validate required fields = %v", gotRequired)
	}

	contractText := []string{capability.Purpose}
	contractText = append(contractText, capability.InputContract.Notes...)
	contractText = append(contractText, capability.Invariants...)
	for _, shape := range capability.InputContract.FieldShapes {
		contractText = append(contractText, shape.Shape, shape.Note)
	}
	joined := strings.Join(contractText, "\n")
	for _, expected := range []string{
		`"verdict":"valid|invalid|underdetermined"`,
		"validation_only_no_write",
		"bundled open-world candidate cannot produce project Valid",
		"writes zero project rows",
		"same decoder, service, and stable JSON presenter",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("memory.validate contract misses %q\n%s", expected, joined)
		}
	}

	report := buildInterfaceContractAuditReport(haftInterfaceCatalog())
	var surface interfaceContractAuditSurface
	for _, candidate := range report.Surfaces {
		if candidate.CapabilityID == "memory.validate" {
			surface = candidate
			break
		}
	}
	if surface.CapabilityID == "" {
		t.Fatal("memory.validate missing from interface contract audit")
	}
	if surface.AuthorityPosture != "read_only_validation_no_persistence" {
		t.Fatalf("memory.validate authority posture = %q", surface.AuthorityPosture)
	}
	if surface.SchemaCoverage.Status != "covered" ||
		surface.ShapeCoverage.Status != "covered" {
		t.Fatalf(
			"memory.validate schema/shape coverage = %q/%q",
			surface.SchemaCoverage.Status,
			surface.ShapeCoverage.Status,
		)
	}
}

func TestMemoryValidateCommandIsRegisteredWithRequiredInputFileFlag(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"memory", "validate"})
	if err != nil {
		t.Fatalf("find memory validate command: %v", err)
	}
	if command != memoryValidateCmd {
		t.Fatalf("found command = %q", command.CommandPath())
	}
	flag := command.Flags().Lookup("input-file")
	if flag == nil {
		t.Fatal("memory validate --input-file flag missing")
	}
	if flag.DefValue != "" {
		t.Fatalf("memory validate --input-file default = %q, want no implicit input", flag.DefValue)
	}

	previousInputFile := memoryValidateInputFile
	t.Cleanup(func() { memoryValidateInputFile = previousInputFile })
	memoryValidateInputFile = ""
	emptyCommand := &cobra.Command{}
	emptyCommand.SetContext(context.Background())
	err = runMemoryValidate(emptyCommand, nil)
	if err == nil || !strings.Contains(err.Error(), "--input-file is required") {
		t.Fatalf("missing --input-file error = %v", err)
	}
}

func runMemoryValidateForTest(
	t *testing.T,
	inputFile string,
	stdin *bytes.Reader,
) []byte {
	t.Helper()
	previousInputFile := memoryValidateInputFile
	t.Cleanup(func() { memoryValidateInputFile = previousInputFile })
	memoryValidateInputFile = inputFile

	command := &cobra.Command{}
	command.SetContext(context.Background())
	if stdin != nil {
		command.SetIn(stdin)
	}
	output := bytes.Buffer{}
	command.SetOut(&output)
	if err := runMemoryValidate(command, nil); err != nil {
		t.Fatalf("runMemoryValidate() error = %v", err)
	}
	return append([]byte(nil), output.Bytes()...)
}

func snapshotMemoryValidationTree(t *testing.T, root string) []string {
	t.Helper()
	var snapshot []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			snapshot = append(snapshot, "dir:"+relative)
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		snapshot = append(snapshot, "file:"+relative+":"+hex.EncodeToString(digest[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot tree: %v", err)
	}
	slices.Sort(snapshot)
	return snapshot
}
func bundledMemoryValidationFixture() []byte {
	return []byte(`{"contract_version":"haft.memory.v1","action":"validate","basis":{"kind":"bundled_candidate_open_world"},"change_set":{"changes":[{"kind":"declare_entity","entity_id":"entity:cli-fixture","local_ref":"local:cli-fixture","context":"context:cli-fixture","label":"CLI fixture entity","provenance":"provenance:cli-fixture"}]}}`)
}
