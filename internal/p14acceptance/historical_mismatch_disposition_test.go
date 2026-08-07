package p14acceptance

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	p14HistoricalMismatchEvidenceSchema = "haft.p14.historical-mismatch-evidence/v1"
	p14HistoricalMismatchEvidencePath   = "internal/p14acceptance/testdata/historical_mismatch_evidence_v1.json"
	p14HistoricalMismatchEvidenceDigest = "sha256:16b471c386405b0b022338c39c68b542f24552871ae9120fa6bee2f62442300b"

	p14HistoricalMismatchExtractSchema           = "haft.p14.historical-mismatch-source-extract/v1"
	p14HistoricalMismatchExtractPayloadSchema    = "haft.p14.historical-mismatch-source-extract-payload/v1"
	p14HistoricalMismatchExtractPath             = "internal/p14acceptance/testdata/historical_mismatch_source_extract_v1.json"
	p14HistoricalMismatchExtractDigest           = "sha256:1bcf286dc2f78ff7c121bdc54a33ba267446a8f31123401ad1ed7a0ba614fd65"
	p14HistoricalMismatchExtractPayloadDigest    = "sha256:b83c8581d0605af0355ac6c8737c09ae8d97a627f642704efd7b95e223f3809c"
	p14HistoricalMismatchExtractCompressedDigest = "sha256:753ca7a6ec8e6d6fc2f200d1a6e39d86fb1d8433daf603fafd50c96d1bc649a2"
	p14HistoricalMismatchExtractEncoding         = "gzip+base64"
	p14HistoricalMismatchExtractLimit            = 1 << 20
)

type p14HistoricalMismatchDisposition uint8

const (
	p14HistoricalMismatchDispositionUnknown p14HistoricalMismatchDisposition = iota
	p14HistoricalMismatchDispositionDefect
	p14HistoricalMismatchDispositionOracle
	p14HistoricalMismatchDispositionStale
)

type p14HistoricalMismatchEvidenceCarrier struct {
	Schema   string                                `json:"schema"`
	Source   p14HistoricalMismatchEvidenceSource   `json:"source"`
	Families []p14HistoricalMismatchFamilyEvidence `json:"families"`
}

type p14HistoricalMismatchEvidenceSource struct {
	CapturePath               string                   `json:"capture_path"`
	CaptureFileDigest         string                   `json:"capture_file_digest"`
	DeclaredCaptureDigest     string                   `json:"declared_capture_digest"`
	PreparedPath              string                   `json:"prepared_path"`
	PreparedFileDigest        string                   `json:"prepared_file_digest"`
	PreparationDigest         string                   `json:"preparation_digest"`
	MemoryFixtureDeclaredPath string                   `json:"memory_fixture_declared_path"`
	MemoryFixtureArchivePath  string                   `json:"memory_fixture_archive_path"`
	MemoryFixtureDigest       string                   `json:"memory_fixture_digest"`
	CandidateDigest           string                   `json:"candidate_digest"`
	KnownEntity               p14HistoricalKnownEntity `json:"known_entity"`
}

type p14HistoricalKnownEntity struct {
	ReferenceID       string `json:"reference_id"`
	BoundedContextRef string `json:"bounded_context_ref"`
}

type p14HistoricalMismatchSourceExtractCarrier struct {
	Schema                  string                              `json:"schema"`
	Source                  p14HistoricalMismatchEvidenceSource `json:"source"`
	PayloadEncoding         string                              `json:"payload_encoding"`
	PayloadDigest           string                              `json:"payload_digest"`
	CompressedPayloadDigest string                              `json:"compressed_payload_digest"`
	PayloadBase64           []string                            `json:"payload_base64"`
}

type p14HistoricalMismatchSourceExtractPayload struct {
	Schema            string            `json:"schema"`
	CaptureHeader     json.RawMessage   `json:"capture_header"`
	ScenarioCaptures  []json.RawMessage `json:"scenario_captures"`
	PreparedHeader    json.RawMessage   `json:"prepared_header"`
	PreparedScenarios []json.RawMessage `json:"prepared_scenarios"`
	MemoryFixture     json.RawMessage   `json:"memory_fixture"`
}

type p14HistoricalMismatchCaptureHeader struct {
	Schema        string `json:"schema"`
	CarrierPath   string `json:"carrier_path"`
	CaptureDigest string `json:"capture_digest"`
	Status        string `json:"status"`
	Capture       struct {
		Schema                    string                        `json:"schema"`
		Status                    string                        `json:"status"`
		CapturedAt                string                        `json:"captured_at"`
		InstalledExecutablePath   string                        `json:"installed_executable_path"`
		InstalledExecutableDigest string                        `json:"installed_executable_digest"`
		PreparedCarrier           p14PreparedObservationBinding `json:"prepared_carrier"`
		ReleaseClaim              bool                          `json:"release_claim"`
		ResultSemantics           string                        `json:"result_semantics"`
	} `json:"capture"`
}

type p14HistoricalMismatchPreparedHeader struct {
	Schema            string `json:"schema"`
	CarrierPath       string `json:"carrier_path"`
	PreparationDigest string `json:"preparation_digest"`
	Status            string `json:"status"`
	Preparation       struct {
		Schema          string `json:"schema"`
		Status          string `json:"status"`
		ContractRef     string `json:"contract_ref"`
		ContractDigest  string `json:"contract_digest"`
		ReleaseClaim    bool   `json:"release_claim"`
		ResultSemantics string `json:"result_semantics"`
	} `json:"preparation"`
}

type p14HistoricalMismatchPreparedScenario struct {
	ID                       string                                 `json:"id"`
	Oracle                   json.RawMessage                        `json:"oracle"`
	Requests                 []p14HistoricalMismatchPreparedRequest `json:"requests"`
	SemanticRequestCanonical string                                 `json:"semantic_request_canonical"`
	SemanticRequestDigest    string                                 `json:"semantic_request_digest"`
}

type p14HistoricalMismatchPreparedRequest struct {
	Surface               string `json:"surface"`
	Builder               string `json:"builder"`
	Encoding              string `json:"encoding"`
	CanonicalPayload      string `json:"canonical_payload"`
	PayloadDigest         string `json:"payload_digest"`
	SemanticRequestDigest string `json:"semantic_request_digest"`
}

type p14HistoricalMismatchFamilyEvidence struct {
	ScenarioID            string          `json:"scenario_id"`
	Disposition           string          `json:"disposition"`
	SemanticRequestDigest string          `json:"semantic_request_digest"`
	RequestPayloadDigest  string          `json:"request_payload_digest"`
	ObservationDigest     string          `json:"observation_digest"`
	OldOutcome            string          `json:"old_outcome"`
	OldFailureCode        string          `json:"old_failure_code"`
	Regression            string          `json:"regression"`
	Projection            json.RawMessage `json:"projection"`
}

type p14HistoricalPreparedEvidenceCarrier struct {
	PreparationDigest string `json:"preparation_digest"`
	Preparation       struct {
		Scenarios []p14HistoricalPreparedEvidenceScenario `json:"scenarios"`
	} `json:"preparation"`
}

type p14HistoricalPreparedEvidenceScenario struct {
	ID                       string `json:"id"`
	SemanticRequestCanonical string `json:"semantic_request_canonical"`
	SemanticRequestDigest    string `json:"semantic_request_digest"`
}

func validP14HistoricalMismatchDisposition(
	disposition p14HistoricalMismatchDisposition,
) bool {
	switch disposition {
	case p14HistoricalMismatchDispositionDefect:
		return true
	case p14HistoricalMismatchDispositionOracle:
		return true
	case p14HistoricalMismatchDispositionStale:
		return true
	default:
		return false
	}
}

func parseP14HistoricalMismatchDisposition(
	value string,
) p14HistoricalMismatchDisposition {
	switch value {
	case "defect":
		return p14HistoricalMismatchDispositionDefect
	case "oracle":
		return p14HistoricalMismatchDispositionOracle
	case "stale":
		return p14HistoricalMismatchDispositionStale
	default:
		return p14HistoricalMismatchDispositionUnknown
	}
}

func loadP14HistoricalMismatchEvidence(
	repositoryRoot string,
) (p14HistoricalMismatchEvidenceCarrier, error) {
	path := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(p14HistoricalMismatchEvidencePath),
	)
	return readP14HistoricalMismatchEvidence(
		path,
		p14HistoricalMismatchEvidenceDigest,
	)
}

func loadP14HistoricalMismatchSourceExtract(
	repositoryRoot string,
) (p14HistoricalMismatchSourceExtractCarrier, error) {
	path := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(p14HistoricalMismatchExtractPath),
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		return p14HistoricalMismatchSourceExtractCarrier{}, fmt.Errorf(
			"historical P14 source extract unavailable at %s: %w",
			path,
			err,
		)
	}
	actualDigest := p14Digest(raw)
	if actualDigest != p14HistoricalMismatchExtractDigest {
		return p14HistoricalMismatchSourceExtractCarrier{}, fmt.Errorf(
			"historical P14 source extract digest differs: got %s, want %s",
			actualDigest,
			p14HistoricalMismatchExtractDigest,
		)
	}
	carrier, err := decodeP14HistoricalStrict[p14HistoricalMismatchSourceExtractCarrier](raw, "historical P14 source extract")
	if err != nil {
		return p14HistoricalMismatchSourceExtractCarrier{}, err
	}
	if carrier.Schema != p14HistoricalMismatchExtractSchema ||
		carrier.PayloadEncoding != p14HistoricalMismatchExtractEncoding ||
		carrier.PayloadDigest != p14HistoricalMismatchExtractPayloadDigest ||
		carrier.CompressedPayloadDigest !=
			p14HistoricalMismatchExtractCompressedDigest ||
		len(carrier.PayloadBase64) == 0 {
		return p14HistoricalMismatchSourceExtractCarrier{}, fmt.Errorf(
			"historical P14 source extract header differs",
		)
	}
	return carrier, nil
}

