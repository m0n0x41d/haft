package p13acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	p13VerifyEvidencePathEnvironmentKey   = "HAFT_P13_VERIFY_ACCEPTANCE_EVIDENCE"
	p13VerifyEvidenceDigestEnvironmentKey = "HAFT_P13_VERIFY_ACCEPTANCE_DIGEST"
)

func TestP13VerifyAcceptanceEvidenceFresh(t *testing.T) {
	if os.Getenv(p13ChildEnvironmentKey) == "1" {
		t.Skip("P13 child suite does not recursively verify acceptance evidence")
	}
	carrierPath := os.Getenv(p13VerifyEvidencePathEnvironmentKey)
	if carrierPath == "" {
		t.Skip("set HAFT_P13_VERIFY_ACCEPTANCE_EVIDENCE after consolidated P13 passes")
	}
	expectedDigest := os.Getenv(p13VerifyEvidenceDigestEnvironmentKey)
	if !validPrefixedSHA256(expectedDigest) {
		t.Fatal("HAFT_P13_VERIFY_ACCEPTANCE_DIGEST must be one sha256 digest")
	}
	cleanPath, err := cleanP13EvidencePath(carrierPath)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	evidence, raw, err := loadP13AcceptanceEvidence(repositoryRoot, cleanPath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256Prefixed(raw) != expectedDigest {
		t.Fatal("P13 acceptance evidence bytes differ from the expected carrier digest")
	}
	if err := validatePassingP13Evidence(evidence, cleanPath); err != nil {
		t.Fatal(err)
	}
	manifest, rawManifest, err := loadAcceptanceManifest(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAcceptanceManifest(repositoryRoot, manifest); err != nil {
		t.Fatal(err)
	}
	if err := validateManifestExecutionState(manifest); err != nil {
		t.Fatal(err)
	}
	if evidence.ManifestDigest != sha256Prefixed(rawManifest) {
		t.Fatal("P13 evidence manifest bytes are stale")
	}
	if err := validateP13EvidenceCoverage(evidence, manifest); err != nil {
		t.Fatal(err)
	}
	current, err := captureAcceptanceIdentity(repositoryRoot, manifest.Identity)
	if err != nil {
		t.Fatalf("recapture current P13 identity: %v", err)
	}
	if current.Digest != evidence.IdentityDigest {
		t.Fatal("P13 evidence identity is no longer current")
	}
	if err := validateCapturedFrozenBasis(manifest.FreezeInput, current); err != nil {
		t.Fatal(err)
	}
	inputs, err := prepareSuiteRuntimeInputs(repositoryRoot, manifest.Identity)
	if err != nil {
		t.Fatalf("recapture P13 suite runtime inputs: %v", err)
	}
	dependencyDigests, err := captureSuiteDependencyDigests(
		repositoryRoot,
		manifest,
		inputs,
		current,
	)
	if err != nil {
		t.Fatalf("recapture P13 suite dependency closures: %v", err)
	}
	if err := validateP13SuiteDependencyFreshness(
		evidence.Suites,
		dependencyDigests,
	); err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"P13_ACCEPTANCE_EVIDENCE_FRESH path=%s carrier_digest=%s identity_digest=%s",
		cleanPath,
		expectedDigest,
		current.Digest,
	)
}

func TestPassingP13EvidenceRejectsUnprovedAnchor(t *testing.T) {
	identityDigest := sha256Prefixed([]byte("identity"))
	evidence := acceptanceEvidence{
		Schema:            acceptanceEvidenceSchema,
		Status:            "passed",
		FinishedAt:        "2026-07-22T00:00:00Z",
		IdentityDigest:    identityDigest,
		IdentityUnchanged: true,
		StartIdentity:     acceptanceIdentity{Digest: identityDigest},
		EndIdentity:       acceptanceIdentity{Digest: identityDigest},
		Suites: []suiteEvidence{
			{
				ID:                 "suite",
				Kind:               "exec",
				Status:             "pass",
				Provenance:         suiteProvenanceExecuted,
				DependencyDigest:   sha256Prefixed([]byte("dependency")),
				InvocationCount:    1,
				InvocationDigest:   sha256Prefixed([]byte("invocation")),
				OutputDigest:       sha256Prefixed([]byte("output")),
				Invocations:        []commandInvocation{{Program: "test"}},
				PreIdentityDigest:  identityDigest,
				PostIdentityDigest: identityDigest,
				IdentityUnchanged:  true,
			},
		},
		Gates: []gateEvidence{
			{
				ID:     "G1",
				Status: "pass",
				Anchors: []anchorEvidence{
					{Key: "package::Test", PassedSuites: []string{"suite"}},
				},
			},
		},
	}
	carrierPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidence.CarrierPath = carrierPath
	if err := validatePassingP13Evidence(evidence, carrierPath); err != nil {
		t.Fatal(err)
	}
	evidence.Gates[0].Anchors[0].MissingSuites = []string{"suite"}
	if err := validatePassingP13Evidence(evidence, carrierPath); err == nil {
		t.Fatal("P13 freshness verifier accepted an unproved anchor")
	}
}

func TestPassingP13EvidenceAcceptsExactImportedSuiteProvenance(t *testing.T) {
	currentIdentity := sha256Prefixed([]byte("current-identity"))
	priorIdentity := sha256Prefixed([]byte("prior-identity"))
	dependencyDigest := sha256Prefixed([]byte("dependency"))
	evidence := acceptanceEvidence{
		Schema:            acceptanceEvidenceSchema,
		Status:            "passed",
		FinishedAt:        "2026-07-22T00:00:00Z",
		IdentityDigest:    currentIdentity,
		IdentityUnchanged: true,
		StartIdentity:     acceptanceIdentity{Digest: currentIdentity},
		EndIdentity:       acceptanceIdentity{Digest: currentIdentity},
		Suites: []suiteEvidence{
			{
				ID:                        "suite",
				Kind:                      "exec",
				Status:                    "pass",
				Provenance:                suiteProvenanceImported,
				DependencyDigest:          dependencyDigest,
				InvocationCount:           1,
				InvocationDigest:          sha256Prefixed([]byte("invocation")),
				OutputDigest:              sha256Prefixed([]byte("output")),
				Invocations:               []commandInvocation{{Program: "test"}},
				PreIdentityDigest:         priorIdentity,
				PostIdentityDigest:        priorIdentity,
				IdentityUnchanged:         true,
				ImportedFromCarrierPath:   ".context/p13/prior.json",
				ImportedFromCarrierDigest: sha256Prefixed([]byte("prior-carrier")),
			},
		},
		Gates: []gateEvidence{
			{
				ID:     "G1",
				Status: "pass",
				Anchors: []anchorEvidence{
					{Key: "package::Test", PassedSuites: []string{"suite"}},
				},
			},
		},
	}
	carrierPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidence.CarrierPath = carrierPath
	if err := validatePassingP13Evidence(evidence, carrierPath); err != nil {
		t.Fatalf("exact imported suite provenance rejected: %v", err)
	}
	evidence.Suites[0].ImportedFromCarrierDigest = ""
	if err := validatePassingP13Evidence(evidence, carrierPath); err == nil {
		t.Fatal("imported suite without a source carrier digest passed")
	}
}

func TestP13EvidenceCoverageRejectsBogusAnchorProofSuite(t *testing.T) {
	anchor := testAnchor{Package: "package", Test: "TestRequired"}
	manifest := acceptanceManifest{
		ResultSemantics: "test",
		SingleCommand:   "test",
		Suites: []suiteSpec{
			{ID: "go_normal", Kind: "go_test_all_non_desktop"},
			{
				ID:   "go_race",
				Kind: "go_test_race_critical",
				GoRaceCases: []goRaceCase{
					{
						Package: "package",
						Tests:   []string{"TestRequired"},
					},
				},
			},
		},
		Gates: []gateSpec{
			{
				ID:       "G1",
				SuiteIDs: []string{"go_normal", "go_race"},
				Anchors:  []testAnchor{anchor},
			},
		},
	}
	evidence := acceptanceEvidence{
		ResultSemantics:     manifest.ResultSemantics,
		ConsolidatedCommand: manifest.SingleCommand,
		Suites: []suiteEvidence{
			{ID: "go_normal", Kind: "go_test_all_non_desktop"},
			{ID: "go_race", Kind: "go_test_race_critical"},
		},
		Gates: []gateEvidence{
			{
				ID:     "G1",
				Suites: []string{"go_normal", "go_race"},
				Anchors: []anchorEvidence{
					{
						Key:          anchorKey(anchor),
						PassedSuites: []string{"go_normal", "go_race"},
					},
				},
			},
		},
	}
	if err := validateP13EvidenceCoverage(evidence, manifest); err != nil {
		t.Fatalf("exact anchor proof rejected: %v", err)
	}
	evidence.Gates[0].Anchors[0].PassedSuites = []string{"bogus"}
	if err := validateP13EvidenceCoverage(evidence, manifest); err == nil {
		t.Fatal("anchor proof by an undeclared suite passed")
	}
}

func cleanP13EvidencePath(value string) (string, error) {
	converted := filepath.FromSlash(value)
	clean := filepath.Clean(converted)
	directory := filepath.Join(".context", "p13")
	absolute := filepath.IsAbs(clean)
	parent := filepath.Dir(clean)
	if absolute || parent != directory {
		return "", fmt.Errorf("P13 acceptance evidence path is outside .context/p13")
	}
	return filepath.ToSlash(clean), nil
}

func loadP13AcceptanceEvidence(
	repositoryRoot string,
	carrierPath string,
) (acceptanceEvidence, []byte, error) {
	root, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		return acceptanceEvidence{}, nil, fmt.Errorf(
			"open P13 evidence repository root: %w",
			err,
		)
	}
	defer root.Close()
	raw, err := root.ReadFile(carrierPath)
	if err != nil {
		return acceptanceEvidence{}, nil, fmt.Errorf("read P13 acceptance evidence: %w", err)
	}
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var evidence acceptanceEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return acceptanceEvidence{}, nil, fmt.Errorf("decode P13 acceptance evidence: %w", err)
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if err != io.EOF {
		return acceptanceEvidence{}, nil, fmt.Errorf("P13 acceptance evidence has trailing JSON")
	}
	reencoded, err := json.Marshal(evidence)
	if err != nil {
		return acceptanceEvidence{}, nil, fmt.Errorf("reencode P13 acceptance evidence: %w", err)
	}
	if !bytes.Equal(raw, reencoded) {
		return acceptanceEvidence{}, nil, fmt.Errorf("P13 acceptance evidence is not canonical JSON")
	}
	return evidence, raw, nil
}

