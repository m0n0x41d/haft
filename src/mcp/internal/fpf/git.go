package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/quint-code/assurance"
)

func (t *Tools) isGitRepo() bool {
	gitDir := filepath.Join(t.RootDir, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

func (t *Tools) getCurrentHead() (string, error) {
	if !t.isGitRepo() {
		return "", fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = t.RootDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (t *Tools) detectCodeChanges(ctx context.Context) (*CodeChangeDetectionResult, error) {
	return nil, nil
}

func (t *Tools) recalculateHolonR(ctx context.Context, holonID string) {
	calc := assurance.New(t.DB.GetRawDB())
	report, err := calc.CalculateReliability(ctx, holonID)
	if err == nil && report != nil {
		t.DB.UpdateHolonRScore(ctx, holonID, report.FinalScore)
	}
}

func (t *Tools) hashCarrierFiles(carrierRef string) string {
	if carrierRef == "" {
		return ""
	}

	files := strings.Split(carrierRef, ",")
	var hashes []string

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		fullPath := filepath.Join(t.RootDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			hashes = append(hashes, fmt.Sprintf("%s:_missing_", file))
			continue
		}

		hash := sha256.Sum256(content)
		hashes = append(hashes, fmt.Sprintf("%s:%s", file, hex.EncodeToString(hash[:8])))
	}

	sortedHashes := make([]string, len(hashes))
	copy(sortedHashes, hashes)
	for i := 0; i < len(sortedHashes)-1; i++ {
		for j := i + 1; j < len(sortedHashes); j++ {
			if sortedHashes[i] > sortedHashes[j] {
				sortedHashes[i], sortedHashes[j] = sortedHashes[j], sortedHashes[i]
			}
		}
	}

	return strings.Join(sortedHashes, ",")
}

func parseCarrierHashes(hashStr string) map[string]string {
	result := make(map[string]string)
	if hashStr == "" {
		return result
	}

	for _, pair := range strings.Split(hashStr, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func diffCarrierHashes(oldHash, newHash string) []string {
	oldMap := parseCarrierHashes(oldHash)
	newMap := parseCarrierHashes(newHash)

	var changed []string

	for file, oldH := range oldMap {
		newH, exists := newMap[file]
		if !exists || newH != oldH {
			changed = append(changed, file)
		}
	}

	for file := range newMap {
		if _, exists := oldMap[file]; !exists {
			changed = append(changed, file)
		}
	}

	return changed
}