func decodeP14HistoricalStrict[T any](
	raw []byte,
	label string,
) (T, error) {
	var zero T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value T
	if err := decoder.Decode(&value); err != nil {
		return zero, fmt.Errorf("decode %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return zero, fmt.Errorf("%s has trailing JSON", label)
	}
	return value, nil
}

func decodeP14HistoricalMismatchExtractPayload(
	carrier p14HistoricalMismatchSourceExtractCarrier,
) (p14HistoricalMismatchSourceExtractPayload, error) {
	payloadBase64 := strings.Join(carrier.PayloadBase64, "")
	compressed, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"decode historical P14 source extract payload: %w",
			err,
		)
	}
	compressedDigest := p14Digest(compressed)
	if compressedDigest != carrier.CompressedPayloadDigest {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"historical P14 compressed source extract digest differs: got %s, want %s",
			compressedDigest,
			carrier.CompressedPayloadDigest,
		)
	}
	compressedReader := bytes.NewReader(compressed)
	gzipReader, err := gzip.NewReader(compressedReader)
	if err != nil {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"open historical P14 source extract payload: %w",
			err,
		)
	}
	gzipReader.Multistream(false)
	limitedReader := io.LimitReader(
		gzipReader,
		p14HistoricalMismatchExtractLimit+1,
	)
	payloadRaw, readErr := io.ReadAll(limitedReader)
	closeErr := gzipReader.Close()
	if readErr != nil {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"read historical P14 source extract payload: %w",
			readErr,
		)
	}
	if closeErr != nil {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"close historical P14 source extract payload: %w",
			closeErr,
		)
	}
	if len(payloadRaw) > p14HistoricalMismatchExtractLimit {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"historical P14 source extract payload exceeds %d bytes",
			p14HistoricalMismatchExtractLimit,
		)
	}
	payloadDigest := p14Digest(payloadRaw)
	if payloadDigest != carrier.PayloadDigest {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"historical P14 source extract payload digest differs: got %s, want %s",
			payloadDigest,
			carrier.PayloadDigest,
		)
	}
	payload, err := decodeP14HistoricalStrict[p14HistoricalMismatchSourceExtractPayload](payloadRaw, "historical P14 source extract payload")
	if err != nil {
		return p14HistoricalMismatchSourceExtractPayload{}, err
	}
	if payload.Schema != p14HistoricalMismatchExtractPayloadSchema {
		return p14HistoricalMismatchSourceExtractPayload{}, fmt.Errorf(
			"historical P14 source extract payload schema differs",
		)
	}
	return payload, nil
}

func readP14HistoricalMismatchEvidence(
	path string,
	expectedDigest string,
) (p14HistoricalMismatchEvidenceCarrier, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return p14HistoricalMismatchEvidenceCarrier{}, fmt.Errorf(
			"historical P14 mismatch evidence unavailable at %s: %w",
			path,
			err,
		)
	}
	return decodeP14HistoricalMismatchEvidence(raw, expectedDigest)
}

func decodeP14HistoricalMismatchEvidence(
	raw []byte,
	expectedDigest string,
) (p14HistoricalMismatchEvidenceCarrier, error) {
	actualDigest := p14Digest(raw)
	if actualDigest != expectedDigest {
		return p14HistoricalMismatchEvidenceCarrier{}, fmt.Errorf(
			"historical P14 mismatch evidence digest differs: got %s, want %s",
			actualDigest,
			expectedDigest,
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var carrier p14HistoricalMismatchEvidenceCarrier
	if err := decoder.Decode(&carrier); err != nil {
		return p14HistoricalMismatchEvidenceCarrier{}, fmt.Errorf(
			"decode historical P14 mismatch evidence: %w",
			err,
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return p14HistoricalMismatchEvidenceCarrier{}, fmt.Errorf(
			"historical P14 mismatch evidence has trailing JSON",
		)
	}
	if carrier.Schema != p14HistoricalMismatchEvidenceSchema ||
		len(carrier.Families) != 5 ||
		!validP14Digest(carrier.Source.CaptureFileDigest) ||
		!validP14Digest(carrier.Source.DeclaredCaptureDigest) ||
		!validP14Digest(carrier.Source.PreparedFileDigest) ||
		!validP14Digest(carrier.Source.PreparationDigest) ||
		!validP14Digest(carrier.Source.MemoryFixtureDigest) ||
		!validP14Digest(carrier.Source.CandidateDigest) ||
		carrier.Source.CapturePath == "" ||
		carrier.Source.PreparedPath == "" ||
		carrier.Source.MemoryFixtureArchivePath == "" ||
		carrier.Source.KnownEntity.ReferenceID == "" ||
		carrier.Source.KnownEntity.BoundedContextRef == "" {
		return p14HistoricalMismatchEvidenceCarrier{}, fmt.Errorf(
			"historical P14 mismatch evidence header is incomplete",
		)
	}
	return carrier, nil
}

func mustP14HistoricalMismatchEvidence(
	t *testing.T,
) p14HistoricalMismatchEvidenceCarrier {
	t.Helper()
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := loadP14HistoricalMismatchEvidence(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	return carrier
}

func mustP14HistoricalMismatchFamily(
	t *testing.T,
	families []p14HistoricalMismatchFamilyEvidence,
	scenarioID string,
) p14HistoricalMismatchFamilyEvidence {
	t.Helper()
	for _, family := range families {
		if family.ScenarioID == scenarioID {
			return family
		}
	}
	t.Fatalf("historical P14 mismatch evidence lacks %q", scenarioID)
	return p14HistoricalMismatchFamilyEvidence{}
}

func mustP14HistoricalProjectionMap(
	t *testing.T,
	raw json.RawMessage,
) map[string]any {
	t.Helper()
	value, err := decodeP14HistoricalJSONMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustP14HistoricalNestedMap(
	t *testing.T,
	value map[string]any,
	key string,
) map[string]any {
	t.Helper()
	nested, err := p14HistoricalNestedMap(value, key)
	if err != nil {
		t.Fatal(err)
	}
	return nested
}

func decodeP14HistoricalJSONMap(
	raw []byte,
) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode historical mismatch projection: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("historical mismatch projection has trailing JSON")
	}
	return value, nil
}

func p14HistoricalNestedMap(
	value map[string]any,
	key string,
) (map[string]any, error) {
	nested, valid := value[key].(map[string]any)
	if !valid {
		return nil, fmt.Errorf("historical mismatch projection lacks object %q", key)
	}
	return nested, nil
}

func p14HistoricalNestedSlice(
	value map[string]any,
	key string,
) ([]any, error) {
	nested, valid := value[key].([]any)
	if !valid {
		return nil, fmt.Errorf("historical mismatch projection lacks array %q", key)
	}
	return nested, nil
}

func p14HistoricalString(
	value map[string]any,
	key string,
) (string, error) {
	text, valid := value[key].(string)
	if !valid || text == "" {
		return "", fmt.Errorf("historical mismatch projection lacks string %q", key)
	}
	return text, nil
}

func p14HistoricalInt(
	value map[string]any,
	key string,
) (int, error) {
	number, valid := value[key].(json.Number)
	if !valid {
		return 0, fmt.Errorf("historical mismatch projection lacks integer %q", key)
	}
	integer, err := number.Int64()
	if err != nil {
		return 0, fmt.Errorf(
			"historical mismatch projection integer %q is invalid: %w",
			key,
			err,
		)
	}
	return int(integer), nil
}

func p14HistoricalBool(
	value map[string]any,
	key string,
) (bool, error) {
	flag, valid := value[key].(bool)
	if !valid {
		return false, fmt.Errorf(
			"historical mismatch projection lacks boolean %q",
			key,
		)
	}
	return flag, nil
}

func p14HistoricalCanonicalEqual(
	left any,
	right any,
) bool {
	leftRaw, leftErr := marshalP14CanonicalJSON(left)
	rightRaw, rightErr := marshalP14CanonicalJSON(right)
	return leftErr == nil &&
		rightErr == nil &&
		bytes.Equal(leftRaw, rightRaw)
}

func classifyP14HistoricalMismatch(
	source p14HistoricalMismatchEvidenceSource,
	family p14HistoricalMismatchFamilyEvidence,
) (p14HistoricalMismatchDisposition, error) {
	projection, err := decodeP14HistoricalJSONMap(family.Projection)
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	switch family.ScenarioID {
	case "positive_typed_write":
		return classifyP14HistoricalPositiveWrite(projection)
	case "invalid":
		return classifyP14HistoricalValidationOracle(
			source,
			projection,
			"invalid",
		)
	case "underdetermined":
		return classifyP14HistoricalValidationOracle(
			source,
			projection,
			"underdetermined",
		)
	case "concurrency_idempotency":
		return classifyP14HistoricalConcurrency(projection)
	case "existing_record_backfill":
		return classifyP14HistoricalBackfill(projection)
	default:
		return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
			"historical mismatch family %q is unknown",
			family.ScenarioID,
		)
	}
}

func classifyP14HistoricalPositiveWrite(
	projection map[string]any,
) (p14HistoricalMismatchDisposition, error) {
	expected, err := p14HistoricalNestedMap(projection, "expected")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	observed, err := p14HistoricalNestedMap(projection, "observed")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	admit, err := p14HistoricalNestedMap(observed, "admit")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	replay, err := p14HistoricalNestedMap(observed, "replay")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	commitCount, err := p14HistoricalInt(expected, "commit_count")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	admitResult, err := p14HistoricalString(admit, "result")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	admitDisposition, err := p14HistoricalString(admit, "disposition")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	replayResult, err := p14HistoricalString(replay, "result")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	replayDisposition, err := p14HistoricalString(replay, "disposition")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	admitReceipt, err := p14HistoricalNestedMap(admit, "receipt")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	replayReceipt, err := p14HistoricalNestedMap(replay, "receipt")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	if expected["admission_result"] != "committed_then_exact_replay" ||
		commitCount != 1 ||
		admitResult != "committed" ||
		replayResult != "committed" ||
		admitDisposition != "applied" ||
		replayDisposition != "replay" ||
		!p14HistoricalCanonicalEqual(admitReceipt, replayReceipt) {
		return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
			"positive-write capture does not reproduce applied-vs-committed mismatch",
		)
	}
	return p14HistoricalMismatchDispositionStale, nil
}

func classifyP14HistoricalValidationOracle(
	source p14HistoricalMismatchEvidenceSource,
	projection map[string]any,
	expectedVerdict string,
) (p14HistoricalMismatchDisposition, error) {
	request, err := p14HistoricalNestedMap(projection, "request")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	expected, err := p14HistoricalNestedMap(projection, "expected")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	observed, err := p14HistoricalNestedMap(projection, "observed")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	rowsWritten, err := p14HistoricalInt(observed, "rows_written")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	entityID, err := p14HistoricalString(request, "entity_id")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	context, err := p14HistoricalString(request, "context")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	if request["kind"] != "declare_entity" ||
		expected["verdict"] != expectedVerdict ||
		observed["verdict"] != "valid" ||
		observed["resolution_kind"] != "resolved_project_basis" ||
		rowsWritten != 0 ||
		entityID == source.KnownEntity.ReferenceID ||
		context != source.KnownEntity.BoundedContextRef {
		return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
			"%s capture does not reproduce a valid new entity in the admitted context",
			expectedVerdict,
		)
	}
	return p14HistoricalMismatchDispositionOracle, nil
}

