package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestProfileOnboardHaftSoftwareCommandIsManualAndBounded(t *testing.T) {
	if profileOnboardHaftSoftwareCmd.Use != "onboard-haft-software" {
		t.Fatalf("command use = %q", profileOnboardHaftSoftwareCmd.Use)
	}
	if !profileOnboardHaftSoftwareCmd.Hidden {
		t.Fatal("Haft-only profile onboarding adapter is exposed as a public cross-project command")
	}
	if err := profileOnboardHaftSoftwareCmd.Args(
		profileOnboardHaftSoftwareCmd,
		[]string{"caller-supplied-profile"},
	); err == nil {
		t.Fatal("profile onboarding accepted caller-supplied positional input")
	}
	if profileOnboardHaftSoftwareCmd.Flags().Lookup("json") == nil {
		t.Fatal("profile onboarding omitted typed JSON result rendering")
	}
	for _, forbidden := range []string{
		"yes",
		"confirm",
		"profile",
		"payload",
		"candidate",
		"receipt",
		"authorization-ref",
		"authorization-digest",
		"project-root",
	} {
		if profileOnboardHaftSoftwareCmd.Flags().Lookup(forbidden) != nil {
			t.Fatalf("manual profile onboarding exposes forbidden flag --%s", forbidden)
		}
	}
	for _, want := range []string{
		"profile-declaration-review.json",
		`"haft profile propose"`,
		`"haft profile declare"`,
		"never mints",
	} {
		if !strings.Contains(profileOnboardHaftSoftwareCmd.Long, want) {
			t.Fatalf("manual boundary help omitted %q:\n%s", want, profileOnboardHaftSoftwareCmd.Long)
		}
	}
}

func TestWriteProfileOnboardResponsePrintsTypedTextAndJSON(t *testing.T) {
	response := profileOnboardTestResponse()
	textOutput := &bytes.Buffer{}
	if err := writeProfileOnboardResponse(textOutput, response, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Project profile declaration succeeded.",
		"Project: /tmp/haft-profile-test",
		"Authority: durable and bound to the admitted profile.",
		"Admission: committed to project memory at ledger revision 1.",
		"Profile projection: synchronized at .haft/project-profile.yaml.",
	} {
		if !strings.Contains(textOutput.String(), want) {
			t.Fatalf("text result omitted %q:\n%s", want, textOutput.String())
		}
	}
	for _, forbidden := range []string{
		"sha256:",
		"_ref:",
		"_digest:",
		"project_id:",
		"qnt_a11ce001",
		"profile-admission:test",
		"authority-resolution:test",
	} {
		if strings.Contains(textOutput.String(), forbidden) {
			t.Fatalf("human text result exposed %q:\n%s", forbidden, textOutput.String())
		}
	}

	jsonOutput := &bytes.Buffer{}
	if err := writeProfileOnboardResponse(jsonOutput, response, true); err != nil {
		t.Fatal(err)
	}
	decoded := profileOnboardResponse{}
	if err := json.Unmarshal(jsonOutput.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON result: %v\n%s", err, jsonOutput.String())
	}
	if !reflect.DeepEqual(decoded, response) {
		t.Fatalf("JSON result = %#v, want %#v", decoded, response)
	}
}

func TestProfileOnboardProjectionFailurePreservesCommittedAdmission(t *testing.T) {
	response := profileOnboardTestResponse()
	response.State = "projection_failed"
	response.Projection = nil
	response.Failure = &profileOnboardFailure{
		Stage:         "projection",
		Code:          "projection_write_failed",
		Detail:        "canonical admission committed; projection could not be written",
		CommitPosture: "committed",
		FailureRef:    "projection-failure:test",
	}
	output := &bytes.Buffer{}
	if err := writeProfileOnboardResponse(output, response, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Project profile declaration: projection_failed",
		"Project: /tmp/haft-profile-test",
		"Admission: committed to project memory at ledger revision 1.",
		"Failure stage: projection.",
		"Failure code: projection_write_failed.",
		"Commit posture: committed.",
		"Use --json for structured audit details.",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("failure result omitted %q:\n%s", want, output.String())
		}
	}
	for _, forbidden := range []string{
		"profile-admission:test",
		"sha256:",
		"canonical admission committed; projection could not be written",
		"projection-failure:test",
		"project_id",
	} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("failure text exposed audit detail %q:\n%s", forbidden, output.String())
		}
	}
	err := profileOnboardOutcomeError(response)
	if err == nil || !strings.Contains(err.Error(), "declaration committed") {
		t.Fatalf("projection failure exit error = %v", err)
	}
}

