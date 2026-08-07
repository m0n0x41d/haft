package specmigrationv2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const gitWitnessSchemaV1 = "haft.spec-migration-v2.git-witness/v1"

type GitSourceProvenanceWitness interface {
	ProjectRoot() ApplyProjectRoot
	SourceCarrier() SourceCarrierID
	DesignatedDigest() SourceDigest
	HeadCommit() GitCommitOID
	ResolutionRecord() ProvenanceRecordBinding
	CanonicalBytes() []byte
	Digest() SHA256
	gitSourceProvenanceWitnessVariant()
}

type gitSourceProvenanceWitness struct {
	projectRoot      ApplyProjectRoot
	sourceCarrier    SourceCarrierID
	designatedDigest SourceDigest
	headCommit       GitCommitOID
	origin           SourceEditionOrigin
	resolutionRecord ProvenanceRecordBinding
	canonical        []byte
	digest           SHA256
}

func (gitSourceProvenanceWitness) gitSourceProvenanceWitnessVariant() {}

func (witness gitSourceProvenanceWitness) ProjectRoot() ApplyProjectRoot {
	return witness.projectRoot
}

func (witness gitSourceProvenanceWitness) SourceCarrier() SourceCarrierID {
	return witness.sourceCarrier
}

func (witness gitSourceProvenanceWitness) DesignatedDigest() SourceDigest {
	return witness.designatedDigest
}

func (witness gitSourceProvenanceWitness) HeadCommit() GitCommitOID {
	return witness.headCommit
}

func (witness gitSourceProvenanceWitness) ResolutionRecord() ProvenanceRecordBinding {
	return witness.resolutionRecord
}

func (witness gitSourceProvenanceWitness) CanonicalBytes() []byte {
	return append([]byte{}, witness.canonical...)
}

func (witness gitSourceProvenanceWitness) Digest() SHA256 {
	return witness.digest
}

// VerifyGitSourceProvenance is a read-only Git witness. It proves the exact
// HEAD object, parent blob, selected working-tree bytes and binary-diff digest,
// plus the separately pinned source-designation record. It does not authorize
// migration apply.
func VerifyGitSourceProvenance(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
	provenance DesignatedSourceProvenance,
) (GitSourceProvenanceWitness, error) {
	if !projectRoot.valid() || !provenance.valid() {
		return nil, fmt.Errorf("git source-provenance witness input is invalid")
	}
	origin := provenance.origin
	originRoot := origin.ProjectRoot()
	originRootText := originRoot.String()
	projectRootText := projectRoot.String()
	if originRootText != projectRootText {
		return nil, fmt.Errorf("designated source provenance belongs to another project root")
	}
	if err := verifyCanonicalRealRoot(projectRootText); err != nil {
		return nil, err
	}
	repositoryRoot, err := resolveRepositoryRoot(ctx, projectRoot)
	if err != nil {
		return nil, err
	}
	head, err := resolveHeadCommit(ctx, repositoryRoot)
	if err != nil {
		return nil, err
	}
	if err := verifySourceEdition(ctx, repositoryRoot, head, origin); err != nil {
		return nil, err
	}
	if err := verifyProvenanceRecord(repositoryRoot, provenance.resolutionRecord); err != nil {
		return nil, err
	}
	witness := gitSourceProvenanceWitness{
		projectRoot:      repositoryRoot,
		sourceCarrier:    origin.Carrier(),
		designatedDigest: origin.DesignatedDigest(),
		headCommit:       head,
		origin:           origin,
		resolutionRecord: provenance.resolutionRecord,
	}
	encoded, err := encodeGitWitness(witness)
	if err != nil {
		return nil, err
	}
	witness.canonical = append([]byte{}, encoded...)
	witness.digest = DigestBytes(witness.canonical)
	return witness, nil
}

