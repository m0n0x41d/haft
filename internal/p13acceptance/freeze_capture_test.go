package p13acceptance

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	p13FreezeCaptureEnvironmentKey = "HAFT_P13_CAPTURE_FREEZE_INPUT"
	p13FreezeVerifyEnvironmentKey  = "HAFT_P13_VERIFY_FREEZE_INPUT"
	p13FreezeCandidateSchema       = "haft.p13.freeze-input-candidate/v1"
	p13FreezeCandidatePosture      = "review_candidate_not_selection_or_evidence"
	p13FreezeCandidateSemantics    = "Read-only capture of an already activated current-source basis for exact P13 manifest review. This carrier does not activate a TypeEnv head, authorize Work, modify the manifest, pass P13, or establish evidence."
)

type freezeInputCandidate struct {
	Schema                 string          `json:"schema"`
	Posture                string          `json:"posture"`
	ResultSemantics        string          `json:"result_semantics"`
	CarrierPath            string          `json:"carrier_path"`
	ManifestPath           string          `json:"manifest_path"`
	ManifestDigest         string          `json:"manifest_digest"`
	CapturedIdentityDigest string          `json:"captured_identity_digest"`
	FreezeInput            freezeInputSpec `json:"freeze_input"`
	CandidateDigest        string          `json:"candidate_digest"`
}

type freezeInputCandidateDigestBasis struct {
	Schema                 string          `json:"schema"`
	Posture                string          `json:"posture"`
	ResultSemantics        string          `json:"result_semantics"`
	CarrierPath            string          `json:"carrier_path"`
	ManifestPath           string          `json:"manifest_path"`
	ManifestDigest         string          `json:"manifest_digest"`
	CapturedIdentityDigest string          `json:"captured_identity_digest"`
	FreezeInput            freezeInputSpec `json:"freeze_input"`
}

func TestP13CaptureFreezeInputCandidate(t *testing.T) {
	child := os.Getenv(p13ChildEnvironmentKey)
	if child == "1" {
		t.Skip("P13 child suite does not publish a freeze-input review candidate")
	}
	capture := os.Getenv(p13FreezeCaptureEnvironmentKey)
	if capture == "" {
		t.Skip("set HAFT_P13_CAPTURE_FREEZE_INPUT=1 after automatic compatible-successor activation")
	}
	if capture != "1" {
		t.Fatalf("%s must equal 1", p13FreezeCaptureEnvironmentKey)
	}
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	manifest, raw, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAcceptanceManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if !freezeInputCandidateCapturePostureAllowed(manifest) {
		t.Fatal("P13 manifest is neither awaiting freeze nor frozen for current-source review")
	}
	identity, err := captureAcceptanceIdentity(root, manifest.Identity)
	if err != nil {
		t.Fatalf("capture exact selected P13 identity: %v", err)
	}
	manifestDigest := sha256Prefixed(raw)
	freezeInput := freezeInputFromIdentity(identity)
	candidate, err := buildFreezeInputCandidate(
		manifestDigest,
		identity.Digest,
		freezeInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistFreezeInputCandidate(root, candidate)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"P13 freeze-input review candidate path=%s carrier_digest=%s candidate_digest=%s",
		path,
		digest,
		candidate.CandidateDigest,
	)
}

