package streamtruth

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var publicDecisionCarrierPaths = []string{
	".haft/decisions/dec-20260716-11f33e36.md",
	".haft/decisions/dec-20260716-318cdec5.md",
}

var publicExecutionCarrierPaths = []string{
	".context/haft-v9-deterministic-closeout.plan.md",
	".context/haft-v9-scope-freeze-inventory-20260728.md",
}

var publicReleaseCarrierPaths = append(
	append([]string(nil), publicDecisionCarrierPaths...),
	publicExecutionCarrierPaths...,
)

var markdownLinkPattern = regexp.MustCompile(`\[[^\]\n]+\]\(([^)\s]+)\)`)

type releaseCarrierFixture struct {
	name           string
	ignoredCarrier string
	exportIgnored  string
	leaveUntracked string
	wantFailure    string
}

func TestPublicContractDecisionLinksResolveToPublishedCarriers(t *testing.T) {
	for _, source := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		"spec/target-system/MODE_ONTOLOGY.md",
	} {
		actual := publicDecisionLinks(t, source)
		assertExactStringSet(t, source, actual, publicDecisionCarrierPaths)
	}

	for _, carrier := range publicDecisionCarrierPaths {
		assertActiveDecisionCarrier(t, carrier)
	}
	for _, carrier := range publicReleaseCarrierPaths {
		assertNotIgnored(t, carrier)
	}
}

func TestPublishedExecutionCarriersNameOneActiveCloseoutPlan(t *testing.T) {
	closeout := readTruthRepoFile(
		t,
		".context/haft-v9-deterministic-closeout.plan.md",
	)
	for _, required := range []string{
		"Status: active",
		"This file is the sole active WorkPlan-like execution carrier",
		"Publication authority: absent",
	} {
		if !strings.Contains(closeout, required) {
			t.Fatalf("active closeout carrier omits %q", required)
		}
	}

	inventory := readTruthRepoFile(
		t,
		".context/haft-v9-scope-freeze-inventory-20260728.md",
	)
	for _, required := range []string{
		"This is an inventory, not evidence",
		"COMMIT, push/integration, SpecSection lifecycle, host restart, tag, and",
	} {
		if !strings.Contains(inventory, required) {
			t.Fatalf("scope-freeze inventory omits %q", required)
		}
	}
}

func TestReleaseCandidateValidationGuardsTrackedArchiveCarriers(t *testing.T) {
	cases := []releaseCarrierFixture{
		{
			name: "tracked and archived",
		},
		{
			name:           "ignored decision even when force tracked",
			ignoredCarrier: publicDecisionCarrierPaths[0],
			wantFailure:    "public release carrier is still ignored",
		},
		{
			name:           "ignored execution plan even when force tracked",
			ignoredCarrier: publicExecutionCarrierPaths[0],
			wantFailure:    "public release carrier is still ignored",
		},
		{
			name:          "tracked execution plan but export ignored",
			exportIgnored: publicExecutionCarrierPaths[0],
			wantFailure:   "public release carrier is absent from git archive",
		},
		{
			name:           "present execution plan but untracked",
			leaveUntracked: publicExecutionCarrierPaths[0],
			wantFailure:    "public release carrier is not tracked",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := prepareReleaseCarrierRepository(t, testCase)
			candidateSHA := runGit(t, repository, "rev-parse", "HEAD")
			output, err := runCandidateValidation(t, repository, candidateSHA)
			if testCase.wantFailure == "" {
				if err != nil {
					t.Fatalf("validate release candidate: %v\n%s", err, output)
				}
				if !strings.Contains(
					output,
					"public release carriers validated",
				) {
					t.Fatalf("validation omitted public-carrier receipt:\n%s", output)
				}
				return
			}
			if err == nil {
				t.Fatalf("validation unexpectedly accepted fixture:\n%s", output)
			}
			if !strings.Contains(output, testCase.wantFailure) {
				t.Fatalf(
					"validation failure = %q, want substring %q",
					output,
					testCase.wantFailure,
				)
			}
		})
	}
}

func TestReleaseWorkflowUsesCandidateGuardForValidationAndPublication(t *testing.T) {
	workflow := readTruthRepoFile(t, ".github/workflows/release.yml")
	const invocation = `scripts/release/validate-candidate.sh`
	if count := strings.Count(workflow, invocation); count != 2 {
		t.Fatalf(
			"release workflow candidate guard invocation count = %d, want 2",
			count,
		)
	}

	candidateGuard := readTruthRepoFile(t, "scripts/release/validate-candidate.sh")
	if !strings.Contains(
		candidateGuard,
		`validate-public-release-carriers.sh`,
	) {
		t.Fatal("release candidate guard omits public release carrier validation")
	}
}

