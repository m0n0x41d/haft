package code

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

type CaptureConfig struct {
	MaxFiles       int
	MaxFileBytes   int64
	MaxTotalBytes  int64
	IgnorePatterns []string
}
type Exclusion struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type CaptureResult struct {
	Config      CaptureConfig     `json:"capture_config"`
	Files       map[string][]byte `json:"files"`
	Complete    bool              `json:"complete"`
	Exclusions  []Exclusion       `json:"exclusions"`
	Diagnostics []Diagnostic      `json:"diagnostics,omitempty"`
}

// Index carries capture incompleteness into the pure index result. Callers must
// not publish a partial capture as a complete epoch merely because parsing passed.
func (capture CaptureResult) Index(config Config) (Index, error) {
	if strings.Join(capture.Config.IgnorePatterns, "\x00") != strings.Join(config.IgnorePatterns, "\x00") {
		return Index{}, fmt.Errorf("capture and index ignore policies differ")
	}
	index, err := IndexFromFiles(capture.Files, config)
	if err != nil {
		return index, err
	}
	if !capture.Complete {
		index.Complete = false
	}
	index.Diagnostics = append(index.Diagnostics, capture.Diagnostics...)
	for _, exclusion := range capture.Exclusions {
		if exclusion.Reason == "ignore_rule" && (isGoBuildInput(exclusion.Path) || path.Base(exclusion.Path) == "vendor") {
			index.Diagnostics = append(index.Diagnostics, diagnostic("excluded_source_closure_partial", exclusion.Path, "Ignored Go build input is outside the captured dependency closure"))
		}
	}
	return index, nil
}

type ignoreRule struct {
	base    string
	negate  bool
	matcher *gitignore.GitIgnore
}
type ignoreSet struct{ rules []ignoreRule }

func newIgnore(files map[string][]byte, extra []string) ignoreSet {
	rules := ignoreSet{}
	add := func(base string, lines []string) {
		for _, line := range lines {
			line = strings.TrimSuffix(line, "\r")
			negate := strings.HasPrefix(line, "!")
			if negate {
				line = strings.TrimPrefix(line, "!")
			}
			rules.rules = append(rules.rules, ignoreRule{base, negate, gitignore.CompileIgnoreLines(line)})
		}
	}
	for _, kind := range []string{".gitignore", ".haftignore"} {
		names := []string{}
		for name := range files {
			if path.Base(name) == kind {
				names = append(names, name)
			}
		}
		sort.Slice(names, func(a, b int) bool {
			x, y := names[a], names[b]
			if strings.Count(x, "/") != strings.Count(y, "/") {
				return strings.Count(x, "/") < strings.Count(y, "/")
			}
			return x < y
		})
		for _, name := range names {
			add(path.Dir(name), strings.Split(string(files[name]), "\n"))
		}
	}
	add(".", extra)
	return rules
}
func (set ignoreSet) match(p string, directory bool) bool {
	ignored := false
	for _, rule := range set.rules {
		relative := p
		if rule.base != "." {
			if !strings.HasPrefix(p, rule.base+"/") {
				continue
			}
			relative = strings.TrimPrefix(p, rule.base+"/")
		}
		if directory {
			relative += "/"
		}
		if rule.matcher.MatchesPath(relative) {
			ignored = !rule.negate
		}
	}
	return ignored
}
func (set ignoreSet) ignored(p string, directory bool) bool {
	parts := strings.Split(p, "/")
	for n := 1; n < len(parts); n++ {
		if set.match(strings.Join(parts[:n], "/"), true) {
			return true
		}
	}
	return set.match(p, directory)
}

