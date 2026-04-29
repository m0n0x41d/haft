package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			RelativePath: filepath.Join("specs", "enabling-system.md"),
			Content:      enablingSystemSpecCarrierContent(),
		},
		{
			RelativePath: filepath.Join("specs", "term-map.md"),
			Content:      termMapSpecCarrierContent(),
		},
	}

	return append([]SpecCarrier(nil), carriers...)
}

func EnsureSpecCarriers(haftDir string) error {
	for _, carrier := range MinimumSpecCarriers() {
		path := filepath.Join(haftDir, carrier.RelativePath)
		info, err := os.Stat(path)
		switch {
		case err == nil && !info.IsDir():
			continue
		case err == nil && info.IsDir():
			return fmt.Errorf("spec carrier path is a directory: %s", path)
		case err != nil && !os.IsNotExist(err):
			return fmt.Errorf("inspect spec carrier %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create spec carrier directory %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(carrier.Content), 0o644); err != nil {
			return fmt.Errorf("write spec carrier %s: %w", path, err)
		}
	}

	return nil
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

func enablingSystemSpecCarrierContent() string {
	return strings.Join([]string{
		"# Enabling System Spec",
		"",
		"## ES.placeholder.001 Enabling system placeholder",
		"",
		"```yaml spec-section",
		"id: ES.placeholder.001",
		"kind: creator-role",
		"title: Enabling system placeholder",
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
		"This placeholder only reserves a parseable carrier for onboarding. It is not active enabling-system governance.",
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
