package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestProfileDeclareCommandExposesOnlyReadableInputAndRenderingFlags(
	t *testing.T,
) {
	if profileDeclareCmd.Use != "declare" {
		t.Fatalf("command use = %q", profileDeclareCmd.Use)
	}
	if err := profileDeclareCmd.Args(profileDeclareCmd, []string{"opaque-id"}); err == nil {
		t.Fatal("profile declare accepted positional authority material")
	}
	flags := []string{}
	profileDeclareCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		flags = append(flags, flag.Name)
	})
	sort.Strings(flags)
	want := []string{"input-file", "json"}
	if !reflect.DeepEqual(flags, want) {
		t.Fatalf("profile declare flags = %#v, want %#v", flags, want)
	}
}

func TestLoadProfileDeclarationPolicyDefaultsWithoutSecondApproval(
	t *testing.T,
) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	policy, err := loadProfileDeclarationPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Mode() != profileonboarding.ProfileDeclarationModeExplicitHOnboard {
		t.Fatalf("default profile declaration mode = %q", policy.Mode())
	}
	strict := []byte("schema_version: 1\nauthority:\n  profile_declaration_mode: strict_cli_speech_act\n")
	configPath := project.ProjectConfigPath(filepath.Join(root, ".haft"))
	if err := os.WriteFile(configPath, strict, 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err = loadProfileDeclarationPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Mode() != profileonboarding.ProfileDeclarationModeStrictSpeechAct {
		t.Fatalf("strict profile declaration mode = %q", policy.Mode())
	}
}

func TestHiddenHaftSoftwareAdapterRequiresThePublicReviewCarrier(t *testing.T) {
	if !profileOnboardHaftSoftwareCmd.Hidden {
		t.Fatal("legacy Haft-software adapter became a public profile surface")
	}
	if strings.Contains(
		profileOnboardHaftSoftwareCmd.Long,
		"Manually authorize and admit the exact built-in",
	) {
		t.Fatal("compatibility help retained the legacy built-in minting promise")
	}
	for _, want := range []string{"profile-declaration-review.json", "never mints"} {
		if !strings.Contains(profileOnboardHaftSoftwareCmd.Long, want) {
			t.Fatalf("compatibility help omitted %q", want)
		}
	}
	fixture := newCLIProfileOnboardLedgerFixture(t)
	t.Chdir(fixture.root)
	command := &cobra.Command{}
	err := runProfileOnboardHaftSoftware(command, nil)
	if err == nil || !strings.Contains(err.Error(), "haft profile propose") {
		t.Fatalf("missing review carrier error = %v", err)
	}
}

func TestExecuteProfileDeclarationPassesSemanticInputAndPolicyToRuntime(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	writeProfileInspectionFixture(t, fixture.root, "go.mod")
	writeProfileInspectionFixture(t, fixture.root, "internal/kernel.go")
	suggestion, err := profiledetector.Inspect(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := profileonboarding.ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	input, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		proposal,
		suggestion,
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := profileonboarding.NewProfileDeclarationPolicy(
		profileonboarding.ProfileDeclarationModeExplicitHOnboard,
		".haft/config.yaml",
		[]byte("authority: explicit_h_onboard"),
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &recordingProfileDeclarationRuntime{t: t}
	response, err := executeProfileDeclaration(
		context.Background(),
		fixture.root,
		input,
		policy,
		runtime,
	)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.calls != 1 {
		t.Fatalf("profile declaration runtime calls = %d", runtime.calls)
	}
	if runtime.inputDigest != input.Digest().String() {
		t.Fatalf("runtime input digest = %q", runtime.inputDigest)
	}
	if runtime.mode != profileonboarding.ProfileDeclarationModeExplicitHOnboard {
		t.Fatalf("runtime authority mode = %q", runtime.mode)
	}
	if response.Kind != profileDeclarationRecordKind ||
		response.State != string(profileonboarding.ResultFailed) {
		t.Fatalf("typed declaration response = %#v", response)
	}
}

func TestProfileDeclarationFreshReviewedCandidateReplaysAfterLedgerRestart(
	t *testing.T,
) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	writeProfileInspectionFixture(t, fixture.root, "go.mod")
	writeProfileInspectionFixture(t, fixture.root, "internal/kernel.go")
	suggestion, err := profiledetector.Inspect(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfileReviewCandidate(fixture.root, suggestion); err != nil {
		t.Fatal(err)
	}
	t.Chdir(fixture.root)
	first := runProfileDeclarationJSON(t)
	if first.State != string(profileonboarding.ResultSynchronized) ||
		first.Admission == nil ||
		first.Revision == nil ||
		first.Projection == nil {
		t.Fatalf("fresh profile declaration = %#v", first)
	}
	second := runProfileDeclarationJSON(t)
	if first.Admission.Delivery != "fresh" {
		t.Fatalf("fresh delivery posture = %q", first.Admission.Delivery)
	}
	if second.Admission.Delivery != "resolved_after_restart" {
		t.Fatalf("restart delivery posture = %q", second.Admission.Delivery)
	}
	firstCanonical := *first.Admission
	secondCanonical := *second.Admission
	firstCanonical.Delivery = ""
	secondCanonical.Delivery = ""
	if !reflect.DeepEqual(secondCanonical, firstCanonical) {
		t.Fatalf(
			"restart replay changed canonical admission:\nfirst=%#v\nsecond=%#v",
			firstCanonical,
			secondCanonical,
		)
	}
	if !reflect.DeepEqual(second.Revision, first.Revision) {
		t.Fatalf(
			"restart replay changed ledger revision: first=%#v second=%#v",
			first.Revision,
			second.Revision,
		)
	}
}

func runProfileDeclarationJSON(t testing.TB) profileOnboardResponse {
	t.Helper()
	command := &cobra.Command{}
	command.SetContext(context.Background())
	output := &bytes.Buffer{}
	command.SetOut(output)
	if err := runProfileDeclarationCommand(command, "", true); err != nil {
		t.Fatal(err)
	}
	response := profileOnboardResponse{}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("decode profile declaration response: %v", err)
	}
	return response
}

type recordingProfileDeclarationRuntime struct {
	t           testing.TB
	calls       int
	inputDigest string
	mode        string
}

func (runtime *recordingProfileDeclarationRuntime) declare(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	input profileonboarding.ProfileOnboardingWorkInput,
	policy profileonboarding.ProfileDeclarationPolicy,
	revalidate profileLedgerRevalidation,
) (profileOnboardOutcome, error) {
	runtime.t.Helper()
	if ctx == nil || database == nil || projectRoot == "" || revalidate == nil {
		runtime.t.Fatal("declaration runtime omitted semantic execution context")
	}
	runtime.calls++
	runtime.inputDigest = input.Digest().String()
	runtime.mode = policy.Mode()
	if err := revalidate(ctx); err != nil {
		return profileOnboardOutcome{}, err
	}
	return profileOnboardOutcome{
		State: string(profileonboarding.ResultFailed),
		Failure: &profileOnboardFailure{
			Stage:  "fixture",
			Code:   "fixture_stop",
			Detail: "runtime boundary exercised",
		},
	}, nil
}
