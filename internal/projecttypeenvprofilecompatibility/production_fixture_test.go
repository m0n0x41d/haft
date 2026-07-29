package projecttypeenvprofilecompatibility_test

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	typeenvcompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	profilecompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	_ "modernc.org/sqlite"
)

func TestProductionUnchangedSuccessorKeepsInstalledProfilesCompatible(
	t *testing.T,
) {
	basePath := filepath.Join("..", "cli", "fpf.db")
	database, err := sql.Open(
		"sqlite",
		"file:"+basePath+"?mode=ro&immutable=1",
	)
	if err != nil {
		t.Fatalf("open embedded FPF database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	base := mustProductionProfileTest(typeenvsql.LoadArtifactReadOnlyDB(
		context.Background(),
		database,
	))
	priorSource := typedmemorycandidates.SourceV1_3()
	currentSource := successorProductionSource(t, priorSource)
	prior := mustProductionProfileTest(localpracticeruntime.Build(base, priorSource))
	current := mustProductionProfileTest(localpracticeruntime.Build(base, currentSource))
	priorEnvironment, present := prior.Preparation().Environment()
	if !present {
		t.Fatal("prior production fixture has no executable environment")
	}
	currentEnvironment, present := current.Preparation().Environment()
	if !present {
		t.Fatal("current production fixture has no executable environment")
	}
	diff := mustProductionProfileTest(typeenvcompatibility.CompareSuccessor(
		priorEnvironment,
		currentEnvironment,
	))
	rules := diff.Rules()
	if len(rules) != 253 {
		t.Fatalf("production successor rule count = %d, want 253", len(rules))
	}
	for _, rule := range rules {
		if rule.Class() != typeenvcompatibility.SuccessorUnchanged {
			t.Fatalf(
				"production successor rule %s is %s, want unchanged",
				rule.Key(),
				rule.Class().String(),
			)
		}
	}
	profiles := mustProductionProfileTest(
		profilecompatibility.AssessInstalledProjectionProfiles(diff),
	)
	for _, profile := range profiles.Profiles() {
		if profile.Kind() != profilecompatibility.ProfileCompatible {
			t.Fatalf(
				"unchanged production successor made %s %s",
				profile.ProfileRef().String(),
				profile.Kind().String(),
			)
		}
		for _, ground := range profile.Grounds() {
			if ground.Posture() != profilecompatibility.ProfileGroundSatisfied {
				t.Fatalf(
					"compatible production profile %s has %s ground",
					profile.ProfileRef().String(),
					ground.Posture().String(),
				)
			}
		}
	}
}

func successorProductionSource(t *testing.T, prior []byte) []byte {
	t.Helper()
	successor := append([]byte(nil), prior...)
	replacements := [][2]string{
		{"  edition: 1.3.0\n", "  edition: 1.3.1\n"},
		{"  version: 1.3.0\n", "  version: 1.3.1\n"},
	}
	for _, replacement := range replacements {
		before := []byte(replacement[0])
		if bytes.Count(successor, before) != 1 {
			t.Fatalf("successor source replacement %q is not unique", replacement[0])
		}
		successor = bytes.Replace(successor, before, []byte(replacement[1]), 1)
	}
	return successor
}

func mustProductionProfileTest[T any](value T, err error) T {
	if err != nil {
		panic("production profile compatibility fixture: " + err.Error())
	}
	return value
}
