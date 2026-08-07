package codebase_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"
)

func TestSourceAdmissionPureCorpus(t *testing.T) {
	path := mustProjectPath(t, "internal/x.go")
	language := mustSourceLanguage(t, "go")
	supported := mustSourceClass(t, "supported")

	t.Run("exact admitted bytes own their digest and usage", func(t *testing.T) {
		content := []byte("package x\nfunc A() {}\n")
		observation, err := codebase.NewContentObservation(
			path,
			language,
			supported,
			content,
		)
		if err != nil {
			t.Fatal(err)
		}
		admission, usage, err := codebase.AdmitSource(
			observation,
			testIndexBudget(t, 64, 4, 128),
			codebase.EmptyAdmissionUsage(),
		)
		if err != nil {
			t.Fatal(err)
		}
		if admission.Kind().String() != "source_admitted" {
			t.Fatalf("admission = %s", admission.Kind().String())
		}
		source, err := codebase.AdmittedSourceFrom(admission)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(content)
		wantDigest := hex.EncodeToString(sum[:])
		if source.Digest() != wantDigest {
			t.Fatalf("digest = %s, want %s", source.Digest(), wantDigest)
		}
		if source.ByteCount().Value() != int64(len(content)) {
			t.Fatalf("source bytes = %d", source.ByteCount().Value())
		}
		if usage.Files().Value() != 1 ||
			usage.Bytes().Value() != int64(len(content)) {
			t.Fatalf(
				"usage = files:%d bytes:%d",
				usage.Files().Value(),
				usage.Bytes().Value(),
			)
		}
	})

	t.Run("oversized metadata never needs parser bytes", func(t *testing.T) {
		size := mustByteCount(t, 65)
		observation, err := codebase.NewMetadataObservation(
			path,
			language,
			supported,
			size,
		)
		if err != nil {
			t.Fatal(err)
		}
		admission, usage, err := codebase.AdmitSource(
			observation,
			testIndexBudget(t, 64, 4, 128),
			codebase.EmptyAdmissionUsage(),
		)
		if err != nil {
			t.Fatal(err)
		}
		requireSkipped(t, admission, "oversized")
		if usage.Files().Value() != 1 || usage.Bytes().Value() != 0 {
			t.Fatalf("oversized observation usage = %#v", usage)
		}
	})

	t.Run("invalid UTF-8 is distinct from empty", func(t *testing.T) {
		observation, err := codebase.NewContentObservation(
			path,
			language,
			supported,
			[]byte{0xff, 0xfe},
		)
		if err != nil {
			t.Fatal(err)
		}
		admission, _, err := codebase.AdmitSource(
			observation,
			testIndexBudget(t, 64, 4, 128),
			codebase.EmptyAdmissionUsage(),
		)
		if err != nil {
			t.Fatal(err)
		}
		requireSkipped(t, admission, "invalid_encoding")
	})

	t.Run("empty supported source remains admitted", func(t *testing.T) {
		observation, err := codebase.NewContentObservation(
			path,
			language,
			supported,
			[]byte{},
		)
		if err != nil {
			t.Fatal(err)
		}
		admission, _, err := codebase.AdmitSource(
			observation,
			testIndexBudget(t, 64, 4, 128),
			codebase.EmptyAdmissionUsage(),
		)
		if err != nil {
			t.Fatal(err)
		}
		if admission.Kind().String() != "source_admitted" {
			t.Fatalf("empty source = %s, want source_admitted", admission.Kind().String())
		}
	})

	for _, code := range []string{
		"unsupported_language",
		"ignored_path",
	} {
		t.Run(code, func(t *testing.T) {
			observation, err := codebase.NewContentObservation(
				path,
				language,
				mustSourceClass(t, code),
				[]byte("content"),
			)
			if err != nil {
				t.Fatal(err)
			}
			admission, _, err := codebase.AdmitSource(
				observation,
				testIndexBudget(t, 64, 4, 128),
				codebase.EmptyAdmissionUsage(),
			)
			if err != nil {
				t.Fatal(err)
			}
			requireSkipped(t, admission, code)
		})
	}

	t.Run("generated exclusion is explicit policy", func(t *testing.T) {
		observation, err := codebase.NewContentObservation(
			path,
			language,
			mustSourceClass(t, "generated_source"),
			[]byte("package x"),
		)
		if err != nil {
			t.Fatal(err)
		}
		budget := testIndexBudgetWithGeneratedPolicy(
			t,
			64,
			4,
			128,
			"exclude_generated",
		)
		admission, _, err := codebase.AdmitSource(
			observation,
			budget,
			codebase.EmptyAdmissionUsage(),
		)
		if err != nil {
			t.Fatal(err)
		}
		requireSkipped(t, admission, "generated_source")
	})

	t.Run("generated inclusion preserves existing indexing contract", func(t *testing.T) {
		observation, err := codebase.NewContentObservation(
			path,
			language,
			mustSourceClass(t, "generated_source"),
			[]byte("package x"),
		)
		if err != nil {
			t.Fatal(err)
		}
		admission, _, err := codebase.AdmitSource(
			observation,
			testIndexBudget(t, 64, 4, 128),
			codebase.EmptyAdmissionUsage(),
		)
		if err != nil {
			t.Fatal(err)
		}
		if admission.Kind().String() != "source_admitted" {
			t.Fatalf(
				"generated default admission = %s",
				admission.Kind().String(),
			)
		}
	})
}

