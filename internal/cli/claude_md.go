package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed claude_md_template.md
var embeddedClaudeMDTemplate string

const (
	haftSectionStart = "<!-- haft:start -->"
	haftSectionEnd   = "<!-- haft:end -->"
)

type claudeMDAction string

const (
	claudeMDCreated   claudeMDAction = "created"
	claudeMDUpdated   claudeMDAction = "updated"
	claudeMDUnchanged claudeMDAction = "unchanged"
	claudeMDAppended  claudeMDAction = "appended"
)

// installClaudeMD writes or updates the haft-managed section in
// projectRoot/CLAUDE.md. The section is delimited by HTML comment markers so
// any operator-authored content outside the markers is preserved across
// repeated runs of `haft init`.
//
// Behavior:
//   - File absent → create it with just the haft section.
//   - File present with markers → replace content between markers.
//   - File present without markers → append the haft section at the end.
//   - Section content identical to embedded template → "unchanged".
//
// To opt out, operators pass --no-file-instructions to `haft init`.
// Removing the markers from the file does NOT signal opt-out — the section
// will be re-appended on next init. Edits *between* the markers are
// overwritten.
func installClaudeMD(projectRoot string) (string, claudeMDAction, error) {
	path := filepath.Join(projectRoot, "CLAUDE.md")
	haftSection := wrapHaftSection(embeddedClaudeMDTemplate)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if writeErr := os.WriteFile(path, []byte(haftSection), 0o644); writeErr != nil {
			return path, "", writeErr
		}
		return path, claudeMDCreated, nil
	}
	if err != nil {
		return path, "", err
	}

	existing := string(data)
	startIdx := strings.Index(existing, haftSectionStart)
	// Search for the end marker AFTER the start marker — the body of the
	// haft section may legitimately mention these marker strings (e.g., in
	// explanatory text), so the first occurrence isn't reliable.
	endIdx := -1
	if startIdx >= 0 {
		bodyStart := startIdx + len(haftSectionStart)
		if rel := strings.Index(existing[bodyStart:], haftSectionEnd); rel >= 0 {
			endIdx = bodyStart + rel
		}
	}

	if startIdx >= 0 && endIdx > startIdx {
		before := existing[:startIdx]
		afterStart := endIdx + len(haftSectionEnd)
		after := ""
		if afterStart < len(existing) {
			after = existing[afterStart:]
		}
		// haftSection already has a trailing newline. Avoid double-newlines
		// where existing content already ended cleanly.
		merged := before + strings.TrimRight(haftSection, "\n") + after
		if merged == existing {
			return path, claudeMDUnchanged, nil
		}
		if err := os.WriteFile(path, []byte(merged), 0o644); err != nil {
			return path, "", err
		}
		return path, claudeMDUpdated, nil
	}

	// No markers found — append at end with a clean separator.
	sep := "\n\n"
	switch {
	case existing == "":
		sep = ""
	case strings.HasSuffix(existing, "\n\n"):
		sep = ""
	case strings.HasSuffix(existing, "\n"):
		sep = "\n"
	}
	merged := existing + sep + haftSection
	if err := os.WriteFile(path, []byte(merged), 0o644); err != nil {
		return path, "", err
	}
	return path, claudeMDAppended, nil
}

func wrapHaftSection(content string) string {
	body := strings.TrimSpace(content)
	return fmt.Sprintf("%s\n%s\n%s\n", haftSectionStart, body, haftSectionEnd)
}
