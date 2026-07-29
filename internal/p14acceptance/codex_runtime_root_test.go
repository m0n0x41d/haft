package p14acceptance

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func p14CanonicalCodexRuntimeRoots() (string, string, error) {
	account, err := user.Current()
	if err != nil {
		return "", "", fmt.Errorf(
			"resolve canonical Codex host account: %w",
			err,
		)
	}
	home := filepath.Clean(account.HomeDir)
	if !filepath.IsAbs(home) {
		return "", "", fmt.Errorf(
			"canonical Codex host home is not absolute",
		)
	}
	stateRoot, err := p14CanonicalExistingPathOrClean(
		filepath.Join(home, ".codex"),
	)
	if err != nil {
		return "", "", err
	}
	sessionRoot, err := p14CanonicalExistingPathOrClean(
		filepath.Join(stateRoot, "sessions"),
	)
	if err != nil {
		return "", "", err
	}
	if !p14PathIsWithin(stateRoot, sessionRoot) {
		return "", "", fmt.Errorf(
			"canonical Codex session root escapes state root",
		)
	}
	return stateRoot, sessionRoot, nil
}

func p14CanonicalExistingPathOrClean(path string) (string, error) {
	clean := filepath.Clean(path)
	_, err := os.Lstat(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return clean, nil
		}
		return "", err
	}
	physical, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(physical)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func p14PathIsWithin(root string, candidate string) bool {
	cleanRoot := filepath.Clean(root)
	cleanCandidate := filepath.Clean(candidate)
	if cleanRoot == cleanCandidate {
		return true
	}
	prefix := cleanRoot + string(filepath.Separator)
	return strings.HasPrefix(cleanCandidate, prefix)
}

func TestP14CanonicalCodexRuntimeRootsIgnoreCallerEnvironment(
	t *testing.T,
) {
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"CODEX_HOME",
		filepath.Join(t.TempDir(), "caller-selected-codex-home"),
	)
	stateRoot, sessionRoot, err := p14CanonicalCodexRuntimeRoots()
	if err != nil {
		t.Fatal(err)
	}
	expectedStateRoot, err := p14CanonicalExistingPathOrClean(
		filepath.Join(filepath.Clean(account.HomeDir), ".codex"),
	)
	if err != nil {
		t.Fatal(err)
	}
	expectedSessionRoot, err := p14CanonicalExistingPathOrClean(
		filepath.Join(expectedStateRoot, "sessions"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if stateRoot != expectedStateRoot ||
		sessionRoot != expectedSessionRoot {
		t.Fatal("caller-controlled CODEX_HOME changed canonical roots")
	}
	if p14PathIsWithin(
		sessionRoot,
		sessionRoot+"-forged",
	) {
		t.Fatal("prefix-sibling escaped segment-safe root containment")
	}
}
