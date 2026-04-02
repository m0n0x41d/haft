// Package skills provides a loadable capability system.
// Skills are markdown files with YAML frontmatter that define
// prompts, tool restrictions, and execution modes.
//
// Loaded from: .haft/skills/ (project) and ~/.haft/skills/ (global)
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillDef is a loadable capability.
type SkillDef struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Trigger     string   `yaml:"trigger,omitempty"`     // when to auto-invoke (regex on user input)
	Mode        string   `yaml:"mode,omitempty"`        // "inline" (inject prompt) or "agent" (fork subagent)
	Model       string   `yaml:"model,omitempty"`       // model override
	Tools       []string `yaml:"tools,omitempty"`       // tool allowlist (for agent mode)
	UserVisible bool     `yaml:"user_visible,omitempty"` // show in /help

	Prompt   string `yaml:"-"` // markdown body (after frontmatter)
	FilePath string `yaml:"-"` // source file
}

// Loader discovers and loads skills from disk.
type Loader struct {
	projectRoot string
	skills      map[string]*SkillDef
}

// NewLoader creates a skill loader and loads skills from standard locations.
func NewLoader(projectRoot string) *Loader {
	l := &Loader{
		projectRoot: projectRoot,
		skills:      make(map[string]*SkillDef),
	}
	l.load()
	return l
}

func (l *Loader) load() {
	// Project skills (higher priority)
	l.loadDir(filepath.Join(l.projectRoot, ".haft", "skills"))

	// Global skills
	home, err := os.UserHomeDir()
	if err == nil {
		l.loadDir(filepath.Join(home, ".haft", "skills"))
	}
}

func (l *Loader) loadDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".md" && ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		skill, err := parseSkillFile(path)
		if err != nil {
			continue
		}
		// Project skills override global ones with same name
		if _, exists := l.skills[skill.Name]; !exists {
			l.skills[skill.Name] = skill
		}
	}
}

// Get returns a skill by name.
func (l *Loader) Get(name string) (*SkillDef, bool) {
	s, ok := l.skills[name]
	return s, ok
}

// List returns all loaded skills.
func (l *Loader) List() []*SkillDef {
	result := make([]*SkillDef, 0, len(l.skills))
	for _, s := range l.skills {
		result = append(result, s)
	}
	return result
}

// Visible returns skills marked as user-visible (for /help).
func (l *Loader) Visible() []*SkillDef {
	var result []*SkillDef
	for _, s := range l.skills {
		if s.UserVisible {
			result = append(result, s)
		}
	}
	return result
}

// parseSkillFile reads a skill file with YAML frontmatter + markdown body.
// Format:
//
//	---
//	name: commit
//	description: Create a git commit
//	mode: inline
//	---
//	You are a commit message generator...
func parseSkillFile(path string) (*SkillDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	skill := &SkillDef{FilePath: path}

	// Split frontmatter and body
	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[4:], "\n---")
		if end >= 0 {
			frontmatter := content[4 : 4+end]
			skill.Prompt = strings.TrimSpace(content[4+end+4:])
			if err := yaml.Unmarshal([]byte(frontmatter), skill); err != nil {
				return nil, fmt.Errorf("parse frontmatter: %w", err)
			}
		}
	}

	if skill.Name == "" {
		// Derive name from filename
		skill.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if skill.Mode == "" {
		skill.Mode = "inline"
	}

	return skill, nil
}

// FormatSkillList returns a human-readable skill list.
func FormatSkillList(skills []*SkillDef) string {
	if len(skills) == 0 {
		return "No skills loaded."
	}
	var b strings.Builder
	for _, s := range skills {
		fmt.Fprintf(&b, "/%s — %s", s.Name, s.Description)
		if s.Mode == "agent" {
			b.WriteString(" [agent]")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
