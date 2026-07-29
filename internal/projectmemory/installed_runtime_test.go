package projectmemory

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestComposeInstalledBaseTypeEnvRuntimeIsExactAndDeterministic(
	t *testing.T,
) {
	t.Parallel()

	artifact := loaderTestBundledArtifact(t)
	environment, codecs, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(
		artifact,
	)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecs() error = %v", err)
	}

	first, err := ComposeInstalledBaseTypeEnvRuntime(environment, codecs)
	if err != nil {
		t.Fatalf("ComposeInstalledBaseTypeEnvRuntime() error = %v", err)
	}
	second, err := ComposeInstalledBaseTypeEnvRuntime(environment, codecs)
	if err != nil {
		t.Fatalf("ComposeInstalledBaseTypeEnvRuntime(second) error = %v", err)
	}
	if first.Codecs.Len() != codecs.Len() {
		t.Fatalf(
			"installed codec count = %d, want %d",
			first.Codecs.Len(),
			codecs.Len(),
		)
	}
	if len(first.MechanismCatalogs) != 1 {
		t.Fatalf(
			"installed mechanism catalogs = %d, want 1",
			len(first.MechanismCatalogs),
		)
	}
	if len(second.MechanismCatalogs) != 1 {
		t.Fatalf(
			"second installed mechanism catalogs = %d, want 1",
			len(second.MechanismCatalogs),
		)
	}
	firstCatalog := first.MechanismCatalogs[0]
	secondCatalog := second.MechanismCatalogs[0]
	if firstCatalog.Identity() != secondCatalog.Identity() ||
		!bytes.Equal(
			firstCatalog.CanonicalBytes(),
			secondCatalog.CanonicalBytes(),
		) {
		t.Fatal("identical embedded runtime inputs produced different catalogs")
	}

	uniqueCodecs := make(map[string]struct{})
	for _, binding := range environment.ValueBindings() {
		uniqueCodecs[binding.Codec().String()] = struct{}{}
	}
	if got := len(firstCatalog.Entries()); got != len(uniqueCodecs) {
		t.Fatalf(
			"catalog entries = %d, want one per exact codec (%d)",
			got,
			len(uniqueCodecs),
		)
	}
}

func TestComposeInstalledBaseTypeEnvRuntimeRejectsMissingCodec(
	t *testing.T,
) {
	t.Parallel()

	artifact := loaderTestBundledArtifact(t)
	environment, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(artifact)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecs() error = %v", err)
	}

	_, err = ComposeInstalledBaseTypeEnvRuntime(
		environment,
		typedmemory.NewCodecRegistry(),
	)
	if err == nil {
		t.Fatal("ComposeInstalledBaseTypeEnvRuntime accepted a missing codec")
	}
}