func publicDecisionLinks(t *testing.T, source string) []string {
	t.Helper()
	root := truthRepoRoot(t)
	content := readTruthRepoFile(t, source)
	matches := markdownLinkPattern.FindAllStringSubmatch(content, -1)
	links := make([]string, 0, len(publicDecisionCarrierPaths))
	for _, match := range matches {
		target := filepath.FromSlash(match[1])
		resolved := filepath.Join(root, filepath.Dir(source), target)
		relative, err := filepath.Rel(root, filepath.Clean(resolved))
		if err != nil {
			t.Fatalf("resolve link %q from %s: %v", match[1], source, err)
		}
		repositoryPath := filepath.ToSlash(relative)
		if !strings.HasPrefix(repositoryPath, ".haft/decisions/") {
			continue
		}
		links = append(links, repositoryPath)
	}
	sort.Strings(links)
	return links
}

func assertExactStringSet(
	t *testing.T,
	source string,
	actual []string,
	expected []string,
) {
	t.Helper()
	sortedExpected := append([]string(nil), expected...)
	sort.Strings(sortedExpected)
	if strings.Join(actual, "\x00") == strings.Join(sortedExpected, "\x00") {
		return
	}
	t.Fatalf(
		"%s decision links = %#v, want exact resolved set %#v",
		source,
		actual,
		sortedExpected,
	)
}

func assertActiveDecisionCarrier(t *testing.T, carrier string) {
	t.Helper()
	content := readTruthRepoFile(t, carrier)
	identifier := strings.TrimSuffix(filepath.Base(carrier), filepath.Ext(carrier))
	for _, required := range []string{
		"id: " + identifier,
		"kind: DecisionRecord",
		"status: active",
	} {
		if !frontmatterHasExactLine(content, required) {
			t.Fatalf("%s frontmatter omits %q", carrier, required)
		}
	}
	if !frontmatterHasPrefix(content, "title: ") {
		t.Fatalf("%s frontmatter omits readable title", carrier)
	}
}

func frontmatterHasExactLine(content string, required string) bool {
	for _, line := range frontmatterLines(content) {
		if line == required {
			return true
		}
	}
	return false
}

func frontmatterHasPrefix(content string, required string) bool {
	for _, line := range frontmatterLines(content) {
		if strings.HasPrefix(line, required) {
			return true
		}
	}
	return false
}

func frontmatterLines(content string) []string {
	normalized := normalizeTruthNewlines(content)
	const start = "---\n"
	if !strings.HasPrefix(normalized, start) {
		return nil
	}
	body := strings.TrimPrefix(normalized, start)
	end := strings.Index(body, "\n---\n")
	if end < 0 {
		return nil
	}
	return strings.Split(body[:end], "\n")
}

func assertNotIgnored(t *testing.T, carrier string) {
	t.Helper()
	command := exec.Command("git", "check-ignore", "--no-index", "-q", "--", carrier)
	command.Dir = truthRepoRoot(t)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("%s is excluded by Git ignore rules", carrier)
	}
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 1 {
		t.Fatalf("check Git ignore state for %s: %v\n%s", carrier, err, output)
	}
}

func prepareReleaseCarrierRepository(
	t *testing.T,
	testCase releaseCarrierFixture,
) string {
	t.Helper()
	repository := t.TempDir()
	runGit(t, repository, "init", "-q")
	runGit(t, repository, "config", "user.name", "Haft Test")
	runGit(t, repository, "config", "user.email", "haft-test@example.invalid")

	for _, carrier := range publicReleaseCarrierPaths {
		writeFixtureFile(t, repository, carrier, "published release carrier\n")
	}
	ignoreRules := strings.Join(
		[]string{
			"!.haft/decisions/",
			".haft/decisions/*",
			"!.context/",
			".context/**",
			"",
		},
		"\n",
	)
	for _, carrier := range publicReleaseCarrierPaths {
		if carrier != testCase.ignoredCarrier {
			ignoreRules += "!" + carrier + "\n"
		}
	}
	writeFixtureFile(t, repository, ".gitignore", ignoreRules)
	if testCase.exportIgnored != "" {
		attributes := testCase.exportIgnored + " export-ignore\n"
		writeFixtureFile(t, repository, ".gitattributes", attributes)
	}

	addArgs := []string{"add", "."}
	if testCase.ignoredCarrier != "" {
		addArgs = []string{"add", "-f", "."}
	}
	runGit(t, repository, addArgs...)
	if testCase.leaveUntracked != "" {
		runGit(t, repository, "rm", "--cached", "--", testCase.leaveUntracked)
	}
	runGit(t, repository, "commit", "-q", "-m", "fixture")
	return repository
}

func writeFixtureFile(t *testing.T, root string, path string, content string) {
	t.Helper()
	absolutePath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		t.Fatalf("create fixture directory for %s: %v", path, err)
	}
	if err := os.WriteFile(absolutePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func runCandidateValidation(
	t *testing.T,
	directory string,
	candidateSHA string,
) (string, error) {
	t.Helper()
	script := filepath.Join(
		truthRepoRoot(t),
		"scripts",
		"release",
		"validate-candidate.sh",
	)
	command := exec.Command("bash", script, "9.0.0", candidateSHA, candidateSHA)
	command.Dir = directory
	output, err := command.CombinedOutput()
	return string(output), err
}
