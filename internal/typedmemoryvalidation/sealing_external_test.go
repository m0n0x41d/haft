package typedmemoryvalidation_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEmbeddedRequestCannotCrossExactValidationBoundary(t *testing.T) {
	source := `package probe

import (
    "github.com/m0n0x41d/haft/internal/typedmemory"
    "github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
    "github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type embeddedRequest struct {
    typedmemorywire.ValidateRequest
}

func (embeddedRequest) ContractVersion() string { return "forged.version" }
func (embeddedRequest) Action() string { return "forged-action" }
func (embeddedRequest) ChangeCount() int { return 1000000 }
func (embeddedRequest) BindChangeSet(typedmemory.TypeEnvRef) (typedmemory.MemoryChangeSet, error) {
    return typedmemory.MemoryChangeSet{}, nil
}

func crossBoundary(service typedmemoryvalidation.Service, decoded typedmemorywire.ValidateRequest) {
    forged := embeddedRequest{ValidateRequest: decoded}
    service.Validate(forged)
}
`
	output := compileExternalRequestProbe(t, source)
	if !strings.Contains(output, "cannot use forged") {
		t.Fatalf("compiler did not reject embedded request at Service.Validate:\n%s", output)
	}
	if !strings.Contains(output, "typedmemorywire.ValidateRequest") {
		t.Fatalf("compiler rejection omitted exact request type:\n%s", output)
	}
}

func TestValidationOutcomeCannotBeImplementedOutsidePackage(t *testing.T) {
	source := `package probe

import (
    "github.com/m0n0x41d/haft/internal/typedmemory"
    "github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
)

type forgedOutcome struct{}

func (forgedOutcome) ContractVersion() string { return "haft.memory.v2" }
func (forgedOutcome) Verdict() typedmemory.ValidationVerdictKind {
    return typedmemory.ValidationValid
}
func (forgedOutcome) BasisProjection() typedmemoryvalidation.BasisProjection {
    return typedmemoryvalidation.BasisProjection{}
}
func (forgedOutcome) Diagnostics() []typedmemoryvalidation.DiagnosticProjection {
    return nil
}
func (forgedOutcome) outcomeVariant() {}

var _ typedmemoryvalidation.Outcome = forgedOutcome{}
`
	output := compileExternalRequestProbe(t, source)
	if !strings.Contains(output, "does not implement typedmemoryvalidation.Outcome") {
		t.Fatalf("compiler did not reject forged validation outcome:\n%s", output)
	}
	if !strings.Contains(output, "unexported method outcomeVariant") {
		t.Fatalf("compiler rejection omitted the sealed outcome boundary:\n%s", output)
	}
}

func compileExternalRequestProbe(t *testing.T, source string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve typed-memory validation test source location")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	probeRoot, err := os.MkdirTemp(repositoryRoot, ".typedmemoryvalidation-probe-")
	if err != nil {
		t.Fatalf("create probe directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(probeRoot) })
	probeFile := filepath.Join(probeRoot, "probe.go")
	if err := os.WriteFile(probeFile, []byte(source), 0o600); err != nil {
		t.Fatalf("write probe source: %v", err)
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(goBinary, "test", ".")
	command.Dir = probeRoot
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("external embedded-request probe compiled successfully:\n%s", output)
	}
	return string(output)
}
