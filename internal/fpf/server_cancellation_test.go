package fpf

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestHandleToolsCallPropagatesCancellationNotification(t *testing.T) {
	server := NewServer("test")
	started := make(chan struct{})
	observed := make(chan error, 1)
	server.SetV5Handler(func(
		ctx context.Context,
		_ string,
		_ json.RawMessage,
	) (string, error) {
		close(started)
		<-ctx.Done()
		observed <- ctx.Err()
		return "", ctx.Err()
	})

	done := make(chan struct{})
	go func() {
		server.handleRequest(JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "tools/call",
			ID:      "slow-request",
			Params: json.RawMessage(
				`{"name":"haft_query","arguments":{"action":"code_context"}}`,
			),
		})
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("tool handler did not start")
	}
	server.handleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/cancelled",
		Params: json.RawMessage(
			`{"requestId":"slow-request","reason":"client timeout"}`,
		),
	})

	select {
	case err := <-observed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler context error = %v, want canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not reach tool handler")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled tool handler did not return")
	}
}
