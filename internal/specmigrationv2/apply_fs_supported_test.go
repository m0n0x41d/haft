//go:build darwin || linux

package specmigrationv2

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestAdoptOpenedFileRejectsNegativeDescriptor(t *testing.T) {
	file, err := adoptOpenedFileWith(
		-1,
		"negative-descriptor",
		func(uintptr, string) *os.File {
			t.Fatal("negative descriptor reached file construction")
			return nil
		},
		func(int) error {
			t.Fatal("negative descriptor reached close")
			return nil
		},
	)
	if err == nil {
		if file != nil {
			_ = file.Close()
		}
		t.Fatal("adoptOpenedFile() accepted a negative descriptor")
	}
	if file != nil {
		_ = file.Close()
		t.Fatal("adoptOpenedFile() returned a file for a negative descriptor")
	}
	if !strings.Contains(err.Error(), "descriptor is negative") {
		t.Fatalf("adoptOpenedFile() error = %q", err)
	}
}

func TestAdoptOpenedFileClosesDescriptorWhenConstructionFails(t *testing.T) {
	const descriptor = 41
	closed := -1
	file, err := adoptOpenedFileWith(
		descriptor,
		"construction-failure",
		func(value uintptr, name string) *os.File {
			if value != descriptor {
				t.Fatalf("descriptor = %d, want %d", value, descriptor)
			}
			if name != "construction-failure" {
				t.Fatalf("name = %q", name)
			}
			return nil
		},
		func(value int) error {
			closed = value
			return nil
		},
	)
	if err == nil || file != nil {
		t.Fatalf("adoptOpenedFileWith() = (%v, %v), want nil file and error", file, err)
	}
	if closed != descriptor {
		t.Fatalf("closed descriptor = %d, want %d", closed, descriptor)
	}
	if !strings.Contains(err.Error(), "os.NewFile returned nil") {
		t.Fatalf("adoptOpenedFileWith() error = %q", err)
	}
}

func TestAdoptOpenedFileReportsCloseFailure(t *testing.T) {
	closeFailure := errors.New("close failed")
	file, err := adoptOpenedFileWith(
		42,
		"close-failure",
		func(uintptr, string) *os.File { return nil },
		func(int) error { return closeFailure },
	)
	if file != nil {
		t.Fatal("adoptOpenedFileWith() returned a file after construction failure")
	}
	if !errors.Is(err, closeFailure) {
		t.Fatalf("adoptOpenedFileWith() error = %v, want close failure", err)
	}
}

func TestAdoptOpenedFileSuccessDoesNotCloseDescriptor(t *testing.T) {
	expected, err := os.CreateTemp(t.TempDir(), "adopted-file-")
	if err != nil {
		t.Fatalf("CreateTemp(): %v", err)
	}
	t.Cleanup(func() { _ = expected.Close() })

	const descriptor = 43
	file, err := adoptOpenedFileWith(
		descriptor,
		"success",
		func(value uintptr, name string) *os.File {
			if value != descriptor {
				t.Fatalf("descriptor = %d, want %d", value, descriptor)
			}
			if name != "success" {
				t.Fatalf("name = %q", name)
			}
			return expected
		},
		func(int) error {
			t.Fatal("successful adoption closed the raw descriptor")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("adoptOpenedFileWith(): %v", err)
	}
	if file != expected {
		t.Fatal("adoptOpenedFileWith() did not return the adopted file")
	}
}
