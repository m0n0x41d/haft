package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	_ "modernc.org/sqlite"
)

var supportedTables = map[string][]string{
	"artifacts":                {"id", "kind", "title", "content"},
	"artifact_links":           {"source_id", "target_id", "link_type"},
	"affected_files":           {"artifact_id", "file_path"},
	"affected_symbols":         {"artifact_id", "file_path", "symbol_name"},
	"artifact_symbol_bindings": {"artifact_id", "file_path", "symbol_name"},
	"evidence_items":           {"id", "content", "artifact_ref"},
	"evidence":                 {"id", "content", "holon_id"},
	"spec_section_editions":    {"section_id", "section_json"},
	"spec_section_baselines":   {"section_id", "hash"},
}

func diagnostic(code, path, message string) carrier.Diagnostic {
	return carrier.Diagnostic{Code: code, Path: path, Message: message, Severity: "warning"}
}
func maxBytes(r Request) int64 {
	if r.MaxBytes > 0 {
		return r.MaxBytes
	}
	return 256 << 20
}
func maxRows(r Request) int {
	if r.MaxRows > 0 {
		return r.MaxRows
	}
	return 10000
}
func regularFile(root *os.Root, p string, limit int64) ([]byte, error) {
	info, err := root.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("nonregular_source: %s", p)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("source_byte_budget: %s", p)
	}
	f, err := root.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("source_identity_changed: %s", p)
	}
	b := make([]byte, info.Size()+1)
	n, err := f.ReadAt(b, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if n != int(info.Size()) {
		return nil, fmt.Errorf("source_size_changed: %s", p)
	}
	return b[:n], nil
}
func capture(ctx context.Context, r Request) (Snapshot, error) {
	s := Snapshot{DatabasePath: r.DatabasePath, CarrierRoot: r.CarrierRoot, DatabaseFiles: map[string]string{}, Carriers: map[string][]byte{}}
	if r.DatabasePath == "" && r.CarrierRoot == "" {
		return s, fmt.Errorf("explicit database_path or carrier_root required")
	}
	if r.MaxRows < 0 || r.MaxBytes < 0 {
		return s, fmt.Errorf("capture limits cannot be negative")
	}
	if r.DatabasePath != "" {
		files, err := captureDatabaseFiles(r.DatabasePath, maxBytes(r))
		if err != nil {
			s.Scopes = append(s.Scopes, Scope{Source: "sqlite:" + r.DatabasePath, Disposition: "source_unreadable", Reason: err.Error()})
			s.Diagnostics = append(s.Diagnostics, diagnostic("source_unreadable", r.DatabasePath, err.Error()))
		} else {
			for p, b := range files {
				s.DatabaseFiles[p] = carrier.Digest(b)
			}
			dir, err := os.MkdirTemp("", "haft10-migration-source-")
			if err != nil {
				return s, err
			}
			func() {
				defer os.RemoveAll(dir)
				for name, b := range files {
					if err = os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
						return
					}
				}
				err = readSQLite(ctx, filepath.Join(dir, filepath.Base(r.DatabasePath)), r, &s)
			}()
			if err != nil {
				s.Scopes = append(s.Scopes, Scope{Source: "sqlite:" + r.DatabasePath, Disposition: "source_unreadable", Reason: err.Error()})
				s.Diagnostics = append(s.Diagnostics, diagnostic("sqlite_snapshot_unreadable", r.DatabasePath, err.Error()))
			}
		}
	}
	if r.CarrierRoot != "" {
		captureCarriers(r, &s)
	}
	sort.Slice(s.Rows, func(i, j int) bool {
		a, b := s.Rows[i], s.Rows[j]
		if a.Table != b.Table {
			return a.Table < b.Table
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		return bytes.Compare(x, y) < 0
	})
	sort.Slice(s.Scopes, func(i, j int) bool { return s.Scopes[i].Source < s.Scopes[j].Source })
	b, _ := json.Marshal(s)
	s.Digest = carrier.Digest(b)
	return s, nil
}
func captureDatabaseFiles(database string, limit int64) (map[string][]byte, error) {
	absolute, err := filepath.Abs(database)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlink_source_path")
	}
	absolute = resolved
	root, err := os.OpenRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	scan := func() (map[string][]byte, error) {
		out := map[string][]byte{}
		var total int64
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			p := absolute + suffix
			b, err := regularFile(root, filepath.Base(p), limit)
			if errors.Is(err, fs.ErrNotExist) && suffix != "" {
				continue
			}
			if err != nil {
				return nil, err
			}
			if suffix == "-journal" && len(b) > 0 {
				return nil, fmt.Errorf("rollback_journal_present: use a stable completed SQLite source")
			}
			total += int64(len(b))
			if total > limit {
				return nil, fmt.Errorf("database_capture_byte_budget")
			}
			out[filepath.Base(p)] = b
		}
		return out, nil
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		a, err := scan()
		if err != nil {
			lastErr = err
			continue
		}
		b, err := scan()
		if err != nil {
			lastErr = err
			continue
		}
		if equalFiles(a, b) {
			return b, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("database_capture_failed: %w", lastErr)
	}
	return nil, fmt.Errorf("unstable_database_files: DB/WAL/SHM presence or bytes moved during bounded capture")
}
func equalFiles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for p, v := range a {
		if !bytes.Equal(v, b[p]) {
			return false
		}
	}
	return true
}
func readSQLite(ctx context.Context, p string, r Request, s *Snapshot) error {
	u := url.URL{Scheme: "file", Path: p}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", "query_only(1)")
	q.Add("_pragma", "busy_timeout(2000)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var readonly int
	if err := tx.QueryRowContext(ctx, "PRAGMA query_only").Scan(&readonly); err != nil || readonly != 1 {
		return fmt.Errorf("read_only_transaction_required")
	}
	var check string
	if err := tx.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		return fmt.Errorf("sqlite_integrity_check: %s %v", check, err)
	}
	rows, err := tx.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	present := map[string]bool{}
	for _, table := range tables {
		present[table] = true
		required, ok := supportedTables[table]
		if !ok {
			s.Scopes = append(s.Scopes, Scope{Source: "sqlite:" + table, Disposition: "unsupported_source", Reason: "Outside the bounded B1 decoder; no payload interpretation or count claimed"})
			continue
		}
		captured, count, truncated, err := readTable(ctx, tx, table, required, maxRows(r), maxBytes(r))
		if err != nil {
			s.Scopes = append(s.Scopes, Scope{Source: "sqlite:" + table, Disposition: "source_unreadable", Reason: err.Error()})
			continue
		}
		disposition := "read"
		reason := "WAL interpreted in private byte capture; one query-only SQLite read transaction"
		if truncated {
			disposition = "source_unreadable"
			reason = "Read limit reached; only listed rows were examined"
		}
		s.Scopes = append(s.Scopes, Scope{Source: "sqlite:" + table, Disposition: disposition, Count: &count, Reason: reason})
		s.Rows = append(s.Rows, captured...)
	}
	for table := range supportedTables {
		if !present[table] {
			zero := 0
			s.Scopes = append(s.Scopes, Scope{Source: "sqlite:" + table, Disposition: "absent", Count: &zero, Reason: "Table absent from captured source"})
		}
	}
	return tx.Commit()
}
func quoteID(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func readTable(ctx context.Context, tx *sql.Tx, table string, required []string, limit int, byteLimit int64) ([]Row, int, bool, error) {
	columnRows, err := tx.QueryContext(ctx, "SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
	if err != nil {
		return nil, 0, false, err
	}
	var columns []string
	available := map[string]bool{}
	for columnRows.Next() {
		var name string
		if err := columnRows.Scan(&name); err != nil {
			columnRows.Close()
			return nil, 0, false, err
		}
		columns = append(columns, name)
		available[name] = true
	}
	columnRows.Close()
	for _, name := range required {
		if !available[name] {
			return nil, 0, false, fmt.Errorf("unsupported_schema: required column %s absent", name)
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteID(table)).Scan(&count); err != nil {
		return nil, 0, false, err
	}
	parts := []string{}
	for _, name := range columns {
		parts = append(parts, "typeof("+quoteID(name)+")", "CAST("+quoteID(name)+" AS BLOB)")
	}
	// Ordering by captured values makes a bounded prefix deterministic even on
	// WITHOUT ROWID tables. Duplicated complete rows are observationally equal.
	order := []string{}
	for _, name := range columns {
		order = append(order, "typeof("+quoteID(name)+")", "CAST("+quoteID(name)+" AS BLOB)")
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+strings.Join(parts, ",")+" FROM "+quoteID(table)+" ORDER BY "+strings.Join(order, ",")+" LIMIT ?", limit)
	if err != nil {
		return nil, count, false, err
	}
	defer rows.Close()
	var out []Row
	var total int64
	for rows.Next() {
		values := make([]any, len(columns)*2)
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return out, count, true, err
		}
		row := Row{Table: table, Columns: map[string]Value{}}
		for i, name := range columns {
			storage, _ := values[i*2].(string)
			var b []byte
			switch v := values[i*2+1].(type) {
			case []byte:
				b = bytes.Clone(v)
			case string:
				b = []byte(v)
			case nil:
			default:
				return out, count, true, fmt.Errorf("unsupported SQLite value %T", v)
			}
			total += int64(len(b))
			row.Columns[name] = Value{StorageClass: storage, Bytes: b}
		}
		if total > byteLimit {
			return out, count, true, nil
		}
		row.Key = text(row, "id")
		if row.Key == "" {
			row.Key = text(row, "section_id")
			if project := text(row, "project_id"); project != "" {
				row.Key = project + "/" + row.Key
			}
		}
		if row.Key == "" {
			b, _ := json.Marshal(row.Columns)
			row.Key = carrier.Digest(b)
		}
		out = append(out, row)
	}
	return out, count, len(out) < count, rows.Err()
}
func text(row Row, name string) string {
	v := row.Columns[name]
	if !utf8.Valid(v.Bytes) {
		return ""
	}
	return string(v.Bytes)
}
func captureCarriers(r Request, s *Snapshot) {
	absolute, err := filepath.Abs(r.CarrierRoot)
	if err != nil {
		s.Scopes = append(s.Scopes, Scope{Source: "carriers", Disposition: "source_unreadable", Reason: err.Error()})
		return
	}
	info, statErr := os.Lstat(absolute)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil || statErr != nil || info.Mode()&os.ModeSymlink != 0 {
		s.Scopes = append(s.Scopes, Scope{Source: "carriers", Disposition: "source_unreadable", Reason: "Carrier root is missing or symlinked"})
		return
	}
	absolute = resolved
	root, err := os.OpenRoot(absolute)
	if err != nil {
		s.Scopes = append(s.Scopes, Scope{Source: "carriers:" + absolute, Disposition: "source_unreadable", Reason: err.Error()})
		return
	}
	defer root.Close()
	var total int64
	count := 0
	scope := Scope{Source: "carriers:" + absolute, Disposition: "read"}
	err = filepath.WalkDir(absolute, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		relative, e := filepath.Rel(absolute, p)
		if e != nil {
			return e
		}
		if d.IsDir() {
			if relative != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "editions" || d.Name() == "transactions") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink_carrier: %s", relative)
		}
		if count >= maxRows(r) {
			return fmt.Errorf("carrier_count_limit")
		}
		a, e := regularFile(root, relative, maxBytes(r))
		if e != nil {
			return e
		}
		b, e := regularFile(root, relative, maxBytes(r))
		if e != nil || !bytes.Equal(a, b) {
			return fmt.Errorf("carrier_changed: %s", relative)
		}
		total += int64(len(b))
		if total > maxBytes(r) {
			return fmt.Errorf("carrier_byte_limit")
		}
		s.Carriers[filepath.ToSlash(relative)] = b
		count++
		return nil
	})
	if err != nil {
		scope.Disposition = "source_unreadable"
		scope.Reason = err.Error()
	}
	scope.Count = &count
	s.Scopes = append(s.Scopes, scope)
}
