package p14acceptance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	buildinfo "debug/buildinfo"
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

const (
	p14ReleaseEvidenceEnvironmentKey    = "HAFT_P14_VERIFY_RELEASE_EVIDENCE"
	p14ReleaseCandidateEnvironmentKey   = "HAFT_P14_VERIFY_RELEASE_CANDIDATE_SHA"
	p14ReleaseVersionEnvironmentKey     = "HAFT_P14_VERIFY_RELEASE_VERSION"
	p14ReleaseP13DigestEnvironmentKey   = "HAFT_P14_VERIFY_RELEASE_P13_DIGEST"
	p14ReleaseFinalDigestEnvironmentKey = "HAFT_P14_VERIFY_RELEASE_FINAL_DIGEST"
	p14ReleaseEvidenceBundleSchema      = "haft.p14.release-evidence-bundle/v1"
	p14ReleaseEvidenceMaximumAge        = 24 * time.Hour
)

type p14ReleaseEvidenceBundle struct {
	Schema                    string                     `json:"schema"`
	Status                    string                     `json:"status"`
	CandidateSHA              string                     `json:"candidate_sha"`
	P13CarrierPath            string                     `json:"p13_carrier_path"`
	P13CarrierDigest          string                     `json:"p13_carrier_digest"`
	PreparedCarrierPath       string                     `json:"prepared_carrier_path"`
	PreparedCarrierDigest     string                     `json:"prepared_carrier_digest"`
	FinalCarrierPath          string                     `json:"final_carrier_path"`
	FinalCarrierDigest        string                     `json:"final_carrier_digest"`
	ValidUntil                string                     `json:"valid_until"`
	QualifiedArchive          string                     `json:"qualified_archive"`
	QualifiedMember           string                     `json:"qualified_member"`
	QualifiedExecutableDigest string                     `json:"qualified_executable_digest"`
	ReleaseArchives           []p14ReleaseArchiveBinding `json:"release_archives"`
}

type p14ReleaseArchiveBinding struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

func TestP14VerifyReleaseEvidenceBundle(t *testing.T) {
	bundlePath := os.Getenv(p14ReleaseEvidenceEnvironmentKey)
	if bundlePath == "" {
		t.Skip("set HAFT_P14_VERIFY_RELEASE_EVIDENCE to verify final release evidence")
	}
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	candidateSHA := os.Getenv(p14ReleaseCandidateEnvironmentKey)
	version := os.Getenv(p14ReleaseVersionEnvironmentKey)
	p13Digest := os.Getenv(p14ReleaseP13DigestEnvironmentKey)
	finalDigest := os.Getenv(p14ReleaseFinalDigestEnvironmentKey)
	if !fullP14GitRevision.MatchString(candidateSHA) ||
		!validP14ReleaseVersion(version) ||
		!validP14Digest(p13Digest) || !validP14Digest(finalDigest) {
		t.Fatal("P14 release verifier exact candidate/evidence inputs are absent")
	}
	if err := verifyP14ReleaseEvidenceBundle(
		root,
		bundlePath,
		candidateSHA,
		version,
		p13Digest,
		finalDigest,
	); err != nil {
		t.Fatal(err)
	}
}

