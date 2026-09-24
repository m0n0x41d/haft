package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Exercise the same requests through both public entrypoints, including the
// normal replay control. A warning diagnostic must not hide a refused replay.
func TestReplayOutcomesAgreeAcrossCLIAndMCP(t *testing.T) {
	for _, kind := range []string{"replayed", "request_conflict", "replay_conflict"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			q := map[string]any{"format": "haft.api/2", "operation": "remember", "request_id": "once", "carrier": "---\nkind: note\ntitle: Preserve edited note\nabout: domain:Replay\n---\nOriginal body.\n"}
			payload, _ := json.Marshal(q)
			exit, first, _ := invoke(t, []string{"api", "--root", root, "--input", "-"}, string(payload))
			if exit != 0 || first["result_kind"] != "written" {
				t.Fatal(exit, first)
			}
			paths := first["data"].(map[string]any)["paths"].([]any)
			var note string
			for _, path := range paths {
				if strings.HasPrefix(path.(string), "notes/") {
					note = filepath.Join(root, ".haft", path.(string))
				}
			}
			before, err := os.ReadFile(note)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "replay_conflict" {
				before = append(before, []byte("\nExternal authored change.\n")...)
				if err := os.WriteFile(note, before, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "request_conflict" {
				q["carrier"] = q["carrier"].(string) + "Changed request.\n"
				payload, _ = json.Marshal(q)
			}
			exit, cli, _ := invoke(t, []string{"api", "--root", root, "--input", "-"}, string(payload))
			wantExit := 1
			if kind == "replayed" {
				wantExit = 0
			}
			if exit != wantExit || cli["result_kind"] != kind {
				t.Errorf("CLI exit=%d result=%v; want %s/%d", exit, cli, kind, wantExit)
			}
			wire := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"regression","version":"1"}}}` + "\n"
			wire += fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"haft\",\"arguments\":%s}}\n", payload)
			var out, log bytes.Buffer
			if code := run(context.Background(), []string{"serve", "--root", root}, strings.NewReader(wire), &out, &log); code != 0 {
				t.Fatal(code, log.String())
			}
			scanner := bufio.NewScanner(&out)
			var rpc map[string]any
			for scanner.Scan() {
				if err := json.Unmarshal(scanner.Bytes(), &rpc); err != nil {
					t.Fatal(err)
				}
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			mcp := rpc["result"].(map[string]any)
			if mcp["isError"] != (wantExit != 0) || !reflect.DeepEqual(mcp["structuredContent"], cli) {
				t.Errorf("MCP error/parity mismatch: %v / %v", mcp, cli)
			}
			after, err := os.ReadFile(note)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("retry changed the carrier", err)
			}
		})
	}
}