func resolveRepositoryRoot(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
) (ApplyProjectRoot, error) {
	projectRootText := projectRoot.String()
	observed, err := runGit(ctx, projectRootText, "rev-parse", "--show-toplevel")
	if err != nil {
		return ApplyProjectRoot{}, err
	}
	rootText := strings.TrimSuffix(string(observed), "\n")
	root, err := NewApplyProjectRoot(rootText)
	if err != nil {
		return ApplyProjectRoot{}, err
	}
	wanted, err := filepath.EvalSymlinks(projectRootText)
	if err != nil {
		return ApplyProjectRoot{}, fmt.Errorf("resolve migration project root: %w", err)
	}
	canonicalRootText := root.String()
	actual, err := filepath.EvalSymlinks(canonicalRootText)
	if err != nil {
		return ApplyProjectRoot{}, fmt.Errorf("resolve Git project root: %w", err)
	}
	if actual != wanted {
		return ApplyProjectRoot{}, fmt.Errorf("git top-level does not match the migration project root")
	}
	return projectRoot, nil
}

func resolveHeadCommit(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
) (GitCommitOID, error) {
	projectRootText := projectRoot.String()
	formatOutput, err := runGit(ctx, projectRootText, "rev-parse", "--show-object-format")
	if err != nil {
		return GitCommitOID{}, err
	}
	format := strings.TrimSpace(string(formatOutput))
	if format != "sha1" && format != "sha256" {
		return GitCommitOID{}, fmt.Errorf("unsupported Git object format %q", format)
	}
	oidOutput, err := runGit(ctx, projectRootText, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return GitCommitOID{}, err
	}
	oid := strings.TrimSpace(string(oidOutput))
	return NewGitCommitOID(format + ":" + oid)
}

func verifySourceEdition(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
	head GitCommitOID,
	origin SourceEditionOrigin,
) error {
	switch value := origin.(type) {
	case RepositoryEdition:
		return verifyRepositoryEdition(ctx, projectRoot, head, value)
	case WorkingTreeEdition:
		return verifyWorkingTreeEdition(ctx, projectRoot, head, value)
	default:
		return fmt.Errorf("unknown designated source-edition origin")
	}
}

func verifyRepositoryEdition(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
	head GitCommitOID,
	edition RepositoryEdition,
) error {
	commit := edition.CommitOID()
	commitText := commit.String()
	headText := head.String()
	if commitText != headText {
		return fmt.Errorf("repository edition commit does not match current HEAD")
	}
	carrier := edition.Carrier()
	blob, err := readGitBlob(ctx, projectRoot, commit, carrier)
	if err != nil {
		return err
	}
	designatedDigest := edition.DesignatedDigest()
	observedBlobDigest := SourceDigestOf(blob)
	if !observedBlobDigest.Equal(designatedDigest) {
		return fmt.Errorf("repository edition blob digest does not match designated source")
	}
	workingBytes, err := readCarrier(projectRoot, carrier)
	if err != nil {
		return err
	}
	observedWorkingDigest := SourceDigestOf(workingBytes)
	if !observedWorkingDigest.Equal(designatedDigest) {
		return fmt.Errorf("working source bytes differ from the designated repository edition")
	}
	return nil
}

func verifyWorkingTreeEdition(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
	head GitCommitOID,
	edition WorkingTreeEdition,
) error {
	parent := edition.Parent()
	parentCommit := parent.CommitOID()
	parentCommitText := parentCommit.String()
	headText := head.String()
	if parentCommitText != headText {
		return fmt.Errorf("working-tree parent commit does not match current HEAD")
	}
	carrier := edition.Carrier()
	blob, err := readGitBlob(ctx, projectRoot, parentCommit, carrier)
	if err != nil {
		return err
	}
	parentDigest := parent.DesignatedDigest()
	observedBlobDigest := SourceDigestOf(blob)
	if !observedBlobDigest.Equal(parentDigest) {
		return fmt.Errorf("working-tree parent blob digest does not match provenance")
	}
	workingBytes, err := readCarrier(projectRoot, carrier)
	if err != nil {
		return err
	}
	designatedDigest := edition.DesignatedDigest()
	observedWorkingDigest := SourceDigestOf(workingBytes)
	if !observedWorkingDigest.Equal(designatedDigest) {
		return fmt.Errorf("working-tree source bytes differ from the designated edition")
	}
	delta, err := readGitBinaryDelta(ctx, projectRoot, parentCommit, carrier)
	if err != nil {
		return err
	}
	observedDelta := WorktreeDeltaDigestOf(delta)
	deltaBinding := edition.Delta()
	expectedDelta := deltaBinding.Digest()
	observedDeltaText := observedDelta.String()
	expectedDeltaText := expectedDelta.String()
	if observedDeltaText != expectedDeltaText {
		return fmt.Errorf("working-tree binary-diff digest does not match provenance")
	}
	return nil
}