func classifyP14HistoricalConcurrency(
	projection map[string]any,
) (p14HistoricalMismatchDisposition, error) {
	expected, err := p14HistoricalNestedMap(projection, "expected")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	observed, err := p14HistoricalNestedMap(projection, "observed")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	commitCount, err := p14HistoricalInt(expected, "commit_count")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	if expected["admission_result"] != "one_commit_and_exact_replays" ||
		expected["conflict_result"] != "idempotency_conflict" ||
		commitCount != 1 {
		return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
			"concurrency expected projection is incomplete",
		)
	}
	appliedCount := 0
	replayCount := 0
	var firstReceipt map[string]any
	for _, commandID := range []string{"writer_a", "writer_b", "replay"} {
		command, commandErr := p14HistoricalNestedMap(observed, commandID)
		if commandErr != nil {
			return p14HistoricalMismatchDispositionUnknown, commandErr
		}
		if command["result"] != "committed" {
			return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
				"concurrency command %q did not return committed",
				commandID,
			)
		}
		disposition, dispositionErr := p14HistoricalString(
			command,
			"disposition",
		)
		if dispositionErr != nil {
			return p14HistoricalMismatchDispositionUnknown, dispositionErr
		}
		switch disposition {
		case "applied":
			appliedCount++
		case "replay":
			replayCount++
		default:
			return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
				"concurrency command %q has open disposition %q",
				commandID,
				disposition,
			)
		}
		receipt, receiptErr := p14HistoricalNestedMap(command, "receipt")
		if receiptErr != nil {
			return p14HistoricalMismatchDispositionUnknown, receiptErr
		}
		if firstReceipt == nil {
			firstReceipt = receipt
		}
		if !p14HistoricalCanonicalEqual(firstReceipt, receipt) {
			return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
				"concurrency command %q returned another receipt",
				commandID,
			)
		}
	}
	conflict, err := p14HistoricalNestedMap(observed, "conflict")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	conflictExit, err := p14HistoricalInt(conflict, "exit_code")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	stderrDigest, err := p14HistoricalString(conflict, "stderr_digest")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	if appliedCount != 1 ||
		replayCount != 2 ||
		conflictExit != 1 ||
		!validP14Digest(stderrDigest) {
		return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
			"concurrency capture does not reproduce one applied writer and exact replays",
		)
	}
	return p14HistoricalMismatchDispositionStale, nil
}

func classifyP14HistoricalBackfill(
	projection map[string]any,
) (p14HistoricalMismatchDisposition, error) {
	expected, err := p14HistoricalNestedSlice(projection, "expected")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	observed, err := p14HistoricalNestedSlice(projection, "observed")
	if err != nil {
		return p14HistoricalMismatchDispositionUnknown, err
	}
	if len(expected) != 3 || len(observed) != len(expected) {
		return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
			"backfill capture call count differs",
		)
	}
	for index := range expected {
		expectedCall, valid := expected[index].(map[string]any)
		if !valid {
			return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
				"backfill expected call %d is invalid",
				index,
			)
		}
		observedCall, valid := observed[index].(map[string]any)
		if !valid {
			return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
				"backfill observed call %d is invalid",
				index,
			)
		}
		exitCode, exitErr := p14HistoricalInt(observedCall, "exit_code")
		if exitErr != nil {
			return p14HistoricalMismatchDispositionUnknown, exitErr
		}
		pretty, prettyErr := p14HistoricalBool(
			observedCall,
			"pretty_json_prefix",
		)
		if prettyErr != nil {
			return p14HistoricalMismatchDispositionUnknown, prettyErr
		}
		digest, digestErr := p14HistoricalString(
			observedCall,
			"stdout_digest",
		)
		if digestErr != nil {
			return p14HistoricalMismatchDispositionUnknown, digestErr
		}
		for _, key := range []string{
			"id",
			"result",
			"graph_revision_delta",
			"durable_change_count",
		} {
			if !p14HistoricalCanonicalEqual(
				expectedCall[key],
				observedCall[key],
			) {
				return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
					"backfill call %d field %q differs",
					index,
					key,
				)
			}
		}
		if exitCode != 0 || !pretty || !validP14Digest(digest) {
			return p14HistoricalMismatchDispositionUnknown, fmt.Errorf(
				"backfill call %d is not the captured pretty JSON success",
				index,
			)
		}
	}
	return p14HistoricalMismatchDispositionOracle, nil
}

func verifyP14HistoricalSourceExtract(
	repositoryRoot string,
	evidence p14HistoricalMismatchEvidenceCarrier,
) error {
	extract, err := loadP14HistoricalMismatchSourceExtract(repositoryRoot)
	if err != nil {
		return err
	}
	if !p14HistoricalCanonicalEqual(extract.Source, evidence.Source) {
		return fmt.Errorf(
			"historical P14 source extract basis differs from mismatch evidence",
		)
	}
	payload, err := decodeP14HistoricalMismatchExtractPayload(extract)
	if err != nil {
		return err
	}
	if len(payload.ScenarioCaptures) != 5 ||
		len(payload.PreparedScenarios) != 5 {
		return fmt.Errorf(
			"historical P14 source extract record count differs",
		)
	}
	captureHeader, err := decodeP14HistoricalStrict[p14HistoricalMismatchCaptureHeader](payload.CaptureHeader, "historical P14 extracted capture header")
	if err != nil {
		return err
	}
	if err := verifyP14HistoricalExtractCaptureHeader(
		evidence.Source,
		captureHeader,
	); err != nil {
		return err
	}
	preparedHeader, err := decodeP14HistoricalStrict[p14HistoricalMismatchPreparedHeader](payload.PreparedHeader, "historical P14 extracted prepared header")
	if err != nil {
		return err
	}
	if err := verifyP14HistoricalExtractPreparedHeader(
		evidence.Source,
		preparedHeader,
	); err != nil {
		return err
	}
	if err := verifyP14HistoricalExtractMemoryFixture(
		evidence.Source,
		payload.MemoryFixture,
	); err != nil {
		return err
	}

	captureScenarios := make(
		[]p14InstalledCLIScenarioCapture,
		0,
		len(payload.ScenarioCaptures),
	)
	captureIDs := make([]string, 0, len(payload.ScenarioCaptures))
	for _, raw := range payload.ScenarioCaptures {
		scenario, decodeErr := decodeP14HistoricalStrict[p14InstalledCLIScenarioCapture](raw, "historical P14 extracted scenario capture")
		if decodeErr != nil {
			return decodeErr
		}
		captureScenarios = append(captureScenarios, scenario)
		captureIDs = append(captureIDs, scenario.ID)
	}
	prepared := p14HistoricalPreparedEvidenceCarrier{
		PreparationDigest: evidence.Source.PreparationDigest,
	}
	preparedByID := make(
		map[string]p14HistoricalMismatchPreparedScenario,
		len(payload.PreparedScenarios),
	)
	preparedIDs := make([]string, 0, len(payload.PreparedScenarios))
	for _, raw := range payload.PreparedScenarios {
		scenario, decodeErr := decodeP14HistoricalStrict[p14HistoricalMismatchPreparedScenario](raw, "historical P14 extracted prepared scenario")
		if decodeErr != nil {
			return decodeErr
		}
		if _, duplicate := preparedByID[scenario.ID]; duplicate {
			return fmt.Errorf(
				"historical P14 extracted prepared scenario %q is duplicated",
				scenario.ID,
			)
		}
		preparedByID[scenario.ID] = scenario
		preparedIDs = append(preparedIDs, scenario.ID)
		prepared.Preparation.Scenarios = append(
			prepared.Preparation.Scenarios,
			p14HistoricalPreparedEvidenceScenario{
				ID:                       scenario.ID,
				SemanticRequestCanonical: scenario.SemanticRequestCanonical,
				SemanticRequestDigest:    scenario.SemanticRequestDigest,
			},
		)
	}
	expectedIDs := []string{
		"positive_typed_write",
		"invalid",
		"underdetermined",
		"concurrency_idempotency",
		"existing_record_backfill",
	}
	if !slices.Equal(captureIDs, expectedIDs) ||
		!slices.Equal(preparedIDs, expectedIDs) {
		return fmt.Errorf(
			"historical P14 source extract scenario IDs differ: capture=%v prepared=%v",
			captureIDs,
			preparedIDs,
		)
	}
	for _, family := range evidence.Families {
		scenario, findErr := findP14HistoricalCapturedScenario(
			captureScenarios,
			family.ScenarioID,
		)
		if findErr != nil {
			return findErr
		}
		preparedScenario, findErr := findP14HistoricalPreparedScenario(
			prepared,
			family.ScenarioID,
		)
		if findErr != nil {
			return findErr
		}
		preparedSource, present := preparedByID[family.ScenarioID]
		if !present {
			return fmt.Errorf(
				"historical P14 source extract lacks prepared source %q",
				family.ScenarioID,
			)
		}
		if err := verifyP14HistoricalExtractScenario(
			evidence.Source,
			family,
			scenario,
			preparedScenario,
			preparedSource,
		); err != nil {
			return err
		}
	}
	return nil
}

func verifyP14HistoricalExtractCaptureHeader(
	source p14HistoricalMismatchEvidenceSource,
	header p14HistoricalMismatchCaptureHeader,
) error {
	prepared := header.Capture.PreparedCarrier
	if header.Schema != p14InstalledCLICaptureCarrierSchema ||
		header.Status != p14InstalledCLICaptureStatus ||
		header.CarrierPath != source.CapturePath ||
		header.CaptureDigest != source.DeclaredCaptureDigest ||
		header.Capture.Schema != p14InstalledCLICaptureInputSchema ||
		header.Capture.Status != p14InstalledCLICaptureStatus ||
		header.Capture.CapturedAt == "" ||
		header.Capture.InstalledExecutablePath == "" ||
		header.Capture.InstalledExecutableDigest != source.CandidateDigest ||
		prepared.CarrierPath != source.PreparedPath ||
		prepared.CarrierDigest != source.PreparedFileDigest ||
		prepared.PreparationDigest != source.PreparationDigest ||
		header.Capture.ReleaseClaim ||
		header.Capture.ResultSemantics == "" {
		return fmt.Errorf("historical P14 extracted capture header differs")
	}
	return nil
}

func verifyP14HistoricalExtractPreparedHeader(
	source p14HistoricalMismatchEvidenceSource,
	header p14HistoricalMismatchPreparedHeader,
) error {
	if header.Schema != p14PreparedCarrierSchema ||
		header.Status != p14ContractStatus ||
		header.CarrierPath != source.PreparedPath ||
		header.PreparationDigest != source.PreparationDigest ||
		header.Preparation.Schema != p14PreparedInputSchema ||
		header.Preparation.Status != p14ContractStatus ||
		header.Preparation.ContractRef == "" ||
		!validP14Digest(header.Preparation.ContractDigest) ||
		header.Preparation.ReleaseClaim ||
		header.Preparation.ResultSemantics == "" {
		return fmt.Errorf("historical P14 extracted prepared header differs")
	}
	return nil
}

