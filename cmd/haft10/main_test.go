package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/app"
)

func invoke(t *testing.T, args []string, input string) (int, map[string]any, string) {
	t.Helper()
	var out, log bytes.Buffer
	code := run(context.Background(), args, strings.NewReader(input), &out, &log)
	var value map[string]any
	if err := json.Unmarshal(out.Bytes(), &value); err != nil {
		t.Fatal(err, out.String(), log.String())
	}
	return code, value, log.String()
}
func TestCLIConvenienceAndAPIParity(t *testing.T) {
	root := t.TempDir()
	q := app.Request{Format: app.Format, Operation: "recall", Query: "bounded", Limit: 4}
	raw, _ := json.Marshal(q)
	code, api, log := invoke(t, []string{"api", "--root", root, "--input", "-"}, string(raw))
	if code != 0 || log != "" {
		t.Fatal(code, api, log)
	}
	code, cli, log := invoke(t, []string{"recall", "--root", root, "--query", "bounded", "--limit", "4"}, "")
	if code != 0 || log != "" || !reflect.DeepEqual(api, cli) {
		t.Fatal(code, api, cli, log)
	}
	wantRaw, _ := json.Marshal((app.Service{Root: root}).Execute(context.Background(), q))
	var want map[string]any
	json.Unmarshal(wantRaw, &want)
	if !reflect.DeepEqual(api, want) {
		t.Fatal("service parity")
	}
}
func TestCLIRejectsMalformedAndConflictingInputs(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		args        []string
		input, code string
	}{
		{[]string{"api", "--input", "-"}, `{"format":"haft.api/1","operation":"recall","wat":true}`, "input_decode"},
		{[]string{"api", "--input", "-"}, `{"operation":"recall","operation":"remember"}`, "input_decode"},
		{[]string{"recall", "--input", "-", "--limit", "4"}, `{"limit":0}`, "request_conflict"},
		{[]string{"check", "--input", "-", "--strict"}, `{"strict":false}`, "request_conflict"},
		{[]string{"api", "--query", "override", "--input", "-"}, `{"format":"haft.api/1","operation":"recall"}`, "invalid_arguments"},
		{[]string{"recall", "--input", "-"}, `{"format":"haft.api/1","operation":"remember"}`, "request_conflict"},
	} {
		args := append(tc.args, "--root", root)
		exit, value, log := invoke(t, args, tc.input)
		if exit != 2 || log == "" || value["diagnostics"].([]any)[0].(map[string]any)["code"] != tc.code {
			t.Fatal(exit, value, log)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".haft")); !os.IsNotExist(err) {
		t.Fatal("malformed input caused writes")
	}
}

// Exercise the actual OS entrypoint, stdout framing and persistent subprocesses,
// rather than proving parity only through the in-process helpers.
func TestActualBinaryCLIMCPAndLocalInit(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "haft10")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	root := t.TempDir()
	input := `{"format":"haft.api/1","operation":"recall"}`
	cli := exec.Command(binary, "api", "--input", "-", "--root", root)
	cli.Stdin = strings.NewReader(input)
	cliOut, err := cli.Output()
	if err != nil {
		t.Fatal(err)
	}
	mcp := exec.Command(binary, "serve", "--root", root)
	stdin, err := mcp.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := mcp.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	mcp.Stderr = &stderr
	if err = mcp.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	fmt.Fprintln(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"subprocess","version":"1"}}}`)
	if _, err = reader.ReadBytes('\n'); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(stdin, "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"haft\",\"arguments\":%s}}\n", input)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	stdin.Close()
	if err = mcp.Wait(); err != nil || stderr.Len() != 0 {
		t.Fatal(err, stderr.String())
	}
	var rpc map[string]any
	json.Unmarshal(line, &rpc)
	var cliResult any
	json.Unmarshal(cliOut, &cliResult)
	if !reflect.DeepEqual(rpc["result"].(map[string]any)["structuredContent"], cliResult) {
		t.Fatal("real CLI/MCP parity")
	}
	init := exec.Command(binary, "init", "--codex", "--root", root)
	if output, err := init.CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	cfg, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil || !bytes.Contains(cfg, []byte(binary)) {
		t.Fatal(err, string(cfg))
	}
	rerun := exec.Command(binary, "init", "--codex", "--root", root)
	if output, err := rerun.CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	for _, verb := range []string{"help", "version"} {
		output, err := exec.Command(binary, verb).Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, retired := range []string{"ProjectTypeEnv", "typed_memory", "haft_query", "haft_method", "SQLite"} {
			if bytes.Contains(output, []byte(retired)) {
				t.Fatal("retired help", retired)
			}
		}
	}
}

func TestMigrationCLIStagesOnlyAndReplays(t *testing.T) {
	source := t.TempDir()
	output := t.TempDir()
	original := []byte("---\nid: note-old\nkind: note\ntitle: Source observation\n---\nPreserve this exact text.\n")
	if err := os.MkdirAll(filepath.Join(source, "notes"), 0755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(source, "notes", "old.md")
	if err := os.WriteFile(sourcePath, original, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"migrate", "--dry-run", "--carrier-root", source, "--output-root", output, "--created-at", "2026-09-24T00:00:00Z"}
	code, first, log := invoke(t, args, "")
	if code != 0 || first["result_kind"] != "staged" {
		t.Fatal(code, first, log)
	}
	code, second, log := invoke(t, args, "")
	if code != 0 || second["result_kind"] != "replayed" {
		t.Fatal(code, second, log)
	}
	code, queue, log := invoke(t, []string{"migrate", "queue", "--output-root", output}, "")
	if code != 0 || queue["result_kind"] != "results" {
		t.Fatal(code, queue, log)
	}
	actual, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("source changed", err)
	}
	code, rejected, _ := invoke(t, []string{"migrate", "--carrier-root", source, "--output-root", output, "--created-at", "2026-09-24T00:00:00Z"}, "")
	if code == 0 || rejected["result_kind"] != "unsupported" {
		t.Fatal(code, rejected)
	}
}
