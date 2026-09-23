// Package host installs project-local delivery assets. It does not interpret
// project records or alter any user-level host settings.
package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	Root, Binary, SourceRoot, SourceRepository string
	Codex                                      bool
}
type Result struct {
	Root      string   `json:"root"`
	Created   []string `json:"created"`
	Updated   []string `json:"updated"`
	Unchanged []string `json:"unchanged"`
}
type planned struct {
	path          string
	before, after []byte
	exists        bool
}

// Init preflights every destination before writing. Identical reruns are no-ops;
// a differing skill or managed block is a conflict, never a clobber permission.
func Init(c Config) (Result, error) {
	r := Result{Created: []string{}, Updated: []string{}, Unchanged: []string{}}
	root, err := filepath.Abs(c.Root)
	if err != nil {
		return r, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return r, err
	}
	r.Root = root
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return r, fmt.Errorf("project root must be an existing directory")
	}
	files := map[string][]byte{"AGENTS.md": []byte(agents)}
	blocks := map[string][2]string{"AGENTS.md": {"<!-- haft10:start -->", "<!-- haft10:end -->"}}
	if c.Codex {
		if !filepath.IsAbs(c.Binary) {
			return r, fmt.Errorf("candidate binary must be absolute")
		}
		bi, e := os.Stat(c.Binary)
		if e != nil || !bi.Mode().IsRegular() || bi.Mode()&0111 == 0 {
			return r, fmt.Errorf("candidate binary must be an existing executable")
		}
		if c.SourceRoot != "" {
			c.SourceRoot, e = filepath.Abs(c.SourceRoot)
			if e != nil {
				return r, e
			}
		}
		args := []string{"serve", "--root", root}
		if c.SourceRoot != "" {
			args = append(args, "--source-root", c.SourceRoot)
		}
		if c.SourceRepository != "" {
			args = append(args, "--source-repository", c.SourceRepository)
		}
		command, _ := json.Marshal(c.Binary)
		argv, _ := json.Marshal(args)
		files[".codex/config.toml"] = []byte("# haft10:start\n[mcp_servers.haft10]\ncommand = " + string(command) + "\nargs = " + string(argv) + "\n# haft10:end\n")
		blocks[".codex/config.toml"] = [2]string{"# haft10:start", "# haft10:end"}
		for name, body := range Skills() {
			files[".agents/skills/"+name+"/SKILL.md"] = []byte(body)
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var plan []planned
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := safePath(root, path); err != nil {
			return r, err
		}
		before, err := os.ReadFile(path)
		exists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return r, err
		}
		after := files[name]
		if marker, ok := blocks[name]; ok && exists {
			if name == "AGENTS.md" && bytes.Contains(before, []byte("<!-- haft:start -->")) {
				return r, fmt.Errorf("host_conflict: AGENTS.md already contains a different managed configuration")
			}
			after, err = mergeBlock(before, after, marker[0], marker[1])
			if err != nil {
				return r, fmt.Errorf("host_conflict: %s: %w", name, err)
			}
			if name == ".codex/config.toml" {
				outside := bytes.Replace(before, files[name], nil, 1)
				if err := checkTOML(outside); err != nil {
					return r, err
				}
			}
		} else if exists && !bytes.Equal(before, after) {
			return r, fmt.Errorf("host_conflict: preserve existing %s", name)
		}
		plan = append(plan, planned{path, before, after, exists})
	}
	for _, p := range plan {
		name, _ := filepath.Rel(root, p.path)
		name = filepath.ToSlash(name)
		if p.exists && bytes.Equal(p.before, p.after) {
			r.Unchanged = append(r.Unchanged, name)
			continue
		}
		if err := safePath(root, p.path); err != nil {
			return r, err
		}
		if err := os.MkdirAll(filepath.Dir(p.path), 0755); err != nil {
			return r, err
		}
		// Recheck content after preflight so an intervening user edit is preserved.
		current, err := os.ReadFile(p.path)
		if p.exists {
			if err != nil || !bytes.Equal(current, p.before) {
				return r, fmt.Errorf("host_conflict: destination changed: %s", name)
			}
		} else if !os.IsNotExist(err) {
			return r, fmt.Errorf("host_conflict: destination appeared: %s", name)
		}
		if !p.exists {
			f, e := os.OpenFile(p.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if e != nil {
				return r, e
			}
			_, e = f.Write(p.after)
			closeErr := f.Close()
			if e != nil {
				return r, e
			}
			if closeErr != nil {
				return r, closeErr
			}
			r.Created = append(r.Created, name)
		} else {
			f, e := os.CreateTemp(filepath.Dir(p.path), ".haft10-init-*")
			if e != nil {
				return r, e
			}
			tmp := f.Name()
			existingInfo, statErr := os.Stat(p.path)
			if statErr != nil {
				f.Close()
				os.Remove(tmp)
				return r, statErr
			}
			if e = f.Chmod(existingInfo.Mode().Perm()); e == nil {
				_, e = f.Write(p.after)
			}
			if closeErr := f.Close(); e == nil {
				e = closeErr
			}
			if e == nil {
				e = os.Rename(tmp, p.path)
			}
			_ = os.Remove(tmp)
			if e != nil {
				return r, e
			}
			r.Updated = append(r.Updated, name)
		}
	}
	return r, nil
}
func safePath(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("host_path_outside_root")
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("host_symlink: %s", current)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("host_special_file: %s", current)
		}
	}
	return nil
}
func mergeBlock(before, block []byte, start, end string) ([]byte, error) {
	s, e := bytes.Count(before, []byte(start)), bytes.Count(before, []byte(end))
	if s != 0 || e != 0 {
		if s == 1 && e == 1 && bytes.Count(before, block) == 1 {
			return append([]byte(nil), before...), nil
		}
		return nil, fmt.Errorf("managed block differs or is malformed")
	}
	after := append([]byte(nil), before...)
	if len(after) > 0 && after[len(after)-1] != '\n' {
		after = append(after, '\n')
	}
	if len(after) > 0 {
		after = append(after, '\n')
	}
	return append(after, block...), nil
}