func verifyP14HistoricalExtractMemoryFixture(
	source p14HistoricalMismatchEvidenceSource,
	raw json.RawMessage,
) error {
	memoryFixture, err := decodeP14HistoricalJSONMap(raw)
	if err != nil {
		return err
	}
	entityRef, err := p14HistoricalNestedMap(memoryFixture, "entity_ref")
	if err != nil {
		return err
	}
	referenceID, err := p14HistoricalString(entityRef, "reference_id")
	if err != nil {
		return err
	}
	boundedContext, err := p14HistoricalString(
		memoryFixture,
		"bounded_context_ref",
	)
	if err != nil {
		return err
	}
	schema, err := p14HistoricalString(memoryFixture, "schema")
	if err != nil {
		return err
	}
	if schema != "haft.p14.golden-memory-fixture/v3" ||
		entityRef["ref_kind_id"] != "U.EntityRef" ||
		referenceID != source.KnownEntity.ReferenceID ||
		boundedContext != source.KnownEntity.BoundedContextRef {
		return fmt.Errorf("historical P14 extracted memory fixture differs")
	}
	return nil
}

func verifyP14HistoricalExtractScenario(
	source p14HistoricalMismatchEvidenceSource,
	family p14HistoricalMismatchFamilyEvidence,
	scenario p14InstalledCLIScenarioCapture,
	prepared p14HistoricalPreparedEvidenceScenario,
	preparedSource p14HistoricalMismatchPreparedScenario,
) error {
	semanticRaw := []byte(prepared.SemanticRequestCanonical)
	semanticDigest := p14Digest(semanticRaw)
	observation := scenario.SurfaceObservation
	observationRaw := []byte(observation.ObservationCanonical)
	observationDigest := p14Digest(observationRaw)
	if scenario.ID != family.ScenarioID ||
		prepared.ID != family.ScenarioID ||
		preparedSource.ID != family.ScenarioID ||
		scenario.SemanticRequestDigest != family.SemanticRequestDigest ||
		prepared.SemanticRequestDigest != family.SemanticRequestDigest ||
		semanticDigest != family.SemanticRequestDigest ||
		observation.Surface != "installed_cli" ||
		observation.Source != "installed_cli_execution" ||
		observation.RequestPayloadDigest != family.RequestPayloadDigest ||
		observation.ObservationDigest != family.ObservationDigest ||
		observation.SourceReceiptDigest != family.ObservationDigest ||
		observationDigest != family.ObservationDigest ||
		observation.Outcome != family.OldOutcome ||
		observation.FailureCode != family.OldFailureCode {
		return fmt.Errorf(
			"historical P14 extracted scenario %q metadata differs",
			family.ScenarioID,
		)
	}
	request, err := verifyP14HistoricalExtractPreparedRequests(
		family,
		preparedSource,
	)
	if err != nil {
		return err
	}
	receipt, err := decodeP14HistoricalStrict[p14InstalledCLIExecutionReceipt](observationRaw, "historical P14 extracted installed CLI receipt")
	if err != nil {
		return err
	}
	if receipt.Schema != p14InstalledCLIReceiptSchema ||
		receipt.ScenarioID != family.ScenarioID ||
		receipt.Builder != request.Builder ||
		receipt.CandidateDigest != source.CandidateDigest ||
		receipt.RequestPayloadDigest != family.RequestPayloadDigest {
		return fmt.Errorf(
			"historical P14 extracted scenario %q receipt basis differs",
			family.ScenarioID,
		)
	}
	if err := verifyP14HistoricalCommandDigests(
		family.ScenarioID,
		receipt.Commands,
	); err != nil {
		return err
	}
	if err := verifyP14HistoricalExtractReceiptRequest(
		family.ScenarioID,
		request,
		receipt,
	); err != nil {
		return err
	}
	semantic, err := decodeP14HistoricalJSONMap(semanticRaw)
	if err != nil {
		return err
	}
	projection, err := projectP14HistoricalSourceScenario(
		family.ScenarioID,
		semantic,
		receipt,
	)
	if err != nil {
		return err
	}
	frozenProjection, err := decodeP14HistoricalJSONMap(family.Projection)
	if err != nil {
		return err
	}
	if !p14HistoricalCanonicalEqual(projection, frozenProjection) {
		return fmt.Errorf(
			"historical P14 extracted projection %q differs",
			family.ScenarioID,
		)
	}
	disposition, err := classifyP14HistoricalMismatch(source, family)
	if err != nil {
		return err
	}
	declared := parseP14HistoricalMismatchDisposition(family.Disposition)
	if disposition != declared {
		return fmt.Errorf(
			"historical P14 extracted disposition %q differs",
			family.ScenarioID,
		)
	}
	regression, err := p14HistoricalExpectedRegression(family.ScenarioID)
	if err != nil {
		return err
	}
	if family.Regression != regression {
		return fmt.Errorf(
			"historical P14 extracted regression claim %q differs",
			family.ScenarioID,
		)
	}
	return nil
}

func verifyP14HistoricalExtractPreparedRequests(
	family p14HistoricalMismatchFamilyEvidence,
	scenario p14HistoricalMismatchPreparedScenario,
) (p14HistoricalMismatchPreparedRequest, error) {
	oracle, err := decodeP14HistoricalJSONMap(scenario.Oracle)
	if err != nil {
		return p14HistoricalMismatchPreparedRequest{}, err
	}
	expectedResultDigest, err := p14HistoricalString(
		oracle,
		"expected_result_digest",
	)
	if err != nil {
		return p14HistoricalMismatchPreparedRequest{}, err
	}
	localOracleDigest, err := p14HistoricalString(
		oracle,
		"local_oracle_output_digest",
	)
	if err != nil {
		return p14HistoricalMismatchPreparedRequest{}, err
	}
	normalizationID, expectedSurfaces, err :=
		p14HistoricalExpectedPreparedRequestShape(family.ScenarioID)
	if err != nil {
		return p14HistoricalMismatchPreparedRequest{}, err
	}
	if oracle["kind"] != "normalized_digest" ||
		oracle["normalization_id"] != normalizationID ||
		oracle["expected_effect"] == "" ||
		!validP14Digest(expectedResultDigest) ||
		!validP14Digest(localOracleDigest) {
		return p14HistoricalMismatchPreparedRequest{}, fmt.Errorf(
			"historical P14 extracted oracle %q differs",
			family.ScenarioID,
		)
	}
	installedCount := 0
	var installed p14HistoricalMismatchPreparedRequest
	actualSurfaces := make([]string, 0, len(scenario.Requests))
	for index, request := range scenario.Requests {
		payloadRaw := []byte(request.CanonicalPayload)
		payloadDigest := p14Digest(payloadRaw)
		expectedEncoding := "canonical_json"
		if request.Surface == "installed_cli" {
			expectedEncoding = "argv_json"
		}
		if request.Surface == "" ||
			request.Builder == "" ||
			request.Encoding != expectedEncoding ||
			request.SemanticRequestDigest != family.SemanticRequestDigest ||
			payloadDigest != request.PayloadDigest {
			return p14HistoricalMismatchPreparedRequest{}, fmt.Errorf(
				"historical P14 extracted request %q/%q differs",
				family.ScenarioID,
				request.Surface,
			)
		}
		payload, decodeErr := decodeP14HistoricalJSONMap(payloadRaw)
		if decodeErr != nil {
			return p14HistoricalMismatchPreparedRequest{}, decodeErr
		}
		if payload["semantic_request_digest"] !=
			family.SemanticRequestDigest {
			return p14HistoricalMismatchPreparedRequest{}, fmt.Errorf(
				"historical P14 extracted request %q semantic basis differs",
				family.ScenarioID,
			)
		}
		if request.Surface == "installed_cli" {
			installed = request
			installedCount++
		}
		if index < len(expectedSurfaces) {
			actualSurfaces = append(actualSurfaces, request.Surface)
		}
	}
	if !slices.Equal(actualSurfaces, expectedSurfaces) ||
		len(scenario.Requests) != len(expectedSurfaces) ||
		installedCount != 1 ||
		installed.PayloadDigest != family.RequestPayloadDigest {
		return p14HistoricalMismatchPreparedRequest{}, fmt.Errorf(
			"historical P14 extracted installed request %q differs",
			family.ScenarioID,
		)
	}
	return installed, nil
}

func p14HistoricalExpectedPreparedRequestShape(
	scenarioID string,
) (string, []string, error) {
	switch scenarioID {
	case "positive_typed_write",
		"invalid",
		"underdetermined",
		"concurrency_idempotency":
		return "p14.memory-operation.semantic-outcome.v1",
			[]string{"installed_cli", "live_mcp"},
			nil
	case "existing_record_backfill":
		return "p14.existing-record-backfill.semantic-outcome.v1",
			[]string{"installed_cli"},
			nil
	default:
		return "", nil, fmt.Errorf(
			"historical P14 prepared request shape %q is unknown",
			scenarioID,
		)
	}
}

func verifyP14HistoricalExtractReceiptRequest(
	scenarioID string,
	request p14HistoricalMismatchPreparedRequest,
	receipt p14InstalledCLIExecutionReceipt,
) error {
	payload, err := decodeP14HistoricalJSONMap(
		[]byte(request.CanonicalPayload),
	)
	if err != nil {
		return err
	}
	calls, err := p14HistoricalNestedSlice(payload, "calls")
	if err != nil {
		return err
	}
	if len(calls) != len(receipt.Commands) {
		return fmt.Errorf(
			"historical P14 extracted request %q command count differs",
			scenarioID,
		)
	}
	seen := make(map[string]struct{}, len(calls))
	for _, command := range receipt.Commands {
		var matching map[string]any
		for _, value := range calls {
			call, valid := value.(map[string]any)
			if !valid {
				return fmt.Errorf(
					"historical P14 extracted request %q call is invalid",
					scenarioID,
				)
			}
			if call["id"] == command.ID {
				matching = call
				break
			}
		}
		if matching == nil {
			return fmt.Errorf(
				"historical P14 extracted request %q lacks command %q",
				scenarioID,
				command.ID,
			)
		}
		if _, duplicate := seen[command.ID]; duplicate {
			return fmt.Errorf(
				"historical P14 extracted receipt %q duplicates command %q",
				scenarioID,
				command.ID,
			)
		}
		seen[command.ID] = struct{}{}
		stdin, valid := matching["stdin"].(string)
		if !valid ||
			p14Digest([]byte(stdin)) != command.StdinDigest ||
			!p14HistoricalCanonicalEqual(
				matching["argv"],
				command.Argv,
			) {
			return fmt.Errorf(
				"historical P14 extracted request %q command %q differs",
				scenarioID,
				command.ID,
			)
		}
	}
	return nil
}

