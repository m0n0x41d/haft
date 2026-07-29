package projecttypeenvselection

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

const historicalStageV3ProvenanceReference = "project-typeenv-stage-provenance:sha256:f3bbada0aa006ceb0ce1cffe0c1d4e674bb205fb2b29af0eeb0374c363a91206"

// reconstructedStageV2GenesisBase64 freezes bytes produced through the sealed
// v2 compatibility encoder retained in this repository. No original ledger
// row was recoverable, so this is explicitly producer-reconstructed evidence,
// not a claim that the bytes were observed in a historical project database.
//
//go:embed testdata/reconstructed_stage_v2_genesis.b64
var reconstructedStageV2GenesisBase64 string

// historicalStageV3GenesisBase64 freezes canonical bytes emitted by the
// pre-v4 producer. The source row carried the known historical store metadata
// label v2 while its canonical Stage schema and domain were v3.
//
//go:embed testdata/historical_stage_v3_genesis.b64
var historicalStageV3GenesisBase64 string

// historicalTransitionStageV5GzipBase64 freezes canonical bytes from the real
// qnt_e3149c17 head-revision 1 -> 2 transition recorded on 2026-07-23. The
// compressed carrier is transport-only; the assertion below pins the exact
// uncompressed Stage identity and canonical bytes.
//
//go:embed testdata/historical_transition_stage_v5.b64.gz
var historicalTransitionStageV5GzipBase64 string

// TestFrozenHistoricalV3GenesisStageDecodesAndReencodesExactly covers only one
// actual pre-v4 v3 Genesis Stage. It deliberately does not claim v2
// compatibility; that still requires an independent historical specimen.
func TestFrozenHistoricalV3GenesisStageDecodesAndReencodesExactly(t *testing.T) {
	fixture := loadFrozenStageBase64(
		t,
		"historical v3 Genesis",
		historicalStageV3GenesisBase64,
	)
	canonical := fixture.canonical
	stage := fixture.stage
	if len(canonical) != 3222 {
		t.Fatalf("frozen historical Stage length = %d; want 3222", len(canonical))
	}

	expectedRef, err := ParseProjectTypeEnvStageRef(
		"project-typeenv-stage:sha256:7ece59d3c0d4f59988749c180f52752a556635895a1f173415f942442e89f909",
	)
	if err != nil {
		t.Fatalf("parse expected historical Stage ref: %v", err)
	}
	if stage.Ref() != expectedRef {
		t.Fatalf("historical Stage ref = %q; want %q", stage.Ref().String(), expectedRef.String())
	}
	if stage.SchemaEdition() != ProjectTypeEnvStageSchemaEditionV3 ||
		stage.CompilerEdition() != StageCompilerEditionV3() ||
		stage.RevalidatorEdition() != StageRevalidatorEditionV3() ||
		stage.ProducerEdition() != StageProducerEditionV3() {
		t.Fatalf(
			"historical Stage editions = %q/%q/%q/%q; want exact v3 tuple",
			stage.SchemaEdition(),
			stage.CompilerEdition().String(),
			stage.RevalidatorEdition().String(),
			stage.ProducerEdition().String(),
		)
	}
	if _, ok := stage.Predecessor().(GenesisStagePredecessor); !ok {
		t.Fatalf("historical predecessor = %T; want tag-only v3 Genesis", stage.Predecessor())
	}
	expectedProvenance := mustHistoricalStageV3ProvenanceRef(t)
	provenance, ok := stage.HistoricalProvenance()
	if !ok || provenance != expectedProvenance {
		t.Fatalf(
			"historical provenance = %q, %t; want %q",
			provenance.String(),
			ok,
			expectedProvenance.String(),
		)
	}

	reader := stageReader{value: canonical}
	domain, err := reader.readString("domain")
	if err != nil {
		t.Fatalf("read frozen historical Stage domain: %v", err)
	}
	if domain != projectTypeEnvStageDomainV3 {
		t.Fatalf("historical Stage domain = %q; want %q", domain, projectTypeEnvStageDomainV3)
	}
	state, err := decodeProjectTypeEnvStageState(&reader, domain)
	if err != nil {
		t.Fatalf("decode frozen historical Stage state: %v", err)
	}
	if reader.remaining() != 0 {
		t.Fatalf("frozen historical Stage has %d unread bytes", reader.remaining())
	}
	reencoded, err := encodeProjectTypeEnvStageState(state)
	if err != nil {
		t.Fatalf("re-encode frozen historical Stage state: %v", err)
	}
	if !bytes.Equal(reencoded, canonical) ||
		!bytes.Equal(stage.CanonicalBytes(), canonical) {
		t.Fatal("frozen historical Stage did not preserve exact canonical bytes")
	}
}

