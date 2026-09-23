package host

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func config(t *testing.T) Config {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Config{Root: t.TempDir(), Binary: binary, SourceRoot: t.TempDir(), SourceRepository: "fixture", Codex: true}
}
func write(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestInitPreservesConfigAndIsIdempotent(t *testing.T) {
	c := config(t)
	cfg := "model = \"example\"\n# operator notes\n[mcp_servers.private]\ncommand = \"private-server\"\n"
	instructions := "# Project\nUser-written rules stay exactly.\n"
	write(t, c.Root, ".codex/config.toml", cfg)
	write(t, c.Root, "AGENTS.md", instructions)
	first, err := Init(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Created) != 4 || len(first.Updated) != 2 {
		t.Fatal(first)
	}
	before := map[string][]byte{}
	for _, name := range append(first.Created, first.Updated...) {
		raw, e := os.ReadFile(filepath.Join(c.Root, name))
		if e != nil {
			t.Fatal(e)
		}
		before[name] = raw
	}
	if !bytes.HasPrefix(before[".codex/config.toml"], []byte(cfg)) || !bytes.HasPrefix(before["AGENTS.md"], []byte(instructions)) {
		t.Fatal("operator content rewritten")
	}
	stat, err := os.Stat(filepath.Join(c.Root, ".codex/config.toml"))
	if err != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("changed existing private mode")
	}
	if !bytes.Contains(before[".codex/config.toml"], []byte(c.Binary)) || !bytes.Contains(before[".codex/config.toml"], []byte("--source-root")) {
		t.Fatal("non-exact command")
	}
	second, err := Init(c)
	if err != nil || len(second.Created) != 0 || len(second.Updated) != 0 || len(second.Unchanged) != 6 {
		t.Fatal(second, err)
	}
	for name, want := range before {
		got, _ := os.ReadFile(filepath.Join(c.Root, name))
		if !bytes.Equal(got, want) {
			t.Fatalf("rerun changed %s", name)
		}
	}
}
func TestConflictsPreflightWithoutWrites(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{".agents/skills/h-spec/SKILL.md", "operator skill"},
		{".codex/config.toml", "[mcp_servers.haft10]\ncommand = \"owned\"\n"},
		{".codex/config.toml", "[\"mcp_servers\" . 'haft10']\ncommand = \"owned\"\n"},
		{".codex/config.toml", "mcp_servers = {haft10 = { command = \"owned\" }}\n"},
		{"AGENTS.md", "<!-- haft10:start -->\nUser edits\n<!-- haft10:end -->\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := config(t)
			write(t, c.Root, tc.name, tc.body)
			_, err := Init(c)
			if err == nil {
				t.Fatal("missing conflict")
			}
			raw, _ := os.ReadFile(filepath.Join(c.Root, tc.name))
			if string(raw) != tc.body {
				t.Fatal("clobber")
			}
			if tc.name != "AGENTS.md" {
				if _, err := os.Stat(filepath.Join(c.Root, "AGENTS.md")); !os.IsNotExist(err) {
					t.Fatal("wrote before complete preflight")
				}
			}
		})
	}
}
func TestSymlinksDoNotEscapeAndTemplatesAreBounded(t *testing.T) {
	c := config(t)
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(c.Root, ".agents")); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(c); err == nil {
		t.Fatal("followed symlink")
	}
	entries, _ := os.ReadDir(target)
	if len(entries) != 0 {
		t.Fatal("external write")
	}
	for name, text := range Skills() {
		if !strings.Contains(text, "name: "+name) || !strings.Contains(text, "description:") {
			t.Fatal("undiscoverable skill")
		}
		for _, retired := range []string{"ProjectTypeEnv", "haft_query", "haft_method", "haft agent", "typed_memory", "SQLite", "FTS5"} {
			if strings.Contains(text, retired) {
				t.Fatalf("%s contains %s", name, retired)
			}
		}
	}
}
