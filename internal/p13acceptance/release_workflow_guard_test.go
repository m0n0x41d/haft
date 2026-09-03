package p13acceptance

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	releaseGuardCandidateSHA = "1111111111111111111111111111111111111111"
	releaseGuardVersion      = "9.1.0"
	releaseGuardP13Digest    = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	releaseGuardP14Digest    = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
)

func TestReleaseEvidenceLineageVerifierFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("release evidence verifier requires jq")
	}
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(
		root,
		"scripts",
		"release",
		"validate-evidence-lineage.sh",
	)
	validRun := map[string]any{
		"id":         101,
		"status":     "completed",
		"conclusion": "success",
		"head_sha":   releaseGuardCandidateSHA,
		"repository": map[string]any{
			"full_name": "m0n0x41d/haft",
		},
		"head_repository": map[string]any{
			"full_name": "m0n0x41d/haft",
		},
		"path":  ".github/workflows/p14-evidence.yml",
		"event": "workflow_dispatch",
	}
	validArtifacts := map[string]any{
		"total_count": 1,
		"artifacts": []any{map[string]any{
			"name":    "p14-final-evidence-" + releaseGuardCandidateSHA,
			"expired": false,
			"id":      501,
			"digest":  "sha256:" + strings.Repeat("4", 64),
		}},
	}
	runVerifier := func(t *testing.T, run map[string]any, artifacts map[string]any, workflow string) error {
		t.Helper()
		temporary := t.TempDir()
		runPath := filepath.Join(temporary, "run.json")
		artifactsPath := filepath.Join(temporary, "artifacts.json")
		writeReleaseGuardJSON(t, runPath, run)
		writeReleaseGuardJSON(t, artifactsPath, artifacts)
		command := exec.Command(
			"bash",
			script,
			runPath,
			artifactsPath,
			"101",
			releaseGuardCandidateSHA,
			workflow,
			"workflow_dispatch",
			"p14-final-evidence-"+releaseGuardCandidateSHA,
			"m0n0x41d/haft",
		)
		return command.Run()
	}
	if err := runVerifier(t, validRun, validArtifacts, ".github/workflows/p14-evidence.yml"); err != nil {
		t.Fatalf("positive release lineage rejected: %v", err)
	}

	tests := map[string]func(map[string]any, map[string]any) string{
		"missing": func(_ map[string]any, artifacts map[string]any) string {
			artifacts["total_count"] = 0
			artifacts["artifacts"] = []any{}
			return ".github/workflows/p14-evidence.yml"
		},
		"expired": func(_ map[string]any, artifacts map[string]any) string {
			artifacts["artifacts"].([]any)[0].(map[string]any)["expired"] = true
			return ".github/workflows/p14-evidence.yml"
		},
		"candidate_mismatch": func(run map[string]any, _ map[string]any) string {
			run["head_sha"] = strings.Repeat("9", 40)
			return ".github/workflows/p14-evidence.yml"
		},
		"duplicate": func(_ map[string]any, artifacts map[string]any) string {
			first := artifacts["artifacts"].([]any)[0].(map[string]any)
			second := cloneReleaseGuardMap(t, first)
			second["id"] = 502
			artifacts["artifacts"] = append(artifacts["artifacts"].([]any), second)
			artifacts["total_count"] = 2
			return ".github/workflows/p14-evidence.yml"
		},
		"truncated_duplicate_page": func(_ map[string]any, artifacts map[string]any) string {
			artifacts["total_count"] = 2
			return ".github/workflows/p14-evidence.yml"
		},
		"non_passed": func(run map[string]any, _ map[string]any) string {
			run["conclusion"] = "failure"
			return ".github/workflows/p14-evidence.yml"
		},
		"wildcard_workflow": func(_ map[string]any, _ map[string]any) string {
			return "*"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			run := cloneReleaseGuardMap(t, validRun)
			artifacts := cloneReleaseGuardMap(t, validArtifacts)
			workflow := mutate(run, artifacts)
			if err := runVerifier(t, run, artifacts, workflow); err == nil {
				t.Fatal("release lineage verifier accepted invalid evidence")
			}
		})
	}
}

func TestReleaseEvidenceManifestVerifierFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("release evidence verifier requires jq")
	}
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(
		root,
		"scripts",
		"release",
		"validate-release-evidence-manifest.sh",
	)
	valid := releaseGuardManifest()
	runVerifier := func(t *testing.T, manifest map[string]any, version, p13, p14 string) error {
		t.Helper()
		path := filepath.Join(t.TempDir(), "release-evidence.json")
		writeReleaseGuardJSON(t, path, manifest)
		return exec.Command(
			"bash",
			script,
			path,
			releaseGuardCandidateSHA,
			version,
			p13,
			p14,
		).Run()
	}
	if err := runVerifier(
		t,
		valid,
		releaseGuardVersion,
		releaseGuardP13Digest,
		releaseGuardP14Digest,
	); err != nil {
		t.Fatalf("positive release manifest rejected: %v", err)
	}

	tests := map[string]func(map[string]any) (string, string, string){
		"missing": func(manifest map[string]any) (string, string, string) {
			delete(manifest, "p14")
			return releaseGuardVersion, releaseGuardP13Digest, releaseGuardP14Digest
		},
		"non_passed": func(manifest map[string]any) (string, string, string) {
			manifest["status"] = "failed"
			return releaseGuardVersion, releaseGuardP13Digest, releaseGuardP14Digest
		},
		"candidate_mismatch": func(manifest map[string]any) (string, string, string) {
			manifest["candidate_sha"] = strings.Repeat("9", 40)
			return releaseGuardVersion, releaseGuardP13Digest, releaseGuardP14Digest
		},
		"version_mismatch": func(_ map[string]any) (string, string, string) {
			return "9.2.0", releaseGuardP13Digest, releaseGuardP14Digest
		},
		"tag_mismatch": func(manifest map[string]any) (string, string, string) {
			manifest["tag"] = "v9.2.0"
			return releaseGuardVersion, releaseGuardP13Digest, releaseGuardP14Digest
		},
		"digest_mismatch": func(_ map[string]any) (string, string, string) {
			return releaseGuardVersion, "sha256:" + strings.Repeat("8", 64), releaseGuardP14Digest
		},
		"expired": func(manifest map[string]any) (string, string, string) {
			manifest["p14"].(map[string]any)["valid_until"] = "2000-01-01T00:00:00Z"
			return releaseGuardVersion, releaseGuardP13Digest, releaseGuardP14Digest
		},
		"archive_set_mismatch": func(manifest map[string]any) (string, string, string) {
			archives := manifest["p14"].(map[string]any)["release_archives"].([]any)
			archives[0].(map[string]any)["name"] = "other.tar.gz"
			return releaseGuardVersion, releaseGuardP13Digest, releaseGuardP14Digest
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			manifest := cloneReleaseGuardMap(t, valid)
			version, p13, p14 := mutate(manifest)
			if err := runVerifier(t, manifest, version, p13, p14); err == nil {
				t.Fatal("release manifest verifier accepted invalid evidence")
			}
		})
	}
}

func TestReleaseWorkflowStaticallyPinsEvidenceAndPreservesPreparationDefault(
	t *testing.T,
) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(raw)
	runnerRaw, err := os.ReadFile(filepath.Join(
		root,
		"scripts",
		"release",
		"run-evidence-verifier-test.sh",
	))
	if err != nil {
		t.Fatal(err)
	}
	guardedRelease := workflow + "\n" + string(runnerRaw)
	required := []string{
		"default: '9.1.0'",
		".github/workflows/p13-basis.yml",
		".github/workflows/p14-evidence.yml",
		"validate-evidence-lineage.sh",
		"validate-release-evidence-manifest.sh",
		"run-evidence-verifier-test.sh",
		"TestP13VerifyFreezeInputCandidate",
		"TestP13ManifestStructureAndAnchors",
		"TestP13VerifyAcceptanceEvidenceFresh",
		"TestP14VerifyReleaseEvidenceBundle",
		"validation_run_attempt",
		"validation_artifact_id",
		"validation_artifact_digest",
		"artifact-ids: ${{ steps.lineage.outputs.bundle_artifact_id }}",
		".total_count == 1",
		"gh api --method GET -f name=\"$artifact\"",
		"gh api --method GET -f name=\"$artifact_name\"",
		"--method GET",
		".run_attempt == $attempt",
		"qualified_executable_digest",
		"release_archives",
		"valid_until",
		"needs: [guard, evidence]",
		"version: $version",
		"tag: (\"v\" + $version)",
		"git cat-file -e \"${CANDIDATE_SHA}:${producer}\"",
	}
	for _, fragment := range required {
		if !strings.Contains(guardedRelease, fragment) {
			t.Fatalf("release workflow omits fail-closed fragment %q", fragment)
		}
	}
	for _, forbidden := range []string{
		"overwrite: true",
		"artifacts?per_page=100",
		"fetch_lineage p13-basis \"$P13_BASIS_RUN_ID\" *",
		"fetch_lineage p14 \"$P14_RUN_ID\" *",
		"BuildDate=$(date",
		"go build -trimpath",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow contains open selector %q", forbidden)
		}
	}

	publishMarker := "      - name: Publish the verified sealed bundle"
	publishIndex := strings.LastIndex(workflow, publishMarker)
	if publishIndex < 0 {
		t.Fatal("release workflow omits the final publish step")
	}
	publishStep := workflow[publishIndex:]
	publishChecks := []string{
		"git fetch origin main --tags",
		"refs/tags/$TAG^{commit}",
		"validate-candidate.sh \"$version\" \"$TAG_SHA\" \"$main_sha\"",
		"validate-release-evidence-manifest.sh",
		"release-evidence.json \"$TAG_SHA\" \"$version\"",
		"gh release create \"$TAG\"",
	}
	previous := -1
	for _, fragment := range publishChecks {
		index := strings.Index(publishStep, fragment)
		if index < 0 {
			t.Fatalf("final publish step omits time-of-use check %q", fragment)
		}
		if index <= previous {
			t.Fatalf("final publish step orders %q after the release effect", fragment)
		}
		previous = index
	}
}