func TestSourceAdmissionRootBudgetsAreTyped(t *testing.T) {
	path := mustProjectPath(t, "a.go")
	language := mustSourceLanguage(t, "go")
	class := mustSourceClass(t, "supported")
	first := mustContentObservation(
		t,
		path,
		language,
		class,
		[]byte("1234"),
	)

	fileBudget := testIndexBudget(t, 8, 1, 16)
	_, usage, err := codebase.AdmitSource(
		first,
		fileBudget,
		codebase.EmptyAdmissionUsage(),
	)
	if err != nil {
		t.Fatal(err)
	}
	second := mustContentObservation(
		t,
		mustProjectPath(t, "b.go"),
		language,
		class,
		[]byte("1"),
	)
	admission, unchanged, err := codebase.AdmitSource(
		second,
		fileBudget,
		usage,
	)
	if err != nil {
		t.Fatal(err)
	}
	requireSkipped(t, admission, "root_file_budget")
	if unchanged.Files().Value() != usage.Files().Value() {
		t.Fatalf("file-budget skip changed usage")
	}

	byteBudget := testIndexBudget(t, 4, 4, 4)
	_, usage, err = codebase.AdmitSource(
		first,
		byteBudget,
		codebase.EmptyAdmissionUsage(),
	)
	if err != nil {
		t.Fatal(err)
	}
	admission, _, err = codebase.AdmitSource(
		second,
		byteBudget,
		usage,
	)
	if err != nil {
		t.Fatal(err)
	}
	requireSkipped(t, admission, "root_byte_budget")
}

func TestOversizedSourcesConsumeRootFileBudget(t *testing.T) {
	language := mustSourceLanguage(t, "go")
	class := mustSourceClass(t, "supported")
	budget := testIndexBudget(t, 4, 1, 8)
	first, err := codebase.NewMetadataObservation(
		mustProjectPath(t, "a.go"),
		language,
		class,
		mustByteCount(t, 5),
	)
	if err != nil {
		t.Fatal(err)
	}
	admission, usage, err := codebase.AdmitSource(
		first,
		budget,
		codebase.EmptyAdmissionUsage(),
	)
	if err != nil {
		t.Fatal(err)
	}
	requireSkipped(t, admission, "oversized")
	second, err := codebase.NewMetadataObservation(
		mustProjectPath(t, "b.go"),
		language,
		class,
		mustByteCount(t, 5),
	)
	if err != nil {
		t.Fatal(err)
	}
	admission, _, err = codebase.AdmitSource(second, budget, usage)
	if err != nil {
		t.Fatal(err)
	}
	requireSkipped(t, admission, "root_file_budget")
}