// Merge only table-oriented TOML we can establish is outside our table family.
// Inline root mcp_servers, multiline values and escaped keys require manual
// preparation; rejection leaves all files unchanged rather than guessing a merge.
func checkTOML(raw []byte) error {
	parent := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "\"\"\"") || strings.Contains(line, "'''") {
			return fmt.Errorf("host_conflict: multiline TOML requires manual preparation")
		}
		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end < 0 {
				return fmt.Errorf("host_conflict: unsupported TOML table")
			}
			key, err := tomlKey(strings.Trim(line[1:end], "[]"))
			if err != nil {
				return err
			}
			parent = key
			if key == "mcp_servers.haft10" || strings.HasPrefix(key, "mcp_servers.haft10.") {
				return fmt.Errorf("host_conflict: mcp_servers.haft10 already exists outside managed block")
			}
		} else {
			key, _, ok := strings.Cut(line, "=")
			if !ok {
				return fmt.Errorf("host_conflict: unsupported multiline TOML")
			}
			key, err := tomlKey(strings.TrimSpace(key))
			if err != nil {
				return err
			}
			if parent != "" {
				key = parent + "." + key
			}
			if key == "mcp_servers" || key == "mcp_servers.haft10" || strings.HasPrefix(key, "mcp_servers.haft10.") {
				return fmt.Errorf("host_conflict: MCP key requires manual preparation")
			}
		}
	}
	return nil
}
func tomlKey(s string) (string, error) {
	parts := strings.Split(s, ".")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "\"") {
			v, err := strconv.Unquote(p)
			if err != nil {
				return "", fmt.Errorf("host_conflict: unsupported quoted TOML key")
			}
			p = v
		} else if strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'") {
			p = strings.Trim(p, "'")
		}
		if p == "" || strings.ContainsAny(p, "\"'[]\\") {
			return "", fmt.Errorf("host_conflict: unsupported TOML key")
		}
		parts[i] = p
	}
	return strings.Join(parts, "."), nil
}
