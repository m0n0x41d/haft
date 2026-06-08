//go:build !windows

package embedding

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedSidecarReusesOneDaemon(t *testing.T) {
	helper := writeSharedSidecarHelper(t)
	socketDir := filepath.Join("/tmp", fmt.Sprintf("haft-test-%d-%d", os.Getpid(), time.Now().UnixNano()))
	startLog := filepath.Join(t.TempDir(), "starts.log")
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })

	t.Setenv(sidecarBinaryEnv, helper)
	t.Setenv(sharedSocketDirEnv, socketDir)
	t.Setenv(sharedIdleTimeoutEnv, "1")
	t.Setenv(sharedStartupTimeoutEnv, "5")
	t.Setenv("HAFT_TEST_SHARED_SIDECAR_STARTS", startLog)

	first, err := New(Config{Provider: ProviderLocal, Model: "fake", Dim: 2})
	if err != nil {
		t.Fatalf("first New(local): %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	second, err := New(Config{Provider: ProviderLocal, Model: "fake", Dim: 2})
	if err != nil {
		t.Fatalf("second New(local): %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	firstVectors, err := first.Embed(context.Background(), RoleQuery, []string{"alpha"})
	if err != nil {
		t.Fatalf("first embed: %v", err)
	}
	secondVectors, err := second.Embed(context.Background(), RoleDocument, []string{"beta"})
	if err != nil {
		t.Fatalf("second embed: %v", err)
	}
	if len(firstVectors) != 1 || len(firstVectors[0]) != 2 {
		t.Fatalf("first vector shape = %v, want one 2-dim vector", firstVectors)
	}
	if len(secondVectors) != 1 || len(secondVectors[0]) != 2 {
		t.Fatalf("second vector shape = %v, want one 2-dim vector", secondVectors)
	}

	starts := readStartLog(t, startLog)
	if starts != 1 {
		t.Fatalf("daemon starts = %d, want 1", starts)
	}
}

func TestSharedSidecarDaemonDoesNotInheritClientCWD(t *testing.T) {
	helper := writeSharedSidecarHelper(t)
	socketDir := filepath.Join("/tmp", fmt.Sprintf("haft-test-%d-%d", os.Getpid(), time.Now().UnixNano()))
	cwdLog := filepath.Join(t.TempDir(), "cwd.log")
	clientDir := t.TempDir()
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(clientDir); err != nil {
		t.Fatalf("chdir client dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	t.Setenv(sidecarBinaryEnv, helper)
	t.Setenv(sharedSocketDirEnv, socketDir)
	t.Setenv(sharedIdleTimeoutEnv, "1")
	t.Setenv(sharedStartupTimeoutEnv, "5")
	t.Setenv("HAFT_TEST_SHARED_SIDECAR_CWD_LOG", cwdLog)

	embedder, err := New(Config{Provider: ProviderLocal, Model: "fake", Dim: 2})
	if err != nil {
		t.Fatalf("New(local): %v", err)
	}
	t.Cleanup(func() { _ = embedder.Close() })

	data, err := os.ReadFile(cwdLog)
	if err != nil {
		t.Fatalf("read cwd log: %v", err)
	}
	got := filepath.Clean(strings.TrimSpace(string(data)))
	want := filepath.Clean(socketDir)
	if got != want {
		t.Fatalf("daemon cwd = %q, want %q", got, want)
	}
}

func TestSharedSidecarDaemonDoesNotInheritClientStderr(t *testing.T) {
	helper := writeSharedSidecarHelper(t)
	socketDir := filepath.Join("/tmp", fmt.Sprintf("haft-test-%d-%d", os.Getpid(), time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })

	t.Setenv(sidecarBinaryEnv, helper)
	t.Setenv(sharedSocketDirEnv, socketDir)
	t.Setenv(sharedIdleTimeoutEnv, "10")
	t.Setenv(sharedStartupTimeoutEnv, "5")

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	t.Cleanup(func() { _ = stderrReader.Close() })

	originalStderr := os.Stderr
	os.Stderr = stderrWriter
	embedder, newErr := New(Config{Provider: ProviderLocal, Model: "fake", Dim: 2})
	os.Stderr = originalStderr

	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close captured stderr writer: %v", err)
	}
	if newErr != nil {
		t.Fatalf("New(local): %v", newErr)
	}
	t.Cleanup(func() { _ = embedder.Close() })

	type readResult struct {
		n   int
		err error
	}

	done := make(chan readResult, 1)
	go func() {
		buffer := make([]byte, 1)
		n, err := stderrReader.Read(buffer)
		done <- readResult{n: n, err: err}
	}()

	select {
	case result := <-done:
		if result.err != io.EOF {
			t.Fatalf("captured stderr read = n %d err %v, want EOF", result.n, result.err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("captured stderr pipe stayed open; shared sidecar inherited client stderr")
	}
}

func TestSidecarSpecFromConfigAbsolutizesCacheDir(t *testing.T) {
	clientDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(clientDir); err != nil {
		t.Fatalf("chdir client dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	spec := sidecarSpecFromConfig(Config{
		Provider: ProviderLocal,
		Model:    "fake",
		CacheDir: "models",
	})

	want, err := filepath.Abs("models")
	if err != nil {
		t.Fatalf("abs cache dir: %v", err)
	}
	if spec.CacheDir != want {
		t.Fatalf("cache dir = %q, want %q", spec.CacheDir, want)
	}
}

func writeSharedSidecarHelper(t *testing.T) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	path := filepath.Join(t.TempDir(), "fake-haft-embed")
	script := fmt.Sprintf(
		"#!/usr/bin/env bash\nHAFT_TEST_SHARED_SIDECAR=1 exec %q -test.run=TestSharedSidecarHelperProcess -- \"$@\"\n",
		binary,
	)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper script: %v", err)
	}
	return path
}

func readStartLog(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read start log: %v", err)
	}
	return len(strings.Fields(string(data)))
}

func TestSharedSidecarHelperProcess(t *testing.T) {
	if os.Getenv("HAFT_TEST_SHARED_SIDECAR") != "1" {
		return
	}
	args := sidecarHelperArgs(os.Args)
	socket := helperArg(args, "--serve-socket")
	model := helperArg(args, "--model")
	dim := helperIntArg(args, "--dim", 2)
	idle := time.Duration(helperIntArg(args, "--idle-timeout-secs", 30)) * time.Second
	if socket == "" {
		os.Exit(2)
	}
	if model == "" {
		model = DefaultLocalModel
	}

	logPath := os.Getenv("HAFT_TEST_SHARED_SIDECAR_STARTS")
	if logPath != "" {
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintln(file, os.Getpid())
			_ = file.Close()
		}
	}
	if cwdLogPath := os.Getenv("HAFT_TEST_SHARED_SIDECAR_CWD_LOG"); cwdLogPath != "" {
		if cwd, err := os.Getwd(); err == nil {
			_ = os.WriteFile(cwdLogPath, []byte(cwd+"\n"), 0o600)
		}
	}

	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		os.Exit(3)
	}
	defer listener.Close()
	_ = os.Chmod(socket, 0o600)

	unixListener := listener.(*net.UnixListener)
	active := atomic.Int32{}
	lastAccept := time.Now()
	for {
		_ = unixListener.SetDeadline(time.Now().Add(100 * time.Millisecond))
		conn, err := unixListener.Accept()
		if err != nil {
			if active.Load() == 0 && time.Since(lastAccept) >= idle {
				os.Exit(0)
			}
			continue
		}
		active.Add(1)
		lastAccept = time.Now()
		go handleSharedSidecarHelperConn(conn, model, dim, &active)
	}
}

func sidecarHelperArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return nil
}

func helperArg(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func helperIntArg(args []string, name string, fallback int) int {
	value := helperArg(args, name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func handleSharedSidecarHelperConn(conn net.Conn, model string, dim int, active *atomic.Int32) {
	defer active.Add(-1)
	defer conn.Close()
	reader := bufio.NewReader(conn)
	handshake := sidecarHandshake{Ready: true, Model: model, Dim: dim}
	line, err := json.Marshal(handshake)
	if err != nil {
		return
	}
	_, _ = conn.Write(append(line, '\n'))

	for {
		requestLine, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var request sidecarRequest
		if err := json.Unmarshal(requestLine, &request); err != nil {
			return
		}
		response := sidecarResponse{
			ID:      request.ID,
			Vectors: helperVectors(len(request.Texts), dim),
		}
		payload, err := json.Marshal(response)
		if err != nil {
			return
		}
		_, _ = conn.Write(append(payload, '\n'))
	}
}

func helperVectors(rows int, dim int) [][]float32 {
	vectors := make([][]float32, rows)
	for row := range vectors {
		vector := make([]float32, dim)
		if dim > 0 {
			vector[0] = 1
		}
		vectors[row] = vector
	}
	return vectors
}