func readGitBlob(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
	commit GitCommitOID,
	carrier SourceCarrierID,
) ([]byte, error) {
	commitText := commit.String()
	commitParts := strings.SplitN(commitText, ":", 2)
	oid := commitParts[1]
	object := oid + ":" + carrier.String()
	root := projectRoot.String()
	return runGit(ctx, root, "show", object)
}

func readGitBinaryDelta(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
	commit GitCommitOID,
	carrier SourceCarrierID,
) ([]byte, error) {
	commitText := commit.String()
	commitParts := strings.SplitN(commitText, ":", 2)
	oid := commitParts[1]
	root := projectRoot.String()
	return runGit(
		ctx,
		root,
		"diff",
		"--binary",
		"--no-ext-diff",
		oid,
		"--",
		carrier.String(),
	)
}

func verifyProvenanceRecord(
	projectRoot ApplyProjectRoot,
	binding ProvenanceRecordBinding,
) error {
	ref := binding.Ref()
	refText := ref.String()
	carrier, err := NewSourceCarrierID(refText)
	if err != nil {
		return fmt.Errorf("provenance-record ref is not a repository-relative carrier: %w", err)
	}
	content, err := readCarrier(projectRoot, carrier)
	if err != nil {
		return err
	}
	observed := ProvenanceRecordDigestOf(content)
	expected := binding.Digest()
	observedText := observed.String()
	expectedText := expected.String()
	if observedText != expectedText {
		return fmt.Errorf("source-designation record digest does not match provenance")
	}
	return nil
}

func readCarrier(root ApplyProjectRoot, carrier SourceCarrierID) ([]byte, error) {
	rootText := root.String()
	carrierText := carrier.String()
	carrierPath := filepath.FromSlash(carrierText)
	path := filepath.Join(rootText, carrierPath)
	if err := verifyConfinedPathComponents(rootText, path); err != nil {
		return nil, err
	}
	content, err := readRegularFileNoFollow(path)
	if err != nil {
		return nil, fmt.Errorf("read provenance carrier %s: %w", carrierText, err)
	}
	return content, nil
}

func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	commandArgs := []string{"-C", root}
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return nil, fmt.Errorf("run Git command: %w", err)
	}
	stderr := string(exitError.Stderr)
	trimmedStderr := strings.TrimSpace(stderr)
	return nil, fmt.Errorf("run Git command: %w: %s", err, trimmedStderr)
}

type gitWitnessJSONV1 struct {
	Schema                 string `json:"schema"`
	ProjectRoot            string `json:"project_root"`
	SourceCarrier          string `json:"source_carrier"`
	DesignatedSourceDigest string `json:"designated_source_digest"`
	HeadCommit             string `json:"head_commit"`
	OriginKind             string `json:"origin_kind"`
	ParentCommit           string `json:"parent_commit,omitempty"`
	ParentSourceDigest     string `json:"parent_source_digest,omitempty"`
	DeltaFormat            string `json:"delta_format,omitempty"`
	DeltaDigest            string `json:"delta_digest,omitempty"`
	ResolutionRecordRef    string `json:"resolution_record_ref"`
	ResolutionRecordDigest string `json:"resolution_record_digest"`
}

