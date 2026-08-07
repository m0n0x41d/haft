package p13acceptance

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	hg0EvidenceDirectory = ".context/v9-remaining-evidence/r2-graph-baseline"
	hg0CorpusManifest    = "corpus-manifest.json"
	hg0ChecksumManifest  = "MANIFEST.sha256"
)

type hg8CorpusManifest struct {
	Schema      string                `json:"schema"`
	Status      string                `json:"status"`
	BaselineRef string                `json:"baseline_ref"`
	LabelPolicy hg8CorpusLabelPolicy  `json:"label_policy"`
	Concerns    []hg8CorpusConcern    `json:"concerns"`
	Adversarial hg8AdversarialCorpora `json:"adversarial_corpora"`
}

type hg8CorpusLabelPolicy struct {
	GraphEpoch        int64             `json:"graph_epoch"`
	RankingThresholds map[string]string `json:"ranking_thresholds"`
}

type hg8CorpusConcern struct {
	ID                    string   `json:"id"`
	Class                 string   `json:"class"`
	AcceptableSymbolIDs   []string `json:"acceptable_symbol_ids"`
	ExpectedReasoningRefs []string `json:"expected_reasoning_refs"`
}

type hg8AdversarialCorpora struct {
	SourceAdmission  []hg8AdversarialCase `json:"source_admission"`
	Traversal        []hg8AdversarialCase `json:"traversal"`
	EpochPublication []hg8AdversarialCase `json:"epoch_publication"`
}

type hg8AdversarialCase struct {
	ID       string `json:"id"`
	Fixture  string `json:"fixture"`
	Expected string `json:"expected"`
}

type hg8AnchorRequirement struct {
	GateID string
	Anchor testAnchor
}

func TestHGAdversarialAcceptance(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	// Носители HG0 живут под .context, который не отслеживается git, поэтому в
	// свежем чекауте их нет. Пропускаем по образцу
	// internal/recall/liveeval_test.go: отсутствие носителя — не регрессия.
	if _, statErr := os.Stat(
		filepath.Join(root, hg0EvidenceDirectory, hg0ChecksumManifest),
	); os.IsNotExist(statErr) {
		t.Skipf("HG0 evidence carriers not found under %s — skipping", hg0EvidenceDirectory)
	}
	if err := validateHG0CarrierDigests(root); err != nil {
		t.Fatal(err)
	}
	corpus, err := readHG8CorpusManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateHG8Corpus(corpus); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateHG8AnchorClosure(root, manifest); err != nil {
		t.Fatal(err)
	}
}

func validateHG0CarrierDigests(root string) error {
	directory := filepath.Join(root, hg0EvidenceDirectory)
	manifestPath := filepath.Join(directory, hg0ChecksumManifest)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	seen := make(map[string]struct{})
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return fmt.Errorf("HG0 checksum line is malformed: %q", scanner.Text())
		}
		name := filepath.Clean(fields[1])
		if name != filepath.Base(name) {
			return fmt.Errorf("HG0 checksum path is not local: %q", fields[1])
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("HG0 checksum path is duplicated: %q", name)
		}
		seen[name] = struct{}{}
		carrier, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		digest := sha256.Sum256(carrier)
		observed := hex.EncodeToString(digest[:])
		if observed != fields[0] {
			return fmt.Errorf(
				"HG0 carrier %q digest = %s, want %s",
				name,
				observed,
				fields[0],
			)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(seen) != 10 {
		return fmt.Errorf("HG0 checksum carrier count = %d, want 10", len(seen))
	}
	return nil
}

func readHG8CorpusManifest(root string) (hg8CorpusManifest, error) {
	path := filepath.Join(root, hg0EvidenceDirectory, hg0CorpusManifest)
	content, err := os.ReadFile(path)
	if err != nil {
		return hg8CorpusManifest{}, err
	}
	var corpus hg8CorpusManifest
	if err := json.Unmarshal(content, &corpus); err != nil {
		return hg8CorpusManifest{}, err
	}
	return corpus, nil
}