func p14HistoricalExpectedRegression(
	scenarioID string,
) (string, error) {
	switch scenarioID {
	case "positive_typed_write":
		return "normalize applied to committed while retaining replay as replay", nil
	case "invalid":
		return "declare the already-known exact EntityRef with contradictory identity metadata", nil
	case "underdetermined":
		return "declare an entity in a bounded context absent from the selected TypeEnv", nil
	case "concurrency_idempotency":
		return "normalize applied to committed before counting one commit and exact replays", nil
	case "existing_record_backfill":
		return "use a whitespace-tolerant single-document parser for backfill only", nil
	default:
		return "", fmt.Errorf(
			"historical P14 regression claim %q is unknown",
			scenarioID,
		)
	}
}

func verifyP14HistoricalSourceCarriersWhenPresent(
	repositoryRoot string,
	evidence p14HistoricalMismatchEvidenceCarrier,
) error {
	sourcePaths := []string{
		evidence.Source.CapturePath,
		evidence.Source.PreparedPath,
		evidence.Source.MemoryFixtureArchivePath,
	}
	present := make([]bool, 0, len(sourcePaths))
	for _, relativePath := range sourcePaths {
		path := filepath.Join(
			repositoryRoot,
			filepath.FromSlash(relativePath),
		)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			present = append(present, false)
			continue
		}
		if err != nil {
			return fmt.Errorf(
				"inspect historical P14 source carrier %s: %w",
				relativePath,
				err,
			)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf(
				"historical P14 source carrier %s is not a regular file",
				relativePath,
			)
		}
		present = append(present, true)
	}
	if !slices.Contains(present, true) {
		return nil
	}
	if slices.Contains(present, false) {
		return fmt.Errorf(
			"historical P14 source carrier set is partial: %v",
			sourcePaths,
		)
	}

	captureRaw, err := readP14HistoricalSourceCarrier(
		repositoryRoot,
		evidence.Source.CapturePath,
		evidence.Source.CaptureFileDigest,
	)
	if err != nil {
		return err
	}
	var capture p14InstalledCLICaptureCarrier
	if err := json.Unmarshal(captureRaw, &capture); err != nil {
		return fmt.Errorf("decode historical P14 installed capture: %w", err)
	}
	if capture.CaptureDigest != evidence.Source.DeclaredCaptureDigest ||
		capture.CarrierPath != evidence.Source.CapturePath ||
		capture.Capture.PreparedCarrier.CarrierPath !=
			evidence.Source.PreparedPath ||
		capture.Capture.PreparedCarrier.CarrierDigest !=
			evidence.Source.PreparedFileDigest ||
		capture.Capture.PreparedCarrier.PreparationDigest !=
			evidence.Source.PreparationDigest ||
		capture.Capture.InstalledExecutableDigest !=
			evidence.Source.CandidateDigest {
		return fmt.Errorf("historical P14 installed capture basis differs")
	}
	captureBasis, err := json.Marshal(capture.Capture)
	if err != nil {
		return fmt.Errorf("redigest historical P14 capture basis: %w", err)
	}
	if p14Digest(captureBasis) != evidence.Source.DeclaredCaptureDigest {
		return fmt.Errorf("historical P14 declared capture digest differs")
	}

	preparedRaw, err := readP14HistoricalSourceCarrier(
		repositoryRoot,
		evidence.Source.PreparedPath,
		evidence.Source.PreparedFileDigest,
	)
	if err != nil {
		return err
	}
	var prepared p14HistoricalPreparedEvidenceCarrier
	if err := json.Unmarshal(preparedRaw, &prepared); err != nil {
		return fmt.Errorf("decode historical P14 prepared carrier: %w", err)
	}
	if prepared.PreparationDigest != evidence.Source.PreparationDigest {
		return fmt.Errorf("historical P14 preparation header differs")
	}
	preparationDigest, err := p14HistoricalPreparationDigest(preparedRaw)
	if err != nil {
		return err
	}
	if preparationDigest != evidence.Source.PreparationDigest {
		return fmt.Errorf(
			"historical P14 preparation digest differs: got %s, want %s",
			preparationDigest,
			evidence.Source.PreparationDigest,
		)
	}

	memoryRaw, err := readP14HistoricalSourceCarrier(
		repositoryRoot,
		evidence.Source.MemoryFixtureArchivePath,
		evidence.Source.MemoryFixtureDigest,
	)
	if err != nil {
		return err
	}
	if err := verifyP14HistoricalExtractMatchesFullSources(
		repositoryRoot,
		evidence,
		captureRaw,
		preparedRaw,
		memoryRaw,
	); err != nil {
		return err
	}
	var memoryFixture struct {
		EntityRef struct {
			ReferenceID string `json:"reference_id"`
		} `json:"entity_ref"`
		BoundedContextRef string `json:"bounded_context_ref"`
	}
	if err := json.Unmarshal(memoryRaw, &memoryFixture); err != nil {
		return fmt.Errorf("decode historical P14 memory fixture: %w", err)
	}
	if memoryFixture.EntityRef.ReferenceID !=
		evidence.Source.KnownEntity.ReferenceID ||
		memoryFixture.BoundedContextRef !=
			evidence.Source.KnownEntity.BoundedContextRef {
		return fmt.Errorf("historical P14 memory fixture identity differs")
	}

	for _, family := range evidence.Families {
		scenario, findErr := findP14HistoricalCapturedScenario(
			capture.Capture.ScenarioCaptures,
			family.ScenarioID,
		)
		if findErr != nil {
			return findErr
		}
		preparedScenario, findErr := findP14HistoricalPreparedScenario(
			prepared,
			family.ScenarioID,
		)
		if findErr != nil {
			return findErr
		}
		if scenario.SemanticRequestDigest !=
			family.SemanticRequestDigest ||
			preparedScenario.SemanticRequestDigest !=
				family.SemanticRequestDigest ||
			scenario.SurfaceObservation.RequestPayloadDigest !=
				family.RequestPayloadDigest ||
			scenario.SurfaceObservation.ObservationDigest !=
				family.ObservationDigest ||
			scenario.SurfaceObservation.Outcome != family.OldOutcome ||
			scenario.SurfaceObservation.FailureCode !=
				family.OldFailureCode {
			return fmt.Errorf(
				"historical P14 scenario %q metadata differs",
				family.ScenarioID,
			)
		}
		observationRaw := []byte(
			scenario.SurfaceObservation.ObservationCanonical,
		)
		if p14Digest(observationRaw) != family.ObservationDigest ||
			scenario.SurfaceObservation.SourceReceiptDigest !=
				family.ObservationDigest {
			return fmt.Errorf(
				"historical P14 scenario %q receipt digest differs",
				family.ScenarioID,
			)
		}
		var receipt p14InstalledCLIExecutionReceipt
		if err := json.Unmarshal(observationRaw, &receipt); err != nil {
			return fmt.Errorf(
				"decode historical P14 scenario %q receipt: %w",
				family.ScenarioID,
				err,
			)
		}
		if receipt.ScenarioID != family.ScenarioID ||
			receipt.CandidateDigest != evidence.Source.CandidateDigest ||
			receipt.RequestPayloadDigest != family.RequestPayloadDigest {
			return fmt.Errorf(
				"historical P14 scenario %q receipt basis differs",
				family.ScenarioID,
			)
		}
		if err := verifyP14HistoricalCommandDigests(
			family.ScenarioID,
			receipt.Commands,
		); err != nil {
			return err
		}
		semantic, err := decodeP14HistoricalJSONMap(
			[]byte(preparedScenario.SemanticRequestCanonical),
		)
		if err != nil {
			return err
		}
		projection, err := projectP14HistoricalSourceScenario(
			family.ScenarioID,
			semantic,
			receipt,
		)
		if err != nil {
			return err
		}
		frozenProjection, err := decodeP14HistoricalJSONMap(
			family.Projection,
		)
		if err != nil {
			return err
		}
		if !p14HistoricalCanonicalEqual(projection, frozenProjection) {
			return fmt.Errorf(
				"normalized historical P14 projection %q differs from exact source carriers",
				family.ScenarioID,
			)
		}
	}
	return nil
}

func verifyP14HistoricalExtractMatchesFullSources(
	repositoryRoot string,
	evidence p14HistoricalMismatchEvidenceCarrier,
	captureRaw []byte,
	preparedRaw []byte,
	memoryRaw []byte,
) error {
	extract, err := loadP14HistoricalMismatchSourceExtract(repositoryRoot)
	if err != nil {
		return err
	}
	if !p14HistoricalCanonicalEqual(extract.Source, evidence.Source) {
		return fmt.Errorf(
			"historical P14 source extract basis differs from full sources",
		)
	}
	payload, err := decodeP14HistoricalMismatchExtractPayload(extract)
	if err != nil {
		return err
	}
	captureRoot, err := decodeP14HistoricalJSONMap(captureRaw)
	if err != nil {
		return err
	}
	capture, err := p14HistoricalNestedMap(captureRoot, "capture")
	if err != nil {
		return err
	}
	sourceCaptures, err := p14HistoricalNestedSlice(
		capture,
		"scenario_captures",
	)
	if err != nil {
		return err
	}
	if err := verifyP14HistoricalExtractRecordSet(
		"capture",
		sourceCaptures,
		payload.ScenarioCaptures,
	); err != nil {
		return err
	}
	preparedRoot, err := decodeP14HistoricalJSONMap(preparedRaw)
	if err != nil {
		return err
	}
	preparation, err := p14HistoricalNestedMap(
		preparedRoot,
		"preparation",
	)
	if err != nil {
		return err
	}
	sourcePrepared, err := p14HistoricalNestedSlice(
		preparation,
		"scenarios",
	)
	if err != nil {
		return err
	}
	if err := verifyP14HistoricalExtractRecordSet(
		"prepared",
		sourcePrepared,
		payload.PreparedScenarios,
	); err != nil {
		return err
	}
	sourceMemory, err := decodeP14HistoricalJSONMap(memoryRaw)
	if err != nil {
		return err
	}
	extractedMemory, err := decodeP14HistoricalJSONMap(
		payload.MemoryFixture,
	)
	if err != nil {
		return err
	}
	if !p14HistoricalCanonicalEqual(sourceMemory, extractedMemory) {
		return fmt.Errorf(
			"historical P14 extracted memory fixture differs from full source",
		)
	}
	return nil
}

