package projecttypeenv

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	_ "modernc.org/sqlite"
)

func TestResolveManifestGraphIsPermutationInvariantAndDependencyOrdered(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alpha := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	beta := parseCarrier(t, carrierFixture(
		t,
		base,
		"beta.signature",
		"1.1.0",
		"Beta",
		[]string{"alpha.signature"},
	))

	forward := ResolveManifestGraph(base, []localpractice.ParsedCarrier{alpha, beta})
	reverse := ResolveManifestGraph(base, []localpractice.ParsedCarrier{beta, alpha})
	forwardCoordinates := resolvedCoordinates(t, forward)
	reverseCoordinates := resolvedCoordinates(t, reverse)
	want := []string{"alpha.signature@1.0.0", "beta.signature@1.1.0"}
	if !reflect.DeepEqual(forwardCoordinates, want) {
		t.Fatalf("forward coordinates = %#v, want %#v", forwardCoordinates, want)
	}
	if !reflect.DeepEqual(reverseCoordinates, want) {
		t.Fatalf("reverse coordinates = %#v, want %#v", reverseCoordinates, want)
	}

	bundle, exists := reverse.Bundle()
	if !exists {
		t.Fatal("accepted resolution did not expose its bundle")
	}
	nodes := bundle.Nodes()
	imports := nodes[1].Imports()
	if len(imports) != 1 || imports[0].String() != "alpha.signature@1.0.0" {
		t.Fatalf("beta imports = %#v", imports)
	}
	imports[0] = ManifestCoordinate{}
	if bundle.Nodes()[1].Imports()[0].String() != "alpha.signature@1.0.0" {
		t.Fatal("resolved bundle leaked its import slice")
	}
}

func TestResolveManifestGraphRejectsMissingSelfAndCyclicImports(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	tests := []struct {
		name     string
		carriers func(*testing.T) []localpractice.ParsedCarrier
		code     LinkIssueCode
	}{
		{
			name: "missing",
			carriers: func(t *testing.T) []localpractice.ParsedCarrier {
				carrier := carrierFixture(t, base, "beta.signature", "1.0.0", "Beta", []string{"absent.signature"})
				return []localpractice.ParsedCarrier{parseCarrier(t, carrier)}
			},
			code: IssueMissingImport,
		},
		{
			name: "self",
			carriers: func(t *testing.T) []localpractice.ParsedCarrier {
				carrier := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", []string{"alpha.signature"})
				return []localpractice.ParsedCarrier{parseCarrier(t, carrier)}
			},
			code: IssueSelfImport,
		},
		{
			name: "cycle",
			carriers: func(t *testing.T) []localpractice.ParsedCarrier {
				alpha := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", []string{"beta.signature"})
				beta := carrierFixture(t, base, "beta.signature", "1.0.0", "Beta", []string{"alpha.signature"})
				return []localpractice.ParsedCarrier{parseCarrier(t, alpha), parseCarrier(t, beta)}
			},
			code: IssueImportCycle,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			resolution := ResolveManifestGraph(base, test.carriers(t))
			assertRejectedWithCode(t, resolution, test.code)
		})
	}
}

func TestImportCycleDiagnosticsExcludeAcyclicDescendants(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alpha := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", []string{"beta.signature"}))
	beta := parseCarrier(t, carrierFixture(t, base, "beta.signature", "1.0.0", "Beta", []string{"alpha.signature"}))
	gamma := parseCarrier(t, carrierFixture(t, base, "gamma.signature", "1.0.0", "Gamma", []string{"alpha.signature"}))
	resolution := ResolveManifestGraph(
		base,
		[]localpractice.ParsedCarrier{gamma, beta, alpha},
	)
	assertRejectedWithCode(t, resolution, IssueImportCycle)

	cyclic := make([]string, 0)
	for _, issue := range resolution.Issues() {
		if issue.Code() == IssueImportCycle {
			cyclic = append(cyclic, issue.Subject())
		}
	}
	want := []string{"alpha.signature", "beta.signature"}
	if !reflect.DeepEqual(cyclic, want) {
		t.Fatalf("cycle subjects = %#v, want %#v", cyclic, want)
	}
}

func TestResolveManifestGraphRejectsAmbiguousSignatureVersionsWithoutLatest(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	first := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	second := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "2.0.0", "Beta", nil))
	resolution := ResolveManifestGraph(base, []localpractice.ParsedCarrier{first, second})
	assertRejectedWithCode(t, resolution, IssueDuplicateSignatureID)
}

func TestResolveManifestGraphRejectsDuplicateManifestCoordinate(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	first := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	second := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Beta", nil))
	resolution := ResolveManifestGraph(base, []localpractice.ParsedCarrier{first, second})
	assertRejectedWithCode(t, resolution, IssueDuplicateManifestCoordinate)
}