func validateHG8Corpus(corpus hg8CorpusManifest) error {
	if corpus.Schema != "haft.v9.hg0-corpus-manifest/v1" ||
		corpus.Status != "frozen_before_hg1" ||
		corpus.BaselineRef != hg0EvidenceDirectory+"/baseline.json" ||
		corpus.LabelPolicy.GraphEpoch != 1 {
		return fmt.Errorf("HG8 corpus does not name the exact frozen HG0 basis")
	}
	expectedThresholds := map[string]string{
		"code_native":       "acceptable stable symbol in top 5 for 6/6 and top 3 for at least 5/6",
		"reasoning_to_code": "acceptable stable symbol in top 5 for 3/3 with every expected exact reasoning ref",
		"ambiguous_negative": "0/3 silent single-symbol selections; exact " +
			"candidate_set or seed_not_found",
	}
	if !equalStringMap(corpus.LabelPolicy.RankingThresholds, expectedThresholds) {
		return fmt.Errorf("HG8 corpus ranking thresholds differ from HG0")
	}
	if err := validateHG8Concerns(corpus.Concerns); err != nil {
		return err
	}
	return validateHG8AdversarialCases(corpus.Adversarial)
}

func validateHG8Concerns(concerns []hg8CorpusConcern) error {
	expectedCounts := map[string]int{
		"code_native":        6,
		"reasoning_to_code":  3,
		"ambiguous_negative": 3,
	}
	observedCounts := make(map[string]int, len(expectedCounts))
	seen := make(map[string]struct{}, len(concerns))
	for _, concern := range concerns {
		if _, duplicate := seen[concern.ID]; duplicate {
			return fmt.Errorf("HG8 concern %q is duplicated", concern.ID)
		}
		seen[concern.ID] = struct{}{}
		if _, supported := expectedCounts[concern.Class]; !supported {
			return fmt.Errorf("HG8 concern %q has unknown class %q", concern.ID, concern.Class)
		}
		observedCounts[concern.Class]++
		if err := validateHG8ConcernEvidence(concern); err != nil {
			return err
		}
	}
	if !equalIntMap(observedCounts, expectedCounts) {
		return fmt.Errorf("HG8 concern class counts = %v, want %v", observedCounts, expectedCounts)
	}
	return nil
}

func validateHG8ConcernEvidence(concern hg8CorpusConcern) error {
	for _, symbolID := range concern.AcceptableSymbolIDs {
		if !strings.HasPrefix(symbolID, "sym:v2:") {
			return fmt.Errorf("HG8 concern %q has unstable symbol ID %q", concern.ID, symbolID)
		}
	}
	if concern.Class == "code_native" && len(concern.AcceptableSymbolIDs) == 0 {
		return fmt.Errorf("HG8 code-native concern %q has no acceptable symbol", concern.ID)
	}
	if concern.Class != "reasoning_to_code" {
		return nil
	}
	if len(concern.AcceptableSymbolIDs) == 0 ||
		len(concern.ExpectedReasoningRefs) == 0 {
		return fmt.Errorf("HG8 reasoning concern %q has no exact bridge", concern.ID)
	}
	for _, ref := range concern.ExpectedReasoningRefs {
		if !strings.HasPrefix(ref, "dec-") {
			return fmt.Errorf("HG8 reasoning concern %q has invalid ref %q", concern.ID, ref)
		}
	}
	return nil
}