func verifyP14HistoricalExtractRecordSet(
	label string,
	sourceRecords []any,
	extractedRecords []json.RawMessage,
) error {
	extractedByID := make(map[string]map[string]any, len(extractedRecords))
	for _, raw := range extractedRecords {
		record, err := decodeP14HistoricalJSONMap(raw)
		if err != nil {
			return err
		}
		id, err := p14HistoricalString(record, "id")
		if err != nil {
			return err
		}
		if _, duplicate := extractedByID[id]; duplicate {
			return fmt.Errorf(
				"historical P14 extracted %s record %q is duplicated",
				label,
				id,
			)
		}
		extractedByID[id] = record
	}
	for id, extracted := range extractedByID {
		var source map[string]any
		for _, value := range sourceRecords {
			record, valid := value.(map[string]any)
			if !valid {
				return fmt.Errorf(
					"historical P14 full %s source record is invalid",
					label,
				)
			}
			if record["id"] == id {
				source = record
				break
			}
		}
		if source == nil ||
			!p14HistoricalCanonicalEqual(source, extracted) {
			return fmt.Errorf(
				"historical P14 extracted %s record %q differs from full source",
				label,
				id,
			)
		}
	}
	return nil
}

func readP14HistoricalSourceCarrier(
	repositoryRoot string,
	relativePath string,
	expectedDigest string,
) ([]byte, error) {
	path := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(relativePath),
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"read historical P14 source carrier %s: %w",
			relativePath,
			err,
		)
	}
	actualDigest := p14Digest(raw)
	if actualDigest != expectedDigest {
		return nil, fmt.Errorf(
			"historical P14 source carrier %s digest differs: got %s, want %s",
			relativePath,
			actualDigest,
			expectedDigest,
		)
	}
	return raw, nil
}