func TestFrozenReconstructedV2GenesisStageDecodesAndReencodesExactly(
	t *testing.T,
) {
	fixture := loadFrozenStageBase64(
		t,
		"reconstructed v2 Genesis",
		reconstructedStageV2GenesisBase64,
	)
	canonical := fixture.canonical
	stage := fixture.stage
	if len(canonical) != 3830 {
		t.Fatalf("reconstructed v2 Stage length = %d; want 3830", len(canonical))
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) !=
		"be81a6023158a7452e1d7014e69377cdacb6d238ada0752f163786a14a7b0434" {
		t.Fatalf("reconstructed v2 Stage digest = %x", digest)
	}

	if err := stage.Verify(); err != nil {
		t.Fatalf("Verify reconstructed v2 Genesis Stage: %v", err)
	}
	if stage.SchemaEdition() != ProjectTypeEnvStageSchemaEditionV2 ||
		stage.CompilerEdition() != StageCompilerEditionV2() ||
		stage.RevalidatorEdition() != StageRevalidatorEditionV2() ||
		stage.ProducerEdition() != StageProducerEditionV2() {
		t.Fatalf(
			"reconstructed v2 Stage editions = %q/%q/%q/%q; want exact v2 tuple",
			stage.SchemaEdition(),
			stage.CompilerEdition().String(),
			stage.RevalidatorEdition().String(),
			stage.ProducerEdition().String(),
		)
	}
	predecessor, ok := stage.Predecessor().(legacyGenesisStagePredecessor)
	if !ok {
		t.Fatalf(
			"reconstructed v2 predecessor = %T; want private legacy Genesis",
			stage.Predecessor(),
		)
	}
	proof, ok := LegacyGenesisNoPriorHeadProof(predecessor)
	if !ok ||
		proof != mustNoPriorProofRef(t, "7") {
		t.Fatal("reconstructed v2 Stage lost its historical no-prior-head proof")
	}
	provenance, ok := stage.HistoricalProvenance()
	if !ok ||
		provenance != mustStageProvenanceRef(t, "b") {
		t.Fatal("reconstructed v2 Stage lost its historical provenance coordinate")
	}

	reader := stageReader{value: canonical}
	domain, err := reader.readString("domain")
	if err != nil {
		t.Fatalf("read reconstructed v2 Stage domain: %v", err)
	}
	if domain != projectTypeEnvStageDomainV2 {
		t.Fatalf(
			"reconstructed v2 Stage domain = %q; want %q",
			domain,
			projectTypeEnvStageDomainV2,
		)
	}
	state, err := decodeProjectTypeEnvStageState(&reader, domain)
	if err != nil {
		t.Fatalf("decode reconstructed v2 Stage state: %v", err)
	}
	if reader.remaining() != 0 {
		t.Fatalf(
			"reconstructed v2 Stage has %d unread bytes",
			reader.remaining(),
		)
	}
	reencoded, err := encodeProjectTypeEnvStageState(state)
	if err != nil {
		t.Fatalf("re-encode reconstructed v2 Stage state: %v", err)
	}
	if !bytes.Equal(reencoded, canonical) ||
		!bytes.Equal(stage.CanonicalBytes(), canonical) {
		t.Fatal("reconstructed v2 Stage did not preserve exact canonical bytes")
	}
}

func mustHistoricalStageV3ProvenanceRef(
	t *testing.T,
) ProjectTypeEnvStageProvenanceRef {
	t.Helper()
	provenance, err := ParseProjectTypeEnvStageProvenanceRef(
		historicalStageV3ProvenanceReference,
	)
	if err != nil {
		t.Fatalf("parse expected historical v3 provenance: %v", err)
	}
	return provenance
}

type frozenStageFixture struct {
	canonical []byte
	stage     ProjectTypeEnvStage
}

func mustLoadReconstructedStageV2Genesis(
	t *testing.T,
) ProjectTypeEnvStage {
	t.Helper()
	fixture := loadFrozenStageBase64(
		t,
		"reconstructed v2 Genesis",
		reconstructedStageV2GenesisBase64,
	)
	return fixture.stage
}