func validateHG8AdversarialCases(corpora hg8AdversarialCorpora) error {
	groups := []struct {
		name  string
		cases []hg8AdversarialCase
		count int
	}{
		{name: "source_admission", cases: corpora.SourceAdmission, count: 8},
		{name: "traversal", cases: corpora.Traversal, count: 10},
		{name: "epoch_publication", cases: corpora.EpochPublication, count: 4},
	}
	seen := make(map[string]struct{})
	for _, group := range groups {
		if len(group.cases) != group.count {
			return fmt.Errorf(
				"HG8 %s case count = %d, want %d",
				group.name,
				len(group.cases),
				group.count,
			)
		}
		for _, adversarialCase := range group.cases {
			if adversarialCase.ID == "" || adversarialCase.Expected == "" {
				return fmt.Errorf("HG8 %s has an incomplete case", group.name)
			}
			key := group.name + "/" + adversarialCase.ID
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("HG8 adversarial case %q is duplicated", key)
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func validateHG8AnchorClosure(
	root string,
	manifest acceptanceManifest,
) error {
	gateAnchors := make(map[string][]string, len(manifest.Gates))
	for _, gate := range manifest.Gates {
		gateAnchors[gate.ID] = anchorKeys(gate.Anchors)
	}
	for _, requirement := range hg8RequiredAnchors() {
		key := anchorKey(requirement.Anchor)
		anchors, found := gateAnchors[requirement.GateID]
		if !found {
			return fmt.Errorf("HG8 requires missing gate %q", requirement.GateID)
		}
		if !slices.Contains(anchors, key) {
			return fmt.Errorf(
				"HG8 anchor %s is absent from gate %q",
				key,
				requirement.GateID,
			)
		}
		exists, err := testAnchorExists(root, requirement.Anchor)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("HG8 anchor %s does not exist", key)
		}
	}
	return nil
}

func hg8RequiredAnchors() []hg8AnchorRequirement {
	const (
		codebasePackage     = modulePath + "/internal/codebase"
		codeintelPackage    = modulePath + "/internal/codeintel"
		cliPackage          = modulePath + "/internal/cli"
		initplanningPackage = modulePath + "/internal/initplanning"
		p13Package          = modulePath + "/internal/p13acceptance"
	)
	return []hg8AnchorRequirement{
		{GateID: "G7", Anchor: testAnchor{Package: codeintelPackage, Test: "TestHGTraversalOutcomeCorpus"}},
		{GateID: "G7", Anchor: testAnchor{Package: codeintelPackage, Test: "TestHGOutcomeSerializationDeterministic"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestSourceAdmissionPureCorpus"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestSourceAdmissionRootBudgetsAreTyped"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestRefreshIncrementalDoesNotFollowSymlinkCycle"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestRootAdmissionBudgetRetainsPriorEpoch"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestIndexPublicationFailureRollsBackCandidateAndBasis"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestIndexBasisSurvivesProcessReopen"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestConcurrentReaderObservesOnlyWholeEpochBasis"}},
		{GateID: "G7", Anchor: testAnchor{Package: codebasePackage, Test: "TestCodeGraphTypeScriptParityQualificationCarrierIsCurrent"}},
		{GateID: "G7R", Anchor: testAnchor{Package: codebasePackage, Test: "TestConcernDiscoveryMeetsFrozenCodeNativeCorpus"}},
		{GateID: "G7R", Anchor: testAnchor{Package: codebasePackage, Test: "TestConcernDiscoveryNeverSelectsFrozenNegativeCorpus"}},
		{GateID: "G7R", Anchor: testAnchor{Package: codeintelPackage, Test: "TestConcernFusionMeetsFrozenReasoningToCodeCorpus"}},
		{GateID: "G7", Anchor: testAnchor{Package: codeintelPackage, Test: "TestPublishedExploreWorkingHostileConcernIsEscapedDeterministicAndBounded"}},
		{GateID: "G7", Anchor: testAnchor{Package: codeintelPackage, Test: "TestPublishedExploreTraceIsDeterministicAndDiagnosticIsExplicit"}},
		{GateID: "G7", Anchor: testAnchor{Package: cliPackage, Test: "TestHandleQuintQueryExploreConcernReturnsEvidenceBearingCandidates"}},
		{GateID: "G7", Anchor: testAnchor{Package: initplanningPackage, Test: "TestSkillComponentRendererDerivesHostCarriersFromOneBundle"}},
		{GateID: "G8", Anchor: testAnchor{Package: p13Package, Test: "TestHGAdversarialAcceptance"}},
	}
}

func equalStringMap(observed map[string]string, expected map[string]string) bool {
	if len(observed) != len(expected) {
		return false
	}
	for key, value := range expected {
		if observed[key] != value {
			return false
		}
	}
	return true
}

func equalIntMap(observed map[string]int, expected map[string]int) bool {
	if len(observed) != len(expected) {
		return false
	}
	for key, value := range expected {
		if observed[key] != value {
			return false
		}
	}
	return true
}
