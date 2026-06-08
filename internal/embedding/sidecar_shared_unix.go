//go:build !windows

package embedding

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/m0n0x41d/haft/logger"
)

const (
	sharedSidecarProtocolVersion = "v1"
	sharedSocketDirEnv           = "HAFT_EMBED_SOCKET_DIR"
	sharedIdleTimeoutEnv         = "HAFT_EMBED_IDLE_TIMEOUT_SECS"
	sharedStartupTimeoutEnv      = "HAFT_EMBED_STARTUP_TIMEOUT_SECS"
	defaultSharedIdleTimeout     = 20 * time.Minute
	defaultSharedStartupTimeout  = 10 * time.Minute
)

type sharedSidecarAdapter struct {
	binary string
	spec   sidecarSpec
	socket string

	mu     sync.Mutex
	conn   io.ReadWriteCloser
	reader *bufio.Reader
	nextID uint64
	desc   Descriptor
	closed bool
}

type sharedDaemonStart struct {
	cmd  *exec.Cmd
	done <-chan error
}

func newSharedSidecarAdapter(binary string, spec sidecarSpec) (Embedder, error) {
	adapter, err := connectOrStartSharedSidecar(binary, spec)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func connectOrStartSharedSidecar(binary string, spec sidecarSpec) (*sharedSidecarAdapter, error) {
	socket, lock, err := sharedSidecarPaths(binary, spec)
	if err != nil {
		return nil, err
	}
	if adapter, err := dialSharedSidecar(binary, spec, socket); err == nil {
		return adapter, nil
	}

	return withSharedSidecarLock(lock, func() (*sharedSidecarAdapter, error) {
		if adapter, err := dialSharedSidecar(binary, spec, socket); err == nil {
			return adapter, nil
		}
		_ = os.Remove(socket)
		started, err := startSharedSidecarDaemon(binary, spec, socket)
		if err != nil {
			return nil, err
		}
		return waitForSharedSidecar(binary, spec, socket, started)
	})
}

func withSharedSidecarLock(
	lockPath string,
	fn func() (*sharedSidecarAdapter, error),
) (*sharedSidecarAdapter, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open shared sidecar lock: %w", err)
	}
	defer file.Close()
	_ = file.Chmod(0o600)

	if err := flockFile(file, syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("lock shared sidecar: %w", err)
	}
	defer func() { _ = flockFile(file, syscall.LOCK_UN) }()

	return fn()
}

func flockFile(file *os.File, how int) error {
	fd := file.Fd()
	if fd > (^uintptr(0) >> 1) {
		return fmt.Errorf("file descriptor %d exceeds int range", fd)
	}
	return syscall.Flock(int(fd), how) // #nosec G115 -- fd is range-checked above.
}

func dialSharedSidecar(binary string, spec sidecarSpec, socket string) (*sharedSidecarAdapter, error) {
	conn, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(5 * time.Second)
	_ = conn.SetDeadline(deadline)
	reader := bufio.NewReader(conn)
	handshake, err := readHandshake(reader)
	_ = conn.SetDeadline(time.Time{})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := validateSharedHandshake(spec, handshake); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &sharedSidecarAdapter{
		binary: binary,
		spec:   spec,
		socket: socket,
		conn:   conn,
		reader: reader,
		desc: Descriptor{
			Provider:   ProviderLocal,
			Model:      handshake.Model,
			Dimensions: handshake.Dim,
		},
	}, nil
}

func validateSharedHandshake(spec sidecarSpec, handshake sidecarHandshake) error {
	if handshake.Model != spec.Model {
		return fmt.Errorf("shared sidecar model = %q, want %q", handshake.Model, spec.Model)
	}
	if spec.Dim > 0 && handshake.Dim != spec.Dim {
		return fmt.Errorf("shared sidecar dim = %d, want %d", handshake.Dim, spec.Dim)
	}
	return nil
}

func startSharedSidecarDaemon(
	binary string,
	spec sidecarSpec,
	socket string,
) (sharedDaemonStart, error) {
	args := append([]string{}, spec.Args...)
	args = append(args, "--serve-socket", socket)
	args = append(args, "--idle-timeout-secs", strconv.Itoa(sharedIdleTimeoutSecs()))

	cmd := exec.Command(binary, args...)
	cmd.Dir = filepath.Dir(socket)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return sharedDaemonStart{}, fmt.Errorf("start shared sidecar: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Info().Err(err).Msg("shared embedding sidecar exited")
		}
		done <- err
	}()

	return sharedDaemonStart{cmd: cmd, done: done}, nil
}

