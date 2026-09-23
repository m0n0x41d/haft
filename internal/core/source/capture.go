package source

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Capture is the package's IO boundary. It reads only publication paths beneath
// root, using os.Root to prevent symlink escape, and requires two consecutive
// matching content manifests in at most three scans. It never fetches or opens a
// project database. Later edits do not change the returned reader.
func Capture(root, repository string, limits Limits) (*Reader, error) {
	return capture(root, repository, limits, nil)
}

// afterScan is private deterministic test injection for equal-metadata edits.
func capture(root, repository string, limits Limits, afterScan func(int)) (*Reader, error) {
	l, err := limits.normalized()
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("source_unavailable: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("source_root_not_directory")
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("source_unavailable: %w", err)
	}
	defer handle.Close()
	var previous *Reader
	for attempt := 0; attempt < 3; attempt++ {
		files, err := scan(handle, l)
		if err != nil {
			return nil, err
		}
		next, err := NewReader(repository, files, l)
		if err != nil {
			return nil, err
		}
		if afterScan != nil {
			afterScan(attempt)
		}
		if previous != nil && reflect.DeepEqual(previous.status.Manifest, next.status.Manifest) {
			return next, nil
		}
		previous = next
	}
	return nil, fmt.Errorf("source_changed_during_capture: no two consecutive content manifests matched in three scans")
}

func scan(root *os.Root, l Limits) ([]File, error) {
	files := []File{}
	entryBudget := l.MaxFiles * 16
	total := int64(0)
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if depth > 8 {
			return fmt.Errorf("source_depth_limit")
		}
		info, err := root.Lstat(dir)
		if err != nil {
			return fmt.Errorf("source_unavailable: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("source_symlink_or_non_directory: %s", dir)
		}
		d, err := root.Open(dir)
		if err != nil {
			return fmt.Errorf("source_unavailable: %w", err)
		}
		entries, readErr := d.ReadDir(entryBudget + 1)
		closeErr := d.Close()
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("source_unavailable: %w", readErr)
		}
		if closeErr != nil {
			return closeErr
		}
		entryBudget -= len(entries)
		if entryBudget < 0 {
			return fmt.Errorf("source_entry_limit")
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			name := entry.Name()
			p := name
			if dir != "." {
				p = dir + "/" + name
			}
			if dir != "." && entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("source_symlink: %s", p)
			}
			isSuite := dir == "." && name == "Engineering DPF Suite"
			if isSuite || dir != "." && entry.IsDir() {
				if err := walk(p, depth+1); err != nil {
					return err
				}
				continue
			}
			if !publicationPath(p) {
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("source_symlink: %s", p)
			}
			before, err := root.Lstat(filepath.FromSlash(p))
			if err != nil {
				return fmt.Errorf("source_unavailable: %w", err)
			}
			if !before.Mode().IsRegular() {
				return fmt.Errorf("source_not_regular: %s", p)
			}
			if before.Size() > l.MaxFileBytes {
				return fmt.Errorf("source_byte_limit: %s", p)
			}
			if len(files) >= l.MaxFiles {
				return fmt.Errorf("source_file_limit")
			}
			f, err := root.Open(filepath.FromSlash(p))
			if err != nil {
				return fmt.Errorf("source_unavailable: %w", err)
			}
			opened, statErr := f.Stat()
			if statErr != nil {
				f.Close()
				return statErr
			}
			if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
				f.Close()
				return fmt.Errorf("source_changed_during_capture: %s", p)
			}
			raw, readErr := io.ReadAll(io.LimitReader(f, l.MaxFileBytes+1))
			after, statErr := f.Stat()
			closeErr = f.Close()
			if readErr != nil {
				return readErr
			}
			if statErr != nil {
				return statErr
			}
			if closeErr != nil {
				return closeErr
			}
			if !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
				return fmt.Errorf("source_changed_during_capture: %s", p)
			}
			total += int64(len(raw))
			if int64(len(raw)) > l.MaxFileBytes || total > l.MaxBytes {
				return fmt.Errorf("source_byte_limit: %s", p)
			}
			files = append(files, File{Path: strings.ReplaceAll(p, string(filepath.Separator), "/"), Raw: raw})
		}
		return nil
	}
	if err := walk(".", 0); err != nil {
		return nil, err
	}
	return files, nil
}
