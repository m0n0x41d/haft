package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"

	"quint-mcp/db"
)

type TamperingEvent struct {
	FilePath     string
	ExpectedHash string
	ActualHash   string
	Regenerated  bool
}

func ComputeContentHash(body string) string {
	hash := sha256.Sum256([]byte(body))
	return hex.EncodeToString(hash[:16])
}

func parseFrontmatter(content string) (frontmatter string, body string, ok bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content, false
	}

	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return "", content, false
	}

	frontmatter = content[4 : 4+endIdx]
	body = content[4+endIdx+5:]
	return frontmatter, body, true
}

func extractHashFromFrontmatter(frontmatter string) string {
	re := regexp.MustCompile(`(?m)^content_hash:\s*([a-f0-9]+)\s*$`)
	matches := re.FindStringSubmatch(frontmatter)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func addHashToFrontmatter(frontmatter, hash string) string {
	re := regexp.MustCompile(`(?m)^content_hash:.*$`)
	if re.MatchString(frontmatter) {
		return re.ReplaceAllString(frontmatter, "content_hash: "+hash)
	}
	return frontmatter + "\ncontent_hash: " + hash
}

func WriteWithHash(path string, frontmatterFields map[string]string, body string) error {
	hash := ComputeContentHash(body)

	var fm strings.Builder
	fm.WriteString("---\n")
	for k, v := range frontmatterFields {
		fm.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	fm.WriteString(fmt.Sprintf("content_hash: %s\n", hash))
	fm.WriteString("---\n")

	content := fm.String() + body
	return os.WriteFile(path, []byte(content), 0644)
}

func ValidateFile(path string) (content string, tampered bool, expectedHash string, actualHash string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, "", "", err
	}

	content = string(data)
	frontmatter, body, hasFM := parseFrontmatter(content)
	if !hasFM {
		return content, false, "", "", nil
	}

	expectedHash = extractHashFromFrontmatter(frontmatter)
	if expectedHash == "" {
		return content, false, "", "", nil
	}

	actualHash = ComputeContentHash(body)
	if expectedHash != actualHash {
		return content, true, expectedHash, actualHash, nil
	}

	return content, false, expectedHash, actualHash, nil
}

func (t *Tools) ReadWithValidation(path string) (string, *TamperingEvent, error) {
	content, tampered, expectedHash, actualHash, err := ValidateFile(path)
	if err != nil {
		return "", nil, err
	}

	if !tampered {
		return content, nil, nil
	}

	event := &TamperingEvent{
		FilePath:     path,
		ExpectedHash: expectedHash,
		ActualHash:   actualHash,
		Regenerated:  false,
	}

	t.AuditLog("projection_validate", "tampering_detected", "system", path, "ALERT", map[string]string{
		"expected_hash": expectedHash,
		"actual_hash":   actualHash,
	}, "Content hash mismatch detected")

	if t.DB != nil {
		regenerated, regErr := t.regenerateFromDB(path)
		if regErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to regenerate %s from DB: %v\n", path, regErr)
		} else if regenerated {
			event.Regenerated = true
			t.AuditLog("projection_validate", "file_regenerated", "system", path, "SUCCESS", nil, "File regenerated from database")
			newContent, _, _, _, _ := ValidateFile(path)
			return newContent, event, nil
		}
	}

	return content, event, nil
}

func (t *Tools) regenerateFromDB(path string) (bool, error) {
	if t.DB == nil {
		return false, fmt.Errorf("DB not initialized")
	}

	holonID := extractHolonIDFromPath(path)
	if holonID == "" {
		return false, nil
	}

	ctx := context.Background()
	holon, err := t.DB.GetHolon(ctx, holonID)
	if err != nil {
		return false, err
	}

	layer := extractLayerFromPath(path)
	if layer == "" || layer != holon.Layer {
		return false, nil
	}

	body := fmt.Sprintf("\n# Hypothesis: %s\n\n%s", holon.Title, holon.Content)

	fields := map[string]string{
		"scope": holon.Scope.String,
		"kind":  holon.Kind.String,
	}

	if err := WriteWithHash(path, fields, body); err != nil {
		return false, err
	}

	return true, nil
}

func extractHolonIDFromPath(path string) string {
	re := regexp.MustCompile(`/knowledge/L[012]/([^/]+)\.md$`)
	matches := re.FindStringSubmatch(path)
	if len(matches) >= 2 {
		return matches[1]
	}

	re = regexp.MustCompile(`/knowledge/invalid/([^/]+)\.md$`)
	matches = re.FindStringSubmatch(path)
	if len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

func extractLayerFromPath(path string) string {
	re := regexp.MustCompile(`/knowledge/(L[012]|invalid)/`)
	matches := re.FindStringSubmatch(path)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func RegenerateHolonFile(store *db.Store, holonID, fpfDir string) error {
	if store == nil {
		return fmt.Errorf("DB not initialized")
	}

	ctx := context.Background()
	holon, err := store.GetHolon(ctx, holonID)
	if err != nil {
		return fmt.Errorf("holon not found: %w", err)
	}

	path := fmt.Sprintf("%s/knowledge/%s/%s.md", fpfDir, holon.Layer, holonID)

	body := fmt.Sprintf("\n# Hypothesis: %s\n\n%s", holon.Title, holon.Content)

	fields := map[string]string{
		"scope": holon.Scope.String,
		"kind":  holon.Kind.String,
	}

	return WriteWithHash(path, fields, body)
}