func verifyP14ReleaseEvidenceBundle(
	repositoryRoot string,
	bundlePath string,
	candidateSHA string,
	expectedVersion string,
	p13Digest string,
	finalDigest string,
) error {
	bundle, bundleRoot, err := loadP14ReleaseEvidenceBundle(bundlePath)
	if err != nil {
		return err
	}
	if bundle.CandidateSHA != candidateSHA ||
		bundle.P13CarrierDigest != p13Digest ||
		bundle.FinalCarrierDigest != finalDigest {
		return fmt.Errorf("P14 release evidence expected identities differ")
	}
	contract, contractRaw, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		return err
	}
	stagedRepository := filepath.Join(bundleRoot, "repository")
	if err := os.MkdirAll(stagedRepository, 0o700); err != nil {
		return err
	}
	paths := []string{
		bundle.P13CarrierPath,
		bundle.PreparedCarrierPath,
		bundle.FinalCarrierPath,
	}
	for _, path := range paths {
		if !validP14ReleaseCarrierPath(path) {
			return fmt.Errorf("P14 release evidence carrier path %q is invalid", path)
		}
	}
	modulePath := filepath.Join(stagedRepository, "go.mod")
	if _, err := os.Stat(modulePath); os.IsNotExist(err) {
		if err := os.WriteFile(modulePath, []byte("module evidence.invalid\n"), 0o600); err != nil {
			return err
		}
	}
	contractPath := filepath.Join(
		stagedRepository,
		filepath.FromSlash(p14ContractRelativePath),
	)
	if err := os.MkdirAll(filepath.Dir(contractPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(contractPath, contractRaw, 0o600); err != nil {
		return err
	}
	p13Raw, err := readP14ReleaseCarrier(
		stagedRepository,
		bundle.P13CarrierPath,
		bundle.P13CarrierDigest,
	)
	if err != nil {
		return err
	}
	preparedRaw, err := readP14ReleaseCarrier(
		stagedRepository,
		bundle.PreparedCarrierPath,
		bundle.PreparedCarrierDigest,
	)
	if err != nil {
		return err
	}
	finalRaw, err := readP14ReleaseCarrier(
		stagedRepository,
		bundle.FinalCarrierPath,
		bundle.FinalCarrierDigest,
	)
	if err != nil {
		return err
	}
	prepared, err := decodePreparedRequestOracleCarrier(contract, preparedRaw)
	if err != nil {
		return err
	}
	if prepared.CarrierPath != bundle.PreparedCarrierPath ||
		prepared.Preparation.ContractDigest != p14Digest(contractRaw) ||
		prepared.Preparation.FrozenBasis.Candidate.GitHead != candidateSHA ||
		prepared.Preparation.P13Evidence.CarrierPath != bundle.P13CarrierPath ||
		prepared.Preparation.P13Evidence.CarrierDigest != p13Digest {
		return fmt.Errorf("P14 prepared release evidence basis differs")
	}
	if err := validateP14ReleaseCandidateVersion(
		prepared.Preparation,
		expectedVersion,
	); err != nil {
		return err
	}
	if err := verifyPreparedInputAgainstP13(
		stagedRepository,
		prepared.Preparation,
	); err != nil {
		return err
	}
	finalCarrier, err := decodeP14InstalledObservationCarrier(
		stagedRepository,
		contract,
		finalRaw,
	)
	if err != nil {
		return err
	}
	if finalCarrier.CarrierPath != bundle.FinalCarrierPath ||
		finalCarrier.Status != p14ObservationStatusPassed ||
		finalCarrier.Observation.Status != p14ObservationStatusPassed ||
		finalCarrier.Observation.PreparedCarrier.CarrierPath !=
			bundle.PreparedCarrierPath ||
		finalCarrier.Observation.PreparedCarrier.CarrierDigest !=
			bundle.PreparedCarrierDigest ||
		finalCarrier.Observation.Runtime.InstalledExecutableDigest !=
			prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest ||
		finalCarrier.Observation.Runtime.LiveMCPExecutableDigest !=
			prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest {
		return fmt.Errorf("P14 final release evidence identity differs")
	}
	if err := validateP14ReleaseEvidenceFreshness(
		finalCarrier,
		bundle.ValidUntil,
		time.Now().UTC(),
	); err != nil {
		return err
	}
	if err := validateP14ReleaseArchives(
		bundleRoot,
		bundle,
		prepared.Preparation.FrozenBasis.Candidate,
	); err != nil {
		return err
	}
	wantIDs := make([]string, 0, len(prepared.Preparation.Scenarios))
	for _, scenario := range prepared.Preparation.Scenarios {
		wantIDs = append(wantIDs, scenario.ID)
	}
	gotIDs := make([]string, 0, len(finalCarrier.Observation.ScenarioObservations))
	for _, observation := range finalCarrier.Observation.ScenarioObservations {
		if observation.Verdict != p14ObservationStatusPassed {
			return fmt.Errorf("P14 final release evidence retains failed scenario %q", observation.ID)
		}
		gotIDs = append(gotIDs, observation.ID)
	}
	if !slices.Equal(gotIDs, wantIDs) ||
		!bytes.Equal(p13Raw, mustReadP14ReleaseCarrier(stagedRepository, bundle.P13CarrierPath)) {
		return fmt.Errorf("P14 final release evidence scenario closure differs")
	}
	return nil
}

func validP14ReleaseVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func validateP14ReleaseCandidateVersion(
	prepared preparedRequestOracleInput,
	expected string,
) error {
	candidate := prepared.FrozenBasis.Candidate
	if !validP14ReleaseVersion(expected) || candidate.Version != expected ||
		validateP14CandidateBuildIdentity(candidate) != nil {
		return fmt.Errorf("P14 release candidate version differs")
	}
	return nil
}

func loadP14ReleaseEvidenceBundle(
	path string,
) (p14ReleaseEvidenceBundle, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return p14ReleaseEvidenceBundle{}, "", err
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return p14ReleaseEvidenceBundle{}, "", fmt.Errorf("read P14 release evidence bundle: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var bundle p14ReleaseEvidenceBundle
	if err := decoder.Decode(&bundle); err != nil {
		return p14ReleaseEvidenceBundle{}, "", fmt.Errorf("decode P14 release evidence bundle: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return p14ReleaseEvidenceBundle{}, "", fmt.Errorf("P14 release evidence bundle has trailing JSON")
	}
	canonical, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return p14ReleaseEvidenceBundle{}, "", err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) ||
		bundle.Schema != p14ReleaseEvidenceBundleSchema ||
		bundle.Status != p14ObservationStatusPassed ||
		!fullP14GitRevision.MatchString(bundle.CandidateSHA) ||
		!validP14Digest(bundle.P13CarrierDigest) ||
		!validP14Digest(bundle.PreparedCarrierDigest) ||
		!validP14Digest(bundle.FinalCarrierDigest) ||
		!validP14Digest(bundle.QualifiedExecutableDigest) ||
		len(bundle.ReleaseArchives) != 3 {
		return p14ReleaseEvidenceBundle{}, "", fmt.Errorf("P14 release evidence bundle header is invalid")
	}
	return bundle, filepath.Dir(absolute), nil
}

func validateP14ReleaseEvidenceFreshness(
	carrier p14InstalledObservationCarrier,
	validUntilRaw string,
	now time.Time,
) error {
	validUntil, err := time.Parse(time.RFC3339, validUntilRaw)
	if err != nil || validUntilRaw != validUntil.UTC().Format(time.RFC3339) {
		return fmt.Errorf("P14 release evidence valid-until is invalid")
	}
	oldest := time.Time{}
	for _, scenario := range carrier.Observation.ScenarioObservations {
		for _, surface := range scenario.SurfaceObservations {
			observedAt, err := time.Parse(time.RFC3339Nano, surface.ObservedAt)
			if err != nil {
				return fmt.Errorf("P14 release evidence observation time is invalid: %w", err)
			}
			observedAt = observedAt.UTC()
			if observedAt.After(now) {
				return fmt.Errorf("P14 release evidence observation is future-dated")
			}
			if oldest.IsZero() || observedAt.Before(oldest) {
				oldest = observedAt
			}
		}
	}
	if oldest.IsZero() {
		return fmt.Errorf("P14 release evidence has no observed surface time")
	}
	want := oldest.Truncate(time.Second).Add(p14ReleaseEvidenceMaximumAge)
	if !validUntil.Equal(want) || !now.Before(validUntil) {
		return fmt.Errorf("P14 release evidence is expired or has an open validity window")
	}
	return nil
}

func TestP14ReleaseEvidenceFreshnessAndVersionFailClosed(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	carrier := p14InstalledObservationCarrier{
		Observation: p14InstalledObservationInput{
			ScenarioObservations: []p14InstalledScenarioObservation{{
				SurfaceObservations: []p14InstalledSurfaceObservation{{
					ObservedAt: now.Add(-time.Hour).Format(time.RFC3339Nano),
				}},
			}},
		},
	}
	validUntil := now.Add(23 * time.Hour).Format(time.RFC3339)
	if err := validateP14ReleaseEvidenceFreshness(carrier, validUntil, now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*p14InstalledObservationCarrier) string{
		"expired": func(value *p14InstalledObservationCarrier) string {
			value.Observation.ScenarioObservations[0].SurfaceObservations[0].ObservedAt =
				now.Add(-25 * time.Hour).Format(time.RFC3339Nano)
			return now.Add(-time.Hour).Format(time.RFC3339)
		},
		"future_observation": func(value *p14InstalledObservationCarrier) string {
			value.Observation.ScenarioObservations[0].SurfaceObservations[0].ObservedAt =
				now.Add(time.Second).Format(time.RFC3339Nano)
			return now.Add(24*time.Hour + time.Second).Format(time.RFC3339)
		},
		"open_window": func(_ *p14InstalledObservationCarrier) string {
			return now.Add(48 * time.Hour).Format(time.RFC3339)
		},
	} {
		t.Run(name, func(t *testing.T) {
			mutated := carrier
			mutated.Observation.ScenarioObservations = slices.Clone(
				carrier.Observation.ScenarioObservations,
			)
			mutated.Observation.ScenarioObservations[0].SurfaceObservations = slices.Clone(
				carrier.Observation.ScenarioObservations[0].SurfaceObservations,
			)
			if err := validateP14ReleaseEvidenceFreshness(
				mutated,
				mutate(&mutated),
				now,
			); err == nil {
				t.Fatal("P14 release freshness accepted invalid semantic evidence")
			}
		})
	}
	clean := false
	prepared := preparedRequestOracleInput{
		FrozenBasis: frozenP14Basis{
			Candidate: candidateP14Basis{
				GitHead:          strings.Repeat("1", 40),
				Version:          "9.2.0",
				VersionCommit:    strings.Repeat("1", 40),
				VersionModified:  &clean,
				BuildVCS:         "git",
				BuildVCSRevision: strings.Repeat("1", 40),
				BuildVCSModified: &clean,
				BuildVCSTime:     p14SyntheticBuildVCSTime,
			},
		},
	}
	if err := validateP14ReleaseCandidateVersion(prepared, "9.2.0"); err != nil {
		t.Fatal(err)
	}
	if err := validateP14ReleaseCandidateVersion(prepared, "9.1.0"); err == nil {
		t.Fatal("P14 release verifier accepted another exact version")
	}
	prepared.FrozenBasis.Candidate.VersionCommit = strings.Repeat("2", 40)
	if err := validateP14ReleaseCandidateVersion(prepared, "9.2.0"); err == nil {
		t.Fatal("P14 release verifier accepted a different executable commit")
	}
	prepared.FrozenBasis.Candidate.VersionCommit = prepared.FrozenBasis.Candidate.GitHead
	modified := true
	prepared.FrozenBasis.Candidate.VersionModified = &modified
	if err := validateP14ReleaseCandidateVersion(prepared, "9.2.0"); err == nil {
		t.Fatal("P14 release verifier accepted a modified executable build")
	}
}

func TestP14ReleaseArchivesBindQualifiedExecutable(t *testing.T) {
	root := t.TempDir()
	repository := newP14BuildInfoTestRepository(t, "release-set")
	type archiveTarget struct {
		Name   string
		GOOS   string
		GOARCH string
	}
	targets := []archiveTarget{
		{Name: "haft-linux-amd64.tar.gz", GOOS: "linux", GOARCH: "amd64"},
		{Name: "haft-linux-arm64.tar.gz", GOOS: "linux", GOARCH: "arm64"},
		{Name: "haft-darwin-arm64.tar.gz", GOOS: "darwin", GOARCH: "arm64"},
	}
	bindings := make([]p14ReleaseArchiveBinding, 0, len(targets))
	archiveMembers := make(map[string][]byte, len(targets))
	var qualifiedExecutable []byte
	for _, target := range targets {
		_, member := repository.build(
			t,
			target.Name,
			target.GOOS,
			target.GOARCH,
			true,
			repository.Revision,
		)
		if target.Name == "haft-darwin-arm64.tar.gz" {
			qualifiedExecutable = member
		}
		archiveMembers[target.Name] = member
		path := filepath.Join(root, "release-artifacts", target.Name)
		writeP14ReleaseArchive(t, path, "haft", member)
		digest, err := digestP14File(path)
		if err != nil {
			t.Fatal(err)
		}
		bindings = append(bindings, p14ReleaseArchiveBinding{
			Name: target.Name, Path: "release-artifacts/" + target.Name,
			Digest: digest, GOOS: target.GOOS, GOARCH: target.GOARCH,
		})
	}
	clean := false
	candidate := candidateP14Basis{
		GitHead:          repository.Revision,
		Version:          p14RequiredCandidateVersion,
		VersionCommit:    repository.Revision,
		VersionModified:  &clean,
		BuildVCS:         "git",
		BuildVCSRevision: repository.Revision,
		BuildVCSModified: &clean,
		BuildVCSTime:     repository.VCSTime,
		ExecutableDigest: p14Digest(qualifiedExecutable),
	}
	bundle := p14ReleaseEvidenceBundle{
		QualifiedArchive:          "haft-darwin-arm64.tar.gz",
		QualifiedMember:           "haft",
		QualifiedExecutableDigest: candidate.ExecutableDigest,
		ReleaseArchives:           bindings,
	}
	if err := validateP14ReleaseArchives(
		root,
		bundle,
		candidate,
	); err != nil {
		t.Fatal(err)
	}
	wrong := bundle
	wrong.QualifiedExecutableDigest = p14TestDigest("another executable")
	if err := validateP14ReleaseArchives(
		root,
		wrong,
		candidate,
	); err == nil {
		t.Fatal("P14 release archives accepted another qualified executable")
	}

	withoutVCS := bundle
	withoutVCS.ReleaseArchives = slices.Clone(bundle.ReleaseArchives)
	_, withoutVCSMember := repository.build(
		t,
		"linux-amd64-without-vcs",
		"linux",
		"amd64",
		false,
		repository.Revision,
	)
	rewriteP14ReleaseArchiveBinding(
		t,
		root,
		&withoutVCS.ReleaseArchives[0],
		withoutVCSMember,
	)
	if err := validateP14ReleaseArchives(root, withoutVCS, candidate); err == nil {
		t.Fatal("P14 release archives accepted a member built with -buildvcs=false")
	}
	writeP14ReleaseArchive(
		t,
		filepath.Join(root, filepath.FromSlash(bundle.ReleaseArchives[0].Path)),
		"haft",
		archiveMembers[bundle.ReleaseArchives[0].Name],
	)

	mismatchedRepository := newP14BuildInfoTestRepository(t, "other-release-set")
	mismatched := bundle
	mismatched.ReleaseArchives = slices.Clone(bundle.ReleaseArchives)
	_, mismatchedMember := mismatchedRepository.build(
		t,
		"linux-arm64-mismatched-vcs",
		"linux",
		"arm64",
		true,
		mismatchedRepository.Revision,
	)
	rewriteP14ReleaseArchiveBinding(
		t,
		root,
		&mismatched.ReleaseArchives[1],
		mismatchedMember,
	)
	if err := validateP14ReleaseArchives(root, mismatched, candidate); err == nil {
		t.Fatal("P14 release archives accepted a member from another VCS revision")
	}

	unsafe := filepath.Join(root, "unsafe.tar.gz")
	writeP14ReleaseArchive(t, unsafe, "../haft", qualifiedExecutable)
	if _, err := readP14ReleaseArchiveMember(unsafe, "haft"); err == nil {
		t.Fatal("P14 release archive accepted an unsafe member")
	}
}

func rewriteP14ReleaseArchiveBinding(
	t *testing.T,
	root string,
	binding *p14ReleaseArchiveBinding,
	member []byte,
) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(binding.Path))
	writeP14ReleaseArchive(t, path, "haft", member)
	digest, err := digestP14File(path)
	if err != nil {
		t.Fatal(err)
	}
	binding.Digest = digest
}