func waitForSharedSidecar(
	binary string,
	spec sidecarSpec,
	socket string,
	started sharedDaemonStart,
) (*sharedSidecarAdapter, error) {
	timeout := sharedStartupTimeout()
	deadline := time.Now().Add(timeout)
	for {
		if adapter, err := dialSharedSidecar(binary, spec, socket); err == nil {
			logger.Info().
				Int("pid", started.cmd.Process.Pid).
				Str("model", adapter.desc.Model).
				Int("dim", adapter.desc.Dimensions).
				Msg("shared embedding sidecar started")
			return adapter, nil
		}

		select {
		case err := <-started.done:
			if err == nil {
				return nil, fmt.Errorf("%w: exited before opening socket", errSharedSidecarUnusable)
			}
			return nil, fmt.Errorf("%w: exited before opening socket: %v", errSharedSidecarUnusable, err)
		default:
		}

		if timeout > 0 && time.Now().After(deadline) {
			_ = started.cmd.Process.Kill()
			return nil, fmt.Errorf("%w: startup timed out", errSharedSidecarUnusable)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func sharedIdleTimeoutSecs() int {
	duration := durationFromEnv(sharedIdleTimeoutEnv, defaultSharedIdleTimeout)
	return int(duration / time.Second)
}

func sharedStartupTimeout() time.Duration {
	return durationFromEnv(sharedStartupTimeoutEnv, defaultSharedStartupTimeout)
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func sharedSidecarPaths(binary string, spec sidecarSpec) (socket string, lock string, err error) {
	dir, err := sharedSidecarDir()
	if err != nil {
		return "", "", err
	}
	key := sharedSidecarKey(binary, spec)
	return filepath.Join(dir, key+".sock"), filepath.Join(dir, key+".lock"), nil
}

func sharedSidecarKey(binary string, spec sidecarSpec) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(sharedSidecarProtocolVersion))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(binary))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(spec.Model))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(spec.CacheDir))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strconv.Itoa(spec.Dim)))
	sum := hex.EncodeToString(hash.Sum(nil))
	return "embed-" + sum[:16]
}

func sharedSidecarDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(sharedSocketDirEnv)); override != "" {
		return ensurePrivateDir(override)
	}
	parent := filepath.Join("/tmp", fmt.Sprintf("haft-%d", os.Getuid()))
	dir, err := ensurePrivateDir(parent)
	if err != nil {
		return "", err
	}
	return ensurePrivateDir(filepath.Join(dir, "embed"))
}

func ensurePrivateDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create private dir %s: %w", dir, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", fmt.Errorf("stat private dir %s: %w", dir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("private dir %s is a symlink", dir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("private dir %s is not a directory", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("chmod private dir %s: %w", dir, err)
	}
	if err := validatePrivateDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func validatePrivateDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat private dir %s: %w", dir, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private dir %s is accessible by group/other", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if ok && int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("private dir %s is not owned by current user", dir)
	}
	return nil
}

func (a *sharedSidecarAdapter) Descriptor() Descriptor {
	return a.desc
}

func (a *sharedSidecarAdapter) Embed(
	ctx context.Context,
	role Role,
	texts []string,
) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, errors.New("embedding sidecar closed")
	}

	vectors, err := a.embedOnce(role, texts)
	if err == nil {
		return vectors, nil
	}
	logger.Warn().Err(err).Msg("shared embedding sidecar connection faulted — reconnecting")
	if err := a.reconnectLocked(); err != nil {
		return nil, fmt.Errorf("shared sidecar reconnect failed: %w", err)
	}
	return a.embedOnce(role, texts)
}

func (a *sharedSidecarAdapter) embedOnce(role Role, texts []string) ([][]float32, error) {
	a.nextID++
	request := sidecarRequest{ID: a.nextID, Task: taskFor(role), Texts: texts}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode embed request: %w", err)
	}
	if _, err := a.conn.Write(append(payload, '\n')); err != nil {
		return nil, fmt.Errorf("write embed request: %w", err)
	}

	line, err := a.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read embed response: %w", err)
	}
	var response sidecarResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("sidecar embed: %s", response.Error)
	}
	if len(response.Vectors) != len(texts) {
		return nil, fmt.Errorf("sidecar returned %d vectors for %d texts", len(response.Vectors), len(texts))
	}
	return response.Vectors, nil
}

func (a *sharedSidecarAdapter) reconnectLocked() error {
	if a.conn != nil {
		_ = a.conn.Close()
	}
	adapter, err := connectOrStartSharedSidecar(a.binary, a.spec)
	if err != nil {
		return err
	}
	a.conn = adapter.conn
	a.reader = adapter.reader
	a.desc = adapter.desc
	return nil
}

func (a *sharedSidecarAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}