func encodeGitWitness(witness gitSourceProvenanceWitness) ([]byte, error) {
	dto, err := gitWitnessToJSON(witness)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(dto)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func gitWitnessToJSON(witness gitSourceProvenanceWitness) (gitWitnessJSONV1, error) {
	projectRoot := witness.projectRoot.String()
	sourceCarrier := witness.sourceCarrier.String()
	designatedDigest := witness.designatedDigest.String()
	headCommit := witness.headCommit.String()
	recordRef := witness.resolutionRecord.Ref()
	recordDigest := witness.resolutionRecord.Digest()
	recordRefText := recordRef.String()
	recordDigestText := recordDigest.String()
	dto := gitWitnessJSONV1{
		Schema:                 gitWitnessSchemaV1,
		ProjectRoot:            projectRoot,
		SourceCarrier:          sourceCarrier,
		DesignatedSourceDigest: designatedDigest,
		HeadCommit:             headCommit,
		ResolutionRecordRef:    recordRefText,
		ResolutionRecordDigest: recordDigestText,
	}
	switch origin := witness.origin.(type) {
	case RepositoryEdition:
		originCommit := origin.CommitOID()
		originDigest := origin.DesignatedDigest()
		dto.OriginKind = "repository"
		dto.ParentCommit = originCommit.String()
		dto.ParentSourceDigest = originDigest.String()
	case WorkingTreeEdition:
		parent := origin.Parent()
		parentCommit := parent.CommitOID()
		parentDigest := parent.DesignatedDigest()
		delta := origin.Delta()
		deltaFormat := delta.Format()
		deltaDigest := delta.Digest()
		dto.OriginKind = "working_tree"
		dto.ParentCommit = parentCommit.String()
		dto.ParentSourceDigest = parentDigest.String()
		dto.DeltaFormat = string(deltaFormat)
		dto.DeltaDigest = deltaDigest.String()
	default:
		return gitWitnessJSONV1{}, fmt.Errorf("cannot encode unknown Git witness origin")
	}
	return dto, nil
}

func decodeGitWitness(content []byte) (gitSourceProvenanceWitness, error) {
	var dto gitWitnessJSONV1
	if err := unmarshalCanonicalJSON(content, &dto); err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	witness, err := gitWitnessFromJSON(dto)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	reencoded, err := encodeGitWitness(witness)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	if !bytes.Equal(content, reencoded) {
		return gitSourceProvenanceWitness{}, fmt.Errorf("git provenance witness is not canonical")
	}
	witness.canonical = append([]byte{}, reencoded...)
	witness.digest = DigestBytes(witness.canonical)
	return witness, nil
}

func gitWitnessFromJSON(dto gitWitnessJSONV1) (gitSourceProvenanceWitness, error) {
	if dto.Schema != gitWitnessSchemaV1 {
		return gitSourceProvenanceWitness{}, fmt.Errorf("unsupported Git witness schema %q", dto.Schema)
	}
	root, err := NewApplyProjectRoot(dto.ProjectRoot)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	projectRef, err := NewProjectRootRef(dto.ProjectRoot)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	carrier, err := NewSourceCarrierID(dto.SourceCarrier)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	designated, err := NewSourceDigest(dto.DesignatedSourceDigest)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	head, err := NewGitCommitOID(dto.HeadCommit)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	parentCommit, err := NewGitCommitOID(dto.ParentCommit)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	parentDigest, err := NewSourceDigest(dto.ParentSourceDigest)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	parent, err := NewRepositoryEdition(projectRef, parentCommit, carrier, parentDigest)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	origin, err := gitWitnessOriginFromJSON(dto, parent, designated)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	recordRef, err := NewProvenanceRecordRef(dto.ResolutionRecordRef)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	recordDigest, err := NewProvenanceRecordDigest(dto.ResolutionRecordDigest)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	record, err := NewProvenanceRecordBinding(recordRef, recordDigest)
	if err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	witness := gitSourceProvenanceWitness{
		projectRoot:      root,
		sourceCarrier:    carrier,
		designatedDigest: designated,
		headCommit:       head,
		origin:           origin,
		resolutionRecord: record,
	}
	if err := validateGitWitness(witness); err != nil {
		return gitSourceProvenanceWitness{}, err
	}
	return witness, nil
}

func gitWitnessOriginFromJSON(
	dto gitWitnessJSONV1,
	parent RepositoryEdition,
	designated SourceDigest,
) (SourceEditionOrigin, error) {
	switch dto.OriginKind {
	case "repository":
		if dto.DeltaFormat != "" || dto.DeltaDigest != "" {
			return nil, fmt.Errorf("repository Git witness cannot carry a working-tree delta")
		}
		return parent, nil
	case "working_tree":
		format := WorktreeDeltaFormat(dto.DeltaFormat)
		deltaDigest, err := NewWorktreeDeltaDigest(dto.DeltaDigest)
		if err != nil {
			return nil, err
		}
		delta, err := NewWorktreeDeltaBinding(format, deltaDigest)
		if err != nil {
			return nil, err
		}
		return NewWorkingTreeEdition(parent, designated, delta)
	default:
		return nil, fmt.Errorf("unknown Git witness origin %q", dto.OriginKind)
	}
}

func validateGitWitness(witness gitSourceProvenanceWitness) error {
	if !witness.projectRoot.valid() || !witness.sourceCarrier.valid() || !witness.designatedDigest.valid() {
		return fmt.Errorf("git witness project or source binding is invalid")
	}
	if !witness.headCommit.valid() || !witness.resolutionRecord.valid() {
		return fmt.Errorf("git witness provenance binding is invalid")
	}
	if _, err := NewDesignatedSourceProvenance(witness.origin, witness.resolutionRecord); err != nil {
		return fmt.Errorf("git witness origin is invalid: %w", err)
	}
	originRoot := witness.origin.ProjectRoot()
	originRootText := originRoot.String()
	witnessRootText := witness.projectRoot.String()
	if originRootText != witnessRootText {
		return fmt.Errorf("git witness origin belongs to another project root")
	}
	originCarrier := witness.origin.Carrier()
	originCarrierText := originCarrier.String()
	witnessCarrierText := witness.sourceCarrier.String()
	if originCarrierText != witnessCarrierText {
		return fmt.Errorf("git witness origin names another source carrier")
	}
	originDigest := witness.origin.DesignatedDigest()
	originDigestText := originDigest.String()
	witnessDigestText := witness.designatedDigest.String()
	if originDigestText != witnessDigestText {
		return fmt.Errorf("git witness origin binds another designated digest")
	}
	parent := witnessHeadFromOrigin(witness.origin)
	if parent.String() != witness.headCommit.String() {
		return fmt.Errorf("git witness HEAD does not match its source origin")
	}
	return nil
}

func validateGitWitnessAgainstProvenance(
	witness gitSourceProvenanceWitness,
	projectRoot ApplyProjectRoot,
	provenance DesignatedSourceProvenance,
) error {
	if err := validateGitWitness(witness); err != nil {
		return err
	}
	canonical, err := encodeGitWitness(witness)
	if err != nil {
		return err
	}
	digest := DigestBytes(canonical)
	if !bytes.Equal(witness.canonical, canonical) || !witness.digest.Equal(digest) {
		return fmt.Errorf("git witness canonical bytes or digest are not preserved")
	}
	expected := gitSourceProvenanceWitness{
		projectRoot:      projectRoot,
		sourceCarrier:    provenance.origin.Carrier(),
		designatedDigest: provenance.origin.DesignatedDigest(),
		headCommit:       witnessHeadFromOrigin(provenance.origin),
		origin:           provenance.origin,
		resolutionRecord: provenance.resolutionRecord,
	}
	expectedBytes, err := encodeGitWitness(expected)
	if err != nil {
		return err
	}
	actualBytes, err := encodeGitWitness(witness)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("persisted Git witness does not match the exact migration provenance")
	}
	return nil
}

func witnessHeadFromOrigin(origin SourceEditionOrigin) GitCommitOID {
	switch value := origin.(type) {
	case RepositoryEdition:
		return value.CommitOID()
	case WorkingTreeEdition:
		parent := value.Parent()
		return parent.CommitOID()
	default:
		return GitCommitOID{}
	}
}