type cliProfileOnboardLedgerFixture struct {
	root      string
	projectID string
}

func newCLIProfileOnboardLedgerFixture(t *testing.T) cliProfileOnboardLedgerFixture {
	t.Helper()
	root := mustCLIProfileOnboardPhysicalPath(t, t.TempDir())
	home := mustCLIProfileOnboardPhysicalPath(t, t.TempDir())
	t.Setenv("HOME", home)
	projectID := "qnt_a11ce001"
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectCarrier := []byte("id: " + projectID + "\nname: cli-profile-onboard\n")
	if err := os.WriteFile(filepath.Join(haftDir, "project.yaml"), projectCarrier, 0o644); err != nil {
		t.Fatal(err)
	}
	databaseDir := filepath.Join(home, ".haft", "projects", projectID)
	if err := os.MkdirAll(databaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := openCurrentKernelTestStore(
		filepath.Join(databaseDir, "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}
	return cliProfileOnboardLedgerFixture{
		root:      root,
		projectID: projectID,
	}
}

func mustCLIProfileOnboardPhysicalPath(t *testing.T, path string) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return physical
}

func assertCLIProfileOnboardMutationCounts(
	t *testing.T,
	root string,
	want int,
) {
	t.Helper()
	handle, err := projectledger.OpenExisting(
		context.Background(),
		root,
		projectledger.ReadOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	assertCLIProfileOnboardDatabaseMutationCounts(t, handle.Database(), want)
}

func assertCLIProfileOnboardDatabaseMutationCounts(
	t *testing.T,
	database *sql.DB,
	want int,
) {
	t.Helper()
	for _, table := range []string{
		"profile_declaration_authorization_contents_v2",
		"profile_declaration_authorization_preparations_v2",
		"terminal_capture_records",
		"speech_acts",
		"profile_declaration_permissions_v2",
		"profile_declaration_instituted_effects_v2",
		"profile_declaration_authority_bases_v2",
		"profile_declaration_authority_resolutions_v2",
		"profile_author_role_assignments",
		"profile_onboarding_work_records",
		"project_profile_admissions_v2",
		"project_profile_revisions_v2",
	} {
		var got int
		query := "SELECT COUNT(*) FROM " + table
		if err := database.QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s rows = %d, want %d", table, got, want)
		}
	}
}

func profileOnboardTestSynchronizedOutcome() profileOnboardOutcome {
	return profileOnboardOutcome{
		State: "synchronized",
		Admission: &profileOnboardAdmission{
			RecordRef:                 "profile-admission:test",
			RecordDigest:              "sha256:" + strings.Repeat("b", 64),
			PayloadDigest:             "sha256:" + strings.Repeat("c", 64),
			ReceiptDigest:             "sha256:" + strings.Repeat("d", 64),
			WorkRecordRef:             "profile-work:test",
			WorkRecordDigest:          "sha256:" + strings.Repeat("e", 64),
			AuthorityBasisRef:         "authority-basis:test",
			AuthorityBasisDigest:      "sha256:" + strings.Repeat("f", 64),
			AuthorityResolutionRef:    "authority-resolution:test",
			AuthorityResolutionDigest: "sha256:" + strings.Repeat("a", 64),
			Delivery:                  "fresh",
			RecordedAt:                "2026-07-15T08:00:00Z",
		},
		Revision: &profileOnboardRevision{
			Expected: 0,
			Current:  1,
		},
		Projection: &profileOnboardProjection{
			Kind:           "synchronized",
			Path:           ".haft/project-profile.yaml",
			ExpectedDigest: "sha256:" + strings.Repeat("1", 64),
			ObservedDigest: "sha256:" + strings.Repeat("1", 64),
		},
	}
}

func profileOnboardTestResponse() profileOnboardResponse {
	outcome := profileOnboardTestSynchronizedOutcome()
	return profileOnboardResponse{
		Kind:        profileDeclarationRecordKind,
		State:       outcome.State,
		ProjectRoot: "/tmp/haft-profile-test",
		ProjectID:   "qnt_a11ce001",
		Admission:   outcome.Admission,
		Revision:    outcome.Revision,
		Projection:  outcome.Projection,
	}
}