func validatePassingP13Evidence(
	evidence acceptanceEvidence,
	carrierPath string,
) error {
	wantPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		return err
	}
	if evidence.Schema != acceptanceEvidenceSchema ||
		evidence.Status != "passed" || evidence.ReleaseClaim ||
		evidence.CarrierPath != carrierPath || evidence.CarrierPath != wantPath ||
		!evidence.IdentityUnchanged || evidence.Failure != "" ||
		len(evidence.Waivers) != 0 ||
		evidence.IdentityDigest == "" ||
		evidence.StartIdentity.Digest != evidence.IdentityDigest ||
		evidence.EndIdentity.Digest != evidence.IdentityDigest {
		return fmt.Errorf("P13 acceptance evidence is not one passing unchanged result")
	}
	if len(evidence.Suites) == 0 || len(evidence.Gates) == 0 {
		return fmt.Errorf("P13 acceptance evidence has no suite or gate results")
	}
	for _, suite := range evidence.Suites {
		if err := validatePassingP13Suite(evidence.IdentityDigest, suite); err != nil {
			return fmt.Errorf("P13 evidence suite %q did not pass on one identity", suite.ID)
		}
	}
	for _, gate := range evidence.Gates {
		if gate.Status != "pass" {
			return fmt.Errorf("P13 evidence gate %q did not pass", gate.ID)
		}
		for _, anchor := range gate.Anchors {
			if len(anchor.PassedSuites) == 0 || len(anchor.MissingSuites) != 0 {
				return fmt.Errorf("P13 evidence gate %q has an unproved anchor", gate.ID)
			}
		}
	}
	return nil
}

