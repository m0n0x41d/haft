package projecttypeenvprofilecompatibility_test

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	basetypeenvartifacts "github.com/m0n0x41d/haft/data/haft/base-typeenv/artifacts"
	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	typeenvcompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	profilecompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

func TestProductionHistoricalToCurrentSuccessorKeepsInstalledProfilesCompatible(
	t *testing.T,
) {
	historicalBaseRef := mustProductionProfileTest(
		typedmemory.ParseTypeEnvRef(basetypeenvartifacts.HistoricalV6Ref),
	)
	historicalBase := mustProductionProfileTest(
		basetypeenvartifacts.LoadExact(historicalBaseRef),
	)

	basePath := filepath.Join("..", "cli", "fpf.db")
	database, err := sql.Open(
		"sqlite",
		"file:"+basePath+"?mode=ro&immutable=1",
	)
	if err != nil {
		t.Fatalf("open embedded FPF database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	currentBase := mustProductionProfileTest(typeenvsql.LoadArtifactReadOnlyDB(
		context.Background(),
		database,
	))

	priorSource := typedmemorycandidates.SourceV1_5()
	currentSource := typedmemorycandidates.SourceV1_6()
	if bytes.Equal(priorSource, currentSource) {
		t.Fatal("historical and current Local-Practice carriers are byte-identical")
	}
	prior := mustProductionProfileTest(
		localpracticeruntime.Build(historicalBase, priorSource),
	)
	current := mustProductionProfileTest(
		localpracticeruntime.Build(currentBase, currentSource),
	)
	assertProductionCarrierBytesStable(
		t,
		"historical Local-Practice 1.5.0",
		priorSource,
		typedmemorycandidates.SourceV1_5(),
	)
	assertProductionCarrierBytesStable(
		t,
		"current Local-Practice 1.6.0",
		currentSource,
		typedmemorycandidates.SourceV1_6(),
	)

	priorEnvironment, present := prior.Preparation().Environment()
	if !present {
		t.Fatal("historical production fixture has no executable environment")
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

	classCounts := map[typeenvcompatibility.SuccessorRuleClass]int{}
	for _, rule := range rules {
		classCounts[rule.Class()]++
		switch rule.Class() {
		case typeenvcompatibility.SuccessorUnchanged:
		case typeenvcompatibility.SuccessorCompilerGap:
			if rule.Family() != typeenvcompatibility.KindClassificationSignatureFamily ||
				rule.Ground() != typeenvcompatibility.GroundSemanticOrderNotImplemented {
				t.Fatalf(
					"production compiler gap %s is %s/%s, want %s/%s",
					rule.Key(),
					rule.Family().String(),
					rule.Ground(),
					typeenvcompatibility.KindClassificationSignatureFamily.String(),
					typeenvcompatibility.GroundSemanticOrderNotImplemented,
				)
			}
		case typeenvcompatibility.SuccessorAdditive,
			typeenvcompatibility.SuccessorWidened,
			typeenvcompatibility.SuccessorNarrowed,
			typeenvcompatibility.SuccessorRemoved:
			t.Fatalf(
				"production successor rule %s is %s, want unchanged or compiler gap",
				rule.Key(),
				rule.Class().String(),
			)
		default:
			t.Fatalf(
				"production successor rule %s has unsupported class %d",
				rule.Key(),
				rule.Class(),
			)
		}
	}
	assertProductionRuleClassCount(
		t,
		classCounts,
		typeenvcompatibility.SuccessorUnchanged,
		241,
	)
	assertProductionRuleClassCount(
		t,
		classCounts,
		typeenvcompatibility.SuccessorCompilerGap,
		12,
	)
	for _, class := range []typeenvcompatibility.SuccessorRuleClass{
		typeenvcompatibility.SuccessorAdditive,
		typeenvcompatibility.SuccessorWidened,
		typeenvcompatibility.SuccessorNarrowed,
		typeenvcompatibility.SuccessorRemoved,
	} {
		assertProductionRuleClassCount(t, classCounts, class, 0)
	}

	diffCanonical := diff.CanonicalBytes()
	decodedDiff := mustProductionProfileTest(
		typeenvcompatibility.DecodeSuccessorDiff(diffCanonical),
	)
	if decodedDiff.Digest() != diff.Digest() ||
		!bytes.Equal(decodedDiff.CanonicalBytes(), diffCanonical) {
		t.Fatal("production successor canonical round-trip changed identity")
	}

	profiles := mustProductionProfileTest(
		profilecompatibility.AssessInstalledProjectionProfiles(diff),
	)
	for _, profile := range profiles.Profiles() {
		if profile.Kind() != profilecompatibility.ProfileCompatible {
			t.Fatalf(
				"production successor made %s %s",
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

	profilesCanonical := profiles.CanonicalBytes()
	decodedProfiles := mustProductionProfileTest(
		profilecompatibility.DecodeProjectionProfileCompatibilitySet(
			profilesCanonical,
		),
	)
	if decodedProfiles.Digest() != profiles.Digest() ||
		!bytes.Equal(decodedProfiles.CanonicalBytes(), profilesCanonical) {
		t.Fatal("production profile compatibility canonical round-trip changed identity")
	}
}

func assertProductionCarrierBytesStable(
	t *testing.T,
	label string,
	used []byte,
	reread []byte,
) {
	t.Helper()
	if !bytes.Equal(used, reread) {
		t.Fatalf("%s carrier bytes changed while building the runtime", label)
	}
}

func assertProductionRuleClassCount(
	t *testing.T,
	counts map[typeenvcompatibility.SuccessorRuleClass]int,
	class typeenvcompatibility.SuccessorRuleClass,
	want int,
) {
	t.Helper()
	if got := counts[class]; got != want {
		t.Fatalf(
			"production successor %s rule count = %d, want %d",
			class.String(),
			got,
			want,
		)
	}
}

func mustProductionProfileTest[T any](value T, err error) T {
	if err != nil {
		panic("production profile compatibility fixture: " + err.Error())
	}
	return value
}
