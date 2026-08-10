//go:build darwin || linux

package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/spf13/cobra"
)

// This fixture runs four race-instrumented server subprocesses. Its bound is
// deliberately separate from the product status deadline, which has a focused
// contract test; here the observable is single-flight completion, not latency.
const stdioSingleFlightResponseTimeout = 60 * time.Second

func TestServeStdioMultiProcessAutomaticMigrationSingleFlight(t *testing.T) {
	root := setupSpecSyncProject(t)
	input, err := absProjectRootInput(root, "stdio-migration-singleflight-test")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := resolveProjectBindingFromInput(input, "")
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", binding.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	_, mutationErr := database.Exec(`
		DELETE FROM schema_version WHERE version = 58;
		INSERT INTO artifacts (
			id, kind, title, content, created_at, updated_at
		) VALUES (
			'stdio-migration-fixture', 'decision', 'fixture', 'fixture',
			'2026-08-10T00:00:00Z', '2026-08-10T00:00:00Z'
		);
		INSERT INTO affected_files (artifact_id, file_path)
		VALUES ('stdio-migration-fixture', '/legacy/stdio.go')`)
	closeErr := database.Close()
	if mutationErr != nil || closeErr != nil {
		t.Fatalf(
			"prepare schema-57 stdio fixture: mutate=%v close=%v",
			mutationErr,
			closeErr,
		)
	}

	const servers = 4
	processes := make([]*stdioServeProcess, 0, servers)
	for range servers {
		processes = append(processes, startStdioServeProcess(t, root))
	}
	for _, process := range processes {
		process.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
		process.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"haft_query","arguments":{"action":"status","view":"governor"}}}`)
	}
	for _, process := range processes {
		response := process.waitForResponse(t, 2)
		if response.Error != nil || response.Result == nil {
			t.Fatalf("stdio status after migration failed: %#v", response)
		}
		var toolResult struct {
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(response.Result, &toolResult); err != nil {
			t.Fatal(err)
		}
		if toolResult.IsError {
			t.Fatalf("stdio status after migration is an error: %s", response.Result)
		}
	}
	for _, process := range processes {
		process.closeAndWait(t)
		if strings.Contains(strings.ToUpper(process.stderr.String()), "SQLITE_BUSY") {
			t.Fatalf("stdio migration server reported SQLite contention: %s", process.stderr.String())
		}
	}

	check, err := sql.Open("sqlite", "file:"+binding.DBPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var frontier int
	if err := check.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&frontier); err != nil {
		t.Fatal(err)
	}
	if frontier != 58 {
		t.Fatalf("stdio-migrated schema frontier = %d, want 58", frontier)
	}
	var invalidRows int
	if err := check.QueryRow(
		"SELECT COUNT(*) FROM affected_files WHERE file_path = '/legacy/stdio.go'",
	).Scan(&invalidRows); err != nil {
		t.Fatal(err)
	}
	if invalidRows != 0 {
		t.Fatalf("stdio-migrated invalid path rows = %d, want 0", invalidRows)
	}
	backups, err := filepath.Glob(
		filepath.Join(filepath.Dir(binding.DBPath), "*.pre-serve-migration-v57-to-v58-*.bak"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("stdio migration backups = %v, want exactly one", backups)
	}
}

func TestServeStdioMultiProcessIndexSingleFlight(t *testing.T) {
	root := setupSpecSyncProject(t)
	sourcePath := filepath.Join(root, "sample.go")
	if err := os.WriteFile(
		sourcePath,
		[]byte("package sample\nfunc A() {}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	input, err := absProjectRootInput(root, "stdio-singleflight-test")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := resolveProjectBindingFromInput(input, "")
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := openServeProjectLedger(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	store := artifact.NewStore(ledger.Database())
	coordinator, err := codeintel.NewProjectIndexCoordinator(
		codeintel.ProjectIndexCoordinates{
			ProjectID:   ledger.ProjectID().String(),
			ProjectRoot: ledger.ProjectRoot().String(),
			LedgerPath:  ledger.DatabasePath(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	service := codeintel.NewServiceWithIndexCoordinator(store, coordinator)
	initial, err := service.EnsureIndex(context.Background(), root)
	if err != nil || initial.Outcome != codeintel.IndexRebuiltPublished {
		t.Fatalf("initial index = %#v, %v", initial, err)
	}
	if err := os.WriteFile(
		sourcePath,
		[]byte("package sample\nfunc A() {}\nfunc B() {}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	const servers = 4
	processes := make([]*stdioServeProcess, 0, servers)
	for range servers {
		processes = append(processes, startStdioServeProcess(t, root))
	}
	for _, process := range processes {
		process.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
		process.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"haft_query","arguments":{"action":"status","view":"governor"}}}`)
	}
	for _, process := range processes {
		response := process.waitForResponse(t, 2)
		if response.Error != nil {
			t.Fatalf("stdio status failed: %#v", response.Error)
		}
		if response.Result == nil {
			t.Fatalf("stdio status returned no result: %#v", response)
		}
		var toolResult struct {
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(response.Result, &toolResult); err != nil {
			t.Fatal(err)
		}
		if toolResult.IsError {
			t.Fatalf("stdio status tool result is an error: %s", response.Result)
		}
	}

	waitForPublishedCodeIndexEpoch(
		t,
		service,
		initial.PublishedEpoch+1,
	)
	var completeAfter int
	if err := ledger.Database().QueryRow(`
		SELECT COUNT(*) FROM code_index_epochs
		WHERE status = 'complete' AND epoch > ?`,
		initial.PublishedEpoch,
	).Scan(&completeAfter); err != nil {
		t.Fatal(err)
	}
	if completeAfter != 1 {
		t.Fatalf("complete publications after stale source = %d, want 1", completeAfter)
	}

	for _, process := range processes {
		process.closeAndWait(t)
		if strings.Contains(strings.ToUpper(process.stderr.String()), "SQLITE_BUSY") {
			t.Fatalf("stdio server reported SQLite contention: %s", process.stderr.String())
		}
	}
}

// TestServeStdioProcessHelper runs one real stdio server in a subprocess. It is
// inert in ordinary package runs.
func TestServeStdioProcessHelper(t *testing.T) {
	if os.Getenv("HAFT_STDIO_SERVE_HELPER") != "1" {
		return
	}
	serveProjectRoot = ""
	serveExpectedProjectID = ""
	serveScopeID = ""
	command := &cobra.Command{}
	command.SetContext(context.Background())
	if err := runServe(command, nil); err != nil {
		t.Fatal(err)
	}
}

type stdioRPCResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type stdioServeProcess struct {
	command     *exec.Cmd
	stdin       io.WriteCloser
	responses   chan stdioRPCResponse
	readErr     chan error
	diagnostics chan string
	stderr      strings.Builder
}

func startStdioServeProcess(t *testing.T, root string) *stdioServeProcess {
	t.Helper()
	process := &stdioServeProcess{
		responses:   make(chan stdioRPCResponse, 8),
		readErr:     make(chan error, 1),
		diagnostics: make(chan string, 32),
	}
	process.command = exec.Command(
		os.Args[0],
		"-test.run=^TestServeStdioProcessHelper$",
	)
	process.command.Env = append(
		os.Environ(),
		"HAFT_STDIO_SERVE_HELPER=1",
		"HAFT_PROJECT_ROOT="+root,
	)
	stdin, err := process.command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process.command.Stderr = &process.stderr
	process.stdin = stdin
	if err := process.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if process.command.ProcessState == nil {
			_ = process.command.Process.Kill()
			_, _ = process.command.Process.Wait()
		}
	})
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var response stdioRPCResponse
			if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
				process.diagnostics <- scanner.Text()
				continue
			}
			process.responses <- response
		}
		process.readErr <- scanner.Err()
	}()
	return process
}

func (process *stdioServeProcess) send(t *testing.T, request string) {
	t.Helper()
	if _, err := fmt.Fprintln(process.stdin, request); err != nil {
		t.Fatal(err)
	}
}

func (process *stdioServeProcess) waitForResponse(
	t *testing.T,
	id int,
) stdioRPCResponse {
	t.Helper()
	timer := time.NewTimer(stdioSingleFlightResponseTimeout)
	defer timer.Stop()
	for {
		select {
		case response := <-process.responses:
			if response.ID == id {
				return response
			}
		case err := <-process.readErr:
			t.Fatalf("stdio server output ended before response %d: %v\noutput: %s\nstderr: %s", id, err, process.diagnosticText(), process.stderr.String())
		case <-timer.C:
			t.Fatalf("timed out waiting for stdio response %d\noutput: %s\nstderr: %s", id, process.diagnosticText(), process.stderr.String())
		}
	}
}

func (process *stdioServeProcess) diagnosticText() string {
	lines := make([]string, 0)
	for {
		select {
		case line := <-process.diagnostics:
			lines = append(lines, line)
		default:
			return strings.Join(lines, "\n")
		}
	}
}

func (process *stdioServeProcess) closeAndWait(t *testing.T) {
	t.Helper()
	if process.stdin != nil {
		if err := process.stdin.Close(); err != nil {
			t.Fatal(err)
		}
		process.stdin = nil
	}
	if err := process.command.Wait(); err != nil {
		t.Fatalf("stdio serve process failed: %v\nstderr: %s", err, process.stderr.String())
	}
}

func waitForPublishedCodeIndexEpoch(
	t *testing.T,
	service *codeintel.Service,
	want int64,
) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		state, err := service.CurrentIndexState(context.Background())
		if err == nil && state.Epoch == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	state, err := service.CurrentIndexState(context.Background())
	t.Fatalf("published epoch = %#v, %v; want %d", state, err, want)
}
