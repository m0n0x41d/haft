package fpf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
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

	return content, event, nil
}
