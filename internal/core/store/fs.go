package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
)

type handle struct {
	project, root                   *os.Root
	lock                            *os.File
	projectPath                     string
	rootInfo, lockInfo, projectInfo fs.FileInfo
}

func (s Store) open(ctx context.Context, write bool) (*handle, error) {
	project, err := os.OpenRoot(s.Root)
	if err != nil {
		return nil, err
	}
	h := &handle{project: project, projectPath: s.Root}
	fail := func(err error) (*handle, error) { h.close(); return nil, err }
	h.projectInfo, err = project.Stat(".")
	if err != nil {
		return fail(err)
	}
	if write {
		if err := project.MkdirAll(".haft", 0755); err != nil {
			return fail(err)
		}
		if err := syncDir(project, "."); err != nil {
			return fail(err)
		}
	}
	info, err := project.Lstat(".haft")
	if err != nil {
		return fail(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fail(fmt.Errorf("unsafe_store_root: .haft must be a real directory"))
	}
	h.rootInfo = info
	h.root, err = project.OpenRoot(".haft")
	if err != nil {
		return fail(err)
	}
	// Read-only calls do not create canonical files, but share the same stable
	// ephemeral lock. It deliberately lives outside the disposable cache.
	if err := h.root.MkdirAll(".runtime", 0700); err != nil {
		return fail(err)
	}
	if err := rejectSymlinks(h.root, ".runtime/writer.lock", true); err != nil {
		return fail(err)
	}
	h.lock, err = h.root.OpenFile(".runtime/writer.lock", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if errors.Is(err, fs.ErrExist) {
		h.lock, err = h.root.OpenFile(".runtime/writer.lock", os.O_RDWR, 0600)
	}
	if err != nil {
		return fail(fmt.Errorf("open writer lock: %w", err))
	}
	h.lockInfo, err = h.lock.Stat()
	if err != nil {
		return fail(err)
	}
	if !h.lockInfo.Mode().IsRegular() {
		return fail(fmt.Errorf("unsafe_lock: not a regular file"))
	}
	wait := s.LockTimeout
	if wait <= 0 {
		wait = 3 * time.Second
	}
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		acquired, lockErr := tryLock(h.lock, write)
		if lockErr != nil {
			return fail(lockErr)
		}
		if acquired {
			break
		}
		tick := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			tick.Stop()
			return fail(ctx.Err())
		case <-deadline.C:
			tick.Stop()
			return fail(fmt.Errorf("lock_timeout"))
		case <-tick.C:
		}
	}
	if err := h.checkIdentity(); err != nil {
		return fail(err)
	}
	return h, nil
}
func (h *handle) close() {
	if h.lock != nil {
		_ = unlock(h.lock)
		_ = h.lock.Close()
	}
	if h.root != nil {
		_ = h.root.Close()
	}
	if h.project != nil {
		_ = h.project.Close()
	}
}
func (h *handle) checkIdentity() error {
	projectInfo, projectErr := os.Stat(h.projectPath)
	if projectErr != nil || !os.SameFile(projectInfo, h.projectInfo) {
		return fmt.Errorf("project_path_changed")
	}
	info, err := h.project.Lstat(".haft")
	if err != nil || !os.SameFile(info, h.rootInfo) {
		return fmt.Errorf("store_path_changed")
	}
	info, err = h.root.Lstat(".runtime/writer.lock")
	if err != nil || !os.SameFile(info, h.lockInfo) {
		return fmt.Errorf("lock_path_changed")
	}
	return nil
}
func safePath(p string) error {
	if p == "" || p == "." || path.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "../") || strings.ContainsAny(p, "\\\x00\r\n") {
		return fmt.Errorf("unsafe_path: %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == ".." || part == "." {
			return fmt.Errorf("unsafe_path: %q", p)
		}
	}
	for _, reserved := range []string{"transactions", ".runtime", ".cache", ".git"} {
		if p == reserved || strings.HasPrefix(p, reserved+"/") {
			return fmt.Errorf("reserved_path: %q", p)
		}
	}
	return nil
}
func rejectSymlinks(root *os.Root, p string, allowMissing bool) error {
	parts := strings.Split(p, "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) && allowMissing {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink_not_allowed: %s", current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("not_directory: %s", current)
		}
	}
	return nil
}
func regularBytes(root *os.Root, p string, max int64) ([]byte, error) {
	if err := rejectSymlinks(root, p, false); err != nil {
		return nil, err
	}
	f, err := root.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not_regular: %s", p)
	}
	if info.Size() > max {
		return nil, fmt.Errorf("file_budget_exceeded: %s", p)
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("file_budget_exceeded: %s", p)
	}
	return b, nil
}
func syncDir(root *os.Root, p string) error {
	f, err := root.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func ensureDirs(root *os.Root, p string, perm fs.FileMode) error {
	if p == "." {
		return nil
	}
	if err := rejectSymlinks(root, p, true); err != nil {
		return err
	}
	parts := strings.Split(p, "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		err := root.Mkdir(current, perm)
		if errors.Is(err, fs.ErrExist) {
			info, e := root.Lstat(current)
			if e != nil {
				return e
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("unsafe_directory: %s", current)
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := syncDir(root, path.Dir(current)); err != nil {
			return err
		}
	}
	return nil
}
func exclusiveFile(root *os.Root, p string, b []byte) error {
	if err := rejectSymlinks(root, p, true); err != nil {
		return err
	}
	f, err := root.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(b)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	return syncDir(root, path.Dir(p))
}

// publishBytes links a complete independent inode into the final name. Staged
// recovery bytes must never share the live carrier inode: manual in-place edits
// are permitted and must not destroy the prepared transaction's original bytes.
func publishBytes(root *os.Root, p string, b []byte) error {
	if err := rejectSymlinks(root, p, true); err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := ".runtime/publish-" + hex.EncodeToString(nonce[:])
	if err := exclusiveFile(root, temp, b); err != nil {
		return err
	}
	defer root.Remove(temp)
	if err := root.Link(temp, p); err != nil {
		return err
	}
	return syncDir(root, path.Dir(p))
}
func (s Store) maxFile() int64 {
	if s.MaxFileBytes > 0 {
		return s.MaxFileBytes
	}
	return 64 << 20
}
func (s Store) maxTotal() int64 {
	if s.MaxTotalBytes > 0 {
		return s.MaxTotalBytes
	}
	return 256 << 20
}