func writeP14ReleaseArchive(
	t *testing.T,
	path string,
	member string,
	content []byte,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{
		Name: member, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func validateP14ReleaseArchives(
	bundleRoot string,
	bundle p14ReleaseEvidenceBundle,
	candidate candidateP14Basis,
) error {
	want := []p14ReleaseArchiveBinding{
		{Name: "haft-linux-amd64.tar.gz", Path: "release-artifacts/haft-linux-amd64.tar.gz", GOOS: "linux", GOARCH: "amd64"},
		{Name: "haft-linux-arm64.tar.gz", Path: "release-artifacts/haft-linux-arm64.tar.gz", GOOS: "linux", GOARCH: "arm64"},
		{Name: "haft-darwin-arm64.tar.gz", Path: "release-artifacts/haft-darwin-arm64.tar.gz", GOOS: "darwin", GOARCH: "arm64"},
	}
	if err := validateP14CandidateBuildIdentity(candidate); err != nil {
		return fmt.Errorf("P14 release candidate build identity differs: %w", err)
	}
	if !validP14Digest(candidate.ExecutableDigest) ||
		bundle.QualifiedArchive != "haft-darwin-arm64.tar.gz" ||
		bundle.QualifiedMember != "haft" ||
		bundle.QualifiedExecutableDigest != candidate.ExecutableDigest ||
		len(bundle.ReleaseArchives) != len(want) {
		return fmt.Errorf("P14 release archive qualification basis differs")
	}
	for index, archive := range bundle.ReleaseArchives {
		expected := want[index]
		if archive.Name != expected.Name || archive.Path != expected.Path ||
			archive.GOOS != expected.GOOS || archive.GOARCH != expected.GOARCH ||
			!validP14Digest(archive.Digest) {
			return fmt.Errorf("P14 release archive binding %d differs", index+1)
		}
		archivePath := filepath.Join(bundleRoot, filepath.FromSlash(archive.Path))
		if err := verifyP14FileDigest(archivePath, archive.Digest); err != nil {
			return err
		}
		member, err := readP14ReleaseArchiveMember(
			archivePath,
			bundle.QualifiedMember,
		)
		if err != nil {
			return err
		}
		info, err := buildinfo.Read(bytes.NewReader(member))
		if err != nil {
			return fmt.Errorf(
				"read P14 release archive %s embedded Go build identity: %w",
				archive.Name,
				err,
			)
		}
		provenance, err := p14BuildVCSProvenanceFromInfo(info)
		if err != nil {
			return fmt.Errorf(
				"validate P14 release archive %s embedded Go build identity: %w",
				archive.Name,
				err,
			)
		}
		if provenance.VCS != candidate.BuildVCS ||
			provenance.Revision != candidate.BuildVCSRevision ||
			candidate.BuildVCSModified == nil ||
			provenance.Modified != *candidate.BuildVCSModified ||
			provenance.Time != candidate.BuildVCSTime {
			return fmt.Errorf(
				"P14 release archive %s embedded Go VCS identity differs",
				archive.Name,
			)
		}
		if archive.Name == bundle.QualifiedArchive &&
			p14Digest(member) != bundle.QualifiedExecutableDigest {
			return fmt.Errorf("P14 qualified archive executable digest differs")
		}
	}
	return nil
}

func readP14ReleaseArchiveMember(path string, memberName string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open P14 release archive: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var member []byte
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read P14 release archive: %w", err)
		}
		clean := filepath.ToSlash(filepath.Clean(header.Name))
		if filepath.IsAbs(header.Name) || clean == ".." ||
			strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("P14 release archive contains an unsafe member")
		}
		if clean != memberName {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Mode&0o111 == 0 || member != nil ||
			header.Size <= 0 || header.Size > 256<<20 {
			return nil, fmt.Errorf("P14 qualified archive member is invalid")
		}
		member, err = io.ReadAll(io.LimitReader(reader, (256<<20)+1))
		if err != nil || int64(len(member)) != header.Size {
			return nil, fmt.Errorf("read P14 qualified archive member")
		}
	}
	if member == nil {
		return nil, fmt.Errorf("P14 qualified archive member is absent")
	}
	return member, nil
}

func validP14ReleaseCarrierPath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	portable := filepath.ToSlash(clean)
	return path == portable && !filepath.IsAbs(clean) &&
		!strings.HasPrefix(portable, "../") &&
		(filepath.Dir(clean) == filepath.Join(".context", "p13") ||
			filepath.Dir(clean) == filepath.Join(".context", "p14"))
}

func readP14ReleaseCarrier(
	root string,
	path string,
	digest string,
) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return nil, fmt.Errorf("read P14 release carrier %q: %w", path, err)
	}
	if p14Digest(raw) != digest {
		return nil, fmt.Errorf("P14 release carrier %q digest differs", path)
	}
	return raw, nil
}

func mustReadP14ReleaseCarrier(root string, path string) []byte {
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	return raw
}
