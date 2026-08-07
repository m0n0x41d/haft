package fpfrefresh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
)

// LocalPracticeRebaseResult describes the bounded source-coordinate update
// applied to the repo-owned Local-Practice candidate. It is not activation,
// admission, approval, or a semantic binding.
type LocalPracticeRebaseResult struct {
	Changed        bool
	BaseTypeEnvRef string
	SourcePinCount int
}

// RebaseLocalPracticeCandidate updates only the exact FPF Base TypeEnv and
// FPF-Spec source pins, then proves that the resulting carrier parses,
// compiles, seals, verifies, and links against the current embedded base.
func RebaseLocalPracticeCandidate(
	path string,
	databasePath string,
	sourceRevision string,
	specDigest string,
) (LocalPracticeRebaseResult, error) {
	base, err := loadBaseTypeEnvArtifact(databasePath)
	if err != nil {
		return LocalPracticeRebaseResult{}, err
	}
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		return LocalPracticeRebaseResult{}, fmt.Errorf(
			"current FPF Base TypeEnv has no executable reference",
		)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return LocalPracticeRebaseResult{}, fmt.Errorf(
			"read latest Local-Practice candidate: %w",
			err,
		)
	}
	rebased, pinCount, err := rebaseLocalPracticeCandidateBytes(
		original,
		baseRef.String(),
		sourceRevision,
		specDigest,
	)
	if err != nil {
		return LocalPracticeRebaseResult{}, err
	}
	if err := verifyLocalPracticeCandidateBytes(
		rebased,
		base,
		baseRef.String(),
		sourceRevision,
		specDigest,
	); err != nil {
		return LocalPracticeRebaseResult{}, err
	}
	changed := !bytes.Equal(original, rebased)
	if changed {
		if err := replaceLocalPracticeCandidate(path, rebased); err != nil {
			return LocalPracticeRebaseResult{}, err
		}
	}
	return LocalPracticeRebaseResult{
		Changed:        changed,
		BaseTypeEnvRef: baseRef.String(),
		SourcePinCount: pinCount,
	}, nil
}

// VerifyLocalPracticeCandidateExact keeps release verification fail-closed if
// refresh updated source/DB/lock but left the repo-owned carrier on an older
// source basis.
func VerifyLocalPracticeCandidateExact(
	path string,
	databasePath string,
	sourceRevision string,
	specDigest string,
) error {
	base, err := loadBaseTypeEnvArtifact(databasePath)
	if err != nil {
		return err
	}
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		return fmt.Errorf("current FPF Base TypeEnv has no executable reference")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read latest Local-Practice candidate: %w", err)
	}
	return verifyLocalPracticeCandidateBytes(
		payload,
		base,
		baseRef.String(),
		sourceRevision,
		specDigest,
	)
}

func rebaseLocalPracticeCandidateBytes(
	payload []byte,
	baseTypeEnvRef string,
	sourceRevision string,
	specDigest string,
) ([]byte, int, error) {
	parsed, err := localpractice.Parse(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("decode latest Local-Practice candidate: %w", err)
	}
	lines := strings.Split(string(payload), "\n")
	carrier := parsed.Carrier()
	baseSource := carrier.BaseTypeEnvRef()
	if err := replaceLocalPracticeScalar(
		lines,
		baseSource.Span().Start(),
		"base_type_env_ref",
		baseSource.Value(),
		baseTypeEnvRef,
	); err != nil {
		return nil, 0, err
	}

	pinCount := 0
	for _, declaration := range carrier.Signature().Vocabulary().Declarations() {
		signature, ok := declaration.(localpractice.KindClassificationSignatureDeclaration)
		if !ok {
			continue
		}
		pin := signature.ReferenceScheme()
		if pin.CarrierRef().Value() != "fpf-source:FPF-Spec.md" {
			continue
		}
		if err := replaceLocalPracticeScalar(
			lines,
			pin.Edition().Span().Start(),
			"edition",
			pin.Edition().Value(),
			sourceRevision,
		); err != nil {
			return nil, 0, err
		}
		if err := replaceLocalPracticeScalar(
			lines,
			pin.Digest().Span().Start(),
			"digest",
			pin.Digest().Value(),
			specDigest,
		); err != nil {
			return nil, 0, err
		}
		pinCount++
	}
	if pinCount == 0 {
		return nil, 0, fmt.Errorf(
			"latest Local-Practice candidate has no fpf-source:FPF-Spec.md pins",
		)
	}
	return []byte(strings.Join(lines, "\n")), pinCount, nil
}

