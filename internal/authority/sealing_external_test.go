package authority_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

func TestLegacyEvaluationValidityDoesNotAlterOpaqueResolution(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	evaluation := authority.EvaluateReceipt(now, authority.Receipt{
		Kind:                    authority.ReceiptKindManualCLI,
		PrincipalIdentitySource: "local-user",
		HostSessionSource:       "local-cli",
		Tool:                    "haft_profile",
		Action:                  "declare",
		ExpiresAt:               now.Add(time.Hour).Format(time.RFC3339),
		SingleUse:               true,
	}, authority.BindingAction{
		Tool:   "haft_profile",
		Action: "declare",
	})
	if evaluation.Status != authority.ReceiptStatusValid {
		t.Fatalf("legacy diagnostic should be valid for this fixture: %+v", evaluation)
	}
	if (authority.Resolution{}).Kind() != authority.ResolutionInvalid {
		t.Fatal("zero Resolution unexpectedly represents admitted authority")
	}
}

func TestExternalCodeCannotConstructAuthorityAdmittedState(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantErrors []string
	}{
		{
			name: "admitted use is not public",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ = authority.AdmittedUse{}
`,
			wantErrors: []string{"undefined: authority.AdmittedUse"},
		},
		{
			name: "legacy evaluation is not a resolution",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ authority.Resolution = authority.Evaluation{}
`,
			wantErrors: []string{"cannot use authority.Evaluation{}", "authority.Resolution"},
		},
		{
			name: "presentation state is private",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ = authority.Presentation{Value: 1}
`,
			wantErrors: []string{"unknown field Value", "authority.Presentation"},
		},
		{
			name: "resolution admitted variant is private",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ = authority.Resolution{Admitted: nil}
`,
			wantErrors: []string{"unknown field Admitted", "authority.Resolution"},
		},
		{
			name: "profile admission mutation surface is removed",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ = authority.ProfileAdmissionMutation{}
`,
			wantErrors: []string{"undefined: authority.ProfileAdmissionMutation"},
		},
		{
			name: "raw profile payload material constructor is removed",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ = authority.NewProfilePayloadMaterial
`,
			wantErrors: []string{"undefined: authority.NewProfilePayloadMaterial"},
		},
		{
			name: "profile admission callback session is removed",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ authority.ProfileAdmissionSession
`,
			wantErrors: []string{"undefined: authority.ProfileAdmissionSession"},
		},
		{
			name: "kernel gate no longer owns profile mutation transaction",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

func probe(gate *authority.KernelGate) {
	_ = gate.WithProfileAdmissionTransaction
}
`,
			wantErrors: []string{"gate.WithProfileAdmissionTransaction undefined"},
		},
		{
			name: "transaction authority snapshot state is private",
			source: `package probe

import "github.com/m0n0x41d/haft/internal/authority"

var _ = authority.ProfileAdmissionAuthoritySnapshot{State: nil}
`,
			wantErrors: []string{"unknown field State", "authority.ProfileAdmissionAuthoritySnapshot"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := compileExternalProbe(t, test.source)
			for _, expected := range test.wantErrors {
				if !strings.Contains(output, expected) {
					t.Fatalf("compiler output does not contain %q:\n%s", expected, output)
				}
			}
		})
	}
}

func compileExternalProbe(t *testing.T, source string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source location")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	probeRoot, err := os.MkdirTemp(repositoryRoot, ".authority-probe-")
	if err != nil {
		t.Fatalf("create probe directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(probeRoot) })
	if err := os.WriteFile(filepath.Join(probeRoot, "probe.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write probe source: %v", err)
	}
	command := exec.Command("go", "test", ".")
	command.Dir = probeRoot
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("external authority forgery probe compiled successfully:\n%s", output)
	}
	return string(output)
}
