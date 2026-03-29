package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// File picker: @ mention completion for attaching files to messages.
// Same pattern as CommandPalette — pure data, Model routes keys.
// ---------------------------------------------------------------------------

const filePickerMaxItems = 15

// FilePicker provides file completion when user types @.
type FilePicker struct {
	items       []fileItem
	selected    int
	filter      string // current filter text (after @)
	projectRoot string
	allFiles    []string // cached file list
}

type fileItem struct {
	Path  string // relative path
	Name  string // basename
	Size  int64
	IsDir bool
}

// NewFilePicker creates a file picker scoped to project root.
func NewFilePicker(projectRoot string) *FilePicker {
	return &FilePicker{projectRoot: projectRoot}
}

// Update recomputes filtered list from input text.
// Triggers on @ prefix: "@src/m" filters to files matching "src/m".
func (p *FilePicker) Update(inputValue string) {
	// Find last @ in input
	atIdx := strings.LastIndex(inputValue, "@")
	if atIdx < 0 {
		p.items = nil
		p.selected = 0
		p.filter = ""
		return
	}

	// Only trigger if @ is at start or after whitespace
	if atIdx > 0 && inputValue[atIdx-1] != ' ' && inputValue[atIdx-1] != '\n' {
		p.items = nil
		return
	}

	p.filter = inputValue[atIdx+1:]

	// Lazy load file list
	if len(p.allFiles) == 0 {
		p.allFiles = p.scanFiles()
	}

	// Filter and rank
	p.items = p.filterFiles(p.filter)

	if p.selected >= len(p.items) {
		p.selected = max(0, len(p.items)-1)
	}
}

// Visible reports whether picker should be shown.
func (p *FilePicker) Visible() bool {
	return len(p.items) > 0
}

// MoveUp moves selection up.
func (p *FilePicker) MoveUp() {
	if len(p.items) == 0 {
		return
	}
	p.selected--
	if p.selected < 0 {
		p.selected = len(p.items) - 1
	}
}

// MoveDown moves selection down.
func (p *FilePicker) MoveDown() {
	if len(p.items) == 0 {
		return
	}
	p.selected++
	if p.selected >= len(p.items) {
		p.selected = 0
	}
}

// Selected returns the selected file path, or "".
func (p *FilePicker) Selected() string {
	if p.selected >= 0 && p.selected < len(p.items) {
		return p.items[p.selected].Path
	}
	return ""
}

// InvalidateCache clears cached file list (call after edit/write).
func (p *FilePicker) InvalidateCache() {
	p.allFiles = nil
}

// Render draws the picker as a styled box (similar to command palette).
func (p *FilePicker) Render(width int, styles Styles) string {
	if len(p.items) == 0 {
		return ""
	}

	nameWidth := 30
	pathWidth := width - nameWidth - 8

	var lines []string
	for i, item := range p.items {
		name := item.Name
		path := item.Path
		if len([]rune(path)) > pathWidth {
			path = string([]rune(path)[:pathWidth-1]) + "…"
		}

		line := padRight(name, nameWidth+1) + styles.Dim.Render(path)

		if i == p.selected {
			line = styles.StatusAccent.Render("▸ ") + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	title := styles.PermTitle.Render("📎 Files") + styles.Dim.Render(" ("+p.filter+")")
	content := title + "\n" + strings.Join(lines, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(min(width-4, 70)).
		Render(content)

	return box
}

func (p *FilePicker) scanFiles() []string {
	if p.projectRoot == "" {
		return nil
	}

	var files []string
	_ = filepath.WalkDir(p.projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".quint" || name == "__pycache__" || name == ".cache" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(p.projectRoot, path)
		files = append(files, rel)
		if len(files) > 2000 {
			return filepath.SkipAll
		}
		return nil
	})

	// Sort by mtime (recent first)
	type ft struct {
		path string
		mod  int64
	}
	fts := make([]ft, len(files))
	for i, f := range files {
		abs := filepath.Join(p.projectRoot, f)
		if info, err := os.Stat(abs); err == nil {
			fts[i] = ft{path: f, mod: info.ModTime().UnixNano()}
		} else {
			fts[i] = ft{path: f}
		}
	}
	sort.Slice(fts, func(i, j int) bool { return fts[i].mod > fts[j].mod })
	result := make([]string, len(fts))
	for i, f := range fts {
		result[i] = f.path
	}
	return result
}

func (p *FilePicker) filterFiles(query string) []fileItem {
	query = strings.ToLower(query)
	var items []fileItem

	for _, path := range p.allFiles {
		if len(items) >= filePickerMaxItems {
			break
		}
		lower := strings.ToLower(path)
		if query == "" || strings.Contains(lower, query) {
			info, _ := os.Stat(filepath.Join(p.projectRoot, path))
			var size int64
			if info != nil {
				size = info.Size()
			}
			items = append(items, fileItem{
				Path: path,
				Name: filepath.Base(path),
				Size: size,
			})
		}
	}

	return items
}