func TestObserveSourceReadsOnlyTheAdmissionBound(t *testing.T) {
	root := t.TempDir()
	path := mustProjectPath(t, "large.go")
	content := []byte("123456789")
	if err := os.WriteFile(
		filepath.Join(root, path.String()),
		content,
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	budget := testIndexBudget(t, 8, 4, 16)
	observation, err := codebase.ObserveSource(
		root,
		path,
		mustSourceLanguage(t, "go"),
		mustSourceClass(t, "supported"),
		budget,
	)
	if err != nil {
		t.Fatal(err)
	}
	admission, _, err := codebase.AdmitSource(
		observation,
		budget,
		codebase.EmptyAdmissionUsage(),
	)
	if err != nil {
		t.Fatal(err)
	}
	requireSkipped(t, admission, "oversized")
}

func TestObserveSourceRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(outside, "secret.go"),
		[]byte("package secret\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	_, err := codebase.ObserveSource(
		root,
		mustProjectPath(t, "escape/secret.go"),
		mustSourceLanguage(t, "go"),
		mustSourceClass(t, "supported"),
		codebase.DefaultIndexBudget(),
	)
	if err == nil || !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("escaping symlink error = %v", err)
	}
}

func TestExtractStableFileRangeRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(outside, "secret.go"),
		[]byte("package secret\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	_, err := codebase.ExtractStableFileRange(
		root,
		"escape/secret.go",
	)
	if err == nil || !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("range source escape error = %v", err)
	}
}

func TestRegistryReturnsTypedReadFailure(t *testing.T) {
	root := t.TempDir()
	admission, usage, err := codebase.NewRegistry().ReadSourceAdmission(
		root,
		"missing.go",
		codebase.DefaultIndexBudget(),
		codebase.EmptyAdmissionUsage(),
	)
	if err != nil {
		t.Fatal(err)
	}
	info, err := codebase.SkippedSourceInfo(admission)
	if err != nil {
		t.Fatal(err)
	}
	if info.Reason != "read_failure" || !info.RequiresRetry() {
		t.Fatalf("missing source info = %+v", info)
	}
	if usage.Files().Value() != 0 || usage.Bytes().Value() != 0 {
		t.Fatalf("read failure changed usage: %#v", usage)
	}
}

func TestRegistryDoesNotAdmitUnsupportedGeneratedPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "generated.min.unknown")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	admission, _, err := codebase.NewRegistry().ReadSourceAdmission(
		root,
		"generated.min.unknown",
		codebase.DefaultIndexBudget(),
		codebase.EmptyAdmissionUsage(),
	)
	if err != nil {
		t.Fatal(err)
	}
	info, err := codebase.SkippedSourceInfo(admission)
	if err != nil {
		t.Fatal(err)
	}
	if info.Reason != "unsupported_language" {
		t.Fatalf("unsupported generated source = %+v", info)
	}
}