// Capture reads a bounded root without following symlinks. A final byte readback
// detects changes during capture, including equal-size/equal-mtime edits. The
// caller publishes no complete epoch when Complete is false.
func Capture(root string, config CaptureConfig) (CaptureResult, error) {
	result := CaptureResult{Files: map[string][]byte{}, Complete: true, Exclusions: []Exclusion{}}
	abs, err := filepath.Abs(root)
	if err != nil {
		return result, err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return result, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("capture root must be a real directory")
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return result, err
	}
	// Canonicalize parent aliases such as macOS /var -> /private/var once.
	// The root itself was checked with Lstat and children are never followed.
	abs = real
	if config.MaxFiles <= 0 {
		config.MaxFiles = 10000
	}
	if config.MaxFileBytes <= 0 {
		config.MaxFileBytes = 8 << 20
	}
	if config.MaxTotalBytes <= 0 {
		config.MaxTotalBytes = 128 << 20
	}
	config.IgnorePatterns = append([]string{}, config.IgnorePatterns...)
	result.Config = config
	var total int64
	entries := map[string]fs.FileMode{}
	exclude := func(p, reason string, partial bool) {
		result.Exclusions = append(result.Exclusions, Exclusion{p, reason})
		if partial {
			result.Complete = false
			result.Diagnostics = append(result.Diagnostics, diagnostic(reason, p, "Capture is incomplete at this path"))
		}
	}
	read := func(p string) {
		if _, ok := result.Files[p]; ok {
			return
		}
		if len(result.Files) >= config.MaxFiles {
			exclude(p, "file_limit", true)
			return
		}
		raw, e := readRegular(filepath.Join(abs, filepath.FromSlash(p)), config.MaxFileBytes)
		if e != nil {
			exclude(p, "read_failure", true)
			result.Diagnostics = append(result.Diagnostics, diagnostic("read_failure_detail", p, e.Error()))
			return
		}
		if total+int64(len(raw)) > config.MaxTotalBytes {
			exclude(p, "byte_limit", true)
			return
		}
		total += int64(len(raw))
		result.Files[p] = raw
	}
	err = filepath.WalkDir(abs, func(full string, entry fs.DirEntry, walkErr error) error {
		rel, e := filepath.Rel(abs, full)
		if e != nil {
			return e
		}
		p := filepath.ToSlash(rel)
		if walkErr != nil {
			exclude(p, "read_failure", true)
			return nil
		}
		entries[p] = entry.Type()
		if p != "." {
			if entry.Type()&os.ModeSymlink != 0 {
				exclude(p, "symlink", true)
				return nil
			}
			if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".haft" || entry.Name() == ".context" || entry.Name() == "node_modules") {
				exclude(p, "managed_or_dependency_directory", false)
				return filepath.SkipDir
			}
			if newIgnore(result.Files, config.IgnorePatterns).ignored(p, entry.IsDir()) {
				exclude(p, "ignore_rule", false)
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if entry.IsDir() {
			for _, name := range []string{".gitignore", ".haftignore"} {
				metadata := name
				if p != "." {
					metadata = p + "/" + name
				}
				if _, e := os.Lstat(filepath.Join(full, name)); e == nil {
					read(metadata)
				} else if !os.IsNotExist(e) {
					exclude(metadata, "read_failure", true)
				}
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			exclude(p, "non_regular", true)
			return nil
		}
		read(p)
		return nil
	})
	if err != nil {
		return result, err
	}
	for p, raw := range result.Files {
		current, e := readRegular(filepath.Join(abs, filepath.FromSlash(p)), config.MaxFileBytes)
		if e != nil || !bytes.Equal(raw, current) {
			exclude(p, "changed_during_capture", true)
		}
	}
	// Byte readback alone cannot notice a sibling added after its directory was
	// visited. Compare a second bounded traversal using the captured ignore basis.
	remaining := map[string]fs.FileMode{}
	for p, mode := range entries {
		remaining[p] = mode
	}
	ignore := newIgnore(result.Files, config.IgnorePatterns)
	_ = filepath.WalkDir(abs, func(full string, entry fs.DirEntry, walkErr error) error {
		rel, relErr := filepath.Rel(abs, full)
		if relErr != nil {
			exclude(".", "changed_during_capture", true)
			return nil
		}
		p := filepath.ToSlash(rel)
		if walkErr != nil {
			exclude(p, "changed_during_capture", true)
			return nil
		}
		mode, existed := remaining[p]
		if !existed || mode != entry.Type() {
			exclude(p, "changed_during_capture", true)
		}
		delete(remaining, p)
		if p != "." && entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".haft" || entry.Name() == ".context" || entry.Name() == "node_modules" || ignore.ignored(p, true)) {
			return filepath.SkipDir
		}
		return nil
	})
	for p := range remaining {
		exclude(p, "changed_during_capture", true)
	}
	sort.Slice(result.Exclusions, func(a, b int) bool {
		x, y := result.Exclusions[a], result.Exclusions[b]
		return x.Path+"\x00"+x.Reason < y.Path+"\x00"+y.Reason
	})
	sort.Slice(result.Diagnostics, func(a, b int) bool {
		x, y := result.Diagnostics[a], result.Diagnostics[b]
		return x.Path+"\x00"+x.Code < y.Path+"\x00"+y.Code
	})
	return result, nil
}
func readRegular(p string, limit int64) ([]byte, error) {
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return nil, err
	}
	if real != p {
		return nil, fmt.Errorf("captured file traverses a symlink")
	}
	before, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("not a regular non-symlink file")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return nil, fmt.Errorf("file changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("file exceeds byte limit")
	}
	after, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(opened, after) || after.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("file changed while reading")
	}
	return raw, nil
}