func mustLoadFrozenHistoricalStageV3Genesis(
	t *testing.T,
) ProjectTypeEnvStage {
	t.Helper()
	fixture := loadFrozenStageBase64(
		t,
		"historical v3 Genesis",
		historicalStageV3GenesisBase64,
	)
	return fixture.stage
}

func loadFrozenStageBase64(
	t *testing.T,
	label string,
	raw string,
) frozenStageFixture {
	t.Helper()
	encoded := strings.Join(strings.Fields(raw), "")
	canonical, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode frozen %s Stage fixture: %v", label, err)
	}
	stage, err := DecodeProjectTypeEnvStage(canonical)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvStage(%s): %v", label, err)
	}
	return frozenStageFixture{
		canonical: canonical,
		stage:     stage,
	}
}

func TestFrozenRealTransitionStageV5DecodesAndReencodesExactly(t *testing.T) {
	encoded := strings.Join(
		strings.Fields(historicalTransitionStageV5GzipBase64),
		"",
	)
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode frozen Transition carrier: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open frozen Transition carrier: %v", err)
	}
	canonical, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("inflate frozen Transition Stage: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close frozen Transition carrier: %v", err)
	}
	if len(canonical) != 137190 {
		t.Fatalf("frozen Transition Stage length = %d; want 137190", len(canonical))
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) !=
		"dbb9dcd1bde548da965a94336703541a43ee4a7e80df54021da514dc461cb19c" {
		t.Fatalf("frozen Transition Stage digest = %x", digest)
	}

	stage, err := DecodeProjectTypeEnvStage(canonical)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvStage(frozen Transition): %v", err)
	}
	if err := stage.Verify(); err != nil {
		t.Fatalf("Verify frozen Transition Stage: %v", err)
	}
	if stage.SchemaEdition() != ProjectTypeEnvStageSchemaEditionV5 {
		t.Fatalf(
			"frozen Transition schema = %q; want %q",
			stage.SchemaEdition(),
			ProjectTypeEnvStageSchemaEditionV5,
		)
	}
	predecessor, ok := stage.Predecessor().(TransitionStagePredecessor)
	if !ok {
		t.Fatalf(
			"frozen predecessor = %T; want TransitionStagePredecessor",
			stage.Predecessor(),
		)
	}
	if predecessor.HeadRevision().Value() != 1 ||
		predecessor.SelectedComposite().String() !=
			"typeenv:sha256:d6097b7231aee200a0b998bd4146496b796222917e1e16505ac897079b7f29c2" {
		t.Fatalf(
			"frozen predecessor = revision %d composite %q",
			predecessor.HeadRevision().Value(),
			predecessor.SelectedComposite().String(),
		)
	}
	if stage.VerifiedComposite().String() !=
		"typeenv:sha256:6dc594a9d5470701b583a6e0893cf75d89629a27673d7aecd34b0993979c6aaf" ||
		stage.GraphRevision().Value() != 3 {
		t.Fatalf(
			"frozen target = %q at graph revision %d",
			stage.VerifiedComposite().String(),
			stage.GraphRevision().Value(),
		)
	}

	stageReader := stageReader{value: canonical}
	domain, err := stageReader.readString("domain")
	if err != nil {
		t.Fatalf("read frozen Transition domain: %v", err)
	}
	if domain != projectTypeEnvStageDomainV5 {
		t.Fatalf(
			"frozen Transition domain = %q; want %q",
			domain,
			projectTypeEnvStageDomainV5,
		)
	}
	state, err := decodeProjectTypeEnvStageState(&stageReader, domain)
	if err != nil {
		t.Fatalf("decode frozen Transition Stage state: %v", err)
	}
	if stageReader.remaining() != 0 {
		t.Fatalf(
			"frozen Transition Stage has %d unread bytes",
			stageReader.remaining(),
		)
	}
	reencoded, err := encodeProjectTypeEnvStageState(state)
	if err != nil {
		t.Fatalf("re-encode frozen Transition Stage state: %v", err)
	}
	if !bytes.Equal(reencoded, canonical) ||
		!bytes.Equal(stage.CanonicalBytes(), canonical) {
		t.Fatal("frozen Transition Stage did not preserve exact canonical bytes")
	}
}