func TestResolveManifestGraphRejectsBaseCompilerAndIdentityMismatch(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	valid := string(carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	wrongBase := strings.Replace(
		valid,
		baseRef(t, base),
		"typeenv:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		1,
	)
	wrongCompiler := strings.Replace(
		valid,
		SupportedCompilerVersion,
		"haft.local-practice.compiler/v2",
		1,
	)
	wrongIdentity := strings.Replace(
		valid,
		"carrier:\n  id: alpha.signature\n  edition: 1.0.0",
		"carrier:\n  id: carrier.identity\n  edition: release-label",
		1,
	)

	tests := []struct {
		name   string
		source string
		code   LinkIssueCode
	}{
		{name: "base", source: wrongBase, code: IssueBaseRefMismatch},
		{name: "compiler", source: wrongCompiler, code: IssueCompilerVersionMismatch},
		{name: "identity", source: wrongIdentity, code: IssueCarrierManifestMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			parsed := parseCarrier(t, []byte(test.source))
			resolution := ResolveManifestGraph(base, []localpractice.ParsedCarrier{parsed})
			assertRejectedWithCode(t, resolution, test.code)
		})
	}
}

func TestResolveManifestGraphRejectsInvalidBaseArtifact(t *testing.T) {
	t.Parallel()

	resolution := ResolveManifestGraph(typeenv.BaseTypeEnvArtifact{}, nil)
	assertRejectedWithCode(t, resolution, IssueBaseArtifactInvalid)
}

func TestCanonicalSemVerPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		valid bool
	}{
		{value: "1.0.0", valid: true},
		{value: "0.0.0-alpha.1+build.5", valid: true},
		{value: "01.0.0", valid: false},
		{value: "1.0", valid: false},
		{value: "1.0.0-01", valid: false},
		{value: "1.0.0+", valid: false},
		{value: "release-label", valid: false},
	}
	for _, test := range tests {
		if got := isCanonicalSemVer(test.value); got != test.valid {
			t.Errorf("isCanonicalSemVer(%q) = %t, want %t", test.value, got, test.valid)
		}
	}
}

func loadBaseArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	databasePath, err := filepath.Abs(filepath.Join("..", "..", "cli", "fpf.db"))
	if err != nil {
		t.Fatalf("resolve embedded FPF database: %v", err)
	}
	database, err := sql.Open("sqlite", "file:"+databasePath+"?mode=ro")
	if err != nil {
		t.Fatalf("open embedded FPF database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		t.Fatalf("load verified P6 base artifact: %v", err)
	}
	return artifact
}

func carrierFixture(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	id string,
	version string,
	prefix string,
	imports []string,
) []byte {
	t.Helper()
	fixturePath := filepath.Join("..", "localpractice", "testdata", "valid.yaml")
	encoded, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read Local-Practice fixture: %v", err)
	}
	source := string(encoded)
	source = strings.Replace(source, "haft.typed-memory", id, 2)
	source = strings.Replace(source, "1.0.0", version, 2)
	source = strings.Replace(source, "typeenv:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", baseRef(t, base), 1)
	source = strings.ReplaceAll(source, "Haft.", prefix+".")
	source = strings.ReplaceAll(source, "ConcernSlot", prefix+".ConcernSlot")
	source = strings.ReplaceAll(source, "EvidenceSlot", prefix+".EvidenceSlot")
	source = strings.Replace(source, "  imports: []", manifestImports(imports), 1)
	return []byte(source)
}

func manifestImports(imports []string) string {
	if len(imports) == 0 {
		return "  imports: []"
	}
	lines := make([]string, 0, len(imports)+1)
	lines = append(lines, "  imports:")
	for _, id := range imports {
		lines = append(lines, "    - "+id)
	}
	return strings.Join(lines, "\n")
}

func parseCarrier(t *testing.T, source []byte) localpractice.ParsedCarrier {
	t.Helper()
	parsed, err := localpractice.Parse(source)
	if err != nil {
		t.Fatalf("parse Local-Practice fixture: %v\n%s", err, source)
	}
	return parsed
}

func baseRef(t *testing.T, base typeenv.BaseTypeEnvArtifact) string {
	t.Helper()
	ref, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("test P6 base has no TypeEnvRef")
	}
	return ref.String()
}

func resolvedCoordinates(t *testing.T, resolution BundleResolution) []string {
	t.Helper()
	if resolution.Rejected() {
		t.Fatalf("ResolveManifestGraph() rejected: %#v", resolution.Issues())
	}
	bundle, exists := resolution.Bundle()
	if !exists {
		t.Fatal("accepted resolution has no bundle")
	}
	nodes := bundle.Nodes()
	coordinates := make([]string, 0, len(nodes))
	for _, node := range nodes {
		coordinates = append(coordinates, node.Coordinate().String())
	}
	return coordinates
}

func assertRejectedWithCode(
	t *testing.T,
	resolution BundleResolution,
	want LinkIssueCode,
) {
	t.Helper()
	if !resolution.Rejected() {
		t.Fatalf("ResolveManifestGraph() accepted, want %s", want)
	}
	for _, issue := range resolution.Issues() {
		if issue.Code() == want {
			return
		}
	}
	t.Fatalf("issues = %#v, want code %s", resolution.Issues(), want)
}