func validatePassingP13Suite(
	currentIdentityDigest string,
	suite suiteEvidence,
) error {
	if suite.ID == "" ||
		suite.Kind == "" ||
		suite.Status != "pass" ||
		suite.Failure != "" ||
		!validPrefixedSHA256(suite.DependencyDigest) ||
		suite.InvocationCount <= 0 ||
		!validPrefixedSHA256(suite.InvocationDigest) ||
		!validPrefixedSHA256(suite.OutputDigest) ||
		len(suite.Invocations) == 0 ||
		!suite.IdentityUnchanged ||
		suite.PreIdentityDigest == "" ||
		suite.PreIdentityDigest != suite.PostIdentityDigest {
		return fmt.Errorf("suite result is incomplete or failed")
	}
	switch suite.Provenance {
	case suiteProvenanceExecuted:
		if suite.PreIdentityDigest != currentIdentityDigest ||
			suite.ImportedFromCarrierPath != "" ||
			suite.ImportedFromCarrierDigest != "" {
			return fmt.Errorf("executed suite identity or provenance is inconsistent")
		}
	case suiteProvenanceImported:
		if _, err := cleanP13EvidencePath(suite.ImportedFromCarrierPath); err != nil {
			return fmt.Errorf("imported suite carrier path: %w", err)
		}
		if !validPrefixedSHA256(suite.ImportedFromCarrierDigest) {
			return fmt.Errorf("imported suite carrier digest is invalid")
		}
	default:
		return fmt.Errorf("suite provenance %q is invalid", suite.Provenance)
	}
	return nil
}