func TestReleaseP14VerifierRunnerRequiresExactRunAndPassEvents(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("release evidence verifier requires jq")
	}
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	temporary := t.TempDir()
	fakeGo := filepath.Join(temporary, "go")
	fake := `#!/usr/bin/env bash
set -euo pipefail
if [[ " $* " == *" -list "* ]]; then
  if [ "${FAKE_GO_MODE:-pass}" != "absent" ]; then
    echo TestP14VerifyReleaseEvidenceBundle
  fi
  echo 'ok example.invalid/p14'
  exit 0
fi
case "${FAKE_GO_MODE:-pass}" in
  pass)
    printf '%s\n' '{"Action":"run","Test":"TestP14VerifyReleaseEvidenceBundle"}' '{"Action":"pass","Test":"TestP14VerifyReleaseEvidenceBundle"}'
    ;;
  skip)
    printf '%s\n' '{"Action":"run","Test":"TestP14VerifyReleaseEvidenceBundle"}' '{"Action":"skip","Test":"TestP14VerifyReleaseEvidenceBundle"}' '{"Action":"pass","Test":"TestP14VerifyReleaseEvidenceBundle"}'
    ;;
  duplicate)
    printf '%s\n' '{"Action":"run","Test":"TestP14VerifyReleaseEvidenceBundle"}' '{"Action":"run","Test":"TestP14VerifyReleaseEvidenceBundle"}' '{"Action":"pass","Test":"TestP14VerifyReleaseEvidenceBundle"}'
    ;;
  fail)
    printf '%s\n' '{"Action":"run","Test":"TestP14VerifyReleaseEvidenceBundle"}' '{"Action":"fail","Test":"TestP14VerifyReleaseEvidenceBundle"}'
    exit 1
    ;;
esac
`
	if err := os.WriteFile(fakeGo, []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(
		root,
		"scripts",
		"release",
		"run-evidence-verifier-test.sh",
	)
	run := func(mode string) error {
		command := exec.Command(
			"bash",
			runner,
			"./internal/p14acceptance",
			"TestP14VerifyReleaseEvidenceBundle",
		)
		command.Env = append(
			os.Environ(),
			"PATH="+temporary+string(os.PathListSeparator)+os.Getenv("PATH"),
			"FAKE_GO_MODE="+mode,
		)
		return command.Run()
	}
	if err := run("pass"); err != nil {
		t.Fatalf("P14 verifier runner rejected exact run/pass events: %v", err)
	}
	for _, mode := range []string{"absent", "skip", "duplicate", "fail"} {
		t.Run(mode, func(t *testing.T) {
			if err := run(mode); err == nil {
				t.Fatal("P14 verifier runner accepted incomplete test evidence")
			}
		})
	}
}

func releaseGuardManifest() map[string]any {
	return map[string]any{
		"schema":        "haft.release.validation-evidence/v1",
		"status":        "passed",
		"candidate_sha": releaseGuardCandidateSHA,
		"version":       releaseGuardVersion,
		"tag":           "v" + releaseGuardVersion,
		"p13": map[string]any{
			"run_id":                "101",
			"artifact_name":         "p13-acceptance-" + releaseGuardCandidateSHA,
			"artifact_id":           401,
			"artifact_digest":       "sha256:" + strings.Repeat("4", 64),
			"basis_run_id":          "102",
			"basis_artifact_name":   "p13-frozen-basis",
			"basis_artifact_id":     402,
			"basis_artifact_digest": "sha256:" + strings.Repeat("5", 64),
			"basis_workflow":        ".github/workflows/p13-basis.yml",
			"carrier_path":          ".context/p13/p13-acceptance.json",
			"carrier_digest":        releaseGuardP13Digest,
		},
		"p14": map[string]any{
			"producer_workflow":           ".github/workflows/p14-evidence.yml",
			"run_id":                      "103",
			"artifact_name":               "p14-final-evidence-" + releaseGuardCandidateSHA,
			"artifact_id":                 403,
			"artifact_digest":             "sha256:" + strings.Repeat("6", 64),
			"bundle_digest":               "sha256:" + strings.Repeat("7", 64),
			"prepared_carrier_path":       ".context/p14/prepared.json",
			"prepared_carrier_digest":     "sha256:" + strings.Repeat("8", 64),
			"final_carrier_path":          ".context/p14/final.json",
			"final_carrier_digest":        releaseGuardP14Digest,
			"valid_until":                 "2099-01-01T00:00:00Z",
			"qualified_executable_digest": "sha256:" + strings.Repeat("9", 64),
			"release_archives": []any{
				map[string]any{"name": "haft-linux-amd64.tar.gz", "digest": "sha256:" + strings.Repeat("a", 64)},
				map[string]any{"name": "haft-linux-arm64.tar.gz", "digest": "sha256:" + strings.Repeat("b", 64)},
				map[string]any{"name": "haft-darwin-arm64.tar.gz", "digest": "sha256:" + strings.Repeat("c", 64)},
			},
		},
	}
}

func writeReleaseGuardJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func cloneReleaseGuardMap(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	clone := map[string]any{}
	if err := decoder.Decode(&clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
