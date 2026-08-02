package present

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestConcernDiscoveryResponseKeepsCandidateOrderAdvisory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writePresentSource(
		t,
		root,
		"incremental.go",
		"package sample\nfunc PublishIndexEpoch() {}\n",
	)
	store, closeStore := newPresentArtifactStore(t, root)
	defer closeStore()
	result, err := codeintel.NewService(store).DiscoverConcern(
		ctx,
		root,
		"Where is the index epoch published?",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	output := ConcernDiscoveryResponse(result)
	for _, wanted := range []string{
		"bounded fused candidate",
		"not identity selection",
		"terms=",
		"anchor=`sym:v2:",
		"graph proximity is neither applicability nor a decision",
		"not evidence that changing the symbol is safe",
		"Fusion replay:",
		"induced",
		"graph node",
	} {
		if !strings.Contains(output, wanted) {
			t.Fatalf("response missing %q:\n%s", wanted, output)
		}
	}
	for _, forbidden := range []string{
		"selected symbol",
		"winner",
		"best match",
	} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("response selected identity via %q:\n%s", forbidden, output)
		}
	}
}

func newPresentArtifactStore(
	t *testing.T,
	root string,
) (*artifact.Store, func()) {
	t.Helper()
	legacy, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(root, "present.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return artifact.NewStore(legacy.GetRawDB()), func() {
		_ = legacy.Close()
	}
}

func writePresentSource(
	t *testing.T,
	root string,
	relativePath string,
	content string,
) {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