func replaceLocalPracticeScalar(
	lines []string,
	lineNumber uint64,
	key string,
	before string,
	after string,
) error {
	if lineNumber == 0 || lineNumber > uint64(len(lines)) {
		return fmt.Errorf("Local-Practice %s source line %d is outside the carrier", key, lineNumber)
	}
	index := int(lineNumber - 1)
	trimmed := strings.TrimSpace(lines[index])
	want := key + ": " + before
	if trimmed != want {
		return fmt.Errorf(
			"Local-Practice %s source line %d = %q, want %q",
			key,
			lineNumber,
			trimmed,
			want,
		)
	}
	indentLength := len(lines[index]) - len(strings.TrimLeft(lines[index], " \t"))
	indent := lines[index][:indentLength]
	lines[index] = indent + key + ": " + after
	return nil
}

func verifyLocalPracticeCandidateBytes(
	payload []byte,
	base typeenv.BaseTypeEnvArtifact,
	baseTypeEnvRef string,
	sourceRevision string,
	specDigest string,
) error {
	parsed, err := localpractice.Parse(payload)
	if err != nil {
		return fmt.Errorf("decode rebased Local-Practice candidate: %w", err)
	}
	carrier := parsed.Carrier()
	if carrier.BaseTypeEnvRef().Value() != baseTypeEnvRef {
		return fmt.Errorf(
			"Local-Practice base_type_env_ref %q differs from current base %q",
			carrier.BaseTypeEnvRef().Value(),
			baseTypeEnvRef,
		)
	}
	pinCount := 0
	for _, declaration := range carrier.Signature().Vocabulary().Declarations() {
		signature, ok := declaration.(localpractice.KindClassificationSignatureDeclaration)
		if !ok {
			continue
		}
		pin := signature.ReferenceScheme()
		if pin.CarrierRef().Value() != "fpf-source:FPF-Spec.md" {
			continue
		}
		if pin.Edition().Value() != sourceRevision || pin.Digest().Value() != specDigest {
			return fmt.Errorf(
				"Local-Practice %s FPF-Spec pin %s@%s differs from current %s@%s",
				signature.Symbol().Value(),
				pin.Edition().Value(),
				pin.Digest().Value(),
				sourceRevision,
				specDigest,
			)
		}
		pinCount++
	}
	if pinCount == 0 {
		return fmt.Errorf("Local-Practice candidate has no FPF-Spec pins")
	}
	resolution := projecttypeenv.ResolveManifestGraph(
		base,
		[]localpractice.ParsedCarrier{parsed},
	)
	if resolution.Rejected() {
		return fmt.Errorf("resolve rebased Local-Practice manifest: %v", resolution.Issues())
	}
	bundle, accepted := resolution.Bundle()
	if !accepted {
		return fmt.Errorf("resolved Local-Practice manifest has no bundle")
	}
	nodes := bundle.Nodes()
	if len(nodes) != 1 {
		return fmt.Errorf("resolved Local-Practice manifest nodes = %d, want 1", len(nodes))
	}
	ir, err := projecttypeenv.CompileProjectTypeEnvExtensionIR(nodes[0], nil)
	if err != nil {
		return fmt.Errorf("compile rebased Local-Practice extension: %w", err)
	}
	artifact, err := projecttypeenv.SealProjectTypeEnvExtension(ir)
	if err != nil {
		return fmt.Errorf("seal rebased Local-Practice extension: %w", err)
	}
	if err := artifact.Verify(); err != nil {
		return fmt.Errorf("verify rebased Local-Practice extension: %w", err)
	}
	linked := projecttypeenv.LinkProjectTypeEnvCompositeIR(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{artifact},
	)
	if linked.Rejected() {
		return fmt.Errorf("link rebased Local-Practice extension: %v", linked.Issues())
	}
	return nil
}

func loadBaseTypeEnvArtifact(path string) (typeenv.BaseTypeEnvArtifact, error) {
	database, err := openIntegrationDatabaseReadOnly(path)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	defer func() { _ = database.Close() }()
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, fmt.Errorf("load current FPF Base TypeEnv: %w", err)
	}
	return artifact, nil
}

func replaceLocalPracticeCandidate(path string, payload []byte) error {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("inspect latest Local-Practice candidate: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(cleanPath), ".local-practice-rebase-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Local-Practice candidate: %w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("set temporary Local-Practice mode: %w", err)
	}
	if _, err := temporary.Write(payload); err != nil {
		return fmt.Errorf("write temporary Local-Practice candidate: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("fsync temporary Local-Practice candidate: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary Local-Practice candidate: %w", err)
	}
	if err := os.Rename(temporaryPath, cleanPath); err != nil {
		return fmt.Errorf("replace Local-Practice candidate: %w", err)
	}
	keepTemporary = false
	directory, err := os.Open(filepath.Dir(cleanPath))
	if err != nil {
		return fmt.Errorf("open Local-Practice directory for fsync: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("fsync Local-Practice directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Local-Practice directory: %w", closeErr)
	}
	return nil
}
