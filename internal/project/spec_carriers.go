package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type SpecCarrier struct {
	RelativePath string
	Content      string
}

func MinimumSpecCarriers() []SpecCarrier {
	carriers := []SpecCarrier{
		{
			RelativePath: filepath.Join("specs", "target-system.md"),
			Content:      targetSystemSpecCarrierContent(),
		},
		{
			RelativePath: filepath.Join("specs", "software-system.md"),
			Content:      softwareSystemSpecCarrierContent(),
		},
		{
			RelativePath: filepath.Join("specs", "term-map.md"),
			Content:      termMapSpecCarrierContent(),
		},
	}

	return append([]SpecCarrier(nil), carriers...)
}

func EnsureSpecCarriers(haftDir string) error {
	return ensureSpecCarrierSet(
		haftDir,
		MinimumSpecCarriers(),
		0,
	)
}

// EnsureRequiredSpecCarriers materializes only carriers whose exact
// scope-local central-matrix entry is Required. Existing files are preserved
// byte-for-byte and excluded or underdetermined files are never deleted.
func EnsureRequiredSpecCarriers(
	haftDir string,
	applicability ProjectSpecificationSetApplicability,
) error {
	carriers, err := RequiredSpecCarriers(applicability)
	if err != nil {
		return err
	}
	return ensureSpecCarrierSet(haftDir, carriers, 0)
}

// RequiredSpecCarriers is the pure carrier-install projection for one exact
// admitted ScopeID.
func RequiredSpecCarriers(
	applicability ProjectSpecificationSetApplicability,
) ([]SpecCarrier, error) {
	if !applicability.Valid() {
		return nil, fmt.Errorf("project specification applicability is invalid")
	}
	carriers := MinimumSpecCarriers()
	required := make([]SpecCarrier, 0, len(carriers))
	return appendRequiredSpecCarriers(
		carriers,
		applicability,
		required,
		0,
	)
}

func appendRequiredSpecCarriers(
	carriers []SpecCarrier,
	applicability ProjectSpecificationSetApplicability,
	required []SpecCarrier,
	index int,
) ([]SpecCarrier, error) {
	if index == len(carriers) {
		return required, nil
	}
	carrier := carriers[index]
	documentKind, err := specCarrierDocumentKind(carrier)
	if err != nil {
		return nil, err
	}
	member, found := applicability.Member(documentKind)
	if !found {
		return nil, fmt.Errorf(
			"project specification applicability omitted %q",
			documentKind,
		)
	}
	next := required
	if member.Kind() == projectprofile.CapabilityRequired {
		next = append(next, carrier)
	}
	return appendRequiredSpecCarriers(
		carriers,
		applicability,
		next,
		index+1,
	)
}

func ensureSpecCarrierSet(
	haftDir string,
	carriers []SpecCarrier,
	index int,
) error {
	if index == len(carriers) {
		return nil
	}
	if err := ensureSpecCarrier(haftDir, carriers[index]); err != nil {
		return err
	}
	return ensureSpecCarrierSet(haftDir, carriers, index+1)
}

func ensureSpecCarrier(haftDir string, carrier SpecCarrier) error {
	path := filepath.Join(haftDir, carrier.RelativePath)
	info, err := os.Stat(path)
	switch {
	case err == nil && !info.IsDir():
		return nil
	case err == nil && info.IsDir():
		return fmt.Errorf("spec carrier path is a directory: %s", path)
	case err != nil && !os.IsNotExist(err):
		return fmt.Errorf("inspect spec carrier %s: %w", path, err)
	}
	if isSoftwareSystemCarrier(carrier) &&
		legacyEnablingSystemCarrierExists(haftDir) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf(
			"create spec carrier directory %s: %w",
			filepath.Dir(path),
			err,
		)
	}
	if err := os.WriteFile(path, []byte(carrier.Content), 0o644); err != nil {
		return fmt.Errorf("write spec carrier %s: %w", path, err)
	}
	return nil
}

func specCarrierDocumentKind(carrier SpecCarrier) (SpecDocumentKind, error) {
	clean := filepath.ToSlash(filepath.Clean(carrier.RelativePath))
	switch clean {
	case "specs/target-system.md":
		return SpecDocumentKindTargetSystem, nil
	case "specs/software-system.md":
		return SpecDocumentKindSoftwareSystem, nil
	case "specs/term-map.md":
		return SpecDocumentKindTermMap, nil
	default:
		return "", fmt.Errorf(
			"spec carrier %q has no applicability policy",
			carrier.RelativePath,
		)
	}
}

func isSoftwareSystemCarrier(carrier SpecCarrier) bool {
	want := filepath.Clean(filepath.Join("specs", "software-system.md"))
	return filepath.Clean(carrier.RelativePath) == want
}

func legacyEnablingSystemCarrierExists(haftDir string) bool {
	path := filepath.Join(haftDir, "specs", "enabling-system.md")
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func targetSystemSpecCarrierContent() string {
	return strings.Join([]string{
		"# Target System Spec",
		"",
		"## TS.placeholder.001 Target system placeholder",
		"",
		"```yaml spec-section",
		"id: TS.placeholder.001",
		"kind: environment-change",
		"title: Target system placeholder",
		"statement_type: explanation",
		"claim_layer: carrier",
		"owner: human",
		"status: draft",
		"valid_until: null",
		"depends_on: []",
		"supersedes: []",
		"terms: []",
		"target_refs: []",
		"evidence_required: []",
		"```",
		"",
		"This placeholder only reserves a parseable carrier for onboarding. It is not an active target-system claim.",
		"",
	}, "\n")
}

func softwareSystemSpecCarrierContent() string {
	return strings.Join([]string{
		"# Software System Spec",
		"",
		"## SS.placeholder.001 Software system placeholder",
		"",
		"```yaml spec-section",
		"id: SS.placeholder.001",
		"kind: software.role",
		"title: Software system placeholder",
		"statement_type: explanation",
		"claim_layer: carrier",
		"owner: human",
		"status: draft",
		"valid_until: null",
		"depends_on: []",
		"supersedes: []",
		"terms: []",
		"target_refs: []",
		"evidence_required: []",
		"```",
		"",
		"This placeholder only reserves a parseable carrier for onboarding. It is not an active software-system claim.",
		"",
	}, "\n")
}

func termMapSpecCarrierContent() string {
	return strings.Join([]string{
		"# Term Map",
		"",
		"```yaml term-map",
		"entries: []",
		"status: draft",
		"```",
		"",
		"This placeholder has no term definitions. Add human-approved vocabulary during onboarding.",
		"",
	}, "\n")
}