func p14HistoricalPreparationDigest(
	raw []byte,
) (string, error) {
	var envelope struct {
		Preparation preparedRequestOracleInput `json:"preparation"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("decode historical P14 preparation envelope: %w", err)
	}
	canonical, err := marshalP14CanonicalJSON(envelope.Preparation)
	if err != nil {
		return "", err
	}
	return p14Digest(canonical), nil
}

func findP14HistoricalCapturedScenario(
	scenarios []p14InstalledCLIScenarioCapture,
	scenarioID string,
) (p14InstalledCLIScenarioCapture, error) {
	for _, scenario := range scenarios {
		if scenario.ID == scenarioID {
			return scenario, nil
		}
	}
	return p14InstalledCLIScenarioCapture{}, fmt.Errorf(
		"historical P14 capture lacks scenario %q",
		scenarioID,
	)
}

func findP14HistoricalPreparedScenario(
	carrier p14HistoricalPreparedEvidenceCarrier,
	scenarioID string,
) (p14HistoricalPreparedEvidenceScenario, error) {
	for _, scenario := range carrier.Preparation.Scenarios {
		if scenario.ID == scenarioID {
			return scenario, nil
		}
	}
	return p14HistoricalPreparedEvidenceScenario{}, fmt.Errorf(
		"historical P14 prepared carrier lacks scenario %q",
		scenarioID,
	)
}

func verifyP14HistoricalCommandDigests(
	scenarioID string,
	commands []p14InstalledCLICommandReceipt,
) error {
	for _, command := range commands {
		stdout, err := base64.StdEncoding.DecodeString(command.StdoutBase64)
		if err != nil {
			return fmt.Errorf(
				"decode historical P14 %s/%s stdout: %w",
				scenarioID,
				command.ID,
				err,
			)
		}
		stderr, err := base64.StdEncoding.DecodeString(command.StderrBase64)
		if err != nil {
			return fmt.Errorf(
				"decode historical P14 %s/%s stderr: %w",
				scenarioID,
				command.ID,
				err,
			)
		}
		if p14Digest(stdout) != command.StdoutDigest ||
			p14Digest(stderr) != command.StderrDigest {
			return fmt.Errorf(
				"historical P14 %s/%s command digest differs",
				scenarioID,
				command.ID,
			)
		}
	}
	return nil
}

func projectP14HistoricalSourceScenario(
	scenarioID string,
	semantic map[string]any,
	receipt p14InstalledCLIExecutionReceipt,
) (map[string]any, error) {
	switch scenarioID {
	case "positive_typed_write":
		return projectP14HistoricalPositiveSource(semantic, receipt)
	case "invalid", "underdetermined":
		return projectP14HistoricalValidationSource(semantic, receipt)
	case "concurrency_idempotency":
		return projectP14HistoricalConcurrencySource(semantic, receipt)
	case "existing_record_backfill":
		return projectP14HistoricalBackfillSource(semantic, receipt)
	default:
		return nil, fmt.Errorf(
			"historical P14 source projector lacks %q",
			scenarioID,
		)
	}
}

func projectP14HistoricalPositiveSource(
	semantic map[string]any,
	receipt p14InstalledCLIExecutionReceipt,
) (map[string]any, error) {
	expected, err := p14HistoricalNestedMap(semantic, "expected")
	if err != nil {
		return nil, err
	}
	admit, _, err := p14HistoricalCommandJSON(receipt.Commands, "admit")
	if err != nil {
		return nil, err
	}
	replay, _, err := p14HistoricalCommandJSON(receipt.Commands, "replay")
	if err != nil {
		return nil, err
	}
	admitProjection, err := p14HistoricalAdmissionProjection(
		admit,
		true,
	)
	if err != nil {
		return nil, err
	}
	replayProjection, err := p14HistoricalAdmissionProjection(
		replay,
		true,
	)
	if err != nil {
		return nil, err
	}
	expectedProjection, err := p14HistoricalSelect(
		expected,
		"admission_result",
		"commit_count",
		"graph_revision_delta",
		"authority_granted",
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"expected": expectedProjection,
		"observed": map[string]any{
			"admit":  admitProjection,
			"replay": replayProjection,
		},
	}, nil
}

func projectP14HistoricalValidationSource(
	semantic map[string]any,
	receipt p14InstalledCLIExecutionReceipt,
) (map[string]any, error) {
	steps, err := p14HistoricalNestedSlice(semantic, "steps")
	if err != nil || len(steps) != 1 {
		return nil, fmt.Errorf("historical validation semantic steps differ")
	}
	step, valid := steps[0].(map[string]any)
	if !valid {
		return nil, fmt.Errorf("historical validation semantic step is invalid")
	}
	request, err := p14HistoricalNestedMap(step, "request")
	if err != nil {
		return nil, err
	}
	changeSet, err := p14HistoricalNestedMap(request, "change_set")
	if err != nil {
		return nil, err
	}
	changes, err := p14HistoricalNestedSlice(changeSet, "changes")
	if err != nil || len(changes) != 1 {
		return nil, fmt.Errorf("historical validation semantic changes differ")
	}
	change, valid := changes[0].(map[string]any)
	if !valid {
		return nil, fmt.Errorf("historical validation change is invalid")
	}
	requestProjection, err := p14HistoricalSelect(
		change,
		"kind",
		"entity_id",
		"context",
	)
	if err != nil {
		return nil, err
	}
	expected, err := p14HistoricalNestedMap(semantic, "expected")
	if err != nil {
		return nil, err
	}
	expectedProjection, err := p14HistoricalSelect(
		expected,
		"verdict",
		"commit_count",
		"graph_revision_delta",
		"authority_granted",
	)
	if err != nil {
		return nil, err
	}
	observed, _, err := p14HistoricalCommandJSON(receipt.Commands, "validate")
	if err != nil {
		return nil, err
	}
	basis, err := p14HistoricalNestedMap(observed, "basis")
	if err != nil {
		return nil, err
	}
	persistence, err := p14HistoricalNestedMap(
		observed,
		"persistence_disposition",
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"request":  requestProjection,
		"expected": expectedProjection,
		"observed": map[string]any{
			"verdict":           observed["verdict"],
			"resolution_kind":   basis["resolution_kind"],
			"rows_written":      persistence["rows_written"],
			"authority_granted": persistence["authority_granted"],
		},
	}, nil
}

func projectP14HistoricalConcurrencySource(
	semantic map[string]any,
	receipt p14InstalledCLIExecutionReceipt,
) (map[string]any, error) {
	expected, err := p14HistoricalNestedMap(semantic, "expected")
	if err != nil {
		return nil, err
	}
	expectedProjection, err := p14HistoricalSelect(
		expected,
		"admission_result",
		"conflict_result",
		"commit_count",
		"graph_revision_delta",
		"authority_granted",
	)
	if err != nil {
		return nil, err
	}
	observed := make(map[string]any)
	for _, commandID := range []string{"writer_a", "writer_b", "replay"} {
		output, _, commandErr := p14HistoricalCommandJSON(
			receipt.Commands,
			commandID,
		)
		if commandErr != nil {
			return nil, commandErr
		}
		projection, projectionErr := p14HistoricalAdmissionProjection(
			output,
			false,
		)
		if projectionErr != nil {
			return nil, projectionErr
		}
		observed[commandID] = projection
	}
	conflict, err := p14HistoricalCommand(receipt.Commands, "conflict")
	if err != nil {
		return nil, err
	}
	observed["conflict"] = map[string]any{
		"exit_code":     conflict.ExitCode,
		"stdout_digest": conflict.StdoutDigest,
		"stderr_digest": conflict.StderrDigest,
	}
	return map[string]any{
		"expected": expectedProjection,
		"observed": observed,
	}, nil
}

func projectP14HistoricalBackfillSource(
	semantic map[string]any,
	receipt p14InstalledCLIExecutionReceipt,
) (map[string]any, error) {
	expectedRoot, err := p14HistoricalNestedMap(semantic, "expected")
	if err != nil {
		return nil, err
	}
	expectedCalls, err := p14HistoricalNestedSlice(expectedRoot, "calls")
	if err != nil {
		return nil, err
	}
	expectedProjection := make([]any, 0, len(expectedCalls))
	for _, value := range expectedCalls {
		call, valid := value.(map[string]any)
		if !valid {
			return nil, fmt.Errorf("historical backfill expected call is invalid")
		}
		selected, selectErr := p14HistoricalSelect(
			call,
			"id",
			"result",
			"graph_revision_delta",
			"durable_change_count",
		)
		if selectErr != nil {
			return nil, selectErr
		}
		expectedProjection = append(expectedProjection, selected)
	}
	observedProjection := make([]any, 0, len(receipt.Commands))
	for _, command := range receipt.Commands {
		output, raw, commandErr := p14HistoricalCommandJSON(
			receipt.Commands,
			command.ID,
		)
		if commandErr != nil {
			return nil, commandErr
		}
		routes, routesErr := p14HistoricalNestedSlice(output, "routes")
		if routesErr != nil || len(routes) != 1 {
			return nil, fmt.Errorf(
				"historical backfill %q routes differ",
				command.ID,
			)
		}
		route, valid := routes[0].(map[string]any)
		if !valid {
			return nil, fmt.Errorf(
				"historical backfill %q route is invalid",
				command.ID,
			)
		}
		report, reportErr := p14HistoricalNestedMap(
			route,
			"projection_report",
		)
		if reportErr != nil {
			return nil, reportErr
		}
		before, beforeErr := p14HistoricalInt(
			output,
			"graph_revision_before",
		)
		if beforeErr != nil {
			return nil, beforeErr
		}
		after, afterErr := p14HistoricalInt(
			output,
			"graph_revision_after",
		)
		if afterErr != nil {
			return nil, afterErr
		}
		durable, durableErr := p14HistoricalInt(
			report,
			"durable_change_count",
		)
		if durableErr != nil {
			return nil, durableErr
		}
		observedProjection = append(
			observedProjection,
			map[string]any{
				"id":                   command.ID,
				"exit_code":            command.ExitCode,
				"stdout_digest":        command.StdoutDigest,
				"pretty_json_prefix":   bytes.HasPrefix(raw, []byte("{\n  ")),
				"result":               route["result"],
				"graph_revision_delta": after - before,
				"durable_change_count": durable,
			},
		)
	}
	return map[string]any{
		"expected": expectedProjection,
		"observed": observedProjection,
	}, nil
}

func p14HistoricalAdmissionProjection(
	output map[string]any,
	includeAuthority bool,
) (map[string]any, error) {
	persistence, err := p14HistoricalNestedMap(
		output,
		"persistence_disposition",
	)
	if err != nil {
		return nil, err
	}
	receipt, err := p14HistoricalNestedMap(output, "receipt")
	if err != nil {
		return nil, err
	}
	projection := map[string]any{
		"result":      output["result"],
		"disposition": persistence["disposition"],
		"receipt":     receipt,
	}
	if includeAuthority {
		projection["authority_granted"] = persistence["authority_granted"]
	}
	return projection, nil
}

func p14HistoricalCommandJSON(
	commands []p14InstalledCLICommandReceipt,
	commandID string,
) (map[string]any, []byte, error) {
	command, err := p14HistoricalCommand(commands, commandID)
	if err != nil {
		return nil, nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(command.StdoutBase64)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"decode historical P14 %q stdout: %w",
			commandID,
			err,
		)
	}
	output, err := decodeP14HistoricalJSONMap(raw)
	if err != nil {
		return nil, nil, err
	}
	return output, raw, nil
}

func p14HistoricalCommand(
	commands []p14InstalledCLICommandReceipt,
	commandID string,
) (p14InstalledCLICommandReceipt, error) {
	for _, command := range commands {
		if command.ID == commandID {
			return command, nil
		}
	}
	return p14InstalledCLICommandReceipt{}, fmt.Errorf(
		"historical P14 receipt lacks command %q",
		commandID,
	)
}

func p14HistoricalSelect(
	source map[string]any,
	keys ...string,
) (map[string]any, error) {
	selected := make(map[string]any, len(keys))
	for _, key := range keys {
		value, present := source[key]
		if !present {
			return nil, fmt.Errorf(
				"historical P14 projection source lacks %q",
				key,
			)
		}
		selected[key] = value
	}
	return selected, nil
}

func TestP14HistoricalMismatchDispositionsAreClosedAndComplete(
	t *testing.T,
) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := loadP14HistoricalMismatchEvidence(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyP14HistoricalSourceExtract(
		repositoryRoot,
		evidence,
	); err != nil {
		t.Fatal(err)
	}
	if err := verifyP14HistoricalSourceCarriersWhenPresent(
		repositoryRoot,
		evidence,
	); err != nil {
		t.Fatal(err)
	}
	expectedIDs := []string{
		"positive_typed_write",
		"invalid",
		"underdetermined",
		"concurrency_idempotency",
		"existing_record_backfill",
	}
	expectedDispositions := []p14HistoricalMismatchDisposition{
		p14HistoricalMismatchDispositionStale,
		p14HistoricalMismatchDispositionOracle,
		p14HistoricalMismatchDispositionOracle,
		p14HistoricalMismatchDispositionStale,
		p14HistoricalMismatchDispositionOracle,
	}
	actualIDs := make(
		[]string,
		0,
		len(evidence.Families),
	)
	actualDispositions := make(
		[]p14HistoricalMismatchDisposition,
		0,
		len(evidence.Families),
	)
	for _, family := range evidence.Families {
		disposition, classifyErr := classifyP14HistoricalMismatch(
			evidence.Source,
			family,
		)
		if classifyErr != nil {
			t.Fatalf(
				"replay historical mismatch %q: %v",
				family.ScenarioID,
				classifyErr,
			)
		}
		declared := parseP14HistoricalMismatchDisposition(
			family.Disposition,
		)
		if disposition != declared {
			t.Fatalf(
				"historical mismatch %q replay classified %d, carrier declares %d",
				family.ScenarioID,
				disposition,
				declared,
			)
		}
		if !validP14HistoricalMismatchDisposition(disposition) ||
			family.Regression == "" ||
			!validP14Digest(family.SemanticRequestDigest) ||
			!validP14Digest(family.RequestPayloadDigest) ||
			!validP14Digest(family.ObservationDigest) ||
			family.OldOutcome != p14SurfaceOutcomeMismatch ||
			family.OldFailureCode != "normalization_failed" {
			t.Fatalf(
				"historical mismatch %q lacks closed capture evidence",
				family.ScenarioID,
			)
		}
		actualIDs = append(actualIDs, family.ScenarioID)
		actualDispositions = append(
			actualDispositions,
			disposition,
		)
	}
	if !slices.Equal(actualIDs, expectedIDs) {
		t.Fatalf(
			"historical mismatch registry IDs = %v, want %v",
			actualIDs,
			expectedIDs,
		)
	}
	if !slices.Equal(actualDispositions, expectedDispositions) {
		t.Fatalf(
			"historical mismatch dispositions = %v, want %v",
			actualDispositions,
			expectedDispositions,
		)
	}
	if validP14HistoricalMismatchDisposition(
		p14HistoricalMismatchDispositionUnknown,
	) || validP14HistoricalMismatchDisposition(
		p14HistoricalMismatchDisposition(255),
	) {
		t.Fatal("historical mismatch disposition enum accepted an open value")
	}
}

func TestP14HistoricalMismatchEvidenceFailsClosedWhenMissingOrTampered(
	t *testing.T,
) {
	missingPath := filepath.Join(
		t.TempDir(),
		"missing-historical-mismatch-evidence.json",
	)
	_, err := readP14HistoricalMismatchEvidence(
		missingPath,
		p14HistoricalMismatchEvidenceDigest,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "historical P14 mismatch evidence unavailable") {
		t.Fatalf("missing historical evidence did not fail closed: %v", err)
	}

	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(p14HistoricalMismatchEvidencePath),
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := slices.Clone(raw)
	tampered[len(tampered)-2] ^= 1
	_, err = decodeP14HistoricalMismatchEvidence(
		tampered,
		p14HistoricalMismatchEvidenceDigest,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "digest differs") {
		t.Fatalf("tampered historical evidence did not fail closed: %v", err)
	}
}

func TestP14HistoricalMismatchSourceExtractSurvivesCleanCheckoutAndFailsClosed(
	t *testing.T,
) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	cleanRoot := t.TempDir()
	for _, relativePath := range []string{
		p14HistoricalMismatchEvidencePath,
		p14HistoricalMismatchExtractPath,
	} {
		sourcePath := filepath.Join(
			repositoryRoot,
			filepath.FromSlash(relativePath),
		)
		raw, readErr := os.ReadFile(sourcePath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		targetPath := filepath.Join(
			cleanRoot,
			filepath.FromSlash(relativePath),
		)
		if mkdirErr := os.MkdirAll(
			filepath.Dir(targetPath),
			0o755,
		); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		if writeErr := os.WriteFile(
			targetPath,
			raw,
			0o600,
		); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	evidence, err := loadP14HistoricalMismatchEvidence(cleanRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyP14HistoricalSourceExtract(
		cleanRoot,
		evidence,
	); err != nil {
		t.Fatalf(
			"tracked source extract did not replay without ignored carriers: %v",
			err,
		)
	}
	if err := verifyP14HistoricalSourceCarriersWhenPresent(
		cleanRoot,
		evidence,
	); err != nil {
		t.Fatalf("absent ignored source carrier probe failed: %v", err)
	}

	extractPath := filepath.Join(
		cleanRoot,
		filepath.FromSlash(p14HistoricalMismatchExtractPath),
	)
	if err := os.Remove(extractPath); err != nil {
		t.Fatal(err)
	}
	if err := verifyP14HistoricalSourceExtract(
		cleanRoot,
		evidence,
	); err == nil ||
		!strings.Contains(
			err.Error(),
			"historical P14 source extract unavailable",
		) {
		t.Fatalf("missing tracked source extract did not fail closed: %v", err)
	}

	extract, err := loadP14HistoricalMismatchSourceExtract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	tamperedPayload := slices.Clone(extract.PayloadBase64)
	tamperedPayload[0] = "A" + tamperedPayload[0][1:]
	extract.PayloadBase64 = tamperedPayload
	if _, err := decodeP14HistoricalMismatchExtractPayload(extract); err == nil ||
		!strings.Contains(err.Error(), "compressed source extract digest differs") {
		t.Fatalf("tampered source extract payload did not fail closed: %v", err)
	}
}

func TestP14HistoricalAdmissionDispositionSeparatesRuntimeFromSemantics(
	t *testing.T,
) {
	evidence := mustP14HistoricalMismatchEvidence(t)
	positive := mustP14HistoricalMismatchFamily(
		t,
		evidence.Families,
		"positive_typed_write",
	)
	applied := p14HistoricalAdmissionProcessResult(
		t,
		positive,
		"admit",
		"",
	)
	committed, err := p14InstalledCLIAdmissionResult(applied)
	if err != nil {
		t.Fatalf("normalize applied admission: %v", err)
	}
	if committed.Disposition != "committed" {
		t.Fatalf(
			"applied disposition normalized to %q, want committed",
			committed.Disposition,
		)
	}

	replay := p14HistoricalAdmissionProcessResult(
		t,
		positive,
		"replay",
		"",
	)
	replayed, err := p14InstalledCLIAdmissionResult(replay)
	if err != nil {
		t.Fatalf("normalize replay admission: %v", err)
	}
	if replayed.Disposition != "replay" {
		t.Fatalf(
			"replay disposition normalized to %q",
			replayed.Disposition,
		)
	}
	if !bytes.Equal(committed.Receipt, replayed.Receipt) {
		t.Fatal("historical applied/replay fixture lost the exact receipt")
	}

	unsupported := p14HistoricalAdmissionProcessResult(
		t,
		positive,
		"admit",
		"committed",
	)
	if _, err := p14InstalledCLIAdmissionResult(unsupported); err == nil ||
		!strings.Contains(err.Error(), "disposition is unsupported") {
		t.Fatalf(
			"raw semantic token was not rejected at runtime boundary: %v",
			err,
		)
	}

	concurrency := mustP14HistoricalMismatchFamily(
		t,
		evidence.Families,
		"concurrency_idempotency",
	)
	commitCount := 0
	var firstReceipt []byte
	for _, commandID := range []string{"writer_a", "writer_b", "replay"} {
		result := p14HistoricalAdmissionProcessResult(
			t,
			concurrency,
			commandID,
			"",
		)
		normalized, normalizeErr := p14InstalledCLIAdmissionResult(result)
		if normalizeErr != nil {
			t.Fatalf("normalize captured %s: %v", commandID, normalizeErr)
		}
		if normalized.Disposition == "committed" {
			commitCount++
		}
		if firstReceipt == nil {
			firstReceipt = normalized.Receipt
		}
		if !bytes.Equal(firstReceipt, normalized.Receipt) {
			t.Fatalf("captured concurrency receipt differs for %s", commandID)
		}
	}
	if commitCount != 1 {
		t.Fatalf("captured concurrency commit count = %d, want 1", commitCount)
	}
}

func TestP14HistoricalNegativeValidationFixturesExerciseClosedOracles(
	t *testing.T,
) {
	evidence := mustP14HistoricalMismatchEvidence(t)
	memoryFixture := syntheticP14MemoryReadFixture()
	operations := syntheticP14MemoryOperationFixture(memoryFixture)

	historicalInvalid := mustP14HistoricalProjectionMap(
		t,
		mustP14HistoricalMismatchFamily(
			t,
			evidence.Families,
			"invalid",
		).Projection,
	)
	historicalInvalidRequest := mustP14HistoricalNestedMap(
		t,
		historicalInvalid,
		"request",
	)
	if historicalInvalidRequest["entity_id"] ==
		evidence.Source.KnownEntity.ReferenceID {
		t.Fatal("historical invalid oracle already targeted the known entity")
	}
	if historicalInvalidRequest["context"] !=
		evidence.Source.KnownEntity.BoundedContextRef {
		t.Fatal("historical invalid oracle did not use the admitted context")
	}

	invalidCase, err := p14MemoryOperationFixtureCaseByBuilder(
		operations.Cases,
		"memory.validate-invalid.v2",
	)
	if err != nil {
		t.Fatal(err)
	}
	invalidDeclaration := p14HistoricalDeclaredEntity(
		t,
		invalidCase.Steps[0].Request,
	)
	if invalidDeclaration["entity_id"] !=
		memoryFixture.EntityRef.ReferenceID {
		t.Fatalf(
			"invalid fixture entity = %#v, want exact existing reference %q",
			invalidDeclaration["entity_id"],
			memoryFixture.EntityRef.ReferenceID,
		)
	}
	if invalidDeclaration["context"] != memoryFixture.BoundedContext {
		t.Fatalf(
			"invalid fixture context = %#v, want admitted context %q",
			invalidDeclaration["context"],
			memoryFixture.BoundedContext,
		)
	}

	historicalUnderdetermined := mustP14HistoricalProjectionMap(
		t,
		mustP14HistoricalMismatchFamily(
			t,
			evidence.Families,
			"underdetermined",
		).Projection,
	)
	historicalUnderdeterminedRequest := mustP14HistoricalNestedMap(
		t,
		historicalUnderdetermined,
		"request",
	)
	if historicalUnderdeterminedRequest["context"] !=
		evidence.Source.KnownEntity.BoundedContextRef {
		t.Fatal(
			"historical underdetermined oracle did not use the admitted context",
		)
	}

	underdeterminedCase, err := p14MemoryOperationFixtureCaseByBuilder(
		operations.Cases,
		"memory.validate-underdetermined.v2",
	)
	if err != nil {
		t.Fatal(err)
	}
	underdeterminedDeclaration := p14HistoricalDeclaredEntity(
		t,
		underdeterminedCase.Steps[0].Request,
	)
	if underdeterminedDeclaration["context"] !=
		"p14-unadmitted-bounded-context" {
		t.Fatalf(
			"underdetermined fixture context = %#v",
			underdeterminedDeclaration["context"],
		)
	}
	if underdeterminedDeclaration["context"] ==
		memoryFixture.BoundedContext {
		t.Fatal(
			"underdetermined fixture still uses the admitted bounded context",
		)
	}
}

func TestP14HistoricalBackfillPrettyJSONUsesDedicatedDocumentParser(
	t *testing.T,
) {
	evidence := mustP14HistoricalMismatchEvidence(t)
	backfill := mustP14HistoricalMismatchFamily(
		t,
		evidence.Families,
		"existing_record_backfill",
	)
	disposition, err := classifyP14HistoricalMismatch(
		evidence.Source,
		backfill,
	)
	if err != nil {
		t.Fatal(err)
	}
	if disposition != p14HistoricalMismatchDispositionOracle {
		t.Fatalf("captured backfill mismatch classified %d", disposition)
	}

	semantic := p14BackfillRegressionSemantic()
	results := p14BackfillRegressionProcessResults(t, semantic)
	err = normalizeP14InstalledCLIExistingRecordBackfill(
		semantic,
		results,
		"project-before-and-after",
		"project-before-and-after",
		"home-before",
		"home-after",
	)
	if err != nil {
		t.Fatalf("normalize pretty backfill documents: %v", err)
	}

	if _, _, err := p14InstalledCLIJSONResult(results[0]); err == nil ||
		!strings.Contains(err.Error(), "not compact JSON") {
		t.Fatalf(
			"strict compact parser accepted pretty backfill JSON: %v",
			err,
		)
	}

	withTrailingDocument := results[0]
	withTrailingDocument.Stdout = append(
		slices.Clone(withTrailingDocument.Stdout),
		[]byte("{}\n")...,
	)
	if _, _, err := p14InstalledCLIJSONDocumentResult(
		withTrailingDocument,
	); err == nil ||
		!strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf(
			"backfill document parser accepted trailing JSON: %v",
			err,
		)
	}
}

func p14HistoricalAdmissionProcessResult(
	t *testing.T,
	family p14HistoricalMismatchFamilyEvidence,
	commandID string,
	overrideDisposition string,
) p14InstalledCLIProcessResult {
	t.Helper()
	projection := mustP14HistoricalProjectionMap(t, family.Projection)
	observed := mustP14HistoricalNestedMap(t, projection, "observed")
	command := mustP14HistoricalNestedMap(t, observed, commandID)
	disposition, valid := command["disposition"].(string)
	if !valid || disposition == "" {
		t.Fatalf("captured %s disposition is absent", commandID)
	}
	if overrideDisposition != "" {
		disposition = overrideDisposition
	}
	receipt := mustP14HistoricalNestedMap(t, command, "receipt")
	payload := map[string]any{
		"action":          "admit",
		"authority_class": "non_binding_semantic_assertion",
		"persistence_disposition": map[string]any{
			"authority_granted": false,
			"disposition":       disposition,
			"mode":              "transactional_project_memory_commit",
		},
		"receipt": receipt,
		"result":  command["result"],
	}
	stdout, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	stdout = append(stdout, '\n')
	return p14InstalledCLIProcessResult{
		ExitCode: 0,
		Stdout:   stdout,
	}
}

func p14HistoricalDeclaredEntity(
	t *testing.T,
	request json.RawMessage,
) map[string]any {
	t.Helper()
	payload, err := p14MemoryOperationRequestMap(request)
	if err != nil {
		t.Fatal(err)
	}
	changeSet, valid := payload["change_set"].(map[string]any)
	if !valid {
		t.Fatal("historical fixture has no change_set")
	}
	changes, valid := changeSet["changes"].([]any)
	if !valid || len(changes) != 1 {
		t.Fatalf("historical fixture changes = %#v", changeSet["changes"])
	}
	declaration, valid := changes[0].(map[string]any)
	if !valid || declaration["kind"] != "declare_entity" {
		t.Fatalf("historical fixture declaration = %#v", changes[0])
	}
	return declaration
}

func p14BackfillRegressionSemantic() p14ExistingRecordBackfillSemanticRequest {
	return p14ExistingRecordBackfillSemanticRequest{
		Calls: []p14ExistingRecordBackfillSemanticCall{
			p14ExistingRecordBackfillCall("dry_run", "dry_run"),
			p14ExistingRecordBackfillCall("apply", "apply"),
			p14ExistingRecordBackfillCall("replay", "dry_run"),
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
		},
	}
}

func p14BackfillRegressionProcessResults(
	t *testing.T,
	semantic p14ExistingRecordBackfillSemanticRequest,
) []p14InstalledCLIProcessResult {
	t.Helper()
	revisions := [][2]uint64{
		{4, 4},
		{4, 5},
		{5, 5},
	}
	results := make(
		[]p14InstalledCLIProcessResult,
		0,
		len(semantic.Calls),
	)
	for index, call := range semantic.Calls {
		expected := semantic.Expected.Calls[index]
		route := map[string]any{
			"result": expected.Result,
		}
		if expected.DurableChangeCount > 0 {
			route["projection_report"] = map[string]any{
				"durable_change_count": expected.DurableChangeCount,
			}
		}
		payload := map[string]any{
			"authority_boundary":     p14BackfillRegressionAuthorityBoundary(call.Request.Mode),
			"contract_version":       p14ExistingRecordBackfillContractVersion,
			"graph_revision_after":   revisions[index][1],
			"graph_revision_before":  revisions[index][0],
			"mode":                   call.Request.Mode,
			"request_provenance_ref": call.Request.RequestProvenanceRef,
			"routes":                 []any{route},
		}
		stdout, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		stdout = append(stdout, '\n')
		results = append(results, p14InstalledCLIProcessResult{
			ID:       call.ID,
			ExitCode: 0,
			Stdout:   stdout,
		})
	}
	return results
}

func p14BackfillRegressionAuthorityBoundary(
	mode string,
) map[string]any {
	mutation := "validation_only_zero_write"
	if mode == "apply" {
		mutation = "explicit_selected_non_binding_projection_admission"
	}
	return map[string]any{
		"decision":       "not_decision_binding_or_supersession",
		"evidence_truth": "not_evidence_truth_or_quality",
		"mutation":       mutation,
		"performed_work": "not_performed_work_or_completion",
		"publication":    "not_publication_or_release",
		"schema":         "not_schema_declaration_or_activation",
		"specification":  "not_specification_approval_reopen_or_rebaseline",
		"type_env_head":  "not_typeenv_head_selection_or_mutation",
	}
}