func validateP13SuiteDependencyFreshness(
	suites []suiteEvidence,
	current map[string]string,
) error {
	for _, suite := range suites {
		digest, found := current[suite.ID]
		if !found {
			return fmt.Errorf(
				"P13 suite %q has no current dependency closure",
				suite.ID,
			)
		}
		if suite.DependencyDigest != digest {
			return fmt.Errorf(
				"P13 suite %q dependency closure is stale",
				suite.ID,
			)
		}
	}
	return nil
}

func validateP13EvidenceCoverage(
	evidence acceptanceEvidence,
	manifest acceptanceManifest,
) error {
	if evidence.ConsolidatedCommand != manifest.SingleCommand ||
		evidence.ResultSemantics != manifest.ResultSemantics ||
		len(evidence.Suites) != len(manifest.Suites) ||
		len(evidence.Gates) != len(manifest.Gates) {
		return fmt.Errorf("P13 evidence coverage differs from the frozen manifest")
	}
	for index, suite := range evidence.Suites {
		declared := manifest.Suites[index]
		if suite.ID != declared.ID || suite.Kind != declared.Kind {
			return fmt.Errorf("P13 evidence suite %d differs from the frozen manifest", index)
		}
	}
	for index, gate := range evidence.Gates {
		declared := manifest.Gates[index]
		if gate.ID != declared.ID || !slices.Equal(gate.Suites, declared.SuiteIDs) ||
			len(gate.Anchors) != len(declared.Anchors) {
			return fmt.Errorf("P13 evidence gate %d differs from the frozen manifest", index)
		}
		for anchorIndex, observed := range gate.Anchors {
			anchor := declared.Anchors[anchorIndex]
			expected := anchorKey(anchor)
			expectedProofSuites := gateGoTestSuites(
				declared,
				manifest.Suites,
				anchor,
			)
			if observed.Key != expected ||
				!slices.Equal(observed.PassedSuites, expectedProofSuites) ||
				len(observed.MissingSuites) != 0 {
				return fmt.Errorf(
					"P13 evidence gate %q anchor proof differs from the frozen manifest",
					gate.ID,
				)
			}
		}
	}
	return nil
}

func gateGoTestSuites(
	gate gateSpec,
	suites []suiteSpec,
	anchor testAnchor,
) []string {
	suiteByID := make(map[string]suiteSpec, len(suites))
	for _, suite := range suites {
		suiteByID[suite.ID] = suite
	}
	result := make([]string, 0)
	for _, suiteID := range gate.SuiteIDs {
		suite := suiteByID[suiteID]
		if !isGoTestSuite(suite.Kind) {
			continue
		}
		if !suiteCoversAnchor(suite, anchor) {
			continue
		}
		result = append(result, suiteID)
	}
	return result
}