func TestParserCompatibilityShellRejectsOversizedSource(t *testing.T) {
	root := t.TempDir()
	content := strings.Repeat("x", 500_001)
	if err := os.WriteFile(
		filepath.Join(root, "oversized.go"),
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	_, err := codebase.NewRegistry().ExtractSymbolSnapshots(
		root,
		"oversized.go",
	)
	var skipped codebase.SourceSkippedError
	if !errors.As(err, &skipped) {
		t.Fatalf("oversized parser input error = %v", err)
	}
	if skipped.Info.Reason != "oversized" {
		t.Fatalf("oversized parser skip = %+v", skipped.Info)
	}
}

func TestAdmissionConstructorsRejectInvalidBoundaries(t *testing.T) {
	if _, err := codebase.NewProjectPath("../escape.go"); err == nil {
		t.Fatal("path traversal must be rejected")
	}
	if _, err := codebase.NewByteCount(-1); err == nil {
		t.Fatal("negative byte count must be rejected")
	}
	zeroWorkers, err := codebase.NewWorkerCount(0)
	if err == nil || zeroWorkers.Value() != 0 {
		t.Fatal("zero workers must be rejected")
	}
	maxFile := mustByteCount(t, 10)
	maxFiles, err := codebase.NewFileCount(1)
	if err != nil {
		t.Fatal(err)
	}
	maxObserved := mustByteCount(t, 9)
	workers, err := codebase.NewWorkerCount(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codebase.NewIndexBudget(codebase.IndexBudgetSpec{
		MaxFileBytes:     maxFile,
		MaxFiles:         maxFiles,
		MaxObservedBytes: maxObserved,
		MaxParseWorkers:  workers,
		GeneratedSources: mustGeneratedSourcePolicy(
			t,
			"include_generated",
		),
	}); err == nil {
		t.Fatal("root bytes smaller than one max file must be rejected")
	}
}

func testIndexBudget(
	t *testing.T,
	maxFileBytes int64,
	maxFiles int64,
	maxObservedBytes int64,
) codebase.IndexBudget {
	t.Helper()
	return testIndexBudgetWithGeneratedPolicy(
		t,
		maxFileBytes,
		maxFiles,
		maxObservedBytes,
		"include_generated",
	)
}

func testIndexBudgetWithGeneratedPolicy(
	t *testing.T,
	maxFileBytes int64,
	maxFiles int64,
	maxObservedBytes int64,
	generatedPolicy string,
) codebase.IndexBudget {
	t.Helper()
	fileBytes := mustByteCount(t, maxFileBytes)
	files, err := codebase.NewFileCount(maxFiles)
	if err != nil {
		t.Fatal(err)
	}
	observedBytes := mustByteCount(t, maxObservedBytes)
	workers, err := codebase.NewWorkerCount(2)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := codebase.NewIndexBudget(codebase.IndexBudgetSpec{
		MaxFileBytes:     fileBytes,
		MaxFiles:         files,
		MaxObservedBytes: observedBytes,
		MaxParseWorkers:  workers,
		GeneratedSources: mustGeneratedSourcePolicy(t, generatedPolicy),
	})
	if err != nil {
		t.Fatal(err)
	}
	return budget
}

func mustGeneratedSourcePolicy(
	t *testing.T,
	value string,
) codebase.GeneratedSourcePolicy {
	t.Helper()
	policy, err := codebase.ParseGeneratedSourcePolicy(value)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func mustProjectPath(
	t *testing.T,
	value string,
) codebase.ProjectPath {
	t.Helper()
	path, err := codebase.NewProjectPath(value)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func mustSourceLanguage(
	t *testing.T,
	value string,
) codebase.SourceLanguage {
	t.Helper()
	language, err := codebase.NewSourceLanguage(value)
	if err != nil {
		t.Fatal(err)
	}
	return language
}

func mustSourceClass(
	t *testing.T,
	value string,
) codebase.SourceClass {
	t.Helper()
	class, err := codebase.ParseSourceClass(value)
	if err != nil {
		t.Fatal(err)
	}
	return class
}

func mustByteCount(
	t *testing.T,
	value int64,
) codebase.ByteCount {
	t.Helper()
	count, err := codebase.NewByteCount(value)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func mustContentObservation(
	t *testing.T,
	path codebase.ProjectPath,
	language codebase.SourceLanguage,
	class codebase.SourceClass,
	content []byte,
) codebase.SourceObservation {
	t.Helper()
	observation, err := codebase.NewContentObservation(
		path,
		language,
		class,
		content,
	)
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func requireSkipped(
	t *testing.T,
	admission codebase.SourceAdmission,
	reason string,
) {
	t.Helper()
	if admission.Kind().String() != "source_skipped" {
		t.Fatalf("admission = %s, want source_skipped", admission.Kind().String())
	}
	if admission.DetailCode() != reason {
		t.Fatalf("skip reason = %s, want %s", admission.DetailCode(), reason)
	}
}
