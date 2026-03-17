package artifact

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseFile parses a markdown file with YAML frontmatter into an Artifact.
func ParseFile(content string) (*Artifact, error) {
	meta, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	a := &Artifact{Body: body}
	if err := parseFrontmatter(meta, &a.Meta); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return a, nil
}

func splitFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimLeft(content, "\n\r ")
	if !strings.HasPrefix(content, "---") {
		return "", content, fmt.Errorf("no frontmatter found (missing opening ---)")
	}

	rest := content[3:]
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	// Find closing --- that is exactly "---" on its own line
	// (not a markdown horizontal rule like "---\nsome content")
	endIdx := -1
	lines := strings.SplitAfter(rest, "\n")
	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "---" {
			endIdx = offset
			break
		}
		offset += len(line)
	}

	if endIdx == -1 {
		return "", content, fmt.Errorf("no closing --- found for frontmatter")
	}

	frontmatter = rest[:endIdx]
	afterClose := rest[endIdx:]
	// Skip the "---" line itself
	if idx := strings.Index(afterClose, "\n"); idx >= 0 {
		body = strings.TrimLeft(afterClose[idx+1:], "\n\r")
	} else {
		body = ""
	}

	return frontmatter, body, nil
}

func parseFrontmatter(fm string, meta *Meta) error {
	meta.Version = 1
	meta.Status = StatusActive

	lines := strings.Split(fm, "\n")
	inLinks := false
	var currentLink *Link

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		if indent == 0 && trimmed != "links:" && !strings.HasPrefix(trimmed, "- ") {
			inLinks = false
		}

		if inLinks {
			if strings.HasPrefix(trimmed, "- ref:") {
				if currentLink != nil {
					meta.Links = append(meta.Links, *currentLink)
				}
				currentLink = &Link{Ref: strings.TrimSpace(strings.TrimPrefix(trimmed, "- ref:"))}
			} else if strings.HasPrefix(trimmed, "ref:") && indent > 2 {
				if currentLink == nil {
					currentLink = &Link{}
				}
				currentLink.Ref = strings.TrimSpace(strings.TrimPrefix(trimmed, "ref:"))
			} else if strings.HasPrefix(trimmed, "type:") {
				if currentLink != nil {
					currentLink.Type = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
				}
			}
			continue
		}

		if trimmed == "links:" {
			inLinks = true
			continue
		}

		key, value, ok := parseYAMLLine(trimmed)
		if !ok {
			continue
		}

		switch key {
		case "id":
			meta.ID = value
		case "kind":
			meta.Kind = Kind(value)
		case "version":
			if v, err := strconv.Atoi(value); err == nil {
				meta.Version = v
			}
		case "status":
			meta.Status = Status(value)
		case "context":
			meta.Context = value
		case "mode":
			meta.Mode = Mode(value)
		case "title":
			meta.Title = value
		case "valid_until":
			meta.ValidUntil = value
		case "created_at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				meta.CreatedAt = t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", value); err == nil {
				meta.CreatedAt = t
			}
		case "updated_at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				meta.UpdatedAt = t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", value); err == nil {
				meta.UpdatedAt = t
			}
		}
	}

	if inLinks && currentLink != nil {
		meta.Links = append(meta.Links, *currentLink)
	}

	if meta.ID == "" {
		return fmt.Errorf("missing required field: id")
	}
	if meta.Kind == "" {
		return fmt.Errorf("missing required field: kind")
	}

	return nil
}

func parseYAMLLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx == -1 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	// Remove surrounding quotes if present
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}
