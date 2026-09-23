package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/m0n0x41d/haft/internal/core/app"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestClosedDecode(t *testing.T) {
	tests := []struct{ raw, want string }{
		{`{"format":"haft.api/1","operation":"recall","operation":"remember"}`, "duplicate_field"},
		{`{"format":"haft.api/1","operation":"recall","wat":1}`, "unknown_field"},
		{`{"format":"haft.api/1","Operation":"recall"}`, "unknown_field"},
		{`{"format":"haft.api/1","operation":"context","code_config":{"goos":"linux","wat":1}}`, "unknown_field"},
		{`{"format":"haft.api/1","operation":"change","metadata":{"target":{"operator_confirmed":true,"OperatorConfirmed":false}}}`, "unknown_field"},
		{`{"format":"haft.api/1","operation":null}`, "null scalar"},
		{`{"format":"haft.api/1","operation":"recall"} {}`, "trailing_json"},
		{`[]`, "expected an object"},
		{`{"format":`, "invalid_json"},
		{`{"snapshots":{"x":"","\u0078":""}}`, "duplicate_field"},
		{`{"carrier":"` + string([]byte{255}) + `"}`, "invalid_utf8"},
		{`{"x":` + strings.Repeat("[", maxDepth+2) + `0` + strings.Repeat("]", maxDepth+2) + `}`, "input_limit"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			_, err := DecodeRequest([]byte(tc.raw))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s got %v", tc.want, err)
			}
		})
	}
	q, err := DecodeRequest([]byte(`{"format":"haft.api/1","operation":"recall","metadata":{"key":{"operator_confirmed":false}},"snapshots":{"sha256:abc":"AAE="}}`))
	if err != nil || !bytes.Equal(q.Snapshots["sha256:abc"], []byte{0, 1}) {
		t.Fatal(q, err)
	}
	if _, err := ReadInput(strings.NewReader(strings.Repeat(" ", MaxInputBytes+1))); err == nil {
		t.Fatal("missing input bound")
	}
}
func TestSchemaMatchesRequestFields(t *testing.T) {
	s := RequestSchema()
	props := s["properties"].(map[string]any)
	if len(props) != len(fieldTypes(reflect.TypeOf(app.Request{}))) || s["additionalProperties"] != false {
		t.Fatal("schema drift")
	}
	for _, name := range []string{"observation", "metadata", "revision", "snapshots", "capture_code", "prior_code"} {
		if props[name] == nil {
			t.Fatalf("hidden field %s", name)
		}
	}
}
func initialize() string {
	return `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n"
}
func call(id int, q app.Request) string {
	b, _ := json.Marshal(q)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"haft","arguments":%s}}`+"\n", id, b)
}
func rpcRead(t *testing.T, r *bufio.Reader) map[string]any {
	t.Helper()
	line, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err = json.Unmarshal(line, &m); err != nil {
		t.Fatal(err, string(line))
	}
	return m
}

type client struct {
	in   *io.PipeWriter
	out  *bufio.Reader
	done chan error
}

func newClient(t *testing.T, s app.Service) *client {
	t.Helper()
	input, w := io.Pipe()
	reader, output := io.Pipe()
	c := &client{w, bufio.NewReader(reader), make(chan error, 1)}
	go func() {
		c.done <- (Server{Service: s, Version: "test"}).Serve(context.Background(), input, output)
		output.Close()
	}()
	t.Cleanup(func() {
		w.Close()
		if err := <-c.done; err != nil {
			t.Error(err)
		}
		reader.Close()
	})
	fmt.Fprint(w, initialize())
	rpcRead(t, c.out)
	return c
}
func TestPersistentClientsFreshStateAndParity(t *testing.T) {
	s := app.Service{Root: t.TempDir()}
	a := newClient(t, s)
	b := newClient(t, s)
	q := app.Request{Format: app.Format, Operation: "recall"}
	fmt.Fprint(a.in, call(2, q))
	before := rpcRead(t, a.out)
	write := app.Request{Format: app.Format, Operation: "remember", RequestID: "transport-note", Carrier: "---\nkind: note\ntitle: Adapter observation\nabout: domain:Order\n---\nBounded test note.\n"}
	fmt.Fprint(b.in, call(2, write))
	written := rpcRead(t, b.out)
	if written["result"].(map[string]any)["structuredContent"].(map[string]any)["result_kind"] != "written" {
		t.Fatal(written)
	}
	fmt.Fprint(a.in, call(3, q))
	after := rpcRead(t, a.out)
	actual := after["result"].(map[string]any)["structuredContent"]
	raw, _ := json.Marshal(s.Execute(context.Background(), q))
	var want any
	json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("API/MCP divergence: %v vs %v", actual, want)
	}
	if reflect.DeepEqual(before["result"], after["result"]) {
		t.Fatal("persistent reader pinned stale state")
	}
	textResult := after["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var textValue any
	json.Unmarshal([]byte(textResult), &textValue)
	if !reflect.DeepEqual(textValue, actual) {
		t.Fatal("MCP text/structured parity")
	}
}
func TestNotificationsErrorsAndFrameRecovery(t *testing.T) {
	s := app.Service{Root: t.TempDir()}
	var in strings.Builder
	in.WriteString(initialize())
	in.WriteString(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"haft","arguments":{"format":"haft.api/1","operation":"remember","carrier":"untrusted notification"}}}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"haft","arguments":{"format":"haft.api/1","operation":"recall","extra":1}}}` + "\n")
	in.WriteString(strings.Repeat("x", MaxInputBytes+1) + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":4,"method":"ping"}` + "\n")
	var out bytes.Buffer
	if err := (Server{Service: s}).Serve(context.Background(), strings.NewReader(in.String()), &out); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 5 {
		t.Fatalf("notification output/stream recovery: %d", len(lines))
	}
	if !bytes.Contains(lines[1], []byte(`"name":"haft"`)) || !bytes.Contains(lines[2], []byte("unknown_field")) || !bytes.Contains(lines[3], []byte("input_limit")) || !bytes.Contains(lines[4], []byte(`"id":4`)) {
		t.Fatal("incorrect MCP response")
	}
}