func TestP13VerifyFreezeInputCandidate(t *testing.T) {
	child := os.Getenv(p13ChildEnvironmentKey)
	if child == "1" {
		t.Skip("P13 child suite does not verify a freeze-input review candidate")
	}
	carrierPath := os.Getenv(p13FreezeVerifyEnvironmentKey)
	if carrierPath == "" {
		t.Skip("set HAFT_P13_VERIFY_FREEZE_INPUT to a captured carrier path")
	}
	cleanPath, err := cleanFreezeInputCandidatePath(carrierPath)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	candidate, carrierDigest, err := loadFreezeInputCandidate(
		repositoryRoot,
		cleanPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.CarrierPath != cleanPath {
		t.Fatal("P13 freeze-input candidate self-declared path differs")
	}
	manifest, rawManifest, err := loadAcceptanceManifest(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAcceptanceManifest(repositoryRoot, manifest); err != nil {
		t.Fatal(err)
	}
	if !freezeInputCandidateCapturePostureAllowed(manifest) {
		t.Fatal("P13 manifest is neither awaiting freeze nor frozen for current-source review")
	}
	manifestDigest := sha256Prefixed(rawManifest)
	if candidate.ManifestDigest != manifestDigest {
		t.Fatal("P13 freeze-input candidate was captured from different manifest bytes")
	}
	identity, err := captureAcceptanceIdentity(repositoryRoot, manifest.Identity)
	if err != nil {
		t.Fatalf("recapture exact selected P13 identity: %v", err)
	}
	if candidate.CapturedIdentityDigest != identity.Digest {
		t.Fatal("P13 freeze-input candidate identity is stale")
	}
	wantFreezeInput := freezeInputFromIdentity(identity)
	if candidate.FreezeInput != wantFreezeInput {
		t.Fatal("P13 freeze-input candidate differs from recaptured selected basis")
	}
	t.Logf(
		"P13 freeze-input review candidate verified path=%s carrier_digest=%s candidate_digest=%s",
		cleanPath,
		carrierDigest,
		candidate.CandidateDigest,
	)
}

func TestFreezeInputCandidateCapturePostureAllowsPendingAndFrozen(t *testing.T) {
	pending := acceptanceManifest{
		Status: manifestStatusPendingActivation,
		FreezeInput: freezeInputSpec{
			Posture: freezePosturePendingActivation,
		},
	}
	frozen := acceptanceManifest{
		Status:      manifestStatusFrozen,
		FreezeInput: completeFrozenInputForTest(),
	}
	invalid := acceptanceManifest{
		Status: manifestStatusPendingFinalSource,
		FreezeInput: freezeInputSpec{
			Posture: freezePosturePendingFinalSource,
		},
	}
	if !freezeInputCandidateCapturePostureAllowed(pending) {
		t.Fatal("pending automatic activation cannot publish a freeze-input review candidate")
	}
	if !freezeInputCandidateCapturePostureAllowed(frozen) {
		t.Fatal("frozen basis cannot publish a current-source review candidate")
	}
	if freezeInputCandidateCapturePostureAllowed(invalid) {
		t.Fatal("pending final source unexpectedly permits freeze-input capture")
	}
}

func TestFreezeInputCandidateIsNonAuthorizingAndNoClobber(t *testing.T) {
	digestCharacters := sha256.Size * 2
	manifestCharacters := strings.Repeat("1", digestCharacters)
	manifestDigest := "sha256:" + manifestCharacters
	identityCharacters := strings.Repeat("2", digestCharacters)
	identityDigest := "sha256:" + identityCharacters
	freezeInput := completeFrozenInputForTest()
	candidate, err := buildFreezeInputCandidate(
		manifestDigest,
		identityDigest,
		freezeInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Posture != p13FreezeCandidatePosture ||
		candidate.ResultSemantics != p13FreezeCandidateSemantics {
		t.Fatal("P13 freeze capture overstates selection, Work, or evidence")
	}
	if err := verifyFreezeInputCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	canonical, err := encodeFreezeInputCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeFreezeInputCandidate(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != candidate {
		t.Fatal("decoded freeze-input candidate differs")
	}
	objectEnd := []byte("\n}")
	unknownAuthority := []byte(",\n  \"authority\": true\n}")
	unknownField := bytes.Replace(
		canonical,
		objectEnd,
		unknownAuthority,
		1,
	)
	if _, err := decodeFreezeInputCandidate(unknownField); err == nil {
		t.Fatal("unknown freeze-input candidate field unexpectedly decoded")
	}
	trailing := bytes.Clone(canonical)
	trailingValue := []byte("{}\n")
	trailing = append(trailing, trailingValue...)
	if _, err := decodeFreezeInputCandidate(trailing); err == nil {
		t.Fatal("trailing freeze-input candidate value unexpectedly decoded")
	}
	root := t.TempDir()
	path, digest, err := persistFreezeInputCandidate(root, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if path != candidate.CarrierPath || digest == "" {
		t.Fatalf("persisted freeze candidate = %q %q", path, digest)
	}
	if _, _, err := persistFreezeInputCandidate(root, candidate); err == nil {
		t.Fatal("second freeze candidate publication replaced existing bytes")
	}
	tampered := candidate
	tampered.FreezeInput.HeadRevision++
	if err := verifyFreezeInputCandidate(tampered); err == nil {
		t.Fatal("tampered freeze candidate retained its digest")
	}
}

func freezeInputCandidateCapturePostureAllowed(manifest acceptanceManifest) bool {
	pending := manifest.Status == manifestStatusPendingActivation &&
		manifest.FreezeInput.Posture == freezePosturePendingActivation
	frozen := manifest.Status == manifestStatusFrozen &&
		manifest.FreezeInput.Posture == freezePostureSelectedAndFrozen
	return pending || frozen
}

func encodeFreezeInputCandidate(candidate freezeInputCandidate) ([]byte, error) {
	if err := verifyFreezeInputCandidate(candidate); err != nil {
		return nil, err
	}
	canonical, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode P13 freeze-input candidate: %w", err)
	}
	canonical = append(canonical, '\n')
	return canonical, nil
}

func decodeFreezeInputCandidate(raw []byte) (freezeInputCandidate, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var candidate freezeInputCandidate
	if err := decoder.Decode(&candidate); err != nil {
		return freezeInputCandidate{}, fmt.Errorf(
			"decode P13 freeze-input candidate: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return freezeInputCandidate{}, fmt.Errorf(
			"decode P13 freeze-input candidate trailing content",
		)
	}
	if err := verifyFreezeInputCandidate(candidate); err != nil {
		return freezeInputCandidate{}, err
	}
	canonical, err := encodeFreezeInputCandidate(candidate)
	if err != nil {
		return freezeInputCandidate{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return freezeInputCandidate{}, fmt.Errorf(
			"P13 freeze-input candidate is not canonical JSON",
		)
	}
	return candidate, nil
}

func buildFreezeInputCandidate(
	manifestDigest string,
	identityDigest string,
	freezeInput freezeInputSpec,
) (freezeInputCandidate, error) {
	path, err := freezeInputCandidateCarrierPath(identityDigest)
	if err != nil {
		return freezeInputCandidate{}, err
	}
	candidate := freezeInputCandidate{
		Schema:                 p13FreezeCandidateSchema,
		Posture:                p13FreezeCandidatePosture,
		ResultSemantics:        p13FreezeCandidateSemantics,
		CarrierPath:            path,
		ManifestPath:           manifestRelativePath,
		ManifestDigest:         manifestDigest,
		CapturedIdentityDigest: identityDigest,
		FreezeInput:            freezeInput,
	}
	basis := freezeInputCandidateDigestBasisFrom(candidate)
	digest, err := digestCanonicalJSON(basis)
	if err != nil {
		return freezeInputCandidate{}, fmt.Errorf(
			"digest P13 freeze-input candidate: %w",
			err,
		)
	}
	candidate.CandidateDigest = digest
	if err := verifyFreezeInputCandidate(candidate); err != nil {
		return freezeInputCandidate{}, err
	}
	return candidate, nil
}

func verifyFreezeInputCandidate(candidate freezeInputCandidate) error {
	if candidate.Schema != p13FreezeCandidateSchema ||
		candidate.Posture != p13FreezeCandidatePosture ||
		candidate.ResultSemantics != p13FreezeCandidateSemantics ||
		candidate.ManifestPath != manifestRelativePath {
		return fmt.Errorf("P13 freeze-input candidate semantics are invalid")
	}
	if !validPrefixedSHA256(candidate.ManifestDigest) ||
		!validPrefixedSHA256(candidate.CapturedIdentityDigest) ||
		!validPrefixedSHA256(candidate.CandidateDigest) {
		return fmt.Errorf("P13 freeze-input candidate digest is invalid")
	}
	wantPath, err := freezeInputCandidateCarrierPath(
		candidate.CapturedIdentityDigest,
	)
	if err != nil {
		return err
	}
	if candidate.CarrierPath != wantPath {
		return fmt.Errorf("P13 freeze-input candidate carrier path is invalid")
	}
	if err := validateFrozenExecutionInput(candidate.FreezeInput); err != nil {
		return fmt.Errorf("validate captured P13 freeze input: %w", err)
	}
	digestBasis := freezeInputCandidateDigestBasisFrom(candidate)
	wantDigest, err := digestCanonicalJSON(digestBasis)
	if err != nil {
		return fmt.Errorf("redigest P13 freeze-input candidate: %w", err)
	}
	if candidate.CandidateDigest != wantDigest {
		return fmt.Errorf("P13 freeze-input candidate digest mismatch")
	}
	return nil
}

func freezeInputCandidateDigestBasisFrom(
	candidate freezeInputCandidate,
) freezeInputCandidateDigestBasis {
	return freezeInputCandidateDigestBasis{
		Schema:                 candidate.Schema,
		Posture:                candidate.Posture,
		ResultSemantics:        candidate.ResultSemantics,
		CarrierPath:            candidate.CarrierPath,
		ManifestPath:           candidate.ManifestPath,
		ManifestDigest:         candidate.ManifestDigest,
		CapturedIdentityDigest: candidate.CapturedIdentityDigest,
		FreezeInput:            candidate.FreezeInput,
	}
}

func freezeInputCandidateCarrierPath(identityDigest string) (string, error) {
	if !validPrefixedSHA256(identityDigest) {
		return "", fmt.Errorf("P13 captured identity digest is invalid")
	}
	identity := strings.TrimPrefix(identityDigest, "sha256:")
	name := "p13-freeze-input-candidate-" + identity[:16] + ".json"
	path := filepath.Join(".context", "p13", name)
	return filepath.ToSlash(path), nil
}

func validPrefixedSHA256(value string) bool {
	raw := strings.TrimPrefix(value, "sha256:")
	if raw == value {
		return false
	}
	decoded, err := hex.DecodeString(raw)
	return err == nil && len(decoded) == sha256.Size
}

func cleanFreezeInputCandidatePath(value string) (string, error) {
	converted := filepath.FromSlash(value)
	clean := filepath.Clean(converted)
	directory := filepath.Join(".context", "p13")
	absolute := filepath.IsAbs(clean)
	parent := filepath.Dir(clean)
	if absolute || parent != directory {
		return "", fmt.Errorf("P13 freeze-input candidate path is outside .context/p13")
	}
	return filepath.ToSlash(clean), nil
}

func loadFreezeInputCandidate(
	repositoryRoot string,
	carrierPath string,
) (freezeInputCandidate, string, error) {
	cleanPath, err := cleanFreezeInputCandidatePath(carrierPath)
	if err != nil {
		return freezeInputCandidate{}, "", err
	}
	root, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		return freezeInputCandidate{}, "", fmt.Errorf(
			"open P13 freeze repository root: %w",
			err,
		)
	}
	defer root.Close()
	raw, err := root.ReadFile(cleanPath)
	if err != nil {
		return freezeInputCandidate{}, "", fmt.Errorf(
			"read P13 freeze-input candidate: %w",
			err,
		)
	}
	candidate, err := decodeFreezeInputCandidate(raw)
	if err != nil {
		return freezeInputCandidate{}, "", err
	}
	carrierDigest := sha256Prefixed(raw)
	return candidate, carrierDigest, nil
}

func persistFreezeInputCandidate(
	repositoryRoot string,
	candidate freezeInputCandidate,
) (string, string, error) {
	canonical, err := encodeFreezeInputCandidate(candidate)
	if err != nil {
		return "", "", err
	}
	root, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		return "", "", fmt.Errorf("open P13 freeze repository root: %w", err)
	}
	defer root.Close()
	directoryPath := filepath.Join(".context", "p13")
	directory := filepath.ToSlash(directoryPath)
	if err := root.MkdirAll(directory, 0o700); err != nil {
		return "", "", fmt.Errorf("create P13 freeze directory: %w", err)
	}
	freezeRoot, err := root.OpenRoot(directory)
	if err != nil {
		return "", "", fmt.Errorf("open P13 freeze directory: %w", err)
	}
	defer freezeRoot.Close()
	carrierPath := filepath.FromSlash(candidate.CarrierPath)
	finalName := filepath.Base(carrierPath)
	temporaryName, temporary, err := createFreezeCandidateTemporary(
		freezeRoot,
		finalName,
	)
	if err != nil {
		return "", "", err
	}
	temporaryPresent := true
	defer func() {
		if temporaryPresent {
			_ = freezeRoot.Remove(temporaryName)
		}
	}()
	if _, err := temporary.Write(canonical); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("write P13 freeze candidate temporary: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("sync P13 freeze candidate temporary: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", "", fmt.Errorf("close P13 freeze candidate temporary: %w", err)
	}
	if err := freezeRoot.Link(temporaryName, finalName); err != nil {
		return "", "", fmt.Errorf("publish P13 freeze candidate without replacement: %w", err)
	}
	if err := freezeRoot.Remove(temporaryName); err != nil {
		return "", "", fmt.Errorf("remove P13 freeze candidate temporary: %w", err)
	}
	temporaryPresent = false
	if err := syncEvidenceDirectory(freezeRoot, ".", "freeze candidate directory"); err != nil {
		return "", "", err
	}
	observed, err := freezeRoot.ReadFile(finalName)
	if err != nil {
		return "", "", fmt.Errorf("reread P13 freeze-input candidate: %w", err)
	}
	if !bytes.Equal(observed, canonical) {
		return "", "", fmt.Errorf("reread P13 freeze-input candidate changed bytes")
	}
	decoded, err := decodeFreezeInputCandidate(observed)
	if err != nil {
		return "", "", err
	}
	if decoded != candidate {
		return "", "", fmt.Errorf("reread P13 freeze-input candidate changed value")
	}
	carrierDigest := sha256Prefixed(canonical)
	return candidate.CarrierPath, carrierDigest, nil
}

func createFreezeCandidateTemporary(
	root *os.Root,
	finalName string,
) (string, *os.File, error) {
	for range 16 {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return "", nil, fmt.Errorf(
				"generate P13 freeze candidate temporary name: %w",
				err,
			)
		}
		suffix := hex.EncodeToString(random)
		name := "." + finalName + "." + suffix + ".tmp"
		file, err := root.OpenFile(
			name,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0o600,
		)
		if err == nil {
			return name, file, nil
		}
		if os.IsExist(err) {
			continue
		}
		return "", nil, fmt.Errorf(
			"create P13 freeze candidate temporary: %w",
			err,
		)
	}
	return "", nil, fmt.Errorf("create unique P13 freeze candidate temporary")
}
