package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

const historicalSoftwareSpecStatusMarker = "historical-software-status-marker"

func TestPublicStatusUsesCurrentCanonicalScopeForSpecHealth(t *testing.T) {
	tests := []struct {
		name                string
		fixture             func(*testing.T) checkTestProject
		scopeID             string
		wantSoftwareFinding bool
		wantProfileCue      string
		wantProfileCueCount int
	}{
		{
			name:                "auto has one neutral cue",
			fixture:             newPublicStatusAutoFixture,
			wantProfileCue:      "Project profile is underdetermined",
			wantProfileCueCount: 1,
		},
		{
			name:    "singleton non-software omits software pressure",
			fixture: newPublicStatusNonSoftwareFixture,
		},
		{
			name:                "singleton software retains software pressure",
			fixture:             newPublicStatusSoftwareFixture,
			wantSoftwareFinding: true,
		},
		{
			name:                "mixed automatic requires exact scope",
			fixture:             newPublicStatusMixedFixture,
			wantProfileCue:      "Project profile has several scopes",
			wantProfileCueCount: 1,
		},
		{
			name:    "mixed exact non-software omits software pressure",
			fixture: newPublicStatusMixedFixture,
			scopeID: "documents-status",
		},
		{
			name:                "mixed exact software retains software pressure",
			fixture:             newPublicStatusMixedFixture,
			scopeID:             "software-status",
			wantSoftwareFinding: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := test.fixture(t)
			storeHistoricalSoftwareSpecStatusSignal(t, fixture.root)
			assertHistoricalSoftwareStatusSignalStored(t, fixture.root)

			args := map[string]any{
				"action": "status",
			}
			if test.scopeID != "" {
				args["scope_id"] = test.scopeID
			}
			result, err := handleQuintQuery(
				context.Background(),
				fixture.store,
				nil,
				fixture.haftDir,
				args,
			)
			if err != nil {
				t.Fatal(err)
			}

			if strings.Contains(result, historicalSoftwareSpecStatusMarker) {
				t.Fatalf("public status replayed historical spec health:\n%s", result)
			}
			softwareFinding := "software-system spec carrier is missing"
			if strings.Contains(result, softwareFinding) !=
				test.wantSoftwareFinding {
				t.Fatalf(
					"software finding posture mismatch; want=%t\n%s",
					test.wantSoftwareFinding,
					result,
				)
			}
			if test.wantProfileCue != "" &&
				strings.Count(result, test.wantProfileCue) !=
					test.wantProfileCueCount {
				t.Fatalf(
					"profile cue %q count = %d, want %d\n%s",
					test.wantProfileCue,
					strings.Count(result, test.wantProfileCue),
					test.wantProfileCueCount,
					result,
				)
			}

			assertHistoricalSoftwareStatusSignalStored(t, fixture.root)
		})
	}
}

func newPublicStatusAutoFixture(t *testing.T) checkTestProject {
	t.Helper()
	harness := profileadmissionfixture.New(t, t.TempDir())
	return publicStatusFixtureFromHarness(harness)
}

func newPublicStatusNonSoftwareFixture(t *testing.T) checkTestProject {
	t.Helper()
	harness := profileadmissionfixture.New(t, t.TempDir())
	harness.AdmitNonSoftwareRevision(t, "public-status-non-software")
	return publicStatusFixtureFromHarness(harness)
}

func newPublicStatusSoftwareFixture(t *testing.T) checkTestProject {
	t.Helper()
	harness := profileadmissionfixture.New(t, t.TempDir())
	harness.AdmitSoftwareRevision(t, "public-status-software")
	return publicStatusFixtureFromHarness(harness)
}

func newPublicStatusMixedFixture(t *testing.T) checkTestProject {
	t.Helper()
	harness := profileadmissionfixture.New(t, t.TempDir())
	scopes := []projectprofile.RealizationScope{
		mustCLIProjectSoftwareScope(t, "software-status"),
		mustCLIProjectNonSoftwareScope(t, "documents-status"),
	}
	admitSpecTestProfile(t, harness, "public-status-mixed", scopes)
	return publicStatusFixtureFromHarness(harness)
}

func publicStatusFixtureFromHarness(
	harness *profileadmissionfixture.Harness,
) checkTestProject {
	root := harness.Root().String()
	return checkTestProject{
		root:    root,
		haftDir: filepath.Join(root, ".haft"),
		store:   artifact.NewStore(harness.Database()),
		db:      harness.Database(),
	}
}

func storeHistoricalSoftwareSpecStatusSignal(t *testing.T, root string) {
	t.Helper()
	run, err := overseer.BuildMaintenanceRun(overseer.MaintenanceInput{
		CreatedAt: "2026-07-19T00:00:00Z",
		SpecHealth: []overseer.FindingSummary{
			{
				ID:       "historical-software-spec",
				Title:    "historical software spec health",
				Category: "error",
				Reason:   historicalSoftwareSpecStatusMarker,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := overseer.StoreMaintenanceRun(root, run); err != nil {
		t.Fatal(err)
	}
}

func assertHistoricalSoftwareStatusSignalStored(t *testing.T, root string) {
	t.Helper()
	summary, err := overseer.LoadStatusSummary(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, signal := range summary.Signals {
		content := signal.Title + " " + signal.Detail
		if strings.Contains(content, historicalSoftwareSpecStatusMarker) {
			return
		}
	}
	t.Fatal("historical software spec signal was not preserved in overseer storage")
}
