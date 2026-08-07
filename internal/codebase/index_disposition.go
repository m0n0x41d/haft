package codebase

import (
	"fmt"
	"strconv"
	"strings"
)

// FileIndexDispositionKind is the closed post-admission state discriminator.
type FileIndexDispositionKind struct {
	code string
}

var fileIndexDispositionKinds = map[string]FileIndexDispositionKind{
	CodeFileIndexed:  {code: CodeFileIndexed},
	CodeFileEmpty:    {code: CodeFileEmpty},
	CodeFileSkipped:  {code: CodeFileSkipped},
	CodeFileDegraded: {code: CodeFileDegraded},
}

func (k FileIndexDispositionKind) String() string {
	return k.code
}

// FileIndexDisposition is the closed post-admission result for one source
// file. Consumers cannot represent "indexed with zero symbols" or a skipped
// file without an explicit admission reason.
type FileIndexDisposition interface {
	Kind() FileIndexDispositionKind
	DetailCode() string
	StatusCode() string
	fileIndexDisposition()
}

type indexedFileDisposition struct {
	symbols FileCount
}

func (indexedFileDisposition) fileIndexDisposition() {}

func (indexedFileDisposition) Kind() FileIndexDispositionKind {
	return fileIndexDispositionKinds[CodeFileIndexed]
}

func (d indexedFileDisposition) DetailCode() string {
	return strconv.FormatInt(d.symbols.Value(), 10)
}

func (indexedFileDisposition) StatusCode() string {
	return CodeFileIndexed
}

type emptyFileDisposition struct{}

func (emptyFileDisposition) fileIndexDisposition() {}

func (emptyFileDisposition) Kind() FileIndexDispositionKind {
	return fileIndexDispositionKinds[CodeFileEmpty]
}

func (emptyFileDisposition) DetailCode() string {
	return "no_symbols"
}

func (emptyFileDisposition) StatusCode() string {
	return CodeFileEmpty
}

type skippedFileDisposition struct {
	reason SourceSkipReason
}

func (skippedFileDisposition) fileIndexDisposition() {}

func (skippedFileDisposition) Kind() FileIndexDispositionKind {
	return fileIndexDispositionKinds[CodeFileSkipped]
}

func (d skippedFileDisposition) DetailCode() string {
	return d.reason.String()
}

func (d skippedFileDisposition) StatusCode() string {
	return CodeFileSkipped + ":" + d.reason.String()
}

type degradedFileDisposition struct {
	reason string
}

func (degradedFileDisposition) fileIndexDisposition() {}

func (degradedFileDisposition) Kind() FileIndexDispositionKind {
	return fileIndexDispositionKinds[CodeFileDegraded]
}

func (d degradedFileDisposition) DetailCode() string {
	return d.reason
}

func (degradedFileDisposition) StatusCode() string {
	return CodeFileDegraded
}

func NewIndexedFileDisposition(
	symbols FileCount,
) (FileIndexDisposition, error) {
	if !symbols.valid() || symbols.Value() < 1 {
		return nil, fmt.Errorf(
			"indexed file disposition requires at least one symbol",
		)
	}
	return indexedFileDisposition{symbols: symbols}, nil
}

func NewEmptyFileDisposition() FileIndexDisposition {
	return emptyFileDisposition{}
}

func NewSkippedFileDisposition(
	reason SourceSkipReason,
) (FileIndexDisposition, error) {
	if !reason.valid() {
		return nil, fmt.Errorf(
			"skipped file disposition requires an admission reason",
		)
	}
	return skippedFileDisposition{reason: reason}, nil
}

func NewDegradedFileDisposition(
	reason string,
) (FileIndexDisposition, error) {
	if reason == "" {
		return nil, fmt.Errorf(
			"degraded file disposition requires a reason",
		)
	}
	return degradedFileDisposition{reason: reason}, nil
}

// FileIndexFailure keeps a failed candidate parse paired with its typed
// degraded disposition. The candidate epoch may be rejected without losing the
// exact per-file reason.
type FileIndexFailure struct {
	Path        string
	Disposition FileIndexDisposition
}

func (f FileIndexFailure) Error() string {
	return fmt.Sprintf(
		"parse %s: %s",
		f.Path,
		f.Disposition.DetailCode(),
	)
}

func NewFileIndexFailure(
	path string,
	reason string,
) (FileIndexFailure, error) {
	if path == "" {
		return FileIndexFailure{}, fmt.Errorf(
			"file index failure requires a path",
		)
	}
	disposition, err := NewDegradedFileDisposition(reason)
	if err != nil {
		return FileIndexFailure{}, err
	}
	return FileIndexFailure{
		Path:        path,
		Disposition: disposition,
	}, nil
}

// ParsePersistedFileIndexDisposition restores the strong in-memory union from
// the compatibility status code plus the separately persisted symbol count.
func ParsePersistedFileIndexDisposition(
	status string,
	symbolCount int64,
) (FileIndexDisposition, error) {
	switch {
	case status == CodeFileIndexed:
		count, err := NewFileCount(symbolCount)
		if err != nil {
			return nil, err
		}
		return NewIndexedFileDisposition(count)
	case status == CodeFileEmpty:
		if symbolCount != 0 {
			return nil, fmt.Errorf(
				"empty persisted file cannot carry symbols",
			)
		}
		return NewEmptyFileDisposition(), nil
	case strings.HasPrefix(status, CodeFileSkipped+":"):
		if symbolCount != 0 {
			return nil, fmt.Errorf(
				"skipped persisted file cannot carry symbols",
			)
		}
		reason := strings.TrimPrefix(status, CodeFileSkipped+":")
		parsed, err := ParseSourceSkipReason(reason)
		if err != nil {
			return nil, err
		}
		return NewSkippedFileDisposition(parsed)
	case status == CodeFileDegraded:
		return NewDegradedFileDisposition(
			"persisted degraded compatibility state",
		)
	default:
		return nil, fmt.Errorf(
			"unknown persisted file disposition %q",
			status,
		)
	}
}
