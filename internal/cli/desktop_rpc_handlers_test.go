package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/spf13/cobra"
)

type testProjectRPCHandler func(*rpcEnv, io.Writer) error

func TestHandleAddProject(t *testing.T) {
	t.Run("rejects normal directory without haft metadata", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		err := runProjectRPCHandlerError(t, handleAddProject, targetPath)
		if err == nil {
			t.Fatal("handleAddProject succeeded for a directory without .haft/")
		}
		if !strings.Contains(err.Error(), "no .haft/ directory") {
			t.Fatalf("handleAddProject error = %q, want missing .haft/", err.Error())
		}

		_, statErr := os.Stat(filepath.Join(targetPath, ".haft"))
		if !os.IsNotExist(statErr) {
			t.Fatalf(".haft/ stat error = %v, want not exists", statErr)
		}
	})

	t.Run("registers existing haft project", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		cfg := createProjectConfig(t, targetPath)

		got := runProjectRPCHandler(t, handleAddProject, targetPath)
		if got.Path != targetPath {
			t.Fatalf("Path = %q, want %q", got.Path, targetPath)
		}
		if got.ID != cfg.ID {
			t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
		}
		if got.Name != cfg.Name {
			t.Fatalf("Name = %q, want %q", got.Name, cfg.Name)
		}

		requireRegisteredProject(t, targetPath, cfg.ID)
	})
}

func TestHandleAddProjectSmart(t *testing.T) {
	t.Run("initializes and registers normal directory", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		got := runProjectRPCHandler(t, handleAddProjectSmart, targetPath)

		cfg := requireProjectConfig(t, targetPath)
		if got.Path != targetPath {
			t.Fatalf("Path = %q, want %q", got.Path, targetPath)
		}
		if got.ID != cfg.ID {
			t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
		}

		dbPath, err := cfg.DBPath()
		if err != nil {
			t.Fatalf("DBPath: %v", err)
		}
		if _, err := os.Stat(dbPath); err != nil {
			t.Fatalf("database stat: %v", err)
		}

		requireRegisteredProject(t, targetPath, cfg.ID)
	})

	t.Run("registers existing project without initialization", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		cfg := createProjectConfig(t, targetPath)

		got := runProjectRPCHandler(t, handleAddProjectSmart, targetPath)
		if got.Path != targetPath {
			t.Fatalf("Path = %q, want %q", got.Path, targetPath)
		}
		if got.ID != cfg.ID {
			t.Fatalf("ID = %q, want existing ID %q", got.ID, cfg.ID)
		}

		dbPath, err := cfg.DBPath()
		if err != nil {
			t.Fatalf("DBPath: %v", err)
		}
		if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
			t.Fatalf("database stat error = %v, want not exists", err)
		}

		requireRegisteredProject(t, targetPath, cfg.ID)
	})
}

func TestDesktopRPCAddProjectSmart(t *testing.T) {
	setRPCProjectHome(t)

	activePath := createInitializedProject(t)
	t.Setenv("HAFT_PROJECT_ROOT", activePath)

	targetPath := t.TempDir()
	cmd := desktopRPCSubcommand(t, "add-project-smart")
	output := bytes.Buffer{}
	cmd.SetOut(&output)

	restore := setRPCInput(t, map[string]string{"path": targetPath})
	defer restore()

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("add-project-smart command: %v", err)
	}

	got := decodeProjectRPCResult(t, output.Bytes())
	cfg := requireProjectConfig(t, targetPath)
	if got.Path != targetPath {
		t.Fatalf("Path = %q, want %q", got.Path, targetPath)
	}
	if got.ID != cfg.ID {
		t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
	}
}

func runProjectRPCHandler(t *testing.T, handler testProjectRPCHandler, path string) rpcProjectInfo {
	t.Helper()

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"path": path})
	defer restore()

	if err := handler(&rpcEnv{}, &output); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	return decodeProjectRPCResult(t, output.Bytes())
}

func runProjectRPCHandlerError(t *testing.T, handler testProjectRPCHandler, path string) error {
	t.Helper()

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"path": path})
	defer restore()

	return handler(&rpcEnv{}, &output)
}

func decodeProjectRPCResult(t *testing.T, data []byte) rpcProjectInfo {
	t.Helper()

	var result rpcResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode rpc result: %v\n%s", err, string(data))
	}
	if !result.OK {
		t.Fatalf("rpc result error: %s", result.Error)
	}

	var info rpcProjectInfo
	if err := json.Unmarshal(result.Data, &info); err != nil {
		t.Fatalf("decode project info: %v", err)
	}

	return info
}

func setRPCInput(t *testing.T, payload any) func() {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal rpc input: %v", err)
	}

	inputFile, err := os.CreateTemp(t.TempDir(), "desktop-rpc-input-*.json")
	if err != nil {
		t.Fatalf("create rpc input: %v", err)
	}
	if _, err := inputFile.Write(data); err != nil {
		t.Fatalf("write rpc input: %v", err)
	}
	if _, err := inputFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek rpc input: %v", err)
	}

	original := os.Stdin
	os.Stdin = inputFile

	return func() {
		os.Stdin = original
		_ = inputFile.Close()
	}
}

func setRPCProjectHome(t *testing.T) string {
	t.Helper()

	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	t.Setenv("USERPROFILE", homePath)

	return homePath
}

func createProjectConfig(t *testing.T, rootPath string) *project.Config {
	t.Helper()

	haftDir := filepath.Join(rootPath, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft/: %v", err)
	}

	cfg, err := project.Create(haftDir, rootPath)
	if err != nil {
		t.Fatalf("create project config: %v", err)
	}

	return cfg
}

func createInitializedProject(t *testing.T) string {
	t.Helper()

	rootPath := t.TempDir()
	cfg := createProjectConfig(t, rootPath)

	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	_ = database.Close()

	return rootPath
}

func requireProjectConfig(t *testing.T, rootPath string) *project.Config {
	t.Helper()

	cfg, err := project.Load(filepath.Join(rootPath, ".haft"))
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if cfg == nil {
		t.Fatalf("project config missing for %s", rootPath)
	}

	return cfg
}

func requireRegisteredProject(t *testing.T, path string, id string) {
	t.Helper()

	reg, err := rpcLoadRegistry()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	for _, registered := range reg.Projects {
		if registered.Path == path && registered.ID == id {
			return
		}
	}

	t.Fatalf("registry missing project path=%q id=%q: %#v", path, id, reg.Projects)
}

func desktopRPCSubcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()

	for _, cmd := range desktopRPCCmd.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}

	t.Fatalf("desktop-rpc command %q is not registered", name)
	return nil
}
